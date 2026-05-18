package domain_test

import (
	"slices"
	"testing"
	"time"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
)

// isSortedByDate returns true if events are sorted by Date ascending.
func isSortedByDate(events []domain.Event) bool {
	return slices.IsSortedFunc(events, func(a, b domain.Event) int {
		return a.Date.Compare(b.Date)
	})
}

// isSortedByDatePkg is the same check for package events.
func isSortedByDatePkg(events []domain.PackageEvent) bool {
	return slices.IsSortedFunc(events, func(a, b domain.PackageEvent) int {
		return a.Date.Compare(b.Date)
	})
}

// TestSortEventsChronologically_AlreadySorted verifies idempotency.
func TestSortEventsChronologically_AlreadySorted(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t1.Add(time.Hour)

	events := []domain.Event{
		{Code: domain.EventCodeENT, Date: t0},
		{Code: domain.EventCodeCOL, Date: t1},
		{Code: domain.EventCodeENM, Date: t2},
	}
	domain.SortEventsChronologically(events)
	if !isSortedByDate(events) {
		t.Error("already-sorted events not sorted after SortEventsChronologically")
	}
}

// TestSortEventsChronologically_ReverseOrder is the TDD test: events arrive in
// reverse chronological order (latest first) and must be sorted to earliest first.
func TestSortEventsChronologically_ReverseOrder(t *testing.T) {
	t0 := time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC) // COL
	t1 := t0.Add(time.Minute)                            // ENM
	t2 := t1.Add(time.Hour + 23*time.Minute)             // ENT (booking created late — real pattern)

	// Simulate upstream delivering events latest-first (reverse of physical order).
	events := []domain.Event{
		{Code: domain.EventCodeENT, Date: t2},
		{Code: domain.EventCodeENM, Date: t1},
		{Code: domain.EventCodeCOL, Date: t0},
	}

	domain.SortEventsChronologically(events)

	if !isSortedByDate(events) {
		t.Fatal("events not sorted chronologically after SortEventsChronologically")
	}
	// Verify the specific order: COL → ENM → ENT.
	if events[0].Code != domain.EventCodeCOL {
		t.Errorf("events[0].Code = %q, want COL", events[0].Code)
	}
	if events[1].Code != domain.EventCodeENM {
		t.Errorf("events[1].Code = %q, want ENM", events[1].Code)
	}
	if events[2].Code != domain.EventCodeENT {
		t.Errorf("events[2].Code = %q, want ENT (booking created after collection)", events[2].Code)
	}
}

// TestSortEventsChronologically_Stable verifies that equal-timestamp events
// maintain their original relative order (stable sort).
func TestSortEventsChronologically_Stable(t *testing.T) {
	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	events := []domain.Event{
		{Code: domain.EventCodeENM, Date: ts, Comment: "first"},
		{Code: domain.EventCodeENM, Date: ts, Comment: "second"},
	}
	domain.SortEventsChronologically(events)
	if events[0].Comment != "first" || events[1].Comment != "second" {
		t.Error("stable sort not preserved for equal-timestamp events")
	}
}

// TestSortEventsChronologically_EmptySlice ensures no panic on empty input.
func TestSortEventsChronologically_EmptySlice(t *testing.T) {
	domain.SortEventsChronologically(nil)
	domain.SortEventsChronologically([]domain.Event{})
}

// TestSortPackageEventsChronologically_ReverseOrder mirrors the shipment test
// for package-level events.
func TestSortPackageEventsChronologically_ReverseOrder(t *testing.T) {
	t0 := time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t1.Add(time.Hour)

	events := []domain.PackageEvent{
		{Code: domain.EventCodeDLV, Date: t2},
		{Code: domain.EventCodeENM, Date: t1},
		{Code: domain.EventCodeCOL, Date: t0},
	}
	domain.SortPackageEventsChronologically(events)

	if !isSortedByDatePkg(events) {
		t.Fatal("package events not sorted after SortPackageEventsChronologically")
	}
	if events[0].Code != domain.EventCodeCOL {
		t.Errorf("events[0].Code = %q, want COL", events[0].Code)
	}
}
