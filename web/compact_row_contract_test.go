package web

// Contract tests for plan 03.1-09: the compact list row (size A of sketch 006)
// at every width, and the plain text trust line that replaces the row's
// toggle. These are static inspections of the embedded scripts and
// stylesheets, in the same spirit as the other contract tests of this
// package: they prove the forbidden shapes are absent and the required shapes
// are present. They cannot prove how a browser lays a row out; the real width
// pass at 320, 360 and 390 px is the orchestrator's browser check (plan 16).
//
// Every helper here carries the compactRow prefix.

import (
	"regexp"
	"strings"
	"testing"
)

// compactRowPhoneWidths are the three phone widths the owner judges first.
// The card is the width less two 16px list gutters: 288, 328 and 358px.
var compactRowPhoneWidths = []string{"320", "360", "390"}

// compactRowLaptopOnlyHeads are the heads feed.css already carried before the
// lift that name a row without the #report-list prefix.
var compactRowOldFeedHeads = map[string]bool{
	".report-row":                         true,
	".report-row__body":                   true,
	".report-row .icon-badge .icon-glyph": true,
}

func compactRowFeedRules(t *testing.T) []p16CSSRule {
	t.Helper()
	return p16ParseCSS("static/css/feed.css", p16ReadStatic(t, "static/css/feed.css"))
}

