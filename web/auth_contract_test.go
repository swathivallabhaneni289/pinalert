// Phase 7.1 plan 12 (2026-10-29, D-12) rewrote web/static/js/auth.js as the panel
// state machine of the new sign in card. Three tests of this file pinned pieces
// that no longer exist and are removed: TestCheckInboxTemplateCarriesTimerAttributes
// (the data-cooldown-seconds and data-link-ttl-seconds attributes of the check
// inbox partial, which the card no longer includes), TestAuthJSReadsTimerAttributes
// (auth.js reading those two attributes) and TestAuthJSHasTwoIndependentTimers (the
// link's two clocks). The new card has two clocks of its own, read from
// data-resend-seconds and data-code-ttl-seconds, and web/auth_flow_test.mjs pins
// both. TestAuthJSUsesNoMarkupParsingSink is kept as it was (only the dash
// characters of its messages were replaced, by the no dash rule). Four tests are
// new: TestAuthJSPanelContract, TestLoginNoLinkNoEmailInURL,
// TestAuthJSSentencesOnce and TestAuthJSLeavesPushIntent.

package web

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// authJSPath and loginGateTemplatePath are read from the same embedded
// filesystems every other contract test in this package uses (TemplatesFS,
// StaticFS), never a copy of their text, so a future edit to either asset can
// only drift this test's own claim, not silently diverge from what ships in the
// binary.
const (
	authJSPath            = "static/js/auth.js"
	loginGateTemplatePath = "templates/login_gate.html.tmpl"
)

func readAuthJS(t *testing.T) string {
	t.Helper()
	raw, err := StaticFS.ReadFile(authJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", authJSPath, err)
	}
	return string(raw)
}

func readLoginGateTemplate(t *testing.T) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile(loginGateTemplatePath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", loginGateTemplatePath, err)
	}
	return string(raw)
}

// TestAuthJSPanelContract: every id auth.js looks up, and every data attribute it
// reads off the card, exists in the sign in template (plan 11 draws it, plan 12
// drives it). A renamed id would otherwise leave the script returning early on a
// page that still looks right.
func TestAuthJSPanelContract(t *testing.T) {
	script := readAuthJS(t)
	page := readLoginGateTemplate(t)

	// Every lookup is a literal byId('...') call. The three Show buttons name
	// their field in data-show-for, checked below against the template as well.
	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`byId\('([^']+)'\)`).FindAllStringSubmatch(script, -1) {
		ids[m[1]] = true
	}
	// The floor stops the regular expression from quietly matching nothing.
	if len(ids) < 40 {
		t.Fatalf("found %d literal id lookups in %s, want at least 40; did the lookup helper change?", len(ids), authJSPath)
	}
	var missing []string
	for id := range ids {
		if !strings.Contains(page, `id="`+id+`"`) {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%s looks up ids that %s does not draw: %v", authJSPath, loginGateTemplatePath, missing)
	}

	// The card's data attributes the script reads.
	for _, attr := range []string{
		"data-start-panel", "data-code-mode", "data-code-seconds-left",
		"data-code-ttl-seconds", "data-resend-seconds", "data-show-for",
		// Amended 2026-10-29 by the Phase 7.1 review fix round 2 (skeptic
		// SK-04): the full resend cooldown, drawn on every page.
		"data-resend-cooldown-seconds",
	} {
		if !strings.Contains(script, "'"+attr+"'") {
			t.Errorf("%s does not read %s", authJSPath, attr)
		}
		if !strings.Contains(page, attr+"=") {
			t.Errorf("%s does not draw %s, which %s reads", loginGateTemplatePath, attr, authJSPath)
		}
	}

	// Each Show button names an input that exists.
	for _, m := range regexp.MustCompile(`data-show-for="([^"]+)"`).FindAllStringSubmatch(page, -1) {
		if !strings.Contains(page, `id="`+m[1]+`"`) {
			t.Errorf("a Show button names the field %q, which the template does not draw", m[1])
		}
	}

	// The routes of plan 07 appear as literal paths, so a rename is a test failure.
	for _, route := range []string{
		"/api/auth/sign-up", "/api/auth/sign-up/code", "/api/auth/code/resend",
		"/api/auth/sign-in", "/api/auth/reset", "/api/auth/reset/code", "/api/auth/reset/password", "/api/auth/reset/cancel",
	} {
		if !strings.Contains(script, "'"+route+"'") {
			t.Errorf("%s never posts to %s", authJSPath, route)
		}
	}
	// None of the removed ids is looked up.
	for _, gone := range []string{"login-email", "request-link", "auth-error", "check-inbox", "resend-link", "refresh-inbox", "link-expiry-notice"} {
		if strings.Contains(script, "'"+gone+"'") {
			t.Errorf("%s still looks up the removed id %q", authJSPath, gone)
		}
	}
}

