// pushdispatch_db_test.go runs the phone alert dispatcher (plan 03.3-09, task 3)
// against a real database with the real report and updates services: who is
// queued for a Critical report, that the resolver gates it, that the Reliable
// rows are 3.2's own, that a race with the bell gives one change push, and that
// the caps and the re run rules hold. Every test starts with
// testutil.NewTestDB, so it skips in the quick run.

package service

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// pushDBRig is one test's world: the real services on one clock the test moves.
// The clock starts two hours before wall time, so every step stays before the
// database's own now() (the same pattern as TestReliableAlertsOnRealHistory).
type pushDBRig struct {
	t       *testing.T
	pool    *pgxpool.Pool
	q       *sqlcgen.Queries
	base    time.Time
	cur     time.Time
	updates *UpdatesService
	namer   *fakeAreaNamer
	d       *PushDispatcher

	// The report point is the middle of the home cell numbered 0.
	homeCell0 string
	lat, lon  float64
	seq       int
}

func newPushDBRig(t *testing.T) *pushDBRig {
	t.Helper()
	pool := testutil.NewTestDB(t)
	r := &pushDBRig{t: t, pool: pool, q: sqlcgen.New(pool)}
	r.base = time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	r.cur = r.base
	clock := func() time.Time { return r.cur }

	reports := NewReportService(r.q, WithReportClock(clock))
	r.updates = NewUpdatesService(r.q, reports, WithUpdatesClock(clock))
	r.namer = &fakeAreaNamer{name: "Central Market"}
	r.d = NewPushDispatcher(r.q, r.updates, r.namer, WithDispatchClock(clock))

	r.homeCell0 = geohash.EncodeWithPrecision(12.9716, 77.5946, 5)
	lat, lon, ok := HomeAreaAnchor(r.homeCell0)
	if !ok {
		t.Fatalf("home cell %q does not decode", r.homeCell0)
	}
	r.lat, r.lon = lat, lon
	return r
}

// cell returns the home cell k cell widths east of cell 0 (about 4.75 km each).
func (r *pushDBRig) cell(k int) string {
	return geohash.EncodeWithPrecision(r.lat, r.lon+float64(k)*0.0439, 5)
}

// distKm is the distance from the report point to the middle of a home cell.
func (r *pushDBRig) distKm(cell string) float64 {
	lat, lon, ok := HomeAreaAnchor(cell)
	if !ok {
		r.t.Fatalf("cell %q does not decode", cell)
	}
	return haversineKm(r.lat, r.lon, lat, lon)
}

func (r *pushDBRig) email(prefix string) string {
	r.seq++
	return fmt.Sprintf("p33-09-%s-%d@example.com", prefix, r.seq)
}

// reader makes an account with a session, a home cell, a device, and the given
// settings row (radius 0 means no settings row, so the defaults apply).
func (r *pushDBRig) reader(prefix string, cellK int, radius int, critical, reliable, changes bool) (int64, string) {
	r.t.Helper()
	session := fmt.Sprintf("p33-09-sess-%s-%d", prefix, r.seq+1)
	id := testutil.SeedReporterAccount(r.t, r.pool, r.email(prefix), session)
	testutil.SetHomeCell(r.t, r.pool, id, "Home", r.cell(cellK))
	testutil.SeedPushSubscription(r.t, r.pool, id, session, "")
	if radius != 0 {
		r.setSettings(id, radius, critical, reliable, changes)
	}
	return id, session
}

func (r *pushDBRig) setSettings(id int64, radius int, critical, reliable, changes bool) {
	r.t.Helper()
	enabled := r.base.Add(-3 * time.Hour)
	var reliableAt, changesAt *time.Time
	if reliable {
		reliableAt = &enabled
	}
	if changes {
		changesAt = &enabled
	}
	testutil.MustExec(r.t, r.pool,
		`INSERT INTO account_alert_settings
			(account_id, reliable_alerts_on, alert_radius_km, enabled_at, critical_push_on, changes_push_on, changes_enabled_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (account_id) DO UPDATE SET
			reliable_alerts_on = EXCLUDED.reliable_alerts_on, alert_radius_km = EXCLUDED.alert_radius_km,
			enabled_at = EXCLUDED.enabled_at, critical_push_on = EXCLUDED.critical_push_on,
			changes_push_on = EXCLUDED.changes_push_on, changes_enabled_at = EXCLUDED.changes_enabled_at,
			updated_at = EXCLUDED.updated_at`,
		id, reliable, radius, reliableAt, critical, changes, changesAt, r.base)
}

