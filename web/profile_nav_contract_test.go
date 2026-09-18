package web

import (
	"strings"
	"testing"
)

// TestActivityPageLinksBackToTheMap closes UAT gap 4: the Activity page
// (web/templates/profile.html.tmpl) previously rendered no way back to the
// map and feed other than the browser's own Back button. This test reads
// the shipped template and stylesheet straight out of the embedded
// filesystems (TemplatesFS, StaticFS) — never a copy of their text — so a
// future edit to either asset can only drift this test's own claim, not
// silently diverge from what actually ships in the binary.
//
// Honest limit of this test's claim, in the same spirit as this package's
// other contract tests: static inspection proves the link exists, is
// reachable in the markup, and declares a 44px min-height floor. It cannot
// prove the rendered element is actually 44px tall after the cascade, or
// that the "/" route resolves to the map and feed shell — those are exactly
// this plan's human-check verify step's job, and the two are complementary,
// not redundant.
func TestActivityPageLinksBackToTheMap(t *testing.T) {
	const backLinkClass = "profile-back-link"
	const emailClass = "profile-email"

	tmplRaw, err := TemplatesFS.ReadFile("templates/profile.html.tmpl")
	if err != nil {
		t.Fatalf("reading embedded templates/profile.html.tmpl: %v", err)
	}
	tmplText := string(tmplRaw)

	if got := strings.Count(tmplText, backLinkClass); got != 1 {
		t.Fatalf("expected %q to appear exactly once in profile.html.tmpl, found %d", backLinkClass, got)
	}

	linkIdx := strings.Index(tmplText, backLinkClass)
	emailIdx := strings.Index(tmplText, emailClass)
	if emailIdx == -1 {
		t.Fatalf("could not locate %q in profile.html.tmpl — this test's premise (the back link "+
			"sits before the email row) no longer holds", emailClass)
	}
	if linkIdx >= emailIdx {
		t.Errorf("expected the back link (%q) to appear before the email row's class (%q) in the "+
			"markup, so it is the first thing in the page rather than buried under the activity "+
			"list — link at %d, email at %d", backLinkClass, emailClass, linkIdx, emailIdx)
	}

	if !strings.Contains(tmplText, `href="/"`) {
		t.Errorf("expected the back link's href to target the application root (\"/\") — found no "+
			`href="/" in profile.html.tmpl`)
	}
	if !strings.Contains(tmplText, "Back to map") {
		t.Errorf("expected the visible label %q in profile.html.tmpl", "Back to map")
	}

	cssRaw, err := StaticFS.ReadFile("static/css/auth.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/auth.css: %v", err)
	}
	rules := parseCSSRules(string(cssRaw))

	rule, ok := ruleBySelector(rules, "."+backLinkClass)
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/auth.css", "."+backLinkClass)
	}
	decls := declsOf(rule.declBody)

	minHeight, ok := decls["min-height"]
	if !ok {
		t.Errorf(".%s: expected a min-height declaration meeting the 44px touch-target floor",
			backLinkClass)
	} else if minHeight != "var(--touch-target-min)" {
		t.Errorf(".%s: expected min-height: var(--touch-target-min), got %q", backLinkClass, minHeight)
	}

	if _, ok := decls["display"]; !ok {
		t.Errorf(".%s: expected a display declaration — min-height alone does not let an inline "+
			"element actually take that height", backLinkClass)
	}
}
