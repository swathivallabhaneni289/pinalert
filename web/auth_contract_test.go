package web

import (
	"strings"
	"testing"
)

// checkInboxTemplatePath and authJSPath are read from the same embedded
// filesystems every other contract test in this package uses (TemplatesFS,
// StaticFS) — never a copy of their text — so a future edit to either
// asset can only drift this test's own claim, not silently diverge from
// what actually ships in the binary.
const (
	checkInboxTemplatePath = "templates/check_inbox.html.tmpl"
	authJSPath             = "static/js/auth.js"
)

// TestCheckInboxTemplateCarriesTimerAttributes asserts the check-inbox
// panel renders both data attributes plan 01.1-05 adds — the server-side
// single source of truth for the two independent timers (UI-SPEC item 9)
// — exactly once each, so a future edit can't silently drop one while
// keeping the other.
func TestCheckInboxTemplateCarriesTimerAttributes(t *testing.T) {
	raw, err := TemplatesFS.ReadFile(checkInboxTemplatePath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", checkInboxTemplatePath, err)
	}
	text := string(raw)

	if got := strings.Count(text, "data-cooldown-seconds"); got != 1 {
		t.Errorf("%s: found %d occurrences of %q, want exactly 1", checkInboxTemplatePath, got, "data-cooldown-seconds")
	}
	if got := strings.Count(text, "data-link-ttl-seconds"); got != 1 {
		t.Errorf("%s: found %d occurrences of %q, want exactly 1", checkInboxTemplatePath, got, "data-link-ttl-seconds")
	}
}

// TestAuthJSReadsTimerAttributes asserts auth.js actually reads both
// server-rendered timer attributes off the check-inbox panel, rather than
// hard-coding either duration client-side.
func TestAuthJSReadsTimerAttributes(t *testing.T) {
	raw, err := StaticFS.ReadFile(authJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", authJSPath, err)
	}
	text := string(raw)

	if !strings.Contains(text, "data-cooldown-seconds") {
		t.Errorf("%s: expected a reference to %q — the resend countdown must read its duration off "+
			"the server-rendered panel, never a hard-coded client-side value", authJSPath, "data-cooldown-seconds")
	}
	if !strings.Contains(text, "data-link-ttl-seconds") {
		t.Errorf("%s: expected a reference to %q — the link-expiry notice must read its duration off "+
			"the server-rendered panel, never a hard-coded client-side value", authJSPath, "data-link-ttl-seconds")
	}
}

// TestAuthJSHasTwoIndependentTimers guards UI-SPEC item 9 directly: the
// resend countdown and the link-expiry notice are separate clocks, which in
// this codebase's plain-JS style means two distinct setInterval call sites,
// not one interval whose callback drives both concerns. Two occurrences is
// the floor, not an exact count — a future addition that needs a third
// independent timer would legitimately push this higher, and this test
// should not have to be revisited for that.
func TestAuthJSHasTwoIndependentTimers(t *testing.T) {
	raw, err := StaticFS.ReadFile(authJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", authJSPath, err)
	}
	text := string(raw)

	if got := strings.Count(text, "setInterval"); got < 2 {
		t.Errorf("%s: found %d setInterval call sites, want at least 2 — the resend countdown and "+
			"the link-expiry notice must run on independent timers, never one interval driving both",
			authJSPath, got)
	}
}

// TestAuthJSUsesNoMarkupParsingSink is this file's own guard against T-01-03
// for the newly-added countdown/expiry-notice code path — every string that
// reaches the DOM here (including the "Resend in {n}s" label and the
// expiry notice's copy) must be inserted as text content, never assembled
// into markup for the browser to parse. Phase 1's threat register forbids
// these sinks project-wide, but nothing previously guarded this specific
// file.
func TestAuthJSUsesNoMarkupParsingSink(t *testing.T) {
	raw, err := StaticFS.ReadFile(authJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", authJSPath, err)
	}
	text := string(raw)

	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(text, sink) {
			t.Errorf("%s: found forbidden markup-parsing sink %q (T-01-03) — every string reaching "+
				"the DOM in this file must be inserted as text content, never assembled into markup",
				authJSPath, sink)
		}
	}
}
