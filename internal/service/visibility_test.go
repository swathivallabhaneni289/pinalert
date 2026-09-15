package service_test

import (
	"testing"
	"time"

	"pinalert/internal/service"
)

// TestResolve is the master decision-ladder table: one t.Run prose subtest
// per row, asserting both return values of Resolve — the reason slug is a
// serialized contract, not debug output.
func TestResolve(t *testing.T) {
	cases := []struct {
		name       string
		meta       service.ReportMeta
		tally      service.VoteTally
		wantVis    service.Visibility
		wantReason service.ResolveReason
	}{
		{
			name:       "a non-critical report with no confirms and no disputes is provisional, awaiting a second independent confirmation",
			meta:       service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:      service.VoteTally{},
			wantVis:    service.VisibilityProvisional,
			wantReason: service.ReasonAwaitingConfirmation,
		},
		{
			name:       "a non-critical report with two independent confirm cells is live and confirmed",
			meta:       service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:      service.VoteTally{ConfirmCells: 2},
			wantVis:    service.VisibilityLive,
			wantReason: service.ReasonConfirmed,
		},
		{
			name:       "one confirm cell against two dispute cells hides the report as disputed",
			meta:       service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:      service.VoteTally{ConfirmCells: 1, DisputeCells: 2},
			wantVis:    service.VisibilityHidden,
			wantReason: service.ReasonDisputed,
		},
		{
			name:       "a single dispute cell alone is below the hidden floor and stays provisional (D-05)",
			meta:       service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:      service.VoteTally{ConfirmCells: 0, DisputeCells: 1},
			wantVis:    service.VisibilityProvisional,
			wantReason: service.ReasonAwaitingConfirmation,
		},
		{
			name:       "a tie between confirms and disputes does not hide the report (D-05 requires a strict majority)",
			meta:       service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:      service.VoteTally{ConfirmCells: 2, DisputeCells: 2},
			wantVis:    service.VisibilityLive,
			wantReason: service.ReasonConfirmed,
		},
		{
			name:       "critical severity bypasses the hidden trigger even with five disputes and zero confirms (D-06)",
			meta:       service.ReportMeta{Severity: service.SeverityCritical, Category: service.CategoryFlood},
			tally:      service.VoteTally{ConfirmCells: 0, DisputeCells: 5},
			wantVis:    service.VisibilityLive,
			wantReason: service.ReasonCriticalBypass,
		},
		{
			name:       "rescue_needed category bypasses the hidden trigger even with five disputes and zero confirms (D-06)",
			meta:       service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryRescueNeeded},
			tally:      service.VoteTally{ConfirmCells: 0, DisputeCells: 5},
			wantVis:    service.VisibilityLive,
			wantReason: service.ReasonCriticalBypass,
		},
		{
			name:       "critical severity publishes at full visibility immediately with an all-zero tally (TRUST-04)",
			meta:       service.ReportMeta{Severity: service.SeverityCritical, Category: service.CategoryFlood},
			tally:      service.VoteTally{},
			wantVis:    service.VisibilityLive,
			wantReason: service.ReasonCriticalBypass,
		},
		{
			name:       "the reporter's own resolve vote retracts the report instantly (D-13)",
			meta:       service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:      service.VoteTally{ReporterResolved: true},
			wantVis:    service.VisibilityRetracted,
			wantReason: service.ReasonResolved,
		},
		{
			name:       "retracted outranks the critical bypass regardless of severity (D-07)",
			meta:       service.ReportMeta{Severity: service.SeverityCritical, Category: service.CategoryRescueNeeded},
			tally:      service.VoteTally{ReporterResolved: true, DisputeCells: 0, ConfirmCells: 9},
			wantVis:    service.VisibilityRetracted,
			wantReason: service.ReasonResolved,
		},
		{
			name:       "a single independent resolve cell is below the threshold and stays provisional",
			meta:       service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:      service.VoteTally{ResolveCells: 1},
			wantVis:    service.VisibilityProvisional,
			wantReason: service.ReasonAwaitingConfirmation,
		},
		{
			name:       "two independent resolve cells reach the threshold and retract the report (D-14)",
			meta:       service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:      service.VoteTally{ResolveCells: 2},
			wantVis:    service.VisibilityRetracted,
			wantReason: service.ReasonResolved,
		},
		{
			name:       "the reporter's instant reopen lifts a retraction caused by independent confirmers, falling through to the ordinary ladder at zero confirm cells (D-16 amended)",
			meta:       service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:      service.VoteTally{ResolveCells: 2, ReporterReopened: true, ReopenCells: 0},
			wantVis:    service.VisibilityProvisional,
			wantReason: service.ReasonAwaitingConfirmation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotVis, gotReason := service.Resolve(tc.meta, tc.tally, time.Now())
			if gotVis != tc.wantVis {
				t.Errorf("visibility = %q, want %q", gotVis, tc.wantVis)
			}
			if gotReason != tc.wantReason {
				t.Errorf("reason = %q, want %q", gotReason, tc.wantReason)
			}
		})
	}
}

