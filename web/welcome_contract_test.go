package web

// Plan 03.4-10 contract tests: the first-time flow at /welcome (D-27, D-28,
// D-24b, LAYOUT-07). Like the other contract files these are static inspection
// of the embedded bytes; nothing here needs a database or a server, and none can
// prove how a browser lays the flow out (the owner walks it on a phone and a
// laptop after Phase 3.4 is built). The rendered page is checked end to end in
// internal/api/handlers/welcome_e2e_test.go. Every helper and test name carries
// the w10 prefix where a helper is needed.

import (
	"regexp"
	"strings"
	"testing"
)

var w10TemplateComment = regexp.MustCompile(`(?s)\{\{/\*.*?\*/\}\}`)

// w10Template returns the welcome template without its template comments, so a
// sentence quoted in a comment is never mistaken for copy.
func w10Template(t *testing.T) string {
	t.Helper()
	return w10TemplateComment.ReplaceAllString(stripReadTemplate(t, "welcome.html.tmpl"), "")
}

func w10Count(t *testing.T, label, text, needle string, want int) {
	t.Helper()
	if n := strings.Count(text, needle); n != want {
		t.Errorf("%s holds %q %d times, want %d", label, needle, n, want)
	}
}

// TestWelcomeFourStepsInOrder pins the four step titles and their order and the
// "Step n of 4" line above each (D-28).
//
// Amended 2026-10-27 by quick 261027-t5w (owner): the alerts ask became a pop-up
// after step 4, so the flow has four steps again and every label reads "of 4".
// The fifth section (Phase 3.3 plan 13, "of 5") is gone; the pop-up is pinned in
// TestWelcomeAlertsAsk. The function was TestWelcomeFiveStepsInOrder.
func TestWelcomeFourStepsInOrder(t *testing.T) {
	page := w10Template(t)

	titles := []string{
		"Welcome to Pinalert",
		"Disclaimer and policy",
		"Show what is happening near you",
		"About you",
	}
	pos := 0
	for i, title := range titles {
		n := i + 1
		want := `<h1 class="welcome__title" id="welcome-step-` + string(rune('0'+n)) + `-title" tabindex="-1">` + title + `</h1>`
		at := strings.Index(page[pos:], want)
		if at == -1 {
			t.Errorf("step %d title %q is missing or out of order", n, title)
			continue
		}
		pos += at + len(want)
		label := `<p class="welcome__label">Step ` + string(rune('0'+n)) + ` of 4</p>`
		w10Count(t, "welcome.html.tmpl", page, label, 1)
		w10Count(t, "welcome.html.tmpl", page, `<section class="welcome__step" id="welcome-step-`+string(rune('0'+n))+`"`, 1)
	}
	w10Count(t, "welcome.html.tmpl", page, `<section class="welcome__step"`, 4)
	if strings.Contains(page, " of 5") || strings.Contains(page, "Step 5") || strings.Contains(page, `id="welcome-step-5`) {
		t.Errorf("the flow has exactly four steps, every one counted of 4, and no fifth section")
	}

	// The laptop step list is drawn from the steps the handler passes.
	p13MustContain(t, "step list", page, `<ol class="welcome__steps"`, `{{range .Steps}}`, `data-step="{{.Number}}"`)

	// Phase 5's sentences are not rendered: the second official alerts card and
	// the Demo, with the second Disagree sentence it adds.
	//
	// Amended 2026-10-18 by Phase 3.3 plan 13 (03.4 D-27, decision 15): the
	// Alerts sentences ("Get alerts when it matters", "Allow alerts", "Critical
	// reports near your home area") are step 5's own words now and left this
	// list; the obsolete "Share, then Add to Home Screen" ban is gone because
	// the guide's real wording lives in device_guide.html.tmpl.
	for _, banned := range []string{
		"Severe official alerts", "Demo", "look around", "looking around",
	} {
		if strings.Contains(page, banned) {
			t.Errorf("welcome.html.tmpl holds %q, which belongs to a later phase", banned)
		}
		if strings.Contains(p421ReadStatic(t, "js/welcome.js"), banned) {
			t.Errorf("welcome.js holds %q, which belongs to a later phase", banned)
		}
	}

	// Step 1: three cards, the third the warning with the configured number.
	p13MustContain(t, "step 1", page,
		"Know what is happening around you.",
		"People nearby post short reports: a flooded road, heavy traffic, an open shelter, a power cut.",
		"See how far to trust each report.",
		"Others nearby confirm or dispute it. The more different places confirm a report, the more likely it is true, but a report can still be wrong.",
		"Pinalert cannot send help.",
		"It tells you a situation is happening near you. If someone is in danger, call {{.Chrome.Numbers.Primary}} first.",
		`class="welcome__card welcome__card--warning"`,
		"Live emergency updates from people nearby, confirmed by others who are there.")
	// Amended 2026-10-17 by Phase 3.4 plan 28: step 3 shows three cards of its
	// own (the viewing session), so the page holds six in all.
	// Amended 2026-10-27 by quick 261027-t5w: step 5 (and its one card) became
	// the pop-up, so the page holds six cards again.
	w10Count(t, "welcome.html.tmpl", page, `<li class="welcome__card`, 6)
}

