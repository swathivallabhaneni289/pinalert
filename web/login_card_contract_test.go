// Phase 7.1 plan 11 pins the sign in card (UI-SPEC Surfaces 1 to 5, sketch 010):
// the ids once each, the order with the Phase 5 parts kept, real POST forms, the
// field attributes, no muted text on the card, the hidden safe display rules, the
// shared full contrast switched off class, the card error size, and the one row
// of four buttons under the card. The tests render the real login_gate template
// and read the shipped stylesheets out of the embedded filesystems, never a copy
// of their text.

package web

import (
	"bytes"
	"html/template"
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// p711Vm is the view model of a fresh browser, as LoginGate builds it
// (loginGateViewModel in internal/api/handlers/auth.go).
func p711Vm() map[string]any {
	return map[string]any{
		"AssetVersion":    "test",
		"LowDataDefault":  false,
		"DoneLine":        "",
		"StartPanel":      "create",
		"CodeMode":        "",
		"CodeEmail":       "",
		"CodeSecondsLeft": 0,
		"CodeDigits":      6,
		"CodeTTLSeconds":  600,
		"ResendSeconds":   45,
		"PasswordMin":     8,
		"PasswordMax":     128,
	}
}

// p711RenderVM draws login_gate.html.tmpl with the given view model.
func p711RenderVM(t *testing.T, vm map[string]any) string {
	t.Helper()
	tmpl, err := template.ParseFS(TemplatesFS, "templates/*.tmpl")
	if err != nil {
		t.Fatalf("parsing the templates: %v", err)
	}
	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "login_gate.html.tmpl", vm); err != nil {
		t.Fatalf("rendering login_gate.html.tmpl: %v", err)
	}
	return out.String()
}

// p711Tag returns the opening tag of the element with the given id.
func p711Tag(t *testing.T, page, id string) string {
	t.Helper()
	re := regexp.MustCompile(`<[a-z0-9]+\b[^>]*\bid="` + regexp.QuoteMeta(id) + `"[^>]*>`)
	tag := re.FindString(page)
	if tag == "" {
		t.Fatalf("the page has no element with id %q", id)
	}
	return tag
}

// p711CardIDs are the ids of UI-SPEC 1b that live on the card, plus the two of
// the gate.
var p711CardIDs = []string{
	"login-gate", "login-form-panel", "login-title", "login-done", "login-choice", "login-tab-create", "login-tab-signin",
	"auth-panel-create", "auth-panel-signin", "auth-panel-code", "auth-panel-forgot",
	"auth-form-create", "auth-form-signin", "auth-form-code", "auth-form-forgot",
	"create-email", "create-password", "create-show", "create-submit", "create-error",
	"signin-email", "signin-password", "signin-show", "signin-submit", "signin-forgot", "signin-error", "signin-status",
	"code-input", "code-new-password", "code-new-show", "code-submit", "code-resend", "code-change-email",
	"code-error", "code-status", "code-email",
	"forgot-email", "forgot-submit", "forgot-back", "forgot-error",
}

// TestLoginCardIdsOnce: every id of UI-SPEC 1b is on the page exactly once, and
// the ids the old card used are gone.
func TestLoginCardIdsOnce(t *testing.T) {
	vm := p711Vm()
	vm["DoneLine"] = "That sign in link no longer works. Choose Create account with the same email to set a password."
	page := p711RenderVM(t, vm)
	for _, id := range p711CardIDs {
		if n := strings.Count(page, `id="`+id+`"`); n != 1 {
			t.Errorf(`id="%s" appears %d times on the sign in page, want 1`, id, n)
		}
	}
	for _, gone := range []string{"login-email", "request-link", "auth-error", "check-inbox", "resend-link", "refresh-inbox", "link-expiry-notice"} {
		if strings.Contains(page, `id="`+gone+`"`) {
			t.Errorf(`the old id "%s" is still on the sign in page`, gone)
		}
	}
	// The data attributes the script reads travel on the card.
	card := p711Tag(t, page, "login-form-panel")
	for _, attr := range []string{"data-start-panel", "data-code-mode", "data-code-seconds-left", "data-code-ttl-seconds", "data-resend-seconds", "data-password-min"} {
		if !strings.Contains(card, attr+"=") {
			t.Errorf("#login-form-panel lacks %s", attr)
		}
	}
	// A done line is absent when no word was given.
	if strings.Contains(p711RenderVM(t, p711Vm()), `id="login-done"`) {
		t.Errorf("the page draws a done line when LoginGate set none")
	}
}

