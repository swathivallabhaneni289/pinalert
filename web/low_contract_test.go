package web

// Contract tests for the Low data view (plan 04-18). They read the shipped
// template, script and stylesheet out of the embedded filesystems, so they
// inspect what the binary serves. Every assertion is written so it still holds
// after the later plans that extend this page (04-19 posting and voting, 04-22
// the queue strip, 04-24 the Flag control): the script and stylesheet sets are
// parsed from the tags, the allowed later additions are listed, and nothing
// here asserts the absence of a vote block or a fixed tag count. Helpers start
// with p418 so they never collide with another file in this package.

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

func p418Static(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, "static/"+path)
	if err != nil {
		t.Fatalf("reading embedded static/%s: %v", path, err)
	}
	return string(raw)
}

func p418Template(t *testing.T) string {
	t.Helper()
	raw, err := fs.ReadFile(TemplatesFS, "templates/low.html.tmpl")
	if err != nil {
		t.Fatalf("reading templates/low.html.tmpl: %v", err)
	}
	return string(raw)
}

var (
	p418StyleLink  = regexp.MustCompile(`<link[^>]*rel="stylesheet"[^>]*href="([^"]+)"`)
	p418ScriptTag  = regexp.MustCompile(`<script[^>]*src="([^"]+)"[^>]*>`)
	p418URLPattern = regexp.MustCompile(`url\(\s*["']?([^"')]+)["']?\s*\)`)
	p418BareL      = regexp.MustCompile(`(^|[^A-Za-z0-9_$.])L\.`)
)

// p418LinkedAssets returns the stylesheet hrefs, the head script srcs and the
// body script srcs of the template, in file order, with the version query cut.
func p418LinkedAssets(t *testing.T, html string) (styles, headScripts, bodyScripts []string) {
	t.Helper()
	headEnd := strings.Index(html, "</head>")
	if headEnd < 0 {
		t.Fatalf("the template has no closing head tag")
	}
	cut := func(s string) string {
		before, _, _ := strings.Cut(s, "?")
		return before
	}
	for _, m := range p418StyleLink.FindAllStringSubmatch(html, -1) {
		styles = append(styles, cut(m[1]))
	}
	for _, loc := range p418ScriptTag.FindAllStringSubmatchIndex(html, -1) {
		src := cut(html[loc[2]:loc[3]])
		if loc[0] < headEnd {
			headScripts = append(headScripts, src)
		} else {
			bodyScripts = append(bodyScripts, src)
		}
	}
	return styles, headScripts, bodyScripts
}

// p418Subsequence checks that the entries of got that belong to canonical are
// exactly canonical, once each and in order.
func p418Subsequence(t *testing.T, label string, got, canonical []string) {
	t.Helper()
	member := map[string]bool{}
	for _, c := range canonical {
		member[c] = true
	}
	var seen []string
	for _, g := range got {
		if member[g] {
			seen = append(seen, g)
		}
	}
	if strings.Join(seen, ",") != strings.Join(canonical, ",") {
		t.Errorf("%s: the required files appear as %v, want %v (once each, in this order)", label, seen, canonical)
	}
}

// p418Tokens splits s on every character that is not a letter or digit and on
// every lower to upper case boundary, and lowercases each token.
func p418Tokens(s string) []string {
	var tokens []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			tokens = append(tokens, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	var prev rune
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			prev = r
			continue
		}
		if unicode.IsUpper(r) && unicode.IsLower(prev) {
			flush()
		}
		cur = append(cur, r)
		prev = r
	}
	flush()
	return tokens
}

func p418HasBannedToken(s string) string {
	for _, tok := range p418Tokens(s) {
		if tok == "light" || tok == "lite" {
			return tok
		}
	}
	return ""
}

// The word boundary keeps .ico from matching a class such as .icon-badge.
var p418RasterExt = regexp.MustCompile(`\.(png|jpe?g|gif|webp|avif|bmp|ico)\b`)
var p418FontExt = regexp.MustCompile(`\.(woff2?|ttf|otf|eot)\b`)

