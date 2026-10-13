package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds plan 03-31's contract for the tick mark trust card: one
// facts list builder fed only by trust.facts, the always open popup card, the
// tone glyphs, the shared distance and fade formatters and the tone tokens.
// It reuses the p13 helpers of trust_contract_test.go; every helper this file
// declares starts with p31. Honest limit: static inspection proves the shipped
// text and the declared CSS, not paint. The browser proof is a separate run.

// p31Trust returns trust.js with its whole line comments removed.
func p31Trust(t *testing.T) string {
	t.Helper()
	return p13CodeLines(p13ReadStatic(t, "static/js/trust.js"))
}

// p31Phrase finds a quoted phrase of two or more words, which would be wording
// the browser invented. Single words such as class names and enum values are
// fine.
var p31Phrase = regexp.MustCompile(`'[A-Za-z][A-Za-z]* [A-Za-z][^']*'`)

// p31GlyphTable returns the TONE_GLYPHS declaration, from its first line to the
// first line that is exactly "  };".
func p31GlyphTable(t *testing.T, code string) string {
	t.Helper()
	start := strings.Index(code, "  var TONE_GLYPHS = {")
	if start == -1 {
		t.Fatalf("trust.js has no TONE_GLYPHS table")
	}
	end := strings.Index(code[start:], "\n  };")
	if end == -1 {
		t.Fatalf("the TONE_GLYPHS table has no closing line")
	}
	return code[start : start+end+len("\n  };")]
}

// TestFactsListIsTheOnlySourceOfRows pins the one builder and its one data
// source (A89, D-18, D-20): the list is built once with a slot per kind in a
// closed order, filled from trust.facts only, and the browser holds no wording
// for a fact.
func TestFactsListIsTheOnlySourceOfRows(t *testing.T) {
	code := p31Trust(t)

	p13MustMatch(t, "trust.js FACT_KINDS", code,
		`FACT_KINDS\s*=\s*\[\s*'status',\s*'confirmations',\s*'disputes',\s*'currency',\s*'reporter'\s*\]`)
	p13MustMatch(t, "trust.js FACT_TONES", code,
		`FACT_TONES\s*=\s*\[\s*'good',\s*'neutral',\s*'caution',\s*'bad'\s*\]`)

	create := jsFunctionBody(t, code, "createFactsList(showScoreDetail)")
	p13MustContain(t, "createFactsList", create,
		"document.createElement('ul')", "'trust-facts'", "setAttribute('role', 'list')",
		"'trust-fact trust-fact--'", "'trust-fact__text'", "'trust-fact__title'",
		"'trust-fact__detail'", "'trust-fact__score'", "FACT_KINDS", ".hidden = true")
	p13MustNotContain(t, "createFactsList", create, "Pinalert.setText(", "innerHTML", "addEventListener", "'button'")

	render := jsFunctionBody(t, code, "renderFacts(list, facts)")
	p13MustContain(t, "renderFacts", render,
		"fact.kind", "fact.tone", "fact.title", "fact.detail", "fact.score_detail", "Pinalert.setText(", ".hidden")
	p13MustNotContain(t, "renderFacts", render, "aria-expanded", "setPanelOpen(", "innerHTML", "'Score", "out of")

	valid := jsFunctionBody(t, code, "validFacts(facts)")
	p13MustContain(t, "validFacts", valid,
		"Array.isArray(facts)", "FACT_KINDS.indexOf(", "FACT_TONES.indexOf(", "isNonEmptyString(", "typeof")

	vt := jsFunctionBody(t, code, "validTrust(trust)")
	p13MustContain(t, "validTrust", vt, "validFacts(trust.facts)", "confirmed_text", "disputed_text")
	p13MustNotContain(t, "validTrust", vt, "lead_sentence", "confirmed_sentence", "disputed_sentence")

	panel := jsFunctionBody(t, code, "renderTrustPanel(block, trust)")
	p13MustContain(t, "renderTrustPanel", panel, "renderFacts(block.facts, trust.facts)")

	// No sentence field, no old panel line and no score plumbing survive.
	p13MustNotContain(t, "trust.js", code,
		"PANEL_LINES", "lead_sentence", "confirmed_sentence", "disputed_sentence", "score_sentence",
		"showScores", "trust-block__line", ".sentence")

	// No wording: none of the function bodies of the facts and card builders
	// holds a quoted phrase.
	for _, name := range []string{
		"createFactsList(showScoreDetail)", "renderFacts(list, facts)", "validFacts(facts)",
		"createToneGlyph(tone)", "createTrustCard()", "updateTrustCard(card, trust)",
	} {
		body := jsFunctionBody(t, code, name)
		if m := p31Phrase.FindString(body); m != "" {
			t.Errorf("%s holds the quoted phrase %s, the browser holds no wording for a fact", name, m)
		}
	}
}

