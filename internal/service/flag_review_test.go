package service_test

// flag_review_test.go proves quick 261117-vvb on a real Postgres: a shown
// report is hidden for review once its open flags come from FlagHideThreshold
// different place cells, exactly once even when flags race, never for a held,
// removed or kept report, and the text check worker never touches it. It also
// proves the author's bell rows: one per event, written by the flag, by Keep
// and Remove and by the late text check. Every test starts with
// testutil.NewTestDB, so it skips in the quick run.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	frLat, frLon  = 12.9716, 77.5946
	frPosterEmail = "vvb-poster@example.com"
	frPosterSess  = "vvb-poster-session"
)

// frRig is one test's world: the poster, a moderator, the real flag and
// moderation services over the real transactions, and a counter for new
// flagger accounts.
type frRig struct {
	t      *testing.T
	pool   *pgxpool.Pool
	q      *sqlcgen.Queries
	poster int64
	mod    int64
	flags  *service.FlagService
	mods   *service.ModerationService
	seq    int
}

func newFrRig(t *testing.T) *frRig {
	t.Helper()
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	r := &frRig{t: t, pool: pool, q: q}
	r.poster = testutil.SeedReporterAccount(t, pool, frPosterEmail, frPosterSess)
	r.mod = testutil.SeedReporterAccount(t, pool, "vvb-moderator@example.com")
	r.flags = service.NewFlagService(q, service.WithFlagTx(service.PgxFlagTx(pool)))
	r.mods = service.NewModerationService(q, service.WithModerationTx(service.PgxModerationTx(pool)))
	return r
}

// report stores one report by the poster in the given state.
func (r *frRig) report(sev service.Severity, state service.ModerationState) int64 {
	r.t.Helper()
	now := time.Now().UTC()
	return testutil.InsertStoredReport(r.t, r.pool, testutil.StoredReport{
		SessionID:       frPosterSess,
		Category:        string(service.CategoryFlood),
		Severity:        string(sev),
		Description:     "Water over the road by the old bridge",
		Latitude:        frLat,
		Longitude:       frLon,
		CreatedAt:       now.Add(-time.Minute),
		ExpiresAt:       now.Add(6 * time.Hour),
		ModerationState: string(state),
	})
}

// flagger makes one new account that can flag.
func (r *frRig) flagger() int64 {
	r.t.Helper()
	r.seq++
	return testutil.SeedReporterAccount(r.t, r.pool, fmt.Sprintf("vvb-flagger-%d@example.com", r.seq))
}

// at is a position k steps of about 330 metres north of the report point, so
// every k is a different place cell at the voter precision and the same k is
// the same cell.
func frAt(k int) *service.FlagPosition {
	return &service.FlagPosition{Latitude: frLat + float64(k)*0.003, Longitude: frLon}
}

func (r *frRig) flag(reportID, account int64, pos *service.FlagPosition) error {
	return r.flags.Flag(context.Background(), service.FlagInput{
		ReportID: reportID, AccountID: account, Reasons: []service.FlagReason{service.FlagFalse}, Position: pos,
	})
}

func (r *frRig) mustFlag(reportID int64, pos *service.FlagPosition) {
	r.t.Helper()
	if err := r.flag(reportID, r.flagger(), pos); err != nil {
		r.t.Fatalf("flag on report %d: %v", reportID, err)
	}
}

func (r *frRig) state(id int64) string {
	r.t.Helper()
	var s string
	if err := r.pool.QueryRow(context.Background(), `SELECT moderation_state FROM reports WHERE id = $1`, id).Scan(&s); err != nil {
		r.t.Fatalf("reading report %d: %v", id, err)
	}
	return s
}

func (r *frRig) count(sql string, args ...any) int {
	r.t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		r.t.Fatalf("counting with %q: %v", sql, err)
	}
	return n
}

// own is how many bell rows of kind the poster has about the report.
func (r *frRig) own(reportID int64, kind service.UpdateKind) int {
	r.t.Helper()
	return r.count(`SELECT count(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = $3`, r.poster, reportID, string(kind))
}

