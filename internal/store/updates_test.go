// Package store_test tests the Phase 3.2 storage: the derived followed set, the
// exactly once cursor writes, the hourly ceiling roll up, mutes and the alert
// settings (03.2-01-PLAN.md). Every test takes its instant as a fixed value
// and passes it to the statements, never reading the wall clock.
package store_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

var (
	updNow    = time.Date(2026, 10, 18, 12, 30, 0, 0, time.UTC)
	updHourAt = updNow.Truncate(time.Hour)
)

const (
	updCeiling = 10
)

// updReport seeds one report filed from sessionID that expires at expiresAt.
func updReport(t *testing.T, pool *pgxpool.Pool, sessionID string, expiresAt time.Time) int64 {
	t.Helper()
	return testutil.SeedHistoryReport(t, pool, sessionID, "medium", "flood", expiresAt)
}

// updRow inserts one updates row directly.
func updRow(t *testing.T, pool *pgxpool.Pool, accountID, reportID int64, kind string, createdAt time.Time) {
	t.Helper()
	testutil.MustExec(t, pool,
		`INSERT INTO updates (account_id, report_id, kind, created_at) VALUES ($1, $2, $3, $4)`,
		accountID, reportID, kind, createdAt)
}

// updCursor inserts one cursor row directly.
func updCursor(t *testing.T, pool *pgxpool.Pool, accountID, reportID int64, state string) {
	t.Helper()
	testutil.MustExec(t, pool,
		`INSERT INTO update_cursors (account_id, report_id, last_state, adopted_at) VALUES ($1, $2, $3, $4)`,
		accountID, reportID, state, updNow.Add(-24*time.Hour))
}

func updCount(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("counting with %q: %v", query, err)
	}
	return n
}

func advanceArg(accountID, reportID int64, oldState, newState, kind string, now time.Time) sqlcgen.AdvanceCursorAndInsertUpdateParams {
	return sqlcgen.AdvanceCursorAndInsertUpdateParams{
		NewState: newState, Now: now, AccountID: accountID, ReportID: reportID, OldState: oldState,
		GapEdge: now.Add(-time.Hour), HourEdge: now.Add(-time.Hour), Kind: kind,
		Ceiling: updCeiling, BucketStart: now.Truncate(time.Hour),
	}
}

func fadingArg(accountID, reportID int64, expiresAt, now time.Time) sqlcgen.MarkFadingAndInsertUpdateParams {
	return sqlcgen.MarkFadingAndInsertUpdateParams{
		ExpiresAt: expiresAt, Now: now, AccountID: accountID, ReportID: reportID,
		GapEdge: now.Add(-time.Hour), HourEdge: now.Add(-time.Hour),
		Ceiling: updCeiling, BucketStart: now.Truncate(time.Hour),
	}
}

func noSensitiveFields(t *testing.T, v any) {
	t.Helper()
	typ := reflect.TypeOf(v)
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		for _, bad := range []string{"SessionID", "Geohash", "Latitude", "Longitude"} {
			if strings.Contains(name, bad) {
				t.Errorf("%s has field %s, which must never be selected", typ.Name(), name)
			}
		}
	}
}

