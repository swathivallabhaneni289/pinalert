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
	end := strings.Index(index[start:], `<div class="modal-backdrop"`)
	if end == -1 {
		t.Fatal("could not find the end of the list pane")
	}
	return index[start : start+end]
}

// TestViewStripMarkup pins the strip: one nav named Report views before the
// list, three real links in the order Active, Disputed, Past, no checkbox left
// in the list pane, and a bottom bar that still holds exactly two tabs.
func TestViewStripMarkup(t *testing.T) {
	pane := vsListPane(t)

	if n := strings.Count(pane, `<nav class="view-tabs" aria-label="Report views">`); n != 1 {
		t.Fatalf("the list pane holds %d nav.view-tabs named Report views, want 1", n)
	}
	nav := regexp.MustCompile(`(?s)<nav class="view-tabs"[^>]*>(.*?)</nav>`).FindStringSubmatch(pane)
	if nav == nil {
		t.Fatal("could not read the strip's links")
	}
	links := regexp.MustCompile(`<a class="view-tab"([^>]*)>([^<]*)</a>`).FindAllStringSubmatch(nav[1], -1)
	if len(links) != 3 {
		t.Fatalf("the strip holds %d links, want 3", len(links))
	}
	want := []struct{ href, word, view string }{
		{"/", "Active", "active"},
		{"/?view=disputed", "Disputed", "disputed"},
		{"/?view=past", "Past", "past"},
	}
	for i, w := range want {
		attrs, text := links[i][1], strings.TrimSpace(links[i][2])
		if !strings.Contains(attrs, `href="`+w.href+`"`) || text != w.word ||
			!strings.Contains(attrs, `data-feed-view="`+w.view+`"`) {
			t.Errorf("link %d is %q %q, want href %q, text %q, view %q", i, attrs, text, w.href, w.word, w.view)
		}
		current := strings.Contains(attrs, `aria-current="page"`)
		if current != (i == 0) {
			t.Errorf("link %d aria-current is %v, only the Active link is current in the served markup", i, current)
		}
	}
	if strings.Contains(nav[1], "role=") || strings.Contains(nav[1], "tabindex") {
		t.Errorf("the strip is a plain list of links: no tablist role and no roving tabindex")
	}

	p13Ordered(t, pane, "list pane", `class="view-tabs"`, `id="report-list"`)
	if strings.Contains(pane, `type="checkbox"`) {
		t.Errorf("a checkbox is left in the list pane, the strip replaced the disputed checkbox")
	}

	index := p13ReadTemplate(t, "index.html.tmpl")
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10):
	// the phone bottom bar is the four item app bar, drawn from the one list of
	// places by the app_bottombar block (a range over .Nav.Items plus the Report
	// link), so the page includes the block instead of holding two tabs. The
	// Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
	// Phase 3.4 is built.
	if n := strings.Count(index, `{{template "app_bottombar" .}}`); n != 1 {
		t.Errorf("index.html.tmpl includes the bottom bar %d times, want exactly 1", n)
	}
	if strings.Contains(index, "bottom-nav") {
		t.Errorf("index.html.tmpl still holds the old two tab bottom nav")
	}
}

// TestViewStripNeverWritesDefaultView pins D-04: the address never carries the
// default tab. The parameter is deleted for Active and set only for Disputed
// and Past, and no string in feed.js, app.js or the template writes the
// default word as a parameter value.
func TestViewStripNeverWritesDefaultView(t *testing.T) {
	feed := p19Code(t, "static/js/feed.js")
	sync := jsFunctionBody(t, feed, "syncViewURL(view)")
	p13Ordered(t, sync, "syncViewURL",
		"if (view === 'disputed' || view === 'past') {",
		"url.searchParams.set(VIEW_PARAM, view);",
		"} else {",
		"url.searchParams.delete(VIEW_PARAM);")
	if n := strings.Count(feed, "searchParams.set("); n != 1 {
		t.Errorf("feed.js sets an address parameter in %d places, want exactly the one in syncViewURL", n)
	}

	banned := regexp.MustCompile(`view=active|VIEW_PARAM,\s*'active'|set\('view',\s*'active'\)`)
	app := p19Code(t, "static/js/app.js")
	index := p13ReadTemplate(t, "index.html.tmpl")
	for name, text := range map[string]string{"feed.js": feed, "app.js": app, "index.html.tmpl": index} {
		if m := banned.FindString(text); m != "" {
			t.Errorf("%s writes the default tab word as a parameter value: %q", name, m)
		}
	}
}