// TestToneGlyphsAreFourShapes pins the tone icons (A92, T-03-31-02): DOM built
// SVG, one mark per tone, never colour alone, no style attribute, no emoji.
func TestToneGlyphsAreFourShapes(t *testing.T) {
	raw := p13ReadStatic(t, "static/js/trust.js")
	code := p13CodeLines(raw)

	table := p31GlyphTable(t, code)
	marks := map[string]string{}
	for _, tone := range []string{"good", "neutral", "caution", "bad"} {
		m := regexp.MustCompile(`\b` + tone + `:\s*'([^']+)'`).FindStringSubmatch(table)
		if m == nil {
			t.Errorf("the TONE_GLYPHS table has no mark for the %s tone", tone)
			continue
		}
		for other, d := range marks {
			if d == m[1] {
				t.Errorf("the %s and %s tones share one mark, colour would be the only cue", tone, other)
			}
		}
		marks[tone] = m[1]
	}
	p13MustContain(t, "TONE_GLYPHS", table, "viewBox:", "ns:")

	glyph := jsFunctionBody(t, code, "createToneGlyph(tone)")
	p13MustContain(t, "createToneGlyph", glyph,
		"document.createElementNS(", "TONE_GLYPHS.ns", "setAttribute('aria-hidden', 'true')",
		"setAttribute('focusable', 'false')", "'trust-fact__glyph trust-fact__glyph--'",
		"'trust-fact__disc'", "'trust-fact__mark'", "TONE_GLYPHS.marks[tone]")
	p13MustNotContain(t, "createToneGlyph", glyph,
		"innerHTML", "setAttribute('style'", ".style.", "setAttribute('fill'", "setAttribute('stroke'",
		"setAttribute('color'", "#")

	for _, r := range raw {
		if (r >= 0x1F000 && r <= 0x1FFFF) || (r >= 0x2600 && r <= 0x27BF) {
			t.Errorf("trust.js holds the emoji or symbol %q, tone icons are SVG built with the DOM API", r)
		}
	}
}

// TestTrustCardIsAlwaysOpen pins the popup card (A90, A91): header row with
// the counters and the state word, the facts list with the score lines, and no
// toggle, chevron, panel or expanded state anywhere in it.
func TestTrustCardIsAlwaysOpen(t *testing.T) {
	code := p31Trust(t)

	create := jsFunctionBody(t, code, "createTrustCard()")
	p13Ordered(t, create, "createTrustCard",
		"'trust-card'", "'trust-card__header'", "'trust-card__counts'", "'trust-card__dot'",
		"'trust-card__state'", "createFactsList(true)")
	p13MustContain(t, "createTrustCard", create,
		"el.hidden = true", "setAttribute('aria-hidden', 'true')", "\\u00b7", "confirmedNum", "disputedNum")
	p13MustNotContain(t, "createTrustCard", create,
		"'button'", "aria-expanded", "aria-controls", "chevron", "toggle", "'trust-panel-'",
		"addEventListener", "tabindex", "tabIndex", "reporter", "createReporterWords")

	update := jsFunctionBody(t, code, "updateTrustCard(card, trust)")
	p13MustContain(t, "updateTrustCard", update,
		"validTrust(trust)", "card.el.hidden = true", "renderTrustCounts(card, trust)",
		"renderFacts(card.facts, trust.facts)", "card.state", "trust.currency.label")
	p13MustNotContain(t, "updateTrustCard", update,
		"aria-expanded", "setPanelOpen(", "innerHTML", "reporter", "fade_stage")

	// The tone of the state word is read from the currency fact, never
	// computed from a band or a score.
	tone := jsFunctionBody(t, code, "currencyTone(facts)")
	p13MustContain(t, "currencyTone", tone, "'currency'", "fact.tone")
	p13MustNotContain(t, "currencyTone", tone, "band", "score")

	p13MustContain(t, "trust.js exports", code,
		"createTrustCard: createTrustCard", "updateTrustCard: updateTrustCard")
	p13MustNotContain(t, "trust.js", code, "collapsePanelsIn")
}

