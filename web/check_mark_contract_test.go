package web

// check_mark_contract_test.go pins the words and the wiring of quick
// 261118-saj (owner, 2026-11-18): flags from three places on a Critical report
// add the Being checked mark instead of hiding it. The behaviour of the mark
// is in check_mark_test.mjs; this file holds the copy, the Policy sentence,
// the places the mark is drawn and the stylesheet rules.

import (
	"io/fs"
	"strings"
	"testing"
)

func cmRead(t *testing.T, fsys fs.FS, name string) string {
	t.Helper()
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(raw)
}

// TestCheckMarkContract pins the mark's words and where it is drawn.
func TestCheckMarkContract(t *testing.T) {
	trust := cmRead(t, StaticFS, "static/js/trust.js")
	// The name and the one line, each written once, and nothing else said.
	for _, want := range []string{"var CHECK_NAME = 'Being checked';", "var CHECK_WORDS = 'Being checked.';",
		"setAttribute('aria-label', CHECK_NAME)", "setAttribute('aria-describedby', tipId)",
		"setAttribute('role', 'tooltip')", "Pinalert.setText(tip, CHECK_WORDS)",
		"report.being_checked === true", "evt.stopPropagation()", "if (closeCheckLine()) {"} {
		if !strings.Contains(trust, want) {
			t.Errorf("trust.js lacks %q", want)
		}
	}
	if n := strings.Count(trust, "'Being checked"); n != 2 {
		t.Errorf("trust.js writes the Being checked words %d times, want the name and the line once each", n)
	}
	// Every surface that draws the severity tag of a report a reader sees
	// draws the mark right after it and refreshes it with the tag.
	for _, file := range []string{"static/js/feed.js", "static/js/map.js", "static/js/low.js"} {
		src := cmRead(t, StaticFS, file)
		if strings.Count(src, "PinalertTrust.createCheckMark()") != 1 || strings.Count(src, "PinalertTrust.updateCheckMark(") != 1 {
			t.Errorf("%s does not build and refresh the mark exactly once", file)
		}
		create := strings.Index(src, "PinalertTrust.createSeverityLabel()")
		mark := strings.Index(src, "PinalertTrust.createCheckMark()")
		if create < 0 || mark < create {
			t.Errorf("%s builds the mark before the severity tag", file)
		}
		update := strings.Index(src, "PinalertTrust.updateSeverityLabel(")
		refresh := strings.Index(src, "PinalertTrust.updateCheckMark(")
		if update < 0 || refresh < update {
			t.Errorf("%s refreshes the mark before the severity tag", file)
		}
	}
	// The moderator's tag.
	mod := cmRead(t, StaticFS, "static/js/moderate.js")
	for _, want := range []string{"var STILL_SHOWING_WORD = 'Still showing';", "report.being_checked === true"} {
		if !strings.Contains(mod, want) {
			t.Errorf("moderate.js lacks %q", want)
		}
	}
	// The stylesheet: the glyph by a literal mask url, a 44px hit area, and the
	// feed row's dot moved from the tag to the mark so the two read as one part.
	css := cmRead(t, StaticFS, "static/css/trust.css")
	for _, want := range []string{`mask-image: url("/static/icons/flag.svg");`, ".check-mark__button::before",
		"width: var(--touch-target-min);", "height: var(--touch-target-min);", ".check-mark__tip:not([hidden])",
		"white-space: nowrap;", ".check-mark__button:focus-visible"} {
		if !strings.Contains(css, want) {
			t.Errorf("trust.css lacks %q", want)
		}
	}
	feed := cmRead(t, StaticFS, "static/css/feed.css")
	for _, want := range []string{"#report-list .report-row .severity-label:not([hidden]):has(+ .check-mark:not([hidden]))::after",
		"#report-list .report-row .check-mark:not([hidden])::after"} {
		if !strings.Contains(feed, want) {
			t.Errorf("feed.css lacks %q", want)
		}
	}
	// The profile's live row: the mark sits in the label line inside the row's
	// link, as text and a line, never a second control.
	person := cmRead(t, TemplatesFS, "templates/person.html.tmpl")
	const personMark = `<span class="person-report__label">{{.LabelLine}}{{if .BeingChecked}} <span class="person-report__check" data-check-mark><span class="person-report__check-glyph" aria-hidden="true"></span><span class="visually-hidden">Being checked.</span><span class="person-report__check-tip" aria-hidden="true" hidden>Being checked.</span></span>{{end}}</span>`
	if strings.Count(person, personMark) != 1 {
		t.Error("person.html.tmpl does not draw the mark once in the live row's label line")
	}
	if strings.Contains(person, `class="person-report__check"><button`) || strings.Contains(person, `data-check-mark><button`) {
		t.Error("the profile row's mark must not be a button inside the link")
	}
	personCSS := cmRead(t, StaticFS, "static/css/person.css")
	for _, want := range []string{`mask-image: url("/static/icons/flag.svg");`, ".person-report__check-tip:not([hidden])", "bottom: calc(100% + var(--space-sm));"} {
		if !strings.Contains(personCSS, want) {
			t.Errorf("person.css lacks %q", want)
		}
	}
	personJS := cmRead(t, StaticFS, "static/js/person-reports.js")
	for _, want := range []string{"querySelectorAll('[data-check-mark]')", "evt.preventDefault();", "evt.key === 'Escape'", "wireCheckMarks: wireCheckMarks"} {
		if !strings.Contains(personJS, want) {
			t.Errorf("person-reports.js lacks %q", want)
		}
	}

	low := cmRead(t, StaticFS, "static/css/low.css")
	if strings.Count(low, ".report-row--low .report-row__body > .check-mark:not([hidden])") != 2 {
		t.Error("low.css does not place the mark on the laptop and the phone card")
	}
}

