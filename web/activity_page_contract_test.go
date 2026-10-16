package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds the static contract tests of the My activity page. It began as
// quick task 261013-wm5's six tab page tests and was amended 2026-10-16 by Phase
// 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21): the six tabs became five parts,
// the tab script was removed, and the stylesheet became the laptop stack with
// aligned columns. The helper and test names keep the wm5 prefix where they were
// kept. Nothing here needs a database or a server: the tests read the embedded
// template, the script and the stylesheets and inspect the shipped text, like the
// other contract files. The Phase 3.1 and Phase 4 hands-on tests are walked on
// this layout after Phase 3.4 is built.

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
// order, the page heading and the data hooks of the page, the hub of five part
// links, the five sections written by a range over the parts, the column head
// row, the kept data attributes of the card and its six cells, no tab strip, no
// email, no back link, no Log out form, no link or button inside a card (W-08),
// and one empty state per part.
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
		`/static/js/account-menu.js?v={{.AssetVersion}}" defer`)
	for _, gone := range []string{
		"activity-tabs", `class="profile-logout"`, "profile-email", "profile-back-link", "activity-subtitle",
		"{{.Email}}", "data-tabs=", "data-tab-empty", "activity-chip__word",
	} {
		if strings.Contains(tmpl, gone) {
			t.Errorf("profile.html.tmpl still holds %q", gone)
		}
	}

	mainTag := p25OpeningTag(t, tmpl, `<main class="profile-page"`)
	for _, want := range []string{`data-account-id="{{.AccountID}}"`, `data-part="{{.CurrentPart}}"`} {
		if !strings.Contains(mainTag, want) {
			t.Errorf("the main tag %q does not contain %s", mainTag, want)
		}
	}
	if !strings.Contains(tmpl, `<h1 class="activity-heading">My activity</h1>`) {
		t.Errorf("the page heading My activity is missing")
	}

	hub := regexp.MustCompile(`(?s)<nav class="activity-hub" aria-label="My activity">(.*?)</nav>`).FindStringSubmatch(tmpl)
	if hub == nil {
		t.Fatal(`profile.html.tmpl has no <nav class="activity-hub" aria-label="My activity">`)
	}
	p13Ordered(t, hub[1], "hub", `{{range .Parts}}`, `class="activity-hub__row"`, `href="{{.Href}}"`,
		`activity-hub__tile`, `app-icon--{{.Icon}}`, `activity-hub__name">{{.Name}}`, `activity-hub__summary">{{.Summary}}`,
		`{{if .ShowCount}}`, `activity-hub__count">{{.Count}}`, `app-icon--chevron-right`)

	// The five sections come from one range over the parts, in the order the Go
	// list gives; TestActivityPartsRenderFromTheAddress counts the rendered ids.
	section := regexp.MustCompile(`<section class="activity-part activity-part--\{\{\.Key\}\}" id="part-\{\{\.Key\}\}" data-part-key="\{\{\.Key\}\}"`)
	if n := len(section.FindAllString(tmpl, -1)); n != 1 {
		t.Errorf("the template writes the part section %d times, want once inside the range", n)
	}
	if !strings.Contains(tmpl, `{{if .Current}} data-current{{end}}`) {
		t.Errorf("the part section does not write data-current from .Current")
	}
	p13Ordered(t, tmpl, "part head", `id="activity-title-{{.Key}}">{{.Name}}{{if .ShowCount}} <span class="activity-part__count">{{.Count}}</span>{{end}}</h2>`, `activity-part__line">{{.Line}}`)

	// The column head row and the six cells of a row, in the order the grid
	// columns give them.
	p13Ordered(t, tmpl, "column head row", `class="activity-cols"`, `>Report<`, `>Status<`, `>What happened<`, `>When<`, `>Distance<`)
	p13Ordered(t, tmpl, "row cells",
		`<li class="report-row`, `class="icon-badge activity-card__icon"`, `class="activity-card__report"`,
		`class="activity-card__status"`, `class="activity-card__what"`, `class="activity-card__when"`,
		`class="activity-card__distance"`, `</li>`)
	if !strings.Contains(tmpl, `data-distance-for="{{.ReportID}}" data-lat="{{.Lat}}" data-lon="{{.Lon}}">Unknown</p>`) {
		t.Errorf("the Distance cell does not carry data-distance-for, data-lat and data-lon and the word Unknown")
	}

	opening := wm5CardOpeningTag(t, tmpl)
	for _, want := range []string{`activity-card--{{.Kind}}`, `data-report-id="{{.ReportID}}"`, `data-trust="{{.TrustJSON}}"`, `data-can-reopen="true"`} {
		if !strings.Contains(opening, want) {
			t.Errorf("the card opening tag does not contain %s", want)
		}
	}
	if strings.Contains(opening, "hidden") {
		t.Errorf("the card opening tag still carries a hidden attribute, no card starts hidden now")
	}

	card := regexp.MustCompile(`(?s)<li class="report-row.*?</li>`).FindString(tmpl)
	if card == "" {
		t.Fatal("could not isolate the card markup")
	}
	// W-08 stands: nothing opens from a card. Plan 04-16 (D-24) allows exactly
	// one exception, the Policy link inside the removed report's note, so a
	// removed row can point to the Policy page when a contact address is set.
	// That one anchor is cut out below; any other link in a card still fails.
	allowed := regexp.MustCompile(`(?s)<p class="report-row__moderation-note">.*?</p>`)
	for _, note := range allowed.FindAllString(card, -1) {
		if n := strings.Count(note, "<a "); n > 1 || (n == 1 && strings.Count(note, `<a href="/policy">`) != 1) {
			t.Errorf("the moderation note holds a link other than the one Policy link (W-08, D-24): %s", note)
		}
	}
	rest := allowed.ReplaceAllString(card, "")
	if strings.Contains(rest, "<a ") || strings.Contains(rest, "<a>") {
		t.Errorf("the card holds a link outside the moderation note, nothing opens from a card yet (W-08)")
	}
	if strings.Contains(card, "<button") {
		t.Errorf("the template card holds a button, the script mounts Reopen")
	}

	// One empty state per part, written once inside the range, with the words
	// the Go list owns; Ended nearby has its own shell with six states.
	if n := strings.Count(tmpl, `<h3>{{.EmptyTitle}}</h3>`); n != 2 {
		t.Errorf("the template writes the part empty title %d times, want 2 (the row parts and Ended nearby)", n)
	}
	if n := strings.Count(tmpl, `<p>{{.EmptyText}}</p>`); n != 2 {
		t.Errorf("the template writes the part empty text %d times, want 2 (the row parts and Ended nearby)", n)
	}
	for _, id := range []string{
		`id="ended-nearby"`, `id="ended-skeleton"`, `id="ended-resolved"`, `id="ended-faded"`,
		`id="ended-empty" hidden`, `id="ended-location" hidden`, `id="ended-error" hidden`,
	} {
		if !strings.Contains(tmpl, id) {
			t.Errorf("the Ended nearby shell lacks %s", id)
		}
	}
	for _, words := range []string{
		"Ended nearby needs your location", "Open Nearby and allow location, then come back.",
		"Couldn't load reports", "Check your connection and try again.", "Try again",
	} {
		if n := strings.Count(tmpl, words); n != 1 {
			t.Errorf("the Ended nearby shell holds %q %d times, want 1", words, n)
		}
	}
	// Only the skeleton is visible in the shell.
	if strings.Contains(tmpl[strings.Index(tmpl, `id="ended-skeleton"`):], `id="ended-skeleton" hidden`) {
		t.Errorf("the Ended nearby skeleton starts hidden")
	}
}

