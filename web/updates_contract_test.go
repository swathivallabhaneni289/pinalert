package web

// Phase 3.2 plan 08: the bell's count and list, the Updates rows of My
// activity and the one stylesheet of the phase. These tests read the shipped
// templates, stylesheet and script from the embedded trees, never a copy, so
// they can only drift with the product.

import (
	"regexp"
	"strings"
	"testing"
)

// updatesCSS returns the comment stripped text of updates.css.
func updatesCSS(t *testing.T) string {
	t.Helper()
	return stripCSSComments(p421ReadStatic(t, "css/updates.css"))
}

// updatesCSSRules parses updates.css into rules with their media preludes.
func updatesCSSRules(t *testing.T) []p16CSSRule {
	t.Helper()
	return p16ParseCSS("static/css/updates.css", p421ReadStatic(t, "css/updates.css"))
}

// TestBellCountMarkup pins UI-SPEC A1 and A4: the bell holds a hidden count
// span that is not announced, the poll interval is a literal on the button, and
// the list holds its body (loading, error, stale line, rows, the empty line)
// between the head and the footer, each once and in that order.
func TestBellCountMarkup(t *testing.T) {
	bar := p421ReadTemplate(t, "app_bar.html.tmpl")

	trigger := bellTagWith(t, bar, `id="bell-trigger"`)
	if !strings.Contains(trigger, `data-poll-interval-ms="30000"`) {
		t.Errorf("the bell button %s lacks data-poll-interval-ms=\"30000\"", trigger)
	}
	if n := strings.Count(bar, `data-poll-interval-ms="30000"`); n != 1 {
		t.Errorf("the poll interval literal appears %d times, want 1", n)
	}
	if !strings.Contains(trigger, `aria-label="Updates"`) {
		t.Errorf("the bell button keeps aria-label=\"Updates\" as its first name: %s", trigger)
	}
	count := bellTagWith(t, bar, `id="bell-count"`)
	for _, want := range []string{`<span`, `class="app-topbar__count"`, `aria-hidden="true"`, ` hidden>`} {
		if !strings.Contains(count, want) {
			t.Errorf("the count %s lacks %s", count, want)
		}
	}
	if strings.Contains(bar, "aria-live") || strings.Contains(bar, `role="status"`) {
		t.Errorf("the bell carries a live region; the number is in the button name only (Pitfall 11)")
	}
	p421InOrder(t, "the count sits inside the bell button", bar,
		`id="bell-trigger"`, `app-icon--bell`, `id="bell-count"`, `</button>`, `id="bell-list"`)

	p421InOrder(t, "bell list body", bar,
		`id="bell-list"`, `bell-list__head`, `aria-label="Close"`,
		`id="bell-body" class="bell-list__body"`,
		`id="bell-loading" class="bell-list__loading" hidden`,
		`id="bell-error" class="bell-list__state" hidden`,
		`Couldn't load updates`, `Check your connection and try again.`, `class="bell-list__retry"`,
		`id="bell-stale" class="bell-list__stale" hidden`, `Couldn't refresh. Showing the last list.`, `class="bell-list__retry"`,
		`id="bell-rows" class="bell-list__rows" hidden`,
		`class="bell-list__empty">No updates yet`,
		`class="bell-list__all"`, `See all updates`)
	if n := strings.Count(bar, `class="skeleton-row"`); n != 3 {
		t.Errorf("the loading block holds %d skeleton rows, want 3", n)
	}
	if n := strings.Count(bar, `class="bell-list__retry"`); n != 2 {
		t.Errorf("the list has %d Try again buttons, want 2 (error and stale)", n)
	}
	if n := strings.Count(bar, "Try again"); n != 2 {
		t.Errorf("Try again appears %d times in the bar, want 2", n)
	}
	for _, dash := range []string{"\u2013", "\u2014"} {
		if strings.Contains(bar, dash) {
			t.Errorf("app_bar.html.tmpl holds the dash %q", dash)
		}
	}
}

// updatesHiddenCapable are the classes of elements that carry the hidden
// attribute at some point, so any display rule naming them needs the guard.
var updatesHiddenCapable = []string{
	"bell-list__loading", "bell-list__state", "bell-list__stale", "bell-list__rows", "bell-list__empty",
	"app-topbar__count", "mute-control", "muted-row", "activity-hub__new",
}

