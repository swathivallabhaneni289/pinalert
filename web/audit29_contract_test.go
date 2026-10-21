package web

import (
	"strings"
	"testing"
)

// Plan 03.4-29 pins the fixes of the visual audit (03.4-UI-REVIEW.md). Each
// test names its finding. The browser measurements before and after are in
// 03.4-29-SUMMARY.md; these tests keep the declarations that produced them.

// TestAudit29PopupFitsTheMapBelowTheTopBar (finding 1): from 900px the opened
// report scrolls inside a box that is one top bar shorter than the viewport,
// because the map starts below the top bar. At 1280x800 the old box was 640px
// in a 691px map and ran off its top.
func TestAudit29PopupFitsTheMapBelowTheTopBar(t *testing.T) {
	laptop := p31mRule(t, "static/css/trust.css", "@media (min-width: 900px)", ".leaflet-container .leaflet-popup-content")
	got := p19NoSpace(laptop.Decls["max-height"])
	for _, want := range []string{"100dvh", "-var(--map-tools-height)", "-var(--disclaimer-bar-height)", "-var(--app-topbar-height)"} {
		if !strings.Contains(got, want) {
			t.Errorf("laptop popup max-height %q lacks %q", laptop.Decls["max-height"], want)
		}
	}
}

// TestAudit29FlagSitsOneMediumSpaceUnderMarkResolved (finding 6): in the opened
// report the Flag this report button sits 16px under the vote block. The Low
// data rows keep the chrome.css margin.
func TestAudit29FlagSitsOneMediumSpaceUnderMarkResolved(t *testing.T) {
	r := p31mRule(t, "static/css/trust.css", "", ".leaflet-container .popup-card__actions .flag-control:not([hidden])")
	if got := r.Decls["margin-top"]; got != "var(--space-md)" {
		t.Errorf("popup Flag margin-top is %q, want var(--space-md)", got)
	}
	chrome := p31mRule(t, "static/css/chrome.css", "", ".flag-control:not([hidden])")
	if got := chrome.Decls["margin-top"]; got != "var(--space-xs)" {
		t.Errorf("the shared Flag margin-top is %q, want it unchanged at var(--space-xs)", got)
	}
}

// TestAudit29BackArrowDoesNotCoverTheSearchField (finding 2): while the phone
// detail sheet is open the top bar's place search is hidden, so the round back
// arrow no longer sits on the field.
func TestAudit29BackArrowDoesNotCoverTheSearchField(t *testing.T) {
	css := stripCSSComments(p410ReadStatic(t, "static/css/phone.css"))
	flat := p19NoSpace(css)
	want := p19NoSpace(`body:has(.leaflet-detail-pane .leaflet-popup[data-popup-open="true"]) .app-topbar > .map-search {
    visibility: hidden;
  }`)
	if !strings.Contains(flat, want) {
		t.Errorf("phone.css does not hide the top bar search while the detail sheet is open")
	}
}

// TestAudit29TitlesTakeFocusWithoutARing (findings 3 and 4): the Post a report
// window opens with focus on its title (tabindex -1), not on the Flood tile,
// and a title that only takes programmatic focus draws no outline.
func TestAudit29TitlesTakeFocusWithoutARing(t *testing.T) {
	tmpl := p410ReadTemplate(t, "index.html.tmpl")
	if !strings.Contains(tmpl, `<h2 id="modal-title" class="post-window__title" tabindex="-1">`) {
		t.Errorf("the Post a report title must be a programmatic focus target (tabindex -1)")
	}
	form := p410CodeText(p410ReadStatic(t, "static/js/report-form.js"))
	for _, want := range []string{
		"var windowTitle = windowMode ? document.getElementById('modal-title') : null;",
		"if (windowTitle) {",
		"windowTitle.focus();",
	} {
		if !strings.Contains(form, want) {
			t.Errorf("report-form.js lacks %q", want)
		}
	}
	if strings.Index(form, "windowTitle.focus();") > strings.Index(form, "firstTile.focus();") {
		t.Errorf("the title must be tried before the first tile")
	}
	for _, c := range []struct{ file, head string }{
		{"static/css/window.css", ".app-window__title:focus"},
		{"static/css/modal.css", ".post-window__panel .post-window__title:focus"},
		{"static/css/welcome.css", ".welcome__title:focus"},
	} {
		r := p31mRule(t, c.file, "", c.head)
		if got := r.Decls["outline"]; got != "none" {
			t.Errorf("%s %s outline is %q, want none", c.file, c.head, got)
		}
	}
}

