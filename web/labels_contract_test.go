package web

// Plan 03-16 contract tests: the visible severity word on every feed row, the
// shared shelter line formatter, the rounded read only labels, and the row
// meta and title legibility.
//
// Amendments pinned here: A1, A24, A26, A37, A38, A40, A42, A44, A45, A53,
// A55, RR-7 and V-40 narrowed to controls (owner Q2).
//
// Honest limit: these tests prove shipped source text and declared CSS. They
// cannot prove how a browser lays a label out, so the 200 percent text size
// check and the 320px wrapping check stay in the plan 03-20 V-42 manual
// script.

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// p16ReadStatic reads one embedded static file and fails the test on error.
func p16ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return string(raw)
}

// p16Ordered fails when a needle is missing or when the first occurrences are
// not strictly increasing, naming the label and the offending pair.
func p16Ordered(t *testing.T, text, label string, needles ...string) {
	t.Helper()
	prevIdx := -1
	prevNeedle := ""
	for _, needle := range needles {
		idx := strings.Index(text, needle)
		if idx == -1 {
			t.Errorf("%s: missing %q", label, needle)
			return
		}
		if idx <= prevIdx {
			t.Errorf("%s: %q must come after %q", label, needle, prevNeedle)
			return
		}
		prevIdx = idx
		prevNeedle = needle
	}
}

// p16MustContain fails once per missing needle.
func p16MustContain(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			t.Errorf("%s: missing %q", label, needle)
		}
	}
}

// p16MustNotContain fails once per present needle.
func p16MustNotContain(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			t.Errorf("%s: must not contain %q", label, needle)
		}
	}
}

// TestRowSeverityLabelMountsFirst pins A26 and A45: the severity word is the
// first child of the row body, built and refreshed through the shared trust.js
// builder, and feed.js owns neither the class name nor the word.
func TestRowSeverityLabelMountsFirst(t *testing.T) {
	feed := p16ReadStatic(t, "static/js/feed.js")
	trust := p16ReadStatic(t, "static/js/trust.js")

	// Plan 03.1-09 (departures R3, R4, R5 of the UI-SPEC): the body order is now
	// severity label, state line, meta, title, trust line. The old literals
	// were body.appendChild(title) before body.appendChild(meta), then
	// PinalertTrust.createTrustBlock( and PinalertVotes.createVoteBlock(, which
	// a compact row no longer builds. The severity word is still the first
	// child appended.
	create := jsFunctionBody(t, feed, "createRow(id)")
	p16Ordered(t, create, "feed.js createRow",
		"PinalertTrust.createSeverityLabel(",
		"body.appendChild(severityLabel)",
		"PinalertVisibility.createVisibilityTag(",
		"PinalertTrust.createTrustLine(",
		"body.appendChild(meta)",
		"body.appendChild(title)",
		"body.appendChild(trust.el)",
	)
	p16MustNotContain(t, "feed.js createRow", create,
		"PinalertTrust.createTrustBlock(", "PinalertVotes.createVoteBlock(")
	if strings.Index(create, "body.appendChild(") != strings.Index(create, "body.appendChild(severityLabel)") {
		t.Errorf("feed.js createRow: the severity label must be the first child appended to the row body (A45)")
	}
	p16MustContain(t, "feed.js createRow", create, "severity: severityLabel")

	update := jsFunctionBody(t, feed, "updateRow(row, report)")
	p16MustContain(t, "feed.js updateRow", update, "PinalertTrust.updateSeverityLabel(row.severity, report)")

	p16MustNotContain(t, "feed.js", feed, "severity-label", "SEVERITY_WORDS")

	build := jsFunctionBody(t, trust, "createSeverityLabel()")
	p16MustContain(t, "createSeverityLabel", build,
		"document.createElement('span')", "'severity-label'", "setAttribute('hidden', '')")

	refresh := jsFunctionBody(t, trust, "updateSeverityLabel(label, report)")
	p16MustContain(t, "updateSeverityLabel", refresh,
		"Pinalert.SEVERITY_WORDS", "hasOwnProperty", "'severity-label severity-label--'",
		"Pinalert.setText(", "removeAttribute('hidden')", "setAttribute('hidden', '')")
	p16MustNotContain(t, "updateSeverityLabel", refresh,
		"innerHTML", "'Critical'", "'Medium'", "'Low'")

	p16MustContain(t, "trust.js exports", trust,
		"createSeverityLabel: createSeverityLabel",
		"updateSeverityLabel: updateSeverityLabel",
		"createTrustBlock: createTrustBlock",
		"updateTrustBlock: updateTrustBlock",
		"fadeStage: fadeStage",
		"focusToggle: focusToggle",
	)
}

