package service_test

// review_ask_test.go proves quick 261118-rz6 on a real Postgres: Remove needs
// one of five reasons and stores it, the late text check stores text_check,
// the author's asks for review follow their rules (author only, removed only,
// one open at a time, 3 in total, concurrent asks cannot pass either), Restore
// and Keep removed change exactly what they should under the row locks, an own
// bell row that happens again comes back unread and newest, a restored report
// can never be hidden by flags again, and its author's rating counts it by its
// votes. Every test starts with testutil.NewTestDB, so it skips in the quick
// run. It reuses the review hide rig of flag_review_test.go.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// rzRig adds the author's side to the review hide rig.
type rzRig struct {
	*frRig
	asks *service.ReviewAskService
}

func newRzRig(t *testing.T) *rzRig {
	t.Helper()
	r := newFrRig(t)
	return &rzRig{frRig: r, asks: service.NewReviewAskService(r.q, service.PgxReviewAskTx(r.pool))}
}

// removed stores one report by the poster and removes it with reason.
func (r *rzRig) removed(reason service.RemovalReason) int64 {
	r.t.Helper()
	id := r.report(service.SeverityLow, service.ModerationClear)
	if _, err := r.mods.Remove(context.Background(), id, r.mod, reason); err != nil {
		r.t.Fatalf("Remove report %d: %v", id, err)
	}
	return id
}

func (r *rzRig) str(sql string, args ...any) string {
	r.t.Helper()
	var s *string
	if err := r.pool.QueryRow(context.Background(), sql, args...).Scan(&s); err != nil {
		r.t.Fatalf("reading with %q: %v", sql, err)
	}
	if s == nil {
		return "<null>"
	}
	return *s
}

func (r *rzRig) ask(id int64, note string) (service.ReviewAskBox, error) {
	return r.asks.Ask(context.Background(), r.poster, id, note)
}

func (r *rzRig) mustAsk(id int64) {
	r.t.Helper()
	if _, err := r.ask(id, ""); err != nil {
		r.t.Fatalf("Ask on report %d: %v", id, err)
	}
}

func (r *rzRig) mustKeepRemoved(id int64) {
	r.t.Helper()
	if _, err := r.mods.KeepRemoved(context.Background(), id, r.mod); err != nil {
		r.t.Fatalf("KeepRemoved on report %d: %v", id, err)
	}
}

func (r *rzRig) state3(id int64) service.ReviewAskBox {
	r.t.Helper()
	box, err := r.asks.State(context.Background(), r.poster, id)
	if err != nil {
		r.t.Fatalf("State of report %d: %v", id, err)
	}
	return box
}

// writes counts every row Restore and Keep removed may write for a report.
func (r *rzRig) writes(id int64) string {
	r.t.Helper()
	return fmt.Sprintf("actions %d, updates %d, answered asks %d, state %s",
		r.count(`SELECT count(*) FROM moderation_actions WHERE report_id = $1`, id),
		r.count(`SELECT count(*) FROM updates WHERE report_id = $1`, id),
		r.count(`SELECT count(*) FROM report_review_asks WHERE report_id = $1 AND answered_at IS NOT NULL`, id),
		r.state(id))
}

