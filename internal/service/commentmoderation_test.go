package service_test

// commentmoderation_test.go proves the moderator side of comments (Phase 8
// plan 13, D-12, D-14): which comments the list holds and why, that Keep
// releases a held comment and tells its recipient once, that Remove withdraws a
// comment, and that every act writes its audit row in the same transaction. It
// reuses the cmsRig of comments_test.go and runs on a real database.

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// cmmNotifier records NotifyCommentVisible calls.
type cmmNotifier struct {
	mu    sync.Mutex
	calls []int64
}

func (n *cmmNotifier) NotifyCommentVisible(_ context.Context, commentID int64) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, commentID)
	return nil
}

func (n *cmmNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.calls)
}

// cmmRig is the comment rig plus a moderator account and a moderation service
// over the real pool transaction.
type cmmRig struct {
	*cmsRig
	moderator int64
	notifier  *cmmNotifier
	svc       *service.CommentModerationService
}

func newCmmRig(t *testing.T) *cmmRig {
	t.Helper()
	base := newCmsRig(t)
	m := &cmmRig{cmsRig: base, notifier: &cmmNotifier{}}
	m.moderator = testutil.SeedReporterAccount(t, base.pool, "cmm-moderator@example.com")
	m.svc = service.NewCommentModerationService(base.q, service.PgxCommentModerationTx(base.pool), m.notifier)
	return m
}

// flagBy adds an open flag with one reason by an account.
func (m *cmmRig) flagBy(t *testing.T, commentID, accountID int64, reasons ...string) {
	t.Helper()
	if _, err := m.pool.Exec(context.Background(),
		`INSERT INTO comment_flags (comment_id, account_id, reasons) VALUES ($1, $2, $3)`,
		commentID, accountID, reasons); err != nil {
		t.Fatalf("flagging comment %d: %v", commentID, err)
	}
}

