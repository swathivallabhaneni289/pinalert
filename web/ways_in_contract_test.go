// Plan 04-21 pins the markup and styles of the ways in to the Low data view:
// the login page panel (D-17, D-23, P4-A18), the account menu items and its
// measured height (P4-A11). The tests read the shipped templates and
// stylesheets out of the embedded filesystems, never a copy of their text.

package web

import (
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
		"gap": "var(--space-sm)", "width": "100%", "max-width": "420px",
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
