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

	"pinalert/internal/service"
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

// TestPostWindowCallBoxCriticalOnly (plan 03.4-12, D-24, D-24a, D-30, T-34-53):
// the Call 112 box is one tel link built from the configured primary number,
// starts hidden, sits between the shelter fields and Describe it, and exists
// only in the window branch. It shows for Critical only and never takes focus.
func TestPostWindowCallBoxCriticalOnly(t *testing.T) {
	partial := p410ReadTemplate(t, "report_form.html.tmpl")
	lowBranch, window := p410Branches(t, partial)

	const open = `<a id="call-112-box" class="call-box" href="{{.Chrome.Numbers.TelHref}}"`
	if n := strings.Count(window, open); n != 1 {
		t.Fatalf("the window branch holds %d Call 112 links built from Chrome.Numbers.TelHref, want 1", n)
	}
	start := strings.Index(window, open)
	box := window[start : start+strings.Index(window[start:], "</a>")]
	if !strings.Contains(box, " hidden>") {
		t.Errorf("the Call 112 box must start hidden")
	}
	for _, want := range []string{
		`<span class="call-box__disc" aria-hidden="true"><span class="app-icon app-icon--phone"></span></span>`,
		`>Someone in danger right now?</span>`,
		`>Call {{.Chrome.Numbers.Primary}} first, then post here.</span>`,
		`<span class="call-box__button" id="call-112-action">Call {{.Chrome.Numbers.Primary}}</span>`,
	} {
		if n := strings.Count(box, want); n != 1 {
			t.Errorf("the Call 112 box holds %q %d times, want 1", want, n)
		}
	}
	// The button is a span: a button inside a link is invalid markup.
	if strings.Contains(box, "<button") {
		t.Errorf("the Call 112 box holds a button inside a link")
	}
	// The number is the configured one: no typed number and no typed tel link,
	// in the box or anywhere in the partial.
	if strings.Contains(partial, "tel:") {
		t.Errorf("the partial holds a typed tel: link; the target is Chrome.Numbers.TelHref")
	}
	words := regexp.MustCompile(`\s(id|aria-labelledby|class)="[^"]*"`).ReplaceAllString(box, "")
	if regexp.MustCompile(`\d{3}`).MatchString(strings.NewReplacer("{{", "", "}}", "").Replace(words)) {
		t.Errorf("the Call 112 box types a number; it comes from the numbers setting")
	}
	// The link's accessible name is its three parts, button words first.
	if !strings.Contains(box, `aria-labelledby="call-112-action call-112-title call-112-line"`) {
		t.Errorf("the link must be named by its button words, its question and its line")
	}

	// It sits after the shelter fields and before Describe it, and the Low data
	// branch has no box at all.
	if !(strings.Index(window, `id="shelter-fields"`) < start && start < strings.Index(window, `id="description"`)) {
		t.Errorf("the Call 112 box must sit between the shelter fields and Describe it")
	}
	for _, banned := range []string{"call-112", "call-box", "Someone in danger", "happening-", "What is happening?"} {
		if strings.Contains(lowBranch, banned) {
			t.Errorf("the Low data branch holds %q; the box and the picks are window only", banned)
		}
	}

	// The script shows it for Critical only and gives it no focus.
	form := p410CodeText(p410ReadStatic(t, "static/js/report-form.js"))
	renderPart, ok := p410Window(form, "function renderCriticalPart() {", "\n  }\n")
	if !ok {
		t.Fatalf("report-form.js has no renderCriticalPart")
	}
	for _, want := range []string{
		"var critical = windowSeverity === 'critical';",
		"call112Box.hidden = !critical;",
		"happeningFields.hidden = !critical;",
		"setHappening(null);",
	} {
		if !strings.Contains(renderPart, want) {
			t.Errorf("renderCriticalPart lacks %q", want)
		}
	}
	if strings.Contains(form, "call112Box.focus") {
		t.Errorf("report-form.js focuses the Call 112 box; it must not take focus")
	}
	// setSeverity is the one place the severity changes in the window, so every
	// way in (tap, arrow key, reset, prefill) shows and hides the box.
	setter, _ := p410Window(form, "function setSeverity(key) {", "\n  }\n")
	if !strings.Contains(setter, "renderCriticalPart();") {
		t.Errorf("setSeverity must call renderCriticalPart")
	}

	// The display rule carries :not([hidden]) so the hidden attribute wins.
	rules := parseCSSRules(p410ReadStatic(t, "static/css/modal.css"))
	display, ok := ruleBySelector(rules, ".post-window .call-box:not([hidden])")
	if !ok {
		t.Fatalf("modal.css has no .post-window .call-box:not([hidden]) rule")
	}
	dd := declsOf(display.declBody)
	for name, want := range map[string]string{
		"display":       "flex",
		"gap":           "12px",
		"padding":       "8px 8px 8px var(--space-12)",
		"border":        "1px solid var(--color-severity-critical)",
		"border-radius": "12px",
		"background":    "var(--color-severity-critical-bg)",
	} {
		if dd[name] != want {
			t.Errorf("the Call 112 box %s is %q, want %q", name, dd[name], want)
		}
	}
	if _, bare := ruleBySelector(rules, ".post-window .call-box"); bare {
		t.Errorf("a bare .post-window .call-box rule would defeat the hidden attribute; use :not([hidden])")
	}
	disc, ok := ruleBySelector(rules, ".post-window .call-box__disc")
	if !ok || declsOf(disc.declBody)["width"] != "36px" || declsOf(disc.declBody)["height"] != "36px" ||
		declsOf(disc.declBody)["background"] != "var(--color-alert-fill)" {
		t.Errorf("the disc must be 36px on --color-alert-fill")
	}
	button, ok := ruleBySelector(rules, ".post-window .call-box__button")
	if !ok {
		t.Fatalf("modal.css has no .post-window .call-box__button rule")
	}
	bd := declsOf(button.declBody)
	if bd["min-height"] != "44px" || bd["background"] != "var(--color-alert-fill)" || bd["color"] != "var(--color-on-alert-fill)" ||
		bd["border-radius"] != "8px" || bd["font-weight"] != "var(--font-weight-semibold)" {
		t.Errorf("the Call 112 button look is %v", bd)
	}
	mask, ok := ruleBySelector(rules, ".app-icon--phone")
	if !ok || declsOf(mask.declBody)["mask-image"] != `url("/static/icons/phone.svg")` {
		t.Errorf("modal.css must mask phone.svg for the disc")
	}
}

