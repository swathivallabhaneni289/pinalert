// updatescopy.go is the one home of the words of an update (plan 03.2-02).
// Every word an update shows is written here or comes from the trust object at
// read time; nothing is stored. An update carries no description beyond the
// report's first line, no place name, no coordinate and no account (D-15 as
// amended by the owner's Q1 answer of 2026-10-18: rows show the report's first
// line, and only through the Rescue blur in ReportFirstLine). Every string is
// built from the constants in this file and integers, so no report text enters
// an event line, and no line is built from a time, so two reads a moment apart
// stay equal.

package service

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	// UnavailableLine is line 1 of a row whose report was held, removed or has
	// gone since the update was made. The list builder (plan 03) uses it, so an
	// update never leaks what the feed now withholds (Pitfall 9).
	UnavailableLine = "This report is no longer available"
	// UnconfirmedLine is the trust line of a Reliable reporter note while the
	// report is active and not confirmed by people (owner amendment
	// 2026-10-18).
	UnconfirmedLine = "Not confirmed yet."
)

// EventLine is the first line of an update row for an event kind (UI-SPEC A3).
// The disputed line replaces D-15's older Hidden by disputes wording: since Phase 3.4 a
// disputed report is not hidden, it sits at the bottom of All marked Disputed.
// The rolled up marker and any unknown kind have no event line.
func EventLine(k UpdateKind) string {
	switch k {
	case UpdateReachedLive:
		return "Now confirmed"
	case UpdateHidden:
		return "Now disputed"
	case UpdateResolved:
		return "Marked resolved"
	case UpdateFading:
		return "Fades in about an hour"
	case UpdateReliableNearby:
		return "Reported nearby by a Reliable reporter."
	default:
		return ""
	}
}

// UpdateTrustLine is line 3 of an update row (UI-SPEC A3 line 3). It is chosen
// at read time from the current trust object and view, and nothing about it is
// stored (D-15), so a count can never be stale. For the Reliable reporter note
// the owner amendment of 2026-10-18 applies: the note comes at posting, so the
// same row reads "Not confirmed yet." while the report is active and not
// confirmed by people, and reads the confirmed sentence once people nearby
// confirm it. The list builder shows UnavailableLine instead once the report is
// held, removed or gone, so this function never has to explain a withheld one.
//
// For every event kind the line is non empty, because the confirmed sentence
// is non empty for any count. The rolled up marker and an unknown kind have no
// trust line.
func UpdateTrustLine(k UpdateKind, t Trust, view FeedView, confirmedByPeople bool) string {
	switch k {
	case UpdateReliableNearby:
		switch view {
		case FeedViewActive:
			if confirmedByPeople {
				return t.ConfirmedSentence
			}
			return UnconfirmedLine
		case FeedViewDisputed:
			return t.DisputedSentence
		default:
			return leadOrConfirmed(t)
		}
	case UpdateHidden:
		return t.DisputedSentence
	case UpdateReachedLive:
		return t.ConfirmedSentence
	case UpdateResolved, UpdateFading:
		return leadOrConfirmed(t)
	default:
		return ""
	}
}

// leadOrConfirmed is the lead sentence when the trust object has one and the
// confirmed sentence otherwise.
func leadOrConfirmed(t Trust) string {
	if t.LeadSentence != "" {
		return t.LeadSentence
	}
	return t.ConfirmedSentence
}

// MoreLine is the text of the rolled up row of D-16: "And 1 more change" and
// "And N more changes".
func MoreLine(n int) string {
	if n == 1 {
		return "And 1 more change"
	}
	return "And " + strconv.Itoa(n) + " more changes"
}

// ReportFirstLine is the first line of a report's description, trimmed and cut
// to ReportLineMaxRunes runes. It is the one place an update reads report
// text. The text comes only through CoarsenFinishedRescue, so a finished Rescue
// report shows no text (D-23, D-28), and an unfinished one and every other
// category show the line. If the owner's Q1 answer changes to no, the list
// builder in plan 03 stops filling the field; nothing changes here.
func ReportFirstLine(r Report, finished bool) string {
	coarse, _ := CoarsenFinishedRescue(r, finished, nil)
	text := strings.TrimSpace(coarse.Description)
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) <= ReportLineMaxRunes {
		return text
	}
	runes := []rune(text)
	return string(runes[:ReportLineMaxRunes])
}
