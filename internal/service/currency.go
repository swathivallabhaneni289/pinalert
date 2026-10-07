// currency.go is the still current signal (TRUST-07, first half): a pure
// function of a report's confirm and dispute vote times, its lifetime, its
// visibility, its metadata and an injected now. It carries decisions D-05
// (the window and which places count), D-06 and R-01 (null below 2 places),
// D-07 and R-02 (the fade stage), D-08 (the 0 to 100 score), D-18 (the words)
// and R-03 and R-08 (nothing for Hidden, Retracted or expired reports).
//
// It reads only the buckets filterVoteRows produced, so there is no second
// cell derivation, and its threshold is IndependentAgreementThreshold, so
// there is no second threshold constant. The browser renders what this
// returns and holds no rule of its own. The signal is recomputed on every
// read and never stored (Phase 2 D-08), and it never changes a report's
// visibility (D-14): it is display data only.

package service

import (
	"slices"
	"time"
)

// CurrencyBand is the still current band of one report.
type CurrencyBand string

const (
	CurrencyUpToDate           CurrencyBand = "up_to_date"
	CurrencyGettingOld         CurrencyBand = "getting_old"
	CurrencyNeedsReconfirming  CurrencyBand = "needs_reconfirming"
	CurrencyNotYetCorroborated CurrencyBand = "not_yet_corroborated"
	CurrencyNotApplicable      CurrencyBand = "not_applicable"
)

// CurrencyBands is the ordered, canonical list of every valid band.
var CurrencyBands = []CurrencyBand{
	CurrencyUpToDate,
	CurrencyGettingOld,
	CurrencyNeedsReconfirming,
	CurrencyNotYetCorroborated,
	CurrencyNotApplicable,
}

// Valid reports whether b is one of the five canonical bands. Comparison is
// exact, with no case-insensitive coercion.
func (b CurrencyBand) Valid() bool {
	for _, x := range CurrencyBands {
		if x == b {
			return true
		}
	}
	return false
}

// FadeStage is how far the server says a row should dim (R-02).
type FadeStage string

const (
	FadeFresh FadeStage = "fresh"
	FadeAging FadeStage = "aging"
	FadeStale FadeStage = "stale"
)

// FadeStages is the ordered, canonical list of every valid fade stage.
var FadeStages = []FadeStage{FadeFresh, FadeAging, FadeStale}

// Valid reports whether f is one of the three canonical fade stages.
func (f FadeStage) Valid() bool {
	for _, x := range FadeStages {
		if x == f {
			return true
		}
	}
	return false
}

const (
	// CurrencyWindowDivisor divides a report's lifetime into the still
	// current window: one quarter of the lifetime (D-05).
	CurrencyWindowDivisor = 4
	// CurrencyPointsPerWindow is how many score points one window costs
	// (D-08). The score reaches 0 after CurrencyWindowDivisor windows.
	CurrencyPointsPerWindow = 25
	// UpToDateMinScore is the lowest score still read as Up to date.
	UpToDateMinScore = 75
	// GettingOldMinScore is the lowest score still read as Getting old.
	GettingOldMinScore = 50
	// currencyFullScore is the score of a place that confirmed just now.
	currencyFullScore = 100
)

// CurrencySignal is the still current signal of one report. 03-14 maps it to
// the wire.
type CurrencySignal struct {
	// Band is the still current band. Never empty.
	Band CurrencyBand
	// Score is 0 to 100, or nil below 2 confirming places and for a
	// not_applicable band (R-01, R-03). It is never stored.
	Score *int
	// Label is the server word for the band. Empty on Provisional rows, on
	// not_applicable rows, and for not_yet_corroborated unless the report
	// is critical or rescue needed (D-18).
	Label string
	// Sentence is the panel sentence. Empty exactly when Label is empty.
	Sentence string
	// ScoreSentence is the popup score sentence. Empty exactly when Score is
	// nil (A50).
	ScoreSentence string
	// RecentPlaces is the count of distinct confirming places inside the
	// window. It stays filled on not_applicable rows.
	RecentPlaces int
	// WindowMinutes is the window length rounded to whole minutes. It stays
	// filled on not_applicable rows, and is 0 for a non positive lifetime.
	WindowMinutes int
	// FadeStage is nil for critical and rescue needed reports and for
	// not_applicable rows (D-07, R-02, R-03).
	FadeStage *FadeStage
}

// currencyInput is everything buildCurrency reads. The two vote buckets must
// come from one filterVoteRows call, and Visibility from the caller's own
// Resolve. There is no description or other user text field here, so no
// report text can reach a currency string (T-03-06).
type currencyInput struct {
	Confirms, Disputes []placeVote
	Meta               ReportMeta
	Visibility         Visibility
	CreatedAt          time.Time
	ExpiresAt          time.Time
	Now                time.Time
}

// currencyScore is 100 minus the ceiling of CurrencyPointsPerWindow * age /
// window, with age clamped to between 0 and the whole lifetime (four
// windows). The arithmetic is integer nanoseconds with no float. Lifetimes
// come from ExpiryDuration (at most 24 hours), so the product cannot
// overflow an int64. A window that is not positive scores 0.
func currencyScore(age, window time.Duration) int {
	if window <= 0 {
		return 0
	}
	maxAge := window * (currencyFullScore / CurrencyPointsPerWindow)
	if age < 0 {
		age = 0
	}
	if age > maxAge {
		age = maxAge
	}
	lost := (int64(CurrencyPointsPerWindow)*int64(age) + int64(window) - 1) / int64(window)
	return currencyFullScore - int(lost)
}

