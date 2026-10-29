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
		">Unblock</button>",
		// P9-C6, P9-C7 the buttons and the menu. Amended 2026-10-30 by the
		// review fix IN-T03: each is matched as visible element text, so an id,
		// a field name or an aria-label that happens to hold the word cannot
		// satisfy it.
		">Follow</button>", ">Following</button>", "<span>More</span>",
		">Edit in Settings</a>", ">Remove follower</button>", ">Block</button>",
		// Surface 5, the part of the section this plan draws.
		"Recent reports", "Reports from the last 30 days, newest first.",
		`person-stars__word">stars</span>`,
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
// Following, Muted people and Blocked people (plan 09-11, UI-SPEC Surface 4): the ids the
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

// TestPersonStarsParts pins plan 09-16 (D-02, UI-SPEC 1c and 3a) in the
// template: the two parts follow the total inside #person-stars, each drawn only
// when the server sends it, with no separator; the rating box mounts under the
// action row only when the server says MayRate, and only on another person's
// page; stars.css and stars.js are linked, the script before person.js.
func TestPersonStarsParts(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	p421InOrder(t, "the stars group", tmpl,
		`id="person-stars"`,
		`class="person-stars__total"`,
		`{{if .Profile.StarsFromReports}}<span id="person-stars-from-reports"`,
		`{{if .Profile.StarsFromRatings}}<span id="person-stars-from-ratings"`,
		`id="person-tag"`,
	)
	// Both parts sit inside the group: only the group's own closing div comes
	// between the total and the tag chip.
	group := tmpl[strings.Index(tmpl, `id="person-stars"`):strings.Index(tmpl, `id="person-tag"`)]
	for _, want := range []string{"person-stars-from-reports", "person-stars-from-ratings"} {
		if !strings.Contains(group, want) {
			t.Errorf("%s is not inside the stars group", want)
		}
	}
	if strings.Count(group, "</div>") != 1 {
		t.Errorf("the stars group holds %d closing divs before the tag chip, want 1 (both parts are inside it)", strings.Count(group, "</div>"))
	}
	for _, sep := range []string{"&middot;", string(rune(0xb7)), "&bull;", string(rune(0x2022)), "|"} {
		if strings.Contains(group, sep) {
			t.Errorf("the stars group draws the separator %q, UI-SPEC 1c draws none", sep)
		}
	}
	// The group's accessible name stays the server's whole sentence.
	if !strings.Contains(tmpl, `aria-label="{{.Profile.StarsLabel}}"`) {
		t.Errorf("person-stars no longer takes its aria-label from StarsLabel")
	}
	// The rating box: only when MayRate, under the action row, in the branch of
	// another person's page, above the action error line.
	p421InOrder(t, "the rating mount", tmpl,
		`id="person-more-menu"`,
		`{{if .Profile.MayRate}}`,
		`id="person-rating"`,
		`data-your-rating="{{.Profile.YourRating}}"`,
		`id="person-action-error"`,
	)
	selfStart := strings.Index(tmpl, `{{if .Profile.IsSelf}}`)
	selfBranch := tmpl[selfStart : selfStart+strings.Index(tmpl[selfStart:], `{{else}}`)]
	if strings.Contains(selfBranch, "person-rating") {
		t.Errorf("the own page branch draws the rating box")
	}
	if strings.Count(tmpl, `id="person-rating"`) != 1 {
		t.Errorf("person.html.tmpl draws the rating mount %d times, want once", strings.Count(tmpl, `id="person-rating"`))
	}
	p421InOrder(t, "the stylesheets and scripts", tmpl,
		`href="/static/css/stars.css?v={{.AssetVersion}}"`,
		`href="/static/css/person.css?v={{.AssetVersion}}"`,
		`src="/static/js/stars.js?v={{.AssetVersion}}"`,
		`src="/static/js/person.js?v={{.AssetVersion}}"`,
	)
	css := p421ReadStatic(t, "css/person.css")
	if !strings.Contains(css, ".person-stars__part") || !strings.Contains(css, "--color-text-muted") {
		t.Errorf("person.css does not style .person-stars__part as muted Label text")
	}
	script := p421ReadStatic(t, "js/person.js")
	for _, want := range []string{"PinalertStars", "createControl", "may_rate", "your_rating", "data-your-rating"} {
		if !strings.Contains(script, want) {
			t.Errorf("person.js lacks %q", want)
		}
	}
}

