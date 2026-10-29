// username_e2e_test.go: Phase 7.1 plan 10, Task 1. The live username check
// (GET /api/username/check) and the username on PATCH /api/me/profile, driven
// through the real router over a real Postgres. Each refusal of a save has one
// fixed status (W12): 400 for taken, wrong characters, kept for official use
// and too soon to change; 422 only for the screening refusal; 429 when the
// per account save allowance is spent. Every package level identifier carries
// the p71u prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// p71uRejectedName is the one username the test screener refuses as hateful.
// It passes the shape and reserved rules, so only the screening can refuse it.
const p71uRejectedName = "rejectme"

// p71uScreener refuses p71uRejectedName and passes everything else. It counts
// its calls, so a test can prove the live check never reaches the provider.
type p71uScreener struct{ calls atomic.Int64 }

func (s *p71uScreener) Screen(_ context.Context, text string) (moderation.Result, error) {
	s.calls.Add(1)
	if text == p71uRejectedName {
		return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{"hate": 0.99}}, nil
	}
	return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{"hate": 0.01}}, nil
}

// p71uServer builds the e2e router with the test screener behind the account
// service. mutate, when non nil, runs after it and may change any other field.
func p71uServer(t *testing.T, scr *p71uScreener, mutate func(d *api.Deps)) (*httptest.Server, *recordingMailer) {
	t.Helper()
	srv, mailer, _ := newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		// Amended 2026-10-29 by Phase 9 plan 14 (D-02): the service built here takes the
		// ratings source the harness built (d.Ratings), as cmd/server/main.go does.
		d.Reports = service.NewReportService(q, service.WithScreener(scr), service.WithReportSignals(d.Ratings))
		if mutate != nil {
			mutate(d)
		}
	})
	return srv, mailer
}

// p71uSave PATCHes the username and returns the status, the body and the
// response headers.
func p71uSave(t *testing.T, client *http.Client, srv *httptest.Server, name string) (int, string, http.Header) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"username": name})
	if err != nil {
		t.Fatalf("marshalling the username body: %v", err)
	}
	resp := meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile", string(body))
	header := resp.Header
	return resp.StatusCode, meBody(t, resp), header
}

// p71uCheck GETs the live check and returns the response and its decoded body.
func p71uCheck(t *testing.T, client *http.Client, srv *httptest.Server, name string) (*http.Response, handlersUsernameCheck) {
	t.Helper()
	resp, err := client.Get(srv.URL + "/api/username/check?u=" + url.QueryEscape(name))
	if err != nil {
		t.Fatalf("GET /api/username/check: %v", err)
	}
	raw := meBody(t, resp)
	var out handlersUsernameCheck
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatalf("decoding the check answer %q: %v", raw, err)
		}
	}
	return resp, out
}

// handlersUsernameCheck mirrors handlers.UsernameCheckResponse.
type handlersUsernameCheck struct {
	State   string `json:"state"`
	Message string `json:"message"`
	Days    int    `json:"days"`
}