// ownAll is how many of the four own rows the poster has about the report.
func (r *frRig) ownAll(reportID int64) int {
	r.t.Helper()
	return r.count(`SELECT count(*) FROM updates WHERE report_id = $1 AND kind IN ('own_shown', 'own_review', 'own_kept', 'own_removed')`, reportID)
}

func TestFlagReviewHide(t *testing.T) {
	r := newFrRig(t)
	ctx := context.Background()

	t.Run("flags from two places do not hide", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		r.mustFlag(id, frAt(1))
		r.mustFlag(id, frAt(2))
		if got := r.state(id); got != "clear" {
			t.Errorf("state = %q, want clear", got)
		}
		if n := r.ownAll(id); n != 0 {
			t.Errorf("%d bell rows, want 0", n)
		}
	})

	t.Run("three accounts in one place do not hide", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		for i := 0; i < 3; i++ {
			r.mustFlag(id, frAt(1))
		}
		if got := r.state(id); got != "clear" {
			t.Errorf("state = %q, want clear", got)
		}
	})

	t.Run("three accounts in three places hide once and tell the author once", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		r.mustFlag(id, frAt(1))
		r.mustFlag(id, frAt(2))
		r.mustFlag(id, frAt(3))
		if got := r.state(id); got != "review" {
			t.Fatalf("state = %q, want review", got)
		}
		if n := r.own(id, service.UpdateOwnReview); n != 1 {
			t.Errorf("own_review rows = %d, want 1", n)
		}
		// A hidden report is withheld: a later flag reads as not found and
		// writes nothing.
		if err := r.flag(id, r.flagger(), frAt(4)); !errors.Is(err, service.ErrReportNotFound) {
			t.Errorf("a flag on a hidden report: %v, want ErrReportNotFound", err)
		}
		if n := r.count(`SELECT count(*) FROM report_flags WHERE report_id = $1`, id); n != 3 {
			t.Errorf("%d flags stored, want 3", n)
		}
		if n := r.ownAll(id); n != 1 {
			t.Errorf("%d bell rows, want 1", n)
		}
	})

	t.Run("a hidden critical report is withheld too", func(t *testing.T) {
		id := r.report(service.SeverityCritical, service.ModerationClear)
		r.mustFlag(id, frAt(1))
		r.mustFlag(id, frAt(2))
		r.mustFlag(id, frAt(3))
		if got := r.state(id); got != "review" {
			t.Fatalf("state = %q, want review", got)
		}
		meta := service.ReportMeta{Severity: service.SeverityCritical, Category: service.CategoryFlood, Moderation: service.ModerationReview}
		if !meta.Withheld() {
			t.Error("a critical report hidden for review is not withheld")
		}
	})

	t.Run("flags with no place never count", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		for i := 0; i < 4; i++ {
			r.mustFlag(id, nil)
		}
		r.mustFlag(id, frAt(1))
		r.mustFlag(id, frAt(2))
		if got := r.state(id); got != "clear" {
			t.Fatalf("state = %q after 4 placeless flags and 2 places, want clear", got)
		}
		if n := r.count(`SELECT count(*) FROM report_flags WHERE report_id = $1 AND flagger_cell IS NULL`, id); n != 4 {
			t.Errorf("%d placeless flags stored, want 4: a flag with no place still reaches the moderator", n)
		}
		r.mustFlag(id, frAt(3))
		if got := r.state(id); got != "review" {
			t.Errorf("state = %q after the third place, want review", got)
		}
	})

	t.Run("a dismissed flag does not count", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		r.mustFlag(id, frAt(1))
		r.mustFlag(id, frAt(2))
		testutil.MustExec(t, r.pool, `UPDATE report_flags SET dismissed_at = now() WHERE report_id = $1 AND flagger_cell = (
			SELECT flagger_cell FROM report_flags WHERE report_id = $1 ORDER BY id LIMIT 1)`, id)
		r.mustFlag(id, frAt(3))
		if got := r.state(id); got != "clear" {
			t.Errorf("state = %q with two open places, want clear", got)
		}
	})

	t.Run("only the cell is stored, at the voter precision", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		r.mustFlag(id, frAt(1))
		if n := r.count(`SELECT count(*) FROM report_flags WHERE report_id = $1 AND char_length(flagger_cell) = 7`, id); n != 1 {
			t.Errorf("%d flags with a 7 character cell, want 1", n)
		}
	})

	t.Run("a held, a pending serious and a removed report never become review", func(t *testing.T) {
		held := r.report(service.SeverityLow, service.ModerationPending)
		serious := r.report(service.SeverityCritical, service.ModerationPending)
		removed := r.report(service.SeverityLow, service.ModerationRemoved)
		for _, id := range []int64{held, removed} {
			if err := r.flag(id, r.flagger(), frAt(1)); !errors.Is(err, service.ErrReportNotFound) {
				t.Errorf("flag on withheld report %d: %v, want ErrReportNotFound", id, err)
			}
		}
		// A pending serious report is published, so it can be flagged, and it
		// stays pending.
		r.mustFlag(serious, frAt(1))
		r.mustFlag(serious, frAt(2))
		r.mustFlag(serious, frAt(3))
		if got := r.state(serious); got != "pending" {
			t.Errorf("pending serious report state = %q, want pending", got)
		}
		// The statement itself refuses every state but clear, whatever the flags.
		for _, id := range []int64{held, removed} {
			for k := 1; k <= 3; k++ {
				testutil.MustExec(t, r.pool, `INSERT INTO report_flags (report_id, account_id, reasons, flagger_cell) VALUES ($1, $2, ARRAY['false'], $3)`,
					id, r.flagger(), fmt.Sprintf("tdr1v8%d", k))
			}
		}
		for _, id := range []int64{held, serious, removed} {
			n, err := r.q.HideReportForReview(ctx, sqlcgen.HideReportForReviewParams{ReportID: id, Threshold: service.FlagHideThreshold})
			if err != nil || n != 0 {
				t.Errorf("HideReportForReview(%d) = %d, %v; want 0 rows", id, n, err)
			}
		}
		if got := r.state(held); got != "pending" {
			t.Errorf("held report state = %q, want pending", got)
		}
		if got := r.state(removed); got != "removed" {
			t.Errorf("removed report state = %q, want removed", got)
		}
	})

	t.Run("after Keep no later flag hides it, and later flags still reach the moderator", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		r.mustFlag(id, frAt(1))
		r.mustFlag(id, frAt(2))
		r.mustFlag(id, frAt(3))
		if got := r.state(id); got != "review" {
			t.Fatalf("state = %q, want review", got)
		}
		row, err := r.mods.Keep(ctx, id, r.mod)
		if err != nil {
			t.Fatalf("Keep: %v", err)
		}
		if row.Moderation != service.ModerationClear {
			t.Errorf("kept row moderation = %q, want clear", row.Moderation)
		}
		if n := r.count(`SELECT count(*) FROM reports WHERE id = $1 AND review_kept_at IS NOT NULL`, id); n != 1 {
			t.Error("Keep did not mark the report kept")
		}
		if n := r.own(id, service.UpdateOwnKept); n != 1 {
			t.Errorf("own_kept rows = %d, want 1", n)
		}
		if n := r.count(`SELECT count(*) FROM report_flags WHERE report_id = $1 AND dismissed_at IS NULL`, id); n != 0 {
			t.Errorf("%d open flags after Keep, want 0", n)
		}
		for k := 4; k <= 7; k++ {
			r.mustFlag(id, frAt(k))
		}
		if got := r.state(id); got != "clear" {
			t.Errorf("state = %q after four new places, want clear", got)
		}
		if n := r.own(id, service.UpdateOwnReview); n != 1 {
			t.Errorf("own_review rows = %d, want still 1", n)
		}
		queues, err := r.mods.Queues(ctx)
		if err != nil {
			t.Fatalf("Queues: %v", err)
		}
		found := false
		for _, fr := range queues.Flagged {
			if fr.ID == id && fr.FlagTotal == 4 {
				found = true
			}
		}
		if !found {
			t.Error("the kept report with four new flags is not on the flagged list")
		}
		// A second Keep finds it in the flagged list, keeps it again and tells
		// the author nothing more.
		if _, err := r.mods.Keep(ctx, id, r.mod); err != nil {
			t.Fatalf("second Keep: %v", err)
		}
		if n := r.ownAll(id); n != 2 {
			t.Errorf("%d bell rows after the second Keep, want 2 (review and kept)", n)
		}
	})

	t.Run("Remove from review removes it and tells the author once", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		r.mustFlag(id, frAt(1))
		r.mustFlag(id, frAt(2))
		r.mustFlag(id, frAt(3))
		if _, err := r.mods.Remove(ctx, id, r.mod, service.RemovalSpam); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if got := r.state(id); got != "removed" {
			t.Errorf("state = %q, want removed", got)
		}
		if n := r.own(id, service.UpdateOwnRemoved); n != 1 {
			t.Errorf("own_removed rows = %d, want 1", n)
		}
		if _, err := r.mods.Remove(ctx, id, r.mod, service.RemovalSpam); !errors.Is(err, service.ErrAlreadyHandled) {
			t.Errorf("second Remove: %v, want ErrAlreadyHandled", err)
		}
		if n := r.own(id, service.UpdateOwnRemoved); n != 1 {
			t.Errorf("own_removed rows = %d after a second Remove, want 1", n)
		}
	})

	t.Run("the moderator list puts a hidden report first", func(t *testing.T) {
		busy := r.report(service.SeverityLow, service.ModerationClear)
		for k := 1; k <= 2; k++ {
			r.mustFlag(busy, frAt(1))
		}
		hidden := r.report(service.SeverityLow, service.ModerationClear)
		r.mustFlag(hidden, frAt(1))
		r.mustFlag(hidden, frAt(2))
		r.mustFlag(hidden, frAt(3))
		testutil.MustExec(t, r.pool, `INSERT INTO report_flags (report_id, account_id, reasons) SELECT $1, id, ARRAY['spam'] FROM accounts WHERE email LIKE 'vvb-flagger-%' AND id NOT IN (SELECT account_id FROM report_flags WHERE report_id = $1)`, busy)
		queues, err := r.mods.Queues(ctx)
		if err != nil {
			t.Fatalf("Queues: %v", err)
		}
		if len(queues.Flagged) == 0 || queues.Flagged[0].Moderation != service.ModerationReview {
			t.Fatalf("the first flagged row is not hidden for review: %+v", queues.Flagged)
		}
		for _, row := range queues.Flagged {
			if row.Moderation == service.ModerationReview && (row.View != service.ModViewNone || row.VisibilityWord != "") {
				t.Errorf("hidden report %d has view %q word %q, want none", row.ID, row.View, row.VisibilityWord)
			}
		}
	})
}

