// Plan 04-21 pins the markup and styles of the ways in to the Low data view:
// the login page panel (D-17, D-23, P4-A18), the account menu items and its
// measured height (P4-A11). The tests read the shipped templates and
// stylesheets out of the embedded filesystems, never a copy of their text.

package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// p421RuleFor returns the first rule whose head, split on commas and trimmed,
// contains selector exactly.
func p421RuleFor(rules []cssRule, selector string) (cssRule, bool) {
	for _, r := range rules {
		for _, part := range strings.Split(r.selectorHead, ",") {
			if strings.TrimSpace(part) == selector {
				return r, true
			}
		}
	}
	return cssRule{}, false
}

// p421ReadTemplate reads one embedded template.
func p421ReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("reading embedded templates/%s: %v", name, err)
	}
	return string(raw)
}

// p421ReadStatic reads one embedded static file.
func p421ReadStatic(t *testing.T, name string) string {
	t.Helper()
	raw, err := StaticFS.ReadFile("static/" + name)
	if err != nil {
		t.Fatalf("reading embedded static/%s: %v", name, err)
	}
	return string(raw)
}

// p421InOrder checks that each part occurs after the previous match.
func p421InOrder(t *testing.T, label, text string, parts ...string) {
	t.Helper()
	pos := 0
	for _, p := range parts {
		i := strings.Index(text[pos:], p)
		if i == -1 {
			t.Errorf("%s: %q not found after offset %d", label, p, pos)
			return
		}
		pos += i + len(p)
	}
}

// p421WantDecls fails for every declaration the rule for selector lacks.
func p421WantDecls(t *testing.T, rules []cssRule, selector string, want map[string]string) {
	t.Helper()
	rule, ok := p421RuleFor(rules, selector)
	if !ok {
		t.Errorf("auth.css has no rule for %s", selector)
		return
	}
	got := declsOf(rule.declBody)
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s: %s = %q, want %q", selector, name, got[name], value)
		}
	}
}

