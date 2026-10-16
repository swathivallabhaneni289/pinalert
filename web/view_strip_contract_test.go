package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds the static contract tests of plan 03.1-05, the tab strip
// (Active, Disputed, Past), the view store and the list pane markup. Every
// helper and test name carries the ViewStrip or vs prefix so a later plan can
// add its own without a clash. Nothing here needs a database or a server: the
// tests read the embedded static and template trees and inspect the shipped
// text, like the other contract files.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-13, D-14,
// D-18): the tab strip, the heading and the Past groups are gone from the list
// pane, and the Show button with its menu took their place. The tests that
// pinned the strip keep their ideas on the new targets: the list head markup,
// the address that nothing writes, the page restore, the empty blocks and
// their words, the render and focus order, the status line, and the 44px
// touch target, the 8px corner and the contrast of the Show button and menu.
// The store test is amended in place. The Phase 3.1 and Phase 4 hands-on tests
// are walked on this layout after Phase 3.4 is built.

// vsListPane returns the markup of the list pane, from its opening section to
// its closing tag, so a test never reads the map pane or the report modal.
func vsListPane(t *testing.T) string {
	t.Helper()
	index := p13ReadTemplate(t, "index.html.tmpl")
	start := strings.Index(index, `<section class="pane pane--list">`)
	if start == -1 {
		t.Fatal("index.html.tmpl has no list pane section")
	}
	// The pane holds nested sections (the Past groups), so it ends where the
	// report modal begins.
	//
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-23, D-24,
	// D-24a): the report window's backdrop carries a second class, post-window,
	// so the search text names the whole class attribute. The Phase 3.1 and
	// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
	end := strings.Index(index[start:], `<div class="modal-backdrop post-window"`)
	if end == -1 {
		t.Fatal("could not find the end of the list pane")
	}
	return index[start : start+end]
}

// TestViewStripMarkup pins the list head: no tab strip, no heading and no
// checkbox in the list pane; the banner slot, then the count line and the Show
// button with its menu, then the polite status line, then the list. The bottom
// bar still comes from the one app_bottombar block.
func TestViewStripMarkup(t *testing.T) {
	pane := vsListPane(t)

	for _, gone := range []string{`class="view-tabs"`, `class="feed-heading"`, `id="feed-note"`, `id="past-groups"`, `type="checkbox"`} {
		if strings.Contains(pane, gone) {
			t.Errorf("the list pane still holds %q", gone)
		}
	}

	p13Ordered(t, pane, "list pane",
		`id="alert-banner"`, `class="list-head"`, `id="feed-count"`, `id="show-button"`, `id="show-menu"`,
		`id="feed-status"`, `id="place-line"`, `id="filter-line"`, `id="queue-strip"`, `id="feed-stale"`,
		`id="report-list"`)

	// The banner slot: a link to /alerts, hidden, no data until Phase 5.
	banner := vsBlock(t, pane, `id="alert-banner"`)
	banner = banner[:strings.Index(banner, ">")]
	p13MustContain(t, "#alert-banner", banner, `href="/alerts"`, " hidden", `class="alert-banner"`)

	// The Show button: a real button that opens a menu, named for what it holds.
	button := vsBlock(t, pane, `id="show-button"`)
	p13MustContain(t, "#show-button", button,
		`type="button"`, `aria-haspopup="menu"`, `aria-expanded="false"`, `aria-controls="show-menu"`, ">Show: All<")

	// The menu: three radio items in the order All, Confirmed, Disputed, the
	// first checked, with no line of explanation under any of them (D-05) and
	// no Only Reliable reporters tick until Phase 3.2.
	menu := vsBlock(t, pane, `id="show-menu"`)
	p13MustContain(t, "#show-menu", menu, `role="menu"`, " hidden")
	items := regexp.MustCompile(`<button type="button" class="show-menu__item" role="menuitemradio" aria-checked="(true|false)" data-show="([a-z]+)">`).FindAllStringSubmatch(menu, -1)
	want := [][2]string{{"true", "all"}, {"false", "confirmed"}, {"false", "disputed"}}
	if len(items) != len(want) {
		t.Fatalf("the Show menu holds %d radio items, want %d", len(items), len(want))
	}
	for i, w := range want {
		if items[i][1] != w[0] || items[i][2] != w[1] {
			t.Errorf("menu item %d is %v, want checked %q and show %q", i, items[i][1:], w[0], w[1])
		}
	}
	p13Ordered(t, menu, "#show-menu", ">All<", ">Confirmed<", ">Disputed<")
	p13MustNotContain(t, "#show-menu", menu, `type="checkbox"`, "Only Reliable reporters", "<p", "<small")

	index := p13ReadTemplate(t, "index.html.tmpl")
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10):
	// the phone bottom bar is the four item app bar, drawn from the one list of
	// places by the app_bottombar block (a range over .Nav.Items plus the Report
	// link), so the page includes the block instead of holding two tabs.
	if n := strings.Count(index, `{{template "app_bottombar" .}}`); n != 1 {
		t.Errorf("index.html.tmpl includes the bottom bar %d times, want exactly 1", n)
	}
	if strings.Contains(index, "bottom-nav") {
		t.Errorf("index.html.tmpl still holds the old two tab bottom nav")
	}

	// The scripts: the menu and the card builders load before feed.js, after
	// app.js and the position module.
	p13Ordered(t, index, "index.html.tmpl scripts",
		"/static/js/app.js", "/static/js/position.js", "/static/js/map.js", "/static/js/ended-cards.js",
		"/static/js/show-menu.js", "/static/js/feed.js")
	for _, script := range []string{"position.js", "ended-cards.js", "show-menu.js"} {
		if n := strings.Count(index, "/static/js/"+script); n != 1 {
			t.Errorf("index.html.tmpl loads %s %d times, want once", script, n)
		}
	}
}

