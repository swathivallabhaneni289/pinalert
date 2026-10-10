// currency_test.go declares package service (the internal test package)
// because it reaches the unexported buildCurrency, currencyInput, placeVote
// and the copy builders. Every helper added here carries the p08 prefix so it
// cannot collide with another plan's helper in this package.

package service

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"
)

// p08Created is the creation time every fixture in this file starts from.
var p08Created = time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)

// p08At is p08Created plus h hours and m minutes.
func p08At(h, m int) time.Time {
	return p08Created.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
}

// p08Place builds one counted vote in the given cell at the given time.
func p08Place(cell string, at time.Time) placeVote {
	return placeVote{Cell: cell, CreatedAt: at}
}

// p08Input builds a currencyInput created at p08Created that lives for
// lifetime.
func p08Input(confirms, disputes []placeVote, meta ReportMeta, vis Visibility, lifetime time.Duration, now time.Time) currencyInput {
	return currencyInput{
		Confirms:   confirms,
		Disputes:   disputes,
		Meta:       meta,
		Visibility: vis,
		CreatedAt:  p08Created,
		ExpiresAt:  p08Created.Add(lifetime),
		Now:        now,
	}
}

var (
	p08Low       = ReportMeta{Severity: SeverityLow, Category: CategoryFlood}
	p08Medium    = ReportMeta{Severity: SeverityMedium, Category: CategoryFlood}
	p08Critical  = ReportMeta{Severity: SeverityCritical, Category: CategoryFlood}
	p08RescueLow = ReportMeta{Severity: SeverityLow, Category: CategoryRescueNeeded}
)

// p08Life8 and p08Life24 are the two lifetimes ExpiryDuration produces.
const (
	p08Life8  = 8 * time.Hour
	p08Life24 = 24 * time.Hour
)

// p08Score fails the test unless sig has a non nil Score, and returns it.
func p08Score(t *testing.T, sig CurrencySignal) int {
	t.Helper()
	if sig.Score == nil {
		t.Fatalf("Score = nil, want a number (band %q)", sig.Band)
	}
	return *sig.Score
}

// p08Fade returns the fade stage as a string, or "nil".
func p08Fade(f *FadeStage) string {
	if f == nil {
		return "nil"
	}
	return string(*f)
}

// p08Worked is the places of the worked example on the 8 hour report.
var p08Worked = []placeVote{
	p08Place("a", p08At(0, 30)),
	p08Place("b", p08At(1, 0)),
	p08Place("c", p08At(3, 0)),
}

// p08Until keeps only the votes cast at or before now. It is a fixture rule:
// buildCurrency itself never filters by now.
func p08Until(votes []placeVote, now time.Time) []placeVote {
	var out []placeVote
	for _, v := range votes {
		if !v.CreatedAt.After(now) {
			out = append(out, v)
		}
	}
	return out
}

