package web

import (
	"io/fs"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// This file locks the plan 04-08 stylesheet contract: the disclaimer bar
// height token, the pages that subtract it, the account trigger anchored
// under the bar, and the shared looks the later Phase 4 surfaces reuse. Every
// test reads the shipped stylesheets and templates out of the embedded
// filesystems, never a copy of their text.

// p408ReadStatic returns one embedded static file as a string.
func p408ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", path, err)
	}
	return string(raw)
}

// p408AllCSS returns every embedded stylesheet keyed by its path.
func p408AllCSS(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := fs.WalkDir(StaticFS, "static/css", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".css") {
			return nil
		}
		files[path] = p408ReadStatic(t, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded static/css: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no stylesheet found under static/css")
	}
	return files
}

// p408MediaBlocks returns the rules inside every `@media <query>` block of
// the given CSS, in source order. The CSS is comment stripped first and each
// block body is found by balanced braces, so a block holding several rules
// is returned as separate rules whose heads carry no at-rule preamble.
func p408MediaBlocks(css, query string) []cssRule {
	text := stripCSSComments(css)
	var out []cssRule
	for _, span := range p408MediaSpans(text, query) {
		out = append(out, parseCSSRules(text[span[1]:span[2]-1]+"}")...)
	}
	return out
}

// p408MediaSpans finds every `@media <query>` block in comment stripped CSS
// by balanced braces. Each span is {start of the at-rule, start of its body,
// one past its closing brace}.
func p408MediaSpans(text, query string) [][3]int {
	marker := "@media " + query
	var spans [][3]int
	from := 0
	for {
		idx := strings.Index(text[from:], marker)
		if idx == -1 {
			return spans
		}
		atStart := from + idx
		start := atStart + len(marker)
		open := strings.Index(text[start:], "{")
		if open == -1 {
			return spans
		}
		if strings.TrimSpace(text[start:start+open]) != "" {
			// A longer query that merely starts with this one.
			from = start
			continue
		}
		bodyStart := start + open + 1
		depth := 1
		i := bodyStart
		for i < len(text) && depth > 0 {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			i++
		}
		if depth != 0 {
			return spans
		}
		spans = append(spans, [3]int{atStart, bodyStart, i})
		from = i
	}
}

// p408Ordered fails unless every part occurs in body, each after the one
// before it.
func p408Ordered(t *testing.T, body string, parts ...string) {
	t.Helper()
	at := 0
	for _, part := range parts {
		idx := strings.Index(body[at:], part)
		if idx == -1 {
			t.Errorf("expected %q after offset %d in %q", part, at, body)
			return
		}
		at += idx + len(part)
	}
}

// p408Squash removes every whitespace character, for whitespace insensitive
// comparison of a declaration value.
func p408Squash(s string) string {
	return strings.Join(strings.Fields(s), "")
}

var p408BarCalc = regexp.MustCompile(
	`^calc\(var\(--space-xs\)\*2\+var\(--font-size-label\)\*var\(--line-height-label\)(?:\*(\d))?\+1px\)$`)

// p408ResolveBar resolves one bar height value against the label tokens.
func p408ResolveBar(t *testing.T, value string, space, size, line float64) float64 {
	t.Helper()
	m := p408BarCalc.FindStringSubmatch(p408Squash(value))
	if m == nil {
		t.Fatalf("bar height value %q does not match the three term calc form", value)
	}
	lines := 1.0
	if m[1] != "" {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("bad line multiplier in %q: %v", value, err)
		}
		lines = float64(n)
	}
	return space*2 + size*line*lines + 1
}

func p408PxToken(t *testing.T, decls map[string]string, name string) float64 {
	t.Helper()
	v, ok := decls[name]
	if !ok {
		t.Fatalf("main.css :root does not declare %s", name)
	}
	f, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
	if err != nil {
		t.Fatalf("%s value %q is not a plain number: %v", name, v, err)
	}
	return f
}

