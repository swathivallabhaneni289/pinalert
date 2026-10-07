package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// This file holds the static contract tests of plan 03-13, the shared trust
// block. Every helper carries the p13 prefix so a later plan can add its own
// without a name clash. Nothing here needs a database or a server: the tests
// read the embedded static and template trees and inspect the shipped text.

// p13ReadStatic returns one embedded static file as text.
func p13ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return string(raw)
}

// p13ReadTemplate returns one embedded template as text.
func p13ReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("failed to read embedded templates/%s: %v", name, err)
	}
	return string(raw)
}

// p13CodeLines returns text without the lines whose trimmed form starts with
// a double slash, so a comment naming a call cannot change an order or a
// count.
func p13CodeLines(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// p13Ordered requires every needle to be present in text and the first
// occurrences to be strictly increasing.
func p13Ordered(t *testing.T, text, label string, needles ...string) {
	t.Helper()
	prev := -1
	prevNeedle := ""
	for _, needle := range needles {
		idx := strings.Index(text, needle)
		if idx == -1 {
			t.Errorf("%s: %q is missing", label, needle)
			return
		}
		if idx <= prev {
			t.Errorf("%s: %q must come after %q", label, needle, prevNeedle)
			return
		}
		prev = idx
		prevNeedle = needle
	}
}

// p13MustContain fails once per missing needle.
func p13MustContain(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			t.Errorf("%s: missing %q", label, needle)
		}
	}
}

// p13MustNotContain fails once per present needle.
func p13MustNotContain(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			t.Errorf("%s: must not contain %q", label, needle)
		}
	}
}

// p13MustMatch fails when the pattern does not match.
func p13MustMatch(t *testing.T, label, text, pattern string) {
	t.Helper()
	if !regexp.MustCompile(pattern).MatchString(text) {
		t.Errorf("%s: no match for %s", label, pattern)
	}
}

// TestTrustModuleLoadsBeforeItsConsumers pins the script order (V-38): the
// trust module sits after the visibility module and before the map, modal
// and feed modules that call it, deferred and versioned like its siblings.
func TestTrustModuleLoadsBeforeItsConsumers(t *testing.T) {
	html := p13ReadTemplate(t, "index.html.tmpl")

	const src = "/static/js/trust.js"
	if n := strings.Count(html, src); n != 1 {
		t.Fatalf("index.html.tmpl references %s %d times, want exactly 1", src, n)
	}
	window := findTagWindow(t, html, src)
	p13MustContain(t, "trust.js tag", window, " defer", "?v={{.AssetVersion}}")

	p13Ordered(t, html, "app module script order",
		"/static/js/app.js",
		"/static/js/votes.js",
		"/static/js/visibility.js",
		"/static/js/trust.js",
		"/static/js/map.js",
		"/static/js/modal.js",
		"/static/js/feed.js",
		"/static/js/account-menu.js",
	)

	lastVendor := vendorScriptSources[len(vendorScriptSources)-1]
	vendorIdx := strings.Index(html, lastVendor)
	if vendorIdx == -1 {
		t.Fatalf("could not locate the last vendor script %q", lastVendor)
	}
	if strings.Index(html, src) <= vendorIdx {
		t.Errorf("trust.js must load after the vendor bridge %q", lastVendor)
	}
}

