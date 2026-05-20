# ADR 0003 — No streaming / SSE tool variant

**Status**: Accepted  
**Date**: 2026-05-20

## Context

MCP supports streaming tool results via Server-Sent Events (SSE transport) and
progress notifications. A hypothetical `watch_shipment` tool could push status
updates to the client as the browser polls DSV periodically.

Arguments for a streaming variant:

- Richer UX: the LLM could narrate "now in customs", "out for delivery" without
  the user manually re-invoking `get_shipment_details`.
- Reduces round-trips when the user wants to monitor a shipment in real time.

Arguments against:

- **Headless browser cost**: each poll requires a full page navigation + Cap.js
  solve (~2–5 s). Long-lived SSE connections holding a browser tab open would
  exhaust the tab pool quickly.
- **MCP client support**: most MCP clients as of v1.6.0 do not render streaming
  tool results in a meaningful way; the UX gain is speculative.
- **Cache invalidation complexity**: a streaming tool bypasses the TTL cache and
  requires its own polling loop, error handling, and backoff — a substantial
  increase in scope.
- **Stateless is simpler**: the current three-tool design is fully stateless
  (no server-side subscriptions). Adding streaming introduces session state that
  must be cleaned up on disconnect.

## Decision

Do not implement a streaming or SSE tool variant in v1. The `get_shipment_details`
tool with its 30 s TTL cache (24 h for Delivered) is sufficient for the
conversational use case: a user asking "where is my parcel?" expects one answer,
not a live feed.

Revisit if:
- A client is identified that meaningfully renders streaming progress.
- A lightweight polling mechanism (not headless browser) becomes available.

## Consequences

- Users who want periodic updates must call `get_shipment_details` multiple
  times. Prompting them to do so is simple for the LLM.
- The server remains stateless, simplifying deployment (no persistent goroutines
  per session, no connection cleanup on client disconnect).
