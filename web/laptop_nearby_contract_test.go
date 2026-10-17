package web

import (
	"regexp"
	"strings"
	"testing"
)

// The laptop Nearby (Phase 3.4 plan 20, UI-SPEC 2a, D-06, D-17, D-03): two
// columns under the top bar and the strip, the list pane 420px on the left with
// a 1px right border and the map taking the rest on the right. The list head
// and the strip lines stay put and only the cards scroll. A pin choice brings
// its row into view inside the cards region with scrollTo, never with
// scrollIntoView and never by scrolling the page. These are source pins; a
// browser pass looks at the real pixels.

const laptopMedia = "@media (min-width: 900px)"

func TestLaptopNearbyTwoColumns(t *testing.T) {
	const main = "static/css/main.css"

	shell := p31mRule(t, main, laptopMedia, ".app-shell")
	p16MustDecl(t, "laptop shell", shell, map[string]string{
		"grid-template-columns": "420px minmax(0, 1fr)",
		"grid-template-rows":    "minmax(0, 1fr)",
		"gap":                   "0",
	})
	if strings.Contains(p16ReadStatic(t, main), "1fr 1fr") {
		t.Errorf("main.css still holds the old equal halves grid %q", "1fr 1fr")
	}

	// The panes are placed by grid placement, not by markup order: the DOM keeps
	// the map first.
	list := p31mRule(t, main, laptopMedia, ".app-shell > .pane--list")
	p16MustDecl(t, "laptop list pane", list, map[string]string{
		"grid-column":    "1",
		"grid-row":       "1",
		"display":        "flex",
		"flex-direction": "column",
		"border-right":   "1px solid var(--color-border)",
	})
	pane := p31mRule(t, main, laptopMedia, ".app-shell > .pane--map")
	p16MustDecl(t, "laptop map pane", pane, map[string]string{"grid-column": "2", "grid-row": "1"})

	index := findFullPageTemplates(t)["templates/index.html.tmpl"]
	mapAt := strings.Index(index, `<section class="pane pane--map">`)
	listAt := strings.Index(index, `<section class="pane pane--list" id="report-sheet"`)
	if mapAt == -1 || listAt == -1 || mapAt > listAt {
		t.Errorf("index.html.tmpl: the map pane must stay before the list pane in the markup (map at %d, list at %d); the columns come from CSS placement", mapAt, listAt)
	}

	// Only the cards scroll: the head, the lines and the notice rows keep their
	// own height, and the cards list is the one scroll region.
	keep := p31mRule(t, main, laptopMedia, ".app-shell > .pane--list > :not(#report-list)")
	p16MustDecl(t, "pane parts that stay put", keep, map[string]string{"flex": "none"})
	cards := p31mRule(t, "static/css/feed.css", laptopMedia, "#report-list")
	p16MustDecl(t, "cards region", cards, map[string]string{
		"overflow-y": "auto", "min-height": "0", "overscroll-behavior": "contain", "flex": "0 1 auto",
	})
	rows := p31mRule(t, "static/css/feed.css", laptopMedia, "#report-list > .report-row")
	p16MustDecl(t, "cards keep their height", rows, map[string]string{"flex": "none"})

	// The notice rows are strip lines in the pane, not a card floating over the
	// cards (UI-SPEC 2a). chrome.css keeps the built fixed rule for other uses.
	card := p31mRule(t, main, laptopMedia, ".app-shell > .pane--list > .notice-card")
	p16MustDecl(t, "laptop notice card", card, map[string]string{
		"position": "static", "max-width": "none", "max-height": "none", "box-shadow": "none", "overflow": "visible",
	})
	cardAt := strings.Index(index, `id="notice-card"`)
	if cardAt < listAt || cardAt > strings.Index(index, `id="report-list"`) {
		t.Errorf("the notice card must sit inside the list pane, above the cards")
	}

	// The message sits over the list pane, never over a report opened in the map.
	toast := p31mRule(t, main, laptopMedia, ".app-shell > #toast:not([hidden])")
	p16MustDecl(t, "laptop message", toast, map[string]string{
		"left": "calc(420px / 2)", "max-width": "calc(420px - var(--space-md) * 2)",
	})

	// The phone block is not part of this plan.
	phone := p16ReadStatic(t, "static/css/phone.css")
	if strings.Contains(phone, "420px") {
		t.Errorf("phone.css names the laptop pane width; the phone sheet is not changed by this plan")
	}
}

func TestLaptopNearbyNoPageScroll(t *testing.T) {
	const main = "static/css/main.css"
	shell := p31mRule(t, main, laptopMedia, ".app-shell")
	want := "calc(100dvh - var(--disclaimer-bar-height) - var(--app-topbar-height))"
	if p19NoSpace(shell.Decls["height"]) != p19NoSpace(want) {
		t.Errorf("laptop shell height is %q, want the viewport less the bars %q", shell.Decls["height"], want)
	}
	// The legacy unit is declared first, the dynamic unit second so it wins.
	raw := p16ReadStatic(t, main)
	start := strings.Index(raw, laptopMedia+" {\n  /* The laptop Nearby")
	if start == -1 {
		t.Fatalf("main.css has no laptop Nearby block")
	}
	block := raw[start:]
	vh := strings.Index(block, "height: calc(100vh - var(--disclaimer-bar-height) - var(--app-topbar-height));")
	dvh := strings.Index(block, "height: calc(100dvh - var(--disclaimer-bar-height) - var(--app-topbar-height));")
	if vh == -1 || dvh == -1 || vh > dvh {
		t.Errorf("the laptop shell height needs the 100vh fallback before the 100dvh value (vh at %d, dvh at %d)", vh, dvh)
	}
	body := p31mRule(t, main, laptopMedia, "body.has-app-bar:has(> .app-shell)")
	p16MustDecl(t, "no document scroll", body, map[string]string{"overflow": "hidden", "overscroll-behavior": "none"})
}

