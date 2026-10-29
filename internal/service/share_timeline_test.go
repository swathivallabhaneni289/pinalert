package service

import (
	"strings"
	"testing"
)

// TestPublicEndedSentenceMatchesTimeline pins that the public page's Ended line
// is the same words the report page's history timeline uses for a report that
// faded out (Phase 8 plan 01, timelinecopy.go), so a forwarded report and the
// app never say two different things about the same ending.
func TestPublicEndedSentenceMatchesTimeline(t *testing.T) {
	if PublicEndedSentence != timelineFaded {
		t.Errorf("PublicEndedSentence = %q, want the timeline's faded sentence %q", PublicEndedSentence, timelineFaded)
	}
	if !strings.HasSuffix(PublicEndedSentence, ".") {
		t.Errorf("PublicEndedSentence = %q, want a full sentence", PublicEndedSentence)
	}
}
