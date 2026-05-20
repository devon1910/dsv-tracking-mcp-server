# DSV Public Tracking API — Upstream Recon

> Status: based on recon samples gathered May 2026. No authenticated endpoints used.

## Overview

DSV's public shipment tracking is served from `mydsv.dsv.com`. The human-facing UI lives at `https://mydsv.dsv.com/app/tracking-public/` and fetches data from a set of JSON endpoints under `/nges-portal/api/public/tracking-public/`. This MCP server calls those JSON endpoints directly rather than scraping the rendered HTML. All observed traffic is unauthenticated; the API appears to be the same backend the public tracking page uses.

---

## Endpoints

### 1. Reference type discovery

```
GET https://mydsv.dsv.com/nges-portal/api/public/tracking-public/reference-types
```

Returns the list of reference type descriptors (id, regex pattern, i18n label key) the search endpoint accepts. Used at startup to understand which input formats the upstream recognises. Response is a JSON array; no query parameters required.

**Observed status codes:** 200

---

### 2. Shipment search / summary

```
GET https://mydsv.dsv.com/nges-portal/api/public/tracking-public/shipments?query=<reference>
```

Accepts any reference string (STT, waybill number, booking ID, etc.) and returns a summary list. Response shape:

```json
{
  "result": [ { "id": "LandStt:LKG6022524:CTTS:LAND", "stt": "...", ... } ],
  "warnings": []
}
```

Each result item contains: `id` (the composite `shipmentId` used as the detail-endpoint path parameter), `stt`, `transportMode`, `percentageProgress`, `lastEventCode`, `fromLocation`, `toLocation`, `startDate`, `endDate` (nullable), `consignment` (nullable), `additionalReferenceValues` (nullable), `isXpress`, `swedenViewAvailable`.

**Observed status codes:** 200 (even for not-found — returns `{"result":[],"warnings":[]}` when no match), 4xx with error body when the reference is structurally invalid (see Error contract).

---

### 3. Shipment detail

```
GET https://mydsv.dsv.com/nges-portal/api/public/tracking-public/shipments/{transportMode}/{shipmentId}
```

- `transportMode` is the **lowercase** transport mode: `land` (only mode observed in samples).
- `shipmentId` is the composite identifier returned by the search endpoint, e.g. `LandStt:VAN5022058:CTTS:LAND`. It must be URL-encoded when used as a path segment.

Returns the full shipment object (events, packages, locations, goods, progress). See field inventory below.

**Observed status codes:** 200, 4xx with `TRACKING-BADREQ-SHIPMENT_NOT_FOUND` (see Error contract).

---

## URL Construction Insight

The public tracking UI accepts a bare reference via:

```
https://mydsv.dsv.com/app/tracking-public/?refNumber=<STT>
```

The UI resolves the reference to an STT and then builds its state from there. An MCP response can include a "view in DSV" link by constructing this URL from the resolved `sttNumber` field in the detail response — not from the raw user input.

---

## Reference Types

21 reference types discovered from the reference-types endpoint. Patterns are intentionally permissive; a given reference string typically matches several types simultaneously — the search endpoint resolves ambiguity server-side.

| ID | Human-readable label | Notes |
|----|----------------------|-------|
| Stt | Shipment tracking number | Primary identifier; STT prefix encodes origin terminal |
| WaybillNo | Waybill number | Numeric string, seen as `waybillAndConsignementNumbers` in detail response |
| ShippersRefNo | Shipper's reference number | |
| PackageId | Package ID | Long numeric string (e.g. `573313432229014382`) |
| ConsigneesRefNo | Consignee's reference number | |
| Hawb | House air waybill | Air-mode; not observed in LAND samples |
| BookingId | Booking ID | |
| CustomerBookingRef | Customer booking reference | |
| PurchaseOrderNo | Purchase order number | |
| Hbl | House bill of lading | Sea-mode; not observed in LAND samples |
| ContainerNo | Container number | Sea-mode; not observed in LAND samples |
| ATOL | ATOL reference | |
| COS | COS reference | |
| ShippingOrderNumber | Shipping order number | |
| MovementReferenceNumber | Movement reference number | |
| ShipmentNoExportImport | Shipment number (export/import) | |
| SalesOrderNumber | Sales order number | |
| DeliveryOrderNumber | Delivery order number | |
| PackageNumber | Package number | |
| AssetId | Asset ID | |
| JobId | Job ID | |

---

## Observed Response Shape — Field Presence Matrix

Based on the 7 detail-endpoint samples.

### Always populated