// TestViewStripWritesNoAddress pins D-04 in its Phase 3.4 form: nothing writes
// the Show choice into the address or the history. The default view word is
// never sent, the only words sent are all and disputed, and feed.js and
// show-menu.js never touch the history or navigate (the one navigation is the
// replace of a finished report's link, pinned in TestFocusLeavesNearby...).
func TestViewStripWritesNoAddress(t *testing.T) {
	feed := p19Code(t, "static/js/feed.js")
	menu := p19Code(t, "static/js/show-menu.js")
	for name, text := range map[string]string{"feed.js": feed, "show-menu.js": menu} {
		for _, banned := range []string{"history.", "pushState", "replaceState", "searchParams.set(", "searchParams.delete(", "location.assign", "location.href ="} {
			if strings.Contains(text, banned) {
				t.Errorf("%s contains %q: the Show choice is written into no address", name, banned)
			}
		}
	}

	banned := regexp.MustCompile(`view=active|VIEW_PARAM,\s*'active'|set\('view',\s*'active'\)`)
	app := p19Code(t, "static/js/app.js")
	index := p13ReadTemplate(t, "index.html.tmpl")
	for name, text := range map[string]string{"feed.js": feed, "show-menu.js": menu, "app.js": app, "index.html.tmpl": index} {
		if m := banned.FindString(text); m != "" {
			t.Errorf("%s writes the default view word as a parameter value: %q", name, m)
		}
	}

	// The menu reads the address once, as an alias, in one function.
	read := jsFunctionBody(t, menu, "readAlias(search)")
	p13MustContain(t, "readAlias", read,
		"new URLSearchParams(", "params.get('view')", "word === 'disputed'",
		"params.get('show_disputed') === 'true'", "return 'all';")
	p13MustNotContain(t, "readAlias", read, "innerHTML", "eval(")
}

