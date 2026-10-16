package web

// Phase wide source gates for the files Phase 4 added (plan 04-27). Two rules
// are checked over the same list of 18 templates, scripts and stylesheets:
// no bare generic label (UI-SPEC P4-A23) and no light or lite name for the
// Low data view (D-13).
//
// Honest limit of this test's claim: the gates prove the scanned source text of
// the listed files only, not every string a page can render at run time, and a
// Phase 4 file that is missing from p427Phase4Files is not scanned at all. The
// git comparison in plan 04-27 Step 0 guards that second gap. The gates are
// scoped to files Phase 4 added because Phase 3 keeps its own shipped label in
// votes.js. Every helper starts with p427 so it never collides with another
// file in this package. The 04-08 helper stripJSComments and the existing
// stripCSSComments are reused, never redeclared.

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// p427Phase4Files lists the 18 files Phase 4 added, relative to the web
// package. Templates are read from TemplatesFS, everything else from StaticFS.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30, D-31):
// the bar on every page is replaced by the laptop strip on Nearby and Post a
// report only. Phase 4 SC7 (both legal pages linked from the bar on every page)
// is retired as worded; Disclaimer and Policy are reached from the laptop
// strip's Read the disclaimer link, Settings, About Pinalert, the first-time
// flow, the login screen and the one reader. The Phase 3.1 and Phase 4
// hands-on tests are walked on this layout after Phase 3.4 is built. The first
// entry was the bar partial; its successor, the strip partial, takes its place
// so the gates still scan the notice copy.
var p427Phase4Files = []string{
	"templates/notice_strip.html.tmpl",
	"templates/report_form.html.tmpl",
	"templates/reader.html.tmpl",
	"templates/legal_body.html.tmpl",
	"templates/low.html.tmpl",
	"templates/moderate.html.tmpl",
	"templates/flag_dialog.html.tmpl",
	"static/js/report-form.js",
	"static/js/slow-notice.js",
	"static/js/low.js",
	"static/js/queue.js",
	"static/js/flag.js",
	"static/js/moderate.js",
	"static/css/chrome.css",
	"static/css/modal-map.css",
	"static/css/legal.css",
	"static/css/low.css",
	"static/css/moderate.css",
}

// p427ViewWordAllowlist holds the only phrases allowed to mention a theme by
// name: the Policy text tells the reader about their Light or Dark choice. Amended
// 2026-10-17 by Phase 3.4 plan 08 (D-31): the Policy text now lives in the shared
// body partial, so the allowance moves from the old policy page to it.
var p427ViewWordAllowlist = map[string][]string{
	"templates/legal_body.html.tmpl": {"Light or Dark"},
}

// p427CSSThemeSyntax is CSS theme syntax that names the Light theme. It is
// removed from stylesheet input before the banned word check.
var p427CSSThemeSyntax = []string{
	`[data-theme="light"]`,
	`[data-theme='light']`,
	`color-scheme: light`,
}

func p427ReadPhase4File(t *testing.T, path string) string {
	t.Helper()
	var raw []byte
	var err error
	if strings.HasPrefix(path, "templates/") {
		raw, err = fs.ReadFile(TemplatesFS, path)
	} else {
		raw, err = fs.ReadFile(StaticFS, path)
	}
	if err != nil {
		t.Fatalf("reading Phase 4 file %s: %v", path, err)
	}
	return string(raw)
}

