// trustview.go builds the one trust object every surface shows (D-21). The
// feed, the vote response and the Activity page each call BuildTrust once per
// report over their one batched vote read, so for the same data and clock the
// three carry the same value, and the value is the same for every viewer. The
// object holds the two counters (D-04, R-04, D-16), the lead line (D-17) and
// the confirmed and disputed panel sentences (D-19, A5, A6). The browser
// renders these values as text and holds no rule of its own.

package service

import (
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// Trust is the server built trust object for one report.
type Trust struct {
	// ConfirmedPlaces is N, the count of distinct places that confirm the
	// report (D-04). It is a lower bound on people, never a head count.
	ConfirmedPlaces int
	// DisputedPlaces is M, the count of distinct places that dispute it
	// (R-04), under the same independence rule as N.
	DisputedPlaces int
	// ConfirmedText is the capped numeral for N: 0 to 99, then 99+ (A27).
	ConfirmedText string
	// DisputedText is the capped numeral for M, with the same cap.
	DisputedText string
	// LeadSentence is the first panel line: empty for a Retracted report,
	// the expiry line once the report has expired, the Provisional line
	// while it needs more confirmations, otherwise empty (D-17).
	LeadSentence string
	// ConfirmedSentence is the panel line for N. It replaces the per word
	// hover reasons (A5).
	ConfirmedSentence string
	// DisputedSentence is the panel line for M (A56).
	DisputedSentence string
}

// TrustInput is everything BuildTrust reads. Visibility is the caller's own
// Resolve output and is never recomputed here. Meta and CreatedAt are read by
// plan 03-08's still current signal. There is deliberately no viewer field,
// so the object is the same for every viewer. ExpiresAt must be the report's
// real expiry.
type TrustInput struct {
	Rows              []sqlcgen.CurrentVotesForReportsRow
	ReportID          int64
	ReporterAccountID *int64
	Meta              ReportMeta
	Visibility        Visibility
	CreatedAt         time.Time
	ExpiresAt         time.Time
	Now               time.Time
}

// trustLeadSentence implements the lead line rule in order. Retracted wins
// over expired, and the expiry test is the same at or after rule CastVote
// refuses with.
func trustLeadSentence(vis Visibility, expiresAt, now time.Time) string {
	if vis == VisibilityRetracted {
		return ""
	}
	if !expiresAt.After(now) {
		return expiredLeadSentence
	}
	if vis == VisibilityProvisional {
		return provisionalLeadSentence()
	}
	return ""
}

// BuildTrust turns one report's batched vote rows into its Trust. It reads
// the same buckets BuildVoteTally reads, through filterVoteRows, and counts
// them with the one independentCellCount, so N and M cannot drift from the
// numbers the resolver gates on (D-04, R-04). It performs no I/O.
func BuildTrust(in TrustInput) Trust {
	buckets := filterVoteRows(in.Rows, in.ReportID, in.ReporterAccountID)
	confirmed := independentCellCount(placeVoteCells(buckets.Confirms))
	disputed := independentCellCount(placeVoteCells(buckets.Disputes))

	var t Trust
	t.ConfirmedPlaces = confirmed
	t.DisputedPlaces = disputed
	t.ConfirmedText = trustCountText(confirmed)
	t.DisputedText = trustCountText(disputed)
	t.LeadSentence = trustLeadSentence(in.Visibility, in.ExpiresAt, in.Now)
	t.ConfirmedSentence = confirmedSentence(confirmed)
	t.DisputedSentence = disputedSentence(disputed)
	return t
}
