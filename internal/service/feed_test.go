package service_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// fakeFeedQuerier is a programmable service.Querier requiring no real
// Postgres. It records every call it receives (callLog) and the id slice
// handed to each batched method, which is what makes
// TestNearbyBatchesVoteReadsOnce mechanical rather than inferred.
type fakeFeedQuerier struct {
	nearbyRows   []sqlcgen.NearbyReportsRow
	nearbyErr    error
	voteRows     []sqlcgen.CurrentVotesForReportsRow
	voteErr      error
	reporterRows []sqlcgen.ReporterAccountsForReportsRow
	reporterErr  error
	historyRows  []sqlcgen.ReporterHistoryReportsRow
	historyErr   error

	// The plan 03.1-06 reads. focusRow nil means the database has no such
	// report, so ReportForFocus answers pgx.ErrNoRows.
	expiredRows []sqlcgen.ExpiredReportsInWindowRow
	expiredErr  error
	openRows    []sqlcgen.OpenReportsInAreaRow
	openErr     error
	focusRow    *sqlcgen.ReportForFocusRow
	focusErr    error
	undoRows    []sqlcgen.ResolutionUndoCountsRow
	undoErr     error

	lastVoteIDs     []int64
	lastReporterIDs []int64
	historyArgs     []sqlcgen.ReporterHistoryReportsParams
	expiredArgs     []sqlcgen.ExpiredReportsInWindowParams
	openArgs        []sqlcgen.OpenReportsInAreaParams
	focusArgs       []sqlcgen.ReportForFocusParams
	undoArgs        []sqlcgen.ResolutionUndoCountsParams
	callLog         []string

	// extrasRows/extrasErr/extrasArgs drive ReportExtrasForReports (plan
	// 04-07). Unconfigured (nil extrasRows) the fake answers a clear row per
	// requested id, so every existing test keeps its result.
	extrasRows []sqlcgen.ReportExtrasForReportsRow
	extrasErr  error
	extrasArgs []sqlcgen.ReportExtrasForReportsParams
}

// ReportExtrasForReports records the call. Configured, it returns only the
// rows of the requested ids, like the query's own filter (a non nil empty
// slice therefore returns none); unconfigured it returns one clear row per
// requested id in request order.
func (f *fakeFeedQuerier) ReportExtrasForReports(ctx context.Context, arg sqlcgen.ReportExtrasForReportsParams) ([]sqlcgen.ReportExtrasForReportsRow, error) {
	f.callLog = append(f.callLog, "ReportExtrasForReports")
	f.extrasArgs = append(f.extrasArgs, arg)
	if f.extrasErr != nil {
		return nil, f.extrasErr
	}
	var out []sqlcgen.ReportExtrasForReportsRow
	if f.extrasRows == nil {
		for _, id := range arg.ReportIds {
			out = append(out, sqlcgen.ReportExtrasForReportsRow{ReportID: id, ModerationState: string(service.ModerationClear)})
		}
		return out, nil
	}
	for _, row := range f.extrasRows {
		if slices.Contains(arg.ReportIds, row.ReportID) {
			out = append(out, row)
		}
	}
	return out, nil
}

// ExpiredReportsInWindow, OpenReportsInArea, ReportForFocus and
// ResolutionUndoCounts are the plan 03.1-06 reads. Unconfigured they return an
// empty result and no error; ReportForFocus returns pgx.ErrNoRows, which is
// the database's own answer for an unknown id.
func (f *fakeFeedQuerier) ExpiredReportsInWindow(ctx context.Context, arg sqlcgen.ExpiredReportsInWindowParams) ([]sqlcgen.ExpiredReportsInWindowRow, error) {
	f.callLog = append(f.callLog, "ExpiredReportsInWindow")
	f.expiredArgs = append(f.expiredArgs, arg)
	return f.expiredRows, f.expiredErr
}

func (f *fakeFeedQuerier) OpenReportsInArea(ctx context.Context, arg sqlcgen.OpenReportsInAreaParams) ([]sqlcgen.OpenReportsInAreaRow, error) {
	f.callLog = append(f.callLog, "OpenReportsInArea")
	f.openArgs = append(f.openArgs, arg)
	return f.openRows, f.openErr
}

func (f *fakeFeedQuerier) ReportForFocus(ctx context.Context, arg sqlcgen.ReportForFocusParams) (sqlcgen.ReportForFocusRow, error) {
	f.callLog = append(f.callLog, "ReportForFocus")
	f.focusArgs = append(f.focusArgs, arg)
	if f.focusErr != nil {
		return sqlcgen.ReportForFocusRow{}, f.focusErr
	}
	if f.focusRow == nil || f.focusRow.ID != arg.ReportID {
		return sqlcgen.ReportForFocusRow{}, pgx.ErrNoRows
	}
	return *f.focusRow, nil
}

// ResolutionUndoCounts records the call and returns only the rows of undoRows
// whose report is in the requested list, which is the query's own id filter.
func (f *fakeFeedQuerier) ResolutionUndoCounts(ctx context.Context, arg sqlcgen.ResolutionUndoCountsParams) ([]sqlcgen.ResolutionUndoCountsRow, error) {
	f.callLog = append(f.callLog, "ResolutionUndoCounts")
	f.undoArgs = append(f.undoArgs, arg)
	if f.undoErr != nil {
		return nil, f.undoErr
	}
	var out []sqlcgen.ResolutionUndoCountsRow
	for _, row := range f.undoRows {
		if slices.Contains(arg.ReportIds, row.ReportID) {
			out = append(out, row)
		}
	}
	return out, nil
}

// InsertReport exists only to satisfy service.Querier; no feed test drives it.
func (f *fakeFeedQuerier) InsertReport(ctx context.Context, arg sqlcgen.InsertReportParams) (sqlcgen.InsertReportRow, error) {
	return sqlcgen.InsertReportRow{}, nil
}

func (f *fakeFeedQuerier) NearbyReports(ctx context.Context, arg sqlcgen.NearbyReportsParams) ([]sqlcgen.NearbyReportsRow, error) {
	f.callLog = append(f.callLog, "NearbyReports")
	return f.nearbyRows, f.nearbyErr
}

func (f *fakeFeedQuerier) CurrentVotesForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	f.callLog = append(f.callLog, "CurrentVotesForReports")
	f.lastVoteIDs = reportIds
	return f.voteRows, f.voteErr
}

func (f *fakeFeedQuerier) ReporterAccountsForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.ReporterAccountsForReportsRow, error) {
	f.callLog = append(f.callLog, "ReporterAccountsForReports")
	f.lastReporterIDs = reportIds
	return f.reporterRows, f.reporterErr
}

// ReporterHistoryReports records the call and returns only the rows of
// historyRows whose account is in the requested list, which is the query's own
// account filter. Unconfigured it returns an empty result and no error.
func (f *fakeFeedQuerier) ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error) {
	f.callLog = append(f.callLog, "ReporterHistoryReports")
	f.historyArgs = append(f.historyArgs, arg)
	if f.historyErr != nil {
		return nil, f.historyErr
	}
	var out []sqlcgen.ReporterHistoryReportsRow
	for _, row := range f.historyRows {
		if slices.Contains(arg.AccountIds, row.AccountID) {
			out = append(out, row)
		}
	}
	return out, nil
}

// nearbyRow builds a sqlcgen.NearbyReportsRow from the fields each test
// table varies; every other field is a realistic constant.
func nearbyRow(id int64, severity service.Severity, category service.Category, distanceKm float64) sqlcgen.NearbyReportsRow {
	now := time.Now().UTC()
	return sqlcgen.NearbyReportsRow{
		ID:          id,
		Category:    string(category),
		Severity:    string(severity),
		Description: "a fake nearby report for feed_test.go",
		Latitude:    12.9716,
		Longitude:   77.5946,
		Geohash:     "tdr1qgzp",
		CreatedAt:   now,
		ExpiresAt:   now.Add(time.Hour),
		DistanceKm:  distanceKm,
	}
}

// voteRow builds a sqlcgen.CurrentVotesForReportsRow from the fields each
// test table varies.
func voteRow(reportID, accountID int64, kind service.VoteKind, value service.VoteValue, cell string) sqlcgen.CurrentVotesForReportsRow {
	return sqlcgen.CurrentVotesForReportsRow{
		ReportID:    reportID,
		AccountID:   accountID,
		Kind:        string(kind),
		Value:       string(value),
		GeohashCell: cell,
		CreatedAt:   time.Now().UTC(),
	}
}

// reporterRow builds a sqlcgen.ReporterAccountsForReportsRow. Pass nil for
// reporterAccountID to model a pre-Phase-1.1 or seeded report with no
// account-bound session.
func reporterRow(reportID int64, reporterAccountID *int64) sqlcgen.ReporterAccountsForReportsRow {
	return sqlcgen.ReporterAccountsForReportsRow{
		ReportID:          reportID,
		ReporterAccountID: reporterAccountID,
	}
}

func ptrID(id int64) *int64 { return &id }

// voteCellNames supplies distinct geohash cell names for the tests that
// need to construct exactly N independent confirm/dispute/resolve/reopen
// cells for a single report.
var voteCellNames = []string{"cellA", "cellB", "cellC", "cellD", "cellE", "cellF"}

// distinctVoteRows returns n votes for reportID, each from a distinct
// account in a distinct geohash cell, so the resulting independent cell
// count is exactly n — used so cases that depend on
// service.IndependentAgreementThreshold track the constant rather than a
// bare literal.
func distinctVoteRows(reportID int64, kind service.VoteKind, value service.VoteValue, n int, startAccountID int64) []sqlcgen.CurrentVotesForReportsRow {
	rows := make([]sqlcgen.CurrentVotesForReportsRow, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, voteRow(reportID, startAccountID+int64(i), kind, value, voteCellNames[i]))
	}
	return rows
}

func containsReportID(views []service.ReportView, id int64) bool {
	_, ok := findView(views, id)
	return ok
}