// TestActivityScriptKeepsTheBansOfTheRemovedTabScript replaces
// TestActivityTabsScriptContract. The tab script is removed with the six tabs,
// so the contract has no file to read; the bans it held (no markup sink, no
// network call of its own, no scrolling of the page, no Date) are kept as a check
// over activity.js, the one script of the page.
func TestActivityScriptKeepsTheBansOfTheRemovedTabScript(t *testing.T) {
	if _, err := StaticFS.ReadFile("static/js/activity-tabs.js"); err == nil {
		t.Errorf("the removed tab script is still shipped")
	}
	code := p13CodeLines(p16ReadStatic(t, "static/js/activity.js"))
	p13MustNotContain(t, "activity.js", code,
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "createContextualFragment", "DOMParser",
		"fetch(", "XMLHttpRequest", "sendBeacon", "new Date",
		"scrollIntoView", "window.scroll", "scrollTo(", "scrollBy(", "scrollingElement", "location.href =", "pushState", "replaceState")
}

// TestActivityStylesheetContract pins activity.css: the hidden override outside
// every media query, the hub and heading hidden by default, the laptop stack
// inside the 900px block, the row grid shared by the head row and the rows, the
// radius of the box, no important flag and no clamp on a title.
func TestActivityStylesheetContract(t *testing.T) {
	raw := p16ReadStatic(t, "static/css/activity.css")
	rules := wm5ActivityRules(t)

	if strings.Contains(raw, "!important") {
		t.Errorf("activity.css uses the important flag")
	}
	if strings.Contains(raw, "line-clamp") {
		t.Errorf("activity.css clamps a title, a long word must wrap and a title is never clamped (A55)")
	}
	if !strings.Contains(raw, "minmax(0, 2.2fr)") {
		t.Errorf("activity.css does not hold the Report column track minmax(0, 2.2fr)")
	}

	for _, head := range []string{
		"main.profile-page .activity-box [hidden]",
		"main.profile-page > .activity-hub",
		"main.profile-page > .activity-heading",
	} {
		r := wm5CSSRule(t, rules, "", head)
		if r.Decls["display"] != "none" {
			t.Errorf("%s display is %q, want none outside every media query", head, r.Decls["display"])
		}
	}

	// The laptop stack lives only inside the 900px block.
	for _, head := range []string{
		"main.profile-page > .activity-heading",
		"main.profile-page .activity-part",
		"main.profile-page .activity-box",
		"main.profile-page .activity-cols",
		"main.profile-page .profile-report-list > .activity-card",
	} {
		wm5CSSRule(t, rules, wm5LaptopMedia, head)
	}
	for _, head := range []string{"main.profile-page .activity-part", "main.profile-page .activity-cols"} {
		for _, r := range rules {
			if r.Head == head && r.Media == "" {
				t.Errorf("%s is also declared outside the 900px block, the laptop look must not reach phones", head)
			}
		}
	}

	part := wm5CSSRule(t, rules, wm5LaptopMedia, "main.profile-page .activity-part")
	if part.Decls["grid-template-columns"] != "260px minmax(0, 1fr)" || part.Decls["gap"] != "40px" {
		t.Errorf("a part is %q with gap %q, want 260px minmax(0, 1fr) and 40px", part.Decls["grid-template-columns"], part.Decls["gap"])
	}
	if !strings.Contains(part.Decls["border-top"], "1px solid var(--color-border)") {
		t.Errorf("a part has border-top %q, want the hairline", part.Decls["border-top"])
	}
	box := wm5CSSRule(t, rules, wm5LaptopMedia, "main.profile-page .activity-box")
	if box.Decls["border-radius"] != "12px" || box.Decls["overflow"] != "hidden" {
		t.Errorf("the box has radius %q and overflow %q, want 12px and hidden", box.Decls["border-radius"], box.Decls["overflow"])
	}

	// The head row and the rows share one grid, so the columns line up.
	const columns = "40px minmax(0, 2.2fr) 112px minmax(0, 1.5fr) 152px 72px"
	head := wm5CSSRule(t, rules, wm5LaptopMedia, "main.profile-page .activity-cols")
	row := wm5CSSRule(t, rules, wm5LaptopMedia, "main.profile-page .profile-report-list > .activity-card")
	if head.Decls["grid-template-columns"] != columns {
		t.Errorf("the head row columns are %q, want %q", head.Decls["grid-template-columns"], columns)
	}
	if row.Decls["grid-template-columns"] != columns {
		t.Errorf("the row columns are %q, want the head row's %q", row.Decls["grid-template-columns"], columns)
	}
	if head.Decls["gap"] != row.Decls["gap"] || head.Decls["gap"] != "var(--space-md)" {
		t.Errorf("the head row gap %q and the row gap %q must both be var(--space-md)", head.Decls["gap"], row.Decls["gap"])
	}
	if head.Decls["padding"] == row.Decls["padding"] {
		// Not an error in itself, but the spec gives the two different padding.
		t.Errorf("the head row and the rows have the same padding %q, the spec gives 8px and 16px", head.Decls["padding"])
	}
	if _, ok := row.Decls["height"]; ok {
		t.Errorf("the row has a fixed height, the trust facts and Reopen need room")
	}

	// The chip and the disc follow the row's state class, so they follow a
	// Reopen with no script.
	for _, h := range []string{
		"main.profile-page .activity-card.sev-critical .activity-chip",
		"main.profile-page .activity-card.sev-medium .activity-chip",
		"main.profile-page .activity-card.sev-low .activity-chip, main.profile-page .activity-card.vis-retracted .activity-chip",
		"main.profile-page .activity-card.vis-hidden .activity-chip, main.profile-page .activity-card.vis-expired .activity-chip, main.profile-page .activity-card.vis-held .activity-chip, main.profile-page .activity-card.vis-removed .activity-chip",
	} {
		wm5CSSRule(t, rules, "", h)
	}
}

