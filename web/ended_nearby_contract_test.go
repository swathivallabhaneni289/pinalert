package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds the static contract tests of My activity, part Ended nearby
// (Phase 3.4, plan 14, D-18 to D-21). Like the other contract files nothing here
// needs a database or a server: the tests read the embedded scripts, the
// template and the stylesheet and inspect the shipped text. The behaviour of the
// read, the groups and the states is run by web/ended_nearby_test.mjs.
//
// Two guarantees are pinned here because plan 11 deleted the Past tab tests that
// held them (orchestrator hand over): the WR-02 guarantee that a partial flag
// never leaves an empty group block (TestEndedNearbyNeverEmptyBlockWhilePartial,
// and run in node), and the look of the Ended nearby rows and group labels with
// their contrast in both themes (TestEndedNearbyStyles,
// TestEndedNearbyContrastAcrossThemes).

// TestEndedNearbyOneFetch pins that the page's one network read of this part is
// in ended-nearby.js: exactly one call, none in activity.js (which computes and
// fetches nothing) and none in the row builders.
func TestEndedNearbyOneFetch(t *testing.T) {
	nearby := p16ReadStatic(t, "static/js/ended-nearby.js")
	if n := strings.Count(nearby, "fetch("); n != 1 {
		t.Errorf("ended-nearby.js holds fetch( %d times, want exactly 1 (comments included)", n)
	}
	for _, name := range []string{"static/js/activity.js", "static/js/ended-cards.js"} {
		if n := strings.Count(p16ReadStatic(t, name), "fetch("); n != 0 {
			t.Errorf("%s holds fetch( %d times, want 0", name, n)
		}
	}
	code := p13CodeLines(nearby)
	p13MustContain(t, "ended-nearby.js", code,
		"'&view=past'", "'?lat=' + encodeURIComponent(position.lat)", "'&lon=' + encodeURIComponent(position.lon)",
		"'&radius_km=' + encodeURIComponent(readRadius())", "'&report=' + encodeURIComponent(chosenId)",
		"Pinalert.requestTimeout(READ_TIMEOUT_MS,", "var READ_TIMEOUT_MS = 10000;",
		"var REPORT_ID_PATTERN = /^[1-9][0-9]{0,17}$/;")
}

// TestEndedNearbyScriptsAndRadius pins that the template loads the three scripts
// of the part once each, in order, after app.js, and writes the radius the read
// sends from the server's page config (the e2e twin is TestActivityPageCarriesRadius).
func TestEndedNearbyScriptsAndRadius(t *testing.T) {
	tmpl := p13ReadTemplate(t, "profile.html.tmpl")
	p13Ordered(t, tmpl, "scripts",
		`/static/js/app.js?v={{.AssetVersion}}" defer`,
		`/static/js/position.js?v={{.AssetVersion}}" defer`,
		`/static/js/ended-cards.js?v={{.AssetVersion}}" defer`,
		`/static/js/ended-nearby.js?v={{.AssetVersion}}" defer`)
	for _, name := range []string{"position.js", "ended-cards.js", "ended-nearby.js"} {
		if n := strings.Count(tmpl, "/static/js/"+name); n != 1 {
			t.Errorf("profile.html.tmpl loads %s %d times, want 1", name, n)
		}
	}
	if n := strings.Count(tmpl, `data-default-radius-km="{{.DefaultRadiusKm}}"`); n != 1 {
		t.Errorf("profile.html.tmpl writes the radius %d times, want 1", n)
	}
}