// TestLoginCardOrder: the Phase 5 order survives inside the new card.
func TestLoginCardOrder(t *testing.T) {
	src := p421ReadTemplate(t, "login_gate.html.tmpl")
	p421InOrder(t, "login_gate.html.tmpl", src,
		`id="login-gate"`, `class="login-column"`, `id="login-form-panel"`, `id="login-done"`, `id="login-title"`, `id="login-choice"`,
		`id="auth-panel-create"`, `id="auth-panel-signin"`, `id="auth-panel-code"`, `id="auth-panel-forgot"`,
		`class="login-ways"`, `{{template "login_ways_links" .}}`, `{{template "login_globe" .}}`)

	page := p711RenderVM(t, p711Vm())
	p421InOrder(t, "rendered card", page,
		`id="login-form-panel"`, `id="login-title"`, `id="login-choice"`, `id="auth-panel-create"`, `id="create-email"`,
		`id="create-password"`, `id="create-submit"`, `id="auth-panel-signin"`, `id="auth-panel-code"`, `id="auth-panel-forgot"`,
		`class="login-ways"`, `id="login-try-demo"`, `id="login-globe"`)
	// The globe sits after the whole column and never inside the card.
	cardAt := strings.Index(page, `id="login-form-panel"`)
	waysAt := strings.Index(page, `class="login-ways"`)
	if strings.Contains(page[cardAt:waysAt], "login-globe") {
		t.Errorf("the globe markup sits inside the card")
	}
	// Within each form the primary button comes before the secondary ones.
	for _, pair := range [][2]string{{"signin-submit", "signin-forgot"}, {"code-submit", "code-resend"}, {"code-resend", "code-change-email"}, {"forgot-submit", "forgot-back"}} {
		a, b := strings.Index(page, `id="`+pair[0]+`"`), strings.Index(page, `id="`+pair[1]+`"`)
		if a == -1 || b == -1 || a > b {
			t.Errorf("%s must come before %s in the markup (tab order: primary first)", pair[0], pair[1])
		}
	}
	// The credit line stays outside the faded wrapper (the partial is untouched).
	if !strings.Contains(page, `id="globe-credit"`) {
		t.Errorf("the credit line is missing from the page")
	}
}

var p711FormTagRE = regexp.MustCompile(`<form\b[^>]*>`)

// TestLoginFormsArePost: every panel is a real POST form to /login, so a press
// of Enter with the script not loaded can never put an email or a password in
// the page address (D-10).
func TestLoginFormsArePost(t *testing.T) {
	page := p711RenderVM(t, p711Vm())
	forms := p711FormTagRE.FindAllString(page, -1)
	if len(forms) != 4 {
		t.Fatalf("the sign in page holds %d forms, want 4", len(forms))
	}
	for _, f := range forms {
		if !strings.Contains(f, `method="post"`) || !strings.Contains(f, `action="/login"`) {
			t.Errorf("form %s is not method=post action=/login", f)
		}
	}
	if strings.Contains(strings.ToLower(page), `method="get"`) {
		t.Errorf("the sign in page holds a GET form")
	}
	src := p421ReadTemplate(t, "login_gate.html.tmpl")
	if strings.Contains(strings.ToLower(src), "method=get") || strings.Contains(strings.ToLower(src), `method="get"`) {
		t.Errorf("login_gate.html.tmpl names a GET form")
	}
}

