package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
)

// Browser manages a long-lived headless Chromium process used to fetch JSON
// responses produced by pages that perform Cap.js validation.
//
// The Browser is a process-wide singleton. Each FetchJSON call creates a fresh
// chromedp tab (child context) so concurrent calls do not interfere, while the
// underlying Chrome process and cookie jar are shared. Once Cap.js validates in
// one tab, subsequent navigations within the same browser session reuse the
// approved session state until it expires — solving the captcha once and
// amortising the cost across all requests.
type Browser struct {
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
	logger        *slog.Logger
	metrics       *obs.Metrics
	navTO         time.Duration
	xhrTO         time.Duration
}

// Config controls browser launch behaviour.
type Config struct {
	// Headless runs Chrome without a visible window. Default true.
	Headless bool
	// NavigationTimeout is the maximum time to wait for a page's load event.
	// Default 30 s.
	NavigationTimeout time.Duration
	// XHRTimeout is the maximum time to wait for the target XHR after
	// navigation completes. Default 20 s.
	XHRTimeout time.Duration
	Logger     *slog.Logger
	Metrics    *obs.Metrics
}

// New launches a Chromium instance and returns a Browser ready to fetch.
// Caller MUST call Close before process exit to release the Chrome process.
func New(ctx context.Context, cfg Config) (*Browser, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.NavigationTimeout <= 0 {
		cfg.NavigationTimeout = 30 * time.Second
	}
	if cfg.XHRTimeout <= 0 {
		cfg.XHRTimeout = 20 * time.Second
	}

	opts := buildAllocatorOptions(cfg)
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)

	// Warm the browser: NewContext starts Chrome lazily; Run forces it alive.
	bCtx, bCancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(bCtx); err != nil {
		bCancel()
		allocCancel()
		return nil, fmt.Errorf("launch chromedp: %w", err)
	}

	cfg.Logger.Info("browser launched", slog.Bool("headless", cfg.Headless))
	return &Browser{
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    bCtx,
		browserCancel: bCancel,
		logger:        cfg.Logger,
		metrics:       cfg.Metrics,
		navTO:         cfg.NavigationTimeout,
		xhrTO:         cfg.XHRTimeout,
	}, nil
}

// FetchJSON navigates to pageURL, intercepts the first XHR/Fetch response
// whose URL contains xhrSubstring, and returns the raw response body bytes.
//
// Transport-level errors (navigation timeout, Chrome crash) return a
// *domain.UpstreamError wrapping domain.ErrUpstreamUnavailable.
// HTTP 4xx from the intercepted XHR returns ErrShipmentNotFound (for 400)
// or ErrInvalidReference (for other 4xx). HTTP 5xx returns ErrUpstreamUnavailable.
func (b *Browser) FetchJSON(ctx context.Context, pageURL, xhrSubstring string) ([]byte, error) {
	reqID := obs.RequestIDFromContext(ctx)
	start := time.Now()

	// Each call gets its own tab. Tabs share the browser process and cookie
	// jar, so Cap.js state from a previous tab is reused automatically.
	tabCtx, tabCancel := chromedp.NewContext(b.browserCtx)
	defer tabCancel()

	type xhrResult struct {
		body   []byte
		status int
	}
	resCh := make(chan xhrResult, 1)

	// Mutable XHR-tracking state. Both EventResponseReceived and
	// EventLoadingFinished are dispatched by the same chromedp event-loop
	// goroutine, so sequential consistency is guaranteed between the two
	// cases — but we still use a mutex because the goroutine launched in
	// EventLoadingFinished reads these values from a different goroutine.
	var (
		mu              sync.Mutex
		targetRequestID network.RequestID
		targetStatus    int
	)

	chromedp.ListenTarget(tabCtx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			// Only match XHR or Fetch resource types to avoid CORS preflights,
			// scripts, and other noise that may share URL substrings.
			if e.Type != network.ResourceTypeXHR && e.Type != network.ResourceTypeFetch {
				return
			}
			if !strings.Contains(e.Response.URL, xhrSubstring) {
				return
			}
			mu.Lock()
			targetRequestID = e.RequestID
			targetStatus = int(e.Response.Status)
			mu.Unlock()
			b.logger.Debug("XHR matched",
				slog.String("url", e.Response.URL),
				slog.Int64("status", e.Response.Status),
			)

		case *network.EventLoadingFinished:
			mu.Lock()
			id := targetRequestID
			status := targetStatus
			mu.Unlock()

			if e.RequestID != id || id == "" {
				return
			}

			// Capture the Target executor NOW (in the event-loop goroutine)
			// before launching the fetch goroutine. tabCtx may be mid-redirect
			// when the goroutine runs, which would cause Do(tabCtx) to fail
			// with "invalid context". Using the raw cdp Executor bypasses the
			// chromedp context lifecycle and targets the tab directly.
			chromedpCtx := chromedp.FromContext(tabCtx)
			if chromedpCtx == nil || chromedpCtx.Target == nil {
				return
			}
			executor := cdp.WithExecutor(context.Background(), chromedpCtx.Target)

			// Capture by value so the goroutine doesn't race with future events.
			go func(reqID network.RequestID, httpStatus int) {
				body, err := network.GetResponseBody(reqID).Do(executor)
				if err != nil {
					b.logger.Warn("GetResponseBody failed, waiting for next match",
						slog.String("reqID", string(reqID)),
						slog.Any("error", err))
					return
				}
				b.logger.Debug("XHR body obtained",
					slog.Int("bytes", len(body)), slog.Int("status", httpStatus))
				if len(body) == 0 {
					return // Empty body; not the API response we want.
				}
				select {
				case resCh <- xhrResult{body: body, status: httpStatus}:
				default:
				}
			}(id, status)
		}
	})

	// Enable Network with an explicit body-buffer budget so Chrome keeps
	// response bodies available for GetResponseBody calls.
	// Navigate; the load event fires when the page shell is ready, but
	// Cap.js may fire the tracking XHR after that — we keep the tab alive.
	navCtx, navCancel := context.WithTimeout(tabCtx, b.navTO)
	defer navCancel()

	b.logger.Info("navigating", slog.String("page", pageURL), slog.String("request_id", reqID))

	if err := chromedp.Run(navCtx,
		network.Enable().
			WithMaxTotalBufferSize(4*1024*1024).
			WithMaxResourceBufferSize(1*1024*1024),
		chromedp.Navigate(pageURL),
	); err != nil && navCtx.Err() == nil {
		// Navigation failed for a non-timeout reason.
		b.recordMetrics(xhrSubstring, "nav_err", time.Since(start))
		return nil, &domain.UpstreamError{Op: "browser_nav", Err: domain.ErrUpstreamUnavailable}
	}

	// Wait for the intercepted XHR or timeout.
	xhrTimer := time.NewTimer(b.xhrTO)
	defer xhrTimer.Stop()

	select {
	case r := <-resCh:
		latency := time.Since(start)
		statusStr := fmt.Sprintf("%d", r.status)
		b.recordMetrics(xhrSubstring, statusStr, latency)
		b.logger.Debug("XHR body received",
			slog.String("xhr", xhrSubstring),
			slog.Int("status", r.status),
			slog.Int("bytes", len(r.body)),
		)
		if r.status >= 400 {
			return nil, mapErrorResponse(r.status, r.body)
		}
		return r.body, nil

	case <-xhrTimer.C:
		b.recordMetrics(xhrSubstring, "timeout", time.Since(start))
		return nil, &domain.UpstreamError{Op: "browser_fetch", Err: domain.ErrUpstreamUnavailable}

	case <-ctx.Done():
		b.recordMetrics(xhrSubstring, "cancelled", time.Since(start))
		return nil, &domain.UpstreamError{Op: "browser_fetch", Err: domain.ErrUpstreamUnavailable}
	}
}

