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

// TestWelcomeFourStepsInOrder pins the four step titles and their order, the
// "Step n of 4" line above each, and the absence of the Alerts step, which
// waits for Phase 3.3 (D-28).
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
	if strings.Contains(page, "Step 5") || strings.Contains(page, "of 5") {
		t.Errorf("the flow has four steps in Phase 3.4, a fifth is Phase 3.3's")
	}

	// The laptop step list is drawn from the steps the handler passes.
	p13MustContain(t, "step list", page, `<ol class="welcome__steps"`, `{{range .Steps}}`, `data-step="{{.Number}}"`)

	// The Alerts step and its sentences are not rendered, and neither is the
	// second Disagree sentence that Phase 5 adds with the Demo.
	for _, banned := range []string{
		"Get alerts when it matters", "Allow alerts", "Critical reports near your home area",
		"Severe official alerts", "Share, then Add to Home Screen", "Demo", "look around", "looking around",
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
	p13Ordered(t, page, "step 2", `{{template "legal_disclaimer_body" .Legal}}`, `{{template "legal_policy_body" .Legal}}`, `id="welcome-disagree"`, `id="welcome-agree"`)
	p13MustContain(t, "step 2", page, "Please read this before you use Pinalert. Tap Agree to continue.")

	// Two buttons in one pair row, Disagree outlined and Agree filled.
	p13MustContain(t, "step 2 buttons", page,
		`<div class="welcome__actions welcome__actions--pair">`,
		`<button type="button" class="btn" id="welcome-disagree">Disagree</button>`,
		`<button type="button" class="btn btn--primary" id="welcome-agree">Agree</button>`)

	// Amended 2026-10-17 by Phase 3.4 plan 31 (the owner asked what should happen
	// on Disagree and the suggestion was built): Disagree shows a calm box, drawn
	// by the template and hidden until then, so the words have one source and
	// welcome.js only shows and hides it. Owner amendment the same day: the first
	// wording was too wordy, so the box is two short lines, You need to agree to
	// use Pinalert. and In danger? Call 112. (a call link built from the same
	// settings the Disclaimer uses, never a typed number), and
	// holds two buttons: Read again and Log out (the app's existing log out,
	// a real POST form to /auth/logout as in the person menu, ending at the sign
	// in page). It is a group named by its first sentence, not a status region,
	// because it holds buttons and takes focus.
	p13MustContain(t, "disagree box", page,
		`<div class="welcome__notice welcome__notice--box" id="welcome-disagree-message" role="group" aria-labelledby="welcome-disagree-lead" tabindex="-1" hidden>`,
		`<p class="welcome__notice-lead" id="welcome-disagree-lead">You need to agree to use Pinalert.</p>`,
		`<p class="welcome__notice-danger">In danger? Call <a class="welcome__notice-call" href="{{.Legal.CallHref}}" aria-label="Call {{.Legal.Numbers.Primary}}">{{.Legal.Numbers.Primary}}</a>.</p>`,
		`<button type="button" class="btn" id="welcome-reread">Read again</button>`,
		`<form class="welcome__notice-form" method="post" action="/auth/logout">`,
		`<button type="submit" class="btn" id="welcome-logout">Log out</button>`)
	w10Count(t, "welcome.html.tmpl", page, `action="/auth/logout"`, 1)
	w10Count(t, "welcome.html.tmpl", page, `id="welcome-disagree-message"`, 1)
	w10Count(t, "welcome.html.tmpl", page, "You need to agree to use Pinalert.", 1)
	p13MustNotContain(t, "disagree box", page, `role="status" hidden>`, "keep everyone safe", "Read it again")
	// The box sits under the buttons it answers, and its own buttons come in
	// the order Read again, Log out.
	p13Ordered(t, page, "step 2", `id="welcome-agree"`, `id="welcome-disagree-message"`, `id="welcome-reread"`, `id="welcome-logout"`, `id="welcome-error-2"`)
	if strings.Contains(strings.NewReplacer("{{.Legal.Numbers.Primary}}", "", "{{.Legal.CallHref}}", "").Replace(page[strings.Index(page, `id="welcome-disagree-message"`):strings.Index(page, `id="welcome-error-2"`)]), "112") {
		t.Errorf("the Disagree box types the emergency number; it comes from the numbers setting")
	}

	js := stripJSComments(p421ReadStatic(t, "js/welcome.js"))
	if strings.Contains(js, "You need to agree") || strings.Contains(js, "DISAGREE_MESSAGE") {
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

// TestWelcomeDisagreeBoxStyle pins the look of the Disagree box (plan 31): the
// existing .welcome__notice family, tokens only, square corners like the other
// buttons (never a pill), no dark outline, the two buttons side by side on a
// laptop and stacked full width on the phone at 48px, and the call link a
// 44px target.
func TestWelcomeDisagreeBoxStyle(t *testing.T) {
	css := stripCSSComments(p421ReadStatic(t, "css/welcome.css"))
	rules := parseCSSRules(css)
	for _, sel := range []string{".welcome__notice--box:not([hidden])", ".welcome__notice-lead", ".welcome__notice-danger", ".welcome__notice-actions", ".welcome__notice-call"} {
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
	if r, ok := ruleBySelector(rules, ".welcome__notice-call"); ok {
		if d := declsOf(r.declBody); d["min-height"] != "44px" && d["min-height"] != "var(--touch-target-min)" {
			t.Errorf("the call link target is %q, want 44px", d["min-height"])
		}
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

// TestWelcomeStep3And4Privacy pins the location and home area rules: the line
// is hidden by default and present once, the step 3 search block is hidden by
// default, there is no reverse lookup, the device position is asked in one
// place and sent nowhere, and the home area is only ever a picked result
// (D-24b, research open question 3).
func TestWelcomeStep3And4Privacy(t *testing.T) {
	page := w10Template(t)

	w10Count(t, "welcome.html.tmpl", page, "Filled in from your location.", 1)
	w10Count(t, "welcome.html.tmpl", page, `id="welcome-home-prefilled"`, 1)
	p13MustMatch(t, "prefilled line", page, `<p class="app-window__help" id="welcome-home-prefilled" hidden>Filled in from your location\.</p>`)
	// Amended 2026-10-17 by Phase 3.4 plan 28: the help line says what the home
	// area is for in the owner's words.
	p13Ordered(t, page, "home area field", `id="welcome-home-prefilled"`, "Used to send you alerts about this area.")

	// Amended 2026-10-17 by Phase 3.4 plan 28: the lone pin tile and the lead
	// sentence became three cards; the privacy fact (never shown, never stored)
	// is the third card.
	p13MustContain(t, "step 3", page,
		"See the reports around you.", "The map and the Nearby list start where you are.",
		"Post from where you are.", "When you post a report, the pin is placed for you.",
		"Your exact location stays private.", "It is never shown to anyone and never stored.",
		`<div class="welcome__actions welcome__actions--under">`,
		`<button type="button" class="btn btn--primary" id="welcome-allow-location">Allow location</button>`,
		`<button type="button" class="btn" id="welcome-search-open">Type my location instead</button>`,
		`<div class="welcome__search" id="welcome-search-block" hidden>`,
		`id="welcome-continue-3"`)
	// Amended 2026-10-17 by Phase 3.4 plan 31 (the owner said "Search a place
	// instead" does not make sense): the second button reads Type my location
	// instead; the search it opens is labelled Type your location and shows the
	// example For example: Koramangala, Bengaluru, as step 4's field does.
	p13MustContain(t, "step 3 search", page,
		`<label class="app-window__label" for="welcome-search-input">Type your location</label>`,
		`placeholder="For example: Koramangala, Bengaluru"`)
	p13MustNotContain(t, "step 3", page, "welcome__tile", "Not now, I will search a place", "Search a place")
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
		"'/api/geocode?q='", "'/api/geocode/place?kind=area&q='",
		"'/api/me/consent'", "'/api/me/profile'", "'/api/me/first-run-done'", "'PATCH'", "'POST'")

	// Plan 28: step 4 is prefilled with the area around the step 3 pick, never
	// with the pick's own name (a street or a building).
	enter := jsFunctionBody(t, js, "enterStep4()")
	p13MustContain(t, "enterStep4", enter, "step3Pick.area", "homeInput.value = step3Pick.area;")
	p13MustNotContain(t, "enterStep4", enter, "step3Pick.name")

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
	for n := 2; n <= 4; n++ {
		d := string(rune('0' + n))
		p13MustContain(t, "step "+d, page,
			`<button type="button" class="welcome__back" id="welcome-back-`+d+`"><span class="app-icon app-icon--arrow-left" aria-hidden="true"></span><span class="welcome__back-label">Back</span></button>`)
		p13Ordered(t, page, "step "+d+" head", `id="welcome-back-`+d+`"`, `<p class="welcome__label">Step `+d+` of 4</p>`, `id="welcome-step-`+d+`-title"`)
	}
	p13MustNotContain(t, "step 1", page, `id="welcome-back-1"`)
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
	p13Ordered(t, page, "stylesheets", "/static/css/main.css", "/static/css/chrome.css", "/static/css/app-bar.css", "/static/css/legal.css", "/static/css/window.css", "/static/css/welcome.css")

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
	for _, head := range []string{".welcome__step:not([hidden])", ".welcome__search:not([hidden])", ".welcome__notice:not([hidden])"} {
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
