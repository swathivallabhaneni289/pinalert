package service_test

// This file pins backlog 999.12 on a real database: a confirmation from a new
// spot pushes a report's fade time back, up to a cap, in the same statement that
// writes the vote (InsertVoteExtending in internal/store/queries/extend.sql).
// The tests need DATABASE_URL (testutil.NewTestDB skips without it).
//
// Amendment A1 (2026-10-30): a resolved or finished report is never extended,
// and CastVote decides that in Go before the write.

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	q64Lat = 12.9716
	q64Lon = 77.5946
)

// q64Cell is the voter cell the service computes for a point.
func q64Cell(lat, lon float64) string {
	return geohash.EncodeWithPrecision(lat, lon, 7)
}

// q64Spot returns the centre of the i-th cell of a small grid around the report
// point. The grid steps are larger than a cell, and the test checks every claim
// of "new cell" and "same cell" at runtime.
func q64Spot(i int) (float64, float64) {
	row, col := i/3, i%3
	lat := q64Lat + 0.0015*float64(row)
	lon := q64Lon + 0.0016*float64(col)
	cLat, cLon := geohash.DecodeCenter(q64Cell(lat, lon))
	return cLat, cLon
}

// q64Spots checks that the first n grid cells are distinct and returns their
// centres.
func q64Spots(t *testing.T, n int) [][2]float64 {
	t.Helper()
	seen := map[string]int{}
	out := make([][2]float64, n)
	for i := 0; i < n; i++ {
		lat, lon := q64Spot(i)
		cell := q64Cell(lat, lon)
		if j, dup := seen[cell]; dup {
			t.Fatalf("spots %d and %d share cell %s", j, i, cell)
		}
		seen[cell] = i
		out[i] = [2]float64{lat, lon}
	}
	return out
}

// q64Near is a point a few metres from a cell centre that must still sit in the
// same cell; the check is made at runtime.
func q64Near(t *testing.T, lat, lon float64) (float64, float64) {
	t.Helper()
	nLat, nLon := lat+0.00002, lon+0.00002
	if q64Cell(nLat, nLon) != q64Cell(lat, lon) {
		t.Fatalf("point (%v,%v) left the cell %s", nLat, nLon, q64Cell(lat, lon))
	}
	return nLat, nLon
}

func q64Report(t *testing.T, pool *pgxpool.Pool, session, severity string, createdAgo, expiresIn time.Duration) int64 {
	t.Helper()
	now := time.Now()
	return testutil.InsertStoredReport(t, pool, testutil.StoredReport{
		SessionID:   session,
		Category:    "flood",
		Severity:    severity,
		Description: "Water over the road near the bridge",
		Latitude:    q64Lat,
		Longitude:   q64Lon,
		CreatedAt:   now.Add(-createdAgo),
		ExpiresAt:   now.Add(expiresIn),
	})
}

func q64Times(t *testing.T, pool *pgxpool.Pool, reportID int64) (created, expires time.Time) {
	t.Helper()
	if err := pool.QueryRow(context.Background(),
		"SELECT created_at, expires_at FROM reports WHERE id = $1", reportID).Scan(&created, &expires); err != nil {
		t.Fatalf("reading report %d times: %v", reportID, err)
	}
	return created, expires
}

func q64VoteTime(t *testing.T, pool *pgxpool.Pool, reportID, accountID int64) time.Time {
	t.Helper()
	var at time.Time
	if err := pool.QueryRow(context.Background(),
		"SELECT created_at FROM votes WHERE report_id = $1 AND account_id = $2 ORDER BY id DESC LIMIT 1",
		reportID, accountID).Scan(&at); err != nil {
		t.Fatalf("reading newest vote of account %d on report %d: %v", accountID, reportID, err)
	}
	return at
}

func q64Cast(t *testing.T, svc *service.VotingService, in service.CastVoteInput) service.CastVoteResult {
	t.Helper()
	res, err := svc.CastVote(context.Background(), in)
	if err != nil {
		t.Fatalf("CastVote report %d account %d %s/%s: %v", in.ReportID, in.AccountID, in.Kind, in.Value, err)
	}
	return res
}

func q64Confirm(report, account int64, lat, lon float64) service.CastVoteInput {
	return service.CastVoteInput{ReportID: report, AccountID: account, Kind: service.VoteKindContent, Value: service.VoteConfirm, Latitude: lat, Longitude: lon}
}

