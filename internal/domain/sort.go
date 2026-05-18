package domain

import "slices"

// SortEventsChronologically sorts events by Date ascending, in place.
//
// The upstream API returns events in ingestion order, not chronological order.
// See UPSTREAM.md "Event Array Ordering" for the observed example: in
// dispatching_parcel_se_se.json the ENT (Booked) event has a timestamp of
// 15:24 appearing after COL (14:00) and ENM (14:01) because the booking record
// was created after the physical collection. The mapper calls this function
// before constructing a Shipment so that consumers always receive events in
// time order.
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
