package service

// sharecopy.go holds the words of Share and of the public status page
// (Phase 8 plan 02, D-04 to D-06). These words reach people with no account
// and WhatsApp's link preview, so they never carry a count except the server's
// own confirmed sentence on the page body, never a claim word stronger than the
// trust block's own sentences, and never a relative time inside the title or
// the fixed line. Every string is built from constants and strconv.Itoa only,
// so no report text can enter one. The closed character set is letters, digits,
// space, comma, full stop, apostrophe and the slash of "Storm/Cyclone damage".

import (
	"strconv"
	"strings"
	"time"
)

const (
	// ShareFixedLine is the line under every share title and the og:description
	// of the public page, the same bytes in both places. It has no full stop and
	// no digit, and it claims nothing: the page is drawn at the moment it is
	// opened, which is what keeps a forward true (D-04, D-06).
	ShareFixedLine = "Open to see if this is still true right now"

	// The status chips of the public page. Each carries a word and never a hue.
	PublicChipActive   = "Active"
	PublicChipDisputed = "Disputed"
	PublicChipResolved = "Resolved"
	PublicChipEnded    = "Ended"

	// PublicRemovedTitle is the heading of a report a moderator removed.
	PublicRemovedTitle = "This report was removed"
	// PublicUnavailableTitle is the one heading of a held, unknown, malformed or
	// too old link, so a moderation state is never revealed (UI-SPEC defaults
	// item 3).
	PublicUnavailableTitle = "This report is no longer available"
	// PublicTooManyTitle and PublicTooManyLine are the words of the rate limited
	// page.
	PublicTooManyTitle = "Too many requests"
	PublicTooManyLine  = "Wait a minute, then open the link again."

	// PublicEndedSentence is the box line of a report that faded out.
	PublicEndedSentence = "Faded out. No longer updating."

	// publicResolvedFallback is the box line of a resolved report whose closer is
	// not known, which a real closure never produces.
	publicResolvedFallback = "Resolved."

	shareReportWord = "report"
	shareNearWord   = "near"
)

// ShareTitle is the title of a share and of the public page: the category word,
// then "report", then, when an area is known, "near" and the area words. The
// reporter's words, a count and a claim have no way in. An empty or blank area
// gives "{Category} report". A value that is not a category gives "Report".
func ShareTitle(c Category, area string) string {
	title := "Report"
	if word := CategoryWord(c); word != "" {
		title = word + " " + shareReportWord
	}
	if a := AreaWords(area); a != "" {
		title += " " + shareNearWord + " " + a
	}
	return title
}

// PublicAgo is how long ago something was, in whole units rounded down: "just
// now" below a minute (and for a time in the future), then minutes, hours and
// days, singular for one.
func PublicAgo(then, now time.Time) string {
	d := now.Sub(then)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return agoUnit(int(d/time.Minute), "minute")
	case d < 24*time.Hour:
		return agoUnit(int(d/time.Hour), "hour")
	default:
		return agoUnit(int(d/(24*time.Hour)), "day")
	}
}

func agoUnit(n int, unit string) string {
	if n == 1 {
		return "1 " + unit + " ago"
	}
	return strconv.Itoa(n) + " " + unit + "s ago"
}

// publicResolvedSentence is the box line of a resolved report: the closed words
// of who closed it, with a full stop.
func publicResolvedSentence(by ClosedBy) string {
	text := strings.TrimSpace(closedText(by))
	if text == "" {
		return publicResolvedFallback
	}
	return text + "."
}
