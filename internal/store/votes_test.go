package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// seedReportForVotes inserts a minimal report and returns its id. No
// sessions row is needed — reports.session_id carries no foreign key.
// description is passed in so each seeded report is uniquely findable by
// the SELECT id FROM reports WHERE description = $1 read-back below.
func seedReportForVotes(t *testing.T, pool *pgxpool.Pool, description string) int64 {
	t.Helper()
	ctx := context.Background()
	testutil.MustExec(t, pool,
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at)
		 VALUES ($1, 'flood', 'low', $2, 12.9716, 77.5946, 'tdr1qgzn', now(), now() + interval '1 hour')`,
		"votes-test-session", description,
	)
	var reportID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM reports WHERE description = $1`, description).Scan(&reportID); err != nil {
		t.Fatalf("reading back report id for %q: %v", description, err)
	}
	return reportID
}

// seedAccountForVotes inserts an account by email and returns its id,
// following identity_test.go's two-step idiom (insert, then read the id
// back).
func seedAccountForVotes(t *testing.T, pool *pgxpool.Pool, email string) int64 {
	t.Helper()
	ctx := context.Background()
	testutil.MustExec(t, pool, `INSERT INTO accounts (email) VALUES ($1)`, email)
	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE email = $1`, email).Scan(&accountID); err != nil {
		t.Fatalf("reading back account id for %q: %v", email, err)
	}
	return accountID
}

// voterEmail builds a distinct, deterministic email for goroutine i in a
// concurrency test.
func voterEmail(prefix string, i int) string {
	return fmt.Sprintf("%s-%d@example.com", prefix, i)
}

// voterGeohashCell builds a distinct 7-character geohash cell for
// goroutine i, matching the pinned "tdr1qg0".."tdr1qg7" pattern.
func voterGeohashCell(i int) string {
	return fmt.Sprintf("tdr1qg%d", i)
}

// TestCastVoteConcurrent, TestCastVoteConcurrentSameAccountKeepsEveryRow,
// TestCurrentVoteIsLatestOnAccountChange, and TestVotesArePerReportPerAccountKind
// prove TRUST-09 and D-02's append-only vote log guarantees against a real
// Postgres. Every test here needs a real database and t.Skip cleanly
// without DATABASE_URL, so go test ./... -short stays green.

// TestCastVoteConcurrent is TRUST-09's headline proof — no vote silently
// dropped. 8 goroutines, 8 distinct accounts, one report, one kind: all 8
// calls succeed, all 8 rows persist, and all 8 are independently readable
// back as current votes. All 8 land with zero contention because the table
// has no shared mutable row to serialise on.
func TestCastVoteConcurrent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	reportID := seedReportForVotes(t, pool, "TestCastVoteConcurrent report")

	const n = 8
	accountIDs := make([]int64, n)
	for i := 0; i < n; i++ {
		accountIDs[i] = seedAccountForVotes(t, pool, voterEmail("concurrent-voter", i))
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{
				ReportID:    reportID,
				AccountID:   accountIDs[i],
				Kind:        "content",
				Value:       "confirm",
				GeohashCell: voterGeohashCell(i),
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else {
				t.Errorf("unexpected InsertVote error for goroutine %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	if successCount != n {
		t.Fatalf("successCount = %d, want exactly %d", successCount, n)
	}

	var rawCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM votes WHERE report_id = $1`, reportID).Scan(&rawCount); err != nil {
		t.Fatalf("counting raw votes: %v", err)
	}
	if rawCount != n {
		t.Fatalf("raw vote count = %d, want exactly %d", rawCount, n)
	}

	current, err := q.CurrentVotesForReports(ctx, []int64{reportID})
	if err != nil {
		t.Fatalf("CurrentVotesForReports: %v", err)
	}
	if len(current) != n {
		t.Fatalf("len(current) = %d, want exactly %d", len(current), n)
	}
}

