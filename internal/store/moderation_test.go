package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// These tests prove the Phase 4 store guarantees against a real Postgres, all
// through the same generated code the later plans call (R-01, R-05, R-10,
// D-04, D-08, D-09, D-19). Database tests skip without DATABASE_URL via
// testutil.NewTestDB. Helpers carry the prefix mod so they never collide with
// helpers in the other test files of this package.

// modParams builds InsertReportIdempotentParams with fixed place and category
// values. Callers set the request fields they care about.
func modParams(description string, accountID *int64, requestID *string, state string) sqlcgen.InsertReportIdempotentParams {
	now := time.Now().UTC()
	return sqlcgen.InsertReportIdempotentParams{
		SessionID:        "mod-test-session",
		Category:         "flood",
		Severity:         "low",
		Description:      description,
		Latitude:         12.9716,
		Longitude:        77.5946,
		Geohash:          "tdr1qgzn",
		CreatedAt:        now,
		ExpiresAt:        now.Add(time.Hour),
		ModerationState:  state,
		RequestID:        requestID,
		RequestAccountID: accountID,
	}
}

// modRawReport inserts a report with an explicit severity, state and times and
// returns its id.
func modRawReport(t *testing.T, pool *pgxpool.Pool, description, severity, state string, createdAt, expiresAt time.Time) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at, moderation_state)
		 VALUES ('mod-test-session', 'flood', $1, $2, 12.9716, 77.5946, 'tdr1qgzn', $3, $4, $5)
		 RETURNING id`,
		severity, description, createdAt, expiresAt, state).Scan(&id)
	if err != nil {
		t.Fatalf("inserting report %q: %v", description, err)
	}
	return id
}

// modPendingCheck writes a pending check that is due a minute ago.
func modPendingCheck(t *testing.T, q *sqlcgen.Queries, reportID int64) {
	t.Helper()
	err := q.InsertModerationCheck(context.Background(), sqlcgen.InsertModerationCheckParams{
		ReportID:      reportID,
		Status:        "pending",
		NextAttemptAt: time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("inserting pending check for %d: %v", reportID, err)
	}
}

// modReviewCheck writes a done borderline check that needs a person's look.
func modReviewCheck(t *testing.T, q *sqlcgen.Queries, reportID int64) {
	t.Helper()
	verdict := "borderline"
	model := "omni-moderation-latest"
	err := q.InsertModerationCheck(context.Background(), sqlcgen.InsertModerationCheckParams{
		ReportID:      reportID,
		Status:        "done",
		NextAttemptAt: time.Now(),
		Verdict:       &verdict,
		Scores:        []byte(`{"hate":0.5}`),
		Model:         &model,
		CheckedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
		NeedsReview:   true,
	})
	if err != nil {
		t.Fatalf("inserting review check for %d: %v", reportID, err)
	}
}

func modFlag(t *testing.T, q *sqlcgen.Queries, reportID, accountID int64, reason string) {
	t.Helper()
	if _, err := q.InsertReportFlag(context.Background(), sqlcgen.InsertReportFlagParams{
		ReportID: reportID, AccountID: accountID, Reason: reason,
	}); err != nil {
		t.Fatalf("flagging %d: %v", reportID, err)
	}
}

func modState(t *testing.T, pool *pgxpool.Pool, reportID int64) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `SELECT moderation_state FROM reports WHERE id = $1`, reportID).Scan(&s); err != nil {
		t.Fatalf("reading state of %d: %v", reportID, err)
	}
	return s
}

// modCheckRow reads the columns of one check that tests compare.
type modCheckRow struct {
	Status      string
	NeedsReview bool
	LeaseNull   bool
	Attempts    int32
}

func modCheck(t *testing.T, pool *pgxpool.Pool, reportID int64) modCheckRow {
	t.Helper()
	var r modCheckRow
	err := pool.QueryRow(context.Background(),
		`SELECT status, needs_review, lease_until IS NULL, attempts FROM moderation_checks WHERE report_id = $1`,
		reportID).Scan(&r.Status, &r.NeedsReview, &r.LeaseNull, &r.Attempts)
	if err != nil {
		t.Fatalf("reading check of %d: %v", reportID, err)
	}
	return r
}

func modOpenFlags(t *testing.T, pool *pgxpool.Pool, reportID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM report_flags WHERE report_id = $1 AND dismissed_at IS NULL`, reportID).Scan(&n); err != nil {
		t.Fatalf("counting open flags of %d: %v", reportID, err)
	}
	return n
}

func modClaim(q *sqlcgen.Queries, limit int32) ([]sqlcgen.ClaimDueModerationChecksRow, error) {
	return q.ClaimDueModerationChecks(context.Background(), sqlcgen.ClaimDueModerationChecksParams{
		LeaseSeconds: 60,
		BatchLimit:   limit,
	})
}

