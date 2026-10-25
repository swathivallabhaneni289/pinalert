// Package store_test tests the Phase 3.3 storage: device subscriptions and
// their ownership, the Critical default, the home cell candidate read, the
// outbox uniqueness rules, the lease conditional claim and marks, the fan out
// counts and the send time switch read (03.3-01-PLAN.md). Every test takes its
// instant as a fixed value and passes it to the statements. The claim and the
// lease use the database clock, so the fixed instants sit in the past.
package store_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

var (
	pushNow = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	pushFar = time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)
)

// pushPerson seeds an account bound to one session with one live subscription.
func pushPerson(t *testing.T, pool *pgxpool.Pool, email, sessionID string) int64 {
	t.Helper()
	id := testutil.SeedReporterAccount(t, pool, email, sessionID)
	testutil.SeedPushSubscription(t, pool, id, sessionID, "")
	return id
}

// pushReport seeds one report filed from sessionID that expires at expiresAt.
func pushReport(t *testing.T, pool *pgxpool.Pool, sessionID string, expiresAt time.Time) int64 {
	t.Helper()
	return testutil.SeedHistoryReport(t, pool, sessionID, "medium", "flood", expiresAt)
}

func pushCount(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("counting with %q: %v", query, err)
	}
	return n
}

// pushEnqueue queues an initial push and returns its id; it fails the test when
// no row comes back.
func pushEnqueue(t *testing.T, q *sqlcgen.Queries, accountID, reportID int64, kind string) int64 {
	t.Helper()
	id, err := q.EnqueueInitialPush(context.Background(), sqlcgen.EnqueueInitialPushParams{
		AccountID: accountID, ReportID: reportID, Kind: kind, ExpiresAt: pushFar, Now: pushNow,
	})
	if err != nil {
		t.Fatalf("enqueue %s for account %d report %d: %v", kind, accountID, reportID, err)
	}
	return id
}

func pushClaim(t *testing.T, q *sqlcgen.Queries, batch int32) []sqlcgen.ClaimDuePushesRow {
	t.Helper()
	rows, err := q.ClaimDuePushes(context.Background(), sqlcgen.ClaimDuePushesParams{BatchLimit: batch, LeaseSeconds: 60})
	if err != nil {
		t.Fatalf("claiming due pushes: %v", err)
	}
	return rows
}

func pushUpsertSub(t *testing.T, q *sqlcgen.Queries, accountID int64, sessionID, endpoint string, at time.Time) int64 {
	t.Helper()
	id, err := q.UpsertPushSubscription(context.Background(), sqlcgen.UpsertPushSubscriptionParams{
		AccountID: accountID, SessionID: sessionID, Endpoint: endpoint,
		EndpointHost: "fcm.googleapis.com", P256dh: "p", Auth: "a", Now: at,
	})
	if err != nil {
		t.Fatalf("upserting subscription: %v", err)
	}
	return id
}

func TestUpsertPushSubscriptionMovesOwnership(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	a := testutil.SeedReporterAccount(t, pool, "move-a@example.com", "move-sess-a")
	b := testutil.SeedReporterAccount(t, pool, "move-b@example.com", "move-sess-b")
	const endpoint = "https://fcm.googleapis.com/fcm/send/shared-phone"

	first := pushUpsertSub(t, q, a, "move-sess-a", endpoint, pushNow)
	if _, err := q.BumpPushSubscriptionFailure(ctx, first); err != nil {
		t.Fatalf("bumping failure: %v", err)
	}
	if err := q.MarkPushSubscriptionOK(ctx, sqlcgen.MarkPushSubscriptionOKParams{ID: first, Now: pushNow}); err != nil {
		t.Fatalf("marking ok: %v", err)
	}
	if _, err := q.BumpPushSubscriptionFailure(ctx, first); err != nil {
		t.Fatalf("bumping failure again: %v", err)
	}

	second := pushUpsertSub(t, q, b, "move-sess-b", endpoint, pushNow.Add(time.Minute))
	if second != first {
		t.Errorf("the second registration got id %d, want the same row %d", second, first)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_subscriptions WHERE endpoint = $1`, endpoint); n != 1 {
		t.Fatalf("%d rows for the endpoint, want 1", n)
	}
	var owner int64
	var session string
	var failures int
	var lastOK *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT account_id, session_id, failures, last_ok_at FROM push_subscriptions WHERE endpoint = $1`, endpoint,
	).Scan(&owner, &session, &failures, &lastOK); err != nil {
		t.Fatalf("reading the row: %v", err)
	}
	if owner != b || session != "move-sess-b" {
		t.Errorf("owner = account %d session %q, want account %d session move-sess-b", owner, session, b)
	}
	if failures != 0 || lastOK != nil {
		t.Errorf("failures %d last_ok_at %v after the move, want 0 and NULL", failures, lastOK)
	}
	subs, err := q.PushSubscriptionsForAccount(ctx, a)
	if err != nil || len(subs) != 0 {
		t.Errorf("account A still has %d devices (%v), want none", len(subs), err)
	}
}