// updatesAlertFillHeads are the only heads allowed to paint --color-alert-fill:
// the bell count, the unread dot and the hub tag (UI-SPEC Color, accent list).
var updatesAlertFillHeads = map[string]bool{
	".app-topbar__count:not([hidden])": true,
	".urow--new .urow__dot":            true,
	".activity-hub__new:not([hidden])": true,
}

// TestUpdatesCSSTokensOnly holds updates.css to the house rules: no dash, no
// important flag, no raw colour, no pill radius, font sizes and weights from
// the tokens, system colours only in the forced colours block, the hidden guard
// on every display rule that can meet a hidden element, and the alert fill on
// its three heads only.
func TestUpdatesCSSTokensOnly(t *testing.T) {
	raw := p421ReadStatic(t, "css/updates.css")
	for _, dash := range []string{"\u2014", "\u2013"} {
		if strings.Contains(raw, dash) {
			t.Errorf("updates.css contains a dash character %q", dash)
		}
	}
	text := stripCSSComments(raw)
	for _, banned := range []string{"!important", "rgb(", "rgba(", "hsl(", "999px", "opacity"} {
		if strings.Contains(text, banned) {
			t.Errorf("updates.css contains %q", banned)
		}
	}
	if m := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindString(text); m != "" {
		t.Errorf("updates.css contains the hex colour %q", m)
	}

	sizes := map[string]bool{
		"var(--font-size-caption)": true, "var(--font-size-label)": true,
		"var(--font-size-body)": true, "var(--font-size-heading)": true, "var(--font-size-display)": true,
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

	// System colour keywords belong inside the forced colors block only.
	outside := text
	spans := p408MediaSpans(text, "(forced-colors: active)")
	if len(spans) != 1 {
		t.Errorf("updates.css holds %d forced colors blocks, want 1", len(spans))
	}
	for i := len(spans) - 1; i >= 0; i-- {
		outside = outside[:spans[i][0]] + outside[spans[i][2]:]
	}
	if m := regexp.MustCompile(`\b(CanvasText|LinkText|GrayText|ButtonFace|ButtonText|Canvas|Highlight)\b`).FindString(outside); m != "" {
		t.Errorf("system colour %s appears outside a forced colors block", m)
	}

	for _, r := range updatesCSSRules(t) {
		for _, part := range p16HeadParts(r.Head) {
			if _, has := r.Decls["display"]; has {
				for _, class := range updatesHiddenCapable {
					// A class name followed by a name character is another class
					// (.muted-row__title is not .muted-row).
					named := regexp.MustCompile(`\.` + regexp.QuoteMeta(class) + `($|[^A-Za-z0-9_-])`).MatchString(part)
					if named && !strings.Contains(part, ":not([hidden])") {
						t.Errorf("%q declares display without :not([hidden]) but %s can carry hidden", r.Head, class)
					}
				}
			}
			if v := r.Decls["background"]; strings.Contains(v, "--color-alert-fill") && !updatesAlertFillHeads[part] {
				t.Errorf("%q paints --color-alert-fill; only the bell count, the unread dot and the hub tag may (UI-SPEC Color)", part)
			}
		}
	}
	for head := range updatesAlertFillHeads {
		found := false
		for _, r := range updatesCSSRules(t) {
			for _, part := range p16HeadParts(r.Head) {
				if part == head && strings.Contains(r.Decls["background"], "--color-alert-fill") {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("no rule %q paints the alert fill", head)
		}
	}
}

// updatesTouchControls are the controls of this plan and the ones it styles for
// later plans; each must reach 44px through a rule of updates.css that names it.
var updatesTouchControls = []string{
	".bell-row", ".bell-list__retry", ".urow__link", ".mute-control", ".place-select",
}

// TestUpdatesControlsMeetTouchTarget pins UI-SPEC Spacing: every row that is a
// link or control, the mute control and the select are at least 44px high.
func TestUpdatesControlsMeetTouchTarget(t *testing.T) {
	rules := updatesCSSRules(t)
	tokens := layoutLengthTokens(t)
	for _, control := range updatesTouchControls {
		best := 0.0
		named := 0
		for _, r := range rules {
			if strings.Contains(r.Media, "forced-colors") {
				continue
			}
			for _, raw := range p16HeadParts(r.Head) {
				compound, ok := layoutLastCompound(raw)
				if !ok || !touchRuleNames(compound, control) {
					continue
				}
				named++
				for _, prop := range []string{"min-height", "height"} {
					if value, has := r.Decls[prop]; has {
						if px := touchValue(value, tokens); px > best {
							best = px
						}
					}
				}
			}
		}
		if named == 0 {
			t.Errorf("%s: no rule of updates.css names this control", control)
			continue
		}
		if best < 44 {
			t.Errorf("%s: the largest height or min-height declared is %.0fpx, want 44px or more", control, best)
		}
	}
	// The head and the footer of the bell list are 48px.
	for _, head := range []string{".bell-list__head", ".bell-list__all"} {
		found := false
		for _, r := range rules {
			if r.Head == head && touchValue(r.Decls["min-height"], tokens) >= 48 {
				found = true
			}
		}
		if !found {
			t.Errorf("%s has no min-height of 48px in updates.css (UI-SPEC D12)", head)
		}
	}
}

// updatesBellPages are the four templates that draw the top bar.
var updatesBellPages = []string{"index.html.tmpl", "profile.html.tmpl", "settings.html.tmpl", "alerts.html.tmpl"}

// TestUpdatesLoadedOnTheFourBellPages pins the page wiring: each page links the
// stylesheet once before phone.css and loads the script once, after app-bar.js,
// so the click listener that reads the open state runs after the one that opens.
func TestUpdatesLoadedOnTheFourBellPages(t *testing.T) {
	for _, name := range updatesBellPages {
		tmpl := p421ReadTemplate(t, name)
		if n := strings.Count(tmpl, "/static/css/updates.css"); n != 1 {
			t.Errorf("%s links updates.css %d times, want 1", name, n)
		}
		if n := strings.Count(tmpl, "/static/js/updates.js"); n != 1 {
			t.Errorf("%s loads updates.js %d times, want 1", name, n)
		}
		p421InOrder(t, name+" stylesheet order", tmpl,
			`/static/css/updates.css?v={{.AssetVersion}}`, `/static/css/phone.css?v={{.AssetVersion}}`)
		p421InOrder(t, name+" script order", tmpl,
			`/static/js/app-bar.js?v={{.AssetVersion}}" defer`, `/static/js/updates.js?v={{.AssetVersion}}" defer`)
	}
}

// TestUpdatesScriptContract pins D-04 and the boundary rules of the bell
// script on its comment stripped code: one request call here, two in app.js and
// none in app-bar.js; the two addresses, the refresh event and the page events;
// text written only with textContent; no markup parsing sink; no use of the
// Pinalert global; no live region; no dash anywhere.
func TestUpdatesScriptContract(t *testing.T) {
	raw := p421ReadStatic(t, "js/updates.js")
	code, bad := scanJSComments(raw)
	if bad {
		t.Fatal("updates.js holds an unterminated string, template or comment, or a regular expression with a quote")
	}
	if n := strings.Count(code, "fetch("); n != 1 {
		t.Errorf("updates.js holds %d request calls, want exactly 1 (one helper, both addresses)", n)
	}
	if n := strings.Count(stripJSComments(p421ReadStatic(t, "js/app.js")), "fetch("); n != 2 {
		t.Errorf("app.js holds %d fetch( calls, want exactly 2 (D-04: the bell does not touch it)", n)
	}
	if n := strings.Count(stripJSComments(p421ReadStatic(t, "js/app-bar.js")), "fetch("); n != 0 {
		t.Errorf("app-bar.js holds %d fetch( calls, want none", n)
	}
	for _, want := range []string{
		"/api/updates", "/api/updates/read", "pinalert:updates-refresh", "textContent",
		"visibilitychange", "pageshow", "data-poll-interval-ms", "setAttribute", "createElement",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("updates.js lacks %q", want)
		}
	}
	for _, banned := range []string{
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "DOMParser", "createContextualFragment",
		"eval(", "new Function",
		"aria-live", "aria-atomic", `role="status"`, `role="alert"`,
		"location.href", "location.assign", "location.replace", "window.open",
	} {
		if strings.Contains(code, banned) {
			t.Errorf("updates.js uses %q", banned)
		}
	}
	if regexp.MustCompile(`\bPinalert\b`).MatchString(code) {
		t.Errorf("updates.js uses the Pinalert global; it must work on pages where app.js does not run")
	}
	for _, dash := range []string{"–", "—"} {
		if strings.Contains(raw, dash) {
			t.Errorf("updates.js holds the dash %q", dash)
		}
	}
	// A row address is the fixed path plus a checked id and nothing else.
	if !strings.Contains(code, `'/?report='`) || !strings.Contains(code, "ID_PATTERN") {
		t.Errorf("updates.js must build the row address from '/?report=' and the checked id")
	}
}
