package official

// Tests of the GDACS client, parser and filters (plan 05-21). No test here
// reaches the real GDACS host: the client is pointed at an httptest server
// through its test seam, and the answers are the two recorded SEARCH fixtures.

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

// gdacsNow is the clock the client tests run at: the day the fixtures were
// recorded.
var gdacsNow = time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)

func parseGdacsFixture(t *testing.T, name string) []Hazard {
	t.Helper()
	hs, err := ParseGdacs(bytes.NewReader(fixture(t, name)))
	if err != nil {
		t.Fatalf("ParseGdacs(%s): %v", name, err)
	}
	return hs
}

func hazardKeys(hs []Hazard) []string {
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, h.EventType+":"+strconv.FormatInt(h.EventID, 10))
	}
	return out
}

func TestParseGdacs(t *testing.T) {
	hs := parseGdacsFixture(t, "gdacs_search_a.json")
	if len(hs) != 5 {
		t.Fatalf("A parsed to %d hazards (%v), want 5", len(hs), hazardKeys(hs))
	}
	h := hs[0]
	if h.EventType != "TC" || h.EventID != 1001345 || h.EpisodeID != 10 {
		t.Errorf("first hazard ids = %s %d %d", h.EventType, h.EventID, h.EpisodeID)
	}
	if h.Name != "Tropical Cyclone ARNAB-26" || h.AlertLevel != "Red" || h.Country != "India" {
		t.Errorf("first hazard text = %q %q %q", h.Name, h.AlertLevel, h.Country)
	}
	// GeoJSON order is longitude then latitude.
	if h.Lon != 81.8 || h.Lat != 16.4 {
		t.Errorf("point = lat %v lon %v, want lat 16.4 lon 81.8", h.Lat, h.Lon)
	}
	if !h.FromDate.Equal(time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)) || !h.ToDate.Equal(time.Date(2026, 10, 28, 6, 0, 0, 0, time.UTC)) {
		t.Errorf("dates = %v to %v", h.FromDate, h.ToDate)
	}
	if !h.DateModified.Equal(time.Date(2026, 10, 28, 14, 9, 15, 0, time.UTC)) {
		t.Errorf("datemodified = %v", h.DateModified)
	}
	if !h.IsCurrent {
		t.Error(`iscurrent "true" was not read as true`)
	}
	if h.SeverityText != "Tropical Storm (maximum wind speed of 222 km/h)" {
		t.Errorf("severity text = %q", h.SeverityText)
	}
	if h.ReportURL != "https://www.gdacs.org/report.aspx?eventid=1001345&episodeid=10&eventtype=TC" {
		t.Errorf("report url = %q", h.ReportURL)
	}
	// The third feature is a Red earthquake that is no longer current.
	if hs[2].EventType != "EQ" || hs[2].IsCurrent {
		t.Errorf(`third hazard = %s current %v, want EQ and iscurrent "false" read as false`, hs[2].EventType, hs[2].IsCurrent)
	}

	b := parseGdacsFixture(t, "gdacs_search_b.json")
	if len(b) != 8 {
		t.Fatalf("B parsed to %d hazards (%v), want 8", len(b), hazardKeys(b))
	}

	t.Run("unusable answers", func(t *testing.T) {
		for name, body := range map[string]string{
			"not json":          "<<< this is not json",
			"empty":             "",
			"an error message":  `{"message":"Eventtype is required."}`,
			"features not list": `{"type":"FeatureCollection","features":"x"}`,
		} {
			if _, err := ParseGdacs(strings.NewReader(body)); !errors.Is(err, ErrParse) {
				t.Errorf("%s: err = %v, want ErrParse", name, err)
			}
		}
		if hs, err := ParseGdacs(strings.NewReader(`{"type":"FeatureCollection","features":[]}`)); err != nil || len(hs) != 0 {
			t.Errorf("an empty collection gave %v, %v; want no hazards and no error", hs, err)
		}
	})

	t.Run("unusable features are skipped, the rest kept", func(t *testing.T) {
		body := `{"type":"FeatureCollection","features":[
		 {"geometry":{"type":"Point","coordinates":[81.8,16.4]},"properties":{"eventtype":"TC","eventid":1,"alertlevel":"Red","iscurrent":"true"}},
		 {"geometry":{"type":"Polygon","coordinates":[[0,0]]},"properties":{"eventtype":"TC","eventid":2}},
		 {"geometry":{"type":"Point","coordinates":[81.8]},"properties":{"eventtype":"TC","eventid":3}},
		 {"geometry":{"type":"Point","coordinates":[81.8,916.4]},"properties":{"eventtype":"TC","eventid":4}},
		 {"geometry":{"type":"Point","coordinates":[81.8,16.4]},"properties":{"eventtype":"tc","eventid":5}},
		 {"geometry":{"type":"Point","coordinates":[81.8,16.4]},"properties":{"eventtype":"TC","eventid":0}},
		 {"geometry":{"type":"Point","coordinates":[81.8,16.4]},"properties":{"eventtype":"EQ","eventid":6,"alertlevel":"mauve"}}
		]}`
		hs, err := ParseGdacs(strings.NewReader(body))
		if err != nil {
			t.Fatalf("ParseGdacs: %v", err)
		}
		if got := strings.Join(hazardKeys(hs), ","); got != "TC:1,EQ:6" {
			t.Fatalf("kept %s, want TC:1,EQ:6", got)
		}
		if hs[1].AlertLevel != "Green" {
			t.Errorf("an unknown level was stored as %q, want Green", hs[1].AlertLevel)
		}
	})

	t.Run("text is cleaned and capped", func(t *testing.T) {
		long := strings.Repeat("x", 500)
		body := `{"type":"FeatureCollection","features":[
		 {"geometry":{"type":"Point","coordinates":[81.8,16.4]},"properties":{"eventtype":"TC","eventid":1,
		   "name":"Cyclone‮ X\n\tY\u0007","country":"` + long + `","severitydata":{"severitytext":"` + long + `"}}}]}`
		hs, err := ParseGdacs(strings.NewReader(body))
		if err != nil || len(hs) != 1 {
			t.Fatalf("ParseGdacs: %v, %v", hs, err)
		}
		if hs[0].Name != "Cyclone X Y" {
			t.Errorf("name = %q, want control characters dropped and spacing collapsed", hs[0].Name)
		}
		if n := utf8.RuneCountInString(hs[0].Country); n != gdacsTextMaxRunes {
			t.Errorf("country has %d runes, want %d", n, gdacsTextMaxRunes)
		}
		if n := utf8.RuneCountInString(hs[0].SeverityText); n != gdacsTextMaxRunes {
			t.Errorf("severity text has %d runes, want %d", n, gdacsTextMaxRunes)
		}
	})
}

