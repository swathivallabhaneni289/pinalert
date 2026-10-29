package service_test

// ratings_db_test.go proves RatingService over a real database (plan 09-13,
// D-02, D-14, D-21, D-22, D-23, G6, W1, W4): who may rate, the distinct cell
// counting, the once a day rule, removal and ignoring, the private min rule, the
// soft end of follows and their restore after a moderator removal. The tests
// skip without DATABASE_URL (NewTestDB).

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// ratingScene is one public Reliable reporter (score 78) with one report, a
// RatingService on a clock the test moves, and the pool.
type ratingScene struct {
	t        *testing.T
	pool     *pgxpool.Pool
	q        *sqlcgen.Queries
	svc      *service.RatingService
	at       time.Time
	reporter int64
	report   int64
	username string
}

func newRatingScene(t *testing.T) *ratingScene {
	t.Helper()
	pool := testutil.NewTestDB(t)
	s := &ratingScene{t: t, pool: pool, q: sqlcgen.New(pool), at: time.Now().UTC().Truncate(time.Microsecond), username: "the.reporter"}
	s.svc = service.NewRatingService(s.q, service.WithRatingClock(func() time.Time { return s.at }))
	s.reporter = testutil.SeedPublicReporter(t, pool, "scene-reporter@example.com", s.username, "scene-session", s.at)
	s.report = testutil.SeedHistoryReport(t, pool, "scene-session", "low", "flood", s.at.Add(time.Hour))
	return s
}

// newRater makes an account with a username (clear) and a vote of the given
// kind and value on the scene's report from cell. An empty value makes no vote.
func (s *ratingScene) newRater(username, kind, value, cell string) int64 {
	s.t.Helper()
	id := testutil.SeedReporterAccount(s.t, s.pool, username+"@example.com")
	testutil.SeedUsername(s.t, s.pool, id, username, "clear")
	if value != "" {
		s.vote(id, kind, value, cell)
	}
	return id
}

