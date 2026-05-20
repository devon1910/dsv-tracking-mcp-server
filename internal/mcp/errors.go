package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
)

// ErrorCode is the machine-readable code included in every tool error.
// LLM callers use this to decide how to handle the error without parsing
// the human-readable Message field. See docs/ERROR_CODES.md for the full
// taxonomy including retry guidance.
type ErrorCode string

const (
	// CodeInvalidInput is returned when a required field is missing or empty.
	CodeInvalidInput ErrorCode = "INVALID_INPUT"
	// CodeInvalidReferenceType is returned when reference_type is not in the catalog.
	CodeInvalidReferenceType ErrorCode = "INVALID_REFERENCE_TYPE"
	// CodeInvalidShipmentID is returned when shipment_id doesn't match the composite form.
	CodeInvalidShipmentID ErrorCode = "INVALID_SHIPMENT_ID"
	// CodeShipmentNotFound is returned when DSV confirms no shipment matches the reference.
	CodeShipmentNotFound ErrorCode = "SHIPMENT_NOT_FOUND"
	// CodeUpstreamError is returned for unexpected DSV errors.
	CodeUpstreamError ErrorCode = "UPSTREAM_ERROR"
	// CodeUpstreamTimeout is returned when the browser fetch exceeds its deadline.
	CodeUpstreamTimeout ErrorCode = "UPSTREAM_TIMEOUT"
	// CodeUpstreamUnavailable is returned when DSV is unreachable after retries.
	CodeUpstreamUnavailable ErrorCode = "UPSTREAM_UNAVAILABLE"
	// CodeInternalError is returned for bugs in this server (mapper panics, etc.).
	CodeInternalError ErrorCode = "INTERNAL_ERROR"
)

// ToolError is the structured error type returned by all MCP tool handlers.
// When returned as a Go error the MCP SDK wraps it in CallToolResult{IsError:true}
// with the JSON-encoded representation in TextContent.
type ToolError struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *ToolError) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// ─── constructor helpers (one per code) ──────────────────────────────────────

func errInvalidInput(field, reason string) *ToolError {
	return &ToolError{Code: CodeInvalidInput, Message: fmt.Sprintf("%s: %s", field, reason)}
}

func errInvalidReferenceType(given string, valid []string) *ToolError {
	return &ToolError{
		Code:    CodeInvalidReferenceType,
		Message: fmt.Sprintf("%q is not a valid reference_type", given),
		Details: map[string]any{"valid_codes": valid},
	}
}

func errInvalidShipmentID(received string) *ToolError {
	return &ToolError{
		Code:    CodeInvalidShipmentID,
		Message: "shipment_id must have the form Provider:Ref:DataProvider:Mode (e.g. LandStt:VAN5022058:CTTS:LAND)",
		Details: map[string]any{"received": received},
	}
}

func errShipmentNotFound(shipmentID string) *ToolError {
	return &ToolError{
		Code:    CodeShipmentNotFound,
		Message: fmt.Sprintf("no shipment found for id %q", shipmentID),
	}
}

// ErrFromUpstream maps an upstream error to the appropriate ToolError.
// Exported so errors_test.go can call it directly to assert the mapping.
func ErrFromUpstream(err error) *ToolError {
	return errFromUpstream(err)
}

func errFromUpstream(err error) *ToolError {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &ToolError{Code: CodeUpstreamTimeout, Message: "browser fetch exceeded deadline"}
	case errors.Is(err, domain.ErrShipmentNotFound):
		return &ToolError{Code: CodeShipmentNotFound, Message: err.Error()}
	case errors.Is(err, domain.ErrUpstreamUnavailable):
		return &ToolError{
			Code:    CodeUpstreamUnavailable,
			Message: "DSV is unreachable",
			Details: map[string]any{"upstream_message": err.Error()},
		}
	case errors.Is(err, domain.ErrThrottled):
		return &ToolError{
			Code:    CodeUpstreamUnavailable,
			Message: "DSV throttled the request",
			Details: map[string]any{"upstream_message": err.Error()},
		}
	default:
		return &ToolError{
			Code:    CodeUpstreamError,
			Message: "DSV returned an unexpected error",
			Details: map[string]any{"upstream_message": err.Error()},
		}
	}
}

func errInternal(msg string) *ToolError {
	return &ToolError{Code: CodeInternalError, Message: msg}
}
