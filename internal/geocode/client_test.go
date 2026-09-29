package geocode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	if _, err := c.Search(context.Background(), "Bengaluru"); err != nil {
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
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	c := newClientWithBaseURL(server.URL+"/search", "contact@example.com", defaultLimiterEvery, defaultLimiterBurst)
	q := "New York & Elsewhere"
	if _, err := c.Search(context.Background(), q); err != nil {
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

	wantKeys := map[string]bool{"q": true, "format": true, "limit": true, "email": true}
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
	results, err := c.Search(context.Background(), "Bengaluru")
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
	results, err := c.Search(context.Background(), "test")
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
		if _, err := c.Search(context.Background(), "q"); err != nil {
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
	if _, err := c.Search(context.Background(), "first"); err != nil {
		t.Fatalf("first Search returned error: %v", err)
	}

	start := time.Now()
	_, err := c.Search(context.Background(), "second")
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
	results, err := c.Search(context.Background(), "q")
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
	results, err := c.Search(context.Background(), "q")
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

	_, err := c.Search(ctx, "q")
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
