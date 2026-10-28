package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/api/handlers"
	"pinalert/internal/session"
)

// uiSpecPWAPaths are the ten public app asset addresses of UI-SPEC Surface 6b
// and the manifest, written out here on purpose so a change to
// handlers.PWAPublicPaths has to be made in two places (review W1).
var uiSpecPWAPaths = []string{
	"/manifest.webmanifest",
	"/sw.js",
	"/icons/app.svg",
	"/icons/app-192.png",
	"/icons/app-512.png",
	"/icons/app-maskable-512.png",
	"/icons/apple-touch-icon.png",
	"/icons/badge-96.png",
	"/apple-touch-icon.png",
	"/apple-touch-icon-precomposed.png",
}

// TestSkipSessionFor proves the session bypass is exact: only a GET of a listed
// path with no encoded form skips the session middleware; every near miss, every
// other method and an encoded spelling still runs it (T-03.3-14). It needs no
// database.
//
// Amended 2026-10-28 by plan 05-04 (D-13, D-04, D-05): the bypass is now one rule
// set, sessionSkip, with exact GET paths, exact POST paths and path prefixes for
// GET and POST. The seven original rows are kept unchanged; rows were added for a
// POST of an exact post path, the prefix, dot segments, a double slash, a wrong
// case, a HEAD and an encoded spelling under the prefix.
func TestSkipSessionFor(t *testing.T) {
	ran := 0
	counting := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ran++
			next.ServeHTTP(w, r)
		})
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := skipSessionFor(sessionSkip{
		get:      map[string]bool{"/a": true},
		post:     map[string]bool{"/q": true},
		prefixes: []string{"/p"},
	}, counting)(ok)

	encoded := httptest.NewRequest(http.MethodGet, "/a", nil)
	encoded.URL.Path = "/a"
	encoded.URL.RawPath = "/%61"
	encodedPrefix := httptest.NewRequest(http.MethodGet, "/p/x", nil)
	encodedPrefix.URL.Path = "/p/x"
	encodedPrefix.URL.RawPath = "/p/%78"

	cases := []struct {
		name string
		req  *http.Request
		runs int
	}{
		{"exact GET skips", httptest.NewRequest(http.MethodGet, "/a", nil), 0},
		{"trailing slash", httptest.NewRequest(http.MethodGet, "/a/", nil), 1},
		{"map suffix", httptest.NewRequest(http.MethodGet, "/a.map", nil), 1},
		{"other path", httptest.NewRequest(http.MethodGet, "/b", nil), 1},
		{"POST", httptest.NewRequest(http.MethodPost, "/a", nil), 1},
		{"HEAD", httptest.NewRequest(http.MethodHead, "/a", nil), 1},
		{"encoded spelling", encoded, 1},

		// Added by plan 05-04.
		{"POST of an exact post path skips", httptest.NewRequest(http.MethodPost, "/q", nil), 0},
		{"GET of a post only path", httptest.NewRequest(http.MethodGet, "/q", nil), 1},
		{"POST of a get only path", httptest.NewRequest(http.MethodPost, "/a", nil), 1},
		{"prefix itself, GET", httptest.NewRequest(http.MethodGet, "/p", nil), 0},
		{"prefix itself, POST", httptest.NewRequest(http.MethodPost, "/p", nil), 0},
		{"under prefix, GET", httptest.NewRequest(http.MethodGet, "/p/x", nil), 0},
		{"under prefix, POST", httptest.NewRequest(http.MethodPost, "/p/x", nil), 0},
		{"deeper under prefix", httptest.NewRequest(http.MethodGet, "/p/x/y/z", nil), 0},
		{"prefix without slash boundary", httptest.NewRequest(http.MethodGet, "/px", nil), 1},
		{"dot dot out of the prefix", httptest.NewRequest(http.MethodGet, "/p/../a", nil), 1},
		{"dot segment", httptest.NewRequest(http.MethodGet, "/p/./x", nil), 1},
		{"double slash", httptest.NewRequest(http.MethodGet, "/p//x", nil), 1},
		{"ends in dot", httptest.NewRequest(http.MethodGet, "/p/.", nil), 1},
		{"ends in dot dot", httptest.NewRequest(http.MethodGet, "/p/..", nil), 1},
		{"other case", httptest.NewRequest(http.MethodGet, "/P/x", nil), 1},
		{"HEAD under prefix", httptest.NewRequest(http.MethodHead, "/p/x", nil), 1},
		{"PUT under prefix", httptest.NewRequest(http.MethodPut, "/p/x", nil), 1},
		{"encoded spelling under prefix", encodedPrefix, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ran = 0
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, tc.req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if ran != tc.runs {
				t.Fatalf("session middleware ran %d times, want %d", ran, tc.runs)
			}
		})
	}
}

