package web

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestActivityPageLinksBackToTheMap locks the Activity page's icon-only back
// control (web/templates/profile.html.tmpl). It reads the shipped template
// and stylesheets straight out of the embedded filesystems (TemplatesFS,
// StaticFS), never a copy of their text, so it cannot drift from what the
// binary serves.
//
// What this proves: the markup is an icon control (an aria-label, a title,
// the arrow-left glyph span, no visible text), the declarations that produce
// the alignment with the account button are present, and they are mutually
// consistent. Two geometry locks compare values read out of the shipped CSS
// against each other: .app-header top must equal .profile-page padding-top
// (one shared line), and .account-trigger height must equal
// .profile-back-link height (one shared center). What it cannot prove: that
// a browser paints those declarations as intended, that the "/" route
// resolves to the map shell, or that the focus ring is visible. Those need
// an eye on a real render.
func TestActivityPageLinksBackToTheMap(t *testing.T) {
	const backLinkClass = "profile-back-link"
	const emailClass = "profile-email"
	const accessibleName = "Back to map"

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
		t.Fatalf("could not locate %q in profile.html.tmpl, this test's premise (the back "+
			"control sits before the email row) no longer holds", emailClass)
	}
	if linkIdx >= emailIdx {
		t.Errorf("expected the back control to appear before the email row in the markup, "+
			"link at %d, email at %d", linkIdx, emailIdx)
	}

	// Slice out the anchor, failing loudly if it cannot be located so the
	// text-content check below never passes vacuously.
	openStart := strings.LastIndex(tmplText[:linkIdx], "<a")
	if openStart == -1 {
		t.Fatalf("could not find the opening <a tag before %q", backLinkClass)
	}
	closeRel := strings.Index(tmplText[openStart:], "</a>")
	if closeRel == -1 {
		t.Fatalf("could not find the closing </a> for the back control")
	}
	anchor := tmplText[openStart : openStart+closeRel+len("</a>")]
	openTagEnd := strings.Index(anchor, ">")
	openTag := anchor[:openTagEnd+1]

	if !strings.Contains(openTag, `href="/"`) {
		t.Errorf(`expected the back control's opening tag to carry href="/", got %s`, openTag)
	}
	for _, attr := range []string{"aria-label", "title"} {
		if !strings.Contains(openTag, attr+`="`+accessibleName+`"`) {
			t.Errorf("expected %s=%q on the back control, got %s", attr, accessibleName, openTag)
		}
	}
	if !strings.Contains(anchor, "auth-icon--arrow-left") {
		t.Errorf("expected an auth-icon--arrow-left span inside the back control")
	}
	tagRe := regexp.MustCompile(`<[^>]*>`)
	if text := strings.TrimSpace(tagRe.ReplaceAllString(anchor, "")); text != "" {
		t.Errorf("expected no visible text inside the back control (icon only), found %q", text)
	}

	cssRaw, err := StaticFS.ReadFile("static/css/auth.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/auth.css: %v", err)
	}
	rules := parseCSSRules(string(cssRaw))

	// Icon mask rule resolves to a real embedded file.
	iconRule, ok := ruleBySelector(rules, ".auth-icon--arrow-left")
	if !ok {
		t.Fatalf("no exact .auth-icon--arrow-left rule found in static/css/auth.css")
	}
	maskSrc := extractMaskURLPath(declsOf(iconRule.declBody)["mask-image"])
	if maskSrc == "" {
		t.Errorf(".auth-icon--arrow-left: no unprefixed mask-image url(...) declaration")
	} else if _, err := fs.Stat(StaticFS, strings.TrimPrefix(maskSrc, "/")); err != nil {
		t.Errorf(".auth-icon--arrow-left: mask-image %q does not resolve to an embedded file: %v",
			maskSrc, err)
	}

	rule, ok := ruleBySelector(rules, "."+backLinkClass)
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/auth.css", "."+backLinkClass)
	}
	decls := declsOf(rule.declBody)

	if _, ok := decls["display"]; !ok {
		t.Errorf(".%s: expected a display declaration", backLinkClass)
	}
	for _, prop := range []string{"width", "height", "min-width", "min-height"} {
		if got := decls[prop]; got != "var(--touch-target-min)" {
			t.Errorf(".%s: expected %s: var(--touch-target-min), got %q", backLinkClass, prop, got)
		}
	}
	if got := decls["border-radius"]; got != "50%" {
		t.Errorf(".%s: expected border-radius: 50%%, got %q", backLinkClass, got)
	}
	if got := decls["text-decoration"]; got != "none" {
		t.Errorf(".%s: expected text-decoration: none, got %q", backLinkClass, got)
	}

	// Every duration in the transition stays inside the restrained band.
	transition := decls["transition"]
	if transition == "" {
		t.Errorf(".%s: expected a transition declaration", backLinkClass)
	}
	durations := regexp.MustCompile(`(\d+)ms`).FindAllStringSubmatch(transition, -1)
	if len(durations) == 0 {
		t.Errorf(".%s: transition %q names no Nms duration", backLinkClass, transition)
	}
	for _, m := range durations {
		n, _ := strconv.Atoi(m[1])
		if n < 120 || n > 180 {
			t.Errorf(".%s: transition duration %dms is outside the 120 to 180ms band", backLinkClass, n)
		}
	}

	// Geometry lock 1: one shared line.
	headerRule, ok := ruleBySelector(rules, ".app-header")
	if !ok {
		t.Fatalf("no exact .app-header rule found in static/css/auth.css")
	}
	pageRule, ok := ruleBySelector(rules, ".profile-page")
	if !ok {
		t.Fatalf("no exact .profile-page rule found in static/css/auth.css")
	}
	headerTop, hasTop := declsOf(headerRule.declBody)["top"]
	pageTop, hasPad := declsOf(pageRule.declBody)["padding-top"]
	if !hasTop || !hasPad {
		t.Fatalf("need .app-header top and .profile-page padding-top declared, got top=%q padding-top=%q",
			headerTop, pageTop)
	}
	if headerTop != pageTop {
		t.Errorf("geometry lock: .app-header top (%q) must equal .profile-page padding-top (%q), "+
			"otherwise the back control and the account button leave their shared line",
			headerTop, pageTop)
	}

	// Geometry lock 2: one shared vertical center.
	triggerRule, ok := ruleBySelector(rules, ".account-trigger")
	if !ok {
		t.Fatalf("no exact .account-trigger rule found in static/css/auth.css")
	}
	triggerH := declsOf(triggerRule.declBody)["height"]
	if triggerH == "" || triggerH != decls["height"] {
		t.Errorf("geometry lock: .account-trigger height (%q) must equal .%s height (%q)",
			triggerH, backLinkClass, decls["height"])
	}

	// No outline suppression on the control; it relies on the global ring.
	for _, r := range rules {
		if strings.Contains(r.selectorHead, "."+backLinkClass) {
			if v := declsOf(r.declBody)["outline"]; v == "none" || v == "0" {
				t.Errorf("rule %q sets outline: %s, which removes the keyboard focus ring", r.selectorHead, v)
			}
		}
	}
	mainRaw, err := StaticFS.ReadFile("static/css/main.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/main.css: %v", err)
	}
	focusRule, ok := ruleBySelector(parseCSSRules(string(mainRaw)), ":focus-visible")
	if !ok {
		t.Errorf("main.css no longer declares a bare :focus-visible rule the back control relies on")
	} else if _, has := declsOf(focusRule.declBody)["outline"]; !has {
		t.Errorf("main.css :focus-visible rule declares no outline")
	}

	// Reduced motion removes the transition.
	foundReduced := false
	for _, r := range rules {
		if strings.Contains(r.selectorHead, "prefers-reduced-motion: reduce") &&
			strings.HasSuffix(r.selectorHead, "."+backLinkClass) &&
			declsOf(r.declBody)["transition"] == "none" {
			foundReduced = true
		}
	}
	if !foundReduced {
		t.Errorf("expected a prefers-reduced-motion: reduce rule setting transition: none on .%s", backLinkClass)
	}
}