func TestUsernameCheckStates(t *testing.T) {
	scr := &p71uScreener{}
	srv, mailer := p71uServer(t, scr, nil)
	me := newVerifiedClient(t, srv, mailer, "check-me@example.com")
	other := newVerifiedClient(t, srv, mailer, "check-other@example.com")

	if status, body, _ := p71uSave(t, other, srv, "someone.else"); status != http.StatusOK {
		t.Fatalf("saving the other account's name: %d %s", status, body)
	}

	for _, c := range []struct {
		label, name, state, message string
	}{
		{"too short", "ab", service.UsernameStateInvalid, service.UsernameRuleMessage},
		{"wrong characters", "ra..vi", service.UsernameStateInvalid, service.UsernameRuleMessage},
		{"kept for official use", "p0lice", service.UsernameStateReserved, service.UsernameReservedMessage},
		{"free", "free.name", service.UsernameStateAvailable, service.UsernameAvailableMessage("free.name")},
		{"typed with capitals and an at sign", "@Free.Name", service.UsernameStateAvailable, service.UsernameAvailableMessage("free.name")},
		{"another account's", "someone.else", service.UsernameStateTaken, service.UsernameTakenMessage("someone.else")},
	} {
		resp, got := p71uCheck(t, me, srv, c.name)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d, want 200", c.label, resp.StatusCode)
		}
		if got.State != c.state || got.Message != c.message || got.Days != 0 {
			t.Errorf("%s: answer %+v, want state %q message %q", c.label, got, c.state, c.message)
		}
		if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", c.label, cc)
		}
	}

	// The account's own name is "yours". A first change is free; the second
	// inside 7 days reads wait, with the days left.
	if status, body, _ := p71uSave(t, me, srv, "my.first"); status != http.StatusOK {
		t.Fatalf("first save: %d %s", status, body)
	}
	if _, got := p71uCheck(t, me, srv, "my.first"); got.State != service.UsernameStateYours || got.Message != service.UsernameYoursMessage {
		t.Errorf("own name: %+v, want yours", got)
	}
	if status, body, _ := p71uSave(t, me, srv, "my.second"); status != http.StatusOK {
		t.Fatalf("first change: %d %s", status, body)
	}
	_, got := p71uCheck(t, me, srv, "my.third")
	if got.State != service.UsernameStateWait || got.Days != 7 || got.Message != service.UsernameWaitMessage(7) {
		t.Errorf("a rename inside 7 days: %+v, want wait with 7 days", got)
	}

	// The check never reaches the screening provider; only the three saves above
	// did (the other account's, and the account's two).
	if n := scr.calls.Load(); n != 3 {
		t.Errorf("the screener ran %d times, want 3 (one per save, none for the checks)", n)
	}

	// A signed out caller gets the gate's 401, not an answer.
	anon := newJarClient(t)
	if resp, _ := p71uCheck(t, anon, srv, "free.name"); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("signed out check: status %d, want 401", resp.StatusCode)
	}
}

func TestUsernameCheckLimited(t *testing.T) {
	srv, mailer := p71uServer(t, &p71uScreener{}, func(d *api.Deps) {
		d.AuthLimits.UsernameCheck = api.BucketLimit{Every: time.Hour, Burst: 2}
	})
	client := newVerifiedClient(t, srv, mailer, "check-limited@example.com")
	other := newVerifiedClient(t, srv, mailer, "check-limited-other@example.com")

	for i := 1; i <= 2; i++ {
		if resp, _ := p71uCheck(t, client, srv, "free.name"); resp.StatusCode != http.StatusOK {
			t.Fatalf("check %d: status %d, want 200", i, resp.StatusCode)
		}
	}
	resp, _ := p71uCheck(t, client, srv, "free.name")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("third check: status %d, want 429", resp.StatusCode)
	}
	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err != nil || secs < 1 {
		t.Errorf("Retry-After = %q, want a number of seconds", resp.Header.Get("Retry-After"))
	}
	// The allowance belongs to the account: another account is untouched.
	if resp, _ := p71uCheck(t, other, srv, "free.name"); resp.StatusCode != http.StatusOK {
		t.Errorf("another account's check: status %d, want 200", resp.StatusCode)
	}
}

func TestUsernameSaveThroughProfile(t *testing.T) {
	srv, mailer := p71uServer(t, &p71uScreener{}, nil)
	first := newVerifiedClient(t, srv, mailer, "save-first@example.com")
	second := newVerifiedClient(t, srv, mailer, "save-second@example.com")

	status, body, _ := p71uSave(t, first, srv, "Ravi.K")
	if status != http.StatusOK {
		t.Fatalf("saving Ravi.K: %d %s", status, body)
	}
	if _, got := p71uCheck(t, first, srv, "ravi.k"); got.State != service.UsernameStateYours {
		t.Errorf("the stored name is not ravi.k: %+v", got)
	}

	status, body, _ = p71uSave(t, second, srv, "Ravi.K")
	if status != http.StatusBadRequest {
		t.Fatalf("the same name from a second account: %d %s, want 400", status, body)
	}
	if field, message := p13alErrorField(t, body); field != "username" || message != "ravi.k is taken. Try another." {
		t.Errorf("taken answer = %q %q", field, message)
	}
}

func TestUsernameSaveRefused422(t *testing.T) {
	srv, mailer := p71uServer(t, &p71uScreener{}, nil)
	client := newVerifiedClient(t, srv, mailer, "save-refused@example.com")

	status, body, _ := p71uSave(t, client, srv, p71uRejectedName)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("a name the screening refuses: %d %s, want 422", status, body)
	}
	if field, message := p13alErrorField(t, body); field != "username" || message != "That username can't be used. Try another." {
		t.Errorf("refused answer = %q %q", field, message)
	}
	// Nothing was stored: a good name still saves as the first, free set.
	if status, body, _ := p71uSave(t, client, srv, "after.refusal"); status != http.StatusOK {
		t.Errorf("a good name after the refusal: %d %s, want 200", status, body)
	}
}

