package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/api/handlers"
	"pinalert/internal/official"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// These tests drive POST /internal/official/refresh through the real router, in
// process, with a fake official service (plan 05-12, D-15, T-05-64, T-05-65,
// T-05-66). The endpoint's own rules (token, methods, summary words) are covered
// without a database in internal/api/handlers/official_refresh_test.go.

// The real official service must be usable as the refresher, or the route would
// answer 503 in production and no test with a fake would notice.
var _ handlers.OfficialRefresher = (*official.Service)(nil)

const routeRefreshToken = "route-test-refresh-token-0123456789abcdef"

// refreshStub is an official service double that counts RefreshAll calls.
type refreshStub struct {
	calls atomic.Int32
}

func (s *refreshStub) Alerts(context.Context, int64, *float64, *float64) (official.View, error) {
	return official.View{}, errors.New("not used by these tests")
}
func (s *refreshStub) Kick() {}
func (s *refreshStub) RefreshAll(context.Context) []official.Result {
	s.calls.Add(1)
	return []official.Result{{Feed: official.FeedSachet, ItemsSeen: 4, Stored: 1}}
}

// refreshRouter builds the real router over the test database with the stub as
// the official service and the given token.
func refreshRouter(t *testing.T, stub *refreshStub, token string) (http.Handler, func() int) {
	t.Helper()
	pool := testutil.NewTestDB(t)
	mgr, err := session.NewManager([]byte("refresh-route-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}
	queries := sqlcgen.New(pool)
	router := NewRouter(Deps{
		Session:              mgr,
		Sessions:             queries,
		Reports:              service.NewReportService(queries),
		Template:             tmpl,
		RateLimits:           gateGenerousRateLimits,
		Official:             stub,
		OfficialRefreshToken: token,
	})
	count := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions`).Scan(&n); err != nil {
			t.Fatalf("counting sessions: %v", err)
		}
		return n
	}
	return router, count
}

func refreshPost(router http.Handler, path, remote, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.RemoteAddr = remote + ":4321"
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// TestRefreshEndpointNoSession: a cookieless POST with the right token is a real
// 200, sets no cookie and writes no sessions row, so the cron's thousands of
// calls a month do not grow the table. A trailing slash is a near miss: it is not
// the route, and it gets a session like any other request.
func TestRefreshEndpointNoSession(t *testing.T) {
	stub := &refreshStub{}
	router, count := refreshRouter(t, stub, routeRefreshToken)

	before := count()
	rec := refreshPost(router, "/internal/official/refresh", "192.0.2.20", "Bearer "+routeRefreshToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST with the token = %d, want 200, body %s", rec.Code, rec.Body.String())
	}
	if c := rec.Result().Cookies(); len(c) != 0 {
		t.Errorf("the cron call set %d cookies, want none", len(c))
	}
	if got := rec.Header().Get("Set-Cookie"); got != "" {
		t.Errorf("Set-Cookie = %q, want none", got)
	}
	if got := stub.calls.Load(); got != 1 {
		t.Errorf("the call ran %d refreshes, want 1", got)
	}
	if after := count(); after != before {
		t.Errorf("sessions went from %d to %d: the cron call must write no row", before, after)
	}

	// A call that fails the token is also sessionless: it is the same route.
	if rec := refreshPost(router, "/internal/official/refresh", "192.0.2.20", "Bearer wrong"); rec.Code != http.StatusUnauthorized {
		t.Errorf("POST with a wrong token = %d, want 401", rec.Code)
	}
	if after := count(); after != before {
		t.Errorf("a refused call changed sessions from %d to %d", before, after)
	}

	// The trailing slash is not the route.
	rec = refreshPost(router, "/internal/official/refresh/", "192.0.2.21", "Bearer "+routeRefreshToken)
	if rec.Code == http.StatusOK {
		t.Errorf("POST with a trailing slash = 200: only the exact path is the route")
	}
	if after := count(); after != before+1 {
		t.Errorf("the near miss left sessions at %d, want %d (a session like any other request)", after, before+1)
	}
	if got := stub.calls.Load(); got != 1 {
		t.Errorf("the near miss ran a refresh: %d calls, want 1", got)
	}
}

// TestRefreshEndpointRateLimit: past the per IP budget (burst 6) a POST gets the
// standard 429 and runs no refresh, whatever its token; another address is
// unaffected; and the 404 the route gives a GET never turns into a 429.
func TestRefreshEndpointRateLimit(t *testing.T) {
	stub := &refreshStub{}
	router, _ := refreshRouter(t, stub, routeRefreshToken)
	good := "Bearer " + routeRefreshToken
	burst := OfficialRefreshLimit.Burst
	if burst != 6 {
		t.Fatalf("OfficialRefreshLimit.Burst = %d, want 6", burst)
	}

	for i := 0; i < burst; i++ {
		if rec := refreshPost(router, "/internal/official/refresh", "192.0.2.30", good); rec.Code != http.StatusOK {
			t.Fatalf("call %d = %d, want 200", i+1, rec.Code)
		}
	}
	rec := refreshPost(router, "/internal/official/refresh", "192.0.2.30", good)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("call past the budget = %d, want 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Too many requests") {
		t.Errorf("429 body = %q, want the standard rate limit message", rec.Body.String())
	}
	if got := stub.calls.Load(); got != int32(burst) {
		t.Errorf("%d refreshes ran, want %d: a refused call must run none", got, burst)
	}

	// A GET from the same address is still the plain 404.
	req := httptest.NewRequest(http.MethodGet, "/internal/official/refresh", nil)
	req.RemoteAddr = "192.0.2.30:4321"
	get := httptest.NewRecorder()
	router.ServeHTTP(get, req)
	if get.Code != http.StatusNotFound {
		t.Errorf("GET from a limited address = %d, want 404", get.Code)
	}

	// Another address still gets through.
	if rec := refreshPost(router, "/internal/official/refresh", "192.0.2.31", good); rec.Code != http.StatusOK {
		t.Errorf("another address = %d, want 200", rec.Code)
	}

	// Wrong tokens spend the budget too, so the token cannot be guessed at speed.
	for i := 0; i < burst; i++ {
		if rec := refreshPost(router, "/internal/official/refresh", "192.0.2.32", "Bearer wrong"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong token %d = %d, want 401", i+1, rec.Code)
		}
	}
	if rec := refreshPost(router, "/internal/official/refresh", "192.0.2.32", good); rec.Code != http.StatusTooManyRequests {
		t.Errorf("right token after six wrong ones = %d, want 429", rec.Code)
	}
}

// TestRefreshEndpointUnsetStaysHidden: with no token the route is a 404 for every
// POST however many arrive; it never turns into a 429 that would show it exists.
func TestRefreshEndpointUnsetStaysHidden(t *testing.T) {
	stub := &refreshStub{}
	router, _ := refreshRouter(t, stub, "")
	for i := 0; i < 20; i++ {
		rec := refreshPost(router, "/internal/official/refresh", "192.0.2.40", "Bearer "+routeRefreshToken)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("call %d with no token configured = %d, want 404", i+1, rec.Code)
		}
	}
	if got := stub.calls.Load(); got != 0 {
		t.Errorf("%d refreshes ran with no token configured, want 0", got)
	}
}

// TestRefreshEndpointIsRegistered: the route is a registered POST route of the
// router even when no token is set, so the skip set and the route table agree.
func TestRefreshEndpointIsRegistered(t *testing.T) {
	mgr, err := session.NewManager([]byte("refresh-route-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}
	router := NewRouter(Deps{Session: mgr, Template: tmpl})
	found := false
	err = chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if route == "/internal/official/refresh" {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the router: %v", err)
	}
	if !found {
		t.Error("/internal/official/refresh is not a registered route")
	}
	for path := range sessionSkipSet().post {
		ok := false
		_ = chi.Walk(router, func(_, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			if route == path {
				ok = true
			}
			return nil
		})
		if !ok {
			t.Errorf("%s is in the POST skip set but is not a registered route", path)
		}
	}
}