func TestTrimPushSubscriptionsKeepsNewest(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	a := testutil.SeedReporterAccount(t, pool, "trim-a@example.com", "trim-sess-a")
	b := testutil.SeedReporterAccount(t, pool, "trim-b@example.com", "trim-sess-b")
	endpoints := make([]string, 10)
	for i := range endpoints {
		endpoints[i] = "https://fcm.googleapis.com/fcm/send/trim-" + string(rune('a'+i))
		pushUpsertSub(t, q, a, "trim-sess-a", endpoints[i], pushNow.Add(time.Duration(i)*time.Minute))
	}
	for i := 0; i < 3; i++ {
		pushUpsertSub(t, q, b, "trim-sess-b", "https://fcm.googleapis.com/fcm/send/trim-other-"+string(rune('a'+i)), pushNow)
	}

	gone, err := q.TrimPushSubscriptions(ctx, sqlcgen.TrimPushSubscriptionsParams{AccountID: a, Keep: 8})
	if err != nil {
		t.Fatalf("trimming: %v", err)
	}
	if gone != 2 {
		t.Errorf("trim removed %d rows, want 2", gone)
	}
	for i, e := range endpoints {
		n := pushCount(t, pool, `SELECT count(*) FROM push_subscriptions WHERE endpoint = $1`, e)
		want := 1
		if i < 2 {
			want = 0
		}
		if n != want {
			t.Errorf("endpoint %d present %d times, want %d", i, n, want)
		}
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_subscriptions WHERE account_id = $1`, b); n != 3 {
		t.Errorf("account B has %d devices after the trim of A, want 3", n)
	}
}

func TestPushSubscriptionDeletes(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	a := testutil.SeedReporterAccount(t, pool, "del-a@example.com", "del-sess-1", "del-sess-2")
	b := testutil.SeedReporterAccount(t, pool, "del-b@example.com", "del-sess-3")
	const e1, e2, e3 = "https://fcm.googleapis.com/fcm/send/del-1", "https://fcm.googleapis.com/fcm/send/del-2", "https://fcm.googleapis.com/fcm/send/del-3"
	id1 := pushUpsertSub(t, q, a, "del-sess-1", e1, pushNow)
	pushUpsertSub(t, q, a, "del-sess-2", e2, pushNow)
	pushUpsertSub(t, q, b, "del-sess-3", e3, pushNow)

	if ok, err := q.PushSubscriptionRegistered(ctx, sqlcgen.PushSubscriptionRegisteredParams{AccountID: a, Endpoint: e1}); err != nil || !ok {
		t.Errorf("registered for the owner = %v (%v), want true", ok, err)
	}
	if ok, err := q.PushSubscriptionRegistered(ctx, sqlcgen.PushSubscriptionRegisteredParams{AccountID: b, Endpoint: e1}); err != nil || ok {
		t.Errorf("registered for another account = %v (%v), want false", ok, err)
	}

	// Failure counter: 1, 2, then back to 0 after a good send.
	for want := int32(1); want <= 2; want++ {
		row, err := q.BumpPushSubscriptionFailure(ctx, id1)
		if err != nil || row.Failures != want {
			t.Fatalf("bump = %d (%v), want %d", row.Failures, err, want)
		}
	}
	if err := q.MarkPushSubscriptionOK(ctx, sqlcgen.MarkPushSubscriptionOKParams{ID: id1, Now: pushNow}); err != nil {
		t.Fatalf("marking ok: %v", err)
	}
	row, err := q.BumpPushSubscriptionFailure(ctx, id1)
	if err != nil || row.Failures != 1 || !row.LastOkAt.Valid {
		t.Errorf("bump after ok = %d last_ok_at valid %v (%v), want 1 and valid", row.Failures, row.LastOkAt.Valid, err)
	}

	if n, err := q.DeletePushSubscriptionsForSession(ctx, "del-sess-1"); err != nil || n != 1 {
		t.Errorf("delete by session = %d (%v), want 1", n, err)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_subscriptions`); n != 2 {
		t.Errorf("%d devices left after the session delete, want 2", n)
	}
	if n, err := q.DeletePushSubscriptionForAccount(ctx, sqlcgen.DeletePushSubscriptionForAccountParams{AccountID: b, Endpoint: e2}); err != nil || n != 0 {
		t.Errorf("delete of another account's endpoint = %d (%v), want 0", n, err)
	}
	if n, err := q.DeletePushSubscriptionForAccount(ctx, sqlcgen.DeletePushSubscriptionForAccountParams{AccountID: a, Endpoint: e2}); err != nil || n != 1 {
		t.Errorf("delete of own endpoint = %d (%v), want 1", n, err)
	}

	// By account removes every device of that account and leaves the other.
	pushUpsertSub(t, q, a, "del-sess-1", e1, pushNow)
	pushUpsertSub(t, q, a, "del-sess-2", e2, pushNow)
	if n, err := q.DeleteAccountPushSubscriptions(ctx, a); err != nil || n != 2 {
		t.Errorf("delete by account = %d (%v), want 2", n, err)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_subscriptions WHERE account_id = $1`, b); n != 1 {
		t.Errorf("account B has %d devices, want 1", n)
	}
	devices, err := q.PushSubscriptionsForAccount(ctx, b)
	if err != nil || len(devices) != 1 {
		t.Fatalf("devices of B = %d (%v), want 1", len(devices), err)
	}
	if err := q.DeletePushSubscriptionByID(ctx, devices[0].ID); err != nil {
		t.Fatalf("delete by id: %v", err)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_subscriptions`); n != 0 {
		t.Errorf("%d devices left, want 0", n)
	}

	// The outbox copy of one account goes alone.
	r := pushReport(t, pool, "del-sess-1", pushFar)
	pushEnqueue(t, q, a, r, "critical")
	pushEnqueue(t, q, b, r, "critical")
	if n, err := q.DeleteAccountPushOutbox(ctx, a); err != nil || n != 1 {
		t.Errorf("delete outbox of A = %d (%v), want 1", n, err)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_outbox WHERE account_id = $1`, b); n != 1 {
		t.Errorf("account B has %d outbox rows, want 1", n)
	}
}

func TestPushSubscriptionsSkipRevokedAndDeleted(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	revoked := pushPerson(t, pool, "live-revoked@example.com", "live-sess-revoked")
	deleted := pushPerson(t, pool, "live-deleted@example.com", "live-sess-deleted")
	normal := pushPerson(t, pool, "live-normal@example.com", "live-sess-normal")
	testutil.MustExec(t, pool, `UPDATE sessions SET revoked_at = $2 WHERE session_id = $1`, "live-sess-revoked", pushNow)
	testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = $2 WHERE id = $1`, deleted, pushNow)

	for _, c := range []struct {
		name string
		id   int64
		want int
	}{{"revoked session", revoked, 0}, {"deleted account", deleted, 0}, {"normal account", normal, 1}} {
		subs, err := q.PushSubscriptionsForAccount(ctx, c.id)
		if err != nil || len(subs) != c.want {
			t.Errorf("%s: %d devices (%v), want %d", c.name, len(subs), err, c.want)
		}
	}
}

