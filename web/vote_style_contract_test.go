package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// Plan 03-11 contract tests for the vote button style in trust.css. This file
// owns a media aware CSS parser (p11ParseRules) because the shared
// parseCSSRules keeps the at-rule prefix only on the first rule of a block.

// p11Rule is one parsed rule. media is "" for a top level rule and the
// trimmed at-rule prelude (for example "@media (forced-colors: active)") for a
// rule inside an at-rule block. head has its whitespace collapsed to single
// spaces, so a comma separated head written over several lines compares equal
// to the same head written on one line. order is the source position.
type p11Rule struct {
	media string
	head  string
	body  string
	order int
}

// p11ReadCSS reads an embedded stylesheet from StaticFS.
func p11ReadCSS(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return string(raw)
}

// p11ParseRules strips comments, walks the text and returns every style rule
// with its at-rule prelude. It supports one level of at-rule nesting and fails
// the test on a block nested two levels deep, because nothing in trust.css
// needs it and silently mis-parsing it would hide a rule.
func p11ParseRules(t *testing.T, css string) []p11Rule {
	t.Helper()
	text := stripCSSComments(css)
	var rules []p11Rule
	media := ""
	inMedia := false
	start := 0
	i := 0
	for i < len(text) {
		switch text[i] {
		case '{':
			head := strings.TrimSpace(text[start:i])
			if strings.HasPrefix(head, "@") {
				if inMedia {
					t.Fatalf("at-rule %q is nested inside %q, which p11ParseRules does not support", head, media)
				}
				inMedia = true
				media = strings.Join(strings.Fields(head), " ")
				i++
				start = i
				continue
			}
			end := strings.Index(text[i+1:], "}")
			if end < 0 {
				t.Fatalf("unterminated rule %q", head)
			}
			body := text[i+1 : i+1+end]
			if strings.Contains(body, "{") {
				t.Fatalf("rule %q holds a nested block, which p11ParseRules does not support", head)
			}
			rules = append(rules, p11Rule{
				media: media,
				head:  strings.Join(strings.Fields(head), " "),
				body:  body,
				order: len(rules),
			})
			i = i + 1 + end + 1
			start = i
			continue
		case '}':
			if inMedia {
				inMedia = false
				media = ""
			}
			i++
			start = i
			continue
		}
		i++
	}
	return rules
}

// p11Norm collapses whitespace so callers can write a head on one line.
func p11Norm(head string) string {
	return strings.Join(strings.Fields(head), " ")
}

// p11TopLevel returns the first top level rule whose head equals head exactly.
func p11TopLevel(rules []p11Rule, head string) (p11Rule, bool) {
	want := p11Norm(head)
	for _, r := range rules {
		if r.media == "" && r.head == want {
			return r, true
		}
	}
	return p11Rule{}, false
}

// p11InMedia returns the first rule inside a block whose prelude starts with
// mediaPrefix and whose head equals head exactly.
func p11InMedia(rules []p11Rule, mediaPrefix, head string) (p11Rule, bool) {
	want := p11Norm(head)
	for _, r := range rules {
		if r.media != "" && strings.HasPrefix(r.media, mediaPrefix) && r.head == want {
			return r, true
		}
	}
	return p11Rule{}, false
}

// p11VoteHead reports whether a head styles the vote controls.
func p11VoteHead(head string) bool {
	for _, frag := range []string{".vote-btn", ".vote-controls", ".vote-error", ".vote-note", ".resolve-confirm"} {
		if strings.Contains(head, frag) {
			return true
		}
	}
	return false
}

// p11MustTop fetches a top level rule or fails the test.
func p11MustTop(t *testing.T, rules []p11Rule, head string) p11Rule {
	t.Helper()
	r, ok := p11TopLevel(rules, head)
	if !ok {
		t.Fatalf("no exact top level rule %q found in static/css/trust.css", head)
	}
	return r
}

// p11ExpectDecls asserts exact declaration values for one rule.
func p11ExpectDecls(t *testing.T, r p11Rule, want map[string]string, why string) {
	t.Helper()
	got := declsOf(r.body)
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s: %s must be exactly %q (%s), found %q", r.head, name, value, why, got[name])
		}
	}
}

// p11MaskBase is the six mask longhands both glyph bases repeat.
var p11MaskBase = map[string]string{
	"-webkit-mask-size":     "contain",
	"mask-size":             "contain",
	"-webkit-mask-repeat":   "no-repeat",
	"mask-repeat":           "no-repeat",
	"-webkit-mask-position": "center",
	"mask-position":         "center",
}

var p11PxRE = regexp.MustCompile(`[0-9]+px`)

