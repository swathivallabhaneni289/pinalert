package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// These tests prove the Phase 8 plan 08 store guarantees against a real
// Postgres: the one query that decides what a viewer sees of a report's
// comments (A-P8-2, D-12, D-14), the real comment count that matches it (D-03),
// the username rule (A-P8-10) and the one like and one flag per account. They
// skip without DATABASE_URL via testutil.NewTestDB. Helpers carry the prefix
// cmt so they never collide with helpers in the other test files.

// cmtWorld is one seeded report with the accounts the cases need. Every case
// starts from a truncated database.
type cmtWorld struct {
	pool     *pgxpool.Pool
	q        *sqlcgen.Queries
	report   int64
	reporter int64 // wrote the report, so is_reporter is true for its comments
	alice    int64 // username alice_w, screen clear
	bob      int64 // username bobby_p, screen pending
	carol    int64 // username carol_c, screen clear
}

func cmtNewWorld(t *testing.T, pool *pgxpool.Pool) cmtWorld {
	t.Helper()
	testutil.Truncate(t, pool)
	w := cmtWorld{pool: pool, q: sqlcgen.New(pool)}
	w.reporter = testutil.SeedReporterAccount(t, pool, "cmt-reporter@example.com", "cmt-reporter-session")
	w.alice = testutil.SeedReporterAccount(t, pool, "cmt-alice@example.com")
	w.bob = testutil.SeedReporterAccount(t, pool, "cmt-bob@example.com")
	w.carol = testutil.SeedReporterAccount(t, pool, "cmt-carol@example.com")
	testutil.SeedUsername(t, pool, w.reporter, "reporter_r", "clear")
	testutil.SeedUsername(t, pool, w.alice, "alice_w", "clear")
	testutil.SeedUsername(t, pool, w.bob, "bobby_p", "pending")
	testutil.SeedUsername(t, pool, w.carol, "carol_c", "clear")
	w.report = testutil.SeedHistoryReport(t, pool, "cmt-reporter-session", "low", "flood", time.Now().Add(time.Hour))
	return w
}

func (w cmtWorld) comment(t *testing.T, c testutil.CommentSeed) int64 {
	t.Helper()
	c.ReportID = w.report
	return testutil.SeedComment(t, w.pool, c)
}

