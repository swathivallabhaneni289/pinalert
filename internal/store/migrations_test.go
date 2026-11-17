package store

import (
	"database/sql"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver, used by goose
	"github.com/pressly/goose/v3"
)

// TestMigrationsEmbedded asserts the embedded FS actually contains the first
// schema migration and that its contents declare both goose direction
// markers and both table names. It also covers the Phase 3 history index
// migration (D-13). This is a pure unit test with no database requirement,
// so it runs under `go test ./... -short`.
func TestMigrationsEmbedded(t *testing.T) {
	const migrationFile = "migrations/00001_create_reports.sql"

	entries, err := MigrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("reading embedded migrations dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Name() == "00001_create_reports.sql" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s to be embedded, entries: %v", migrationFile, entries)
	}

	contents, err := MigrationsFS.ReadFile(migrationFile)
	if err != nil {
		t.Fatalf("reading embedded migration file: %v", err)
	}
	sql := string(contents)

	if !strings.Contains(sql, "+goose Up") {
		t.Error("expected migration to contain a +goose Up marker")
	}
	if !strings.Contains(sql, "+goose Down") {
		t.Error("expected migration to contain a +goose Down marker")
	}
	if !strings.Contains(sql, "CREATE TABLE reports") {
		t.Error("expected migration to create the reports table")
	}
	if !strings.Contains(sql, "CREATE TABLE sessions") {
		t.Error("expected migration to create the sessions table")
	}

	// Phase 3 history index (D-13, V-27): migration 00005 must be embedded,
	// carry its four markers in order, and build the index without a
	// concurrent build keyword, because goose runs each migration inside a
	// transaction.
	const indexFile = "migrations/00005_add_reports_session_expires_index.sql"
	indexFound := false
	for _, e := range entries {
		if e.Name() == "00005_add_reports_session_expires_index.sql" {
			indexFound = true
			break
		}
	}
	if !indexFound {
		t.Fatalf("expected %s to be embedded, entries: %v", indexFile, entries)
	}
	indexContents, err := MigrationsFS.ReadFile(indexFile)
	if err != nil {
		t.Fatalf("reading embedded migration file %s: %v", indexFile, err)
	}
	indexSQL := string(indexContents)

	markers := []string{
		"+goose Up",
		"CREATE INDEX idx_reports_session_expires ON reports (session_id, expires_at DESC, id DESC)",
		"+goose Down",
		"DROP INDEX idx_reports_session_expires",
	}
	prev := -1
	for _, m := range markers {
		at := strings.Index(indexSQL, m)
		if at < 0 {
			t.Fatalf("expected %s to contain %q", indexFile, m)
		}
		if at <= prev {
			t.Fatalf("expected %q to come after the previous marker in %s", m, indexFile)
		}
		prev = at
	}

	var code []string
	for _, line := range strings.Split(indexSQL, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(code, "\n")), "CONCURRENTLY") {
		t.Error("migration 00005 must not build the index concurrently: goose runs it inside a transaction")
	}

	// Quick task 261013-wm5 votes by account index: migration 00006 must be
	// embedded, carry its four markers in order, and build the index without a
	// concurrent build keyword.
	const votesIndexFile = "migrations/00006_add_votes_account_kind_index.sql"
	votesIndexFound := false
	for _, e := range entries {
		if e.Name() == "00006_add_votes_account_kind_index.sql" {
			votesIndexFound = true
			break
		}
	}
	if !votesIndexFound {
		t.Fatalf("expected %s to be embedded, entries: %v", votesIndexFile, entries)
	}
	votesIndexContents, err := MigrationsFS.ReadFile(votesIndexFile)
	if err != nil {
		t.Fatalf("reading embedded migration file %s: %v", votesIndexFile, err)
	}
	votesIndexSQL := string(votesIndexContents)

	votesMarkers := []string{
		"+goose Up",
		"CREATE INDEX idx_votes_account_kind_report_created ON votes (account_id, kind, report_id, created_at DESC, id DESC)",
		"+goose Down",
		"DROP INDEX idx_votes_account_kind_report_created",
	}
	votesPrev := -1
	for _, m := range votesMarkers {
		at := strings.Index(votesIndexSQL, m)
		if at < 0 {
			t.Fatalf("expected %s to contain %q", votesIndexFile, m)
		}
		if at <= votesPrev {
			t.Fatalf("expected %q to come after the previous marker in %s", m, votesIndexFile)
		}
		votesPrev = at
	}

	var votesCode []string
	for _, line := range strings.Split(votesIndexSQL, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		votesCode = append(votesCode, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(votesCode, "\n")), "CONCURRENTLY") {
		t.Error("migration 00006 must not build the index concurrently: goose runs it inside a transaction")
	}

	// Phase 4 moderation and idempotency schema (R-01, R-05, D-04, D-08,
	// D-09, D-19): exactly one embedded file, found by suffix because its
	// number is whatever was free when it was written.
	phase4Name := phase4MigrationName(t)
	phase4Contents, err := MigrationsFS.ReadFile("migrations/" + phase4Name)
	if err != nil {
		t.Fatalf("reading embedded migration file %s: %v", phase4Name, err)
	}
	for _, want := range []string{
		"+goose Up",
		"+goose Down",
		"moderation_state",
		"request_account_id",
		"client_written_at",
		"WHERE request_id IS NOT NULL",
		"CREATE TABLE moderation_checks",
		"CREATE TABLE report_flags",
		"CREATE TABLE moderation_actions",
	} {
		if !strings.Contains(string(phase4Contents), want) {
			t.Errorf("%s must contain %q", phase4Name, want)
		}
	}
}

// phase4MigrationName returns the one embedded migration file that adds the
// Phase 4 moderation and idempotency schema, found by suffix.
func phase4MigrationName(t *testing.T) string {
	t.Helper()
	entries, err := MigrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("reading embedded migrations dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_add_moderation_and_idempotency.sql") {
			names = append(names, e.Name())
		}
	}
	if len(names) != 1 {
		t.Fatalf("expected exactly one *_add_moderation_and_idempotency.sql migration, got %v", names)
	}
	return names[0]
}