// TestCurrencyScoreBoundaries is V-13: the score and band at every boundary
// for 8 hour and 24 hour reports, and the worked example.
func TestCurrencyScoreBoundaries(t *testing.T) {
	type row struct {
		name  string
		age   func(w time.Duration) time.Duration
		score int
		band  CurrencyBand
	}
	rows := []row{
		{"0", func(w time.Duration) time.Duration { return 0 }, 100, CurrencyUpToDate},
		{"1ns", func(w time.Duration) time.Duration { return 1 }, 99, CurrencyUpToDate},
		{"W/25", func(w time.Duration) time.Duration { return w / 25 }, 99, CurrencyUpToDate},
		{"W/25+1ns", func(w time.Duration) time.Duration { return w/25 + 1 }, 98, CurrencyUpToDate},
		{"W-1ns", func(w time.Duration) time.Duration { return w - 1 }, 75, CurrencyUpToDate},
		{"W", func(w time.Duration) time.Duration { return w }, 75, CurrencyUpToDate},
		{"W+1ns", func(w time.Duration) time.Duration { return w + 1 }, 74, CurrencyGettingOld},
		{"1.5W", func(w time.Duration) time.Duration { return w * 3 / 2 }, 62, CurrencyGettingOld},
		{"2W", func(w time.Duration) time.Duration { return 2 * w }, 50, CurrencyGettingOld},
		{"2W+1ns", func(w time.Duration) time.Duration { return 2*w + 1 }, 49, CurrencyNeedsReconfirming},
		{"3W", func(w time.Duration) time.Duration { return 3 * w }, 25, CurrencyNeedsReconfirming},
		{"4W", func(w time.Duration) time.Duration { return 4 * w }, 0, CurrencyNeedsReconfirming},
		{"5W", func(w time.Duration) time.Duration { return 5 * w }, 0, CurrencyNeedsReconfirming},
		{"5s in the future", func(w time.Duration) time.Duration { return -5 * time.Second }, 100, CurrencyUpToDate},
	}

	for _, w := range []time.Duration{2 * time.Hour, 6 * time.Hour} {
		for _, r := range rows {
			t.Run(fmt.Sprintf("pure/%s/%s", w, r.name), func(t *testing.T) {
				got := currencyScore(r.age(w), w)
				if got != r.score {
					t.Errorf("currencyScore = %d, want %d", got, r.score)
				}
				if band := currencyBandForScore(got); band != r.band {
					t.Errorf("band = %q, want %q", band, r.band)
				}
			})
		}
	}

	t.Run("a window of zero scores 0", func(t *testing.T) {
		if got := currencyScore(time.Minute, 0); got != 0 {
			t.Errorf("currencyScore with window 0 = %d, want 0", got)
		}
	})

	cases := []struct {
		name     string
		meta     ReportMeta
		lifetime time.Duration
		window   time.Duration
		minutes  int
	}{
		{"8 hour low", p08Low, p08Life8, 2 * time.Hour, 120},
		{"24 hour critical", p08Critical, p08Life24, 6 * time.Hour, 360},
	}
	for _, c := range cases {
		for _, r := range rows {
			if r.name == "4W" || r.name == "5W" {
				continue
			}
			t.Run(fmt.Sprintf("build/%s/%s", c.name, r.name), func(t *testing.T) {
				age := r.age(c.window)
				now := p08Created.Add(3*c.window + time.Minute)
				at := now.Add(-age)
				sig := buildCurrency(p08Input(
					[]placeVote{p08Place("a", at), p08Place("b", at)},
					nil, c.meta, VisibilityLive, c.lifetime, now))
				if got := p08Score(t, sig); got != r.score {
					t.Errorf("Score = %d, want %d", got, r.score)
				}
				if sig.Band != r.band {
					t.Errorf("Band = %q, want %q", sig.Band, r.band)
				}
				if want := currencyLabel(r.band); sig.Label != want {
					t.Errorf("Label = %q, want %q", sig.Label, want)
				}
				wantFade := "nil"
				if c.meta == p08Low {
					wantFade = string(*fadeStageForBand(r.band))
				}
				if got := p08Fade(sig.FadeStage); got != wantFade {
					t.Errorf("FadeStage = %s, want %s", got, wantFade)
				}
				wantRecent := 0
				if age <= c.window {
					wantRecent = 2
				}
				if sig.RecentPlaces != wantRecent {
					t.Errorf("RecentPlaces = %d, want %d", sig.RecentPlaces, wantRecent)
				}
				if sig.WindowMinutes != c.minutes {
					t.Errorf("WindowMinutes = %d, want %d", sig.WindowMinutes, c.minutes)
				}
			})
		}
	}

	t.Run("worked example", func(t *testing.T) {
		type step struct {
			now    time.Time
			n      int
			score  *int
			band   CurrencyBand
			recent int
		}
		i := func(v int) *int { return &v }
		steps := []step{
			{p08At(0, 45), 1, nil, CurrencyNotYetCorroborated, 1},
			{p08At(1, 0), 2, i(93), CurrencyUpToDate, 2},
			{p08At(1, 30), 2, i(87), CurrencyUpToDate, 2},
			{p08At(3, 0), 3, i(75), CurrencyUpToDate, 2},
			{p08At(3, 30), 3, i(68), CurrencyGettingOld, 1},
			{p08At(5, 0), 3, i(50), CurrencyGettingOld, 1},
			{p08At(6, 0), 3, i(37), CurrencyNeedsReconfirming, 0},
			{p08At(7, 59), 3, i(12), CurrencyNeedsReconfirming, 0},
		}
		for _, s := range steps {
			t.Run(s.now.Format("15:04"), func(t *testing.T) {
				confirms := p08Until(p08Worked, s.now)
				if n := independentCellCount(placeVoteCells(confirms)); n != s.n {
					t.Fatalf("N = %d, want %d", n, s.n)
				}
				sig := buildCurrency(p08Input(confirms, nil, p08Low, VisibilityLive, p08Life8, s.now))
				if !reflect.DeepEqual(sig.Score, s.score) {
					t.Errorf("Score = %v, want %v", sig.Score, s.score)
				}
				if sig.Band != s.band {
					t.Errorf("Band = %q, want %q", sig.Band, s.band)
				}
				if sig.RecentPlaces != s.recent {
					t.Errorf("RecentPlaces = %d, want %d", sig.RecentPlaces, s.recent)
				}
			})
		}

		sentenceAt := func(now time.Time) CurrencySignal {
			return buildCurrency(p08Input(p08Until(p08Worked, now), nil, p08Low, VisibilityLive, p08Life8, now))
		}
		if got, want := sentenceAt(p08At(1, 0)).Sentence, "2 confirmations in the last 2 hours."; got != want {
			t.Errorf("11:00 Sentence = %q, want %q", got, want)
		}
		if got, want := sentenceAt(p08At(3, 30)).Sentence, "Fewer than 2 confirmations in the last 2 hours."; got != want {
			t.Errorf("13:30 Sentence = %q, want %q", got, want)
		}
		if got, want := sentenceAt(p08At(6, 0)).Sentence, "Gone quiet. People nearby can confirm it again."; got != want {
			t.Errorf("16:00 Sentence = %q, want %q", got, want)
		}
		if got, want := sentenceAt(p08At(1, 0)).ScoreSentence, "Score 93 out of 100."; got != want {
			t.Errorf("11:00 ScoreSentence = %q, want %q", got, want)
		}
	})
}