// TestRowPanelHoldsTheFactsList pins that the row toggle and its focus rules
// are unchanged while its panel now holds the same facts list, without the
// score lines (A91, D-09, A70).
func TestRowPanelHoldsTheFactsList(t *testing.T) {
	code := p31Trust(t)
	create := jsFunctionBody(t, code, "createTrustBlock()")
	p13MustContain(t, "createTrustBlock", create,
		"createFactsList(false)", "panel.appendChild(facts.el)", "facts: facts",
		"'trust-block__toggle'", "aria-expanded", "'trust-panel-'", "'trust-block__chevron'")
	p13MustNotContain(t, "createTrustBlock", create, "createFactsList(true)", "showScores", "options")

	// One Escape in the popup closes the popup: no panel step inside it.
	level := jsFunctionBody(t, code, "closePopupLevel(popupEl, mayClose)")
	p13Ordered(t, level, "closePopupLevel", "!mayClose", "closeOpenPopup(popupEl)")
	p13MustNotContain(t, "closePopupLevel", level, "closeRowPanel(")
	one := jsFunctionBody(t, code, "closeOneLevel()")
	p13MustContain(t, "closeOneLevel", one, "closeRowPanel(owner)", "closePopupLevel(owner")
}

// TestPopupBuildsTheCardAndTheFadeLine pins the popup consumer (A90, A93, A94).
func TestPopupBuildsTheCardAndTheFadeLine(t *testing.T) {
	mapJS := p23CodeOf(t, "static/js/map.js")

	build := jsFunctionBody(t, mapJS, "buildPopupCard(report)")
	p13Ordered(t, build, "map.js buildPopupCard",
		"PinalertTrust.createTrustCard()",
		"header.appendChild(PinalertTrust.createStateLine('popup-card__stateline', tag, trust))",
		"fade.className = 'popup-card__fade'",
		"body.appendChild(trust.el)",
		"body.appendChild(fade)",
		"body.appendChild(actions)")
	p13MustContain(t, "map.js buildPopupCard", build, "fade: fade", "document.createElement('p')")
	p13MustNotContain(t, "map.js buildPopupCard", build, "createTrustBlock(", "showScores")

	update := jsFunctionBody(t, mapJS, "updatePopupCard(card, report)")
	p13MustContain(t, "map.js updatePopupCard", update,
		"PinalertTrust.updateTrustCard(card.trust, report.trust)",
		"Pinalert.feedIsAroundReader()",
		"Pinalert.formatDistance(report.distance_km)",
		"Pinalert.formatFade(report)",
		"card.fade.hidden")
	p13MustNotContain(t, "map.js updatePopupCard", update,
		"createElement(", "appendChild(", "innerHTML", "new Date", "Date.now", "getTime", "expires_at",
		"updateTrustBlock(")
	// The distance sits behind the centre source gate, in the meta line.
	p13Ordered(t, update, "map.js updatePopupCard distance",
		"Pinalert.feedIsAroundReader()", "Pinalert.formatDistance(report.distance_km)", "card.metaLine")

	p13MustNotContain(t, "map.js", mapJS, "collapsePanelsIn", "confirmed again", "createTrustBlock(")
}

// TestSharedDistanceAndFadeFormatters pins the app.js helpers and the centre
// source flag (A93, A94, A95, T-03-31-03).
func TestSharedDistanceAndFadeFormatters(t *testing.T) {
	app := p23CodeOf(t, "static/js/app.js")

	dist := jsFunctionBody(t, app, "formatDistance(km)")
	p13MustContain(t, "formatDistance", dist,
		"' km away'", "'under 100 m away'", ".toFixed(1)", "Math.round(", "isFinite(")

	fade := jsFunctionBody(t, app, "formatFade(report)")
	p13MustContain(t, "formatFade", fade,
		"report.expires_at", "nowMs()", "Math.ceil(", "Math.floor(", "'Fades in '", "' min.'", "' hr.'", "' hrs.'")
	p13MustNotContain(t, "formatFade", fade, "confirmed again", "unless", "Date.now(")

	around := jsFunctionBody(t, app, "feedIsAroundReader()")
	p13MustContain(t, "feedIsAroundReader", around, "state.center.own === true")

	set := jsFunctionBody(t, app, "setCenter(lat, lon, own)")
	p13MustContain(t, "setCenter", set, "own: own === true")
	p13MustContain(t, "app.js", app,
		"center: { lat: config.fallbackLat, lon: config.fallbackLon, own: false }",
		"formatDistance: formatDistance", "formatFade: formatFade", "feedIsAroundReader: feedIsAroundReader")

	mapJS := p23CodeOf(t, "static/js/map.js")
	centre := jsFunctionBody(t, mapJS, "centerOnVisitor()")
	p13MustContain(t, "centerOnVisitor", centre,
		"function boot(lat, lon, zoom, own)", "Pinalert.setCenter(lat, lon, own)",
		"boot(pos.coords.latitude, pos.coords.longitude, 13, true)",
		"boot(fallbackLat, fallbackLon, 11, false)")
	if n := strings.Count(centre, "boot(fallbackLat, fallbackLon, 11, false)"); n != 2 {
		t.Errorf("centerOnVisitor boots the fallback centre %d times, want 2 (denied and unsupported)", n)
	}
}