// TestTrustBlockMountedOnBothSurfaces pins the one builder, the mount order
// and the discipline of the consumers (V-38 feed and popup, D-20, D-09,
// D-14, A33).
func TestTrustBlockMountedOnBothSurfaces(t *testing.T) {
	feed := stripCSSComments(p13ReadStatic(t, "static/js/feed.js"))
	mapJS := stripCSSComments(p13ReadStatic(t, "static/js/map.js"))
	trust := p13ReadStatic(t, "static/js/trust.js")

	for label, text := range map[string]string{"feed.js": feed, "map.js": mapJS} {
		p13MustContain(t, label, text,
			"PinalertTrust.createTrustBlock(", "PinalertTrust.updateTrustBlock(")
	}

	p13MustContain(t, "feed.js", feed, "PinalertTrust.createTrustBlock()")
	p13MustNotContain(t, "feed.js", feed, "showScores")
	p13MustMatch(t, "map.js", mapJS,
		`PinalertTrust\.createTrustBlock\(\s*\{\s*showScores:\s*true\s*\}\s*\)`)

	createRow := jsFunctionBody(t, feed, "createRow(id)")
	p13Ordered(t, createRow, "feed.js createRow",
		"PinalertVisibility.createVisibilityTag(",
		"PinalertTrust.createTrustBlock(",
		"PinalertVotes.createVoteBlock(",
	)
	p13Ordered(t, p13CodeLines(mapJS), "map.js popup builder",
		"PinalertVisibility.createVisibilityTag(",
		"PinalertTrust.createTrustBlock(",
		"PinalertVotes.createVoteBlock(",
	)

	updateRow := jsFunctionBody(t, feed, "updateRow(row, report)")
	p13MustContain(t, "feed.js updateRow", updateRow,
		"PinalertTrust.updateTrustBlock(row.trust, report.trust)")

	for label, text := range map[string]string{"feed.js": feed, "map.js": mapJS} {
		p13MustNotContain(t, label, text,
			"'trust-", "\"trust-",
			"trust-block", "trust-num", "trust-signal", "trust-icon",
			"confirmed_places", "disputed_places", "confirmed_text", "disputed_text",
			"lead_sentence", "fade_stage", "score_sentence", ".currency", ".reporter",
		)
	}

	sortBody := jsFunctionBody(t, feed, "sortReports(reports)")
	if strings.Contains(sortBody, "trust") {
		t.Errorf("sortReports reads trust, but the block never reorders a report (D-14)")
	}

	p13MustContain(t, "trust.js exports", trust,
		"window.PinalertTrust = {",
		"createTrustBlock: createTrustBlock",
		"updateTrustBlock: updateTrustBlock",
		"fadeStage: fadeStage",
		"focusToggle: focusToggle",
	)

	create := jsFunctionBody(t, trust, "createTrustBlock(options)")
	p13MustContain(t, "createTrustBlock", create, "'trust-panel-'")
	if regexp.MustCompile(`\breport\b`).MatchString(p13CodeLines(create)) {
		t.Errorf("createTrustBlock names a report, so a panel id could come from the report id")
	}

	p13MustContain(t, "trust.js", trust,
		". Shows what these mean.",
		", score",
		"visually-hidden",
		"aria-hidden",
		"trust-icon trust-icon--clock",
		"trust-icon trust-icon--user",
		"trust-num",
		"trust-block__count--second",
		"trust-signal--second",
		"trust-block__panel",
		"trust-block__line",
	)
	p13MustMatch(t, "trust.js PANEL_LINES", trust,
		`PANEL_LINES\s*=\s*\[\s*'lead',\s*'confirmed',\s*'disputed',\s*'currency',\s*'currency_score',\s*'reporter',\s*'reporter_score'\s*\]`)

	panel := jsFunctionBody(t, trust, "renderTrustPanel(block, trust)")
	p13MustContain(t, "renderTrustPanel", panel,
		"showScores", "lead_sentence", "confirmed_sentence", "disputed_sentence", "score_sentence")

	update := jsFunctionBody(t, trust, "updateTrustBlock(block, trust)")
	p13MustContain(t, "updateTrustBlock", update, "validTrust(", "hidden")

	// The open state is never reset by an update, so a panel stays open
	// across every poll.
	for _, name := range []string{
		"updateTrustBlock(block, trust)",
		"renderTrustCounts(block, trust)",
		"renderTrustSignal(signal, shown, label, score)",
		"renderTrustPanel(block, trust)",
	} {
		body := jsFunctionBody(t, trust, name)
		p13MustNotContain(t, name, body, "aria-expanded", "setPanelOpen(")
	}

	counts := jsFunctionBody(t, trust, "renderTrustCounts(block, trust)")
	p13MustContain(t, "renderTrustCounts", counts, "confirmed_text", "disputed_text")
	p13MustNotContain(t, "renderTrustCounts", counts, "=== 0", "!== 0", "> 0")

	if regexp.MustCompile(`trust\.[a-z_]+\s*=[^=]`).MatchString(trust) {
		t.Errorf("trust.js assigns to a trust field, but the browser never changes a count (A33)")
	}
}