var (
	createTableRE = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)`)
	dropTableRE   = regexp.MustCompile(`(?i)DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?([a-z_][a-z0-9_]*)`)
)

// upSectionTables returns the tables created and dropped by one Up section.
// Comment lines are removed first so a WHY comment that names a table never
// adds or removes a name.
func upSectionTables(up string) (created, dropped map[string]bool) {
	created = map[string]bool{}
	dropped = map[string]bool{}
	var code []string
	for _, line := range strings.Split(up, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	text := strings.Join(code, "\n")
	for _, m := range createTableRE.FindAllStringSubmatch(text, -1) {
		created[strings.ToLower(m[1])] = true
	}
	for _, m := range dropTableRE.FindAllStringSubmatch(text, -1) {
		dropped[strings.ToLower(m[1])] = true
	}
	return created, dropped
}

// TestMigrationsTruncateListsEveryTable keeps testutil.Truncate honest: the
// tables the migrations create must equal the tables the TRUNCATE literal
// names, in both directions. It reads the file because package store cannot
// import internal/testutil (testutil imports store).
func TestMigrationsTruncateListsEveryTable(t *testing.T) {
	// Negative control: a comment naming a table adds nothing.
	ghost, _ := upSectionTables("-- CREATE TABLE ghost\nSELECT 1;\n")
	if ghost["ghost"] {
		t.Fatal("upSectionTables must ignore comment lines")
	}

	entries, err := MigrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("reading embedded migrations dir: %v", err)
	}
	made := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		b, err := MigrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		text := string(b)
		upAt := strings.Index(text, "-- +goose Up")
		downAt := strings.Index(text, "-- +goose Down")
		if upAt < 0 || downAt < upAt {
			t.Fatalf("%s: goose markers missing or out of order", e.Name())
		}
		created, dropped := upSectionTables(text[upAt:downAt])
		for n := range created {
			made[n] = true
		}
		for n := range dropped {
			delete(made, n)
		}
	}

	src, err := os.ReadFile("../testutil/db.go")
	if err != nil {
		t.Fatalf("reading testutil/db.go: %v", err)
	}
	s := string(src)
	from := strings.Index(s, "TRUNCATE ")
	to := strings.Index(s, " RESTART IDENTITY")
	if from < 0 || to < from {
		t.Fatal("testutil/db.go has no TRUNCATE ... RESTART IDENTITY literal")
	}
	listed := map[string]bool{}
	for _, n := range strings.Split(s[from+len("TRUNCATE "):to], ",") {
		listed[strings.TrimSpace(n)] = true
	}

	for n := range made {
		if !listed[n] {
			t.Errorf("table %q is created by a migration but missing from testutil.Truncate", n)
		}
	}
	for n := range listed {
		if !made[n] {
			t.Errorf("table %q is in testutil.Truncate but no migration creates it", n)
		}
	}
}

// TestMigrationsUpDown applies the Phase 4 migration, rolls it back, and
// applies it again over a row written before it existed. It needs a database
// and skips without DATABASE_URL.
func TestMigrationsUpDown(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping migration up/down test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	name := phase4MigrationName(t)
	version, err := goose.NumericComponent(name)
	if err != nil {
		t.Fatalf("parsing version of %s: %v", name, err)
	}
	entries, err := MigrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("reading migrations: %v", err)
	}
	var previous int64
	for _, e := range entries {
		v, verr := goose.NumericComponent(e.Name())
		if verr == nil && v < version && v > previous {
			previous = v
		}
	}

	var reportID int64
	t.Cleanup(func() {
		if reportID != 0 {
			_, _ = db.Exec("DELETE FROM reports WHERE id = $1", reportID)
		}
		_ = goose.Up(db, "migrations")
	})

	columns := "{moderation_state,request_id,request_account_id,client_written_at}"
	tables := []string{"moderation_checks", "report_flags", "moderation_actions"}

	columnCount := func() int {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'reports' AND column_name = ANY($1::text[])`,
			columns).Scan(&n)
		if err != nil {
			t.Fatalf("counting columns: %v", err)
		}
		return n
	}
	tableCount := func() int {
		n := 0
		for _, tbl := range tables {
			var reg sql.NullString
			if err := db.QueryRow("SELECT to_regclass('public.' || $1)::text", tbl).Scan(&reg); err != nil {
				t.Fatalf("to_regclass %s: %v", tbl, err)
			}
			if reg.Valid {
				n++
			}
		}
		return n
	}
	indexCount := func() int {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM pg_indexes
			WHERE schemaname = 'public' AND indexname = 'idx_reports_request_idempotency'`).Scan(&n); err != nil {
			t.Fatalf("counting index: %v", err)
		}
		return n
	}

	if got := columnCount(); got != 4 {
		t.Errorf("after Up: %d of 4 new columns present", got)
	}
	if got := tableCount(); got != len(tables) {
		t.Errorf("after Up: %d of %d new tables present", got, len(tables))
	}
	if got := indexCount(); got != 1 {
		t.Errorf("after Up: idempotency index count %d, want 1", got)
	}

	if err := goose.DownTo(db, "migrations", previous); err != nil {
		t.Fatalf("goose down to %d: %v", previous, err)
	}
	if got := columnCount(); got != 0 {
		t.Errorf("after Down: %d new columns still present", got)
	}
	if got := tableCount(); got != 0 {
		t.Errorf("after Down: %d new tables still present", got)
	}
	if got := indexCount(); got != 0 {
		t.Errorf("after Down: idempotency index still present")
	}

	err = db.QueryRow(`INSERT INTO reports (session_id, category, severity, description, latitude, longitude, geohash, expires_at)
		VALUES ('migration-updown', 'road', 'info', 'before phase 4', 12.97, 77.59, 'tdr1w', now() + interval '1 hour')
		RETURNING id`).Scan(&reportID)
	if err != nil {
		t.Fatalf("inserting pre Phase 4 report: %v", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}

	var state string
	var reqID, reqAcct sql.NullString
	var written sql.NullTime
	err = db.QueryRow(`SELECT moderation_state, request_id, request_account_id::text, client_written_at
		FROM reports WHERE id = $1`, reportID).Scan(&state, &reqID, &reqAcct, &written)
	if err != nil {
		t.Fatalf("reading report after Up: %v", err)
	}
	if state != "clear" {
		t.Errorf("moderation_state = %q, want clear", state)
	}
	if reqID.Valid || reqAcct.Valid || written.Valid {
		t.Errorf("request_id, request_account_id and client_written_at must be NULL, got %v %v %v", reqID, reqAcct, written)
	}

	_, err = db.Exec("UPDATE reports SET moderation_state = 'held' WHERE id = $1", reportID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Errorf("setting moderation_state to held: want SQLSTATE 23514, got %v", err)
	}
}

// TestMigration00008 covers the Phase 3.4 migration (D-24b): the file carries
// both goose markers, is embedded, builds nothing concurrently, and on a
// database adds the account and reports columns with the CHECK constraints
// that refuse a malformed cell, an over long name, a name without a cell and
// an unknown What is happening value. Down removes everything again.
func TestMigration00008(t *testing.T) {
	const file = "migrations/00008_add_onboarding_and_happening.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(code, "\n")), "CONCURRENTLY") {
		t.Error("migration 00008 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	if strings.Count(text, "reports_happening_valid") < 2 {
		t.Error("migration 00008 must add and drop reports_happening_valid")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00008 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	var accountID, reportID int64
	t.Cleanup(func() {
		if reportID != 0 {
			_, _ = db.Exec("DELETE FROM reports WHERE id = $1", reportID)
		}
		if accountID != 0 {
			_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", accountID)
		}
		_ = goose.Up(db, "migrations")
	})

	accountColumns := "{terms_agreed_at,terms_version,first_run_done_at,display_name,home_area_name,home_area_cell}"
	count := func(table, cols string) int {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = ANY($2::text[])`,
			table, cols).Scan(&n)
		if err != nil {
			t.Fatalf("counting columns of %s: %v", table, err)
		}
		return n
	}
	if got := count("accounts", accountColumns); got != 6 {
		t.Errorf("after Up: %d of 6 new accounts columns present", got)
	}
	if got := count("reports", "{happening}"); got != 1 {
		t.Errorf("after Up: reports.happening count %d, want 1", got)
	}

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00008@example.com') RETURNING id`).Scan(&accountID); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO reports (session_id, category, severity, description, latitude, longitude, geohash, expires_at)
		VALUES ('migration-00008', 'road', 'info', 'for the 00008 test', 12.97, 77.59, 'tdr1w', now() + interval '1 hour')
		RETURNING id`).Scan(&reportID); err != nil {
		t.Fatalf("inserting report: %v", err)
	}

	wantCheck := func(label, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("%s: want SQLSTATE 23514, got %v", label, err)
		}
	}
	wantCheck("a 6 character cell",
		`UPDATE accounts SET home_area_name = 'Indiranagar', home_area_cell = 'tdr1wx' WHERE id = $1`, accountID)
	wantCheck("a cell with a letter outside the geohash alphabet",
		`UPDATE accounts SET home_area_name = 'Indiranagar', home_area_cell = 'tdr1a' WHERE id = $1`, accountID)
	wantCheck("a 61 character name",
		`UPDATE accounts SET display_name = $2 WHERE id = $1`, accountID, strings.Repeat("a", 61))
	wantCheck("an empty name",
		`UPDATE accounts SET display_name = '' WHERE id = $1`, accountID)
	wantCheck("a home area name without a cell",
		`UPDATE accounts SET home_area_name = 'Indiranagar' WHERE id = $1`, accountID)
	wantCheck("a cell without a home area name",
		`UPDATE accounts SET home_area_cell = 'tdr1w' WHERE id = $1`, accountID)
	wantCheck("a terms version without a time",
		`UPDATE accounts SET terms_version = '2026-10-16' WHERE id = $1`, accountID)
	wantCheck("an unknown happening value",
		`UPDATE reports SET happening = 'zombies' WHERE id = $1`, reportID)

	if _, err := db.Exec(`UPDATE accounts SET home_area_name = 'Indiranagar', home_area_cell = 'tdr1w',
		display_name = 'Asha', terms_version = '2026-10-16', terms_agreed_at = now() WHERE id = $1`, accountID); err != nil {
		t.Errorf("a valid profile must be accepted: %v", err)
	}
	if _, err := db.Exec(`UPDATE reports SET happening = 'water_rising' WHERE id = $1`, reportID); err != nil {
		t.Errorf("a valid happening value must be accepted: %v", err)
	}

	name := "00008_add_onboarding_and_happening.sql"
	version, err := goose.NumericComponent(name)
	if err != nil {
		t.Fatalf("parsing version of %s: %v", name, err)
	}
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if got := count("accounts", accountColumns); got != 0 {
		t.Errorf("after Down: %d accounts columns still present", got)
	}
	if got := count("reports", "{happening}"); got != 0 {
		t.Errorf("after Down: reports.happening still present")
	}
}

// TestMigration00009 covers the Phase 3.2 migration: the file carries both
// goose markers, is embedded, builds nothing concurrently, and on a database
// adds the four tables and the two lifecycle columns with CHECK constraints
// that refuse an unknown cursor state, a malformed rolled up row and an
// unlisted alert radius. Down removes everything again and Up applies again.
func TestMigration00009(t *testing.T) {
	const file = "migrations/00009_add_updates_and_alerts.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(code, "\n")), "CONCURRENTLY") {
		t.Error("migration 00009 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	for _, name := range []string{"updates_kind_valid", "updates_more_shape", "update_cursors_state_valid", "account_alert_settings_radius_valid"} {
		if !strings.Contains(text[:down], name) {
			t.Errorf("migration 00009 Up must add %s", name)
		}
	}
	for _, table := range []string{"updates", "update_cursors", "report_mutes", "account_alert_settings"} {
		if !strings.Contains(text[down:], "DROP TABLE "+table+";") {
			t.Errorf("migration 00009 Down must drop table %s", table)
		}
	}
	for _, col := range []string{"revoked_at", "deleted_at"} {
		if !strings.Contains(text[down:], "DROP COLUMN "+col) {
			t.Errorf("migration 00009 Down must drop column %s", col)
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00009 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	var accountID, reportID int64
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		if accountID != 0 {
			_, _ = db.Exec("DELETE FROM account_alert_settings WHERE account_id = $1", accountID)
			_, _ = db.Exec("DELETE FROM update_cursors WHERE account_id = $1", accountID)
			_, _ = db.Exec("DELETE FROM updates WHERE account_id = $1", accountID)
		}
		if reportID != 0 {
			_, _ = db.Exec("DELETE FROM reports WHERE id = $1", reportID)
		}
		if accountID != 0 {
			_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", accountID)
		}
	})

	tableCount := func() int {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name IN ('updates', 'update_cursors', 'report_mutes', 'account_alert_settings')`).Scan(&n)
		if err != nil {
			t.Fatalf("counting tables: %v", err)
		}
		return n
	}
	columnCount := func() int {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND ((table_name = 'sessions' AND column_name = 'revoked_at')
			    OR (table_name = 'accounts' AND column_name = 'deleted_at'))`).Scan(&n)
		if err != nil {
			t.Fatalf("counting columns: %v", err)
		}
		return n
	}
	if got := tableCount(); got != 4 {
		t.Errorf("after Up: %d of 4 new tables present", got)
	}
	if got := columnCount(); got != 2 {
		t.Errorf("after Up: %d of 2 new columns present", got)
	}

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00009@example.com') RETURNING id`).Scan(&accountID); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO reports (session_id, category, severity, description, latitude, longitude, geohash, expires_at)
		VALUES ('migration-00009', 'road', 'info', 'for the 00009 test', 12.97, 77.59, 'tdr1w', now() + interval '1 hour')
		RETURNING id`).Scan(&reportID); err != nil {
		t.Fatalf("inserting report: %v", err)
	}

	wantCheck := func(label, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("%s: want SQLSTATE 23514, got %v", label, err)
		}
	}
	wantCheck("a cursor state of withheld",
		`INSERT INTO update_cursors (account_id, report_id, last_state, adopted_at) VALUES ($1, $2, 'withheld', now())`,
		accountID, reportID)
	wantCheck("a more row that names a report",
		`INSERT INTO updates (account_id, report_id, kind, bucket_start, more_count) VALUES ($1, $2, 'more', now(), 1)`,
		accountID, reportID)
	wantCheck("a more row with a zero count",
		`INSERT INTO updates (account_id, kind, bucket_start, more_count) VALUES ($1, 'more', now(), 0)`,
		accountID)
	wantCheck("a normal row without a report",
		`INSERT INTO updates (account_id, kind) VALUES ($1, 'hidden')`, accountID)
	wantCheck("an unknown kind",
		`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'shouted')`, accountID, reportID)
	wantCheck("a radius of 3 km",
		`INSERT INTO account_alert_settings (account_id, alert_radius_km) VALUES ($1, 3)`, accountID)

	if _, err := db.Exec(`INSERT INTO account_alert_settings (account_id) VALUES ($1)`, accountID); err != nil {
		t.Fatalf("inserting default alert settings: %v", err)
	}
	var on bool
	var radius int
	if err := db.QueryRow(`SELECT reliable_alerts_on, alert_radius_km FROM account_alert_settings WHERE account_id = $1`,
		accountID).Scan(&on, &radius); err != nil {
		t.Fatalf("reading alert settings: %v", err)
	}
	if on || radius != 5 {
		t.Errorf("defaults = on %v radius %d, want false and 5", on, radius)
	}

	name := "00009_add_updates_and_alerts.sql"
	version, err := goose.NumericComponent(name)
	if err != nil {
		t.Fatalf("parsing version of %s: %v", name, err)
	}
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if got := tableCount(); got != 0 {
		t.Errorf("after Down: %d new tables still present", got)
	}
	if got := columnCount(); got != 0 {
		t.Errorf("after Down: %d new columns still present", got)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if got := tableCount(); got != 4 {
		t.Errorf("after Up again: %d of 4 new tables present", got)
	}
}

// TestMigration00010 covers the Flag window migration of plan 04-31: the
// file carries both goose markers and builds nothing concurrently; Up turns
// every existing flag's single reason into a one element set and drops the
// old column; Down brings the single reason back as the first of the set and
// drops the set, the note and the helper function; Up again restores both
// columns with every flag keeping its reason.
func TestMigration00010(t *testing.T) {
	const name = "00010_add_flag_reasons_and_note.sql"
	contents, err := MigrationsFS.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", name, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", name)
	}
	if strings.Contains(strings.ToUpper(text), "CONCURRENTLY") {
		t.Error("migration 00010 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	for _, want := range []string{"report_flags_reasons_valid", "report_flags_note_length", "UPDATE report_flags SET reasons = ARRAY[reason]", "DROP COLUMN reason;"} {
		if !strings.Contains(text[:down], want) {
			t.Errorf("migration 00010 Up must contain %q", want)
		}
	}
	for _, want := range []string{"UPDATE report_flags SET reason = reasons[1]", "DROP COLUMN reasons;", "DROP COLUMN note;", "DROP FUNCTION report_flag_reasons_distinct"} {
		if !strings.Contains(text[down:], want) {
			t.Errorf("migration 00010 Down must contain %q", want)
		}
	}
	if !strings.Contains(text[:down], "report_flags_one_per_account") || strings.Contains(text, "DROP CONSTRAINT report_flags_one_per_account") {
		t.Error("migration 00010 must keep the one flag per account per report constraint")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00010 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	version, err := goose.NumericComponent(name)
	if err != nil {
		t.Fatalf("parsing version of %s: %v", name, err)
	}

	var a1, a2, reportID int64
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		if reportID != 0 {
			_, _ = db.Exec("DELETE FROM report_flags WHERE report_id = $1", reportID)
			_, _ = db.Exec("DELETE FROM reports WHERE id = $1", reportID)
		}
		for _, id := range []int64{a1, a2} {
			if id != 0 {
				_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", id)
			}
		}
	})

	columns := func() string {
		rows, err := db.Query(`SELECT column_name FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'report_flags'
			  AND column_name IN ('reason', 'reasons', 'note') ORDER BY column_name`)
		if err != nil {
			t.Fatalf("listing columns: %v", err)
		}
		defer rows.Close()
		var names []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				t.Fatalf("scanning column: %v", err)
			}
			names = append(names, n)
		}
		return strings.Join(names, ",")
	}
	if got := columns(); got != "note,reasons" {
		t.Fatalf("after Up: columns %q, want note,reasons", got)
	}

	// Down to just before 00010, store two flags the old way, then Up.
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if got := columns(); got != "reason" {
		t.Fatalf("after Down: columns %q, want reason", got)
	}
	var fn sql.NullString
	if err := db.QueryRow(`SELECT to_regproc('report_flag_reasons_distinct')::text`).Scan(&fn); err != nil || fn.Valid {
		t.Errorf("after Down: the helper function is still there (%v, %v)", fn, err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00010-a@example.com') RETURNING id`).Scan(&a1); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00010-b@example.com') RETURNING id`).Scan(&a2); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO reports (session_id, category, severity, description, latitude, longitude, geohash, expires_at)
		VALUES ('migration-00010', 'road', 'info', 'for the 00010 test', 12.97, 77.59, 'tdr1w', now() + interval '1 hour')
		RETURNING id`).Scan(&reportID); err != nil {
		t.Fatalf("inserting report: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO report_flags (report_id, account_id, reason) VALUES ($1, $2, 'false'), ($1, $3, 'dangerous')`,
		reportID, a1, a2); err != nil {
		t.Fatalf("inserting old style flags: %v", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if got := columns(); got != "note,reasons" {
		t.Fatalf("after Up again: columns %q, want note,reasons", got)
	}
	readSets := func() string {
		var s string
		if err := db.QueryRow(`SELECT string_agg(array_to_string(reasons, '+') || ':' || COALESCE(note, '-'), ' ' ORDER BY account_id)
			FROM report_flags WHERE report_id = $1`, reportID).Scan(&s); err != nil {
			t.Fatalf("reading sets: %v", err)
		}
		return s
	}
	if got := readSets(); got != "false:- dangerous:-" {
		t.Errorf("after Up: flags %q, want each old reason as a one element set with no note", got)
	}

	// A flag with two reasons and a note goes back to its first reason.
	if _, err := db.Exec(`UPDATE report_flags SET reasons = ARRAY['spam','false'], note = 'old photo' WHERE report_id = $1 AND account_id = $2`,
		reportID, a1); err != nil {
		t.Fatalf("updating a flag: %v", err)
	}
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down again: %v", err)
	}
	var back string
	if err := db.QueryRow(`SELECT string_agg(reason, ' ' ORDER BY account_id) FROM report_flags WHERE report_id = $1`, reportID).Scan(&back); err != nil {
		t.Fatalf("reading reasons after Down: %v", err)
	}
	if back != "spam dangerous" {
		t.Errorf("after Down: reasons %q, want spam dangerous", back)
	}
	_, err = db.Exec(`UPDATE report_flags SET reason = 'police' WHERE report_id = $1`, reportID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Errorf("after Down: reason police: want SQLSTATE 23514, got %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up a third time: %v", err)
	}
	if got := readSets(); got != "spam:- dangerous:-" {
		t.Errorf("after the round trip: flags %q, want spam:- dangerous:-", got)
	}
}

