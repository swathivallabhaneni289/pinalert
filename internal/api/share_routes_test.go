package api

// share_routes_test.go proves the route level properties of the public status
// page GET /r/{token} (Phase 8 plan 04; D-05, D-06, A-P8-8, research Pitfall 2):
// opening it writes no sessions row and sets no cookie, whoever opens it, and
// the paths near it never answer with a page. The page itself, its states and
// its Open Graph tags are proved in internal/api/handlers (public_status_e2e_test).

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestShareTokenNeverInAccessLog proves review WR-S01 (2026-10-29): the token in
// /r/{token} is a capability, so the access log line of the public page names
// the route, never the token. A shared link opened by a reader or by WhatsApp's
// crawler leaves "/r/{token}" in the host's log store, not the link itself.
func TestShareTokenNeverInAccessLog(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	srv, pool, _ := newGateTestServerWith(t, func(d *Deps, _ *sqlcgen.Queries) {
		d.RequestLog = log.New(&shareLogWriter{mu: &mu, w: &buf}, "", 0)
	})
	token := seedSharedReport(t, pool, "accesslog")

	// Amended 2026-10-29 by review fix round 2 (SK-04): a path that resolves
	// under /r/ but is not written as /r/ (a doubled slash, an encoded r, a dot
	// segment, an encoded slash) still carries the token, so it is redacted too.
	paths := []string{
		"/r/" + token,
		"/r/" + token + "/extra",
		"//r/" + token,
		"/%72/" + token,
		"/./r/" + token,
		"/%2Fr/" + token,
	}
	for _, path := range paths {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	// The logger writes after the handler returns; wait for both lines.
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		logged := buf.String()
		mu.Unlock()
		if strings.Count(logged, "\n") >= len(paths) || time.Now().After(deadline) {
			if strings.Contains(logged, token) {
				t.Errorf("the access log holds the share token: %q", logged)
			}
			if strings.Count(logged, "GET /r/{token} ") < len(paths) {
				t.Errorf("the access log does not name the route /r/{token} for every request: %q", logged)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestFaviconSetsNoCookie proves the server half of review WR-T02 (2026-10-29):
// a browser that opens the public page with no cookie may still ask for
// /favicon.ico, and that request answers a plain 404 with no Set-Cookie header
// and no new sessions row, so a reader with no account never gets a Pinalert
// cookie from a shared link.
func TestFaviconSetsNoCookie(t *testing.T) {
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
	before := count()
	resp, err := http.Get(srv.URL + "/favicon.ico")
	if err != nil {
		t.Fatalf("GET /favicon.ico: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /favicon.ico = %d, want 404", resp.StatusCode)
	}
	if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
		t.Errorf("GET /favicon.ico set a cookie: %v", got)
	}
	if after := count(); after != before {
		t.Errorf("GET /favicon.ico moved the sessions count from %d to %d, want no change", before, after)
	}
}

// shareLogWriter serialises writes to a shared buffer, so the test can read it
// while the server goroutines log.
type shareLogWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (l *shareLogWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

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
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (the encoded spelling is never the page)", path, resp.StatusCode)
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
