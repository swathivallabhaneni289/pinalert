// expiry.go holds the rule that lets a confirmation from a new spot push a
// report's fade time back (backlog 999.12, owner decision 2026-10-18), and the
// server's word on whether that can still happen.
//
// The rule, in plain words:
//
//   - R1. Only a content confirm can extend. A dispute never extends. A
//     resolve, a reopen and a take back are resolution votes and never extend.
//   - R2. A new spot: the voter's precision 7 cell has no earlier confirm row on
//     this report in the whole append only votes log, from any account other
//     than the reporter. The log, not the standing votes, is the reference, so
//     a confirm, dispute, confirm toggle can never extend twice from one cell.
//   - R3. One person extends at most once: the voter must have no earlier
//     confirm row on this report. Walking around cannot add places, and the
//     same is now true of the fade time.
//   - R4. The new fade time is the confirmation's own time plus ExpiryDuration of
//     the report's severity. It is never earlier than the current fade time and
//     never later than created_at plus ExpiryCap of the severity.
//   - R5. A report that has faded at the confirmation's time is never revived.
//   - R6. Nothing ever shortens a fade time. Switching a confirm to a dispute
//     (the only way to take a confirmation back) leaves it as it is.
//   - R7 (amended by A1, 2026-10-30). A report that is finished in any way,
//     resolved included, is never extended. A finished report has no life left to
//     extend, and extending it would delay the reliability judging that waits
//     for expiry. CastVote decides this in Go with Resolve and passes Extend
//     false; the SQL holds no visibility rule.
//
// InsertVoteExtending (internal/store/queries/extend.sql) is the one statement
// that applies the rule atomically with the vote. ExtendedExpiry below is the
// same rule in Go, used by the Demo twin and the tests.

package service

import "time"

// ExtendedExpiry is the fade time after a confirmation made at confirmedAt on a
// report created at createdAt that currently fades at current. It returns
// current when current is not after confirmedAt (a faded report is never
// revived); otherwise the later of current and the earlier of createdAt plus
// limit and confirmedAt plus life. life is ExpiryDuration of the severity and
// limit is ExpiryCap of it. The result is never earlier than current, so a
// seeded fade time past the cap is never shortened.
func ExtendedExpiry(createdAt, current, confirmedAt time.Time, life, limit time.Duration) time.Time {
	if !current.After(confirmedAt) {
		return current
	}
	candidate := confirmedAt.Add(life)
	if ceiling := createdAt.Add(limit); ceiling.Before(candidate) {
		candidate = ceiling
	}
	if candidate.After(current) {
		return candidate
	}
	return current
}

// mayExtend is the one place that says whether a report is still live enough for
// a confirmation to move its fade time: its visibility is the active or the
// disputed view at now. A Retracted report is the resolved view and a faded one
// is the past view, so neither extends. It is built on ViewOf, so Go stays the
// one visibility authority (T-02-04) and the SQL holds no visibility rule.
func mayExtend(v Visibility, expiresAt, now time.Time) bool {
	fv := ViewOf(v, expiresAt, now)
	return fv == FeedViewActive || fv == FeedViewDisputed
}
