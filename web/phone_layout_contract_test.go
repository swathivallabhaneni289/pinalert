package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds the static contract tests of quick task 261013-lrw, the
// phone layout under 900px. Every helper and test name carries the phone
// prefix so a later change can add its own without a clash. Nothing here
// needs a database or a server: the tests read the embedded static and
// template trees and inspect the shipped text, like the other contract files.
//
// The departures from the owner's pictures and from the house rules are
// written in .planning/sketches/005-phone-layout/README.md.

const phoneBlockMedia = "@media (max-width: 899.98px)"

// phoneBaseHideHeads are the three phone only classes that phone.css hides at
// every width outside its media block. The rule declares nothing else.
//
// Departure (quick task 261013-wm5, W-10): .profile-record left this list. The
// reporter record card shows at every width now, because the laptop page draws
// it between the help line and the tabs. activity.css styles it at 900px and
// up and phone.css under 900px.
//
// Departure R6 of plan 03.1-09: .report-row__away left this list. The row shows
// its distance segment on a laptop as the phone already does, so the class is
// no longer hidden at 900px and up (feed.css styles it). The old list held four
// heads, ending ".report-row__away", ".profile-logout".
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10 and D-11):
// .bottom-nav left this list, replaced by the phone only parts of the app bars,
// .app-bar (the four item bottom bar) and the title bar's title, back arrow and
// Settings gear. The Phase 3.1 and Phase 4 hands-on tests are walked on this
// layout after Phase 3.4 is built.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21):
// .profile-logout left this list, because the Log out form is removed from the
// Activity page (Log out lives in Settings and the person menu). The Phase 3.1
// and Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-16, D-17): the
// sheet handle row joins this list, because the phone list sheet and its handle
// are not drawn at 900px and up. The Phase 3.1 and Phase 4 hands-on tests are
// walked on this layout after Phase 3.4 is built.
// Amended 2026-10-19 by Phase 3.4 plan 39: the heading Report of the phone
// report view and the not available view are phone only elements too.
var phoneBaseHideHeads = []string{
	".feed-heading", ".app-bar", ".app-topbar__title", ".app-topbar__back", ".app-topbar__gear",
	".sheet__handle-row", ".report-view__title", ".report-gone",
}

// phoneRules parses phone.css alone.
func phoneRules(t *testing.T) []p16CSSRule {
	t.Helper()
	return p16ParseCSS("static/css/phone.css", p16ReadStatic(t, "static/css/phone.css"))
}

// TestPhoneStylesheetIsLinkedLast pins the load order: phone.css follows every
// other app stylesheet on the feed and on the Activity page, with the same
// AssetVersion query the others carry, so it can override main.css's narrow
// rules without a specificity trick.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10): the new
// app-bar.css links right before phone.css on both pages, so the phone rules of
// the bars override its laptop rules without a specificity trick, and phone.css
// stays last. The Phase 3.1 and Phase 4 hands-on tests are walked on this
// layout after Phase 3.4 is built.
func TestPhoneStylesheetIsLinkedLast(t *testing.T) {
	phone := `/static/css/phone.css?v={{.AssetVersion}}`
	appBar := `/static/css/app-bar.css?v={{.AssetVersion}}`

	index := p13ReadTemplate(t, "index.html.tmpl")
	p13Ordered(t, index, "index.html.tmpl",
		`/static/css/main.css?v={{.AssetVersion}}`,
		`/static/css/modal.css?v={{.AssetVersion}}`,
		`/static/css/feed.css?v={{.AssetVersion}}`,
		`/static/css/trust.css?v={{.AssetVersion}}`,
		`/static/css/auth.css?v={{.AssetVersion}}`,
		appBar,
		phone,
	)
	if n := strings.Count(index, phone); n != 1 {
		t.Errorf("index.html.tmpl links phone.css %d times, want exactly 1", n)
	}

	profile := p13ReadTemplate(t, "profile.html.tmpl")
	p13Ordered(t, profile, "profile.html.tmpl",
		`/static/css/main.css?v={{.AssetVersion}}`,
		`/static/css/feed.css?v={{.AssetVersion}}`,
		`/static/css/trust.css?v={{.AssetVersion}}`,
		`/static/css/auth.css?v={{.AssetVersion}}`,
		appBar,
		phone,
	)
	if n := strings.Count(profile, phone); n != 1 {
		t.Errorf("profile.html.tmpl links phone.css %d times, want exactly 1", n)
	}

	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10 and
	// D-25): Official alerts and Settings are two more pages that carry the app
	// bars, so phone.css is linked last on them too, after places.css. The
	// Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
	// Phase 3.4 is built.
	for _, name := range []string{"alerts.html.tmpl", "settings.html.tmpl"} {
		page := p13ReadTemplate(t, name)
		p13Ordered(t, page, name,
			`/static/css/main.css?v={{.AssetVersion}}`,
			`/static/css/auth.css?v={{.AssetVersion}}`,
			appBar,
			`/static/css/places.css?v={{.AssetVersion}}`,
			phone,
		)
		if n := strings.Count(page, phone); n != 1 {
			t.Errorf("%s links phone.css %d times, want exactly 1", name, n)
		}
	}
}

