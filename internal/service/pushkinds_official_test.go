// pushkinds_official_test.go pins the official push constants and the default of
// the official switch (plan 05-11, owner decisions D-11 and D-16). A change to a
// value is a deliberate edit of this file.

package service

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestOfficialPushCapsAreNamed(t *testing.T) {
	ints := []struct {
		name      string
		got, want int
	}{
		{"OfficialPushPerAccountPer6Hours", OfficialPushPerAccountPer6Hours, 3},
		{"OfficialPushRecipientsPerAlert", OfficialPushRecipientsPerAlert, 2000},
	}
	for _, c := range ints {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}

	durations := []struct {
		name      string
		got, want time.Duration
	}{
		{"OfficialPushCapWindow", OfficialPushCapWindow, 6 * time.Hour},
		{"OfficialPushTTL", OfficialPushTTL, 2 * time.Hour},
		{"OfficialPushSentWindow", OfficialPushSentWindow, time.Hour},
		{"OfficialPushExpiryMargin", OfficialPushExpiryMargin, 10 * time.Minute},
	}
	for _, c := range durations {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}

	if !DefaultOfficialPushOn {
		t.Error("DefaultOfficialPushOn must be true (D-11: on by default)")
	}
}

// TestOfficialPushDefaultMatchesSQL pins the Go default to the SQL default: the
// constant is true, and the query text and the migration both read an account
// with no settings row as on. It reads the files, as Phase 3.3's Critical
// default test does.
func TestOfficialPushDefaultMatchesSQL(t *testing.T) {
	if !DefaultOfficialPushOn {
		t.Fatal("DefaultOfficialPushOn must be true")
	}
	queries, err := os.ReadFile("../store/queries/official_push.sql")
	if err != nil {
		t.Fatalf("reading official_push.sql: %v", err)
	}
	text := string(queries)
	if !strings.Contains(text, "COALESCE(s.official_push_on, true)") {
		t.Error("official_push.sql must read official_push_on with COALESCE(s.official_push_on, true)")
	}
	if strings.Contains(text, "COALESCE(s.official_push_on, false)") || strings.Contains(text, "COALESCE(official_push_on, false)") {
		t.Error("official_push.sql must never default official_push_on to false")
	}
	migration, err := os.ReadFile("../store/migrations/00013_add_official_push.sql")
	if err != nil {
		t.Fatalf("reading the migration: %v", err)
	}
	if !strings.Contains(string(migration), "official_push_on BOOLEAN NOT NULL DEFAULT true") {
		t.Error("the migration must add official_push_on with DEFAULT true")
	}
}
