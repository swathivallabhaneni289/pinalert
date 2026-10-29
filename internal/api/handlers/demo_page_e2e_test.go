// demo_page_e2e_test.go proves the Demo's Nearby page (plan 05-13, D-01, D-05,
// T-05-69, T-05-70, T-05-72, T-05-73). The page starts a visit, so every GET
// /demo is a fresh world; it writes no row and sets no cookie, it names no
// account and no phone number, its Exit button goes only to /login or /welcome,
// and it is rate limited per IP. The first test runs the whole router on a real
// database; the others serve the Demo routes alone and need no database.
package handlers_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/demo"
	sqlcgen "pinalert/internal/store/sqlc"
)

var (
	demoAPIBaseRe = regexp.MustCompile(`data-api-base="([^"]+)"`)
	demoExitRe    = regexp.MustCompile(`<a [^>]*demo-strip__exit[^>]*>`)
	demoHrefRe    = regexp.MustCompile(`href="([^"]*)"`)
)

// demoPageGet fetches one page from a server and returns the response and body.
func demoPageGet(t *testing.T, client *http.Client, url string) (*http.Response, string) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", url, err)
	}
	return resp, string(raw)
}

// demoPageServer serves the Demo routes alone, mounted at /demo as the router
// does, over a store with the real seed. visit is the per IP limit on starting
// a visit; a zero value takes the default.
func demoPageServer(t *testing.T, visit handlers.DemoLimit) *httptest.Server {
	t.Helper()
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing the page templates: %v", err)
	}
	r := chi.NewRouter()
	r.Mount("/demo", handlers.DemoRoutes(handlers.DemoConfig{
		Store:      demo.NewStore(demo.WithWorldSetup(demo.SeedSetup)),
		VisitLimit: visit,
		Template:   tmpl,
		Page: handlers.PageConfig{
			FallbackLat: 12.9716, FallbackLon: 77.5946, DefaultRadiusKm: 10, VoteRadiusKm: 1,
			AssetVersion: "test",
		},
	}))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// TestDemoPageWritesNothing drives the whole router: two cookieless visits to
// /demo answer 200 twice with a different visit each, no cookie, the same-origin
// referrer policy, no store, and not one table row more.
func TestDemoPageWritesNothing(t *testing.T) {
	store := demo.NewStore(demo.WithWorldSetup(demo.SeedSetup))
	srv, _, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.Demo = store
	})
	before := tableCounts(t, pool)
	for _, must := range []string{"sessions", "reports", "votes", "accounts"} {
		if _, ok := before[must]; !ok {
			t.Fatalf("the table list %v lacks %s: the count would prove nothing", before, must)
		}
	}

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var visits []string
	for i := 0; i < 2; i++ {
		resp, body := demoPageGet(t, client, srv.URL+"/demo")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /demo #%d = %d, want 200: %s", i+1, resp.StatusCode, body)
		}
		if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
			t.Fatalf("GET /demo #%d set a cookie: %v", i+1, got)
		}
		if got := resp.Header.Get("Referrer-Policy"); got != "same-origin" {
			t.Errorf("GET /demo #%d Referrer-Policy = %q, want same-origin", i+1, got)
		}
		if got := resp.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("GET /demo #%d Cache-Control = %q, want no-store", i+1, got)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("GET /demo #%d Content-Type = %q, want text/html", i+1, ct)
		}
		m := demoAPIBaseRe.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("GET /demo #%d has no data-api-base: %s", i+1, body)
		}
		if !strings.HasPrefix(m[1], "/demo/api/v/") {
			t.Errorf("GET /demo #%d data-api-base = %q, want a /demo/api/v/ path", i+1, m[1])
		}
		visits = append(visits, m[1])
	}
	if visits[0] == visits[1] {
		t.Fatalf("two visits share one world: %q", visits[0])
	}
	if store.Len() != 2 {
		t.Errorf("the store holds %d worlds after two visits, want 2", store.Len())
	}

	// The api base the page names is a live Demo API: its feed answers 200.
	resp, body := demoPageGet(t, client, srv.URL+visits[0]+"/reports?lat=16.5424&lon=80.6386&radius_km=10")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the page's own api base answers %d, want 200: %s", resp.StatusCode, body)
	}

	// Amended 2026-10-29 by the Phase 5 follow up to plan 05-13 (D-05): the
	// browser fetches the page's stylesheets and scripts with no cookie as well,
	// and those fetches must set no cookie and write no sessions row either.
	for _, asset := range []string{"/static/css/demo.css", "/static/js/demo.js", "/static/js/api-base.js"} {
		resp, body := demoPageGet(t, client, srv.URL+asset)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200: %.80s", asset, resp.StatusCode, body)
		}
		if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
			t.Fatalf("GET %s set a cookie: %v", asset, got)
		}
	}

	if got := tableCounts(t, pool); !reflect.DeepEqual(before, got) {
		t.Fatalf("opening the Demo changed the table row counts\nbefore: %v\nafter:  %v", before, got)
	}
}

