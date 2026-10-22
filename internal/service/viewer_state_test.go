package service_test

import (
	"context"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// mutedExtras is a clear extras row that also says the viewer muted the report.
func mutedExtras(id int64, muted bool) sqlcgen.ReportExtrasForReportsRow {
	row := extrasRow(id, service.ModerationClear, nil, false)
	row.ViewerMuted = muted
	return row
}

// TestNearbyCarriesViewerFollowAndMute: the reader's own follow and mute state
// rides on every Nearby report, from rows the read already holds and from the
// one batched extras read (D-09, D-10, D-11).
func TestNearbyCarriesViewerFollowAndMute(t *testing.T) {
	fake := &fakeFeedQuerier{
		nearbyRows: []sqlcgen.NearbyReportsRow{
			nearbyRow(1, service.SeverityLow, service.CategoryFlood, 1),
			nearbyRow(2, service.SeverityLow, service.CategoryFlood, 2),
			nearbyRow(3, service.SeverityLow, service.CategoryFlood, 3),
			nearbyRow(4, service.SeverityLow, service.CategoryFlood, 4),
		},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{
			reporterRow(1, ptrID(7)),
			reporterRow(2, ptrID(9)),
			reporterRow(3, ptrID(9)),
			reporterRow(4, ptrID(9)),
		},
		voteRows: []sqlcgen.CurrentVotesForReportsRow{
			voteRow(2, 7, service.VoteKindContent, service.VoteConfirm, "cellX"),
			voteRow(3, 8, service.VoteKindContent, service.VoteConfirm, "cellY"),
		},
		extrasRows: []sqlcgen.ReportExtrasForReportsRow{
			mutedExtras(1, false),
			mutedExtras(2, true),
			mutedExtras(3, false),
			mutedExtras(4, true),
		},
	}
	svc := service.NewReportService(fake)
	read := func(viewer int64) map[int64]service.ReportView {
		t.Helper()
		views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, ViewerAccountID: viewer})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		out := map[int64]service.ReportView{}
		for _, v := range views {
			out[v.ID] = v
		}
		return out
	}

	got := read(7)
	want := map[int64][2]bool{
		1: {true, false},  // posted by 7
		2: {true, true},   // confirmed by 7, muted
		3: {false, false}, // another account's vote only
		4: {false, true},  // muted without a vote or a post: still reads muted
	}
	for id, w := range want {
		v, ok := got[id]
		if !ok {
			t.Fatalf("report %d missing from %v", id, got)
		}
		if v.ViewerFollows != w[0] || v.ViewerMuted != w[1] {
			t.Errorf("report %d for viewer 7: follows %v muted %v, want follows %v muted %v", id, v.ViewerFollows, v.ViewerMuted, w[0], w[1])
		}
	}

	// Another reader, whose extras read says nothing is muted, sees what is
	// theirs: report 3 is theirs by vote, nothing of 7's leaks across.
	fake.extrasRows = []sqlcgen.ReportExtrasForReportsRow{mutedExtras(1, false), mutedExtras(2, false), mutedExtras(3, false), mutedExtras(4, false)}
	other := read(8)
	for id, w := range map[int64][2]bool{1: {false, false}, 2: {false, false}, 3: {true, false}, 4: {false, false}} {
		v := other[id]
		if v.ViewerFollows != w[0] || v.ViewerMuted != w[1] {
			t.Errorf("report %d for viewer 8: follows %v muted %v, want follows %v muted %v", id, v.ViewerFollows, v.ViewerMuted, w[0], w[1])
		}
	}

	// No identified reader follows and mutes nothing.
	for id, v := range read(0) {
		if v.ViewerFollows || v.ViewerMuted {
			t.Errorf("report %d for viewer 0: follows %v muted %v, want both false", id, v.ViewerFollows, v.ViewerMuted)
		}
	}

	if n := countCalls(fake.callLog, "ReportExtrasForReports"); n != 3 {
		t.Errorf("ReportExtrasForReports called %d times for 3 reads, want 3 (one per read)", n)
	}
}

// TestFocusCarriesViewerMute: the report link read passes the viewer to the one
// extras read and carries both fields on an active report (D-11).
func TestFocusCarriesViewerMute(t *testing.T) {
	newFake := func(muted bool) *fakeFeedQuerier {
		return &fakeFeedQuerier{
			focusRow:     h6FocusRow(7, service.CategoryFlood, service.SeverityLow, time.Hour, 1),
			reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(7, ptrID(9))},
			voteRows:     []sqlcgen.CurrentVotesForReportsRow{voteRow(7, 7, service.VoteKindContent, service.VoteConfirm, "cellX")},
			extrasRows:   []sqlcgen.ReportExtrasForReportsRow{mutedExtras(7, muted)},
		}
	}
	focus := func(fake *fakeFeedQuerier, viewer int64) service.ReportView {
		t.Helper()
		res, err := h6Focus(fake, service.FocusQuery{ReportID: 7, Latitude: h6Centre.Lat, Longitude: h6Centre.Lon, ViewerAccountID: viewer})
		if err != nil {
			t.Fatalf("Focus: %v", err)
		}
		if res.Status != service.FocusOK || res.View == nil {
			t.Fatalf("result = %+v, want ok with a view", res)
		}
		return *res.View
	}

	t.Run("a voter who muted reads follows and muted", func(t *testing.T) {
		fake := newFake(true)
		v := focus(fake, 7)
		if !v.ViewerMuted || !v.ViewerFollows {
			t.Errorf("follows %v muted %v, want both true", v.ViewerFollows, v.ViewerMuted)
		}
		if len(fake.extrasArgs) != 1 || fake.extrasArgs[0].ViewerAccountID != 7 {
			t.Errorf("extrasArgs = %+v, want one call for viewer 7", fake.extrasArgs)
		}
	})

	t.Run("a voter who did not mute reads follows only", func(t *testing.T) {
		v := focus(newFake(false), 7)
		if !v.ViewerFollows || v.ViewerMuted {
			t.Errorf("follows %v muted %v, want follows true muted false", v.ViewerFollows, v.ViewerMuted)
		}
	})

	t.Run("another reader reads both false", func(t *testing.T) {
		fake := newFake(false)
		v := focus(fake, 8)
		if v.ViewerFollows || v.ViewerMuted {
			t.Errorf("follows %v muted %v, want both false", v.ViewerFollows, v.ViewerMuted)
		}
	})
}

// TestHistoryKeepsViewerStateFalse: the Ended nearby read keeps passing viewer 0
// to its extras read, and its finished views carry neither field, even for a
// reader who closed the report themselves (a finished report gets no updates).
func TestHistoryKeepsViewerStateFalse(t *testing.T) {
	fake := h6RescueFake()
	fake.extrasRows = []sqlcgen.ReportExtrasForReportsRow{mutedExtras(1, true), mutedExtras(2, true)}
	q := h6Query()
	q.ViewerAccountID = 11
	page, err := h6History(fake, q)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(page.Reports) == 0 {
		t.Fatal("History returned no reports")
	}
	for _, v := range page.Reports {
		if v.ViewerFollows || v.ViewerMuted {
			t.Errorf("report %d: follows %v muted %v, want both false", v.ID, v.ViewerFollows, v.ViewerMuted)
		}
	}
	if len(fake.extrasArgs) != 1 || fake.extrasArgs[0].ViewerAccountID != 0 {
		t.Errorf("extrasArgs = %+v, want one call with viewer 0", fake.extrasArgs)
	}
}