// TestEndedNearbyNoServerLocation pins D-27 and T-34-60: the reader's position is
// read from the browser's own store and sent only as the query of the one read.
// The script never saves it, never keeps it in any store, never posts it and never
// puts it in a link or a cookie, and the page the server draws carries none.
func TestEndedNearbyNoServerLocation(t *testing.T) {
	code := p13CodeLines(p16ReadStatic(t, "static/js/ended-nearby.js"))

	p13MustContain(t, "ended-nearby.js", code, "PinalertPosition.read()")
	p13MustNotContain(t, "ended-nearby.js", code,
		"PinalertPosition.save", "localStorage", "sessionStorage", "document.cookie", "setItem(", "indexedDB",
		"sendBeacon", "XMLHttpRequest", "method:", "body:", "JSON.stringify", "FormData", "navigator.geolocation",
		"history.pushState", "history.replaceState", "location.href", "location.assign", "location.replace")

	// The only fetch call passes the built address and the abort signal, nothing
	// that carries a body.
	p13MustContain(t, "ended-nearby.js", code, "fetch(url, { signal: limit.signal })")

	// The server side of the page: nothing in the template says where the reader
	// is, and the position is read by the script, not written into the page.
	tmpl := p13ReadTemplate(t, "profile.html.tmpl")
	p13MustNotContain(t, "profile.html.tmpl", tmpl, "data-lat=\"{{.ReaderLat", "data-reader", "geolocation")
}

// TestEndedNearbyStatesCopy pins the exact words of the part's states in the
// template (UI-SPEC 3c) and the two toasts of a link to a report that cannot be
// shown, and that no user visible string holds an em or en dash.
func TestEndedNearbyStatesCopy(t *testing.T) {
	tmpl := p13ReadTemplate(t, "profile.html.tmpl")
	for _, words := range []string{
		"<h3>Ended nearby needs your location</h3>", "<p>Open Nearby and allow location, then come back.</p>",
		"<h3>Couldn't load reports</h3>", "<p>Check your connection and try again.</p>",
		`id="ended-retry">Try again</button>`,
	} {
		if n := strings.Count(tmpl, words); n != 1 {
			t.Errorf("profile.html.tmpl holds %q %d times, want 1", words, n)
		}
	}
	// The empty state words come from the Go list, the same two as before.
	p13MustContain(t, "profile.html.tmpl", tmpl, `id="ended-empty" hidden>`, `<h3>{{.EmptyTitle}}</h3>`)

	// The four skeleton rows show first; every other state starts hidden.
	skeleton := regexp.MustCompile(`(?s)id="ended-skeleton">(.*?)</div>\s*<div class="activity-cols"`).FindStringSubmatch(tmpl)
	if skeleton == nil {
		t.Fatal("could not isolate the Ended nearby skeleton")
	}
	if n := strings.Count(skeleton[1], `class="skeleton-row"`); n != 4 {
		t.Errorf("the Ended nearby skeleton holds %d rows, want 4", n)
	}
	for _, id := range []string{"ended-cols", "ended-resolved", "ended-faded", "ended-empty", "ended-location", "ended-error",
		"ended-resolved-cut", "ended-faded-cut", "ended-hub-count", "ended-title-count"} {
		if !regexp.MustCompile(`id="` + id + `"[^>]*\bhidden\b`).MatchString(tmpl) {
			t.Errorf("the Ended nearby element %s does not start hidden", id)
		}
	}

	nearby := p16ReadStatic(t, "static/js/ended-nearby.js")
	p13MustContain(t, "ended-nearby.js", nearby,
		`var TOO_OLD_TOAST = 'That report is older than 7 days and is no longer shown.';`,
		`var UNAVAILABLE_TOAST = "That report isn't available.";`)

	for _, name := range []string{"static/js/ended-nearby.js", "static/js/ended-cards.js", "static/css/activity.css"} {
		raw := p16ReadStatic(t, name)
		for _, dash := range []string{string(rune(0x2014)), string(rune(0x2013))} {
			if strings.Contains(raw, dash) {
				t.Errorf("%s holds the dash character %q", name, dash)
			}
		}
	}
}

