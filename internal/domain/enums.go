package domain

// TransportMode is the mode of transport for a shipment.
type TransportMode string

const (
	// TransportModeLand is the only mode observed in current samples.
	TransportModeLand TransportMode = "LAND"
	// TransportModeSea, Air, Rail are known modes not yet in scope for v1.
	TransportModeSea  TransportMode = "SEA"
	TransportModeAir  TransportMode = "AIR"
	TransportModeRail TransportMode = "RAIL"
	// TransportModeUnknown is the fallback for unrecognised values.
	TransportModeUnknown TransportMode = ""
)

// String returns the raw upstream code.
func (m TransportMode) String() string { return string(m) }

// IsKnown reports whether the mode is one of the defined constants.
func (m TransportMode) IsKnown() bool {
	switch m {
	case TransportModeLand, TransportModeSea, TransportModeAir, TransportModeRail:
		return true
	}
	return false
}

// Description returns a stable human-readable label for the mode.
func (m TransportMode) Description() string {
	switch m {
	case TransportModeLand:
		return "Land"
	case TransportModeSea:
		return "Sea"
	case TransportModeAir:
		return "Air"
	case TransportModeRail:
		return "Rail"
	default:
		return "Unknown"
	}
}

// ParseTransportMode returns the TransportMode for s, or TransportModeUnknown.
func ParseTransportMode(s string) TransportMode {
	switch TransportMode(s) {
	case TransportModeLand, TransportModeSea, TransportModeAir, TransportModeRail:
		return TransportMode(s)
	}
	return TransportModeUnknown
}

// ─── EventCode ────────────────────────────────────────────────────────────────

// EventCode is the stable machine-readable code for a shipment event.
// The upstream also provides a human-readable "comment" field, but that is
// English prose and not part of the API contract.
type EventCode string

const (
	EventCodeENT     EventCode = "ENT" // Booked
	EventCodeCOL     EventCode = "COL" // Collected
	EventCodeENM     EventCode = "ENM" // Arrived at hub
	EventCodeMAN     EventCode = "MAN" // Departed
	EventCodeDOT     EventCode = "DOT" // Out for delivery
	EventCodeDLV     EventCode = "DLV" // Delivered
	EventCodeNLO     EventCode = "NLO" // Loaded for transport (package-level only)
	EventCodeDIS     EventCode = "DIS" // To consignee's disposal
	EventCodeUnknown EventCode = ""
)

// String returns the raw upstream code.
func (c EventCode) String() string { return string(c) }

// IsKnown reports whether the code is one of the defined constants.
func (c EventCode) IsKnown() bool {
	switch c {
	case EventCodeENT, EventCodeCOL, EventCodeENM, EventCodeMAN,
		EventCodeDOT, EventCodeDLV, EventCodeNLO, EventCodeDIS:
		return true
	}
	return false
}

// Description returns a stable human-readable label. Unlike the upstream's
// "comment" field, this label is part of the domain contract.
func (c EventCode) Description() string {
	switch c {
	case EventCodeENT:
		return "Booked"
	case EventCodeCOL:
		return "Collected"
	case EventCodeENM:
		return "Arrived at hub"
	case EventCodeMAN:
		return "Departed"
	case EventCodeDOT:
		return "Out for delivery"
	case EventCodeDLV:
		return "Delivered"
	case EventCodeNLO:
		return "Loaded for transport"
	case EventCodeDIS:
		return "To consignee's disposal"
	default:
		return "Unknown"
	}
}

// ParseEventCode returns the EventCode for s, or EventCodeUnknown.
func ParseEventCode(s string) EventCode {
	switch EventCode(s) {
	case EventCodeENT, EventCodeCOL, EventCodeENM, EventCodeMAN,
		EventCodeDOT, EventCodeDLV, EventCodeNLO, EventCodeDIS:
		return EventCode(s)
	}
	return EventCodeUnknown
}

// ─── ProgressStage ────────────────────────────────────────────────────────────

// ProgressStage represents a step in the shipment lifecycle.
type ProgressStage string

const (
	ProgressStageBooked            ProgressStage = "BOOKED"
	ProgressStageTransportation    ProgressStage = "TRANSPORTATION"
	ProgressStageDispatchingCenter ProgressStage = "DISPATCHING_CENTER"
	ProgressStageInDelivery        ProgressStage = "IN_DELIVERY"
	ProgressStageDelivered         ProgressStage = "DELIVERED"
	ProgressStageUnknown           ProgressStage = ""
)

// String returns the raw upstream stage code.
func (s ProgressStage) String() string { return string(s) }

// IsKnown reports whether the stage is one of the defined constants.
func (s ProgressStage) IsKnown() bool {
	switch s {
	case ProgressStageBooked, ProgressStageTransportation,
		ProgressStageDispatchingCenter, ProgressStageInDelivery, ProgressStageDelivered:
		return true
	}
	return false
}

// Description returns a stable human-readable label for the stage.
func (s ProgressStage) Description() string {
	switch s {
	case ProgressStageBooked:
		return "Booked"
	case ProgressStageTransportation:
		return "In transportation"
	case ProgressStageDispatchingCenter:
		return "At dispatching centre"
	case ProgressStageInDelivery:
		return "Out for delivery"
	case ProgressStageDelivered:
		return "Delivered"
	default:
		return "Unknown"
	}
}

// ParseProgressStage returns the ProgressStage for s, or ProgressStageUnknown.
func ParseProgressStage(s string) ProgressStage {
	switch ProgressStage(s) {
	case ProgressStageBooked, ProgressStageTransportation,
		ProgressStageDispatchingCenter, ProgressStageInDelivery, ProgressStageDelivered:
		return ProgressStage(s)
	}
	return ProgressStageUnknown
}
