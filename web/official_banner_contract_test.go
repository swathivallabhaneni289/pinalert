package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file pins the markup, the stylesheet and the script of the Nearby
// banner of Phase 5 plan 15 (ALERT-03, ALERT-04, D-07, D-08, D-09, D-16,
// UI-SPEC Surface 2). The behaviour of the script (what shows, what is said,
// which place it follows) is pinned by official_banner_test.mjs, which draws
// into the template's own markup. These tests hold what a static read can
// hold: the inner spans and their order, the one line 44px row that is one
// link, the level icon and tint pairs, no underline and no motion, and a
// script that builds no markup from a string.

const (
	bannerOpeningTag = `<a class="alert-banner" id="alert-banner" href="/alerts" hidden>`
	bannerScript     = "js/official-banner.js"
	bannerFeedCSS    = "static/css/feed.css"
	bannerPhoneCSS   = "static/css/phone.css"
	bannerPhoneMedia = "@media (max-width: 899.98px)"
	bannerForced     = "@media (forced-colors: active)"
)

// bannerBlock returns the inner markup of the banner link in index.html.tmpl.
func bannerBlock(t *testing.T) string {
	t.Helper()
	index := p421ReadTemplate(t, "index.html.tmpl")
	if got := strings.Count(index, bannerOpeningTag); got != 1 {
		t.Fatalf("index.html.tmpl holds the banner opening tag %d times, want 1: %s", got, bannerOpeningTag)
	}
	at := strings.Index(index, bannerOpeningTag) + len(bannerOpeningTag)
	end := strings.Index(index[at:], "</a>")
	if end == -1 {
		t.Fatalf("the banner link is never closed")
	}
	return index[at : at+end]
}

// bannerRule is the merged rule of one selector head in feed.css.
func bannerRule(t *testing.T, media, head string) p16CSSRule {
	t.Helper()
	return p31mRule(t, bannerFeedCSS, media, head)
}

// TestBannerMarkup pins the opening tag (so every 3.4 pin on it holds) and the
// inner spans in order: the level read first by a screen reader, the icon, the
// event phrase, the dot, the area, the count with its words and the chevron.
// The accessible name is built from the content, with no aria-label.
func TestBannerMarkup(t *testing.T) {
	index := p421ReadTemplate(t, "index.html.tmpl")
	block := bannerBlock(t)

	p421InOrder(t, "banner inner spans", block,
		`<span class="visually-hidden" id="alert-banner-level"></span>`,
		`<span class="alert-banner__icon" aria-hidden="true"></span>`,
		`<span class="alert-banner__event" id="alert-banner-event"></span>`,
		`<span class="alert-banner__sep" aria-hidden="true">&middot;</span>`,
		`<span class="visually-hidden">, </span>`,
		`<span class="alert-banner__area" id="alert-banner-area"></span>`,
		`<span class="visually-hidden">. </span>`,
		`<span class="alert-banner__more" id="alert-banner-more" aria-hidden="true" hidden></span>`,
		`<span class="visually-hidden" id="alert-banner-more-sr"></span>`,
		`<span class="app-icon app-icon--chevron-right alert-banner__chevron" aria-hidden="true"></span>`,
	)

	// The old two line shape is gone.
	for _, gone := range []string{"alert-banner__title", "alert-banner__line", "alert-banner__text", "alert-banner-title", "alert-banner-line"} {
		if strings.Contains(index, gone) {
			t.Errorf("index.html.tmpl still holds %q, the old two line banner", gone)
		}
	}
	// The row is one link: nothing interactive and no heading inside it, and
	// the name comes from the content (no aria-label on the link).
	p13MustNotContain(t, "the banner link", block, "<a ", "<button", "<input", "<h1", "<h2", "<h3", "aria-label", "<img", "<svg")
	if tag := bannerOpeningTag; strings.Contains(tag, "aria-label") || strings.Contains(tag, "role=") {
		t.Errorf("the banner link carries an aria-label or a role: %s", tag)
	}
	// Each id is on one element only.
	for _, id := range []string{"alert-banner", "alert-banner-level", "alert-banner-event", "alert-banner-area", "alert-banner-more", "alert-banner-more-sr"} {
		if got := strings.Count(index, `id="`+id+`"`); got != 1 {
			t.Errorf("index.html.tmpl has %d elements with id %q, want 1", got, id)
		}
	}
	// The banner stays in the list sheet, above the list head, as 3.4 placed it.
	p421InOrder(t, "list pane", index, `id="report-sheet"`, bannerOpeningTag, `<div class="list-head">`)

	// The script comes after app.js, deferred, with the asset version.
	p421InOrder(t, "script order", index,
		`/static/js/app.js?v={{.AssetVersion}}" defer`,
		`/static/js/official-banner.js?v={{.AssetVersion}}" defer`)
	if got := strings.Count(index, "official-banner.js"); got != 1 {
		t.Errorf("index.html.tmpl loads official-banner.js %d times, want 1", got)
	}
	// The Low data view gets no banner (UI-SPEC Defaults 11).
	for _, name := range []string{"low.html.tmpl"} {
		page := p421ReadTemplate(t, name)
		p13MustNotContain(t, name, page, "alert-banner", "official-banner.js")
	}
}

