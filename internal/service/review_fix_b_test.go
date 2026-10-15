package service_test

import (
	"context"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// The tests in this file belong to the 03.1 review fixes (fixer B): the cut flag
// when a candidate read hits its cap (WR-02, W8), the Nearby view guard (IN-01,
// W1), the ListableInFeed and ViewOf drift test (W3) and undos_left on a faded
// report (IN-02).

// TestHistoryCutWhenCandidateReadHitsItsCap pins WR-02 and W8. The candidate
// reads carry no vote predicate, so a read that returns exactly its row cap may
// be missing reports that belong in a group. The service says so through the cut
// flags even though far fewer than HistoryGroupCap entries were kept.
func TestHistoryCutWhenCandidateReadHitsItsCap(t *testing.T) {
	t.Run("an open read at its cap cuts the resolved group only", func(t *testing.T) {
		fake := &fakeFeedQuerier{}
		for i := int64(1); i <= int64(service.HistoryOpenCandidateCap); i++ {
			fake.openRows = append(fake.openRows, h6Open(i, service.CategoryFlood, service.SeverityLow, time.Hour))
			fake.reporterRows = append(fake.reporterRows, reporterRow(i, nil))
		}
		page, err := h6History(fake, h6Query())
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		if len(page.Reports) != 0 {
			t.Errorf("kept %d entries, want 0 (every open report is live)", len(page.Reports))
		}
		if !page.ResolvedCut || page.ExpiredCut {
			t.Errorf("ResolvedCut = %v, ExpiredCut = %v, want true and false", page.ResolvedCut, page.ExpiredCut)
		}
	})

	t.Run("an open read one short of its cap sets no cut", func(t *testing.T) {
		fake := &fakeFeedQuerier{}
		for i := int64(1); i < int64(service.HistoryOpenCandidateCap); i++ {
			fake.openRows = append(fake.openRows, h6Open(i, service.CategoryFlood, service.SeverityLow, time.Hour))
			fake.reporterRows = append(fake.reporterRows, reporterRow(i, nil))
		}
		page, err := h6History(fake, h6Query())
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		if page.ResolvedCut || page.ExpiredCut {
			t.Errorf("ResolvedCut = %v, ExpiredCut = %v, want false and false", page.ResolvedCut, page.ExpiredCut)
		}
	})

	t.Run("an expired read at its cap cuts both groups", func(t *testing.T) {
		fake := &fakeFeedQuerier{}
		for i := int64(1); i <= int64(service.HistoryExpiredCandidateCap); i++ {
			fake.expiredRows = append(fake.expiredRows, h6Expired(i, service.CategoryFlood, service.SeverityLow, time.Hour))
			fake.reporterRows = append(fake.reporterRows, reporterRow(i, nil))
		}
		page, err := h6History(fake, h6Query())
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		if !page.ResolvedCut || !page.ExpiredCut {
			t.Errorf("ResolvedCut = %v, ExpiredCut = %v, want true and true", page.ResolvedCut, page.ExpiredCut)
		}
		if len(page.Reports) != service.HistoryGroupCap {
			t.Errorf("kept %d entries, want %d", len(page.Reports), service.HistoryGroupCap)
		}
	})
}

// TestRecentHistoryCountsKeepsItsShapeWhenCapped: a capped read changes nothing
// on the counts, which have no wire field for it. The call still returns them.
func TestRecentHistoryCountsKeepsItsShapeWhenCapped(t *testing.T) {
	fake := &fakeFeedQuerier{}
	for i := int64(1); i <= int64(service.HistoryOpenCandidateCap); i++ {
		fake.openRows = append(fake.openRows, h6Open(i, service.CategoryFlood, service.SeverityLow, time.Hour))
		fake.reporterRows = append(fake.reporterRows, reporterRow(i, nil))
	}
	svc := service.NewReportService(fake, service.WithReportClock(p12Clock))
	got, err := svc.RecentHistoryCounts(context.Background(), service.RecentQuery{Latitude: h6Centre.Lat, Longitude: h6Centre.Lon, RadiusKm: 10})
	if err != nil {
		t.Fatalf("RecentHistoryCounts: %v", err)
	}
	if got != (service.RecentCounts{}) {
		t.Errorf("counts = %+v, want zeros", got)
	}
}

// TestNearbySkipsAReportThatExpiredInTheClockGap pins IN-01 and W1. The query
// selects with the database clock and the service judges with its own, so a
// report can come back from the query already expired by the service clock. It
// must not be in the answer: stamped as finished it would carry no history and,
// for Rescue, its exact point and text.
func TestNearbySkipsAReportThatExpiredInTheClockGap(t *testing.T) {
	live := p12Row(1, service.SeverityLow, service.CategoryFlood, 1.0)
	gapFlood := p12Row(2, service.SeverityLow, service.CategoryFlood, 2.0)
	gapFlood.ExpiresAt = p12Now.Add(-time.Millisecond)
	gapRescue := p12Row(3, service.SeverityCritical, service.CategoryRescueNeeded, 3.0)
	gapRescue.ExpiresAt = p12Now

	fake := &fakeFeedQuerier{
		nearbyRows: []sqlcgen.NearbyReportsRow{live, gapFlood, gapRescue},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{
			reporterRow(1, nil), reporterRow(2, nil), reporterRow(3, nil),
		},
	}
	svc := service.NewReportService(fake, service.WithReportClock(p12Clock))
	for _, includeDisputed := range []bool{false, true} {
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{
			Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, IncludeDisputed: includeDisputed,
		})
		if err != nil {
			t.Fatalf("Nearby(includeDisputed=%v): %v", includeDisputed, err)
		}
		for _, v := range views {
			if v.View != service.FeedViewActive && v.View != service.FeedViewDisputed {
				t.Errorf("includeDisputed=%v: report %d came back with view %q", includeDisputed, v.ID, v.View)
			}
			if v.ID != 1 {
				t.Errorf("includeDisputed=%v: report %d is in the answer, want only the live report 1", includeDisputed, v.ID)
			}
		}
		if !includeDisputed && len(views) != 1 {
			t.Errorf("active read returned %d views, want 1", len(views))
		}
	}
}
