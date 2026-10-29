package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file pins the markup, the script and the stylesheet of the Official
// alerts page of Phase 5 plan 09 (ALERT-01, ALERT-02, ALERT-04, ROBUST-06,
// D-06 to D-10, UI-SPEC Surface 1). The behaviour of the script in every state
// is pinned by official_alerts_test.mjs; the contrast of the level chips by
// official_contrast_test.go. These tests hold what a static read can hold: the
// template carries the shell and no claim, the script builds no markup from a
// string and fetches in one place, the laptop grid is the UI-SPEC value, every
// display rule on an element that can carry hidden is guarded, and the copy has
// no long dash and promises no sound.

const (
	officialTemplate = "alerts.html.tmpl"
	officialScript   = "js/official-alerts.js"
	officialSheet    = "static/css/official.css"

	// officialLaptopColumns is the UI-SPEC "New components" value: Level, Alert,
	// Issued by, Issued, Until and a 20px chevron column.
	officialLaptopColumns = "112px minmax(0, 2.6fr) minmax(0, 1.2fr) 112px 150px 20px"

	officialAttribution = "Alerts are copied from NDMA SACHET. Pinalert is not an official channel. Hazard events come from the Global Disaster Alert and Coordination System, GDACS."
	officialNoscript    = "Turn on JavaScript to see official alerts, or check your state disaster management authority."
)

// TestOfficialAlertsShell pins the markup the script fills: the ids, the empty
// state blocks all hidden, the stale notice as a status region, the retry
// button, the attribution and the noscript line, and the stylesheet and script
// order.
func TestOfficialAlertsShell(t *testing.T) {
	page := p421ReadTemplate(t, officialTemplate)
	for _, id := range []string{
		"official-page", "official-status", "official-notice", "official-notice-text", "official-section",
		"official-title", "official-districts", "official-list", "official-skeleton",
		"official-empty", "official-empty-title", "official-empty-body",
		"official-unavailable", "official-unavailable-title", "official-unavailable-body",
		"official-noarea", "official-noarea-title", "official-noarea-body",
		"official-error", "official-error-title", "official-error-body", "official-retry", "official-attribution",
	} {
		if got := strings.Count(page, `id="`+id+`"`); got != 1 {
			t.Errorf("alerts.html.tmpl has %d elements with id %q, want 1", got, id)
		}
	}

	// Every block the script shows or hides starts hidden, so the page without
	// script shows only the noscript line and the attribution.
	for _, id := range []string{
		"official-notice", "official-section", "official-skeleton", "official-empty",
		"official-unavailable", "official-noarea", "official-error",
	} {
		tag := bellTagWith(t, page, `id="`+id+`"`)
		if !strings.Contains(tag, " hidden") {
			t.Errorf("#%s does not start hidden: %s", id, tag)
		}
	}
	if tag := bellTagWith(t, page, `id="official-notice"`); !strings.Contains(tag, `role="status"`) {
		t.Errorf("the stale notice is not a status region: %s", tag)
	}
	// The status line is plain text, not a live region (UI-SPEC 1a: no chatter).
	if tag := bellTagWith(t, page, `id="official-status"`); strings.Contains(tag, "aria-live") || strings.Contains(tag, "role=") {
		t.Errorf("the status line is a live region: %s", tag)
	}
	if tag := bellTagWith(t, page, `id="official-page"`); !strings.Contains(tag, `data-account-id="{{.AccountID}}"`) || strings.Contains(tag, "data-api-base") || strings.Contains(tag, "data-fixed") {
		t.Errorf("the page root must keep data-account-id and carry no api base or fixed point: %s", tag)
	}
	if tag := bellTagWith(t, page, `id="official-retry"`); !strings.Contains(tag, `type="button"`) || !strings.Contains(page, ">Try again</button>") {
		t.Errorf("the retry control is not a typed button named Try again: %s", tag)
	}

	p13MustContain(t, "alerts.html.tmpl", page,
		officialAttribution, officialNoscript, "<noscript>",
		`<title>Pinalert: Official alerts</title>`, `<h1 class="place-head">Official alerts</h1>`)

	p421InOrder(t, "stylesheet order", page,
		`/static/css/places.css?v={{.AssetVersion}}`,
		`/static/css/official.css?v={{.AssetVersion}}`,
		`/static/css/phone.css?v={{.AssetVersion}}`)
	p421InOrder(t, "script order", page,
		`/static/js/updates.js?v={{.AssetVersion}}`,
		`/static/js/position.js?v={{.AssetVersion}}`,
		`/static/js/official-alerts.js?v={{.AssetVersion}}`)
	if strings.Count(page, "official-alerts.js") != 1 || strings.Count(page, "official.css") != 1 {
		t.Errorf("alerts.html.tmpl must link official.css and official-alerts.js once each")
	}
	for _, tag := range []string{`official-alerts.js?v={{.AssetVersion}}" defer`, `position.js?v={{.AssetVersion}}" defer`} {
		if !strings.Contains(page, tag) {
			t.Errorf("alerts.html.tmpl lacks a deferred script %q", tag)
		}
	}
	// The list is built by the script: the template draws no list or table.
	for _, banned := range []string{"<ul", "<ol", "<table", "data-count"} {
		if strings.Contains(strings.ToLower(page), banned) {
			t.Errorf("alerts.html.tmpl holds %q; the script builds the list", banned)
		}
	}
}

