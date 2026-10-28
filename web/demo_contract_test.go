package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// This file holds the static contract tests of plan 05-13, the Demo's Nearby
// page (D-01, D-02, D-05, ROBUST-06, ROBUST-07). The Demo page is its own
// template that mirrors index.html.tmpl, so the many tests that pin the real
// page stay as they are. The real scripts run the Demo through two seams: the
// report API base (window.PinalertApiBase, default /api) and a fixed pretend
// position (data-fixed-lat and data-fixed-lon on the page shell). Nothing here
// needs a database or a server: the tests read the embedded static and
// template trees and inspect the shipped text, like the other contract files.
// Every helper carries the dm13 prefix so a later plan cannot clash with it.

// dm13Scripts returns the file names of the local scripts a template loads, in
// document order, without the version query.
func dm13Scripts(text string) []string {
	re := regexp.MustCompile(`<script src="/static/js/([a-z0-9-]+\.js)`)
	var names []string
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		names = append(names, m[1])
	}
	return names
}

// dm13Index is the position of name in names, or -1.
func dm13Index(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}

// dm13Code is a JavaScript text without the lines that are only a comment.
func dm13Code(text string) string { return p13CodeLines(text) }

// dm13OpenTag returns the opening tag of the element that carries attr, from
// its "<" to its ">".
func dm13OpenTag(t *testing.T, text, attr string) string {
	t.Helper()
	at := strings.Index(text, attr)
	if at == -1 {
		t.Fatalf("%q is not in the template", attr)
	}
	start := strings.LastIndex(text[:at], "<")
	end := strings.Index(text[at:], ">")
	if start == -1 || end == -1 {
		t.Fatalf("the tag carrying %q is not closed", attr)
	}
	return text[start : at+end+1]
}

// dm13Rule returns the body of the one CSS rule that starts with head, from the
// head to the first closing brace.
func dm13Rule(t *testing.T, css, head string) string {
	t.Helper()
	at := strings.Index(css, "\n"+head)
	if at == -1 && strings.HasPrefix(css, head) {
		at = -1
	} else if at == -1 {
		t.Fatalf("demo.css has no rule %q", head)
	}
	rest := css[at+1:]
	end := strings.Index(rest, "}")
	if end == -1 {
		t.Fatalf("the rule %q is not closed", head)
	}
	return rest[:end]
}

var dm13ActionRe = regexp.MustCompile(`\{\{-?\s*(if|range|define|with|end|else)\b[^}]*\}\}`)

// dm13GuardsOf returns the actions of every block that is open at the needle,
// outermost first, so a test can require that an element sits under a Demo
// guard. A block that has turned to its else branch is written "else of ...".
func dm13GuardsOf(t *testing.T, text, needle string) []string {
	t.Helper()
	at := strings.Index(text, needle)
	if at == -1 {
		t.Fatalf("%q is not in the template", needle)
	}
	var open []string
	for _, loc := range dm13ActionRe.FindAllStringSubmatchIndex(text[:at], -1) {
		action := text[loc[0]:loc[1]]
		switch text[loc[2]:loc[3]] {
		case "end":
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
		case "else":
			if len(open) > 0 && !strings.HasPrefix(open[len(open)-1], "else of ") {
				open[len(open)-1] = "else of " + open[len(open)-1]
			}
		default:
			open = append(open, action)
		}
	}
	return open
}

// dm13UnderDemoGuard says whether the needle sits in the true branch of an
// {{if not .Nav.Demo}} block.
func dm13UnderDemoGuard(t *testing.T, text, needle string) bool {
	t.Helper()
	for _, g := range dm13GuardsOf(t, text, needle) {
		if strings.Contains(g, "not .Nav.Demo") && !strings.HasPrefix(g, "else of ") {
			return true
		}
	}
	return false
}