func TestInsertReportIdempotentOneRow(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	a := seedAccountForVotes(t, pool, "idem-a@example.com")
	b := seedAccountForVotes(t, pool, "idem-b@example.com")
	x := "request-x"

	first, err := q.InsertReportIdempotent(ctx, modParams("idem one", &a, &x, "pending"))
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if first.ModerationState != "pending" {
		t.Errorf("first insert state = %q, want pending", first.ModerationState)
	}
	if _, err := q.InsertReportIdempotent(ctx, modParams("idem one", &a, &x, "pending")); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("retry: want pgx.ErrNoRows, got %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE request_id = $1 AND request_account_id = $2`, x, a).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows for account A and request x = %d (err %v), want 1", n, err)
	}

	other, err := q.InsertReportIdempotent(ctx, modParams("idem two", &b, &x, "clear"))
	if err != nil {
		t.Fatalf("account B with the same request id: %v", err)
	}
	if other.ID == first.ID {
		t.Errorf("account B got account A's row %d", first.ID)
	}

	r1, err := q.InsertReportIdempotent(ctx, modParams("idem three", nil, nil, "clear"))
	if err != nil {
		t.Fatalf("insert without request id: %v", err)
	}
	r2, err := q.InsertReportIdempotent(ctx, modParams("idem three", nil, nil, "clear"))
	if err != nil {
		t.Fatalf("second insert without request id: %v", err)
	}
	if r1.ID == r2.ID {
		t.Errorf("two inserts without a request id must make two rows, both are %d", r1.ID)
	}
}

