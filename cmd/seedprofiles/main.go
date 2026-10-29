// Command seedprofiles fills a scratch database with made up people and
// reports around Benz Circle, Vijayawada, so the owner can look at the Phase 9
// profile screens in one viewing session. It is for a scratch copy only.
//
// Usage:
//
//	DATABASE_URL=postgres://localhost:5432/p9_view?sslmode=disable go run ./cmd/migrate
//	go run ./cmd/seedprofiles -db postgres://localhost:5432/p9_view?sslmode=disable
//	go run ./cmd/seedprofiles -db postgres://localhost:5432/p9_view?sslmode=disable -viewer-email you@example.com
//
// The database must already be migrated (cmd/migrate). The seed refuses, before
// it connects, the databases pinalert, pinalert_test, p31_4_uat and p34_uat,
// any database whose name does not start with p9_, and any server that is not
// on this machine. Run it a second time with -viewer-email once the viewer has
// signed up on the test copy: the base data is left as it is and only the
// viewer's part is added.
//
// The live reports end 3 to 5 hours after the seed runs, so run it just before
// the viewing. For a later viewing, drop the scratch database, make it again,
// migrate and seed again; a second run never makes new live reports.
//
// Everyone it makes is made up. Every seeded account has the address
// <username>@example.com (for example ravi.k@example.com) and one shared
// password, printed at the end, so the owner can sign in as the reporter and
// post during the follow check. -viewer-email must be your own made up
// example.com address that you signed up with on the test copy (for example
// viewer@example.com). Never use a real email address. The address is never
// printed or logged.
//
// What it makes:
//
//	ravi.k        public, Reliable reporter, six raters in six different places,
//	              two finished reports, one live report
//	anita.r       public, Reliable reporter, two raters (stars show from five), one live report
//	new.reporter  public, New reporter, one live report
//	private.one   profile switched off, one live report
//	follows between them, and people who only vote or rate (voter.a, voter.b,
//	rater.one to rater.six)
//
// It writes accounts, sessions, reports and votes directly, because there is
// no other way to place reports in the past. Everything else goes through the
// real services: the profile switch (SetProfilePublic), the ratings
// (RatingService.Rate), the follows (PeopleService.Follow) and the stars that
// come out of the real arithmetic. At the end it reads the four profiles back
// through PeopleService.Profile and fails unless they read as described.
//
// Exit codes: 0 done, 1 refused or failed, 2 bad usage.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/auth"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	// Benz Circle, Vijayawada.
	benzLat = 16.5062
	benzLon = 80.6480

	// reportGeohashPrecision is the precision stored on reports.geohash
	// (internal/service/report.go geohashPrecision).
	reportGeohashPrecision = 8
	// voterCellPrecision is the precision of a voter cell (votes.geohash_cell).
	voterCellPrecision = 7
	// homeCellPrecision is the precision of an account's home area cell.
	homeCellPrecision = 5

	// scratchPrefix is the only database name prefix the seed accepts.
	scratchPrefix = "p9_"

	// seedPassword is the one password of every made up account. It is a made
	// up phrase, not a secret.
	seedPassword = "wide river morning 47"

	// exampleSuffix is what a viewer address must end with.
	exampleSuffix = "@example.com"
)

// protectedDatabases are the owner's databases the seed never touches.
var protectedDatabases = []string{"pinalert", "pinalert_test", "p31_4_uat", "p34_uat"}

// errNotScratch is returned for a target the seed refuses.
type errNotScratch struct{ reason string }

func (e errNotScratch) Error() string { return e.reason }

