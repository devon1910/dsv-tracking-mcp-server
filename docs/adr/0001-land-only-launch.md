# ADR 0001 — Land-only launch scope

**Status**: Accepted  
**Date**: 2026-05-20

## Context

DSV's public tracking portal (`mydsv.dsv.com`) exposes four transport modes:
LAND, SEA, AIR, and RAIL. Each mode has a distinct URL path and distinct DTO
shape. The headless-browser strategy we use to bypass Cap.js requires learning
the XHR fingerprint for each endpoint independently.

At launch we have:

- One confirmed XHR fingerprint for LAND search and LAND detail.
- No fixture files or mapper coverage for SEA, AIR, or RAIL.
- No integration-test harness that can validate the other modes against live
  traffic without running a browser.

## Decision

Ship v1 with LAND-only support. Surface this limitation explicitly in:

- Tool descriptions (`track_shipment`, `get_shipment_details`) — both state
  "Currently supports LAND shipments only".
- `README.md` Known Gaps section.

The domain layer and client are designed so that SEA/AIR/RAIL can be added by:
1. Adding fixture files for each mode.
2. Adding a mapper in `internal/upstream/dsv/`.
3. Registering the XHR URL pattern in `browser.go`.

No breaking changes to the public MCP API are anticipated.

## Consequences

- LLMs that call `track_shipment` with an air-waybill or container number will
  receive an empty results list rather than an error, because DSV's LAND
  endpoint simply returns no matches. This is the same behaviour a human sees
  when searching the wrong tab on the web UI.
- Users who need SEA/AIR/RAIL tracking cannot use this server until those modes
  are added. The tool descriptions warn them of this.
