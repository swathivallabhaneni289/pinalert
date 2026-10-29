// profilecopy.go holds every sentence of the people profile that the server
// builds (Phase 9, plan 09-04, UI-SPEC Copywriting Contract rows P9-C3 to
// P9-C14 and P9-C18). Counts, plurals, dates and the stars text are built here
// so the browser never rounds, pluralises or formats a number (D-13). Nothing
// here mentions a sound or a vibration.

package service

import (
	"strconv"
	"time"
)

// CannotFollowMessage is the one sentence for every reason a follow is refused
// except the caller's own cap, so the answer is not an oracle about tags or
// blocks (T-09-13).
const CannotFollowMessage = "You can't follow this person right now."

// ProfileNotFoundMessage is the one line of the not found answer for an
// unknown, private, deleted, pending or blocking profile.
const ProfileNotFoundMessage = "Profile not found."

// FollowingCapMessage is the caller's own cap refusal, built from FollowingCap
// so the number never drifts.
var FollowingCapMessage = "You can follow up to " + strconv.Itoa(FollowingCap) + " people."

// FollowersWord is "Follower" for 1 and "Followers" for any other number.
func FollowersWord(n int) string {
	if n == 1 {
		return "Follower"
	}
	return "Followers"
}

// FollowersCountText is the whole Followers phrase: "1 Follower", "12 Followers".
func FollowersCountText(n int) string {
	return strconv.Itoa(n) + " " + FollowersWord(n)
}

// FollowingCountText is the whole Following phrase: "30 Following".
func FollowingCountText(n int) string {
	return strconv.Itoa(n) + " Following"
}

// MemberSinceText is "Member since October 2026": the English month name and
// the four digit year. The app has no display time zone for dates (no
// time.LoadLocation anywhere in this package), so the month is the UTC one.
func MemberSinceText(t time.Time) string {
	u := t.UTC()
	return "Member since " + u.Month().String() + " " + strconv.Itoa(u.Year())
}

// OthersText is the line under a named list for the people shown only as a
// number (D-03): "and 3 others" and "and 1 other" when some are named, "3 others"
// and "1 other" when none are, and empty for 0.
func OthersText(n int, anyNamed bool) string {
	if n <= 0 {
		return ""
	}
	word := "others"
	if n == 1 {
		word = "other"
	}
	text := strconv.Itoa(n) + " " + word
	if anyNamed {
		return "and " + text
	}
	return text
}

// StarsTotalText is the stars total with the word, "3.6 stars", floored to one
// decimal through StarsText.
func StarsTotalText(points int) string {
	return StarsText(points) + " stars"
}

// StarsGroupLabel is the first sentence of the stars group's accessible name,
// "Stars: 3.6 out of 5." The parts sentences are added by plan 09-13.
func StarsGroupLabel(points int) string {
	return "Stars: " + StarsText(points) + " out of 5."
}

// OwnStateLine is the line under the buttons on your own page (UI-SPEC 1g):
// one for a public profile that holds the tag, one for a public profile that
// does not, and one for a profile that is off.
func OwnStateLine(public, reliable bool) string {
	switch {
	case !public:
		return "Only you can see this page. Turn on Let others follow me in Settings."
	case reliable:
		return "People can follow you while you hold the Reliable reporter tag."
	default:
		return "People can follow you once you hold the Reliable reporter tag."
	}
}