// TestWelcomeStep2ReadsTheSharedText pins step 2: the scroll box holds the two
// legal partials the reader uses, is focusable and named, and Agree and
// Disagree are two buttons side by side. The Disagree message has one sentence.
func TestWelcomeStep2ReadsTheSharedText(t *testing.T) {
	page := w10Template(t)

	w10Count(t, "welcome.html.tmpl", page, `{{template "legal_disclaimer_body" .Legal}}`, 1)
	w10Count(t, "welcome.html.tmpl", page, `{{template "legal_policy_body" .Legal}}`, 1)
	box := regexp.MustCompile(`<div class="welcome__scrollbox legal-text"[^>]*>`).FindString(page)
	if box == "" {
		t.Fatalf("no scroll box with the legal-text class")
	}
	p13MustContain(t, "scroll box", box, `tabindex="0"`, `aria-label="Disclaimer and policy text"`)
	// Amended 2026-10-27 by quick 261027-jhd (owner): the end-of-text marker comes
	// right after the policy partial, so it sits under the last line of the text
	// ("Last updated ..."), and Agree and Disagree come after the marker.
	p13Ordered(t, page, "step 2", `{{template "legal_disclaimer_body" .Legal}}`, `{{template "legal_policy_body" .Legal}}`, `id="welcome-scrollbox-end"`, `id="welcome-disagree"`, `id="welcome-agree"`)
	p13MustContain(t, "step 2", page, "Please read this to the end before you use Pinalert.")
	p13MustMatch(t, "step 2 scroll box", page,
		`(?s)<div class="welcome__scrollbox legal-text"[^>]*>\s*\{\{template "legal_disclaimer_body" \.Legal\}\}\s*\{\{template "legal_policy_body" \.Legal\}\}\s*<div class="welcome__scrollbox-end" id="welcome-scrollbox-end" aria-hidden="true"></div>\s*</div>`)
	w10Count(t, "welcome.html.tmpl", page, `id="welcome-scrollbox-end"`, 1)
	// The hold is the script's, so a page whose script never runs still shows Agree.
	p13MustNotContain(t, "welcome.html.tmpl", page, "data-unread")

	// Two buttons in one pair row, Disagree outlined and Agree filled.
	//
	// Amended 2026-10-27 by quick 261027-jhd (owner): the row has an id so
	// welcome.js can hold it back until the end of the text has been seen.
	p13MustContain(t, "step 2 buttons", page,
		`<div class="welcome__actions welcome__actions--pair" id="welcome-consent-actions">`,
		`<button type="button" class="btn" id="welcome-disagree">Disagree</button>`,
		`<button type="button" class="btn btn--primary" id="welcome-agree">Agree</button>`)

	// Amended 2026-10-17 by Phase 3.4 plan 31 (the owner asked what should happen
	// on Disagree and the suggestion was built): Disagree shows a calm box, drawn
	// by the template and hidden until then, so the words have one source and
	// welcome.js only shows and hides it. Owner amendments the same day: the
	// first wording was too wordy, then the call line was dropped, so the box is
	// one line, Sorry, you need to agree before you can continue., with no call link and no
	// number, and two buttons: Read again and Log out (the app's existing log
	// out, a real POST form to /auth/logout as in the person menu, ending at the
	// sign in page). It is a group named by its sentence, not a status region,
	// because it holds buttons and takes focus.
	p13MustContain(t, "disagree box", page,
		`<div class="welcome__notice welcome__notice--box" id="welcome-disagree-message" role="group" aria-labelledby="welcome-disagree-lead" tabindex="-1" hidden>`,
		`<p class="welcome__notice-lead" id="welcome-disagree-lead">Sorry, you need to agree before you can continue.</p>`,
		`<button type="button" class="btn" id="welcome-reread">Read again</button>`,
		`<form class="welcome__notice-form" method="post" action="/auth/logout">`,
		`<button type="submit" class="btn" id="welcome-logout">Log out</button>`)
	w10Count(t, "welcome.html.tmpl", page, `action="/auth/logout"`, 1)
	w10Count(t, "welcome.html.tmpl", page, `id="welcome-disagree-message"`, 1)
	w10Count(t, "welcome.html.tmpl", page, "Sorry, you need to agree before you can continue.", 1)
	p13MustNotContain(t, "disagree box", page, "keep everyone safe", "Read it again", "In danger?", "welcome__notice-call", "welcome__notice-danger")
	// The box sits under the buttons it answers, and its own buttons come in
	// the order Read again, Log out.
	p13Ordered(t, page, "step 2", `id="welcome-agree"`, `id="welcome-disagree-message"`, `id="welcome-reread"`, `id="welcome-logout"`, `id="welcome-error-2"`)
	// The box holds no number and no call link, typed or built.
	boxHTML := page[strings.Index(page, `id="welcome-disagree-message"`):strings.Index(page, `id="welcome-error-2"`)]
	// Amended 2026-10-18 by Phase 3.3 plan 13 (03.4 D-27, decision 15): step 5
	// has a real status line (role="status" hidden), so the ban on a status
	// region now covers the Disagree box only, which is a group that takes focus.
	p13MustNotContain(t, "disagree box", boxHTML, `role="status"`)
	for _, banned := range []string{"112", "tel:", "CallHref", "Numbers", "<a "} {
		if strings.Contains(boxHTML, banned) {
			t.Errorf("the Disagree box holds %q; the owner dropped the call line", banned)
		}
	}

	js := stripJSComments(p421ReadStatic(t, "js/welcome.js"))
	if strings.Contains(js, "Sorry, you need to agree") || strings.Contains(js, "You need to agree") || strings.Contains(js, "DISAGREE_MESSAGE") {
		t.Errorf("welcome.js holds the Disagree sentence; the template is its one source")
	}
	// Disagree shows the box and moves focus into it, and records nothing.
	dis := jsFunctionBody(t, js, "disagree()")
	p13MustContain(t, "disagree", dis, "disagreeBox.hidden = false;", "disagreeBox.focus();")
	p13MustNotContain(t, "disagree", dis, "request(", "fetch(", "CONSENT_URL")
	// Read again scrolls the text box to the top and moves focus to it.
	reread := jsFunctionBody(t, js, "readAgain()")
	p13MustContain(t, "readAgain", reread, "scrollbox.scrollTop = 0;", "scrollbox.focus();")
	p13MustNotContain(t, "readAgain", reread, "request(", "fetch(")
	p13MustContain(t, "welcome.js", js, "el('welcome-reread').addEventListener('click', readAgain);")
	// Log out is the form's own submit: no handler, no request of the script's own.
	p13MustNotContain(t, "welcome.js", js, "welcome-logout", "/auth/logout")
	// Agree hides the box at once; the box is never passed to hideLine, which
	// would empty its text.
	agree := jsFunctionBody(t, js, "agree()")
	p13MustContain(t, "agree", agree, "hideDisagreeBox();")
	p13MustNotContain(t, "welcome.js", js, "hideLine(disagreeMessage)", "showLine(disagreeMessage")
}

