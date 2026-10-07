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

// ---------------------------------------------------------------------------
// The cascade resolver (Task 2).
//
// Supported grammar, so a future editor knows what a head may contain: type,
// class, id, attribute ([name] and [name="value"]) and pseudo-class selectors
// from the set hover, active, focus, focus-visible and disabled; :not() with
// one compound inside; the :: pseudo-element form; compounds joined by the
// descendant combinator only. Earlier compounds of a head are treated as page
// context and assumed present. Any other syntax, and the > + ~ combinators,
// fail the test naming the head.
//
// Honest limit: this resolves declared values by specificity and source
// order. It does not render anything, inherit anything or expand shorthands
// beyond treating background, background-color, border and border-color as
// the fill and stroke groups. Heads whose last compound carries no vote-btn
// class token are skipped before parsing, so later plans can append
// unrelated trust block, label and popup rules without touching this test.
// ---------------------------------------------------------------------------

// p11Element models one rendered button for the resolver.
type p11Element struct {
	classes map[string]bool
	attrs   map[string]string
	states  map[string]bool
}

// p11Simple is one parsed simple selector.
type p11Simple struct {
	kind   byte // c class, i id, a attribute, p pseudo-class, e pseudo-element, t type, n :not
	name   string
	val    string
	hasVal bool
	inner  []p11Simple
}

var p11SupportedPseudo = map[string]bool{
	"hover": true, "active": true, "focus": true, "focus-visible": true, "disabled": true,
}

