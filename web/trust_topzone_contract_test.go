package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds plan 03-23's contract for the reporter words, the state
// line, the placement calls and the empty line rule (A75, A76, A80 to A83). It
// reuses the p13 helpers of trust_contract_test.go; every helper this file
// declares would start with p23. The tests read static text and declared CSS
// only: the browser proof is run by the orchestrator after the merge.

// p23RetiredClasses are the names plan 03-23 removed. They are built from
// parts so this file holds none of them whole.
var p23RetiredClasses = []string{
	"trust-icon--" + "user",
	"trust-signal--" + "second",
}

// p23UserGlyph is the file name the trust block no longer masks.
var p23UserGlyph = "user" + ".svg"

// p23CodeOf reads one embedded script with its block comments and its whole
// line comments removed, so a comment naming a call cannot count.
func p23CodeOf(t *testing.T, path string) string {
	t.Helper()
	return p13CodeLines(stripCSSComments(p13ReadStatic(t, path)))
}

// TestTrustStateLineMarkup pins the reporter words, the one placement helper,
// the invalid trust branch and the three placement calls (A75, A76, A80, A82,
// D-18, D-20, T-03-06, T-03-23-01, T-03-23-03).
func TestTrustStateLineMarkup(t *testing.T) {
	trust := p13ReadStatic(t, "static/js/trust.js")
	code := p13CodeLines(trust)

	p13MustContain(t, "trust.js", trust,
		"function createReporterWords()",
		"function createStateLine(className, tag, block)",
		"createStateLine: createStateLine",
		"trust-icon trust-icon--clock",
	)
	p13MustNotContain(t, "trust.js", trust, p23RetiredClasses...)

	// Plan 03-31 (A89): the reporter words carry no score number any more.
	words := jsFunctionBody(t, code, "createReporterWords()")
	p13MustContain(t, "createReporterWords", words,
		"'trust-reporter'",
		"el.hidden = true",
		"return { el: el, word: word };",
	)
	p13MustNotContain(t, "createReporterWords", words,
		"trust-icon", "trust-num", "trust-signal", "aria-hidden", "'button'", "tabindex",
		"'visually-hidden'", "score",
	)

	create := jsFunctionBody(t, code, "createTrustBlock()")
	p13MustContain(t, "createTrustBlock", create, "createReporterWords()", "reporter: reporter")
	p13MustNotContain(t, "createTrustBlock", create, "reporter.el", "createStateLine(")

	line := jsFunctionBody(t, code, "createStateLine(className, tag, block)")
	p13Ordered(t, line, "createStateLine",
		"document.createElement('div')",
		"line.className = className",
		"line.appendChild(tag)",
		"line.appendChild(document.createTextNode(' '))",
		"line.appendChild(block.reporter.el)",
		"return line",
	)
	p13MustNotContain(t, "createStateLine", line, "Pinalert.setText(", "innerHTML", ".hidden")

	update := jsFunctionBody(t, code, "updateTrustBlock(block, trust)")
	start := strings.Index(update, "if (!validTrust(trust)) {")
	if start == -1 {
		t.Fatalf("updateTrustBlock has no invalid trust branch")
	}
	branch := update[start:]
	end := strings.Index(branch, "return;")
	if end == -1 {
		t.Fatalf("the invalid trust branch of updateTrustBlock does not return")
	}
	p13MustContain(t, "updateTrustBlock invalid branch", branch[:end],
		"block.el.hidden = true", "block.reporter.el.hidden = true")
	p13MustContain(t, "updateTrustBlock", update,
		"block.signals.hidden = !state.currency;", "renderTrustSignal(block.reporter,")
	p13MustNotContain(t, "updateTrustBlock", update, "state.currency || state.reporter")

	feed := p23CodeOf(t, "static/js/feed.js")
	// Plan 03.1-09 (R4, R5): the list row builds the trust line, not the
	// block, and no vote block. The old literals were createTrustBlock() and
	// PinalertVotes.createVoteBlock( after body.appendChild(trust.el). The
	// state line still takes the visibility tag and the reporter words, and
	// feed.js still has exactly one createStateLine call.
	createRow := jsFunctionBody(t, feed, "createRow(id)")
	p13Ordered(t, createRow, "feed.js createRow",
		"PinalertVisibility.createVisibilityTag(",
		"PinalertTrust.createTrustLine()",
		"body.appendChild(PinalertTrust.createStateLine('report-row__stateline', tag, trust))",
		"body.appendChild(trust.el)",
	)
	p13MustNotContain(t, "feed.js createRow", createRow, "PinalertVotes.createVoteBlock(")
	p13MustNotContain(t, "feed.js createRow", createRow, "body.appendChild(tag)")
	p13MustNotContain(t, "feed.js updateRow", jsFunctionBody(t, feed, "updateRow(row, report)"), "createStateLine(")

	mapJS := p23CodeOf(t, "static/js/map.js")
	popup := jsFunctionBody(t, mapJS, "buildPopupCard(report)")
	p13Ordered(t, popup, "map.js buildPopupCard",
		"header.className = 'popup-card__header'",
		"pills.className = 'popup-card__pills'",
		"pills.appendChild(badge)",
		"pills.appendChild(severity)",
		"header.appendChild(pills)",
		"PinalertTrust.createTrustCard()",
		"header.appendChild(PinalertTrust.createStateLine('popup-card__stateline', tag, trust))",
		"PinalertVotes.createVoteBlock(",
	)
	p13MustNotContain(t, "map.js buildPopupCard", popup,
		"header.appendChild(badge)", "header.appendChild(severity)", "header.appendChild(tag)")
	p13MustNotContain(t, "map.js updatePopupCard", jsFunctionBody(t, mapJS, "updatePopupCard(card, report)"), "createStateLine(")

	activity := p23CodeOf(t, "static/js/activity.js")
	p13Ordered(t, activity, "activity.js row loop",
		"PinalertVisibility.updateVisibilityTag(tag, report)",
		"PinalertVisibility.applyVisibilityClass(row, report)",
		"PinalertTrust.createTrustBlock()",
		"body.appendChild(PinalertTrust.createStateLine('report-row__stateline', tag, trustBlock))",
		"body.appendChild(trustBlock.el)",
		"PinalertTrust.updateTrustBlock(trustBlock, readRowTrust(row))",
		"PinalertVotes.createReopenControl(row.dataset.reportId)",
	)
	p13MustNotContain(t, "activity.js", activity, "body.appendChild(tag)")
	p13MustNotContain(t, "activity.js onReopenClick", jsFunctionBody(t, activity, "onReopenClick(row, block)"), "createStateLine(")

	// T-03-23-01: one call per consumer, and the first argument is a quoted
	// literal that ends in __stateline.
	call := regexp.MustCompile(`PinalertTrust\.createStateLine\(([^,)]*),`)
	literal := regexp.MustCompile(`^'[a-z][a-z-]*__stateline'$`)
	for label, text := range map[string]string{"feed.js": feed, "map.js": mapJS, "activity.js": activity} {
		matches := call.FindAllStringSubmatch(text, -1)
		if len(matches) != 1 {
			t.Errorf("%s has %d createStateLine calls, want exactly 1", label, len(matches))
			continue
		}
		if !literal.MatchString(strings.TrimSpace(matches[0][1])) {
			t.Errorf("%s passes %q as the state line class, want a quoted literal ending in __stateline", label, matches[0][1])
		}
	}
}