// report stores one report at the report point, written at createdAt, and
// returns its id.
func (r *pushDBRig) report(session string, severity Severity, createdAt time.Time, moderation string) int64 {
	r.t.Helper()
	return testutil.InsertStoredReport(r.t, r.pool, testutil.StoredReport{
		SessionID:       session,
		Category:        "flood",
		Severity:        string(severity),
		Description:     "p33-09 seeded report",
		Latitude:        r.lat,
		Longitude:       r.lon,
		CreatedAt:       createdAt,
		ExpiresAt:       createdAt.Add(ExpiryDuration(severity)),
		ModerationState: moderation,
	})
}

func (r *pushDBRig) vote(reportID, account int64, kind, value, cell string, at time.Time) {
	r.t.Helper()
	testutil.MustExec(r.t, r.pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		reportID, account, kind, value, cell, at)
}

func (r *pushDBRig) eval(reportID, actor int64) {
	r.t.Helper()
	if err := r.d.EvaluateReport(context.Background(), reportID, actor); err != nil {
		r.t.Fatalf("EvaluateReport(%d, %d): %v", reportID, actor, err)
	}
}

// outbox returns the kinds queued for a report, by account id, in id order of
// the rows (an account can hold a first alert and a later row of another kind,
// so the value is a sorted list).
func (r *pushDBRig) outbox(reportID int64) map[int64][]string {
	r.t.Helper()
	rows, err := r.pool.Query(context.Background(),
		`SELECT account_id, kind FROM push_outbox WHERE report_id = $1 ORDER BY id`, reportID)
	if err != nil {
		r.t.Fatalf("reading the outbox: %v", err)
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var acct int64
		var kind string
		if err := rows.Scan(&acct, &kind); err != nil {
			r.t.Fatalf("scanning the outbox: %v", err)
		}
		out[acct] = append(out[acct], kind)
	}
	if err := rows.Err(); err != nil {
		r.t.Fatalf("reading the outbox: %v", err)
	}
	for _, kinds := range out {
		sort.Strings(kinds)
	}
	return out
}

func (r *pushDBRig) count(sql string, args ...any) int {
	r.t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		r.t.Fatalf("counting: %v", err)
	}
	return n
}

func (r *pushDBRig) progress(reportID int64) (critical, reliable string) {
	r.t.Helper()
	p, err := r.q.GetPushReportProgress(context.Background(), reportID)
	if err != nil {
		r.t.Fatalf("reading progress of report %d: %v", reportID, err)
	}
	if p.CriticalState != nil {
		critical = *p.CriticalState
	}
	if p.ReliableState != nil {
		reliable = *p.ReliableState
	}
	return critical, reliable
}