// TestOfficialAlertsNoClaimInTemplate pins T-05-47: every state sentence is
// written by the script from the server's state, so the template (comments
// included) holds no phrase that says nothing is in force or when it was last
// checked.
func TestOfficialAlertsNoClaimInTemplate(t *testing.T) {
	lower := strings.ToLower(p421ReadTemplate(t, officialTemplate))
	for _, banned := range []string{
		"no" + " alerts", "no official" + " alerts", "no" + " warnings", "all" + " clear",
		"nothing in" + " force", "in" + " force", "last" + " checked", "0" + " alerts",
		"not connected", "none in" + " force", "nothing to" + " report",
	} {
		if strings.Contains(lower, banned) {
			t.Errorf("alerts.html.tmpl holds %q, a claim only the script may write from the server's state", banned)
		}
	}
	if strings.Contains(lower, "disclaimer_bar.html.tmpl") {
		t.Errorf("alerts.html.tmpl calls the notice strip partial")
	}
}

// TestOfficialAlertsScriptSinks pins T-05-46 and T-05-50: no markup is built
// from a string, there is exactly one place that fetches, and the only timer is
// the minute tick of the status line.
func TestOfficialAlertsScriptSinks(t *testing.T) {
	raw := p421ReadStatic(t, officialScript)
	code, bad := scanJSComments(raw)
	if bad {
		t.Fatalf("official-alerts.js has an unterminated string or comment, or a quote inside a regular expression")
	}
	for _, banned := range []string{"innerHTML", "insertAdjacentHTML", "outerHTML", "document.write", "eval(", "new Function", "createContextualFragment", "DOMParser"} {
		if strings.Contains(code, banned) {
			t.Errorf("official-alerts.js uses %s; feed text enters the page through textContent only (D-10)", banned)
		}
	}
	if got := strings.Count(code, "fetch("); got != 1 {
		t.Errorf("official-alerts.js calls fetch( %d times, want exactly 1 (one request per load and per Try again)", got)
	}
	if got := strings.Count(code, "setInterval("); got != 1 {
		t.Errorf("official-alerts.js calls setInterval( %d times, want exactly 1 (the minute tick)", got)
	}
	if strings.Contains(code, "setTimeout(") {
		t.Errorf("official-alerts.js uses setTimeout, which could become a refetch loop")
	}
	p13MustContain(t, "official-alerts.js", code,
		"STATUS_TICK_MS = 60000", "NEARBY_CENTRE_KEY = 'pinalert.nearbyCentre'", "PinalertApiBase",
		"/official/alerts", "PinalertPosition", "window.PinalertOfficialAlerts",
		"function areaSource", "function load", "function render", "function renderStatus",
		"function renderGroups", "function renderRow", "function toggleDetails",
		"function relativeIssued", "function untilText", "function clockText", "function langCode")
	// The honesty sentence is written in one place, behind the one state test.
	if got := strings.Count(code, "No official alerts in force for your area"); got != 1 {
		t.Errorf("the claim sentence appears %d times in official-alerts.js, want 1", got)
	}
	// Sessions of other accounts are never read (T-05-48).
	p13MustContain(t, "official-alerts.js", code, "data-account-id")
}

// officialRule returns the merged rule of one selector head in official.css.
func officialRule(t *testing.T, media, head string) p16CSSRule {
	t.Helper()
	return p31mRule(t, officialSheet, media, head)
}

