package web

// 09-REVIEW K2 (round 2, 2026-10-30): the persistent page host for the star box
// after "Yes, mark resolved". The message (#toast) hides and is emptied by any
// later message, and the card is gone once the resolve closes the report, so the
// box needs a node of its own beside the message. These tests read the shipped
// templates, stylesheet and script, never a copy.

import (
	"regexp"
	"strings"
	"testing"
)

// TestRateHostIsBesideTheMessageOnPagesThatCarryStars pins that index and the
// Activity page (profile.html.tmpl) carry one #rate-host directly after the
// message, with no live region attribute, and that the pages which cannot mount
// a box (no stars.js) carry none.
func TestRateHostIsBesideTheMessageOnPagesThatCarryStars(t *testing.T) {
	toast := `<div id="toast" role="status" aria-live="polite" hidden></div>`
	host := regexp.MustCompile(`(?s)<div id="toast" role="status" aria-live="polite" hidden></div>\s*(?:\{\{/\*.*?\*/\}\}\s*)?<div id="rate-host"></div>`)
	for _, name := range []string{"index.html.tmpl", "profile.html.tmpl"} {
		src := p421ReadTemplate(t, name)
		if n := strings.Count(src, toast); n != 1 {
			t.Errorf("%s: want one message element, found %d", name, n)
		}
		if n := strings.Count(src, `id="rate-host"`); n != 1 {
			t.Errorf("%s: want one #rate-host, found %d", name, n)
		}
		if !host.MatchString(src) {
			t.Errorf("%s: #rate-host must follow the message directly and carry no role or aria-live", name)
		}
		if !strings.Contains(src, "/static/js/stars.js") || !strings.Contains(src, "/static/css/stars.css") {
			t.Errorf("%s: a page with a host must load stars.js and stars.css", name)
		}
	}
	// Low data cannot carry stars.js or stars.css (TestLowDataAssetPolicy pins its
	// exact script and stylesheet lists), and the demo pages have no account and
	// no star script, so a host there could never hold a box.
	for _, name := range []string{"low.html.tmpl", "demo.html.tmpl", "demo_activity.html.tmpl"} {
		src := p421ReadTemplate(t, name)
		if strings.Contains(src, "rate-host") {
			t.Errorf("%s: carries a #rate-host but cannot load stars.js", name)
		}
		if strings.Contains(src, "/static/js/stars.js") {
			t.Errorf("%s: loads stars.js; revisit the host decision", name)
		}
	}
}

// TestRateHostStyle pins the placement: fixed beside the message, centred as the
// message is, above it while it shows, under it in the stack, empty takes no
// room, and the box inside drops its top margin.
func TestRateHostStyle(t *testing.T) {
	css := p421ReadStatic(t, "css/stars.css")
	noComments := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	rule := regexp.MustCompile(`(?s)#rate-host\s*\{(.*?)\}`).FindStringSubmatch(noComments)
	if rule == nil {
		t.Fatal("stars.css has no #rate-host rule")
	}
	// Amended 2026-10-30 (skeptic SK-W01): the z-index was 1190, above every
	// window and the phone report card; it is now 1070 (see
	// TestRateHostSitsUnderWindowsAndThePhoneReportCard).
	for _, want := range []string{"position: fixed;", "left: 50%;", "z-index: 1070;", "translateX(-50%)", "var(--rate-host-lift)"} {
		if !strings.Contains(rule[1], want) {
			t.Errorf("#rate-host lacks %q", want)
		}
	}
	main := p421ReadStatic(t, "css/main.css")
	toast := regexp.MustCompile(`(?s)\n#toast\s*\{(.*?)\}`).FindStringSubmatch(main)
	if toast == nil || !strings.Contains(toast[1], "z-index: 1200;") {
		t.Fatal("main.css #toast rule has no z-index 1200 to stay above the host")
	}
	for _, want := range []string{
		"#rate-host:empty {\n  display: none;",
		"#toast:not([hidden]) + #rate-host",
		"#rate-host .rate-box:not([hidden])",
		".app-shell > #rate-host",
		"max(calc(var(--touch-target-min) + var(--space-sm) + var(--space-md)), calc(var(--app-bottombar-height) + env(safe-area-inset-bottom)))",
	} {
		if !strings.Contains(noComments, want) {
			t.Errorf("stars.css lacks %q", want)
		}
	}
	if strings.Contains(rule[1], "aria-") || strings.Contains(noComments, "!important") {
		t.Errorf("#rate-host rule holds an aria or !important")
	}
}

