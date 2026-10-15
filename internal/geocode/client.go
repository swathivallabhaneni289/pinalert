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
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
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
	reader       *readerCache
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
		reader:       newReaderCache(),
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
//
// extra carries the fixed, server-computed parameters the reader lane adds
// (viewbox and bounded); Search passes nil so its request is unchanged.
func (c *Client) buildRequest(ctx context.Context, q string, extra url.Values) (*http.Request, error) {
	params := url.Values{}
	params.Set("q", q)
	params.Set("format", "jsonv2")
	params.Set("limit", resultLimit)
	if c.contactEmail != "" {
		params.Set("email", c.contactEmail)
	}
	for key, values := range extra {
		for _, v := range values {
			params.Set(key, v)
		}
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
	return c.fetch(ctx, q, nil)
}

// UpstreamStatusError is returned when Nominatim answers with a non 2xx
// status. It carries the status code and nothing else, so it is safe to log.
type UpstreamStatusError struct {
	Status int
}

func (e *UpstreamStatusError) Error() string {
	return fmt.Sprintf("geocode: upstream status %d", e.Status)
}

// scrubURLError drops the request URL from an error. net/http wraps every
// transport failure in a *url.Error whose text carries the full URL, and
// that URL holds the reader's search text, the viewbox around their map
// centre and the contact email. The inner error keeps the useful class (a
// timeout, a refused connection, a cancelled context) and works with
// errors.Is and errors.As, so callers lose nothing they need.
func scrubURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return urlErr.Err
	}
	return err
}

// fetch is the one upstream call Search and SearchReader share, so the two
// lanes cannot drift: build the request, check the status, decode, and parse
// the string coordinates. It never touches the limiter; each caller decides
// how to take a token before calling it.
func (c *Client) fetch(ctx context.Context, q string, extra url.Values) ([]Result, error) {
	req, err := c.buildRequest(ctx, q, extra)
	if err != nil {
		return nil, fmt.Errorf("geocode: building request: %w", scrubURLError(err))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocode: executing request: %w", scrubURLError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &UpstreamStatusError{Status: resp.StatusCode}
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

// ErrReaderBusy means the reader lane found no free limiter token and gave
// up at once instead of waiting. The handler maps it to the plain "Search
// unavailable" message (D-16, D-28).
var ErrReaderBusy = errors.New("geocode: reader lane busy")

// Point is a map centre used to bias reader place search toward the area the
// reader is looking at.
type Point struct {
	Lat, Lon float64
}

// Reader lane tuning (03.1 RESEARCH A4). All four are easy to retune: they
// are plain constants with no other code depending on their values.
const (
	// readerCacheEntries bounds the in memory cache of reader searches.
	readerCacheEntries = 256
	// readerCacheTTL is how long a cached answer is served before the next
	// identical search goes upstream again. Place names barely change.
	readerCacheTTL = 6 * time.Hour
	// readerBiasDegrees is the half width of the viewbox sent around the map
	// centre, about 55 km each way.
	readerBiasDegrees = 0.5
	// readerBucketDegrees is the grid the map centre is rounded to for the
	// cache key, about 11 km, so one reader's ranking is never served for a
	// different area but panning a little still hits the cache.
	readerBucketDegrees = 0.1
)

// readerCache is a small bounded TTL cache for reader searches: a mutex, a
// map, and an insertion ordered list for eviction. now is a field so tests
// can move the clock.
type readerCache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]*list.Element
	order   *list.List // front is the oldest entry
}

type readerCacheEntry struct {
	key     string
	results []Result
	stored  time.Time
}

func newReaderCache() *readerCache {
	return &readerCache{
		now:     time.Now,
		entries: make(map[string]*list.Element),
		order:   list.New(),
	}
}

// get returns a fresh cached answer, dropping an expired one.
func (rc *readerCache) get(key string) ([]Result, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	el, ok := rc.entries[key]
	if !ok {
		return nil, false
	}
	entry := el.Value.(*readerCacheEntry)
	if rc.now().Sub(entry.stored) >= readerCacheTTL {
		rc.order.Remove(el)
		delete(rc.entries, key)
		return nil, false
	}
	return append([]Result(nil), entry.results...), true
}

// put stores an answer and evicts the oldest entry past the bound.
func (rc *readerCache) put(key string, results []Result) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	stored := append([]Result(nil), results...)
	if el, ok := rc.entries[key]; ok {
		entry := el.Value.(*readerCacheEntry)
		entry.results = stored
		entry.stored = rc.now()
		return
	}
	rc.entries[key] = rc.order.PushBack(&readerCacheEntry{key: key, results: stored, stored: rc.now()})
	for rc.order.Len() > readerCacheEntries {
		oldest := rc.order.Front()
		rc.order.Remove(oldest)
		delete(rc.entries, oldest.Value.(*readerCacheEntry).key)
	}
}

// size reports how many entries the cache holds.
func (rc *readerCache) size() int {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.order.Len()
}

// readerCacheKey builds the cache key from the normalised text (trimmed,
// lower cased, runs of whitespace collapsed) and the map centre rounded to
// readerBucketDegrees, or the word none when there is no centre.
func readerCacheKey(q string, near *Point) string {
	text := strings.ToLower(strings.Join(strings.Fields(q), " "))
	if near == nil {
		return text + "|none"
	}
	latBucket := int(math.Round(near.Lat / readerBucketDegrees))
	lonBucket := int(math.Round(near.Lon / readerBucketDegrees))
	return text + "|" + strconv.Itoa(latBucket) + "," + strconv.Itoa(lonBucket)
}

// SearchReader is the reader's place search lane (HIST-05). It shares the
// process wide limiter with Search but never waits for it: a cached answer
// costs no token, otherwise it takes a token only if one is free right now
// and returns ErrReaderBusy if not. A reporter's Search that is waiting
// leaves the bucket in debt, so a reader can never jump ahead of a reporter
// filing an emergency (D-16, D-28).
//
// When near is set the request carries a non bounded viewbox around it, the
// planner's call on backlog 999.7: a viewbox with bounded=0 ranks matches
// near the map centre first without hiding matches elsewhere. Successful
// answers, including an empty list, are cached; errors never are.
func (c *Client) SearchReader(ctx context.Context, q string, near *Point) ([]Result, error) {
	key := readerCacheKey(q, near)
	if cached, ok := c.reader.get(key); ok {
		return cached, nil
	}

	if !c.limiter.Allow() {
		return nil, ErrReaderBusy
	}

	var extra url.Values
	if near != nil {
		extra = url.Values{}
		// Nominatim wants left, top, right, bottom: longitude first.
		extra.Set("viewbox", strings.Join([]string{
			strconv.FormatFloat(near.Lon-readerBiasDegrees, 'f', 6, 64),
			strconv.FormatFloat(near.Lat+readerBiasDegrees, 'f', 6, 64),
			strconv.FormatFloat(near.Lon+readerBiasDegrees, 'f', 6, 64),
			strconv.FormatFloat(near.Lat-readerBiasDegrees, 'f', 6, 64),
		}, ","))
		extra.Set("bounded", "0")
	}

	results, err := c.fetch(ctx, strings.TrimSpace(q), extra)
	if err != nil {
		return nil, err
	}
	c.reader.put(key, results)
	return results, nil
}
