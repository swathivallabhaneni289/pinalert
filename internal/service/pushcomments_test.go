package service

// pushcomments_test.go proves the phone half of comment events (Phase 8 plan
// 15, D-16, PAGE-02, A-P8-9): the two fixed phrases a lock screen may show, the
// send time rule, the commented arm of the push follow set, and that a like or
// a flag queues nothing. The word tests need no database; the follow and queue
// tests use the pushDBRig of pushdispatch_db_test.go and skip in the quick run.

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestCommentPushStillHolds proves the query behind review IN-S05 (2026-10-29)
// on a real database: a queued comment note holds only while a clear, undeleted
// comment it could be about is there, written by someone else, of the right
// shape for the kind, and newer than the recipient's last read note of that
// kind on that report.
func TestCommentPushStillHolds(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	reporter := testutil.SeedReporterAccount(t, pool, "in-s05-reporter@example.com", "in-s05-reporter")
	first := testutil.SeedReporterAccount(t, pool, "in-s05-first@example.com")
	second := testutil.SeedReporterAccount(t, pool, "in-s05-second@example.com")
	now := time.Now()
	newReport := func() int64 {
		return testutil.InsertStoredReport(t, pool, testutil.StoredReport{
			SessionID: "in-s05-reporter", Category: "flood", Severity: "low", Description: "water",
			Latitude: 16.5062, Longitude: 80.6480, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
		})
	}
	holds := func(report, recipient int64, kind UpdateKind) bool {
		t.Helper()
		got, err := q.CommentPushStillHolds(ctx, sqlcgen.CommentPushStillHoldsParams{
			ReportID: report, RecipientAccountID: recipient, Kind: string(kind),
		})
		if err != nil {
			t.Fatalf("CommentPushStillHolds: %v", err)
		}
		return got
	}

	t.Run("comment on your report", func(t *testing.T) {
		report := newReport()
		id := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: first})
		if !holds(report, reporter, UpdateCommentOnYours) {
			t.Error("a clear comment does not hold")
		}
		testutil.MustExec(t, pool, `UPDATE comments SET moderation_state = 'removed' WHERE id = $1`, id)
		if holds(report, reporter, UpdateCommentOnYours) {
			t.Error("a removed comment still holds")
		}
		again := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: first})
		if !holds(report, reporter, UpdateCommentOnYours) {
			t.Error("a second clear comment does not hold")
		}
		testutil.MustExec(t, pool, `UPDATE comments SET deleted_at = now(), body = '' WHERE id = $1`, again)
		if holds(report, reporter, UpdateCommentOnYours) {
			t.Error("a deleted comment still holds")
		}
		testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: reporter})
		if holds(report, reporter, UpdateCommentOnYours) {
			t.Error("the recipient's own comment holds")
		}
	})

	t.Run("reply to your comment", func(t *testing.T) {
		report := newReport()
		root := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: first})
		if holds(report, first, UpdateReplyToYours) {
			t.Error("a thread with no reply holds a reply note")
		}
		other := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: second})
		testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: reporter, ParentID: other})
		if holds(report, first, UpdateReplyToYours) {
			t.Error("a reply to someone else holds a reply note")
		}
		reply := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: second, ParentID: root})
		if !holds(report, first, UpdateReplyToYours) {
			t.Error("a clear reply to your comment does not hold")
		}
		testutil.MustExec(t, pool, `UPDATE comments SET moderation_state = 'removed' WHERE id = $1`, reply)
		if holds(report, first, UpdateReplyToYours) {
			t.Error("a removed reply still holds")
		}
	})

	t.Run("only comments after the last read note count", func(t *testing.T) {
		report := newReport()
		testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: first})
		testutil.MustExec(t, pool,
			`INSERT INTO updates (account_id, report_id, kind, read_at) VALUES ($1, $2, 'comment_on_yours', now() - interval '1 minute')`,
			reporter, report)
		if !holds(report, reporter, UpdateCommentOnYours) {
			t.Error("a comment newer than the last read note does not hold")
		}
		testutil.MustExec(t, pool,
			`INSERT INTO updates (account_id, report_id, kind, read_at) VALUES ($1, $2, 'comment_on_yours', now() + interval '1 minute')`,
			reporter, report)
		if holds(report, reporter, UpdateCommentOnYours) {
			t.Error("a comment the recipient already read about still holds")
		}
	})
}