// TestWelcomeAgreeAfterReading pins the pieces of quick 261027-jhd (owner,
// 2026-10-27) that live in the stylesheet and the script: step 2 holds Agree and
// Disagree back until the end of the text has been in view, then shows them in
// the same place. The behaviour is run in welcome_flow_test.mjs; this file pins
// what a Node fake cannot see.
func TestWelcomeAgreeAfterReading(t *testing.T) {
	css := stripCSSComments(p421ReadStatic(t, "css/welcome.css"))
	rules := parseCSSRules(css)

	// The held back row keeps its place: visibility only, so nothing moves when
	// the buttons appear, and they leave Tab and the accessibility tree. One
	// declaration is the no-motion proof for the reveal.
	if r, ok := ruleBySelector(rules, ".welcome__actions--pair[data-unread]"); !ok {
		t.Errorf("welcome.css has no .welcome__actions--pair[data-unread] rule")
	} else {
		d := declsOf(r.declBody)
		if len(d) != 1 || d["visibility"] != "hidden" {
			t.Errorf("the held back row must set visibility: hidden and nothing else, got %v", d)
		}
	}
	// The end marker is 1px tall, not zero, so every browser reports it as in view.
	if r, ok := ruleBySelector(rules, ".welcome__scrollbox-end"); !ok {
		t.Errorf("welcome.css has no .welcome__scrollbox-end rule")
	} else if d := declsOf(r.declBody); d["height"] != "1px" {
		t.Errorf("the end marker is %q high, want 1px", d["height"])
	}

	js := stripJSComments(p421ReadStatic(t, "js/welcome.js"))
	// The watcher is rooted in the text box, so reading to the end with
	// VoiceOver counts like scrolling, and it is dropped once the end is seen.
	p13MustContain(t, "welcome.js", js, "window.IntersectionObserver", "root: scrollbox", ".disconnect()", "function revealConsent()", "function watchForEnd()")
	// The focus trap must skip a held back row.
	p13MustContain(t, "isShown", jsFunctionBody(t, js, "isShown(node)"), "hasAttribute(")
	// Read again only scrolls and focuses: it never holds the buttons back again.
	p13MustNotContain(t, "readAgain", jsFunctionBody(t, js, "readAgain()"), "revealConsent", "watchForEnd")
}