func TestPushKindSettingsDefaults(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	a := testutil.SeedReporterAccount(t, pool, "kinds@example.com", "kinds-sess")

	got, err := q.GetPushKindSettings(ctx, a)
	if err != nil {
		t.Fatalf("reading defaults: %v", err)
	}
	if !got.CriticalPushOn || got.ChangesPushOn || got.ChangesEnabledAt.Valid {
		t.Errorf("no row reads critical %v changes %v enabled %v, want true, false, NULL",
			got.CriticalPushOn, got.ChangesPushOn, got.ChangesEnabledAt.Valid)
	}

	t1 := pushNow
	saved, err := q.UpsertPushKindSettings(ctx, sqlcgen.UpsertPushKindSettingsParams{
		AccountID: a, CriticalPushOn: true, ChangesPushOn: true, Now: t1})
	if err != nil || !saved.ChangesPushOn || !saved.ChangesEnabledAt.Valid || !saved.ChangesEnabledAt.Time.Equal(t1) {
		t.Fatalf("turning Changes on = %+v (%v), want enabled at %v", saved, err, t1)
	}
	again, err := q.UpsertPushKindSettings(ctx, sqlcgen.UpsertPushKindSettingsParams{
		AccountID: a, CriticalPushOn: false, ChangesPushOn: true, Now: t1.Add(time.Hour)})
	if err != nil || again.CriticalPushOn || !again.ChangesEnabledAt.Time.Equal(t1) {
		t.Errorf("a second on = %+v (%v), want Critical off and enabled still %v", again, err, t1)
	}
	off, err := q.UpsertPushKindSettings(ctx, sqlcgen.UpsertPushKindSettingsParams{
		AccountID: a, CriticalPushOn: false, ChangesPushOn: false, Now: t1.Add(2 * time.Hour)})
	if err != nil || off.ChangesPushOn || off.ChangesEnabledAt.Valid {
		t.Errorf("turning Changes off = %+v (%v), want enabled cleared", off, err)
	}
	on2, err := q.UpsertPushKindSettings(ctx, sqlcgen.UpsertPushKindSettingsParams{
		AccountID: a, CriticalPushOn: false, ChangesPushOn: true, Now: t1.Add(3 * time.Hour)})
	if err != nil || !on2.ChangesEnabledAt.Time.Equal(t1.Add(3*time.Hour)) {
		t.Errorf("turning Changes on again = %+v (%v), want a fresh stamp", on2, err)
	}

	// A row inserted by the push upsert keeps the 3.2 defaults.
	b := testutil.SeedReporterAccount(t, pool, "kinds-b@example.com", "kinds-sess-b")
	if _, err := q.UpsertPushKindSettings(ctx, sqlcgen.UpsertPushKindSettingsParams{
		AccountID: b, CriticalPushOn: true, ChangesPushOn: false, Now: t1}); err != nil {
		t.Fatalf("upsert for B: %v", err)
	}
	var reliable bool
	var radius int
	var reliableAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT reliable_alerts_on, alert_radius_km, enabled_at FROM account_alert_settings WHERE account_id = $1`, b,
	).Scan(&reliable, &radius, &reliableAt); err != nil {
		t.Fatalf("reading the 3.2 columns: %v", err)
	}
	if reliable || radius != 5 || reliableAt != nil {
		t.Errorf("3.2 columns = %v, %d, %v, want false, 5, NULL", reliable, radius, reliableAt)
	}
}

func TestPushHomeCandidates(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	author := testutil.SeedReporterAccount(t, pool, "cand-author@example.com", "cand-sess-author")
	_ = author
	report := pushReport(t, pool, "cand-sess-author", pushFar)
	cells := []string{"tdr1v", "tdr1y", "tdr3b"}

	in1 := pushPerson(t, pool, "cand-in1@example.com", "cand-sess-in1")
	testutil.SetHomeCell(t, pool, in1, "Benz Circle", "tdr1v")
	in2 := pushPerson(t, pool, "cand-in2@example.com", "cand-sess-in2")
	testutil.SetHomeCell(t, pool, in2, "Hebbal", "tdr1y")
	testutil.MustExec(t, pool,
		`INSERT INTO account_alert_settings (account_id, reliable_alerts_on, alert_radius_km, critical_push_on) VALUES ($1, true, 10, false)`, in2)
	out := pushPerson(t, pool, "cand-out@example.com", "cand-sess-out")
	testutil.SetHomeCell(t, pool, out, "Far away", "tdr3c")
	muted := pushPerson(t, pool, "cand-muted@example.com", "cand-sess-muted")
	testutil.SetHomeCell(t, pool, muted, "Muted", "tdr1v")
	testutil.MustExec(t, pool, `INSERT INTO report_mutes (account_id, report_id) VALUES ($1, $2)`, muted, report)
	gone := pushPerson(t, pool, "cand-gone@example.com", "cand-sess-gone")
	testutil.SetHomeCell(t, pool, gone, "Gone", "tdr1v")
	testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = $2 WHERE id = $1`, gone, pushNow)
	revoked := pushPerson(t, pool, "cand-revoked@example.com", "cand-sess-revoked")
	testutil.SetHomeCell(t, pool, revoked, "Revoked", "tdr1v")
	testutil.MustExec(t, pool, `UPDATE sessions SET revoked_at = $2 WHERE session_id = $1`, "cand-sess-revoked", pushNow)
	nosub := testutil.SeedReporterAccount(t, pool, "cand-nosub@example.com", "cand-sess-nosub")
	testutil.SetHomeCell(t, pool, nosub, "No device", "tdr1v")

	rows, err := q.PushHomeCandidates(ctx, sqlcgen.PushHomeCandidatesParams{Cells: cells, ReportID: report, RowCap: 100})
	if err != nil {
		t.Fatalf("candidates: %v", err)
	}
	if len(rows) != 2 || rows[0].AccountID != in1 || rows[1].AccountID != in2 {
		t.Fatalf("candidates = %+v, want accounts %d then %d", rows, in1, in2)
	}
	if rows[0].AlertRadiusKm != 5 || !rows[0].CriticalPushOn || rows[0].ReliableAlertsOn || rows[0].HomeAreaCell != "tdr1v" {
		t.Errorf("defaults row = %+v, want radius 5, critical on, reliable off, cell tdr1v", rows[0])
	}
	if rows[1].AlertRadiusKm != 10 || rows[1].CriticalPushOn || !rows[1].ReliableAlertsOn {
		t.Errorf("stored row = %+v, want radius 10, critical off, reliable on", rows[1])
	}

	// The candidate row carries no position of any kind (D-20).
	typ := reflect.TypeOf(sqlcgen.PushHomeCandidatesRow{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		for _, bad := range []string{"Latitude", "Longitude", "Geo"} {
			if strings.Contains(name, bad) {
				t.Errorf("PushHomeCandidatesRow has a field %q that looks like a position", name)
			}
		}
	}

	// Cap order (N1): the accounts of the first two cells survive, whatever the
	// order they were created in.
	capCells := []string{"u4pru", "u4prv", "u4prw"}
	var capIDs []int64
	for i := 2; i >= 0; i-- {
		id := pushPerson(t, pool, "cand-cap-"+capCells[i]+"@example.com", "cand-sess-cap-"+capCells[i])
		testutil.SetHomeCell(t, pool, id, "Cap "+capCells[i], capCells[i])
		capIDs = append([]int64{id}, capIDs...)
	}
	capRows, err := q.PushHomeCandidates(ctx, sqlcgen.PushHomeCandidatesParams{Cells: capCells, ReportID: report, RowCap: 2})
	if err != nil {
		t.Fatalf("capped candidates: %v", err)
	}
	if len(capRows) != 2 || capRows[0].AccountID != capIDs[0] || capRows[1].AccountID != capIDs[1] {
		t.Errorf("capped candidates = %+v, want accounts %d then %d (never the third cell)", capRows, capIDs[0], capIDs[1])
	}
}

func TestFollowersWithChangesPush(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	author := pushPerson(t, pool, "fol-author@example.com", "fol-sess-author")
	confirmer := pushPerson(t, pool, "fol-confirmer@example.com", "fol-sess-confirmer")
	reopener := pushPerson(t, pool, "fol-reopener@example.com", "fol-sess-reopener")
	off := pushPerson(t, pool, "fol-off@example.com", "fol-sess-off")
	norow := pushPerson(t, pool, "fol-norow@example.com", "fol-sess-norow")
	mutedF := pushPerson(t, pool, "fol-muted@example.com", "fol-sess-muted")
	stranger := pushPerson(t, pool, "fol-stranger@example.com", "fol-sess-stranger")
	report := pushReport(t, pool, "fol-sess-author", pushFar)

	vote := func(account int64, kind, value string) {
		testutil.MustExec(t, pool,
			`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, $3, $4, 'tdr1v8y')`,
			report, account, kind, value)
	}
	vote(confirmer, "content", "confirm")
	vote(reopener, "resolution", "reopen")
	vote(off, "content", "confirm")
	vote(norow, "content", "confirm")
	vote(mutedF, "content", "confirm")
	testutil.MustExec(t, pool, `INSERT INTO report_mutes (account_id, report_id) VALUES ($1, $2)`, mutedF, report)

	for _, c := range []struct {
		id      int64
		changes bool
		write   bool
	}{{author, true, true}, {confirmer, true, true}, {reopener, true, true}, {off, false, true},
		{mutedF, true, true}, {stranger, true, true}} {
		if !c.write {
			continue
		}
		if _, err := q.UpsertPushKindSettings(ctx, sqlcgen.UpsertPushKindSettingsParams{
			AccountID: c.id, CriticalPushOn: true, ChangesPushOn: c.changes, Now: pushNow}); err != nil {
			t.Fatalf("settings for %d: %v", c.id, err)
		}
	}

	got, err := q.FollowersWithChangesPush(ctx, report)
	if err != nil {
		t.Fatalf("followers: %v", err)
	}
	want := []int64{author, confirmer, reopener}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("followers = %v, want %v (author, confirmer, reopener)", got, want)
	}
}

