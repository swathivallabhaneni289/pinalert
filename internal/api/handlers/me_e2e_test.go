// me_e2e_test.go proves the three /api/me routes of plan 03.4-01 end to end
// over the real router and a real Postgres: consent, the optional name and home
// area, and the first-time flow flag. They run only with DATABASE_URL set (the
// test database helper skips otherwise). Every package level identifier carries
// the me prefix.
package handlers_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	"pinalert/internal/testutil"
)

// meSend sends one request with a JSON body and returns the response.
func meSend(t *testing.T, client *http.Client, method, url, body string) *http.Response {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatalf("building %s %s: %v", method, url, err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// meBody reads and closes a response body.
func meBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(raw)
}

// meWant requires a status and returns the body.
func meWant(t *testing.T, label string, resp *http.Response, want int) string {
	t.Helper()
	body := meBody(t, resp)
	if resp.StatusCode != want {
		t.Fatalf("%s: status = %d, want %d, body %s", label, resp.StatusCode, want, body)
	}
	return body
}

type meRow struct {
	name, homeName, homeCell, terms *string
	done                            bool
}

// meRead reads the account's stored profile columns.
func meRead(t *testing.T, pool *pgxpool.Pool, email string) meRow {
	t.Helper()
	var r meRow
	err := pool.QueryRow(context.Background(),
		`SELECT display_name, home_area_name, home_area_cell, terms_version, first_run_done_at IS NOT NULL
		 FROM accounts WHERE email = $1`, email).Scan(&r.name, &r.homeName, &r.homeCell, &r.terms, &r.done)
	if err != nil {
		t.Fatalf("reading the profile of %s: %v", email, err)
	}
	return r
}

// meRemoveHomeArea puts the account into the "no home area" state directly in
// the database, the way an account from before step 4 was required is, and
// switches its Reliable reporter alert off as the old clear did. Added
// 2026-10-29 by Phase 7.1 plan 06 (D-15): PATCH /api/me/profile no longer clears
// the home area, so tests that only need the state use this instead. The
// account is found by its address.
func meRemoveHomeArea(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	testutil.MustExec(t, pool,
		`UPDATE account_alert_settings SET reliable_alerts_on = false, enabled_at = NULL
		 WHERE account_id = (SELECT id FROM accounts WHERE email = $1)`, email)
	testutil.MustExec(t, pool,
		`UPDATE accounts SET home_area_name = NULL, home_area_cell = NULL WHERE email = $1`, email)
}

func TestMeConsent(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)

	// An unverified caller gets 401 and nothing is stored.
	anon := newJarClient(t)
	resp := meSend(t, anon, http.MethodPost, srv.URL+"/api/me/consent", "")
	body := meWant(t, "unverified consent", resp, http.StatusUnauthorized)
	if strings.Contains(body, "terms") {
		t.Fatalf("the refusal must carry no data: %s", body)
	}

	client := newJarClient(t)
	verifySession(t, client, srv, mailer, "consent@example.com")
	other := newJarClient(t)
	verifySession(t, other, srv, mailer, "consent-other@example.com")

	// An unknown field is refused, a body naming an account changes nothing.
	resp = meSend(t, client, http.MethodPost, srv.URL+"/api/me/consent", `{"account_id": 1}`)
	meWant(t, "unknown field", resp, http.StatusBadRequest)
	if r := meRead(t, pool, "consent@example.com"); r.terms != nil {
		t.Fatalf("a refused request stored a version: %v", *r.terms)
	}

	// No body and an empty object both agree.
	resp = meSend(t, client, http.MethodPost, srv.URL+"/api/me/consent", "")
	meWant(t, "empty body consent", resp, http.StatusNoContent)
	resp = meSend(t, client, http.MethodPost, srv.URL+"/api/me/consent", "{}")
	meWant(t, "empty object consent", resp, http.StatusNoContent)

	if r := meRead(t, pool, "consent@example.com"); r.terms == nil || *r.terms != service.CurrentTermsVersion {
		t.Fatalf("terms version not stored: %+v", r)
	}
	// Only the caller's own account changed.
	if r := meRead(t, pool, "consent-other@example.com"); r.terms != nil {
		t.Fatalf("another account was changed: %v", *r.terms)
	}
}

