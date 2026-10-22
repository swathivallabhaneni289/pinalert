package geocode

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestClient_Search_SetsUserAgent asserts every outbound request carries an
// identifying, non-stock User-Agent header (D-02).
func TestClient_Search_SetsUserAgent(t *testing.T) {
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	c := newClientWithBaseURL(server.URL, "", defaultLimiterEvery, defaultLimiterBurst)
	if _, err := c.Search(context.Background(), "Bengaluru", nil); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if gotUserAgent == "" {
		t.Fatal("User-Agent header was empty")
	}
	if !strings.HasPrefix(gotUserAgent, "Pinalert/") {
		t.Fatalf("User-Agent %q does not start with Pinalert/", gotUserAgent)
	}
	if gotUserAgent == "Go-http-client/1.1" {
		t.Fatal("User-Agent is Go's stock default, not an identifying agent")
	}
}

// TestClient_Search_ForwardsOnlyQueryToFixedEndpoint asserts the outbound
// request path is /search and that only q, format, limit and (when
// configured) email are ever forwarded — never a caller-supplied URL, host,
// or path fragment (T-07-01).
func TestClient_Search_ForwardsOnlyQueryToFixedEndpoint(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotQuery == nil {
			gotPath = r.URL.Path
			gotQuery = r.URL.Query()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	c := newClientWithBaseURL(server.URL+"/search", "contact@example.com", defaultLimiterEvery, defaultLimiterBurst)
	q := "New York & Elsewhere"
	if _, err := c.Search(context.Background(), q, nil); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if gotPath != "/search" {
		t.Fatalf("path = %q, want /search", gotPath)
	}
	if got := gotQuery.Get("q"); got != q {
		t.Fatalf("q = %q, want %q", got, q)
	}
	if got := gotQuery.Get("format"); got != "jsonv2" {
		t.Fatalf("format = %q, want jsonv2", got)
	}
	if got := gotQuery.Get("limit"); got != "5" {
		t.Fatalf("limit = %q, want 5", got)
	}
	if got := gotQuery.Get("email"); got != "contact@example.com" {
		t.Fatalf("email = %q, want contact@example.com", got)
	}

	if got := gotQuery.Get("countrycodes"); got != "in" {
		t.Fatalf("countrycodes = %q, want in", got)
	}

	wantKeys := map[string]bool{"q": true, "format": true, "limit": true, "email": true, "countrycodes": true}
	for key := range gotQuery {
		if !wantKeys[key] {
			t.Fatalf("unexpected forwarded query key %q", key)
		}
	}
}

// TestClient_Search_ParsesStringCoordinates asserts Nominatim's string-typed
// lat/lon fields arrive at the caller as float64, using the exact Bengaluru
// record recorded in 07-RESEARCH.md.
func TestClient_Search_ParsesStringCoordinates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"place_id":252163534,"licence":"Data © OpenStreetMap contributors, ODbL 1.0. http://osm.org/copyright","osm_type":"relation","osm_id":7902476,"lat":"12.9767936","lon":"77.5900820","category":"boundary","type":"administrative","place_rank":14,"importance":0.6364646632219508,"addresstype":"city","name":"Bengaluru","display_name":"Bengaluru, Bangalore North, Bengaluru Urban, Karnataka, India","boundingbox":["12.8334905","13.1426196","77.4598797","77.7840639"]}]`))
	}))
	defer server.Close()

	c := newClientWithBaseURL(server.URL, "", defaultLimiterEvery, defaultLimiterBurst)
	answer, err := c.Search(context.Background(), "Bengaluru", nil)
	results := answer.Results
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	const epsilon = 1e-6
	if diff := results[0].Lat - 12.9767936; diff > epsilon || diff < -epsilon {
		t.Fatalf("Lat = %v, want ~12.9767936", results[0].Lat)
	}
	if diff := results[0].Lon - 77.5900820; diff > epsilon || diff < -epsilon {
		t.Fatalf("Lon = %v, want ~77.5900820", results[0].Lon)
	}
	if results[0].Name != "Bengaluru" {
		t.Fatalf("Name = %q, want Bengaluru", results[0].Name)
	}
	if results[0].DisplayName != "Bengaluru, Bangalore North, Bengaluru Urban, Karnataka, India" {
		t.Fatalf("DisplayName = %q", results[0].DisplayName)
	}
}

// TestClient_Search_SkipsUnparseableRecords asserts a record with an
// unparseable coordinate is skipped, not zero-filled, so a bad row never
// becomes a 0,0 pin.
func TestClient_Search_SkipsUnparseableRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"Bad","display_name":"Bad Record","lat":"not-a-number","lon":"77.59"},{"name":"Good","display_name":"Good Record","lat":"12.97","lon":"77.59"}]`))
	}))
	defer server.Close()

	c := newClientWithBaseURL(server.URL, "", defaultLimiterEvery, defaultLimiterBurst)
	answer, err := c.Search(context.Background(), "test", nil)
	results := answer.Results
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Name != "Good" {
		t.Fatalf("Name = %q, want Good", results[0].Name)
	}
}

