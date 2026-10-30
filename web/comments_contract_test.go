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
// phone. A pinned position on the composer appears only inside
// report-panel.css's min-width 900px block.
//
// Amended 2026-10-29 by the Phase 8 review fix (WR-T01): the pin is position
// fixed, not sticky, because a sticky box only held while .comments was on
// screen. TestPanelComposerPinnedToPanelBottom pins its geometry.
func TestComposerPinnedOnLaptopOnly(t *testing.T) {
	pinned := 0
	for _, file := range []string{"static/css/comments.css", "static/css/phone.css", "static/css/report-panel.css"} {
		for _, r := range commentRules(t, file) {
			pos := r.Decls["position"]
			if (pos != "sticky" && pos != "fixed") || !strings.Contains(r.Head, "composer") {
				continue
			}
			if file != "static/css/report-panel.css" || !strings.Contains(r.Media, "min-width: 900px") {
				t.Errorf("%s (%q) pins the box with position %s outside report-panel.css's min-width block", file, r.Head, pos)
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

// TestCommentFlagUsesCommentMode (Phase 8 plan 17, D-14, review B3): the Flag
// control of a comment calls PinalertFlag.openForComment, and flag.js exports
// it, so the two plans cannot drift apart and leave the flag path silently
// absent while every other test stays green. The control sends nothing itself.
func TestCommentFlagUsesCommentMode(t *testing.T) {
	comments := p16ReadStatic(t, "static/js/comments.js")
	flag := p16ReadStatic(t, "static/js/flag.js")
	if !strings.Contains(comments, "PinalertFlag.openForComment(") {
		t.Error("comments.js never calls PinalertFlag.openForComment, so a comment cannot be flagged (D-14)")
	}
	for _, want := range []string{"function openForComment(", "openForComment: openForComment"} {
		if !strings.Contains(flag, want) {
			t.Errorf("flag.js lacks %q, which the Flag control of a comment calls", want)
		}
	}
	if strings.Contains(comments, "/flag") {
		t.Error("comments.js names a /flag path; the dialog sends the flag, the control never does")
	}
	if strings.Contains(comments, "commentId: rec.id") == false || strings.Contains(comments, "onSent:") == false {
		t.Error("comments.js must pass commentId, opener and onSent to openForComment")
	}
}

// TestThumbsUpIconsRegistered (Phase 8 plan 17, D-02): both thumbs up glyphs
// exist with the icons' licence line and are drawn by literal mask urls in
// comments.css; the filled one carries a solid fill and the outline one a
// plain stroke.
func TestThumbsUpIconsRegistered(t *testing.T) {
	css := p16ReadStatic(t, "static/css/comments.css")
	for _, name := range []string{"thumbs-up.svg", "thumbs-up-filled.svg"} {
		raw := p16ReadStatic(t, "static/icons/"+name)
		first, _, _ := strings.Cut(raw, "\n")
		if !strings.Contains(first, "@license lucide") {
			t.Errorf("%s first line %q lacks the icons' licence line", name, first)
		}
		if !strings.Contains(raw, `viewBox="0 0 24 24"`) || !strings.Contains(raw, `stroke="currentColor"`) {
			t.Errorf("%s is not a 24 by 24 currentColor glyph", name)
		}
		if strings.Contains(raw, "<script") || strings.Contains(raw, "href") {
			t.Errorf("%s holds a script or a link, which a mask glyph never carries", name)
		}
		want := `url("/static/icons/` + name + `")`
		if !strings.Contains(css, want) {
			t.Errorf("comments.css does not draw %s with the literal %s", name, want)
		}
	}
	if !strings.Contains(p16ReadStatic(t, "static/icons/thumbs-up.svg"), `fill="none"`) {
		t.Error("thumbs-up.svg must be the outline glyph (fill none)")
	}
	if !strings.Contains(p16ReadStatic(t, "static/icons/thumbs-up-filled.svg"), `fill="currentColor"`) {
		t.Error("thumbs-up-filled.svg must be the filled glyph (fill currentColor), so a liked comment is a shape and not colour alone")
	}
}

// TestNoHeartOrEmojiLike (Phase 8 plan 17, owner 2026-10-17: a thumbs up, not
// a heart): comments.js and comments.css name no heart, and comments.js holds
// no character above U+2FFF other than the ellipsis U+2026, so no emoji can
// stand in for the icon.
func TestNoHeartOrEmojiLike(t *testing.T) {
	for _, file := range []string{"static/js/comments.js", "static/css/comments.css"} {
		text := p16ReadStatic(t, file)
		if strings.Contains(strings.ToLower(text), "heart") {
			t.Errorf("%s names a heart; a comment is liked with a thumbs up", file)
		}
		for i, r := range text {
			if r > 0x2FFF || (r >= 0x2190 && r <= 0x2BFF) {
				t.Errorf("%s holds the character %q (U+%04X) at byte %d; no emoji or symbol stands in for an icon", file, r, r, i)
				break
			}
		}
	}
}

// TestDeleteBoxReusesLogoutConfirm (Phase 8 plan 17, D-14, owner rule on
// confirm boxes, 2026-10-22): the Delete comment box is the Log out box's own
// markup, the .logout-confirm classes and the btn and btn--destructive buttons,
// with no alert class of its own, and index.html.tmpl includes it.
func TestDeleteBoxReusesLogoutConfirm(t *testing.T) {
	box := p13ReadTemplate(t, "comment_delete_confirm.html.tmpl")
	index := p13ReadTemplate(t, "index.html.tmpl")
	p13MustContain(t, "the Delete box", box,
		`<dialog id="comment-delete-confirm" class="logout-confirm"`,
		`class="logout-confirm__body"`,
		`class="logout-confirm__title" id="comment-delete-title" tabindex="-1">Delete this comment?<`,
		`class="logout-confirm__actions"`,
		`class="btn" id="comment-delete-cancel">Cancel<`,
		`class="btn btn--destructive" id="comment-delete-yes">Delete<`)
	allowed := map[string]bool{
		"logout-confirm": true, "logout-confirm__body": true, "logout-confirm__title": true,
		"logout-confirm__actions": true, "btn": true, "btn--destructive": true,
	}
	markup := box
	if _, rest, ok := strings.Cut(box, `{{define "comment_delete_confirm"}}`); ok {
		markup = rest
	}
	for _, chunk := range strings.Split(markup, `class="`)[1:] {
		classes, _, _ := strings.Cut(chunk, `"`)
		for _, name := range strings.Fields(classes) {
			if !allowed[name] {
				t.Errorf("the Delete box uses the class %q; it reuses .logout-confirm and adds no alert styling of its own", name)
			}
		}
	}
	if !strings.Contains(index, `{{template "comment_delete_confirm"}}`) {
		t.Error("index.html.tmpl does not include the Delete box")
	}
	if strings.Contains(index, "<dialog id=\"comment-delete-confirm\"") {
		t.Error("index.html.tmpl holds the Delete box itself; it lives in comment_delete_confirm.html.tmpl (TestReportFormPartialKeepsMainFormOrder keeps no bare Cancel button in the index)")
	}
}

// TestBlockBoxReusesLogoutConfirm (quick 261030-u78, WR-E04, owner rule on
// confirm boxes, 2026-10-22): the Block box beside Flag is the Log out box's own
// markup, like the Delete box, with no autofocus, and index.html.tmpl includes
// its partial and does not hold the dialog.
func TestBlockBoxReusesLogoutConfirm(t *testing.T) {
	box := p13ReadTemplate(t, "comment_block_confirm.html.tmpl")
	index := p13ReadTemplate(t, "index.html.tmpl")
	p13MustContain(t, "the Block box", box,
		`<dialog id="comment-block-confirm" class="logout-confirm" aria-labelledby="comment-block-title" aria-describedby="comment-block-text"`,
		`class="logout-confirm__body"`,
		`class="logout-confirm__title" id="comment-block-title" tabindex="-1"`,
		`class="logout-confirm__text" id="comment-block-text"`,
		`class="logout-confirm__actions"`,
		`class="btn" id="comment-block-cancel">Cancel<`,
		`class="btn btn--destructive" id="comment-block-yes">Block<`)
	allowed := map[string]bool{
		"logout-confirm": true, "logout-confirm__body": true, "logout-confirm__title": true,
		"logout-confirm__text": true, "logout-confirm__actions": true, "btn": true, "btn--destructive": true,
	}
	markup := box
	if _, rest, ok := strings.Cut(box, `{{define "comment_block_confirm"}}`); ok {
		markup = rest
	}
	if strings.Contains(markup, "autofocus") {
		t.Error("the Block box carries autofocus; the title takes focus from the script and no button looks chosen")
	}
	for _, chunk := range strings.Split(markup, `class="`)[1:] {
		classes, _, _ := strings.Cut(chunk, `"`)
		for _, name := range strings.Fields(classes) {
			if !allowed[name] {
				t.Errorf("the Block box uses the class %q; it reuses .logout-confirm and adds no alert styling of its own", name)
			}
		}
	}
	if !strings.Contains(index, `{{template "comment_block_confirm"}}`) {
		t.Error("index.html.tmpl does not include the Block box")
	}
	if strings.Contains(index, "<dialog id=\"comment-block-confirm\"") {
		t.Error("index.html.tmpl holds the Block box itself; it lives in comment_block_confirm.html.tmpl")
	}
}

// TestCommentBlockIsAButton (quick 261030-u78, WR-E04, owner 2026-10-21: an
// action is a real button, never an underlined link): comments.js builds the
// Block control as a button and never as an anchor, names the route and holds
// the box words, and both style sheets list .comment__block in the rules that
// list .comment__flag, so it has Flag's look.
func TestCommentBlockIsAButton(t *testing.T) {
	js := stripJSComments(p16ReadStatic(t, "static/js/comments.js"))
	for _, want := range []string{
		"block-author", "'Block'",
		"You will not see their comments. They are not told.",
		"You can unblock them in Settings, Blocked people.",
		"element('button', 'comment__block')",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("comments.js (comments stripped) lacks %q", want)
		}
	}
	// Review CR-01 (2026-10-30): Block is offered only where the name is shown,
	// so the words for a person whose name is not shown are gone.
	for _, gone := range []string{
		"Block this person", "This person is blocked.", "once their username is checked",
	} {
		if strings.Contains(js, gone) {
			t.Errorf("comments.js (comments stripped) still holds %q; a New member comment has no Block", gone)
		}
	}
	if strings.Contains(js, "element('a', 'comment__block')") || strings.Contains(js, `createElement('a')`) {
		t.Error("comments.js builds the Block control as a link; it is a button")
	}
	for _, file := range []string{"static/css/comments.css", "static/css/phone.css"} {
		shared := 0
		for _, r := range commentRules(t, file) {
			heads := strings.Join(p16HeadParts(r.Head), "\n")
			// The Flagged state (.comment__flag--sent) is its own rule and has no
			// Block counterpart; only the base Flag rules are shared.
			if strings.Contains(heads, ".comment__flag:not([hidden])") {
				if !strings.Contains(heads, ".comment__block") {
					t.Errorf("%s: the rule %q lists .comment__flag but not .comment__block", file, r.Head)
				}
				shared++
			}
		}
		if shared == 0 {
			t.Errorf("%s has no rule listing .comment__flag; the walk may have stopped matching", file)
		}
	}
}
