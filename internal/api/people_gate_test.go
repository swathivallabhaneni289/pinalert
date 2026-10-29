// people_gate_test.go is V20 of Phase 9 plan 07: every people route sits behind
// the access gate, so a caller who is not signed in gets the 401 JSON envelope
// and never a profile, a list or a changed row. Plan 09-08 adds the two HTML
// routes to this table.
//
// Amended 2026-10-29 by Phase 9 plan 14 (D-02): the two rating routes join the
// table. Every other assertion is unchanged.
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAccessGateBlocksPeopleRoutes(t *testing.T) {
	srv, _, _ := newGateTestServer(t)
	client := newGateJarClient(t)

	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/people/asha_k"},
		{http.MethodGet, "/api/people/asha_k/followers"},
		{http.MethodGet, "/api/people/asha_k/following"},
		{http.MethodGet, "/api/people/asha_k/reports"},
		{http.MethodPost, "/api/people/asha_k/follow"},
		{http.MethodPost, "/api/people/asha_k/unfollow"},
		{http.MethodPost, "/api/people/asha_k/remove-follower"},
		{http.MethodPost, "/api/people/asha_k/block"},
		{http.MethodPost, "/api/people/asha_k/unblock"},
		{http.MethodPost, "/api/people/asha_k/rating"},
		{http.MethodPost, "/api/people/asha_k/rating/clear"},
		{http.MethodPost, "/api/me/followers/remove-all"},
		{http.MethodGet, "/api/me/blocked"},
		// Plan 09-19 (D-12): the three person mute routes are gated too.
		{http.MethodPost, "/api/people/asha_k/mute"},
		{http.MethodPost, "/api/people/asha_k/unmute"},
		{http.MethodGet, "/api/me/muted-people"},
		// Plan 09-23 (D-08): the Notify me route is gated too.
		{http.MethodPost, "/api/people/asha_k/notify"},
		{http.MethodPatch, "/api/me/visibility"},
	}
	// Plan 09-08: the two HTML routes are gated like every page, so an
	// anonymous caller is sent to /login and never sees a profile.
	pages := []string{"/u/someone", "/me"}
	for _, path := range pages {
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			t.Errorf("GET %s status = %d, want 302: %s", path, resp.StatusCode, raw)
			continue
		}
		if loc := resp.Header.Get("Location"); loc != "/login" {
			t.Errorf("GET %s Location = %q, want /login", path, loc)
		}
		if strings.Contains(string(raw), "someone") {
			t.Errorf("GET %s body carries the requested name: %s", path, raw)
		}
	}
	for _, rt := range routes {
		var body io.Reader
		if rt.method != http.MethodGet {
			body = strings.NewReader(`{"profile_public":true}`)
		}
		req, err := http.NewRequest(rt.method, srv.URL+rt.path, body)
		if err != nil {
			t.Fatalf("building %s %s: %v", rt.method, rt.path, err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", rt.method, rt.path, err)
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("reading %s %s body: %v", rt.method, rt.path, err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d, want 401: %s", rt.method, rt.path, resp.StatusCode, raw)
			continue
		}
		var envelope struct {
			Error struct {
				Field string `json:"field"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Errorf("%s %s: the 401 body is not the JSON envelope: %s", rt.method, rt.path, raw)
			continue
		}
		if envelope.Error.Field != "auth" {
			t.Errorf("%s %s 401 field = %q, want %q", rt.method, rt.path, envelope.Error.Field, "auth")
		}
		for _, leak := range []string{"followers_count", "username\":\"asha_k", "entries"} {
			if strings.Contains(string(raw), leak) {
				t.Errorf("%s %s 401 body carries %q: %s", rt.method, rt.path, leak, raw)
			}
		}
	}
}