// q64WantExtends fails unless expires_at moved to exactly the rule's value, and
// that value is later than before.
func q64WantExtends(t *testing.T, name string, pool *pgxpool.Pool, reportID, accountID int64, sev service.Severity, before time.Time, res service.CastVoteResult) time.Time {
	t.Helper()
	created, after := q64Times(t, pool, reportID)
	voteAt := q64VoteTime(t, pool, reportID, accountID)
	want := service.ExtendedExpiry(created, before, voteAt, service.ExpiryDuration(sev), service.ExpiryCap(sev))
	if !after.Equal(want) || !after.After(before) {
		t.Fatalf("%s: before %v, vote at %v, after %v, want %v and later than before", name, before, voteAt, after, want)
	}
	if !res.ExpiresAt.Equal(after) {
		t.Fatalf("%s: CastVoteResult.ExpiresAt %v, stored %v", name, res.ExpiresAt, after)
	}
	return after
}

// q64WantUnchanged fails unless expires_at is exactly what it was.
func q64WantUnchanged(t *testing.T, name string, pool *pgxpool.Pool, reportID, accountID int64, before time.Time, res service.CastVoteResult) {
	t.Helper()
	_, after := q64Times(t, pool, reportID)
	if !after.Equal(before) {
		t.Fatalf("%s: vote at %v moved expires_at from %v to %v, want unchanged", name, q64VoteTime(t, pool, reportID, accountID), before, after)
	}
	if !res.ExpiresAt.Equal(before) {
		t.Fatalf("%s: CastVoteResult.ExpiresAt %v, stored %v", name, res.ExpiresAt, before)
	}
}