// list reads the comments of the world's report for viewer with generous caps.
func (w cmtWorld) list(t *testing.T, viewer int64) map[int64]sqlcgen.CommentsForReportRow {
	t.Helper()
	rows := w.listRows(t, viewer, 0, 100, 1000)
	byID := make(map[int64]sqlcgen.CommentsForReportRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	return byID
}

func (w cmtWorld) listRows(t *testing.T, viewer, after int64, rootCap, replyCap int32) []sqlcgen.CommentsForReportRow {
	t.Helper()
	rows, err := w.q.CommentsForReport(context.Background(), sqlcgen.CommentsForReportParams{
		ViewerAccountID: viewer,
		ReportID:        w.report,
		AfterID:         after,
		RootCap:         rootCap,
		ReplyCap:        replyCap,
	})
	if err != nil {
		t.Fatalf("CommentsForReport viewer %d: %v", viewer, err)
	}
	return rows
}

func (w cmtWorld) extras(t *testing.T, viewer int64) sqlcgen.ReportExtrasForReportsRow {
	t.Helper()
	rows, err := w.q.ReportExtrasForReports(context.Background(), sqlcgen.ReportExtrasForReportsParams{
		ReportIds:       []int64{w.report},
		ViewerAccountID: viewer,
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("ReportExtrasForReports viewer %d: %d rows, err %v", viewer, len(rows), err)
	}
	return rows[0]
}

func cmtStr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestCommentsForReportVisibility(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("a clear comment is listed for everyone with its author name", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		id := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
		for _, viewer := range []int64{0, w.carol, w.alice} {
			got, ok := w.list(t, viewer)[id]
			if !ok {
				t.Fatalf("viewer %d: comment not listed", viewer)
			}
			if got.AuthorKind != "user" || got.AuthorUsername == nil || *got.AuthorUsername != "alice_w" {
				t.Errorf("viewer %d: author = %q %s, want user alice_w", viewer, got.AuthorKind, cmtStr(got.AuthorUsername))
			}
			if got.Gone != "" || got.Held || got.Body == "" {
				t.Errorf("viewer %d: gone %q held %v body %q, want a live shown comment", viewer, got.Gone, got.Held, got.Body)
			}
			if got.ParentID != nil || got.ReplyToID != nil || got.ReplyToAuthor != nil {
				t.Errorf("viewer %d: a top level comment has no parent, reply_to or reply_to_author", viewer)
			}
		}
	})

	t.Run("a pending comment is listed for its author only", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		id := w.comment(t, testutil.CommentSeed{AccountID: w.alice, State: "pending"})
		own, ok := w.list(t, w.alice)[id]
		if !ok || !own.Held {
			t.Fatalf("the author must see the held comment with held true, got listed %v held %v", ok, own.Held)
		}
		for _, viewer := range []int64{0, w.carol, w.bob} {
			if _, ok := w.list(t, viewer)[id]; ok {
				t.Errorf("viewer %d must not see a held comment", viewer)
			}
		}
	})

	t.Run("a deleted comment with a shown reply is a placeholder, without one it is gone", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		withReply := w.comment(t, testutil.CommentSeed{AccountID: w.alice, Deleted: true})
		reply := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: withReply})
		alone := w.comment(t, testutil.CommentSeed{AccountID: w.alice, Deleted: true})

		got := w.list(t, w.carol)
		ph, ok := got[withReply]
		if !ok {
			t.Fatal("a deleted root with a shown reply must be listed as a placeholder")
		}
		if ph.Gone != "deleted" || ph.Body != "" || ph.AuthorUsername != nil || ph.AuthorKind != "" {
			t.Errorf("placeholder = gone %q body %q author %s kind %q, want deleted, empty, nil, empty", ph.Gone, ph.Body, cmtStr(ph.AuthorUsername), ph.AuthorKind)
		}
		if _, ok := got[reply]; !ok {
			t.Error("the reply under a placeholder must be listed")
		}
		if _, ok := got[alone]; ok {
			t.Error("a deleted root with no shown reply must not be listed")
		}
	})

	t.Run("a placeholder carries nothing that says who wrote it (review W4)", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		// The deleted comment was the reporter's own, was liked and was flagged
		// by the viewer before it was deleted.
		root := w.comment(t, testutil.CommentSeed{AccountID: w.reporter, Deleted: true})
		testutil.SeedCommentLike(t, w.pool, root, w.carol)
		testutil.SeedCommentLike(t, w.pool, root, w.alice)
		testutil.SeedCommentFlag(t, w.pool, root, w.carol)
		w.comment(t, testutil.CommentSeed{AccountID: w.alice, ParentID: root})

		ph := w.list(t, w.carol)[root]
		if ph.Gone != "deleted" {
			t.Fatalf("gone = %q, want deleted", ph.Gone)
		}
		if ph.IsReporter || ph.Likes != 0 || ph.ViewerLiked || ph.ViewerFlagged || ph.ReplyToAuthor != nil || ph.AccountID != 0 {
			t.Errorf("placeholder leaks: is_reporter %v likes %d liked %v flagged %v reply_to_author %s account_id %d",
				ph.IsReporter, ph.Likes, ph.ViewerLiked, ph.ViewerFlagged, cmtStr(ph.ReplyToAuthor), ph.AccountID)
		}
	})

	t.Run("a removed root with a shown reply is a removed placeholder", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		root := w.comment(t, testutil.CommentSeed{AccountID: w.alice, State: "removed"})
		w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root})
		ph, ok := w.list(t, 0)[root]
		if !ok || ph.Gone != "removed" || ph.Body != "" || ph.AuthorUsername != nil || ph.AuthorKind != "" {
			t.Errorf("removed placeholder = listed %v gone %q body %q kind %q", ok, ph.Gone, ph.Body, ph.AuthorKind)
		}
	})

	t.Run("a reply is listed only when it is shown", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		root := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
		held := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root, State: "pending"})
		gone := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root, Deleted: true})
		removed := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root, State: "removed"})
		shown := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root})

		other := w.list(t, w.alice)
		for id, label := range map[int64]string{held: "held", gone: "deleted", removed: "removed"} {
			if _, ok := other[id]; ok {
				t.Errorf("a %s reply must not be listed for another viewer", label)
			}
		}
		if r, ok := other[shown]; !ok || r.ParentID == nil || *r.ParentID != root {
			t.Errorf("the shown reply must be listed under its root, got %v", ok)
		}
		if _, ok := w.list(t, w.carol)[held]; !ok {
			t.Error("the author of a held reply must see it")
		}
	})

	t.Run("reply_to_author names the answered author only when everything is clear (review W3)", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		dave := testutil.SeedReporterAccount(t, pool, "cmt-dave@example.com")
		testutil.SeedUsername(t, pool, dave, "dave_d", "clear")
		eve := testutil.SeedReporterAccount(t, pool, "cmt-eve@example.com")
		testutil.SeedUsername(t, pool, eve, "eve_e", "clear")

		root := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
		ansClear := w.comment(t, testutil.CommentSeed{AccountID: dave, ParentID: root})
		ansPendingScreen := w.comment(t, testutil.CommentSeed{AccountID: w.bob, ParentID: root})
		ansDeleted := w.comment(t, testutil.CommentSeed{AccountID: dave, ParentID: root})
		ansRemoved := w.comment(t, testutil.CommentSeed{AccountID: dave, ParentID: root})
		ansDeletedAccount := w.comment(t, testutil.CommentSeed{AccountID: eve, ParentID: root})

		toRoot := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root, ReplyToID: root})
		toClear := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root, ReplyToID: ansClear})
		toPendingScreen := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root, ReplyToID: ansPendingScreen})
		toDeleted := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root, ReplyToID: ansDeleted})
		toRemoved := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root, ReplyToID: ansRemoved})
		toDeletedAccount := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root, ReplyToID: ansDeletedAccount})

		// The answered comments change after the replies were written.
		testutil.MustExec(t, pool, `UPDATE comments SET body = '', deleted_at = now() WHERE id = $1`, ansDeleted)
		testutil.MustExec(t, pool, `UPDATE comments SET moderation_state = 'removed' WHERE id = $1`, ansRemoved)
		testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = now(), username = NULL, username_screen = NULL WHERE id = $1`, eve)

		got := w.list(t, w.alice)
		want := map[int64]string{
			toRoot:           "alice_w",
			toClear:          "dave_d",
			toPendingScreen:  "<nil>",
			toDeleted:        "<nil>",
			toRemoved:        "<nil>",
			toDeletedAccount: "<nil>",
		}
		for id, name := range want {
			r, ok := got[id]
			if !ok {
				t.Errorf("reply %d not listed", id)
				continue
			}
			if cmtStr(r.ReplyToAuthor) != name {
				t.Errorf("reply %d reply_to_author = %s, want %s", id, cmtStr(r.ReplyToAuthor), name)
			}
		}
		if r := got[root]; r.ReplyToAuthor != nil {
			t.Errorf("a root has no reply_to_author, got %s", cmtStr(r.ReplyToAuthor))
		}
	})

	t.Run("reply_to_author names a clear author in the plain case", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		root := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
		reply := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: root, ReplyToID: root})
		got := w.list(t, w.carol)[reply]
		if cmtStr(got.ReplyToAuthor) != "alice_w" {
			t.Errorf("reply_to_author = %s, want alice_w", cmtStr(got.ReplyToAuthor))
		}
	})

	t.Run("a username that is not screened clear is New member to others", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		id := w.comment(t, testutil.CommentSeed{AccountID: w.bob})
		for _, viewer := range []int64{0, w.alice} {
			got := w.list(t, viewer)[id]
			if got.AuthorKind != "new_member" || got.AuthorUsername != nil {
				t.Errorf("viewer %d: author = %q %s, want new_member and no name", viewer, got.AuthorKind, cmtStr(got.AuthorUsername))
			}
		}
		own := w.list(t, w.bob)[id]
		if own.AuthorKind != "user" || own.AuthorUsername == nil || *own.AuthorUsername != "bobby_p" {
			t.Errorf("the author sees %q %s, want user bobby_p", own.AuthorKind, cmtStr(own.AuthorUsername))
		}
	})

	t.Run("a deleted account reads deleted_account", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		id := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
		testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = now(), username = NULL, username_screen = NULL WHERE id = $1`, w.alice)
		got := w.list(t, w.carol)[id]
		if got.AuthorKind != "deleted_account" || got.AuthorUsername != nil {
			t.Errorf("author = %q %s, want deleted_account and no name", got.AuthorKind, cmtStr(got.AuthorUsername))
		}
	})

	t.Run("is_reporter is true for the reporter's comment only", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		mine := w.comment(t, testutil.CommentSeed{AccountID: w.reporter})
		theirs := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
		got := w.list(t, w.carol)
		if !got[mine].IsReporter || got[theirs].IsReporter {
			t.Errorf("is_reporter = %v for the reporter and %v for another, want true and false", got[mine].IsReporter, got[theirs].IsReporter)
		}
	})

	t.Run("likes and the viewer's own like and flag", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		id := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
		testutil.SeedCommentLike(t, w.pool, id, w.carol)
		testutil.SeedCommentLike(t, w.pool, id, w.bob)
		testutil.SeedCommentFlag(t, w.pool, id, w.carol)
		byCarol := w.list(t, w.carol)[id]
		if byCarol.Likes != 2 || !byCarol.ViewerLiked || !byCarol.ViewerFlagged {
			t.Errorf("carol sees likes %d liked %v flagged %v, want 2 true true", byCarol.Likes, byCarol.ViewerLiked, byCarol.ViewerFlagged)
		}
		byAlice := w.list(t, w.alice)[id]
		if byAlice.Likes != 2 || byAlice.ViewerLiked || byAlice.ViewerFlagged {
			t.Errorf("alice sees likes %d liked %v flagged %v, want 2 false false", byAlice.Likes, byAlice.ViewerLiked, byAlice.ViewerFlagged)
		}
	})

	t.Run("caps and paging", func(t *testing.T) {
		w := cmtNewWorld(t, pool)
		r1 := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
		r2 := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
		r3 := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
		a1 := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: r1})
		a2 := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: r1})
		a3 := w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: r3})

		ids := func(rows []sqlcgen.CommentsForReportRow) []int64 {
			out := make([]int64, 0, len(rows))
			for _, r := range rows {
				out = append(out, r.ID)
			}
			return out
		}
		same := func(a, b []int64) bool {
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
		if got, want := ids(w.listRows(t, 0, 0, 2, 1000)), []int64{r1, a1, a2, r2}; !same(got, want) {
			t.Errorf("root_cap 2 = %v, want %v (two roots, the replies of those only, thread order)", got, want)
		}
		if got, want := ids(w.listRows(t, 0, r2, 5, 1000)), []int64{r3, a3};!same(got, want) {
			t.Errorf("after_id = %v, want %v", got, want)
		}
		if got := ids(w.listRows(t, 0, 0, 5, 1)); len(got) != 4 {
			t.Errorf("reply_cap 1 = %v, want 3 roots and 1 reply", got)
		}
	})

}

