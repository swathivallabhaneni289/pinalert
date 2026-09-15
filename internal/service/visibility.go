// visibility.go is the sole authority for a report's visibility state: the
// question of whether a report is Hidden, Provisional, Live, or Retracted.
// Every surface that displays a report — feed, map, triage view, shareable
// card — crosses into Resolve to learn the answer, so that the answer is
// identical everywhere (TRUST-02). See 02-CONTEXT.md decisions D-05 through
// D-16 for the reasoning this ladder encodes.

package service

import "time"

// Visibility is one of the four states a report's confirm/dispute/resolve
// tally can resolve to.
type Visibility string

const (
	VisibilityHidden      Visibility = "hidden"
	VisibilityProvisional Visibility = "provisional"
	VisibilityLive        Visibility = "live"
	VisibilityRetracted   Visibility = "retracted"
)

// Visibilities is the ordered, canonical list of every valid visibility.
var Visibilities = []Visibility{VisibilityHidden, VisibilityProvisional, VisibilityLive, VisibilityRetracted}

// Valid reports whether v is one of the four canonical visibilities.
// Comparison is exact — no case-insensitive coercion.
func (v Visibility) Valid() bool {
	for _, x := range Visibilities {
		if x == v {
			return true
		}
	}
	return false
}

// IndependentAgreementThreshold is the one independence number read by all
// four independence-gated rungs of the ladder (D-14): the Hidden trigger's
// floor, the Provisional-to-Live gate, confirmer-driven resolving, and
// non-reporter-driven reopening. The value 2 comes from TRUST-04's own
// "second independent confirmation" wording, logged as assumption A2 in
// 02-RESEARCH.md. The two reporter-instant paths (D-13 resolve,
// D-16-amended reopen) read no threshold at all — they are booleans, not a
// threshold of 1.
const IndependentAgreementThreshold = 2

// ResolveReason names the specific rung of the ladder a Visibility came
// from. It is a closed, serialized contract: 02-04 puts the slug on the
// wire and 02-06 maps it to display copy, so these string values are
// frozen here.
type ResolveReason string

const (
	ReasonResolved             ResolveReason = "resolved"
	ReasonCriticalBypass       ResolveReason = "critical_bypasses_gates"
	ReasonDisputed             ResolveReason = "disputed"
	ReasonAwaitingConfirmation ResolveReason = "awaiting_second_independent_confirmation"
	ReasonConfirmed            ResolveReason = "confirmed"
)

// ResolveReasons is the ordered, canonical list of every valid resolve
// reason.
var ResolveReasons = []ResolveReason{
	ReasonResolved,
	ReasonCriticalBypass,
	ReasonDisputed,
	ReasonAwaitingConfirmation,
	ReasonConfirmed,
}

// ReportMeta is deliberately the smallest subset of a report Resolve
// needs — not the full Report struct — so this file carries no
// store-shaped dependency.
type ReportMeta struct {
	Severity Severity
	Category Category
}

// criticalBypass reports whether m exempts its report from both the
// Provisional gate (TRUST-04) and the Hidden-by-dispute trigger (D-06).
// TODO(02-01 Task 2): implement the decision ladder.
func (m ReportMeta) criticalBypass() bool {
	return false
}

// VoteTally is an already-loaded snapshot of a report's confirm, dispute,
// resolve, and reopen votes, in the exact shape Resolve needs.
//
// Populating this from raw vote rows is trust.go's job in 02-03a; Resolve
// itself never sees a vote row or a database handle.
//
// The reporter gets an instant, threshold-free path on BOTH halves of the
// resolve/reopen pair (D-16 amended 2026-09-15, symmetric with D-13), while
// a non-reporter's resolve or reopen still needs
// IndependentAgreementThreshold distinct cells either way.
//
// ReporterResolved and ReporterReopened are both derived from the
// reporter's ONE current resolution vote, so BuildVoteTally never produces
// both true. Where a caller constructs it anyway, the behaviour is
// defined, not undefined: reopen wins, matching the non-reporter half
// where reopen outranks resolve at equal standing.
//
// These two booleans are the only reporter-privileged inputs in the whole
// tally — there is no reporter carve-out on the confirm/dispute (Hidden)
// side at all.
type VoteTally struct {
	// ConfirmCells is the standing independent confirm count — distinct
	// geohash cells among current confirm votes.
	ConfirmCells int
	// DisputeCells is the same, for disputes.
	DisputeCells int
	// ResolveCells is the standing independent resolve count, EXCLUDING
	// the reporter's own vote.
	ResolveCells int
	// ReopenCells is the standing independent reopen count, EXCLUDING the
	// reporter's own vote.
	ReopenCells int
	// ReporterResolved is true when the reporter's own current
	// resolution vote is resolve — D-13's instant path.
	ReporterResolved bool
	// ReporterReopened is true when the reporter's own current
	// resolution vote is reopen — D-16-amended's instant path.
	ReporterReopened bool
}

// isRetracted reports whether t currently retracts its report.
// TODO(02-01 Task 2): implement the decision ladder.
func (t VoteTally) isRetracted() bool {
	return false
}

// Resolve maps a report's static metadata and its currently-loaded vote
// tally to the single Visibility state and ResolveReason every read path
// must display. Resolve is THE single authority for this decision
// (TRUST-02 / threat T-02-04): no read path may re-derive the answer in
// SQL, in a handler, or in JavaScript.
//
// Resolve is pure: no I/O, no context.Context, and no package-level
// mutable state. now is currently unused — expiry is a separate Phase 1
// read-time predicate (expires_at > now()) applied alongside this
// resolver's output, rather than being a fifth state (D-07) — and the
// parameter is retained for Phase 3's decay scoring (TRUST-07) per the
// VisibilityResolver contract in .planning/research/ARCHITECTURE.md.
func Resolve(meta ReportMeta, tally VoteTally, now time.Time) (Visibility, ResolveReason) {
	// TODO(02-01 Task 2): implement the decision ladder.
	return VisibilityProvisional, ReasonAwaitingConfirmation
}