func TestPushCommentWords(t *testing.T) {
	const (
		onWords    = "has a new comment"
		replyWords = "has a new reply"
	)

	t.Run("the fixed phrases", func(t *testing.T) {
		cases := []struct {
			name      string
			event     UpdateKind
			own       bool
			wantTitle string
		}{
			{"comment on a Flood report", UpdateCommentOnYours, true, "Flood report near Benz Circle has a new comment"},
			{"comment, not the reporter's own", UpdateCommentOnYours, false, "Flood report near Benz Circle has a new comment"},
			{"reply", UpdateReplyToYours, false, "Flood report near Benz Circle has a new reply"},
			{"reply to a reporter", UpdateReplyToYours, true, "Flood report near Benz Circle has a new reply"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				got, ok := BuildPushText(PushTextInput{
					Kind: PushKindChanged, Event: c.event, OwnReport: c.own,
					Category: CategoryFlood, Severity: SeverityLow, AreaName: "Benz Circle",
				})
				if !ok {
					t.Fatal("BuildPushText gave no text")
				}
				if got.Title != c.wantTitle {
					t.Errorf("title = %q, want %q", got.Title, c.wantTitle)
				}
				if want := "Flood, Low. Benz Circle."; got.Body != want {
					t.Errorf("body = %q, want %q", got.Body, want)
				}
			})
		}
	})

	t.Run("no comment words, no username, no count, no noise word", func(t *testing.T) {
		km := 2.0
		noise := regexp.MustCompile(`(?i)\b(` + strings.Join(bannedNoiseWords, "|") + `)\b`)
		for _, cat := range Categories {
			for _, area := range []string{"", "Benz Circle", "Mahatma Gandhi Road Junction Stop Number Nine"} {
				for _, event := range []UpdateKind{UpdateCommentOnYours, UpdateReplyToYours} {
					for _, own := range []bool{false, true} {
						in := PushTextInput{
							Kind: PushKindChanged, Event: event, OwnReport: own,
							Category: cat, Severity: SeverityMedium, AreaName: area, DistanceKm: &km,
						}
						got, ok := BuildPushText(in)
						if !ok {
							t.Fatalf("no text for %+v", in)
						}
						want := onWords
						if event == UpdateReplyToYours {
							want = replyWords
						}
						if !strings.HasSuffix(got.Title, want) {
							t.Errorf("title %q does not end with %q", got.Title, want)
						}
						if n := utf8.RuneCountInString(got.Title); n > PushTitleMaxRunes {
							t.Errorf("title %q is %d runes, over %d", got.Title, n, PushTitleMaxRunes)
						}
						if n := utf8.RuneCountInString(got.Body); n > PushBodyMaxRunes {
							t.Errorf("body %q is %d runes, over %d", got.Body, n, PushBodyMaxRunes)
						}
						if strings.Contains(strings.ToLower(got.Body), "comment") || strings.Contains(strings.ToLower(got.Body), "repl") {
							t.Errorf("body %q names a comment", got.Body)
						}
						if strings.ContainsAny(got.Title+got.Body, "@") {
							t.Errorf("text %q %q holds an at sign", got.Title, got.Body)
						}
						if noise.MatchString(got.Title) || noise.MatchString(got.Body) {
							t.Errorf("text %q %q holds a noise word", got.Title, got.Body)
						}
						for _, r := range got.Title + got.Body {
							if r > 0x7e && r != '…' {
								t.Errorf("text %q %q holds the non plain rune %q", got.Title, got.Body, r)
							}
						}
					}
				}
			}
		}
	})

	t.Run("a long area is dropped before the title passes its cap", func(t *testing.T) {
		got, ok := BuildPushText(PushTextInput{
			Kind: PushKindChanged, Event: UpdateCommentOnYours,
			Category: CategoryStormCyclone, Severity: SeverityLow,
			AreaName: "Mahatma Gandhi Road Junction Stop Number Nine",
		})
		if !ok {
			t.Fatal("BuildPushText gave no text")
		}
		if got.Title != "Storm/Cyclone damage report has a new comment" {
			t.Errorf("title = %q", got.Title)
		}
	})

	t.Run("an unknown category gives no text", func(t *testing.T) {
		if _, ok := BuildPushText(PushTextInput{
			Kind: PushKindChanged, Event: UpdateCommentOnYours,
			Category: Category("zzz"), Severity: SeverityLow,
		}); ok {
			t.Error("a comment note was built for an unknown category")
		}
	})

	t.Run("the word table knows exactly the two comment phrases", func(t *testing.T) {
		if w, ok := changeEventWords(UpdateCommentOnYours); !ok || w != onWords {
			t.Errorf("changeEventWords(comment_on_yours) = %q, %v", w, ok)
		}
		if w, ok := changeEventWords(UpdateReplyToYours); !ok || w != replyWords {
			t.Errorf("changeEventWords(reply_to_yours) = %q, %v", w, ok)
		}
		if _, ok := changeEventWords(UpdateMore); ok {
			t.Error("changeEventWords knows more")
		}
	})
}

