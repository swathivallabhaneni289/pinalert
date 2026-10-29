package service_test

// reporterpath_test.go proves the viewer dependent reporter_profile_path on a
// real database (plan 09-09, PROF-01, PROF-03, D-04, D-12, D-20, T-09-33,
// T-09-98). Helpers carry the prefix rpp so they never collide with other files.

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	rppLat = 12.9716
	rppLon = 77.5946
)

type rppRig struct {
	pool *pgxpool.Pool
	now  time.Time
	svc  *service.ReportService
}

func newRppRig(t *testing.T) *rppRig {
	t.Helper()
	pool := testutil.NewTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	return &rppRig{
		pool: pool,
		now:  now,
		svc:  service.NewReportService(sqlcgen.New(pool), service.WithReportClock(func() time.Time { return now })),
	}
}

// person seeds an account bound to a session, with a username, its screen and
// the profile flag as given, and returns the account id.
func (r *rppRig) person(t *testing.T, email, session, username, screen string, public bool) int64 {
	t.Helper()
	id := testutil.SeedReporterAccount(t, r.pool, email, session)
	testutil.SeedUsername(t, r.pool, id, username, screen)
	if public {
		testutil.MustExec(t, r.pool, "UPDATE accounts SET profile_public = true, profile_public_at = now() WHERE id = $1", id)
	}
	return id
}

// live files a live report from the session that expires in six hours.
func (r *rppRig) live(t *testing.T, session string) int64 {
	t.Helper()
	return testutil.SeedHistoryReport(t, r.pool, session, "low", "flood", r.now.Add(6*time.Hour))
}

