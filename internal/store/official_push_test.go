package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// officialPushNow is the fixed instant of the official push query tests. The
// claim and the lease use the database clock, so every time passed to a claimed
// row is relative to that clock, not to this value.
var officialPushNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// officialPushPerson seeds an account with a live device and a home cell.
func officialPushPerson(t *testing.T, pool *pgxpool.Pool, email, sessionID, cell string) int64 {
	t.Helper()
	id := testutil.SeedReporterAccount(t, pool, email, sessionID)
	testutil.SeedPushSubscription(t, pool, id, sessionID, "")
	if cell != "" {
		testutil.SetHomeCell(t, pool, id, "Home", cell)
	}
	return id
}

func officialPushEnqueue(t *testing.T, q *sqlcgen.Queries, accountID, alertID int64, capID, reason string, expires time.Time) int64 {
	t.Helper()
	n, err := q.EnqueueOfficialPush(context.Background(), sqlcgen.EnqueueOfficialPushParams{
		AccountID: accountID, AlertID: alertID, CapIdentifier: capID, Reason: reason,
		ExpiresAt: expires, Now: officialPushNow,
	})
	if err != nil {
		t.Fatalf("enqueue for account %d alert %d: %v", accountID, alertID, err)
	}
	return n
}

// TestOfficialPushQueries covers the plan 05-11 statements on a real database:
// the switch default, the recipient candidate read, the once only enqueue, the
// lease conditional claim and marks, the window count, the sent for alert read,
// the stale skip, the prune, the alert facts and the account delete.
func TestOfficialPushQueries(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	far := time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("the switch defaults to on and the upsert keeps other columns", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := testutil.SeedReporterAccount(t, pool, "op-setting@example.com", "op-setting-sess")

		on, err := q.GetOfficialPushSetting(ctx, a)
		if err != nil || !on {
			t.Fatalf("an account with no settings row reads %v (%v), want true", on, err)
		}
		got, err := q.UpsertOfficialPushSetting(ctx, sqlcgen.UpsertOfficialPushSettingParams{
			AccountID: a, OfficialPushOn: false, Now: officialPushNow})
		if err != nil || got {
			t.Fatalf("turning the switch off = %v (%v), want false", got, err)
		}
		if on, _ := q.GetOfficialPushSetting(ctx, a); on {
			t.Error("the switch reads on after it was saved off")
		}
		// The 3.3 switches keep their defaults when this upsert inserts the row.
		kinds, err := q.GetPushKindSettings(ctx, a)
		if err != nil || !kinds.CriticalPushOn || kinds.ChangesPushOn {
			t.Errorf("3.3 switches = critical %v changes %v (%v), want true and false", kinds.CriticalPushOn, kinds.ChangesPushOn, err)
		}
		// Saving a 3.3 switch afterwards leaves the official switch alone.
		if _, err := q.UpsertPushKindSettings(ctx, sqlcgen.UpsertPushKindSettingsParams{
			AccountID: a, CriticalPushOn: true, ChangesPushOn: true, Now: officialPushNow}); err != nil {
			t.Fatalf("UpsertPushKindSettings: %v", err)
		}
		if on, _ := q.GetOfficialPushSetting(ctx, a); on {
			t.Error("saving a 3.3 switch turned the official switch back on")
		}
		if got, err := q.UpsertOfficialPushSetting(ctx, sqlcgen.UpsertOfficialPushSettingParams{
			AccountID: a, OfficialPushOn: true, Now: officialPushNow}); err != nil || !got {
			t.Fatalf("turning the switch on = %v (%v), want true", got, err)
		}
		kinds, _ = q.GetPushKindSettings(ctx, a)
		if !kinds.ChangesPushOn {
			t.Error("saving the official switch turned Changes off")
		}
	})

	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box,
	// D2): the stored official switch is no longer a candidate condition, so the
	// account with the switch saved off is returned like the others.
	t.Run("candidates need a home cell, a live device and no deletion, and no stored switch", func(t *testing.T) {
		testutil.Truncate(t, pool)
		ok := officialPushPerson(t, pool, "op-ok@example.com", "op-ok-sess", "tf3bq")
		noRow := officialPushPerson(t, pool, "op-ok2@example.com", "op-ok2-sess", "tf3bq")
		off := officialPushPerson(t, pool, "op-off@example.com", "op-off-sess", "tf3bq")
		if _, err := q.UpsertOfficialPushSetting(ctx, sqlcgen.UpsertOfficialPushSettingParams{
			AccountID: off, OfficialPushOn: false, Now: officialPushNow}); err != nil {
			t.Fatalf("switch off: %v", err)
		}
		_ = officialPushPerson(t, pool, "op-nohome@example.com", "op-nohome-sess", "")
		noDevice := testutil.SeedReporterAccount(t, pool, "op-nodevice@example.com", "op-nodevice-sess")
		testutil.SetHomeCell(t, pool, noDevice, "Home", "tf3bq")
		deleted := officialPushPerson(t, pool, "op-deleted@example.com", "op-deleted-sess", "tf3bq")
		testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, deleted)
		revoked := officialPushPerson(t, pool, "op-revoked@example.com", "op-revoked-sess", "tf3bq")
		testutil.MustExec(t, pool, `UPDATE sessions SET revoked_at = now() WHERE session_id = 'op-revoked-sess'`)

		// Amended 2026-10-29 by the Phase 5 review fix (WR-S05): the read now takes
		// the geohash prefixes of the alert's area; every old case is kept.
		area := []string{"tf3b"}
		rows, err := q.OfficialPushCandidates(ctx, sqlcgen.OfficialPushCandidatesParams{PrefixLen: 4, Prefixes: area, RowCap: 100})
		if err != nil {
			t.Fatalf("OfficialPushCandidates: %v", err)
		}
		if len(rows) != 3 || rows[0].AccountID != ok || rows[1].AccountID != noRow || rows[2].AccountID != off {
			t.Fatalf("candidates = %+v, want accounts %d, %d and %d (switch off) in id order (deleted %d, revoked %d must be absent)",
				rows, ok, noRow, off, deleted, revoked)
		}
		if rows[0].HomeAreaCell != "tf3bq" {
			t.Errorf("home cell = %q, want tf3bq", rows[0].HomeAreaCell)
		}
		one, err := q.OfficialPushCandidates(ctx, sqlcgen.OfficialPushCandidatesParams{PrefixLen: 4, Prefixes: area, RowCap: 1})
		if err != nil || len(one) != 1 || one[0].AccountID != ok {
			t.Errorf("a row cap of 1 gave %+v (%v), want the lowest account id", one, err)
		}
		if far, err := q.OfficialPushCandidates(ctx, sqlcgen.OfficialPushCandidatesParams{PrefixLen: 4, Prefixes: []string{"tdr1"}, RowCap: 100}); err != nil || len(far) != 0 {
			t.Errorf("prefixes of another area gave %+v (%v), want none", far, err)
		}
	})

	t.Run("enqueue is once per person, alert and version", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := officialPushPerson(t, pool, "op-enq@example.com", "op-enq-sess", "tf3bq")
		alert := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{Severity: "Severe", Tier: "district", DistrictCodes: []int{749}})

		if n := officialPushEnqueue(t, q, a, alert, "IN-x_1", "new", far); n != 1 {
			t.Fatalf("first enqueue added %d rows, want 1", n)
		}
		if n := officialPushEnqueue(t, q, a, alert, "IN-x_1", "new", far); n != 0 {
			t.Errorf("a repeat enqueue added %d rows, want 0", n)
		}
		if n := officialPushEnqueue(t, q, a, alert, "IN-x_2", "escalated", far); n != 1 {
			t.Errorf("a second version added %d rows, want 1", n)
		}
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM official_push_outbox`).Scan(&count); err != nil || count != 2 {
			t.Errorf("%d rows (%v), want 2", count, err)
		}
	})

	t.Run("a claimed row's marks need the held lease", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := officialPushPerson(t, pool, "op-claim@example.com", "op-claim-sess", "tf3bq")
		alert := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{Severity: "Severe", Tier: "district", DistrictCodes: []int{749}})
		for _, v := range []string{"IN-c_1", "IN-c_2", "IN-c_3", "IN-c_4"} {
			officialPushEnqueue(t, q, a, alert, v, "new", far)
		}
		// next_attempt_at is officialPushNow, long past, so all four are due.
		claimed, err := q.ClaimDueOfficialPushes(ctx, sqlcgen.ClaimDueOfficialPushesParams{BatchLimit: 10, LeaseSeconds: 60})
		if err != nil || len(claimed) != 4 {
			t.Fatalf("claim = %d rows (%v), want 4", len(claimed), err)
		}
		for _, c := range claimed {
			if c.Attempts != 1 || !c.LeaseUntil.After(time.Now()) {
				t.Errorf("claimed row %d: attempts %d lease %v, want 1 and a future lease", c.ID, c.Attempts, c.LeaseUntil)
			}
		}
		if again, _ := q.ClaimDueOfficialPushes(ctx, sqlcgen.ClaimDueOfficialPushesParams{BatchLimit: 10, LeaseSeconds: 60}); len(again) != 0 {
			t.Errorf("a second claim took %d rows while the leases hold, want 0", len(again))
		}

		wrong := claimed[0].LeaseUntil.Add(-time.Hour)
		if n, err := q.MarkOfficialPushSent(ctx, sqlcgen.MarkOfficialPushSentParams{ID: claimed[0].ID, HeldLease: wrong, Now: officialPushNow}); err != nil || n != 0 {
			t.Errorf("MarkOfficialPushSent with a wrong lease changed %d rows (%v), want 0", n, err)
		}
		if n, err := q.MarkOfficialPushSkipped(ctx, sqlcgen.MarkOfficialPushSkippedParams{ID: claimed[0].ID, HeldLease: wrong, SkipReason: "stale"}); err != nil || n != 0 {
			t.Errorf("MarkOfficialPushSkipped with a wrong lease changed %d rows (%v), want 0", n, err)
		}
		if n, err := q.RescheduleOfficialPush(ctx, sqlcgen.RescheduleOfficialPushParams{ID: claimed[0].ID, HeldLease: wrong, NextAttemptAt: far}); err != nil || n != 0 {
			t.Errorf("RescheduleOfficialPush with a wrong lease changed %d rows (%v), want 0", n, err)
		}
		if n, err := q.MarkOfficialPushFailed(ctx, sqlcgen.MarkOfficialPushFailedParams{ID: claimed[0].ID, HeldLease: wrong}); err != nil || n != 0 {
			t.Errorf("MarkOfficialPushFailed with a wrong lease changed %d rows (%v), want 0", n, err)
		}
		if _, err := q.ExtendOfficialPushLease(ctx, sqlcgen.ExtendOfficialPushLeaseParams{ID: claimed[0].ID, HeldLease: wrong, LeaseSeconds: 60}); err == nil {
			t.Error("ExtendOfficialPushLease with a wrong lease returned a row, want none")
		}

		// The right lease works, once.
		if n, err := q.MarkOfficialPushSent(ctx, sqlcgen.MarkOfficialPushSentParams{ID: claimed[0].ID, HeldLease: claimed[0].LeaseUntil, Now: officialPushNow}); err != nil || n != 1 {
			t.Errorf("MarkOfficialPushSent with the held lease changed %d rows (%v), want 1", n, err)
		}
		if n, _ := q.MarkOfficialPushFailed(ctx, sqlcgen.MarkOfficialPushFailedParams{ID: claimed[0].ID, HeldLease: claimed[0].LeaseUntil}); n != 0 {
			t.Errorf("a sent row was failed afterwards (%d rows)", n)
		}
		if n, err := q.MarkOfficialPushSkipped(ctx, sqlcgen.MarkOfficialPushSkippedParams{ID: claimed[1].ID, HeldLease: claimed[1].LeaseUntil, SkipReason: "already_notified"}); err != nil || n != 1 {
			t.Errorf("MarkOfficialPushSkipped = %d (%v), want 1", n, err)
		}
		if n, err := q.RescheduleOfficialPush(ctx, sqlcgen.RescheduleOfficialPushParams{ID: claimed[2].ID, HeldLease: claimed[2].LeaseUntil, NextAttemptAt: far}); err != nil || n != 1 {
			t.Errorf("RescheduleOfficialPush = %d (%v), want 1", n, err)
		}
		if n, err := q.MarkOfficialPushFailed(ctx, sqlcgen.MarkOfficialPushFailedParams{ID: claimed[3].ID, HeldLease: claimed[3].LeaseUntil}); err != nil || n != 1 {
			t.Errorf("MarkOfficialPushFailed = %d (%v), want 1", n, err)
		}
		if _, err := q.ExtendOfficialPushLease(ctx, sqlcgen.ExtendOfficialPushLeaseParams{ID: claimed[2].ID, HeldLease: claimed[2].LeaseUntil, LeaseSeconds: 60}); err == nil {
			t.Error("a rescheduled row (lease cleared) still extended its lease")
		}
	})

	t.Run("an expired lease lets another claimant take the row", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := officialPushPerson(t, pool, "op-lapse@example.com", "op-lapse-sess", "tf3bq")
		alert := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{Severity: "Severe", Tier: "district", DistrictCodes: []int{749}})
		officialPushEnqueue(t, q, a, alert, "IN-l_1", "new", far)
		first, err := q.ClaimDueOfficialPushes(ctx, sqlcgen.ClaimDueOfficialPushesParams{BatchLimit: 10, LeaseSeconds: 60})
		if err != nil || len(first) != 1 {
			t.Fatalf("first claim = %d rows (%v), want 1", len(first), err)
		}
		testutil.MustExec(t, pool, `UPDATE official_push_outbox SET lease_until = now() - interval '1 second'`)
		second, err := q.ClaimDueOfficialPushes(ctx, sqlcgen.ClaimDueOfficialPushesParams{BatchLimit: 10, LeaseSeconds: 60})
		if err != nil || len(second) != 1 || second[0].Attempts != 2 {
			t.Fatalf("claim after the lease lapsed = %+v (%v), want one row with attempts 2", second, err)
		}
		if n, _ := q.MarkOfficialPushSent(ctx, sqlcgen.MarkOfficialPushSentParams{ID: first[0].ID, HeldLease: first[0].LeaseUntil, Now: officialPushNow}); n != 0 {
			t.Error("the first claimant marked a row it lost")
		}
	})

	t.Run("the window count, the sent for alert read, the stale skip and the prune", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := officialPushPerson(t, pool, "op-count@example.com", "op-count-sess", "tf3bq")
		b := officialPushPerson(t, pool, "op-count-b@example.com", "op-count-b-sess", "tf3bq")
		alert := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{Severity: "Severe", Tier: "district", DistrictCodes: []int{749}})
		other := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{Severity: "Severe", Tier: "district", DistrictCodes: []int{749}})

		// Sent rows: two inside the window, one outside, one for another account.
		testutil.MustExec(t, pool,
			`INSERT INTO official_push_outbox (account_id, alert_id, cap_identifier, reason, state, sent_at, created_at, expires_at)
			 VALUES ($1, $2, 'IN-w_1', 'new', 'sent', $4, $4, $5),
			        ($1, $3, 'IN-w_2', 'new', 'sent', $6, $6, $5),
			        ($1, $3, 'IN-w_3', 'escalated', 'sent', $7, $7, $5),
			        ($8, $2, 'IN-w_4', 'new', 'sent', $6, $6, $5)`,
			a, alert, other, officialPushNow.Add(-5*time.Hour), far, officialPushNow.Add(-time.Hour), officialPushNow.Add(-7*time.Hour), b)
		// A skipped row and a failed row never count.
		testutil.MustExec(t, pool,
			`INSERT INTO official_push_outbox (account_id, alert_id, cap_identifier, reason, state, skip_reason, created_at, expires_at)
			 VALUES ($1, $2, 'IN-w_5', 'new', 'skipped', 'cap_account', $3, $4)`, a, alert, officialPushNow, far)
		n, err := q.CountAccountOfficialPushesInWindow(ctx, sqlcgen.CountAccountOfficialPushesInWindowParams{
			AccountID: a, Since: officialPushNow.Add(-6 * time.Hour)})
		if err != nil || n != 2 {
			t.Errorf("sent rows in the 6 hour window = %d (%v), want 2", n, err)
		}
		if n, _ := q.CountAccountOfficialPushesInWindow(ctx, sqlcgen.CountAccountOfficialPushesInWindowParams{
			AccountID: a, Since: officialPushNow.Add(-30 * time.Minute)}); n != 0 {
			t.Errorf("sent rows in a later window = %d, want 0", n)
		}

		sent, err := q.OfficialPushSentForAlert(ctx, sqlcgen.OfficialPushSentForAlertParams{AccountID: a, AlertID: alert})
		if err != nil || !sent {
			t.Errorf("sent for the alert = %v (%v), want true", sent, err)
		}
		sent, _ = q.OfficialPushSentForAlert(ctx, sqlcgen.OfficialPushSentForAlertParams{AccountID: b, AlertID: other})
		if sent {
			t.Error("sent for an alert nothing was sent about, want false")
		}

		// Stale skip: one row past its own end, one whose alert was cancelled,
		// one whose alert was superseded, one that is fine, one held by a worker.
		cancelled := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{Severity: "Severe", Tier: "district", DistrictCodes: []int{749}})
		testutil.MustExec(t, pool, `UPDATE official_alerts SET cancelled_at = $2 WHERE id = $1`, cancelled, officialPushNow)
		superseded := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{Severity: "Severe", Tier: "district", DistrictCodes: []int{749}})
		testutil.MustExec(t, pool, `UPDATE official_alerts SET superseded_at = $2 WHERE id = $1`, superseded, officialPushNow)
		testutil.MustExec(t, pool,
			`INSERT INTO official_push_outbox (account_id, alert_id, cap_identifier, reason, expires_at, lease_until, created_at)
			 VALUES ($1, $2, 'IN-s_over', 'new', $5, NULL, $9),
			        ($1, $3, 'IN-s_cancelled', 'new', $6, NULL, $9),
			        ($1, $4, 'IN-s_superseded', 'new', $6, NULL, $9),
			        ($1, $2, 'IN-s_fine', 'new', $6, NULL, $9),
			        ($7, $3, 'IN-s_held', 'new', $6, $8, $9)`,
			a, alert, cancelled, superseded, officialPushNow.Add(-time.Minute), far, b, officialPushNow.Add(time.Minute), officialPushNow)
		skipped, err := q.SkipStaleOfficialPushes(ctx, officialPushNow)
		if err != nil || skipped != 3 {
			t.Errorf("SkipStaleOfficialPushes changed %d rows (%v), want 3 (own end, cancelled, superseded)", skipped, err)
		}
		var stale int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM official_push_outbox WHERE skip_reason = 'stale' AND state = 'skipped'`).Scan(&stale); err != nil || stale != 3 {
			t.Errorf("%d stale rows (%v), want 3", stale, err)
		}
		var fine string
		if err := pool.QueryRow(ctx, `SELECT state FROM official_push_outbox WHERE cap_identifier = 'IN-s_fine'`).Scan(&fine); err != nil || fine != "queued" {
			t.Errorf("the fine row is %q (%v), want queued", fine, err)
		}
		if err := pool.QueryRow(ctx, `SELECT state FROM official_push_outbox WHERE cap_identifier = 'IN-s_held'`).Scan(&fine); err != nil || fine != "queued" {
			t.Errorf("the row a worker holds is %q (%v), want queued", fine, err)
		}

		// Prune removes finished rows created at or before the edge, never queued ones.
		if err := q.PruneOfficialPushOutbox(ctx, officialPushNow.Add(time.Hour)); err != nil {
			t.Fatalf("PruneOfficialPushOutbox: %v", err)
		}
		var left int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM official_push_outbox`).Scan(&left); err != nil || left != 2 {
			t.Errorf("%d rows after the prune (%v), want the 2 queued rows", left, err)
		}
	})

	t.Run("the alert facts carry district codes and state codes apart", func(t *testing.T) {
		testutil.Truncate(t, pool)
		id := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
			Severity: "Extreme", Tier: "district", Event: "Severe Cyclonic Storm", Sender: "IMD-Chennai",
			DistrictCodes: []int{749, 750}, StateCodes: []int{28}})
		row, err := q.OfficialAlertForPush(ctx, id)
		if err != nil {
			t.Fatalf("OfficialAlertForPush: %v", err)
		}
		if row.Severity != "Extreme" || row.PlacementTier != "district" || row.Sender != "IMD-Chennai" || row.Event != "Severe Cyclonic Storm" {
			t.Errorf("row = %+v", row)
		}
		if len(row.DistrictCodes) != 2 || row.DistrictCodes[0] != 749 || row.DistrictCodes[1] != 750 {
			t.Errorf("district codes = %v, want [749 750] (state rows must not leak in)", row.DistrictCodes)
		}
		if len(row.StateCodes) != 1 || row.StateCodes[0] != 28 {
			t.Errorf("state codes = %v, want [28]", row.StateCodes)
		}
		if row.CancelledAt.Valid || row.SupersededAt.Valid || row.LastSeenAt.IsZero() || row.SentAt.IsZero() {
			t.Errorf("times = cancelled %v superseded %v last_seen %v sent %v", row.CancelledAt, row.SupersededAt, row.LastSeenAt, row.SentAt)
		}
		bare := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{Tier: "unplaced"})
		row, err = q.OfficialAlertForPush(ctx, bare)
		if err != nil || len(row.DistrictCodes) != 0 || len(row.StateCodes) != 0 {
			t.Errorf("an alert with no areas = %+v (%v), want empty code lists", row, err)
		}
	})

	t.Run("predecessors and withdrawn by a stored message", func(t *testing.T) {
		testutil.Truncate(t, pool)
		first := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
			GUID: "1792000000000401", CapIdentifier: "IN-1792000000000401_1", Severity: "Severe", Tier: "district",
			DistrictCodes: []int{749}, StateCodes: []int{28}})
		testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
			GUID: "1792000000000402", CapIdentifier: "IN-1792000000000402_1", Severity: "Extreme", Tier: "state", StateCodes: []int{28}})

		rows, err := q.OfficialPredecessorsForPush(ctx, sqlcgen.OfficialPredecessorsForPushParams{
			Identifiers: []string{"IN-1792000000000401_1", "IN-nothing_1"}, Guid: "1792000000000999"})
		if err != nil || len(rows) != 1 || rows[0].ID != first || rows[0].Severity != "Severe" ||
			len(rows[0].DistrictCodes) != 1 || rows[0].DistrictCodes[0] != 749 || len(rows[0].StateCodes) != 1 {
			t.Fatalf("predecessors = %+v (%v), want the one Severe alert with district 749 and state 28", rows, err)
		}
		// An alert never counts as its own predecessor.
		rows, err = q.OfficialPredecessorsForPush(ctx, sqlcgen.OfficialPredecessorsForPushParams{
			Identifiers: []string{"IN-1792000000000401_1"}, Guid: "1792000000000401"})
		if err != nil || len(rows) != 0 {
			t.Errorf("an alert listed itself as a predecessor: %+v (%v)", rows, err)
		}

		withdrawn, err := q.OfficialAlertWithdrawnByStored(ctx, sqlcgen.OfficialAlertWithdrawnByStoredParams{
			Guid: "1792000000000401", CapIdentifier: "IN-1792000000000401_1"})
		if err != nil || withdrawn {
			t.Fatalf("withdrawn with nothing stored = %v (%v), want false", withdrawn, err)
		}
		testutil.MustExec(t, pool,
			`UPDATE official_alerts SET references_text = 'IMD-Chennai IN-1792000000000401_1 2026-10-25T09:00:00+05:30' WHERE guid = '1792000000000402'`)
		withdrawn, err = q.OfficialAlertWithdrawnByStored(ctx, sqlcgen.OfficialAlertWithdrawnByStoredParams{
			Guid: "1792000000000401", CapIdentifier: "IN-1792000000000401_1"})
		if err != nil || !withdrawn {
			t.Errorf("withdrawn by a stored message that names it = %v (%v), want true", withdrawn, err)
		}
		// A longer identifier that merely contains the text is not a match.
		withdrawn, _ = q.OfficialAlertWithdrawnByStored(ctx, sqlcgen.OfficialAlertWithdrawnByStoredParams{
			Guid: "1792000000000401", CapIdentifier: "IN-1792000000000401"})
		if withdrawn {
			t.Error("a prefix of an identifier was read as a match")
		}
	})

	t.Run("deleting an account deletes only its rows", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := officialPushPerson(t, pool, "op-del-a@example.com", "op-del-a-sess", "tf3bq")
		b := officialPushPerson(t, pool, "op-del-b@example.com", "op-del-b-sess", "tf3bq")
		alert := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{Severity: "Severe", Tier: "district", DistrictCodes: []int{749}})
		officialPushEnqueue(t, q, a, alert, "IN-d_1", "new", far)
		officialPushEnqueue(t, q, a, alert, "IN-d_2", "escalated", far)
		officialPushEnqueue(t, q, b, alert, "IN-d_1", "new", far)
		n, err := q.DeleteAccountOfficialPushOutbox(ctx, a)
		if err != nil || n != 2 {
			t.Errorf("DeleteAccountOfficialPushOutbox = %d (%v), want 2", n, err)
		}
		var left int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM official_push_outbox WHERE account_id = $1`, b).Scan(&left); err != nil || left != 1 {
			t.Errorf("the other account holds %d rows (%v), want 1", left, err)
		}
	})
}

