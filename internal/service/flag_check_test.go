package service_test

// flag_check_test.go proves quick 261118-saj on a real Postgres: flags from
// FlagHideThreshold different places never hide a serious report (critical,
// or a stored rescue_needed report). It stays shown, Live everywhere a clear
// Critical report is, with the Being checked mark; it comes first on the
// moderator's flagged list; Keep takes the mark off and settles it, so flags
// never mark or hide it again; Remove removes it and the mark goes with it. A
// non serious report keeps the review hide of quick 261117-vvb, which
// flag_review_test.go proves. Every test starts with testutil.NewTestDB, so it
// skips in the quick run.

import (
	"context"
	"errors"
	"testing"
	"time"

	"pinalert/internal/service"
	"pinalert/internal/testutil"
)

// marked says whether the Being checked mark is on: flag_check_at set and the
// report not settled (migration 00028).
func (r *frRig) marked(id int64) bool {
	r.t.Helper()
	return r.count(`SELECT count(*) FROM reports WHERE id = $1 AND flag_check_at IS NOT NULL AND review_kept_at IS NULL`, id) == 1
}

// rescue stores one report in the retired rescue_needed category at low
// severity, which is serious by its category.
func (r *frRig) rescue() int64 {
	r.t.Helper()
	now := time.Now().UTC()
	return testutil.InsertStoredReport(r.t, r.pool, testutil.StoredReport{
		SessionID:   frPosterSess,
		Category:    string(service.CategoryRescueNeeded),
		Severity:    string(service.SeverityLow),
		Description: "Two people on a roof near the old bridge",
		Latitude:    frLat,
		Longitude:   frLon,
		CreatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(6 * time.Hour),
	})
}

// threePlaces flags the report from three new accounts in three different
// place cells, starting at cell k.
func (r *frRig) threePlaces(id int64, k int) {
	r.t.Helper()
	for i := 0; i < 3; i++ {
		r.mustFlag(id, frAt(k+i))
	}
}

// liveView reads the report through Nearby and Focus, as every reader does,
// and returns the Nearby view. A marked report must be listed there.
func (r *frRig) liveView(id int64) service.ReportView {
	r.t.Helper()
	ctx := context.Background()
	reports := service.NewReportService(r.q)
	views, err := reports.Nearby(ctx, service.NearbyQuery{Latitude: frLat, Longitude: frLon, RadiusKm: 5})
	if err != nil {
		r.t.Fatalf("Nearby: %v", err)
	}
	var found *service.ReportView
	for i := range views {
		if views[i].ID == id {
			found = &views[i]
		}
	}
	if found == nil {
		r.t.Fatalf("report %d is not on Nearby", id)
	}
	focus, err := reports.Focus(ctx, service.FocusQuery{ReportID: id, Latitude: frLat, Longitude: frLon})
	if err != nil {
		r.t.Fatalf("Focus: %v", err)
	}
	if focus.Status != service.FocusOK || focus.View == nil {
		r.t.Fatalf("Focus of report %d = %q, want ok", id, focus.Status)
	}
	if focus.View.BeingChecked != found.BeingChecked || focus.View.Visibility != found.Visibility {
		r.t.Errorf("Focus and Nearby disagree on report %d: mark %v and %v, visibility %q and %q",
			id, focus.View.BeingChecked, found.BeingChecked, focus.View.Visibility, found.Visibility)
	}
	return *found
}