func TestInsertReportIdempotentConcurrent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	a := seedAccountForVotes(t, pool, "idem-race@example.com")
	x := "race-request"

	const workers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	won, noRows := 0, 0
	var others []error
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := q.InsertReportIdempotent(ctx, modParams("idem race", &a, &x, "clear"))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				won++
			case errors.Is(err, pgx.ErrNoRows):
				noRows++
			default:
				others = append(others, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if len(others) != 0 {
		t.Fatalf("unexpected errors: %v", others)
	}
	if won != 1 || noRows != workers-1 {
		t.Fatalf("won %d, no row %d; want 1 and %d", won, noRows, workers-1)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE request_id = $1`, x).Scan(&n); err != nil || n != 1 {
		t.Fatalf("stored rows = %d (err %v), want 1", n, err)
	}
}

func TestReportByRequestScopedToAccount(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	a := seedAccountForVotes(t, pool, "replay-a@example.com")
	b := seedAccountForVotes(t, pool, "replay-b@example.com")
	x := "replay-x"

	made, err := q.InsertReportIdempotent(ctx, modParams("replay one", &a, &x, "pending"))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := q.ReportByRequest(ctx, sqlcgen.ReportByRequestParams{AccountID: a, RequestID: x})
	if err != nil {
		t.Fatalf("replay for A: %v", err)
	}
	if got.ID != made.ID || got.ModerationState != "pending" {
		t.Errorf("replay = id %d state %q, want id %d state pending", got.ID, got.ModerationState, made.ID)
	}
	if _, err := q.ReportByRequest(ctx, sqlcgen.ReportByRequestParams{AccountID: b, RequestID: x}); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("account B asking for A's request id: want pgx.ErrNoRows, got %v", err)
	}
	if _, err := q.ReportByRequest(ctx, sqlcgen.ReportByRequestParams{AccountID: a, RequestID: "unknown"}); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("unknown request id: want pgx.ErrNoRows, got %v", err)
	}
}

func TestReportExtrasForReports(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	viewer := seedAccountForVotes(t, pool, "extras-viewer@example.com")
	other := seedAccountForVotes(t, pool, "extras-other@example.com")

	r1 := seedReportForVotes(t, pool, "extras clear")
	r2 := seedReportForVotes(t, pool, "extras pending")
	r3 := seedReportForVotes(t, pool, "extras removed")
	testutil.MustExec(t, pool, `UPDATE reports SET moderation_state = 'pending', client_written_at = now() - interval '5 minutes' WHERE id = $1`, r2)
	testutil.MustExec(t, pool, `UPDATE reports SET moderation_state = 'removed' WHERE id = $1`, r3)

	modFlag(t, q, r2, viewer, "spam")
	modFlag(t, q, r1, other, "spam")
	modFlag(t, q, r3, viewer, "other")
	testutil.MustExec(t, pool, `UPDATE report_flags SET dismissed_at = now() WHERE report_id = $1`, r3)

	rows, err := q.ReportExtrasForReports(ctx, sqlcgen.ReportExtrasForReportsParams{
		ReportIds:       []int64{r1, r2, r3, r3 + 1000},
		ViewerAccountID: viewer,
	})
	if err != nil {
		t.Fatalf("extras: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("extras rows = %d, want 3 (the missing id returns nothing)", len(rows))
	}
	byID := map[int64]sqlcgen.ReportExtrasForReportsRow{}
	for _, r := range rows {
		byID[r.ReportID] = r
	}
	want := []struct {
		id      int64
		state   string
		written bool
		flagged bool
	}{
		{r1, "clear", false, false},
		{r2, "pending", true, true},
		{r3, "removed", false, true},
	}
	for _, w := range want {
		got, ok := byID[w.id]
		if !ok {
			t.Errorf("report %d missing from extras", w.id)
			continue
		}
		if got.ModerationState != w.state {
			t.Errorf("report %d state = %q, want %q", w.id, got.ModerationState, w.state)
		}
		if got.ClientWrittenAt.Valid != w.written {
			t.Errorf("report %d ClientWrittenAt valid = %v, want %v", w.id, got.ClientWrittenAt.Valid, w.written)
		}
		if got.ViewerFlagged != w.flagged {
			t.Errorf("report %d ViewerFlagged = %v, want %v", w.id, got.ViewerFlagged, w.flagged)
		}
	}

	empty, err := q.ReportExtrasForReports(ctx, sqlcgen.ReportExtrasForReportsParams{
		ReportIds:       []int64{},
		ViewerAccountID: viewer,
	})
	if err != nil || len(empty) != 0 {
		t.Errorf("empty id slice: %d rows, err %v; want 0 and nil", len(empty), err)
	}
}

func TestClaimModerationChecksSingleClaimant(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)

	r := seedReportForVotes(t, pool, "claim single")
	modPendingCheck(t, q, r)

	const workers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var claimed []sqlcgen.ClaimDueModerationChecksRow
	var errs []error
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			rows, err := modClaim(q, 5)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			claimed = append(claimed, rows...)
		}()
	}
	close(start)
	wg.Wait()
	if len(errs) != 0 {
		t.Fatalf("claim errors: %v", errs)
	}
	if len(claimed) != 1 || claimed[0].ReportID != r || claimed[0].Attempts != 1 {
		t.Fatalf("claimed %+v; want exactly one row for report %d with attempts 1", claimed, r)
	}

	again, err := modClaim(q, 5)
	if err != nil || len(again) != 0 {
		t.Fatalf("second claim under a live lease: %d rows, err %v; want 0", len(again), err)
	}

	testutil.MustExec(t, pool, `UPDATE moderation_checks SET lease_until = now() - interval '1 second' WHERE report_id = $1`, r)
	lapsed, err := modClaim(q, 5)
	if err != nil || len(lapsed) != 1 || lapsed[0].Attempts != 2 {
		t.Fatalf("claim after the lease lapsed: %+v, err %v; want one row with attempts 2", lapsed, err)
	}
	testutil.MustExec(t, pool, `UPDATE moderation_checks SET status = 'done', lease_until = NULL WHERE report_id = $1`, r)

	// Checks that must never be claimed.
	done := seedReportForVotes(t, pool, "claim done")
	modPendingCheck(t, q, done)
	testutil.MustExec(t, pool, `UPDATE moderation_checks SET status = 'done' WHERE report_id = $1`, done)

	future := seedReportForVotes(t, pool, "claim not yet due")
	modPendingCheck(t, q, future)
	testutil.MustExec(t, pool, `UPDATE moderation_checks SET next_attempt_at = now() + interval '1 hour' WHERE report_id = $1`, future)

	expired := seedReportForVotes(t, pool, "claim expired")
	modPendingCheck(t, q, expired)
	testutil.MustExec(t, pool, `UPDATE reports SET expires_at = now() - interval '1 minute' WHERE id = $1`, expired)

	removed := seedReportForVotes(t, pool, "claim removed")
	modPendingCheck(t, q, removed)
	testutil.MustExec(t, pool, `UPDATE reports SET moderation_state = 'removed' WHERE id = $1`, removed)

	none, err := modClaim(q, 100)
	if err != nil || len(none) != 0 {
		t.Fatalf("claim with only ineligible checks: %+v, err %v; want none", none, err)
	}

	// Batch limit: seven due checks, limit five, the five earliest come back.
	testutil.MustExec(t, pool, `UPDATE moderation_checks SET status = 'done'`)
	var ids []int64
	for i := 0; i < 7; i++ {
		id := seedReportForVotes(t, pool, fmt.Sprintf("claim batch %d", i))
		modPendingCheck(t, q, id)
		testutil.MustExec(t, pool, `UPDATE moderation_checks SET next_attempt_at = now() - make_interval(mins => $2) WHERE report_id = $1`, id, 10+(7-i))
		ids = append(ids, id)
	}
	batch, err := modClaim(q, 5)
	if err != nil {
		t.Fatalf("batch claim: %v", err)
	}
	if len(batch) != 5 {
		t.Fatalf("batch claim returned %d rows, want 5", len(batch))
	}
	got := make([]int64, 0, len(batch))
	for _, row := range batch {
		got = append(got, row.ReportID)
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	wantIDs := append([]int64(nil), ids[:5]...)
	sort.Slice(wantIDs, func(i, j int) bool { return wantIDs[i] < wantIDs[j] })
	if !reflect.DeepEqual(got, wantIDs) {
		t.Errorf("batch claimed %v, want the five earliest due %v", got, wantIDs)
	}
}

func TestReportFlagsUniquePerAccount(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	a := seedAccountForVotes(t, pool, "flag-a@example.com")
	b := seedAccountForVotes(t, pool, "flag-b@example.com")
	r := seedReportForVotes(t, pool, "flag target")

	snapshot := func() string {
		var s string
		if err := pool.QueryRow(ctx, `SELECT to_jsonb(r)::text FROM reports r WHERE id = $1`, r).Scan(&s); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		return s
	}
	before := snapshot()

	if id, err := q.InsertReportFlag(ctx, sqlcgen.InsertReportFlagParams{ReportID: r, AccountID: a, Reason: "spam"}); err != nil || id == 0 {
		t.Fatalf("A flags: id %d, err %v", id, err)
	}
	if _, err := q.InsertReportFlag(ctx, sqlcgen.InsertReportFlagParams{ReportID: r, AccountID: a, Reason: "abusive"}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("A flags again: want pgx.ErrNoRows, got %v", err)
	}
	if id, err := q.InsertReportFlag(ctx, sqlcgen.InsertReportFlagParams{ReportID: r, AccountID: b, Reason: "false"}); err != nil || id == 0 {
		t.Fatalf("B flags: id %d, err %v", id, err)
	}

	c := seedAccountForVotes(t, pool, "flag-c@example.com")
	_, err := pool.Exec(ctx, `INSERT INTO report_flags (report_id, account_id, reason) VALUES ($1, $2, 'police')`, r, c)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Errorf("reason police: want SQLSTATE 23514, got %v", err)
	}

	if after := snapshot(); after != before {
		t.Errorf("a flag changed the reports row:\nbefore %s\nafter  %s", before, after)
	}
}

func TestSeededReportIsClear(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	testutil.SeedReports(t, pool, 20, 12.9716, 77.5946)
	testutil.SeedExpiringReport(t, pool, 3600)
	now := time.Now().UTC()
	if _, err := q.InsertReport(ctx, sqlcgen.InsertReportParams{
		SessionID: "plain", Category: "flood", Severity: "low", Description: "plain insert",
		Latitude: 12.9716, Longitude: 77.5946, Geohash: "tdr1qgzn",
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("plain InsertReport: %v", err)
	}

	var total, notClear int
	err := pool.QueryRow(ctx, `SELECT count(*),
			count(*) FILTER (WHERE moderation_state <> 'clear' OR request_id IS NOT NULL
				OR request_account_id IS NOT NULL OR client_written_at IS NOT NULL)
		FROM reports`).Scan(&total, &notClear)
	if err != nil {
		t.Fatalf("reading reports: %v", err)
	}
	if total != 22 || notClear != 0 {
		t.Errorf("total %d, not clear or carrying request fields %d; want 22 and 0", total, notClear)
	}
	var checks int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM moderation_checks`).Scan(&checks); err != nil || checks != 0 {
		t.Errorf("moderation_checks rows = %d (err %v), want 0", checks, err)
	}
}

