package dsv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
)

const (
	defaultBaseURL    = "https://mydsv.dsv.com"
	defaultTimeout    = 10 * time.Second
	defaultMaxRetries = 2
	userAgent         = "dsv-tracking-mcp-server/0.1.0"
	initialBackoff    = 100 * time.Millisecond

	pathSearch         = "/nges-portal/api/public/tracking-public/shipments"
	pathDetail         = "/nges-portal/api/public/tracking-public/shipments"
	pathReferenceTypes = "/nges-portal/api/public/tracking-public/reference-types"
)

// ClientConfig configures the DSV HTTP client.
type ClientConfig struct {
	// BaseURL overrides the default DSV host. Defaults to "https://mydsv.dsv.com".
	BaseURL string
	// Timeout per HTTP request. Defaults to 10 s.
	Timeout time.Duration
	// MaxRetries is the number of retries for 5xx and connection errors.
	// Default 2 means up to 3 total attempts.
	MaxRetries int
	Logger     *slog.Logger
	Metrics    *obs.Metrics
}

// Client calls the DSV public tracking API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	logger     *slog.Logger
	metrics    *obs.Metrics
}

// NewClient constructs a Client from cfg, applying defaults for zero values.
func NewClient(cfg ClientConfig) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Client{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		httpClient: &http.Client{Timeout: cfg.Timeout},
		maxRetries: cfg.MaxRetries,
		logger:     cfg.Logger,
		metrics:    cfg.Metrics,
	}
}

// Search calls the shipment search endpoint and returns the raw DTO.
// Callers use MapShipmentSummaries to translate to domain types.
func (c *Client) Search(ctx context.Context, reference string) (*SearchResponseDTO, error) {
	u, err := url.Parse(c.baseURL + pathSearch)
	if err != nil {
		return nil, c.wrapErr("search", reference, "", 0, domain.ErrMalformedResponse)
	}
	u.RawQuery = url.Values{"query": {reference}}.Encode()

	body, resp, err := c.doWithRetry(ctx, u.String(), "search", reference)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.mapErrorBody(body, "search", reference, resp.StatusCode)
	}

	var dto SearchResponseDTO
	if jsonErr := json.Unmarshal(body, &dto); jsonErr != nil {
		return nil, c.wrapErr("search", reference, "", resp.StatusCode, domain.ErrMalformedResponse)
	}
	return &dto, nil
}

// Detail calls the shipment detail endpoint for the given composite shipmentID.
// The transport mode segment is derived from the shipmentID itself (last colon-
// separated component, lowercased).
// Callers use MapShipmentDetail to translate to a domain.Shipment.
//
// URL construction note: colons in shipmentIDs are NOT percent-encoded; the
// upstream routing requires them raw. See UPSTREAM.md "Headers and Request
// Requirements".
func (c *Client) Detail(ctx context.Context, shipmentID string) (*ShipmentDetailDTO, error) {
	mode := transportModeFromShipmentID(shipmentID)
	// Colons in the shipmentID must NOT be percent-encoded.
	// url.PathEscape would encode them as %3A, breaking upstream routing.
	rawURL := c.baseURL + pathDetail + "/" + mode + "/" + shipmentID

	body, resp, err := c.doWithRetry(ctx, rawURL, "detail", shipmentID)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.mapErrorBody(body, "detail", shipmentID, resp.StatusCode)
	}

	var dto ShipmentDetailDTO
	if jsonErr := json.Unmarshal(body, &dto); jsonErr != nil {
		return nil, c.wrapErr("detail", shipmentID, "", resp.StatusCode, domain.ErrMalformedResponse)
	}
	return &dto, nil
}

// ReferenceTypes fetches the 21 reference type descriptors from the discovery
// endpoint. Intended to be called once at startup and cached for the process
// lifetime by the caller.
//
// Note: the bundled patterns in internal/domain/reference.go are used as the
// primary source. This method exists for validation/comparison purposes and
// does not need to succeed at startup. See README.md "Setup" for details.
func (c *Client) ReferenceTypes(ctx context.Context) ([]ReferenceTypeDTO, error) {
	rawURL := c.baseURL + pathReferenceTypes
	body, resp, err := c.doWithRetry(ctx, rawURL, "reference_types", "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.mapErrorBody(body, "reference_types", "", resp.StatusCode)
	}
	var dtos []ReferenceTypeDTO
	if jsonErr := json.Unmarshal(body, &dtos); jsonErr != nil {
		return nil, c.wrapErr("reference_types", "", "", resp.StatusCode, domain.ErrMalformedResponse)
	}
	return dtos, nil
}