func TestConfirmFromNewSpotExtendsExpiry(t *testing.T) {
	pool := testutil.NewTestDB(t)
	svc := service.NewVotingService(sqlcgen.New(pool))
	spots := q64Spots(t, 8)

	reporter := testutil.SeedReporterAccount(t, pool, "q64-reporter@example.com", "q64-rep-sess")
	seed := func(name string) int64 {
		return testutil.SeedReporterAccount(t, pool, fmt.Sprintf("q64-%s@example.com", name))
	}
	a, b, d, e, f, g := seed("a"), seed("b"), seed("d"), seed("e"), seed("f"), seed("g")

	r1 := q64Report(t, pool, "q64-rep-sess", "medium", 2*time.Hour, 6*time.Hour)
	_, expires := q64Times(t, pool, r1)
	sev := service.SeverityMedium

	t.Run("a confirmation from a new spot extends", func(t *testing.T) {
		res := q64Cast(t, svc, q64Confirm(r1, a, spots[0][0], spots[0][1]))
		expires = q64WantExtends(t, "A confirms from PA", pool, r1, a, sev, expires, res)
		voteAt := q64VoteTime(t, pool, r1, a)
		if !expires.Equal(voteAt.Add(8 * time.Hour)) {
			t.Fatalf("expires_at %v, want vote time %v plus 8h", expires, voteAt)
		}
	})

	t.Run("a second person at the same spot does not extend again", func(t *testing.T) {
		lat, lon := q64Near(t, spots[0][0], spots[0][1])
		res := q64Cast(t, svc, q64Confirm(r1, b, lat, lon))
		q64WantUnchanged(t, "B confirms from PA's cell", pool, r1, b, expires, res)
	})

	t.Run("the same person from a new spot does not extend again", func(t *testing.T) {
		res := q64Cast(t, svc, q64Confirm(r1, a, spots[1][0], spots[1][1]))
		q64WantUnchanged(t, "A confirms from PB", pool, r1, a, expires, res)
	})

	t.Run("a dispute never extends", func(t *testing.T) {
		res := q64Cast(t, svc, service.CastVoteInput{ReportID: r1, AccountID: d, Kind: service.VoteKindContent, Value: service.VoteDispute, Latitude: spots[2][0], Longitude: spots[2][1]})
		q64WantUnchanged(t, "D disputes from PC", pool, r1, d, expires, res)
	})

	t.Run("taking a confirmation back never shortens", func(t *testing.T) {
		res := q64Cast(t, svc, service.CastVoteInput{ReportID: r1, AccountID: a, Kind: service.VoteKindContent, Value: service.VoteDispute, Latitude: spots[0][0], Longitude: spots[0][1]})
		q64WantUnchanged(t, "A switches to dispute", pool, r1, a, expires, res)
	})

	t.Run("a second account in a taken back spot does not extend", func(t *testing.T) {
		lat, lon := q64Near(t, spots[0][0], spots[0][1])
		res := q64Cast(t, svc, q64Confirm(r1, e, lat, lon))
		q64WantUnchanged(t, "E confirms from PA's cell", pool, r1, e, expires, res)
	})

	t.Run("switching back to confirm does not extend", func(t *testing.T) {
		res := q64Cast(t, svc, q64Confirm(r1, a, spots[0][0], spots[0][1]))
		q64WantUnchanged(t, "A confirms again", pool, r1, a, expires, res)
	})

	t.Run("resolve, reopen and take back never extend", func(t *testing.T) {
		res := q64Cast(t, svc, service.CastVoteInput{ReportID: r1, AccountID: f, Kind: service.VoteKindResolution, Value: service.VoteResolve, Latitude: spots[3][0], Longitude: spots[3][1]})
		q64WantUnchanged(t, "F resolves from a new cell", pool, r1, f, expires, res)
		res = q64Cast(t, svc, service.CastVoteInput{ReportID: r1, AccountID: f, Kind: service.VoteKindResolution, Value: service.VoteUnresolve, Latitude: spots[3][0], Longitude: spots[3][1]})
		q64WantUnchanged(t, "F takes the resolve back", pool, r1, f, expires, res)
		res = q64Cast(t, svc, service.CastVoteInput{ReportID: r1, AccountID: reporter, Kind: service.VoteKindResolution, Value: service.VoteResolve, LocationMissing: true})
		q64WantUnchanged(t, "the reporter resolves", pool, r1, reporter, expires, res)
		res = q64Cast(t, svc, service.CastVoteInput{ReportID: r1, AccountID: reporter, Kind: service.VoteKindResolution, Value: service.VoteReopen, LocationMissing: true})
		q64WantUnchanged(t, "the reporter reopens", pool, r1, reporter, expires, res)
	})

	t.Run("a later new spot extends from its own time", func(t *testing.T) {
		time.Sleep(50 * time.Millisecond)
		res := q64Cast(t, svc, q64Confirm(r1, g, spots[4][0], spots[4][1]))
		expires = q64WantExtends(t, "G confirms from a new cell", pool, r1, g, sev, expires, res)
		voteAt := q64VoteTime(t, pool, r1, g)
		if !expires.Equal(voteAt.Add(8 * time.Hour)) {
			t.Fatalf("expires_at %v, want G's vote time %v plus 8h", expires, voteAt)
		}
	})

	t.Run("capped at 24 hours after posting", func(t *testing.T) {
		r := q64Report(t, pool, "", "medium", 20*time.Hour, time.Hour)
		h1, h2 := seed("cap1"), seed("cap2")
		_, before := q64Times(t, pool, r)
		res := q64Cast(t, svc, q64Confirm(r, h1, spots[0][0], spots[0][1]))
		after := q64WantExtends(t, "first new spot near the cap", pool, r, h1, service.SeverityMedium, before, res)
		created, _ := q64Times(t, pool, r)
		if !after.Equal(created.Add(24 * time.Hour)) {
			t.Fatalf("expires_at %v, want created %v plus 24h", after, created)
		}
		res = q64Cast(t, svc, q64Confirm(r, h2, spots[1][0], spots[1][1]))
		q64WantUnchanged(t, "second new spot at the cap", pool, r, h2, after, res)
	})

	t.Run("a Critical report is capped at 48 hours", func(t *testing.T) {
		r := q64Report(t, pool, "", "critical", 44*time.Hour, time.Hour)
		h := seed("crit-cap")
		_, before := q64Times(t, pool, r)
		res := q64Cast(t, svc, q64Confirm(r, h, spots[0][0], spots[0][1]))
		after := q64WantExtends(t, "critical near the cap", pool, r, h, service.SeverityCritical, before, res)
		created, _ := q64Times(t, pool, r)
		if !after.Equal(created.Add(48 * time.Hour)) {
			t.Fatalf("expires_at %v, want created %v plus 48h", after, created)
		}
	})

	t.Run("a Critical confirmation adds 24 hours", func(t *testing.T) {
		r := q64Report(t, pool, "", "critical", time.Hour, 23*time.Hour)
		h := seed("crit-add")
		_, before := q64Times(t, pool, r)
		res := q64Cast(t, svc, q64Confirm(r, h, spots[0][0], spots[0][1]))
		after := q64WantExtends(t, "critical adds a day", pool, r, h, service.SeverityCritical, before, res)
		voteAt := q64VoteTime(t, pool, r, h)
		if !after.Equal(voteAt.Add(24 * time.Hour)) {
			t.Fatalf("expires_at %v, want vote time %v plus 24h", after, voteAt)
		}
	})

	t.Run("never earlier than the current fade time", func(t *testing.T) {
		now := time.Now()
		r := testutil.InsertStoredReport(t, pool, testutil.StoredReport{
			Category: "flood", Severity: "medium", Description: "Water over the road near the bridge",
			Latitude: q64Lat, Longitude: q64Lon,
			CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Hour).Add(20 * time.Hour),
		})
		h := seed("never-earlier")
		_, before := q64Times(t, pool, r)
		res := q64Cast(t, svc, q64Confirm(r, h, spots[0][0], spots[0][1]))
		q64WantUnchanged(t, "new spot while the fade time is already later", pool, r, h, before, res)
	})

	t.Run("the reporter's own rows never use up a spot", func(t *testing.T) {
		r := q64Report(t, pool, "q64-rep-sess", "medium", time.Hour, 3*time.Hour)
		h := seed("stranger")
		testutil.MustExec(t, pool,
			"INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, 'content', 'confirm', $3)",
			r, reporter, q64Cell(spots[0][0], spots[0][1]))
		_, before := q64Times(t, pool, r)
		res := q64Cast(t, svc, q64Confirm(r, h, spots[0][0], spots[0][1]))
		q64WantExtends(t, "stranger in the reporter's old cell", pool, r, h, service.SeverityMedium, before, res)
	})

	t.Run("a resolved report is never extended, and extends again once reopened", func(t *testing.T) {
		r := q64Report(t, pool, "q64-rep-sess", "medium", time.Hour, 3*time.Hour)
		h, i := seed("resolved-h"), seed("resolved-i")
		_, before := q64Times(t, pool, r)
		if res := q64Cast(t, svc, service.CastVoteInput{ReportID: r, AccountID: reporter, Kind: service.VoteKindResolution, Value: service.VoteResolve, LocationMissing: true}); res.Visibility != service.VisibilityRetracted {
			t.Fatalf("after the reporter resolves, visibility %s, want retracted", res.Visibility)
		}
		res := q64Cast(t, svc, q64Confirm(r, h, spots[0][0], spots[0][1]))
		if res.Visibility != service.VisibilityRetracted {
			t.Fatalf("after a confirm on a resolved report, visibility %s, want retracted", res.Visibility)
		}
		q64WantUnchanged(t, "H confirms a resolved report from a new spot", pool, r, h, before, res)

		q64Cast(t, svc, service.CastVoteInput{ReportID: r, AccountID: reporter, Kind: service.VoteKindResolution, Value: service.VoteReopen, LocationMissing: true})
		res = q64Cast(t, svc, q64Confirm(r, i, spots[1][0], spots[1][1]))
		q64WantExtends(t, "I confirms a reopened report from a new spot", pool, r, i, service.SeverityMedium, before, res)
	})
}