// TestFadeUsesCombinedStage proves the browser holds no fade threshold of its
// own: every fade site asks the trust module for the worse of the time stage
// and the server stage (V-37, D-07, R-02).
func TestFadeUsesCombinedStage(t *testing.T) {
	feed := stripCSSComments(p13ReadStatic(t, "static/js/feed.js"))
	mapJS := stripCSSComments(p13ReadStatic(t, "static/js/map.js"))
	trust := p13ReadStatic(t, "static/js/trust.js")

	for label, text := range map[string]string{"feed.js": feed, "map.js": mapJS} {
		if strings.Contains(text, "Pinalert.ageStage(") {
			t.Errorf("%s calls the time only stage directly", label)
		}
	}

	sites := []struct{ label, text, fn string }{
		{"feed.js updateRow", feed, "updateRow(row, report)"},
		{"feed.js refreshAgeAndTime", feed, "refreshAgeAndTime()"},
		{"map.js buildBadgeElement", mapJS, "buildBadgeElement(report)"},
		{"map.js popupStateClasses", mapJS, "popupStateClasses(report)"},
		{"map.js syncPopupState", mapJS, "syncPopupState(marker, report)"},
	}
	for _, s := range sites {
		body := jsFunctionBody(t, s.text, s.fn)
		if !strings.Contains(body, "PinalertTrust.fadeStage(report)") {
			t.Errorf("%s does not call PinalertTrust.fadeStage(report)", s.label)
		}
	}

	code := p13CodeLines(trust)
	if n := strings.Count(code, "Pinalert.ageStage("); n != 1 {
		t.Errorf("trust.js calls Pinalert.ageStage( %d times on code lines, want exactly 1", n)
	}
	fade := jsFunctionBody(t, code, "fadeStage(report)")
	p13MustContain(t, "fadeStage", fade, "Pinalert.ageStage(", "fade_stage", "FADE_STAGES", "indexOf")
	p13MustMatch(t, "trust.js FADE_STAGES", trust, `FADE_STAGES\s*=\s*\[\s*'fresh',\s*'aging',\s*'stale'\s*\]`)
}

// TestTrustModuleHasNoThresholdsOrSinks keeps the module a pure renderer
// (R-02, T-03-06, A69): no markup sink, no network, no date arithmetic, no
// server copy, no threshold and no long dash.
func TestTrustModuleHasNoThresholdsOrSinks(t *testing.T) {
	trust := p13ReadStatic(t, "static/js/trust.js")

	// The two long dash code points are written as escapes so this test file
	// stays dash free.
	if strings.ContainsRune(trust, '\u2014') || strings.ContainsRune(trust, '\u2013') {
		t.Errorf("trust.js contains a long dash (A69)")
	}

	p13MustNotContain(t, "trust.js sinks", trust,
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write",
		"DOMParser", "createContextualFragment")
	p13MustNotContain(t, "trust.js network and dates", trust,
		"fetch(", "XMLHttpRequest", "Date.now", "new Date", "getTime", "created_at", "expires_at")
	p13MustNotContain(t, "trust.js server copy", trust,
		"Up to date", "Getting old", "re-confirming", "Too early", "Reliable reporter",
		"Mixed record", "Unreliable reporter", "New reporter", "Counts separate places",
		"out of 100", "Unconfirmed", "confirmations before")
	p13MustContain(t, "trust.js", trust, "Pinalert.setText(", "'use strict'")
	p13MustMatch(t, "trust.js REPORTER_TAGS", trust,
		`REPORTER_TAGS\s*=\s*\[\s*'reliable',\s*'mixed',\s*'unreliable',\s*'new',\s*'none'\s*\]`)
	p13MustMatch(t, "trust.js CURRENCY_BANDS", trust,
		`CURRENCY_BANDS\s*=\s*\[\s*'up_to_date',\s*'getting_old',\s*'needs_reconfirming',\s*'not_yet_corroborated',\s*'not_applicable'\s*\]`)
	p13MustNotContain(t, "trust.js row state classes", trust, "'vis-'", "'sev-'", "'age-'")

	code := p13CodeLines(trust)
	if regexp.MustCompile(`(?i)threshold|cutoff|min_?score|half_?life|window_minutes|recent_places|judged_reports|corroborated_reports`).MatchString(code) {
		t.Errorf("trust.js names a threshold or a server fact field on a code line")
	}
	if loc := regexp.MustCompile(`\b(?:[2-9]|[1-9][0-9]+)\b`).FindString(code); loc != "" {
		t.Errorf("trust.js has the numeric literal %q on a code line, only 0, 1 and -1 may appear", loc)
	}
}

