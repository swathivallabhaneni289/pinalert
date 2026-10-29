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
	// OutcomeScore, Raters and TappedHalves are set only when tapped ratings
	// applied to this signal (plan 09-13, D-02, D-22): at least
	// MinRatersForStars counted raters, and either the profile is on or the
	// ratings only lowered the points. Score, Tag, Label and ScoreSentence
	// then follow the combined points. OutcomeScore is the points the report
	// outcomes alone gave, Raters is the number of counted raters (distinct
	// voter cells, D-21), and TappedHalves is the tapped average rounded to
	// half stars, in halves (review W5), never an exact average. When the
	// ratings did not apply, OutcomeScore and TappedHalves are nil, Raters is 0
	// and Score is the outcome points, so a signal built without ratings is
	// exactly the signal every surface showed before ratings existed.
	OutcomeScore *int
	Raters       int
	TappedHalves *int
}

// RatingTally is what the ratings read says about one reporter (plan 09-13,
// D-21): how many raters count, counted by distinct voter cell with one rating
// per cell, the sum of the counted stars, and whether the reporter's profile
// is on. It carries no rater and no cell.
type RatingTally struct {
	Raters        int
	TapSum        int
	ProfilePublic bool
}

// reporterHistoryMeta converts a history row into the report metadata the
// classifier reads. An unknown category falls back to other and an unknown
// severity to low, the same fallback the Activity read path uses, so the
// classification sees the metadata every read path sees. It also carries the
// stored moderation state so ClassifyOutcome can make a withheld report
// neutral (Phase 4 R-02).
func reporterHistoryMeta(row sqlcgen.ReporterHistoryReportsRow) ReportMeta {
	meta := ReportMeta{Severity: Severity(row.Severity), Category: Category(row.Category), Moderation: ModerationState(row.ModerationState)}
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

// noneReporterSignal is the canonical none signal (A47): tag none, null score,
// empty words and zero counts. The browser renders nothing for it.
func noneReporterSignal() ReporterSignal {
	return ReporterSignal{Tag: ReporterNone}
}

// reporterSignalFor turns the optional input signal into the one a trust
// object carries. A nil pointer is the real rule for accountless reports and
// for reports past the account cap, which the read paths leave unset (Q7).
// A none or invalid tag also gives the canonical none, so none never carries
// a word, a number or a count. Otherwise the result is a copy with its own
// Score pointer, so no two trust objects share one and a caller cannot change
// a built trust object through its input.
func reporterSignalFor(p *ReporterSignal) ReporterSignal {
	if p == nil || p.Tag == ReporterNone || !p.Tag.Valid() {
		return noneReporterSignal()
	}
	out := *p
	if p.Score != nil {
		score := *p.Score
		out.Score = &score
	}
	if p.OutcomeScore != nil {
		outcome := *p.OutcomeScore
		out.OutcomeScore = &outcome
	}
	if p.TappedHalves != nil {
		halves := *p.TappedHalves
		out.TappedHalves = &halves
	}
	return out
}

// SignalOption carries one input that is not in the history rows or the vote
// rows into ReporterSignals (tapped ratings, supplied by plan 09-13 through a
// SignalSource). The options are applied into a signalConfig before the loop,
// so ReporterSignals stays pure: it reads nothing and holds no state.
type SignalOption func(*signalConfig)

// signalConfig is what the options fill in. With nothing set ReporterSignals
// behaves as it always did.
type signalConfig struct {
	// ratings is the tapped rating tally per reporter (plan 09-13).
	ratings map[int64]RatingTally
}

// WithRatings supplies the counted tapped ratings of each reporter (plan 09-13,
// D-02, D-21, D-22). For a reporter with a score and at least MinRatersForStars
// counted raters the points become CombinePoints of the outcome score and the
// tally, and the tag is cut from those points by reporterTagForScore, the one
// place the Reliable line lives. A New reporter has no score, so ratings never
// rate a reporter who has too few judged reports (Phase 3 D-13). While the
// reporter's profile is off the ratings can only lower the points: the points
// are the smaller of the outcome score and the combined points, so switching
// the profile off never raises the tag (D-22).
func WithRatings(t map[int64]RatingTally) SignalOption {
	return func(c *signalConfig) {
		c.ratings = t
	}
}

// applyRatings returns the signal of one reporter with the tapped ratings
// applied when they apply, and the outcome only signal otherwise (see
// WithRatings).
func applyRatings(rel Reliability, tally RatingTally, ok bool) ReporterSignal {
	if rel.Score == nil || !ok || tally.Raters < MinRatersForStars {
		return newReporterSignal(rel)
	}
	outcome := *rel.Score
	combined := CombinePoints(outcome, tally.Raters, tally.TapSum)
	if !tally.ProfilePublic && combined > outcome {
		return newReporterSignal(rel)
	}
	rated := rel
	rated.Score = &combined
	rated.Tag = reporterTagForScore(combined)
	sig := newReporterSignal(rated)
	sig.OutcomeScore = &outcome
	sig.Raters = tally.Raters
	halves := TappedHalfStars(tally.Raters, tally.TapSum)
	sig.TappedHalves = &halves
	return sig
}

// ReporterSignals rates every requested account from its history rows and a
// batch of current vote rows (D-10, D-13, D-15, D-23, R-05).
//
// Production code never calls it directly: every read goes through
// reporterSignalsWith or LoadReporterSignals (reportersignals.go), so a source
// of extra inputs reaches every surface at once.
//
// Every requested account gets exactly one entry, and an account that was not
// requested gets none. The reporter of each history report is the scored
// account, so that account's own content votes never count, and its own
// resolve or reopen cannot erase a record, because ClassifyOutcome clears the
// resolution fields. Every classified row, neutral included, goes on to
// ScoreReliability, which applies the window, drops the neutral rows and caps
// the latest 30 judged reports (WR-05). Held and removed reports are neutral
// rows too (Phase 4 R-02), so they ride in the read like any other neutral
// row and ScoreReliability drops them before it caps. The SQL read caps at
// ReliabilityHistoryReadCap expired reports, neutral included, so the answer
// on raw rows is the same as on the capped read unless more than that many
// rows are neutral.
//
// The caller passes one vote slice that includes the history report ids. The
// votes are grouped by report once, so the work grows with history rows plus
// vote rows rather than their product (T-03-05). The result is display only
// (D-14). The none signal for accountless reports and for reports past the
// account cap is not produced here: it comes from a nil TrustInput.Reporter.
func ReporterSignals(accountIDs []int64, history []sqlcgen.ReporterHistoryReportsRow, votes []sqlcgen.CurrentVotesForReportsRow, now time.Time, opts ...SignalOption) map[int64]ReporterSignal {
	var cfg signalConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

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
		tally, ok := cfg.ratings[id]
		signals[id] = applyRatings(ScoreReliability(outcomes[id], now), tally, ok)
	}
	return signals
}
