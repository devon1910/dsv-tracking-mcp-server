package domain

import "regexp"

// ReferenceType identifies which of DSV's 21 reference categories an input
// string belongs to. A single input string typically matches multiple types;
// the upstream search endpoint resolves the ambiguity server-side.
type ReferenceType string

// The 21 reference types exposed by the upstream discovery endpoint.
const (
	ReferenceTypeSTT                    ReferenceType = "Stt"
	ReferenceTypeWaybillNo              ReferenceType = "WaybillNo"
	ReferenceTypeShippersRefNo          ReferenceType = "ShippersRefNo"
	ReferenceTypePackageID              ReferenceType = "PackageId"
	ReferenceTypeConsigneesRefNo        ReferenceType = "ConsigneesRefNo"
	ReferenceTypeHawb                   ReferenceType = "Hawb"
	ReferenceTypeBookingID              ReferenceType = "BookingId"
	ReferenceTypeCustomerBookingRef     ReferenceType = "CustomerBookingRef"
	ReferenceTypePurchaseOrderNo        ReferenceType = "PurchaseOrderNo"
	ReferenceTypeHbl                    ReferenceType = "Hbl"
	ReferenceTypeContainerNo            ReferenceType = "ContainerNo"
	ReferenceTypeATOL                   ReferenceType = "ATOL"
	ReferenceTypeCOS                    ReferenceType = "COS"
	ReferenceTypeShippingOrderNumber    ReferenceType = "ShippingOrderNumber"
	ReferenceTypeMovementReferenceNumber ReferenceType = "MovementReferenceNumber"
	ReferenceTypeShipmentNoExportImport ReferenceType = "ShipmentNoExportImport"
	ReferenceTypeSalesOrderNumber       ReferenceType = "SalesOrderNumber"
	ReferenceTypeDeliveryOrderNumber    ReferenceType = "DeliveryOrderNumber"
	ReferenceTypePackageNumber          ReferenceType = "PackageNumber"
	ReferenceTypeAssetID                ReferenceType = "AssetId"
	ReferenceTypeJobID                  ReferenceType = "JobId"
)

// referencePatterns maps each type to its compiled regex.
//
// NOTE: These patterns approximate the upstream /reference-types discovery
// endpoint. They are intentionally permissive: a given reference string will
// typically match several types, and the upstream resolves the ambiguity.
// Phase 3 should verify these against the live endpoint response and update
// them if the upstream patterns differ materially.
var referencePatterns = map[ReferenceType]*regexp.Regexp{
	// STT numbers: 2-8 uppercase letters followed by 5-12 digits.
	// Observed: VAN5022058, SEKSD620143489, SESTO620296604.
	ReferenceTypeSTT: regexp.MustCompile(`^[A-Z]{2,8}\d{5,12}$`),

	// Waybill numbers: 7-12 digit numeric strings.
	// Observed: 3476257542 (10 digits), 03368220 (8 digits).
	ReferenceTypeWaybillNo: regexp.MustCompile(`^\d{7,12}$`),

	// Package IDs: 15-20 digit numeric strings (DSV internal asset IDs).
	// Observed: 573313432229014382 (18 digits).
	ReferenceTypePackageID: regexp.MustCompile(`^\d{15,20}$`),

	// Movement Reference Number: typically 14 digits.
	ReferenceTypeMovementReferenceNumber: regexp.MustCompile(`^\d{14}$`),

	// Container numbers follow the ISO 6346 format: 4 uppercase letters + 7 digits.
	ReferenceTypeContainerNo: regexp.MustCompile(`^[A-Z]{4}\d{7}$`),

	// All remaining types are broadly permissive: 1-100 non-whitespace characters.
	// These will match almost any non-empty reference string.
	ReferenceTypeShippersRefNo:          regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeConsigneesRefNo:        regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeHawb:                   regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeBookingID:              regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeCustomerBookingRef:     regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypePurchaseOrderNo:        regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeHbl:                    regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeATOL:                   regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeCOS:                    regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeShippingOrderNumber:    regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeShipmentNoExportImport: regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeSalesOrderNumber:       regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeDeliveryOrderNumber:    regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypePackageNumber:          regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeAssetID:                regexp.MustCompile(`^\S.{0,99}$`),
	ReferenceTypeJobID:                  regexp.MustCompile(`^\S.{0,99}$`),
}

// ReferenceTypePattern returns the compiled regex for t, or nil if t is unknown.
func ReferenceTypePattern(t ReferenceType) *regexp.Regexp {
	return referencePatterns[t]
}

// InferReferenceTypes returns the reference types whose pattern matches ref.
// The result may be empty (no known type matched) or contain multiple types
// (the common case for permissive patterns). The upstream search endpoint
// resolves which type is authoritative.
func InferReferenceTypes(ref string) []ReferenceType {
	var matches []ReferenceType
	for t, re := range referencePatterns {
		if re.MatchString(ref) {
			matches = append(matches, t)
		}
	}
	return matches
}
