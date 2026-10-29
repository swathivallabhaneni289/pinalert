package web

// Phase 9 plan 08: the profile page. These tests read the shipped template,
// stylesheet and script from the embedded trees, never a copy, so they can only
// drift with the product (UI-SPEC Surface 1, Surface 6 and the Copywriting
// Contract rows P9-C14 and P9-C15).

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestPersonPageHeaderOrder pins D-13 and UI-SPEC 1b: the header holds the
// round letter, the username as the h1, the Name, the stars, the tag chip and
// its sentence, Member since, the home area, then the Followers and Following
// buttons and the actions, in that order.
func TestPersonPageHeaderOrder(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	p421InOrder(t, "the profile header order", tmpl,
		`id="person-header"`,
		`id="person-disc"`,
		`<h1 id="person-username"`,
		`id="person-name"`,
		`id="person-stars"`,
		`id="person-tag"`,
		`id="person-tag-sentence"`,
		`id="person-member-since"`,
		`id="person-home-area"`,
		`id="person-followers-button"`,
		`id="person-following-button"`,
		`id="person-actions"`,
		`id="person-edit"`,
		`id="person-state-line"`,
		`id="person-follow"`,
		`id="person-unfollow"`,
		`id="person-more"`,
		`id="person-more-menu"`,
		`id="person-action-error"`,
		`id="person-reports-section"`,
		`id="person-blocked"`,
		`id="person-confirm"`,
	)
	if strings.Count(tmpl, "<h1") != 1 {
		t.Errorf("the page has %d h1 elements, want exactly one (the username)", strings.Count(tmpl, "<h1"))
	}
	for _, want := range []string{`class="logout-confirm"`, `id="person-not-found"`, `data-username=`, `data-self=`} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("person.html.tmpl lacks %s", want)
		}
	}
	// The stars are drawn in the text colour and never coloured (UI-SPEC Color).
	css := p421ReadStatic(t, "css/person.css")
	if !strings.Contains(css, `url("/static/icons/star-filled.svg")`) {
		t.Errorf("person.css does not mask star-filled.svg with the literal url")
	}
}

// TestPersonPageNoindexAndFrame pins T-09-31 and UI-SPEC 1a: the noindex meta,
// the two app bars, the page's own stylesheet and script linked with the asset
// version, and the live status line.
func TestPersonPageNoindexAndFrame(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	if strings.Count(tmpl, `name="robots" content="noindex"`) != 1 {
		t.Errorf("person.html.tmpl must carry the noindex meta exactly once")
	}
	for _, want := range []string{
		`{{template "app_topbar" .}}`, `{{template "app_bottombar" .}}`,
		`href="/static/css/person.css?v={{.AssetVersion}}"`,
		`src="/static/js/person.js?v={{.AssetVersion}}"`,
		`id="notice-live"`, `aria-live="polite"`,
		`<title>{{.PageTitle}}</title>`,
	} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("person.html.tmpl lacks %s", want)
		}
	}
	// A profile is not a main place: no bar item is marked current, and the
	// phone bar keeps its gear, so the handler must not use the Settings key.
	raw, err := os.ReadFile("../internal/api/handlers/person_page.go")
	if err != nil {
		t.Fatalf("reading person_page.go: %v", err)
	}
	handler := string(raw)
	if strings.Contains(handler, "NavSettings") {
		t.Errorf("person_page.go uses NavSettings, which hides the gear (UI-SPEC 1a)")
	}
	if !strings.Contains(handler, `"My profile"`) || !strings.Contains(handler, `"Profile"`) {
		t.Errorf("person_page.go lacks the two phone top bar titles")
	}
}

// TestPersonPageCopy pins the Copywriting Contract rows P9-C14 and P9-C15, the
// action words and the section title and line, verbatim, in the template and in
// the script.
func TestPersonPageCopy(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	for _, want := range []string{
		// P9-C14 Not found.
		"Profile not found",
		"There is no public profile with this username. It may have been made private, or the username may have changed.",
		"Go to Nearby",
		// P9-C15 Blocked by you.
		"You blocked ",
		"They can't see your profile or follow you. Their reports still show in the feed.",
		"Unblock",
		// P9-C6, P9-C7 the buttons and the menu.
		"Follow", "Following", "More", "Edit in Settings", "Remove follower", "Block",
		// Surface 5, the part of the section this plan draws.
		"Recent reports", "Reports from the last 30 days, newest first.",
		"stars",
	} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("person.html.tmpl lacks the sentence %q", want)
		}
	}
	script := p421ReadStatic(t, "js/person.js")
	for _, want := range []string{
		// P9-C22 the two confirm boxes.
		"' as a follower?'", "They stop following you. They can follow you again.",
		"They can no longer see your profile or follow you, and you stop following each other.",
		"Their reports still show in the feed. You will not see their comments.",
		// P9-C8 the status lines.
		"'You followed '", "'You unfollowed '", "' no longer follows you.'", "' is unblocked.'",
		// P9-C13 the action errors.
		"Couldn\\'t follow. Try again.", "Couldn\\'t unfollow. Try again.",
		"Couldn\\'t remove this follower. Try again.", "Couldn\\'t block. Try again.",
		"Couldn\\'t unblock. Try again.",
		"You\\'re offline, so this wasn\\'t saved. Try again when you have a signal.",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("person.js lacks the sentence %q", want)
		}
	}
	// The script holds no markup sink (T-09-29).
	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(script, sink) {
			t.Errorf("person.js uses the markup sink %s", sink)
		}
	}
}