// checkTarget decides, from the connection string alone and without
// connecting, whether the seed may write there. It reads the database name the
// way the driver will (ParseConfig applies the path, a dbname query value, the
// keyword form and the PGDATABASE fallback), so no spelling of a protected name
// slips through. It also refuses a server that is not on this machine.
func checkTarget(dsn string) (string, error) {
	if strings.TrimSpace(dsn) == "" {
		return "", errNotScratch{"-db is required"}
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		// The driver's message may quote parts of the string, so it is not shown.
		return "", errNotScratch{"-db is not a postgres connection string the driver can read"}
	}
	name := cfg.Database
	if name == "" {
		return "", errNotScratch{"-db names no database"}
	}
	lower := strings.ToLower(name)
	for _, p := range protectedDatabases {
		if lower == p {
			return "", errNotScratch{fmt.Sprintf("refusing %q: it is one of the owner's databases (%s)", name, strings.Join(protectedDatabases, ", "))}
		}
	}
	if !strings.HasPrefix(name, scratchPrefix) {
		return "", errNotScratch{fmt.Sprintf("refusing %q: a scratch database name must start with %s", name, scratchPrefix)}
	}
	hosts := []string{cfg.Host}
	for _, fb := range cfg.Fallbacks {
		hosts = append(hosts, fb.Host)
	}
	for _, h := range hosts {
		if !localHost(h) {
			return "", errNotScratch{"refusing a server that is not on this machine: the seed writes to a local scratch database only"}
		}
	}
	return name, nil
}

// localHost says whether a host value from a connection string is this machine:
// localhost, a loopback address, or a socket directory.
func localHost(h string) bool {
	switch h {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return strings.HasPrefix(h, "/")
}

// checkViewerEmail accepts only a made up example.com address (owner rule: no
// real person's details on this machine). The address is never echoed.
func checkViewerEmail(email string) error {
	e := strings.ToLower(strings.TrimSpace(email))
	if !strings.HasSuffix(e, exampleSuffix) || len(e) == len(exampleSuffix) || strings.Count(e, "@") != 1 {
		return errors.New("-viewer-email must be a made up address ending in " + exampleSuffix + ", the one you signed up with on the test copy; never use a real email address")
	}
	return nil
}

// connectFunc opens the pool for run. The default also checks which database
// the server really connected to.
type connectFunc func(ctx context.Context, dsn, wantDB string) (*pgxpool.Pool, error)

func defaultConnect(ctx context.Context, dsn, wantDB string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("could not read -db")
	}
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("could not connect to the database")
	}
	var got string
	if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&got); err != nil {
		pool.Close()
		return nil, errors.New("could not reach the database")
	}
	if got != wantDB {
		pool.Close()
		return nil, fmt.Errorf("connected to %q, not the %q that was asked for; stopping", got, wantDB)
	}
	return pool, nil
}

func main() {
	os.Exit(run(os.Args[1:], defaultConnect, os.Stdout, os.Stderr))
}

