package service

import "testing"

// ratedFixture is the Reliability a score of n and a Mixed or Reliable tag give,
// as ScoreReliability would produce it for 8 judged reports with 5 confirmed.
func ratedFixture(score int) Reliability {
	s := score
	return Reliability{Tag: reporterTagForScore(score), Score: &s, Judged: 8, Corroborated: 5}
}

// TestRatingsNeverRateANewReporter: a New reporter has fewer than five judged
// reports and no score, so tapped ratings cannot rate them however many there are
// (Phase 3 D-13, T-09-48).
func TestRatingsNeverRateANewReporter(t *testing.T) {
	accountIDs, history, votes := p10GroupFixture()
	tally := map[int64]RatingTally{
		105: {Raters: 40, TapSum: 200, ProfilePublic: true},
		106: {Raters: 40, TapSum: 200, ProfilePublic: true},
	}
	got := ReporterSignals(accountIDs, history, votes, p10Now, WithRatings(tally))
	for _, id := range []int64{105, 106} {
		sig := got[id]
		if sig.Tag != ReporterNew || sig.Score != nil || sig.OutcomeScore != nil || sig.Raters != 0 || sig.TappedHalves != nil {
			t.Errorf("account %d with 40 five star raters = %+v, want a plain New reporter", id, sig)
		}
	}
	// The same through applyRatings with a nil score.
	sig := applyRatings(Reliability{Tag: ReporterNew, Judged: 3}, RatingTally{Raters: 40, TapSum: 200, ProfilePublic: true}, true)
	if sig.Tag != ReporterNew || sig.Score != nil {
		t.Errorf("a reliability with no score became %+v", sig)
	}
}

// TestCombinedTagCut: the tag of a rated reporter is cut from the combined
// points by reporterTagForScore, so the score shown and the tag agree at every
// point of the scale (D-02, D-05).
func TestCombinedTagCut(t *testing.T) {
	// A Mixed record of 60 with five five star raters reads 80 and Reliable.
	sig := applyRatings(ratedFixture(60), RatingTally{Raters: 5, TapSum: 25, ProfilePublic: true}, true)
	if sig.Score == nil || *sig.Score != 80 || sig.Tag != ReporterReliable {
		t.Fatalf("60 with five fives = score %v tag %q, want 80 Reliable", p10Deref(sig.Score), sig.Tag)
	}
	if sig.OutcomeScore == nil || *sig.OutcomeScore != 60 || sig.Raters != 5 || sig.TappedHalves == nil || *sig.TappedHalves != 10 {
		t.Errorf("rated parts = outcome %v raters %d halves %v, want 60, 5 and 10", p10Deref(sig.OutcomeScore), sig.Raters, p10Deref(sig.TappedHalves))
	}
	if sig.Label != "Reliable reporter" || sig.ScoreSentence != "Score 80 out of 100." {
		t.Errorf("words = %q and %q, want Reliable reporter and Score 80 out of 100.", sig.Label, sig.ScoreSentence)
	}
	if sig.JudgedReports != 8 || sig.CorroboratedReports != 5 {
		t.Errorf("counts = %d and %d, want the history counts 8 and 5", sig.JudgedReports, sig.CorroboratedReports)
	}

	// Five one star ratings pull a Reliable 86 to 53, Mixed record.
	accountIDs, history, votes := p10GroupFixture()
	low := ReporterSignals(accountIDs, history, votes, p10Now, WithRatings(map[int64]RatingTally{
		101: {Raters: 5, TapSum: 5, ProfilePublic: true},
	}))[101]
	if low.Score == nil || *low.Score != 53 || low.Tag != ReporterMixed || low.Label != "Mixed record" {
		t.Errorf("86 with five ones = %+v, want 53 Mixed record", low)
	}

	// The tag and the score agree at every outcome and every tap sum.
	for outcome := 0; outcome <= 100; outcome++ {
		for taps := 5; taps <= 25; taps++ {
			s := applyRatings(ratedFixture(outcome), RatingTally{Raters: 5, TapSum: taps, ProfilePublic: true}, true)
			if s.Score == nil {
				t.Fatalf("outcome %d taps %d lost its score", outcome, taps)
			}
			if want := reporterTagForScore(*s.Score); s.Tag != want {
				t.Fatalf("outcome %d taps %d: score %d with tag %q, want %q", outcome, taps, *s.Score, s.Tag, want)
			}
		}
	}
}

