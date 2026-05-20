# cmd/dsv-verify

Live end-to-end verification harness. Launches a headless browser, calls all three MCP tools against the real DSV public tracking endpoint, and prints the JSON-RPC request/response pairs to stdout.

**This hits live infrastructure.** It is not part of `go test`; run it manually when you want to verify the full stack against real data, or after an upstream change to confirm nothing silently broke.

```bash
# Basic run — uses waybill 1806290829 (resolves to LandStt:VAN5022058:CTTS:LAND)
go run ./cmd/dsv-verify/

# Watch the browser window to observe Cap.js being solved
BROWSER_HEADLESS=false go run ./cmd/dsv-verify/
```

Windows PowerShell:
```powershell
go run .\cmd\dsv-verify\

# With browser window visible
$env:BROWSER_HEADLESS="false"; go run .\cmd\dsv-verify\
```

The tool calls made, in order:
1. `list_reference_types` — reads bundled JSON, no upstream call; validates the embedded data is intact
2. `track_shipment reference=1806290829` — resolves waybill → shipment summary
3. `get_shipment_details shipment_id=LandStt:VAN5022058:CTTS:LAND` — full detail

Each call's request and response are printed as Markdown-fenced JSON to stdout.

For testing all 10 of the Sendify challenge reference numbers in one run, use `cmd/dsv-verify-all` instead:

```bash
go run ./cmd/dsv-verify-all/
```
