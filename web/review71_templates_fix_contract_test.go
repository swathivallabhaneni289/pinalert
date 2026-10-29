// The Phase 7.1 code review fix, templates part (07.1-REVIEW-templates.md, fix
// round 1, 2026-10-29). Each test pins one finding: the sign in card's focus
// ring and its title, the Reliable reporter tick box of the installed app's ask,
// the Policy's storage and services lines, the install box on a short iPhone,
// the globe box on 360 to 389 wide phones, the dead rules of the old emailed
// link sign in, and the gofmt state of one test file. The tests read the shipped
// stylesheets and templates out of the embedded filesystems, never a copy.

package web

import (
	"strings"
	"testing"
)

// r71tRules parses one embedded stylesheet.
func r71tRules(t *testing.T, file string) []p16CSSRule {
	t.Helper()
	return p16ParseCSS(file, p16ReadStatic(t, file))
}

// r71tFind returns the rule with exactly this head in exactly this media
// prelude (empty means top level), without stopping the test when it is
// missing.
func r71tFind(rules []p16CSSRule, media, head string) (p16CSSRule, bool) {
	for _, r := range rules {
		if r.Head == head && r.Media == media {
			return r, true
		}
	}
	return p16CSSRule{}, false
}

// r71tThemes returns the light and dark token sets of main.css.
func r71tThemes(t *testing.T) map[string]map[string]string {
	t.Helper()
	main := r71tRules(t, "static/css/main.css")
	return map[string]map[string]string{
		"light": p16Rule(t, main, "", ":root").Decls,
		"dark":  p16Rule(t, main, "", `:root[data-theme="dark"]`).Decls,
	}
}

// TestLoginFieldFocusRing (CR-T01): every field of the sign in card shows a
// focus ring, as sketch 010 drew it (.login-field:focus-within, 2px solid, 2px
// out). The input itself draws none, so there is one ring per field. The ring is
// the focus ring token, drawn 2px outside the field over the card, so it is
// checked against what the card can be: the 65 percent glass over anything from
// black to white, and the solid surface and page colour of the three fallbacks
// and the Low data page, in both themes, with the 3 to 1 floor for a focus
// indicator (WCAG 1.4.11).
func TestLoginFieldFocusRing(t *testing.T) {
	auth := r71tRules(t, "static/css/auth.css")
	ring, ok := r71tFind(auth, "", ".login-field:focus-within")
	if !ok {
		t.Fatalf("auth.css has no top level .login-field:focus-within rule, so no card field shows focus")
	}
	p16MustDecl(t, ".login-field:focus-within", ring, map[string]string{
		"outline":        "2px solid var(--color-focus-ring)",
		"outline-offset": "2px",
	})
	if r, ok := r71tFind(auth, "", ".login-field input:focus"); !ok || r.Decls["outline"] != "none" {
		t.Errorf(".login-field input:focus must keep outline: none, so the field draws the one ring")
	}
	// Nothing on the page with the globe takes the ring away again.
	for _, file := range []string{"static/css/login-globe.css", "static/css/chrome.css", "static/css/auth.css"} {
		for _, r := range r71tRules(t, file) {
			if strings.Contains(r.Head, ".login-field") && r.Decls["outline"] == "none" && !strings.Contains(r.Head, "input:focus") {
				t.Errorf("%s: %q (%s) sets outline: none on the field", file, r.Head, r.Media)
			}
		}
	}

	for name, tok := range r71tThemes(t) {
		ringHex := tok["--color-focus-ring"]
		backs := map[string]string{
			"solid surface":    tok["--color-surface"],
			"solid page":       tok["--color-bg"],
			"glass over black": mixHexSRGB(t, tok["--color-surface"], 65, "#000000", 35, name+" glass over black"),
			"glass over white": mixHexSRGB(t, tok["--color-surface"], 65, "#FFFFFF", 35, name+" glass over white"),
		}
		for back, hex := range backs {
			ctx := name + " focus ring on the " + back
			got := wcagContrastRatio(t, ringHex, hex, ctx)
			t.Logf("%s: %.2f to 1", ctx, got)
			if got < 3 {
				t.Errorf("%s is %.2f to 1, want at least 3", ctx, got)
			}
		}
	}
}