// TestFlagReviewConcurrent sends flags from different places at the same
// instant. The report row lock makes each flag see the ones committed before
// it, so the hide happens exactly once and is never skipped.
func TestFlagReviewConcurrent(t *testing.T) {
	r := newFrRig(t)

	run := func(id int64, places []int) {
		t.Helper()
		accounts := make([]int64, len(places))
		for i := range places {
			accounts[i] = r.flagger()
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, len(places))
		for i := range places {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				errs[i] = r.flag(id, accounts[i], frAt(places[i]))
			}(i)
		}
		close(start)
		wg.Wait()
		for i, err := range errs {
			if err != nil && !errors.Is(err, service.ErrReportNotFound) {
				t.Errorf("flag %d: %v", i, err)
			}
		}
	}

	for round := 0; round < 5; round++ {
		t.Run(fmt.Sprintf("six places at once, round %d", round), func(t *testing.T) {
			id := r.report(service.SeverityLow, service.ModerationClear)
			run(id, []int{1, 2, 3, 4, 5, 6})
			if got := r.state(id); got != "review" {
				t.Errorf("state = %q, want review", got)
			}
			if n := r.own(id, service.UpdateOwnReview); n != 1 {
				t.Errorf("own_review rows = %d, want exactly 1", n)
			}
		})
		t.Run(fmt.Sprintf("the second and third place at once never skip, round %d", round), func(t *testing.T) {
			id := r.report(service.SeverityLow, service.ModerationClear)
			r.mustFlag(id, frAt(1))
			run(id, []int{2, 3})
			if got := r.state(id); got != "review" {
				t.Errorf("state = %q, want review", got)
			}
			if n := r.own(id, service.UpdateOwnReview); n != 1 {
				t.Errorf("own_review rows = %d, want exactly 1", n)
			}
		})
	}
}

