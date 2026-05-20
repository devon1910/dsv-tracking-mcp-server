# ADR 0002 — Party names omitted from domain model

**Status**: Accepted  
**Date**: 2026-05-20

## Context

DSV's public tracking API (`/api/public/tracking-public/shipments/land/...`)
returns party records that contain addresses (city, country, postal code) but
**no name field** for shipper, consignee, or notify party. The authenticated
DSV portal exposes party names, but that API requires a customer login and is
outside scope.

Options considered:

1. **Omit the `Name` field entirely** from `domain.Party` and the outbound
   JSON. Callers receive addresses only.
2. **Include `Name` as `*string`** (nullable). The field is always `nil` for
   data originating from the public API but can be populated if a future
   integration supplies names.
3. **Include `Name` as `string`** with a sentinel like `"(not available)"`.
   Cleaner JSON but misleads callers into thinking a name is always expected.

## Decision

Option 2: include `Name *string` in `domain.Party`. Rationale:

- Option 1 forces a breaking schema change the day we add an authenticated
  source; option 2 is additive.
- Option 3 pollutes downstream text with placeholder strings that LLMs may
  repeat verbatim.
- The MCP tool description (`get_shipment_details`) explicitly notes "addresses
  but not names", so callers are informed at the API contract level.

## Consequences

- `domain.Party.Name` is always `nil` in practice today. Code that assumes it
  is non-nil will panic; call sites must nil-check or use the view mapper which
  handles this.
- If a future authenticated source populates `Name`, no schema change is
  required on the MCP wire format.