// TestDemoPageMarkup pins the Demo Nearby page's frame: the strip first, the
// Demo top bar, the shell with the four data attributes the scripts read, the
// map and the list pane, and none of the parts the Demo leaves out.
func TestDemoPageMarkup(t *testing.T) {
	page := p13ReadTemplate(t, "demo.html.tmpl")

	p13Ordered(t, page, "demo page",
		`<body`, `{{template "demo_strip" .}}`, `<div class="top-chrome">`, `{{template "app_topbar" .}}`,
		`id="app-shell"`, `class="pane pane--map"`, `id="map"`, `class="pane pane--list"`,
		`id="report-list"`, `id="report-modal"`, `{{template "app_bottombar" .}}`)

	shell := dm13OpenTag(t, page, `id="app-shell"`)
	p13MustContain(t, "#app-shell", shell,
		`data-demo="1"`, `data-api-base="{{.APIBase}}"`, `data-fixed-lat="{{.FixedLat}}"`,
		`data-fixed-lon="{{.FixedLon}}"`, `data-vote-radius-km="{{.VoteRadiusKm}}"`,
		`data-default-radius-km="{{.DefaultRadiusKm}}"`)
	p13MustNotContain(t, "#app-shell", shell, "data-account-id", "data-push-key")

	// The Demo draws one strip. The laptop Unofficial strip, the Search a place
	// form, the flag dialog, the Only Reliable reporters item, the reader
	// window, the first-post pop-up and every tel link are left out.
	p13MustNotContain(t, "demo page", page,
		`notice_strip`, `place_search`, `flag_dialog`, `data-reliable`, `reader_dialog`,
		`changes_ask`, `tel:`, `TelHref`, `Only Reliable reporters`, `href="/alerts"`, `href="/profile`)

	// The strip partial and the page both name the same strip once.
	if n := strings.Count(page, `{{template "demo_strip"`); n != 1 {
		t.Errorf("demo.html.tmpl draws the Demo strip %d times, want 1", n)
	}
}

// TestDemoPageLoadsNoForbiddenScripts pins the script order and the scripts the
// Demo must not load: api-base.js is first and runs before app.js; the offline
// queue, the service worker, the bell, the flag and mute controls, the person
// menu, the reader window and the stored position never load.
func TestDemoPageLoadsNoForbiddenScripts(t *testing.T) {
	page := p13ReadTemplate(t, "demo.html.tmpl")
	names := dm13Scripts(page)

	want := []string{"api-base.js", "app.js", "votes.js", "visibility.js", "trust.js", "report-form.js",
		"sheet.js", "map.js", "modal.js", "ended-cards.js", "show-menu.js", "feed.js", "map-tools.js",
		"app-bar.js", "demo.js"}
	var local []string
	for _, n := range names {
		if n != "theme.js" {
			local = append(local, n)
		}
	}
	if strings.Join(local, ",") != strings.Join(want, ",") {
		t.Errorf("the Demo page loads %v, want %v in this order", local, want)
	}
	if dm13Index(names, "api-base.js") == -1 || dm13Index(names, "api-base.js") > dm13Index(names, "app.js") {
		t.Errorf("api-base.js must load before app.js: %v", names)
	}
	for _, banned := range []string{"notify.js", "queue.js", "flag.js", "mute.js", "updates.js",
		"account-menu.js", "position.js", "reader.js", "reliable.js", "settings-alerts.js", "slow-notice.js",
		"changes-ask.js", "low.js", "welcome.js"} {
		if dm13Index(names, banned) != -1 {
			t.Errorf("the Demo page must not load %s", banned)
		}
	}
	// The app_head partial registers the service worker (it loads notify.js).
	p13MustNotContain(t, "demo page", page, `template "app_head"`, "manifest", "serviceWorker")
}

