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

// phoneBaseHideHeads are the four phone only classes that phone.css hides at
// every width outside its media block. The rule declares nothing else.
//
// Departure (quick task 261013-wm5, W-10): .profile-record left this list. The
// reporter record card shows at every width now, because the laptop page draws
// it between the help line and the tabs. activity.css styles it at 900px and
// up and phone.css under 900px.
var phoneBaseHideHeads = []string{
	".feed-heading", ".bottom-nav", ".report-row__away", ".profile-logout",
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
func TestPhoneStylesheetIsLinkedLast(t *testing.T) {
	phone := `/static/css/phone.css?v={{.AssetVersion}}`

	index := p13ReadTemplate(t, "index.html.tmpl")
	p13Ordered(t, index, "index.html.tmpl",
		`/static/css/main.css?v={{.AssetVersion}}`,
		`/static/css/modal.css?v={{.AssetVersion}}`,
		`/static/css/feed.css?v={{.AssetVersion}}`,
		`/static/css/trust.css?v={{.AssetVersion}}`,
		`/static/css/auth.css?v={{.AssetVersion}}`,
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
		phone,
	)
	if n := strings.Count(profile, phone); n != 1 {
		t.Errorf("profile.html.tmpl links phone.css %d times, want exactly 1", n)
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
					// shows at every width).
					t.Errorf("base rule head %q is not one of the four phone only classes", part)
				}
			}
		default:
			// The phone block, and the short screen block that adds one
			// condition to it, both bounded by the 899.98px query.
			if !strings.HasPrefix(r.Media, phoneBlockMedia) {
				t.Errorf("rule %q sits in %q, want only the %s block", r.Head, r.Media, phoneBlockMedia)
			}
		}

		for prop, value := range r.Decls {
			if hex.MatchString(value) {
				t.Errorf("%q %s: %q uses a raw hex colour, tokens only", r.Head, prop, value)
			}
			switch prop {
			case "font-size":
				if !strings.HasPrefix(value, "var(--font-size-") {
					t.Errorf("%q font-size %q is not a type size token", r.Head, value)
				}
			case "font-weight":
				if value != "var(--font-weight-regular)" && value != "var(--font-weight-semibold)" {
					t.Errorf("%q font-weight %q is not one of the two weight tokens", r.Head, value)
				}
			case "border-radius":
				for _, component := range strings.Fields(value) {
					ok := component == "0" || component == "8px" || component == "50%" || component == "0px"
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
			// The four phone only classes are new, so a base hide rule is fine.
			if r.Media == "" {
				continue
			}
			if forbidden[part] {
				t.Errorf("phone.css head %q equals a head other tests read by exact text, add a container prefix", part)
			}
		}
	}
}

// TestPhoneBottomNavMarkup pins the bottom bar on the feed: exactly two tabs,
// Feed current, Activity linking to the account page, and no Past tab (the
// history view does not exist, so a Past tab would be a dead control).
func TestPhoneBottomNavMarkup(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")

	nav := regexp.MustCompile(`(?s)<nav class="bottom-nav"[^>]*>(.*?)</nav>`).FindStringSubmatch(index)
	if nav == nil {
		t.Fatal("index.html.tmpl has no nav.bottom-nav")
	}
	tabs := regexp.MustCompile(`<a class="bottom-nav__tab"([^>]*)>([^<]*)</a>`).FindAllStringSubmatch(nav[1], -1)
	if len(tabs) != 2 {
		t.Fatalf("bottom nav has %d tabs, want 2", len(tabs))
	}
	if !strings.Contains(tabs[0][1], `href="/"`) || !strings.Contains(tabs[0][1], `aria-current="page"`) ||
		strings.TrimSpace(tabs[0][2]) != "Feed" {
		t.Errorf("first tab is %q %q, want the current Feed tab linking to /", tabs[0][1], tabs[0][2])
	}
	if !strings.Contains(tabs[1][1], `href="/profile"`) || strings.Contains(tabs[1][1], "aria-current") ||
		strings.TrimSpace(tabs[1][2]) != "Activity" {
		t.Errorf("second tab is %q %q, want the plain Activity tab linking to /profile", tabs[1][1], tabs[1][2])
	}
	if strings.Contains(nav[1], "Past") {
		t.Errorf("bottom nav names a Past tab, which needs a history view that does not exist")
	}
	if strings.Contains(index, `class="view-toggle" id="view-toggle"`) == false {
		t.Errorf("the view toggle button stays in the markup, only phone.css hides it on phones")
	}
}

