package domain_test

// fixtures_test.go: golden-fixture contract tests (Tier 3).
//
// Each test loads one of the 7 detail fixtures from testdata/, maps it to a
// domain.Shipment via the private DTO defined below, and asserts the domain
// shape captures every observed variation correctly.
//
// This file contains temporary test scaffolding: the fixtureDTO type and the
// fromFixtureDTO mapper are stand-ins for the real DTO and mapper that will be
// written in Phase 3 (internal/upstream/dsv/). They will be deleted once the
// real mapper exists and the tests are updated to use it.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
)

// ─── Private DTO ─────────────────────────────────────────────────────────────
// fixtureDTO is a minimal struct that mirrors the upstream JSON shape just
// enough to unmarshal the 7 golden fixtures.

type fixtureDTO struct {
	STTNumber          string     `json:"sttNumber"`
	ShipmentID         string     `json:"shipmentId"`
	TransportMode      string     `json:"transportMode"`
	DataProvider       string     `json:"dataProvider"`
	Product            string     `json:"product"`
	IsXpress           bool       `json:"isXpress"`
	PercentageProgress int        `json:"percentageProgress"`
	Combiterms         *string    `json:"combiterms"`
	References         dtoRefs    `json:"references"`
	Goods              dtoGoods   `json:"goods"`
	Events             []dtoEvent `json:"events"`
	Packages           []dtoPkg   `json:"packages"`
	DeliveryDate       struct {
		Estimated *time.Time `json:"estimated"`
		Agreed    *time.Time `json:"agreed"`
	} `json:"deliveryDate"`
	ProgressBar struct {
		Steps      []string `json:"steps"`
		ActiveStep string   `json:"activeStep"`
	} `json:"progressBar"`
	Location dtoLocation `json:"location"`
}

type dtoRefs struct {
	Shipper                         []string `json:"shipper"`
	Consignee                       []string `json:"consignee"`
	WaybillAndConsignementNumbers   []string `json:"waybillAndConsignementNumbers"` // upstream typo preserved
}

type dtoGoods struct {
	Pieces        int            `json:"pieces"`
	Weight        dtoMeasurement `json:"weight"`
	Volume        dtoMeasurement `json:"volume"`
	LoadingMeters dtoMeasurement `json:"loadingMeters"`
}

type dtoMeasurement struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type dtoEvent struct {
	Code      string    `json:"code"`
	Date      time.Time `json:"date"`
	CreatedAt time.Time `json:"createdAt"`
	Location  struct {
		Name        string `json:"name"`
		Code        string `json:"code"`
		CountryCode string `json:"countryCode"`
	} `json:"location"`
	Comment string `json:"comment"`
	Reasons []struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	} `json:"reasons"`
}

type dtoPkg struct {
	ID     string `json:"id"`
	Events []struct {
		Code        string    `json:"code"`
		CountryCode string    `json:"countryCode"`
		Location    string    `json:"location"`
		Date        time.Time `json:"date"`
	} `json:"events"`
}

type dtoLocation struct {
	CollectFrom struct {
		CountryCode string `json:"countryCode"`
		Country     string `json:"country"`
		City        string `json:"city"`
		PostCode    string `json:"postCode"`
	} `json:"collectFrom"`
	DeliverTo struct {
		CountryCode string `json:"countryCode"`
		Country     string `json:"country"`
		City        string `json:"city"`
		PostCode    string `json:"postCode"`
	} `json:"deliverTo"`
	ShipperPlace struct {
		CountryCode string `json:"countryCode"`
		Country     string `json:"country"`
		City        string `json:"city"`
		PostCode    string `json:"postCode"`
	} `json:"shipperPlace"`
	ConsigneePlace struct {
		CountryCode string `json:"countryCode"`
		Country     string `json:"country"`
		City        string `json:"city"`
		PostCode    string `json:"postCode"`
	} `json:"consigneePlace"`
	DispatchingOffice struct {
		CountryCode string `json:"countryCode"`
		Country     string `json:"country"`
		City        string `json:"city"`
	} `json:"dispatchingOffice"`
	ReceivingOffice struct {
		CountryCode string `json:"countryCode"`
		Country     string `json:"country"`
		City        string `json:"city"`
	} `json:"receivingOffice"`
}

// ─── Mapper ──────────────────────────────────────────────────────────────────

