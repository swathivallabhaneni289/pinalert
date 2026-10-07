package service_test

import (
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