// TestStarsControlLook pins plan 09-16 (UI-SPEC Color, 3a and Interaction): the
// star control is drawn in the text colour only, every star button is 44px
// through --touch-target-min, the hover is the surface fill, nothing animates,
// and the sheet holds no colour other than the text colour, the surface fill and
// Highlight for forced colours.
func TestStarsControlLook(t *testing.T) {
	css := p421ReadStatic(t, "css/stars.css")
	for _, want := range []string{
		".stars-control__star", ".stars-control__glyph", ".stars-control__remove",
		`url("/static/icons/star.svg")`, `url("/static/icons/star-filled.svg")`,
		"background-color: var(--color-text)", "--touch-target-min", "var(--color-surface)",
		"Highlight", "forced-colors",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("stars.css lacks %s", want)
		}
	}
	noComments := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	// Amended 2026-10-29 by Phase 9 plan 17 (UI-SPEC 3b): the star box after
	// Yes, mark resolved is a bordered block, so the rules whose selector names
	// .rate-box may also use the border and background tokens. The control's own
	// rules keep the text and surface colours only, as before.
	for _, rule := range regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatch(noComments, -1) {
		allowed := map[string]bool{"var(--color-text)": true, "var(--color-surface)": true}
		if strings.Contains(rule[1], ".rate-box") {
			allowed["var(--color-border)"] = true
			allowed["var(--color-bg)"] = true
		}
		for _, m := range regexp.MustCompile(`var\(--color-[a-z-]+\)`).FindAllString(rule[2], -1) {
			if !allowed[m] {
				t.Errorf("stars.css uses %s in %q, only the text and surface colours are allowed (the .rate-box rules add the border and background)", m, strings.TrimSpace(rule[1]))
			}
		}
	}
	for _, bad := range []string{"#", "rgb(", "rgba(", "hsl(", "amber", "gold", "!important", "animation", "transition", "@keyframes", "box-shadow", "outline"} {
		if strings.Contains(noComments, bad) {
			t.Errorf("stars.css holds %q: the stars are never coloured, ringed or animated", bad)
		}
	}
	// The glyph fill is the text colour and nothing else.
	glyph := regexp.MustCompile(`(?s)\.stars-control__glyph\s*\{(.*?)\}`).FindStringSubmatch(noComments)
	if glyph == nil || !strings.Contains(glyph[1], "background-color: var(--color-text);") {
		t.Errorf("the .stars-control__glyph rule does not fill with var(--color-text)")
	}
	// Each star button is 44 by 44 through the touch token.
	star := regexp.MustCompile(`(?s)\.stars-control__star\s*\{(.*?)\}`).FindStringSubmatch(noComments)
	if star == nil {
		t.Fatalf("stars.css has no .stars-control__star rule")
	}
	for _, want := range []string{"width: var(--touch-target-min);", "height: var(--touch-target-min);", "min-height: var(--touch-target-min);"} {
		if !strings.Contains(star[1], want) {
			t.Errorf(".stars-control__star lacks %s", want)
		}
	}
	// The five sit 4px apart (--space-xs) on one wrapping row.
	row := regexp.MustCompile(`(?s)\.stars-control__row\s*\{(.*?)\}`).FindStringSubmatch(noComments)
	if row == nil || !strings.Contains(row[1], "gap: var(--space-xs);") {
		t.Errorf(".stars-control__row does not keep a 4px gap through --space-xs")
	}
	// The hover is the surface fill, and a disabled star gets none.
	if !strings.Contains(noComments, ".stars-control__star:hover:not(:disabled)") {
		t.Errorf("stars.css has no hover rule that leaves a disabled star alone")
	}
	// Under forced colours the chosen star takes a Highlight border.
	forced := noComments[strings.Index(noComments, "@media (forced-colors: active)"):]
	if !strings.Contains(forced, `[aria-pressed="true"]`) || !strings.Contains(forced, "border: 1px solid Highlight;") {
		t.Errorf("the forced colours block does not give the chosen star border: 1px solid Highlight")
	}
}

