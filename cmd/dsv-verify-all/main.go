// dsv-verify-all runs all 10 of Holger's reference numbers through the full
// stack and prints a Markdown verification table for PHASE_6_VERIFICATION.md.
// This is a one-shot CLI tool; it is not part of go test.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/cache"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/mcp"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv/browser"
)

var refs = []string{
	"3476472018",
	"3476265230",
	"3476265248",
	"3476257542",
	"3476238161",
	"3476236157",
	"3476230325",
	"3476219849",
	"3476207869",
	"3476186295",
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	metrics := obs.NewMetrics()
	ctx := context.Background()

	br, err := browser.New(ctx, browser.Config{
		Headless:          true,
		NavigationTimeout: 45 * time.Second,
		XHRTimeout:        30 * time.Second,
		Logger:            logger,
		Metrics:           metrics,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to launch browser:", err)
		os.Exit(1)
	}
	defer br.Close()

	dsvClient := dsv.NewClient(dsv.ClientConfig{Browser: br, Logger: logger, Metrics: metrics})
	upstream := &dsvAdapter{client: dsvClient}

	searchCache := cache.New[[]domain.ShipmentSummary](
		cache.Config{TTL: 60 * time.Second, StaleWindow: 5 * time.Minute}, logger,
	)
	detailCache := cache.New[domain.Shipment](
		cache.Config{TTL: 30 * time.Second, StaleWindow: 5 * time.Minute}, logger,
	)

	srv := mcp.New(logger, metrics)
	mcp.RegisterAll(srv, mcp.ToolDeps{
		Upstream:    upstream,
		SearchCache: searchCache,
		DetailCache: detailCache,
		Logger:      logger,
		Metrics:     metrics,
	})

	ct1, ct2 := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.RunTransport(ctx, ct1) }()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "verify-all-client"}, nil)
	sess, err := client.Connect(ctx, ct2, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "client connect:", err)
		os.Exit(1)
	}
	defer sess.Close()

	type row struct {
		ref        string
		shipmentID string
		progress   int
		lastCode   string
		from, to   string
		goods      string
		pkgs       string
		notes      string
	}

	rows := make([]row, 0, len(refs))

	for _, ref := range refs {
		r := row{ref: ref}

		// track_shipment
		searchResult, serr := sess.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      "track_shipment",
			Arguments: map[string]any{"reference": ref},
		})
		if serr != nil {
			r.notes = "track_shipment error: " + serr.Error()
			rows = append(rows, r)
			continue
		}
		if searchResult.IsError {
			r.notes = "track_shipment tool error: " + textContent(searchResult)
			rows = append(rows, r)
			continue
		}

		var searchOut struct {
			Shipments []domain.ShipmentSummaryView `json:"shipments"`
		}
		if err := unmarshalContent(searchResult, &searchOut); err != nil {
			r.notes = "search parse error: " + err.Error()
			rows = append(rows, r)
			continue
		}
		if len(searchOut.Shipments) == 0 {
			r.notes = "not found"
			rows = append(rows, r)
			continue
		}
		s := searchOut.Shipments[0]
		r.shipmentID = s.ShipmentID
		r.progress = s.Progress
		r.lastCode = s.LastEventCode
		r.from = s.FromLocation
		r.to = s.ToLocation

		// get_shipment_details
		detailResult, derr := sess.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      "get_shipment_details",
			Arguments: map[string]any{"shipment_id": s.ShipmentID},
		})
		if derr != nil {
			r.notes = "detail error: " + derr.Error()
			rows = append(rows, r)
			continue
		}
		if detailResult.IsError {
			r.notes = "detail tool error: " + textContent(detailResult)
			rows = append(rows, r)
			continue
		}

		var detailOut struct {
			Shipment domain.ShipmentDetailView `json:"shipment"`
		}
		if err := unmarshalContent(detailResult, &detailOut); err != nil {
			r.notes = "detail parse error: " + err.Error()
			rows = append(rows, r)
			continue
		}
		d := detailOut.Shipment

		// Goods summary
		if d.Goods != nil {
			hasDims := len(d.Goods.Dimensions) > 0
			r.goods = fmt.Sprintf("%d pcs, %.3g %s", d.Goods.Pieces,
				goWeight(d.Goods), goWeightUnit(d.Goods))
			if hasDims {
				r.goods += fmt.Sprintf(", %d dims", len(d.Goods.Dimensions))
				r.notes = appendNote(r.notes, "dimensions populated")
			}
		} else {
			r.goods = "nil"
			r.notes = appendNote(r.notes, "goods block absent")
		}

		// Packages summary
		totalPkgEvts := 0
		for _, p := range d.Packages {
			totalPkgEvts += len(p.Events)
		}
		r.pkgs = fmt.Sprintf("%d pkg / %d evts", len(d.Packages), totalPkgEvts)
		if len(d.Packages) > 1 {
			r.notes = appendNote(r.notes, fmt.Sprintf("%d packages", len(d.Packages)))
		}

		rows = append(rows, r)
	}

	// Print Markdown table
	fmt.Println("| Ref | Shipment ID | Progress | Last Code | From → To | Goods | Packages | Notes |")
	fmt.Println("|-----|-------------|----------|-----------|-----------|-------|----------|-------|")
	for _, r := range rows {
		sid := r.shipmentID
		if sid == "" {
			sid = "—"
		}
		fmt.Printf("| %s | %s | %d%% | %s | %s → %s | %s | %s | %s |\n",
			r.ref, sid, r.progress, r.lastCode,
			r.from, r.to,
			r.goods, r.pkgs, r.notes)
	}
}

func goWeight(g *domain.GoodsView) float64 {
	if g.Weight != nil {
		return g.Weight.Value
	}
	return 0
}
func goWeightUnit(g *domain.GoodsView) string {
	if g.Weight != nil {
		return g.Weight.Unit
	}
	return ""
}

func textContent(r *sdkmcp.CallToolResult) string {
	for _, c := range r.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func unmarshalContent(r *sdkmcp.CallToolResult, v any) error {
	text := textContent(r)
	return json.Unmarshal([]byte(text), v)
}

func appendNote(existing, note string) string {
	if existing == "" {
		return note
	}
	return existing + "; " + note
}

type dsvAdapter struct{ client *dsv.Client }

func (a *dsvAdapter) Search(ctx context.Context, ref string) ([]domain.ShipmentSummary, error) {
	dto, err := a.client.Search(ctx, ref)
	if err != nil {
		return nil, err
	}
	return dsv.MapShipmentSummaries(dto), nil
}

func (a *dsvAdapter) Detail(ctx context.Context, shipmentID string) (domain.Shipment, error) {
	dto, err := a.client.Detail(ctx, shipmentID)
	if err != nil {
		return domain.Shipment{}, err
	}
	return dsv.MapShipmentDetail(dto)
}

// Silence unused import warning for strings
var _ = strings.TrimSpace
