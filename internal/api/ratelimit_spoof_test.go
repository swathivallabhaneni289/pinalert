// Package api's own test file (not api_test), reusing newGateTestServer from
// gate_test.go so the whole real router, not a hand-built handler chain, is
// under test. This is the regression proof for T-01-92: the buckets of the
// routes that send a code email must key on the browser's session, and on the
// network address only when the host names the header it overwrites, never on a
// caller-supplied proxy header.
//
// Amended 2026-10-29 by Phase 7.1 plan 09: this file used to hold the proof for
// the per IP limiter of POST /api/auth/request-link (TestRequestLinkLimiterIgnoresSpoofedProxyHeaders).
// The test is replaced by TestAuthSendLimiterIgnoresSpoofedProxyHeaders, the
// same property for POST /api/auth/sign-up, the code sending route that takes
// the place of the link route. The link limiter itself is removed with the link
// routes in the cleanup plan of Phase 7.1.
package api

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// TestAuthSendLimiterIgnoresSpoofedProxyHeaders sends six code emails from one
// browser, each for a distinct address (so the per address claim can never be
// the reason a request is refused) and each carrying a distinct forged
// X-Forwarded-For value. With no CLIENT_IP_HEADER set, the only control in play
// is the per browser bucket: exactly five requests pass and the sixth is a 429.
// Against a router that read the forged header into the bucket key, every
// request would mint a fresh bucket and all six would pass.
//
// The second case sets CLIENT_IP_HEADER to X-Real-IP. Cookieless requests that
// carry the same X-Real-IP and a different forged X-Forwarded-For each share
// one network bucket (three pass, the fourth is a 429); the forged header never
// selects a bucket, and a request from another X-Real-IP is not blocked.
func TestAuthSendLimiterIgnoresSpoofedProxyHeaders(t *testing.T) {
	t.Run("no_client_ip_header_keys_on_the_browser", func(t *testing.T) {
		srv, _, _ := newGateTestServerWith(t, func(d *Deps, _ *sqlcgen.Queries) {
			limits := gateGenerousAuthLimits
			limits.CodeSendDevice = BucketLimit{Every: time.Hour, Burst: 5}
			d.AuthLimits = limits
		})
		client := newGateJarClient(t)

		var got []int
		for i := 0; i < 6; i++ {
			email := "spoof-guard-" + strconv.Itoa(i+1) + "@example.com"
			status := postSignUp(t, client, srv.URL, email, forgedIPFor(i), "")
			got = append(got, status)
		}
		want := []int{200, 200, 200, 200, 200, 429}
		if !equalInts(got, want) {
			t.Fatalf("status codes = %v, want %v (CodeSendDevice burst is 5)", got, want)
		}
	})

	t.Run("client_ip_header_keys_on_the_named_header_only", func(t *testing.T) {
		srv, _, _ := newGateTestServerWith(t, func(d *Deps, _ *sqlcgen.Queries) {
			limits := gateGenerousAuthLimits
			limits.CodeSendIP = BucketLimit{Every: time.Hour, Burst: 3}
			d.AuthLimits = limits
			d.ClientIPHeader = "X-Real-IP"
		})

		var got []int
		for i := 0; i < 4; i++ {
			email := "spoof-net-" + strconv.Itoa(i+1) + "@example.com"
			got = append(got, postSignUp(t, http.DefaultClient, srv.URL, email, forgedIPFor(i), "203.0.113.50"))
		}
		want := []int{200, 200, 200, 429}
		if !equalInts(got, want) {
			t.Fatalf("one X-Real-IP with four forged X-Forwarded-For values: status codes = %v, want %v", got, want)
		}
		if other := postSignUp(t, http.DefaultClient, srv.URL, "spoof-net-other@example.com", forgedIPFor(0), "203.0.113.51"); other != http.StatusOK {
			t.Fatalf("a request from another X-Real-IP = %d, want 200", other)
		}
	})
}

// postSignUp POSTs a sign up for email with a forged X-Forwarded-For and, when
// realIP is not empty, an X-Real-IP, and returns the status.
func postSignUp(t *testing.T, client *http.Client, base, email, forged, realIP string) int {
	t.Helper()
	body := `{"email":"` + email + `","password":"` + gateTestPassword + `"}`
	req, err := http.NewRequest(http.MethodPost, base+"/api/auth/sign-up", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("building request for %s: %v", email, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", forged)
	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST sign up for %s: %v", email, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("POST sign up for %s: unexpected status %d: %s", email, resp.StatusCode, raw)
	}
	return resp.StatusCode
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// forgedIPFor returns a distinct, syntactically valid IPv4 address for index
// i, used only to prove the limiter never reads it.
func forgedIPFor(i int) string {
	return "198.51.100." + string(rune('1'+i))
}
