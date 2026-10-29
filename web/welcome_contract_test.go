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

	// Official alerts are a Settings switch only (owner 2026-10-27, quick
	// 261027-t5w), so "Severe official alerts" stays out of the flow for good.
	//
	// Amended 2026-10-29 by plan 05-18 (D-03): Phase 5 adds Try the demo to step 1
	// and the Disagree box, with the second Disagree sentence, so "Demo", "look
	// around" and "looking around" left this list. Every other pin stays; the new
	// pins are in demo_entry_contract_test.go.
	//
	// Amended 2026-10-18 by Phase 3.3 plan 13 (03.4 D-27, decision 15): the
	// Alerts sentences ("Get alerts when it matters", "Allow alerts", "Critical
	// reports near your home area") are step 5's own words now and left this
	// list; the obsolete "Share, then Add to Home Screen" ban is gone because
	// the guide's real wording lives in device_guide.html.tmpl.
	for _, banned := range []string{
		"Severe official alerts",
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
		// Amended 2026-10-29 by plan 05-18 (D-03, UI-SPEC 4f): the lead keeps its
		// built first sentence, unchanged, and adds a space and the restored
		// second sentence. The old pin was the paragraph holding the first
		// sentence alone; the first sentence is still pinned on its own below
		// (it is counted once in the page).
		`<p class="welcome__notice-lead" id="welcome-disagree-lead">Sorry, you need to agree before you can continue. You can still look around in the Demo without agreeing.</p>`,
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
	//
	// Amended 2026-10-29 by plan 05-18 (D-03): the box gains one link, Try the
	// demo, so "<a " left the ban list. The ban on a call line is unchanged (no
	// number, no tel: link); the box may hold exactly one anchor and it is the
	// Demo link.
	for _, banned := range []string{"112", "tel:", "CallHref", "Numbers"} {
		if strings.Contains(boxHTML, banned) {
			t.Errorf("the Disagree box holds %q; the owner dropped the call line", banned)
		}
	}
	w10Count(t, "the Disagree box", boxHTML, "<a ", 1)
	w10Count(t, "the Disagree box", boxHTML, `<a class="btn" id="welcome-try-demo-2" href="/demo?from=welcome">Try the demo</a>`, 1)

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
	// Amended 2026-10-29 by Phase 7.1 plan 13: the help line under the home area now
	// reads "A town or area, never your street. Used for alerts." (UI-SPEC Surface
	// 7). The UI-SPEC also starts it with "Filled in from your location.", which
	// stays out on purpose: nothing fills the home area from the device (plan 33),
	// so the sentence would be untrue and the ban above is kept.
	p13Ordered(t, page, "home area field", `id="welcome-home-input"`, "A town or area, never your street. Used for alerts.")

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
	// Amended 2026-10-29 by Phase 7.1 plan 13: the optional lead and the Skip button
	// are gone (step 4 is required, see TestWelcomeStep4Required); the name and home
	// area inputs keep their placeholders and stored values, and Continue stays.
	p13MustContain(t, "step 4", page,
		`placeholder="For example: Ravi Kumar"`,
		`value="{{.DisplayName}}"`, `value="{{.HomeAreaName}}"`,
		`id="welcome-continue-4" aria-disabled="true">Continue</button>`)

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
	//
	// Amended 2026-10-29 by Phase 7.1 plan 13b (D-18): localStorage leaves the
	// banned list, because the install pop-up keeps a list of account ids in the
	// browser. It may be touched in exactly two places, reading and writing the
	// key pinalert.installAsked, and the pins below fail on any other use; a
	// position or a place is still never stored.
	//
	// Amended 2026-10-29 by Phase 7.1 plan 13c (D-18): localStorage is touched in
	// exactly four places now, reading and writing pinalert.installAsked and
	// reading and writing pinalert.alertsAsked (the list the alerts pop-up adds the
	// account id to when it opens, which installed-ask.js on Nearby reads). The
	// pins below fail on any other use.
	//
	// Amended 2026-10-29 by the Phase 7.1 review fix (browser): WR-B06, five places
	// now. The fifth only reads notify.js's own intent flag (its exported
	// INTENT_KEY), so the sign up alerts pop-up does not ask a device whose alerts
	// are already on for this account. It is never written here.
	w10Count(t, "welcome.js", js, "localStorage", 5)
	p13MustContain(t, "welcome.js", js,
		"var INSTALL_ASKED_KEY = 'pinalert.installAsked';",
		"window.localStorage.getItem(INSTALL_ASKED_KEY)",
		"window.localStorage.setItem(INSTALL_ASKED_KEY, JSON.stringify(list.slice(-INSTALL_ASKED_KEEP)))",
		"var ALERTS_ASKED_KEY = 'pinalert.alertsAsked';",
		"window.localStorage.getItem(ALERTS_ASKED_KEY)",
		"window.localStorage.setItem(ALERTS_ASKED_KEY, JSON.stringify(list.slice(-INSTALL_ASKED_KEEP)))",
		"window.localStorage.getItem(notify.INTENT_KEY) === account")
	w10Count(t, "welcome.js", js, "INTENT_KEY", 3)
	p13MustNotContain(t, "welcome.js", js, "setItem(notify.INTENT_KEY", "removeItem(")
	for _, banned := range []string{"reverse", "nominatim", "watchPosition", "&lat=", "&lon="} {
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
	// Amended 2026-10-28 by plan 03.3-19 (03.3-UAT gap for test 4): the alerts
	// pop-up has the same areas-mode search for an account with no home area, so
	// there are two instances, step 4's and the pop-up's. Both use STEP4_SEARCH_URL
	// (the address is pinned once above) and neither reads the device position.
	w10Count(t, "welcome.js", js, "makeSearch({", 2)
	w10Count(t, "welcome.js", js, "url: STEP4_SEARCH_URL", 2)
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
	// Amended 2026-10-29 by Phase 7.1 plan 13: finish takes no argument (there is no
	// Skip), and it checks the three fields after the agreement and before it sends.
	finish := jsFunctionBody(t, js, "finishStep4()")
	p13Ordered(t, finish, "finish", "if (!agreed) {", "goTo(CONSENT_STEP);", "showLine(consentError, MESSAGE_AGREE_FIRST);", "if (!allGood()) {", "setFinishing(true);")
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

	// The theme script is in the head, the scripts load deferred in order.
	// Amended 2026-10-29 by Phase 7.1 plan 13: username-check.js loads between
	// position.js and welcome.js, so PinalertUsernameCheck exists when welcome.js runs.
	p13Ordered(t, page, "scripts", `/static/js/theme.js?v={{.AssetVersion}}"></script>`, "</head>",
		`/static/js/position.js?v={{.AssetVersion}}" defer></script>`, `/static/js/username-check.js?v={{.AssetVersion}}" defer></script>`, `/static/js/welcome.js?v={{.AssetVersion}}" defer></script>`)
	// Amended 2026-10-27 by quick 261027-t5w: auth.css comes second, as in
	// settings.html.tmpl, because the alerts pop-up reuses the Log out box rules.
	p13Ordered(t, page, "stylesheets", "/static/css/main.css", "/static/css/auth.css", "/static/css/chrome.css", "/static/css/app-bar.css", "/static/css/legal.css", "/static/css/window.css", "/static/css/welcome.css")

	for _, name := range []string{"js/welcome.js", "js/position.js", "js/username-check.js"} {
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
		"js/username-check.js":        p421ReadStatic(t, "js/username-check.js"),
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
// plan 13), as amended by plan 03.3-19 (03.3-UAT gap for test 4, owner
// 2026-10-27): a native modal dialog that is a direct child of body after the
// flow root, with the Log out box markup, a title that names the home area, the
// home area search (hidden when an area is stored), the unticked Reliable
// reporter tick box, Not now then Allow, the key on the flow root, and not a
// trace of the removed step 5 page.
func TestWelcomeAlertsAsk(t *testing.T) {
	page := w10Template(t)

	const open = `<dialog id="welcome-alerts-ask" class="logout-confirm" aria-labelledby="welcome-alerts-ask-title" data-home-area="{{.HomeAreaName}}">`
	// Amended 2026-10-28 by plan 03.3-19: the pin on the number of dialog tags is
	// gone, because Phase 7.1 plan 13b adds a second dialog to this page. This
	// pop-up is counted by its id.
	w10Count(t, "welcome.html.tmpl", page, `id="welcome-alerts-ask"`, 1)
	w10Count(t, "welcome.html.tmpl", page, open, 1)
	// A direct child of body: it comes right after the closing of the #welcome
	// root (the last step section, the pane and the root) and before the scripts,
	// so the welcome Tab trap never sees its keys and the native modal can make
	// #welcome inert.
	p13MustMatch(t, "alerts pop-up position", page, `(?s)</section>\s*</div>\s*</div>\s*`+regexp.QuoteMeta(open))
	// Amended 2026-10-29 by Phase 7.1 plan 13b: the install pop-up sits between the
	// alerts pop-up and the scripts, so the alerts pop-up is followed by it and the
	// install pop-up by the scripts (pinned in TestWelcomeInstallAsk).
	p13MustMatch(t, "alerts pop-up position", page, `(?s)</dialog>\s*<dialog id="welcome-install-ask"`)

	ask := regexp.MustCompile(`(?s)<dialog id="welcome-alerts-ask".*?</dialog>`).FindString(page)
	if ask == "" {
		t.Fatalf("no alerts pop-up in welcome.html.tmpl")
	}
	// The title names the home area: the words, the stored name or your home
	// area, and a question mark. No title ends in near you with a question mark.
	const titleHead = `<h2 class="logout-confirm__title" id="welcome-alerts-ask-title" tabindex="-1">`
	const titleWords = "Get alerts for critical reports near "
	p13MustContain(t, "alerts pop-up", ask,
		`<div class="logout-confirm__body">`,
		titleHead+titleWords+`{{if .HomeAreaName}}{{.HomeAreaName}}{{else}}your home area{{end}}?</h2>`,
		`<div class="logout-confirm__actions">`,
		`<button type="button" class="btn" id="welcome-alerts-later">Not now</button>`,
		`<button type="button" class="btn btn--primary" id="welcome-alerts-allow">Allow</button>`)
	w10Count(t, "welcome.html.tmpl", page, "near you?", 0)

	// The search block: the step 4 search markup, hidden when an area is stored.
	p13MustContain(t, "alerts pop-up", ask,
		`<div class="welcome-ask__home" id="welcome-alerts-home"{{if .HomeAreaName}} hidden{{end}}>`,
		`<form id="welcome-alerts-home-form" novalidate>`,
		`<label class="app-window__label" for="welcome-alerts-home-input">Home area</label>`,
		`type="search" id="welcome-alerts-home-input"`,
		`<p class="app-window__status" id="welcome-alerts-home-status" role="status" aria-live="polite" hidden></p>`,
		`<ul class="app-window__results" id="welcome-alerts-home-results" role="listbox" aria-label="Places found" hidden></ul>`)

	// The tick box: unticked, labelled with the Settings row words.
	box := regexp.MustCompile(`<input[^>]*id="welcome-alerts-reliable"[^>]*>`).FindString(ask)
	if box == "" {
		t.Fatalf("no Reliable tick box in the alerts pop-up")
	}
	p13MustContain(t, "tick box", box, `type="checkbox"`)
	p13MustNotContain(t, "tick box", box, "checked")
	p13MustContain(t, "alerts pop-up", ask,
		`<label class="welcome-ask__check"><input type="checkbox" id="welcome-alerts-reliable"><span>A Reliable reporter posts near me</span></label>`)

	// Amended by the 03.3-REVIEW fix WR-05: one hidden error line, in the look of
	// the step 4 error line, says when the home area picked here was not saved.
	// It sits after the tick box and before the buttons.
	p13MustContain(t, "alerts pop-up", ask,
		`<p class="inline-error" id="welcome-alerts-ask-error" role="alert" hidden></p>`)
	p13Ordered(t, ask, "alerts pop-up", `id="welcome-alerts-ask-title"`, `id="welcome-alerts-home"`, `id="welcome-alerts-reliable"`, `id="welcome-alerts-ask-error"`, `class="logout-confirm__actions"`, `id="welcome-alerts-later"`, `id="welcome-alerts-allow"`)
	// Nothing is chosen on open, there is no supporting line (the paragraphs are
	// the search status line and the hidden error line), and no link.
	p13MustNotContain(t, "alerts pop-up", ask, "autofocus", "<a ")
	w10Count(t, "alerts pop-up", ask, "<p", 2)
	w10Count(t, "alerts pop-up", ask, "<button", 2)
	w10Count(t, "alerts pop-up", ask, "<input", 2)
	for _, id := range []string{"welcome-alerts-ask-title", "welcome-alerts-later", "welcome-alerts-allow", "welcome-alerts-home", "welcome-alerts-home-form", "welcome-alerts-home-input", "welcome-alerts-home-status", "welcome-alerts-home-results", "welcome-alerts-reliable", "welcome-alerts-ask-error"} {
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
	// Amended 2026-10-29 by Phase 7.1 plan 13b (D-17): the install guide used to
	// live only in Settings. The partial and the stylesheet now appear once each,
	// for the install pop-up (TestWelcomeInstallAsk). The alerts pop-up itself holds
	// none of it, "Home Screen" may appear only inside #welcome-install-ask, and the
	// Settings window title "Install to get alerts" still never appears here.
	w10Count(t, "welcome.html.tmpl", page, `{{template "device_guide"}}`, 1)
	w10Count(t, "welcome.html.tmpl", page, "/static/css/device-guide.css?v={{.AssetVersion}}", 1)
	p13MustNotContain(t, "alerts pop-up", ask, "Home Screen", "device_guide")
	outsideInstall := regexp.MustCompile(`(?s)<dialog id="welcome-install-ask".*?</dialog>`).ReplaceAllString(page, "")
	p13MustNotContain(t, "welcome.html.tmpl outside the install pop-up", outsideInstall, "Home Screen", "device_guide")
	p13MustNotContain(t, "welcome.html.tmpl", page, "Install to get alerts")

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
	// auth.css carries the look of the box (plan 03.3-19 adds only layout rules
	// for the search block and the tick box, scoped by id), so welcome.css never
	// names the Log out box class.
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

	// Plan 03.3-19 (03.3-UAT gap for test 4, owner 2026-10-27): the pop-up saves a
	// home area and the Reliable choice after enable. The alerts address and the
	// Reliable field are each written once in the whole file (comments included, so
	// no comment spells them), the one Reliable request is saveReliable, and the
	// old changes_push field is not used. A Mac Dock app never gets the pop-up.
	w10Count(t, "welcome.js", raw, "/api/me/alerts", 1)
	w10Count(t, "welcome.js", raw, "reliable_alerts", 1)
	w10Count(t, "welcome.js", raw, "function saveReliable(", 1)
	p13MustNotContain(t, "welcome.js", js, "changes_push")
	p13MustContain(t, "canAsk", jsFunctionBody(t, js, "canAsk()"), "isMacDockApp")
	// The title words in the script are the words before the home area in the
	// template, so the two cannot drift apart.
	words := regexp.MustCompile(`var ASK_TITLE_START = '([^']*)';`).FindStringSubmatch(js)
	if words == nil {
		t.Fatalf("welcome.js has no ASK_TITLE_START constant")
	}
	titleLine := regexp.MustCompile(`id="welcome-alerts-ask-title" tabindex="-1">([^{]*)\{\{if \.HomeAreaName\}\}`).FindStringSubmatch(w10Template(t))
	if titleLine == nil {
		t.Fatalf("welcome.html.tmpl has no title with the home area action")
	}
	if words[1] != titleLine[1] {
		t.Errorf("ASK_TITLE_START is %q, but the template title starts with %q", words[1], titleLine[1])
	}

	// The Allow handler: busy check, then enable as the first real statement,
	// then the busy flags. Nothing is awaited before enable.
	allow := jsFunctionBody(t, js, "onAskAllow()")
	p13Ordered(t, allow, "onAskAllow", "if (askBusy) {", "window.PinalertNotify.enable(pushKey)", "askBusy = true;", ".then(", "saveReliable(", "postDone()")
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
	// Amended 2026-10-29 by Phase 7.1 plan 13: finish(save) is finish().
	p13MustNotContain(t, "finish", jsFunctionBody(t, js, "finishStep4()"), "postDone(")
	p13MustContain(t, "finish", jsFunctionBody(t, js, "finishStep4()"), "endFlow(")
	post := jsFunctionBody(t, js, "postDone()")
	p13MustContain(t, "postDone", post, "answer.status === 409", "goTo(2);", "MESSAGE_AGREE_FIRST", "doneFailed();", "profileError")

	// The needs-install title and every step 5 helper are gone.
	p13MustNotContain(t, "welcome.js", js,
		"'Alerts'", "TITLE_LATER", "ALERTS_STEP", "alertsStep", "alertsSeq", "setAlertsBusy", "drawAlerts", "enterAlerts",
		"toAlerts", "alertsDone", "welcome-step-5", "welcome-alerts-status", "welcome-alerts-nohome", "hasHomeArea", "doneErrorLine",
		"'Install to get alerts'", "MESSAGE_ALERTS_")
}

// TestWelcomeInstallAsk pins Phase 7.1 plan 13b (D-17, D-18, UI-SPEC Surface
// 10a): the install pop-up that ends sign up on an iPhone or iPad Safari tab. It
// is a native modal dialog after the alerts pop-up, in the Log out box look, with
// a title that names the home area and no question mark, the install guide
// partial once, and one button, Not now. There is no Allow in it, the stylesheet
// of the guide is linked, and welcome.js keeps it away from the browser's
// notification prompt, shows it once per account through the list kept under
// pinalert.installAsked, and records the flow as done from Not now, Escape and a
// dialog that will not open.
func TestWelcomeInstallAsk(t *testing.T) {
	page := w10Template(t)

	const open = `<dialog id="welcome-install-ask" class="logout-confirm" aria-labelledby="welcome-install-ask-title" data-home-area="{{.HomeAreaName}}">`
	w10Count(t, "welcome.html.tmpl", page, `id="welcome-install-ask"`, 1)
	w10Count(t, "welcome.html.tmpl", page, open, 1)
	w10Count(t, "welcome.html.tmpl", page, "<dialog", 2)
	// After the alerts pop-up, outside the flow root, before the scripts.
	p13MustMatch(t, "install pop-up position", page, `(?s)</dialog>\s*`+regexp.QuoteMeta(open))
	p13MustMatch(t, "install pop-up position", page, `(?s)</dialog>\s*<script src="/static/js/position\.js`)

	ask := regexp.MustCompile(`(?s)<dialog id="welcome-install-ask".*?</dialog>`).FindString(page)
	if ask == "" {
		t.Fatalf("no install pop-up in welcome.html.tmpl")
	}
	const titleWords = "Get alerts for critical reports near "
	p13MustContain(t, "install pop-up", ask,
		`<div class="logout-confirm__body">`,
		`<h2 class="logout-confirm__title" id="welcome-install-ask-title" tabindex="-1">`+titleWords+`{{if .HomeAreaName}}{{.HomeAreaName}}{{else}}your home area{{end}}</h2>`,
		`<div class="logout-confirm__actions">`,
		`<button type="button" class="btn" id="welcome-install-later">Not now</button>`)
	// The box asks nothing, so the title has no question mark.
	p13MustNotContain(t, "install pop-up", ask, "?</h2>")
	// The guide partial exactly once, between the title and the button, and the
	// one button is Not now: no Allow, no primary button, no link, no input.
	w10Count(t, "install pop-up", ask, `{{template "device_guide"}}`, 1)
	p13Ordered(t, ask, "install pop-up", `id="welcome-install-ask-title"`, `{{template "device_guide"}}`, `class="logout-confirm__actions"`, `id="welcome-install-later"`)
	w10Count(t, "install pop-up", ask, "<button", 1)
	p13MustNotContain(t, "install pop-up", ask, "Allow", "btn--primary", "autofocus", "<a ", "<input")
	// The stylesheet of the guide is linked in the head, versioned like the others.
	w10Count(t, "welcome.html.tmpl", page, `<link rel="stylesheet" href="/static/css/device-guide.css?v={{.AssetVersion}}">`, 1)
	p13MustNotContain(t, "install pop-up", ask, "near you")

	// The script.
	raw := p421ReadStatic(t, "js/welcome.js")
	js := stripJSComments(raw)
	words := regexp.MustCompile(`var ASK_TITLE_START = '([^']*)';`).FindStringSubmatch(js)
	if words == nil || words[1] != titleWords {
		t.Errorf("ASK_TITLE_START is %v, want %q (the install pop-up title starts with it)", words, titleWords)
	}
	w10Count(t, "welcome.js", js, "'welcome-install-ask'", 1)
	w10Count(t, "welcome.js", js, "'welcome-install-ask-title'", 1)
	w10Count(t, "welcome.js", js, "'welcome-install-later'", 1)
	p13MustContain(t, "welcome.js", js,
		"var INSTALL_ASKED_KEY = 'pinalert.installAsked';",
		"var INSTALL_ASKED_KEEP = 8;",
		"installLater.addEventListener('click', onInstallLater);",
		"installDialog.addEventListener('cancel', onInstallCancel);")

	// Never the browser's prompt, never a turn-on, from any part of the install path.
	for _, name := range []string{"canInstallAsk()", "openInstallAsk()", "onInstallLater()", "onInstallCancel(e)", "installAskedList()", "rememberInstallAsk()", "closeInstall()"} {
		body := jsFunctionBody(t, js, name)
		p13MustNotContain(t, name, body, "enable(", "requestPermission", "Notification.", "innerHTML", "insertAdjacentHTML")
	}
	// It opens only for needs-install, so it never opens beside the alerts pop-up
	// (which needs maybe or ready).
	can := jsFunctionBody(t, js, "canInstallAsk()")
	p13MustContain(t, "canInstallAsk", can, "installDialog", "showModal", "baseState()", "'needs-install'", "installShown", "installAskedList()")
	p13MustMatch(t, "canInstallAsk", can, `(?s)try \{.*baseState\(\).*\} catch`)
	p13MustNotContain(t, "canInstallAsk", can, "'maybe'", "'ready'")
	// endFlow tries the alerts pop-up, then the install pop-up, then the done record.
	end := jsFunctionBody(t, js, "endFlow()")
	p13Ordered(t, end, "endFlow", "canAsk()", "openAsk()", "canInstallAsk()", "openInstallAsk()", "postDone()")
	// The id is written just before showModal, inside a try, and the box shows the
	// home area through textContent only.
	openFn := jsFunctionBody(t, js, "openInstallAsk()")
	p13Ordered(t, openFn, "openInstallAsk", "setText(installTitle", "rememberInstallAsk()", "installDialog.showModal()", "postDone()", "installTitle.focus()")
	remember := jsFunctionBody(t, js, "rememberInstallAsk()")
	p13Ordered(t, remember, "rememberInstallAsk", "try {", "installAskedList()", "setItem(INSTALL_ASKED_KEY", "} catch")
	// Not now closes the box and then records the flow as done, once; Escape is the
	// cancel event and does the same, never a close event.
	later := jsFunctionBody(t, js, "onInstallLater()")
	p13Ordered(t, later, "onInstallLater", "if (installBusy) {", "installBusy = true;", "closeInstall()", "postDone()")
	p13MustContain(t, "onInstallCancel", jsFunctionBody(t, js, "onInstallCancel(e)"), "e.preventDefault();", "onInstallLater()")
	// A failed record puts the box away and frees it, so the next Continue works.
	p13MustContain(t, "doneFailed", jsFunctionBody(t, js, "doneFailed()"), "closeInstall()", "installBusy = false;")
	// The list holds the last 8 ids, each a plain account id string.
	p13MustContain(t, "installAskedList", jsFunctionBody(t, js, "installAskedList()"), "ACCOUNT_ID_PATTERN", "Array.isArray(list)", "JSON.parse(raw)")
	p13MustContain(t, "rememberInstallAsk", remember, "slice(-INSTALL_ASKED_KEEP)")
	// The flow is still recorded from one sender.
	w10Count(t, "welcome.js", js, "request(DONE_URL", 1)
}

// TestWelcomeStep4Required pins Phase 7.1 plan 13 (D-06, D-08, UI-SPEC Surface 7):
// step 4 About you asks for a Username, a Name and a Home area, all three
// required. There is no Skip and no optional lead, the lead follows the account
// that is filling in, Continue is the only button and starts switched off with
// aria-disabled (the shared btn--off look), the alerts pop-up and the "of 4"
// count are as built, and the script keeps a 409 with field profile on step 4.
func TestWelcomeStep4Required(t *testing.T) {
	page := w10Template(t)
	step4 := regexp.MustCompile(`(?s)<section class="welcome__step" id="welcome-step-4".*?</section>`).FindString(page)
	if step4 == "" {
		t.Fatalf("no step 4 section in welcome.html.tmpl")
	}

	// Nothing optional is left: no Skip button, no id, no optional lead.
	p13MustNotContain(t, "step 4", step4, "welcome-skip", ">Skip<", "Both are optional", "They show only if you later let others follow you")
	p13MustNotContain(t, "welcome.html.tmpl", page, "welcome-skip", ">Skip<", "Both are optional")

	// The two leads, switched on the account that is filling in.
	p13Ordered(t, step4, "step 4 lead", `<p class="welcome__lead">{{if .FillingIn}}`,
		"Pinalert now asks everyone for a username, a name and a home area. Fill in what is missing.",
		"{{else}}", "All three are needed to finish signing up. You can change them later in Settings.", "{{end}}</p>")

	// The three fields in order, with their labels and their small lines.
	p13Ordered(t, step4, "step 4 fields",
		`<label class="app-window__label" for="welcome-username-input">Username</label>`,
		`id="welcome-username-status"`,
		`id="welcome-username-help">Shown on your comments, to everyone.</p>`,
		`<label class="app-window__label" for="welcome-name-input">Name</label>`,
		`id="welcome-name-help">Shown only if you let others follow you.</p>`,
		`<label class="app-window__label" for="welcome-home-input">Home area</label>`,
		`id="welcome-home-help">A town or area, never your street. Used for alerts.</p>`,
		`id="welcome-error-4"`, `id="welcome-continue-4"`)
	p13MustNotContain(t, "step 4", step4, ">Your name<", "Used to send you alerts about this area.")
	p13MustContain(t, "step 4", step4,
		`placeholder="For example: ravi.k"`, `value="{{.Username}}"`,
		`placeholder="For example: Ravi Kumar"`, `maxlength="60"`, `autocomplete="name"`,
		`id="welcome-name-help"`, `aria-describedby="welcome-name-help"`)

	// The live line: a status that announces politely, empty until the script writes it.
	p13MustContain(t, "step 4", step4,
		`<p class="username-status" id="welcome-username-status" role="status" aria-live="polite"></p>`)

	// Continue is the only button of the step's actions, switched off by aria-disabled
	// and never by the disabled attribute, in the shared full contrast look.
	actions := regexp.MustCompile(`(?s)<div class="welcome__actions">.*?</div>`).FindString(step4)
	if actions == "" {
		t.Fatalf("no actions row in step 4")
	}
	w10Count(t, "step 4 actions", actions, "<button", 1)
	p13MustContain(t, "step 4 actions", actions,
		`<button type="button" class="btn btn--off" id="welcome-continue-4" aria-disabled="true">Continue</button>`)
	p13MustNotContain(t, "step 4 actions", actions, "disabled>", " disabled ", "btn--primary", "btn--destructive")
	w10Count(t, "step 4", step4, `class="btn`, 1)

	// The server names the step Agree leads to.
	p13MustContain(t, "flow root", page, `data-after-terms-step="{{.AfterTermsStep}}"`)

	// The end of the flow is as built: four steps counted of 4, the alerts pop-up
	// with Not now and Allow, and no ask for a home area after step 4.
	w10Count(t, "welcome.html.tmpl", page, ` of 4</p>`, 4)
	p13MustNotContain(t, "welcome.html.tmpl", page, " of 5", "Step 5", "welcome-alerts-nohome")
	p13MustContain(t, "alerts pop-up", page,
		`<dialog id="welcome-alerts-ask"`, `id="welcome-alerts-later">Not now</button>`, `id="welcome-alerts-allow">Allow</button>`)

	// The script: no Skip, the three checks, the off look, the guide line and the
	// 409 profile branch that returns to step 4.
	js := stripJSComments(p421ReadStatic(t, "js/welcome.js"))
	p13MustNotContain(t, "welcome.js", js, "welcome-skip", "skipButton")
	p13MustContain(t, "welcome.js", js,
		"var MESSAGE_ADD_ALL = 'Add your username, name and home area.';",
		"function updateContinue()", "continueButton.setAttribute('aria-disabled', 'true');",
		"continueButton.removeAttribute('aria-disabled');", "'btn--off'", "'btn--primary'",
		"function focusFirstMissing()", "PinalertUsernameCheck.attach(", "mode: 'signup'",
		"root.getAttribute('data-after-terms-step')", "goTo(afterTermsStep);")
	p13MustNotContain(t, "welcome.js", js, ".disabled = !", "continueButton.setAttribute('disabled'")
	post := jsFunctionBody(t, js, "postDone()")
	p13Ordered(t, post, "postDone", "doneFailed();", "answer.status === 409 && answer.body", "answer.body.error.field === 'profile'", "goTo(4);", "showLine(profileError", "goTo(2);")
	body := jsFunctionBody(t, js, "profileBody()")
	p13MustContain(t, "profileBody", body, "body.username = username;", "storedUsername")

	// The shared look: the box, the @ and the status line, each display rule
	// guarded for hidden, no !important and the tone classes by name.
	css := stripCSSComments(p421ReadStatic(t, "css/chrome.css"))
	p13MustContain(t, "chrome.css", css,
		".username-field {", ".username-field:focus-within {", ".username-field__at {", ".username-field__input {",
		".username-status:not([hidden]) {", ".username-status--good {", ".username-status--bad {",
		"min-height: var(--touch-target-min);", "color: var(--color-tone-good);", "color: var(--color-tone-bad);")
	block := css[strings.Index(css, ".username-field {"):]
	if strings.Contains(block, "!important") {
		t.Errorf("the username rules in chrome.css use !important")
	}
}

// TestWelcomeUsernameNotLoginField pins T-07.1-30: a password manager must never
// save the Username as the login. The input is autocomplete off (never username),
// is not named username, switches off capitals, correction and spell check, holds
// a plain @ in front of it inside one box, and the page has no password field.
func TestWelcomeUsernameNotLoginField(t *testing.T) {
	page := w10Template(t)
	w10Count(t, "welcome.html.tmpl", page, `id="welcome-username-input"`, 1)
	input := regexp.MustCompile(`<input[^>]*id="welcome-username-input"[^>]*>`).FindString(page)
	if input == "" {
		t.Fatalf("no Username input in welcome.html.tmpl")
	}
	p13MustContain(t, "username input", input,
		`autocomplete="off"`, `autocapitalize="none"`, `autocorrect="off"`, `spellcheck="false"`,
		`inputmode="text"`, `enterkeyhint="next"`, `maxlength="20"`, `type="text"`, `name="handle"`)
	p13MustNotContain(t, "username input", input, `autocomplete="username"`, `name="username"`, `type="email"`, `type="password"`)
	p13MustNotContain(t, "welcome.html.tmpl", page, `autocomplete="username"`, `type="password"`, `autocomplete="current-password"`, `autocomplete="new-password"`)
	// The plain @ is decoration inside the same box, ahead of the input.
	p13MustMatch(t, "username box", page, `(?s)<div class="username-field">\s*<span class="username-field__at" aria-hidden="true">@</span>\s*`+regexp.QuoteMeta(input)+`\s*</div>`)
}

// TestWelcomeNoTaglineAdded pins the owner rules for this plan: no tagline, no
// hint and no sentence is added to the laptop step panel. The panel keeps the
// elements it had before plan 13, in order, and the one tagline that was already
// there.
func TestWelcomeNoTaglineAdded(t *testing.T) {
	page := w10Template(t)
	w10Count(t, "welcome.html.tmpl", page, "welcome__tagline", 1)
	w10Count(t, "welcome.html.tmpl", page, "welcome__wordmark", 1)

	side := regexp.MustCompile(`(?s)<aside class="welcome__side">.*?</aside>`).FindString(page)
	if side == "" {
		t.Fatalf("no laptop step panel in welcome.html.tmpl")
	}
	var tags []string
	for _, m := range regexp.MustCompile(`<([a-z][a-z0-9]*)`).FindAllStringSubmatch(side, -1) {
		tags = append(tags, m[1])
	}
	// The list recorded before this plan: the panel, the wordmark, the tagline and
	// the step list (one item per step: the button, the dot with its number and its
	// tick, and the name).
	want := []string{"aside", "p", "p", "ol", "li", "button", "span", "span", "span", "span"}
	if strings.Join(tags, " ") != strings.Join(want, " ") {
		t.Errorf("the laptop step panel holds the elements %v, want exactly %v", tags, want)
	}
	p13MustContain(t, "laptop step panel", side,
		`<p class="welcome__wordmark">Pinalert</p>`,
		`<p class="welcome__tagline">Live emergency updates from people nearby, confirmed by others who are there.</p>`)
}