func TestEventStillHoldsComment(t *testing.T) {
	w := &PushWorker{}
	now := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	facts := func(v Visibility, expires time.Time) pushReportFacts {
		return pushReportFacts{Vis: v, ExpiresAt: expires}
	}
	cases := []struct {
		name string
		f    pushReportFacts
		want bool
	}{
		{"active Live", facts(VisibilityLive, now.Add(3*time.Hour)), true},
		{"Provisional", facts(VisibilityProvisional, now.Add(3*time.Hour)), false},
		{"disputed (Hidden)", facts(VisibilityHidden, now.Add(3*time.Hour)), false},
		{"resolved (Retracted)", facts(VisibilityRetracted, now.Add(3*time.Hour)), false},
		{"expired Live", facts(VisibilityLive, now.Add(-time.Minute)), false},
		{"expired exactly now", facts(VisibilityLive, now), false},
		{"withheld", facts(VisibilityWithheld, now.Add(3*time.Hour)), false},
	}
	for _, event := range []UpdateKind{UpdateCommentOnYours, UpdateReplyToYours} {
		for _, c := range cases {
			t.Run(string(event)+" "+c.name, func(t *testing.T) {
				if got := w.eventStillHolds(event, c.f, now, false); got != c.want {
					t.Errorf("eventStillHolds(%s, %s) = %v, want %v", event, c.name, got, c.want)
				}
			})
		}
	}
}

// pcNotifier runs the dispatcher at once for every poke, in the test's own
// goroutine, so a test can read the outbox right after a comment is stored.
type pcNotifier struct {
	r     *pushDBRig
	calls int
}

func (n *pcNotifier) ReportChanged(reportID, actor int64) {
	n.calls++
	n.r.eval(reportID, actor)
}

// pcRig adds a CommentService wired to the dispatcher to a pushDBRig.
type pcRig struct {
	*pushDBRig
	svc  *CommentService
	note *pcNotifier
}

func newPCRig(t *testing.T) *pcRig {
	t.Helper()
	base := newPushDBRig(t)
	note := &pcNotifier{r: base}
	return &pcRig{
		pushDBRig: base,
		note:      note,
		svc: NewCommentService(base.q,
			WithCommentClock(func() time.Time { return base.cur }),
			WithCommentChangeNotifier(note),
		),
	}
}

// post stores one comment as the account.
func (r *pcRig) post(reportID, account int64, words string, replyTo *int64) CommentView {
	r.t.Helper()
	v, err := r.svc.Post(context.Background(), CommentInput{
		ReportID: reportID, AccountID: account, HasUsername: true, Words: words, ReplyTo: replyTo,
	})
	if err != nil {
		r.t.Fatalf("Post(%q) = %v, want it stored", words, err)
	}
	return v
}