func TestFollowedOpenReports(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "follow-a@example.com", "sess-a1", "sess-a2")
	b := testutil.SeedReporterAccount(t, pool, "follow-b@example.com")
	c := testutil.SeedReporterAccount(t, pool, "follow-c@example.com")
	testutil.MustExec(t, pool, `UPDATE sessions SET revoked_at = $1 WHERE session_id = 'sess-a2'`, updNow)

	ra1 := updReport(t, pool, "sess-a1", updNow.Add(5*time.Hour))
	ra2 := updReport(t, pool, "sess-a2", updNow.Add(4*time.Hour))
	raExpired := updReport(t, pool, "sess-a1", updNow.Add(-time.Hour))
	rc := updReport(t, pool, "sess-other", updNow.Add(3*time.Hour))
	rd := updReport(t, pool, "sess-other", updNow.Add(2*time.Hour))
	rbExpired := updReport(t, pool, "sess-other", updNow.Add(-2*time.Hour))

	testutil.MustExec(t, pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, 'content', 'confirm', 'tdr1qgz')`, rc, b)
	testutil.MustExec(t, pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, 'resolution', 'unresolve', 'tdr1qgz')`, rd, c)
	testutil.MustExec(t, pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, 'content', 'confirm', 'tdr1qgz')`, rbExpired, b)
	testutil.MustExec(t, pool, `INSERT INTO report_mutes (account_id, report_id) VALUES ($1, $2)`, a, ra2)

	ids := func(rows []sqlcgen.FollowedOpenReportsRow) []int64 {
		out := make([]int64, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.ID)
		}
		return out
	}

	rowsA, err := q.FollowedOpenReports(ctx, sqlcgen.FollowedOpenReportsParams{AccountID: a, Now: updNow, RowCap: 200})
	if err != nil {
		t.Fatalf("FollowedOpenReports(a): %v", err)
	}
	// Newest first: ra1 expires later so it was created later (created_at is expiry minus 8 hours).
	if got, want := ids(rowsA), []int64{ra1, ra2}; !reflect.DeepEqual(got, want) {
		t.Errorf("account a follows %v, want %v (revoked session's post included, expired one absent, %d expired)", got, want, raExpired)
	}
	for _, r := range rowsA {
		wantMuted := r.ID == ra2
		if r.Muted != wantMuted {
			t.Errorf("report %d muted = %v, want %v", r.ID, r.Muted, wantMuted)
		}
	}

	rowsB, err := q.FollowedOpenReports(ctx, sqlcgen.FollowedOpenReportsParams{AccountID: b, Now: updNow, RowCap: 200})
	if err != nil {
		t.Fatalf("FollowedOpenReports(b): %v", err)
	}
	if got, want := ids(rowsB), []int64{rc}; !reflect.DeepEqual(got, want) {
		t.Errorf("account b follows %v, want %v (a confirm follows; the expired voted report does not)", got, want)
	}

	rowsC, err := q.FollowedOpenReports(ctx, sqlcgen.FollowedOpenReportsParams{AccountID: c, Now: updNow, RowCap: 200})
	if err != nil {
		t.Fatalf("FollowedOpenReports(c): %v", err)
	}
	if got, want := ids(rowsC), []int64{rd}; !reflect.DeepEqual(got, want) {
		t.Errorf("account c follows %v, want %v (any vote kind follows)", got, want)
	}

	capped, err := q.FollowedOpenReports(ctx, sqlcgen.FollowedOpenReportsParams{AccountID: a, Now: updNow, RowCap: 1})
	if err != nil {
		t.Fatalf("FollowedOpenReports(a, cap 1): %v", err)
	}
	if got, want := ids(capped), []int64{ra1}; !reflect.DeepEqual(got, want) {
		t.Errorf("row_cap 1 returned %v, want the newest %v", got, want)
	}

	noSensitiveFields(t, sqlcgen.FollowedOpenReportsRow{})
	noSensitiveFields(t, sqlcgen.UpdatesForAccountRow{})
	noSensitiveFields(t, sqlcgen.ReportsForUpdatesRow{})
	noSensitiveFields(t, sqlcgen.MutedReportsForAccountRow{})
	noSensitiveFields(t, sqlcgen.UpdateCursorsForReportsRow{})
}

func TestAdvanceCursorOnce(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "once@example.com", "sess-once")
	r := updReport(t, pool, "sess-once", updNow.Add(5*time.Hour))
	if err := q.InsertBaselineCursor(ctx, sqlcgen.InsertBaselineCursorParams{AccountID: a, ReportID: r, State: "provisional", Now: updNow.Add(-2 * time.Hour)}); err != nil {
		t.Fatalf("InsertBaselineCursor: %v", err)
	}

	const callers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	won, lost, failed := 0, 0, 0
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := q.AdvanceCursorAndInsertUpdate(ctx, advanceArg(a, r, "provisional", "live", "reached_live", updNow))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				won++
			case errors.Is(err, pgx.ErrNoRows):
				lost++
			default:
				failed++
				t.Errorf("AdvanceCursorAndInsertUpdate: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if won != 1 || lost != callers-1 || failed != 0 {
		t.Errorf("won %d lost %d failed %d, want exactly one winner", won, lost, failed)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1`, a); n != 1 {
		t.Errorf("%d updates rows, want 1", n)
	}
	var state string
	if err := pool.QueryRow(ctx, `SELECT last_state FROM update_cursors WHERE account_id = $1 AND report_id = $2`, a, r).Scan(&state); err != nil {
		t.Fatalf("reading cursor: %v", err)
	}
	if state != "live" {
		t.Errorf("cursor = %q, want live", state)
	}

	// A second state change inside the hour is refused and moves nothing.
	_, err := q.AdvanceCursorAndInsertUpdate(ctx, advanceArg(a, r, "live", "hidden", "hidden", updNow.Add(10*time.Minute)))
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("second change inside the hour: want pgx.ErrNoRows, got %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT last_state FROM update_cursors WHERE account_id = $1 AND report_id = $2`, a, r).Scan(&state); err != nil {
		t.Fatalf("reading cursor: %v", err)
	}
	if state != "live" {
		t.Errorf("cursor after refused change = %q, want live", state)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1`, a); n != 1 {
		t.Errorf("%d updates rows after refused change, want 1", n)
	}

	// A wrong old state also changes nothing.
	_, err = q.AdvanceCursorAndInsertUpdate(ctx, advanceArg(a, r, "provisional", "hidden", "hidden", updNow.Add(3*time.Hour)))
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("stale old state: want pgx.ErrNoRows, got %v", err)
	}
}

func TestUpdatesCeilingRollsUp(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "ceiling@example.com", "sess-ceiling")
	r0 := updReport(t, pool, "sess-ceiling", updNow.Add(5*time.Hour))
	r1 := updReport(t, pool, "sess-ceiling", updNow.Add(5*time.Hour))
	r2 := updReport(t, pool, "sess-ceiling", updNow.Add(5*time.Hour))
	for i := 0; i < updCeiling; i++ {
		updRow(t, pool, a, r0, "hidden", updNow.Add(-time.Duration(i+1)*time.Minute))
	}
	updCursor(t, pool, a, r1, "provisional")
	updCursor(t, pool, a, r2, "provisional")

	if _, err := q.AdvanceCursorAndInsertUpdate(ctx, advanceArg(a, r1, "provisional", "live", "reached_live", updNow)); err != nil {
		t.Fatalf("event over the ceiling: %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND kind <> 'more'`, a); n != updCeiling {
		t.Errorf("%d normal rows, want still %d", n, updCeiling)
	}
	var moreCount int
	var bucket time.Time
	if err := pool.QueryRow(ctx, `SELECT more_count, bucket_start FROM updates WHERE account_id = $1 AND kind = 'more'`, a).Scan(&moreCount, &bucket); err != nil {
		t.Fatalf("reading the rolled up row: %v", err)
	}
	if moreCount != 1 || !bucket.Equal(updHourAt) {
		t.Errorf("rolled up row = count %d bucket %v, want 1 and %v", moreCount, bucket, updHourAt)
	}
	var state string
	if err := pool.QueryRow(ctx, `SELECT last_state FROM update_cursors WHERE account_id = $1 AND report_id = $2`, a, r1).Scan(&state); err != nil {
		t.Fatalf("reading cursor: %v", err)
	}
	if state != "live" {
		t.Errorf("cursor = %q, want live (a blocked event is never announced later)", state)
	}

	// The reader looks at the bell, then a second blocked event arrives.
	testutil.MustExec(t, pool, `UPDATE updates SET read_at = $2 WHERE account_id = $1 AND kind = 'more'`, a, updNow)
	if _, err := q.AdvanceCursorAndInsertUpdate(ctx, advanceArg(a, r2, "provisional", "live", "reached_live", updNow.Add(time.Minute))); err != nil {
		t.Fatalf("second event over the ceiling: %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND kind = 'more'`, a); n != 1 {
		t.Errorf("%d rolled up rows, want one per account per bucket", n)
	}
	var readAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT more_count, read_at FROM updates WHERE account_id = $1 AND kind = 'more'`, a).Scan(&moreCount, &readAt); err != nil {
		t.Fatalf("reading the rolled up row: %v", err)
	}
	if moreCount != 2 || readAt != nil {
		t.Errorf("rolled up row = count %d read_at %v, want 2 and unread", moreCount, readAt)
	}
}

func TestFadingOncePerExpiry(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "fading@example.com", "sess-fading")
	r := updReport(t, pool, "sess-fading", updNow.Add(30*time.Minute))
	updCursor(t, pool, a, r, "live")

	e1 := updNow.Add(30 * time.Minute)
	if _, err := q.MarkFadingAndInsertUpdate(ctx, fadingArg(a, r, e1, updNow)); err != nil {
		t.Fatalf("first fading: %v", err)
	}
	var fadingFor *time.Time
	if err := pool.QueryRow(ctx, `SELECT fading_for FROM update_cursors WHERE account_id = $1 AND report_id = $2`, a, r).Scan(&fadingFor); err != nil {
		t.Fatalf("reading cursor: %v", err)
	}
	if fadingFor == nil || !fadingFor.Equal(e1) {
		t.Errorf("fading_for = %v, want %v", fadingFor, e1)
	}

	if _, err := q.MarkFadingAndInsertUpdate(ctx, fadingArg(a, r, e1, updNow.Add(2*time.Hour))); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("same expiry again: want pgx.ErrNoRows, got %v", err)
	}
	e2 := updNow.Add(4 * time.Hour)
	if _, err := q.MarkFadingAndInsertUpdate(ctx, fadingArg(a, r, e2, updNow.Add(time.Minute))); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("changed expiry inside the hour: want pgx.ErrNoRows, got %v", err)
	}
	if _, err := q.MarkFadingAndInsertUpdate(ctx, fadingArg(a, r, e2, updNow.Add(3*time.Hour+30*time.Minute))); err != nil {
		t.Errorf("changed expiry after the gap: %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND kind = 'fading'`, a); n != 2 {
		t.Errorf("%d fading rows, want 2", n)
	}
}

