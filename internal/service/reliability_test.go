package service_test

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

	"pinalert/internal/service"
)

// relNow is the fixed clock every reliability test injects.
var relNow = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

// relZeroResolution returns tally with its four resolution fields cleared,
// the shape the classifier hands to Resolve.
func relZeroResolution(tally service.VoteTally) service.VoteTally {
	tally.ResolveCells = 0
	tally.ReopenCells = 0
	tally.ReporterResolved = false
	tally.ReporterReopened = false
	return tally
}

// relBypasses is the test's own copy of the critical bypass predicate, kept
// here so the property test does not lean on the code under test.
func relBypasses(meta service.ReportMeta) bool {
	return meta.Severity == service.SeverityCritical || meta.Category == service.CategoryRescueNeeded
}

// TestClassifyOutcome is the classifier table: one prose subtest per row.
func TestClassifyOutcome(t *testing.T) {
	low := service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood}
	critical := service.ReportMeta{Severity: service.SeverityCritical, Category: service.CategoryFlood}

	cases := []struct {
		name      string
		meta      service.ReportMeta
		tally     service.VoteTally
		expiresAt time.Time
		want      service.Outcome
	}{
		{"a low report with two confirm places is corroborated", low, service.VoteTally{ConfirmCells: 2}, relNow, service.OutcomeCorroborated},
		{"a low report with three confirm places and one dispute place is corroborated", low, service.VoteTally{ConfirmCells: 3, DisputeCells: 1}, relNow, service.OutcomeCorroborated},
		{"a tie of two confirm places against two dispute places does not hide, so it is corroborated", low, service.VoteTally{ConfirmCells: 2, DisputeCells: 2}, relNow, service.OutcomeCorroborated},
		{"one confirm place against two dispute places is contradicted", low, service.VoteTally{ConfirmCells: 1, DisputeCells: 2}, relNow, service.OutcomeContradicted},
		{"a low report that expired with no votes is unconfirmed", low, service.VoteTally{}, relNow, service.OutcomeUnconfirmed},
		{"a medium road_blocked report with one confirm place and one dispute place is unconfirmed", service.ReportMeta{Severity: service.SeverityMedium, Category: service.CategoryRoadBlocked}, service.VoteTally{ConfirmCells: 1, DisputeCells: 1}, relNow, service.OutcomeUnconfirmed},
		{"a critical report that expired with no votes is neutral", critical, service.VoteTally{}, relNow, service.OutcomeNeutral},
		{"a low rescue_needed report with one confirm place is neutral", service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryRescueNeeded}, service.VoteTally{ConfirmCells: 1}, relNow, service.OutcomeNeutral},
		{"a critical report with two confirm places is still corroborated", critical, service.VoteTally{ConfirmCells: 2}, relNow, service.OutcomeCorroborated},
		{"a medium rescue_needed report with one confirm place and three dispute places is contradicted", service.ReportMeta{Severity: service.SeverityMedium, Category: service.CategoryRescueNeeded}, service.VoteTally{ConfirmCells: 1, DisputeCells: 3}, relNow, service.OutcomeContradicted},
		{"a report still in flight by one nanosecond is neutral", low, service.VoteTally{ConfirmCells: 2}, relNow.Add(time.Nanosecond), service.OutcomeNeutral},
		{"a disputed report still in flight by an hour is neutral", low, service.VoteTally{ConfirmCells: 1, DisputeCells: 2}, relNow.Add(time.Hour), service.OutcomeNeutral},
		{"a report expiring exactly now is judged", low, service.VoteTally{ConfirmCells: 2}, relNow, service.OutcomeCorroborated},
		{"a self resolve with no content votes is unconfirmed", low, service.VoteTally{ReporterResolved: true}, relNow, service.OutcomeUnconfirmed},
		{"a self resolve cannot erase two dispute places", low, service.VoteTally{DisputeCells: 2, ReporterResolved: true}, relNow, service.OutcomeContradicted},
		{"a reporter reopen with two confirm places is corroborated", low, service.VoteTally{ConfirmCells: 2, ReporterReopened: true}, relNow, service.OutcomeCorroborated},
		{"a report retracted by two resolve places is judged by its confirm places", low, service.VoteTally{ConfirmCells: 2, ResolveCells: 2}, relNow, service.OutcomeCorroborated},
		{"a critical report retracted by two resolve places with no content votes is neutral", critical, service.VoteTally{ResolveCells: 2}, relNow, service.OutcomeNeutral},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := service.ClassifyOutcome(tc.meta, tc.tally, tc.expiresAt, relNow)
			if got != tc.want {
				t.Fatalf("ClassifyOutcome = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("an out disputed critical report is contradicted for the rating while Resolve keeps it live", func(t *testing.T) {
		tally := service.VoteTally{DisputeCells: 2}
		if got := service.ClassifyOutcome(critical, tally, relNow, relNow); got != service.OutcomeContradicted {
			t.Fatalf("ClassifyOutcome = %q, want %q", got, service.OutcomeContradicted)
		}
		vis, _ := service.Resolve(critical, tally, relNow)
		if vis != service.VisibilityLive {
			t.Fatalf("Resolve = %q, want %q (the classifier must never change visibility)", vis, service.VisibilityLive)
		}
	})
}

