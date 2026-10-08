// This file seeds reporter history for the D-10 and D-13 store tests: an
// account bound to one or more sessions, and reports filed from those
// sessions with an exact expiry the test chooses.
package testutil

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"
)

// Default sizes for SeedReporterHistory. The 48,000 reports (200 accounts times
// 3 sessions times 80 reports) sit near DefaultSeedCount, so the planner's
// choice of index for the history read is a real one and not a small table
// artefact.
const (
	HistorySeedAccounts           = 200
	HistorySeedSessionsPerAccount = 3
	HistorySeedReportsPerSession  = 80
)

// SeedReporterAccount inserts an account with the given email, binds each of
// sessionIDs to it through sessions.account_id, and returns the account id.
func SeedReporterAccount(t *testing.T, pool *pgxpool.Pool, email string, sessionIDs ...string) int64 {
	t.Helper()

	ctx := context.Background()
	var accountID int64
	if err := pool.QueryRow(ctx,
		"INSERT INTO accounts (email) VALUES ($1) RETURNING id", email,
	).Scan(&accountID); err != nil {
		t.Fatalf("seeding account %q: %v", email, err)
	}
	for _, sessionID := range sessionIDs {
		if _, err := pool.Exec(ctx,
			"INSERT INTO sessions (session_id, account_id) VALUES ($1, $2)", sessionID, accountID,
		); err != nil {
			t.Fatalf("seeding session %q for account %d: %v", sessionID, accountID, err)
		}
	}
	return accountID
}

// SeedHistoryReport inserts one report filed from sessionID that expires at
// expiresAt and returns its id. The location is Bengaluru, created_at is
// eight hours before expiresAt, and the shelter fields are null.
func SeedHistoryReport(t *testing.T, pool *pgxpool.Pool, sessionID, severity, category string, expiresAt time.Time) int64 {
	t.Helper()

	lat, lon := 12.9716, 77.5946
	gh := geohash.EncodeWithPrecision(lat, lon, geohashPrecision)

	var id int64
	err := pool.QueryRow(
		context.Background(),
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash,
			 shelter_capacity_status, shelter_headcount, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, $8, $9)
		 RETURNING id`,
		sessionID, category, severity, "seeded history report",
		lat, lon, gh,
		expiresAt.Add(-8*time.Hour), expiresAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seeding history report for session %q: %v", sessionID, err)
	}
	return id
}

// SeedReporterHistory bulk seeds accounts, sessions and reports for the
// index and cap test, and returns the account ids in ascending order. It uses
// one INSERT for the accounts and one CopyFrom each for the sessions and the
// reports, with no per row round trips.
//
// Report j (0 based) of session s of every account expires at now plus 48
// hours, minus j times 36 hours, minus s minutes, and was created 8 hours
// earlier. Severity cycles low, medium, critical and category cycles through
// the seed categories. At the default sizes the derivation behind the
// store test's 20,000 rows is: j = 0 and 1 are still in flight (48 and 12
// hours ahead). j = 2 to 61 are 60 expired reports inside a 90 day window
// ending at now; the oldest, j = 61, is 89.5 days old plus at most 2
// minutes, which leaves 12 hours of slack. j = 62 to 79 are older than the
// window. Each account therefore has 180 in window reports, the production
// read cap of 100 keeps 100, and 200 accounts give 20,000 rows. The jitter
// uses the session's ordinal within its account (at most 2 minutes), never a
// global ordinal, because a global ordinal reaches about 10 hours and would
// eat that slack.
func SeedReporterHistory(t *testing.T, pool *pgxpool.Pool, accounts, sessionsPerAccount, reportsPerSession int, now time.Time) []int64 {
	t.Helper()

	ctx := context.Background()

	idRows, err := pool.Query(ctx,
		"INSERT INTO accounts (email) SELECT 'history-seed-' || g || '@example.com' FROM generate_series(1, $1::int) AS g RETURNING id",
		accounts)
	if err != nil {
		t.Fatalf("seeding %d history accounts: %v", accounts, err)
	}
	var accountIDs []int64
	for idRows.Next() {
		var id int64
		if err := idRows.Scan(&id); err != nil {
			idRows.Close()
			t.Fatalf("scanning history account id: %v", err)
		}
		accountIDs = append(accountIDs, id)
	}
	idRows.Close()
	if err := idRows.Err(); err != nil {
		t.Fatalf("reading history account ids: %v", err)
	}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })
	if len(accountIDs) != accounts {
		t.Fatalf("seeded %d history accounts, want %d", len(accountIDs), accounts)
	}

	sessionRows := make([][]any, 0, accounts*sessionsPerAccount)
	for a, accountID := range accountIDs {
		for s := 0; s < sessionsPerAccount; s++ {
			sessionRows = append(sessionRows, []any{fmt.Sprintf("history-seed-%d-%d", a, s), accountID})
		}
	}
	sessionCount, err := pool.CopyFrom(ctx, pgx.Identifier{"sessions"},
		[]string{"session_id", "account_id"}, pgx.CopyFromRows(sessionRows))
	if err != nil {
		t.Fatalf("seeding history sessions via CopyFrom: %v", err)
	}
	if int(sessionCount) != len(sessionRows) {
		t.Fatalf("CopyFrom inserted %d sessions, want %d", sessionCount, len(sessionRows))
	}

	lat, lon := 12.9716, 77.5946
	gh := geohash.EncodeWithPrecision(lat, lon, geohashPrecision)
	reportRows := make([][]any, 0, accounts*sessionsPerAccount*reportsPerSession)
	for a := 0; a < accounts; a++ {
		for s := 0; s < sessionsPerAccount; s++ {
			sessionID := fmt.Sprintf("history-seed-%d-%d", a, s)
			for j := 0; j < reportsPerSession; j++ {
				expires := now.Add(48*time.Hour - time.Duration(j)*36*time.Hour - time.Duration(s)*time.Minute)
				reportRows = append(reportRows, []any{
					sessionID,
					seedCategories[j%len(seedCategories)],
					seedSeverities[j%3],
					"seeded history report",
					lat, lon, gh,
					nil, nil,
					expires.Add(-8 * time.Hour),
					expires,
				})
			}
		}
	}
	reportCount, err := pool.CopyFrom(ctx, pgx.Identifier{"reports"},
		[]string{
			"session_id", "category", "severity", "description",
			"latitude", "longitude", "geohash",
			"shelter_capacity_status", "shelter_headcount",
			"created_at", "expires_at",
		},
		pgx.CopyFromRows(reportRows))
	if err != nil {
		t.Fatalf("seeding history reports via CopyFrom: %v", err)
	}
	if want := accounts * sessionsPerAccount * reportsPerSession; int(reportCount) != want {
		t.Fatalf("CopyFrom inserted %d history reports, want %d", reportCount, want)
	}

	return accountIDs
}
