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

// TestInstallAskNotNowStaysInView (WR-T05): on a short or zoomed iPhone (320x568,
// or an iPhone SE tab at 115 percent, 326x476) the install box is taller than
// the screen, and its only way out, Not now, sat under the box's visible bottom.
// The box keeps the Log out box look and scrolls inside itself as before, and
// its row of buttons sticks to the box's bottom edge on the page colour, so the
// steps scroll under Not now and Not now never leaves the screen. The row's
// negative margins give back exactly the space its own padding adds, so a box
// that fits looks as it did: 24px from the steps to the button and 24px under
// it. Measured in Chromium and WebKit at 320x568, 326x476, 375x548 and 390x664.
func TestInstallAskNotNowStaysInView(t *testing.T) {
	guide := r71tRules(t, "static/css/device-guide.css")
	const head = "#welcome-install-ask .logout-confirm__actions"
	r, ok := r71tFind(guide, "", head)
	if !ok {
		t.Fatalf("device-guide.css has no %s rule, so Not now can sit under the box's bottom on a short iPhone", head)
	}
	p16MustDecl(t, head, r, map[string]string{
		"position":   "sticky",
		"bottom":     "0",
		"margin":     "calc(-1 * var(--space-sm)) 0 calc(-1 * var(--space-lg))",
		"padding":    "var(--space-sm) 0 var(--space-lg)",
		"background": "var(--color-bg)",
	})
	// The row never takes a display of its own here, so the hidden guard of
	// .logout-confirm__actions:not([hidden]) stays the only display rule.
	if _, has := r.Decls["display"]; has {
		t.Errorf("%s declares a display; it must leave that to the guarded rule in auth.css", head)
	}
	// The steps keep their 24px under them, and the box keeps its cap and its own
	// scroll, with the page colour the sticky row paints.
	p16MustDecl(t, ".logout-confirm__body > .device-guide__steps",
		p16Rule(t, guide, "", ".logout-confirm__body > .device-guide__steps"), map[string]string{"margin": "0 0 var(--space-lg)"})
	box := p16Rule(t, r71tRules(t, "static/css/auth.css"), "", ".logout-confirm")
	p16MustDecl(t, ".logout-confirm", box, map[string]string{
		"max-height": "calc(100% - 2 * var(--space-md))",
		"overflow-y": "auto",
		"background": "var(--color-bg)",
	})
	p16MustDecl(t, ".logout-confirm__body", p16Rule(t, r71tRules(t, "static/css/auth.css"), "", ".logout-confirm__body"),
		map[string]string{"padding": "var(--space-lg)"})
	// The row is the last thing in the box body, after the guide.
	ask := stripReadTemplate(t, "welcome.html.tmpl")
	if i := strings.Index(ask, `<dialog id="welcome-install-ask"`); i < 0 {
		t.Fatalf("welcome.html.tmpl lost the install box")
	} else {
		p13MustMatch(t, "install box", ask[i:], `(?s)^<dialog id="welcome-install-ask"[^>]*>\s*<div class="logout-confirm__body">.*\{\{template "device_guide"\}\}\s*<div class="logout-confirm__actions">\s*<button type="button" class="btn" id="welcome-install-later">Not now</button>\s*</div>\s*</div>\s*</dialog>`)
	}
}