// TestRateHostSitsUnderWindowsAndThePhoneReportCard pins SK-W01 (skeptic round,
// 2026-10-30): the hosted star box sits above the page and its menus but under
// the phone report card (1080) and every .app-window (1100), so it can never
// cover a control of a report or a sheet. The message (#toast, 1200) stays
// above it. The numbers are read from the shipped stylesheets.
func TestRateHostSitsUnderWindowsAndThePhoneReportCard(t *testing.T) {
	strip := regexp.MustCompile(`(?s)/\*.*?\*/`)
	z := func(file, selectorPattern string) int {
		t.Helper()
		src := strip.ReplaceAllString(p421ReadStatic(t, file), "")
		m := regexp.MustCompile(`(?s)` + selectorPattern + `\s*\{[^}]*?z-index:\s*(\d+);`).FindStringSubmatch(src)
		if m == nil {
			t.Fatalf("%s: no z-index found for %s", file, selectorPattern)
		}
		n := 0
		for _, c := range m[1] {
			n = n*10 + int(c-'0')
		}
		return n
	}
	host := z("css/stars.css", `#rate-host`)
	phoneCard := z("css/phone.css", `\.leaflet-container \.leaflet-detail-pane`)
	window := z("css/window.css", `\.app-window:not\(\[hidden\]\)`)
	toast := z("css/main.css", `#toast`)
	menu := z("css/feed.css", `\.show-menu`)
	if host >= phoneCard || host >= window {
		t.Errorf("#rate-host z-index %d must be under the phone report card %d and the windows %d", host, phoneCard, window)
	}
	if host >= toast {
		t.Errorf("#rate-host z-index %d must stay under the message %d", host, toast)
	}
	if host <= menu {
		t.Errorf("#rate-host z-index %d must stay above the page menus %d", host, menu)
	}
}

// TestVotesScriptEndsTheHostedBox pins that the script has the three ends of a
// hosted box (SK-W01) and that map.js calls the report-open end.
func TestVotesScriptEndsTheHostedBox(t *testing.T) {
	js := p421ReadStatic(t, "js/votes.js")
	for _, want := range []string{
		"dismissHostedRateBoxes: dismissHostedRateBoxes",
		"var RATE_HOST_MAX_MS = 30000;",
		"attributeFilter: ['hidden', 'open']",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("votes.js lacks %q", want)
		}
	}
	mapJS := p421ReadStatic(t, "js/map.js")
	if !strings.Contains(mapJS, "PinalertVotes.dismissHostedRateBoxes()") {
		t.Error("map.js does not drop the hosted box when a report opens")
	}
}

// TestVotesScriptMountsInTheRateHost pins the script side: the host is looked up
// by its id, used only when the card is not live, and the box built for it never
// moves focus on mount and leaves after its line.
func TestVotesScriptMountsInTheRateHost(t *testing.T) {
	js := p421ReadStatic(t, "js/votes.js")
	for _, want := range []string{
		"var RATE_HOST_ID = 'rate-host';",
		"document.getElementById(RATE_HOST_ID)",
		"if (!live) {",
		"buildRateBox(block, reportId, prompt.username, true)",
		"pageHost.appendChild(hosted);",
		"RATE_HOST_LINE_MS",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("votes.js lacks %q", want)
		}
	}
	i := strings.Index(js, "pageHost.appendChild(hosted);")
	if i < 0 {
		t.Fatal("no host mount")
	}
	around := js[i : i+120]
	if strings.Contains(around, ".focus(") {
		t.Errorf("the host mount moves focus: %q", around)
	}
}
