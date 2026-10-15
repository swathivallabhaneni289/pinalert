package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds the static contract tests of quick task 261013-wm5, the six
// tab Activity page. Every helper and test name carries the wm5 prefix. Nothing
// here needs a database or a server: the tests read the embedded template, the
// script and the stylesheets and inspect the shipped text, like the other
// contract files. The departures from the owner's picture and from the house
// rules are written in .planning/sketches/004-activity-page/README.md.

const wm5LaptopMedia = "@media (min-width: 900px)"

// wm5CSSRule finds the first rule of a file with the exact head and the exact
// media prelude, failing the test when there is none.
func wm5CSSRule(t *testing.T, rules []p16CSSRule, media, head string) p16CSSRule {
	t.Helper()
	for _, r := range rules {
		if r.Head == head && r.Media == media {
			return r
		}
	}
	t.Fatalf("no rule with head %q in media %q", head, media)
	return p16CSSRule{}
}

func wm5ActivityRules(t *testing.T) []p16CSSRule {
	t.Helper()
	return p16ParseCSS("static/css/activity.css", p16ReadStatic(t, "static/css/activity.css"))
}

// wm5CardOpeningTag returns the opening tag of the card li in the template.
func wm5CardOpeningTag(t *testing.T, tmpl string) string {
	t.Helper()
	start := strings.Index(tmpl, `<li class="report-row`)
	if start == -1 {
		t.Fatal("profile.html.tmpl has no report row li")
	}
	end := strings.Index(tmpl[start:], ">")
	if end == -1 {
		t.Fatal("the report row li has no closing angle bracket")
	}
	return tmpl[start : start+end]
}

// TestActivityPageTemplateMarkup pins the template: the stylesheet and script
// order, the tab nav and the hooks each link carries, the card's tab and hidden
// hooks inside its opening tag, the three chip words, no link inside a card
// (W-08), and one empty state per tab written by a range.
func TestActivityPageTemplateMarkup(t *testing.T) {
	tmpl := p13ReadTemplate(t, "profile.html.tmpl")

	p13Ordered(t, tmpl, "stylesheets",
		`/static/css/auth.css?v={{.AssetVersion}}`,
		`/static/css/activity.css?v={{.AssetVersion}}`,
		`/static/css/phone.css?v={{.AssetVersion}}`)
	if n := strings.Count(tmpl, "/static/css/activity.css"); n != 1 {
		t.Errorf("profile.html.tmpl links activity.css %d times, want 1", n)
	}

	p13Ordered(t, tmpl, "scripts",
		`/static/js/activity.js?v={{.AssetVersion}}" defer`,
		`/static/js/activity-tabs.js?v={{.AssetVersion}}" defer`,
		`/static/js/account-menu.js?v={{.AssetVersion}}" defer`)

	nav := regexp.MustCompile(`(?s)<nav class="activity-tabs" aria-label="Activity views">(.*?)</nav>`).FindStringSubmatch(tmpl)
	if nav == nil {
		t.Fatal(`profile.html.tmpl has no <nav class="activity-tabs" aria-label="Activity views">`)
	}
	p13Ordered(t, nav[1], "nav", `{{range .Tabs}}`,
		`class="activity-tab"`, `href="{{.Href}}"`, `data-tab="{{.Key}}"`, `data-count="{{.Count}}"`,
		`{{if .Selected}} aria-current="page"{{end}}`,
		`<span class="activity-tab__label">{{.Label}}</span>`, `<span class="activity-tab__count">{{.Count}}</span>`)

	opening := wm5CardOpeningTag(t, tmpl)
	for _, want := range []string{`data-tabs="{{.Tabs}}"`, `{{if .Hidden}}hidden{{end}}`, `activity-card--{{.Kind}}`} {
		if !strings.Contains(opening, want) {
			t.Errorf("the card opening tag does not contain %s", want)
		}
	}

	for _, word := range []string{
		`activity-chip__word activity-chip__word--severity">{{.SeverityWord}}`,
		`activity-chip__word activity-chip__word--resolved">Resolved`,
		`activity-chip__word activity-chip__word--open">Not resolved`,
	} {
		if n := strings.Count(tmpl, word); n != 1 {
			t.Errorf("chip word %q occurs %d times, want 1", word, n)
		}
	}

	card := regexp.MustCompile(`(?s)<li class="report-row.*?</li>`).FindString(tmpl)
	if card == "" {
		t.Fatal("could not isolate the card markup")
	}
	if strings.Contains(card, "<a ") || strings.Contains(card, "<a>") {
		t.Errorf("the card holds a link, nothing opens from a card yet (W-08)")
	}
	if strings.Contains(card, "<button") {
		t.Errorf("the template card holds a button, the script mounts Reopen")
	}

	rangeAt := strings.LastIndex(tmpl, "{{range .Tabs}}")
	emptyAt := strings.Index(tmpl, `data-tab-empty="{{.Key}}"`)
	if rangeAt == -1 || emptyAt == -1 || rangeAt > emptyAt {
		t.Errorf("the empty states are not written by a range over .Tabs (range at %d, empty at %d)", rangeAt, emptyAt)
	}
	if n := strings.Count(tmpl, "data-tab-empty="); n != 1 {
		t.Errorf("data-tab-empty is written %d times, want once inside the range", n)
	}
	for _, want := range []string{`{{if not .ShowEmpty}} hidden{{end}}`, `{{.EmptyTitle}}`, `{{.EmptyText}}`} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("the empty state does not contain %s", want)
		}
	}

	if !strings.Contains(tmpl, `<p class="activity-subtitle">A quick view of what's happening with your reports.</p>`) {
		t.Errorf("the help line is missing or reworded")
	}
}

