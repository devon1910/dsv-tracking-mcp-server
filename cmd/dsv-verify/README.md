# cmd/dsv-verify

Live-API verification harness. Launches a headless browser, calls all three MCP
tools against the real DSV public tracking endpoint, and prints the JSON-RPC
request/response pairs to stdout.

**This hits live infrastructure.** It is not part of `go test`; run it manually
when you want to verify the full stack against real data, or after an upstream
change to confirm nothing silently broke.

```bash
# Basic run (uses waybill 1806290829 → VAN5022058)
go run ./cmd/dsv-verify/

# Show browser window to watch Cap.js solve
BROWSER_HEADLESS=false go run ./cmd/dsv-verify/

# Use make target
make verify
```

The tool calls made are:
1. `list_reference_types` — no upstream call, validates bundled data is intact
2. `track_shipment reference=1806290829` — resolves waybill → shipment summary
3. `get_shipment_details shipment_id=LandStt:VAN5022058:CTTS:LAND` — full detail

Expected outputs are captured in `docs/PHASE_4_VERIFICATION.md`.