// TestShelterLineFormatterIsShared pins A24: one shelter status and headcount
// formatter in app.js, called by feed.js, never containing the word Capacity.
func TestShelterLineFormatterIsShared(t *testing.T) {
	app := p16ReadStatic(t, "static/js/app.js")
	feed := p16ReadStatic(t, "static/js/feed.js")

	words := regexp.MustCompile(`SEVERITY_WORDS\s*=\s*\{\s*low:\s*'Low',\s*medium:\s*'Medium',\s*critical:\s*'Critical'\s*\}`)
	if !words.MatchString(app) {
		t.Errorf("app.js: SEVERITY_WORDS must be exactly low Low, medium Medium, critical Critical")
	}
	p16MustContain(t, "app.js exports", app,
		"SEVERITY_WORDS: SEVERITY_WORDS", "formatShelterLine: formatShelterLine")

	format := jsFunctionBody(t, app, "formatShelterLine(report)")
	p16MustContain(t, "formatShelterLine", format,
		"'shelter_open'", "CAPACITY_STATUSES.indexOf(", "Number.isInteger(", "' person'", "' people'")
	p16MustNotContain(t, "formatShelterLine", format, "Capacity")

	meta := jsFunctionBody(t, feed, "buildMetaText(report)")
	p16MustContain(t, "feed.js buildMetaText", meta, "Pinalert.formatShelterLine(report)")
	p16MustNotContain(t, "feed.js buildMetaText", meta, "' person'", "' people'", "CAPACITY_STATUSES")
	p16MustNotContain(t, "feed.js", feed, "CAPACITY_LABELS")
}

// p16CSSRule is one parsed rule: its file, the at-rule prelude it sits in
// (empty at top level), its whitespace collapsed head, its declarations and
// its position in parse order within the file.
type p16CSSRule struct {
	File, Media, Head string
	Decls             map[string]string
	Index             int
}

// p16ParseCSS parses one stylesheet into rules, one level of at-rule nesting
// deep. A top level prelude that starts with @ and holds nested rules opens a
// block whose prelude becomes Media for every rule inside it.
func p16ParseCSS(file, raw string) []p16CSSRule {
	text := stripCSSComments(raw)
	var rules []p16CSSRule
	var walk func(s, media string)
	walk = func(s, media string) {
		for {
			open := strings.Index(s, "{")
			if open == -1 {
				return
			}
			prelude := strings.Join(strings.Fields(s[:open]), " ")
			depth := 1
			end := open + 1
			for end < len(s) && depth > 0 {
				switch s[end] {
				case '{':
					depth++
				case '}':
					depth--
				}
				end++
			}
			inner := s[open+1 : end-1]
			if strings.HasPrefix(prelude, "@") && strings.Contains(inner, "{") {
				walk(inner, prelude)
			} else {
				rules = append(rules, p16CSSRule{
					File:  file,
					Media: media,
					Head:  prelude,
					Decls: declsOf(inner),
					Index: len(rules),
				})
			}
			s = s[end:]
		}
	}
	walk(text, "")
	return rules
}

// p16AllCSS parses every stylesheet under static/css in the embedded tree.
func p16AllCSS(t *testing.T) []p16CSSRule {
	t.Helper()
	var all []p16CSSRule
	err := fs.WalkDir(StaticFS, "static/css", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".css") {
			return nil
		}
		all = append(all, p16ParseCSS(path, p16ReadStatic(t, path))...)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk static/css: %v", err)
	}
	return all
}

// p16Rule returns the first rule whose Head equals head exactly and whose
// Media is empty when media is empty, or contains media otherwise.
func p16Rule(t *testing.T, rules []p16CSSRule, media, head string) p16CSSRule {
	t.Helper()
	for _, r := range rules {
		if r.Head != head {
			continue
		}
		if (media == "" && r.Media == "") || (media != "" && strings.Contains(r.Media, media)) {
			return r
		}
	}
	t.Fatalf("no rule with head %q in media %q", head, media)
	return p16CSSRule{}
}

// p16TrustRules parses trust.css alone.
func p16TrustRules(t *testing.T) []p16CSSRule {
	t.Helper()
	return p16ParseCSS("static/css/trust.css", p16ReadStatic(t, "static/css/trust.css"))
}