// TestCheckMarkPolicyAndNoDashes pins the one amended Policy sentence and
// that none of the new words carry an em or en dash.
func TestCheckMarkPolicyAndNoDashes(t *testing.T) {
	policy := cmRead(t, TemplatesFS, "templates/legal_body.html.tmpl")
	const sentence = "A Critical report flagged this way stays visible, marked as being checked, and sends no alerts until a moderator looks."
	if n := strings.Count(policy, sentence); n != 1 {
		t.Errorf("the Policy holds the Being checked sentence %d times, want 1", n)
	}
	if !strings.Contains(policy, "it is hidden until a moderator keeps or removes it. "+sentence+" Its author is told in the bell") {
		t.Error("the Being checked sentence is not right after the hide sentence of the moderation part")
	}
	// Older lines of these files are not this task's; the new code is checked
	// whole where it is one block, and line by line where it is spread out.
	const dashes = "—–"
	trust := cmRead(t, StaticFS, "static/js/trust.js")
	start := strings.Index(trust, "---- The Being checked mark")
	end := strings.Index(trust, "The one capture phase key listener for the whole page")
	if start < 0 || end < start {
		t.Fatal("trust.js lacks the Being checked section")
	}
	blocks := map[string]string{
		"trust.js mark section": trust[start:end],
		"flag.svg":              cmRead(t, StaticFS, "static/icons/flag.svg"),
	}
	for _, css := range []string{"static/css/trust.css", "static/css/feed.css"} {
		src := cmRead(t, StaticFS, css)
		at := strings.Index(src, "261118-saj")
		if at < 0 {
			t.Fatalf("%s lacks the Being checked block", css)
		}
		blocks[css+" mark block"] = src[at:]
	}
	for name, block := range blocks {
		if strings.ContainsAny(block, dashes) {
			t.Errorf("%s carries an em or en dash", name)
		}
	}
	marks := []string{"261118-saj", "check-mark", "CheckMark", "Being checked", "being checked", "Still showing", "STILL_SHOWING", "being_checked"}
	for _, f := range []struct {
		fsys fs.FS
		name string
	}{
		{TemplatesFS, "templates/legal_body.html.tmpl"}, {TemplatesFS, "templates/moderate.html.tmpl"},
		{StaticFS, "static/js/moderate.js"}, {StaticFS, "static/js/feed.js"}, {StaticFS, "static/js/map.js"},
		{StaticFS, "static/js/low.js"}, {StaticFS, "static/css/low.css"},
		{TemplatesFS, "templates/person.html.tmpl"}, {StaticFS, "static/css/person.css"},
		{StaticFS, "static/js/person-reports.js"},
	} {
		for i, line := range strings.Split(cmRead(t, f.fsys, f.name), "\n") {
			for _, m := range marks {
				if strings.Contains(line, m) && strings.ContainsAny(line, dashes) {
					t.Errorf("%s line %d carries an em or en dash", f.name, i+1)
					break
				}
			}
		}
	}
}