// TestCastVoteConcurrentSameAccountKeepsEveryRow is the discriminating
// test. TestCastVoteConcurrent alone would still pass under the rejected
// upsert design (8 distinct accounts are 8 distinct keys, so an upsert
// keyed on report/account/kind would also produce 8 rows); this one would
// not. Under an upsert-keyed-on-(report, account, kind) design the raw
// count assertion below would read 1, not 8 — this test is the executable
// form of T-02-02's "structurally avoided, not locked" claim.
func TestCastVoteConcurrentSameAccountKeepsEveryRow(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	reportID := seedReportForVotes(t, pool, "TestCastVoteConcurrentSameAccountKeepsEveryRow report")
	accountID := seedAccountForVotes(t, pool, "same-account-voter@example.com")

	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			value := "confirm"
			if i%2 != 0 {
				value = "dispute"
			}
			err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{
				ReportID:    reportID,
				AccountID:   accountID,
				Kind:        "content",
				Value:       value,
				GeohashCell: "tdr1qg0",
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else {
				t.Errorf("unexpected InsertVote error for goroutine %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	if successCount != n {
		t.Fatalf("successCount = %d, want exactly %d", successCount, n)
	}

	// The full vote-change history Phase 3 needs must survive: every one of
	// the 8 concurrent casts is a distinct row, never overwritten.
	var rawCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM votes WHERE report_id = $1 AND account_id = $2`,
		reportID, accountID).Scan(&rawCount); err != nil {
		t.Fatalf("counting raw votes: %v", err)
	}
	if rawCount != n {
		t.Fatalf("raw vote count for (report, account) = %d, want exactly %d — an upsert design would read 1 here", rawCount, n)
	}

	current, err := q.CurrentVotesForReports(ctx, []int64{reportID})
	if err != nil {
		t.Fatalf("CurrentVotesForReports: %v", err)
	}
	if len(current) != 1 {
		t.Fatalf("len(current) = %d, want exactly 1", len(current))
	}

	// Compare against the independently-read MAX(id) row rather than a
	// hardcoded string, keeping the assertion deterministic regardless of
	// which goroutine won the race, while proving DISTINCT ON returns the
	// genuinely latest row rather than an arbitrary one.
	var latestValue string
	if err := pool.QueryRow(ctx,
		`SELECT value FROM votes WHERE report_id = $1 AND account_id = $2 ORDER BY id DESC LIMIT 1`,
		reportID, accountID).Scan(&latestValue); err != nil {
		t.Fatalf("reading latest raw value: %v", err)
	}
	if current[0].Value != latestValue {
		t.Fatalf("CurrentVotesForReports value = %q, want %q (the MAX(id) row's value)", current[0].Value, latestValue)
	}
}

// TestCurrentVoteIsLatestOnAccountChange proves D-02 at the storage layer:
// changing a vote appends rather than overwrites. It also exercises the
// id DESC tiebreaker with an identical-created_at sub-case — without
// id DESC in the query's ORDER BY, Postgres would be free to return either
// row there.
func TestCurrentVoteIsLatestOnAccountChange(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	reportID := seedReportForVotes(t, pool, "TestCurrentVoteIsLatestOnAccountChange report")
	accountID := seedAccountForVotes(t, pool, "vote-changer@example.com")

	if err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{
		ReportID: reportID, AccountID: accountID, Kind: "content", Value: "confirm", GeohashCell: "tdr1qg0",
	}); err != nil {
		t.Fatalf("first InsertVote: %v", err)
	}
	if err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{
		ReportID: reportID, AccountID: accountID, Kind: "content", Value: "dispute", GeohashCell: "tdr1qg0",
	}); err != nil {
		t.Fatalf("second InsertVote: %v", err)
	}

	var rawCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM votes WHERE report_id = $1 AND account_id = $2`,
		reportID, accountID).Scan(&rawCount); err != nil {
		t.Fatalf("counting raw votes: %v", err)
	}
	if rawCount != 2 {
		t.Fatalf("raw vote count = %d, want exactly 2 (a changed vote appends, D-02)", rawCount)
	}

	current, err := q.CurrentVotesForReports(ctx, []int64{reportID})
	if err != nil {
		t.Fatalf("CurrentVotesForReports: %v", err)
	}
	if len(current) != 1 {
		t.Fatalf("len(current) = %d, want exactly 1", len(current))
	}
	if current[0].Kind != "content" || current[0].Value != "dispute" {
		t.Fatalf("current vote = {Kind: %q, Value: %q}, want {content, dispute}", current[0].Kind, current[0].Value)
	}

	// Identical-created_at sub-case: makes the id DESC tiebreaker
	// load-bearing. Insert both rows in one multi-row statement so they
	// share a single explicitly supplied timestamp.
	tieReportID := seedReportForVotes(t, pool, "TestCurrentVoteIsLatestOnAccountChange tie report")
	tieCreatedAt := time.Now().UTC().Truncate(time.Second)
	testutil.MustExec(t, pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
		 VALUES ($1, $2, 'content', 'confirm', 'tdr1qg0', $3::timestamptz),
		        ($1, $2, 'content', 'dispute', 'tdr1qg0', $3::timestamptz)`,
		tieReportID, accountID, tieCreatedAt,
	)

	var tieLatestValue string
	if err := pool.QueryRow(ctx,
		`SELECT value FROM votes WHERE report_id = $1 ORDER BY id DESC LIMIT 1`,
		tieReportID).Scan(&tieLatestValue); err != nil {
		t.Fatalf("reading latest raw value for tie report: %v", err)
	}

	tieCurrent, err := q.CurrentVotesForReports(ctx, []int64{tieReportID})
	if err != nil {
		t.Fatalf("CurrentVotesForReports for tie report: %v", err)
	}
	if len(tieCurrent) != 1 {
		t.Fatalf("len(tieCurrent) = %d, want exactly 1", len(tieCurrent))
	}
	if tieCurrent[0].Value != tieLatestValue {
		t.Fatalf("tie-case current value = %q, want %q (the id DESC tiebreaker winner)", tieCurrent[0].Value, tieLatestValue)
	}
}

// TestVotesArePerReportPerAccountKind proves D-02 and D-16's separation of
// "is this report still true" (content) from "is this resolved"
// (resolution): they coexist as independent current rows, and a later
// content vote does not disturb the resolution row.
func TestVotesArePerReportPerAccountKind(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	reportID := seedReportForVotes(t, pool, "TestVotesArePerReportPerAccountKind report")
	accountID := seedAccountForVotes(t, pool, "kind-separation-voter@example.com")

	if err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{
		ReportID: reportID, AccountID: accountID, Kind: "content", Value: "confirm", GeohashCell: "tdr1qg0",
	}); err != nil {
		t.Fatalf("InsertVote content/confirm: %v", err)
	}
	if err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{
		ReportID: reportID, AccountID: accountID, Kind: "resolution", Value: "resolve", GeohashCell: "tdr1qg0",
	}); err != nil {
		t.Fatalf("InsertVote resolution/resolve: %v", err)
	}

	current, err := q.CurrentVotesForReports(ctx, []int64{reportID})
	if err != nil {
		t.Fatalf("CurrentVotesForReports: %v", err)
	}
	if len(current) != 2 {
		t.Fatalf("len(current) = %d, want exactly 2", len(current))
	}
	sort.Slice(current, func(i, j int) bool { return current[i].Kind < current[j].Kind })
	if current[0].Kind != "content" || current[0].Value != "confirm" {
		t.Fatalf("current[0] = {Kind: %q, Value: %q}, want {content, confirm}", current[0].Kind, current[0].Value)
	}
	if current[1].Kind != "resolution" || current[1].Value != "resolve" {
		t.Fatalf("current[1] = {Kind: %q, Value: %q}, want {resolution, resolve}", current[1].Kind, current[1].Value)
	}

	// A later content vote must not disturb the resolution row.
	if err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{
		ReportID: reportID, AccountID: accountID, Kind: "content", Value: "dispute", GeohashCell: "tdr1qg0",
	}); err != nil {
		t.Fatalf("InsertVote content/dispute: %v", err)
	}

	current2, err := q.CurrentVotesForReports(ctx, []int64{reportID})
	if err != nil {
		t.Fatalf("CurrentVotesForReports after second content vote: %v", err)
	}
	if len(current2) != 2 {
		t.Fatalf("len(current2) = %d, want exactly 2", len(current2))
	}
	sort.Slice(current2, func(i, j int) bool { return current2[i].Kind < current2[j].Kind })
	if current2[0].Kind != "content" || current2[0].Value != "dispute" {
		t.Fatalf("current2[0] = {Kind: %q, Value: %q}, want {content, dispute}", current2[0].Kind, current2[0].Value)
	}
	if current2[1].Kind != "resolution" || current2[1].Value != "resolve" {
		t.Fatalf("current2[1] = {Kind: %q, Value: %q}, want {resolution, resolve} (unchanged)", current2[1].Kind, current2[1].Value)
	}
}

// TestVotesHaveNoUniqueKeyBeyondPrimaryKey is the primary, database-level
// guard against T-02-02 (the lost-update class of bug) being silently
// reintroduced. If a later change adds UNIQUE (report_id, account_id,
// kind), this test fails — and it should, because that key is exactly what
// would turn a vote write from a lock-free append into a contended row,
// reintroducing the race this plan structurally avoids and destroying the
// vote-change history Phase 3 needs.
func TestVotesHaveNoUniqueKeyBeyondPrimaryKey(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	// (1) The only uniqueness-bearing constraint is the primary key on id.
	var constraintCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM pg_constraint WHERE conrelid = 'votes'::regclass AND contype IN ('u','p')`,
	).Scan(&constraintCount); err != nil {
		t.Fatalf("counting unique/primary-key constraints on votes: %v", err)
	}
	if constraintCount != 1 {
		t.Fatalf("unique/primary-key constraint count on votes = %d, want exactly 1 (only the id primary key)", constraintCount)
	}

	// (2) Catches a bare unique index created without a constraint, which
	// (1) above would miss.
	var uniqueIndexCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM pg_index WHERE indrelid = 'votes'::regclass AND indisunique`,
	).Scan(&uniqueIndexCount); err != nil {
		t.Fatalf("counting unique indexes on votes: %v", err)
	}
	if uniqueIndexCount != 1 {
		t.Fatalf("unique index count on votes = %d, want exactly 1", uniqueIndexCount)
	}

	// (3) That sole unique index must be the id primary key, not some other
	// uniqueness-bearing index. pg_get_indexdef is used rather than
	// inspecting indkey directly — indkey is an int2vector and joining it
	// against pg_attribute needs an explicit ::smallint[] cast that is easy
	// to get wrong for no added value here.
	var indexDef string
	if err := pool.QueryRow(ctx,
		`SELECT pg_get_indexdef(indexrelid) FROM pg_index WHERE indrelid = 'votes'::regclass AND indisunique`,
	).Scan(&indexDef); err != nil {
		t.Fatalf("reading unique index definition on votes: %v", err)
	}
	if !strings.HasSuffix(indexDef, "(id)") {
		t.Fatalf("unique index definition = %q, want it to end with (id)", indexDef)
	}
}

// forbiddenClauseIn strips SQL comment lines (leading -- after
// strings.TrimSpace) from sql, then returns the first of ON CONFLICT,
// FOR UPDATE, or FOR NO KEY UPDATE found in the remaining text (uppercased
// before scanning), or the empty string if none is present. Filtering out
// comment lines is mandatory, not optional: Task 1's rationale comments
// deliberately name the rejected patterns in prose, so an unfiltered scan
// would fail against the very comments that document the decision.
func forbiddenClauseIn(sql string) string {
	var kept []string
	for _, line := range strings.Split(sql, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		kept = append(kept, line)
	}
	upper := strings.ToUpper(strings.Join(kept, "\n"))

	for _, forbidden := range []string{"ON CONFLICT", "FOR UPDATE", "FOR NO KEY UPDATE"} {
		if strings.Contains(upper, forbidden) {
			return forbidden
		}
	}
	return ""
}

// TestVotesQuerySourceHasNoUpsertOrLock is the secondary, source-level
// guard, modelled on TestNearbyReportsQuerySourceHasExpectedShape
// (internal/store/reports_test.go). It needs no database and must run
// under go test ./... -short.
func TestVotesQuerySourceHasNoUpsertOrLock(t *testing.T) {
	data, err := os.ReadFile("queries/votes.sql")
	if err != nil {
		t.Fatalf("reading queries/votes.sql: %v", err)
	}
	src := string(data)

	if found := forbiddenClauseIn(src); found != "" {
		t.Fatalf("queries/votes.sql contains forbidden clause %q — see 02-RESEARCH.md Pitfall 1: votes is append-only and must never gain an upsert-conflict or row-lock clause", found)
	}

	upper := strings.ToUpper(src)
	if !strings.Contains(upper, "DISTINCT ON (REPORT_ID, ACCOUNT_ID, KIND)") {
		t.Fatalf("queries/votes.sql missing DISTINCT ON (report_id, account_id, kind) — the current-vote read's deduplication key must not be dropped silently")
	}
	if !strings.Contains(upper, "ID DESC") {
		t.Fatalf("queries/votes.sql missing id DESC — the current-vote read's tiebreaker must not be dropped silently")
	}

	// Negative control: prove the guard can actually fail. A hardcoded
	// known-bad counter-example string carrying an upsert-conflict clause
	// in its statement body, plus a line of -- comment prose that also
	// names that clause — the comment-filter must not blind the helper
	// into passing.
	const knownBad = `-- name: BadInsertVote :exec
-- This comment mentions ON CONFLICT in prose but must not trigger a match.
INSERT INTO votes (report_id, account_id, kind, value, geohash_cell)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (report_id, account_id, kind) DO UPDATE SET value = EXCLUDED.value;
`
	if got := forbiddenClauseIn(knownBad); got != "ON CONFLICT" {
		t.Fatalf("forbiddenClauseIn(knownBad) = %q, want %q — the guard must detect a real forbidden clause in the statement body, not be fooled by the comment line naming it", got, "ON CONFLICT")
	}
}

// TestVotedReportsByAccountKeepsTheLatestContentVote proves the Activity page's
// My votes read (quick task 261013-wm5): one row per report holding the
// account's latest content vote (D-02 latest wins), no resolution only report,
// no other account's votes, and the report's own columns.
func TestVotedReportsByAccountKeepsTheLatestContentVote(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	r1 := seedReportForVotes(t, pool, "voted-by-account-r1")
	r2 := seedReportForVotes(t, pool, "voted-by-account-r2")
	r3 := seedReportForVotes(t, pool, "voted-by-account-r3")
	acct := seedAccountForVotes(t, pool, "voted-a@example.com")
	other := seedAccountForVotes(t, pool, "voted-b@example.com")

	insert := func(report, account int64, kind, value, at string) {
		testutil.MustExec(t, pool,
			`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
			 VALUES ($1, $2, $3, $4, 'tdr1qg0', $5::timestamptz)`,
			report, account, kind, value, at)
	}
	// r1: confirmed, then disputed later. The later vote must win.
	insert(r1, acct, "content", "confirm", "2026-10-01T10:00:00Z")
	insert(r1, acct, "content", "dispute", "2026-10-01T11:00:00Z")
	// r2: a single confirm.
	insert(r2, acct, "content", "confirm", "2026-10-01T09:00:00Z")
	// r3: the account only resolved it, which is not a content vote.
	insert(r3, acct, "resolution", "resolve", "2026-10-01T08:00:00Z")
	// Another account's votes never appear.
	insert(r1, other, "content", "confirm", "2026-10-01T12:00:00Z")
	insert(r3, other, "content", "dispute", "2026-10-01T12:30:00Z")

	q := sqlcgen.New(pool)
	rows, err := q.VotedReportsByAccount(ctx, sqlcgen.VotedReportsByAccountParams{AccountID: acct, Kind: "content"})
	if err != nil {
		t.Fatalf("VotedReportsByAccount: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (r1 and r2): %+v", len(rows), rows)
	}
	byID := map[int64]sqlcgen.VotedReportsByAccountRow{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	if got := byID[r1].VoteValue; got != "dispute" {
		t.Errorf("r1 vote value = %q, want dispute (latest wins)", got)
	}
	if want := time.Date(2026, 10, 1, 11, 0, 0, 0, time.UTC); !byID[r1].VotedAt.Equal(want) {
		t.Errorf("r1 voted_at = %v, want %v", byID[r1].VotedAt, want)
	}
	if got := byID[r2].VoteValue; got != "confirm" {
		t.Errorf("r2 vote value = %q, want confirm", got)
	}
	if _, ok := byID[r3]; ok {
		t.Error("r3 appeared although the account only cast a resolution vote")
	}

	// The eleven report columns match the report itself.
	var wantDesc string
	var wantLat, wantLon float64
	var wantGeohash, wantCategory, wantSeverity string
	if err := pool.QueryRow(ctx,
		`SELECT description, latitude, longitude, geohash, category, severity FROM reports WHERE id = $1`, r1,
	).Scan(&wantDesc, &wantLat, &wantLon, &wantGeohash, &wantCategory, &wantSeverity); err != nil {
		t.Fatalf("reading report r1: %v", err)
	}
	got := byID[r1]
	if got.Description != wantDesc || got.Latitude != wantLat || got.Longitude != wantLon ||
		got.Geohash != wantGeohash || got.Category != wantCategory || got.Severity != wantSeverity {
		t.Errorf("report columns of r1 = %+v do not match the report", got)
	}
	if got.ExpiresAt.IsZero() || got.CreatedAt.IsZero() {
		t.Errorf("report times of r1 not populated: %+v", got)
	}

	// The other account's own read holds its own two votes only.
	otherRows, err := q.VotedReportsByAccount(ctx, sqlcgen.VotedReportsByAccountParams{AccountID: other, Kind: "content"})
	if err != nil {
		t.Fatalf("VotedReportsByAccount other: %v", err)
	}
	if len(otherRows) != 2 {
		t.Errorf("other account got %d rows, want 2", len(otherRows))
	}
}

// TestVotedReportsByAccountUsesTheAccountIndex proves migration 00006 built
// the index with its planned column list and that the planner names it for the
// query when sequential scans are off.
func TestVotedReportsByAccountUsesTheAccountIndex(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	var indexDef string
	if err := pool.QueryRow(ctx,
		`SELECT indexdef FROM pg_indexes WHERE indexname = 'idx_votes_account_kind_report_created'`,
	).Scan(&indexDef); err != nil {
		t.Fatalf("reading idx_votes_account_kind_report_created: %v", err)
	}
	const wantCols = "(account_id, kind, report_id, created_at DESC, id DESC)"
	if !strings.Contains(indexDef, wantCols) {
		t.Fatalf("index definition = %q, want it to contain %q", indexDef, wantCols)
	}

	data, err := os.ReadFile("queries/votes.sql")
	if err != nil {
		t.Fatalf("reading queries/votes.sql: %v", err)
	}
	src := string(data)
	at := strings.Index(src, "-- name: VotedReportsByAccount :many")
	if at < 0 {
		t.Fatal("queries/votes.sql has no VotedReportsByAccount block")
	}
	var stmt []string
	for _, line := range strings.Split(src[at:], "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		stmt = append(stmt, line)
	}
	stmtSQL := strings.Join(stmt, "\n")
	stmtSQL = strings.Replace(stmtSQL, "sqlc.arg(account_id)", "$1", 1)
	stmtSQL = strings.Replace(stmtSQL, "sqlc.arg(kind)", "$2", 1)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
		t.Fatalf("SET LOCAL enable_seqscan: %v", err)
	}
	// EXPLAIN cannot take bind parameters for a generic plan, so the two
	// arguments are written as literals of the right types.
	explainSQL := strings.Replace(stmtSQL, "$1", "1", 1)
	explainSQL = strings.Replace(explainSQL, "$2", "'content'", 1)
	rows, err := tx.Query(ctx, "EXPLAIN "+explainSQL)
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan line: %v", err)
		}
		plan = append(plan, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	if !strings.Contains(strings.Join(plan, "\n"), "idx_votes_account_kind_report_created") {
		t.Fatalf("plan does not name idx_votes_account_kind_report_created:\n%s", strings.Join(plan, "\n"))
	}
}

// TestReportVoteContextReturnsCoordinates proves the vote context read
// returns the report's latitude, longitude and creation time alongside the
// reporter, severity, category and expiry (D-01, V-09). CastVote reads the
// report point from here, never from the client.
func TestReportVoteContextReturnsCoordinates(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	reportID := seedReportForVotes(t, pool, "TestReportVoteContextReturnsCoordinates report")

	created := time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC)
	testutil.MustExec(t, pool, "UPDATE reports SET created_at = $1 WHERE id = $2", created, reportID)

	q := sqlcgen.New(pool)
	row, err := q.ReportVoteContext(ctx, reportID)
	if err != nil {
		t.Fatalf("ReportVoteContext: %v", err)
	}
	if row.Latitude != 12.9716 {
		t.Errorf("Latitude = %v, want 12.9716", row.Latitude)
	}
	if row.Longitude != 77.5946 {
		t.Errorf("Longitude = %v, want 77.5946", row.Longitude)
	}
	if !row.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", row.CreatedAt, created)
	}
	if row.Severity != "low" {
		t.Errorf("Severity = %q, want low", row.Severity)
	}
	if row.Category != "flood" {
		t.Errorf("Category = %q, want flood", row.Category)
	}
	if row.ReporterAccountID != nil {
		t.Errorf("ReporterAccountID = %v, want nil (no sessions row)", *row.ReporterAccountID)
	}

	if _, err := q.ReportVoteContext(ctx, 999999); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("missing report error = %v, want pgx.ErrNoRows", err)
	}
}
