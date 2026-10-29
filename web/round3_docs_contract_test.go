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
