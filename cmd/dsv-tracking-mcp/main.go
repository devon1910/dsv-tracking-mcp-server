package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/cache"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/mcp"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv/browser"
)

func main() {
	cfg := loadConfig()

	logger := obs.NewLogger()
	metrics := obs.NewMetrics()

	// ── Metrics HTTP server ────────────────────────────────────────────────
	var metricsSrv *http.Server
	if cfg.metricsAddr != "" {
		metricsSrv = obs.MetricsServer(cfg.metricsAddr, metrics)
		go func() {
			logger.Info("metrics server starting", slog.String("addr", cfg.metricsAddr))
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Warn("metrics server stopped", slog.Any("reason", err))
			}
		}()
	}

	// ── Signal context ─────────────────────────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Browser + DSV client ───────────────────────────────────────────────
	br, err := browser.New(ctx, browser.Config{
		Headless:          cfg.browserHeadless,
		NavigationTimeout: cfg.navTimeout,
		XHRTimeout:        cfg.xhrTimeout,
		Logger:            logger,
		Metrics:           metrics,
	})
	if err != nil {
		logger.Error("failed to launch browser", slog.Any("err", err))
		os.Exit(1)
	}
	defer br.Close()

	dsvClient := dsv.NewClient(dsv.ClientConfig{
		Browser: br,
		Logger:  logger,
		Metrics: metrics,
	})

	upstream := &dsvAdapter{client: dsvClient}

	// ── Caches ────────────────────────────────────────────────────────────
	// Search cache: 60s TTL, uniform across all search results.
	searchCache := cache.New[[]domain.ShipmentSummary](
		cache.Config{TTL: cfg.searchTTL, StaleWindow: 5 * cfg.searchTTL},
		logger,
	)
	// Detail cache: 30s default TTL for non-terminal statuses.
	// The get_shipment_details handler upgrades Delivered entries to 24h TTL
	// via cache.SetWithTTL after each successful live fetch.
	detailCache := cache.New[domain.Shipment](
		cache.Config{TTL: cfg.detailTTL, StaleWindow: 5 * cfg.detailTTL},
		logger,
	)

	// ── MCP server ─────────────────────────────────────────────────────────
	mcpSrv := mcp.New(logger, metrics)
	mcp.RegisterAll(mcpSrv, mcp.ToolDeps{
		Upstream:    upstream,
		SearchCache: searchCache,
		DetailCache: detailCache,
		Logger:      logger,
		Metrics:     metrics,
	})

	logger.Info("dsv-tracking-mcp-server ready",
		slog.String("metrics_addr", cfg.metricsAddr),
		slog.Duration("search_ttl", cfg.searchTTL),
		slog.Duration("detail_ttl", cfg.detailTTL),
	)

	// ── Run (blocks) ───────────────────────────────────────────────────────
	if err := mcpSrv.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("MCP server error", slog.Any("error", err))
		os.Exit(1)
	}

	// ── Graceful shutdown ──────────────────────────────────────────────────
	if metricsSrv != nil {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsSrv.Shutdown(shutCtx)
	}
}

// dsvAdapter adapts *dsv.Client to mcp.Upstream by applying the mapper layer.
type dsvAdapter struct {
	client *dsv.Client
}

func (a *dsvAdapter) Search(ctx context.Context, ref string) ([]domain.ShipmentSummary, error) {
	dto, err := a.client.Search(ctx, ref)
	if err != nil {
		return nil, err
	}
	return dsv.MapShipmentSummaries(dto), nil
}

func (a *dsvAdapter) Detail(ctx context.Context, shipmentID string) (domain.Shipment, error) {
	dto, err := a.client.Detail(ctx, shipmentID)
	if err != nil {
		return domain.Shipment{}, err
	}
	return dsv.MapShipmentDetail(dto)
}

// ─── config ───────────────────────────────────────────────────────────────────

type config struct {
	metricsAddr     string
	searchTTL       time.Duration
	detailTTL       time.Duration
	browserHeadless bool
	navTimeout      time.Duration
	xhrTimeout      time.Duration
}

func loadConfig() config {
	return config{
		metricsAddr:     envOr("METRICS_ADDR", ":9090"),
		searchTTL:       mustDuration("CACHE_SEARCH_TTL", 60*time.Second),
		detailTTL:       mustDuration("CACHE_DETAIL_TTL", 30*time.Second),
		browserHeadless: envOr("BROWSER_HEADLESS", "true") == "true",
		navTimeout:      mustDuration("BROWSER_NAVIGATION_TIMEOUT", 30*time.Second),
		xhrTimeout:      mustDuration("BROWSER_XHR_TIMEOUT", 20*time.Second),
	}
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
