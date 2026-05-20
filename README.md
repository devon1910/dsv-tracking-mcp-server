# dsv-tracking-mcp-server

A [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server written in Go that wraps DSV's public shipment tracking portal. It exposes three tools so that LLM agents can query live DSV shipment state without scraping HTML.

Built as a Sendify code challenge submission.

---

## What it does

DSV's public tracking API (`mydsv.dsv.com`) is protected by Cap.js proof-of-work bot detection. This server launches a persistent headless Chrome instance that solves Cap.js automatically on the first request and reuses the browser session for all subsequent calls. The raw XHR responses are parsed into a clean domain model and served over MCP.

**Transport**: MCP over stdio (stdin/stdout). The server has no HTTP API of its own — it is driven entirely by an MCP client such as Claude Desktop, Cursor, or any tool that speaks the MCP protocol. Connect a client to the process's stdin/stdout and it will discover the three tools automatically.

---

## Tools

| Tool | Description |
|------|-------------|
| `list_reference_types` | Lists the 21 reference number types DSV accepts (shipment number, container, house bill, etc.) including validation regex for each. Call this first if the user's reference is ambiguous. |
| `track_shipment` | Search by tracking reference (waybill, STT, container, etc.). Returns matching shipment summaries — status, locations, progress %, and the `shipment_id` needed for details. |
| `get_shipment_details` | Full detail for a known `shipment_id`: locations (postcode/city/country), goods (weight, pieces, volume), chronological event history, and per-package events. |

**Current scope**: LAND shipments only (DSV road freight). SEA, AIR, and RAIL are not yet covered — see [ADR 0001](docs/adr/0001-land-only-launch.md).

---

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| **Go 1.24+** | `go version` to check |
| **Chrome or Chromium** | Must be on `PATH`, or set `CHROMEDP_EXEC_PATH` to the binary |
| **Network access** | Calls `mydsv.dsv.com` on every tool invocation |

Chrome is required at runtime — the server will not start without it. On headless Linux environments, install `chromium-browser` or `google-chrome-stable` and ensure the binary is on `PATH`.

---

## Quickstart

### 1. Clone and build

```bash
git clone https://github.com/devon1910/dsv-tracking-mcp-server
cd dsv-tracking-mcp-server
go build -o bin/dsv-tracking-mcp ./cmd/dsv-tracking-mcp
```

On Windows (PowerShell):
```powershell
go build -o bin\dsv-tracking-mcp.exe .\cmd\dsv-tracking-mcp\
```

### 2. Verify it works end-to-end

This runs the full stack against the real DSV API and prints the tool responses. Chrome will launch in the background.

```bash
go run ./cmd/dsv-verify/
```

Expected output: three JSON-RPC request/response blocks — `list_reference_types`, `track_shipment`, and `get_shipment_details`. The first run takes 5–10 s while Cap.js is solved; subsequent calls are faster.

### 3. Connect to an MCP client

#### Claude Desktop

Find your config file:
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

Add the server:

```json
{
  "mcpServers": {
    "dsv-tracking": {
      "command": "/absolute/path/to/bin/dsv-tracking-mcp"
    }
  }
}
```

On Windows use double backslashes or forward slashes:
```json
{
  "mcpServers": {
    "dsv-tracking": {
      "command": "C:/Users/you/dsv-tracking-mcp-server/bin/dsv-tracking-mcp.exe"
    }
  }
}
```

Restart Claude Desktop. The three tools will appear in the tool picker. Ask Claude: *"Where is waybill 3476472018?"*

#### Cursor / other MCP clients

Most MCP clients accept the same JSON config shape. Point `command` at the binary and restart the client.

#### MCP Inspector (browser UI for interactive testing)

The fastest way to test tool inputs and inspect raw responses without setting up a full AI client:

```bash
npx @modelcontextprotocol/inspector ./bin/dsv-tracking-mcp
```

This opens a browser UI at `http://localhost:5173` where you can call each tool manually and see the raw JSON response.

#### Custom client code (Go SDK)

```go
import sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

ct1, ct2 := sdkmcp.NewInMemoryTransports()
// start server in a goroutine writing to ct1
client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "my-client"}, nil)
sess, _ := client.Connect(ctx, ct2, nil)

result, _ := sess.CallTool(ctx, &sdkmcp.CallToolParams{
    Name:      "track_shipment",
    Arguments: map[string]any{"reference": "3476472018"},
})
```

See [`cmd/dsv-verify/main.go`](cmd/dsv-verify/main.go) for a complete working example.

---

## Configuration

All settings via environment variables. Defaults work out of the box.

| Variable | Default | Description |
|----------|---------|-------------|
| `METRICS_ADDR` | `:9090` | Prometheus metrics endpoint. Set to empty string to disable. |
| `CACHE_SEARCH_TTL` | `60s` | How long a search result is considered fresh before re-querying DSV. |
| `CACHE_DETAIL_TTL` | `30s` | TTL for shipment detail. Delivered shipments are automatically extended to 24 h. |
| `BROWSER_HEADLESS` | `true` | Set `false` to see the Chrome window (useful for debugging Cap.js). |
| `BROWSER_NAVIGATION_TIMEOUT` | `30s` | Max time to wait for the page to load. |
| `BROWSER_XHR_TIMEOUT` | `20s` | Max time to wait for the tracking XHR after page load. |
| `CHROMEDP_EXEC_PATH` | *(auto)* | Override Chrome binary path when it is not on `PATH`. |
| `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, `error`. |

Duration values accept Go duration strings: `30s`, `5m`, `1h30m`.

Setting environment variables when running with `go run`:

```bash
# macOS / Linux
BROWSER_HEADLESS=false go run ./cmd/dsv-tracking-mcp/

# Windows PowerShell
$env:BROWSER_HEADLESS="false"; go run .\cmd\dsv-tracking-mcp\
```

---

## Observability

The server exposes Prometheus metrics at `http://localhost:9090/metrics` and a health check at `http://localhost:9090/healthz`.

Key metrics:

| Metric | Labels | Description |
|--------|--------|-------------|
| `dsv_mcp_tool_calls_total` | `tool`, `outcome` | Tool call count by outcome (`success`, `SHIPMENT_NOT_FOUND`, etc.) |
| `dsv_mcp_tool_latency_seconds` | `tool` | End-to-end tool latency histogram |
| `dsv_browser_fetches_total` | `endpoint`, `status` | Browser XHR fetch count by HTTP status |
| `dsv_browser_fetch_latency_seconds` | `endpoint` | Browser fetch latency histogram |

To inspect metrics locally, run the server and then:

```bash
curl http://localhost:9090/metrics
```

---

## Architecture

```
MCP client (Claude Desktop, Cursor, Inspector, custom code)
    │  stdin/stdout  JSON-RPC
    ▼
internal/mcp          ← tool handlers, TTL cache, typed error taxonomy
    │
    ▼
internal/upstream/dsv ← DTOs, domain mapper, browser-backed client
    │
    ▼
internal/upstream/dsv/browser  ← headless Chrome singleton, Cap.js bypass
    │  XHR interception via Chrome DevTools Protocol
    ▼
https://mydsv.dsv.com  (public tracking API)
```

Key design decisions:
- **Generic TTL cache** with singleflight coalescing and stale-while-revalidate. Delivered shipments cached 24 h; in-transit shipments 30 s.
- **Typed error taxonomy** — eight `ErrorCode` constants, all `errors.Is`-transparent. See [`docs/ERROR_CODES.md`](docs/ERROR_CODES.md).
- **Anti-detection browser** — Chrome allocator built from scratch (no `--enable-automation` flag); `cdp.WithExecutor` pattern to read XHR bodies across tab redirects.

Further reading:
- [`docs/UPSTREAM.md`](docs/UPSTREAM.md) — DSV API recon, XHR fingerprint, Cap.js behaviour
- [`docs/adr/`](docs/adr/) — Architecture Decision Records (land-only scope, location shape, no streaming)
- [`docs/SDK_NOTES.md`](docs/SDK_NOTES.md) — MCP Go SDK gotchas with source file line references
- [`docs/ERROR_CODES.md`](docs/ERROR_CODES.md) — Full error taxonomy for LLM callers

---

## Development

```bash
# Run tests (no race detector — CGO required for -race on Linux/CI)
go test ./...

# Run with race detector (Linux / CI)
go test -race ./...

# Live end-to-end verification against real DSV API
go run ./cmd/dsv-verify/

# Verify all 10 reference numbers from the Sendify challenge
go run ./cmd/dsv-verify-all/

# Lint
golangci-lint run ./...
```

Windows equivalents (PowerShell):
```powershell
go test ./...
go run .\cmd\dsv-verify\
```

---

## Known gaps

This server uses DSV's public tracking endpoint (`mydsv.dsv.com/app/tracking-public/`), which is intentionally privacy-limited:

- **Party names** — never exposed by the public endpoint. See [ADR 0002](docs/adr/0002-party-name-nullable.md).
- **Street addresses** — only postcode + city + country are available.
- **Dimensions** — `goods.dimensions` is empty on most shipments; surfaced as-is when populated.
- **SEA / AIR / RAIL** — only LAND (road freight) is implemented. See [ADR 0001](docs/adr/0001-land-only-launch.md).
- **No streaming** — `get_shipment_details` is a one-shot call. See [ADR 0003](docs/adr/0003-no-streaming-tool.md).

These are properties of the data source, not limitations of this server.

Operational notes:
- **Chrome required** — the server will not start without a Chrome or Chromium binary.
- **Cold-start latency** — the first request takes 5–10 s while Cap.js is solved. Subsequent requests reuse the session and typically complete in 2–4 s.
- **Windows** — `make` targets require GNU Make (install via Chocolatey: `choco install make`) or use the `go` commands directly as shown above.