// TestReviewUntouchedByTextCheck proves the text check worker never claims a
// report hidden for review and that its verdict write never moves one.
func TestReviewUntouchedByTextCheck(t *testing.T) {
	r := newFrRig(t)
	ctx := context.Background()
	now := time.Now().UTC()

	id := insertPendingReport(t, r.pool, r.q, service.SeverityLow, service.CategoryFlood, mwDesc, now.Add(-time.Minute), now.Add(6*time.Hour))
	testutil.MustExec(t, r.pool, `UPDATE reports SET moderation_state = 'review' WHERE id = $1`, id)

	fs := screenerWith(mwPassScores)
	if err := mwDBWorker(r.q, fs, now).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if fs.calls != 0 {
		t.Errorf("the worker screened a hidden report %d times, want 0", fs.calls)
	}
	if c := mwCheckOf(t, r.pool, id); c.Attempts != 0 || c.Status != "pending" {
		t.Errorf("check attempts %d status %q, want 0 and pending", c.Attempts, c.Status)
	}
	if err := r.q.FinishCheckAndSetState(ctx, sqlcgen.FinishCheckAndSetStateParams{
		ModerationState: string(service.ModerationClear), Verdict: "pass", ReportID: id,
		OwnKind: func() *string { s := string(service.UpdateOwnShown); return &s }(),
	}); err != nil {
		t.Fatalf("FinishCheckAndSetState: %v", err)
	}
	if got := r.state(id); got != "review" {
		t.Errorf("state = %q after a pass write, want review", got)
	}
	if n := r.ownAll(id); n != 0 {
		t.Errorf("%d bell rows, want 0", n)
	}
}