func p11IsIdent(c byte) bool {
	return c == '-' || c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// p11Split splits s on top level commas (comma true) or on top level
// whitespace (comma false), ignoring separators inside () and [].
func p11Split(s string, comma bool) []string {
	var parts []string
	depth := 0
	start := 0
	flush := func(end int) {
		if piece := strings.TrimSpace(s[start:end]); piece != "" {
			parts = append(parts, piece)
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		}
		if depth != 0 {
			continue
		}
		if comma && c == ',' {
			flush(i)
			start = i + 1
		} else if !comma && (c == ' ' || c == '\n' || c == '\t' || c == '\r') {
			flush(i)
			start = i + 1
		}
	}
	flush(len(s))
	return parts
}

// p11ParseCompound parses one compound selector such as
// .vote-btn[aria-pressed="true"]:not(:disabled):hover.
func p11ParseCompound(t *testing.T, s, whole string) []p11Simple {
	t.Helper()
	var out []p11Simple
	i := 0
	ident := func() string {
		j := i
		for j < len(s) && p11IsIdent(s[j]) {
			j++
		}
		got := s[i:j]
		i = j
		return got
	}
	for i < len(s) {
		switch c := s[i]; {
		case c == '.':
			i++
			out = append(out, p11Simple{kind: 'c', name: ident()})
		case c == '#':
			i++
			out = append(out, p11Simple{kind: 'i', name: ident()})
		case c == '[':
			end := strings.IndexByte(s[i:], ']')
			if end < 0 {
				t.Fatalf("unterminated attribute selector in %q", whole)
			}
			content := s[i+1 : i+end]
			i += end + 1
			name, val, has := strings.Cut(content, "=")
			out = append(out, p11Simple{kind: 'a', name: strings.TrimSpace(name), val: strings.Trim(strings.TrimSpace(val), `"`), hasVal: has})
		case c == ':':
			i++
			if i < len(s) && s[i] == ':' {
				i++
				out = append(out, p11Simple{kind: 'e', name: ident()})
				continue
			}
			name := ident()
			if name == "not" {
				if i >= len(s) || s[i] != '(' {
					t.Fatalf(":not without parentheses in %q", whole)
				}
				depth := 0
				j := i
				for ; j < len(s); j++ {
					if s[j] == '(' {
						depth++
					} else if s[j] == ')' {
						depth--
						if depth == 0 {
							break
						}
					}
				}
				if j >= len(s) {
					t.Fatalf("unbalanced :not( in %q", whole)
				}
				inner := p11ParseCompound(t, s[i+1:j], whole)
				i = j + 1
				out = append(out, p11Simple{kind: 'n', name: "not", inner: inner})
				continue
			}
			if !p11SupportedPseudo[name] {
				t.Fatalf("unsupported pseudo-class :%s in %q", name, whole)
			}
			out = append(out, p11Simple{kind: 'p', name: name})
		case i == 0 && p11IsIdent(c):
			out = append(out, p11Simple{kind: 't', name: ident()})
		default:
			t.Fatalf("unsupported selector syntax %q at %q in %q", string(c), s[i:], whole)
		}
	}
	return out
}

func p11Compounds(t *testing.T, sel string) [][]p11Simple {
	t.Helper()
	var out [][]p11Simple
	for _, field := range p11Split(sel, false) {
		if field == ">" || field == "+" || field == "~" {
			t.Fatalf("combinator %q is not supported in %q", field, sel)
		}
		out = append(out, p11ParseCompound(t, field, sel))
	}
	return out
}

func p11SpecOf(simples []p11Simple) [3]int {
	var spec [3]int
	for _, s := range simples {
		switch s.kind {
		case 'i':
			spec[0]++
		case 'c', 'a', 'p':
			spec[1]++
		case 't', 'e':
			spec[2]++
		case 'n':
			inner := p11SpecOf(s.inner)
			for k := range spec {
				spec[k] += inner[k]
			}
		}
	}
	return spec
}

// p11Specificity returns the Selectors level 4 specificity of one selector.
func p11Specificity(t *testing.T, sel string) [3]int {
	t.Helper()
	var spec [3]int
	for _, comp := range p11Compounds(t, sel) {
		one := p11SpecOf(comp)
		for k := range spec {
			spec[k] += one[k]
		}
	}
	return spec
}

func p11CompoundMatches(simples []p11Simple, el p11Element) bool {
	for _, s := range simples {
		switch s.kind {
		case 'c':
			if !el.classes[s.name] {
				return false
			}
		case 'a':
			v, ok := el.attrs[s.name]
			if !ok || (s.hasVal && v != s.val) {
				return false
			}
		case 'p':
			if !el.states[s.name] {
				return false
			}
		case 'n':
			if p11CompoundMatches(s.inner, el) {
				return false
			}
		case 't':
			if s.name != "button" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// p11Matches reports whether sel (one selector, no commas) selects el.
func p11Matches(t *testing.T, sel string, el p11Element) bool {
	t.Helper()
	comps := p11Compounds(t, sel)
	if len(comps) == 0 {
		t.Fatalf("empty selector %q", sel)
	}
	return p11CompoundMatches(comps[len(comps)-1], el)
}

type p11Decl struct{ name, value string }

func p11Decls(body string) []p11Decl {
	var out []p11Decl
	for _, d := range strings.Split(body, ";") {
		name, value, ok := strings.Cut(d, ":")
		if !ok {
			continue
		}
		out = append(out, p11Decl{strings.TrimSpace(name), strings.TrimSpace(value)})
	}
	return out
}

var p11Groups = map[string][]string{
	"fill":   {"background", "background-color"},
	"text":   {"color"},
	"stroke": {"border-color", "border"},
	"cursor": {"cursor"},
}

var p11VoteClassRE = regexp.MustCompile(`\.vote-btn[A-Za-z0-9_-]*`)

const p11ForcedMedia = "@media (forced-colors: active)"

// p11Resolve returns the winning declared value of one property group for el,
// and the head part that supplied it. Candidates are the top level rules plus,
// when forced is true, the rules of the forced colours block. The winner has
// the highest specificity; ties go to the later rule.
func p11Resolve(t *testing.T, rules []p11Rule, el p11Element, forced bool, prop string) (string, string) {
	t.Helper()
	names := p11Groups[prop]
	if names == nil {
		t.Fatalf("unknown property group %q", prop)
	}
	bestValue, bestHead := "", ""
	var bestSpec [3]int
	bestOrder := -1
	for _, r := range rules {
		if r.media != "" && !(forced && strings.HasPrefix(r.media, p11ForcedMedia)) {
			continue
		}
		value := ""
		has := false
		for _, d := range p11Decls(r.body) {
			for _, n := range names {
				if d.name == n {
					value, has = d.value, true
				}
			}
		}
		if !has {
			continue
		}
		for _, part := range p11Split(r.head, true) {
			comps := p11Split(part, false)
			if len(comps) == 0 || !p11VoteClassRE.MatchString(comps[len(comps)-1]) {
				continue
			}
			if !p11Matches(t, part, el) {
				continue
			}
			spec := p11Specificity(t, part)
			win := bestOrder < 0 || spec[0] > bestSpec[0] ||
				(spec[0] == bestSpec[0] && (spec[1] > bestSpec[1] ||
					(spec[1] == bestSpec[1] && (spec[2] > bestSpec[2] ||
						(spec[2] == bestSpec[2] && r.order >= bestOrder)))))
			if win {
				bestValue, bestHead, bestSpec, bestOrder = value, part, spec, r.order
			}
		}
	}
	return bestValue, bestHead
}

// p11Token reduces a declared value to a comparable token.
func p11Token(value string) string {
	v := strings.TrimSpace(value)
	if strings.HasPrefix(v, "color-mix(") {
		return v
	}
	if idx := strings.Index(v, "var("); idx >= 0 {
		rest := v[idx+len("var("):]
		if end := strings.IndexAny(rest, ",)"); end >= 0 {
			return strings.TrimSpace(rest[:end])
		}
	}
	fields := strings.Fields(v)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// Element builders model the DOM plan 03-09 ships.
func p11Btn(modifier string, pressed string, states ...string) p11Element {
	el := p11Element{
		classes: map[string]bool{"vote-btn": true, modifier: true},
		attrs:   map[string]string{},
		states:  map[string]bool{},
	}
	if pressed != "" {
		el.attrs["aria-pressed"] = pressed
	}
	for _, s := range states {
		if s == "aria-disabled" {
			el.attrs["aria-disabled"] = "true"
		} else {
			el.states[s] = true
		}
	}
	return el
}

// p11Subsets returns every subset of names.
func p11Subsets(names []string) [][]string {
	var out [][]string
	for mask := 0; mask < 1<<len(names); mask++ {
		var subset []string
		for i, n := range names {
			if mask&(1<<i) != 0 {
				subset = append(subset, n)
			}
		}
		out = append(out, subset)
	}
	return out
}

func p11Has(states []string, want string) bool {
	for _, s := range states {
		if s == want {
			return true
		}
	}
	return false
}

// p11Expect asserts the resolved tokens for one element.
func p11Expect(t *testing.T, rules []p11Rule, label string, el p11Element, forced bool, want map[string]string) {
	t.Helper()
	for prop, wantTok := range want {
		got, head := p11Resolve(t, rules, el, forced, prop)
		if p11Token(got) != wantTok {
			t.Errorf("%s: %s resolves to %q (from %q), want %q", label, prop, p11Token(got), head, wantTok)
		}
	}
}

// p11Phase2Fixture is the shipped Phase 2 cascade order: base rule, pressed
// rule, then the disabled rule, all at (0,2,0).
const p11Phase2Fixture = `
.vote-btn:not([hidden]) {
  background: var(--color-bg);
  color: var(--color-text);
  border: 1px solid var(--color-border);
  cursor: pointer;
}
.vote-btn--confirm[aria-pressed="true"] {
  border-color: var(--color-severity-low);
  color: var(--color-severity-low);
  background: var(--color-severity-low-bg);
}
.vote-btn:disabled {
  color: var(--color-text-muted);
  border-color: var(--color-border);
  background: var(--color-surface);
  cursor: not-allowed;
}
`

// TestVoteStyleResolverCatchesPhase2Precedence is the negative control: on the
// Phase 2 order a pressed and disabled Confirm resolves to the disabled fill,
// which is the bug the shipped stylesheet must not have. It also pins the
// specificity and matching rules the resolver relies on.
func TestVoteStyleResolverCatchesPhase2Precedence(t *testing.T) {
	rules := p11ParseRules(t, p11Phase2Fixture)
	el := p11Btn("vote-btn--confirm", "true", "disabled")
	got, head := p11Resolve(t, rules, el, false, "fill")
	if p11Token(got) != "--color-surface" {
		t.Errorf("negative control: pressed and disabled Confirm resolved fill %q from %q, want the disabled fill --color-surface", got, head)
	}

	specs := map[string][3]int{
		`.vote-btn:not([hidden])`:                           {0, 2, 0},
		`.vote-btn.vote-btn--confirm-resolve:not([hidden])`: {0, 3, 0},
		`.vote-btn:not([hidden]):not([aria-pressed="true"]):not([aria-disabled="true"]):not(:disabled):not(.vote-btn--confirm-resolve):hover`: {0, 7, 0},
		`.leaflet-container .popup-card .vote-error:not([hidden])::before`:                                                                    {0, 4, 1},
	}
	for sel, want := range specs {
		if got := p11Specificity(t, sel); got != want {
			t.Errorf("specificity of %q is %v, want %v", sel, got, want)
		}
	}

	resolveEl := p11Btn("vote-btn--resolve", "")
	if !p11Matches(t, ".vote-btn", resolveEl) {
		t.Errorf(".vote-btn must match an element with classes vote-btn and vote-btn--resolve")
	}
	confirmEl := p11Btn("vote-btn--confirm", "false")
	if p11Matches(t, ".vote-btn--confirm-resolve", confirmEl) {
		t.Errorf(".vote-btn--confirm-resolve must not match the class vote-btn--confirm")
	}
	if p11Matches(t, `[aria-pressed="true"]`, confirmEl) {
		t.Errorf(`[aria-pressed="true"] must not match aria-pressed="false"`)
	}
}

const (
	p11HoverHead  = `.vote-btn:not([hidden]):not([aria-pressed="true"]):not([aria-disabled="true"]):not(:disabled):not(.vote-btn--confirm-resolve):hover`
	p11ActiveHead = `.vote-btn:not([hidden]):not([aria-pressed="true"]):not([aria-disabled="true"]):not(:disabled):not(.vote-btn--confirm-resolve):active`

	p11PressedBusyHead   = `.vote-btn[aria-pressed="true"]:disabled:not([hidden])`
	p11PressedTooFarHead = `.vote-btn[aria-pressed="true"][aria-disabled="true"]:not([hidden])`
	p11TooFarHead        = `.vote-btn[aria-disabled="true"]:not([aria-pressed="true"]):not([hidden])`
	p11YesHead           = `.vote-btn.vote-btn--confirm-resolve:not([hidden])`
	p11YesBusyHead       = `.vote-btn.vote-btn--confirm-resolve:disabled:not([hidden])`
	p11YesHoverHead      = `.vote-btn.vote-btn--confirm-resolve:not([hidden]):not(:disabled):hover`
	p11ResolveDisabledHd = `.vote-btn.vote-btn--resolve:disabled:not([hidden])`
	p11ResolveTooFarHead = `.vote-btn.vote-btn--resolve[aria-disabled="true"]:not([hidden])`
	p11YesHoverMix       = `color-mix(in srgb, var(--color-severity-critical) 85%, var(--color-text) 15%)`
	p11CancelLayoutHead  = `.resolve-confirm .vote-btn--cancel-resolve:not([hidden])`
	p11YesLayoutHead     = `.resolve-confirm .vote-btn--confirm-resolve:not([hidden])`
)

// TestPressedVoteStatesWinEveryCombination (A46, A13, A18, A72, A14) resolves
// the shipped cascade for every state combination and proves a pressed Confirm
// or Dispute keeps its inverted look, a busy or too far button looks disabled
// and Mark resolved is outlined. The 64 pressed cases are 2 buttons times the
// 32 subsets of hover, active, focus-visible, disabled and aria-disabled.
func TestPressedVoteStatesWinEveryCombination(t *testing.T) {
	rules := p11ParseRules(t, p11ReadCSS(t, "static/css/trust.css"))

	cases := 0
	for _, modifier := range []string{"vote-btn--confirm", "vote-btn--dispute"} {
		for _, states := range p11Subsets([]string{"hover", "active", "focus-visible", "disabled", "aria-disabled"}) {
			cases++
			el := p11Btn(modifier, "true", states...)
			label := modifier + " pressed + " + strings.Join(states, "+")
			want := map[string]string{"fill": "--color-text", "text": "--color-bg", "stroke": "--color-text"}
			if p11Has(states, "aria-disabled") {
				want["cursor"] = "not-allowed"
			} else if p11Has(states, "disabled") {
				want["cursor"] = "progress"
			}
			p11Expect(t, rules, label, el, false, want)
		}
	}
	if cases != 64 {
		t.Fatalf("expected 64 pressed cases, ran %d", cases)
	}

	type buttonCase struct{ name, modifier, pressed string }
	buttons := []buttonCase{
		{"Confirm", "vote-btn--confirm", "false"},
		{"Dispute", "vote-btn--dispute", "false"},
		{"Mark resolved", "vote-btn--resolve", ""},
		{"Reopen", "vote-btn--reopen", ""},
		{"Cancel", "vote-btn--cancel-resolve", ""},
	}
	for _, b := range buttons {
		name := b.name
		mk := func(states ...string) p11Element { return p11Btn(b.modifier, b.pressed, states...) }
		p11Expect(t, rules, name+" at rest", mk(), false, map[string]string{
			"fill": "--color-surface", "text": "--color-text", "stroke": "--color-border"})
		p11Expect(t, rules, name+" hover", mk("hover"), false, map[string]string{"fill": "--color-border"})
		p11Expect(t, rules, name+" hover+active", mk("hover", "active"), false, map[string]string{
			"fill": "--color-border", "stroke": "--color-text-muted"})
		for _, states := range [][]string{{"disabled"}, {"disabled", "hover"}, {"disabled", "active"}} {
			p11Expect(t, rules, name+" in flight "+strings.Join(states, "+"), mk(states...), false, map[string]string{
				"fill": "--color-bg", "text": "--color-text-muted", "stroke": "--color-border"})
		}
		if name == "Confirm" || name == "Dispute" || name == "Mark resolved" {
			for _, states := range [][]string{{"aria-disabled"}, {"aria-disabled", "hover"}, {"aria-disabled", "active"}} {
				p11Expect(t, rules, name+" too far "+strings.Join(states, "+"), mk(states...), false, map[string]string{
					"fill": "--color-bg", "text": "--color-text-muted", "stroke": "--color-border", "cursor": "not-allowed"})
			}
		}
	}

	// Structural: the four pinned heads stay present and unguarded.
	for _, head := range []string{
		`.vote-btn:not([hidden])`, `.vote-btn:disabled`,
		`.vote-btn--confirm[aria-pressed="true"]`, `.vote-btn--dispute[aria-pressed="true"]`,
	} {
		p11MustTop(t, rules, head)
	}
	disabled := p11MustTop(t, rules, `.vote-btn:disabled`)
	for _, head := range []string{p11PressedBusyHead, p11PressedTooFarHead} {
		r := p11MustTop(t, rules, head)
		if got := p11Specificity(t, head); got != [3]int{0, 4, 0} {
			t.Errorf("%s has specificity %v, want (0,4,0)", head, got)
		}
		if r.order <= disabled.order {
			t.Errorf("%s must come after .vote-btn:disabled in source order", head)
		}
	}
	busy := p11MustTop(t, rules, p11PressedBusyHead)
	tooFar := p11MustTop(t, rules, p11PressedTooFarHead)
	if tooFar.order <= busy.order {
		t.Errorf("the pressed too far rule must come after the pressed in flight rule so aria-disabled sets the cursor")
	}
	resolve := p11MustTop(t, rules, `.vote-btn--resolve:not([hidden])`)
	yes := p11MustTop(t, rules, p11YesHead)
	for _, head := range []string{p11ResolveDisabledHd, p11ResolveTooFarHead} {
		r := p11MustTop(t, rules, head)
		if got := p11Specificity(t, head); got != [3]int{0, 4, 0} {
			t.Errorf("%s has specificity %v, want (0,4,0)", head, got)
		}
		if r.order <= resolve.order || r.order <= yes.order {
			t.Errorf("%s must come after .vote-btn--resolve:not([hidden]) and the destructive Yes head (A72)", head)
		}
	}
}

// TestDestructiveYesHeadWins (A43, A40) proves the destructive Yes keeps its
// critical fill at rest, focused, pressed and busy, wins every generic rule,
// and that its word clears 4.5:1 on that fill in all four theme sources.
func TestDestructiveYesHeadWins(t *testing.T) {
	rules := p11ParseRules(t, p11ReadCSS(t, "static/css/trust.css"))

	base := p11MustTop(t, rules, `.vote-btn:not([hidden])`)
	yes := p11MustTop(t, rules, p11YesHead)
	if got := p11Specificity(t, p11YesHead); got != [3]int{0, 3, 0} {
		t.Errorf("the Yes head has specificity %v, want (0,3,0)", got)
	}
	if yes.order <= base.order {
		t.Errorf("the Yes head must come after the base vote button rule")
	}
	p11ExpectDecls(t, yes, map[string]string{
		"background":   "var(--color-severity-critical)",
		"color":        "var(--color-bg)",
		"border-color": "var(--color-severity-critical)",
	}, "A43 destructive fill")

	old := p11MustTop(t, rules, `.vote-btn--confirm-resolve`)
	if !strings.Contains(old.body, "--color-severity-critical") {
		t.Errorf(".vote-btn--confirm-resolve must still reference the critical token (a test reads it)")
	}
	hover := p11MustTop(t, rules, p11YesHoverHead)
	p11ExpectDecls(t, hover, map[string]string{
		"background":   p11YesHoverMix,
		"border-color": p11YesHoverMix,
	}, "A43 hover mix")

	yesEl := func(states ...string) p11Element { return p11Btn("vote-btn--confirm-resolve", "", states...) }
	critical := map[string]string{"fill": "--color-severity-critical", "text": "--color-bg"}
	for _, states := range [][]string{{}, {"focus-visible"}, {"active"}} {
		p11Expect(t, rules, "Yes "+strings.Join(states, "+"), yesEl(states...), false, critical)
	}
	p11Expect(t, rules, "Yes hover", yesEl("hover"), false, map[string]string{"fill": p11YesHoverMix, "text": "--color-bg"})
	for _, states := range [][]string{{"disabled"}, {"disabled", "hover"}} {
		p11Expect(t, rules, "Yes busy "+strings.Join(states, "+"), yesEl(states...), false, map[string]string{
			"fill": "--color-severity-critical", "text": "--color-bg", "cursor": "progress"})
	}

	mainRules := parseCSSRules(p11ReadCSS(t, "static/css/main.css"))
	light1, light2, dark1, dark2 := extractThemeTokens(mainRules, []string{"--color-bg", "--color-severity-critical"})
	sources := map[string]themeTokens{
		"light :root": light1, `light :root[data-theme="light"]`: light2,
		"dark @media": dark1, `dark :root[data-theme="dark"]`: dark2,
	}
	for name, tokens := range sources {
		bg, crit := tokens["--color-bg"], tokens["--color-severity-critical"]
		if !strings.HasPrefix(bg, "#") || !strings.HasPrefix(crit, "#") {
			t.Fatalf("%s: tokens did not resolve to hex (bg %q, critical %q)", name, bg, crit)
		}
		if ratio := wcagContrastRatio(t, bg, crit, "Yes word on critical fill / "+name); ratio < 4.5 {
			t.Errorf("%s: the Yes word on the critical fill is %.2f:1, below 4.5:1", name, ratio)
		}
	}
}

// TestResolveConfirmWrapsCancelBeforeYes (A16) pins the wrapping row: the
// question on its own line, then Cancel, then a Yes about twice as wide.
func TestResolveConfirmWrapsCancelBeforeYes(t *testing.T) {
	rules := p11ParseRules(t, p11ReadCSS(t, "static/css/trust.css"))

	p11ExpectDecls(t, p11MustTop(t, rules, `.resolve-confirm:not([hidden])`), map[string]string{
		"display":        "flex",
		"flex-direction": "row",
		"flex-wrap":      "wrap",
		"gap":            "var(--space-sm)",
		"grid-column":    "1 / -1",
		"margin-top":     "0",
		"padding":        "var(--space-sm)",
		"border-radius":  "8px",
		"background":     "var(--color-bg)",
	}, "A16 wrapping row")
	p11ExpectDecls(t, p11MustTop(t, rules, `.resolve-confirm p`), map[string]string{
		"flex":  "1 1 100%",
		"color": "var(--color-text)",
	}, "A16 question on its own line")
	p11ExpectDecls(t, p11MustTop(t, rules, p11CancelLayoutHead), map[string]string{
		"flex": "1 1 0", "min-width": "96px",
	}, "A16 Cancel")
	p11ExpectDecls(t, p11MustTop(t, rules, p11YesLayoutHead), map[string]string{
		"flex": "2 1 0", "min-width": "144px",
	}, "A16 Yes about twice as wide")
}
