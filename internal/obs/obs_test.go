package obs_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
)

func TestNewLogger(t *testing.T) {
	l := obs.NewLogger()
	if l == nil {
		t.Fatal("NewLogger returned nil")
	}
}

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := context.Background()
	if id := obs.RequestIDFromContext(ctx); id != "" {
		t.Fatalf("expected empty string, got %q", id)
	}

	ctx2 := obs.WithRequestID(ctx)
	id1 := obs.RequestIDFromContext(ctx2)
	if id1 == "" {
		t.Fatal("expected non-empty request ID after WithRequestID")
	}
}

func TestRequestIDUniqueness(t *testing.T) {
	ctx := context.Background()
	id1 := obs.RequestIDFromContext(obs.WithRequestID(ctx))
	id2 := obs.RequestIDFromContext(obs.WithRequestID(ctx))
	if id1 == id2 {
		t.Fatalf("expected unique IDs, got %q twice", id1)
	}
}

func TestNewMetrics(t *testing.T) {
	m := obs.NewMetrics()
	if m.ToolCalls == nil {
		t.Fatal("ToolCalls is nil")
	}
	if m.ToolLatency == nil {
		t.Fatal("ToolLatency is nil")
	}
	// Smoke-test: must not panic.
	m.ToolCalls.WithLabelValues("greet", "success").Inc()
	m.ToolLatency.WithLabelValues("greet").Observe(0.042)
}

func TestMetricsServerEndpoints(t *testing.T) {
	m := obs.NewMetrics()
	srv := obs.MetricsServer(":0", m)

	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	for _, path := range []string{"/healthz", "/metrics"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200", path, resp.StatusCode)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
	}
}