// openFlags counts the undismissed flags of a comment.
func (m *cmmRig) openFlags(t *testing.T, commentID int64) int {
	t.Helper()
	var n int
	if err := m.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM comment_flags WHERE comment_id = $1 AND dismissed_at IS NULL`, commentID).Scan(&n); err != nil {
		t.Fatalf("counting open flags: %v", err)
	}
	return n
}

// actions reads the audit actions written for a comment, oldest first.
func (m *cmmRig) actions(t *testing.T, commentID int64) []string {
	t.Helper()
	rows, err := m.pool.Query(context.Background(),
		`SELECT action FROM comment_moderation_actions WHERE comment_id = $1 ORDER BY id`, commentID)
	if err != nil {
		t.Fatalf("reading audit rows: %v", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatalf("scanning audit row: %v", err)
		}
		out = append(out, a)
	}
	return out
}

// markReview sets needs_review on a comment, the borderline outcome.
func (m *cmmRig) markReview(t *testing.T, commentID int64) {
	t.Helper()
	if _, err := m.pool.Exec(context.Background(),
		`UPDATE comments SET needs_review = true WHERE id = $1`, commentID); err != nil {
		t.Fatalf("marking comment %d for review: %v", commentID, err)
	}
}

func TestCommentModerationQueueReasons(t *testing.T) {
	m := newCmmRig(t)
	ctx := context.Background()
	rep := m.open(t)

	flagged := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, Body: "flagged words"})
	m.flagBy(t, flagged, m.bob, "spam")
	m.flagBy(t, flagged, m.carol, "abusive", "spam")

	near := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, Body: "near the line"})
	m.markReview(t, near)

	held := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.bob, Body: "held words", State: "pending"})

	both := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.carol, Body: "flagged and near"})
	m.markReview(t, both)
	m.flagBy(t, both, m.alice, "dangerous")

	// None of these belong in the list.
	plain := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, Body: "all fine"})
	dismissed := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.bob, Body: "dismissed flag"})
	f := testutil.SeedCommentFlag(t, m.pool, dismissed, m.carol)
	if _, err := m.pool.Exec(ctx, `UPDATE comment_flags SET dismissed_at = now() WHERE id = $1`, f); err != nil {
		t.Fatalf("dismissing: %v", err)
	}
	deleted := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.bob, Deleted: true})
	m.markReview(t, deleted)
	m.flagBy(t, deleted, m.carol, "spam")
	removed := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.bob, Body: "taken down", State: "removed"})
	m.flagBy(t, removed, m.carol, "spam")

	items, err := m.svc.Queue(ctx)
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	byID := map[int64]service.CommentModerationItem{}
	for _, it := range items {
		byID[it.ID] = it
	}
	for _, id := range []int64{plain, dismissed, deleted, removed} {
		if _, ok := byID[id]; ok {
			t.Errorf("comment %d is in the list, want it left out", id)
		}
	}
	if len(items) != 4 {
		t.Fatalf("list holds %d comments, want 4 (flagged, near, held, both)", len(items))
	}

	check := func(name string, id int64, words string, reasons []string, flags int, labels []string, held, review bool) {
		t.Helper()
		it, ok := byID[id]
		if !ok {
			t.Errorf("%s: comment %d missing from the list", name, id)
			return
		}
		if it.Words != words || it.ReportID != rep || it.Category != "flood" {
			t.Errorf("%s: words %q report %d category %q, want %q on %d flood", name, it.Words, it.ReportID, it.Category, words, rep)
		}
		if !reflect.DeepEqual(it.Reasons, reasons) {
			t.Errorf("%s: reasons %v, want %v", name, it.Reasons, reasons)
		}
		if it.Flags != flags || it.Held != held || it.NeedsReview != review {
			t.Errorf("%s: flags %d held %v review %v, want %d %v %v", name, it.Flags, it.Held, it.NeedsReview, flags, held, review)
		}
		if !reflect.DeepEqual(it.FlagReasons, labels) {
			t.Errorf("%s: flag reasons %v, want %v", name, it.FlagReasons, labels)
		}
	}
	check("flagged", flagged, "flagged words", []string{service.CommentReasonFlagged}, 2,
		[]string{"Spam or gibberish", "Abusive or hateful"}, false, false)
	check("near", near, "near the line", []string{service.CommentReasonNearLine}, 0, []string{}, false, true)
	check("held", held, "held words", []string{service.CommentReasonCheckPending}, 0, []string{}, true, false)
	check("both", both, "flagged and near", []string{service.CommentReasonFlagged, service.CommentReasonNearLine}, 1,
		[]string{"Dangerous or illegal"}, false, true)

	// Newest first.
	for i := 1; i < len(items); i++ {
		if items[i-1].At.Before(items[i].At) {
			t.Errorf("list is not newest first at %d", i)
		}
	}

	// An empty list is an empty slice, not nil.
	empty := newCmmRig(t)
	got, err := empty.svc.Queue(ctx)
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("empty Queue = %v, %v, want a non nil empty slice", got, err)
	}
}

func TestCommentKeepReleasesHeld(t *testing.T) {
	ctx := context.Background()

	t.Run("a held comment becomes clear, flags dismissed, one notify", func(t *testing.T) {
		m := newCmmRig(t)
		rep := m.open(t)
		held := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, Body: "held words", State: "pending"})
		m.flagBy(t, held, m.carol, "spam")
		if err := m.svc.Keep(ctx, m.moderator, held); err != nil {
			t.Fatalf("Keep: %v", err)
		}
		row := m.row(t, held)
		if row.state != "clear" || row.needsReview {
			t.Errorf("after Keep state %q needs_review %v, want clear and false", row.state, row.needsReview)
		}
		var due, lease *string
		if err := m.pool.QueryRow(ctx,
			`SELECT next_attempt_at::text, lease_until::text FROM comments WHERE id = $1`, held).Scan(&due, &lease); err != nil {
			t.Fatalf("reading the lease: %v", err)
		}
		if due != nil || lease != nil {
			t.Errorf("due %v lease %v, want both cleared", due, lease)
		}
		if got := m.openFlags(t, held); got != 0 {
			t.Errorf("open flags = %d, want 0", got)
		}
		if got := m.actions(t, held); !reflect.DeepEqual(got, []string{"keep"}) {
			t.Errorf("audit rows = %v, want [keep]", got)
		}
		if m.notifier.count() != 1 || m.notifier.calls[0] != held {
			t.Errorf("notifier calls = %v, want exactly [%d]", m.notifier.calls, held)
		}
	})

	t.Run("the real notifier writes the recipient's bell row once", func(t *testing.T) {
		m := newCmmRig(t)
		rep := m.open(t)
		held := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, Body: "held words", State: "pending"})
		withReal := service.NewCommentModerationService(m.q, service.PgxCommentModerationTx(m.pool), m.cmsRig.svc)
		if err := withReal.Keep(ctx, m.moderator, held); err != nil {
			t.Fatalf("Keep: %v", err)
		}
		var n int
		if err := m.pool.QueryRow(ctx,
			`SELECT count(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'comment_on_yours'`,
			m.reporter, rep).Scan(&n); err != nil {
			t.Fatalf("counting bell rows: %v", err)
		}
		if n != 1 {
			t.Errorf("reporter bell rows = %d, want 1", n)
		}
	})

	t.Run("a flagged clear comment is kept without a notify", func(t *testing.T) {
		m := newCmmRig(t)
		rep := m.open(t)
		c := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, Body: "flagged"})
		m.flagBy(t, c, m.bob, "spam")
		m.markReview(t, c)
		if err := m.svc.Keep(ctx, m.moderator, c); err != nil {
			t.Fatalf("Keep: %v", err)
		}
		row := m.row(t, c)
		if row.state != "clear" || row.needsReview {
			t.Errorf("state %q needs_review %v, want clear and false", row.state, row.needsReview)
		}
		if m.openFlags(t, c) != 0 {
			t.Error("flags still open after Keep")
		}
		if m.notifier.count() != 0 {
			t.Errorf("notifier called %d times for a comment that was never held", m.notifier.count())
		}
		items, err := m.svc.Queue(ctx)
		if err != nil || len(items) != 0 {
			t.Errorf("Queue after Keep = %v, %v, want empty", items, err)
		}
	})

	t.Run("a comment that needs no decision is already handled and writes no audit row", func(t *testing.T) {
		m := newCmmRig(t)
		rep := m.open(t)
		c := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice})
		if err := m.svc.Keep(ctx, m.moderator, c); !errors.Is(err, service.ErrAlreadyHandled) {
			t.Errorf("Keep of a plain comment = %v, want ErrAlreadyHandled", err)
		}
		if got := m.actions(t, c); len(got) != 0 {
			t.Errorf("audit rows = %v, want none", got)
		}
	})

	t.Run("unknown, deleted and removed comments are gone, and a removed one stays removed", func(t *testing.T) {
		m := newCmmRig(t)
		rep := m.open(t)
		deleted := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, Deleted: true})
		removed := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, State: "removed"})
		for name, id := range map[string]int64{"unknown": 987654, "deleted": deleted, "removed": removed} {
			if err := m.svc.Keep(ctx, m.moderator, id); !errors.Is(err, service.ErrCommentGone) {
				t.Errorf("Keep of %s = %v, want ErrCommentGone", name, err)
			}
		}
		if got := m.row(t, removed).state; got != "removed" {
			t.Errorf("removed comment state = %q after Keep, want removed", got)
		}
		if m.notifier.count() != 0 {
			t.Error("notifier called for a Keep that failed")
		}
	})
}

func TestCommentRemove(t *testing.T) {
	ctx := context.Background()
	m := newCmmRig(t)
	rep := m.open(t)

	root := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, Body: "root words"})
	m.flagBy(t, root, m.carol, "abusive")
	m.markReview(t, root)
	reply := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.carol, ParentID: root, Body: "a shown reply"})
	lone := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.bob, Body: "alone", State: "pending"})

	if err := m.svc.Remove(ctx, m.moderator, root); err != nil {
		t.Fatalf("Remove root: %v", err)
	}
	if err := m.svc.Remove(ctx, m.moderator, lone); err != nil {
		t.Fatalf("Remove held: %v", err)
	}
	for _, id := range []int64{root, lone} {
		row := m.row(t, id)
		if row.state != "removed" || row.needsReview {
			t.Errorf("comment %d state %q needs_review %v, want removed and false", id, row.state, row.needsReview)
		}
		if got := m.actions(t, id); !reflect.DeepEqual(got, []string{"remove"}) {
			t.Errorf("comment %d audit rows = %v, want [remove]", id, got)
		}
	}
	if m.openFlags(t, root) != 0 {
		t.Error("flags still open after Remove")
	}
	if m.notifier.count() != 0 {
		t.Error("Remove told someone a comment became visible")
	}

	// The root reads as the removed placeholder because a reply shows under it;
	// the held one with no replies is simply gone.
	list := m.list(t, rep, m.alice)
	var rootSeen, loneSeen bool
	for _, c := range list.Comments {
		switch c.ID {
		case root:
			rootSeen = true
			if !c.Removed || c.Words != "" {
				t.Errorf("root = %+v, want the removed placeholder with no words", c)
			}
			if len(c.Replies) != 1 || c.Replies[0].ID != reply {
				t.Errorf("replies under the removed root = %+v, want the one reply", c.Replies)
			}
		case lone:
			loneSeen = true
		}
	}
	if !rootSeen || loneSeen {
		t.Errorf("root listed %v, removed held listed %v, want true and false", rootSeen, loneSeen)
	}
	items, err := m.svc.Queue(ctx)
	if err != nil || len(items) != 0 {
		t.Errorf("Queue after Remove = %v, %v, want empty", items, err)
	}

	// A second Remove, and Remove of a deleted or unknown comment, are gone.
	deleted := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, Deleted: true})
	for name, id := range map[string]int64{"again": root, "deleted": deleted, "unknown": 987654} {
		if err := m.svc.Remove(ctx, m.moderator, id); !errors.Is(err, service.ErrCommentGone) {
			t.Errorf("Remove of %s = %v, want ErrCommentGone", name, err)
		}
	}
	if got := m.actions(t, root); len(got) != 1 {
		t.Errorf("audit rows of root = %v after a repeated Remove, want 1", got)
	}
}

// cmmFailAudit wraps a querier so the audit insert fails.
type cmmFailAudit struct {
	service.CommentModerationQuerier
}

func (cmmFailAudit) InsertCommentModerationAction(context.Context, sqlcgen.InsertCommentModerationActionParams) error {
	return errors.New("audit insert failed")
}

func TestCommentModerationAuditInTx(t *testing.T) {
	ctx := context.Background()

	t.Run("an audit row is written with the act", func(t *testing.T) {
		m := newCmmRig(t)
		rep := m.open(t)
		c := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice})
		m.flagBy(t, c, m.bob, "spam")
		if err := m.svc.Remove(ctx, m.moderator, c); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		var who int64
		var action string
		if err := m.pool.QueryRow(ctx,
			`SELECT moderator_account_id, action FROM comment_moderation_actions WHERE comment_id = $1`, c).Scan(&who, &action); err != nil {
			t.Fatalf("reading the audit row: %v", err)
		}
		if who != m.moderator || action != "remove" {
			t.Errorf("audit row = moderator %d action %q, want %d remove", who, action, m.moderator)
		}
	})

	t.Run("a failed audit insert rolls back every write", func(t *testing.T) {
		m := newCmmRig(t)
		rep := m.open(t)
		held := testutil.SeedComment(t, m.pool, testutil.CommentSeed{ReportID: rep, AccountID: m.alice, State: "pending"})
		m.flagBy(t, held, m.bob, "spam")
		m.markReview(t, held)
		failing := func(ctx context.Context, fn func(q service.CommentModerationQuerier) error) error {
			return pgx.BeginFunc(ctx, m.pool, func(tx pgx.Tx) error {
				return fn(cmmFailAudit{sqlcgen.New(tx)})
			})
		}
		svc := service.NewCommentModerationService(m.q, failing, m.notifier)

		for name, act := range map[string]func() error{
			"Keep":   func() error { return svc.Keep(ctx, m.moderator, held) },
			"Remove": func() error { return svc.Remove(ctx, m.moderator, held) },
		} {
			if err := act(); err == nil {
				t.Fatalf("%s with a failing audit insert returned nil", name)
			}
			row := m.row(t, held)
			if row.state != "pending" || !row.needsReview {
				t.Errorf("after a failed %s state %q needs_review %v, want pending and true", name, row.state, row.needsReview)
			}
			if m.openFlags(t, held) != 1 {
				t.Errorf("after a failed %s the flag was dismissed", name)
			}
			if got := m.actions(t, held); len(got) != 0 {
				t.Errorf("after a failed %s audit rows = %v, want none", name, got)
			}
		}
		if m.notifier.count() != 0 {
			t.Error("a Keep that rolled back still notified")
		}
	})
}

// TestServerCopyCommentModeration pins the comment moderation words against the
// closed character set and the UI-SPEC text.
func TestServerCopyCommentModeration(t *testing.T) {
	exact := map[string]string{
		service.ModCommentAlreadyHandledMessage: "This comment was already handled. Refresh the lists.",
		service.ModCommentNotFoundMessage:       "No comment found with that id. Check the id and try again.",
		service.CommentReasonFlagged:            "Flagged by people",
		service.CommentReasonNearLine:           "Near the line",
		service.CommentReasonCheckPending:       "Check not finished",
	}
	for got, want := range exact {
		assertServerCopy(t, got)
		if got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	}
}