// TestCurrencyNotYetCorroborated is V-14: below 2 distinct places over the
// whole life the score is null, never 0, and only critical and rescue needed
// reports get a word.
func TestCurrencyNotYetCorroborated(t *testing.T) {
	now := p08At(3, 0)
	one := []placeVote{p08Place("a", now)}

	check := func(t *testing.T, sig CurrencySignal, label, sentence, fade string) {
		t.Helper()
		if sig.Band != CurrencyNotYetCorroborated {
			t.Errorf("Band = %q, want not_yet_corroborated", sig.Band)
		}
		if sig.Score != nil {
			t.Errorf("Score = %d, want nil", *sig.Score)
		}
		if sig.Label != label || sig.Sentence != sentence {
			t.Errorf("Label, Sentence = %q, %q, want %q, %q", sig.Label, sig.Sentence, label, sentence)
		}
		if sig.ScoreSentence != "" {
			t.Errorf("ScoreSentence = %q, want empty", sig.ScoreSentence)
		}
		if got := p08Fade(sig.FadeStage); got != fade {
			t.Errorf("FadeStage = %s, want %s", got, fade)
		}
	}

	t.Run("low provisional", func(t *testing.T) {
		check(t, buildCurrency(p08Input(nil, nil, p08Low, VisibilityProvisional, p08Life8, now)), "", "", "fresh")
		check(t, buildCurrency(p08Input(one, nil, p08Low, VisibilityProvisional, p08Life8, now)), "", "", "fresh")
	})
	t.Run("critical live", func(t *testing.T) {
		const word, line = "Too early to tell", "Fewer than 2 people have confirmed this yet."
		check(t, buildCurrency(p08Input(nil, nil, p08Critical, VisibilityLive, p08Life24, now)), word, line, "nil")
		check(t, buildCurrency(p08Input(one, nil, p08Critical, VisibilityLive, p08Life24, now)), word, line, "nil")
	})
	t.Run("low rescue needed live", func(t *testing.T) {
		const word, line = "Too early to tell", "Fewer than 2 people have confirmed this yet."
		check(t, buildCurrency(p08Input(one, nil, p08RescueLow, VisibilityLive, p08Life8, now)), word, line, "nil")
	})
	t.Run("disputes never turn null into 0", func(t *testing.T) {
		disputes := []placeVote{p08Place("x", now), p08Place("y", now), p08Place("z", now)}
		const word, line = "Too early to tell", "Fewer than 2 people have confirmed this yet."
		check(t, buildCurrency(p08Input(one, disputes, p08Critical, VisibilityLive, p08Life24, now)), word, line, "nil")
	})
	t.Run("two accounts in one cell are one place", func(t *testing.T) {
		same := []placeVote{p08Place("a", now), p08Place("a", now)}
		check(t, buildCurrency(p08Input(same, nil, p08Low, VisibilityLive, p08Life8, now)), "", "", "fresh")
	})
	t.Run("whole life, not window", func(t *testing.T) {
		old := []placeVote{p08Place("a", p08At(0, 1)), p08Place("b", p08At(0, 1))}
		sig := buildCurrency(p08Input(old, nil, p08Low, VisibilityLive, p08Life8, p08At(7, 0)))
		if got := p08Score(t, sig); got != 12 {
			t.Errorf("Score = %d, want 12", got)
		}
		if sig.Band != CurrencyNeedsReconfirming {
			t.Errorf("Band = %q, want needs_reconfirming", sig.Band)
		}
	})
}

