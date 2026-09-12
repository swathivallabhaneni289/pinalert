// Package api's own test file (not api_test), reusing newGateTestServer from
// gate_test.go so the whole real router — not a hand-built handler chain —
// is under test. This is the regression proof for T-01-92: the per-IP
// limiter on POST /api/auth/request-link must key on the TCP peer address
// alone, never on a caller-supplied proxy header.
package api

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

// TestRequestLinkLimiterIgnoresSpoofedProxyHeaders drives six POSTs to the
// real /api/auth/request-link route from one http.Client (one underlying TCP
// connection pool, one apparent peer address as far as the server is
// concerned), each carrying a distinct forged X-Forwarded-For value and a
// distinct email address so the per-address cooldown can never be the
// reason a request is refused — the only control this test exercises is the
// per-IP bucket. Against the pre-fix router (middleware.RealIP mutating
// r.RemoteAddr to the forged header value) every request would mint a fresh
// bucket and all six would return 200; this test fails against that tree by
// design.
func TestRequestLinkLimiterIgnoresSpoofedProxyHeaders(t *testing.T) {
	srv, _, _ := newGateTestServer(t)

	emails := []string{
		"spoof-guard-1@example.com",
		"spoof-guard-2@example.com",
		"spoof-guard-3@example.com",
		"spoof-guard-4@example.com",
		"spoof-guard-5@example.com",
		"spoof-guard-6@example.com",
	}

	var got []int
	for i, email := range emails {
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/request-link",
			bytes.NewReader([]byte(`{"email":"`+email+`"}`)))
		if err != nil {
			t.Fatalf("building request %d: %v", i+1, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", forgedIPFor(i))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		got = append(got, resp.StatusCode)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("request %d: unexpected status %d: %s", i+1, resp.StatusCode, body)
		}
	}

	okCount, refusedCount := 0, 0
	for _, code := range got {
		switch code {
		case http.StatusOK:
			okCount++
		case http.StatusTooManyRequests:
			refusedCount++
		}
	}
	if okCount != 5 || refusedCount != 1 {
		t.Fatalf("status codes = %v, want exactly five 200s and one 429 (burst is %d)", got, RequestLinkRateLimitDefault.Burst)
	}
}

// forgedIPFor returns a distinct, syntactically valid IPv4 address for index
// i, used only to prove the limiter never reads it.
func forgedIPFor(i int) string {
	return "198.51.100." + string(rune('1'+i))
}