func TestRemoveNeedsAReason(t *testing.T) {
	r := newRzRig(t)
	ctx := context.Background()

	t.Run("a Remove without one of the five reasons is refused and writes nothing", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		for _, bad := range []service.RemovalReason{"", service.RemovalTextCheck, "rude", "Spam"} {
			if _, err := r.mods.Remove(ctx, id, r.mod, bad); !errors.Is(err, service.ErrRemovalReasonMissing) {
				t.Errorf("Remove with %q: %v, want ErrRemovalReasonMissing", bad, err)
			}
		}
		if got := r.state(id); got != "clear" {
			t.Errorf("state = %q, want clear", got)
		}
		if n := r.count(`SELECT count(*) FROM moderation_actions WHERE report_id = $1`, id); n != 0 {
			t.Errorf("%d audit rows, want 0", n)
		}
	})

	t.Run("each reason is stored on the report and the audit row and read back", func(t *testing.T) {
		for _, reason := range service.RemovalReasons {
			id := r.removed(reason)
			if got := r.str(`SELECT removed_reason FROM reports WHERE id = $1`, id); got != string(reason) {
				t.Errorf("%s: report reason %q", reason, got)
			}
			if got := r.str(`SELECT reason FROM moderation_actions WHERE report_id = $1 AND action = 'remove'`, id); got != string(reason) {
				t.Errorf("%s: audit reason %q", reason, got)
			}
			if n := r.count(`SELECT count(*) FROM reports WHERE id = $1 AND removed_at IS NOT NULL`, id); n != 1 {
				t.Errorf("%s: no removal time", reason)
			}
			row, _, err := r.mods.Find(ctx, id)
			if err != nil {
				t.Fatalf("Find: %v", err)
			}
			if row.RemovedReason != reason {
				t.Errorf("%s: Find reads %q", reason, row.RemovedReason)
			}
			if box := r.state3(id); box.Why != "Why: "+service.RemovalReasonLabel(reason) || box.State != service.ReviewAskCanAsk {
				t.Errorf("%s: the author's box is %+v", reason, box)
			}
		}
	})

	t.Run("Keep clears no reason of a report that is not removed", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		testutil.MustExec(t, r.pool, `INSERT INTO report_flags (report_id, account_id, reasons) VALUES ($1, $2, ARRAY['spam'])`, id, r.flagger())
		if _, err := r.mods.Keep(ctx, id, r.mod); err != nil {
			t.Fatalf("Keep: %v", err)
		}
		if got := r.str(`SELECT removed_reason FROM reports WHERE id = $1`, id); got != "<null>" {
			t.Errorf("a kept report has the reason %q", got)
		}
		if got := r.str(`SELECT reason FROM moderation_actions WHERE report_id = $1`, id); got != "<null>" {
			t.Errorf("a Keep audit row has the reason %q", got)
		}
	})
}

func TestLateTextCheckRecordsTextCheck(t *testing.T) {
	r := newRzRig(t)
	ctx := context.Background()
	now := time.Now().UTC()
	pending := func() int64 {
		id := insertPendingReport(t, r.pool, r.q, service.SeverityLow, service.CategoryFlood, mwDesc, now.Add(-time.Minute), now.Add(6*time.Hour))
		testutil.MustExec(t, r.pool, `UPDATE reports SET session_id = $1 WHERE id = $2`, frPosterSess, id)
		return id
	}

	testutil.MustExec(t, r.pool, `DELETE FROM moderation_checks`)
	rejected := pending()
	if err := mwDBWorker(r.q, screenerWith(mwRejectScores), now).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if got := r.state(rejected); got != "removed" {
		t.Fatalf("state = %q, want removed", got)
	}
	if got := r.str(`SELECT removed_reason FROM reports WHERE id = $1`, rejected); got != "text_check" {
		t.Errorf("reason = %q, want text_check", got)
	}
	if n := r.count(`SELECT count(*) FROM reports WHERE id = $1 AND removed_at IS NOT NULL`, rejected); n != 1 {
		t.Error("a late reject has no removal time")
	}
	if box := r.state3(rejected); box.Why != "Why: Text check" {
		t.Errorf("the box says %q, want Why: Text check", box.Why)
	}

	testutil.MustExec(t, r.pool, `DELETE FROM moderation_checks`)
	passed := pending()
	if err := mwDBWorker(r.q, screenerWith(mwPassScores), now).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if got := r.str(`SELECT removed_reason FROM reports WHERE id = $1`, passed); got != "<null>" || r.state(passed) != "clear" {
		t.Errorf("a late pass: reason %q state %q, want no reason and clear", got, r.state(passed))
	}
}

