// trustfacts_ratings_test.go pins the ratings line of the reporter row (plan
// 09-29, review W6, UI-SPEC P9-C29): from MinRatersForStars counted raters,
// when the ratings applied to the tag, the row's detail ends with Includes
// ratings by N people. Every other reporter row reads exactly as before. The
// tests are package service and need no database. Helpers start with p929 so
// they cannot collide with another plan's names.

package service

import (
	"reflect"
	"testing"
)

// p929Rated is a Reliable reporter signal whose tag follows tapped ratings by
// raters counted people.
func p929Rated(raters int) ReporterSignal {
	score, outcome, halves := 80, 60, 10
	return ReporterSignal{
		Tag: ReporterReliable, Score: &score,
		Label:         reporterLabel(ReporterReliable),
		Sentence:      reporterSentence(ReporterReliable, 8, 5),
		ScoreSentence: reporterScoreSentence(score),
		JudgedReports: 8, CorroboratedReports: 5,
		OutcomeScore: &outcome, Raters: raters, TappedHalves: &halves,
	}
}

// p929Reporter returns the reporter fact of the trust object built for sig.
func p929Reporter(t *testing.T, sig ReporterSignal) TrustFact {
	t.Helper()
	for _, f := range BuildTrust(p10TrustInput(VisibilityLive, &sig)).Facts {
		if f.Kind == FactReporter {
			return f
		}
	}
	t.Fatalf("no reporter fact for %+v", sig)
	return TrustFact{}
}

func TestReporterFactSaysRatingsIncluded(t *testing.T) {
	for _, raters := range []int{MinRatersForStars, 12, 150} {
		sig := p929Rated(raters)
		got := p929Reporter(t, sig)
		want := TrustFact{
			Kind: FactReporter, Tone: ToneGood, Title: "Reliable reporter",
			Detail:      "5 of their last 8 reports were confirmed. " + RatingsIncludedSentence(raters),
			ScoreDetail: "Score 80 out of 100.",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%d raters: fact\n got  %+v\n want %+v", raters, got, want)
		}
		p31Check(t, "ratings line", got)
	}

	if got := RatingsIncludedSentence(12); got != "Includes ratings by 12 people." {
		t.Errorf("RatingsIncludedSentence(12) = %q", got)
	}

	// The fact is the one in the trust object, so the signal itself is not
	// changed by the line: only the row carries it.
	sig := p929Rated(12)
	if sig.Sentence != "5 of their last 8 reports were confirmed." {
		t.Errorf("signal sentence changed: %q", sig.Sentence)
	}
}

func TestReporterFactWithoutRatingsUnchanged(t *testing.T) {
	score := 71
	notApplied := ReporterSignal{
		Tag: ReporterReliable, Score: &score,
		Label:         reporterLabel(ReporterReliable),
		Sentence:      reporterSentence(ReporterReliable, 5, 4),
		ScoreSentence: reporterScoreSentence(score),
		JudgedReports: 5, CorroboratedReports: 4,
	}
	halves := 6
	outcome := 70
	belowFive := p929Rated(4)
	belowFive.TappedHalves = nil   // the ratings did not apply below five raters
	belowFiveDirty := p929Rated(4) // even a stray tap count under the minimum adds no line
	belowFiveDirty.OutcomeScore = &outcome
	belowFiveDirty.TappedHalves = &halves

	newScore := ReporterSignal{
		Tag: ReporterNew, Label: reporterLabel(ReporterNew),
		Sentence: reporterSentence(ReporterNew, 2, 1),
	}

	cases := []struct {
		name string
		sig  ReporterSignal
	}{
		{"ratings did not apply, no raters", notApplied},
		{"fewer than five counted raters", belowFive},
		{"tap count set with fewer than five raters", belowFiveDirty},
		{"new reporter", newScore},
	}
	for _, c := range cases {
		got := p929Reporter(t, c.sig)
		if got.Detail != c.sig.Sentence {
			t.Errorf("%s: detail %q, want the signal sentence %q", c.name, got.Detail, c.sig.Sentence)
		}
		if got.Title != c.sig.Label || got.ScoreDetail != c.sig.ScoreSentence {
			t.Errorf("%s: title %q or score %q changed", c.name, got.Title, got.ScoreDetail)
		}
	}

	// A private profile whose ratings only lowered the points still has
	// TappedHalves set (09-13), so the line shows: the tag rests partly on
	// taps whoever can see the profile (D-22).
	private := p929Rated(5)
	got := p929Reporter(t, private)
	if got.Detail != private.Sentence+" Includes ratings by 5 people." {
		t.Errorf("applied private ratings: detail %q", got.Detail)
	}

	// The title, tone and score line are the same with and without the line.
	with := p929Reporter(t, p929Rated(12))
	without := p929Rated(12)
	without.TappedHalves, without.OutcomeScore, without.Raters = nil, nil, 0
	plain := p929Reporter(t, without)
	if with.Title != plain.Title || with.Tone != plain.Tone || with.ScoreDetail != plain.ScoreDetail {
		t.Errorf("the line changed more than the detail: with %+v, without %+v", with, plain)
	}
}