func TestAccountsWithChangesPush(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	a := pushPerson(t, pool, "awc-a@example.com", "awc-sess-a")
	b := pushPerson(t, pool, "awc-b@example.com", "awc-sess-b")
	c := testutil.SeedReporterAccount(t, pool, "awc-c@example.com", "awc-sess-c") // no device
	for _, id := range []int64{a, b, c} {
		if _, err := q.UpsertPushKindSettings(ctx, sqlcgen.UpsertPushKindSettingsParams{
			AccountID: id, CriticalPushOn: true, ChangesPushOn: true, Now: pushNow}); err != nil {
			t.Fatalf("settings: %v", err)
		}
	}
	got, err := q.AccountsWithChangesPush(ctx, sqlcgen.AccountsWithChangesPushParams{RowCap: 10})
	if err != nil || !reflect.DeepEqual(got, []int64{a, b}) {
		t.Errorf("accounts = %v (%v), want %v", got, err, []int64{a, b})
	}
	got, err = q.AccountsWithChangesPush(ctx, sqlcgen.AccountsWithChangesPushParams{RowCap: 1})
	if err != nil || !reflect.DeepEqual(got, []int64{a}) {
		t.Errorf("capped accounts = %v (%v), want %v", got, err, []int64{a})
	}
	// Review S-04 (2026-10-25): the next page starts after the given id, so a
	// caller can rotate past the cap instead of always taking the same first rows.
	got, err = q.AccountsWithChangesPush(ctx, sqlcgen.AccountsWithChangesPushParams{AfterID: a, RowCap: 1})
	if err != nil || !reflect.DeepEqual(got, []int64{b}) {
		t.Errorf("accounts after %d = %v (%v), want %v", a, got, err, []int64{b})
	}
	got, err = q.AccountsWithChangesPush(ctx, sqlcgen.AccountsWithChangesPushParams{AfterID: b, RowCap: 10})
	if err != nil || len(got) != 0 {
		t.Errorf("accounts after the last = %v (%v), want none", got, err)
	}
}

func TestEnqueueInitialOncePerAccountReport(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	a := pushPerson(t, pool, "once-a@example.com", "once-sess-a")
	r1 := pushReport(t, pool, "once-sess-a", pushFar)
	r2 := pushReport(t, pool, "once-sess-a", pushFar)

	pushEnqueue(t, q, a, r1, "critical")
	_, err := q.EnqueueInitialPush(ctx, sqlcgen.EnqueueInitialPushParams{
		AccountID: a, ReportID: r1, Kind: "reliable", ExpiresAt: pushFar, Now: pushNow})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("a reliable row beside a critical one = %v, want no row", err)
	}
	_, err = q.EnqueueInitialPush(ctx, sqlcgen.EnqueueInitialPushParams{
		AccountID: a, ReportID: r1, Kind: "critical", ExpiresAt: pushFar, Now: pushNow})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("a second critical row = %v, want no row", err)
	}
	var kind string
	if err := pool.QueryRow(ctx, `SELECT kind FROM push_outbox WHERE account_id = $1 AND report_id = $2`, a, r1).Scan(&kind); err != nil || kind != "critical" {
		t.Errorf("the one row is kind %q (%v), want critical", kind, err)
	}
	pushEnqueue(t, q, a, r2, "reliable")
	if n := pushCount(t, pool, `SELECT count(*) FROM push_outbox WHERE account_id = $1`, a); n != 2 {
		t.Errorf("%d rows in all, want 2", n)
	}
}

