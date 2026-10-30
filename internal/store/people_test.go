package store_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// peopleNow is the fixed clock of the people tests.
var peopleNow = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

func follow(t *testing.T, q *sqlcgen.Queries, follower, followee int64, at time.Time) int64 {
	t.Helper()
	n, err := q.FollowAccount(context.Background(), sqlcgen.FollowAccountParams{FollowerID: follower, FolloweeID: followee, Now: at})
	if err != nil {
		t.Fatalf("FollowAccount %d -> %d: %v", follower, followee, err)
	}
	return n
}

func endPair(t *testing.T, q *sqlcgen.Queries, follower, followee int64, reason string, at time.Time) int64 {
	t.Helper()
	n, err := q.EndFollowPair(context.Background(), sqlcgen.EndFollowPairParams{
		FollowerID: follower, FolloweeID: followee, Reason: reason, Now: at,
	})
	if err != nil {
		t.Fatalf("EndFollowPair %d -> %d: %v", follower, followee, err)
	}
	return n
}

func markDeleted(t *testing.T, pool *pgxpool.Pool, id int64) {
	t.Helper()
	testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, id)
}

// TestFollowAccountGuards proves every precondition of a follow is checked in
// the one statement (D-01, D-12, T-09-05): a private, pending or deleted
// followee and a block in either direction write nothing, a valid follow writes
// one row, a repeat writes nothing and a follow of yourself is SQLSTATE 23514.
func TestFollowAccountGuards(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)

	me := testutil.SeedPerson(t, pool, "guard-me@example.com", "guard.me", false)
	private := testutil.SeedPerson(t, pool, "guard-private@example.com", "guard.private", false)
	pending := testutil.SeedPerson(t, pool, "guard-pending@example.com", "guard.pending", true)
	testutil.SeedUsername(t, pool, pending, "guard.pending", "pending")
	deleted := testutil.SeedPerson(t, pool, "guard-deleted@example.com", "guard.deleted", true)
	markDeleted(t, pool, deleted)
	blockedByMe := testutil.SeedPerson(t, pool, "guard-bbm@example.com", "guard.bbm", true)
	testutil.SeedBlock(t, pool, me, blockedByMe)
	blocksMe := testutil.SeedPerson(t, pool, "guard-bm@example.com", "guard.bm", true)
	testutil.SeedBlock(t, pool, blocksMe, me)
	good := testutil.SeedPerson(t, pool, "guard-good@example.com", "guard.good", true)
	nameless := testutil.SeedReporterAccount(t, pool, "guard-nameless@example.com")
	testutil.MustExec(t, pool, `UPDATE accounts SET profile_public = true, profile_public_at = now() WHERE id = $1`, nameless)

	for _, c := range []struct {
		name     string
		followee int64
	}{
		{"a private followee", private},
		{"a pending username", pending},
		{"a deleted followee", deleted},
		{"a followee I blocked", blockedByMe},
		{"a followee who blocked me", blocksMe},
		{"a followee with no username", nameless},
	} {
		if n := follow(t, q, me, c.followee, peopleNow); n != 0 {
			t.Errorf("%s: FollowAccount wrote %d rows, want 0", c.name, n)
		}
	}
	var rows int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM account_follows`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("after the refused follows: %d rows (%v), want 0", rows, err)
	}

	if n := follow(t, q, me, good, peopleNow); n != 1 {
		t.Fatalf("a valid follow wrote %d rows, want 1", n)
	}
	if n := follow(t, q, me, good, peopleNow.Add(time.Hour)); n != 0 {
		t.Errorf("a repeat follow wrote %d rows, want 0", n)
	}

	// A follow of yourself reaches the CHECK: the profile must be public for the
	// insert to be attempted at all.
	self := testutil.SeedPerson(t, pool, "guard-self@example.com", "guard.self", true)
	_, err := q.FollowAccount(context.Background(), sqlcgen.FollowAccountParams{FollowerID: self, FolloweeID: self, Now: peopleNow})
	if got := sqlState(err); got != "23514" {
		t.Errorf("a self follow: SQLSTATE %q (%v), want 23514", got, err)
	}
}

// TestFollowReactivatesEndedRow proves following again reuses the same row (D-21).
func TestFollowReactivatesEndedRow(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	a := testutil.SeedPerson(t, pool, "react-a@example.com", "react.a", false)
	b := testutil.SeedPerson(t, pool, "react-b@example.com", "react.b", true)

	if n := follow(t, q, a, b, peopleNow); n != 1 {
		t.Fatalf("first follow wrote %d rows, want 1", n)
	}
	if n := endPair(t, q, a, b, "unfollowed", peopleNow.Add(time.Hour)); n != 1 {
		t.Fatalf("EndFollowPair ended %d rows, want 1", n)
	}
	if n := endPair(t, q, a, b, "unfollowed", peopleNow.Add(2*time.Hour)); n != 0 {
		t.Errorf("a second end changed %d rows, want 0", n)
	}
	later := peopleNow.Add(48 * time.Hour)
	if n := follow(t, q, a, b, later); n != 1 {
		t.Fatalf("following again wrote %d rows, want 1", n)
	}

	var count int
	var createdAt time.Time
	var endedAt pgtype.Timestamptz
	var reason *string
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM account_follows WHERE follower_id = $1 AND followee_id = $2`, a, b).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rows for the pair = %d (%v), want 1", count, err)
	}
	if err := pool.QueryRow(context.Background(),
		`SELECT created_at, ended_at, end_reason FROM account_follows WHERE follower_id = $1 AND followee_id = $2`, a, b,
	).Scan(&createdAt, &endedAt, &reason); err != nil {
		t.Fatalf("reading the row: %v", err)
	}
	if !createdAt.Equal(later) || endedAt.Valid || reason != nil {
		t.Errorf("row = created %v ended %v reason %v, want created %v and no end", createdAt, endedAt.Valid, reason, later)
	}
}