// Close shuts down the browser process and releases all associated resources.
func (b *Browser) Close() error {
	b.browserCancel()
	b.allocCancel()
	b.logger.Info("browser shut down")
	return nil
}

// mapErrorResponse converts a 4xx/5xx XHR response to a domain.UpstreamError.
// It parses the upstream error body to extract the machine-readable code so
// callers can match on ErrShipmentNotFound via errors.Is.
func mapErrorResponse(status int, body []byte) *domain.UpstreamError {
	var upstreamCode string
	if len(body) > 0 {
		var errBody struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(body, &errBody); err == nil {
			upstreamCode = errBody.Code
		}
	}

	var sentinel error
	switch {
	case upstreamCode == "TRACKING-BADREQ-SHIPMENT_NOT_FOUND" || status == 404:
		sentinel = domain.ErrShipmentNotFound
	case status == 400:
		sentinel = domain.ErrInvalidReference
	case status == 429:
		sentinel = domain.ErrThrottled
	case status >= 500:
		sentinel = domain.ErrUpstreamUnavailable
	default:
		sentinel = domain.ErrInvalidReference
	}
	return &domain.UpstreamError{
		Op:           "browser_fetch",
		UpstreamCode: upstreamCode,
		HTTPStatus:   status,
		Err:          sentinel,
	}
}

// buildAllocatorOptions assembles the chromedp ExecAllocator option list.
//
// We intentionally build from scratch rather than extending
// DefaultExecAllocatorOptions because DefaultExecAllocatorOptions includes
// --enable-automation, which sets navigator.webdriver=true — a flag that
// Cap.js (and other bot-detection systems) test explicitly. Building from
// scratch lets us omit that flag while keeping everything else we need.
func buildAllocatorOptions(cfg Config) []func(*chromedp.ExecAllocator) {
	opts := []func(*chromedp.ExecAllocator){
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		// Suppress the navigator.webdriver=true indicator that automation sets.
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		// Performance / stability flags from DefaultExecAllocatorOptions that
		// are safe to keep.
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
		// Skip images — we only care about JSON responses.
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.WindowSize(1280, 800),
		// Realistic UA so Cap.js doesn't fingerprint us as a bare bot.
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
	}

	if cfg.Headless {
		// --headless=new is the modern Chromium headless mode, introduced in
		// Chrome 112. It shares more code with the visible browser than the
		// legacy --headless mode and is harder for bot-detection to fingerprint.
		opts = append(opts, chromedp.Flag("headless", "new"))
	}

	if os.Getenv("BROWSER_NO_SANDBOX") == "true" {
		opts = append(opts, chromedp.NoSandbox)
	}

	// Allow overriding the Chrome binary path, useful on machines where Chrome
	// is not in PATH (e.g. non-standard Windows installations).
	if execPath := os.Getenv("CHROMEDP_EXEC_PATH"); execPath != "" {
		opts = append(opts, chromedp.ExecPath(execPath))
	}

	return opts
}

func (b *Browser) recordMetrics(endpoint, status string, latency time.Duration) {
	if b.metrics == nil {
		return
	}
	b.metrics.DSVBrowserFetches.WithLabelValues(endpoint, status).Inc()
	b.metrics.DSVBrowserLatency.WithLabelValues(endpoint).Observe(latency.Seconds())
}
