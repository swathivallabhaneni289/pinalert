// credentials_e2e_test.go drives the six code and password routes over the real
// router and a real Postgres (Phase 7.1, plan 07). They are smoke tests: the
// full matrix of limits, CSRF and replay is plan 09's.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/auth"
	"pinalert/internal/mailer"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

var authRoutePaths = []string{
	"/api/auth/sign-up",
	"/api/auth/sign-up/code",
	"/api/auth/code/resend",
	"/api/auth/sign-in",
	"/api/auth/reset",
	"/api/auth/reset/code",
	"/api/auth/reset/password",
	"/api/auth/reset/cancel",
}

// getLoginPage visits GET /login so the jar holds a session cookie, the way a
// browser on the sign in page does before it posts anything.
func getLoginPage(t *testing.T, client *http.Client, srv *httptest.Server) {
	t.Helper()
	resp, err := client.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /login status = %d, want 200", resp.StatusCode)
	}
}

// errorField decodes the shared error envelope of an answer.
func errorField(t *testing.T, raw []byte) (field, message string) {
	t.Helper()
	var env struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("answer %q is not the error envelope: %v", raw, err)
	}
	return env.Error.Field, env.Error.Message
}

func TestSignUpCodeSignsInSmoke(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newJarClient(t)
	const email = "smoke-signup@example.com"

	getLoginPage(t, client, srv)
	before := sessionIDFromJar(t, client, srv.URL)

	if status := signUpRequest(t, client, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	code := mailer.codeFor(t, email)

	status, raw, resp := postAuthJSON(t, client, srv, "/api/auth/sign-up/code", map[string]string{"code": code})
	if status != http.StatusOK {
		t.Fatalf("confirm code status = %d, want 200: %s", status, raw)
	}
	var signedIn struct {
		Next string `json:"next"`
	}
	if err := json.Unmarshal(raw, &signedIn); err != nil || signedIn.Next != "/" {
		t.Fatalf("confirm code answer = %q (err %v), want next /", raw, err)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	after := sessionIDFromJar(t, client, srv.URL)
	if after == before {
		t.Fatalf("the session id did not change at sign in (fixation): still %q", after)
	}
	if strings.Contains(string(raw), after) {
		t.Fatalf("the answer carries the session id")
	}

	reports, err := client.Get(srv.URL + "/api/reports?lat=12.9716&lon=77.5946&radius_km=5")
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer reports.Body.Close()
	if reports.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/reports after sign up status = %d, want 200", reports.StatusCode)
	}
}

func TestSignInWithPasswordSmoke(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	const email = "smoke-signin@example.com"

	first := newJarClient(t)
	getLoginPage(t, first, srv)
	if status := signUpRequest(t, first, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	if status := confirmSignUpCode(t, first, srv, mailer.codeFor(t, email)); status != http.StatusOK {
		t.Fatalf("confirm code status = %d, want 200", status)
	}

	// A second browser signs in with the password.
	second := newJarClient(t)
	getLoginPage(t, second, srv)
	before := sessionIDFromJar(t, second, srv.URL)
	status, raw, _ := postAuthJSON(t, second, srv, "/api/auth/sign-in", map[string]string{"email": email, "password": e2eTestPassword})
	if status != http.StatusOK {
		t.Fatalf("sign in status = %d, want 200: %s", status, raw)
	}
	if after := sessionIDFromJar(t, second, srv.URL); after == before {
		t.Fatalf("the session id did not change at sign in")
	}
	reports, err := second.Get(srv.URL + "/api/reports?lat=12.9716&lon=77.5946&radius_km=5")
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer reports.Body.Close()
	if reports.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/reports after sign in status = %d, want 200", reports.StatusCode)
	}

	// A third browser with a wrong password: 400 on the field credentials, never 401.
	third := newJarClient(t)
	getLoginPage(t, third, srv)
	status, raw, _ = postAuthJSON(t, third, srv, "/api/auth/sign-in", map[string]string{"email": email, "password": "not the right one 99"})
	if status != http.StatusBadRequest {
		t.Fatalf("wrong password status = %d, want 400: %s", status, raw)
	}
	if field, message := errorField(t, raw); field != "credentials" || message != "Email or password is wrong." {
		t.Fatalf("wrong password error = field %q message %q, want credentials and the fixed sentence", field, message)
	}
	// An unknown address gets the very same answer.
	status, raw2, _ := postAuthJSON(t, third, srv, "/api/auth/sign-in", map[string]string{"email": "nobody-here@example.com", "password": e2eTestPassword})
	if status != http.StatusBadRequest || string(raw2) != string(raw) {
		t.Fatalf("unknown address answer = %d %q, want the same 400 body as a wrong password %q", status, raw2, raw)
	}
}

func TestAuthRoutesWithoutCredentials(t *testing.T) {
	srv, _, _ := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.Credentials = nil
	})
	client := newJarClient(t)
	for _, path := range authRoutePaths {
		status, raw, resp := postAuthJSON(t, client, srv, path, map[string]string{})
		if status != http.StatusServiceUnavailable {
			t.Errorf("POST %s without a CredentialService status = %d, want 503: %s", path, status, raw)
			continue
		}
		if field, message := errorField(t, raw); field != "auth" || message != "Something went wrong. Try again." {
			t.Errorf("POST %s error = field %q message %q, want auth and the generic sentence", path, field, message)
		}
		if got := resp.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("POST %s Cache-Control = %q, want no-store", path, got)
		}
	}
	getLoginPage(t, client, srv)
}