func TestEnqueueReliableFromUpdates(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	author := pushPerson(t, pool, "rel-author@example.com", "rel-sess-author")
	_ = author
	withDevice := pushPerson(t, pool, "rel-device@example.com", "rel-sess-device")
	noDevice := testutil.SeedReporterAccount(t, pool, "rel-nodevice@example.com", "rel-sess-nodevice")
	alreadyCritical := pushPerson(t, pool, "rel-critical@example.com", "rel-sess-critical")
	report := pushReport(t, pool, "rel-sess-author", pushFar)
	for _, id := range []int64{withDevice, noDevice, alreadyCritical} {
		testutil.MustExec(t, pool,
			`INSERT INTO updates (account_id, report_id, kind, created_at) VALUES ($1, $2, 'reliable_nearby', $3)`,
			id, report, pushNow)
	}
	pushEnqueue(t, q, alreadyCritical, report, "critical")

	n, err := q.EnqueueReliableFromUpdates(ctx, sqlcgen.EnqueueReliableFromUpdatesParams{ReportID: report, ExpiresAt: pushFar, Now: pushNow})
	if err != nil || n != 1 {
		t.Fatalf("enqueue reliable = %d (%v), want 1 (only the account with a device and no first row)", n, err)
	}
	var kind string
	if err := pool.QueryRow(ctx, `SELECT kind FROM push_outbox WHERE account_id = $1`, withDevice).Scan(&kind); err != nil || kind != "reliable" {
		t.Errorf("row kind %q (%v), want reliable", kind, err)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_outbox WHERE account_id = $1 AND kind = 'critical'`, alreadyCritical); n != 1 {
		t.Errorf("the critical row count = %d, want 1 (critical wins)", n)
	}
	if again, err := q.EnqueueReliableFromUpdates(ctx, sqlcgen.EnqueueReliableFromUpdatesParams{ReportID: report, ExpiresAt: pushFar, Now: pushNow}); err != nil || again != 0 {
		t.Errorf("a second call = %d (%v), want 0", again, err)
	}
}

func TestEnqueueChangedFromUpdatesOnce(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	a := pushPerson(t, pool, "chg-a@example.com", "chg-sess-a")
	r := pushReport(t, pool, "chg-sess-a", pushFar)
	r2 := pushReport(t, pool, "chg-sess-a", pushFar)
	row := func(report int64, kind string, at time.Time) {
		testutil.MustExec(t, pool,
			`INSERT INTO updates (account_id, report_id, kind, created_at) VALUES ($1, $2, $3, $4)`, a, report, kind, at)
	}
	row(r, "reached_live", pushNow.Add(-3*time.Minute))
	row(r, "fading", pushNow.Add(-2*time.Minute))
	row(r, "reliable_nearby", pushNow.Add(-2*time.Minute))
	row(r, "hidden", pushNow.Add(-5*time.Hour)) // older than since
	testutil.MustExec(t, pool,
		`INSERT INTO updates (account_id, kind, bucket_start, more_count, created_at) VALUES ($1, 'more', $2, 2, $3)`,
		a, pushNow.Truncate(time.Hour), pushNow.Add(-time.Minute))
	row(r2, "reached_live", pushNow.Add(-time.Minute))
	pushEnqueue(t, q, a, r2, "critical")

	arg := sqlcgen.EnqueueChangedFromUpdatesParams{AccountID: a, Since: pushNow.Add(-time.Hour), Now: pushNow}
	n, err := q.EnqueueChangedFromUpdates(ctx, arg)
	if err != nil || n != 2 {
		t.Fatalf("enqueue changed = %d (%v), want 2 (reached_live and fading on the first report)", n, err)
	}
	if again, err := q.EnqueueChangedFromUpdates(ctx, arg); err != nil || again != 0 {
		t.Errorf("a second call = %d (%v), want 0", again, err)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_outbox WHERE account_id = $1 AND kind = 'changed' AND report_id = $2`, a, r2); n != 0 {
		t.Errorf("%d changed rows for the report that has a critical row, want 0", n)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_outbox WHERE account_id = $1 AND kind = 'changed' AND update_id IS NOT NULL AND report_id = $2`, a, r); n != 2 {
		t.Errorf("%d changed rows for the first report, want 2", n)
	}
}

func TestEnqueueConfirmedUpdatesOnlyUnconfirmedSent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	a := pushPerson(t, pool, "conf-a@example.com", "conf-sess-a")
	b := pushPerson(t, pool, "conf-b@example.com", "conf-sess-b")
	c := pushPerson(t, pool, "conf-c@example.com", "conf-sess-c")
	d := pushPerson(t, pool, "conf-d@example.com", "conf-sess-d")
	r := pushReport(t, pool, "conf-sess-a", pushFar)
	pushEnqueue(t, q, a, r, "critical")
	pushEnqueue(t, q, b, r, "reliable")
	pushEnqueue(t, q, c, r, "critical")
	pushEnqueue(t, q, d, r, "critical")

	// Make A, B and D sent the way the worker does it: claim, then mark with the
	// lease the claim returned. C stays queued.
	for _, row := range pushClaim(t, q, 10) {
		var unconfirmed bool
		switch row.AccountID {
		case a, b:
			unconfirmed = true
		case d:
			unconfirmed = false
		default:
			continue
		}
		n, err := q.MarkPushSent(ctx, sqlcgen.MarkPushSentParams{
			ID: row.ID, HeldLease: row.LeaseUntil, Now: pushNow, SentUnconfirmed: unconfirmed})
		if err != nil || n != 1 {
			t.Fatalf("marking %d sent = %d (%v), want 1", row.ID, n, err)
		}
	}

	arg := sqlcgen.EnqueueConfirmedUpdatesParams{ReportID: r, ExpiresAt: pushFar, Now: pushNow}
	n, err := q.EnqueueConfirmedUpdates(ctx, arg)
	if err != nil || n != 2 {
		t.Fatalf("enqueue confirmed = %d (%v), want 2", n, err)
	}
	if again, err := q.EnqueueConfirmedUpdates(ctx, arg); err != nil || again != 0 {
		t.Errorf("a second call = %d (%v), want 0", again, err)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_outbox WHERE kind = 'confirmed_update' AND account_id IN ($1, $2)`, a, b); n != 2 {
		t.Errorf("confirmed rows for A and B = %d, want 2", n)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_outbox WHERE kind = 'confirmed_update' AND account_id IN ($1, $2)`, c, d); n != 0 {
		t.Errorf("confirmed rows for the queued and the confirmed-words rows = %d, want 0", n)
	}

	// The claim carries the base kind of each confirmed update.
	got := map[int64]string{}
	for _, row := range pushClaim(t, q, 10) {
		if row.Kind != "confirmed_update" {
			t.Errorf("claimed kind %q, want only confirmed_update (the others are sent or leased)", row.Kind)
			continue
		}
		if row.FirstKind == nil {
			t.Errorf("confirmed update for account %d has no first_kind", row.AccountID)
			continue
		}
		got[row.AccountID] = *row.FirstKind
	}
	if got[a] != "critical" || got[b] != "reliable" || len(got) != 2 {
		t.Errorf("first kinds = %v, want A critical and B reliable", got)
	}
}

func TestClaimDuePushesLease(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	clearOutbox := func() { testutil.MustExec(t, pool, `TRUNCATE push_outbox RESTART IDENTITY`) }

	var people []int64
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		people = append(people, pushPerson(t, pool, "claim-"+n+"@example.com", "claim-sess-"+n))
	}
	r := pushReport(t, pool, "claim-sess-a", pushFar)

	// Priority: critical before reliable whatever the id order.
	pushEnqueue(t, q, people[0], r, "reliable")
	pushEnqueue(t, q, people[1], r, "critical")
	rows := pushClaim(t, q, 10)
	if len(rows) != 2 || rows[0].Kind != "critical" || rows[1].Kind != "reliable" {
		t.Fatalf("claimed %+v, want critical then reliable", rows)
	}
	if rows[0].LeaseUntil.Before(time.Now()) || rows[0].Attempts != 1 {
		t.Errorf("claim lease %v attempts %d, want a future lease and 1 attempt", rows[0].LeaseUntil, rows[0].Attempts)
	}
	if again := pushClaim(t, q, 10); len(again) != 0 {
		t.Errorf("a leased row was claimed again: %+v", again)
	}

	// Two concurrent claims never return one id twice.
	clearOutbox()
	for i := 0; i < 3; i++ {
		pushEnqueue(t, q, people[i], r, "critical")
	}
	var wg sync.WaitGroup
	results := make([][]sqlcgen.ClaimDuePushesRow, 2)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := q.ClaimDuePushes(ctx, sqlcgen.ClaimDuePushesParams{BatchLimit: 2, LeaseSeconds: 60})
			if err != nil {
				t.Errorf("concurrent claim %d: %v", i, err)
			}
			results[i] = got
		}()
	}
	wg.Wait()
	seen := map[int64]bool{}
	for _, res := range results {
		for _, row := range res {
			if seen[row.ID] {
				t.Errorf("row %d was claimed twice", row.ID)
			}
			seen[row.ID] = true
		}
	}
	if len(seen) == 0 || len(seen) > 3 {
		t.Errorf("%d rows claimed, want between 1 and 3", len(seen))
	}

	// Lease rules (W2): five rows, claimed, aged and claimed again by a second
	// worker; the first worker's lease is now stale.
	clearOutbox()
	var ids []int64
	for i := 0; i < 5; i++ {
		ids = append(ids, pushEnqueue(t, q, people[i], r, "critical"))
	}
	first := map[int64]time.Time{}
	for _, row := range pushClaim(t, q, 10) {
		first[row.ID] = row.LeaseUntil
	}
	if len(first) != 5 {
		t.Fatalf("first claim took %d rows, want 5", len(first))
	}
	testutil.MustExec(t, pool, `UPDATE push_outbox SET lease_until = now() - interval '1 second'`)
	current := map[int64]time.Time{}
	for _, row := range pushClaim(t, q, 10) {
		current[row.ID] = row.LeaseUntil
		if row.Attempts != 2 {
			t.Errorf("row %d attempts %d after the second claim, want 2", row.ID, row.Attempts)
		}
	}
	if len(current) != 5 {
		t.Fatalf("second claim took %d rows, want 5", len(current))
	}

	stale := func(id int64) time.Time { return first[id] }
	for _, id := range ids {
		if _, err := q.ExtendPushLease(ctx, sqlcgen.ExtendPushLeaseParams{ID: id, HeldLease: stale(id), LeaseSeconds: 60}); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("extend with a stale lease on %d = %v, want no row", id, err)
		}
	}
	mustZero := func(label string, n int64, err error) {
		t.Helper()
		if err != nil || n != 0 {
			t.Errorf("%s = %d (%v), want 0 rows", label, n, err)
		}
	}
	mustOne := func(label string, n int64, err error) {
		t.Helper()
		if err != nil || n != 1 {
			t.Errorf("%s = %d (%v), want 1 row", label, n, err)
		}
	}
	later := pushNow.Add(time.Hour)

	// Stale marks change nothing.
	n, err := q.MarkPushSent(ctx, sqlcgen.MarkPushSentParams{ID: ids[0], HeldLease: stale(ids[0]), Now: pushNow})
	mustZero("stale MarkPushSent", n, err)
	n, err = q.MarkPushSkipped(ctx, sqlcgen.MarkPushSkippedParams{ID: ids[1], HeldLease: stale(ids[1]), SkipReason: "no_device"})
	mustZero("stale MarkPushSkipped", n, err)
	n, err = q.ReschedulePush(ctx, sqlcgen.ReschedulePushParams{ID: ids[2], HeldLease: stale(ids[2]), NextAttemptAt: later})
	mustZero("stale ReschedulePush", n, err)
	n, err = q.MarkPushFailed(ctx, sqlcgen.MarkPushFailedParams{ID: ids[3], HeldLease: stale(ids[3])})
	mustZero("stale MarkPushFailed", n, err)

	// Current marks change exactly one row.
	n, err = q.MarkPushSent(ctx, sqlcgen.MarkPushSentParams{ID: ids[0], HeldLease: current[ids[0]], Now: pushNow, SentUnconfirmed: true})
	mustOne("MarkPushSent", n, err)
	n, err = q.MarkPushSkipped(ctx, sqlcgen.MarkPushSkippedParams{ID: ids[1], HeldLease: current[ids[1]], SkipReason: "no_device"})
	mustOne("MarkPushSkipped", n, err)
	n, err = q.ReschedulePush(ctx, sqlcgen.ReschedulePushParams{ID: ids[2], HeldLease: current[ids[2]], NextAttemptAt: later})
	mustOne("ReschedulePush", n, err)
	n, err = q.MarkPushFailed(ctx, sqlcgen.MarkPushFailedParams{ID: ids[3], HeldLease: current[ids[3]]})
	mustOne("MarkPushFailed", n, err)

	// A finished row can never be changed again, even with the lease it held.
	n, err = q.MarkPushSkipped(ctx, sqlcgen.MarkPushSkippedParams{ID: ids[0], HeldLease: current[ids[0]], SkipReason: "stale"})
	mustZero("skip after sent", n, err)
	n, err = q.ReschedulePush(ctx, sqlcgen.ReschedulePushParams{ID: ids[0], HeldLease: current[ids[0]], NextAttemptAt: later})
	mustZero("reschedule after sent", n, err)
	n, err = q.MarkPushFailed(ctx, sqlcgen.MarkPushFailedParams{ID: ids[0], HeldLease: current[ids[0]]})
	mustZero("fail after sent", n, err)
	n, err = q.MarkPushSent(ctx, sqlcgen.MarkPushSentParams{ID: ids[0], HeldLease: current[ids[0]], Now: pushNow})
	mustZero("sent twice", n, err)
	n, err = q.MarkPushFailed(ctx, sqlcgen.MarkPushFailedParams{ID: ids[1], HeldLease: current[ids[1]]})
	mustZero("fail after skipped", n, err)

	var state string
	var unconfirmed bool
	if err := pool.QueryRow(ctx, `SELECT state, sent_unconfirmed FROM push_outbox WHERE id = $1`, ids[0]).Scan(&state, &unconfirmed); err != nil || state != "sent" || !unconfirmed {
		t.Errorf("row 0 = %q unconfirmed %v (%v), want sent and true", state, unconfirmed, err)
	}
	var lease *time.Time
	if err := pool.QueryRow(ctx, `SELECT state, lease_until FROM push_outbox WHERE id = $1`, ids[2]).Scan(&state, &lease); err != nil || state != "queued" || lease != nil {
		t.Errorf("rescheduled row = %q lease %v (%v), want queued with no lease", state, lease, err)
	}

	// Renewal: the current holder gets a later lease, and then the old value is stale.
	extended, err := q.ExtendPushLease(ctx, sqlcgen.ExtendPushLeaseParams{ID: ids[4], HeldLease: current[ids[4]], LeaseSeconds: 120})
	if err != nil || !extended.After(current[ids[4]]) {
		t.Fatalf("extend by the holder = %v (%v), want a later lease than %v", extended, err, current[ids[4]])
	}
	if _, err := q.ExtendPushLease(ctx, sqlcgen.ExtendPushLeaseParams{ID: ids[4], HeldLease: current[ids[4]], LeaseSeconds: 120}); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("extend with the replaced lease = %v, want no row", err)
	}
	n, err = q.MarkPushFailed(ctx, sqlcgen.MarkPushFailedParams{ID: ids[4], HeldLease: current[ids[4]]})
	mustZero("mark with the replaced lease", n, err)
	n, err = q.MarkPushFailed(ctx, sqlcgen.MarkPushFailedParams{ID: ids[4], HeldLease: extended})
	mustOne("mark with the extended lease", n, err)
}

func TestCountReporterFanouts(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	reporter := testutil.SeedReporterAccount(t, pool, "fan-reporter@example.com", "fan-sess-r")
	other := testutil.SeedReporterAccount(t, pool, "fan-other@example.com", "fan-sess-o")
	_ = other
	since := pushNow.Add(-time.Hour)
	inWindow := pushNow.Add(-10 * time.Minute)
	r1 := pushReport(t, pool, "fan-sess-r", pushFar)
	r2 := pushReport(t, pool, "fan-sess-r", pushFar)
	r3 := pushReport(t, pool, "fan-sess-r", pushFar)
	r4 := pushReport(t, pool, "fan-sess-o", pushFar)
	old := pushReport(t, pool, "fan-sess-r", pushFar)

	critical := func(report int64, state string, at time.Time) {
		if err := q.MarkCriticalFanout(ctx, sqlcgen.MarkCriticalFanoutParams{ReportID: report, State: state, Now: at}); err != nil {
			t.Fatalf("marking critical fan out: %v", err)
		}
	}
	reliable := func(report int64, state string, at time.Time) {
		if err := q.MarkReliableFanout(ctx, sqlcgen.MarkReliableFanoutParams{ReportID: report, State: state, Now: at}); err != nil {
			t.Fatalf("marking reliable fan out: %v", err)
		}
	}
	count := func(kind string, exclude int64) int64 {
		t.Helper()
		n, err := q.CountReporterFanouts(ctx, sqlcgen.CountReporterFanoutsParams{
			ReporterAccountID: reporter, Kind: kind, Since: since, ExcludeReportID: exclude})
		if err != nil {
			t.Fatalf("counting fan outs: %v", err)
		}
		return n
	}

	critical(r1, "fanned", inWindow)
	critical(r2, "fanned", inWindow)
	critical(r3, "capped", inWindow)
	critical(r4, "fanned", inWindow)
	critical(old, "fanned", since.Add(-time.Minute))
	if n := count("critical", 0); n != 2 {
		t.Errorf("critical count = %d, want 2 (a capped row, another reporter and an old row do not count)", n)
	}

	// Sticky states.
	critical(r3, "fanned", inWindow)
	critical(r1, "capped", inWindow)
	for report, want := range map[int64]string{r3: "capped", r1: "fanned"} {
		p, err := q.GetPushReportProgress(ctx, report)
		if err != nil || p.CriticalState == nil || *p.CriticalState != want {
			t.Errorf("report %d critical state = %v (%v), want %s", report, p.CriticalState, err, want)
		}
	}
	if n := count("critical", 0); n != 2 {
		t.Errorf("critical count after the overwrite attempts = %d, want 2", n)
	}

	// Exclusion (N2): a re run of r1 does not count itself.
	if n := count("critical", r1); n != 1 {
		t.Errorf("critical count excluding r1 = %d, want 1", n)
	}

	// Once across kinds (N4): r1 fanned under both counts only for critical; r5
	// fanned only under reliable counts for reliable.
	reliable(r1, "fanned", inWindow)
	r5 := pushReport(t, pool, "fan-sess-r", pushFar)
	reliable(r5, "fanned", inWindow)
	if n := count("critical", 0); n != 2 {
		t.Errorf("critical count with a double fanned report = %d, want 2", n)
	}
	if n := count("reliable", 0); n != 1 {
		t.Errorf("reliable count = %d, want 1 (r1 already holds its Critical slot)", n)
	}
	if n := count("reliable", r5); n != 0 {
		t.Errorf("reliable count excluding r5 = %d, want 0", n)
	}
}

func TestPushReportProgressAndRecent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	testutil.SeedReporterAccount(t, pool, "prog@example.com", "prog-sess")
	live := pushReport(t, pool, "prog-sess", pushNow.Add(2*time.Hour))
	expired := pushReport(t, pool, "prog-sess", pushNow.Add(-time.Hour))

	if _, err := q.GetPushReportProgress(ctx, live); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("progress of an unseen report = %v, want no row", err)
	}
	if err := q.MarkAreaLookupTried(ctx, sqlcgen.MarkAreaLookupTriedParams{ReportID: live, Now: pushNow}); err != nil {
		t.Fatalf("area lookup mark: %v", err)
	}
	if err := q.MarkReportEvaluated(ctx, sqlcgen.MarkReportEvaluatedParams{ReportID: live, Now: pushNow.Add(time.Minute)}); err != nil {
		t.Fatalf("evaluated mark: %v", err)
	}
	p, err := q.GetPushReportProgress(ctx, live)
	if err != nil || !p.AreaLookupAt.Valid || !p.EvaluatedAt.Valid || p.CriticalState != nil || p.ReliableState != nil {
		t.Errorf("progress = %+v (%v), want lookup and evaluation stamped, no fan out", p, err)
	}

	ids, err := q.RecentReportsForPush(ctx, sqlcgen.RecentReportsForPushParams{
		Since: pushNow.Add(-24 * time.Hour), Now: pushNow, RowCap: 10})
	if err != nil || !reflect.DeepEqual(ids, []int64{live}) {
		t.Errorf("recent reports = %v (%v), want only %d (not the expired %d)", ids, err, live, expired)
	}
}

// TestRecentReportsForPushServesTheLeastRecentlyEvaluated (review S-04,
// 2026-10-25): the recovery sweep used to take the same newest row_cap reports
// every time, so an older report past the cap was never evaluated. A report
// never evaluated now comes first, then the one evaluated longest ago, then
// the newest.
func TestRecentReportsForPushServesTheLeastRecentlyEvaluated(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	testutil.SeedReporterAccount(t, pool, "rot@example.com", "rot-sess")
	oldest := pushReport(t, pool, "rot-sess", pushFar)
	middle := pushReport(t, pool, "rot-sess", pushFar)
	newest := pushReport(t, pool, "rot-sess", pushFar)
	for i, id := range []int64{oldest, middle, newest} {
		testutil.MustExec(t, pool, `UPDATE reports SET created_at = $1 WHERE id = $2`,
			pushNow.Add(time.Duration(i-3)*time.Minute), id)
	}
	recent := func(rowCap int32) []int64 {
		t.Helper()
		ids, err := q.RecentReportsForPush(ctx, sqlcgen.RecentReportsForPushParams{
			Since: pushNow.Add(-time.Hour), Now: pushNow, RowCap: rowCap})
		if err != nil {
			t.Fatalf("recent reports: %v", err)
		}
		return ids
	}
	evaluated := func(id int64, at time.Time) {
		t.Helper()
		if err := q.MarkReportEvaluated(ctx, sqlcgen.MarkReportEvaluatedParams{ReportID: id, Now: at}); err != nil {
			t.Fatalf("evaluated mark: %v", err)
		}
	}

	// Nothing evaluated yet: newest first, as before.
	if got, want := recent(10), []int64{newest, middle, oldest}; !reflect.DeepEqual(got, want) {
		t.Errorf("unevaluated order = %v, want %v", got, want)
	}
	// The two newest were evaluated by a capped sweep: the oldest, never
	// evaluated, now comes first and fits under a cap of 1.
	evaluated(newest, pushNow.Add(-2*time.Minute))
	evaluated(middle, pushNow.Add(-2*time.Minute))
	if got, want := recent(1), []int64{oldest}; !reflect.DeepEqual(got, want) {
		t.Errorf("capped at 1 = %v, want the never evaluated %v", got, want)
	}
	// Once every report has been evaluated, the one evaluated longest ago leads,
	// and a tie falls back to newest first.
	evaluated(oldest, pushNow.Add(-time.Minute))
	if got, want := recent(10), []int64{newest, middle, oldest}; !reflect.DeepEqual(got, want) {
		t.Errorf("all evaluated order = %v, want %v", got, want)
	}
	evaluated(newest, pushNow)
	if got, want := recent(2), []int64{middle, oldest}; !reflect.DeepEqual(got, want) {
		t.Errorf("capped at 2 after newest re-evaluated = %v, want %v", got, want)
	}
}

func TestPushOutboxWindowCountAndPrune(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	a := pushPerson(t, pool, "win-a@example.com", "win-sess-a")
	r := pushReport(t, pool, "win-sess-a", pushFar)
	r2 := pushReport(t, pool, "win-sess-a", pushFar)
	pushEnqueue(t, q, a, r, "critical")
	pushEnqueue(t, q, a, r2, "reliable")
	testutil.MustExec(t, pool,
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at, state, sent_at, created_at)
		 VALUES ($1, $2, 'confirmed_update', $3, 'sent', $4, $5)`, a, r, pushFar, pushNow.Add(-time.Minute), pushNow.Add(-time.Minute))
	for _, row := range pushClaim(t, q, 10) {
		if row.Kind == "confirmed_update" {
			continue
		}
		if n, err := q.MarkPushSent(ctx, sqlcgen.MarkPushSentParams{ID: row.ID, HeldLease: row.LeaseUntil, Now: pushNow.Add(-5 * time.Minute)}); err != nil || n != 1 {
			t.Fatalf("mark sent = %d (%v)", n, err)
		}
	}
	// Both initial rows were sent 5 minutes before pushNow; the confirmed update
	// does not count.
	n, err := q.CountAccountPushesInWindow(ctx, sqlcgen.CountAccountPushesInWindowParams{AccountID: a, Since: pushNow.Add(-time.Hour)})
	if err != nil || n != 2 {
		t.Errorf("pushes in the window = %d (%v), want 2 (confirmed updates do not count)", n, err)
	}
	n, err = q.CountAccountPushesInWindow(ctx, sqlcgen.CountAccountPushesInWindowParams{AccountID: a, Since: pushNow.Add(-time.Minute)})
	if err != nil || n != 0 {
		t.Errorf("pushes in a later window = %d (%v), want 0", n, err)
	}

	// Prune removes finished rows created at or before the edge, never queued ones.
	r3 := pushReport(t, pool, "win-sess-a", pushFar)
	queued := pushEnqueue(t, q, a, r3, "critical")
	testutil.MustExec(t, pool, `UPDATE push_outbox SET lease_until = NULL WHERE id = $1`, queued)
	if err := q.PrunePushOutbox(ctx, pushNow.Add(time.Hour)); err != nil {
		t.Fatalf("pruning: %v", err)
	}
	if n := pushCount(t, pool, `SELECT count(*) FROM push_outbox`); n != 1 {
		t.Errorf("%d rows left after the prune, want only the queued one", n)
	}
}