// TestBlockAccountInsertsBlockOnly proves BlockAccount writes the block row and
// nothing else, and that FollowAccount then refuses in both directions (D-12,
// review W1): the service ends the follows through endFollowPair.
func TestBlockAccountInsertsBlockOnly(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	a := testutil.SeedPerson(t, pool, "block-a@example.com", "block.a", true)
	b := testutil.SeedPerson(t, pool, "block-b@example.com", "block.b", true)
	if follow(t, q, a, b, peopleNow) != 1 || follow(t, q, b, a, peopleNow) != 1 {
		t.Fatal("seeding the two follows failed")
	}

	args := sqlcgen.BlockAccountParams{BlockerID: b, BlockedID: a, Now: peopleNow}
	if err := q.BlockAccount(ctx, args); err != nil {
		t.Fatalf("BlockAccount: %v", err)
	}
	if err := q.BlockAccount(ctx, args); err != nil {
		t.Fatalf("a repeat BlockAccount must be a no op: %v", err)
	}
	var blocks, active int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM person_blocks WHERE blocker_id = $1 AND blocked_id = $2`, b, a).Scan(&blocks); err != nil || blocks != 1 {
		t.Fatalf("block rows = %d (%v), want 1", blocks, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM account_follows WHERE ended_at IS NULL`).Scan(&active); err != nil || active != 2 {
		t.Fatalf("active follows after BlockAccount = %d (%v), want both untouched", active, err)
	}

	endPair(t, q, a, b, "blocked", peopleNow)
	endPair(t, q, b, a, "blocked", peopleNow)
	if n := follow(t, q, a, b, peopleNow.Add(time.Hour)); n != 0 {
		t.Errorf("follow by the blocked person wrote %d rows, want 0", n)
	}
	if n := follow(t, q, b, a, peopleNow.Add(time.Hour)); n != 0 {
		t.Errorf("follow by the blocker wrote %d rows, want 0", n)
	}

	err := q.BlockAccount(ctx, sqlcgen.BlockAccountParams{BlockerID: a, BlockedID: a, Now: peopleNow})
	if got := sqlState(err); got != "23514" {
		t.Errorf("a self block: SQLSTATE %q (%v), want 23514", got, err)
	}
	if n, err := q.UnblockAccount(ctx, sqlcgen.UnblockAccountParams{BlockerID: b, BlockedID: a}); err != nil || n != 1 {
		t.Errorf("UnblockAccount = %d (%v), want 1", n, err)
	}
	if n := follow(t, q, a, b, peopleNow.Add(2*time.Hour)); n != 1 {
		t.Errorf("after the unblock a follow wrote %d rows, want 1", n)
	}
}