func TestLowDataAssetPolicy(t *testing.T) {
	html := p418Template(t)
	styles, headScripts, bodyScripts := p418LinkedAssets(t, html)

	// Floors: the page links 6 stylesheets and 6 body scripts today (this
	// plan's own list), and a parse that finds fewer has stopped reading the
	// tags rather than found a smaller page.
	if len(styles) < 6 {
		t.Fatalf("found %d linked stylesheets, want at least 6", len(styles))
	}
	if len(bodyScripts) < 6 {
		t.Fatalf("found %d body scripts, want at least 6", len(bodyScripts))
	}

	canonicalStyles := []string{"/static/css/main.css", "/static/css/feed.css", "/static/css/trust.css", "/static/css/auth.css", "/static/css/chrome.css", "/static/css/low.css"}
	p418Subsequence(t, "stylesheets", styles, canonicalStyles)
	allowedStyle := map[string]bool{"/static/css/modal.css": true}
	for _, s := range canonicalStyles {
		allowedStyle[s] = true
	}
	for _, s := range styles {
		if !allowedStyle[s] {
			t.Errorf("stylesheet %s is not allowed on the Low data page", s)
		}
	}

	if len(headScripts) != 1 || headScripts[0] != "/static/js/theme.js" {
		t.Errorf("head scripts = %v, want only /static/js/theme.js", headScripts)
	}
	if strings.Contains(findTagWindow(t, html, "/static/js/theme.js"), " defer") {
		t.Errorf("theme.js must not be deferred")
	}

	canonicalScripts := []string{"/static/js/app.js", "/static/js/votes.js", "/static/js/visibility.js", "/static/js/trust.js", "/static/js/low.js", "/static/js/account-menu.js"}
	p418Subsequence(t, "body scripts", bodyScripts, canonicalScripts)
	index := map[string]int{}
	for i, s := range bodyScripts {
		if _, dup := index[s]; dup {
			t.Errorf("script %s is linked twice", s)
		}
		index[s] = i
	}
	later := map[string]bool{"/static/js/queue.js": true, "/static/js/flag.js": true, "/static/js/report-form.js": true}
	for _, s := range bodyScripts {
		isCanonical := false
		for _, c := range canonicalScripts {
			if c == s {
				isCanonical = true
			}
		}
		if isCanonical {
			continue
		}
		if !later[s] {
			t.Errorf("script %s is not allowed on the Low data page", s)
			continue
		}
		if index[s] < index["/static/js/trust.js"] || index[s] > index["/static/js/low.js"] {
			t.Errorf("script %s must sit between trust.js and low.js", s)
		}
	}

	for _, p := range append(append(append([]string{}, styles...), headScripts...), bodyScripts...) {
		if !strings.HasPrefix(p, "/static/") {
			t.Errorf("linked asset %q is not a local /static/ file", p)
			continue
		}
		if _, err := fs.Stat(StaticFS, "static/"+strings.TrimPrefix(p, "/static/")); err != nil {
			t.Errorf("linked asset %s does not exist in the embedded files", p)
		}
	}

	lowerHTML := strings.ToLower(html)
	for _, banned := range []string{"unpkg", "openfreemap", "preconnect", "<img", "<svg", "<picture", "srcset", "<style", "url(", "@font-face", "leaflet", "maplibre", "map.js", "modal.js", "feed.js", "slow-notice.js", "modal-map.css"} {
		if strings.Contains(lowerHTML, banned) {
			t.Errorf("the template holds %q", banned)
		}
	}
	for _, m := range regexp.MustCompile(`(?:src|href)="([^"]*)"`).FindAllStringSubmatch(html, -1) {
		if strings.HasPrefix(m[1], "http") || strings.HasPrefix(m[1], "//") {
			t.Errorf("the template links the external address %q", m[1])
		}
	}

	targets := 0
	for _, sheet := range styles {
		css := p418Static(t, strings.TrimPrefix(sheet, "/static/"))
		stripped := stripCSSComments(css)
		lower := strings.ToLower(stripped)
		if strings.Contains(lower, "@font-face") || strings.Contains(lower, "image-set(") {
			t.Errorf("%s holds @font-face or image-set(", sheet)
		}
		if m := p418FontExt.FindString(lower); m != "" {
			t.Errorf("%s names a font file %s", sheet, m)
		}
		if m := p418RasterExt.FindString(lower); m != "" {
			t.Errorf("%s names a raster file %s", sheet, m)
		}
		for _, rule := range parseCSSRules(css) {
			for _, m := range p418URLPattern.FindAllStringSubmatch(rule.declBody, -1) {
				if strings.Contains(rule.selectorHead, ".globe-bg__mask") {
					continue
				}
				targets++
				target := m[1]
				if !strings.HasPrefix(target, "/static/icons/") || !strings.HasSuffix(target, ".svg") {
					t.Errorf("%s: url(%s) in %q is not a shipped icon file", sheet, target, rule.selectorHead)
					continue
				}
				if _, err := fs.Stat(StaticFS, "static/"+strings.TrimPrefix(target, "/static/")); err != nil {
					t.Errorf("%s: url(%s) does not exist in the embedded files", sheet, target)
				}
			}
		}
	}
	if targets == 0 {
		t.Errorf("found no url( target in the linked stylesheets; the walk is not reading them")
	}

	for _, script := range append(append([]string{}, headScripts...), bodyScripts...) {
		text := stripJSComments(p418Static(t, strings.TrimPrefix(script, "/static/")))
		for _, banned := range []string{"unpkg", "createElement('img')", "new Image(", "createElement('script')", "importScripts", "import("} {
			if strings.Contains(text, banned) {
				t.Errorf("%s holds %q", script, banned)
			}
		}
		if m := p418RasterExt.FindString(text); m != "" {
			t.Errorf("%s names a raster file %s", script, m)
		}
	}

	low := stripJSComments(p418Static(t, "js/low.js"))
	lowerLow := strings.ToLower(low)
	if strings.Contains(low, "PinalertMap") || strings.Contains(lowerLow, "leaflet") || strings.Contains(lowerLow, "maplibre") {
		t.Errorf("low.js names the map")
	}
	if p418BareL.MatchString(low) {
		t.Errorf("low.js reads a bare L global")
	}
}