func findView(views []service.ReportView, id int64) (service.ReportView, bool) {
	for _, v := range views {
		if v.ID == id {
			return v, true
		}
	}
	return service.ReportView{}, false
}

func viewIDs(views []service.ReportView) []int64 {
	out := make([]int64, 0, len(views))
	for _, v := range views {
		out = append(out, v.ID)
	}
	return out
}

func equalIDs(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func assertAscendingDistance(t *testing.T, views []service.ReportView) {
	t.Helper()
	for i := 1; i < len(views); i++ {
		prev := views[i-1].DistanceKm
		curr := views[i].DistanceKm
		if prev == nil || curr == nil {
			t.Fatalf("expected DistanceKm to be populated on every nearby result")
		}
		if *prev > *curr {
			t.Fatalf("ordering is not ascending by distance: %v then %v", *prev, *curr)
		}
	}
}

func countCalls(log []string, name string) int {
	n := 0
	for _, c := range log {
		if c == name {
			n++
		}
	}
	return n
}

func TestListableInFeed(t *testing.T) {
	cases := []struct {
		name            string
		visibility      service.Visibility
		includeDisputed bool
		want            bool
	}{
		{"Live is listable when disputed reports are excluded", service.VisibilityLive, false, true},
		{"Live is excluded when disputed reports are included (the partition, not a union)", service.VisibilityLive, true, false},
		{"Provisional is listable when disputed reports are excluded", service.VisibilityProvisional, false, true},
		{"Provisional is excluded when disputed reports are included (the partition, not a union)", service.VisibilityProvisional, true, false},
		{"Hidden is not listable when disputed reports are excluded (D-10)", service.VisibilityHidden, false, false},
		{"Hidden is listable when disputed reports are included (D-10)", service.VisibilityHidden, true, true},
		{"Retracted is not listable when disputed reports are excluded (D-12)", service.VisibilityRetracted, false, false},
		{"Retracted is not listable when disputed reports are included either (D-12)", service.VisibilityRetracted, true, false},
		{"an unrecognised visibility fails closed when disputed reports are excluded", service.Visibility("sideways"), false, false},
		{"an unrecognised visibility fails closed when disputed reports are included", service.Visibility("sideways"), true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.visibility.ListableInFeed(tc.includeDisputed)
			if got != tc.want {
				t.Errorf("Visibility(%q).ListableInFeed(%v) = %v, want %v", tc.visibility, tc.includeDisputed, got, tc.want)
			}
		})
	}
}

func TestViewerContentVote(t *testing.T) {
	t.Run("no rows yields an empty vote value", func(t *testing.T) {
		got := service.ViewerContentVote(nil, 1, 9)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("a content confirm row from the viewer's own account returns confirm", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 9, service.VoteKindContent, service.VoteConfirm, "cellA")}
		got := service.ViewerContentVote(rows, 1, 9)
		if got != service.VoteConfirm {
			t.Errorf("got %q, want %q", got, service.VoteConfirm)
		}
	})

	t.Run("the same row queried for a different viewer returns empty", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 9, service.VoteKindContent, service.VoteConfirm, "cellA")}
		got := service.ViewerContentVote(rows, 1, 7)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("a content dispute row from the viewer's own account returns dispute", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 9, service.VoteKindContent, service.VoteDispute, "cellA")}
		got := service.ViewerContentVote(rows, 1, 9)
		if got != service.VoteDispute {
			t.Errorf("got %q, want %q", got, service.VoteDispute)
		}
	})

	t.Run("a resolution vote never surfaces as the viewer's content vote (D-02)", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 9, service.VoteKindResolution, service.VoteResolve, "cellA")}
		got := service.ViewerContentVote(rows, 1, 9)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("rows for a different report id are ignored even when the account matches", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(2, 9, service.VoteKindContent, service.VoteConfirm, "cellA")}
		got := service.ViewerContentVote(rows, 1, 9)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("viewer account id 0 matches nothing, including a row whose account id is 0", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 0, service.VoteKindContent, service.VoteConfirm, "cellA")}
		got := service.ViewerContentVote(rows, 1, 0)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestNearbyExcludesRetractedFromBothViews(t *testing.T) {
	reporterID := int64(100)

	build := func(severity service.Severity, category service.Category) *fakeFeedQuerier {
		return &fakeFeedQuerier{
			nearbyRows: []sqlcgen.NearbyReportsRow{
				nearbyRow(1, severity, category, 1.0),
				nearbyRow(2, service.SeverityLow, service.CategoryFlood, 2.0),
				nearbyRow(3, service.SeverityLow, service.CategoryFlood, 3.0),
			},
			reporterRows: []sqlcgen.ReporterAccountsForReportsRow{
				reporterRow(1, ptrID(reporterID)),
				reporterRow(2, nil),
				reporterRow(3, nil),
			},
			voteRows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, reporterID, service.VoteKindResolution, service.VoteResolve, "cellR"),
			},
		}
	}

	t.Run("a report the reporter has resolved is absent from both the default and the disputed views", func(t *testing.T) {
		fake := build(service.SeverityLow, service.CategoryFlood)
		svc := service.NewReportService(fake)

		for _, includeDisputed := range []bool{false, true} {
			views, err := svc.Nearby(context.Background(), service.NearbyQuery{
				Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: includeDisputed,
			})
			if err != nil {
				t.Fatalf("Nearby: %v", err)
			}
			if containsReportID(views, 1) {
				t.Errorf("includeDisputed=%v: report 1 (retracted) is present, want absent", includeDisputed)
			}
			// Reports 2 and 3 are Provisional (unvoted), so under the
			// partition they belong to the default view only — the
			// disputed view is Hidden alone.
			wantPresent := !includeDisputed
			if got := containsReportID(views, 2); got != wantPresent {
				t.Errorf("includeDisputed=%v: report 2 present=%v, want %v", includeDisputed, got, wantPresent)
			}
			if got := containsReportID(views, 3); got != wantPresent {
				t.Errorf("includeDisputed=%v: report 3 present=%v, want %v", includeDisputed, got, wantPresent)
			}
		}
	})

	t.Run("severity does not rescue a resolved report — critical/rescue_needed is retracted anyway (D-07)", func(t *testing.T) {
		fake := build(service.SeverityCritical, service.CategoryRescueNeeded)
		svc := service.NewReportService(fake)

		for _, includeDisputed := range []bool{false, true} {
			views, err := svc.Nearby(context.Background(), service.NearbyQuery{
				Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: includeDisputed,
			})
			if err != nil {
				t.Fatalf("Nearby: %v", err)
			}
			if containsReportID(views, 1) {
				t.Errorf("includeDisputed=%v: critical/rescue_needed retracted report is present, want absent (D-07 outranks the critical bypass)", includeDisputed)
			}
		}
	})
}

func TestNearbyHidesDisputedUnlessRequested(t *testing.T) {
	fake := &fakeFeedQuerier{
		nearbyRows: []sqlcgen.NearbyReportsRow{
			nearbyRow(1, service.SeverityLow, service.CategoryFlood, 1.0), // disputed past the threshold
			nearbyRow(2, service.SeverityLow, service.CategoryFlood, 2.0), // no votes at all
			nearbyRow(3, service.SeverityLow, service.CategoryFlood, 3.0), // confirmed past the threshold
		},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{
			reporterRow(1, nil), reporterRow(2, nil), reporterRow(3, nil),
		},
	}
	fake.voteRows = append(fake.voteRows, distinctVoteRows(1, service.VoteKindContent, service.VoteDispute, service.IndependentAgreementThreshold, 11)...)
	fake.voteRows = append(fake.voteRows, distinctVoteRows(3, service.VoteKindContent, service.VoteConfirm, service.IndependentAgreementThreshold, 21)...)
	svc := service.NewReportService(fake)

	t.Run("a report disputed past the threshold is absent by default", func(t *testing.T) {
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: false})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if containsReportID(views, 1) {
			t.Errorf("report 1 (hidden) present in the default view, want absent")
		}
	})

	t.Run("show_disputed reveals the hidden report with its visibility and reason", func(t *testing.T) {
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		v, ok := findView(views, 1)
		if !ok {
			t.Fatalf("report 1 absent under show_disputed, want present")
		}
		if v.Visibility != service.VisibilityHidden || v.VisibilityReason != service.ReasonDisputed {
			t.Errorf("got visibility=%q reason=%q, want %q/%q", v.Visibility, v.VisibilityReason, service.VisibilityHidden, service.ReasonDisputed)
		}
	})

	t.Run("an unvoted report is present with Provisional in the default view and absent under show_disputed — the toggle replaces the view, it does not add to it", func(t *testing.T) {
		defViews, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: false})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		v, ok := findView(defViews, 2)
		if !ok {
			t.Fatalf("default view: report 2 absent, want present")
		}
		if v.Visibility != service.VisibilityProvisional || v.VisibilityReason != service.ReasonAwaitingConfirmation {
			t.Errorf("default view: got visibility=%q reason=%q, want %q/%q", v.Visibility, v.VisibilityReason, service.VisibilityProvisional, service.ReasonAwaitingConfirmation)
		}

		disputedViews, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if containsReportID(disputedViews, 2) {
			t.Errorf("show_disputed view: report 2 (provisional) present, want absent — the disputed view is Hidden alone")
		}
	})

	t.Run("a confirmed report is present with Live in the default view and absent under show_disputed — the toggle replaces the view, it does not add to it", func(t *testing.T) {
		defViews, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: false})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		v, ok := findView(defViews, 3)
		if !ok {
			t.Fatalf("default view: report 3 absent, want present")
		}
		if v.Visibility != service.VisibilityLive || v.VisibilityReason != service.ReasonConfirmed {
			t.Errorf("default view: got visibility=%q reason=%q, want %q/%q", v.Visibility, v.VisibilityReason, service.VisibilityLive, service.ReasonConfirmed)
		}

		disputedViews, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if containsReportID(disputedViews, 3) {
			t.Errorf("show_disputed view: report 3 (live) present, want absent — the disputed view is Hidden alone")
		}
	})

	t.Run("the default and disputed views partition the three seeded reports: their union is all three ids and their intersection is empty", func(t *testing.T) {
		defViews, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: false})
		if err != nil {
			t.Fatalf("Nearby (default): %v", err)
		}
		disputedViews, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true})
		if err != nil {
			t.Fatalf("Nearby (disputed): %v", err)
		}

		defIDs := viewIDs(defViews)
		disputedIDs := viewIDs(disputedViews)

		seen := map[int64]int{}
		for _, id := range defIDs {
			seen[id]++
		}
		for _, id := range disputedIDs {
			seen[id]++
		}
		wantIDs := []int64{1, 2, 3}
		for _, id := range wantIDs {
			if seen[id] == 0 {
				t.Errorf("report %d present in neither view, want present in exactly one; default=%v disputed=%v", id, defIDs, disputedIDs)
			}
			if seen[id] > 1 {
				t.Errorf("report %d present in both views, want present in exactly one; default=%v disputed=%v", id, defIDs, disputedIDs)
			}
		}
		if len(seen) != len(wantIDs) {
			t.Errorf("union of default and disputed views = %v, want exactly %v", seen, wantIDs)
		}
	})
}