// fromFixtureDTO maps a fixtureDTO to a domain.Shipment.
// This is temporary scaffolding; it will be superseded by the real mapper
// in internal/upstream/dsv/ in Phase 3.
func fromFixtureDTO(d fixtureDTO) domain.Shipment {
	events := make([]domain.Event, len(d.Events))
	for i, e := range d.Events {
		reasons := make([]domain.EventReason, len(e.Reasons))
		for j, r := range e.Reasons {
			reasons[j] = domain.EventReason{Code: r.Code, Description: r.Description}
		}
		events[i] = domain.Event{
			Code:    domain.ParseEventCode(e.Code),
			RawCode: e.Code,
			Date:    e.Date,
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

	packages := make([]domain.Package, len(d.Packages))
	for i, p := range d.Packages {
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

	stages := make([]domain.ProgressStage, len(d.ProgressBar.Steps))
	for i, s := range d.ProgressBar.Steps {
		stages[i] = domain.ParseProgressStage(s)
	}

	loc := d.Location
	return domain.Shipment{
		STTNumber:          d.STTNumber,
		ShipmentID:         d.ShipmentID,
		TransportMode:      domain.ParseTransportMode(d.TransportMode),
		DataProvider:       d.DataProvider,
		Product:            d.Product,
		IsXpress:           d.IsXpress,
		PercentageProgress: d.PercentageProgress,
		Combiterms:         d.Combiterms,
		References: domain.References{
			Shipper:                      d.References.Shipper,
			Consignee:                    d.References.Consignee,
			WaybillAndConsignmentNumbers: d.References.WaybillAndConsignementNumbers,
		},
		Goods: domain.Goods{
			Pieces:        d.Goods.Pieces,
			Weight:        domain.Measurement{Value: d.Goods.Weight.Value, Unit: d.Goods.Weight.Unit},
			Volume:        domain.Measurement{Value: d.Goods.Volume.Value, Unit: d.Goods.Volume.Unit},
			LoadingMeters: domain.Measurement{Value: d.Goods.LoadingMeters.Value, Unit: d.Goods.LoadingMeters.Unit},
		},
		Events:   events,
		Packages: packages,
		Locations: domain.Locations{
			CollectFrom:       domain.Place{CountryCode: loc.CollectFrom.CountryCode, Country: loc.CollectFrom.Country, City: loc.CollectFrom.City, PostCode: loc.CollectFrom.PostCode},
			DeliverTo:         domain.Place{CountryCode: loc.DeliverTo.CountryCode, Country: loc.DeliverTo.Country, City: loc.DeliverTo.City, PostCode: loc.DeliverTo.PostCode},
			ShipperPlace:      domain.Place{CountryCode: loc.ShipperPlace.CountryCode, Country: loc.ShipperPlace.Country, City: loc.ShipperPlace.City, PostCode: loc.ShipperPlace.PostCode},
			ConsigneePlace:    domain.Place{CountryCode: loc.ConsigneePlace.CountryCode, Country: loc.ConsigneePlace.Country, City: loc.ConsigneePlace.City, PostCode: loc.ConsigneePlace.PostCode},
			DispatchingOffice: domain.Office{CountryCode: loc.DispatchingOffice.CountryCode, Country: loc.DispatchingOffice.Country, City: loc.DispatchingOffice.City},
			ReceivingOffice:   domain.Office{CountryCode: loc.ReceivingOffice.CountryCode, Country: loc.ReceivingOffice.Country, City: loc.ReceivingOffice.City},
		},
		DeliveryDate: domain.DeliveryDate{
			Estimated: d.DeliveryDate.Estimated,
			Agreed:    d.DeliveryDate.Agreed,
		},
		Progress: domain.Progress{
			Steps:      stages,
			ActiveStep: domain.ParseProgressStage(d.ProgressBar.ActiveStep),
		},
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// testdataDir returns the absolute path to testdata/ regardless of working dir.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../internal/domain/fixtures_test.go
	// testdata is two directories up: .../testdata/
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "testdata")
}

func loadFixture(t *testing.T, name string) domain.Shipment {
	t.Helper()
	path := filepath.Join(testdataDir(t), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var dto fixtureDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", name, err)
	}
	return fromFixtureDTO(dto)
}

// assertSorted verifies that events are in chronological order.
func assertSorted(t *testing.T, events []domain.Event) {
	t.Helper()
	if !slices.IsSortedFunc(events, func(a, b domain.Event) int {
		return a.Date.Compare(b.Date)
	}) {
		t.Error("events are not sorted chronologically")
		for i, e := range events {
			t.Logf("  [%d] %s %s", i, e.Code, e.Date.Format(time.RFC3339))
		}
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return ts
}

// ─── Fixture tests ───────────────────────────────────────────────────────────

func TestFixture_DeliveredLTL_SE_FR(t *testing.T) {
	s := loadFixture(t, "delivered_ltl_se_fr.json")

	// Identity
	if s.STTNumber != "VAN5022058" {
		t.Errorf("STTNumber = %q, want VAN5022058", s.STTNumber)
	}
	if s.ShipmentID != "LandStt:VAN5022058:CTTS:LAND" {
		t.Errorf("ShipmentID = %q", s.ShipmentID)
	}
	if s.TransportMode != domain.TransportModeLand {
		t.Errorf("TransportMode = %q, want LAND", s.TransportMode)
	}
	if s.Product != "DSV LTL" {
		t.Errorf("Product = %q, want DSV LTL", s.Product)
	}

	// Progress
	if s.Progress.ActiveStep != domain.ProgressStageDelivered {
		t.Errorf("ActiveStep = %q, want DELIVERED", s.Progress.ActiveStep)
	}
	if s.PercentageProgress != 100 {
		t.Errorf("PercentageProgress = %d, want 100", s.PercentageProgress)
	}
	if len(s.Progress.Steps) != 5 {
		t.Errorf("len(Steps) = %d, want 5", len(s.Progress.Steps))
	}

	// Combiterms (set for LTL)
	if s.Combiterms == nil {
		t.Error("Combiterms is nil, want non-nil for DSV LTL")
	} else if *s.Combiterms != "Delivered buyer's premises Duty Unpaid" {
		t.Errorf("Combiterms = %q", *s.Combiterms)
	}

	// Events
	if len(s.Events) != 10 {
		t.Errorf("len(Events) = %d, want 10", len(s.Events))
	}
	assertSorted(t, s.Events)
	if s.Events[0].Code != domain.EventCodeENT {
		t.Errorf("Events[0].Code = %q, want ENT", s.Events[0].Code)
	}
	if s.Events[len(s.Events)-1].Code != domain.EventCodeDLV {
		t.Errorf("last event Code = %q, want DLV", s.Events[len(s.Events)-1].Code)
	}

	// Packages (2 packages, both with events populated)
	if len(s.Packages) != 2 {
		t.Errorf("len(Packages) = %d, want 2", len(s.Packages))
	}
	for i, p := range s.Packages {
		if len(p.Events) == 0 {
			t.Errorf("Package[%d] has no events", i)
		}
		// NLO should appear in package events
		hasNLO := slices.ContainsFunc(p.Events, func(e domain.PackageEvent) bool {
			return e.Code == domain.EventCodeNLO
		})
		if !hasNLO {
			t.Errorf("Package[%d] has no NLO event", i)
		}
	}

	// Locations
	if s.Locations.CollectFrom.CountryCode != "SE" {
		t.Errorf("CollectFrom.CountryCode = %q, want SE", s.Locations.CollectFrom.CountryCode)
	}
	if s.Locations.DeliverTo.CountryCode != "FR" {
		t.Errorf("DeliverTo.CountryCode = %q, want FR", s.Locations.DeliverTo.CountryCode)
	}

	// Sender / Receiver
	if s.Sender().Name != nil {
		t.Error("Sender.Name is non-nil")
	}
	if len(s.References.Shipper) == 0 {
		t.Error("Shipper references are empty")
	}

	// Delivery date
	if s.DeliveryDate.Estimated == nil {
		t.Error("DeliveryDate.Estimated is nil, want non-nil for delivered shipment")
	}

	// Waybills (2 for this shipment)
	if len(s.References.WaybillAndConsignmentNumbers) != 2 {
		t.Errorf("len(WaybillAndConsignmentNumbers) = %d, want 2", len(s.References.WaybillAndConsignmentNumbers))
	}
}

func TestFixture_DispatchingParcel_SE_SE(t *testing.T) {
	s := loadFixture(t, "dispatching_parcel_se_se.json")

	if s.STTNumber != "SEKSD620143489" {
		t.Errorf("STTNumber = %q, want SEKSD620143489", s.STTNumber)
	}
	if s.Product != "DSVparcel" {
		t.Errorf("Product = %q, want DSVparcel", s.Product)
	}
	if s.Combiterms != nil {
		t.Errorf("Combiterms = %q, want nil for DSVparcel", *s.Combiterms)
	}

	// Progress
	if s.Progress.ActiveStep != domain.ProgressStageDispatchingCenter {
		t.Errorf("ActiveStep = %q, want DISPATCHING_CENTER", s.Progress.ActiveStep)
	}
	if s.PercentageProgress != 66 {
		t.Errorf("PercentageProgress = %d, want 66", s.PercentageProgress)
	}

	// 6 events, must be sorted chronologically.
	if len(s.Events) != 6 {
		t.Errorf("len(Events) = %d, want 6", len(s.Events))
	}
	assertSorted(t, s.Events)

	// After sorting: COL → ENM → ENT → MAN → DIS → ENM.
	// The ENT (booking created at 15:24) appears after COL (14:00) and ENM (14:01)
	// because the booking record was entered after physical collection — a real
	// operational pattern, not a data error.
	wantOrder := []domain.EventCode{
		domain.EventCodeCOL,
		domain.EventCodeENM,
		domain.EventCodeENT,
		domain.EventCodeMAN,
		domain.EventCodeDIS,
		domain.EventCodeENM,
	}
	for i, want := range wantOrder {
		if s.Events[i].Code != want {
			t.Errorf("Events[%d].Code = %q, want %q", i, s.Events[i].Code, want)
		}
	}

	// The DIS event (index 4) must have a reason populated.
	disEvent := s.Events[4]
	if disEvent.Code != domain.EventCodeDIS {
		t.Fatalf("Events[4].Code = %q, want DIS", disEvent.Code)
	}
	if len(disEvent.Reasons) != 1 {
		t.Fatalf("DIS event has %d reasons, want 1", len(disEvent.Reasons))
	}
	if disEvent.Reasons[0].Code != "PA" {
		t.Errorf("DIS reason Code = %q, want PA", disEvent.Reasons[0].Code)
	}
	if disEvent.Reasons[0].Description != "Pre-advice initiated" {
		t.Errorf("DIS reason Description = %q, want Pre-advice initiated", disEvent.Reasons[0].Description)
	}

	// 1 package, with events populated.
	if len(s.Packages) != 1 {
		t.Errorf("len(Packages) = %d, want 1", len(s.Packages))
	}
	if len(s.Packages[0].Events) == 0 {
		t.Error("Package[0] has no events")
	}

	// Delivery date estimated is populated (route planned).
	if s.DeliveryDate.Estimated == nil {
		t.Error("DeliveryDate.Estimated is nil")
	}
}

func TestFixture_BookedParcel_SE_SE_Simple(t *testing.T) {
	s := loadFixture(t, "booked_parcel_se_se_simple.json")

	if s.STTNumber != "SEGOT620304613" {
		t.Errorf("STTNumber = %q", s.STTNumber)
	}
	if s.Progress.ActiveStep != domain.ProgressStageBooked {
		t.Errorf("ActiveStep = %q, want BOOKED", s.Progress.ActiveStep)
	}
	if s.PercentageProgress != 16 {
		t.Errorf("PercentageProgress = %d, want 16", s.PercentageProgress)
	}

	// Only the ENT event — minimum observed event set.
	if len(s.Events) != 1 {
		t.Errorf("len(Events) = %d, want 1", len(s.Events))
	}
	if s.Events[0].Code != domain.EventCodeENT {
		t.Errorf("Events[0].Code = %q, want ENT", s.Events[0].Code)
	}
	assertSorted(t, s.Events)

	// Package exists but has no events yet.
	if len(s.Packages) != 1 {
		t.Errorf("len(Packages) = %d, want 1", len(s.Packages))
	}
	if len(s.Packages[0].Events) != 0 {
		t.Errorf("Package[0] has %d events, want 0 for just-booked shipment", len(s.Packages[0].Events))
	}

	// Delivery date not yet estimated.
	if s.DeliveryDate.Estimated != nil {
		t.Errorf("DeliveryDate.Estimated non-nil for just-booked shipment: %v", s.DeliveryDate.Estimated)
	}
}

func TestFixture_BookedLTL_SE_DK(t *testing.T) {
	s := loadFixture(t, "booked_ltl_se_dk.json")

	if s.STTNumber != "LKG6022524" {
		t.Errorf("STTNumber = %q", s.STTNumber)
	}
	if s.Product != "DSV LTL" {
		t.Errorf("Product = %q, want DSV LTL", s.Product)
	}

	// Cross-border: SE → DK
	if s.Locations.CollectFrom.CountryCode != "SE" {
		t.Errorf("CollectFrom.CountryCode = %q, want SE", s.Locations.CollectFrom.CountryCode)
	}
	if s.Locations.DeliverTo.CountryCode != "DK" {
		t.Errorf("DeliverTo.CountryCode = %q, want DK", s.Locations.DeliverTo.CountryCode)
	}

	// 3 packages, none with events (just booked).
	if len(s.Packages) != 3 {
		t.Errorf("len(Packages) = %d, want 3", len(s.Packages))
	}
	for i, p := range s.Packages {
		if len(p.Events) != 0 {
			t.Errorf("Package[%d] has %d events, want 0", i, len(p.Events))
		}
	}

	// Combiterms set for LTL, even when just booked.
	if s.Combiterms == nil {
		t.Error("Combiterms nil for DSV LTL")
	}

	// 1 waybill (3 packages but single booking — see UPSTREAM.md Cache-Key Insight).
	if len(s.References.WaybillAndConsignmentNumbers) != 1 {
		t.Errorf("len(WaybillAndConsignmentNumbers) = %d, want 1", len(s.References.WaybillAndConsignmentNumbers))
	}

	assertSorted(t, s.Events)
}

func TestFixture_InDeliveryParcel_SE_SE(t *testing.T) {
	s := loadFixture(t, "in_delivery_parcel_se_se.json")

	if s.STTNumber != "SESTO620296604" {
		t.Errorf("STTNumber = %q", s.STTNumber)
	}
	if s.Progress.ActiveStep != domain.ProgressStageInDelivery {
		t.Errorf("ActiveStep = %q, want IN_DELIVERY", s.Progress.ActiveStep)
	}
	if s.PercentageProgress != 83 {
		t.Errorf("PercentageProgress = %d, want 83", s.PercentageProgress)
	}
	if len(s.Events) != 6 {
		t.Errorf("len(Events) = %d, want 6", len(s.Events))
	}
	assertSorted(t, s.Events)

	// Last event should be DOT (Out for delivery).
	if last := s.Events[len(s.Events)-1].Code; last != domain.EventCodeDOT {
		t.Errorf("last event = %q, want DOT", last)
	}

	// Delivery estimated today (2026-05-18).
	if s.DeliveryDate.Estimated == nil {
		t.Error("DeliveryDate.Estimated is nil")
	} else {
		want := mustParseTime(t, "2026-05-18T00:00:00Z")
		if !s.DeliveryDate.Estimated.Equal(want) {
			t.Errorf("DeliveryDate.Estimated = %v, want %v", s.DeliveryDate.Estimated, want)
		}
	}

	// 1 package with events populated.
	if len(s.Packages) != 1 {
		t.Errorf("len(Packages) = %d, want 1", len(s.Packages))
	}
	if len(s.Packages[0].Events) == 0 {
		t.Error("Package[0] has no events for in-delivery shipment")
	}

	// Sender reference present.
	if len(s.References.Shipper) == 0 {
		t.Error("Shipper references empty")
	}
}

func TestFixture_BookedParcel_SE_SE_PostcodeMismatch(t *testing.T) {
	s := loadFixture(t, "booked_parcel_se_se_postcode_mismatch.json")

	if s.STTNumber != "SESTO620298048" {
		t.Errorf("STTNumber = %q", s.STTNumber)
	}

	// The key invariant: deliverTo and consigneePlace are both in Hudiksvall
	// but have different post codes. Consumers must not assume they are equal.
	deliverToPC := s.Locations.DeliverTo.PostCode
	consigneePC := s.Locations.ConsigneePlace.PostCode
	if deliverToPC == consigneePC {
		t.Errorf("DeliverTo.PostCode == ConsigneePlace.PostCode (%q) — expected mismatch for this fixture", deliverToPC)
	}
	if deliverToPC != "82455" {
		t.Errorf("DeliverTo.PostCode = %q, want 82455", deliverToPC)
	}
	if consigneePC != "82450" {
		t.Errorf("ConsigneePlace.PostCode = %q, want 82450", consigneePC)
	}
	if s.Locations.DeliverTo.City != s.Locations.ConsigneePlace.City {
		t.Errorf("DeliverTo.City = %q, ConsigneePlace.City = %q; expected same city",
			s.Locations.DeliverTo.City, s.Locations.ConsigneePlace.City)
	}

	// Receiver address uses ConsigneePlace, not DeliverTo.
	receiver := s.Receiver()
	if receiver.Address.PostCode != "82450" {
		t.Errorf("Receiver.Address.PostCode = %q, want 82450 (ConsigneePlace)", receiver.Address.PostCode)
	}

	assertSorted(t, s.Events)
}

func TestFixture_BookedParcel_SE_SE_ThreePackages(t *testing.T) {
	s := loadFixture(t, "booked_parcel_se_se_three_packages.json")

	if s.STTNumber != "SESTO620298049" {
		t.Errorf("STTNumber = %q", s.STTNumber)
	}
	if len(s.Packages) != 3 {
		t.Errorf("len(Packages) = %d, want 3", len(s.Packages))
	}
	for i, p := range s.Packages {
		if len(p.Events) != 0 {
			t.Errorf("Package[%d] has %d events, want 0", i, len(p.Events))
		}
		if p.ID == "" {
			t.Errorf("Package[%d].ID is empty", i)
		}
	}

	// 1 waybill (3 packages, single booking).
	if len(s.References.WaybillAndConsignmentNumbers) != 1 {
		t.Errorf("len(WaybillAndConsignmentNumbers) = %d, want 1", len(s.References.WaybillAndConsignmentNumbers))
	}

	if s.Progress.ActiveStep != domain.ProgressStageBooked {
		t.Errorf("ActiveStep = %q, want BOOKED", s.Progress.ActiveStep)
	}

	// Shipper and consignee references both present.
	if len(s.References.Shipper) == 0 {
		t.Error("Shipper references empty")
	}
	if len(s.References.Consignee) == 0 {
		t.Error("Consignee references empty")
	}

	assertSorted(t, s.Events)
}

// TestAllFixtures_TransportModeKnown ensures every fixture uses a known
// transport mode — a guard against unexpected data in the fixtures.
func TestAllFixtures_TransportModeKnown(t *testing.T) {
	fixtures := []string{
		"delivered_ltl_se_fr.json",
		"dispatching_parcel_se_se.json",
		"booked_parcel_se_se_simple.json",
		"booked_ltl_se_dk.json",
		"in_delivery_parcel_se_se.json",
		"booked_parcel_se_se_postcode_mismatch.json",
		"booked_parcel_se_se_three_packages.json",
	}
	for _, name := range fixtures {
		s := loadFixture(t, name)
		if !s.TransportMode.IsKnown() {
			t.Errorf("%s: TransportMode %q is not known", name, s.TransportMode)
		}
	}
}

// TestAllFixtures_ProgressStageKnown ensures every fixture's active progress
// stage is one of the defined constants.
func TestAllFixtures_ProgressStageKnown(t *testing.T) {
	fixtures := []string{
		"delivered_ltl_se_fr.json",
		"dispatching_parcel_se_se.json",
		"booked_parcel_se_se_simple.json",
		"booked_ltl_se_dk.json",
		"in_delivery_parcel_se_se.json",
		"booked_parcel_se_se_postcode_mismatch.json",
		"booked_parcel_se_se_three_packages.json",
	}
	for _, name := range fixtures {
		s := loadFixture(t, name)
		if !s.Progress.ActiveStep.IsKnown() {
			t.Errorf("%s: ActiveStep %q is not known", name, s.Progress.ActiveStep)
		}
	}
}

// TestAllFixtures_EventsSorted ensures the mapper delivers sorted events for
// every fixture — the contract that consumers depend on.
func TestAllFixtures_EventsSorted(t *testing.T) {
	fixtures := []string{
		"delivered_ltl_se_fr.json",
		"dispatching_parcel_se_se.json",
		"booked_parcel_se_se_simple.json",
		"booked_ltl_se_dk.json",
		"in_delivery_parcel_se_se.json",
		"booked_parcel_se_se_postcode_mismatch.json",
		"booked_parcel_se_se_three_packages.json",
	}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			s := loadFixture(t, name)
			assertSorted(t, s.Events)
		})
	}
}