// TestMigration00011 covers the Phase 3.3 migration: the file carries both
// goose markers, builds nothing concurrently and names every constraint where
// it adds it; on a database it adds the three push tables and the four new
// columns, the CHECK lists and partial unique indexes refuse what they must,
// an alert settings row with only an account id reads Critical on and Changes
// off, Down removes everything again and Up applies again.
func TestMigration00011(t *testing.T) {
	const file = "migrations/00011_add_push.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(code, "\n")), "CONCURRENTLY") {
		t.Error("migration 00011 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	for _, name := range []string{
		"push_subscriptions_endpoint_shape", "push_subscriptions_key_len",
		"push_outbox_kind_valid", "push_outbox_state_valid", "push_outbox_skip_reason_valid",
		"push_outbox_update_shape", "push_report_progress_state_valid", "reports_area_name_len",
		"uq_push_outbox_first", "uq_push_outbox_confirmed", "uq_push_outbox_changed",
		"idx_push_outbox_due", "idx_push_outbox_account_sent", "idx_push_subscriptions_account",
		"idx_push_subscriptions_session", "idx_accounts_home_area_cell",
	} {
		if !strings.Contains(text[:down], name) {
			t.Errorf("migration 00011 Up must add %s", name)
		}
	}
	for _, table := range []string{"push_subscriptions", "push_outbox", "push_report_progress"} {
		if !strings.Contains(text[down:], "DROP TABLE "+table+";") {
			t.Errorf("migration 00011 Down must drop table %s", table)
		}
	}
	for _, col := range []string{"area_name", "critical_push_on", "changes_push_on", "changes_enabled_at"} {
		if !strings.Contains(text[down:], "DROP COLUMN "+col) {
			t.Errorf("migration 00011 Down must drop column %s", col)
		}
	}
	if !strings.Contains(text[down:], "DROP INDEX idx_accounts_home_area_cell") {
		t.Error("migration 00011 Down must drop idx_accounts_home_area_cell")
	}
	if strings.Contains(strings.Join(code, "\n"), "'more'") {
		t.Error("migration 00011 must not list a rolled up kind (decision 6)")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00011 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	var accountID, otherID, reportID int64
	const sessionID = "migration-00011-session"
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		_, _ = db.Exec("DELETE FROM push_outbox WHERE report_id = $1", reportID)
		_, _ = db.Exec("DELETE FROM push_subscriptions WHERE session_id = $1", sessionID)
		for _, id := range []int64{accountID, otherID} {
			if id != 0 {
				_, _ = db.Exec("DELETE FROM account_alert_settings WHERE account_id = $1", id)
			}
		}
		if reportID != 0 {
			_, _ = db.Exec("DELETE FROM reports WHERE id = $1", reportID)
		}
		_, _ = db.Exec("DELETE FROM sessions WHERE session_id = $1", sessionID)
		for _, id := range []int64{accountID, otherID} {
			if id != 0 {
				_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", id)
			}
		}
	})

	tableCount := func() int {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name IN ('push_subscriptions', 'push_outbox', 'push_report_progress')`).Scan(&n)
		if err != nil {
			t.Fatalf("counting tables: %v", err)
		}
		return n
	}
	columnCount := func() int {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND ((table_name = 'reports' AND column_name = 'area_name')
			    OR (table_name = 'account_alert_settings'
			        AND column_name IN ('critical_push_on', 'changes_push_on', 'changes_enabled_at')))`).Scan(&n)
		if err != nil {
			t.Fatalf("counting columns: %v", err)
		}
		return n
	}
	if got := tableCount(); got != 3 {
		t.Errorf("after Up: %d of 3 new tables present", got)
	}
	if got := columnCount(); got != 4 {
		t.Errorf("after Up: %d of 4 new columns present", got)
	}
	var idx int
	if err := db.QueryRow(`SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_accounts_home_area_cell'`).Scan(&idx); err != nil || idx != 1 {
		t.Errorf("after Up: home cell index count %d (%v), want 1", idx, err)
	}

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00011-a@example.com') RETURNING id`).Scan(&accountID); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00011-b@example.com') RETURNING id`).Scan(&otherID); err != nil {
		t.Fatalf("inserting second account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (session_id, account_id) VALUES ($1, $2)`, sessionID, accountID); err != nil {
		t.Fatalf("inserting session: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO reports (session_id, category, severity, description, latitude, longitude, geohash, expires_at)
		VALUES ($1, 'road', 'info', 'for the 00011 test', 12.97, 77.59, 'tdr1w', now() + interval '1 hour')
		RETURNING id`, sessionID).Scan(&reportID); err != nil {
		t.Fatalf("inserting report: %v", err)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}
	wantCode("an outbox kind of more", "23514",
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'more', now() + interval '1 hour')`,
		accountID, reportID)
	wantCode("a changed row without an update id", "23514",
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'changed', now() + interval '1 hour')`,
		accountID, reportID)
	wantCode("a critical row with an update id", "23514",
		`INSERT INTO push_outbox (account_id, report_id, kind, update_id, expires_at) VALUES ($1, $2, 'critical', 7, now() + interval '1 hour')`,
		accountID, reportID)
	wantCode("an unknown outbox state", "23514",
		`INSERT INTO push_outbox (account_id, report_id, kind, state, expires_at) VALUES ($1, $2, 'critical', 'sending', now() + interval '1 hour')`,
		accountID, reportID)
	wantCode("an unknown skip reason", "23514",
		`INSERT INTO push_outbox (account_id, report_id, kind, state, skip_reason, expires_at) VALUES ($1, $2, 'critical', 'skipped', 'because', now() + interval '1 hour')`,
		accountID, reportID)
	wantCode("an http endpoint", "23514",
		`INSERT INTO push_subscriptions (account_id, session_id, endpoint, endpoint_host, p256dh, auth) VALUES ($1, $2, 'http://fcm.googleapis.com/x', 'fcm.googleapis.com', 'k', 'a')`,
		accountID, sessionID)
	wantCode("an unknown progress state", "23514",
		`INSERT INTO push_report_progress (report_id, critical_state) VALUES ($1, 'sent')`, reportID)
	wantCode("an empty area name", "23514", `UPDATE reports SET area_name = '' WHERE id = $1`, reportID)

	if _, err := db.Exec(`INSERT INTO push_subscriptions (account_id, session_id, endpoint, endpoint_host, p256dh, auth)
		VALUES ($1, $2, 'https://fcm.googleapis.com/fcm/send/abc', 'fcm.googleapis.com', 'k', 'a')`, accountID, sessionID); err != nil {
		t.Fatalf("inserting a subscription: %v", err)
	}
	wantCode("a second subscription with the same endpoint", "23505",
		`INSERT INTO push_subscriptions (account_id, session_id, endpoint, endpoint_host, p256dh, auth) VALUES ($1, $2, 'https://fcm.googleapis.com/fcm/send/abc', 'fcm.googleapis.com', 'k', 'a')`,
		otherID, sessionID)

	if _, err := db.Exec(`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'critical', now() + interval '1 hour')`,
		accountID, reportID); err != nil {
		t.Fatalf("inserting a critical row: %v", err)
	}
	wantCode("a second critical row for one account and report", "23505",
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'critical', now() + interval '1 hour')`,
		accountID, reportID)
	wantCode("a reliable row beside the critical row", "23505",
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'reliable', now() + interval '1 hour')`,
		accountID, reportID)

	if _, err := db.Exec(`INSERT INTO account_alert_settings (account_id) VALUES ($1)`, accountID); err != nil {
		t.Fatalf("inserting default alert settings: %v", err)
	}
	var critical, changes bool
	var enabledAt sql.NullTime
	if err := db.QueryRow(`SELECT critical_push_on, changes_push_on, changes_enabled_at FROM account_alert_settings WHERE account_id = $1`,
		accountID).Scan(&critical, &changes, &enabledAt); err != nil {
		t.Fatalf("reading alert settings: %v", err)
	}
	if !critical || changes || enabledAt.Valid {
		t.Errorf("defaults = critical %v changes %v enabled %v, want true, false, NULL", critical, changes, enabledAt.Valid)
	}

	name := "00011_add_push.sql"
	version, err := goose.NumericComponent(name)
	if err != nil {
		t.Fatalf("parsing version of %s: %v", name, err)
	}
	// Down drops the push tables whole, so their rows need no clean up first.
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if got := tableCount(); got != 0 {
		t.Errorf("after Down: %d new tables still present", got)
	}
	if got := columnCount(); got != 0 {
		t.Errorf("after Down: %d new columns still present", got)
	}
	if err := db.QueryRow(`SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_accounts_home_area_cell'`).Scan(&idx); err != nil || idx != 0 {
		t.Errorf("after Down: home cell index count %d (%v), want 0", idx, err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if got := tableCount(); got != 3 {
		t.Errorf("after Up again: %d of 3 new tables present", got)
	}
	if got := columnCount(); got != 4 {
		t.Errorf("after Up again: %d of 4 new columns present", got)
	}
}

// TestMigration00012 covers the Phase 5 migration: the file carries both goose
// markers, builds nothing concurrently, uses no ON DELETE and names every
// constraint where it adds it; on a database it adds the four official tables
// with both feed rows, the CHECK lists and the unique rules refuse what they
// must, Down removes everything and Up applies again.
func TestMigration00012(t *testing.T) {
	const file = "migrations/00012_add_official_alerts.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	upCode := strings.ToUpper(strings.Join(code, "\n"))
	if strings.Contains(upCode, "CONCURRENTLY") {
		t.Error("migration 00012 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	if strings.Contains(upCode, "ON DELETE") {
		t.Error("migration 00012 must not use ON DELETE: the repo convention has none")
	}
	for _, name := range []string{
		"official_feed_status_feed_valid", "official_feed_status_error_valid", "official_feed_status_counts",
		"official_alerts_guid_unique", "official_alerts_msg_type_valid", "official_alerts_severity_valid",
		"official_alerts_placement_valid", "idx_official_alerts_expires_at",
		"official_alert_areas_code_present", "uq_official_alert_areas",
		"idx_official_alert_areas_district", "idx_official_alert_areas_state",
		"official_hazards_event_unique", "official_hazards_position_valid",
	} {
		if !strings.Contains(text[:down], name) {
			t.Errorf("migration 00012 Up must add %s", name)
		}
	}
	for _, feed := range []string{"'sachet'", "'gdacs'"} {
		if !strings.Contains(text[up:down], "INSERT INTO official_feed_status") || !strings.Contains(text[up:down], feed) {
			t.Errorf("migration 00012 Up must insert the %s feed row", feed)
		}
	}
	// Down drops the four tables in reverse order of their creation.
	last := -1
	for _, table := range []string{"official_hazards", "official_alert_areas", "official_alerts", "official_feed_status"} {
		at := strings.Index(text[down:], "DROP TABLE "+table+";")
		if at < 0 {
			t.Errorf("migration 00012 Down must drop table %s", table)
			continue
		}
		if at < last {
			t.Errorf("migration 00012 Down drops %s out of order", table)
		}
		last = at
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00012 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	const guid = "1792000000000012"
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		_, _ = db.Exec(`DELETE FROM official_alert_areas WHERE alert_id IN (SELECT id FROM official_alerts WHERE guid = $1)`, guid)
		_, _ = db.Exec(`DELETE FROM official_alerts WHERE guid = $1`, guid)
		_, _ = db.Exec(`DELETE FROM official_hazards WHERE event_id = 9000012`)
		_, _ = db.Exec(`UPDATE official_feed_status SET last_error = NULL, consecutive_failures = 0`)
	})

	tableCount := func() int {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name IN ('official_alerts', 'official_alert_areas', 'official_feed_status', 'official_hazards')`).Scan(&n)
		if err != nil {
			t.Fatalf("counting tables: %v", err)
		}
		return n
	}
	feedRows := func() int {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM official_feed_status WHERE feed IN ('sachet', 'gdacs')`).Scan(&n); err != nil {
			t.Fatalf("counting feed rows: %v", err)
		}
		return n
	}
	if got := tableCount(); got != 4 {
		t.Errorf("after Up: %d of 4 new tables present", got)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}
	const insertAlert = `INSERT INTO official_alerts (guid, cap_identifier, msg_type, severity, sent_at, expires_at, placement_tier, first_seen_at, last_seen_at)
		VALUES ($1, 'IN-x_1', $2, $3, now(), now() + interval '1 hour', $4, now(), now())`
	wantCode("a bad severity", "23514", insertAlert, guid, "Alert", "Catastrophic", "unplaced")
	wantCode("a bad msg_type", "23514", insertAlert, guid, "Ack", "Severe", "unplaced")
	wantCode("a bad placement_tier", "23514", insertAlert, guid, "Alert", "Severe", "polygon")
	wantCode("a bad feed name", "23514", `INSERT INTO official_feed_status (feed) VALUES ('nws')`)
	wantCode("a bad last_error", "23514", `UPDATE official_feed_status SET last_error = 'dns failure' WHERE feed = 'sachet'`)
	wantCode("a negative failure count", "23514", `UPDATE official_feed_status SET consecutive_failures = -1 WHERE feed = 'sachet'`)

	var alertID int64
	if err := db.QueryRow(insertAlert+` RETURNING id`, guid, "Alert", "Severe", "district").Scan(&alertID); err != nil {
		t.Fatalf("inserting an alert: %v", err)
	}
	wantCode("a second alert with the same guid", "23505", insertAlert, guid, "Alert", "Severe", "district")
	wantCode("an area with both codes null", "23514",
		`INSERT INTO official_alert_areas (alert_id) VALUES ($1)`, alertID)
	if _, err := db.Exec(`INSERT INTO official_alert_areas (alert_id, district_lgd, state_lgd) VALUES ($1, 568, 33)`, alertID); err != nil {
		t.Fatalf("inserting an area: %v", err)
	}
	wantCode("the same area twice", "23505",
		`INSERT INTO official_alert_areas (alert_id, district_lgd, state_lgd) VALUES ($1, 568, 33)`, alertID)
	wantCode("an area for a missing alert", "23503",
		`INSERT INTO official_alert_areas (alert_id, district_lgd) VALUES (0, 568)`)
	wantCode("a hazard off the globe", "23514",
		`INSERT INTO official_hazards (event_type, event_id, latitude, longitude, first_seen_at, last_seen_at) VALUES ('FL', 9000012, 91, 0, now(), now())`)
	if _, err := db.Exec(`INSERT INTO official_hazards (event_type, event_id, latitude, longitude, first_seen_at, last_seen_at) VALUES ('FL', 9000012, 20.5, 85.8, now(), now())`); err != nil {
		t.Fatalf("inserting a hazard: %v", err)
	}
	wantCode("the same hazard twice", "23505",
		`INSERT INTO official_hazards (event_type, event_id, latitude, longitude, first_seen_at, last_seen_at) VALUES ('FL', 9000012, 20.5, 85.8, now(), now())`)

	if got := feedRows(); got != 2 {
		t.Errorf("after Up: %d of 2 feed rows present", got)
	}

	name := "00012_add_official_alerts.sql"
	version, err := goose.NumericComponent(name)
	if err != nil {
		t.Fatalf("parsing version of %s: %v", name, err)
	}
	// Down drops the tables whole, so their rows need no clean up first.
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if got := tableCount(); got != 0 {
		t.Errorf("after Down: %d new tables still present", got)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if got := tableCount(); got != 4 {
		t.Errorf("after Up again: %d of 4 new tables present", got)
	}
	if got := feedRows(); got != 2 {
		t.Errorf("after Up again: %d of 2 feed rows present", got)
	}
}