// p16HeadParts splits a head on commas and trims each part.
func p16HeadParts(head string) []string {
	var parts []string
	for _, part := range strings.Split(head, ",") {
		parts = append(parts, strings.TrimSpace(part))
	}
	return parts
}

// p16NamesLabel is true for a head that names a severity label or a state
// label.
func p16NamesLabel(head string) bool {
	return strings.Contains(head, ".severity-label") || strings.Contains(head, ".visibility-tag")
}

// p16IsPseudoOnly is true when every part of the head ends in ::before.
func p16IsPseudoOnly(head string) bool {
	for _, part := range p16HeadParts(head) {
		if !strings.HasSuffix(part, "::before") {
			return false
		}
	}
	return true
}

// p16MustDecl fails unless the rule declares name with exactly want.
func p16MustDecl(t *testing.T, label string, r p16CSSRule, want map[string]string) {
	t.Helper()
	for name, value := range want {
		got, ok := r.Decls[name]
		if !ok {
			t.Errorf("%s: missing declaration %q", label, name)
			continue
		}
		if got != value {
			t.Errorf("%s: %s is %q, want %q", label, name, got, value)
		}
	}
}

const (
	p16SeverityBase = ".severity-label:not([hidden])"
	p16StateBase    = ".visibility-tag:not([hidden])"
	p16LevelCrit    = ".severity-label.severity-label--critical"
	p16LevelMedium  = ".severity-label.severity-label--medium"
	p16LevelLow     = ".severity-label.severity-label--low"
	p16DotHead      = ".severity-label.severity-label--medium::before, .severity-label.severity-label--low::before"
	p16RowLabelHead = ".report-row .severity-label:not([hidden])"
	p16ForcedHead   = ".severity-label:not([hidden]), .report-row .severity-label:not([hidden]), .visibility-tag:not([hidden])"
)

var p16PxValue = regexp.MustCompile(`^(-?[0-9.]+)(px|%)$`)

// The two phone.css heads (quick task 261013-lrw) that the owner's phone
// pictures need and that a house rule here would otherwise refuse. Each is
// matched exactly, in phone.css only, and each is a departure written in
// .planning/sketches/005-phone-layout/README.md.
const (
	// phoneTabDotHead was the dot above the current tab word of the old two tab
	// bottom bar.
	//
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10): the
	// bottom bar is the four item app bar, whose current item shows a short bar
	// above it with a 4px corner at its bottom edge, not a round dot. The head
	// below no longer exists in phone.css; the constant is kept so the
	// exemption in TestReadOnlyLabelsRoundedControlsNot stays harmless, and the
	// new bar needs none because its corner is far from fully rounded. The Phase
	// 3.1 and Phase 4 hands-on tests are walked on this layout after Phase 3.4 is
	// built.
	phoneTabDotHead = `.bottom-nav .bottom-nav__tab[aria-current="page"]::before`
	// phoneBackArrowHead is the round back arrow that Leaflet's close anchor
	// becomes on the report detail sheet.
	phoneBackArrowHead = ".leaflet-container .leaflet-detail-pane .leaflet-popup a.leaflet-popup-close-button:not([hidden])"
	// phoneFeedTitleHead is the feed row title, drawn up to two lines at every
	// width. The head lives in feed.css since plan 03.1-09.
	phoneFeedTitleHead = "#report-list .report-row .report-row__title"
)