// TestOfficialAlertsLaptopGridAndPhoneCard pins the arrangement: the laptop
// row is the six column grid of UI-SPEC, the phone card is a bordered 12px
// rounded card, the level chips follow the one level mapping, and every
// display rule on an element that can carry hidden is guarded.
func TestOfficialAlertsLaptopGridAndPhoneCard(t *testing.T) {
	raw := p421ReadStatic(t, "css/official.css")
	css := stripCSSComments(raw)

	// The laptop row and the column head row share the one column template.
	if got := strings.Count(css, officialLaptopColumns); got != 2 {
		t.Errorf("official.css holds the laptop column template %d times, want 2 (the row and the head row)", got)
	}
	p16MustDecl(t, ".official-row", officialRule(t, "", ".official-row"), map[string]string{
		"display": "grid", "grid-template-columns": officialLaptopColumns, "gap": "var(--space-md)", "align-items": "center",
		"padding": "var(--space-md) var(--space-lg)", "min-height": "var(--touch-target-min)",
	})
	p16MustDecl(t, ".official-columns", officialRule(t, "", ".official-columns"), map[string]string{
		"display": "grid", "grid-template-columns": officialLaptopColumns,
		"padding": "var(--space-sm) var(--space-lg)", "font-size": "var(--font-size-caption)", "text-transform": "uppercase",
	})

	const phone = "@media (max-width: 899.98px)"
	card := officialRule(t, phone, ".official-item")
	p16MustDecl(t, "phone .official-item", card, map[string]string{
		"border": "1px solid var(--color-border)", "border-radius": "12px",
	})
	p16MustDecl(t, "phone .official-card", officialRule(t, phone, ".official-row.official-card"), map[string]string{
		"padding": "var(--space-md)", "row-gap": "var(--space-12)",
	})
	// The phone card ends with Issued and Until on one line over a hairline.
	for _, head := range []string{".official-card .official-row__issued", ".official-card .official-row__until"} {
		p16MustDecl(t, head, officialRule(t, phone, head), map[string]string{"border-top": "1px solid var(--color-border)"})
	}

	// The level mapping (UI-SPEC Color).
	p16MustDecl(t, "extreme chip", officialRule(t, "", ".official-chip--extreme"), map[string]string{
		"background": "var(--color-severity-critical-bg)", "border": "1px solid var(--color-severity-critical)",
	})
	p16MustDecl(t, "severe chip", officialRule(t, "", ".official-chip--severe"), map[string]string{
		"background": "var(--color-severity-medium-bg)", "border": "1px solid var(--color-severity-medium)",
	})
	p16MustDecl(t, "neutral chip", officialRule(t, "", ".official-chip--neutral"), map[string]string{
		"background": "var(--color-surface)", "border": "1px solid var(--color-border)",
	})
	p16MustDecl(t, "extreme icon", officialRule(t, "", ".official-chip--extreme .official-chip__icon"), map[string]string{
		"background-color": "var(--color-tone-bad)", "-webkit-mask-image": `url("/static/icons/octagon-alert.svg")`, "mask-image": `url("/static/icons/octagon-alert.svg")`,
	})
	p16MustDecl(t, "severe icon", officialRule(t, "", ".official-chip--severe .official-chip__icon"), map[string]string{
		"background-color": "var(--color-tone-caution)", "-webkit-mask-image": `url("/static/icons/triangle-alert.svg")`, "mask-image": `url("/static/icons/triangle-alert.svg")`,
	})
	p16MustDecl(t, "neutral icon", officialRule(t, "", ".official-chip--neutral .official-chip__icon"), map[string]string{
		"background-color": "var(--color-text)", "-webkit-mask-image": `url("/static/icons/circle-alert.svg")`, "mask-image": `url("/static/icons/circle-alert.svg")`,
	})
	p16MustDecl(t, "badge icon", officialRule(t, "", ".official-badge__icon"), map[string]string{
		"-webkit-mask-image": `url("/static/icons/shield-check.svg")`, "mask-image": `url("/static/icons/shield-check.svg")`,
	})
	p16MustDecl(t, "chevron", officialRule(t, "", ".official-row__chevron"), map[string]string{
		"-webkit-mask-image": `url("/static/icons/chevron-down.svg")`, "mask-image": `url("/static/icons/chevron-down.svg")`,
		"width": "20px", "height": "20px",
	})
	p16MustDecl(t, "open chevron", officialRule(t, "", `.official-row[aria-expanded="true"] .official-row__chevron`), map[string]string{
		"transform": "rotate(180deg)",
	})
	p16MustDecl(t, "reduced motion chevron", officialRule(t, "@media (prefers-reduced-motion: reduce)", ".official-row__chevron"), map[string]string{
		"transition": "none",
	})
	for _, head := range []string{".official-badge", ".demo-tag"} {
		p16MustDecl(t, head, officialRule(t, "", head), map[string]string{
			"min-height": "24px", "border-radius": "4px", "border": "1px solid var(--color-text)", "background": "var(--color-bg)",
		})
	}

	// Rows are 44px or taller on both sizes.
	if got := officialRule(t, "", ".official-row").Decls["min-height"]; got != "var(--touch-target-min)" {
		t.Errorf(".official-row min-height = %q, want the touch target token", got)
	}
	// Try again is the shared outlined button, which is already 44px.
	page := p421ReadTemplate(t, officialTemplate)
	if tag := bellTagWith(t, page, `id="official-retry"`); !strings.Contains(tag, `class="btn`) {
		t.Errorf("Try again is not the shared .btn button: %s", tag)
	}

	// Hidden guard: a rule that gives an element that can carry hidden a display
	// must say :not([hidden]), and the shared rules that would beat the hidden
	// attribute (.empty-state flex, .place-section grid) are answered by id.
	canHide := []string{".official-notice", ".official-details", ".official-skeleton", ".official-state", ".official-section", ".official-list"}
	inspected := 0
	for _, r := range p16ParseCSS(officialSheet, raw) {
		display, has := r.Decls["display"]
		if !has || display == "none" {
			continue
		}
		for _, sel := range p16HeadParts(r.Head) {
			if strings.HasSuffix(sel, "::before") || strings.HasSuffix(sel, "::after") {
				continue
			}
			for _, name := range canHide {
				if regexp.MustCompile(regexp.QuoteMeta(name) + `($|[^a-z_-])`).MatchString(sel) {
					inspected++
					if !strings.Contains(sel, ":not([hidden])") {
						t.Errorf("official.css: %q sets display: %s without :not([hidden])", sel, display)
					}
				}
			}
		}
	}
	if inspected < 3 {
		t.Errorf("inspected %d guarded display rules, want at least 3", inspected)
	}
	for _, id := range []string{"official-section", "official-empty", "official-unavailable", "official-noarea", "official-error"} {
		p16MustDecl(t, "#"+id+"[hidden]", officialRule(t, "", "#"+id+"[hidden]"), map[string]string{"display": "none"})
	}

	// Plain stylesheet hygiene.
	for _, banned := range []string{"!important", "rgb(", "rgba(", "hsl(", "999px", "opacity"} {
		if strings.Contains(css, banned) {
			t.Errorf("official.css contains %q", banned)
		}
	}
	if m := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindString(strings.ReplaceAll(css, "#official-", "")); m != "" {
		t.Errorf("official.css contains the hex colour %q", m)
	}
	sizes := map[string]bool{
		"var(--font-size-label)": true, "var(--font-size-body)": true, "var(--font-size-heading)": true,
		"var(--font-size-display)": true, "var(--font-size-caption)": true,
	}
	weights := map[string]bool{"var(--font-weight-regular)": true, "var(--font-weight-semibold)": true}
	for _, r := range parseCSSRules(raw) {
		decls := declsOf(r.declBody)
		if v, has := decls["font-size"]; has && !sizes[v] {
			t.Errorf("%q: font-size %q is not a size token", r.selectorHead, v)
		}
		if v, has := decls["font-weight"]; has && !weights[v] {
			t.Errorf("%q: font-weight %q is not a weight token", r.selectorHead, v)
		}
	}
	if !strings.Contains(css, "@media (forced-colors: active)") || !strings.Contains(css, "CanvasText") {
		t.Errorf("official.css lacks a forced colors block that keeps the chip borders as CanvasText")
	}
}

