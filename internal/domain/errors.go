package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors for categorical failure cases. Callers use errors.Is to
// match these; use UpstreamError.Unwrap to retrieve them from wrapped errors.
var (
	// ErrShipmentNotFound is returned when the upstream confirms no shipment
	// matches the given reference (upstream code TRACKING-BADREQ-SHIPMENT_NOT_FOUND).
	ErrShipmentNotFound = errors.New("shipment not found")

	// ErrInvalidReference is returned when the reference string fails
	// structural validation before the upstream is contacted.
	ErrInvalidReference = errors.New("invalid reference format")

	// ErrUpstreamUnavailable is returned when the upstream is unreachable or
	// returns a 5xx response.
	ErrUpstreamUnavailable = errors.New("upstream unavailable")

	// ErrThrottled is returned when the upstream signals rate limiting.
	ErrThrottled = errors.New("upstream throttled request")

	// ErrMalformedResponse is returned when the upstream returns a 2xx with a
	// body that cannot be mapped to a valid domain type.
	ErrMalformedResponse = errors.New("upstream returned malformed response")

	// ErrUnsupportedTransportMode is returned when the upstream detail response
	// contains a transport mode the adapter does not yet handle (SEA, AIR, RAIL).
	ErrUnsupportedTransportMode = errors.New("transport mode not supported")
)

// UpstreamError wraps a sentinel error with upstream-specific context.
// Use errors.Is(err, ErrShipmentNotFound) for categorical matching and
// errors.As(err, &upstreamErr) to retrieve the full context.
type UpstreamError struct {
	// Op is the operation that failed, e.g. "lookup_shipment", "get_detail".
	Op string
	// Reference is the input reference that was being resolved, if applicable.
	Reference string
	// UpstreamCode is the upstream's machine-readable error code,
	// e.g. "TRACKING-BADREQ-SHIPMENT_NOT_FOUND".
	UpstreamCode string
	// HTTPStatus is the HTTP status code returned by the upstream.
	HTTPStatus int
	// Err is the sentinel error this wraps.
	Err error
}

// Error implements the error interface.
func (e *UpstreamError) Error() string {
	if e.Reference != "" {
		return fmt.Sprintf("%s [%s]: %v (upstream_code=%q, http=%d)",
			e.Op, e.Reference, e.Err, e.UpstreamCode, e.HTTPStatus)
	}
	return fmt.Sprintf("%s: %v (upstream_code=%q, http=%d)",
		e.Op, e.Err, e.UpstreamCode, e.HTTPStatus)
}

// Unwrap returns the sentinel error for use with errors.Is.
func (e *UpstreamError) Unwrap() error { return e.Err }
