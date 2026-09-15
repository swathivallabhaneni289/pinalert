// trust.go holds the vote vocabulary, the one independence predicate
// (independentCellCount), and the voting service (VotingService). Unlike
// visibility.go, this file deliberately DOES consume generated store
// types — it is the layer that turns raw vote rows into the VoteTally
// visibility.go's Resolve function consumes.

package service

import (
	"errors"

	sqlcgen "pinalert/internal/store/sqlc"
)

// voterGeohashPrecision is the geohash precision used to compute a
// VOTER's own cell for the independence predicate (TRUST-03) —
// deliberately DIFFERENT from report.go's geohashPrecision = 8. The two
// constants answer different questions: geohashPrecision locates where a
// report happened; voterGeohashPrecision decides how far apart two voters
// must stand to count as independent confirmers. 7 is roughly a 153m
// cell, chosen against PITFALLS.md's ~100-300m realistic incident radius.
// This is an interim, documented choice logged as assumption A3 in
// 02-RESEARCH.md, which Phase 3 revisits with a benchmarked value
// (TRUST-05) — it is explicitly not a library default. Reusing precision
// 8 here is the mistake this comment exists to prevent.
const voterGeohashPrecision = 7

// VoteKind discriminates the two independently-tracked signals a vote can
// carry: is this report still true (content), or is it resolved
// (resolution). Keeping them as two kinds — rather than one overloaded
// vote vocabulary — is what lets "is this report still true" and "is this
// resolved" stay separately tracked signals (D-02, D-16). These string
// values are written into votes.kind, which 02-02 left as unconstrained
// TEXT precisely so this enum is the single validator.
type VoteKind string

const (
	VoteKindContent    VoteKind = "content"
	VoteKindResolution VoteKind = "resolution"
)

// VoteKinds is the ordered, canonical list of every valid vote kind.
var VoteKinds = []VoteKind{VoteKindContent, VoteKindResolution}

// Valid reports whether k is one of the two canonical vote kinds.
// Comparison is exact — no case-insensitive coercion.
func (k VoteKind) Valid() bool {
	for _, v := range VoteKinds {
		if v == k {
			return true
		}
	}
	return false
}

// VoteValue is the specific value a vote carries within its kind.
type VoteValue string

const (
	VoteConfirm VoteValue = "confirm"
	VoteDispute VoteValue = "dispute"
	VoteResolve VoteValue = "resolve"
	VoteReopen  VoteValue = "reopen"
)

// VoteValues is the ordered, canonical list of every valid vote value.
var VoteValues = []VoteValue{VoteConfirm, VoteDispute, VoteResolve, VoteReopen}

// ValidFor reports whether v is a legal value for kind k: confirm/dispute
// under VoteKindContent, resolve/reopen under VoteKindResolution, and
// false for every other combination including an unknown kind. Written as
// a switch on k so that adding a third kind later fails to compile
// silently rather than defaulting to permissive. This is the V5 Input
// Validation control: the routes 02-03b mounts pair kind and value at
// compile time, so a mismatch is unreachable through the HTTP surface
// today — this method is the defence that survives a future caller that
// does not have that guarantee.
func (v VoteValue) ValidFor(k VoteKind) bool {
	switch k {
	case VoteKindContent:
		return v == VoteConfirm || v == VoteDispute
	case VoteKindResolution:
		return v == VoteResolve || v == VoteReopen
	default:
		return false
	}
}

// ErrCannotVoteOwnReport is returned when a caller attempts a content vote
// (confirm/dispute) on their own report — D-03. 02-03b maps it to 403.
var ErrCannotVoteOwnReport = errors.New("service: cannot vote on your own report")

// ErrReportExpired is returned when a vote is cast against a report whose
// expires_at is not after the server's own clock — RESEARCH Open Question
// 2's locked answer: reject, do not accept-and-ignore. 02-03b maps it to
// 409.
var ErrReportExpired = errors.New("service: report has expired")

