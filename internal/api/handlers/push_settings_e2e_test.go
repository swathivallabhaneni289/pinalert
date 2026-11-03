// push_settings_e2e_test.go proved the Critical and Changes phone switches of
// plan 03.3-12 (PWA-03, PWA-06, D-08, D-21, decision 12) end to end over the real
// router and a real Postgres. Every package level identifier here carries the
// p12pk prefix.
//
// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D2,
// D3, D7, D8): the two switches left the API. critical_push and changes_push are
// accepted and ignored by PATCH /api/me/alerts, the stored columns keep their
// defaults and their values, and nothing in the answer names them. The test now
// proves exactly that, reading the stored columns back by SQL.
package handlers_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// p12pkRow is the stored push columns, found false when the account has no
// settings row.
type p12pkRow struct {
	found    bool
	critical bool
	changes  bool
	reliable bool
}

func p12pkRead(t *testing.T, pool *pgxpool.Pool, email string) p12pkRow {
	t.Helper()
	var r p12pkRow
	err := pool.QueryRow(context.Background(),
		`SELECT s.critical_push_on, s.changes_push_on, s.reliable_alerts_on
		 FROM account_alert_settings s JOIN accounts a ON a.id = s.account_id
		 WHERE a.email = $1`, email).Scan(&r.critical, &r.changes, &r.reliable)
	if err == pgx.ErrNoRows {
		return p12pkRow{}
	}
	if err != nil {
		t.Fatalf("reading the push switches of %s: %v", email, err)
	}
	r.found = true
	return r
}

func TestPushKindSettingsRoute(t *testing.T) {
	srv, mailer, pool := newPushServer(t)
	alerts := srv.URL + "/api/me/alerts"
	profile := srv.URL + "/api/me/profile"

	// No session: 401.
	anon := newJarClient(t)
	meWant(t, "anonymous", meSend(t, anon, http.MethodPatch, alerts, `{"critical_push":false}`), http.StatusUnauthorized)

	const emailA, emailB = "p12pk-a@example.com", "p12pk-b@example.com"
	a := newJarClient(t)
	verifySession(t, a, srv, mailer, emailA)
	b := newJarClient(t)
	verifySession(t, b, srv, mailer, emailB)

	// The Settings page builds with the push key configured and no row.
	meWant(t, "GET /settings", meSend(t, a, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)

	// A body with only the old phone switch keys answers 200 with the stored
	// settings, names neither key, and stores nothing.
	for _, legacy := range []string{`{"critical_push":false}`, `{"changes_push":true}`, `{"critical_push":true,"changes_push":true}`} {
		raw := meWant(t, "ignored "+legacy, meSend(t, a, http.MethodPatch, alerts, legacy), http.StatusOK)
		got := p13alDecode(t, raw)
		if got.AlertRadiusKm != 5 || got.HasHomeArea {
			t.Errorf("%s: answer = %+v, want 5 km and no home area", legacy, got)
		}
		if strings.Contains(raw, "critical_push") || strings.Contains(raw, "changes_push") {
			t.Errorf("%s: the answer names an old key: %s", legacy, raw)
		}
	}
	if r := p12pkRead(t, pool, emailA); r.found {
		t.Fatalf("a request with only old keys stored a row: %+v", r)
	}

	// A distance only edit writes the row and leaves the push columns at their
	// defaults (decision 12: Critical on, Changes off).
	meWant(t, "radius", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":2}`), http.StatusOK)
	if r := p12pkRead(t, pool, emailA); !r.found || !r.critical || r.changes || r.reliable {
		t.Errorf("stored after a distance only edit = %+v, want a row with Critical on, Changes off, Reliable off", r)
	}

	// Strict decoding and the body rules still hold.
	for _, c := range []struct{ name, body, field string }{
		{"an unknown field", `{"critical_push":true,"severe_official":true}`, "body"},
		{"a wrong type", `{"changes_push":"yes"}`, "body"},
		{"an empty object", `{}`, "body"},
	} {
		body := meWant(t, c.name, meSend(t, a, http.MethodPatch, alerts, c.body), http.StatusBadRequest)
		if field, _ := p13alErrorField(t, body); field != c.field {
			t.Errorf("%s: error field = %q, want %q (body %s)", c.name, field, c.field, body)
		}
	}

	// The old keys sent beside the distance change none of the stored switches,
	// with or without a home area.
	for _, withHome := range []bool{false, true} {
		if withHome {
			meWant(t, "set home area", meSend(t, a, http.MethodPatch, profile,
				`{"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)
		}
		got := p13alDecode(t, meWant(t, "ignored beside the distance", meSend(t, a, http.MethodPatch, alerts,
			`{"alert_radius_km":5,"critical_push":false,"changes_push":true,"reliable_alerts":true}`), http.StatusOK))
		if got.AlertRadiusKm != 5 || got.HasHomeArea != withHome {
			t.Errorf("answer = %+v, want 5 km and home area %v", got, withHome)
		}
		if r := p12pkRead(t, pool, emailA); !r.critical || r.changes || r.reliable {
			t.Errorf("stored after the ignored keys (home %v) = %+v, want Critical on, Changes off, Reliable off as before", withHome, r)
		}
	}

	// B never changed through all of A's edits, and has no row at all.
	if r := p12pkRead(t, pool, emailB); r.found {
		t.Errorf("another account has a row: %+v", r)
	}

	// The Settings page builds for both.
	meWant(t, "GET /settings A", meSend(t, a, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
	meWant(t, "GET /settings B", meSend(t, b, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
}