// TestTrustStateLineStyle pins the declared CSS of the reporter words, the two
// state lines, the narrow reach, the empty line rule and the removed icon and
// separator rules (A75, A76, A80 to A83, T-03-23-02).
func TestTrustStateLineStyle(t *testing.T) {
	raw := p13ReadStatic(t, "static/css/trust.css")
	rules := p13ParseCSS(raw)

	want := func(label string, r p13CSSRule, decls map[string]string) {
		t.Helper()
		for prop, value := range decls {
			if got, ok := r.Decls[prop]; !ok || got != value {
				t.Errorf("%s %s = %q, want %q", label, prop, got, value)
			}
		}
	}

	const wordsHead = ".trust-reporter:not([hidden])"
	words, _ := p13Rule(t, rules, "", wordsHead)
	want(wordsHead, words, map[string]string{
		"display": "block", "min-width": "0", "margin": "0", "padding": "0",
		"font-size": "var(--font-size-label)", "line-height": "var(--line-height-label)",
		"font-weight": "var(--font-weight-regular)", "color": "var(--color-text)",
		"text-decoration": "none", "overflow-wrap": "anywhere",
	})

	// Plain words: no chip, border, fill, ring, shadow or icon mask in any
	// media, and only the regular weight (A75, A82).
	plainBans := []string{
		"background", "background-color", "border", "border-color", "border-radius",
		"outline", "box-shadow", "mask-image", "-webkit-mask-image",
	}
	for _, r := range rules {
		if !strings.Contains(r.Head, ".trust-reporter") {
			continue
		}
		for _, prop := range plainBans {
			if _, ok := r.Decls[prop]; ok {
				t.Errorf("%q declares %s, but the reporter words are plain text (A75)", r.Head, prop)
			}
		}
		if v, ok := r.Decls["font-weight"]; ok && v != "var(--font-weight-regular)" {
			t.Errorf("%q font-weight = %q, want var(--font-weight-regular) (A82)", r.Head, v)
		}
	}

	lineWant := map[string]string{
		"display": "flex", "flex-wrap": "wrap", "align-items": "center",
		"column-gap": "var(--space-sm)", "row-gap": "var(--space-xs)",
	}
	const rowHead = ".report-row__stateline:not([hidden])"
	const popupHead = ".popup-card__stateline:not([hidden])"
	rowLine, rowIdx := p13Rule(t, rules, "", rowHead)
	want(rowHead, rowLine, lineWant)
	popupLine, popupIdx := p13Rule(t, rules, "", popupHead)
	want(popupHead, popupLine, lineWant)
	want(popupHead, popupLine, map[string]string{"flex": "1 1 100%", "min-width": "0"})

	narrow, _ := p13Rule(t, rules, "max-width: 340px", rowHead)
	want("max-width 340px "+rowHead, narrow, map[string]string{"margin-right": "calc(-1 * var(--space-md))"})

	// A83: one rule per line class, display none and nothing else, after the
	// base line rule.
	empty := map[string]int{rowHead: rowIdx, popupHead: popupIdx}
	for head, baseIdx := range empty {
		emptyHead := head + ":not(:has(> :not([hidden])))"
		r, idx := p13Rule(t, rules, "", emptyHead)
		if len(r.Decls) != 1 {
			t.Errorf("%q declares %d properties, want exactly 1: %v", emptyHead, len(r.Decls), r.Decls)
		}
		want(emptyHead, r, map[string]string{"display": "none"})
		if idx <= baseIdx {
			t.Errorf("%q must come after %q, or the base display wins", emptyHead, head)
		}
	}
	hasHeads := 0
	for _, r := range rules {
		if strings.Contains(r.Head, ":has(") {
			hasHeads++
		}
	}
	if hasHeads != 2 {
		t.Errorf("trust.css has %d rule heads with :has(, want exactly 2 (A83)", hasHeads)
	}

	stripped := stripCSSComments(raw)
	for _, name := range append(append([]string{}, p23RetiredClasses...), p23UserGlyph) {
		if strings.Contains(stripped, name) {
			t.Errorf("trust.css still holds %q outside its comments (A76)", name)
		}
	}
}