func TestMeProfile(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	url := srv.URL + "/api/me/profile"

	anon := newJarClient(t)
	resp := meSend(t, anon, http.MethodPatch, url, `{"display_name":"Asha"}`)
	meWant(t, "unverified profile", resp, http.StatusUnauthorized)

	client := newJarClient(t)
	verifySession(t, client, srv, mailer, "profile@example.com")
	other := newJarClient(t)
	verifySession(t, other, srv, mailer, "profile-other@example.com")

	// Unknown fields are refused at both levels.
	resp = meSend(t, client, http.MethodPatch, url, `{"display_name":"Asha","account_id":2}`)
	meWant(t, "unknown top level field", resp, http.StatusBadRequest)
	resp = meSend(t, client, http.MethodPatch, url, `{"home_area":{"name":"X","lat":12.9,"lon":77.5,"extra":1}}`)
	meWant(t, "unknown home area field", resp, http.StatusBadRequest)
	resp = meSend(t, client, http.MethodPatch, url, `{"home_area":{"name":"X","lat":12.9}}`)
	meWant(t, "missing longitude", resp, http.StatusBadRequest)

	// A name alone.
	resp = meSend(t, client, http.MethodPatch, url, `{"display_name":"  Asha  "}`)
	body := meWant(t, "name only", resp, http.StatusOK)
	if !strings.Contains(body, `"display_name":"Asha"`) || !strings.Contains(body, `"home_area_name":null`) {
		t.Fatalf("name only answer = %s", body)
	}

	// A home area: the answer holds the name, never a coordinate.
	resp = meSend(t, client, http.MethodPatch, url, `{"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`)
	body = meWant(t, "home area", resp, http.StatusOK)
	if !strings.Contains(body, `"home_area_name":"Indiranagar, Bengaluru"`) || !strings.Contains(body, `"display_name":"Asha"`) {
		t.Fatalf("home area answer = %s", body)
	}
	if strings.Contains(body, "12.97") || strings.Contains(body, "77.64") || strings.Contains(body, "lat") {
		t.Fatalf("the answer must carry no coordinates: %s", body)
	}
	r := meRead(t, pool, "profile@example.com")
	if r.homeCell == nil || len(*r.homeCell) != 5 {
		t.Fatalf("home cell not stored as 5 characters: %+v", r)
	}

	// Validation errors name the field.
	resp = meSend(t, client, http.MethodPatch, url, `{"display_name":"`+strings.Repeat("a", 61)+`"}`)
	meWant(t, "long name", resp, http.StatusBadRequest)
	resp = meSend(t, client, http.MethodPatch, url, `{"home_area":{"name":"X","lat":95,"lon":77.5}}`)
	meWant(t, "latitude out of range", resp, http.StatusBadRequest)

	// Amended 2026-10-29 by Phase 7.1 plan 06 (D-15, D-06): null used to remove
	// the home area and an empty name used to clear the name. Both are now
	// refused with 400 and a plain sentence, and what is stored stays.
	resp = meSend(t, client, http.MethodPatch, url, `{"home_area":null}`)
	body = meWant(t, "clear home area", resp, http.StatusBadRequest)
	if field, message := p13alErrorField(t, body); field != "home_area" || message != "Choose a home area." {
		t.Fatalf("clear home area answer = %s, want field home_area and Choose a home area.", body)
	}
	r = meRead(t, pool, "profile@example.com")
	if r.homeName == nil || r.homeCell == nil {
		t.Fatalf("a refused clear must keep the home area: %+v", r)
	}

	resp = meSend(t, client, http.MethodPatch, url, `{"display_name":""}`)
	body = meWant(t, "clear name", resp, http.StatusBadRequest)
	if field, message := p13alErrorField(t, body); field != "display_name" || message != "Your name can't be empty." {
		t.Fatalf("clear name answer = %s, want field display_name and Your name can't be empty.", body)
	}
	if r = meRead(t, pool, "profile@example.com"); r.name == nil || *r.name != "Asha" {
		t.Fatalf("a refused empty name must keep the name: %+v", r)
	}

	// The other account was never touched.
	if o := meRead(t, pool, "profile-other@example.com"); o.name != nil || o.homeName != nil {
		t.Fatalf("another account was changed: %+v", o)
	}
}