func TestCommentCountMatchesList(t *testing.T) {
	pool := testutil.NewTestDB(t)
	w := cmtNewWorld(t, pool)

	// A seeded mix of every state. Clear and live: rootA, replyA1, replyA2,
	// rootB (4). Everything else must not be counted.
	rootA := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
	w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: rootA})
	w.comment(t, testutil.CommentSeed{AccountID: w.bob, ParentID: rootA})
	w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: rootA, State: "pending"})
	w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: rootA, State: "removed"})
	w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: rootA, Deleted: true})
	w.comment(t, testutil.CommentSeed{AccountID: w.carol})
	w.comment(t, testutil.CommentSeed{AccountID: w.alice, State: "pending"})
	w.comment(t, testutil.CommentSeed{AccountID: w.alice, State: "removed"})
	w.comment(t, testutil.CommentSeed{AccountID: w.alice, Deleted: true})
	deletedWithReply := w.comment(t, testutil.CommentSeed{AccountID: w.alice, Deleted: true})
	w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: deletedWithReply})
	removedWithReply := w.comment(t, testutil.CommentSeed{AccountID: w.alice, State: "removed"})
	w.comment(t, testutil.CommentSeed{AccountID: w.carol, ParentID: removedWithReply})

	listed := 0
	for _, r := range w.listRows(t, 0, 0, 100, 1000) {
		if r.Gone == "" {
			listed++
		}
	}
	count := int(w.extras(t, 0).CommentCount)
	if count != listed {
		t.Errorf("comment_count %d, but viewer 0 lists %d non placeholder rows", count, listed)
	}
	// rootA, 2 clear replies, the other clear root, 1 reply under the deleted
	// root and 1 under the removed root.
	if count != 6 {
		t.Errorf("comment_count = %d, want 6", count)
	}
	// Every reader gets the same number, a held comment never counts, not even
	// for its author.
	for _, viewer := range []int64{w.alice, w.carol, w.bob} {
		if got := int(w.extras(t, viewer).CommentCount); got != count {
			t.Errorf("viewer %d: comment_count %d, want %d", viewer, got, count)
		}
	}
}