// reporter makes an account that wants Changes pushes (changes true) or not,
// and a report posted from its session. The account has a home cell and a
// device.
func (r *pcRig) reporter(prefix string, changes bool) (account, report int64) {
	r.t.Helper()
	id, session := r.reader(prefix, 0, 5, false, false, changes)
	r.cur = r.base.Add(time.Minute)
	return id, r.report(session, SeverityLow, r.base, "clear")
}

// pushedKinds lists, in row order, "account:update kind" for each changed row
// queued for the report.
func (r *pcRig) pushedKinds(reportID int64) []string {
	r.t.Helper()
	rows, err := r.pool.Query(context.Background(),
		`SELECT o.account_id, u.kind
		   FROM push_outbox o
		   JOIN updates u ON u.id = o.update_id
		  WHERE o.report_id = $1 AND o.kind = 'changed'
		  ORDER BY o.id`, reportID)
	if err != nil {
		r.t.Fatalf("reading the changed rows: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var account int64
		var kind string
		if err := rows.Scan(&account, &kind); err != nil {
			r.t.Fatalf("scanning a changed row: %v", err)
		}
		out = append(out, fmt.Sprintf("%d:%s", account, kind))
	}
	if err := rows.Err(); err != nil {
		r.t.Fatalf("reading the changed rows: %v", err)
	}
	return out
}

func (r *pcRig) followers(reportID int64) map[int64]bool {
	r.t.Helper()
	ids, err := r.q.FollowersForBellRefresh(context.Background(), reportID)
	if err != nil {
		r.t.Fatalf("FollowersForBellRefresh: %v", err)
	}
	out := map[int64]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func TestCommenterFollowsReportForPush(t *testing.T) {
	t.Run("only a clear, undeleted comment makes its author a push follower", func(t *testing.T) {
		r := newPCRig(t)
		reporter, report := r.reporter("pc-fol-rep", true)

		clearID, _ := r.reader("pc-fol-clear", 1, 5, false, false, true)
		deletedID, _ := r.reader("pc-fol-deleted", 1, 5, false, false, true)
		removedID, _ := r.reader("pc-fol-removed", 1, 5, false, false, true)
		pendingID, _ := r.reader("pc-fol-pending", 1, 5, false, false, true)
		offID, _ := r.reader("pc-fol-off", 1, 5, false, false, false)
		noneID, _ := r.reader("pc-fol-none", 1, 5, false, false, true)

		seed := func(account int64, state string, deleted bool) {
			testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: report, AccountID: account, State: state, Deleted: deleted})
		}
		seed(clearID, "clear", false)
		seed(deletedID, "clear", true)
		seed(removedID, "removed", false)
		seed(pendingID, "pending", false)
		seed(offID, "clear", false)

		got := r.followers(report)
		if !got[reporter] {
			t.Error("the reporter is not a push follower")
		}
		if !got[clearID] {
			t.Error("an account whose only link is a clear comment is not a push follower")
		}
		// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box):
		// the set is the accounts whose bell is refreshed early and no stored
		// Changes switch picks it, so a clear comment with Changes off counts.
		if !got[offID] {
			t.Error("an account with a clear comment and Changes switched off is not in the bell refresh set")
		}
		for name, id := range map[string]int64{
			"deleted comment": deletedID, "removed comment": removedID, "held comment": pendingID,
			"no link at all": noneID,
		} {
			if got[id] {
				t.Errorf("an account with %s is a push follower", name)
			}
		}
	})

	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box):
	// comment notes stay in the bell, so the reporter gets the bell row and no
	// changed phone row is queued, whatever the stored Changes switch says.
	t.Run("a comment on your report writes the bell row and queues no changed row", func(t *testing.T) {
		r := newPCRig(t)
		reporter, report := r.reporter("pc-q-rep", true)
		commenter, _ := r.reader("pc-q-com", 1, 5, false, false, true)

		r.post(report, commenter, "water is rising here", nil)
		if got := r.pushedKinds(report); len(got) != 0 {
			t.Fatalf("changed rows = %v, want none", got)
		}
		if n := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'comment_on_yours'`, reporter, report); n != 1 {
			t.Errorf("bell rows for the reporter = %d, want 1", n)
		}
		if r.note.calls != 1 {
			t.Errorf("dispatcher pokes = %d, want 1", r.note.calls)
		}
	})

	t.Run("Changes off queues nothing", func(t *testing.T) {
		r := newPCRig(t)
		_, report := r.reporter("pc-off-rep", false)
		commenter, _ := r.reader("pc-off-com", 1, 5, false, false, true)

		r.post(report, commenter, "water is rising here", nil)
		if got := r.pushedKinds(report); len(got) != 0 {
			t.Errorf("changed rows = %v, want none for an account with Changes off", got)
		}
		if n := r.count(`SELECT COUNT(*) FROM updates WHERE report_id = $1 AND kind = 'comment_on_yours'`, report); n != 1 {
			t.Errorf("bell rows = %d, want 1: the bell still tells the reporter", n)
		}
	})

	t.Run("a commenter hears about a reply like a voter", func(t *testing.T) {
		r := newPCRig(t)
		_, report := r.reporter("pc-rep-rep", true)
		first, _ := r.reader("pc-rep-first", 1, 5, false, false, true)
		second, _ := r.reader("pc-rep-second", 1, 5, false, false, true)

		root := r.post(report, first, "first words", nil)
		r.post(report, second, "an answer", &root.ID)

		// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box):
		// the reply reaches the first commenter's bell and no phone.
		if got := r.pushedKinds(report); len(got) != 0 {
			t.Errorf("changed rows = %v, want none", got)
		}
		if n := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reply_to_yours'`, first, report); n != 1 {
			t.Errorf("bell rows for the first commenter = %d, want 1", n)
		}
		if n := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reply_to_yours'`, second, report); n != 0 {
			t.Errorf("the replier got %d bell rows about their own reply", n)
		}
	})
}

