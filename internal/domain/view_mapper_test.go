package domain_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv"
)

var updateGolden = flag.Bool("update", false, "update golden view files in testdata/views/")

func fixtureDir(t *testing.T) string {
	t.Helper()
	_, f, _, _ := runtime.Caller(0)
	// .../internal/domain/view_mapper_test.go  → repo root is three dirs up
	return filepath.Join(filepath.Dir(f), "..", "..", "testdata")
}

func viewGoldenDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(fixtureDir(t), "views")
}

// loadDetailShipment reads a fixture JSON, unmarshals via the real DTO+mapper.
func loadDetailShipment(t *testing.T, name string) domain.Shipment {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var dto dsv.ShipmentDetailDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
	s, err := dsv.MapShipmentDetail(&dto)
	if err != nil {
		t.Fatalf("MapShipmentDetail(%s): %v", name, err)
	}
	return s
}

func runGoldenDetailTest(t *testing.T, fixtureName, goldenName string) {
	t.Helper()
	s := loadDetailShipment(t, fixtureName)
	view := domain.MapShipmentDetailView(s)

	gotJSON, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		t.Fatalf("marshal view: %v", err)
	}

	goldenPath := filepath.Join(viewGoldenDir(t), goldenName)

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, gotJSON, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated golden: %s", goldenPath)
		return
	}

	wantJSON, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s (run with -update to create): %v", goldenPath, err)
	}

	if string(gotJSON) != string(wantJSON) {
		t.Errorf("view mismatch for %s\ngot:\n%s\nwant:\n%s", fixtureName, gotJSON, wantJSON)
	}
}

// ─── Golden tests (one per fixture) ─────────────────────────────────────────

func TestViewMapper_DeliveredLTL_SE_FR(t *testing.T) {
	runGoldenDetailTest(t, "delivered_ltl_se_fr.json", "delivered_ltl_se_fr.json")
}

func TestViewMapper_DispatchingParcel_SE_SE(t *testing.T) {
	runGoldenDetailTest(t, "dispatching_parcel_se_se.json", "dispatching_parcel_se_se.json")
}

func TestViewMapper_BookedParcel_SE_SE_Simple(t *testing.T) {
	runGoldenDetailTest(t, "booked_parcel_se_se_simple.json", "booked_parcel_se_se_simple.json")
}

func TestViewMapper_BookedLTL_SE_DK(t *testing.T) {
	runGoldenDetailTest(t, "booked_ltl_se_dk.json", "booked_ltl_se_dk.json")
}

func TestViewMapper_InDelivery_SE_SE(t *testing.T) {
	runGoldenDetailTest(t, "in_delivery_parcel_se_se.json", "in_delivery_parcel_se_se.json")
}

func TestViewMapper_BookedParcel_PostcodeMismatch(t *testing.T) {
	runGoldenDetailTest(t, "booked_parcel_se_se_postcode_mismatch.json", "booked_parcel_se_se_postcode_mismatch.json")
}

func TestViewMapper_BookedParcel_ThreePackages(t *testing.T) {
	runGoldenDetailTest(t, "booked_parcel_se_se_three_packages.json", "booked_parcel_se_se_three_packages.json")
}

// ─── Unit assertions ─────────────────────────────────────────────────────────

func TestViewMapper_ViewInUIURL(t *testing.T) {
	s := loadDetailShipment(t, "delivered_ltl_se_fr.json")
	view := domain.MapShipmentDetailView(s)

	want := "https://mydsv.dsv.com/app/tracking-public/?refNumber=VAN5022058"
	if view.ViewInUIURL != want {
		t.Errorf("ViewInUIURL = %q, want %q", view.ViewInUIURL, want)
	}
}

func TestViewMapper_EventsSortedChronologically(t *testing.T) {
	s := loadDetailShipment(t, "dispatching_parcel_se_se.json")
	view := domain.MapShipmentDetailView(s)

	for i := 1; i < len(view.Events); i++ {
		if view.Events[i].Date < view.Events[i-1].Date {
			t.Errorf("events not sorted: [%d]=%s > [%d]=%s",
				i-1, view.Events[i-1].Date, i, view.Events[i].Date)
		}
	}
}

func TestViewMapper_SenderNameIsNil(t *testing.T) {
	s := loadDetailShipment(t, "delivered_ltl_se_fr.json")
	view := domain.MapShipmentDetailView(s)

	if view.Sender == nil {
		t.Fatal("Sender is nil")
	}
	if view.Sender.Name != nil {
		t.Errorf("Sender.Name = %q, want nil (DSV public API never exposes party names)", *view.Sender.Name)
	}
}