func TestNearbyBatchesVoteReadsOnce(t *testing.T) {
	t.Run("a 25-report page costs exactly one batched vote read and one batched reporter read", func(t *testing.T) {
		rows := make([]sqlcgen.NearbyReportsRow, 0, 25)
		reporterRows := make([]sqlcgen.ReporterAccountsForReportsRow, 0, 25)
		for i := int64(1); i <= 25; i++ {
			rows = append(rows, nearbyRow(i, service.SeverityLow, service.CategoryFlood, float64(i)))
			reporterRows = append(reporterRows, reporterRow(i, nil))
		}
		fake := &fakeFeedQuerier{nearbyRows: rows, reporterRows: reporterRows}
		svc := service.NewReportService(fake)

		if _, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, ViewerAccountID: 30}); err != nil {
			t.Fatalf("Nearby: %v", err)
		}

		if got := countCalls(fake.callLog, "CurrentVotesForReports"); got != 1 {
			t.Errorf("CurrentVotesForReports called %d times, want 1", got)
		}
		if got := countCalls(fake.callLog, "ReporterAccountsForReports"); got != 1 {
			t.Errorf("ReporterAccountsForReports called %d times, want 1", got)
		}
		if len(fake.lastVoteIDs) != 25 {
			t.Errorf("CurrentVotesForReports received %d ids, want 25", len(fake.lastVoteIDs))
		}
		if len(fake.lastReporterIDs) != 25 {
			t.Errorf("ReporterAccountsForReports received %d ids, want 25", len(fake.lastReporterIDs))
		}
		// U11, V-20: the moderation extras ride one more batched read over the
		// page's own ids and the viewer's account id.
		if got := countCalls(fake.callLog, "ReportExtrasForReports"); got != 1 {
			t.Errorf("ReportExtrasForReports called %d times, want 1", got)
		}
		if len(fake.extrasArgs) == 1 {
			wantIDs := make([]int64, 0, 25)
			for i := int64(1); i <= 25; i++ {
				wantIDs = append(wantIDs, i)
			}
			if !equalIDs(fake.extrasArgs[0].ReportIds, wantIDs) {
				t.Errorf("extras ReportIds = %v, want the 25 page ids in order", fake.extrasArgs[0].ReportIds)
			}
			if fake.extrasArgs[0].ViewerAccountID != 30 {
				t.Errorf("extras ViewerAccountID = %d, want 30", fake.extrasArgs[0].ViewerAccountID)
			}
		}
	})

	t.Run("an empty page costs neither batched call", func(t *testing.T) {
		fake := &fakeFeedQuerier{}
		svc := service.NewReportService(fake)

		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if len(views) != 0 {
			t.Fatalf("expected 0 views, got %d", len(views))
		}
		if got := countCalls(fake.callLog, "CurrentVotesForReports"); got != 0 {
			t.Errorf("CurrentVotesForReports called %d times, want 0", got)
		}
		if got := countCalls(fake.callLog, "ReporterAccountsForReports"); got != 0 {
			t.Errorf("ReporterAccountsForReports called %d times, want 0", got)
		}
		if got := countCalls(fake.callLog, "ReportExtrasForReports"); got != 0 {
			t.Errorf("ReportExtrasForReports called %d times, want 0", got)
		}
	})
}

func TestNearbyOrderingIsIndependentOfVisibility(t *testing.T) {
	reporterID3 := int64(300)
	fake := &fakeFeedQuerier{
		nearbyRows: []sqlcgen.NearbyReportsRow{
			nearbyRow(1, service.SeverityLow, service.CategoryFlood, 1.0),      // live
			nearbyRow(2, service.SeverityLow, service.CategoryFlood, 2.0),      // hidden
			nearbyRow(3, service.SeverityLow, service.CategoryFlood, 3.0),      // retracted
			nearbyRow(4, service.SeverityMedium, service.CategoryFlood, 4.0),   // provisional
			nearbyRow(5, service.SeverityCritical, service.CategoryFlood, 5.0), // critical bypass, farthest
		},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{
			reporterRow(1, nil), reporterRow(2, nil), reporterRow(3, ptrID(reporterID3)), reporterRow(4, nil), reporterRow(5, nil),
		},
	}
	fake.voteRows = append(fake.voteRows, distinctVoteRows(1, service.VoteKindContent, service.VoteConfirm, service.IndependentAgreementThreshold, 11)...)
	fake.voteRows = append(fake.voteRows, distinctVoteRows(2, service.VoteKindContent, service.VoteDispute, service.IndependentAgreementThreshold, 21)...)
	fake.voteRows = append(fake.voteRows, voteRow(3, reporterID3, service.VoteKindResolution, service.VoteResolve, "cellR"))
	svc := service.NewReportService(fake)

	t.Run("the default view stays ascending by distance and the farthest critical report does not move to the front", func(t *testing.T) {
		// The farthest report (id 5, distance 5.0) is critical-severity.
		// Severity-first triage ordering is a CLIENT-side concern
		// (web/static/js/feed.js's severityRank sort, 01-CONTEXT.md D-08)
		// applied to whatever this endpoint returns; this plan changes
		// neither the SQL ORDER BY nor feed.js, so filtering by visibility
		// cannot reorder anything.
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		wantIDs := []int64{1, 4, 5}
		if got := viewIDs(views); !equalIDs(got, wantIDs) {
			t.Fatalf("got %v, want %v", got, wantIDs)
		}
		assertAscendingDistance(t, views)
	})

	t.Run("the show_disputed view also stays ascending by distance, and contains only the Hidden report (partition, not union)", func(t *testing.T) {
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		wantIDs := []int64{2}
		if got := viewIDs(views); !equalIDs(got, wantIDs) {
			t.Fatalf("got %v, want %v", got, wantIDs)
		}
		assertAscendingDistance(t, views)
	})
}

func TestNearbySeverityDoesNotChangeGatingAmongNonCritical(t *testing.T) {
	t.Run("two disputed reports differing only in severity are both hidden by default and both revealed together", func(t *testing.T) {
		fake := &fakeFeedQuerier{
			nearbyRows: []sqlcgen.NearbyReportsRow{
				nearbyRow(1, service.SeverityLow, service.CategoryFlood, 1.0),
				nearbyRow(2, service.SeverityMedium, service.CategoryFlood, 2.0),
			},
			reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(1, nil), reporterRow(2, nil)},
		}
		fake.voteRows = append(fake.voteRows, distinctVoteRows(1, service.VoteKindContent, service.VoteDispute, service.IndependentAgreementThreshold, 11)...)
		fake.voteRows = append(fake.voteRows, distinctVoteRows(2, service.VoteKindContent, service.VoteDispute, service.IndependentAgreementThreshold, 21)...)
		svc := service.NewReportService(fake)

		def, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if containsReportID(def, 1) || containsReportID(def, 2) {
			t.Fatalf("expected both reports absent from the default view, got %v", viewIDs(def))
		}

		disputed, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if !containsReportID(disputed, 1) || !containsReportID(disputed, 2) {
			t.Fatalf("expected both reports present under show_disputed, got %v", viewIDs(disputed))
		}
	})

	t.Run("one confirm cell each leaves both provisional regardless of severity — bumping severity does not clear the gate", func(t *testing.T) {
		fake := &fakeFeedQuerier{
			nearbyRows: []sqlcgen.NearbyReportsRow{
				nearbyRow(1, service.SeverityLow, service.CategoryFlood, 1.0),
				nearbyRow(2, service.SeverityMedium, service.CategoryFlood, 2.0),
			},
			reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(1, nil), reporterRow(2, nil)},
		}
		fake.voteRows = append(fake.voteRows, distinctVoteRows(1, service.VoteKindContent, service.VoteConfirm, service.IndependentAgreementThreshold-1, 11)...)
		fake.voteRows = append(fake.voteRows, distinctVoteRows(2, service.VoteKindContent, service.VoteConfirm, service.IndependentAgreementThreshold-1, 21)...)
		svc := service.NewReportService(fake)

		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		for _, id := range []int64{1, 2} {
			v, ok := findView(views, id)
			if !ok {
				t.Fatalf("report %d absent, want present", id)
			}
			if v.Visibility != service.VisibilityProvisional {
				t.Errorf("report %d: visibility = %q, want %q — bumping severity from low to medium must not clear the gate", id, v.Visibility, service.VisibilityProvisional)
			}
		}
	})
}