// TestNearbyReportsQueryUnchanged needs no database. It compares the
// NearbyReports block of queries/reports.sql with the drift const used by the
// EXPLAIN test, after dropping comments, turning each sqlc.arg into a numbered
// parameter in order of first appearance and collapsing whitespace.
func TestNearbyReportsQueryUnchanged(t *testing.T) {
	data, err := os.ReadFile("queries/reports.sql")
	if err != nil {
		t.Fatalf("reading queries/reports.sql: %v", err)
	}
	var block []string
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- name: NearbyReports") {
			in = true
			continue
		}
		if !in {
			continue
		}
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		block = append(block, line)
		if strings.HasSuffix(trimmed, ";") {
			break
		}
	}
	if len(block) == 0 {
		t.Fatal("NearbyReports block not found in queries/reports.sql")
	}
	body := strings.Join(block, "\n")

	for _, banned := range []string{"moderation_state", "request_id", "client_written_at", "report_flags", "moderation_checks"} {
		if strings.Contains(body, banned) {
			t.Errorf("NearbyReports must not name %q", banned)
		}
	}

	order := map[string]int{}
	argRE := regexp.MustCompile(`sqlc\.arg\((\w+)\)`)
	body = argRE.ReplaceAllStringFunc(body, func(m string) string {
		name := argRE.FindStringSubmatch(m)[1]
		n, ok := order[name]
		if !ok {
			n = len(order) + 1
			order[name] = n
		}
		return fmt.Sprintf("$%d", n)
	})
	body = strings.TrimSuffix(strings.TrimSpace(body), ";")

	collapse := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	if got, want := collapse(body), collapse(nearbyReportsSQLForExplain); got != want {
		t.Errorf("NearbyReports changed.\nsource: %s\nconst:  %s", got, want)
	}
}

