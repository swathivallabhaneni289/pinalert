package web

import (
	"regexp"
	"strings"
	"testing"
)

// Phase 8 plan 05 pins the report page body (PAGE-01, D-01, D-17, D-18): the
// mount agnostic builder report-page.js, its place in the phone's card, and the
// rule that the page never owns a trust rule. Like the rest of this package
// these tests are static inspection of the embedded bytes. The node tests in
// web/report_page_test.mjs run the builder, and the real phone layout is the
// owner's viewing (plan 08-20).

// TestReportPageReusesTrustBlock pins A-P8-7: the page builder adds no trust
// logic. The trust block above the history is the popup card's own nodes,
// built by trust.js once per card, so report-page.js names nothing of
// PinalertTrust, and map.js builds the trust card exactly once per card.
func TestReportPageReusesTrustBlock(t *testing.T) {
	page := p13CodeLines(p13ReadStatic(t, "static/js/report-page.js"))
	p13MustNotContain(t, "report-page.js", page,
		"PinalertTrust", "createTrustCard", "createTrustBlock", "updateTrustCard",
		"createStateLine", "createSeverityLabel", "updateSeverityLabel",
		"PinalertVisibility", "trust-card", "trust-block")

	mapJS := p13CodeLines(p13ReadStatic(t, "static/js/map.js"))
	if n := strings.Count(mapJS, "PinalertTrust.createTrustCard("); n != 1 {
		t.Errorf("map.js calls PinalertTrust.createTrustCard %d times, want exactly 1 (one trust block per card, reused by the page)", n)
	}
	if n := strings.Count(mapJS, "PinalertReportPage.create("); n != 1 {
		t.Errorf("map.js calls PinalertReportPage.create %d times, want exactly 1", n)
	}
	// The page is created right after the vote block, where its Share button
	// joins the actions group, and guarded, so a failed
	// report-page.js load leaves the card without it (WR-W04), and the Demo,
	// which has no history route, never builds one.
	p13MustContain(t, "map.js", mapJS, "window.PinalertReportPage", "!window.PinalertDemoPage")
	p13Ordered(t, mapJS, "buildPopupCard",
		"PinalertVotes.createVoteBlock(", "PinalertReportPage.create(", "body.appendChild(page.el)")
}

// reportPageNamedNumbers are the constants of report-page.js that hold a
// number, each on its own `var NAME = value;` line. Nothing else in the file
// may hold a number above one: the poll, the timeout, the minute of "Now", the
// skeleton row count, the twelve hour clock, the two digit pad and the 404
// status are timing, layout and transport, never a trust threshold.
var reportPageNamedNumbers = []string{
	"TIMELINE_POLL_MS", "TIMELINE_TIMEOUT_MS", "JUST_NOW_MS", "SKELETON_ROWS",
	"HALF_DAY_HOURS", "TWO_DIGITS", "STATUS_GONE",
}

// TestReportPageNoThresholds keeps the page a pure renderer of what the
// server wrote (A-P8-7, T-08-24): no number literal above one outside the named
// constant lines, no comparison on a place count, and no threshold word.
//
// The plan named the poll, the timeout and the sixty second constants. The
// twelve hour clock, the two digit pad, the skeleton row count and the 404
// status need numbers too, so they are named the same way and listed above.
func TestReportPageNoThresholds(t *testing.T) {
	page := p13ReadStatic(t, "static/js/report-page.js")
	code := p13CodeLines(page)

	named := map[string]bool{}
	for _, name := range reportPageNamedNumbers {
		named[name] = true
	}
	declaration := regexp.MustCompile(`^\s*var ([A-Z_]+) = [0-9]+;\s*$`)
	var scanned []string
	seen := map[string]bool{}
	for _, line := range strings.Split(code, "\n") {
		if m := declaration.FindStringSubmatch(line); m != nil {
			if !named[m[1]] {
				t.Errorf("report-page.js declares the number constant %s, which is not in the named list: %q", m[1], line)
			}
			seen[m[1]] = true
			continue
		}
		scanned = append(scanned, line)
	}
	for _, name := range reportPageNamedNumbers {
		if !seen[name] {
			t.Errorf("report-page.js has no `var %s = number;` line", name)
		}
	}
	body := strings.Join(scanned, "\n")
	if loc := regexp.MustCompile(`\b(?:[2-9]|[1-9][0-9]+)\b`).FindString(body); loc != "" {
		t.Errorf("report-page.js has the numeric literal %q outside the named constants, only 0 and 1 may appear", loc)
	}
	if regexp.MustCompile(`(?:confirmed_places|disputed_places)\s*(?:[<>]|[=!]=)`).MatchString(body) {
		t.Errorf("report-page.js compares a place count; the browser holds no trust threshold")
	}
	if regexp.MustCompile(`(?i)threshold|cutoff|min_?score|half_?life|recent_places|judged_reports|corroborated_reports`).MatchString(body) {
		t.Errorf("report-page.js names a threshold or a server fact field on a code line")
	}
	p13MustNotContain(t, "report-page.js sinks", page,
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "DOMParser", "createContextualFragment")
	p13MustContain(t, "report-page.js", page, "'use strict'", "Pinalert.setText(", "window.PinalertReportPage")
	if strings.ContainsFunc(page, func(r rune) bool { return r == 0x2013 || r == 0x2014 }) {
		t.Errorf("report-page.js holds an em or en dash")
	}
}

