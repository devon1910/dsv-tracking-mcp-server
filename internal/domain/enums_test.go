package domain_test

import (
	"testing"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
)

// ─── TransportMode ────────────────────────────────────────────────────────────

func TestTransportMode_KnownValues(t *testing.T) {
	cases := []struct {
		mode domain.TransportMode
		desc string
	}{
		{domain.TransportModeLand, "Land"},
		{domain.TransportModeSea, "Sea"},
		{domain.TransportModeAir, "Air"},
		{domain.TransportModeRail, "Rail"},
	}
	for _, tc := range cases {
		if !tc.mode.IsKnown() {
			t.Errorf("TransportMode(%q).IsKnown() = false, want true", tc.mode)
		}
		if got := tc.mode.Description(); got != tc.desc {
			t.Errorf("TransportMode(%q).Description() = %q, want %q", tc.mode, got, tc.desc)
		}
		if got := tc.mode.String(); got != string(tc.mode) {
			t.Errorf("TransportMode(%q).String() = %q, want %q", tc.mode, got, string(tc.mode))
		}
	}
}

func TestTransportMode_UnknownValue(t *testing.T) {
	m := domain.ParseTransportMode("BOAT")
	if m != domain.TransportModeUnknown {
		t.Errorf("ParseTransportMode(%q) = %q, want Unknown", "BOAT", m)
	}
	if m.IsKnown() {
		t.Error("TransportModeUnknown.IsKnown() = true, want false")
	}
	if got := m.Description(); got != "Unknown" {
		t.Errorf("TransportModeUnknown.Description() = %q, want %q", got, "Unknown")
	}
}

func TestParseTransportMode_RoundTrip(t *testing.T) {
	for _, m := range []domain.TransportMode{
		domain.TransportModeLand, domain.TransportModeSea,
		domain.TransportModeAir, domain.TransportModeRail,
	} {
		if got := domain.ParseTransportMode(m.String()); got != m {
			t.Errorf("ParseTransportMode(String(%q)) = %q, want %q", m, got, m)
		}
	}
}

// ─── EventCode ────────────────────────────────────────────────────────────────

func TestEventCode_KnownValues(t *testing.T) {
	cases := []struct {
		code domain.EventCode
		desc string
	}{
		{domain.EventCodeENT, "Booked"},
		{domain.EventCodeCOL, "Collected"},
		{domain.EventCodeENM, "Arrived at hub"},
		{domain.EventCodeMAN, "Departed"},
		{domain.EventCodeDOT, "Out for delivery"},
		{domain.EventCodeDLV, "Delivered"},
		{domain.EventCodeNLO, "Loaded for transport"},
		{domain.EventCodeDIS, "To consignee's disposal"},
	}
	for _, tc := range cases {
		if !tc.code.IsKnown() {
			t.Errorf("EventCode(%q).IsKnown() = false, want true", tc.code)
		}
		if got := tc.code.Description(); got != tc.desc {
			t.Errorf("EventCode(%q).Description() = %q, want %q", tc.code, got, tc.desc)
		}
		if got := tc.code.String(); got != string(tc.code) {
			t.Errorf("EventCode(%q).String() = %q, want %q", tc.code, got, string(tc.code))
		}
		if got := domain.ParseEventCode(string(tc.code)); got != tc.code {
			t.Errorf("ParseEventCode(%q) = %q, want %q", string(tc.code), got, tc.code)
		}
	}
}

func TestEventCode_UnknownValue(t *testing.T) {
	c := domain.ParseEventCode("ZZZ")
	if c != domain.EventCodeUnknown {
		t.Errorf("ParseEventCode(%q) = %q, want Unknown", "ZZZ", c)
	}
	if c.IsKnown() {
		t.Error("EventCodeUnknown.IsKnown() = true, want false")
	}
	if got := c.Description(); got != "Unknown" {
		t.Errorf("EventCodeUnknown.Description() = %q, want %q", got, "Unknown")
	}
}

func TestParseEventCode_RoundTrip(t *testing.T) {
	known := []domain.EventCode{
		domain.EventCodeENT, domain.EventCodeCOL, domain.EventCodeENM,
		domain.EventCodeMAN, domain.EventCodeDOT, domain.EventCodeDLV,
		domain.EventCodeNLO, domain.EventCodeDIS,
	}
	for _, c := range known {
		if got := domain.ParseEventCode(c.String()); got != c {
			t.Errorf("ParseEventCode(String(%q)) = %q, want %q", c, got, c)
		}
	}
}

// ─── ProgressStage ────────────────────────────────────────────────────────────

func TestProgressStage_KnownValues(t *testing.T) {
	cases := []struct {
		stage domain.ProgressStage
		desc  string
	}{
		{domain.ProgressStageBooked, "Booked"},
		{domain.ProgressStageTransportation, "In transportation"},
		{domain.ProgressStageDispatchingCenter, "At dispatching centre"},
		{domain.ProgressStageInDelivery, "Out for delivery"},
		{domain.ProgressStageDelivered, "Delivered"},
	}
	for _, tc := range cases {
		if !tc.stage.IsKnown() {
			t.Errorf("ProgressStage(%q).IsKnown() = false, want true", tc.stage)
		}
		if got := tc.stage.Description(); got != tc.desc {
			t.Errorf("ProgressStage(%q).Description() = %q, want %q", tc.stage, got, tc.desc)
		}
		if got := tc.stage.String(); got != string(tc.stage) {
			t.Errorf("ProgressStage(%q).String() = %q, want %q", tc.stage, got, string(tc.stage))
		}
	}
}

func TestProgressStage_UnknownValue(t *testing.T) {
	s := domain.ParseProgressStage("PENDING")
	if s != domain.ProgressStageUnknown {
		t.Errorf("ParseProgressStage(%q) = %q, want Unknown", "PENDING", s)
	}
	if s.IsKnown() {
		t.Error("ProgressStageUnknown.IsKnown() = true, want false")
	}
	if got := s.Description(); got != "Unknown" {
		t.Errorf("ProgressStageUnknown.Description() = %q, want %q", got, "Unknown")
	}
}

func TestParseProgressStage_RoundTrip(t *testing.T) {
	known := []domain.ProgressStage{
		domain.ProgressStageBooked, domain.ProgressStageTransportation,
		domain.ProgressStageDispatchingCenter, domain.ProgressStageInDelivery,
		domain.ProgressStageDelivered,
	}
	for _, s := range known {
		if got := domain.ParseProgressStage(s.String()); got != s {
			t.Errorf("ParseProgressStage(String(%q)) = %q, want %q", s, got, s)
		}
	}
}