func TestGdacsFilters(t *testing.T) {
	a := parseGdacsFixture(t, "gdacs_search_a.json")
	b := parseGdacsFixture(t, "gdacs_search_b.json")

	keptA := filterGdacsA(a)
	if got := strings.Join(hazardKeys(keptA), ","); got != "TC:1001345,TC:1001338" {
		t.Errorf("A kept %s, want only the two current cyclones (the non current earthquake and cyclone and the drought go)", got)
	}
	keptB := filterGdacsB(b)
	if got := strings.Join(hazardKeys(keptB), ","); got != "TC:1001345,EQ:1574196,EQ:1574148,EQ:1573845" {
		t.Errorf("B kept %s, want the four points inside the South Asia box (the box edge flood at longitude 100.4776, the far flood, the Philippines cyclone and the drought go)", got)
	}
	for _, h := range append(append([]Hazard(nil), keptA...), keptB...) {
		if h.EventType == "DR" {
			t.Errorf("a drought was kept: %+v", h)
		}
	}

	merged := mergeHazards(keptA, keptB)
	if got := strings.Join(hazardKeys(merged), ","); got != "TC:1001345,TC:1001338,EQ:1574196,EQ:1574148,EQ:1573845" {
		t.Errorf("merged = %s, want five hazards with the cyclone in both lists once", got)
	}

	t.Run("a repeat keeps the later modification", func(t *testing.T) {
		old := Hazard{EventType: "TC", EventID: 7, Name: "old", DateModified: gdacsNow}
		newer := Hazard{EventType: "TC", EventID: 7, Name: "newer", DateModified: gdacsNow.Add(time.Hour)}
		if got := mergeHazards([]Hazard{old}, []Hazard{newer}); len(got) != 1 || got[0].Name != "newer" {
			t.Errorf("merged = %+v, want the newer copy once", got)
		}
		if got := mergeHazards([]Hazard{newer}, []Hazard{old}); len(got) != 1 || got[0].Name != "newer" {
			t.Errorf("merged = %+v, want the newer copy once", got)
		}
		same := Hazard{EventType: "TC", EventID: 7, Name: "same time", DateModified: gdacsNow}
		if got := mergeHazards([]Hazard{old}, []Hazard{same}); len(got) != 1 || got[0].Name != "old" {
			t.Errorf("merged = %+v, want the first list's copy on a tie", got)
		}
		if got := mergeHazards([]Hazard{{EventType: "TC", EventID: 7}}, []Hazard{{EventType: "EQ", EventID: 7}}); len(got) != 2 {
			t.Errorf("the same id under two event types merged into %d, want 2", len(got))
		}
	})

	t.Run("the box includes its edges", func(t *testing.T) {
		for _, c := range []struct {
			lat, lon float64
			in       bool
		}{
			{5, 60, true}, {38, 100, true}, {20, 80, true},
			{4.99, 80, false}, {38.01, 80, false}, {20, 59.99, false}, {20, 100.01, false}, {-23.37, -48.18, false},
		} {
			if got := InSouthAsiaBox(c.lat, c.lon); got != c.in {
				t.Errorf("InSouthAsiaBox(%v, %v) = %v, want %v", c.lat, c.lon, got, c.in)
			}
		}
	})
}

