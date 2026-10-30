// people_comment_block_test.go proves BlockCommentAuthor (quick 261030-u78,
// WR-E04, D-12): a reader can block the author of any comment they can see,
// private or pending commenters included, through the same block tail the
// profile Block uses, and every refusal is one ErrCommentGone that writes
// nothing and reveals nothing. The tests run on a real database and skip
// without DATABASE_URL.
//
// Every person here is made up and uses example.com.
package service_test

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

type blockCommentRig struct {
	t        *testing.T
	pool     *pgxpool.Pool
	q        *sqlcgen.Queries
	comments *service.CommentService
	people   *service.PeopleService
	now      time.Time
	seq      int
}

func newBlockCommentRig(t *testing.T) *blockCommentRig {
	t.Helper()
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	comments := service.NewCommentService(q)
	return &blockCommentRig{
		t: t, pool: pool, q: q, comments: comments, now: time.Now(),
		people: service.NewPeopleService(q, service.WithPeopleCommentAuthors(comments)),
	}
}

// reporter makes an account that owns a session, so a report can be posted by it.
func (r *blockCommentRig) reporter(name string) (int64, string) {
	r.t.Helper()
	r.seq++
	session := fmt.Sprintf("u78-%s-%d", name, r.seq)
	id := testutil.SeedReporterAccount(r.t, r.pool, fmt.Sprintf("u78-%s-%d@example.com", name, r.seq), session)
	return id, session
}

// report stores a flood report on the Benz Circle point. expiresIn is added to
// now, so a negative value gives a faded report.
func (r *blockCommentRig) report(session, category, moderation string, expiresIn time.Duration) int64 {
	r.t.Helper()
	return testutil.InsertStoredReport(r.t, r.pool, testutil.StoredReport{
		SessionID: session, Category: category, Severity: "low", Description: "water",
		Latitude: 16.5062, Longitude: 80.6480,
		CreatedAt: r.now.Add(expiresIn - time.Hour), ExpiresAt: r.now.Add(expiresIn),
		ModerationState: moderation,
	})
}

func (r *blockCommentRig) blockRows() int {
	r.t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM person_blocks`).Scan(&n); err != nil {
		r.t.Fatalf("counting blocks: %v", err)
	}
	return n
}

func (r *blockCommentRig) hasBlock(blocker, blocked int64) bool {
	r.t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM person_blocks WHERE blocker_id = $1 AND blocked_id = $2`, blocker, blocked).Scan(&n); err != nil {
		r.t.Fatalf("reading a block: %v", err)
	}
	return n == 1
}

