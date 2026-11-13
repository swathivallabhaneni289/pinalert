// reset_steps_limits_e2e_test.go pins the try bucket of the two reset routes
// (quick 261113-r5w, review fix WR-03): both take a token from the code try
// bucket of the browser, answer 429 when it is spent, give the token back on
// success, and step two also gives it back when the new password itself was
// refused, but keeps it for a wrong code answer.
package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"pinalert/internal/api"
	sqlcgen "pinalert/internal/store/sqlc"
)

// TestResetStepsTryBucket runs on a code try bucket of one token per browser
// that refills only after an hour, so the second call of a browser is refused
// unless the first gave its token back.
func TestResetStepsTryBucket(t *testing.T) {
	srv, mail, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.AuthLimits.CodeTryDevice = api.BucketLimit{Every: time.Hour, Burst: 1}
	})
	const email = "reset-try-bucket@example.com"
	verifySession(t, newPlainJarClient(t), srv, mail, email)
	ageClaim(t, pool, email)

	t.Run("step one answers 429 once the token is spent by a wrong code", func(t *testing.T) {
		r := newLoginClient(t, srv)
		if status := requestResetRequest(t, r, srv, email); status != http.StatusOK {
			t.Fatalf("reset request status = %d, want 200", status)
		}
		code := mail.codeFor(t, email)
		first := doAuth(t, r, srv, "/api/auth/reset/code", map[string]string{"code": wrongCodeFor(code)})
		expectError(t, "the first wrong code", first, http.StatusBadRequest, "code", secWrongCodeSentence)
		second := doAuth(t, r, srv, "/api/auth/reset/code", map[string]string{"code": code})
		expectError(t, "the right code with the bucket spent", second, http.StatusTooManyRequests, "email", secMinuteSentence)
	})

	t.Run("step two keeps the token for a wrong code answer and answers 429 after it", func(t *testing.T) {
		r := newLoginClient(t, srv) // no code asked at all: step two has no confirm
		first := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
		expectError(t, "step two with no confirm", first, http.StatusBadRequest, "code", secWrongCodeSentence)
		second := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
		if second.status != http.StatusTooManyRequests {
			t.Fatalf("a second step two after a wrong code answer = %d %s, want 429: the token must stay spent", second.status, second.body)
		}
	})

	t.Run("step two gives the token back for a refused password and on success", func(t *testing.T) {
		r := newLoginClient(t, srv)
		if status := requestResetRequest(t, r, srv, email); status != http.StatusOK {
			t.Fatalf("reset request status = %d, want 200", status)
		}
		// Step one's right code gives its token back, and the browser moves to a
		// fresh id with a bucket of its own.
		if status := confirmResetCodeRequest(t, r, srv, mail.codeFor(t, email)); status != http.StatusOK {
			t.Fatalf("step one status = %d, want 200", status)
		}
		for i := 0; i < 3; i++ {
			a := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": "short"})
			expectError(t, "a refused new password", a, http.StatusBadRequest, "password", "Use at least 8 characters.")
		}
		done := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
		if done.status != http.StatusOK {
			t.Fatalf("step two after refused passwords = %d %s, want 200: the token must come back", done.status, done.body)
		}
		// The success gave it back too, so the next call is a wrong code answer, not a 429.
		again := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": "yet another password 91"})
		expectError(t, "step two after the reset", again, http.StatusBadRequest, "code", secWrongCodeSentence)
	})
}
