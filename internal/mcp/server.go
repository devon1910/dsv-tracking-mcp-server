// Package mcp is a thin wrapper around the official MCP SDK that adds
// per-request observability (request IDs, structured logging, Prometheus metrics).
package mcp

import (
	"context"
	"log/slog"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
)

const version = "0.1.0"

// Server wraps the MCP SDK server with cross-cutting observability.
type Server struct {
	sdk     *sdkmcp.Server
	logger  *slog.Logger
	metrics *obs.Metrics
}

// New constructs a Server with the given logger and metrics.
// The underlying MCP server is configured with name "dsv-tracking-mcp-server"
// and version 0.1.0. No tools are registered at construction time.
func New(logger *slog.Logger, metrics *obs.Metrics) *Server {
	sdk := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "dsv-tracking-mcp-server", Version: version},
		&sdkmcp.ServerOptions{Logger: logger},
	)

	s := &Server{sdk: sdk, logger: logger, metrics: metrics}
	sdk.AddReceivingMiddleware(s.observingMiddleware)
	return s
}

// Run starts the MCP server over stdio and blocks until ctx is cancelled or
// the client disconnects.
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("MCP server starting", "transport", "stdio", "version", version)
	return s.sdk.Run(ctx, &sdkmcp.StdioTransport{})
}

// RegisterTool adds a typed tool to the underlying SDK server. Cross-cutting
// concerns (request IDs, logging, metrics) are provided by the receiving
// middleware installed at construction time.
func RegisterTool[In, Out any](s *Server, t *sdkmcp.Tool, h sdkmcp.ToolHandlerFor[In, Out]) {
	sdkmcp.AddTool(s.sdk, t, h)
}

// observingMiddleware is a receiving middleware that:
//   - Injects a request ID into every incoming request's context.
//   - For tools/call requests: logs entry at debug, result at info/error,
//     and records ToolCalls and ToolLatency metrics.
func (s *Server) observingMiddleware(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
	return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
		ctx = obs.WithRequestID(ctx)
		reqID := obs.RequestIDFromContext(ctx)

		if method != "tools/call" {
			return next(ctx, method, req)
		}

		toolName := "unknown"
		if p, ok := req.GetParams().(*sdkmcp.CallToolParamsRaw); ok {
			toolName = p.Name
		}

		s.logger.Debug("tool call received",
			slog.String("tool", toolName),
			slog.String("request_id", reqID),
		)

		start := time.Now()
		res, err := next(ctx, method, req)
		latency := time.Since(start)

		status := "success"
		if err != nil {
			status = "error"
			s.logger.Error("tool call failed",
				slog.String("tool", toolName),
				slog.String("request_id", reqID),
				slog.Int64("latency_ms", latency.Milliseconds()),
				slog.Any("error", err),
			)
		} else {
			s.logger.Info("tool call completed",
				slog.String("tool", toolName),
				slog.String("request_id", reqID),
				slog.Int64("latency_ms", latency.Milliseconds()),
			)
		}

		s.metrics.ToolCalls.WithLabelValues(toolName, status).Inc()
		s.metrics.ToolLatency.WithLabelValues(toolName).Observe(latency.Seconds())

		return res, err
	}
}