// TestReviewRuleIsStateBased: a pending serious report that three places
// flagged stays pending, and once the late check passes it, the next flag, even
// one with no place, hides it, because the rule reads the open flags and not
// the flag that triggers it.
func TestReviewRuleIsStateBased(t *testing.T) {
	r := newFrRig(t)
	ctx := context.Background()
	now := time.Now().UTC()
	id := insertPendingReport(t, r.pool, r.q, service.SeverityCritical, service.CategoryFlood, mwDesc, now.Add(-time.Minute), now.Add(6*time.Hour))
	r.mustFlag(id, frAt(1))
	r.mustFlag(id, frAt(2))
	r.mustFlag(id, frAt(3))
	if got := r.state(id); got != "pending" {
		t.Fatalf("state = %q, want pending", got)
	}
	if err := mwDBWorker(r.q, screenerWith(mwPassScores), now).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if got := r.state(id); got != "clear" {
		t.Fatalf("state = %q after the pass, want clear", got)
	}
	r.mustFlag(id, nil)
	if got := r.state(id); got != "review" {
		t.Errorf("state = %q after the next flag, want review", got)
	}
}

// TestHeldCommentOnHiddenReportWaits: the comment worker does not check a held
// comment on a report hidden for review, as it does not on a removed one, so
// no comment row reaches the author's bell about a report nobody can see.
func TestHeldCommentOnHiddenReportWaits(t *testing.T) {
	r := newFrRig(t)
	ctx := context.Background()
	commenter := r.flagger()
	hidden := r.report(service.SeverityLow, service.ModerationReview)
	shown := r.report(service.SeverityLow, service.ModerationClear)
	testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: hidden, AccountID: commenter, State: "pending"})
	onShown := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: shown, AccountID: commenter, State: "pending"})
	now := time.Now().UTC().Add(time.Minute)
	rows, err := r.q.ClaimDueCommentChecks(ctx, sqlcgen.ClaimDueCommentChecksParams{LeaseUntil: now.Add(time.Minute), Now: now, BatchLimit: 10})
	if err != nil {
		t.Fatalf("ClaimDueCommentChecks: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != onShown {
		t.Errorf("claimed %+v, want only the comment on the shown report", rows)
	}
}