func (r *rppRig) vote(t *testing.T, reportID, account int64, kind, value, cell string, at time.Time) {
	t.Helper()
	testutil.MustExec(t, r.pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`, reportID, account, kind, value, cell, at)
}

func (r *rppRig) nearby(t *testing.T, viewer int64) map[int64]service.ReportView {
	t.Helper()
	views, err := r.svc.Nearby(context.Background(), service.NearbyQuery{
		Latitude: rppLat, Longitude: rppLon, RadiusKm: 10, ViewerAccountID: viewer,
	})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	out := make(map[int64]service.ReportView, len(views))
	for _, v := range views {
		out[v.ID] = v
	}
	return out
}

func (r *rppRig) focus(t *testing.T, reportID, viewer int64) service.ReportView {
	t.Helper()
	res, err := r.svc.Focus(context.Background(), service.FocusQuery{
		ReportID: reportID, Latitude: rppLat, Longitude: rppLon, ViewerAccountID: viewer,
	})
	if err != nil {
		t.Fatalf("Focus %d: %v", reportID, err)
	}
	if res.Status != service.FocusOK || res.View == nil {
		t.Fatalf("Focus %d status = %q, want ok", reportID, res.Status)
	}
	return *res.View
}

func TestReporterProfilePath(t *testing.T) {
	r := newRppRig(t)
	viewer := testutil.SeedReporterAccount(t, r.pool, "rpp-viewer@example.com", "rpp-viewer-s")

	pub := r.person(t, "rpp-pub@example.com", "rpp-pub-s", "ravi.k", "clear", true)
	r.person(t, "rpp-private@example.com", "rpp-private-s", "quiet.one", "clear", false)
	r.person(t, "rpp-pending@example.com", "rpp-pending-s", "waiting.name", "pending", true)
	blocker := r.person(t, "rpp-blocker@example.com", "rpp-blocker-s", "blocks.me", "clear", true)
	testutil.SeedBlock(t, r.pool, blocker, viewer)
	// The viewer is a public person too, so their own report is the case where
	// the path must still be empty.
	testutil.MustExec(t, r.pool, "UPDATE accounts SET username = 'viewer.me', username_screen = 'clear', profile_public = true, profile_public_at = now() WHERE id = $1", viewer)

	pubRep := r.live(t, "rpp-pub-s")
	privRep := r.live(t, "rpp-private-s")
	pendRep := r.live(t, "rpp-pending-s")
	blockRep := r.live(t, "rpp-blocker-s")
	ownRep := r.live(t, "rpp-viewer-s")

	got := r.nearby(t, viewer)
	for _, id := range []int64{pubRep, privRep, pendRep, blockRep, ownRep} {
		if _, ok := got[id]; !ok {
			t.Fatalf("report %d missing from Nearby", id)
		}
	}
	if p := got[pubRep].ReporterProfilePath; p != "/u/ravi.k" {
		t.Errorf("public reporter path = %q, want /u/ravi.k", p)
	}
	if got[pubRep].ReporterAccount() != pub {
		t.Errorf("ReporterAccount = %d, want %d", got[pubRep].ReporterAccount(), pub)
	}
	for name, id := range map[string]int64{"private": privRep, "pending name": pendRep, "blocked the viewer": blockRep, "own report": ownRep} {
		if p := got[id].ReporterProfilePath; p != "" {
			t.Errorf("%s: path = %q, want empty", name, p)
		}
	}

	// Focus gives the same answer for a live report.
	if p := r.focus(t, pubRep, viewer).ReporterProfilePath; p != "/u/ravi.k" {
		t.Errorf("Focus public path = %q, want /u/ravi.k", p)
	}
	for name, id := range map[string]int64{"private": privRep, "pending name": pendRep, "blocked the viewer": blockRep, "own report": ownRep} {
		if p := r.focus(t, id, viewer).ReporterProfilePath; p != "" {
			t.Errorf("Focus %s: path = %q, want empty", name, p)
		}
	}

	// An unidentified reader gets no path anywhere.
	for id, v := range r.nearby(t, 0) {
		if v.ReporterProfilePath != "" {
			t.Errorf("viewer 0: report %d path = %q, want empty", id, v.ReporterProfilePath)
		}
	}
	if p := r.focus(t, pubRep, 0).ReporterProfilePath; p != "" {
		t.Errorf("Focus viewer 0 path = %q, want empty", p)
	}

	// Switching the profile off takes the link away at the next read (D-12).
	testutil.MustExec(t, r.pool, "UPDATE accounts SET profile_public = false WHERE id = $1", pub)
	if p := r.nearby(t, viewer)[pubRep].ReporterProfilePath; p != "" {
		t.Errorf("after profile off: path = %q, want empty", p)
	}
}

func TestFinishedViewsCarryNoProfilePath(t *testing.T) {
	r := newRppRig(t)
	viewer := testutil.SeedReporterAccount(t, r.pool, "rpp2-viewer@example.com", "rpp2-viewer-s")
	voter := testutil.SeedReporterAccount(t, r.pool, "rpp2-voter@example.com")
	pub := r.person(t, "rpp2-pub@example.com", "rpp2-pub-s", "ravi.k", "clear", true)

	// A live report the voter confirms: the control that proves the voted row of
	// Activity does carry the path while the report is live.
	liveRep := r.live(t, "rpp2-pub-s")
	r.vote(t, liveRep, voter, "content", "confirm", "tdr1v8y", r.now.Add(-30*time.Minute))

	// A report the poster resolved, and one that faded an hour ago.
	resolvedRep := r.live(t, "rpp2-pub-s")
	r.vote(t, resolvedRep, voter, "content", "confirm", "tdr1v8y", r.now.Add(-90*time.Minute))
	r.vote(t, resolvedRep, pub, "resolution", "resolve", "tdr1v8y", r.now.Add(-time.Hour))
	expiredRep := testutil.SeedHistoryReport(t, r.pool, "rpp2-pub-s", "low", "flood", r.now.Add(-time.Hour))
	r.vote(t, expiredRep, voter, "content", "confirm", "tdr1v8y", r.now.Add(-3*time.Hour))

	if p := r.focus(t, liveRep, viewer).ReporterProfilePath; p != "/u/ravi.k" {
		t.Fatalf("control: live Focus path = %q, want /u/ravi.k", p)
	}

	// Focus of the two finished reports.
	for name, id := range map[string]int64{"resolved": resolvedRep, "expired": expiredRep} {
		v := r.focus(t, id, viewer)
		if v.View != service.FeedViewResolved && v.View != service.FeedViewPast {
			t.Fatalf("Focus %s: view = %q, want a finished view", name, v.View)
		}
		if v.ReporterProfilePath != "" {
			t.Errorf("Focus %s: path = %q, want empty (D-20)", name, v.ReporterProfilePath)
		}
	}

	// History (the Past tab).
	page, err := r.svc.History(context.Background(), service.HistoryQuery{
		Latitude: rppLat, Longitude: rppLon, RadiusKm: 10, ViewerAccountID: viewer,
	})
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	seen := map[int64]bool{}
	for _, v := range page.Reports {
		seen[v.ID] = true
		if v.ReporterProfilePath != "" {
			t.Errorf("History report %d: path = %q, want empty (D-20)", v.ID, v.ReporterProfilePath)
		}
	}
	if !seen[resolvedRep] || !seen[expiredRep] {
		t.Fatalf("History listed %v, want both the resolved and the expired report", seen)
	}

	// The Activity page of the voter: live row carries the path, finished rows
	// carry none.
	auth := service.NewAuthService(sqlcgen.New(r.pool), nil, "https://pinalert.example", service.WithClock(func() time.Time { return r.now }))
	act, err := auth.ActivityPageForAccount(context.Background(), voter)
	if err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}
	byID := map[int64]service.VotedReport{}
	for _, v := range act.Votes {
		byID[v.Row.ID] = v
	}
	for _, id := range []int64{liveRep, resolvedRep, expiredRep} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("Activity votes missing report %d", id)
		}
	}
	if p := byID[liveRep].ReporterProfilePath; p != "/u/ravi.k" {
		t.Errorf("Activity live row path = %q, want /u/ravi.k", p)
	}
	for name, id := range map[string]int64{"resolved": resolvedRep, "expired": expiredRep} {
		if p := byID[id].ReporterProfilePath; p != "" {
			t.Errorf("Activity %s row: path = %q, want empty (D-20)", name, p)
		}
	}
}