// words lists every words string of a viewer's comment list, replies included.
func (r *blockCommentRig) words(reportID, viewer int64) []string {
	r.t.Helper()
	list, err := r.comments.List(context.Background(), reportID, viewer, 0)
	if err != nil {
		r.t.Fatalf("List: %v", err)
	}
	var out []string
	var walk func(cs []service.CommentView)
	walk = func(cs []service.CommentView) {
		for _, c := range cs {
			if c.Words != "" {
				out = append(out, c.Words)
			}
			walk(c.Replies)
		}
	}
	walk(list.Comments)
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestBlockCommentAuthorBlocksAnyCommenter(t *testing.T) {
	ctx := context.Background()
	r := newBlockCommentRig(t)
	viewer, session := r.reporter("viewer")
	other, _ := r.reporter("other")
	report := r.report(session, "flood", "clear", time.Hour)

	t.Run("a private commenter, and the words leave the viewer's list only", func(t *testing.T) {
		private := testutil.SeedPerson(t, r.pool, "u78-private@example.com", "private.one", false)
		c := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: report, AccountID: private, Body: "private words"})
		if !contains(r.words(report, viewer), "private words") {
			t.Fatal("the comment is not shown before the block")
		}
		if err := r.people.BlockCommentAuthor(ctx, viewer, c); err != nil {
			t.Fatalf("BlockCommentAuthor: %v", err)
		}
		if !r.hasBlock(viewer, private) {
			t.Fatal("no person_blocks row from the viewer to the commenter")
		}
		if contains(r.words(report, viewer), "private words") {
			t.Error("the blocker still sees the blocked person's words")
		}
		if !contains(r.words(report, other), "private words") {
			t.Error("another reader lost the comment")
		}
		before := r.blockRows()
		if err := r.people.BlockCommentAuthor(ctx, viewer, c); err != nil {
			t.Fatalf("second BlockCommentAuthor: %v", err)
		}
		if got := r.blockRows(); got != before {
			t.Errorf("a repeat changed the block rows from %d to %d", before, got)
		}
	})

	t.Run("a commenter whose username is still pending", func(t *testing.T) {
		pending := testutil.SeedReporterAccount(t, r.pool, "u78-pending@example.com")
		testutil.SeedUsername(t, r.pool, pending, "pending.one", service.UsernameScreenPending)
		c := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: report, AccountID: pending})
		if err := r.people.BlockCommentAuthor(ctx, viewer, c); err != nil {
			t.Fatalf("BlockCommentAuthor: %v", err)
		}
		if !r.hasBlock(viewer, pending) {
			t.Error("no block row for a pending username")
		}
	})

	t.Run("a commenter who blocked the viewer answers like any other", func(t *testing.T) {
		blocker := testutil.SeedPerson(t, r.pool, "u78-blocker@example.com", "blocker.one", true)
		testutil.SeedBlock(t, r.pool, blocker, viewer)
		c := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: report, AccountID: blocker})
		if err := r.people.BlockCommentAuthor(ctx, viewer, c); err != nil {
			t.Fatalf("BlockCommentAuthor: %v", err)
		}
		if !r.hasBlock(viewer, blocker) {
			t.Error("the viewer's own block row is missing")
		}
	})

	t.Run("a public Reliable commenter, follows end both ways", func(t *testing.T) {
		pub := testutil.SeedPublicReporter(t, r.pool, "u78-public@example.com", "public.one", "u78-public-session", r.now)
		testutil.SeedFollow(t, r.pool, viewer, pub, r.now.Add(-time.Hour))
		testutil.SeedFollow(t, r.pool, pub, viewer, r.now.Add(-time.Hour))
		c := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: report, AccountID: pub})
		if err := r.people.BlockCommentAuthor(ctx, viewer, c); err != nil {
			t.Fatalf("BlockCommentAuthor: %v", err)
		}
		for _, p := range [][2]int64{{viewer, pub}, {pub, viewer}} {
			exists, ended, reason := followRow(t, r.pool, p[0], p[1])
			if !exists || !ended || reason != string(service.FollowEndBlocked) {
				t.Errorf("follow %d -> %d: exists=%v ended=%v reason=%q, want ended with reason blocked", p[0], p[1], exists, ended, reason)
			}
		}
	})

	t.Run("a faded report inside the history window still allows it", func(t *testing.T) {
		faded := r.report(session, "flood", "clear", -time.Hour)
		who := testutil.SeedPerson(t, r.pool, "u78-faded@example.com", "faded.one", false)
		c := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: faded, AccountID: who})
		if err := r.people.BlockCommentAuthor(ctx, viewer, c); err != nil {
			t.Fatalf("BlockCommentAuthor on a closed report: %v", err)
		}
		if !r.hasBlock(viewer, who) {
			t.Error("no block row on a closed report")
		}
	})
}