// TestTrustDisclosure pins the keyboard and click behaviour of the toggle and
// the one Escape listener (V-39, renamed per A5). Plan 03-19 later extends
// the listener with the popup branches and must keep the single document
// listener.
func TestTrustDisclosure(t *testing.T) {
	trust := p13ReadStatic(t, "static/js/trust.js")
	code := p13CodeLines(trust)

	p13MustContain(t, "trust.js code", code,
		"aria-expanded", "aria-controls", "addEventListener('click'",
		"'Enter'", "' '", "'Escape'", "stopPropagation", "preventDefault")

	create := jsFunctionBody(t, trust, "createTrustBlock(options)")
	p13MustContain(t, "createTrustBlock", create, "'button'", "stopPropagation", "'Enter'")
	p13MustNotContain(t, "createTrustBlock", create, "preventDefault")

	const listener = "document.addEventListener("
	if n := strings.Count(code, listener); n != 1 {
		t.Fatalf("trust.js has %d %s calls on code lines, want exactly 1 (A23)", n, listener)
	}
	for _, line := range strings.Split(code, "\n") {
		if strings.Contains(line, listener) {
			p13MustContain(t, "the document listener statement", line, "'keydown'", "onDocumentEscape", ", true)")
		}
	}

	escape := jsFunctionBody(t, trust, "onDocumentEscape(evt)")
	p13MustContain(t, "onDocumentEscape", escape, "'Escape'", "closeOneLevel()")
	p13Ordered(t, escape, "onDocumentEscape", "if (!acted)", "stopPropagation()")
	p13Ordered(t, escape, "onDocumentEscape", "if (!acted)", "preventDefault()")
	if n := strings.Count(escape, "stopPropagation()"); n != 1 {
		t.Errorf("onDocumentEscape calls stopPropagation() %d times, want exactly 1", n)
	}

	one := jsFunctionBody(t, trust, "closeOneLevel()")
	p13Ordered(t, one, "closeOneLevel", "'report-modal'", "'account-menu'",
		"PinalertVotes.cancelResolveConfirm(", "closeRowPanel(")
	p13MustContain(t, "closeOneLevel", one, "surfaceIsDisplayed(")

	closeRow := jsFunctionBody(t, trust, "closeRowPanel(row)")
	p13MustContain(t, "closeRowPanel", closeRow, "aria-expanded", "setPanelOpen(", ".focus()")

	displayed := jsFunctionBody(t, trust, "surfaceIsDisplayed(el)")
	p13MustContain(t, "surfaceIsDisplayed", displayed, "getClientRects")

	// A5: the hover, title and tooltip design is dropped, and there is no
	// live region because the 30 second poll would re-announce it.
	p13MustNotContain(t, "trust.js", trust,
		"aria-live", "tooltip", "mouseenter", "mouseover", "'title'", ".title")
}

