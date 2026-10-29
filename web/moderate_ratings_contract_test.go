package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// This file pins the Ratings section of the moderator screen (Phase 9 plan 15,
// D-14, UI-SPEC P9-C30, review W12): the template holds the section, its field
// and the Remove all box built like the Settings Delete box; moderate.js reads
// one reporter's ratings and posts the two removals, drawing every name and
// number through Pinalert.setText; and no other file of the app reaches the
// rater routes. Every identifier here starts with p915 so it cannot collide
// with another plan's helpers.
//
// Honest limit of this test's claim: static inspection proves the shapes in the
// source; it cannot prove how the box looks or that its first focus lands where
// the code says in a real phone browser. web/moderate_ratings_test.mjs runs the
// script against a fake page for the behaviour.

// p915Count is how many times s occurs in body.
func p915Count(body, s string) int { return strings.Count(body, s) }

// TestModeratorRatingsSection is the section, the box and the script.
func TestModeratorRatingsSection(t *testing.T) {
	tmpl := p426ReadTemplate(t)
	js := p426ReadJS(t)

	// The section and its ids, once each.
	for _, id := range []string{
		"mod-h-ratings", "mod-ratings-form", "mod-ratings-name", "mod-ratings-error", "mod-ratings-status",
		"mod-ratings-list", "mod-ratings-confirm", "mod-ratings-confirm-title", "mod-ratings-confirm-text",
		"mod-ratings-confirm-error", "mod-ratings-confirm-cancel", "mod-ratings-confirm-go",
	} {
		if n := p915Count(tmpl, `id="`+id+`"`); n != 1 {
			t.Errorf(`id="%s" occurs %d times, want 1`, id, n)
		}
	}
	p426WantAll(t, "section", tmpl, `<section class="mod-section" aria-labelledby="mod-h-ratings">`)
	p426InOrder(t, "sections", tmpl, `id="mod-h-comments"`, `id="mod-h-ratings"`, `id="mod-ratings-confirm"`, "</main>")
	list := findTagWindow(t, tmpl, `id="mod-ratings-list"`)
	if !strings.HasPrefix(list, "<ol") {
		t.Errorf("mod-ratings-list must be an ol, got %q", list)
	}
	p426WantAll(t, "ratings list", list, `class="mod-list"`, `role="list"`, `aria-labelledby="mod-h-ratings"`)
	p426WantAll(t, "form", findTagWindow(t, tmpl, `id="mod-ratings-form"`), "<form", `class="mod-find"`)
	p426WantAll(t, "field", findTagWindow(t, tmpl, `id="mod-ratings-name"`), `type="text"`, `autocomplete="off"`,
		`autocapitalize="off"`, `spellcheck="false"`)
	p426WantAll(t, "label", tmpl, `<label for="mod-ratings-name">`)
	p426WantAll(t, "submit", findTagWindow(t, tmpl, ">Show ratings<"), `type="submit"`, `class="btn btn--primary"`)
	p426WantAll(t, "error line", findTagWindow(t, tmpl, `id="mod-ratings-error"`), `class="inline-error"`, `role="alert"`, "hidden")
	p426WantAll(t, "status region", findTagWindow(t, tmpl, `id="mod-ratings-status"`), `aria-live="polite"`)
	p426WantNone(t, "status region", findTagWindow(t, tmpl, `id="mod-ratings-status"`), "role=")

	// Every string the plan names appears once across the template and the script.
	for _, s := range []string{
		"Ratings", "Find one reporter's tapped ratings. Removed ratings stop counting at once.",
		"Reporter's username", "Show ratings",
		"Their ratings stop counting for every reporter, and they cannot rate again.", "Cancel", "Remove all",
	} {
		if n := p426CountTemplateText(tmpl, s); n != 1 {
			t.Errorf("%q occurs %d times in the template as the text of an element, want 1", s, n)
		}
	}
	for _, s := range []string{
		"No ratings for this person.", "Couldn't load ratings. Try again.", "Removed", "Cleared by the rater", "Remove",
	} {
		if n := p426CountQuoted(js, s); n != 1 {
			t.Errorf("%q occurs %d times as a string of moderate.js, want 1", s, n)
		}
	}
	p426WantAll(t, "composed fragments", js, `' gave '`, `' star'`, `' stars'`, `'Remove all by '`, `'Remove all ratings by '`,
		`'Removed '`, `' rating'`, `' ratings'`, `' by '`, `"Enter the reporter's username."`)

	// The box is built like #settings-delete-confirm: a native dialog of the
	// logout-confirm class, a title that takes the first focus (tabindex -1), the
	// sentence, an inline error line, then Cancel and the destructive Remove all.
	dialog := findTagWindow(t, tmpl, `id="mod-ratings-confirm"`)
	p426WantAll(t, "dialog", dialog, "<dialog", `class="logout-confirm"`, `aria-labelledby="mod-ratings-confirm-title"`,
		`aria-describedby="mod-ratings-confirm-text"`)
	p426WantNone(t, "dialog", dialog, "autofocus", "open", "onclick", "onkeydown")
	title := findTagWindow(t, tmpl, `id="mod-ratings-confirm-title"`)
	p426WantAll(t, "box title", title, "<h2", `class="logout-confirm__title"`, `tabindex="-1"`)
	p426WantAll(t, "box error", findTagWindow(t, tmpl, `id="mod-ratings-confirm-error"`), `class="inline-error"`, `role="alert"`, "hidden")
	cancel := findTagWindow(t, tmpl, `id="mod-ratings-confirm-cancel"`)
	p426WantAll(t, "Cancel", cancel, `type="button"`, `class="btn"`, `data-mod-rating-action="box-cancel"`)
	p426WantNone(t, "Cancel", cancel, "btn--destructive", "autofocus")
	remove := findTagWindow(t, tmpl, `id="mod-ratings-confirm-go"`)
	p426WantAll(t, "Remove all", remove, `type="button"`, `class="btn btn--destructive"`, `data-mod-rating-action="box-go"`)
	p426InOrder(t, "box", tmpl, `id="mod-ratings-confirm-title"`, `id="mod-ratings-confirm-text"`, `id="mod-ratings-confirm-error"`,
		`id="mod-ratings-confirm-cancel"`, `id="mod-ratings-confirm-go"`)
	// No X: the box holds exactly its two buttons and nothing named close.
	boxStart := strings.Index(tmpl, `id="mod-ratings-confirm"`)
	box := tmpl[boxStart : boxStart+strings.Index(tmpl[boxStart:], "</dialog>")]
	if n := strings.Count(box, "<button"); n != 2 {
		t.Errorf("the box holds %d buttons, want exactly 2 (Cancel and Remove all)", n)
	}
	p426WantNone(t, "box", strings.ToLower(box), "close", "&times;", "aria-label=")

	// The script: rows through setText only, the two routes, the box asked first.
	p426WantNone(t, "moderate.js", js, "textContent", "innerHTML", "createTextNode", "insertAdjacentHTML")
	ratingFns := strings.Join([]string{
		jsFunctionBody(t, js, "ratingsStatus(text)"), jsFunctionBody(t, js, "ratingsError(text)"),
		jsFunctionBody(t, js, "buildRatingRow(rating, index)"), jsFunctionBody(t, js, "renderRatings(ratings)"),
		jsFunctionBody(t, js, "loadRatings(name)"), jsFunctionBody(t, js, "removeOneRating(rater, rowEl)"),
		jsFunctionBody(t, js, "openRatingsBox(rater)"), jsFunctionBody(t, js, "sendRemoveAll()"),
		jsFunctionBody(t, js, "onRatingButton(button)"),
	}, "\n")
	p426WantNone(t, "ratings code", ratingFns, "'href'", "createElement('a')", "makeEl('a'", "'a'", "title =", "alert(")
	row := jsFunctionBody(t, js, "buildRatingRow(rating, index)")
	p426WantAll(t, "buildRatingRow", row, "Pinalert.setText(name, ratingStarsLine(rater, stars))", "RATING_REMOVED_WORD",
		"RATING_CLEARED_WORD", "Pinalert.relativeTime(rating.updated_at)", "makeRatingButton('remove', RATING_REMOVE_LABEL",
		"makeRatingButton('remove-all', allLabel, allLabel)", "'Remove all by '")
	if i, j := strings.Index(row, "if (!removed && !cleared)"), strings.Index(row, "makeRatingButton('remove'"); i < 0 || j < i {
		t.Errorf("the Remove buttons must be built only for a live rating (indexes %d and %d)", i, j)
	}
	p426WantAll(t, "makeRatingButton", jsFunctionBody(t, js, "makeRatingButton(action, label, ariaLabel)"),
		"makeEl('button', 'btn')", "'data-mod-rating-action'", "'aria-label'")
	p426WantAll(t, "load", jsFunctionBody(t, js, "loadRatings(name)"), "RATINGS_URL + encodeURIComponent(name) + '/ratings'",
		"RATINGS_EMPTY_MESSAGE", "RATINGS_FAILED_MESSAGE", "RATINGS_NO_PERSON_MESSAGE", "res.status === 404")
	p426WantAll(t, "removeOneRating", jsFunctionBody(t, js, "removeOneRating(rater, rowEl)"),
		"'/ratings/remove'", "apiRequest('POST', path, 0, { rater: rater })", "ratingsState.busy")
	p426WantAll(t, "sendRemoveAll", jsFunctionBody(t, js, "sendRemoveAll()"),
		"RATERS_URL + encodeURIComponent(rater) + '/remove-ratings'", "apiRequest('POST', ", ", 0)", "res.body.removed",
		"removedRatingsLine(removed, rater)", "ratingsState.busy")
	p426WantAll(t, "constants", js, "var RATINGS_URL = '/api/moderation/people/'", "var RATERS_URL = '/api/moderation/raters/'")

	// Only the box's own Remove all branch sends the removal of everything one
	// rater gave. The row button opens the box, and the fallback of a browser with
	// no native box asks with confirm first.
	pick := jsFunctionBody(t, js, "onRatingButton(button)")
	w, okA, okT := windowAfter(pick, "action === 'box-go'", "}")
	if !okA || !okT {
		t.Fatalf("the box-go branch was not found (anchor %v, terminator %v)", okA, okT)
	}
	p426WantAll(t, "box-go branch", w, "sendRemoveAll()")
	w, okA, okT = windowAfter(pick, "action === 'remove-all'", "}")
	if !okA || !okT {
		t.Fatalf("the remove-all branch was not found (anchor %v, terminator %v)", okA, okT)
	}
	p426WantAll(t, "remove-all branch", w, "openRatingsBox(rater)")
	p426WantNone(t, "remove-all branch", w, "sendRemoveAll(", "apiRequest(")
	if n := p915Count(js, "sendRemoveAll()"); n != 3 {
		t.Errorf("sendRemoveAll() occurs %d times (its definition, the box button and the confirm fallback), want 3", n)
	}
	open := jsFunctionBody(t, js, "openRatingsBox(rater)")
	p426WantAll(t, "openRatingsBox", open, "ratingsUi.box.showModal()", "ratingsUi.boxTitle.focus()",
		"'Remove all ratings by ' + rater + '?'", "window.confirm(question)")
	p426InOrder(t, "openRatingsBox", open, "ratingsUi.box.showModal()", "ratingsUi.boxTitle.focus()")
	p426WantNone(t, "openRatingsBox", open, "boxCancel.focus", "boxGo.focus", "autofocus")
	p426WantAll(t, "onRatingsBoxCancel", jsFunctionBody(t, js, "onRatingsBoxCancel(evt)"), "ratingsState.busy", "evt.preventDefault()")
	// The page behind a native box stays still and a tap on the dim page closes
	// nothing: the box has no click listener of its own.
	p426WantAll(t, "listeners", js, "ratingsUi.form.addEventListener('submit', onRatingsSubmit)",
		"ratingsUi.box.addEventListener('cancel', onRatingsBoxCancel)")
	p426WantNone(t, "listeners", js, "ratingsUi.box.addEventListener('click'", "ratingsUi.box.onclick")

	// Disabled buttons use aria-disabled with the shared .btn--off look.
	p426WantAll(t, "setButtonOff", jsFunctionBody(t, js, "setButtonOff(button, off)"), "'aria-disabled'", "'btn--off'")
	p426WantAll(t, "onRootClick", jsFunctionBody(t, js, "onRootClick(evt)"), "button[data-mod-rating-action]", "onRatingButton(")
	// The two request routes hang under the moderator group only.
	p426WantAll(t, "payload", jsFunctionBody(t, js, "apiRequest(method, path, timeoutMs, payload)"),
		"payload !== undefined", "JSON.stringify(payload)", "'Content-Type'")
	if n := strings.Count(js, "fetch("); n != 1 {
		t.Errorf("fetch( occurs %d times, want exactly 1", n)
	}

	// The stylesheet spaces the section and sets nothing hidden-sensitive without the guard.
	css := p426ReadCSS(t)
	p426WantNone(t, "moderate.css", css, "!important")
	rules := parseCSSRules(css)
	for _, sel := range []string{"#mod-ratings-error", "#mod-ratings-status", "#mod-ratings-list:not([hidden])"} {
		if _, ok := p426RuleWithSelector(rules, sel); !ok {
			t.Errorf("moderate.css has no rule for %q", sel)
		}
	}

	// Touch size: every button of the section takes its height from .btn, which
	// reads --touch-target-min, and the rows' own buttons are built as .btn.
	main, err := StaticFS.ReadFile("static/css/main.css")
	if err != nil {
		t.Fatalf("reading main.css: %v", err)
	}
	btn, ok := ruleBySelector(parseCSSRules(string(main)), ".btn")
	if !ok || declsOf(btn.declBody)["min-height"] != "var(--touch-target-min)" {
		t.Errorf(".btn min-height = %q, want var(--touch-target-min)", declsOf(btn.declBody)["min-height"])
	}
	chrome, err := StaticFS.ReadFile("static/css/chrome.css")
	if err != nil {
		t.Fatalf("reading chrome.css: %v", err)
	}
	if !regexp.MustCompile(`\.btn\.btn--off\[aria-disabled="true"\]`).Match(chrome) {
		t.Error("chrome.css has no .btn--off rule for an aria-disabled button")
	}
}

// TestModeratorRatingsNamesStayOnTheModeratorScreen is T-09-57 on the client:
// no script or template other than the moderator screen reaches the rater
// routes, so no other screen can show who rated whom.
func TestModeratorRatingsNamesStayOnTheModeratorScreen(t *testing.T) {
	found := map[string]bool{}
	scan := func(fsys fs.FS, root string) {
		err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			raw, err := fs.ReadFile(fsys, path)
			if err != nil {
				return err
			}
			if strings.Contains(string(raw), "/api/moderation/people/") || strings.Contains(string(raw), "/api/moderation/raters/") {
				found[path] = true
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	scan(StaticFS, "static")
	scan(TemplatesFS, "templates")
	if len(found) != 1 || !found["static/js/moderate.js"] {
		t.Errorf("files that reach the rater routes = %v, want only static/js/moderate.js", found)
	}
}
