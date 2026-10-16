package web

// Plan 04-10 contract tests (D-15, P4-A23, P4-A28): the report form lives in
// one shared partial (report_form.html.tmpl), one shared script
// (report-form.js) behind a small location interface, and the three map rules
// live in modal-map.css so a page without a map can link modal.css and carry
// no map rule.
//
// Honest limit: every test here is static inspection of the shipped template,
// script and stylesheet text. The rendered main page is covered by
// TestPageShellServesDOMContract and TestExistingPagesRenderWithChrome in the
// handlers package, and real form behaviour (pin placement, search, posting,
// discard confirmation) by the end of phase human check, not by these tests.

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// p410ReadStatic returns the raw text of a file under the embedded static
// tree, for example "static/js/report-form.js".
func p410ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("%s: could not read embedded file: %v", path, err)
	}
	return string(raw)
}

// p410ReadTemplate returns the raw text of templates/<name> from the embedded
// template tree.
func p410ReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := fs.ReadFile(TemplatesFS, "templates/"+name)
	if err != nil {
		t.Fatalf("templates/%s: could not read embedded file: %v", name, err)
	}
	return string(raw)
}

// p410CodeText strips block comments and then drops every line whose trimmed
// text starts with two slashes, so negative checks never trip on prose.
func p410CodeText(raw string) string {
	text := stripCSSComments(raw)
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

var p410IDPattern = regexp.MustCompile(`id="([^"]+)"`)

// p410IDs returns every id attribute value in document order.
func p410IDs(markup string) []string {
	var ids []string
	for _, m := range p410IDPattern.FindAllStringSubmatch(markup, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// p410Subsequence reports whether want appears in have in order, with other
// values allowed between.
func p410Subsequence(have, want []string) bool {
	i := 0
	for _, h := range have {
		if i < len(want) && h == want[i] {
			i++
		}
	}
	return i == len(want)
}

// p410Window returns the text from the first occurrence of anchor to the next
// occurrence of terminator after it, or to the end of text when no terminator
// follows. ok is false when the anchor is missing.
func p410Window(text, anchor, terminator string) (string, bool) {
	idx := strings.Index(text, anchor)
	if idx == -1 {
		return "", false
	}
	rest := text[idx:]
	end := strings.Index(rest, terminator)
	if end == -1 {
		return rest, true
	}
	return rest[:end], true
}

// p410FormIDs is the main page's report form id list in its pinned order.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-23, D-24,
// D-24a): the main page is the Post a report window. The first ten ids and the
// last three stay in the page (the header with its X, the location block, the
// pinned footer); the nine in the middle live in the shared partial's window
// branch. Cancel report is gone from the window (the X closes it); it stays in
// the partial's Low data branch. The Phase 3.1 and Phase 4 hands-on tests are
// walked on this layout after Phase 3.4 is built.
var p410FormIDs = []string{
	"report-modal", "modal-title", "close-report", "location-search",
	"location-search-results", "location-search-status", "modal-map", "near-place",
	"coord-readout", "location-notice",
	"category-grid", "severity-group", "shelter-fields", "shelter-capacity-status",
	"shelter-headcount", "description", "discard-confirm", "discard-confirm-discard",
	"discard-confirm-keep",
	"form-error", "send-after-wait", "submit-report",
}

// p410PageIDCount is how many ids at the start of p410FormIDs stay in the page,
// and p410FooterIDCount how many at its end (the pinned footer).
const (
	p410PageIDCount   = 10
	p410FooterIDCount = 3
)

const p410IncludeAction = `{{template "report_form.html.tmpl" .}}`

// p410LowOpen is the one template action the partial holds: it picks the form
// the Low data view was built with or the Post a report window form.
const p410LowOpen = "{{if .Chrome.LowDataPage}}"

// p410Branches splits the partial into its Low data branch and its window
// branch. The partial is a comment, the if, the Low markup, the else, the
// window markup and the closing end.
func p410Branches(t *testing.T, partial string) (low, window string) {
	t.Helper()
	open := strings.Index(partial, p410LowOpen)
	elseAt := strings.Index(partial, "{{else}}")
	endAt := strings.LastIndex(partial, "{{end}}")
	if open == -1 || elseAt == -1 || endAt == -1 || !(open < elseAt && elseAt < endAt) {
		t.Fatalf("report_form.html.tmpl must hold %s, {{else}} and {{end}} in that order (%d, %d, %d)",
			p410LowOpen, open, elseAt, endAt)
	}
	return partial[open+len(p410LowOpen) : elseAt], partial[elseAt+len("{{else}}") : endAt]
}

func TestReportFormPartialKeepsMainFormOrder(t *testing.T) {
	index := p410ReadTemplate(t, "index.html.tmpl")
	partialFile := p410ReadTemplate(t, "report_form.html.tmpl")
	lowBranch, partial := p410Branches(t, partialFile)

	if n := strings.Count(index, p410IncludeAction); n != 1 {
		t.Fatalf("index.html.tmpl must include the partial exactly once, found %d", n)
	}
	if strings.Contains(strings.ToLower(partialFile), "<html") {
		t.Errorf("the partial contains an html element opening tag, which would class it as a full page")
	}
	// Amended 2026-10-17 by Phase 3.4 plan 12 (03.4-CONTEXT D-24a): the Call 112
	// box of the window branch reads the configured emergency number, so the
	// partial holds three more actions, and only these: the tel target
	// (Chrome.Numbers.TelHref, a typed URL) and the number twice
	// (Chrome.Numbers.Primary). Any other action that reads a view model field
	// is still refused, and the Low data branch stays static markup.
	allowed := regexp.MustCompile(`\{\{\s*\.Chrome\.Numbers\.(TelHref|Primary)\s*\}\}`)
	rest := allowed.ReplaceAllString(partialFile, "")
	if n := len(allowed.FindAllString(partialFile, -1)); n != 3 {
		t.Errorf("the partial holds %d number actions, want 3: the tel target and the number twice", n)
	}
	if regexp.MustCompile(`\{\{\s*\.`).MatchString(rest) {
		t.Errorf("the partial contains a template action that reads a view model field beyond the emergency number; it must be static markup")
	}
	if regexp.MustCompile(`\{\{\s*\.`).MatchString(lowBranch) {
		t.Errorf("the Low data branch contains a template action that reads a view model field")
	}
	if n := strings.Count(rest, "{{"); n != 4 {
		t.Errorf("the partial holds %d other template actions, want 4: the comment, the if, the else and the end", n)
	}

	inlined := strings.Replace(index, p410IncludeAction, partial, 1)
	ids := p410IDs(inlined)
	if !p410Subsequence(ids, p410FormIDs) {
		t.Errorf("with the partial inlined the %d form ids are not present in their pinned order: %v", len(p410FormIDs), ids)
	}
	for _, id := range p410FormIDs {
		n := 0
		for _, got := range ids {
			if got == id {
				n++
			}
		}
		if n != 1 {
			t.Errorf("id %q appears %d times in the inlined main page, want exactly 1", id, n)
		}
	}

	pageIDs := p410IDs(index)
	partialIDs := p410IDs(partial)
	has := func(list []string, id string) bool {
		for _, got := range list {
			if got == id {
				return true
			}
		}
		return false
	}
	sharedIDs := p410FormIDs[p410PageIDCount : len(p410FormIDs)-p410FooterIDCount]
	pageOwnIDs := append(append([]string{}, p410FormIDs[:p410PageIDCount]...),
		p410FormIDs[len(p410FormIDs)-p410FooterIDCount:]...)
	for _, id := range sharedIDs {
		if !has(partialIDs, id) {
			t.Errorf("shared id %q is missing from the partial's window branch", id)
		}
		if has(pageIDs, id) {
			t.Errorf("shared id %q is still in index.html.tmpl itself", id)
		}
	}
	for _, id := range pageOwnIDs {
		if !has(pageIDs, id) {
			t.Errorf("page id %q is missing from index.html.tmpl", id)
		}
		if has(partialIDs, id) {
			t.Errorf("page id %q must not be in the partial's window branch", id)
		}
	}

	// The window closes with the X only (D-02): no Cancel report button in the
	// window branch or the page. The Low data branch keeps the one it was built
	// with.
	const cancel = `id="cancel-report" class="btn">Cancel report</button>`
	if n := strings.Count(lowBranch, cancel); n != 1 {
		t.Errorf("the Low data branch must hold %q exactly once, found %d", cancel, n)
	}
	for name, text := range map[string]string{"index.html.tmpl": index, "the window branch": partial} {
		if strings.Contains(text, "cancel-report") || strings.Contains(text, "Cancel report") {
			t.Errorf("%s still holds a Cancel report button (D-02)", name)
		}
	}
	for name, text := range map[string]string{"index.html.tmpl": index, "report_form.html.tmpl": partialFile} {
		if strings.Contains(text, ">Cancel<") {
			t.Errorf("%s still holds a button whose whole text is the single word Cancel (P4-A23)", name)
		}
	}

	for _, want := range []string{
		"Discard this report? Your description and location won't be saved.",
		"Discard",
		"Keep editing",
	} {
		if !strings.Contains(partial, want) || !strings.Contains(lowBranch, want) {
			t.Errorf("a branch of the partial lost shipped copy %q", want)
		}
	}
	if !strings.Contains(index, ">Post report</button>") {
		t.Errorf("the window footer lost the Post report button")
	}
	if !strings.Contains(lowBranch, "Post report") || !strings.Contains(lowBranch, "1 · Low") {
		t.Errorf("the Low data branch lost shipped copy")
	}
}

func TestModalMapRulesMoved(t *testing.T) {
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-23, D-24,
	// D-24a): modal-map.css also holds rules for the Post a report window, and
	// every one of them starts with .post-window. The three shipped rules below
	// are pinned as before; the window rules are left out of the count here and
	// pinned by post_window_contract_test.go. The Phase 3.1 and Phase 4
	// hands-on tests are walked on this layout after Phase 3.4 is built.
	var mapRules []cssRule
	for _, r := range parseCSSRules(p410ReadStatic(t, "static/css/modal-map.css")) {
		if !strings.Contains(r.selectorHead, ".post-window") {
			mapRules = append(mapRules, r)
		}
	}
	want := map[string]map[string]string{
		"#modal-map": {
			"height":        "220px",
			"border-radius": "8px",
			"overflow":      "hidden",
		},
		"#coord-readout": {
			"margin":      "0",
			"font-size":   "var(--font-size-label)",
			"line-height": "var(--line-height-label)",
			"color":       "var(--color-text-muted)",
		},
		"#location-notice": {
			"margin":        "0",
			"padding":       "var(--space-sm)",
			"border-radius": "6px",
			"background":    "var(--color-surface)",
			"font-size":     "var(--font-size-label)",
			"line-height":   "var(--line-height-label)",
			"color":         "var(--color-text-muted)",
		},
	}
	if len(mapRules) != len(want) {
		t.Errorf("modal-map.css has %d rules, want exactly %d", len(mapRules), len(want))
	}
	for head, decls := range want {
		rule, ok := ruleBySelector(mapRules, head)
		if !ok {
			t.Errorf("modal-map.css has no %q rule", head)
			continue
		}
		got := declsOf(rule.declBody)
		if len(got) != len(decls) {
			t.Errorf("%s: %d declarations, want %d: %v", head, len(got), len(decls), got)
		}
		for name, value := range decls {
			if got[name] != value {
				t.Errorf("%s: %s is %q, want %q", head, name, got[name], value)
			}
		}
	}

	modalRules := parseCSSRules(p410ReadStatic(t, "static/css/modal.css"))
	for _, r := range modalRules {
		for _, id := range []string{"#modal-map", "#coord-readout", "#location-notice"} {
			// The window's own rules (Phase 3.4) are scoped under .post-window.
			if strings.Contains(r.selectorHead, id) && !strings.Contains(r.selectorHead, ".post-window") {
				t.Errorf("modal.css still has a rule naming %s: %q", id, r.selectorHead)
			}
		}
	}
	for _, head := range []string{
		".modal-panel > * + *",
		"#location-search-results:not([hidden])",
		"#discard-confirm:not([hidden])",
	} {
		if _, ok := ruleBySelector(modalRules, head); !ok {
			t.Errorf("modal.css lost the shell rule %q", head)
		}
	}

	index := p410ReadTemplate(t, "index.html.tmpl")
	const link = `href="/static/css/modal-map.css?v={{.AssetVersion}}"`
	if n := strings.Count(index, link); n != 1 {
		t.Fatalf("index.html.tmpl must link modal-map.css exactly once, found %d", n)
	}
	mapIdx := strings.Index(index, link)
	modalIdx := strings.Index(index, `/static/css/modal.css?v=`)
	feedIdx := strings.Index(index, `/static/css/feed.css?v=`)
	if modalIdx == -1 || feedIdx == -1 || !(modalIdx < mapIdx && mapIdx < feedIdx) {
		t.Errorf("modal-map.css must be linked after modal.css and before feed.css (indexes %d, %d, %d)",
			modalIdx, mapIdx, feedIdx)
	}
}

func TestReportFormSubmitDisabledLook(t *testing.T) {
	rules := parseCSSRules(p410ReadStatic(t, "static/css/modal.css"))
	rule, ok := ruleBySelector(rules, "#submit-report:disabled")
	if !ok {
		t.Fatalf("modal.css has no #submit-report:disabled rule")
	}
	decls := declsOf(rule.declBody)
	if decls["cursor"] != "not-allowed" {
		t.Errorf("#submit-report:disabled cursor is %q, want not-allowed", decls["cursor"])
	}
	if _, has := decls["opacity"]; has {
		t.Errorf("#submit-report:disabled still declares opacity; the shared primary disabled look is the one look (P4-A28)")
	}
}

// p410WordParts splits text into words on every non letter character and then
// splits each word on lower to upper case boundaries.
func p410WordParts(text string) []string {
	var parts []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			parts = append(parts, string(cur))
			cur = cur[:0]
		}
	}
	var prev rune
	for _, r := range text {
		if !unicode.IsLetter(r) {
			flush()
			prev = 0
			continue
		}
		if prev != 0 && unicode.IsLower(prev) && unicode.IsUpper(r) {
			flush()
		}
		cur = append(cur, r)
		prev = r
	}
	flush()
	return parts
}

func TestReportFormNewFilesPlainText(t *testing.T) {
	emDash := string(rune(0x2014))
	enDash := string(rune(0x2013))
	files := map[string]string{
		"templates/report_form.html.tmpl": p410ReadTemplate(t, "report_form.html.tmpl"),
		"static/css/modal-map.css":        p410ReadStatic(t, "static/css/modal-map.css"),
		"static/js/report-form.js":        p410ReadStatic(t, "static/js/report-form.js"),
	}
	for name, text := range files {
		if strings.Contains(text, emDash) || strings.Contains(text, enDash) {
			t.Errorf("%s contains an em dash or en dash", name)
		}
		for _, part := range p410WordParts(text) {
			lower := strings.ToLower(part)
			if lower == "light" || lower == "lite" {
				t.Errorf("%s contains the banned word part %q", name, part)
			}
		}
	}
}

func TestReportFormDescriptionHint(t *testing.T) {
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-23, D-24,
	// D-24a): the hint line above the Description box is the Low data view's.
	// The Post a report window has Describe it with its placeholder instead
	// (UI-SPEC Surface 5d), so this test reads the Low data branch. The Phase 3.1
	// and Phase 4 hands-on tests are walked on this layout after Phase 3.4 is
	// built.
	partial, _ := p410Branches(t, p410ReadTemplate(t, "report_form.html.tmpl"))
	index := p410ReadTemplate(t, "index.html.tmpl")

	const hint = `<p id="description-hint">Describe what is happening, for example: water is knee deep on Main Road and three people need help.</p>`
	if n := strings.Count(partial, hint); n != 1 {
		t.Errorf("the partial must hold the hint line exactly once, found %d", n)
	}
	const field = `<textarea id="description" name="description" rows="3" aria-describedby="description-hint"></textarea>`
	if n := strings.Count(partial, field); n != 1 {
		t.Errorf("the partial must hold the described textarea tag exactly once, found %d", n)
	}
	if n := strings.Count(partial, "<textarea"); n != 1 {
		t.Errorf("the partial must hold exactly one textarea, found %d", n)
	}
	labelIdx := strings.Index(partial, `<label for="description">Description</label>`)
	hintIdx := strings.Index(partial, hint)
	fieldIdx := strings.Index(partial, field)
	if labelIdx == -1 || hintIdx == -1 || fieldIdx == -1 || !(labelIdx < hintIdx && hintIdx < fieldIdx) {
		t.Errorf("the hint must sit between the Description label and the textarea (indexes %d, %d, %d)",
			labelIdx, hintIdx, fieldIdx)
	}
	if n := strings.Count(index, `id="description-hint"`); n != 0 {
		t.Errorf("index.html.tmpl must hold no copy of the hint, found %d", n)
	}

	rules := parseCSSRules(p410ReadStatic(t, "static/css/modal.css"))
	count := 0
	for _, r := range rules {
		if r.selectorHead == "#description-hint" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("modal.css must hold the #description-hint rule exactly once, found %d", count)
	}
	rule, _ := ruleBySelector(rules, "#description-hint")
	got := declsOf(rule.declBody)
	want := map[string]string{
		"margin":      "var(--space-xs) 0 0",
		"font-size":   "var(--font-size-label)",
		"line-height": "var(--line-height-label)",
		"color":       "var(--color-text-muted)",
	}
	if len(got) != len(want) {
		t.Errorf("#description-hint has %d declarations, want %d: %v", len(got), len(want), got)
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("#description-hint %s is %q, want %q", name, got[name], value)
		}
	}
}