func TestMeFirstRunDone(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	url := srv.URL + "/api/me/first-run-done"

	anon := newJarClient(t)
	resp := meSend(t, anon, http.MethodPost, url, "")
	meWant(t, "unverified done", resp, http.StatusUnauthorized)

	client := newJarClient(t)
	verifySession(t, client, srv, mailer, "done@example.com")

	resp = meSend(t, client, http.MethodPost, url, `{"account_id":1}`)
	meWant(t, "unknown field", resp, http.StatusBadRequest)

	// Before consent: 409 with field terms, nothing stored.
	resp = meSend(t, client, http.MethodPost, url, "")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("done before consent: status = %d, want 409", resp.StatusCode)
	}
	field, message := decodeErrorResponse(t, resp)
	if field != "terms" || message != service.TermsNotAgreedMessage {
		t.Fatalf("409 error = (%q, %q)", field, message)
	}
	if r := meRead(t, pool, "done@example.com"); r.done {
		t.Fatal("first run was marked done without consent")
	}

	resp = meSend(t, client, http.MethodPost, srv.URL+"/api/me/consent", "")
	meWant(t, "consent", resp, http.StatusNoContent)
	// Amended 2026-10-29 by Phase 7.1 plan 10 (D-06): done after consent alone is
	// now refused with 409 field profile (TestFirstRunDoneNeedsProfile covers the
	// refusal); the profile is filled first. Every other assertion is unchanged.
	fillProfile(t, client, srv, "done.user")
	resp = meSend(t, client, http.MethodPost, url, "")
	meWant(t, "done after consent", resp, http.StatusNoContent)
	resp = meSend(t, client, http.MethodPost, url, "{}")
	meWant(t, "done again", resp, http.StatusNoContent)
	if r := meRead(t, pool, "done@example.com"); !r.done {
		t.Fatal("first run was not marked done")
	}
}

// TestFirstRunDoneNeedsProfile pins D-06 and SC4: the end of the flow cannot be
// reached without a username, a name and a home area. With the terms agreed but
// no profile, first-run-done answers 409 with the field profile and the plan 04
// sentence, and stores nothing. A partly filled profile is refused the same
// way, whichever of the three is missing. Once all three are saved it succeeds.
func TestFirstRunDoneNeedsProfile(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	url := srv.URL + "/api/me/first-run-done"
	client := newJarClient(t)
	const email = "done-needs-profile@example.com"
	verifySession(t, client, srv, mailer, email)
	meWant(t, "consent", meSend(t, client, http.MethodPost, srv.URL+"/api/me/consent", ""), http.StatusNoContent)

	refused := func(label string) {
		t.Helper()
		resp := meSend(t, client, http.MethodPost, url, "")
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("%s: status = %d, want 409", label, resp.StatusCode)
		}
		field, message := decodeErrorResponse(t, resp)
		if field != "profile" || message != "Add your username, name and home area." {
			t.Fatalf("%s: 409 error = (%q, %q), want field profile and the plan 04 sentence", label, field, message)
		}
		if r := meRead(t, pool, email); r.done {
			t.Fatalf("%s: first run was marked done", label)
		}
	}

	refused("nothing saved")

	meWant(t, "username only", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile", `{"username":"needs.profile"}`), http.StatusOK)
	refused("username only")

	meWant(t, "name added", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile", `{"display_name":"Asha"}`), http.StatusOK)
	refused("username and name")

	meWant(t, "home area added", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)
	meWant(t, "done with all three", meSend(t, client, http.MethodPost, url, ""), http.StatusNoContent)
	if r := meRead(t, pool, email); !r.done {
		t.Fatal("first run was not marked done with all three saved")
	}

	// The terms refusal still comes first: an account that agreed to nothing is
	// told to agree, not to fill in a profile.
	other := newJarClient(t)
	verifySession(t, other, srv, mailer, "done-needs-terms@example.com")
	fillProfile(t, other, srv, "needs.terms")
	resp := meSend(t, other, http.MethodPost, url, "")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("a full profile without consent: status = %d, want 409", resp.StatusCode)
	}
	if field, _ := decodeErrorResponse(t, resp); field != "terms" {
		t.Fatalf("a full profile without consent: field = %q, want terms", field)
	}
}
