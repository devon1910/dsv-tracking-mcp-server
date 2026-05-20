package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/cache"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	mcpinternal "github.com/devon1910/dsv-tracking-mcp-server/internal/mcp"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	// in-process MCP transport for testing
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
)

// ─── fake upstream ────────────────────────────────────────────────────────────

type fakeUpstream struct {
	searchResult []domain.ShipmentSummary
	searchErr    error
	searchCalls  int

	detailResult domain.Shipment
	detailErr    error
	detailCalls  int
}

func (f *fakeUpstream) Search(_ context.Context, _ string) ([]domain.ShipmentSummary, error) {
	f.searchCalls++
	return f.searchResult, f.searchErr
}

func (f *fakeUpstream) Detail(_ context.Context, _ string) (domain.Shipment, error) {
	f.detailCalls++
	return f.detailResult, f.detailErr
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

func newDeps(up *fakeUpstream) mcpinternal.ToolDeps {
	return mcpinternal.ToolDeps{
		Upstream:    up,
		SearchCache: cache.New[[]domain.ShipmentSummary](cache.Config{TTL: time.Minute, StaleWindow: 5 * time.Minute}, noopLogger()),
		DetailCache: cache.New[domain.Shipment](cache.Config{TTL: 30 * time.Second, StaleWindow: 5 * time.Minute}, noopLogger()),
		Logger:      noopLogger(),
		Metrics:     obs.NewMetrics(),
	}
}

// callTool calls a registered tool by name with the given JSON args and
// returns the CallToolResult via an in-process transport.
func callTool(t *testing.T, s *mcpinternal.Server, toolName string, args any) *sdkmcp.CallToolResult {
	t.Helper()

	ct1, ct2 := sdkmcp.NewInMemoryTransports()
	ctx := context.Background()

	go func() { _ = s.RunTransport(ctx, ct1) }()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client"}, nil)
	clientSess, err := client.Connect(ctx, ct2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSess.Close()

	// Arguments must be a Go value (map/struct), not pre-marshaled []byte.
	// The SDK (CallToolParams.Arguments is `any`) marshals it to JSON internally.
	result, err := clientSess.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", toolName, err)
	}
	return result
}

// loadDetailFixture loads a detail fixture and maps it to a domain.Shipment.
func loadDetailFixture(t *testing.T, name string) domain.Shipment {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(f), "..", "..", "testdata")
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var dto dsv.ShipmentDetailDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	s, err := dsv.MapShipmentDetail(&dto)
	if err != nil {
		t.Fatalf("MapShipmentDetail: %v", err)
	}
	return s
}

// firstText returns the text from the first TextContent item in a result.
func firstText(r *sdkmcp.CallToolResult) string {
	for _, c := range r.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// ─── track_shipment tests ─────────────────────────────────────────────────────

func TestTrackShipment_HappyPath(t *testing.T) {
	up := &fakeUpstream{
		searchResult: []domain.ShipmentSummary{{
			ShipmentID:    "LandStt:VAN5022058:CTTS:LAND",
			STTNumber:     "VAN5022058",
			TransportMode: domain.TransportModeLand,
			LastEventCode: domain.EventCodeDLV,
		}},
	}
	s := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s, newDeps(up))

	result := callTool(t, s, "track_shipment", map[string]any{"reference": "VAN5022058"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", firstText(result))
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(firstText(result)), &out); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	shipments := out["shipments"].([]any)
	if len(shipments) != 1 {
		t.Errorf("len(shipments) = %d, want 1", len(shipments))
	}
	if out["freshness"] != "live" {
		t.Errorf("freshness = %v, want live", out["freshness"])
	}
	if up.searchCalls != 1 {
		t.Errorf("searchCalls = %d, want 1", up.searchCalls)
	}
}

func TestTrackShipment_CachedHit(t *testing.T) {
	up := &fakeUpstream{
		searchResult: []domain.ShipmentSummary{{ShipmentID: "LandStt:X:CTTS:LAND", STTNumber: "X"}},
	}
	deps := newDeps(up)
	s := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s, deps)

	// Prime the cache with a first call.
	callTool(t, s, "track_shipment", map[string]any{"reference": "X"})

	// Second call: must not call upstream.
	result := callTool(t, s, "track_shipment", map[string]any{"reference": "X"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", firstText(result))
	}
	var out map[string]any
	json.Unmarshal([]byte(firstText(result)), &out)
	if out["freshness"] != "cached" {
		t.Errorf("freshness = %v, want cached", out["freshness"])
	}
	if up.searchCalls != 1 {
		t.Errorf("searchCalls = %d, want 1 (second call should be cache hit)", up.searchCalls)
	}
}

func TestTrackShipment_EmptyReference(t *testing.T) {
	s := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s, newDeps(&fakeUpstream{}))

	result := callTool(t, s, "track_shipment", map[string]any{"reference": "  "})
	if !result.IsError {
		t.Fatal("expected error for empty reference")
	}
	assertErrCode(t, firstText(result), "INVALID_INPUT")
}

