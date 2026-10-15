package service

// ratecopy.go holds the server composed rate limit messages of D-12 and the
// wait phrase they share (UI-SPEC "Rate limit messages" and "Wait format").
// Handlers never type these sentences, they call these functions. The sentence
// "In an emergency, call ... now." is never built here, because the client
// appends it from the 429 body's serious flag and number.

import (
	"strconv"
	"time"
)

// waitSeconds rounds d up to whole seconds. Any value at or below one second,
// zero and negatives included, gives 1.
func waitSeconds(d time.Duration) int64 {
	if d <= time.Second {
		return 1
	}
	return int64((d + time.Second - 1) / time.Second)
}

// countWord returns "<n> <singular>" for one and "<n> <plural>" otherwise.
func countWord(n int64, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return strconv.FormatInt(n, 10) + " " + plural
}

// WaitPhrase names a wait in plain words. It first rounds up to whole seconds
// (at least 1), then uses seconds below 60, minutes rounded up below 3600
// seconds, and hours rounded up from there. So 59.5 seconds is "1 minute" and
// 3599 seconds is "60 minutes".
func WaitPhrase(d time.Duration) string {
	s := waitSeconds(d)
	switch {
	case s < 60:
		return countWord(s, "second", "seconds")
	case s < 3600:
		return countWord((s+59)/60, "minute", "minutes")
	default:
		return countWord((s+3599)/3600, "hour", "hours")
	}
}

// ReportLimitMessage is the sentence shown on the report form and on a queued
// row when a report is refused for posting too often. Its 429 field is
// rate_report. The serious form names serious reports; neither form carries an
// emergency sentence.
func ReportLimitMessage(serious bool, wait time.Duration) string {
	if serious {
		return "You've posted several serious reports in a short time. You can post again in " + WaitPhrase(wait) + ". Your text is still here."
	}
	return "You've posted several reports in a short time. You can post again in " + WaitPhrase(wait) + ". Your text is still here."
}

// VoteLimitMessage is the sentence shown in .vote-error when a voter has cast
// too many votes too fast. Its 429 field is rate_vote.
func VoteLimitMessage(wait time.Duration) string {
	return "You're voting very quickly. You can vote again in " + WaitPhrase(wait) + "."
}

// VoteChangeLimitMessage is the sentence shown in .vote-error when the same
// kind of vote on one report is cast again too soon. Its 429 field is
// rate_vote_change.
func VoteChangeLimitMessage(wait time.Duration) string {
	return "You changed this vote a moment ago. You can change it again in " + WaitPhrase(wait) + "."
}

// FlagLimitMessage is the sentence shown in the Flag dialog when a reader has
// flagged several reports in a short time. Its 429 field is rate_flag.
func FlagLimitMessage(wait time.Duration) string {
	return "You've flagged several reports in a short time. You can flag another in " + WaitPhrase(wait) + "."
}