func TestReliableAlertOncePerReport(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	recipient := testutil.SeedReporterAccount(t, pool, "alert-recipient@example.com")
	x := testutil.SeedReporterAccount(t, pool, "alert-x@example.com", "sess-x")
	y := testutil.SeedReporterAccount(t, pool, "alert-y@example.com", "sess-y")
	rx1 := updReport(t, pool, "sess-x", updNow.Add(5*time.Hour))
	rx2 := updReport(t, pool, "sess-x", updNow.Add(5*time.Hour))
	ry := updReport(t, pool, "sess-y", updNow.Add(5*time.Hour))

	arg := func(report int64, now time.Time) sqlcgen.InsertReliableAlertParams {
		return sqlcgen.InsertReliableAlertParams{
			AccountID: recipient, ReportID: report, Now: now, HourEdge: now.Add(-time.Hour), Ceiling: updCeiling,
		}
	}
	if _, err := q.InsertReliableAlert(ctx, arg(rx1, updNow)); err != nil {
		t.Fatalf("first alert: %v", err)
	}
	if _, err := q.InsertReliableAlert(ctx, arg(rx1, updNow.Add(time.Minute))); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("same report again: want pgx.ErrNoRows, got %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND kind = 'reliable_nearby'`, recipient); n != 1 {
		t.Errorf("%d alert rows, want 1", n)
	}

	// The per reporter count reads only that reporter's reports inside the window.
	testutil.MustExec(t, pool, `UPDATE updates SET created_at = $2 WHERE account_id = $1`, recipient, updNow.Add(-30*time.Minute))
	updRow(t, pool, recipient, rx2, "reliable_nearby", updNow.Add(-3*time.Hour))
	updRow(t, pool, recipient, ry, "reliable_nearby", updNow.Add(-10*time.Minute))
	count := func(reporter int64, window time.Time) int64 {
		n, err := q.AlertsFromReporterInWindow(ctx, sqlcgen.AlertsFromReporterInWindowParams{
			RecipientID: recipient, ReporterID: reporter, WindowStart: window,
		})
		if err != nil {
			t.Fatalf("AlertsFromReporterInWindow: %v", err)
		}
		return n
	}
	if got := count(x, updNow.Add(-time.Hour)); got != 1 {
		t.Errorf("reporter x in the last hour = %d, want 1", got)
	}
	if got := count(y, updNow.Add(-time.Hour)); got != 1 {
		t.Errorf("reporter y in the last hour = %d, want 1", got)
	}
	if got := count(x, updNow.Add(-6*time.Hour)); got != 2 {
		t.Errorf("reporter x in the last 6 hours = %d, want 2", got)
	}

	// Over the hourly ceiling nothing is written.
	full := testutil.SeedReporterAccount(t, pool, "alert-full@example.com")
	rf := updReport(t, pool, "sess-x", updNow.Add(5*time.Hour))
	for i := 0; i < updCeiling; i++ {
		updRow(t, pool, full, rf, "hidden", updNow.Add(-time.Duration(i+1)*time.Minute))
	}
	over := arg(ry, updNow)
	over.AccountID = full
	if _, err := q.InsertReliableAlert(ctx, over); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("alert over the ceiling: want pgx.ErrNoRows, got %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND kind = 'reliable_nearby'`, full); n != 0 {
		t.Errorf("%d alert rows over the ceiling, want 0", n)
	}
}

