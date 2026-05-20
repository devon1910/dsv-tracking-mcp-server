package domain

import (
	"fmt"
	"strings"
	"time"
)

// MapShipmentSummaryView projects a ShipmentSummary to a ShipmentSummaryView.
func MapShipmentSummaryView(s ShipmentSummary) ShipmentSummaryView {
	var startDate, endDate string
	if s.StartDate != nil {
		startDate = s.StartDate.UTC().Format(time.RFC3339)
	}
	if s.EndDate != nil {
		endDate = s.EndDate.UTC().Format(time.RFC3339)
	}
	return ShipmentSummaryView{
		ShipmentID:    s.ShipmentID,
		STT:           s.STTNumber,
		Reference:     s.STTNumber,
		Status:        s.LastEventCode.Description(),
		LastEventCode: s.LastEventRawCode,
		TransportMode: s.TransportMode.String(),
		DataProvider:  dataProviderFromShipmentID(s.ShipmentID),
		Progress:      s.PercentageProgress,
		FromLocation:  s.FromLocation,
		ToLocation:    s.ToLocation,
		StartDate:     startDate,
		EndDate:       endDate,
	}
}

// MapShipmentDetailView projects a Shipment to a ShipmentDetailView.
// Events must arrive sorted; the mapper does not re-sort.
func MapShipmentDetailView(s Shipment) ShipmentDetailView {
	events := make([]EventView, len(s.Events))
	for i, e := range s.Events {
		events[i] = mapEventView(e)
	}

	return ShipmentDetailView{
		ShipmentID:        s.ShipmentID,
		Reference:         s.STTNumber,
		Status:            s.Progress.ActiveStep.Description(),
		TransportMode:     s.TransportMode.String(),
		DataProvider:      s.DataProvider,
		ShipperPlace:      mapPlaceView(s.Locations.ShipperPlace),
		ConsigneePlace:    mapPlaceView(s.Locations.ConsigneePlace),
		CollectFrom:       mapPlaceView(s.Locations.CollectFrom),
		DeliverTo:         mapPlaceView(s.Locations.DeliverTo),
		DispatchingOffice: mapOfficeView(s.Locations.DispatchingOffice),
		Events:            events,
		Packages:          mapPackagesView(s.Packages),
		Goods:             mapGoodsView(s.Goods),
		ViewInUIURL:       viewInUIURL(s.ShipmentID),
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

func mapPackagesView(pkgs []Package) []PackageView {
	views := make([]PackageView, len(pkgs))
	for i, p := range pkgs {
		evts := make([]PackageEventView, len(p.Events))
		for j, e := range p.Events {
			evts[j] = PackageEventView{
				Date:        e.Date.UTC().Format(time.RFC3339),
				Code:        string(e.Code),
				RawCode:     e.RawCode,
				Location:    e.Location,
				CountryCode: e.CountryCode,
			}
		}
		views[i] = PackageView{ID: p.ID, Events: evts}
	}
	return views
}

func mapMeasurementView(m Measurement) *MeasurementView {
	if m.Unit == "" {
		return nil
	}
	return &MeasurementView{Value: m.Value, Unit: m.Unit}
}

func mapMeasurementPtrView(m *Measurement) *MeasurementView {
	if m == nil {
		return nil
	}
	return mapMeasurementView(*m)
}

func mapGoodsView(g Goods) *GoodsView {
	var dims []DimensionView
	for _, d := range g.Dimensions {
		dv := DimensionView{
			Length: mapMeasurementPtrView(d.Length),
			Width:  mapMeasurementPtrView(d.Width),
			Height: mapMeasurementPtrView(d.Height),
		}
		dims = append(dims, dv)
	}
	return &GoodsView{
		Pieces:        g.Pieces,
		Weight:        mapMeasurementView(g.Weight),
		Volume:        mapMeasurementView(g.Volume),
		LoadingMeters: mapMeasurementView(g.LoadingMeters),
		Dimensions:    dims,
	}
}

func mapPlaceView(p Place) *LocationView {
	if p.City == "" && p.CountryCode == "" && p.PostCode == "" {
		return nil
	}
	return &LocationView{
		PostCode:    p.PostCode,
		City:        p.City,
		CountryCode: p.CountryCode,
		Country:     p.Country,
	}
}

func mapOfficeView(o Office) *LocationView {
	if o.City == "" && o.CountryCode == "" {
		return nil
	}
	return &LocationView{
		City:        o.City,
		CountryCode: o.CountryCode,
		Country:     o.Country,
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
