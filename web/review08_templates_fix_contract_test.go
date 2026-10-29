package web

import (
	"os"
	"strings"
	"testing"
)

// Phase 8 code review, templates part (08-REVIEW-templates.md): the pins
// added by the fix round, one test or more per finding.

// TestCommentsHeadingNoRing (WR-T03): after a keyboard Delete, focus moves to
// the Comments heading (tabindex -1, never a Tab stop), and UI-SPEC 7d says it
// draws no ring. Without the rule, :focus-visible draws one.
func TestCommentsHeadingNoRing(t *testing.T) {
	head := ".leaflet-container .report-page .comments .comments__heading:focus"
	r := p16Rule(t, commentRules(t, "static/css/comments.css"), "", head)
	p16MustDecl(t, head, r, map[string]string{"outline": "none"})
}

// TestSharePageNamesItsIcon (WR-T02): the public page names the app icon in
// its head like every other page. With no icon link a browser asks for
// /favicon.ico, a path that is not in the session skip set, and that answer
// would set a cookie the page promises not to set. /icons/app.svg skips it.
func TestSharePageNamesItsIcon(t *testing.T) {
	page := p13ReadTemplate(t, "share_status.html.tmpl")
	head := page
	if i := strings.Index(page, "</head>"); i >= 0 {
		head = page[:i]
	}
	r08tMust(t, "share_status.html.tmpl head", head, `<link rel="icon" href="/icons/app.svg" type="image/svg+xml">`)
}

// TestTrustModelSharingRemovedReport (WR-T06): the public page answers a
// removed report with "This report was removed" (D-06), so TRUST-MODEL's
// Sharing section must not say a removed report answers like an unknown one.
func TestTrustModelSharingRemovedReport(t *testing.T) {
	doc := r08tReadRepo(t, "../docs/TRUST-MODEL.md")
	if strings.Contains(doc, "held, removed or too old report answers like an unknown one") {
		t.Errorf("TRUST-MODEL.md says a removed report answers like an unknown one; it says it was removed")
	}
	r08tMust(t, "TRUST-MODEL.md", doc,
		"token for a held or too old report answers like an unknown one",
		"a removed one says it was removed (D-06)")
}

// TestPolicyCommentCheckKeptForEvery (WR-T04): every checked comment keeps
// the check's verdict, its scores and the model with it, clear or borderline,
// so the Policy must not say only a borderline one does.
//
// Amended 2026-10-29 by review fix round 2 (SK-07): a comment keeps a check
// result only when the check finished on it (comments.go writes the verdict
// only then; with no screener, or a held comment a moderator settled, nothing
// is kept), so the Policy must not say every comment keeps one.
func TestPolicyCommentCheckKeptForEvery(t *testing.T) {
	keeps := r71tPolicySection(t, "What Pinalert keeps")
	if strings.Contains(keeps, "for a borderline one the check") {
		t.Errorf("the Policy says only a borderline comment keeps its check result")
	}
	if strings.Contains(keeps, "For every comment the check's result") {
		t.Errorf("the Policy says every comment keeps a check result; only one the check finished on does")
	}
	r08tMust(t, "the Policy's keeps list", keeps,
		"A comment is checked by the same service as a report. When the check finishes on a comment, its result, its scores and which model gave them are kept with the comment.")
}

// TestPolicyAndReadmeSharedPageFields (WR-T05): the public page also shows
// the severity, the confirmation count in its sentence and, for a Critical
// report with a pick, what is happening. The Policy and the README say so.
func TestPolicyAndReadmeSharedPageFields(t *testing.T) {
	keeps := r71tPolicySection(t, "What Pinalert keeps")
	if strings.Contains(keeps, "only the report's kind, a rough area, its state in one short sentence and when") {
		t.Errorf("the Policy's share line still leaves out the severity, the count and the pick")
	}
	r08tMust(t, "the Policy's share line", keeps,
		"Anyone with the link can see only the report's kind and severity, a rough area, what is happening for a Critical report, its state and how many people confirmed it in one short sentence, and when.")
	readme := strings.Join(strings.Fields(r08tReadRepo(t, "../README.md")), " ")
	r08tMust(t, "README.md", readme,
		"The public page shows only the category and severity, a rough area, what is happening for a Critical report, whether it is still active, disputed, resolved or ended with how many people confirmed it, and when.")
	// The page itself still shows those three, so the words stay true.
	page := p13ReadTemplate(t, "share_status.html.tmpl")
	r08tMust(t, "share_status.html.tmpl", page, "{{.SeverityWord}}", "{{.Happening}}", "{{.Sentence}}")
}

