package web

import (
	"strings"
	"testing"
)

// Phase 8 plan 16 (PAGE-02, D-02, UI-SPEC Surface 7): the contracts of the
// comments half of the report page that the node tests cannot see in the
// source. comments_test.mjs runs the real comments.js; this file pins the
// parts that live in the stylesheets, the template and the shape of the script.

// commentRules returns the rules of a stylesheet, parsed without comments.
func commentRules(t *testing.T, file string) []p16CSSRule {
	t.Helper()
	return p16ParseCSS(file, p16ReadStatic(t, file))
}

// commentCustomProperty reads a custom property declared on the .comments root
// rule of comments.css, so a rule that says var(--name) can be resolved.
func commentCustomProperty(t *testing.T, name string) string {
	t.Helper()
	for _, r := range commentRules(t, "static/css/comments.css") {
		if v, ok := r.Decls[name]; ok {
			return v
		}
	}
	t.Fatalf("comments.css declares no %s", name)
	return ""
}

// TestComposerIsNotAPill pins UI-SPEC departure D2: the box is one rounded
// field with a 12px radius, below half its 44px height, because a 22px radius
// on a 44px field is a pill (TestNoPillShapedControls).
func TestComposerIsNotAPill(t *testing.T) {
	var field *p16CSSRule
	rules := commentRules(t, "static/css/comments.css")
	for i := range rules {
		for _, part := range p16HeadParts(rules[i].Head) {
			if strings.HasSuffix(strings.TrimSuffix(part, ":not([hidden])"), ".composer__field") && rules[i].Decls["border-radius"] != "" {
				field = &rules[i]
			}
		}
	}
	if field == nil {
		t.Fatal("comments.css has no rule for .composer__field")
	}
	radius := field.Decls["border-radius"]
	if strings.HasPrefix(radius, "var(") {
		radius = commentCustomProperty(t, strings.TrimSuffix(strings.TrimPrefix(radius, "var("), ")"))
	}
	if radius != "12px" {
		t.Errorf(".composer__field radius = %q, want 12px (UI-SPEC D2)", radius)
	}
	if min := field.Decls["min-height"]; min != "var(--touch-target-min)" {
		t.Errorf(".composer__field min-height = %q, want var(--touch-target-min), so 12px is below half the height", min)
	}
	for _, r := range rules {
		if strings.Contains(r.Head, "composer") && strings.Contains(r.Decls["border-radius"], "999") {
			t.Errorf("%s draws a pill radius: %q", r.Head, r.Decls["border-radius"])
		}
	}
}

// TestComposerCountsCodePoints pins review W2 and research Pitfall 7: the
// counter counts with Array.from, as the server counts runes, and the textarea
// carries no length attribute, because maxlength counts UTF-16 units and would
// cut Telugu or emoji text short. The UI-SPEC calls maxlength "a guard"; the
// plan, review W2 and this test win (recorded in the 08-16 SUMMARY).
func TestComposerCountsCodePoints(t *testing.T) {
	code := p13CodeLines(p13ReadStatic(t, "static/js/comments.js"))
	p13MustContain(t, "comments.js", code,
		"Array.from(", "COMMENT_MAX = 280", "COUNTER_FROM = 40", "COMMENTS_POLL_MS = 30000", "COMMENTS_TIMEOUT_MS = 10000",
		"'use strict'", "window.PinalertComments", "Pinalert.setText(", "document.createTextNode(", "stopPropagation()")
	if strings.Contains(strings.ToLower(code), "maxlength") {
		t.Errorf("comments.js names maxlength; the textarea must carry no length attribute (review W2)")
	}
	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(code, sink) {
			t.Errorf("comments.js holds the markup sink %s (T-08-70)", sink)
		}
	}
	if strings.Contains(code, ".length > COMMENT_MAX") {
		t.Errorf("comments.js measures the box in UTF-16 units; count with Array.from")
	}
}

// TestCommentsHiddenGuard pins the hidden guard. Every rule that sets display
// (other than none) on a comments, comment or composer part carries
// :not([hidden]) in each selector part, in comments.css and in the host rules
// of phone.css and report-panel.css, so the hidden attribute still hides it.
func TestCommentsHiddenGuard(t *testing.T) {
	checked := 0
	for _, file := range []string{"static/css/comments.css", "static/css/phone.css", "static/css/report-panel.css"} {
		for _, r := range commentRules(t, file) {
			display, has := r.Decls["display"]
			if !has || display == "none" || strings.HasPrefix(r.Media, "@keyframes") {
				continue
			}
			for _, part := range p16HeadParts(r.Head) {
				if !strings.Contains(part, ".comment") && !strings.Contains(part, ".composer") {
					continue
				}
				checked++
				if !strings.Contains(part, ":not([hidden])") {
					t.Errorf("%s: %q sets display %s without :not([hidden])", file, part, display)
				}
			}
		}
	}
	if checked < 20 {
		t.Errorf("only %d display rules on comments parts were checked; the walk may have stopped matching", checked)
	}
}

// TestComposerPinnedOnLaptopOnly pins UI-SPEC 7c: the box is pinned to the
// bottom of the laptop panel and sits in the flow after the last comment on a
// phone. position sticky on the composer appears only inside report-panel.css's
// min-width 900px block.
func TestComposerPinnedOnLaptopOnly(t *testing.T) {
	pinned := 0
	for _, file := range []string{"static/css/comments.css", "static/css/phone.css", "static/css/report-panel.css"} {
		for _, r := range commentRules(t, file) {
			if r.Decls["position"] != "sticky" || !strings.Contains(r.Head, "composer") {
				continue
			}
			if file != "static/css/report-panel.css" || !strings.Contains(r.Media, "min-width: 900px") {
				t.Errorf("%s (%q) pins the box with position sticky outside report-panel.css's min-width block", file, r.Head)
				continue
			}
			pinned++
			if r.Decls["bottom"] != "0" {
				t.Errorf("the pinned box has bottom %q, want 0", r.Decls["bottom"])
			}
		}
	}
	if pinned == 0 {
		t.Errorf("report-panel.css pins no composer; the laptop box is pinned to the bottom of the panel (UI-SPEC 7c)")
	}
}

// TestCommentsAreLinkedInOrder pins where index.html.tmpl links the comments:
// the stylesheet once, after report-page.css and before report-panel.css and
// phone.css (the host rules win a tie by loading last), and the script once,
// deferred, after report-page.js and before map.js.
func TestCommentsAreLinkedInOrder(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")
	if n := strings.Count(index, "/static/js/comments.js"); n != 1 {
		t.Fatalf("index.html.tmpl loads comments.js %d times, want 1", n)
	}
	if n := strings.Count(index, "/static/css/comments.css"); n != 1 {
		t.Fatalf("index.html.tmpl links comments.css %d times, want 1", n)
	}
	p13MustContain(t, "comments.js tag", findTagWindow(t, index, "/static/js/comments.js"), " defer", "?v={{.AssetVersion}}")
	p13MustContain(t, "comments.css tag", findTagWindow(t, index, "/static/css/comments.css"), "?v={{.AssetVersion}}")
	p13Ordered(t, index, "index.html.tmpl scripts",
		"/static/js/report-page.js", "/static/js/comments.js", "/static/js/map.js")
	p13Ordered(t, index, "index.html.tmpl styles",
		"/static/css/report-page.css", "/static/css/comments.css", "/static/css/report-panel.css", "/static/css/phone.css")
}
