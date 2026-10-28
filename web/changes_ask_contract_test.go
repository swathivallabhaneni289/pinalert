package web

import (
	"strings"
	"testing"
)

// Phase 3.3 plan 20 (the 03.3-UAT gap for test 4, owner 2026-10-27): the
// pop-up that asks "Tell you when this report changes?" the first time a person
// posts or votes. These checks pin its partial and its script. The behaviour is
// pinned by changes_ask_test.mjs. Plan 21 renders the partial on Nearby and Low
// data, which is when the pop-up goes live.

// TestChangesAskPartialContract pins the markup: the Log out box, copied, with
// the three words of the owner and nothing else.
func TestChangesAskPartialContract(t *testing.T) {
	src := templateCopy(stripReadTemplate(t, "changes_ask.html.tmpl"))

	w10Count(t, "changes_ask.html.tmpl", src, `{{define "changes_ask"}}`, 1)
	w10Count(t, "changes_ask.html.tmpl", src, `<dialog id="changes-ask" class="logout-confirm" aria-labelledby="changes-ask-title">`, 1)
	w10Count(t, "changes_ask.html.tmpl", src, "<dialog", 1)
	w10Count(t, "changes_ask.html.tmpl", src, "</dialog>", 1)
	w10Count(t, "changes_ask.html.tmpl", src, `<div class="logout-confirm__body">`, 1)
	w10Count(t, "changes_ask.html.tmpl", src,
		`<h2 class="logout-confirm__title" id="changes-ask-title" tabindex="-1">Tell you when this report changes?</h2>`, 1)
	w10Count(t, "changes_ask.html.tmpl", src, `<div class="logout-confirm__actions">`, 1)

	// Not now first, Allow second, and exactly these two buttons.
	p421InOrder(t, "changes_ask.html.tmpl", src,
		`<button type="button" class="btn" id="changes-ask-later">Not now</button>`,
		`<button type="button" class="btn btn--primary" id="changes-ask-allow">Allow</button>`)
	w10Count(t, "changes_ask.html.tmpl", src, "<button", 2)

	// No autofocus (no button looks chosen), no supporting line, no link, no
	// field: the pop-up holds a question and two buttons.
	for _, banned := range []string{"autofocus", "<p", "<a ", "<a>", "<input", "<form", "<label"} {
		if strings.Contains(src, banned) {
			t.Errorf("changes_ask.html.tmpl holds %q", banned)
		}
	}
	for _, dash := range []string{"—", "–"} {
		if strings.Contains(stripReadTemplate(t, "changes_ask.html.tmpl"), dash) {
			t.Errorf("changes_ask.html.tmpl holds the dash character %q", dash)
		}
	}
}

// TestChangesAskScriptContract pins what the script may and may not do: one
// enable call (the Allow tap), one address, one switch, the once per account
// key, and no markup sink.
func TestChangesAskScriptContract(t *testing.T) {
	raw := p421ReadStatic(t, "js/changes-ask.js")
	code := stripJSComments(raw)

	w10Count(t, "changes-ask.js", code, "PinalertNotify.enable(", 1)
	for _, banned := range []string{"requestPermission", "deviceState", "confirm("} {
		if strings.Contains(code, banned) {
			t.Errorf("changes-ask.js holds %q", banned)
		}
	}

	// The address and the one switch it turns on are each written once in the
	// whole file, comments included, so a comment cannot hide a second use.
	w10Count(t, "changes-ask.js", raw, "/api/me/alerts", 1)
	w10Count(t, "changes-ask.js", raw, "changes_push", 1)
	w10Count(t, "changes-ask.js", raw, "reliable_alerts", 0)

	p13MustContain(t, "changes-ask.js", code, "pinalert.changesAsked", "dialog[open]", "isMacDockApp",
		"window.PinalertChangesAsk = {")

	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(", "new Function", "DOMParser"} {
		if strings.Contains(code, sink) {
			t.Errorf("changes-ask.js uses the markup parsing sink %q", sink)
		}
	}

	for _, dash := range []string{"—", "–"} {
		if strings.Contains(raw, dash) {
			t.Errorf("changes-ask.js holds the dash character %q", dash)
		}
	}
}