// TestViewStripPageRestore pins D-05 and the back forward cache fix: a restored
// page hands the store's Show choice back to the store and the menu before it
// refetches, the report parameter is never removed from the address, and the
// boot step hands the store its Show choice before the first request.
func TestViewStripPageRestore(t *testing.T) {
	feed := p19Code(t, "static/js/feed.js")

	listener, anchorFound, terminatorFound := windowAfter(feed, "addEventListener('pageshow'", "});")
	if !anchorFound || !terminatorFound {
		t.Fatalf("could not bound the page restore listener in feed.js")
	}
	p13Ordered(t, listener, "page restore listener",
		"evt.persisted", "Pinalert.setShow(Pinalert.state.show);", "PinalertShowMenu.sync();", "Pinalert.fetchReports();")

	if strings.Contains(feed, "delete(REPORT_PARAM)") || strings.Contains(feed, "delete('report')") {
		t.Errorf("feed.js removes the report parameter from the address, a report link stays as it was sent")
	}
	p13MustContain(t, "feed.js", feed, "var REPORT_PARAM = 'report';")
	p13Ordered(t, feed, "feed.js load",
		"Pinalert.setShow(Pinalert.state.show);", "Pinalert.setFocusId(new URLSearchParams(window.location.search).get(REPORT_PARAM));",
		"Pinalert.subscribe(render);")
}

