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