func TestTrackShipment_BadReferenceType(t *testing.T) {
	s := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s, newDeps(&fakeUpstream{}))

	result := callTool(t, s, "track_shipment", map[string]any{
		"reference":      "VAN5022058",
		"reference_type": "NotARealType",
	})
	if !result.IsError {
		t.Fatal("expected error for bad reference_type")
	}
	assertErrCode(t, firstText(result), "INVALID_REFERENCE_TYPE")
}

func TestTrackShipment_UpstreamEmptyResult(t *testing.T) {
	up := &fakeUpstream{searchResult: []domain.ShipmentSummary{}}
	s := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s, newDeps(up))

	result := callTool(t, s, "track_shipment", map[string]any{"reference": "NOTEXIST"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", firstText(result))
	}
	var out map[string]any
	json.Unmarshal([]byte(firstText(result)), &out)
	shipments := out["shipments"].([]any)
	if len(shipments) != 0 {
		t.Errorf("len(shipments) = %d, want 0", len(shipments))
	}
}

// ─── get_shipment_details tests ───────────────────────────────────────────────

func TestGetShipmentDetails_HappyPath(t *testing.T) {
	shipment := loadDetailFixture(t, "delivered_ltl_se_fr.json")
	up := &fakeUpstream{detailResult: shipment}
	s := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s, newDeps(up))

	result := callTool(t, s, "get_shipment_details", map[string]any{
		"shipment_id": "LandStt:VAN5022058:CTTS:LAND",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", firstText(result))
	}

	var out map[string]any
	json.Unmarshal([]byte(firstText(result)), &out)
	ship := out["shipment"].(map[string]any)

	if ship["shipment_id"] != "LandStt:VAN5022058:CTTS:LAND" {
		t.Errorf("shipment_id = %v", ship["shipment_id"])
	}
	if ship["status"] != "Delivered" {
		t.Errorf("status = %v, want Delivered", ship["status"])
	}
	events := ship["events"].([]any)
	if len(events) == 0 {
		t.Error("expected non-empty events")
	}
	// Events must be in ascending date order.
	for i := 1; i < len(events); i++ {
		prev := events[i-1].(map[string]any)["date"].(string)
		curr := events[i].(map[string]any)["date"].(string)
		if curr < prev {
			t.Errorf("events not sorted: [%d]=%s > [%d]=%s", i-1, prev, i, curr)
		}
	}
	if out["freshness"] != "live" {
		t.Errorf("freshness = %v, want live", out["freshness"])
	}
}

func TestGetShipmentDetails_NotFound(t *testing.T) {
	up := &fakeUpstream{detailErr: &domain.UpstreamError{Err: domain.ErrShipmentNotFound}}
	s := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s, newDeps(up))

	result := callTool(t, s, "get_shipment_details", map[string]any{
		"shipment_id": "LandStt:NOTEXIST:CTTS:LAND",
	})
	if !result.IsError {
		t.Fatal("expected error for not-found shipment")
	}
	assertErrCode(t, firstText(result), "SHIPMENT_NOT_FOUND")
}

func TestGetShipmentDetails_MalformedID(t *testing.T) {
	up := &fakeUpstream{}
	s := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s, newDeps(up))

	result := callTool(t, s, "get_shipment_details", map[string]any{
		"shipment_id": "not-a-composite-id",
	})
	if !result.IsError {
		t.Fatal("expected error for malformed shipment_id")
	}
	assertErrCode(t, firstText(result), "INVALID_SHIPMENT_ID")
	if up.detailCalls != 0 {
		t.Errorf("detailCalls = %d, want 0 (no upstream call for malformed id)", up.detailCalls)
	}
}

