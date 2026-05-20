# ADR 0002 — Location shape matches public surface, not freight semantics

**Status**: Accepted  
**Date**: 2026-05-20 (revised 2026-05-20)

## Context

DSV's tracking-public endpoint (`mydsv.dsv.com/app/tracking-public/`) is a
deliberately privacy-limited surface. It exposes shipment-level location data
under `shipperPlace`, `consigneePlace`, `collectFrom`, `deliverTo`, and
`dispatchingOffice` — all at postcode/city/country level. It never exposes
party names (shipper company, consignee company) or street addresses.

The Sendify challenge spec lists "sender/receiver name and address" as required
output. This data is not available from the named source. DSV's authenticated
APIs may expose it; they are out of scope for a public-tracking challenge.

Direct verification: response bodies from `GET .../shipments/land/<id>` contain
`location.shipperPlace.postCode`, `.city`, `.countryCode`, `.country` — and
nothing else. See `docs/evidence/sample_segot620304613.json` for the canonical
shape.

## Decision

Model the response shape after the upstream's actual fields. Use `LocationView`
(postCode/city/countryCode/country) rather than `PartyView` (name/address).
Surface five named locations that match the upstream's own vocabulary:
`ShipperPlace`, `ConsigneePlace`, `CollectFrom`, `DeliverTo`,
`DispatchingOffice`.

## Consequences

- The MCP response is honest about what `tracking-public` provides. LLM callers
  cannot promise users a street address or company name that the API doesn't
  supply.
- `PartyView`, `Sender`, and `Receiver` no longer exist in the view layer.
  The `Party` domain type and `Sender()`/`Receiver()` helpers remain (they are
  not wrong, just unused by the view).
- Adding authenticated-tier support later requires a new `PartyView` type
  alongside `LocationView`, not a rename — no breaking change to the current
  wire format.