// TestPeopleListWindowMarkup pins the shared list window of Followers,
// Following and Blocked people (plan 09-11, UI-SPEC Surface 4): the ids the
// script reads, the X named Close, the words of the error state and the confirm
// dialog built exactly like #settings-delete-confirm.
func TestPeopleListWindowMarkup(t *testing.T) {
	part := p421ReadTemplate(t, "people_list_window.html.tmpl")
	for _, want := range []string{
		`{{define "people_list_window"}}`,
		`<div class="app-window app-window--edit people-list-window" id="people-list-window" hidden>`,
		`role="dialog" aria-modal="true" aria-labelledby="people-list-title"`,
		`<h2 class="app-window__title" id="people-list-title" tabindex="-1"></h2>`,
		`<button type="button" class="app-window__close" id="people-list-close" aria-label="Close">`,
		`id="people-list-subtitle"`,
		`<p class="inline-error" id="people-list-action-error" role="alert" hidden></p>`,
		`<ul class="people-list" id="people-list-rows" hidden></ul>`,
		`<p class="people-list__others" id="people-list-others" hidden></p>`,
		`id="people-list-more" hidden>Show more</button>`,
		`id="people-list-remove-all" hidden>Remove all followers</button>`,
		`id="people-list-empty"`, `id="people-list-empty-title" tabindex="-1"`,
		`id="people-list-error"`,
		"Couldn't load this list", "Check your connection and try again.",
		`id="people-list-retry">Try again</button>`,
	} {
		if !strings.Contains(part, want) {
			t.Errorf("people_list_window.html.tmpl lacks %q", want)
		}
	}
	if strings.Count(part, `class="skeleton-row"`) != 4 {
		t.Errorf("the loading state must hold 4 skeleton rows, found %d", strings.Count(part, `class="skeleton-row"`))
	}
	// The rows and the others line are filled by the script, so the template
	// holds no link in the list region and nothing here can name a person.
	list := part[strings.Index(part, `id="people-list-rows"`):strings.Index(part, `id="people-list-more"`)]
	if strings.Contains(list, "<a ") || strings.Contains(list, "href=") {
		t.Errorf("the list region of the window holds a link: %s", list)
	}

	// The confirm dialog is built like #settings-delete-confirm, a direct child
	// of the body outside the window.
	const open = `<dialog id="people-list-confirm" class="logout-confirm" aria-labelledby="people-list-confirm-title" aria-describedby="people-list-confirm-text">`
	at := strings.Index(part, open)
	if at == -1 {
		t.Fatalf("people_list_window.html.tmpl lacks the exact opening tag %s", open)
	}
	if at < strings.Index(part, `id="people-list-window"`) {
		t.Errorf("the confirm dialog must come after the window")
	}
	end := strings.Index(part[at:], "</dialog>")
	if end == -1 {
		t.Fatalf("the confirm box has no closing dialog tag")
	}
	region := part[at : at+end]
	for _, want := range []string{
		`class="logout-confirm__body"`,
		`<h2 class="logout-confirm__title" id="people-list-confirm-title" tabindex="-1">Remove all followers?</h2>`,
		`<div class="logout-confirm__text" id="people-list-confirm-text">`,
		`<p>Everyone who follows you stops following. Your profile stays on.</p>`,
		`<p class="inline-error" id="people-list-confirm-error" role="alert" hidden></p>`,
		`<div class="logout-confirm__actions">`,
		`<button type="button" class="btn" id="people-list-confirm-cancel">Cancel</button>`,
		`<button type="button" class="btn btn--destructive" id="people-list-confirm-go">Remove all</button>`,
	} {
		if !strings.Contains(region, want) {
			t.Errorf("the confirm box lacks %q", want)
		}
	}
	p421InOrder(t, "confirm box order", region,
		"Remove all followers?", "Everyone who follows you", `id="people-list-confirm-error"`,
		`id="people-list-confirm-cancel"`, `id="people-list-confirm-go"`)
	for _, banned := range []string{`aria-label="Close"`, "app-icon--x", "autofocus", "btn--primary"} {
		if strings.Contains(region, banned) {
			t.Errorf("the confirm box holds %q", banned)
		}
	}

	// The profile page includes the window and loads the script, and the script
	// holds the UI-SPEC words.
	page := p421ReadTemplate(t, "person.html.tmpl")
	for _, want := range []string{`{{template "people_list_window" .}}`, `src="/static/js/people-list.js?v={{.AssetVersion}}"`} {
		if !strings.Contains(page, want) {
			t.Errorf("person.html.tmpl lacks %s", want)
		}
	}
	script := p421ReadStatic(t, "js/people-list.js")
	for _, want := range []string{
		"'Followers'", "'Following'", "'Blocked people'",
		"No followers yet", "People who follow you will show here.", "Nobody follows ",
		"You are not following anyone", "People you follow show here.",
		"Not following anyone", " does not follow anyone yet.",
		"No blocked people", "People you block show here, with a way to unblock them.",
		"' is unblocked.'", "No one follows you now.",
		"Couldn\\'t unblock. Try again.", "Couldn\\'t load this list",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("people-list.js lacks the sentence %q", want)
		}
	}
	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(script, sink) {
			t.Errorf("people-list.js uses the markup sink %s", sink)
		}
	}
}

