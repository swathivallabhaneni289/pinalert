package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// This file pins plan 04-13's second half: the head script that shows the
// slow message even when the map libraries hang (D-17, P4-A24), the vendor
// stylesheets that no longer hold first paint, and the notice card.
//
// Honest limit of these tests' claim: they inspect source text. They cannot
// prove that a real browser paints the card by 6 seconds with unpkg blocked,
// or that a stylesheet swapped from print media paints without a flash. That
// is the job of the human check in plan 04-27.

const (
	p413LeafletCSS   = "https://unpkg.com/leaflet@1.9.4/dist/leaflet.css"
	p413MaplibreCSS  = "https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.css"
	p413SlowTagBlock = `<script src="/static/js/slow-notice.js?v={{.AssetVersion}}"></script>`
)

// TestSlowNoticeLoadsFirst pins D-17 and P4-A24: slow-notice.js is the first
// script in the head, not deferred, only on the main page, and the card, the
// bar and the live region sit in the body before the blocking vendor scripts.
func TestSlowNoticeLoadsFirst(t *testing.T) {
	tmpl := p413ReadTemplate(t, "index.html.tmpl")
	if n := strings.Count(tmpl, p413SlowTagBlock); n != 1 {
		t.Fatalf("the slow-notice.js tag must appear once, found %d", n)
	}
	tagAt := strings.Index(tmpl, p413SlowTagBlock)
	headAt := strings.Index(tmpl, "<head>")
	headEnd := strings.Index(tmpl, "</head>")
	firstLink := strings.Index(tmpl, "<link")
	themeAt := strings.Index(tmpl, "theme.js")
	if !(headAt != -1 && tagAt > headAt && tagAt < headEnd && tagAt < firstLink && tagAt < themeAt) {
		t.Errorf("slow-notice.js must be first in the head, before every link and theme.js (%d, %d, %d, %d)",
			tagAt, headAt, firstLink, themeAt)
	}
	window := findTagWindow(t, tmpl, "/static/js/slow-notice.js")
	p413MustNotContain(t, "the slow-notice.js tag", window, " defer", " async")

	err := fs.WalkDir(TemplatesFS, "templates", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || strings.HasSuffix(path, "index.html.tmpl") {
			return walkErr
		}
		raw, readErr := TemplatesFS.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(raw), "slow-notice.js") {
			t.Errorf("%s mentions slow-notice.js; only the main page loads it", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	firstUnpkgScript := strings.Index(tmpl, `<script src="https://unpkg.com`)
	shellAt := strings.Index(tmpl, `id="app-shell"`)
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-08): the
	// light and dark button moved into the laptop top bar, so the page no
	// longer includes it; the point the card follows is the top bar that holds
	// it. The Phase 3.1 and Phase 4 hands-on tests are walked on this layout
	// after Phase 3.4 is built.
	//
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-03, D-01):
	// the card moved from the top of the body into the list pane, right after
	// the list head, so on a phone its rows sit inside the sheet and never float
	// over the list. It is still parsed before the first vendor script, which is
	// what the slow message needs when the libraries hang; on a laptop its fixed
	// bottom left place is unchanged. The Phase 3.1 and Phase 4 hands-on tests
	// are walked on this layout after Phase 3.4 is built.
	toggleAt := strings.Index(tmpl, `{{template "app_topbar" .}}`)
	for _, marker := range []string{`class="top-chrome"`, `id="notice-card"`, `id="notice-live"`} {
		at := strings.Index(tmpl, marker)
		if at == -1 || at > firstUnpkgScript {
			t.Errorf("%s must come before the first vendor script (%d, %d)", marker, at, firstUnpkgScript)
		}
	}
	cardAt := strings.Index(tmpl, `id="notice-card"`)
	listHeadAt := strings.Index(tmpl, `class="list-head"`)
	listAt := strings.Index(tmpl, `<ol id="report-list">`)
	if !(toggleAt != -1 && shellAt > toggleAt && listHeadAt > shellAt && cardAt > listHeadAt && cardAt < listAt) {
		t.Errorf("the card must sit in the list pane, after the list head and before the list (%d, %d, %d, %d, %d)",
			toggleAt, shellAt, listHeadAt, cardAt, listAt)
	}
	p413MustContain(t, "index.html.tmpl", tmpl,
		`<p class="notice-card__text">Taking long?</p>`,
		`<a class="notice-card__link" href="/low">Open the low data view.</a>`)
	for _, id := range []string{`id="notice-slow"`, `id="notice-stale"`, `id="notice-stale-text"`} {
		if n := strings.Count(tmpl, id); n != 1 {
			t.Errorf("%s must appear once, found %d", id, n)
		}
	}

	feed := p413ReadStatic(t, "static/js/feed.js")
	p413MustContain(t, "markFeedLoaded", jsFunctionBody(t, feed, "markFeedLoaded()"),
		"setAttribute('data-feed-state', 'ok')", "setHidden(noticeSlowEl, true)", "syncNoticeCard()")
	r := jsFunctionBody(t, feed, "render(state)")
	p413Ordered(t, "render", r, "if (!listShown) {", "if (state.status === 'ok') {", "markFeedLoaded();")
}