// TestActivityTabsScriptContract pins activity-tabs.js (W-11, T-wm5-04): one
// click listener, the address replaced and never pushed, only server written
// data read, no markup sink, no network call, and scrolling limited to the
// strip's own scroll position.
func TestActivityTabsScriptContract(t *testing.T) {
	raw := p16ReadStatic(t, "static/js/activity-tabs.js")
	code := p13CodeLines(raw)

	if n := strings.Count(raw, "addEventListener"); n != 1 {
		t.Errorf("activity-tabs.js has %d addEventListener occurrences, want exactly 1", n)
	}
	if !strings.Contains(code, "window.history.replaceState(null, '', link.href)") {
		t.Errorf("activity-tabs.js must call history.replaceState with the link's server built address")
	}
	if strings.Contains(code, "pushState") {
		t.Errorf("activity-tabs.js uses the push form of the history API, a tab choice adds no Back steps")
	}
	p13MustContain(t, "activity-tabs.js", code,
		"dataset.tabs", "dataset.tab", "dataset.count", "dataset.tabEmpty", "link.href",
		"evt.button !== 0", "evt.metaKey", "evt.ctrlKey", "evt.shiftKey", "evt.altKey", "evt.preventDefault()",
		"nav.scrollLeft")
	p13MustNotContain(t, "activity-tabs.js", code,
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "createContextualFragment", "DOMParser",
		"fetch(", "XMLHttpRequest", "sendBeacon", "Pinalert", "Date", "new Date",
		"scrollIntoView", "window.scroll", "scrollTo(", "scrollBy(", "scrollingElement", "location.href =")
	if n := strings.Count(code, "history."); n != 1 {
		t.Errorf("activity-tabs.js touches the history object %d times, want 1", n)
	}
}