// TestDemoPageExitAllowlist pins the Exit address: from=welcome goes to
// /welcome, and nothing else (no value, a free address, another word) goes to
// /login (T-05-69).
func TestDemoPageExitAllowlist(t *testing.T) {
	srv := demoPageServer(t, handlers.DemoLimit{})
	for _, tc := range []struct{ query, want string }{
		{"", "/login"},
		{"?from=welcome", "/welcome"},
		{"?from=https://evil.example", "/login"},
		{"?from=%2F%2Fevil.example", "/login"},
		{"?from=settings", "/login"},
		{"?from=Welcome", "/login"},
		{"?from=welcome&from=https://evil.example", "/welcome"},
	} {
		resp, body := demoPageGet(t, http.DefaultClient, srv.URL+"/demo"+tc.query)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /demo%s = %d, want 200", tc.query, resp.StatusCode)
		}
		tag := demoExitRe.FindString(body)
		if tag == "" {
			t.Fatalf("GET /demo%s has no Exit button", tc.query)
		}
		m := demoHrefRe.FindStringSubmatch(tag)
		if m == nil || m[1] != tc.want {
			t.Errorf("GET /demo%s Exit goes to %v, want %q", tc.query, m, tc.want)
		}
	}
}

// TestDemoPageRateLimited pins the per IP limit on starting a visit: with a
// burst of 2 the third GET /demo from one address answers 429 (T-05-72).
func TestDemoPageRateLimited(t *testing.T) {
	srv := demoPageServer(t, handlers.DemoLimit{Every: time.Hour, Burst: 2})
	for i := 1; i <= 3; i++ {
		resp, _ := demoPageGet(t, http.DefaultClient, srv.URL+"/demo")
		want := http.StatusOK
		if i == 3 {
			want = http.StatusTooManyRequests
		}
		if resp.StatusCode != want {
			t.Fatalf("GET /demo #%d = %d, want %d", i, resp.StatusCode, want)
		}
		if i == 3 {
			if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
				t.Errorf("the 429 Content-Type = %q, want text/plain (a page, not the API's JSON)", got)
			}
		}
	}
}

// TestDemoVisitLimitChargesOnlyNewWorlds pins WR-S06 of the Phase 5 review: the
// per address visit limit is spent only when a page starts a new world. Moving
// between the three places of a living visit (?v=) is never refused, while a
// fresh visit, an unknown visit or a malformed one is charged.
func TestDemoVisitLimitChargesOnlyNewWorlds(t *testing.T) {
	srv := demoPageServer(t, handlers.DemoLimit{Every: time.Hour, Burst: 1})
	resp, body := demoPageGet(t, http.DefaultClient, srv.URL+"/demo")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /demo = %d, want 200", resp.StatusCode)
	}
	m := demoAPIBaseRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("the Demo page names no API base")
	}
	visit := m[1][strings.LastIndex(m[1], "/")+1:]
	for i := 0; i < 40; i++ {
		for _, place := range []string{"/demo/alerts", "/demo/activity", "/demo"} {
			resp, _ := demoPageGet(t, http.DefaultClient, srv.URL+place+"?v="+visit)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("move %d to %s of a living visit = %d, want 200", i+1, place, resp.StatusCode)
			}
		}
	}
	for _, q := range []string{"", "?v=" + strings.Repeat("0", len(visit)), "?v=nonsense"} {
		resp, _ := demoPageGet(t, http.DefaultClient, srv.URL+"/demo/alerts"+q)
		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("GET /demo/alerts%s with the burst spent = %d, want 429: a new world is charged", q, resp.StatusCode)
		}
	}
}

// TestDemoPageRendersWhatItPromises renders the page and pins what a visitor
// and a screen reader get: the strip first, the pretend position on the shell,
// the Demo places in both bars, and nothing that asks for an account, a
// position, a bell, a person menu or a phone call (T-05-70, T-05-73, T-05-75).
func TestDemoPageRendersWhatItPromises(t *testing.T) {
	srv := demoPageServer(t, handlers.DemoLimit{})
	resp, body := demoPageGet(t, http.DefaultClient, srv.URL+"/demo/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /demo/ = %d, want 200", resp.StatusCode)
	}
	strip := strings.Index(body, "Demo. Nothing here is real.")
	bar := strings.Index(body, `class="app-topbar"`)
	if strip == -1 || bar == -1 || strip > bar {
		t.Errorf("the Demo strip must come before the top bar (strip %d, bar %d)", strip, bar)
	}
	for _, want := range []string{
		`data-demo="1"`,
		`data-fixed-lat="` + strconv.FormatFloat(demo.PretendLat, 'f', -1, 64) + `"`,
		`data-fixed-lon="` + strconv.FormatFloat(demo.PretendLon, 'f', -1, 64) + `"`,
		`data-fixed-place="` + demo.PretendPlace + `"`,
		// Amended 2026-10-29 by plan 05-19: the places of the Demo bars carry the
		// visit (?v=<visit>), so the three places of one visit show one world.
		// The wordmark still points at the bare /demo.
		`href="/demo"`, `href="/demo/alerts?v=`, `href="/demo/activity?v=`,
		`<h1 class="app-topbar__title">Pinalert</h1>`, `href="/demo?post=1"`,
		// Amended 2026-10-29 by the Phase 5 review fix (WR-T07): Exit demo is named by
		// its visible text, so the aria-label "Exit the demo" is gone (WCAG 2.5.3).
		`>Exit demo</a>`, `id="theme-toggle"`, `/static/js/api-base.js`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the Demo page is missing %s", want)
		}
	}
	for _, banned := range []string{
		`aria-label="Exit the demo"`, "data-account-id", "data-push-key", "tel:", "bell-trigger", "app-topbar__gear",
		"account-menu", "map-search", "Unofficial, not a government service", "notice-strip",
		"/static/js/queue.js", "/static/js/notify.js", "/static/js/flag.js", "/static/js/mute.js",
		"/static/js/position.js", "manifest.webmanifest", `href="/alerts"`, `href="/profile`,
		`href="/settings"`, `href="/?post=1"`, `href="/"`,
	} {
		if strings.Contains(body, banned) {
			t.Errorf("the Demo page must not contain %s", banned)
		}
	}
}
