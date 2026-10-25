// push_settings_e2e_test.go proves the Critical and Changes phone switches of
// plan 03.3-12 (PWA-03, PWA-06, D-08, D-21, decision 12) end to end over the real
// router and a real Postgres: they ride PATCH /api/me/alerts next to 3.2's two
// keys. Every package level identifier here carries the p12pk prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// p12pkResponse is the answer of PATCH /api/me/alerts with the two new keys.
type p12pkResponse struct {
	ReliableAlerts bool `json:"reliable_alerts"`
	AlertRadiusKm  int  `json:"alert_radius_km"`
	HasHomeArea    bool `json:"has_home_area"`
	CriticalPush   bool `json:"critical_push"`
	ChangesPush    bool `json:"changes_push"`
}

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

func p12pkDecode(t *testing.T, body string) p12pkResponse {
	t.Helper()
	var out p12pkResponse
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decoding the alert settings %q: %v", body, err)
	}
	return out
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

	// Defaults: a radius only PATCH on a new account answers Critical on and
	// Changes off (decision 12), and does not write the push columns away from
	// their defaults.
	got := p12pkDecode(t, meWant(t, "defaults", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":2}`), http.StatusOK))
	if !got.CriticalPush || got.ChangesPush || got.ReliableAlerts || got.AlertRadiusKm != 2 || got.HasHomeArea {
		t.Errorf("defaults = %+v, want Critical on, Changes off, Reliable off, 2 km, no home area", got)
	}
	if r := p12pkRead(t, pool, emailA); !r.found || !r.critical || r.changes {
		t.Errorf("stored after a radius only edit = %+v, want a row with Critical on, Changes off", r)
	}

	// Strict decoding and the body rules still hold with the new keys.
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

	// Changes needs no home area.
	got = p12pkDecode(t, meWant(t, "changes on", meSend(t, a, http.MethodPatch, alerts, `{"changes_push":true}`), http.StatusOK))
	if !got.ChangesPush || !got.CriticalPush || got.HasHomeArea {
		t.Errorf("after Changes on = %+v, want Changes on, Critical still on, no home area", got)
	}
	if r := p12pkRead(t, pool, emailA); !r.changes || !r.critical {
		t.Errorf("stored after Changes on = %+v", r)
	}

	// Turning Critical off needs no home area either.
	got = p12pkDecode(t, meWant(t, "critical off", meSend(t, a, http.MethodPatch, alerts, `{"critical_push":false}`), http.StatusOK))
	if got.CriticalPush || !got.ChangesPush {
		t.Errorf("after Critical off = %+v, want Critical off, Changes on", got)
	}

	// Turning Critical on without a home area is refused with field critical_push,
	// and nothing changes.
	body := meWant(t, "critical on without area", meSend(t, a, http.MethodPatch, alerts, `{"critical_push":true}`), http.StatusBadRequest)
	if field, msg := p13alErrorField(t, body); field != "critical_push" || msg != "Add a home area first." {
		t.Errorf("error = %q, %q; want critical_push, Add a home area first.", field, msg)
	}
	if r := p12pkRead(t, pool, emailA); r.critical {
		t.Errorf("a refused request changed Critical: %+v", r)
	}

	// With a home area, Critical goes off and on and is stored.
	meWant(t, "set home area", meSend(t, a, http.MethodPatch, profile,
		`{"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)
	got = p12pkDecode(t, meWant(t, "critical on", meSend(t, a, http.MethodPatch, alerts, `{"critical_push":true}`), http.StatusOK))
	if !got.CriticalPush || !got.HasHomeArea {
		t.Errorf("after Critical on = %+v, want Critical on with a home area", got)
	}
	if r := p12pkRead(t, pool, emailA); !r.critical {
		t.Errorf("stored after Critical on = %+v", r)
	}
	got = p12pkDecode(t, meWant(t, "critical off again", meSend(t, a, http.MethodPatch, alerts, `{"critical_push":false}`), http.StatusOK))
	if got.CriticalPush {
		t.Errorf("after Critical off = %+v", got)
	}
	if r := p12pkRead(t, pool, emailA); r.critical {
		t.Errorf("stored after Critical off = %+v", r)
	}

	// Clearing the home area keeps the stored Critical value and turns 3.2's
	// Reliable switch off. Turn Critical on and Reliable on first.
	meWant(t, "both on", meSend(t, a, http.MethodPatch, alerts, `{"critical_push":true,"reliable_alerts":true}`), http.StatusOK)
	if r := p12pkRead(t, pool, emailA); !r.critical || !r.reliable || !r.changes {
		t.Fatalf("stored before clearing = %+v, want Critical, Reliable and Changes on", r)
	}
	meWant(t, "clear home area", meSend(t, a, http.MethodPatch, profile, `{"home_area":null}`), http.StatusOK)
	if r := p12pkRead(t, pool, emailA); !r.critical || r.reliable || !r.changes {
		t.Errorf("stored after clearing the area = %+v, want Critical kept on, Reliable off, Changes kept on", r)
	}
	// The page and the settings answer both still build, and Critical reads on
	// as stored while no home area exists.
	meWant(t, "GET /settings after clearing", meSend(t, a, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
	got = p12pkDecode(t, meWant(t, "radius after clearing", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":5}`), http.StatusOK))
	if !got.CriticalPush || got.HasHomeArea || got.ReliableAlerts {
		t.Errorf("after clearing = %+v, want Critical on as stored, no home area, Reliable off", got)
	}
	// Setting an area again brings Critical back as it was, without any edit.
	meWant(t, "set home area again", meSend(t, a, http.MethodPatch, profile,
		`{"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)
	got = p12pkDecode(t, meWant(t, "radius after setting", meSend(t, a, http.MethodPatch, alerts, `{"alert_radius_km":5}`), http.StatusOK))
	if !got.CriticalPush || !got.HasHomeArea {
		t.Errorf("after setting the area again = %+v, want Critical on and a home area", got)
	}

	// B never changed through all of A's edits, and has no row at all.
	if r := p12pkRead(t, pool, emailB); r.found {
		t.Errorf("another account has a row: %+v", r)
	}
	// B edits its own switches without disturbing A.
	meWant(t, "B changes on", meSend(t, b, http.MethodPatch, alerts, `{"changes_push":true,"critical_push":false}`), http.StatusOK)
	if r := p12pkRead(t, pool, emailB); !r.found || r.critical || !r.changes {
		t.Errorf("B stored = %+v, want Critical off, Changes on", r)
	}
	if r := p12pkRead(t, pool, emailA); !r.critical || !r.changes {
		t.Errorf("A changed when B saved: %+v", r)
	}

	// The Settings page builds for both.
	meWant(t, "GET /settings A", meSend(t, a, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
	meWant(t, "GET /settings B", meSend(t, b, http.MethodGet, srv.URL+"/settings", ""), http.StatusOK)
}