| Field | Notes |
|-------|-------|
| `sttNumber` | Terminal-prefixed STT string |
| `shipmentId` | Composite key: `LandStt:<STT>:CTTS:LAND` |
| `transportMode` | Always `"LAND"` in observed samples |
| `dataProvider` | Always `"CTTS"` in observed samples |
| `product` | `"DSV LTL"` or `"DSVparcel"` |
| `goods.pieces` | Integer count of pieces |
| `goods.volume` | `{value, unit:"CBM"}` |
| `goods.weight` | `{value, unit:"KGS"}` |
| `goods.loadingMeters` | `{value, unit:"MTR"}` — zero for parcel |
| `events[]` | At least one event; minimum is a single `ENT` (Booked) event |
| `packages[]` | At least one package object with `id`; `events` array present but may be empty |
| `location.collectFrom` | Commercial address with `countryCode`, `country`, `city`, `postCode` |
| `location.deliverTo` | Same shape as `collectFrom` |
| `location.shipperPlace` | Same shape; matches `collectFrom` in all observed samples |
| `location.consigneePlace` | Same shape; matches `deliverTo` in all observed samples except the postcode-mismatch fixture |
| `location.dispatchingOffice` | DSV hub: `countryCode`, `country`, `city` — **no postCode** |
| `location.receivingOffice` | Same shape as `dispatchingOffice` |
| `progressBar` | `{steps: [...], activeStep: "..."}` |
| `percentageProgress` | Integer 0–100 |
| `isXpress` | Boolean; `false` in all observed samples |

### Sometimes populated

| Field | Condition |
|-------|-----------|
| `references.shipper` | Present for some shipments; empty array `[]` for others |
| `references.consignee` | Present for some shipments; empty array `[]` for others |
| `references.waybillAndConsignementNumbers` | Note upstream typo: `Consignement` (missing 's'). Always an array; one or two entries observed |
| `combiterms` | Set for `DSV LTL` freight (e.g. `"Delivered buyer's premises Duty Unpaid"`); `null` for `DSVparcel` |
| `deliveryDate.estimated` | Populated once a route is planned; `null` when only booked |
| `packages[].events` | Empty array `[]` when shipment not yet collected; populated once movement begins |
| event `reasons[]` | Non-empty only on `DIS` events; observed value: `[{code:"PA", description:"Pre-advice initiated"}]` |

### Never populated in observed data

| Field | Notes |
|-------|-------|
| `goods.dimensions[]` | Always `[]` |
| `goods.stackable` | Always `null` |
| `goods.chargeableWeight` | Always `null` |
| `goods.agreementDangerousRoad` | Always `null` |
| `goods.customsDuty` | Always `null` |
| `service` | Always `null` |
| `services[]` | Always `[]` |
| `deliveryDate.agreed` | Always `null` |
| `transportUnits` | Always `null` |
| `references.originalStt` | Always `null` |
| `references.additionalReferences[]` | Always `[]` |
| event `recipient` | Always `null` |
| event `shellIconName` | Always `null` |

---

## Event Codes Observed

| Code | Comment seen | Description |
|------|-------------|-------------|
| `ENT` | Booked | Shipment registered in system |
| `COL` | Collected | Picked up from sender |
| `ENM` | Arrived | Arrived at a DSV hub or terminal |
| `MAN` | Departed | Departed from a hub |
| `DOT` | Out for Delivery | On final delivery vehicle |
| `DLV` | Delivered | Delivered to recipient |
| `NLO` | *(no comment seen)* | Internal load event; appears only at package level, not shipment level |
| `DIS` | To Consignee's Disposal | Available for collection; the only code observed with non-empty `reasons[]` |

The `comment` field is user-facing English prose and is **not** a stable API contract. The adapter should maintain its own enum keyed on `code`, with an `Unknown` fallback for codes outside the observed set.

`NLO` was observed only in **package-level** event arrays (`packages[].events`) across all 7 fixtures. The shipment-level `events[]` array never contained `NLO` in observed data, though the type system permits it.

---

## Event Array Ordering

All 7 observed fixtures have their `events[]` arrays sorted by `date` ascending. The upstream's API contract makes no explicit ordering guarantee, however, so this may not hold for all shipments or future responses.

The mapper sorts `events[]` defensively before constructing a `Shipment`, ensuring consumers always receive a stable chronological contract regardless of what the upstream delivers.