func TestModerationQueueCounts(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	acc1 := seedAccountForVotes(t, pool, "queue-1@example.com")
	acc2 := seedAccountForVotes(t, pool, "queue-2@example.com")

	base := time.Now().UTC().Add(-3 * time.Hour)
	future := time.Now().UTC().Add(24 * time.Hour)

	// List (a): 101 review rows, newest first at index 0.
	reviewIDs := make([]int64, 0, 101)
	for i := 0; i < 101; i++ {
		created := base.Add(-time.Duration(i) * time.Minute)
		id := modRawReport(t, pool, fmt.Sprintf("queue review %d", i), "low", "clear", created, future)
		modReviewCheck(t, q, id)
		reviewIDs = append(reviewIDs, id)
	}

	// List (b): one report with two reasons from two accounts, no check row.
	flagged := modRawReport(t, pool, "queue flagged", "low", "clear", base, future)
	modFlag(t, q, flagged, acc1, "spam")
	modFlag(t, q, flagged, acc2, "abusive")

	// List (c): one unexpired and one expired pending check.
	pending := modRawReport(t, pool, "queue pending", "high", "pending", base.Add(-time.Hour), future)
	modPendingCheck(t, q, pending)
	expired := modRawReport(t, pool, "queue expired", "low", "pending", base.Add(-2*time.Hour), time.Now().UTC().Add(-time.Minute))
	modPendingCheck(t, q, expired)

	// A removed report with a review check, an open flag and a pending check
	// row is in no list and no count. One check row carries both facts.
	removed := modRawReport(t, pool, "queue removed", "low", "removed", base, future)
	modPendingCheck(t, q, removed)
	testutil.MustExec(t, pool, `UPDATE moderation_checks SET needs_review = true WHERE report_id = $1`, removed)
	modFlag(t, q, removed, acc1, "spam")

	review, err := q.ReviewQueue(ctx)
	if err != nil {
		t.Fatalf("ReviewQueue: %v", err)
	}
	if len(review) != 100 {
		t.Fatalf("ReviewQueue length = %d, want 100", len(review))
	}
	for i, row := range review {
		if row.ID != reviewIDs[i] {
			t.Fatalf("ReviewQueue row %d is report %d, want %d (newest first)", i, row.ID, reviewIDs[i])
		}
	}
	if review[0].CheckStatus != "done" || review[0].Verdict == nil || *review[0].Verdict != "borderline" {
		t.Errorf("review row check = %q %v, want done borderline", review[0].CheckStatus, review[0].Verdict)
	}

	counts, err := q.ModerationQueueCounts(ctx)
	if err != nil {
		t.Fatalf("ModerationQueueCounts: %v", err)
	}
	if counts.ReviewCount != 101 || counts.FlaggedCount != 1 || counts.PendingCount != 1 {
		t.Errorf("counts = %+v, want review 101, flagged 1, pending 1", counts)
	}

	flaggedRows, err := q.FlagQueue(ctx)
	if err != nil {
		t.Fatalf("FlagQueue: %v", err)
	}
	if len(flaggedRows) != 1 || flaggedRows[0].ID != flagged {
		t.Fatalf("FlagQueue = %d rows, want exactly report %d", len(flaggedRows), flagged)
	}
	if flaggedRows[0].OpenFlagTotal != 2 {
		t.Errorf("open_flag_total = %d, want 2", flaggedRows[0].OpenFlagTotal)
	}
	var reasons map[string]int
	if err := json.Unmarshal(flaggedRows[0].FlagCounts, &reasons); err != nil {
		t.Fatalf("decoding flag_counts %s: %v", flaggedRows[0].FlagCounts, err)
	}
	if len(reasons) != 2 || reasons["spam"] != 1 || reasons["abusive"] != 1 {
		t.Errorf("flag_counts = %v, want one spam and one abusive", reasons)
	}
	if flaggedRows[0].CheckStatus != nil {
		t.Errorf("flagged report with no check has check_status %v, want NULL", *flaggedRows[0].CheckStatus)
	}

	pendingRows, err := q.PendingQueue(ctx)
	if err != nil {
		t.Fatalf("PendingQueue: %v", err)
	}
	if len(pendingRows) != 1 || pendingRows[0].ID != pending {
		t.Fatalf("PendingQueue = %d rows, want exactly report %d", len(pendingRows), pending)
	}

	// Status booleans match list membership.
	type statusWant struct{ review, flagged, pending bool }
	cases := map[string]struct {
		id   int64
		want statusWant
	}{
		"review":  {reviewIDs[0], statusWant{true, false, false}},
		"flagged": {flagged, statusWant{false, true, false}},
		"pending": {pending, statusWant{false, false, true}},
		"expired": {expired, statusWant{false, false, false}},
		"removed": {removed, statusWant{false, false, false}},
	}
	for name, c := range cases {
		st, err := q.ModerationStatusForReport(ctx, c.id)
		if err != nil {
			t.Errorf("status %s: %v", name, err)
			continue
		}
		if got := (statusWant{st.InReview, st.InFlagged, st.InPending}); got != c.want {
			t.Errorf("status %s = %+v, want %+v", name, got, c.want)
		}
	}
	if _, err := q.ModerationStatusForReport(ctx, removed+100000); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("unknown id: want pgx.ErrNoRows, got %v", err)
	}

	// Ordering: flagged is most flags first then newest; pending is oldest first.
	newerFlagged := modRawReport(t, pool, "queue flagged newer", "low", "clear", base.Add(time.Hour), future)
	modFlag(t, q, newerFlagged, acc1, "other")
	flaggedRows, err = q.FlagQueue(ctx)
	if err != nil {
		t.Fatalf("FlagQueue again: %v", err)
	}
	if len(flaggedRows) != 2 || flaggedRows[0].ID != flagged || flaggedRows[1].ID != newerFlagged {
		t.Errorf("FlagQueue order = %v, want [%d %d] (most flags first)", modQueueIDs(flaggedRows), flagged, newerFlagged)
	}
	older := modRawReport(t, pool, "queue pending older", "low", "pending", base.Add(-5*time.Hour), future)
	modPendingCheck(t, q, older)
	pendingRows, err = q.PendingQueue(ctx)
	if err != nil {
		t.Fatalf("PendingQueue again: %v", err)
	}
	if len(pendingRows) != 2 || pendingRows[0].ID != older || pendingRows[1].ID != pending {
		t.Errorf("PendingQueue order has %d rows, want [%d %d] (oldest first)", len(pendingRows), older, pending)
	}
	counts, err = q.ModerationQueueCounts(ctx)
	if err != nil {
		t.Fatalf("ModerationQueueCounts again: %v", err)
	}
	if counts.FlaggedCount != 2 || counts.PendingCount != 2 {
		t.Errorf("counts after adding = %+v, want flagged 2, pending 2", counts)
	}
}

