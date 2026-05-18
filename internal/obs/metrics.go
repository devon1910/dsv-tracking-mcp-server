package obs

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds Prometheus counters and histograms for MCP tool calls.
// Each instance has its own registry; there is no global state.
type Metrics struct {
	ToolCalls   *prometheus.CounterVec
	ToolLatency *prometheus.HistogramVec
	reg         *prometheus.Registry
}

// NewMetrics constructs and registers MCP metrics on a fresh Prometheus registry.
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
	}
	reg.MustRegister(m.ToolCalls, m.ToolLatency)
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
