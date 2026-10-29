// people_body_strict_e2e_test.go pins IN-E04 of the Phase 9 code review
// (2026-10-30): the Phase 9 JSON bodies take exactly one value. Anything after
// it is a 400 on the field the route names, like the push routes. It runs on the
// real router and a real Postgres.
package handlers_test

import (
	"net/http"
	"testing"

	"pinalert/internal/api"
	sqlcgen "pinalert/internal/store/sqlc"
)

func TestPhase9BodiesRefuseTrailingData(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.RateLimits.PeopleRead = e2eGenerousBucket
		d.RateLimits.PeopleAction = e2eGenerousBucket
		d.RateLimits.Rate = e2eGenerousBucket
	})
	a, _ := p907Person(t, srv, mailer, pool, "in-e04-a@example.com", "in_e04_a", false)

	cases := []struct {
		name, method, path, body, field string
	}{
		{"notify with a second value", http.MethodPost, "/api/people/in_e04_a/notify", `{"on":true}{"x":1}`, "on"},
		{"notify with trailing text", http.MethodPost, "/api/people/in_e04_a/notify", `{"on":true} garbage`, "on"},
		{"rating with trailing text", http.MethodPost, "/api/people/in_e04_a/rating", `{"stars":4} garbage`, "stars"},
		{"visibility with a second value", http.MethodPatch, "/api/me/visibility", `{"profile_public":false}{}`, "profile_public"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body, _ := p907Do(t, a, c.method, srv.URL+c.path, c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body %s", status, body)
			}
			if field, _ := p907ErrorOf(t, body); field != c.field {
				t.Errorf("error field = %q, want %q", field, c.field)
			}
		})
	}
	// A clean single value still passes.
	if status, body, _ := p907Do(t, a, http.MethodPatch, srv.URL+"/api/me/visibility", `{"profile_public":false}`); status != http.StatusOK {
		t.Errorf("a single value: status = %d, want 200, body %s", status, body)
	}
}