func outboxKindsEqual(got []string, want ...string) bool {
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

func TestPushRecipients(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	// The reporter is an author with a home cell and a device of their own.
	const authorSession = "p33-09-author-session"
	author := testutil.SeedReporterAccount(t, r.pool, r.email("author"), authorSession)
	testutil.SetHomeCell(t, r.pool, author, "Home", r.cell(0))
	testutil.SeedPushSubscription(t, r.pool, author, authorSession, "")

	type reader struct {
		name   string
		id     int64
		cell   int
		radius int // 0 means no settings row: 5 km and Critical on
	}
	var readers []reader
	add := func(name string, cellK, radius int, critical bool) {
		id, _ := r.reader(name, cellK, radius, critical, false, false)
		readers = append(readers, reader{name: name, id: id, cell: cellK, radius: radius})
	}
	add("here-1km", 0, 1, true)
	add("next-1km", 1, 1, true)
	add("next-5km", 1, 5, true)
	add("next-default", 1, 0, true)
	add("two-10km", 2, 10, true)
	add("two-5km", 2, 5, true)
	add("here-off", 0, 5, false)
	add("far-10km", 3, 10, true)

	// Readers who would be in range but must get nothing.
	mutedID, _ := r.reader("muted", 0, 5, true, false, false)
	deletedID, _ := r.reader("deleted", 0, 5, true, false, false)
	revokedID, revokedSession := r.reader("revoked", 0, 5, true, false, false)
	testutil.MustExec(t, r.pool, "UPDATE accounts SET deleted_at = $2 WHERE id = $1", deletedID, r.base)
	testutil.MustExec(t, r.pool, "UPDATE sessions SET revoked_at = $2 WHERE session_id = $1", revokedSession, r.base)
	noDevice := testutil.SeedReporterAccount(t, r.pool, r.email("nodevice"), "p33-09-nodevice-session")
	testutil.SetHomeCell(t, r.pool, noDevice, "Home", r.cell(0))

	r.cur = r.base.Add(10 * time.Minute)
	reportID := r.report(authorSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
	testutil.MustExec(t, r.pool, "INSERT INTO report_mutes (account_id, report_id) VALUES ($1, $2)", mutedID, reportID)

	r.eval(reportID, 0)
	got := r.outbox(reportID)

	// Expected from the stated rule: the distance from the middle of the home
	// cell to the report is at most the account's own distance.
	inRange, outOfRange := 0, 0
	for _, rd := range readers {
		radius := rd.radius
		if radius == 0 {
			radius = DefaultAlertRadiusKm
		}
		// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications
		// box): the stored Critical switch of here-off no longer decides, so it is
		// queued like the others.
		critical := true
		want := critical && r.distKm(r.cell(rd.cell)) <= float64(radius)
		has := outboxKindsEqual(got[rd.id], "critical")
		if want {
			inRange++
		} else {
			outOfRange++
		}
		if want != has {
			t.Errorf("%s: critical row = %v, want %v (home %.2f km away, distance %d km)",
				rd.name, has, want, r.distKm(r.cell(rd.cell)), radius)
		}
		if len(got[rd.id]) > 1 {
			t.Errorf("%s holds %v, want at most one row", rd.name, got[rd.id])
		}
	}
	if inRange < 3 || outOfRange < 3 {
		t.Fatalf("the fixture is not meaningful: %d in range, %d out", inRange, outOfRange)
	}
	for name, id := range map[string]int64{
		"the author": author, "a muted reader": mutedID, "a deleted account": deletedID,
		"a revoked session": revokedID, "an account with no device": noDevice,
	} {
		if len(got[id]) != 0 {
			t.Errorf("%s was queued %v", name, got[id])
		}
	}
	if c, _ := r.progress(reportID); c != "fanned" {
		t.Errorf("critical_state = %q, want fanned", c)
	}

	// The area name was looked up once, stored, and read by no feed query.
	if r.namer.calls != 1 {
		t.Errorf("area lookups = %d, want 1", r.namer.calls)
	}
	var area *string
	if err := r.pool.QueryRow(ctx, "SELECT area_name FROM reports WHERE id = $1", reportID).Scan(&area); err != nil {
		t.Fatalf("reading area name: %v", err)
	}
	if area == nil || *area != "Central Market" {
		t.Errorf("stored area name = %v", area)
	}
}

func TestPushDispatchResolver(t *testing.T) {
	r := newPushDBRig(t)

	const authorSession = "p33-09-resolver-author"
	testutil.SeedReporterAccount(t, r.pool, r.email("author"), authorSession)
	reader, _ := r.reader("resolver", 0, 5, true, false, false)

	r.cur = r.base.Add(10 * time.Minute)
	at := r.base.Add(5 * time.Minute)

	live := r.report(authorSession, SeverityCritical, at, "clear")
	removed := r.report(authorSession, SeverityCritical, at, "removed")
	pendingCritical := r.report(authorSession, SeverityCritical, at, "pending")
	pendingLow := r.report(authorSession, SeverityLow, at, "pending")
	for _, id := range []int64{live, removed, pendingCritical, pendingLow} {
		r.eval(id, 0)
	}

	if !outboxKindsEqual(r.outbox(live)[reader], "critical") {
		t.Errorf("a listed critical report queued %v", r.outbox(live)[reader])
	}
	if got := r.outbox(removed)[reader]; len(got) != 0 {
		t.Errorf("a removed critical report queued %v", got)
	}
	if !outboxKindsEqual(r.outbox(pendingCritical)[reader], "critical") {
		t.Errorf("a pending critical report queued %v: the lock screen carries none of the reporter's words", r.outbox(pendingCritical)[reader])
	}
	if got := r.outbox(pendingLow)[reader]; len(got) != 0 {
		t.Errorf("a pending low report queued %v", got)
	}

	// A report removed after it fanned out: a reader who registers a device
	// afterwards gets nothing, because the resolver no longer shows it.
	testutil.MustExec(t, r.pool, "UPDATE reports SET moderation_state = 'removed' WHERE id = $1", live)
	late, _ := r.reader("late", 0, 5, true, false, false)
	r.eval(live, 0)
	if got := r.outbox(live)[late]; len(got) != 0 {
		t.Errorf("a report removed after posting still queued %v for a late reader", got)
	}
}

func TestPushReliable(t *testing.T) {
	r := newPushDBRig(t)

	const reporterSession = "p33-09-reliable-reporter"
	reporter := testutil.SeedReliableReporter(t, r.pool, r.email("reliable"), reporterSession, r.base)
	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box):
	// a Reliable reporter note never reaches a phone, whatever the stored flags
	// say. The dispatcher only refreshes the bell, so every subscribed reader in
	// range holds the reliable_nearby bell row and no reader holds a reliable
	// phone row. R1: Critical off, Reliable on. R2: both on. R3: Reliable off.
	r1, _ := r.reader("r1", 0, 5, false, true, false)
	r2, _ := r.reader("r2", 0, 5, true, true, false)
	r3, _ := r.reader("r3", 0, 5, true, false, false)
	_ = reporter

	r.cur = r.base.Add(10 * time.Minute)
	low := r.report(reporterSession, SeverityLow, r.base.Add(5*time.Minute), "clear")
	r.eval(low, 0)

	got := r.outbox(low)
	for name, id := range map[string]int64{"R1": r1, "R2": r2, "R3": r3} {
		if len(got[id]) != 0 {
			t.Errorf("%s holds %v, want no phone row for a Reliable report", name, got[id])
		}
		// The early refresh wrote 3.2's own reliable_nearby row for every reader.
		if n := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reliable_nearby'`, id, low); n != 1 {
			t.Errorf("3.2 reliable_nearby rows for %s = %d, want 1", name, n)
		}
	}

	// A second evaluation (sweep or poke) adds nothing.
	r.eval(low, 0)
	r.eval(low, 77)
	if n := len(r.outbox(low)[r1]); n != 0 {
		t.Errorf("R1 holds %d rows after repeat evaluations, want 0", n)
	}
	if n := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reliable_nearby'`, r1, low); n != 1 {
		t.Errorf("3.2 reliable_nearby rows for R1 after repeats = %d, want 1", n)
	}
	if _, rel := r.progress(low); rel != "fanned" {
		t.Errorf("reliable_state = %q, want fanned", rel)
	}

	// A Critical report by the Reliable reporter goes out as Critical to all
	// three readers, R1 (stored Critical off) included, and no reliable row is
	// queued even after R2's own bell poll wrote the 3.2 note.
	r.cur = r.base.Add(20 * time.Minute)
	critical := r.report(reporterSession, SeverityCritical, r.base.Add(15*time.Minute), "clear")
	r.eval(critical, 0)
	if err := r.updates.Refresh(context.Background(), r2); err != nil {
		t.Fatalf("bell refresh for R2: %v", err)
	}
	r.eval(critical, 41)
	got = r.outbox(critical)
	for name, id := range map[string]int64{"R1": r1, "R2": r2, "R3": r3} {
		if !outboxKindsEqual(got[id], "critical") {
			t.Errorf("%s holds %v for a Critical report, want exactly one critical row and no reliable row", name, got[id])
		}
	}
}

