package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"pinalert/internal/service"
	"pinalert/internal/session"
)

const authTestIPHeader = "X-Test-IP"

// authTestLimits opens every bucket wide; a test narrows the one it studies.
func authTestLimits() AuthLimits {
	wide := BucketLimit{Every: time.Minute, Burst: 100}
	return AuthLimits{
		SignInDevice:     wide,
		SignInIP:         wide,
		CodeSendDevice:   wide,
		CodeSendIP:       wide,
		CodeSendNoCookie: wide,
		CodeTryDevice:    wide,
		UsernameCheck:    wide,
		UsernameSave:     wide,
	}
}

// authTestGate builds a gate on a clock that never moves, so a bucket never
// refills during a test.
func authTestGate(l AuthLimits, header string) *AuthGate {
	fixed := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	return NewAuthGate(l, func() time.Time { return fixed }, header)
}

func authTestManager(t *testing.T) *session.Manager {
	t.Helper()
	mgr, err := session.NewManager([]byte("authlimits-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	return mgr
}

// runAuthRequest sends one request through the client IP and session
// middleware, the way the router does, and calls fn with the request the
// handler would see. sessionID "" means the request carries no cookie at all;
// ip "" sends no client address.
func runAuthRequest(mgr *session.Manager, sessionID, ip string, fn func(r *http.Request)) {
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { fn(r) })
	persist := func(context.Context, string) error { return nil }
	h := middleware.ClientIPFromHeader(authTestIPHeader)(mgr.Middleware(persist)(inner))
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	if sessionID != "" {
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: mgr.Sign(sessionID)})
	}
	if ip != "" {
		req.Header.Set(authTestIPHeader, ip)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestAuthGateDeviceBuckets(t *testing.T) {
	mgr := authTestManager(t)
	l := authTestLimits()
	l.SignInDevice = BucketLimit{Every: time.Minute, Burst: 2}
	gate := authTestGate(l, "")

	reserve := func(sessionID string) (ok bool, retry time.Duration, cancel func()) {
		runAuthRequest(mgr, sessionID, "", func(r *http.Request) {
			d := gate.ReserveSignIn(r)
			ok, retry, cancel = d.OK, d.RetryAfter, d.Cancel
		})
		return ok, retry, cancel
	}

	for i := 0; i < 2; i++ {
		if ok, _, _ := reserve("dev-a"); !ok {
			t.Fatalf("sign in reservation %d for dev-a was refused, want admitted", i+1)
		}
	}
	ok, retry, _ := reserve("dev-a")
	if ok || retry <= 0 {
		t.Fatalf("third reservation for dev-a: ok=%v retry=%v, want refused with a wait", ok, retry)
	}
	if ok, _, _ := reserve("dev-b"); !ok {
		t.Fatalf("another session id was refused: the device key must be the session id")
	}

	// Cancel refunds: burst 2 for dev-c, one spent and given back, two more pass.
	_, _, cancel := reserve("dev-c")
	cancel()
	for i := 0; i < 2; i++ {
		if ok, _, _ := reserve("dev-c"); !ok {
			t.Fatalf("reservation %d for dev-c after a refund was refused", i+1)
		}
	}
	if ok, _, _ := reserve("dev-c"); ok {
		t.Fatalf("a refund must give back one token only")
	}

	// The code try bucket is device only and independent of the sign in one.
	l.CodeTryDevice = BucketLimit{Every: time.Minute, Burst: 1}
	gate = authTestGate(l, "")
	var first, second bool
	runAuthRequest(mgr, "dev-t", "", func(r *http.Request) { first = gate.ReserveCodeTry(r).OK })
	runAuthRequest(mgr, "dev-t", "", func(r *http.Request) { second = gate.ReserveCodeTry(r).OK })
	if !first || second {
		t.Fatalf("code try bucket burst 1: first=%v second=%v, want true then false", first, second)
	}
}

func TestAuthGateIPBackstopOnlyWithHeader(t *testing.T) {
	mgr := authTestManager(t)
	l := authTestLimits()
	l.SignInDevice = BucketLimit{Every: time.Minute, Burst: 1}
	l.SignInIP = BucketLimit{Every: time.Minute, Burst: 2}

	// Without CLIENT_IP_HEADER the network bucket is never consulted: ten
	// browsers behind one address each get their one device token.
	off := authTestGate(l, "")
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("off-%d", i)
		var ok bool
		runAuthRequest(mgr, id, "203.0.113.5", func(r *http.Request) { ok = off.ReserveSignIn(r).OK })
		if !ok {
			t.Fatalf("session %d behind one address was refused with the network bucket off", i)
		}
	}

	// With a header the third browser from one address is refused, and the
	// device token of that refused call is given back.
	on := authTestGate(l, authTestIPHeader)
	for i, want := range []bool{true, true, false} {
		id := fmt.Sprintf("on-%d", i)
		var got bool
		runAuthRequest(mgr, id, "203.0.113.5", func(r *http.Request) { got = on.ReserveSignIn(r).OK })
		if got != want {
			t.Fatalf("browser %d from one address: admitted=%v, want %v", i, got, want)
		}
	}
	var again bool
	runAuthRequest(mgr, "on-2", "198.51.100.9", func(r *http.Request) { again = on.ReserveSignIn(r).OK })
	if !again {
		t.Fatalf("the device token of the call the network bucket refused was not given back")
	}

	// With the header on but no address found, the bucket is skipped.
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("noip-%d", i)
		var ok bool
		runAuthRequest(mgr, id, "", func(r *http.Request) { ok = on.ReserveSignIn(r).OK })
		if !ok {
			t.Fatalf("a request with no client address was refused by the network bucket")
		}
	}
}

