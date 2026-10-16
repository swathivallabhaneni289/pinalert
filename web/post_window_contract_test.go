package web

// Phase 3.4 plan 09 contract tests (LAYOUT-05, D-02, D-23, D-24, D-24a, D-29):
// Post a report is a full screen sheet on the phone and a wide two pane window
// on the laptop, closed with the X only, with eight category tiles from one
// constant, three severity boxes in their own soft colours and one button in
// the footer. The Low data view keeps the form it was built with.
//
// Honest limit: these are static inspections of the shipped template, script
// and stylesheet text. How the window looks at 320, 390, 900 and 1280 pixels
// is checked by the owner when Phase 3.4 is built.

import (
	"io/fs"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// postWindowBlock returns the #report-modal block of index.html.tmpl: from the
// backdrop that holds it to the flag dialog that follows.
func postWindowBlock(t *testing.T) string {
	t.Helper()
	index := p410ReadTemplate(t, "index.html.tmpl")
	start := strings.Index(index, `<div class="modal-backdrop post-window" id="report-modal" hidden>`)
	end := strings.Index(index, `{{template "flag_dialog.html.tmpl" .}}`)
	if start == -1 || end == -1 || end < start {
		t.Fatalf("index.html.tmpl has no Post a report window block (%d, %d)", start, end)
	}
	return index[start:end]
}

// TestPostWindowClosesWithXOnly: one X named Close in the window header, no
// Cancel report control, no backdrop click close, and Escape is handled.
func TestPostWindowClosesWithXOnly(t *testing.T) {
	block := postWindowBlock(t)

	if n := strings.Count(block, `id="close-report"`); n != 1 {
		t.Fatalf("the window holds %d close-report controls, want exactly 1", n)
	}
	if !strings.Contains(block, `<button type="button" class="post-window__close" id="close-report" aria-label="Close">`) {
		t.Errorf("the X must be a button named Close")
	}
	if !strings.Contains(block, `app-icon--x`) {
		t.Errorf("the X must draw x.svg through app-icon--x")
	}
	head := block[strings.Index(block, "<header"):strings.Index(block, "</header>")]
	if !strings.Contains(head, `id="close-report"`) || !strings.Contains(head, `<h2 id="modal-title" class="post-window__title">Post a report</h2>`) ||
		!strings.Contains(head, "Say what is happening and where.") {
		t.Errorf("the header must hold the title Post a report, the line under it and the X")
	}
	for _, banned := range []string{"cancel-report", "Cancel report", ">Cancel<"} {
		if strings.Contains(block, banned) {
			t.Errorf("the window still holds %q; it closes with the X only (D-02)", banned)
		}
	}
	if strings.Contains(block, "Report an emergency") {
		t.Errorf("the window still carries the old title")
	}

	form := p410CodeText(p410ReadStatic(t, "static/js/report-form.js"))
	for _, want := range []string{
		"closeButton = document.getElementById('close-report')",
		"closeButton.addEventListener('click', requestClose)",
		"e.key === 'Escape'",
		"if (!closeButton) {",
	} {
		if !strings.Contains(form, want) {
			t.Errorf("report-form.js lacks %q", want)
		}
	}
	// The backdrop click close is registered only on a page without the X.
	at := strings.Index(form, "if (!closeButton) {")
	if at == -1 || !strings.Contains(form[at:at+200], "backdropEl.addEventListener('click'") {
		t.Errorf("the backdrop click close must sit inside if (!closeButton)")
	}
}

// TestPostWindowNoNotConfirmedFooterLine: the mockup's line under Post report
// about the report showing as not confirmed is built on neither size (owner
// answer O-1, mockup override 2). No template and no script holds its tail.
func TestPostWindowNoNotConfirmedFooterLine(t *testing.T) {
	const fragment = "until people nearby confirm it"
	for _, root := range []struct {
		fsys fs.FS
		dir  string
	}{{TemplatesFS, "templates"}, {StaticFS, "static"}} {
		err := fs.WalkDir(root.fsys, root.dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !strings.HasSuffix(path, ".tmpl") && !strings.HasSuffix(path, ".js") && !strings.HasSuffix(path, ".css") {
				return nil
			}
			raw, readErr := fs.ReadFile(root.fsys, path)
			if readErr != nil {
				return readErr
			}
			if strings.Contains(string(raw), fragment) {
				t.Errorf("%s holds the mockup's footer line about %q; it is built on neither size", path, "Not confirmed yet")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root.dir, err)
		}
	}

	// The footer holds the error line, Send after the wait and the one button.
	block := postWindowBlock(t)
	foot := block[strings.Index(block, "<footer"):strings.Index(block, "</footer>")]
	if n := strings.Count(foot, "<button"); n != 2 {
		t.Errorf("the footer holds %d buttons, want 2 (Send after the wait, hidden, and Post report)", n)
	}
	if strings.Contains(foot, "<span") || strings.Count(foot, "<p") != 1 {
		t.Errorf("the footer must hold only the error line and the buttons, no hint text")
	}
}

// TestPostWindowBackdropBelowStrip (D-29, T-34-41): on the laptop the backdrop
// starts below the notice strip so the strip stays on screen and undimmed.
func TestPostWindowBackdropBelowStrip(t *testing.T) {
	css := p410ReadStatic(t, "static/css/modal.css")
	if !strings.Contains(css, "top: var(--disclaimer-bar-height);") {
		t.Fatalf("modal.css must start the laptop backdrop at top: var(--disclaimer-bar-height)")
	}
	var laptop *cssRule
	for _, r := range parseCSSRules(css) {
		if strings.Contains(r.selectorHead, "@media (min-width: 900px)") &&
			strings.Contains(r.selectorHead, ".modal-backdrop.post-window:not([hidden])") {
			rule := r
			laptop = &rule
		}
	}
	if laptop == nil {
		t.Fatalf("modal.css has no 900px rule for .modal-backdrop.post-window:not([hidden])")
	}
	decls := declsOf(laptop.declBody)
	if decls["top"] != "var(--disclaimer-bar-height)" {
		t.Errorf("the laptop backdrop top is %q, want var(--disclaimer-bar-height)", decls["top"])
	}
	if decls["background"] != "rgb(0 0 0 / 45%)" {
		t.Errorf("the laptop backdrop background is %q, want rgb(0 0 0 / 45%%)", decls["background"])
	}
	// The window itself: min(1180px, 94%) by min(780px, 92%), radius 12px.
	for _, want := range []string{"width: min(1180px, 94%);", "height: min(780px, 92%);", "border-radius: 12px;", "box-shadow: var(--shadow-window);"} {
		if !strings.Contains(css, want) {
			t.Errorf("modal.css lacks the laptop window declaration %q", want)
		}
	}
}

// windowTilesFromScript reads the ordered category and word pairs of the
// WINDOW_TILES constant in report-form.js.
func windowTilesFromScript(t *testing.T) (categories, words []string) {
	t.Helper()
	form := p410ReadStatic(t, "static/js/report-form.js")
	block, ok := p410Window(form, "var WINDOW_TILES = [", "];")
	if !ok {
		t.Fatalf("report-form.js has no WINDOW_TILES constant")
	}
	entry := regexp.MustCompile(`\{ category: '([a-z_]+)', word: '([^']+)' \}`)
	for _, m := range entry.FindAllStringSubmatch(block, -1) {
		categories = append(categories, m[1])
		words = append(words, m[2])
	}
	return categories, words
}

// windowBranch returns the window branch of the shared partial.
func windowBranch(t *testing.T) string {
	t.Helper()
	_, window := p410Branches(t, p410ReadTemplate(t, "report_form.html.tmpl"))
	return window
}

// TestPostWindowEightTiles (D-24, D-24a): What is it? shows eight tiles from one
// constant, in order, with no Rescue needed and no Earthquake tile.
func TestPostWindowEightTiles(t *testing.T) {
	categories, words := windowTilesFromScript(t)
	wantCategories := []string{"flood", "traffic", "shelter_open", "power_outage", "road_blocked", "fire", "storm_cyclone", "other"}
	wantWords := []string{"Flood", "Traffic", "Shelter open", "Power out", "Road blocked", "Fire", "Storm", "Other"}
	if !reflect.DeepEqual(categories, wantCategories) {
		t.Errorf("WINDOW_TILES categories = %v, want %v", categories, wantCategories)
	}
	if !reflect.DeepEqual(words, wantWords) {
		t.Errorf("WINDOW_TILES words = %v, want %v", words, wantWords)
	}
	for _, retired := range []string{"rescue_needed", "earthquake"} {
		for _, c := range categories {
			if c == retired {
				t.Errorf("WINDOW_TILES holds %s, which is not a tile", retired)
			}
		}
	}

	form := p410CodeText(p410ReadStatic(t, "static/js/report-form.js"))
	// One constant: the window grid is built from it and nothing else, four
	// columns wide, and Up and Down move by a row of four.
	for _, want := range []string{
		"WINDOW_TILES.forEach(function (entry, index) {",
		"var WINDOW_GRID_COLUMNS = 4;",
		"var LOW_GRID_COLUMNS = 3;",
		"windowMode = !!severityGroup;",
	} {
		if !strings.Contains(form, want) {
			t.Errorf("report-form.js lacks %q", want)
		}
	}
	if n := strings.Count(form, "var WINDOW_TILES = ["); n != 1 {
		t.Errorf("report-form.js declares WINDOW_TILES %d times, want 1", n)
	}
	// Pinalert.CATEGORIES stays the server's postable list (it still holds
	// earthquake), so an old queued item sends unchanged.
	app := p410ReadStatic(t, "static/js/app.js")
	if !strings.Contains(app, "'earthquake'") || !strings.Contains(app, "var CATEGORIES = [") {
		t.Errorf("app.js must keep earthquake in Pinalert.CATEGORIES")
	}

	window := windowBranch(t)
	if n := strings.Count(window, ">What is it?</legend>"); n != 1 {
		t.Errorf("the window branch holds %d What is it? legends, want 1", n)
	}
	if !strings.Contains(window, `<div id="category-grid" role="radiogroup" aria-labelledby="category-legend"></div>`) {
		t.Errorf("the tile grid must be a radiogroup named by its legend")
	}
}

// TestPostWindowChosenTileSoftFill (D-04, owner answer O-2, mockup override 3):
// the chosen tile in the window has the soft grey fill and the border it had,
// never a dark fill, a dark border or a 2px border.
func TestPostWindowChosenTileSoftFill(t *testing.T) {
	css := p410ReadStatic(t, "static/css/modal.css")
	var chosen, glyph *cssRule
	for _, r := range parseCSSRules(css) {
		switch r.selectorHead {
		case ".post-window #category-grid .category-tile--selected":
			rule := r
			chosen = &rule
		case ".post-window #category-grid .category-tile--selected .icon-glyph":
			rule := r
			glyph = &rule
		}
	}
	if chosen == nil || glyph == nil {
		t.Fatalf("modal.css has no window scoped rule for the chosen tile and its glyph")
	}
	decls := declsOf(chosen.declBody)
	if decls["background"] != "var(--color-surface)" {
		t.Errorf("the chosen tile background is %q, want var(--color-surface)", decls["background"])
	}
	if decls["font-weight"] != "var(--font-weight-semibold)" {
		t.Errorf("the chosen tile label weight is %q, want var(--font-weight-semibold)", decls["font-weight"])
	}
	if decls["border-color"] != "var(--color-border)" {
		t.Errorf("the chosen tile border colour is %q, want var(--color-border), never the text colour", decls["border-color"])
	}
	for name, value := range decls {
		if strings.Contains(name, "border") && (strings.Contains(value, "2px") || value == "var(--color-text)") {
			t.Errorf("the chosen tile declares %s: %s; no dark and no 2px border (mockup override 3)", name, value)
		}
		if strings.Contains(name, "outline") || name == "box-shadow" {
			t.Errorf("the chosen tile declares %s; it is a fill only", name)
		}
	}
	if declsOf(glyph.declBody)["color"] != "var(--color-text)" {
		t.Errorf("the chosen glyph colour is %q, want var(--color-text)", declsOf(glyph.declBody)["color"])
	}

	// The base tile keeps its 1px hairline.
	base, ok := ruleBySelector(parseCSSRules(css), ".post-window #category-grid .category-tile")
	if !ok {
		t.Fatalf("modal.css has no window scoped tile rule")
	}
	if declsOf(base.declBody)["border"] != "1px solid var(--color-border)" {
		t.Errorf("the window tile border is %q, want 1px solid var(--color-border)", declsOf(base.declBody)["border"])
	}

	// The Low data view keeps its dark chosen tile: it does not carry
	// .post-window, so main.css still draws it.
	mainRule, ok := ruleBySelector(parseCSSRules(p410ReadStatic(t, "static/css/main.css")), ".category-tile--selected")
	if !ok {
		t.Fatalf("main.css lost .category-tile--selected")
	}
	if declsOf(mainRule.declBody)["background"] != "var(--color-text)" {
		t.Errorf("main.css .category-tile--selected background is %q, the Low data view keeps var(--color-text)", declsOf(mainRule.declBody)["background"])
	}
	if strings.Contains(p410ReadTemplate(t, "low.html.tmpl"), "post-window") {
		t.Errorf("low.html.tmpl carries post-window; the Low data view must keep its dark chosen tile")
	}
}

// TestPostWindowSeverityBoxes (D-04, D-05, D-24): three boxes, each with a 4px
// edge in its own colour and the label only; the chosen box is filled with its
// own soft colour; no radio circle, no outline, no ring and no hint text.
func TestPostWindowSeverityBoxes(t *testing.T) {
	window := windowBranch(t)
	if n := strings.Count(window, ">How serious?</legend>"); n != 1 {
		t.Fatalf("the window branch holds %d How serious? legends, want 1", n)
	}
	group := window[strings.Index(window, `<div id="severity-group"`):]
	group = group[:strings.Index(group, "</div>")]
	if !strings.Contains(group, `role="radiogroup" aria-labelledby="severity-legend"`) {
		t.Errorf("the severity group must be a radiogroup named by its legend")
	}
	boxes := regexp.MustCompile(`<button type="button" class="severity-box severity-box--(low|medium|critical)" role="radio" aria-checked="(true|false)" tabindex="(0|-1)" data-severity="(low|medium|critical)"><span class="severity-box__label">(Low|Medium|Critical)</span></button>`).FindAllStringSubmatch(group, -1)
	if len(boxes) != 3 {
		t.Fatalf("the group holds %d severity boxes, want 3", len(boxes))
	}
	for i, want := range []struct{ key, label, checked, tab string }{
		{"low", "Low", "true", "0"}, {"medium", "Medium", "false", "-1"}, {"critical", "Critical", "false", "-1"},
	} {
		b := boxes[i]
		if b[1] != want.key || b[4] != want.key || b[5] != want.label || b[2] != want.checked || b[3] != want.tab {
			t.Errorf("box %d = %v, want %+v (the form starts on Low)", i, b[1:], want)
		}
	}
	// The label only: no hint line, no input, no output and no range slider.
	for _, banned := range []string{"<p", "<input", "<output", `type="range"`, "hint"} {
		if strings.Contains(group, banned) {
			t.Errorf("the severity group holds %q; the boxes carry the label only (D-05)", banned)
		}
	}
	if strings.Contains(window, `type="range"`) || strings.Contains(window, "severity-slider") {
		t.Errorf("the window branch still holds the range slider")
	}

	rules := parseCSSRules(p410ReadStatic(t, "static/css/modal.css"))
	for key, colour := range map[string]string{"low": "--color-severity-low", "medium": "--color-severity-medium", "critical": "--color-severity-critical"} {
		edge, ok := ruleBySelector(rules, ".post-window .severity-box--"+key)
		if !ok {
			t.Errorf("modal.css has no edge rule for the %s box", key)
			continue
		}
		if got := declsOf(edge.declBody)["border-left"]; got != "4px solid var("+colour+")" {
			t.Errorf("the %s box border-left is %q, want 4px solid var(%s)", key, got, colour)
		}
		fill, ok := ruleBySelector(rules, ".post-window .severity-box--"+key+`[aria-checked="true"]`)
		if !ok {
			t.Errorf("modal.css has no chosen fill rule for the %s box", key)
			continue
		}
		fillDecls := declsOf(fill.declBody)
		if fillDecls["background"] != "var("+colour+"-bg)" {
			t.Errorf("the chosen %s box background is %q, want var(%s-bg)", key, fillDecls["background"], colour)
		}
		for name := range fillDecls {
			if strings.Contains(name, "outline") || name == "box-shadow" || strings.Contains(name, "border") {
				t.Errorf("the chosen %s box declares %s; the fill is the only change", key, name)
			}
		}
	}
	box, ok := ruleBySelector(rules, ".post-window .severity-box")
	if !ok {
		t.Fatalf("modal.css has no .post-window .severity-box rule")
	}
	bd := declsOf(box.declBody)
	if bd["min-height"] != "calc(var(--touch-target-min) + var(--space-md))" || bd["border-radius"] != "8px" || bd["border"] != "1px solid var(--color-border)" {
		t.Errorf("the severity box geometry is %v", bd)
	}
	if bd["color"] != "var(--color-text)" {
		t.Errorf("the severity box text colour is %q, want var(--color-text) on the tint", bd["color"])
	}

	// The script reads the payload value from the chosen box.
	form := p410CodeText(p410ReadStatic(t, "static/js/report-form.js"))
	for _, want := range []string{"'aria-checked'", "windowSeverity = key;", "severity: currentSeverity(),", "e.key === 'ArrowRight'", "e.key === 'ArrowLeft'"} {
		if !strings.Contains(form, want) {
			t.Errorf("report-form.js lacks %q", want)
		}
	}
}

// TestPostWindowDescribeIt (UI-SPEC Surface 5d, P34-C18): Describe it has a
// 280 letter limit, its placeholder and the one line under it; the old hint
// above the box is not in the window.
func TestPostWindowDescribeIt(t *testing.T) {
	window := windowBranch(t)
	for _, want := range []string{
		`<label for="description" class="post-group__legend">Describe it</label>`,
		`maxlength="280"`,
		`placeholder="For example: water above knee level near the signal, cars stuck."`,
		`<p id="description-help">Short and exact helps people confirm it. Up to 280 letters.</p>`,
	} {
		if n := strings.Count(window, want); n != 1 {
			t.Errorf("the window branch holds %q %d times, want 1", want, n)
		}
	}
	if strings.Contains(window, "description-hint") {
		t.Errorf("the window branch still holds the old hint above the Description box")
	}
	partial := p410ReadTemplate(t, "report_form.html.tmpl")
	if n := strings.Count(partial, "Short and exact helps people confirm it. Up to 280 letters."); n != 1 {
		t.Errorf("the partial holds the line under Describe it %d times, want 1", n)
	}
	// Shelter capacity sits directly under How serious? and above Describe it.
	if !(strings.Index(window, `id="severity-group"`) < strings.Index(window, `id="shelter-fields"`) &&
		strings.Index(window, `id="shelter-fields"`) < strings.Index(window, `id="description"`)) {
		t.Errorf("the window order must be tiles, severity, shelter capacity, Describe it")
	}
}

// TestLowFormKeepsBuiltMode (D-24a, T-34-40): the Low data view keeps the form
// it was built with, a second mode of the same partial and script: the slider,
// its readout, the nine tile list, the hint line and Cancel report.
func TestLowFormKeepsBuiltMode(t *testing.T) {
	lowBranch, _ := p410Branches(t, p410ReadTemplate(t, "report_form.html.tmpl"))
	for _, want := range []string{
		`<div id="category-grid" role="radiogroup" aria-label="Category"></div>`,
		`<input type="range" id="severity" min="1" max="3" step="1" value="1"`,
		`class="severity-slider"`,
		`<output id="severity-readout" for="severity">1 · Low</output>`,
		`<p id="description-hint">`,
		`id="cancel-report" class="btn">Cancel report</button>`,
		`id="submit-report"`,
		`id="send-after-wait"`,
		`id="discard-confirm"`,
	} {
		if !strings.Contains(lowBranch, want) {
			t.Errorf("the Low data branch lost %q", want)
		}
	}
	if strings.Contains(lowBranch, "severity-box") || strings.Contains(lowBranch, "post-group") {
		t.Errorf("the Low data branch holds window markup")
	}
	if n := strings.Count(p410ReadTemplate(t, "report_form.html.tmpl"), `type="range"`); n != 1 {
		t.Errorf("the partial holds %d range sliders, want 1 (the Low branch only)", n)
	}
	low := p410ReadTemplate(t, "low.html.tmpl")
	if !strings.Contains(low, `{{template "report_form.html.tmpl" .}}`) {
		t.Errorf("low.html.tmpl no longer includes the shared form")
	}
	if strings.Contains(low, "close-report") || strings.Contains(low, "severity-group") {
		t.Errorf("low.html.tmpl holds window markup")
	}
	form := p410CodeText(p410ReadStatic(t, "static/js/report-form.js"))
	for _, want := range []string{
		"Pinalert.CATEGORIES.forEach(function (category, index) {",
		"severityInput.addEventListener('input', updateSeverityReadout);",
		"buildSeverityControl();",
	} {
		if !strings.Contains(form, want) {
			t.Errorf("report-form.js lost the Low data mode piece %q", want)
		}
	}
}

// TestPostWindowPlainText: the strings this plan wrote carry no em dash and no
// en dash.
func TestPostWindowPlainText(t *testing.T) {
	form := p410ReadStatic(t, "static/js/report-form.js")
	tiles, _ := p410Window(form, "var WINDOW_TILES = [", "];")
	for name, text := range map[string]string{
		"the window block of index.html.tmpl": postWindowBlock(t),
		"the window branch of the partial":    windowBranch(t),
		"WINDOW_TILES":                        tiles,
	} {
		if strings.ContainsRune(text, rune(0x2013)) || strings.ContainsRune(text, rune(0x2014)) {
			t.Errorf("%s contains an em dash or an en dash", name)
		}
	}
}