// TestCurrencyRetapCannotRefresh is V-15: one account re-tapping Confirm
// moves only its own place's time, so it cannot refresh the signal.
func TestCurrencyRetapCannotRefresh(t *testing.T) {
	now := p08At(5, 0)
	build := func(confirms ...placeVote) CurrencySignal {
		return buildCurrency(p08Input(confirms, nil, p08Low, VisibilityLive, p08Life8, now))
	}

	t.Run("base", func(t *testing.T) {
		sig := build(p08Place("a", p08At(0, 30)), p08Place("b", p08At(0, 30)))
		if got := p08Score(t, sig); got != 43 {
			t.Errorf("Score = %d, want 43", got)
		}
		if sig.Band != CurrencyNeedsReconfirming || sig.RecentPlaces != 0 {
			t.Errorf("Band, RecentPlaces = %q, %d, want needs_reconfirming, 0", sig.Band, sig.RecentPlaces)
		}
	})
	t.Run("re-tap at now", func(t *testing.T) {
		sig := build(p08Place("a", now), p08Place("b", p08At(0, 30)))
		if got := p08Score(t, sig); got != 43 {
			t.Errorf("Score = %d, want 43", got)
		}
		if sig.Band != CurrencyNeedsReconfirming || sig.Label != "Report needs re-confirming" || sig.RecentPlaces != 1 {
			t.Errorf("Band, Label, RecentPlaces = %q, %q, %d", sig.Band, sig.Label, sig.RecentPlaces)
		}
	})
	t.Run("every re-tap time", func(t *testing.T) {
		for at := p08At(0, 30); !at.After(now); at = at.Add(10 * time.Minute) {
			sig := build(p08Place("a", at), p08Place("b", p08At(0, 30)))
			if got := p08Score(t, sig); got != 43 || sig.Band == CurrencyUpToDate {
				t.Errorf("re-tap at %s: Score = %d, Band = %q", at.Format("15:04"), got, sig.Band)
			}
		}
	})
	t.Run("more accounts in the same cell", func(t *testing.T) {
		sig := build(p08Place("a", now), p08Place("a", now), p08Place("a", now), p08Place("b", p08At(0, 30)))
		if got := p08Score(t, sig); got != 43 || sig.RecentPlaces != 1 {
			t.Errorf("Score, RecentPlaces = %d, %d, want 43, 1", got, sig.RecentPlaces)
		}
	})
	t.Run("a new place can", func(t *testing.T) {
		sig := build(p08Place("a", now), p08Place("c", now), p08Place("b", p08At(0, 30)))
		if got := p08Score(t, sig); got != 100 {
			t.Errorf("Score = %d, want 100", got)
		}
		if sig.Band != CurrencyUpToDate || sig.RecentPlaces != 2 {
			t.Errorf("Band, RecentPlaces = %q, %d", sig.Band, sig.RecentPlaces)
		}
		if want := "2 confirmations in the last 2 hours."; sig.Sentence != want {
			t.Errorf("Sentence = %q, want %q", sig.Sentence, want)
		}
	})
}