func TestNearbyMarksOwnReportAndViewerVote(t *testing.T) {
	reporterID1 := int64(7)
	reporterID2 := int64(9)
	fake := &fakeFeedQuerier{
		nearbyRows: []sqlcgen.NearbyReportsRow{
			nearbyRow(1, service.SeverityLow, service.CategoryFlood, 1.0),
			nearbyRow(2, service.SeverityLow, service.CategoryFlood, 2.0),
			nearbyRow(3, service.SeverityLow, service.CategoryFlood, 3.0),
		},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{
			reporterRow(1, ptrID(reporterID1)),
			reporterRow(2, ptrID(reporterID2)),
			reporterRow(3, nil),
		},
		voteRows: []sqlcgen.CurrentVotesForReportsRow{
			voteRow(2, 7, service.VoteKindContent, service.VoteConfirm, "cellX"),
		},
	}
	svc := service.NewReportService(fake)

	t.Run("is_own_report is true only for the report the viewer actually reported", func(t *testing.T) {
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: false, ViewerAccountID: 7})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		v1, ok1 := findView(views, 1)
		v2, ok2 := findView(views, 2)
		if !ok1 || !ok2 {
			t.Fatalf("expected reports 1 and 2 present, got %v", viewIDs(views))
		}
		if !v1.IsOwnReport {
			t.Errorf("report 1: IsOwnReport = false, want true")
		}
		if v2.IsOwnReport {
			t.Errorf("report 2: IsOwnReport = true, want false")
		}
	})

	t.Run("a nil reporter account id is never own-report for any viewer, including viewer 0", func(t *testing.T) {
		for _, viewer := range []int64{7, 0} {
			views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: false, ViewerAccountID: viewer})
			if err != nil {
				t.Fatalf("Nearby: %v", err)
			}
			v3, ok := findView(views, 3)
			if !ok {
				t.Fatalf("report 3 absent, want present")
			}
			if v3.IsOwnReport {
				t.Errorf("viewer %d: report 3 (nil reporter) IsOwnReport = true, want false", viewer)
			}
		}
	})

	t.Run("the viewer's own confirm vote is echoed back on the report it was cast on, and nowhere else", func(t *testing.T) {
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: false, ViewerAccountID: 7})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		v1, _ := findView(views, 1)
		v2, _ := findView(views, 2)
		if v1.ViewerVote != "" {
			t.Errorf("report 1: ViewerVote = %q, want empty", v1.ViewerVote)
		}
		if v2.ViewerVote != service.VoteConfirm {
			t.Errorf("report 2: ViewerVote = %q, want %q", v2.ViewerVote, service.VoteConfirm)
		}
	})

	t.Run("viewer account id 0 yields an empty vote and false is_own_report for every report, with no panic", func(t *testing.T) {
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: false, ViewerAccountID: 0})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		for _, v := range views {
			if v.IsOwnReport {
				t.Errorf("report %d: IsOwnReport = true for viewer 0, want false", v.ID)
			}
			if v.ViewerVote != "" {
				t.Errorf("report %d: ViewerVote = %q for viewer 0, want empty", v.ID, v.ViewerVote)
			}
		}
	})
}

// The p12 fixtures belong to plan 03-12. The times are in 2031 on purpose, so
// only an injected clock can make a report read as expired or unexpired.
var (
	p12Now    = time.Date(2031, 3, 1, 12, 0, 0, 0, time.UTC)
	p12Judged = p12Now.Add(-48 * time.Hour)
)

// p12Row is a nearby row created an hour before p12Now that expires 7 hours
// after it.
func p12Row(id int64, severity service.Severity, category service.Category, distanceKm float64) sqlcgen.NearbyReportsRow {
	row := nearbyRow(id, severity, category, distanceKm)
	row.CreatedAt = p12Now.Add(-time.Hour)
	row.ExpiresAt = p12Now.Add(7 * time.Hour)
	return row
}

// p12Vote is a vote row cast 20 minutes before p12Now.
func p12Vote(reportID, accountID int64, kind service.VoteKind, value service.VoteValue, cell string) sqlcgen.CurrentVotesForReportsRow {
	return sqlcgen.CurrentVotesForReportsRow{
		ReportID:    reportID,
		AccountID:   accountID,
		Kind:        string(kind),
		Value:       string(value),
		GeohashCell: cell,
		CreatedAt:   p12Now.Add(-20 * time.Minute),
	}
}

// p12Places returns n content votes on one report from distinct accounts in
// distinct cells, so the independent place count is exactly n.
func p12Places(reportID int64, value service.VoteValue, n int, firstAccount int64) []sqlcgen.CurrentVotesForReportsRow {
	rows := make([]sqlcgen.CurrentVotesForReportsRow, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, p12Vote(reportID, firstAccount+int64(i), service.VoteKindContent, value, fmt.Sprintf("p12cell%d", i)))
	}
	return rows
}

// p12History is a low severity flood history row.
func p12History(accountID, reportID int64, expiresAt time.Time) sqlcgen.ReporterHistoryReportsRow {
	return sqlcgen.ReporterHistoryReportsRow{
		AccountID: accountID,
		ReportID:  reportID,
		Severity:  string(service.SeverityLow),
		Category:  string(service.CategoryFlood),
		ExpiresAt: expiresAt,
	}
}

// p12Corroborated returns n judged history reports of one account, each
// confirmed by accounts 21 and 22 in two distinct cells and never by the
// reporter, together with those confirming votes.
func p12Corroborated(accountID, firstReportID int64, n int) ([]sqlcgen.ReporterHistoryReportsRow, []sqlcgen.CurrentVotesForReportsRow) {
	history := make([]sqlcgen.ReporterHistoryReportsRow, 0, n)
	var votes []sqlcgen.CurrentVotesForReportsRow
	for i := 0; i < n; i++ {
		id := firstReportID + int64(i)
		history = append(history, p12History(accountID, id, p12Judged))
		votes = append(votes, p12Places(id, service.VoteConfirm, service.IndependentAgreementThreshold, 21)...)
	}
	return history, votes
}

// p12Unconfirmed returns n judged history reports of one account that nobody
// voted on.
func p12Unconfirmed(accountID, firstReportID int64, n int) []sqlcgen.ReporterHistoryReportsRow {
	history := make([]sqlcgen.ReporterHistoryReportsRow, 0, n)
	for i := 0; i < n; i++ {
		history = append(history, p12History(accountID, firstReportID+int64(i), p12Judged))
	}
	return history
}

// p12VoteContext is the vote context of a low severity flood report at the
// feed fixtures' point, created an hour before p12Now.
func p12VoteContext(reporter *int64) sqlcgen.ReportVoteContextRow {
	return sqlcgen.ReportVoteContextRow{
		Severity:          string(service.SeverityLow),
		Category:          string(service.CategoryFlood),
		ExpiresAt:         p12Now.Add(7 * time.Hour),
		ReporterAccountID: reporter,
		Latitude:          12.9716,
		Longitude:         77.5946,
		CreatedAt:         p12Now.Add(-time.Hour),
	}
}

// p12ActivityRow is the Activity row that matches a p12Row report.
func p12ActivityRow(id int64) sqlcgen.ReportsByAccountRow {
	return sqlcgen.ReportsByAccountRow{
		ID:          id,
		Category:    string(service.CategoryFlood),
		Severity:    string(service.SeverityLow),
		Description: "a p12 activity report",
		Latitude:    12.9716,
		Longitude:   77.5946,
		CreatedAt:   p12Now.Add(-time.Hour),
		ExpiresAt:   p12Now.Add(7 * time.Hour),
	}
}

func p12Clock() time.Time { return p12Now }

// p12ReporterOf returns the reporter signal of one listed report.
func p12ReporterOf(t *testing.T, views []service.ReportView, id int64) service.ReporterSignal {
	t.Helper()
	v, ok := findView(views, id)
	if !ok {
		t.Fatalf("report %d is not listed in %v", id, viewIDs(views))
	}
	return v.Trust.Reporter
}