func TestBlockCommentAuthorUniformRefusal(t *testing.T) {
	ctx := context.Background()
	r := newBlockCommentRig(t)
	viewer, session := r.reporter("viewer")
	live := r.report(session, "flood", "clear", time.Hour)
	author := testutil.SeedPerson(t, r.pool, "u78-author@example.com", "author.one", true)

	// own, held, and the author's follow that must survive a refusal.
	testutil.SeedFollow(t, r.pool, viewer, author, r.now.Add(-time.Hour))

	deletedAccount := testutil.SeedPerson(t, r.pool, "u78-gone@example.com", "gone.one", true)
	testutil.MustExec(t, r.pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, deletedAccount)

	withheld := r.report(session, "flood", "removed", time.Hour)
	old := testutil.InsertStoredReport(t, r.pool, testutil.StoredReport{
		SessionID: session, Category: "flood", Severity: "low", Description: "water",
		Latitude: 16.5062, Longitude: 80.6480,
		CreatedAt: r.now.Add(-9 * 24 * time.Hour), ExpiresAt: r.now.Add(-8 * 24 * time.Hour),
	})
	rescue := r.report(session, string(service.CategoryRescueNeeded), "clear", -time.Hour)

	cases := []struct {
		name string
		id   func() int64
	}{
		{"an unknown comment", func() int64 { return 987654321 }},
		{"the viewer's own comment", func() int64 {
			return testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: live, AccountID: viewer})
		}},
		{"a deleted comment", func() int64 {
			return testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: live, AccountID: author, Deleted: true})
		}},
		{"a removed comment", func() int64 {
			return testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: live, AccountID: author, State: "removed"})
		}},
		{"another person's held comment", func() int64 {
			return testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: live, AccountID: author, State: "pending"})
		}},
		{"a comment on a withheld report", func() int64 {
			return testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: withheld, AccountID: author})
		}},
		{"a comment on a report that ended before the history window", func() int64 {
			return testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: old, AccountID: author})
		}},
		{"a comment on a finished Rescue report", func() int64 {
			return testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rescue, AccountID: author})
		}},
		{"a comment by a deleted account", func() int64 {
			return testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: live, AccountID: deletedAccount})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := tc.id()
			before := r.blockRows()
			err := r.people.BlockCommentAuthor(ctx, viewer, id)
			if !errors.Is(err, service.ErrCommentGone) {
				t.Fatalf("err = %v, want ErrCommentGone", err)
			}
			if got := r.blockRows(); got != before {
				t.Errorf("block rows went from %d to %d on a refusal", before, got)
			}
			if !activeFollows(t, r.pool, viewer, author) {
				t.Error("a refusal ended a follow")
			}
		})
	}

	t.Run("a service built without a comment source fails closed", func(t *testing.T) {
		c := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: live, AccountID: author})
		bare := service.NewPeopleService(r.q)
		before := r.blockRows()
		if err := bare.BlockCommentAuthor(ctx, viewer, c); !errors.Is(err, service.ErrCommentGone) {
			t.Fatalf("err = %v, want ErrCommentGone", err)
		}
		if got := r.blockRows(); got != before {
			t.Errorf("block rows went from %d to %d", before, got)
		}
	})
}

func TestBlockCommentAuthorStopsCommentNotes(t *testing.T) {
	ctx := context.Background()
	r := newBlockCommentRig(t)
	reporter, session := r.reporter("reporter")
	report := r.report(session, "flood", "clear", time.Hour)
	commenter := testutil.SeedPerson(t, r.pool, "u78-note-commenter@example.com", "note.commenter", false)
	c := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: report, AccountID: commenter})

	holds := func() bool {
		t.Helper()
		got, err := r.q.CommentPushStillHolds(ctx, sqlcgen.CommentPushStillHoldsParams{
			ReportID: report, RecipientAccountID: reporter, Kind: string(service.UpdateCommentOnYours),
		})
		if err != nil {
			t.Fatalf("CommentPushStillHolds: %v", err)
		}
		return got
	}
	if !holds() {
		t.Fatal("the comment note does not hold before the block")
	}
	if err := r.people.BlockCommentAuthor(ctx, reporter, c); err != nil {
		t.Fatalf("BlockCommentAuthor: %v", err)
	}
	if holds() {
		t.Error("the comment note still holds after the reporter blocked the commenter from the comment")
	}
}