func TestLowDataNoBannedWords(t *testing.T) {
	// Self check: the tokenizer must flag the banned words in every spelling
	// and pass the lookalikes.
	for _, bad := range []string{"lowLiteMode", "low_light", "low-lite", "Light", "isLight"} {
		if p418HasBannedToken(bad) == "" {
			t.Errorf("the tokenizer missed %q", bad)
		}
	}
	for _, good := range []string{`aria-live="polite"`, "highlight", "Low data view", "lowDataView", "delight"} {
		if w := p418HasBannedToken(good); w != "" {
			t.Errorf("the tokenizer flagged %q as %q", good, w)
		}
	}

	files := map[string]string{
		"low.html.tmpl": p418Template(t),
		"low.js":        p418Static(t, "js/low.js"),
		"low.css":       p418Static(t, "css/low.css"),
	}
	raw, err := os.ReadFile("../internal/api/handlers/low.go")
	if err != nil {
		t.Fatalf("reading low.go: %v", err)
	}
	files["low.go"] = string(raw)
	for name, text := range files {
		if w := p418HasBannedToken(text); w != "" {
			t.Errorf("%s holds the banned word %q", name, w)
		}
	}
}

func TestLowDataRowsNotInteractive(t *testing.T) {
	text := stripJSComments(p418Static(t, "js/low.js"))

	create := jsFunctionBody(t, text, "createLowRow(report)")
	update := jsFunctionBody(t, text, "updateLowRow(row, report)")
	render := jsFunctionBody(t, text, "renderLowRows(reports)")
	for name, body := range map[string]string{"createLowRow": create, "updateLowRow": update, "renderLowRows": render} {
		for _, banned := range []string{"addEventListener", "onclick", "onkeydown", "tabindex", "tabIndex", "activateRow", "Pinalert.select("} {
			if strings.Contains(body, banned) {
				t.Errorf("%s holds %q; a Low data row is not interactive", name, banned)
			}
		}
	}

	for _, want := range []string{"setAttribute('role', 'listitem')", "'report-row report-row--low'", "PinalertVisibility.createVisibilityTag(", "PinalertTrust.createTrustBlock(", "PinalertTrust.createSeverityLabel("} {
		if !strings.Contains(create, want) {
			t.Errorf("createLowRow is missing %q", want)
		}
	}
	for _, want := range []string{"PinalertVisibility.applyVisibilityClass(", "PinalertVisibility.updateVisibilityTag(", "PinalertTrust.updateTrustBlock(row.trust, report.trust)", "Pinalert.iconClass(", "PinalertTrust.updateSeverityLabel("} {
		if !strings.Contains(update, want) {
			t.Errorf("updateLowRow is missing %q", want)
		}
	}
	if meta := jsFunctionBody(t, text, "buildLowMetaText(report)"); !strings.Contains(meta, "Pinalert.formatShelterLine(report)") {
		t.Errorf("buildLowMetaText does not call Pinalert.formatShelterLine(report)")
	}
	if strings.Contains(text, "CAPACITY_LABELS") {
		t.Errorf("low.js declares a shelter wording map of its own")
	}
	if got := strings.Count(text, "createElement('li')"); got != 1 {
		t.Errorf("low.js creates %d list items, want exactly 1", got)
	}
	if !strings.Contains(create, "createElement('li')") {
		t.Errorf("the one list item is not created inside createLowRow")
	}
}

