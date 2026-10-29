package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// neverConnect fails the test if run ever tries to open a connection: the
// refusals must happen before any connection (T-09-92).
func neverConnect(t *testing.T) connectFunc {
	t.Helper()
	return func(ctx context.Context, dsn, wantDB string) (*pgxpool.Pool, error) {
		t.Fatalf("run connected to %q although it should have refused first", wantDB)
		return nil, nil
	}
}

// runRefused runs the command with args and checks that it exited 1 without
// connecting, and returns what it printed on stderr.
func runRefused(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, neverConnect(t), &out, &errOut)
	if code != 1 {
		t.Fatalf("run(%v) exit code = %d, want 1; stderr: %s", args, code, errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("run(%v) printed on stdout before refusing: %q", args, out.String())
	}
	return errOut.String()
}

func TestSeedRefusesProtectedDatabases(t *testing.T) {
	for _, name := range []string{"pinalert", "pinalert_test", "p31_4_uat", "p34_uat"} {
		for _, spelling := range []string{
			"postgres://localhost:5432/" + name + "?sslmode=disable",
			"postgres://localhost:5432/" + strings.ToUpper(name) + "?sslmode=disable",
			// A dbname value in the query string overrides the path in the driver.
			"postgres://localhost:5432/p9_innocent?sslmode=disable&dbname=" + name,
			// The keyword form.
			"host=localhost dbname=" + name + " sslmode=disable",
		} {
			t.Run(name+" "+spelling, func(t *testing.T) {
				msg := runRefused(t, "-db", spelling)
				if !strings.Contains(strings.ToLower(msg), name) {
					t.Errorf("the refusal does not name %q: %q", name, msg)
				}
			})
		}
	}
}

func TestSeedRefusesNonScratchNames(t *testing.T) {
	t.Run("names", func(t *testing.T) {
		for _, name := range []string{"myapp", "p9", "p10_x", "xp9_a", "postgres", "P9_upper_prefix"} {
			runRefused(t, "-db", "postgres://localhost:5432/"+name+"?sslmode=disable")
		}
	})
	t.Run("empty path falls back to PGDATABASE", func(t *testing.T) {
		t.Setenv("PGDATABASE", "pinalert_test")
		runRefused(t, "-db", "postgres://localhost:5432?sslmode=disable")
		t.Setenv("PGDATABASE", "somethingelse")
		runRefused(t, "-db", "postgres://localhost:5432?sslmode=disable")
	})
	t.Run("a scratch name on a server that is not this machine", func(t *testing.T) {
		msg := runRefused(t, "-db", "postgres://db.example.com:5432/p9_view?sslmode=require")
		if !strings.Contains(msg, "not on this machine") {
			t.Errorf("refusal = %q", msg)
		}
	})
	t.Run("a garbled string says nothing about it", func(t *testing.T) {
		msg := runRefused(t, "-db", "postgres://user:topsecret@@@/::bad")
		if strings.Contains(msg, "topsecret") {
			t.Errorf("the refusal quotes the connection string: %q", msg)
		}
	})
	t.Run("a viewer address that is not made up", func(t *testing.T) {
		const real = "someone.real@gmail.com"
		msg := runRefused(t, "-db", "postgres://localhost:5432/p9_view?sslmode=disable", "-viewer-email", real)
		if strings.Contains(msg, real) || strings.Contains(msg, "gmail") {
			t.Errorf("the refusal echoes the address: %q", msg)
		}
		if !strings.Contains(msg, "@example.com") {
			t.Errorf("the refusal does not say what to use: %q", msg)
		}
		runRefused(t, "-db", "postgres://localhost:5432/p9_view?sslmode=disable", "-viewer-email", "@example.com")
	})
	t.Run("usage errors exit 2", func(t *testing.T) {
		var out, errOut bytes.Buffer
		if code := run(nil, neverConnect(t), &out, &errOut); code != 2 {
			t.Errorf("no -db: exit code = %d, want 2", code)
		}
		if code := run([]string{"-db", "postgres://localhost/p9_x", "stray"}, neverConnect(t), &out, &errOut); code != 2 {
			t.Errorf("stray argument: exit code = %d, want 2", code)
		}
	})
	t.Run("scratch names pass the check", func(t *testing.T) {
		for _, dsn := range []string{
			"postgres://localhost:5432/p9_view?sslmode=disable",
			"postgres://127.0.0.1:5432/p9_28_a?sslmode=disable",
			"postgres://localhost/p9_x",
		} {
			if _, err := checkTarget(dsn); err != nil {
				t.Errorf("checkTarget(%q) = %v, want nil", dsn, err)
			}
		}
	})
}

