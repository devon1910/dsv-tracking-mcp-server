// verify exercises all three MCP tools end-to-end against the live DSV API
// and prints the JSON-RPC request/response pairs for PHASE_4_VERIFICATION.md.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/cache"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/mcp"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv/browser"
)

func main() {
	logger := obs.NewLogger()
	metrics := obs.NewMetrics()

	ctx := context.Background()

	br, err := browser.New(ctx, browser.Config{
		Headless: true,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		Metrics:  metrics,
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

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "verify-client"}, nil)
	sess, err := client.Connect(ctx, ct2, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "client connect:", err)
		os.Exit(1)
	}
	defer sess.Close()

	call := func(name string, args any) {
		reqJSON, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params":  map[string]any{"name": name, "arguments": args},
		})
		fmt.Printf("\n### Request: %s\n```json\n%s\n```\n\n", name, prettyJSON(reqJSON))

		result, err := sess.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      name,
			Arguments: args,
		})
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			return
		}

		var text string
		for _, c := range result.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				text = tc.Text
			}
		}
		respJSON, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"content": []map[string]any{{"type": "text", "text": json.RawMessage(text)}},
				"isError": result.IsError,
			},
		})
		fmt.Printf("### Response\n```json\n%s\n```\n", prettyJSON(respJSON))
	}

	// 1. list_reference_types (no upstream call)
	call("list_reference_types", map[string]any{})

	// 2. track_shipment for a known waybill
	call("track_shipment", map[string]any{"reference": "1806290829"})

	// 3. get_shipment_details for the resolved shipmentID
	call("get_shipment_details", map[string]any{"shipment_id": "LandStt:VAN5022058:CTTS:LAND"})
}

func prettyJSON(raw []byte) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
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
