package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/auth"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// newSendWaitServer is the e2e server with the real 45 second wait between two
// code emails, on a clock the test moves, so no test sleeps.
func newSendWaitServer(t *testing.T) (*httptest.Server, *recordingMailer, *ceilingClock) {
	t.Helper()
	rec := &recordingMailer{}
	clk := &ceilingClock{t: time.Now()}
	srv, _, _ := newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		d.Credentials = service.NewCredentialService(q, rec,
			service.WithHasher(auth.NewHasher(auth.TestParams())),
			service.WithCodeKey(bytes.Repeat([]byte("c"), 32)),
			service.WithThrottleKey(bytes.Repeat([]byte("t"), 32)),
			service.WithCredentialClock(clk.Now),
		)
	})
	return srv, rec, clk
}

// expectSendWait checks one refused answer: 429 on the field email, the sentence
// with the seconds left, and Retry-After and retry_after_seconds equal to them.
func expectSendWait(t *testing.T, label string, a authAnswer, seconds int, sentence string) {
	t.Helper()
	if a.status != http.StatusTooManyRequests {
		t.Fatalf("%s: status = %d, want 429: %s", label, a.status, a.body)
	}
	var got struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
		RetryAfterSeconds int `json:"retry_after_seconds"`
	}
	if err := json.Unmarshal(a.body, &got); err != nil {
		t.Fatalf("%s: body %q is not JSON: %v", label, a.body, err)
	}
	if got.Error.Field != "email" || got.Error.Message != sentence {
		t.Errorf("%s: error = %+v, want field email and %q", label, got.Error, sentence)
	}
	if got.RetryAfterSeconds != seconds {
		t.Errorf("%s: retry_after_seconds = %d, want %d", label, got.RetryAfterSeconds, seconds)
	}
	if h := retryAfterOf(t, a); h != seconds {
		t.Errorf("%s: Retry-After = %d, want %d", label, h, seconds)
	}
}

// TestSendWaitSaysTheSecondsLeft is quick 261116-isd: a second code email to one
// address inside the 45 second wait is refused with the real time left, on
// sign up, Resend and Forgot password, and the answer is the same for an
// address with an account and one without.
func TestSendWaitSaysTheSecondsLeft(t *testing.T) {
	const twenty = "A code was just sent to this email. Try again in 20 seconds."

	t.Run("sign up twice", func(t *testing.T) {
		srv, _, clk := newSendWaitServer(t)
		c := newLoginClient(t, srv)
		body := map[string]string{"email": "wait.signup@example.com", "password": e2eTestPassword}
		if a := doAuth(t, c, srv, "/api/auth/sign-up", body); a.status != http.StatusOK {
			t.Fatalf("first sign up status = %d: %s", a.status, a.body)
		}
		clk.Advance(25 * time.Second)
		expectSendWait(t, "second sign up", doAuth(t, c, srv, "/api/auth/sign-up", body), 20, twenty)
		// The words move with the clock and the wait ends at 45 seconds.
		clk.Advance(10 * time.Second)
		expectSendWait(t, "third sign up", doAuth(t, c, srv, "/api/auth/sign-up", body), 10,
			"A code was just sent to this email. Try again in 10 seconds.")
		clk.Advance(11 * time.Second)
		if a := doAuth(t, c, srv, "/api/auth/sign-up", body); a.status != http.StatusOK {
			t.Fatalf("sign up after the wait status = %d: %s", a.status, a.body)
		}
	})

	t.Run("resend", func(t *testing.T) {
		srv, _, clk := newSendWaitServer(t)
		c := newLoginClient(t, srv)
		if a := doAuth(t, c, srv, "/api/auth/sign-up", map[string]string{"email": "wait.resend@example.com", "password": e2eTestPassword}); a.status != http.StatusOK {
			t.Fatalf("sign up status = %d: %s", a.status, a.body)
		}
		clk.Advance(25 * time.Second)
		expectSendWait(t, "resend", doAuth(t, c, srv, "/api/auth/code/resend", map[string]string{}), 20, twenty)
	})

	t.Run("forgot password", func(t *testing.T) {
		srv, _, clk := newSendWaitServer(t)
		c := newLoginClient(t, srv)
		body := map[string]string{"email": "wait.reset@example.com"}
		if a := doAuth(t, c, srv, "/api/auth/reset", body); a.status != http.StatusOK {
			t.Fatalf("reset status = %d: %s", a.status, a.body)
		}
		clk.Advance(25 * time.Second)
		expectSendWait(t, "second reset", doAuth(t, c, srv, "/api/auth/reset", body), 20, twenty)
	})

	t.Run("the same answer for an address with an account and one without", func(t *testing.T) {
		srv, mail, clk := newSendWaitServer(t)
		const known = "wait.known@example.com"
		const unknown = "wait.unknown@example.com"
		createAccountViaCode(t, srv, mail, known)
		clk.Advance(46 * time.Second)

		// Create account: the known address is the "you already have an
		// account" branch, the unknown one a plain code. Both claim first.
		kc, uc := newLoginClient(t, srv), newLoginClient(t, srv)
		if a := doAuth(t, kc, srv, "/api/auth/sign-up", map[string]string{"email": known, "password": e2eTestPassword}); a.status != http.StatusOK {
			t.Fatalf("known sign up status = %d: %s", a.status, a.body)
		}
		if a := doAuth(t, uc, srv, "/api/auth/sign-up", map[string]string{"email": unknown, "password": e2eTestPassword}); a.status != http.StatusOK {
			t.Fatalf("unknown sign up status = %d: %s", a.status, a.body)
		}
		clk.Advance(25 * time.Second)
		ka := doAuth(t, kc, srv, "/api/auth/sign-up", map[string]string{"email": known, "password": e2eTestPassword})
		ua := doAuth(t, uc, srv, "/api/auth/sign-up", map[string]string{"email": unknown, "password": e2eTestPassword})
		expectSendWait(t, "known sign up", ka, 20, twenty)
		expectSendWait(t, "unknown sign up", ua, 20, twenty)
		compareAuthAnswers(t, "sign up known vs unknown", ka, ua)
		// Resend of each browser.
		compareAuthAnswers(t, "resend known vs unknown",
			doAuth(t, kc, srv, "/api/auth/code/resend", map[string]string{}),
			doAuth(t, uc, srv, "/api/auth/code/resend", map[string]string{}))

		// Forgot password: a known address and an unknown one.
		clk.Advance(46 * time.Second)
		kr, ur := newLoginClient(t, srv), newLoginClient(t, srv)
		if a := doAuth(t, kr, srv, "/api/auth/reset", map[string]string{"email": known}); a.status != http.StatusOK {
			t.Fatalf("known reset status = %d: %s", a.status, a.body)
		}
		if a := doAuth(t, ur, srv, "/api/auth/reset", map[string]string{"email": "wait.noaccount@example.com"}); a.status != http.StatusOK {
			t.Fatalf("unknown reset status = %d: %s", a.status, a.body)
		}
		clk.Advance(25 * time.Second)
		kra := doAuth(t, kr, srv, "/api/auth/reset", map[string]string{"email": known})
		ura := doAuth(t, ur, srv, "/api/auth/reset", map[string]string{"email": "wait.noaccount@example.com"})
		expectSendWait(t, "known reset", kra, 20, twenty)
		expectSendWait(t, "unknown reset", ura, 20, twenty)
		compareAuthAnswers(t, "reset known vs unknown", kra, ura)
	})
}
