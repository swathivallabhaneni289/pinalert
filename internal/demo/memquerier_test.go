package demo

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// The tests in this file drive the in memory querier directly, one method at a
// time, against the behaviour the SQL gives. The services built on top of it
// are exercised in world_test.go and, against a real database, in
// parity_db_test.go.

const (
	baseLat = 16.5200
	baseLon = 80.6200
)

var _ service.Querier = (*memQuerier)(nil)
var _ service.VotingQuerier = (*memQuerier)(nil)

// testClock is a hand moved clock for the querier tests.
type testClock struct{ t time.Time }

func (c *testClock) now() time.Time          { return c.t }
func (c *testClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestQuerier() (*memQuerier, *testClock) {
	c := &testClock{t: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)}
	return newMemQuerier(c.now), c
}

// northKm returns a latitude km kilometres north of lat, close enough to 111.2
// km per degree for the distances these tests need.
func northKm(lat, km float64) float64 { return lat + km/111.195 }

// insertAt stores a report with a life of life at the given point and returns
// its id. The querier restamps created_at from its own clock.
func insertAt(t *testing.T, q *memQuerier, c *testClock, lat, lon float64, life time.Duration) int64 {
	t.Helper()
	now := c.now()
	row, err := q.InsertReportIdempotent(context.Background(), sqlcgen.InsertReportIdempotentParams{
		SessionID:       "demo-1",
		Category:        "flood",
		Severity:        "medium",
		Description:     "water over the road",
		Latitude:        lat,
		Longitude:       lon,
		Geohash:         "tdr1",
		CreatedAt:       now,
		ExpiresAt:       now.Add(life),
		ModerationState: "clear",
	})
	if err != nil {
		t.Fatalf("InsertReportIdempotent: %v", err)
	}
	return row.ID
}

func nearby(t *testing.T, q *memQuerier, lat, lon, radiusKm float64) []sqlcgen.NearbyReportsRow {
	t.Helper()
	latMin, latMax, lonMin, lonMax := service.BoundingBox(lat, lon, radiusKm)
	rows, err := q.NearbyReports(context.Background(), sqlcgen.NearbyReportsParams{
		Lat: lat, Lon: lon, LatMin: latMin, LatMax: latMax, LonMin: lonMin, LonMax: lonMax, RadiusKm: radiusKm,
	})
	if err != nil {
		t.Fatalf("NearbyReports: %v", err)
	}
	return rows
}

func TestMemQuerierImplementsInterfaces(t *testing.T) {
	w := NewWorld("abcdefghijklmnopqrstuv", time.Now)
	if w.Reports == nil || w.Votes == nil {
		t.Fatalf("a world must carry the real services, got reports=%v votes=%v", w.Reports, w.Votes)
	}
	if w.ID != "abcdefghijklmnopqrstuv" {
		t.Fatalf("world id = %q", w.ID)
	}
}

func TestNearbyReportsSemantics(t *testing.T) {
	q, c := newTestQuerier()
	near := insertAt(t, q, c, northKm(baseLat, 0.5), baseLon, 8*time.Hour)
	far := insertAt(t, q, c, northKm(baseLat, 3), baseLon, 8*time.Hour)
	nearer := insertAt(t, q, c, northKm(baseLat, 0.2), baseLon, 8*time.Hour)
	short := insertAt(t, q, c, northKm(baseLat, 0.1), baseLon, 1*time.Hour)

	rows := nearby(t, q, baseLat, baseLon, 2)
	var ids []int64
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	want := []int64{short, nearer, near}
	if len(ids) != len(want) {
		t.Fatalf("radius 2 km returned %v, want %v (the 3 km report must be out: %d)", ids, want, far)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order = %v, want nearest first %v", ids, want)
		}
	}
	if math.Abs(rows[2].DistanceKm-0.5) > 0.01 {
		t.Fatalf("distance of the 0.5 km report = %v", rows[2].DistanceKm)
	}

	// The same report leaves the answer once the world clock passes its
	// expiry, as expires_at > now() does in the SQL.
	c.advance(2 * time.Hour)
	rows = nearby(t, q, baseLat, baseLon, 2)
	for _, r := range rows {
		if r.ID == short {
			t.Fatalf("an expired report was returned")
		}
	}
	if len(rows) != 2 {
		t.Fatalf("after expiry got %d rows, want 2", len(rows))
	}
}