// TestVoteButtonsShareOneOutlinedStyle (A12, A14, A15, A17, A18, A28, A35,
// A53) pins the one outlined style every vote button shares, the two track
// grid, the glyph mask rules and the spacing and display discipline.
//
// Honest limit: static inspection proves the declarations, not the rendering.
// Plan 03-20's V-42 session covers what a browser paints.
func TestVoteButtonsShareOneOutlinedStyle(t *testing.T) {
	css := p11ReadCSS(t, "static/css/trust.css")
	rules := p11ParseRules(t, css)

	controls := p11MustTop(t, rules, ".vote-controls:not([hidden])")
	p11ExpectDecls(t, controls, map[string]string{
		"display":               "grid",
		"grid-template-columns": "repeat(2, minmax(0, 1fr))",
		"gap":                   "var(--space-sm)",
		"margin-top":            "var(--space-sm)",
	}, "A17 two track grid")
	if _, has := declsOf(controls.body)["flex-wrap"]; has {
		t.Errorf(".vote-controls:not([hidden]) must not keep flex-wrap once it is a grid (A17)")
	}

	btn := p11MustTop(t, rules, ".vote-btn:not([hidden])")
	p11ExpectDecls(t, btn, map[string]string{
		"display":       "inline-flex",
		"gap":           "var(--space-sm)",
		"min-width":     "0",
		"min-height":    "var(--touch-target-min)",
		"padding":       "0 var(--space-sm)",
		"border":        "1px solid var(--color-border)",
		"border-radius": "8px",
		"background":    "var(--color-surface)",
		"color":         "var(--color-text)",
		"font-size":     "var(--font-size-body)",
		"line-height":   "var(--line-height-body)",
		"font-weight":   "var(--font-weight-semibold)",
	}, "A15 and A18 outlined button")

	disabled := p11MustTop(t, rules, ".vote-btn:disabled")
	p11ExpectDecls(t, disabled, map[string]string{
		"background": "var(--color-bg)",
		"color":      "var(--color-text-muted)",
	}, "A18 disabled look")

	resolve := p11MustTop(t, rules, ".vote-btn--resolve:not([hidden])")
	p11ExpectDecls(t, resolve, map[string]string{
		"background":   "var(--color-surface)",
		"color":        "var(--color-text)",
		"border-color": "var(--color-border)",
		"grid-column":  "1 / -1",
	}, "A14 and A17 outlined, full width Mark resolved")

	reopen := p11MustTop(t, rules, ".vote-btn--reopen")
	p11ExpectDecls(t, reopen, map[string]string{
		"grid-column": "1 / -1",
	}, "A28 full width Reopen")

	icon := p11MustTop(t, rules, ".vote-btn__icon")
	iconWant := map[string]string{
		"flex":             "none",
		"width":            "20px",
		"height":           "20px",
		"background-color": "currentColor",
	}
	for k, v := range p11MaskBase {
		iconWant[k] = v
	}
	p11ExpectDecls(t, icon, iconWant, "A12 glyph mask base")
	if _, has := declsOf(icon.body)["display"]; has {
		t.Errorf(".vote-btn__icon must not declare display (a flex item is blockified)")
	}

	for _, name := range []string{"check", "ban", "shield-check", "rotate-ccw"} {
		mod := p11MustTop(t, rules, ".vote-btn__icon--"+name)
		want := `url("/static/icons/` + name + `.svg")`
		p11ExpectDecls(t, mod, map[string]string{
			"-webkit-mask-image": want,
			"mask-image":         want,
		}, "A39 literal url mask")
		if _, err := fs.Stat(StaticFS, "static/icons/"+name+".svg"); err != nil {
			t.Errorf("static/icons/%s.svg is missing from the embedded tree: %v", name, err)
		}
	}

	label := p11MustTop(t, rules, ".vote-btn__label")
	p11ExpectDecls(t, label, map[string]string{
		"min-width":     "0",
		"overflow-wrap": "anywhere",
	}, "A28 content driven wrap")

	// A53: spacing is tokens only on every top level vote rule.
	spacingProps := []string{
		"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
		"margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
		"gap", "row-gap", "column-gap",
	}
	for _, r := range rules {
		if r.media != "" || !p11VoteHead(r.head) {
			continue
		}
		decls := declsOf(r.body)
		for _, prop := range spacingProps {
			if v, ok := decls[prop]; ok && p11PxRE.MatchString(v) {
				t.Errorf("%s: %s is %q, but spacing must be a token (A53)", r.head, prop, v)
			}
		}
	}

	// A35: every vote rule that declares display carries the hidden guard.
	for _, r := range rules {
		if !p11VoteHead(r.head) {
			continue
		}
		if _, ok := declsOf(r.body)["display"]; ok && !strings.Contains(r.head, ":not([hidden])") {
			t.Errorf("%s (media %q): declares display without the :not([hidden]) guard (A35)", r.head, r.media)
		}
	}

	stripped := stripCSSComments(css)
	if strings.Contains(stripped, "!important") {
		t.Errorf("trust.css must not use !important")
	}
	if strings.Contains(stripped, "999px") {
		t.Errorf("trust.css must not use 999px (no pill shaped controls)")
	}
}