// TestClient_Search_SerializesConcurrentCalls asserts the process-wide
// limiter, not the network, paces calls to <=1/token-interval.
func TestClient_Search_SerializesConcurrentCalls(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	c := newClientWithBaseURL(server.URL, "", 120*time.Millisecond, 1)

	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.Search(context.Background(), fmt.Sprintf("place%d", i), nil); err != nil {
			t.Fatalf("Search call %d returned error: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	if got := atomic.LoadInt64(&requestCount); got != 3 {
		t.Fatalf("upstream served %d requests, want 3", got)
	}
	if elapsed < 240*time.Millisecond {
		t.Fatalf("elapsed = %v, want >= 240ms (limiter should have paced the calls)", elapsed)
	}
}

// TestClient_Search_LimiterWaitTimeout asserts a limiter wait longer than
// the bounded timeout returns an error instead of queueing without limit.
func TestClient_Search_LimiterWaitTimeout(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	c := newClientWithBaseURL(server.URL, "", 10*time.Second, 1)

	// First call succeeds and consumes the only token.
	if _, err := c.Search(context.Background(), "first", nil); err != nil {
		t.Fatalf("first Search returned error: %v", err)
	}

	start := time.Now()
	_, err := c.Search(context.Background(), "second", nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("second Search returned nil error, want non-nil (bounded wait should have refused)")
	}
	if elapsed > 3*time.Second {
		t.Fatalf("second Search took %v, want within 3s", elapsed)
	}
	if got := atomic.LoadInt64(&requestCount); got != 1 {
		t.Fatalf("upstream served %d requests, want exactly 1", got)
	}
}

// TestClient_Search_UpstreamNon200 asserts a non-2xx upstream status
// produces a non-nil error and zero results.
func TestClient_Search_UpstreamNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	c := newClientWithBaseURL(server.URL, "", defaultLimiterEvery, defaultLimiterBurst)
	answer, err := c.Search(context.Background(), "q", nil)
	results := answer.Results
	if err == nil {
		t.Fatal("Search returned nil error, want non-nil")
	}
	if len(results) != 0 {
		t.Fatalf("len(results) = %d, want 0", len(results))
	}
}

// TestClient_Search_MalformedBody asserts a malformed JSON body produces a
// non-nil error and zero results.
func TestClient_Search_MalformedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	c := newClientWithBaseURL(server.URL, "", defaultLimiterEvery, defaultLimiterBurst)
	answer, err := c.Search(context.Background(), "q", nil)
	results := answer.Results
	if err == nil {
		t.Fatal("Search returned nil error, want non-nil")
	}
	if len(results) != 0 {
		t.Fatalf("len(results) = %d, want 0", len(results))
	}
}

// TestClient_Search_ContextCancelled asserts a context already past its
// deadline produces a non-nil error and zero outbound requests.
func TestClient_Search_ContextCancelled(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	c := newClientWithBaseURL(server.URL, "", defaultLimiterEvery, defaultLimiterBurst)

	ctx, cancel := context.WithTimeout(context.Background(), -1*time.Second)
	defer cancel()

	_, err := c.Search(ctx, "q", nil)
	if err == nil {
		t.Fatal("Search returned nil error, want non-nil")
	}
	if got := atomic.LoadInt64(&requestCount); got != 0 {
		t.Fatalf("upstream served %d requests, want 0", got)
	}
}