func TestLikesNotifyNoPush(t *testing.T) {
	r := newPCRig(t)
	ctx := context.Background()
	_, report := r.reporter("pc-like-rep", true)
	author, _ := r.reader("pc-like-author", 1, 5, false, false, true)

	c := r.post(report, author, "words to like", nil)
	before := r.count(`SELECT COUNT(*) FROM push_outbox`)
	pokes := r.note.calls
	bell := r.count(`SELECT COUNT(*) FROM updates`)

	for i := 0; i < 10; i++ {
		liker := testutil.SeedReporterAccount(t, r.pool, fmt.Sprintf("pc-liker-%d@example.com", i))
		if _, err := r.svc.Like(ctx, c.ID, liker); err != nil {
			t.Fatalf("Like %d: %v", i, err)
		}
	}
	flagger := testutil.SeedReporterAccount(t, r.pool, "pc-flagger@example.com")
	if err := r.svc.Flag(ctx, c.ID, flagger, FlagInput{Reasons: []FlagReason{FlagSpam}}); err != nil {
		t.Fatalf("Flag: %v", err)
	}

	// The bell poll of every account that could be told: nothing is new.
	for _, id := range r.followersOf(report) {
		if err := r.updates.Refresh(ctx, id); err != nil {
			t.Fatalf("Refresh(%d): %v", id, err)
		}
	}
	if got := r.count(`SELECT COUNT(*) FROM push_outbox`); got != before {
		t.Errorf("push_outbox rows %d -> %d: likes and a flag must queue none", before, got)
	}
	if got := r.count(`SELECT COUNT(*) FROM updates`); got != bell {
		t.Errorf("updates rows %d -> %d: likes and a flag must write none", bell, got)
	}
	if r.note.calls != pokes {
		t.Errorf("dispatcher pokes %d -> %d: a like or a flag must not poke", pokes, r.note.calls)
	}
}

// followersOf lists the push follower ids of a report, in id order.
func (r *pcRig) followersOf(reportID int64) []int64 {
	r.t.Helper()
	ids, err := r.q.FollowersForBellRefresh(context.Background(), reportID)
	if err != nil {
		r.t.Fatalf("FollowersForBellRefresh: %v", err)
	}
	return ids
}
