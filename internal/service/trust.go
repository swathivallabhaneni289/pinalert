// trust.go holds the vote vocabulary, the one independence predicate
// (independentCellCount), and the voting service (VotingService). Unlike
// visibility.go, this file deliberately DOES consume generated store
// types — it is the layer that turns raw vote rows into the VoteTally
// visibility.go's Resolve function consumes.

package service

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
)

// voterGeohashPrecision is the geohash precision used to compute a
// VOTER's own cell for the independence predicate (TRUST-03), and it is
// deliberately DIFFERENT from report.go's geohashPrecision = 8. The two
// constants answer different questions: geohashPrecision locates where a
// report happened; voterGeohashPrecision decides how far apart two voters
// must stand to count as independent confirmers. Precision 7 is the
// deliberate, documented choice (D-03): a cell is about 153 m tall and about
// 125 to 151 m wide at Indian latitudes, because the width is 152.9 m times
// the cosine of the latitude (about 134 m at Delhi and about 149 m at
// Chennai). Phase 3 closes assumption A3 from 02-RESEARCH.md with this
// decision. The derivation table for precisions 5 to 8 lives in
// docs/TRUST-MODEL.md, and the 300 m witness simulation lives in
// TestGeohashPrecisionWitnesses; plan 03-02 writes both in this wave. It is
// explicitly not a library default. Reusing precision 8 here is the mistake
// this comment exists to prevent.
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

// placeVote is one counted vote: the place it was cast in and the time it was
// cast. The time is kept for plan 03-08's still current window and plays no
// part in any count here.
type placeVote struct {
	Cell      string
	CreatedAt time.Time
}

// voteBuckets is the output of filterVoteRows: the counted votes of one
// report in four buckets, plus the reporter's own resolution state.
type voteBuckets struct {
	Confirms, Disputes, Resolves, Reopens []placeVote
	ReporterResolved, ReporterReopened    bool
}

// filterVoteRows is the one routing of vote rows. Both BuildVoteTally and
// BuildTrust read it, so the resolver's gate and the displayed counters
// cannot drift (D-04, RESEARCH Pattern 1). Rows of other reports are skipped,
// the reporter's own content rows are skipped, and the reporter's resolution
// row sets the two reporter flags instead of entering a bucket. Unrecognised
// kinds and values are ignored. Callers must pass rows already deduplicated
// per account by CurrentVotesForReports.
func filterVoteRows(rows []sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID *int64) voteBuckets {
	var b voteBuckets

	for _, row := range rows {
		if row.ReportID != reportID {
			continue
		}
		isReporter := reporterAccountID != nil && row.AccountID == *reporterAccountID
		pv := placeVote{Cell: row.GeohashCell, CreatedAt: row.CreatedAt}

		switch VoteKind(row.Kind) {
		case VoteKindContent:
			if isReporter {
				// T-02-05 defence in depth: D-03 stops these at write
				// time, and this covers any row that predates the check.
				continue
			}
			switch VoteValue(row.Value) {
			case VoteConfirm:
				b.Confirms = append(b.Confirms, pv)
			case VoteDispute:
				b.Disputes = append(b.Disputes, pv)
			}
		case VoteKindResolution:
			if isReporter {
				// D-13's instant resolve and the amended D-16 instant
				// reopen are one privilege with two values, not a
				// privilege plus a special case. Both assignments read
				// off this same row, side by side, in the same shape,
				// because the symmetry is the amended decision.
				// CurrentVotesForReports keeps at most one resolution row
				// per account, so at most one of these flags can be set.
				// 02-01 forbids a defensive check for the both true case,
				// and none is added.
				b.ReporterResolved = VoteValue(row.Value) == VoteResolve
				b.ReporterReopened = VoteValue(row.Value) == VoteReopen
				continue
			}
			switch VoteValue(row.Value) {
			case VoteResolve:
				b.Resolves = append(b.Resolves, pv)
			case VoteReopen:
				b.Reopens = append(b.Reopens, pv)
			}
		}
	}
	return b
}