func TestFlagCheckMark(t *testing.T) {
	r := newFrRig(t)
	ctx := context.Background()

	t.Run("three places on a critical report mark it and do not hide it", func(t *testing.T) {
		id := r.report(service.SeverityCritical, service.ModerationClear)
		r.mustFlag(id, frAt(1))
		r.mustFlag(id, frAt(2))
		if r.marked(id) {
			t.Fatal("two places marked the report")
		}
		r.mustFlag(id, frAt(3))
		if got := r.state(id); got != "clear" {
			t.Errorf("state = %q, want clear", got)
		}
		if !r.marked(id) {
			t.Error("three places did not mark the report")
		}
		if n := r.ownAll(id); n != 0 {
			t.Errorf("%d bell rows for the author, want 0: a marked report was never hidden", n)
		}
		v := r.liveView(id)
		if v.Visibility != service.VisibilityLive || !v.BeingChecked {
			t.Errorf("marked report reads visibility %q and mark %v, want live and true", v.Visibility, v.BeingChecked)
		}
		// A marked report is shown, so more people can still flag it, and the
		// mark is set once.
		var before time.Time
		if err := r.pool.QueryRow(ctx, `SELECT flag_check_at FROM reports WHERE id = $1`, id).Scan(&before); err != nil {
			t.Fatal(err)
		}
		r.mustFlag(id, frAt(4))
		var after time.Time
		if err := r.pool.QueryRow(ctx, `SELECT flag_check_at FROM reports WHERE id = $1`, id).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if !after.Equal(before) || r.state(id) != "clear" {
			t.Errorf("a fourth place moved the mark from %v to %v or the state to %q", before, after, r.state(id))
		}
	})

	t.Run("a stored rescue report is serious by its category", func(t *testing.T) {
		id := r.rescue()
		r.threePlaces(id, 1)
		if got := r.state(id); got != "clear" || !r.marked(id) {
			t.Errorf("rescue report: state %q, marked %v; want clear and marked", got, r.marked(id))
		}
	})

	t.Run("a non serious report is still hidden and never marked", func(t *testing.T) {
		id := r.report(service.SeverityMedium, service.ModerationClear)
		r.threePlaces(id, 1)
		if got := r.state(id); got != "review" {
			t.Errorf("state = %q, want review", got)
		}
		if n := r.count(`SELECT count(*) FROM reports WHERE id = $1 AND flag_check_at IS NOT NULL`, id); n != 0 {
			t.Error("a non serious report got the mark")
		}
	})

	t.Run("an unmarked critical report reads no mark", func(t *testing.T) {
		id := r.report(service.SeverityCritical, service.ModerationClear)
		r.mustFlag(id, frAt(1))
		if v := r.liveView(id); v.BeingChecked || v.Visibility != service.VisibilityLive {
			t.Errorf("one flag: mark %v visibility %q, want false and live", v.BeingChecked, v.Visibility)
		}
	})

	t.Run("a pending serious report gets the mark when the threshold is met", func(t *testing.T) {
		now := time.Now().UTC()
		id := insertPendingReport(t, r.pool, r.q, service.SeverityCritical, service.CategoryFlood, mwDesc, now.Add(-time.Minute), now.Add(6*time.Hour))
		r.threePlaces(id, 1)
		if got := r.state(id); got != "pending" || !r.marked(id) {
			t.Fatalf("pending serious report: state %q, marked %v; want pending and marked", got, r.marked(id))
		}
		if v := r.liveView(id); !v.BeingChecked || v.Visibility != service.VisibilityLive {
			t.Errorf("pending marked report: mark %v visibility %q, want true and live", v.BeingChecked, v.Visibility)
		}
		// The late check passes it: it is clear and the mark stays until a
		// moderator looks.
		if err := mwDBWorker(r.q, screenerWith(mwPassScores), now).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if got := r.state(id); got != "clear" || !r.marked(id) {
			t.Errorf("after the pass: state %q, marked %v; want clear and marked", got, r.marked(id))
		}
	})

	t.Run("a late text check removal takes the mark with it", func(t *testing.T) {
		now := time.Now().UTC()
		id := insertPendingReport(t, r.pool, r.q, service.SeverityCritical, service.CategoryFlood, mwDesc, now.Add(-time.Minute), now.Add(6*time.Hour))
		r.threePlaces(id, 1)
		if err := mwDBWorker(r.q, screenerWith(mwRejectScores), now).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if got := r.state(id); got != "removed" {
			t.Fatalf("state = %q, want removed", got)
		}
		if n := r.count(`SELECT count(*) FROM reports WHERE id = $1 AND flag_check_at IS NULL`, id); n != 1 {
			t.Error("the removal left flag_check_at set")
		}
	})

	t.Run("Keep from the mark settles it for good and tells the author nothing", func(t *testing.T) {
		id := r.report(service.SeverityCritical, service.ModerationClear)
		r.threePlaces(id, 1)
		row, err := r.mods.Keep(ctx, id, r.mod)
		if err != nil {
			t.Fatalf("Keep: %v", err)
		}
		if row.BeingChecked || row.Moderation != service.ModerationClear {
			t.Errorf("Keep answer: mark %v moderation %q, want false and clear", row.BeingChecked, row.Moderation)
		}
		if r.marked(id) {
			t.Error("the mark is still on after Keep")
		}
		if n := r.count(`SELECT count(*) FROM reports WHERE id = $1 AND review_kept_at IS NOT NULL AND flag_check_at IS NOT NULL`, id); n != 1 {
			t.Error("Keep from the mark did not settle the report with flag_check_at kept")
		}
		if n := r.ownAll(id); n != 0 {
			t.Errorf("%d bell rows after Keep, want 0", n)
		}
		if _, err := r.mods.Keep(ctx, id, r.mod); !errors.Is(err, service.ErrAlreadyHandled) {
			t.Errorf("a repeat Keep: %v, want ErrAlreadyHandled", err)
		}
		// A fresh set of flags from three new places neither marks nor hides it,
		// and still reaches the moderator.
		r.threePlaces(id, 5)
		if got := r.state(id); got != "clear" || r.marked(id) {
			t.Errorf("after a fresh set of flags: state %q, marked %v; want clear and unmarked", got, r.marked(id))
		}
		if v := r.liveView(id); v.BeingChecked {
			t.Error("a settled report reads the mark")
		}
		queues, err := r.mods.Queues(ctx)
		if err != nil {
			t.Fatalf("Queues: %v", err)
		}
		found := false
		for _, fr := range queues.Flagged {
			if fr.ID == id {
				found = true
				if fr.BeingChecked {
					t.Error("a settled report is tagged Still showing")
				}
			}
		}
		if !found {
			t.Error("the settled report with three new flags is not on the flagged list")
		}
	})

	t.Run("Remove from the mark removes it, tells the author once and the mark goes", func(t *testing.T) {
		id := r.report(service.SeverityCritical, service.ModerationClear)
		r.threePlaces(id, 1)
		row, err := r.mods.Remove(ctx, id, r.mod, service.RemovalFalse)
		if err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if row.BeingChecked || row.Moderation != service.ModerationRemoved {
			t.Errorf("Remove answer: mark %v moderation %q", row.BeingChecked, row.Moderation)
		}
		if n := r.count(`SELECT count(*) FROM reports WHERE id = $1 AND flag_check_at IS NULL AND review_kept_at IS NULL`, id); n != 1 {
			t.Error("Remove left the mark or settled the report")
		}
		if n := r.own(id, service.UpdateOwnRemoved); n != 1 {
			t.Errorf("own_removed rows = %d, want 1", n)
		}
		if _, err := r.mods.Remove(ctx, id, r.mod, service.RemovalFalse); !errors.Is(err, service.ErrAlreadyHandled) {
			t.Errorf("a repeat Remove: %v, want ErrAlreadyHandled", err)
		}
		// Restored, it is settled and silent, and flags never mark it.
		if _, err := r.mods.Restore(ctx, id, r.mod); err != nil {
			t.Fatalf("Restore: %v", err)
		}
		r.threePlaces(id, 5)
		if got := r.state(id); got != "clear" || r.marked(id) {
			t.Errorf("restored report after flags: state %q, marked %v; want clear and unmarked", got, r.marked(id))
		}
		if n := r.count(`SELECT count(*) FROM reports WHERE id = $1 AND flag_check_at IS NULL`, id); n != 1 {
			t.Error("a restored report carries flag_check_at")
		}
	})

	t.Run("a report kept after a hide is never marked", func(t *testing.T) {
		id := r.report(service.SeverityCritical, service.ModerationClear)
		testutil.MustExec(t, r.pool, `UPDATE reports SET review_kept_at = now() WHERE id = $1`, id)
		r.threePlaces(id, 1)
		if got := r.state(id); got != "clear" || r.marked(id) {
			t.Errorf("state %q, marked %v; want clear and unmarked", got, r.marked(id))
		}
		if n := r.count(`SELECT count(*) FROM reports WHERE id = $1 AND flag_check_at IS NULL`, id); n != 1 {
			t.Error("a settled report got flag_check_at")
		}
	})

	t.Run("the moderator list puts a marked report first with the hidden ones", func(t *testing.T) {
		busy := r.report(service.SeverityLow, service.ModerationClear)
		marked := r.report(service.SeverityCritical, service.ModerationClear)
		r.threePlaces(marked, 1)
		// busy gets more flags than the marked report, all from one place.
		for i := 0; i < 5; i++ {
			r.mustFlag(busy, frAt(1))
		}
		queues, err := r.mods.Queues(ctx)
		if err != nil {
			t.Fatalf("Queues: %v", err)
		}
		pos := map[int64]int{}
		for i, fr := range queues.Flagged {
			pos[fr.ID] = i
			if fr.ID == marked {
				if !fr.BeingChecked || fr.Moderation != service.ModerationClear || fr.View != service.ModViewActive {
					t.Errorf("marked row: mark %v moderation %q view %q, want true, clear and active", fr.BeingChecked, fr.Moderation, fr.View)
				}
			}
			if fr.ID == busy && fr.BeingChecked {
				t.Error("an unmarked report is tagged Still showing")
			}
		}
		if pos[marked] > pos[busy] {
			t.Errorf("the marked report is at %d, after the busier unmarked one at %d", pos[marked], pos[busy])
		}
		for i, fr := range queues.Flagged {
			first := fr.Moderation == service.ModerationReview || fr.BeingChecked
			if !first {
				for _, later := range queues.Flagged[i+1:] {
					if later.Moderation == service.ModerationReview || later.BeingChecked {
						t.Fatalf("a hidden or marked report %d comes after an ordinary one %d", later.ID, fr.ID)
					}
				}
				break
			}
		}
		found, _, err := r.mods.Find(ctx, marked)
		if err != nil || !found.BeingChecked {
			t.Errorf("Find of the marked report: mark %v, err %v", found.BeingChecked, err)
		}
	})
}