// TestBlockAccountSkipsADeletedAccount (review IN-04 of quick 261030-u78,
// 2026-10-30): the insert checks the blocked account in the same statement, so
// an account already deleted gets no block row that no list could show or take
// back, and the call is still not an error. A live account in the same test
// gets its row, so the skip comes from the deleted stamp.
func TestBlockAccountSkipsADeletedAccount(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	a := testutil.SeedPerson(t, pool, "block-del-a@example.com", "block.del.a", true)
	gone := testutil.SeedPerson(t, pool, "block-del-gone@example.com", "block.del.gone", true)
	live := testutil.SeedPerson(t, pool, "block-del-live@example.com", "block.del.live", true)
	testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, gone)

	if err := q.BlockAccount(ctx, sqlcgen.BlockAccountParams{BlockerID: a, BlockedID: gone, Now: peopleNow}); err != nil {
		t.Fatalf("BlockAccount of a deleted account: %v", err)
	}
	if err := q.BlockAccount(ctx, sqlcgen.BlockAccountParams{BlockerID: a, BlockedID: live, Now: peopleNow}); err != nil {
		t.Fatalf("BlockAccount of a live account: %v", err)
	}
	count := func(blocked int64) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM person_blocks WHERE blocker_id = $1 AND blocked_id = $2`, a, blocked).Scan(&n); err != nil {
			t.Fatalf("counting blocks: %v", err)
		}
		return n
	}
	if n := count(gone); n != 0 {
		t.Errorf("block rows against a deleted account = %d, want 0", n)
	}
	if n := count(live); n != 1 {
		t.Errorf("block rows against a live account = %d, want 1", n)
	}
}

// TestEndedFollowsAreInvisible proves an ended follow appears in no read of
// current follows (D-21, T-09-95). A control follow that stays active proves
// every read does see an active one.
func TestEndedFollowsAreInvisible(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	viewer := testutil.SeedPerson(t, pool, "ended-viewer@example.com", "ended.viewer", false)
	a := testutil.SeedPerson(t, pool, "ended-a@example.com", "ended.a", true)
	b := testutil.SeedPerson(t, pool, "ended-b@example.com", "ended.b", true)
	if follow(t, q, a, b, peopleNow) != 1 || follow(t, q, b, a, peopleNow) != 1 {
		t.Fatal("seeding the follows failed")
	}

	type reads struct {
		following, followers, followingTotal, namedFollowers, namedFollowing int64
		followersNames, followingNames                                       int
		aFollowsB, bFollowsA                                                 bool
		ids                                                                  int
	}
	read := func() reads {
		t.Helper()
		var r reads
		var err error
		if r.following, err = q.CountFollowing(ctx, a); err != nil {
			t.Fatalf("CountFollowing: %v", err)
		}
		counts, err := q.FollowCounts(ctx, b)
		if err != nil {
			t.Fatalf("FollowCounts: %v", err)
		}
		r.followers, r.followingTotal = counts.Followers, counts.Following
		rel, err := q.PersonRelation(ctx, sqlcgen.PersonRelationParams{ViewerID: a, OtherID: b})
		if err != nil {
			t.Fatalf("PersonRelation: %v", err)
		}
		r.aFollowsB, r.bFollowsA = rel.ViewerFollows, rel.FollowsViewer
		fp, err := q.FollowersPage(ctx, sqlcgen.FollowersPageParams{AccountID: b, ViewerID: viewer, RowLimit: 50})
		if err != nil {
			t.Fatalf("FollowersPage: %v", err)
		}
		r.followersNames = len(fp)
		gp, err := q.FollowingPage(ctx, sqlcgen.FollowingPageParams{AccountID: a, ViewerID: viewer, RowLimit: 50})
		if err != nil {
			t.Fatalf("FollowingPage: %v", err)
		}
		r.followingNames = len(gp)
		if r.namedFollowers, err = q.CountNamedFollowers(ctx, sqlcgen.CountNamedFollowersParams{AccountID: b, ViewerID: viewer}); err != nil {
			t.Fatalf("CountNamedFollowers: %v", err)
		}
		if r.namedFollowing, err = q.CountNamedFollowing(ctx, sqlcgen.CountNamedFollowingParams{AccountID: a, ViewerID: viewer}); err != nil {
			t.Fatalf("CountNamedFollowing: %v", err)
		}
		ids, err := q.FolloweeIDsPage(ctx, sqlcgen.FolloweeIDsPageParams{AfterID: 0, RowLimit: 50})
		if err != nil {
			t.Fatalf("FolloweeIDsPage: %v", err)
		}
		r.ids = len(ids)
		return r
	}

	active := read()
	want := reads{following: 1, followers: 1, followingTotal: 1, namedFollowers: 1, namedFollowing: 1,
		followersNames: 1, followingNames: 1, aFollowsB: true, bFollowsA: true, ids: 2}
	if active != want {
		t.Fatalf("with active follows: %+v, want %+v", active, want)
	}

	endPair(t, q, a, b, "unfollowed", peopleNow.Add(time.Hour))
	endPair(t, q, b, a, "removed", peopleNow.Add(time.Hour))
	if got := read(); got != (reads{}) {
		t.Errorf("with both follows ended: %+v, want every read empty", got)
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM account_follows`).Scan(&rows); err != nil || rows != 2 {
		t.Errorf("ended rows kept = %d (%v), want 2 (soft ending)", rows, err)
	}
}