func TestLoginLowDataPanelStyles(t *testing.T) {
	login := p421ReadTemplate(t, "login_gate.html.tmpl")

	if n := strings.Count(login, `class="login-column"`); n != 1 {
		t.Fatalf(`class="login-column" appears %d times, want 1`, n)
	}
	p421InOrder(t, "login column order", login,
		`class="login-column"`, `id="login-form-panel"`, `{{template "check_inbox.html.tmpl" .}}`, `<div class="login-ways">`)
	for _, id := range []string{`id="login-gate"`, `id="login-form-panel"`, `id="login-email"`, `id="request-link"`, `id="auth-error"`} {
		if n := strings.Count(login, id); n != 1 {
			t.Errorf("%s appears %d times in login_gate.html.tmpl, want 1", id, n)
		}
	}

	start := strings.Index(login, `<div class="login-ways">`)
	if start == -1 {
		t.Fatalf("login_gate.html.tmpl has no login-ways panel")
	}
	end := strings.Index(login[start:], "</div>")
	if end == -1 {
		t.Fatalf("login-ways panel is not closed")
	}
	panel := login[start : start+end]
	for _, want := range []string{
		"{{if .LowDataDefault}}", "{{else}}", "{{end}}",
		`href="/login?low=0"`, `href="/login?low=1"`, `href="/disclaimer"`, `href="/policy"`,
		"Low data view is on for this device. It opens after you sign in.",
		"Text only, no map",
	} {
		if !strings.Contains(panel, want) {
			t.Errorf("login-ways panel lacks %q", want)
		}
	}
	if strings.Contains(panel, "{{.Email}}") {
		t.Errorf("login-ways panel renders the email")
	}
	if strings.Contains(panel, " hidden") {
		t.Errorf("login-ways panel carries a hidden attribute")
	}
	if strings.Contains(p421ReadTemplate(t, "verify_outcome.html.tmpl"), "login-ways") {
		t.Errorf("verify_outcome.html.tmpl carries the login panel")
	}

	rules := parseCSSRules(p421ReadStatic(t, "css/auth.css"))
	p421WantDecls(t, rules, ".login-column", map[string]string{
		"position": "relative", "z-index": "1", "display": "flex", "flex-direction": "column",
		"gap": "var(--space-md)", "width": "100%", "max-width": "420px", // plan 29 finding 15: 16px between the cards
	})
	p421WantDecls(t, rules, ".login-ways", map[string]string{
		"background": "var(--color-surface)", "border": "1px solid var(--color-border)",
		"border-radius": "8px", "padding": "var(--space-md)", "color": "var(--color-text)",
	})
	for _, sel := range []string{".login-ways__low", ".login-ways__off"} {
		p421WantDecls(t, rules, sel, map[string]string{
			"min-height": "var(--touch-target-min)", "font-size": "var(--font-size-body)",
			"font-weight": "var(--font-weight-semibold)", "color": "var(--color-text)",
			"text-decoration": "underline",
		})
	}
	p421WantDecls(t, rules, ".login-ways__links a", map[string]string{
		"min-height": "var(--touch-target-min)", "color": "var(--color-text)", "text-decoration": "underline",
	})
	p421WantDecls(t, rules, ".login-ways__line", map[string]string{
		"margin": "0", "font-size": "var(--font-size-label)",
		"line-height": "var(--line-height-label)", "color": "var(--color-text)",
	})
	p421WantDecls(t, rules, ".login-ways__note", map[string]string{
		"margin": "0", "color": "var(--color-text)",
	})
	p421WantDecls(t, rules, ".login-ways__links", map[string]string{
		"margin": "0", "display": "flex", "flex-wrap": "wrap",
	})

	foundMedia := false
	for _, r := range rules {
		if strings.HasPrefix(r.selectorHead, "@media (max-width: 359.98px)") && strings.HasSuffix(r.selectorHead, ".login-ways__links") {
			foundMedia = true
			if declsOf(r.declBody)["flex-direction"] != "column" {
				t.Errorf("the 359.98px rule does not stack the links: %q", r.declBody)
			}
		}
		if (strings.Contains(r.selectorHead, ".login-ways") || strings.Contains(r.selectorHead, ".login-column")) &&
			strings.Contains(r.declBody, "--color-text-muted") {
			t.Errorf("rule %q uses the muted colour", r.selectorHead)
		}
	}
	if !foundMedia {
		t.Errorf("auth.css has no @media (max-width: 359.98px) rule for .login-ways__links")
	}
}

var p421AnchorTagRE = regexp.MustCompile(`<a [^>]*>`)

func TestAccountMenuItems(t *testing.T) {
	header := p421ReadTemplate(t, "account_header.html.tmpl")
	if strings.Contains(header, "<html") {
		t.Fatalf("account_header.html.tmpl is a partial and must not hold <html")
	}
	start := strings.Index(header, `<div id="account-menu"`)
	if start == -1 {
		t.Fatalf("account_header.html.tmpl has no account menu panel")
	}
	end := strings.Index(header[start:], "</div>")
	if end == -1 {
		t.Fatalf("account menu panel is not closed")
	}
	panel := header[start : start+end]

	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-11, D-02 and
	// D-26): the person menu holds Settings and Log out only. The email line, the
	// Activity item, the Low data view and Open full version items, and the
	// Disclaimer and Policy items left it (each now lives in one place: Settings,
	// About Pinalert, or the nav), and the Low data view's stacked two line item
	// went with them, so its two rules are gone from auth.css. My profile waits
	// for Phase 9. The Phase 3.1 and Phase 4 hands-on tests are walked on this
	// layout after Phase 3.4 is built.
	p421InOrder(t, "menu item order", panel,
		`href="/settings"`, "Settings",
		`action="/auth/logout"`, "Log out")
	for _, gone := range []string{
		"account-menu__email", "account-menu__line", "account-menu__item--stacked",
		`href="/profile"`, `href="/low"`, `href="/?full=1"`, `href="/disclaimer"`, `href="/policy"`,
		"{{.Email}}", "{{if .Chrome.LowDataPage}}",
	} {
		if strings.Contains(panel, gone) {
			t.Errorf("the person menu still holds %q", gone)
		}
	}

	for _, tag := range p421AnchorTagRE.FindAllString(panel, -1) {
		if !strings.Contains(tag, `role="menuitem"`) || !strings.Contains(tag, "account-menu__item") {
			t.Errorf("menu link %s lacks role=menuitem or the account-menu__item class", tag)
		}
	}
	if strings.Contains(panel, "/moderate") {
		t.Errorf("the account menu links to the moderator screen")
	}

	authCSS := p421ReadStatic(t, "css/auth.css")
	for _, gone := range []string{".account-menu__email", ".account-menu__line", ".account-menu__item--stacked"} {
		if strings.Contains(stripCSSComments(authCSS), gone) {
			t.Errorf("auth.css still styles %s, which the person menu no longer draws", gone)
		}
	}
}