func TestGetShipmentDetails_StaleFallback(t *testing.T) {
	// Use an in-transit fixture: a DELIVERED shipment would get a 24 h TTL
	// from the conditional-TTL logic and would never appear stale in this test.
	shipment := loadDetailFixture(t, "dispatching_parcel_se_se.json")
	// Short TTL to force expiry quickly.
	shortDeps := mcpinternal.ToolDeps{
		Upstream:    &fakeUpstream{detailResult: shipment},
		SearchCache: cache.New[[]domain.ShipmentSummary](cache.Config{TTL: time.Minute}, noopLogger()),
		DetailCache: cache.New[domain.Shipment](cache.Config{TTL: 20 * time.Millisecond, StaleWindow: time.Minute}, noopLogger()),
		Logger:      noopLogger(),
		Metrics:     obs.NewMetrics(),
	}

	s1 := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s1, shortDeps)

	const sid = "LandStt:SEKSD620143489:CTTS:LAND" // dispatching (non-terminal)
	// Prime cache.
	callTool(t, s1, "get_shipment_details", map[string]any{"shipment_id": sid})

	// Let it expire.
	time.Sleep(40 * time.Millisecond)

	// Replace upstream with one that fails on detail.
	shortDeps.Upstream = &fakeUpstream{detailErr: errors.New("upstream down")}
	s2 := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s2, shortDeps)

	result := callTool(t, s2, "get_shipment_details", map[string]any{"shipment_id": sid})
	if result.IsError {
		t.Fatalf("expected stale fallback, got error: %s", firstText(result))
	}
	var out map[string]any
	json.Unmarshal([]byte(firstText(result)), &out)
	if out["freshness"] != "stale_fallback" {
		t.Errorf("freshness = %v, want stale_fallback", out["freshness"])
	}
}

// TestConditionalTTL_DeliveredShipmentGets24hTTL verifies that after fetching
// a DELIVERED shipment, a second call well past the default 30 s TTL still
// returns a cached result (24 h TTL was applied).
func TestConditionalTTL_DeliveredShipmentGets24hTTL(t *testing.T) {
	shipment := loadDetailFixture(t, "delivered_ltl_se_fr.json") // DELIVERED
	up := &fakeUpstream{detailResult: shipment}

	// Use a very short default TTL; the conditional 24 h TTL must override it.
	const defaultTTL = 15 * time.Millisecond
	deps := mcpinternal.ToolDeps{
		Upstream:    up,
		SearchCache: cache.New[[]domain.ShipmentSummary](cache.Config{TTL: time.Minute}, noopLogger()),
		DetailCache: cache.New[domain.Shipment](cache.Config{TTL: defaultTTL, StaleWindow: defaultTTL}, noopLogger()),
		Logger:      noopLogger(),
		Metrics:     obs.NewMetrics(),
	}
	s := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s, deps)

	// First call — live fetch.
	callTool(t, s, "get_shipment_details", map[string]any{"shipment_id": "LandStt:VAN5022058:CTTS:LAND"})

	// Wait past the default TTL; the conditional 24 h TTL must keep the entry alive.
	time.Sleep(defaultTTL * 3)

	result := callTool(t, s, "get_shipment_details", map[string]any{"shipment_id": "LandStt:VAN5022058:CTTS:LAND"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", firstText(result))
	}
	var out map[string]any
	json.Unmarshal([]byte(firstText(result)), &out)
	if out["freshness"] != "cached" {
		t.Errorf("freshness = %v, want cached (24 h TTL should still be valid)", out["freshness"])
	}
	// Upstream must not have been called a second time.
	if up.detailCalls != 1 {
		t.Errorf("detailCalls = %d, want 1 (cache hit expected)", up.detailCalls)
	}
}

// ─── list_reference_types tests ───────────────────────────────────────────────

func TestListReferenceTypes(t *testing.T) {
	s := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(s, newDeps(&fakeUpstream{}))

	result := callTool(t, s, "list_reference_types", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", firstText(result))
	}

	var out map[string]any
	json.Unmarshal([]byte(firstText(result)), &out)

	types := out["reference_types"].([]any)
	if len(types) != 21 {
		t.Errorf("len(reference_types) = %d, want 21", len(types))
	}
	// First entry must be Stt (JSON array order is deterministic).
	first := types[0].(map[string]any)
	if first["code"] != "Stt" {
		t.Errorf("first code = %v, want Stt", first["code"])
	}
	if out["freshness"] != "static" {
		t.Errorf("freshness = %v, want static", out["freshness"])
	}
}

// ─── assertion helpers ────────────────────────────────────────────────────────

func assertErrCode(t *testing.T, text, wantCode string) {
	t.Helper()
	var e map[string]any
	if err := json.Unmarshal([]byte(text), &e); err != nil {
		t.Fatalf("parse error text %q: %v", text, err)
	}
	if e["code"] != wantCode {
		t.Errorf("error code = %v, want %s", e["code"], wantCode)
	}
}