// TestLoginFieldAttributes: the labels, types and attributes of UI-SPEC Surfaces
// 2 to 5, with the length numbers taken from the view model.
func TestLoginFieldAttributes(t *testing.T) {
	page := p711RenderVM(t, p711Vm())
	want := func(id string, attrs ...string) {
		t.Helper()
		tag := p711Tag(t, page, id)
		for _, a := range attrs {
			if !strings.Contains(tag, a) {
				t.Errorf("#%s lacks %s in %s", id, a, tag)
			}
		}
	}
	for _, id := range []string{"create-email", "signin-email", "forgot-email"} {
		want(id, `type="email"`, `name="email"`, `autocomplete="username"`, `inputmode="email"`, `autocapitalize="none"`,
			`autocorrect="off"`, `spellcheck="false"`, ` required`, `placeholder="you@example.com"`)
	}
	want("create-password", `type="password"`, `name="new-password"`, `autocomplete="new-password"`, `minlength="8"`, `maxlength="128"`,
		` required`, `autocapitalize="none"`, `autocorrect="off"`, `spellcheck="false"`)
	want("signin-password", `type="password"`, `name="password"`, `autocomplete="current-password"`, `maxlength="128"`, ` required`)
	if strings.Contains(p711Tag(t, page, "signin-password"), "minlength") {
		t.Errorf("the Sign in password field has a minlength; only new passwords have a minimum")
	}
	want("code-new-password", `type="password"`, `name="new-password"`, `autocomplete="new-password"`, `minlength="8"`, `maxlength="128"`)
	want("code-input", `type="text"`, `name="code"`, `inputmode="numeric"`, `pattern="[0-9]*"`, `autocomplete="one-time-code"`,
		`maxlength="6"`, `enterkeyhint="done"`, ` required`, `autocapitalize="none"`, `autocorrect="off"`, `spellcheck="false"`)
	if n := strings.Count(page, `autocomplete="one-time-code"`); n != 1 {
		t.Errorf(`autocomplete="one-time-code" appears %d times, want 1`, n)
	}
	for _, show := range []string{"create-show", "signin-show", "code-new-show"} {
		want(show, `type="button"`, `aria-label="Show password"`, `data-show-for="`)
		if !regexp.MustCompile(`id="` + show + `"[^>]*>Show</button>`).MatchString(page) {
			t.Errorf("#%s does not read Show", show)
		}
	}
	for _, submit := range []string{"create-submit", "signin-submit", "code-submit", "forgot-submit"} {
		want(submit, `type="submit"`)
	}
	for _, secondary := range []string{"signin-forgot", "code-resend", "code-change-email", "forgot-back"} {
		want(secondary, `type="button"`)
	}
	want("code-resend", `aria-disabled="true"`)
	want("login-choice", `role="tablist"`, `aria-label="Account"`)
	want("login-tab-create", `role="tab"`, `aria-selected="true"`, `aria-controls="auth-panel-create"`, `tabindex="0"`)
	want("login-tab-signin", `role="tab"`, `aria-selected="false"`, `aria-controls="auth-panel-signin"`, `tabindex="-1"`)
	for _, errID := range []string{"create-error", "signin-error", "code-error", "forgot-error"} {
		want(errID, `role="alert"`, ` hidden`, `class="inline-error"`)
	}
	for _, statusID := range []string{"signin-status", "code-status"} {
		want(statusID, `role="status"`, ` hidden`)
	}
	// The labels.
	for _, label := range []string{`for="create-email">Email address`, `for="create-password">Password`, `for="signin-email">Email address`,
		`for="signin-password">Password`, `for="code-input">6 digit code`, `for="code-new-password">New password`, `for="forgot-email">Email address`} {
		if !strings.Contains(page, label) {
			t.Errorf("the page lacks the label %q", label)
		}
	}
	// The numbers come from the view model: change them and the page follows.
	vm := p711Vm()
	vm["PasswordMin"], vm["PasswordMax"], vm["CodeDigits"] = 10, 64, 8
	other := p711RenderVM(t, vm)
	for _, w := range []string{`minlength="10"`, `maxlength="64"`, `maxlength="8"`, "At least 10 characters."} {
		if !strings.Contains(other, w) {
			t.Errorf("the page does not follow the view model: lacks %s", w)
		}
	}
}

// p711NamesCard reports whether a selector head styles the card or its row.
func p711NamesCard(head string) bool {
	for _, name := range []string{".login-card", ".login-choice", ".login-rule", ".login-show", ".login-status", ".login-ways", ".login-actions", ".login-field--code", ".login-email"} {
		if strings.Contains(head, name) {
			return true
		}
	}
	return false
}