// TestNearbyBatchesVoteReadsOnceWithReporters is V-29, the batching part: a
// feed read makes one reporter read, at most one history read and one vote
// read, and the vote read covers the page ids and then the new history ids.
func TestNearbyBatchesVoteReadsOnceWithReporters(t *testing.T) {
	newPage := func(reporters []*int64) *fakeFeedQuerier {
		f := &fakeFeedQuerier{}
		for i, rep := range reporters {
			id := int64(i + 1)
			f.nearbyRows = append(f.nearbyRows, p12Row(id, service.SeverityLow, service.CategoryFlood, float64(id)))
			f.reporterRows = append(f.reporterRows, reporterRow(id, rep))
		}
		return f
	}
	query := service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5}

	t.Run("one call each, in the order reporters, history, votes, with the history ids joined to the vote read", func(t *testing.T) {
		fake := newPage([]*int64{ptrID(7), ptrID(8), nil, ptrID(7), ptrID(9), ptrID(8)})
		corroborated, corroboratingVotes := p12Corroborated(7, 701, 1)
		fake.historyRows = append(fake.historyRows, corroborated...)
		fake.historyRows = append(fake.historyRows, p12History(7, 702, p12Judged), p12History(8, 801, p12Judged), p12History(9, 901, p12Judged), p12History(9, 5, p12Judged))
		fake.voteRows = append(fake.voteRows, corroboratingVotes...)
		fake.voteRows = append(fake.voteRows, p12Places(1, service.VoteConfirm, 2, 31)...)
		svc := service.NewReportService(fake, service.WithReportClock(p12Clock))

		views, err := svc.Nearby(context.Background(), query)
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}

		wantLog := []string{"NearbyReports", "ReporterAccountsForReports", "ReporterHistoryReports", "CurrentVotesForReports", "ReportExtrasForReports"}
		if !slices.Equal(fake.callLog, wantLog) {
			t.Fatalf("callLog = %v, want %v", fake.callLog, wantLog)
		}
		wantArgs := sqlcgen.ReporterHistoryReportsParams{
			Now:         p12Now,
			WindowStart: p12Now.Add(-service.ReliabilityWindow),
			RowCap:      int32(service.ReliabilityHistoryReadCap),
			AccountIds:  []int64{7, 8, 9},
		}
		if len(fake.historyArgs) != 1 || !reflect.DeepEqual(fake.historyArgs[0], wantArgs) {
			t.Fatalf("historyArgs = %+v, want one entry %+v", fake.historyArgs, wantArgs)
		}
		if want := []int64{1, 2, 3, 4, 5, 6, 701, 702, 801, 901}; !equalIDs(fake.lastVoteIDs, want) {
			t.Errorf("lastVoteIDs = %v, want %v", fake.lastVoteIDs, want)
		}

		want := service.ReporterSignals([]int64{7, 8, 9}, fake.historyRows, fake.voteRows, p12Now)[7]
		for _, id := range []int64{1, 4} {
			if got := p12ReporterOf(t, views, id); !reflect.DeepEqual(got, want) {
				t.Errorf("report %d reporter = %+v, want %+v", id, got, want)
			}
		}
		if got := p12ReporterOf(t, views, 3); !reflect.DeepEqual(got, service.ReporterSignal{Tag: service.ReporterNone}) {
			t.Errorf("accountless report 3 reporter = %+v, want none", got)
		}
	})

	t.Run("a page with no reporter account makes no history call", func(t *testing.T) {
		fake := newPage([]*int64{nil, nil, nil})
		svc := service.NewReportService(fake, service.WithReportClock(p12Clock))

		views, err := svc.Nearby(context.Background(), query)
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if got := countCalls(fake.callLog, "ReporterHistoryReports"); got != 0 {
			t.Errorf("ReporterHistoryReports called %d times, want 0", got)
		}
		if want := []int64{1, 2, 3}; !equalIDs(fake.lastVoteIDs, want) {
			t.Errorf("lastVoteIDs = %v, want %v", fake.lastVoteIDs, want)
		}
		for _, v := range views {
			if v.Trust.Reporter.Tag != service.ReporterNone {
				t.Errorf("report %d tag = %q, want none", v.ID, v.Trust.Reporter.Tag)
			}
		}
	})

	t.Run("an empty page makes none of the three batched calls", func(t *testing.T) {
		fake := &fakeFeedQuerier{}
		svc := service.NewReportService(fake, service.WithReportClock(p12Clock))

		if _, err := svc.Nearby(context.Background(), query); err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		for _, name := range []string{"ReporterAccountsForReports", "ReporterHistoryReports", "CurrentVotesForReports", "ReportExtrasForReports"} {
			if got := countCalls(fake.callLog, name); got != 0 {
				t.Errorf("%s called %d times, want 0", name, got)
			}
		}
	})

	t.Run("a failed history read is returned and no votes are read", func(t *testing.T) {
		boom := errors.New("history unavailable")
		fake := newPage([]*int64{ptrID(7)})
		fake.historyErr = boom
		svc := service.NewReportService(fake, service.WithReportClock(p12Clock))

		views, err := svc.Nearby(context.Background(), query)
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
		if views != nil {
			t.Errorf("views = %v, want nil", views)
		}
		if got := countCalls(fake.callLog, "CurrentVotesForReports"); got != 0 {
			t.Errorf("CurrentVotesForReports called %d times, want 0", got)
		}
	})
}

// TestReporterSignalKeyedByAccount is V-28, the service part (D-10, R-05,
// T-03-10): every read path asks for history by the verified account id, so
// two reports by one account carry one signal and one account's history never
// moves another account's signal.
func TestReporterSignalKeyedByAccount(t *testing.T) {
	ctx := context.Background()
	query := service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5}
	wantSentence := "5 of their last 5 reports were confirmed."

	hist7, votes7 := p12Corroborated(7, 701, 5)
	hist8 := []sqlcgen.ReporterHistoryReportsRow{
		p12History(8, 801, p12Judged),
		p12History(8, 802, p12Now),
	}
	pageVotes := p12Places(1, service.VoteConfirm, 1, 31)
	history := append(append([]sqlcgen.ReporterHistoryReportsRow(nil), hist7...), hist8...)
	votes := append(append([]sqlcgen.CurrentVotesForReportsRow(nil), votes7...), pageVotes...)

	// Reports 1 and 2 stand for two reports of one verified account filed from
	// two different sessions: the service only sees the account id.
	nearbyFake := func(history []sqlcgen.ReporterHistoryReportsRow, votes []sqlcgen.CurrentVotesForReportsRow) *fakeFeedQuerier {
		return &fakeFeedQuerier{
			nearbyRows: []sqlcgen.NearbyReportsRow{
				p12Row(1, service.SeverityLow, service.CategoryFlood, 1),
				p12Row(2, service.SeverityLow, service.CategoryFlood, 2),
				p12Row(3, service.SeverityLow, service.CategoryFlood, 3),
			},
			reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(1, ptrID(7)), reporterRow(2, ptrID(7)), reporterRow(3, ptrID(8))},
			historyRows:  history,
			voteRows:     votes,
		}
	}
	runNearby := func(t *testing.T, history []sqlcgen.ReporterHistoryReportsRow, votes []sqlcgen.CurrentVotesForReportsRow) ([]service.ReportView, *fakeFeedQuerier) {
		t.Helper()
		fake := nearbyFake(history, votes)
		views, err := service.NewReportService(fake, service.WithReportClock(p12Clock)).Nearby(ctx, query)
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		return views, fake
	}

	views, fake := runNearby(t, history, votes)
	var reliable service.ReporterSignal

	t.Run("two reports by one account carry one signal built from that account's history", func(t *testing.T) {
		if len(fake.historyArgs) != 1 || !slices.Equal(fake.historyArgs[0].AccountIds, []int64{7, 8}) {
			t.Fatalf("historyArgs = %+v, want one call for accounts [7 8]", fake.historyArgs)
		}
		reliable = p12ReporterOf(t, views, 1)
		if !reflect.DeepEqual(reliable, p12ReporterOf(t, views, 2)) {
			t.Errorf("reports 1 and 2 differ: %+v and %+v", reliable, p12ReporterOf(t, views, 2))
		}
		if reliable.Tag != service.ReporterReliable || reliable.Score == nil || *reliable.Score != 86 ||
			reliable.Label != "Reliable reporter" || reliable.Sentence != wantSentence {
			t.Errorf("reporter = %+v, want reliable 86 %q", reliable, wantSentence)
		}
	})

	t.Run("a report expiring exactly at now counts as judged and another account's history does not leak", func(t *testing.T) {
		got := p12ReporterOf(t, views, 3)
		if got.Tag != service.ReporterNew || got.Score != nil || got.JudgedReports != 2 ||
			got.Sentence != "Not rated yet. It needs 5 reports with a clear result." {
			t.Errorf("report 3 reporter = %+v", got)
		}
	})

	t.Run("changing account 8's history leaves account 7's signal unchanged", func(t *testing.T) {
		hist8Full, votes8Full := p12Corroborated(8, 801, 5)
		swapped := append(append([]sqlcgen.ReporterHistoryReportsRow(nil), hist7...), hist8Full...)
		swappedVotes := append(append(append([]sqlcgen.CurrentVotesForReportsRow(nil), votes7...), votes8Full...), pageVotes...)
		again, _ := runNearby(t, swapped, swappedVotes)
		if got := p12ReporterOf(t, again, 1); !reflect.DeepEqual(got, reliable) {
			t.Errorf("report 1 reporter = %+v, want %+v", got, reliable)
		}
		if got := p12ReporterOf(t, again, 3); got.Tag != service.ReporterReliable {
			t.Errorf("report 3 tag = %q, want reliable", got.Tag)
		}
	})

	t.Run("the Activity page reads history for the gate's account and every row carries its signal", func(t *testing.T) {
		q, m, calls := newFakes()
		q.reportsRows = []sqlcgen.ReportsByAccountRow{p12ActivityRow(1), p12ActivityRow(2)}
		q.historyRows = history
		q.votesRows = votes
		svc := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(p12Clock))

		acts, err := svc.ActivityForAccount(ctx, 7)
		if err != nil {
			t.Fatalf("ActivityForAccount: %v", err)
		}
		if countCalls(*calls, "ReporterHistoryReports") != 1 || len(q.historyArgs) != 1 {
			t.Fatalf("history calls = %d (%v), want 1", len(q.historyArgs), *calls)
		}
		wantArgs := sqlcgen.ReporterHistoryReportsParams{
			Now:         p12Now,
			WindowStart: p12Now.Add(-service.ReliabilityWindow),
			RowCap:      int32(service.ReliabilityHistoryReadCap),
			AccountIds:  []int64{7},
		}
		if !reflect.DeepEqual(q.historyArgs[0], wantArgs) {
			t.Errorf("historyArgs[0] = %+v, want %+v", q.historyArgs[0], wantArgs)
		}
		if len(acts) != 2 {
			t.Fatalf("got %d rows, want 2", len(acts))
		}
		for _, a := range acts {
			if !reflect.DeepEqual(a.Trust.Reporter, reliable) {
				t.Errorf("row %d reporter = %+v, want %+v", a.Row.ID, a.Trust.Reporter, reliable)
			}
		}
	})

	t.Run("a confirm vote reads history for the report's reporter account", func(t *testing.T) {
		vf := &p05VotingFake{
			fakeFeedQuerier: &fakeFeedQuerier{voteRows: votes, historyRows: history},
			voteContext:     p12VoteContext(ptrID(7)),
		}
		svc := service.NewVotingService(vf, service.WithVotingClock(p12Clock))

		res, err := svc.CastVote(ctx, service.CastVoteInput{
			ReportID: 1, AccountID: 30, Kind: service.VoteKindContent, Value: service.VoteConfirm,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("CastVote: %v", err)
		}
		if len(vf.historyArgs) != 1 || !slices.Equal(vf.historyArgs[0].AccountIds, []int64{7}) {
			t.Fatalf("historyArgs = %+v, want one call for [7]", vf.historyArgs)
		}
		if !reflect.DeepEqual(res.Trust.Reporter, reliable) {
			t.Errorf("result reporter = %+v, want %+v", res.Trust.Reporter, reliable)
		}
	})
}