// happeningFromScript reads the ordered value and word pairs of the
// HAPPENING_PICKS constant in report-form.js.
func happeningFromScript(t *testing.T) (values, words []string, block string) {
	t.Helper()
	form := p410ReadStatic(t, "static/js/report-form.js")
	block, ok := p410Window(form, "var HAPPENING_PICKS = [", "];")
	if !ok {
		t.Fatalf("report-form.js has no HAPPENING_PICKS constant")
	}
	entry := regexp.MustCompile(`\{ value: '([a-z_]+)', word: '([^']+)' \}`)
	for _, m := range entry.FindAllStringSubmatch(block, -1) {
		values = append(values, m[1])
		words = append(words, m[2])
	}
	return values, words, block
}

// TestPostWindowHappeningPicks (plan 03.4-12, D-24, D-24b, D-05, T-34-52): What
// is happening? offers the seven picks of the server's closed set from one
// constant, in order, optional, sent only with Critical, with no hint line.
func TestPostWindowHappeningPicks(t *testing.T) {
	values, words, block := happeningFromScript(t)
	wantValues := []string{"water_rising", "people_trapped", "fire", "building_collapsed", "landslide", "medical_emergency", "other"}
	wantWords := []string{"Water rising", "People trapped", "Fire", "Building collapsed", "Landslide", "Medical emergency", "Other"}
	if !reflect.DeepEqual(values, wantValues) {
		t.Errorf("HAPPENING_PICKS values = %v, want %v", values, wantValues)
	}
	if !reflect.DeepEqual(words, wantWords) {
		t.Errorf("HAPPENING_PICKS words = %v, want %v", words, wantWords)
	}
	if strings.ContainsRune(block, rune(0x2013)) || strings.ContainsRune(block, rune(0x2014)) {
		t.Errorf("HAPPENING_PICKS holds an em dash or an en dash")
	}

	// The server's closed set is the same seven values in the same order
	// (service.Happenings, plan 03.4-02): the client sends nothing else.
	var server []string
	for _, h := range service.Happenings {
		server = append(server, string(h))
	}
	if !reflect.DeepEqual(values, server) {
		t.Errorf("HAPPENING_PICKS values = %v, the server's service.Happenings = %v", values, server)
	}

	window := windowBranch(t)
	if n := strings.Count(window, ">What is happening?</legend>"); n != 1 {
		t.Fatalf("the window branch holds %d What is happening? legends, want 1", n)
	}
	start := strings.Index(window, `<fieldset id="happening-fields" class="post-group" hidden>`)
	if start == -1 {
		t.Fatalf("the window branch has no hidden fieldset#happening-fields")
	}
	set := window[start : start+strings.Index(window[start:], "</fieldset>")]
	if !strings.Contains(set, `<div id="happening-group" role="radiogroup" aria-labelledby="happening-legend"></div>`) {
		t.Errorf("the picks must be a radiogroup named by the legend")
	}
	// No hint line under the legend or the picks (D-05), and no button of its
	// own: the picks are built by the script from the one constant.
	for _, banned := range []string{"<p", "hint", "<button", "optional", "Optional"} {
		if strings.Contains(set, banned) {
			t.Errorf("the happening fieldset holds %q; the picks carry no hint text (D-05)", banned)
		}
	}
	if !(strings.Index(window, `id="call-112-box"`) < start && start < strings.Index(window, `id="description"`)) {
		t.Errorf("the picks must sit right under the Call 112 box and above Describe it")
	}

	// The picks never appear for Low or Medium and are sent for Critical only.
	form := p410CodeText(p410ReadStatic(t, "static/js/report-form.js"))
	for _, want := range []string{
		"if (windowMode && currentSeverity() === 'critical' && selectedHappening !== null) {",
		"payload.happening = selectedHappening;",
		"setHappening(value === selectedHappening ? null : value);",
		"if (windowMode && windowSeverity === 'critical' && typeof prefill.happening === 'string') {",
		"selectedHappening = isHappeningValue(value) ? value : null;",
		"pick.setAttribute('role', 'radio');",
		"Pinalert.setText(pick, entry.word);",
		"e.key === 'ArrowRight'",
	} {
		if !strings.Contains(form, want) {
			t.Errorf("report-form.js lacks %q", want)
		}
	}
	if n := strings.Count(form, "payload.happening"); n != 1 {
		t.Errorf("report-form.js sets payload.happening %d times, want 1 (in buildPayload)", n)
	}
	if n := strings.Count(form, "var HAPPENING_PICKS = ["); n != 1 {
		t.Errorf("report-form.js declares HAPPENING_PICKS %d times, want 1", n)
	}
	// The Low data mode never builds the picks.
	if !strings.Contains(form, "} else if (happeningGroup) {\n      buildHappeningPicks();") {
		t.Errorf("the picks are built only in window mode, beside the severity control of the Low data mode")
	}
}

