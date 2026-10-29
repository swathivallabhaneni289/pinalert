package service_test

// comments_test.go proves CommentService on a real database (Phase 8 plan 09,
// PAGE-02, SC3, D-02, D-12 to D-15): the screening outcomes, replies that join
// a thread, the closed state, delete, the access rule that mirrors the timeline
// and Share, the names a reader sees and the finished Rescue rule. Helpers carry
// the prefix cms so they never collide with other files of this package.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// cmsScreener answers each text check outcome on demand and counts its calls.
type cmsScreener struct {
	outcome string // clear, borderline, reject, unfinished
	calls   int
}

func (s *cmsScreener) Screen(_ context.Context, _ string) (moderation.Result, error) {
	s.calls++
	switch s.outcome {
	case "borderline":
		return moderation.Result{Model: "test-model", Scores: map[string]float64{"hate": 0.5}}, nil
	case "reject":
		return moderation.Result{Model: "test-model", Scores: map[string]float64{"hate": 0.95}}, nil
	case "unfinished":
		return moderation.Result{}, errors.New("provider down")
	default:
		return moderation.Result{Model: "test-model", Scores: map[string]float64{"hate": 0.01}}, nil
	}
}

type cmsNotifier struct{ calls int }

func (n *cmsNotifier) ReportChanged(_, _ int64) { n.calls++ }

// cmsRig is one database with a reporter and three readers.
type cmsRig struct {
	pool     *pgxpool.Pool
	q        *sqlcgen.Queries
	svc      *service.CommentService
	scr      *cmsScreener
	note     *cmsNotifier
	now      time.Time
	session  string
	reporter int64
	alice    int64 // alice_w, screen clear
	bob      int64 // bobby_p, screen pending
	carol    int64 // carol_c, screen clear
}

func newCmsRig(t *testing.T) *cmsRig {
	t.Helper()
	pool := testutil.NewTestDB(t)
	r := &cmsRig{
		pool:    pool,
		q:       sqlcgen.New(pool),
		scr:     &cmsScreener{outcome: "clear"},
		note:    &cmsNotifier{},
		now:     time.Now().UTC(),
		session: "cms-reporter-session",
	}
	r.reporter = testutil.SeedReporterAccount(t, pool, "cms-reporter@example.com", r.session)
	r.alice = testutil.SeedReporterAccount(t, pool, "cms-alice@example.com")
	r.bob = testutil.SeedReporterAccount(t, pool, "cms-bob@example.com")
	r.carol = testutil.SeedReporterAccount(t, pool, "cms-carol@example.com")
	testutil.SeedUsername(t, pool, r.reporter, "reporter_r", "clear")
	testutil.SeedUsername(t, pool, r.alice, "alice_w", "clear")
	testutil.SeedUsername(t, pool, r.bob, "bobby_p", "pending")
	testutil.SeedUsername(t, pool, r.carol, "carol_c", "clear")
	r.svc = service.NewCommentService(r.q,
		service.WithCommentScreener(r.scr),
		service.WithCommentClock(func() time.Time { return r.now }),
		service.WithCommentChangeNotifier(r.note),
	)
	return r
}

// report stores one report posted by the rig's reporter. It expires expiresIn
// from the rig's now (negative for a report that already ended).
func (r *cmsRig) report(t *testing.T, category, severity, moderationState string, expiresIn time.Duration) int64 {
	t.Helper()
	return testutil.InsertStoredReport(t, r.pool, testutil.StoredReport{
		SessionID:       r.session,
		Category:        category,
		Severity:        severity,
		Description:     "water on the road",
		Latitude:        16.5062,
		Longitude:       80.6480,
		CreatedAt:       r.now.Add(expiresIn - 8*time.Hour),
		ExpiresAt:       r.now.Add(expiresIn),
		ModerationState: moderationState,
	})
}

// open is a report people may comment on.
func (r *cmsRig) open(t *testing.T) int64 {
	t.Helper()
	return r.report(t, "flood", "low", "clear", 6*time.Hour)
}

