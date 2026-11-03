package service

// comments_block_test.go proves the block on the Phase 8 comment surfaces
// (Phase 9 plan 10, PROF-03, D-12, review M1, M2, M3): the list, the count, the
// Comment hidden placeholder, the mention and the notifications of a person the
// viewer blocked, and that the block runs one way only. It reuses the pcRig of
// pushcomments_test.go, which is a CommentService wired to the push dispatcher
// on a real database. Helpers carry the prefix blk.

import (
	"context"
	"fmt"
	"testing"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// blkPeople is a report with a reporter and four readers, all with a clear
// username: a blocks b, c and d block nobody.
type blkPeople struct {
	r                    *pcRig
	reporter, a, b, c, d int64
	report               int64
}

func newBlkPeople(t *testing.T, prefix string) *blkPeople {
	t.Helper()
	r := newPCRig(t)
	p := &blkPeople{r: r}
	p.reporter, p.report = r.reporter(prefix+"-rep", true)
	p.a, _ = r.reader(prefix+"-a", 1, 5, false, false, true)
	p.b, _ = r.reader(prefix+"-b", 1, 5, false, false, true)
	p.c, _ = r.reader(prefix+"-c", 1, 5, false, false, true)
	p.d, _ = r.reader(prefix+"-d", 1, 5, false, false, true)
	testutil.SeedUsername(t, r.pool, p.reporter, "reporter_r", "clear")
	testutil.SeedUsername(t, r.pool, p.a, "blocker_a", "clear")
	testutil.SeedUsername(t, r.pool, p.b, "blocked_b", "clear")
	testutil.SeedUsername(t, r.pool, p.c, "carol_c", "clear")
	testutil.SeedUsername(t, r.pool, p.d, "dave_d", "clear")
	return p
}

func (p *blkPeople) list(t *testing.T, report, viewer int64) CommentList {
	t.Helper()
	l, err := p.r.svc.List(context.Background(), report, viewer, 0)
	if err != nil {
		t.Fatalf("List(report %d, viewer %d) = %v", report, viewer, err)
	}
	return l
}

// blkIDs lists the ids of the roots, then for each root the ids of its replies,
// as "root" and "root>reply" words, so a whole list reads in one comparison.
func blkShape(l CommentList) []string {
	var out []string
	for _, c := range l.Comments {
		out = append(out, fmt.Sprintf("%d", c.ID))
		for _, rep := range c.Replies {
			out = append(out, fmt.Sprintf("%d>%d", c.ID, rep.ID))
		}
	}
	return out
}

func blkSame(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBlockHidesCommentsFromBlocker(t *testing.T) {
	p := newBlkPeople(t, "blk-list")
	r := p.r
	seed := func(account int64, parent, replyTo int64) int64 {
		return testutil.SeedComment(t, r.pool, testutil.CommentSeed{
			ReportID: p.report, AccountID: account, ParentID: parent, ReplyToID: replyTo,
		})
	}
	b1 := seed(p.b, 0, 0)   // b's root, answered by c
	c1 := seed(p.c, b1, b1) // c answers b's root
	b2 := seed(p.b, 0, 0)   // b's root, nobody answered
	c0 := seed(p.c, 0, 0)   // c's own root
	b3 := seed(p.b, c0, c0) // b answers c's root
	c2 := seed(p.c, c0, b3) // c answers b's reply
	testutil.SeedCommentLike(t, r.pool, b1, p.c)
	testutil.SeedCommentLike(t, r.pool, b1, p.d)
	testutil.SeedCommentLike(t, r.pool, b1, p.a)
	testutil.SeedCommentFlag(t, r.pool, b1, p.a)
	testutil.SeedBlock(t, r.pool, p.a, p.b)

	t.Run("the blocker sees a placeholder for the thread b started and none of b's other words", func(t *testing.T) {
		l := p.list(t, p.report, p.a)
		want := []string{fmt.Sprint(b1), fmt.Sprintf("%d>%d", b1, c1), fmt.Sprint(c0), fmt.Sprintf("%d>%d", c0, c2)}
		if got := blkShape(l); !blkSame(got, want) {
			t.Fatalf("list shape = %v, want %v (b2 and b3 left out, b1 a placeholder)", got, want)
		}
		ph := l.Comments[0]
		if !ph.Hidden || ph.Deleted || ph.Removed {
			t.Errorf("placeholder flags = hidden %v deleted %v removed %v, want hidden only", ph.Hidden, ph.Deleted, ph.Removed)
		}
		if ph.Words != "" || ph.Author != "" || ph.AuthorKind != "" || ph.ReplyToAuthor != "" {
			t.Errorf("placeholder shows words %q, author %q kind %q or mention %q, want none", ph.Words, ph.Author, ph.AuthorKind, ph.ReplyToAuthor)
		}
		if ph.IsReporter || ph.IsOwn || ph.CanDelete || ph.Held {
			t.Errorf("placeholder carries a flag: %+v", ph)
		}
		if ph.Likes != 0 || ph.ViewerLiked || ph.ViewerFlagged {
			t.Errorf("placeholder shows likes %d, liked %v, flagged %v, want 0 false false", ph.Likes, ph.ViewerLiked, ph.ViewerFlagged)
		}
		if got := l.Comments[1].ReplyCount; got != 1 {
			t.Errorf("c's thread reply_count = %d, want 1 (b's reply is not counted for the blocker)", got)
		}
	})

	t.Run("a reply to a blocked person names no one for the blocker, and names them for everyone else", func(t *testing.T) {
		la := p.list(t, p.report, p.a)
		if got := la.Comments[0].Replies[0].ReplyToAuthor; got != "" {
			t.Errorf("blocker sees mention %q on c's reply to the hidden root, want none (M3)", got)
		}
		ld := p.list(t, p.report, p.d)
		if got := ld.Comments[0].Replies[0].ReplyToAuthor; got != "blocked_b" {
			t.Errorf("another reader sees mention %q, want blocked_b", got)
		}
		lc := p.list(t, p.report, p.c)
		want := []string{fmt.Sprint(b1), fmt.Sprintf("%d>%d", b1, c1), fmt.Sprint(b2), fmt.Sprint(c0),
			fmt.Sprintf("%d>%d", c0, b3), fmt.Sprintf("%d>%d", c0, c2)}
		if got := blkShape(lc); !blkSame(got, want) {
			t.Errorf("a reader who blocked nobody sees %v, want %v", got, want)
		}
		if got := lc.Comments[0]; got.Hidden || got.Words == "" || got.Author != "blocked_b" || got.Likes != 3 {
			t.Errorf("a reader who blocked nobody sees b's root as %+v", got)
		}
	})

	t.Run("a blocked reporter's thread is a placeholder with no Reporter mark", func(t *testing.T) {
		rr := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: p.report, AccountID: p.reporter})
		testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: p.report, AccountID: p.d, ParentID: rr, ReplyToID: rr})
		before := p.list(t, p.report, p.c)
		var seen *CommentView
		for i := range before.Comments {
			if before.Comments[i].ID == rr {
				seen = &before.Comments[i]
			}
		}
		if seen == nil || !seen.IsReporter {
			t.Fatalf("a reader who blocked nobody does not see the reporter's comment with the Reporter mark: %+v", seen)
		}
		testutil.SeedBlock(t, r.pool, p.a, p.reporter)
		after := p.list(t, p.report, p.a)
		for _, c := range after.Comments {
			if c.ID != rr {
				continue
			}
			if !c.Hidden || c.IsReporter || c.Author != "" || c.Words != "" {
				t.Errorf("the blocked reporter's root = %+v, want a hidden placeholder with no Reporter mark", c)
			}
			return
		}
		t.Error("the blocked reporter's answered root is not listed as a placeholder")
	})
}