func TestReviewAskRules(t *testing.T) {
	r := newRzRig(t)
	ctx := context.Background()
	stranger := r.flagger()

	t.Run("only the author, only while removed; everything else looks like no report", func(t *testing.T) {
		id := r.removed(service.RemovalFalse)
		if _, err := r.asks.State(ctx, stranger, id); !errors.Is(err, service.ErrReportNotFound) {
			t.Errorf("a stranger's State: %v", err)
		}
		if _, err := r.asks.Ask(ctx, stranger, id, ""); !errors.Is(err, service.ErrReportNotFound) {
			t.Errorf("a stranger's Ask: %v", err)
		}
		for _, st := range []service.ModerationState{service.ModerationClear, service.ModerationPending, service.ModerationReview} {
			other := r.report(service.SeverityLow, st)
			if _, err := r.asks.State(ctx, r.poster, other); !errors.Is(err, service.ErrReportNotFound) {
				t.Errorf("State of a %s report: %v", st, err)
			}
			if _, err := r.ask(other, ""); !errors.Is(err, service.ErrReportNotFound) {
				t.Errorf("Ask on a %s report: %v", st, err)
			}
		}
		if _, err := r.ask(999999, ""); !errors.Is(err, service.ErrReportNotFound) {
			t.Errorf("Ask on no report: %v", err)
		}
		if n := r.count(`SELECT count(*) FROM report_review_asks`); n != 0 {
			t.Errorf("%d asks stored by refused calls, want 0", n)
		}
	})

	t.Run("one open at a time, three in total, the count carrying over a Restore", func(t *testing.T) {
		id := r.removed(service.RemovalFalse)
		box, err := r.ask(id, "  I took the photo myself.  ")
		if err != nil || box.State != service.ReviewAskAsked || box.Why != "Why: False" {
			t.Fatalf("first Ask = %+v, %v", box, err)
		}
		if got := r.str(`SELECT note FROM report_review_asks WHERE report_id = $1`, id); got != "I took the photo myself." {
			t.Errorf("stored note %q, want it trimmed", got)
		}
		if _, err := r.ask(id, ""); !errors.Is(err, service.ErrReviewAskOpen) {
			t.Errorf("an ask while one is open: %v", err)
		}
		r.mustKeepRemoved(id)
		if box := r.state3(id); box.State != service.ReviewAskCanAsk {
			t.Errorf("after one answered ask: %+v", box)
		}
		r.mustAsk(id)
		if _, err := r.mods.Restore(ctx, id, r.mod); err != nil {
			t.Fatalf("Restore: %v", err)
		}
		if _, err := r.mods.Remove(ctx, id, r.mod, service.RemovalOther); err != nil {
			t.Fatalf("Remove again: %v", err)
		}
		if box := r.state3(id); box.State != service.ReviewAskCanAsk || box.Why != "Why: Other" {
			t.Errorf("after Restore and a second Remove: %+v", box)
		}
		r.mustAsk(id)
		if box := r.state3(id); box.State != service.ReviewAskAsked {
			t.Errorf("the third ask is open, the box says %+v", box)
		}
		r.mustKeepRemoved(id)
		if box := r.state3(id); box.State != service.ReviewAskNoneLeft {
			t.Errorf("after three asks: %+v", box)
		}
		if _, err := r.ask(id, ""); !errors.Is(err, service.ErrReviewAskNoneLeft) {
			t.Errorf("a fourth ask: %v", err)
		}
		if n := r.count(`SELECT count(*) FROM report_review_asks WHERE report_id = $1`, id); n != service.ReviewAskLimit {
			t.Errorf("%d asks stored, want %d", n, service.ReviewAskLimit)
		}
	})

	t.Run("the note is trimmed, capped at 200 and empty is none", func(t *testing.T) {
		id := r.removed(service.RemovalSpam)
		var ve service.ValidationError
		if _, err := r.ask(id, strings.Repeat("a", 201)); !errors.As(err, &ve) || ve.Field != "note" || ve.Message != service.ReviewAskNoteTooLongMessage {
			t.Errorf("a 201 character note: %v", err)
		}
		if n := r.count(`SELECT count(*) FROM report_review_asks WHERE report_id = $1`, id); n != 0 {
			t.Errorf("a refused note stored %d asks", n)
		}
		if _, err := r.ask(id, "   "); err != nil {
			t.Fatalf("a blank note: %v", err)
		}
		if got := r.str(`SELECT note FROM report_review_asks WHERE report_id = $1`, id); got != "<null>" {
			t.Errorf("a blank note is stored as %q, want none", got)
		}
		other := r.removed(service.RemovalSpam)
		if _, err := r.ask(other, " "+strings.Repeat("é", 200)+" "); err != nil {
			t.Errorf("200 characters after trimming: %v", err)
		}
	})

	t.Run("a deleted author account cannot ask", func(t *testing.T) {
		id := r.removed(service.RemovalSpam)
		testutil.MustExec(t, r.pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, r.poster)
		defer testutil.MustExec(t, r.pool, `UPDATE accounts SET deleted_at = NULL WHERE id = $1`, r.poster)
		if _, err := r.ask(id, ""); !errors.Is(err, service.ErrReportNotFound) {
			t.Errorf("a deleted author's ask: %v", err)
		}
	})
}

