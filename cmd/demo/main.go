package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv"
	browserpkg "github.com/devon1910/dsv-tracking-mcp-server/internal/upstream/dsv/browser"
)

func main() {
	ref := flag.String("ref", "", "reference to search (STT, waybill)")
	headless := flag.Bool("headless", true, "run Chrome headless")
	flag.Parse()
	if *ref == "" {
		fmt.Fprintln(os.Stderr, "ref is required")
		os.Exit(2)
	}

	logger := obs.NewLogger()
	metrics := obs.NewMetrics()

	br, err := browserpkg.New(context.Background(), browserpkg.Config{
		Headless: *headless,
		Logger:   logger,
		Metrics:  metrics,
	})
	if err != nil {
		logger.Error("failed to launch browser", slog.Any("err", err))
		os.Exit(1)
	}
	defer br.Close()

	client := dsv.NewClient(dsv.ClientConfig{Browser: br, Metrics: metrics, Logger: logger})

	start := time.Now()
	dto, err := client.Search(context.Background(), *ref)
	if err != nil {
		logger.Error("search failed", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("search returned", slog.Int("count", len(dto.Result)))
	if len(dto.Result) > 0 {
		first := dto.Result[0]
		fmt.Printf("first shipmentID=%s stt=%s\n", first.ID, first.Stt)
		// try detail
		if first.ID != "" {
			detailStart := time.Now()
			detail, derr := client.Detail(context.Background(), first.ID)
			if derr != nil {
				logger.Error("detail failed", slog.Any("err", derr))
				os.Exit(1)
			}
			logger.Info("detail returned", slog.String("stt", detail.STTNumber), slog.Int("progress", detail.PercentageProgress), slog.Duration("took", time.Since(detailStart)))
			fmt.Printf("STT=%s product=%s percent=%d\n", detail.STTNumber, detail.Product, detail.PercentageProgress)
		}
	}
	logger.Info("demo finished", slog.Duration("total", time.Since(start)))
}
