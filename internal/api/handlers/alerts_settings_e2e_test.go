// alerts_settings_e2e_test.go proves PATCH /api/me/alerts of plan 03.2-13
// (NOTIF-05, D-26) end to end over the real router and a real Postgres: the
// defaults, the closed distance list, the refusal to turn the alert on without a
// home area, the alert going off when the home area is removed, and that one
// account's settings never change another's. Every package level identifier
// here carries the p13al prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// p13alResponse is the answer of PATCH /api/me/alerts.
type p13alResponse struct {
	ReliableAlerts bool `json:"reliable_alerts"`
	AlertRadiusKm  int  `json:"alert_radius_km"`
	HasHomeArea    bool `json:"has_home_area"`
}

// p13alRow is the stored row, found = false when the account has none.
type p13alRow struct {
	found     bool
	on        bool
	radius    int
	enabledAt *time.Time
}

// p13alRead reads the stored alert settings of the account with this email.
func p13alRead(t *testing.T, pool *pgxpool.Pool, email string) p13alRow {
	t.Helper()
	var r p13alRow
	err := pool.QueryRow(context.Background(),
		`SELECT s.reliable_alerts_on, s.alert_radius_km, s.enabled_at
		 FROM account_alert_settings s JOIN accounts a ON a.id = s.account_id
		 WHERE a.email = $1`, email).Scan(&r.on, &r.radius, &r.enabledAt)
	if err == pgx.ErrNoRows {
		return p13alRow{}
	}
	if err != nil {
		t.Fatalf("reading the alert settings of %s: %v", email, err)
	}
	r.found = true
	return r
}

// p13alErrorField decodes the field of a 400 error answer.
func p13alErrorField(t *testing.T, body string) (field, message string) {
	t.Helper()
	var out struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decoding the error body %q: %v", body, err)
	}
	return out.Error.Field, out.Error.Message
}

func p13alDecode(t *testing.T, body string) p13alResponse {
	t.Helper()
	var out p13alResponse
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decoding the alert settings %q: %v", body, err)
	}
	return out
}

func TestAlertSettingsRoute(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	alerts := srv.URL + "/api/me/alerts"
	profile := srv.URL + "/api/me/profile"

	// No session: 401 and nothing stored.
	anon := newJarClient(t)
	meWant(t, "anonymous alerts", meSend(t, anon, http.MethodPatch, alerts, `{"reliable_alerts":false}`), http.StatusUnauthorized)

	const emailA, emailB = "p13al-a@example.com", "p13al-b@example.com"
	a := newJarClient(t)
	verifySession(t, a, srv, mailer, emailA)
	b := newJarClient(t)
	verifySession(t, b, srv, mailer, emailB)

	// The Settings page builds with the new fields for an account with no row.
	meWant(t, "GET /settings", meSend(t, a, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)

	// Refusals store nothing.
	cases := []struct {
		name, body, field string
	}{
		{"a distance outside the list", `{"alert_radius_km":3}`, "alert_radius_km"},
		{"a distance of zero", `{"alert_radius_km":0}`, "alert_radius_km"},
		{"an empty object", `{}`, "body"},
		{"an unknown field", `{"reliable_alerts":true,"account_id":2}`, "body"},
		{"a malformed body", `{"reliable_alerts":`, "body"},
		{"a wrong type", `{"reliable_alerts":"yes"}`, "body"},
		{"on without a home area", `{"reliable_alerts":true}`, "reliable_alerts"},
	}
	for _, c := range cases {
		body := meWant(t, c.name, meSend(t, a, http.MethodPatch, alerts, c.body), http.StatusBadRequest)
		if field, _ := p13alErrorField(t, body); field != c.field {
			t.Errorf("%s: error field = %q, want %q (body %s)", c.name, field, c.field, body)
		}
	}
	if r := p13alRead(t, pool, emailA); r.found {
		t.Fatalf("a refused request stored a row: %+v", r)
	}

	// Defaults: off, 5 km, no home area, as the route answers them.
	resp := meSend(t, a, http.MethodPatch, alerts, `{"reliable_alerts":false}`)
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	got := p13alDecode(t, meWant(t, "defaults", resp, http.StatusOK))
	if got.ReliableAlerts || got.AlertRadiusKm != 5 || got.HasHomeArea {
		t.Errorf("defaults = %+v, want off, 5 km, no home area", got)
	}

	// After a home area is saved the alert can go on, and that stamps enabled_at.
	meWant(t, "set home area", meSend(t, a, http.MethodPatch, profile,
		`{"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)
	got = p13alDecode(t, meWant(t, "turn on", meSend(t, a, http.MethodPatch, alerts, `{"reliable_alerts":true}`), http.StatusOK))
	if !got.ReliableAlerts || got.AlertRadiusKm != 5 || !got.HasHomeArea {
		t.Errorf("after turning on = %+v, want on, 5 km, home area", got)
	}
	on := p13alRead(t, pool, emailA)
	if !on.found || !on.on || on.radius != 5 || on.enabledAt == nil {
		t.Fatalf("stored after turning on = %+v, want on, 5 km, enabled_at stamped", on)
	}

	// A distance change keeps the switch on and the stamp as it was.
	got = p13alDecode(t, meWant(t, "radius 10", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":10}`), http.StatusOK))
	if !got.ReliableAlerts || got.AlertRadiusKm != 10 {
		t.Errorf("after the distance change = %+v, want on, 10 km", got)
	}
	kept := p13alRead(t, pool, emailA)
	if !kept.on || kept.radius != 10 || kept.enabledAt == nil || !kept.enabledAt.Equal(*on.enabledAt) {
		t.Errorf("stored after the distance change = %+v, want on, 10 km, enabled_at %v", kept, on.enabledAt)
	}

	// Another account's settings never changed.
	if r := p13alRead(t, pool, emailB); r.found {
		t.Errorf("another account has a row: %+v", r)
	}

	// Removing the home area turns the alert off on the server.
	meWant(t, "clear home area", meSend(t, a, http.MethodPatch, profile, `{"home_area":null}`), http.StatusOK)
	off := p13alRead(t, pool, emailA)
	if !off.found || off.on || off.enabledAt != nil {
		t.Errorf("stored after clearing the home area = %+v, want off with no enabled_at", off)
	}
	if off.radius != 10 {
		t.Errorf("the chosen distance was lost: %d, want 10", off.radius)
	}
	body := meWant(t, "turn on again", meSend(t, a, http.MethodPatch, alerts, `{"reliable_alerts":true}`), http.StatusBadRequest)
	if field, _ := p13alErrorField(t, body); field != "reliable_alerts" {
		t.Errorf("error field = %q, want reliable_alerts", field)
	}

	// B turns its own alert on without disturbing A.
	meWant(t, "B home", meSend(t, b, http.MethodPatch, profile,
		`{"home_area":{"name":"Whitefield","lat":12.9698,"lon":77.7500}}`), http.StatusOK)
	meWant(t, "B on", meSend(t, b, http.MethodPatch, alerts, `{"reliable_alerts":true,"alert_radius_km":1}`), http.StatusOK)
	if r := p13alRead(t, pool, emailB); !r.on || r.radius != 1 {
		t.Errorf("B stored = %+v, want on, 1 km", r)
	}
	if r := p13alRead(t, pool, emailA); r.on || r.radius != 10 {
		t.Errorf("A changed when B saved: %+v", r)
	}

	// The Settings page still builds for both.
	meWant(t, "GET /settings after", meSend(t, a, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
	meWant(t, "GET /settings B", meSend(t, b, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
}
