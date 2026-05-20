package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/cache"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/data"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
)

// Upstream is the abstraction the tools use to reach DSV data.
// The concrete implementation wraps the DSV client + mapper.
type Upstream interface {
	Search(ctx context.Context, reference string) ([]domain.ShipmentSummary, error)
	Detail(ctx context.Context, shipmentID string) (domain.Shipment, error)
}

// ToolDeps groups the dependencies injected into all tool handlers.
type ToolDeps struct {
	Upstream    Upstream
	SearchCache *cache.Cache[[]domain.ShipmentSummary]
	DetailCache *cache.Cache[domain.Shipment]
	Logger      *slog.Logger
	Metrics     *obs.Metrics
}

// RegisterAll registers the three DSV tracking tools on the MCP server.
func RegisterAll(s *Server, d ToolDeps) {
	h := &toolHandlers{deps: d, refTypeCodes: loadRefTypeCodes()}

	sdkmcp.AddTool(s.sdk,
		&sdkmcp.Tool{
			Name: "track_shipment",
			Description: `Search for a shipment by reference number. Use this when the user provides a tracking number, waybill number, container number, booking reference, or similar identifier and you do not yet know the internal shipment id. Returns a list of matching shipment summaries; if exactly one matches, you can immediately call get_shipment_details with its shipment_id. Currently supports LAND shipments only (DSV road freight); SEA, AIR, and RAIL are not yet covered. If the user's reference type is ambiguous (e.g. a numeric string that could be several things), pass reference_type to narrow the search — call list_reference_types first to see valid values. Returns a list of matching shipment summaries including from/to locations, progress percentage, last event code, and start/end dates — enough signal to decide whether to immediately call get_shipment_details or surface a quick status to the user.`,
		},
		h.trackShipment,
	)

	sdkmcp.AddTool(s.sdk,
		&sdkmcp.Tool{
			Name: "get_shipment_details",
			Description: `Fetch full tracking detail for a known shipment. Returns shipment status, locations (shipper place, consignee place, collection point, delivery point, dispatching office — all at postcode/city/country level only; street addresses and party names are not exposed by DSV's public tracking endpoint), goods (weight, pieces, volume, loading meters, and dimensions when populated), the full chronological event history, and per-package events for each item in the shipment. Currently LAND-only. Use after track_shipment returns a shipment_id, or when the user provides one directly.`,
		},
		h.getShipmentDetails,
	)

	sdkmcp.AddTool(s.sdk,
		&sdkmcp.Tool{
			Name: "list_reference_types",
			Description: `List the 21 reference number types DSV's tracking API accepts (e.g. shipment number, container number, house bill, master bill). Each entry includes the code to pass as reference_type to track_shipment, a human label, and a regex pattern the reference must match. Use this when the user's reference is ambiguous, or when validating input before calling track_shipment.`,
		},
		h.listReferenceTypes,
	)
}

// ─── handler struct ──────────────────────────────────────────────────────────

type toolHandlers struct {
	deps         ToolDeps
	refTypeCodes map[string]struct{} // valid reference_type codes
}

// ─── Input / Output types (schema inferred by SDK) ───────────────────────────

type trackShipmentInput struct {
	Reference     string  `json:"reference"                jsonschema:"The tracking reference as provided by the user. Trim whitespace; do not modify case."`
	ReferenceType *string `json:"reference_type,omitempty" jsonschema:"Optional. One of the codes returned by list_reference_types. Omit to let DSV match across all types."`
}

type trackShipmentOutput struct {
	Shipments   []domain.ShipmentSummaryView `json:"shipments"`
	Freshness   string                       `json:"freshness"`
	RetrievedAt string                       `json:"retrieved_at"`
	Warnings    []string                     `json:"warnings,omitempty"`
}

type getShipmentDetailsInput struct {
	ShipmentID string `json:"shipment_id" jsonschema:"The composite shipment id from track_shipment results (e.g. 'LandStt:VAN5022058:CTTS:LAND'). Pass it through verbatim."`
}

type getShipmentDetailsOutput struct {
	Shipment    domain.ShipmentDetailView `json:"shipment"`
	Freshness   string                    `json:"freshness"`
	RetrievedAt string                    `json:"retrieved_at"`
}

type listReferenceTypesInput struct{} // no parameters

type listReferenceTypesOutput struct {
	ReferenceTypes []domain.ReferenceTypeView `json:"reference_types"`
	Freshness      string                     `json:"freshness"`
}

// ─── Tool handlers ────────────────────────────────────────────────────────────