func (s *ratingScene) vote(account int64, kind, value, cell string) {
	s.t.Helper()
	testutil.MustExec(s.t, s.pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, $3, $4, $5)`,
		s.report, account, kind, value, cell)
}

// tally reads the counted tally of the reporter straight from the query.
func (s *ratingScene) tally(reporter int64) (raters, tapSum int, found bool) {
	s.t.Helper()
	rows, err := s.q.RatingTalliesForReporters(context.Background(), []int64{reporter})
	if err != nil {
		s.t.Fatalf("RatingTalliesForReporters: %v", err)
	}
	if len(rows) == 0 {
		return 0, 0, false
	}
	return int(rows[0].Raters), int(rows[0].TapSum), true
}

// signal reads the reporter's signal through the one loader with the service as
// the source, at the scene clock.
func (s *ratingScene) signal(reporter int64) service.ReporterSignal {
	s.t.Helper()
	m, err := service.LoadReporterSignals(context.Background(), s.q, s.svc, []int64{reporter}, s.at)
	if err != nil {
		s.t.Fatalf("LoadReporterSignals: %v", err)
	}
	return m[reporter]
}

func (s *ratingScene) count(query string, args ...any) int {
	s.t.Helper()
	var n int
	if err := s.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		s.t.Fatalf("counting: %v", err)
	}
	return n
}

// sceneCell is the precision 7 cell of the scene's report, the first 7
// characters of its geohash.
func (s *ratingScene) sceneCell() string {
	s.t.Helper()
	var cell string
	if err := s.pool.QueryRow(context.Background(), `SELECT substr(geohash, 1, 7) FROM reports WHERE id = $1`, s.report).Scan(&cell); err != nil {
		s.t.Fatalf("reading the scene cell: %v", err)
	}
	return cell
}

// TestRatingEligibility: only a verified account that cast a confirm, dispute,
// resolve or reopen vote on one of the reporter's reports may rate; never
// yourself; never a reporter whose profile is off, pending, or blocked either
// way (D-23, T-09-47).
func TestRatingEligibility(t *testing.T) {
	s := newRatingScene(t)
	ctx := context.Background()

	cases := []struct {
		name, kind, value string
		want              bool
	}{
		{"confirm", "content", "confirm", true},
		{"dispute", "content", "dispute", true},
		{"resolve", "resolution", "resolve", true},
		{"reopen", "resolution", "reopen", true},
		{"unresolve only", "resolution", "unresolve", false},
		{"no vote", "", "", false},
	}
	for i, c := range cases {
		rater := s.newRater(fmt.Sprintf("elig.%d", i), c.kind, c.value, "tdr1v8y")
		may, err := s.svc.MayRate(ctx, rater, s.reporter)
		if err != nil || may != c.want {
			t.Errorf("%s: MayRate = %v (%v), want %v", c.name, may, err, c.want)
		}
		err = s.svc.Rate(ctx, rater, s.username, 4)
		if c.want && err != nil {
			t.Errorf("%s: Rate = %v, want nil", c.name, err)
		}
		if !c.want && !errors.Is(err, service.ErrRatingNotAllowed) {
			t.Errorf("%s: Rate = %v, want ErrRatingNotAllowed", c.name, err)
		}
	}

	good := s.newRater("elig.good", "content", "confirm", "tdr1v8y")
	for _, stars := range []int{0, 6, -1} {
		var verr service.ValidationError
		if err := s.svc.Rate(ctx, good, s.username, stars); !errors.As(err, &verr) || verr.Field != "stars" || verr.Message != service.RatingStarsMessage {
			t.Errorf("Rate with %d stars = %v, want a stars ValidationError", stars, err)
		}
	}
	if err := s.svc.Rate(ctx, good, "nobody.here", 3); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Rate of an unknown username = %v, want ErrPersonNotFound", err)
	}
	if err := s.svc.Rate(ctx, good, "!!", 3); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Rate of a malformed username = %v, want ErrPersonNotFound", err)
	}

	// Yourself.
	if err := s.svc.Rate(ctx, s.reporter, s.username, 5); !errors.Is(err, service.ErrRatingNotAllowed) {
		t.Errorf("Rate of yourself = %v, want ErrRatingNotAllowed", err)
	}
	if may, _ := s.svc.MayRate(ctx, s.reporter, s.reporter); may {
		t.Error("MayRate of yourself = true")
	}

	// A block either way.
	blocker := s.newRater("elig.blocker", "content", "confirm", "tdr1v8y")
	testutil.SeedBlock(t, s.pool, blocker, s.reporter)
	if err := s.svc.Rate(ctx, blocker, s.username, 4); !errors.Is(err, service.ErrRatingNotAllowed) {
		t.Errorf("Rate after you blocked them = %v, want ErrRatingNotAllowed", err)
	}
	blocked := s.newRater("elig.blocked", "content", "confirm", "tdr1v8y")
	testutil.SeedBlock(t, s.pool, s.reporter, blocked)
	if err := s.svc.Rate(ctx, blocked, s.username, 4); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Rate after they blocked you = %v, want ErrPersonNotFound", err)
	}
	for _, id := range []int64{blocker, blocked} {
		if may, _ := s.svc.MayRate(ctx, id, s.reporter); may {
			t.Errorf("MayRate with a block = true for account %d", id)
		}
	}

	// A reporter whose profile is off or whose username is pending.
	testutil.MustExec(t, s.pool, `UPDATE accounts SET profile_public = false WHERE id = $1`, s.reporter)
	if err := s.svc.Rate(ctx, good, s.username, 4); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Rate of a private profile = %v, want ErrPersonNotFound", err)
	}
	if may, _ := s.svc.MayRate(ctx, good, s.reporter); may {
		t.Error("MayRate of a private profile = true")
	}
	testutil.MustExec(t, s.pool, `UPDATE accounts SET profile_public = true, username_screen = 'pending' WHERE id = $1`, s.reporter)
	if err := s.svc.Rate(ctx, good, s.username, 4); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Rate of a pending username = %v, want ErrPersonNotFound", err)
	}
	testutil.MustExec(t, s.pool, `UPDATE accounts SET username_screen = 'clear' WHERE id = $1`, s.reporter)

	// A good rater rates once and reads their own rating back.
	if err := s.svc.Rate(ctx, good, s.username, 5); err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if stars, ok, err := s.svc.YourRating(ctx, good, s.reporter); err != nil || !ok || stars != 5 {
		t.Errorf("YourRating = %d %v (%v), want 5 true", stars, ok, err)
	}
	if _, ok, _ := s.svc.YourRating(ctx, blocker, s.reporter); ok {
		t.Error("YourRating of a rater who never rated = true")
	}
}

// TestWitnessAtTheSceneCanRate: a rater whose only qualifying vote was cast from
// the report's own precision 7 cell, the scene, may rate, and that rating counts
// under that cell (orchestrator revision of 2026-10-27, D-21).
func TestWitnessAtTheSceneCanRate(t *testing.T) {
	s := newRatingScene(t)
	ctx := context.Background()

	testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{5, 5, 5, 5})
	witness := s.newRater("scene.witness", "content", "confirm", s.sceneCell())
	if may, err := s.svc.MayRate(ctx, witness, s.reporter); err != nil || !may {
		t.Fatalf("MayRate for a witness at the scene = %v (%v), want true", may, err)
	}
	if err := s.svc.Rate(ctx, witness, s.username, 5); err != nil {
		t.Fatalf("Rate by a witness at the scene: %v", err)
	}
	raters, taps, ok := s.tally(s.reporter)
	if !ok || raters != 5 || taps != 25 {
		t.Errorf("tally = %d raters, %d stars (%v), want 5 and 25: the scene rating counts", raters, taps, ok)
	}
	sig := s.signal(s.reporter)
	if sig.Raters != 5 || sig.TappedHalves == nil || *sig.TappedHalves != 10 {
		t.Errorf("signal = %d raters, halves %v, want 5 raters and 10 halves", sig.Raters, sig.TappedHalves)
	}
}

// TestRatersCountedByDistinctCell: five raters in one cell count as one, five in
// five cells count as five, and two ratings in one cell count once, the most
// recently written (D-21, T-09-47).
func TestRatersCountedByDistinctCell(t *testing.T) {
	s := newRatingScene(t)

	// Five raters in the scene cell count as one rater: below five, outcome only.
	testutil.SeedRatersInOneCell(t, s.pool, s.reporter, s.report, []int{5, 5, 5, 5, 5})
	raters, taps, ok := s.tally(s.reporter)
	if !ok || raters != 1 || taps != 5 {
		t.Errorf("five raters in one cell = %d raters, %d stars (%v), want 1 and 5", raters, taps, ok)
	}
	sig := s.signal(s.reporter)
	if sig.Raters != 0 || sig.OutcomeScore != nil || sig.Score == nil || *sig.Score != 78 {
		t.Errorf("five raters in one cell: signal %+v (score %v), want the outcome only score 78", sig, sig.Score)
	}

	// Four more raters in four different cells make five cells.
	four := testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{5, 5, 5, 5})
	raters, taps, _ = s.tally(s.reporter)
	if raters != 5 || taps != 25 {
		t.Errorf("one cell plus four cells = %d raters, %d stars, want 5 and 25", raters, taps)
	}
	if sig := s.signal(s.reporter); sig.Raters != 5 {
		t.Errorf("five cells: signal Raters = %d, want 5", sig.Raters)
	}

	// Two ratings in one cell count once, the most recently written one.
	older := s.newRater("cell.older", "content", "confirm", "tdr1x11")
	newer := s.newRater("cell.newer", "content", "confirm", "tdr1x11")
	testutil.MustExec(t, s.pool, `INSERT INTO profile_ratings (rater_id, reporter_id, stars, updated_at) VALUES ($1, $3, 1, $4), ($2, $3, 3, $5)`,
		older, newer, s.reporter, s.at.Add(-time.Hour), s.at)
	raters, taps, _ = s.tally(s.reporter)
	if raters != 6 || taps != 28 {
		t.Errorf("a sixth cell with two raters = %d raters, %d stars, want 6 and 28 (the newer 3 counts, the older 1 does not)", raters, taps)
	}

	// The rater's most recent qualifying vote decides the cell: a rater who moved
	// into the cell of another rater counts under the new cell, not the old one,
	// so the old cell is empty and the two share one slot.
	var neighbour string
	if err := s.pool.QueryRow(context.Background(),
		`SELECT geohash_cell FROM votes WHERE account_id = $1`, four[1]).Scan(&neighbour); err != nil {
		t.Fatalf("reading the neighbour's cell: %v", err)
	}
	testutil.MustExec(t, s.pool, `UPDATE votes SET created_at = $2 WHERE account_id = $1`, four[0], s.at.Add(-48*time.Hour))
	s.vote(four[0], "content", "dispute", neighbour)
	raters, taps, _ = s.tally(s.reporter)
	if raters != 5 || taps != 23 {
		t.Errorf("a rater who moved into a neighbour's cell = %d raters, %d stars, want 5 and 23 (the old cell is empty, the shared one counts once)", raters, taps)
	}
}

// TestRepeatTapDoesNotStealTheCell: when two raters share a cell the most
// recently written rating counts; repeating the same tap must not make an older
// rating the newest one, or a rater could take the slot from another (D-21).
func TestRepeatTapDoesNotStealTheCell(t *testing.T) {
	s := newRatingScene(t)
	ctx := context.Background()
	a := s.newRater("steal.a", "content", "confirm", "tdr1x77")
	b := s.newRater("steal.b", "content", "confirm", "tdr1x77")

	if err := s.svc.Rate(ctx, a, s.username, 1); err != nil {
		t.Fatalf("Rate by a: %v", err)
	}
	s.at = s.at.Add(time.Hour)
	if err := s.svc.Rate(ctx, b, s.username, 3); err != nil {
		t.Fatalf("Rate by b: %v", err)
	}
	if raters, taps, _ := s.tally(s.reporter); raters != 1 || taps != 3 {
		t.Fatalf("two raters in one cell = %d raters, %d stars, want 1 and 3 (the newer counts)", raters, taps)
	}
	s.at = s.at.Add(time.Hour)
	if err := s.svc.Rate(ctx, a, s.username, 1); err != nil {
		t.Fatalf("a taps the same value again: %v", err)
	}
	if raters, taps, _ := s.tally(s.reporter); raters != 1 || taps != 3 {
		t.Errorf("after a's repeat tap = %d raters, %d stars, want still 1 and 3", raters, taps)
	}
}

// TestRatingUpsertAndDailyLimit: a first rating is free; a different value inside
// 24 hours is too soon; the same value again succeeds and changes nothing, not
// even the time; after 24 hours a change succeeds; a rating a moderator removed
// is refused (D-02, W4).
func TestRatingUpsertAndDailyLimit(t *testing.T) {
	s := newRatingScene(t)
	ctx := context.Background()
	rater := s.newRater("daily.rater", "content", "confirm", "tdr1v8y")

	if err := s.svc.Rate(ctx, rater, s.username, 4); err != nil {
		t.Fatalf("first Rate: %v", err)
	}
	first := s.at
	s.at = s.at.Add(2 * time.Hour)
	if err := s.svc.Rate(ctx, rater, s.username, 2); !errors.Is(err, service.ErrRatingTooSoon) {
		t.Errorf("a different value after 2 hours = %v, want ErrRatingTooSoon", err)
	}
	if err := s.svc.Rate(ctx, rater, s.username, 4); err != nil {
		t.Errorf("the same value again = %v, want nil", err)
	}
	row, err := s.q.GetRating(ctx, sqlcgen.GetRatingParams{RaterID: rater, ReporterID: s.reporter})
	if err != nil {
		t.Fatalf("GetRating: %v", err)
	}
	if row.Stars != 4 || !row.UpdatedAt.Equal(first) {
		t.Errorf("after the repeat: stars %d, updated %v, want 4 and the first write time %v (a repeat must not reset the lock)", row.Stars, row.UpdatedAt, first)
	}

	s.at = first.Add(service.RatingChangeGap)
	if err := s.svc.Rate(ctx, rater, s.username, 2); err != nil {
		t.Errorf("a change exactly 24 hours later = %v, want nil", err)
	}
	if stars, ok, _ := s.svc.YourRating(ctx, rater, s.reporter); !ok || stars != 2 {
		t.Errorf("YourRating after the change = %d %v, want 2", stars, ok)
	}

	// A moderator removes it: the rater cannot write it back.
	mod := s.newRater("mod.daily", "", "", "")
	s.at = s.at.Add(48 * time.Hour)
	if err := s.svc.ModeratorRemove(ctx, mod, s.username, "daily.rater"); err != nil {
		t.Fatalf("ModeratorRemove: %v", err)
	}
	s.at = s.at.Add(48 * time.Hour)
	if err := s.svc.Rate(ctx, rater, s.username, 5); !errors.Is(err, service.ErrRatingNotAllowed) {
		t.Errorf("Rate after a moderator removed the rating = %v, want ErrRatingNotAllowed", err)
	}
	if _, ok, _ := s.svc.YourRating(ctx, rater, s.reporter); ok {
		t.Error("YourRating shows a rating a moderator removed")
	}
}

// TestRatingClear: Remove my rating removes its effect, follows the same daily
// rule, and with nothing to remove succeeds (D-02).
func TestRatingClear(t *testing.T) {
	s := newRatingScene(t)
	ctx := context.Background()
	testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{5, 5, 5, 5})
	rater := s.newRater("clear.rater", "content", "confirm", "tdr1x33")

	if err := s.svc.Rate(ctx, rater, s.username, 5); err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if raters, _, _ := s.tally(s.reporter); raters != 5 {
		t.Fatalf("raters = %d, want 5 before the clear", raters)
	}
	s.at = s.at.Add(time.Hour)
	if err := s.svc.Clear(ctx, rater, s.username); !errors.Is(err, service.ErrRatingTooSoon) {
		t.Errorf("Clear after 1 hour = %v, want ErrRatingTooSoon", err)
	}
	s.at = s.at.Add(25 * time.Hour)
	if err := s.svc.Clear(ctx, rater, s.username); err != nil {
		t.Fatalf("Clear after 26 hours: %v", err)
	}
	if raters, _, _ := s.tally(s.reporter); raters != 4 {
		t.Errorf("raters after the clear = %d, want 4", raters)
	}
	if _, ok, _ := s.svc.YourRating(ctx, rater, s.reporter); ok {
		t.Error("YourRating shows a cleared rating")
	}
	if err := s.svc.Clear(ctx, rater, s.username); err != nil {
		t.Errorf("a second Clear = %v, want nil", err)
	}
	if err := s.svc.Clear(ctx, rater, "nobody.here"); err != nil {
		t.Errorf("Clear of an unknown username = %v, want nil", err)
	}
	other := s.newRater("clear.other", "content", "confirm", "tdr1x44")
	if err := s.svc.Clear(ctx, other, s.username); err != nil {
		t.Errorf("Clear with no rating = %v, want nil", err)
	}
	if err := s.svc.Clear(ctx, rater, "!!"); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Clear of a malformed username = %v, want ErrPersonNotFound", err)
	}
	// Rating again right after a clear is a change inside the daily rule.
	if err := s.svc.Rate(ctx, rater, s.username, 5); !errors.Is(err, service.ErrRatingTooSoon) {
		t.Errorf("Rate right after a clear = %v, want ErrRatingTooSoon", err)
	}
	s.at = s.at.Add(25 * time.Hour)
	if err := s.svc.Rate(ctx, rater, s.username, 5); err != nil {
		t.Errorf("Rate a day after a clear = %v, want nil", err)
	}
	if raters, _, _ := s.tally(s.reporter); raters != 5 {
		t.Errorf("raters after rating again = %d, want 5", raters)
	}
	// A rater can take a rating back after the reporter went private.
	testutil.MustExec(t, s.pool, `UPDATE accounts SET profile_public = false WHERE id = $1`, s.reporter)
	s.at = s.at.Add(25 * time.Hour)
	if err := s.svc.Clear(ctx, rater, s.username); err != nil {
		t.Errorf("Clear after the reporter went private = %v, want nil", err)
	}
}

// TestRatingsIgnoreRemoved: a rating a moderator removed and a rating its rater
// cleared are ignored by the tally (D-14).
func TestRatingsIgnoreRemoved(t *testing.T) {
	s := newRatingScene(t)
	ctx := context.Background()
	raters := testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{5, 5, 5, 5, 5})
	testutil.SeedUsername(t, s.pool, raters[0], "removed.one", "clear")
	mod := s.newRater("mod.removed", "", "", "")

	if n, taps, _ := s.tally(s.reporter); n != 5 || taps != 25 {
		t.Fatalf("before: %d raters %d stars, want 5 and 25", n, taps)
	}
	if err := s.svc.ModeratorRemove(ctx, mod, s.username, "removed.one"); err != nil {
		t.Fatalf("ModeratorRemove: %v", err)
	}
	if n, taps, _ := s.tally(s.reporter); n != 4 || taps != 20 {
		t.Errorf("after a moderator removal: %d raters %d stars, want 4 and 20", n, taps)
	}
	if n := s.count(`SELECT count(*) FROM profile_ratings WHERE rater_id = $1 AND removed_at IS NOT NULL AND removed_by = $2`, raters[0], mod); n != 1 {
		t.Errorf("the removed row stamped by the moderator = %d, want 1", n)
	}
	// Removing it again changes nothing.
	if err := s.svc.ModeratorRemove(ctx, mod, s.username, "removed.one"); err != nil {
		t.Errorf("a second ModeratorRemove = %v, want nil", err)
	}

	testutil.MustExec(t, s.pool, `UPDATE profile_ratings SET cleared_at = now() WHERE rater_id = $1`, raters[1])
	if n, taps, _ := s.tally(s.reporter); n != 3 || taps != 15 {
		t.Errorf("after a cleared rating: %d raters %d stars, want 3 and 15", n, taps)
	}

	list, err := s.svc.ModeratorList(ctx, s.username)
	if err != nil {
		t.Fatalf("ModeratorList: %v", err)
	}
	sawRemoved := false
	for _, r := range list {
		if r.RaterUsername == "removed.one" && r.Removed && r.Stars == 5 {
			sawRemoved = true
		}
	}
	if !sawRemoved {
		t.Errorf("ModeratorList = %+v, want the removed rating of removed.one marked removed", list)
	}
	if _, err := s.svc.ModeratorList(ctx, "nobody.here"); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("ModeratorList of an unknown username = %v, want ErrPersonNotFound", err)
	}
}

// TestRatingsIgnoreDeletedRater: a rating from a deleted account stops counting
// (D-14, G6, Pitfall 11).
func TestRatingsIgnoreDeletedRater(t *testing.T) {
	s := newRatingScene(t)
	raters := testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{5, 5, 5, 5, 5})
	if n, _, _ := s.tally(s.reporter); n != 5 {
		t.Fatalf("before: %d raters, want 5", n)
	}
	testutil.MustExec(t, s.pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, raters[2])
	if n, taps, _ := s.tally(s.reporter); n != 4 || taps != 20 {
		t.Errorf("with one deleted rater: %d raters %d stars, want 4 and 20", n, taps)
	}
	if sig := s.signal(s.reporter); sig.Raters != 0 || *sig.Score != 78 {
		t.Errorf("with 4 live raters the signal = %d raters score %d, want outcome only 78", sig.Raters, *sig.Score)
	}
}

// TestSwitchingOffNeverRaisesTag: while the profile is off the ratings only
// lower the points, so going private never raises the tag (D-22, T-09-97).
func TestSwitchingOffNeverRaisesTag(t *testing.T) {
	s := newRatingScene(t)

	if sig := s.signal(s.reporter); *sig.Score != 78 || sig.Tag != service.ReporterReliable {
		t.Fatalf("seed: score %d tag %s, want 78 Reliable", *sig.Score, sig.Tag)
	}
	testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{1, 1, 1, 1, 1})

	public := s.signal(s.reporter)
	if *public.Score != 49 || public.Tag != service.ReporterMixed || public.Raters != 5 {
		t.Fatalf("public with five ones: score %d tag %s raters %d, want 49 Mixed 5", *public.Score, public.Tag, public.Raters)
	}
	testutil.MustExec(t, s.pool, `UPDATE accounts SET profile_public = false WHERE id = $1`, s.reporter)
	private := s.signal(s.reporter)
	if *private.Score != 49 || private.Tag != service.ReporterMixed {
		t.Errorf("private with five ones: score %d tag %s, want still 49 Mixed (going private must not raise the tag)", *private.Score, private.Tag)
	}
	testutil.MustExec(t, s.pool, `UPDATE accounts SET profile_public = true WHERE id = $1`, s.reporter)
	if again := s.signal(s.reporter); *again.Score != 49 {
		t.Errorf("switched on again: score %d, want 49", *again.Score)
	}

	// Five star ratings lift a public profile and are ignored while private.
	s2 := newRatingScene(t)
	testutil.SeedRaters(t, s2.pool, s2.reporter, s2.report, []int{5, 5, 5, 5, 5})
	if hi := s2.signal(s2.reporter); *hi.Score != 89 {
		t.Fatalf("public with five fives: score %d, want 89", *hi.Score)
	}
	testutil.MustExec(t, s2.pool, `UPDATE accounts SET profile_public = false WHERE id = $1`, s2.reporter)
	if hi := s2.signal(s2.reporter); *hi.Score != 78 || hi.Raters != 0 {
		t.Errorf("private with five fives: score %d raters %d, want the outcome 78 and no rating parts", *hi.Score, hi.Raters)
	}
	// The source returns the tally with its flag for a private reporter too.
	opts, err := s2.svc.SignalOptions(context.Background(), []int64{s2.reporter}, s2.at)
	if err != nil || len(opts) != 1 {
		t.Errorf("SignalOptions for a private reporter = %d options (%v), want 1", len(opts), err)
	}
	if opts, err := s2.svc.SignalOptions(context.Background(), nil, s2.at); err != nil || len(opts) != 0 {
		t.Errorf("SignalOptions with no ids = %d options (%v), want none", len(opts), err)
	}
}

// followScene adds followers to the reporter: two active followers and one that
// ended for another reason, and returns their ids.
func (s *ratingScene) followScene() (active []int64, unfollowed int64) {
	for i := 0; i < 2; i++ {
		id := testutil.SeedPerson(s.t, s.pool, fmt.Sprintf("follower-%d@example.com", i), fmt.Sprintf("follower.%d", i), true)
		testutil.SeedFollow(s.t, s.pool, id, s.reporter, s.at.Add(-time.Hour))
		active = append(active, id)
	}
	unfollowed = testutil.SeedPerson(s.t, s.pool, "follower-gone@example.com", "follower.gone", true)
	testutil.SeedFollow(s.t, s.pool, unfollowed, s.reporter, s.at.Add(-time.Hour))
	testutil.MustExec(s.t, s.pool,
		`UPDATE account_follows SET ended_at = $2, end_reason = 'unfollowed' WHERE follower_id = $1`, unfollowed, s.at.Add(-30*time.Minute))
	return active, unfollowed
}

// TestRatingWriteEndsFollowsSoftly: a rating write that pulls the tag below
// Reliable ends the reporter's follows softly with reason tag_lost, and the rows
// remain (D-21, review W1).
func TestRatingWriteEndsFollowsSoftly(t *testing.T) {
	ctx := context.Background()

	// A rating that keeps the tag Reliable (four fives and a fifth five, 89)
	// ends nothing.
	keep := newRatingScene(t)
	keep.followScene()
	testutil.SeedRaters(t, keep.pool, keep.reporter, keep.report, []int{5, 5, 5, 5})
	keepFifth := keep.newRater("keep.fifth", "content", "confirm", "tdr1x55")
	if err := keep.svc.Rate(ctx, keepFifth, keep.username, 5); err != nil {
		t.Fatalf("Rate 5: %v", err)
	}
	if n := keep.count(`SELECT count(*) FROM account_follows WHERE followee_id = $1 AND ended_at IS NULL`, keep.reporter); n != 2 {
		t.Errorf("active follows after a rating that keeps the tag = %d, want 2", n)
	}

	// Four one star ratings are below the threshold, so nothing has ended; the
	// fifth one star rating pulls the points to 49 and ends the follows.
	s := newRatingScene(t)
	s.followScene()
	testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{1, 1, 1, 1})
	fifth := s.newRater("fifth.rater", "content", "confirm", "tdr1x55")
	if n := s.count(`SELECT count(*) FROM account_follows WHERE followee_id = $1 AND ended_at IS NULL`, s.reporter); n != 2 {
		t.Fatalf("active follows before = %d, want 2", n)
	}
	if err := s.svc.Rate(ctx, fifth, s.username, 1); err != nil {
		t.Fatalf("Rate 1: %v", err)
	}
	if sig := s.signal(s.reporter); sig.Tag == service.ReporterReliable {
		t.Fatalf("the tag is still Reliable at score %d", *sig.Score)
	}
	if n := s.count(`SELECT count(*) FROM account_follows WHERE followee_id = $1 AND ended_at IS NULL`, s.reporter); n != 0 {
		t.Errorf("active follows after the tag fell = %d, want 0", n)
	}
	if n := s.count(`SELECT count(*) FROM account_follows WHERE followee_id = $1 AND end_reason = 'tag_lost'`, s.reporter); n != 2 {
		t.Errorf("follows ended with tag_lost = %d, want 2", n)
	}
	if n := s.count(`SELECT count(*) FROM account_follows WHERE followee_id = $1`, s.reporter); n != 3 {
		t.Errorf("follow rows remaining = %d, want all 3 (ending is soft)", n)
	}
}

// TestModeratorRemovalRestoresFollows: a moderator removal re-evaluates the
// reporter and brings back the tag_lost follows ended since the removed rating
// was written, when the reporter is Reliable again; a follow ended for another
// reason, and a follow of someone who blocked the reporter, stay ended (D-21,
// review W1).
func TestModeratorRemovalRestoresFollows(t *testing.T) {
	s := newRatingScene(t)
	ctx := context.Background()
	active, unfollowed := s.followScene()
	blockedFollower := testutil.SeedPerson(t, s.pool, "follower-blocked@example.com", "follower.blocked", true)
	testutil.SeedFollow(t, s.pool, blockedFollower, s.reporter, s.at.Add(-time.Hour))
	testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{1, 1, 1, 1})
	fifth := s.newRater("fifth.rater", "content", "confirm", "tdr1x55")
	mod := s.newRater("the.moderator", "", "", "")

	s.at = s.at.Add(time.Hour)
	if err := s.svc.Rate(ctx, fifth, s.username, 1); err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if n := s.count(`SELECT count(*) FROM account_follows WHERE followee_id = $1 AND end_reason = 'tag_lost'`, s.reporter); n != 3 {
		t.Fatalf("follows ended with tag_lost = %d, want 3", n)
	}
	testutil.SeedBlock(t, s.pool, blockedFollower, s.reporter)

	s.at = s.at.Add(time.Hour)
	if err := s.svc.ModeratorRemove(ctx, mod, s.username, "fifth.rater"); err != nil {
		t.Fatalf("ModeratorRemove: %v", err)
	}
	if sig := s.signal(s.reporter); sig.Tag != service.ReporterReliable {
		t.Fatalf("after the removal the tag = %s, want Reliable", sig.Tag)
	}
	for _, id := range active {
		if n := s.count(`SELECT count(*) FROM account_follows WHERE follower_id = $1 AND followee_id = $2 AND ended_at IS NULL AND end_reason IS NULL`, id, s.reporter); n != 1 {
			t.Errorf("follower %d: restored rows = %d, want 1", id, n)
		}
	}
	if n := s.count(`SELECT count(*) FROM account_follows WHERE follower_id = $1 AND ended_at IS NOT NULL AND end_reason = 'unfollowed'`, unfollowed); n != 1 {
		t.Error("a follow ended for another reason was changed")
	}
	if n := s.count(`SELECT count(*) FROM account_follows WHERE follower_id = $1 AND ended_at IS NOT NULL AND end_reason = 'tag_lost'`, blockedFollower); n != 1 {
		t.Error("a follow of a blocked pair was restored")
	}

	// Removing a rating that leaves the tag below Reliable restores nothing and
	// ends the follows again.
	s2 := newRatingScene(t)
	s2.followScene()
	six := testutil.SeedRaters(t, s2.pool, s2.reporter, s2.report, []int{1, 1, 1, 1, 1, 1})
	testutil.SeedUsername(t, s2.pool, six[0], "still.low", "clear")
	mod2 := s2.newRater("the.moderator", "", "", "")
	if err := s2.svc.ModeratorRemove(ctx, mod2, s2.username, "still.low"); err != nil {
		t.Fatalf("ModeratorRemove: %v", err)
	}
	if sig := s2.signal(s2.reporter); sig.Tag == service.ReporterReliable {
		t.Fatalf("five low raters should leave the tag below Reliable, score %d", *sig.Score)
	}
	if n := s2.count(`SELECT count(*) FROM account_follows WHERE followee_id = $1 AND ended_at IS NULL`, s2.reporter); n != 0 {
		t.Errorf("active follows with the tag still low = %d, want 0", n)
	}
}

// TestRemovedRaterCannotRateAgain: removing every rating of one rater marks them
// removed for good, drops each of their ratings from the tallies, and refuses any
// later rating (D-14, G2, W4, T-09-51).
func TestRemovedRaterCannotRateAgain(t *testing.T) {
	s := newRatingScene(t)
	ctx := context.Background()
	raters := testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{5, 5, 5, 5, 5})
	testutil.SeedUsername(t, s.pool, raters[0], "bad.rater", "clear")
	mod := s.newRater("the.moderator", "", "", "")

	// The rater also rated a second reporter.
	other := testutil.SeedPublicReporter(t, s.pool, "second-reporter@example.com", "second.reporter", "second-session", s.at)
	otherReport := testutil.SeedHistoryReport(t, s.pool, "second-session", "low", "flood", s.at.Add(time.Hour))
	testutil.MustExec(t, s.pool, `INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, 'content', 'confirm', 'tdr1x66')`, otherReport, raters[0])
	testutil.MustExec(t, s.pool, `INSERT INTO profile_ratings (rater_id, reporter_id, stars) VALUES ($1, $2, 1)`, raters[0], other)

	n, err := s.svc.ModeratorRemoveRater(ctx, mod, "bad.rater")
	if err != nil || n != 2 {
		t.Fatalf("ModeratorRemoveRater = %d (%v), want 2", n, err)
	}
	if raters, _, _ := s.tally(s.reporter); raters != 4 {
		t.Errorf("raters after the rater was removed = %d, want 4", raters)
	}
	if _, _, found := s.tally(other); found {
		t.Error("the second reporter still has a counted rating")
	}
	if c := s.count(`SELECT count(*) FROM accounts WHERE id = $1 AND rater_removed_at IS NOT NULL AND rater_removed_by = $2`, raters[0], mod); c != 1 {
		t.Error("the rater was not marked removed")
	}

	s.at = s.at.Add(72 * time.Hour)
	if err := s.svc.Rate(ctx, raters[0], s.username, 5); !errors.Is(err, service.ErrRatingNotAllowed) {
		t.Errorf("Rate by a removed rater = %v, want ErrRatingNotAllowed", err)
	}
	if err := s.svc.Rate(ctx, raters[0], "second.reporter", 5); !errors.Is(err, service.ErrRatingNotAllowed) {
		t.Errorf("Rate of another reporter by a removed rater = %v, want ErrRatingNotAllowed", err)
	}
	if may, _ := s.svc.MayRate(ctx, raters[0], s.reporter); may {
		t.Error("MayRate for a removed rater = true")
	}
	if again, err := s.svc.ModeratorRemoveRater(ctx, mod, "bad.rater"); err != nil || again != 0 {
		t.Errorf("a second ModeratorRemoveRater = %d (%v), want 0", again, err)
	}
	if _, err := s.svc.ModeratorRemoveRater(ctx, mod, "nobody.here"); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("ModeratorRemoveRater of an unknown username = %v, want ErrPersonNotFound", err)
	}
}

// TestBlockKeepsRatings: a block in either direction never deletes or hides a
// rating, and the tally keeps counting it (D-14, research Pitfall 10, T-09-50).
func TestBlockKeepsRatings(t *testing.T) {
	s := newRatingScene(t)
	raters := testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{5, 5, 5, 5, 5})

	testutil.SeedBlock(t, s.pool, raters[0], s.reporter)
	testutil.SeedBlock(t, s.pool, s.reporter, raters[1])
	if n := s.count(`SELECT count(*) FROM profile_ratings WHERE reporter_id = $1 AND removed_at IS NULL AND cleared_at IS NULL`, s.reporter); n != 5 {
		t.Errorf("rating rows after the blocks = %d, want 5", n)
	}
	if n, taps, _ := s.tally(s.reporter); n != 5 || taps != 25 {
		t.Errorf("tally after the blocks = %d raters %d stars, want 5 and 25", n, taps)
	}
	if sig := s.signal(s.reporter); sig.Raters != 5 || *sig.Score != 89 {
		t.Errorf("signal after the blocks = %d raters score %d, want 5 and 89", sig.Raters, *sig.Score)
	}
}
