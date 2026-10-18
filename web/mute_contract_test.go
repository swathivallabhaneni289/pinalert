package web

// Phase 3.2 plan 11: the Mute control of an opened report. These tests read
// the shipped scripts, templates and stylesheet from the embedded trees, never
// a copy. Honest limit: static inspection proves the words, the routes, the
// order and the absence of forbidden shapes. It cannot prove how a browser
// routes a tap or where focus lands; web/mute_test.mjs runs the real script
// against fakes for the behaviour, and the owner's look at the finished build
// (Mute and Flag on one line at 1280, Flag under Mute in the 296px phone
// pane) is the layout check.

import (
	"strings"
	"testing"
)

// TestMuteControlShape pins the words, the routes and the build rules of
// mute.js (UI-SPEC Surface C, P32-C15 and P32-C16).
func TestMuteControlShape(t *testing.T) {
	mute := p424ReadJS(t, "static/js/mute.js")
	raw := p424ReadRaw(t, "static/js/mute.js")

	if n := strings.Count(mute, "window.PinalertMute = "); n != 1 {
		t.Errorf("mute.js assigns window.PinalertMute %d times, want 1", n)
	}
	p424Contains(t, "mute.js", mute,
		"createMuteControl: createMuteControl", "updateMuteControl: updateMuteControl", "shouldShow: shouldShow")

	// Every word of the two controls and the four lines appears once.
	for _, want := range []string{
		"'Mute this report'",
		"'Turn updates back on'",
		"'Updates are off for this report.'",
		"'Updates are on again for this report.'",
		"\"Couldn't change this. Try again.\"",
		"\"You're offline, so this wasn't saved. Try again when you have a signal.\"",
	} {
		if n := strings.Count(mute, want); n != 1 {
			t.Errorf("mute.js holds %s %d times, want 1", want, n)
		}
	}
	if n := strings.Count(mute, "function shouldShow(report)"); n != 1 {
		t.Errorf("mute.js declares shouldShow(report) %d times, want 1", n)
	}

	// Both routes and the refresh event.
	p424Contains(t, "mute.js", mute,
		"'/mute'", "'/unmute'", "'/api/reports/' + encodeURIComponent(", "'pinalert:updates-refresh'",
		"typeof CustomEvent", "method: 'POST'", "credentials: 'same-origin'",
		"'Content-Type': 'application/json'", "body: '{}'",
		"Pinalert.requestTimeout(MUTE_TIMEOUT_MS, 'mute')", "Promise.race(", "limit.expired")
	if n := strings.Count(mute, "fetch("); n != 1 {
		t.Errorf("mute.js makes %d fetch( calls, want 1", n)
	}

	// The control is built like Flag's: a button, born hidden, two listeners
	// that stop propagation and leave the browser's own activation alone.
	create := jsFunctionBody(t, mute, "createMuteControl(report)")
	p424Contains(t, "createMuteControl", create,
		"createElement('button')", "'mute-control'", "dataset.muteReportId", "el.hidden = true",
		"'inline-error'", "setAttribute('role', 'alert')", "'click'", "'keydown'", "'Enter'", "' '", "'Spacebar'",
		"var control = { el: el, error: error }", "return control;")
	if n := strings.Count(create, "stopPropagation"); n < 2 {
		t.Errorf("createMuteControl stops propagation %d times, want at least 2", n)
	}
	if n := strings.Count(create, "addEventListener("); n != 2 {
		t.Errorf("createMuteControl registers %d listeners, want exactly 2", n)
	}
	p424Lacks(t, "createMuteControl", create, "prevent"+"Default")

	// The update runs on every poll: it builds nothing, moves no focus and
	// reads no element by id.
	upd := jsFunctionBody(t, mute, "updateMuteControl(control, report)")
	p424Lacks(t, "updateMuteControl", upd, "createElement(", ".focus(", "getElementById(")
	p424Contains(t, "updateMuteControl", upd, "shouldShow(report)", "'aria-label'", "accessibleName(")

	// The controls for one report are found by a loop over the class, never by
	// a selector built from report data.
	every := jsFunctionBody(t, mute, "markMutedEverywhere(reportId, muted)")
	p424Contains(t, "markMutedEverywhere", every, "querySelectorAll('.mute-control')", "dataset.muteReportId")
	p424Lacks(t, "markMutedEverywhere", every, "[data-mute-report-id=")

	// Text goes in through Pinalert.setText or setAttribute only.
	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(", "new Function"} {
		if strings.Contains(mute, sink) {
			t.Errorf("mute.js holds the markup sink %q", sink)
		}
	}
	// Dashes: a test file may name them only as escapes, so this reads the raw
	// source of the script.
	p424NoDashes(t, "mute.js", raw)

	// The control reaches 44px through a rule of updates.css that names it.
	rules := updatesCSSRules(t)
	tokens := layoutLengthTokens(t)
	best, named := 0.0, 0
	for _, r := range rules {
		if strings.Contains(r.Media, "forced-colors") {
			continue
		}
		for _, part := range p16HeadParts(r.Head) {
			compound, ok := layoutLastCompound(part)
			if !ok || !touchRuleNames(compound, ".mute-control") {
				continue
			}
			named++
			for _, prop := range []string{"min-height", "height"} {
				if value, has := r.Decls[prop]; has {
					if px := touchValue(value, tokens); px > best {
						best = px
					}
				}
			}
		}
	}
	if named == 0 {
		t.Errorf(".mute-control: no rule of updates.css names the control")
	} else if best < 44 {
		t.Errorf(".mute-control: the largest height declared is %.0fpx, want 44px or more", best)
	}
}