// TestReadOnlyLabelsRoundedControlsNot pins A1, A38 and owner Q2: read only
// labels are rounded 12px boxes, no control is fully rounded (V-40 narrowed
// to controls), and the labels keep a visible shape in forced colors (RR-7).
//
// Phone departure (quick task 261013-lrw): the phone.css heads named by
// phoneTabDotHead and phoneBackArrowHead draw the bottom bar's current tab dot
// and the report detail's back arrow round, as the owner's pictures do. The
// dot is a mark above the word, and the arrow is a round icon control of the
// same kind as the exempt .profile-back-link.
func TestReadOnlyLabelsRoundedControlsNot(t *testing.T) {
	trust := p16TrustRules(t)
	all := p16AllCSS(t)

	severity := p16Rule(t, trust, "", p16SeverityBase)
	state := p16Rule(t, trust, "", p16StateBase)
	for label, r := range map[string]p16CSSRule{"severity label": severity, "state label": state} {
		p16MustDecl(t, label, r, map[string]string{"border-radius": "12px"})
	}

	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10): .fab
	// left the list with the floating report button; the exemption must stay
	// backed by a real 50 percent rule, so a name with no rule would fail the
	// non vacuous check below. The Phase 3.1 and Phase 4 hands-on tests are
	// walked on this layout after Phase 3.4 is built.
	exemptControls := []string{".theme-toggle", ".account-trigger", ".profile-back-link", ".icon-badge"}
	checkedHeads := map[string]bool{}
	for _, r := range all {
		radius, ok := r.Decls["border-radius"]
		if !ok {
			continue
		}
		if strings.Contains(r.Head, ".severity-slider") || p16NamesLabel(r.Head) {
			continue
		}
		if r.File == "static/css/phone.css" && (r.Head == phoneTabDotHead || r.Head == phoneBackArrowHead) {
			// The bottom bar's current tab indicator (a 6px dot drawn as a
			// pseudo element above the word) and the report detail's back
			// arrow (a round 44px control on the map strip, like the round
			// .profile-back-link the list above already exempts). The
			// owner's pictures draw both round. Departures recorded in
			// .planning/sketches/005-phone-layout/README.md.
			continue
		}
		exempt := false
		for _, name := range exemptControls {
			if strings.Contains(r.Head, name) {
				exempt = true
			}
		}
		if exempt {
			continue
		}
		checkedHeads[r.Head] = true
		basis := 44.0
		for _, prop := range []string{"min-height", "height"} {
			if m := p16PxValue.FindStringSubmatch(r.Decls[prop]); m != nil && m[2] == "px" {
				if v, err := strconv.ParseFloat(m[1], 64); err == nil {
					basis = v
				}
				break
			}
		}
		for _, component := range strings.Fields(radius) {
			if component == "999px" {
				t.Errorf("%s %q: radius %q, no control may be fully rounded (owner Q2; V-40 narrowed to controls)", r.File, r.Head, radius)
				continue
			}
			m := p16PxValue.FindStringSubmatch(component)
			if m == nil {
				continue
			}
			v, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				continue
			}
			if m[2] == "%" && v >= 50 {
				t.Errorf("%s %q: radius %q, no control may be fully rounded (owner Q2; V-40 narrowed to controls)", r.File, r.Head, radius)
			}
			if m[2] == "px" && v >= basis/2 {
				t.Errorf("%s %q: radius %q is at least half the height %vpx, no control may be fully rounded (owner Q2; V-40 narrowed to controls)", r.File, r.Head, radius, basis)
			}
		}
	}

	// Non vacuous: the checked set holds the well known controls, and the
	// exemptions are real rules that still declare 50 percent.
	for _, head := range []string{".vote-btn:not([hidden])", ".btn", ".view-toggle"} {
		if !checkedHeads[head] {
			t.Errorf("the radius scan never checked a rule with head %q", head)
		}
	}
	main := p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css"))
	for _, head := range []string{".severity-slider::-webkit-slider-thumb", ".severity-slider::-moz-range-thumb"} {
		p16MustDecl(t, head, p16Rule(t, main, "", head), map[string]string{"border-radius": "50%"})
	}
	for _, name := range exemptControls {
		found := false
		for _, r := range all {
			if strings.Contains(r.Head, name) && r.Decls["border-radius"] == "50%" {
				found = true
			}
		}
		if !found {
			t.Errorf("no rule head containing %q declares border-radius: 50%%, so the exemption would pass vacuously", name)
		}
	}

	// A1: a label is 12px, never a percentage or 999px. The dot is pseudo only.
	for _, r := range all {
		radius, ok := r.Decls["border-radius"]
		if !ok || !p16NamesLabel(r.Head) || p16IsPseudoOnly(r.Head) {
			continue
		}
		if strings.HasSuffix(radius, "%") || radius == "999px" {
			t.Errorf("%s %q: label radius %q, want 12px (A1)", r.File, r.Head, radius)
		}
	}

	// RR-7: forced colors keep the rounded shape through a visible border.
	forcedBox := p16Rule(t, trust, "forced-colors: active", p16ForcedHead)
	p16MustDecl(t, "forced colors label box", forcedBox, map[string]string{
		"border": "1px solid CanvasText",
		"color":  "CanvasText",
	})
	forcedDot := p16Rule(t, trust, "forced-colors: active", p16DotHead)
	p16MustDecl(t, "forced colors dot", forcedDot, map[string]string{
		"forced-color-adjust": "none",
		"background-color":    "CanvasText",
	})
	for _, base := range []string{p16SeverityBase, p16LevelCrit, p16LevelMedium, p16LevelLow, p16DotHead, p16RowLabelHead} {
		if r := p16Rule(t, trust, "", base); forcedBox.Index <= r.Index || forcedDot.Index <= r.Index {
			t.Errorf("the forced colors label rules must come after %q", base)
		}
	}
}

