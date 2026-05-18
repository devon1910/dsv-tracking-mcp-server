package domain

import "slices"

// SortEventsChronologically sorts events by Date ascending, in place.
//
// The upstream's contract makes no guarantee about event ordering. All 7
// observed fixtures happened to deliver events sorted by date ascending, but
// the mapper sorts defensively so that consumers receive a stable chronological
// contract regardless of future upstream behaviour. See UPSTREAM.md
// "Event Array Ordering" for details on the semantic vs. positional distinction.
func SortEventsChronologically(events []Event) {
	slices.SortStableFunc(events, func(a, b Event) int {
		return a.Date.Compare(b.Date)
	})
}

// SortPackageEventsChronologically sorts package-level events by Date ascending,
// in place. Same motivation as SortEventsChronologically.
func SortPackageEventsChronologically(events []PackageEvent) {
	slices.SortStableFunc(events, func(a, b PackageEvent) int {
		return a.Date.Compare(b.Date)
	})
}