// TestBannerOneLineAndTarget pins UI-SPEC 2a: one 44px line that never wraps,
// the whole row the link, only the area shortening (with an ellipsis), boxed
// on the laptop and a flush row on the phone.
func TestBannerOneLineAndTarget(t *testing.T) {
	row := bannerRule(t, "", ".alert-banner:not([hidden])")
	p16MustDecl(t, ".alert-banner:not([hidden])", row, map[string]string{
		"display":         "flex",
		"align-items":     "center",
		"box-sizing":      "border-box",
		"min-height":      "var(--touch-target-min)",
		"white-space":     "nowrap",
		"margin":          "var(--space-sm) var(--space-md) 0",
		"padding":         "0 var(--space-12)",
		"border-radius":   "8px",
		"color":           "var(--color-text)",
		"text-decoration": "none",
		"font-size":       "var(--font-size-label)",
		"line-height":     "var(--line-height-label)",
	})
	if v, ok := row.Decls["height"]; ok {
		t.Errorf("the banner row sets a fixed height %q; it is min-height only so a larger text size is never clipped", v)
	}
	// The row can never wrap, so no flex-wrap and no normal white space anywhere.
	if v, ok := row.Decls["flex-wrap"]; ok && v != "nowrap" {
		t.Errorf("the banner row wraps: flex-wrap %q", v)
	}

	// Only the area shortens, with an ellipsis; every other part holds its size.
	area := bannerRule(t, "", ".alert-banner__area")
	p16MustDecl(t, ".alert-banner__area", area, map[string]string{
		"overflow":      "hidden",
		"text-overflow": "ellipsis",
		"white-space":   "nowrap",
		"min-width":     "0",
		"font-weight":   "var(--font-weight-regular)",
	})
	for _, head := range []string{".alert-banner__icon", ".alert-banner__event", ".alert-banner__sep", ".alert-banner__more", ".alert-banner__chevron"} {
		r := bannerRule(t, "", head)
		p16MustDecl(t, head, r, map[string]string{"flex": "none"})
	}
	p16MustDecl(t, ".alert-banner__event", bannerRule(t, "", ".alert-banner__event"), map[string]string{
		"font-weight": "var(--font-weight-semibold)",
	})
	p16MustDecl(t, ".alert-banner__more", bannerRule(t, "", ".alert-banner__more:not([hidden])"), map[string]string{
		"font-weight": "var(--font-weight-semibold)",
		"margin-left": "auto",
	})
	chev := bannerRule(t, "", ".alert-banner__chevron")
	p16MustDecl(t, ".alert-banner__chevron", chev, map[string]string{"width": "20px", "height": "20px"})
	icon := bannerRule(t, "", ".alert-banner__icon")
	p16MustDecl(t, ".alert-banner__icon", icon, map[string]string{"width": "20px", "height": "20px"})

	// The focus ring is the global one, drawn inside the row.
	p16MustDecl(t, "focus ring", bannerRule(t, "", ".alert-banner:focus-visible"), map[string]string{"outline-offset": "-2px"})

	// The hidden attribute keeps working: display is only on :not([hidden]), and
	// the id guard stays.
	css := stripCSSComments(p421ReadStatic(t, "css/feed.css"))
	if !strings.Contains(css, "#alert-banner[hidden]") {
		t.Errorf("feed.css lost the #alert-banner[hidden] guard")
	}
	for _, r := range p16ParseCSS(bannerFeedCSS, p421ReadStatic(t, "css/feed.css")) {
		for _, part := range p16HeadParts(r.Head) {
			if strings.HasPrefix(part, ".alert-banner") && !strings.Contains(part, ":not([hidden])") && !strings.Contains(part, "__") &&
				!strings.Contains(part, ":hover") && !strings.Contains(part, ":focus") {
				if _, ok := r.Decls["display"]; ok {
					t.Errorf("feed.css rule %q sets display without :not([hidden])", r.Head)
				}
			}
		}
	}

	// The phone: a flush row, no side borders, top and bottom hairlines in the
	// level colour, 16px padding, no margin, scrolling with the cards.
	phone := p31mRule(t, bannerPhoneCSS, bannerPhoneMedia, ".pane--list > .alert-banner:not([hidden])")
	p16MustDecl(t, "phone banner row", phone, map[string]string{
		"flex":          "none",
		"order":         "-1",
		"margin":        "0",
		"padding":       "0 var(--space-md)",
		"border-radius": "0",
		"border-width":  "1px 0",
	})
}

