// reliability.go holds the pure outcome classifier and the reporter
// reliability score behind the "is this source reliable" half of TRUST-07.
// ClassifyOutcome decides what one finished report says about its reporter
// (D-11, D-12, R-05, R-06) and ScoreReliability turns a history of those
// outcomes into the public rating and tag (D-13, D-15, D-23, R-05). Both are
// pure: no clock, no I/O, no store types.

package service

import (
	"cmp"
	"math"
	"slices"
	"time"
)

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
	for _, x := range ReporterTags {
		if x == t {
			return true
		}
	}
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

// creditUnits expresses every credit in hundredths, so the credits 1, 0 and
// 0.25 are the integers 100, 0 and 25.
const creditUnits = 100

// unconfirmedCreditUnits is UnconfirmedCredit in hundredths. It is an exact
// compile time constant; if UnconfirmedCredit is ever set to a value that is
// not a whole number of hundredths the build fails, which is intended.
const unconfirmedCreditUnits int64 = UnconfirmedCredit * creditUnits

// weightShift makes every recency weight an integer in units of 2^-30, so
// equal ages give equal integers and the score boundaries stay exact under
// uniform ageing.
const weightShift = 30

// scoreScale is the top of the public score range.
const scoreScale = 100

// outcomeCredit returns the credit of o in hundredths and whether o is
// judged at all. Neutral and anything outside Outcomes is not judged.
func outcomeCredit(o Outcome) (int64, bool) {
	switch o {
	case OutcomeCorroborated:
		return creditUnits, true
	case OutcomeContradicted:
		return 0, true
	case OutcomeUnconfirmed:
		return unconfirmedCreditUnits, true
	}
	return 0, false
}

// outcomeWeight is the recency weight of a report that expired age ago: it
// halves every ReliabilityHalfLife, quantised to an integer in units of
// 2^-30 (age 0 gives exactly 2^30 and one half life gives exactly 2^29).
// Floating point appears only here, before the quantisation.
func outcomeWeight(age time.Duration) int64 {
	w := math.Exp2(-float64(age) / float64(ReliabilityHalfLife))
	return int64(math.Round(math.Ldexp(w, weightShift)))
}

// reporterTagForScore cuts the tag from the rounded integer score (D-15, Q2).
func reporterTagForScore(score int) ReporterTag {
	switch {
	case score >= ReliableMinScore:
		return ReporterReliable
	case score >= MixedMinScore:
		return ReporterMixed
	}
	return ReporterUnreliable
}

// ScoreReliability turns a reporter's history into the public rating
// (D-13, D-15, D-23, R-05).
//
// The score is round(100 * (n * p + 1) / (n + 2)) with rounding half up,
// where n is the number of judged reports and p is the recency weighted mean
// credit. The credits are 1 for corroborated, 0 for contradicted and 0.25 for
// unconfirmed (D-23). Each report's weight halves every 30 days, measured
// from expires_at (D-13).
//
// The history first keeps only reports with now - 90 days < expires_at <= now
// (so a report exactly 90 days old is out and one at now is in), then keeps
// the latest 30 by expires_at. The cap counts neutral rows too, which mirrors
// the SQL that caps the latest 30 expired reports (Q4), so the function gives
// the same answer on raw input and on SQL capped input. Ties on expires_at
// are broken by outcome slug, a total order, so input order never changes
// which rows survive. Only judged outcomes (not neutral) then count, and with
// fewer than 5 of them (unweighted) the reporter is New with a nil score and
// the real Judged and Corroborated counts.
//
// The tag is cut from the rounded integer score: 75 or more is Reliable
// reporter, 35 to 74 Mixed record, 34 or less Unreliable reporter (D-15).
//
// The arithmetic is exact integer arithmetic over int64 sums, so the
// boundaries 75, 35 and 25 are exact and the result does not depend on input
// order. Invariants: I1 a history of only corroborated outcomes is Reliable
// with score round(100 (n + 1) / (n + 2)); I2 restated for the credit 0.25, a
// history of only unconfirmed outcomes with at least 5 judged always reads
// Unreliable with score round(25 (n + 4) / (n + 2)); I3 a better outcome never
// lowers the score and order never matters; I4 the score lies in
// [100 / (n + 2), 100 (n + 1) / (n + 2)].
//
// ScoreReliability never returns ReporterNone, never reads the clock (now is
// a parameter), and never reorders or edits the caller's slice.
func ScoreReliability(outcomes []JudgedOutcome, now time.Time) Reliability {
	windowStart := now.Add(-ReliabilityWindow)
	kept := make([]JudgedOutcome, 0, len(outcomes))
	for _, e := range outcomes {
		if !e.ExpiresAt.After(now) && e.ExpiresAt.After(windowStart) {
			kept = append(kept, e)
		}
	}
	slices.SortFunc(kept, func(a, b JudgedOutcome) int {
		if c := b.ExpiresAt.Compare(a.ExpiresAt); c != 0 {
			return c
		}
		return cmp.Compare(string(a.Outcome), string(b.Outcome))
	})
	if len(kept) > ReliabilityRowCap {
		kept = kept[:ReliabilityRowCap]
	}

	var n, corroborated int
	var sum, weights int64
	for _, e := range kept {
		credit, judged := outcomeCredit(e.Outcome)
		if !judged {
			continue
		}
		n++
		if e.Outcome == OutcomeCorroborated {
			corroborated++
		}
		w := outcomeWeight(now.Sub(e.ExpiresAt))
		sum += w * credit
		weights += w
	}
	if n < ReliabilityMinJudged {
		return Reliability{Tag: ReporterNew, Judged: n, Corroborated: corroborated}
	}

	// score = round half up of scoreScale * (n*p + 1) / (n + 2) with
	// p = sum / (creditUnits * weights).
	num := scoreScale * (int64(n)*sum + creditUnits*weights)
	den := creditUnits * weights * int64(n+2)
	score := int((2*num + den) / (2 * den))
	return Reliability{
		Tag:          reporterTagForScore(score),
		Score:        &score,
		Judged:       n,
		Corroborated: corroborated,
	}
}