func run(args []string, connect connectFunc, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("seedprofiles", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dsn := fs.String("db", "", "postgres URL of a migrated scratch database whose name starts with p9_ (required)")
	viewer := fs.String("viewer-email", "", "made up example.com address you signed up with on the test copy (optional, never a real address)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: seedprofiles -db postgres://localhost:5432/p9_view?sslmode=disable [-viewer-email viewer@example.com]")
		fmt.Fprintln(stderr, "Fills a scratch database with made up people near Benz Circle. Every seeded account has the address")
		fmt.Fprintln(stderr, "<username>@example.com (for example ravi.k@example.com) and the one shared password printed at the end.")
		fmt.Fprintln(stderr, "-viewer-email takes the made up example.com address you signed up with on the test copy, never your real email.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "seedprofiles: unexpected arguments")
		return 2
	}
	if strings.TrimSpace(*dsn) == "" {
		fmt.Fprintln(stderr, "seedprofiles: -db is required")
		fs.Usage()
		return 2
	}
	if *viewer != "" {
		if err := checkViewerEmail(*viewer); err != nil {
			fmt.Fprintln(stderr, "seedprofiles:", err)
			return 1
		}
	}
	dbName, err := checkTarget(*dsn)
	if err != nil {
		fmt.Fprintln(stderr, "seedprofiles:", err)
		return 1
	}

	ctx := context.Background()
	pool, err := connect(ctx, *dsn, dbName)
	if err != nil {
		fmt.Fprintln(stderr, "seedprofiles:", err)
		return 1
	}
	defer pool.Close()

	if err := checkMigrated(ctx, pool); err != nil {
		fmt.Fprintln(stderr, "seedprofiles:", err)
		return 1
	}
	if err := seed(ctx, pool, *viewer, stdout); err != nil {
		fmt.Fprintln(stderr, "seedprofiles:", err)
		return 1
	}
	return 0
}

// checkMigrated says so when the database has not been through cmd/migrate.
func checkMigrated(ctx context.Context, pool *pgxpool.Pool) error {
	for _, table := range []string{"accounts", "account_follows", "profile_ratings", "person_mutes"} {
		var found *string
		if err := pool.QueryRow(ctx, "SELECT to_regclass($1)::text", "public."+table).Scan(&found); err != nil {
			return fmt.Errorf("checking the schema: %w", err)
		}
		if found == nil {
			return errors.New("the database is not migrated yet; run DATABASE_URL=<the same url> go run ./cmd/migrate first")
		}
	}
	return nil
}

// person is one made up account.
type person struct {
	username string
	name     string
	optIn    bool // turned on through SetProfilePublic
}

func (p person) email() string { return p.username + exampleSuffix }

var (
	raviK       = person{"ravi.k", "Ravi Kumar", true}
	anitaR      = person{"anita.r", "Anita Rao", true}
	newReporter = person{"new.reporter", "Kiran Babu", true}
	privateOne  = person{"private.one", "Priya Nair", false}
	voterA      = person{"voter.a", "Voter A", false}
	voterB      = person{"voter.b", "Voter B", false}
	raters      = []person{
		{"rater.one", "Rater One", false},
		{"rater.two", "Rater Two", false},
		{"rater.three", "Rater Three", false},
		{"rater.four", "Rater Four", false},
		{"rater.five", "Rater Five", false},
		{"rater.six", "Rater Six", false},
	}
	// raviStars are the six ratings of ravi.k, one per rater above. They average
	// 4.0, a little under the stars the reports alone give, so both parts of the
	// stars read differently.
	raviStars = []int{4, 4, 3, 5, 4, 4}
	// anitaStars are the two ratings of anita.r, from voter.a and voter.b. Two is
	// below the five raters the stars need, so they change nothing yet.
	anitaStars = []int{5, 4}
)

// allPeople is every made up account, in the order they are made.
func allPeople() []person {
	return append([]person{raviK, anitaR, newReporter, privateOne, voterA, voterB}, raters...)
}

// spot is a point near Benz Circle, as offsets in degrees.
type spot struct{ dLat, dLon float64 }

func (s spot) point() (float64, float64) { return benzLat + s.dLat, benzLon + s.dLon }

// historyReport is a finished report of a reporter: when it ended and what it was.
type historyReport struct {
	endedDaysAgo int
	category     string
	severity     string
	description  string
	at           spot
}

// seed writes the base data once, then (always, and safely to repeat) the
// profile switches, ratings and follows through the real services, then the
// viewer's part, then reads the profiles back. out gets usernames and paths
// only.
func seed(ctx context.Context, pool *pgxpool.Pool, viewerEmail string, out io.Writer) error {
	q := sqlcgen.New(pool)
	ratingsSvc := service.NewRatingService(q)
	people := service.NewPeopleService(q,
		service.WithPeopleSignals(ratingsSvc),
		service.WithPeopleRatings(ratingsSvc),
	)

	var seeded bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM accounts WHERE username = $1)", raviK.username,
	).Scan(&seeded); err != nil {
		return fmt.Errorf("looking for earlier seed data: %w", err)
	}
	if !seeded {
		if err := insertBase(ctx, pool); err != nil {
			return err
		}
		fmt.Fprintln(out, "Seeded the base data.")
	} else {
		fmt.Fprintln(out, "The base data is already there; leaving it as it is.")
	}

	ids, err := idsOf(ctx, q, allPeople())
	if err != nil {
		return err
	}

	// The profile switches go through the real service, which applies the
	// username and Name checks. A name it refuses stops the seed here.
	for _, p := range []person{raviK, anitaR, newReporter} {
		if err := people.SetProfilePublic(ctx, ids[p.username], true); err != nil {
			return fmt.Errorf("switching the profile of %s on: %w", p.username, err)
		}
	}

	// Ratings through RatingService.Rate: the real eligibility rule (a vote on
	// one of the reporter's reports) and the real cell counted arithmetic.
	for i, r := range raters {
		if err := ratingsSvc.Rate(ctx, ids[r.username], raviK.username, raviStars[i]); err != nil {
			return fmt.Errorf("%s rating %s: %w", r.username, raviK.username, err)
		}
	}
	for i, r := range []person{voterA, voterB} {
		if err := ratingsSvc.Rate(ctx, ids[r.username], anitaR.username, anitaStars[i]); err != nil {
			return fmt.Errorf("%s rating %s: %w", r.username, anitaR.username, err)
		}
	}

	// Follows through PeopleService.Follow: both people must be Reliable now.
	for _, f := range [][2]person{
		{anitaR, raviK},
		{newReporter, raviK},
		{newReporter, anitaR},
		{privateOne, raviK},
		{raviK, anitaR},
	} {
		if err := people.Follow(ctx, ids[f[0].username], f[1].username); err != nil {
			return fmt.Errorf("%s following %s: %w", f[0].username, f[1].username, err)
		}
	}

	if viewerEmail != "" {
		if err := viewerPart(ctx, pool, q, people, ids, viewerEmail); err != nil {
			return err
		}
		fmt.Fprintln(out, "Added the viewer's vote on a live report of ravi.k and a follow of anita.r with the bell off.")
	}

	return verify(ctx, people, ids, out)
}