// TestPhoneRulesLiveInTheNarrowBlock pins that nothing in phone.css can reach
// a screen of 900px or more, except the base rule that hides the phone only
// elements there. Colours, sizes and weights are tokens, and the shapes keep
// the house corner limit.
func TestPhoneRulesLiveInTheNarrowBlock(t *testing.T) {
	rules := phoneRules(t)
	if len(rules) == 0 {
		t.Fatal("phone.css holds no rules")
	}

	baseRules := 0
	hex := regexp.MustCompile(`#[0-9A-Fa-f]{3,8}\b`)
	for _, r := range rules {
		switch r.Media {
		case "":
			baseRules++
			if len(r.Decls) != 1 || r.Decls["display"] != "none" {
				t.Errorf("base rule %q declares %v, want display none only", r.Head, r.Decls)
			}
			for _, part := range p16HeadParts(r.Head) {
				found := false
				for _, want := range phoneBaseHideHeads {
					if part == want {
						found = true
					}
				}
				if !found {
					// Four classes since quick task 261013-wm5 (the record card
					// shows at every width), three since plan 03.1-09 (the row
					// distance shows at every width).
					t.Errorf("base rule head %q is not one of the phone only classes in phoneBaseHideHeads", part)
				}
			}
		default:
			// The phone block, and the short screen block that adds one
			// condition to it, both bounded by the 899.98px query.
			//
			// Amended 2026-10-17 by Phase 3.4 plan 22 (plan 17 finding 5): the
			// parser records a rule inside a container query nested in the
			// phone block with the container query as its Media, so that one
			// named query is accepted too. TestPhoneActivityCardReflowsAtLargeText
			// proves from the raw text that it sits inside the 899.98px block.
			// Nothing broader is allowed.
			if !strings.HasPrefix(r.Media, phoneBlockMedia) && r.Media != phoneActivityContainerMedia {
				t.Errorf("rule %q sits in %q, want only the %s block", r.Head, r.Media, phoneBlockMedia)
			}
		}

		for prop, value := range r.Decls {
			if hex.MatchString(value) {
				t.Errorf("%q %s: %q uses a raw hex colour, tokens only", r.Head, prop, value)
			}
			switch prop {
			case "font-size":
				// Amended 2026-10-23 by plan 04-33: the one exception is the
				// bottom bar item at 380px and under, whose caption token is
				// capped at 20px so the four labels fit a 320px screen at 200
				// percent text. Only that head, that query and that exact
				// value pass; every other phone rule still names a token.
				if r.Head == "body > .app-bar > .app-bar__item" && strings.Contains(r.Media, "(max-width: 380px)") &&
					value == "min(var(--font-size-caption), 20px)" {
					break
				}
				if !strings.HasPrefix(value, "var(--font-size-") {
					t.Errorf("%q font-size %q is not a type size token", r.Head, value)
				}
			case "font-weight":
				if value != "var(--font-weight-regular)" && value != "var(--font-weight-semibold)" {
					t.Errorf("%q font-weight %q is not one of the two weight tokens", r.Head, value)
				}
			case "border-radius":
				for _, component := range strings.Fields(value) {
					// Amended 2026-10-16 by Phase 3.4 (UI-SPEC Radius scale): 4px
					// joins the allowed corners for the short bar above the
					// current bottom bar item. It is below the 8px house corner,
					// so no control becomes a pill.
					//
					// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT
					// D-19 and D-21, UI-SPEC Surface 3): 12px joins the allowed
					// corners for the My activity hub box and the report cards,
					// the same corner the trust panel already uses. No control
					// becomes a pill. The Phase 3.1 and Phase 4 hands-on tests are
					// walked on this layout after Phase 3.4 is built.
					//
					// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT
					// D-16, UI-SPEC 2b): 16px joins the allowed corners for the top
					// corners of the phone list sheet and its handle row, the sheet's own
					// radius. It is a sheet edge, not a control, so no control becomes
					// a pill. The Phase 3.1 and Phase 4 hands-on tests are walked on
					// this layout after Phase 3.4 is built.
					ok := component == "0" || component == "4px" || component == "8px" || component == "12px" || component == "16px" || component == "50%" || component == "0px"
					if !ok {
						t.Errorf("%q border-radius %q is more than the 8px house corner", r.Head, value)
					}
				}
			}
			// The important flag is for the four properties Leaflet writes as
			// inline styles on the detail sheet, and nowhere else.
			if strings.Contains(value, "!important") {
				allowed := map[string]bool{"left": true, "bottom": true, "transform": true, "width": true}
				if !strings.Contains(r.Head, ".leaflet-detail-pane") || !allowed[prop] {
					t.Errorf("%q %s: the important flag is allowed only on left, bottom, transform and width of a detail pane head", r.Head, prop)
				}
			}
		}
	}
	if baseRules != 1 {
		t.Errorf("phone.css has %d base rules, want exactly 1 (the grouped hide rule)", baseRules)
	}
}