// TestClassifyOutcomeMatchesResolve walks every severity, category and tally
// combination and holds the classifier to the visibility ladder.
func TestClassifyOutcomeMatchesResolve(t *testing.T) {
	n := service.IndependentAgreementThreshold
	bools := []bool{false, true}
	combos := 0
	for _, sev := range service.Severities {
		for _, cat := range service.Categories {
			meta := service.ReportMeta{Severity: sev, Category: cat}
			for confirm := 0; confirm <= n+2; confirm++ {
				for dispute := 0; dispute <= n+2; dispute++ {
					for resolve := 0; resolve <= n+1; resolve++ {
						for reopen := 0; reopen <= n+1; reopen++ {
							for _, rr := range bools {
								for _, ro := range bools {
									combos++
									tally := service.VoteTally{
										ConfirmCells: confirm, DisputeCells: dispute,
										ResolveCells: resolve, ReopenCells: reopen,
										ReporterResolved: rr, ReporterReopened: ro,
									}
									zeroed := relZeroResolution(tally)

									vis, _ := service.Resolve(service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryOther}, zeroed, relNow)
									var want service.Outcome
									switch vis {
									case service.VisibilityLive:
										want = service.OutcomeCorroborated
									case service.VisibilityHidden:
										want = service.OutcomeContradicted
									case service.VisibilityProvisional:
										if relBypasses(meta) {
											want = service.OutcomeNeutral
										} else {
											want = service.OutcomeUnconfirmed
										}
									default:
										t.Fatalf("Resolve on a zeroed tally returned %q for %+v", vis, tally)
									}

									got := service.ClassifyOutcome(meta, tally, relNow, relNow)
									if got != want {
										t.Fatalf("meta %+v tally %+v: ClassifyOutcome = %q, want %q", meta, tally, got, want)
									}
									if again := service.ClassifyOutcome(meta, tally, relNow, relNow); again != got {
										t.Fatalf("meta %+v tally %+v: not deterministic, %q then %q", meta, tally, got, again)
									}
									if !got.Valid() {
										t.Fatalf("meta %+v tally %+v: %q is not a valid outcome", meta, tally, got)
									}
									if withZeroed := service.ClassifyOutcome(meta, zeroed, relNow, relNow); withZeroed != got {
										t.Fatalf("meta %+v tally %+v: resolution votes changed the outcome, %q versus %q", meta, tally, got, withZeroed)
									}
									if inFlight := service.ClassifyOutcome(meta, tally, relNow.Add(time.Nanosecond), relNow); inFlight != service.OutcomeNeutral {
										t.Fatalf("meta %+v tally %+v: in flight report was %q, want neutral", meta, tally, inFlight)
									}
									if old := service.ClassifyOutcome(meta, tally, relNow.Add(-400*24*time.Hour), relNow); old != got {
										t.Fatalf("meta %+v tally %+v: a 400 day old report was %q, want %q (no window in the classifier)", meta, tally, old, got)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	if combos == 0 {
		t.Fatal("the grid was empty")
	}
}

const (
	relC = service.OutcomeCorroborated
	relX = service.OutcomeContradicted
	relU = service.OutcomeUnconfirmed
	relN = service.OutcomeNeutral

	relDay = 24 * time.Hour
)

// relOutcomes builds count entries of outcome o that all expired age before
// relNow.
func relOutcomes(o service.Outcome, count int, age time.Duration) []service.JudgedOutcome {
	out := make([]service.JudgedOutcome, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, service.JudgedOutcome{Outcome: o, ExpiresAt: relNow.Add(-age)})
	}
	return out
}

// relHourly builds entries of outcome o that expired i hours before relNow
// for every i from first to last inclusive, so every ExpiresAt is distinct.
func relHourly(o service.Outcome, first, last int) []service.JudgedOutcome {
	var out []service.JudgedOutcome
	for i := first; i <= last; i++ {
		out = append(out, service.JudgedOutcome{Outcome: o, ExpiresAt: relNow.Add(-time.Duration(i) * time.Hour)})
	}
	return out
}

// relJoin concatenates histories into a fresh slice.
func relJoin(parts ...[]service.JudgedOutcome) []service.JudgedOutcome {
	var out []service.JudgedOutcome
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// relRoundHalfUp is the integer oracle for round half up of num/den.
func relRoundHalfUp(num, den int) int {
	return (2*num + den) / (2 * den)
}

// relTagRank orders tags from worst to best for the monotonic invariant.
func relTagRank(tag service.ReporterTag) int {
	switch tag {
	case service.ReporterUnreliable:
		return 0
	case service.ReporterMixed:
		return 1
	case service.ReporterReliable:
		return 2
	}
	return -1
}

// relScore returns the score and fails the test when it is nil, so a missing
// score is one assertion failure and never a nil dereference.
func relScore(t *testing.T, r service.Reliability) int {
	t.Helper()
	if r.Score == nil {
		t.Fatalf("Score is nil, tag %q judged %d", r.Tag, r.Judged)
	}
	return *r.Score
}

// relSame reports whether two ratings are equal, comparing score values and
// not pointers.
func relSame(a, b service.Reliability) bool {
	if a.Tag != b.Tag || a.Judged != b.Judged || a.Corroborated != b.Corroborated {
		return false
	}
	if (a.Score == nil) != (b.Score == nil) {
		return false
	}
	return a.Score == nil || *a.Score == *b.Score
}

// relShift moves every ExpiresAt by d.
func relShift(in []service.JudgedOutcome, d time.Duration) []service.JudgedOutcome {
	out := make([]service.JudgedOutcome, len(in))
	for i, e := range in {
		out[i] = service.JudgedOutcome{Outcome: e.Outcome, ExpiresAt: e.ExpiresAt.Add(d)}
	}
	return out
}

// TestReliabilityTable pins the 19 anchor rows of the outline plus boundary
// rows. Every expected score is a literal integer, never recomputed in
// floating point. A wantScore of -1 means a nil score.
func TestReliabilityTable(t *testing.T) {
	cases := []struct {
		name       string
		history    []service.JudgedOutcome
		wantTag    service.ReporterTag
		wantScore  int
		wantJudged int
		wantCorr   int
	}{
		{"5 corroborated fresh", relOutcomes(relC, 5, 0), service.ReporterReliable, 86, 5, 5},
		{"5 corroborated all at 45 days", relOutcomes(relC, 5, 45*relDay), service.ReporterReliable, 86, 5, 5},
		{"5 corroborated all at 89 days", relOutcomes(relC, 5, 89*relDay), service.ReporterReliable, 86, 5, 5},
		{"30 corroborated", relOutcomes(relC, 30, 0), service.ReporterReliable, 97, 30, 30},
		{"4 corroborated and 1 unconfirmed sit exactly on 75", relJoin(relOutcomes(relC, 4, 0), relOutcomes(relU, 1, 0)), service.ReporterReliable, 75, 5, 4},
		{"4 corroborated and 1 contradicted", relJoin(relOutcomes(relC, 4, 0), relOutcomes(relX, 1, 0)), service.ReporterMixed, 71, 5, 4},
		{"3 corroborated and 2 unconfirmed", relJoin(relOutcomes(relC, 3, 0), relOutcomes(relU, 2, 0)), service.ReporterMixed, 64, 5, 3},
		{"3 corroborated and 2 contradicted", relJoin(relOutcomes(relC, 3, 0), relOutcomes(relX, 2, 0)), service.ReporterMixed, 57, 5, 3},
		{"5 corroborated and 5 unconfirmed", relJoin(relOutcomes(relC, 5, 0), relOutcomes(relU, 5, 0)), service.ReporterMixed, 60, 10, 5},
		{"2 corroborated and 3 contradicted", relJoin(relOutcomes(relC, 2, 0), relOutcomes(relX, 3, 0)), service.ReporterMixed, 43, 5, 2},
		{"5 unconfirmed", relOutcomes(relU, 5, 0), service.ReporterUnreliable, 32, 5, 0},
		{"30 unconfirmed", relOutcomes(relU, 30, 0), service.ReporterUnreliable, 27, 30, 0},
		{"1 contradicted and 4 unconfirmed", relJoin(relOutcomes(relX, 1, 0), relOutcomes(relU, 4, 0)), service.ReporterUnreliable, 29, 5, 0},
		{"2 contradicted and 3 unconfirmed sit exactly on 25", relJoin(relOutcomes(relX, 2, 0), relOutcomes(relU, 3, 0)), service.ReporterUnreliable, 25, 5, 0},
		{"1 corroborated and 4 contradicted", relJoin(relOutcomes(relC, 1, 0), relOutcomes(relX, 4, 0)), service.ReporterUnreliable, 29, 5, 1},
		{"5 contradicted", relOutcomes(relX, 5, 0), service.ReporterUnreliable, 14, 5, 0},
		{"8 corroborated and 2 contradicted sit exactly on 75", relJoin(relOutcomes(relC, 8, 0), relOutcomes(relX, 2, 0)), service.ReporterReliable, 75, 10, 8},
		{"6 corroborated and 12 contradicted sit exactly on 35", relJoin(relOutcomes(relC, 6, 0), relOutcomes(relX, 12, 0)), service.ReporterMixed, 35, 18, 6},
		{"mixed ages across the half life", relJoin(
			relOutcomes(relC, 1, 5*relDay), relOutcomes(relC, 1, 20*relDay), relOutcomes(relC, 1, 50*relDay),
			relOutcomes(relX, 1, 10*relDay), relOutcomes(relU, 1, 40*relDay)), service.ReporterMixed, 60, 5, 3},
		{"5 corroborated at 80 days and 2 contradicted yesterday", relJoin(relOutcomes(relC, 5, 80*relDay), relOutcomes(relX, 2, 1*relDay)), service.ReporterUnreliable, 33, 7, 5},
		{"5 contradicted at 80 days and 2 corroborated yesterday", relJoin(relOutcomes(relX, 5, 80*relDay), relOutcomes(relC, 2, 1*relDay)), service.ReporterMixed, 67, 7, 2},

		{"an empty history is a new reporter", nil, service.ReporterNew, -1, 0, 0},
		{"4 corroborated is below the minimum of 5", relOutcomes(relC, 4, 0), service.ReporterNew, -1, 4, 4},
		{"neutral outcomes are not judged", relJoin(relOutcomes(relC, 4, 0), relOutcomes(relN, 3, 0)), service.ReporterNew, -1, 4, 4},
		{"an outcome outside the closed set is ignored like neutral", relJoin(relOutcomes(relC, 5, 0), relOutcomes("", 1, 0)), service.ReporterReliable, 86, 5, 5},
		{"6 corroborated, 1 unconfirmed, 1 contradicted rounds 72.5 up to 73", relJoin(relOutcomes(relC, 6, 0), relOutcomes(relU, 1, 0), relOutcomes(relX, 1, 0)), service.ReporterMixed, 73, 8, 6},
		{"2 corroborated, 1 unconfirmed, 5 contradicted rounds 32.5 up to 33", relJoin(relOutcomes(relC, 2, 0), relOutcomes(relU, 1, 0), relOutcomes(relX, 5, 0)), service.ReporterUnreliable, 33, 8, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := service.ScoreReliability(tc.history, relNow)
			if got.Tag != tc.wantTag {
				t.Fatalf("Tag = %q, want %q", got.Tag, tc.wantTag)
			}
			if tc.wantScore < 0 {
				if got.Score != nil {
					t.Fatalf("Score = %d, want nil", *got.Score)
				}
			} else if s := relScore(t, got); s != tc.wantScore {
				t.Fatalf("Score = %d, want %d", s, tc.wantScore)
			}
			if got.Judged != tc.wantJudged {
				t.Fatalf("Judged = %d, want %d", got.Judged, tc.wantJudged)
			}
			if got.Corroborated != tc.wantCorr {
				t.Fatalf("Corroborated = %d, want %d", got.Corroborated, tc.wantCorr)
			}
		})
	}
}

// TestReliabilityDecayAndWindow pins the half life, the window edges, the
// row cap and its tie order, and time shift equality.
func TestReliabilityDecayAndWindow(t *testing.T) {
	check := func(t *testing.T, h []service.JudgedOutcome, wantTag service.ReporterTag, wantScore, wantJudged int) service.Reliability {
		t.Helper()
		got := service.ScoreReliability(h, relNow)
		if got.Tag != wantTag {
			t.Fatalf("Tag = %q, want %q", got.Tag, wantTag)
		}
		if wantScore < 0 {
			if got.Score != nil {
				t.Fatalf("Score = %d, want nil", *got.Score)
			}
		} else if s := relScore(t, got); s != wantScore {
			t.Fatalf("Score = %d, want %d", s, wantScore)
		}
		if got.Judged != wantJudged {
			t.Fatalf("Judged = %d, want %d", got.Judged, wantJudged)
		}
		return got
	}

	t.Run("a contradicted run at 30 days weighs half a fresh corroborated report", func(t *testing.T) {
		check(t, relJoin(relOutcomes(relC, 1, 0), relOutcomes(relX, 4, 30*relDay)), service.ReporterMixed, 38, 5)
	})
	t.Run("a contradicted run at 60 days weighs a quarter", func(t *testing.T) {
		check(t, relJoin(relOutcomes(relC, 1, 0), relOutcomes(relX, 4, 60*relDay)), service.ReporterMixed, 50, 5)
	})
	t.Run("with every report fresh the same history is unreliable", func(t *testing.T) {
		check(t, relJoin(relOutcomes(relC, 1, 0), relOutcomes(relX, 4, 0)), service.ReporterUnreliable, 29, 5)
	})

	t.Run("uniform ageing keeps the exact boundaries", func(t *testing.T) {
		check(t, relJoin(relOutcomes(relC, 4, 45*relDay), relOutcomes(relU, 1, 45*relDay)), service.ReporterReliable, 75, 5)
		check(t, relJoin(relOutcomes(relC, 6, 45*relDay), relOutcomes(relU, 1, 45*relDay), relOutcomes(relX, 1, 45*relDay)), service.ReporterMixed, 73, 8)
		check(t, relJoin(relOutcomes(relC, 8, 70*relDay), relOutcomes(relX, 2, 70*relDay)), service.ReporterReliable, 75, 10)
		check(t, relJoin(relOutcomes(relC, 6, 70*relDay), relOutcomes(relX, 12, 70*relDay)), service.ReporterMixed, 35, 18)
		check(t, relJoin(relOutcomes(relX, 2, 45*relDay), relOutcomes(relU, 3, 45*relDay)), service.ReporterUnreliable, 25, 5)
	})

	t.Run("a report exactly 90 days old is outside the window", func(t *testing.T) {
		check(t, relJoin(relOutcomes(relC, 4, 0), relOutcomes(relC, 1, 90*relDay)), service.ReporterNew, -1, 4)
	})
	t.Run("a report one minute inside the window counts", func(t *testing.T) {
		check(t, relJoin(relOutcomes(relC, 4, 0), relOutcomes(relC, 1, 90*relDay-time.Minute)), service.ReporterReliable, 86, 5)
	})
	t.Run("a report expiring after now is excluded", func(t *testing.T) {
		future := []service.JudgedOutcome{{Outcome: relC, ExpiresAt: relNow.Add(time.Nanosecond)}}
		check(t, relJoin(relOutcomes(relC, 4, 0), future), service.ReporterNew, -1, 4)
	})

	t.Run("the cap drops the 31st oldest report", func(t *testing.T) {
		h := relJoin(relHourly(relC, 0, 29), relOutcomes(relX, 1, 40*relDay))
		got := check(t, h, service.ReporterReliable, 97, 30)
		if got.Corroborated != 30 {
			t.Fatalf("Corroborated = %d, want 30", got.Corroborated)
		}
	})
	t.Run("a newer outcome pushes out the oldest", func(t *testing.T) {
		h := relJoin(relOutcomes(relX, 1, 0), relHourly(relC, 1, 30))
		got := check(t, h, service.ReporterReliable, relScore(t, service.ScoreReliability(relJoin(relOutcomes(relX, 1, 0), relHourly(relC, 1, 29)), relNow)), 30)
		if got.Corroborated != 29 {
			t.Fatalf("Corroborated = %d, want 29", got.Corroborated)
		}
		explicit := service.ScoreReliability(relJoin(relOutcomes(relX, 1, 0), relHourly(relC, 1, 29)), relNow)
		if !relSame(got, explicit) {
			t.Fatalf("capped result differs from the explicit kept 30")
		}
	})
	t.Run("the cap counts neutral rows like the SQL cap does", func(t *testing.T) {
		check(t, relJoin(relHourly(relN, 0, 29), relOutcomes(relC, 5, 10*relDay)), service.ReporterNew, -1, 0)
	})
	t.Run("a tie at the cap boundary keeps the contradicted row, for either input order", func(t *testing.T) {
		base := relHourly(relC, 0, 28)
		tied := []service.JudgedOutcome{
			{Outcome: relC, ExpiresAt: relNow.Add(-40 * relDay)},
			{Outcome: relX, ExpiresAt: relNow.Add(-40 * relDay)},
		}
		reversed := []service.JudgedOutcome{tied[1], tied[0]}
		a := service.ScoreReliability(relJoin(base, tied), relNow)
		b := service.ScoreReliability(relJoin(base, reversed), relNow)
		if a.Judged != 30 || a.Corroborated != 29 {
			t.Fatalf("Judged %d Corroborated %d, want 30 and 29", a.Judged, a.Corroborated)
		}
		if !relSame(a, b) {
			t.Fatalf("input order changed the result")
		}
	})

	t.Run("shifting now and every expiry together gives an equal result", func(t *testing.T) {
		histories := [][]service.JudgedOutcome{
			relJoin(relOutcomes(relC, 1, 5*relDay), relOutcomes(relC, 1, 20*relDay), relOutcomes(relC, 1, 50*relDay), relOutcomes(relX, 1, 10*relDay), relOutcomes(relU, 1, 40*relDay)),
			relJoin(relOutcomes(relC, 5, 80*relDay), relOutcomes(relX, 2, 1*relDay)),
			relJoin(relOutcomes(relX, 5, 80*relDay), relOutcomes(relC, 2, 1*relDay)),
		}
		shift := 17*relDay + 3*time.Hour
		for i, h := range histories {
			before := service.ScoreReliability(h, relNow)
			after := service.ScoreReliability(relShift(h, shift), relNow.Add(shift))
			if !relSame(before, after) {
				t.Fatalf("history %d changed under a time shift", i)
			}
		}
	})

	t.Run("the caller's slice is neither reordered nor edited", func(t *testing.T) {
		h := relJoin(relOutcomes(relC, 3, 40*relDay), relHourly(relX, 0, 4), relOutcomes(relU, 2, 85*relDay), relOutcomes(relN, 1, 95*relDay))
		snapshot := make([]service.JudgedOutcome, len(h))
		copy(snapshot, h)
		service.ScoreReliability(h, relNow)
		if !reflect.DeepEqual(h, snapshot) {
			t.Fatalf("ScoreReliability modified its input slice")
		}
	})
}

// TestReliabilityInvariants checks I1 to I4 over seeded random histories for
// every n from 5 to 30, and the minimum for n below 5.
func TestReliabilityInvariants(t *testing.T) {
	rng := rand.New(rand.NewSource(20261004))
	classes := []service.Outcome{relX, relU, relC}
	randAge := func() time.Duration {
		return time.Duration(rng.Int63n(int64(service.ReliabilityWindow)))
	}
	build := func(n int, pick func() service.Outcome) []service.JudgedOutcome {
		h := make([]service.JudgedOutcome, n)
		for i := range h {
			h[i] = service.JudgedOutcome{Outcome: pick(), ExpiresAt: relNow.Add(-randAge())}
		}
		return h
	}
	expectedTag := func(score int) service.ReporterTag {
		switch {
		case score >= 75:
			return service.ReporterReliable
		case score >= 35:
			return service.ReporterMixed
		}
		return service.ReporterUnreliable
	}

	for n := 5; n <= 30; n++ {
		for iter := 0; iter < 40; iter++ {
			// I1: all corroborated.
			r := service.ScoreReliability(build(n, func() service.Outcome { return relC }), relNow)
			if r.Tag != service.ReporterReliable || relScore(t, r) != relRoundHalfUp(100*(n+1), n+2) {
				t.Fatalf("I1 n=%d: tag %q score %d, want reliable %d", n, r.Tag, relScore(t, r), relRoundHalfUp(100*(n+1), n+2))
			}

			// I2: all unconfirmed always reads unreliable.
			r = service.ScoreReliability(build(n, func() service.Outcome { return relU }), relNow)
			if r.Tag != service.ReporterUnreliable || relScore(t, r) != relRoundHalfUp(25*(n+4), n+2) {
				t.Fatalf("I2 n=%d: tag %q score %d, want unreliable %d", n, r.Tag, relScore(t, r), relRoundHalfUp(25*(n+4), n+2))
			}

			// I3 and I4 on random class mixes.
			h := build(n, func() service.Outcome { return classes[rng.Intn(3)] })
			base := service.ScoreReliability(h, relNow)
			score := relScore(t, base)
			if base.Judged != n {
				t.Fatalf("n=%d: Judged = %d", n, base.Judged)
			}
			if lo, hi := relRoundHalfUp(100, n+2), relRoundHalfUp(100*(n+1), n+2); score < lo || score > hi {
				t.Fatalf("I4 n=%d: score %d outside [%d, %d]", n, score, lo, hi)
			}
			if base.Tag != expectedTag(score) {
				t.Fatalf("I4 n=%d: tag %q does not match score %d", n, base.Tag, score)
			}
			if base.Tag == service.ReporterNone || !base.Tag.Valid() {
				t.Fatalf("n=%d: invalid tag %q", n, base.Tag)
			}

			var upgradable []int
			for i, e := range h {
				if e.Outcome != relC {
					upgradable = append(upgradable, i)
				}
			}
			if len(upgradable) > 0 {
				up := make([]service.JudgedOutcome, n)
				copy(up, h)
				i := upgradable[rng.Intn(len(upgradable))]
				if up[i].Outcome == relX {
					up[i].Outcome = relU
				} else {
					up[i].Outcome = relC
				}
				better := service.ScoreReliability(up, relNow)
				if relScore(t, better) < score || relTagRank(better.Tag) < relTagRank(base.Tag) {
					t.Fatalf("I3 n=%d: upgrading entry %d lowered the rating from %d (%s) to %d (%s)", n, i, score, base.Tag, relScore(t, better), better.Tag)
				}
			}
			shuffled := make([]service.JudgedOutcome, n)
			copy(shuffled, h)
			rng.Shuffle(n, func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
			if !relSame(base, service.ScoreReliability(shuffled, relNow)) {
				t.Fatalf("I3 n=%d: order changed the result", n)
			}
		}
	}

	for n := 0; n <= 4; n++ {
		for iter := 0; iter < 40; iter++ {
			r := service.ScoreReliability(build(n, func() service.Outcome { return classes[rng.Intn(3)] }), relNow)
			if r.Tag != service.ReporterNew || r.Score != nil || r.Judged != n {
				t.Fatalf("n=%d: tag %q judged %d, want new with a nil score", n, r.Tag, r.Judged)
			}
		}
	}
}

// TestReporterTagSetIsClosed pins the five reporter tag slugs and their order.
func TestReporterTagSetIsClosed(t *testing.T) {
	want := []service.ReporterTag{"reliable", "mixed", "unreliable", "new", "none"}
	if len(service.ReporterTags) != len(want) {
		t.Fatalf("ReporterTags has %d entries, want %d", len(service.ReporterTags), len(want))
	}
	for i, tag := range want {
		if service.ReporterTags[i] != tag {
			t.Fatalf("ReporterTags[%d] = %q, want %q", i, service.ReporterTags[i], tag)
		}
		if !service.ReporterTags[i].Valid() {
			t.Fatalf("%q should be valid", tag)
		}
	}
	for _, bad := range []service.ReporterTag{"", "Reliable"} {
		if bad.Valid() {
			t.Fatalf("%q should not be valid", bad)
		}
	}
}

// TestOutcomeSetIsClosed pins the four outcome slugs and their order.
func TestOutcomeSetIsClosed(t *testing.T) {
	want := []service.Outcome{"corroborated", "contradicted", "unconfirmed", "neutral"}
	if len(service.Outcomes) != len(want) {
		t.Fatalf("Outcomes has %d entries, want %d", len(service.Outcomes), len(want))
	}
	for i, o := range want {
		if service.Outcomes[i] != o {
			t.Fatalf("Outcomes[%d] = %q, want %q", i, service.Outcomes[i], o)
		}
		if !service.Outcomes[i].Valid() {
			t.Fatalf("%q should be valid", o)
		}
	}
	for _, bad := range []service.Outcome{"", "Corroborated"} {
		if bad.Valid() {
			t.Fatalf("%q should not be valid", bad)
		}
	}
}
