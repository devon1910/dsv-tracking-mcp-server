package dsv_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func clientTestdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata")
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(clientTestdataDir(t), name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// noRetryClient creates a client pointing at srv with MaxRetries=0 and no logger noise.
func noRetryClient(t *testing.T, srv *httptest.Server) *dsv.Client {
	t.Helper()
	return dsv.NewClient(dsv.ClientConfig{
		BaseURL:    srv.URL,
		Timeout:    2 * time.Second,
		MaxRetries: 0,
		Metrics:    obs.NewMetrics(),
	})
}

// fastRetryClient creates a client with MaxRetries=2 and very short timeout so
// retry tests don't take long.
func fastRetryClient(t *testing.T, srv *httptest.Server) *dsv.Client {
	t.Helper()
	return dsv.NewClient(dsv.ClientConfig{
		BaseURL:    srv.URL,
		Timeout:    500 * time.Millisecond,
		MaxRetries: 2,
		Metrics:    obs.NewMetrics(),
	})
}

// ─── Search tests ─────────────────────────────────────────────────────────────

func TestClient_Search_Success(t *testing.T) {
	fixture := readFixture(t, "search_single_result.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nges-portal/api/public/tracking-public/shipments" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "LKG6022524" {
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	c := noRetryClient(t, srv)
	dto, err := c.Search(context.Background(), "LKG6022524")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(dto.Result) != 1 {
		t.Errorf("len(Result) = %d, want 1", len(dto.Result))
	}
	if dto.Result[0].Stt != "LKG6022524" {
		t.Errorf("Result[0].Stt = %q, want LKG6022524", dto.Result[0].Stt)
	}
}

func TestClient_Detail_Success(t *testing.T) {
	fixture := readFixture(t, "delivered_ltl_se_fr.json")
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	c := noRetryClient(t, srv)
	shipmentID := "LandStt:VAN5022058:CTTS:LAND"
	dto, err := c.Detail(context.Background(), shipmentID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if dto.STTNumber != "VAN5022058" {
		t.Errorf("STTNumber = %q", dto.STTNumber)
	}
	// Verify the URL path contains the raw shipmentID with unencoded colons.
	wantSuffix := "/shipments/land/LandStt:VAN5022058:CTTS:LAND"
	if !hasPathSuffix(capturedPath, wantSuffix) {
		t.Errorf("path = %q, want suffix %q", capturedPath, wantSuffix)
	}
}

func hasPathSuffix(path, suffix string) bool {
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}

// ─── Error mapping tests ──────────────────────────────────────────────────────

func TestClient_NotFound(t *testing.T) {
	fixture := readFixture(t, "error_shipment_not_found.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	c := noRetryClient(t, srv)
	_, err := c.Search(context.Background(), "18062908291")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrShipmentNotFound) {
		t.Errorf("errors.Is(ErrShipmentNotFound) = false; got %v", err)
	}
	var ue *domain.UpstreamError
	if !errors.As(err, &ue) {
		t.Fatalf("errors.As(*UpstreamError) false")
	}
	if ue.UpstreamCode != "TRACKING-BADREQ-SHIPMENT_NOT_FOUND" {
		t.Errorf("UpstreamCode = %q", ue.UpstreamCode)
	}
	if ue.HTTPStatus != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d, want 400", ue.HTTPStatus)
	}
}

func TestClient_4xx_OtherCode_MapsToInvalidReference(t *testing.T) {
	body := `{"message":"bad input","code":"TRACKING-BADREQ-UNKNOWN_CODE"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := noRetryClient(t, srv)
	_, err := c.Search(context.Background(), "bad-ref")
	if !errors.Is(err, domain.ErrInvalidReference) {
		t.Errorf("errors.Is(ErrInvalidReference) = false; got %v", err)
	}
	var ue *domain.UpstreamError
	if errors.As(err, &ue) && ue.UpstreamCode != "TRACKING-BADREQ-UNKNOWN_CODE" {
		t.Errorf("UpstreamCode = %q", ue.UpstreamCode)
	}
}

func TestClient_429_MapsToThrottled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := noRetryClient(t, srv)
	_, err := c.Search(context.Background(), "any")
	if !errors.Is(err, domain.ErrThrottled) {
		t.Errorf("errors.Is(ErrThrottled) = false; got %v", err)
	}
	var ue *domain.UpstreamError
	if !errors.As(err, &ue) {
		t.Fatalf("errors.As(*UpstreamError) false")
	}
	if ue.HTTPStatus != http.StatusTooManyRequests {
		t.Errorf("HTTPStatus = %d, want 429", ue.HTTPStatus)
	}
}

func TestClient_5xx_RetriesAndReturnsUnavailable(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := fastRetryClient(t, srv)
	_, err := c.Search(context.Background(), "any")
	if !errors.Is(err, domain.ErrUpstreamUnavailable) {
		t.Errorf("errors.Is(ErrUpstreamUnavailable) = false; got %v", err)
	}
	// MaxRetries=2 means 3 total attempts.
	if n := atomic.LoadInt32(&callCount); n != 3 {
		t.Errorf("callCount = %d, want 3 (1 initial + 2 retries)", n)
	}
}

func TestClient_MalformedJSON_MapsToMalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{this is not json`))
	}))
	defer srv.Close()

	c := noRetryClient(t, srv)
	_, err := c.Search(context.Background(), "any")
	if !errors.Is(err, domain.ErrMalformedResponse) {
		t.Errorf("errors.Is(ErrMalformedResponse) = false; got %v", err)
	}
}

func TestClient_ContextCancellationStopsRetries(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately — the first call should still complete (we cancel during backoff).
	cancel()

	c := fastRetryClient(t, srv)
	_, err := c.Search(ctx, "any")
	if err == nil {
		t.Fatal("expected error after cancellation")
	}
	// The error should be context-related or ErrUpstreamUnavailable
	// (depending on whether cancellation fires before or after the first attempt).
	if !errors.Is(err, context.Canceled) && !errors.Is(err, domain.ErrUpstreamUnavailable) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClient_RequestIDPropagatedAsHeader(t *testing.T) {
	var capturedReqID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReqID = r.Header.Get("X-Request-ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[],"warnings":[]}`))
	}))
	defer srv.Close()

	ctx := obs.WithRequestID(context.Background())
	wantID := obs.RequestIDFromContext(ctx)

	c := noRetryClient(t, srv)
	_, _ = c.Search(ctx, "any")

	if capturedReqID != wantID {
		t.Errorf("X-Request-ID header = %q, want %q", capturedReqID, wantID)
	}
}

func TestClient_ConnectionRefused_RetriesAndReturnsUnavailable(t *testing.T) {
	// Use a URL that is guaranteed to refuse connections.
	c := dsv.NewClient(dsv.ClientConfig{
		BaseURL:    "http://127.0.0.1:1", // port 1 is never open
		Timeout:    200 * time.Millisecond,
		MaxRetries: 1,
		Metrics:    obs.NewMetrics(),
	})
	_, err := c.Search(context.Background(), "any")
	if !errors.Is(err, domain.ErrUpstreamUnavailable) {
		t.Errorf("errors.Is(ErrUpstreamUnavailable) = false; got %v", err)
	}
}

func TestClient_UserAgentHeader(t *testing.T) {
	var capturedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[],"warnings":[]}`))
	}))
	defer srv.Close()

	c := noRetryClient(t, srv)
	_, _ = c.Search(context.Background(), "any")
	if capturedUA != "dsv-tracking-mcp-server/0.1.0" {
		t.Errorf("User-Agent = %q, want dsv-tracking-mcp-server/0.1.0", capturedUA)
	}
}