// TestPhoneHeadsNeverEqualAnExactLookupHead pins that no phone.css rule
// carries a head another contract test looks up by exact text in main.css,
// feed.css, trust.css or auth.css, so those tests keep reading the base rules.
func TestPhoneHeadsNeverEqualAnExactLookupHead(t *testing.T) {
	forbidden := map[string]bool{
		".fab": true, ".theme-toggle": true, ".app-header": true, ".account-trigger": true,
		".profile-page": true, "#toast": true, ".icon-badge": true, ".icon-badge--pin": true,
		".view-toggle": true, ".report-row": true, ".app-shell": true, ".pane": true,
		".pane--map": true, ".pane--list": true, "#map": true, "#report-list": true,
		".profile-back-link": true, ".profile-email": true, ".profile-activity h2": true,
	}
	for _, r := range phoneRules(t) {
		for _, part := range p16HeadParts(r.Head) {
			// The three phone only classes are new, so a base hide rule is fine.
			if r.Media == "" {
				continue
			}
			if forbidden[part] {
				t.Errorf("phone.css head %q equals a head other tests read by exact text, add a container prefix", part)
			}
		}
	}
}

// TestPhoneBottomNavMarkup pins the bottom bar on the feed.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10): the bar
// is the four item app bar, the three main places drawn from the one Go list
// (a range over .Nav.Items, the current place marked aria-current) and then the
// Report link to /?post=1, which opens the report window and is never current.
// The old two tab Feed and Activity bar is gone. A Past tab is still absent. The
// Phase 3.1 and Phase 4 hands-on tests are walked on this layout after Phase
// 3.4 is built.
func TestPhoneBottomNavMarkup(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")
	if n := strings.Count(index, `{{template "app_bottombar" .}}`); n != 1 {
		t.Fatalf("index.html.tmpl includes the bottom bar %d times, want exactly 1", n)
	}
	if strings.Contains(index, "bottom-nav") {
		t.Errorf("index.html.tmpl still holds the old two tab bottom nav")
	}

	bar := p13ReadTemplate(t, "app_bar.html.tmpl")
	_, bottom, found := strings.Cut(bar, `{{define "app_bottombar"}}`)
	if !found {
		t.Fatal("app_bar.html.tmpl has no app_bottombar block")
	}
	nav := regexp.MustCompile(`(?s)<nav class="app-bar" aria-label="Main">(.*?)</nav>`).FindStringSubmatch(bottom)
	if nav == nil {
		t.Fatal("the app_bottombar block has no nav.app-bar labelled Main")
	}
	if n := strings.Count(nav[1], "{{range .Nav.Items}}"); n != 1 {
		t.Errorf("the bottom bar ranges over .Nav.Items %d times, want 1", n)
	}
	if !strings.Contains(nav[1], `{{if .Current}} aria-current="page"{{end}}`) {
		t.Errorf("the bottom bar items must mark the current place with aria-current from the list")
	}
	report := regexp.MustCompile(`<a class="app-bar__item app-bar__item--report" href="/\?post=1" data-open-report>`)
	if !report.MatchString(nav[1]) {
		t.Errorf("the bottom bar needs the Report link to /?post=1 with data-open-report")
	}
	if strings.Contains(nav[1], "Past") {
		t.Errorf("bottom nav names a Past tab, which needs a history view that does not exist")
	}
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-16, D-17):
	// the List and Map switch is retired. The list is a sheet over the map with
	// a handle button (a real button with an expanded state, named by its
	// action), and the markup holds no switch and no data-view attribute. The
	// Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
	// Phase 3.4 is built.
	if strings.Contains(index, `view-toggle`) || strings.Contains(index, `data-view=`) {
		t.Errorf("the markup still holds the retired List and Map switch or its data-view attribute")
	}
	if !strings.Contains(index, `<button type="button" class="sheet__handle" id="sheet-handle" aria-expanded="false"`) {
		t.Errorf("the phone list sheet needs its handle button with aria-expanded")
	}
}

