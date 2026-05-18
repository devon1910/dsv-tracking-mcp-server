package domain

// Party represents either the sender or receiver of a shipment.
//
// Name is always nil when using DSV's public tracking API: the public endpoints
// never return party names (company name, contact name). Authenticated DSV
// endpoints would populate Name; the field is present here so the type can
// accommodate that data if the adapter gains authenticated access in a future
// version. Callers must treat nil Name as "name not available" rather than
// "no name exists".
type Party struct {
	Name       *string  // always nil via DSV public API — see above
	Address    Place
	References []string // shipper or consignee reference numbers from the shipment
}

// Sender synthesises the sender Party from the shipment's locations and
// shipper references.
func (s Shipment) Sender() Party {
	return Party{
		Name:       nil,
		Address:    s.Locations.ShipperPlace,
		References: s.References.Shipper,
	}
}

// Receiver synthesises the receiver Party from the shipment's locations and
// consignee references.
func (s Shipment) Receiver() Party {
	return Party{
		Name:       nil,
		Address:    s.Locations.ConsigneePlace,
		References: s.References.Consignee,
	}
}