func TestBlockedCommentCount(t *testing.T) {
	p := newBlkPeople(t, "blk-count")
	r := p.r
	seed := func(account int64, parent int64) int64 {
		return testutil.SeedComment(t, r.pool, testutil.CommentSeed{
			ReportID: p.report, AccountID: account, ParentID: parent, ReplyToID: parent,
		})
	}
	b1 := seed(p.b, 0)
	seed(p.c, b1)
	seed(p.b, 0)
	c0 := seed(p.c, 0)
	seed(p.b, c0)
	seed(p.c, c0)
	testutil.SeedBlock(t, r.pool, p.a, p.b)

	extras := func(viewer int64) int {
		t.Helper()
		rows, err := r.q.ReportExtrasForReports(context.Background(), sqlcgen.ReportExtrasForReportsParams{
			ViewerAccountID: viewer, ReportIds: []int64{p.report},
		})
		if err != nil || len(rows) != 1 {
			t.Fatalf("ReportExtrasForReports(viewer %d) = %v, %v", viewer, rows, err)
		}
		return int(rows[0].CommentCount)
	}
	if got := extras(p.a); got != 3 {
		t.Errorf("the blocker's comment_count = %d, want 3 (c's three comments are not b's)", got)
	}
	for name, viewer := range map[string]int64{"c": p.c, "d": p.d, "b": p.b, "viewer 0": 0} {
		if got := extras(viewer); got != 6 {
			t.Errorf("%s comment_count = %d, want 6", name, got)
		}
	}
	// The count a reader sees on the page equals what the list shows of real
	// comments, placeholders left out, as TestCommentCountMatchesList pins.
	l := p.list(t, p.report, p.a)
	if l.Count != 3 {
		t.Errorf("the blocker's list count = %d, want 3", l.Count)
	}
	real := 0
	for _, c := range l.Comments {
		if !c.Hidden && !c.Deleted && !c.Removed {
			real++
		}
		for _, rep := range c.Replies {
			if !rep.Hidden && !rep.Deleted && !rep.Removed {
				real++
			}
		}
	}
	if real != l.Count {
		t.Errorf("real comments listed = %d, count = %d, want them equal", real, l.Count)
	}
	// A blocked person's comment never counts as the viewer having commented.
	var commented bool
	rows, err := r.q.ReportExtrasForReports(context.Background(), sqlcgen.ReportExtrasForReportsParams{
		ViewerAccountID: p.b, ReportIds: []int64{p.report},
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("ReportExtrasForReports(b) = %v, %v", rows, err)
	}
	commented = rows[0].ViewerCommented
	if !commented {
		t.Error("b's own viewer_commented = false, want true (the block touches the blocker's view only)")
	}
}

func TestBlockIsOneWayForComments(t *testing.T) {
	p := newBlkPeople(t, "blk-oneway")
	r := p.r
	a1 := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: p.report, AccountID: p.a})
	b1 := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: p.report, AccountID: p.b})
	var commentsBefore int
	if err := r.pool.QueryRow(context.Background(), `SELECT count(*) FROM comments`).Scan(&commentsBefore); err != nil {
		t.Fatalf("counting comments: %v", err)
	}
	testutil.SeedBlock(t, r.pool, p.a, p.b)

	lb := p.list(t, p.report, p.b)
	found := map[int64]CommentView{}
	for _, c := range lb.Comments {
		found[c.ID] = c
	}
	if got, ok := found[a1]; !ok || got.Hidden || got.Author != "blocker_a" || got.Words == "" {
		t.Errorf("the blocked person sees the blocker's comment as %+v (listed %v), want it with its name", got, ok)
	}
	if got, ok := found[b1]; !ok || !got.IsOwn || got.Words == "" {
		t.Errorf("the blocked person sees their own comment as %+v (listed %v), want it as theirs", got, ok)
	}
	ld := p.list(t, p.report, p.d)
	if len(ld.Comments) != 2 {
		t.Errorf("a third reader sees %d comments, want both", len(ld.Comments))
	}
	la := p.list(t, p.report, p.a)
	if len(la.Comments) != 1 || la.Comments[0].ID != a1 {
		t.Errorf("the blocker sees %v, want only their own comment (b's has no reply)", blkShape(la))
	}

	// Blocking changes no stored row: no comment is deleted or changed, and no
	// vote or rating table is touched (research Pitfall 10).
	var commentsAfter int
	if err := r.pool.QueryRow(context.Background(), `SELECT count(*) FROM comments`).Scan(&commentsAfter); err != nil {
		t.Fatalf("counting comments: %v", err)
	}
	if commentsAfter != commentsBefore {
		t.Errorf("comments rows %d -> %d, want the block to delete none", commentsBefore, commentsAfter)
	}
	if got := r.count(`SELECT count(*) FROM comments WHERE moderation_state <> 'clear' OR deleted_at IS NOT NULL`); got != 0 {
		t.Errorf("%d comments changed state after a block, want none", got)
	}
}

