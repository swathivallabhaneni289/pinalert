package web

// Plan 03.4-11 contract tests: the Show menu and the disputed group of the
// Nearby list (Phase 3.4, D-13, D-14, D-24a, D-07, TRUST-02). Like the other
// contract files these are static inspection of the embedded bytes; nothing
// here needs a database or a server. The behaviour itself is driven by
// show_menu_test.mjs and feed_groups_test.mjs. Helpers carry the sm11 prefix.

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// TestShowMenuRadioSemantics pins the menu's semantics (UI-SPEC 2c): a real
// button that has a menu, three radio items with a checked state, 44px rows,
// the keyboard keys of the pattern, and the one writer of aria-expanded.
func TestShowMenuRadioSemantics(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")

	if n := strings.Count(index, `role="menuitemradio"`); n != 3 {
		t.Fatalf("index.html.tmpl holds %d menuitemradio items, want 3", n)
	}
	button := regexp.MustCompile(`<button type="button" class="show-button" id="show-button" aria-haspopup="menu"\s+aria-expanded="false" aria-controls="show-menu">`)
	if !button.MatchString(index) {
		t.Errorf("the Show button must be a button with aria-haspopup=menu, aria-expanded=false and aria-controls=show-menu")
	}
	p13MustContain(t, "index.html.tmpl", index, `<div class="show-menu" id="show-menu" role="menu"`)
	checked := 0
	for _, item := range regexp.MustCompile(`<button[^>]*role="menuitemradio"[^>]*>`).FindAllString(index, -1) {
		if !strings.Contains(item, `aria-checked="`) || !strings.Contains(item, `data-show="`) {
			t.Errorf("a menu item lacks aria-checked or data-show: %s", item)
		}
		if strings.Contains(item, `aria-checked="true"`) {
			checked++
		}
	}
	if checked != 1 {
		t.Errorf("%d menu items ship checked, want exactly 1 (All)", checked)
	}

	// 44px rows and button, an 8px corner (CSS is pinned in the view strip tests;
	// here only the targets the menu depends on).
	rules := p13ParseCSS(p13ReadStatic(t, "static/css/feed.css"))
	for _, head := range []string{".show-button", ".show-menu__item"} {
		r, _ := p13Rule(t, rules, "", head)
		if got := r.Decls["min-height"]; got != "var(--touch-target-min)" {
			t.Errorf("%s min-height is %q, want the 44px token", head, got)
		}
	}

	menu := p19Code(t, "static/js/show-menu.js")
	p13MustContain(t, "show-menu.js", menu,
		"key === 'ArrowDown'", "key === 'ArrowUp'", "key === 'Home'", "key === 'End'", "key === 'Escape'",
		"key === 'Enter'", "'[role=\"menuitemradio\"]'", "setAttribute('aria-checked'",
		"setAttribute('aria-expanded', 'true')", "setAttribute('aria-expanded', 'false')",
		"document.addEventListener('click', onDocumentClick)", "document.removeEventListener('click', onDocumentClick)",
		"window.PinalertShowMenu = (function () {")
	// aria-expanded is written in open and close only, so the announced state
	// cannot disagree with the visible one.
	if n := strings.Count(menu, "setAttribute('aria-expanded'"); n != 2 {
		t.Errorf("show-menu.js writes aria-expanded in %d places, want 2 (open and close)", n)
	}
	p13MustNotContain(t, "show-menu.js", menu, "innerHTML", "insertAdjacentHTML", "scrollIntoView", "fetch(", "history.", "localStorage")
	// Escape returns focus to the button; the choice goes to the store's setter.
	closeBody := jsFunctionBody(t, menu, "close(restoreFocus)")
	p13Ordered(t, closeBody, "close", "menuEl.hidden = true;", "setAttribute('aria-expanded', 'false')", "buttonEl.focus();")
	chooseBody := jsFunctionBody(t, menu, "choose(show)")
	p13MustContain(t, "choose", chooseBody, "SHOW_WORDS.indexOf(show) === -1", "close(true);", "Pinalert.setShow(show);")

	// The Only Reliable reporters tick is Phase 3.2's, not drawn now.
	p13MustNotContain(t, "index.html.tmpl", index, "Only Reliable reporters", "Reliable reporters")
	if !regexp.MustCompile(`(?s)id="show-menu".*?</div>`).MatchString(index) {
		t.Errorf("could not read the Show menu block")
	}
}

