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
	} {
		r := p31mRule(t, c.file, "", c.head)
		if got := r.Decls["outline"]; got != "none" {
			t.Errorf("%s %s outline is %q, want none", c.file, c.head, got)
		}
	}
}
