package web

// review_ask_buttons_contract_test.go pins quick 261118-uoe (owner 2026-11-18,
// labels "not just clumped up together"): in the author's box for a removed
// report, Close and Ask for review stack full width at every width, the action
// on top, with the Log out box's own gap, so neither label wraps inside its
// button. The rule is scoped to this box: the Log out box and every other box
// built on .logout-confirm keep their row of equal buttons.

import (
	"regexp"
	"strings"
	"testing"
)

// uoeRuleBody returns the declarations of the one rule in css whose selector
// is exactly selector, and fails when there is not exactly one.
func uoeRuleBody(t *testing.T, file, css, selector string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(selector) + ` \{([^}]*)\}`)
	found := re.FindAllStringSubmatch(css, -1)
	if len(found) != 1 {
		t.Fatalf("%s has %d rules for %q, want 1", file, len(found), selector)
	}
	return found[0][1]
}

func TestReviewAskButtonsStack(t *testing.T) {
	css := p421ReadStatic(t, "css/updates.css")
	row := uoeRuleBody(t, "updates.css", css, ".review-ask .logout-confirm__actions:not([hidden])")
	for _, want := range []string{"flex-direction: column;", "flex-wrap: nowrap;"} {
		if !strings.Contains(row, want) {
			t.Errorf("the box's button row lacks %q", want)
		}
	}
	if strings.Contains(row, "gap") {
		t.Error("the box's button row sets its own gap; it must keep the Log out box's gap")
	}
	btn := uoeRuleBody(t, "updates.css", css, ".review-ask .logout-confirm__actions .btn")
	if !strings.Contains(btn, "flex: none;") {
		t.Error("the box's buttons lack flex: none, so the row's equal halves basis would size their height")
	}

	// Only this box: updates.css has no unscoped rule for the shared row or
	// its buttons, and auth.css keeps the Log out box's row and equal halves.
	for _, line := range strings.Split(css, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ".logout-confirm__actions") {
			t.Errorf("updates.css has an unscoped rule for the shared button row: %q", trimmed)
		}
	}
	auth := p421ReadStatic(t, "css/auth.css")
	shared := uoeRuleBody(t, "auth.css", auth, ".logout-confirm__actions:not([hidden])")
	for _, want := range []string{"display: flex;", "flex-wrap: wrap;", "gap: var(--space-sm);"} {
		if !strings.Contains(shared, want) {
			t.Errorf("auth.css's shared button row lacks %q", want)
		}
	}
	if strings.Contains(shared, "column") {
		t.Error("auth.css's shared button row stacks; only the review ask box may")
	}
	if !strings.Contains(uoeRuleBody(t, "auth.css", auth, ".logout-confirm__actions .btn"), "flex: 1 1 0;") {
		t.Error("auth.css's shared buttons are no longer equal halves")
	}

	// The words are unchanged. Amended 2026-11-18: Ask for review comes first in
	// the markup as on screen, so keyboard order matches the stack.
	box := p421ReadTemplate(t, "review_ask_box.html.tmpl")
	p421InOrder(t, "the box buttons", box,
		`id="review-ask-send" hidden>Ask for review</button>`,
		`id="review-ask-close">Close</button>`)

	for _, path := range []string{"static/css/updates.css", "review_ask_buttons_contract_test.go"} {
		text := rzReadRepo(t, path)
		for _, dash := range []string{string(rune(0x2013)), string(rune(0x2014))} {
			if strings.Contains(text, dash) {
				t.Errorf("%s holds a dash %q", path, dash)
			}
		}
	}
}