func TestDisclaimerBarToken(t *testing.T) {
	const token = "--disclaimer-bar-height"
	main := p408ReadStatic(t, "static/css/main.css")
	rules := parseCSSRules(main)

	root, ok := ruleBySelector(rules, ":root")
	if !ok {
		t.Fatal("main.css has no exact :root rule")
	}
	rootDecls := declsOf(root.declBody)
	base, ok := rootDecls[token]
	if !ok {
		t.Fatalf("main.css :root does not declare %s", token)
	}
	wantBase := "calc(var(--space-xs)*2+var(--font-size-label)*var(--line-height-label)*3+1px)"
	if p408Squash(base) != wantBase {
		t.Errorf(":root %s = %q, want the three line form", token, base)
	}

	space := p408PxToken(t, rootDecls, "--space-xs")
	size := p408PxToken(t, rootDecls, "--font-size-label")
	line, err := strconv.ParseFloat(rootDecls["--line-height-label"], 64)
	if err != nil {
		t.Fatalf("--line-height-label %q: %v", rootDecls["--line-height-label"], err)
	}

	tiers := []struct {
		query string
		want  string
		px    float64
	}{
		{"(min-width: 480px)", "calc(var(--space-xs)*2+var(--font-size-label)*var(--line-height-label)*2+1px)", 48.2},
		{"(min-width: 900px)", "calc(var(--space-xs)*2+var(--font-size-label)*var(--line-height-label)+1px)", 28.6},
	}
	if got := p408ResolveBar(t, base, space, size, line); math.Abs(got-67.8) > 0.01 {
		t.Errorf("base bar height resolves to %.2fpx, want 67.8", got)
	}
	for _, tier := range tiers {
		blocks := p408MediaBlocks(main, tier.query)
		var roots []cssRule
		for _, r := range blocks {
			if r.selectorHead == ":root" {
				roots = append(roots, r)
			}
		}
		if len(roots) != 1 {
			t.Errorf("%s: want exactly one :root rule, got %d (of %d rules)", tier.query, len(roots), len(blocks))
			continue
		}
		decls := declsOf(roots[0].declBody)
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10):
		// the 900px tier also carries the two app bar lengths (the laptop top
		// bar is 61px, the laptop has no bottom bar), so they are set aside
		// before the "only the bar token" count. Both are lengths, never a
		// colour. The Phase 3.1 and Phase 4 hands-on tests are walked on this
		// layout after Phase 3.4 is built.
		delete(decls, "--app-topbar-height")
		delete(decls, "--app-bottombar-height")
		if len(decls) != 1 {
			t.Errorf("%s :root must declare only %s, got %v", tier.query, token, decls)
		}
		if p408Squash(decls[token]) != tier.want {
			t.Errorf("%s :root %s = %q, want %q", tier.query, token, decls[token], tier.want)
		}
		if got := p408ResolveBar(t, decls[token], space, size, line); math.Abs(got-tier.px) > 0.01 {
			t.Errorf("%s bar height resolves to %.2fpx, want %.1f", tier.query, got, tier.px)
		}
	}

	// Exactly three declarations in main.css, none of them in a theme source.
	declaring := 0
	for _, r := range rules {
		if _, has := declsOf(r.declBody)[token]; !has {
			continue
		}
		declaring++
		head := r.selectorHead
		if strings.Contains(head, "data-theme") || strings.Contains(head, "prefers-color-scheme") {
			t.Errorf("%s is declared in the theme source %q", token, head)
		}
	}
	if declaring != 3 {
		t.Errorf("main.css declares %s in %d rules, want 3 (the :root rule and two width tiers)", token, declaring)
	}

	// No other stylesheet declares it; reading it is fine.
	decl := regexp.MustCompile(regexp.QuoteMeta(token) + `\s*:`)
	for path, css := range p408AllCSS(t) {
		if path == "static/css/main.css" {
			continue
		}
		if decl.MatchString(stripCSSComments(css)) {
			t.Errorf("%s declares %s; only main.css may", path, token)
		}
	}
}

