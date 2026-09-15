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
// This single predicate serves BOTH exemptions — deliberately one method
// rather than two copies that could drift apart — and is keyed on the
// report's severity/category fields, never on triage sort weight
// (TRUST-06: the same field has two independent consumers).
func (m ReportMeta) criticalBypass() bool {
	return m.Severity == SeverityCritical || m.Category == CategoryRescueNeeded
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

// isRetracted reports whether t currently retracts its report. The
// expression encodes four semantics:
//
//   - D-13 — the reporter's own current resolution vote retracts
//     instantly, with no threshold.
//   - D-14 — a non-reporter confirmer needs IndependentAgreementThreshold
//     standing independent resolve cells, the one shared independence
//     number, not a second bespoke one. The same number, on the other
//     side, is what a non-reporter reopen needs.
//   - D-16 (amended 2026-09-15) — reopening is symmetric with D-13's
//     resolve: the reporter's own standing reopen vote lifts the
//     retraction INSTANTLY at zero independent reopen cells, and does so
//     regardless of how the report became Retracted — their own instant
//     resolve, or independent confirmers reaching the threshold. Everyone
//     else still needs IndependentAgreementThreshold standing independent
//     reopen cells. This reverses the original 2026-09-12 reading
//     (threshold-only reopen for every account) — see 02-CONTEXT.md D-16's
//     amendment history; do not "fix" the symmetry back out.
//   - D-08 — both sides read STANDING counts and standing votes (each
//     account's current resolution vote, as produced by 02-03a's
//     BuildVoteTally), so nothing latches on either half. Reopen outranks
//     resolve at equal standing; a report whose standing reopen cells
//     later drop back below the threshold re-retracts; and a reporter who
//     changes their resolution vote away from reopen likewise lets any
//     surviving resolve agreement reassert itself. State is always a pure
//     function of the current tally, never a one-way unlock.
func (t VoteTally) isRetracted() bool {
	resolved := t.ReporterResolved || t.ResolveCells >= IndependentAgreementThreshold
	reopened := t.ReporterReopened || t.ReopenCells >= IndependentAgreementThreshold
	return resolved && !reopened
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
//
// The rungs below are checked in this exact order — the order is the
// specification:
//
//  1. Retracted (D-07): a distinct trigger, fired only by explicit resolve
//     marking, outranks everything below it, including the critical
//     bypass, because marking a report resolved must work regardless of
//     severity. The only ways past this rung are the two reopen paths
//     (D-16 amended), both of which isRetracted already accounts for;
//     there is no reopen branch out here in Resolve itself.
//  2. Critical bypass (D-06, TRUST-04): sits ABOVE the dispute rung (a
//     critical or rescue-needed report can never be Hidden by disputes)
//     and ABOVE the confirm gate (critical/rescue-needed publishes at
//     full visibility immediately, with no gate). One rung, both
//     exemptions.
//  3. Hidden (D-05): a floor (DisputeCells >= IndependentAgreementThreshold,
//     so a single dispute can never hide a report) AND a strict majority
//     (DisputeCells > ConfirmCells, so a bare tie does not hide either).
//     There is deliberately no matching unhide branch anywhere in this
//     file — D-08's reversibility falls out of recomputing this
//     expression against the current tally on every read.
//  4. Provisional gate (TRUST-04): Provisional is emitted as a state
//     distinct from Live rather than folded into it, because D-09's
//     dimmed-and-labelled treatment (built in 02-06) needs a discrete
//     state to key off.
//  5. Live: the default once nothing above fired.
func Resolve(meta ReportMeta, tally VoteTally, now time.Time) (Visibility, ResolveReason) {
	if tally.isRetracted() {
		return VisibilityRetracted, ReasonResolved
	}
	if meta.criticalBypass() {
		return VisibilityLive, ReasonCriticalBypass
	}
	if tally.DisputeCells >= IndependentAgreementThreshold && tally.DisputeCells > tally.ConfirmCells {
		return VisibilityHidden, ReasonDisputed
	}
	if tally.ConfirmCells < IndependentAgreementThreshold {
		return VisibilityProvisional, ReasonAwaitingConfirmation
	}
	return VisibilityLive, ReasonConfirmed
}