// TestResolve_ProvisionalGate proves the Provisional-to-Live boundary sits
// exactly at IndependentAgreementThreshold and that the critical/
// rescue_needed bypass clears it entirely (TRUST-04).
func TestResolve_ProvisionalGate(t *testing.T) {
	cases := []struct {
		name    string
		meta    service.ReportMeta
		tally   service.VoteTally
		wantVis service.Visibility
	}{
		{
			name:    "non-critical with zero confirm cells is provisional",
			meta:    service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:   service.VoteTally{ConfirmCells: 0},
			wantVis: service.VisibilityProvisional,
		},
		{
			name:    "non-critical with one confirm cell, one below the threshold, is still provisional",
			meta:    service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:   service.VoteTally{ConfirmCells: service.IndependentAgreementThreshold - 1},
			wantVis: service.VisibilityProvisional,
		},
		{
			name:    "non-critical reaching IndependentAgreementThreshold confirm cells is live",
			meta:    service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood},
			tally:   service.VoteTally{ConfirmCells: service.IndependentAgreementThreshold},
			wantVis: service.VisibilityLive,
		},
		{
			name:    "critical severity with zero confirm cells bypasses the gate and is live",
			meta:    service.ReportMeta{Severity: service.SeverityCritical, Category: service.CategoryFlood},
			tally:   service.VoteTally{ConfirmCells: 0},
			wantVis: service.VisibilityLive,
		},
		{
			name:    "rescue_needed category with zero confirm cells bypasses the gate and is live",
			meta:    service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryRescueNeeded},
			tally:   service.VoteTally{ConfirmCells: 0},
			wantVis: service.VisibilityLive,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotVis, _ := service.Resolve(tc.meta, tc.tally, time.Now())
			if gotVis != tc.wantVis {
				t.Errorf("visibility = %q, want %q", gotVis, tc.wantVis)
			}
		})
	}
}

// TestResolve_SeverityNeverBypassesGateAlone proves self-declared severity
// moves triage sort order only and can never by itself unlock full
// visibility (TRUST-06).
func TestResolve_SeverityNeverBypassesGateAlone(t *testing.T) {
	for _, sev := range service.Severities {
		if sev == service.SeverityCritical {
			continue
		}
		t.Run("severity "+string(sev)+" with one confirm cell alone cannot unlock full visibility (TRUST-06)", func(t *testing.T) {
			meta := service.ReportMeta{Severity: sev, Category: service.CategoryFlood}
			tally := service.VoteTally{ConfirmCells: 1}
			gotVis, _ := service.Resolve(meta, tally, time.Now())
			if gotVis != service.VisibilityProvisional {
				t.Errorf("visibility = %q, want %q", gotVis, service.VisibilityProvisional)
			}
		})
	}
}

