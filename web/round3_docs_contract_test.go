package web

// Phase 9 round 3 (2026-10-30): pins for the doc lines the skeptic fixes changed,
// so a doc line cannot drift from the code it describes.

import (
	"strings"
	"testing"
)

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