func TestBlockedCommentDoesNotNotify(t *testing.T) {
	const on, reply = "comment_on_yours", "reply_to_yours"

	t.Run("a blocked person's comment on your report writes no row and queues no note", func(t *testing.T) {
		p := newBlkPeople(t, "blk-note-on")
		r := p.r
		testutil.SeedBlock(t, r.pool, p.reporter, p.b)
		pokes := r.note.calls

		r.post(p.report, p.b, "words from the blocked person", nil)
		if got := r.count(`SELECT count(*) FROM updates WHERE account_id = $1 AND kind = $2`, p.reporter, on); got != 0 {
			t.Errorf("the blocker got %d comment_on_yours rows, want none", got)
		}
		if got := r.pushedKinds(p.report); len(got) != 0 {
			t.Errorf("changed push rows = %v, want none", got)
		}
		if got := r.count(`SELECT count(*) FROM push_outbox WHERE account_id = $1`, p.reporter); got != 0 {
			t.Errorf("push_outbox rows for the blocker = %d, want none", got)
		}
		if r.note.calls != pokes {
			t.Errorf("dispatcher pokes %d -> %d, want none for a row that was not written", pokes, r.note.calls)
		}

		r.post(p.report, p.c, "words from someone else", nil)
		if got := r.count(`SELECT count(*) FROM updates WHERE account_id = $1 AND kind = $2`, p.reporter, on); got != 1 {
			t.Errorf("after c's comment the blocker has %d rows, want 1", got)
		}
		// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box):
		// comment notes stay in the bell, so no changed phone row is queued.
		if got := r.pushedKinds(p.report); len(got) != 0 {
			t.Errorf("changed push rows = %v, want none (comment notes are bell only)", got)
		}
	})

	t.Run("a blocked person's reply to you writes no row", func(t *testing.T) {
		p := newBlkPeople(t, "blk-note-reply")
		r := p.r
		root := r.post(p.report, p.a, "my own words", nil)
		testutil.SeedBlock(t, r.pool, p.a, p.b)

		r.post(p.report, p.b, "an answer from the blocked person", &root.ID)
		if got := r.count(`SELECT count(*) FROM updates WHERE account_id = $1 AND kind = $2`, p.a, reply); got != 0 {
			t.Errorf("the blocker got %d reply_to_yours rows, want none", got)
		}
		for _, g := range r.pushedKinds(p.report) {
			if g == fmt.Sprintf("%d:%s", p.a, reply) {
				t.Errorf("a changed push was queued for the blocker: %v", r.pushedKinds(p.report))
			}
		}

		r.post(p.report, p.c, "an answer from someone else", &root.ID)
		if got := r.count(`SELECT count(*) FROM updates WHERE account_id = $1 AND kind = $2`, p.a, reply); got != 1 {
			t.Errorf("after c's reply the blocker has %d rows, want 1", got)
		}
	})

	t.Run("the block is one way: the person blocked still hears from the blocker", func(t *testing.T) {
		p := newBlkPeople(t, "blk-note-oneway")
		r := p.r
		testutil.SeedBlock(t, r.pool, p.b, p.reporter) // b blocks the reporter
		r.post(p.report, p.b, "b comments on the reporter's report", nil)
		if got := r.count(`SELECT count(*) FROM updates WHERE account_id = $1 AND kind = $2`, p.reporter, on); got != 1 {
			t.Errorf("the reporter, blocked by b, got %d rows for b's comment, want 1", got)
		}
	})
}