// TestResolve_HiddenIsReversible proves Hidden is fully reversible by
// recomputation with a later tally — there is no unhide branch anywhere in
// the file (D-08).
func TestResolve_HiddenIsReversible(t *testing.T) {
	meta := service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood}

	tally1 := service.VoteTally{ConfirmCells: 1, DisputeCells: 2}
	gotVis1, _ := service.Resolve(meta, tally1, time.Now())
	if gotVis1 != service.VisibilityHidden {
		t.Fatalf("first call: visibility = %q, want %q", gotVis1, service.VisibilityHidden)
	}

	tally2 := service.VoteTally{ConfirmCells: 3, DisputeCells: 2}
	gotVis2, _ := service.Resolve(meta, tally2, time.Now())
	if gotVis2 != service.VisibilityLive {
		t.Fatalf("second call: visibility = %q, want %q", gotVis2, service.VisibilityLive)
	}
}

// TestResolve_RetractedAndReopen proves D-07 (Retracted outranks everything
// below it), D-13 (the reporter's own resolve vote retracts instantly), and
// D-16 amended (the reporter's own reopen vote lifts a retraction
// instantly and symmetrically, regardless of how the report became
// Retracted; a non-reporter reopen is still threshold-gated; neither side
// latches).
func TestResolve_RetractedAndReopen(t *testing.T) {
	meta := service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood}

	cases := []struct {
		name          string
		tally         service.VoteTally
		wantRetracted bool
	}{
		// Non-reporter reopen — still threshold-gated (D-14).
		{
			name:          "two independent resolve cells and zero reopen cells stay retracted",
			tally:         service.VoteTally{ResolveCells: 2, ReopenCells: 0},
			wantRetracted: true,
		},
		{
			name:          "one independent reopen cell is below the threshold and the report stays retracted",
			tally:         service.VoteTally{ResolveCells: 2, ReopenCells: 1},
			wantRetracted: true,
		},
		{
			name:          "two independent reopen cells reach the threshold and the report is no longer retracted",
			tally:         service.VoteTally{ResolveCells: 2, ReopenCells: 2},
			wantRetracted: false,
		},
		{
			name:          "one non-reporter reopen cell cannot lift a reporter-instant retraction",
			tally:         service.VoteTally{ReporterResolved: true, ReporterReopened: false, ReopenCells: 1},
			wantRetracted: true,
		},

		// Reporter-instant reopen — threshold-free, symmetric with D-13 (D-16 amended).
		{
			name:          "the reporter's own reopen overrides their own instant resolve at zero independent cells",
			tally:         service.VoteTally{ReporterResolved: true, ReporterReopened: true, ReopenCells: 0},
			wantRetracted: false,
		},
		{
			name:          "the reporter's own reopen instantly lifts a retraction caused by independent confirmers (D-16 amended)",
			tally:         service.VoteTally{ReporterResolved: false, ResolveCells: 2, ReporterReopened: true, ReopenCells: 0},
			wantRetracted: false,
		},
		{
			name:          "no volume of independent resolve cells outranks the reporter's own standing reopen",
			tally:         service.VoteTally{ResolveCells: 9, ReporterResolved: true, ReporterReopened: true, ReopenCells: 0},
			wantRetracted: false,
		},
		{
			name:          "a standing reopen vote on a report that was never retracted is a no-op, not an error state",
			tally:         service.VoteTally{ReporterReopened: true, ResolveCells: 0, ReporterResolved: false},
			wantRetracted: false,
		},

		// Neither side latches (D-08).
		{
			name:          "non-reporter reopen outranks resolve at equal standing and above",
			tally:         service.VoteTally{ResolveCells: 3, ReopenCells: 2},
			wantRetracted: false,
		},
		{
			name:          "reopen cells falling back below the threshold re-retracts the report",
			tally:         service.VoteTally{ResolveCells: 3, ReopenCells: 1},
			wantRetracted: true,
		},
		{
			name:          "the reporter's standing reopen vote is not a one-way unlock",
			tally:         service.VoteTally{ResolveCells: 2, ReporterReopened: true},
			wantRetracted: false,
		},
		{
			name:          "when the reporter's resolution vote changes away from reopen, surviving independent resolve agreement reasserts itself",
			tally:         service.VoteTally{ResolveCells: 2, ReporterReopened: false},
			wantRetracted: true,
		},

		// Withdrawal is not the same mechanism as reopen (D-02 permits changing a vote).
		{
			name:          "the reporter withdrawing their own resolve vote removes the sole cause of retraction",
			tally:         service.VoteTally{ReporterResolved: false, ReporterReopened: false, ResolveCells: 0, ReopenCells: 0},
			wantRetracted: false,
		},
		{
			name:          "withdrawing only the reporter's own resolve vote does not override independent agreement",
			tally:         service.VoteTally{ReporterResolved: false, ReporterReopened: false, ResolveCells: 2},
			wantRetracted: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotVis, gotReason := service.Resolve(meta, tc.tally, time.Now())
			gotRetracted := gotVis == service.VisibilityRetracted
			if gotRetracted != tc.wantRetracted {
				t.Errorf("retracted = %v, want %v (visibility %q, reason %q)", gotRetracted, tc.wantRetracted, gotVis, gotReason)
			}
			if tc.wantRetracted && gotReason != service.ReasonResolved {
				t.Errorf("reason = %q, want %q for a retracted report", gotReason, service.ReasonResolved)
			}
		})
	}
}

