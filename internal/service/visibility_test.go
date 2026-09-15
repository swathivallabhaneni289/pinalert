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