// TestBannerLevelIconAndTint pins the one level mapping (UI-SPEC Color): the
// Extreme alert is red with the octagon, the Severe one orange with the
// triangle, each icon a mask in the tone colour (never a text glyph, never
// colour alone), and forced colours keep a 1px CanvasText border.
func TestBannerLevelIconAndTint(t *testing.T) {
	p16MustDecl(t, "extreme tint", bannerRule(t, "", ".alert-banner--extreme:not([hidden])"), map[string]string{
		"background":   "var(--color-severity-critical-bg)",
		"border-color": "var(--color-severity-critical)",
	})
	p16MustDecl(t, "severe tint", bannerRule(t, "", ".alert-banner--severe:not([hidden])"), map[string]string{
		"background":   "var(--color-severity-medium-bg)",
		"border-color": "var(--color-severity-medium)",
	})
	p16MustDecl(t, "extreme icon", bannerRule(t, "", ".alert-banner--extreme .alert-banner__icon"), map[string]string{
		"background-color":   "var(--color-tone-bad)",
		"-webkit-mask-image": `url("/static/icons/octagon-alert.svg")`,
		"mask-image":         `url("/static/icons/octagon-alert.svg")`,
	})
	p16MustDecl(t, "severe icon", bannerRule(t, "", ".alert-banner--severe .alert-banner__icon"), map[string]string{
		"background-color":   "var(--color-tone-caution)",
		"-webkit-mask-image": `url("/static/icons/triangle-alert.svg")`,
		"mask-image":         `url("/static/icons/triangle-alert.svg")`,
	})
	p16MustDecl(t, "icon mask", bannerRule(t, "", ".alert-banner__icon"), map[string]string{
		"-webkit-mask-size": "contain", "mask-size": "contain",
		"-webkit-mask-repeat": "no-repeat", "mask-repeat": "no-repeat",
	})
	// The old cyclone storm glyph is gone: the level picks the shape.
	css := stripCSSComments(p421ReadStatic(t, "css/feed.css"))
	if strings.Contains(css, "storm_cyclone") {
		t.Errorf("feed.css still masks the old cyclone icon on the banner")
	}
	// Text on the tint is the normal text colour, never muted (UI-SPEC Color).
	for _, r := range p16ParseCSS(bannerFeedCSS, p421ReadStatic(t, "css/feed.css")) {
		if !strings.Contains(r.Head, "alert-banner") {
			continue
		}
		if v, ok := r.Decls["color"]; ok && strings.Contains(v, "muted") {
			t.Errorf("feed.css rule %q colours banner text %s; text on a tint is --color-text", r.Head, v)
		}
	}
	// Forced colours: a 1px CanvasText border and CanvasText icons.
	forced := bannerRule(t, bannerForced, ".alert-banner:not([hidden])")
	p16MustDecl(t, "forced colours border", forced, map[string]string{"border-color": "CanvasText"})
	icons := bannerRule(t, bannerForced, ".alert-banner--extreme .alert-banner__icon")
	severeIcons := bannerRule(t, bannerForced, ".alert-banner--severe .alert-banner__icon")
	p16MustDecl(t, "forced colours severe icon", severeIcons, map[string]string{"background-color": "CanvasText"})
	p16MustDecl(t, "forced colours icons", icons, map[string]string{"background-color": "CanvasText"})
}