// TestWelcomeDisagreeBoxStyle pins the look of the Disagree box (plan 31): the
// existing .welcome__notice family, tokens only, square corners like the other
// buttons (never a pill), no dark outline, and the two buttons side by side at
// 48px high.
func TestWelcomeDisagreeBoxStyle(t *testing.T) {
	css := stripCSSComments(p421ReadStatic(t, "css/welcome.css"))
	rules := parseCSSRules(css)
	for _, sel := range []string{".welcome__notice--box:not([hidden])", ".welcome__notice-lead", ".welcome__notice-actions"} {
		if _, ok := ruleBySelector(rules, sel); !ok {
			t.Errorf("welcome.css has no %s rule", sel)
		}
	}
	if r, ok := ruleBySelector(rules, ".welcome__notice--box:not([hidden])"); ok {
		d := declsOf(r.declBody)
		if d["border"] != "" && strings.Contains(d["border"], "var(--color-text)") {
			t.Errorf("the Disagree box has a dark outline: %q", d["border"])
		}
		if strings.Contains(d["border-radius"], "999") || strings.Contains(d["border-radius"], "50%") {
			t.Errorf("the Disagree box is not a pill: %q", d["border-radius"])
		}
	}
	if r, ok := ruleBySelector(rules, ".welcome__notice-actions .btn"); !ok {
		t.Errorf("welcome.css has no .welcome__notice-actions .btn rule")
	} else if d := declsOf(r.declBody); d["min-height"] != "48px" {
		t.Errorf("the Disagree box buttons are %q high, want 48px like the other step buttons", d["min-height"])
	}
	for _, banned := range []string{"999px"} {
		if strings.Contains(css, banned) {
			t.Errorf("welcome.css holds %q", banned)
		}
	}
}

// TestWelcomeHasNoCloseControl pins that the flow is completed and not closed:
// no X, no Close, no Cancel, and no Escape handling (UI-SPEC Surface 7, D-27).
func TestWelcomeHasNoCloseControl(t *testing.T) {
	page := w10Template(t)
	for _, banned := range []string{`aria-label="Close"`, "app-window__close", "app-icon--x", "welcome-close", ">Close<", ">Cancel<", ">Done<", "welcome__close"} {
		if strings.Contains(page, banned) {
			t.Errorf("welcome.html.tmpl holds %q: the flow has no close control", banned)
		}
	}
	js := stripJSComments(p421ReadStatic(t, "js/welcome.js"))
	if strings.Contains(js, "Escape") {
		t.Errorf("welcome.js handles Escape: it must do nothing")
	}
	for _, banned := range []string{"root.addEventListener('click'", "document.addEventListener('keydown'"} {
		if strings.Contains(js, banned) {
			t.Errorf("welcome.js uses %q: no click on the page closes or moves the flow", banned)
		}
	}

	// The dialog is modal, labelled by the current step title, and the page
	// opens its body with the flow and no bar or strip.
	p13MustContain(t, "dialog", page, `role="dialog" aria-modal="true" aria-labelledby="welcome-step-{{.StartStep}}-title"`, `data-start-step="{{.StartStep}}"`)
	for _, banned := range []string{"top-chrome", "notice_strip", "app_topbar", "app_bottombar", "account_header"} {
		if strings.Contains(page, banned) {
			t.Errorf("welcome.html.tmpl holds %q: the flow is a page of its own with no bar", banned)
		}
	}
}