// TestPersonBellMarkup pins plan 09-24 (D-08, D-10, UI-SPEC 1d, 1e and Defaults
// 11): the Notify me bell is a button after Following and before More inside
// #person-actions, it carries aria-pressed and the hidden name text, it is
// hidden unless you follow the person and have not muted them, person.css draws
// bell-off.svg while off and bell.svg while on at 44 by 44 with the surface fill
// as the chosen look and a Highlight border under forced colours, and no
// sentence promises a sound or a vibration.
func TestPersonBellMarkup(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	p421InOrder(t, "the bell sits in the action row after Following and before More", tmpl,
		`id="person-actions"`, `id="person-follow"`, `id="person-unfollow"`, `id="person-bell"`, `id="person-more"`)

	bell := regexp.MustCompile(`(?s)<button[^>]*id="person-bell".*?</button>`).FindString(tmpl)
	if bell == "" {
		t.Fatalf("person.html.tmpl has no button#person-bell")
	}
	for _, want := range []string{
		`type="button"`, `class="person-bell"`, `aria-pressed="`, `.Profile.Notify`,
		`class="visually-hidden">Notify me when {{.Profile.Username}} posts</span>`,
		`class="person-bell__icon" aria-hidden="true"`,
		`{{if or (not .Profile.Following) .Profile.Muted}} hidden{{end}}`,
	} {
		if !strings.Contains(bell, want) {
			t.Errorf("the bell button lacks %s", want)
		}
	}
	// The bell is in the branch for another person's page, never in your own.
	selfStart := strings.Index(tmpl, "{{if .Profile.IsSelf}}\n      <div id=\"person-actions\"")
	otherStart := strings.Index(tmpl, "{{else}}\n      {{/* Follow and Following")
	if selfStart < 0 || otherStart < selfStart {
		t.Fatalf("the IsSelf and other-person branches of #person-actions were not found")
	}
	if strings.Contains(tmpl[selfStart:otherStart], "person-bell") {
		t.Errorf("your own page draws the bell")
	}
	if !strings.Contains(tmpl[otherStart:], `id="person-bell"`) {
		t.Errorf("the bell is not in the other person's branch of #person-actions")
	}

	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(p421ReadStatic(t, "css/person.css"), "")
	rules := regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatch(css, -1)
	var bellCSS strings.Builder
	for _, r := range rules {
		if strings.Contains(r[1], ".person-bell") {
			bellCSS.WriteString(r[0])
		}
	}
	b := bellCSS.String()
	for _, want := range []string{
		".person-bell:not([hidden])", "width: var(--touch-target-min);", "height: var(--touch-target-min);",
		"min-height: var(--touch-target-min);", "border: 1px solid var(--color-border);", "border-radius: 8px;",
		"width: 24px;", "height: 24px;",
		`url("/static/icons/bell-off.svg")`, `url("/static/icons/bell.svg")`,
		`.person-bell[aria-pressed="true"]`, "background: var(--color-surface);",
	} {
		if !strings.Contains(b, want) {
			t.Errorf("the .person-bell rules lack %s", want)
		}
	}
	if regexp.MustCompile(`border[a-z-]*:[^;]*var\(--color-text\)`).MatchString(b) {
		t.Errorf("the bell has a dark border")
	}
	for _, bad := range []string{"!important", "animation", "transition", "@keyframes", "#"} {
		if strings.Contains(b, bad) {
			t.Errorf("the .person-bell rules hold %q: no animation, no hard coded colour", bad)
		}
	}
	// A rule that shows the bell button must be guarded so the hidden attribute
	// works (the icon span inside it is never hidden on its own).
	for _, r := range rules {
		if strings.Contains(r[1], ".person-bell") && !strings.Contains(r[1], "__icon") && strings.Contains(r[2], "display:") && !strings.Contains(r[1], ":not([hidden])") {
			t.Errorf("%q sets display without :not([hidden])", strings.TrimSpace(r[1]))
		}
	}
	// The pressed bell draws bell.svg and the unpressed one bell-off.svg.
	on := regexp.MustCompile(`(?s)\.person-bell\[aria-pressed="true"\] \.person-bell__icon\s*\{(.*?)\}`).FindStringSubmatch(css)
	if on == nil || !strings.Contains(on[1], `url("/static/icons/bell.svg")`) {
		t.Errorf("a pressed bell does not draw bell.svg")
	}
	off := regexp.MustCompile(`(?s)\.person-bell__icon\s*\{(.*?)\}`).FindStringSubmatch(css)
	if off == nil || !strings.Contains(off[1], `url("/static/icons/bell-off.svg")`) {
		t.Errorf("an unpressed bell does not draw bell-off.svg")
	}
	at := strings.Index(css, "@media (forced-colors: active)")
	if at < 0 {
		t.Fatalf("person.css has no forced colours block")
	}
	forced := css[at:]
	if !strings.Contains(forced, `.person-bell[aria-pressed="true"]`) || !strings.Contains(forced, "border: 1px solid Highlight;") {
		t.Errorf("the forced colours block does not give the chosen bell border: 1px solid Highlight")
	}

	// The script: the exact status sentences, the refusal wording, the notify
	// address and the element id; and no word that promises a sound or a
	// vibration anywhere in the bell's markup, rules or script (D-10).
	script := p421ReadStatic(t, "js/person.js")
	for _, want := range []string{
		"'You will be notified when ' + username + ' posts.'",
		"'You will not be notified when ' + username + ' posts.'",
		"notify: 'Couldn\\'t change this. Try again.'",
		"postJSON('notify', { on: want })", "bell: 'person-bell'",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("person.js lacks %s", want)
		}
	}
	for _, text := range []string{bell, b, script} {
		for _, bad := range []string{"buzz", "vibrat", "ringtone", "beep"} {
			if strings.Contains(strings.ToLower(text), bad) {
				t.Errorf("the bell wording holds %q: every phone note is silent (D-10)", bad)
			}
		}
	}
}