// TestPWASkipSetMatchesRoutes proves the route lists and the bypass set are one
// set: sessionSkipSet's GET paths are exactly the entries of
// handlers.PWAPublicPaths plus handlers.GlobeTilePaths(), each is a registered
// GET route of the real router, and the PWA list is the ten addresses of UI-SPEC
// 6b. It needs no database.
//
// Amended 2026-10-28 by plan 05-04 (D-13): it was over pwaSkipSet, a map of the
// ten PWA paths. It is now over sessionSkipSet, whose GET set also holds the 85
// globe tile paths; every old assertion about the ten paths is kept, and the
// POST set and the prefixes are asserted empty until plans 05-10 and 05-12 add
// the Demo prefix and the refresh endpoint.
//
// Amended 2026-10-29 by plan 05-10 (D-04, D-05): the prefix list is now asserted
// to be exactly ["/demo"]; the POST set is still asserted empty until plan 05-12.
//
// Amended 2026-10-29 by plan 05-12 (D-15): the POST set is no longer asserted
// empty. It is asserted to be exactly ["/internal/official/refresh"], and that
// path must be a registered route of the router. The prefixes stay exactly
// ["/demo"] and every GET assertion is unchanged.
//
// Amended 2026-10-29 by the Phase 5 follow up to plan 05-13 (D-05): the prefixes
// are now exactly ["/demo", "/static"], so a cookieless fetch of a stylesheet,
// script or icon writes no sessions row, and /static/* must be a registered route.
// Every other assertion is unchanged.
func TestPWASkipSetMatchesRoutes(t *testing.T) {
	mgr, err := session.NewManager([]byte("pwa-routes-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing the session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing the page template: %v", err)
	}
	router := NewRouter(Deps{Session: mgr, Template: tmpl})

	getRoutes := map[string]bool{}
	err = chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == http.MethodGet {
			getRoutes[route] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the router: %v", err)
	}

	skip := sessionSkipSet()
	set := skip.get
	tilePaths := handlers.GlobeTilePaths()
	if len(set) != len(handlers.PWAPublicPaths)+len(tilePaths) {
		t.Errorf("sessionSkipSet has %d GET keys, PWAPublicPaths and GlobeTilePaths have %d entries together",
			len(set), len(handlers.PWAPublicPaths)+len(tilePaths))
	}
	if len(tilePaths) != 85 {
		t.Errorf("GlobeTilePaths has %d entries, want 85", len(tilePaths))
	}
	for _, p := range tilePaths {
		if !set[p] {
			t.Errorf("%s is a globe tile path but not in the skip set", p)
		}
		if !getRoutes[p] {
			t.Errorf("%s is a globe tile path but is not a registered GET route", p)
		}
	}
	// Amended 2026-10-29 by plan 05-12 (D-15): it was asserted empty; it is now
	// exactly the one refresh endpoint, which is also a registered route.
	if len(skip.post) != 1 || !skip.post["/internal/official/refresh"] {
		t.Errorf("sessionSkipSet has POST paths %v, want exactly [/internal/official/refresh]", skip.post)
	}
	postRoutes := map[string]bool{}
	_ = chi.Walk(router, func(_, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		postRoutes[route] = true
		return nil
	})
	if !postRoutes["/internal/official/refresh"] {
		t.Errorf("/internal/official/refresh is in the POST skip set but is not a registered route")
	}
	// Amended 2026-10-29 by plan 05-10 (D-04, D-05): the prefix list was asserted
	// empty; it is now exactly the Demo prefix. The POST set assertion above is
	// unchanged.
	// Amended 2026-10-29 by the Phase 5 follow up to plan 05-13 (D-05): "/static"
	// joins the Demo prefix, and the static route is a registered route.
	if !slices.Equal(skip.prefixes, []string{"/demo", "/static"}) {
		t.Errorf("sessionSkipSet prefixes = %v, want exactly [/demo /static]", skip.prefixes)
	}
	if !getRoutes["/static/*"] {
		t.Errorf("/static/* is a skip set prefix but is not a registered GET route")
	}
	if len(handlers.PWAPublicPaths) != 10 {
		t.Errorf("PWAPublicPaths has %d entries, want 10", len(handlers.PWAPublicPaths))
	}
	for _, p := range handlers.PWAPublicPaths {
		if !set[p] {
			t.Errorf("%s is on the list but not in the skip set", p)
		}
		if !getRoutes[p] {
			t.Errorf("%s is on the list but is not a registered GET route", p)
		}
	}
	for p := range set {
		if !slices.Contains(handlers.PWAPublicPaths, p) && !slices.Contains(tilePaths, p) {
			t.Errorf("%s is in the skip set but not on the list", p)
		}
	}
	if !slices.Equal(handlers.PWAPublicPaths, uiSpecPWAPaths) {
		t.Errorf("PWAPublicPaths = %v, want the ten addresses of UI-SPEC 6b %v", handlers.PWAPublicPaths, uiSpecPWAPaths)
	}
}