func TestLowDataOwnFetchBoot(t *testing.T) {
	text := stripJSComments(p418Static(t, "js/low.js"))

	for _, banned := range []string{"fetch(", "XMLHttpRequest", "sendBeacon", "EventSource", "WebSocket"} {
		if strings.Contains(text, banned) {
			t.Errorf("low.js holds %q; the list comes only from the shared store", banned)
		}
	}

	boot := jsFunctionBody(t, text, "boot()")
	subscribeAt := strings.Index(boot, "Pinalert.subscribe(renderLow)")
	centreAt := strings.Index(boot, "Pinalert.setCenter(")
	positionAt := strings.Index(boot, "requestPosition()")
	if subscribeAt < 0 || centreAt < 0 || positionAt < 0 || !(subscribeAt < centreAt && centreAt < positionAt) {
		t.Errorf("boot must subscribe, then set the centre, then ask for the position (subscribe=%d, centre=%d, position=%d)", subscribeAt, centreAt, positionAt)
	}

	show := jsFunctionBody(t, text, "showListFor(lat, lon, own)")
	centreAt = strings.Index(show, "Pinalert.setCenter(lat, lon, own)")
	fetchAt := strings.Index(show, "Pinalert.fetchReports(")
	if centreAt < 0 || fetchAt < 0 || centreAt > fetchAt {
		t.Errorf("showListFor must set the centre before it fetches (centre=%d, fetch=%d)", centreAt, fetchAt)
	}
	for _, want := range []string{"Pinalert.startPolling(", ".then("} {
		if !strings.Contains(show, want) {
			t.Errorf("showListFor is missing %q", want)
		}
	}

	request := jsFunctionBody(t, text, "requestPosition()")
	for _, want := range []string{"navigator.geolocation.getCurrentPosition(", "timeout: LIST_POSITION_TIMEOUT_MS"} {
		if !strings.Contains(request, want) {
			t.Errorf("requestPosition is missing %q", want)
		}
	}
	for _, want := range []string{"LIST_POSITION_TIMEOUT_MS = 8000", "STORED_POSITION_KEY = 'pinalert.lowLastPosition'"} {
		if !strings.Contains(text, want) {
			t.Errorf("low.js is missing %q", want)
		}
	}

	for _, name := range []string{"readStoredPosition()", "storePosition(lat, lon)"} {
		body := jsFunctionBody(t, text, name)
		for _, want := range []string{"try {", "catch", "localStorage"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s is missing %q", name, want)
			}
		}
	}

	found := jsFunctionBody(t, text, "onPositionFound(pos)")
	storeAt := strings.Index(found, "storePosition(")
	showAt := strings.Index(found, "showListFor(")
	if storeAt < 0 || showAt < 0 || storeAt > showAt {
		t.Errorf("onPositionFound must store the position before it shows the list (store=%d, show=%d)", storeAt, showAt)
	}

	if got := strings.Count(text, "'pageshow'"); got != 1 {
		t.Errorf("low.js has %d pageshow listeners, want exactly 1", got)
	}
}

