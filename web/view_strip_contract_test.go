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
	end := strings.Index(index[start:], "</section>")
	if end == -1 {
		t.Fatal("the list pane section is not closed")
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
	bottom := regexp.MustCompile(`(?s)<nav class="bottom-nav"[^>]*>(.*?)</nav>`).FindStringSubmatch(index)
	if bottom == nil {
		t.Fatal("index.html.tmpl has no bottom nav")
	}
	if n := strings.Count(bottom[1], `class="bottom-nav__tab"`); n != 2 {
		t.Errorf("the bottom bar holds %d tabs, want exactly Feed and Activity", n)
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
		"pending = fetch(url)")
	if n := strings.Count(fetchBody, "'&view='"); n != 1 {
		t.Errorf("fetchReports builds the view parameter in %d places, want 1", n)
	}
	// A parameter is added only for a non default value, so the Active,
	// unfiltered URL is the Phase 1 URL byte for byte.
	p13MustNotContain(t, "fetchReports", fetchBody, "'&view=active'", "'&category=' + sentCategory")

	// The response keeps the three optional objects without rendering them,
	// and clears the link id after the first answered request.
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
		"slug === ''", "CATEGORIES.indexOf(slug) !== -1", "state.category = slug;")
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
