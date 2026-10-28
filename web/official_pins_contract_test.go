package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file pins the GDACS pin layer of Phase 5 plan 22 (ROBUST-05, ROBUST-06,
// D-12, UI-SPEC Surface 3, research Pitfall 12): the stylesheet that makes the
// official pin a flat rectangle no crowd or Demo pin can be mistaken for, the
// card that reads like an estimate and not like a crowd report, the layer that
// never feeds a crowd number, and the script that builds nothing from a string.
// The behaviour of the script (what is drawn, when, what a key does) is pinned by
// official_pins_test.mjs; these tests hold what a static read can hold.

const (
	officialPinsScript = "static/js/official-pins.js"
	officialPinsCSS    = "static/css/official-pins.css"
	officialPinsLink   = `href="/static/css/official-pins.css?v={{.AssetVersion}}"`
	officialPinsTag    = `src="/static/js/official-pins.js?v={{.AssetVersion}}"`
)

// officialPinRule is the merged rule of one selector head in official-pins.css.
func officialPinRule(t *testing.T, head string) p16CSSRule {
	t.Helper()
	return p31mRule(t, officialPinsCSS, "", head)
}

// pxOrToken says whether a declared size is the 44px touch target, written as
// the literal or as the token.
func pxOrToken(value string) bool {
	return value == "44px" || value == "var(--touch-target-min)"
}

// TestOfficialPinDistinctFromCrowdAndDemo pins D-12 and ROBUST-06: the official
// pin is a flat rectangle 28px high with a 4px radius, filled with the text
// colour and never with a severity colour, centred in a marker box that is at
// least 44px each way, and it shares no class with the crowd pin (round, a
// severity fill) or the Demo pin (round, a dashed ring). The crowd rules in
// main.css and the Demo ring in demo.css are untouched.
func TestOfficialPinDistinctFromCrowdAndDemo(t *testing.T) {
	pin := officialPinRule(t, ".official-pin")
	if pin.Decls["height"] != "28px" {
		t.Errorf(".official-pin height = %q, want 28px", pin.Decls["height"])
	}
	if pin.Decls["border-radius"] != "4px" {
		t.Errorf(".official-pin border-radius = %q, want 4px (never round)", pin.Decls["border-radius"])
	}
	fill := pin.Decls["background"]
	if fill == "" {
		fill = pin.Decls["background-color"]
	}
	if fill != "var(--color-text)" {
		t.Errorf(".official-pin fill = %q, want var(--color-text)", fill)
	}
	if pin.Decls["color"] != "var(--color-bg)" {
		t.Errorf(".official-pin glyph and word colour = %q, want var(--color-bg)", pin.Decls["color"])
	}

	// No official pin rule names a severity or tone colour, a round shape or a
	// dashed ring.
	css := p16ReadStatic(t, officialPinsCSS)
	for _, rule := range p16ParseCSS(officialPinsCSS, css) {
		if !strings.Contains(rule.Head, ".official-pin") {
			continue
		}
		for prop, value := range rule.Decls {
			if strings.Contains(value, "severity") || strings.Contains(value, "--color-tone") {
				t.Errorf("%s { %s: %s }: the official pin carries no severity colour", rule.Head, prop, value)
			}
			if strings.Contains(value, "50%") || strings.Contains(value, "dashed") {
				t.Errorf("%s { %s: %s }: the official pin is never round and has no dashed ring", rule.Head, prop, value)
			}
		}
	}

	// The marker box is at least 44px each way, and the script asks Leaflet for
	// the same box (Leaflet writes the icon size inline, which would beat the
	// stylesheet).
	box := officialPinRule(t, ".official-pin-box")
	if !pxOrToken(box.Decls["min-width"]) || !pxOrToken(box.Decls["min-height"]) {
		t.Errorf(".official-pin-box min-width %q and min-height %q, want the 44px touch target", box.Decls["min-width"], box.Decls["min-height"])
	}
	code := p19Code(t, officialPinsScript)
	p13MustContain(t, "official-pins.js pinIcon", code,
		"className: 'official-pin-box'", "iconSize: [96, 44]", "iconAnchor: [48, 22]", "official-pin__glyph")
	p13MustNotContain(t, "official-pins.js", code, "icon-badge", "icon-badge--pin", "icon-badge--demo", "map-pin--chosen")

	// The chosen look is a grow and nothing else: no ring, nothing under reduced
	// motion.
	chosen := officialPinRule(t, ".official-pin.is-chosen")
	if chosen.Decls["transform"] != "scale(1.35)" {
		t.Errorf(".official-pin.is-chosen transform = %q, want scale(1.35)", chosen.Decls["transform"])
	}
	for prop := range chosen.Decls {
		if strings.Contains(prop, "outline") || strings.Contains(prop, "border") {
			t.Errorf(".official-pin.is-chosen sets %s: the chosen pin has no ring (R4)", prop)
		}
	}
	reduced := p31mRule(t, officialPinsCSS, "@media (prefers-reduced-motion: reduce)", ".official-pin")
	if reduced.Decls["transition"] != "none" {
		t.Errorf("reduced motion: .official-pin transition = %q, want none", reduced.Decls["transition"])
	}

	// The crowd pin stays round on a severity fill and the Demo pin keeps its
	// dashed ring: the two things the official pin is told apart from.
	crowd := p31mRule(t, "static/css/main.css", "", ".icon-badge")
	if crowd.Decls["border-radius"] != "50%" {
		t.Errorf("main.css .icon-badge border-radius = %q, want 50%% (the crowd pin stays round)", crowd.Decls["border-radius"])
	}
	demo := p31mRule(t, "static/css/demo.css", "", "#map .icon-badge--pin.icon-badge--demo")
	if !strings.Contains(demo.Decls["outline"], "dashed") {
		t.Errorf("demo.css ring = %q, want a dashed ring on the Demo pin", demo.Decls["outline"])
	}
}

// TestOfficialPinCard pins UI-SPEC 3b and P5-C14: the card carries the GDACS
// badge, the level word, the title, the Hazard, Issued, Until and Issued by
// lines, the severity text, the honesty line and the credit, and one outlined
// 44px link that opens a gdacs.org page in a new tab. It builds no vote, trust,
// Flag or Mute part.
func TestOfficialPinCard(t *testing.T) {
	code := p19Code(t, officialPinsScript)
	card := jsFunctionBody(t, code, "buildCard(pin)")
	p13MustContain(t, "buildCard", card,
		"official-card__badge", "'GDACS'", "official-card__chip",
		"official-card__title", "'Hazard'", "'Issued '", "'Until '", "'Issued by GDACS'",
		"Automatic estimate. Follow your state disaster management authority.",
		"Global Disaster Alert and Coordination System, GDACS",
		"Open GDACS page", "'_blank'", "noopener noreferrer", "safeGdacsLink(", "make(")
	p13MustContain(t, "official-pins.js make", jsFunctionBody(t, code, "make(tag, className, text)"), "textContent")
	p13MustNotContain(t, "buildCard", card, "innerHTML", "createElement('button')", "createElement('input')")
	// The three level words come from one table the pin name and the chip share.
	p13MustContain(t, "official-pins.js level words", code, "'Red'", "'Orange'", "'Green'")

	// Nothing of the crowd card is built here, by name.
	lower := strings.ToLower(code)
	for _, banned := range []string{"vote", "trust", "'flag", "mute", "confirm", "dispute", "reliab", "resolved", "popup-card", "severity-label"} {
		if strings.Contains(lower, banned) {
			t.Errorf("official-pins.js mentions %q: the card has no vote, trust, Flag or Mute part", banned)
		}
	}

	// The link is drawn only for a gdacs.org page.
	link := jsFunctionBody(t, code, "safeGdacsLink(url)")
	p13MustContain(t, "safeGdacsLink", link, "GDACS_PREFIX", "indexOf(")
	p13MustContain(t, "official-pins.js", code, "var GDACS_PREFIX = 'https://www.gdacs.org/';")

	// The look: the link is an outlined 44px button, never underlined text.
	rule := officialPinRule(t, ".official-card__link")
	if !pxOrToken(rule.Decls["min-height"]) {
		t.Errorf(".official-card__link min-height = %q, want the 44px touch target", rule.Decls["min-height"])
	}
	if rule.Decls["text-decoration"] != "none" {
		t.Errorf(".official-card__link text-decoration = %q, want none (an action is never an underlined link)", rule.Decls["text-decoration"])
	}
	if !strings.Contains(rule.Decls["border"], "1px solid") {
		t.Errorf(".official-card__link border = %q, want an outline", rule.Decls["border"])
	}
	head := officialPinRule(t, ".official-card__head")
	if head.Decls["background"] != "var(--color-surface)" {
		t.Errorf(".official-card__head background = %q, want var(--color-surface)", head.Decls["background"])
	}
	credit := officialPinRule(t, ".official-card__credit")
	if credit.Decls["color"] != "var(--color-text-muted)" {
		t.Errorf(".official-card__credit colour = %q, want the muted token", credit.Decls["color"])
	}
	badge := officialPinRule(t, ".official-card__badge")
	if badge.Decls["border-radius"] != "4px" || badge.Decls["min-height"] != "24px" {
		t.Errorf(".official-card__badge radius %q min-height %q, want 4px and 24px", badge.Decls["border-radius"], badge.Decls["min-height"])
	}
}

// TestOfficialPinsNeverCounted pins D-12 and Phase 3 D-10: the layer is fed by
// /official/pins and nothing else, owns one fetch, and never reaches the list,
// the feed count, the report store or app.js state. A GDACS pin is never a
// crowd number.
func TestOfficialPinsNeverCounted(t *testing.T) {
	code := p19Code(t, officialPinsScript)
	p13MustContain(t, "official-pins.js", code, "'/official/pins'", "PinalertMap.addOverlay(", "PinalertMap.openCard(")
	if n := strings.Count(code, "fetch("); n != 1 {
		t.Errorf("official-pins.js calls fetch( %d times, want 1 (only /official/pins)", n)
	}
	p13MustNotContain(t, "official-pins.js", code,
		"/reports", "/votes", "/feed", "/api/reports",
		"Pinalert.select", "Pinalert.subscribe", "Pinalert.setCenter", "Pinalert.state", "Pinalert.visibleReports",
		"report-list", "feed-count", "reportListEl", "getElementById", "querySelector", "localStorage", "sessionStorage")
	p13MustNotMatchCode(t, "official-pins.js", code, regexp.MustCompile(`\bPinalert\.`))

	// The acceptance grep of the plan: no line of code that is not a comment
	// names the reports route.
	for _, line := range strings.Split(code, "\n") {
		if strings.Contains(line, "'/api/reports") || strings.Contains(line, "/reports'") {
			t.Errorf("official-pins.js names the reports route: %s", line)
		}
	}

	// The map side adds exactly the two seams and nothing that counts.
	mapCode := p19Code(t, "static/js/map.js")
	p13MustContain(t, "map.js API", mapCode, "addOverlay: addOverlay", "openCard: openCard")
	add := jsFunctionBody(t, mapCode, "addOverlay(layer)")
	p13MustNotContain(t, "addOverlay", add, "markers[", "cards[", "Pinalert.")
}

// p13MustNotMatchCode fails when the pattern matches the code.
func p13MustNotMatchCode(t *testing.T, label, code string, re *regexp.Regexp) {
	t.Helper()
	if loc := re.FindStringIndex(code); loc != nil {
		t.Errorf("%s: must not match %s (found %q)", label, re, code[loc[0]:loc[1]])
	}
}

// TestOfficialPinsScriptSinks pins T-05-114: no feed text reaches the page as
// markup.
func TestOfficialPinsScriptSinks(t *testing.T) {
	raw := p16ReadStatic(t, officialPinsScript)
	p13MustNotContain(t, "official-pins.js", raw,
		"innerHTML", "insertAdjacentHTML", "outerHTML", "document.write", "eval(", "new Function", "setAttribute('on", "srcdoc")
	code := p19Code(t, officialPinsScript)
	p13MustContain(t, "official-pins.js", code, "textContent")
}

// TestOfficialPinsAreLinked pins the wiring: Nearby loads the stylesheet and
// the script (the script after map.js, whose API it needs), and the Demo loads
// neither, because the Demo shows no GDACS layer.
func TestOfficialPinsAreLinked(t *testing.T) {
	index := p421ReadTemplate(t, "index.html.tmpl")
	if n := strings.Count(index, officialPinsLink); n != 1 {
		t.Errorf("index.html.tmpl links official-pins.css %d times, want 1", n)
	}
	if n := strings.Count(index, officialPinsTag); n != 1 {
		t.Errorf("index.html.tmpl loads official-pins.js %d times, want 1", n)
	}
	p13MustContain(t, "official-pins.js is deferred", index, officialPinsTag+" defer")
	mapAt := strings.Index(index, `src="/static/js/map.js?v={{.AssetVersion}}"`)
	pinsAt := strings.Index(index, officialPinsTag)
	if mapAt == -1 || pinsAt == -1 || pinsAt < mapAt {
		t.Errorf("official-pins.js must load after map.js (map %d, pins %d)", mapAt, pinsAt)
	}
	cssAt := strings.Index(index, officialPinsLink)
	phoneAt := strings.Index(index, `href="/static/css/phone.css?v={{.AssetVersion}}"`)
	if cssAt == -1 || phoneAt == -1 || cssAt < phoneAt {
		t.Errorf("official-pins.css must load after phone.css so the card wins over the detail pane rules (phone %d, pins %d)", phoneAt, cssAt)
	}
	demo := p421ReadTemplate(t, "demo.html.tmpl")
	p13MustNotContain(t, "demo.html.tmpl", demo, "official-pins")
}