func TestNearbyReportsBoundingBoxIsInclusive(t *testing.T) {
	q, c := newTestQuerier()
	insertAt(t, q, c, baseLat, baseLon, 8*time.Hour)
	// A box that excludes the report even though the radius would include it:
	// the SQL applies BETWEEN first, so the box wins.
	rows, err := q.NearbyReports(context.Background(), sqlcgen.NearbyReportsParams{
		Lat: baseLat, Lon: baseLon, LatMin: baseLat + 0.001, LatMax: baseLat + 0.002,
		LonMin: baseLon - 1, LonMax: baseLon + 1, RadiusKm: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("a report outside the box was returned: %d rows", len(rows))
	}
	// A box whose edge is exactly the report's latitude includes it.
	rows, err = q.NearbyReports(context.Background(), sqlcgen.NearbyReportsParams{
		Lat: baseLat, Lon: baseLon, LatMin: baseLat, LatMax: baseLat,
		LonMin: baseLon, LonMax: baseLon, RadiusKm: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("BETWEEN is inclusive, got %d rows", len(rows))
	}
}

func TestCurrentVotesNewestWins(t *testing.T) {
	q, c := newTestQuerier()
	ctx := context.Background()
	id := insertAt(t, q, c, baseLat, baseLon, 8*time.Hour)

	vote := func(acct int64, kind, value, cell string) {
		t.Helper()
		if err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{ReportID: id, AccountID: acct, Kind: kind, Value: value, GeohashCell: cell}); err != nil {
			t.Fatalf("InsertVote: %v", err)
		}
	}
	// Two confirms by account 7 at the same instant: the later id wins.
	vote(7, "content", "confirm", "tdr1aaa")
	vote(7, "content", "confirm", "tdr1bbb")
	rows, err := q.CurrentVotesForReports(ctx, []int64{id})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].GeohashCell != "tdr1bbb" {
		t.Fatalf("want one row with the newer cell, got %+v", rows)
	}

	// A later clock reading wins over an earlier one whatever the ids: time first, id second.
	c.advance(time.Minute)
	vote(7, "content", "dispute", "tdr1ccc")
	vote(9, "resolution", "resolve", "tdr1ddd")
	vote(5, "content", "confirm", "tdr1eee")
	rows, err = q.CurrentVotesForReports(ctx, []int64{id})
	if err != nil {
		t.Fatal(err)
	}
	// Confirm and dispute are both values of kind content, so the dispute
	// replaces the confirm. DISTINCT ON is (report, account, kind): only a
	// resolution row by the same account would be a second row.
	if len(rows) != 3 {
		t.Fatalf("want 3 rows (7 content, 9 resolution, 5 content), got %+v", rows)
	}
	// Output is ordered by report, account, kind like the SQL.
	wantOrder := []struct {
		acct  int64
		kind  string
		value string
	}{{5, "content", "confirm"}, {7, "content", "dispute"}, {9, "resolution", "resolve"}}
	for i, w := range wantOrder {
		if rows[i].AccountID != w.acct || rows[i].Kind != w.kind || rows[i].Value != w.value {
			t.Fatalf("row %d = %+v, want %+v", i, rows[i], w)
		}
	}

	// Other report ids are not mixed in and an unknown id gives no rows.
	other, err := q.CurrentVotesForReports(ctx, []int64{id + 100})
	if err != nil || len(other) != 0 {
		t.Fatalf("unknown id: rows=%v err=%v", other, err)
	}
}