// TestWelcomeStep3And4Privacy pins the location and home area rules: step 3 has
// two buttons only (plan 33), there is no reverse lookup, the device position is
// asked in one place and sent nowhere, step 3 sends no place search, and the
// home area is only ever a picked result of step 4's own search (D-24b,
// research open question 3).
func TestWelcomeStep3And4Privacy(t *testing.T) {
	page := w10Template(t)

	// Plan 33: nothing fills the home area from step 3 any more, so the
	// Filled in from your location line is gone with it. The help line says what
	// the home area is for in the owner's words.
	p13MustNotContain(t, "step 4", page, "Filled in from your location.", "welcome-home-prefilled")
	w10Count(t, "welcome.html.tmpl", page, `id="welcome-home-help"`, 1)
	p13Ordered(t, page, "home area field", `id="welcome-home-input"`, "Used to send you alerts about this area.")

	// Amended 2026-10-17 by Phase 3.4 plan 28: the lone pin tile and the lead
	// sentence became three cards; the privacy fact (never shown, never stored)
	// is the third card.
	p13MustContain(t, "step 3", page,
		"See the reports around you.", "The map and the Nearby list start where you are.",
		"Post from where you are.", "When you post a report, the pin is placed for you.",
		"Your exact location stays private.", "It is never shown to anyone and never stored.",
		`<div class="welcome__actions welcome__actions--under">`,
		`<button type="button" class="btn btn--primary" id="welcome-allow-location">Allow location</button>`,
		`<button type="button" class="btn" id="welcome-not-now">Not now</button>`)
	// Plan 33 (the owner, 2026-10-18): step 3 is only Allow location or Not now.
	// The typed location search, its box, its results and its Continue are gone,
	// so nothing in step 3 can send a place search.
	step3 := regexp.MustCompile(`(?s)<section class="welcome__step" id="welcome-step-3".*?</section>`).FindString(page)
	if step3 == "" {
		t.Fatalf("no step 3 section in welcome.html.tmpl")
	}
	w10Count(t, "step 3", step3, `class="btn`, 2)
	w10Count(t, "step 3", step3, "<input", 0)
	w10Count(t, "step 3", step3, "<form", 0)
	p13Ordered(t, step3, "step 3 buttons", `id="welcome-allow-location">Allow location</button>`, `id="welcome-not-now">Not now</button>`)
	p13MustNotContain(t, "welcome.html.tmpl", page,
		"welcome-search", "welcome__search", "welcome-continue-3", "Type my location instead", "Type your location",
		"Koramangala", "welcome__tile", "Search a place")
	p13MustContain(t, "step 4", page,
		"Both are optional. They show only if you later let others follow you.",
		`placeholder="For example: Ravi Kumar"`,
		`value="{{.DisplayName}}"`, `value="{{.HomeAreaName}}"`,
		`id="welcome-continue-4">Continue</button>`, `id="welcome-skip">Skip</button>`)

	raw := p421ReadStatic(t, "js/welcome.js")
	js := stripJSComments(raw)

	// One browser position request, in allowLocation, sent nowhere.
	w10Count(t, "welcome.js", js, "navigator.geolocation.getCurrentPosition(", 1)
	allow := jsFunctionBody(t, js, "allowLocation()")
	p13MustContain(t, "allowLocation", allow, "navigator.geolocation.getCurrentPosition(", "window.PinalertPosition.save(coords.latitude, coords.longitude)")
	p13MustNotContain(t, "allowLocation", allow, "request(", "fetch(", "JSON.stringify", "homeInput", "homePick", "step3Pick")
	w10Count(t, "welcome.js", js, "latitude", 1)
	w10Count(t, "welcome.js", js, "longitude", 1)

	// No reverse lookup and no stored position of its own.
	for _, banned := range []string{"reverse", "nominatim", "localStorage", "watchPosition", "&lat=", "&lon="} {
		if strings.Contains(strings.ToLower(js), strings.ToLower(banned)) {
			t.Errorf("welcome.js uses %q: there is no reverse lookup and the position is saved only through PinalertPosition", banned)
		}
	}
	// The flag for the Nearby search focus is the only session storage use.
	w10Count(t, "welcome.js", js, "sessionStorage", 1)
	p13MustContain(t, "welcome.js", js, "window.sessionStorage.setItem(FOCUS_SEARCH_KEY, '1')", "var FOCUS_SEARCH_KEY = 'pinalert.focusSearch';")

	// The searches and the calls, each by its own address.
	p13MustContain(t, "welcome.js", js,
		"'/api/geocode/place?kind=area&q='",
		"'/api/me/consent'", "'/api/me/profile'", "'/api/me/first-run-done'", "'PATCH'", "'POST'")

	// Plan 33: step 3 has no place search and step 4 is not prefilled from it.
	// The only place search left is step 4's own, in areas mode; the report
	// form's address search is a different script.
	p13MustNotContain(t, "welcome.js", js,
		"'/api/geocode?q='", "/api/geocode?", "step3", "Step3", "STEP3", "welcome-search", "enterStep4", "homePrefilled", "lastPrefill", "homeTouched")
	w10Count(t, "welcome.js", js, "makeSearch({", 1)
	notNow := jsFunctionBody(t, js, "notNow()")
	p13MustContain(t, "notNow", notNow, "goTo(4)")
	p13MustNotContain(t, "notNow", notNow, "geolocation", "PinalertPosition", "request(", "fetch(", "homeInput", "homePick")

	// The home area body is built from a picked place only.
	body := jsFunctionBody(t, js, "profileBody()")
	p13MustContain(t, "profileBody", body, "if (homePick !== null) {", "home_area", "homePick.name", "homePick.lat", "homePick.lon", "display_name")
	p13MustNotContain(t, "profileBody", body, "homeInput.value", "PinalertPosition", "geolocation", "step3Pick")
}