// idsOf maps usernames to account ids.
func idsOf(ctx context.Context, q *sqlcgen.Queries, ps []person) (map[string]int64, error) {
	ids := make(map[string]int64, len(ps))
	for _, p := range ps {
		id, err := q.GetAccountIDByUsername(ctx, p.username)
		if err != nil {
			return nil, fmt.Errorf("finding the seeded account %s: %w", p.username, err)
		}
		ids[p.username] = id
	}
	return ids, nil
}

// insertBase writes the accounts, sessions, reports and votes in one
// transaction, so a failure leaves nothing half made.
func insertBase(ctx context.Context, pool *pgxpool.Pool) error {
	hash, err := auth.NewHasher(auth.DefaultParams).Hash(ctx, seedPassword)
	if err != nil {
		return fmt.Errorf("hashing the shared password: %w", err)
	}
	now := time.Now().UTC()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting the seed: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	s := &seeder{ctx: ctx, tx: tx, hash: hash, now: now, ids: map[string]int64{}}
	for _, p := range allPeople() {
		if err := s.account(p); err != nil {
			return err
		}
	}

	// Two confirmers from two different places back every finished report of the
	// two reliable reporters. The cells are far from the raters' cells and from
	// each other.
	cellA := cellAt(spot{-0.006, 0})
	cellB := cellAt(spot{0, 0.006})
	if cellA == cellB {
		return errors.New("internal: the two voter cells collide")
	}

	// ravi.k: four older reports build the record (beyond the 30 days a profile
	// lists, inside the 90 days the score reads), two recent finished reports are
	// the ones the profile shows, and one live report.
	raviHistory := []historyReport{
		{36, "flood", "medium", "Water over the service road near the flyover", spot{0.0021, -0.0014}},
		{44, "road_blocked", "medium", "Fallen tree across the Eluru road", spot{-0.0017, 0.0023}},
		{52, "power_outage", "low", "No power since the evening", spot{0.0030, 0.0011}},
		{61, "shelter_open", "low", "School hall open for families", spot{-0.0026, -0.0019}},
		{4, "flood", "medium", "Knee deep water under the flyover, buses turning back", spot{0.0008, -0.0006}},
		{13, "traffic", "low", "Long queue from the circle towards Bandar road", spot{-0.0011, 0.0016}},
	}
	for _, h := range raviHistory {
		if err := s.finished(raviK, h, cellA, cellB); err != nil {
			return err
		}
	}
	raviLive, err := s.live(raviK, "flood", "medium", "Water is rising on the slip road, two wheelers are stuck", raviLiveSpot, 40*time.Minute, 5*time.Hour)
	if err != nil {
		return err
	}

	// anita.r: six older reports, none inside the 30 days, and one live report.
	anitaHistory := []historyReport{
		{33, "road_blocked", "low", "Barricade on the left lane of the circle", spot{0.0012, 0.0019}},
		{38, "flood", "low", "Drain overflowing at the bus stop", spot{-0.0009, -0.0021}},
		{45, "power_outage", "medium", "Transformer down, street dark", spot{0.0025, -0.0008}},
		{55, "shelter_open", "low", "Community hall has beds and water", spot{-0.0022, 0.0014}},
		{64, "traffic", "low", "Signal not working at the circle", spot{0.0006, 0.0027}},
		{72, "flood", "low", "Puddles on the approach road", spot{-0.0014, -0.0005}},
	}
	for _, h := range anitaHistory {
		if err := s.finished(anitaR, h, cellA, cellB); err != nil {
			return err
		}
	}
	if _, err := s.live(anitaR, "road_blocked", "low", "One lane closed by the circle, use the service road", spot{-0.0012, 0.0013}, 25*time.Minute, 4*time.Hour); err != nil {
		return err
	}

	if _, err := s.live(newReporter, "traffic", "low", "Slow traffic towards the railway station", spot{0.0004, 0.0021}, 15*time.Minute, 3*time.Hour); err != nil {
		return err
	}
	if _, err := s.live(privateOne, "power_outage", "low", "Power cut in the lane behind the circle", spot{-0.0019, -0.0012}, 20*time.Minute, 3*time.Hour); err != nil {
		return err
	}

	// ravi.k's six raters each confirm his live report from their own cell, one
	// step of 0.003 degrees of latitude apart (about 330 m), so they count as six
	// places under the cell rule.
	lat, lon := raviLiveSpot.point()
	seen := map[string]bool{}
	for i, r := range raters {
		cell := geohash.EncodeWithPrecision(lat+0.003*float64(i+1), lon, voterCellPrecision)
		if seen[cell] {
			return fmt.Errorf("internal: rater %s would share the cell %s", r.username, cell)
		}
		seen[cell] = true
		if err := s.vote(raviLive, r, cell, now.Add(-time.Duration(30-i)*time.Minute)); err != nil {
			return err
		}
	}
	// voter.a and voter.b confirmed anita.r's finished reports above, from two
	// different places, so they may rate her.
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing the seed: %w", err)
	}
	return nil
}