// p410NeedAll reports one error per needle missing from text.
func p410NeedAll(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(text, n) {
			t.Errorf("%s: expected %q, found none", label, n)
		}
	}
}

// p410NeedNone reports one error per needle present in text.
func p410NeedNone(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(text, n) {
			t.Errorf("%s: found %q, expected none", label, n)
		}
	}
}

// TestReportFormLocationInterface (V-21, D-15): report-form.js reads location
// only through the five interface methods plus the optional start and reset
// hooks, and modal.js is the Leaflet implementation that mounts the form.
func TestReportFormLocationInterface(t *testing.T) {
	form := p410CodeText(p410ReadStatic(t, "static/js/report-form.js"))
	pin := p410CodeText(p410ReadStatic(t, "static/js/modal.js"))

	if n := strings.Count(form, "window.PinalertReportForm = {"); n != 1 {
		t.Fatalf("report-form.js must export window.PinalertReportForm exactly once, found %d", n)
	}
	export, _ := p410Window(form, "window.PinalertReportForm = {", "};")
	p410NeedAll(t, "report-form.js export", export, "mount: mount", "open: openForm", "close: closeForm")

	win := func(text, anchor, terminator string) string {
		w, ok := p410Window(text, anchor, terminator)
		if !ok {
			t.Errorf("missing anchor %q", anchor)
		}
		return w
	}
	const nextFn = "\n  function "
	p410NeedAll(t, "function mount", win(form, "function mount(", nextFn),
		"options.backdrop", "options.form", "options.location", "options.variant")
	p410NeedAll(t, "report-form.js interface calls", form,
		"picker.current()", "picker.choose(", "picker.isSet()", "picker.missingMessage()",
		"picker.searchFailureMessage(",
		"typeof picker.start === 'function'", "typeof picker.reset === 'function'")
	p410NeedAll(t, "function validate", win(form, "function validate(", nextFn),
		"picker.isSet()", "picker.missingMessage()")
	p410NeedAll(t, "function buildPayload", win(form, "function buildPayload(", nextFn), "picker.current()")
	p410NeedAll(t, "function renderResultRow", win(form, "function renderResultRow(", nextFn),
		"picker.choose(", "displayName")
	p410NeedAll(t, "function openForm", win(form, "function openForm(", nextFn),
		"options.opener", "picker.start(")
	p410NeedAll(t, "function resetForm", win(form, "function resetForm(", nextFn), "picker.reset()")

	p410NeedAll(t, "leafletLocation object", win(pin, "var leafletLocation = {", "};"),
		"current: currentPosition", "choose: chooseSearchResult", "isSet: hasMarker",
		"missingMessage: missingLocationMessage", "searchFailureMessage: searchUnavailableMessage",
		"start: startLocation", "reset: clearMarker")
	if n := strings.Count(pin, "PinalertReportForm.mount("); n != 1 {
		t.Errorf("modal.js must call PinalertReportForm.mount( exactly once, found %d", n)
	}
	p410NeedAll(t, "mount call", win(pin, "PinalertReportForm.mount(", ");"),
		"variant: 'main'", "location: leafletLocation")
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10 and
	// D-02): the floating report button is gone. One delegated click handler on
	// the elements carrying data-open-report (the Report item of the phone
	// bottom bar and the Post a report link of the laptop top bar) opens the
	// form with the clicked element as the opener, and the address post=1 opens
	// it once on load. The Phase 3.1 and Phase 4 hands-on tests are walked on
	// this layout after Phase 3.4 is built.
	p410NeedAll(t, "opener click", win(pin, "document.addEventListener('click'", "});"),
		"openerFor(e.target)", "e.preventDefault()", "PinalertReportForm.open(", "opener: opener")
	p410NeedAll(t, "post=1 on load", win(pin, "function openFromAddress(", "\n  openFromAddress();"),
		"searchParams.has(POST_PARAM)", "searchParams.delete(POST_PARAM)", "history.replaceState(", "PinalertReportForm.open(")
	if strings.Contains(pin, "fab-report") || strings.Contains(pin, "fabButton") {
		t.Errorf("modal.js still names the retired floating report button")
	}
}