// TestMigration00013 covers the Phase 5 push migration (plan 05-11): the file
// carries both goose markers, builds nothing concurrently, uses no ON DELETE and
// names every constraint where it adds it; on a database it adds the official
// push outbox and the official_push_on column, the CHECK lists and the unique
// index refuse what they must (skip_reason already_notified is accepted), an
// account with no settings row reads the switch as on, Down removes both and Up
// applies again.
func TestMigration00013(t *testing.T) {
	const file = "migrations/00013_add_official_push.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	allCode := strings.ToUpper(strings.Join(code, "\n"))
	if strings.Contains(allCode, "CONCURRENTLY") {
		t.Error("migration 00013 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	if strings.Contains(allCode, "ON DELETE") {
		t.Error("migration 00013 must not use ON DELETE: the repo convention has none")
	}
	for _, want := range []string{
		"CREATE TABLE official_push_outbox",
		"official_push_on BOOLEAN NOT NULL DEFAULT true",
		"official_push_outbox_reason_valid", "official_push_outbox_state_valid",
		"official_push_outbox_skip_reason_valid", "uq_official_push_once",
		"idx_official_push_outbox_due",
		"'already_notified'",
	} {
		if !strings.Contains(text[up:down], want) {
			t.Errorf("migration 00013 Up must contain %q", want)
		}
	}
	// Down drops the table before the column, the reverse of the Up order.
	dropTable := strings.Index(text[down:], "DROP TABLE official_push_outbox;")
	dropColumn := strings.Index(text[down:], "DROP COLUMN official_push_on")
	if dropTable < 0 || dropColumn < 0 || dropTable > dropColumn {
		t.Error("migration 00013 Down must drop official_push_outbox, then the official_push_on column")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00013 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	const guid = "1792000000000013"
	const email = "migration-00013@example.com"
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		_, _ = db.Exec(`DELETE FROM official_push_outbox WHERE alert_id IN (SELECT id FROM official_alerts WHERE guid = $1)`, guid)
		_, _ = db.Exec(`DELETE FROM official_alerts WHERE guid = $1`, guid)
		_, _ = db.Exec(`DELETE FROM account_alert_settings WHERE account_id IN (SELECT id FROM accounts WHERE email = $1)`, email)
		_, _ = db.Exec(`DELETE FROM accounts WHERE email = $1`, email)
	})

	objects := func() (tables, columns int) {
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'official_push_outbox'`).Scan(&tables); err != nil {
			t.Fatalf("counting tables: %v", err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'account_alert_settings' AND column_name = 'official_push_on'`).Scan(&columns); err != nil {
			t.Fatalf("counting columns: %v", err)
		}
		return tables, columns
	}
	if tables, columns := objects(); tables != 1 || columns != 1 {
		t.Errorf("after Up: %d table and %d column present, want 1 and 1", tables, columns)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}

	var accountID, alertID int64
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ($1) RETURNING id`, email).Scan(&accountID); err != nil {
		t.Fatalf("inserting an account: %v", err)
	}
	// An account with no settings row reads the switch as on (D-11).
	var on bool
	if err := db.QueryRow(`SELECT COALESCE(s.official_push_on, true)
		FROM accounts a LEFT JOIN account_alert_settings s ON s.account_id = a.id WHERE a.id = $1`, accountID).Scan(&on); err != nil || !on {
		t.Errorf("a missing settings row reads %v (%v), want true", on, err)
	}
	if _, err := db.Exec(`INSERT INTO account_alert_settings (account_id) VALUES ($1)`, accountID); err != nil {
		t.Fatalf("inserting a settings row: %v", err)
	}
	if err := db.QueryRow(`SELECT official_push_on FROM account_alert_settings WHERE account_id = $1`, accountID).Scan(&on); err != nil || !on {
		t.Errorf("a new settings row has official_push_on %v (%v), want the default true", on, err)
	}
	if err := db.QueryRow(`INSERT INTO official_alerts (guid, cap_identifier, msg_type, severity, sent_at, expires_at, placement_tier, first_seen_at, last_seen_at)
		VALUES ($1, 'IN-x_1', 'Alert', 'Severe', now(), now() + interval '1 hour', 'district', now(), now()) RETURNING id`, guid).Scan(&alertID); err != nil {
		t.Fatalf("inserting an alert: %v", err)
	}

	const insert = `INSERT INTO official_push_outbox (account_id, alert_id, cap_identifier, reason, state, skip_reason, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, now() + interval '1 hour')`
	wantCode("a bad reason", "23514", insert, accountID, alertID, "IN-x_bad1", "stacked", "queued", nil)
	wantCode("a bad state", "23514", insert, accountID, alertID, "IN-x_bad2", "new", "pending", nil)
	wantCode("a bad skip_reason", "23514", insert, accountID, alertID, "IN-x_bad3", "new", "skipped", "because")
	wantCode("an alert that does not exist", "23503", insert, accountID, 0, "IN-x_bad4", "new", "queued", nil)
	for _, reason := range []string{"not_eligible", "stale", "no_home_area", "out_of_area", "no_device",
		"cap_account", "switched_off", "feed_not_fresh", "already_notified"} {
		if _, err := db.Exec(insert, accountID, alertID, "IN-x_"+reason, "new", "skipped", reason); err != nil {
			t.Errorf("skip_reason %q must be accepted: %v", reason, err)
		}
	}
	wantCode("the same person, alert and version twice", "23505", insert, accountID, alertID, "IN-x_already_notified", "new", "queued", nil)
	if _, err := db.Exec(insert, accountID, alertID, "IN-x_2", "escalated", "queued", nil); err != nil {
		t.Errorf("a second version of the same alert for the same person must be accepted: %v", err)
	}

	name := "00013_add_official_push.sql"
	version, err := goose.NumericComponent(name)
	if err != nil {
		t.Fatalf("parsing version of %s: %v", name, err)
	}
	// Down drops the table whole, so its rows need no clean up first.
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if tables, columns := objects(); tables != 0 || columns != 0 {
		t.Errorf("after Down: %d table and %d column still present", tables, columns)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if tables, columns := objects(); tables != 1 || columns != 1 {
		t.Errorf("after Up again: %d table and %d column present, want 1 and 1", tables, columns)
	}
}

// TestMigration00014 covers the Phase 7.1 migration: the file carries both goose
// markers, builds nothing concurrently, uses no ON DELETE and no citext, names
// every constraint and index of the phase, and on a database the CHECK
// constraints refuse a bad username shape, a duplicate username, an unknown
// code purpose, a reset row carrying a password and a password hash that is not
// an argon2id string. Down removes both tables and every added column and Up
// applies again.
func TestMigration00014(t *testing.T) {
	const file = "migrations/00014_add_passwords_and_usernames.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	allCode := strings.ToUpper(strings.Join(code, "\n"))
	if strings.Contains(allCode, "CONCURRENTLY") {
		t.Error("migration 00014 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	if strings.Contains(allCode, "ON DELETE") {
		t.Error("migration 00014 must not use ON DELETE: the repo convention has none")
	}
	if strings.Contains(allCode, "CITEXT") {
		t.Error("migration 00014 must not use citext: the stored username is already canonical")
	}
	for _, want := range []string{
		"CREATE TABLE auth_codes", "CREATE TABLE auth_throttles",
		"accounts_password_hash_shape", "accounts_username_shape", "accounts_username_unique",
		"accounts_username_screen_valid", "username_changed_at",
		"auth_codes_purpose_valid", "auth_codes_delivery_valid", "auth_codes_code_hash_shape",
		"auth_codes_password_hash_shape", "auth_codes_attempts_valid",
		"idx_auth_codes_session_live", "idx_auth_codes_email",
		"auth_throttles_scope_valid", "auth_throttles_key_hash_shape", "auth_throttles_failures_valid",
		"window_started_at", "window_sends", "email_cooldowns_window_sends_valid",
		"IN ('code', 'account_exists', 'no_account')",
		"UPDATE sessions SET revoked_at",
	} {
		if !strings.Contains(text[up:down], want) {
			t.Errorf("migration 00014 Up must contain %q", want)
		}
	}
	downText := text[down:]
	for _, want := range []string{
		"DROP TABLE auth_throttles;", "DROP TABLE auth_codes;",
		"DROP COLUMN window_sends", "DROP COLUMN window_started_at",
		"DROP COLUMN password_hash", "DROP COLUMN username_changed_at",
		"DROP COLUMN username_screen", "DROP COLUMN username,",
		"cannot be undone",
	} {
		if !strings.Contains(downText, want) {
			t.Errorf("migration 00014 Down must contain %q", want)
		}
	}
	// The data step is the last statement of Up.
	if strings.LastIndex(text[up:down], "UPDATE sessions SET revoked_at") < strings.LastIndex(text[up:down], "CREATE TABLE") {
		t.Error("migration 00014 must end Up with the session revoke data step")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00014 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		_, _ = db.Exec(`DELETE FROM auth_codes WHERE session_id = 'migration-00014'`)
		_, _ = db.Exec(`DELETE FROM accounts WHERE email LIKE 'migration-00014-%@example.com'`)
		_ = db.Close()
	})

	objects := func() (tables, columns int) {
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name IN ('auth_codes', 'auth_throttles')`).Scan(&tables); err != nil {
			t.Fatalf("counting tables: %v", err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND (
				(table_name = 'accounts' AND column_name IN ('password_hash', 'username', 'username_screen', 'username_changed_at'))
				OR (table_name = 'email_cooldowns' AND column_name IN ('window_started_at', 'window_sends')))`).Scan(&columns); err != nil {
			t.Fatalf("counting columns: %v", err)
		}
		return tables, columns
	}
	if tables, columns := objects(); tables != 2 || columns != 6 {
		t.Errorf("after Up: %d tables and %d columns present, want 2 and 6", tables, columns)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}

	var first, second int64
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00014-a@example.com') RETURNING id`).Scan(&first); err != nil {
		t.Fatalf("inserting an account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00014-b@example.com') RETURNING id`).Scan(&second); err != nil {
		t.Fatalf("inserting a second account: %v", err)
	}

	const setUsername = `UPDATE accounts SET username = $2, username_screen = 'clear' WHERE id = $1`
	for _, bad := range []string{"Asha", ".asha", "asha.", "as..ha", "123", "ab", "asha-k", "a2345678901234567890x"} {
		wantCode("username "+bad, "23514", setUsername, first, bad)
	}
	if _, err := db.Exec(setUsername, first, "asha_k.1"); err != nil {
		t.Errorf("username asha_k.1 must be accepted: %v", err)
	}
	wantCode("the same username on a second account", "23505", setUsername, second, "asha_k.1")
	wantCode("a username with no screen state", "23514", `UPDATE accounts SET username_screen = NULL WHERE id = $1`, first)
	wantCode("a screen state with no username", "23514", `UPDATE accounts SET username_screen = 'clear' WHERE id = $1`, second)
	wantCode("a password that is not argon2id", "23514", `UPDATE accounts SET password_hash = 'plain' WHERE id = $1`, first)
	if _, err := db.Exec(`UPDATE accounts SET password_hash = '$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA' WHERE id = $1`, first); err != nil {
		t.Errorf("an argon2id string must be accepted: %v", err)
	}

	const hash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	const insertCode = `INSERT INTO auth_codes (session_id, purpose, delivery, email, code_hash, password_hash, expires_at)
		VALUES ('migration-00014', $1, $2, 'migration-00014-a@example.com', $3, $4, now() + interval '10 minutes')`
	wantCode("purpose login", "23514", insertCode, "login", "code", hash, nil)
	wantCode("delivery other", "23514", insertCode, "signup", "other", hash, nil)
	wantCode("a code hash that is not 64 hex", "23514", insertCode, "signup", "code", "abc", nil)
	wantCode("a reset row carrying a password", "23514", insertCode, "reset", "code", hash, "$argon2id$v=19$x")
	wantCode("a signup password that is not argon2id", "23514", insertCode, "signup", "code", hash, "plain")
	if _, err := db.Exec(insertCode, "signup", "code", hash, "$argon2id$v=19$x"); err != nil {
		t.Errorf("a signup row with an argon2id password must be accepted: %v", err)
	}
	for _, delivery := range []string{"account_exists", "no_account"} {
		if _, err := db.Exec(insertCode, "reset", delivery, hash, nil); err != nil {
			t.Errorf("delivery %q must be accepted: %v", delivery, err)
		}
	}
	wantCode("negative attempts", "23514", `UPDATE auth_codes SET attempts = -1 WHERE session_id = 'migration-00014'`)
	wantCode("a throttle scope that is unknown", "23514",
		`INSERT INTO auth_throttles (scope, key_hash, failures, last_failure_at) VALUES ('other', $1, 1, now())`, hash)
	wantCode("a throttle key that is a plain address", "23514",
		`INSERT INTO auth_throttles (scope, key_hash, failures, last_failure_at) VALUES ('sign_in', 'a@example.com', 1, now())`)
	wantCode("negative window sends", "23514",
		`INSERT INTO email_cooldowns (email, last_requested_at, window_sends) VALUES ('migration-00014-c@example.com', now(), -1)`)

	// Down drops both tables and every added column whole, so no rows need to
	// be cleaned up first.
	if err := goose.DownTo(db, "migrations", 13); err != nil {
		t.Fatalf("goose down to 13: %v", err)
	}
	if tables, columns := objects(); tables != 0 || columns != 0 {
		t.Errorf("after Down: %d tables and %d columns still present", tables, columns)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if tables, columns := objects(); tables != 2 || columns != 6 {
		t.Errorf("after Up again: %d tables and %d columns present, want 2 and 6", tables, columns)
	}
}

// TestMigrationRevokesPasswordlessSessions proves the D-14 Option X data step on
// a real database: rolling back and applying 00014 ends every live session of an
// account with no password, keeps the time of a session that was already
// revoked, and a later Up changes nothing, so the step runs once.
func TestMigrationRevokesPasswordlessSessions(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the Option X migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		_, _ = db.Exec(`DELETE FROM sessions WHERE session_id LIKE 'optx-%'`)
		_, _ = db.Exec(`DELETE FROM accounts WHERE email LIKE 'optx-%@example.com'`)
		_ = db.Close()
	})

	if err := goose.DownTo(db, "migrations", 13); err != nil {
		t.Fatalf("goose down to 13: %v", err)
	}

	var a, b int64
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('optx-a@example.com') RETURNING id`).Scan(&a); err != nil {
		t.Fatalf("inserting account a: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('optx-b@example.com') RETURNING id`).Scan(&b); err != nil {
		t.Fatalf("inserting account b: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (session_id, account_id) VALUES ('optx-live-a', $1), ('optx-live-b', $2)`, a, b); err != nil {
		t.Fatalf("inserting live sessions: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (session_id, account_id, revoked_at) VALUES ('optx-old-b', $1, '2026-01-02 03:04:05+00')`, b); err != nil {
		t.Fatalf("inserting a revoked session: %v", err)
	}
	// A session with no account is not touched.
	if _, err := db.Exec(`INSERT INTO sessions (session_id) VALUES ('optx-anon')`); err != nil {
		t.Fatalf("inserting anonymous session: %v", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}

	revokedAt := func(id string) sql.NullTime {
		t.Helper()
		var at sql.NullTime
		if err := db.QueryRow(`SELECT revoked_at FROM sessions WHERE session_id = $1`, id).Scan(&at); err != nil {
			t.Fatalf("reading session %s: %v", id, err)
		}
		return at
	}
	for _, id := range []string{"optx-live-a", "optx-live-b"} {
		if !revokedAt(id).Valid {
			t.Errorf("session %s must be revoked by the migration", id)
		}
	}
	var kept bool
	if err := db.QueryRow(`SELECT revoked_at = '2026-01-02 03:04:05+00'::timestamptz FROM sessions WHERE session_id = 'optx-old-b'`).Scan(&kept); err != nil || !kept {
		t.Errorf("an already revoked session must keep its original revoked_at (%v, %v)", kept, err)
	}
	if revokedAt("optx-anon").Valid {
		t.Error("a session with no account must not be revoked")
	}

	// The step ran once: an account that now has a password and a fresh session
	// is untouched by a later Up.
	if _, err := db.Exec(`UPDATE accounts SET password_hash = '$argon2id$v=19$x' WHERE id = $1`, a); err != nil {
		t.Fatalf("setting a password: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (session_id, account_id) VALUES ('optx-fresh-a', $1)`, a); err != nil {
		t.Fatalf("inserting a fresh session: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up a third time: %v", err)
	}
	if revokedAt("optx-fresh-a").Valid {
		t.Error("a later Up must not revoke a fresh session of an account with a password")
	}
}

// TestMigration00015 covers the migration that drops the sign in link token
// table (Phase 7.1 plan 16, D-12, research Pitfall 15). The text part runs
// without a database: Up drops the table, Down recreates it with its columns
// and index as migration 00002 made them. With DATABASE_URL the table is absent
// after Up, goose.DownTo 14 brings it back with its columns and index, and
// goose.Up removes it again.
func TestMigration00015(t *testing.T) {
	const linkTable = "magic_link_" + "tokens" // Phase 7.1 plan 16: the dropped table, named once
	const linkIndex = "idx_" + linkTable + "_email_created"
	const file = "migrations/00015_drop_" + linkTable + ".sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	if !strings.Contains(text[up:down], "DROP TABLE "+linkTable+";") {
		t.Error("migration 00015 Up must drop the link token table")
	}
	for _, want := range []string{
		"CREATE TABLE " + linkTable,
		"token_hash  TEXT NOT NULL UNIQUE",
		"CREATE INDEX " + linkIndex + " ON " + linkTable + " (email, created_at DESC);",
	} {
		if !strings.Contains(text[down:], want) {
			t.Errorf("migration 00015 Down must contain %q", want)
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00015 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		_ = db.Close()
	})

	present := func() (table, cols, indexes int) {
		t.Helper()
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1`, linkTable).Scan(&table); err != nil {
			t.Fatalf("counting the table: %v", err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
			AND column_name IN ('id', 'token_hash', 'email', 'created_at', 'expires_at', 'used_at')`, linkTable).Scan(&cols); err != nil {
			t.Fatalf("counting columns: %v", err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = $1
			AND indexname = $2`, linkTable, linkIndex).Scan(&indexes); err != nil {
			t.Fatalf("counting indexes: %v", err)
		}
		return table, cols, indexes
	}
	if tbl, cols, idx := present(); tbl != 0 || cols != 0 || idx != 0 {
		t.Errorf("after Up: table %d, columns %d, index %d still present, want none", tbl, cols, idx)
	}
	if err := goose.DownTo(db, "migrations", 14); err != nil {
		t.Fatalf("goose down to 14: %v", err)
	}
	if tbl, cols, idx := present(); tbl != 1 || cols != 6 || idx != 1 {
		t.Errorf("after Down: table %d, columns %d, index %d, want 1, 6 and 1", tbl, cols, idx)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if tbl, cols, idx := present(); tbl != 0 || cols != 0 || idx != 0 {
		t.Errorf("after Up again: table %d, columns %d, index %d still present, want none", tbl, cols, idx)
	}
}

// TestMigration00017 covers the Phase 8 plan 02 migration: the file carries
// both goose markers, builds nothing concurrently and uses no ON DELETE; on a
// database the share token column exists and is nullable, the shape check
// refuses a short value and a value with a character outside base64url, two
// reports cannot hold one token, any number of reports may hold none, and Down
// then Up leaves the table as it was.
func TestMigration00017(t *testing.T) {
	const file = "migrations/00017_add_report_share_token.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	joined := strings.ToUpper(strings.Join(code, "\n"))
	if strings.Contains(joined, "CONCURRENTLY") {
		t.Error("migration 00017 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	if strings.Contains(joined, "ON DELETE") {
		t.Error("migration 00017 must not use ON DELETE")
	}
	for _, name := range []string{"reports_share_token_shape", "uq_reports_share_token"} {
		if !strings.Contains(text[up:down], name) {
			t.Errorf("migration 00017 Up must add %s", name)
		}
	}
	if !strings.Contains(text[down:], "DROP INDEX uq_reports_share_token") ||
		!strings.Contains(text[down:], "DROP COLUMN share_token") {
		t.Error("migration 00017 Down must drop the index and the column")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00017 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	const sessionID = "migration-00017-session"
	var reportA, reportB, reportC int64
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		for _, id := range []int64{reportA, reportB, reportC} {
			if id != 0 {
				_, _ = db.Exec("DELETE FROM reports WHERE id = $1", id)
			}
		}
		_, _ = db.Exec("DELETE FROM sessions WHERE session_id = $1", sessionID)
	})

	columnState := func() (present int, nullable string) {
		t.Helper()
		err := db.QueryRow(`SELECT count(*), coalesce(max(is_nullable), '') FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'reports' AND column_name = 'share_token'`).Scan(&present, &nullable)
		if err != nil {
			t.Fatalf("reading the column: %v", err)
		}
		return present, nullable
	}
	indexCount := func() int {
		t.Helper()
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM pg_indexes WHERE indexname = 'uq_reports_share_token'`).Scan(&n); err != nil {
			t.Fatalf("counting the index: %v", err)
		}
		return n
	}
	if present, nullable := columnState(); present != 1 || nullable != "YES" {
		t.Errorf("after Up: column count %d nullable %q, want 1 and YES", present, nullable)
	}
	if got := indexCount(); got != 1 {
		t.Errorf("after Up: index count %d, want 1", got)
	}

	if _, err := db.Exec(`INSERT INTO sessions (session_id) VALUES ($1)`, sessionID); err != nil {
		t.Fatalf("inserting session: %v", err)
	}
	insertReport := func(label string) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRow(`INSERT INTO reports (session_id, category, severity, description, latitude, longitude, geohash, expires_at)
			VALUES ($1, 'road', 'info', $2, 12.97, 77.59, 'tdr1w', now() + interval '1 hour')
			RETURNING id`, sessionID, "for the 00017 test "+label).Scan(&id); err != nil {
			t.Fatalf("inserting report %s: %v", label, err)
		}
		return id
	}
	reportA = insertReport("a")
	reportB = insertReport("b")
	reportC = insertReport("c")

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}
	wantCode("a short token", "23514", `UPDATE reports SET share_token = 'abc' WHERE id = $1`, reportA)
	wantCode("a 22 character token holding a plus", "23514",
		`UPDATE reports SET share_token = 'abcdefghijklmnopqrst+u' WHERE id = $1`, reportA)
	wantCode("a 23 character token", "23514",
		`UPDATE reports SET share_token = 'abcdefghijklmnopqrstuvw' WHERE id = $1`, reportA)

	// Two NULLs coexist: B and C still hold none.
	const token = "AbCdEfGhIjKlMnOpQr_-12"
	if _, err := db.Exec(`UPDATE reports SET share_token = $1 WHERE id = $2`, token, reportA); err != nil {
		t.Fatalf("setting a valid token: %v", err)
	}
	wantCode("the same token on a second report", "23505",
		`UPDATE reports SET share_token = $1 WHERE id = $2`, token, reportB)
	var nulls int
	if err := db.QueryRow(`SELECT count(*) FROM reports WHERE id IN ($1, $2) AND share_token IS NULL`, reportB, reportC).Scan(&nulls); err != nil || nulls != 2 {
		t.Errorf("reports with no token = %d (%v), want 2", nulls, err)
	}

	version, err := goose.NumericComponent("00017_add_report_share_token.sql")
	if err != nil {
		t.Fatalf("parsing version: %v", err)
	}
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if present, _ := columnState(); present != 0 {
		t.Errorf("after Down: column count %d, want 0", present)
	}
	if got := indexCount(); got != 0 {
		t.Errorf("after Down: index count %d, want 0", got)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if present, nullable := columnState(); present != 1 || nullable != "YES" {
		t.Errorf("after Up again: column count %d nullable %q, want 1 and YES", present, nullable)
	}
	if got := indexCount(); got != 1 {
		t.Errorf("after Up again: index count %d, want 1", got)
	}
}

// TestMigration00018 covers the Phase 8 plan 08 migration (PAGE-02, D-02, D-12,
// D-14): the file carries both goose markers, builds nothing concurrently and
// names no ON DELETE, and on a database creates the four comment tables whose
// checks refuse an over long or empty live body, a parent without a reply_to,
// a repeated like, a repeated flag and a bad or repeated reason. Down drops the
// four tables, Up brings them back.
func TestMigration00018(t *testing.T) {
	const file = "migrations/00018_add_comments.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	joined := strings.ToUpper(strings.Join(code, "\n"))
	if strings.Contains(joined, "CONCURRENTLY") {
		t.Error("migration 00018 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	if strings.Contains(joined, "ON DELETE") {
		t.Error("migration 00018 must not use ON DELETE")
	}
	tables := []string{"comments", "comment_likes", "comment_flags", "comment_moderation_actions"}
	for _, tbl := range tables {
		if !strings.Contains(text[up:down], "CREATE TABLE "+tbl+" ") {
			t.Errorf("migration 00018 Up must create %s", tbl)
		}
		if !strings.Contains(text[down:], "DROP TABLE "+tbl+";") {
			t.Errorf("migration 00018 Down must drop %s", tbl)
		}
	}
	if strings.Contains(joined, "CREATE FUNCTION") {
		t.Error("migration 00018 must reuse report_flag_reasons_distinct, not make a second function")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00018 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	// Closed by a cleanup registered first, so it runs after the row cleanup
	// below (a defer would close the handle before that cleanup could use it).
	t.Cleanup(func() { _ = db.Close() })

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	const sessionID = "migration-00018-session"
	var accountA, accountB, reportID int64
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		for _, tbl := range []string{"comment_moderation_actions", "comment_flags", "comment_likes"} {
			_, _ = db.Exec("DELETE FROM "+tbl+" WHERE comment_id IN (SELECT id FROM comments WHERE report_id = $1)", reportID)
		}
		_, _ = db.Exec("UPDATE comments SET parent_id = NULL, reply_to_id = NULL WHERE report_id = $1", reportID)
		_, _ = db.Exec("DELETE FROM comments WHERE report_id = $1", reportID)
		_, _ = db.Exec("DELETE FROM reports WHERE id = $1", reportID)
		_, _ = db.Exec("DELETE FROM sessions WHERE session_id = $1", sessionID)
		_, _ = db.Exec("DELETE FROM accounts WHERE id IN ($1, $2)", accountA, accountB)
	})

	tableCount := func() int {
		t.Helper()
		n := 0
		for _, tbl := range tables {
			var reg sql.NullString
			if err := db.QueryRow("SELECT to_regclass('public.' || $1)::text", tbl).Scan(&reg); err != nil {
				t.Fatalf("to_regclass %s: %v", tbl, err)
			}
			if reg.Valid {
				n++
			}
		}
		return n
	}
	if got := tableCount(); got != len(tables) {
		t.Errorf("after Up: %d of %d tables present", got, len(tables))
	}

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00018-a@example.com') RETURNING id`).Scan(&accountA); err != nil {
		t.Fatalf("inserting account a: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00018-b@example.com') RETURNING id`).Scan(&accountB); err != nil {
		t.Fatalf("inserting account b: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (session_id) VALUES ($1)`, sessionID); err != nil {
		t.Fatalf("inserting session: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO reports (session_id, category, severity, description, latitude, longitude, geohash, expires_at)
		VALUES ($1, 'road', 'info', 'for the 00018 test', 12.97, 77.59, 'tdr1w', now() + interval '1 hour')
		RETURNING id`, sessionID).Scan(&reportID); err != nil {
		t.Fatalf("inserting report: %v", err)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}

	const insert = `INSERT INTO comments (report_id, account_id, body) VALUES ($1, $2, $3)`
	wantCode("an empty live body", "23514", insert, reportID, accountA, "")
	wantCode("a 281 character body", "23514", insert, reportID, accountA, strings.Repeat("a", 281))
	if _, err := db.Exec(insert, reportID, accountA, strings.Repeat("a", 280)); err != nil {
		t.Errorf("a 280 character body must be accepted: %v", err)
	}
	var root int64
	if err := db.QueryRow(insert+` RETURNING id`, reportID, accountA, "the root").Scan(&root); err != nil {
		t.Fatalf("inserting the root: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO comments (report_id, account_id, body, deleted_at) VALUES ($1, $2, '', now())`, reportID, accountA); err != nil {
		t.Errorf("a deleted row with empty words must be accepted: %v", err)
	}
	wantCode("a deleted row that still holds words", "23514",
		`INSERT INTO comments (report_id, account_id, body, deleted_at) VALUES ($1, $2, 'words', now())`, reportID, accountA)
	wantCode("a parent without a reply_to", "23514",
		`INSERT INTO comments (report_id, account_id, body, parent_id) VALUES ($1, $2, 'x', $3)`, reportID, accountB, root)
	wantCode("a reply_to without a parent", "23514",
		`INSERT INTO comments (report_id, account_id, body, reply_to_id) VALUES ($1, $2, 'x', $3)`, reportID, accountB, root)
	wantCode("a comment that is its own parent", "23514",
		`UPDATE comments SET parent_id = id, reply_to_id = id WHERE id = $1`, root)
	wantCode("an unknown moderation state", "23514",
		`INSERT INTO comments (report_id, account_id, body, moderation_state) VALUES ($1, $2, 'x', 'held')`, reportID, accountA)
	wantCode("a pending row with no next_attempt_at", "23514",
		`INSERT INTO comments (report_id, account_id, body, moderation_state) VALUES ($1, $2, 'x', 'pending')`, reportID, accountA)
	wantCode("negative attempts", "23514",
		`INSERT INTO comments (report_id, account_id, body, check_attempts) VALUES ($1, $2, 'x', -1)`, reportID, accountA)
	wantCode("an unknown verdict", "23514",
		`INSERT INTO comments (report_id, account_id, body, verdict) VALUES ($1, $2, 'x', 'maybe')`, reportID, accountA)
	var reply int64
	if err := db.QueryRow(`INSERT INTO comments (report_id, account_id, body, parent_id, reply_to_id) VALUES ($1, $2, 'a reply', $3, $3) RETURNING id`,
		reportID, accountB, root).Scan(&reply); err != nil {
		t.Errorf("a reply with both ids must be accepted: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO comments (report_id, account_id, body, moderation_state, next_attempt_at) VALUES ($1, $2, 'held', 'pending', now())`, reportID, accountB); err != nil {
		t.Errorf("a pending row with next_attempt_at must be accepted: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO comment_likes (comment_id, account_id) VALUES ($1, $2)`, root, accountB); err != nil {
		t.Fatalf("first like: %v", err)
	}
	wantCode("a second like by the same account", "23505", `INSERT INTO comment_likes (comment_id, account_id) VALUES ($1, $2)`, root, accountB)

	const flag = `INSERT INTO comment_flags (comment_id, account_id, reasons, note) VALUES ($1, $2, $3::text[], $4)`
	wantCode("an unknown reason", "23514", flag, root, accountB, "{rude}", nil)
	wantCode("a repeated reason", "23514", flag, root, accountB, "{spam,spam}", nil)
	wantCode("an empty reason set", "23514", flag, root, accountB, "{}", nil)
	wantCode("an empty note", "23514", flag, root, accountB, "{spam}", "")
	wantCode("a 301 character note", "23514", flag, root, accountB, "{spam}", strings.Repeat("n", 301))
	if _, err := db.Exec(flag, root, accountB, "{spam,abusive}", "a note"); err != nil {
		t.Fatalf("first flag: %v", err)
	}
	wantCode("a second flag by the same account", "23505", flag, root, accountB, "{other}", nil)

	wantCode("an unknown moderation action", "23514",
		`INSERT INTO comment_moderation_actions (comment_id, moderator_account_id, action) VALUES ($1, $2, 'hide')`, root, accountA)
	if _, err := db.Exec(`INSERT INTO comment_moderation_actions (comment_id, moderator_account_id, action) VALUES ($1, $2, 'keep')`, root, accountA); err != nil {
		t.Errorf("a keep action must be accepted: %v", err)
	}

	// Down drops the four tables and leaves the helper function of 00010.
	for _, tbl := range []string{"comment_moderation_actions", "comment_flags", "comment_likes"} {
		if _, err := db.Exec("DELETE FROM " + tbl); err != nil {
			t.Fatalf("emptying %s before Down: %v", tbl, err)
		}
	}
	if _, err := db.Exec(`UPDATE comments SET parent_id = NULL, reply_to_id = NULL WHERE report_id = $1`, reportID); err != nil {
		t.Fatalf("detaching replies before Down: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM comments WHERE report_id = $1`, reportID); err != nil {
		t.Fatalf("emptying comments before Down: %v", err)
	}
	version, err := goose.NumericComponent("00018_add_comments.sql")
	if err != nil {
		t.Fatalf("parsing version: %v", err)
	}
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if got := tableCount(); got != 0 {
		t.Errorf("after Down: %d comment tables still present", got)
	}
	var fn int
	if err := db.QueryRow(`SELECT count(*) FROM pg_proc WHERE proname = 'report_flag_reasons_distinct'`).Scan(&fn); err != nil || fn != 1 {
		t.Errorf("after Down: report_flag_reasons_distinct count %d (%v), want 1", fn, err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if got := tableCount(); got != len(tables) {
		t.Errorf("after Up again: %d of %d tables present", got, len(tables))
	}
}

// TestMigration00019 covers the Phase 8 plan 11 migration (D-16, PAGE-02,
// A-P8-3): the two comment kinds join updates_kind_valid, and the unread index
// keeps thirty comments to one row per account, report and kind.
func TestMigration00019(t *testing.T) {
	const file = "migrations/00019_add_comment_update_kinds.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	joined := strings.ToUpper(strings.Join(code, "\n"))
	if strings.Contains(joined, "CONCURRENTLY") {
		t.Error("migration 00019 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	const wantList = "CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more', 'comment_on_yours', 'reply_to_yours'))"
	if !strings.Contains(text[up:down], wantList) {
		t.Errorf("migration 00019 Up must carry the widened list %q", wantList)
	}
	const oldList = "CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more'))"
	if !strings.Contains(text[down:], oldList) {
		t.Errorf("migration 00019 Down must restore the old list %q", oldList)
	}
	if !strings.Contains(text[up:down], "uq_updates_comment_unread") || !strings.Contains(text[down:], "DROP INDEX uq_updates_comment_unread") {
		t.Error("migration 00019 must create uq_updates_comment_unread in Up and drop it in Down")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00019 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	const sessionID = "migration-00019-session"
	var accountA, reportID, otherReport int64
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		_, _ = db.Exec("DELETE FROM updates WHERE account_id = $1", accountA)
		_, _ = db.Exec("DELETE FROM reports WHERE id IN ($1, $2)", reportID, otherReport)
		_, _ = db.Exec("DELETE FROM sessions WHERE session_id = $1", sessionID)
		_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", accountA)
	})

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00019-a@example.com') RETURNING id`).Scan(&accountA); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (session_id) VALUES ($1)`, sessionID); err != nil {
		t.Fatalf("inserting session: %v", err)
	}
	for _, dst := range []*int64{&reportID, &otherReport} {
		if err := db.QueryRow(`INSERT INTO reports (session_id, category, severity, description, latitude, longitude, geohash, expires_at)
			VALUES ($1, 'road', 'info', 'for the 00019 test', 12.97, 77.59, 'tdr1w', now() + interval '1 hour')
			RETURNING id`, sessionID).Scan(dst); err != nil {
			t.Fatalf("inserting report: %v", err)
		}
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}
	const insert = `INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, $3)`

	for _, kind := range []string{"comment_on_yours", "reply_to_yours"} {
		if _, err := db.Exec(insert, accountA, reportID, kind); err != nil {
			t.Errorf("kind %s must insert: %v", kind, err)
		}
	}
	wantCode("an unknown kind", "23514", insert, accountA, reportID, "comment_liked")
	wantCode("a second unread comment_on_yours for the same report", "23505", insert, accountA, reportID, "comment_on_yours")
	wantCode("a second unread reply_to_yours for the same report", "23505", insert, accountA, reportID, "reply_to_yours")
	if _, err := db.Exec(insert, accountA, otherReport, "comment_on_yours"); err != nil {
		t.Errorf("another report must insert: %v", err)
	}
	if _, err := db.Exec(`UPDATE updates SET read_at = now() WHERE account_id = $1 AND report_id = $2 AND kind = 'comment_on_yours'`, accountA, reportID); err != nil {
		t.Fatalf("marking read: %v", err)
	}
	if _, err := db.Exec(insert, accountA, reportID, "comment_on_yours"); err != nil {
		t.Errorf("after the first row is read a new unread row must insert: %v", err)
	}
	// The index is scoped to the two comment kinds: other kinds are not capped.
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(insert, accountA, reportID, "fading"); err != nil {
			t.Errorf("fading row %d must insert (the unread index covers comment kinds only): %v", i, err)
		}
	}

	version, err := goose.NumericComponent("00019_add_comment_update_kinds.sql")
	if err != nil {
		t.Fatalf("parsing version: %v", err)
	}
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM updates WHERE account_id = $1 AND kind IN ('comment_on_yours', 'reply_to_yours')`, accountA).Scan(&n); err != nil || n != 0 {
		t.Errorf("after Down: %d comment rows remain (%v), want 0", n, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM updates WHERE account_id = $1 AND kind = 'fading'`, accountA).Scan(&n); err != nil || n != 2 {
		t.Errorf("after Down: %d fading rows (%v), want the 2 older rows kept", n, err)
	}
	wantCode("a comment kind after Down", "23514", insert, accountA, reportID, "comment_on_yours")
	var idx sql.NullString
	if err := db.QueryRow(`SELECT to_regclass('public.uq_updates_comment_unread')::text`).Scan(&idx); err != nil || idx.Valid {
		t.Errorf("after Down: unread index still present (%v, %v)", idx, err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if _, err := db.Exec(insert, accountA, otherReport, "reply_to_yours"); err != nil {
		t.Errorf("after Up again a comment kind must insert: %v", err)
	}
}

// TestMigration00020 covers the Phase 9 migration (plan 09-02): the file carries
// both goose markers and builds nothing concurrently; on a database it adds the
// profile flag, the soft ending follows table and the blocks table, the named
// CHECK constraints and the primary keys refuse what they must, Down removes
// everything and Up applies again (D-01, D-12, D-21).
func TestMigration00020(t *testing.T) {
	const file = "migrations/00020_add_people_profiles.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(code, "\n")), "CONCURRENTLY") {
		t.Error("migration 00020 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	for _, name := range []string{
		"accounts_profile_public_at_set", "account_follows_not_self", "account_follows_end_shape",
		"account_follows_end_reason_valid", "person_blocks_not_self",
		"idx_account_follows_followee", "idx_account_follows_follower", "idx_account_follows_tag_lost",
		"idx_person_blocks_blocked",
	} {
		if !strings.Contains(text[:down], name) {
			t.Errorf("migration 00020 Up must add %s", name)
		}
	}
	for _, want := range []string{"DROP TABLE person_blocks;", "DROP TABLE account_follows;",
		"DROP CONSTRAINT accounts_profile_public_at_set", "DROP COLUMN profile_public_at", "DROP COLUMN profile_public"} {
		if !strings.Contains(text[down:], want) {
			t.Errorf("migration 00020 Down must contain %q", want)
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00020 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	var a, b int64
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		for _, id := range []int64{a, b} {
			if id != 0 {
				_, _ = db.Exec("DELETE FROM account_follows WHERE follower_id = $1 OR followee_id = $1", id)
				_, _ = db.Exec("DELETE FROM person_blocks WHERE blocker_id = $1 OR blocked_id = $1", id)
				_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", id)
			}
		}
	})

	present := func() (tables, columns, indexes int) {
		t.Helper()
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name IN ('account_follows', 'person_blocks')`).Scan(&tables); err != nil {
			t.Fatalf("counting tables: %v", err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'accounts'
			  AND column_name IN ('profile_public', 'profile_public_at')`).Scan(&columns); err != nil {
			t.Fatalf("counting columns: %v", err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM pg_indexes WHERE indexname IN (
			'idx_account_follows_followee', 'idx_account_follows_follower',
			'idx_account_follows_tag_lost', 'idx_person_blocks_blocked')`).Scan(&indexes); err != nil {
			t.Fatalf("counting indexes: %v", err)
		}
		return
	}
	if tb, c, i := present(); tb != 2 || c != 2 || i != 4 {
		t.Errorf("after Up: %d of 2 tables, %d of 2 columns, %d of 4 indexes", tb, c, i)
	}
	for _, name := range []string{"accounts_profile_public_at_set", "account_follows_not_self", "account_follows_end_shape",
		"account_follows_end_reason_valid", "person_blocks_not_self"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM pg_constraint WHERE conname = $1`, name).Scan(&n); err != nil || n != 1 {
			t.Errorf("after Up: constraint %s count %d (%v), want 1", name, n, err)
		}
	}

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00020-a@example.com') RETURNING id`).Scan(&a); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00020-b@example.com') RETURNING id`).Scan(&b); err != nil {
		t.Fatalf("inserting second account: %v", err)
	}
	var public bool
	var publicAt sql.NullTime
	if err := db.QueryRow(`SELECT profile_public, profile_public_at FROM accounts WHERE id = $1`, a).Scan(&public, &publicAt); err != nil {
		t.Fatalf("reading profile flag: %v", err)
	}
	if public || publicAt.Valid {
		t.Errorf("defaults = profile_public %v profile_public_at %v, want false and NULL", public, publicAt.Valid)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}
	wantCode("a public profile with no time", "23514",
		`UPDATE accounts SET profile_public = true WHERE id = $1`, a)
	wantCode("a self follow", "23514",
		`INSERT INTO account_follows (follower_id, followee_id) VALUES ($1, $1)`, a)
	wantCode("ended_at without end_reason", "23514",
		`INSERT INTO account_follows (follower_id, followee_id, ended_at) VALUES ($1, $2, now())`, a, b)
	wantCode("end_reason without ended_at", "23514",
		`INSERT INTO account_follows (follower_id, followee_id, end_reason) VALUES ($1, $2, 'unfollowed')`, a, b)
	wantCode("an unknown end_reason", "23514",
		`INSERT INTO account_follows (follower_id, followee_id, ended_at, end_reason) VALUES ($1, $2, now(), 'because')`, a, b)
	wantCode("a self block", "23514",
		`INSERT INTO person_blocks (blocker_id, blocked_id) VALUES ($1, $1)`, a)

	if _, err := db.Exec(`INSERT INTO account_follows (follower_id, followee_id) VALUES ($1, $2)`, a, b); err != nil {
		t.Fatalf("inserting a follow: %v", err)
	}
	wantCode("the same follow twice", "23505",
		`INSERT INTO account_follows (follower_id, followee_id) VALUES ($1, $2)`, a, b)
	if _, err := db.Exec(`INSERT INTO account_follows (follower_id, followee_id, ended_at, end_reason) VALUES ($1, $2, now(), 'tag_lost')`, b, a); err != nil {
		t.Errorf("an ended follow with a valid reason must insert: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO person_blocks (blocker_id, blocked_id) VALUES ($1, $2)`, a, b); err != nil {
		t.Fatalf("inserting a block: %v", err)
	}
	wantCode("the same block twice", "23505",
		`INSERT INTO person_blocks (blocker_id, blocked_id) VALUES ($1, $2)`, a, b)

	version, err := goose.NumericComponent("00020_add_people_profiles.sql")
	if err != nil {
		t.Fatalf("parsing version: %v", err)
	}
	// Down drops both tables whole, so their rows need no clean up first.
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if tb, c, i := present(); tb != 0 || c != 0 || i != 0 {
		t.Errorf("after Down: %d tables, %d columns, %d indexes still present", tb, c, i)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if tb, c, i := present(); tb != 2 || c != 2 || i != 4 {
		t.Errorf("after Up again: %d of 2 tables, %d of 2 columns, %d of 4 indexes", tb, c, i)
	}
}

// TestMigration00021 covers the Phase 9 ratings migration (plan 09-13): the file
// carries both goose markers and builds nothing concurrently; on a database it
// adds the profile_ratings table, its two indexes and the rater removed columns
// of accounts, the named CHECK constraints refuse a self rating, stars outside 1
// to 5 and both half stamped removal shapes, Down removes everything and Up
// applies again (D-02, D-14, D-21, W4).
func TestMigration00021(t *testing.T) {
	const file = "migrations/00021_add_profile_ratings.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(code, "\n")), "CONCURRENTLY") {
		t.Error("migration 00021 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	for _, name := range []string{
		"CREATE TABLE profile_ratings", "profile_ratings_not_self", "profile_ratings_stars_range",
		"profile_ratings_removed_shape", "idx_profile_ratings_reporter", "idx_profile_ratings_rater",
		"accounts_rater_removed_shape", "rater_removed_at", "rater_removed_by",
	} {
		if !strings.Contains(text[:down], name) {
			t.Errorf("migration 00021 Up must add %s", name)
		}
	}
	for _, want := range []string{"DROP TABLE profile_ratings;", "DROP CONSTRAINT accounts_rater_removed_shape",
		"DROP COLUMN rater_removed_by", "DROP COLUMN rater_removed_at"} {
		if !strings.Contains(text[down:], want) {
			t.Errorf("migration 00021 Down must contain %q", want)
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00021 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	var a, b int64
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		for _, id := range []int64{a, b} {
			if id != 0 {
				_, _ = db.Exec("DELETE FROM profile_ratings WHERE rater_id = $1 OR reporter_id = $1", id)
				_, _ = db.Exec("UPDATE accounts SET rater_removed_at = NULL, rater_removed_by = NULL WHERE id = $1", id)
				_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", id)
			}
		}
	})

	present := func() (tables, columns, indexes int) {
		t.Helper()
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'profile_ratings'`).Scan(&tables); err != nil {
			t.Fatalf("counting tables: %v", err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'accounts'
			  AND column_name IN ('rater_removed_at', 'rater_removed_by')`).Scan(&columns); err != nil {
			t.Fatalf("counting columns: %v", err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM pg_indexes WHERE indexname IN (
			'idx_profile_ratings_reporter', 'idx_profile_ratings_rater')`).Scan(&indexes); err != nil {
			t.Fatalf("counting indexes: %v", err)
		}
		return
	}
	if tb, c, i := present(); tb != 1 || c != 2 || i != 2 {
		t.Errorf("after Up: %d of 1 tables, %d of 2 columns, %d of 2 indexes", tb, c, i)
	}
	for _, name := range []string{"profile_ratings_not_self", "profile_ratings_stars_range",
		"profile_ratings_removed_shape", "accounts_rater_removed_shape"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM pg_constraint WHERE conname = $1`, name).Scan(&n); err != nil || n != 1 {
			t.Errorf("after Up: constraint %s count %d (%v), want 1", name, n, err)
		}
	}

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00021-a@example.com') RETURNING id`).Scan(&a); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00021-b@example.com') RETURNING id`).Scan(&b); err != nil {
		t.Fatalf("inserting second account: %v", err)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}
	wantCode("a self rating", "23514",
		`INSERT INTO profile_ratings (rater_id, reporter_id, stars) VALUES ($1, $1, 3)`, a)
	wantCode("zero stars", "23514",
		`INSERT INTO profile_ratings (rater_id, reporter_id, stars) VALUES ($1, $2, 0)`, a, b)
	wantCode("six stars", "23514",
		`INSERT INTO profile_ratings (rater_id, reporter_id, stars) VALUES ($1, $2, 6)`, a, b)
	wantCode("removed_at without removed_by", "23514",
		`INSERT INTO profile_ratings (rater_id, reporter_id, stars, removed_at) VALUES ($1, $2, 3, now())`, a, b)
	wantCode("removed_by without removed_at", "23514",
		`INSERT INTO profile_ratings (rater_id, reporter_id, stars, removed_by) VALUES ($1, $2, 3, $1)`, a, b)
	wantCode("rater_removed_at without rater_removed_by", "23514",
		`UPDATE accounts SET rater_removed_at = now() WHERE id = $1`, a)
	wantCode("rater_removed_by without rater_removed_at", "23514",
		`UPDATE accounts SET rater_removed_by = $2 WHERE id = $1`, a, b)

	if _, err := db.Exec(`INSERT INTO profile_ratings (rater_id, reporter_id, stars) VALUES ($1, $2, 5)`, a, b); err != nil {
		t.Fatalf("inserting a rating: %v", err)
	}
	wantCode("the same rating twice", "23505",
		`INSERT INTO profile_ratings (rater_id, reporter_id, stars) VALUES ($1, $2, 4)`, a, b)
	if _, err := db.Exec(`INSERT INTO profile_ratings (rater_id, reporter_id, stars, removed_at, removed_by) VALUES ($1, $2, 1, now(), $1)`, b, a); err != nil {
		t.Errorf("a removed rating with both stamps must insert: %v", err)
	}
	if _, err := db.Exec(`UPDATE accounts SET rater_removed_at = now(), rater_removed_by = $2 WHERE id = $1`, a, b); err != nil {
		t.Errorf("a rater removal with both stamps must update: %v", err)
	}

	version, err := goose.NumericComponent("00021_add_profile_ratings.sql")
	if err != nil {
		t.Fatalf("parsing version: %v", err)
	}
	// Down drops the table whole, so its rows need no clean up first.
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if tb, c, i := present(); tb != 0 || c != 0 || i != 0 {
		t.Errorf("after Down: %d tables, %d columns, %d indexes still present", tb, c, i)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if tb, c, i := present(); tb != 1 || c != 2 || i != 2 {
		t.Errorf("after Up again: %d of 1 tables, %d of 2 columns, %d of 2 indexes", tb, c, i)
	}
}

// TestMigration00022 covers the Phase 9 person mute migration (plan 09-18): the
// file carries both goose markers and builds nothing concurrently; on a database
// it adds the person_mutes table and its index, the named CHECK constraint
// refuses a mute of yourself (SQLSTATE 23514), a second row for the same pair is
// refused (23505), Down removes everything and Up applies again (D-12, G4). The
// table is separate from report_mutes, which it must not touch.
func TestMigration00022(t *testing.T) {
	const file = "migrations/00022_add_person_mutes.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(code, "\n")), "CONCURRENTLY") {
		t.Error("migration 00022 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	if strings.Contains(strings.Join(code, "\n"), "report_mutes") {
		t.Error("migration 00022 must not touch report_mutes: the two tables are separate (G4)")
	}
	for _, name := range []string{"CREATE TABLE person_mutes", "person_mutes_not_self", "idx_person_mutes_muted",
		"PRIMARY KEY (muter_id, muted_id)"} {
		if !strings.Contains(text[:down], name) {
			t.Errorf("migration 00022 Up must add %s", name)
		}
	}
	if !strings.Contains(text[down:], "DROP TABLE person_mutes;") {
		t.Error("migration 00022 Down must drop person_mutes")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00022 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	var a, b int64
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		for _, id := range []int64{a, b} {
			if id != 0 {
				_, _ = db.Exec("DELETE FROM person_mutes WHERE muter_id = $1 OR muted_id = $1", id)
				_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", id)
			}
		}
	})

	present := func() (tables, indexes int) {
		t.Helper()
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'person_mutes'`).Scan(&tables); err != nil {
			t.Fatalf("counting tables: %v", err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_person_mutes_muted'`).Scan(&indexes); err != nil {
			t.Fatalf("counting indexes: %v", err)
		}
		return
	}
	if tb, i := present(); tb != 1 || i != 1 {
		t.Errorf("after Up: %d of 1 tables, %d of 1 indexes", tb, i)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM pg_constraint WHERE conname = 'person_mutes_not_self'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("after Up: constraint person_mutes_not_self count %d (%v), want 1", n, err)
	}

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00022-a@example.com') RETURNING id`).Scan(&a); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00022-b@example.com') RETURNING id`).Scan(&b); err != nil {
		t.Fatalf("inserting second account: %v", err)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}
	wantCode("a mute of yourself", "23514", `INSERT INTO person_mutes (muter_id, muted_id) VALUES ($1, $1)`, a)
	wantCode("a mute of an account that does not exist", "23503", `INSERT INTO person_mutes (muter_id, muted_id) VALUES ($1, 999999999)`, a)
	if _, err := db.Exec(`INSERT INTO person_mutes (muter_id, muted_id) VALUES ($1, $2)`, a, b); err != nil {
		t.Fatalf("inserting a mute: %v", err)
	}
	wantCode("the same mute twice", "23505", `INSERT INTO person_mutes (muter_id, muted_id) VALUES ($1, $2)`, a, b)
	if _, err := db.Exec(`INSERT INTO person_mutes (muter_id, muted_id) VALUES ($1, $2)`, b, a); err != nil {
		t.Errorf("the opposite mute is a different row and must insert: %v", err)
	}

	version, err := goose.NumericComponent("00022_add_person_mutes.sql")
	if err != nil {
		t.Fatalf("parsing version: %v", err)
	}
	// Down drops the table whole, so its rows need no clean up first.
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if tb, i := present(); tb != 0 || i != 0 {
		t.Errorf("after Down: %d tables, %d indexes still present", tb, i)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if tb, i := present(); tb != 1 || i != 1 {
		t.Errorf("after Up again: %d of 1 tables, %d of 1 indexes", tb, i)
	}
}

// TestMigration00023 covers the Phase 9 follow note migration (plan 09-21): the
// file carries both goose markers and builds nothing concurrently, never names
// Phase 8's unread comment index, and carries the two literal kind lists. On a
// database it adds the Notify me columns and their CHECK (notify is true exactly
// when notify_since is set), widens the kind CHECK to accept followed_posted,
// replaces the Reliable only unique index with one first alert index over both
// kinds, and Down removes the followed_posted rows first, restores the old index
// and the eight kind list, and Up applies again (D-08, D-09, Pitfall 5).
func TestMigration00023(t *testing.T) {
	const file = "migrations/00023_add_follow_notes.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(code, "\n")), "CONCURRENTLY") {
		t.Error("migration 00023 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	if strings.Contains(text, "uq_updates_comment_unread") {
		t.Error("migration 00023 must not name Phase 8's unread comment index")
	}
	for _, name := range []string{"ADD COLUMN notify ", "ADD COLUMN notify_since", "account_follows_notify_since",
		"idx_account_follows_notify_follower", "idx_account_follows_notify_followee",
		"'reply_to_yours', 'followed_posted'))", "DROP INDEX uq_updates_reliable_once", "uq_updates_first_alert",
		"WHERE kind IN ('reliable_nearby', 'followed_posted')"} {
		if !strings.Contains(text[:down], name) {
			t.Errorf("migration 00023 Up must contain %s", name)
		}
	}
	for _, want := range []string{"DELETE FROM updates WHERE kind = 'followed_posted'", "DROP INDEX uq_updates_first_alert",
		"WHERE kind = 'reliable_nearby'", "'comment_on_yours', 'reply_to_yours'))", "DROP COLUMN notify_since", "DROP COLUMN notify"} {
		if !strings.Contains(text[down:], want) {
			t.Errorf("migration 00023 Down must contain %q", want)
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00023 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	var a, b, report int64
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		if report != 0 {
			_, _ = db.Exec("DELETE FROM updates WHERE report_id = $1", report)
			_, _ = db.Exec("DELETE FROM reports WHERE id = $1", report)
		}
		for _, id := range []int64{a, b} {
			if id != 0 {
				_, _ = db.Exec("DELETE FROM account_follows WHERE follower_id = $1 OR followee_id = $1", id)
				_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", id)
			}
		}
	})

	count := func(query string) (n int) {
		t.Helper()
		if err := db.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("counting with %q: %v", query, err)
		}
		return
	}
	present := func() (columns, indexes, oldIndexes int) {
		t.Helper()
		columns = count(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'account_follows'
			  AND column_name IN ('notify', 'notify_since')`)
		indexes = count(`SELECT count(*) FROM pg_indexes WHERE indexname IN (
			'idx_account_follows_notify_follower', 'idx_account_follows_notify_followee', 'uq_updates_first_alert')`)
		oldIndexes = count(`SELECT count(*) FROM pg_indexes WHERE indexname = 'uq_updates_reliable_once'`)
		return
	}
	if c, i, o := present(); c != 2 || i != 3 || o != 0 {
		t.Errorf("after Up: %d of 2 columns, %d of 3 indexes, %d old indexes (want 0)", c, i, o)
	}
	if n := count(`SELECT count(*) FROM pg_constraint WHERE conname = 'account_follows_notify_since'`); n != 1 {
		t.Errorf("after Up: constraint account_follows_notify_since count %d, want 1", n)
	}
	if n := count(`SELECT count(*) FROM pg_indexes WHERE indexname = 'uq_updates_comment_unread'`); n != 1 {
		t.Errorf("after Up: Phase 8's uq_updates_comment_unread count %d, want 1 (untouched)", n)
	}

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00023-a@example.com') RETURNING id`).Scan(&a); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00023-b@example.com') RETURNING id`).Scan(&b); err != nil {
		t.Fatalf("inserting second account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO reports
		(session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at)
		VALUES ('migration-00023-session', 'flood', 'medium', 'migration test report', 12.9716, 77.5946, 'tdr1v8y',
		        now() - interval '1 hour', now() + interval '5 hours') RETURNING id`).Scan(&report); err != nil {
		t.Fatalf("inserting report: %v", err)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}
	// Notify me is off by default and carries no time.
	if _, err := db.Exec(`INSERT INTO account_follows (follower_id, followee_id) VALUES ($1, $2)`, a, b); err != nil {
		t.Fatalf("inserting a follow: %v", err)
	}
	var notify bool
	var since sql.NullTime
	if err := db.QueryRow(`SELECT notify, notify_since FROM account_follows WHERE follower_id = $1 AND followee_id = $2`, a, b).
		Scan(&notify, &since); err != nil || notify || since.Valid {
		t.Errorf("a new follow: notify %v, notify_since valid %v (%v), want off with no time", notify, since.Valid, err)
	}
	wantCode("notify on with no time", "23514",
		`UPDATE account_follows SET notify = true WHERE follower_id = $1 AND followee_id = $2`, a, b)
	wantCode("a time with notify off", "23514",
		`UPDATE account_follows SET notify_since = now() WHERE follower_id = $1 AND followee_id = $2`, a, b)
	if _, err := db.Exec(`UPDATE account_follows SET notify = true, notify_since = now() WHERE follower_id = $1 AND followee_id = $2`, a, b); err != nil {
		t.Errorf("notify on with a time must update: %v", err)
	}

	// The kind CHECK accepts followed_posted and the one index covers both kinds.
	if _, err := db.Exec(`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'followed_posted')`, a, report); err != nil {
		t.Fatalf("followed_posted must be accepted by the kind check: %v", err)
	}
	wantCode("a Reliable alert after the follow note", "23505",
		`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'reliable_nearby')`, a, report)
	wantCode("a second follow note", "23505",
		`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'followed_posted')`, a, report)
	wantCode("an unknown kind", "23514",
		`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'followed_nothing')`, b, report)

	version, err := goose.NumericComponent("00023_add_follow_notes.sql")
	if err != nil {
		t.Fatalf("parsing version: %v", err)
	}
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if c, i, o := present(); c != 0 || i != 0 || o != 1 {
		t.Errorf("after Down: %d columns, %d new indexes, %d of 1 old index", c, i, o)
	}
	if n := count(`SELECT count(*) FROM updates WHERE kind = 'followed_posted'`); n != 0 {
		t.Errorf("after Down: %d followed_posted rows, want 0", n)
	}
	wantCode("followed_posted after Down", "23514",
		`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'followed_posted')`, b, report)
	// The old index is back: a second Reliable alert for the pair is refused.
	if _, err := db.Exec(`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'reliable_nearby')`, a, report); err != nil {
		t.Fatalf("a Reliable alert after Down: %v", err)
	}
	wantCode("a second Reliable alert after Down", "23505",
		`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'reliable_nearby')`, a, report)
	if _, err := db.Exec(`DELETE FROM updates WHERE report_id = $1`, report); err != nil {
		t.Fatalf("clearing updates: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if c, i, o := present(); c != 2 || i != 3 || o != 0 {
		t.Errorf("after Up again: %d of 2 columns, %d of 3 indexes, %d old indexes", c, i, o)
	}
}

// TestMigration00024 pins the follow push migration (Phase 9 plan 25, D-09): the
// outbox kind list and the one first alert index learn the follow kind, and
// push_report_progress gains the follow pass columns. Up, Down and Up again.
func TestMigration00024(t *testing.T) {
	const file = "migrations/00024_add_follow_push.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(code, "\n")), "CONCURRENTLY") {
		t.Error("migration 00024 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	for _, name := range []string{"'critical', 'reliable', 'changed', 'confirmed_update', 'follow'",
		"DROP INDEX uq_push_outbox_first", "WHERE kind IN ('critical', 'reliable', 'follow')",
		"ADD COLUMN follow_state TEXT", "follow_at ", "follow_state IS NULL OR follow_state IN ('fanned', 'capped')"} {
		if !strings.Contains(text[:down], name) {
			t.Errorf("migration 00024 Up must contain %s", name)
		}
	}
	for _, want := range []string{"DELETE FROM push_outbox WHERE kind = 'follow'", "WHERE kind IN ('critical', 'reliable');",
		"'critical', 'reliable', 'changed', 'confirmed_update'));", "DROP COLUMN follow_at", "DROP COLUMN follow_state"} {
		if !strings.Contains(text[down:], want) {
			t.Errorf("migration 00024 Down must contain %q", want)
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00024 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	var a, report int64
	const sessionID = "migration-00024-session"
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		if report != 0 {
			_, _ = db.Exec("DELETE FROM push_outbox WHERE report_id = $1", report)
			_, _ = db.Exec("DELETE FROM push_report_progress WHERE report_id = $1", report)
			_, _ = db.Exec("DELETE FROM reports WHERE id = $1", report)
		}
		if a != 0 {
			_, _ = db.Exec("DELETE FROM sessions WHERE account_id = $1", a)
			_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", a)
		}
	})

	count := func(query string) (n int) {
		t.Helper()
		if err := db.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("counting with %q: %v", query, err)
		}
		return
	}
	columns := func() int {
		t.Helper()
		return count(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'push_report_progress'
			  AND column_name IN ('follow_state', 'follow_at')`)
	}
	indexDef := func() string {
		t.Helper()
		var def string
		if err := db.QueryRow(`SELECT indexdef FROM pg_indexes WHERE indexname = 'uq_push_outbox_first'`).Scan(&def); err != nil {
			t.Fatalf("reading uq_push_outbox_first: %v", err)
		}
		return def
	}
	if got := columns(); got != 2 {
		t.Errorf("after Up: %d of 2 progress columns", got)
	}
	if def := indexDef(); !strings.Contains(def, "'follow'") {
		t.Errorf("after Up: the first alert index is %q, want it to name follow", def)
	}

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00024-a@example.com') RETURNING id`).Scan(&a); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (session_id, account_id) VALUES ($1, $2)`, sessionID, a); err != nil {
		t.Fatalf("inserting session: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO reports
		(session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at)
		VALUES ($1, 'flood', 'medium', 'migration test report', 12.9716, 77.5946, 'tdr1v8y',
		        now() - interval '1 hour', now() + interval '5 hours') RETURNING id`, sessionID).Scan(&report); err != nil {
		t.Fatalf("inserting report: %v", err)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}
	// The kind CHECK accepts follow; the one index covers all three first alert kinds.
	if _, err := db.Exec(`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'follow', now() + interval '1 hour')`,
		a, report); err != nil {
		t.Fatalf("follow must be accepted by the kind check: %v", err)
	}
	wantCode("a second follow row", "23505",
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'follow', now() + interval '1 hour')`, a, report)
	wantCode("a reliable row beside the follow row", "23505",
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'reliable', now() + interval '1 hour')`, a, report)
	wantCode("a critical row beside the follow row", "23505",
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'critical', now() + interval '1 hour')`, a, report)
	wantCode("an unknown kind", "23514",
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'followed', now() + interval '1 hour')`, a, report)
	// A follow row is not a changed row: it carries no update id.
	wantCode("a follow row with an update id", "23514",
		`INSERT INTO push_outbox (account_id, report_id, kind, update_id, expires_at) VALUES ($1, $2, 'follow', 7, now() + interval '1 hour')`, a, report)
	// follow_state has the shape of reliable_state.
	wantCode("an unknown follow state", "23514",
		`INSERT INTO push_report_progress (report_id, follow_state) VALUES ($1, 'sent')`, report)
	wantCode("an unknown reliable state still refused", "23514",
		`INSERT INTO push_report_progress (report_id, reliable_state) VALUES ($1, 'sent')`, report)
	if _, err := db.Exec(`INSERT INTO push_report_progress (report_id, follow_state, follow_at) VALUES ($1, 'fanned', now())`, report); err != nil {
		t.Errorf("follow_state fanned must be accepted: %v", err)
	}

	version, err := goose.NumericComponent("00024_add_follow_push.sql")
	if err != nil {
		t.Fatalf("parsing version: %v", err)
	}
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if got := columns(); got != 0 {
		t.Errorf("after Down: %d progress columns still present", got)
	}
	if def := indexDef(); strings.Contains(def, "'follow'") {
		t.Errorf("after Down: the first alert index is %q, want the two kind form", def)
	}
	if n := count(`SELECT count(*) FROM push_outbox WHERE kind = 'follow'`); n != 0 {
		t.Errorf("after Down: %d follow rows, want 0", n)
	}
	wantCode("follow after Down", "23514",
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'follow', now() + interval '1 hour')`, a, report)
	// The old index is back: a reliable row is accepted once and a critical one beside it is refused.
	if _, err := db.Exec(`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'reliable', now() + interval '1 hour')`,
		a, report); err != nil {
		t.Fatalf("a reliable row after Down: %v", err)
	}
	wantCode("a critical row beside the reliable row after Down", "23505",
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'critical', now() + interval '1 hour')`, a, report)
	if _, err := db.Exec(`DELETE FROM push_outbox WHERE report_id = $1`, report); err != nil {
		t.Fatalf("clearing the outbox: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if got := columns(); got != 2 {
		t.Errorf("after Up again: %d of 2 progress columns", got)
	}
	if def := indexDef(); !strings.Contains(def, "'follow'") {
		t.Errorf("after Up again: the first alert index is %q, want it to name follow", def)
	}
}

