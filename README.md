# dsv-tracking-mcp-server

A [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server written in Go that wraps DSV's public shipment tracking portal. It exposes three tools so that LLM agents can query live DSV shipment state without scraping HTML.

Built as a Sendify code challenge submission.

---

## What it does

DSV's public tracking API (`mydsv.dsv.com`) is protected by Cap.js proof-of-work. This server launches a persistent headless Chrome instance that solves Cap.js once on the first request and reuses the browser session for all subsequent calls. The raw XHR responses are parsed into a clean domain model and served over MCP stdio.

---

## Tools

| Tool | Description |
|------|-------------|
| `list_reference_types` | Lists the 21 reference number types DSV accepts (shipment number, container, house bill, etc.) including validation regex for each. Useful before calling `track_shipment` with an ambiguous reference. |
| `track_shipment` | Searches by tracking reference. Returns matching shipment summaries including the `shipment_id` needed for details. Pass `reference_type` to narrow the search when the reference is ambiguous. |
| `get_shipment_details` | Fetches full tracking detail for a known `shipment_id`: parties with addresses, ordered events, current status, and a URL to the DSV web UI. |

**Current scope**: LAND shipments only (DSV road freight). SEA, AIR, and RAIL are not yet covered — see [ADR 0001](docs/adr/0001-land-only-launch.md).

---

## Quickstart

**Requirements**: Go 1.24+, Chrome or Chromium on `PATH` (or set `CHROMEDP_EXEC_PATH`).

```bash
# Build
make build           # → bin/dsv-tracking-mcp

# Run (MCP over stdio)
./bin/dsv-tracking-mcp

# Development
make test            # unit + race detector
make verify          # live end-to-end against real DSV API (requires Chrome)
```

### Claude Desktop config

```json
{
  "mcpServers": {
    "dsv-tracking": {
      "command": "/path/to/bin/dsv-tracking-mcp"
    }
  }
}
```

---

## Configuration

All settings via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `METRICS_ADDR` | `:9090` | Prometheus metrics endpoint (empty string disables) |
| `CACHE_SEARCH_TTL` | `60s` | How long a search result is considered fresh |
| `CACHE_DETAIL_TTL` | `30s` | Default TTL for shipment detail (Delivered shipments get 24 h automatically) |
| `BROWSER_HEADLESS` | `true` | Set `false` to watch the browser window during debugging |
| `BROWSER_NAVIGATION_TIMEOUT` | `30s` | Max time to wait for page load |
| `BROWSER_XHR_TIMEOUT` | `20s` | Max time to wait for the tracking XHR after page load |
| `CHROMEDP_EXEC_PATH` | *(auto)* | Override Chrome binary path |

Duration values accept Go duration strings (`30s`, `5m`, `1h`).

---

## Architecture

```
stdin/stdout (MCP JSON-RPC)
    │
    ▼
internal/mcp          ← three tool handlers, TTL cache, error taxonomy
    │
    ▼
internal/upstream/dsv ← DTOs, mapper, retrying client
    │
    ▼
internal/upstream/dsv/browser  ← headless Chrome singleton (Cap.js bypass)
    │
    ▼
DSV public tracking API
```

Key design points:
- **Generic cache** (`internal/cache`) with singleflight coalescing, stale-fallback, and per-entry TTL override for Delivered shipments (24 h).
- **Typed error taxonomy** (`internal/mcp/errors.go`) — `ErrorCode` constants with structured JSON errors; all domain sentinel errors (`ErrShipmentNotFound`, etc.) pass through `errors.Is`-transparent.
- **Anti-detection browser** — allocator built from scratch (no `DefaultExecAllocatorOptions`) to avoid `--enable-automation`; uses `cdp.WithExecutor` to read XHR bodies across Tab redirects.

Documentation:
- [docs/UPSTREAM.md](docs/UPSTREAM.md) — DSV API recon, XHR fingerprint, error contract
- [docs/adr/](docs/adr/) — Architecture Decision Records
- [docs/SDK_NOTES.md](docs/SDK_NOTES.md) — MCP SDK gotchas with source line references
- [docs/ERROR_CODES.md](docs/ERROR_CODES.md) — Error code reference for LLM callers

---

## Verification

`cmd/dsv-verify` is a live-API harness that exercises all three tools end-to-end and prints JSON-RPC request/response pairs. It is not part of `go test`; run it manually to confirm the full stack against real DSV data.

```bash
make verify
# or
go run ./cmd/dsv-verify/
```

See [cmd/dsv-verify/README.md](cmd/dsv-verify/README.md) for details.

---

## Known gaps

- **SEA / AIR / RAIL**: only LAND shipments are implemented. Other modes return empty results.
- **Party names**: DSV's public API does not expose shipper or consignee names, only addresses. See [ADR 0002](docs/adr/0002-party-name-nullable.md).
- **No streaming**: `get_shipment_details` is a one-shot call; there is no push/watch variant. See [ADR 0003](docs/adr/0003-no-streaming-tool.md).
- **Chrome required**: the server will not start without a Chrome or Chromium binary. Distroless containers must bundle one and set `CHROMEDP_EXEC_PATH`.
- **Cold-start latency**: the first request after startup takes 2–5 s while Cap.js is solved. Subsequent requests reuse the browser session and are typically under 3 s.

---

## Development

```bash
make test             # go test -race ./...
make lint             # golangci-lint run
make build            # compile to bin/dsv-tracking-mcp
make verify           # live end-to-end (needs Chrome + network)
make test-integration # DSV_INTEGRATION_REF=<waybill> go test -tags integration ./...
```
