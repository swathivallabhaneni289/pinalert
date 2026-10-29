package service

// fixpins_test.go pins the wording of comments and docs that the Phase 9 code
// review (2026-10-30) corrected, so they cannot drift back: K4, IN-E06 and
// IN-T01. Each reads a source or doc file and checks a phrase. No database.

import (
	"os"
	"strings"
	"testing"
)

func fixPinRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

func TestReviewFixCommentPins(t *testing.T) {
	cases := []struct {
		name, path string
		absent     []string
		present    []string
	}{
		{"K4: the fan out cap comment tells the truth", "pushdispatch.go",
			[]string{"only the phone note is left out"},
			[]string{"does not promise that nobody past it"}},
		{"IN-E06: the first alert index names follow", "pushkinds.go",
			[]string{"with critical and reliable (uq_push_outbox_first)"},
			[]string{"across critical, follow and reliable"}},
		{"IN-E06: account deletion removes queued follow notes", "accountdelete.go",
			nil,
			[]string{"DeleteQueuedFollowPushesForFollowee(ctx, accountID)"}},
		{"IN-E06: PUSH.md lists deleting the account", "../../docs/PUSH.md",
			nil,
			[]string{"deleting the person's account"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := fixPinRead(t, c.path)
			for _, s := range c.absent {
				if strings.Contains(src, s) {
					t.Errorf("%s still contains %q", c.path, s)
				}
			}
			for _, s := range c.present {
				if !strings.Contains(src, s) {
					t.Errorf("%s does not contain %q", c.path, s)
				}
			}
		})
	}
}
