// timelinecopy_test.go pins the closed wording of the report history (plan
// 08-01): every sentence the copy can produce stays inside the closed character
// set, ends with a full stop and never names a place. It is package service and
// needs no database.

package service

import (
	"strings"
	"testing"
)

// p801Check fails when a sentence leaves the closed set, lacks its full stop or
// holds the word place.
func p801Check(t *testing.T, label, s string) {
	t.Helper()
	assertPlainCopy(t, s)
	if s == "" {
		t.Errorf("%s: empty sentence", label)
		return
	}
	if !strings.HasSuffix(s, ".") {
		t.Errorf("%s: %q does not end with a full stop", label, s)
	}
	if strings.Contains(strings.ToLower(s), "place") {
		t.Errorf("%s: %q names a place", label, s)
	}
}

func TestTimelineWording(t *testing.T) {
	for _, sev := range []Severity{SeverityLow, SeverityMedium, SeverityCritical, Severity("other")} {
		p801Check(t, "posted "+string(sev), timelinePostedText(sev))
	}
	if got := timelinePostedText(SeverityCritical); got != "Posted as Critical." {
		t.Errorf("posted critical = %q", got)
	}
	for _, n := range TimelineMilestones {
		p801Check(t, "confirmed milestone", timelineConfirmedText(n))
		p801Check(t, "disputed milestone", timelineDisputedText(n))
		p801Check(t, "disputed marked milestone", timelineDisputedMarkedText(n))
	}
	for n := 1; n <= 130; n++ {
		p801Check(t, "confirmed live count", timelineConfirmedLiveText(n))
		p801Check(t, "disputed marked count", timelineDisputedMarkedCountText(n))
		p801Check(t, "confirmed any count", timelineConfirmedText(n))
		p801Check(t, "disputed any count", timelineDisputedText(n))
	}
	for _, by := range []ClosedBy{ClosedByYou, ClosedByPoster, ClosedByPeople, ClosedByPosterAndPeople, ClosedByYouAndPeople, ClosedBy("")} {
		p801Check(t, "resolved "+string(by), timelineResolvedText(by))
	}
	for _, byPoster := range []bool{true, false} {
		p801Check(t, "reopened", timelineReopenedText(byPoster))
	}
	for label, s := range map[string]string{
		"no longer disputed": timelineNoLongerDisputed,
		"faded":              timelineFaded,
		"you confirmed":      timelineYouConfirmed,
		"you disputed":       timelineYouDisputed,
		"you resolved":       timelineYouResolved,
		"you reopened":       timelineYouReopened,
	} {
		p801Check(t, label, s)
	}

	exact := map[string]string{
		timelineConfirmedText(1):             "Confirmed by someone nearby.",
		timelineConfirmedText(2):             "Confirmed from a second spot.",
		timelineConfirmedText(3):             "Confirmed from a third spot.",
		timelineConfirmedText(4):             "Confirmed from a fourth spot.",
		timelineConfirmedText(5):             "Confirmed from a fifth spot.",
		timelineConfirmedText(10):            "Confirmed from 10 spots.",
		timelineConfirmedText(50):            "Confirmed from 50 spots.",
		timelineConfirmedLiveText(2):         "Confirmed from 2 spots. Shown to everyone.",
		timelineDisputedText(1):              "Disputed by someone nearby.",
		timelineDisputedText(25):             "Disputed from 25 spots.",
		timelineDisputedMarkedText(3):        "Disputed from a third spot. Marked Disputed.",
		timelineDisputedMarkedCountText(6):   "Disputed from 6 spots. Marked Disputed.",
		timelineResolvedText(ClosedByPoster): "Resolved by the poster.",
	}
	for got, want := range exact {
		if got != want {
			t.Errorf("sentence = %q, want %q", got, want)
		}
	}
}