// TestDemoPageHasNoAccountAndNoStorageWrites pins that the Demo page names no
// account and that the two scripts the plan adds write no browser storage.
func TestDemoPageHasNoAccountAndNoStorageWrites(t *testing.T) {
	page := p13ReadTemplate(t, "demo.html.tmpl")
	p13MustNotContain(t, "demo page", page, "data-account-id", "AccountID", ".Email")

	for _, name := range []string{"api-base.js", "demo.js"} {
		code := dm13Code(p13ReadStatic(t, "static/js/"+name))
		p13MustNotContain(t, name, code, "localStorage", "sessionStorage", "indexedDB", "IndexedDB",
			"lowLastPosition", "document.cookie", "navigator.geolocation", "serviceWorker")
	}
}

// TestDemoStripCopyAndExit pins the strip's words and the Exit button, and the
// stylesheet's size, underline and corner rules for them.
func TestDemoStripCopyAndExit(t *testing.T) {
	strip := p13ReadTemplate(t, "demo_strip.html.tmpl")
	p13MustContain(t, "demo strip", strip,
		`{{define "demo_strip"}}`, `<aside class="demo-strip" role="note" aria-label="Demo">`,
		`Demo. Nothing here is real.`, `class="btn demo-strip__exit"`, `href="{{.ExitHref}}"`,
		`aria-label="Exit the demo"`, `>Exit demo</a>`)
	p13MustNotContain(t, "demo strip", strip, "<button", "tel:")

	css := p13ReadStatic(t, "static/css/demo.css")
	block := dm13Rule(t, css, ".demo-strip {")
	p13MustContain(t, ".demo-strip", block, "min-height: 52px", "background: var(--color-surface)",
		"border-bottom: 2px solid var(--color-text)", "display: flex", "gap: 12px", "padding: 4px 16px",
		"align-items: center")

	exit := dm13Rule(t, css, ".demo-strip__exit {")
	p13MustContain(t, ".demo-strip__exit", exit, "min-height: 44px", "text-decoration: none")
	p13MustNotContain(t, "demo.css", css, "border-radius: 999", "border-radius: 50%", "border-radius: 9999", "opacity:")
}

// TestDemoNavHidesBellGearSearch pins that the app bar draws no Search a place
// form, no bell, no gear and no person menu on the Demo, keeps the theme toggle,
// and that the Demo's places point at the Demo.
func TestDemoNavHidesBellGearSearch(t *testing.T) {
	bar := p13ReadTemplate(t, "app_bar.html.tmpl")
	for _, needle := range []string{`place_search.html.tmpl" .}}`, `id="bell-trigger"`,
		`class="app-topbar__gear"`, `template "account_header_menu" .`} {
		if !dm13UnderDemoGuard(t, bar, needle) {
			t.Errorf("%s is not under {{if not .Nav.Demo}}; the open blocks are %q", needle, dm13GuardsOf(t, bar, needle))
		}
	}
	// The theme toggle stays on the laptop bar in the Demo.
	if dm13UnderDemoGuard(t, bar, `template "theme_toggle.html.tmpl" .`) {
		t.Error("the theme toggle must stay in the Demo, but it sits under {{if not .Nav.Demo}}")
	}

	nav, err := os.ReadFile("../internal/api/handlers/nav.go")
	if err != nil {
		t.Fatalf("reading nav.go: %v", err)
	}
	p13MustContain(t, "nav.go", string(nav), "func demoNavFor(", "Demo bool",
		`"/demo"`, `"/demo/alerts"`, `"/demo/activity"`)
}

// TestApiBaseSeam pins that the three scripts that build report URLs hold no
// literal /api/reports path outside the base expression and read
// PinalertApiBase with the /api default.
func TestApiBaseSeam(t *testing.T) {
	for _, name := range []string{"app.js", "votes.js", "ended-nearby.js"} {
		code := dm13Code(p13ReadStatic(t, "static/js/"+name))
		p13MustContain(t, name, code, "PinalertApiBase", "'/api'")
		p13MustNotContain(t, name, code, "'/api/reports", `"/api/reports`)
	}
	base := p13ReadStatic(t, "static/js/api-base.js")
	p13MustContain(t, "api-base.js", base, "PinalertApiBase", "PinalertDemoPage", "data-api-base", "data-demo", "'/api'")
	// The base is checked before it is used: a path on this site only.
	p13MustMatch(t, "api-base.js", base, `/\^\\/`)

	// The app.js feed event for the banner (D-09).
	app := dm13Code(p13ReadStatic(t, "static/js/app.js"))
	p13MustContain(t, "app.js", app, "FEED_CENTRE_EVENT", "'pinalert:feed-centre'", "typeof CustomEvent")
}

