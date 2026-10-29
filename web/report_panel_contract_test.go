package web

import (
	"regexp"
	"strings"
	"testing"
)

// Phase 8 plan 07 (D-18 default A, UI-SPEC Surface 2b and Surface 8): the
// laptop report panel and the Open report row of the laptop bubble. These
// tests pin the parts a node test cannot see: the stylesheets.

// TestPopupFooterOpenReportRow pins the Open report row (SC4, D-03, T-08-30):
// its rule is a full width 44px button through --touch-target-min, its count
// uses tabular numbers, and map.js writes the count words from the server's
// comment_count alone, with no number literal next to the words.
func TestPopupFooterOpenReportRow(t *testing.T) {
	trust := p16TrustRules(t)
	row := p16Rule(t, trust, "", ".leaflet-container .popup-card__open-report:not([hidden])")
	if got := row.Decls["min-height"]; got != "var(--touch-target-min)" {
		t.Errorf("the Open report row min-height is %q, want var(--touch-target-min)", got)
	}
	for prop, want := range map[string]string{
		"display":         "flex",
		"text-decoration": "none",
		"background":      "var(--color-bg)",
		"color":           "var(--color-text)",
	} {
		if got := row.Decls[prop]; got != want {
			t.Errorf("the Open report row %s is %q, want %q", prop, got, want)
		}
	}
	if !strings.Contains(row.Decls["border-top"], "var(--color-border)") {
		t.Errorf("the Open report row border-top is %q, want the border token hairline", row.Decls["border-top"])
	}
	count := p16Rule(t, trust, "", ".leaflet-container .popup-card__open-report .popup-card__open-count:not([hidden])")
	if got := count.Decls["font-variant-numeric"]; got != "tabular-nums" {
		t.Errorf("the count font-variant-numeric is %q, want tabular-nums (UI-SPEC Surface 8)", got)
	}
	for _, r := range trust {
		if !strings.Contains(r.Head, "popup-card__open") {
			continue
		}
		for name, value := range r.Decls {
			if strings.Contains(value, "!important") || strings.Contains(value, "rgb(") || strings.Contains(value, "#") && !strings.Contains(value, "url(") {
				t.Errorf("%q %s: %q uses !important or a raw colour", r.Head, name, value)
			}
		}
		if _, hasDisplay := r.Decls["display"]; hasDisplay {
			for _, part := range p16HeadParts(r.Head) {
				if !strings.Contains(part, ":not([hidden])") {
					t.Errorf("%q declares display but %q lacks :not([hidden])", r.Head, part)
				}
			}
		}
	}

	code := p13CodeLines(p13ReadStatic(t, "static/js/map.js"))
	if n := strings.Count(code, "report.comment_count"); n < 2 {
		t.Errorf("map.js reads report.comment_count %d times, want the row's count and name to read it", n)
	}
	if regexp.MustCompile(`comment_count\s*(\|\||\?\?|\?)`).MatchString(code) {
		t.Errorf("map.js gives comment_count a fallback; the row shows the server's number or nothing (T-08-30)")
	}
	if regexp.MustCompile(`[0-9]\s*comment`).MatchString(code) || regexp.MustCompile(`comments?['"]?\s*[+:]\s*[0-9]`).MatchString(code) {
		t.Errorf("map.js has a number literal next to the comment words (T-08-30)")
	}
	if strings.Contains(code, "popup-card__footer") || strings.Contains(code, "-footer") {
		t.Errorf("map.js names a class with footer in it")
	}
}