// raviLiveSpot is where ravi.k's live report sits (kept in one place so the
// raters' cells and the viewer's cell are computed from it).
var raviLiveSpot = spot{0.0015, -0.0010}

// cellAt is the precision 7 cell of a spot near Benz Circle.
func cellAt(s spot) string {
	lat, lon := s.point()
	return geohash.EncodeWithPrecision(lat, lon, voterCellPrecision)
}

// seeder carries the transaction and the accounts made so far.
type seeder struct {
	ctx  context.Context
	tx   pgx.Tx
	hash string
	now  time.Time
	ids  map[string]int64
}

// account inserts one verified, onboarded account (a password, the current
// terms, the first-time flow done, a clear username, a Name and a home area)
// and one session bound to it.
func (s *seeder) account(p person) error {
	homeCell := geohash.EncodeWithPrecision(benzLat, benzLon, homeCellPrecision)
	var id int64
	err := s.tx.QueryRow(s.ctx,
		`INSERT INTO accounts
			(email, password_hash, terms_agreed_at, terms_version, first_run_done_at,
			 username, username_screen, display_name, home_area_name, home_area_cell)
		 VALUES ($1, $2, $3, $4, $3, $5, 'clear', $6, 'Benz Circle', $7)
		 RETURNING id`,
		p.email(), s.hash, s.now, service.CurrentTermsVersion, p.username, p.name, homeCell,
	).Scan(&id)
	if err != nil {
		return fmt.Errorf("inserting the account %s: %w", p.username, err)
	}
	if _, err := s.tx.Exec(s.ctx,
		"INSERT INTO sessions (session_id, account_id) VALUES ($1, $2)", sessionOf(p), id,
	); err != nil {
		return fmt.Errorf("inserting the session of %s: %w", p.username, err)
	}
	s.ids[p.username] = id
	return nil
}