// TestSeverityLabelHasNoFixedHeight pins A1, A37, A53, A40 and open item 10:
// min-height only so a label grows with text, the pinned geometry and type,
// and severity colour only in the closed A40 places.
func TestSeverityLabelHasNoFixedHeight(t *testing.T) {
	trust := p16TrustRules(t)
	all := p16AllCSS(t)

	for _, r := range all {
		if !p16NamesLabel(r.Head) || p16IsPseudoOnly(r.Head) {
			continue
		}
		if _, ok := r.Decls["height"]; ok {
			t.Errorf("%s %q declares height, labels use min-height only (A1, A53)", r.File, r.Head)
		}
		if _, ok := r.Decls["display"]; ok {
			for _, part := range p16HeadParts(r.Head) {
				if !strings.Contains(part, ":not([hidden])") {
					t.Errorf("%s %q declares display but %q lacks :not([hidden]) (A37)", r.File, r.Head, part)
				}
			}
		}
		for name, value := range r.Decls {
			if strings.Contains(value, "!important") {
				t.Errorf("%s %q: %s uses !important", r.File, r.Head, name)
			}
			if strings.HasPrefix(name, "--") {
				t.Errorf("%s %q declares the custom property %s", r.File, r.Head, name)
			}
		}
	}

	shared := map[string]string{
		"display":        "inline-flex",
		"align-items":    "center",
		"box-sizing":     "border-box",
		"min-height":     "24px",
		"min-width":      "0",
		"padding":        "0 var(--space-sm)",
		"white-space":    "normal",
		"overflow-wrap":  "anywhere",
		"vertical-align": "top",
		"border-radius":  "12px",
		"font-family":    "var(--font-family)",
		"font-size":      "var(--font-size-label)",
		"line-height":    "var(--line-height-label)",
		"font-weight":    "var(--font-weight-semibold)",
		"color":          "var(--color-text)",
	}
	severity := p16Rule(t, trust, "", p16SeverityBase)
	state := p16Rule(t, trust, "", p16StateBase)
	p16MustDecl(t, "severity label", severity, shared)
	p16MustDecl(t, "state label", state, shared)
	p16MustDecl(t, "severity label", severity, map[string]string{
		"gap":    "var(--space-sm)",
		"border": "1px solid transparent",
	})
	p16MustDecl(t, "state label", state, map[string]string{
		"border":     "1px solid var(--color-border)",
		"background": "var(--color-bg)",
		"margin":     "var(--space-xs) 0 0",
	})

	// Both level heads are (0,2,0), so the Critical colour wins by source order.
	crit := p16Rule(t, trust, "", p16LevelCrit)
	medium := p16Rule(t, trust, "", p16LevelMedium)
	low := p16Rule(t, trust, "", p16LevelLow)
	p16MustDecl(t, "critical level", crit, map[string]string{
		"background": "var(--color-severity-critical-bg)",
		"color":      "var(--color-severity-critical)",
	})
	p16MustDecl(t, "medium level", medium, map[string]string{"background": "var(--color-severity-medium-bg)"})
	p16MustDecl(t, "low level", low, map[string]string{"background": "var(--color-severity-low-bg)"})
	for name, r := range map[string]p16CSSRule{"medium": medium, "low": low} {
		if _, ok := r.Decls["color"]; ok {
			t.Errorf("%s level declares color, its text stays --color-text", name)
		}
	}
	for name, r := range map[string]p16CSSRule{"critical": crit, "medium": medium, "low": low} {
		if r.Index <= severity.Index {
			t.Errorf("%s level rule must come after the base severity rule", name)
		}
	}

	dot := p16Rule(t, trust, "", p16DotHead)
	p16MustDecl(t, "dot", dot, map[string]string{
		"content":          `""`,
		"flex":             "none",
		"width":            "8px",
		"height":           "8px",
		"border-radius":    "50%",
		"background-color": "var(--severity-current)",
	})

	row := p16Rule(t, trust, "", p16RowLabelHead)
	p16MustDecl(t, "row label", row, map[string]string{
		"margin":       "0 0 var(--space-sm)",
		"background":   "var(--color-bg)",
		"border-color": "var(--severity-current)",
	})
	for name, r := range map[string]p16CSSRule{"dot": dot, "row label": row, "critical": crit} {
		if r.Index <= severity.Index {
			t.Errorf("%s rule must come after the base severity rule", name)
		}
	}

	// Departure R2 of plan 03.1-09 (A1, A26, A45): the label of a feed list
	// row, at every width, is a dot and a word with no pill, no fill and no
	// border, and no height of its own. It is the lifted feed.css head; the
	// popup's label (the base rule above) and the Activity row's label (the row
	// head above) keep their bordered shape and are still pinned.
	feedRules := p16ParseCSS("static/css/feed.css", p16ReadStatic(t, "static/css/feed.css"))
	feedRowLabel := p16Rule(t, feedRules, "", "#report-list .report-row .severity-label:not([hidden])")
	p16MustDecl(t, "feed list row label", feedRowLabel, map[string]string{
		"border":        "0",
		"border-radius": "0",
		"background":    "transparent",
		"padding":       "0",
		"margin":        "0",
		"min-height":    "0",
	})
	if _, ok := feedRowLabel.Decls["height"]; ok {
		t.Errorf("the feed list row label declares height, labels use min-height only")
	}

	// A40 closed accent list, over top level rules that name a label.
	for _, r := range trust {
		if r.Media != "" || !p16NamesLabel(r.Head) {
			continue
		}
		for name, value := range r.Decls {
			if strings.Contains(value, "--color-severity") {
				allowed := (r.Head == p16LevelCrit || r.Head == p16LevelMedium || r.Head == p16LevelLow) && name == "background"
				if r.Head == p16LevelCrit && name == "color" {
					allowed = true
				}
				if !allowed {
					t.Errorf("%q: %s uses a severity colour outside the closed list (A40)", r.Head, name)
				}
			}
			if strings.Contains(value, "var(--severity-current)") {
				allowed := (r.Head == p16RowLabelHead && name == "border-color") ||
					(r.Head == p16DotHead && name == "background-color")
				if !allowed {
					t.Errorf("%q: %s uses --severity-current outside the row border and the dot (A40)", r.Head, name)
				}
			}
			for _, banned := range []string{"--severity-tint", "--severity-base", "--color-text-muted"} {
				if strings.Contains(value, banned) {
					t.Errorf("%q: %s references %s (A37, A40)", r.Head, name, banned)
				}
			}
		}
	}

	// Every state label modifier keeps the neutral look, Expired included.
	for _, r := range all {
		if !strings.Contains(r.Head, "visibility-tag--") {
			continue
		}
		for _, name := range []string{"color", "background", "background-color", "border-color"} {
			if _, ok := r.Decls[name]; ok {
				t.Errorf("%s %q declares %s, state labels stay neutral (A1, A44)", r.File, r.Head, name)
			}
		}
	}
}