// TestLoginCardNoMutedText: muted text on the --color-surface card measures 4.34
// to 1 and fails, so no rule that styles the card, its choices, rule lines or the
// row uses --color-text-muted for a colour.
func TestLoginCardNoMutedText(t *testing.T) {
	rules := parseCSSRules(p421ReadStatic(t, "css/auth.css"))
	checked := 0
	for _, r := range rules {
		if !p711NamesCard(r.selectorHead) {
			continue
		}
		checked++
		for prop, value := range declsOf(r.declBody) {
			if strings.Contains(value, "--color-text-muted") && (prop == "color" || strings.HasPrefix(prop, "background") || prop == "fill") {
				t.Errorf("%s sets %s to %s: no muted text on the card", r.selectorHead, prop, value)
			}
		}
	}
	if checked < 12 {
		t.Errorf("only %d card rules were found in auth.css; the selector filter has drifted", checked)
	}
	// The globe stylesheet adds only glass and a solid backing, never a muted colour.
	for _, r := range parseCSSRules(p421ReadStatic(t, "css/login-globe.css")) {
		if p711NamesCard(r.selectorHead) && strings.Contains(r.declBody, "--color-text-muted") {
			t.Errorf("login-globe.css %s uses the muted colour", r.selectorHead)
		}
	}
}

// TestLoginCardHiddenSafeDisplay: a display rule on an element the script swaps
// with the hidden attribute has :not([hidden]) in its head (the repo's standing
// trap: an author display beats the user agent's hidden).
func TestLoginCardHiddenSafeDisplay(t *testing.T) {
	swappable := []string{".login-choice", "#auth-panel", "#auth-form", "#code-new-wrap", ".login-status", ".inline-error", "#login-done", ".login-done"}
	checked := 0
	for _, r := range parseCSSRules(p421ReadStatic(t, "css/auth.css")) {
		if _, has := declsOf(r.declBody)["display"]; !has {
			continue
		}
		head := r.selectorHead
		if at := strings.LastIndex(head, "{"); at != -1 {
			head = head[at+1:]
		}
		for _, part := range strings.Split(head, ",") {
			part = strings.TrimSpace(part)
			named := false
			for _, s := range swappable {
				if strings.Contains(part, s) && !strings.Contains(part, "login-choice__tab") {
					named = true
				}
			}
			if !named {
				continue
			}
			checked++
			if !strings.Contains(part, ":not([hidden])") {
				t.Errorf("auth.css display rule %q names a swappable element without :not([hidden])", part)
			}
		}
	}
	if checked == 0 {
		t.Errorf("no display rule on a swappable element was found; the filter has drifted")
	}
	// The markup really does use hidden on these, so the rules above matter.
	page := p711RenderVM(t, p711Vm())
	for _, id := range []string{"auth-panel-signin", "auth-panel-code", "auth-panel-forgot", "code-new-wrap", "create-error"} {
		if !regexp.MustCompile(`\shidden[\s>]`).MatchString(p711Tag(t, page, id)) {
			t.Errorf("#%s is not hidden when the page opens on Create account", id)
		}
	}
}