// fakeGdacs is the upstream: it answers request A with the A fixture and
// request B with the B fixture (A names volcanoes, B does not) and records the
// queries it saw.
type fakeGdacs struct {
	srv *httptest.Server

	mu      sync.Mutex
	queries []url.Values
	agents  []string
	paths   []string
	mode    string // "", status, garbage, oversize, redirect, slow, failB, empty
	a, b    []byte
}

func newFakeGdacs(t *testing.T) *fakeGdacs {
	t.Helper()
	f := &fakeGdacs{a: fixture(t, "gdacs_search_a.json"), b: fixture(t, "gdacs_search_b.json")}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGdacs) setMode(mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = mode
}

func (f *fakeGdacs) hits() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.queries)
}

func (f *fakeGdacs) serve(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	isA := strings.Contains(q.Get("eventlist"), "VO")
	f.mu.Lock()
	f.queries = append(f.queries, q)
	f.agents = append(f.agents, r.Header.Get("User-Agent"))
	f.paths = append(f.paths, r.URL.Path)
	mode := f.mode
	body := f.b
	if isA {
		body = f.a
	}
	f.mu.Unlock()

	switch {
	case mode == "failB" && !isA:
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	case mode == "status":
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	case mode == "garbage":
		_, _ = w.Write([]byte("<<< this is not json"))
		return
	case mode == "oversize":
		_, _ = w.Write(bytes.Repeat([]byte(" "), GdacsMaxBytes+16))
		return
	case mode == "redirect":
		http.Redirect(w, r, "http://127.0.0.1:1/elsewhere", http.StatusFound)
		return
	case mode == "slow":
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
		return
	case mode == "empty":
		body = []byte(`{"type":"FeatureCollection","features":[]}`)
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func testGdacsClient(base string, cfg ClientConfig, timeout time.Duration) *GdacsClient {
	return newGdacsClientWithBase(base, cfg, timeout)
}

func TestGdacsClientBounds(t *testing.T) {
	t.Run("the production client is fixed to the constant host", func(t *testing.T) {
		if gdacsBaseURL != "https://www.gdacs.org" {
			t.Fatalf("gdacsBaseURL = %q", gdacsBaseURL)
		}
		c := NewGdacsClient(ClientConfig{})
		if c.base != gdacsBaseURL {
			t.Errorf("NewGdacsClient base = %q, want the constant", c.base)
		}
		if gdacsSearchPath != "/gdacsapi/api/events/geteventlist/SEARCH" {
			t.Errorf("search path = %q", gdacsSearchPath)
		}
		// Amended 2026-10-29 by quick 261029-6wb: GdacsTimeout is 40 s per request
		// (was 15 s). The live service took 20.9 s for one request, and A and B now
		// run at the same time, so a refresh takes as long as the slower one.
		if GdacsMaxBytes != 2<<20 || GdacsTimeout != 40*time.Second || GdacsMinGap != 15*time.Minute || GdacsEndedKeep != 48*time.Hour {
			t.Errorf("limits = %d %v %v %v", GdacsMaxBytes, GdacsTimeout, GdacsMinGap, GdacsEndedKeep)
		}
	})

	t.Run("the two requests carry the documented query", func(t *testing.T) {
		f := newFakeGdacs(t)
		c := testGdacsClient(f.srv.URL, ClientConfig{BaseURL: "https://pinalert.example", ContactEmail: "ops@pinalert.example"}, 0)
		res, err := c.FetchHazards(context.Background(), gdacsNow)
		if err != nil {
			t.Fatalf("FetchHazards: %v", err)
		}
		if f.hits() != 2 {
			t.Fatalf("%d requests, want 2 (A and B)", f.hits())
		}
		f.mu.Lock() // the two handlers ran at the same time; read what they recorded under the lock
		defer f.mu.Unlock()
		for i, p := range f.paths {
			if p != gdacsSearchPath {
				t.Errorf("request %d path = %q", i, p)
			}
		}
		want := []url.Values{
			{"eventlist": {"TC;FL;EQ;VO;WF"}, "fromdate": {"2026-10-14"}, "todate": {"2026-10-28"},
				"alertlevel": {"orange;red"}, "pagenumber": {"1"}, "pagesize": {"100"}},
			{"eventlist": {"EQ;TC;FL"}, "fromdate": {"2026-10-25"}, "todate": {"2026-10-28"},
				"alertlevel": {"green;orange;red"}, "pagenumber": {"1"}, "pagesize": {"100"}},
		}
		// Amended 2026-10-29 by quick 261029-6wb: A and B now run at the same
		// time, so the order they arrive in is not fixed. Each recorded query is
		// matched to A or B by content (A is the one whose eventlist holds VO)
		// instead of by arrival index; every assertion below is kept.
		byRequest := make([]url.Values, 2)
		for _, q := range f.queries {
			if strings.Contains(q.Get("eventlist"), "VO") {
				byRequest[0] = q
			} else {
				byRequest[1] = q
			}
		}
		for i, w := range want {
			got := byRequest[i]
			if got == nil {
				t.Fatalf("request %d never arrived", i)
			}
			if len(got) != len(w) {
				t.Errorf("request %d has parameters %v, want exactly %v", i, got, w)
			}
			for k, v := range w {
				if got.Get(k) != v[0] {
					t.Errorf("request %d %s = %q, want %q", i, k, got.Get(k), v[0])
				}
			}
		}
		for _, ua := range f.agents {
			if !strings.HasPrefix(ua, "Pinalert/1.0 (+") || !strings.Contains(ua, "ops@pinalert.example") {
				t.Errorf("User-Agent = %q, want Pinalert/1.0 (+ with the contact", ua)
			}
		}
		if res.Seen != 13 {
			t.Errorf("Seen = %d, want 13 raw features (5 from A, 8 from B)", res.Seen)
		}
		if got := strings.Join(hazardKeys(res.Hazards), ","); got != "TC:1001345,TC:1001338,EQ:1574196,EQ:1574148,EQ:1573845" {
			t.Errorf("hazards = %s", got)
		}
	})

	t.Run("the query is built with url.Values, not pasted in", func(t *testing.T) {
		var seen string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
		}))
		defer srv.Close()
		c := testGdacsClient(srv.URL, ClientConfig{}, 0)
		if _, err := c.Search(context.Background(), SearchRequest{EventTypes: []string{"EQ", "TC"}, From: gdacsNow, To: gdacsNow, Levels: []string{"red"}}); err != nil {
			t.Fatalf("Search: %v", err)
		}
		if strings.Contains(seen, ";") || !strings.Contains(seen, "eventlist=EQ%3BTC") {
			t.Errorf("raw query = %q, want the semicolons escaped", seen)
		}
	})

	failures := []struct {
		name  string
		mode  string
		class string
	}{
		{"a redirect is refused and not followed", "redirect", "status"},
		{"a 500", "status", "status"},
		{"a garbage body", "garbage", "parse"},
		{"a body over 2 MB", "oversize", "parse"},
		{"a slow server", "slow", "timeout"},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGdacs(t)
			f.setMode(tc.mode)
			c := testGdacsClient(f.srv.URL, ClientConfig{}, 150*time.Millisecond)
			_, err := c.FetchHazards(context.Background(), gdacsNow)
			if err == nil {
				t.Fatal("no error")
			}
			if got := ErrorClass(err); got != tc.class {
				t.Errorf("class = %q, want %q (%v)", got, tc.class, err)
			}
			if strings.Contains(err.Error(), f.srv.URL) || strings.Contains(err.Error(), "eventlist") {
				t.Errorf("the error text holds the address or query: %q", err.Error())
			}
			if !strings.HasPrefix(err.Error(), "official: gdacs ") {
				t.Errorf("error text = %q, want it to name gdacs", err.Error())
			}
		})
	}

	t.Run("a failure in B fails the whole run", func(t *testing.T) {
		f := newFakeGdacs(t)
		f.setMode("failB")
		c := testGdacsClient(f.srv.URL, ClientConfig{}, 0)
		res, err := c.FetchHazards(context.Background(), gdacsNow)
		if err == nil || ErrorClass(err) != "status" {
			t.Fatalf("err = %v, want a status error", err)
		}
		if len(res.Hazards) != 0 {
			t.Errorf("a failed run returned %d hazards, want none", len(res.Hazards))
		}
	})

	t.Run("an unreachable host is a network error", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		base := srv.URL
		srv.Close()
		c := testGdacsClient(base, ClientConfig{}, time.Second)
		if _, err := c.FetchHazards(context.Background(), gdacsNow); ErrorClass(err) != "network" {
			t.Errorf("class = %q, want network (%v)", ErrorClass(err), err)
		}
	})
}