func (h *toolHandlers) trackShipment(
	ctx context.Context,
	_ *sdkmcp.CallToolRequest,
	in trackShipmentInput,
) (*sdkmcp.CallToolResult, trackShipmentOutput, error) {
	start := time.Now()
	ref := strings.TrimSpace(in.Reference)

	if ref == "" {
		h.record("track_shipment", "invalid_input", start)
		return nil, trackShipmentOutput{}, errInvalidInput("reference", "must be non-empty")
	}

	var refType string
	if in.ReferenceType != nil {
		rt := strings.TrimSpace(*in.ReferenceType)
		if _, ok := h.refTypeCodes[rt]; !ok {
			h.record("track_shipment", "invalid_reference_type", start)
			return nil, trackShipmentOutput{}, errInvalidReferenceType(rt, h.validRefTypeCodes())
		}
		refType = rt
	}

	cacheKey := strings.ToLower(ref) + "|" + refType

	resp, err := h.deps.SearchCache.Fetch(ctx, cacheKey, func(ctx context.Context) ([]domain.ShipmentSummary, error) {
		return h.deps.Upstream.Search(ctx, ref)
	})
	if err != nil {
		h.record("track_shipment", "upstream_error", start)
		return nil, trackShipmentOutput{}, errFromUpstream(err)
	}

	views := make([]domain.ShipmentSummaryView, len(resp.Data))
	for i, s := range resp.Data {
		views[i] = domain.MapShipmentSummaryView(s)
	}

	h.record("track_shipment", "success", start)
	return nil, trackShipmentOutput{
		Shipments:   views,
		Freshness:   string(resp.Freshness),
		RetrievedAt: resp.FetchedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (h *toolHandlers) getShipmentDetails(
	ctx context.Context,
	_ *sdkmcp.CallToolRequest,
	in getShipmentDetailsInput,
) (*sdkmcp.CallToolResult, getShipmentDetailsOutput, error) {
	start := time.Now()
	sid := strings.TrimSpace(in.ShipmentID)

	if !validShipmentID(sid) {
		h.record("get_shipment_details", "invalid_shipment_id", start)
		return nil, getShipmentDetailsOutput{}, errInvalidShipmentID(sid)
	}

	resp, err := h.deps.DetailCache.Fetch(ctx, sid, func(ctx context.Context) (domain.Shipment, error) {
		return h.deps.Upstream.Detail(ctx, sid)
	})
	if err != nil {
		te := errFromUpstream(err)
		h.record("get_shipment_details", string(te.Code), start)
		return nil, getShipmentDetailsOutput{}, te
	}

	// Conditional TTL: delivered shipments are immutable — extend their cache
	// entry to 24 h so re-queries ("did it arrive?") never hit the browser.
	// Non-terminal shipments keep the default 30 s TTL for freshness.
	if resp.Data.Progress.ActiveStep == domain.ProgressStageDelivered && resp.Freshness == cache.FreshnessLive {
		h.deps.DetailCache.SetWithTTL(sid, resp.Data, 24*time.Hour)
	}

	h.record("get_shipment_details", "success", start)
	return nil, getShipmentDetailsOutput{
		Shipment:    domain.MapShipmentDetailView(resp.Data),
		Freshness:   string(resp.Freshness),
		RetrievedAt: resp.FetchedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (h *toolHandlers) listReferenceTypes(
	_ context.Context,
	_ *sdkmcp.CallToolRequest,
	_ listReferenceTypesInput,
) (*sdkmcp.CallToolResult, listReferenceTypesOutput, error) {
	var types []domain.ReferenceTypeView
	if err := json.Unmarshal(data.ReferenceTypesJSON, &types); err != nil {
		return nil, listReferenceTypesOutput{}, errInternal("failed to load reference types")
	}
	return nil, listReferenceTypesOutput{
		ReferenceTypes: types,
		Freshness:      "static",
	}, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func validShipmentID(id string) bool {
	parts := strings.Split(id, ":")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	return true
}


func (h *toolHandlers) record(tool, outcome string, start time.Time) {
	if h.deps.Metrics == nil {
		return
	}
	h.deps.Metrics.DSVMCPCalls.WithLabelValues(tool, outcome).Inc()
	h.deps.Metrics.DSVMCPLatency.WithLabelValues(tool).Observe(time.Since(start).Seconds())
}

// loadRefTypeCodes parses the embedded reference_types.json and builds a set
// of valid code values for fast validation in track_shipment.
func loadRefTypeCodes() map[string]struct{} {
	var types []domain.ReferenceTypeView
	_ = json.Unmarshal(data.ReferenceTypesJSON, &types)
	codes := make(map[string]struct{}, len(types))
	for _, t := range types {
		codes[t.Code] = struct{}{}
	}
	return codes
}

func (h *toolHandlers) validRefTypeCodes() []string {
	codes := make([]string, 0, len(h.refTypeCodes))
	for c := range h.refTypeCodes {
		codes = append(codes, c)
	}
	return codes
}