// p13CSSRule is one parsed rule: the at-rule prelude it sits in (empty at top
// level), its whitespace normalised head and its declarations.
type p13CSSRule struct {
	Media string
	Head  string
	Decls map[string]string
}

// p13ParseCSS parses a stylesheet into rules, one level of at-rule nesting
// deep. Every rule inside a block opened by an at-rule carries that at-rule's
// prelude as its Media.
func p13ParseCSS(raw string) []p13CSSRule {
	var rules []p13CSSRule
	var walk func(s, media string)
	walk = func(s, media string) {
		for {
			open := strings.Index(s, "{")
			if open == -1 {
				return
			}
			prelude := strings.Join(strings.Fields(s[:open]), " ")
			depth := 1
			i := open + 1
			for ; i < len(s) && depth > 0; i++ {
				switch s[i] {
				case '{':
					depth++
				case '}':
					depth--
				}
			}
			body := s[open+1 : i-1]
			if strings.HasPrefix(prelude, "@") {
				walk(body, prelude)
			} else {
				rules = append(rules, p13CSSRule{Media: media, Head: prelude, Decls: declsOf(body)})
			}
			s = s[i:]
		}
	}
	walk(stripCSSComments(raw), "")
	return rules
}

// p13Rule returns the rule whose head equals head exactly and whose Media
// contains media (an empty media means top level only), with its index in
// parse order. It fails the test when no rule matches.
func p13Rule(t *testing.T, rules []p13CSSRule, media, head string) (p13CSSRule, int) {
	t.Helper()
	for i, r := range rules {
		if r.Head != head {
			continue
		}
		if media == "" && r.Media == "" {
			return r, i
		}
		if media != "" && strings.Contains(r.Media, media) {
			return r, i
		}
	}
	t.Fatalf("no rule with head %q in media %q", head, media)
	return p13CSSRule{}, -1
}