// resolve has the reporter close a report, ago before the rig's now.
func (r *cmsRig) resolve(t *testing.T, reportID int64, ago time.Duration) {
	t.Helper()
	if _, err := r.pool.Exec(context.Background(),
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
		 VALUES ($1, $2, 'resolution', 'resolve', 'tdr1v8y', $3)`,
		reportID, r.reporter, r.now.Add(-ago)); err != nil {
		t.Fatalf("seeding the reporter's resolve vote: %v", err)
	}
}

func (r *cmsRig) post(reportID, account int64, words string, replyTo *int64) (service.CommentView, error) {
	return r.svc.Post(context.Background(), service.CommentInput{
		ReportID: reportID, AccountID: account, HasUsername: true, Words: words, ReplyTo: replyTo,
	})
}

func (r *cmsRig) mustPost(t *testing.T, reportID, account int64, words string, replyTo *int64) service.CommentView {
	t.Helper()
	v, err := r.post(reportID, account, words, replyTo)
	if err != nil {
		t.Fatalf("Post(%q) = %v, want it stored", words, err)
	}
	return v
}

func (r *cmsRig) list(t *testing.T, reportID, viewer int64) service.CommentList {
	t.Helper()
	l, err := r.svc.List(context.Background(), reportID, viewer, 0)
	if err != nil {
		t.Fatalf("List(report %d, viewer %d) = %v", reportID, viewer, err)
	}
	return l
}

// cmsRow is the stored shape of one comment.
type cmsRow struct {
	state       string
	needsReview bool
	parent      *int64
	replyTo     *int64
	body        string
	deleted     bool
	verdict     *string
	hasNext     bool
	hasScores   bool
}

func (r *cmsRig) row(t *testing.T, id int64) cmsRow {
	t.Helper()
	var c cmsRow
	err := r.pool.QueryRow(context.Background(),
		`SELECT moderation_state, needs_review, parent_id, reply_to_id, body, deleted_at IS NOT NULL,
		        verdict, next_attempt_at IS NOT NULL, scores IS NOT NULL
		 FROM comments WHERE id = $1`, id,
	).Scan(&c.state, &c.needsReview, &c.parent, &c.replyTo, &c.body, &c.deleted, &c.verdict, &c.hasNext, &c.hasScores)
	if err != nil {
		t.Fatalf("reading comment %d: %v", id, err)
	}
	return c
}

func (r *cmsRig) countComments(t *testing.T, reportID int64) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM comments WHERE report_id = $1", reportID).Scan(&n); err != nil {
		t.Fatalf("counting comments: %v", err)
	}
	return n
}

func cmsID(i int64) *int64 { return &i }

func TestCommentScreeningOutcomes(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)

	t.Run("clear text is stored clear and returned as the author's own view", func(t *testing.T) {
		r.scr.outcome = "clear"
		v := r.mustPost(t, rep, r.alice, "  Thanks  ", nil)
		if !v.IsOwn || !v.CanDelete || v.AuthorKind != "user" || v.Author != "alice_w" || v.Held || v.Words != "Thanks" {
			t.Errorf("view = %+v, want an own, deletable, unheld comment by alice_w saying Thanks", v)
		}
		if v.IsReporter {
			t.Errorf("IsReporter = true on the viewer's own comment")
		}
		row := r.row(t, v.ID)
		if row.state != "clear" || row.needsReview || row.hasNext || row.body != "Thanks" {
			t.Errorf("row = %+v, want clear, no review, no retry, trimmed body", row)
		}
		if row.verdict == nil || *row.verdict != "pass" || !row.hasScores {
			t.Errorf("verdict/scores = %v / %v, want pass with scores kept", row.verdict, row.hasScores)
		}
	})

	t.Run("borderline text posts and is listed for moderators", func(t *testing.T) {
		r.scr.outcome = "borderline"
		v := r.mustPost(t, rep, r.alice, "this is close to the line", nil)
		row := r.row(t, v.ID)
		if row.state != "clear" || !row.needsReview || row.hasNext {
			t.Errorf("row = %+v, want clear with needs_review", row)
		}
		if row.verdict == nil || *row.verdict != "borderline" {
			t.Errorf("verdict = %v, want borderline", row.verdict)
		}
		if v.Held {
			t.Errorf("a borderline comment is not held")
		}
	})

	t.Run("abusive text is refused with the phase four sentence and nothing is stored", func(t *testing.T) {
		r.scr.outcome = "reject"
		before := r.countComments(t, rep)
		_, err := r.post(rep, r.alice, "something hateful", nil)
		var rej service.CommentRejectedError
		if !errors.As(err, &rej) || !errors.Is(err, service.ErrCommentRejected) {
			t.Fatalf("Post error = %v, want a CommentRejectedError", err)
		}
		if rej.Reason != service.RejectHateful || service.RejectMessage(rej.Reason) == "" {
			t.Errorf("reason = %q, want hateful with a message", rej.Reason)
		}
		if after := r.countComments(t, rep); after != before {
			t.Errorf("comments %d -> %d, want nothing stored", before, after)
		}
	})

	t.Run("text the check could not finish is held for its author only", func(t *testing.T) {
		r.scr.outcome = "unfinished"
		v := r.mustPost(t, rep, r.alice, "waiting for the check", nil)
		if !v.Held {
			t.Errorf("Held = false, want true")
		}
		row := r.row(t, v.ID)
		if row.state != "pending" || !row.hasNext || row.needsReview {
			t.Errorf("row = %+v, want pending with next_attempt_at", row)
		}
		seen := func(viewer int64) bool {
			for _, c := range r.list(t, rep, viewer).Comments {
				if c.ID == v.ID {
					return true
				}
			}
			return false
		}
		if !seen(r.alice) || seen(r.carol) || seen(0) {
			t.Errorf("held comment visible to author %v, other %v, nobody %v; want true, false, false",
				seen(r.alice), seen(r.carol), seen(0))
		}
	})

	t.Run("no screener passes every text and keeps no verdict", func(t *testing.T) {
		svc := service.NewCommentService(r.q, service.WithCommentClock(func() time.Time { return r.now }))
		v, err := svc.Post(context.Background(), service.CommentInput{ReportID: rep, AccountID: r.carol, HasUsername: true, Words: "ok"})
		if err != nil {
			t.Fatalf("Post without a screener = %v", err)
		}
		row := r.row(t, v.ID)
		if row.state != "clear" || row.verdict != nil || row.hasScores {
			t.Errorf("row = %+v, want clear with no verdict and no scores", row)
		}
	})

	t.Run("the text rule runs before the screener", func(t *testing.T) {
		r.scr.outcome = "clear"
		calls := r.scr.calls
		if _, err := r.post(rep, r.alice, "   ", nil); !errors.Is(err, service.ErrCommentEmpty) {
			t.Errorf("empty Post = %v, want ErrCommentEmpty", err)
		}
		// Amended 2026-10-29 by review IN-S01: a tab is now read as a space, so
		// the refused control character is the bell character instead.
		if _, err := r.post(rep, r.alice, "a\ab", nil); !errors.Is(err, service.ErrCommentCharacters) {
			t.Errorf("control character Post = %v, want ErrCommentCharacters", err)
		}
		if r.scr.calls != calls {
			t.Errorf("screener called %d times for refused text, want 0", r.scr.calls-calls)
		}
	})

	// Amended 2026-10-29 by Phase 8 plan 11 (D-16, A-P8-3): plan 08-11 wired the
	// notifier. Post now pokes it once for each bell row it writes and for
	// nothing else, so the calls must equal the comment update rows written.
	t.Run("posting pokes the notifier once per bell row written", func(t *testing.T) {
		var rows int
		if err := r.pool.QueryRow(context.Background(),
			`SELECT count(*) FROM updates WHERE kind IN ('comment_on_yours', 'reply_to_yours')`).Scan(&rows); err != nil {
			t.Fatalf("counting comment update rows: %v", err)
		}
		if r.note.calls != rows {
			t.Errorf("notifier calls = %d, want %d (one per bell row written)", r.note.calls, rows)
		}
	})
}

func TestCommentReplyJoinsThread(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)

	root := r.mustPost(t, rep, r.alice, "road is blocked here", nil)
	first := r.mustPost(t, rep, r.carol, "yes I saw it too", &root.ID)
	second := r.mustPost(t, rep, r.alice, "thanks both", &first.ID)

	rootRow := r.row(t, root.ID)
	if rootRow.parent != nil || rootRow.replyTo != nil {
		t.Errorf("root row = %+v, want no parent and no reply_to", rootRow)
	}
	firstRow := r.row(t, first.ID)
	if firstRow.parent == nil || *firstRow.parent != root.ID || firstRow.replyTo == nil || *firstRow.replyTo != root.ID {
		t.Errorf("reply to a root = %+v, want parent and reply_to both the root", firstRow)
	}
	secondRow := r.row(t, second.ID)
	if secondRow.parent == nil || *secondRow.parent != root.ID {
		t.Errorf("reply to a reply: parent = %v, want the thread root %d", secondRow.parent, root.ID)
	}
	if secondRow.replyTo == nil || *secondRow.replyTo != first.ID {
		t.Errorf("reply to a reply: reply_to = %v, want the reply answered %d", secondRow.replyTo, first.ID)
	}

	if first.ReplyToAuthor != "alice_w" || second.ReplyToAuthor != "carol_c" {
		t.Errorf("ReplyToAuthor = %q and %q, want alice_w and carol_c", first.ReplyToAuthor, second.ReplyToAuthor)
	}

	l := r.list(t, rep, r.bob)
	if len(l.Comments) != 1 || l.Comments[0].ID != root.ID {
		t.Fatalf("list = %+v, want one thread", l.Comments)
	}
	th := l.Comments[0]
	if th.ReplyCount != 2 || len(th.Replies) != 2 || th.Replies[0].ID != first.ID || th.Replies[1].ID != second.ID {
		t.Errorf("thread = %+v, want two replies oldest first", th.Replies)
	}
	if th.Replies[1].ReplyToAuthor != "carol_c" {
		t.Errorf("listed ReplyToAuthor = %q, want carol_c", th.Replies[1].ReplyToAuthor)
	}
	if l.Count != 3 {
		t.Errorf("Count = %d, want 3", l.Count)
	}

	t.Run("a pending username is not named in a reply", func(t *testing.T) {
		b := r.mustPost(t, rep, r.bob, "bob here", nil)
		got := r.mustPost(t, rep, r.carol, "hello", &b.ID)
		if got.ReplyToAuthor != "" {
			t.Errorf("ReplyToAuthor = %q, want empty for a username that has not passed its screen", got.ReplyToAuthor)
		}
	})
}

// TestCommentReplyCapPerThread proves review WR-S02 (2026-10-29): the reply cap
// is per thread, so one busy thread never hides a later thread's replies, and
// ReplyCount is the real number of replies the viewer may see in that thread,
// not the number returned. Thread A holds one reply more than the cap; thread B,
// posted after all of them, holds one.
func TestCommentReplyCapPerThread(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)

	a := r.mustPost(t, rep, r.alice, "is the road open", nil)
	testutil.MustExec(t, r.pool,
		`INSERT INTO comments (report_id, account_id, parent_id, reply_to_id, body, created_at)
		 SELECT $1, $2, $3, $3, 'still closed ' || g, $4
		 FROM generate_series(1, $5::int) AS g`,
		rep, r.carol, a.ID, r.now, service.CommentReplyCap+1)
	b := r.mustPost(t, rep, r.carol, "shelter at the school is full", nil)
	bReply := r.mustPost(t, rep, r.alice, "the hall next door has room", &b.ID)

	l := r.list(t, rep, 0)
	if len(l.Comments) != 2 || l.Comments[0].ID != a.ID || l.Comments[1].ID != b.ID {
		t.Fatalf("list = %d threads, want threads A and B", len(l.Comments))
	}
	ta, tb := l.Comments[0], l.Comments[1]
	if len(ta.Replies) != service.CommentReplyCap {
		t.Errorf("thread A lists %d replies, want the cap %d", len(ta.Replies), service.CommentReplyCap)
	}
	if ta.ReplyCount != service.CommentReplyCap+1 {
		t.Errorf("thread A ReplyCount = %d, want the real count %d", ta.ReplyCount, service.CommentReplyCap+1)
	}
	if len(tb.Replies) != 1 || tb.Replies[0].ID != bReply.ID {
		t.Errorf("thread B lists %d replies, want its one reply", len(tb.Replies))
	}
	if tb.ReplyCount != 1 {
		t.Errorf("thread B ReplyCount = %d, want 1", tb.ReplyCount)
	}
	if want := service.CommentReplyCap + 4; l.Count != want {
		t.Errorf("Count = %d, want %d", l.Count, want)
	}
}

// TestCommentRepliesPaged proves review fix round 2 (SK-01): a thread longer
// than CommentReplyCap keeps every reply reachable. ListReplies reads one
// thread's replies after a cursor, oldest first, a page of at most the cap,
// with More set while some are left. An id that is not a listed root of the
// report (a reply, another report's comment, an unknown id, a removed root
// with nothing shown) answers an empty list, all the same way.
func TestCommentRepliesPaged(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)
	other := r.open(t)
	ctx := context.Background()

	a := r.mustPost(t, rep, r.alice, "is the road open", nil)
	testutil.MustExec(t, r.pool,
		`INSERT INTO comments (report_id, account_id, parent_id, reply_to_id, body, created_at)
		 SELECT $1, $2, $3, $3, 'still closed ' || g, $4
		 FROM generate_series(1, $5::int) AS g`,
		rep, r.carol, a.ID, r.now, service.CommentReplyCap+2)
	b := r.mustPost(t, rep, r.carol, "shelter at the school is full", nil)
	bReply := r.mustPost(t, rep, r.alice, "the hall next door has room", &b.ID)
	elsewhere := r.mustPost(t, other, r.alice, "on another report", nil)
	removed := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice, State: "removed"})

	p1, err := r.svc.ListReplies(ctx, rep, 0, a.ID, 0)
	if err != nil {
		t.Fatalf("ListReplies page 1 = %v", err)
	}
	if len(p1.Comments) != 1 || p1.Comments[0].ID != a.ID {
		t.Fatalf("page 1 = %d roots, want thread A alone", len(p1.Comments))
	}
	t1 := p1.Comments[0]
	if len(t1.Replies) != service.CommentReplyCap || !p1.More {
		t.Errorf("page 1 lists %d replies, more %v; want the cap %d and more", len(t1.Replies), p1.More, service.CommentReplyCap)
	}
	if t1.ReplyCount != service.CommentReplyCap+2 {
		t.Errorf("page 1 ReplyCount = %d, want %d", t1.ReplyCount, service.CommentReplyCap+2)
	}
	if n := len(t1.Replies); n == 0 || p1.NextAfter != t1.Replies[n-1].ID {
		t.Fatalf("page 1 NextAfter = %d, want the last reply id", p1.NextAfter)
	}

	p2, err := r.svc.ListReplies(ctx, rep, 0, a.ID, p1.NextAfter)
	if err != nil {
		t.Fatalf("ListReplies page 2 = %v", err)
	}
	if len(p2.Comments) != 1 || len(p2.Comments[0].Replies) != 2 || p2.More {
		t.Fatalf("page 2 = %+v, want thread A with its last 2 replies and no more", p2)
	}
	t2 := p2.Comments[0].Replies
	if t2[0].ID <= p1.NextAfter || t2[1].ID <= t2[0].ID {
		t.Errorf("page 2 replies %d, %d are not after %d, oldest first", t2[0].ID, t2[1].ID, p1.NextAfter)
	}
	if p2.NextAfter != t2[1].ID {
		t.Errorf("page 2 NextAfter = %d, want %d", p2.NextAfter, t2[1].ID)
	}

	pb, err := r.svc.ListReplies(ctx, rep, 0, b.ID, 0)
	if err != nil {
		t.Fatalf("ListReplies thread B = %v", err)
	}
	if len(pb.Comments) != 1 || len(pb.Comments[0].Replies) != 1 || pb.Comments[0].Replies[0].ID != bReply.ID || pb.More {
		t.Errorf("thread B = %+v, want its one reply and no more", pb)
	}

	for name, id := range map[string]int64{
		"a reply id":            bReply.ID,
		"another report's root": elsewhere.ID,
		"an unknown id":         999999,
		"a removed root":        removed,
	} {
		got, err := r.svc.ListReplies(ctx, rep, 0, id, 0)
		if err != nil {
			t.Errorf("%s: ListReplies = %v, want an empty list", name, err)
			continue
		}
		if len(got.Comments) != 0 || got.More || got.NextAfter != 0 {
			t.Errorf("%s: ListReplies = %+v, want an empty list", name, got)
		}
	}
}

func TestCommentReplyTargetMustBeVisible(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)
	other := r.open(t)

	deleted := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice, Deleted: true})
	removed := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice, State: "removed"})
	held := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice, State: "pending"})
	elsewhere := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: other, AccountID: r.alice})

	cases := map[string]int64{
		"deleted":               deleted,
		"removed":               removed,
		"held":                  held,
		"on another report":     elsewhere,
		"an id that is unknown": 999999,
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			calls := r.scr.calls
			before := r.countComments(t, rep)
			_, err := r.post(rep, r.carol, "replying", cmsID(id))
			if !errors.Is(err, service.ErrCommentGone) {
				t.Fatalf("Post reply = %v, want ErrCommentGone", err)
			}
			if r.scr.calls != calls || r.countComments(t, rep) != before {
				t.Errorf("a refused reply must not screen or store")
			}
		})
	}
}

func TestCommentsClosed(t *testing.T) {
	r := newCmsRig(t)

	resolved := r.open(t)
	r.resolve(t, resolved, time.Hour)
	faded := r.report(t, "flood", "low", "clear", -time.Hour)

	for name, rep := range map[string]int64{"resolved": resolved, "faded": faded} {
		t.Run(name, func(t *testing.T) {
			existing := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice})
			calls := r.scr.calls
			if _, err := r.post(rep, r.carol, "late comment", nil); !errors.Is(err, service.ErrCommentsClosed) {
				t.Errorf("Post = %v, want ErrCommentsClosed", err)
			}
			if _, err := r.post(rep, r.carol, "late reply", &existing); !errors.Is(err, service.ErrCommentsClosed) {
				t.Errorf("reply = %v, want ErrCommentsClosed", err)
			}
			if r.scr.calls != calls {
				t.Errorf("a closed report must cost no screener call")
			}
			l := r.list(t, rep, r.carol)
			if l.Open || len(l.Comments) != 1 {
				t.Errorf("List open=%v comments=%d, want closed and still readable", l.Open, len(l.Comments))
			}
			if err := r.svc.Delete(context.Background(), existing, r.alice); err != nil {
				t.Errorf("Delete on a closed report = %v, want nil", err)
			}
		})
	}

	t.Run("an open report says so", func(t *testing.T) {
		rep := r.open(t)
		if l := r.list(t, rep, r.carol); !l.Open {
			t.Errorf("Open = false on an active report")
		}
	})

	t.Run("a disputed report is still open", func(t *testing.T) {
		rep := r.open(t)
		for i, cell := range []string{"tdr1v8y", "tdr1vc3"} {
			acct := testutil.SeedReporterAccount(t, r.pool, fmt.Sprintf("cms-disputer-%d@example.com", i))
			if _, err := r.pool.Exec(context.Background(),
				`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
				 VALUES ($1, $2, 'content', 'dispute', $3, $4)`, rep, acct, cell, r.now.Add(-time.Hour)); err != nil {
				t.Fatalf("seeding dispute: %v", err)
			}
		}
		if l := r.list(t, rep, r.carol); !l.Open {
			t.Errorf("Open = false on a disputed report")
		}
	})
}

func TestCommentDelete(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)
	ctx := context.Background()

	mine := r.mustPost(t, rep, r.alice, "I will delete this", nil)

	if err := r.svc.Delete(ctx, mine.ID, r.carol); !errors.Is(err, service.ErrCommentNotYours) {
		t.Errorf("another account Delete = %v, want ErrCommentNotYours", err)
	}
	if err := r.svc.Delete(ctx, mine.ID, r.reporter); !errors.Is(err, service.ErrCommentNotYours) {
		t.Errorf("the reporter Delete = %v, want ErrCommentNotYours", err)
	}
	if row := r.row(t, mine.ID); row.deleted || row.body == "" {
		t.Fatalf("a refused delete changed the row: %+v", row)
	}

	if err := r.svc.Delete(ctx, mine.ID, r.alice); err != nil {
		t.Fatalf("author Delete = %v", err)
	}
	if row := r.row(t, mine.ID); !row.deleted || row.body != "" {
		t.Errorf("row after delete = %+v, want deleted_at set and the words blank", row)
	}
	if err := r.svc.Delete(ctx, mine.ID, r.alice); err != nil {
		t.Errorf("second Delete = %v, want nil", err)
	}
	if err := r.svc.Delete(ctx, mine.ID, r.carol); !errors.Is(err, service.ErrCommentNotYours) {
		t.Errorf("another account Delete of a deleted comment = %v, want ErrCommentNotYours", err)
	}
	if err := r.svc.Delete(ctx, 999999, r.alice); !errors.Is(err, service.ErrCommentGone) {
		t.Errorf("Delete of an unknown id = %v, want ErrCommentGone", err)
	}
	if l := r.list(t, rep, r.carol); len(l.Comments) != 0 || l.Count != 0 {
		t.Errorf("a deleted comment with no replies is listed: %+v", l)
	}

	t.Run("delete is allowed on a withheld report and one that ended long ago", func(t *testing.T) {
		old := r.report(t, "flood", "low", "clear", -8*24*time.Hour)
		id := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: old, AccountID: r.alice})
		if err := r.svc.Delete(ctx, id, r.alice); err != nil {
			t.Errorf("Delete on a too old report = %v, want nil", err)
		}
		held := r.report(t, "flood", "low", "pending", 6*time.Hour)
		id = testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: held, AccountID: r.alice})
		if err := r.svc.Delete(ctx, id, r.alice); err != nil {
			t.Errorf("Delete on a withheld report = %v, want nil", err)
		}
	})

	t.Run("a deleted root with a visible reply reads as a placeholder", func(t *testing.T) {
		rep := r.open(t)
		root := r.mustPost(t, rep, r.alice, "first words", nil)
		reply := r.mustPost(t, rep, r.carol, "an answer", &root.ID)
		if err := r.svc.Delete(ctx, root.ID, r.alice); err != nil {
			t.Fatal(err)
		}
		l := r.list(t, rep, r.bob)
		if len(l.Comments) != 1 {
			t.Fatalf("comments = %d, want the placeholder thread", len(l.Comments))
		}
		p := l.Comments[0]
		if !p.Deleted || p.Words != "" || p.Author != "" || p.AuthorKind != "" || p.IsReporter || p.CanDelete || p.IsOwn || p.Likes != 0 {
			t.Errorf("placeholder = %+v, want deleted with no words, name, tag or likes", p)
		}
		if len(p.Replies) != 1 || p.Replies[0].ID != reply.ID || p.ReplyCount != 1 {
			t.Errorf("placeholder replies = %+v", p.Replies)
		}
		if l.Count != 1 {
			t.Errorf("Count = %d, want 1 (the placeholder is not counted)", l.Count)
		}
	})
}

func TestCommentNeedsUsername(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)
	calls := r.scr.calls
	_, err := r.svc.Post(context.Background(), service.CommentInput{ReportID: rep, AccountID: r.alice, HasUsername: false, Words: "hello"})
	if !errors.Is(err, service.ErrCommentNeedsUsername) {
		t.Fatalf("Post without a username = %v, want ErrCommentNeedsUsername", err)
	}
	if r.scr.calls != calls || r.countComments(t, rep) != 0 {
		t.Errorf("a person without a username must not screen or store")
	}
}

func TestCommentWithheld(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()

	held := r.report(t, "flood", "low", "pending", 6*time.Hour)
	removed := r.report(t, "flood", "low", "removed", 6*time.Hour)
	tooOld := r.report(t, "flood", "low", "clear", -8*24*time.Hour)
	resolvedLongAgo := r.open(t)
	r.resolve(t, resolvedLongAgo, 8*24*time.Hour)

	cases := map[string]int64{
		"held":                     held,
		"removed":                  removed,
		"an unknown id":            999999,
		"ended over a week ago":    tooOld,
		"resolved over a week ago": resolvedLongAgo,
	}
	for name, rep := range cases {
		t.Run(name, func(t *testing.T) {
			calls := r.scr.calls
			if _, err := r.post(rep, r.carol, "hello", nil); !errors.Is(err, service.ErrReportNotFound) {
				t.Errorf("Post = %v, want ErrReportNotFound", err)
			}
			if _, err := r.svc.List(ctx, rep, r.carol, 0); !errors.Is(err, service.ErrReportNotFound) {
				t.Errorf("List = %v, want ErrReportNotFound", err)
			}
			if r.scr.calls != calls {
				t.Errorf("a report nobody may open must cost no screener call")
			}
		})
	}

	t.Run("an old report that is still listed in a view reads", func(t *testing.T) {
		rep := r.report(t, "flood", "low", "clear", -6*24*time.Hour)
		if _, err := r.svc.List(ctx, rep, r.carol, 0); err != nil {
			t.Errorf("List of a report that ended six days ago = %v, want it to read", err)
		}
	})
}

func TestCommentListNames(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)

	fromAlice := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice})
	fromBob := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.bob})
	fromReporter := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.reporter})
	gone := testutil.SeedReporterAccount(t, r.pool, "cms-gone@example.com")
	fromGone := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: gone})
	testutil.MustExec(t, r.pool, "UPDATE accounts SET deleted_at = now() WHERE id = $1", gone)
	heldBob := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.bob, State: "pending"})

	byID := func(l service.CommentList) map[int64]service.CommentView {
		m := map[int64]service.CommentView{}
		for _, c := range l.Comments {
			m[c.ID] = c
		}
		return m
	}

	as := byID(r.list(t, rep, r.carol))
	if got := as[fromAlice]; got.Author != "alice_w" || got.AuthorKind != "user" {
		t.Errorf("alice seen by carol = %+v", got)
	}
	if got := as[fromBob]; got.Author != service.CommentAuthorNewMember || got.AuthorKind != "new_member" {
		t.Errorf("bob (pending name) seen by carol = %+v, want New member", got)
	}
	if got := as[fromGone]; got.Author != service.CommentAuthorDeletedAccount || got.AuthorKind != "deleted_account" {
		t.Errorf("deleted account seen by carol = %+v, want Deleted account", got)
	}
	if got := as[fromReporter]; !got.IsReporter || got.Author != "reporter_r" {
		t.Errorf("reporter seen by carol = %+v, want the Reporter tag", got)
	}
	if _, ok := as[heldBob]; ok {
		t.Errorf("bob's held comment is visible to carol")
	}

	asBob := byID(r.list(t, rep, r.bob))
	if got := asBob[fromBob]; got.Author != "bobby_p" || !got.IsOwn || !got.CanDelete {
		t.Errorf("bob seen by bob = %+v, want his own name", got)
	}
	if got := asBob[heldBob]; !got.Held || got.Author != "bobby_p" {
		t.Errorf("bob's held comment seen by bob = %+v, want Held with his name", got)
	}

	asReporter := byID(r.list(t, rep, r.reporter))
	if got := asReporter[fromReporter]; got.IsReporter || !got.IsOwn {
		t.Errorf("reporter seen by the reporter = %+v, want no tag on yourself", got)
	}
	if got := asReporter[fromAlice]; got.CanDelete {
		t.Errorf("the reporter can delete alice's comment")
	}

	if l := r.list(t, rep, 0); len(l.Comments) == 0 {
		t.Errorf("viewer 0 sees nothing; want the clear comments")
	}

	t.Run("a page of fifty and the cursor", func(t *testing.T) {
		rep := r.open(t)
		var ids []int64
		for i := 0; i < service.CommentPageSize+1; i++ {
			ids = append(ids, testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice}))
		}
		first := r.list(t, rep, r.carol)
		if len(first.Comments) != service.CommentPageSize || !first.More || first.NextAfter != ids[service.CommentPageSize-1] {
			t.Fatalf("first page = %d comments, more %v, next %d; want %d, true, %d",
				len(first.Comments), first.More, first.NextAfter, service.CommentPageSize, ids[service.CommentPageSize-1])
		}
		if first.Count != service.CommentPageSize+1 {
			t.Errorf("Count = %d, want the real number %d", first.Count, service.CommentPageSize+1)
		}
		next, err := r.svc.List(context.Background(), rep, r.carol, first.NextAfter)
		if err != nil {
			t.Fatal(err)
		}
		if len(next.Comments) != 1 || next.More || next.Comments[0].ID != ids[service.CommentPageSize] {
			t.Errorf("second page = %+v, want the last comment and no more", next.Comments)
		}
	})
}

func TestCommentFinishedRescueHidden(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()

	finished := r.report(t, "rescue_needed", "critical", "clear", -time.Hour)
	existing := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: finished, AccountID: r.alice})
	l := r.list(t, finished, r.carol)
	if len(l.Comments) != 0 || l.Open || l.Count != 0 {
		t.Errorf("finished Rescue list = %+v, want nothing and closed", l)
	}
	if _, err := r.post(finished, r.carol, "late", nil); !errors.Is(err, service.ErrCommentsClosed) {
		t.Errorf("Post on a finished Rescue report = %v, want ErrCommentsClosed", err)
	}
	if err := r.svc.Delete(ctx, existing, r.alice); err != nil {
		t.Errorf("Delete on a finished Rescue report = %v, want nil", err)
	}

	t.Run("a live Rescue report lists", func(t *testing.T) {
		live := r.report(t, "rescue_needed", "critical", "clear", 6*time.Hour)
		testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: live, AccountID: r.alice})
		if l := r.list(t, live, r.carol); len(l.Comments) != 1 || !l.Open {
			t.Errorf("live Rescue list = %+v, want one comment and open", l)
		}
	})

	t.Run("a finished report of another category still lists", func(t *testing.T) {
		past := r.report(t, "flood", "low", "clear", -time.Hour)
		testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: past, AccountID: r.alice})
		if l := r.list(t, past, r.carol); len(l.Comments) != 1 || l.Open {
			t.Errorf("past flood list = %+v, want one comment and closed", l)
		}
	})
}

// TestCommentCountHiddenOnFinishedRescue proves review WR-S03 (2026-10-29): a
// finished Rescue report lists no comments (03.1 D-23), so every view of it
// says 0 comments too, the same number List answers, whether it faded or was
// resolved. A finished report of another category keeps its real count.
func TestCommentCountHiddenOnFinishedRescue(t *testing.T) {
	r := newCmsRig(t)
	reports := service.NewReportService(r.q)

	focusCount := func(t *testing.T, id int64) int {
		t.Helper()
		res, err := reports.Focus(context.Background(), service.FocusQuery{
			ReportID: id, Latitude: 16.5062, Longitude: 80.6480, ViewerAccountID: r.carol,
		})
		if err != nil || res.Status != service.FocusOK || res.View == nil {
			t.Fatalf("Focus(%d) = %+v, %v, want a view", id, res, err)
		}
		return res.View.CommentCount
	}

	past := r.report(t, "rescue_needed", "critical", "clear", -time.Hour)
	resolved := r.report(t, "rescue_needed", "critical", "clear", 6*time.Hour)
	for _, id := range []int64{past, resolved} {
		testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: id, AccountID: r.alice})
		testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: id, AccountID: r.carol})
	}
	r.resolve(t, resolved, time.Minute)

	for name, id := range map[string]int64{"faded": past, "resolved": resolved} {
		t.Run(name, func(t *testing.T) {
			listed := r.list(t, id, r.carol).Count
			if got := focusCount(t, id); got != 0 || listed != 0 {
				t.Errorf("finished Rescue: Focus count %d, List count %d, want both 0", got, listed)
			}
		})
	}

	t.Run("a finished flood report keeps its count", func(t *testing.T) {
		flood := r.report(t, "flood", "low", "clear", -time.Hour)
		testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: flood, AccountID: r.alice})
		if got, listed := focusCount(t, flood), r.list(t, flood, r.carol).Count; got != 1 || listed != 1 {
			t.Errorf("past flood: Focus count %d, List count %d, want both 1", got, listed)
		}
	})
}

func TestCommentFlag(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()
	rep := r.open(t)
	c := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice})
	side := cmsSideRows(t, r)

	flagRows := func() int {
		var n int
		if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM comment_flags WHERE comment_id = $1", c).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	spam := []service.FlagReason{service.FlagSpam}

	t.Run("a flag by another account stores the reasons and the note", func(t *testing.T) {
		err := r.svc.Flag(ctx, c, r.carol, service.FlagInput{
			Reasons: []service.FlagReason{service.FlagOther, service.FlagSpam}, Note: "  looks like an advert  ",
		})
		if err != nil {
			t.Fatalf("Flag = %v", err)
		}
		var reasons []string
		var note *string
		if err := r.pool.QueryRow(ctx,
			"SELECT reasons, note FROM comment_flags WHERE comment_id = $1 AND account_id = $2", c, r.carol,
		).Scan(&reasons, &note); err != nil {
			t.Fatal(err)
		}
		if len(reasons) != 2 || reasons[0] != "spam" || reasons[1] != "other" || note == nil || *note != "looks like an advert" {
			t.Errorf("stored reasons %v note %v, want spam and other in list order and the trimmed note", reasons, note)
		}
	})

	t.Run("a second flag is already flagged", func(t *testing.T) {
		if err := r.svc.Flag(ctx, c, r.carol, service.FlagInput{Reasons: spam}); !errors.Is(err, service.ErrAlreadyFlagged) {
			t.Errorf("second Flag = %v, want ErrAlreadyFlagged", err)
		}
		if n := flagRows(); n != 1 {
			t.Errorf("flag rows = %d, want 1", n)
		}
	})

	t.Run("a flag never hides the comment", func(t *testing.T) {
		for _, viewer := range []int64{r.alice, r.carol, r.bob} {
			l := r.list(t, rep, viewer)
			if len(l.Comments) != 1 || l.Comments[0].ID != c {
				t.Fatalf("viewer %d sees %+v, want the flagged comment still listed", viewer, l.Comments)
			}
			if got := l.Comments[0].ViewerFlagged; got != (viewer == r.carol) {
				t.Errorf("viewer %d ViewerFlagged = %v", viewer, got)
			}
		}
	})

	t.Run("your own comment cannot be flagged", func(t *testing.T) {
		if err := r.svc.Flag(ctx, c, r.alice, service.FlagInput{Reasons: spam}); !errors.Is(err, service.ErrCommentOwnFlag) {
			t.Errorf("own Flag = %v, want ErrCommentOwnFlag", err)
		}
	})

	t.Run("the reasons are validated first", func(t *testing.T) {
		var ve service.ValidationError
		if err := r.svc.Flag(ctx, c, r.bob, service.FlagInput{}); !errors.As(err, &ve) {
			t.Errorf("Flag with no reason = %v, want a ValidationError", err)
		}
	})

	t.Run("a refused note stores nothing", func(t *testing.T) {
		r.scr.outcome = "reject"
		defer func() { r.scr.outcome = "clear" }()
		err := r.svc.Flag(ctx, c, r.bob, service.FlagInput{Reasons: spam, Note: "some hateful words"})
		var nr service.FlagNoteRejectedError
		if !errors.As(err, &nr) || !errors.Is(err, service.ErrFlagNoteRejected) {
			t.Fatalf("Flag = %v, want a FlagNoteRejectedError", err)
		}
		if n := flagRows(); n != 1 {
			t.Errorf("flag rows = %d, want still 1", n)
		}
	})

	t.Run("a note the check could not finish still stores the flag", func(t *testing.T) {
		r.scr.outcome = "unfinished"
		defer func() { r.scr.outcome = "clear" }()
		if err := r.svc.Flag(ctx, c, r.bob, service.FlagInput{Reasons: spam, Note: "please look"}); err != nil {
			t.Errorf("Flag = %v, want it stored", err)
		}
		if n := flagRows(); n != 2 {
			t.Errorf("flag rows = %d, want 2", n)
		}
	})

	t.Run("a flag on a closed report is allowed", func(t *testing.T) {
		closed := r.open(t)
		r.resolve(t, closed, time.Hour)
		id := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: closed, AccountID: r.alice})
		if err := r.svc.Flag(ctx, id, r.carol, service.FlagInput{Reasons: spam}); err != nil {
			t.Errorf("Flag on a resolved report = %v, want nil", err)
		}
		faded := r.report(t, "flood", "low", "clear", -time.Hour)
		id = testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: faded, AccountID: r.alice})
		if err := r.svc.Flag(ctx, id, r.carol, service.FlagInput{Reasons: spam}); err != nil {
			t.Errorf("Flag on a faded report = %v, want nil", err)
		}
	})

	t.Run("a report nobody may open answers not found", func(t *testing.T) {
		for name, rp := range map[string]int64{
			"too old":  r.report(t, "flood", "low", "clear", -8*24*time.Hour),
			"withheld": r.report(t, "flood", "low", "pending", 6*time.Hour),
		} {
			id := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rp, AccountID: r.alice})
			if err := r.svc.Flag(ctx, id, r.carol, service.FlagInput{Reasons: spam}); !errors.Is(err, service.ErrReportNotFound) {
				t.Errorf("%s: Flag = %v, want ErrReportNotFound", name, err)
			}
		}
	})

	t.Run("a comment nobody else can see is gone", func(t *testing.T) {
		for name, seed := range map[string]testutil.CommentSeed{
			"deleted": {ReportID: rep, AccountID: r.alice, Deleted: true},
			"removed": {ReportID: rep, AccountID: r.alice, State: "removed"},
			"held":    {ReportID: rep, AccountID: r.alice, State: "pending"},
		} {
			id := testutil.SeedComment(t, r.pool, seed)
			if err := r.svc.Flag(ctx, id, r.carol, service.FlagInput{Reasons: spam}); !errors.Is(err, service.ErrCommentGone) {
				t.Errorf("%s: Flag = %v, want ErrCommentGone", name, err)
			}
		}
		if err := r.svc.Flag(ctx, 999999, r.carol, service.FlagInput{Reasons: spam}); !errors.Is(err, service.ErrCommentGone) {
			t.Errorf("unknown id: Flag = %v, want ErrCommentGone", err)
		}
	})

	t.Run("a finished Rescue report shows no comments, so none can be flagged", func(t *testing.T) {
		fin := r.report(t, "rescue_needed", "critical", "clear", -time.Hour)
		id := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: fin, AccountID: r.alice})
		if err := r.svc.Flag(ctx, id, r.carol, service.FlagInput{Reasons: spam}); !errors.Is(err, service.ErrCommentGone) {
			t.Errorf("Flag = %v, want ErrCommentGone", err)
		}
	})

	// Last, so every flag above has been made: none wrote a row or called out.
	t.Run("a flag writes no update and no outbox row and calls no notifier", func(t *testing.T) {
		if got := cmsSideRows(t, r); got != side {
			t.Errorf("update and outbox rows %d -> %d, want none written", side, got)
		}
		if r.note.calls != 0 {
			t.Errorf("notifier calls = %d, want 0", r.note.calls)
		}
	})
}

// TestCommentLikeAccess covers the access rules of Like and Unlike: a closed
// report, a comment that is gone, and a report nobody may open.
func TestCommentLikeAccess(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()
	rep := r.open(t)

	for name, seed := range map[string]testutil.CommentSeed{
		"deleted":            {ReportID: rep, AccountID: r.alice, Deleted: true},
		"removed":            {ReportID: rep, AccountID: r.alice, State: "removed"},
		"held, someone else": {ReportID: rep, AccountID: r.alice, State: "pending"},
		"held, your own":     {ReportID: rep, AccountID: r.carol, State: "pending"},
	} {
		id := testutil.SeedComment(t, r.pool, seed)
		if _, err := r.svc.Like(ctx, id, r.carol); !errors.Is(err, service.ErrCommentGone) {
			t.Errorf("%s: Like = %v, want ErrCommentGone", name, err)
		}
		if _, err := r.svc.Unlike(ctx, id, r.carol); !errors.Is(err, service.ErrCommentGone) {
			t.Errorf("%s: Unlike = %v, want ErrCommentGone", name, err)
		}
	}
	if _, err := r.svc.Like(ctx, 999999, r.carol); !errors.Is(err, service.ErrCommentGone) {
		t.Errorf("unknown id: Like = %v, want ErrCommentGone", err)
	}

	t.Run("a closed report stops likes and unlikes", func(t *testing.T) {
		resolved := r.open(t)
		r.resolve(t, resolved, time.Hour)
		faded := r.report(t, "flood", "low", "clear", -time.Hour)
		for name, rp := range map[string]int64{"resolved": resolved, "faded": faded} {
			id := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rp, AccountID: r.alice})
			if _, err := r.svc.Like(ctx, id, r.carol); !errors.Is(err, service.ErrCommentsClosed) {
				t.Errorf("%s: Like = %v, want ErrCommentsClosed", name, err)
			}
			if _, err := r.svc.Unlike(ctx, id, r.carol); !errors.Is(err, service.ErrCommentsClosed) {
				t.Errorf("%s: Unlike = %v, want ErrCommentsClosed", name, err)
			}
		}
	})

	t.Run("a report nobody may open is not found, even for a deleted comment", func(t *testing.T) {
		for name, rp := range map[string]int64{
			"too old":  r.report(t, "flood", "low", "clear", -8*24*time.Hour),
			"withheld": r.report(t, "flood", "low", "removed", 6*time.Hour),
		} {
			live := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rp, AccountID: r.alice})
			dead := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rp, AccountID: r.alice, Deleted: true})
			for _, id := range []int64{live, dead} {
				if _, err := r.svc.Like(ctx, id, r.carol); !errors.Is(err, service.ErrReportNotFound) {
					t.Errorf("%s: Like = %v, want ErrReportNotFound", name, err)
				}
				if _, err := r.svc.Unlike(ctx, id, r.carol); !errors.Is(err, service.ErrReportNotFound) {
					t.Errorf("%s: Unlike = %v, want ErrReportNotFound", name, err)
				}
			}
		}
	})
}
