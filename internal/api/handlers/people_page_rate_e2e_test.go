// people_page_rate_e2e_test.go pins WR-E01 of the Phase 9 code review
// (2026-10-30): the profile page GET /u/{username} takes a token from the same
// per account people read bucket as the JSON reads, and a refusal is a 429 page
// with Retry-After, never JSON. It runs on the real router and a real Postgres.
package handlers_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"pinalert/internal/api"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

func TestProfilePageUsesThePeopleReadBucket(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.RateLimits.PeopleRead = api.BucketLimit{Every: time.Hour, Burst: 2}
		d.RateLimits.PeopleAction = e2eGenerousBucket
	})
	a, _ := p907Person(t, srv, mailer, pool, "wre01-a@example.com", "wre_a", false)
	b, _ := p907Person(t, srv, mailer, pool, "wre01-b@example.com", "wre_b", false)
	testutil.SeedPublicReporter(t, pool, "wre01-rep@example.com", "wre_rep", "sess-wre-rep", time.Now().UTC())

	// One JSON read and one page read spend the two tokens of the one bucket.
	p907Get(t, a, srv, "/api/people/wre_rep", http.StatusOK)
	if status, body, _ := p907Do(t, a, http.MethodGet, srv.URL+"/u/wre_rep", ""); status != http.StatusOK {
		t.Fatalf("the page read: status = %d, want 200, body %s", status, body)
	}

	// The third read, by page, is refused as a page: HTML, Retry-After, no JSON.
	status, body, header := p907Do(t, a, http.MethodGet, srv.URL+"/u/wre_rep", "")
	if status != http.StatusTooManyRequests {
		t.Fatalf("the third read by page: status = %d, want 429, body %s", status, body)
	}
	if ct := header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("the 429 content type = %q, want text/html", ct)
	}
	if header.Get("Retry-After") == "" {
		t.Error("the 429 has no Retry-After header")
	}
	if strings.HasPrefix(strings.TrimSpace(body), "{") || strings.Contains(body, `"retry_after_seconds"`) {
		t.Errorf("the 429 body looks like JSON: %s", body)
	}
	if !strings.Contains(body, "<h1") || !strings.Contains(body, "You can try again in") {
		t.Errorf("the 429 page lacks its heading or its wait sentence: %s", body)
	}
	if header.Get("Cache-Control") != "no-store" {
		t.Errorf("the 429 Cache-Control = %q, want no-store", header.Get("Cache-Control"))
	}

	// A 404 probe pays too, and another account is not touched.
	if status, _, _ := p907Do(t, b, http.MethodGet, srv.URL+"/u/nobody_here", ""); status != http.StatusNotFound {
		t.Errorf("another account's probe: status = %d, want 404", status)
	}
}