// TestSeedBuildsViewingData runs the seed on the shared test database (the
// pool is migrated and truncated by NewTestDB). It calls seed and run with a
// fake connect, so it works on whatever database name CI uses, while run's own
// refusal of that name is covered above.
func TestSeedBuildsViewingData(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	q := sqlcgen.New(pool)
	ratings := service.NewRatingService(q)
	people := service.NewPeopleService(q, service.WithPeopleSignals(ratings), service.WithPeopleRatings(ratings))

	var out bytes.Buffer
	if err := seed(ctx, pool, "", &out); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, want := range []string{"/u/ravi.k", "/u/anita.r", "/u/new.reporter", "/u/private.one"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the output does not list %s: %q", want, out.String())
		}
	}
	if strings.Contains(out.String(), "@") {
		t.Errorf("the output prints an email address: %q", out.String())
	}

	idOf := func(username string) int64 {
		id, err := q.GetAccountIDByUsername(ctx, username)
		if err != nil {
			t.Fatalf("account %s: %v", username, err)
		}
		return id
	}
	viewerID := idOf("new.reporter")

	ravi, err := people.Profile(ctx, viewerID, "ravi.k")
	if err != nil {
		t.Fatalf("Profile ravi.k: %v", err)
	}
	if !ravi.Reliable {
		t.Errorf("ravi.k is not Reliable: tag %q", ravi.TagWord)
	}
	if !strings.Contains(ravi.StarsFromRatings, "6 people") || ravi.StarsFromReports == "" {
		t.Errorf("ravi.k parts = %q and %q, want a ratings part from 6 people", ravi.StarsFromReports, ravi.StarsFromRatings)
	}
	if ravi.FollowersCount != 3 {
		t.Errorf("ravi.k followers = %d, want 3 (anita.r, new.reporter, private.one)", ravi.FollowersCount)
	}

	anita, err := people.Profile(ctx, viewerID, "anita.r")
	if err != nil {
		t.Fatalf("Profile anita.r: %v", err)
	}
	if !anita.Reliable || anita.StarsFromRatings != "" {
		t.Errorf("anita.r: Reliable = %v, ratings part = %q; want Reliable with no ratings part", anita.Reliable, anita.StarsFromRatings)
	}

	fresh, err := people.Profile(ctx, idOf("ravi.k"), "new.reporter")
	if err != nil {
		t.Fatalf("Profile new.reporter: %v", err)
	}
	if fresh.Reliable || fresh.StarsNumber != "" {
		t.Errorf("new.reporter should be New with no stars: tag %q stars %q", fresh.TagWord, fresh.StarsNumber)
	}

	if _, err := people.Profile(ctx, viewerID, "private.one"); err != service.ErrPersonNotFound {
		t.Errorf("Profile private.one = %v, want ErrPersonNotFound", err)
	}

	// The raters really sit in six different places.
	var cells int
	if err := pool.QueryRow(ctx,
		`SELECT count(DISTINCT v.geohash_cell)
		 FROM votes v JOIN accounts a ON a.id = v.account_id
		 WHERE a.username LIKE 'rater.%'`).Scan(&cells); err != nil {
		t.Fatalf("counting rater cells: %v", err)
	}
	if cells != 6 {
		t.Errorf("rater cells = %d, want 6", cells)
	}

	// A second run changes nothing and does not fail.
	var accountsBefore, votesBefore int
	count := func() (int, int) {
		var a, v int
		if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM accounts), (SELECT count(*) FROM votes)").Scan(&a, &v); err != nil {
			t.Fatalf("counting rows: %v", err)
		}
		return a, v
	}
	accountsBefore, votesBefore = count()
	if err := seed(ctx, pool, "", &bytes.Buffer{}); err != nil {
		t.Fatalf("second seed run: %v", err)
	}
	if a, v := count(); a != accountsBefore || v != votesBefore {
		t.Errorf("a second run changed the rows: accounts %d to %d, votes %d to %d", accountsBefore, a, votesBefore, v)
	}

	// The viewer part: an account made through sign up (here a plain row with a
	// made up address), then the seed again with that address.
	const viewerEmail = "seed.viewer@example.com"
	if err := seed(ctx, pool, viewerEmail, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "sign up") {
		t.Errorf("seed for an address with no account = %v, want a sign up message", err)
	} else if strings.Contains(err.Error(), viewerEmail) {
		t.Errorf("the error echoes the address: %v", err)
	}
	vID := testutil.SeedReporterAccount(t, pool, viewerEmail)
	var viewerOut bytes.Buffer
	for i := 0; i < 2; i++ {
		viewerOut.Reset()
		if err := seed(ctx, pool, viewerEmail, &viewerOut); err != nil {
			t.Fatalf("seed with the viewer address (run %d): %v", i+1, err)
		}
	}
	if strings.Contains(viewerOut.String(), "@") {
		t.Errorf("the output prints an email address: %q", viewerOut.String())
	}
	var viewerVotes int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM votes WHERE account_id = $1", vID).Scan(&viewerVotes); err != nil {
		t.Fatalf("counting viewer votes: %v", err)
	}
	if viewerVotes != 1 {
		t.Errorf("viewer votes after two runs = %d, want 1", viewerVotes)
	}
	var viewerCells int
	if err := pool.QueryRow(ctx,
		`SELECT count(DISTINCT v.geohash_cell) FROM votes v JOIN reports r ON r.id = v.report_id
		 WHERE r.id = (SELECT report_id FROM votes WHERE account_id = $1)`, vID).Scan(&viewerCells); err != nil {
		t.Fatalf("counting cells on the viewer's report: %v", err)
	}
	if viewerCells != 7 {
		t.Errorf("cells on ravi.k's live report with the viewer = %d, want 7 (six raters and the viewer)", viewerCells)
	}

	onRavi, err := people.Profile(ctx, vID, "ravi.k")
	if err != nil {
		t.Fatalf("Profile ravi.k as the viewer: %v", err)
	}
	if !onRavi.MayRate || onRavi.YourRating != 0 {
		t.Errorf("the viewer on ravi.k: MayRate = %v, YourRating = %d; want true and 0", onRavi.MayRate, onRavi.YourRating)
	}
	onAnita, err := people.Profile(ctx, vID, "anita.r")
	if err != nil {
		t.Fatalf("Profile anita.r as the viewer: %v", err)
	}
	if !onAnita.Following || onAnita.Notify {
		t.Errorf("the viewer on anita.r: Following = %v, Notify = %v; want true and false", onAnita.Following, onAnita.Notify)
	}

	// run end to end through a fake connect, on the same pool. The name in -db
	// is a scratch name; the pool is the test database.
	var runOut, runErr bytes.Buffer
	if err := checkMigrated(ctx, pool); err != nil {
		t.Errorf("checkMigrated on a migrated database: %v", err)
	}
	if code := run([]string{"-db", "postgres://localhost:5432/p9_runcheck?sslmode=disable", "-viewer-email", viewerEmail},
		fakePool(pool, t), &runOut, &runErr); code != 0 {
		t.Errorf("run exit code = %d, stderr: %s", code, runErr.String())
	}
}

// fakePool returns a connect that hands run the shared test pool. run closes
// the pool it is given, so the wrapper opens a second pool on the same
// database.
func fakePool(shared *pgxpool.Pool, t *testing.T) connectFunc {
	t.Helper()
	return func(ctx context.Context, dsn, wantDB string) (*pgxpool.Pool, error) {
		return pgxpool.NewWithConfig(ctx, shared.Config())
	}
}