// placeVoteCells returns the cells of votes, in order.
func placeVoteCells(votes []placeVote) []string {
	cells := make([]string, 0, len(votes))
	for _, v := range votes {
		cells = append(cells, v.Cell)
	}
	return cells
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
// takes no context.Context, so it is as testable as Resolve itself. The row
// routing now lives in filterVoteRows, which BuildTrust reads too.
func BuildVoteTally(rows []sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID *int64) VoteTally {
	b := filterVoteRows(rows, reportID, reporterAccountID)
	return VoteTally{
		ConfirmCells:     independentCellCount(placeVoteCells(b.Confirms)),
		DisputeCells:     independentCellCount(placeVoteCells(b.Disputes)),
		ResolveCells:     independentCellCount(placeVoteCells(b.Resolves)),
		ReopenCells:      independentCellCount(placeVoteCells(b.Reopens)),
		ReporterResolved: b.ReporterResolved,
		ReporterReopened: b.ReporterReopened,
	}
}

// gpsDeniedMessage is 02-UI-SPEC.md's Copywriting Contract copy for a
// missing or out-of-range coordinate on the wire, verbatim — reused here
// rather than paraphrased because report.go's ValidationError doc comment
// requires a server rejection to never contradict the client's own inline
// copy, and a garbled/absent coordinate is exactly the situation this
// copy describes.
const gpsDeniedMessage = "Voting needs your location, so nearby confirmations can be verified as independent. Turn on location access for this site and try again."

// VotingQuerier is the entire store surface the voting path touches —
// small enough to fake without a real Postgres, mirroring report.go's
// Querier. 02-04 writes its own fake against this exact shape. The
// parameter is named reportIDs here while sqlc emits reportIds; Go
// matches interfaces structurally, so the names need not agree, but the
// types must.
type VotingQuerier interface {
	InsertVote(ctx context.Context, arg sqlcgen.InsertVoteParams) error
	CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error)
}

// VotingService authorizes, records, and resolves confirm/dispute/
// resolve/reopen votes.
type VotingService struct {
	q VotingQuerier
}

// NewVotingService constructs a VotingService backed by q.
func NewVotingService(q VotingQuerier) *VotingService {
	return &VotingService{q: q}
}

// CastVoteInput is every client-settable field of a cast vote.
// GeohashCell and CreatedAt are deliberately absent because the server
// computes them (following SubmitInput's own idiom), and AccountID comes
// from the verified-account gate's request context (02-03b), never from a
// request body — a client cannot assert whose vote this is.
type CastVoteInput struct {
	ReportID  int64
	AccountID int64
	Kind      VoteKind
	Value     VoteValue
	Latitude  float64
	Longitude float64
}

// CastVoteResult is the two values 02-03b serialises and 02-05/02-06
// render. A named result, rather than a bare pair, leaves room for
// 02-04's response work to grow the shape without changing every
// caller's arity.
type CastVoteResult struct {
	Visibility Visibility
	Reason     ResolveReason
}