func TestGdacsReportURLAllowlist(t *testing.T) {
	for _, c := range []struct {
		in   string
		keep bool
	}{
		{"https://www.gdacs.org/report.aspx?eventid=1001345&episodeid=10&eventtype=TC", true},
		{"https://www.gdacs.org/", true},
		{"http://www.gdacs.org/report.aspx", false},
		{"https://www.gdacs.org.evil.example/report.aspx", false},
		{"https://www.gdacs.org@evil.example/", false},
		{"https://evil.example/https://www.gdacs.org/", false},
		{"javascript:alert(1)", false},
		{"//www.gdacs.org/report.aspx", false},
		{"https://www.gdacs.org/a b", false},
		{"https://www.gdacs.org/a\nb", false},
		{"https://www.gdacs.org/\\@evil.example", false},
		{"https://www.gdacs.org/\"onmouseover=\"x", false},
		{"", false},
	} {
		got := SafeGdacsURL(c.in)
		switch {
		case c.keep && got != c.in:
			t.Errorf("SafeGdacsURL(%q) = %q, want it kept", c.in, got)
		case !c.keep && got != "":
			t.Errorf("SafeGdacsURL(%q) = %q, want empty", c.in, got)
		}
	}

	// The same rule applies when a feature is parsed.
	body := `{"features":[
	 {"geometry":{"type":"Point","coordinates":[81.8,16.4]},"properties":{"eventtype":"TC","eventid":1,"url":{"report":"https://www.gdacs.org.evil.example/x"}}},
	 {"geometry":{"type":"Point","coordinates":[81.8,16.4]},"properties":{"eventtype":"TC","eventid":2,"url":{"report":"https://www.gdacs.org/report.aspx?eventid=2"}}}]}`
	hs, err := ParseGdacs(strings.NewReader(body))
	if err != nil || len(hs) != 2 {
		t.Fatalf("ParseGdacs: %v, %v", hs, err)
	}
	if hs[0].ReportURL != "" || hs[1].ReportURL != "https://www.gdacs.org/report.aspx?eventid=2" {
		t.Errorf("report urls = %q, %q", hs[0].ReportURL, hs[1].ReportURL)
	}
}