func TestSkipStalePushes(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	held := pushPerson(t, pool, "stale-held@example.com", "stale-sess-held")
	expiredRow := pushPerson(t, pool, "stale-row@example.com", "stale-sess-row")
	expiredReport := pushPerson(t, pool, "stale-report@example.com", "stale-sess-report")
	sentRow := pushPerson(t, pool, "stale-sent@example.com", "stale-sess-sent")
	fresh := pushPerson(t, pool, "stale-fresh@example.com", "stale-sess-fresh")
	rLive := pushReport(t, pool, "stale-sess-held", pushNow.Add(time.Hour))
	rGone := pushReport(t, pool, "stale-sess-held", pushNow.Add(-time.Minute))

	insert := func(account, report int64, state string, expires time.Time) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx,
			`INSERT INTO push_outbox (account_id, report_id, kind, state, expires_at, created_at) VALUES ($1, $2, 'critical', $3, $4, $5) RETURNING id`,
			account, report, state, expires, pushNow.Add(-2*time.Hour)).Scan(&id); err != nil {
			t.Fatalf("inserting outbox row: %v", err)
		}
		return id
	}
	stateOf := func(id int64) (string, *string) {
		t.Helper()
		var state string
		var reason *string
		if err := pool.QueryRow(ctx, `SELECT state, skip_reason FROM push_outbox WHERE id = $1`, id).Scan(&state, &reason); err != nil {
			t.Fatalf("reading row %d: %v", id, err)
		}
		return state, reason
	}

	// A claim holds this row while it is already past its life.
	heldID := insert(held, rLive, "queued", pushNow.Add(-time.Minute))
	if rows := pushClaim(t, q, 10); len(rows) != 1 || rows[0].ID != heldID {
		t.Fatalf("setup claim = %+v, want the held row only", rows)
	}
	rowID := insert(expiredRow, rLive, "queued", pushNow.Add(-time.Minute))
	reportID := insert(expiredReport, rGone, "queued", pushFar)
	sentID := insert(sentRow, rLive, "sent", pushNow.Add(-time.Minute))
	freshID := insert(fresh, rLive, "queued", pushFar)

	n, err := q.SkipStalePushes(ctx, pushNow)
	if err != nil || n != 2 {
		t.Fatalf("skip stale = %d (%v), want 2", n, err)
	}
	for _, id := range []int64{rowID, reportID} {
		if state, reason := stateOf(id); state != "skipped" || reason == nil || *reason != "stale" {
			t.Errorf("row %d = %q %v, want skipped with reason stale", id, state, reason)
		}
	}
	for _, id := range []int64{sentID, freshID, heldID} {
		if state, _ := stateOf(id); state == "skipped" {
			t.Errorf("row %d was skipped, want it untouched", id)
		}
	}
	if state, _ := stateOf(sentID); state != "sent" {
		t.Errorf("the sent row is %q, want sent", state)
	}

	// Once the held lease has passed, the expired row is skipped as well.
	testutil.MustExec(t, pool, `UPDATE push_outbox SET lease_until = $2 WHERE id = $1`, heldID, pushNow.Add(-time.Second))
	if n, err := q.SkipStalePushes(ctx, pushNow); err != nil || n != 1 {
		t.Errorf("skip stale after the lease passed = %d (%v), want 1", n, err)
	}
	if state, reason := stateOf(heldID); state != "skipped" || reason == nil || *reason != "stale" {
		t.Errorf("held row = %q %v, want skipped stale", state, reason)
	}
}