func TestBarTokenReadByPageWrappers(t *testing.T) {
	const want1 = "calc(100vh - var(--disclaimer-bar-height))"
	const want2 = "calc(100dvh - var(--disclaimer-bar-height))"
	main := p408ReadStatic(t, "static/css/main.css")
	auth := p408ReadStatic(t, "static/css/auth.css")

	cases := []struct {
		file string
		css  string
		head string
		prop string
	}{
		{"main.css", main, ".app-shell", "min-height"},
		{"main.css", main, "#map", "height"},
		{"auth.css", auth, ".login-gate", "min-height"},
	}
	for _, c := range cases {
		rule, ok := ruleBySelector(parseCSSRules(c.css), c.head)
		if !ok {
			t.Errorf("%s has no exact %s rule", c.file, c.head)
			continue
		}
		var lines []string
		for _, decl := range strings.Split(rule.declBody, ";") {
			name, value, has := strings.Cut(decl, ":")
			if has && strings.TrimSpace(name) == c.prop {
				lines = append(lines, c.prop+": "+strings.TrimSpace(value))
			}
		}
		p408Ordered(t, strings.Join(lines, "; "), want1, want2)
	}

	// No wrapper may state a bare viewport unit as its whole height.
	for file, css := range map[string]string{"main.css": main, "auth.css": auth} {
		for _, r := range parseCSSRules(css) {
			decls := declsOf(r.declBody)
			for _, prop := range []string{"height", "min-height"} {
				if v := decls[prop]; v == "100vh" || v == "100dvh" {
					t.Errorf("%s: %q declares %s: %s, which ignores the bar (P4-A3)", file, r.selectorHead, prop, v)
				}
			}
		}
	}
}

// The trust.css half of the bar token contract (P4-A4): every popup max
// height that is stated against 100dvh subtracts the bar exactly once, so the
// card fits the map that now sits under the bar. There are exactly four such
// values: the base rule, the narrow block and the short block of plan 03-17,
// and the laptop block that Phase 3.1 plan 11 added after the plan text was
// written (it still says three).
func TestPopupMaxHeightsReadTheBarToken(t *testing.T) {
	trust := p408ReadStatic(t, "static/css/trust.css")
	found := 0
	for _, r := range parseCSSRules(trust) {
		v, has := declsOf(r.declBody)["max-height"]
		if !has || !strings.Contains(v, "100dvh") {
			continue
		}
		found++
		if got := strings.Count(v, "var(--disclaimer-bar-height)"); got != 1 {
			t.Errorf("%q: max-height %q reads the bar token %d times, want once", r.selectorHead, v, got)
		}
	}
	if found != 4 {
		t.Errorf("trust.css has %d max-height values stated against 100dvh, want 4 (base, narrow, short, laptop)", found)
	}
}

func TestPopupOpenFlagOnRoot(t *testing.T) {
	code := stripJSComments(p408ReadStatic(t, "static/js/map.js"))
	open := jsFunctionBody(t, code, "onPopupOpen(evt)")
	if !strings.Contains(open, "document.documentElement.setAttribute('data-popup-open', 'true')") {
		t.Error("onPopupOpen must set data-popup-open on the root element")
	}
	closeBody := jsFunctionBody(t, code, "onPopupClose(evt)")
	if !strings.Contains(closeBody, "document.documentElement.removeAttribute('data-popup-open')") {
		t.Error("onPopupClose must remove data-popup-open from the root element")
	}
}