// TestReviewAskConcurrent fires many asks at once on one removed report: the
// report row lock makes them queue, so exactly one is stored and the rest see
// it open; then, with two asks already answered, a burst still stores one,
// the third, and never a fourth.
func TestReviewAskConcurrent(t *testing.T) {
	r := newRzRig(t)
	burst := func(id int64) (ok, open, none int) {
		t.Helper()
		const n = 12
		var wg sync.WaitGroup
		var mu sync.Mutex
		start := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, err := r.ask(id, "")
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					ok++
				case errors.Is(err, service.ErrReviewAskOpen):
					open++
				case errors.Is(err, service.ErrReviewAskNoneLeft):
					none++
				default:
					t.Errorf("an ask failed: %v", err)
				}
			}()
		}
		close(start)
		wg.Wait()
		return
	}

	id := r.removed(service.RemovalFalse)
	if ok, open, _ := burst(id); ok != 1 || open != 11 {
		t.Errorf("first burst: %d stored, %d open, want 1 and 11", ok, open)
	}
	r.mustKeepRemoved(id)
	if ok, _, _ := burst(id); ok != 1 {
		t.Errorf("second burst: %d stored, want 1", ok)
	}
	r.mustKeepRemoved(id)
	if ok, _, _ := burst(id); ok != 1 {
		t.Errorf("third burst: %d stored, want 1", ok)
	}
	r.mustKeepRemoved(id)
	if ok, open, none := burst(id); ok != 0 || open != 0 || none != 12 {
		t.Errorf("after three: %d stored, %d open, %d none left, want 0, 0 and 12", ok, open, none)
	}
	if n := r.count(`SELECT count(*) FROM report_review_asks WHERE report_id = $1`, id); n != 3 {
		t.Errorf("%d asks stored, want 3", n)
	}
}