// TestBtnOffFullContrast: the shared switched off class keeps full contrast text
// (sketch 010 finding 3), outranks both shared aria-disabled looks, and is worn by
// the buttons that switch off on this card.
func TestBtnOffFullContrast(t *testing.T) {
	raw := p421ReadStatic(t, "css/chrome.css")
	css := stripCSSComments(raw)
	var rule cssRule
	found := false
	for _, r := range parseCSSRules(raw) {
		// The top level rule is the first match; the forced colours one repeats the
		// head inside its media block.
		if !found && r.selectorHead == `.btn.btn--off[aria-disabled="true"]:not(.btn--destructive)` {
			rule, found = r, true
		}
	}
	if !found {
		t.Fatalf("chrome.css has no .btn.btn--off[aria-disabled=\"true\"] rule")
	}
	got := declsOf(rule.declBody)
	for prop, want := range map[string]string{"color": "var(--color-text)", "background": "var(--color-surface)", "border-color": "var(--color-border)", "box-shadow": "none", "cursor": "not-allowed"} {
		if got[prop] != want {
			t.Errorf(".btn--off %s = %q, want %q", prop, got[prop], want)
		}
	}
	if strings.Contains(rule.declBody, "--color-text-muted") {
		t.Errorf(".btn--off uses the muted colour; the switched off look keeps full contrast")
	}
	// It comes after both shared aria-disabled rules and counts as much as the
	// heavier one (four: a class, two :not() arguments and the attribute), so it
	// wins, hover included.
	offAt := strings.Index(css, `.btn.btn--off[aria-disabled="true"]:not(.btn--destructive) {`)
	outlinedAt := strings.Index(css, `.btn:not(.btn--primary):not(.btn--destructive)[aria-disabled="true"]`)
	primaryAt := strings.Index(css, `.btn--primary[aria-disabled="true"]`)
	if offAt == -1 || outlinedAt == -1 || primaryAt == -1 || offAt < outlinedAt || offAt < primaryAt {
		t.Errorf(".btn--off must come after the shared aria-disabled rules (off %d, outlined %d, primary %d)", offAt, outlinedAt, primaryAt)
	}
	// Forced colours: a plain text coloured border.
	forcedOK := false
	for _, it := range p47jItems(css) {
		if it.Prelude != "@media (forced-colors: active)" {
			continue
		}
		for _, inner := range p47jItems(it.Body) {
			if inner.Prelude == `.btn.btn--off[aria-disabled="true"]:not(.btn--destructive)` &&
				strings.Contains(strings.Join(strings.Fields(inner.Body), " "), "border: 1px solid CanvasText;") {
				forcedOK = true
			}
		}
	}
	if !forcedOK {
		t.Errorf("chrome.css has no forced colours rule giving .btn--off a CanvasText border")
	}
	// The contrast of the pair in both themes.
	main := p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css"))
	themes := map[string]map[string]string{
		"light": p16Rule(t, main, "", ":root").Decls,
		"dark":  p16Rule(t, main, "", `:root[data-theme="dark"]`).Decls,
	}
	for name, tok := range themes {
		if ratio := wcagContrastRatio(t, tok["--color-text"], tok["--color-surface"], name+" switched off button"); ratio < 4.5 {
			t.Errorf("%s: the switched off button text is %.2f to 1, want at least 4.5", name, ratio)
		}
	}
	// The card's buttons that switch off wear the class.
	page := p711RenderVM(t, p711Vm())
	for _, id := range []string{"code-submit", "code-resend"} {
		if !strings.Contains(p711Tag(t, page, id), "btn--off") {
			t.Errorf("#%s does not carry btn--off", id)
		}
	}
	// The old card's muted look is not re-added on the card.
	authCSS := p421ReadStatic(t, "css/auth.css")
	if strings.Contains(authCSS, `.login-card .btn[aria-disabled="true"]`) {
		t.Errorf("auth.css restyles aria-disabled buttons on the card; use .btn--off from chrome.css")
	}
}

// TestLoginCardNoTagline: nothing sits between the title and the choices except
// whitespace (owner rule: no hero text, sketch 010 finding 4).
func TestLoginCardNoTagline(t *testing.T) {
	src := p421ReadTemplate(t, "login_gate.html.tmpl")
	titleEnd := strings.Index(src, "</h1>")
	choiceAt := strings.Index(src, `<div class="login-choice"`)
	if titleEnd == -1 || choiceAt == -1 || choiceAt < titleEnd {
		t.Fatalf("login_gate.html.tmpl: title ends at %d, choices start at %d", titleEnd, choiceAt)
	}
	if between := strings.TrimSpace(src[titleEnd+len("</h1>") : choiceAt]); between != "" {
		t.Errorf("something sits between the title and the choices: %q", between)
	}
	page := p711RenderVM(t, p711Vm())
	if !regexp.MustCompile(`</h1>\s*<div class="login-choice"`).MatchString(page) {
		t.Errorf("the rendered page has an element between the title and the choices")
	}
	if !regexp.MustCompile(`<h1 id="login-title"[^>]*>Pinalert</h1>`).MatchString(page) {
		t.Errorf("the title does not read Pinalert on the entry panels")
	}
}

