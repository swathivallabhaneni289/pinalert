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