// TestPanelComposerPinnedToPanelBottom (WR-T01): D-02, sketch 008 and UI-SPEC
// 2b pin the laptop box to the bottom of the panel whatever the scroll. The box
// is a child of .comments, and a sticky box only sticks while its parent is on
// screen, so sticky is not enough. The box is fixed to the panel's own left
// edge, width and bottom (the panel popup is itself fixed with no transform,
// map.js clears it), and the popup content keeps a reserve of the box's height
// below the last comment so it stays reachable.
func TestPanelComposerPinnedToPanelBottom(t *testing.T) {
	const media = "min-width: 900px"
	const pane = "html[data-report-panel] .leaflet-container .leaflet-detail-pane"
	rules := commentRules(t, "static/css/report-panel.css")
	popup := p16Rule(t, rules, media, pane+" .leaflet-popup")
	box := p16Rule(t, rules, media, pane+" .report-page .comments .composer:not([hidden])")

	// No composer rule anywhere is sticky: its parent is .comments.
	for _, r := range p16AllCSS(t) {
		if strings.Contains(r.Head, "composer") && r.Decls["position"] == "sticky" {
			t.Errorf("%s: %q is sticky, which sticks only while .comments is on screen", r.File, r.Head)
		}
	}
	if box.Decls["position"] != "fixed" || box.Decls["bottom"] != "0" {
		t.Errorf("the panel box is position %q bottom %q, want fixed at 0", box.Decls["position"], box.Decls["bottom"])
	}
	for _, prop := range []string{"left", "width", "border-right"} {
		if box.Decls[prop] == "" || box.Decls[prop] != popup.Decls[prop] {
			t.Errorf("the panel box %s is %q, want the panel's own %q", prop, box.Decls[prop], popup.Decls[prop])
		}
	}
	if box.Decls["box-sizing"] != "border-box" || popup.Decls["box-sizing"] != "border-box" {
		t.Errorf("the box and the panel must both be border-box so equal widths match")
	}
	if box.Decls["margin"] != "0" {
		t.Errorf("the fixed box has margin %q, want 0 (a negative margin moves it off the panel)", box.Decls["margin"])
	}
	// The popup is the containing block's stand in: it is fixed at the top
	// left, so a transform on it would trap the box (map.js clears it).
	if popup.Decls["position"] != "fixed" || popup.Decls["transform"] != "" {
		t.Errorf("the panel popup is position %q transform %q", popup.Decls["position"], popup.Decls["transform"])
	}

	// The reserve: the content keeps the box's height free at the end of the
	// scroll, only while a box is shown, and one row more for Replying to.
	height := p16Rule(t, rules, media, pane+" .leaflet-popup").Decls["--panel-composer-height"]
	for _, part := range []string{"1px", "var(--space-12)", "var(--space-md)", "var(--line-height-body)", "var(--font-size-body)"} {
		if !strings.Contains(height, part) {
			t.Errorf("--panel-composer-height %q lacks %s, a part of the box's own height", height, part)
		}
	}
	if !strings.Contains(box.Decls["padding"], "var(--space-12)") || !strings.HasSuffix(box.Decls["padding"], "var(--space-md)") {
		t.Errorf("the box padding %q no longer matches the reserve's parts", box.Decls["padding"])
	}
	reserve := p16Rule(t, rules, media, pane+" .leaflet-popup-content:has(.composer:not([hidden]))")
	if reserve.Decls["padding-bottom"] != "var(--panel-composer-height)" {
		t.Errorf("the content reserve is %q, want var(--panel-composer-height)", reserve.Decls["padding-bottom"])
	}
	replying := p16Rule(t, rules, media, pane+" .leaflet-popup-content:has(.composer__replying:not([hidden]))")
	if replying.Decls["padding-bottom"] != "calc(var(--panel-composer-height) + var(--touch-target-min))" {
		t.Errorf("the content reserve while replying is %q", replying.Decls["padding-bottom"])
	}
}

