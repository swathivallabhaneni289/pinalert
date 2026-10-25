package web

import (
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

var _ = strings.Contains