// TestCastVoteTrustMatchesNearbyTrust is D-21 for the reporter signal: for the
// same rows, history and clock, the feed, the Activity page and the vote
// response return deeply equal trust objects.
func TestCastVoteTrustMatchesNearbyTrust(t *testing.T) {
	ctx := context.Background()
	const reportID int64 = 51
	votes := []sqlcgen.CurrentVotesForReportsRow{
		p12Vote(reportID, 11, service.VoteKindContent, service.VoteConfirm, "p12cellA"),
		p12Vote(reportID, 12, service.VoteKindContent, service.VoteConfirm, "p12cellB"),
		p12Vote(reportID, 13, service.VoteKindContent, service.VoteDispute, "p12cellC"),
	}

	type paths struct {
		nearby, cast service.Trust
		activity     *service.Trust
		feed         *fakeFeedQuerier
		vote         *p05VotingFake
	}
	run := func(t *testing.T, reporter *int64, history []sqlcgen.ReporterHistoryReportsRow, allVotes []sqlcgen.CurrentVotesForReportsRow) paths {
		t.Helper()
		var out paths

		out.feed = &fakeFeedQuerier{
			nearbyRows:   []sqlcgen.NearbyReportsRow{p12Row(reportID, service.SeverityLow, service.CategoryFlood, 0.2)},
			voteRows:     allVotes,
			reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(reportID, reporter)},
			historyRows:  history,
		}
		views, err := service.NewReportService(out.feed, service.WithReportClock(p12Clock)).Nearby(ctx, service.NearbyQuery{
			Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5,
		})
		if err != nil || len(views) != 1 {
			t.Fatalf("Nearby: %v (%d views)", err, len(views))
		}
		out.nearby = views[0].Trust

		if reporter != nil {
			q, m, _ := newFakes()
			q.reportsRows = []sqlcgen.ReportsByAccountRow{p12ActivityRow(reportID)}
			q.votesRows = allVotes
			q.historyRows = history
			acts, err := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(p12Clock)).ActivityForAccount(ctx, *reporter)
			if err != nil || len(acts) != 1 {
				t.Fatalf("ActivityForAccount: %v (%d rows)", err, len(acts))
			}
			tr := acts[0].Trust
			out.activity = &tr
		}

		out.vote = &p05VotingFake{
			fakeFeedQuerier: &fakeFeedQuerier{voteRows: allVotes, historyRows: history},
			voteContext:     p12VoteContext(reporter),
		}
		res, err := service.NewVotingService(out.vote, service.WithVotingClock(p12Clock)).CastVote(ctx, service.CastVoteInput{
			ReportID: reportID, AccountID: 12, Kind: service.VoteKindContent, Value: service.VoteConfirm,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("CastVote: %v", err)
		}
		out.cast = res.Trust
		return out
	}

	t.Run("a reporter with five corroborated reports reads reliable on all three paths", func(t *testing.T) {
		history, historyVotes := p12Corroborated(7, 701, 5)
		got := run(t, ptrID(7), history, append(append([]sqlcgen.CurrentVotesForReportsRow(nil), votes...), historyVotes...))
		if !reflect.DeepEqual(got.nearby, got.cast) || got.activity == nil || !reflect.DeepEqual(got.nearby, *got.activity) {
			t.Errorf("trust differs:\nnearby   %+v\nactivity %+v\ncast     %+v", got.nearby, got.activity, got.cast)
		}
		if r := got.nearby.Reporter; r.Tag != service.ReporterReliable || r.Score == nil || *r.Score != 86 {
			t.Errorf("reporter = %+v, want reliable 86", r)
		}
	})

	t.Run("a reporter with no history reads new on all three paths", func(t *testing.T) {
		got := run(t, ptrID(7), nil, votes)
		if !reflect.DeepEqual(got.nearby, got.cast) || got.activity == nil || !reflect.DeepEqual(got.nearby, *got.activity) {
			t.Errorf("trust differs:\nnearby   %+v\nactivity %+v\ncast     %+v", got.nearby, got.activity, got.cast)
		}
		r := got.nearby.Reporter
		if r.Tag != service.ReporterNew || r.JudgedReports != 0 ||
			r.Sentence != "Not rated yet. It needs 5 reports with a clear result." {
			t.Errorf("reporter = %+v, want new with no judged reports", r)
		}
	})

	t.Run("an accountless report reads none on the feed and the vote response and reads no history", func(t *testing.T) {
		got := run(t, nil, nil, votes)
		if !reflect.DeepEqual(got.nearby, got.cast) {
			t.Errorf("trust differs:\nnearby %+v\ncast   %+v", got.nearby, got.cast)
		}
		if want := (service.ReporterSignal{Tag: service.ReporterNone}); !reflect.DeepEqual(got.nearby.Reporter, want) {
			t.Errorf("reporter = %+v, want none", got.nearby.Reporter)
		}
		if n := countCalls(got.feed.callLog, "ReporterHistoryReports") + countCalls(got.vote.callLog, "ReporterHistoryReports"); n != 0 {
			t.Errorf("ReporterHistoryReports called %d times, want 0", n)
		}
	})
}

// p12WithoutReporter returns a copy of tr with the reporter signal reset, and
// the reporter row of its facts (plan 03-31) taken out, so a test can compare
// everything else on a trust object.
func p12WithoutReporter(tr service.Trust) service.Trust {
	tr.Reporter = service.ReporterSignal{}
	var facts []service.TrustFact
	for _, f := range tr.Facts {
		if f.Kind != service.FactReporter {
			facts = append(facts, f)
		}
	}
	tr.Facts = facts
	return tr
}

// TestNearbyReporterAccountCap is Q7 and T-03-05: a feed read loads history
// for at most MaxReporterAccountsPerRead accounts, nearest reports first, and a
// report past the cap carries the none signal and stays listed in its place.
func TestNearbyReporterAccountCap(t *testing.T) {
	if service.MaxReporterAccountsPerRead != 200 {
		t.Fatalf("MaxReporterAccountsPerRead = %d, want 200", service.MaxReporterAccountsPerRead)
	}
	const baseAccount int64 = 1000
	const baseHistory int64 = 100000
	n := service.MaxReporterAccountsPerRead + 1

	fake := &fakeFeedQuerier{}
	for i := 1; i <= n; i++ {
		id := int64(i)
		fake.nearbyRows = append(fake.nearbyRows, p12Row(id, service.SeverityLow, service.CategoryFlood, float64(i)))
		fake.reporterRows = append(fake.reporterRows, reporterRow(id, ptrID(baseAccount+id)))
		fake.historyRows = append(fake.historyRows, p12History(baseAccount+id, baseHistory+id, p12Judged))
	}
	farther := int64(n + 1)
	farthest := int64(n + 2)
	fake.nearbyRows = append(fake.nearbyRows,
		p12Row(farther, service.SeverityLow, service.CategoryFlood, float64(farther)),
		p12Row(farthest, service.SeverityLow, service.CategoryFlood, float64(farthest)),
	)
	fake.reporterRows = append(fake.reporterRows, reporterRow(farther, ptrID(baseAccount+1)), reporterRow(farthest, nil))

	views, err := service.NewReportService(fake, service.WithReportClock(p12Clock)).Nearby(context.Background(), service.NearbyQuery{
		Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 50,
	})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}

	if len(fake.historyArgs) != 1 {
		t.Fatalf("ReporterHistoryReports called %d times, want 1", len(fake.historyArgs))
	}
	var wantAccounts []int64
	for i := 1; i <= service.MaxReporterAccountsPerRead; i++ {
		wantAccounts = append(wantAccounts, baseAccount+int64(i))
	}
	got := fake.historyArgs[0].AccountIds
	if len(got) != service.MaxReporterAccountsPerRead || !equalIDs(got, wantAccounts) {
		t.Errorf("history accounts: %d of them, want the nearest %d in order", len(got), service.MaxReporterAccountsPerRead)
	}
	if slices.Contains(got, baseAccount+int64(n)) {
		t.Errorf("the over cap account %d was requested", baseAccount+int64(n))
	}
	if c := countCalls(fake.callLog, "CurrentVotesForReports"); c != 1 {
		t.Errorf("CurrentVotesForReports called %d times, want 1", c)
	}

	none := service.ReporterSignal{Tag: service.ReporterNone}
	if r := p12ReporterOf(t, views, int64(n)); !reflect.DeepEqual(r, none) {
		t.Errorf("over cap report %d reporter = %+v, want none", n, r)
	}
	first := p12ReporterOf(t, views, 1)
	if first.Tag == service.ReporterNone {
		t.Fatalf("report 1 reporter = %+v, want a real signal", first)
	}
	if r := p12ReporterOf(t, views, farther); !reflect.DeepEqual(r, first) {
		t.Errorf("farther report %d reporter = %+v, want report 1's %+v", farther, r, first)
	}
	if r := p12ReporterOf(t, views, farthest); !reflect.DeepEqual(r, none) {
		t.Errorf("accountless report %d reporter = %+v, want none", farthest, r)
	}

	if len(views) != n+2 {
		t.Fatalf("listed %d reports, want %d", len(views), n+2)
	}
	assertAscendingDistance(t, views)
	for _, v := range views {
		if v.Visibility != service.VisibilityProvisional {
			t.Errorf("report %d Visibility = %q, want provisional", v.ID, v.Visibility)
		}
	}
}