// TestCurrencyDisputes is V-16: a standing dispute place inside the window
// raises the bar by one place and never refreshes the signal.
func TestCurrencyDisputes(t *testing.T) {
	a, b, c := p08Place("a", p08At(0, 30)), p08Place("b", p08At(1, 0)), p08Place("c", p08At(3, 0))
	x, y := p08Place("x", p08At(1, 30)), p08Place("y", p08At(2, 0))
	low := func(confirms, disputes []placeVote, now time.Time) CurrencySignal {
		return buildCurrency(p08Input(confirms, disputes, p08Low, VisibilityLive, p08Life8, now))
	}
	const quiet = "Fewer than 2 confirmations in the last 2 hours."

	t.Run("contested", func(t *testing.T) {
		sig := low([]placeVote{a, b}, []placeVote{x}, p08At(1, 30))
		if got := p08Score(t, sig); got != 0 {
			t.Errorf("Score = %d, want 0", got)
		}
		if sig.Band != CurrencyNeedsReconfirming || sig.Label != "Report needs re-confirming" {
			t.Errorf("Band, Label = %q, %q", sig.Band, sig.Label)
		}
		if want := "Disputed nearby, so it needs 3 confirmations in the last 2 hours."; sig.Sentence != want {
			t.Errorf("Sentence = %q, want %q", sig.Sentence, want)
		}
		if got := p08Fade(sig.FadeStage); got != "stale" {
			t.Errorf("FadeStage = %s, want stale", got)
		}
		if sig.RecentPlaces != 2 {
			t.Errorf("RecentPlaces = %d, want 2", sig.RecentPlaces)
		}
		if got := p08Score(t, low([]placeVote{a, b}, []placeVote{x}, p08At(3, 30))); got != 0 {
			t.Errorf("at 13:30 Score = %d, want 0", got)
		}
	})
	t.Run("the dispute leaves the window", func(t *testing.T) {
		sig := low([]placeVote{a, b}, []placeVote{x}, p08At(3, 31))
		if got := p08Score(t, sig); got != 62 {
			t.Errorf("Score = %d, want 62", got)
		}
		if sig.Band != CurrencyGettingOld || sig.Sentence != quiet {
			t.Errorf("Band, Sentence = %q, %q", sig.Band, sig.Sentence)
		}
	})
	t.Run("three places and one dispute", func(t *testing.T) {
		sig := low([]placeVote{a, b, c}, []placeVote{x}, p08At(3, 0))
		if got := p08Score(t, sig); got != 68 {
			t.Errorf("Score = %d, want 68", got)
		}
		if want := "Disputed nearby, so it needs 3 confirmations in the last 2 hours."; sig.Band != CurrencyGettingOld || sig.Sentence != want || sig.RecentPlaces != 2 {
			t.Errorf("Band, Sentence, RecentPlaces = %q, %q, %d", sig.Band, sig.Sentence, sig.RecentPlaces)
		}
		clean := low([]placeVote{a, b, c}, nil, p08At(3, 0))
		if got := p08Score(t, clean); got != 75 || clean.Band != CurrencyUpToDate {
			t.Errorf("without the dispute: Score, Band = %d, %q, want 75 up_to_date", got, clean.Band)
		}
	})
	t.Run("two dispute accounts in one cell count once", func(t *testing.T) {
		x2 := p08Place("x", p08At(2, 0))
		if got := p08Score(t, low([]placeVote{a, b, c}, []placeVote{x, x2}, p08At(3, 0))); got != 68 {
			t.Errorf("Score = %d, want 68", got)
		}
	})
	t.Run("two dispute places", func(t *testing.T) {
		sig := low([]placeVote{a, b, c}, []placeVote{x, y}, p08At(3, 0))
		if got := p08Score(t, sig); got != 0 || sig.Band != CurrencyNeedsReconfirming {
			t.Errorf("Score, Band = %d, %q, want 0 needs_reconfirming", got, sig.Band)
		}
		if want := "Disputed nearby, so it needs 4 confirmations in the last 2 hours."; sig.Sentence != want {
			t.Errorf("Sentence = %q, want %q", sig.Sentence, want)
		}
	})
	t.Run("a dispute older than the window", func(t *testing.T) {
		old := p08Place("x", p08At(0, 45))
		sig := low([]placeVote{a, b, c}, []placeVote{old}, p08At(3, 0))
		if got := p08Score(t, sig); got != 75 || sig.Band != CurrencyUpToDate {
			t.Errorf("Score, Band = %d, %q, want 75 up_to_date", got, sig.Band)
		}
	})
	t.Run("a dispute never refreshes", func(t *testing.T) {
		sig := low([]placeVote{a, b}, nil, p08At(6, 0))
		if got := p08Score(t, sig); got != 31 {
			t.Errorf("Score = %d, want 31", got)
		}
		if want := "Gone quiet. People nearby can confirm it again."; sig.Sentence != want {
			t.Errorf("Sentence = %q, want %q", sig.Sentence, want)
		}
		fresh := p08Place("x", p08At(6, 0))
		if got := p08Score(t, low([]placeVote{a, b}, []placeVote{fresh}, p08At(6, 0))); got != 0 {
			t.Errorf("with a fresh dispute Score = %d, want 0", got)
		}
	})
	t.Run("critical with a 24 hour lifetime", func(t *testing.T) {
		sig := buildCurrency(p08Input([]placeVote{a, b}, []placeVote{x}, p08Critical, VisibilityLive, p08Life24, p08At(1, 30)))
		if got := p08Score(t, sig); got != 0 {
			t.Errorf("Score = %d, want 0", got)
		}
		if sig.Band != CurrencyNeedsReconfirming || sig.Label != "Report needs re-confirming" || sig.FadeStage != nil {
			t.Errorf("Band, Label, FadeStage = %q, %q, %v", sig.Band, sig.Label, sig.FadeStage)
		}
		if want := "Disputed nearby, so it needs 3 confirmations in the last 6 hours."; sig.Sentence != want {
			t.Errorf("Sentence = %q, want %q", sig.Sentence, want)
		}
	})
}

