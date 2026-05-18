package dsv

import (
	"fmt"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
)

// MapShipmentDetail converts a ShipmentDetailDTO into a domain.Shipment.
// Events are sorted chronologically before returning.
// Unknown event codes, transport modes, and progress stages are mapped to their
// respective Unknown constants; the raw upstream string is preserved in RawCode.
// Returns a *domain.UpstreamError wrapping domain.ErrMalformedResponse if
// required fields are structurally absent.
func MapShipmentDetail(dto *ShipmentDetailDTO) (domain.Shipment, error) {
	if dto.ProgressBar == nil {
		return domain.Shipment{}, &domain.UpstreamError{
			Op:           "map_shipment_detail",
			UpstreamCode: "",
			HTTPStatus:   0,
			Err:          fmt.Errorf("progressBar is nil: %w", domain.ErrMalformedResponse),
		}
	}

	events := make([]domain.Event, len(dto.Events))
	for i, e := range dto.Events {
		reasons := make([]domain.EventReason, len(e.Reasons))
		for j, r := range e.Reasons {
			reasons[j] = domain.EventReason{Code: r.Code, Description: r.Description}
		}
		events[i] = domain.Event{
			Code:      domain.ParseEventCode(e.Code),
			RawCode:   e.Code,
			Date:      e.Date,
			CreatedAt: e.CreatedAt,
			Location: domain.EventLocation{
				Name:        e.Location.Name,
				Code:        e.Location.Code,
				CountryCode: e.Location.CountryCode,
			},
			Comment: e.Comment,
			Reasons: reasons,
		}
	}
	domain.SortEventsChronologically(events)

	packages := make([]domain.Package, len(dto.Packages))
	for i, p := range dto.Packages {
		pkgEvents := make([]domain.PackageEvent, len(p.Events))
		for j, pe := range p.Events {
			pkgEvents[j] = domain.PackageEvent{
				Code:        domain.ParseEventCode(pe.Code),
				RawCode:     pe.Code,
				Location:    pe.Location,
				CountryCode: pe.CountryCode,
				Date:        pe.Date,
			}
		}
		domain.SortPackageEventsChronologically(pkgEvents)
		packages[i] = domain.Package{ID: p.ID, Events: pkgEvents}
	}

	stages := make([]domain.ProgressStage, len(dto.ProgressBar.Steps))
	for i, s := range dto.ProgressBar.Steps {
		stages[i] = domain.ParseProgressStage(s)
	}

	loc := dto.Location
	return domain.Shipment{
		STTNumber:          dto.STTNumber,
		ShipmentID:         dto.ShipmentID,
		TransportMode:      domain.ParseTransportMode(dto.TransportMode),
		DataProvider:       dto.DataProvider,
		Product:            dto.Product,
		IsXpress:           dto.IsXpress,
		PercentageProgress: dto.PercentageProgress,
		Combiterms:         dto.Combiterms,
		References: domain.References{
			Shipper:                      nilToEmpty(dto.References.Shipper),
			Consignee:                    nilToEmpty(dto.References.Consignee),
			WaybillAndConsignmentNumbers: nilToEmpty(dto.References.WaybillAndConsignmentNumbers),
		},
		Goods: domain.Goods{
			Pieces:        dto.Goods.Pieces,
			Weight:        mapMeasurement(dto.Goods.Weight),
			Volume:        mapMeasurement(dto.Goods.Volume),
			LoadingMeters: mapMeasurement(dto.Goods.LoadingMeters),
		},
		Events:   events,
		Packages: packages,
		Locations: domain.Locations{
			CollectFrom:       mapPlace(loc.CollectFrom),
			DeliverTo:         mapPlace(loc.DeliverTo),
			ShipperPlace:      mapPlace(loc.ShipperPlace),
			ConsigneePlace:    mapPlace(loc.ConsigneePlace),
			DispatchingOffice: mapOffice(loc.DispatchingOffice),
			ReceivingOffice:   mapOffice(loc.ReceivingOffice),
		},
		DeliveryDate: domain.DeliveryDate{
			Estimated: dto.DeliveryDate.Estimated,
			Agreed:    dto.DeliveryDate.Agreed,
		},
		Progress: domain.Progress{
			Steps:      stages,
			ActiveStep: domain.ParseProgressStage(dto.ProgressBar.ActiveStep),
		},
	}, nil
}

// MapShipmentSummaries converts the search-endpoint result array to domain summaries.
// Returns an empty (non-nil) slice for an empty result.
func MapShipmentSummaries(dto *SearchResponseDTO) []domain.ShipmentSummary {
	result := make([]domain.ShipmentSummary, len(dto.Result))
	for i, s := range dto.Result {
		result[i] = domain.ShipmentSummary{
			ShipmentID:         s.ID,
			STTNumber:          s.Stt,
			TransportMode:      domain.ParseTransportMode(s.TransportMode),
			PercentageProgress: s.PercentageProgress,
			LastEventCode:      domain.ParseEventCode(s.LastEventCode),
			LastEventRawCode:   s.LastEventCode,
			FromLocation:       s.FromLocation,
			ToLocation:         s.ToLocation,
			StartDate:          s.StartDate,
			EndDate:            s.EndDate,
			IsXpress:           s.IsXpress,
		}
	}
	return result
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func mapMeasurement(dto measurementDTO) domain.Measurement {
	return domain.Measurement{Value: dto.Value, Unit: dto.Unit}
}

func mapPlace(dto placeDTO) domain.Place {
	return domain.Place{
		CountryCode: dto.CountryCode,
		Country:     dto.Country,
		City:        dto.City,
		PostCode:    dto.PostCode,
	}
}

func mapOffice(dto officeDTO) domain.Office {
	return domain.Office{
		CountryCode: dto.CountryCode,
		Country:     dto.Country,
		City:        dto.City,
	}
}

// nilToEmpty returns the slice as-is if non-nil, otherwise returns an empty
// (non-nil) slice. Domain slices must never be nil; empty is the correct zero.
func nilToEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
