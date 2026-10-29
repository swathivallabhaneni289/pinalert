package web

// Phase 9 round 2 (2026-10-30): the docs handoffs of the first fix round. These
// tests read the shipped docs and the shipped constants, so a doc line cannot
// drift from the code it describes.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func round2Doc(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile("../docs/" + name)
	if err != nil {
		t.Fatalf("reading docs/%s: %v", name, err)
	}
	return strings.Join(strings.Fields(string(raw)), " ")
}

// TestModerationDocNamesTheVisibilityBucketAndTheProfilePage pins the rate table
// row of the Visibility bucket with the numbers main.go holds, the note that
// /u/{username} shares the People reads bucket and answers a 429 page, and the
// words "first written" for the restore window (WR-S01).
func TestModerationDocNamesTheVisibilityBucketAndTheProfilePage(t *testing.T) {
	doc := round2Doc(t, "MODERATION.md")
	main, err := os.ReadFile("../cmd/server/main.go")
	if err != nil {
		t.Fatalf("reading cmd/server/main.go: %v", err)
	}
	burst := regexp.MustCompile(`visibilityBurst\s*=\s*(\d+)`).FindStringSubmatch(string(main))
	every := regexp.MustCompile(`visibilityEvery\s*=\s*(\d+) \* time\.Second`).FindStringSubmatch(string(main))
	if burst == nil || every == nil {
		t.Fatal("cmd/server/main.go does not define visibilityBurst and visibilityEvery in seconds")
	}
	row := "| account | " + burst[1] + " | 1 per " + every[1] + " seconds | `visibilityBurst`, `visibilityEvery` |"
	if !strings.Contains(doc, row) {
		t.Errorf("docs/MODERATION.md rate table lacks the Visibility row %q", row)
	}
	for _, want := range []string{
		"Phase 9 added the four people rows.",
		"turning it off is never refused by it",
		"the field `rate_visibility`",
		"The profile page `/u/{username}` takes its token from the People reads bucket",
		"the page answers a 429 page with Retry-After, never JSON",
		"since the earliest removed rating was first written are restored",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("docs/MODERATION.md lacks %q", want)
		}
	}
	if strings.Contains(doc, "removed rating was written are restored") {
		t.Error("docs/MODERATION.md still says the restore window starts when the rating was written, not first written")
	}
}

// TestDocsSayAProfileBlursFinishedRescueAtPrecisionFive pins that no doc sends a
// finished Rescue report on a profile through the precision 6 blur (IN-T01).
func TestDocsSayAProfileBlursFinishedRescueAtPrecisionFive(t *testing.T) {
	trust := round2Doc(t, "TRUST-MODEL.md")
	for _, want := range []string{
		"a finished Rescue report is blurred the same way and shows no text (it takes the precision 5 cell too",
		"Phase 9's followable profile does not use that function",
		"through `CoarsenFinishedForProfile`",
		"A person's profile does not use this blur",
	} {
		if !strings.Contains(trust, want) {
			t.Errorf("docs/TRUST-MODEL.md lacks %q", want)
		}
	}
	if strings.Contains(trust, "Phase 9's followable profiles show finished reports, so they call `CoarsenFinishedRescue`") {
		t.Error("docs/TRUST-MODEL.md still sends profiles through CoarsenFinishedRescue")
	}
	for _, name := range []string{"MODERATION.md", "PUSH.md"} {
		doc := round2Doc(t, name)
		for _, bad := range []string{"CoarsenFinishedRescue", "precision 6 cell"} {
			if strings.Contains(doc, bad) {
				t.Errorf("docs/%s mentions %q; a profile's finished Rescue report is at precision 5", name, bad)
			}
		}
	}
}