// TestNominatimSearchURLIsHTTPS is the V9 communications control, expressed
// as a test rather than a runtime scheme check, because the unexported test
// seam deliberately points at an http httptest server.
func TestNominatimSearchURLIsHTTPS(t *testing.T) {
	if !strings.HasPrefix(nominatimSearchURL, "https://") {
		t.Fatalf("nominatimSearchURL = %q, want https:// prefix", nominatimSearchURL)
	}
	if !strings.Contains(nominatimSearchURL, "nominatim.openstreetmap.org") {
		t.Fatalf("nominatimSearchURL = %q, want it to contain nominatim.openstreetmap.org", nominatimSearchURL)
	}
}

// readerServer returns an httptest upstream that counts calls, records the
// last query and answers with one Chennai style record.
func readerServer(t *testing.T) (*httptest.Server, *int64, *atomic.Value) {
	t.Helper()
	var calls int64
	var last atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		last.Store(r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"Velachery","display_name":"Velachery, Chennai, Tamil Nadu, India","lat":"12.9791","lon":"80.2212"}]`))
	}))
	t.Cleanup(server.Close)
	return server, &calls, &last
}

// TestSearchReaderNeverWaits asserts the reader lane fails fast when no
// limiter token is free, without calling the upstream (D-16, D-28).
func TestSearchReaderNeverWaits(t *testing.T) {
	server, calls, _ := readerServer(t)
	c := newClientWithBaseURL(server.URL, "", time.Hour, 1)

	if _, err := c.SearchReader(context.Background(), "velachery", nil); err != nil {
		t.Fatalf("first SearchReader returned error: %v", err)
	}

	start := time.Now()
	_, err := c.SearchReader(context.Background(), "adyar", nil)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrReaderBusy) {
		t.Fatalf("second SearchReader error = %v, want ErrReaderBusy", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("second SearchReader took %v, want well under 100ms", elapsed)
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Fatalf("upstream served %d requests, want 1", got)
	}
}

// TestSearchReaderCacheHitCostsNoUpstreamCall asserts a repeated search is
// answered from the cache until the 6 hour TTL passes.
func TestSearchReaderCacheHitCostsNoUpstreamCall(t *testing.T) {
	server, calls, _ := readerServer(t)
	c := newClientWithBaseURL(server.URL, "", time.Millisecond, 10)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	c.reader.now = func() time.Time { return now }

	firstAnswer, err := c.SearchReader(context.Background(), "Velachery Chennai", nil)
	first := firstAnswer.Results
	if err != nil {
		t.Fatalf("first SearchReader returned error: %v", err)
	}
	secondAnswer, err := c.SearchReader(context.Background(), "  velachery   CHENNAI ", nil)
	second := secondAnswer.Results
	if err != nil {
		t.Fatalf("second SearchReader returned error: %v", err)
	}
	if len(first) != 1 || len(second) != 1 || first[0] != second[0] {
		t.Fatalf("cached answer differs: %v vs %v", first, second)
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Fatalf("upstream served %d requests, want 1", got)
	}

	now = now.Add(searchCacheTTL - time.Minute)
	if _, err := c.SearchReader(context.Background(), "velachery chennai", nil); err != nil {
		t.Fatalf("third SearchReader returned error: %v", err)
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Fatalf("upstream served %d requests before the TTL, want 1", got)
	}

	now = now.Add(2 * time.Minute)
	if _, err := c.SearchReader(context.Background(), "velachery chennai", nil); err != nil {
		t.Fatalf("fourth SearchReader returned error: %v", err)
	}
	if got := atomic.LoadInt64(calls); got != 2 {
		t.Fatalf("upstream served %d requests after the TTL, want 2", got)
	}
}

// TestSearchReaderDoesNotStarveSearch asserts a reporter's Search that is
// waiting for a token (leaving the bucket in debt) is never jumped by a
// reader, and is still answered.
func TestSearchReaderDoesNotStarveSearch(t *testing.T) {
	server, _, _ := readerServer(t)
	c := newClientWithBaseURL(server.URL, "", 300*time.Millisecond, 1)

	// The first Search takes the only token at once.
	if _, err := c.Search(context.Background(), "first", nil); err != nil {
		t.Fatalf("first Search returned error: %v", err)
	}

	// The second Search waits for the next token, which puts the bucket in
	// debt for as long as it waits.
	done := make(chan error, 1)
	go func() {
		_, err := c.Search(context.Background(), "second", nil)
		done <- err
	}()
	time.Sleep(60 * time.Millisecond)

	start := time.Now()
	_, err := c.SearchReader(context.Background(), "reader query", nil)
	if !errors.Is(err, ErrReaderBusy) {
		t.Fatalf("SearchReader error = %v, want ErrReaderBusy", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("SearchReader took %v, want it not to queue", elapsed)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waiting Search returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiting Search was never answered")
	}
}

// TestSearchReaderBiasAndBucket asserts the non bounded viewbox, its corner
// order, and the coarse bucket the cache is keyed by.
func TestSearchReaderBiasAndBucket(t *testing.T) {
	server, calls, last := readerServer(t)
	c := newClientWithBaseURL(server.URL, "", time.Millisecond, 20)

	near := &Point{Lat: 13.0827, Lon: 80.2707}
	if _, err := c.SearchReader(context.Background(), "velachery", near); err != nil {
		t.Fatalf("SearchReader returned error: %v", err)
	}
	got := last.Load().(url.Values)
	if got.Get("q") != "velachery" || got.Get("format") != "jsonv2" || got.Get("limit") != "5" {
		t.Fatalf("q, format or limit wrong: %v", got)
	}
	if got.Get("bounded") != "1" {
		t.Fatalf("bounded = %q, want 1", got.Get("bounded"))
	}
	parts := strings.Split(got.Get("viewbox"), ",")
	if len(parts) != 4 {
		t.Fatalf("viewbox = %q, want four numbers", got.Get("viewbox"))
	}
	want := []float64{80.2707 - 0.5, 13.0827 + 0.5, 80.2707 + 0.5, 13.0827 - 0.5}
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			t.Fatalf("viewbox part %q: %v", p, err)
		}
		if math.Abs(v-want[i]) > 1e-4 {
			t.Fatalf("viewbox[%d] = %v, want %v", i, v, want[i])
		}
	}

	// Same 0.1 degree bucket: served from the cache.
	if _, err := c.SearchReader(context.Background(), "velachery", &Point{Lat: 13.0801, Lon: 80.2733}); err != nil {
		t.Fatalf("same bucket SearchReader returned error: %v", err)
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Fatalf("same bucket made %d upstream calls, want 1", got)
	}

	// A different bucket is a different entry.
	// Amended 2026-10-23 by plan 04-32 (owner): the other bucket's point moved
	// from latitude 12.5 to 12.7. At 12.5 the stand in's Velachery row is 54 km
	// away, past nearRadiusKm, so it is now dropped and the search goes on to
	// its widened try; at 12.7 it is 31 km away and still a different bucket.
	if _, err := c.SearchReader(context.Background(), "velachery", &Point{Lat: 12.7, Lon: 80.2707}); err != nil {
		t.Fatalf("other bucket SearchReader returned error: %v", err)
	}
	if got := atomic.LoadInt64(calls); got != 2 {
		t.Fatalf("other bucket made %d upstream calls, want 2", got)
	}

	// No centre: no viewbox, and its own entry.
	if _, err := c.SearchReader(context.Background(), "velachery", nil); err != nil {
		t.Fatalf("nil near SearchReader returned error: %v", err)
	}
	got = last.Load().(url.Values)
	if _, has := got["viewbox"]; has {
		t.Fatalf("viewbox sent with no centre: %v", got)
	}
	if _, has := got["bounded"]; has {
		t.Fatalf("bounded sent with no centre: %v", got)
	}
	if got := atomic.LoadInt64(calls); got != 3 {
		t.Fatalf("nil near made %d upstream calls in total, want 3", got)
	}
}

// TestSearchReaderCacheIsBounded asserts the 257th distinct key evicts the
// oldest entry.
func TestSearchReaderCacheIsBounded(t *testing.T) {
	server, calls, _ := readerServer(t)
	c := newClientWithBaseURL(server.URL, "", time.Microsecond, 1000)

	for i := 0; i < searchCacheEntries+1; i++ {
		if _, err := c.SearchReader(context.Background(), fmt.Sprintf("place %04d", i), nil); err != nil {
			t.Fatalf("SearchReader %d returned error: %v", i, err)
		}
	}
	if got := atomic.LoadInt64(calls); got != int64(searchCacheEntries+1) {
		t.Fatalf("upstream served %d requests, want %d", got, searchCacheEntries+1)
	}
	if n := c.reader.size(); n != searchCacheEntries {
		t.Fatalf("cache holds %d entries, want %d", n, searchCacheEntries)
	}

	// The newest key is still cached, the oldest was evicted.
	before := atomic.LoadInt64(calls)
	if _, err := c.SearchReader(context.Background(), fmt.Sprintf("place %04d", searchCacheEntries), nil); err != nil {
		t.Fatalf("newest SearchReader returned error: %v", err)
	}
	if got := atomic.LoadInt64(calls); got != before {
		t.Fatalf("newest key was not cached (%d calls, was %d)", got, before)
	}
	if _, err := c.SearchReader(context.Background(), "place 0000", nil); err != nil {
		t.Fatalf("oldest SearchReader returned error: %v", err)
	}
	if got := atomic.LoadInt64(calls); got != before+1 {
		t.Fatalf("oldest key was still cached (%d calls, was %d)", got, before)
	}
}

// TestFailedUpstreamErrorHoldsNoReaderData pins IN-04 at the source: the
// error a failed upstream call returns must not carry the request URL, which
// holds the search text, the map centre and the contact email. Callers log
// this error, so leaking it here would put a reader's place into the log.
func TestFailedUpstreamErrorHoldsNoReaderData(t *testing.T) {
	const (
		secretQuery = "flooded-colony-near-my-house"
		secretEmail = "owner-contact@example.org"
	)
	near := &Point{Lat: 13.082680, Lon: 80.270718}
	// The outgoing URL carries the centre only as a viewbox of the centre plus
	// or minus 0.5, so the four viewbox numbers are the coordinate secrets
	// that really travel. The raw centre values are kept as well.
	secrets := []string{
		secretQuery, secretEmail, "13.0826", "80.2707", "viewbox", "email=",
		"79.770718", "13.582680", "80.770718", "12.582680",
	}

	// A server that is already closed refuses the connection, which makes
	// net/http return a *url.Error carrying the full URL.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	baseURL := dead.URL
	dead.Close()

	statusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer statusServer.Close()

	check := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("got nil error, want a failure")
		}
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			t.Fatalf("error still wraps a *url.Error: %v", err)
		}
		for _, secret := range secrets {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("error %q contains %q", err.Error(), secret)
			}
		}
	}

	t.Run("reader lane, connection refused", func(t *testing.T) {
		c := newClientWithBaseURL(baseURL, secretEmail, defaultLimiterEvery, defaultLimiterBurst)
		_, err := c.SearchReader(context.Background(), secretQuery, near)
		check(t, err)
	})
	t.Run("reporter lane, connection refused", func(t *testing.T) {
		c := newClientWithBaseURL(baseURL, secretEmail, defaultLimiterEvery, defaultLimiterBurst)
		_, err := c.Search(context.Background(), secretQuery, nil)
		check(t, err)
	})
	t.Run("reader lane, cancelled context keeps its class", func(t *testing.T) {
		c := newClientWithBaseURL(statusServer.URL, secretEmail, defaultLimiterEvery, defaultLimiterBurst)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := c.SearchReader(ctx, secretQuery, near)
		check(t, err)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("errors.Is(err, context.Canceled) = false for %v", err)
		}
	})
	t.Run("upstream status is a typed error", func(t *testing.T) {
		c := newClientWithBaseURL(statusServer.URL, secretEmail, defaultLimiterEvery, defaultLimiterBurst)
		_, err := c.SearchReader(context.Background(), secretQuery, near)
		check(t, err)
		var statusErr *UpstreamStatusError
		if !errors.As(err, &statusErr) || statusErr.Status != http.StatusBadGateway {
			t.Fatalf("err = %v, want UpstreamStatusError 502", err)
		}
	})
}
