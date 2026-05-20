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
	Reference     string `json:"reference"`
	Status        string `json:"status"`
	TransportMode string `json:"transport_mode"`
	DataProvider  string `json:"data_provider"`
}

// ShipmentDetailView is returned by the get_shipment_details tool.
type ShipmentDetailView struct {
	ShipmentID    string      `json:"shipment_id"`
	Reference     string      `json:"reference"`
	Status        string      `json:"status"`
	TransportMode string      `json:"transport_mode"`
	DataProvider  string      `json:"data_provider"`
	Sender        *PartyView  `json:"sender,omitempty"`
	Receiver      *PartyView  `json:"receiver,omitempty"`
	Events        []EventView `json:"events"`
	ViewInUIURL   string      `json:"view_in_ui_url,omitempty"`
}

// PartyView represents a sender or receiver.
//
// Name is intentionally nullable: DSV's public tracking API exposes addresses
// but never party names. Party names are only available through authenticated
// DSV API endpoints that are out of scope for v1. Do not synthesise a name
// from other fields — leave it nil so callers know the gap is real.
type PartyView struct {
	Name    *string `json:"name,omitempty"`
	Address string  `json:"address,omitempty"`
	City    string  `json:"city,omitempty"`
	Country string  `json:"country,omitempty"`
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
