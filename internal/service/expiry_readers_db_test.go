package service_test

// This file pins backlog 999.12 across the readers of reports.expires_at: after a
// confirmation from a new spot moves the one stored column, every reader agrees
// on the moved time, and the vote answer's trust equals the feed's. The tests need
// DATABASE_URL (testutil.NewTestDB skips without it).

import (
	"context"
	"reflect"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

func q64HasID[T any](rows []T, id int64, idOf func(T) int64) bool {
	for _, r := range rows {
		if idOf(r) == id {
			return true
		}
	}
	return false
}

func TestExtendedExpiryOneColumnEveryReader(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	votes := service.NewVotingService(q)
	reports := service.NewReportService(q)

	reporter := testutil.SeedReporterAccount(t, pool, "q64-readers-reporter@example.com", "q64-readers-sess")
	stranger := testutil.SeedReporterAccount(t, pool, "q64-readers-stranger@example.com")
	recipient := testutil.SeedReporterAccount(t, pool, "q64-readers-recipient@example.com")

	now := time.Now()
	r := testutil.InsertStoredReport(t, pool, testutil.StoredReport{
		SessionID:   "q64-readers-sess",
		Category:    "flood",
		Severity:    "medium",
		Description: "Water over the road near the bridge",
		Latitude:    q64Lat,
		Longitude:   q64Lon,
		CreatedAt:   now.Add(-7 * time.Hour),
		ExpiresAt:   now.Add(time.Hour),
	})
	created, e0 := q64Times(t, pool, r)

	// A queued phone note whose own life is far ahead, so only the report's life
	// can make it stale.
	if _, err := q.EnqueueInitialPush(ctx, sqlcgen.EnqueueInitialPushParams{
		AccountID: recipient, ReportID: r, Kind: "reliable", ExpiresAt: now.Add(48 * time.Hour), Now: now,
	}); err != nil {
		t.Fatalf("queueing the phone note: %v", err)
	}

	// The stranger confirms from a new spot: the stored time moves to E1.
	res := q64Cast(t, votes, q64Confirm(r, stranger, q64Lat, q64Lon))
	_, e1 := q64Times(t, pool, r)
	if !e1.After(e0) || !res.ExpiresAt.Equal(e1) {
		t.Fatalf("confirm did not move expires_at: before %v, after %v, answer %v", e0, e1, res.ExpiresAt)
	}

	// The feed's read time expiry agrees.
	views, err := reports.Nearby(ctx, service.NearbyQuery{Latitude: q64Lat, Longitude: q64Lon, RadiusKm: 5, ViewerAccountID: stranger})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	var found bool
	for _, v := range views {
		if v.ID == r {
			found = true
			if !v.ExpiresAt.Equal(e1) || !v.ConfirmExtends {
				t.Fatalf("Nearby: ExpiresAt %v and ConfirmExtends %v, want %v and true", v.ExpiresAt, v.ConfirmExtends, e1)
			}
		}
	}
	if !found {
		t.Fatalf("Nearby does not list report %d at the moved time", r)
	}

	latMin, latMax, lonMin, lonMax := service.BoundingBox(q64Lat, q64Lon, 5)
	expiredIDs := func(at time.Time) bool {
		rows, err := q.ExpiredReportsInWindow(ctx, sqlcgen.ExpiredReportsInWindowParams{
			Lat: q64Lat, Lon: q64Lon, Now: at, WindowStart: created.Add(-24 * time.Hour),
			LatMin: latMin, LatMax: latMax, LonMin: lonMin, LonMax: lonMax, RadiusKm: 5, RowCap: 500,
		})
		if err != nil {
			t.Fatalf("ExpiredReportsInWindow: %v", err)
		}
		return q64HasID(rows, r, func(x sqlcgen.ExpiredReportsInWindowRow) int64 { return x.ID })
	}
	openIDs := func(at time.Time) bool {
		rows, err := q.OpenReportsInArea(ctx, sqlcgen.OpenReportsInAreaParams{
			Lat: q64Lat, Lon: q64Lon, Now: at,
			LatMin: latMin, LatMax: latMax, LonMin: lonMin, LonMax: lonMax, RadiusKm: 5, RowCap: 1000,
		})
		if err != nil {
			t.Fatalf("OpenReportsInArea: %v", err)
		}
		return q64HasID(rows, r, func(x sqlcgen.OpenReportsInAreaRow) int64 { return x.ID })
	}
	judged := func(at time.Time) bool {
		rows, err := q.ReporterHistoryReports(ctx, sqlcgen.ReporterHistoryReportsParams{
			Now: at, WindowStart: at.Add(-90 * 24 * time.Hour), RowCap: 100, AccountIds: []int64{reporter},
		})
		if err != nil {
			t.Fatalf("ReporterHistoryReports: %v", err)
		}
		return q64HasID(rows, r, func(x sqlcgen.ReporterHistoryReportsRow) int64 { return x.ReportID })
	}
	pushSweep := func(at time.Time) bool {
		ids, err := q.RecentReportsForPush(ctx, sqlcgen.RecentReportsForPushParams{Since: created.Add(-time.Hour), Now: at, RowCap: 50})
		if err != nil {
			t.Fatalf("RecentReportsForPush: %v", err)
		}
		return q64HasID(ids, r, func(x int64) int64 { return x })
	}

	// At E0 plus an hour the old time has passed and the new one has not.
	mid := e0.Add(time.Hour)
	if !openIDs(mid) || expiredIDs(mid) || judged(mid) || !pushSweep(mid) {
		t.Fatalf("at %v (old time passed, new not): open %v, expired %v, judged %v, push sweep %v; want true, false, false, true",
			mid, openIDs(mid), expiredIDs(mid), judged(mid), pushSweep(mid))
	}
	if n, err := q.SkipStalePushes(ctx, mid); err != nil || n != 0 {
		t.Fatalf("SkipStalePushes at %v skipped %d (err %v), want 0", mid, n, err)
	}

	// A minute after E1 the report has really faded for every reader.
	late := e1.Add(time.Minute)
	if openIDs(late) || !expiredIDs(late) || !judged(late) || pushSweep(late) {
		t.Fatalf("at %v (new time passed): open %v, expired %v, judged %v, push sweep %v; want false, true, true, false",
			late, openIDs(late), expiredIDs(late), judged(late), pushSweep(late))
	}
	if n, err := q.SkipStalePushes(ctx, late); err != nil || n != 1 {
		t.Fatalf("SkipStalePushes at %v skipped %d (err %v), want 1", late, n, err)
	}
}

func TestCastVoteTrustMatchesFeedAfterExtension(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	pin := time.Now()
	clock := func() time.Time { return pin }
	votes := service.NewVotingService(q, service.WithVotingClock(clock))
	reports := service.NewReportService(q, service.WithReportClock(clock))

	reporter := testutil.SeedReporterAccount(t, pool, "q64-parity-reporter@example.com", "q64-parity-sess")
	voter := testutil.SeedReporterAccount(t, pool, "q64-parity-voter@example.com")

	short := q64Report(t, pool, "q64-parity-sess", "medium", time.Hour, time.Hour)
	nearCap := q64Report(t, pool, "q64-parity-sess", "medium", 20*time.Hour, time.Hour)

	viewOf := func(t *testing.T, viewer, id int64) service.ReportView {
		t.Helper()
		views, err := reports.Nearby(ctx, service.NearbyQuery{Latitude: q64Lat, Longitude: q64Lon, RadiusKm: 5, ViewerAccountID: viewer})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		for _, v := range views {
			if v.ID == id {
				return v
			}
		}
		t.Fatalf("Nearby does not list report %d", id)
		return service.ReportView{}
	}

	cases := []struct {
		name        string
		id          int64
		window      int
		wantExtends bool
	}{
		{"a report with a seeded 2 hour life, extended", short, 120, true},
		{"a report confirmed to its cap", nearCap, 120, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := votes.CastVote(ctx, q64Confirm(tc.id, voter, q64Lat, q64Lon))
			if err != nil {
				t.Fatalf("CastVote: %v", err)
			}
			if res.Trust.Currency.WindowMinutes != tc.window {
				t.Fatalf("vote answer WindowMinutes = %d, want %d", res.Trust.Currency.WindowMinutes, tc.window)
			}
			for who, viewer := range map[string]int64{"the voter": voter, "the reporter": reporter} {
				view := viewOf(t, viewer, tc.id)
				if !reflect.DeepEqual(res.Trust, view.Trust) {
					t.Fatalf("trust read by %s differs from the vote answer:\nfeed %+v\nvote %+v", who, view.Trust, res.Trust)
				}
				if view.ConfirmExtends != tc.wantExtends {
					t.Fatalf("ConfirmExtends read by %s = %v, want %v", who, view.ConfirmExtends, tc.wantExtends)
				}
			}
		})
	}
}
