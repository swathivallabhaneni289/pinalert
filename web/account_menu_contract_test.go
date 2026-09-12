package web

import (
	"strings"
	"testing"
)

// accountHeaderTemplatePath and accountMenuJSPath are read from the same
// embedded filesystems every other contract test in this package uses
// (TemplatesFS, StaticFS) — never a copy of their text — so a future edit
// to either asset can only drift this test's own claim, not silently
// diverge from what actually ships in the binary.
const (
	accountHeaderTemplatePath = "templates/account_header.html.tmpl"
	accountMenuJSPath         = "static/js/account-menu.js"
)

// TestAccountMenuJSReferencesHeaderElements asserts account-menu.js
// addresses the two server-rendered elements the header partial actually
// renders (account_header.html.tmpl), by id — not a copy of either id
// pasted into this test, but the live embedded template text, so a rename
// on either side can only drift this test, never silently pass while the
// script addresses an element that no longer exists.
func TestAccountMenuJSReferencesHeaderElements(t *testing.T) {
	tmplRaw, err := TemplatesFS.ReadFile(accountHeaderTemplatePath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", accountHeaderTemplatePath, err)
	}
	tmplText := string(tmplRaw)

	jsRaw, err := StaticFS.ReadFile(accountMenuJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", accountMenuJSPath, err)
	}
	jsText := string(jsRaw)

	for _, id := range []string{`id="account-menu-trigger"`, `id="account-menu"`} {
		if !strings.Contains(tmplText, id) {
			t.Fatalf("%s no longer contains %s — this test's premise (the script addresses "+
				"elements the header partial actually renders) no longer holds", accountHeaderTemplatePath, id)
		}
	}

	if !strings.Contains(jsText, "account-menu-trigger") {
		t.Errorf("%s: expected a reference to %q — the trigger button the header partial renders",
			accountMenuJSPath, "account-menu-trigger")
	}
	if got := strings.Count(jsText, "account-menu"); got < 2 {
		t.Errorf("%s: found %d occurrences of %q, want at least 2 — the script must address both "+
			"the trigger and the panel by id", accountMenuJSPath, got, "account-menu")
	}
}

// TestAccountMenuJSHandlesEscapeAndTab guards UI-SPEC item 14's keyboard
// contract: Escape dismisses the menu, and Tab is handled so focus stays
// trapped inside the open panel.
func TestAccountMenuJSHandlesEscapeAndTab(t *testing.T) {
	raw, err := StaticFS.ReadFile(accountMenuJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", accountMenuJSPath, err)
	}
	text := string(raw)

	for _, key := range []string{"'Escape'", "'Tab'"} {
		if !strings.Contains(text, key) {
			t.Errorf("%s: expected a reference to %s — both the Escape-dismiss and the Tab-trap "+
				"handlers must exist", accountMenuJSPath, key)
		}
	}
}

// TestAccountMenuJSAssignsAriaExpanded guards the announced-state-matches-
// visible-state contract: a menu whose aria-expanded can disagree with its
// actual open/closed state is worse for a screen-reader user than one
// carrying no ARIA at all.
func TestAccountMenuJSAssignsAriaExpanded(t *testing.T) {
	raw, err := StaticFS.ReadFile(accountMenuJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", accountMenuJSPath, err)
	}
	text := string(raw)

	if !strings.Contains(text, "aria-expanded") {
		t.Errorf("%s: expected a reference to %q", accountMenuJSPath, "aria-expanded")
	}
}

// TestAccountMenuJSOutsideClickListenerIsTornDown asserts the document-level
// outside-click listener is both bound and unbound — a listener that is
// only ever added and never removed would still work functionally but
// would cost every stray click on the page a no-op handler call for the
// rest of the page's lifetime, on every gated page, once this header
// renders everywhere.
func TestAccountMenuJSOutsideClickListenerIsTornDown(t *testing.T) {
	raw, err := StaticFS.ReadFile(accountMenuJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", accountMenuJSPath, err)
	}
	text := string(raw)

	if got := strings.Count(text, "addEventListener"); got < 2 {
		t.Errorf("%s: found %d addEventListener call sites, want at least 2 (trigger click, panel "+
			"keydown, and/or the outside-click listener)", accountMenuJSPath, got)
	}
	if !strings.Contains(text, "removeEventListener") {
		t.Errorf("%s: expected at least one removeEventListener call — the outside-click listener "+
			"must be torn down when the menu closes, not left bound forever", accountMenuJSPath)
	}
}

// TestAccountMenuJSUsesNoMarkupParsingSink is this file's own guard against
// T-01-03: account-menu.js only toggles attributes and moves focus over
// markup Task 1 already server-rendered, so it must never assemble a
// string into markup for the browser to parse.
func TestAccountMenuJSUsesNoMarkupParsingSink(t *testing.T) {
	raw, err := StaticFS.ReadFile(accountMenuJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", accountMenuJSPath, err)
	}
	text := string(raw)

	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(text, sink) {
			t.Errorf("%s: found forbidden markup-parsing sink %q (T-01-03) — every string reaching "+
				"the DOM in this file must be inserted as text content or an attribute value, never "+
				"assembled into markup", accountMenuJSPath, sink)
		}
	}
}

// TestAccountMenuJSHasNoAppShellDependency asserts account-menu.js never
// references window.Pinalert — plan 01.1-07's profile page loads this file
// alone, without app.js/map.js/modal.js/feed.js, so a Pinalert reference
// here would throw on that page.
func TestAccountMenuJSHasNoAppShellDependency(t *testing.T) {
	raw, err := StaticFS.ReadFile(accountMenuJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", accountMenuJSPath, err)
	}
	text := string(raw)

	if strings.Contains(text, "Pinalert") {
		t.Errorf("%s: found a reference to %q — this script must run unchanged on 01.1-07's profile "+
			"page, which loads none of app.js/map.js/modal.js/feed.js", accountMenuJSPath, "Pinalert")
	}
}