// TestMigration00026 pins the review hide migration (quick 261117-vvb): a flag
// gains a nullable place cell of 7 characters, a report gains the review state
// and the review_kept_at mark, and the bell learns the four rows about the
// reader's own report, once per account, report and kind. Up, Down and Up
// again; Down moves a report still hidden for review back to pending.
func TestMigration00026(t *testing.T) {
	const file = "migrations/00026_add_review_hide.sql"
	contents, err := MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		code = append(code, line)
	}
	if strings.Contains(strings.ToUpper(strings.Join(code, "\n")), "CONCURRENTLY") {
		t.Error("migration 00026 must not use CONCURRENTLY: goose runs it inside a transaction")
	}
	for _, name := range []string{"ADD COLUMN flagger_cell TEXT", "char_length(flagger_cell) = 7",
		"'clear', 'pending', 'removed', 'review'", "ADD COLUMN review_kept_at TIMESTAMPTZ",
		"'followed_posted', 'own_shown', 'own_review', 'own_kept', 'own_removed'", "CREATE UNIQUE INDEX uq_updates_own_report"} {
		if !strings.Contains(text[:down], name) {
			t.Errorf("migration 00026 Up must contain %s", name)
		}
	}
	for _, want := range []string{"DROP INDEX uq_updates_own_report",
		"DELETE FROM updates WHERE kind IN ('own_shown', 'own_review', 'own_kept', 'own_removed')",
		"UPDATE reports SET moderation_state = 'pending' WHERE moderation_state = 'review'",
		"CHECK (moderation_state IN ('clear', 'pending', 'removed'));", "DROP COLUMN review_kept_at", "DROP COLUMN flagger_cell"} {
		if !strings.Contains(text[down:], want) {
			t.Errorf("migration 00026 Down must contain %q", want)
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00026 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	var a, b, report int64
	const sessionID = "migration-00026-session"
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		if report != 0 {
			_, _ = db.Exec("DELETE FROM updates WHERE report_id = $1", report)
			_, _ = db.Exec("DELETE FROM report_flags WHERE report_id = $1", report)
			_, _ = db.Exec("DELETE FROM reports WHERE id = $1", report)
		}
		for _, id := range []int64{a, b} {
			if id != 0 {
				_, _ = db.Exec("DELETE FROM sessions WHERE account_id = $1", id)
				_, _ = db.Exec("DELETE FROM accounts WHERE id = $1", id)
			}
		}
	})

	count := func(query string, args ...any) (n int) {
		t.Helper()
		if err := db.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatalf("counting with %q: %v", query, err)
		}
		return
	}
	columns := func() int {
		t.Helper()
		return count(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND ((table_name = 'report_flags' AND column_name = 'flagger_cell')
			    OR (table_name = 'reports' AND column_name = 'review_kept_at'))`)
	}
	index := func() int {
		t.Helper()
		return count(`SELECT count(*) FROM pg_indexes WHERE indexname = 'uq_updates_own_report'`)
	}
	if c, i := columns(), index(); c != 2 || i != 1 {
		t.Errorf("after Up: %d of 2 columns, %d of 1 index", c, i)
	}

	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00026-a@example.com') RETURNING id`).Scan(&a); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email) VALUES ('migration-00026-b@example.com') RETURNING id`).Scan(&b); err != nil {
		t.Fatalf("inserting account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (session_id, account_id) VALUES ($1, $2)`, sessionID, a); err != nil {
		t.Fatalf("inserting session: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO reports
		(session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at)
		VALUES ($1, 'flood', 'medium', 'migration test report', 12.9716, 77.5946, 'tdr1v8y',
		        now() - interval '1 hour', now() + interval '5 hours') RETURNING id`, sessionID).Scan(&report); err != nil {
		t.Fatalf("inserting report: %v", err)
	}

	wantCode := func(label, code, query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Errorf("%s: want SQLSTATE %s, got %v", label, code, err)
		}
	}
	// The cell is 7 characters or NULL.
	if _, err := db.Exec(`INSERT INTO report_flags (report_id, account_id, reasons, flagger_cell) VALUES ($1, $2, ARRAY['spam'], 'tdr1v8y')`, report, b); err != nil {
		t.Fatalf("a flag with a 7 character cell: %v", err)
	}
	wantCode("a 5 character cell", "23514",
		`UPDATE report_flags SET flagger_cell = 'tdr1v' WHERE report_id = $1`, report)
	if _, err := db.Exec(`UPDATE report_flags SET flagger_cell = NULL WHERE report_id = $1`, report); err != nil {
		t.Errorf("a flag with no cell: %v", err)
	}
	// review is a state, held still is not.
	if _, err := db.Exec(`UPDATE reports SET moderation_state = 'review' WHERE id = $1`, report); err != nil {
		t.Errorf("setting review: %v", err)
	}
	wantCode("setting held", "23514", `UPDATE reports SET moderation_state = 'held' WHERE id = $1`, report)
	// The four own kinds, once per account, report and kind.
	for _, kind := range []string{"own_shown", "own_review", "own_kept", "own_removed"} {
		if _, err := db.Exec(`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, $3)`, a, report, kind); err != nil {
			t.Errorf("own kind %s: %v", kind, err)
		}
	}
	wantCode("a second own_review row", "23505",
		`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'own_review')`, a, report)
	wantCode("an unknown kind", "23514",
		`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'own_hidden')`, a, report)
	// A read row still blocks a second one: each event happens once.
	if _, err := db.Exec(`UPDATE updates SET read_at = now() WHERE report_id = $1`, report); err != nil {
		t.Fatalf("marking read: %v", err)
	}
	wantCode("a second own_kept row after the first was read", "23505",
		`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'own_kept')`, a, report)

	version, err := goose.NumericComponent("00026_add_review_hide.sql")
	if err != nil {
		t.Fatalf("parsing version: %v", err)
	}
	if err := goose.DownTo(db, "migrations", version-1); err != nil {
		t.Fatalf("goose down to %d: %v", version-1, err)
	}
	if c, i := columns(), index(); c != 0 || i != 0 {
		t.Errorf("after Down: %d columns, %d indexes still present", c, i)
	}
	if n := count(`SELECT count(*) FROM updates WHERE report_id = $1`, report); n != 0 {
		t.Errorf("after Down: %d own rows, want 0", n)
	}
	var state string
	if err := db.QueryRow(`SELECT moderation_state FROM reports WHERE id = $1`, report).Scan(&state); err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if state != "pending" {
		t.Errorf("after Down: a hidden report is %q, want pending", state)
	}
	wantCode("review after Down", "23514", `UPDATE reports SET moderation_state = 'review' WHERE id = $1`, report)
	wantCode("own_shown after Down", "23514",
		`INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'own_shown')`, a, report)

	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if c, i := columns(), index(); c != 2 || i != 1 {
		t.Errorf("after Up again: %d of 2 columns, %d of 1 index", c, i)
	}
}