// r71tPolicySection returns the Policy's section that starts with the heading
// and runs to the next heading.
func r71tPolicySection(t *testing.T, heading string) string {
	t.Helper()
	body := stripReadTemplate(t, "legal_body.html.tmpl")
	_, policy, found := strings.Cut(body, `{{define "legal_policy_body"}}`)
	if !found {
		t.Fatalf("legal_body.html.tmpl has no policy part")
	}
	start := strings.Index(policy, "<h3>"+heading+"</h3>")
	if start < 0 {
		t.Fatalf("the Policy lost its %q section", heading)
	}
	rest := policy[start+len(heading)+9:]
	if end := strings.Index(rest, "<h3>"); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

// TestPolicyNamesAskedLists (WR-T03): Phase 7.1 added two browser keys that keep
// lists of account ids, pinalert.installAsked (plan 13b) and
// pinalert.alertsAsked (plan 13c), so a phone is not asked twice. The Policy's
// storage list reads as complete, so it names them in plain words. The older
// keys the list does not name (pinalert.pushIntent, pinalert.changesAsked and
// the per account pinalert.locationAsked. marker) predate this phase and are
// recorded in 07.1-REVIEW-FIX-templates.md, not changed here.
func TestPolicyNamesAskedLists(t *testing.T) {
	const line = "<li>Which accounts on this device were already asked about alerts and about adding Pinalert to the Home Screen, kept as a list of account numbers, so this device does not ask again.</li>"
	storage := r71tPolicySection(t, "Cookies and storage on your device")
	if !strings.Contains(storage, line) {
		t.Errorf("the Policy's storage list does not name the asked lists:\n%s", line)
	}
	// The line stays true: the sign up flow still keeps both lists.
	js := stripJSComments(p421ReadStatic(t, "js/welcome.js"))
	for _, key := range []string{"'pinalert.installAsked'", "'pinalert.alertsAsked'"} {
		if !strings.Contains(js, key) {
			t.Errorf("welcome.js no longer uses %s; the Policy line about it may be stale", key)
		}
	}
}

// TestPolicyNamesHomeSquareScreeningAndMail (IN-T02): the Policy says what is
// kept and who sees it, in plain words. Pinalert keeps a 5 character geohash of
// the home area (about 5 km wide, migration 00008, HomeAreaCellPrecision 5)
// beside its name, new usernames go to OpenAI's moderation service as the
// report text does, and every sign in email goes out through Resend, which sees
// the address and the email, codes included.
func TestPolicyNamesHomeSquareScreeningAndMail(t *testing.T) {
	keeps := r71tPolicySection(t, "What Pinalert keeps")
	const home = "<li>The name of your home area, a town or area and never a street, and a rough square about 5 km wide around it, used for alerts.</li>"
	if !strings.Contains(keeps, home) {
		t.Errorf("the Policy's keeps list does not name the home area square:\n%s", home)
	}
	others := r71tPolicySection(t, "Other services Pinalert uses")
	for _, line := range []string{
		"<li>Report and username checks: OpenAI, as described above.</li>",
		"<li>Sign in emails: Resend sends them, so it sees your email address and the email, including any code.</li>",
	} {
		if !strings.Contains(others, line) {
			t.Errorf("the Policy's other services list lacks:\n%s", line)
		}
	}
	if strings.Contains(others, "<li>Report checks: OpenAI, as described above.</li>") {
		t.Errorf("the other services list still names OpenAI for report checks only")
	}
	// The username screening paragraph the services line points back to stays.
	if !strings.Contains(r71tPolicySection(t, "How reports are checked"), "Usernames are checked by the same service when you choose or change one.") {
		t.Errorf("the Policy lost the username screening paragraph")
	}
}

// TestAuthCSSHasNoEmailedLinkRules (IN-T03): the old emailed link sign in is
// gone (Phase 7.1 plans 11 and 16), and so are its ids and classes in every
// template, so auth.css keeps none of its rules. The Low data page links
// auth.css and counts its bytes.
func TestAuthCSSHasNoEmailedLinkRules(t *testing.T) {
	dead := []string{"#check-inbox", ".verify-outcome", ".auth-error", "#request-link", "#resend-link"}
	auth := p16ReadStatic(t, "static/css/auth.css")
	for _, r := range r71tRules(t, "static/css/auth.css") {
		for _, sel := range dead {
			if strings.Contains(r.Head, sel) {
				t.Errorf("auth.css still has a rule for %s (%q)", sel, r.Head)
			}
		}
	}
	// The file's header no longer describes the link gate.
	head := auth
	if end := strings.Index(auth, "*/"); end >= 0 {
		head = auth[:end]
	}
	for _, stale := range []string{"check-inbox", "mandatory-email-verification", "link-landing"} {
		if strings.Contains(head, stale) {
			t.Errorf("the auth.css header still describes the old link gate (%q)", stale)
		}
	}
	// Nothing draws them any more, so nothing lost a look.
	for _, name := range []string{"login_gate.html.tmpl", "login_ways_links.html.tmpl", "low.html.tmpl", "welcome.html.tmpl", "settings.html.tmpl", "index.html.tmpl"} {
		page := stripReadTemplate(t, name)
		for _, attr := range []string{`id="check-inbox"`, `verify-outcome`, `auth-error`, `id="request-link"`, `id="resend-link"`} {
			if strings.Contains(page, attr) {
				t.Errorf("%s still uses %s; its rule in auth.css was removed", name, attr)
			}
		}
	}
}

// TestLoginGlobeBoxFrom360 (IN-T01): from 360px wide the gate and the card pad
// 24px instead of 16px, so the card sits 16px lower and the 404px term of --g,
// set for 320, left Email me a code 5 or 6px under the first screen at 360x640,
// 375x548 and 389x600. A block from 360px reserves 420px (404 plus the two 8px
// steps) in the same clamp, vh first and svh in @supports, after the phone
// block so it wins. The phone block, its clamp and every fallback stay as they
// are (TestLoginGlobePhoneBoxReserved and TestLoginGlassFallbacks).
func TestLoginGlobeBoxFrom360(t *testing.T) {
	css := stripCSSComments(p421ReadStatic(t, "css/login-globe.css"))
	const prelude = "@media (min-width: 360px) and (max-width: 899.98px)"
	const clampVH = "clamp(140px, min(100vw - 48px, (100vh - 420px) * 1.28), 300px)"
	const clampSVH = "clamp(140px, min(100vw - 48px, (100svh - 420px) * 1.28), 300px)"
	wide, ok := p47jFindBlock(css, prelude)
	if !ok {
		t.Fatalf("login-globe.css has no %q block, so the main button sits under the first screen from 360 to 389 wide", prelude)
	}
	phone, ok := p47jFindBlock(css, p47jPhonePrelude)
	if !ok {
		t.Fatalf("login-globe.css lost the phone block")
	}
	if wide.Start < phone.Start {
		t.Errorf("the 360px block comes before the phone block and would lose to it")
	}
	gate := p47jRuleRaw(t, prelude, wide.Body, p47jGateHead)
	if !strings.Contains(gate, "--g: "+clampVH+";") {
		t.Errorf("the 360px block's %s lacks --g: %s (has %q)", p47jGateHead, clampVH, gate)
	}
	if n := strings.Count(gate, "--g:"); n != 1 {
		t.Errorf("the 360px vh rule declares --g %d times, want 1", n)
	}
	supports, ok := p47jFindBlock(wide.Body, "@supports (height: 100svh)")
	if !ok {
		t.Fatalf("the 360px block has no nested @supports (height: 100svh)")
	}
	if svh := p47jRuleRaw(t, "360px svh", supports.Body, p47jGateHead); !strings.Contains(svh, "--g: "+clampSVH+";") {
		t.Errorf("the 360px svh rule lacks --g: %s (has %q)", clampSVH, svh)
	}
	// The block declares only --g: the rest of the phone layout is the phone
	// block's.
	for _, it := range p47jLeaves(wide.Body) {
		for _, decl := range strings.Split(it.Body, ";") {
			if d := strings.TrimSpace(decl); d != "" && !strings.HasPrefix(d, "--g:") {
				t.Errorf("the 360px block sets %q; it only moves --g", d)
			}
		}
	}
	// The width where the padding grows is the same as in auth.css.
	if !strings.Contains(stripCSSComments(p421ReadStatic(t, "css/auth.css")), "@media (max-width: 359.98px)") {
		t.Errorf("auth.css lost its 359.98px padding step; the 360px globe block follows it")
	}
}