// TestOwnShownAndRemovedRows proves the author's rows of the text check and of
// Keep: a held report that passes the late check or is kept gets own_shown
// once, a late reject gets own_removed once, and a pending serious report that
// was already showing gets nothing.
func TestOwnShownAndRemovedRows(t *testing.T) {
	r := newFrRig(t)
	ctx := context.Background()
	now := time.Now().UTC()
	pending := func(sev service.Severity) int64 {
		id := insertPendingReport(t, r.pool, r.q, sev, service.CategoryFlood, mwDesc, now.Add(-time.Minute), now.Add(6*time.Hour))
		testutil.MustExec(t, r.pool, `UPDATE reports SET session_id = $1 WHERE id = $2`, frPosterSess, id)
		return id
	}

	t.Run("a late pass of a held report", func(t *testing.T) {
		testutil.MustExec(t, r.pool, `DELETE FROM moderation_checks`)
		id := pending(service.SeverityLow)
		if err := mwDBWorker(r.q, screenerWith(mwPassScores), now).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if got := r.state(id); got != "clear" {
			t.Fatalf("state = %q, want clear", got)
		}
		if n := r.own(id, service.UpdateOwnShown); n != 1 {
			t.Errorf("own_shown rows = %d, want 1", n)
		}
		if n := r.ownAll(id); n != 1 {
			t.Errorf("%d bell rows, want 1", n)
		}
	})

	t.Run("a late reject", func(t *testing.T) {
		testutil.MustExec(t, r.pool, `DELETE FROM moderation_checks`)
		id := pending(service.SeverityLow)
		if err := mwDBWorker(r.q, screenerWith(mwRejectScores), now).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if got := r.state(id); got != "removed" {
			t.Fatalf("state = %q, want removed", got)
		}
		if n := r.own(id, service.UpdateOwnRemoved); n != 1 {
			t.Errorf("own_removed rows = %d, want 1", n)
		}
	})

	t.Run("a late pass of a pending serious report that was already showing", func(t *testing.T) {
		testutil.MustExec(t, r.pool, `DELETE FROM moderation_checks`)
		id := pending(service.SeverityCritical)
		if err := mwDBWorker(r.q, screenerWith(mwPassScores), now).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if got := r.state(id); got != "clear" {
			t.Fatalf("state = %q, want clear", got)
		}
		if n := r.ownAll(id); n != 0 {
			t.Errorf("%d bell rows, want 0", n)
		}
	})

	t.Run("a moderator Keep of a held report", func(t *testing.T) {
		id := pending(service.SeverityLow)
		if _, err := r.mods.Keep(ctx, id, r.mod); err != nil {
			t.Fatalf("Keep: %v", err)
		}
		if n := r.own(id, service.UpdateOwnShown); n != 1 {
			t.Errorf("own_shown rows = %d, want 1", n)
		}
		if _, err := r.mods.Keep(ctx, id, r.mod); !errors.Is(err, service.ErrAlreadyHandled) {
			t.Errorf("second Keep: %v, want ErrAlreadyHandled", err)
		}
		if n := r.ownAll(id); n != 1 {
			t.Errorf("%d bell rows, want 1", n)
		}
	})

	t.Run("a moderator Remove of a shown report", func(t *testing.T) {
		id := r.report(service.SeverityLow, service.ModerationClear)
		if _, err := r.mods.Remove(ctx, id, r.mod, service.RemovalSpam); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if n := r.own(id, service.UpdateOwnRemoved); n != 1 {
			t.Errorf("own_removed rows = %d, want 1", n)
		}
	})

	t.Run("the rows are never queued for a phone", func(t *testing.T) {
		if n := r.count(`SELECT count(*) FROM push_outbox`); n != 0 {
			t.Errorf("%d phone rows queued, want 0", n)
		}
	})
}

