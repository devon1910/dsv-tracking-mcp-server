package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	mcpinternal "github.com/devon1910/dsv-tracking-mcp-server/internal/mcp"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
)

// assertCode is a helper that decodes the JSON error string from an MCP
// tool result and checks its code field.
func assertToolErrCode(t *testing.T, err error, want mcpinternal.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected non-nil error with code %q", want)
	}
	var te *mcpinternal.ToolError
	if !errors.As(err, &te) {
		t.Fatalf("error is not *ToolError: %T %v", err, err)
	}
	if te.Code != want {
		t.Errorf("Code = %q, want %q", te.Code, want)
	}
}

// ─── Constructor tests ────────────────────────────────────────────────────────

func TestToolError_Constructors(t *testing.T) {
	cases := []struct {
		name          string
		err           error
		wantCode      mcpinternal.ErrorCode
		wantDetailKey string
	}{
		{
			name:     "upstream ErrShipmentNotFound",
			err:      &domain.UpstreamError{Op: "detail", Err: domain.ErrShipmentNotFound},
			wantCode: mcpinternal.CodeShipmentNotFound,
		},
		{
			name:     "upstream ErrInvalidReference",
			err:      &domain.UpstreamError{Op: "search", Err: domain.ErrInvalidReference},
			wantCode: mcpinternal.CodeShipmentNotFound,
		},
		{
			name:     "upstream ErrUpstreamUnavailable",
			err:      &domain.UpstreamError{Op: "detail", Err: domain.ErrUpstreamUnavailable},
			wantCode: mcpinternal.CodeUpstreamUnavailable,
		},
		{
			name:     "upstream ErrThrottled",
			err:      &domain.UpstreamError{Op: "detail", Err: domain.ErrThrottled},
			wantCode: mcpinternal.CodeUpstreamUnavailable,
		},
		{
			name:     "upstream ErrMalformedResponse",
			err:      &domain.UpstreamError{Op: "detail", Err: domain.ErrMalformedResponse},
			wantCode: mcpinternal.CodeUpstreamError,
		},
		{
			name:     "upstream ErrUnsupportedTransportMode",
			err:      &domain.UpstreamError{Op: "detail", Err: domain.ErrUnsupportedTransportMode},
			wantCode: mcpinternal.CodeUpstreamError,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			wantCode: mcpinternal.CodeUpstreamTimeout,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			wantCode: mcpinternal.CodeUpstreamTimeout,
		},
		{
			name:          "unknown upstream error",
			err:           errors.New("something weird"),
			wantCode:      mcpinternal.CodeUpstreamError,
			wantDetailKey: "upstream_message",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			te := mcpinternal.ErrFromUpstream(tc.err)
			if te.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", te.Code, tc.wantCode)
			}
			if tc.wantDetailKey != "" {
				if te.Details == nil {
					t.Errorf("Details is nil, want key %q", tc.wantDetailKey)
				} else if _, ok := te.Details[tc.wantDetailKey]; !ok {
					t.Errorf("Details missing key %q; got %v", tc.wantDetailKey, te.Details)
				}
			}
		})
	}
}

// TestToolError_JSONRoundTrip verifies the shape the MCP SDK puts in TextContent.
func TestToolError_JSONRoundTrip(t *testing.T) {
	te := &mcpinternal.ToolError{
		Code:    mcpinternal.CodeShipmentNotFound,
		Message: `no shipment found for id "LandStt:X:CTTS:LAND"`,
	}
	raw := te.Error() // should be JSON

	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("ToolError.Error() is not valid JSON: %v\nraw: %s", err, raw)
	}
	if m["code"] != string(mcpinternal.CodeShipmentNotFound) {
		t.Errorf("code = %v, want %q", m["code"], mcpinternal.CodeShipmentNotFound)
	}
	if m["message"] == "" {
		t.Error("message is empty")
	}
}

// TestToolError_ErrorsAs verifies *ToolError implements errors.As correctly.
func TestToolError_ErrorsAs(t *testing.T) {
	original := &mcpinternal.ToolError{Code: mcpinternal.CodeInvalidInput, Message: "field: required"}
	wrapped := &wrappedError{err: original}

	var te *mcpinternal.ToolError
	if !errors.As(wrapped, &te) {
		t.Error("errors.As(*ToolError) returned false through one-level wrap")
	}
	if te.Code != mcpinternal.CodeInvalidInput {
		t.Errorf("Code = %q", te.Code)
	}
}

type wrappedError struct{ err error }

func (w *wrappedError) Error() string  { return w.err.Error() }
func (w *wrappedError) Unwrap() error  { return w.err }

// TestTools_ErrorCodesUsed verifies that track_shipment and get_shipment_details
// return the expected taxonomy codes for each error scenario. This is an
// integration check that tools.go uses errors.go constructors, not bare strings.
func TestTools_ErrorCodesUsed(t *testing.T) {
	// These are thin wrappers over the existing table-driven tests in tools_test.go.
	// The purpose here is to assert Code fields explicitly.

	srv := mcpinternal.New(noopLogger(), obs.NewMetrics())
	mcpinternal.RegisterAll(srv, newDeps(&fakeUpstream{}))

	t.Run("track_shipment empty reference → INVALID_INPUT", func(t *testing.T) {
		r := callTool(t, srv, "track_shipment", map[string]any{"reference": ""})
		if !r.IsError {
			t.Fatal("expected IsError=true")
		}
		var m map[string]any
		json.Unmarshal([]byte(firstText(r)), &m)
		if m["code"] != string(mcpinternal.CodeInvalidInput) {
			t.Errorf("code = %v, want %q", m["code"], mcpinternal.CodeInvalidInput)
		}
	})

	t.Run("get_shipment_details malformed id → INVALID_SHIPMENT_ID", func(t *testing.T) {
		r := callTool(t, srv, "get_shipment_details", map[string]any{"shipment_id": "bad"})
		if !r.IsError {
			t.Fatal("expected IsError=true")
		}
		var m map[string]any
		json.Unmarshal([]byte(firstText(r)), &m)
		if m["code"] != string(mcpinternal.CodeInvalidShipmentID) {
			t.Errorf("code = %v, want %q", m["code"], mcpinternal.CodeInvalidShipmentID)
		}
	})

	t.Run("get_shipment_details not found → SHIPMENT_NOT_FOUND", func(t *testing.T) {
		up := &fakeUpstream{detailErr: &domain.UpstreamError{Err: domain.ErrShipmentNotFound}}
		srv2 := mcpinternal.New(noopLogger(), obs.NewMetrics())
		mcpinternal.RegisterAll(srv2, newDeps(up))
		r := callTool(t, srv2, "get_shipment_details", map[string]any{"shipment_id": "LandStt:X:CTTS:LAND"})
		if !r.IsError {
			t.Fatal("expected IsError=true")
		}
		var m map[string]any
		json.Unmarshal([]byte(firstText(r)), &m)
		if m["code"] != string(mcpinternal.CodeShipmentNotFound) {
			t.Errorf("code = %v, want %q", m["code"], mcpinternal.CodeShipmentNotFound)
		}
	})
}
