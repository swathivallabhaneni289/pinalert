package api

// share_routes_test.go proves the route level properties of the public status
// page GET /r/{token} (Phase 8 plan 04; D-05, D-06, A-P8-8, research Pitfall 2):
// opening it writes no sessions row and sets no cookie, whoever opens it, and
// the paths near it never answer with a page. The page itself, its states and
// its Open Graph tags are proved in internal/api/handlers (public_status_e2e_test).

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	"pinalert/internal/testutil"
)

// seedSharedReport stores one clear report of its own account and gives it a
// fresh share token, which it returns. label keeps the account email and the
// session id unique inside a test.
func seedSharedReport(t *testing.T, pool *pgxpool.Pool, label string) string {
	t.Helper()
	session := "share-route-" + label
	testutil.SeedReporterAccount(t, pool, session+"@example.com", session)
	now := time.Now()
	id := testutil.InsertStoredReport(t, pool, testutil.StoredReport{
		SessionID: session,
		Category:  "flood",
		Severity:  "low",
		// A marker the page must never show.
		Description: "share route private words " + label,
		Latitude:    16.5062,
		Longitude:   80.6480,
		CreatedAt:   now.Add(-10 * time.Minute),
		ExpiresAt:   now.Add(2 * time.Hour),
	})
	token, err := service.NewShareToken()
	if err != nil {
		t.Fatalf("making a share token: %v", err)
	}
	testutil.MustExec(t, pool, `UPDATE reports SET share_token = $1 WHERE id = $2`, token, id)
	return token
}

// TestSharePageSetsNoCookie copies TestPWAAssetsSetNoCookie for /r/{token}: a
// cookieless GET with no user agent, with WhatsApp's crawler agent and with a
// browser agent answers 200 with no Set-Cookie header and leaves the sessions
// count unchanged (T-08-15). It counts a shared table, so it never runs in
// parallel.
func TestSharePageSetsNoCookie(t *testing.T) {
	srv, pool, _ := newGateTestServer(t)
	ctx := context.Background()
	token := seedSharedReport(t, pool, "cookie")

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

	agents := []struct{ name, ua string }{
		{"no user agent", ""},
		{"WhatsApp crawler", "WhatsApp/2.23.20.0 A"},
		{"browser", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"},
	}
	for _, a := range agents {
		t.Run(a.name, func(t *testing.T) {
			before := count()
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/r/"+token, nil)
			if err != nil {
				t.Fatalf("building the request: %v", err)
			}
			// Go sends its own agent when none is set; an empty header removes it.
			req.Header.Set("User-Agent", a.ua)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("GET /r/<token>: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /r/<token> = %d, want 200", resp.StatusCode)
			}
			if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
				t.Errorf("GET /r/<token> set a cookie: %v", got)
			}
			if after := count(); after != before {
				t.Errorf("GET /r/<token> moved the sessions count from %d to %d, want no change", before, after)
			}
		})
	}
}

// TestSharePageNearMisses proves what the /r prefix does not open. A POST under
// it is a 405 with no cookie and no sessions row; the encoded spelling of the
// path is never the page; a HEAD is accepted as observed (Phase 5's matcher
// covers GET and POST only, so the session middleware runs for a HEAD exactly as
// it does for a HEAD to any other app path) and fails only if it returns the
// page body (T-08-85, accepted).
func TestSharePageNearMisses(t *testing.T) {
	srv, pool, _ := newGateTestServer(t)
	ctx := context.Background()
	token := seedSharedReport(t, pool, "miss")

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

	t.Run("POST is a 405 without a cookie", func(t *testing.T) {
		before := count()
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/r/"+token, strings.NewReader(""))
		if err != nil {
			t.Fatalf("building the request: %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST /r/<token>: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST /r/<token> = %d, want 405", resp.StatusCode)
		}
		if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
			t.Errorf("POST /r/<token> set a cookie: %v", got)
		}
		if after := count(); after != before {
			t.Errorf("POST /r/<token> moved the sessions count from %d to %d, want no change", before, after)
		}
	})

	t.Run("encoded spelling is never the page", func(t *testing.T) {
		// "r" written as %72: the URL carries an encoded form, so the session
		// skip does not apply and the router must not serve the page either.
		path := "/%72/" + token
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("GET %s = 200, want never the page", path)
		}
	})

	t.Run("HEAD is observed and never returns the page body", func(t *testing.T) {
		before := count()
		req, err := http.NewRequest(http.MethodHead, srv.URL+"/r/"+token, nil)
		if err != nil {
			t.Fatalf("building the request: %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("HEAD /r/<token>: %v", err)
		}
		defer resp.Body.Close()
		t.Logf("HEAD /r/<token>: status %d, Set-Cookie %d, sessions %d -> %d",
			resp.StatusCode, len(resp.Header.Values("Set-Cookie")), before, count())
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading the HEAD answer: %v", err)
		}
		if strings.Contains(string(body), "<html") || strings.Contains(string(body), service.ShareFixedLine) {
			t.Errorf("HEAD /r/<token> returned the page body: %s", body)
		}
	})
}
