package service_test

import (
	"context"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// TestUndoReadOnlyWhenNeeded pins the conditional undo count read (D-22): the
// read runs only for reports where the viewer has a resolution row or is the
// poster, only for unexpired reports, and costs nothing on an ordinary page.
func TestUndoReadOnlyWhenNeeded(t *testing.T) {
	page := func() *fakeFeedQuerier {
		return &fakeFeedQuerier{
			nearbyRows: []sqlcgen.NearbyReportsRow{
				p12Row(1, service.SeverityLow, service.CategoryFlood, 1.0),
				p12Row(2, service.SeverityLow, service.CategoryFlood, 2.0),
				p12Row(3, service.SeverityLow, service.CategoryFlood, 3.0),
			},
			reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(1, nil), reporterRow(2, nil), reporterRow(3, nil)},
		}
	}
	query := service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, ViewerAccountID: 7}

	t.Run("a viewer with no resolution row who posted nothing makes no undo call", func(t *testing.T) {
		fake := page()
		views, err := service.NewReportService(fake, service.WithReportClock(p12Clock)).Nearby(context.Background(), query)
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if got := countCalls(fake.callLog, "ResolutionUndoCounts"); got != 0 {
			t.Fatalf("ResolutionUndoCounts called %d times, want 0", got)
		}
		for _, v := range views {
			if v.UndosLeft != service.UndoLimit {
				t.Errorf("report %d: UndosLeft = %d, want %d", v.ID, v.UndosLeft, service.UndoLimit)
			}
		}
	})

	t.Run("one resolved report makes exactly one call for that one id", func(t *testing.T) {
		fake := page()
		fake.voteRows = append(fake.voteRows, p12Vote(2, 7, service.VoteKindResolution, service.VoteResolve, "cellV"))
		fake.undoRows = []sqlcgen.ResolutionUndoCountsRow{{ReportID: 2, Unresolves: 1}}
		views, err := service.NewReportService(fake, service.WithReportClock(p12Clock)).Nearby(context.Background(), query)
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if got := countCalls(fake.callLog, "ResolutionUndoCounts"); got != 1 {
			t.Fatalf("ResolutionUndoCounts called %d times, want 1", got)
		}
		if len(fake.undoArgs) != 1 || len(fake.undoArgs[0].ReportIds) != 1 || fake.undoArgs[0].ReportIds[0] != 2 || fake.undoArgs[0].AccountID != 7 {
			t.Fatalf("undo args = %+v, want one call for report 2 and account 7", fake.undoArgs)
		}
		want := map[int64]int{1: service.UndoLimit, 2: service.UndoLimit - 1, 3: service.UndoLimit}
		for _, v := range views {
			if v.UndosLeft != want[v.ID] {
				t.Errorf("report %d: UndosLeft = %d, want %d", v.ID, v.UndosLeft, want[v.ID])
			}
		}
	})

	t.Run("a report the viewer posted is a candidate and its reopens spend the budget", func(t *testing.T) {
		fake := page()
		fake.reporterRows[0] = reporterRow(1, ptrID(7))
		fake.undoRows = []sqlcgen.ResolutionUndoCountsRow{{ReportID: 1, Unresolves: 1, Reopens: 1}}
		views, err := service.NewReportService(fake, service.WithReportClock(p12Clock)).Nearby(context.Background(), query)
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if got := countCalls(fake.callLog, "ResolutionUndoCounts"); got != 1 {
			t.Fatalf("ResolutionUndoCounts called %d times, want 1", got)
		}
		v1, _ := findView(views, 1)
		if v1.UndosLeft != 0 {
			t.Errorf("report 1: UndosLeft = %d, want 0", v1.UndosLeft)
		}
	})

	t.Run("an expired report is never a candidate", func(t *testing.T) {
		fake := page()
		fake.nearbyRows = fake.nearbyRows[:1]
		fake.nearbyRows[0].ExpiresAt = p12Now.Add(-time.Minute)
		fake.voteRows = append(fake.voteRows, p12Vote(1, 7, service.VoteKindResolution, service.VoteResolve, "cellV"))
		views, err := service.NewReportService(fake, service.WithReportClock(p12Clock)).Nearby(context.Background(), query)
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if got := countCalls(fake.callLog, "ResolutionUndoCounts"); got != 0 {
			t.Fatalf("ResolutionUndoCounts called %d times, want 0", got)
		}
		for _, v := range views {
			if v.UndosLeft != service.UndoLimit {
				t.Errorf("report %d: UndosLeft = %d, want %d", v.ID, v.UndosLeft, service.UndoLimit)
			}
		}
	})
}