// TestPWAAssetsSetNoCookie runs on a real database (newGateTestServer). A
// cookieless GET of each of the ten listed paths answers 200 with no Set-Cookie
// header and writes no sessions row; every near miss, other method, encoded
// spelling and /login gets a cookie and exactly one new row (T-03.3-05,
// T-03.3-14). It counts a shared table, so it never runs in parallel.
func TestPWAAssetsSetNoCookie(t *testing.T) {
	srv, pool, _ := newGateTestServer(t)
	ctx := context.Background()

	count := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&n); err != nil {
			t.Fatalf("counting sessions: %v", err)
		}
		return n
	}
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	do := func(method, path string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, nil)
		if err != nil {
			t.Fatalf("building %s %s: %v", method, path, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		resp.Body.Close()
		return resp
	}

	before := count()
	for _, p := range handlers.PWAPublicPaths {
		resp := do(http.MethodGet, p)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, resp.StatusCode)
		}
		if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
			t.Errorf("GET %s set a cookie: %v", p, got)
		}
	}
	if after := count(); after != before {
		t.Fatalf("the ten public paths changed the sessions count from %d to %d, want no change", before, after)
	}

	misses := []struct{ method, path string }{
		{http.MethodGet, "/sw.js/"},
		{http.MethodGet, "/sw.js.map"},
		{http.MethodGet, "/icons/other.png"},
		{http.MethodGet, "/icons/"},
		{http.MethodGet, "/manifest.webmanifest/"},
		{http.MethodGet, "/sw%2Ejs"},
		{http.MethodPost, "/sw.js"},
		{http.MethodHead, "/manifest.webmanifest"},
		{http.MethodGet, "/login"},
	}
	for _, m := range misses {
		n := count()
		resp := do(m.method, m.path)
		if len(resp.Header.Values("Set-Cookie")) == 0 {
			t.Errorf("%s %s set no cookie, want a session cookie", m.method, m.path)
		}
		if got := count(); got != n+1 {
			t.Errorf("%s %s moved the sessions count from %d to %d, want %d", m.method, m.path, n, got, n+1)
		}
	}
}