// TestPhoneFeedHeadingMarkup pins the list head and the live count slot inside
// the list pane, in front of the list itself.
//
// Test change on purpose (plan 03.1-05): the title gained id="feed-heading-title"
// so feed.js can write Nearby now, Disputed reports or Past reports into it,
// and the strip sits first in the pane. The old literal was
// <h2 class="feed-heading__title">Nearby now</h2>.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-13, D-14,
// D-15, D-18): the tab strip and the heading are gone. The pane holds, in
// order, the hidden official alert banner slot, the list head (the count line
// and the Show button with its menu), the polite status line and the list.
// The Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
// Phase 3.4 is built.
func TestPhoneFeedHeadingMarkup(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")
	p13Ordered(t, index, "index.html.tmpl",
		`class="pane pane--list"`,
		`<a class="alert-banner" id="alert-banner" href="/alerts" hidden>`,
		`<div class="list-head">`,
		`<p class="list-head__count" id="feed-count"></p>`,
		`id="show-button"`,
		`<div class="show-menu" id="show-menu" role="menu"`,
		`id="feed-status"`,
		`<ol id="report-list">`,
	)
	p13MustNotContain(t, "index.html.tmpl", index, `<nav class="view-tabs"`, `class="feed-heading"`, `id="feed-heading-title"`)
	if n := strings.Count(index, `id="feed-count"`); n != 1 {
		t.Errorf("index.html.tmpl has %d feed-count nodes, want 1", n)
	}
}

// TestPhoneFeedScriptContract pins feed.js: the count line is real numbers
// written through setText, the distance line follows the reader's own
// position rule, and the phone list scroll sets only the list pane's own
// scroll position, from the map branch of the one onSelect subscriber.
func TestPhoneFeedScriptContract(t *testing.T) {
	code := p19Code(t, "static/js/feed.js")

	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-13, D-14,
	// D-18): the count line is "{N} reports, {R} km", N being the cards shown
	// under the Show choice (Pinalert.visibleReports), with no tab word in it.
	// The Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
	// Phase 3.4 is built.
	count := jsFunctionBody(t, code, "updateFeedCount(state)")
	p13MustContain(t, "updateFeedCount", count,
		"Pinalert.config.defaultRadiusKm", "Pinalert.visibleReports().length", "' report, '", "' reports, '",
		"Pinalert.setText(feedCountEl, text)")
	p13MustNotContain(t, "updateFeedCount", count, "'disputed'", "'active'", "'past'")
	p13MustNotContain(t, "updateFeedCount", count, "innerHTML", "insertAdjacentHTML")

	render := jsFunctionBody(t, code, "render(state)")
	p13Ordered(t, render, "render", "updateFeedCount(state);", "renderFirstLoad(state);")

	create := jsFunctionBody(t, code, "createRow(id)")
	p13MustContain(t, "createRow", create, "'report-row__away'", "body.appendChild(away)", "away: away")
	// Plan 03.1-09 (R6): the distance is the store's one sentence at every
	// width. The old literals were Pinalert.feedIsAroundReader() and
	// Pinalert.formatDistance(report.distance_km); the around the reader rule
	// now lives inside Pinalert.distanceText (plan 05).
	update := jsFunctionBody(t, code, "updateRow(row, report)")
	p13Ordered(t, update, "updateRow",
		"Pinalert.distanceText(report)",
		"Pinalert.setText(row.away, away)", "row.away.hidden = !away")

	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-16, D-17):
	// the list is the sheet on a phone, so the reveal scrolls the sheet's own
	// element (found by its id, sheet.scrollTo) and no longer asks the phone
	// query: on a laptop the pane is not a scroll box and the scrollHeight check
	// leaves it alone. PHONE_QUERY left feed.js with the check. The Phase 3.1 and
	// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
	p13MustNotContain(t, "feed.js", code, "PHONE_QUERY")

	reveal := jsFunctionBody(t, code, "revealRowInListPane(id)")
	p13MustContain(t, "revealRowInListPane", reveal,
		"getElementById('report-sheet')", "sheet.scrollHeight <= sheet.clientHeight", "sheet.scrollTo(", "rowsById[String(id)]")
	banned := regexp.MustCompile(`(window|document\.documentElement|document\.body)\.scroll|scrollingElement|scrollIntoView`)
	if m := banned.FindString(reveal); m != "" {
		t.Errorf("revealRowInListPane calls %q, it may set only the list pane's own scroll position", m)
	}

	if n := strings.Count(code, "revealRowInListPane("); n != 2 {
		t.Errorf("revealRowInListPane appears %d times (definition and one call), want 2", n)
	}
	subscriber, anchorFound, terminatorFound := windowAfter(code, "Pinalert.onSelect(", "});")
	if !anchorFound || !terminatorFound {
		t.Fatalf("could not bound the Pinalert.onSelect subscriber in feed.js")
	}
	p13Ordered(t, subscriber, "onSelect subscriber",
		"source === 'map'", "applySelectionHighlight(id);", "revealRowInListPane(id);")

	activate := jsFunctionBody(t, code, "activateRow(id)")
	p13MustNotContain(t, "activateRow", activate, "revealRowInListPane")
}

