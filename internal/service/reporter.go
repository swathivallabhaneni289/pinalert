// reporter.go composes the public reporter signal (D-10, D-13, D-14, D-15,
// D-23, R-05, A47). It turns a reporter's bounded history rows and one batch
// of current vote rows into the tag, the score, the words and the counts a
// reader sees. It only composes: reliability.go decides every rule (what an
// outcome is, what counts as judged, the window, the cap, the credits and the
// tag cuts), and trustcopy.go holds every word. The file is pure: now is a
// parameter, nothing here reads the clock or performs I/O, and no state
// outlives a call.

package service

import (
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// ReporterSignal is what a reader sees about the person who posted a report.
// It is display only (D-14): it never hides, dims, blocks, reorders or
// filters anything. It is public by design (D-10), and it carries no account
// id, email or session id. Score is nil for a New reporter and for none.
type ReporterSignal struct {
	// Tag is the closed reliability tag: reliable, mixed, unreliable, new
	// or none.
	Tag ReporterTag
	// Score is the 0 to 100 rating, nil for a New reporter and for none.
	Score *int
	// Label is the everyday word for the tag, empty for none (D-15).
	Label string
	// Sentence is the panel line, built on the server (D-19).
	Sentence string
	// ScoreSentence is the popup score line, empty when Score is nil (A50).
	ScoreSentence string
	// JudgedReports is J, the count of reports with a clear result.
	JudgedReports int
	// CorroboratedReports is C, how many of the judged reports were
	// confirmed by people nearby.
	CorroboratedReports int
}

// reporterHistoryMeta converts a history row into the report metadata the
// classifier reads. An unknown category falls back to other and an unknown
// severity to low, the same fallback the Activity read path uses, so the
// classification sees the metadata every read path sees.
func reporterHistoryMeta(row sqlcgen.ReporterHistoryReportsRow) ReportMeta {
	meta := ReportMeta{Severity: Severity(row.Severity), Category: Category(row.Category)}
	if !meta.Category.Valid() {
		meta.Category = CategoryOther
	}
	if !meta.Severity.Valid() {
		meta.Severity = SeverityLow
	}
	return meta
}

// newReporterSignal builds the signal for a computed rating. Score is a fresh
// pointer to a copy, and the popup score sentence exists only when there is a
// score (A50).
func newReporterSignal(r Reliability) ReporterSignal {
	s := ReporterSignal{
		Tag:                 r.Tag,
		Label:               reporterLabel(r.Tag),
		Sentence:            reporterSentence(r.Tag, r.Judged, r.Corroborated),
		JudgedReports:       r.Judged,
		CorroboratedReports: r.Corroborated,
	}
	if r.Score != nil {
		score := *r.Score
		s.Score = &score
		s.ScoreSentence = reporterScoreSentence(score)
	}
	return s
}

// ReporterSignals rates every requested account from its history rows and a
// batch of current vote rows (D-10, D-13, D-15, D-23, R-05).
//
// Every requested account gets exactly one entry, and an account that was not
// requested gets none. The reporter of each history report is the scored
// account, so that account's own content votes never count, and its own
// resolve or reopen cannot erase a record, because ClassifyOutcome clears the
// resolution fields. Every classified row, neutral included, goes on to
// ScoreReliability, which applies the window and the cap of the latest 30
// expired reports, so the answer on raw rows is the same as on the capped SQL
// read (RESOLVED Q4: the read caps expired reports, not judged ones).
//
// The caller passes one vote slice that includes the history report ids. The
// votes are grouped by report once, so the work grows with history rows plus
// vote rows rather than their product (T-03-05). The result is display only
// (D-14). The none signal for accountless reports and for reports past the
// account cap is not produced here: it comes from a nil TrustInput.Reporter.
func ReporterSignals(accountIDs []int64, history []sqlcgen.ReporterHistoryReportsRow, votes []sqlcgen.CurrentVotesForReportsRow, now time.Time) map[int64]ReporterSignal {
	requested := make(map[int64]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		requested[id] = struct{}{}
	}

	votesByReport := make(map[int64][]sqlcgen.CurrentVotesForReportsRow)
	for _, v := range votes {
		votesByReport[v.ReportID] = append(votesByReport[v.ReportID], v)
	}

	outcomes := make(map[int64][]JudgedOutcome, len(requested))
	for _, row := range history {
		if _, ok := requested[row.AccountID]; !ok {
			continue
		}
		reporter := row.AccountID
		tally := BuildVoteTally(votesByReport[row.ReportID], row.ReportID, &reporter)
		outcomes[row.AccountID] = append(outcomes[row.AccountID], JudgedOutcome{
			Outcome:   ClassifyOutcome(reporterHistoryMeta(row), tally, row.ExpiresAt, now),
			ExpiresAt: row.ExpiresAt,
		})
	}

	signals := make(map[int64]ReporterSignal, len(requested))
	for id := range requested {
		signals[id] = newReporterSignal(ScoreReliability(outcomes[id], now))
	}
	return signals
}
