package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pinalert/internal/api/handlers"
	"pinalert/internal/demo"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestDemoSessionSkip proves the Demo writes no sessions row (plan 05-10, D-02,
// D-05, T-05-51, T-05-52). It runs on a real database and drives the real router
// in process, so a path that http.Client or net/http would clean is still
// delivered exactly as written. A cookieless GET and POST under /demo set no
// cookie and leave the sessions count unchanged; the near misses are not Demo
// routes, get a session like any other request, and none answers 200 with Demo
// data. It counts a shared table, so it never runs in parallel.
func TestDemoSessionSkip(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	mgr, err := session.NewManager([]byte("demo-session-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}
	store := demo.NewStore(demo.WithWorldSetup(demo.SeedSetup))
	world, err := store.Create()
	if err != nil {
		t.Fatalf("creating a Demo world: %v", err)
	}
	queries := sqlcgen.New(pool)
	router := NewRouter(Deps{
		Session:    mgr,
		Sessions:   queries,
		Reports:    service.NewReportService(queries),
		Votes:      service.NewVotingService(queries),
		Template:   tmpl,
		RateLimits: gateGenerousRateLimits,
		Demo:       store,
	})

	count := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&n); err != nil {
			t.Fatalf("counting sessions: %v", err)
		}
		return n
	}
	do := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	base := "/demo/api/v/" + world.ID
	feed := base + "/reports?lat=16.5424&lon=80.6386&radius_km=10"
	practice := `{"category":"flood","severity":"low","description":"Water rising near the bridge","latitude":16.5424,"longitude":80.6386}`

	// Under /demo: no cookie and no row, whatever the answer. The first two must
	// be real Demo answers, so the test cannot pass on a route that is missing.
	before := count()
	rec := do(http.MethodGet, feed, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200: %s", feed, rec.Code, rec.Body.String())
	}
	rec = do(http.MethodPost, base+"/reports", practice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST %s/reports = %d, want 201: %s", base, rec.Code, rec.Body.String())
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, feed, ""},
		{http.MethodPost, base + "/reports", practice},
		{http.MethodPost, base + "/reports/1/confirm", `{"latitude":16.5424,"longitude":80.6386}`},
		{http.MethodGet, "/demo/api/v/short/reports", ""},
		{http.MethodGet, "/demo/nothing-here", ""},
		{http.MethodPost, base + "/reports/1/flag", `{}`},
	} {
		rec := do(c.method, c.path, c.body)
		if got := rec.Header().Values("Set-Cookie"); len(got) != 0 {
			t.Errorf("%s %s set a cookie: %v", c.method, c.path, got)
		}
	}
	if after := count(); after != before {
		t.Fatalf("requests under /demo changed the sessions count from %d to %d, want no change", before, after)
	}

	// Near misses: not Demo routes, so they go through the session middleware and
	// each writes exactly one new row. None answers 200 with Demo data.
	misses := []struct{ name, method, path string }{
		{"longer word", http.MethodGet, "/demonstration"},
		{"dot dot out of the prefix", http.MethodGet, "/demo/../api/reports"},
		{"other case", http.MethodGet, "/Demo/api"},
		{"double slash", http.MethodGet, "/demo//api/v/" + world.ID + "/reports"},
		{"HEAD", http.MethodHead, feed},
		{"PUT", http.MethodPut, base + "/reports"},
	}
	for _, m := range misses {
		t.Run(m.name, func(t *testing.T) {
			n := count()
			rec := do(m.method, m.path, "")
			if len(rec.Header().Values("Set-Cookie")) == 0 {
				t.Errorf("%s %s set no cookie, want a session cookie", m.method, m.path)
			}
			if got := count(); got != n+1 {
				t.Errorf("%s %s moved the sessions count from %d to %d, want %d", m.method, m.path, n, got, n+1)
			}
			if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), `"reports"`) {
				t.Errorf("%s %s answered 200 with Demo data", m.method, m.path)
			}
		})
	}
}