// TestReviewHideLeavesTheRating is the review fix of 2026-11-18 on a real
// Postgres: flags are not a moderation outcome, so a Past report that three
// places hide for review keeps counting in its author's rating exactly as
// before, whether it helped or hurt them, and Keep changes nothing. Remove
// still makes the report neutral, as a removal always has.
func TestReviewHideLeavesTheRating(t *testing.T) {
	r := newFrRig(t)
	ctx := context.Background()
	now := time.Now().UTC()
	past := func(i int, value service.VoteValue) int64 {
		t.Helper()
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
				`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at) VALUES ($1, $2, 'content', $3, $4, $5)`,
				id, r.flagger(), string(value), fmt.Sprintf("tdr1v%d%d", i, k), now.Add(-time.Duration(28+i)*time.Hour)); err != nil {
				t.Fatalf("vote on report %d: %v", id, err)
			}
		}
		return id
	}
	signal := func() service.ReporterSignal {
		t.Helper()
		got, err := service.LoadReporterSignals(ctx, r.q, nil, []int64{r.poster}, now)
		if err != nil {
			t.Fatalf("LoadReporterSignals: %v", err)
		}
		return got[r.poster]
	}
	same := func(name string, got, want service.ReporterSignal) {
		t.Helper()
		sameScore := (got.Score == nil) == (want.Score == nil) && (got.Score == nil || *got.Score == *want.Score)
		if got.Tag != want.Tag || !sameScore || got.JudgedReports != want.JudgedReports || got.CorroboratedReports != want.CorroboratedReports {
			t.Errorf("%s: tag %q judged %d corroborated %d, want %q, %d, %d (score same %v)", name, got.Tag, got.JudgedReports, got.CorroboratedReports, want.Tag, want.JudgedReports, want.CorroboratedReports, sameScore)
		}
	}
	hide := func(id int64) {
		t.Helper()
		for k := 1; k <= service.FlagHideThreshold; k++ {
			r.mustFlag(id, frAt(k))
		}
		if got := r.state(id); got != string(service.ModerationReview) {
			t.Fatalf("report %d after %d places flagged it: state %q, want review", id, service.FlagHideThreshold, got)
		}
	}

	var good, bad []int64
	for i := 0; i < 5; i++ {
		good = append(good, past(i, service.VoteConfirm))
	}
	for i := 5; i < 7; i++ {
		bad = append(bad, past(i, service.VoteDispute))
	}
	before := signal()
	if before.JudgedReports != 7 || before.CorroboratedReports != 5 {
		t.Fatalf("setup: judged %d corroborated %d, want 7 and 5", before.JudgedReports, before.CorroboratedReports)
	}

	hide(good[0])
	hide(bad[0])
	same("one corroborated and one contradicted Past report hidden for review", signal(), before)

	if _, err := r.mods.Keep(ctx, good[0], r.mod); err != nil {
		t.Fatalf("Keep: %v", err)
	}
	same("after Keep", signal(), before)

	if _, err := r.mods.Remove(ctx, bad[0], r.mod, service.RemovalSpam); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := signal(); got.JudgedReports != 6 || got.CorroboratedReports != 5 {
		t.Errorf("after Remove: judged %d corroborated %d, want 6 and 5, a removed report stays neutral", got.JudgedReports, got.CorroboratedReports)
	}
}
