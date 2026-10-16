// Package geocode provides a rate-limited, identified proxy client for place
// and address search. Photon (photon.komoot.io, see photon.go) is asked
// first; Nominatim's public /search endpoint is the fallback when Photon
// fails, times out or finds nothing. Each provider has its own limiter and
// its own cache. The package has no dependency on
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
	"unicode/utf8"

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
	limiter      *rate.Limiter // Nominatim's
	baseURL      string        // Nominatim's
	contactEmail string
	reader       *answerCache // Nominatim answers for the place search
	reporter     *answerCache // Nominatim answers for the report search

	// Photon, asked first. An empty photonURL turns Photon off, which only
	// the Nominatim tests do.
	photonURL     string
	photonLimiter *rate.Limiter
	photonTimeout time.Duration
	photon        *answerCache // Photon answers, shared by both searches
}

// NewClient constructs a Client pointed at the real Photon and Nominatim
// services, enforcing D-02's 1 request/second global ceiling on Nominatim and
// two a second on Photon. contactEmail is optional; when non-empty it is sent
// as Nominatim's suggested "email" identifier param and folded into the
// User-Agent string both services receive.
func NewClient(contactEmail string) *Client {
	return newClientWithProviders(photonSearchURL, nominatimSearchURL, contactEmail,
		defaultLimiterEvery, defaultLimiterBurst, photonLimiterEvery, photonLimiterBurst)
}

// newClientWithBaseURL is a Nominatim only client for this package's own
// tests: an explicit base URL and limiter rate so they can drive a fake
// httptest upstream and a millisecond-scale limiter, mirroring
// ratelimit.newPerIPWithIdleWindow's unexported-test-seam precedent exactly.
// Photon is off. The exported NewClient signature is the contract other
// packages use.
func newClientWithBaseURL(baseURL, contactEmail string, every time.Duration, burst int) *Client {
	return newClientWithProviders("", baseURL, contactEmail, every, burst, photonLimiterEvery, photonLimiterBurst)
}

