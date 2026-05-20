# DSV Tracking MCP — Error Code Reference

This document describes every error code the server can return. It is written
for LLM callers: each section explains when the code fires, whether retrying
helps, and what action to take next.

All tool errors are returned as `{"code": "...", "message": "...", "details": {...}}`
in the tool result's TextContent with `isError: true`.

---

## INVALID_INPUT

**When it fires:** A required field was missing, empty after trimming, or
obviously malformed (e.g. `reference` is an empty string).

**Retry-worthy:** No. The input is wrong; retrying the same call returns the
same error.

**User-actionable:** Yes. Ask the user to provide or correct the field named in
`message`.

**Example:**
```json
{ "code": "INVALID_INPUT", "message": "reference: must be non-empty" }
```

---

## INVALID_REFERENCE_TYPE

**When it fires:** The optional `reference_type` field was provided but its
value is not in the catalog of 21 known types.

**Retry-worthy:** No.

**User-actionable:** Yes. Call `list_reference_types` to get the valid codes and
present them to the user.

**Example:**
```json
{
  "code": "INVALID_REFERENCE_TYPE",
  "message": "\"UNKNOWN\" is not a valid reference_type",
  "details": { "valid_codes": ["Stt","WaybillNo","PackageId",...] }
}
```

---

## INVALID_SHIPMENT_ID

**When it fires:** `shipment_id` does not match the composite form
`Provider:Ref:DataProvider:Mode` (e.g. `LandStt:VAN5022058:CTTS:LAND`).

**Retry-worthy:** No.

**User-actionable:** Yes. Call `track_shipment` first to resolve a reference
string to a valid shipment_id, then call `get_shipment_details` with the result.

**Example:**
```json
{
  "code": "INVALID_SHIPMENT_ID",
  "message": "shipment_id must have the form Provider:Ref:DataProvider:Mode ...",
  "details": { "received": "VAN5022058" }
}
```

---

## SHIPMENT_NOT_FOUND

**When it fires:** DSV's tracking API confirmed that no shipment matches the
given reference or shipment_id.

**Retry-worthy:** No. DSV is authoritative; retrying the same reference returns
the same answer.

**User-actionable:** Yes. Ask the user to double-check the reference. Common
mistakes: leading/trailing spaces, a waybill number that has been archived, or
a reference that belongs to a SEA/AIR shipment (not yet supported).

**Example:**
```json
{ "code": "SHIPMENT_NOT_FOUND", "message": "no shipment found for id \"LandStt:X:CTTS:LAND\"" }
```

---

## UPSTREAM_ERROR

**When it fires:** DSV returned an unexpected error that doesn't map to a
specific code above.

**Retry-worthy:** Sometimes. If `details.upstream_message` mentions a temporary
condition, one retry after 5–10 seconds is reasonable.

**User-actionable:** No. Surface the `message` to the user; they cannot resolve
a DSV-side error.

**Example:**
```json
{
  "code": "UPSTREAM_ERROR",
  "message": "DSV returned an unexpected error",
  "details": { "upstream_message": "browser_fetch: upstream unavailable ..." }
}
```

---

## UPSTREAM_TIMEOUT

**When it fires:** The headless browser did not receive the expected XHR
response within the configured deadline (default 20 s for XHR, 30 s for page
navigation).

**Retry-worthy:** Yes, once. Cap.js can occasionally take longer on a cold
browser session. A second attempt usually succeeds after the session is warm.

**User-actionable:** No. If retries fail, surface the timeout to the user.

**Example:**
```json
{ "code": "UPSTREAM_TIMEOUT", "message": "browser fetch exceeded deadline" }
```

---

## UPSTREAM_UNAVAILABLE

**When it fires:** DSV is unreachable, or Cap.js throttled the request (HTTP
429). This can happen on the first request to a cold server if the browser pool
hasn't warmed up, or during a DSV outage.

**Retry-worthy:** Yes. Wait 10–30 seconds before retrying.

**User-actionable:** No. If unavailability persists, inform the user that DSV
tracking is temporarily unavailable.

**Example:**
```json
{
  "code": "UPSTREAM_UNAVAILABLE",
  "message": "DSV is unreachable",
  "details": { "upstream_message": "browser_fetch: upstream unavailable ..." }
}
```

---

## INTERNAL_ERROR

**When it fires:** A bug in this server — a mapper panic, corrupted embedded
data, or any other condition that should not occur in normal operation.

**Retry-worthy:** No.

**User-actionable:** No. The server needs to be restarted or debugged. Log the
full response for the developer.

**Example:**
```json
{ "code": "INTERNAL_ERROR", "message": "failed to load reference types" }
```