// TestFixedPositionSeam pins that map.js, modal.js and votes.js read the page's
// fixed pretend position before any browser location call, and return early when
// it is there.
func TestFixedPositionSeam(t *testing.T) {
	for _, tc := range []struct {
		file, fn, call string
	}{
		{"map.js", "locateReader(onFix, onFail)", "navigator.geolocation.getCurrentPosition("},
		{"modal.js", "initLocation()", "navigator.geolocation.getCurrentPosition("},
		{"votes.js", "getVoterLocation(opts)", "navigator.geolocation.getCurrentPosition("},
	} {
		code := dm13Code(p13ReadStatic(t, "static/js/"+tc.file))
		p13MustContain(t, tc.file, code, "data-fixed-lat", "data-fixed-lon")
		body := jsFunctionBody(t, code, tc.fn)
		fixedAt := strings.Index(body, "fixedPosition()")
		callAt := strings.Index(body, tc.call)
		if fixedAt == -1 {
			t.Errorf("%s: %s does not read fixedPosition()", tc.file, tc.fn)
			continue
		}
		if callAt != -1 && fixedAt > callAt {
			t.Errorf("%s: %s reads the fixed position after the browser location call", tc.file, tc.fn)
		}
		if !strings.Contains(body[fixedAt:], "return") {
			t.Errorf("%s: %s does not return early on a fixed position", tc.file, tc.fn)
		}
	}
	// The hint that marks far reports reads the same position, so the row says
	// Too far to vote with no stored fix.
	votes := dm13Code(p13ReadStatic(t, "static/js/votes.js"))
	peek := jsFunctionBody(t, votes, "peekVoterLocation()")
	if !strings.Contains(peek, "fixedPosition()") {
		t.Error("votes.js: peekVoterLocation does not read the fixed position")
	}
}

// TestDemoMarksInFeedAndMap pins the marks (D-02, ROBUST-06, T-05-71): the Demo
// tag on every row and popup, the dashed ring class on every pin, and no Flag
// and no Mute control on the Demo page.
func TestDemoMarksInFeedAndMap(t *testing.T) {
	feed := dm13Code(p13ReadStatic(t, "static/js/feed.js"))
	create := jsFunctionBody(t, feed, "createRow(id)")
	p13MustContain(t, "feed.js createRow", create, "PinalertDemoPage", "addDemoRowMarks(body)")
	marks := jsFunctionBody(t, feed, "addDemoRowMarks(body)")
	p13MustContain(t, "feed.js addDemoRowMarks", marks, "demo-tag", "'Demo'", "'Demo. '", "report-row__stateline")
	if strings.Contains(create, "PinalertFlag") || strings.Contains(create, "PinalertMute") {
		t.Error("feed.js createRow draws a Flag or Mute control")
	}

	mapJS := dm13Code(p13ReadStatic(t, "static/js/map.js"))
	badge := jsFunctionBody(t, mapJS, "buildBadgeElement(report)")
	p13MustContain(t, "map.js buildBadgeElement", badge, "PinalertDemoPage", "icon-badge--demo")
	popup := jsFunctionBody(t, mapJS, "buildPopupCard(report)")
	p13MustContain(t, "map.js buildPopupCard", popup, "PinalertDemoPage", "demo-tag")
	// Flag and Mute are not drawn on the Demo page, whatever scripts loaded.
	p13MustMatch(t, "map.js mute", popup, `window\.PinalertMute\s*&&\s*!window\.PinalertDemoPage`)
	p13MustMatch(t, "map.js flag", popup, `window\.PinalertFlag\s*&&\s*!window\.PinalertDemoPage`)

	css := p13ReadStatic(t, "static/css/demo.css")
	ring := dm13Rule(t, css, "#map .icon-badge--pin.icon-badge--demo {")
	p13MustContain(t, ".icon-badge--demo", ring, "outline: 2px dashed var(--color-text)", "outline-offset: 2px")
	p13MustContain(t, "demo.css", css, ".demo-tag", "forced-colors: active")
}