func TestPushReportFactsAreaName(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	reporter := testutil.SeedReporterAccount(t, pool, "facts@example.com", "facts-sess")
	bound := pushReport(t, pool, "facts-sess", pushFar)
	unbound := pushReport(t, pool, "facts-unbound-session", pushFar)

	facts, err := q.PushReportFacts(ctx, bound)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}
	if facts.AreaName != nil {
		t.Errorf("area name before the lookup = %q, want NULL", *facts.AreaName)
	}
	if facts.ReporterAccountID == nil || *facts.ReporterAccountID != reporter {
		t.Errorf("reporter = %v, want %d from the session binding", facts.ReporterAccountID, reporter)
	}
	if facts.Latitude == 0 || facts.Longitude == 0 || facts.ModerationState != "clear" {
		t.Errorf("facts = %+v, want the position and a clear report", facts)
	}

	if err := q.SetReportAreaName(ctx, sqlcgen.SetReportAreaNameParams{ReportID: bound, AreaName: "Benz Circle"}); err != nil {
		t.Fatalf("setting the area name: %v", err)
	}
	facts, err = q.PushReportFacts(ctx, bound)
	if err != nil || facts.AreaName == nil || *facts.AreaName != "Benz Circle" {
		t.Errorf("area name after the lookup = %v (%v), want Benz Circle", facts.AreaName, err)
	}

	other, err := q.PushReportFacts(ctx, unbound)
	if err != nil || other.ReporterAccountID != nil {
		t.Errorf("unbound reporter = %v (%v), want NULL", other.ReporterAccountID, err)
	}
}