// TestDisputedGroupCues pins the five cues of a disputed row and its group
// (UI-SPEC 2e, D-14, D-07): the cue words come from fixed strings, the group is
// built after the feed's sort and last, the lead is text inside the row (not an
// aria-label that would hide the counters), the classes carry the grey dot, the
// bold critical line, the shown glyph, and no rule fades a row.
func TestDisputedGroupCues(t *testing.T) {
	feed := p19Code(t, "static/js/feed.js")

	p13MustContain(t, "feed.js", feed,
		"var DISPUTED_LINE = 'Disputed: may not be true';",
		"var DISPUTED_LEAD = 'Disputed, may not be true.';",
		"var PROVISIONAL_LEAD = 'Not confirmed yet.';",
		"'report-row__disputed-line'", "'report-row__lead visually-hidden'",
		"'report-row--disputed'")
	// Amended 2026-10-17 by Phase 3.4 plan 13: the group heading is removed; the
	// UI-SPEC puts the words on each disputed row only (D-02 nothing repeated).
	// The test now pins the absence of the heading item.
	p13MustNotContain(t, "feed.js", feed, "report-group-head", "placeGroupHead", "removeGroupHead", "groupHeadEl")
	if n := strings.Count(feed, "Disputed: may not be true"); n != 1 {
		t.Errorf("feed.js names the disputed line %d times, want 1 (the one constant of the row's bold line)", n)
	}
	p13MustNotContain(t, "feed.css", p13ReadStatic(t, "static/css/feed.css"), "report-group-head")

	// The partition is a server word and nothing else, and it runs after the
	// feed's own sort, so a disputed Critical report never sits above a live row.
	render := jsFunctionBody(t, feed, "renderRows(reports)")
	p13MustContain(t, "renderRows", render, "Pinalert.disputedLast(sortReports(reports))")
	disputed := jsFunctionBody(t, feed, "isDisputed(report)")
	p13MustContain(t, "isDisputed", disputed, "report.view === 'disputed'")
	p13MustNotContain(t, "isDisputed", disputed, "trust", "_places", "visibility", ">=", "<=")

	update := jsFunctionBody(t, feed, "updateRow(row, report)")
	p13MustContain(t, "updateRow", update,
		"row.el.classList.toggle('report-row--disputed', disputed);", "row.disputedLine.hidden = !disputed;",
		"Pinalert.setText(row.lead, lead);")
	// The state word "Disputed" is the visibility tag, from the one source of
	// state words, so the row cannot name a second spelling.
	p13MustContain(t, "updateRow", update, "PinalertVisibility.updateVisibilityTag(row.tag, report);")
	if !strings.Contains(p19Code(t, "static/js/visibility.js"), "hidden: 'Disputed',") {
		t.Errorf("visibility.js lost the Disputed state word")
	}
	// The lead is inside the row body, in front of the severity word, and the
	// row has no aria-label of its own.
	create := jsFunctionBody(t, feed, "createRow(id)")
	p13Ordered(t, create, "createRow", "body.appendChild(severityLabel);", "body.insertBefore(lead, severityLabel);")
	p13MustNotContain(t, "createRow", create, "aria-label")
	p13MustNotContain(t, "updateRow", update, "aria-label")

	css := p13ReadStatic(t, "static/css/feed.css")
	rules := p13ParseCSS(css)

	line, _ := p13Rule(t, rules, "", "#report-list .report-row .report-row__disputed-line:not([hidden])")
	for prop, want := range map[string]string{
		"color":       "var(--color-severity-critical)",
		"font-weight": "var(--font-weight-semibold)",
		"flex":        "0 0 100%",
	} {
		if got := line.Decls[prop]; got != want {
			t.Errorf("the disputed line %s is %q, want %q", prop, got, want)
		}
	}
	dot, _ := p13Rule(t, rules, "", "#report-list .report-row.report-row--disputed .severity-label::before")
	if got := dot.Decls["background-color"]; got != "var(--color-age-stale)" {
		t.Errorf("the disputed dot is %q, want the grey --color-age-stale", got)
	}
	badge, _ := p13Rule(t, rules, "", "#report-list .report-row.report-row--disputed > .icon-badge")
	if got := badge.Decls["display"]; got != "inline-flex" {
		t.Errorf("the disputed row's category glyph display is %q, want it shown", got)
	}
	text, _ := p13Rule(t, rules, "", "#report-list .report-row.vis-hidden.report-row--disputed .report-row__meta, #report-list .report-row.vis-hidden.report-row--disputed .report-row__away:not([hidden]), #report-list .report-row.vis-hidden.report-row--disputed .report-row__far:not([hidden]), #report-list .report-row.vis-hidden.report-row--disputed .report-row__trustline:not([hidden])")
	if got := text.Decls["color"]; got != "var(--color-text)" {
		t.Errorf("a disputed row's text color is %q, want full contrast (--color-text)", got)
	}

	// No rule fades a row, the heading or the banner: no opacity under 1 in
	// any rule of the list's stylesheets that names them.
	for _, name := range []string{"static/css/feed.css", "static/css/phone.css", "static/css/trust.css"} {
		for _, r := range p13ParseCSS(p13ReadStatic(t, name)) {
			if !strings.Contains(r.Head, "report-row") &&
				!strings.Contains(r.Head, "alert-banner") && !strings.Contains(r.Head, "vis-hidden") {
				continue
			}
			if v, ok := r.Decls["opacity"]; ok && strings.TrimSpace(v) != "1" {
				t.Errorf("%s rule %q sets opacity %s: a disputed or hidden row keeps full contrast (D-07)", name, r.Head, v)
			}
		}
	}
}