func p418Squash(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func TestLowDataHiddenOverrides(t *testing.T) {
	css := p418Static(t, "css/low.css")
	rules := parseCSSRules(css)
	if len(rules) < 10 {
		t.Fatalf("parsed %d rules from low.css, want at least 10", len(rules))
	}

	const overrideHead = "#low-list[hidden], #low-skeleton[hidden], #low-empty[hidden], #low-error[hidden]"
	foundOverride := false
	for _, r := range rules {
		if p418Squash(r.selectorHead) == overrideHead {
			foundOverride = true
			if declsOf(r.declBody)["display"] != "none" {
				t.Errorf("the override rule does not declare display: none")
			}
		}
	}
	if !foundOverride {
		t.Errorf("low.css has no rule headed %q", overrideHead)
	}

	// Amended 2026-10-21 by plan 44 (owner): the Retry position button is gone and Reload is always shown, so #low-retry-position left the hidden lists; see web/low_toprow_contract_test.go.
	hiddenCapable := []string{"#low-list", "#low-skeleton", "#low-empty", "#low-error", "#low-stale"}
	for _, r := range rules {
		display, has := declsOf(r.declBody)["display"]
		if !has || display == "none" || p418Squash(r.selectorHead) == overrideHead {
			continue
		}
		relevant := strings.Contains(r.selectorHead, ".low-") || strings.Contains(r.selectorHead, ".report-row--low")
		for _, id := range hiddenCapable {
			if strings.Contains(r.selectorHead, id) {
				relevant = true
			}
		}
		if relevant && !strings.Contains(r.selectorHead, ":not([hidden])") {
			t.Errorf("rule %q declares display %q without :not([hidden])", r.selectorHead, display)
		}
	}

	html := p418Template(t)
	for _, id := range []string{"low-skeleton", "low-empty", "low-error", "low-stale"} {
		tag := findTagWindow(t, html, `id="`+id+`"`)
		if !regexp.MustCompile(`\shidden[\s>]`).MatchString(tag) {
			t.Errorf("#%s does not start hidden: %s", id, tag)
		}
	}
	list := findTagWindow(t, html, `id="low-list"`)
	for _, want := range []string{`aria-busy="true"`, `aria-labelledby="low-list-heading"`} {
		if !strings.Contains(list, want) {
			t.Errorf("#low-list is missing %s", want)
		}
	}
	basis := findTagWindow(t, html, `id="low-basis"`)
	if !strings.Contains(basis, `aria-live="polite"`) || strings.Contains(basis, "role=") {
		t.Errorf("#low-basis must be a polite region without a role: %s", basis)
	}
	stale := findTagWindow(t, html, `id="low-stale"`)
	if strings.Contains(stale, "role=") || strings.Contains(stale, "aria-live") {
		t.Errorf("#low-stale is passive text and must carry no role or live region: %s", stale)
	}

	if got := strings.Count(html, `id="notice-live"`); got != 1 {
		t.Errorf("notice-live appears %d times, want 1", got)
	}
	notice := findTagWindow(t, html, `id="notice-live"`)
	if !strings.Contains(notice, `role="status"`) || !strings.Contains(notice, `aria-live="polite"`) {
		t.Errorf("#notice-live must be a polite status region: %s", notice)
	}
	if strings.Index(html, `id="notice-live"`) > strings.Index(html, `id="app-shell"`) {
		t.Errorf("#notice-live must come before the shell")
	}
}

func TestLowDataCopy(t *testing.T) {
	js := p418Static(t, "js/low.js")
	for _, want := range []string{
		"Near your position.",
		"Near your last known position. Finding your position.",
		"Near your last known position. Location is off, so it may be out of date.",
		"Finding your position.",
		"Location is off, so this list shows reports near the default area. Turn on location access to see reports near you.",
		"No reports near you yet",
		"Nothing has been reported in your area recently. Use Report an emergency above to add one.",
		"No reports in this area yet",
		"Turn on location access to check near you, or use Report an emergency above to add one.",
		"'Last updated '",
		"'. Trying again.'",
		"'Connection lost. Showing reports from '",
		"'Back online. The list is up to date.'",
		"Pinalert.updatedAgo(",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("low.js is missing %q", want)
		}
	}

	html := p418Template(t)
	for _, want := range []string{
		"Low data view", "Text only, no map", "This device opens the Low data view by default.",
		// Amended 2026-10-21 by plan 44 (owner): Retry now and Retry are gone; Back and Reload are new; Open full version and Retry position remain in the report dialog.
		"Open full version", "Retry position", "Back to the full version", "Reload", "Active reports",
		"Couldn't load reports", "Check your connection, then use Reload at the top.",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the template is missing %q", want)
		}
	}

	files := map[string]string{"low.js": js, "low.css": p418Static(t, "css/low.css"), "low.html.tmpl": html}
	for name, text := range files {
		for _, r := range []rune{0x2014, 0x2013, 0x2026, 0x2018, 0x2019, 0x201c, 0x201d} {
			if strings.ContainsRune(text, r) {
				t.Errorf("%s holds the character U+%04X", name, r)
			}
		}
	}
}

