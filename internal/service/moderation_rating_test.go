// This file proves Phase 4 R-02 on the pure classifier and on its composition
// with history rows: a report that moderation withholds is neutral for the
// reporter rating whatever its votes. It declares no fake (the pure functions
// need none), and every helper is prefixed p429 so it cannot collide with the
// helpers of the other test files in this package.
package service_test

import (
	"fmt"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

var p429Now = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

// p429Score prints an optional score for a failure message.
func p429Score(s *int) string {
	if s == nil {
		return "nil"
	}
	return fmt.Sprint(*s)
}

func p429Meta(sev service.Severity, cat service.Category, state service.ModerationState) service.ReportMeta {
	return service.ReportMeta{Severity: sev, Category: cat, Moderation: state}
}

func p429Row(account, report int64, sev service.Severity, cat service.Category, state service.ModerationState, expiresAt time.Time) sqlcgen.ReporterHistoryReportsRow {
	return sqlcgen.ReporterHistoryReportsRow{
		AccountID:       account,
		ReportID:        report,
		Severity:        string(sev),
		Category:        string(cat),
		ExpiresAt:       expiresAt,
		ModerationState: string(state),
	}
}

// p429Confirms gives two confirm votes from two accounts in two distinct cells.
func p429Confirms(report int64) []sqlcgen.CurrentVotesForReportsRow {
	return []sqlcgen.CurrentVotesForReportsRow{
		{
			ReportID:    report,
			AccountID:   9001,
			Kind:        string(service.VoteKindContent),
			Value:       string(service.VoteConfirm),
			GeohashCell: fmt.Sprintf("p429c%da", report),
			CreatedAt:   p429Now.Add(-3 * time.Hour),
		},
		{
			ReportID:    report,
			AccountID:   9002,
			Kind:        string(service.VoteKindContent),
			Value:       string(service.VoteConfirm),
			GeohashCell: fmt.Sprintf("p429c%db", report),
			CreatedAt:   p429Now.Add(-3 * time.Hour),
		},
	}
}

func TestModeratedReportsAreNeutralForRating(t *testing.T) {
	now := p429Now
	classify := func(meta service.ReportMeta, confirm, dispute int) service.Outcome {
		return service.ClassifyOutcome(meta, service.VoteTally{ConfirmCells: confirm, DisputeCells: dispute}, now, now)
	}
	want := func(t *testing.T, name string, got, wantOutcome service.Outcome) {
		t.Helper()
		if got != wantOutcome {
			t.Errorf("%s: outcome = %q, want %q (R-02)", name, got, wantOutcome)
		}
	}

	t.Run("an expired held low flood report with no votes is neutral, not unconfirmed", func(t *testing.T) {
		want(t, "pending", classify(p429Meta(service.SeverityLow, service.CategoryFlood, service.ModerationPending), 0, 0), service.OutcomeNeutral)
		want(t, "clear control", classify(p429Meta(service.SeverityLow, service.CategoryFlood, service.ModerationClear), 0, 0), service.OutcomeUnconfirmed)
	})

	t.Run("a removed low flood report confirmed by two places is neutral, not corroborated", func(t *testing.T) {
		want(t, "removed", classify(p429Meta(service.SeverityLow, service.CategoryFlood, service.ModerationRemoved), 2, 0), service.OutcomeNeutral)
		want(t, "clear control", classify(p429Meta(service.SeverityLow, service.CategoryFlood, service.ModerationClear), 2, 0), service.OutcomeCorroborated)
	})

	t.Run("a removed or held low flood report out disputed is neutral, not contradicted", func(t *testing.T) {
		want(t, "removed", classify(p429Meta(service.SeverityLow, service.CategoryFlood, service.ModerationRemoved), 1, 2), service.OutcomeNeutral)
		want(t, "pending", classify(p429Meta(service.SeverityLow, service.CategoryFlood, service.ModerationPending), 1, 2), service.OutcomeNeutral)
		want(t, "clear control", classify(p429Meta(service.SeverityLow, service.CategoryFlood, service.ModerationClear), 1, 2), service.OutcomeContradicted)
	})

	t.Run("removed outranks the critical rule", func(t *testing.T) {
		want(t, "removed critical flood disputed", classify(p429Meta(service.SeverityCritical, service.CategoryFlood, service.ModerationRemoved), 0, 2), service.OutcomeNeutral)
		want(t, "clear critical flood disputed", classify(p429Meta(service.SeverityCritical, service.CategoryFlood, service.ModerationClear), 0, 2), service.OutcomeContradicted)
		want(t, "removed low rescue needed confirmed", classify(p429Meta(service.SeverityLow, service.CategoryRescueNeeded, service.ModerationRemoved), 2, 0), service.OutcomeNeutral)
		want(t, "clear low rescue needed confirmed", classify(p429Meta(service.SeverityLow, service.CategoryRescueNeeded, service.ModerationClear), 2, 0), service.OutcomeCorroborated)
	})

	t.Run("a held serious report is judged exactly as a clear one", func(t *testing.T) {
		serious := []service.ReportMeta{
			{Severity: service.SeverityCritical, Category: service.CategoryFlood},
			{Severity: service.SeverityLow, Category: service.CategoryRescueNeeded},
		}
		for _, base := range serious {
			for _, tally := range [][2]int{{2, 0}, {0, 2}, {0, 0}} {
				held := base
				held.Moderation = service.ModerationPending
				clear := base
				clear.Moderation = service.ModerationClear
				name := fmt.Sprintf("%s %s confirm %d dispute %d", base.Severity, base.Category, tally[0], tally[1])
				want(t, name, classify(held, tally[0], tally[1]), classify(clear, tally[0], tally[1]))
			}
		}
		pendingCritical := p429Meta(service.SeverityCritical, service.CategoryFlood, service.ModerationPending)
		want(t, "pending critical flood, 2 confirm places", classify(pendingCritical, 2, 0), service.OutcomeCorroborated)
		want(t, "pending critical flood, 2 dispute places", classify(pendingCritical, 0, 2), service.OutcomeContradicted)
	})

	t.Run("the rule is exactly Withheld over the whole grid", func(t *testing.T) {
		states := append([]service.ModerationState{""}, service.ModerationStates...)
		top := service.IndependentAgreementThreshold + 1
		for _, sev := range service.Severities {
			for _, cat := range service.Categories {
				for _, state := range states {
					for confirm := 0; confirm <= top; confirm++ {
						for dispute := 0; dispute <= top; dispute++ {
							for _, expires := range []time.Time{now, now.Add(time.Nanosecond)} {
								meta := p429Meta(sev, cat, state)
								tally := service.VoteTally{ConfirmCells: confirm, DisputeCells: dispute}
								got := service.ClassifyOutcome(meta, tally, expires, now)
								name := fmt.Sprintf("%s %s state %q confirm %d dispute %d expires %v", sev, cat, state, confirm, dispute, expires.Sub(now))
								if meta.Withheld() {
									want(t, name+" (withheld)", got, service.OutcomeNeutral)
									continue
								}
								clearMeta := p429Meta(sev, cat, service.ModerationClear)
								want(t, name+" (not withheld)", got, service.ClassifyOutcome(clearMeta, tally, expires, now))
							}
						}
					}
				}
			}
		}
	})

	t.Run("five expired held reports leave a New reporter", func(t *testing.T) {
		const account = 501
		build := func(state service.ModerationState) service.ReporterSignal {
			var rows []sqlcgen.ReporterHistoryReportsRow
			var votes []sqlcgen.CurrentVotesForReportsRow
			for i := int64(0); i < 5; i++ {
				id := 5101 + i
				rows = append(rows, p429Row(account, id, service.SeverityLow, service.CategoryFlood, state, now.Add(-2*time.Hour)))
				votes = append(votes, p429Confirms(id)...)
			}
			return service.ReporterSignals([]int64{account}, rows, votes, now)[account]
		}
		for _, state := range []service.ModerationState{service.ModerationPending, service.ModerationRemoved} {
			got := build(state)
			if got.Tag != service.ReporterNew || got.Score != nil || got.JudgedReports != 0 || got.CorroboratedReports != 0 {
				t.Errorf("%s: tag %q score nil %v judged %d corroborated %d, want New with no score and zero counts (R-02)", state, got.Tag, got.Score == nil, got.JudgedReports, got.CorroboratedReports)
			}
		}
		clear := build(service.ModerationClear)
		if clear.JudgedReports != 5 || clear.CorroboratedReports != 5 || clear.Tag == service.ReporterNew {
			t.Errorf("clear control: tag %q judged %d corroborated %d, want 5 judged, 5 corroborated and not New (R-02)", clear.Tag, clear.JudgedReports, clear.CorroboratedReports)
		}
	})

	heldRun := func(account, firstID int64, state service.ModerationState) []sqlcgen.ReporterHistoryReportsRow {
		var rows []sqlcgen.ReporterHistoryReportsRow
		for i := 0; i < service.ReliabilityRowCap; i++ {
			expires := now.Add(-48 * time.Hour).Add(time.Duration(i) * time.Hour)
			rows = append(rows, p429Row(account, firstID+int64(i), service.SeverityLow, service.CategoryFlood, state, expires))
		}
		return rows
	}

	t.Run("a run of held reports pushes no judged record out of the cap", func(t *testing.T) {
		const account = 502
		var older []sqlcgen.ReporterHistoryReportsRow
		var votes []sqlcgen.CurrentVotesForReportsRow
		for i := int64(0); i < 5; i++ {
			id := 5201 + i
			older = append(older, p429Row(account, id, service.SeverityLow, service.CategoryFlood, service.ModerationClear, now.Add(-72*time.Hour)))
			votes = append(votes, p429Confirms(id)...)
		}
		held := append(append([]sqlcgen.ReporterHistoryReportsRow(nil), older...), heldRun(account, 5300, service.ModerationPending)...)
		got := service.ReporterSignals([]int64{account}, held, votes, now)[account]
		if got.Tag != service.ReporterReliable || got.Score == nil || *got.Score != 86 || got.JudgedReports != 5 || got.CorroboratedReports != 5 {
			t.Errorf("held run: tag %q score %s judged %d corroborated %d, want Reliable, 86, 5, 5 (R-02, WR-05)", got.Tag, p429Score(got.Score), got.JudgedReports, got.CorroboratedReports)
		}
		clearRun := append(append([]sqlcgen.ReporterHistoryReportsRow(nil), older...), heldRun(account, 5300, service.ModerationClear)...)
		control := service.ReporterSignals([]int64{account}, clearRun, votes, now)[account]
		if control.JudgedReports != service.ReliabilityRowCap || control.CorroboratedReports != 0 {
			t.Errorf("clear control: judged %d corroborated %d, want %d judged and 0 corroborated (R-02)", control.JudgedReports, control.CorroboratedReports, service.ReliabilityRowCap)
		}
	})

	t.Run("a run of held reports cannot wipe a poor record", func(t *testing.T) {
		const account = 504
		var poor []sqlcgen.ReporterHistoryReportsRow
		for i := int64(0); i < 6; i++ {
			poor = append(poor, p429Row(account, 5401+i, service.SeverityLow, service.CategoryFlood, service.ModerationClear, now.Add(-72*time.Hour)))
		}
		alone := service.ReporterSignals([]int64{account}, poor, nil, now)[account]
		if alone.Tag != service.ReporterUnreliable {
			t.Fatalf("setup: six unconfirmed reports read %q, want unreliable", alone.Tag)
		}
		held := append(append([]sqlcgen.ReporterHistoryReportsRow(nil), poor...), heldRun(account, 5500, service.ModerationPending)...)
		got := service.ReporterSignals([]int64{account}, held, nil, now)[account]
		sameScore := (got.Score == nil) == (alone.Score == nil) && (got.Score == nil || *got.Score == *alone.Score)
		if got.Tag != alone.Tag || !sameScore || got.JudgedReports != 6 || got.CorroboratedReports != 0 {
			t.Errorf("held run: tag %q score %s judged %d corroborated %d, want the six reports' own tag %q, score %s, 6, 0 (R-02)", got.Tag, p429Score(got.Score), got.JudgedReports, got.CorroboratedReports, alone.Tag, p429Score(alone.Score))
		}
	})

	t.Run("a held serious report in the history is judged as before", func(t *testing.T) {
		const account = 503
		rows := []sqlcgen.ReporterHistoryReportsRow{
			p429Row(account, 5301, service.SeverityCritical, service.CategoryFlood, service.ModerationPending, now.Add(-2*time.Hour)),
		}
		got := service.ReporterSignals([]int64{account}, rows, p429Confirms(5301), now)[account]
		if got.JudgedReports != 1 || got.CorroboratedReports != 1 {
			t.Errorf("judged %d corroborated %d, want 1 and 1: a pending serious report was public and is judged as before (R-02)", got.JudgedReports, got.CorroboratedReports)
		}
	})
}