// currencyBandForScore cuts the integer score into a band, so the word and
// the number cannot disagree.
func currencyBandForScore(score int) CurrencyBand {
	switch {
	case score >= UpToDateMinScore:
		return CurrencyUpToDate
	case score >= GettingOldMinScore:
		return CurrencyGettingOld
	default:
		return CurrencyNeedsReconfirming
	}
}

// fadeStageForBand maps a band to its fade stage (R-02). It returns a pointer
// to a fresh local copy, never to a package variable, so no caller can change
// a shared value.
func fadeStageForBand(b CurrencyBand) *FadeStage {
	var s FadeStage
	switch b {
	case CurrencyUpToDate, CurrencyNotYetCorroborated:
		s = FadeFresh
	case CurrencyGettingOld:
		s = FadeAging
	case CurrencyNeedsReconfirming:
		s = FadeStale
	default:
		return nil
	}
	return &s
}

// placeTimes returns one time per distinct cell, the latest CreatedAt among
// that cell's votes, sorted most recent first.
func placeTimes(votes []placeVote) []time.Time {
	latest := make(map[string]time.Time, len(votes))
	for _, v := range votes {
		if t, ok := latest[v.Cell]; !ok || v.CreatedAt.After(t) {
			latest[v.Cell] = v.CreatedAt
		}
	}
	times := make([]time.Time, 0, len(latest))
	for _, t := range latest {
		times = append(times, t)
	}
	slices.SortFunc(times, func(a, b time.Time) int { return b.Compare(a) })
	return times
}

// cellsSince counts the distinct places among votes cast at or after start.
func cellsSince(votes []placeVote, start time.Time) int {
	var cells []string
	for _, v := range votes {
		if !v.CreatedAt.Before(start) {
			cells = append(cells, v.Cell)
		}
	}
	return independentCellCount(cells)
}

// buildCurrency is the one pure function behind the still current signal. Its
// rules run in this order:
//
//  1. The window is the lifetime divided by CurrencyWindowDivisor. A window
//     that is not positive gives band not_applicable and nothing else. It is
//     unreachable through Submit and keeps the function total.
//  2. WindowMinutes is the window rounded to the nearest whole minute, and
//     RecentPlaces is the count of distinct confirming places inside the
//     exact window ending at Now. Both stay filled on every later return.
//  3. Hidden, Retracted and expired reports give not_applicable (R-03, R-08).
//  4. N is the distinct confirming places over the whole life, the same
//     number as Trust.ConfirmedPlaces.
//  5. Below IndependentAgreementThreshold places the band is
//     not_yet_corroborated with a nil score (D-06, R-01). Only critical and
//     rescue needed reports get a word, Too early to tell (D-18).
//  6. D is the distinct dispute places inside the window, and k is the
//     threshold plus D: each standing dispute place cancels one confirming
//     place.
//  7. The score is 0 when N is below k, otherwise it comes from the age of
//     the k-th most recent confirming place (D-08).
//  8. Unless the report is Provisional, the word and sentence follow the band
//     (D-19).
//  9. The fade stage follows the band, except for critical and rescue needed
//     reports (D-07, R-02).
func buildCurrency(in currencyInput) CurrencySignal {
	window := in.ExpiresAt.Sub(in.CreatedAt) / CurrencyWindowDivisor
	if window <= 0 {
		return CurrencySignal{Band: CurrencyNotApplicable}
	}

	sig := CurrencySignal{
		WindowMinutes: int((window + 30*time.Second) / time.Minute),
	}
	start := in.Now.Add(-window)
	sig.RecentPlaces = cellsSince(in.Confirms, start)

	if in.Visibility == VisibilityHidden || in.Visibility == VisibilityRetracted || !in.ExpiresAt.After(in.Now) {
		sig.Band = CurrencyNotApplicable
		return sig
	}

	n := independentCellCount(placeVoteCells(in.Confirms))
	critical := in.Meta.criticalBypass()

	if n < IndependentAgreementThreshold {
		sig.Band = CurrencyNotYetCorroborated
		if critical && in.Visibility != VisibilityProvisional {
			sig.Label = currencyLabelTooEarly
			sig.Sentence = tooEarlySentence()
		}
		if !critical {
			sig.FadeStage = fadeStageForBand(sig.Band)
		}
		return sig
	}

	dw := cellsSince(in.Disputes, start)
	k := IndependentAgreementThreshold + dw

	score := 0
	if n >= k {
		score = currencyScore(in.Now.Sub(placeTimes(in.Confirms)[k-1]), window)
	}
	sig.Band = currencyBandForScore(score)
	sig.Score = &score
	sig.ScoreSentence = currencyScoreSentence(score)

	if in.Visibility != VisibilityProvisional {
		phrase := windowPhrase(sig.WindowMinutes)
		sig.Label = currencyLabel(sig.Band)
		switch {
		case sig.Band == CurrencyUpToDate:
			sig.Sentence = upToDateSentence(sig.RecentPlaces, phrase)
		case dw > 0:
			sig.Sentence = disputeRaisedBarSentence(k, phrase)
		case sig.Band == CurrencyGettingOld:
			sig.Sentence = gettingOldSentence(phrase)
		default:
			sig.Sentence = needsReconfirmingSentence
		}
	}

	if !critical {
		sig.FadeStage = fadeStageForBand(sig.Band)
	}
	return sig
}
