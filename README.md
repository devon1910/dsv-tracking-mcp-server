# dsv-tracking-mcp-server

A Model Context Protocol (MCP) server written in Go that wraps DSV's public shipment tracking API, built as a Sendify code challenge submission. It exposes shipment tracking as MCP tools so that LLM agents can query live DSV shipment state without scraping HTML.

**Status: in development — chassis complete, DSV adapter and MCP tools forthcoming.**

---

## Documentation

- [docs/UPSTREAM.md](docs/UPSTREAM.md) — DSV public API recon: endpoints, field inventory, error contract, caching notes.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — system design and ADRs (Phase 5).

---

## Setup

Requires Go 1.25+ and Chrome (or Chromium) installed and discoverable via `PATH` or `CHROMEDP_EXEC_PATH` env var. Run `make build` to compile the binary to `bin/dsv-tracking-mcp-server`.

## Usage

Run `make run` to start the server. The MCP server speaks over stdio; connect
an MCP client to the process's stdin/stdout. The Prometheus metrics endpoint is
available at `http://localhost:9090/metrics` by default (override with
`METRICS_ADDR`).

Environment variables:

| Variable            | Default | Description                                      |
|---------------------|---------|--------------------------------------------------|
| `LOG_LEVEL`         | `info`  | Log level: `debug`, `info`, `warn`, `error`      |
| `METRICS_ADDR`      | `:9090` | Address for the Prometheus metrics HTTP server   |
| `CACHE_TTL`              | `5m`    | How long a cached shipment result is considered fresh |
| `CACHE_STALE_WINDOW`     | `15m`   | How long past TTL a stale result may be served on upstream failure |
| `BROWSER_HEADLESS`       | `true`  | Set to `false` to watch the browser window (useful for local debugging) |
| `BROWSER_NO_SANDBOX`     | `false` | Set to `true` when running in environments without user namespaces (e.g. some Docker setups) |
| `BROWSER_NAVIGATION_TIMEOUT` | `30s` | Maximum time to wait for a page's load event |
| `BROWSER_XHR_TIMEOUT`   | `20s`   | Maximum time to wait for the tracking API XHR after page load |
| `CHROMEDP_EXEC_PATH`     | *(auto)* | Override Chrome binary path; auto-discovered from PATH if unset |

## Architecture

To be completed in Phase 5.

## Testing

Run `make test` to execute the test suite with the race detector (`go test -race ./...`).
The race detector requires CGO and a C compiler; on CI this runs on Ubuntu.

Run integration tests against the live DSV API (requires Chrome and a working reference):
```
DSV_INTEGRATION_REF=VAN5022058 make test-integration
```

## Startup behaviour

The server launches a headless Chrome process at startup. This adds approximately 2–4 seconds to cold-start time. Once Cap.js validates in the first request, subsequent requests reuse the browser's session state (cookies) until the session expires — the captcha is solved once and amortised across all requests.

## Limitations

- Requires a working Chrome or Chromium installation. Running in distroless containers requires bundling a Chromium binary and pointing `CHROMEDP_EXEC_PATH` at it.
- Tested with LAND transport mode only; SEA/AIR/RAIL transport modes are not implemented in v1.