var (
	p427TemplateComment = regexp.MustCompile(`(?s)\{\{-?\s*/\*.*?\*/\s*-?\}\}`)
	p427HTMLComment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	p427TextNode        = regexp.MustCompile(`>([^<>]+)`)
	p427LabelAttr       = regexp.MustCompile(`\b(?:aria-label|title|value|alt)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	p427JSString        = regexp.MustCompile("'(?:[^'\\\\\\n]|\\\\.)*'|\"(?:[^\"\\\\\\n]|\\\\.)*\"|`(?:[^`\\\\\\n]|\\\\.)*`")
	p427CSSContent      = regexp.MustCompile(`content\s*:\s*(?:"([^"]*)"|'([^']*)')`)
	p427TemplateAction  = regexp.MustCompile(`(?s)\{\{.*?\}\}`)
)

// p427StripTemplateComments removes Go template comments and HTML comments.
func p427StripTemplateComments(s string) string {
	s = p427TemplateComment.ReplaceAllString(s, "")
	return p427HTMLComment.ReplaceAllString(s, "")
}

// p427LabelCandidates returns every string in text that a reader could see as a
// label. Comments must already be removed by the caller for templates; for
// scripts and stylesheets the matching comment stripper runs here.
func p427LabelCandidates(path, text string) []string {
	var out []string
	switch {
	case strings.HasSuffix(path, ".js"):
		text = stripJSComments(text)
		for _, lit := range p427JSString.FindAllString(text, -1) {
			out = append(out, lit[1:len(lit)-1])
		}
	case strings.HasSuffix(path, ".css"):
		text = stripCSSComments(text)
		for _, m := range p427CSSContent.FindAllStringSubmatch(text, -1) {
			if m[1] != "" {
				out = append(out, m[1])
			} else {
				out = append(out, m[2])
			}
		}
	default:
		for _, m := range p427LabelAttr.FindAllStringSubmatch(text, -1) {
			if m[1] != "" {
				out = append(out, m[1])
			} else {
				out = append(out, m[2])
			}
		}
		for _, m := range p427TextNode.FindAllStringSubmatch(text, -1) {
			node := strings.TrimSpace(p427TemplateAction.ReplaceAllString(m[1], " "))
			if node != "" {
				out = append(out, node)
			}
		}
	}
	return out
}

// p427IsBareLabel is true for a capitalised generic label. The leading capital
// rule is deliberate: event names, the type attribute and the pinned ok status
// are lower case code words, while a label a reader sees is capitalised.
func p427IsBareLabel(s string) bool {
	s = strings.TrimSpace(s)
	r, _ := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError || !unicode.IsUpper(r) {
		return false
	}
	switch strings.ToLower(s) {
	case "cancel", "submit", "ok", "click here", "save":
		return true
	}
	return false
}

func TestPhase4NoBareLabels(t *testing.T) {
	t.Run("matcher", func(t *testing.T) {
		for _, s := range []string{"Cancel", "CANCEL", "Save", "Submit", "OK", "Ok", "Click Here", "Click here", "  Cancel  "} {
			if !p427IsBareLabel(s) {
				t.Errorf("p427IsBareLabel(%q) = false, want true", s)
			}
		}
		for _, s := range []string{"cancel", "submit", "ok", "save", "Cancel report", "Yes, remove report", "Discard", "Keep editing", ""} {
			if p427IsBareLabel(s) {
				t.Errorf("p427IsBareLabel(%q) = true, want false", s)
			}
		}

		flagged := func(path, text string) []string {
			var hits []string
			for _, c := range p427LabelCandidates(path, text) {
				if p427IsBareLabel(c) {
					hits = append(hits, c)
				}
			}
			return hits
		}
		if got := flagged("templates/x.html.tmpl", `<button type="button" class="btn">Cancel</button>`); len(got) != 1 || got[0] != "Cancel" {
			t.Errorf("template text node: got %v, want [Cancel]", got)
		}
		if got := flagged("templates/x.html.tmpl", `<button aria-label="OK">x</button>`); len(got) != 1 || got[0] != "OK" {
			t.Errorf("aria-label: got %v, want [OK]", got)
		}
		if got := flagged("static/js/x.js", "btn.textContent = 'Submit';"); len(got) != 1 || got[0] != "Submit" {
			t.Errorf("js string: got %v, want [Submit]", got)
		}
		if got := flagged("static/js/x.js", "form.addEventListener('submit', f);\nif (state.status === 'ok') {}\n"); len(got) != 0 {
			t.Errorf("js code words: got %v, want none", got)
		}
		if got := flagged("templates/x.html.tmpl", `<button type="submit" data-feed-state="ok">Send report</button>`); len(got) != 0 {
			t.Errorf("template code words: got %v, want none", got)
		}
		if got := flagged("static/css/x.css", `.a::before { content: "Save"; }`); len(got) != 1 {
			t.Errorf("css content: got %v, want one hit", got)
		}
		if got := p427StripTemplateComments("a{{/* Cancel */}}b<!-- OK -->c"); got != "abc" {
			t.Errorf("p427StripTemplateComments = %q, want abc", got)
		}
	})

	t.Run("files", func(t *testing.T) {
		for _, path := range p427Phase4Files {
			text := p427ReadPhase4File(t, path)
			if strings.HasPrefix(path, "templates/") {
				text = p427StripTemplateComments(text)
			}
			candidates := p427LabelCandidates(path, text)
			floor := 0
			switch {
			case strings.HasPrefix(path, "templates/"):
				floor = 1
			case strings.HasSuffix(path, ".js"):
				floor = 1
			}
			if len(candidates) < floor {
				t.Errorf("%s: %d candidates, want at least %d (the scan would pass empty)", path, len(candidates), floor)
			}
			for i, c := range candidates {
				if p427IsBareLabel(c) {
					t.Errorf("%s: candidate %d is the bare label %q", path, i+1, c)
				}
			}
		}
	})
}

// p427ViewWordTokens splits s on every character that is not a letter or digit,
// then on lower to upper case boundaries, and lower cases each part (the same
// rule as the Low data tokenizer, so polite and highlight stay single tokens).
func p427ViewWordTokens(s string) []string {
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

func p427BannedViewToken(s string) string {
	for _, tok := range p427ViewWordTokens(s) {
		if tok == "light" || tok == "lite" {
			return tok
		}
	}
	return ""
}

// p427CleanForViewWords strips the comments of the file kind and removes the
// allowlisted phrases and the CSS theme syntax. It fails when an allowlisted
// phrase is no longer present (a stale entry).
func p427CleanForViewWords(t *testing.T, path, text string) string {
	t.Helper()
	switch {
	case strings.HasSuffix(path, ".js"):
		text = stripJSComments(text)
	case strings.HasSuffix(path, ".css"):
		text = stripCSSComments(text)
		for _, syn := range p427CSSThemeSyntax {
			text = strings.ReplaceAll(text, syn, " ")
		}
	default:
		text = p427StripTemplateComments(text)
	}
	for _, phrase := range p427ViewWordAllowlist[path] {
		if !strings.Contains(text, phrase) {
			t.Errorf("%s: the allowlisted phrase %q is no longer present (stale entry)", path, phrase)
		}
		text = strings.ReplaceAll(text, phrase, " ")
	}
	return text
}

func TestPhase4NoBannedViewWords(t *testing.T) {
	t.Run("tokenizer", func(t *testing.T) {
		for _, s := range []string{"lowLiteMode", "low_light", "low-lite", "LiteView", "Light view"} {
			if p427BannedViewToken(s) == "" {
				t.Errorf("%q should be flagged", s)
			}
		}
		for _, s := range []string{`aria-live="polite"`, "highlight", "Low data view"} {
			if got := p427BannedViewToken(s); got != "" {
				t.Errorf("%q flagged as %q, want a pass", s, got)
			}
		}
		for _, syn := range []string{`:root[data-theme="light"] { color: red; }`, `:root { color-scheme: light; }`} {
			cleaned := syn
			for _, s := range p427CSSThemeSyntax {
				cleaned = strings.ReplaceAll(cleaned, s, " ")
			}
			if got := p427BannedViewToken(cleaned); got != "" {
				t.Errorf("CSS theme syntax %q left %q after removal", syn, got)
			}
		}
		phrase := "Light or Dark"
		if got := p427ViewWordAllowlist["templates/legal_body.html.tmpl"]; len(got) != 1 || got[0] != phrase {
			t.Errorf("the policy allowlist = %v, want only %q", got, phrase)
		}
		if len(p427ViewWordAllowlist) != 1 {
			t.Errorf("the allowlist has %d files, want exactly the policy page", len(p427ViewWordAllowlist))
		}
		if p427BannedViewToken(phrase) == "" {
			t.Errorf("%q should be flagged outside the policy path", phrase)
		}
	})

	t.Run("files", func(t *testing.T) {
		for _, path := range p427Phase4Files {
			text := p427CleanForViewWords(t, path, p427ReadPhase4File(t, path))
			for n, line := range strings.Split(text, "\n") {
				if tok := p427BannedViewToken(line); tok != "" {
					t.Errorf("%s:%d: the word %q is banned in Phase 4 files (D-13): %s", path, n+1, tok, strings.TrimSpace(line))
				}
			}
		}
	})

	t.Run("paths", func(t *testing.T) {
		for _, path := range p427Phase4Files {
			if tok := p427BannedViewToken(path); tok != "" {
				t.Errorf("path %s holds the banned word %q", path, tok)
			}
		}
	})
}
