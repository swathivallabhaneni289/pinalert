package web

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// This file holds the static contract tests of the two app bars of Phase 3.4
// (plan 03.4-03, UI-SPEC Surface 1): the phone bottom bar and the laptop top
// bar are drawn from one Go list of places by app_bar.html.tmpl. Nothing here
// needs a database or a server; the rendered pages are compared by the handler
// tests TestAppBarRendersSameDestinations and TestAppBarCurrentItem.

const appBarTemplate = "app_bar.html.tmpl"

// appBarPlaceLabels are the words of the main places. They live in
// mainNavItems in internal/api/handlers/nav.go and nowhere else.
var appBarPlaceLabels = []string{"Nearby", "Official alerts", "My activity"}

// TestAppBarFromOneList pins D-01 and D-12: the partial draws both bars from
// .Nav.Items, one range for the bottom bar and one for the top bar, and holds
// no place name of its own, so a place added or renamed in the Go list appears
// on both sizes.
func TestAppBarFromOneList(t *testing.T) {
	bar := p13ReadTemplate(t, appBarTemplate)
	if n := strings.Count(bar, "range .Nav.Items"); n != 2 {
		t.Errorf("%s ranges over .Nav.Items %d times, want exactly 2 (the top bar and the bottom bar)", appBarTemplate, n)
	}
	// Outside template comments, no place label may be written.
	comment := regexp.MustCompile(`(?s)\{\{/\*.*?\*/\}\}`)
	code := comment.ReplaceAllString(bar, "")
	for _, label := range appBarPlaceLabels {
		if strings.Contains(code, label) {
			t.Errorf("%s writes the place name %q; it must come from .Nav.Items", appBarTemplate, label)
		}
	}
	// Both blocks exist, and each page that carries the bars includes both once.
	for _, block := range []string{`{{define "app_topbar"}}`, `{{define "app_bottombar"}}`} {
		if n := strings.Count(bar, block); n != 1 {
			t.Errorf("%s holds %d %s blocks, want 1", appBarTemplate, n, block)
		}
	}
	for _, page := range []string{"index.html.tmpl", "profile.html.tmpl"} {
		text := p13ReadTemplate(t, page)
		for _, call := range []string{`{{template "app_topbar" .}}`, `{{template "app_bottombar" .}}`} {
			if n := strings.Count(text, call); n != 1 {
				t.Errorf("%s includes %s %d times, want 1", page, call, n)
			}
		}
		if !strings.Contains(text, `<body class="has-app-bar">`) {
			t.Errorf("%s: the body needs class has-app-bar, which the page frame rules read", page)
		}
	}
	// The Go list is the only place the words are written.
	nav := goSourceText(t, "../internal/api/handlers/nav.go")
	for _, label := range appBarPlaceLabels {
		if strings.Count(nav, `"`+label+`"`) != 1 {
			t.Errorf("nav.go must write %q exactly once, in mainNavItems", label)
		}
	}
}

// TestAppBarReportNeverCurrent pins D-10: Report (phone) and Post a report
// (laptop) are links to /?post=1 that open the report window, so they work
// without script, they carry data-open-report so one script handles both, and
// neither is ever the current place.
func TestAppBarReportNeverCurrent(t *testing.T) {
	bar := p13ReadTemplate(t, appBarTemplate)
	links := regexp.MustCompile(`<a [^>]*data-open-report[^>]*>`).FindAllString(bar, -1)
	if len(links) != 2 {
		t.Fatalf("%s has %d data-open-report links, want the Report link and the Post a report link", appBarTemplate, len(links))
	}
	for _, link := range links {
		if !strings.Contains(link, `href="/?post=1"`) {
			t.Errorf("%s must link to /?post=1, so it works without script", link)
		}
		if strings.Contains(link, "aria-current") {
			t.Errorf("%s is current; Report and Post a report open a window and are never a place", link)
		}
	}
	// The current marker is written only on the ranged place links.
	if n := strings.Count(bar, "aria-current"); n != 2 {
		t.Errorf("%s writes aria-current %d times, want 2 (one per range)", appBarTemplate, n)
	}

	// Report is drawn like the others: no filled block, and no bar above it.
	rules := phoneRules(t)
	report := p16Rule(t, rules, phoneBlockMedia, "body > .app-bar > .app-bar__item--report")
	p16MustDecl(t, "phone Report item", report, map[string]string{"color": "var(--color-text)"})
	for prop := range report.Decls {
		if strings.HasPrefix(prop, "background") {
			t.Errorf("the phone Report item declares %s; it is never a filled block", prop)
		}
	}
	for _, r := range rules {
		for _, part := range p16HeadParts(r.Head) {
			if strings.Contains(part, "--report") && strings.Contains(part, "::before") {
				t.Errorf("phone.css draws a bar above the Report item with %q; Report is never current", part)
			}
		}
	}
}