// TestViewStripStoreContract pins the store the strip and every later screen
// plug into: one view, one category, one place, one link id, the new request
// parameters on the one list fetch, the staleness rules, and the poll.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-13, D-14,
// D-18, D-24a): the store's views are active (the default, which the Low data
// view keeps), all and disputed; the Past word is gone, so the poll never
// skips; the chip list swaps Rescue for Traffic; and the Show choice lives in
// state.show beside setShow, reportInShow and visibleReports. The Phase 3.1
// and Phase 4 hands-on tests are walked on this layout after Phase 3.4 is
// built.
func TestViewStripStoreContract(t *testing.T) {
	app := p19Code(t, "static/js/app.js")

	if n := strings.Count(app, "fetch("); n != 2 {
		t.Errorf("app.js holds %d fetch( calls, want exactly 2 (the one list request and the submit)", n)
	}
	feed := p19Code(t, "static/js/feed.js")
	if n := strings.Count(feed, "fetch("); n != 0 {
		t.Errorf("feed.js holds %d fetch( calls, want 0", n)
	}

	p13MustContain(t, "app.js state", app,
		"view: 'active',", "show: 'all',", "category: '',", "place: null,", "focusId: null,", "focus: null,",
		"recent: null,", "cut: null",
		"var VIEWS = ['active', 'all', 'disputed'];", "var SHOWS = ['all', 'confirmed', 'disputed'];")
	p13MustNotContain(t, "app.js", app, "'past'")

	fetchBody := jsFunctionBody(t, app, "fetchReports(options)")
	p13Ordered(t, fetchBody, "fetchReports request",
		"var sentView = state.view;", "var sentCategory = state.category;",
		"var sentLat = state.center.lat;", "var sentLon = state.center.lon;",
		"var sentFocusId = state.focusId;",
		"if (sentView === 'all' || sentView === 'disputed') {",
		"url += '&view=' + sentView;",
		"if (sentCategory !== '') {",
		"url += '&category=' + encodeURIComponent(sentCategory);",
		"if (sentFocusId !== null) {",
		"url += '&report=' + encodeURIComponent(sentFocusId);",
		"var answered = fetch(url)")
	if n := strings.Count(fetchBody, "'&view='"); n != 1 {
		t.Errorf("fetchReports builds the view parameter in %d places, want 1", n)
	}
	// A parameter is added only for a non default value, so the Active,
	// unfiltered URL is the Phase 1 URL byte for byte.
	p13MustNotContain(t, "fetchReports", fetchBody, "'&view=active'", "'&category=' + sentCategory")

	// The response keeps the three optional objects without rendering them.
	// The link id is kept and sent again while the link is held; it is
	// cleared only when an answer says the report cannot be shown (too old,
	// unavailable, or no ok focus for the id that request carried).
	p13Ordered(t, fetchBody, "fetchReports response",
		"state.reports = (body && body.reports) || [];",
		"state.recent = (body && body.recent) || null;",
		"state.cut = (body && body.cut) || null;",
		"if (body && body.focus) {",
		"state.focus = body.focus;",
		"state.focusId = null;",
		"rowsFor = { view: sentView, category: sentCategory, lat: sentLat, lon: sentLon };",
		"lastAppliedSeq = mySeq;")

	// A response is dropped when the tab, the chip or the centre it was sent
	// with is no longer the store's, on the success path and the failure path.
	p13MustContain(t, "isStale", fetchBody,
		"sentView !== state.view", "sentCategory !== state.category",
		"sentLat !== state.center.lat", "sentLon !== state.center.lon",
		"mySeq < lastAppliedSeq", "mySeq < writeFloor")
	p13Ordered(t, fetchBody, "failure path",
		".catch(function (err) {", "if (!rowsMatchStore()) {", "state.reports = [];", "state.status = 'error';")

	// The poll never skips: the Past tab is gone from Nearby, so every view is
	// refreshed on the timer.
	poll := jsFunctionBody(t, app, "startPolling()")
	p13Ordered(t, poll, "startPolling", "window.setInterval(function () {", "fetchReports();")
	p13MustNotContain(t, "startPolling", poll, "state.view", "'past'")

	// The setters assign only (no notify, no fetch) and ignore a bad value.
	for _, name := range []string{"setView(view)", "setShow(show)", "setCategory(slug)", "setPlace(place)", "clearPlace()", "setFocusId(id)"} {
		body := jsFunctionBody(t, app, name)
		p13MustNotContain(t, name, body, "notify(", "fetchReports(", "fetch(")
	}
	p13MustContain(t, "setView", jsFunctionBody(t, app, "setView(view)"),
		"VIEWS.indexOf(view) !== -1", "state.view = view;")
	p13MustContain(t, "setCategory", jsFunctionBody(t, app, "setCategory(slug)"),
		"slug === ''", "KNOWN_CATEGORIES.indexOf(slug) !== -1", "state.category = slug;")
	p13MustContain(t, "setPlace", jsFunctionBody(t, app, "setPlace(place)"),
		"PLACE_NAME_MAX", "'\\u2026'", "state.place = { name: name, lat: place.lat, lon: place.lon };")
	p13MustContain(t, "app.js", app, "var PLACE_NAME_MAX = 32;")

	p13MustContain(t, "CHIP_LABELS", app,
		"flood: 'Flood',", "traffic: 'Traffic',", "shelter_open: 'Shelter',", "power_outage: 'Power'")
	chips, chipsAnchor, chipsEnd := windowAfter(app, "var CHIP_LABELS = {", "};")
	if !chipsAnchor || !chipsEnd {
		t.Fatalf("could not bound the CHIP_LABELS table")
	}
	p13MustNotContain(t, "CHIP_LABELS", chips, "rescue_needed")

	// The Show choice: the words, the one membership rule and the one visible
	// set, all reading the server's two words only (D-13, D-14, TRUST-02).
	p13MustContain(t, "setShow", jsFunctionBody(t, app, "setShow(show)"),
		"SHOWS.indexOf(show) === -1", "state.show = show;", "show === 'disputed' ? 'disputed' : 'all'")
	member := jsFunctionBody(t, app, "reportInShow(report, show)")
	p13MustContain(t, "reportInShow", member,
		"report.confirmed_by_people === true", "report.view !== 'disputed'", "report.view === 'disputed'")
	p13MustNotContain(t, "reportInShow", member, "confirmed_places", "disputed_places", "trust.", ".length", ">=", "<=")
	p13MustContain(t, "visibleReports", jsFunctionBody(t, app, "visibleReports()"),
		"reportInShow(state.reports[i], state.show)")
	p13MustContain(t, "disputedLast", jsFunctionBody(t, app, "disputedLast(reports)"),
		"reports[i].view === 'disputed'", "live.concat(disputed)")

	// The distance sentence: the browser formats, the server owns the number.
	dist := jsFunctionBody(t, app, "distanceText(report)")
	p13MustContain(t, "distanceText", dist,
		"report.approximate === true", "Math.max(1, Math.round(km))",
		"'about ' + whole + ' km from ' + place.name", "' km away'",
		"'under 100 m from ' + place.name", "' km from ' + place.name",
		"state.center.own === true", "formatDistance(km)", "return '';")

	exports := []string{
		"CHIP_LABELS: CHIP_LABELS,", "VIEWS: VIEWS,", "SHOWS: SHOWS,", "setView: setView,", "setShow: setShow,",
		"reportInShow: reportInShow,", "visibleReports: visibleReports,", "disputedLast: disputedLast,", "setCategory: setCategory,",
		"setPlace: setPlace,", "clearPlace: clearPlace,", "setFocusId: setFocusId,",
		"distanceText: distanceText,",
	}
	p13MustContain(t, "app.js exports", app, exports...)
	if strings.Contains(app, "setShowDisputed") {
		t.Errorf("app.js still exports the old boolean setter")
	}
}

