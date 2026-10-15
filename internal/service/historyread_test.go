package service_test

import (
	"context"
	"math"
	"slices"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// The h6 fixtures belong to plan 03.1-06. Every time hangs off p12Now, so only
// the injected clock decides what is expired and what is inside a window.

// h6Centre is the reader's query centre.
var h6Centre = service.GeoPoint{Lat: 12.98, Lon: 77.60}

// h6Expired is an expired candidate that expired expiredAgo before p12Now.
func h6Expired(id int64, category service.Category, severity service.Severity, expiredAgo time.Duration) sqlcgen.ExpiredReportsInWindowRow {
	return sqlcgen.ExpiredReportsInWindowRow{
		ID:          id,
		Category:    string(category),
		Severity:    string(severity),
		Description: "a fake history report for historyread_test.go",
		Latitude:    12.9716,
		Longitude:   77.5946,
		Geohash:     "tdr1qgzp",
		CreatedAt:   p12Now.Add(-expiredAgo - 8*time.Hour),
		ExpiresAt:   p12Now.Add(-expiredAgo),
		DistanceKm:  float64(id) / 10,
	}
}

// h6Open is an unexpired candidate that expires expiresIn after p12Now.
func h6Open(id int64, category service.Category, severity service.Severity, expiresIn time.Duration) sqlcgen.OpenReportsInAreaRow {
	return sqlcgen.OpenReportsInAreaRow{
		ID:          id,
		Category:    string(category),
		Severity:    string(severity),
		Description: "a fake open report for historyread_test.go",
		Latitude:    12.9716,
		Longitude:   77.5946,
		Geohash:     "tdr1qgzp",
		CreatedAt:   p12Now.Add(expiresIn - 8*time.Hour),
		ExpiresAt:   p12Now.Add(expiresIn),
		DistanceKm:  float64(id) / 10,
	}
}

// h6Closed is a standing resolve vote written at the given time. When the
// account is the report's poster it closes the report at once; otherwise two
// of them in distinct cells do.
func h6Closed(reportID, accountID int64, at time.Time, cell string) sqlcgen.CurrentVotesForReportsRow {
	return sqlcgen.CurrentVotesForReportsRow{
		ReportID:    reportID,
		AccountID:   accountID,
		Kind:        string(service.VoteKindResolution),
		Value:       string(service.VoteResolve),
		GeohashCell: cell,
		CreatedAt:   at,
	}
}

func h6History(fake *fakeFeedQuerier, q service.HistoryQuery) (service.HistoryPage, error) {
	svc := service.NewReportService(fake, service.WithReportClock(p12Clock))
	return svc.History(context.Background(), q)
}

func h6Query() service.HistoryQuery {
	return service.HistoryQuery{Latitude: h6Centre.Lat, Longitude: h6Centre.Lon, RadiusKm: 10}
}

// h6Haversine is an independent distance, so the test does not lean on the
// function under test.
func h6Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

func TestHistoryGroupsAndOrder(t *testing.T) {
	fake := &fakeFeedQuerier{
		openRows: []sqlcgen.OpenReportsInAreaRow{
			h6Open(1, service.CategoryFlood, service.SeverityLow, 5*time.Hour),
			h6Open(4, service.CategoryFlood, service.SeverityLow, 4*time.Hour),
		},
		expiredRows: []sqlcgen.ExpiredReportsInWindowRow{
			h6Expired(5, service.CategoryFlood, service.SeverityLow, time.Hour),
			h6Expired(2, service.CategoryFlood, service.SeverityLow, 2*time.Hour),
			h6Expired(3, service.CategoryFlood, service.SeverityLow, 5*time.Hour),
		},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{
			reporterRow(1, ptrID(11)), reporterRow(2, ptrID(12)), reporterRow(3, nil), reporterRow(4, nil), reporterRow(5, nil),
		},
	}
	fake.voteRows = append(fake.voteRows,
		h6Closed(1, 11, p12Now.Add(-3*time.Hour), "c1"),
		h6Closed(2, 12, p12Now.Add(-10*time.Hour), "c2"),
	)
	fake.voteRows = append(fake.voteRows, p12Places(3, service.VoteDispute, service.IndependentAgreementThreshold, 41)...)
	fake.voteRows = append(fake.voteRows, p12Places(5, service.VoteConfirm, service.IndependentAgreementThreshold, 51)...)

	q := h6Query()
	q.ViewerAccountID = 11
	page, err := h6History(fake, q)
	if err != nil {
		t.Fatalf("History: %v", err)
	}

	if ids := viewIDs(page.Reports); !equalIDs(ids, []int64{1, 2, 5, 3}) {
		t.Fatalf("ids = %v, want resolved [1 2] then expired [5 3]", ids)
	}
	if containsReportID(page.Reports, 4) {
		t.Errorf("a live unexpired report appeared in the Past read")
	}
	want := map[int64]service.FeedView{1: service.FeedViewResolved, 2: service.FeedViewResolved, 5: service.FeedViewPast, 3: service.FeedViewPast}
	for _, v := range page.Reports {
		if v.View != want[v.ID] {
			t.Errorf("report %d: View = %q, want %q", v.ID, v.View, want[v.ID])
		}
		if v.History == nil {
			t.Errorf("report %d: History is nil", v.ID)
		}
	}
	v1, _ := findView(page.Reports, 1)
	if !v1.IsOwnReport || v1.Visibility != service.VisibilityRetracted {
		t.Errorf("report 1: IsOwnReport = %v, Visibility = %q, want the viewer's own Retracted report", v1.IsOwnReport, v1.Visibility)
	}
	if page.ResolvedCut || page.ExpiredCut {
		t.Errorf("cut flags = %v %v, want false false", page.ResolvedCut, page.ExpiredCut)
	}

	if len(fake.expiredArgs) != 1 || len(fake.openArgs) != 1 {
		t.Fatalf("candidate reads = %d and %d, want 1 and 1", len(fake.expiredArgs), len(fake.openArgs))
	}
	ea, oa := fake.expiredArgs[0], fake.openArgs[0]
	if !ea.Now.Equal(p12Now) || !oa.Now.Equal(p12Now) {
		t.Errorf("Now = %v and %v, want the injected clock %v for both reads", ea.Now, oa.Now, p12Now)
	}
	if !ea.WindowStart.Equal(p12Now.Add(-service.HistoryWindow)) {
		t.Errorf("WindowStart = %v, want now minus HistoryWindow", ea.WindowStart)
	}
	if int(ea.RowCap) != service.HistoryExpiredCandidateCap || int(oa.RowCap) != service.HistoryOpenCandidateCap {
		t.Errorf("row caps = %d and %d, want %d and %d", ea.RowCap, oa.RowCap, service.HistoryExpiredCandidateCap, service.HistoryOpenCandidateCap)
	}
}

func TestHistoryResolvedOutsideWindowDropped(t *testing.T) {
	fake := &fakeFeedQuerier{
		expiredRows: []sqlcgen.ExpiredReportsInWindowRow{
			h6Expired(1, service.CategoryFlood, service.SeverityLow, 6*24*time.Hour),
			h6Expired(2, service.CategoryFlood, service.SeverityLow, 5*24*time.Hour),
		},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(1, ptrID(11)), reporterRow(2, ptrID(12))},
	}
	fake.voteRows = append(fake.voteRows,
		h6Closed(1, 11, p12Now.Add(-8*24*time.Hour), "c1"),
		h6Closed(2, 12, p12Now.Add(-6*24*time.Hour-time.Hour), "c2"),
	)
	page, err := h6History(fake, h6Query())
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if ids := viewIDs(page.Reports); !equalIDs(ids, []int64{2}) {
		t.Fatalf("ids = %v, want only [2]: report 1 closed 8 days ago is outside the window", ids)
	}
}