// TestAppBarLabelsNoWrap pins D-03: the bottom bar labels never break onto two
// lines at 320, 360 or 390px. The label rule declares white-space nowrap and
// the 12px caption size, and the narrowest phones let the four items share
// the bar by their own width.
func TestAppBarLabelsNoWrap(t *testing.T) {
	rules := phoneRules(t)
	item := p16Rule(t, rules, phoneBlockMedia, "body > .app-bar > .app-bar__item")
	p16MustDecl(t, "bottom bar item", item, map[string]string{
		"white-space": "nowrap",
		"font-size":   "var(--font-size-caption)",
		"line-height": "var(--line-height-caption)",
		"flex":        "1 1 0",
		"min-width":   "0",
	})
	narrow := p16Rule(t, rules, "(max-width: 380px)", "body > .app-bar > .app-bar__item")
	p16MustDecl(t, "narrow bottom bar item", narrow, map[string]string{
		"flex":    "1 1 auto",
		"padding": "0 var(--space-xs)",
	})
	// Four equal items: three places and Report.
	bar := p13ReadTemplate(t, appBarTemplate)
	_, bottom, _ := strings.Cut(bar, `{{define "app_bottombar"}}`)
	if n := strings.Count(bottom, `class="app-bar__item`); n != 2 {
		t.Errorf("the bottom bar block writes %d item anchors, want the ranged one and Report (four items rendered)", n)
	}
}

// goSourceText reads a Go source file of the project by a path relative to
// this package, for the one place where a contract names the Go list.
func goSourceText(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}

// appBarTokens reads the pixel tokens of main.css :root, the spacing scale and
// the touch target, so a calc of tokens in a rule can be resolved.
func appBarTokens(t *testing.T) map[string]float64 {
	t.Helper()
	tokens := map[string]float64{}
	root := p16Rule(t, p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css")), "", ":root")
	for name, value := range root.Decls {
		if px, ok := strings.CutSuffix(value, "px"); ok {
			if f, err := strconv.ParseFloat(px, 64); err == nil {
				tokens[name] = f
			}
		}
	}
	return tokens
}

// appBarPx resolves a length written as px, var(--token) or a calc of those
// with plus signs. It fails the test for any other form.
func appBarPx(t *testing.T, value string, tokens map[string]float64) float64 {
	t.Helper()
	v := strings.TrimSpace(value)
	if inner, ok := strings.CutPrefix(v, "calc("); ok {
		inner = strings.TrimSuffix(inner, ")")
		total := 0.0
		for _, term := range strings.Split(inner, " + ") {
			total += appBarPx(t, term, tokens)
		}
		return total
	}
	if name, ok := strings.CutPrefix(v, "var("); ok {
		name = strings.TrimSuffix(name, ")")
		f, found := tokens[name]
		if !found {
			t.Fatalf("token %s is not a px token of main.css :root", name)
		}
		return f
	}
	if px, ok := strings.CutSuffix(v, "px"); ok {
		f, err := strconv.ParseFloat(px, 64)
		if err != nil {
			t.Fatalf("length %q: %v", value, err)
		}
		return f
	}
	t.Fatalf("length %q is not px, a token or a calc of those", value)
	return 0
}

// TestAppBarControlsMeetTouchTarget pins D-03: every new control of the bars is
// at least 44px high (and the icon buttons 44px wide): the bottom bar items, the
// Settings gear and back arrow, the laptop place links and Post a report.
func TestAppBarControlsMeetTouchTarget(t *testing.T) {
	tokens := appBarTokens(t)
	floor := tokens["--touch-target-min"]
	if floor != 44 {
		t.Fatalf("--touch-target-min resolves to %vpx, want 44", floor)
	}
	phone := phoneRules(t)
	laptop := p16ParseCSS("static/css/app-bar.css", p16ReadStatic(t, "static/css/app-bar.css"))

	check := func(label string, r p16CSSRule, prop string) {
		t.Helper()
		value, ok := r.Decls[prop]
		if !ok {
			t.Errorf("%s declares no %s", label, prop)
			return
		}
		if got := appBarPx(t, value, tokens); got < floor {
			t.Errorf("%s %s resolves to %vpx, below the %vpx touch target", label, prop, got, floor)
		}
	}

	check("bottom bar item", p16Rule(t, phone, phoneBlockMedia, "body > .app-bar > .app-bar__item"), "min-height")
	gear := p31mRule(t, "static/css/phone.css", phoneBlockMedia, ".app-topbar > .app-topbar__gear")
	check("Settings gear", gear, "width")
	check("Settings gear", gear, "height")
	back := p31mRule(t, "static/css/phone.css", phoneBlockMedia, ".app-topbar > .app-topbar__back")
	check("back arrow", back, "width")
	check("back arrow", back, "height")
	check("laptop place link", p16Rule(t, laptop, "", ".app-topbar__link"), "min-height")
	check("Post a report", p16Rule(t, laptop, "", ".app-topbar__post"), "min-height")
	check("wordmark link", p16Rule(t, laptop, "", ".app-topbar__brand"), "min-height")
}