func TestIPBucketKeyIPv6Prefix(t *testing.T) {
	if a, b := ipBucketKey("2001:db8:1:2:3:4:5:6"), ipBucketKey("2001:db8:1:2:aaaa::1"); a == "" || a != b {
		t.Fatalf("two addresses in one /64 gave %q and %q, want one equal key", a, b)
	}
	if a, b := ipBucketKey("2001:db8:1:2::1"), ipBucketKey("2001:db8:1:3::1"); a == b {
		t.Fatalf("addresses in two /64 prefixes share the key %q", a)
	}
	if got := ipBucketKey("203.0.113.5"); got != "203.0.113.5" {
		t.Fatalf("IPv4 key = %q, want the address itself", got)
	}
	if got := ipBucketKey("::ffff:203.0.113.5"); got != "203.0.113.5" {
		t.Fatalf("IPv4 mapped key = %q, want the IPv4 address", got)
	}
	for _, bad := range []string{"", "   ", "not-an-ip", "203.0.113"} {
		if got := ipBucketKey(bad); got != "" {
			t.Fatalf("ipBucketKey(%q) = %q, want empty so the bucket is skipped", bad, got)
		}
	}
}

func TestAuthGateNoCookieSendCap(t *testing.T) {
	mgr := authTestManager(t)
	l := authTestLimits()
	l.CodeSendNoCookie = BucketLimit{Every: time.Minute, Burst: 2}
	gate := authTestGate(l, "")

	cookieless := func() bool {
		var ok bool
		runAuthRequest(mgr, "", "", func(r *http.Request) { ok = gate.ReserveCodeSend(r).OK })
		return ok
	}
	if !cookieless() || !cookieless() {
		t.Fatalf("the first two cookieless code sends must pass")
	}
	if cookieless() {
		t.Fatalf("the third cookieless code send must be refused by the shared cap")
	}
	// A request that carries the session cookie is not counted, even when the
	// shared bucket is spent.
	for i := 0; i < 5; i++ {
		var ok bool
		runAuthRequest(mgr, fmt.Sprintf("with-cookie-%d", i), "", func(r *http.Request) { ok = gate.ReserveCodeSend(r).OK })
		if !ok {
			t.Fatalf("a code send that carried the cookie was refused by the cookieless cap")
		}
	}

	// A refusal by the shared cap gives the device token back: burst 1 on the
	// device bucket would otherwise lock the id for good.
	l.CodeSendDevice = BucketLimit{Every: time.Minute, Burst: 1}
	l.CodeSendNoCookie = BucketLimit{Every: time.Minute, Burst: 1}
	gate = authTestGate(l, "")
	if !cookieless() {
		t.Fatalf("first cookieless send on the narrow gate was refused")
	}
	if cookieless() {
		t.Fatalf("second cookieless send on the narrow gate must be refused")
	}
	var ok bool
	runAuthRequest(mgr, "fresh", "", func(r *http.Request) { ok = gate.ReserveCodeSend(r).OK })
	if !ok {
		t.Fatalf("a browser with the cookie was refused after the cookieless cap filled")
	}
}

func TestDecodeAuthBodyStrict(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{"valid", "application/json", `{"email":"a@example.com"}`, 0},
		{"valid with charset", "application/json; charset=utf-8", `{"email":"a@example.com"}`, 0},
		{"text plain", "text/plain", `{"email":"a@example.com"}`, http.StatusUnsupportedMediaType},
		{"form", "application/x-www-form-urlencoded", `email=a%40example.com`, http.StatusUnsupportedMediaType},
		{"no content type", "", `{"email":"a@example.com"}`, http.StatusUnsupportedMediaType},
		{"too large", "application/json", `{"email":"` + strings.Repeat("a", maxCredentialBodyBytes) + `"}`, http.StatusRequestEntityTooLarge},
		{"unknown field", "application/json", `{"email":"a@example.com","extra":1}`, http.StatusBadRequest},
		{"trailing data", "application/json", `{"email":"a@example.com"}{"email":"b@example.com"}`, http.StatusBadRequest},
		{"malformed", "application/json", `{"email":`, http.StatusBadRequest},
		{"empty body", "application/json", ``, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			var dst ResetRequest
			ok := decodeAuthBody(rec, req, &dst)
			if tc.wantStatus == 0 {
				if !ok || dst.Email != "a@example.com" {
					t.Fatalf("valid body refused or not decoded: ok=%v dst=%+v status=%d", ok, dst, rec.Code)
				}
				return
			}
			if ok {
				t.Fatalf("body was accepted, want status %d", tc.wantStatus)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", got)
			}
			var env ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("refusal body is not the shared envelope: %v", err)
			}
			if env.Error.Message != "Something went wrong. Try again." {
				t.Fatalf("refusal message = %q, want the generic sentence", env.Error.Message)
			}
		})
	}
}

