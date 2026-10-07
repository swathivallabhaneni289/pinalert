// Package store_test tests the reporter history read (D-10, D-13, R-05): the
// ReporterHistoryReports query returns, for a batch of reporter accounts, each
// account's latest expired reports across all of its sessions, capped per
// account and bounded to the reliability window. The query source, the hand
// copied EXPLAIN statement and the sqlc generated const are held equal here.
package store_test

import (
	"context"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// reporterHistorySQLForExplain must stay equal, after whitespace
// normalisation, to the ReporterHistoryReports statement in
// internal/store/queries/reliability.sql with each sqlc.arg replaced by its
// positional parameter, and to the const sqlc generated from it (sqlc emits
// the query as an unexported const, unreachable from this external test
// package). TestReporterHistoryQuerySourceHasExpectedShape is the drift guard
// that keeps the three from diverging silently.
const reporterHistorySQLForExplain = `
SELECT t.account_id::bigint AS account_id, t.report_id, t.severity, t.category, t.expires_at
FROM (
    SELECT x.account_id, rr.id AS report_id, rr.severity, rr.category, rr.expires_at,
           row_number() OVER (PARTITION BY x.account_id ORDER BY rr.expires_at DESC, rr.id DESC) AS rn
    FROM sessions x
    CROSS JOIN LATERAL (
        SELECT r.id, r.severity, r.category, r.expires_at
        FROM reports r
        WHERE r.session_id = x.session_id
          AND r.expires_at <= $1::timestamptz
          AND r.expires_at > $2::timestamptz
        ORDER BY r.expires_at DESC, r.id DESC
        LIMIT $3::int
    ) rr
    WHERE x.account_id = ANY($4::bigint[])
) t
WHERE t.rn <= $3::int
ORDER BY t.account_id, t.expires_at DESC, t.report_id DESC
`

// Fixture values for every test in this file. They mirror D-13 (a 90 day
// window and a row cap of 30). In production plan 03-12 passes
// service.ReliabilityRowCap and now minus service.ReliabilityWindow, but this
// store test fixes its own values so it depends on no service constant.
var (
	histNow    = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	histWindow = 90 * 24 * time.Hour
)

const histRowCap = int32(30)

// histSQLBody returns the statement of src with every comment line dropped,
// the remaining lines joined and trimmed, and one trailing semicolon removed.
func histSQLBody(src string) string {
	var kept []string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		kept = append(kept, line)
	}
	body := strings.TrimSpace(strings.Join(kept, "\n"))
	return strings.TrimSuffix(body, ";")
}