// TestBannerNoUnderlineNoAnimation pins the house rules on the banner: no
// underline at rest, on hover or on focus, no crawl and no animation or
// transition, no fade, and the level is an SVG mask in an empty span (no text
// glyph, no emoji).
func TestBannerNoUnderlineNoAnimation(t *testing.T) {
	underline := regexp.MustCompile(`(?i)underline`)
	for _, name := range []string{bannerFeedCSS, bannerPhoneCSS} {
		raw := p16ReadStatic(t, name)
		for _, r := range p16ParseCSS(name, raw) {
			if !strings.Contains(r.Head, "alert-banner") {
				continue
			}
			for prop, value := range r.Decls {
				if (prop == "text-decoration" || prop == "text-decoration-line") && underline.MatchString(value) {
					t.Errorf("%s rule %q underlines the banner: %s: %s", name, r.Head, prop, value)
				}
				if strings.HasPrefix(prop, "animation") || strings.HasPrefix(prop, "transition") || prop == "will-change" {
					t.Errorf("%s rule %q animates the banner: %s", name, r.Head, prop)
				}
				if prop == "opacity" && strings.TrimSpace(value) != "1" {
					t.Errorf("%s rule %q fades the banner: opacity %s", name, r.Head, value)
				}
				if strings.Contains(value, "marquee") || strings.Contains(value, "scroll") && prop != "overflow" {
					t.Errorf("%s rule %q makes the banner crawl: %s: %s", name, r.Head, prop, value)
				}
			}
		}
	}
	// Hover and focus keep the text as it is.
	hoverRule := bannerRule(t, "", ".alert-banner:hover")
	p16MustDecl(t, "hover", hoverRule, map[string]string{"text-decoration": "none"})
	focusRule := bannerRule(t, "", ".alert-banner:focus")
	p16MustDecl(t, "focus", focusRule, map[string]string{"text-decoration": "none"})
	// No keyframes name the banner.
	css := stripCSSComments(p16ReadStatic(t, bannerFeedCSS))
	for _, banned := range []string{"@keyframes alert-banner", "@keyframes banner"} {
		if strings.Contains(css, banned) {
			t.Errorf("feed.css holds %q", banned)
		}
	}
	// The markup: no marquee, no blink, no text glyph for the level.
	block := bannerBlock(t)
	p13MustNotContain(t, "the banner link", strings.ToLower(block), "<marquee", "<blink", "&#x", "&#1")
	icon := regexp.MustCompile(`<span class="alert-banner__icon" aria-hidden="true"></span>`)
	if !icon.MatchString(block) {
		t.Errorf("the level icon must be an empty aria-hidden span drawn by a mask: %s", block)
	}
	for _, r := range block {
		if r > 0x7e {
			t.Errorf("the banner markup holds the non ASCII character %q", r)
		}
	}
}

// TestBannerScriptSinks pins T-05-81, T-05-82 and T-05-83: no markup is built
// from a string, one place fetches, one interval refreshes, the centre is kept
// in sessionStorage under the key Official alerts reads and only for an
// account, and the script follows the place Nearby announces.
func TestBannerScriptSinks(t *testing.T) {
	raw := p421ReadStatic(t, bannerScript)
	code, bad := scanJSComments(raw)
	if bad {
		t.Fatalf("official-banner.js has an unterminated string or comment, or a quote inside a regular expression")
	}
	for _, banned := range []string{"innerHTML", "insertAdjacentHTML", "outerHTML", "document.write", "eval(", "new Function", "createContextualFragment", "DOMParser", "localStorage"} {
		if strings.Contains(code, banned) {
			t.Errorf("official-banner.js uses %s; feed text enters the page through textContent only", banned)
		}
	}
	if got := strings.Count(code, "fetch("); got != 1 {
		t.Errorf("official-banner.js calls fetch( %d times, want exactly 1", got)
	}
	if got := strings.Count(code, "setInterval("); got != 1 {
		t.Errorf("official-banner.js calls setInterval( %d times, want exactly 1 (the five minute refresh)", got)
	}
	if !strings.Contains(code, "textContent") {
		t.Errorf("official-banner.js never writes textContent")
	}
	p13MustContain(t, "official-banner.js", code,
		"REFRESH_MS = 300000", "NEARBY_CENTRE_KEY = 'pinalert.nearbyCentre'", "'pinalert:feed-centre'",
		"PinalertApiBase", "/official/alerts", "data-account-id", "sessionStorage",
		"function follow", "function fetchBanner", "function render", "function announce", "function rememberCentre",
		"window.PinalertOfficialBanner", "alert-banner--extreme", "alert-banner--severe", "notice-live")
	// The announcement is the one sentence of UI-SPEC 2b.
	p13MustContain(t, "official-banner.js", code, "'Official alert: '")
	// The state gate: the banner is drawn only for a fresh answer (ALERT-04).
	p13MustContain(t, "official-banner.js", code, "'fresh'")
	// No long dash and no sound or vibration wording in the script.
	for _, bad := range []string{string(rune(0x2013)), string(rune(0x2014))} {
		if strings.Contains(raw, bad) {
			t.Errorf("official-banner.js holds a long dash")
		}
	}
	for _, bad := range []string{"vibrate", "Audio(", ".play("} {
		if strings.Contains(code, bad) {
			t.Errorf("official-banner.js uses %s; every notification is silent", bad)
		}
	}
}
