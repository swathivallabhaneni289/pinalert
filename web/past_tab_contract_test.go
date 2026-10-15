package web

import (
	"strings"
	"testing"
)

// This file holds the static contract tests of plan 03.1-12, the Past tab's
// cards and groups, the empty Active tab's recent counts and the tab choice
// for a report link. Like the other contract files nothing here needs a
// database or a server: the tests read the embedded script and stylesheet and
// inspect the shipped text. Every helper and test name carries the pt12 or
// Past prefix so no other plan's file can clash with it.

// pt12Feed returns feed.js with its whole line comments removed.
func pt12Feed(t *testing.T) string {
	t.Helper()
	return p19Code(t, "static/js/feed.js")
}

// TestPastGroupsOrderAndCap pins D-08: the groups are split by the server's
// history state and keep the server's order (no sort), a group with no card is
// hidden with its header, and the cap line says how many cards are shown only
// when the server cut that group.
func TestPastGroupsOrderAndCap(t *testing.T) {
	feed := pt12Feed(t)

	groups := jsFunctionBody(t, feed, "renderPastGroups(state)")
	p13MustContain(t, "renderPastGroups", groups,
		".history.state", "'resolved'", "'expired'", "cut.resolved === true", "cut.expired === true",
		"pastResolvedListEl", "pastExpiredListEl", "removeDepartedPastCards(seen);", "return resolved.length + expired.length;")
	p13Ordered(t, groups, "renderPastGroups",
		"pastResolvedListEl", "pastExpiredListEl")
	p13MustNotContain(t, "renderPastGroups", groups, "sortReports(", ".sort(", "innerHTML", "slice(")

	place := jsFunctionBody(t, feed, "placePastGroup(groupEl, listEl, cutEl, reports, wasCut)")
	p13MustContain(t, "placePastGroup", place,
		"setHidden(groupEl, reports.length === 0);", "setHidden(cutEl, !(wasCut && reports.length > 0));",
		"Pinalert.setText(cutEl, cutLineText(reports.length));")

	cut := jsFunctionBody(t, feed, "cutLineText(count)")
	p13MustContain(t, "cutLineText", cut, "'Showing the newest ' + count + '.'")
	if n := strings.Count(feed, "Showing the newest "); n != 1 {
		t.Errorf("feed.js holds the cap line words %d times, want 1 (one helper for both groups)", n)
	}

	tab := jsFunctionBody(t, feed, "renderPastTab(state)")
	p13Ordered(t, tab, "renderPastTab",
		"setHidden(reportListEl, true);", "clearRows();", "state.reports.length === 0",
		"setHidden(pastGroupsEl, false);", "renderPastGroups(state)")

	// The Past cards are cleared when the live list is drawn, so a card of
	// another tab never lingers in a hidden list.
	render := jsFunctionBody(t, feed, "render(state)")
	p13MustContain(t, "render", render, "clearPastCards();")
}

// TestPastCardMarkup pins D-11: a card is one tab stop, holds no button and no
// link, opens the report through activateRow on a click, Enter or Space, and
// ends with the visually hidden Open report cue.
func TestPastCardMarkup(t *testing.T) {
	feed := pt12Feed(t)
	body := jsFunctionBody(t, feed, "createPastCard(id)")

	p13MustContain(t, "createPastCard", body,
		"createElement('li')", "'past-card'", "setAttribute('tabindex', '0')", "dataset.reportId = String(id)",
		"'past-card__title'", "'past-card__meta'", "'past-card__facts'", "'past-card__open-cue visually-hidden'",
		"Pinalert.setText(cue, 'Open report');",
		"addEventListener('click'", "addEventListener('keydown'",
		"evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar'", "activateRow(id);")
	p13MustNotContain(t, "createPastCard", body,
		"createElement('button')", "createElement('a')", "createElement('input')", "innerHTML", "href")
	p13Ordered(t, body, "createPastCard",
		"li.appendChild(title);", "li.appendChild(meta);", "li.appendChild(facts);", "li.appendChild(cue);")

	update := jsFunctionBody(t, feed, "updatePastCard(card, report)")
	p13MustNotContain(t, "updatePastCard", update, "appendChild(card.cue", "card.el.appendChild(")
	p13MustContain(t, "updatePastCard", update, "past-card--selected")
}