// TestPhoneActivityPageMarkup pins the phone My activity page's markup: phone.css
// follows auth.css, the record card sits inside the with block that skips it for
// an account with no signal and inside the Posted by you part, the page carries
// the part it shows as data-part, the hub holds one link per part, and the
// bottom bar is included once.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21):
// the Log out form, the email line and the back link are gone from the page
// (Log out lives in Settings and the person menu), and the six tab strip became
// the hub of five part links. The Phase 3.1 and Phase 4 hands-on tests are
// walked on this layout after Phase 3.4 is built.
func TestPhoneActivityPageMarkup(t *testing.T) {
	profile := p13ReadTemplate(t, "profile.html.tmpl")

	for _, gone := range []string{`class="profile-logout"`, `class="profile-email"`, `class="profile-back-link"`, `activity-tabs`} {
		if strings.Contains(profile, gone) {
			t.Errorf("profile.html.tmpl still holds %s", gone)
		}
	}
	p13Ordered(t, profile, "profile.html.tmpl",
		`<main class="profile-page"`, `data-part="{{.CurrentPart}}"`,
		`<nav class="activity-hub" aria-label="My activity">`, `{{range .Parts}}`, `class="activity-hub__row"`, `href="{{.Href}}"`,
		`</nav>`, `<section class="activity-part`, `{{if eq .Key "posted"}}`,
		`{{with $.Record}}`, `<section class="profile-record"`, `{{.Label}}`, `{{.Counts}}`, `</section>`,
		`<ol class="profile-report-list">`)

	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10): the
	// Activity page carries the same four item app bar as the map page, drawn
	// from the one list of places; the current place is marked by the list
	// (checked on the rendered page by TestAppBarCurrentItem). The Phase 3.1
	// and Phase 4 hands-on tests are walked on this layout after Phase 3.4 is
	// built.
	if n := strings.Count(profile, `{{template "app_bottombar" .}}`); n != 1 {
		t.Fatalf("profile.html.tmpl includes the bottom bar %d times, want exactly 1", n)
	}
	if strings.Contains(profile, "bottom-nav") {
		t.Errorf("profile.html.tmpl still holds the old two tab bottom nav")
	}
}

