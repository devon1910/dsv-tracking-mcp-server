package domain_test

import (
	"testing"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
)

func baseShipment() domain.Shipment {
	return domain.Shipment{
		References: domain.References{
			Shipper:   []string{"J11709 / J11709"},
			Consignee: []string{"8156735"},
		},
		Locations: domain.Locations{
			ShipperPlace: domain.Place{
				CountryCode: "SE",
				Country:     "Sweden",
				City:        "Sjuntorp",
				PostCode:    "46178",
			},
			ConsigneePlace: domain.Place{
				CountryCode: "FR",
				Country:     "France",
				City:        "Dourges",
				PostCode:    "62119",
			},
		},
	}
}

func TestSender_NameIsAlwaysNil(t *testing.T) {
	s := baseShipment()
	if s.Sender().Name != nil {
		t.Error("Sender().Name is non-nil; DSV public API never returns party names")
	}
}

func TestReceiver_NameIsAlwaysNil(t *testing.T) {
	s := baseShipment()
	if s.Receiver().Name != nil {
		t.Error("Receiver().Name is non-nil; DSV public API never returns party names")
	}
}

func TestSender_AddressFromShipperPlace(t *testing.T) {
	s := baseShipment()
	sender := s.Sender()

	want := s.Locations.ShipperPlace
	if sender.Address != want {
		t.Errorf("Sender().Address = %+v, want %+v", sender.Address, want)
	}
}

func TestReceiver_AddressFromConsigneePlace(t *testing.T) {
	s := baseShipment()
	receiver := s.Receiver()

	want := s.Locations.ConsigneePlace
	if receiver.Address != want {
		t.Errorf("Receiver().Address = %+v, want %+v", receiver.Address, want)
	}
}

func TestSender_ReferencesFromShipper(t *testing.T) {
	s := baseShipment()
	sender := s.Sender()

	if len(sender.References) != 1 || sender.References[0] != "J11709 / J11709" {
		t.Errorf("Sender().References = %v, want [J11709 / J11709]", sender.References)
	}
}

func TestReceiver_ReferencesFromConsignee(t *testing.T) {
	s := baseShipment()
	receiver := s.Receiver()

	if len(receiver.References) != 1 || receiver.References[0] != "8156735" {
		t.Errorf("Receiver().References = %v, want [8156735]", receiver.References)
	}
}

func TestSender_EmptyReferences(t *testing.T) {
	s := baseShipment()
	s.References.Shipper = nil

	sender := s.Sender()
	if len(sender.References) != 0 {
		t.Errorf("Sender().References = %v, want empty", sender.References)
	}
}

func TestReceiver_EmptyReferences(t *testing.T) {
	s := baseShipment()
	s.References.Consignee = nil

	receiver := s.Receiver()
	if len(receiver.References) != 0 {
		t.Errorf("Receiver().References = %v, want empty", receiver.References)
	}
}

// TestParty_SenderReceiverAreDifferent guards against the common mistake of
// returning the same address for both parties.
func TestParty_SenderReceiverAreDifferent(t *testing.T) {
	s := baseShipment()
	if s.Sender().Address == s.Receiver().Address {
		t.Error("Sender and Receiver have the same address; this is only correct for same-city shipments")
	}
}