func TestHistoryCapAndCut(t *testing.T) {
	fake := &fakeFeedQuerier{}
	for i := int64(1); i <= 30; i++ {
		fake.expiredRows = append(fake.expiredRows, h6Expired(i, service.CategoryFlood, service.SeverityLow, time.Hour))
		fake.reporterRows = append(fake.reporterRows, reporterRow(i, ptrID(1000+i)))
		fake.voteRows = append(fake.voteRows, h6Closed(i, 1000+i, p12Now.Add(-2*time.Hour-time.Duration(i)*time.Minute), "c1"))
	}
	for i := int64(101); i <= 110; i++ {
		fake.expiredRows = append(fake.expiredRows, h6Expired(i, service.CategoryFlood, service.SeverityLow, time.Duration(i)*time.Minute))
		fake.reporterRows = append(fake.reporterRows, reporterRow(i, nil))
	}
	page, err := h6History(fake, h6Query())
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	var resolved, expired []int64
	for _, v := range page.Reports {
		switch v.View {
		case service.FeedViewResolved:
			resolved = append(resolved, v.ID)
		case service.FeedViewPast:
			expired = append(expired, v.ID)
		}
	}
	if len(resolved) != service.HistoryGroupCap || !page.ResolvedCut {
		t.Errorf("resolved = %d entries, cut = %v, want %d and true", len(resolved), page.ResolvedCut, service.HistoryGroupCap)
	}
	if resolved[0] != 1 || resolved[len(resolved)-1] != 25 {
		t.Errorf("resolved kept ids %d to %d, want the newest closures 1 to 25", resolved[0], resolved[len(resolved)-1])
	}
	if len(expired) != 10 || page.ExpiredCut {
		t.Errorf("expired = %d entries, cut = %v, want 10 and false", len(expired), page.ExpiredCut)
	}
}