// TestRealPageUnchangedByDemoSeams pins that the real Nearby page holds none of
// the Demo's attributes, so the seams default to the real behaviour, and that
// the real page still loads no Demo file.
func TestRealPageUnchangedByDemoSeams(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")
	p13MustNotContain(t, "index.html.tmpl", index, "data-demo", "data-api-base", "data-fixed-lat",
		"data-fixed-lon", "api-base.js", "demo.js", "demo.css", "demo_strip")
	low := p13ReadTemplate(t, "low.html.tmpl")
	p13MustNotContain(t, "low.html.tmpl", low, "api-base.js", "data-fixed-lat")
}

// This part is plan 05-17, the Demo's practice posting (D-01, D-05, UI-SPEC 4d,
// T-05-91 to T-05-95). The rendered pages are pinned in
// internal/api/handlers/demo_practice_e2e_test.go; here the shipped source is
// read. Every helper carries the dm17 prefix.

// dm17Sentence is the toast after a practice post, word for word (UI-SPEC 4d step 2).
const dm17Sentence = "Practice report posted. Pretend neighbours will confirm it in a few seconds."

// TestDemoPracticeFlowSource pins the shape of the practice flow: the poll
// length, the toast, the announcement prefix, the event the app raises and the
// rule that the page never types a trust sentence of the server's (T-05-92).
func TestDemoPracticeFlowSource(t *testing.T) {
	demo := p13ReadStatic(t, "static/js/demo.js")
	p13MustContain(t, "demo.js", demo, "PRACTICE_POLL_MS = 3000", dm17Sentence, "'Practice report confirmed. '",
		"pinalert:report-posted", "Pinalert.subscribe(", "openReport(", "function onPosted(", "function watchPractice(",
		"function stopWatching(", "function announceConfirmed(", "function onEndedVisit(", "confirmed_by_people")
	// The page reads the sentences from the server's trust object and types none.
	code := dm13Code(demo)
	p13MustNotContain(t, "demo.js code", code, "Needs 1 more person", "Needs 2 people", "Confirmed by ",
		"at different spots")

	app := p13ReadStatic(t, "static/js/app.js")
	p13MustContain(t, "app.js", app, "REPORT_POSTED_EVENT = 'pinalert:report-posted'", "PinalertDemoEnded", "'visit'")
}

// TestDemoFormGuardSource pins the Demo mode of the shared report form: the Call
// 112 box and the What is happening picks sit under one Demo guard, and the
// Demo's post window has no address search, no location notice and no tel link.
func TestDemoFormGuardSource(t *testing.T) {
	form := p13ReadTemplate(t, "report_form.html.tmpl")
	for _, needle := range []string{`id="call-112-box"`, `id="happening-fields"`} {
		guarded := false
		for _, g := range dm13GuardsOf(t, form, needle) {
			if g == "{{if not .Demo}}" {
				guarded = true
			}
		}
		if !guarded {
			t.Errorf("%s in report_form.html.tmpl is not under {{if not .Demo}}", needle)
		}
	}
	demoPage := p13ReadTemplate(t, "demo.html.tmpl")
	p13MustContain(t, "demo.html.tmpl", demoPage, "data-pretend-place=", `id="near-place"`)
	p13MustNotContain(t, "demo.html.tmpl", demoPage, `id="location-search"`, `id="location-search-results"`,
		`id="location-search-status"`, `id="location-notice"`, "location-search-field", "tel:")
}