// TestAudit29RowDotsEndAPartNeverStartOne (finding 5): in the report rows the
// separator dot is drawn after the part before it, so a wrapped line never
// starts with a lone dot. No row rule draws the dot with ::before.
func TestAudit29RowDotsEndAPartNeverStartOne(t *testing.T) {
	for _, file := range []string{"static/css/feed.css", "static/css/phone.css"} {
		after := 0
		for _, r := range p16ParseCSS(file, p16ReadStatic(t, file)) {
			if !strings.Contains(r.Head, "#report-list") {
				continue
			}
			for _, part := range p16HeadParts(r.Head) {
				if strings.HasSuffix(part, "::before") && strings.Contains(r.Decls["content"], "00b7") {
					t.Errorf("%s: %q draws the row dot before a part", file, part)
				}
				if strings.HasSuffix(part, "::after") && strings.Contains(r.Decls["content"], "00b7") {
					after++
				}
			}
		}
		if file == "static/css/feed.css" && after < 8 {
			t.Errorf("feed.css draws %d dot heads after a part, want the label, the state, three meta, the distance and the two trust line parts", after)
		}
	}
	second := p31mRule(t, "static/css/feed.css", "", "#report-list .report-row .report-row__trustline-count--second:has(~ .report-row__trustline-word:not([hidden]))::after")
	if !strings.Contains(second.Decls["content"], "00b7") {
		t.Errorf("the second count must draw its dot only when the word is shown")
	}
}

// TestAudit29ReporterLineHasItsOwnLine (finding 13): the reporter words take a
// whole line on every row, so an unconfirmed row never reads "0 disputed New
// reporter".
func TestAudit29ReporterLineHasItsOwnLine(t *testing.T) {
	r := p31mRule(t, "static/css/feed.css", "", "#report-list .report-row .trust-reporter:not([hidden])")
	if got := r.Decls["flex"]; got != "0 0 100%" {
		t.Errorf("reporter line flex is %q, want 0 0 100%%", got)
	}
}

// TestAudit29TextFieldsAndPlaceholdersReadAtThreeToOne (findings 10 and 11):
// the edge of every text field of these pages reads at 3 to 1 or more, and the
// placeholder takes the muted text colour at full opacity.
func TestAudit29TextFieldsAndPlaceholdersReadAtThreeToOne(t *testing.T) {
	for _, c := range []struct{ file, head string }{
		{"static/css/modal.css", "#location-search"},
		{"static/css/modal.css", "#description"},
		{"static/css/modal.css", "#shelter-capacity-status"},
		{"static/css/window.css", ".app-window__field"},
		{"static/css/auth.css", ".login-field"},
	} {
		r := p31mRule(t, c.file, "", c.head)
		if got := r.Decls["border"]; got != "1px solid var(--color-text-muted)" {
			t.Errorf("%s %s border is %q, want the muted text colour", c.file, c.head, got)
		}
	}
	ph := p31mRule(t, "static/css/main.css", "", "::placeholder")
	p16MustDecl(t, "placeholder", ph, map[string]string{"color": "var(--color-text-muted)", "opacity": "1"})
}

// TestAudit29MutedTextLeavesTheGreySurface (finding 7): the sign in card, the
// verify page and the two Post a report map column lines use the normal text
// colour, since muted grey on --color-surface is 4.34 to 1.
func TestAudit29MutedTextLeavesTheGreySurface(t *testing.T) {
	lead := p31mRule(t, "static/css/auth.css", "", ".login-card p")
	if got := lead.Decls["color"]; got != "var(--color-text)" {
		t.Errorf(".login-card p color is %q, want var(--color-text)", got)
	}
	for _, head := range []string{"#coord-readout", "#location-notice"} {
		r := p31mRule(t, "static/css/modal-map.css", "", head)
		if got := r.Decls["color"]; got != "var(--color-text)" {
			t.Errorf("%s color is %q, want var(--color-text)", head, got)
		}
	}
}

// TestAudit29SignInHasOneRhythm (finding 15): 16px between the cards, an opaque
// backing so the drawing never shows in the gap, and an empty error line that
// takes no room.
func TestAudit29SignInHasOneRhythm(t *testing.T) {
	col := p31mRule(t, "static/css/auth.css", "", ".login-column")
	p16MustDecl(t, ".login-column", col, map[string]string{"gap": "var(--space-md)", "background": "var(--color-bg)"})
	p16MustDecl(t, ".login-card p", p31mRule(t, "static/css/auth.css", "", ".login-card p"),
		map[string]string{"margin": "0 0 var(--space-md) 0"})
	p16MustDecl(t, ".auth-error:empty", p31mRule(t, "static/css/auth.css", "", ".auth-error:empty"),
		map[string]string{"min-height": "0", "margin": "0"})
}

// TestAudit29SettingsValueDoesNotShrink (finding 8): "Not set" never breaks
// onto two lines.
func TestAudit29SettingsValueDoesNotShrink(t *testing.T) {
	r := p31mRule(t, "static/css/places.css", "", ".place-row__value")
	p16MustDecl(t, ".place-row__value", r, map[string]string{"flex": "0 0 auto", "max-width": "50%"})
}

