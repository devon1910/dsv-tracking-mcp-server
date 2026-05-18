//go:build integration

package dsv_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv"
)

// Integration tests hit the live DSV public tracking API.
// Run with: go test -race -tags=integration ./internal/upstream/dsv/
//
// Required env vars:
//   DSV_INTEGRATION_REF   — a working reference (STT, waybill, etc.) that resolves
//                           to a valid shipment. Rotate when references decay.
//
// Optional env vars:
//   DSV_INTEGRATION_COOKIE — a valid INGRESSCOOKIE value from a browser session
//                             that has solved the Captcha-Puzzle. Without this,
//                             the API may return 429. See UPSTREAM.md.

func integrationClient(t *testing.T) *dsv.Client {
	t.Helper()
	return dsv.NewClient(dsv.ClientConfig{
		Metrics: obs.NewMetrics(),
	})
}

func integrationRef(t *testing.T) string {
	t.Helper()
	ref := os.Getenv("DSV_INTEGRATION_REF")
	if ref == "" {
		t.Skip("DSV_INTEGRATION_REF not set; skipping integration test")
	}
	return ref
}

func TestIntegration_Search_ValidRef(t *testing.T) {
	ref := integrationRef(t)
	c := integrationClient(t)

	dto, err := c.Search(context.Background(), ref)
	if err != nil {
		if errors.Is(err, domain.ErrThrottled) {
			t.Skip("upstream returned 429 (Captcha-Puzzle); set DSV_INTEGRATION_COOKIE and retry")
		}
		t.Fatalf("Search(%q): %v", ref, err)
	}
	if len(dto.Result) == 0 {
		t.Errorf("Search(%q): got empty result, want at least one match", ref)
	}
	t.Logf("Search(%q): %d result(s), first shipmentID=%q", ref, len(dto.Result), dto.Result[0].ID)
}

func TestIntegration_Detail_ResolvedShipmentID(t *testing.T) {
	ref := integrationRef(t)
	c := integrationClient(t)

	// First resolve the reference to a shipmentID via search.
	searchDTO, err := c.Search(context.Background(), ref)
	if err != nil {
		if errors.Is(err, domain.ErrThrottled) {
			t.Skip("upstream 429; set DSV_INTEGRATION_COOKIE")
		}
		t.Fatalf("Search(%q): %v", ref, err)
	}
	if len(searchDTO.Result) == 0 {
		t.Skipf("Search(%q) returned no results; skipping detail test", ref)
	}

	shipmentID := searchDTO.Result[0].ID
	detailDTO, err := c.Detail(context.Background(), shipmentID)
	if err != nil {
		if errors.Is(err, domain.ErrThrottled) {
			t.Skip("upstream 429; set DSV_INTEGRATION_COOKIE")
		}
		t.Fatalf("Detail(%q): %v", shipmentID, err)
	}
	if detailDTO.STTNumber == "" {
		t.Errorf("Detail returned empty STTNumber")
	}
	if detailDTO.ProgressBar == nil {
		t.Error("Detail returned nil progressBar")
	}
	t.Logf("Detail(%q): STT=%q product=%q progress=%d%%",
		shipmentID, detailDTO.STTNumber, detailDTO.Product, detailDTO.PercentageProgress)

	// Map to domain to verify the full pipeline.
	s, err := dsv.MapShipmentDetail(detailDTO)
	if err != nil {
		t.Fatalf("MapShipmentDetail: %v", err)
	}
	if s.STTNumber == "" {
		t.Error("mapped STTNumber empty")
	}
	t.Logf("Mapped: STT=%q mode=%q activeStep=%q", s.STTNumber, s.TransportMode, s.Progress.ActiveStep)
}

func TestIntegration_Search_KnownBadReference(t *testing.T) {
	integrationRef(t) // ensure DSV_INTEGRATION_REF is set (proves API is accessible)
	c := integrationClient(t)

	// "0000000000" is a structurally valid-looking reference that is known not to
	// exist. The upstream should return either an empty result or a 4xx not-found.
	_, err := c.Search(context.Background(), "0000000000")
	if err != nil {
		if errors.Is(err, domain.ErrThrottled) {
			t.Skip("upstream 429; set DSV_INTEGRATION_COOKIE")
		}
		if !errors.Is(err, domain.ErrShipmentNotFound) && !errors.Is(err, domain.ErrInvalidReference) {
			t.Errorf("unexpected error for bad reference: %v", err)
		}
		// Error case is acceptable for a bad reference.
		t.Logf("bad reference returned error (expected): %v", err)
		return
	}
	// Some upstreams return empty result instead of 4xx for bad references.
	t.Log("bad reference returned empty result (also acceptable)")
}