// vsBlock returns the markup of one element of the list pane, from the tag
// that carries the given attribute text to the closing tag of the same
// element (it counts nested tags of the same name).
func vsBlock(t *testing.T, pane, openAttr string) string {
	t.Helper()
	at := strings.Index(pane, openAttr)
	if at == -1 {
		t.Fatalf("the list pane has no element with %q", openAttr)
	}
	start := strings.LastIndex(pane[:at], "<")
	nameEnd := strings.IndexAny(pane[start+1:], " >")
	name := pane[start+1 : start+1+nameEnd]
	depth := 0
	for i := start; i < len(pane); i++ {
		switch {
		case strings.HasPrefix(pane[i:], "</"+name):
			depth--
			if depth == 0 {
				return pane[start : i+len(name)+3]
			}
		case strings.HasPrefix(pane[i:], "<"+name+" ") || strings.HasPrefix(pane[i:], "<"+name+">"):
			depth++
		}
	}
	t.Fatalf("the element with %q is not closed", openAttr)
	return ""
}

// vsRule returns the one rule of rules whose head equals head and whose media
// contains media (an empty media means top level only), failing the test when
// there is none.
func vsRule(t *testing.T, rules []p13CSSRule, media, head string) p13CSSRule {
	t.Helper()
	r, _ := p13Rule(t, rules, media, head)
	return r
}

// TestViewStripEmptyBlocks pins the list pane markup of the states: the status
// line, the place line, the filter line, the five empty blocks and the error
// block, each hidden at first, in the order of the UI spec, with the spec's
// words (P34-C10).
func TestViewStripEmptyBlocks(t *testing.T) {
	pane := vsListPane(t)

	p13Ordered(t, pane, "list pane",
		`id="feed-status"`, `id="place-line"`, `id="filter-line"`, `id="report-list"`,
		`id="feed-skeleton"`, `id="feed-empty"`, `id="feed-empty-history"`, `id="confirmed-empty"`,
		`id="disputed-empty"`, `id="filter-empty"`, `id="feed-error"`)

	for _, id := range []string{
		"feed-status", "place-line", "filter-line", "feed-empty", "feed-empty-history", "confirmed-empty",
		"disputed-empty", "filter-empty", "feed-error", "see-ended-reports", "filter-empty-clear",
		"show-button", "show-menu", "feed-count", "alert-banner",
	} {
		if n := strings.Count(pane, `id="`+id+`"`); n != 1 {
			t.Errorf("the list pane holds id %q %d times, want exactly once", id, n)
		}
	}

	status := vsBlock(t, pane, `id="feed-status"`)
	p13MustContain(t, "#feed-status", status, `class="visually-hidden"`, `role="status"`)

	// Each of these ships hidden: a block nobody asked for is never drawn.
	for _, id := range []string{
		"place-line", "filter-line", "feed-empty", "feed-empty-history", "confirmed-empty",
		"disputed-empty", "filter-empty", "feed-error", "show-menu", "alert-banner",
	} {
		block := vsBlock(t, pane, `id="`+id+`"`)
		open := block[:strings.Index(block, ">")]
		if !strings.Contains(open, " hidden") {
			t.Errorf("#%s must ship the hidden attribute, opening tag: %q", id, open)
		}
	}

	words := []struct{ id, heading, body, button string }{
		{"feed-empty", "No reports near you yet", "Nothing has been reported in your area recently. Post a report to share what you're seeing.", ""},
		{"feed-empty-history", "No active reports nearby", "Post a report to share what you're seeing.", "See ended reports"},
		{"confirmed-empty", "No confirmed reports nearby", "Reports show here once people nearby confirm them.", ""},
		{"disputed-empty", "No disputed reports nearby", "Reports only show up here if enough nearby people have disputed them.", ""},
		{"filter-empty", "Nothing matches your filters", "Try another category or clear the filter.", "Clear filter"},
	}
	for _, w := range words {
		block := vsBlock(t, pane, `id="`+w.id+`"`)
		if !strings.Contains(block, `class="empty-state"`) {
			t.Errorf("#%s must carry the empty-state class", w.id)
		}
		p13MustContain(t, "#"+w.id, block, "<h2>"+w.heading+"</h2>", w.body)
		switch {
		case w.id == "feed-empty-history":
			// The one link of the empty blocks goes to My activity, Ended nearby.
			p13MustContain(t, "#"+w.id, block, `<a class="btn" id="see-ended-reports" href="/profile?part=ended">See ended reports</a>`)
		case w.button != "":
			p13MustContain(t, "#"+w.id, block, `<button type="button" class="btn"`, ">"+w.button+"</button>")
		case strings.Contains(block, "<button"):
			t.Errorf("#%s holds a button, it has none in the spec", w.id)
		}
	}
	p13MustNotContain(t, "list pane", pane, "Tap + to report", "See past reports")
}

