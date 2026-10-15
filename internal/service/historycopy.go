// historycopy.go holds the server written history strings (plan 03.1-01):
// who closed a report, the outcome label of an expired report and how long a
// report lived. The wording is people and never a place (D-09), the numbers
// are whole, and every string stays inside the closed character set of server
// authored copy. None of these strings says a confirmation extended the life
// of a report (backlog 999.12).

package service

import (
	"strconv"
	"time"
)

// closedText is the line under a resolved report: who closed it. An empty or
// unknown ClosedBy gives an empty string.
func closedText(by ClosedBy) string {
	switch by {
	case ClosedByYou:
		return "Resolved by you"
	case ClosedByPoster:
		return "Resolved by the poster"
	case ClosedByPeople:
		return "Resolved by people nearby"
	case ClosedByPosterAndPeople:
		return "Resolved by the poster and people nearby"
	case ClosedByYouAndPeople:
		return "Resolved by you and people nearby"
	default:
		return ""
	}
}

// outcomeLabelText is the label of an expired report, written from the outcome
// ClassifyOutcome gave, so the history label can never disagree with the
// reporter rating. A corroborated report names the number of independent
// people who confirmed it; when that number is below one, which a real tally
// cannot produce, the label falls back to Expired. A critical or rescue report
// that was out disputed arrives here as contradicted and reads Disputed (D-10).
// An unknown Outcome reads Expired.
func outcomeLabelText(o Outcome, confirmed int) string {
	switch o {
	case OutcomeCorroborated:
		switch {
		case confirmed == 1:
			return "Confirmed by 1 person nearby"
		case confirmed > 1:
			return "Confirmed by " + strconv.Itoa(confirmed) + " people nearby"
		}
		return "Expired"
	case OutcomeContradicted:
		return "Disputed"
	case OutcomeUnconfirmed:
		return "Expired unconfirmed"
	default:
		return "Expired"
	}
}

// lifetimeText is how long a report lived, from posting to expiry (D-08): whole
// hours rounded down from one hour up, whole minutes rounded up below that. A
// span that is not positive gives an empty string.
func lifetimeText(createdAt, expiresAt time.Time) string {
	d := expiresAt.Sub(createdAt)
	if d <= 0 {
		return ""
	}
	if d >= time.Hour {
		hours := int(d / time.Hour)
		if hours == 1 {
			return "after 1 hr"
		}
		return "after " + strconv.Itoa(hours) + " hrs"
	}
	minutes := int((d + time.Minute - 1) / time.Minute)
	return "after " + strconv.Itoa(minutes) + " min"
}