// histNormalise collapses every run of whitespace to one space.
func histNormalise(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// histGeneratedConst returns the statement text of the sqlc generated const
// reporterHistoryReports, with its leading "-- name:" line removed.
func histGeneratedConst(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("sqlc/reliability.sql.go")
	if err != nil {
		t.Fatalf("reading sqlc/reliability.sql.go: %v", err)
	}
	const marker = "const reporterHistoryReports = `"
	text := string(data)
	start := strings.Index(text, marker)
	if start < 0 {
		t.Fatalf("sqlc/reliability.sql.go has no %q; run make sqlc", marker)
	}
	rest := text[start+len(marker):]
	end := strings.Index(rest, "`")
	if end < 0 {
		t.Fatalf("sqlc/reliability.sql.go: unterminated reporterHistoryReports const")
	}
	var kept []string
	for _, line := range strings.Split(rest[:end], "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "-- name:") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// histArg builds the query parameters: now is histNow, the window start is
// histNow minus histWindow.
func histArg(accountIDs []int64, rowCap int32) sqlcgen.ReporterHistoryReportsParams {
	return sqlcgen.ReporterHistoryReportsParams{
		Now:         histNow,
		WindowStart: histNow.Add(-histWindow),
		RowCap:      rowCap,
		AccountIds:  accountIDs,
	}
}

func TestReporterHistoryQuerySourceHasExpectedShape(t *testing.T) {
	data, err := os.ReadFile("queries/reliability.sql")
	if err != nil {
		t.Fatalf("reading queries/reliability.sql: %v", err)
	}
	src := string(data)

	if got := strings.Count(src, "-- name:"); got != 1 {
		t.Fatalf("queries/reliability.sql declares %d queries, want exactly 1", got)
	}
	if !strings.Contains(src, "-- name: ReporterHistoryReports :many\n") {
		t.Fatalf("queries/reliability.sql must declare ReporterHistoryReports :many")
	}

	replacer := strings.NewReplacer(
		"sqlc.arg(now)", "$1",
		"sqlc.arg(window_start)", "$2",
		"sqlc.arg(row_cap)", "$3",
		"sqlc.arg(account_ids)", "$4",
	)
	replaced := replacer.Replace(histSQLBody(src))
	if strings.Contains(replaced, "sqlc.arg(") {
		t.Fatalf("an unexpected sqlc.arg remains after replacement: %s", replaced)
	}

	fromSource := histNormalise(replaced)
	fromConst := histNormalise(strings.TrimSuffix(strings.TrimSpace(reporterHistorySQLForExplain), ";"))
	fromGenerated := histNormalise(histGeneratedConst(t))

	if fromSource != fromConst {
		t.Fatalf("queries/reliability.sql drifted from reporterHistorySQLForExplain\nsource: %s\nconst:  %s", fromSource, fromConst)
	}
	if fromGenerated != fromConst {
		t.Fatalf("sqlc/reliability.sql.go drifted from reporterHistorySQLForExplain (run make sqlc)\ngenerated: %s\nconst:     %s", fromGenerated, fromConst)
	}

	rules := []struct{ rule, want string }{
		{"column list", "SELECT t.account_id::bigint AS account_id, t.report_id, t.severity, t.category, t.expires_at FROM ("},
		{"D-10 account keying", "FROM sessions x CROSS JOIN LATERAL ("},
		{"D-13 per session scan", "WHERE r.session_id = x.session_id"},
		{"R-05 upper window edge is inclusive", "r.expires_at <= $1::timestamptz"},
		{"R-05 lower window edge is exclusive", "r.expires_at > $2::timestamptz"},
		{"D-13 per session cap", "ORDER BY r.expires_at DESC, r.id DESC LIMIT $3::int"},
		{"D-10 partition by account", "PARTITION BY x.account_id ORDER BY rr.expires_at DESC, rr.id DESC"},
		{"D-10 account list", "WHERE x.account_id = ANY($4::bigint[])"},
		{"D-13 per account cap", "WHERE t.rn <= $3::int"},
	}
	for _, r := range rules {
		if !strings.Contains(fromSource, r.want) {
			t.Errorf("%s: statement must contain %q", r.rule, r.want)
		}
	}
	const wantSuffix = "ORDER BY t.account_id, t.expires_at DESC, t.report_id DESC"
	if !strings.HasSuffix(fromSource, wantSuffix) {
		t.Errorf("output order: statement must end with %q", wantSuffix)
	}

	lower := strings.ToLower(fromSource)
	for _, banned := range []string{"now()", "interval", "geohash", "description", "email"} {
		if strings.Contains(lower, banned) {
			t.Errorf("statement must not contain %q (D-13 parameters only, no text or location in the output)", banned)
		}
	}
	if m := regexp.MustCompile(`\b(30|90)\b`).FindString(lower); m != "" {
		t.Errorf("statement holds the literal %s; the cap and the window come from parameters (D-13)", m)
	}

	// Negative control 1: dropping the id tie break from the per session scan
	// must make the normalised source differ, so the equality above can fail.
	broken := strings.Replace(fromConst, "r.id DESC LIMIT", "LIMIT", 1)
	if broken == fromConst {
		t.Fatalf("negative control 1 did not change the statement")
	}
	if broken == fromSource {
		t.Errorf("negative control 1: a statement without the id tie break compares equal to the source")
	}

	// Negative control 2: a comment line that mentions now() must not hide a
	// real call on a body line, so the comment filter cannot blind the scan.
	bad := "-- now() is explained here\nSELECT now();"
	if !strings.Contains(strings.ToLower(histSQLBody(bad)), "now()") {
		t.Errorf("negative control 2: the comment filter hid a real now() call")
	}
	if strings.Contains(strings.ToLower(histSQLBody("-- now() is explained here\nSELECT 1;")), "now()") {
		t.Errorf("negative control 2: the comment filter kept a comment line")
	}
}

// histSeverities and histCategories seed the A account fixture; the severity
// and category of each report id are remembered so rows can be checked.
var histSeverities = []string{"low", "medium", "critical"}

type histMeta struct{ severity, category string }

// histAssertRows checks that rows are ordered by account id, then expires_at
// DESC, then report id DESC, all inside (windowStart, histNow].
func histAssertRows(t *testing.T, rows []sqlcgen.ReporterHistoryReportsRow) {
	t.Helper()
	windowStart := histNow.Add(-histWindow)
	for i, r := range rows {
		if r.ExpiresAt.After(histNow) {
			t.Errorf("row %d (report %d) expires after now: %v", i, r.ReportID, r.ExpiresAt)
		}
		if !r.ExpiresAt.After(windowStart) {
			t.Errorf("row %d (report %d) is not after the window start: %v", i, r.ReportID, r.ExpiresAt)
		}
		if i == 0 {
			continue
		}
		p := rows[i-1]
		switch {
		case p.AccountID < r.AccountID:
		case p.AccountID > r.AccountID:
			t.Errorf("row %d: account ids are not ascending (%d then %d)", i, p.AccountID, r.AccountID)
		case p.ExpiresAt.After(r.ExpiresAt):
		case p.ExpiresAt.Before(r.ExpiresAt):
			t.Errorf("row %d: expires_at is not descending within account %d", i, r.AccountID)
		case p.ReportID <= r.ReportID:
			t.Errorf("row %d: report id is not descending on an expires_at tie within account %d", i, r.AccountID)
		}
	}
}

func histIDs(rows []sqlcgen.ReporterHistoryReportsRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ReportID)
	}
	return ids
}

func TestReporterHistoryBounds(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	windowStart := histNow.Add(-histWindow)

	// Accounts are created in the order A, B, C, D so A's id is lower than
	// B's, and "A's rows, then B's rows" follows from ORDER BY account_id.
	accA := testutil.SeedReporterAccount(t, pool, "hist-a@example.com", "hist-a-phone", "hist-a-laptop")
	accB := testutil.SeedReporterAccount(t, pool, "hist-b@example.com", "hist-b-only")
	accC := testutil.SeedReporterAccount(t, pool, "hist-c@example.com", "hist-c-only")
	accD := testutil.SeedReporterAccount(t, pool, "hist-d@example.com", "hist-d-only")
	testutil.MustExec(t, pool, "INSERT INTO sessions (session_id) VALUES ($1)", "hist-unbound")

	meta := map[int64]histMeta{}

	// Account A: 40 reports, report k expiring k hours before now, odd k from
	// the phone and even k from the laptop. aIDs[k] is report k's id.
	aIDs := map[int]int64{}
	aSessions := map[int64]string{}
	for k := 1; k <= 40; k++ {
		session, category := "hist-a-phone", "flood"
		if k%2 == 0 {
			session, category = "hist-a-laptop", "rescue_needed"
		}
		severity := histSeverities[k%3]
		id := testutil.SeedHistoryReport(t, pool, session, severity, category, histNow.Add(-time.Duration(k)*time.Hour))
		aIDs[k] = id
		aSessions[id] = session
		meta[id] = histMeta{severity, category}
	}

	// Account B: seven reports probing both window edges and the tie break.
	seedB := func(expires time.Time) int64 {
		id := testutil.SeedHistoryReport(t, pool, "hist-b-only", "medium", "road_blocked", expires)
		meta[id] = histMeta{"medium", "road_blocked"}
		return id
	}
	b1 := seedB(histNow)
	b2 := seedB(histNow.Add(-time.Hour))
	b3 := seedB(histNow.Add(-time.Hour))
	b4 := seedB(windowStart.Add(time.Minute))
	b5 := seedB(windowStart)
	b6 := seedB(windowStart.Add(-24 * time.Hour))
	b7 := seedB(histNow.Add(time.Minute))
	_, _, _ = b5, b6, b7
	// b2 carries B's own resolve vote and is still returned (R-05): a report
	// is judged once it expires whether or not its reporter resolved it.
	testutil.MustExec(t, pool,
		"INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, 'resolution', 'resolve', 'tdr1qgz')",
		b2, accB)

	// Account C is never requested. D is requested and has no reports.
	var cIDs []int64
	for i := 1; i <= 2; i++ {
		id := testutil.SeedHistoryReport(t, pool, "hist-c-only", "low", "flood", histNow.Add(-time.Duration(i)*time.Hour))
		cIDs = append(cIDs, id)
	}
	// A session with no account and a session id with no sessions row.
	testutil.SeedHistoryReport(t, pool, "hist-unbound", "low", "flood", histNow.Add(-time.Hour))
	testutil.SeedHistoryReport(t, pool, "hist-orphan", "low", "flood", histNow.Add(-time.Hour))

	rows, err := q.ReporterHistoryReports(ctx, histArg([]int64{accA, accB, accD}, histRowCap))
	if err != nil {
		t.Fatalf("ReporterHistoryReports: %v", err)
	}
	if len(rows) != 34 {
		t.Fatalf("got %d rows, want 34 (A's 30 then B's 4)", len(rows))
	}
	if len(rows) > 3*int(histRowCap) {
		t.Fatalf("got %d rows for 3 accounts, more than 3 times the cap", len(rows))
	}
	histAssertRows(t, rows)

	// A: exactly the reports for k = 1 to 30 in that order, from both sessions.
	sessionsSeen := map[string]bool{}
	for i := 0; i < 30; i++ {
		r := rows[i]
		want := aIDs[i+1]
		if r.AccountID != accA || r.ReportID != want {
			t.Fatalf("row %d = account %d report %d, want account %d report %d (k=%d)", i, r.AccountID, r.ReportID, accA, want, i+1)
		}
		sessionsSeen[aSessions[r.ReportID]] = true
	}
	if len(sessionsSeen) != 2 {
		t.Errorf("A's rows came from %d sessions, want both hist-a-phone and hist-a-laptop", len(sessionsSeen))
	}
	// B: [b1, b3, b2, b4].
	gotB := histIDs(rows[30:])
	wantB := []int64{b1, b3, b2, b4}
	if !reflect.DeepEqual(gotB, wantB) {
		t.Fatalf("B's rows = %v, want %v (b1, b3, b2, b4)", gotB, wantB)
	}
	for _, r := range rows[30:] {
		if r.AccountID != accB {
			t.Errorf("row for report %d has account %d, want B %d", r.ReportID, r.AccountID, accB)
		}
	}
	// Every severity and category equals what was seeded.
	for _, r := range rows {
		m, ok := meta[r.ReportID]
		if !ok {
			t.Fatalf("row for report %d, which was never seeded for A or B", r.ReportID)
		}
		if r.Severity != m.severity || r.Category != m.category {
			t.Errorf("report %d = %s/%s, want %s/%s", r.ReportID, r.Severity, r.Category, m.severity, m.category)
		}
	}

	// Row cap 2 for [A, B]: A's k=1 (phone) and k=2 (laptop), then b1 and b3.
	capped, err := q.ReporterHistoryReports(ctx, histArg([]int64{accA, accB}, 2))
	if err != nil {
		t.Fatalf("ReporterHistoryReports cap 2: %v", err)
	}
	wantCapped := []int64{aIDs[1], aIDs[2], b1, b3}
	if got := histIDs(capped); !reflect.DeepEqual(got, wantCapped) {
		t.Fatalf("row cap 2 = %v, want %v", got, wantCapped)
	}

	// A duplicated account id returns that account's rows once.
	dup, err := q.ReporterHistoryReports(ctx, histArg([]int64{accB, accB}, histRowCap))
	if err != nil {
		t.Fatalf("ReporterHistoryReports duplicate ids: %v", err)
	}
	if got := histIDs(dup); !reflect.DeepEqual(got, wantB) {
		t.Fatalf("[B, B] = %v, want B's rows once: %v", got, wantB)
	}

	// C alone returns C's two rows, newest first.
	cRows, err := q.ReporterHistoryReports(ctx, histArg([]int64{accC}, histRowCap))
	if err != nil {
		t.Fatalf("ReporterHistoryReports [C]: %v", err)
	}
	if got, want := histIDs(cRows), []int64{cIDs[0], cIDs[1]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("[C] = %v, want %v", got, want)
	}

	// A nil list and an empty list each return no rows and no error.
	for name, ids := range map[string][]int64{"nil": nil, "empty": {}} {
		none, err := q.ReporterHistoryReports(ctx, histArg(ids, histRowCap))
		if err != nil {
			t.Fatalf("ReporterHistoryReports with %s account list: %v", name, err)
		}
		if len(none) != 0 {
			t.Errorf("%s account list returned %d rows, want 0", name, len(none))
		}
	}
}

func TestReporterHistoryUsesAccountNotSession(t *testing.T) {
	// The row and params structs must have exactly the pinned fields, so no
	// field is session shaped and no caller can key the query by a session
	// (D-10, V-28). Checked before the database opens so it also runs under
	// -short with no database.
	assertFields := func(v any, want []string) {
		t.Helper()
		typ := reflect.TypeOf(v)
		var got []string
		for i := 0; i < typ.NumField(); i++ {
			got = append(got, typ.Field(i).Name)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s fields = %v, want exactly %v", typ.Name(), got, want)
		}
	}
	assertFields(sqlcgen.ReporterHistoryReportsRow{}, []string{"AccountID", "ReportID", "Severity", "Category", "ExpiresAt"})
	assertFields(sqlcgen.ReporterHistoryReportsParams{}, []string{"Now", "WindowStart", "RowCap", "AccountIds"})

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	// X is created before Y so X's account id is lower and sorts first.
	accX := testutil.SeedReporterAccount(t, pool, "hist-x@example.com", "hist-x-phone", "hist-x-laptop", "hist-x-tablet")
	accY := testutil.SeedReporterAccount(t, pool, "hist-y@example.com", "hist-y-phone")
	testutil.MustExec(t, pool, "INSERT INTO sessions (session_id) VALUES ($1)", "hist-unbound")

	owner := map[int64]int64{}
	seed := func(account int64, session string, ago time.Duration) int64 {
		id := testutil.SeedHistoryReport(t, pool, session, "medium", "flood", histNow.Add(-ago))
		owner[id] = account
		return id
	}
	x1 := seed(accX, "hist-x-phone", time.Hour)
	x2 := seed(accX, "hist-x-laptop", 2*time.Hour)
	x3 := seed(accX, "hist-x-tablet", 3*time.Hour)
	y1 := seed(accY, "hist-y-phone", 30*time.Minute)
	stray := testutil.SeedHistoryReport(t, pool, "hist-unbound", "medium", "flood", histNow.Add(-10*time.Minute))

	check := func(name string, ids []int64, want []int64) {
		t.Helper()
		rows, err := q.ReporterHistoryReports(ctx, histArg(ids, histRowCap))
		if err != nil {
			t.Fatalf("%s: ReporterHistoryReports: %v", name, err)
		}
		if got := histIDs(rows); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
		for _, r := range rows {
			if r.ReportID == stray {
				t.Errorf("%s: a report from an unbound session appeared", name)
			}
			if r.AccountID != owner[r.ReportID] {
				t.Errorf("%s: report %d carries account %d, want the filing account %d", name, r.ReportID, r.AccountID, owner[r.ReportID])
			}
		}
	}
	check("[X]", []int64{accX}, []int64{x1, x2, x3})
	check("[Y]", []int64{accY}, []int64{y1})
	check("[X, Y]", []int64{accX, accY}, []int64{x1, x2, x3, y1})
}