// TestTrustCSS pins the look of the trust block (V-40): geometry from tokens,
// the normal text colour only, the hidden guard on every display rule, the
// press fill, the inset ring, the glyph masks, and the forced colours and
// reduced motion rules.
func TestTrustCSS(t *testing.T) {
	raw := p13ReadStatic(t, "static/css/trust.css")
	rules := p13ParseCSS(raw)

	isTrust := func(r p13CSSRule) bool { return strings.Contains(r.Head, ".trust-") }

	// 1. Every display declaration carries the hidden guard.
	for _, r := range rules {
		if !isTrust(r) {
			continue
		}
		if _, ok := r.Decls["display"]; !ok {
			continue
		}
		for _, part := range strings.Split(r.Head, ",") {
			if !strings.Contains(part, ":not([hidden])") {
				t.Errorf("%q declares display without the :not([hidden]) guard", strings.TrimSpace(part))
			}
		}
	}

	// 2 and 3. Tokens, type sizes and weights, colour, no severity colour.
	allowedSizes := map[string]bool{
		"var(--font-size-label)": true, "var(--font-size-body)": true,
		"var(--font-size-heading)": true, "var(--font-size-display)": true,
	}
	allowedWeights := map[string]bool{
		"var(--font-weight-regular)": true, "var(--font-weight-semibold)": true,
	}
	for _, r := range rules {
		if !isTrust(r) {
			continue
		}
		if strings.Contains(r.Head, ".icon-glyph--") {
			t.Errorf("%q uses the category glyph prefix (A39)", r.Head)
		}
		for prop, value := range r.Decls {
			if strings.Contains(value, "!important") || strings.Contains(value, "999px") {
				t.Errorf("%q %s: %q uses !important or 999px", r.Head, prop, value)
			}
			for _, banned := range []string{"--color-text-muted", "--color-severity", "--severity-"} {
				if strings.Contains(value, banned) {
					t.Errorf("%q %s: %q uses %s, but colour is reserved for severity (A3, D-18)", r.Head, prop, value, banned)
				}
			}
			switch prop {
			case "font-size":
				if !allowedSizes[value] {
					t.Errorf("%q font-size %q is not one of the four type sizes", r.Head, value)
				}
			case "font-weight":
				if !allowedWeights[value] {
					t.Errorf("%q font-weight %q is not one of the two weights", r.Head, value)
				}
			case "color":
				if value != "var(--color-text)" {
					t.Errorf("%q color %q is not the normal text colour", r.Head, value)
				}
			}
		}
	}

	// 4. Exact heads and values, in source order.
	maskBase := map[string]string{
		"-webkit-mask-size": "contain", "mask-size": "contain",
		"-webkit-mask-repeat": "no-repeat", "mask-repeat": "no-repeat",
		"-webkit-mask-position": "center", "mask-position": "center",
	}
	iconBase := map[string]string{
		"display": "block", "flex": "none", "width": "16px", "height": "20px",
		"background-color": "currentColor",
	}
	for k, v := range maskBase {
		iconBase[k] = v
	}
	table := []struct {
		head string
		want map[string]string
	}{
		{".trust-block:not([hidden])", map[string]string{
			"display": "block", "margin-top": "var(--space-sm)",
			"border-top": "1px solid var(--color-border)"}},
		{".trust-block__toggle:not([hidden])", map[string]string{
			"display": "block", "width": "100%", "min-height": "var(--touch-target-min)",
			"padding": "var(--space-sm) var(--space-xs)", "border-radius": "8px",
			"background": "transparent", "color": "var(--color-text)",
			"font-family": "var(--font-family)", "font-size": "var(--font-size-body)",
			"text-align": "start", "text-underline-offset": "4px",
			"transition": "background-color 150ms ease"}},
		{".trust-block__toggle:hover", map[string]string{"text-decoration": "underline"}},
		{".trust-block__toggle:active", map[string]string{"background-color": "var(--color-border)"}},
		{".trust-block__toggle:focus-visible", map[string]string{"outline-offset": "-2px"}},
		{".trust-block__counts:not([hidden])", map[string]string{
			"display": "flex", "flex-wrap": "wrap", "align-items": "center",
			"column-gap": "var(--space-lg)", "min-height": "32px", "overflow": "hidden",
			"font-size": "var(--font-size-body)", "line-height": "var(--line-height-body)"}},
		{".trust-block__count", map[string]string{
			"position": "relative", "min-width": "0", "overflow-wrap": "anywhere"}},
		{".trust-block__count--second::before", map[string]string{
			"content": `""`, "position": "absolute", "top": "var(--space-sm)",
			"bottom": "var(--space-sm)", "left": "calc(-0.5 * var(--space-lg))",
			"width": "1px", "background-color": "var(--color-border)"}},
		{".trust-num", map[string]string{
			"font-weight": "var(--font-weight-semibold)", "color": "var(--color-text)"}},
		{".trust-block__signals:not([hidden])", map[string]string{
			"display": "flex", "flex-wrap": "wrap", "align-items": "center",
			"column-gap": "var(--space-md)", "min-height": "28px", "overflow": "hidden",
			"font-size": "var(--font-size-label)", "line-height": "var(--line-height-label)",
			"font-weight": "var(--font-weight-regular)", "color": "var(--color-text)"}},
		{".trust-signal:not([hidden])", map[string]string{
			"display": "flex", "position": "relative", "align-items": "center",
			"gap": "var(--space-sm)", "min-width": "0"}},
		{".trust-signal--second::before", map[string]string{
			"content": `""`, "position": "absolute", "top": "var(--space-xs)",
			"bottom": "var(--space-xs)", "left": "calc(-1 * var(--space-sm))",
			"width": "1px", "background-color": "var(--color-border)"}},
		{".trust-signal__text", map[string]string{
			"min-width": "0", "overflow-wrap": "anywhere"}},
		{".trust-icon:not([hidden])", iconBase},
		{".trust-icon--clock", map[string]string{
			"-webkit-mask-image": `url("/static/icons/clock.svg")`,
			"mask-image":         `url("/static/icons/clock.svg")`}},
		{".trust-icon--user", map[string]string{
			"-webkit-mask-image": `url("/static/icons/user.svg")`,
			"mask-image":         `url("/static/icons/user.svg")`}},
		{".trust-block__panel:not([hidden])", map[string]string{
			"display": "grid", "gap": "var(--space-xs)",
			"padding": "0 var(--space-xs) var(--space-sm)", "margin": "0"}},
		{".trust-block__panel .trust-block__line", map[string]string{
			"margin": "0", "font-size": "var(--font-size-label)",
			"line-height": "var(--line-height-label)",
			"font-weight": "var(--font-weight-regular)", "color": "var(--color-text)",
			"overflow-wrap": "anywhere"}},
	}
	index := map[string]int{}
	for _, row := range table {
		r, i := p13Rule(t, rules, "", row.head)
		index[row.head] = i
		for prop, want := range row.want {
			if got, ok := r.Decls[prop]; !ok || got != want {
				t.Errorf("%q %s = %q, want %q", row.head, prop, got, want)
			}
		}
	}
	hover, _ := p13Rule(t, rules, "", ".trust-block__toggle:hover")
	if _, ok := hover.Decls["text-decoration-thickness"]; ok {
		t.Errorf("the toggle hover rule sets text-decoration-thickness, which the spec leaves out")
	}

	// 4a. The press fill ties the base toggle rule on specificity, so it must
	// come later in the file.
	if index[".trust-block__toggle:active"] <= index[".trust-block__toggle:not([hidden])"] {
		t.Errorf("the :active rule must come after the base toggle rule, or the press fill never shows")
	}

	// 5. Reduced motion and forced colours.
	motion, motionIdx := p13Rule(t, rules, "prefers-reduced-motion: reduce", ".trust-block__toggle:not([hidden])")
	if motion.Decls["transition"] != "none" {
		t.Errorf("reduced motion toggle transition = %q, want none", motion.Decls["transition"])
	}
	if motionIdx <= index[".trust-block__toggle:not([hidden])"] {
		t.Errorf("the reduced motion rule must come after the base toggle rule")
	}
	icon, iconIdx := p13Rule(t, rules, "forced-colors: active", ".trust-icon:not([hidden])")
	if icon.Decls["forced-color-adjust"] != "none" || icon.Decls["background-color"] != "CanvasText" {
		t.Errorf("forced colours icon rule = %v, want forced-color-adjust none and CanvasText", icon.Decls)
	}
	if iconIdx <= index[".trust-icon:not([hidden])"] {
		t.Errorf("the forced colours icon rule must come after the base icon rule")
	}
	sep, sepIdx := p13Rule(t, rules, "forced-colors: active",
		".trust-block__count--second::before, .trust-signal--second::before")
	if sep.Decls["forced-color-adjust"] != "none" || sep.Decls["background-color"] != "CanvasText" {
		t.Errorf("forced colours separator rule = %v, want forced-color-adjust none and CanvasText", sep.Decls)
	}
	if sepIdx <= index[".trust-signal--second::before"] {
		t.Errorf("the forced colours separator rule must come after the base separator rules")
	}

	// 6. System colour keywords stay inside forced colours blocks.
	system := regexp.MustCompile(`\b(Canvas|CanvasText|GrayText|Highlight|HighlightText|ButtonFace|ButtonText|LinkText)\b`)
	for _, r := range rules {
		if strings.Contains(r.Media, "forced-colors: active") {
			continue
		}
		for prop, value := range r.Decls {
			if system.MatchString(value) {
				t.Errorf("%q %s: %q uses a system colour outside a forced colours block", r.Head, prop, value)
			}
		}
	}

	// 7. Declarations and content values carry no long dash (A69). Comments
	// are stripped first, so the older comment dashes are not scanned.
	stripped := stripCSSComments(raw)
	if strings.ContainsRune(stripped, '\u2014') || strings.ContainsRune(stripped, '\u2013') {
		t.Errorf("trust.css has a long dash outside its comments (A69)")
	}
}