// p418TitlePaddingRight is the padding the title row reserves on its right.
// Provenance: plan 04-18 Step 0 searched web/templates, web/static/css and
// web/static/js for the Phase 3.2 bell and found none (the only hits were
// aria-labelledby and a comment in activity.css), so the title row only needs
// to clear the account trigger: the 44px touch target plus the 16px gap. No
// built bell position exists to widen it by. If a bell ships later, this
// constant and the stylesheet value change together.
const p418TitlePaddingRight = "calc(var(--touch-target-min) + var(--space-md))"

func TestLowDataStyles(t *testing.T) {
	css := p418Static(t, "css/low.css")
	stripped := stripCSSComments(css)
	for _, banned := range []string{"rgb(", "rgba(", "hsl(", "!important", "999px"} {
		if strings.Contains(stripped, banned) {
			t.Errorf("low.css holds %q", banned)
		}
	}

	hex := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)
	fontSizes := map[string]bool{"var(--font-size-label)": true, "var(--font-size-body)": true, "var(--font-size-heading)": true, "var(--font-size-display)": true}
	fontWeights := map[string]bool{"var(--font-weight-regular)": true, "var(--font-weight-semibold)": true}
	rules := parseCSSRules(css)
	if len(rules) < 10 {
		t.Fatalf("parsed %d rules from low.css, want at least 10", len(rules))
	}
	for _, r := range rules {
		decls := declsOf(r.declBody)
		for name, value := range decls {
			if hex.MatchString(value) {
				t.Errorf("rule %q: %s holds a hex colour %q", r.selectorHead, name, value)
			}
			if name == "font-size" && !fontSizes[value] {
				t.Errorf("rule %q: font-size %q is not a size token", r.selectorHead, value)
			}
			if name == "font-weight" && !fontWeights[value] {
				t.Errorf("rule %q: font-weight %q is not a weight token", r.selectorHead, value)
			}
		}
	}

	expect := func(head string, want map[string]string) {
		t.Helper()
		r, ok := ruleBySelector(rules, head)
		if !ok {
			t.Errorf("low.css has no rule headed %q", head)
			return
		}
		got := declsOf(r.declBody)
		for name, value := range want {
			if got[name] != value {
				t.Errorf("rule %q: %s = %q, want %q", head, name, got[name], value)
			}
		}
	}
	expect(".low-page", map[string]string{
		"max-width":      "640px",
		"margin":         "0 auto",
		"padding-bottom": "calc(var(--space-lg) + var(--touch-target-min) + var(--space-md))",
	})
	expect(".low-page__head h1", map[string]string{"padding-right": p418TitlePaddingRight})
	expect(".low-page__line", map[string]string{"color": "var(--color-text-muted)"})
	expect(".low-basis", map[string]string{"color": "var(--color-text)"})
	expect(".report-row.report-row--low", map[string]string{"cursor": "default"})
	expect(".low-stale:not([hidden])", map[string]string{"display": "flex", "flex-wrap": "wrap"})
}