// TestLoginCardErrorSize: the card's error and status lines are Label 14px as the
// approved UI-SPEC and sketch 010 draw them (sketch 010 finding 2, decided in
// plan 11), through a card scoped rule that beats ".login-card p"; the shared
// .inline-error stays Body 16px for step 4 and the windows.
func TestLoginCardErrorSize(t *testing.T) {
	auth := parseCSSRules(p421ReadStatic(t, "css/auth.css"))
	cardRule, ok := p421RuleFor(auth, ".login-card .inline-error:not([hidden])")
	if !ok {
		t.Fatalf("auth.css has no .login-card .inline-error:not([hidden]) rule")
	}
	got := declsOf(cardRule.declBody)
	if got["font-size"] != "var(--font-size-label)" || got["line-height"] != "var(--line-height-label)" {
		t.Errorf("the card error rule is %q / %q, want the Label size and line height", got["font-size"], got["line-height"])
	}
	chrome := parseCSSRules(p421ReadStatic(t, "css/chrome.css"))
	shared, ok := p421RuleFor(chrome, ".inline-error:not([hidden])")
	if !ok {
		t.Fatalf("chrome.css has no shared .inline-error:not([hidden]) rule")
	}
	if sg := declsOf(shared.declBody); sg["font-size"] != "var(--font-size-body)" {
		t.Errorf("the shared .inline-error font-size is %q, want var(--font-size-body); the card scoped rule is the only 14px one", sg["font-size"])
	}
	// The card rule has three class level parts, the .login-card p rule only one
	// class and one element, so it wins; the rule line and status line are Label too.
	pRule, ok := p421RuleFor(auth, ".login-card p")
	if !ok {
		t.Fatalf("auth.css has no .login-card p rule")
	}
	if declsOf(pRule.declBody)["font-size"] != "var(--font-size-body)" {
		t.Errorf(".login-card p is no longer Body size; the card scoped rule exists to beat it")
	}
	for _, sel := range []string{".login-card .login-rule", ".login-card .login-status"} {
		if r, has := p421RuleFor(auth, sel); !has || declsOf(r.declBody)["font-size"] != "var(--font-size-label)" {
			t.Errorf("%s is not Label 14px", sel)
		}
	}
	// The windows keep 16px: no other card rule changes .inline-error size.
	for _, r := range auth {
		if strings.Contains(r.selectorHead, ".inline-error") && !strings.Contains(r.selectorHead, ".login-card") && !strings.Contains(r.selectorHead, ".logout-confirm") {
			if v := declsOf(r.declBody)["font-size"]; v != "" {
				t.Errorf("%s sets font-size %s outside the card", r.selectorHead, v)
			}
		}
	}
}

var p711LinkRE = regexp.MustCompile(`(?s)<a class="btn"([^>]*)>(.*?)</a>`)

// TestLoginWaysOneRowOfButtons: UI-SPEC 1e. One row of four outlined buttons
// under the card, no box, no hint line, a two by two grid on the phone.
func TestLoginWaysOneRowOfButtons(t *testing.T) {
	for _, low := range []bool{false, true} {
		vm := p711Vm()
		vm["LowDataDefault"] = low
		page := p711RenderVM(t, vm)
		cardAt := strings.Index(page, `id="login-form-panel"`)
		waysAt := strings.Index(page, `id="login-ways"`)
		if cardAt == -1 || waysAt == -1 || waysAt < cardAt {
			t.Fatalf("lowData=%v: card at %d, wrapper at %d", low, cardAt, waysAt)
		}
		card := page[cardAt:waysAt]
		for _, word := range []string{"Try the demo", "Low data view", "Disclaimer", "Policy", "Turn off low data view"} {
			if strings.Contains(card, word) {
				t.Errorf("lowData=%v: %q is inside the card; the four buttons sit under it", low, word)
			}
		}
		// The wrapper and the row, up to the globe or the end of the gate.
		region := page[waysAt:]
		if g := strings.Index(region, `class="globe-area"`); g != -1 {
			region = region[:g]
		}
		links := p711LinkRE.FindAllStringSubmatch(region, -1)
		if len(links) != 4 {
			t.Fatalf("lowData=%v: %d a.btn elements under the card, want 4", low, len(links))
		}
		lowWord, lowHref := "Low data view", "/login?low=1"
		if low {
			lowWord, lowHref = "Turn off low data view", "/login?low=0"
		}
		want := []struct{ word, href string }{
			{"Try the demo", "/demo"}, {lowWord, lowHref}, {"Disclaimer", "/disclaimer"}, {"Policy", "/policy"},
		}
		for i, w := range want {
			if strings.TrimSpace(links[i][2]) != w.word || !strings.Contains(links[i][1], `href="`+w.href+`"`) {
				t.Errorf("lowData=%v: button %d is %q %q, want %q %q", low, i+1, strings.TrimSpace(links[i][2]), links[i][1], w.word, w.href)
			}
		}
		if !strings.Contains(links[0][1], `id="login-try-demo"`) {
			t.Errorf("lowData=%v: Try the demo has lost id login-try-demo", low)
		}
		if !strings.Contains(links[1][1], `id="login-low"`) {
			t.Errorf("lowData=%v: the Low data button has no id login-low", low)
		}
		if strings.Contains(page, "Text only, no map") {
			t.Errorf("lowData=%v: the hint line of the old panel is still on the page", low)
		}
		statusLine := "Low data view is on. It opens after you sign in."
		wantStatus := 0
		if low {
			wantStatus = 1
		}
		if n := strings.Count(page, statusLine); n != wantStatus {
			t.Errorf("lowData=%v: the status line appears %d times, want %d", low, n, wantStatus)
		}
		if low {
			if at, first := strings.Index(region, statusLine), strings.Index(region, `<a class="btn"`); at == -1 || at > first {
				t.Errorf("the status line is not above the row")
			}
		}
	}

	// The gate template holds none of the four words as a button; only the call.
	src := p421ReadTemplate(t, "login_gate.html.tmpl")
	for _, word := range []string{"Try the demo", "Disclaimer", "Policy", "Turn off low data view", ">Low data view<"} {
		if strings.Contains(src, word) {
			t.Errorf("login_gate.html.tmpl holds %q; the row lives in login_ways_links.html.tmpl", word)
		}
	}
	if n := strings.Count(src, `{{template "login_ways_links" .}}`); n != 1 {
		t.Errorf("login_gate.html.tmpl includes the row %d times, want 1", n)
	}

	// The style: no box on the wrapper or the row, a two column grid on the phone.
	rules := parseCSSRules(p421ReadStatic(t, "css/auth.css"))
	for _, sel := range []string{".login-ways", ".login-ways__row"} {
		r, ok := p421RuleFor(rules, sel)
		if !ok {
			t.Errorf("auth.css has no %s rule", sel)
			continue
		}
		got := declsOf(r.declBody)
		for _, boxProp := range []string{"background", "border", "padding", "box-shadow"} {
			if v, has := got[boxProp]; has {
				t.Errorf("%s sets %s %q; the row has no box behind it", sel, boxProp, v)
			}
		}
	}
	if r, ok := p421RuleFor(rules, ".login-ways__row"); ok {
		got := declsOf(r.declBody)
		if got["display"] != "grid" || got["grid-template-columns"] != "1fr 1fr" || got["gap"] != "var(--space-sm)" {
			t.Errorf(".login-ways__row is not the two by two phone grid: %q", r.declBody)
		}
	}
}

