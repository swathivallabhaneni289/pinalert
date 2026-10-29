package web

// Phase 8 review fixes, round 2 (08-REVIEW-FIX-SKEPTIC.md): pins for the
// written words the skeptic pass found untrue.

import (
	"strings"
	"testing"
)

// TestTrustModelSharingPageFields (SK-02): the public page shows the What is
// happening pick for a Critical report (share.go sets Happening), and its one
// sentence names how many people confirmed the report (ConfirmedSentence), so
// TRUST-MODEL's Sharing bullet must list the pick and must not say the page
// has no trust facts at all. The README and the Policy already say so.
func TestTrustModelSharingPageFields(t *testing.T) {
	doc := strings.Join(strings.Fields(r08tReadRepo(t, "../docs/TRUST-MODEL.md")), " ")
	if strings.Contains(doc, "a username, a comment or the trust facts") {
		t.Errorf("TRUST-MODEL.md says the public page has no trust facts; its sentence names the confirmation count")
	}
	r08tMust(t, "TRUST-MODEL.md", doc,
		"the severity, the What is happening pick for a Critical report, and when it was posted and ended",
		"a comment, or the trust details beyond the one sentence (the sentence names how many people nearby confirmed it; the cells, the votes and the scores are never shown)",
		"(`TestSharePageShowsOnlyAllowedFields`)")
}