// TestPostWindowPicksSoftFill (plan 03.4-12, D-04): a chosen pick has the soft
// grey fill and weight 600 with the border unchanged, like the tiles; never a
// dark fill, a dark border or a 2px border. A pick is 44px high with radius 8px.
func TestPostWindowPicksSoftFill(t *testing.T) {
	rules := parseCSSRules(p410ReadStatic(t, "static/css/modal.css"))
	chosen, ok := ruleBySelector(rules, `.post-window .happening-pick[aria-checked="true"]`)
	if !ok {
		t.Fatalf("modal.css has no chosen rule for .happening-pick")
	}
	decls := declsOf(chosen.declBody)
	if decls["background"] != "var(--color-surface)" {
		t.Errorf("the chosen pick background is %q, want var(--color-surface)", decls["background"])
	}
	if decls["font-weight"] != "var(--font-weight-semibold)" {
		t.Errorf("the chosen pick weight is %q, want var(--font-weight-semibold)", decls["font-weight"])
	}
	if decls["border-color"] != "var(--color-border)" {
		t.Errorf("the chosen pick border colour is %q, want var(--color-border)", decls["border-color"])
	}
	for name, value := range decls {
		if strings.Contains(name, "border") && (strings.Contains(value, "2px") || value == "var(--color-text)") {
			t.Errorf("the chosen pick declares %s: %s; no dark and no 2px border", name, value)
		}
		if strings.Contains(name, "outline") || name == "box-shadow" {
			t.Errorf("the chosen pick declares %s; it is a fill only", name)
		}
	}
	base, ok := ruleBySelector(rules, ".post-window .happening-pick")
	if !ok {
		t.Fatalf("modal.css has no .post-window .happening-pick rule")
	}
	bd := declsOf(base.declBody)
	if bd["min-height"] != "44px" || bd["border-radius"] != "8px" || bd["border"] != "1px solid var(--color-border)" ||
		bd["font-size"] != "var(--font-size-label)" || bd["padding"] != "0 var(--space-12)" {
		t.Errorf("the pick geometry is %v", bd)
	}
	group, ok := ruleBySelector(rules, ".post-window #happening-group")
	if !ok || declsOf(group.declBody)["flex-wrap"] != "wrap" || declsOf(group.declBody)["gap"] != "var(--space-sm)" {
		t.Errorf("the picks must sit in a wrapping row with a gap of var(--space-sm)")
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