// TestReportFormOwnsTheFormLogic (D-15, U6): the form functions live in
// report-form.js, the Leaflet functions stay in modal.js, and neither side
// uses a name that now lives in the other file's separate script scope.
func TestReportFormOwnsTheFormLogic(t *testing.T) {
	form := p410CodeText(p410ReadStatic(t, "static/js/report-form.js"))
	pin := p410CodeText(p410ReadStatic(t, "static/js/modal.js"))

	moved := []string{
		"buildCategoryGrid", "validate", "buildPayload", "handleSubmit", "requestClose",
		"resetForm", "resetSearch", "runSearch", "renderResultRow", "trapTab",
		"mount", "openForm", "closeForm",
	}
	for _, name := range moved {
		def := "function " + name + "("
		if !strings.Contains(form, def) {
			t.Errorf("report-form.js must define %s", def)
		}
		if strings.Contains(pin, def) {
			t.Errorf("modal.js still defines %s", def)
		}
	}
	for _, def := range []string{"function openModal(", "function closeModal("} {
		p410NeedNone(t, "report-form.js", form, def)
		p410NeedNone(t, "modal.js", pin, def)
	}
	for _, name := range []string{
		"placeMarker", "initLocation", "showLocationDenied", "hasVectorBasemap",
		"updateCoordReadout", "chooseSearchResult", "clearMarker",
	} {
		p410NeedAll(t, "modal.js", pin, "function "+name+"(")
	}

	leafletGlobal := regexp.MustCompile(`(^|[^A-Za-z0-9_$.])L\.`)
	if leafletGlobal.MatchString(form) {
		t.Errorf("report-form.js references the Leaflet global; it must stay map free")
	}
	p410NeedNone(t, "report-form.js", form, "modalMap", "modalMarker", "placeMarker", "typeof L")
	if n := strings.Count(form, "fetch("); n != 1 {
		t.Errorf("report-form.js must contain fetch( exactly once (the geocode search), found %d", n)
	}
	p410NeedAll(t, "report-form.js", form, "'/api/geocode?q='", "Pinalert.submitReport(")
	p410NeedNone(t, "modal.js", pin, "fetch(", "Pinalert.submitReport(", "Pinalert.iconClass(")

	ellipsis := string(rune(0x2026))
	p410NeedAll(t, "report-form.js shipped copy", form,
		"'Choose a category to continue.'", "'Pick a severity level.'",
		"'Add a short description (at least 10 characters).'",
		"'Choose a shelter capacity status.'", "'Report posted.'", "'Post report'",
		"'No matches found.'", "'Posting"+ellipsis+"'")

	// Split guard: the two files are separate scripts, so a name left on the
	// wrong side of the split is an undefined name at run time.
	p410NeedNone(t, "report-form.js split guard", form,
		"SEARCH_UNAVAILABLE_MESSAGE", "SEARCH_RESULT_ZOOM", "coordReadout", "locationNotice",
		"modalMapEl", "fabButton", "initLocation", "updateCoordReadout", "openModal", "closeModal")
	bareModal := regexp.MustCompile(`(^|[^A-Za-z0-9_$.\-])modal($|[^A-Za-z0-9_\-])`)
	if bareModal.MatchString(form) {
		t.Errorf("report-form.js still uses a bare modal identifier")
	}
	p410NeedNone(t, "modal.js split guard", pin,
		"formTouched", "selectedCategory", "categoryGrid", "submitButton", "locationSearchInput",
		"searchSeq", "openModal(", "closeModal(")
	bareTouched := regexp.MustCompile(`(^|[^A-Za-z.])markTouched\(`)
	if bareTouched.MatchString(pin) {
		t.Errorf("modal.js still calls a bare markTouched(")
	}
	if n := strings.Count(pin, "var SEARCH_UNAVAILABLE_MESSAGE ="); n != 1 {
		t.Errorf("modal.js must define SEARCH_UNAVAILABLE_MESSAGE exactly once, found %d", n)
	}
}