// TestNearbyRatingNeverChangesVisibilityOrOrder is D-14: the reporter signal is
// display only. Swapping one reporter between four ratings changes nothing but
// Trust.Reporter, in either view.
func TestNearbyRatingNeverChangesVisibilityOrOrder(t *testing.T) {
	const reporter, other int64 = 7, 300
	pageRows := []sqlcgen.NearbyReportsRow{
		p12Row(1, service.SeverityLow, service.CategoryFlood, 1),
		p12Row(2, service.SeverityLow, service.CategoryFlood, 2),
		p12Row(3, service.SeverityLow, service.CategoryFlood, 3),
		p12Row(4, service.SeverityMedium, service.CategoryFlood, 4),
		p12Row(5, service.SeverityCritical, service.CategoryFlood, 5),
	}
	pageReporters := []sqlcgen.ReporterAccountsForReportsRow{
		reporterRow(1, ptrID(reporter)), reporterRow(2, ptrID(reporter)), reporterRow(3, ptrID(other)),
		reporterRow(4, ptrID(reporter)), reporterRow(5, ptrID(reporter)),
	}
	var pageVotes []sqlcgen.CurrentVotesForReportsRow
	pageVotes = append(pageVotes, p12Places(1, service.VoteConfirm, service.IndependentAgreementThreshold, 11)...)
	pageVotes = append(pageVotes, p12Places(2, service.VoteDispute, service.IndependentAgreementThreshold, 31)...)
	pageVotes = append(pageVotes, p12Vote(3, other, service.VoteKindResolution, service.VoteResolve, "p12cellR"))

	type variant struct {
		name    string
		tag     service.ReporterTag
		score   int
		history []sqlcgen.ReporterHistoryReportsRow
		votes   []sqlcgen.CurrentVotesForReportsRow
	}
	build := func(corroborated, unconfirmed int) ([]sqlcgen.ReporterHistoryReportsRow, []sqlcgen.CurrentVotesForReportsRow) {
		var history []sqlcgen.ReporterHistoryReportsRow
		var votes []sqlcgen.CurrentVotesForReportsRow
		for i, account := range []int64{reporter, other} {
			base := int64(7000 + 1000*i)
			h, v := p12Corroborated(account, base, corroborated)
			history = append(history, h...)
			votes = append(votes, v...)
			history = append(history, p12Unconfirmed(account, base+100, unconfirmed)...)
		}
		return history, votes
	}
	var variants []variant
	variants = append(variants, variant{name: "no history", tag: service.ReporterNew, score: -1})
	h, v := build(5, 0)
	variants = append(variants, variant{name: "five corroborated", tag: service.ReporterReliable, score: 86, history: h, votes: v})
	h, v = build(3, 2)
	variants = append(variants, variant{name: "three corroborated and two unconfirmed", tag: service.ReporterMixed, score: 64, history: h, votes: v})
	h, v = build(0, 5)
	variants = append(variants, variant{name: "five unconfirmed", tag: service.ReporterUnreliable, score: 32, history: h, votes: v})

	run := func(t *testing.T, vr variant, includeDisputed bool) []service.ReportView {
		t.Helper()
		fake := &fakeFeedQuerier{
			nearbyRows:   pageRows,
			reporterRows: pageReporters,
			voteRows:     append(append([]sqlcgen.CurrentVotesForReportsRow(nil), pageVotes...), vr.votes...),
			historyRows:  vr.history,
		}
		views, err := service.NewReportService(fake, service.WithReportClock(p12Clock)).Nearby(context.Background(), service.NearbyQuery{
			Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, ViewerAccountID: reporter, IncludeDisputed: includeDisputed,
		})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		return views
	}

	for _, includeDisputed := range []bool{false, true} {
		view := "default view"
		if includeDisputed {
			view = "show disputed view"
		}
		t.Run(view, func(t *testing.T) {
			base := run(t, variants[0], includeDisputed)
			if len(base) == 0 {
				t.Fatal("the baseline run lists nothing")
			}
			tags := map[service.ReporterTag]bool{}
			for _, vr := range variants {
				got := run(t, vr, includeDisputed)
				if !equalIDs(viewIDs(got), viewIDs(base)) {
					t.Fatalf("%s: listed ids %v, want %v", vr.name, viewIDs(got), viewIDs(base))
				}
				assertAscendingDistance(t, got)
				for i := range got {
					g, b := got[i], base[i]
					if g.Visibility != b.Visibility || g.VisibilityReason != b.VisibilityReason ||
						g.ViewerVote != b.ViewerVote || g.IsOwnReport != b.IsOwnReport {
						t.Errorf("%s: report %d visibility, reason, viewer vote or ownership changed", vr.name, g.ID)
					}
					if !reflect.DeepEqual(p12WithoutReporter(g.Trust), p12WithoutReporter(b.Trust)) {
						t.Errorf("%s: report %d trust changed beyond the reporter:\n%+v\n%+v", vr.name, g.ID, g.Trust, b.Trust)
					}
					if g.ID == 5 && g.Trust.Currency.FadeStage != nil {
						t.Errorf("%s: critical report 5 FadeStage = %v, want nil", vr.name, *g.Trust.Currency.FadeStage)
					}
					if g.ID == 3 {
						t.Errorf("%s: retracted report 3 is listed", vr.name)
					}
				}
				if !includeDisputed {
					r := p12ReporterOf(t, got, 1)
					tags[r.Tag] = true
					if r.Tag != vr.tag {
						t.Errorf("%s: report 1 tag = %q, want %q", vr.name, r.Tag, vr.tag)
					}
					if vr.score >= 0 && (r.Score == nil || *r.Score != vr.score) {
						t.Errorf("%s: report 1 score = %v, want %d", vr.name, r.Score, vr.score)
					}
				}
			}
			if !includeDisputed && len(tags) != len(variants) {
				t.Errorf("report 1 showed %d distinct tags across the runs, want %d", len(tags), len(variants))
			}
		})
	}
}

// TestCategoryFilterNearby pins D-17 for the live list: a category chip is the
// reader's own request, so it narrows the list in Go after Resolve and keeps the
// nearest first order. Rescue and critical reports get no special rule.
func TestCategoryFilterNearby(t *testing.T) {
	newFake := func() *fakeFeedQuerier {
		return &fakeFeedQuerier{
			nearbyRows: []sqlcgen.NearbyReportsRow{
				p12Row(1, service.SeverityLow, service.CategoryFlood, 1.0),
				p12Row(2, service.SeverityCritical, service.CategoryRescueNeeded, 2.0),
				p12Row(3, service.SeverityMedium, service.CategoryShelterOpen, 3.0),
				p12Row(4, service.SeverityLow, service.CategoryFlood, 4.0),
			},
			reporterRows: []sqlcgen.ReporterAccountsForReportsRow{
				reporterRow(1, nil), reporterRow(2, nil), reporterRow(3, nil), reporterRow(4, nil),
			},
		}
	}
	run := func(t *testing.T, category service.Category) []service.ReportView {
		t.Helper()
		svc := service.NewReportService(newFake(), service.WithReportClock(p12Clock))
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, Category: category})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		return views
	}

	t.Run("flood keeps only flood reports in nearest first order", func(t *testing.T) {
		if got := viewIDs(run(t, service.CategoryFlood)); !equalIDs(got, []int64{1, 4}) {
			t.Fatalf("ids = %v, want [1 4]", got)
		}
	})
	t.Run("an empty category returns everything", func(t *testing.T) {
		got := run(t, "")
		if ids := viewIDs(got); !equalIDs(ids, []int64{1, 2, 3, 4}) {
			t.Fatalf("ids = %v, want [1 2 3 4]", ids)
		}
		assertAscendingDistance(t, got)
	})
	t.Run("rescue is filtered like any other category", func(t *testing.T) {
		if got := viewIDs(run(t, service.CategoryRescueNeeded)); !equalIDs(got, []int64{2}) {
			t.Fatalf("ids = %v, want [2]", got)
		}
	})
}

// TestNearbyFillsViewerResolution pins that every live row carries its view and
// the viewer's own standing resolution, while membership and order stay as the
// resolver left them.
func TestNearbyFillsViewerResolution(t *testing.T) {
	fake := &fakeFeedQuerier{
		nearbyRows: []sqlcgen.NearbyReportsRow{
			p12Row(1, service.SeverityLow, service.CategoryFlood, 1.0),
			p12Row(2, service.SeverityLow, service.CategoryFlood, 2.0),
			p12Row(3, service.SeverityLow, service.CategoryFlood, 3.0),
		},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(1, nil), reporterRow(2, nil), reporterRow(3, nil)},
	}
	fake.voteRows = append(fake.voteRows, p12Places(1, service.VoteConfirm, service.IndependentAgreementThreshold, 31)...)
	fake.voteRows = append(fake.voteRows, p12Places(2, service.VoteDispute, service.IndependentAgreementThreshold, 41)...)
	fake.voteRows = append(fake.voteRows, p12Vote(3, 7, service.VoteKindResolution, service.VoteResolve, "cellV"))
	svc := service.NewReportService(fake, service.WithReportClock(p12Clock))

	active, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, ViewerAccountID: 7})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	if ids := viewIDs(active); !equalIDs(ids, []int64{1, 3}) {
		t.Fatalf("active ids = %v, want [1 3]", ids)
	}
	for _, v := range active {
		if v.View != service.FeedViewActive {
			t.Errorf("report %d: View = %q, want %q", v.ID, v.View, service.FeedViewActive)
		}
		if v.History != nil {
			t.Errorf("report %d: History = %+v, want nil", v.ID, v.History)
		}
	}
	v3, _ := findView(active, 3)
	if v3.ViewerResolution != service.VoteResolve {
		t.Errorf("report 3: ViewerResolution = %q, want %q", v3.ViewerResolution, service.VoteResolve)
	}
	v1, _ := findView(active, 1)
	if v1.ViewerResolution != "" {
		t.Errorf("report 1: ViewerResolution = %q, want empty", v1.ViewerResolution)
	}

	disputed, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, ViewerAccountID: 7, IncludeDisputed: true})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	if ids := viewIDs(disputed); !equalIDs(ids, []int64{2}) {
		t.Fatalf("disputed ids = %v, want [2]", ids)
	}
	if disputed[0].View != service.FeedViewDisputed {
		t.Errorf("report 2: View = %q, want %q", disputed[0].View, service.FeedViewDisputed)
	}
}