// TestPersonColumnHeadsAndHoverKeepContrast pins review fix WR-T02 (2026-10-30).
// The laptop Recent reports column heads and a hovered row's line sit on
// --color-surface, where the muted grey reads 4.34 to 1 in the light theme.
// Muted text stays on --color-bg only; on the surface the text takes
// --color-text, which clears 4.5 to 1 in both themes.
func TestPersonColumnHeadsAndHoverKeepContrast(t *testing.T) {
	rules := p26Rules(t, "person.css")
	heads := p26Find(rules, "", ".person-report-cols")
	if len(heads) == 0 {
		heads = p26Find(rules, "min-width", ".person-report-cols")
	}
	if len(heads) == 0 {
		t.Fatalf("person.css: no .person-report-cols rule")
	}
	for _, r := range heads {
		if c := r.Decls["color"]; c != "" && c != "var(--color-text)" {
			t.Errorf(".person-report-cols color is %q, want var(--color-text): muted text sits only on --color-bg (WR-T02)", c)
		}
		if r.Decls["background"] == "var(--color-surface)" && r.Decls["color"] != "var(--color-text)" {
			t.Errorf(".person-report-cols is on the surface colour and must take var(--color-text) (WR-T02)")
		}
	}
	hover := p26Find(rules, "", "a.person-report__body:hover .person-report__label")
	if len(hover) == 0 {
		t.Errorf("person.css: a hovered row's line must switch to var(--color-text) while the row is on the surface colour (WR-T02)")
	}
	for _, r := range hover {
		if r.Decls["color"] != "var(--color-text)" {
			t.Errorf("hovered row line color is %q, want var(--color-text) (WR-T02)", r.Decls["color"])
		}
	}
	main := p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css"))
	for name, head := range map[string]string{"light": ":root", "dark": ":root[data-theme=\"dark\"]"} {
		d := p16Rule(t, main, "", head).Decls
		got := wcagContrastRatio(t, d["--color-text"], d["--color-surface"], "text on surface, "+name)
		if got < 4.5 {
			t.Errorf("%s theme: --color-text on --color-surface is %.2f to 1, want at least 4.5 (WR-T02)", name, got)
		}
	}
}