func TestTopChromeGeometry(t *testing.T) {
	chrome := p408ReadStatic(t, "static/css/chrome.css")
	auth := p408ReadStatic(t, "static/css/auth.css")
	chromeRules := parseCSSRules(chrome)
	authRules := parseCSSRules(auth)

	mustDecl := func(rules []cssRule, head, prop, want string) {
		t.Helper()
		r, ok := ruleBySelector(rules, head)
		if !ok {
			t.Errorf("no exact %s rule", head)
			return
		}
		if got := declsOf(r.declBody)[prop]; got != want {
			t.Errorf("%s %s = %q, want %q", head, prop, got, want)
		}
	}

	mustDecl(chromeRules, ".top-chrome", "position", "sticky")
	mustDecl(chromeRules, ".top-chrome", "top", "0")
	mustDecl(chromeRules, ".top-chrome", "z-index", "1050")
	mustDecl(chromeRules, ".disclaimer-bar", "min-height", "var(--disclaimer-bar-height)")
	if r, ok := ruleBySelector(chromeRules, ".disclaimer-bar"); ok {
		if _, has := declsOf(r.declBody)["height"]; has {
			t.Error(".disclaimer-bar must not declare height; the token is its min-height")
		}
	}
	mustDecl(chromeRules, ".disclaimer-bar p", "margin", "0")
	mustDecl(chromeRules, "html", "scroll-padding-top", "calc(var(--disclaimer-bar-height) + var(--space-sm))")

	mustDecl(authRules, ".app-header", "position", "absolute")
	mustDecl(authRules, ".app-header", "top", "calc(100% + var(--space-md))")
	mustDecl(authRules, ".app-header", "right", "var(--space-md)")
	for _, r := range authRules {
		if strings.Contains(r.selectorHead, ".app-header") && declsOf(r.declBody)["position"] == "fixed" {
			t.Errorf("rule %q declares position: fixed; the header hangs off .top-chrome", r.selectorHead)
		}
	}

	// chrome.css is linked on every full page and, after main.css.
	for path, text := range findFullPageTemplates(t) {
		chromeAt := strings.Index(text, "/static/css/chrome.css")
		if chromeAt == -1 {
			t.Errorf("%s does not link chrome.css", path)
			continue
		}
		if mainAt := strings.Index(text, "/static/css/main.css"); mainAt != -1 && chromeAt < mainAt {
			t.Errorf("%s links chrome.css before main.css; the button state cascade needs it after", path)
		}
	}

	// P4-A20: no bottom anchored rule reads the bar token.
	for path, css := range p408AllCSS(t) {
		for _, r := range parseCSSRules(css) {
			if !strings.Contains(r.declBody, "--disclaimer-bar-height") {
				continue
			}
			for _, name := range []string{".fab", ".theme-toggle", ".view-toggle", "#toast"} {
				if strings.Contains(r.selectorHead, name) {
					t.Errorf("%s: rule %q reads the bar token but is a bottom anchored control", path, r.selectorHead)
				}
			}
		}
	}
}

// p408TopLevelRules parses the CSS with every @media block removed, so the
// rules returned are the ones that apply at any width.
func p408TopLevelRules(css string) []cssRule {
	text := stripCSSComments(css)
	for {
		at := strings.Index(text, "@media")
		if at == -1 {
			break
		}
		open := strings.Index(text[at:], "{")
		if open == -1 {
			break
		}
		depth, i := 1, at+open+1
		for i < len(text) && depth > 0 {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			i++
		}
		text = text[:at] + text[i:]
	}
	return parseCSSRules(text)
}

// p408Selectors splits a rule head into its trimmed selectors, dropping any
// at-rule preamble that parseCSSRules leaves on the first rule of a block.
func p408Selectors(head string) []string {
	if i := strings.LastIndex(head, "{"); i != -1 {
		head = head[i+1:]
	}
	var out []string
	for _, sel := range strings.Split(head, ",") {
		if sel = strings.TrimSpace(sel); sel != "" {
			out = append(out, sel)
		}
	}
	return out
}

// p408Merged merges the declarations of every rule whose selector list holds
// the selector, per selector, in source order.
func p408Merged(rules []cssRule) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, r := range rules {
		decls := declsOf(r.declBody)
		for _, sel := range p408Selectors(r.selectorHead) {
			if out[sel] == nil {
				out[sel] = map[string]string{}
			}
			for k, v := range decls {
				out[sel][k] = v
			}
		}
	}
	return out
}