func TestPushAccountHomeSwitchesAndMute(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	a := testutil.SeedReporterAccount(t, pool, "home@example.com", "home-sess")
	testutil.SetHomeCell(t, pool, a, "Benz Circle", "tdr1v")
	r1 := pushReport(t, pool, "home-sess", pushFar)
	r2 := pushReport(t, pool, "home-sess", pushFar)

	got, err := q.PushAccountHome(ctx, sqlcgen.PushAccountHomeParams{AccountID: a, ReportID: r1})
	if err != nil {
		t.Fatalf("home read: %v", err)
	}
	if got.HomeAreaCell == nil || *got.HomeAreaCell != "tdr1v" || got.AlertRadiusKm != 5 ||
		!got.CriticalPushOn || got.ChangesPushOn || got.ReliableAlertsOn || got.Muted {
		t.Errorf("defaults = %+v, want cell tdr1v, 5 km, critical on, changes off, reliable off, not muted", got)
	}

	if _, err := q.UpsertPushKindSettings(ctx, sqlcgen.UpsertPushKindSettingsParams{
		AccountID: a, CriticalPushOn: false, ChangesPushOn: true, Now: pushNow}); err != nil {
		t.Fatalf("settings: %v", err)
	}
	testutil.MustExec(t, pool, `INSERT INTO report_mutes (account_id, report_id) VALUES ($1, $2)`, a, r1)

	got, err = q.PushAccountHome(ctx, sqlcgen.PushAccountHomeParams{AccountID: a, ReportID: r1})
	if err != nil || got.CriticalPushOn || !got.ChangesPushOn || !got.Muted {
		t.Errorf("after the switches and the mute = %+v (%v), want critical off, changes on, muted", got, err)
	}
	other, err := q.PushAccountHome(ctx, sqlcgen.PushAccountHomeParams{AccountID: a, ReportID: r2})
	if err != nil || other.Muted {
		t.Errorf("another report = %+v (%v), want not muted", other, err)
	}
}
