package web

import (
	"strings"
	"testing"
)

// TestActivityPageHasNoBackLinkOrEmail replaces TestActivityPageLinksBackToTheMap.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21): the
// icon only back control and the signed in email are removed from My activity.
// The way to the map is the Nearby item of the two app bars, one tap away (D-02),
// and a part screen on the phone gets its back arrow to /profile from the top
// bar. The geometry locks between the old back link, the account button and the
// page padding are retired with the control. The Phase 3.1 and Phase 4 hands-on
// tests are walked on this layout after Phase 3.4 is built.
//
// What this proves: the shipped template and the shipped stylesheets carry no
// back link and no email line, and the page has no sentence that prints the
// verified address (T-34-32). It reads the embedded filesystems, never a copy.
func TestActivityPageHasNoBackLinkOrEmail(t *testing.T) {
	tmplRaw, err := TemplatesFS.ReadFile("templates/profile.html.tmpl")
	if err != nil {
		t.Fatalf("reading embedded templates/profile.html.tmpl: %v", err)
	}
	tmpl := string(tmplRaw)

	for _, gone := range []string{"profile-back-link", "profile-email", "{{.Email}}", "Back to map", "Back to Low data view"} {
		if strings.Contains(tmpl, gone) {
			t.Errorf("profile.html.tmpl still holds %q", gone)
		}
	}

	// The map page and the Activity page reach each other through the bars.
	if n := strings.Count(tmpl, `{{template "app_topbar" .}}`); n != 1 {
		t.Errorf("profile.html.tmpl includes the top bar %d times, want 1", n)
	}
	if n := strings.Count(tmpl, `{{template "app_bottombar" .}}`); n != 1 {
		t.Errorf("profile.html.tmpl includes the bottom bar %d times, want 1", n)
	}
}