// TestViewStripPaneScript pins the render of feed.js under the Show choice: the
// count line words (P34-C6), the empty block chosen in the order filter,
// disputed, confirmed, history, general, exactly one of the empty blocks
// showing, the three focus fallbacks, and the Clear filter button.
func TestViewStripPaneScript(t *testing.T) {
	feed := p19Code(t, "static/js/feed.js")

	count := jsFunctionBody(t, feed, "updateFeedCount(state)")
	p13MustContain(t, "updateFeedCount", count,
		"Pinalert.visibleReports().length", "' report, '", "' reports, '", "Pinalert.setText(feedCountEl, text);")
	p13MustNotContain(t, "updateFeedCount", count, "innerHTML", "insertAdjacentHTML", "\\u00b7")

	empty := jsFunctionBody(t, feed, "setEmptyState(kind)")
	for _, pair := range []string{
		"setHidden(emptyEl, kind !== 'general');", "setHidden(historyEmptyEl, kind !== 'history');",
		"setHidden(confirmedEmptyEl, kind !== 'confirmed');", "setHidden(disputedEmptyEl, kind !== 'disputed');",
		"setHidden(filterEmptyEl, kind !== 'filter');",
	} {
		if n := strings.Count(feed, pair); n != 1 {
			t.Errorf("feed.js holds %q %d times, want once inside setEmptyState", pair, n)
		}
		if !strings.Contains(empty, pair) {
			t.Errorf("setEmptyState lacks %q", pair)
		}
	}
	if n := strings.Count(feed, "setEmptyState('history')"); n != 0 {
		t.Errorf("the history empty block is chosen through render's kind variable (plan 03.1-12), never by a literal call")
	}

	render := jsFunctionBody(t, feed, "render(state)")
	p13Ordered(t, render, "render",
		"var visible = Pinalert.visibleReports();", "updateFeedStatus(state);",
		"if (visible.length === 0) {", "Pinalert.state.category !== ''", "kind = 'filter';",
		"Pinalert.state.show === 'disputed'", "kind = 'disputed';",
		"Pinalert.state.show === 'confirmed'", "kind = 'confirmed';",
		"kind = 'history';", "setEmptyState(kind);", "renderRows(visible);")
	p13MustNotContain(t, "render", render, "state.reports.length === 0", "renderRows(state.reports)")

	p13MustContain(t, "feed.js", feed,
		"filterClearBtn.addEventListener('click', function () {", "Pinalert.setCategory('');")

	// Focus falls back to the map first and then to the Show button.
	after := jsFunctionBody(t, feed, "focusAfterRemoval(candidates)")
	p13Ordered(t, after, "focusAfterRemoval",
		"getElementById('map')", "showButton()", "viewToggleBtn.focus()", "var empties = [")
}

