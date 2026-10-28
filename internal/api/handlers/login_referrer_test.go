// Plan 05-07 (D-13, sketch item 15, T-05-35): the sign in page sends
// Referrer-Policy same-origin, so the sign in globe's tile requests, which
// MapLibre sends with the full page address as the referrer, never carry the
// address (it holds the email after Send) to another host. The page also draws
// the globe only when the device has not chosen the Low data view.
//
// No database: the handler is served directly with the real templates.

package handlers_test

import (
	"net/http"
	"strings"
	"testing"

	"pinalert/internal/api/handlers"
)

func TestLoginGateReferrerPolicyAndGlobe(t *testing.T) {
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}

	cases := []struct {
		name       string
		target     string
		withCookie bool
		wantGlobe  bool
	}{
		{name: "plain", target: "/login", wantGlobe: true},
		{name: "after send", target: "/login?sent=priya%40example.com", wantGlobe: true},
		{name: "low data remembered", target: "/login", withCookie: true, wantGlobe: false},
		{name: "low data just chosen", target: "/login?low=1", wantGlobe: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := p421Login(t, tmpl, tc.target, tc.withCookie)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", tc.target, rec.Code)
			}
			if got := rec.Header().Get("Referrer-Policy"); got != "same-origin" {
				t.Errorf("Referrer-Policy = %q, want same-origin", got)
			}
			body := rec.Body.String()
			hasGlobe := strings.Contains(body, `id="login-globe"`)
			if hasGlobe != tc.wantGlobe {
				t.Errorf("globe drawn = %v, want %v", hasGlobe, tc.wantGlobe)
			}
			if !tc.wantGlobe {
				for _, banned := range []string{"unpkg", "login-globe", "globe"} {
					if strings.Contains(strings.ToLower(body), banned) {
						t.Errorf("the Low data page holds %q", banned)
					}
				}
			}
			if n := strings.Count(body, "Try the demo"); n != 1 {
				t.Errorf("the page holds Try the demo %d times, want 1", n)
			}
		})
	}
}