func sessionOf(p person) string { return "seed-" + p.username }

// report inserts one report of the person's session and returns its id.
func (s *seeder) report(p person, category, severity, description string, at spot, createdAt, expiresAt time.Time) (int64, error) {
	lat, lon := at.point()
	gh := geohash.EncodeWithPrecision(lat, lon, reportGeohashPrecision)
	var id int64
	err := s.tx.QueryRow(s.ctx,
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash,
			 shelter_capacity_status, shelter_headcount, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, $8, $9)
		 RETURNING id`,
		sessionOf(p), category, severity, description, lat, lon, gh, createdAt, expiresAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("inserting a report of %s: %w", p.username, err)
	}
	return id, nil
}

// vote inserts one confirm vote of a person from a voter cell.
func (s *seeder) vote(reportID int64, voter person, cell string, at time.Time) error {
	if _, err := s.tx.Exec(s.ctx,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
		 VALUES ($1, $2, 'content', 'confirm', $3, $4)`,
		reportID, s.ids[voter.username], cell, at,
	); err != nil {
		return fmt.Errorf("inserting the vote of %s: %w", voter.username, err)
	}
	return nil
}

// finished inserts a report that ended h.endedDaysAgo days ago, confirmed by
// voter.a and voter.b from two different places two hours before it ended.
func (s *seeder) finished(p person, h historyReport, cellA, cellB string) error {
	expires := s.now.Add(-time.Duration(h.endedDaysAgo) * 24 * time.Hour)
	id, err := s.report(p, h.category, h.severity, h.description, h.at, expires.Add(-8*time.Hour), expires)
	if err != nil {
		return err
	}
	votedAt := expires.Add(-2 * time.Hour)
	if err := s.vote(id, voterA, cellA, votedAt); err != nil {
		return err
	}
	return s.vote(id, voterB, cellB, votedAt)
}

// live inserts a report that started `age` ago and ends `left` from now.
func (s *seeder) live(p person, category, severity, description string, at spot, age, left time.Duration) (int64, error) {
	return s.report(p, category, severity, description, at, s.now.Add(-age), s.now.Add(left))
}