// TestActivityStylesheetContract pins activity.css: the hidden overrides
// outside every media query, the laptop layout inside the 900px block, 44px
// tabs, the drawn corners at their calc values, no important flag and no
// clamp on a title.
func TestActivityStylesheetContract(t *testing.T) {
	raw := p16ReadStatic(t, "static/css/activity.css")
	rules := wm5ActivityRules(t)

	if !strings.Contains(raw, "--u: 0.8px") {
		t.Errorf("activity.css does not declare the one unit variable --u: 0.8px")
	}
	if strings.Contains(raw, "!important") {
		t.Errorf("activity.css uses the important flag")
	}
	if strings.Contains(raw, "line-clamp") {
		t.Errorf("activity.css clamps a title, a long word must wrap and a title is never clamped (A55)")
	}

	for _, head := range []string{
		"main.profile-page .profile-report-list > .report-row[hidden]",
		"main.profile-page .profile-activity > .activity-empty[hidden]",
	} {
		r := wm5CSSRule(t, rules, "", head)
		if r.Decls["display"] != "none" {
			t.Errorf("%s display is %q, want none outside every media query", head, r.Decls["display"])
		}
	}

	// Laptop layout lives only inside the 900px block.
	for _, head := range []string{
		"main.profile-page .profile-activity h2",
		"main.profile-page .activity-tabs",
		"main.profile-page .activity-tab",
		"main.profile-page .activity-tab[aria-current=\"page\"]",
		"main.profile-page .profile-report-list > .activity-card",
		"main.profile-page .activity-card .activity-chip",
		"main.profile-page .profile-record",
	} {
		wm5CSSRule(t, rules, wm5LaptopMedia, head)
		for _, r := range rules {
			// The card head also carries the all widths cursor rule and nothing
			// else outside the block.
			if r.Head == head && r.Media == "" && !(len(r.Decls) == 1 && r.Decls["cursor"] == "default") {
				t.Errorf("%s is also declared outside the 900px block, the laptop look must not reach phones", head)
			}
		}
	}

	tab := wm5CSSRule(t, rules, wm5LaptopMedia, "main.profile-page .activity-tab")
	if tab.Decls["min-height"] != "44px" {
		t.Errorf("tab min-height is %q, want 44px (W-14)", tab.Decls["min-height"])
	}

	// The drawn corners, pinned at their calc values.
	for _, c := range []struct{ head, radius string }{
		{"main.profile-page .profile-report-list > .activity-card", "calc(14 * var(--u))"},
		{"main.profile-page .activity-tab", "calc(16 * var(--u))"},
		{"main.profile-page .activity-card .activity-card__tag", "calc(10 * var(--u))"},
		{"main.profile-page .activity-card .activity-chip", "calc(10 * var(--u))"},
		{"main.profile-page .activity-tab__count", "calc(13 * var(--u))"},
	} {
		r := wm5CSSRule(t, rules, wm5LaptopMedia, c.head)
		if r.Decls["border-radius"] != c.radius {
			t.Errorf("%s border-radius is %q, want %q", c.head, r.Decls["border-radius"], c.radius)
		}
	}
	card := wm5CSSRule(t, rules, wm5LaptopMedia, "main.profile-page .profile-report-list > .activity-card")
	if card.Decls["border-left"] != "calc(2 * var(--u)) solid var(--a-event)" {
		t.Errorf("card bar is %q, want the 2u bar in the event colour", card.Decls["border-left"])
	}
	if card.Decls["min-height"] != "calc(122 * var(--u))" {
		t.Errorf("card min-height is %q, want calc(122 * var(--u)), a card grows past the picture's height", card.Decls["min-height"])
	}
	if _, ok := card.Decls["height"]; ok {
		t.Errorf("the card has a fixed height, the trust facts and Reopen need room")
	}

	// The chip shows exactly one word, chosen by the card's state class.
	words := wm5CSSRule(t, rules, "", "main.profile-page .activity-card .activity-chip__word")
	if words.Decls["display"] != "none" {
		t.Errorf("chip words display is %q, want none until a state class shows one", words.Decls["display"])
	}
	wm5CSSRule(t, rules, "", "main.profile-page .activity-card:not(.vis-retracted):not(.vis-hidden) .activity-chip__word--severity")
	wm5CSSRule(t, rules, "", "main.profile-page .activity-card.vis-retracted .activity-chip__word--resolved")
	wm5CSSRule(t, rules, "", "main.profile-page .activity-card.vis-hidden .activity-chip__word--open")
}