// TestPastCardWords pins D-08 to D-10: the meta and facts lines are the
// server's words joined by the app's middle dot, every string goes through
// setText, and the browser computes no count and no outcome.
func TestPastCardWords(t *testing.T) {
	feed := pt12Feed(t)

	meta := jsFunctionBody(t, feed, "buildPastMeta(report)")
	p13MustContain(t, "buildPastMeta", meta,
		"history.closed_text", "Pinalert.relativeTime(history.at)", "'Faded '", "history.lifetime_text",
		"history.state === 'resolved'", "SEPARATOR")
	p13MustNotContain(t, "buildPastMeta", meta, "innerHTML", "Math.", "parseInt(")

	// The facts line is the one trust line builder: the standing pin of
	// TestTrustBlockMountedOnBothSurfaces keeps every trust field name out of
	// feed.js, so the numerals and words come from PinalertTrust.
	create := jsFunctionBody(t, feed, "createPastCard(id)")
	p13MustContain(t, "createPastCard", create,
		"PinalertTrust.createTrustLine();", "facts.classList.add('past-card__facts');",
		"'past-card__outcome'", "querySelectorAll('.report-row__trustline-count')",
		"classList.add('past-card__count')", "classList.add('past-card__count', 'past-card__count--second')")

	update := jsFunctionBody(t, feed, "updatePastCard(card, report)")
	p13MustContain(t, "updatePastCard", update,
		"Pinalert.setText(card.title,", "Pinalert.setText(card.meta, buildPastMeta(report));",
		"PinalertTrust.updateTrustLine(card.trust, report.trust);", "card.trust.word.hidden = true;",
		"history.outcome_label", "history.outcome === 'corroborated'",
		"Pinalert.setText(card.outcome,", "card.firstCount.hidden", "card.secondCount.hidden")
	p13MustNotContain(t, "updatePastCard", update,
		"innerHTML", "insertAdjacentHTML", "Math.", "parseInt(", "+ 1", "_text", "createTextNode(")
	if strings.Contains(feed, "confirmed_text") || strings.Contains(feed, "disputed_text") {
		t.Errorf("feed.js names a trust text field, a Past card must use PinalertTrust's trust line")
	}

	// The label is written first, and the corroborated outcome leaves the
	// confirm count to the label while no other outcome hides it.
	p13Ordered(t, update, "updatePastCard facts",
		"history.outcome_label", "card.firstCount.hidden", "history.outcome === 'corroborated'",
		"card.secondCount.hidden")
}

// TestPastCardApproximate pins D-23 and T-31-55: a finished Rescue card is
// titled Rescue needed, its meta begins Approximate location and the
// description is read only when the report is not approximate.
func TestPastCardApproximate(t *testing.T) {
	feed := pt12Feed(t)

	p13MustContain(t, "feed.js", feed,
		"var FINISHED_RESCUE_TITLE = 'Rescue needed';", "var APPROXIMATE_LABEL = 'Approximate location';")

	update := jsFunctionBody(t, feed, "updatePastCard(card, report)")
	p13MustContain(t, "updatePastCard", update,
		"var approximate = report.approximate === true;",
		"approximate ? FINISHED_RESCUE_TITLE : report.description")
	if n := strings.Count(update, "report.description"); n != 1 {
		t.Errorf("updatePastCard reads report.description %d times, want exactly the one guarded by approximate", n)
	}
	if strings.Contains(update, "description") && !strings.Contains(update, "approximate ? FINISHED_RESCUE_TITLE : report.description") {
		t.Errorf("updatePastCard reads the description outside the approximate guard")
	}

	meta := jsFunctionBody(t, feed, "buildPastMeta(report)")
	p13MustContain(t, "buildPastMeta", meta,
		"report.approximate === true", "APPROXIMATE_LABEL : Pinalert.categoryLabel(report.category)")
	p13MustNotContain(t, "buildPastMeta", meta, "description")
}

