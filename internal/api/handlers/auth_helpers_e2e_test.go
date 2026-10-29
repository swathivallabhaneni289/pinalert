// auth_helpers_e2e_test.go holds the harness pieces of the code and password
// routes (Phase 7.1, plan 07): the service the e2e router is built with and
// small helpers that drive the six routes over a real HTTP server.
package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pinalert/internal/auth"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// e2eTestPassword passes the password rule of plan 02 (length, not common, not
// the email).
const e2eTestPassword = "wide river morning 47"

// newTestCredentials builds the CredentialService the e2e router uses. The e2e
// harness is the only place the fast test hasher is passed: production uses the
// default parameters. The keys are fixed 32 byte test values. SendCooldown is
// one millisecond, so a sign up followed by a reset on the same address in one
// test is not refused by the 45 second per address cooldown (plan 05 note).
func newTestCredentials(q *sqlcgen.Queries, m *recordingMailer) *service.CredentialService {
	limits := service.DefaultCredentialLimits
	limits.SendCooldown = time.Millisecond
	return service.NewCredentialService(q, m,
		service.WithHasher(auth.NewHasher(auth.TestParams())),
		service.WithCodeKey(bytes.Repeat([]byte("c"), 32)),
		service.WithThrottleKey(bytes.Repeat([]byte("t"), 32)),
		service.WithCredentialLimits(limits),
	)
}

// postAuthJSON posts body as JSON to path on srv and returns the status, the
// body bytes and the response.
func postAuthJSON(t *testing.T, client *http.Client, srv *httptest.Server, path string, body any) (int, []byte, *http.Response) {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling request body for %s: %v", path, err)
	}
	resp, err := client.Post(srv.URL+path, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the answer of %s: %v", path, err)
	}
	return resp.StatusCode, raw, resp
}

func signUpRequest(t *testing.T, client *http.Client, srv *httptest.Server, email, password string) int {
	t.Helper()
	status, _, _ := postAuthJSON(t, client, srv, "/api/auth/sign-up", map[string]string{"email": email, "password": password})
	return status
}

func confirmSignUpCode(t *testing.T, client *http.Client, srv *httptest.Server, code string) int {
	t.Helper()
	status, _, _ := postAuthJSON(t, client, srv, "/api/auth/sign-up/code", map[string]string{"code": code})
	return status
}

func signInRequest(t *testing.T, client *http.Client, srv *httptest.Server, email, password string) int {
	t.Helper()
	status, _, _ := postAuthJSON(t, client, srv, "/api/auth/sign-in", map[string]string{"email": email, "password": password})
	return status
}

func requestResetRequest(t *testing.T, client *http.Client, srv *httptest.Server, email string) int {
	t.Helper()
	status, _, _ := postAuthJSON(t, client, srv, "/api/auth/reset", map[string]string{"email": email})
	return status
}

func confirmResetRequest(t *testing.T, client *http.Client, srv *httptest.Server, code, password string) int {
	t.Helper()
	status, _, _ := postAuthJSON(t, client, srv, "/api/auth/reset/code", map[string]string{"code": code, "password": password})
	return status
}