// TestViewStripStatusAnnouncement pins UI-SPEC 2i: one sentence in the visually
// hidden live region per Show, place or chip change, built from the store's
// own numbers and words, written through setText, never repeated for a poll.
func TestViewStripStatusAnnouncement(t *testing.T) {
	feed := p19Code(t, "static/js/feed.js")
	body := jsFunctionBody(t, feed, "updateFeedStatus(state)")

	p13Ordered(t, body, "updateFeedStatus",
		"state.status !== 'ok'",
		"var key = store.show + '|' + placeName + '|' + store.category;",
		"if (key === lastAnnouncedKey) {", "lastAnnouncedKey = key;",
		"if (!statusStarted) {",
		"var count = Pinalert.visibleReports().length;",
		"(count === 1 ? ' report' : ' reports')",
		"SHOW_STATUS_WORDS[store.show]",
		"text += '.';",
		"text += ' Near ' + placeName + '.';",
		"text += ' Filtered by '",
		"Pinalert.setText(feedStatusEl, text);")
	p13MustContain(t, "updateFeedStatus", body, "Pinalert.CHIP_LABELS[store.category]")
	p13MustNotContain(t, "updateFeedStatus", body, "innerHTML", "insertAdjacentHTML")
	p13MustContain(t, "feed.js", feed, "var SHOW_STATUS_WORDS = { confirmed: 'Show Confirmed', disputed: 'Show Disputed' };")
	if n := strings.Count(feed, "Pinalert.setText(feedStatusEl,"); n != 1 {
		t.Errorf("the status line is written in %d places, want exactly 1", n)
	}
	// The count of reports is not part of the key, so a poll that only changes
	// the count never speaks again.
	if strings.Contains(body, "reports.length + '|'") || strings.Contains(body, "visibleReports().length + '|'") {
		t.Errorf("the announcement key must not hold the count")
	}
}

