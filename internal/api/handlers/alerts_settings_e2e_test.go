// alerts_settings_e2e_test.go proves PATCH /api/me/alerts of plan 03.2-13
// (NOTIF-05, D-26) end to end over the real router and a real Postgres: the
// defaults, the closed distance list, the old switch keys accepted and ignored
// (quick 261103-jal), and that one account's settings never change another's.
// Every package level identifier here carries the p13al prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/testutil"
)

// p13alResponse is the answer of PATCH /api/me/alerts.
type p13alResponse struct {
	AlertRadiusKm int  `json:"alert_radius_km"`
	HasHomeArea   bool `json:"has_home_area"`
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

// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D7,
// D8, C4): PATCH /api/me/alerts takes the distance alone. The four switch keys of
// earlier versions are accepted and ignored, the answer holds alert_radius_km and
// has_home_area only, and the stored columns are read back by SQL to prove a
// request that carries the old keys changes none of them.
func TestAlertSettingsRoute(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	alerts := srv.URL + "/api/me/alerts"
	profile := srv.URL + "/api/me/profile"

	// No session: 401 and nothing stored.
	anon := newJarClient(t)
	meWant(t, "anonymous alerts", meSend(t, anon, http.MethodPatch, alerts, `{"alert_radius_km":5}`), http.StatusUnauthorized)

	const emailA, emailB = "p13al-a@example.com", "p13al-b@example.com"
	a := newJarClient(t)
	verifySession(t, a, srv, mailer, emailA)
	b := newJarClient(t)
	verifySession(t, b, srv, mailer, emailB)

	// The Settings page builds for an account with no row.
	meWant(t, "GET /settings", meSend(t, a, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)

	// Refusals store nothing.
	cases := []struct {
		name, body, field string
	}{
		{"a distance outside the list", `{"alert_radius_km":3}`, "alert_radius_km"},
		{"a distance of zero", `{"alert_radius_km":0}`, "alert_radius_km"},
		{"an empty object", `{}`, "body"},
		{"only null keys", `{"critical_push":null}`, "body"},
		{"an unknown field", `{"alert_radius_km":5,"account_id":2}`, "body"},
		{"a malformed body", `{"alert_radius_km":`, "body"},
		{"a wrong type", `{"alert_radius_km":"yes"}`, "body"},
		{"a wrong type on an ignored key", `{"reliable_alerts":"yes"}`, "body"},
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

	// A body that names only the ignored keys answers 200 with the stored
	// settings (the defaults here) and stores nothing.
	for _, legacy := range []string{`{"critical_push":false}`, `{"reliable_alerts":true}`, `{"changes_push":true,"official_push":false}`} {
		resp := meSend(t, a, http.MethodPatch, alerts, legacy)
		if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", legacy, cc)
		}
		raw := meWant(t, "ignored keys only "+legacy, resp, http.StatusOK)
		got := p13alDecode(t, raw)
		if got.AlertRadiusKm != 5 || got.HasHomeArea {
			t.Errorf("%s: answer = %+v, want 5 km and no home area", legacy, got)
		}
		for _, banned := range []string{"reliable_alerts", "critical_push", "changes_push", "official_push"} {
			if strings.Contains(raw, banned) {
				t.Errorf("%s: the answer names %q: %s", legacy, banned, raw)
			}
		}
	}
	if r := p13alRead(t, pool, emailA); r.found {
		t.Fatalf("a request with only ignored keys stored a row: %+v", r)
	}

	// The distance is saved and the answer holds exactly the two keys.
	meWant(t, "set home area", meSend(t, a, http.MethodPatch, profile,
		`{"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)
	raw := meWant(t, "radius 2", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":2}`), http.StatusOK)
	if strings.TrimSpace(raw) != `{"alert_radius_km":2,"has_home_area":true}` {
		t.Errorf("answer = %s, want exactly {\"alert_radius_km\":2,\"has_home_area\":true}", raw)
	}
	if r := p13alRead(t, pool, emailA); !r.found || r.radius != 2 {
		t.Fatalf("stored after the distance change = %+v, want 2 km", r)
	}

	// A body with the distance and every ignored key saves the distance and
	// leaves the four stored columns and the stamp as they were. The row is
	// seeded by SQL with the old values, a stamp included.
	stamp := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Second)
	testutil.MustExec(t, pool,
		`UPDATE account_alert_settings SET reliable_alerts_on = true, enabled_at = $2,
		        critical_push_on = true, changes_push_on = false, official_push_on = true
		 WHERE account_id = (SELECT id FROM accounts WHERE email = $1)`, emailA, stamp)
	got := p13alDecode(t, meWant(t, "distance with ignored keys", meSend(t, a, http.MethodPatch, alerts,
		`{"alert_radius_km":10,"reliable_alerts":false,"critical_push":false,"changes_push":true,"official_push":false}`), http.StatusOK))
	if got.AlertRadiusKm != 10 || !got.HasHomeArea {
		t.Errorf("answer = %+v, want 10 km and a home area", got)
	}
	var on, critical, changes, official bool
	var radius int
	var enabled *time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT s.reliable_alerts_on, s.alert_radius_km, s.enabled_at, s.critical_push_on, s.changes_push_on, s.official_push_on
		 FROM account_alert_settings s JOIN accounts a ON a.id = s.account_id WHERE a.email = $1`, emailA,
	).Scan(&on, &radius, &enabled, &critical, &changes, &official); err != nil {
		t.Fatalf("reading the row back: %v", err)
	}
	if !on || radius != 10 || !critical || changes || !official {
		t.Errorf("stored = on %v, radius %d, critical %v, changes %v, official %v; want the seeded flags and 10 km", on, radius, critical, changes, official)
	}
	if enabled == nil || !enabled.Equal(stamp) {
		t.Errorf("enabled_at = %v, want the seeded stamp %v kept", enabled, stamp)
	}

	// Another account's settings never changed.
	if r := p13alRead(t, pool, emailB); r.found {
		t.Errorf("another account has a row: %+v", r)
	}

	// B saves its own distance without disturbing A.
	meWant(t, "B home", meSend(t, b, http.MethodPatch, profile,
		`{"home_area":{"name":"Whitefield","lat":12.9698,"lon":77.7500}}`), http.StatusOK)
	meWant(t, "B radius", meSend(t, b, http.MethodPatch, alerts, `{"alert_radius_km":1}`), http.StatusOK)
	if r := p13alRead(t, pool, emailB); r.radius != 1 || r.on {
		t.Errorf("B stored = %+v, want 1 km and the switch off", r)
	}
	if r := p13alRead(t, pool, emailA); r.radius != 10 {
		t.Errorf("A changed when B saved: %+v", r)
	}

	// Amended 2026-10-29 by Phase 7.1 plan 06 (D-15): removing the home area
	// through the route is refused.
	refused := meWant(t, "clear home area", meSend(t, a, http.MethodPatch, profile, `{"home_area":null}`), http.StatusBadRequest)
	if field, _ := p13alErrorField(t, refused); field != "home_area" {
		t.Errorf("clear home area error field = %q, want home_area", field)
	}

	// The Settings page still builds for both.
	meWant(t, "GET /settings after", meSend(t, a, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
	meWant(t, "GET /settings B", meSend(t, b, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
}
