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
	return false
}

// JudgedOutcome is one history row: an outcome and the moment the report
// expired, which is the time every weight is measured from.
type JudgedOutcome struct {
	Outcome   Outcome
	ExpiresAt time.Time
}

// ClassifyOutcome is not implemented yet.
func ClassifyOutcome(meta ReportMeta, tally VoteTally, expiresAt, now time.Time) Outcome {
	return ""
}