// sm11ThresholdScan returns the lines of a script that compare a trust count
// with a number from 1 up, or name the resolver's threshold. A comparison with
// 0 (a validity check such as >= 0) is not a threshold.
func sm11ThresholdScan(text string) []string {
	compare := regexp.MustCompile(`(confirmed_places|disputed_places|confirmed_by_people|trust\.confirmed|trust\.disputed)\s*(>=|<=|>|<|===|!==|==|!=)\s*[1-9]`)
	reverse := regexp.MustCompile(`[1-9][0-9]*\s*(>=|<=|>|<|===|!==|==|!=)\s*[a-zA-Z_.]*(confirmed_places|disputed_places)`)
	constant := regexp.MustCompile(`IndependentAgreementThreshold|AGREEMENT_THRESHOLD|MIN_CONFIRM|CONFIRM_THRESHOLD|DISPUTE_THRESHOLD`)
	var hits []string
	for _, line := range strings.Split(text, "\n") {
		if compare.MatchString(line) || reverse.MatchString(line) || constant.MatchString(line) {
			hits = append(hits, strings.TrimSpace(line))
		}
	}
	return hits
}

// TestNoClientTrustThreshold pins TRUST-02 for the Show menu: no script of the
// page compares a trust count with a number or holds the resolver's threshold.
// Membership of Show Confirmed and Show Disputed is read from two server words,
// view and confirmed_by_people, in one function, and Show Confirmed never lets
// a disputed report in.
func TestNoClientTrustThreshold(t *testing.T) {
	// The scanner bites: it flags a threshold and ignores a validity check.
	for _, bad := range []string{
		"if (report.trust.confirmed_places >= 2) {",
		"return trust.disputed_places > 1;",
		"if (2 <= report.trust.confirmed_places) {",
		"var IndependentAgreementThreshold = 2;",
		"return report.confirmed_by_people === 1;",
	} {
		if len(sm11ThresholdScan(bad)) != 1 {
			t.Errorf("the threshold scan does not flag %q", bad)
		}
	}
	for _, ok := range []string{
		"Number.isInteger(trust.confirmed_places) && trust.confirmed_places >= 0 &&",
		"return report.confirmed_by_people === true && report.view !== 'disputed';",
	} {
		if len(sm11ThresholdScan(ok)) != 0 {
			t.Errorf("the threshold scan flags %q", ok)
		}
	}

	entries, err := fs.ReadDir(StaticFS, "static/js")
	if err != nil {
		t.Fatalf("reading static/js: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		code := p19Code(t, "static/js/"+e.Name())
		if hits := sm11ThresholdScan(code); len(hits) > 0 {
			t.Errorf("static/js/%s compares a trust count with a number or names the threshold: %v", e.Name(), hits)
		}
	}

	// The Show scripts name no trust count at all.
	for _, name := range []string{"static/js/feed.js", "static/js/map.js", "static/js/show-menu.js", "static/js/ended-cards.js"} {
		code := p19Code(t, name)
		p13MustNotContain(t, name, code, "confirmed_places", "disputed_places")
	}

	app := p19Code(t, "static/js/app.js")
	member := jsFunctionBody(t, app, "reportInShow(report, show)")
	p13MustContain(t, "reportInShow", member, "report.confirmed_by_people === true", "report.view !== 'disputed'", "report.view === 'disputed'")
	p13MustNotContain(t, "reportInShow", member, "trust", "_places", "visibility", "severity", ".length", ">=", "<=", "> ", "< ")
	if n := strings.Count(app, "confirmed_by_people"); n != 1 {
		t.Errorf("app.js names confirmed_by_people %d times, want 1 (the one rule)", n)
	}
	// Both surfaces draw the one visible set.
	p13MustContain(t, "map.js", p19Code(t, "static/js/map.js"), "Pinalert.visibleReports()")
	p13MustContain(t, "feed.js", p19Code(t, "static/js/feed.js"), "Pinalert.visibleReports()")
}

// TestChipsSwapRescueForTraffic pins D-24a: the chip row is All, Flood,
// Traffic, Shelter, Power in that order, there is no Rescue chip, the chip
// words come from the store's one table, and old rescue reports still have a
// label and an icon so they show under All until they fade.
func TestChipsSwapRescueForTraffic(t *testing.T) {
	tmpl := p13ReadTemplate(t, "index.html.tmpl")
	chips := regexp.MustCompile(`<button type="button" class="map-chip" data-category="([a-z_]*)" aria-pressed="(true|false)">([A-Za-z]+)</button>`).FindAllStringSubmatch(tmpl, -1)
	want := []string{"All", "Flood", "Traffic", "Shelter", "Power"}
	if len(chips) != len(want) {
		t.Fatalf("the page holds %d chips, want %d", len(chips), len(want))
	}
	for i, w := range want {
		if chips[i][3] != w {
			t.Errorf("chip %d reads %q, want %q", i, chips[i][3], w)
		}
	}
	if chips[2][1] != "traffic" {
		t.Errorf("the third chip filters %q, want traffic", chips[2][1])
	}
	for _, c := range chips {
		if c[1] == "rescue_needed" || c[3] == "Rescue" {
			t.Errorf("the page still holds the Rescue chip: %v", c)
		}
	}

	app := p19Code(t, "static/js/app.js")
	table, found, closed := windowAfter(app, "var CHIP_LABELS = {", "};")
	if !found || !closed {
		t.Fatalf("could not bound CHIP_LABELS")
	}
	p13MustContain(t, "CHIP_LABELS", table, "flood: 'Flood',", "traffic: 'Traffic',", "shelter_open: 'Shelter',", "power_outage: 'Power'")
	p13MustNotContain(t, "CHIP_LABELS", table, "rescue_needed")

	// Old rescue reports keep a word and stay a known category.
	p13MustContain(t, "app.js", app,
		"var RETIRED_CATEGORIES = ['rescue_needed'];", "rescue_needed: 'Rescue needed',",
		"var KNOWN_CATEGORIES = CATEGORIES.concat(RETIRED_CATEGORIES);")
}