**Semantic vs. positional ordering:** event positions in observed data reflect ingestion order; `date` timestamps reflect physical event time. In `dispatching_parcel_se_se.json` (STT `SEKSD620143489`) these coincide — `ENT` (Booked, 15:24) sits in array position 3, after `COL` (14:00) and `ENM` (14:01) — but the timestamps tell a semantically surprising story: the booking record was created 84 minutes after the parcel was physically collected. This is a real-world operational pattern (driver scans before booking is entered), not a data error. Consumers must not infer operational sequence from array position.

---

## Progress Stages Observed

| Stage | `percentageProgress` | Meaning |
|-------|---------------------|---------|
| `BOOKED` | 16% | Shipment registered, not yet collected |
| `DISPATCHING_CENTER` | 66% | At a DSV hub, pre-advice or processing underway |
| `IN_DELIVERY` | 83% | Out for delivery |
| `DELIVERED` | 100% | Delivered |

`progressBar.steps` always contains 5 values in order: `["BOOKED","TRANSPORTATION","DISPATCHING_CENTER","IN_DELIVERY","DELIVERED"]`. The `TRANSPORTATION` stage was not observed as an `activeStep` in any sample. Percentage and stage are not strictly monotonic — treat them as independent fields.

---

## Products Observed

| Product | Characteristics |
|---------|----------------|
| `DSV LTL` | Heavy freight; `combiterms` set; `loadingMeters` > 0; `goods.weight` in hundreds/thousands of KGS |
| `DSVparcel` | Small parcel; `combiterms` null; `loadingMeters` = 0.0 |

---

## Cache-Key Insight

Multiple input references can resolve to the same shipment. The adapter's cache key must therefore be the resolved `shipmentId` (e.g. `LandStt:LKG6022524:CTTS:LAND`), not the raw input reference. The reference→`shipmentId` resolution (search endpoint) is a cheaper, separately cacheable lookup.

**In-sample evidence — multiple waybills per shipment:** `delivered_ltl_se_fr.json` (STT `VAN5022058`) has 2 packages and 2 distinct waybill numbers (`1806290829` and `03368220`). Both waybills route to the same shipment via the search endpoint.

**In-sample evidence — waybill count does not track package count:** `booked_ltl_se_dk.json` has 3 packages but only 1 waybill (`3476238161`), and `booked_parcel_se_se_three_packages.json` likewise has 3 packages and 1 waybill (`3476265248`). The relationship is one waybill per booking, with packages as components of a single booking. Multiple waybills appear on a shipment when separate bookings are consolidated — not because there are multiple packages.

**Manual-testing evidence:** Querying the search endpoint with waybill `3476236157` and separately with `3476238161` both returned `shipmentId = LandStt:LKG6022524:CTTS:LAND`. This confirms the resolution collapse that the static fixtures illustrate with different waybill-pair examples.

---

## Location Field Semantics

`collectFrom`, `deliverTo`, `shipperPlace`, and `consigneePlace` are commercial addresses with `postCode`. In all observed samples `collectFrom == shipperPlace` and `deliverTo == consigneePlace`, but the `postCode` values of `deliverTo` and `consigneePlace` can differ within the same city — observed in `booked_parcel_se_se_postcode_mismatch.json` where `deliverTo.postCode = "82455"` and `consigneePlace.postCode = "82450"` both in Hudiksvall, SE. Consumers should not assume the two pairs are identical.

`dispatchingOffice` and `receivingOffice` are DSV facility addresses. They have `countryCode`, `country`, and `city` but **no `postCode` field**.

---

## Error Contract

Observed not-found response body (HTTP 4xx):

```json
{
  "message": "Shipment not found [reference=18062908291]",
  "code": "TRACKING-BADREQ-SHIPMENT_NOT_FOUND"
}
```

The `code` field is the stable machine-readable contract. The `message` is human-readable prose that embeds the input reference and must not be parsed. The adapter maps `code` values to typed sentinel errors; unknown codes should surface as a generic upstream error.

---

## Anti-bot protection

### What Cap.js is

Every endpoint under `/nges-portal/api/public/tracking-public/` is gated by Cap.js — a combined SHA-256 proof-of-work and browser-instrumentation challenge. Unlike a simple CAPTCHA, Cap.js couples a PoW puzzle with deep browser environment checks: it validates the presence of real browser APIs (DOM, canvas, WebGL), inspects `navigator` properties, and detects headless/automated browser signatures. A plain HTTP client gets an immediate 429; a naive headless browser gets detected and also fails.

### What we tried first

The initial approach was a standard retrying HTTP client (exponential backoff, up to 3 attempts). This worked on the first call (before Cap.js activated) but reliably failed from the second request onward. Every retry returned 429 regardless of wait time because Cap.js had flagged the client as non-browser traffic.

### Why a headless browser