func TestUsernameSaveStatuses(t *testing.T) {
	srv, mailer := p71uServer(t, &p71uScreener{}, nil)
	holder := newVerifiedClient(t, srv, mailer, "status-holder@example.com")
	client := newVerifiedClient(t, srv, mailer, "status-client@example.com")
	if status, body, _ := p71uSave(t, holder, srv, "held.name"); status != http.StatusOK {
		t.Fatalf("seeding the held name: %d %s", status, body)
	}

	// Two saves of the account's own, so the third is inside the 7 day gap.
	for _, name := range []string{"status.one", "status.two"} {
		if status, body, _ := p71uSave(t, client, srv, name); status != http.StatusOK {
			t.Fatalf("saving %s: %d %s", name, status, body)
		}
	}

	// The gap check runs before the name checks, so the account inside its gap
	// answers the wait sentence for any name. The other rows come from an account
	// that has no username yet, whose first set is free.
	fresh := newVerifiedClient(t, srv, mailer, "status-fresh@example.com")
	for _, c := range []struct {
		label, name string
		who         *http.Client
		status      int
		message     string
	}{
		{"taken", "held.name", fresh, http.StatusBadRequest, service.UsernameTakenMessage("held.name")},
		{"wrong characters", "ra..vi", fresh, http.StatusBadRequest, service.UsernameRuleMessage},
		{"kept for official use", "p0lice", fresh, http.StatusBadRequest, service.UsernameReservedMessage},
		{"too soon to change", "status.three", client, http.StatusBadRequest, service.UsernameWaitMessage(7)},
		{"refused by the screening", p71uRejectedName, fresh, http.StatusUnprocessableEntity, service.UsernameRefusedMessage},
	} {
		status, body, _ := p71uSave(t, c.who, srv, c.name)
		if status != c.status {
			t.Errorf("%s: status %d, want %d (%s)", c.label, status, c.status, body)
			continue
		}
		if field, message := p13alErrorField(t, body); field != "username" || message != c.message {
			t.Errorf("%s: answer %q %q, want field username and %q", c.label, field, message, c.message)
		}
	}
}

func TestUsernameSaveLimited(t *testing.T) {
	// The default allowance: 5 saves, then one every 10 seconds.
	srv, mailer := p71uServer(t, &p71uScreener{}, func(d *api.Deps) {
		d.AuthLimits.UsernameSave = api.AuthLimitsDefault.UsernameSave
	})
	client := newVerifiedClient(t, srv, mailer, "save-limited@example.com")

	// The first two reach the service and succeed (the first set and the first
	// change are free). The next three reach it too and are refused as too soon
	// to change, which still spends a token.
	for i, name := range []string{"limit.one", "limit.two"} {
		if status, body, _ := p71uSave(t, client, srv, name); status != http.StatusOK {
			t.Fatalf("save %d (%s): %d %s, want 200", i+1, name, status, body)
		}
	}
	for i, name := range []string{"limit.three", "limit.four", "limit.five"} {
		status, body, _ := p71uSave(t, client, srv, name)
		if status != http.StatusBadRequest {
			t.Fatalf("save %d (%s): %d %s, want 400 too soon to change", i+3, name, status, body)
		}
		if field, message := p13alErrorField(t, body); field != "username" || message != service.UsernameWaitMessage(7) {
			t.Fatalf("save %d: answer %q %q, want the wait sentence", i+3, field, message)
		}
	}

	status, body, header := p71uSave(t, client, srv, "limit.six")
	if status != http.StatusTooManyRequests {
		t.Fatalf("sixth save: %d %s, want 429", status, body)
	}
	if field, _ := p13alErrorField(t, body); field != "username" {
		t.Errorf("429 field = %q, want username", field)
	}
	if secs, err := strconv.Atoi(header.Get("Retry-After")); err != nil || secs < 1 {
		t.Errorf("Retry-After = %q, want a number of seconds", header.Get("Retry-After"))
	}

	// A save that names no username does not touch the allowance.
	resp := meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile", `{"display_name":"Asha"}`)
	meWant(t, "a save with no username while the allowance is spent", resp, http.StatusOK)
}
