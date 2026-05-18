# dsv-tracking-mcp-server

A Model Context Protocol (MCP) server written in Go that wraps DSV's public shipment tracking API, built as a Sendify code challenge submission. It exposes shipment tracking as MCP tools so that LLM agents can query live DSV shipment state without scraping HTML.

**Status: in development — chassis complete, DSV adapter and MCP tools forthcoming.**

---

## Documentation

- [docs/UPSTREAM.md](docs/UPSTREAM.md) — DSV public API recon: endpoints, field inventory, error contract, caching notes.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — system design and ADRs (Phase 5).

---

## Setup

Requires Go 1.25+. Run `make build` to compile the binary to `bin/dsv-tracking-mcp-server`.

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
| `CACHE_TTL`         | `5m`    | How long a cached shipment result is considered fresh |
| `CACHE_STALE_WINDOW`| `15m`   | How long past TTL a stale result may be served on upstream failure |

## Architecture

To be completed in Phase 5.

## Testing

Run `make test` to execute the test suite with the race detector (`go test -race ./...`).
The race detector requires CGO and a C compiler; on CI this runs on Ubuntu.

## Limitations

To be completed in Phase 5.