// TestReportPageNodeStaysLast pins research Pitfall 5b: arrangeCard runs on
// every placement, a resize across 900px included, and must put Stop updates,
// its error line and Flag before the page node, never after it. The page node
// is the last child of the card body when it is built.
func TestReportPageNodeStaysLast(t *testing.T) {
	mapJS := p13CodeLines(p13ReadStatic(t, "static/js/map.js"))

	arrange := jsFunctionBody(t, mapJS, "arrangeCard(card, phone)")
	p13MustContain(t, "arrangeCard", arrange,
		"card.page.el", "insertBefore(node, pageEl)", "place(mute.el)", "place(mute.error)", "place(flag)")
	p13MustNotContain(t, "arrangeCard", arrange,
		"lastParent.appendChild(mute.el)", "lastParent.appendChild(flag)")

	build := jsFunctionBody(t, mapJS, "buildPopupCard(report)")
	p13Ordered(t, build, "buildPopupCard",
		"body.appendChild(actions)", "body.appendChild(page.el)", "el.appendChild(body)")
	p13MustContain(t, "buildPopupCard", build, "actions.appendChild(page.share)", "page: page")

	// The page follows the card: update last, setHosted with the placement,
	// open and close with the popup.
	update := jsFunctionBody(t, mapJS, "updatePopupCard(card, report)")
	p13MustContain(t, "updatePopupCard", update, "card.page.update(report)")
	placement := jsFunctionBody(t, mapJS, "applyPopupPlacement(popup, card)")
	p13Ordered(t, placement, "applyPopupPlacement", "arrangeCard(card, phone)", "card.page.setHosted(phone)")
	p13MustContain(t, "onPopupOpen", jsFunctionBody(t, mapJS, "onPopupOpen(evt)"), "cards[id].page.open()")
	p13MustContain(t, "onPopupClose", jsFunctionBody(t, mapJS, "onPopupClose(evt)"), "cards[id].page.close()")

	// The two plan 39 markers stay, once each, byte for byte.
	raw := p13ReadStatic(t, "static/js/map.js")
	for _, marker := range []string{
		"// ---- The report view and its address (plan 39)",
		"// ---- End of the report view (plan 39)",
	} {
		if n := strings.Count(raw, marker); n != 1 {
			t.Errorf("map.js holds the marker %q %d times, want exactly 1", marker, n)
		}
	}
}

// TestReportPageIsLinkedInOrder pins where index.html.tmpl links the page: the
// stylesheet after trust.css, and the script after votes.js and trust.js (the
// page reads their cards' numbers only through the feed, but the card it joins
// is built by them) and before map.js, which builds the page.
func TestReportPageIsLinkedInOrder(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")
	if n := strings.Count(index, "/static/js/report-page.js"); n != 1 {
		t.Fatalf("index.html.tmpl loads report-page.js %d times, want 1", n)
	}
	if n := strings.Count(index, "/static/css/report-page.css"); n != 1 {
		t.Fatalf("index.html.tmpl links report-page.css %d times, want 1", n)
	}
	p13MustContain(t, "report-page.js tag", findTagWindow(t, index, "/static/js/report-page.js"), " defer", "?v={{.AssetVersion}}")
	p13MustContain(t, "report-page.css tag", findTagWindow(t, index, "/static/css/report-page.css"), "?v={{.AssetVersion}}")
	p13Ordered(t, index, "index.html.tmpl scripts",
		"/static/js/votes.js", "/static/js/trust.js", "/static/js/report-page.js", "/static/js/map.js")
	p13Ordered(t, index, "index.html.tmpl styles",
		"/static/css/trust.css", "/static/css/report-page.css", "/static/css/phone.css")
}