// TestPastCardReachesTheSharedHelpers pins that the 60 second refresh, the
// selection highlight and the pane reveal find a Past card by its id.
func TestPastCardReachesTheSharedHelpers(t *testing.T) {
	feed := pt12Feed(t)

	refresh := jsFunctionBody(t, feed, "refreshAgeAndTime()")
	p13MustContain(t, "refreshAgeAndTime", refresh, "pastCardsById", "buildPastMeta(")

	highlight := jsFunctionBody(t, feed, "applySelectionHighlight(id)")
	p13MustContain(t, "applySelectionHighlight", highlight, "pastCardsById", "past-card--selected")

	reveal := jsFunctionBody(t, feed, "revealRowInListPane(id)")
	p13MustContain(t, "revealRowInListPane", reveal, "pastCardsById[String(id)]")

	after := jsFunctionBody(t, feed, "focusAfterRemoval(candidates)")
	p13MustContain(t, "focusAfterRemoval", after, "classList.contains('past-card')")
}

// TestRecentSentence pins C39's counts sentence {A}: only the parts whose
// count is above zero, the words of the four UI-SPEC examples, the numbers
// read from the server's recent object and never invented, and the empty
// Active tab choosing the history block only for an unfiltered Active tab.
func TestRecentSentence(t *testing.T) {
	feed := pt12Feed(t)

	sentence := jsFunctionBody(t, feed, "recentSentence(recent)")
	p13MustContain(t, "recentSentence", sentence,
		"recent.resolved_24h", "recent.expired_24h", "Number.isInteger(", "> 0",
		"' was resolved'", "' were resolved'", "' faded out'", "' and '", "' in the last 24 hours.'")
	p13MustNotContain(t, "recentSentence", sentence, "Math.", "parseInt(", "innerHTML")

	render := jsFunctionBody(t, feed, "render(state)")
	p13Ordered(t, render, "render history kind",
		"kind = 'filter';", "kind = 'disputed';", "Pinalert.state.view === 'active'",
		"recentSentence(Pinalert.state.recent)", "kind = 'history';",
		"Pinalert.setText(historyEmptyBodyEl,", "setEmptyState(kind);")
	p13MustContain(t, "render", render, "' ' + REPORT_PROMPT")
	p13MustContain(t, "feed.js", feed, `var REPORT_PROMPT = "Tap + to report what you're seeing.";`)
	p13MustContain(t, "feed.js", feed,
		"seePastBtn.addEventListener('click', function () {", "switchTab('past');")
}