// TestViewStripUsesReplaceState pins D-04 and D-05: a tab press and the one
// time rewrite use history.replaceState, the history entry adding variant is
// absent, the old bookmark is rewritten once on load, the page restore
// re-reads the address and refetches, and the report parameter is never
// removed from the address.
func TestViewStripUsesReplaceState(t *testing.T) {
	feed := p19Code(t, "static/js/feed.js")

	if n := strings.Count(feed, "history.replaceState("); n != 1 {
		t.Errorf("feed.js calls history.replaceState in %d places, want exactly 1 (syncViewURL)", n)
	}
	if strings.Contains(feed, "pushState") || strings.Contains(feed, "location.assign") ||
		strings.Contains(feed, "location.href =") {
		t.Errorf("feed.js must not add a history entry or navigate: a tab press adds no Back step")
	}

	switchBody := jsFunctionBody(t, feed, "switchTab(view)")
	p13Ordered(t, switchBody, "switchTab",
		"Pinalert.setView(view);", "showTab(view);", "syncViewURL(view);", "Pinalert.fetchReports();")

	read := jsFunctionBody(t, feed, "readViewFromURL()")
	p13MustContain(t, "readViewFromURL", read,
		"word === 'disputed' || word === 'past'", "word === 'resolved'",
		"params.get(OLD_BOOKMARK_PARAM) === 'true'", "rewrite: true")
	p13Ordered(t, feed, "feed.js load",
		"var initialAddress = readViewFromURL();",
		"Pinalert.setView(initialAddress.view);",
		"if (initialAddress.rewrite) {",
		"syncViewURL(initialAddress.view);")
	if n := strings.Count(feed, "syncViewURL(initialAddress.view)"); n != 1 {
		t.Errorf("the address is rewritten on load in %d places, want exactly once", n)
	}

	listener, anchorFound, terminatorFound := windowAfter(feed, "addEventListener('pageshow'", "});")
	if !anchorFound || !terminatorFound {
		t.Fatalf("could not bound the page restore listener in feed.js")
	}
	p13Ordered(t, listener, "page restore listener",
		"evt.persisted", "readViewFromURL()", "Pinalert.setView(restored.view);", "Pinalert.fetchReports();")

	if strings.Contains(feed, "delete(REPORT_PARAM)") || strings.Contains(feed, "delete('report')") {
		t.Errorf("feed.js removes the report parameter from the address, a report link stays as it was sent")
	}
	p13MustContain(t, "feed.js", feed, "var REPORT_PARAM = 'report';")

	click := regexp.MustCompile(`(?s)tabStripEl\.addEventListener\('click', function \(evt\) \{(.*?)\n    \}\);`).FindStringSubmatch(feed)
	if click == nil {
		t.Fatal("could not find the strip's click handler")
	}
	p13Ordered(t, click[1], "strip click handler",
		"evt.button !== 0", "evt.metaKey", "evt.ctrlKey", "evt.shiftKey", "evt.altKey",
		"evt.preventDefault();", "switchTab(")
}