func TestHistoryCategoryFilterBeforeCap(t *testing.T) {
	fake := &fakeFeedQuerier{}
	for i := int64(1); i <= 30; i++ {
		category := service.CategoryShelterOpen
		if i > 25 {
			category = service.CategoryFlood
		}
		fake.expiredRows = append(fake.expiredRows, h6Expired(i, category, service.SeverityLow, time.Hour))
		fake.reporterRows = append(fake.reporterRows, reporterRow(i, ptrID(1000+i)))
		fake.voteRows = append(fake.voteRows, h6Closed(i, 1000+i, p12Now.Add(-2*time.Hour-time.Duration(i)*time.Minute), "c1"))
	}
	q := h6Query()
	q.Category = service.CategoryFlood
	page, err := h6History(fake, q)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(page.Reports) != 5 || page.ResolvedCut {
		t.Fatalf("reports = %d, cut = %v, want the 5 flood reports and no cut (the flood ones are the oldest, so a cap before the filter would lose them)", len(page.Reports), page.ResolvedCut)
	}
	for _, v := range page.Reports {
		if v.Category != service.CategoryFlood {
			t.Errorf("report %d has category %q, want flood", v.ID, v.Category)
		}
	}
}

func TestHistoryOutcomeAndClosedBy(t *testing.T) {
	newFake := func() *fakeFeedQuerier {
		fake := &fakeFeedQuerier{
			expiredRows: []sqlcgen.ExpiredReportsInWindowRow{
				h6Expired(1, service.CategoryFlood, service.SeverityLow, 2*time.Hour),
				h6Expired(2, service.CategoryFlood, service.SeverityLow, 2*time.Hour),
				h6Expired(3, service.CategoryRoadBlocked, service.SeverityCritical, time.Hour),
				h6Expired(4, service.CategoryPowerOutage, service.SeverityCritical, time.Hour),
			},
			reporterRows: []sqlcgen.ReporterAccountsForReportsRow{
				reporterRow(1, ptrID(11)), reporterRow(2, ptrID(12)), reporterRow(3, nil), reporterRow(4, nil),
			},
		}
		fake.voteRows = append(fake.voteRows, h6Closed(1, 11, p12Now.Add(-4*time.Hour), "c1"))
		fake.voteRows = append(fake.voteRows,
			h6Closed(2, 31, p12Now.Add(-5*time.Hour), "cellA"),
			h6Closed(2, 32, p12Now.Add(-4*time.Hour), "cellB"),
		)
		fake.voteRows = append(fake.voteRows, p12Places(3, service.VoteDispute, service.IndependentAgreementThreshold, 41)...)
		return fake
	}
	check := func(t *testing.T, viewer int64, id int64, by service.ClosedBy, text string) {
		t.Helper()
		q := h6Query()
		q.ViewerAccountID = viewer
		page, err := h6History(newFake(), q)
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		v, ok := findView(page.Reports, id)
		if !ok || v.History == nil {
			t.Fatalf("report %d missing or without history", id)
		}
		if v.History.State != service.HistoryStateResolved || v.History.ClosedBy != by || v.History.ClosedText != text {
			t.Errorf("report %d for viewer %d: history = %+v, want resolved, ClosedBy %q, text %q", id, viewer, *v.History, by, text)
		}
	}
	t.Run("the poster sees you and everyone else sees the poster", func(t *testing.T) {
		check(t, 11, 1, service.ClosedByYou, "Resolved by you")
		check(t, 99, 1, service.ClosedByPoster, "Resolved by the poster")
	})
	t.Run("independent closure reads people nearby", func(t *testing.T) {
		check(t, 99, 2, service.ClosedByPeople, "Resolved by people nearby")
	})

	page, err := h6History(newFake(), h6Query())
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	t.Run("a critical report that was out disputed reads Disputed", func(t *testing.T) {
		v, ok := findView(page.Reports, 3)
		if !ok || v.History == nil {
			t.Fatalf("report 3 missing or without history")
		}
		if v.History.State != service.HistoryStateExpired || v.History.Outcome != service.OutcomeContradicted || v.History.OutcomeLabel != "Disputed" {
			t.Errorf("history = %+v, want expired, contradicted, Disputed", *v.History)
		}
	})
	t.Run("a neutral expiry reads Expired", func(t *testing.T) {
		v, ok := findView(page.Reports, 4)
		if !ok || v.History == nil {
			t.Fatalf("report 4 missing or without history")
		}
		if v.History.Outcome != service.OutcomeNeutral || v.History.OutcomeLabel != "Expired" {
			t.Errorf("history = %+v, want neutral and Expired", *v.History)
		}
	})
}