// TestVendorStylesheetsDoNotBlockPaint pins P4-A24: both vendor stylesheets
// load through the print media swap, keeping integrity and crossorigin.
func TestVendorStylesheetsDoNotBlockPaint(t *testing.T) {
	tmpl := p413ReadTemplate(t, "index.html.tmpl")
	for _, href := range []string{p413LeafletCSS, p413MaplibreCSS} {
		if n := strings.Count(tmpl, href); n != 1 {
			t.Errorf("%s must appear once, found %d", href, n)
			continue
		}
		window := findTagWindow(t, tmpl, href)
		p413MustContain(t, href, window,
			`rel="stylesheet"`, `media="print"`, `onload="this.media='all'"`,
			`integrity="sha256-`, `crossorigin="anonymous"`)
	}
	p413MustNotContain(t, "index.html.tmpl", tmpl, "<noscript")
	if n := strings.Count(tmpl, "leaflet@1.9.4/dist/leaflet.css"); n != 1 {
		t.Errorf("the Leaflet stylesheet must appear once, found %d", n)
	}
}

// TestNoticeCardHidesForToastAndPopup pins P4-A12 and the popup flag: one
// rule hides the card under the three root flags, and the scripts set them.
func TestNoticeCardHidesForToastAndPopup(t *testing.T) {
	chrome := p413ReadStatic(t, "static/css/chrome.css")
	want := map[string]bool{
		"html[data-toast-open] .notice-card": true,
		"html[data-popup-open] .notice-card": true,
		"html[data-modal-open] .notice-card": true,
	}
	found := 0
	for _, r := range parseCSSRules(chrome) {
		heads := map[string]bool{}
		for h := range strings.SplitSeq(r.selectorHead, ",") {
			heads[strings.TrimSpace(h)] = true
		}
		if len(heads) != len(want) {
			continue
		}
		match := true
		for h := range want {
			if !heads[h] {
				match = false
			}
		}
		if match {
			found++
			if declsOf(r.declBody)["display"] != "none" {
				t.Errorf("the hide rule must declare display: none")
			}
		}
	}
	if found != 1 {
		t.Errorf("chrome.css must hold exactly one hide rule for the three flags, found %d", found)
	}

	app := p413ReadStatic(t, "static/js/app.js")
	p413MustContain(t, "showToast", jsFunctionBody(t, app, "showToast(message, action, durationMs)"),
		"setAttribute('data-toast-open', 'true')")

	rawMap, err := StaticFS.ReadFile("static/js/map.js")
	if err != nil {
		t.Fatal(err)
	}
	mapJS := stripJSComments(string(rawMap))
	p413MustContain(t, "onPopupOpen", jsFunctionBody(t, mapJS, "onPopupOpen(evt)"),
		"document.documentElement.setAttribute('data-popup-open', 'true')")
	p413MustContain(t, "onPopupClose", jsFunctionBody(t, mapJS, "onPopupClose(evt)"),
		"document.documentElement.removeAttribute('data-popup-open')")
}

