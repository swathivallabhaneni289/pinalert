// visibility_already_on_e2e_test.go pins SK-I02 of the Phase 9 skeptic round
// (2026-10-30): the Visibility bucket takes a token for every request that asks
// for on, including one that finds the profile already on, because the handler
// charges before it knows the stored state. docs/MODERATION.md says exactly
// that. It runs on the real router and a real Postgres.
package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"pinalert/internal/api"
	sqlcgen "pinalert/internal/store/sqlc"
)

func TestVisibilityOnForAnAlreadyOnProfileStillSpendsAToken(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.RateLimits.PeopleRead = e2eGenerousBucket
		d.RateLimits.PeopleAction = e2eGenerousBucket
		d.RateLimits.Visibility = api.BucketLimit{Every: time.Hour, Burst: 2}
	})
	a, _ := p907Person(t, srv, mailer, pool, "ski02-a@example.com", "sk_i02_a", false)

	patch := func(on string) (int, string, http.Header) {
		return p907Do(t, a, http.MethodPatch, srv.URL+"/api/me/visibility", `{"profile_public":`+on+`}`)
	}
	if status, body, _ := patch("true"); status != http.StatusOK {
		t.Fatalf("the first on: status = %d, body %s", status, body)
	}
	if status, body, _ := patch("true"); status != http.StatusOK {
		t.Fatalf("on again while already on: status = %d, body %s", status, body)
	}
	status, body, _ := patch("true")
	if status != http.StatusTooManyRequests {
		t.Fatalf("the third on while already on: status = %d, want 429 (the no-op spent a token), body %s", status, body)
	}
}
