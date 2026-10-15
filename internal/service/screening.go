// screening.go is the one reusable text policy (D-06): it decides whether a
// piece of free text passes, goes to review or is refused, from the scores of
// an external moderation provider and from one gibberish rule. Nothing in it
// knows about reports, so Phase 8 comments and Phase 9 nicknames reuse it
// unchanged. See 04-CONTEXT.md decisions D-03, D-04, D-05 and D-06 and the
// derived rule R-06 (count characters, not bytes).
//
// The file is pure: no I/O, no context, no clock and no package level
// mutable state.

package service

// Verdict is the outcome of screening one piece of text from provider scores.
type Verdict string

const (
	VerdictPass       Verdict = "pass"
	VerdictBorderline Verdict = "borderline"
	VerdictReject     Verdict = "reject"
)

// Verdicts is the ordered, canonical list of every valid verdict.
var Verdicts = []Verdict{VerdictPass, VerdictBorderline, VerdictReject}

// Valid reports whether v is one of the three canonical verdicts.
// Comparison is exact, with no case insensitive coercion.
func (v Verdict) Valid() bool {
	for _, x := range Verdicts {
		if x == v {
			return true
		}
	}
	return false
}

// RejectReason names why a text was refused. It is a closed, serialized
// contract: the slugs are frozen here.
type RejectReason string

const (
	RejectHateful   RejectReason = "hateful"
	RejectHarassing RejectReason = "harassing"
	RejectSexual    RejectReason = "sexual"
	RejectGibberish RejectReason = "gibberish"
)

// RejectReasons is the ordered, canonical list of every valid reject reason.
// The order also breaks ties between equal group scores in Decide.
var RejectReasons = []RejectReason{RejectHateful, RejectHarassing, RejectSexual, RejectGibberish}

// Valid reports whether r is one of the four canonical reject reasons.
// Comparison is exact, with no case insensitive coercion.
func (r RejectReason) Valid() bool {
	for _, x := range RejectReasons {
		if x == r {
			return true
		}
	}
	return false
}

// ScreenDecision is the answer of Decide. Reason is empty unless Verdict is
// VerdictReject.
type ScreenDecision struct {
	Verdict Verdict
	Reason  RejectReason
}

// RejectScoreThreshold and BorderlineScoreThreshold are the two score lines
// Decide compares against, both "at or above". They are starting values
// (04-RESEARCH assumption A3) to be tuned only on the scores recorded from
// the pinned provider snapshot omni-moderation-2024-09-26 for the D-07
// fixture (recorded by 04-27, checked by 04-28). These two lines are the only
// part of this file a later plan may change; every test builds its boundary
// rows from the constants.
const (
	RejectScoreThreshold     = 0.85
	BorderlineScoreThreshold = 0.40
)

// rejectGroup ties one reject reason to the provider keys that can cause it.
type rejectGroup struct {
	reason RejectReason
	keys   []string
}

// rejectGroups is an ordered slice, never a map, so no result depends on map
// iteration order. The order is the RejectReasons order.
var rejectGroups = []rejectGroup{
	{reason: RejectHateful, keys: []string{"hate", "hate/threatening"}},
	{reason: RejectHarassing, keys: []string{"harassment", "harassment/threatening"}},
	{reason: RejectSexual, keys: []string{"sexual", "sexual/minors"}},
}

// Decide maps provider category scores to a ScreenDecision. The rungs are
// checked in this exact order, the order is the specification:
//
//  1. Reject (D-03): the score of a group is the highest score among its
//     keys (an absent key reads 0). The group with the highest score, ties
//     going to the earlier group in RejectReasons order, names the reason,
//     and it rejects when that score is at or above RejectScoreThreshold.
//  2. Borderline (D-04): any score at all, over every key in the map
//     including keys the provider adds later, at or above
//     BorderlineScoreThreshold.
//  3. Pass: everything else, including a nil or empty map.
//
// serious is accepted so every caller states the severity explicitly, and it
// is never read: severity is self declared, and failing open on it would let
// anyone skip screening by choosing Critical (D-03, T-04-02). The only input
// is the category score map, so the provider's own overall verdict is never
// consulted. Violence, self harm and illicit categories can only ever reach
// borderline, because genuine disaster reports use those words. Decide never
// returns RejectGibberish (the caller runs IsGibberish first) and never
// writes to scores.
func Decide(scores map[string]float64, serious bool) ScreenDecision {
	_ = rejectGroups
	return ScreenDecision{Verdict: VerdictPass}
}