// p408Expect fails for every property whose squashed value differs.
func p408Expect(t *testing.T, label string, got map[string]string, want map[string]string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: no rule found", label)
		return
	}
	for prop, value := range want {
		if p408Squash(got[prop]) != p408Squash(value) {
			t.Errorf("%s: %s = %q, want %q", label, prop, got[prop], value)
		}
	}
}

func TestInlineMessageLooks(t *testing.T) {
	chrome := p408ReadStatic(t, "static/css/chrome.css")
	rules := p408Merged(p408TopLevelRules(chrome))

	p408Expect(t, ".inline-error", rules[".inline-error:not([hidden])"], map[string]string{
		"display":      "flex",
		"gap":          "var(--space-sm)",
		"margin":       "var(--space-xs) 0 0",
		"padding-left": "var(--space-sm)",
		"border-left":  "4px solid var(--color-severity-critical)",
		"color":        "var(--color-text)",
		"font-size":    "var(--font-size-body)",
		"line-height":  "var(--line-height-body)",
		"font-weight":  "var(--font-weight-regular)",
	})
	icon := rules[".inline-error::before"]
	p408Expect(t, ".inline-error::before", icon, map[string]string{
		"content":          `""`,
		"flex-shrink":      "0",
		"width":            "var(--space-md)",
		"height":           "var(--space-md)",
		"margin-top":       "var(--space-xs)",
		"background-color": "currentColor",
		"mask-size":        "contain",
		"mask-repeat":      "no-repeat",
		"mask-position":    "center",
	})
	if _, has := icon["display"]; has {
		t.Error(".inline-error::before must not set display")
	}
	maskPath := extractMaskURLPath(icon["mask-image"])
	if maskPath != "/static/icons/circle-alert.svg" {
		t.Errorf(".inline-error::before mask-image path = %q, want /static/icons/circle-alert.svg", maskPath)
	} else if _, err := fs.Stat(StaticFS, strings.TrimPrefix(maskPath, "/")); err != nil {
		t.Errorf("mask image %s is not embedded: %v", maskPath, err)
	}
	if icon["-webkit-mask-image"] != icon["mask-image"] {
		t.Error(".inline-error::before needs the -webkit- mask-image beside the plain one")
	}

	p408Expect(t, ".inline-note", rules[".inline-note:not([hidden])"], map[string]string{
		"display":      "block",
		"margin":       "var(--space-xs) 0 0",
		"padding-left": "var(--space-sm)",
		"border-left":  "4px solid var(--color-text)",
		"color":        "var(--color-text)",
		"font-size":    "var(--font-size-label)",
		"line-height":  "var(--line-height-label)",
		"font-weight":  "var(--font-weight-regular)",
	})
	for sel := range rules {
		if strings.HasPrefix(sel, ".inline-note") && strings.Contains(sel, "::") {
			t.Errorf("%s: the note has no icon", sel)
		}
		if strings.Contains(sel, ".vote-btn") || strings.Contains(sel, ".vote-controls") || strings.Contains(sel, ".vote-error") {
			t.Errorf("chrome.css must not restyle %s", sel)
		}
	}

	// chrome.css holds two forced colors blocks (the bar's and this plan's),
	// so read the union of them.
	forced := p408Merged(p408MediaBlocks(chrome, "(forced-colors: active)"))
	p408Expect(t, "forced .inline-error", forced[".inline-error:not([hidden])"], map[string]string{"border-left": "4px solid CanvasText"})
	p408Expect(t, "forced .inline-note", forced[".inline-note:not([hidden])"], map[string]string{"border-left": "4px solid CanvasText"})
	p408Expect(t, "forced .inline-error::before", forced[".inline-error::before"],
		map[string]string{"forced-color-adjust": "none", "background-color": "CanvasText"})
}

