package web

import (
	"regexp"
	"strings"
	"testing"
)

// Phase 8 plan 06 pins the Share window (COORD-08, D-04): the static markup
// share_window.html.tmpl, its two stylesheets and, in the second task, the
// script that fills it. Like the rest of this package these are static
// inspection of the embedded bytes. The node tests in web/share_window_test.mjs
// run the script, and the real look on a phone and a laptop is the owner's
// viewing (plan 08-20).

// p806TemplateComment matches one {{/* ... */}} template comment.
var p806TemplateComment = regexp.MustCompile(`(?s)\{\{/\*.*?\*/\}\}`)

// p806TextNodes returns every non empty text node of the markup, trimmed.
func p806TextNodes(markup string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`>([^<>]+)<`).FindAllStringSubmatch(markup, -1) {
		if text := strings.TrimSpace(m[1]); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// TestShareWindowMarkup pins the window of UI-SPEC Surface 5a: the dialog
// roles and ids, the card that shows only what a forward carries, the two
// buttons that start off, and the rule that the words which change are written
// by the script, not typed here, so no count or state word can sit in the
// template (D-04, T-08-26).
func TestShareWindowMarkup(t *testing.T) {
	raw := p13ReadTemplate(t, "share_window.html.tmpl")
	markup := p806TemplateComment.ReplaceAllString(raw, "")

	p13MustContain(t, "share_window.html.tmpl", markup,
		`id="share-window"`, `class="modal-backdrop share-window"`,
		`role="dialog"`, `aria-modal="true"`, `aria-labelledby="share-window-title"`,
		`<h2 id="share-window-title"`, `tabindex="-1"`, "Share this report",
		`aria-label="Close"`, `app-icon--x`,
		`class="share-card"`, `aria-label="What people see"`,
		`share-card__band`, `share-card__disc`, `share-card__brand`, "Pinalert",
		`id="share-card-title"`, `id="share-card-line"`, `id="share-card-link"`, `id="share-card-bar"`,
		`id="share-status"`, `role="status"`, `visually-hidden`,
		`share-window__error`, `id="share-window-retry"`, "Try again",
		`share-window__whatsapp`, "WhatsApp", `share-window__copy`, "Copy link")

	// The backdrop starts hidden, like the Flag dialog: share.js opens it.
	if !regexp.MustCompile(`<div class="modal-backdrop share-window" id="share-window" hidden>`).MatchString(markup) {
		t.Errorf("the Share backdrop must carry the hidden attribute in the markup")
	}

	// The WhatsApp link opens another origin in a new tab: noopener noreferrer
	// stops the new page reaching this one (T-08-28). It starts with no href
	// and off; share.js sets the href once the link is known.
	whatsapp := regexp.MustCompile(`(?s)<a [^>]*share-window__whatsapp[^>]*>`).FindString(markup)
	if whatsapp == "" {
		t.Fatalf("no WhatsApp link in the Share window markup")
	}
	p13MustContain(t, "WhatsApp link", whatsapp,
		`target="_blank"`, `rel="noopener noreferrer"`, `aria-disabled="true"`, `btn--off`)
	p13MustNotContain(t, "WhatsApp link", whatsapp, "href=")
	copyButton := regexp.MustCompile(`(?s)<button [^>]*share-window__copy[^>]*>`).FindString(markup)
	p13MustContain(t, "Copy link button", copyButton, `aria-disabled="true"`, `btn--off`)

	// The fixed line is written by the script from the server's own string, so
	// the template never types it, and no inline handler or style exists.
	p13MustNotContain(t, "share_window.html.tmpl", markup,
		"Open to see if this is still true right now", " onclick=", " onchange=", " style=", "innerHTML")

	// No count in any text node of the static window, and not the word
	// verified: a forward must never carry a stale number or a claim.
	for _, text := range p806TextNodes(markup) {
		if strings.ContainsAny(text, "0123456789") {
			t.Errorf("a text node of the Share window holds a digit: %q", text)
		}
		if strings.Contains(strings.ToLower(text), "verified") {
			t.Errorf("a text node of the Share window says verified: %q", text)
		}
	}
	if strings.ContainsFunc(raw, func(r rune) bool { return r == 0x2013 || r == 0x2014 }) {
		t.Errorf("share_window.html.tmpl holds an em or en dash")
	}
}

// TestShareWindowIsIncludedOnce pins where index.html.tmpl carries the window:
// the partial once, beside the Flag dialog, and the stylesheet after
// report-page.css and before phone.css (the sheet rules sit in phone.css and
// win a tie by loading last).
func TestShareWindowIsIncludedOnce(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")
	if n := strings.Count(index, `{{template "share_window.html.tmpl" .}}`); n != 1 {
		t.Fatalf("index.html.tmpl includes the Share window %d times, want 1", n)
	}
	if n := strings.Count(index, "/static/css/share.css"); n != 1 {
		t.Fatalf("index.html.tmpl links share.css %d times, want 1", n)
	}
	p13MustContain(t, "share.css tag", findTagWindow(t, index, "/static/css/share.css"), "?v={{.AssetVersion}}")
	p13Ordered(t, index, "index.html.tmpl",
		`{{template "flag_dialog.html.tmpl" .}}`, `{{template "share_window.html.tmpl" .}}`)
	p13Ordered(t, index, "index.html.tmpl styles",
		"/static/css/report-page.css", "/static/css/share.css", "/static/css/phone.css")
}

// TestShareWindowHiddenGuard pins the hidden guard. Every rule in share.css
// and phone.css that sets display on the Share window or one of its parts
// carries :not([hidden]) in each selector part, so the hidden attribute still
// hides it. The browser's own [hidden] rule loses to an author display rule of
// equal weight, the failure that shipped Phase 1's backdrop open on load.
func TestShareWindowHiddenGuard(t *testing.T) {
	seen := 0
	for _, file := range []string{"static/css/share.css", "static/css/phone.css"} {
		for _, rule := range p16ParseCSS(file, p16ReadStatic(t, file)) {
			if strings.HasPrefix(rule.Media, "@keyframes") {
				continue
			}
			if _, sets := rule.Decls["display"]; !sets {
				continue
			}
			for _, part := range p16HeadParts(rule.Head) {
				if !strings.Contains(part, ".share-window") && !strings.Contains(part, ".share-card") {
					continue
				}
				seen++
				if !strings.Contains(part, ":not([hidden])") {
					t.Errorf("%s: %q sets display without a :not([hidden]) guard", file, part)
				}
			}
		}
	}
	if seen < 6 {
		t.Errorf("only %d display rules on the Share window were seen; the walk may have stopped matching", seen)
	}
}

// TestShareWindowCardIsNeutral pins D-04 in the stylesheet: the card carries
// no severity colour and no opacity, because a forward shows no state.
func TestShareWindowCardIsNeutral(t *testing.T) {
	css := stripCSSComments(p16ReadStatic(t, "static/css/share.css"))
	p13MustNotContain(t, "share.css", css, "--color-severity", "--color-tone", "opacity")
}

// TestShareScriptIsLinkedInOrder pins where index.html.tmpl loads share.js: once,
// deferred, right after report-page.js and before map.js, so window.PinalertShare
// exists before the page builds a Share button (the button stays hidden until it
// does, plan 08-05).
func TestShareScriptIsLinkedInOrder(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")
	if n := strings.Count(index, "/static/js/share.js"); n != 1 {
		t.Fatalf("index.html.tmpl loads share.js %d times, want 1", n)
	}
	p13MustContain(t, "share.js tag", findTagWindow(t, index, "/static/js/share.js"), " defer", "?v={{.AssetVersion}}")
	p13Ordered(t, index, "index.html.tmpl scripts",
		"/static/js/app.js", "/static/js/report-page.js", "/static/js/share.js", "/static/js/map.js")
}

// TestShareScriptContract pins the rules of share.js that the node tests cannot
// see in the source: the module name and the call the page makes, the closed
// set of words, the wa.me chooser link, and the rules of the owner's windows (no
// backdrop close, a paired page lock). The browser's own share sheet is never
// called anywhere in the script (research Pitfall 8: a share after an await is
// refused by Safari, and the plan's default is the chooser link).
func TestShareScriptContract(t *testing.T) {
	raw := p13ReadStatic(t, "static/js/share.js")
	code := p13CodeLines(raw)

	p13MustContain(t, "share.js", code,
		"'use strict'", "window.PinalertShare", "open: open",
		"SHARE_TIMEOUT_MS = 10000", "COPIED_MS = 3000",
		"Pinalert.requestTimeout(SHARE_TIMEOUT_MS", "Pinalert.lockPage(", "Pinalert.unlockPage(",
		"Pinalert.iconClass(", "Pinalert.categoryLabel(", "Pinalert.setText(",
		"'https://wa.me/?text='", "encodeURIComponent(title + ' ' + body.url)",
		"/share'", "method: 'POST'", "credentials: 'same-origin'",
		"'Open to see if this is still true right now'",
		"'Link copied'", "'Link copied.'", "\"Couldn't copy. Select the link and copy it.\"",
		"\"Couldn't make a link. Try again.\"",
		"\"You're offline, so there is no link yet. Try again when you have a signal.\"",
		"'This report is no longer available.'", "'Copy link'",
		"navigator.clipboard.writeText(")
	p13MustNotContain(t, "share.js", code,
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "DOMParser",
		"navigator.share", "navigator.canShare", "execCommand", "window.open(", "location.href", "location.assign")
	if regexp.MustCompile(`(?i)verified`).MatchString(code) {
		t.Errorf("share.js names the word verified on a code line; a forward carries no claim (D-04)")
	}
	// A tap on the backdrop must not close the window (03.4 R2, D-14): the only
	// click listeners are on the X, Try again, Copy link and the WhatsApp link.
	if n := strings.Count(code, "backdrop.addEventListener('click'"); n != 0 {
		t.Errorf("share.js binds a click listener on the backdrop %d times, want 0", n)
	}
	p13Ordered(t, jsFunctionBody(t, code, "closeWindow()"), "closeWindow",
		"refs.backdrop.hidden = true", "Pinalert.unlockPage(", "returnFocus(state)")
	p13MustContain(t, "onWindowKeydown", jsFunctionBody(t, code, "onWindowKeydown(evt)"),
		"'Escape'", "closeWindow(", "'Tab'", "focusableInPanel(")
	if strings.ContainsFunc(raw, func(r rune) bool { return r == 0x2013 || r == 0x2014 }) {
		t.Errorf("share.js holds an em or en dash")
	}
}