// TestOfficialAlertsCopy pins the copy of UI-SPEC P5-C2 to P5-C12 in the
// script, and the house rules on every file this plan wrote: no long dash, no
// wording that promises a sound, no underlined text link.
func TestOfficialAlertsCopy(t *testing.T) {
	script := p421ReadStatic(t, officialScript)
	code := stripJSComments(script)
	p13MustContain(t, "official-alerts.js copy", code,
		"Last checked ", "Not checked yet", "Checking for official alerts", "Not checked",
		"No official alerts in force for your area",
		"Nothing is in force for where you are or for your home area. ",
		"Check your state disaster management authority for anything else.",
		"We could not check for alerts since ", "Last known alerts", "As of ", "In force now",
		"Official alerts could not be checked",
		"Pinalert could not reach the official feed since ",
		"Check your state disaster management authority for warnings.",
		"Pinalert has not been able to check the official feed yet.",
		"Official alerts need your area",
		"Open Nearby and allow location, or add a home area in Settings, then come back.",
		"Couldn't load official alerts", "Check your connection and try again.",
		"Issued by", "Issued ", "Until ", "about ", "Just now", " min ago", "1 hour ago", " hours ago",
		" alerts like this", " alerts like this are in force. This shows the newest.",
		"What it says", "What to do", "Original text", "Areas", "Source", "No end time was given.",
		"Original language", "Demo", "Level", "Alert", "Issued", "Until",
	)

	files := map[string]string{
		"alerts.html.tmpl":   p421ReadTemplate(t, officialTemplate),
		"official-alerts.js": script,
		"official.css":       p421ReadStatic(t, "css/official.css"),
		"octagon-alert.svg":  p421ReadStatic(t, "icons/octagon-alert.svg"),
		"triangle-alert.svg": p421ReadStatic(t, "icons/triangle-alert.svg"),
	}
	for name, text := range files {
		for i, line := range strings.Split(text, "\n") {
			if strings.ContainsFunc(line, func(r rune) bool { return r == 0x2013 || r == 0x2014 }) {
				t.Errorf("%s:%d holds an em or en dash: %q", name, i+1, strings.TrimSpace(line))
			}
		}
	}
	// Every notification is silent; nothing on this page promises a sound.
	for _, name := range []string{"alerts.html.tmpl", "official-alerts.js"} {
		lower := strings.ToLower(stripJSComments(files[name]))
		for _, banned := range []string{"buzz", "vibrat", "sound"} {
			if strings.Contains(lower, banned) {
				t.Errorf("%s holds %q; no wording may promise a sound or a vibration", name, banned)
			}
		}
		if regexp.MustCompile(`\bring\b`).MatchString(lower) {
			t.Errorf("%s holds the word ring; no wording may promise a sound", name)
		}
	}
	if strings.Contains(stripCSSComments(files["official.css"]), "underline") {
		t.Errorf("official.css draws an underline; rows are buttons, never underlined links")
	}
	// The licence line the other icons carry.
	ref := p421ReadStatic(t, "icons/search.svg")
	refFirst, _, _ := strings.Cut(ref, "\n")
	for _, name := range []string{"octagon-alert.svg", "triangle-alert.svg"} {
		first, _, _ := strings.Cut(files[name], "\n")
		if first != refFirst {
			t.Errorf("%s first line %q differs from search.svg's licence line %q", name, first, refFirst)
		}
	}
}