func TestPlainButtonStates(t *testing.T) {
	const mix = "color-mix(in srgb, var(--color-severity-critical) 85%, var(--color-text) 15%)"
	chrome := p408ReadStatic(t, "static/css/chrome.css")
	topRules := p408TopLevelRules(chrome)
	rules := p408Merged(topRules)
	const plain = ".btn:not(.btn--primary):not(.btn--destructive)"
	disabledPlain := map[string]string{
		"background": "var(--color-bg)", "color": "var(--color-text-muted)",
		"border-color": "var(--color-border)", "box-shadow": "none", "cursor": "not-allowed",
	}

	p408Expect(t, "plain hover", rules[plain+":hover"], map[string]string{
		"background": "var(--color-border)", "color": "var(--color-text)", "border-color": "var(--color-border)"})
	p408Expect(t, "plain press", rules[plain+":active"], map[string]string{
		"background": "var(--color-border)", "color": "var(--color-text)", "border-color": "var(--color-border)",
		"box-shadow": "inset 0 0 0 1px var(--color-text-muted)"})
	p408Expect(t, "plain disabled", rules[plain+":disabled"], disabledPlain)
	p408Expect(t, "plain aria-disabled", rules[plain+`[aria-disabled="true"]`], disabledPlain)

	p408Expect(t, "primary hover", rules[".btn--primary:hover"], map[string]string{
		"background": "var(--color-text-muted)", "color": "var(--color-bg)", "border-color": "var(--color-text-muted)"})
	p408Expect(t, "primary press", rules[".btn--primary:active"], map[string]string{
		"background": "var(--color-text-muted)", "color": "var(--color-bg)", "border-color": "var(--color-text-muted)",
		"box-shadow": "inset 0 0 0 1px var(--color-bg)"})
	p408Expect(t, "primary disabled", rules[".btn--primary:disabled"], disabledPlain)
	p408Expect(t, "primary aria-disabled", rules[`.btn--primary[aria-disabled="true"]`], disabledPlain)

	p408Expect(t, "destructive hover", rules[".btn--destructive:hover"], map[string]string{
		"background": mix, "color": "var(--color-bg)", "border-color": mix})
	p408Expect(t, "destructive press", rules[".btn--destructive:active"], map[string]string{
		"background": mix, "color": "var(--color-bg)", "border-color": mix,
		"box-shadow": "inset 0 0 0 1px var(--color-bg)"})
	inFlight := map[string]string{
		"background": "var(--color-severity-critical)", "color": "var(--color-bg)",
		"border-color": "var(--color-severity-critical)", "box-shadow": "none", "cursor": "progress"}
	p408Expect(t, "destructive disabled", rules[".btn--destructive:disabled"], inFlight)
	p408Expect(t, "destructive aria-disabled", rules[`.btn--destructive[aria-disabled="true"]`], inFlight)

	p408Expect(t, ".btn[hidden]", rules[".btn[hidden]"], map[string]string{"display": "none"})
	p408Expect(t, ".btn transition", rules[".btn"],
		map[string]string{"transition": "background-color 150ms ease, border-color 150ms ease, color 150ms ease"})

	// Each disabled rule comes after its hover and press rules.
	index := func(sel string) int {
		for i, r := range topRules {
			for _, s := range p408Selectors(r.selectorHead) {
				if s == sel {
					return i
				}
			}
		}
		return -1
	}
	for _, base := range []string{plain, ".btn--primary", ".btn--destructive"} {
		hover, press, disabled := index(base+":hover"), index(base+":active"), index(base+":disabled")
		if hover == -1 || press == -1 || disabled == -1 || !(hover < press && press < disabled) {
			t.Errorf("%s: want hover, press, disabled in that order, got %d, %d, %d", base, hover, press, disabled)
		}
	}

	// The reduced motion block turns the transition off.
	motion := p408Merged(p408MediaBlocks(chrome, "(prefers-reduced-motion: reduce)"))
	p408Expect(t, "reduced motion .btn", motion[".btn"], map[string]string{"transition": "none"})

	// Forced colors: the union of both blocks holds the button treatments,
	// and the block that holds them comes after the last .btn state rule.
	forced := p408Merged(p408MediaBlocks(chrome, "(forced-colors: active)"))
	for _, sel := range []string{
		plain + ":disabled", plain + `[aria-disabled="true"]`, ".btn--primary:disabled", `.btn--primary[aria-disabled="true"]`,
	} {
		p408Expect(t, "forced "+sel, forced[sel], map[string]string{"border": "1px solid GrayText"})
	}
	for _, sel := range []string{
		".btn--destructive", ".btn--destructive:hover", ".btn--destructive:active",
		".btn--destructive:disabled", `.btn--destructive[aria-disabled="true"]`,
	} {
		p408Expect(t, "forced "+sel, forced[sel],
			map[string]string{"background": "ButtonFace", "color": "ButtonText", "border": "1px solid ButtonText"})
	}
	text := stripCSSComments(chrome)
	lastState := strings.LastIndex(text, "cursor: progress;")
	forcedAt := strings.Index(text, "ButtonFace")
	if lastState == -1 || forcedAt == -1 || forcedAt < lastState {
		t.Errorf("the forced colors block with the button treatments must come after the last .btn state rule (found %d and %d)", lastState, forcedAt)
	}
	for _, r := range topRules {
		head := r.selectorHead
		if strings.Contains(head, ".btn") && (strings.Contains(head, ".vote-btn") || strings.Contains(head, ".vote-controls")) {
			t.Errorf("rule %q mixes .btn with a vote control", head)
		}
	}
}