// TestPhoneActivityStylesheet pins the phone My activity rules: the hub shows
// only on the list, a part shows only when it is the current one, the report
// card is a grid of disc and chip, text, when, and what happened beside the
// distance, and the cards use the border token hairline on the page colour.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21):
// rewritten from the six tab page. The Phase 3.1 and Phase 4 hands-on tests are
// walked on this layout after Phase 3.4 is built.
func TestPhoneActivityStylesheet(t *testing.T) {
	rules := phoneRules(t)
	find := func(head string) p16CSSRule {
		t.Helper()
		for _, r := range rules {
			if r.Head == head && r.Media == phoneBlockMedia {
				return r
			}
		}
		t.Fatalf("phone.css has no rule with head %q", head)
		return p16CSSRule{}
	}

	if got := find("main.profile-page").Decls["display"]; got != "block" {
		t.Errorf("main.profile-page display is %q on phones, want block", got)
	}
	if got := find("main.profile-page[data-part=\"\"] > .activity-hub").Decls["display"]; got != "block" {
		t.Errorf("the hub display is %q on the list, want block", got)
	}
	if got := find("main.profile-page > .activity-part").Decls["display"]; got != "none" {
		t.Errorf("a part display is %q by default on phones, want none", got)
	}
	if got := find("main.profile-page > .activity-part[data-current]").Decls["display"]; got != "block" {
		t.Errorf("the current part display is %q, want block", got)
	}
	if got := find("main.profile-page > .activity-part .activity-cols").Decls["display"]; got != "none" {
		t.Errorf("the column head row display is %q on phones, want none", got)
	}
	if got := find("main.profile-page > .activity-part .activity-part__title").Decls["display"]; got != "none" {
		t.Errorf("the part title display is %q on phones, want none (the top bar names the screen)", got)
	}

	row := find("main.profile-page > .activity-hub .activity-hub__row")
	if row.Decls["min-height"] != "72px" {
		t.Errorf("a hub row min-height is %q, want 72px", row.Decls["min-height"])
	}
	box := find("main.profile-page[data-part=\"\"] > .activity-hub")
	if box.Decls["border"] != "1px solid var(--color-border)" || box.Decls["background"] != "var(--color-bg)" || box.Decls["overflow"] != "hidden" {
		t.Errorf("the hub box must be a rounded box with the hairline on the page colour, got %v", box.Decls)
	}
	if got := find("main.profile-page > .activity-hub .activity-hub__summary").Decls["text-overflow"]; got != "ellipsis" {
		t.Errorf("the hub summary text-overflow is %q, want ellipsis", got)
	}

	card := find("main.profile-page .profile-report-list > .activity-card")
	if card.Decls["display"] != "grid" || card.Decls["background"] != "var(--color-bg)" || card.Decls["border"] != "1px solid var(--color-border)" {
		t.Errorf("the report card must be a grid with the hairline on the page colour, got %v", card.Decls)
	}
	areas := strings.Join(strings.Fields(card.Decls["grid-template-areas"]), " ")
	if areas != `"disc status" "report report" "when when" "what distance"` {
		t.Errorf("card grid-template-areas is %q, want disc and chip, text, when, then what happened and distance", areas)
	}
	for _, c := range []struct{ head, area string }{
		{"main.profile-page .profile-report-list > .activity-card > .icon-badge", "disc"},
		{"main.profile-page .profile-report-list > .activity-card > .activity-card__status", "status"},
		{"main.profile-page .profile-report-list > .activity-card > .activity-card__report", "report"},
		{"main.profile-page .profile-report-list > .activity-card > .activity-card__when", "when"},
		{"main.profile-page .profile-report-list > .activity-card > .activity-card__what", "what"},
		{"main.profile-page .profile-report-list > .activity-card > .activity-card__distance", "distance"},
	} {
		if got := find(c.head).Decls["grid-area"]; got != c.area {
			t.Errorf("%s grid-area is %q, want %q", c.head, got, c.area)
		}
	}
	if got := find("main.profile-page:not([data-part=\"\"]):not([data-part=\"posted\"]) > .queue-strip").Decls["display"]; got != "none" {
		t.Errorf("the saved reports strip display is %q on the other part screens, want none", got)
	}
}