func TestLaptopRowScrollUsesScrollTo(t *testing.T) {
	raw := p13ReadStatic(t, "static/js/feed.js")
	p13MustNotContain(t, "feed.js", raw, "scrollIntoView")

	code := p19Code(t, "static/js/feed.js")
	reveal := jsFunctionBody(t, code, "revealRowInListPane(id)")
	// The scroller is chosen by the layout at the moment of the pick: the sheet
	// on the narrow query, the cards list otherwise.
	p13Ordered(t, reveal, "revealRowInListPane",
		"narrowQuery && narrowQuery.matches", "document.getElementById('report-sheet')", ": reportListEl",
		"sheet.scrollHeight <= sheet.clientHeight", "sheet.contains(head)", "sheet.scrollTo(")
	if !regexp.MustCompile(`sheet\.scrollTo\(\{ top: Math\.max\(0, top\), behavior: calm \? 'auto' : 'smooth' \}\)`).MatchString(reveal) {
		t.Errorf("revealRowInListPane must call scrollTo on the scrolling element with a clamped top and the reduced motion behaviour")
	}
	banned := regexp.MustCompile(`(window|document\.documentElement|document\.body)\.scroll|scrollingElement`)
	if m := banned.FindString(reveal); m != "" {
		t.Errorf("revealRowInListPane calls %q; it may set only the list's own scroll position", m)
	}
}

// TestLaptopMapSizeFollowsTheLayout pins D-03 for the laptop: the map is told
// its new size once the layout settles, by one debounced observer on the map
// element, never on every frame, and not on a phone width where the sheet's
// own settle path runs.
func TestLaptopMapSizeFollowsTheLayout(t *testing.T) {
	code := p19Code(t, "static/js/map.js")
	watch := jsFunctionBody(t, code, "watchMapBox(container)")
	p13Ordered(t, watch, "watchMapBox",
		"typeof ResizeObserver !== 'function'", "new ResizeObserver(", "window.clearTimeout(timer)",
		"window.setTimeout(", "!phoneDetail()", "map.invalidateSize({ animate: false, pan: false })", "LAPTOP_SETTLE_MS", ".observe(container)")
	p13MustNotContain(t, "watchMapBox", watch, "requestAnimationFrame")
	initBody := jsFunctionBody(t, code, "init()")
	p13MustContain(t, "init", initBody, "watchMapBox(container);")
}

// TestLaptopNearbyShellFitsWindow pins the arithmetic behind the laptop shell
// height (Phase 3.4 plan 21, plan 17 finding 1). The shell, the map and the
// panes are the window less the notice strip token and the top bar token, so
// every pixel of the top bar must be inside that token. The token is 61px: a
// 44px control row, 8px above and below and the 1px hairline. The place search
// field is a 44px control wrapped in its own 1px border, which makes the
// bordered box 46px; left alone it stretched the bar to 63px and the shell was
// 2px taller than the window. The field's border now hangs into the bar's
// padding with a negative block margin, so the form is 44px and the bar is
// exactly its token. Both bars are border-box and the strip's floor is its own
// border-box height, so a future border cannot add height without failing here.
func TestLaptopNearbyShellFitsWindow(t *testing.T) {
	const appBar = "static/css/app-bar.css"
	const main = "static/css/main.css"

	// The token is the control row, the padding and the hairline.
	tok := p31mRule(t, main, laptopMedia, ":root")
	if got, want := p19NoSpace(tok.Decls["--app-topbar-height"]), p19NoSpace("calc(var(--touch-target-min) + var(--space-md) + 1px)"); got != want {
		t.Errorf("laptop --app-topbar-height is %q, want 44 control + 16 padding + 1px hairline %q", tok.Decls["--app-topbar-height"], want)
	}

	// The bar states its token as a floor, in border-box, with the hairline
	// and the 8px padding inside it.
	bar := p31mRule(t, appBar, "", ".app-topbar")
	p16MustDecl(t, "top bar", bar, map[string]string{
		"min-height":    "var(--app-topbar-height)",
		"padding":       "var(--space-sm) var(--space-md)",
		"border-bottom": "1px solid var(--color-border)",
	})
	if !regexp.MustCompile(`(?s)\*,\s*\*::before,\s*\*::after\s*\{\s*box-sizing:\s*border-box;`).MatchString(p16ReadStatic(t, main)) {
		t.Errorf("main.css no longer sets box-sizing: border-box on everything; the bars' tokens would then exclude their padding and border")
	}

	// The bordered search field is a 44px control plus its own 1px border on
	// both sides; the negative margin gives those 2px back to the bar.
	form := p31mRule(t, appBar, "", ".app-topbar > .map-search")
	p16MustDecl(t, "search form", form, map[string]string{"flex": "0 1 260px"})
	field := p31mRule(t, appBar, "", ".map-search__field")
	p16MustDecl(t, "search field", field, map[string]string{
		"min-height":   "var(--touch-target-min)",
		"border":       "1px solid var(--color-border)",
		"box-sizing":   "border-box",
	})
	p16MustDecl(t, "search field on a laptop", p31mRule(t, appBar, laptopMedia, ".map-search__field"), map[string]string{"margin-block": "-1px"})
	// The phone field has no border, so the margin must stay out of its rules.
	if _, ok := field.Decls["margin-block"]; ok {
		t.Errorf("the base .map-search__field rule holds margin-block; it belongs in the 900px block only, the phone field has no border to give back")
	}
	if strings.Contains(p16ReadStatic(t, "static/css/phone.css"), "margin-block: -1px") {
		t.Errorf("phone.css holds the laptop field margin; the phone is not changed by this plan")
	}
}
