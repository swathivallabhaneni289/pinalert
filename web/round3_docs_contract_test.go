package web

// Phase 9 round 3 (2026-10-30): pins for the doc lines the skeptic fixes changed,
// so a doc line cannot drift from the code it describes.

import (
	"os"
	"strings"
	"testing"
)

// TestTrustModelSaysARefusedCursorStartsTheListAgain pins SK-W02: the doc says
// what people-list.js does, and the script does it.
func TestTrustModelSaysARefusedCursorStartsTheListAgain(t *testing.T) {
	trust := round2Doc(t, "TRUST-MODEL.md")
	for _, want := range []string{
		"the list window reads the list again from the top (an empty cursor) and shows the server's sentence once",
		"it never sends the dead cursor a second time",
	} {
		if !strings.Contains(trust, want) {
			t.Errorf("docs/TRUST-MODEL.md lacks %q", want)
		}
	}
	script, err := os.ReadFile("static/js/people-list.js")
	if err != nil {
		t.Fatalf("reading people-list.js: %v", err)
	}
	if !strings.Contains(string(script), "startAgain(answer)") || !strings.Contains(string(script), "load('')") {
		t.Error("people-list.js no longer starts the list again on a refused cursor")
	}
}

// TestModerationDocSaysWhatTheVisibilityBucketCharges pins SK-I02: the doc says
// the bucket charges every request that asks for on, as the code does, and no
// longer says it charges only a real off to on turn.
func TestModerationDocSaysWhatTheVisibilityBucketCharges(t *testing.T) {
	doc := round2Doc(t, "MODERATION.md")
	for _, want := range []string{
		"takes a token for every request that asks for on, including a request that finds the profile already on",
		"it still spends a token",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("docs/MODERATION.md lacks %q", want)
		}
	}
	if strings.Contains(doc, "takes a token only when the profile goes from off to on") {
		t.Error("docs/MODERATION.md still says the Visibility bucket charges only a real off to on turn")
	}
}

// TestTrustModelSaysThePeopleCursorIsSealedAndBound pins SK-W03.
func TestTrustModelSaysThePeopleCursorIsSealedAndBound(t *testing.T) {
	trust := round2Doc(t, "TRUST-MODEL.md")
	for _, want := range []string{
		"The paging cursor is sealed by the server (AES-GCM under a random per process key, with a random nonce each time)",
		"the follow time inside it cannot be read",
		"a cursor issued on one list is refused on any other with the same calm sentence",
	} {
		if !strings.Contains(trust, want) {
			t.Errorf("docs/TRUST-MODEL.md lacks %q", want)
		}
	}
}