// ErrReportNotFound is returned when ReportVoteContext yields
// pgx.ErrNoRows for the given report id. 02-03b maps it to 404.
var ErrReportNotFound = errors.New("service: report not found")

// independentCellCount is the ONE implementation TRUST-03's independence
// predicate reduces to. D-14 requires it be reused by the Hidden trigger,
// the Provisional gate, confirmer-driven resolving and reopening, rather
// than reimplemented per gate. The distinct-ACCOUNT half of the predicate
// is already guaranteed by CurrentVotesForReports' DISTINCT ON — callers
// must pass an already-per-account-deduped slice, and this function does
// not deduplicate by account itself. It stays unexported so nothing
// outside this package can build a competing count.
func independentCellCount(cells []string) int {
	seen := make(map[string]struct{}, len(cells))
	for _, c := range cells {
		seen[c] = struct{}{}
	}
	return len(seen)
}

// BuildVoteTally turns a batch of current-vote rows into the six-field
// VoteTally visibility.go's Resolve consumes. The reportID parameter
// exists because 02-04 loads votes for a whole page of reports in ONE
// CurrentVotesForReports call and then calls this per report — removing
// the parameter would push the feed path into an N+1. Unrecognised kinds
// and values are ignored rather than erroring, so Phase 3 can extend the
// vocabulary without a migration. The reporter's own resolution vote is
// surfaced via the symmetric ReporterResolved/ReporterReopened pair and
// excluded from ResolveCells/ReopenCells exactly as visibility.go's
// six-field VoteTally doc comment specifies (D-13, D-16 amended
// 2026-09-15 — see 02-CONTEXT.md D-16's amendment history so a reader who
// finds an older document does not remove the reopen flag). Both flags
// are set only on a server-side reporter-identity match against the
// account ReportVoteContext resolved, never from anything a client can
// assert, because a client able to set ReporterReopened could un-retract
// any report in the system (T-02-03). This function performs no I/O and
// takes no context.Context, so it is as testable as Resolve itself.
func BuildVoteTally(rows []sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID *int64) VoteTally {
	var confirmCells, disputeCells, resolveCells, reopenCells []string
	var reporterResolved, reporterReopened bool

	for _, row := range rows {
		if row.ReportID != reportID {
			continue
		}
		isReporter := reporterAccountID != nil && row.AccountID == *reporterAccountID

		switch VoteKind(row.Kind) {
		case VoteKindContent:
			if isReporter {
				// T-02-05 defence in depth: D-03 stops these at write
				// time, and this covers any row that predates the check.
				continue
			}
			switch VoteValue(row.Value) {
			case VoteConfirm:
				confirmCells = append(confirmCells, row.GeohashCell)
			case VoteDispute:
				disputeCells = append(disputeCells, row.GeohashCell)
			}
		case VoteKindResolution:
			if isReporter {
				// D-13's instant resolve and D-16-amended's instant
				// reopen are one privilege with two values, not a
				// privilege plus a special case — both assignments read
				// off this SAME row, side by side, in the same shape:
				// the symmetry IS the amended decision.
				// CurrentVotesForReports' DISTINCT ON (report_id,
				// account_id, kind) guarantees at most one resolution
				// row per account survives the read, so at most one of
				// these two flags can ever be set here; 02-01 forbids
				// adding a defensive invariant check for the both-true
				// case, and none is added.
				reporterResolved = VoteValue(row.Value) == VoteResolve
				reporterReopened = VoteValue(row.Value) == VoteReopen
				continue
			}
			switch VoteValue(row.Value) {
			case VoteResolve:
				resolveCells = append(resolveCells, row.GeohashCell)
			case VoteReopen:
				reopenCells = append(reopenCells, row.GeohashCell)
			}
		}
	}

	return VoteTally{
		ConfirmCells:     independentCellCount(confirmCells),
		DisputeCells:     independentCellCount(disputeCells),
		ResolveCells:     independentCellCount(resolveCells),
		ReopenCells:      independentCellCount(reopenCells),
		ReporterResolved: reporterResolved,
		ReporterReopened: reporterReopened,
	}
}