// CastVote authorizes, records, and resolves a single confirm/dispute/
// resolve/reopen vote. The steps run in this exact order, and the order is
// part of the specification:
//
//  1. Validate the input, before touching the store, so garbage input costs
//     zero database round trips.
//  2. Load the report's vote context (severity, category, expiry, reporter,
//     coordinates and creation time) in one round trip.
//  3. Apply D-03's self-vote block, scoped to VoteKindContent alone: resolve
//     and reopen are both VoteKindResolution values the reporter is always
//     allowed to cast on their own report (D-13, D-16 amended). This sits
//     above the expiry check because it is the authorisation decision and
//     should not depend on a state check that might later move.
//  4. Decide the reporter resolution exemption. It is true only for a
//     resolution vote whose account is the report's reporter, read from the
//     database against the authenticated account (T-03-01). It skips step 6
//     and nothing else.
//  5. Reject an expired report. This is RESEARCH Open Question 2's locked
//     answer: reject, do not accept-and-ignore. The exemption never skips it.
//  6. Reject a voter farther than MaxVoteDistanceKm from the report (D-01),
//     unless step 4 exempted the vote. This runs before the cell is computed
//     and before anything is written, and a NaN or Inf coordinate fails
//     closed (T-03-02).
//  7. Compute the voter's geohash cell server-side from raw coordinates (the
//     T-02-01 control: CastVoteInput has no geohash field for a client to
//     populate), append the vote, read the current votes back AFTER the
//     insert so the response carries the POST-vote state (D-04), then build
//     the tally and resolve. CastVote contains NO branch of its own for the
//     reporter's instant resolve or instant reopen: both arrive as flags on
//     the tally BuildVoteTally builds and are adjudicated by isRetracted()
//     inside Resolve. A reporter reopening their own Retracted report
//     therefore gets the un-retracted visibility back through the ordinary
//     path, with no special case anywhere in this function. Adding one here
//     would create the second visibility authority T-02-04 exists to
//     prevent.
//
// CastVote calls Resolve rather than deciding visibility itself, so the
// vote-cast response and 02-04's feed read give byte-identical answers
// for the same data (TRUST-02 / T-02-04). No transaction, row lock or
// conflict clause appears anywhere in this sequence because 02-02's
// append-only schema has no contended row to serialise (T-02-02).
func (s *VotingService) CastVote(ctx context.Context, in CastVoteInput) (CastVoteResult, error) {
	// 1. Validate.
	if !in.Kind.Valid() {
		return CastVoteResult{}, ValidationError{Field: "kind", Message: "Unrecognised vote kind."}
	}
	if !in.Value.ValidFor(in.Kind) {
		return CastVoteResult{}, ValidationError{Field: "value", Message: "Unrecognised vote value for this kind."}
	}
	if in.Latitude < -90 || in.Latitude > 90 {
		return CastVoteResult{}, ValidationError{Field: "latitude", Message: gpsDeniedMessage}
	}
	if in.Longitude < -180 || in.Longitude > 180 {
		return CastVoteResult{}, ValidationError{Field: "longitude", Message: gpsDeniedMessage}
	}

	// 2. Load the report's vote context.
	rc, err := s.q.ReportVoteContext(ctx, in.ReportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CastVoteResult{}, ErrReportNotFound
		}
		return CastVoteResult{}, err
	}

	// 3. D-03's self-vote block. The VoteKindContent guard is
	// load-bearing in both directions: dropping it would break the
	// reporter's instant resolution path, and widening it past content
	// votes would silently disable the two actions that path exists to
	// grant, D-13's instant resolve and D-16-amended's instant reopen.
	// The nil check is not defensive noise: a pre-Phase-1.1 report
	// legitimately has no reporter account, and a nil pointer must never
	// compare equal to a caller.
	if in.Kind == VoteKindContent && rc.ReporterAccountID != nil && *rc.ReporterAccountID == in.AccountID {
		return CastVoteResult{}, ErrCannotVoteOwnReport
	}

	// 4. The reporter resolution exemption (T-03-01). It compares the
	// reporter identity the database returned with the authenticated
	// account, for the resolution kind only, and reads nothing from the
	// request body. A nil reporter exempts nobody. It skips the distance
	// check in step 6 and nothing else.
	isReporterResolutionVote := in.Kind == VoteKindResolution && rc.ReporterAccountID != nil && *rc.ReporterAccountID == in.AccountID

	// 5. Reject an expired report. now is captured once, here, as the
	// authoritative clock, never a client-supplied timestamp, matching
	// Submit's own convention.
	now := time.Now().UTC()
	if !rc.ExpiresAt.After(now) {
		return CastVoteResult{}, ErrReportExpired
	}

	// 6. D-01's distance refusal, before the cell is computed and before
	// anything is written. The check is written as a negated within test so
	// a NaN or Inf coordinate on either side is refused (T-03-02).
	if !isReporterResolutionVote && !withinVoteRadius(rc.Latitude, rc.Longitude, in.Latitude, in.Longitude) {
		return CastVoteResult{}, ErrVoterTooFar
	}

	// 7. Compute the voter's cell server-side. Any geohash string a
	// client might invent has nowhere to enter, because CastVoteInput has no
	// such field.
	cell := geohash.EncodeWithPrecision(in.Latitude, in.Longitude, voterGeohashPrecision)

	// Append the vote.
	if err := s.q.InsertVote(ctx, sqlcgen.InsertVoteParams{
		ReportID:    in.ReportID,
		AccountID:   in.AccountID,
		Kind:        string(in.Kind),
		Value:       string(in.Value),
		GeohashCell: cell,
	}); err != nil {
		return CastVoteResult{}, err
	}

	// Read the current votes back. This must come after the insert: the
	// response's whole purpose (D-04) is to carry the POST-vote state.
	rows, err := s.q.CurrentVotesForReports(ctx, []int64{in.ReportID})
	if err != nil {
		return CastVoteResult{}, err
	}

	// Build the tally and resolve.
	tally := BuildVoteTally(rows, in.ReportID, rc.ReporterAccountID)
	meta := ReportMeta{Severity: Severity(rc.Severity), Category: Category(rc.Category)}
	vis, reason := Resolve(meta, tally, now)
	return CastVoteResult{Visibility: vis, Reason: reason}, nil
}