// queryBlocks splits one query file into its named blocks, with comment lines
// removed, keyed by the query name.
func queryBlocks(text string) map[string]string {
	blocks := map[string]string{}
	name := ""
	var body []string
	flush := func() {
		if name != "" {
			blocks[name] = strings.Join(body, "\n")
		}
	}
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "-- name:") {
			flush()
			fields := strings.Fields(trim)
			name = fields[2]
			body = nil
			continue
		}
		if strings.HasPrefix(trim, "--") {
			continue
		}
		body = append(body, line)
	}
	flush()
	return blocks
}

// followReadsMissingEnded returns the names of the blocks that touch
// account_follows without naming ended_at, except the hard delete of an account.
func followReadsMissingEnded(blocks map[string]string) []string {
	var bad []string
	for name, body := range blocks {
		if name == "DeleteAccountFollows" {
			continue
		}
		if strings.Contains(body, "account_follows") && !strings.Contains(body, "ended_at") {
			bad = append(bad, name)
		}
	}
	sort.Strings(bad)
	return bad
}

// TestFollowReadsFilterEnded reads every query file as text and fails when a
// query touches account_follows without naming ended_at, so a read added later
// in ratings.sql, updates.sql or push.sql cannot count an ended follow (D-21,
// T-09-95). The one exception is the hard delete with an account. It needs no
// database.
func TestFollowReadsFilterEnded(t *testing.T) {
	// Negative control: a block that reads the table with no ended_at is caught,
	// a comment naming ended_at does not rescue it, and the exception is let by.
	sample := queryBlocks("-- name: Bad :many\n-- ended_at is only in this comment\nSELECT * FROM account_follows;\n" +
		"-- name: Good :many\nSELECT * FROM account_follows WHERE ended_at IS NULL;\n" +
		"-- name: DeleteAccountFollows :execrows\nDELETE FROM account_follows;\n")
	if got := followReadsMissingEnded(sample); len(got) != 1 || got[0] != "Bad" {
		t.Fatalf("the scan found %v in its own sample, want [Bad]", got)
	}

	files, err := filepath.Glob("queries/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("listing query files: %v (%d files)", err, len(files))
	}
	seen := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		blocks := queryBlocks(string(b))
		for _, body := range blocks {
			if strings.Contains(body, "account_follows") {
				seen++
			}
		}
		for _, name := range followReadsMissingEnded(blocks) {
			t.Errorf("%s: query %s touches account_follows without naming ended_at", f, name)
		}
	}
	if seen < 10 {
		t.Errorf("the scan saw only %d queries on account_follows, want at least 10", seen)
	}
}