// TestPhoneDetailSheetStylesheet pins the sheet: a fixed popup in the detail
// pane starting under the 192px strip, the three Leaflet written properties
// carrying the important flag, the back arrow mask, and the detail mode that
// steps the zoom control and the theme toggle aside.
func TestPhoneDetailSheetStylesheet(t *testing.T) {
	rules := phoneRules(t)
	byHead := func(media, head string) []p16CSSRule {
		var found []p16CSSRule
		for _, r := range rules {
			if r.Head == head && strings.HasPrefix(r.Media, media) {
				found = append(found, r)
			}
		}
		return found
	}
	must := func(head string) []p16CSSRule {
		t.Helper()
		found := byHead(phoneBlockMedia, head)
		if len(found) == 0 {
			t.Fatalf("phone.css has no rule with head %q", head)
		}
		return found
	}
	declared := func(rs []p16CSSRule, prop string) string {
		for _, r := range rs {
			if v, ok := r.Decls[prop]; ok {
				return v
			}
		}
		return ""
	}

	pane := must(".leaflet-container .leaflet-detail-pane")
	if v := declared(pane, "z-index"); v != "1080" {
		t.Errorf("detail pane z-index is %q, want 1080 (above the bar at 1010, below the modal at 1100)", v)
	}

	// Amended 2026-10-19 by Phase 3.4 plan 39 (the report view of the locked
	// sketch 008): the report starts under the 60px top row and a 160px map
	// strip, was 192px from the top with no top row of its own, and ends on the
	// bottom bar, which stays shown as in the sketch, where it ran to the
	// screen bottom over the bar. TestReportViewPhoneStylesheet pins the rest.
	// Amended 2026-10-21 by plan 40: the top controls float over the strip, so
	// the strip starts at the top of the page and the report starts 160px down,
	// where it was calc(var(--app-topbar-height) + var(--space-3xl) * 2.5).
	popup := must(".leaflet-container .leaflet-detail-pane .leaflet-popup")
	for prop, want := range map[string]string{
		"position": "fixed", "top": "calc(var(--space-3xl) * 2.5)", "right": "0",
		"overflow": "visible",
		"left":     "0 !important", "bottom": "calc(var(--app-bottombar-height) + env(safe-area-inset-bottom)) !important", "transform": "none !important",
	} {
		if got := declared(popup, prop); got != want {
			t.Errorf("sheet %s is %q, want %q", prop, got, want)
		}
	}
	// Amended 2026-10-21: Safari on the iPhone clipped Back and Report (fixed
	// children of the popup) to the popup while it was the scroller, so the
	// wrapper scrolls instead.
	wrapper := must(".leaflet-container .leaflet-detail-pane .leaflet-popup-content-wrapper")
	for prop, want := range map[string]string{
		"height": "100%", "overflow-x": "hidden", "overflow-y": "auto", "overscroll-behavior": "contain", "touch-action": "pan-y",
	} {
		if got := declared(wrapper, prop); got != want {
			t.Errorf("sheet wrapper %s is %q, want %q", prop, got, want)
		}
	}

	content := must(".leaflet-container .leaflet-detail-pane .leaflet-popup-content")
	if got := declared(content, "width"); got != "auto !important" {
		t.Errorf("sheet content width is %q, want auto !important (Leaflet writes it inline)", got)
	}
	if got := declared(content, "max-height"); got != "none" {
		t.Errorf("sheet content max-height is %q, want none", got)
	}

	// Browser finding F-P4-5: on a short landscape phone the sheet starts right
	// under the unofficial bar. The bar token follows the real bar (theme.js),
	// so a fixed 64px value started the sheet inside a 68px bar at 320x256.
	//
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
	// D-31): the bar on every page is replaced by the laptop strip on Nearby and
	// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
	// every page) is retired as worded; Disclaimer and Policy are reached from the
	// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
	// first-time flow, the login screen and the one reader. The Phase 3.1 and
	// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
	// The rule still reads the token; on a phone the token is 0px (the strip is
	// hidden there), so the sheet starts at the top of its pane.
	// Amended 2026-10-19 by Phase 3.4 plan 39: the top row with Back and the
	// heading stays on a short screen too, so the report starts under it.
	short := byHead("@media (max-width: 899.98px) and (max-height: 359px)", ".leaflet-container .leaflet-detail-pane .leaflet-popup")
	if len(short) != 1 || short[0].Decls["top"] != "calc(var(--disclaimer-bar-height) + var(--app-topbar-height))" {
		t.Errorf("short screen block must start the view under the bar and the top row, got %v", short)
	}

	back := must(".leaflet-container .leaflet-detail-pane .leaflet-popup a.leaflet-popup-close-button:not([hidden])::before")
	if got := declared(back, "mask-image"); got != `url("/static/icons/arrow-left.svg")` {
		t.Errorf("back arrow mask is %q, want the arrow-left icon", got)
	}
	arrow := must(".leaflet-container .leaflet-detail-pane .leaflet-popup a.leaflet-popup-close-button:not([hidden])")
	if got := declared(arrow, "position"); got != "fixed" {
		t.Errorf("back arrow position is %q, want fixed", got)
	}

	var detailMode bool
	for _, r := range rules {
		if strings.HasPrefix(r.Head, `body:has(.leaflet-detail-pane .leaflet-popup[data-popup-open="true"])`) &&
			r.Decls["display"] == "none" &&
			strings.Contains(r.Head, ".leaflet-control-zoom") && strings.Contains(r.Head, ".map-tools") {
			detailMode = true
		}
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-08, UX-01
		// as amended): the theme toggle left this list; it is in the laptop top
		// bar and hidden on every phone by .app-topbar > .theme-toggle. The Phase
		// 3.1 and Phase 4 hands-on tests are walked on this layout after Phase 3.4
		// is built.
		if strings.Contains(r.Head, "> .theme-toggle") && strings.Contains(r.Head, "body:has(") {
			t.Errorf("phone.css still has a body:has guard for the floating theme toggle: %q", r.Head)
		}
	}
	if !detailMode {
		t.Errorf("phone.css has no detail mode rule hiding the zoom control and the map tools")
	}

	// The markup the sheet restyles is never edited by the sheet: no rule here
	// generates content that adds words, apart from the dots and the empty
	// content of the marks.
	for _, r := range rules {
		if v, ok := r.Decls["content"]; ok {
			switch v {
			case `""`, `"\00b7"`, `"\00b7" / ""`:
			default:
				t.Errorf("%q declares content %q, phone.css adds no words", r.Head, v)
			}
		}
	}
}