// TestLoginCardTitleNoRing (WR-T06): the card title takes focus when a panel
// swaps (tabindex -1, never a Tab stop) and draws no ring, as UI-SPEC 1c and
// sketch 010 say, like the first time flow title and the Log out box title.
// Without the rule, :focus-visible draws a ring after a keyboard Enter.
func TestLoginCardTitleNoRing(t *testing.T) {
	auth := r71tRules(t, "static/css/auth.css")
	r, ok := r71tFind(auth, "", ".login-card h1:focus")
	if !ok {
		t.Fatalf("auth.css has no .login-card h1:focus rule, so the title shows a ring when the code panel opens")
	}
	p16MustDecl(t, ".login-card h1:focus", r, map[string]string{"outline": "none"})
	// The same pin the Log out box title has, kept beside it.
	p16MustDecl(t, ".logout-confirm__title:focus", p16Rule(t, auth, "", ".logout-confirm__title:focus"), map[string]string{"outline": "none"})
	// The title is still the focus target and still not a Tab stop.
	page := p711RenderVM(t, p711Vm())
	if tag := p711Tag(t, page, "login-title"); !strings.Contains(tag, `tabindex="-1"`) {
		t.Errorf("#login-title lost tabindex -1: %s", tag)
	}
}

// TestReliableTickBoxSharedLook (WR-T01 with WR-B04): the Reliable reporter tick
// box looks the same in the installed app's ask on Nearby as in the sign up
// pop-up: a flex row at least 44px high, a 20px box, the text colour and 16px
// above Not now and Allow. Before the fix the rules were scoped to the sign up
// pop-up's ids and lived in welcome.css, which Nearby does not link, so on
// Nearby the label was an inline line with a 12px box against the buttons. The
// shared rules live in auth.css, which both pages link, scoped to the Log out
// box class both pop-ups use, and welcome.css keeps no copy.
func TestReliableTickBoxSharedLook(t *testing.T) {
	auth := r71tRules(t, "static/css/auth.css")
	const labelHead = ".logout-confirm .welcome-ask__check"
	const boxHead = `.logout-confirm .welcome-ask__check input[type="checkbox"]`
	if r, ok := r71tFind(auth, "", labelHead); !ok {
		t.Errorf("auth.css has no %s rule, so the tick box on Nearby is unstyled", labelHead)
	} else {
		p16MustDecl(t, labelHead, r, map[string]string{
			"display":     "flex",
			"align-items": "center",
			"gap":         "var(--space-sm)",
			"min-height":  "var(--touch-target-min)",
			"margin":      "0 0 var(--space-md)",
			"color":       "var(--color-text)",
			"font-size":   "var(--font-size-body)",
			"line-height": "var(--line-height-body)",
			"cursor":      "pointer",
		})
	}
	if r, ok := r71tFind(auth, "", boxHead); !ok {
		t.Errorf("auth.css has no %s rule, so the box on Nearby is the small native one", boxHead)
	} else {
		p16MustDecl(t, boxHead, r, map[string]string{"flex": "none", "width": "20px", "height": "20px", "margin": "0"})
	}
	// The installed ask's title leaves 16px to the tick box, as the sign up
	// pop-up's does, not the Log out box's 24px.
	if r, ok := r71tFind(auth, "", "#installed-alerts-ask-title"); !ok {
		t.Errorf("auth.css has no #installed-alerts-ask-title rule")
	} else {
		p16MustDecl(t, "#installed-alerts-ask-title", r, map[string]string{"margin-bottom": "var(--space-md)"})
	}

	// Both pages link auth.css, so both pop-ups get the one rule.
	for _, page := range []string{"index.html.tmpl", "welcome.html.tmpl"} {
		if !strings.Contains(stripReadTemplate(t, page), `<link rel="stylesheet" href="/static/css/auth.css?v={{.AssetVersion}}">`) {
			t.Errorf("%s does not link auth.css", page)
		}
	}
	// One rule: welcome.css keeps no id scoped copy, and no other stylesheet
	// names the tick box.
	welcome := stripCSSComments(p16ReadStatic(t, "static/css/welcome.css"))
	for _, gone := range []string{"welcome-ask__check", "#welcome-alerts-reliable"} {
		if strings.Contains(welcome, gone) {
			t.Errorf("welcome.css still holds %q; the look lives in auth.css only", gone)
		}
	}
	for _, r := range p16AllCSS(t) {
		if strings.Contains(r.Head, "welcome-ask__check") && r.File != "static/css/auth.css" {
			t.Errorf("%s names the tick box (%q); auth.css holds its only rules", r.File, r.Head)
		}
	}

	// The markup the script reads is unchanged: the box keeps its id inside the
	// shared label class in both pop-ups.
	p13MustContain(t, "installed_alerts_ask.html.tmpl", stripReadTemplate(t, "installed_alerts_ask.html.tmpl"),
		`<label class="welcome-ask__check"><input type="checkbox" id="installed-alerts-ask-reliable"><span>A Reliable reporter posts near me</span></label>`,
		`<h2 class="logout-confirm__title" id="installed-alerts-ask-title" tabindex="-1">`,
		`<dialog id="installed-alerts-ask" class="logout-confirm"`)
	p13MustContain(t, "welcome.html.tmpl", stripReadTemplate(t, "welcome.html.tmpl"),
		`<label class="welcome-ask__check"><input type="checkbox" id="welcome-alerts-reliable"><span>A Reliable reporter posts near me</span></label>`)
}