// TestExpiredRowTreatmentMatchesRetracted pins A44: an expired Activity row
// reads as inactive exactly like a retracted or hidden one, in a rule that
// names no hidden state selector.
func TestExpiredRowTreatmentMatchesRetracted(t *testing.T) {
	raw := p16ReadStatic(t, "static/css/trust.css")
	rules := parseCSSRules(raw)

	expired, ok := ruleBySelector(rules, ".vis-expired")
	if !ok {
		t.Fatalf("trust.css has no .vis-expired rule (A44)")
	}
	expiredDecls := declsOf(expired.declBody)
	for _, other := range []string{".vis-retracted", ".vis-hidden"} {
		r, found := ruleBySelector(rules, other)
		if !found {
			t.Fatalf("trust.css has no %s rule", other)
		}
		want := declsOf(r.declBody)
		for name, value := range want {
			if got, has := expiredDecls[name]; !has || got != value {
				t.Errorf(".vis-expired %s is %q, %s has %q", name, got, other, value)
			}
		}
		for name := range expiredDecls {
			if _, has := want[name]; !has {
				t.Errorf(".vis-expired declares %s, which %s does not", name, other)
			}
		}
	}

	badgeRules := 0
	for _, r := range rules {
		if strings.Contains(r.selectorHead, "vis-expired") && strings.Contains(r.declBody, "background") {
			badgeRules++
			if !strings.Contains(r.selectorHead, ".icon-badge.vis-expired") || !strings.Contains(r.selectorHead, ".vis-expired .icon-badge") {
				t.Errorf("the expired badge rule head %q must hold the compound and the descendant form", r.selectorHead)
			}
		}
		if strings.Contains(r.selectorHead, "vis-expired") &&
			(strings.Contains(r.selectorHead, "vis-hidden") || strings.Contains(r.selectorHead, "vis-retracted")) {
			t.Errorf("rule head %q names expired together with another state", r.selectorHead)
		}
	}
	if badgeRules != 1 {
		t.Errorf("found %d expired rules with a background, want exactly 1", badgeRules)
	}
}