// TestWelcomeStepsAreFreelyNavigable pins plan 30: a person can move back and
// forth between the four steps. Steps 2 to 4 carry a Back button (the arrow-left
// glyph and the word Back) beside the Step n of 4 label, each name in the laptop
// step list is a button that shows its step with the current one marked
// aria-current, the step goes in the address with history, and finishing still
// needs the agreement. The consent rule itself lives on the server and is not
// touched by this plan (internal/api/handlers/welcome_e2e_test.go).
func TestWelcomeStepsAreFreelyNavigable(t *testing.T) {
	page := w10Template(t)

	// Back on steps 2 to 4 only, in the head line with the step label.
	// Amended 2026-10-27 by quick 261027-t5w: the loop runs to step 4 and every
	// label reads "of 4"; the Back count is three.
	for n := 2; n <= 4; n++ {
		d := string(rune('0' + n))
		p13MustContain(t, "step "+d, page,
			`<button type="button" class="welcome__back" id="welcome-back-`+d+`"><span class="app-icon app-icon--arrow-left" aria-hidden="true"></span><span class="welcome__back-label">Back</span></button>`)
		p13Ordered(t, page, "step "+d+" head", `id="welcome-back-`+d+`"`, `<p class="welcome__label">Step `+d+` of 4</p>`, `id="welcome-step-`+d+`-title"`)
	}
	p13MustNotContain(t, "step 1", page, `id="welcome-back-1"`, `id="welcome-back-5"`)
	w10Count(t, "welcome.html.tmpl", page, `class="welcome__head"`, 3)
	w10Count(t, "welcome.html.tmpl", page, `class="welcome__back"`, 3)

	// Each name in the step list is a button; the current one is marked on it.
	p13MustContain(t, "step list", page,
		`<button type="button" class="welcome__steps-button" data-step="{{.Number}}"{{if eq .Number $.StartStep}} aria-current="step"{{end}}>`,
		`<span class="welcome__steps-name">{{.Name}}</span></button></li>`)
	w10Count(t, "welcome.html.tmpl", page, `aria-current="step"`, 1)

	// The script wires them, keeps the step in the address, honours the browser
	// back, and holds Finish to the agreement.
	js := stripJSComments(p421ReadStatic(t, "js/welcome.js"))
	p13MustContain(t, "welcome.js", js,
		"'.welcome__steps-button'", "el('welcome-back-' + n)", "goTo(n - 1)",
		"window.history.pushState({ pinalertStep: n }, '', stepAddress(n))",
		"window.history.replaceState({ pinalertStep: n }, '', stepAddress(n))",
		"window.addEventListener('popstate'", "goTo(n, 'none')",
		"var CONSENT_STEP = 2;")
	finish := jsFunctionBody(t, js, "finish(save)")
	p13Ordered(t, finish, "finish", "if (!agreed) {", "goTo(CONSENT_STEP);", "showLine(consentError, MESSAGE_AGREE_FIRST);", "setFinishing(true);")
	// An address step never skips the agreement for an account that has not agreed.
	first := jsFunctionBody(t, js, "firstStep(serverStep)")
	p13MustContain(t, "firstStep", first, "serverStep > CONSENT_STEP ? stepCount : CONSENT_STEP", "return serverStep;")
	// The server rule is still handled: a 409 sends the person back to step 2.
	post := jsFunctionBody(t, js, "postDone()")
	p13MustContain(t, "postDone", post, "answer.status === 409", "goTo(2);", "MESSAGE_AGREE_FIRST")

	css := stripCSSComments(p421ReadStatic(t, "css/welcome.css"))
	p13MustContain(t, "welcome.css", css, ".welcome__head {", ".welcome__back {", ".welcome__steps-button {", "min-height: 44px;", ".welcome__steps-button:focus-visible {")
}

// TestWelcomeScriptsAndStyles pins the load order, the sinks, the hidden
// guards, the two sizes and the plain punctuation of every file this plan adds.
func TestWelcomeScriptsAndStyles(t *testing.T) {
	page := w10Template(t)

	// The theme script is in the head, the two scripts load deferred in order.
	p13Ordered(t, page, "scripts", `/static/js/theme.js?v={{.AssetVersion}}"></script>`, "</head>",
		`/static/js/position.js?v={{.AssetVersion}}" defer></script>`, `/static/js/welcome.js?v={{.AssetVersion}}" defer></script>`)
	// Amended 2026-10-27 by quick 261027-t5w: auth.css comes second, as in
	// settings.html.tmpl, because the alerts pop-up reuses the Log out box rules.
	p13Ordered(t, page, "stylesheets", "/static/css/main.css", "/static/css/auth.css", "/static/css/chrome.css", "/static/css/app-bar.css", "/static/css/legal.css", "/static/css/window.css", "/static/css/welcome.css")

	for _, name := range []string{"js/welcome.js", "js/position.js"} {
		js := stripJSComments(p421ReadStatic(t, name))
		for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(", "new Function", "DOMParser"} {
			if strings.Contains(js, sink) {
				t.Errorf("%s uses the markup parsing sink %q", name, sink)
			}
		}
	}
	p13MustContain(t, "welcome.js", p421ReadStatic(t, "js/welcome.js"), "textContent", "window.PinalertWelcome = {")
	w10Count(t, "position.js", p421ReadStatic(t, "js/position.js"), "pinalert.lowLastPosition", 1)

	css := stripCSSComments(p421ReadStatic(t, "css/welcome.css"))
	for _, head := range []string{".welcome__step:not([hidden])", ".welcome__notice:not([hidden])"} {
		p13MustContain(t, "welcome.css", css, head+" {")
	}
	// Mobile first with one laptop block, no phone.css rule needed.
	w10Count(t, "welcome.css", css, "@media (min-width: 900px) {", 1)
	p13MustContain(t, "welcome.css", css, "grid-template-columns: 300px", "width: min(980px, 94%)", "height: min(680px, 92vh)", "box-shadow: var(--shadow-window)", "background: var(--color-notice-bg)")
	if strings.Contains(css, "!important") {
		t.Errorf("welcome.css uses !important")
	}
	if m := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindString(css); m != "" {
		t.Errorf("welcome.css holds the hex colour %q, tokens only", m)
	}
	if strings.Contains(page, "phone.css") {
		t.Errorf("welcome.html.tmpl links phone.css: the page is mobile first and needs none")
	}

	// No em dash or en dash in any file this plan adds, copy or comment.
	files := map[string]string{
		"templates/welcome.html.tmpl": stripReadTemplate(t, "welcome.html.tmpl"),
		"js/welcome.js":               p421ReadStatic(t, "js/welcome.js"),
		"js/position.js":              p421ReadStatic(t, "js/position.js"),
		"css/welcome.css":             p421ReadStatic(t, "css/welcome.css"),
	}
	for name, text := range files {
		for _, dash := range []string{"\u2014", "\u2013"} {
			if strings.Contains(text, dash) {
				t.Errorf("%s holds the dash character %q", name, dash)
			}
		}
	}
}

