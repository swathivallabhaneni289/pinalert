package web

// Phase 9 plan 08: the profile page. These tests read the shipped template,
// stylesheet and script from the embedded trees, never a copy, so they can only
// drift with the product (UI-SPEC Surface 1, Surface 6 and the Copywriting
// Contract rows P9-C14 and P9-C15).

import (
	"os"
	"strings"
	"testing"
)

// TestPersonPageHeaderOrder pins D-13 and UI-SPEC 1b: the header holds the
// round letter, the username as the h1, the Name, the stars, the tag chip and
// its sentence, Member since, the home area, then the Followers and Following
// buttons and the actions, in that order.
func TestPersonPageHeaderOrder(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	p421InOrder(t, "the profile header order", tmpl,
		`id="person-header"`,
		`id="person-disc"`,
		`<h1 id="person-username"`,
		`id="person-name"`,
		`id="person-stars"`,
		`id="person-tag"`,
		`id="person-tag-sentence"`,
		`id="person-member-since"`,
		`id="person-home-area"`,
		`id="person-followers-button"`,
		`id="person-following-button"`,
		`id="person-actions"`,
		`id="person-edit"`,
		`id="person-state-line"`,
		`id="person-follow"`,
		`id="person-unfollow"`,
		`id="person-more"`,
		`id="person-more-menu"`,
		`id="person-action-error"`,
		`id="person-reports-section"`,
		`id="person-blocked"`,
		`id="person-confirm"`,
	)
	if strings.Count(tmpl, "<h1") != 1 {
		t.Errorf("the page has %d h1 elements, want exactly one (the username)", strings.Count(tmpl, "<h1"))
	}
	for _, want := range []string{`class="logout-confirm"`, `id="person-not-found"`, `data-username=`, `data-self=`} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("person.html.tmpl lacks %s", want)
		}
	}
	// The stars are drawn in the text colour and never coloured (UI-SPEC Color).
	css := p421ReadStatic(t, "css/person.css")
	if !strings.Contains(css, `url("/static/icons/star-filled.svg")`) {
		t.Errorf("person.css does not mask star-filled.svg with the literal url")
	}
}

// TestPersonPageNoindexAndFrame pins T-09-31 and UI-SPEC 1a: the noindex meta,
// the two app bars, the page's own stylesheet and script linked with the asset
// version, and the live status line.
func TestPersonPageNoindexAndFrame(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	if strings.Count(tmpl, `name="robots" content="noindex"`) != 1 {
		t.Errorf("person.html.tmpl must carry the noindex meta exactly once")
	}
	for _, want := range []string{
		`{{template "app_topbar" .}}`, `{{template "app_bottombar" .}}`,
		`href="/static/css/person.css?v={{.AssetVersion}}"`,
		`src="/static/js/person.js?v={{.AssetVersion}}"`,
		`id="notice-live"`, `aria-live="polite"`,
		`<title>{{.PageTitle}}</title>`,
	} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("person.html.tmpl lacks %s", want)
		}
	}
	// A profile is not a main place: no bar item is marked current, and the
	// phone bar keeps its gear, so the handler must not use the Settings key.
	raw, err := os.ReadFile("../internal/api/handlers/person_page.go")
	if err != nil {
		t.Fatalf("reading person_page.go: %v", err)
	}
	handler := string(raw)
	if strings.Contains(handler, "NavSettings") {
		t.Errorf("person_page.go uses NavSettings, which hides the gear (UI-SPEC 1a)")
	}
	if !strings.Contains(handler, `"My profile"`) || !strings.Contains(handler, `"Profile"`) {
		t.Errorf("person_page.go lacks the two phone top bar titles")
	}
}

// TestPersonPageCopy pins the Copywriting Contract rows P9-C14 and P9-C15, the
// action words and the section title and line, verbatim, in the template and in
// the script.
func TestPersonPageCopy(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	for _, want := range []string{
		// P9-C14 Not found.
		"Profile not found",
		"There is no public profile with this username. It may have been made private, or the username may have changed.",
		"Go to Nearby",
		// P9-C15 Blocked by you.
		"You blocked ",
		"They can't see your profile or follow you. Their reports still show in the feed.",
		"Unblock",
		// P9-C6, P9-C7 the buttons and the menu.
		"Follow", "Following", "More", "Edit in Settings", "Remove follower", "Block",
		// Surface 5, the part of the section this plan draws.
		"Recent reports", "Reports from the last 30 days, newest first.",
		"stars",
	} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("person.html.tmpl lacks the sentence %q", want)
		}
	}
	script := p421ReadStatic(t, "js/person.js")
	for _, want := range []string{
		// P9-C22 the two confirm boxes.
		"' as a follower?'", "They stop following you. They can follow you again.",
		"They can no longer see your profile or follow you, and you stop following each other.",
		"Their reports still show in the feed. You will not see their comments.",
		// P9-C8 the status lines.
		"'You followed '", "'You unfollowed '", "' no longer follows you.'", "' is unblocked.'",
		// P9-C13 the action errors.
		"Couldn\\'t follow. Try again.", "Couldn\\'t unfollow. Try again.",
		"Couldn\\'t remove this follower. Try again.", "Couldn\\'t block. Try again.",
		"Couldn\\'t unblock. Try again.",
		"You\\'re offline, so this wasn\\'t saved. Try again when you have a signal.",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("person.js lacks the sentence %q", want)
		}
	}
	// The script holds no markup sink (T-09-29).
	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(script, sink) {
			t.Errorf("person.js uses the markup sink %s", sink)
		}
	}
}