// TestReportFormScriptOrder (D-15): the main page loads report-form.js once,
// deferred and versioned, after app.js and trust.js and before map.js and
// modal.js.
func TestReportFormScriptOrder(t *testing.T) {
	index := p410ReadTemplate(t, "index.html.tmpl")
	const tag = `src="/static/js/report-form.js?v={{.AssetVersion}}" defer`
	if n := strings.Count(index, tag); n != 1 {
		t.Fatalf("index.html.tmpl must load report-form.js exactly once, found %d", n)
	}
	at := strings.Index(index, tag)
	app := strings.Index(index, "/static/js/app.js")
	trust := strings.Index(index, "/static/js/trust.js")
	mapJS := strings.Index(index, "/static/js/map.js")
	modalJS := strings.Index(index, "/static/js/modal.js")
	if app == -1 || trust == -1 || mapJS == -1 || modalJS == -1 {
		t.Fatalf("a neighbouring script tag is missing: app %d trust %d map %d modal %d", app, trust, mapJS, modalJS)
	}
	if !(app < at && trust < at && at < mapJS && at < modalJS) {
		t.Errorf("report-form.js must load after app.js and trust.js and before map.js and modal.js "+
			"(indexes app %d trust %d form %d map %d modal %d)", app, trust, at, mapJS, modalJS)
	}
}
