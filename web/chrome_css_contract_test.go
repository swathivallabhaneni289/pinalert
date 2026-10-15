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
	marker := "@media " + query
	var out []cssRule
	from := 0
	for {
		idx := strings.Index(text[from:], marker)
		if idx == -1 {
			return out
		}
		start := from + idx + len(marker)
		open := strings.Index(text[start:], "{")
		if open == -1 {
			return out
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
			return out
		}
		out = append(out, parseCSSRules(text[bodyStart:i-1]+"}")...)
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