func TestMarkUpdatesReadScoped(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "read-a@example.com", "sess-read")
	b := testutil.SeedReporterAccount(t, pool, "read-b@example.com")
	r := updReport(t, pool, "sess-read", updNow.Add(5*time.Hour))
	for i := 0; i < 3; i++ {
		updRow(t, pool, a, r, "hidden", updNow.Add(-time.Duration(3-i)*time.Minute))
		updRow(t, pool, b, r, "hidden", updNow.Add(-time.Duration(3-i)*time.Minute))
	}
	var aIDs []int64
	rows, err := pool.Query(ctx, `SELECT id FROM updates WHERE account_id = $1 ORDER BY id`, a)
	if err != nil {
		t.Fatalf("listing ids: %v", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scanning id: %v", err)
		}
		aIDs = append(aIDs, id)
	}
	rows.Close()
	if len(aIDs) != 3 {
		t.Fatalf("seeded %d rows for a, want 3", len(aIDs))
	}

	n, err := q.MarkUpdatesRead(ctx, sqlcgen.MarkUpdatesReadParams{Now: updNow, AccountID: a, UpTo: aIDs[1]})
	if err != nil {
		t.Fatalf("MarkUpdatesRead: %v", err)
	}
	if n != 2 {
		t.Errorf("affected %d rows, want 2", n)
	}
	if got := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND read_at IS NOT NULL`, a); got != 2 {
		t.Errorf("%d of a's rows read, want 2", got)
	}
	if got := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND read_at IS NOT NULL`, b); got != 0 {
		t.Errorf("%d of b's rows read, want 0 (another account's rows are never touched)", got)
	}
	again, err := q.MarkUpdatesRead(ctx, sqlcgen.MarkUpdatesReadParams{Now: updNow, AccountID: a, UpTo: aIDs[1]})
	if err != nil || again != 0 {
		t.Errorf("repeat marked %d rows (err %v), want 0", again, err)
	}
}