// compactRowHeadParts returns every comma part of every head in rules that
// contains needle.
func compactRowRulesNaming(rules []p16CSSRule, needle string) []p16CSSRule {
	var out []p16CSSRule
	for _, r := range rules {
		for _, part := range p16HeadParts(r.Head) {
			if strings.Contains(part, needle) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// TestRowHasNoControls pins D-21: a list row holds no button and no link, no
// vote block and no trust toggle. Its one tab stop is the li itself, which
// still opens the report on click, Enter and Space.
func TestRowHasNoControls(t *testing.T) {
	code := p19Code(t, "static/js/feed.js")
	create := jsFunctionBody(t, code, "createRow(id)")
	update := jsFunctionBody(t, code, "updateRow(row, report)")

	p13MustNotContain(t, "feed.js createRow", create,
		"createElement('button')", "createElement('a')", "createElement(\"button\")",
		"createVoteBlock(", "createTrustBlock(", "createReopenControl(",
		"aria-expanded", "block.controls", "block.note", "block.error")
	p13MustNotContain(t, "feed.js", code,
		"row.votes", "PinalertVotes.createVoteBlock(", "PinalertVotes.updateVoteBlock(",
		"PinalertTrust.createTrustBlock(", "PinalertTrust.updateTrustBlock(")
	p13MustNotContain(t, "feed.js updateRow", update, "updateVoteBlock(", "updateTrustBlock(")

	p13MustContain(t, "feed.js createRow", create,
		"li.setAttribute('tabindex', '0')",
		"li.addEventListener('click'",
		"li.addEventListener('keydown'",
		"evt.key === 'Enter'", "evt.key === ' '",
		"activateRow(id)")
}

// TestCompactRowAnatomy pins the DOM order of the row body (UI-SPEC Surface 2,
// elements 1 to 8): severity label, state line (visibility tag and reporter
// words), meta, distance, far fact, title, trust line, hidden open cue. The
// badge stays in the DOM for the pins' sake and is hidden by CSS.
func TestCompactRowAnatomy(t *testing.T) {
	code := p19Code(t, "static/js/feed.js")
	create := jsFunctionBody(t, code, "createRow(id)")

	p13Ordered(t, create, "feed.js createRow",
		"PinalertTrust.createSeverityLabel()",
		"body.appendChild(severityLabel)",
		"PinalertTrust.createStateLine('report-row__stateline', tag, trust)",
		"body.appendChild(meta)",
		"body.appendChild(away)",
		"body.appendChild(far)",
		"body.appendChild(title)",
		"body.appendChild(trust.el)",
		"body.appendChild(cue)",
		"li.appendChild(badge)",
		"li.appendChild(body)",
	)
	p13MustContain(t, "feed.js createRow", create,
		"PinalertTrust.createTrustLine()",
		"'report-row__away'", "away: away",
		"'report-row__far'", "far: far",
		"'report-row__open-cue visually-hidden'",
		"'Open report'")

	update := jsFunctionBody(t, code, "updateRow(row, report)")
	p13MustContain(t, "feed.js updateRow", update,
		"Pinalert.setText(row.title, report.description)",
		"Pinalert.distanceText(report)",
		"PinalertTrust.updateTrustLine(row.trust, report.trust)")
	// The distance is the store's sentence, never the old feed rule.
	p13MustNotContain(t, "feed.js updateRow", update,
		"Pinalert.formatDistance(", "Pinalert.feedIsAroundReader()")
}

// TestCompactRowTrustLine pins the plain text trust line: a paragraph, no
// button, no aria-expanded, every string through setText, the tone read from
// the server's facts and the words from the server's trust object.
func TestCompactRowTrustLine(t *testing.T) {
	code := p13CodeLines(p13ReadStatic(t, "static/js/trust.js"))
	create := jsFunctionBody(t, code, "createTrustLine()")
	update := jsFunctionBody(t, code, "updateTrustLine(line, trust)")

	p13MustContain(t, "createTrustLine", create,
		"createElement('p')", "'report-row__trustline'", "createReporterWords()", "reporter: reporter")
	p13MustNotContain(t, "createTrustLine", create,
		"'button'", "aria-expanded", "aria-controls", "addEventListener", "createFactsList(", "createStateLine(")
	p13MustContain(t, "updateTrustLine", update,
		"validTrust(trust)", "currencyTone(trust.facts)", "'data-currency-tone'",
		"trust.confirmed_text", "trust.disputed_text", "signalState(trust)", "Pinalert.setText(")
	p13MustContain(t, "trust.js exports", code,
		"createTrustLine: createTrustLine", "updateTrustLine: updateTrustLine",
		"createTrustBlock: createTrustBlock")
}

// TestCompactRowFarFact pins the too far fact of a row (D-14): it reads only
// the cached fix and the vote radius, never asks for a position, and shows the
// exact words only when the helper says the reader is farther than the radius.
func TestCompactRowFarFact(t *testing.T) {
	votes := p13CodeLines(p13ReadStatic(t, "static/js/votes.js"))
	body := jsFunctionBody(t, votes, "farFromCachedFix(report)")

	p13MustContain(t, "farFromCachedFix", body,
		"report.is_own_report", "readVoteRadiusKm()", "peekVoterLocation()",
		"isFreshVoterLocation(fix, VOTER_LOCATION_HINT_MS)", "greatCircleKm(", "> radiusKm")
	p13MustNotContain(t, "farFromCachedFix", body,
		"getCurrentPosition", "navigator.geolocation", "getVoterLocation(", "permissions",
		"sessionStorage", "setItem", "fetch(", ">=", "<=")
	p13MustContain(t, "votes.js exports", votes, "farFromCachedFix: farFromCachedFix")
	if n := strings.Count(votes, "farFromCachedFix: farFromCachedFix"); n != 1 {
		t.Errorf("votes.js exports farFromCachedFix %d times, want 1", n)
	}

	feed := p13ReadStatic(t, "static/js/feed.js")
	if n := strings.Count(feed, "Too far to vote"); n != 1 {
		t.Errorf("feed.js holds the words Too far to vote %d times, want 1", n)
	}
	update := jsFunctionBody(t, p13CodeLines(feed), "updateRow(row, report)")
	p13MustContain(t, "feed.js updateRow", update,
		"PinalertVotes.farFromCachedFix(report)", "row.far.hidden")
}

// TestCompactRowLiftScopedToList pins the lift of the shared row rules to
// feed.css: every lifted head starts with #report-list, so the Activity page's
// .profile-report-list rows never change, and the laptop keeps the 4px bar and
// the tint of main.css because no lifted rule sets a border or a radius on the
// row itself (those two stay phone only).
func TestCompactRowLiftScopedToList(t *testing.T) {
	feed := compactRowFeedRules(t)

	for _, needle := range []string{".report-row", ".severity-label", ".visibility-tag", ".trust-reporter"} {
		for _, r := range compactRowRulesNaming(feed, needle) {
			for _, part := range p16HeadParts(r.Head) {
				if !strings.Contains(part, needle) || compactRowOldFeedHeads[part] {
					continue
				}
				if !strings.HasPrefix(part, "#report-list ") {
					t.Errorf("feed.css head %q (media %q) names %s without the #report-list prefix", part, r.Media, needle)
				}
			}
		}
	}

	// Amended 2026-10-17 by Phase 3.4 plan 29 (visual audit finding 14): the
	// title clamps at three lines, not two, because two cut every title at 200
	// percent text.
	title := p16Rule(t, feed, "", "#report-list .report-row .report-row__title")
	p16MustDecl(t, "feed.css row title", title, map[string]string{
		"-webkit-line-clamp": "3",
		"line-clamp":         "3",
		"overflow-wrap":      "anywhere",
		"overflow":           "hidden",
	})

	badge := p16Rule(t, feed, "", "#report-list .report-row > .icon-badge")
	p16MustDecl(t, "feed.css row badge", badge, map[string]string{"display": "none"})

	for _, head := range []string{
		"#report-list .report-row .report-row__trustline:not([hidden])",
		"#report-list .report-row .report-row__far:not([hidden])",
		"#report-list .report-row .report-row__away:not([hidden])",
		"#report-list .report-row .report-row__open-cue",
		"#report-list .report-row .report-row__stateline:not([hidden])",
	} {
		p16Rule(t, feed, "", head)
	}

	// No lifted rule decorates the row element itself, at any width.
	for _, r := range feed {
		for _, part := range p16HeadParts(r.Head) {
			if !strings.HasPrefix(part, "#report-list .report-row") {
				continue
			}
			rest := strings.TrimPrefix(part, "#report-list .report-row")
			if rest != "" && !strings.HasPrefix(rest, ".vis-") && !strings.HasPrefix(rest, ".sev-") {
				continue
			}
			// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-17,
			// D-04): a chosen row takes the soft --color-surface fill in place of
			// its tint, so the heads that end in .report-row--selected may set
			// background. They may set nothing else of the row's edge: the border
			// checks below still apply to them. The Phase 3.1 and Phase 4
			// hands-on tests are walked on this layout after Phase 3.4 is built.
			chosen := strings.HasSuffix(part, ".report-row--selected")
			// Amended 2026-10-17 by Phase 3.4 plan 26 (code review part 3 WR-02):
			// inside a forced colours block a chosen head sets the 1px
			// Highlight border UI-SPEC Color asks for (the fill is dropped there)
			// and nothing else; the laptop block keeps the 4px severity bar.
			// Outside forced colours every check below still applies.
			if chosen && strings.Contains(r.Media, "forced-colors") {
				for prop, value := range r.Decls {
					switch {
					case prop == "border" && value == "1px solid Highlight":
					case prop == "border-left-width" && value == "4px":
					default:
						t.Errorf("feed.css head %q in forced colours sets %s: %s; a chosen row may set only a 1px solid Highlight border (and the laptop 4px bar)", part, prop, value)
					}
				}
				continue
			}
			for _, prop := range []string{"border", "border-left", "border-radius", "border-width", "border-color", "background"} {
				if chosen && prop == "background" {
					if r.Decls[prop] != "var(--color-surface)" {
						t.Errorf("feed.css head %q sets background %q, a chosen row takes --color-surface", part, r.Decls[prop])
					}
					continue
				}
				if _, ok := r.Decls[prop]; ok {
					t.Errorf("feed.css head %q sets %s on the row: the 4px bar and the tint of main.css must stay", part, prop)
				}
			}
		}
	}

	// The phone card keeps its own corner and has no bar.
	phone := phoneRules(t)
	card := p16Rule(t, phone, phoneBlockMedia, "#report-list .report-row")
	p16MustDecl(t, "phone.css row card", card, map[string]string{"border": "0", "border-radius": "8px"})

	main := p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css"))
	row := p16Rule(t, main, "", ".report-row")
	p16MustDecl(t, "main.css .report-row", row, map[string]string{
		"border-left": "4px solid var(--severity-current, var(--severity-base))",
	})
	if !strings.Contains(row.Decls["background"], "--severity-tint") {
		t.Errorf("main.css .report-row background is %q, want the severity tint", row.Decls["background"])
	}

	// The lifted rules left phone.css: moved, not copied. The one exception is
	// the denser phone card of the browser findings F1 and F2: phone.css may
	// name the body and the title again, but only to set the spacing between
	// the parts (column-gap on the body, margin on the title), never a lifted
	// layout declaration.
	phoneSpacingOnly := map[string]string{
		"#report-list .report-row .report-row__body":  "column-gap",
		"#report-list .report-row .report-row__title": "margin",
	}
	for _, r := range phone {
		for _, part := range p16HeadParts(r.Head) {
			if prop, ok := phoneSpacingOnly[part]; ok {
				if len(r.Decls) != 1 || r.Decls[prop] == "" {
					t.Errorf("phone.css head %q may set only %s, got %v", part, prop, r.Decls)
				}
				continue
			}
			for _, moved := range []string{
				".report-row__title", ".report-row__stateline", ".report-row > .icon-badge",
				".trust-block__", ".vote-controls", ".vote-note", ".vote-error", ".report-row__body",
			} {
				if strings.HasPrefix(part, "#report-list") && strings.Contains(part, moved) {
					t.Errorf("phone.css still holds the lifted head %q", part)
				}
			}
		}
	}
}

// TestCompactRowGoodToneStaysPhoneOnly pins Pitfall 6: the good tone clears
// 4.5 to 1 only on the phone card, so it lives in phone.css, keyed to the trust
// line, and never reaches a laptop tint. The laptop's muted pair is allowed
// only on a hidden row, whose card is --color-bg.
func TestCompactRowGoodToneStaysPhoneOnly(t *testing.T) {
	feedRaw := p16ReadStatic(t, "static/css/feed.css")
	if strings.Contains(feedRaw, "color-tone-good") {
		t.Errorf("feed.css names --color-tone-good, which is phone only")
	}

	phone := phoneRules(t)
	found := 0
	for _, r := range phone {
		if r.Decls["color"] != "var(--color-tone-good)" || !strings.Contains(r.Head, "#report-list") {
			continue
		}
		found++
		if !strings.HasPrefix(r.Media, phoneBlockMedia) {
			t.Errorf("the good tone rule %q sits in %q, want the phone block", r.Head, r.Media)
		}
		if !strings.Contains(r.Head, `[data-currency-tone="good"]`) {
			t.Errorf("the good tone rule %q is not keyed to the trust line's data-currency-tone", r.Head)
		}
		if strings.Contains(r.Head, ".trust-block") || strings.Contains(r.Head, ".trust-fact") {
			t.Errorf("the good tone rule %q still names the old trust block", r.Head)
		}
		if !strings.Contains(r.Head, ":not(.sev-critical)") || !strings.Contains(r.Head, ":not(.vis-provisional)") {
			t.Errorf("the good tone rule %q must exclude a critical and a provisional row", r.Head)
		}
	}
	if found != 1 {
		t.Errorf("phone.css holds %d good tone rules, want 1", found)
	}

	// Laptop colours: text on a tint, muted only for a hidden row, in the
	// 900px and up block.
	feed := compactRowFeedRules(t)
	base := p16Rule(t, feed, "", "#report-list .report-row .report-row__trustline:not([hidden])")
	p16MustDecl(t, "feed.css trust line", base, map[string]string{"color": "var(--color-text)"})
	for _, r := range feed {
		if r.Decls["color"] != "var(--color-text-muted)" {
			continue
		}
		for _, part := range p16HeadParts(r.Head) {
			if !strings.Contains(part, "#report-list") {
				continue
			}
			if !strings.Contains(part, ".vis-hidden") || strings.Contains(part, "vis-provisional") {
				t.Errorf("feed.css gives a row part muted text through %q: only a hidden row's --color-bg card may", part)
			}
			if !strings.Contains(r.Media, "min-width: 900px") {
				t.Errorf("feed.css muted row rule %q sits in %q, want the 900px and up block", part, r.Media)
			}
		}
	}
}

// TestCompactRowFitsPhoneWidths is the 320, 360 and 390 px guard. It runs no
// browser: it asserts the static rules that make a 288px card (320px less two
// 16px gutters), a 328px card (360px) and a 358px card (390px) unable to force
// a horizontal scroll. The real width pass is plan 16's.
func TestCompactRowFitsPhoneWidths(t *testing.T) {
	widths := strings.Join(compactRowPhoneWidths, ", ")
	feed := compactRowFeedRules(t)
	phone := phoneRules(t)
	var all []p16CSSRule
	all = append(all, feed...)
	all = append(all, phone...)

	dotHead := func(part string) bool { return strings.HasSuffix(part, "::before") }

	// 1. No fixed width and no min-width other than 0, 100% or the tap floor
	//    on a row or anything inside it. The 8px severity dot is a ::before.
	for _, r := range all {
		for _, part := range p16HeadParts(r.Head) {
			if !strings.HasPrefix(part, "#report-list .report-row") || dotHead(part) {
				continue
			}
			if v, ok := r.Decls["width"]; ok && v != "auto" && v != "100%" {
				t.Errorf("at %s px: %q sets width %q, a fixed width can overflow a phone card", widths, part, v)
			}
			if v, ok := r.Decls["min-width"]; ok && v != "0" && v != "100%" && v != "var(--touch-target-min)" {
				t.Errorf("at %s px: %q sets min-width %q, only 0, 100%% or the tap floor are safe", widths, part, v)
			}
			if v, ok := r.Decls["font-size"]; ok {
				switch v {
				case "var(--font-size-label)", "var(--font-size-body)", "var(--font-size-heading)", "var(--font-size-display)":
				default:
					t.Errorf("at %s px: %q font-size %q is not one of the 14, 16, 18, 24 tokens", widths, part, v)
				}
			}
			if strings.Contains(r.Decls["white-space"], "nowrap") && !strings.Contains(part, "open-cue") {
				t.Errorf("at %s px: %q sets white-space nowrap, text could not wrap in a 288px card", widths, part)
			}
		}
	}

	// 2. Every text bearing flex child can shrink and wrap.
	for _, class := range []string{
		".report-row__meta", ".report-row__away", ".report-row__far", ".report-row__title", ".report-row__trustline",
	} {
		shrinks := false
		for _, r := range all {
			for _, part := range p16HeadParts(r.Head) {
				if strings.HasPrefix(part, "#report-list ") && strings.Contains(part, class) && r.Decls["min-width"] == "0" {
					shrinks = true
				}
			}
		}
		if !shrinks {
			t.Errorf("at %s px: no #report-list rule gives %s min-width 0, so it cannot shrink in a 288px card", widths, class)
		}
	}

	// 3. The body wraps, the title wraps long words and clamps at three lines
	// (amended 2026-10-17 by plan 29, finding 14; it was two).
	body := p16Rule(t, feed, "", "#report-list .report-row .report-row__body")
	p16MustDecl(t, "feed.css row body (at "+widths+" px)", body, map[string]string{"display": "flex", "flex-wrap": "wrap"})
	title := p16Rule(t, feed, "", "#report-list .report-row .report-row__title")
	p16MustDecl(t, "feed.css row title (at "+widths+" px)", title, map[string]string{
		"overflow-wrap": "anywhere", "-webkit-line-clamp": "3",
	})

	// 4. The trust line is plain text that may wrap between whole facts.
	line := p16Rule(t, feed, "", "#report-list .report-row .report-row__trustline:not([hidden])")
	if strings.Contains(line.Decls["white-space"], "nowrap") {
		t.Errorf("at %s px: the trust line sets white-space nowrap", widths)
	}
	p16MustDecl(t, "feed.css trust line (at "+widths+" px)", line, map[string]string{"flex-wrap": "wrap", "overflow-wrap": "anywhere"})

	// 5. The row keeps a 44px minimum height from its padding or min-height.
	rowRule := p16Rule(t, feed, "", "#report-list .report-row")
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-17, UI-SPEC
	// 2e and 2h): the laptop row's padding is 12px above and below and 16px at
	// the sides; with a title and a trust line the row is far above 44px. The
	// Phase 3.1 and Phase 4 hands-on tests are walked on this layout after Phase
	// 3.4 is built.
	if rowRule.Decls["padding"] != "var(--space-12) var(--space-md)" && rowRule.Decls["min-height"] != "var(--touch-target-min)" {
		t.Errorf("at %s px: the row has neither the 16px padding nor a 44px min-height: %v", widths, rowRule.Decls)
	}
	if rowRule.Decls["display"] != "block" {
		t.Errorf("at %s px: the row display is %q, want block", widths, rowRule.Decls["display"])
	}
}

// TestCompactRowContrastPairs asserts that the lifted colour rules keep the
// text colour on every tint and the muted colour only where the row is a
// --color-bg card (Pitfall 6, A42), by name: the numbers are resolved by
// TestLabelAndMetaContrastAcrossThemes.
func TestCompactRowContrastPairs(t *testing.T) {
	feed := compactRowFeedRules(t)
	for _, head := range []string{
		"#report-list .report-row .report-row__away:not([hidden])",
		"#report-list .report-row .report-row__far:not([hidden])",
	} {
		r := p16Rule(t, feed, "", head)
		p16MustDecl(t, head, r, map[string]string{"color": "var(--color-text)"})
	}
	// Both phone and laptop meta rules keep the main.css text colour: no lifted
	// rule sets the meta colour to muted except on a white card (phone.css).
	pat := regexp.MustCompile(`report-row__meta`)
	for _, r := range feed {
		if pat.MatchString(r.Head) && r.Decls["color"] == "var(--color-text-muted)" {
			t.Errorf("feed.css sets muted meta text through %q: muted fails 4.5 to 1 on every tint", r.Head)
		}
	}
}