// newClientWithProviders builds a client with both providers' URLs and
// limiter rates given. An empty photonURL turns Photon off.
func newClientWithProviders(photonURL, nominatimURL, contactEmail string, every time.Duration, burst int, photonEvery time.Duration, photonBurst int) *Client {
	return &Client{
		httpClient:    &http.Client{Timeout: httpClientTimeout},
		limiter:       rate.NewLimiter(rate.Every(every), burst),
		baseURL:       nominatimURL,
		contactEmail:  contactEmail,
		reader:        newAnswerCache(),
		reporter:      newAnswerCache(),
		photonURL:     photonURL,
		photonLimiter: rate.NewLimiter(rate.Every(photonEvery), photonBurst),
		photonTimeout: defaultPhotonTimeout,
		photon:        newAnswerCache(),
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
// Every request is India only: countrycodes is a fixed parameter here, not
// something a caller passes, so no path can forget it. extra carries the
// other fixed, server computed parameters (viewbox and bounded) a boxed try
// adds; a widened try passes nil.
func (c *Client) buildRequest(ctx context.Context, q string, extra url.Values) (*http.Request, error) {
	params := url.Values{}
	params.Set("q", q)
	params.Set("format", "jsonv2")
	params.Set("limit", resultLimit)
	params.Set("countrycodes", countryCodes)
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

// Search is the report form's address search. Photon is asked first, with a
// bounded wait for its limiter and its own time limit (see askPhoton). When
// Photon fails, times out or finds nothing, the Nominatim search plan runs
// (see searchPlan) and returns the first non empty answer, with MatchedQuery
// set when a shorter text found it. Before every Nominatim request the rate
// limiter is waited on (Wait, never Allow: a one second wait serves two
// simultaneous typers better than instantly rejecting the second one), and
// each wait is bounded by limiterWaitTimeout, so contention degrades into an
// error, the "Search unavailable" answer, rather than an unbounded queue. A
// wait that runs out part way through the plan is that same error, never an
// empty list, because the texts not yet tried were never sent.
//
// near is the report pin, or nil. Answers are cached under the original text
// and area; errors never are (see search).
func (c *Client) Search(ctx context.Context, q string, near *Point) (Answer, error) {
	return c.search(ctx, q, near, c.reporter, c.waitForPhotonToken, c.waitForToken)
}

// search is the order both lanes share. The lane decides how a limiter token
// is taken from each provider and which Nominatim cache it uses.
//
//   - A Photon answer with matches, fresh or cached, is the answer.
//   - Otherwise the Nominatim plan runs, after its own cache, with whatever
//     is left of the maxUpstreamPerSearch requests once Photon's is counted.
//   - A Nominatim answer is cached in the lane's cache, except an empty one
//     after Photon failed: Photon may be back on the next search, and an
//     empty answer must not hide it for six hours.
func (c *Client) search(ctx context.Context, q string, near *Point, cache *answerCache,
	takePhoton func(ctx context.Context) error, takeNominatim func(ctx context.Context, i int) error) (Answer, error) {
	key := cacheKey(q, near)
	var photon photonOutcome
	if c.photonURL != "" {
		var err error
		photon, err = c.askPhoton(ctx, key, q, near, takePhoton)
		if err != nil {
			return Answer{}, err
		}
		if photon.found {
			return photon.answer, nil
		}
	}
	if cached, ok := cache.get(key); ok {
		return cached, nil
	}
	answer, err := c.run(ctx, q, near, takeNominatim, maxUpstreamPerSearch-photon.sent)
	if err != nil {
		return Answer{}, err
	}
	if !photon.failed || len(answer.Results) > 0 {
		cache.put(key, answer)
	}
	return answer, nil
}

// waitForToken is the reporter lane's turn at the limiter, taken before
// every upstream request.
func (c *Client) waitForToken(ctx context.Context, _ int) error {
	waitCtx, cancel := context.WithTimeout(ctx, limiterWaitTimeout)
	defer cancel()
	if err := c.limiter.Wait(waitCtx); err != nil {
		return fmt.Errorf("geocode: rate limiter wait: %w", err)
	}
	return nil
}

// Answer is what a search returns: the matches, and the shorter text that
// found them when the text as typed found nothing. MatchedQuery is empty
// when the text as typed matched, and when nothing matched at all.
type Answer struct {
	Results      []Result
	MatchedQuery string
	// provider is which service gave the answer (providerPhoton or
	// providerNominatim), for the live check and the tests only.
	provider string
}

// Search plan tuning (quick task 261016-adr, backlog 999.7).
const (
	// countryCodes keeps every search inside India.
	countryCodes = "in"
	// nearBoxDegrees is the half width of the box searched first around a
	// known position, about 55 km each way.
	nearBoxDegrees = 0.5
	// maxShorterTries is how many times the most specific part is dropped.
	maxShorterTries = 2
	// maxUpstreamPerSearch caps the upstream requests one search may send,
	// the widened tries and the shorter tries included. At one request a
	// second this keeps the worst search to about three seconds of waiting.
	maxUpstreamPerSearch = 4
	// minShorterRunes is the shortest text a shorter try may send, the same
	// minimum the handlers apply to what people type.
	minShorterRunes = 3
)

// attempt is one upstream request in a search plan.
type attempt struct {
	text      string
	boxed     bool
	shortened bool
}

// params returns the request parameters an attempt adds: a bounded viewbox
// around near for a boxed try, nothing for a widened one.
func (a attempt) params(near *Point) url.Values {
	if !a.boxed || near == nil {
		return nil
	}
	extra := url.Values{}
	// Nominatim wants left, top, right, bottom: longitude first.
	extra.Set("viewbox", strings.Join([]string{
		strconv.FormatFloat(near.Lon-nearBoxDegrees, 'f', 6, 64),
		strconv.FormatFloat(near.Lat+nearBoxDegrees, 'f', 6, 64),
		strconv.FormatFloat(near.Lon+nearBoxDegrees, 'f', 6, 64),
		strconv.FormatFloat(near.Lat-nearBoxDegrees, 'f', 6, 64),
	}, ","))
	extra.Set("bounded", "1")
	return extra
}

// searchTexts returns the text as typed (trimmed) followed by at most
// maxShorterTries shorter forms, each made from the one before by
// dropMostSpecific.
func searchTexts(q string) []string {
	texts := []string{strings.TrimSpace(q)}
	current := texts[0]
	for i := 0; i < maxShorterTries; i++ {
		next, ok := dropMostSpecific(current)
		if !ok {
			break
		}
		texts = append(texts, next)
		current = next
	}
	return texts
}

// dropMostSpecific drops the first comma separated part of text, or the first
// word when there is only one part. Parts and words are trimmed and empty
// parts skipped, so "Flat 2 ,, Banjara Hills" becomes "Banjara Hills". It
// reports false when nothing is left to drop or what is left is shorter than
// minShorterRunes.
func dropMostSpecific(text string) (string, bool) {
	var parts []string
	for _, part := range strings.Split(text, ",") {
		if part = strings.Join(strings.Fields(part), " "); part != "" {
			parts = append(parts, part)
		}
	}
	var next string
	switch {
	case len(parts) >= 2:
		next = strings.Join(parts[1:], ", ")
	case len(parts) == 1:
		words := strings.Fields(parts[0])
		if len(words) < 2 {
			return "", false
		}
		next = strings.Join(words[1:], " ")
	default:
		return "", false
	}
	if utf8.RuneCountInString(next) < minShorterRunes {
		return "", false
	}
	return next, true
}

// searchPlan is the ordered list of upstream requests for one search. Each
// text, the whole one first, is tried inside the box around near and then
// widened; without near there is only the widened try. A text counts as
// finding nothing only when both of its tries came back empty, and only then
// is a shorter text tried. The list is cut at maxUpstreamPerSearch, so with a
// position the second shorter text is never reached.
func searchPlan(q string, near *Point) []attempt {
	var plan []attempt
	for i, text := range searchTexts(q) {
		if near != nil {
			plan = append(plan, attempt{text: text, boxed: true, shortened: i > 0})
		}
		plan = append(plan, attempt{text: text, boxed: false, shortened: i > 0})
	}
	if len(plan) > maxUpstreamPerSearch {
		plan = plan[:maxUpstreamPerSearch]
	}
	return plan
}

// run sends the Nominatim search plan in order and stops at the first try
// that finds something. take is called before every request with the try's
// index and decides how the lane gets its limiter token; its error ends the
// search. maxRequests cuts the plan further when Photon already used one of
// the search's maxUpstreamPerSearch requests.
func (c *Client) run(ctx context.Context, q string, near *Point, take func(ctx context.Context, i int) error, maxRequests int) (Answer, error) {
	plan := searchPlan(q, near)
	if len(plan) > maxRequests {
		plan = plan[:max(maxRequests, 0)]
	}
	for i, a := range plan {
		if err := take(ctx, i); err != nil {
			return Answer{}, err
		}
		results, err := c.fetch(ctx, a.text, a.params(near))
		if err != nil {
			return Answer{}, err
		}
		if len(results) > 0 {
			answer := Answer{Results: results, provider: providerNominatim}
			if a.shortened {
				answer.MatchedQuery = a.text
			}
			return answer, nil
		}
	}
	return Answer{Results: []Result{}, provider: providerNominatim}, nil
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
// up instead of queueing. The handler maps it to the plain "Search
// unavailable" message (D-16, D-28).
var ErrReaderBusy = errors.New("geocode: reader lane busy")

// Point is a position a search looks around first: the report pin for the
// report form's search, the map centre for the reader's place search.
type Point struct {
	Lat, Lon float64
}

// Cache tuning (03.1 RESEARCH A4), shared by both lanes, each of which has a
// cache of its own. All three are easy to retune: they are plain constants
// with no other code depending on their values.
const (
	// searchCacheEntries bounds each in memory cache of searches.
	searchCacheEntries = 256
	// searchCacheTTL is how long a cached answer is served before the next
	// identical search goes upstream again. Place names barely change.
	searchCacheTTL = 6 * time.Hour
	// cacheBucketDegrees is the grid a position is rounded to for the cache
	// key, about 11 km, so one area's answer is never served for a different
	// area but moving a little still hits the cache.
	cacheBucketDegrees = 0.1
)

// readerWakeSlack is added to a reader's computed wait for its next token, so
// float rounding at the boundary does not make the Allow after the wait fail.
const readerWakeSlack = 5 * time.Millisecond

// answerCache is a small bounded TTL cache of search answers: a mutex, a map,
// and an insertion ordered list for eviction. now is a field so tests can
// move the clock.
type answerCache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]*list.Element
	order   *list.List // front is the oldest entry
}

type answerCacheEntry struct {
	key    string
	answer Answer
	stored time.Time
}

func newAnswerCache() *answerCache {
	return &answerCache{
		now:     time.Now,
		entries: make(map[string]*list.Element),
		order:   list.New(),
	}
}

// copyAnswer copies the result list, so a caller can never change a cached
// answer. An empty list stays a non nil empty list.
func copyAnswer(a Answer) Answer {
	return Answer{Results: append([]Result{}, a.Results...), MatchedQuery: a.MatchedQuery, provider: a.provider}
}

// get returns a fresh cached answer, dropping an expired one.
func (ac *answerCache) get(key string) (Answer, bool) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	el, ok := ac.entries[key]
	if !ok {
		return Answer{}, false
	}
	entry := el.Value.(*answerCacheEntry)
	if ac.now().Sub(entry.stored) >= searchCacheTTL {
		ac.order.Remove(el)
		delete(ac.entries, key)
		return Answer{}, false
	}
	return copyAnswer(entry.answer), true
}

// put stores an answer and evicts the oldest entry past the bound.
func (ac *answerCache) put(key string, answer Answer) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	stored := copyAnswer(answer)
	if el, ok := ac.entries[key]; ok {
		entry := el.Value.(*answerCacheEntry)
		entry.answer = stored
		entry.stored = ac.now()
		return
	}
	ac.entries[key] = ac.order.PushBack(&answerCacheEntry{key: key, answer: stored, stored: ac.now()})
	for ac.order.Len() > searchCacheEntries {
		oldest := ac.order.Front()
		ac.order.Remove(oldest)
		delete(ac.entries, oldest.Value.(*answerCacheEntry).key)
	}
}