// TestReportPanelGeometry pins UI-SPEC Surface 2b: the panel is 520px, the
// wrapper is the only scroll box (overflow-y auto, min-height 0, overscroll
// contain), the Back anchor is 44px high, and all of it sits behind the
// data-report-panel attribute inside the one min-width 900px block.
func TestReportPanelGeometry(t *testing.T) {
	css := p16ReadStatic(t, "static/css/report-panel.css")
	rules := p16ParseCSS("static/css/report-panel.css", css)
	if len(rules) < 8 {
		t.Fatalf("only %d rules parsed from report-panel.css", len(rules))
	}
	byHead := map[string]p16CSSRule{}
	for _, r := range rules {
		if !strings.Contains(r.Media, "min-width: 900px") {
			t.Errorf("%q is outside the min-width 900px block (%q)", r.Head, r.Media)
		}
		if !strings.Contains(r.Head, "data-report-panel") {
			t.Errorf("%q does not start from html[data-report-panel]", r.Head)
		}
		byHead[r.Head] = r
	}
	popup, ok := byHead["html[data-report-panel] .leaflet-container .leaflet-detail-pane .leaflet-popup"]
	if !ok {
		t.Fatalf("no rule for the panel's popup")
	}
	for prop, want := range map[string]string{
		"position": "fixed",
		"width":    "520px",
		"left":     "0",
		"top":      "calc(var(--disclaimer-bar-height) + var(--app-topbar-height))",
		"height":   "calc(100dvh - var(--disclaimer-bar-height) - var(--app-topbar-height))",
	} {
		if got := popup.Decls[prop]; got != want {
			t.Errorf("the panel %s is %q, want %q", prop, got, want)
		}
	}
	if !strings.Contains(popup.Decls["border-right"], "var(--color-border)") {
		t.Errorf("the panel border-right is %q, want the border token hairline", popup.Decls["border-right"])
	}
	wrapper, ok := byHead["html[data-report-panel] .leaflet-container .leaflet-detail-pane .leaflet-popup-content-wrapper"]
	if !ok {
		t.Fatalf("no rule for the panel's scroll box")
	}
	for prop, want := range map[string]string{
		"overflow-y":          "auto",
		"min-height":          "0",
		"overscroll-behavior": "contain",
		"padding":             "var(--space-md) var(--space-lg) 0",
	} {
		if got := wrapper.Decls[prop]; got != want {
			t.Errorf("the panel scroll box %s is %q, want %q", prop, got, want)
		}
	}
	content, ok := byHead["html[data-report-panel] .leaflet-container .leaflet-detail-pane .leaflet-popup-content"]
	if !ok {
		t.Fatalf("no rule for the panel's content")
	}
	if content.Decls["min-width"] != "100%" || content.Decls["max-width"] != "100%" || content.Decls["max-height"] != "none" {
		t.Errorf("the panel content must state min-width 100%%, max-width 100%% and max-height none (they beat Leaflet's inline width): %v", content.Decls)
	}
	back := byHead["html[data-report-panel] .leaflet-container .leaflet-detail-pane .leaflet-popup a.leaflet-popup-close-button:not([hidden])"]
	if back.Decls["height"] != "var(--touch-target-min)" || back.Decls["position"] != "absolute" {
		t.Errorf("the Back anchor must be an absolute 44px high row at the top left: %v", back.Decls)
	}
	if !strings.Contains(css, `content: "Back"`) || !strings.Contains(css, "arrow-left.svg") {
		t.Errorf("the Back anchor must draw the arrow-left glyph and the word Back")
	}
	shell := byHead["html[data-report-panel] .app-shell"]
	if shell.Decls["grid-template-columns"] != "520px minmax(0, 1fr)" {
		t.Errorf("the shell columns while the panel is open are %q, want 520px minmax(0, 1fr)", shell.Decls["grid-template-columns"])
	}
	list := byHead["html[data-report-panel] .app-shell > .pane--list"]
	if list.Decls["display"] != "none" {
		t.Errorf("the Nearby list is not hidden while the panel is open: %v", list.Decls)
	}
}

// TestReportPanelHasNoPhoneRules pins that report-panel.css is the laptop's
// alone: one media block, min-width 900px, no narrow screen query, no
// important flag (Leaflet's inline styles are beaten by stated sizes and by
// map.js, never by the flag).
func TestReportPanelHasNoPhoneRules(t *testing.T) {
	css := p16ReadStatic(t, "static/css/report-panel.css")
	if n := strings.Count(css, "@media"); n != 1 {
		t.Errorf("report-panel.css has %d @media blocks, want exactly 1 (min-width: 900px)", n)
	}
	if !strings.Contains(css, "@media (min-width: 900px)") {
		t.Errorf("report-panel.css lacks its @media (min-width: 900px) block")
	}
	for _, banned := range []string{"max-width: 899", "max-width:899", "max-width: 7", "(max-", "!important", "rgb(", "rgba(", "hsl(", "999px"} {
		if strings.Contains(css, banned) {
			t.Errorf("report-panel.css contains %q", banned)
		}
	}
	for _, r := range p16ParseCSS("static/css/report-panel.css", css) {
		if _, hasDisplay := r.Decls["display"]; hasDisplay && r.Decls["display"] != "none" {
			for _, part := range p16HeadParts(r.Head) {
				if !strings.Contains(part, ":not([hidden])") {
					t.Errorf("%q declares display but %q lacks :not([hidden])", r.Head, part)
				}
			}
		}
	}
	// The panel never reaches the phone: the phone's sheet heads stay in
	// phone.css, and this file names no phone variable.
	for _, banned := range []string{"--sheet-top", "--app-bottombar-height", "data-popup-open]"} {
		if strings.Contains(css, banned) {
			t.Errorf("report-panel.css names the phone's %q", banned)
		}
	}
}