func modQueueIDs(rows []sqlcgen.FlagQueueRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestKeepReportStatement(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	acc1 := seedAccountForVotes(t, pool, "keep-1@example.com")
	acc2 := seedAccountForVotes(t, pool, "keep-2@example.com")
	mod := seedAccountForVotes(t, pool, "keep-mod@example.com")
	now := time.Now().UTC()
	future := now.Add(24 * time.Hour)

	pend := modRawReport(t, pool, "keep pending", "low", "pending", now, future)
	modPendingCheck(t, q, pend)
	testutil.MustExec(t, pool, `UPDATE moderation_checks SET needs_review = true, lease_until = now() + interval '1 minute' WHERE report_id = $1`, pend)
	modFlag(t, q, pend, acc1, "spam")
	modFlag(t, q, pend, acc2, "false")

	if err := q.KeepReport(ctx, pend); err != nil {
		t.Fatalf("KeepReport: %v", err)
	}
	if s := modState(t, pool, pend); s != "clear" {
		t.Errorf("kept pending report state = %q, want clear", s)
	}
	c := modCheck(t, pool, pend)
	if c.Status != "done" || c.NeedsReview || !c.LeaseNull {
		t.Errorf("check after Keep = %+v, want done, no review, no lease", c)
	}
	if n := modOpenFlags(t, pool, pend); n != 0 {
		t.Errorf("open flags after Keep = %d, want 0", n)
	}

	serious := modRawReport(t, pool, "keep serious", "critical", "pending", now, future)
	modPendingCheck(t, q, serious)
	if err := q.KeepReport(ctx, serious); err != nil {
		t.Fatalf("KeepReport serious: %v", err)
	}
	if s := modState(t, pool, serious); s != "clear" {
		t.Errorf("kept serious report state = %q, want clear", s)
	}

	gone := modRawReport(t, pool, "keep removed", "low", "removed", now, future)
	if err := q.KeepReport(ctx, gone); err != nil {
		t.Fatalf("KeepReport removed: %v", err)
	}
	if s := modState(t, pool, gone); s != "removed" {
		t.Errorf("Keep on a removed report left state %q, want removed", s)
	}

	if err := q.InsertModerationAction(ctx, sqlcgen.InsertModerationActionParams{ReportID: pend, ModeratorAccountID: mod, Action: "keep"}); err != nil {
		t.Fatalf("InsertModerationAction: %v", err)
	}
	var n int
	var who int64
	if err := pool.QueryRow(ctx, `SELECT count(*), COALESCE(max(moderator_account_id), 0) FROM moderation_actions WHERE report_id = $1 AND action = 'keep'`, pend).Scan(&n, &who); err != nil || n != 1 || who != mod {
		t.Errorf("audit rows = %d moderator %d (err %v), want 1 and %d", n, who, err, mod)
	}
}

func TestRemoveReportStatement(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	acc1 := seedAccountForVotes(t, pool, "remove-1@example.com")
	acc2 := seedAccountForVotes(t, pool, "remove-2@example.com")
	mod := seedAccountForVotes(t, pool, "remove-mod@example.com")
	now := time.Now().UTC()
	future := now.Add(24 * time.Hour)

	listed := modRawReport(t, pool, "remove listed", "low", "clear", now, future)
	modReviewCheck(t, q, listed)
	modFlag(t, q, listed, acc1, "spam")
	modFlag(t, q, listed, acc2, "abusive")
	if err := q.RemoveReport(ctx, listed); err != nil {
		t.Fatalf("RemoveReport: %v", err)
	}
	if s := modState(t, pool, listed); s != "removed" {
		t.Errorf("state = %q, want removed", s)
	}
	if c := modCheck(t, pool, listed); c.NeedsReview || c.Status != "done" {
		t.Errorf("check after Remove = %+v, want done and no review", c)
	}
	if n := modOpenFlags(t, pool, listed); n != 0 {
		t.Errorf("open flags after Remove = %d, want 0", n)
	}
	st, err := q.ModerationStatusForReport(ctx, listed)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.InReview || st.InFlagged || st.InPending {
		t.Errorf("status after Remove = %+v, want all three false", st)
	}

	pend := modRawReport(t, pool, "remove pending", "low", "pending", now, future)
	modPendingCheck(t, q, pend)
	if err := q.RemoveReport(ctx, pend); err != nil {
		t.Fatalf("RemoveReport pending: %v", err)
	}
	if s := modState(t, pool, pend); s != "removed" {
		t.Errorf("pending report state = %q, want removed", s)
	}
	if c := modCheck(t, pool, pend); c.Status != "done" {
		t.Errorf("pending check status = %q, want done", c.Status)
	}
	if rows, err := modClaim(q, 100); err != nil || len(rows) != 0 {
		t.Errorf("claim after Remove: %d rows, err %v; want none", len(rows), err)
	}

	bare := modRawReport(t, pool, "remove bare", "low", "clear", now, future)
	if err := q.RemoveReport(ctx, bare); err != nil {
		t.Fatalf("RemoveReport bare: %v", err)
	}
	if s := modState(t, pool, bare); s != "removed" {
		t.Errorf("bare report state = %q, want removed", s)
	}

	if err := q.InsertModerationAction(ctx, sqlcgen.InsertModerationActionParams{ReportID: listed, ModeratorAccountID: mod, Action: "remove"}); err != nil {
		t.Fatalf("InsertModerationAction: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM moderation_actions WHERE report_id = $1 AND action = 'remove'`, listed).Scan(&n); err != nil || n != 1 {
		t.Errorf("audit rows = %d (err %v), want 1", n, err)
	}
}

func TestWorkerCheckStatements(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	now := time.Now().UTC()
	future := now.Add(24 * time.Hour)

	// Claim, reschedule, finish.
	r := modRawReport(t, pool, "worker one", "low", "pending", now, future)
	modPendingCheck(t, q, r)
	if rows, err := modClaim(q, 5); err != nil || len(rows) != 1 {
		t.Fatalf("first claim: %d rows, err %v", len(rows), err)
	}
	lastErr := "transient"
	if err := q.RescheduleModerationCheck(ctx, sqlcgen.RescheduleModerationCheckParams{
		NextAttemptAt: time.Now().Add(time.Hour), LastError: &lastErr, ReportID: r,
	}); err != nil {
		t.Fatalf("Reschedule: %v", err)
	}
	var status, gotErr string
	var leaseNull bool
	if err := pool.QueryRow(ctx, `SELECT status, COALESCE(last_error, ''), lease_until IS NULL FROM moderation_checks WHERE report_id = $1`, r).Scan(&status, &gotErr, &leaseNull); err != nil {
		t.Fatalf("reading rescheduled check: %v", err)
	}
	if status != "pending" || gotErr != "transient" || !leaseNull {
		t.Errorf("rescheduled check = %q %q lease null %v; want pending transient true", status, gotErr, leaseNull)
	}
	if rows, err := modClaim(q, 5); err != nil || len(rows) != 0 {
		t.Errorf("claim after Reschedule: %d rows, err %v; want none", len(rows), err)
	}

	model := "omni-moderation-latest"
	if err := q.FinishModerationCheck(ctx, sqlcgen.FinishModerationCheckParams{
		Verdict: "borderline", Scores: []byte(`{"hate":0.5}`), Model: &model, NeedsReview: true, ReportID: r,
	}); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	read := func() (string, string, string, bool) {
		var st, verdict, scores string
		var checked bool
		if err := pool.QueryRow(ctx, `SELECT status, COALESCE(verdict, ''), COALESCE(scores::text, ''), checked_at IS NOT NULL FROM moderation_checks WHERE report_id = $1`, r).Scan(&st, &verdict, &scores, &checked); err != nil {
			t.Fatalf("reading finished check: %v", err)
		}
		return st, verdict, scores, checked
	}
	st, verdict, scores, checked := read()
	var decoded map[string]float64
	if err := json.Unmarshal([]byte(scores), &decoded); err != nil || decoded["hate"] != 0.5 {
		t.Errorf("scores = %s (err %v), want hate 0.5", scores, err)
	}
	if st != "done" || verdict != "borderline" || !checked {
		t.Errorf("finished check = %q %q checked %v; want done borderline true", st, verdict, checked)
	}

	if err := q.FinishModerationCheck(ctx, sqlcgen.FinishModerationCheckParams{Verdict: "pass", NeedsReview: false, ReportID: r}); err != nil {
		t.Fatalf("second Finish: %v", err)
	}
	if err := q.RescheduleModerationCheck(ctx, sqlcgen.RescheduleModerationCheckParams{NextAttemptAt: time.Now().Add(2 * time.Hour), ReportID: r}); err != nil {
		t.Fatalf("Reschedule on done: %v", err)
	}
	st2, verdict2, scores2, _ := read()
	if st2 != st || verdict2 != verdict || scores2 != scores {
		t.Errorf("a done check changed: %q %q %s -> %q %q %s", st, verdict, scores, st2, verdict2, scores2)
	}
	var due bool
	if err := pool.QueryRow(ctx, `SELECT next_attempt_at < now() + interval '90 minutes' FROM moderation_checks WHERE report_id = $1`, r).Scan(&due); err != nil || !due {
		t.Errorf("Reschedule moved a done check (due-within-90-minutes %v, err %v)", due, err)
	}

	// SetReportModerationState.
	p := modRawReport(t, pool, "worker release", "low", "pending", now, future)
	modPendingCheck(t, q, p)
	if err := q.SetReportModerationState(ctx, sqlcgen.SetReportModerationStateParams{ModerationState: "clear", ReportID: p}); err != nil {
		t.Fatalf("SetReportModerationState: %v", err)
	}
	if s := modState(t, pool, p); s != "clear" {
		t.Errorf("released report state = %q, want clear", s)
	}

	gone := modRawReport(t, pool, "worker removed", "low", "removed", now, future)
	modPendingCheck(t, q, gone)
	if err := q.SetReportModerationState(ctx, sqlcgen.SetReportModerationStateParams{ModerationState: "clear", ReportID: gone}); err != nil {
		t.Fatalf("SetReportModerationState on removed: %v", err)
	}
	if s := modState(t, pool, gone); s != "removed" {
		t.Errorf("removed report state = %q after a worker write, want removed", s)
	}

	kept := modRawReport(t, pool, "worker kept", "low", "pending", now, future)
	modPendingCheck(t, q, kept)
	if err := q.KeepReport(ctx, kept); err != nil {
		t.Fatalf("KeepReport: %v", err)
	}
	if err := q.SetReportModerationState(ctx, sqlcgen.SetReportModerationStateParams{ModerationState: "removed", ReportID: kept}); err != nil {
		t.Fatalf("late worker reject: %v", err)
	}
	if s := modState(t, pool, kept); s != "clear" {
		t.Errorf("a late worker reject changed a kept report to %q, want clear", s)
	}

	// An inline borderline row shows up in the review list.
	inline := modRawReport(t, pool, "worker inline", "low", "clear", now, future)
	modReviewCheck(t, q, inline)
	review, err := q.ReviewQueue(ctx)
	if err != nil {
		t.Fatalf("ReviewQueue: %v", err)
	}
	found := false
	for _, row := range review {
		if row.ID == inline {
			found = true
		}
	}
	if !found {
		t.Errorf("inline borderline report %d missing from ReviewQueue", inline)
	}
}

func TestReportVoteContextCarriesModeration(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	r := seedReportForVotes(t, pool, "vote context moderation")

	row, err := q.ReportVoteContext(ctx, r)
	if err != nil {
		t.Fatalf("ReportVoteContext: %v", err)
	}
	if row.ModerationState != "clear" {
		t.Errorf("state = %q, want clear", row.ModerationState)
	}
	testutil.MustExec(t, pool, `UPDATE reports SET moderation_state = 'removed' WHERE id = $1`, r)
	row, err = q.ReportVoteContext(ctx, r)
	if err != nil {
		t.Fatalf("ReportVoteContext after update: %v", err)
	}
	if row.ModerationState != "removed" {
		t.Errorf("state = %q, want removed", row.ModerationState)
	}
}

// TestModerationRowsCarryNoSessionID needs no database. No generated row type
// of a Phase 4 query may expose a session, an email or an account field
// (T-04-04, T-01-02 continued).
func TestModerationRowsCarryNoSessionID(t *testing.T) {
	rows := []any{
		sqlcgen.InsertReportIdempotentRow{},
		sqlcgen.ReportByRequestRow{},
		sqlcgen.ReportExtrasForReportsRow{},
		sqlcgen.ReportForModerationRow{},
		sqlcgen.ReviewQueueRow{},
		sqlcgen.FlagQueueRow{},
		sqlcgen.PendingQueueRow{},
		sqlcgen.ClaimDueModerationChecksRow{},
		sqlcgen.ModerationStatusForReportRow{},
		sqlcgen.ModerationQueueCountsRow{},
	}
	for _, row := range rows {
		typ := reflect.TypeOf(row)
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			for _, banned := range []string{"Session", "Email", "Account"} {
				if strings.Contains(name, banned) {
					t.Errorf("%s has field %s containing %q", typ.Name(), name, banned)
				}
			}
		}
	}
}
