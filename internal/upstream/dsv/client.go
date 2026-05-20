package dsv

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"

	"github.com/devon1910/dsv-tracking-mcp-server/internal/domain"
	"github.com/devon1910/dsv-tracking-mcp-server/internal/obs"
)

const (
	defaultBaseURL = "https://mydsv.dsv.com"
	pathSearch         = "/nges-portal/api/public/tracking-public/shipments"
	pathDetail         = "/nges-portal/api/public/tracking-public/shipments"
	pathReferenceTypes = "/nges-portal/api/public/tracking-public/reference-types"
)

// Fetcher performs a browser-backed fetch of a page and extracts the JSON
// response from an XHR whose URL contains the provided substring.
//
// The interface exists to allow unit tests to inject a fake fetcher without
// launching Chrome. The concrete implementation in this package is the
// Browser type in internal/upstream/dsv/browser.
type Fetcher interface {
	FetchJSON(ctx context.Context, pageURL, xhrSubstring string) ([]byte, error)
}

// ClientConfig configures the DSV client.
type ClientConfig struct {
	Browser Fetcher
	BaseURL string
	Logger  *slog.Logger
	Metrics *obs.Metrics
}

// Client calls the DSV public tracking API via a browser-backed Fetcher.
type Client struct {
	fetcher Fetcher
	baseURL string
	logger  *slog.Logger
	metrics *obs.Metrics
}

// NewClient constructs a Client from cfg. Browser must be non-nil.
func NewClient(cfg ClientConfig) *Client {
	if cfg.Browser == nil {
		panic("dsv: Browser fetcher must be provided")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Client{
		fetcher: cfg.Browser,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		logger:  cfg.Logger,
		metrics: cfg.Metrics,
	}
}

// Search calls the shipment search endpoint and returns the raw DTO.
// Callers use MapShipmentSummaries to translate to domain types.
func (c *Client) Search(ctx context.Context, reference string) (*SearchResponseDTO, error) {
	pageURL := c.baseURL + "/app/tracking-public/?refNumber=" + url.QueryEscape(reference)
	xhrSubstring := "/nges-portal/api/public/tracking-public/shipments?query=" + reference

	body, err := c.fetcher.FetchJSON(ctx, pageURL, xhrSubstring)
	if err != nil {
		return nil, err
	}
	var dto SearchResponseDTO
	if jsonErr := json.Unmarshal(body, &dto); jsonErr != nil {
		return nil, &domain.UpstreamError{Op: "search", Reference: reference, Err: domain.ErrMalformedResponse}
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
// detailFetcher is an optional extension of Fetcher implemented by the real
// browser. It navigates once and watches both the search XHR (fail-fast on
// not-found) and the detail XHR, mirroring DSV's own frontend behaviour.
type detailFetcher interface {
	FetchDetailJSON(ctx context.Context, pageURL, searchSubstring, detailSubstring string) ([]byte, error)
}

func (c *Client) Detail(ctx context.Context, shipmentID string) (*ShipmentDetailDTO, error) {
	stt := extractSTT(shipmentID)
	pageURL := c.baseURL + "/app/tracking-public/?refNumber=" + url.QueryEscape(stt)
	searchSubstring := pathSearch + "?query=" + stt
	detailSubstring := "/shipments/land/" + shipmentID

	var (
		body []byte
		err  error
	)
	if df, ok := c.fetcher.(detailFetcher); ok {
		body, err = df.FetchDetailJSON(ctx, pageURL, searchSubstring, detailSubstring)
	} else {
		// Fallback for tests that inject a simple fake fetcher.
		body, err = c.fetcher.FetchJSON(ctx, pageURL, detailSubstring)
	}
	if err != nil {
		return nil, err
	}
	var dto ShipmentDetailDTO
	if jsonErr := json.Unmarshal(body, &dto); jsonErr != nil {
		return nil, &domain.UpstreamError{Op: "detail", Reference: shipmentID, Err: domain.ErrMalformedResponse}
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
	pageURL := c.baseURL + "/app/tracking-public/"
	xhrSubstring := "/tracking-public/reference-types"
	body, err := c.fetcher.FetchJSON(ctx, pageURL, xhrSubstring)
	if err != nil {
		return nil, err
	}
	var dtos []ReferenceTypeDTO
	if jsonErr := json.Unmarshal(body, &dtos); jsonErr != nil {
		return nil, &domain.UpstreamError{Op: "reference_types", Err: domain.ErrMalformedResponse}
	}
	return dtos, nil
}
// extractSTT extracts the STT number from a composite shipmentID. If it
// cannot be found, it returns the original shipmentID as a fallback.
func extractSTT(shipmentID string) string {
	// STT is usually the second colon-separated component.
	parts := strings.Split(shipmentID, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return shipmentID
}