func TestCredentialErrorBusy503(t *testing.T) {
	for _, err := range []error{service.ErrBusy, service.ErrMailUnavailable, fmt.Errorf("wrapped: %w", service.ErrBusy)} {
		rec := httptest.NewRecorder()
		writeCredentialError(rec, "Test", err)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%v: status = %d, want 503", err, rec.Code)
		}
		var env ErrorResponse
		if jerr := json.Unmarshal(rec.Body.Bytes(), &env); jerr != nil {
			t.Fatalf("%v: body is not the shared envelope: %v", err, jerr)
		}
		if env.Error.Message != "Something went wrong. Try again." {
			t.Fatalf("%v: message = %q, want the shared sentence", err, env.Error.Message)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("%v: Cache-Control = %q, want no-store", err, got)
		}
	}
}

func TestCredentialErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantField  string
		wantMsg    string
		wantRetry  string
	}{
		{"validation", service.ValidationError{Field: "password", Message: service.PasswordTooShortMessage}, 400, "password", service.PasswordTooShortMessage, ""},
		{"wrong credentials", service.ErrWrongCredentials, 400, "credentials", "Email or password is wrong.", ""},
		{"wrong code", service.ErrWrongCode, 400, "code", service.WrongCodeMessage, ""},
		{"rate limited", service.ErrRateLimited, 429, "email", "Too many requests. Try again in a minute.", "60"},
		{"too many codes", service.ErrTooManyCodes, 429, "email", service.TooManyCodesMessage, "3600"},
		{"locked", service.SignInLockedError{Wait: 5 * time.Minute}, 429, "email", service.SignInLockedMessage(5 * time.Minute), "300"},
		{"anything else", fmt.Errorf("database is down"), 500, "auth", "Something went wrong. Try again.", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeCredentialError(rec, "Test", tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("a credential error must never be a 401")
			}
			var env ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("body is not the shared envelope: %v", err)
			}
			if env.Error.Field != tc.wantField || env.Error.Message != tc.wantMsg {
				t.Fatalf("error = %+v, want field %q message %q", env.Error, tc.wantField, tc.wantMsg)
			}
			if got := rec.Header().Get("Retry-After"); got != tc.wantRetry {
				t.Fatalf("Retry-After = %q, want %q", got, tc.wantRetry)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", got)
			}
		})
	}
}

func TestSignedInNextIsFixed(t *testing.T) {
	plain := httptest.NewRequest(http.MethodPost, "/api/auth/sign-in?next=https://evil.example/", nil)
	if got := signedInNext(plain); got != "/" {
		t.Fatalf("signedInNext = %q, want / whatever the query says", got)
	}
	low := httptest.NewRequest(http.MethodPost, "/api/auth/sign-in", nil)
	low.AddCookie(&http.Cookie{Name: LowDataCookie, Value: "1"})
	if got := signedInNext(low); got != LowDataPath {
		t.Fatalf("signedInNext with the Low data cookie = %q, want %q", got, LowDataPath)
	}
}

// TestExtendWriteDeadline pins the choice for the 10 second write limit: a
// handler that sends mail calls extendWriteDeadline, and its answer then
// outlives the server's WriteTimeout. The control handler without the call is
// cut off, so the test would catch a helper that does nothing.
func TestExtendWriteDeadline(t *testing.T) {
	serve := func(extend bool) (string, error) {
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if extend {
				extendWriteDeadline(w)
			}
			time.Sleep(400 * time.Millisecond)
			_, _ = io.WriteString(w, "answered")
		}))
		srv.Config.WriteTimeout = 100 * time.Millisecond
		srv.Start()
		defer srv.Close()
		resp, err := http.Get(srv.URL)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		return string(b), err
	}

	if body, err := serve(false); err == nil && body == "answered" {
		t.Fatalf("control: the slow handler without extendWriteDeadline was answered, so the test proves nothing")
	}
	body, err := serve(true)
	if err != nil || body != "answered" {
		t.Fatalf("with extendWriteDeadline: body=%q err=%v, want the answer to arrive", body, err)
	}

	// A writer that cannot take a deadline is left alone and does not panic.
	extendWriteDeadline(httptest.NewRecorder())
}