// size reports how many entries the cache holds.
func (ac *answerCache) size() int {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return ac.order.Len()
}

// cacheKey builds the cache key from the normalised original text (trimmed,
// lower cased, runs of whitespace collapsed) and the position rounded to
// cacheBucketDegrees, or the word none when there is no position. A shorter
// text a search fell back to is never a key: the answer is stored under what
// the person typed.
func cacheKey(q string, near *Point) string {
	text := strings.ToLower(strings.Join(strings.Fields(q), " "))
	if near == nil {
		return text + "|none"
	}
	latBucket := int(math.Round(near.Lat / cacheBucketDegrees))
	lonBucket := int(math.Round(near.Lon / cacheBucketDegrees))
	return text + "|" + strconv.Itoa(latBucket) + "," + strconv.Itoa(lonBucket)
}

// SearchReader is the reader's place search lane (HIST-05). It runs the same
// search as Search and shares both process wide limiters with it, but it
// never holds a place in the limiter's queue, so a reader can never jump
// ahead of a reporter filing an emergency (D-16, D-28):
//
//   - A cached answer costs no token.
//   - The first try on each provider takes a token only if one is free right
//     now, and returns ErrReaderBusy if not.
//   - A later Nominatim try (the widened one or a shorter text) waits for
//     the limiter without reserving a token: it gives up at once if a reporter is
//     already waiting (the bucket is in debt), otherwise it sleeps until a
//     token would be free and then takes it only if it still is. A reporter
//     who started waiting in that gap gets the token and the reader gets
//     ErrReaderBusy. This is how quick task 261016-adr keeps "the limiter is
//     waited on for every request" without breaking D-16.
//
// Running out of limiter time part way through is ErrReaderBusy, never an
// empty list. Answers are cached as Search's are; errors never are.
//
// Photon comes first here too, on its own limiter and by the same rule: a
// token only if one is free right now. When none is free the place search is
// busy at once (ErrReaderBusy) rather than going on to Nominatim, so a busy
// Photon never moves readers onto the Nominatim budget report searches fall
// back to.
func (c *Client) SearchReader(ctx context.Context, q string, near *Point) (Answer, error) {
	return c.search(ctx, q, near, c.reader, c.takeReaderPhotonToken, c.takeReaderToken)
}

// takeReaderToken is the reader lane's turn at the Nominatim limiter for
// try i.
func (c *Client) takeReaderToken(ctx context.Context, i int) error {
	return takeReaderTokenFrom(ctx, c.limiter, i)
}

// takeReaderTokenFrom is the reader rule on any limiter: the first try takes
// a token only if one is free right now; a later try waits without reserving
// one and gives up when a reporter is already waiting.
func takeReaderTokenFrom(ctx context.Context, limiter *rate.Limiter, i int) error {
	if i > 0 {
		tokens := limiter.Tokens()
		if tokens < 0 {
			// A reporter's Wait already holds the next token.
			return ErrReaderBusy
		}
		if tokens < 1 {
			wait := time.Duration((1-tokens)/float64(limiter.Limit())*float64(time.Second)) + readerWakeSlack
			if wait > limiterWaitTimeout {
				return ErrReaderBusy
			}
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	if !limiter.Allow() {
		return ErrReaderBusy
	}
	return nil
}