// TestWrittenAtForDisplay pins D-19's strict 30 minute rule: a written time is
// shown only when the upload came MORE than WrittenLabelThreshold after it, and
// what comes back is a copy.
func TestWrittenAtForDisplay(t *testing.T) {
	created := time.Date(2031, 3, 1, 12, 0, 0, 0, time.UTC)
	before := func(d time.Duration) *time.Time {
		w := created.Add(-d)
		return &w
	}

	t.Run("nil written gives nil", func(t *testing.T) {
		if got := service.WrittenAtForDisplay(created, nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("29m59s before gives nil", func(t *testing.T) {
		if got := service.WrittenAtForDisplay(created, before(29*time.Minute+59*time.Second)); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("exactly the threshold gives nil", func(t *testing.T) {
		if got := service.WrittenAtForDisplay(created, before(service.WrittenLabelThreshold)); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("30m01s before gives the written time", func(t *testing.T) {
		w := before(30*time.Minute + time.Second)
		got := service.WrittenAtForDisplay(created, w)
		if got == nil || !got.Equal(*w) {
			t.Errorf("got %v, want %v", got, *w)
		}
	})
	t.Run("4 hours before gives the written time", func(t *testing.T) {
		w := before(4 * time.Hour)
		got := service.WrittenAtForDisplay(created, w)
		if got == nil || !got.Equal(*w) {
			t.Errorf("got %v, want %v", got, *w)
		}
	})
	t.Run("written after created gives nil", func(t *testing.T) {
		w := created.Add(time.Hour)
		if got := service.WrittenAtForDisplay(created, &w); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("the returned pointer is a copy", func(t *testing.T) {
		w := before(4 * time.Hour)
		orig := *w
		got := service.WrittenAtForDisplay(created, w)
		if got == nil {
			t.Fatal("got nil, want a copy")
		}
		if got == w {
			t.Fatal("returned the input pointer, want a copy")
		}
		*got = got.Add(24 * time.Hour)
		if !w.Equal(orig) {
			t.Errorf("input changed to %v, want %v", *w, orig)
		}
	})
}

// extrasRow builds one ReportExtrasForReportsRow for the moderation tests.
func extrasRow(id int64, state service.ModerationState, written *time.Time, flagged bool) sqlcgen.ReportExtrasForReportsRow {
	row := sqlcgen.ReportExtrasForReportsRow{ReportID: id, ModerationState: string(state), ViewerFlagged: flagged}
	if written != nil {
		row.ClientWrittenAt = pgtype.Timestamptz{Time: *written, Valid: true}
	}
	return row
}

// TestNearbyCarriesModerationExtras is R-01 and D-02 on the feed read: the
// stored moderation state reaches Resolve through the one extras read, a
// withheld report never lists, a pending serious one stays published, a report
// the extras read did not return is not listed, and the display only written
// time and the viewer's flag ride along.
func TestNearbyCarriesModerationExtras(t *testing.T) {
	query := service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, ViewerAccountID: 30}
	newFake := func() *fakeFeedQuerier {
		f := &fakeFeedQuerier{}
		f.nearbyRows = []sqlcgen.NearbyReportsRow{
			nearbyRow(1, service.SeverityLow, service.CategoryFlood, 1),
			nearbyRow(2, service.SeverityCritical, service.CategoryFlood, 2),
			nearbyRow(3, service.SeverityLow, service.CategoryFlood, 3),
			nearbyRow(4, service.SeverityCritical, service.CategoryFlood, 4),
			nearbyRow(5, service.SeverityLow, service.CategoryRescueNeeded, 5),
			nearbyRow(6, service.SeverityLow, service.CategoryFlood, 6),
			nearbyRow(7, service.SeverityLow, service.CategoryFlood, 7),
		}
		written4h := f.nearbyRows[5].CreatedAt.Add(-4 * time.Hour)
		notWritten := f.nearbyRows[3].CreatedAt.Add(-4 * time.Hour)
		report4 := extrasRow(4, service.ModerationPending, nil, false)
		// A NULL column whose Time is not zero: Valid false must win.
		report4.ClientWrittenAt = pgtype.Timestamptz{Time: notWritten, Valid: false}
		f.extrasRows = []sqlcgen.ReportExtrasForReportsRow{
			extrasRow(1, service.ModerationRemoved, nil, false),
			extrasRow(2, service.ModerationRemoved, nil, false),
			extrasRow(3, service.ModerationPending, nil, false),
			report4,
			extrasRow(5, service.ModerationPending, nil, false),
			extrasRow(6, service.ModerationClear, &written4h, true),
		}
		return f
	}

	t.Run("lists only the published reports and fills the extras", func(t *testing.T) {
		fake := newFake()
		views, err := service.NewReportService(fake).Nearby(context.Background(), query)
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if got := viewIDs(views); !equalIDs(got, []int64{4, 5, 6}) {
			t.Fatalf("ids = %v, want [4 5 6]", got)
		}
		for _, v := range views {
			if v.Visibility == service.VisibilityWithheld {
				t.Errorf("report %d carries visibility withheld", v.ID)
			}
		}
		v4, _ := findView(views, 4)
		if v4.Visibility != service.VisibilityLive || v4.VisibilityReason != service.ReasonCriticalBypass {
			t.Errorf("report 4 = %s/%s, want live/critical_bypass", v4.Visibility, v4.VisibilityReason)
		}
		if v4.WrittenAt != nil {
			t.Errorf("report 4 WrittenAt = %v, want nil for a NULL column", v4.WrittenAt)
		}
		if v4.ViewerFlagged {
			t.Error("report 4 ViewerFlagged = true, want false")
		}
		v6, _ := findView(views, 6)
		want6 := fake.nearbyRows[5].CreatedAt.Add(-4 * time.Hour)
		if v6.WrittenAt == nil || !v6.WrittenAt.Equal(want6) {
			t.Errorf("report 6 WrittenAt = %v, want %v", v6.WrittenAt, want6)
		}
		if !v6.ViewerFlagged {
			t.Error("report 6 ViewerFlagged = false, want true")
		}
		if len(fake.extrasArgs) != 1 || fake.extrasArgs[0].ViewerAccountID != 30 {
			t.Errorf("extrasArgs = %+v, want one call for viewer 30", fake.extrasArgs)
		}
	})

	t.Run("an extras error is returned", func(t *testing.T) {
		boom := errors.New("extras unavailable")
		fake := newFake()
		fake.extrasErr = boom
		if _, err := service.NewReportService(fake).Nearby(context.Background(), query); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
	})

	t.Run("an unconfigured fake lists every page report as before", func(t *testing.T) {
		fake := newFake()
		fake.extrasRows = nil
		views, err := service.NewReportService(fake).Nearby(context.Background(), query)
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		if got := viewIDs(views); !equalIDs(got, []int64{1, 2, 3, 4, 5, 6, 7}) {
			t.Errorf("ids = %v, want all seven", got)
		}
	})
}

// TestHistoryReadsCarryModeration is R-01 on the Phase 3.1 read paths: one
// extras read in the shared context load, and a held or removed report is in
// no view.
func TestHistoryReadsCarryModeration(t *testing.T) {
	newFake := func() *fakeFeedQuerier {
		return &fakeFeedQuerier{
			expiredRows: []sqlcgen.ExpiredReportsInWindowRow{
				h6Expired(1, service.CategoryFlood, service.SeverityLow, time.Hour),
				h6Expired(2, service.CategoryFlood, service.SeverityLow, time.Hour),
				h6Expired(3, service.CategoryFlood, service.SeverityLow, time.Hour),
			},
			extrasRows: []sqlcgen.ReportExtrasForReportsRow{
				extrasRow(1, service.ModerationRemoved, nil, false),
				extrasRow(2, service.ModerationPending, nil, false),
				extrasRow(3, service.ModerationClear, nil, false),
			},
		}
	}

	t.Run("History lists only the clear report with one extras read", func(t *testing.T) {
		fake := newFake()
		page, err := h6History(fake, h6Query())
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		if got := viewIDs(page.Reports); !equalIDs(got, []int64{3}) {
			t.Errorf("ids = %v, want [3]", got)
		}
		if n := countCalls(fake.callLog, "ReportExtrasForReports"); n != 1 {
			t.Errorf("ReportExtrasForReports called %d times, want 1", n)
		}
		if len(fake.extrasArgs) == 1 && !equalIDs(fake.extrasArgs[0].ReportIds, []int64{1, 2, 3}) {
			t.Errorf("extras ReportIds = %v, want the candidate ids [1 2 3]", fake.extrasArgs[0].ReportIds)
		}
	})

	t.Run("RecentHistoryCounts counts only the clear report", func(t *testing.T) {
		fake := newFake()
		svc := service.NewReportService(fake, service.WithReportClock(p12Clock))
		got, err := svc.RecentHistoryCounts(context.Background(), service.RecentQuery{Latitude: h6Centre.Lat, Longitude: h6Centre.Lon, RadiusKm: 10})
		if err != nil {
			t.Fatalf("RecentHistoryCounts: %v", err)
		}
		if got.Expired != 1 || got.Resolved != 0 {
			t.Errorf("counts = %+v, want Expired 1", got)
		}
		if n := countCalls(fake.callLog, "ReportExtrasForReports"); n != 1 {
			t.Errorf("ReportExtrasForReports called %d times, want 1", n)
		}
	})

	t.Run("Focus on the removed report is unavailable", func(t *testing.T) {
		fake := newFake()
		fake.focusRow = &sqlcgen.ReportForFocusRow{}
		*fake.focusRow = sqlcgen.ReportForFocusRow(fake.expiredRows[0])
		res, err := h6Focus(fake, service.FocusQuery{ReportID: 1, Latitude: h6Centre.Lat, Longitude: h6Centre.Lon})
		if err != nil {
			t.Fatalf("Focus: %v", err)
		}
		if res.Status != service.FocusUnavailable || res.View != nil {
			t.Errorf("result = %+v, want unavailable with no view", res)
		}
	})

	t.Run("Focus on the clear report is ok", func(t *testing.T) {
		fake := newFake()
		fake.focusRow = &sqlcgen.ReportForFocusRow{}
		*fake.focusRow = sqlcgen.ReportForFocusRow(fake.expiredRows[2])
		res, err := h6Focus(fake, service.FocusQuery{ReportID: 3, Latitude: h6Centre.Lat, Longitude: h6Centre.Lon})
		if err != nil {
			t.Fatalf("Focus: %v", err)
		}
		if res.Status != service.FocusOK || res.View == nil {
			t.Errorf("result = %+v, want ok with a view", res)
		}
	})
}
