// timelinecopy.go holds every sentence of the report page's history, "What
// happened so far" (plan 08-01, PAGE-01, SC2). The sentences are built only
// from constants and strconv.Itoa, they stay inside the closed character set of
// server authored copy (ASCII letters, digits, space, comma, full stop and
// apostrophe), and the word place appears in none of them: the wording is
// people and spots (A88). No sentence holds a relative time, because the wire
// carries the time as RFC 3339 and the browser formats it (Pitfall 8). The
// numbers in these sentences always come from the VoteTally the trust block
// reads, never from a count of rows.

package service

import "strconv"

const (
	timelineNoLongerDisputed = "No longer marked Disputed."
	timelineReopenedPoster   = "Reopened by the poster."
	timelineReopenedPeople   = "Reopened by people nearby."
	timelineFaded            = "Faded out. No longer updating."
	timelineYouConfirmed     = "You confirmed it."
	timelineYouDisputed      = "You disputed it."
	timelineYouResolved      = "You marked it resolved."
	timelineYouReopened      = "You reopened it."

	timelineShownSuffix  = " Shown to everyone."
	timelineMarkedSuffix = " Marked Disputed."
)

// timelinePostedText is the first line: how serious the poster called it. A
// severity this package does not know gives the plain "Posted."
func timelinePostedText(s Severity) string {
	w := SeverityWord(s)
	if w == "" {
		return "Posted."
	}
	return "Posted as " + w + "."
}

// timelineSpotWord is the milestone phrase for n spots: "a second spot" to "a
// fifth spot" for 2 to 5, and "N spots" for every other count of 2 or more.
func timelineSpotWord(n int) string {
	switch n {
	case 2:
		return "a second spot"
	case 3:
		return "a third spot"
	case 4:
		return "a fourth spot"
	case 5:
		return "a fifth spot"
	}
	return strconv.Itoa(n) + " spots"
}

// timelineConfirmedText is the milestone sentence for n confirming spots: "by
// someone nearby" for the first, an ordinal for 2 to 5, a count after that.
func timelineConfirmedText(n int) string {
	if n <= 1 {
		return "Confirmed by someone nearby."
	}
	return "Confirmed from " + timelineSpotWord(n) + "."
}

// timelineConfirmedLiveText is the count form that also says the report is now
// shown to everyone. It is used at the first step the report is Live, whether
// or not that step is a milestone.
func timelineConfirmedLiveText(n int) string {
	return "Confirmed from " + strconv.Itoa(n) + " spots." + timelineShownSuffix
}

// timelineDisputedText is the milestone sentence for n disputing spots.
func timelineDisputedText(n int) string {
	if n <= 1 {
		return "Disputed by someone nearby."
	}
	return "Disputed from " + timelineSpotWord(n) + "."
}

// timelineDisputedMarkedText is the milestone sentence followed by the state
// change, for the step that enters the Disputed view on a milestone.
func timelineDisputedMarkedText(n int) string {
	return timelineDisputedText(n) + timelineMarkedSuffix
}

// timelineDisputedMarkedCountText is the count form of the same state change,
// for a step that enters the Disputed view without crossing a milestone.
func timelineDisputedMarkedCountText(n int) string {
	return "Disputed from " + strconv.Itoa(n) + " spots." + timelineMarkedSuffix
}

// timelineResolvedText is the closing line: the built closedText words for who
// closed it, then a full stop. An unknown ClosedBy gives the plain "Resolved."
func timelineResolvedText(by ClosedBy) string {
	w := closedText(by)
	if w == "" {
		return "Resolved."
	}
	return w + "."
}

// timelineReopenedText is the reopen line: the poster, or people nearby.
func timelineReopenedText(byPoster bool) string {
	if byPoster {
		return timelineReopenedPoster
	}
	return timelineReopenedPeople
}