func TestViewMapper_GoodsPopulated(t *testing.T) {
	s := loadDetailShipment(t, "booked_parcel_se_se_simple.json")
	view := domain.MapShipmentDetailView(s)
	if view.Goods == nil {
		t.Fatal("Goods is nil")
	}
	if view.Goods.Pieces != 1 {
		t.Errorf("Pieces = %d, want 1", view.Goods.Pieces)
	}
	if view.Goods.Weight == nil || view.Goods.Weight.Value != 4.0 || view.Goods.Weight.Unit != "KGS" {
		t.Errorf("Weight = %+v, want {4.0 KGS}", view.Goods.Weight)
	}
	if view.Goods.Dimensions != nil {
		t.Errorf("Dimensions = %v, want nil (empty upstream array)", view.Goods.Dimensions)
	}
}

func TestViewMapper_GoodsNullMeasurementOmitted(t *testing.T) {
	g := domain.Goods{Pieces: 0, Weight: domain.Measurement{Value: 0, Unit: ""}}
	view := domain.MapShipmentDetailView(domain.Shipment{
		ShipmentID: "LandStt:T:CTTS:LAND",
		Goods:      g,
		Progress:   domain.Progress{ActiveStep: domain.ProgressStageBooked},
		Events:     nil,
		Packages:   nil,
	})
	if view.Goods == nil {
		t.Fatal("Goods is nil — should always be emitted")
	}
	if view.Goods.Weight != nil {
		t.Errorf("Weight = %+v, want nil (empty unit)", view.Goods.Weight)
	}
}

func TestViewMapper_PackagesAlwaysSlice(t *testing.T) {
	s := loadDetailShipment(t, "booked_parcel_se_se_simple.json")
	view := domain.MapShipmentDetailView(s)
	if view.Packages == nil {
		t.Fatal("Packages is nil, want empty slice []")
	}
	if len(view.Packages) != 1 {
		t.Errorf("len(Packages) = %d, want 1", len(view.Packages))
	}
	if view.Packages[0].Events == nil {
		t.Fatal("Package.Events is nil, want empty slice []")
	}
}

func TestViewMapper_PackageEventsAscending(t *testing.T) {
	s := loadDetailShipment(t, "in_delivery_parcel_se_se.json")
	view := domain.MapShipmentDetailView(s)
	if len(view.Packages) == 0 {
		t.Fatal("expected packages")
	}
	evts := view.Packages[0].Events
	for i := 1; i < len(evts); i++ {
		if evts[i].Date < evts[i-1].Date {
			t.Errorf("package events not sorted: [%d]=%s > [%d]=%s", i-1, evts[i-1].Date, i, evts[i].Date)
		}
	}
}

func TestViewMapper_ThreePackages(t *testing.T) {
	s := loadDetailShipment(t, "booked_parcel_se_se_three_packages.json")
	view := domain.MapShipmentDetailView(s)
	if len(view.Packages) != 3 {
		t.Errorf("len(Packages) = %d, want 3", len(view.Packages))
	}
}

func TestViewMapper_UnknownEventCodePreservesRawCode(t *testing.T) {
	var dto dsv.ShipmentDetailDTO
	raw := []byte(`{
		"sttNumber":"T1","shipmentId":"LandStt:T1:CTTS:LAND",
		"transportMode":"LAND","dataProvider":"CTTS","product":"DSVparcel",
		"progressBar":{"steps":["BOOKED"],"activeStep":"BOOKED"},
		"events":[{"code":"XYZUNKNOWN","date":"2026-01-01T00:00:00Z","createdAt":"2026-01-01T00:00:00Z","location":{"name":"","code":"","countryCode":""},"comment":"","reasons":[]}],
		"packages":[{"id":"p1","events":[]}],
		"goods":{"pieces":1,"volume":{"value":0,"unit":"CBM"},"weight":{"value":0,"unit":"KGS"},"dimensions":[],"loadingMeters":{"value":0,"unit":"MTR"}},
		"references":{"shipper":[],"consignee":[],"waybillAndConsignementNumbers":[],"additionalReferences":[]},
		"deliveryDate":{"estimated":null,"agreed":null},
		"location":{"collectFrom":{"countryCode":"","country":"","city":"","postCode":""},"deliverTo":{"countryCode":"","country":"","city":"","postCode":""},"shipperPlace":{"countryCode":"","country":"","city":"","postCode":""},"consigneePlace":{"countryCode":"","country":"","city":"","postCode":""},"dispatchingOffice":{"countryCode":"","country":"","city":""},"receivingOffice":{"countryCode":"","country":"","city":""}}
	}`)
	if err := json.Unmarshal(raw, &dto); err != nil {
		t.Fatal(err)
	}
	s, err := dsv.MapShipmentDetail(&dto)
	if err != nil {
		t.Fatal(err)
	}
	view := domain.MapShipmentDetailView(s)

	if len(view.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(view.Events))
	}
	if view.Events[0].Code != "" { // EventCodeUnknown.String() == ""
		t.Errorf("Code = %q, want empty (Unknown)", view.Events[0].Code)
	}
	if view.Events[0].RawCode != "XYZUNKNOWN" {
		t.Errorf("RawCode = %q, want XYZUNKNOWN", view.Events[0].RawCode)
	}
	if view.Events[0].Description != "Unknown" {
		t.Errorf("Description = %q, want Unknown", view.Events[0].Description)
	}
}