// TestCurrencyWindowFollowsLifetime is V-17: the window is a quarter of the
// lifetime, whatever the severity or category.
func TestCurrencyWindowFollowsLifetime(t *testing.T) {
	now := p08At(1, 0)
	two := []placeVote{p08Place("a", now), p08Place("b", now)}

	cases := []struct {
		name    string
		meta    ReportMeta
		minutes int
		phrase  string
		fade    string
	}{
		{"low", p08Low, 120, "the last 2 hours.", "fresh"},
		{"medium", p08Medium, 120, "the last 2 hours.", "fresh"},
		{"critical", p08Critical, 360, "the last 6 hours.", "nil"},
		{"low rescue needed", p08RescueLow, 120, "the last 2 hours.", "nil"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sig := buildCurrency(p08Input(two, nil, c.meta, VisibilityLive, ExpiryDuration(c.meta.Severity), now))
			if sig.WindowMinutes != c.minutes {
				t.Errorf("WindowMinutes = %d, want %d", sig.WindowMinutes, c.minutes)
			}
			if len(sig.Sentence) < len(c.phrase) || sig.Sentence[len(sig.Sentence)-len(c.phrase):] != c.phrase {
				t.Errorf("Sentence = %q, want it to end in %q", sig.Sentence, c.phrase)
			}
			if got := p08Fade(sig.FadeStage); got != c.fade {
				t.Errorf("FadeStage = %s, want %s", got, c.fade)
			}
		})
	}

	t.Run("two places two hours old", func(t *testing.T) {
		old := []placeVote{p08Place("a", p08At(1, 0)), p08Place("b", p08At(1, 0))}
		at := p08At(3, 0)
		if got := p08Score(t, buildCurrency(p08Input(old, nil, p08Low, VisibilityLive, p08Life8, at))); got != 75 {
			t.Errorf("8 hour Score = %d, want 75", got)
		}
		if got := p08Score(t, buildCurrency(p08Input(old, nil, p08Critical, VisibilityLive, p08Life24, at))); got != 91 {
			t.Errorf("24 hour Score = %d, want 91", got)
		}
	})

	t.Run("windowPhrase", func(t *testing.T) {
		want := map[int]string{0: "0 minutes", 1: "1 minute", 45: "45 minutes", 60: "1 hour", 90: "90 minutes", 120: "2 hours", 360: "6 hours"}
		for minutes, phrase := range want {
			if got := windowPhrase(minutes); got != phrase {
				t.Errorf("windowPhrase(%d) = %q, want %q", minutes, got, phrase)
			}
		}
	})

	t.Run("WindowMinutes rounds to the nearest minute", func(t *testing.T) {
		for _, life := range []time.Duration{p08Life8 - 5*time.Millisecond, p08Life8 + 5*time.Millisecond} {
			sig := buildCurrency(p08Input(two, nil, p08Low, VisibilityLive, life, now))
			if sig.WindowMinutes != 120 {
				t.Errorf("lifetime %s: WindowMinutes = %d, want 120", life, sig.WindowMinutes)
			}
			if want := "2 confirmations in the last 2 hours."; sig.Sentence != want {
				t.Errorf("lifetime %s: Sentence = %q, want %q", life, sig.Sentence, want)
			}
		}
	})

	t.Run("the window edge", func(t *testing.T) {
		at := p08At(3, 0)
		edge := p08Place("a", at.Add(-2*time.Hour))
		older := p08Place("b", at.Add(-2*time.Hour-time.Nanosecond))
		if got := buildCurrency(p08Input([]placeVote{edge}, nil, p08Low, VisibilityLive, p08Life8, at)).RecentPlaces; got != 1 {
			t.Errorf("a place exactly one window old: RecentPlaces = %d, want 1", got)
		}
		if got := buildCurrency(p08Input([]placeVote{older}, nil, p08Low, VisibilityLive, p08Life8, at)).RecentPlaces; got != 0 {
			t.Errorf("a place 1 ns older: RecentPlaces = %d, want 0", got)
		}
	})

	t.Run("a lifetime that is not positive", func(t *testing.T) {
		sig := buildCurrency(p08Input(two, nil, p08Low, VisibilityLive, 0, p08Created.Add(-time.Minute)))
		if sig.Band != CurrencyNotApplicable || sig.WindowMinutes != 0 || sig.RecentPlaces != 0 || sig.Score != nil || sig.FadeStage != nil {
			t.Errorf("signal = %+v, want not_applicable with nothing else", sig)
		}
	})
}