// h6RescueFake holds one finished rescue report, closed by its poster, and one
// finished flood report, on the open and the expired candidate sets.
func h6RescueFake() *fakeFeedQuerier {
	fake := &fakeFeedQuerier{
		expiredRows: []sqlcgen.ExpiredReportsInWindowRow{
			h6Expired(1, service.CategoryRescueNeeded, service.SeverityCritical, time.Hour),
			h6Expired(2, service.CategoryFlood, service.SeverityLow, time.Hour),
		},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(1, ptrID(11)), reporterRow(2, ptrID(12))},
	}
	fake.voteRows = append(fake.voteRows,
		h6Closed(1, 11, p12Now.Add(-3*time.Hour), "c1"),
		h6Closed(2, 12, p12Now.Add(-3*time.Hour), "c2"),
	)
	return fake
}

// h6AssertCoarse asserts all four location fields of a finished rescue view.
func h6AssertCoarse(t *testing.T, v service.ReportView) {
	t.Helper()
	cLat, cLon, cell := service.CoarsePoint(12.9716, 77.5946)
	if !v.Approximate {
		t.Errorf("report %d: Approximate = false, want true", v.ID)
	}
	if v.Description != "" {
		t.Errorf("report %d: Description = %q, want empty", v.ID, v.Description)
	}
	if len(v.Geohash) != service.ApproxGeohashPrecision || v.Geohash != cell {
		t.Errorf("report %d: Geohash = %q, want the precision 6 cell %q", v.ID, v.Geohash, cell)
	}
	if v.Latitude != cLat || v.Longitude != cLon {
		t.Errorf("report %d: point = %v,%v, want the cell centre %v,%v", v.ID, v.Latitude, v.Longitude, cLat, cLon)
	}
	if v.DistanceKm == nil {
		t.Fatalf("report %d: DistanceKm is nil, want the distance to the coarse point", v.ID)
	}
	want := h6Haversine(h6Centre.Lat, h6Centre.Lon, cLat, cLon)
	if math.Abs(*v.DistanceKm-want) > 1e-6 {
		t.Errorf("report %d: DistanceKm = %v, want %v from the query centre to the coarse point", v.ID, *v.DistanceKm, want)
	}
}

// h6AssertExact asserts a finished flood view is left as it was.
func h6AssertExact(t *testing.T, v service.ReportView) {
	t.Helper()
	if v.Approximate || v.Description == "" || v.Geohash != "tdr1qgzp" || v.Latitude != 12.9716 || v.Longitude != 77.5946 {
		t.Errorf("report %d: %+v, want an exact flood report", v.ID, v.Report)
	}
}

func TestFinishedRescueCoarsenedOnEveryPath(t *testing.T) {
	t.Run("History", func(t *testing.T) {
		page, err := h6History(h6RescueFake(), h6Query())
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		rescue, ok1 := findView(page.Reports, 1)
		flood, ok2 := findView(page.Reports, 2)
		if !ok1 || !ok2 {
			t.Fatalf("ids = %v, want 1 and 2", viewIDs(page.Reports))
		}
		h6AssertCoarse(t, rescue)
		h6AssertExact(t, flood)
	})
}

func TestHistoryEmptyCostsNoBatchedReads(t *testing.T) {
	fake := &fakeFeedQuerier{}
	page, err := h6History(fake, h6Query())
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if page.Reports == nil || len(page.Reports) != 0 || page.ResolvedCut || page.ExpiredCut {
		t.Errorf("page = %+v, want an empty page with no cut", page)
	}
	want := []string{"ExpiredReportsInWindow", "OpenReportsInArea"}
	if !slices.Equal(fake.callLog, want) {
		t.Errorf("callLog = %v, want only the two candidate reads %v", fake.callLog, want)
	}
}

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