// TestAudit29LowDataTagsAndButtons (findings 12 and 16): the tags are 4px
// cornered and the two buttons sit 8px apart.
func TestAudit29LowDataTagsAndButtons(t *testing.T) {
	for _, head := range []string{".low-list .severity-label:not([hidden])", ".low-list .visibility-tag:not([hidden])"} {
		p16MustDecl(t, head, p31mRule(t, "static/css/low.css", "", head), map[string]string{"border-radius": "4px"})
	}
	p16MustDecl(t, ".low-page__actions", p31mRule(t, "static/css/low.css", "", ".low-page__actions:not([hidden])"),
		map[string]string{"display": "flex", "gap": "var(--space-sm)"})
	tmpl := p410ReadTemplate(t, "low.html.tmpl")
	if !strings.Contains(tmpl, `<div class="low-page__actions">`) {
		t.Errorf("low.html.tmpl must wrap the Report an emergency button in .low-page__actions (Amended 2026-10-21 by plan 44 (owner): Open full version moved to the top row as Back)")
	}
}

// TestAudit29TypeSizesAreRem (finding 14): the five text sizes are rem, so the
// browser's text size setting scales the app.
func TestAudit29TypeSizesAreRem(t *testing.T) {
	root := p31mRule(t, "static/css/main.css", "", ":root")
	for name, want := range map[string]string{
		"--font-size-caption": "0.75rem", "--font-size-label": "0.875rem", "--font-size-body": "1rem",
		"--font-size-heading": "1.125rem", "--font-size-display": "1.5rem",
	} {
		if got := root.Decls[name]; got != want {
			t.Errorf("%s is %q, want %q", name, got, want)
		}
	}
}

// TestAudit29MapControls (findings 17 and 20): the zoom buttons are 44px and
// the chip row has room for its last chip at the screen edge.
func TestAudit29MapControls(t *testing.T) {
	zoom := p31mRule(t, "static/css/main.css", "", ".leaflet-container .leaflet-bar.leaflet-control-zoom a")
	p16MustDecl(t, "zoom button", zoom, map[string]string{
		"width": "var(--touch-target-min)", "height": "var(--touch-target-min)", "line-height": "var(--touch-target-min)",
	})
	chips := p31mRule(t, "static/css/feed.css", "", ".map-chips")
	if got := chips.Decls["padding-right"]; got != "var(--space-md)" {
		t.Errorf(".map-chips padding-right is %q, want var(--space-md)", got)
	}
}

// TestAudit29NineTilesInThreeEvenColumns: the window grid has three columns, so
// nine tiles make three even rows and no tile sits alone on a row, on the
// phone and the laptop (one rule serves both).
func TestAudit29NineTilesInThreeEvenColumns(t *testing.T) {
	grid := p31mRule(t, "static/css/modal.css", "", ".post-window #category-grid")
	if got := grid.Decls["grid-template-columns"]; got != "repeat(3, minmax(0, 1fr))" {
		t.Errorf("window tile grid columns are %q, want repeat(3, minmax(0, 1fr))", got)
	}
	for _, r := range p16ParseCSS("static/css/modal.css", p16ReadStatic(t, "static/css/modal.css")) {
		if strings.Contains(r.Head, "#category-grid") && strings.Contains(r.Decls["grid-template-columns"], "repeat(4") {
			t.Errorf("modal.css still has a four column tile grid: %q (media %q)", r.Head, r.Media)
		}
	}
}

// TestAudit29ChosenSeverityBoxShowsACheck (owner answer 14): the chosen How
// serious box keeps its soft tint and its border, and gains a small check drawn
// from check.svg, decorative (a pseudo element with an empty alternative text),
// with a forced colours rule so it does not vanish.
func TestAudit29ChosenSeverityBoxShowsACheck(t *testing.T) {
	const head = `.post-window .severity-box[aria-checked="true"]::after`
	check := p31mRule(t, "static/css/modal.css", "", head)
	if !strings.Contains(check.Decls["-webkit-mask-image"], "/static/icons/check.svg") || !strings.Contains(check.Decls["mask-image"], "/static/icons/check.svg") {
		t.Errorf("the chosen box check must be drawn from check.svg, got %q", check.Decls["mask-image"])
	}
	if got := check.Decls["background-color"]; got != "currentColor" {
		t.Errorf("the check colour is %q, want currentColor", got)
	}
	if check.Decls["content"] != `"" / ""` {
		t.Errorf("the check must be decorative (content: \"\" / \"\"), got %q", check.Decls["content"])
	}
	for _, decl := range []string{"border", "border-color", "outline"} {
		if _, has := check.Decls[decl]; has {
			t.Errorf("the check rule sets %s; the chosen box must not change its border", decl)
		}
	}
	forced := p31mRule(t, "static/css/modal.css", "@media (forced-colors: active)", head)
	if forced.Decls["forced-color-adjust"] != "none" || forced.Decls["background-color"] != "ButtonText" {
		t.Errorf("the forced colours check rule is %v", forced.Decls)
	}
	// The template needs no extra markup: the three boxes keep their label only.
	if strings.Contains(windowBranch(t), "severity-box__check") {
		t.Errorf("the check is a pseudo element, the boxes carry no check markup")
	}
}
