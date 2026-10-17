package web

import (
	"regexp"
	"strings"
	"testing"
)

// Phase 3.4 plan 26 (gap closure, code review part 3, templates and
// stylesheets): the pins for WR-01 to WR-06, IN-02 and IN-04 of
// .planning/phases/03.4-app-layout/03.4-REVIEW.md. WR-07 (the chosen severity
// box differs by tint alone) is an owner question and is not pinned here.
//
// These read the declared CSS and the shipped templates. How a screen looks in
// a forced colours theme is proved by the browser probe the plan summary
// records (Chromium only: Playwright emulates forced colours there).

// p26Rules returns the rules of one stylesheet.
func p26Rules(t *testing.T, file string) []p16CSSRule {
	t.Helper()
	return p16ParseCSS("static/css/"+file, p16ReadStatic(t, "static/css/"+file))
}

// p26Find returns every rule that has the selector part among its head parts
// and whose media contains the wanted text ("" means any media).
func p26Find(rules []p16CSSRule, media, part string) []p16CSSRule {
	var out []p16CSSRule
	for _, r := range rules {
		if media != "" && !strings.Contains(r.Media, media) {
			continue
		}
		for _, p := range p16HeadParts(r.Head) {
			if p == part {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// p26Has reports whether any rule found for the part declares prop with a
// value that contains want.
func p26Has(rules []p16CSSRule, media, part, prop, want string) bool {
	for _, r := range p26Find(rules, media, part) {
		if strings.Contains(r.Decls[prop], want) {
			return true
		}
	}
	return false
}

// TestForcedColoursShowsTheMaskGlyphs pins part 3 WR-01. A forced colours
// theme replaces a background colour with Canvas, so a glyph drawn as a
// coloured box behind a mask, and the sheet grabber bar, paint Canvas on Canvas
// and vanish. Each opts out of the adjustment and takes a system colour, the
// way .notice-strip__icon already does (chrome.css) and the trust chevron does
// (trust.css, A78).
func TestForcedColoursShowsTheMaskGlyphs(t *testing.T) {
	glyphs := []struct{ file, part string }{
		{"app-bar.css", ".app-icon"},
		{"feed.css", ".map-search__icon"},
		{"feed.css", `.map-chip[aria-pressed="true"]::before`},
		{"phone.css", ".sheet__handle > .sheet__grabber"},
	}
	for _, g := range glyphs {
		rules := p26Rules(t, g.file)
		if !p26Has(rules, "forced-colors", g.part, "forced-color-adjust", "none") {
			t.Errorf("%s: no forced colours rule gives %s forced-color-adjust: none (part 3 WR-01)", g.file, g.part)
			continue
		}
		ok := false
		for _, r := range p26Find(rules, "forced-colors", g.part) {
			switch r.Decls["background-color"] {
			case "CanvasText", "ButtonText", "LinkText":
				ok = true
			}
		}
		if !ok {
			t.Errorf("%s: the forced colours rule for %s must set background-color to a system colour (part 3 WR-01)", g.file, g.part)
		}
	}
	// The phone grabber rule lives in phone.css's one narrow block, so its
	// query carries the 899.98px bound as well as the forced colours test.
	for _, r := range p26Find(p26Rules(t, "phone.css"), "forced-colors", ".sheet__handle > .sheet__grabber") {
		if !strings.Contains(r.Media, "max-width: 899.98px") {
			t.Errorf("phone.css: the grabber forced colours rule is outside the 899.98px block: %q", r.Media)
		}
	}
}

// TestChosenRowsShowAHighlightBorderInForcedColours pins part 3 WR-02. UI-SPEC
// Color says chosen items add 1px solid Highlight in a forced colours theme;
// the tile, the severity box, the pick and the place option do, and the chosen
// list row and the chosen activity card did not.
func TestChosenRowsShowAHighlightBorderInForcedColours(t *testing.T) {
	feed := p26Rules(t, "feed.css")
	// Three heads at (1,3,0): the phone card's provisional and hidden looks in
	// phone.css are (1,2,0) and load later, so a two class head would lose.
	for _, head := range []string{
		"#report-list .report-row.report-row--selected",
		"#report-list .report-row.vis-provisional.report-row--selected",
		"#report-list .report-row.vis-hidden.report-row--selected",
	} {
		found := false
		for _, r := range p26Find(feed, "forced-colors", head) {
			if strings.Contains(r.Decls["border"], "1px solid Highlight") {
				found = true
			}
		}
		if !found {
			t.Errorf("feed.css: no forced colours rule gives %q a 1px solid Highlight border (part 3 WR-02)", head)
		}
	}
	// The laptop row keeps its 4px severity bar in forced colours too.
	if !p26Has(feed, "forced-colors", "#report-list .report-row.report-row--selected", "border-left-width", "4px") {
		t.Errorf("feed.css: the laptop chosen row loses its 4px severity bar in forced colours (part 3 WR-02)")
	}
	act := p26Rules(t, "activity.css")
	card := "main.profile-page .profile-report-list > .activity-card.activity-card--chosen"
	if !p26Has(act, "forced-colors", card, "border", "1px solid Highlight") {
		t.Errorf("activity.css: no forced colours rule gives the chosen activity card a 1px solid Highlight border (part 3 WR-02)")
	}
}

// TestFullWidthRowsKeepTheirFocusRingInside pins part 3 WR-03. The global ring
// is drawn 2px outside the row, so a full width row inside a box with
// overflow hidden or auto lost its left and right edges (and the top or bottom
// edge at the ends of the box). The rows take the inset ring the house already
// uses on the chips and the Show menu; the welcome pane, whose buttons are
// filled with the ring colour, takes a 4px gutter instead.
func TestFullWidthRowsKeepTheirFocusRingInside(t *testing.T) {
	inset := func(file, media, part string) {
		t.Helper()
		if !p26Has(p26Rules(t, file), media, part, "outline-offset", "-2px") {
			t.Errorf("%s: %s needs outline-offset: -2px so the ring is not clipped (part 3 WR-03)", file, part)
		}
	}
	inset("places.css", "", ".place-row:focus-visible")
	inset("window.css", "", ".app-window__option:focus-visible")
	inset("phone.css", "max-width: 899.98px", "main.profile-page > .activity-hub .activity-hub__row:focus-visible")

	// A primary button is filled with the ring colour, so an inset ring would
	// be invisible on it. The pane gets a gutter as wide as the ring and the
	// same amount of negative margin, so nothing moves.
	welcome := p26Rules(t, "welcome.css")
	if !p26Has(welcome, "", ".welcome__pane", "padding", "var(--space-xs)") || !p26Has(welcome, "", ".welcome__pane", "margin", "calc(-1 * var(--space-xs))") {
		t.Errorf("welcome.css: .welcome__pane needs a var(--space-xs) gutter with matching negative margin so its children's focus rings fit (part 3 WR-03)")
	}
	if !p26Has(welcome, "min-width: 900px", ".welcome__pane", "margin", "0") {
		t.Errorf("welcome.css: the laptop pane must reset the gutter margin to 0")
	}
}

// TestNearbyKeepsAnH1ForScreenReaders pins part 3 WR-04. On Nearby the top bar
// title is the page's only h1, and it was display: none at both sizes. It is
// now hidden by clipping, on Nearby only (the pages that show a heading keep
// one visible h1).
func TestNearbyKeepsAnH1ForScreenReaders(t *testing.T) {
	const head = ".app-topbar:has(> .map-search) > .app-topbar__title"
	hidden := false
	for _, file := range layoutStylesheets(t) {
		for _, r := range p26Find(p16ParseCSS(file, p16ReadStatic(t, file)), "", head) {
			if r.Decls["display"] == "none" {
				t.Errorf("%s: %s is display: none, which takes the Nearby h1 out of the page (part 3 WR-04)", file, head)
			}
			if r.Media == "" && r.Decls["position"] == "absolute" && strings.Contains(r.Decls["clip"], "rect(") && r.Decls["display"] == "block" {
				hidden = true
			}
		}
	}
	if !hidden {
		t.Errorf("no base rule hides %s by clipping (position: absolute, clip: rect(...), display: block) (part 3 WR-04)", head)
	}
	bar := p421ReadTemplate(t, "app_bar.html.tmpl")
	if strings.Count(bar, `<h1 class="app-topbar__title">`) != 1 {
		t.Errorf("app_bar.html.tmpl must keep exactly one h1.app-topbar__title")
	}
}

var p26HeaderOpen = regexp.MustCompile(`<header[\s>]`)

// TestNoHeaderInsideHeader pins part 3 WR-05. app_topbar opens a header, and it
// used to include account_header.html.tmpl, whose root was a header too: two
// nested banner landmarks on a laptop. The top bar now wraps the shared menu
// markup in a div. The partial itself keeps its header root for the pages that
// call it on its own (Low data view, Moderator screen), where it is the page's
// one banner.
func TestNoHeaderInsideHeader(t *testing.T) {
	bar := p421ReadTemplate(t, "app_bar.html.tmpl")
	start := strings.Index(bar, `{{define "app_topbar"}}`)
	end := strings.Index(bar, `{{define "app_bottombar"}}`)
	if start == -1 || end == -1 || end < start {
		t.Fatalf("app_bar.html.tmpl: the app_topbar block was not found")
	}
	topbar := bar[start:end]
	if n := len(p26HeaderOpen.FindAllString(topbar, -1)); n != 1 {
		t.Errorf("the app_topbar block has %d header elements, want exactly 1 (part 3 WR-05)", n)
	}
	if strings.Contains(topbar, `"account_header.html.tmpl"`) {
		t.Errorf("app_topbar includes account_header.html.tmpl, whose root is a header: a header nested in a header (part 3 WR-05)")
	}
	if !strings.Contains(topbar, `<div class="app-header">{{template "account_header_menu" .}}</div>`) {
		t.Errorf("app_topbar must wrap the shared account menu markup in a div.app-header (part 3 WR-05)")
	}

	header := p421ReadTemplate(t, "account_header.html.tmpl")
	if !strings.Contains(header, `{{define "account_header_menu"}}`) {
		t.Errorf("account_header.html.tmpl must define account_header_menu, the menu markup both callers share")
	}
	if !strings.Contains(header, `<header class="app-header">{{template "account_header_menu" .}}</header>`) {
		t.Errorf("account_header.html.tmpl must keep a header root around the shared markup for the pages that call it on their own")
	}
	// Each page that calls the partial on its own has no other header, so it
	// keeps exactly one banner.
	for _, page := range []string{"low.html.tmpl", "moderate.html.tmpl"} {
		raw := p421ReadTemplate(t, page)
		if !strings.Contains(raw, `{{template "account_header.html.tmpl" .}}`) {
			t.Errorf("%s no longer calls the account header; it would lose its banner (part 3 WR-05)", page)
		}
		if p26HeaderOpen.MatchString(raw) {
			t.Errorf("%s holds a header of its own next to the account header: two banners (part 3 WR-05)", page)
		}
	}
}

// TestLegalMutedTextClearsContrastOnTheGreyBox pins part 3 WR-06. The legal
// subtitle and the updated date used the muted text colour, and the shared
// legal body now sits on the grey surface in the reader page, the reader
// window and the first-time flow, where muted text is 4.34 to 1 in the light
// theme. Inside those three boxes they take the normal text colour.
func TestLegalMutedTextClearsContrastOnTheGreyBox(t *testing.T) {
	legal := p26Rules(t, "legal.css")
	window := p26Rules(t, "window.css")
	welcome := p26Rules(t, "welcome.css")
	boxes := []struct {
		rules []p16CSSRule
		file  string
		box   string
	}{
		{legal, "legal.css", ".legal-box"},
		{window, "window.css", ".app-window__box"},
		{welcome, "welcome.css", ".welcome__scrollbox"},
	}
	main := p26Rules(t, "main.css")
	lightTokens := p16Rule(t, main, "", ":root").Decls
	darkTokens := p16Rule(t, main, "", `:root[data-theme="dark"]`).Decls
	for _, b := range boxes {
		for _, line := range []string{".legal-subtitle", ".legal-updated"} {
			part := b.box + " " + line
			fg := "var(--color-text-muted)"
			for _, r := range p26Find(b.rules, "", part) {
				if c := r.Decls["color"]; c != "" {
					fg = c
				}
			}
			name, ok := bareVarName(fg)
			if !ok {
				t.Errorf("%s: %s has colour %q, want a token", b.file, part, fg)
				continue
			}
			for theme, tokens := range map[string]map[string]string{"light": lightTokens, "dark": darkTokens} {
				ratio := wcagContrastRatio(t, p31Resolve(t, themeTokens(tokens), name), p31Resolve(t, themeTokens(tokens), "--color-surface"), b.file+" "+part+" "+theme)
				if ratio < 4.5 {
					t.Errorf("%s: %s is %.2f to 1 on the grey box in the %s theme, under 4.5 to 1 (part 3 WR-06)", b.file, part, ratio, theme)
				}
			}
		}
	}
}

// TestDynamicViewportHeightsHaveAFallback pins part 3 IN-04: every declaration
// that uses 100dvh sits right after the same declaration with 100vh, so an
// engine without dvh keeps a height limit.
//
// The walk covers the stylesheets written or reshaped by Phase 3.4. trust.css
// and its popup heights belong to earlier phases and are not part of this
// finding.
func TestDynamicViewportHeightsHaveAFallback(t *testing.T) {
	for _, name := range []string{"app-bar", "feed", "phone", "main", "activity", "places", "window", "welcome", "legal", "chrome", "modal"} {
		file := "static/css/" + name + ".css"
		lines := strings.Split(stripCSSComments(p16ReadStatic(t, file)), "\n")
		for i, line := range lines {
			if !strings.Contains(line, "100dvh") {
				continue
			}
			prop := strings.TrimSpace(line[:strings.Index(line, ":")+1])
			prev := ""
			for j := i - 1; j >= 0; j-- {
				if s := strings.TrimSpace(lines[j]); s != "" {
					prev = s
					break
				}
			}
			if !strings.HasPrefix(prev, prop) || !strings.Contains(prev, "100vh") {
				t.Errorf("%s (comment stripped line %d): %q has no 100vh declaration of the same property before it (part 3 IN-04)", file, i+1, strings.TrimSpace(line))
			}
		}
	}
}