// TestAuthRoutesRefuseCrossSitePost pins that the cross origin guard is on all
// six routes: a browser POST marked cross site is a 403 before any handler runs.
func TestAuthRoutesRefuseCrossSitePost(t *testing.T) {
	srv, _, _ := newE2EServer(t)
	for _, path := range authRoutePaths {
		req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(`{}`))
		if err != nil {
			t.Fatalf("building request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("cross site POST %s status = %d, want 403", path, resp.StatusCode)
		}
	}
}

// slowMailer delays every auth email, as a slow mail provider would.
type slowMailer struct {
	*recordingMailer
	delay time.Duration
}

func (m slowMailer) SendAuthEmail(ctx context.Context, to string, msg mailer.AuthEmail) error {
	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		return ctx.Err()
	}
	return m.recordingMailer.SendAuthEmail(ctx, to, msg)
}

// TestSendingRoutesOutliveTheServerWriteTimeout is the 10 second write limit
// check (07.1-PLAN-REVIEW, execution note): the production server cuts a
// response off after WriteTimeout, while the mail provider may take up to a
// minute. Here the server's WriteTimeout is 300 ms and the mailer takes 800 ms;
// each of the three routes that send a code email inside the request must still
// be answered.
func TestSendingRoutesOutliveTheServerWriteTimeout(t *testing.T) {
	rec := &recordingMailer{}
	slow := slowMailer{recordingMailer: rec, delay: 800 * time.Millisecond}
	limits := service.DefaultCredentialLimits
	limits.SendCooldown = time.Millisecond
	srv, _, _ := newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		d.Credentials = service.NewCredentialService(q, slow,
			service.WithHasher(auth.NewHasher(auth.TestParams())),
			service.WithCodeKey(bytes.Repeat([]byte("c"), 32)),
			service.WithThrottleKey(bytes.Repeat([]byte("t"), 32)),
			service.WithCredentialLimits(limits),
		)
	})
	// Set before the first request, so no connection has read it yet.
	srv.Config.WriteTimeout = 300 * time.Millisecond

	client := newJarClient(t)
	getLoginPage(t, client, srv)
	const email = "slow-mail@example.com"

	started := time.Now()
	status, raw, _ := postAuthJSON(t, client, srv, "/api/auth/sign-up", map[string]string{"email": email, "password": e2eTestPassword})
	if status != http.StatusOK || string(bytes.TrimSpace(raw)) != `{"sent":true}` {
		t.Fatalf("sign up with a slow mailer: status %d body %q, want 200 {\"sent\":true}", status, raw)
	}
	if took := time.Since(started); took < slow.delay {
		t.Fatalf("sign up took %v, less than the mailer delay %v: the test did not exercise a slow send", took, slow.delay)
	}

	status, raw, _ = postAuthJSON(t, client, srv, "/api/auth/code/resend", map[string]string{})
	if status != http.StatusOK {
		t.Fatalf("resend with a slow mailer: status %d body %q, want 200", status, raw)
	}

	status, raw, _ = postAuthJSON(t, client, srv, "/api/auth/reset", map[string]string{"email": email})
	if status != http.StatusOK {
		t.Fatalf("reset request with a slow mailer: status %d body %q, want 200", status, raw)
	}
	if got := rec.authSendCount(); got != 3 {
		t.Fatalf("auth emails sent = %d, want 3 (sign up, resend, reset)", got)
	}
}