// TestCurrencyMonotone is V-18: for a fixed now, adding a confirm never
// lowers the score and adding a dispute never raises it. With no dispute
// rows, advancing now never raises it.
func TestCurrencyMonotone(t *testing.T) {
	rng := rand.New(rand.NewPCG(8, 3))
	confirmCells := []string{"c1", "c2", "c3", "c4", "c5", "c6"}
	disputeCells := []string{"d1", "d2", "d3"}
	metas := []ReportMeta{p08Low, p08Medium, p08Critical, p08RescueLow}

	between := func(lo, hi time.Time) time.Time {
		span := int64(hi.Sub(lo))
		if span <= 0 {
			return lo
		}
		return lo.Add(time.Duration(rng.Int64N(span + 1)))
	}
	votes := func(cells []string, n int, lo, hi time.Time) []placeVote {
		out := make([]placeVote, 0, n)
		for j := 0; j < n; j++ {
			out = append(out, p08Place(cells[rng.IntN(len(cells))], between(lo, hi)))
		}
		return out
	}

	for iter := 0; iter < 2000; iter++ {
		life := p08Life8
		if rng.IntN(2) == 0 {
			life = p08Life24
		}
		meta := metas[rng.IntN(len(metas))]
		expires := p08Created.Add(life)
		last := expires.Add(-time.Nanosecond)
		t1 := between(p08Created, last)
		t2 := between(t1, last)

		confirms := votes(confirmCells, rng.IntN(9), p08Created, t1)
		disputes := votes(disputeCells, rng.IntN(4), p08Created, t1)
		at := func(c, d []placeVote, now time.Time) CurrencySignal {
			return buildCurrency(p08Input(c, d, meta, VisibilityLive, life, now))
		}
		s1, s2 := at(confirms, nil, t1), at(confirms, nil, t2)

		// (a) has no disputes on purpose: a dispute that leaves the window
		// lowers the bar, so time alone can raise a contested score (0 at
		// 13:30 and 62 at 13:31 in TestCurrencyDisputes).
		if (s1.Score == nil) != (s2.Score == nil) {
			t.Fatalf("iteration %d: null changed between %s and %s", iter, t1, t2)
		}
		if s1.Score != nil && *s2.Score > *s1.Score {
			t.Fatalf("iteration %d: score rose from %d to %d as now advanced", iter, *s1.Score, *s2.Score)
		}

		full := at(confirms, disputes, t1)

		// (b) one more confirm at t1.
		extra := p08Place(confirmCells[rng.IntN(len(confirmCells))], between(p08Created, t1))
		more := at(append(append([]placeVote(nil), confirms...), extra), disputes, t1)
		if full.Score != nil && (more.Score == nil || *more.Score < *full.Score) {
			t.Fatalf("iteration %d: adding a confirm lowered %v to %v", iter, full.Score, more.Score)
		}

		// (c) one more dispute at t1.
		dispute := p08Place(disputeCells[rng.IntN(len(disputeCells))], between(p08Created, t1))
		worse := at(confirms, append(append([]placeVote(nil), disputes...), dispute), t1)
		if (full.Score == nil) != (worse.Score == nil) {
			t.Fatalf("iteration %d: adding a dispute changed null", iter)
		}
		if full.Score != nil && *worse.Score > *full.Score {
			t.Fatalf("iteration %d: adding a dispute raised %d to %d", iter, *full.Score, *worse.Score)
		}

		// (d) order never matters.
		shuffledC := append([]placeVote(nil), confirms...)
		shuffledD := append([]placeVote(nil), disputes...)
		rng.Shuffle(len(shuffledC), func(i, j int) { shuffledC[i], shuffledC[j] = shuffledC[j], shuffledC[i] })
		rng.Shuffle(len(shuffledD), func(i, j int) { shuffledD[i], shuffledD[j] = shuffledD[j], shuffledD[i] })
		if again := at(shuffledC, shuffledD, t1); !reflect.DeepEqual(full, again) {
			t.Fatalf("iteration %d: shuffling changed the signal:\n%+v\n%+v", iter, full, again)
		}

		// (e) invariants on every result.
		for _, sig := range []CurrencySignal{s1, s2, full, more, worse} {
			if sig.Score != nil {
				if *sig.Score < 0 || *sig.Score > 100 {
					t.Fatalf("iteration %d: Score = %d outside 0 to 100", iter, *sig.Score)
				}
				if sig.Band != currencyBandForScore(*sig.Score) {
					t.Fatalf("iteration %d: Band %q does not match Score %d", iter, sig.Band, *sig.Score)
				}
			}
			if sig.Band == CurrencyUpToDate && sig.RecentPlaces < IndependentAgreementThreshold {
				t.Fatalf("iteration %d: up_to_date with RecentPlaces = %d", iter, sig.RecentPlaces)
			}
			if (sig.ScoreSentence == "") != (sig.Score == nil) {
				t.Fatalf("iteration %d: ScoreSentence %q and Score %v disagree", iter, sig.ScoreSentence, sig.Score)
			}
			if (sig.Sentence == "") != (sig.Label == "") {
				t.Fatalf("iteration %d: Sentence %q and Label %q disagree", iter, sig.Sentence, sig.Label)
			}
		}
	}
}

