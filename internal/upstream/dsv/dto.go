// Package dsv contains the HTTP client, DTOs, and inbound mapper for the
// DSV public tracking API.
package dsv

import "time"

// ─── Error body ──────────────────────────────────────────────────────────────

// errorBodyDTO matches the upstream's 4xx error response body.
type errorBodyDTO struct {
	Message string `json:"message"` // human-readable prose; do not parse
	Code    string `json:"code"`    // stable machine-readable code; e.g. TRACKING-BADREQ-SHIPMENT_NOT_FOUND
}

// ─── Search / summary endpoint ───────────────────────────────────────────────

// SearchResponseDTO matches {"result": [...], "warnings": []}.
type SearchResponseDTO struct {
	Result   []ShipmentSummaryDTO `json:"result"`
	Warnings []string             `json:"warnings"`
}

// ShipmentSummaryDTO matches one item in the search result array.
type ShipmentSummaryDTO struct {
	ID                    string     `json:"id"`   // composite shipmentId
	Stt                   string     `json:"stt"`
	TransportMode         string     `json:"transportMode"`
	PercentageProgress    int        `json:"percentageProgress"`
	LastEventCode         string     `json:"lastEventCode"`
	FromLocation          string     `json:"fromLocation"`
	ToLocation            string     `json:"toLocation"`
	StartDate             *time.Time `json:"startDate"`
	EndDate               *time.Time `json:"endDate"`
	Consignment           *string    `json:"consignment"`
	AdditionalReferenceValues *string `json:"additionalReferenceValues"`
	IsXpress              bool       `json:"isXpress"`
	SwedenViewAvailable   bool       `json:"swedenViewAvailable"`
}

// ─── Detail endpoint ─────────────────────────────────────────────────────────

// ShipmentDetailDTO matches the full detail-endpoint response body.
// Field naming uses correct English spelling; JSON tags preserve upstream quirks.
// Pointer types are used for fields that may be null or are "sometimes populated"
// per UPSTREAM.md. "Never populated" fields are included as pointers to capture
// any future upstream changes without losing data.
type ShipmentDetailDTO struct {
	STTNumber          string          `json:"sttNumber"`
	ShipmentID         string          `json:"shipmentId"`
	TransportMode      string          `json:"transportMode"`
	DataProvider       string          `json:"dataProvider"`
	Product            string          `json:"product"`
	IsXpress           bool            `json:"isXpress"`
	PercentageProgress int             `json:"percentageProgress"`
	Combiterms         *string         `json:"combiterms"`
	References         referencesDTO   `json:"references"`
	Goods              goodsDTO        `json:"goods"`
	Events             []eventDTO      `json:"events"`
	Packages           []packageDTO    `json:"packages"`
	Service            *string         `json:"service"`
	Services           []string        `json:"services"`
	DeliveryDate       deliveryDateDTO `json:"deliveryDate"`
	TransportMode2     *string         `json:"-"` // deduplicated; transportMode covers this
	ProgressBar        *progressBarDTO `json:"progressBar"` // pointer for nil-guard in mapper
	Location           locationDTO     `json:"location"`
	TransportUnits     *string         `json:"transportUnits"` // always null in observed data
}

type referencesDTO struct {
	Shipper                         []string `json:"shipper"`
	Consignee                       []string `json:"consignee"`
	// JSON tag preserves the upstream typo; Go field uses correct spelling.
	WaybillAndConsignmentNumbers    []string `json:"waybillAndConsignementNumbers"`
	AdditionalReferences            []string `json:"additionalReferences"`
	OriginalStt                     *string  `json:"originalStt"`
}

type dimensionDTO struct {
	Length *measurementDTO `json:"length,omitempty"`
	Width  *measurementDTO `json:"width,omitempty"`
	Height *measurementDTO `json:"height,omitempty"`
}

type goodsDTO struct {
	Pieces               int            `json:"pieces"`
	Volume               measurementDTO `json:"volume"`
	Weight               measurementDTO `json:"weight"`
	Dimensions           []dimensionDTO `json:"dimensions"`
	LoadingMeters        measurementDTO `json:"loadingMeters"`
	Stackable            *bool          `json:"stackable"`
	ChargeableWeight     *float64       `json:"chargeableWeight"`
	AgreementDangerousRoad *bool        `json:"agreementDangerousRoad"`
	CustomsDuty          *string        `json:"customsDuty"`
}

type measurementDTO struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type eventDTO struct {
	Code        string          `json:"code"`
	Date        time.Time       `json:"date"`
	CreatedAt   time.Time       `json:"createdAt"`
	Location    eventLocationDTO `json:"location"`
	Comment     string          `json:"comment"`
	Recipient   *string         `json:"recipient"`   // always null in observed data
	Reasons     []eventReasonDTO `json:"reasons"`
	ShellIconName *string        `json:"shellIconName"` // always null in observed data
}

type eventLocationDTO struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	CountryCode string `json:"countryCode"`
}

type eventReasonDTO struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type packageDTO struct {
	ID     string           `json:"id"`
	Events []packageEventDTO `json:"events"`
}

type packageEventDTO struct {
	Code        string    `json:"code"`
	CountryCode string    `json:"countryCode"`
	Location    string    `json:"location"`
	Date        time.Time `json:"date"`
}

type placeDTO struct {
	CountryCode string `json:"countryCode"`
	Country     string `json:"country"`
	City        string `json:"city"`
	PostCode    string `json:"postCode"`
}

type officeDTO struct {
	CountryCode string `json:"countryCode"`
	Country     string `json:"country"`
	City        string `json:"city"`
	// No postCode — see UPSTREAM.md "Location Field Semantics"
}

type locationDTO struct {
	CollectFrom       placeDTO  `json:"collectFrom"`
	DeliverTo         placeDTO  `json:"deliverTo"`
	ShipperPlace      placeDTO  `json:"shipperPlace"`
	ConsigneePlace    placeDTO  `json:"consigneePlace"`
	DispatchingOffice officeDTO `json:"dispatchingOffice"`
	ReceivingOffice   officeDTO `json:"receivingOffice"`
}

type deliveryDateDTO struct {
	Estimated *time.Time `json:"estimated"`
	Agreed    *time.Time `json:"agreed"`
}

type progressBarDTO struct {
	Steps      []string `json:"steps"`
	ActiveStep string   `json:"activeStep"`
}

// ─── Reference types endpoint ────────────────────────────────────────────────

// ReferenceTypeDTO matches one item from the reference-types discovery endpoint.
// Field names are approximate; verify against the live endpoint in Phase 3.
type ReferenceTypeDTO struct {
	ID      string `json:"id"`
	Pattern string `json:"pattern"`
	Label   string `json:"label"`
}