// TestRowTitleWrapsLongWords pins A55 and A42: a long unbroken word wraps
// inside the narrow row body, the title is never clamped, and the meta line
// uses the full text colour.
//
// Phone departure (quick task 261013-lrw), widened to every width by plan
// 03.1-09 (R1): the one feed.css head named by phoneFeedTitleHead clamps the
// feed row title to two lines, as the owner's picture draws it, and phone.css
// no longer holds a title head. Every other head still may not clamp.
//
// The value must be anywhere and not break-word: only anywhere lowers the
// min-content width that sizes the row body flex item, so a pasted URL wraps
// inside the 236px feed body and the 188px Activity body, whose page may not
// apply the feed stylesheet min-width of 0.
func TestRowTitleWrapsLongWords(t *testing.T) {
	main := p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css"))

	title := p16Rule(t, main, "", ".report-row .report-row__title")
	p16MustDecl(t, "row title", title, map[string]string{
		"overflow-wrap": "anywhere",
		"font-size":     "var(--font-size-heading)",
		"font-weight":   "var(--font-weight-semibold)",
		"color":         "var(--color-text)",
	})

	for _, r := range p16AllCSS(t) {
		if !strings.Contains(r.Head, "report-row__title") {
			continue
		}
		if r.File == "static/css/feed.css" && r.Head == phoneFeedTitleHead {
			// Departure R1 of plan 03.1-09 (A55): the compact row of size A
			// clamps the feed row title to two lines at every width, so the
			// one shared head in feed.css clamps and no other head does. The
			// old exempt head sat in phone.css (quick task 261013-lrw); the
			// lift moved it. The opened report shows the whole text.
			continue
		}
		for _, name := range []string{"-webkit-line-clamp", "line-clamp", "text-overflow"} {
			if _, ok := r.Decls[name]; ok {
				t.Errorf("%s %q declares %s, a row title is never clamped (A55)", r.File, r.Head, name)
			}
		}
		if r.Decls["white-space"] == "nowrap" || r.Decls["overflow"] == "hidden" {
			t.Errorf("%s %q hides or stops wrapping the title, a long description must grow the row (A55)", r.File, r.Head)
		}
	}

	meta := p16Rule(t, main, "", ".report-row .report-row__meta")
	p16MustDecl(t, "row meta", meta, map[string]string{
		"color":     "var(--color-text)",
		"font-size": "var(--font-size-label)",
	})
}