// TestEndedNearbyScriptBans pins the bans of the one script that reads: no
// markup sink, no scroll of the window or the page (only the page's own region
// by its scrollTop), no clock of its own, no address writing. activity.js keeps
// the same bans in TestActivityScriptKeepsTheBansOfTheRemovedTabScript.
func TestEndedNearbyScriptBans(t *testing.T) {
	for _, name := range []string{"static/js/ended-nearby.js", "static/js/ended-cards.js"} {
		raw := p16ReadStatic(t, name)
		code := p13CodeLines(raw)
		p13MustNotContain(t, name, raw, "innerHTML", "insertAdjacentHTML", "scrollIntoView")
		p13MustNotContain(t, name, code,
			"outerHTML", "document.write", "createContextualFragment", "DOMParser", "new Date", "Date.now",
			"window.scroll", "scrollTo(", "scrollBy(", "scrollingElement", "pushState", "replaceState")
	}
	nearby := p13CodeLines(p16ReadStatic(t, "static/js/ended-nearby.js"))
	p13MustContain(t, "ended-nearby.js", nearby, "region.scrollTop = ", "row.el.focus({ preventScroll: true });")

	// The browser computes no outcome and no count of history: no trust field
	// name, no vote count, no threshold.
	ended := p13CodeLines(p16ReadStatic(t, "static/js/ended-cards.js"))
	for _, script := range []string{nearby, ended} {
		p13MustNotContain(t, "ended scripts", script,
			"confirmed_places", "disputed_places", "IndependentAgreementThreshold", "outcome = ", "outcome_label =")
	}
}

// TestEndedNearbyNeverEmptyBlockWhilePartial pins the WR-02 guarantee of Phase
// 3.1, handed over by plan 11 when the Past tab left Nearby: while either partial
// flag is true no empty block is drawn, a group that is partial stays on screen
// even with no rows, and the cap line is never printed for a partial flag. The
// behaviour is run in web/ended_nearby_test.mjs; this pins the shape of the code.
func TestEndedNearbyNeverEmptyBlockWhilePartial(t *testing.T) {
	nearby := p13CodeLines(p16ReadStatic(t, "static/js/ended-nearby.js"))

	// Amended 2026-10-17 by Phase 3.4 plan 19: render takes a second argument, quiet,
	// for the redraw after a take back (nothing scrolls or takes focus). The guards below
	// are unchanged.
	//
	// Amended 2026-10-17 by Phase 3.4 plan 25 (code review part 2 CR-01): render takes a
	// third argument, linkOnly, for the read of a link to a finished report with no kept
	// position; that branch leaves render through finishLinkOnly before any of the guards
	// below, so the guards are unchanged.
	render := jsFunctionBody(t, nearby, "render(answer, quiet, linkOnly)")
	p13MustContain(t, "render", render,
		"var resolvedPartial = cut.resolved_partial === true;", "var expiredPartial = cut.expired_partial === true;",
		"if (shown === 0 && !resolvedPartial && !expiredPartial) {", "showState('empty');")
	p13Ordered(t, render, "render",
		"var resolvedPartial = ", "var expiredPartial = ", "if (shown === 0 && !resolvedPartial && !expiredPartial) {",
		"showState('empty');", "showState('groups');")
	if n := strings.Count(render, "showState('empty')"); n != 1 {
		t.Errorf("render shows the empty state in %d places, want exactly the one guarded by both partial flags", n)
	}
	p13MustContain(t, "render", render,
		"cut.resolved === true, resolvedPartial);", "cut.expired === true, expiredPartial);")
	// The two flags are read as exactly true, so a missing field is false.
	p13MustNotContain(t, "render", render, "cut.resolved_partial)", "cut.expired_partial)")

	group := jsFunctionBody(t, nearby, "drawGroup(group, reports, linked, wasCut, wasPartial)")
	p13MustContain(t, "drawGroup", group,
		"setHidden(group.el, all.length === 0 && !wasPartial);", "setHidden(group.cut, lineText === '');",
		"PinalertEnded.groupLineText(reports.length, wasCut, wasPartial);", "Pinalert.setText(group.cut, lineText);")
	p13MustNotContain(t, "drawGroup", group, "innerHTML", "textContent", "createTextNode(")

	// The cap line and the partial line are separate sentences with separate
	// guards (the words are pinned in TestPastPartialLineWords).
	ended := p13CodeLines(p16ReadStatic(t, "static/js/ended-cards.js"))
	line := jsFunctionBody(t, ended, "groupLineText(count, wasCut, wasPartial)")
	p13Ordered(t, line, "groupLineText",
		"if (wasCut && count > 0) {", "parts.push(cutLineText(count));", "if (wasPartial) {", "parts.push(PARTIAL_LINE_TEXT);")
}

