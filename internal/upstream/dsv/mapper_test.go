package dsv

// mapper_test.go is a white-box test (package dsv) so it can construct
// unexported DTO sub-types directly. This is intentional: the DTO types are
// internal implementation detail; only the public mapper functions are the API.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file = .../internal/upstream/dsv/mapper_test.go
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata")
}

func loadFixtureDetail(t *testing.T, name string) domain.Shipment {
	t.Helper()
	path := filepath.Join(testdataDir(t), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var dto ShipmentDetailDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", name, err)
	}
	s, err := MapShipmentDetail(&dto)
	if err != nil {
		t.Fatalf("MapShipmentDetail(%s): %v", name, err)
	}
	return s
}

func assertSortedEvents(t *testing.T, events []domain.Event) {
	t.Helper()
	if !slices.IsSortedFunc(events, func(a, b domain.Event) int {
		return a.Date.Compare(b.Date)
	}) {
		t.Error("events not sorted chronologically")
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

// ─── Golden fixture tests ─────────────────────────────────────────────────────

func TestMapper_DeliveredLTL_SE_FR(t *testing.T) {
	s := loadFixtureDetail(t, "delivered_ltl_se_fr.json")

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
	if s.Progress.ActiveStep != domain.ProgressStageDelivered {
		t.Errorf("ActiveStep = %q, want DELIVERED", s.Progress.ActiveStep)
	}
	if s.PercentageProgress != 100 {
		t.Errorf("PercentageProgress = %d, want 100", s.PercentageProgress)
	}
	if s.Combiterms == nil {
		t.Fatal("Combiterms nil for DSV LTL")
	}
	if *s.Combiterms != "Delivered buyer's premises Duty Unpaid" {
		t.Errorf("Combiterms = %q", *s.Combiterms)
	}
	if len(s.Events) != 10 {
		t.Errorf("len(Events) = %d, want 10", len(s.Events))
	}
	assertSortedEvents(t, s.Events)
	if s.Events[0].Code != domain.EventCodeENT {
		t.Errorf("Events[0].Code = %q, want ENT", s.Events[0].Code)
	}
	if s.Events[len(s.Events)-1].Code != domain.EventCodeDLV {
		t.Errorf("last event = %q, want DLV", s.Events[len(s.Events)-1].Code)
	}
	if len(s.Packages) != 2 {
		t.Errorf("len(Packages) = %d, want 2", len(s.Packages))
	}
	for i, p := range s.Packages {
		hasNLO := slices.ContainsFunc(p.Events, func(e domain.PackageEvent) bool {
			return e.Code == domain.EventCodeNLO
		})
		if !hasNLO {
			t.Errorf("Package[%d] missing NLO event", i)
		}
	}
	if s.Locations.CollectFrom.CountryCode != "SE" {
		t.Errorf("CollectFrom.CountryCode = %q, want SE", s.Locations.CollectFrom.CountryCode)
	}
	if s.Locations.DeliverTo.CountryCode != "FR" {
		t.Errorf("DeliverTo.CountryCode = %q, want FR", s.Locations.DeliverTo.CountryCode)
	}
	if s.DeliveryDate.Estimated == nil {
		t.Error("DeliveryDate.Estimated nil for delivered shipment")
	}
	if len(s.References.WaybillAndConsignmentNumbers) != 2 {
		t.Errorf("len(Waybills) = %d, want 2", len(s.References.WaybillAndConsignmentNumbers))
	}
	if s.Sender().Name != nil {
		t.Error("Sender.Name is non-nil")
	}
}

func TestMapper_DispatchingParcel_SE_SE(t *testing.T) {
	s := loadFixtureDetail(t, "dispatching_parcel_se_se.json")

	if s.STTNumber != "SEKSD620143489" {
		t.Errorf("STTNumber = %q", s.STTNumber)
	}
	if s.Combiterms != nil {
		t.Errorf("Combiterms non-nil for parcel: %q", *s.Combiterms)
	}
	if s.Progress.ActiveStep != domain.ProgressStageDispatchingCenter {
		t.Errorf("ActiveStep = %q, want DISPATCHING_CENTER", s.Progress.ActiveStep)
	}
	if s.PercentageProgress != 66 {
		t.Errorf("PercentageProgress = %d, want 66", s.PercentageProgress)
	}
	if len(s.Events) != 6 {
		t.Errorf("len(Events) = %d, want 6", len(s.Events))
	}
	assertSortedEvents(t, s.Events)

	wantOrder := []domain.EventCode{
		domain.EventCodeCOL, domain.EventCodeENM, domain.EventCodeENT,
		domain.EventCodeMAN, domain.EventCodeDIS, domain.EventCodeENM,
	}
	for i, want := range wantOrder {
		if s.Events[i].Code != want {
			t.Errorf("Events[%d].Code = %q, want %q", i, s.Events[i].Code, want)
		}
	}

	dis := s.Events[4]
	if dis.Code != domain.EventCodeDIS {
		t.Fatalf("Events[4] = %q, want DIS", dis.Code)
	}
	if len(dis.Reasons) != 1 {
		t.Fatalf("DIS reasons = %d, want 1", len(dis.Reasons))
	}
	if dis.Reasons[0].Code != "PA" || dis.Reasons[0].Description != "Pre-advice initiated" {
		t.Errorf("DIS reason = {%q, %q}", dis.Reasons[0].Code, dis.Reasons[0].Description)
	}

	if len(s.Packages) != 1 || len(s.Packages[0].Events) == 0 {
		t.Error("expected 1 package with events")
	}
}

func TestMapper_BookedParcel_SE_SE_Simple(t *testing.T) {
	s := loadFixtureDetail(t, "booked_parcel_se_se_simple.json")

	if s.Progress.ActiveStep != domain.ProgressStageBooked {
		t.Errorf("ActiveStep = %q", s.Progress.ActiveStep)
	}
	if len(s.Events) != 1 || s.Events[0].Code != domain.EventCodeENT {
		t.Errorf("expected single ENT, got %d events", len(s.Events))
	}
	assertSortedEvents(t, s.Events)
	if s.Packages[0].Events == nil {
		t.Error("Package.Events is nil, want empty slice")
	}
	if len(s.Packages[0].Events) != 0 {
		t.Errorf("Package.Events has %d events, want 0", len(s.Packages[0].Events))
	}
	if s.DeliveryDate.Estimated != nil {
		t.Error("DeliveryDate.Estimated non-nil for just-booked")
	}
}

func TestMapper_BookedLTL_SE_DK(t *testing.T) {
	s := loadFixtureDetail(t, "booked_ltl_se_dk.json")

	if s.STTNumber != "LKG6022524" {
		t.Errorf("STTNumber = %q", s.STTNumber)
	}
	if s.Locations.DeliverTo.CountryCode != "DK" {
		t.Errorf("DeliverTo.CountryCode = %q, want DK", s.Locations.DeliverTo.CountryCode)
	}
	if len(s.Packages) != 3 {
		t.Errorf("len(Packages) = %d, want 3", len(s.Packages))
	}
	for i, p := range s.Packages {
		if p.Events == nil {
			t.Errorf("Package[%d].Events nil", i)
		}
	}
	if s.Combiterms == nil {
		t.Error("Combiterms nil for DSV LTL")
	}
	if len(s.References.WaybillAndConsignmentNumbers) != 1 {
		t.Errorf("len(Waybills) = %d, want 1", len(s.References.WaybillAndConsignmentNumbers))
	}
	assertSortedEvents(t, s.Events)
}

func TestMapper_InDelivery_SE_SE(t *testing.T) {
	s := loadFixtureDetail(t, "in_delivery_parcel_se_se.json")

	if s.Progress.ActiveStep != domain.ProgressStageInDelivery {
		t.Errorf("ActiveStep = %q", s.Progress.ActiveStep)
	}
	if s.Events[len(s.Events)-1].Code != domain.EventCodeDOT {
		t.Errorf("last event = %q, want DOT", s.Events[len(s.Events)-1].Code)
	}
	assertSortedEvents(t, s.Events)
	want := mustParseTime(t, "2026-05-18T00:00:00Z")
	if s.DeliveryDate.Estimated == nil || !s.DeliveryDate.Estimated.Equal(want) {
		t.Errorf("DeliveryDate.Estimated = %v, want %v", s.DeliveryDate.Estimated, want)
	}
}

func TestMapper_BookedParcel_PostcodeMismatch(t *testing.T) {
	s := loadFixtureDetail(t, "booked_parcel_se_se_postcode_mismatch.json")

	if s.Locations.DeliverTo.PostCode == s.Locations.ConsigneePlace.PostCode {
		t.Errorf("PostCodes equal (%q), expected mismatch", s.Locations.DeliverTo.PostCode)
	}
	if s.Locations.DeliverTo.PostCode != "82455" {
		t.Errorf("DeliverTo.PostCode = %q, want 82455", s.Locations.DeliverTo.PostCode)
	}
	if s.Locations.ConsigneePlace.PostCode != "82450" {
		t.Errorf("ConsigneePlace.PostCode = %q, want 82450", s.Locations.ConsigneePlace.PostCode)
	}
	if s.Receiver().Address.PostCode != "82450" {
		t.Errorf("Receiver.PostCode = %q, want 82450", s.Receiver().Address.PostCode)
	}
	assertSortedEvents(t, s.Events)
}

func TestMapper_BookedParcel_ThreePackages(t *testing.T) {
	s := loadFixtureDetail(t, "booked_parcel_se_se_three_packages.json")

	if len(s.Packages) != 3 {
		t.Errorf("len(Packages) = %d, want 3", len(s.Packages))
	}
	for i, p := range s.Packages {
		if p.Events == nil {
			t.Errorf("Package[%d].Events nil", i)
		}
		if p.ID == "" {
			t.Errorf("Package[%d].ID empty", i)
		}
	}
	assertSortedEvents(t, s.Events)
}

// ─── Mapper-specific edge-case tests ─────────────────────────────────────────

func TestMapper_UnknownEventCode(t *testing.T) {
	dto := &ShipmentDetailDTO{
		STTNumber:  "TEST123",
		ShipmentID: "LandStt:TEST123:CTTS:LAND",
		ProgressBar: &progressBarDTO{
			Steps: []string{"BOOKED"}, ActiveStep: "BOOKED",
		},
		Events:   []eventDTO{{Code: "XYZ", Date: time.Now(), CreatedAt: time.Now()}},
		Packages: []packageDTO{{ID: "p1", Events: []packageEventDTO{}}},
	}
	s, err := MapShipmentDetail(dto)
	if err != nil {
		t.Fatalf("MapShipmentDetail: %v", err)
	}
	if s.Events[0].Code != domain.EventCodeUnknown {
		t.Errorf("Code = %q, want Unknown", s.Events[0].Code)
	}
	if s.Events[0].RawCode != "XYZ" {
		t.Errorf("RawCode = %q, want XYZ", s.Events[0].RawCode)
	}
}

func TestMapper_UnknownTransportMode(t *testing.T) {
	dto := &ShipmentDetailDTO{
		STTNumber:     "T1",
		ShipmentID:    "SeaStt:T1:CTTS:SEA",
		TransportMode: "HELICOPTER",
		ProgressBar:   &progressBarDTO{Steps: []string{"BOOKED"}, ActiveStep: "BOOKED"},
		Events:        []eventDTO{{Code: "ENT", Date: time.Now(), CreatedAt: time.Now()}},
		Packages:      []packageDTO{{ID: "p1", Events: []packageEventDTO{}}},
	}
	s, err := MapShipmentDetail(dto)
	if err != nil {
		t.Fatal(err)
	}
	if s.TransportMode != domain.TransportModeUnknown {
		t.Errorf("TransportMode = %q, want Unknown", s.TransportMode)
	}
}

func TestMapper_NilProgressBarReturnsError(t *testing.T) {
	dto := &ShipmentDetailDTO{
		STTNumber:   "T1",
		ShipmentID:  "LandStt:T1:CTTS:LAND",
		ProgressBar: nil,
		Events:      []eventDTO{},
		Packages:    []packageDTO{},
	}
	_, err := MapShipmentDetail(dto)
	if err == nil {
		t.Fatal("expected error for nil progressBar")
	}
	if !errors.Is(err, domain.ErrMalformedResponse) {
		t.Errorf("errors.Is(ErrMalformedResponse) = false; got %v", err)
	}
	var upstreamErr *domain.UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Errorf("errors.As(*UpstreamError) = false; got %T", err)
	}
	if upstreamErr.Op != "map_shipment_detail" {
		t.Errorf("Op = %q, want map_shipment_detail", upstreamErr.Op)
	}
}

func TestMapper_DefensiveSortOnOutOfOrderEvents(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t1.Add(time.Hour)
	dto := &ShipmentDetailDTO{
		STTNumber:  "T1",
		ShipmentID: "LandStt:T1:CTTS:LAND",
		ProgressBar: &progressBarDTO{
			Steps: []string{"BOOKED", "DELIVERED"}, ActiveStep: "DELIVERED",
		},
		// Feed events in reverse chronological order.
		Events: []eventDTO{
			{Code: "DLV", Date: t2, CreatedAt: t2},
			{Code: "COL", Date: t1, CreatedAt: t1},
			{Code: "ENT", Date: t0, CreatedAt: t0},
		},
		Packages: []packageDTO{{ID: "p1", Events: []packageEventDTO{}}},
	}
	s, err := MapShipmentDetail(dto)
	if err != nil {
		t.Fatal(err)
	}
	assertSortedEvents(t, s.Events)
	if s.Events[0].Code != domain.EventCodeENT {
		t.Errorf("Events[0] = %q, want ENT", s.Events[0].Code)
	}
}

func TestMapper_EmptyPackageEventsNotNil(t *testing.T) {
	dto := &ShipmentDetailDTO{
		STTNumber:   "T1",
		ShipmentID:  "LandStt:T1:CTTS:LAND",
		ProgressBar: &progressBarDTO{Steps: []string{"BOOKED"}, ActiveStep: "BOOKED"},
		Events:      []eventDTO{{Code: "ENT", Date: time.Now(), CreatedAt: time.Now()}},
		Packages:    []packageDTO{{ID: "pkg1", Events: nil}},
	}
	s, err := MapShipmentDetail(dto)
	if err != nil {
		t.Fatal(err)
	}
	if s.Packages[0].Events == nil {
		t.Error("Package.Events is nil; expected empty non-nil slice")
	}
}

func TestMapper_SearchSummary(t *testing.T) {
	path := filepath.Join(testdataDir(t), "search_single_result.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var dto SearchResponseDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatal(err)
	}
	summaries := MapShipmentSummaries(&dto)
	if len(summaries) != 1 {
		t.Fatalf("len = %d, want 1", len(summaries))
	}
	s := summaries[0]
	if s.ShipmentID != "LandStt:LKG6022524:CTTS:LAND" {
		t.Errorf("ShipmentID = %q", s.ShipmentID)
	}
	if s.STTNumber != "LKG6022524" {
		t.Errorf("STTNumber = %q", s.STTNumber)
	}
	if s.TransportMode != domain.TransportModeLand {
		t.Errorf("TransportMode = %q, want LAND", s.TransportMode)
	}
	if s.LastEventCode != domain.EventCodeENT {
		t.Errorf("LastEventCode = %q, want ENT", s.LastEventCode)
	}
	if s.LastEventRawCode != "ENT" {
		t.Errorf("LastEventRawCode = %q, want ENT", s.LastEventRawCode)
	}
}

func TestMapper_SearchSummary_EmptyResult(t *testing.T) {
	dto := &SearchResponseDTO{Result: nil}
	summaries := MapShipmentSummaries(dto)
	if summaries == nil {
		t.Error("returned nil, want empty slice")
	}
}
