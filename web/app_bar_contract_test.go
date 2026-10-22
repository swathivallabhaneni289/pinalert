package web

import (
	"bytes"
	"html/template"
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
// lines at 320, 360 or 390px at the normal text size. The label rule declares
// the 12px caption size, and the narrowest phones let the four items share
// the bar by their own width.
//
// Amended 2026-10-22 by plan 04-30 (owner): at 200 percent text "Official
// alerts" ran into "My activity", so the label rule now lets a label wrap
// between words inside its own column, centred (white-space normal, no break
// inside a word), and on the narrowest phones an item is never narrower than
// its longest word. The normal size keeps one line per label; the browser
// check in 04-30-SUMMARY.md measured that at 320, 360 and 390px.
func TestAppBarLabelsNoWrap(t *testing.T) {
	rules := phoneRules(t)
	item := p16Rule(t, rules, phoneBlockMedia, "body > .app-bar > .app-bar__item")
	p16MustDecl(t, "bottom bar item", item, map[string]string{
		"white-space":   "normal",
		"overflow-wrap": "normal",
		"word-break":    "normal",
		"text-align":    "center",
		"font-size":     "var(--font-size-caption)",
		"line-height":   "var(--line-height-caption)",
		"flex":          "1 1 0",
		"min-width":     "0",
	})
	narrow := p16Rule(t, rules, "(max-width: 380px)", "body > .app-bar > .app-bar__item")
	p16MustDecl(t, "narrow bottom bar item", narrow, map[string]string{
		"flex":      "1 1 auto",
		"min-width": "min-content",
		"padding":   "0 var(--space-xs)",
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

// searchBarNavItem and searchBarNav are the shapes the app bar partial reads
// from the page data (internal/api/handlers/nav.go NavView), rebuilt here
// because this package cannot import the handlers. ShowGear is the method the
// partial calls.
type searchBarNavItem struct {
	Key, Label, Href, Icon string
	Current                bool
}

type searchBarNav struct {
	Items    []searchBarNavItem
	Current  string
	Title    string
	BackHref string
}

func (v searchBarNav) ShowGear() bool { return v.Current != "settings" }

// renderTopBar draws the app_topbar block for the place with the given key.
func renderTopBar(t *testing.T, current string) string {
	t.Helper()
	tmpl, err := template.ParseFS(TemplatesFS, "templates/*.tmpl")
	if err != nil {
		t.Fatalf("parsing the templates: %v", err)
	}
	items := []searchBarNavItem{
		{Key: "nearby", Label: "Nearby", Href: "/", Icon: "map-pin"},
		{Key: "alerts", Label: "Official alerts", Href: "/alerts", Icon: "megaphone"},
		{Key: "activity", Label: "My activity", Href: "/profile", Icon: "user"},
	}
	nav := searchBarNav{Items: items, Current: current, Title: "Title"}
	for i := range nav.Items {
		nav.Items[i].Current = nav.Items[i].Key == current
	}
	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "app_topbar", map[string]any{"Nav": nav}); err != nil {
		t.Fatalf("rendering the top bar for %q: %v", current, err)
	}
	return out.String()
}

// TestSearchOnlyOnNearby pins D-11 and UI-SPEC 1c: Search a place is drawn in
// the top bar on Nearby and on no other place, between Post a report and the
// bell, and the partial is included in one spot under a Nearby condition. The
// bar is rendered for each place so the claim is about what a page draws, not
// only about the source text.
func TestSearchOnlyOnNearby(t *testing.T) {
	bar := p13ReadTemplate(t, appBarTemplate)
	if n := strings.Count(bar, `{{template "place_search.html.tmpl" .}}`); n != 1 {
		t.Errorf("%s includes the search partial %d times, want 1", appBarTemplate, n)
	}
	includeAt := strings.Index(bar, `{{template "place_search.html.tmpl" .}}`)
	ifAt := strings.LastIndex(bar[:includeAt], "{{if ")
	if ifAt == -1 || !strings.HasPrefix(bar[ifAt:], `{{if eq .Nav.Current "nearby"}}`) {
		t.Errorf("the search partial must sit directly under {{if eq .Nav.Current \"nearby\"}}")
	}
	// The key is the constant of the Go list, not a second spelling.
	nav := goSourceText(t, "../internal/api/handlers/nav.go")
	if !regexp.MustCompile(`NavNearby\s+=\s+"nearby"`).MatchString(nav) {
		t.Errorf("nav.go no longer names the Nearby key \"nearby\", which the search condition reads")
	}
	// No page includes the partial on its own, and only the bar names it.
	for _, page := range []string{"index.html.tmpl", "alerts.html.tmpl", "settings.html.tmpl", "profile.html.tmpl"} {
		if strings.Contains(p13ReadTemplate(t, page), "place_search.html.tmpl") {
			t.Errorf("%s includes the search partial itself; only the top bar does", page)
		}
	}

	post := `data-open-report><span class="app-icon app-icon--plus"`
	for _, tc := range []struct {
		current string
		want    bool
	}{{"nearby", true}, {"alerts", false}, {"activity", false}, {"settings", false}} {
		html := renderTopBar(t, tc.current)
		has := strings.Contains(html, `id="map-search"`)
		if has != tc.want {
			t.Errorf("the %q top bar draws the search field: %v, want %v", tc.current, has, tc.want)
		}
		if !tc.want {
			for _, id := range []string{"map-search-input", "map-search-submit", "map-search-status", "map-search-results"} {
				if strings.Contains(html, `id="`+id+`"`) {
					t.Errorf("the %q top bar draws #%s", tc.current, id)
				}
			}
			continue
		}
		// The laptop order: Post a report, the search, the bell, the light and
		// dark button, the person button.
		p13Ordered(t, html, "the Nearby top bar", post, `id="map-search"`, `id="bell-trigger"`, `id="theme-toggle"`, `id="account-menu-trigger"`)
	}
}

// TestSearchAlwaysVisible pins D-11: the field has no separate open or close
// button and no collapsed state. The banned names are held here, so the page,
// the script and the styles can be checked for their absence without naming
// them anywhere else.
func TestSearchAlwaysVisible(t *testing.T) {
	banned := []string{"map-search-open", "map-search-close", "map-search__open", "map-search__close", "data-search-open", "Close search"}
	files := []string{
		"templates/place_search.html.tmpl", "templates/app_bar.html.tmpl", "templates/index.html.tmpl",
		"static/js/map-tools.js", "static/css/feed.css", "static/css/phone.css", "static/css/app-bar.css", "static/css/main.css",
	}
	for _, file := range files {
		var text string
		if strings.HasPrefix(file, "templates/") {
			text = p13ReadTemplate(t, strings.TrimPrefix(file, "templates/"))
		} else {
			text = p13ReadStatic(t, file)
		}
		for _, name := range banned {
			if strings.Contains(text, name) {
				t.Errorf("%s still holds %q; the search field is always drawn, with no open or close control", file, name)
			}
		}
	}
	// The partial's only button is the magnifier submit.
	partial := p13ReadTemplate(t, "place_search.html.tmpl")
	if n := strings.Count(partial, "<button"); n != 1 {
		t.Errorf("place_search.html.tmpl holds %d buttons, want the one submit", n)
	}
	// The field gives width back before anything else leaves the bar (D-03):
	// it shrinks from its 260px, and the 900 to 1099px rule trims the gaps.
	appBar := p16ParseCSS("static/css/app-bar.css", p16ReadStatic(t, "static/css/app-bar.css"))
	form := p16Rule(t, appBar, "", ".app-topbar > .map-search")
	p16MustDecl(t, "top bar search form", form, map[string]string{"flex": "0 1 260px", "min-width": "0"})
	p16Rule(t, appBar, "@media (min-width: 900px) and (max-width: 1099.98px)", ".app-topbar .app-topbar__brand")
	p16Rule(t, appBar, "@media (min-width: 900px) and (max-width: 1099.98px)", ".app-topbar .app-topbar__link")
}