func TestCommentCountExtras(t *testing.T) {
	pool := testutil.NewTestDB(t)
	w := cmtNewWorld(t, pool)

	onlyDeleted := w.alice
	onlyRemoved := w.bob
	onlyPending := w.reporter
	hasClear := w.carol
	w.comment(t, testutil.CommentSeed{AccountID: onlyDeleted, Deleted: true})
	w.comment(t, testutil.CommentSeed{AccountID: onlyRemoved, State: "removed"})
	w.comment(t, testutil.CommentSeed{AccountID: onlyPending, State: "pending"})
	w.comment(t, testutil.CommentSeed{AccountID: hasClear})

	cases := []struct {
		label  string
		viewer int64
		want   bool
	}{
		{"a viewer with only a deleted comment", onlyDeleted, false},
		{"a viewer with only a removed comment", onlyRemoved, false},
		{"a viewer with only a held comment", onlyPending, false},
		{"a viewer with a clear comment", hasClear, true},
		{"viewer 0", 0, false},
	}
	for _, c := range cases {
		if got := w.extras(t, c.viewer).ViewerCommented; got != c.want {
			t.Errorf("%s: viewer_commented = %v, want %v", c.label, got, c.want)
		}
	}
	if got := w.extras(t, 0).CommentCount; got != 1 {
		t.Errorf("comment_count = %d, want 1 (only the clear comment)", got)
	}

	// A report with no comments reports 0 and false.
	other := testutil.SeedHistoryReport(t, pool, "cmt-reporter-session", "low", "flood", time.Now().Add(time.Hour))
	rows, err := w.q.ReportExtrasForReports(context.Background(), sqlcgen.ReportExtrasForReportsParams{
		ReportIds: []int64{other}, ViewerAccountID: hasClear,
	})
	if err != nil || len(rows) != 1 || rows[0].CommentCount != 0 || rows[0].ViewerCommented {
		t.Errorf("a report with no comments: %+v err %v, want count 0 and not commented", rows, err)
	}
}

