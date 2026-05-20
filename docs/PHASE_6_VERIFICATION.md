# Phase 6 Verification — All 10 Reference Numbers

Run date: 2026-05-20  
Branch: feat/phase-6-spec-compliance  
Tool: `go run ./cmd/dsv-verify-all/`

All 10 waybill references from the Sendify challenge were resolved end-to-end:
`track_shipment` → `get_shipment_details`. No errors, no missing goods blocks,
no unexpected response shapes.

---

## Results

| Ref | Shipment ID | Progress | Last Code | From → To | Goods | Packages | Notes |
|-----|-------------|----------|-----------|-----------|-------|----------|-------|
| 3476472018 | LandStt:SEGOT620304613:CTTS:LAND | 100% | DLV | Kungälv → Fjärås | 1 pcs, 2.45 KGS | 1 pkg / 4 evts | Anchor case |
| 3476265230 | LandStt:SESTO620298048:CTTS:LAND | 100% | DLV | Järfälla → Hudiksvall | 1 pcs, 7.75 KGS | 1 pkg / 4 evts | |
| 3476265248 | LandStt:SESTO620298049:CTTS:LAND | 100% | DLV | Järfälla → Upplands väsby | 3 pcs, 30 KGS | 3 pkg / 12 evts | Multi-package: 3 pkgs × 4 evts each |
| 3476257542 | LandStt:SEKSD620143489:CTTS:LAND | 100% | DLV | Karlstad → Arvika | 1 pcs, 3.7 KGS | 1 pkg / 6 evts | |
| 3476238161 | LandStt:LKG6022524:CTTS:LAND | 16% | ENT | Linköping → Skive | 3 pcs, 5430 KGS | 3 pkg / 0 evts | Multi-package, booked: 3 pkgs, 0 events |
| 3476236157 | LandStt:SESOE620172194:CTTS:LAND | 100% | DLV | Norsborg → Växjö | 1 pcs, 0.8 KGS | 1 pkg / 4 evts | |
| 3476230325 | LandStt:SEBLE620159398:CTTS:LAND | 16% | ENT | Grängesberg → Garpenberg | 1 pcs, 2700 KGS | 1 pkg / 0 evts | Heavy freight, booked: 2.7 t, 0 pkg events |
| 3476219849 | LandStt:SELPI620605038:CTTS:LAND | 16% | ENT | Linköping → Valdemarsvik | 2 pcs, 2000 KGS | 2 pkg / 0 evts | Multi-package, booked: 2 pkgs, 0 events |
| 3476207869 | LandStt:SESTO620296939:CTTS:LAND | 83% | DOT | Järfälla → Ängelholm | 1 pcs, 3.9 KGS | 1 pkg / 4 evts | In delivery |
| 3476186295 | LandStt:SESTO620296604:CTTS:LAND | 100% | DLV | Järfälla → Floby | 1 pcs, 0.6 KGS | 1 pkg / 4 evts | |

**Column key**  
- Progress: `percentageProgress` (0 = booked, 100 = delivered)  
- Last Code: raw `lastEventCode` from search (ENT = booked, DLV = delivered, DOT = out for delivery)  
- Goods: pieces × weight (KGS)  
- Packages: count × total package-level event count

---

## Shape findings

**No surprises.** Every ref returned:

- A populated `goods` block (pieces, weight, volume, loading_meters). `dimensions` was empty (`[]`) on all 10 shipments — consistent with prior observations across all fixtures.
- A `packages` array with 1–3 entries. Package event counts range from 0 (booked, not yet collected) to 4–6 (in transit or delivered).
- All five location fields populated (shipper_place, consignee_place, collect_from, deliver_to, dispatching_office) with postcode + city + country. No street addresses or party names, as expected.
- Transport mode: `LAND` on all 10. No SEA/AIR/RAIL encountered in this set.

**Multi-package shape confirmed:** refs 3476265248 (3 pkgs × 4 evts), 3476238161 (3 pkgs × 0 evts), and 3476219849 (2 pkgs × 0 evts) all return correct `packages` arrays with events sorted ascending by date. The three-packages fixture in `testdata/` covers the booked-with-no-events case.

**Heavy freight (refs 3476230325, 3476219849):** weights of 2700 KGS and 2000 KGS. Same field shape as parcel shipments — no structural difference, just larger values. The `GoodsView.Weight.Value` field is `float64` and handles this without overflow or formatting issues (the verification harness formats with `%g`; the actual JSON output uses standard float64 encoding).

**Booked with 0 package events:** refs 3476238161, 3476230325, 3476219849 are at 16% / ENT (booked). Their `packages[*].events` arrays are empty `[]`. This is the correct behaviour — the mapper always returns a non-nil slice, and the view emits `"events": []`.

---

## Evidence

- `docs/evidence/sample_segot620304613.json` — canonical upstream detail response for ref 3476472018 (anchor case, STT SEGOT620304613).
- The 7 fixture files in `testdata/` cover all observed shapes: booked (single/multi-package), dispatching, in-delivery, and delivered; parcel and LTL freight.
