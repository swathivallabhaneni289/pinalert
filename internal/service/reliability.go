// reliability.go holds the pure outcome classifier and the reporter
// reliability score behind the "is this source reliable" half of TRUST-07.
// ClassifyOutcome decides what one finished report says about its reporter
// (D-11, D-12, R-05, R-06) and ScoreReliability turns a history of those
// outcomes into the public rating and tag (D-13, D-15, D-23, R-05). Both are
// pure: no clock, no I/O, no store types.

package service

import "time"

// Outcome is what one finished report says about the person who posted it.
type Outcome string

const (
	OutcomeCorroborated Outcome = "corroborated"
	OutcomeContradicted Outcome = "contradicted"
	OutcomeUnconfirmed  Outcome = "unconfirmed"
	OutcomeNeutral      Outcome = "neutral"
)

// Outcomes is the ordered, canonical list of every valid outcome.
var Outcomes = []Outcome{OutcomeCorroborated, OutcomeContradicted, OutcomeUnconfirmed, OutcomeNeutral}

// Valid reports whether o is one of the four canonical outcomes.
func (o Outcome) Valid() bool {
	for _, x := range Outcomes {
		if x == o {
			return true
		}
	}
	return false
}

// JudgedOutcome is one history row: an outcome and the moment the report
// expired, which is the time every weight is measured from.
type JudgedOutcome struct {
	Outcome   Outcome
	ExpiresAt time.Time
}

// ClassifyOutcome decides what one finished report says about its reporter
// (D-11, D-12, R-05, R-06, T-03-11). It never re-derives the visibility
// ladder: it asks the one authority, Resolve, with neutralised metadata and
// a copy of the tally whose four resolution fields are cleared, so the
// reporter rating can never disagree with the ladder and the reporter's own
// resolve or reopen, or any resolve agreement, can never change an outcome.
// A self resolved or Retracted report is therefore judged by its content
// votes only.
//
// A report is judged once expiresAt is at or before now; a report still in
// flight is neutral. Live maps to corroborated, Hidden to contradicted, and
// Provisional to unconfirmed, except that an unconfirmed report exempted by
// the critical bypass predicate is neutral and not judged (D-12).
//
// A critical or rescue needed report that independent places out dispute
// comes back contradicted for the rating only; the real read path still
// shows it Live, because this function never changes visibility (R-06).
// Phase 3.1 reuses this function, including its neutral result, for
// history labels.
func ClassifyOutcome(meta ReportMeta, tally VoteTally, expiresAt, now time.Time) Outcome {
	if expiresAt.After(now) {
		return OutcomeNeutral
	}
	zeroed := tally
	zeroed.ResolveCells = 0
	zeroed.ReopenCells = 0
	zeroed.ReporterResolved = false
	zeroed.ReporterReopened = false
	vis, _ := Resolve(ReportMeta{Severity: SeverityLow, Category: CategoryOther}, zeroed, now)
	switch vis {
	case VisibilityLive:
		return OutcomeCorroborated
	case VisibilityHidden:
		return OutcomeContradicted
	case VisibilityProvisional:
		if meta.criticalBypass() {
			return OutcomeNeutral
		}
		return OutcomeUnconfirmed
	}
	// Retracted is unreachable after zeroing, and neutral never moves a
	// rating, so any other value is safe to ignore.
	return OutcomeNeutral
}

// ReporterTag is the closed set of reliability tags a reporter can carry.
type ReporterTag string

const (
	ReporterReliable   ReporterTag = "reliable"
	ReporterMixed      ReporterTag = "mixed"
	ReporterUnreliable ReporterTag = "unreliable"
	ReporterNew        ReporterTag = "new"
	// ReporterNone is for accountless reports and reports past the account
	// cap. ScoreReliability never returns it; the reporter signal does.
	ReporterNone ReporterTag = "none"
)

// ReporterTags is the ordered, canonical list of every valid reporter tag.
var ReporterTags = []ReporterTag{ReporterReliable, ReporterMixed, ReporterUnreliable, ReporterNew, ReporterNone}

// Valid reports whether t is one of the five canonical reporter tags.
func (t ReporterTag) Valid() bool {
	return false
}

const (
	// ReliabilityMinJudged is the number of judged reports needed before a
	// reporter gets a score (D-13).
	ReliabilityMinJudged = 5
	// ReliabilityHalfLife is the age, measured from expires_at, at which a
	// report's weight has halved (D-13).
	ReliabilityHalfLife = 30 * 24 * time.Hour
	// ReliabilityWindow is how far back the history reaches (D-13, R-05).
	ReliabilityWindow = 90 * 24 * time.Hour
	// ReliabilityRowCap is the most expired reports considered (D-13, Q4).
	ReliabilityRowCap = 30
	// UnconfirmedCredit is the credit an unconfirmed report earns (D-23).
	UnconfirmedCredit = 0.25
	// ReliableMinScore is the lowest score tagged Reliable reporter (D-15, Q2).
	ReliableMinScore = 75
	// MixedMinScore is the lowest score tagged Mixed record (D-15, Q2).
	MixedMinScore = 35
)

// Reliability is a reporter's public rating. Score is nil for a New reporter.
type Reliability struct {
	Tag          ReporterTag
	Score        *int
	Judged       int
	Corroborated int
}

// ScoreReliability is not implemented yet.
func ScoreReliability(outcomes []JudgedOutcome, now time.Time) Reliability {
	return Reliability{}
}