func TestAccountMenuMeasuresHeight(t *testing.T) {
	js := stripJSComments(p421ReadStatic(t, "js/account-menu.js"))
	for _, want := range []string{
		"'--account-menu-max-height'", "getBoundingClientRect()", "window.innerHeight",
		".style.setProperty(", "var(--space-md)", "var(--touch-target-min)", "scrollIntoView(",
		"'focusin'", "addEventListener('resize'", "addEventListener('orientationchange'",
		"removeEventListener('resize'", "removeEventListener('orientationchange'",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("account-menu.js lacks %q", want)
		}
	}
	p421InOrder(t, "openMenu order", jsFunctionBody(t, js, "openMenu()"),
		"setOpen(true)", "measurePanel()", "focusFirstMenuItem()")
	setOpen := jsFunctionBody(t, js, "setOpen(open)")
	for _, want := range []string{"addEventListener('resize'", "removeEventListener('resize'"} {
		if !strings.Contains(setOpen, want) {
			t.Errorf("setOpen lacks %q", want)
		}
	}

	css := p421ReadStatic(t, "css/auth.css")
	rules := parseCSSRules(css)
	open, ok := ruleBySelector(rules, "#account-menu:not([hidden])")
	if !ok {
		t.Fatalf("auth.css has no #account-menu:not([hidden]) rule")
	}
	const tail = " - var(--disclaimer-bar-height) - var(--space-md) - var(--touch-target-min) - var(--space-sm) - var(--space-md)))"
	legacy := "max-height: var(--account-menu-max-height, calc(100vh" + tail
	modern := "max-height: var(--account-menu-max-height, calc(100dvh" + tail
	if !strings.Contains(open.declBody, legacy) || !strings.Contains(open.declBody, modern) {
		t.Errorf("the open panel lacks the legacy and dvh max-height pair: %q", open.declBody)
	} else if strings.Index(open.declBody, legacy) > strings.Index(open.declBody, modern) {
		t.Errorf("the 100vh max-height must come before the 100dvh one")
	}
	decls := declsOf(open.declBody)
	if decls["overflow-y"] != "auto" || decls["overscroll-behavior"] != "contain" {
		t.Errorf("the open panel must scroll inside itself: %v", decls)
	}
	kids, ok := ruleBySelector(rules, "#account-menu:not([hidden]) > *")
	if !ok {
		t.Fatalf("auth.css has no #account-menu:not([hidden]) > * rule")
	}
	kd := declsOf(kids.declBody)
	if kd["flex-shrink"] != "0" {
		t.Errorf("menu children must not shrink: %v", kd)
	}
	if _, has := kd["display"]; has {
		t.Errorf("the children rule declares display")
	}

	err := fs.WalkDir(StaticFS, "static/css", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".css") {
			return err
		}
		raw, rerr := StaticFS.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(stripCSSComments(string(raw)), "--account-menu-max-height:") {
			t.Errorf("%s declares --account-menu-max-height, it is only set at runtime", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking static/css: %v", err)
	}
}