// p31Resolve turns a token value into a hex colour: a literal, or one var()
// hop through the same theme source.
func p31Resolve(t *testing.T, tokens themeTokens, name string) string {
	t.Helper()
	v, ok := tokens[name]
	if !ok {
		t.Fatalf("token %s is missing", name)
	}
	if ref, isVar := bareVarName(v); isVar {
		hex, found := tokens[ref]
		if !found {
			t.Fatalf("token %s points at %s, which is not in the table", name, ref)
		}
		return hex
	}
	return v
}

// TestToneTokensClearTheContrastFloors pins the three tone tokens in all four
// theme sources and the contrast of every tone pair on the card surface, from
// the real declared values (A92, A96): text 4.5:1, icon and mark 3:1.
func TestToneTokensClearTheContrastFloors(t *testing.T) {
	mainRaw := p13ReadStatic(t, "static/css/main.css")
	needed := []string{
		"--color-bg", "--color-text", "--color-text-muted", "--color-border",
		"--color-severity-critical",
		"--color-tone-good", "--color-tone-caution", "--color-tone-bad",
	}
	light1, light2, dark1, dark2 := extractThemeTokens(parseCSSRules(mainRaw), needed)
	sources := []struct {
		name   string
		tokens themeTokens
	}{
		{":root", light1}, {`:root[data-theme="light"]`, light2},
		{"prefers dark :root", dark1}, {`:root[data-theme="dark"]`, dark2},
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
	for _, s := range sources {
		bg := p31Resolve(t, s.tokens, "--color-bg")
		check := func(pair, fg string, floor float64) {
			t.Helper()
			ratio := wcagContrastRatio(t, fg, bg, s.name+" "+pair)
			if ratio < floor {
				t.Errorf("%s: %s is %.3f:1 on the card, want at least %.1f:1", s.name, pair, ratio, floor)
			}
		}
		for _, tone := range []string{"good", "caution", "bad"} {
			hex := p31Resolve(t, s.tokens, "--color-tone-"+tone)
			check("the "+tone+" state word and glyph", hex, 4.5)
		}
		check("the muted detail text", p31Resolve(t, s.tokens, "--color-text-muted"), 4.5)
		check("the title text", p31Resolve(t, s.tokens, "--color-text"), 4.5)
	}
}

// TestFactsCSS pins the declared look of the card and the rows (A92, A96).
func TestFactsCSS(t *testing.T) {
	raw := p13ReadStatic(t, "static/css/trust.css")
	rules := p13ParseCSS(raw)

	want := func(head string, decls map[string]string) {
		t.Helper()
		r, _ := p13Rule(t, rules, "", head)
		for prop, value := range decls {
			if got, ok := r.Decls[prop]; !ok || got != value {
				t.Errorf("%q %s = %q, want %q", head, prop, got, value)
			}
		}
	}

	want(".trust-facts", map[string]string{"margin": "0", "padding": "0", "list-style": "none"})
	want(".trust-fact:not([hidden])", map[string]string{
		"display": "grid", "grid-template-columns": "var(--space-lg) minmax(0, 1fr)",
		"column-gap": "var(--space-sm)", "padding": "var(--space-xs) 0",
	})
	want(".trust-fact:not([hidden]) ~ .trust-fact:not([hidden])", map[string]string{
		"border-top": "1px solid var(--color-border)",
	})
	want(".trust-fact__text", map[string]string{"min-width": "0", "overflow-wrap": "anywhere"})
	want(".trust-fact__title", map[string]string{
		"font-size": "var(--font-size-body)", "line-height": "var(--line-height-body)",
		"font-weight": "var(--font-weight-semibold)", "color": "var(--color-text)",
	})
	want(".trust-fact__detail", map[string]string{
		"font-size": "var(--font-size-label)", "line-height": "var(--line-height-label)",
		"font-weight": "var(--font-weight-regular)", "color": "var(--color-text-muted)",
	})
	want(".trust-fact__score", map[string]string{
		"font-size": "var(--font-size-label)", "line-height": "var(--line-height-label)",
		"font-weight": "var(--font-weight-regular)", "color": "var(--color-text-muted)",
	})
	want(".trust-fact__glyph", map[string]string{
		"width": "var(--space-lg)", "height": "var(--space-lg)",
	})
	want(".trust-fact__disc", map[string]string{"fill": "currentColor"})
	want(".trust-fact__mark", map[string]string{
		"fill": "none", "stroke": "var(--color-bg)", "stroke-width": "2",
		"stroke-linecap": "round", "stroke-linejoin": "round",
	})
	want(".trust-fact__glyph--good", map[string]string{"color": "var(--color-tone-good)"})
	want(".trust-fact__glyph--caution", map[string]string{"color": "var(--color-tone-caution)"})
	want(".trust-fact__glyph--bad", map[string]string{"color": "var(--color-tone-bad)"})
	want(".trust-fact__glyph--neutral", map[string]string{"color": "var(--color-text-muted)"})

	want(".trust-card:not([hidden])", map[string]string{
		"display": "block", "margin-top": "var(--space-md)",
		"border": "1px solid var(--color-border)", "border-radius": "12px",
		"background": "var(--color-bg)", "color": "var(--color-text)",
	})
	want(".trust-card__header:not([hidden])", map[string]string{
		"display": "flex", "flex-wrap": "wrap", "justify-content": "space-between",
		"align-items": "baseline", "border-bottom": "1px solid var(--color-border)",
	})
	want(".trust-card__counts", map[string]string{"min-width": "0", "font-size": "var(--font-size-body)"})
	want(".trust-card__state", map[string]string{
		"min-width": "0", "margin-left": "auto", "font-size": "var(--font-size-label)",
		"font-weight": "var(--font-weight-semibold)", "overflow-wrap": "anywhere",
	})
	want(".trust-card__state--good", map[string]string{"color": "var(--color-tone-good)"})
	want(".trust-card__state--caution", map[string]string{"color": "var(--color-tone-caution)"})
	want(".trust-card__state--bad", map[string]string{"color": "var(--color-tone-bad)"})

	// The row panel is the same white card on the tinted row, so the muted
	// detail text keeps its contrast there too.
	want(".trust-block__panel:not([hidden])", map[string]string{
		"display": "block", "margin": "var(--space-sm) 0 0", "padding": "0 var(--space-sm)",
		"border": "1px solid var(--color-border)", "border-radius": "12px",
		"background": "var(--color-bg)", "color": "var(--color-text)",
	})

	// Forced colours keep the glyph shapes and a border (A96).
	forcedGlyph, _ := p13Rule(t, rules, "forced-colors: active", ".trust-fact__glyph")
	if forcedGlyph.Decls["forced-color-adjust"] != "none" || forcedGlyph.Decls["color"] != "CanvasText" {
		t.Errorf("forced colours glyph rule = %v, want forced-color-adjust none and CanvasText", forcedGlyph.Decls)
	}
	forcedMark, _ := p13Rule(t, rules, "forced-colors: active", ".trust-fact__mark")
	if forcedMark.Decls["stroke"] != "Canvas" {
		t.Errorf("forced colours mark stroke = %q, want Canvas", forcedMark.Decls["stroke"])
	}
	forcedCard, _ := p13Rule(t, rules, "forced-colors: active", ".trust-card:not([hidden])")
	if forcedCard.Decls["border-color"] != "CanvasText" {
		t.Errorf("forced colours card border-color = %q, want CanvasText", forcedCard.Decls["border-color"])
	}

	// No raw colour, no declared token, no !important in trust.css.
	stripped := stripCSSComments(raw)
	if m := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\(`).FindString(stripped); m != "" {
		t.Errorf("trust.css holds the raw colour %q", m)
	}
	if strings.Contains(stripped, "!important") {
		t.Errorf("trust.css uses !important")
	}
	if m := regexp.MustCompile(`--color-tone-[a-z]+\s*:`).FindString(stripped); m != "" {
		t.Errorf("trust.css declares %q, the tone tokens live in main.css only", m)
	}

	// The popup keeps its look through the card's own classes: no popup head
	// borrows a trust class (the popup surface test bans it).
	for _, r := range rules {
		if strings.Contains(r.Head, "popup-card") && strings.Contains(r.Head, ".trust-") {
			t.Errorf("popup head %q names a trust class", r.Head)
		}
	}

	// The fade line is a popup owned paragraph with the ancestor field and
	// three classes, so it outranks Leaflet's paragraph reset.
	fade, _ := p13Rule(t, rules, "", ".leaflet-container .popup-card .popup-card__fade:not([hidden])")
	for prop, value := range map[string]string{
		"display": "block", "font-size": "var(--font-size-label)",
		"line-height": "var(--line-height-label)", "color": "var(--color-text-muted)",
	} {
		if got := fade.Decls[prop]; got != value {
			t.Errorf("fade line %s = %q, want %q", prop, got, value)
		}
	}
}
