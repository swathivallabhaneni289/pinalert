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
func TestSkipSessionFor(t *testing.T) {
	ran := 0
	counting := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ran++
			next.ServeHTTP(w, r)
		})
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := skipSessionFor(map[string]bool{"/a": true}, counting)(ok)

	encoded := httptest.NewRequest(http.MethodGet, "/a", nil)
	encoded.URL.Path = "/a"
	encoded.URL.RawPath = "/%61"

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

// TestPWASkipSetMatchesRoutes proves the route list and the bypass set are one
// set: pwaSkipSet has exactly the entries of handlers.PWAPublicPaths, each is a
// registered GET route of the real router, and the list is the ten addresses of
// UI-SPEC 6b. It needs no database.
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

	set := pwaSkipSet()
	if len(set) != len(handlers.PWAPublicPaths) {
		t.Errorf("pwaSkipSet has %d keys, PWAPublicPaths has %d entries", len(set), len(handlers.PWAPublicPaths))
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
		if !slices.Contains(handlers.PWAPublicPaths, p) {
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