func TestHazardWordsAndLevels(t *testing.T) {
	for in, want := range map[string]string{
		"EQ": "Earthquake", "TC": "Cyclone", "FL": "Flood", "VO": "Volcano", "WF": "Wildfire",
		"DR": "Hazard", "": "Hazard", "XX": "Hazard", "eq": "Hazard",
	} {
		if got := HazardWord(in); got != want {
			t.Errorf("HazardWord(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"Red": "Red", "red": "Red", "RED": "Red", " Orange ": "Orange", "orange": "Orange",
		"Green": "Green", "green": "Green", "": "Green", "purple": "Green",
	} {
		if got := GdacsLevelWord(in); got != want {
			t.Errorf("GdacsLevelWord(%q) = %q, want %q", in, got, want)
		}
	}
}

// fakeHazardStore records what the fetcher stores.
type fakeHazardStore struct {
	mu    sync.Mutex
	rows  []Hazard
	ended []Hazard
	err   error
}

func (s *fakeHazardStore) upsert(_ context.Context, h Hazard, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.rows = append(s.rows, h)
	return nil
}

// end records an event the fetcher marked not current (WR-S03).
func (s *fakeHazardStore) end(_ context.Context, h Hazard, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.ended = append(s.ended, h)
	return nil
}

// TestGdacsEndedA pins WR-S03 of the Phase 5 review: the events request A marks
// not current, droughts aside, are handed on to be marked not current.
func TestGdacsEndedA(t *testing.T) {
	a := parseGdacsFixture(t, "gdacs_search_a.json")
	b := parseGdacsFixture(t, "gdacs_search_b.json")
	kept := mergeHazards(filterGdacsA(a), filterGdacsB(b))
	if got := strings.Join(hazardKeys(endedGdacsA(a, kept)), ","); got != "EQ:1573141,TC:1001335" {
		t.Errorf("ended = %s, want the non current earthquake and cyclone of request A", got)
	}
}

// TestGdacsFetcher: the fetcher stores the kept hazards, counts the raw
// features as ItemsSeen (so a quiet week is not read as an anomaly), and stores
// nothing at all when either request failed.
func TestGdacsFetcher(t *testing.T) {
	f := newFakeGdacs(t)
	store := &fakeHazardStore{}
	fetch := newGdacsFetcher(testGdacsClient(f.srv.URL, ClientConfig{}, 0),
		func() hazardStore { return store }, func() time.Time { return gdacsNow })

	res, err := fetch.Fetch(context.Background(), FeedStatus{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.ItemsSeen != 13 || res.Fetched != 13 || res.Stored != 5 || len(store.rows) != 5 {
		t.Errorf("result = %+v with %d rows stored, want 13 seen, 5 stored", res, len(store.rows))
	}

	t.Run("an empty week is a clean success, not an anomaly", func(t *testing.T) {
		f.setMode("empty")
		store.rows = nil
		res, err := fetch.Fetch(context.Background(), FeedStatus{ItemsSeen: 50})
		if err != nil || res.ItemsSeen != 0 || res.Stored != 0 {
			t.Errorf("result = %+v, err %v; want zero seen and no error from the fetcher", res, err)
		}
	})

	t.Run("a failed request stores nothing", func(t *testing.T) {
		f.setMode("failB")
		store.rows = nil
		if _, err := fetch.Fetch(context.Background(), FeedStatus{}); ErrorClass(err) != "status" {
			t.Fatalf("err = %v, want a status error", err)
		}
		if len(store.rows) != 0 {
			t.Errorf("%d rows stored by a failed run, want none", len(store.rows))
		}
	})

	t.Run("a failed write fails the run with a closed word", func(t *testing.T) {
		f.setMode("")
		store.err = errors.New("connection reset by peer 10.0.0.1:5432")
		_, err := fetch.Fetch(context.Background(), FeedStatus{})
		if err == nil || ErrorClass(err) != "network" {
			t.Fatalf("err = %v, want a network class error", err)
		}
	})
}

// pairServer is an upstream that routes by "VO" in the raw query: a request that
// names volcanoes is request A, any other is request B. That works with either
// way of writing the semicolons.
func pairServer(t *testing.T, a, b http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "VO") {
			a(w, r)
			return
		}
		b(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// answerAfter serves body after d, or gives up when the request's context ends.
func answerAfter(d time.Duration, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(d):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
}

// TestGdacsRequestsRunTogether (quick 261029-6wb): request A and request B are in
// flight at the same time, the call takes as long as the slower one, the first
// failure ends the call at once and cancels the other request, and the caller's
// deadline still bounds both.
func TestGdacsRequestsRunTogether(t *testing.T) {
	a, b := fixture(t, "gdacs_search_a.json"), fixture(t, "gdacs_search_b.json")

	t.Run("A and B are in flight at the same time and the result is unchanged", func(t *testing.T) {
		var inflight, maxInflight atomic.Int32
		track := func(h http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				n := inflight.Add(1)
				defer inflight.Add(-1)
				for {
					m := maxInflight.Load()
					if n <= m || maxInflight.CompareAndSwap(m, n) {
						break
					}
				}
				h(w, r)
			}
		}
		srv := pairServer(t, track(answerAfter(300*time.Millisecond, a)), track(answerAfter(400*time.Millisecond, b)))
		c := testGdacsClient(srv.URL, ClientConfig{}, 0)

		begin := time.Now()
		res, err := c.FetchHazards(context.Background(), gdacsNow)
		took := time.Since(begin)
		if err != nil {
			t.Fatalf("FetchHazards: %v", err)
		}
		if got := maxInflight.Load(); got != 2 {
			t.Errorf("at most %d requests were in flight together, want 2", got)
		}
		if took >= 650*time.Millisecond {
			t.Errorf("the call took %v, want about the slower request (400 ms), not the sum (700 ms)", took)
		}
		if res.Seen != 13 {
			t.Errorf("Seen = %d, want 13", res.Seen)
		}
		if got := strings.Join(hazardKeys(res.Hazards), ","); got != "TC:1001345,TC:1001338,EQ:1574196,EQ:1574148,EQ:1573845" {
			t.Errorf("hazards = %s, want A's kept list first, then B's, once each", got)
		}
	})

	// failFast: one request answers 500 at once; the other waits 2 s or until its
	// own request context ends, and says which of the two happened.
	failFast := func(t *testing.T, failA bool) {
		ended := make(chan struct{}, 1)
		slow := func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
				ended <- struct{}{}
			case <-time.After(2 * time.Second):
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
			}
		}
		boom := func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", http.StatusInternalServerError) }
		srv := pairServer(t, slow, boom) // A waits, B fails
		if failA {
			srv = pairServer(t, boom, slow) // A fails, B waits
		}
		c := testGdacsClient(srv.URL, ClientConfig{}, 0)

		begin := time.Now()
		res, err := c.FetchHazards(context.Background(), gdacsNow)
		took := time.Since(begin)
		if err == nil {
			t.Fatal("no error")
		}
		if got := ErrorClass(err); got != "status" {
			t.Errorf("class = %q, want status (the first real failure, not the other request's cancellation): %v", got, err)
		}
		if !strings.HasPrefix(err.Error(), "official: gdacs ") {
			t.Errorf("error text = %q, want it to name gdacs", err.Error())
		}
		if len(res.Hazards) != 0 || res.Seen != 0 {
			t.Errorf("a failed call returned %d hazards and Seen %d, want none", len(res.Hazards), res.Seen)
		}
		if took >= time.Second {
			t.Errorf("the call took %v, want it to return at the failure and not wait for the other request", took)
		}
		select {
		case <-ended:
		case <-time.After(2 * time.Second):
			t.Error("the other request's context never ended: it was not cancelled")
		}
	}
	t.Run("a fast failure of B ends the call without waiting for A", func(t *testing.T) { failFast(t, false) })
	t.Run("a fast failure of A ends the call without waiting for B", func(t *testing.T) { failFast(t, true) })

	t.Run("the caller's deadline still bounds both requests", func(t *testing.T) {
		wait := func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(3 * time.Second):
			}
		}
		srv := pairServer(t, wait, wait)
		c := testGdacsClient(srv.URL, ClientConfig{}, 0) // the GdacsTimeout default applies
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		begin := time.Now()
		_, err := c.FetchHazards(ctx, gdacsNow)
		took := time.Since(begin)
		if got := ErrorClass(err); got != "timeout" {
			t.Errorf("class = %q, want timeout (%v)", got, err)
		}
		if took >= time.Second {
			t.Errorf("the call took %v, want it bounded by the caller's 200 ms", took)
		}
	})
}
