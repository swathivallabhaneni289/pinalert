// visibility_rate_e2e_test.go pins WR-E02 of the Phase 9 code review
// (2026-10-30): turning the profile on may call the shared text screener, so the
// off to on change has a tight bucket of its own, and turning it off is never
// refused by that bucket. It runs on the real router and a real Postgres.
package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"pinalert/internal/api"
	sqlcgen "pinalert/internal/store/sqlc"
)

func TestVisibilityOnHasATightBucketAndOffIsNeverRefusedByIt(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.RateLimits.PeopleRead = e2eGenerousBucket
		d.RateLimits.PeopleAction = e2eGenerousBucket
		d.RateLimits.Visibility = api.BucketLimit{Every: time.Hour, Burst: 2}
	})
	a, _ := p907Person(t, srv, mailer, pool, "wre02-a@example.com", "wre02_a", false)
	b, _ := p907Person(t, srv, mailer, pool, "wre02-b@example.com", "wre02_b", false)

	patch := func(client *http.Client, on string) (int, string, http.Header) {
		return p907Do(t, client, http.MethodPatch, srv.URL+"/api/me/visibility", `{"profile_public":`+on+`}`)
	}
	for i := 0; i < 2; i++ {
		if status, body, _ := patch(a, "true"); status != http.StatusOK {
			t.Fatalf("turn on %d: status = %d, body %s", i+1, status, body)
		}
		if status, body, _ := patch(a, "false"); status != http.StatusOK {
			t.Fatalf("turn off %d: status = %d, body %s", i+1, status, body)
		}
	}
	status, body, header := patch(a, "true")
	if status != http.StatusTooManyRequests {
		t.Fatalf("the third turn on: status = %d, want 429, body %s", status, body)
	}
	if field, _ := p907ErrorOf(t, body); field != "rate_visibility" {
		t.Errorf("the 429 field = %q, want rate_visibility", field)
	}
	if header.Get("Retry-After") == "" {
		t.Error("the 429 has no Retry-After header")
	}
	// Turning off is never refused by the on bucket.
	if status, body, _ := patch(a, "false"); status != http.StatusOK {
		t.Errorf("turn off after the bucket ran out: status = %d, want 200, body %s", status, body)
	}
	// Another account has its own allowance.
	if status, body, _ := patch(b, "true"); status != http.StatusOK {
		t.Errorf("another account's turn on: status = %d, want 200, body %s", status, body)
	}
}
