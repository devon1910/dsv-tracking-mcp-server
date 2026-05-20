package domain

import (
	"fmt"
	"strings"
	"time"
)

// MapShipmentSummaryView projects a ShipmentSummary to a ShipmentSummaryView.
func MapShipmentSummaryView(s ShipmentSummary) ShipmentSummaryView {
	return ShipmentSummaryView{
		ShipmentID:    s.ShipmentID,
		Reference:     s.STTNumber,
		Status:        s.LastEventCode.Description(),
		TransportMode: s.TransportMode.String(),
		DataProvider:  dataProviderFromShipmentID(s.ShipmentID),
	}
}

// MapShipmentDetailView projects a Shipment to a ShipmentDetailView.
// Events must arrive sorted; the mapper does not re-sort.
func MapShipmentDetailView(s Shipment) ShipmentDetailView {
	events := make([]EventView, len(s.Events))
	for i, e := range s.Events {
		events[i] = mapEventView(e)
	}

	sender := mapPartyView(s.Sender())
	receiver := mapPartyView(s.Receiver())

	return ShipmentDetailView{
		ShipmentID:    s.ShipmentID,
		Reference:     s.STTNumber,
		Status:        s.Progress.ActiveStep.Description(),
		TransportMode: s.TransportMode.String(),
		DataProvider:  s.DataProvider,
		Sender:        &sender,
		Receiver:      &receiver,
		Events:        events,
		ViewInUIURL:   viewInUIURL(s.ShipmentID),
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func mapEventView(e Event) EventView {
	loc := e.Location.Name
	if e.Location.CountryCode != "" {
		loc = fmt.Sprintf("%s (%s)", e.Location.Name, e.Location.CountryCode)
	}
	return EventView{
		Date:        e.Date.UTC().Format(time.RFC3339),
		Code:        string(e.Code),
		RawCode:     e.RawCode,
		Description: e.Code.Description(),
		Location:    loc,
	}
}

func mapPartyView(p Party) PartyView {
	addr := strings.TrimSpace(p.Address.PostCode + " " + p.Address.City)
	return PartyView{
		Name:    p.Name, // always nil via DSV public API
		Address: addr,
		City:    p.Address.City,
		Country: p.Address.Country,
	}
}

// viewInUIURL synthesises the DSV web-UI tracking URL from a composite
// shipmentID (e.g. "LandStt:VAN5022058:CTTS:LAND"). Returns empty string
// if the shipmentID is malformed so callers omit the field rather than
// emit a broken URL.
func viewInUIURL(shipmentID string) string {
	parts := strings.Split(shipmentID, ":")
	if len(parts) < 2 || parts[1] == "" {
		return ""
	}
	stt := parts[1]
	return "https://mydsv.dsv.com/app/tracking-public/?refNumber=" + stt
}

// dataProviderFromShipmentID parses the data provider segment from a
// composite shipmentID ("LandStt:REF:CTTS:LAND" → "CTTS").
// Returns "CTTS" as the default when parsing fails.
func dataProviderFromShipmentID(shipmentID string) string {
	parts := strings.Split(shipmentID, ":")
	if len(parts) >= 3 && parts[2] != "" {
		return parts[2]
	}
	return "CTTS"
}
