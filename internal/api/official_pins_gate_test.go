package api

// The access gate rows for GET /api/official/pins (plan 05-21). They live in
// their own file so gate_test.go keeps its rows as they were. The pins route is
// in the gated group next to /api/official/alerts: an unverified caller gets the
// 401 envelope and never a pin list that could be read as "no hazards", and the
// path is not in the session skip set, so it is never answered without the
// session middleware.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOfficialPinsGated(t *testing.T) {
	srv, _, mailer := newGateTestServer(t)

	t.Run("an unverified caller gets the 401 envelope", func(t *testing.T) {
		client := newGateJarClient(t)
		resp, err := client.Get(srv.URL + "/api/official/pins")
		if err != nil {
			t.Fatalf("GET /api/official/pins: %v", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading the body: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401: %s", resp.StatusCode, body)
		}
		var env struct {
			Error struct {
				Field string `json:"field"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &env); err != nil || env.Error.Field != "auth" {
			t.Fatalf("body = %s (%v), want the auth envelope", body, err)
		}
		if strings.Contains(string(body), "pins") {
			t.Fatalf("an unverified body must carry no pin data: %s", body)
		}
	})

	t.Run("a verified caller reaches the route, not a 404", func(t *testing.T) {
		client := newGateJarClient(t)
		gateVerifySession(t, client, srv, mailer, "pins-gated@example.com")
		resp, err := client.Get(srv.URL + "/api/official/pins")
		if err != nil {
			t.Fatalf("GET /api/official/pins: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		// This server has no official service, so the handler answers its plain
		// 503: the route exists and is past the gate.
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503 (no service on this server): %s", resp.StatusCode, body)
		}
		if strings.Contains(string(body), `"pins"`) {
			t.Errorf("a refusal must not hold a pin list: %s", body)
		}
	})

	t.Run("the path is not in the session skip set", func(t *testing.T) {
		skip := sessionSkipSet()
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			if skip.matches(httptest.NewRequest(method, "/api/official/pins", nil)) {
				t.Errorf("%s /api/official/pins is in the session skip set", method)
			}
		}
	})
}