// TestNoticeCardGeometry pins the card's element table row and its tokens.
func TestNoticeCardGeometry(t *testing.T) {
	raw, err := StaticFS.ReadFile("static/css/chrome.css")
	if err != nil {
		t.Fatal(err)
	}
	chrome := stripCSSComments(string(raw))
	rules := parseCSSRules(chrome)

	card, ok := ruleBySelector(rules, ".notice-card")
	if !ok {
		t.Fatal("chrome.css has no .notice-card rule")
	}
	d := declsOf(card.declBody)
	stack := "calc(var(--space-lg) + var(--touch-target-min) + var(--space-md))"
	for prop, want := range map[string]string{
		"position": "fixed", "z-index": "1000", "left": "var(--space-md)", "bottom": stack, "right": stack,
		"max-width": "360px", "box-sizing": "border-box", "background": "var(--color-bg)",
		"border": "1px solid var(--color-border)", "border-radius": "8px", "box-shadow": "var(--shadow-popup)",
		"padding": "var(--space-sm) var(--space-md)", "color": "var(--color-text)",
		"font-size": "var(--font-size-label)", "line-height": "var(--line-height-label)",
		"overflow-y": "auto", "overscroll-behavior": "contain",
	} {
		if d[prop] != want {
			t.Errorf(".notice-card %s = %q, want %q", prop, d[prop], want)
		}
	}
	if _, has := d["display"]; has {
		t.Error(".notice-card must not declare display; the :not([hidden]) rule does")
	}
	heights := regexp.MustCompile(`max-height:\s*([^;]+);`).FindAllStringSubmatch(card.declBody, -1)
	if len(heights) != 2 || !strings.Contains(heights[0][1], "100vh") || !strings.Contains(heights[1][1], "100dvh") {
		t.Errorf(".notice-card must hold two max-height declarations, 100vh first and 100dvh second, found %v", heights)
	}
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
	// D-31): the bar on every page is replaced by the laptop strip on Nearby and
	// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
	// every page) is retired as worded; Disclaimer and Policy are reached from the
	// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
	// first-time flow, the login screen and the one reader. The Phase 3.1 and
	// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
	// The card still subtracts the token, now the strip's height or 0px.
	for _, h := range heights {
		if !strings.Contains(h[1], "var(--disclaimer-bar-height)") {
			t.Errorf("max-height %q must subtract the bar height", h[1])
		}
	}

	for sel, want := range map[string]map[string]string{
		".notice-card:not([hidden])":      {"display": "flex", "flex-direction": "column", "gap": "var(--space-sm)"},
		".notice-card__row:not([hidden])": {"display": "flex", "flex-direction": "column"},
		".notice-card__text":              {"margin": "0"},
		".notice-card__link:not([hidden])": {
			"display": "flex", "align-items": "center", "min-height": "var(--touch-target-min)",
			"font-size": "var(--font-size-body)", "font-weight": "var(--font-weight-semibold)",
			"text-decoration": "underline", "color": "var(--color-text)",
		},
	} {
		rule, found := ruleBySelector(rules, sel)
		if !found {
			t.Errorf("chrome.css has no %s rule", sel)
			continue
		}
		got := declsOf(rule.declBody)
		for prop, value := range want {
			if got[prop] != value {
				t.Errorf("%s %s = %q, want %q", sel, prop, got[prop], value)
			}
		}
	}

	wide := p408MediaBlocks(chrome, "(min-width: 900px)")
	wideCard, ok := ruleBySelector(wide, ".notice-card")
	if !ok {
		t.Fatal("the 900px block has no .notice-card rule")
	}
	wd := declsOf(wideCard.declBody)
	if wd["bottom"] != "var(--space-lg)" || wd["right"] != "auto" {
		t.Errorf("the 900px card must sit at bottom var(--space-lg) with right auto, got %q and %q", wd["bottom"], wd["right"])
	}
	wideHeights := regexp.MustCompile(`max-height:\s*([^;]+);`).FindAllStringSubmatch(wideCard.declBody, -1)
	if len(wideHeights) != 2 || !strings.Contains(wideHeights[0][1], "var(--disclaimer-bar-height)") ||
		!strings.Contains(wideHeights[1][1], "var(--disclaimer-bar-height)") {
		t.Errorf("the 900px card needs two max-height forms that subtract the bar height, found %v", wideHeights)
	}

	forced := p408MediaBlocks(chrome, "(forced-colors: active)")
	fc, ok := ruleBySelector(forced, ".notice-card")
	if !ok || declsOf(fc.declBody)["border"] != "1px solid CanvasText" {
		t.Error("the forced colors block must give .notice-card a 1px solid CanvasText border")
	}

	for _, r := range rules {
		if !strings.Contains(r.selectorHead, ".notice-card") {
			continue
		}
		got := declsOf(r.declBody)
		for _, banned := range []string{"transition", "animation"} {
			if _, has := got[banned]; has {
				t.Errorf("%q must not declare %s; the card appears with no fade", r.selectorHead, banned)
			}
		}
	}
}

// TestSlowNoticeScript pins the head script: plain text only, no store, no
// wait for the document parsed event, no newer request API.
func TestSlowNoticeScript(t *testing.T) {
	raw, err := StaticFS.ReadFile("static/js/slow-notice.js")
	if err != nil {
		t.Fatal(err)
	}
	full := string(raw)
	for _, dash := range []string{string(rune(0x2014)), string(rune(0x2013))} {
		if strings.Contains(full, dash) {
			t.Errorf("slow-notice.js contains a dash character %q", dash)
		}
	}
	code := stripJSComments(full)
	p413MustContain(t, "slow-notice.js", code,
		"'use strict'", "SLOW_NOTICE_MS = 6000", "SLOW_SENTENCE = 'Taking long? Open the low data view.'",
		"MutationObserver", "attributeFilter: ['data-feed-requested']", "data-feed-state",
		"typeof window.Pinalert === 'undefined'", "typeof window.L === 'undefined'",
		"getElementById('notice-card')", "getElementById('notice-slow')", "getElementById('notice-live')",
		".textContent = SLOW_SENTENCE", ".notice-card__row:not([hidden])")
	if m := regexp.MustCompile(`Pinalert\.\w`).FindString(code); m != "" {
		t.Errorf("slow-notice.js must never call the app store, found %q", m)
	}
	p413MustNotContain(t, "slow-notice.js", code, "DOMContentLoaded", "AbortSignal", "fetch(")
}