// TestOfficialRowHoverKeepsMutedTextReadable pins the Phase 5 review fix
// WR-T06 (added 2026-10-29): a hovered laptop row fills with --color-surface,
// where the muted grey is 4.34 to 1 in the light theme, under 4.5. The area and
// "N alerts like this" lines take the normal text colour while the row is
// hovered, so they clear 4.5 to 1 in light and dark.
func TestOfficialRowHoverKeepsMutedTextReadable(t *testing.T) {
	const hover = "@media (hover: hover)"
	p16MustDecl(t, "hovered row", officialRule(t, hover, ".official-row:hover"), map[string]string{
		"background": "var(--color-surface)",
	})
	for _, head := range []string{".official-row:hover .official-row__area", ".official-row:hover .official-row__grouped"} {
		p16MustDecl(t, head, officialRule(t, hover, head), map[string]string{"color": "var(--color-text)"})
	}
	sheet := p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css"))
	themes := map[string]map[string]string{
		"light": p16Rule(t, sheet, "", ":root").Decls,
		"dark":  p16Rule(t, sheet, "", `:root[data-theme="dark"]`).Decls,
	}
	for name, tok := range themes {
		if got := wcagContrastRatio(t, tok["--color-text"], tok["--color-surface"], name+" hovered row"); got < 4.5 {
			t.Errorf("%s: the hovered row's area text is %.2f to 1, want at least 4.5", name, got)
		}
	}
}

// TestOfficialHazardLineSitsCloseOnBothSizes pins the Phase 5 review fix IN-T05
// (added 2026-10-29): the GDACS hazard status line sits 4px under the first
// status line on the phone and on the laptop. On the laptop the first line's
// 8px bottom padding is dropped when the hazard line follows, so the gap is the
// hazard line's own 4px there too, not 12px.
func TestOfficialHazardLineSitsCloseOnBothSizes(t *testing.T) {
	p16MustDecl(t, "hazard line", officialRule(t, "", ".official-status.official-status--hazards:not([hidden])"),
		map[string]string{"padding-top": "var(--space-xs)"})
	p16MustDecl(t, "phone first status line", officialRule(t, "", ".official-status"),
		map[string]string{"padding": "var(--space-12) var(--space-md) 0"})
	p16MustDecl(t, "laptop first status line before the hazard line",
		officialRule(t, "@media (min-width: 900px)", ".official-status:has(+ .official-status--hazards:not([hidden]))"),
		map[string]string{"padding-bottom": "0"})
}