// wm5Pair is one text pair of the page. Both names are custom properties of
// activity.css or tokens of main.css.
type wm5Pair struct{ fg, bg string }

// TestActivityContrast checks every chip text pair of the Activity page at 4.5 to
// 1 in both themes, plus the grey chip and the muted lines on the page colour.
// Light values come from the hex values of activity.css. Dark values come from the
// app tokens each variable maps to in main.css.
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
		if strings.HasPrefix(name, "--a-") {
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
		{"--a-chip-critical-ink", "--a-chip-critical-bg"},
		{"--a-chip-medium-ink", "--a-chip-medium-bg"},
		{"--a-chip-low-ink", "--a-chip-low-bg"},
		// The grey chip and the grey disc: the text token on the surface token.
		{"--color-text", "--color-surface"},
		// The muted lines sit on the page colour only.
		{"--color-text-muted", "--color-bg"},
		{"--color-text", "--color-bg"},
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
	}
}

// TestActivityNoOpacity pins D-07: no row, card, text or chip of My activity is
// faded. No declaration in activity.css, and none in the phone My activity
// section of phone.css, sets an opacity below 1.
func TestActivityNoOpacity(t *testing.T) {
	opacity := regexp.MustCompile(`(?m)\bopacity\s*:\s*([^;}]+)`)
	check := func(name, text string) {
		t.Helper()
		for _, m := range opacity.FindAllStringSubmatch(text, -1) {
			v := strings.TrimSpace(m[1])
			if v != "1" && v != "1.0" && v != "100%" {
				t.Errorf("%s sets opacity %s, a disputed or expired row is marked by its grey disc and chip, never by fading (D-07)", name, v)
			}
		}
	}
	check("activity.css", p16ReadStatic(t, "static/css/activity.css"))

	phone := p16ReadStatic(t, "static/css/phone.css")
	start := strings.Index(phone, "/* 03.4 my activity")
	end := strings.Index(phone, "/* ---- The report detail")
	if start == -1 || end == -1 || end < start {
		t.Fatalf("phone.css has no /* 03.4 my activity section before the report detail section")
	}
	check("the phone My activity section", phone[start:end])
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