// TestLabelAndMetaContrastAcrossThemes pins open item 5 and A42: every label
// and meta text pair clears 4.5:1 in all four theme sources, resolved from the
// declared CSS rather than assumed.
//
// Planning time measurement with the WCAG formula over the main.css tokens:
// Critical text on its tint is 4.639 light and 5.021 dark (margin sensitive,
// 0.14 above the floor, which is why the real tokens are resolved), Critical
// on --color-bg is 5.438 and 6.029, --color-text on the tints is at least
// 14.43 light and 12.23 dark, and the old muted meta text measured 4.12 to
// 4.37 on the light tints, the failure A42 fixes.
func TestLabelAndMetaContrastAcrossThemes(t *testing.T) {
	mainRaw := p16ReadStatic(t, "static/css/main.css")
	main := p16ParseCSS("static/css/main.css", mainRaw)
	trust := p16TrustRules(t)

	needed := []string{
		"--color-text", "--color-text-muted", "--color-bg", "--color-border", "--color-surface",
		"--color-severity-critical", "--color-severity-low-bg", "--color-severity-medium-bg",
		"--color-severity-critical-bg",
	}
	light1, light2, dark1, dark2 := extractThemeTokens(parseCSSRules(mainRaw), needed)
	sources := []struct {
		name   string
		tokens themeTokens
	}{
		{":root", light1},
		{`:root[data-theme="light"]`, light2},
		{"prefers dark :root", dark1},
		{`:root[data-theme="dark"]`, dark2},
	}
	for _, name := range needed {
		for _, s := range sources {
			if _, ok := s.tokens[name]; !ok {
				t.Fatalf("token %s is missing from the %s source", name, s.name)
			}
		}
		if light1[name] != light2[name] {
			t.Errorf("token %s differs between the two light sources: %s and %s", name, light1[name], light2[name])
		}
		if dark1[name] != dark2[name] {
			t.Errorf("token %s differs between the two dark sources: %s and %s", name, dark1[name], dark2[name])
		}
	}

	decl := func(r p16CSSRule, name string) string {
		v, ok := r.Decls[name]
		if !ok {
			t.Fatalf("rule %q declares no %s", r.Head, name)
		}
		return v
	}
	metaText := decl(p16Rule(t, main, "", ".report-row .report-row__meta"), "color")
	baseText := decl(p16Rule(t, trust, "", p16SeverityBase), "color")
	critText := decl(p16Rule(t, trust, "", p16LevelCrit), "color")
	stateText := decl(p16Rule(t, trust, "", p16StateBase), "color")
	critFill := decl(p16Rule(t, trust, "", p16LevelCrit), "background")
	mediumFill := decl(p16Rule(t, trust, "", p16LevelMedium), "background")
	lowFill := decl(p16Rule(t, trust, "", p16LevelLow), "background")
	rowFill := decl(p16Rule(t, trust, "", p16RowLabelHead), "background")
	stateFill := decl(p16Rule(t, trust, "", p16StateBase), "background")
	feedRules := p16ParseCSS("static/css/feed.css", p16ReadStatic(t, "static/css/feed.css"))
	awayText := decl(p16Rule(t, feedRules, "", "#report-list .report-row .report-row__away:not([hidden])"), "color")
	farText := decl(p16Rule(t, feedRules, "", "#report-list .report-row .report-row__far:not([hidden])"), "color")
	trustText := decl(p16Rule(t, feedRules, "", "#report-list .report-row .report-row__trustline:not([hidden])"), "color")
	hiddenTrustText := decl(p16Rule(t, feedRules, "min-width: 900px",
		"#report-list .report-row.vis-hidden .report-row__trustline:not([hidden])"), "color")
	tints := map[string]string{
		"sev-low":         decl(p16Rule(t, main, "", ".sev-low"), "--severity-tint"),
		"sev-medium":      decl(p16Rule(t, main, "", ".sev-medium"), "--severity-tint"),
		"sev-critical":    decl(p16Rule(t, main, "", ".sev-critical"), "--severity-tint"),
		"vis-provisional": decl(p16Rule(t, trust, "", ".vis-provisional"), "--severity-tint"),
	}

	for _, s := range sources {
		resolve := func(value string) string {
			name, ok := bareVarName(value)
			if !ok {
				t.Fatalf("%s: %q is not a bare var() reference", s.name, value)
			}
			hex, ok := s.tokens[name]
			if !ok {
				t.Fatalf("%s: token %s is not in the table", s.name, name)
			}
			return hex
		}
		check := func(pair, fg, bg string) {
			ratio := wcagContrastRatio(t, resolve(fg), resolve(bg), s.name+" "+pair)
			if ratio < 4.5 {
				t.Errorf("%s: %s is %.3f:1, want at least 4.5:1", s.name, pair, ratio)
			}
		}
		check("Critical text on the Critical popup fill", critText, critFill)
		check("Critical text on the row label fill", critText, rowFill)
		check("label text on the Medium fill", baseText, mediumFill)
		check("label text on the Low fill", baseText, lowFill)
		check("label text on the row label fill", baseText, rowFill)
		check("state label text on its fill", stateText, stateFill)
		for name, tint := range tints {
			check("meta text on the "+name+" row tint", metaText, tint)
			// Plan 03.1-09: the laptop row takes the same inner layout, so
			// the distance, the far fact and the trust line sit on the same
			// tints (provisional is --color-surface) in --color-text.
			check("distance text on the "+name+" row tint", awayText, tint)
			check("far fact text on the "+name+" row tint", farText, tint)
			check("trust line text on the "+name+" row tint", trustText, tint)
		}
		// Muted text only on a --color-bg card: the hidden row of a laptop.
		check("muted trust line on a hidden row card", hiddenTrustText, "var(--color-bg)")
	}
}