// TestEndedNearbyStyles pins the look the Ended nearby rows and groups have of
// their own in activity.css: the group label (Label 14px, 600, uppercase, 0.04em
// letter spacing, the text colour), the muted cut line, the What happened
// sentence, the trust line, the chosen row's soft grey fill and its focus ring.
// Every colour is a token, no rule fades anything, and the padding that parts
// the label from the cards on a laptop lives only in the 900px block.
func TestEndedNearbyStyles(t *testing.T) {
	raw := p16ReadStatic(t, "static/css/activity.css")
	rules := wm5ActivityRules(t)

	label := wm5CSSRule(t, rules, "", "main.profile-page .activity-group__label")
	p16MustDecl(t, ".activity-group__label", label, map[string]string{
		"margin":         "0",
		"font-size":      "var(--font-size-label)",
		"font-weight":    "var(--font-weight-semibold)",
		"letter-spacing": "0.04em",
		"text-transform": "uppercase",
		"color":          "var(--color-text)",
	})
	cut := wm5CSSRule(t, rules, "", "main.profile-page .activity-group__cut")
	p16MustDecl(t, ".activity-group__cut", cut, map[string]string{
		"font-size": "var(--font-size-label)",
		"color":     "var(--color-text-muted)",
	})
	// On a laptop the label and the cut line take the box padding.
	wm5CSSRule(t, rules, wm5LaptopMedia, "main.profile-page .activity-group__label")
	wm5CSSRule(t, rules, wm5LaptopMedia, "main.profile-page .activity-group__cut")

	p16MustDecl(t, ".activity-card__happened", wm5CSSRule(t, rules, "", "main.profile-page .activity-card__happened"),
		map[string]string{"margin": "0", "font-weight": "var(--font-weight-semibold)", "color": "var(--color-text)"})
	p16MustDecl(t, ".activity-card__facts", wm5CSSRule(t, rules, "", "main.profile-page .activity-card--ended .activity-card__facts:not([hidden])"),
		map[string]string{"font-size": "var(--font-size-label)", "color": "var(--color-text)"})

	chosen := wm5CSSRule(t, rules, "", "main.profile-page .profile-report-list > .activity-card.activity-card--chosen")
	p16MustDecl(t, ".activity-card--chosen", chosen, map[string]string{"background": "var(--color-surface)"})
	focus := wm5CSSRule(t, rules, "", "main.profile-page .activity-card--chosen:focus-visible")
	p16MustDecl(t, ".activity-card--chosen:focus-visible", focus,
		map[string]string{"outline": "2px solid var(--color-focus-ring)", "outline-offset": "-2px"})

	for _, r := range rules {
		if !strings.Contains(r.Head, "activity-group") && !strings.Contains(r.Head, "activity-card--chosen") &&
			!strings.Contains(r.Head, "activity-card--ended") && !strings.Contains(r.Head, "activity-card__happened") {
			continue
		}
		for name, value := range r.Decls {
			if name == "opacity" {
				t.Errorf("%q declares opacity: no text is faded (D-07)", r.Head)
			}
			if strings.Contains(value, "#") || strings.Contains(value, "rgb(") || strings.Contains(value, "hsl(") {
				t.Errorf("%q declares %s: %q, a colour literal where a token belongs", r.Head, name, value)
			}
		}
	}
	if strings.Contains(raw, "!important") {
		t.Errorf("activity.css uses the important flag")
	}
}

