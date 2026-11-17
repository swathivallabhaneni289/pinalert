package web

import (
	"testing"
)

// Phase 3.3 template and CSS review fixes (03.3-REVIEW-templates.md, T-02 and
// T-03, 2026-10-25). Each test fails on the code before its fix.
//
// Amended 2026-10-27 by quick 261027-t5w (owner): step 5 became the alerts
// pop-up, so the pins on its alerts part gap (T-02, step 5 half) and on its
// always-present status line (T-04) are gone with the markup they described.

// TestDeviceGuideLeadSpacing pins T-02. The lead to steps distance is the
// contract's 16px in the Settings window. Flex margins do not collapse, so the
// lead carries no bottom margin of its own and in the window (.app-window__body
// gap 8px) it carries exactly the 8px that brings the distance to 16px.
func TestDeviceGuideLeadSpacing(t *testing.T) {
	const guide = "static/css/device-guide.css"
	lead := p31mRule(t, guide, "", ".device-guide__lead")
	if got := lead.Decls["margin"]; got != "0" {
		t.Errorf(".device-guide__lead margin is %q, want 0 so the container gap alone spaces it", got)
	}
	win := p31mRule(t, guide, "", ".app-window__body > .device-guide__lead")
	if got := win.Decls["margin-bottom"]; got != "var(--space-sm)" {
		t.Errorf("the window lead margin-bottom is %q, want var(--space-sm) (8px gap + 8px = 16px)", got)
	}
	// Amended 2026-10-29 by Phase 7.1 plan 13b: the guide is also shown inside the
	// install pop-up at the end of sign up (the Log out box, a plain block body
	// with no gap). Nothing spaces its parts there, so the lead and the steps carry
	// their own margins: 16px under the lead (the contract's lead to steps distance)
	// and 24px under the steps (the Log out box's distance above its buttons). The
	// lead keeps no top or side margin, so the 24px under the title stays the
	// title's own.
	boxLead := p31mRule(t, guide, "", ".logout-confirm__body > .device-guide__lead")
	if got := boxLead.Decls["margin"]; got != "0 0 var(--space-md)" {
		t.Errorf("the install pop-up lead margin is %q, want 0 0 var(--space-md) (16px to the steps)", got)
	}
	boxSteps := p31mRule(t, guide, "", ".logout-confirm__body > .device-guide__steps")
	if got := boxSteps.Decls["margin"]; got != "0 0 var(--space-lg)" {
		t.Errorf("the install pop-up steps margin is %q, want 0 0 var(--space-lg) (24px to the button)", got)
	}
	body := p31mRule(t, "static/css/window.css", "", ".app-window__body")
	if body.Decls["gap"] != "var(--space-sm)" {
		t.Errorf(".app-window__body gap is %q, want var(--space-sm); the window lead margin assumes it", body.Decls["gap"])
	}
}

// TestDeviceGuideShareStepIcons (quick 261117-kqy): step 1's tile holds the
// Share and the Menu (three dots) glyphs and only one shows. .app-icon sets
// display: block, which outranks the browser's own [hidden] rule, so the guard
// must hide the other one; each glyph keeps its own mask rule.
func TestDeviceGuideShareStepIcons(t *testing.T) {
	const guide = "static/css/device-guide.css"
	if got := p31mRule(t, guide, "", ".device-guide__tile .app-icon[hidden]").Decls["display"]; got != "none" {
		t.Errorf("the hidden glyph of step 1 has display %q, want none, or both glyphs show", got)
	}
	for head, file := range map[string]string{".app-icon--share": "share.svg", ".app-icon--ellipsis": "ellipsis.svg"} {
		rule := p31mRule(t, guide, "", head)
		want := `url("/static/icons/` + file + `")`
		for _, prop := range []string{"mask-image", "-webkit-mask-image"} {
			if got := rule.Decls[prop]; got != want {
				t.Errorf("%s %s is %q, want %s", head, prop, got, want)
			}
		}
	}
}

// TestPlainSettingsRowsHaveNoPointerCursor pins T-03. The blocked and
// unsupported On this device rows are plain div rows (not a control), so the
// bare .place-row rule must not set the pointer cursor; only the rows that act
// (button and anchor rows here, the switch label rows in updates.css) get it.
func TestPlainSettingsRowsHaveNoPointerCursor(t *testing.T) {
	const places = "static/css/places.css"
	if got := p31mRule(t, places, "", ".place-row").Decls["cursor"]; got == "pointer" {
		t.Errorf("the bare .place-row rule sets cursor: pointer, so the blocked and unsupported div rows show a hand")
	}
	for _, head := range []string{"button.place-row", "a.place-row"} {
		if got := p31mRule(t, places, "", head).Decls["cursor"]; got != "pointer" {
			t.Errorf("%s cursor is %q, want pointer (the row acts)", head, got)
		}
	}
	if got := p31mRule(t, "static/css/updates.css", "", ".place-row--switch").Decls["cursor"]; got != "pointer" {
		t.Errorf(".place-row--switch cursor is %q, want pointer (the whole label is the target)", got)
	}
}
