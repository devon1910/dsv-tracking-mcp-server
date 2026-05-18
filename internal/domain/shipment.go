// Package domain contains the core types that the rest of the system speaks in.
// These types are designed for consumers (MCP tools and the cache layer), not
// as a faithful mirror of the upstream wire format. The DTO and mapper in
// internal/upstream/dsv/ perform translation from the upstream JSON shape.
package domain

import "time"

// Shipment represents a DSV tracked shipment.
//
// Field-presence rules follow UPSTREAM.md:
//   - Always-populated upstream fields → value types.
//   - Sometimes-populated fields → pointer types or empty slices.
//   - Never-populated fields in observed data are omitted entirely; see
//     UPSTREAM.md "Never populated in observed data" for the full list.
type Shipment struct {
	STTNumber          string
	ShipmentID         string // composite key, e.g. "LandStt:VAN5022058:CTTS:LAND"
	TransportMode      TransportMode
	DataProvider       string // "CTTS" in all observed data
	Product            string // "DSV LTL", "DSVparcel", etc.
	IsXpress           bool
	PercentageProgress int

	References   References
	Goods        Goods
	Events       []Event  // sorted chronologically by Date; see sort.go
	Packages     []Package
	Locations    Locations
	DeliveryDate DeliveryDate
	Progress     Progress
	Combiterms   *string // nil for DSVparcel; set for DSV LTL freight
}

// References holds customer-supplied and system-generated reference numbers.
//
// Note: the upstream JSON key "waybillAndConsignementNumbers" contains a typo
// ("Consignement" instead of "Consignment"). The domain uses correct spelling.
type References struct {
	Shipper                      []string
	Consignee                    []string
	WaybillAndConsignmentNumbers []string
}

// Goods describes the cargo dimensions and weight.
//
// Omitted upstream fields (never populated in observed data):
// dimensions, stackable, chargeableWeight, agreementDangerousRoad, customsDuty.
type Goods struct {
	Pieces        int
	Weight        Measurement
	Volume        Measurement
	LoadingMeters Measurement // zero for DSVparcel shipments
}

// Measurement pairs a numeric value with its unit string.
type Measurement struct {
	Value float64
	Unit  string
}

// Event is a single shipment-level tracking event.
// The Events slice on Shipment is always delivered sorted by Date ascending.
// See UPSTREAM.md "Event Array Ordering" for why sorting is required.
//
// Omitted upstream fields (never populated in observed data): recipient, shellIconName.
type Event struct {
	Code      EventCode
	RawCode   string        // preserved for forward compatibility when Code is Unknown
	Date      time.Time
	CreatedAt time.Time
	Location  EventLocation
	Comment   string        // upstream's English label; not a stable API contract
	Reasons   []EventReason // non-empty only on DIS events in observed data
}

// EventLocation is the location attached to a shipment-level event.
type EventLocation struct {
	Name        string
	Code        string // DSV internal hub code, e.g. "VAN", "GOT" — not IATA
	CountryCode string
}

// EventReason is an additional qualifier on an event.
// Observed only on DIS (To Consignee's Disposal) events.
type EventReason struct {
	Code        string
	Description string
}

// Package is a physical unit within a shipment.
type Package struct {
	ID     string
	Events []PackageEvent // empty until the shipment starts moving
}

// PackageEvent is a tracking event at the individual-package level.
// Package events carry a flat location string rather than the nested
// EventLocation used by shipment-level events.
type PackageEvent struct {
	Code        EventCode
	RawCode     string
	Location    string // flat string, e.g. "Värnamo"
	CountryCode string
	Date        time.Time
}

// Locations holds the commercial and operational addresses for a shipment.
type Locations struct {
	CollectFrom       Place  // commercial pickup address
	DeliverTo         Place  // commercial delivery address
	ShipperPlace      Place  // shipper's registered address; matches CollectFrom in observed data
	ConsigneePlace    Place  // consignee's registered address; may differ from DeliverTo postcode
	DispatchingOffice Office // DSV origin hub
	ReceivingOffice   Office // DSV destination hub
}

// Place is a commercial address that includes a post code.
type Place struct {
	CountryCode string
	Country     string
	City        string
	PostCode    string
}

// Office is a DSV facility address. It has no post code in the upstream
// response — see UPSTREAM.md "Location Field Semantics".
type Office struct {
	CountryCode string
	Country     string
	City        string
}

// DeliveryDate holds estimated and agreed delivery dates.
// Agreed is present in the upstream schema but never populated in observed data.
type DeliveryDate struct {
	Estimated *time.Time
	Agreed    *time.Time // schema-present; always nil in observed data
}

// Progress describes where the shipment is in its lifecycle stages.
type Progress struct {
	Steps      []ProgressStage
	ActiveStep ProgressStage
}
