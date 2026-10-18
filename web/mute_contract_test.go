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

// TestMuteMountedBeforeFlag pins where the control joins the opened report
// (UI-SPEC Surface C): built once after the vote error and before the Flag
// control, kept as the card's mute part, and refreshed in place after the vote
// block and before the Flag control.
func TestMuteMountedBeforeFlag(t *testing.T) {
	mapjs := p424ReadJS(t, "static/js/map.js")
	build := jsFunctionBody(t, mapjs, "buildPopupCard(report)")
	p424Ordered(t, "buildPopupCard", build,
		"block.error", "PinalertMute.createMuteControl(report)", ".appendChild(muteControl.el)",
		".appendChild(muteControl.error)", "PinalertFlag.createFlagControl(report)",
		".appendChild(flagControl.el)", "mute: muteControl", "flag: flagControl", "updatePopupCard(card, report)")
	if n := strings.Count(mapjs, "PinalertMute.createMuteControl("); n != 1 {
		t.Errorf("map.js builds the Mute control %d times, want 1", n)
	}

	upd := jsFunctionBody(t, mapjs, "updatePopupCard(card, report)")
	voteAt := strings.Index(upd, "PinalertVotes.updateVoteBlock(card.votes, report)")
	muteAt := strings.Index(upd, "PinalertMute.updateMuteControl(card.mute, report)")
	flagAt := strings.Index(upd, "PinalertFlag.updateFlagControl(card.flag, report)")
	if voteAt == -1 || muteAt == -1 || flagAt == -1 || !(voteAt < muteAt && muteAt < flagAt) {
		t.Errorf("updatePopupCard must refresh Mute after the vote block and before Flag (vote %d, mute %d, flag %d)",
			voteAt, muteAt, flagAt)
	}
	p424Lacks(t, "updatePopupCard", upd, "createMuteControl(")

	// The list rows and the Past cards never build the control: it lives in the
	// opened report only, as Flag does.
	for _, name := range []string{"feed.js", "ended-cards.js", "activity.js"} {
		p424Lacks(t, name, p424ReadJS(t, "static/js/"+name), "PinalertMute")
	}
}

// TestMuteScriptLoadOrder pins the one script tag: on Nearby only, after flag.js
// and before map.js, deferred and versioned like its neighbours.
func TestMuteScriptLoadOrder(t *testing.T) {
	const tag = `<script src="/static/js/mute.js?v={{.AssetVersion}}" defer></script>`
	index := p424ReadTemplate(t, "index.html.tmpl")
	if n := strings.Count(index, tag); n != 1 {
		t.Errorf("index.html.tmpl loads mute.js %d times, want 1", n)
	}
	if n := strings.Count(index, "static/js/mute.js"); n != 1 {
		t.Errorf("index.html.tmpl names static/js/mute.js %d times, want 1", n)
	}
	p424Ordered(t, "index.html.tmpl", index, "/static/js/flag.js", "/static/js/mute.js", "/static/js/map.js")

	for name, text := range findFullPageTemplates(t) {
		page := strings.TrimPrefix(name, "templates/")
		if got := strings.Contains(text, "/static/js/mute.js"); got != (page == "index.html.tmpl") {
			t.Errorf("%s: loads mute.js = %v, want %v", page, got, page == "index.html.tmpl")
		}
	}
}

// TestVotesNudgeTheBell pins D-04 and UI-SPEC A1: a saved vote asks the bell to
// refresh once, on the success path only, with no loop and nothing else
// changed in the vote request.
func TestVotesNudgeTheBell(t *testing.T) {
	votes := p424ReadJS(t, "static/js/votes.js")
	cast := jsFunctionBody(t, votes, "castVote(reportId, action, opts)")
	if n := strings.Count(cast, "'pinalert:updates-refresh'"); n != 1 {
		t.Errorf("castVote holds the refresh event name %d times, want 1", n)
	}
	if n := strings.Count(votes, "pinalert:updates-refresh"); n != 1 {
		t.Errorf("votes.js holds the refresh event name %d times, want 1", n)
	}
	p424Ordered(t, "castVote", cast,
		"if (!res.ok) {", "throw err;", "typeof CustomEvent", "'pinalert:updates-refresh'", "return body;")
	for _, banned := range []string{"while (", "for (", "setInterval", "setTimeout"} {
		if strings.Contains(cast, banned) {
			t.Errorf("castVote must not contain %q", banned)
		}
	}
	p424Lacks(t, "votes.js", votes, "PinalertMute")
}

// TestMuteNotOnLowData pins UI-SPEC Scope: the Low data view has no bell, no
// mute and no tick.
func TestMuteNotOnLowData(t *testing.T) {
	p424Lacks(t, "low.js", p424ReadJS(t, "static/js/low.js"), "PinalertMute", "mute.js")
	p424Lacks(t, "low.html.tmpl", p424ReadTemplate(t, "low.html.tmpl"), "PinalertMute", "mute.js")
}