// TestFlagCheckConcurrent sends flags from six places at the same instant on a
// critical report. The report row lock makes the mark happen exactly once,
// with no hide and no bell row.
func TestFlagCheckConcurrent(t *testing.T) {
	r := newFrRig(t)
	for round := 0; round < 5; round++ {
		id := r.report(service.SeverityCritical, service.ModerationClear)
		accounts := make([]int64, 6)
		for i := range accounts {
			accounts[i] = r.flagger()
		}
		errs := make(chan error, len(accounts))
		start := make(chan struct{})
		for i, acc := range accounts {
			go func(k int, acc int64) {
				<-start
				errs <- r.flag(id, acc, frAt(k))
			}(i+1, acc)
		}
		close(start)
		for range accounts {
			if err := <-errs; err != nil {
				t.Errorf("round %d: flag: %v", round, err)
			}
		}
		if got := r.state(id); got != "clear" || !r.marked(id) {
			t.Errorf("round %d: state %q, marked %v; want clear and marked", round, got, r.marked(id))
		}
		if n := r.ownAll(id); n != 0 {
			t.Errorf("round %d: %d bell rows, want 0", round, n)
		}
		if n := r.count(`SELECT count(*) FROM report_flags WHERE report_id = $1`, id); n != 6 {
			t.Errorf("round %d: %d flags stored, want 6", round, n)
		}
	}
}