// TestOfficialSeriesQueries covers the two series statements of plan 05-16
// (amended 2026-10-29): the walk over references_text and the sent check over
// every alert id of a series.
func TestOfficialSeriesQueries(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	far := time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)
	base := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)

	seed := func(guid string, minutes int, refs string) int64 {
		id := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
			GUID: guid, CapIdentifier: "IN-" + guid + "_1", Severity: "Severe",
			Tier: "district", DistrictCodes: []int{749}, SentAt: base.Add(time.Duration(minutes) * time.Minute),
		})
		if refs != "" {
			testutil.MustExec(t, pool, `UPDATE official_alerts SET references_text = $2 WHERE id = $1`, id, refs)
		}
		return id
	}
	ids := func(rows []sqlcgen.OfficialAlertSeriesRow) map[int64]bool {
		out := map[int64]bool{}
		for _, r := range rows {
			if out[r.ID] {
				t.Errorf("alert %d appears twice in the series", r.ID)
			}
			out[r.ID] = true
		}
		return out
	}
	roots := func(rows []sqlcgen.OfficialAlertSeriesRow) []int64 {
		var out []int64
		for _, r := range rows {
			if !r.NamesMember {
				out = append(out, r.ID)
			}
		}
		return out
	}

	t.Run("a chain A to B to C returns all three with A as the root", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := seed("1900000000000001", 0, "")
		b := seed("1900000000000002", 10, "IN-1900000000000001_1")
		c := seed("1900000000000003", 20, "IN-1900000000000002_1")
		rows, err := q.OfficialAlertSeries(ctx, c)
		if err != nil {
			t.Fatalf("OfficialAlertSeries: %v", err)
		}
		got := ids(rows)
		if len(rows) != 3 || !got[a] || !got[b] || !got[c] {
			t.Fatalf("series = %+v, want alerts %d, %d and %d", rows, a, b, c)
		}
		if r := roots(rows); len(r) != 1 || r[0] != a {
			t.Errorf("roots = %v, want only %d", r, a)
		}
		if rows[0].ID != a || rows[0].Guid != "1900000000000001" || !rows[0].SentAt.Equal(base) {
			t.Errorf("first row = %+v, want the oldest alert first", rows[0])
		}
		// From the root the walk goes nowhere: it names nothing.
		fromRoot, err := q.OfficialAlertSeries(ctx, a)
		if err != nil || len(fromRoot) != 1 || fromRoot[0].ID != a || fromRoot[0].NamesMember {
			t.Errorf("series from the root = %+v (%v), want only itself", fromRoot, err)
		}
	})

	t.Run("an alert naming two stored alerts returns both", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := seed("1900000000000011", 0, "")
		b := seed("1900000000000012", 5, "")
		c := seed("1900000000000013", 20, "IN-1900000000000011_1 IN-1900000000000012_1")
		rows, err := q.OfficialAlertSeries(ctx, c)
		if err != nil {
			t.Fatalf("OfficialAlertSeries: %v", err)
		}
		got := ids(rows)
		if len(rows) != 3 || !got[a] || !got[b] || !got[c] {
			t.Fatalf("series = %+v, want alerts %d, %d and %d", rows, a, b, c)
		}
		if r := roots(rows); len(r) != 2 {
			t.Errorf("roots = %v, want the two alerts that name nothing", r)
		}
	})

	t.Run("a references cycle ends and returns each alert once", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := seed("1900000000000021", 0, "IN-1900000000000022_1")
		b := seed("1900000000000022", 10, "IN-1900000000000021_1")
		rows, err := q.OfficialAlertSeries(ctx, a)
		if err != nil {
			t.Fatalf("OfficialAlertSeries: %v", err)
		}
		got := ids(rows)
		if len(rows) != 2 || !got[a] || !got[b] {
			t.Fatalf("series = %+v, want alerts %d and %d once each", rows, a, b)
		}
		if r := roots(rows); len(r) != 0 {
			t.Errorf("roots = %v, want none in a cycle (the caller falls back to the earliest)", r)
		}
	})

	t.Run("an alert that names itself or nothing returns only itself", func(t *testing.T) {
		testutil.Truncate(t, pool)
		lone := seed("1900000000000031", 0, "")
		self := seed("1900000000000032", 0, "IN-1900000000000032_1")
		unknown := seed("1900000000000033", 0, "IN-not-stored_1")
		for _, id := range []int64{lone, self, unknown} {
			rows, err := q.OfficialAlertSeries(ctx, id)
			if err != nil || len(rows) != 1 || rows[0].ID != id || rows[0].NamesMember {
				t.Errorf("series of %d = %+v (%v), want only itself naming no member", id, rows, err)
			}
		}
		rows, err := q.OfficialAlertSeries(ctx, 999999)
		if err != nil || len(rows) != 0 {
			t.Errorf("series of a missing alert = %+v (%v), want no rows", rows, err)
		}
	})

	t.Run("sent for series is true for any sent row of the series and only for that account", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := seed("1900000000000041", 0, "")
		b := seed("1900000000000042", 10, "IN-1900000000000041_1")
		other := seed("1900000000000043", 10, "")
		mine := officialPushPerson(t, pool, "series-mine@example.com", "series-mine-sess", "tf3bq")
		theirs := officialPushPerson(t, pool, "series-theirs@example.com", "series-theirs-sess", "tf3bq")
		sent := func(account, alert int64, capID, state string) {
			testutil.MustExec(t, pool,
				`INSERT INTO official_push_outbox (account_id, alert_id, cap_identifier, reason, state, sent_at, created_at, expires_at)
				 VALUES ($1, $2, $3, 'new', $4, $5, $5, $6)`,
				account, alert, capID, state, base, far)
		}
		ask := func(account int64, alertIDs ...int64) bool {
			got, err := q.OfficialPushSentForSeries(ctx, sqlcgen.OfficialPushSentForSeriesParams{AccountID: account, AlertIds: alertIDs})
			if err != nil {
				t.Fatalf("OfficialPushSentForSeries: %v", err)
			}
			return got
		}

		if ask(mine, a, b) {
			t.Error("true with no rows at all")
		}
		sent(mine, a, "IN-1900000000000041_1", "skipped")
		sent(mine, b, "IN-1900000000000042_1", "failed")
		if ask(mine, a, b) {
			t.Error("true when the only rows are skipped or failed")
		}
		sent(mine, a, "IN-1900000000000041_9", "sent")
		if !ask(mine, a, b) {
			t.Error("false though the account had a sent row for the first alert of the series")
		}
		if !ask(mine, b, a) {
			t.Error("false when the ids come in another order")
		}
		if ask(theirs, a, b) {
			t.Error("true for another account")
		}
		if ask(mine, other) {
			t.Error("true for an alert outside the series")
		}
		if ask(mine) {
			t.Error("true for an empty list of alerts")
		}
	})
}
