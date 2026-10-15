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
	t.Run("an open read past its cap cuts the resolved group only", func(t *testing.T) {
		fake := &fakeFeedQuerier{}
		for i := int64(1); i <= int64(service.HistoryOpenCandidateCap)+1; i++ {
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
		if !page.ResolvedPartial || page.ExpiredPartial || page.ResolvedCut || page.ExpiredCut {
			t.Errorf("partial = %v and %v, cut = %v and %v, want partial true and false, no cut", page.ResolvedPartial, page.ExpiredPartial, page.ResolvedCut, page.ExpiredCut)
		}
	})

	t.Run("an open read of exactly its cap sets no cut", func(t *testing.T) {
		fake := &fakeFeedQuerier{}
		for i := int64(1); i <= int64(service.HistoryOpenCandidateCap); i++ {
			fake.openRows = append(fake.openRows, h6Open(i, service.CategoryFlood, service.SeverityLow, time.Hour))
			fake.reporterRows = append(fake.reporterRows, reporterRow(i, nil))
		}
		page, err := h6History(fake, h6Query())
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		if page.ResolvedPartial || page.ExpiredPartial || page.ResolvedCut || page.ExpiredCut {
			t.Errorf("partial = %v and %v, cut = %v and %v, want none", page.ResolvedPartial, page.ExpiredPartial, page.ResolvedCut, page.ExpiredCut)
		}
	})

	t.Run("an expired read past its cap cuts both groups", func(t *testing.T) {
		fake := &fakeFeedQuerier{}
		for i := int64(1); i <= int64(service.HistoryExpiredCandidateCap)+1; i++ {
			fake.expiredRows = append(fake.expiredRows, h6Expired(i, service.CategoryFlood, service.SeverityLow, time.Hour))
			fake.reporterRows = append(fake.reporterRows, reporterRow(i, nil))
		}
		page, err := h6History(fake, h6Query())
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		if !page.ResolvedPartial || !page.ExpiredPartial || page.ResolvedCut || !page.ExpiredCut {
			t.Errorf("partial = %v and %v, cut = %v and %v, want partial true and true, cut false and true (500 past entries, no resolved one)", page.ResolvedPartial, page.ExpiredPartial, page.ResolvedCut, page.ExpiredCut)
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
	for i := int64(1); i <= int64(service.HistoryOpenCandidateCap)+1; i++ {
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

// TestListableInFeedAgreesWithViewOf pins W3: ListableInFeed (membership of the
// Active and Disputed reads) and ViewOf (the one partition into tabs) are two
// hand written predicates, and this loops every visibility and both expiry sides
// so they cannot drift apart unnoticed.
func TestListableInFeedAgreesWithViewOf(t *testing.T) {
	now := p12Now
	sides := map[string]time.Time{
		"unexpired":           now.Add(time.Hour),
		"expired":             now.Add(-time.Hour),
		"expires exactly now": now,
	}
	visibilities := append(append([]service.Visibility{}, service.Visibilities...), service.Visibility("withheld"), service.Visibility(""))

	for _, vis := range visibilities {
		for side, expiresAt := range sides {
			view := service.ViewOf(vis, expiresAt, now)
			active := view == service.FeedViewActive
			disputed := view == service.FeedViewDisputed
			unexpired := expiresAt.After(now)

			// The two reads are disjoint and together are exactly the active and
			// disputed views of ViewOf.
			if got := vis.ListableInFeed(false) && unexpired; got != active {
				t.Errorf("%q %s: ListableInFeed(false) on an unexpired report = %v, ViewOf active = %v", vis, side, got, active)
			}
			if got := vis.ListableInFeed(true) && unexpired; got != disputed {
				t.Errorf("%q %s: ListableInFeed(true) on an unexpired report = %v, ViewOf disputed = %v", vis, side, got, disputed)
			}
			if vis.ListableInFeed(false) && vis.ListableInFeed(true) {
				t.Errorf("%q: listed in both the active and the disputed read", vis)
			}
			if !unexpired && (active || disputed) {
				t.Errorf("%q %s: an expired report has view %q, want it in no live tab", vis, side, view)
			}

			switch vis {
			case service.VisibilityRetracted:
				if vis.ListableInFeed(false) || vis.ListableInFeed(true) {
					t.Errorf("retracted %s is listable in a live read", side)
				}
				if view != service.FeedViewResolved {
					t.Errorf("retracted %s: view = %q, want resolved at any expiry", side, view)
				}
			case service.VisibilityLive, service.VisibilityProvisional, service.VisibilityHidden:
				// Covered by the agreement checks above.
			default:
				if vis.ListableInFeed(false) || vis.ListableInFeed(true) {
					t.Errorf("unknown visibility %q %s is listable", vis, side)
				}
				if view != service.FeedViewNone {
					t.Errorf("unknown visibility %q %s: view = %q, want none", vis, side, view)
				}
			}
		}
	}
}

// TestUndosLeftIsZeroOnAFadedReport pins IN-02: a report that has faded, a Past
// one or a resolved one past its expiry, reports 0 undos left whatever the
// budget was, and a live report still reports the full limit.
func TestUndosLeftIsZeroOnAFadedReport(t *testing.T) {
	fake := &fakeFeedQuerier{
		expiredRows: []sqlcgen.ExpiredReportsInWindowRow{
			h6Expired(1, service.CategoryFlood, service.SeverityLow, time.Hour),
			h6Expired(2, service.CategoryFlood, service.SeverityLow, time.Hour),
		},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(1, nil), reporterRow(2, ptrID(1002))},
		voteRows:     []sqlcgen.CurrentVotesForReportsRow{h6Closed(2, 1002, p12Now.Add(-2*time.Hour), "c1")},
	}
	q := h6Query()
	q.ViewerAccountID = 7
	page, err := h6History(fake, q)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	seen := map[service.FeedView]bool{}
	for _, v := range page.Reports {
		seen[v.View] = true
		if v.UndosLeft != 0 {
			t.Errorf("report %d (%s): UndosLeft = %d, want 0", v.ID, v.View, v.UndosLeft)
		}
	}
	if !seen[service.FeedViewPast] || !seen[service.FeedViewResolved] {
		t.Fatalf("views seen = %v, want a past one and a resolved one past its expiry", seen)
	}
	if got := countCalls(fake.callLog, "ResolutionUndoCounts"); got != 0 {
		t.Errorf("ResolutionUndoCounts called %d times, want 0 for faded reports", got)
	}

	t.Run("a report link to a past report says 0 too", func(t *testing.T) {
		focus := &fakeFeedQuerier{
			focusRow:     h6FocusRow(9, service.CategoryFlood, service.SeverityLow, -time.Hour, 1),
			reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(9, nil)},
		}
		res, err := h6Focus(focus, service.FocusQuery{ReportID: 9, Latitude: h6Centre.Lat, Longitude: h6Centre.Lon, ViewerAccountID: 7})
		if err != nil {
			t.Fatalf("Focus: %v", err)
		}
		if res.Status != service.FocusOK || res.View == nil || res.View.View != service.FeedViewPast {
			t.Fatalf("result = %+v, want ok with a past view", res)
		}
		if res.View.UndosLeft != 0 {
			t.Errorf("UndosLeft = %d, want 0", res.View.UndosLeft)
		}
	})
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