// TestShareButtonsSideBySideAt320 (IN-T03): UI-SPEC 5a says the phone stacks
// nothing, so WhatsApp and Copy link share one row at 320 wide. Each button
// is half the 288px row less half the 8px gap, and it never shrinks below its
// words (min-width max-content). The widest words, Link copied, measured 91px
// at body size in Chromium and WebKit; with the 20px glyph, the 8px gap, the
// 1px .btn border on each side and the side padding they must fit in 140px.
func TestShareButtonsSideBySideAt320(t *testing.T) {
	const row, gap, glyph, border, words = 288, 8, 20, 1, 91
	rules := commentRules(t, "static/css/share.css")
	btn := p16Rule(t, rules, "", ".share-window__whatsapp:not([hidden]), .share-window__copy:not([hidden])")
	if btn.Decls["min-width"] != "max-content" {
		t.Fatalf("the Share buttons min-width is %q; the sum below assumes max-content", btn.Decls["min-width"])
	}
	tokens := map[string]int{"var(--space-xs)": 4, "var(--space-sm)": 8, "var(--space-12)": 12, "var(--space-md)": 16, "var(--space-lg)": 24}
	side := -1
	for _, prop := range []string{"padding-left", "padding-right"} {
		v, ok := tokens[btn.Decls[prop]]
		if !ok {
			t.Fatalf("the Share buttons %s is %q, want a space token (else .btn's 24px sides apply)", prop, btn.Decls[prop])
		}
		if v > side {
			side = v
		}
	}
	need := 2*side + 2*border + glyph + gap + words
	if half := (row - gap) / 2; need > half {
		t.Errorf("Copy link needs %dpx at 320 wide but has %dpx, so the two buttons stack", need, half)
	}
	if btn.Decls["min-height"] != "var(--touch-target-min)" {
		t.Errorf("the Share buttons lost their 44px height")
	}
}

// r08tMutedOnField names the muted lines that sit on the composer field's own
// --color-bg, not on the tinted phone sheet, so they need no phone override.
var r08tMutedOnField = map[string]string{
	".composer__input::placeholder":         "the placeholder is inside the field, on its --color-bg",
	`.composer__post[aria-disabled="true"]`: "the off Post is inside the field, on its --color-bg",
}

// r08tLastCompound returns the last compound selector of a head part, with
// :not([hidden]) dropped so a phone head and a base head compare equal.
func r08tLastCompound(part string) string {
	fields := strings.Fields(part)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimSuffix(fields[len(fields)-1], ":not([hidden])")
}

// TestPhoneSheetHasNoMutedText (WR-T07, P8-R8): the phone report sheet is
// severity tinted, and muted text sits only on --color-bg. Every comments.css
// and report-page.css line drawn in --color-text-muted therefore has a rule in
// phone.css's narrow block that switches the same element to --color-text (or
// does not draw it), unless it sits on the field's own --color-bg (r08tMutedOnField). A new muted
// line with no phone override fails here.
func TestPhoneSheetHasNoMutedText(t *testing.T) {
	overridden := map[string]bool{}
	for _, r := range commentRules(t, "static/css/phone.css") {
		// A line the phone does not draw at all (display none) needs no colour.
		if r.Media != "@media (max-width: 899.98px)" || (r.Decls["color"] != "var(--color-text)" && r.Decls["display"] != "none") {
			continue
		}
		for _, part := range p16HeadParts(r.Head) {
			if strings.Contains(part, ".report-page") {
				overridden[r08tLastCompound(part)] = true
			}
		}
	}
	checked := 0
	for _, file := range []string{"static/css/comments.css", "static/css/report-page.css"} {
		for _, r := range commentRules(t, file) {
			if r.Media != "" || r.Decls["color"] != "var(--color-text-muted)" {
				continue
			}
			for _, part := range p16HeadParts(r.Head) {
				key := r08tLastCompound(part)
				checked++
				if _, ok := r08tMutedOnField[key]; ok {
					continue
				}
				if !overridden[key] {
					t.Errorf("%s: %q is muted text with no --color-text override in phone.css's narrow block, so it renders muted on the tinted sheet", file, part)
				}
			}
		}
	}
	if checked < 10 {
		t.Errorf("only %d muted heads were checked; the walk may have stopped matching", checked)
	}
	for key := range r08tMutedOnField {
		if overridden[key] {
			t.Errorf("%s is allow listed as on the field but phone.css overrides it; drop the entry", key)
		}
	}
}

// r08tReadRepo reads a file of the repository, relative to web/.
func r08tReadRepo(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// r08tMust fails when text lacks any of the needles.
func r08tMust(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(text, n) {
			t.Errorf("%s lacks %q", label, n)
		}
	}
}