func TestPhase4HiddenGuard(t *testing.T) {
	prefixes := []string{
		".inline-error", ".inline-note", ".notice-card", ".queue-strip", ".flag-control", ".flag-dialog",
		".low-", ".report-row--low", ".report-form--low", ".mod-", ".legal-",
	}
	inspected := 0
	for path, css := range p408AllCSS(t) {
		for _, r := range parseCSSRules(css) {
			display, has := declsOf(r.declBody)["display"]
			if !has || display == "none" {
				continue
			}
			for _, sel := range p408Selectors(r.selectorHead) {
				if strings.HasSuffix(sel, "::before") || strings.HasSuffix(sel, "::after") {
					continue
				}
				matches := false
				for _, prefix := range prefixes {
					if strings.Contains(sel, prefix) {
						matches = true
					}
				}
				if !matches {
					continue
				}
				inspected++
				if !strings.Contains(sel, ":not([hidden])") {
					t.Errorf("%s: %q sets display: %s without :not([hidden]), so it would override the hidden attribute", path, sel, display)
				}
			}
		}
	}
	if inspected < 2 {
		t.Errorf("inspected %d selectors, want at least 2 (the two inline message rules of plan 04-08)", inspected)
	}

	chrome := p408Merged(parseCSSRules(p408ReadStatic(t, "static/css/chrome.css")))
	p408Expect(t, ".btn[hidden]", chrome[".btn[hidden]"], map[string]string{"display": "none"})
}