// TestPersonReportsSection pins the Recent reports part of the profile page
// (plan 09-11, UI-SPEC Surface 5): the column heads, the empty and error
// sentences, the cut line, and that the finished row branch of the template has
// no link and no tabindex, so a profile never leads to a finished report.
func TestPersonReportsSection(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	section := tmpl[strings.Index(tmpl, `id="person-reports-section"`):]
	section = section[:strings.Index(section, "</section>")]
	p421InOrder(t, "the column heads", section,
		`<span>Report</span>`, `<span>Status</span>`, `<span>When</span>`, `<span>Distance</span>`)
	for _, want := range []string{
		"Couldn't load reports", "Check your connection and try again.", `<a class="btn" href="/u/{{.Profile.Username}}">Try again</a>`,
		"No reports in the last 30 days",
		"Reports you post will show here.",
		"Nothing from {{.Profile.Username}} in the last 30 days.",
		`{{if .ReportsCut}}<p class="person-reports__cut">{{.ReportsCutText}}</p>{{end}}`,
		`class="person-report__distance" data-lat="{{.Lat}}" data-lon="{{.Lon}}" data-approx="{{.Approx}}">Unknown</span>`,
		`<a class="person-report__body" href="{{.Href}}">`,
	} {
		if !strings.Contains(section, want) {
			t.Errorf("the reports section lacks %q", want)
		}
	}

	// The finished row branch: from its li to the end of that li.
	const open = `<li class="person-report person-report--finished">`
	at := strings.Index(section, open)
	if at == -1 {
		t.Fatalf("the reports section has no finished row branch")
	}
	finished := section[at : at+strings.Index(section[at:], "</li>")]
	for _, banned := range []string{"<a ", "href", "tabindex", "app-icon--chevron-right", "{{.Href}}", "data-report-id"} {
		if strings.Contains(finished, banned) {
			t.Errorf("the finished row branch holds %q: a finished row is never a link or focusable", banned)
		}
	}
	// The live branch is the only place a link is made.
	if strings.Count(section, `<a class="person-report__body"`) != 1 {
		t.Errorf("exactly one row branch may make a link")
	}

	// The scripts of the page: position first, then the distance filler.
	p421InOrder(t, "the page scripts", tmpl,
		`src="/static/js/position.js?v={{.AssetVersion}}"`,
		`src="/static/js/person-reports.js?v={{.AssetVersion}}"`,
		`src="/static/js/person.js?v={{.AssetVersion}}"`)
	if !strings.Contains(tmpl, `data-account-id="{{.AccountID}}"`) {
		t.Errorf("the page must carry data-account-id for position.js; without it every distance reads Unknown")
	}
	script := p421ReadStatic(t, "js/person-reports.js")
	for _, want := range []string{"PinalertPosition", "'Unknown'", "'About '", "person-report__distance"} {
		if !strings.Contains(script, want) {
			t.Errorf("person-reports.js lacks %q", want)
		}
	}
	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(script, sink) {
			t.Errorf("person-reports.js uses the markup sink %s", sink)
		}
	}
}

// TestPersonChipTintsMatchActivity holds the page local chip tints of
// person.css equal to the My activity values that TestActivityContrast checks
// at 4.5 to 1, so the profile's chips and discs keep that contrast in both
// themes without a second set of numbers.
func TestPersonChipTintsMatchActivity(t *testing.T) {
	person := p421ReadStatic(t, "css/person.css")
	activity := p421ReadStatic(t, "css/activity.css")
	for _, name := range []string{"critical-bg", "critical-ink", "medium-bg", "medium-ink", "low-bg", "low-ink"} {
		pv := regexp.MustCompile(`--p-chip-`+name+`:\s*([^;]+);`).FindAllStringSubmatch(person, -1)
		av := regexp.MustCompile(`--a-chip-`+name+`:\s*([^;]+);`).FindAllStringSubmatch(activity, -1)
		if len(pv) != 4 || len(av) != 4 {
			t.Errorf("%s: want 4 declarations in each file (light, OS dark, dark, light), found %d and %d", name, len(pv), len(av))
			continue
		}
		for i := range pv {
			if strings.TrimSpace(pv[i][1]) != strings.TrimSpace(av[i][1]) {
				t.Errorf("--p-chip-%s declaration %d is %q, My activity has %q", name, i+1, pv[i][1], av[i][1])
			}
		}
	}
}