// TestEndedNearbyContrastAcrossThemes resolves the colours the Ended nearby group
// labels, cut lines, row lines and the chosen row's fill really declare and
// checks each at 4.5 to 1 in all four theme sources (light, light chosen, OS dark
// and explicit dark). It replaces the deleted TestPastCardContrastAcrossThemes:
// the Past card is gone, and the same words now sit in these rows. Muted text is
// only used on the page colour or the surface fill, both checked here.
func TestEndedNearbyContrastAcrossThemes(t *testing.T) {
	needed := []string{"--color-bg", "--color-surface", "--color-text", "--color-text-muted"}
	light1, light2, dark1, dark2 := extractThemeTokens(parseCSSRules(p13ReadStatic(t, "static/css/main.css")), needed)
	sources := []struct {
		name   string
		tokens themeTokens
	}{
		{":root", light1}, {`:root[data-theme="light"]`, light2},
		{"prefers dark :root", dark1}, {`:root[data-theme="dark"]`, dark2},
	}

	rules := wm5ActivityRules(t)
	colour := func(head string) string {
		r := wm5CSSRule(t, rules, "", head)
		v, ok := r.Decls["color"]
		if !ok {
			t.Fatalf("%q declares no color", head)
		}
		return v
	}
	page := "var(--color-bg)"
	fill := wm5CSSRule(t, rules, "", "main.profile-page .profile-report-list > .activity-card.activity-card--chosen").Decls["background"]
	pairs := []struct{ name, fg, bg string }{
		{"group label on the page", colour("main.profile-page .activity-group__label"), page},
		{"cut line on the page", colour("main.profile-page .activity-group__cut"), page},
		{"What happened on the page", colour("main.profile-page .activity-card__happened"), page},
		{"What happened on the chosen row", colour("main.profile-page .activity-card__happened"), fill},
		{"counters on the page", colour("main.profile-page .activity-card--ended .activity-card__facts:not([hidden])"), page},
		{"counters on the chosen row", colour("main.profile-page .activity-card--ended .activity-card__facts:not([hidden])"), fill},
		{"report text on the chosen row", colour("main.profile-page .activity-card .report-row__title"), fill},
		// Muted text on the surface fill is 4.34 to 1 in the light theme, so the
		// chosen row's detail line declares the text colour instead.
		{"detail line on the chosen row", colour("main.profile-page .profile-report-list > .activity-card--chosen .activity-card__sub"), fill},
		{"detail line on the page", colour("main.profile-page .activity-card .activity-card__sub"), page},
		{"when and distance on the chosen row", colour("main.profile-page .activity-card__when, main.profile-page .activity-card__distance"), fill},
	}
	for _, s := range sources {
		for _, p := range pairs {
			fgName, ok := bareVarName(p.fg)
			if !ok {
				t.Fatalf("%s: %q is not a bare var() reference", p.name, p.fg)
			}
			bgName, ok := bareVarName(p.bg)
			if !ok {
				t.Fatalf("%s: %q is not a bare var() reference", p.name, p.bg)
			}
			ratio := wcagContrastRatio(t, p31Resolve(t, s.tokens, fgName), p31Resolve(t, s.tokens, bgName), s.name+" "+p.name)
			if ratio < 4.5 {
				t.Errorf("%s: %s is %.2f:1, want at least 4.5:1", s.name, p.name, ratio)
			}
		}
	}
}

