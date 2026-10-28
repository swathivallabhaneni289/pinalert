// official_settings_e2e_test.go proves the switch "Severe official alerts for my
// area" of plan 05-14 (D-11) end to end over the real router and a real Postgres:
// official_push rides PATCH /api/me/alerts next to the other four keys, is on by
// default, needs a home area only to be turned on, and never puts the home cell
// into the Settings page. Every package level identifier here carries the p14os
// prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"
)

// p14osResponse is the answer of PATCH /api/me/alerts with all five keys.
type p14osResponse struct {
	ReliableAlerts bool `json:"reliable_alerts"`
	AlertRadiusKm  int  `json:"alert_radius_km"`
	HasHomeArea    bool `json:"has_home_area"`
	CriticalPush   bool `json:"critical_push"`
	ChangesPush    bool `json:"changes_push"`
	OfficialPush   bool `json:"official_push"`
}

func p14osDecode(t *testing.T, body string) p14osResponse {
	t.Helper()
	var out p14osResponse
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decoding the alert settings %q: %v", body, err)
	}
	return out
}

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

	// Default: a radius only edit on a new account answers official on (D-11).
	got := p14osDecode(t, meWant(t, "defaults", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":2}`), http.StatusOK))
	if !got.OfficialPush || !got.CriticalPush || got.ChangesPush || got.ReliableAlerts || got.HasHomeArea {
		t.Errorf("defaults = %+v, want official on, Critical on, Changes off, Reliable off, no home area", got)
	}

	// Strict decoding and the body rules still hold with the new key.
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

	// Turning it off needs no home area, and the other four values stay.
	got = p14osDecode(t, meWant(t, "official off", meSend(t, a, http.MethodPatch, alerts, `{"official_push":false}`), http.StatusOK))
	if got.OfficialPush || !got.CriticalPush || got.ChangesPush || got.ReliableAlerts || got.AlertRadiusKm != 2 {
		t.Errorf("after official off = %+v, want only official changed", got)
	}
	if found, on := p14osStored(t, pool, emailA); !found || on {
		t.Errorf("stored after official off = %v, %v; want a row with official off", found, on)
	}

	// Turning it on without a home area is refused with field official_push and
	// nothing changes.
	body := meWant(t, "official on without area", meSend(t, a, http.MethodPatch, alerts, `{"official_push":true}`), http.StatusBadRequest)
	if field, msg := p13alErrorField(t, body); field != "official_push" || msg != "Add a home area first." {
		t.Errorf("error = %q, %q; want official_push, Add a home area first.", field, msg)
	}
	if _, on := p14osStored(t, pool, emailA); on {
		t.Error("a refused request turned the official switch on")
	}

	// With a home area it goes on and is stored.
	meWant(t, "set home area", meSend(t, a, http.MethodPatch, profile,
		`{"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)
	got = p14osDecode(t, meWant(t, "official on", meSend(t, a, http.MethodPatch, alerts, `{"official_push":true}`), http.StatusOK))
	if !got.OfficialPush || !got.HasHomeArea {
		t.Errorf("after official on = %+v, want official on with a home area", got)
	}
	if _, on := p14osStored(t, pool, emailA); !on {
		t.Error("the official switch was not stored on")
	}

	// The Settings page builds and never holds the home cell (T-05-77).
	cell := geohash.EncodeWithPrecision(12.9784, 77.6408, 5)
	page := meWant(t, "GET /settings", meSend(t, a, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
	if strings.Contains(page, cell) {
		t.Errorf("the Settings page holds the home area cell %q", cell)
	}

	// Clearing the home area keeps the stored value as it was.
	meWant(t, "official off again", meSend(t, a, http.MethodPatch, alerts, `{"official_push":false}`), http.StatusOK)
	meWant(t, "clear home area", meSend(t, a, http.MethodPatch, profile, `{"home_area":null}`), http.StatusOK)
	if _, on := p14osStored(t, pool, emailA); on {
		t.Error("clearing the home area changed the stored official value")
	}
	got = p14osDecode(t, meWant(t, "radius after clearing", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":5}`), http.StatusOK))
	if got.OfficialPush || got.HasHomeArea {
		t.Errorf("after clearing = %+v, want official off as stored, no home area", got)
	}

	// B never changed through all of A's edits, and has no official row of its own
	// until it saves one; A's saves never touch B.
	if found, _ := p14osStored(t, pool, emailB); found {
		t.Error("another account has a settings row")
	}
	got = p14osDecode(t, meWant(t, "B radius", meSend(t, b, http.MethodPatch, alerts, `{"alert_radius_km":1}`), http.StatusOK))
	if !got.OfficialPush {
		t.Errorf("B = %+v, want official on by default", got)
	}
	meWant(t, "GET /settings B", meSend(t, b, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
}