func TestConfirmExtensionUnderConcurrentVotes(t *testing.T) {
	pool := testutil.NewTestDB(t)
	svc := service.NewVotingService(sqlcgen.New(pool))
	spots := q64Spots(t, 8)

	r := q64Report(t, pool, "", "medium", 2*time.Hour, 6*time.Hour)
	accounts := make([]int64, 8)
	for i := range accounts {
		accounts[i] = testutil.SeedReporterAccount(t, pool, fmt.Sprintf("q64-race-%d@example.com", i))
	}

	start := make(chan struct{})
	errs := make(chan error, len(accounts))
	var wg sync.WaitGroup
	for i := range accounts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.CastVote(context.Background(), q64Confirm(r, accounts[i], spots[i][0], spots[i][1]))
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a concurrent CastVote failed: %v", err)
		}
	}

	var votes int
	var latest time.Time
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*), max(created_at) FROM votes WHERE report_id = $1", r).Scan(&votes, &latest); err != nil {
		t.Fatalf("reading the votes: %v", err)
	}
	if votes != len(accounts) {
		t.Fatalf("vote rows %d, want %d", votes, len(accounts))
	}
	_, after := q64Times(t, pool, r)
	if want := latest.Add(8 * time.Hour); !after.Equal(want) {
		t.Fatalf("expires_at %v, want the latest of the eight vote times %v plus 8h = %v (an extension was lost)", after, latest, want)
	}
}