// TestPhoneFeedHeadingMarkup pins the heading and the live count slot inside
// the list pane, in front of the list itself.
func TestPhoneFeedHeadingMarkup(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")
	p13Ordered(t, index, "index.html.tmpl",
		`class="pane pane--list"`,
		`<div class="feed-heading">`,
		`<h2 class="feed-heading__title">Nearby now</h2>`,
		`<p class="feed-heading__count" id="feed-count"></p>`,
		`<ol id="report-list">`,
	)
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

	count := jsFunctionBody(t, code, "updateFeedCount(state)")
	p13MustContain(t, "updateFeedCount", count,
		"Pinalert.config.defaultRadiusKm", "state.reports.length", "'disputed'", "'active'",
		"Pinalert.setText(feedCountEl, text)")
	p13MustNotContain(t, "updateFeedCount", count, "innerHTML", "insertAdjacentHTML")

	render := jsFunctionBody(t, code, "render(state)")
	p13Ordered(t, render, "render", "updateFeedCount(state);", "state.status === 'idle'")

	create := jsFunctionBody(t, code, "createRow(id)")
	p13MustContain(t, "createRow", create, "'report-row__away'", "body.appendChild(away)", "away: away")
	update := jsFunctionBody(t, code, "updateRow(row, report)")
	p13Ordered(t, update, "updateRow",
		"Pinalert.feedIsAroundReader()", "Pinalert.formatDistance(report.distance_km)",
		"Pinalert.setText(row.away, away)", "row.away.hidden = !away")

	p13MustContain(t, "feed.js", code, "var PHONE_QUERY = '(max-width: 899.98px)';")

	reveal := jsFunctionBody(t, code, "revealRowInListPane(id)")
	p13MustContain(t, "revealRowInListPane", reveal,
		"window.matchMedia(PHONE_QUERY).matches", "closest('.pane--list')", "pane.scrollTo(", "rowsById[String(id)]")
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

// TestPhoneActivityPageMarkup pins the phone Activity page's markup: phone.css
// follows auth.css, the Log out form is a copy of the account header's form
// (same method and action, one submit button, no other field), the record card
// sits inside the with block that skips it for an account with no signal, the
// bottom nav has two tabs with Activity current and no Past, and the back link
// is still there exactly once.
func TestPhoneActivityPageMarkup(t *testing.T) {
	profile := p13ReadTemplate(t, "profile.html.tmpl")
	header := p13ReadTemplate(t, "account_header.html.tmpl")

	headerForm := regexp.MustCompile(`<form method="(\w+)" action="([^"]+)">`).FindStringSubmatch(header)
	if headerForm == nil {
		t.Fatal("account_header.html.tmpl has no logout form")
	}
	pageForm := regexp.MustCompile(`(?s)<form class="profile-logout" method="(\w+)"\s+action="([^"]+)">(.*?)</form>`).FindStringSubmatch(profile)
	if pageForm == nil {
		t.Fatal("profile.html.tmpl has no .profile-logout form")
	}
	if pageForm[1] != headerForm[1] || pageForm[2] != headerForm[2] {
		t.Errorf("Log out form is %s %s, the account header's is %s %s", pageForm[1], pageForm[2], headerForm[1], headerForm[2])
	}
	if n := strings.Count(pageForm[3], "<button"); n != 1 || strings.Contains(pageForm[3], "<input") {
		t.Errorf("Log out form holds %d buttons or an input, want exactly one submit button and no other field", n)
	}
	p13Ordered(t, profile, "profile.html.tmpl",
		`<section class="profile-activity">`, `<h2>Activity</h2>`, `<form class="profile-logout"`,
		`{{with .Record}}`, `<section class="profile-record"`, `{{.Label}}`, `{{.Counts}}`, `{{end}}`,
		`<ol class="profile-report-list">`)

	nav := regexp.MustCompile(`(?s)<nav class="bottom-nav"[^>]*>(.*?)</nav>`).FindStringSubmatch(profile)
	if nav == nil {
		t.Fatal("profile.html.tmpl has no bottom nav")
	}
	tabs := regexp.MustCompile(`<a class="bottom-nav__tab"([^>]*)>([^<]*)</a>`).FindAllStringSubmatch(nav[1], -1)
	if len(tabs) != 2 {
		t.Fatalf("bottom nav has %d tabs, want 2", len(tabs))
	}
	if strings.Contains(tabs[0][1], "aria-current") || strings.TrimSpace(tabs[0][2]) != "Feed" {
		t.Errorf("first tab is %q %q, want a plain Feed tab", tabs[0][1], tabs[0][2])
	}
	if !strings.Contains(tabs[1][1], `aria-current="page"`) || strings.TrimSpace(tabs[1][2]) != "Activity" {
		t.Errorf("second tab is %q %q, want the current Activity tab", tabs[1][1], tabs[1][2])
	}
	if strings.Contains(nav[1], "Past") {
		t.Errorf("bottom nav names a Past tab")
	}

	if n := strings.Count(profile, `class="profile-back-link"`); n != 1 {
		t.Errorf("profile.html.tmpl has the back link %d times, want exactly 1", n)
	}
	p13Ordered(t, profile, "profile.html.tmpl", `class="profile-back-link"`, `class="profile-email"`)
}

// TestPhoneActivityStylesheet pins the Activity rules: the title and Log out
// share the first grid row, the back link is not drawn, the record card and
// the cards use the border token hairline on the page colour, and Log out
// keeps the margin that clears the fixed theme toggle.
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

	page := find("main.profile-page")
	if page.Decls["display"] != "grid" {
		t.Errorf("main.profile-page display is %q, want grid", page.Decls["display"])
	}
	areas := strings.Join(strings.Fields(page.Decls["grid-template-areas"]), " ")
	// Departure (quick task 261013-wm5): the grid gains a tabs row between the
	// record card and the list, and an empty row after the list for the one
	// empty state each tab owns. The first three rows are unchanged.
	if !strings.HasPrefix(areas, `"title logout" "email email" "record record" "tabs tabs" "list list" "empty empty"`) {
		t.Errorf("grid-template-areas is %q, want title and logout on the first row, then tabs, list and empty", areas)
	}
	if got := find("main.profile-page > .profile-back-link").Decls["display"]; got != "none" {
		t.Errorf("back link display is %q on phones, want none", got)
	}
	if got := find("main.profile-page > .profile-activity").Decls["display"]; got != "contents" {
		t.Errorf(".profile-activity display is %q on phones, want contents", got)
	}
	logout := find("main.profile-page .profile-logout")
	if logout.Decls["margin"] != "0 calc(var(--touch-target-min) + var(--space-sm)) 0 0" {
		t.Errorf("Log out margin is %q, want the touch target plus the small space on the right", logout.Decls["margin"])
	}
	button := find("main.profile-page .profile-logout .profile-logout__button")
	if button.Decls["min-height"] != "var(--touch-target-min)" || button.Decls["color"] != "var(--color-text-muted)" {
		t.Errorf("Log out button must be 44px tall and muted, got %v", button.Decls)
	}

	for _, head := range []string{"main.profile-page .profile-record", "main.profile-page .profile-report-list .report-row"} {
		r := find(head)
		if r.Decls["background"] != "var(--color-bg)" || r.Decls["border"] != "1px solid var(--color-border)" ||
			r.Decls["border-radius"] != "8px" {
			t.Errorf("%s must be a white card with the hairline and 8px corners, got %v", head, r.Decls)
		}
	}
	// Departure (quick task 261013-wm5): the chip holds the top right corner of
	// the card, so the state word sits right after the tag with no auto margin.
	if got := find("main.profile-page .profile-report-list .report-row .visibility-tag:not([hidden])").Decls["margin"]; got != "0" {
		t.Errorf("state word margin is %q, want 0 so it sits right after the tag", got)
	}

	// The six tab strip (quick task 261013-wm5): a grey track that clips and
	// scrolls inside itself, with 44px tabs and a white selected tab.
	strip := find("main.profile-page > .profile-activity > .activity-tabs")
	for prop, want := range map[string]string{
		"grid-area":             "tabs",
		"min-width":             "0",
		"overflow-x":            "auto",
		"overscroll-behavior-x": "contain",
		"background":            "var(--color-surface)",
		"display":               "flex",
		"border-radius":         "8px",
	} {
		if got := strip.Decls[prop]; got != want {
			t.Errorf("tab strip %s is %q, want %q", prop, got, want)
		}
	}
	tab := find("main.profile-page .activity-tabs .activity-tab")
	if tab.Decls["min-height"] != "var(--touch-target-min)" || tab.Decls["flex"] != "none" || tab.Decls["white-space"] != "nowrap" {
		t.Errorf("a tab must be 44px tall, never shrink and never wrap, got %v", tab.Decls)
	}
	if got := find("main.profile-page .activity-tabs .activity-tab[aria-current=\"page\"]").Decls["background"]; got != "var(--color-bg)" {
		t.Errorf("selected tab background is %q, want the page colour on the surface track", got)
	}
	if got := find("main.profile-page > .profile-activity > .activity-empty").Decls["grid-area"]; got != "empty" {
		t.Errorf("empty states grid-area is %q, want empty", got)
	}
	if got := find("main.profile-page .profile-activity > .activity-subtitle").Decls["display"]; got != "none" {
		t.Errorf("the help line display is %q on phones, want none", got)
	}
	icon := find("main.profile-page .profile-report-list .report-row > .icon-badge")
	if icon.Decls["width"] != "48px" || icon.Decls["height"] != "48px" || icon.Decls["border-radius"] != "50%" {
		t.Errorf("the phone event icon must be a 48px circle, got %v", icon.Decls)
	}
	if got := find("main.profile-page .profile-report-list .report-row .activity-card__action").Decls["display"]; got != "none" {
		t.Errorf("the action phrase display is %q on phones, want none (the tag says the same)", got)
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

	popup := must(".leaflet-container .leaflet-detail-pane .leaflet-popup")
	for prop, want := range map[string]string{
		"position": "fixed", "top": "calc(var(--space-3xl) * 3)", "right": "0",
		"overflow-y": "auto", "overscroll-behavior": "contain", "touch-action": "pan-y",
		"left": "0 !important", "bottom": "0 !important", "transform": "none !important",
	} {
		if got := declared(popup, prop); got != want {
			t.Errorf("sheet %s is %q, want %q", prop, got, want)
		}
	}

	content := must(".leaflet-container .leaflet-detail-pane .leaflet-popup-content")
	if got := declared(content, "width"); got != "auto !important" {
		t.Errorf("sheet content width is %q, want auto !important (Leaflet writes it inline)", got)
	}
	if got := declared(content, "max-height"); got != "none" {
		t.Errorf("sheet content max-height is %q, want none", got)
	}

	short := byHead("@media (max-width: 899.98px) and (max-height: 359px)", ".leaflet-container .leaflet-detail-pane .leaflet-popup")
	if len(short) != 1 || short[0].Decls["top"] != "var(--space-3xl)" {
		t.Errorf("short screen block must set the strip to var(--space-3xl), got %v", short)
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
			strings.Contains(r.Head, ".leaflet-control-zoom") && strings.Contains(r.Head, "> .theme-toggle") {
			detailMode = true
		}
	}
	if !detailMode {
		t.Errorf("phone.css has no detail mode rule hiding the zoom control and the theme toggle")
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
	p13Ordered(t, initBody, "init",
		"map = L.map(container,",
		"map.createPane('detailPane', container)",
		"L.DomEvent.disableClickPropagation(detailPane)",
		"L.DomEvent.disableScrollPropagation(detailPane)",
		"if (hasVectorBasemap())")

	place := jsFunctionBody(t, code, "applyPopupPlacement(popup)")
	p13MustContain(t, "applyPopupPlacement", place,
		"popup.options.pane = phone ? 'detailPane' : 'popupPane'",
		"popup.options.autoPan = !reducedMotion && !phone")

	upsert := jsFunctionBody(t, code, "upsertMarker(report)")
	p13Ordered(t, upsert, "upsertMarker", "marker.bindPopup(", "applyPopupPlacement(marker.getPopup());", "marker.on('click'")
	p13MustContain(t, "upsertMarker bind literal", upsert, "autoPan: !reducedMotion", "keepInView: true")

	resize := jsFunctionBody(t, code, "onMapResize()")
	p13Ordered(t, resize, "onMapResize",
		"var narrow = phoneDetail();", "var crossed = narrow !== lastNarrow;", "lastNarrow = narrow;",
		"map.closePopup();", "applyPopupPlacement(markers[ids[i]].getPopup());")
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
	p13Ordered(t, strip, "centerPinInStrip",
		"getBoundingClientRect().top", "map.setView(popup.getLatLng(), Math.max(map.getZoom(), 16), { animate: false });",
		"map.panBy([0, Math.round(map.getSize().y / 2 - strip / 2)], { animate: false });")

	// The parts the sheet must leave alone.
	closeBody := jsFunctionBody(t, code, "onPopupClose(evt)")
	p13MustNotContain(t, "onPopupClose", closeBody, "phoneDetail", "detailPane")
	returnFocus := jsFunctionBody(t, code, "returnFocusToOpener(opener)")
	p13MustNotContain(t, "returnFocusToOpener", returnFocus, "phoneDetail", "detailPane")
	fly := jsFunctionBody(t, code, "flyTo(id, opts)")
	p13MustNotContain(t, "flyTo", fly, "phoneDetail", "detailPane")
}