// TestReportFormToleratesNoSearch pins that the form scripts accept a window
// without the address search and the location notice (the Demo's post window),
// and that the Demo's success toast is the practice sentence.
func TestReportFormToleratesNoSearch(t *testing.T) {
	form := p13ReadStatic(t, "static/js/report-form.js")
	p13MustContain(t, "report-form.js", form, "PinalertDemoPage")
	modal := p13ReadStatic(t, "static/js/modal.js")
	p13MustMatch(t, "modal.js", modal, `if \(locationNotice\)`)
	p13MustContain(t, "modal.js", modal, "data-pretend-place")
}

// ---- Plan 05-19: the Demo's other two places and the sample banner ----------
//
// Official alerts and My activity each have their own Demo template, with the
// same shell as the real page and the Demo strip first. The helpers carry the
// dm19 prefix.

// dm19Attribution is the one sentence the Demo's Official alerts page ends with
// (UI-SPEC 1a). The real page's attribution names the issuers; the Demo's
// names none, because nothing on it is from an issuer.
const dm19Attribution = "These alerts are made up for the Demo."

// TestDemoAlertsPageMarkup pins the Demo's Official alerts page (plan 05-19,
// D-05, D-06, ROBUST-06, UI-SPEC 4b and 4c): the Demo strip first, the same
// shell the real renderer fills, the Demo's API base and pretend position on
// the page root, no account, no stored position, and the Demo attribution in
// place of the real one.
func TestDemoAlertsPageMarkup(t *testing.T) {
	page := p13ReadTemplate(t, "demo_alerts.html.tmpl")

	p13Ordered(t, page, "demo alerts page",
		`<body`, `{{template "demo_strip" .}}`, `<div class="top-chrome">`, `{{template "app_topbar" .}}`,
		`id="official-page"`, `id="official-status"`, `id="official-section"`, `id="official-list"`,
		`id="official-attribution"`, `{{template "app_bottombar" .}}`)

	root := dm13OpenTag(t, page, `id="official-page"`)
	p13MustContain(t, "#official-page", root,
		`data-demo="1"`, `data-api-base="{{.APIBase}}"`, `data-fixed-lat="{{.FixedLat}}"`, `data-fixed-lon="{{.FixedLon}}"`)

	if n := strings.Count(page, dm19Attribution); n != 1 {
		t.Errorf("demo_alerts.html.tmpl holds the Demo attribution %d times, want 1", n)
	}
	attribution := dm13OpenTag(t, page, `id="official-attribution"`)
	at := strings.Index(page, attribution) + len(attribution)
	if got := page[at : at+strings.Index(page[at:], "</p>")]; got != dm19Attribution {
		t.Errorf("#official-attribution says %q, want %q", got, dm19Attribution)
	}

	// The real page names the issuers and holds an account id and the stored
	// position script. None of that belongs to the Demo.
	p13MustNotContain(t, "demo alerts page", page,
		"data-account-id", "AccountID", ".Email", "NDMA", "SACHET", "GDACS", "position.js", `template "app_head"`,
		"manifest", "serviceWorker", "tel:", `href="/alerts"`, `href="/profile`, `href="/settings`)

	names := dm13Scripts(page)
	want := []string{"api-base.js", "app-bar.js", "demo.js", "official-alerts.js"}
	var local []string
	for _, n := range names {
		if n != "theme.js" {
			local = append(local, n)
		}
	}
	if strings.Join(local, ",") != strings.Join(want, ",") {
		t.Errorf("the Demo alerts page loads %v, want %v in this order", local, want)
	}
	if dm13Index(names, "api-base.js") > dm13Index(names, "official-alerts.js") {
		t.Errorf("api-base.js must load before official-alerts.js: %v", names)
	}
	p13MustContain(t, "demo alerts page", page, "/static/css/official.css", "/static/css/demo.css")

	// The sentence is written by the page, and the renderer leaves it alone.
	script := dm13Code(p13ReadStatic(t, "static/js/official-alerts.js"))
	p13MustNotContain(t, "official-alerts.js code", script, "official-attribution")
}