func TestRestoreAndKeepRemoved(t *testing.T) {
	r := newRzRig(t)
	ctx := context.Background()

	t.Run("Restore with an open ask shows it again, closes the ask and tells the author once", func(t *testing.T) {
		id := r.removed(service.RemovalFalse)
		other := r.flagger()
		testutil.MustExec(t, r.pool, `INSERT INTO report_flags (report_id, account_id, reasons) VALUES ($1, $2, ARRAY['spam'])`, id, other)
		r.mustAsk(id)
		row, err := r.mods.Restore(ctx, id, r.mod)
		if err != nil {
			t.Fatalf("Restore: %v", err)
		}
		if row.Moderation != service.ModerationClear || row.RemovedReason != "" {
			t.Errorf("the answer row is %s with reason %q", row.Moderation, row.RemovedReason)
		}
		if r.state(id) != "clear" || r.str(`SELECT removed_reason FROM reports WHERE id = $1`, id) != "<null>" {
			t.Errorf("state %q reason %q, want clear and none", r.state(id), r.str(`SELECT removed_reason FROM reports WHERE id = $1`, id))
		}
		if n := r.count(`SELECT count(*) FROM reports WHERE id = $1 AND removed_at IS NULL AND review_kept_at IS NOT NULL`, id); n != 1 {
			t.Error("Restore must clear the removal time and stamp review_kept_at")
		}
		if n := r.count(`SELECT count(*) FROM report_flags WHERE report_id = $1 AND dismissed_at IS NULL`, id); n != 0 {
			t.Errorf("%d open flags left", n)
		}
		if n := r.count(`SELECT count(*) FROM report_review_asks WHERE report_id = $1 AND answer = 'restored' AND answered_by_account_id = $2`, id, r.mod); n != 1 {
			t.Errorf("the ask is not closed as restored")
		}
		if n := r.count(`SELECT count(*) FROM moderation_actions WHERE report_id = $1 AND action = 'restore' AND reason IS NULL`, id); n != 1 {
			t.Errorf("%d restore audit rows, want 1", n)
		}
		if n := r.count(`SELECT count(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'own_restored' AND read_at IS NULL`, r.poster, id); n != 1 {
			t.Errorf("%d unread own_restored rows, want 1", n)
		}
		before := r.writes(id)
		if _, err := r.mods.Restore(ctx, id, r.mod); !errors.Is(err, service.ErrAlreadyHandled) {
			t.Errorf("a repeated Restore: %v", err)
		}
		if _, err := r.mods.KeepRemoved(ctx, id, r.mod); !errors.Is(err, service.ErrAlreadyHandled) {
			t.Errorf("Keep removed of a shown report: %v", err)
		}
		if after := r.writes(id); after != before {
			t.Errorf("a refused answer wrote: %s, then %s", before, after)
		}
		if _, err := r.asks.State(ctx, r.poster, id); !errors.Is(err, service.ErrReportNotFound) {
			t.Errorf("the box of a restored report: %v", err)
		}
	})

	t.Run("Restore from the Removed list needs no ask", func(t *testing.T) {
		id := r.removed(service.RemovalSpam)
		if _, err := r.mods.Restore(ctx, id, r.mod); err != nil {
			t.Fatalf("Restore: %v", err)
		}
		if r.state(id) != "clear" || r.own(id, service.UpdateOwnRestored) != 1 {
			t.Errorf("state %q, %d own_restored rows", r.state(id), r.own(id, service.UpdateOwnRestored))
		}
	})

	t.Run("Keep removed needs an open ask and keeps the report removed with its reason", func(t *testing.T) {
		id := r.removed(service.RemovalAbusive)
		before := r.writes(id)
		if _, err := r.mods.KeepRemoved(ctx, id, r.mod); !errors.Is(err, service.ErrAlreadyHandled) {
			t.Errorf("Keep removed with no ask: %v", err)
		}
		if after := r.writes(id); after != before {
			t.Errorf("a refused Keep removed wrote: %s, then %s", before, after)
		}
		r.mustAsk(id)
		r.mustKeepRemoved(id)
		if r.state(id) != "removed" || r.str(`SELECT removed_reason FROM reports WHERE id = $1`, id) != "abusive" {
			t.Errorf("state %q reason %q, want removed and abusive", r.state(id), r.str(`SELECT removed_reason FROM reports WHERE id = $1`, id))
		}
		if n := r.count(`SELECT count(*) FROM report_review_asks WHERE report_id = $1 AND answer = 'kept_removed'`, id); n != 1 {
			t.Error("the ask is not closed as kept_removed")
		}
		if n := r.count(`SELECT count(*) FROM moderation_actions WHERE report_id = $1 AND action = 'keep_removed'`, id); n != 1 {
			t.Errorf("%d keep_removed audit rows, want 1", n)
		}
		if r.own(id, service.UpdateOwnStaysRemoved) != 1 {
			t.Error("no own_stays_removed row")
		}
		before = r.writes(id)
		if _, err := r.mods.KeepRemoved(ctx, id, r.mod); !errors.Is(err, service.ErrAlreadyHandled) {
			t.Errorf("a repeated Keep removed: %v", err)
		}
		if after := r.writes(id); after != before {
			t.Errorf("a repeated Keep removed wrote: %s, then %s", before, after)
		}
		// A second Keep removed answer, after a second ask, brings the one row
		// back unread instead of adding one.
		testutil.MustExec(t, r.pool, `UPDATE updates SET read_at = now(), created_at = now() - interval '1 hour' WHERE report_id = $1`, id)
		r.mustAsk(id)
		r.mustKeepRemoved(id)
		if n := r.count(`SELECT count(*) FROM updates WHERE report_id = $1 AND kind = 'own_stays_removed'`, id); n != 1 {
			t.Errorf("%d own_stays_removed rows, want 1", n)
		}
		if n := r.count(`SELECT count(*) FROM updates WHERE report_id = $1 AND kind = 'own_stays_removed' AND read_at IS NULL AND created_at > now() - interval '1 minute'`, id); n != 1 {
			t.Error("the stays removed row did not come back unread and newest")
		}
	})

	t.Run("an unknown report is not found", func(t *testing.T) {
		if _, err := r.mods.Restore(ctx, 999999, r.mod); !errors.Is(err, service.ErrReportNotFound) {
			t.Errorf("Restore: %v", err)
		}
		if _, err := r.mods.KeepRemoved(ctx, 999999, r.mod); !errors.Is(err, service.ErrReportNotFound) {
			t.Errorf("KeepRemoved: %v", err)
		}
	})

	t.Run("a restored report can never be hidden by flags again", func(t *testing.T) {
		id := r.removed(service.RemovalFalse)
		if _, err := r.mods.Restore(ctx, id, r.mod); err != nil {
			t.Fatalf("Restore: %v", err)
		}
		for k := 1; k <= service.FlagHideThreshold+1; k++ {
			r.mustFlag(id, frAt(k))
		}
		if got := r.state(id); got != "clear" {
			t.Errorf("flags from %d places moved a restored report to %q", service.FlagHideThreshold+1, got)
		}
	})

	t.Run("Remove after Restore brings own_removed back unread and newest, one row", func(t *testing.T) {
		id := r.removed(service.RemovalFalse)
		if _, err := r.mods.Restore(ctx, id, r.mod); err != nil {
			t.Fatalf("Restore: %v", err)
		}
		testutil.MustExec(t, r.pool, `UPDATE updates SET read_at = now(), created_at = now() - interval '2 hours' WHERE account_id = $1`, r.poster)
		if _, err := r.mods.Remove(ctx, id, r.mod, service.RemovalDangerous); err != nil {
			t.Fatalf("Remove again: %v", err)
		}
		if n := r.own(id, service.UpdateOwnRemoved); n != 1 {
			t.Errorf("%d own_removed rows, want 1", n)
		}
		var kind string
		var unread bool
		if err := r.pool.QueryRow(ctx, `SELECT kind, read_at IS NULL FROM updates WHERE account_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, r.poster).Scan(&kind, &unread); err != nil {
			t.Fatalf("reading the newest row: %v", err)
		}
		if kind != "own_removed" || !unread {
			t.Errorf("the newest row is %s unread %v, want own_removed unread", kind, unread)
		}
		// The bell's read mark is the largest id shown; the row kept its old id
		// and is still marked read by it.
		var maxID int64
		if err := r.pool.QueryRow(ctx, `SELECT max(id) FROM updates WHERE account_id = $1`, r.poster).Scan(&maxID); err != nil {
			t.Fatalf("reading the largest id: %v", err)
		}
		if _, err := r.q.MarkUpdatesRead(ctx, sqlcgen.MarkUpdatesReadParams{AccountID: r.poster, UpTo: maxID, Now: time.Now()}); err != nil {
			t.Fatalf("MarkUpdatesRead: %v", err)
		}
		if n := r.count(`SELECT count(*) FROM updates WHERE account_id = $1 AND read_at IS NULL`, r.poster); n != 0 {
			t.Errorf("%d unread rows after the read mark, want 0", n)
		}
	})

	t.Run("the new rows are never queued for a phone", func(t *testing.T) {
		if n := r.count(`SELECT count(*) FROM push_outbox`); n != 0 {
			t.Errorf("%d phone rows queued, want 0", n)
		}
	})
}

// TestRestoreUnderTheRowLock races two Restores and a Restore against a Keep
// removed on one report: exactly one answer wins, the other is already
// handled, and exactly one audit row and one bell row are written.
func TestRestoreUnderTheRowLock(t *testing.T) {
	r := newRzRig(t)
	ctx := context.Background()
	for round := 0; round < 5; round++ {
		id := r.removed(service.RemovalFalse)
		r.mustAsk(id)
		var wg sync.WaitGroup
		errs := make([]error, 2)
		start := make(chan struct{})
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				if i == 0 {
					_, errs[i] = r.mods.Restore(ctx, id, r.mod)
				} else {
					_, errs[i] = r.mods.KeepRemoved(ctx, id, r.mod)
				}
			}(i)
		}
		close(start)
		wg.Wait()
		// Restore always wins: it either runs first, and Keep removed then finds
		// a shown report, or runs after Keep removed closed the ask and restores
		// the still removed report. Either way the ask is answered once.
		if errs[0] != nil {
			t.Fatalf("round %d: Restore: %v", round, errs[0])
		}
		won := 1
		if errs[1] == nil {
			won++
		} else if !errors.Is(errs[1], service.ErrAlreadyHandled) {
			t.Fatalf("round %d: KeepRemoved: %v", round, errs[1])
		}
		if n := r.count(`SELECT count(*) FROM report_review_asks WHERE report_id = $1 AND answered_at IS NOT NULL`, id); n != 1 {
			t.Errorf("round %d: the ask was answered %d times", round, n)
		}
		if n := r.count(`SELECT count(*) FROM updates WHERE report_id = $1 AND kind IN ('own_restored', 'own_stays_removed')`, id); n != won {
			t.Errorf("round %d: %d bell rows for %d winning answers", round, n, won)
		}
	}
	for round := 0; round < 5; round++ {
		id := r.removed(service.RemovalFalse)
		var wg sync.WaitGroup
		errs := make([]error, 4)
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, errs[i] = r.mods.Restore(ctx, id, r.mod)
			}(i)
		}
		wg.Wait()
		won := 0
		for _, err := range errs {
			if err == nil {
				won++
			} else if !errors.Is(err, service.ErrAlreadyHandled) {
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if won != 1 {
			t.Errorf("round %d: %d Restores won, want 1", round, won)
		}
		if n := r.count(`SELECT count(*) FROM moderation_actions WHERE report_id = $1 AND action = 'restore'`, id); n != 1 {
			t.Errorf("round %d: %d restore audit rows, want 1", round, n)
		}
		if n := r.own(id, service.UpdateOwnRestored); n != 1 {
			t.Errorf("round %d: %d own_restored rows, want 1", round, n)
		}
	}
}

// TestRestoredReportCountsInTheRating: a removal makes a Past report neutral
// for its author's rating, as it always has, and a Restore makes it count by
// its votes again, like any shown report; asks change nothing.
func TestRestoredReportCountsInTheRating(t *testing.T) {
	r := newRzRig(t)
	ctx := context.Background()
	now := time.Now().UTC()
	var ids []int64
	for i := 0; i < 5; i++ {
		id := testutil.InsertStoredReport(t, r.pool, testutil.StoredReport{
			SessionID:       frPosterSess,
			Category:        string(service.CategoryFlood),
			Severity:        string(service.SeverityLow),
			Description:     "Water over the road by the old bridge",
			Latitude:        frLat,
			Longitude:       frLon,
			CreatedAt:       now.Add(-time.Duration(30+i) * time.Hour),
			ExpiresAt:       now.Add(-time.Duration(24+i) * time.Hour),
			ModerationState: string(service.ModerationClear),
		})
		for k := 0; k < service.IndependentAgreementThreshold; k++ {
			if _, err := r.pool.Exec(ctx,
				`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at) VALUES ($1, $2, 'content', 'confirm', $3, $4)`,
				id, r.flagger(), fmt.Sprintf("tdr1v%d%d", i, k), now.Add(-time.Duration(28+i)*time.Hour)); err != nil {
				t.Fatalf("vote on report %d: %v", id, err)
			}
		}
		ids = append(ids, id)
	}
	signal := func() service.ReporterSignal {
		t.Helper()
		got, err := service.LoadReporterSignals(ctx, r.q, nil, []int64{r.poster}, now)
		if err != nil {
			t.Fatalf("LoadReporterSignals: %v", err)
		}
		return got[r.poster]
	}
	before := signal()
	if before.JudgedReports != 5 || before.CorroboratedReports != 5 {
		t.Fatalf("setup: judged %d corroborated %d, want 5 and 5", before.JudgedReports, before.CorroboratedReports)
	}
	if _, err := r.mods.Remove(ctx, ids[0], r.mod, service.RemovalFalse); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := signal(); got.JudgedReports != 4 {
		t.Errorf("after Remove: judged %d, want 4, a removed report is neutral", got.JudgedReports)
	}
	r.mustAsk(ids[0])
	if got := signal(); got.JudgedReports != 4 {
		t.Errorf("an ask changed the rating: judged %d", got.JudgedReports)
	}
	if _, err := r.mods.Restore(ctx, ids[0], r.mod); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := signal(); got.JudgedReports != before.JudgedReports || got.CorroboratedReports != before.CorroboratedReports || got.Tag != before.Tag {
		t.Errorf("after Restore: judged %d corroborated %d tag %q, want %d, %d, %q",
			got.JudgedReports, got.CorroboratedReports, got.Tag, before.JudgedReports, before.CorroboratedReports, before.Tag)
	}
}

// TestUpdatesListBoxRows reads the author's updates through the real
// UpdatesService: the removed row opens the box only while the report is
// removed, the back row opens the report, and no row carries the report's
// words.
func TestUpdatesListBoxRows(t *testing.T) {
	r := newRzRig(t)
	ctx := context.Background()
	upd := service.NewUpdatesService(r.q, nil)
	id := r.removed(service.RemovalFalse)
	r.mustAsk(id)
	r.mustKeepRemoved(id)
	page, err := upd.List(ctx, r.poster, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byKind := map[service.UpdateKind]service.UpdateItem{}
	for _, it := range page.Items {
		byKind[it.Kind] = it
	}
	for _, k := range []service.UpdateKind{service.UpdateOwnRemoved, service.UpdateOwnStaysRemoved} {
		it, ok := byKind[k]
		if !ok {
			t.Fatalf("no %s row", k)
		}
		if it.ReviewReportID == nil || *it.ReviewReportID != id || it.ReportID != nil || !it.Available || it.ReportLine != "" {
			t.Errorf("%s row = %+v, want the box of report %d and nothing else", k, it, id)
		}
	}
	if byKind[service.UpdateOwnStaysRemoved].Event != "Your report stays removed." {
		t.Errorf("stays removed line %q", byKind[service.UpdateOwnStaysRemoved].Event)
	}
	if _, err := r.mods.Restore(ctx, id, r.mod); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	page, err = upd.List(ctx, r.poster, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Items[0].Kind != service.UpdateOwnRestored || page.Items[0].Event != "Your report is back." ||
		page.Items[0].ReportID == nil || *page.Items[0].ReportID != id {
		t.Errorf("the newest row = %+v, want the back row opening report %d", page.Items[0], id)
	}
	for _, it := range page.Items {
		if it.ReviewReportID != nil {
			t.Errorf("%s still opens the box of a restored report", it.Kind)
		}
	}
}