// viewerPart adds, for the account the viewer signed up with, a confirm vote on
// ravi.k's live report from a nearby cell no rater uses (so the rating control
// and the star box can be seen) and a follow of anita.r with the bell off. It
// is safe to repeat. The address is never echoed.
func viewerPart(ctx context.Context, pool *pgxpool.Pool, q *sqlcgen.Queries, people *service.PeopleService, ids map[string]int64, email string) error {
	var viewerID int64
	err := pool.QueryRow(ctx,
		"SELECT id FROM accounts WHERE lower(email) = lower($1)", strings.TrimSpace(email),
	).Scan(&viewerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("no account has that viewer address yet; sign up on the test copy with it first, then run the seed again")
	}
	if err != nil {
		return errors.New("looking up the viewer account failed")
	}
	for _, seededID := range ids {
		if seededID == viewerID {
			return errors.New("the viewer address belongs to a seeded person; use the address you signed up with")
		}
	}

	var reportID int64
	if err := pool.QueryRow(ctx,
		`SELECT r.id FROM reports r JOIN sessions s ON s.session_id = r.session_id
		 WHERE s.account_id = $1 AND r.expires_at > now() ORDER BY r.id LIMIT 1`, ids[raviK.username],
	).Scan(&reportID); err != nil {
		return fmt.Errorf("finding ravi.k's live report: %w", err)
	}
	var has bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM votes WHERE report_id = $1 AND account_id = $2)", reportID, viewerID,
	).Scan(&has); err != nil {
		return errors.New("looking for the viewer's vote failed")
	}
	if !has {
		lat, lon := raviLiveSpot.point()
		cell := geohash.EncodeWithPrecision(lat-0.003, lon, voterCellPrecision)
		if _, err := pool.Exec(ctx,
			`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell)
			 VALUES ($1, $2, 'content', 'confirm', $3)`, reportID, viewerID, cell,
		); err != nil {
			return errors.New("writing the viewer's vote failed")
		}
	}
	if err := people.Follow(ctx, viewerID, anitaR.username); err != nil {
		return fmt.Errorf("the viewer following %s: %w", anitaR.username, err)
	}
	return nil
}

// verify reads the four profiles back through PeopleService.Profile, as the new
// reporter would see them, and fails unless they read as the seed describes.
// It prints usernames, paths and the tag only.
func verify(ctx context.Context, people *service.PeopleService, ids map[string]int64, out io.Writer) error {
	viewer := ids[newReporter.username]

	ravi, err := people.Profile(ctx, viewer, raviK.username)
	if err != nil {
		return fmt.Errorf("reading back %s: %w", raviK.username, err)
	}
	if !ravi.Reliable {
		return fmt.Errorf("%s does not read as a Reliable reporter (%q); the seed did not build what it should", raviK.username, ravi.TagWord)
	}
	if !strings.Contains(ravi.StarsFromRatings, "6 people") {
		return fmt.Errorf("%s does not show ratings by 6 people (%q)", raviK.username, ravi.StarsFromRatings)
	}

	anita, err := people.Profile(ctx, viewer, anitaR.username)
	if err != nil {
		return fmt.Errorf("reading back %s: %w", anitaR.username, err)
	}
	if !anita.Reliable {
		return fmt.Errorf("%s does not read as a Reliable reporter (%q)", anitaR.username, anita.TagWord)
	}
	if anita.StarsFromRatings != "" {
		return fmt.Errorf("%s shows a ratings part with fewer than five raters", anitaR.username)
	}

	fresh, err := people.Profile(ctx, ids[raviK.username], newReporter.username)
	if err != nil {
		return fmt.Errorf("reading back %s: %w", newReporter.username, err)
	}
	if fresh.Reliable || fresh.StarsNumber != "" {
		return fmt.Errorf("%s should be a New reporter with no stars (%q)", newReporter.username, fresh.TagWord)
	}

	if _, err := people.Profile(ctx, viewer, privateOne.username); !errors.Is(err, service.ErrPersonNotFound) {
		return fmt.Errorf("%s should not be found by others (err %v)", privateOne.username, err)
	}

	fmt.Fprintf(out, "/u/%s   %s, %s stars, ratings by 6 people\n", raviK.username, ravi.TagWord, ravi.StarsNumber)
	fmt.Fprintf(out, "/u/%s  %s, %s stars, fewer than five raters\n", anitaR.username, anita.TagWord, anita.StarsNumber)
	fmt.Fprintf(out, "/u/%s  %s, no stars\n", newReporter.username, fresh.TagWord)
	fmt.Fprintf(out, "/u/%s  profile off (others get Profile not found)\n", privateOne.username)
	fmt.Fprintf(out, "Every seeded person signs in with the shared password %q (the address pattern is in -h).\n", seedPassword)
	fmt.Fprintln(out, "The live reports end 3 to 5 hours after the first seed run; for a later viewing, seed a fresh database.")
	return nil
}