func TestPhase4ContrastPairs(t *testing.T) {
	needed := []string{
		"--color-bg", "--color-surface", "--color-border", "--color-text", "--color-text-muted",
		"--color-severity-critical", "--color-severity-low-bg", "--color-severity-medium-bg", "--color-severity-critical-bg",
	}
	light1, light2, dark1, dark2 := extractThemeTokens(parseCSSRules(p408ReadStatic(t, "static/css/main.css")), needed)
	sources := []struct {
		name   string
		tokens themeTokens
	}{
		{"root light", light1}, {`data-theme="light"`, light2}, {"prefers dark", dark1}, {`data-theme="dark"`, dark2},
	}
	// Row backgrounds: main.css maps each severity to its tint token, and
	// trust.css only paints --color-surface or transparent (the page
	// background), so these are the backgrounds a row text can sit on.
	pairs := [][2]string{
		{"--color-text", "--color-surface"},
		{"--color-text", "--color-bg"},
		{"--color-text", "--color-severity-low-bg"},
		{"--color-text", "--color-severity-medium-bg"},
		{"--color-text", "--color-severity-critical-bg"},
		{"--color-text", "--color-border"},
		{"--color-text-muted", "--color-bg"},
		{"--color-bg", "--color-text-muted"},
		{"--color-bg", "--color-severity-critical"},
	}
	for _, src := range sources {
		for _, p := range pairs {
			fg, okFg := src.tokens[p[0]]
			bg, okBg := src.tokens[p[1]]
			if !okFg || !okBg {
				t.Errorf("%s: %s or %s does not resolve", src.name, p[0], p[1])
				continue
			}
			ctx := src.name + " " + p[0] + " on " + p[1]
			if ratio := wcagContrastRatio(t, fg, bg, ctx); ratio < 4.5 {
				t.Errorf("%s: contrast %.2f is below 4.5", ctx, ratio)
			}
		}
		fill := mixHexSRGB(t, src.tokens["--color-severity-critical"], 85, src.tokens["--color-text"], 15, src.name)
		ctx := src.name + " --color-bg on the destructive hover fill"
		if ratio := wcagContrastRatio(t, src.tokens["--color-bg"], fill, ctx); ratio < 4.5 {
			t.Errorf("%s: contrast %.2f is below 4.5", ctx, ratio)
		}
	}
}

func TestChromeCSSTokensOnly(t *testing.T) {
	raw := p408ReadStatic(t, "static/css/chrome.css")
	for _, dash := range []string{"\u2014", "\u2013"} {
		if strings.Contains(raw, dash) {
			t.Errorf("chrome.css contains a dash character %q", dash)
		}
	}
	text := stripCSSComments(raw)
	for _, banned := range []string{"!important", "rgb(", "rgba(", "hsl(", "999px"} {
		if strings.Contains(text, banned) {
			t.Errorf("chrome.css contains %q", banned)
		}
	}
	if m := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindString(text); m != "" {
		t.Errorf("chrome.css contains the hex colour %q", m)
	}

	sizes := map[string]bool{
		"var(--font-size-label)": true, "var(--font-size-body)": true,
		"var(--font-size-heading)": true, "var(--font-size-display)": true,
		// Phase 3.4 (plan 03.4-03, UI-SPEC departure D4): the 12px caption token,
		// limited to the closed list in UI-SPEC Typography (bottom bar labels,
		// column heads, bell count digits, step digits, bell list time line).
		"var(--font-size-caption)": true,
	}
	weights := map[string]bool{"var(--font-weight-regular)": true, "var(--font-weight-semibold)": true}
	for _, r := range parseCSSRules(raw) {
		decls := declsOf(r.declBody)
		if v, has := decls["font-size"]; has && !sizes[v] {
			t.Errorf("%q: font-size %q is not a size token", r.selectorHead, v)
		}
		if v, has := decls["font-weight"]; has && !weights[v] {
			t.Errorf("%q: font-weight %q is not a weight token", r.selectorHead, v)
		}
	}

	// System colour keywords belong inside forced colors blocks only.
	outside := text
	spans := p408MediaSpans(text, "(forced-colors: active)")
	for i := len(spans) - 1; i >= 0; i-- {
		outside = outside[:spans[i][0]] + outside[spans[i][2]:]
	}
	if len(spans) < 2 {
		t.Errorf("chrome.css holds %d forced colors blocks, want the bar's and this plan's", len(spans))
	}
	if m := regexp.MustCompile(`\b(CanvasText|LinkText|GrayText|ButtonFace|ButtonText|Canvas)\b`).FindString(outside); m != "" {
		t.Errorf("system colour %s appears outside a forced colors block", m)
	}
}
