package obs

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds Prometheus counters and histograms for the server.
// Each instance has its own registry; there is no global state.
type Metrics struct {
	// MCP tool call metrics.
	ToolCalls   *prometheus.CounterVec
	ToolLatency *prometheus.HistogramVec

	// DSV upstream request metrics.
	DSVRequests *prometheus.CounterVec   // labels: endpoint, status
	DSVLatency  *prometheus.HistogramVec // labels: endpoint

	reg *prometheus.Registry
}

// NewMetrics constructs and registers all metrics on a fresh Prometheus registry.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		ToolCalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mcp_tool_calls_total",
			Help: "Total MCP tool calls partitioned by tool name and status.",
		}, []string{"tool", "status"}),
		ToolLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "mcp_tool_call_duration_seconds",
			Help:    "Latency of MCP tool calls in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"tool"}),
		DSVRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dsv_upstream_requests_total",
			Help: "Total DSV upstream HTTP requests partitioned by endpoint and HTTP status class.",
		}, []string{"endpoint", "status"}),
		DSVLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "dsv_upstream_latency_seconds",
			Help:    "Latency of DSV upstream HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"endpoint"}),
	}
	reg.MustRegister(m.ToolCalls, m.ToolLatency, m.DSVRequests, m.DSVLatency)
	return m
}

// MetricsServer returns an *http.Server that serves /metrics and /healthz on addr.
func MetricsServer(addr string, m *Metrics) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return &http.Server{Addr: addr, Handler: mux}
}