// TestPhoneDetailMapScript pins map.js: the detail pane is created inside the
// map container, every popup gets its pane and autoPan written before it can
// open, a resize across 900px closes the open popup, the pan helper stands
// down on phones, and the pin is centred in the strip only behind the phone
// check.
func TestPhoneDetailMapScript(t *testing.T) {
	code := p19Code(t, "static/js/map.js")

	initBody := jsFunctionBody(t, code, "init()")
	// Amended 2026-10-21: the detail pane moved to a page level host, because
	// Safari on the iPhone clipped the fixed view to the map container.
	p13Ordered(t, initBody, "init",
		"map = L.map(container,",
		"document.body.appendChild(detailHost)",
		"map.createPane('detailPane', detailHost)",
		"L.DomEvent.disableClickPropagation(detailPane)",
		"L.DomEvent.disableScrollPropagation(detailPane)",
		"if (hasVectorBasemap())")

	// Amended 2026-10-21: the card comes in too, so the phone puts Confirm and
	// Dispute above the trust details (owner) and the laptop keeps its order.
	place := jsFunctionBody(t, code, "applyPopupPlacement(popup, card)")
	p13MustContain(t, "applyPopupPlacement", place,
		"popup.options.pane = phone ? 'detailPane' : 'popupPane'",
		"popup.options.autoPan = !reducedMotion && !phone",
		"arrangeCard(card, phone);")
	arrange := jsFunctionBody(t, code, "arrangeCard(card, phone)")
	p13Ordered(t, arrange, "arrangeCard",
		"if (phone) {",
		"body.insertBefore(fade, trust);",
		"body.insertBefore(actions, trust);",
		"} else {",
		"body.insertBefore(trust, fade);")

	upsert := jsFunctionBody(t, code, "upsertMarker(report)")
	p13Ordered(t, upsert, "upsertMarker", "marker.bindPopup(", "applyPopupPlacement(marker.getPopup(), card);", "marker.on('click'")
	p13MustContain(t, "upsertMarker bind literal", upsert, "autoPan: !reducedMotion", "keepInView: true")

	resize := jsFunctionBody(t, code, "onMapResize()")
	p13Ordered(t, resize, "onMapResize",
		"var narrow = phoneDetail();", "var crossed = narrow !== lastNarrow;", "lastNarrow = narrow;",
		"map.closePopup();", "applyPopupPlacement(markers[ids[i]].getPopup(), cards[ids[i]]);")
	p13MustContain(t, "onMapResize", resize, "applyPanPadding(markers[ids[i]].getPopup());")

	pan := jsFunctionBody(t, code, "panPopupIntoView(popup)")
	p13Ordered(t, pan, "panPopupIntoView", "if (!reducedMotion ||", "if (phoneDetail()) {\n      return;\n    }", "var el = popup.getElement();")

	open := jsFunctionBody(t, code, "onPopupOpen(evt)")
	p13Ordered(t, open, "onPopupOpen",
		"card.focus({ preventScroll: true });", "if (phoneDetail()) {", "centerPinInStrip(popup, container);")
	p13MustNotContain(t, "onPopupOpen", open, "setView(", "panBy(")
	if n := strings.Count(code, "centerPinInStrip("); n != 2 {
		t.Errorf("centerPinInStrip appears %d times (definition and one call), want 2", n)
	}

	strip := jsFunctionBody(t, code, "centerPinInStrip(popup, popupEl)")
	// Plan 03.1-13 (D-23): an approximate pin is capped at the zoom ceiling
	// and a live pin keeps street level. The old pin was the single literal
	// map.setView(popup.getLatLng(), Math.max(map.getZoom(), 16), { animate: false });
	p13Ordered(t, strip, "centerPinInStrip",
		"getBoundingClientRect().top", "Math.max(map.getZoom(), 16)",
		"map.setView(popup.getLatLng(), zoom, { animate: false });",
		"map.panBy([0, Math.round(map.getSize().y / 2 - strip / 2)], { animate: false });")

	// The parts the sheet must leave alone.
	closeBody := jsFunctionBody(t, code, "onPopupClose(evt)")
	p13MustNotContain(t, "onPopupClose", closeBody, "phoneDetail", "detailPane")
	returnFocus := jsFunctionBody(t, code, "returnFocusToOpener(opener)")
	p13MustNotContain(t, "returnFocusToOpener", returnFocus, "phoneDetail", "detailPane")
	fly := jsFunctionBody(t, code, "flyTo(id, opts)")
	p13MustNotContain(t, "flyTo", fly, "phoneDetail", "detailPane")
}