// TestEndedNearbyTakeBackOnlyForMarker pins plan 19 (03.1 D-22, 03.4 D-21,
// UI-SPEC Surface 3): an Ended nearby row is read only apart from the take back
// of the person who marked the report resolved. Who has one, and how many undos
// are left, is the server's answer read by the one builder in votes.js
// (createTakeBackFor over takeBackState); the row script holds no limit, no
// comparison on the undo count and no fetch of its own. The behaviour is run in
// web/ended_nearby_test.mjs.
func TestEndedNearbyTakeBackOnlyForMarker(t *testing.T) {
	ended := p13CodeLines(p16ReadStatic(t, "static/js/ended-cards.js"))

	p13MustContain(t, "ended-cards.js", ended,
		"PinalertVotes.createTakeBackFor(report, {", "card.takeBack.update(report);", "card.what.appendChild(made.el);")
	// No fetch, no limit, no judgement of who may take back.
	p13MustNotContain(t, "ended-cards.js", ended,
		"fetch(", "XMLHttpRequest", "castVote(", "undos_left", "your_resolution", "is_own_report",
		"takeBackState", "innerHTML")
	for _, number := range []string{"> 0", "> 1", "< 1", "< 2", "=== 0", "=== 1", "=== 2", "<= 2"} {
		for _, line := range strings.Split(ended, "\n") {
			if strings.Contains(strings.ToLower(line), "undo") && strings.Contains(line, number) {
				t.Errorf("ended-cards.js compares an undo count in %q: the server is the only judge of the limit", strings.TrimSpace(line))
			}
		}
	}
	// The read after a take back is the part's own, asked of ended-nearby.js.
	p13MustContain(t, "ended-cards.js", ended, "window.PinalertEndedNearby", "part.refresh(reportId)")

	nearby := p13CodeLines(p16ReadStatic(t, "static/js/ended-nearby.js"))
	p13MustContain(t, "ended-nearby.js", nearby, "refresh: refresh,", "load(true)")
	if n := strings.Count(nearby, "fetch("); n != 1 {
		t.Errorf("ended-nearby.js has %d fetch( calls, want 1: the take back's read goes through load", n)
	}
	refresh := jsFunctionBody(t, nearby, "refresh(reportId)")
	p13MustNotContain(t, "refresh", refresh, "fetch(", "showState(", "scroll", "innerHTML")

	// The builder in votes.js: one block, one decision, one handler.
	votes := p13CodeLines(p16ReadStatic(t, "static/js/votes.js"))
	build := jsFunctionBody(t, votes, "createTakeBackFor(report, options)")
	p13MustContain(t, "createTakeBackFor", build,
		"var firstState = takeBackState(report);", "createTakeBackBlock(report.id)",
		"onTakeBackClick(block, parts.undoBtn);", "var state = takeBackState(next);")
	p13MustNotContain(t, "createTakeBackFor", build,
		"castVote(", "fetch(", "undos_left", "your_resolution", "innerHTML", "opacity")
	if n := strings.Count(votes, "createTakeBackFor: createTakeBackFor"); n != 1 {
		t.Errorf("votes.js exports createTakeBackFor %d times, want 1", n)
	}
	// votes.js runs before the two Ended nearby scripts, so a fast answer can never
	// be drawn before the take back builder exists.
	tmpl := p13ReadTemplate(t, "profile.html.tmpl")
	votesAt := strings.Index(tmpl, "/static/js/votes.js?v=")
	cardsAt := strings.Index(tmpl, "/static/js/ended-cards.js?v=")
	nearbyAt := strings.Index(tmpl, "/static/js/ended-nearby.js?v=")
	if votesAt == -1 || cardsAt == -1 || nearbyAt == -1 || votesAt > cardsAt || votesAt > nearbyAt {
		t.Errorf("profile.html.tmpl must load votes.js before ended-cards.js and ended-nearby.js")
	}
	// The read after a take back is a choice of the block, nothing else changed.
	p13MustContain(t, "readAfterWrite", jsFunctionBody(t, votes, "readAfterWrite(block)"),
		"typeof block.readAgain === 'function'", "Pinalert.fetchReports({ afterWrite: true })")
}