// wm5Pair is one text pair of the page. Both names are custom properties of
// activity.css or tokens of main.css.
type wm5Pair struct{ fg, bg string }

// TestActivityContrast checks every Activity text pair at 4.5 to 1 in both
// themes, and the icon glyph pairs at 3 to 1 (W-14). Light values come from the
// hex values of activity.css. Dark values come from the app tokens each
// variable maps to in main.css.
func TestActivityContrast(t *testing.T) {
	act := wm5ActivityRules(t)
	main := p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css"))

	lightAct := wm5CSSRule(t, act, "", "main.profile-page").Decls
	darkAct := wm5CSSRule(t, act, "", ":root[data-theme=\"dark\"] main.profile-page").Decls
	osDarkAct := wm5CSSRule(t, act, "@media (prefers-color-scheme: dark)", "main.profile-page").Decls
	explicitLight := wm5CSSRule(t, act, "", ":root[data-theme=\"light\"] main.profile-page").Decls
	lightTokens := wm5CSSRule(t, main, "", ":root").Decls
	darkTokens := wm5CSSRule(t, main, "", ":root[data-theme=\"dark\"]").Decls

	// The OS preference block and the explicit dark block hold the same
	// values, and the explicit light block restates the light values.
	for name, want := range darkAct {
		if got := osDarkAct[name]; got != want {
			t.Errorf("OS dark block %s is %q, the explicit dark block has %q", name, got, want)
		}
	}
	for name, want := range explicitLight {
		if got := lightAct[name]; got != want {
			t.Errorf("explicit light block %s is %q, the base block has %q", name, want, got)
		}
	}

	var resolve func(name string, dark bool, depth int) string
	resolve = func(name string, dark bool, depth int) string {
		t.Helper()
		if depth > 4 {
			t.Fatalf("variable %s does not resolve to a hex in %d steps", name, depth)
		}
		var value string
		if strings.HasPrefix(name, "--a-") || name == "--u" {
			value = lightAct[name]
			if dark {
				if v, ok := darkAct[name]; ok {
					value = v
				}
			}
		} else {
			value = lightTokens[name]
			if dark {
				value = darkTokens[name]
			}
		}
		if value == "" {
			t.Fatalf("variable %s has no value (dark=%v)", name, dark)
		}
		if strings.HasPrefix(value, "#") {
			return value
		}
		m := regexp.MustCompile(`^var\((--[a-z0-9-]+)\)$`).FindStringSubmatch(value)
		if m == nil {
			t.Fatalf("variable %s has value %q, want a hex or a bare var()", name, value)
		}
		return resolve(m[1], dark, depth+1)
	}

	text := []wm5Pair{
		{"--a-h1", "--color-bg"},
		{"--a-subtitle", "--color-bg"},
		{"--a-title", "--a-card-bg"},
		{"--a-sub", "--a-card-bg"},
		{"--a-tab", "--color-bg"},
		{"--a-tab", "--a-tab-hover-bg"},
		{"--a-tab-sel-text", "--a-tab-sel-bg"},
		{"--a-count-ink", "--a-count-bg"},
		{"--a-tag-ink", "--a-tag-bg"},
		{"--a-chip-critical-ink", "--a-chip-critical-bg"},
		{"--a-chip-medium-ink", "--a-chip-medium-bg"},
		{"--a-chip-low-ink", "--a-chip-low-bg"},
		{"--a-chip-resolved-ink", "--a-chip-resolved-bg"},
		{"--a-chip-open-ink", "--a-chip-open-bg"},
	}
	for _, dark := range []bool{false, true} {
		theme := "light"
		if dark {
			theme = "dark"
		}
		for _, p := range text {
			fg, bg := resolve(p.fg, dark, 0), resolve(p.bg, dark, 0)
			ratio := wcagContrastRatio(t, fg, bg, p.fg+" on "+p.bg+" ("+theme+")")
			if ratio < 4.5 {
				t.Errorf("%s: %s %s on %s %s is %.2f to 1, want at least 4.5", theme, p.fg, fg, p.bg, bg, ratio)
			}
		}
		for _, ev := range []string{"--a-ev-post", "--a-ev-vote", "--a-ev-dispute"} {
			fg, bg := resolve("--a-icon-ink", dark, 0), resolve(ev, dark, 0)
			ratio := wcagContrastRatio(t, fg, bg, ev+" ("+theme+")")
			if ratio < 3.0 {
				t.Errorf("%s: icon glyph %s on %s %s is %.2f to 1, want at least 3 for a graphic", theme, fg, ev, bg, ratio)
			}
		}
	}
}

