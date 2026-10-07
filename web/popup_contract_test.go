package web

import (
	"io/fs"
	"strings"
	"testing"
)

// Plan 03-17 pins the map popup card. Like the rest of this package these
// tests are static inspection of the embedded bytes: they prove the wiring,
// the order and the stylesheet declarations exist, and they cannot prove a
// browser rendered the card or kept a panel open across a poll. The browser
// half belongs to the V-42 owner script in plan 03-20.

// p17ReadStatic returns the embedded static file with block comments
// stripped. Line comments stay, so a comment in a script is scanned by every
// check below and must not name a banned literal.
func p17ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return stripCSSComments(string(raw))
}

// p17Ordered requires every needle to be present in body and the first
// indexes to strictly increase in the order given.
func p17Ordered(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	prev := -1
	prevNeedle := ""
	for _, needle := range needles {
		idx := strings.Index(body, needle)
		if idx == -1 {
			t.Errorf("%s: missing %q", label, needle)
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

// p17MustContain reports each needle missing from body.
func p17MustContain(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(body, needle) {
			t.Errorf("%s: missing %q", label, needle)
		}
	}
}

// p17MustNotContain reports each needle present in body.
func p17MustNotContain(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(body, needle) {
			t.Errorf("%s: must not contain %q", label, needle)
		}
	}
}

// TestPopupCardBuiltOnceUpdatedInPlace pins A19, A21, A24, A30, A31, A32,
// D-09 and D-20 on the popup: the card is built once per marker in the A21
// order, every poll writes text, classes and attributes on the existing
// nodes, Leaflet's content setter is gone, and a removed report drops its
// observer.
func TestPopupCardBuiltOnceUpdatedInPlace(t *testing.T) {
	text := p17ReadStatic(t, "static/js/map.js")

	// The old rebuild path is gone. The two literals are split so this file
	// does not name them whole.
	setter := "set" + "PopupContent("
	oldBuilder := "build" + "PopupContent"
	p17MustNotContain(t, "map.js", text, oldBuilder, setter, "unbindPopup(")

	build := jsFunctionBody(t, text, "buildPopupCard(report)")
	p17MustContain(t, "buildPopupCard", build,
		"'popup-card'",
		"setAttribute('tabindex', '-1')",
		"'pinalert-popup-title-'",
		"++popupTitleSeq",
		"PinalertTrust.createSeverityLabel(",
		"updatePopupCard(card, report)",
	)
	p17Ordered(t, "buildPopupCard", build,
		"PinalertTrust.createSeverityLabel(",
		"PinalertVisibility.createVisibilityTag(",
		"Pinalert.setText(titleEl, report.description)",
		"PinalertTrust.createTrustBlock({ showScores: true })",
		"PinalertVotes.createVoteBlock(",
		"actions.appendChild(block.controls)",
		"actions.appendChild(block.note)",
		"actions.appendChild(block.error)",
	)
	if open := strings.Index(build, "{"); open == -1 {
		t.Errorf("buildPopupCard: no opening brace")
	} else if vote := strings.Index(build, "PinalertVotes.createVoteBlock("); vote > open {
		if strings.Contains(build[open:vote], "function") {
			t.Errorf("buildPopupCard declares a nested function before the vote block call (A21)")
		}
	}

	update := jsFunctionBody(t, text, "updatePopupCard(card, report)")
	p17MustContain(t, "updatePopupCard", update,
		"PinalertTrust.updateSeverityLabel(",
		"PinalertVisibility.updateVisibilityTag(",
		"PinalertTrust.updateTrustBlock(",
		"PinalertVotes.updateVoteBlock(",
		"Pinalert.formatShelterLine(report)",
		"Pinalert.relativeTime(",
		"Pinalert.iconClass(",
	)
	p17MustNotContain(t, "updatePopupCard", update,
		"createElement(", "appendChild(",
		"createVisibilityTag(", "createTrustBlock(", "createVoteBlock(", "createSeverityLabel(",
		"innerHTML", "SEVERITY_LABELS", "distance_km",
		"setBlockBusy(", "disabled", ".focus(", "update()",
	)

	bind, anchorFound, termFound := windowAfter(text, "bindPopup(", "marker.on(")
	if !anchorFound || !termFound {
		t.Fatalf("map.js: could not bound the bind call window (anchor=%v terminator=%v)", anchorFound, termFound)
	}
	if v, ok := readOptionValue(bind, "className"); !ok || v != "popupStateClasses(report)" {
		t.Errorf("bind call className must be popupStateClasses(report), found %q (present=%v)", v, ok)
	}
	p17MustContain(t, "bind call", bind, "card.el", "minWidth: 340", "maxWidth: 340", "keepInView: true")
	p17MustNotContain(t, "bind call", bind, "closeOnClick", "maxHeight", "autoPan")

	upsert := jsFunctionBody(t, text, "upsertMarker(report)")
	p17MustContain(t, "upsertMarker", upsert,
		"popupAnchor: [0, -22]",
		"updatePopupCard(cards[report.id], report)",
		"cards[report.id] = card",
		"observePopupCard(marker, card)",
	)

	render := jsFunctionBody(t, text, "render(state)")
	p17MustContain(t, "render", render, "disconnect()", "delete cards[")

	observe := jsFunctionBody(t, text, "observePopupCard(marker, card)")
	p17MustContain(t, "observePopupCard", observe, "ResizeObserver", "requestAnimationFrame(")

	relayout := jsFunctionBody(t, text, "relayoutOpenPopup(marker, card)")
	p17MustContain(t, "relayoutOpenPopup", relayout, "setLatLng(", "getLatLng()")
	// The observer repositions only. The popup update call would re-append
	// the content node and drop focus (A32).
	p17MustNotContain(t, "relayoutOpenPopup", relayout, "update()")

	votes := p17ReadStatic(t, "static/js/votes.js")
	p17MustNotContain(t, "votes.js", votes, "rebuilds its popup", "detached node")
}

// p17Squash collapses runs of whitespace so a multi line value compares equal
// to its one line form.
func p17Squash(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// p17Decls is declsOf with every value whitespace collapsed.
func p17Decls(r cssRule) map[string]string {
	out := map[string]string{}
	for k, v := range declsOf(r.declBody) {
		out[k] = p17Squash(v)
	}
	return out
}

// p17BlockEnd returns the index of the brace that closes the block opened at
// open, or -1 when the braces never balance.
func p17BlockEnd(css string, open int) int {
	depth := 0
	for i := open; i < len(css); i++ {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// p17MediaRules returns the rules of every @media block whose preamble is
// exactly "@media " plus query, in source order. The rules are parsed from
// the block bodies, so their heads carry no at-rule text.
func p17MediaRules(css, query string) []cssRule {
	css = stripCSSComments(css)
	want := "@media " + query
	var out []cssRule
	from := 0
	for {
		idx := strings.Index(css[from:], "@media ")
		if idx == -1 {
			return out
		}
		idx += from
		open := strings.Index(css[idx:], "{")
		if open == -1 {
			return out
		}
		open += idx
		end := p17BlockEnd(css, open)
		if end == -1 {
			return out
		}
		if strings.TrimSpace(css[idx:open]) == want {
			out = append(out, parseCSSRules(css[open+1:end])...)
		}
		from = end + 1
	}
}

// p17MediaIndex returns the byte index of the first @media block whose
// preamble is exactly "@media " plus query, or -1.
func p17MediaIndex(css, query string) int {
	css = stripCSSComments(css)
	want := "@media " + query
	from := 0
	for {
		idx := strings.Index(css[from:], "@media ")
		if idx == -1 {
			return -1
		}
		idx += from
		open := strings.Index(css[idx:], "{")
		if open == -1 {
			return -1
		}
		open += idx
		if strings.TrimSpace(css[idx:open]) == want {
			return idx
		}
		from = open + 1
	}
}

// p17RuleIndex returns the position of the first rule with exactly this head,
// or -1.
func p17RuleIndex(rules []cssRule, head string) int {
	for i, r := range rules {
		if r.selectorHead == head {
			return i
		}
	}
	return -1
}

// p17MustRule returns the rule with exactly this head and requires every
// wanted declaration.
func p17MustRule(t *testing.T, label string, rules []cssRule, head string, want map[string]string) cssRule {
	t.Helper()
	r, ok := ruleBySelector(rules, head)
	if !ok {
		t.Errorf("%s: no rule with head %q", label, head)
		return cssRule{}
	}
	got := p17Decls(r)
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s: %q declares %s: %q, want %q", label, head, name, got[name], value)
		}
	}
	return r
}

// p17InnerHead drops any at-rule text the split on closing braces leaves in
// front of a rule nested in an at-rule.
func p17InnerHead(head string) string {
	if i := strings.LastIndex(head, "{"); i != -1 {
		head = head[i+1:]
	}
	return strings.TrimSpace(head)
}

// TestPopupHeaderGlyphIs20px pins A54: the header glyph is 20px through a
// popup scoped head that outranks main.css's two class 48 percent rule, and
// the feed and Activity glyph keep 48 percent.
func TestPopupHeaderGlyphIs20px(t *testing.T) {
	trust := parseCSSRules(p17ReadStatic(t, "static/css/trust.css"))
	const head = ".leaflet-container .popup-card .icon-badge .icon-glyph"
	p17MustRule(t, "trust.css", trust, head, map[string]string{"width": "20px", "height": "20px"})
	if strings.Contains(head, ".icon-glyph--") {
		t.Errorf("the glyph head must not contain .icon-glyph--")
	}

	mainRules := parseCSSRules(p17ReadStatic(t, "static/css/main.css"))
	p17MustRule(t, "main.css", mainRules, ".icon-badge .icon-glyph",
		map[string]string{"width": "48%", "height": "48%"})
}

// TestPopupCardSurfaceAndGeometry pins the neutral card, its Leaflet
// overrides, the max height ladder and the forced colours block (A2, A9, A10,
// A11, A40, A62, A65, RR-5, RR-7).
func TestPopupCardSurfaceAndGeometry(t *testing.T) {
	css := p17ReadStatic(t, "static/css/trust.css")
	rules := parseCSSRules(css)
	top := func(head string, want map[string]string) cssRule {
		t.Helper()
		return p17MustRule(t, "trust.css", rules, head, want)
	}

	// The one combined wrapper and tip rule (A10).
	combined := 0
	for _, r := range rules {
		h := p17InnerHead(r.selectorHead)
		if strings.Contains(h, ".leaflet-popup-content-wrapper") && strings.Contains(h, ".leaflet-popup-tip") {
			combined++
			d := p17Decls(r)
			if len(d) != 2 || d["background"] != "var(--color-bg)" || d["color"] != "var(--color-text)" {
				t.Errorf("the combined wrapper and tip rule must declare exactly background var(--color-bg) and color var(--color-text), found %v", d)
			}
		}
	}
	if combined != 1 {
		t.Errorf("trust.css has %d rules naming both the wrapper and the tip, want exactly 1", combined)
	}

	top(".leaflet-container .leaflet-popup-content-wrapper", map[string]string{
		"padding": "0", "border-radius": "12px", "overflow": "hidden", "box-shadow": "var(--shadow-popup)",
	})
	top(".leaflet-container .leaflet-popup-tip", map[string]string{"box-shadow": "var(--shadow-popup)"})

	const contentHead = ".leaflet-container .leaflet-popup-content"
	content := top(contentHead, map[string]string{
		"margin":      "0",
		"font-family": "var(--font-family)",
		"font-size":   "var(--font-size-label)",
		"line-height": "var(--line-height-label)",
		"overflow-y":  "auto",
		"max-width":   "calc(100vw - var(--space-md) - var(--space-md))",
	})
	const baseMax = "max-height: max(120px, calc(100dvh - var(--space-md) - var(--space-md) - 20px))"
	body := p17Squash(content.declBody)
	fallback := strings.Index(body, "max-height: 70vh")
	ladder := strings.Index(body, baseMax)
	if fallback == -1 || ladder == -1 || fallback > ladder {
		t.Errorf("the content rule must declare max-height: 70vh before %q (found %d and %d)", baseMax, fallback, ladder)
	}

	narrowQuery := "(max-width: 899.98px)"
	const narrowMax = "max(120px, calc(100dvh - (var(--space-md) + var(--touch-target-min) + var(--space-md)) - (var(--space-lg) + var(--touch-target-min) + var(--space-md)) - 20px))"
	p17MustRule(t, "narrow block", p17MediaRules(css, narrowQuery), contentHead,
		map[string]string{"max-height": narrowMax})
	shortQuery := "(max-height: 359px)"
	p17MustRule(t, "short block", p17MediaRules(css, shortQuery), contentHead,
		map[string]string{"max-height": "max(120px, calc(100dvh - var(--space-md) - var(--space-md) - 20px))"})
	narrowAt, shortAt := p17MediaIndex(css, narrowQuery), p17MediaIndex(css, shortQuery)
	if narrowAt == -1 || shortAt == -1 || shortAt < narrowAt {
		t.Errorf("the max-height 359px block must be declared after the narrow block (found %d and %d)", narrowAt, shortAt)
	}

	top(".leaflet-container .leaflet-popup-content p", map[string]string{"margin": "0"})

	top(".popup-card:not([hidden])", map[string]string{
		"display":     "block",
		"border-left": "4px solid var(--severity-current, var(--color-border))",
		"background":  "var(--color-bg)",
		"color":       "var(--color-text)",
	})
	top(".popup-card__body", map[string]string{"padding": "var(--space-md)"})
	top(".popup-card__header:not([hidden])", map[string]string{
		"display": "flex", "flex-wrap": "wrap", "align-items": "center", "gap": "var(--space-sm)",
		"min-height": "32px", "padding-right": "var(--space-2xl)",
	})
	top(".leaflet-container .popup-card .popup-card__title:not([hidden])", map[string]string{
		"display": "-webkit-box", "-webkit-box-orient": "vertical", "-webkit-line-clamp": "4",
		"overflow": "hidden", "overflow-wrap": "anywhere", "margin": "var(--space-sm) 0 0",
		"padding": "0 var(--space-xs)", "font-size": "var(--font-size-heading)",
		"line-height": "var(--line-height-heading)", "font-weight": "var(--font-weight-semibold)",
		"color": "var(--color-text)",
	})
	top(".popup-card__meta:not([hidden])", map[string]string{
		"display": "flex", "align-items": "flex-start", "gap": "var(--space-sm)",
		"margin-top": "var(--space-xs)", "padding": "0 var(--space-xs)", "color": "var(--color-text-muted)",
	})
	top(".popup-card__meta .icon-glyph", map[string]string{"flex": "none", "width": "16px", "height": "20px"})
	top(".popup-card__meta-lines", map[string]string{"min-width": "0"})
	top(".leaflet-container .popup-card .popup-card__meta-line", map[string]string{
		"margin": "0", "font-size": "var(--font-size-label)", "line-height": "var(--line-height-label)",
		"color": "var(--color-text-muted)", "overflow-wrap": "anywhere",
	})
	top(".leaflet-container .popup-card .popup-card__meta-line + .popup-card__meta-line",
		map[string]string{"margin-top": "var(--space-xs)"})

	actions := top(".popup-card__actions", map[string]string{"padding-top": "var(--space-sm)"})
	if bt := p17Decls(actions)["border-top"]; !strings.Contains(bt, "1px") || !strings.Contains(bt, "var(--color-border)") {
		t.Errorf(".popup-card__actions border-top must be a 1px hairline in var(--color-border), found %q", bt)
	}
	top(".leaflet-container .popup-card__actions .vote-controls:not([hidden])", map[string]string{"margin-top": "0"})
	top(".leaflet-container .popup-card .vote-note", map[string]string{"margin-top": "var(--space-sm)"})
	top(".leaflet-container .popup-card .vote-error:not([hidden])", map[string]string{"margin-top": "var(--space-xs)"})
	top(".leaflet-container .popup-card:focus-visible", map[string]string{"outline-offset": "-2px"})

	// Every popup rule that declares display keeps the hidden guard, none
	// borrows another owner's heads, and every Leaflet head carries the
	// ancestor field.
	for _, r := range rules {
		head := p17InnerHead(r.selectorHead)
		isPopup := strings.Contains(head, "popup-card") || strings.Contains(head, "leaflet-popup")
		if !isPopup {
			continue
		}
		for _, banned := range []string{".trust-", ".icon-glyph--", ".severity-label", ".visibility-tag"} {
			if strings.Contains(head, banned) {
				t.Errorf("popup head %q contains %q, which another plan owns", head, banned)
			}
		}
		_, hasDisplay := declsOf(r.declBody)["display"]
		for _, part := range strings.Split(head, ",") {
			part = strings.TrimSpace(part)
			if hasDisplay && !strings.Contains(part, ":not([hidden])") {
				t.Errorf("popup head part %q declares display without :not([hidden])", part)
			}
			if strings.Contains(part, "leaflet-popup") && !strings.Contains(part, ".leaflet-container ") {
				t.Errorf("popup head part %q lacks the .leaflet-container ancestor", part)
			}
		}
	}

	// Forced colours (RR-7).
	forced := p17MediaRules(css, "(forced-colors: active)")
	p17MustRule(t, "forced colors", forced, ".popup-card:not([hidden])", map[string]string{"border-left-color": "CanvasText"})
	p17MustRule(t, "forced colors", forced, ".leaflet-container .popup-card .icon-badge", map[string]string{"outline": "1px solid CanvasText"})
	p17MustRule(t, "forced colors", forced, ".leaflet-container .popup-card .icon-glyph", map[string]string{
		"forced-color-adjust": "none", "background-color": "CanvasText",
	})

	// The one new token lives in main.css only.
	mainRules := parseCSSRules(p17ReadStatic(t, "static/css/main.css"))
	p17MustRule(t, "main.css", mainRules, ":root", map[string]string{"--shadow-popup": "0 2px 8px rgb(0 0 0 / 25%)"})
	declared := 0
	for _, r := range mainRules {
		if _, ok := declsOf(r.declBody)["--shadow-popup"]; ok {
			declared++
		}
	}
	if declared != 1 {
		t.Errorf("main.css declares --shadow-popup %d times, want exactly 1", declared)
	}
	for _, r := range rules {
		if _, ok := declsOf(r.declBody)["--shadow-popup"]; ok {
			t.Errorf("trust.css must not declare --shadow-popup")
		}
	}

	// No footer, comments count or comments link (A11).
	mapJS := p17ReadStatic(t, "static/js/map.js")
	p17MustNotContain(t, "map.js", mapJS, "popup-card__footer", "__footer", "-footer")
	p17MustNotContain(t, "trust.css", css, "popup-card__footer", "__footer", ".footer", "-footer")
}
