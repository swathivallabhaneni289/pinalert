// Package geocode provides a rate-limited, identified proxy client for
// Nominatim's public /search geocoding endpoint. It has no dependency on
// internal/service or internal/api so it stays trivially unit testable and
// reusable, the same standalone-package precedent internal/ratelimit
// establishes.
//
// This package deliberately does not hand-roll bucket arithmetic — it wraps
// golang.org/x/time/rate.Limiter, this project's declared standard for
// exactly this problem (see .claude/CLAUDE.md's Supporting Libraries table).
//
// This package exists at all because browser JavaScript cannot set the
// User-Agent header Nominatim's usage policy requires (browsers silently
// drop it), and a per-browser debounce cannot enforce a rate ceiling that is
// global across every one of this app's callers — only a single server-side
// choke point can do either, which is what Search below provides (D-02).
package geocode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

// nominatimSearchURL is the only outbound base URL this package ever calls.
// It is a constant precisely so this proxy can never become an open proxy
// (T-07-01): no caller-supplied URL, host, or path fragment is ever read or
// forwarded.
const nominatimSearchURL = "https://nominatim.openstreetmap.org/search"

// defaultLimiterEvery and defaultLimiterBurst are the policy ceiling from
// D-02: at most one outbound request per second, across every caller of
// this process, no matter how many people are typing concurrently.
const (
	defaultLimiterEvery = 1 * time.Second
	defaultLimiterBurst = 1
)

// limiterWaitTimeout bounds how long Search will wait for a limiter token
// before giving up. A bounded wait means contention degrades into the D-04
// "search unavailable" message rather than an unbounded server-side queue.
const limiterWaitTimeout = 1500 * time.Millisecond

// httpClientTimeout bounds the outbound HTTP round trip to Nominatim itself.
const httpClientTimeout = 5 * time.Second

// resultLimit is the fixed "limit" query value sent with every outbound
// request — at most five matches per query.
const resultLimit = "5"

// Result is the shape this package returns to its callers: a numeric,
// ready-to-render subset of Nominatim's much larger record.
type Result struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

// nominatimRecord is the shape actually returned by Nominatim's jsonv2
// search endpoint, verified live against the real service and recorded in
// 07-RESEARCH.md. lat and lon arrive as JSON strings on the wire, not
// numbers, and must be parsed before use. Every other field Nominatim sends
// (place_id, licence, osm_id, place_rank, importance, boundingbox, and the
// rest) is deliberately absent from this struct so it can never be
// forwarded onward to a caller (T-07-04).
type nominatimRecord struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
}

// Client is a rate-limited, identified proxy to Nominatim's /search
// endpoint. Safe for concurrent use.
type Client struct {
	httpClient   *http.Client
	limiter      *rate.Limiter
	baseURL      string
	contactEmail string
}

// NewClient constructs a Client pointed at the real Nominatim service,
// enforcing D-02's 1 request/second global ceiling. contactEmail is
// optional; when non-empty it is sent as Nominatim's suggested "email"
// identifier param and folded into the User-Agent string.
func NewClient(contactEmail string) *Client {
	return newClientWithBaseURL(nominatimSearchURL, contactEmail, defaultLimiterEvery, defaultLimiterBurst)
}

// newClientWithBaseURL is NewClient's unexported implementation, taking an
// explicit base URL and limiter rate so this package's own tests can drive
// a fake httptest upstream and a millisecond-scale limiter — mirroring
// ratelimit.newPerIPWithIdleWindow's unexported-test-seam precedent
// exactly. The exported two-value NewClient signature is the contract other
// packages use.
func newClientWithBaseURL(baseURL, contactEmail string, every time.Duration, burst int) *Client {
	return &Client{
		httpClient:   &http.Client{Timeout: httpClientTimeout},
		limiter:      rate.NewLimiter(rate.Every(every), burst),
		baseURL:      baseURL,
		contactEmail: contactEmail,
	}
}

// userAgent returns the identifying, non-stock User-Agent string every
// outbound request carries. When contactEmail is configured it is folded
// in, matching Nominatim's suggested good-citizen identifier. When it is
// empty, a fallback identifier is used instead of an empty parenthetical:
// the policy requirement is an identifying agent string, and a geocoding
// convenience feature must never be the reason the process cannot identify
// itself — the same D-04 reasoning that keeps this feature from ever
// blocking report submission.
func (c *Client) userAgent() string {
	if c.contactEmail != "" {
		return "Pinalert/1.0 (+" + c.contactEmail + ")"
	}
	return "Pinalert/1.0 (+https://github.com/swathivallabhaneni289/pinalert)"
}

// buildRequest constructs the single outbound GET this package ever issues.
// Only q is caller-influenced; the base URL is the package constant and the
// query string is always built with url.Values.Encode(), never string
// concatenation, so this proxy cannot become an open proxy or an SSRF pivot
// (T-07-01).
func (c *Client) buildRequest(ctx context.Context, q string) (*http.Request, error) {
	params := url.Values{}
	params.Set("q", q)
	params.Set("format", "jsonv2")
	params.Set("limit", resultLimit)
	if c.contactEmail != "" {
		params.Set("email", c.contactEmail)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent())
	return req, nil
}

// Search issues one rate-limited, identified GET to Nominatim for q and
// returns the matches as Results with numeric coordinates. The rate limiter
// is waited on first (Wait, never Allow — a 1-second wait serves two
// simultaneous typers better than instantly rejecting the second one), and
// that wait is itself bounded by limiterWaitTimeout so contention degrades
// into an error rather than an unbounded queue.
func (c *Client) Search(ctx context.Context, q string) ([]Result, error) {
	waitCtx, cancel := context.WithTimeout(ctx, limiterWaitTimeout)
	defer cancel()
	if err := c.limiter.Wait(waitCtx); err != nil {
		return nil, fmt.Errorf("geocode: rate limiter wait: %w", err)
	}

	req, err := c.buildRequest(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("geocode: building request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocode: executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("geocode: upstream status %d", resp.StatusCode)
	}

	var records []nominatimRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, fmt.Errorf("geocode: decoding upstream response: %w", err)
	}

	results := make([]Result, 0, len(records))
	for _, rec := range records {
		lat, err := strconv.ParseFloat(rec.Lat, 64)
		if err != nil {
			// Skip, don't zero-fill: a bad row must never become a 0,0 pin.
			continue
		}
		lon, err := strconv.ParseFloat(rec.Lon, 64)
		if err != nil {
			continue
		}
		results = append(results, Result{
			Name:        rec.Name,
			DisplayName: rec.DisplayName,
			Lat:         lat,
			Lon:         lon,
		})
	}
	return results, nil
}