// ─── retry core ──────────────────────────────────────────────────────────────

// doWithRetry executes a GET request with exponential-backoff retries on 5xx
// and connection errors. Returns the response body bytes and the final response
// struct (caller is responsible for status-code handling). Context cancellation
// propagates immediately.
func (c *Client) doWithRetry(ctx context.Context, rawURL, op, ref string) ([]byte, *http.Response, error) {
	reqID := obs.RequestIDFromContext(ctx)

	backoff := initialBackoff
	var (
		lastBody []byte
		lastResp *http.Response
		lastErr  error
	)

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}

		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, nil, c.wrapErr(op, ref, "", 0, domain.ErrMalformedResponse)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)
		if reqID != "" {
			req.Header.Set("X-Request-ID", reqID)
		}

		c.logger.Debug("upstream request",
			slog.String("op", op),
			slog.String("url", rawURL),
			slog.String("request_id", reqID),
			slog.Int("attempt", attempt+1),
		)

		resp, err := c.httpClient.Do(req)
		latency := time.Since(start)

		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			c.logger.Warn("upstream connection error",
				slog.String("op", op),
				slog.String("error", err.Error()),
				slog.Int("attempt", attempt+1),
			)
			c.recordMetrics(op, "conn_error", latency)
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4 MiB
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			lastResp = resp
			c.recordMetrics(op, fmt.Sprintf("%d", resp.StatusCode), latency)
			continue
		}

		statusStr := fmt.Sprintf("%d", resp.StatusCode)
		c.recordMetrics(op, statusStr, latency)

		c.logger.Debug("upstream response",
			slog.String("op", op),
			slog.String("request_id", reqID),
			slog.Int("status", resp.StatusCode),
			slog.Duration("latency", latency),
		)

		// 429: throttled — do not retry.
		if resp.StatusCode == http.StatusTooManyRequests {
			return body, resp, nil
		}

		// 4xx: deterministic, do not retry.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return body, resp, nil
		}

		// 2xx: success.
		if resp.StatusCode < 400 {
			return body, resp, nil
		}

		// 5xx: retry.
		lastBody = body
		lastResp = resp
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	_ = lastBody
	_ = lastResp
	_ = lastErr
	return nil, nil, &domain.UpstreamError{
		Op:        op,
		Reference: ref,
		Err:       domain.ErrUpstreamUnavailable,
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// mapErrorBody inspects a non-2xx response body to produce the right sentinel.
func (c *Client) mapErrorBody(body []byte, op, ref string, status int) error {
	if status == http.StatusTooManyRequests {
		return &domain.UpstreamError{
			Op:         op,
			Reference:  ref,
			HTTPStatus: status,
			Err:        domain.ErrThrottled,
		}
	}

	var errBody errorBodyDTO
	if json.Unmarshal(body, &errBody) == nil && errBody.Code != "" {
		sentinel := domain.ErrInvalidReference
		if errBody.Code == "TRACKING-BADREQ-SHIPMENT_NOT_FOUND" {
			sentinel = domain.ErrShipmentNotFound
		}
		return &domain.UpstreamError{
			Op:           op,
			Reference:    ref,
			UpstreamCode: errBody.Code,
			HTTPStatus:   status,
			Err:          sentinel,
		}
	}

	// Generic 4xx without a parseable code.
	return &domain.UpstreamError{
		Op:         op,
		Reference:  ref,
		HTTPStatus: status,
		Err:        domain.ErrInvalidReference,
	}
}

func (c *Client) wrapErr(op, ref, upstreamCode string, status int, sentinel error) error {
	return &domain.UpstreamError{
		Op:           op,
		Reference:    ref,
		UpstreamCode: upstreamCode,
		HTTPStatus:   status,
		Err:          sentinel,
	}
}

func (c *Client) recordMetrics(op, status string, latency time.Duration) {
	if c.metrics == nil {
		return
	}
	c.metrics.DSVRequests.WithLabelValues(op, status).Inc()
	c.metrics.DSVLatency.WithLabelValues(op).Observe(latency.Seconds())
}

// transportModeFromShipmentID extracts the lowercase transport mode from a
// composite shipmentID. e.g. "LandStt:VAN5022058:CTTS:LAND" → "land".
func transportModeFromShipmentID(shipmentID string) string {
	parts := strings.Split(shipmentID, ":")
	if len(parts) < 1 {
		return "land"
	}
	last := parts[len(parts)-1]
	if last == "" {
		return "land"
	}
	return strings.ToLower(last)
}