func TestCommentLikeOnePerAccount(t *testing.T) {
	pool := testutil.NewTestDB(t)
	w := cmtNewWorld(t, pool)
	ctx := context.Background()
	id := w.comment(t, testutil.CommentSeed{AccountID: w.alice})

	for i, want := range []int64{1, 0} {
		n, err := w.q.LikeComment(ctx, sqlcgen.LikeCommentParams{CommentID: id, AccountID: w.carol})
		if err != nil || n != want {
			t.Fatalf("like %d: %d rows, err %v, want %d", i+1, n, err, want)
		}
	}
	if _, err := w.q.LikeComment(ctx, sqlcgen.LikeCommentParams{CommentID: id, AccountID: w.bob}); err != nil {
		t.Fatalf("a second account's like: %v", err)
	}
	state, err := w.q.CommentLikeState(ctx, sqlcgen.CommentLikeStateParams{CommentID: id, AccountID: w.carol})
	if err != nil || state.Likes != 2 || !state.ViewerLiked {
		t.Errorf("like state = %+v err %v, want 2 likes and viewer_liked", state, err)
	}
	other, err := w.q.CommentLikeState(ctx, sqlcgen.CommentLikeStateParams{CommentID: id, AccountID: w.alice})
	if err != nil || other.Likes != 2 || other.ViewerLiked {
		t.Errorf("another viewer's like state = %+v err %v, want 2 likes and not liked", other, err)
	}
	for i, want := range []int64{1, 0} {
		n, err := w.q.UnlikeComment(ctx, sqlcgen.UnlikeCommentParams{CommentID: id, AccountID: w.carol})
		if err != nil || n != want {
			t.Fatalf("unlike %d: %d rows, err %v, want %d", i+1, n, err, want)
		}
	}
}