// TestPersonPageHasOneH1AtEachSize pins review fix IN-T02 (2026-10-30). Under
// 900px the top bar title is the h1, so the username there is a plain line and
// the username h1 is not drawn; from 900px the top bar title is not drawn, so
// the username h1 shows. The not found and You blocked states draw no username,
// so on the laptop the top bar title is clipped in, never display none, the way
// app-bar.css does it for Nearby.
func TestPersonPageHasOneH1AtEachSize(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	if !strings.Contains(tmpl, `<p class="person-username person-username--phone">{{.Profile.Username}}</p>`) {
		t.Errorf("person.html.tmpl lacks the phone username line (IN-T02)")
	}
	rules := p26Rules(t, "person.css")
	if !p26Has(rules, "", "h1.person-username", "display", "none") {
		t.Errorf("person.css: the username h1 must be hidden under 900px (IN-T02)")
	}
	if !p26Has(rules, "min-width: 900px", "h1.person-username", "display", "block") {
		t.Errorf("person.css: the username h1 must show from 900px (IN-T02)")
	}
	if !p26Has(rules, "min-width: 900px", ".person-username--phone", "display", "none") {
		t.Errorf("person.css: the phone username line must be hidden from 900px (IN-T02)")
	}
	for _, head := range []string{
		"body:has(#person-not-found) .app-topbar__title",
		"body:has(#person-blocked:not([hidden])) .app-topbar__title",
	} {
		found := p26Find(rules, "min-width: 900px", head)
		if len(found) == 0 {
			t.Errorf("person.css: no laptop rule %q seats the top bar title as the h1 of a state page (IN-T02)", head)
		}
		for _, r := range found {
			if r.Decls["display"] != "block" || r.Decls["clip"] != "rect(0, 0, 0, 0)" || r.Decls["position"] != "absolute" {
				t.Errorf("person.css: the state heading rule must clip, not remove, the title: %v (IN-T02)", r.Decls)
			}
		}
	}
}

// TestListWindowCommentsNameMutedPeople pins review fix IN-T06 (2026-10-30):
// since plan 09-20 the shared list window also serves Muted people, so the
// comments that describe it must not list only three kinds.
func TestListWindowCommentsNameMutedPeople(t *testing.T) {
	for file, text := range map[string]string{
		"people_list_window.html.tmpl": p421ReadTemplate(t, "people_list_window.html.tmpl"),
		"settings.html.tmpl":           p421ReadTemplate(t, "settings.html.tmpl"),
		"css/person.css":               p421ReadStatic(t, "css/person.css"),
	} {
		for _, stale := range []string{
			"Followers, Following and Blocked people",
			"Following, Followers and Blocked people",
			"Followers, Following and Blocked\n   people",
		} {
			if strings.Contains(text, stale) {
				t.Errorf("%s still describes the list window without Muted people: %q", file, stale)
			}
		}
	}
}

// TestModerationDocRetentionNamesPhase9Items pins review fix WR-T04
// (2026-10-30): the Retention section of docs/MODERATION.md says the Policy
// page names what Phase 9 keeps, so it must list the same items the Policy
// does: the switch time, ended follows with their reason, a cleared rating that
// stays stored, and the permanent bar on rating.
func TestModerationDocRetentionNamesPhase9Items(t *testing.T) {
	raw, err := os.ReadFile("../docs/MODERATION.md")
	if err != nil {
		t.Fatalf("reading docs/MODERATION.md: %v", err)
	}
	doc := strings.Join(strings.Fields(string(raw)), " ")
	for _, want := range []string{
		"the profile switch with the time it was switched on",
		"each follow kept with an end time and a reason after it ends",
		"A rating its rater clears with Remove my rating stays stored, marked cleared with the time",
		"the account is also barred from rating for good, and the moderator and the time are kept on the account",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("docs/MODERATION.md Retention lacks %q (WR-T04)", want)
		}
	}
}