func TestPushChangedRaceWithBell(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const posterSession = "p33-09-race-poster"
	testutil.SeedReporterAccount(t, r.pool, r.email("poster"), posterSession)
	// Followers vote on the report; none has a home area, so no initial rows.
	follower := testutil.SeedReporterAccount(t, r.pool, r.email("follower"), "p33-09-race-follower")
	voter := testutil.SeedReporterAccount(t, r.pool, r.email("voter"), "p33-09-race-voter")
	other := testutil.SeedReporterAccount(t, r.pool, r.email("other"), "p33-09-race-other")
	for _, a := range []struct {
		id      int64
		session string
	}{{follower, "p33-09-race-follower"}, {voter, "p33-09-race-voter"}} {
		r.setSettings(a.id, 5, true, false, true)
		testutil.SeedPushSubscription(t, r.pool, a.id, a.session, "")
	}

	// run builds a non critical report that reaches Live while the follower has a
	// baseline cursor, and returns it. Each ordering uses its own report.
	run := func(offset time.Duration) int64 {
		t.Helper()
		start := r.base.Add(offset)
		r.cur = start
		report := r.report(posterSession, SeverityLow, start, "clear")
		r.vote(report, follower, "content", "confirm", "tdr1v8y", start.Add(1*time.Minute))
		r.cur = start.Add(2 * time.Minute)
		if err := r.updates.Refresh(ctx, follower); err != nil {
			t.Fatalf("baseline refresh: %v", err)
		}
		r.vote(report, other, "content", "confirm", "tdr1vc3", start.Add(5*time.Minute))
		r.vote(report, voter, "content", "confirm", "tdr1vd1", start.Add(6*time.Minute))
		r.cur = start.Add(10 * time.Minute)
		return report
	}
	changedFor := func(report, account int64) int {
		return r.count(`SELECT COUNT(*) FROM push_outbox WHERE report_id = $1 AND account_id = $2 AND kind = 'changed'`, report, account)
	}
	reachedLive := func(report, account int64) int {
		return r.count(`SELECT COUNT(*) FROM updates WHERE report_id = $1 AND account_id = $2 AND kind = 'reached_live'`, report, account)
	}

	t.Run("the bell writes the row first", func(t *testing.T) {
		report := run(0)
		if err := r.updates.Refresh(ctx, follower); err != nil {
			t.Fatalf("bell refresh: %v", err)
		}
		if reachedLive(report, follower) != 1 {
			t.Fatalf("the bell wrote %d reached_live rows, want 1", reachedLive(report, follower))
		}
		r.eval(report, voter)
		r.eval(report, voter)
		// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications
		// box): change notes stay in the bell, so no phone row is ever queued and
		// the bell row stays.
		if n := changedFor(report, follower); n != 0 {
			t.Errorf("changed rows for the follower = %d, want 0", n)
		}
		if reachedLive(report, follower) != 1 {
			t.Errorf("the bell holds %d reached_live rows after the dispatcher ran, want 1", reachedLive(report, follower))
		}
		if n := changedFor(report, voter); n != 0 {
			t.Errorf("the voter got %d change pushes for their own vote", n)
		}
	})

	t.Run("the dispatcher writes the row first", func(t *testing.T) {
		report := run(30 * time.Minute)
		r.eval(report, voter)
		if reachedLive(report, follower) != 1 {
			t.Fatalf("Refresh wrote %d reached_live rows, want 1", reachedLive(report, follower))
		}
		if err := r.updates.Refresh(ctx, follower); err != nil {
			t.Fatalf("bell refresh: %v", err)
		}
		r.eval(report, voter)
		if n := changedFor(report, follower); n != 0 {
			t.Errorf("changed rows for the follower = %d, want 0", n)
		}
		if reachedLive(report, follower) != 1 {
			t.Errorf("the bell holds %d reached_live rows after both ran, want 1", reachedLive(report, follower))
		}
		if n := changedFor(report, voter); n != 0 {
			t.Errorf("the voter got %d change pushes for their own vote", n)
		}
	})
}