// TestFollowersPageSplit proves the named list shows only opted in, clear, live
// people with no block either way, and that everyone else is only counted (D-03,
// T-09-04).
func TestFollowersPageSplit(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	target := testutil.SeedPerson(t, pool, "split-target@example.com", "split.target", true)
	viewer := testutil.SeedPerson(t, pool, "split-viewer@example.com", "split.viewer", false)

	named := testutil.SeedPerson(t, pool, "split-named@example.com", "split.named", true)
	private := testutil.SeedPerson(t, pool, "split-private@example.com", "split.private", false)
	pending := testutil.SeedPerson(t, pool, "split-pending@example.com", "split.pending", true)
	testutil.SeedUsername(t, pool, pending, "split.pending", "pending")
	blockedByViewer := testutil.SeedPerson(t, pool, "split-bbv@example.com", "split.bbv", true)
	testutil.SeedBlock(t, pool, viewer, blockedByViewer)
	blocksViewer := testutil.SeedPerson(t, pool, "split-bv@example.com", "split.bv", true)
	testutil.SeedBlock(t, pool, blocksViewer, viewer)

	for i, id := range []int64{named, private, pending, blockedByViewer, blocksViewer} {
		testutil.SeedFollow(t, pool, id, target, peopleNow.Add(time.Duration(i)*time.Minute))
	}

	counts, err := q.FollowCounts(ctx, target)
	if err != nil || counts.Followers != 5 {
		t.Fatalf("FollowCounts = %+v (%v), want 5 followers", counts, err)
	}
	page, err := q.FollowersPage(ctx, sqlcgen.FollowersPageParams{AccountID: target, ViewerID: viewer, RowLimit: 50})
	if err != nil {
		t.Fatalf("FollowersPage: %v", err)
	}
	if len(page) != 1 || page[0].Username != "split.named" {
		t.Fatalf("named followers = %+v, want only split.named", page)
	}
	n, err := q.CountNamedFollowers(ctx, sqlcgen.CountNamedFollowersParams{AccountID: target, ViewerID: viewer})
	if err != nil || n != 1 {
		t.Errorf("CountNamedFollowers = %d (%v), want 1", n, err)
	}

	// The same rules on the Following side: the target follows each of them.
	for i, id := range []int64{named, private, pending, blockedByViewer, blocksViewer} {
		testutil.SeedFollow(t, pool, target, id, peopleNow.Add(time.Duration(i)*time.Minute))
	}
	gp, err := q.FollowingPage(ctx, sqlcgen.FollowingPageParams{AccountID: target, ViewerID: viewer, RowLimit: 50})
	if err != nil {
		t.Fatalf("FollowingPage: %v", err)
	}
	if len(gp) != 1 || gp[0].Username != "split.named" {
		t.Errorf("named following = %+v, want only split.named", gp)
	}
	if n, err := q.CountNamedFollowing(ctx, sqlcgen.CountNamedFollowingParams{AccountID: target, ViewerID: viewer}); err != nil || n != 1 {
		t.Errorf("CountNamedFollowing = %d (%v), want 1", n, err)
	}
}

// TestFollowersPageKeyset proves 55 followers page as 50 then 5 with no repeat
// and a stable order, including followers who share one created_at.
func TestFollowersPageKeyset(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	target := testutil.SeedPerson(t, pool, "keyset-target@example.com", "keyset.target", true)
	viewer := testutil.SeedPerson(t, pool, "keyset-viewer@example.com", "keyset.viewer", false)
	for i := 0; i < 55; i++ {
		f := testutil.SeedPerson(t, pool, fmt.Sprintf("keyset-f%02d@example.com", i), fmt.Sprintf("kf%02d", i), true)
		// Pairs share a created_at, so the username tie break is exercised.
		testutil.SeedFollow(t, pool, f, target, peopleNow.Add(time.Duration(i/2)*time.Minute))
	}

	first, err := q.FollowersPage(ctx, sqlcgen.FollowersPageParams{AccountID: target, ViewerID: viewer, RowLimit: 50})
	if err != nil || len(first) != 50 {
		t.Fatalf("first page = %d rows (%v), want 50", len(first), err)
	}
	last := first[len(first)-1]
	afterUsername := last.Username
	second, err := q.FollowersPage(ctx, sqlcgen.FollowersPageParams{
		AccountID: target, ViewerID: viewer, RowLimit: 50,
		AfterAt:       pgtype.Timestamptz{Time: last.CreatedAt, Valid: true},
		AfterUsername: &afterUsername,
	})
	if err != nil || len(second) != 5 {
		t.Fatalf("second page = %d rows (%v), want 5", len(second), err)
	}
	seen := map[string]bool{}
	var prev *sqlcgen.FollowersPageRow
	for _, r := range append(append([]sqlcgen.FollowersPageRow{}, first...), second...) {
		if seen[r.Username] {
			t.Errorf("%s appears twice across the pages", r.Username)
		}
		seen[r.Username] = true
		if prev != nil {
			if r.CreatedAt.After(prev.CreatedAt) || (r.CreatedAt.Equal(prev.CreatedAt) && r.Username >= prev.Username) {
				t.Errorf("order broke between %s (%v) and %s (%v)", prev.Username, prev.CreatedAt, r.Username, r.CreatedAt)
			}
		}
		cur := r
		prev = &cur
	}
	if len(seen) != 55 {
		t.Errorf("distinct followers over both pages = %d, want 55", len(seen))
	}
}