// TestFadeStageMapping is V-19: the closed sets, the band to stage mapping,
// the critical bypass and the not applicable cases.
func TestFadeStageMapping(t *testing.T) {
	t.Run("closed sets", func(t *testing.T) {
		wantBands := []CurrencyBand{"up_to_date", "getting_old", "needs_reconfirming", "not_yet_corroborated", "not_applicable"}
		if !reflect.DeepEqual(CurrencyBands, wantBands) {
			t.Errorf("CurrencyBands = %v, want %v", CurrencyBands, wantBands)
		}
		for _, b := range CurrencyBands {
			if !b.Valid() {
				t.Errorf("%q should be valid", b)
			}
		}
		for _, b := range []CurrencyBand{"", "UP_TO_DATE", "stale"} {
			if b.Valid() {
				t.Errorf("%q should not be valid", b)
			}
		}
		wantStages := []FadeStage{"fresh", "aging", "stale"}
		if !reflect.DeepEqual(FadeStages, wantStages) {
			t.Errorf("FadeStages = %v, want %v", FadeStages, wantStages)
		}
		for _, f := range FadeStages {
			if !f.Valid() {
				t.Errorf("%q should be valid", f)
			}
		}
		for _, f := range []FadeStage{"", "Fresh", "up_to_date"} {
			if f.Valid() {
				t.Errorf("%q should not be valid", f)
			}
		}
	})

	t.Run("fadeStageForBand", func(t *testing.T) {
		want := map[CurrencyBand]string{
			CurrencyUpToDate: "fresh", CurrencyGettingOld: "aging", CurrencyNeedsReconfirming: "stale",
			CurrencyNotYetCorroborated: "fresh", CurrencyNotApplicable: "nil",
		}
		for band, stage := range want {
			if got := p08Fade(fadeStageForBand(band)); got != stage {
				t.Errorf("fadeStageForBand(%q) = %s, want %s", band, got, stage)
			}
		}
		if fadeStageForBand(CurrencyUpToDate) == fadeStageForBand(CurrencyUpToDate) {
			t.Error("two calls returned the same pointer")
		}
	})

	times := []time.Time{p08At(3, 0), p08At(3, 30), p08At(6, 0)}
	build := func(meta ReportMeta, life time.Duration, vis Visibility, now time.Time) CurrencySignal {
		return buildCurrency(p08Input(p08Until(p08Worked, now), nil, meta, vis, life, now))
	}

	t.Run("low and medium follow the band", func(t *testing.T) {
		for _, meta := range []ReportMeta{p08Low, p08Medium} {
			for i, want := range []string{"fresh", "aging", "stale"} {
				if got := p08Fade(build(meta, p08Life8, VisibilityLive, times[i]).FadeStage); got != want {
					t.Errorf("%v at %s: FadeStage = %s, want %s", meta, times[i].Format("15:04"), got, want)
				}
			}
		}
		one := []placeVote{p08Place("a", p08At(0, 30))}
		sig := buildCurrency(p08Input(one, nil, p08Low, VisibilityProvisional, p08Life8, p08At(1, 0)))
		if got := p08Fade(sig.FadeStage); got != "fresh" {
			t.Errorf("provisional with one place: FadeStage = %s, want fresh", got)
		}
	})

	t.Run("critical and rescue needed keep the word and lose the fade", func(t *testing.T) {
		for _, c := range []struct {
			meta ReportMeta
			life time.Duration
		}{{p08Critical, p08Life24}, {p08RescueLow, p08Life8}} {
			for _, now := range times {
				sig := build(c.meta, c.life, VisibilityLive, now)
				if sig.FadeStage != nil {
					t.Errorf("%v at %s: FadeStage = %s, want nil", c.meta, now.Format("15:04"), p08Fade(sig.FadeStage))
				}
				if sig.Label == "" || sig.Score == nil {
					t.Errorf("%v at %s: Label %q and Score %v should be set", c.meta, now.Format("15:04"), sig.Label, sig.Score)
				}
			}
		}
	})

	t.Run("not applicable", func(t *testing.T) {
		all := p08Worked[:2]
		disputes := []placeVote{p08Place("x", p08At(1, 0)), p08Place("y", p08At(1, 15)), p08Place("z", p08At(1, 30))}
		expires := p08Created.Add(p08Life8)
		cases := []struct {
			name   string
			in     currencyInput
			recent int
		}{
			{"hidden", p08Input(all, disputes, p08Low, VisibilityHidden, p08Life8, p08At(1, 30)), 2},
			{"retracted", p08Input(all, disputes, p08Low, VisibilityRetracted, p08Life8, p08At(1, 30)), 2},
			{"live at expiry", p08Input(all, nil, p08Low, VisibilityLive, p08Life8, expires), 0},
			{"provisional at expiry", p08Input(all[:1], nil, p08Low, VisibilityProvisional, p08Life8, expires), 0},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				sig := buildCurrency(c.in)
				if sig.Band != CurrencyNotApplicable || sig.Score != nil || sig.FadeStage != nil {
					t.Errorf("Band, Score, FadeStage = %q, %v, %v", sig.Band, sig.Score, sig.FadeStage)
				}
				if sig.Label != "" || sig.Sentence != "" || sig.ScoreSentence != "" {
					t.Errorf("strings = %q, %q, %q, want all empty", sig.Label, sig.Sentence, sig.ScoreSentence)
				}
				if sig.WindowMinutes != 120 || sig.RecentPlaces != c.recent {
					t.Errorf("WindowMinutes, RecentPlaces = %d, %d, want 120, %d", sig.WindowMinutes, sig.RecentPlaces, c.recent)
				}
			})
		}
		t.Run("one nanosecond before expiry is applicable", func(t *testing.T) {
			late := []placeVote{p08Place("a", p08At(6, 30)), p08Place("b", p08At(7, 0))}
			sig := buildCurrency(p08Input(late, nil, p08Low, VisibilityLive, p08Life8, expires.Add(-time.Nanosecond)))
			if sig.Band != CurrencyUpToDate || sig.RecentPlaces != 2 {
				t.Errorf("Band, RecentPlaces = %q, %d, want up_to_date, 2", sig.Band, sig.RecentPlaces)
			}
		})
	})
}