func TestMuteClearsUnreadAndUnmuteDropsCursor(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "mute-a@example.com", "sess-mute")
	b := testutil.SeedReporterAccount(t, pool, "mute-b@example.com")
	r := updReport(t, pool, "sess-mute", updNow.Add(5*time.Hour))
	other := updReport(t, pool, "sess-mute", updNow.Add(5*time.Hour))
	updRow(t, pool, a, r, "reached_live", updNow.Add(-2*time.Hour))
	updRow(t, pool, a, r, "hidden", updNow.Add(-time.Hour))
	updRow(t, pool, a, other, "hidden", updNow.Add(-time.Hour))
	testutil.MustExec(t, pool, `UPDATE updates SET read_at = $1 WHERE account_id = $2 AND kind = 'reached_live'`, updNow, a)
	updCursor(t, pool, a, r, "live")

	arg := sqlcgen.MuteReportParams{AccountID: a, ReportID: r}
	if err := q.MuteReport(ctx, arg); err != nil {
		t.Fatalf("MuteReport: %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND report_id = $2`, a, r); n != 1 {
		t.Errorf("%d updates left for the muted report, want only the read one", n)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND report_id = $2`, a, other); n != 1 {
		t.Errorf("other report's update count = %d, want 1", n)
	}
	if err := q.MuteReport(ctx, arg); err != nil {
		t.Errorf("second MuteReport must be idempotent: %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM report_mutes WHERE account_id = $1 AND report_id = $2`, a, r); n != 1 {
		t.Errorf("%d mute rows, want 1", n)
	}

	for viewer, want := range map[int64]bool{a: true, b: false} {
		extras, err := q.ReportExtrasForReports(ctx, sqlcgen.ReportExtrasForReportsParams{ViewerAccountID: viewer, ReportIds: []int64{r, other}})
		if err != nil {
			t.Fatalf("ReportExtrasForReports: %v", err)
		}
		if len(extras) != 2 {
			t.Fatalf("got %d extras rows, want 2 (a mute never duplicates a report)", len(extras))
		}
		for _, e := range extras {
			if wantRow := want && e.ReportID == r; e.ViewerMuted != wantRow {
				t.Errorf("viewer %d report %d viewer_muted = %v, want %v", viewer, e.ReportID, e.ViewerMuted, wantRow)
			}
		}
	}

	un := sqlcgen.UnmuteReportParams{AccountID: a, ReportID: r}
	if err := q.UnmuteReport(ctx, un); err != nil {
		t.Fatalf("UnmuteReport: %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM report_mutes WHERE account_id = $1`, a); n != 0 {
		t.Errorf("%d mute rows after unmute, want 0", n)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM update_cursors WHERE account_id = $1 AND report_id = $2`, a, r); n != 0 {
		t.Errorf("%d cursor rows after unmute, want 0 (the next refresh starts a silent baseline)", n)
	}
	if err := q.UnmuteReport(ctx, un); err != nil {
		t.Errorf("second UnmuteReport must be idempotent: %v", err)
	}
}

// TestReportExtrasViewerMuted covers the mute flag on its own: it is the
// viewer's own mute only, and a report muted by someone else reads false.
func TestReportExtrasViewerMuted(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "extras-a@example.com", "sess-extras")
	b := testutil.SeedReporterAccount(t, pool, "extras-b@example.com")
	r := updReport(t, pool, "sess-extras", updNow.Add(5*time.Hour))
	testutil.MustExec(t, pool, `INSERT INTO report_mutes (account_id, report_id) VALUES ($1, $2)`, b, r)

	for viewer, want := range map[int64]bool{a: false, b: true} {
		extras, err := q.ReportExtrasForReports(ctx, sqlcgen.ReportExtrasForReportsParams{ViewerAccountID: viewer, ReportIds: []int64{r}})
		if err != nil {
			t.Fatalf("ReportExtrasForReports: %v", err)
		}
		if len(extras) != 1 || extras[0].ViewerMuted != want {
			t.Errorf("viewer %d: %+v, want one row with viewer_muted %v", viewer, extras, want)
		}
	}

	muted, err := q.MutedReportsForAccount(ctx, sqlcgen.MutedReportsForAccountParams{AccountID: b, RowLimit: 10})
	if err != nil || len(muted) != 1 || muted[0].ReportID != r {
		t.Errorf("MutedReportsForAccount = %+v (err %v), want the one muted report", muted, err)
	}
	if n, err := q.CountMutedReports(ctx, b); err != nil || n != 1 {
		t.Errorf("CountMutedReports = %d (err %v), want 1", n, err)
	}
	if n, err := q.CountMutedReports(ctx, a); err != nil || n != 0 {
		t.Errorf("CountMutedReports for a = %d (err %v), want 0", n, err)
	}
}

func TestAlertSettingsRadiusAndStamp(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "settings@example.com")
	if _, err := q.GetAlertSettings(ctx, a); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetAlertSettings with no row: want pgx.ErrNoRows, got %v", err)
	}

	t1 := updNow
	got, err := q.UpsertAlertSettings(ctx, sqlcgen.UpsertAlertSettingsParams{AccountID: a, ReliableAlertsOn: true, AlertRadiusKm: 2, Now: t1})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !got.ReliableAlertsOn || got.AlertRadiusKm != 2 || !got.EnabledAt.Valid || !got.EnabledAt.Time.Equal(t1) {
		t.Errorf("first upsert = %+v, want on, 2 km, enabled_at %v", got, t1)
	}

	t2 := updNow.Add(time.Hour)
	got, err = q.UpsertAlertSettings(ctx, sqlcgen.UpsertAlertSettingsParams{AccountID: a, ReliableAlertsOn: true, AlertRadiusKm: 10, Now: t2})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if got.AlertRadiusKm != 10 || !got.EnabledAt.Valid || !got.EnabledAt.Time.Equal(t1) {
		t.Errorf("second upsert = %+v, want 10 km and the first enabled_at %v", got, t1)
	}

	got, err = q.UpsertAlertSettings(ctx, sqlcgen.UpsertAlertSettingsParams{AccountID: a, ReliableAlertsOn: false, AlertRadiusKm: 10, Now: t2})
	if err != nil {
		t.Fatalf("off upsert: %v", err)
	}
	if got.ReliableAlertsOn || got.EnabledAt.Valid {
		t.Errorf("off upsert = %+v, want off and enabled_at NULL", got)
	}

	t3 := updNow.Add(2 * time.Hour)
	got, err = q.UpsertAlertSettings(ctx, sqlcgen.UpsertAlertSettingsParams{AccountID: a, ReliableAlertsOn: true, AlertRadiusKm: 5, Now: t3})
	if err != nil {
		t.Fatalf("on again upsert: %v", err)
	}
	if !got.EnabledAt.Valid || !got.EnabledAt.Time.Equal(t3) {
		t.Errorf("on again = %+v, want a fresh enabled_at %v", got, t3)
	}

	_, err = q.UpsertAlertSettings(ctx, sqlcgen.UpsertAlertSettingsParams{AccountID: a, ReliableAlertsOn: true, AlertRadiusKm: 3, Now: t3})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Errorf("radius 3: want SQLSTATE 23514, got %v", err)
	}

	if err := q.DisableReliableAlerts(ctx, sqlcgen.DisableReliableAlertsParams{Now: t3, AccountID: a}); err != nil {
		t.Fatalf("DisableReliableAlerts: %v", err)
	}
	read, err := q.GetAlertSettings(ctx, a)
	if err != nil {
		t.Fatalf("GetAlertSettings: %v", err)
	}
	if read.ReliableAlertsOn || read.EnabledAt.Valid || read.AlertRadiusKm != 5 {
		t.Errorf("after disable = %+v, want off, enabled_at NULL and the radius kept", read)
	}
}

func TestUnreadCountCapped(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "unread@example.com", "sess-unread")
	r := updReport(t, pool, "sess-unread", updNow.Add(5*time.Hour))
	testutil.MustExec(t, pool,
		`INSERT INTO updates (account_id, report_id, kind, created_at)
		 SELECT $1, $2, 'hidden', $3::timestamptz - (g * interval '1 second') FROM generate_series(1, 120) AS g`,
		a, r, updNow)

	since := updNow.Add(-24 * time.Hour)
	n, err := q.UnreadUpdateCount(ctx, sqlcgen.UnreadUpdateCountParams{AccountID: a, Since: since, Cap: 99})
	if err != nil {
		t.Fatalf("UnreadUpdateCount: %v", err)
	}
	if n != 99 {
		t.Errorf("unread count = %d, want 99 (capped)", n)
	}
	n, err = q.UnreadUpdateCount(ctx, sqlcgen.UnreadUpdateCountParams{AccountID: a, Since: since, Cap: 500})
	if err != nil || n != 120 {
		t.Errorf("unread count with a high cap = %d (err %v), want 120", n, err)
	}
	total, err := q.CountUpdatesForAccount(ctx, sqlcgen.CountUpdatesForAccountParams{AccountID: a, Since: since})
	if err != nil || total != 120 {
		t.Errorf("CountUpdatesForAccount = %d (err %v), want 120", total, err)
	}
	list, err := q.UpdatesForAccount(ctx, sqlcgen.UpdatesForAccountParams{AccountID: a, Since: since, RowLimit: 50})
	if err != nil || len(list) != 50 {
		t.Fatalf("UpdatesForAccount = %d rows (err %v), want 50", len(list), err)
	}
	if list[0].CreatedAt.Before(list[1].CreatedAt) {
		t.Errorf("UpdatesForAccount is not newest first")
	}
}

func TestPruneUpdatesAndCursors(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "prune-a@example.com", "sess-prune")
	b := testutil.SeedReporterAccount(t, pool, "prune-b@example.com")
	edge := updNow.Add(-30 * 24 * time.Hour)
	open := updReport(t, pool, "sess-prune", updNow.Add(5*time.Hour))
	oldReport := updReport(t, pool, "sess-prune", edge.Add(-time.Hour))

	updRow(t, pool, a, open, "hidden", edge.Add(-time.Hour))
	updRow(t, pool, a, open, "hidden", edge.Add(time.Hour))
	updRow(t, pool, b, open, "hidden", edge.Add(-time.Hour))
	updCursor(t, pool, a, open, "live")
	updCursor(t, pool, a, oldReport, "live")
	updCursor(t, pool, b, oldReport, "live")

	if err := q.PruneUpdates(ctx, sqlcgen.PruneUpdatesParams{AccountID: a, Edge: edge}); err != nil {
		t.Fatalf("PruneUpdates: %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1`, a); n != 1 {
		t.Errorf("%d of a's updates left, want the one newer than the edge", n)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1`, b); n != 1 {
		t.Errorf("%d of b's updates left, want 1 (another account is untouched)", n)
	}

	if err := q.PruneCursors(ctx, sqlcgen.PruneCursorsParams{AccountID: a, Edge: edge}); err != nil {
		t.Fatalf("PruneCursors: %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM update_cursors WHERE account_id = $1`, a); n != 1 {
		t.Errorf("%d of a's cursors left, want the one on the open report", n)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM update_cursors WHERE account_id = $1`, b); n != 1 {
		t.Errorf("%d of b's cursors left, want 1", n)
	}

	cursors, err := q.UpdateCursorsForReports(ctx, sqlcgen.UpdateCursorsForReportsParams{AccountID: a, ReportIds: []int64{open, oldReport}})
	if err != nil || len(cursors) != 1 || cursors[0].ReportID != open {
		t.Errorf("UpdateCursorsForReports = %+v (err %v), want the one on the open report", cursors, err)
	}
	adopted, err := q.AdoptCursorState(ctx, sqlcgen.AdoptCursorStateParams{NewState: "hidden", Now: updNow, AccountID: a, ReportID: open, OldState: "live"})
	if err != nil || adopted != 1 {
		t.Errorf("AdoptCursorState = %d (err %v), want 1", adopted, err)
	}
	adopted, err = q.AdoptCursorState(ctx, sqlcgen.AdoptCursorStateParams{NewState: "hidden", Now: updNow, AccountID: a, ReportID: open, OldState: "live"})
	if err != nil || adopted != 0 {
		t.Errorf("AdoptCursorState on a stale old state = %d (err %v), want 0", adopted, err)
	}
	reports, err := q.ReportsForUpdates(ctx, []int64{open, oldReport})
	if err != nil || len(reports) != 2 {
		t.Errorf("ReportsForUpdates = %d rows (err %v), want 2 (any expiry)", len(reports), err)
	}
}

func TestUpdatesForAccountUsesIndex(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	a := testutil.SeedReporterAccount(t, pool, "plan@example.com", "sess-plan")
	r := updReport(t, pool, "sess-plan", updNow.Add(5*time.Hour))
	for i := 0; i < 5; i++ {
		updRow(t, pool, a, r, "hidden", updNow.Add(-time.Duration(i+1)*time.Minute))
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
		t.Fatalf("SET LOCAL: %v", err)
	}
	explain := fmt.Sprintf(`EXPLAIN SELECT id, report_id, kind, created_at, read_at, more_count
		FROM updates
		WHERE account_id = %d AND created_at > '%s'::timestamptz
		ORDER BY created_at DESC, id DESC
		LIMIT 50`, a, updNow.Add(-24*time.Hour).Format(time.RFC3339))
	rows, err := tx.Query(ctx, explain)
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scanning plan: %v", err)
		}
		plan.WriteString(line + "\n")
	}
	rows.Close()
	if !strings.Contains(plan.String(), "idx_updates_account_created") {
		t.Errorf("plan does not use idx_updates_account_created:\n%s", plan.String())
	}
}