// TestPublicProfilePathsForViewer proves the batch read leaves out private,
// pending, deleted and viewer blocking accounts (D-01, D-12).
func TestPublicProfilePathsForViewer(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	viewer := testutil.SeedPerson(t, pool, "paths-viewer@example.com", "paths.viewer", false)
	ok := testutil.SeedPerson(t, pool, "paths-ok@example.com", "paths.ok", true)
	viewerBlocked := testutil.SeedPerson(t, pool, "paths-vb@example.com", "paths.vb", true)
	testutil.SeedBlock(t, pool, viewer, viewerBlocked)
	private := testutil.SeedPerson(t, pool, "paths-private@example.com", "paths.private", false)
	pending := testutil.SeedPerson(t, pool, "paths-pending@example.com", "paths.pending", true)
	testutil.SeedUsername(t, pool, pending, "paths.pending", "pending")
	deleted := testutil.SeedPerson(t, pool, "paths-deleted@example.com", "paths.deleted", true)
	markDeleted(t, pool, deleted)
	blocking := testutil.SeedPerson(t, pool, "paths-blocking@example.com", "paths.blocking", true)
	testutil.SeedBlock(t, pool, blocking, viewer)

	rows, err := q.PublicProfilePathsForViewer(context.Background(), sqlcgen.PublicProfilePathsForViewerParams{
		AccountIds: []int64{ok, viewerBlocked, private, pending, deleted, blocking}, ViewerID: viewer,
	})
	if err != nil {
		t.Fatalf("PublicProfilePathsForViewer: %v", err)
	}
	got := map[int64]string{}
	for _, r := range rows {
		got[r.ID] = r.Username
	}
	want := map[int64]string{ok: "paths.ok", viewerBlocked: "paths.vb"}
	if len(got) != len(want) || got[ok] != "paths.ok" || got[viewerBlocked] != "paths.vb" {
		t.Errorf("paths = %v, want %v", got, want)
	}
}

// TestFolloweeStanding proves the sweep's read says who is still followable.
func TestFolloweeStanding(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	good := testutil.SeedPerson(t, pool, "stand-good@example.com", "stand.good", true)
	private := testutil.SeedPerson(t, pool, "stand-private@example.com", "stand.private", false)
	pending := testutil.SeedPerson(t, pool, "stand-pending@example.com", "stand.pending", true)
	testutil.SeedUsername(t, pool, pending, "stand.pending", "pending")
	deleted := testutil.SeedPerson(t, pool, "stand-deleted@example.com", "stand.deleted", true)
	markDeleted(t, pool, deleted)
	nameless := testutil.SeedReporterAccount(t, pool, "stand-nameless@example.com")
	testutil.MustExec(t, pool, `UPDATE accounts SET profile_public = true, profile_public_at = now() WHERE id = $1`, nameless)

	rows, err := q.FolloweeStanding(context.Background(), []int64{good, private, pending, deleted, nameless, 999999})
	if err != nil {
		t.Fatalf("FolloweeStanding: %v", err)
	}
	got := map[int64]bool{}
	for _, r := range rows {
		got[r.ID] = r.Followable
	}
	want := map[int64]bool{good: true, private: false, pending: false, deleted: false, nameless: false}
	if len(got) != len(want) {
		t.Fatalf("standing = %v, want %v (an unknown id has no row)", got, want)
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("account %d followable = %v, want %v", id, got[id], w)
		}
	}
}