// TestActivityUndosUsedSentence pins plan 03.1-07 (D-22, C52). The template
// writes data-undos-used only from UndosUsed, and activity.js turns that hook
// into one fixed sentence written through Pinalert.setText, never mounting the
// Reopen control on such a card. The module keeps its one addEventListener and
// its one showToast call, and makes no network call of its own.
func TestActivityUndosUsedSentence(t *testing.T) {
	tmpl := p13ReadTemplate(t, "profile.html.tmpl")
	if n := strings.Count(tmpl, "data-undos-used"); n != 1 {
		t.Fatalf("profile.html.tmpl writes data-undos-used %d times, want 1", n)
	}
	if !strings.Contains(tmpl, `{{if .UndosUsed}}data-undos-used="true"{{end}}`) {
		t.Errorf("data-undos-used is not written only from UndosUsed")
	}
	if !strings.Contains(wm5CardOpeningTag(t, tmpl), "data-undos-used") {
		t.Errorf("data-undos-used is not inside the card opening tag")
	}

	raw := p15ReadStatic(t, "static/js/activity.js")
	code := p15CodeLines(raw)
	const sentence = "You have used both undos on this report."
	if n := strings.Count(raw, sentence); n != 1 {
		t.Errorf("activity.js holds the sentence %d times, want 1", n)
	}
	if !regexp.MustCompile(`var UNDOS_USED_SENTENCE = '` + regexp.QuoteMeta(sentence) + `';`).MatchString(code) {
		t.Errorf("the sentence is not held in one fixed constant")
	}

	hookAt := strings.Index(code, "row.dataset.undosUsed === 'true'")
	reopenAt := strings.Index(code, "row.dataset.canReopen === 'true'")
	mountAt := strings.Index(code, "PinalertVotes.createReopenControl(row.dataset.reportId)")
	if hookAt == -1 || reopenAt == -1 || mountAt == -1 || !(reopenAt < mountAt && mountAt < hookAt) {
		t.Fatalf("activity.js does not read the undos hook after the Reopen branch (reopen %d, mount %d, hook %d)", reopenAt, mountAt, hookAt)
	}
	if !strings.Contains(code[mountAt:], "} else if (row.dataset.undosUsed === 'true'") {
		t.Errorf("the sentence branch is not an else of the Reopen branch, so Reopen could mount beside it")
	}
	branch := code[hookAt:]
	if !regexp.MustCompile(`Pinalert\.setText\(\s*usedNote\s*,\s*UNDOS_USED_SENTENCE\s*\)`).MatchString(branch) {
		t.Errorf("the sentence is not written through Pinalert.setText")
	}
	for _, banned := range []string{"innerHTML", "insertAdjacentHTML", "textContent ="} {
		if strings.Contains(code, banned) {
			t.Errorf("activity.js contains %q", banned)
		}
	}

	if n := strings.Count(raw, "Pinalert.showToast("); n != 1 {
		t.Errorf("activity.js has %d Pinalert.showToast calls, want 1", n)
	}
	if n := strings.Count(raw, "addEventListener"); n != 1 {
		t.Errorf("activity.js has %d addEventListener occurrences, want 1", n)
	}
	for _, banned := range []string{"fetch(", "XMLHttpRequest", "sendBeacon"} {
		if strings.Contains(code, banned) {
			t.Errorf("activity.js makes a network call of its own: %q", banned)
		}
	}
}