// TestWelcomeAlertsAsk pins the alerts pop-up that opens after step 4 (quick
// 261027-t5w, owner, 2026-10-27, which replaced step 5, Alerts, of Phase 3.3
// plan 13): a native modal dialog that is a direct child of body after the flow
// root, with the Log out box markup, the title, no supporting line, Not now then
// Allow, the key on the flow root, and not a trace of the removed step 5 page.
func TestWelcomeAlertsAsk(t *testing.T) {
	page := w10Template(t)

	const open = `<dialog id="welcome-alerts-ask" class="logout-confirm" aria-labelledby="welcome-alerts-ask-title">`
	w10Count(t, "welcome.html.tmpl", page, open, 1)
	w10Count(t, "welcome.html.tmpl", page, "<dialog", 1)
	// A direct child of body: it comes right after the closing of the #welcome
	// root (the last step section, the pane and the root) and before the scripts,
	// so the welcome Tab trap never sees its keys and the native modal can make
	// #welcome inert.
	p13MustMatch(t, "alerts pop-up position", page, `(?s)</section>\s*</div>\s*</div>\s*`+regexp.QuoteMeta(open))
	p13MustMatch(t, "alerts pop-up position", page, `(?s)</dialog>\s*<script src="/static/js/position\.js`)

	ask := regexp.MustCompile(`(?s)<dialog id="welcome-alerts-ask".*?</dialog>`).FindString(page)
	if ask == "" {
		t.Fatalf("no alerts pop-up in welcome.html.tmpl")
	}
	p13MustContain(t, "alerts pop-up", ask,
		`<div class="logout-confirm__body">`,
		`<h2 class="logout-confirm__title" id="welcome-alerts-ask-title" tabindex="-1">Get alerts for critical reports near you?</h2>`,
		`<div class="logout-confirm__actions">`,
		`<button type="button" class="btn" id="welcome-alerts-later">Not now</button>`,
		`<button type="button" class="btn btn--primary" id="welcome-alerts-allow">Allow</button>`)
	p13Ordered(t, ask, "alerts pop-up", `id="welcome-alerts-ask-title"`, `class="logout-confirm__actions"`, `id="welcome-alerts-later"`, `id="welcome-alerts-allow"`)
	// D-A: the Log out box is a title and buttons, so there is no supporting line,
	// and nothing is chosen on open.
	p13MustNotContain(t, "alerts pop-up", ask, "autofocus", "<p", "<form", "<a ", "<input")
	w10Count(t, "alerts pop-up", ask, "<button", 2)
	for _, id := range []string{"welcome-alerts-ask", "welcome-alerts-ask-title", "welcome-alerts-later", "welcome-alerts-allow"} {
		w10Count(t, "welcome.html.tmpl", page, `id="`+id+`"`, 1)
	}

	// The key rides on the flow root: an empty key means no pop-up (welcome.js).
	root := regexp.MustCompile(`<div class="welcome" id="welcome"[^>]*>`).FindString(page)
	p13MustContain(t, "flow root", root, `data-push-key="{{.PushPublicKey}}"`)

	// The page holds none of the removed step 5 parts.
	p13MustNotContain(t, "welcome.html.tmpl", page,
		`id="welcome-step-5`, `id="welcome-back-5"`,
		"welcome__example", "welcome__alerts-part",
		`id="welcome-alerts-install"`, `id="welcome-alerts-blocked"`, `id="welcome-alerts-unsupported"`,
		`id="welcome-alerts-status"`, `id="welcome-alerts-continue"`, `id="welcome-alerts-error"`,
		`id="welcome-alerts-ready"`, `id="welcome-alerts-card"`, `id="welcome-alerts-nohome"`,
		".AlertExample", ".HasHomeArea", "Get alerts when it matters", "Allow alerts")
	// The install guide lives only in Settings, as before.
	w10Count(t, "welcome.html.tmpl", page, `{{template "device_guide"}}`, 0)
	w10Count(t, "welcome.html.tmpl", page, "/static/css/device-guide.css?v={{.AssetVersion}}", 0)
	p13MustNotContain(t, "welcome.html.tmpl", page, "Home Screen", "Install to get alerts")

	// Nothing in the page promises a sound, and no long dash is written.
	if got := noiseWordsIn(htmlTagPattern.ReplaceAllString(templateCopy(stripReadTemplate(t, "welcome.html.tmpl")), " ")); len(got) != 0 {
		t.Errorf("welcome.html.tmpl holds the noise words %v", got)
	}
	for _, dash := range []string{string(rune(0x2013)), string(rune(0x2014))} {
		if strings.Contains(stripReadTemplate(t, "welcome.html.tmpl"), dash) {
			t.Errorf("welcome.html.tmpl holds the long dash U+%04X", []rune(dash)[0])
		}
	}

	// The progress bar has four even stops and none of the step 5 rules are left.
	css := stripCSSComments(p421ReadStatic(t, "css/welcome.css"))
	rules := parseCSSRules(css)
	for sel, want := range map[string]string{
		`.welcome__progress-fill`:                         "25%",
		`.welcome[data-step="2"] .welcome__progress-fill`: "50%",
		`.welcome[data-step="3"] .welcome__progress-fill`: "75%",
		`.welcome[data-step="4"] .welcome__progress-fill`: "100%",
	} {
		r, ok := ruleBySelector(rules, sel)
		if !ok {
			t.Errorf("welcome.css has no %s rule", sel)
			continue
		}
		if got := declsOf(r.declBody)["width"]; got != want {
			t.Errorf("%s is %q wide, want %q", sel, got, want)
		}
	}
	p13MustNotContain(t, "welcome.css", css,
		`data-step="5"`, ".welcome__example", ".welcome__alerts-part", ".welcome__status", ".btn[hidden]")
	// auth.css carries the whole look of the pop-up, so welcome.css adds nothing.
	p13MustNotContain(t, "welcome.css", css, "logout-confirm")
}

