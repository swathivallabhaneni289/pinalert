package web

// Phase 3.4 plan 04: the person menu, the bell, Official alerts and Settings.
// These tests read the shipped templates and scripts from the embedded trees,
// never a copy, so they can only drift with the product.

import (
	"strings"
	"testing"
)

// TestPersonMenuItems pins D-11, D-02 and D-26: the laptop person menu holds
// exactly Settings and Log out, shows no email, and links nowhere that has its
// own place (Settings holds the Disclaimer, Policy and Low data view rows).
func TestPersonMenuItems(t *testing.T) {
	header := p421ReadTemplate(t, "account_header.html.tmpl")
	if got := strings.Count(header, `role="menuitem"`); got != 2 {
		t.Errorf("the person menu has %d items, want exactly 2 (Settings and Log out)", got)
	}
	for _, want := range []string{`href="/settings"`, "Settings", `action="/auth/logout"`, "Log out", `auth-icon--settings`, `auth-icon--log-out`} {
		if !strings.Contains(header, want) {
			t.Errorf("the person menu lacks %q", want)
		}
	}
	for _, banned := range []string{
		"{{.Email}}", "{{if .Chrome.LowDataPage}}", `href="/profile"`, `href="/low"`,
		`href="/disclaimer"`, `href="/policy"`, `href="/?full=1"`, "My profile",
	} {
		if strings.Contains(header, banned) {
			t.Errorf("the person menu still holds %q", banned)
		}
	}
}

// TestBellListContract pins D-32 and UI-SPEC Surface 9: the bell is a button
// named Updates with no count, it opens a dialog list that says there are no
// updates yet and links to the Updates part of My activity, and the script that
// runs it builds no markup and makes no request.
func TestBellListContract(t *testing.T) {
	bar := p421ReadTemplate(t, "app_bar.html.tmpl")
	trigger := bellTagWith(t, bar, `id="bell-trigger"`)
	for _, want := range []string{
		`aria-label="Updates"`, `aria-haspopup="dialog"`, `aria-expanded="false"`, `aria-controls="bell-list"`,
		`class="app-topbar__bell"`, `type="button"`,
	} {
		if !strings.Contains(trigger, want) {
			t.Errorf("the bell button %s lacks %s", trigger, want)
		}
	}
	list := bellTagWith(t, bar, `id="bell-list"`)
	for _, want := range []string{`role="dialog"`, `aria-label="Updates"`, ` hidden`} {
		if !strings.Contains(list, want) {
			t.Errorf("the bell list %s lacks %s", list, want)
		}
	}
	p421InOrder(t, "bell list order", bar,
		`id="bell-list"`, "Updates", `aria-label="Close"`, "No updates yet", `href="/profile?part=updates"`, "See all updates")
	if strings.Count(bar, "No updates yet") != 1 || strings.Count(bar, "See all updates") != 1 {
		t.Errorf("the bell list words must each appear once")
	}
	// No count: the bell draws no badge while no updates exist.
	for _, banned := range []string{"bell-badge", "app-topbar__badge", "data-count"} {
		if strings.Contains(bar, banned) {
			t.Errorf("the bell carries %q, but no count exists before Phase 3.2", banned)
		}
	}
	// The bell sits left of the gear and before the person menu.
	p421InOrder(t, "bell before gear before person", bar, `id="bell-trigger"`, "app-topbar__gear", "account_header.html.tmpl")

	js := stripJSComments(p421ReadStatic(t, "js/app-bar.js"))
	for _, want := range []string{"aria-expanded", "'Escape'", "bell-trigger", "bell-list", "history.back()", "document.referrer"} {
		if !strings.Contains(js, want) {
			t.Errorf("app-bar.js lacks %q", want)
		}
	}
	for _, banned := range []string{"fetch(", "XMLHttpRequest", "innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "DOMParser", "createContextualFragment", "eval("} {
		if strings.Contains(js, banned) {
			t.Errorf("app-bar.js uses %q, a markup parsing sink or a request", banned)
		}
	}
	// The back link target is the server's own href: the script never reads a
	// target out of the address or the referrer (T-34-20).
	for _, banned := range []string{"location.href =", "location.assign", "location.replace", "window.open"} {
		if strings.Contains(js, banned) {
			t.Errorf("app-bar.js navigates with %q; the back arrow follows its own href", banned)
		}
	}

	css := stripCSSComments(p421ReadStatic(t, "css/app-bar.css"))
	for _, want := range []string{".bell-list:not([hidden])", "width: 340px", "max-width: calc(100% - 24px)", "z-index: 1060", "var(--shadow-menu)", "border-radius: 8px"} {
		if !strings.Contains(css, want) {
			t.Errorf("app-bar.css lacks %q for the bell list", want)
		}
	}
}

// bellTagWith returns the opening tag of the element in the template that
// carries the given attribute text.
func bellTagWith(t *testing.T, tmpl, attr string) string {
	t.Helper()
	at := strings.Index(tmpl, attr)
	if at == -1 {
		t.Fatalf("the template has no element with %s", attr)
	}
	start := strings.LastIndex(tmpl[:at], "<")
	end := strings.Index(tmpl[at:], ">")
	if start == -1 || end == -1 {
		t.Fatalf("cannot find the tag around %s", attr)
	}
	return tmpl[start : at+end+1]
}
