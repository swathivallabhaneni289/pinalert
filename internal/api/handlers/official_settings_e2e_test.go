// official_settings_e2e_test.go proved the switch "Severe official alerts for my
// area" of plan 05-14 (D-11) end to end over the real router and a real
// Postgres. Every package level identifier here carries the p14os prefix.
//
// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D2,
// D7, D8): the switch left the product. official_push is accepted and ignored by
// PATCH /api/me/alerts, the stored column keeps its default (on) and any value
// stored before, nothing in the answer names it, and the Settings page never
// holds the home cell (T-05-77). The test proves exactly that, reading the
// stored column back by SQL.
package handlers_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"
)

// p14osStored reads the stored official column; found is false when the account
// has no settings row.
func p14osStored(t *testing.T, pool *pgxpool.Pool, email string) (found, official bool) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT s.official_push_on
		 FROM account_alert_settings s JOIN accounts a ON a.id = s.account_id
		 WHERE a.email = $1`, email).Scan(&official)
	if err == pgx.ErrNoRows {
		return false, false
	}
	if err != nil {
		t.Fatalf("reading the official switch of %s: %v", email, err)
	}
	return true, official
}

func TestOfficialPushSettingRoute(t *testing.T) {
	srv, mailer, pool := newPushServer(t)
	alerts := srv.URL + "/api/me/alerts"
	profile := srv.URL + "/api/me/profile"

	// No session: 401.
	anon := newJarClient(t)
	meWant(t, "anonymous", meSend(t, anon, http.MethodPatch, alerts, `{"official_push":false}`), http.StatusUnauthorized)

	const emailA, emailB = "p14os-a@example.com", "p14os-b@example.com"
	a := newJarClient(t)
	verifySession(t, a, srv, mailer, emailA)
	b := newJarClient(t)
	verifySession(t, b, srv, mailer, emailB)

	// A body with only the old official key answers 200 with the stored
	// settings, names no old key, and stores nothing.
	for _, legacy := range []string{`{"official_push":false}`, `{"official_push":true}`} {
		raw := meWant(t, "ignored "+legacy, meSend(t, a, http.MethodPatch, alerts, legacy), http.StatusOK)
		if got := p13alDecode(t, raw); got.AlertRadiusKm != 5 || got.HasHomeArea {
			t.Errorf("%s: answer = %+v, want 5 km and no home area", legacy, got)
		}
		if strings.Contains(raw, "official_push") {
			t.Errorf("%s: the answer names the old key: %s", legacy, raw)
		}
	}
	if found, _ := p14osStored(t, pool, emailA); found {
		t.Fatal("a request with only the old official key stored a row")
	}

	// Default: a distance only edit on a new account leaves official on (D-11).
	meWant(t, "defaults", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":2}`), http.StatusOK)
	if found, on := p14osStored(t, pool, emailA); !found || !on {
		t.Errorf("stored after a distance only edit = %v, %v; want a row with official on", found, on)
	}

	// Strict decoding and the body rules still hold.
	for _, c := range []struct{ name, body, field string }{
		{"an unknown field", `{"official_push":true,"severe_official":true}`, "body"},
		{"a wrong type", `{"official_push":"yes"}`, "body"},
		{"an empty object", `{}`, "body"},
	} {
		body := meWant(t, c.name, meSend(t, a, http.MethodPatch, alerts, c.body), http.StatusBadRequest)
		if field, _ := p13alErrorField(t, body); field != c.field {
			t.Errorf("%s: error field = %q, want %q (body %s)", c.name, field, c.field, body)
		}
	}

	// The old key sent beside the distance changes the stored column neither
	// way, with a home area or without one.
	testutilOfficialOff := func() {
		t.Helper()
		if _, err := pool.Exec(context.Background(),
			`UPDATE account_alert_settings SET official_push_on = false
			 WHERE account_id = (SELECT id FROM accounts WHERE email = $1)`, emailA); err != nil {
			t.Fatalf("storing official off by SQL: %v", err)
		}
	}
	testutilOfficialOff()
	meWant(t, "official on is ignored", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":5,"official_push":true}`), http.StatusOK)
	if _, on := p14osStored(t, pool, emailA); on {
		t.Error("the ignored official_push true turned the stored value on")
	}
	meWant(t, "set home area", meSend(t, a, http.MethodPatch, profile,
		`{"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)
	meWant(t, "official on is ignored with a home area", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":5,"official_push":true}`), http.StatusOK)
	if _, on := p14osStored(t, pool, emailA); on {
		t.Error("the ignored official_push true turned the stored value on with a home area")
	}

	// The Settings page builds and never holds the home cell (T-05-77).
	cell := geohash.EncodeWithPrecision(12.9784, 77.6408, 5)
	page := meWant(t, "GET /settings", meSend(t, a, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
	if strings.Contains(page, cell) {
		t.Errorf("the Settings page holds the home area cell %q", cell)
	}

	// B never changed through all of A's edits.
	if found, _ := p14osStored(t, pool, emailB); found {
		t.Error("another account has a settings row")
	}
	meWant(t, "B radius", meSend(t, b, http.MethodPatch, alerts, `{"alert_radius_km":1}`), http.StatusOK)
	if found, on := p14osStored(t, pool, emailB); !found || !on {
		t.Errorf("B stored = %v, %v; want a row with official on by default", found, on)
	}
	meWant(t, "GET /settings B", meSend(t, b, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
}