func TestCommentFlagOnePerAccount(t *testing.T) {
	pool := testutil.NewTestDB(t)
	w := cmtNewWorld(t, pool)
	ctx := context.Background()
	id := w.comment(t, testutil.CommentSeed{AccountID: w.alice})
	note := "this is not true"

	first, err := w.q.InsertCommentFlag(ctx, sqlcgen.InsertCommentFlagParams{
		CommentID: id, AccountID: w.carol, Reasons: []string{"spam", "false"}, Note: &note,
	})
	if err != nil || first == 0 {
		t.Fatalf("first flag: id %d err %v", first, err)
	}
	_, err = w.q.InsertCommentFlag(ctx, sqlcgen.InsertCommentFlagParams{
		CommentID: id, AccountID: w.carol, Reasons: []string{"other"},
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("a second flag by the same account: want pgx.ErrNoRows, got %v", err)
	}
	if _, err := w.q.InsertCommentFlag(ctx, sqlcgen.InsertCommentFlagParams{
		CommentID: id, AccountID: w.bob, Reasons: []string{"abusive"},
	}); err != nil {
		t.Errorf("a different account may flag: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM comment_flags WHERE comment_id = $1`, id).Scan(&n); err != nil || n != 2 {
		t.Errorf("flag rows = %d (err %v), want 2", n, err)
	}
}

// TestCommentWriteQueries covers the plain write and read queries the service
// builds on: InsertComment, CommentTarget and MarkCommentDeleted.
func TestCommentWriteQueries(t *testing.T) {
	pool := testutil.NewTestDB(t)
	w := cmtNewWorld(t, pool)
	ctx := context.Background()

	root, err := w.q.InsertComment(ctx, sqlcgen.InsertCommentParams{
		ReportID: w.report, AccountID: w.alice, Body: "the root", ModerationState: "clear",
	})
	if err != nil || root.ID == 0 || root.CreatedAt.IsZero() {
		t.Fatalf("InsertComment: %+v err %v", root, err)
	}
	parent := root.ID
	held, err := w.q.InsertComment(ctx, sqlcgen.InsertCommentParams{
		ReportID: w.report, AccountID: w.bob, ParentID: &parent, ReplyToID: &parent, Body: "waiting",
		ModerationState: "pending", NextAttemptAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("InsertComment held reply: %v", err)
	}

	target, err := w.q.CommentTarget(ctx, held.ID)
	if err != nil {
		t.Fatalf("CommentTarget: %v", err)
	}
	if target.ReportID != w.report || target.AccountID != w.bob || target.ParentID == nil || *target.ParentID != parent ||
		target.Deleted || target.ModerationState != "pending" || cmtStr(target.AuthorUsername) != "bobby_p" ||
		cmtStr(target.AuthorUsernameScreen) != "pending" || target.AuthorDeleted {
		t.Errorf("CommentTarget = %+v", target)
	}
	if _, err := w.q.CommentTarget(ctx, held.ID+1000); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("CommentTarget of a missing id: want pgx.ErrNoRows, got %v", err)
	}

	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	n, err := w.q.MarkCommentDeleted(ctx, sqlcgen.MarkCommentDeletedParams{Now: now, CommentID: root.ID, AccountID: w.carol})
	if err != nil || n != 0 {
		t.Errorf("deleting another person's comment: %d rows, err %v, want 0", n, err)
	}
	for i, want := range []int64{1, 0} {
		n, err := w.q.MarkCommentDeleted(ctx, sqlcgen.MarkCommentDeletedParams{Now: now, CommentID: root.ID, AccountID: w.alice})
		if err != nil || n != want {
			t.Errorf("author delete %d: %d rows, err %v, want %d", i+1, n, err, want)
		}
	}
	var body string
	var deleted bool
	if err := pool.QueryRow(ctx, `SELECT body, deleted_at IS NOT NULL FROM comments WHERE id = $1`, root.ID).Scan(&body, &deleted); err != nil || body != "" || !deleted {
		t.Errorf("after delete: body %q deleted %v err %v, want empty and deleted", body, deleted, err)
	}
}
