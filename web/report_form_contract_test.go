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

// p410FormIDs is the main page's report form id list in its pinned order:
// the first eight stay in the page (shell and location block), the last
// thirteen live in the shared partial.
var p410FormIDs = []string{
	"report-modal", "modal-title", "location-search", "location-search-results",
	"location-search-status", "modal-map", "coord-readout", "location-notice",
	"category-grid", "severity", "severity-readout", "description",
	"shelter-fields", "shelter-capacity-status", "shelter-headcount", "form-error",
	"submit-report", "cancel-report", "discard-confirm", "discard-confirm-discard",
	"discard-confirm-keep",
}

const p410IncludeAction = `{{template "report_form.html.tmpl" .}}`

func TestReportFormPartialKeepsMainFormOrder(t *testing.T) {
	index := p410ReadTemplate(t, "index.html.tmpl")
	partial := p410ReadTemplate(t, "report_form.html.tmpl")

	if n := strings.Count(index, p410IncludeAction); n != 1 {
		t.Fatalf("index.html.tmpl must include the partial exactly once, found %d", n)
	}
	if strings.Contains(strings.ToLower(partial), "<html") {
		t.Errorf("the partial contains an html element opening tag, which would class it as a full page")
	}
	if regexp.MustCompile(`\{\{\s*\.`).MatchString(partial) {
		t.Errorf("the partial contains a template action that reads a view model field; it must be static markup")
	}

	inlined := strings.Replace(index, p410IncludeAction, partial, 1)
	ids := p410IDs(inlined)
	if !p410Subsequence(ids, p410FormIDs) {
		t.Errorf("with the partial inlined the 21 form ids are not present in their pinned order: %v", ids)
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
	for _, id := range p410FormIDs[8:] {
		if !has(partialIDs, id) {
			t.Errorf("shared id %q is missing from the partial", id)
		}
		if has(pageIDs, id) {
			t.Errorf("shared id %q is still in index.html.tmpl itself", id)
		}
	}
	for _, id := range p410FormIDs[:8] {
		if !has(pageIDs, id) {
			t.Errorf("page id %q is missing from index.html.tmpl", id)
		}
		if has(partialIDs, id) {
			t.Errorf("page id %q must not be in the partial", id)
		}
	}

	const cancel = `id="cancel-report" class="btn">Cancel report</button>`
	if n := strings.Count(partial, cancel); n != 1 {
		t.Errorf("the partial must hold %q exactly once, found %d", cancel, n)
	}
	for name, text := range map[string]string{"index.html.tmpl": index, "report_form.html.tmpl": partial} {
		if strings.Contains(text, ">Cancel<") {
			t.Errorf("%s still holds a button whose whole text is the single word Cancel (P4-A23)", name)
		}
	}

	for _, want := range []string{
		"Post report",
		"Discard this report? Your description and location won't be saved.",
		"Discard",
		"Keep editing",
		"1 · Low",
	} {
		if !strings.Contains(partial, want) {
			t.Errorf("the partial lost shipped copy %q", want)
		}
	}
}

func TestModalMapRulesMoved(t *testing.T) {
	mapRules := parseCSSRules(p410ReadStatic(t, "static/css/modal-map.css"))
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
			if strings.Contains(r.selectorHead, id) {
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
	partial := p410ReadTemplate(t, "report_form.html.tmpl")
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
