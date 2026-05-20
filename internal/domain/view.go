package domain

// View types are the outbound projections sent over MCP.
// They are deliberately simpler than the canonical domain types and are
// designed to be consumed by LLMs rather than by application code.
//
// Rule: never expose DTOs or raw upstream shapes through MCP;
// always project through these types.

// ShipmentSummaryView is returned by the track_shipment tool.
type ShipmentSummaryView struct {
	ShipmentID    string `json:"shipment_id"`
	STT           string `json:"stt"`
	Reference     string `json:"reference"`
	Status        string `json:"status"`
	LastEventCode string `json:"last_event_code"`
	TransportMode string `json:"transport_mode"`
	DataProvider  string `json:"data_provider"`
	Progress      int    `json:"progress"`
	FromLocation  string `json:"from_location,omitempty"`
	ToLocation    string `json:"to_location,omitempty"`
	StartDate     string `json:"start_date,omitempty"`
	EndDate       string `json:"end_date,omitempty"`
}

// MeasurementView is a numeric value with its unit.
type MeasurementView struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// DimensionView holds optional length/width/height for a single item.
type DimensionView struct {
	Length *MeasurementView `json:"length,omitempty"`
	Width  *MeasurementView `json:"width,omitempty"`
	Height *MeasurementView `json:"height,omitempty"`
}

// GoodsView summarises cargo weight, volume, and piece count.
type GoodsView struct {
	Pieces        int              `json:"pieces"`
	Weight        *MeasurementView `json:"weight,omitempty"`
	Volume        *MeasurementView `json:"volume,omitempty"`
	LoadingMeters *MeasurementView `json:"loading_meters,omitempty"`
	Dimensions    []DimensionView  `json:"dimensions,omitempty"`
}

// ShipmentDetailView is returned by the get_shipment_details tool.
type ShipmentDetailView struct {
	ShipmentID        string        `json:"shipment_id"`
	Reference         string        `json:"reference"`
	Status            string        `json:"status"`
	TransportMode     string        `json:"transport_mode"`
	DataProvider      string        `json:"data_provider"`
	ShipperPlace      *LocationView `json:"shipper_place,omitempty"`
	ConsigneePlace    *LocationView `json:"consignee_place,omitempty"`
	CollectFrom       *LocationView `json:"collect_from,omitempty"`
	DeliverTo         *LocationView `json:"deliver_to,omitempty"`
	DispatchingOffice *LocationView `json:"dispatching_office,omitempty"`
	Events            []EventView   `json:"events"`
	Packages          []PackageView `json:"packages"`
	Goods             *GoodsView    `json:"goods,omitempty"`
	ViewInUIURL       string        `json:"view_in_ui_url,omitempty"`
}

// LocationView represents a location at postcode/city/country level.
// DSV's public tracking endpoint does not expose party names or street
// addresses — only postCode, city, countryCode, and country are available.
type LocationView struct {
	PostCode    string `json:"post_code,omitempty"`
	City        string `json:"city,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	Country     string `json:"country,omitempty"`
}

// PartyView is kept for backwards-compatibility; no longer used by ShipmentDetailView.
type PartyView struct {
	Name    *string `json:"name,omitempty"`
	Address string  `json:"address,omitempty"`
	City    string  `json:"city,omitempty"`
	Country string  `json:"country,omitempty"`
}

// PackageEventView is a single tracking event at the individual-package level.
type PackageEventView struct {
	Date        string `json:"date"`
	Code        string `json:"code"`
	RawCode     string `json:"raw_code"`
	Location    string `json:"location,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
}

// PackageView holds per-package tracking history.
type PackageView struct {
	ID     string             `json:"id"`
	Events []PackageEventView `json:"events"`
}

// EventView is a single tracking event in a shipment's history.
type EventView struct {
	Date        string `json:"date"`         // RFC3339, UTC
	Code        string `json:"code"`         // domain EventCode string, e.g. "ENT", "DLV"
	RawCode     string `json:"raw_code"`     // preserved upstream string; same as Code unless unknown
	Description string `json:"description"`  // stable human label from the domain enum
	Location    string `json:"location,omitempty"`
}

// ReferenceTypeView is one entry returned by list_reference_types.
type ReferenceTypeView struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	Pattern string `json:"pattern"`
}
