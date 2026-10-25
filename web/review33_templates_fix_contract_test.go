package web

import (
	"regexp"
	"strings"
	"testing"
)

// Phase 3.3 template and CSS review fixes (03.3-REVIEW-templates.md, T-02 and
// T-03, 2026-10-25). Each test fails on the code before its fix.

// TestDeviceGuideLeadSpacing pins T-02. The lead to steps distance is the
// contract's 16px in step 5 and in the Settings window. Flex margins do not
// collapse, so the lead carries no bottom margin of its own in step 5 (the
// .welcome__alerts-part gap is 16px), and in the window (.app-window__body gap
// 8px) it carries exactly the 8px that brings the distance to 16px.
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
	part := p31mRule(t, "static/css/welcome.css", "", ".welcome__alerts-part:not([hidden])")
	if part.Decls["gap"] != "var(--space-md)" {
		t.Errorf(".welcome__alerts-part gap is %q, want var(--space-md)", part.Decls["gap"])
	}
	body := p31mRule(t, "static/css/window.css", "", ".app-window__body")
	if body.Decls["gap"] != "var(--space-sm)" {
		t.Errorf(".app-window__body gap is %q, want var(--space-sm); the window lead margin assumes it", body.Decls["gap"])
	}
}

// TestWelcomeStatusIsAlwaysPresentLiveRegion pins T-04. The step 5 refusal line
// is a role="status" element that is in the page from the start, never hidden
// (a region unhidden in the same moment as its text may not be announced by
// VoiceOver). While empty, welcome.css clips it out of the flow so it takes no
// gap; welcome.js only writes its text.
func TestWelcomeStatusIsAlwaysPresentLiveRegion(t *testing.T) {
	page := p421ReadTemplate(t, "welcome.html.tmpl")
	at := strings.Index(page, `id="welcome-alerts-status"`)
	if at == -1 {
		t.Fatal("welcome.html.tmpl has no welcome-alerts-status")
	}
	start := strings.LastIndex(page[:at], "<")
	tag := page[start : at+strings.Index(page[at:], "</p>")+len("</p>")]
	if !strings.Contains(tag, `role="status"`) {
		t.Errorf("the status line is not role=status: %s", tag)
	}
	if strings.Contains(tag, "hidden") {
		t.Errorf("the status line carries hidden, so it is not an always-present live region: %s", tag)
	}
	if !strings.HasSuffix(tag, "></p>") {
		t.Errorf("the status line is not empty in the template (a space would defeat :empty): %s", tag)
	}
	empty := p31mRule(t, "static/css/welcome.css", "", ".welcome__status:empty")
	if empty.Decls["position"] != "absolute" || empty.Decls["clip"] != "rect(0, 0, 0, 0)" {
		t.Errorf("an empty status line is not clipped out of the flow: %v", empty.Decls)
	}
	js := p16ReadStatic(t, "static/js/welcome.js")
	if regexp.MustCompile(`(?:hideLine|showLine)\(alertsStatus`).MatchString(js) {
		t.Errorf("welcome.js hides or shows the status line; it must only write its text")
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