// TestViewStripStoreContract pins the store the strip and every later screen
// plug into: one view, one category, one place, one link id, the new request
// parameters on the one list fetch, the staleness rules, and the poll skipping
// the Past tab.
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
		"view: 'active',", "category: '',", "place: null,", "focusId: null,", "focus: null,",
		"recent: null,", "cut: null",
		"var VIEWS = ['active', 'disputed', 'past'];")

	fetchBody := jsFunctionBody(t, app, "fetchReports(options)")
	p13Ordered(t, fetchBody, "fetchReports request",
		"var sentView = state.view;", "var sentCategory = state.category;",
		"var sentLat = state.center.lat;", "var sentLon = state.center.lon;",
		"var sentFocusId = state.focusId;",
		"if (sentView === 'disputed' || sentView === 'past') {",
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

	// The poll skips while the Past tab is selected.
	poll := jsFunctionBody(t, app, "startPolling()")
	p13Ordered(t, poll, "startPolling",
		"window.setInterval(function () {", "if (state.view === 'past') {\n        return;\n      }", "fetchReports();")

	// The setters assign only (no notify, no fetch) and ignore a bad value.
	for _, name := range []string{"setView(view)", "setCategory(slug)", "setPlace(place)", "clearPlace()", "setFocusId(id)"} {
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
		"flood: 'Flood',", "rescue_needed: 'Rescue',", "shelter_open: 'Shelter',", "power_outage: 'Power'")

	// The distance sentence: the browser formats, the server owns the number.
	dist := jsFunctionBody(t, app, "distanceText(report)")
	p13MustContain(t, "distanceText", dist,
		"report.approximate === true", "Math.max(1, Math.round(km))",
		"'about ' + whole + ' km from ' + place.name", "' km away'",
		"'under 100 m from ' + place.name", "' km from ' + place.name",
		"state.center.own === true", "formatDistance(km)", "return '';")

	exports := []string{
		"CHIP_LABELS: CHIP_LABELS,", "VIEWS: VIEWS,", "setView: setView,", "setCategory: setCategory,",
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

// TestViewStripEmptyBlocks pins the list pane markup of the whole phase: the
// status line, the note, the place line, the filter line, the Past groups and
// the five empty blocks, each hidden at first, in the order of the UI spec's
// page structure, with the spec's words.
func TestViewStripEmptyBlocks(t *testing.T) {
	pane := vsListPane(t)

	p13Ordered(t, pane, "list pane",
		`class="view-tabs"`, `id="feed-status"`, `class="feed-heading"`, `id="feed-note"`,
		`id="place-line"`, `id="filter-line"`, `id="report-list"`, `id="past-groups"`,
		`id="feed-skeleton"`, `id="feed-empty"`, `id="feed-empty-history"`, `id="disputed-empty"`,
		`id="past-empty"`, `id="filter-empty"`, `id="feed-error"`)

	for _, id := range []string{
		"feed-status", "feed-note", "place-line", "filter-line", "past-groups", "feed-empty",
		"feed-empty-history", "disputed-empty", "past-empty", "filter-empty", "feed-error",
		"past-resolved", "past-expired", "past-resolved-list", "past-expired-list",
		"past-resolved-cut", "past-expired-cut", "feed-heading-title", "see-past-reports",
		"filter-empty-clear",
	} {
		if n := strings.Count(pane, `id="`+id+`"`); n != 1 {
			t.Errorf("the list pane holds id %q %d times, want exactly once", id, n)
		}
	}

	status := vsBlock(t, pane, `id="feed-status"`)
	p13MustContain(t, "#feed-status", status, `class="visually-hidden"`, `role="status"`)

	// Each of these ships hidden: a block nobody asked for is never drawn.
	for _, id := range []string{
		"feed-note", "place-line", "filter-line", "past-groups", "past-resolved", "past-expired",
		"past-resolved-cut", "past-expired-cut", "feed-empty", "feed-empty-history",
		"disputed-empty", "past-empty", "filter-empty", "feed-error",
	} {
		block := vsBlock(t, pane, `id="`+id+`"`)
		open := block[:strings.Index(block, ">")]
		if !strings.Contains(open, " hidden") {
			t.Errorf("#%s must ship the hidden attribute, opening tag: %q", id, open)
		}
	}

	words := []struct{ id, heading, body, button string }{
		{"feed-empty-history", "No active reports nearby", "Tap + to report what you're seeing.", "See past reports"},
		{"past-empty", "No past reports nearby", "Nothing in this area was resolved or faded out in the last 7 days.", ""},
		{"filter-empty", "Nothing matches your filters", "Try another category or clear the filter.", "Clear filter"},
		{"disputed-empty", "No disputed reports nearby", "Reports only show up here if enough nearby people have disputed them.", ""},
		{"feed-empty", "No reports near you yet", "Nothing has been reported in your area recently. Tap + to report what you're seeing.", ""},
	}
	for _, w := range words {
		block := vsBlock(t, pane, `id="`+w.id+`"`)
		if !strings.Contains(block, `class="empty-state"`) {
			t.Errorf("#%s must carry the empty-state class", w.id)
		}
		p13MustContain(t, "#"+w.id, block, "<h2>"+w.heading+"</h2>", w.body)
		if w.button != "" {
			p13MustContain(t, "#"+w.id, block, `<button type="button" class="btn"`, ">"+w.button+"</button>")
		} else if strings.Contains(block, "<button") {
			t.Errorf("#%s holds a button, it has none in the spec", w.id)
		}
	}

	groups := vsBlock(t, pane, `id="past-groups"`)
	p13Ordered(t, groups, "#past-groups",
		`id="past-resolved"`, `<h3 class="past-group__label">Resolved</h3>`, `id="past-resolved-list"`,
		`id="past-resolved-cut"`, `id="past-expired"`, `<h3 class="past-group__label">Expired</h3>`,
		`id="past-expired-list"`, `id="past-expired-cut"`)
	if n := strings.Count(groups, `class="past-group"`); n != 2 {
		t.Errorf("#past-groups holds %d groups, want Resolved and Expired", n)
	}

	title := vsBlock(t, pane, `id="feed-heading-title"`)
	if !strings.Contains(title, ">Nearby now</h2>") {
		t.Errorf("the heading must start as Nearby now, got %q", title)
	}
}

// TestViewStripPaneScript pins the per tab render of feed.js: the heading and
// note words of C3 to C7, the count line words of C8 to C10, exactly one of
// the empty blocks showing, a chip emptied list saying so and never the
// history line, the two buttons, and focus falling back to the current tab.
func TestViewStripPaneScript(t *testing.T) {
	feed := p19Code(t, "static/js/feed.js")

	p13MustContain(t, "feed.js", feed,
		"active: 'Nearby now', disputed: 'Disputed reports', past: 'Past reports'",
		"disputed: 'Nearby people disputed these. You can still confirm or dispute them.',",
		"past: 'Resolved and expired reports from the last 7 days.'")

	heading := jsFunctionBody(t, feed, "updateFeedHeading(view)")
	p13MustContain(t, "updateFeedHeading", heading,
		"Pinalert.setText(feedHeadingTitleEl, heading);", "Pinalert.setText(feedNoteEl, note);",
		"setHidden(feedNoteEl, note === '');", "appShellEl.setAttribute('data-feed-view', view);")
	p13MustNotContain(t, "updateFeedHeading", heading, "innerHTML", "insertAdjacentHTML")

	count := jsFunctionBody(t, feed, "updateFeedCount(state)")
	p13MustContain(t, "updateFeedCount", count,
		"view === 'disputed' ? 'disputed' : 'active'", "count === 1 ? 'report' : 'reports'",
		"' \\u00b7 '")

	empty := jsFunctionBody(t, feed, "setEmptyState(kind)")
	for _, pair := range []string{
		"setHidden(emptyEl, kind !== 'general');", "setHidden(historyEmptyEl, kind !== 'history');",
		"setHidden(disputedEmptyEl, kind !== 'disputed');", "setHidden(pastEmptyEl, kind !== 'past');",
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
		"if (Pinalert.state.view === 'past') {", "renderPastTab(state);",
		"if (state.reports.length === 0) {", "if (Pinalert.state.category !== '') {", "kind = 'filter';",
		"kind = 'disputed';", "setEmptyState(kind);")
	p13MustContain(t, "render", render, "updateFeedStatus(state);", "updateFeedHeading(Pinalert.state.view);")

	past := jsFunctionBody(t, feed, "renderPastTab(state)")
	p13Ordered(t, past, "renderPastTab",
		"setHidden(reportListEl, true);", "clearRows();", "state.reports.length === 0",
		"setEmptyState(Pinalert.state.category !== '' ? 'filter' : 'past');",
		"setHidden(pastGroupsEl, false);")

	p13MustContain(t, "feed.js", feed,
		"filterClearBtn.addEventListener('click', function () {", "Pinalert.setCategory('');",
		"seePastBtn.addEventListener('click', function () {", "switchTab('past');")

	// Focus falls back to the map first and then to the current tab link.
	after := jsFunctionBody(t, feed, "focusAfterRemoval(candidates)")
	p13Ordered(t, after, "focusAfterRemoval",
		"getElementById('map')", "currentTabLink()", "viewToggleBtn.focus()", "var empties = [")
}

// TestViewStripStatusAnnouncement pins C11: one sentence in the visually
// hidden live region per tab, place or chip change, built from the store's own
// numbers and words, written through setText, never repeated for a poll.
func TestViewStripStatusAnnouncement(t *testing.T) {
	feed := p19Code(t, "static/js/feed.js")
	body := jsFunctionBody(t, feed, "updateFeedStatus(state)")

	p13Ordered(t, body, "updateFeedStatus",
		"state.status !== 'ok'",
		"var key = store.view + '|' + placeName + '|' + store.category;",
		"if (key === lastAnnouncedKey) {", "lastAnnouncedKey = key;",
		"if (!statusStarted) {",
		"var text = count + ' ' + store.view + (count === 1 ? ' report.' : ' reports.');",
		"text += ' Near ' + placeName + '.';",
		"text += ' Filtered by '",
		"Pinalert.setText(feedStatusEl, text);")
	p13MustContain(t, "updateFeedStatus", body, "Pinalert.CHIP_LABELS[store.category]", "state.reports.length")
	p13MustNotContain(t, "updateFeedStatus", body, "innerHTML", "insertAdjacentHTML")
	if n := strings.Count(feed, "Pinalert.setText(feedStatusEl,"); n != 1 {
		t.Errorf("the status line is written in %d places, want exactly 1", n)
	}
	// The count of reports is not part of the key, so a poll that only changes
	// the count never speaks again.
	if strings.Contains(body, "state.reports.length + '|'") {
		t.Errorf("the announcement key must not hold the count")
	}
}

// vsRule returns the one rule of rules whose head equals head and whose media
// contains media (an empty media means top level only), failing the test when
// there is none.
func vsRule(t *testing.T, rules []p13CSSRule, media, head string) p13CSSRule {
	t.Helper()
	r, _ := p13Rule(t, rules, media, head)
	return r
}

// TestViewStripNoPillAndTouchTarget pins the strip's look (UI-SPEC Surface 1):
// a 44px target and an 8px corner on every tab, never a pill, the grey track,
// the white current tab with a border and weight 600 (so it is never told apart
// by colour alone), the inset focus ring, the forced colours border, the laptop
// cap and margins, tokens only, and the checkbox rule gone.
func TestViewStripNoPillAndTouchTarget(t *testing.T) {
	rules := p13ParseCSS(p13ReadStatic(t, "static/css/feed.css"))

	tab := vsRule(t, rules, "", ".view-tab")
	for prop, want := range map[string]string{
		"min-height":      "var(--touch-target-min)",
		"border-radius":   "8px",
		"color":           "var(--color-text)",
		"text-decoration": "none",
	} {
		if got := tab.Decls[prop]; got != want {
			t.Errorf(".view-tab %s is %q, want %q", prop, got, want)
		}
	}
	if r := tab.Decls["border-radius"]; r == "999px" || strings.Contains(r, "%") {
		t.Errorf(".view-tab corner %q is a pill", r)
	}

	track := vsRule(t, rules, "", ".view-tabs")
	for prop, want := range map[string]string{
		"background":    "var(--color-surface)",
		"border-radius": "8px",
		"flex-wrap":     "wrap",
		"margin":        "0 0 var(--space-sm)",
	} {
		if got := track.Decls[prop]; got != want {
			t.Errorf(".view-tabs %s is %q, want %q", prop, got, want)
		}
	}

	current := vsRule(t, rules, "", `.view-tab[aria-current="page"], .view-tab[aria-current="page"]:hover, .view-tab[aria-current="page"]:active`)
	for prop, want := range map[string]string{
		"background":   "var(--color-bg)",
		"border-color": "var(--color-border)",
		"font-weight":  "var(--font-weight-semibold)",
	} {
		if got := current.Decls[prop]; got != want {
			t.Errorf("the current tab %s is %q, want %q", prop, got, want)
		}
	}

	focus := vsRule(t, rules, "", ".view-tab:focus-visible")
	if got := focus.Decls["outline-offset"]; got != "-2px" {
		t.Errorf(".view-tab:focus-visible outline-offset is %q, want -2px", got)
	}

	forced := vsRule(t, rules, "forced-colors", `.view-tab[aria-current="page"]`)
	if got := forced.Decls["border"]; got != "1px solid CanvasText" {
		t.Errorf("the forced colours current tab border is %q, want 1px solid CanvasText", got)
	}

	button := vsRule(t, rules, "", ".inline-text-button")
	if got := button.Decls["min-height"]; got != "var(--touch-target-min)" {
		t.Errorf(".inline-text-button min-height is %q, want the 44px token", got)
	}
	if got := button.Decls["border-radius"]; got != "8px" {
		t.Errorf(".inline-text-button corner is %q, want 8px", got)
	}

	laptop := vsRule(t, rules, "min-width: 900px", ".view-tabs")
	if laptop.Decls["max-width"] != "360px" || laptop.Decls["margin"] != "var(--space-md) 0 var(--space-sm)" {
		t.Errorf("the laptop strip is %v, want max-width 360px and margin var(--space-md) 0 var(--space-sm)", laptop.Decls)
	}
	heading := vsRule(t, rules, "min-width: 900px",
		`#app-shell[data-feed-view="disputed"] .feed-heading, #app-shell[data-feed-view="past"] .feed-heading`)
	if heading.Decls["display"] != "flex" {
		t.Errorf("the laptop heading for Disputed and Past must display flex, got %q", heading.Decls["display"])
	}
	// phone.css hides .feed-heading with its base rule; the laptop rule must
	// outrank it by specificity, with phone.css left as it is.
	if !strings.Contains(heading.Head, "#app-shell") {
		t.Errorf("the laptop heading rule must be more specific than phone.css's base hide")
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

// TestViewStripContrast computes the two text pairs the strip and its note
// introduce, from the shipped tokens in all four theme sources: the tab text
// on the grey track and the muted note on the page background (UI-SPEC Color).
func TestViewStripContrast(t *testing.T) {
	needed := []string{"--color-bg", "--color-surface", "--color-text", "--color-text-muted"}
	light1, light2, dark1, dark2 := extractThemeTokens(parseCSSRules(p13ReadStatic(t, "static/css/main.css")), needed)
	sources := []struct {
		name   string
		tokens themeTokens
	}{
		{":root", light1}, {`:root[data-theme="light"]`, light2},
		{"prefers dark :root", dark1}, {`:root[data-theme="dark"]`, dark2},
	}
	pairs := []struct{ name, fg, bg string }{
		{"tab text on the track", "--color-text", "--color-surface"},
		{"current tab text on the page background", "--color-text", "--color-bg"},
		{"the note line on the page background", "--color-text-muted", "--color-bg"},
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
