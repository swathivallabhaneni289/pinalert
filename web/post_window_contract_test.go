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