// TestBelowFiveRatersOutcomesOnly: with fewer than five counted raters the
// signal is the outcome only signal, whatever the stars (D-02, review C1).
func TestBelowFiveRatersOutcomesOnly(t *testing.T) {
	accountIDs, history, votes := p10GroupFixture()
	base := ReporterSignals(accountIDs, history, votes, p10Now)
	for _, raters := range []int{0, 1, 4} {
		got := ReporterSignals(accountIDs, history, votes, p10Now, WithRatings(map[int64]RatingTally{
			101: {Raters: raters, TapSum: raters, ProfilePublic: true},
			102: {Raters: raters, TapSum: 5 * raters, ProfilePublic: true},
		}))
		for _, id := range []int64{101, 102} {
			sig := got[id]
			if *sig.Score != *base[id].Score || sig.Tag != base[id].Tag || sig.Raters != 0 || sig.OutcomeScore != nil || sig.TappedHalves != nil {
				t.Errorf("%d raters, account %d = %+v, want the outcome only signal %+v", raters, id, sig, base[id])
			}
		}
	}
}

// TestPrivateProfileRatingsOnlyLower: while the profile is off the ratings can
// only lower the points, so going private never raises the tag (D-22, T-09-97).
func TestPrivateProfileRatingsOnlyLower(t *testing.T) {
	// 60 Mixed with five fives would read 80 on a public profile, and stays 60
	// on a private one, with the ratings not applied.
	sig := applyRatings(ratedFixture(60), RatingTally{Raters: 5, TapSum: 25, ProfilePublic: false}, true)
	if sig.Score == nil || *sig.Score != 60 || sig.Tag != ReporterMixed || sig.Raters != 0 || sig.TappedHalves != nil || sig.OutcomeScore != nil {
		t.Errorf("a private 60 with five fives = %+v (score %v), want 60 Mixed with no rating parts", sig, p10Deref(sig.Score))
	}

	// 86 with five ones reads 53 on a private profile as well as on a public one.
	accountIDs, history, votes := p10GroupFixture()
	for _, public := range []bool{false, true} {
		got := ReporterSignals(accountIDs, history, votes, p10Now, WithRatings(map[int64]RatingTally{
			101: {Raters: 5, TapSum: 5, ProfilePublic: public},
		}))[101]
		if got.Score == nil || *got.Score != 53 || got.Tag != ReporterMixed || got.Raters != 5 {
			t.Errorf("public=%v: 86 with five ones = %+v (score %v), want 53 Mixed with 5 raters", public, got, p10Deref(got.Score))
		}
	}

	// A private profile never scores above the outcome, at any outcome and tap sum.
	for outcome := 0; outcome <= 100; outcome++ {
		for taps := 5; taps <= 25; taps++ {
			private := applyRatings(ratedFixture(outcome), RatingTally{Raters: 5, TapSum: taps, ProfilePublic: false}, true)
			if *private.Score > outcome {
				t.Fatalf("outcome %d taps %d: private points %d are above the outcome", outcome, taps, *private.Score)
			}
			public := applyRatings(ratedFixture(outcome), RatingTally{Raters: 5, TapSum: taps, ProfilePublic: true}, true)
			if *private.Score > *public.Score && *public.Score < outcome {
				t.Fatalf("outcome %d taps %d: private %d beats public %d while public lowers", outcome, taps, *private.Score, *public.Score)
			}
		}
	}
}

// TestReporterSignalForCopiesRatingFields: a built trust object never shares a
// pointer with its input, including the new rating fields.
func TestReporterSignalForCopiesRatingFields(t *testing.T) {
	sig := applyRatings(ratedFixture(60), RatingTally{Raters: 5, TapSum: 25, ProfilePublic: true}, true)
	out := reporterSignalFor(&sig)
	if out.OutcomeScore == sig.OutcomeScore || out.TappedHalves == sig.TappedHalves || out.Score == sig.Score {
		t.Fatal("reporterSignalFor shares a pointer with its input")
	}
	*out.OutcomeScore, *out.TappedHalves = 1, 1
	if *sig.OutcomeScore != 60 || *sig.TappedHalves != 10 {
		t.Errorf("changing the copy changed the input: %d and %d", *sig.OutcomeScore, *sig.TappedHalves)
	}
}