Running a real Chromium instance via [`chromedp`](https://github.com/chromedp/chromedp) means Cap.js runs inside a genuine V8 JavaScript environment with all the browser APIs it checks. The proof-of-work is solved by the page's own JavaScript — we never touch the PoW algorithm directly. This is slower at cold-start (5–10 s for the first request) and consumes more memory (~150 MB for the Chrome process), but it is the correct approach: the browser behaves exactly as a real user's browser would.

### The `navigator.webdriver` detection problem

The first headless Chrome attempt still got detected. The cause: `chromedp`'s `DefaultExecAllocatorOptions` includes `--enable-automation`, which sets `navigator.webdriver = true` in JavaScript. Cap.js checks this property explicitly — any browser where `navigator.webdriver` is truthy is treated as automated.

**Fix:** build the Chrome allocator option list from scratch rather than extending the defaults. Never include `--enable-automation`. Add `--disable-blink-features=AutomationControlled` to suppress the webdriver property:

```go
opts := []chromedp.ExecAllocatorOption{
    chromedp.NoFirstRun,
    chromedp.NoDefaultBrowserCheck,
    chromedp.Flag("disable-blink-features", "AutomationControlled"),
    chromedp.Flag("headless", "new"),
    // ... other flags — but NOT chromedp.Flag("enable-automation", ...)
}
allocCtx, _ := chromedp.NewExecAllocator(ctx, opts...)
```

This makes the headless browser indistinguishable from a regular Chrome window on the properties Cap.js inspects.

### The `GetResponseBody` "invalid context" problem

After fixing detection, a second bug appeared: `network.GetResponseBody` returned an "invalid context" error when called from the `EventLoadingFinished` callback. The cause: Chrome dispatches network events asynchronously; by the time the `LoadingFinished` event fires, the tab's context may be mid-redirect, making the chromedp context stale.

**Fix:** capture the raw CDP `Target` executor synchronously inside the event handler goroutine (before any goroutine launch), then wrap it in a fresh `context.Background()`:

```go
chromedpCtx := chromedp.FromContext(tabCtx)
executor := cdp.WithExecutor(context.Background(), chromedpCtx.Target)

go func(reqID network.RequestID) {
    body, err := network.GetResponseBody(reqID).Do(executor)
    // ...
}(requestID)
```

`cdp.WithExecutor` bypasses the chromedp context lifecycle entirely and queries the tab directly. This is robust to redirects and tab lifecycle transitions.

### Session amortisation

Because the Chrome process is a long-lived singleton (shared `browserCtx`), Cap.js cookies and session tokens persist across all tab-level calls. The PoW challenge is solved once on the first request; every subsequent request in the same process lifetime reuses the solved session state. Cold-start latency is paid once per server restart.

Source: `internal/upstream/dsv/browser/browser.go`

---

## MCP Tool Surfaces

Three tools are registered by the server (Phase 4). Each returns a freshness tag and a retrieved_at timestamp.

### `track_shipment`
- **Input**: `reference` (string, required); `reference_type` (string, optional — one of the 21 codes from `list_reference_types`)
- **Output**: `{ shipments: [ShipmentSummaryView], freshness, retrieved_at }`
- **Error codes**: `INVALID_INPUT`, `INVALID_REFERENCE_TYPE`, `UPSTREAM_ERROR`
- **Cache**: search cache, key = `lower(reference)|reference_type`, TTL 60 s

### `get_shipment_details`
- **Input**: `shipment_id` (string, required — composite form `Provider:Ref:DataProvider:Mode`)
- **Output**: `{ shipment: ShipmentDetailView, freshness, retrieved_at }`
- **Error codes**: `INVALID_SHIPMENT_ID`, `SHIPMENT_NOT_FOUND`, `UPSTREAM_ERROR`
- **Cache**: detail cache, key = `shipment_id`, TTL 30 s

### `list_reference_types`
- **Input**: none
- **Output**: `{ reference_types: [{ code, label, pattern }], freshness: "static" }`
- **No upstream call** — returns the bundled `internal/data/reference_types.json`

---

## Known Gaps (not implemented in v1)

- SEA, AIR, and RAIL transport modes — only `LAND` samples observed; endpoint URL pattern supports other modes via the `{transportMode}` path segment.
- Data providers other than `CTTS` — `dataProvider` may take other values for non-CTTS shipments.
- Populated `goods.dimensions[]`, `services[]`, or `transportUnits` — structure is present in the schema but never populated in observed data.
- Authenticated endpoints — DSV's public API never returns party names (sender company, recipient company); those require authentication not explored in this recon.