// TestFocusSelectsTabWithoutAddressWrite pins D-04 and D-11: a report link is
// acted on once per link, the two plain toasts say what the server said, a
// readable report selects the tab of its own state through the store, the
// strip and a refetch, and nothing in that path touches the address.
//
// Test change on purpose (03.1 review fix CR-01): the guard was pinned as
// "focus === handledFocus" (once per answer). Every answer now carries a new
// focus object while the link is held, so the guard is "handledFocus !== null"
// (once per link), which keeps the toast and the tab choice from repeating.
func TestFocusSelectsTabWithoutAddressWrite(t *testing.T) {
	feed := pt12Feed(t)

	handle := jsFunctionBody(t, feed, "handleFocus(state)")
	p13Ordered(t, handle, "handleFocus",
		"handledFocus !== null", "handledFocus = focus;", "focus.status === 'too_old'", "Pinalert.showToast(",
		"focus.status === 'unavailable'", "focus.status === 'ok'", "selectTabOnScreen(")
	p13MustNotContain(t, "handleFocus", handle,
		"replaceState", "syncViewURL", "searchParams", "location.", "switchTab(", "fetch(")

	for _, words := range []string{
		`"That report is older than 7 days and is no longer shown."`,
		`"That report isn't available."`,
	} {
		if n := strings.Count(feed, words); n != 1 {
			t.Errorf("feed.js holds %s %d times, want exactly 1", words, n)
		}
	}

	p13MustContain(t, "feed.js", feed,
		"resolved: 'past'", "past: 'past'", "disputed: 'disputed'", "active: 'active'")

	on := jsFunctionBody(t, feed, "selectTabOnScreen(view)")
	p13Ordered(t, on, "selectTabOnScreen",
		"Pinalert.setView(view);", "showTab(view);", "Pinalert.fetchReports();")
	p13MustNotContain(t, "selectTabOnScreen", on, "replaceState", "syncViewURL", "searchParams")

	// switchTab keeps its own order, the address write included.
	sw := jsFunctionBody(t, feed, "switchTab(view)")
	p13Ordered(t, sw, "switchTab",
		"Pinalert.setView(view);", "showTab(view);", "syncViewURL(view);", "Pinalert.fetchReports();")

	// The link is acted on after every list draw of the ok path: the live
	// list, the empty list and the Past tab, and never on loading or error.
	render := jsFunctionBody(t, feed, "render(state)")
	if n := strings.Count(render, "handleFocus(state);"); n != 3 {
		t.Errorf("render calls handleFocus %d times, want 3 (live list, empty list, Past tab)", n)
	}
	errorBranch := render[strings.Index(render, "if (state.status === 'error') {"):strings.Index(render, "setHidden(errorEl, true);\n    updateFeedStatus(state);")]
	if strings.Contains(errorBranch, "handleFocus(") {
		t.Errorf("the error branch must not act on a link")
	}
}

// pt12FeedRules parses feed.css alone.
func pt12FeedRules(t *testing.T) []p16CSSRule {
	t.Helper()
	return p16ParseCSS("static/css/feed.css", p16ReadStatic(t, "static/css/feed.css"))
}