func TestPastWindowReads(t *testing.T) {
	q, c := newTestQuerier()
	ctx := context.Background()
	now := c.now()

	// Expired 8 days ago: out of the 7 day window.
	c.t = now.Add(-9*24*time.Hour - 8*time.Hour)
	old := insertAt(t, q, c, baseLat, baseLon, 8*time.Hour)
	// Expired one hour ago.
	c.t = now.Add(-9 * time.Hour)
	recent := insertAt(t, q, c, baseLat, baseLon, 8*time.Hour)
	// Still open, 2 km away, and a second one 0.5 km away.
	c.t = now
	openFar := insertAt(t, q, c, northKm(baseLat, 2), baseLon, 8*time.Hour)
	openNear := insertAt(t, q, c, northKm(baseLat, 0.5), baseLon, 8*time.Hour)

	latMin, latMax, lonMin, lonMax := service.BoundingBox(baseLat, baseLon, 5)
	expired, err := q.ExpiredReportsInWindow(ctx, sqlcgen.ExpiredReportsInWindowParams{
		Lat: baseLat, Lon: baseLon, Now: now, WindowStart: now.Add(-7 * 24 * time.Hour),
		LatMin: latMin, LatMax: latMax, LonMin: lonMin, LonMax: lonMax, RadiusKm: 5, RowCap: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || expired[0].ID != recent {
		t.Fatalf("expired window = %+v, want only report %d (not %d)", expired, recent, old)
	}

	open, err := q.OpenReportsInArea(ctx, sqlcgen.OpenReportsInAreaParams{
		Lat: baseLat, Lon: baseLon, Now: now,
		LatMin: latMin, LatMax: latMax, LonMin: lonMin, LonMax: lonMax, RadiusKm: 5, RowCap: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 2 || open[0].ID != openNear || open[1].ID != openFar {
		t.Fatalf("open set = %+v, want nearest first %d then %d", open, openNear, openFar)
	}

	// The cap keeps the nearest.
	capped, err := q.OpenReportsInArea(ctx, sqlcgen.OpenReportsInAreaParams{
		Lat: baseLat, Lon: baseLon, Now: now,
		LatMin: latMin, LatMax: latMax, LonMin: lonMin, LonMax: lonMax, RadiusKm: 5, RowCap: 1,
	})
	if err != nil || len(capped) != 1 || capped[0].ID != openNear {
		t.Fatalf("capped open = %+v err=%v", capped, err)
	}

	// ReportForFocus has no expiry or area predicate and reports no rows as pgx does.
	focus, err := q.ReportForFocus(ctx, sqlcgen.ReportForFocusParams{Lat: baseLat, Lon: baseLon, ReportID: old})
	if err != nil || focus.ID != old {
		t.Fatalf("focus on an old report: %+v err=%v", focus, err)
	}
	if _, err := q.ReportForFocus(ctx, sqlcgen.ReportForFocusParams{ReportID: 9999}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("focus on a missing report: err=%v, want pgx.ErrNoRows", err)
	}
}

func TestMissingRowsLookLikeNoRows(t *testing.T) {
	q, _ := newTestQuerier()
	ctx := context.Background()
	if _, err := q.ReportVoteContext(ctx, 42); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("ReportVoteContext: %v", err)
	}
	if _, err := q.ReportByRequest(ctx, sqlcgen.ReportByRequestParams{AccountID: 1, RequestID: "x"}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("ReportByRequest: %v", err)
	}
	if errors.Is(errors.New("some other failure"), pgx.ErrNoRows) {
		t.Fatalf("control: an unrelated error matched ErrNoRows")
	}
}

func TestUndoCountsAndHistory(t *testing.T) {
	q, c := newTestQuerier()
	ctx := context.Background()
	q.BindSession("demo-1", 1)
	id := insertAt(t, q, c, baseLat, baseLon, 8*time.Hour)
	for _, v := range []string{"resolve", "unresolve", "reopen", "unresolve"} {
		if err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{ReportID: id, AccountID: 1, Kind: "resolution", Value: v, GeohashCell: "tdr1aaa"}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := q.ResolutionUndoCounts(ctx, sqlcgen.ResolutionUndoCountsParams{
		UnresolveValue: "unresolve", ReopenValue: "reopen", AccountID: 1, Kind: "resolution", ReportIds: []int64{id, id + 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ReportID != id || rows[0].Unresolves != 2 || rows[0].Reopens != 1 {
		t.Fatalf("undo counts = %+v", rows)
	}
	none, err := q.ResolutionUndoCounts(ctx, sqlcgen.ResolutionUndoCountsParams{
		UnresolveValue: "unresolve", ReopenValue: "reopen", AccountID: 2, Kind: "resolution", ReportIds: []int64{id},
	})
	if err != nil || len(none) != 0 {
		t.Fatalf("another account's counts = %+v err=%v", none, err)
	}

	// History: only expired, only inside the window, only the account's own.
	c.advance(9 * time.Hour)
	hist, err := q.ReporterHistoryReports(ctx, sqlcgen.ReporterHistoryReportsParams{
		Now: c.now(), WindowStart: c.now().Add(-90 * 24 * time.Hour), RowCap: 100, AccountIds: []int64{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || hist[0].AccountID != 1 || hist[0].ReportID != id {
		t.Fatalf("history = %+v", hist)
	}
	// The session map mirrors the LEFT JOIN: an unbound session gives a nil reporter.
	other, err := q.InsertReportIdempotent(ctx, sqlcgen.InsertReportIdempotentParams{
		SessionID: "demo-unbound", Category: "flood", Severity: "low", Description: "no reporter account",
		Latitude: baseLat, Longitude: baseLon, CreatedAt: c.now(), ExpiresAt: c.now().Add(time.Hour), ModerationState: "clear",
	})
	if err != nil {
		t.Fatal(err)
	}
	accts, err := q.ReporterAccountsForReports(ctx, []int64{id, other.ID})
	if err != nil || len(accts) != 2 {
		t.Fatalf("accounts = %+v err=%v", accts, err)
	}
	if accts[0].ReporterAccountID == nil || *accts[0].ReporterAccountID != 1 {
		t.Fatalf("bound session lost its account: %+v", accts[0])
	}
	if accts[1].ReporterAccountID != nil {
		t.Fatalf("an unbound session must give a nil reporter, got %v", *accts[1].ReporterAccountID)
	}
}

func TestRequestIDReplay(t *testing.T) {
	q, c := newTestQuerier()
	ctx := context.Background()
	rid := "abcdefabcdefabcdef"
	acct := int64(1)
	params := sqlcgen.InsertReportIdempotentParams{
		SessionID: "demo-1", Category: "flood", Severity: "low", Description: "first and only copy",
		Latitude: baseLat, Longitude: baseLon, CreatedAt: c.now(), ExpiresAt: c.now().Add(time.Hour),
		ModerationState: "clear", RequestID: &rid, RequestAccountID: &acct,
	}
	first, err := q.InsertReportIdempotent(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.InsertReportIdempotent(ctx, params); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("a conflicting request id must give no row like ON CONFLICT DO NOTHING, got %v", err)
	}
	got, err := q.ReportByRequest(ctx, sqlcgen.ReportByRequestParams{AccountID: 1, RequestID: rid})
	if err != nil || got.ID != first.ID {
		t.Fatalf("ReportByRequest = %+v err=%v", got, err)
	}
	// Scoped to the account: another account cannot read it back (T-04-05).
	if _, err := q.ReportByRequest(ctx, sqlcgen.ReportByRequestParams{AccountID: 2, RequestID: rid}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("another account read a stored request: %v", err)
	}
}

func TestWorldCaps(t *testing.T) {
	q, c := newTestQuerier()
	ctx := context.Background()
	rid := "abcdefabcdefabcdef"
	acct := int64(1)

	var firstID int64
	for i := 0; i < MaxReportsPerWorld; i++ {
		id := insertAt(t, q, c, baseLat, baseLon, time.Hour)
		if i == 0 {
			firstID = id
		}
	}
	if q.reportCount() != MaxReportsPerWorld {
		t.Fatalf("reports = %d", q.reportCount())
	}
	_, err := q.InsertReportIdempotent(ctx, sqlcgen.InsertReportIdempotentParams{
		SessionID: "demo-1", Category: "flood", Severity: "low", Description: "one too many",
		Latitude: baseLat, Longitude: baseLon, CreatedAt: c.now(), ExpiresAt: c.now().Add(time.Hour), ModerationState: "clear",
		RequestID: &rid, RequestAccountID: &acct,
	})
	if !errors.Is(err, ErrWorldFull) {
		t.Fatalf("41st report: err=%v, want ErrWorldFull", err)
	}
	if q.reportCount() != MaxReportsPerWorld {
		t.Fatalf("a refused insert changed the world: %d reports", q.reportCount())
	}

	for i := 0; i < MaxVotesPerWorld; i++ {
		if err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{ReportID: firstID, AccountID: int64(i%7 + 1), Kind: "content", Value: "confirm", GeohashCell: "tdr1aaa"}); err != nil {
			t.Fatalf("vote %d: %v", i+1, err)
		}
	}
	err = q.InsertVote(ctx, sqlcgen.InsertVoteParams{ReportID: firstID, AccountID: 1, Kind: "content", Value: "confirm", GeohashCell: "tdr1aaa"})
	if !errors.Is(err, ErrWorldFull) {
		t.Fatalf("401st vote: err=%v, want ErrWorldFull", err)
	}
	if q.voteCount() != MaxVotesPerWorld {
		t.Fatalf("a refused vote changed the world: %d votes", q.voteCount())
	}
}

func TestInsertModerationCheckStoresNothingVisible(t *testing.T) {
	q, c := newTestQuerier()
	id := insertAt(t, q, c, baseLat, baseLon, time.Hour)
	if err := q.InsertModerationCheck(context.Background(), sqlcgen.InsertModerationCheckParams{ReportID: id, Status: "pending", NextAttemptAt: c.now()}); err != nil {
		t.Fatal(err)
	}
	extras, err := q.ReportExtrasForReports(context.Background(), sqlcgen.ReportExtrasForReportsParams{ReportIds: []int64{id, id + 5}, ViewerAccountID: 1})
	if err != nil {
		t.Fatal(err)
	}
	// One row per existing report, none for the missing id (the SQL joins from reports).
	if len(extras) != 1 || extras[0].ReportID != id || extras[0].ModerationState != "clear" || extras[0].ViewerFlagged || extras[0].ViewerMuted {
		t.Fatalf("extras = %+v", extras)
	}
}

// TestMemQuerierExtendsLikeSQL pins the twin of extend.sql (backlog 999.12) with
// no database: the same cases as the database test, driven through the querier
// with a clock the test moves. The stored fade time is read back through
// ReportVoteContext.
func TestMemQuerierExtendsLikeSQL(t *testing.T) {
	ctx := context.Background()
	const life = 8 * time.Hour
	const limit = 24 * time.Hour

	storedExpiry := func(t *testing.T, q *memQuerier, id int64) (created, expires time.Time) {
		t.Helper()
		rc, err := q.ReportVoteContext(ctx, id)
		if err != nil {
			t.Fatalf("ReportVoteContext: %v", err)
		}
		return rc.CreatedAt, rc.ExpiresAt
	}
	voteWith := func(t *testing.T, q *memQuerier, id, account int64, value, cell string, extend bool, lifeD, capD time.Duration) time.Time {
		t.Helper()
		var reporter *int64
		if rc, err := q.ReportVoteContext(ctx, id); err == nil {
			reporter = rc.ReporterAccountID
		}
		got, err := q.InsertVoteExtending(ctx, sqlcgen.InsertVoteExtendingParams{
			ReportID: id, AccountID: account, Kind: "content", Value: value, GeohashCell: cell,
			CapSeconds: int64(capD / time.Second), LifeSeconds: int64(lifeD / time.Second),
			Extend: extend, ReporterAccountID: reporter,
		})
		if err != nil {
			t.Fatalf("InsertVoteExtending: %v", err)
		}
		_, stored := storedExpiry(t, q, id)
		if !got.Equal(stored) {
			t.Fatalf("returned %v but stored %v", got, stored)
		}
		return got
	}
	vote := func(t *testing.T, q *memQuerier, id, account int64, value, cell string, extend bool) time.Time {
		t.Helper()
		return voteWith(t, q, id, account, value, cell, extend, life, limit)
	}

	t.Run("a new spot extends to the vote time plus the lifetime", func(t *testing.T) {
		q, c := newTestQuerier()
		q.BindSession("demo-1", 1)
		id := insertAt(t, q, c, baseLat, baseLon, 6*time.Hour)
		created, before := storedExpiry(t, q, id)
		c.advance(2 * time.Hour)
		got := vote(t, q, id, 2, "confirm", "cellaaa", true)
		want := service.ExtendedExpiry(created, before, c.now(), life, limit)
		if !got.Equal(want) || !got.After(before) || !got.Equal(c.now().Add(life)) {
			t.Fatalf("got %v, want %v (vote time plus 8h, later than %v)", got, want, before)
		}
	})

	t.Run("the same cell by another account does not extend", func(t *testing.T) {
		q, c := newTestQuerier()
		id := insertAt(t, q, c, baseLat, baseLon, 6*time.Hour)
		c.advance(time.Hour)
		first := vote(t, q, id, 2, "confirm", "cellaaa", true)
		c.advance(time.Hour)
		if got := vote(t, q, id, 3, "confirm", "cellaaa", true); !got.Equal(first) {
			t.Fatalf("got %v, want unchanged %v", got, first)
		}
	})

	t.Run("the same account from another cell does not extend", func(t *testing.T) {
		q, c := newTestQuerier()
		id := insertAt(t, q, c, baseLat, baseLon, 6*time.Hour)
		c.advance(time.Hour)
		first := vote(t, q, id, 2, "confirm", "cellaaa", true)
		c.advance(time.Hour)
		if got := vote(t, q, id, 2, "confirm", "cellbbb", true); !got.Equal(first) {
			t.Fatalf("got %v, want unchanged %v", got, first)
		}
	})

	t.Run("a dispute then a confirm from a taken back spot never extends twice", func(t *testing.T) {
		q, c := newTestQuerier()
		id := insertAt(t, q, c, baseLat, baseLon, 6*time.Hour)
		c.advance(time.Hour)
		first := vote(t, q, id, 2, "confirm", "cellaaa", true)
		c.advance(time.Hour)
		vote(t, q, id, 2, "dispute", "cellaaa", false)
		c.advance(time.Hour)
		if got := vote(t, q, id, 3, "confirm", "cellaaa", true); !got.Equal(first) {
			t.Fatalf("a second account in a taken back spot: got %v, want unchanged %v", got, first)
		}
	})

	t.Run("Extend false never extends", func(t *testing.T) {
		q, c := newTestQuerier()
		id := insertAt(t, q, c, baseLat, baseLon, 6*time.Hour)
		_, before := storedExpiry(t, q, id)
		c.advance(time.Hour)
		if got := vote(t, q, id, 2, "confirm", "cellaaa", false); !got.Equal(before) {
			t.Fatalf("got %v, want unchanged %v", got, before)
		}
	})

	t.Run("the medium cap holds", func(t *testing.T) {
		q, c := newTestQuerier()
		id := insertAt(t, q, c, baseLat, baseLon, 21*time.Hour)
		created, _ := storedExpiry(t, q, id)
		c.advance(20 * time.Hour)
		got := vote(t, q, id, 2, "confirm", "cellaaa", true)
		if !got.Equal(created.Add(24 * time.Hour)) {
			t.Fatalf("medium got %v, want created plus 24h", got)
		}
		if got := vote(t, q, id, 3, "confirm", "cellbbb", true); !got.Equal(created.Add(24 * time.Hour)) {
			t.Fatalf("second new spot at the cap got %v", got)
		}
	})

	t.Run("the Critical lifetime and cap hold", func(t *testing.T) {
		const critLife, critCap = 24 * time.Hour, 48 * time.Hour
		q, c := newTestQuerier()
		id := insertAt(t, q, c, baseLat, baseLon, 20*time.Hour)
		created, _ := storedExpiry(t, q, id)
		c.advance(10 * time.Hour)
		if got := voteWith(t, q, id, 2, "confirm", "cellaaa", true, critLife, critCap); !got.Equal(c.now().Add(critLife)) {
			t.Fatalf("critical under its cap got %v, want vote time plus 24h = %v", got, c.now().Add(critLife))
		}
		c.advance(15 * time.Hour)
		if got := voteWith(t, q, id, 3, "confirm", "cellbbb", true, critLife, critCap); !got.Equal(created.Add(critCap)) {
			t.Fatalf("critical near its cap got %v, want created plus 48h = %v", got, created.Add(critCap))
		}
	})

	t.Run("never earlier than the current fade time", func(t *testing.T) {
		q, c := newTestQuerier()
		id := insertAt(t, q, c, baseLat, baseLon, 20*time.Hour)
		_, before := storedExpiry(t, q, id)
		c.advance(time.Hour)
		if got := vote(t, q, id, 2, "confirm", "cellaaa", true); !got.Equal(before) {
			t.Fatalf("got %v, want unchanged %v", got, before)
		}
	})

	t.Run("a faded report is not revived", func(t *testing.T) {
		q, c := newTestQuerier()
		id := insertAt(t, q, c, baseLat, baseLon, time.Hour)
		_, before := storedExpiry(t, q, id)
		c.advance(2 * time.Hour)
		if got := vote(t, q, id, 2, "confirm", "cellaaa", true); !got.Equal(before) {
			t.Fatalf("got %v, want unchanged %v", got, before)
		}
	})

	t.Run("the reporter's own earlier confirm in a cell does not block a stranger", func(t *testing.T) {
		q, c := newTestQuerier()
		q.BindSession("demo-1", 1)
		id := insertAt(t, q, c, baseLat, baseLon, 6*time.Hour)
		if err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{ReportID: id, AccountID: 1, Kind: "content", Value: "confirm", GeohashCell: "cellaaa"}); err != nil {
			t.Fatal(err)
		}
		_, before := storedExpiry(t, q, id)
		c.advance(time.Hour)
		if got := vote(t, q, id, 2, "confirm", "cellaaa", true); !got.After(before) {
			t.Fatalf("got %v, want later than %v", got, before)
		}
	})

	t.Run("a missing report and a full world behave as InsertVote does", func(t *testing.T) {
		q, c := newTestQuerier()
		if _, err := q.InsertVoteExtending(ctx, sqlcgen.InsertVoteExtendingParams{ReportID: 99, AccountID: 2, Kind: "content", Value: "confirm", GeohashCell: "cellaaa", Extend: true}); err == nil {
			t.Fatal("a vote on a missing report must fail")
		}
		id := insertAt(t, q, c, baseLat, baseLon, 6*time.Hour)
		q.votes = make([]memVote, MaxVotesPerWorld)
		_, err := q.InsertVoteExtending(ctx, sqlcgen.InsertVoteExtendingParams{ReportID: id, AccountID: 2, Kind: "content", Value: "confirm", GeohashCell: "cellaaa", Extend: true})
		if !errors.Is(err, ErrWorldFull) {
			t.Fatalf("a full world gave %v, want ErrWorldFull", err)
		}
	})
}
