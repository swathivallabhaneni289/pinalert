package store

import (
	"strings"
	"testing"
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
}