// TestPastCardCSS pins UI-SPEC Surface 3 and the Typography rows for the group
// labels: a white card with a 1px border and an 8px corner, no left bar, no
// tint and no severity colour, a two line clamped bold title, a muted meta
// line and a text colour facts line, uppercase group labels with 0.04em of
// letter spacing, a muted cap line, rings that sit inside the card, and
// tokens only. The card is the same at every width, so no media rule styles it.
func TestPastCardCSS(t *testing.T) {
	rules := pt12FeedRules(t)

	card := p16Rule(t, rules, "", ".past-card")
	p16MustDecl(t, ".past-card", card, map[string]string{
		"background":    "var(--color-bg)",
		"border":        "1px solid var(--color-border)",
		"border-radius": "8px",
		"padding":       "calc(var(--space-md) - 1px)",
		"cursor":        "pointer",
	})
	for name := range card.Decls {
		if strings.HasPrefix(name, "border-left") || name == "box-shadow" || name == "--severity-tint" {
			t.Errorf(".past-card declares %s: the card has no left bar, no shadow and no tint", name)
		}
	}

	p16MustDecl(t, ".past-card:focus-visible", p16Rule(t, rules, "", ".past-card:focus-visible"),
		map[string]string{"outline-offset": "-2px"})
	p16MustDecl(t, ".past-card--selected", p16Rule(t, rules, "", ".past-card--selected"),
		map[string]string{"outline": "2px solid var(--color-focus-ring)", "outline-offset": "-2px"})

	p16MustDecl(t, ".past-card__title", p16Rule(t, rules, "", ".past-card__title"), map[string]string{
		"margin":             "0",
		"font-size":          "var(--font-size-body)",
		"font-weight":        "var(--font-weight-semibold)",
		"color":              "var(--color-text)",
		"-webkit-line-clamp": "2",
		"line-clamp":         "2",
		"overflow-wrap":      "anywhere",
		"overflow":           "hidden",
		"display":            "-webkit-box",
		"-webkit-box-orient": "vertical",
	})
	p16MustDecl(t, ".past-card__meta", p16Rule(t, rules, "", ".past-card__meta"), map[string]string{
		"margin":      "var(--space-xs) 0 0",
		"font-size":   "var(--font-size-label)",
		"font-weight": "var(--font-weight-regular)",
		"color":       "var(--color-text-muted)",
	})
	p16MustDecl(t, ".past-card__facts", p16Rule(t, rules, "", ".past-card__facts:not([hidden])"), map[string]string{
		"margin":      "var(--space-xs) 0 0",
		"font-size":   "var(--font-size-label)",
		"font-weight": "var(--font-weight-regular)",
		"color":       "var(--color-text)",
	})
	p16MustDecl(t, ".past-card__outcome", p16Rule(t, rules, "", ".past-card__outcome"),
		map[string]string{"font-weight": "var(--font-weight-semibold)"})

	p16MustDecl(t, ".past-group__list", p16Rule(t, rules, "", ".past-group__list"),
		map[string]string{"gap": "var(--space-sm)", "list-style": "none", "margin": "0", "padding": "0"})
	p16MustDecl(t, ".past-group__label", p16Rule(t, rules, "", ".past-group__label"), map[string]string{
		"margin":         "var(--space-md) 0 var(--space-sm)",
		"font-size":      "var(--font-size-label)",
		"font-weight":    "var(--font-weight-semibold)",
		"text-transform": "uppercase",
		"letter-spacing": "0.04em",
		"color":          "var(--color-text-muted)",
	})
	p16MustDecl(t, ".past-group__cut", p16Rule(t, rules, "", ".past-group__cut"), map[string]string{
		"font-size": "var(--font-size-label)",
		"color":     "var(--color-text-muted)",
	})

	// No colour literal and no media rule in anything this plan styles.
	for _, r := range rules {
		if !strings.Contains(r.Head, "past-card") && !strings.Contains(r.Head, "past-group") {
			continue
		}
		if r.Media != "" {
			t.Errorf("%q sits in the media query %q: the Past card is the same at every width", r.Head, r.Media)
		}
		for name, value := range r.Decls {
			if strings.Contains(value, "#") || strings.Contains(value, "rgb(") || strings.Contains(value, "hsl(") {
				t.Errorf("%q declares %s: %q, a colour literal where a token belongs", r.Head, name, value)
			}
		}
	}

	css := p16ReadStatic(t, "static/css/feed.css")
	if n := strings.Count(css, "letter-spacing: 0.04em"); n != 1 {
		t.Errorf("feed.css holds letter-spacing: 0.04em %d times, want exactly 1 (the group label)", n)
	}
}

// TestPastCardContrastAcrossThemes resolves the colours the Past card, its
// group labels and its cap line really declare and checks each against the
// white card or page background, in all four theme sources. Muted text is
// only used on --color-bg.
func TestPastCardContrastAcrossThemes(t *testing.T) {
	needed := []string{"--color-bg", "--color-text", "--color-text-muted"}
	light1, light2, dark1, dark2 := extractThemeTokens(parseCSSRules(p13ReadStatic(t, "static/css/main.css")), needed)
	sources := []struct {
		name   string
		tokens themeTokens
	}{
		{":root", light1}, {`:root[data-theme="light"]`, light2},
		{"prefers dark :root", dark1}, {`:root[data-theme="dark"]`, dark2},
	}

	rules := pt12FeedRules(t)
	colour := func(head string) string {
		r := p16Rule(t, rules, "", head)
		v, ok := r.Decls["color"]
		if !ok {
			t.Fatalf("%q declares no color", head)
		}
		return v
	}
	background := p16Rule(t, rules, "", ".past-card").Decls["background"]
	pairs := []struct{ name, fg, bg string }{
		{"card meta line on the card", colour(".past-card__meta"), background},
		{"card facts line on the card", colour(".past-card__facts:not([hidden])"), background},
		{"card title on the card", colour(".past-card__title"), background},
		{"group label on the page", colour(".past-group__label"), "var(--color-bg)"},
		{"cap line on the page", colour(".past-group__cut"), "var(--color-bg)"},
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