// TestWelcomeAlertsScript pins how welcome.js runs the alerts pop-up (quick
// 261027-t5w; before it, Phase 3.3 plan 13, T-03.3-25, T-03.3-15, decision 15):
// the browser's prompt runs only from the Allow tap, through the one enable
// call, inside the click and before anything is awaited; Not now and Escape never
// reach it; first-run-done has one sender; and nothing of step 5 is left.
func TestWelcomeAlertsScript(t *testing.T) {
	raw := p421ReadStatic(t, "js/welcome.js")
	js := stripJSComments(raw)

	// One call to enable, with the page's key; the script never names the prompt
	// itself, never asks the device state (a request) and has no confirm box.
	w10Count(t, "welcome.js", js, "PinalertNotify.enable(pushKey)", 1)
	p13MustNotContain(t, "welcome.js", js, "requestPermission", "Notification.", "serviceWorker", "pushManager", "deviceState", "confirm(")
	w10Count(t, "welcome.js", js, "'welcome-alerts-ask'", 1)

	// The Allow handler: busy check, then enable as the first real statement,
	// then the busy flags. Nothing is awaited before enable.
	allow := jsFunctionBody(t, js, "onAskAllow()")
	p13Ordered(t, allow, "onAskAllow", "if (askBusy) {", "window.PinalertNotify.enable(pushKey)", "askBusy = true;", ".then(")
	p13MustNotContain(t, "onAskAllow", allow, "await ", "setTimeout(")
	// Not now and Escape never reach enable.
	for _, name := range []string{"onAskLater()", "onAskCancel(e)", "endFlow()", "openAsk()", "canAsk()"} {
		p13MustNotContain(t, name, jsFunctionBody(t, js, name), "enable(", "requestPermission")
	}
	p13MustContain(t, "wiring", js,
		"askAllow.addEventListener('click', onAskAllow);",
		"askLater.addEventListener('click', onAskLater);",
		"askDialog.addEventListener('cancel', onAskCancel);")
	// Escape is the cancel event, handled there and never from a close event.
	p13MustContain(t, "onAskCancel", jsFunctionBody(t, js, "onAskCancel(e)"), "e.preventDefault();", "onAskLater()")
	p13MustNotContain(t, "welcome.js", js, "addEventListener('close'", "'Escape'")

	// The decision to ask reads the base state only, once, when step 4 finishes.
	can := jsFunctionBody(t, js, "canAsk()")
	p13MustContain(t, "canAsk", can, "askDialog", "showModal", "pushKey", "baseState()", "'maybe'", "'ready'")
	p13MustMatch(t, "canAsk", can, `(?s)try \{.*baseState\(\).*\} catch`)
	end := jsFunctionBody(t, js, "endFlow()")
	p13Ordered(t, end, "endFlow", "canAsk()", "openAsk()", "postDone()")
	p13MustContain(t, "openAsk", jsFunctionBody(t, js, "openAsk()"), "showModal()", "askTitle.focus()", "postDone()")

	// The flow is recorded as done from one sender, and a finish attempt reaches
	// it once: finish only ends through endFlow.
	w10Count(t, "welcome.js", js, "request(DONE_URL", 1)
	p13MustNotContain(t, "finish", jsFunctionBody(t, js, "finish(save)"), "postDone(")
	p13MustContain(t, "finish", jsFunctionBody(t, js, "finish(save)"), "endFlow(")
	post := jsFunctionBody(t, js, "postDone()")
	p13MustContain(t, "postDone", post, "answer.status === 409", "goTo(2);", "MESSAGE_AGREE_FIRST", "doneFailed();", "profileError")

	// The needs-install title and every step 5 helper are gone.
	p13MustNotContain(t, "welcome.js", js,
		"'Alerts'", "TITLE_LATER", "ALERTS_STEP", "alertsStep", "alertsSeq", "setAlertsBusy", "drawAlerts", "enterAlerts",
		"toAlerts", "alertsDone", "welcome-step-5", "welcome-alerts-status", "welcome-alerts-nohome", "hasHomeArea", "doneErrorLine",
		"'Install to get alerts'", "MESSAGE_ALERTS_")
}