func TestPushCaps(t *testing.T) {
	t.Run("the fourth report in an hour is capped for good", func(t *testing.T) {
		r := newPushDBRig(t)

		const reporterSession = "p33-09-caps-reporter"
		testutil.SeedReliableReporter(t, r.pool, r.email("caps"), reporterSession, r.base)
		x, _ := r.reader("x", 0, 5, true, false, false) // Critical on
		y, _ := r.reader("y", 0, 5, false, true, false) // Critical off, Reliable on

		var ids []int64
		for i, offset := range []time.Duration{0, 5 * time.Minute, 10 * time.Minute} {
			r.cur = r.base.Add(offset + time.Minute)
			id := r.report(reporterSession, SeverityCritical, r.base.Add(offset), "clear")
			r.eval(id, 0)
			ids = append(ids, id)
			if !outboxKindsEqual(r.outbox(id)[x], "critical") {
				t.Fatalf("report %d queued %v for X, want one critical row", i+1, r.outbox(id)[x])
			}
			if c, _ := r.progress(id); c != "fanned" {
				t.Fatalf("report %d critical_state = %q, want fanned", i+1, c)
			}
		}

		// One row per account and report, however many evaluations run.
		for _, actor := range []int64{0, 5, 0} {
			r.eval(ids[0], actor)
		}
		if n := len(r.outbox(ids[0])[x]); n != 1 {
			t.Errorf("X holds %d rows for the first report after repeat evaluations, want 1", n)
		}

		// N4: a report fanned under Critical takes no Reliable slot.
		n, err := r.q.CountReporterFanouts(context.Background(), sqlcgen.CountReporterFanoutsParams{
			ReporterAccountID: pushDBReporterAccount(t, r, reporterSession),
			ExcludeReportID:   0,
			Kind:              "reliable",
			Since:             r.base.Add(-time.Hour),
		})
		if err != nil {
			t.Fatalf("CountReporterFanouts: %v", err)
		}
		if n != 0 {
			t.Errorf("reliable fan outs of the reporter = %d, want 0 after three Critical fan outs", n)
		}

		// The fourth report, 50 minutes in, is over the cap.
		r.cur = r.base.Add(50*time.Minute + time.Minute)
		fourth := r.report(reporterSession, SeverityCritical, r.base.Add(50*time.Minute), "clear")
		r.eval(fourth, 0)
		if c, rel := r.progress(fourth); c != "capped" || rel != "capped" {
			t.Errorf("fourth report states = %q and %q, want capped and capped", c, rel)
		}
		got := r.outbox(fourth)
		if len(got[x]) != 0 {
			t.Errorf("X was queued %v for a capped report", got[x])
		}
		if len(got[y]) != 0 {
			t.Errorf("Y (Critical off, Reliable on) was queued %v for a report capped under Critical", got[y])
		}

		// Past the hour the reporter's earlier fan outs no longer count, but a
		// capped report stays capped for good.
		r.cur = r.base.Add(75 * time.Minute)
		r.eval(fourth, 0)
		r.eval(fourth, 9)
		if c, _ := r.progress(fourth); c != "capped" {
			t.Errorf("fourth report critical_state = %q after the hour, want still capped", c)
		}
		if len(r.outbox(fourth)) != 0 {
			t.Errorf("a capped report was queued later: %v", r.outbox(fourth))
		}
	})

	t.Run("a confirmed update is queued once, only after an alert that said Not confirmed yet", func(t *testing.T) {
		r := newPushDBRig(t)
		ctx := context.Background()

		const reporterSession = "p33-09-confirm-reporter"
		testutil.SeedReporterAccount(t, r.pool, r.email("confirm"), reporterSession)
		x, _ := r.reader("x", 0, 5, true, false, false)
		z, _ := r.reader("z", 0, 5, true, false, false)
		v1 := testutil.SeedReporterAccount(t, r.pool, r.email("v1"), "p33-09-confirm-v1")
		v2 := testutil.SeedReporterAccount(t, r.pool, r.email("v2"), "p33-09-confirm-v2")

		r.cur = r.base.Add(10 * time.Minute)
		first := r.report(reporterSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
		second := r.report(reporterSession, SeverityCritical, r.base.Add(6*time.Minute), "clear")
		r.eval(first, 0)
		r.eval(second, 0)

		// Mark X's first row sent saying Not confirmed yet, Z's sent with words that
		// did not say it, and leave X's second row queued, the way the worker does:
		// claim, then mark with the lease the claim returned.
		claimed, err := r.q.ClaimDuePushes(ctx, sqlcgen.ClaimDuePushesParams{BatchLimit: 100, LeaseSeconds: 60})
		if err != nil {
			t.Fatalf("ClaimDuePushes: %v", err)
		}
		marked := 0
		for _, c := range claimed {
			if c.ReportID != first {
				continue
			}
			unconfirmed := c.AccountID == x
			n, err := r.q.MarkPushSent(ctx, sqlcgen.MarkPushSentParams{
				Now: r.cur, SentUnconfirmed: unconfirmed, ID: c.ID, HeldLease: c.LeaseUntil,
			})
			if err != nil || n != 1 {
				t.Fatalf("MarkPushSent: %d rows, %v", n, err)
			}
			marked++
		}
		if marked != 2 {
			t.Fatalf("marked %d rows of the first report, want 2", marked)
		}

		// Not confirmed yet: no confirmed update.
		r.eval(first, 0)
		if n := r.count(`SELECT COUNT(*) FROM push_outbox WHERE kind = 'confirmed_update'`); n != 0 {
			t.Fatalf("confirmed updates before any confirmation = %d", n)
		}

		// Two cells confirm the first report.
		r.vote(first, v1, "content", "confirm", "tdr1v8y", r.cur)
		r.vote(first, v2, "content", "confirm", "tdr1vc3", r.cur)
		r.eval(first, v1)
		r.eval(first, v2)
		r.eval(first, 0)
		if n := r.count(`SELECT COUNT(*) FROM push_outbox WHERE kind = 'confirmed_update' AND report_id = $1 AND account_id = $2`, first, x); n != 1 {
			t.Errorf("confirmed updates for X = %d, want 1", n)
		}
		if n := r.count(`SELECT COUNT(*) FROM push_outbox WHERE kind = 'confirmed_update' AND account_id = $1`, z); n != 0 {
			t.Errorf("confirmed updates for Z = %d, want 0: its alert did not say Not confirmed yet", n)
		}
		if n := r.count(`SELECT COUNT(*) FROM push_outbox WHERE kind = 'confirmed_update' AND report_id = $1`, second); n != 0 {
			t.Errorf("confirmed updates for the unconfirmed second report = %d", n)
		}
	})

	t.Run("N2 a re run serves a reader who registered since and keeps the report fanned", func(t *testing.T) {
		r := newPushDBRig(t)

		const reporterSession = "p33-09-rerun-reporter"
		testutil.SeedReporterAccount(t, r.pool, r.email("rerun"), reporterSession)
		x, _ := r.reader("x", 0, 5, true, false, false)
		w, _ := r.reader("w", 0, 5, true, false, false)

		r.cur = r.base.Add(10 * time.Minute)
		id := r.report(reporterSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
		r.eval(id, 0)
		if c, _ := r.progress(id); c != "fanned" {
			t.Fatalf("critical_state = %q, want fanned", c)
		}

		// A new reader in range registers a device; the 30 second sweep (actor 0)
		// evaluates the report again.
		late, _ := r.reader("late", 0, 5, true, false, false)
		r.cur = r.base.Add(11 * time.Minute)
		r.eval(id, 0)

		got := r.outbox(id)
		for name, acct := range map[string]int64{"X": x, "W": w, "the late reader": late} {
			if !outboxKindsEqual(got[acct], "critical") {
				t.Errorf("%s holds %v, want exactly one critical row", name, got[acct])
			}
		}
		if c, _ := r.progress(id); c != "fanned" {
			t.Errorf("critical_state = %q after the re run, want still fanned", c)
		}
	})
}

// pushDBReporterAccount returns the account bound to a session.
func pushDBReporterAccount(t *testing.T, r *pushDBRig, session string) int64 {
	t.Helper()
	var id int64
	if err := r.pool.QueryRow(context.Background(), "SELECT account_id FROM sessions WHERE session_id = $1", session).Scan(&id); err != nil {
		t.Fatalf("reading the account of session %q: %v", session, err)
	}
	return id
}