// TestLoginNoLinkNoEmailInURL pins D-05 and D-10: the sign in script never reads
// the page address's query string, rewrites the address bar, or builds a sent
// query, and the card holds no link and no form that could put an email in the
// address. The row of links under the card is outside the card and allowed.
func TestLoginNoLinkNoEmailInURL(t *testing.T) {
	script := readAuthJS(t)
	for _, banned := range []string{
		"location.search", "location.hash", "URLSearchParams", "new URL(",
		"replaceState", "pushState", "history.", "sent=", "document.referrer",
	} {
		if strings.Contains(script, banned) {
			t.Errorf("%s contains %q; the email must never reach the page address (D-05)", authJSPath, banned)
		}
	}

	page := readLoginGateTemplate(t)
	for _, banned := range []string{"?sent", "sent=", "location.search", "URLSearchParams", `method="get"`, `method='get'`} {
		if strings.Contains(page, banned) {
			t.Errorf("%s contains %q; the card must not carry an address in the page address", loginGateTemplatePath, banned)
		}
	}

	// The card runs from its opening tag to the row of links under it.
	start := strings.Index(page, `id="login-form-panel"`)
	end := strings.Index(page, `class="login-ways"`)
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("could not find the card in %s (start %d, end %d)", loginGateTemplatePath, start, end)
	}
	card := page[start:end]
	for _, banned := range []string{"<a ", "<a>", "href="} {
		if strings.Contains(card, banned) {
			t.Errorf("the sign in card holds %q; no link may sit inside the card panels", banned)
		}
	}
	// Every form of the card is a POST form.
	forms := regexp.MustCompile(`<form\b[^>]*>`).FindAllString(card, -1)
	if len(forms) != 4 {
		t.Fatalf("found %d forms in the card, want 4", len(forms))
	}
	for _, form := range forms {
		if !strings.Contains(form, `method="post"`) {
			t.Errorf("a card form is not a POST form: %s", form)
		}
	}
}

// TestAuthJSSentencesOnce: the two pinned sentences are one constant each, so
// they cannot drift apart. The rate limit sentence is shared with the server
// (internal/ratelimit and handlers.authRateLimitedMessage); the wrong sign in
// sentence is shared with service.WrongCredentialsMessage.
func TestAuthJSSentencesOnce(t *testing.T) {
	script := readAuthJS(t)
	for _, sentence := range []string{
		"Too many requests. Try again in a minute.",
		"Email or password is wrong.",
	} {
		if got := strings.Count(script, sentence); got != 1 {
			t.Errorf("%s: the sentence %q appears %d times, want exactly 1", authJSPath, sentence, got)
		}
	}
}

// TestAuthJSLeavesPushIntent: the sign in script never touches browser storage,
// and so never the Phase 3.3 alert intent flag, whose clearing at log out is an
// open owner question (UI-SPEC default 18). The server deletes a replaced
// session's push subscriptions instead (plan 05).
func TestAuthJSLeavesPushIntent(t *testing.T) {
	script := readAuthJS(t)
	for _, banned := range []string{"localStorage", "sessionStorage", "indexedDB", "document.cookie", "pushIntent"} {
		if strings.Contains(script, banned) {
			t.Errorf("%s contains %q; the sign in script must not read or write anything the browser keeps", authJSPath, banned)
		}
	}
}

// TestAuthJSUsesNoMarkupParsingSink is this file's own guard against T-01-03 for
// every string auth.js puts on the page (the server's sentences, the address on
// the code screen, the busy words and the Resend label): all of it is inserted as
// text content, never assembled into markup for the browser to parse. Phase 1's
// threat register forbids these sinks project-wide, but nothing previously
// guarded this specific file.
func TestAuthJSUsesNoMarkupParsingSink(t *testing.T) {
	text := readAuthJS(t)

	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(text, sink) {
			t.Errorf("%s: found forbidden markup-parsing sink %q (T-01-03): every string reaching "+
				"the DOM in this file must be inserted as text content, never assembled into markup",
				authJSPath, sink)
		}
	}
}