// TestResolve_IsTotalAndDeterministic is a single exhaustive matrix over
// every severity x category x {ConfirmCells,DisputeCells,ResolveCells,
// ReopenCells} in 0..IndependentAgreementThreshold+1 x ReporterResolved x
// ReporterReopened (~27.6k combinations of a pure function — milliseconds).
// For every combination it proves Resolve is total (T-02-04), returns only
// slugs from the closed reason set, is deterministic across repeat calls,
// and is time-invariant for Phase 2. It additionally pins three structural
// properties: the retraction predicate is exactly the symmetric expression
// (D-13, D-14, D-16 amended), Retracted outranks the critical bypass, and
// the critical_bypasses_gates reason is returned if and only if the two
// documented triggers (D-06) hold.
//
// The matrix necessarily generates ReporterResolved && ReporterReopened
// together, a combination production cannot produce (one account has one
// current resolution vote, so 02-03a's BuildVoteTally sets at most one of
// the two). That combination is deliberately exercised here rather than
// skipped: the defined behaviour is that reopen wins, matching the
// non-reporter half where reopen outranks resolve at equal standing — no
// defensive invariant check should ever be added downstream for it.
func TestResolve_IsTotalAndDeterministic(t *testing.T) {
	// This time-invariance assertion holds for Phase 2 only, because expiry
	// is a separate read-time predicate (D-07) and no decay signal exists
	// yet. Phase 3 (TRUST-07) introduces time-decaying confidence/
	// reliability scoring, at which point now becomes load-bearing and this
	// specific assertion is expected to be revised — its later removal is a
	// planned Phase 3 change, not a regression.
	now1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now2 := now1.AddDate(5, 0, 0)

	for _, sev := range service.Severities {
		for _, cat := range service.Categories {
			sev, cat := sev, cat
			t.Run(string(sev)+"/"+string(cat), func(t *testing.T) {
				meta := service.ReportMeta{Severity: sev, Category: cat}

				for confirmCells := 0; confirmCells <= service.IndependentAgreementThreshold+1; confirmCells++ {
					for disputeCells := 0; disputeCells <= service.IndependentAgreementThreshold+1; disputeCells++ {
						for resolveCells := 0; resolveCells <= service.IndependentAgreementThreshold+1; resolveCells++ {
							for reopenCells := 0; reopenCells <= service.IndependentAgreementThreshold+1; reopenCells++ {
								for _, reporterResolved := range []bool{false, true} {
									for _, reporterReopened := range []bool{false, true} {
										tally := service.VoteTally{
											ConfirmCells:     confirmCells,
											DisputeCells:     disputeCells,
											ResolveCells:     resolveCells,
											ReopenCells:      reopenCells,
											ReporterResolved: reporterResolved,
											ReporterReopened: reporterReopened,
										}

										vis1, reason1 := service.Resolve(meta, tally, now1)

										// Total: no fall-through returning the zero value.
										if !vis1.Valid() {
											t.Errorf("meta=%+v tally=%+v: visibility %q is not a valid Visibility", meta, tally, vis1)
										}

										// Closed reason set: always one of the five known slugs.
										validReason := false
										for _, r := range service.ResolveReasons {
											if r == reason1 {
												validReason = true
												break
											}
										}
										if !validReason {
											t.Errorf("meta=%+v tally=%+v: reason %q is not in the closed ResolveReasons set", meta, tally, reason1)
										}

										// Deterministic / caller-independent: identical meta+tally,
										// identical pair, on a second call.
										vis1b, reason1b := service.Resolve(meta, tally, now1)
										if vis1b != vis1 || reason1b != reason1 {
											t.Errorf("meta=%+v tally=%+v: Resolve is not deterministic: first (%q,%q), second (%q,%q)", meta, tally, vis1, reason1, vis1b, reason1b)
										}

										// Time-invariant for Phase 2 (see comment above).
										vis2, reason2 := service.Resolve(meta, tally, now2)
										if vis2 != vis1 || reason2 != reason1 {
											t.Errorf("meta=%+v tally=%+v: Resolve is not time-invariant: now1 (%q,%q), now2 (%q,%q)", meta, tally, vis1, reason1, vis2, reason2)
										}

										// The retraction predicate is exactly the symmetric expression
										// (D-13, D-14, D-16 amended): dropping the ReporterReopened
										// term, adding a threshold to the reporter's instant path, or
										// making reopen lose to resolve at equal standing each break
										// this single property.
										wantRetracted := (tally.ReporterResolved || tally.ResolveCells >= service.IndependentAgreementThreshold) &&
											!(tally.ReporterReopened || tally.ReopenCells >= service.IndependentAgreementThreshold)
										gotRetracted := vis1 == service.VisibilityRetracted
										if gotRetracted != wantRetracted {
											t.Errorf("meta=%+v tally=%+v: retracted = %v, want %v (visibility %q)", meta, tally, gotRetracted, wantRetracted, vis1)
										}

										// Retracted outranks the critical bypass: a tally with
										// ReporterResolved true, ReporterReopened false and no
										// standing reopen cells always yields retracted, never live —
										// the reporter's own standing reopen vote lifts a retraction
										// instantly at any independent cell count (D-16 amended), but
										// absent that vote nothing (not even a critical/rescue_needed
										// report) outranks an active resolve.
										if reporterResolved && !reporterReopened && reopenCells == 0 {
											if vis1 != service.VisibilityRetracted {
												t.Errorf("meta=%+v tally=%+v: expected retracted to outrank the critical bypass, got %q", meta, tally, vis1)
											}
										}

										// The bypass is exactly the two documented triggers (D-06):
										// pinned in both directions so neither adding a third trigger
										// nor removing one of the two existing triggers can pass
										// silently.
										wantBypass := (sev == service.SeverityCritical || cat == service.CategoryRescueNeeded) && !gotRetracted
										gotBypass := reason1 == service.ReasonCriticalBypass
										if gotBypass != wantBypass {
											t.Errorf("meta=%+v tally=%+v: critical_bypasses_gates reason = %v, want %v (visibility %q, reason %q)", meta, tally, gotBypass, wantBypass, vis1, reason1)
										}
									}
								}
							}
						}
					}
				}
			})
		}
	}
}
