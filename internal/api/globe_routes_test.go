package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/api/handlers"
	"pinalert/internal/session"
)

// Amended 2026-11-18 by quick 261118-v75: the 85 pictures ship embedded in the
// binary, so the fake NASA upstream these tests pointed the router at
// (fakeGlobeUpstream, Deps.GlobeUpstream) is gone; the router now serves the
// embedded pictures and every assertion below is otherwise unchanged.

// TestGlobeTilePathsAreRoutes proves the list and the router agree: every entry
// of handlers.GlobeTilePaths answers 200 through the real router with no cookie
// and no Set-Cookie, and an address outside the list is not a route at all. It
// needs no database, because a tile request skips the session middleware.
func TestGlobeTilePathsAreRoutes(t *testing.T) {
	mgr, err := session.NewManager([]byte("globe-routes-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing the session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing the page template: %v", err)
	}
	router := NewRouter(Deps{Session: mgr, Template: tmpl})

	for _, p := range handlers.GlobeTilePaths() {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, rec.Code)
			continue
		}
		if got := rec.Header().Values("Set-Cookie"); len(got) != 0 {
			t.Errorf("GET %s set a cookie: %v", p, got)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("GET %s Content-Type = %q, want image/jpeg", p, ct)
		}
	}

	// Asked of the route table itself, so no session middleware (and no
	// database) is involved.
	for _, p := range []string{
		"/globe/tiles/4/0/0.jpg",
		"/globe/tiles/0/0/0.png",
		"/globe/tiles/0/0/0.jpg/",
		"/globe/tiles/1/2/0.jpg",
		"/globe/tiles",
		"/globe/tiles/",
	} {
		if router.Match(chi.NewRouteContext(), http.MethodGet, p) {
			t.Errorf("GET %s is a registered route, want only the 85 listed tiles", p)
		}
	}
	// Only GET is registered.
	if router.Match(chi.NewRouteContext(), http.MethodPost, "/globe/tiles/0/0/0.jpg") {
		t.Errorf("POST /globe/tiles/0/0/0.jpg is a registered route, want GET only")
	}
}

// TestGlobeTilesSetNoCookie runs on a real database (newGateTestServerWith). A
// cookieless GET of a tile answers 200 with no Set-Cookie header and writes no
// sessions row, even all 85 of them; every near miss, other method and encoded
// spelling gets a cookie and exactly one new row (T-05-19, research Pitfall 2).
// It counts a shared table, so it never runs in parallel.
func TestGlobeTilesSetNoCookie(t *testing.T) {
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
	resp := do(http.MethodGet, "/globe/tiles/0/0/0.jpg")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /globe/tiles/0/0/0.jpg = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("a tile set a cookie: %v", got)
	}
	for _, p := range handlers.GlobeTilePaths() {
		resp := do(http.MethodGet, p)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, resp.StatusCode)
		}
		if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
			t.Errorf("GET %s set a cookie: %v", p, got)
		}
	}
	if after := count(); after != before {
		t.Fatalf("the 85 tiles changed the sessions count from %d to %d, want no change", before, after)
	}

	misses := []struct{ method, path string }{
		{http.MethodGet, "/globe/tiles/0/0/0.jpg/"},
		{http.MethodGet, "/globe/tiles/0/0/0.jpg.map"},
		{http.MethodGet, "/globe/tiles/4/0/0.jpg"},
		{http.MethodGet, "/globe/tiles/0/0/0%2Ejpg"},
		{http.MethodGet, "/globe/tiles/0/0/../0/0.jpg"},
		{http.MethodGet, "/globe/tiles"},
		{http.MethodHead, "/globe/tiles/0/0/0.jpg"},
		{http.MethodPost, "/globe/tiles/0/0/0.jpg"},
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