// TestViewStripNoPillAndTouchTarget pins the look of the Show button and menu
// (UI-SPEC 2c): a 44px target and an 8px corner on the button and on every menu
// row, never a pill, the 220px menu above the list on its own layer with the
// menu shadow, the check kept in place so the words line up, the inline text
// button, tokens only, and the checkbox rule gone.
func TestViewStripNoPillAndTouchTarget(t *testing.T) {
	rules := p13ParseCSS(p13ReadStatic(t, "static/css/feed.css"))

	button := vsRule(t, rules, "", ".show-button")
	for prop, want := range map[string]string{
		"min-height":    "var(--touch-target-min)",
		"border-radius": "8px",
		"border":        "1px solid var(--color-border)",
		"color":         "var(--color-text)",
		"font-size":     "var(--font-size-label)",
	} {
		if got := button.Decls[prop]; got != want {
			t.Errorf(".show-button %s is %q, want %q", prop, got, want)
		}
	}
	if r := button.Decls["border-radius"]; r == "999px" || strings.Contains(r, "%") {
		t.Errorf(".show-button corner %q is a pill", r)
	}

	head := vsRule(t, rules, "", ".list-head")
	for prop, want := range map[string]string{
		"display":       "flex",
		"padding":       "var(--space-sm) var(--space-md)",
		"border-bottom": "1px solid var(--color-border)",
	} {
		if got := head.Decls[prop]; got != want {
			t.Errorf(".list-head %s is %q, want %q", prop, got, want)
		}
	}

	menu := vsRule(t, rules, "", ".show-menu")
	for prop, want := range map[string]string{
		"position":      "absolute",
		"z-index":       "1060",
		"min-width":     "220px",
		"box-shadow":    "var(--shadow-menu)",
		"border-radius": "8px",
	} {
		if got := menu.Decls[prop]; got != want {
			t.Errorf(".show-menu %s is %q, want %q", prop, got, want)
		}
	}

	item := vsRule(t, rules, "", ".show-menu__item")
	if got := item.Decls["min-height"]; got != "var(--touch-target-min)" {
		t.Errorf(".show-menu__item min-height is %q, want the 44px token", got)
	}
	if got := vsRule(t, rules, "", ".show-menu__item:focus-visible").Decls["outline-offset"]; got != "-2px" {
		t.Errorf(".show-menu__item:focus-visible outline-offset is %q, want -2px", got)
	}
	// The check keeps its place on every row and shows only on the chosen one.
	if got := vsRule(t, rules, "", ".show-menu__check").Decls["visibility"]; got != "hidden" {
		t.Errorf(".show-menu__check visibility is %q, want hidden", got)
	}
	if got := vsRule(t, rules, "", `.show-menu__item[aria-checked="true"] .show-menu__check`).Decls["visibility"]; got != "visible" {
		t.Errorf("the chosen row's check visibility is %q, want visible", got)
	}
	if got := vsRule(t, rules, "", `.show-menu[hidden]`).Decls["display"]; got != "none" {
		t.Errorf(".show-menu[hidden] display is %q, want none", got)
	}
	forced := vsRule(t, rules, "forced-colors", ".show-button, .show-menu")
	if got := forced.Decls["border-color"]; got != "CanvasText" {
		t.Errorf("the forced colours border of the Show control is %q, want CanvasText", got)
	}

	inline := vsRule(t, rules, "", ".inline-text-button")
	if got := inline.Decls["min-height"]; got != "var(--touch-target-min)" {
		t.Errorf(".inline-text-button min-height is %q, want the 44px token", got)
	}
	if got := inline.Decls["border-radius"]; got != "8px" {
		t.Errorf(".inline-text-button corner is %q, want 8px", got)
	}

	// Tokens only: no colour literal in any declaration of feed.css.
	literal := regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$|rgb\(|rgba\(|hsl\(`)
	for _, r := range rules {
		for prop, value := range r.Decls {
			if literal.MatchString(strings.TrimSpace(value)) {
				t.Errorf("feed.css rule %q sets %s to the colour literal %q", r.Head, prop, value)
			}
		}
	}

	// The checkbox label rule is gone from trust.css.
	trust := p13ReadStatic(t, "static/css/trust.css")
	if strings.Contains(trust, ".filter-toggle") {
		t.Errorf("trust.css still holds the .filter-toggle rule of the removed checkbox")
	}
}

// TestViewStripContrast computes the text pairs the list head and the disputed
// cues introduce, from the shipped tokens in all four theme sources: the Show
// button and menu text on the page background, the count line (muted) on the
// page background, and the bold disputed line on the page background.
func TestViewStripContrast(t *testing.T) {
	needed := []string{"--color-bg", "--color-surface", "--color-text", "--color-text-muted", "--color-severity-critical"}
	light1, light2, dark1, dark2 := extractThemeTokens(parseCSSRules(p13ReadStatic(t, "static/css/main.css")), needed)
	sources := []struct {
		name   string
		tokens themeTokens
	}{
		{":root", light1}, {`:root[data-theme="light"]`, light2},
		{"prefers dark :root", dark1}, {`:root[data-theme="dark"]`, dark2},
	}
	pairs := []struct{ name, fg, bg string }{
		{"Show button text on the page background", "--color-text", "--color-bg"},
		{"menu text on the page background", "--color-text", "--color-bg"},
		{"the count line on the page background", "--color-text-muted", "--color-bg"},
		{"the disputed line on the page background", "--color-severity-critical", "--color-bg"},
	}
	for _, s := range sources {
		for _, p := range pairs {
			fg := p31Resolve(t, s.tokens, p.fg)
			bg := p31Resolve(t, s.tokens, p.bg)
			if ratio := wcagContrastRatio(t, fg, bg, s.name+" "+p.name); ratio < 4.5 {
				t.Errorf("%s: %s is %.2f:1, want at least 4.5:1", s.name, p.name, ratio)
			}
		}
	}
}