// TestLoginCardLockIcon: lock.svg is the one new icon file of the phase. It is a
// plain Lucide stroke glyph with the licence line of search.svg, registered in
// nonCategoryIcons, and drawn by a literal url mask in auth.css that the card uses.
func TestLoginCardLockIcon(t *testing.T) {
	ref, err := fs.ReadFile(StaticFS, "static/icons/search.svg")
	if err != nil {
		t.Fatalf("reading search.svg: %v", err)
	}
	refFirst, _, _ := strings.Cut(string(ref), "\n")
	raw, err := fs.ReadFile(StaticFS, "static/icons/lock.svg")
	if err != nil {
		t.Fatalf("lock.svg is missing from the embedded tree: %v", err)
	}
	text := string(raw)
	if first, _, _ := strings.Cut(text, "\n"); first != refFirst {
		t.Errorf("lock.svg first line %q differs from search.svg's licence line %q", first, refFirst)
	}
	for _, want := range []string{`viewBox="0 0 24 24"`, `stroke="currentColor"`, `stroke-width="2"`, `fill="none"`} {
		if !strings.Contains(text, want) {
			t.Errorf("lock.svg lacks %s", want)
		}
	}
	if strings.Contains(text, "<script") || strings.Contains(text, "href") || strings.Contains(text, "<text") {
		t.Errorf("lock.svg holds a script, a link or text, which a mask glyph never carries")
	}
	if !nonCategoryIcons["static/icons/lock.svg"] {
		t.Errorf("lock.svg is not registered in nonCategoryIcons")
	}
	if !strings.Contains(p421ReadStatic(t, "css/auth.css"), `url("/static/icons/lock.svg")`) {
		t.Errorf("auth.css does not mask lock.svg with the literal url")
	}
	if n := strings.Count(p711RenderVM(t, p711Vm()), "auth-icon--lock"); n != 3 {
		t.Errorf("the card draws the lock %d times, want 3 (Create, Sign in, New password)", n)
	}
}
