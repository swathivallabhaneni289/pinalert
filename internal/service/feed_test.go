package service_test

import (
	"context"
	"testing"
	"time"

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

	lastVoteIDs     []int64
	lastReporterIDs []int64
	callLog         []string
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
		{"Live is listable when disputed reports are included", service.VisibilityLive, true, true},
		{"Provisional is listable when disputed reports are excluded", service.VisibilityProvisional, false, true},
		{"Provisional is listable when disputed reports are included", service.VisibilityProvisional, true, true},
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

	t.Run("a report the reporter has resolved is absent from the default and the disputed views", func(t *testing.T) {
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
			if !containsReportID(views, 2) || !containsReportID(views, 3) {
				t.Errorf("includeDisputed=%v: expected reports 2 and 3 present, got %v", includeDisputed, viewIDs(views))
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

	t.Run("an unvoted report stays provisional in both views (D-08, D-09) — the toggle adds Hidden reports, never removes Provisional ones", func(t *testing.T) {
		for _, includeDisputed := range []bool{false, true} {
			views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: includeDisputed})
			if err != nil {
				t.Fatalf("Nearby: %v", err)
			}
			v, ok := findView(views, 2)
			if !ok {
				t.Fatalf("includeDisputed=%v: report 2 absent, want present", includeDisputed)
			}
			if v.Visibility != service.VisibilityProvisional || v.VisibilityReason != service.ReasonAwaitingConfirmation {
				t.Errorf("includeDisputed=%v: got visibility=%q reason=%q, want %q/%q", includeDisputed, v.Visibility, v.VisibilityReason, service.VisibilityProvisional, service.ReasonAwaitingConfirmation)
			}
		}
	})

	t.Run("a confirmed report stays live in both views", func(t *testing.T) {
		for _, includeDisputed := range []bool{false, true} {
			views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: includeDisputed})
			if err != nil {
				t.Fatalf("Nearby: %v", err)
			}
			v, ok := findView(views, 3)
			if !ok {
				t.Fatalf("includeDisputed=%v: report 3 absent, want present", includeDisputed)
			}
			if v.Visibility != service.VisibilityLive || v.VisibilityReason != service.ReasonConfirmed {
				t.Errorf("includeDisputed=%v: got visibility=%q reason=%q, want %q/%q", includeDisputed, v.Visibility, v.VisibilityReason, service.VisibilityLive, service.ReasonConfirmed)
			}
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

		if _, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5}); err != nil {
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

	t.Run("the show_disputed view also stays ascending by distance", func(t *testing.T) {
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		wantIDs := []int64{1, 2, 4, 5}
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
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true, ViewerAccountID: 7})
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
			views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true, ViewerAccountID: viewer})
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
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true, ViewerAccountID: 7})
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
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: true, ViewerAccountID: 0})
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
