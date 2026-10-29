package service_test

// commentupdates_test.go proves the bell rows of comments (Phase 8 plan 11,
// D-16, PAGE-02, A-P8-3): who is told about a comment and who is not, that a
// held comment tells nobody until it is visible, that a row carries no words,
// that likes tell nobody, and that commenting makes the account follow the
// report. It reuses the cmsRig of comments_test.go.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// cmuCount counts the account's update rows of one kind, optionally only the
// unread ones, optionally only for one report (0 means any report).
func cmuCount(t *testing.T, r *cmsRig, account int64, kind string, report int64, unreadOnly bool) int {
	t.Helper()
	q := `SELECT count(*) FROM updates WHERE account_id = $1 AND kind = $2`
	args := []any{account, kind}
	if report != 0 {
		q += ` AND report_id = $3`
		args = append(args, report)
	}
	if unreadOnly {
		q += ` AND read_at IS NULL`
	}
	var n int
	if err := r.pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatalf("counting %s rows of account %d: %v", kind, account, err)
	}
	return n
}

// cmuAnyRows counts every update row of the two comment kinds, any account.
func cmuAnyRows(t *testing.T, r *cmsRig) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM updates WHERE kind IN ('comment_on_yours', 'reply_to_yours')`).Scan(&n); err != nil {
		t.Fatalf("counting comment update rows: %v", err)
	}
	return n
}

func TestCommentUpdatesRecipients(t *testing.T) {
	r := newCmsRig(t)
	const on, reply = "comment_on_yours", "reply_to_yours"

	t.Run("a comment by someone else tells the reporter, not the commenter", func(t *testing.T) {
		rep := r.open(t)
		r.mustPost(t, rep, r.alice, "water is rising here", nil)
		if got := cmuCount(t, r, r.reporter, on, rep, true); got != 1 {
			t.Errorf("reporter comment_on_yours rows = %d, want 1", got)
		}
		if got := cmuCount(t, r, r.alice, on, 0, false) + cmuCount(t, r, r.alice, reply, 0, false); got != 0 {
			t.Errorf("the commenter got %d rows, want none", got)
		}
		if r.note.calls != 1 {
			t.Errorf("notifier calls = %d, want 1 (one row written)", r.note.calls)
		}
	})

	t.Run("a reply tells the author answered, not the reporter", func(t *testing.T) {
		rep := r.open(t)
		root := r.mustPost(t, rep, r.alice, "first words", nil)
		before := cmuCount(t, r, r.reporter, on, rep, false)
		calls := r.note.calls
		r.mustPost(t, rep, r.bob, "an answer", &root.ID)
		if got := cmuCount(t, r, r.alice, reply, rep, true); got != 1 {
			t.Errorf("alice reply_to_yours rows = %d, want 1", got)
		}
		if got := cmuCount(t, r, r.reporter, on, rep, false); got != before {
			t.Errorf("reporter comment_on_yours rows %d -> %d, want no row for a reply", before, got)
		}
		if cmuCount(t, r, r.bob, reply, 0, false)+cmuCount(t, r, r.bob, on, 0, false) != 0 {
			t.Error("the replier was told about their own reply")
		}
		if r.note.calls != calls+1 {
			t.Errorf("notifier calls %d -> %d, want one more", calls, r.note.calls)
		}
	})

	t.Run("a reply to a reply tells the author of that reply", func(t *testing.T) {
		rep := r.open(t)
		root := r.mustPost(t, rep, r.alice, "root words", nil)
		first := r.mustPost(t, rep, r.bob, "bob answers", &root.ID)
		r.mustPost(t, rep, r.carol, "carol answers bob", &first.ID)
		if got := cmuCount(t, r, r.bob, reply, rep, true); got != 1 {
			t.Errorf("bob reply_to_yours rows = %d, want 1", got)
		}
		if got := cmuCount(t, r, r.alice, reply, rep, false); got != 1 {
			t.Errorf("alice reply_to_yours rows = %d, want the one from bob only", got)
		}
	})

	t.Run("the reporter commenting on their own report tells nobody", func(t *testing.T) {
		rep := r.open(t)
		before, calls := cmuAnyRows(t, r), r.note.calls
		r.mustPost(t, rep, r.reporter, "update from me", nil)
		if got := cmuAnyRows(t, r); got != before {
			t.Errorf("comment update rows %d -> %d, want none", before, got)
		}
		if r.note.calls != calls {
			t.Errorf("notifier called for a row that was not written")
		}
	})

	t.Run("a muted report gets no row", func(t *testing.T) {
		rep := r.open(t)
		if _, err := r.pool.Exec(context.Background(),
			`INSERT INTO report_mutes (account_id, report_id) VALUES ($1, $2)`, r.reporter, rep); err != nil {
			t.Fatalf("muting: %v", err)
		}
		calls := r.note.calls
		r.mustPost(t, rep, r.alice, "hello", nil)
		if got := cmuCount(t, r, r.reporter, on, rep, false); got != 0 {
			t.Errorf("muted reporter got %d rows, want none", got)
		}
		if r.note.calls != calls {
			t.Error("notifier called for a row that was not written")
		}
	})

	t.Run("a deleted account gets no row", func(t *testing.T) {
		rep := r.open(t)
		if _, err := r.pool.Exec(context.Background(),
			`UPDATE accounts SET deleted_at = now() WHERE id = $1`, r.reporter); err != nil {
			t.Fatalf("deleting the reporter: %v", err)
		}
		t.Cleanup(func() {
			_, _ = r.pool.Exec(context.Background(), `UPDATE accounts SET deleted_at = NULL WHERE id = $1`, r.reporter)
		})
		r.mustPost(t, rep, r.alice, "hello", nil)
		if got := cmuCount(t, r, r.reporter, on, rep, false); got != 0 {
			t.Errorf("deleted reporter got %d rows, want none", got)
		}
	})

	t.Run("a report with no reporter account gets no row", func(t *testing.T) {
		const orphan = "cmu-orphan-session"
		if _, err := r.pool.Exec(context.Background(), `INSERT INTO sessions (session_id) VALUES ($1)`, orphan); err != nil {
			t.Fatalf("seeding the session: %v", err)
		}
		rep := testutil.InsertStoredReport(t, r.pool, testutil.StoredReport{
			SessionID: orphan, Category: "flood", Severity: "low", Description: "water on the road",
			Latitude: 16.5062, Longitude: 80.6480,
			CreatedAt: r.now.Add(-2 * time.Hour), ExpiresAt: r.now.Add(6 * time.Hour), ModerationState: "clear",
		})
		before := cmuAnyRows(t, r)
		r.mustPost(t, rep, r.alice, "hello", nil)
		if got := cmuAnyRows(t, r); got != before {
			t.Errorf("comment update rows %d -> %d, want none", before, got)
		}
	})

	t.Run("thirty comments are one unread row, a read row lets the next one in", func(t *testing.T) {
		rep := r.open(t)
		commenters := []int64{r.alice, r.bob, r.carol}
		for i := 0; i < 30; i++ {
			r.mustPost(t, rep, commenters[i%3], "comment number "+strings.Repeat("x", i%5+1), nil)
		}
		if got := cmuCount(t, r, r.reporter, on, rep, false); got != 1 {
			t.Errorf("rows after thirty comments = %d, want 1", got)
		}
		if _, err := r.pool.Exec(context.Background(),
			`UPDATE updates SET read_at = now() WHERE account_id = $1`, r.reporter); err != nil {
			t.Fatalf("marking read: %v", err)
		}
		r.mustPost(t, rep, r.alice, "one more after reading", nil)
		if all, unread := cmuCount(t, r, r.reporter, on, rep, false), cmuCount(t, r, r.reporter, on, rep, true); all != 2 || unread != 1 {
			t.Errorf("rows after reading = %d with %d unread, want 2 with 1", all, unread)
		}
	})

	t.Run("comment rows stay out of the hourly ceiling", func(t *testing.T) {
		// A fresh account gets one comment row on each of ten reports (the ceiling
		// is ten). A Reliable alert, which is written only under the ceiling,
		// must still be written: comment rows are not counted by the ceiling
		// reads, so a busy report cannot cut a person off from state changes.
		owner := testutil.SeedReporterAccount(t, r.pool, "cmu-busy@example.com", "cmu-busy-session")
		for i := 0; i < service.UpdatesHourlyCeiling; i++ {
			rep := testutil.InsertStoredReport(t, r.pool, testutil.StoredReport{
				SessionID: "cmu-busy-session", Category: "flood", Severity: "low", Description: "water on the road",
				Latitude: 16.5062, Longitude: 80.6480,
				CreatedAt: r.now.Add(-2 * time.Hour), ExpiresAt: r.now.Add(6 * time.Hour), ModerationState: "clear",
			})
			r.mustPost(t, rep, r.alice, "comment on a busy report", nil)
		}
		if got := cmuCount(t, r, owner, on, 0, true); got != service.UpdatesHourlyCeiling {
			t.Fatalf("setup: comment rows = %d, want %d", got, service.UpdatesHourlyCeiling)
		}
		alertAbout := testutil.InsertStoredReport(t, r.pool, testutil.StoredReport{
			SessionID: "cmu-busy-session", Category: "flood", Severity: "low", Description: "another report",
			Latitude: 16.5062, Longitude: 80.6480,
			CreatedAt: r.now.Add(-time.Hour), ExpiresAt: r.now.Add(6 * time.Hour), ModerationState: "clear",
		})
		now := time.Now().UTC()
		if _, err := r.q.InsertReliableAlert(context.Background(), sqlcgen.InsertReliableAlertParams{
			AccountID: owner, ReportID: alertAbout, Now: now, HourEdge: now.Add(-time.Hour),
			Ceiling: int64(service.UpdatesHourlyCeiling), State: "live",
		}); err != nil {
			t.Errorf("a Reliable alert after ten comment rows = %v, want it written (comment rows are outside the ceiling)", err)
		}
	})
}

func TestCommentUpdatesHeldWaits(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()
	rep := r.open(t)

	r.scr.outcome = "unfinished"
	held := r.mustPost(t, rep, r.alice, "held words", nil)
	if !held.Held {
		t.Fatal("setup: the comment was not held")
	}
	if got := cmuCount(t, r, r.reporter, "comment_on_yours", 0, false); got != 0 {
		t.Fatalf("a held comment wrote %d rows, want none", got)
	}
	if r.note.calls != 0 {
		t.Errorf("notifier calls = %d for a held comment, want 0", r.note.calls)
	}
	// Telling the bell while it is still held writes nothing either.
	if err := r.svc.NotifyCommentVisible(ctx, held.ID); err != nil {
		t.Fatalf("NotifyCommentVisible on a held comment: %v", err)
	}
	if got := cmuCount(t, r, r.reporter, "comment_on_yours", 0, false); got != 0 {
		t.Errorf("NotifyCommentVisible on a held comment wrote %d rows, want none", got)
	}

	// The comment becomes visible: now the row is written.
	if _, err := r.pool.Exec(ctx,
		`UPDATE comments SET moderation_state = 'clear', next_attempt_at = NULL WHERE id = $1`, held.ID); err != nil {
		t.Fatalf("settling the comment: %v", err)
	}
	if err := r.svc.NotifyCommentVisible(ctx, held.ID); err != nil {
		t.Fatalf("NotifyCommentVisible: %v", err)
	}
	if got := cmuCount(t, r, r.reporter, "comment_on_yours", rep, true); got != 1 {
		t.Errorf("rows once visible = %d, want 1", got)
	}
	if r.note.calls != 1 {
		t.Errorf("notifier calls = %d once visible, want 1", r.note.calls)
	}
	// Telling again changes nothing (the unread row already exists).
	if err := r.svc.NotifyCommentVisible(ctx, held.ID); err != nil {
		t.Fatalf("second NotifyCommentVisible: %v", err)
	}
	if got := cmuCount(t, r, r.reporter, "comment_on_yours", rep, false); got != 1 {
		t.Errorf("rows after a second call = %d, want 1", got)
	}
	if r.note.calls != 1 {
		t.Errorf("notifier calls = %d after a call that wrote nothing, want 1", r.note.calls)
	}

	// A deleted, removed or unknown comment writes nothing.
	rep2 := r.open(t)
	deleted := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep2, AccountID: r.alice, Deleted: true})
	removed := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep2, AccountID: r.alice, State: "removed"})
	for name, id := range map[string]int64{"deleted": deleted, "removed": removed, "unknown": 999999} {
		if err := r.svc.NotifyCommentVisible(ctx, id); err != nil {
			t.Errorf("%s: NotifyCommentVisible = %v, want nil", name, err)
		}
	}
	if got := cmuCount(t, r, r.reporter, "comment_on_yours", rep2, false); got != 0 {
		t.Errorf("rows for deleted, removed and unknown comments = %d, want none", got)
	}
}

func TestCommentUpdateCarriesNoText(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()
	rep := r.open(t)
	const words = "secret flood words zebra"
	root := r.mustPost(t, rep, r.alice, words, nil)
	r.mustPost(t, rep, r.bob, "bobs reply quokka", &root.ID)

	// The stored rows hold a kind and a report id, nothing else.
	rows, err := r.pool.Query(ctx,
		`SELECT account_id, report_id, kind, more_count, bucket_start IS NULL FROM updates WHERE kind IN ('comment_on_yours', 'reply_to_yours') ORDER BY id`)
	if err != nil {
		t.Fatalf("reading rows: %v", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var account, report int64
		var kind string
		var more int
		var noBucket bool
		if err := rows.Scan(&account, &report, &kind, &more, &noBucket); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if report != rep || more != 0 || !noBucket {
			t.Errorf("row %s = report %d more %d noBucket %v, want only a kind and the report id", kind, report, more, noBucket)
		}
		n++
	}
	if n != 2 {
		t.Fatalf("comment rows = %d, want 2", n)
	}

	upd := service.NewUpdatesService(r.q, nil, service.WithUpdatesClock(func() time.Time { return r.now.Add(time.Minute) }))
	page, err := upd.List(ctx, r.reporter, service.ActivityUpdatesListSize)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("reporter rows = %d, want 1", len(page.Items))
	}
	item := page.Items[0]
	if item.Kind != service.UpdateCommentOnYours || item.Event != "New comment on your report" || !item.Available {
		t.Errorf("item = %+v, want an available comment_on_yours row reading %q", item, "New comment on your report")
	}
	if item.Trust == "" {
		t.Error("the row has no trust line")
	}
	alicePage, err := upd.List(ctx, r.alice, service.ActivityUpdatesListSize)
	if err != nil {
		t.Fatalf("List for alice: %v", err)
	}
	if len(alicePage.Items) != 1 || alicePage.Items[0].Event != "New reply to your comment" {
		t.Errorf("alice rows = %+v, want one reply_to_yours row", alicePage.Items)
	}
	for _, p := range []service.UpdatesPage{page, alicePage} {
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		text := strings.ToLower(string(raw))
		for _, leak := range []string{"zebra", "quokka", "secret flood", "alice_w", "bobby_p", "carol_c"} {
			if strings.Contains(text, leak) {
				t.Errorf("the updates page holds %q: %s", leak, raw)
			}
		}
	}
}

func TestLikesNotify(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()
	rep := r.open(t)
	c := r.mustPost(t, rep, r.alice, "words to like", nil)
	rowsBefore, side, calls := cmuAnyRows(t, r), cmsSideRows(t, r), r.note.calls

	if _, err := r.svc.Like(ctx, c.ID, r.bob); err != nil {
		t.Fatalf("Like: %v", err)
	}
	if _, err := r.svc.Like(ctx, c.ID, r.carol); err != nil {
		t.Fatalf("Like: %v", err)
	}
	if _, err := r.svc.Unlike(ctx, c.ID, r.carol); err != nil {
		t.Fatalf("Unlike: %v", err)
	}
	if err := r.svc.Flag(ctx, c.ID, r.carol, service.FlagInput{Reasons: []service.FlagReason{service.FlagSpam}}); err != nil {
		t.Fatalf("Flag: %v", err)
	}

	if got := cmuAnyRows(t, r); got != rowsBefore {
		t.Errorf("comment update rows %d -> %d: likes and flags must tell nobody", rowsBefore, got)
	}
	if got := cmsSideRows(t, r); got != side {
		t.Errorf("update and outbox rows %d -> %d: likes and flags must write none", side, got)
	}
	if r.note.calls != calls {
		t.Errorf("notifier calls %d -> %d, want no poke for a like or a flag", calls, r.note.calls)
	}
}

func TestCommenterFollowsReportInUpdates(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()
	follows := func(account, report int64) bool {
		t.Helper()
		rows, err := r.q.FollowedOpenReports(ctx, sqlcgen.FollowedOpenReportsParams{AccountID: account, Now: r.now, RowCap: service.FollowedReportsCap})
		if err != nil {
			t.Fatalf("FollowedOpenReports: %v", err)
		}
		for _, row := range rows {
			if row.ID == report {
				return true
			}
		}
		return false
	}

	rep := r.open(t)
	if follows(r.alice, rep) {
		t.Fatal("setup: alice follows a report she never touched")
	}
	cl := r.mustPost(t, rep, r.alice, "now I follow", nil)
	if !follows(r.alice, rep) {
		t.Error("a clear comment does not make its author follow the report")
	}
	if follows(r.carol, rep) {
		t.Error("someone with no comment follows the report")
	}

	// Deleted: the follow ends (the only link was the comment).
	if err := r.svc.Delete(ctx, cl.ID, r.alice); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if follows(r.alice, rep) {
		t.Error("a deleted comment still makes its author follow")
	}

	// Removed, held: no follow either.
	repB := r.open(t)
	testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: repB, AccountID: r.bob, State: "removed"})
	testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: repB, AccountID: r.carol, State: "pending"})
	if follows(r.bob, repB) {
		t.Error("a removed comment makes its author follow")
	}
	if follows(r.carol, repB) {
		t.Error("a still held comment makes its author follow")
	}
}
