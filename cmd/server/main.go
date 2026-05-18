package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/mcp"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
)

func main() {
	cfg := loadConfig()

	logger := obs.NewLogger()
	metrics := obs.NewMetrics()

	mcpSrv := mcp.New(logger, metrics)
	metricsSrv := obs.MetricsServer(cfg.metricsAddr, metrics)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start the Prometheus metrics server.
	go func() {
		logger.Info("metrics server starting", slog.String("addr", cfg.metricsAddr))
		if err := metricsSrv.ListenAndServe(); err != nil {
			// ErrServerClosed is expected on graceful shutdown.
			logger.Info("metrics server stopped", slog.Any("reason", err))
		}
	}()

	// Shut down the metrics server when the signal context is cancelled.
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsSrv.Shutdown(shutCtx); err != nil {
			logger.Error("metrics server shutdown error", slog.Any("error", err))
		}
	}()

	logger.Info("dsv-tracking-mcp-server ready",
		slog.String("metrics_addr", cfg.metricsAddr),
		slog.Duration("cache_ttl", cfg.cacheTTL),
		slog.Duration("cache_stale_window", cfg.cacheStaleWindow),
	)

	if err := mcpSrv.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("MCP server error", slog.Any("error", err))
		os.Exit(1)
	}
}

type config struct {
	metricsAddr      string
	cacheTTL         time.Duration
	cacheStaleWindow time.Duration
}

func loadConfig() config {
	cfg := config{
		metricsAddr:      envOr("METRICS_ADDR", ":9090"),
		cacheTTL:         mustDuration("CACHE_TTL", 5*time.Minute),
		cacheStaleWindow: mustDuration("CACHE_STALE_WINDOW", 15*time.Minute),
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid %s=%q: %v\n", key, v, err)
		os.Exit(1)
	}
	return d
}
