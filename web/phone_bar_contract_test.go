package web

import (
	"regexp"
	"strings"
	"testing"
)

// TestPhoneShellSubtractsTheBar pins the Phase 4 follow up to plan 04-08:
// phone.css sits below main.css and wins at phone width, so its own
// body > .app-shell height must give up the height of the unofficial bar the
// same way main.css does. Otherwise the shell is one bar taller than the
// viewport and the page scrolls on a phone. Nothing here needs a database or a
// server; the test reads the embedded stylesheet.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
// D-31): the bar on every page is replaced by the laptop strip on Nearby and
// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
// every page) is retired as worded; Disclaimer and Policy are reached from the
// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
// first-time flow, the login screen and the one reader. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
// The phone shell still subtracts the token, which is now 0px on a phone
// because the strip is hidden there (D-30); the rule itself is unchanged.
func TestPhoneShellSubtractsTheBar(t *testing.T) {
	css := stripCSSComments(p16ReadStatic(t, "static/css/phone.css"))

	mediaAt := strings.Index(css, phoneBlockMedia)
	if mediaAt == -1 {
		t.Fatalf("phone.css has no %s block", phoneBlockMedia)
	}

	shell := regexp.MustCompile(`(?m)^\s*body > \.app-shell\s*\{([^}]*)\}`)
	matches := shell.FindAllStringSubmatchIndex(css, -1)
	if len(matches) != 1 {
		t.Fatalf("phone.css has %d body > .app-shell rules, want exactly 1", len(matches))
	}
	m := matches[0]
	if m[0] < mediaAt {
		t.Fatalf("the body > .app-shell rule sits before the %s block, want it inside", phoneBlockMedia)
	}
	body := css[m[2]:m[3]]

	var heights []string
	for _, decl := range strings.Split(body, ";") {
		name, value, has := strings.Cut(decl, ":")
		if !has {
			continue
		}
		switch strings.TrimSpace(name) {
		case "height", "min-height", "max-height":
			heights = append(heights, strings.TrimSpace(name)+": "+strings.TrimSpace(value))
		}
	}
	if len(heights) == 0 {
		t.Fatal("the phone body > .app-shell rule declares no height")
	}

	// Both forms, the legacy unit first and the dynamic unit second, with the
	// bar subtracted from each.
	p16Ordered(t, strings.Join(heights, "; "), "phone body > .app-shell heights",
		"height: calc(100vh - var(--disclaimer-bar-height))",
		"height: calc(100dvh - var(--disclaimer-bar-height))",
	)

	// No height on the shell may state a viewport unit without the bar.
	for _, h := range heights {
		if !strings.Contains(h, "100vh") && !strings.Contains(h, "100dvh") {
			continue
		}
		if !strings.Contains(h, "calc(") || !strings.Contains(h, "- var(--disclaimer-bar-height))") {
			t.Errorf("phone body > .app-shell declares %q, which ignores the bar", h)
		}
	}

	// And nowhere else in the file may a bare full viewport height remain on
	// the shell or on the map container.
	bare := regexp.MustCompile(`(?m)^\s*(height|min-height)\s*:\s*100d?vh\s*;`)
	if loc := bare.FindStringIndex(css); loc != nil {
		t.Errorf("phone.css declares a bare full viewport height near %q, which ignores the bar", strings.TrimSpace(css[loc[0]:loc[1]]))
	}
}
