package service

import (
	"context"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestFollowEndHelpers proves the two helpers every path that ends follows goes
// through (review W1): endFollowsOf ends every active follow of the followee
// with the reason and returns the count, endFollowPair ends one, and ended rows
// are left in place with their reason (D-21).
func TestFollowEndHelpers(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

	target := testutil.SeedPerson(t, pool, "end-target@example.com", "end.target", true)
	other := testutil.SeedPerson(t, pool, "end-other@example.com", "end.other", true)
	f1 := testutil.SeedPerson(t, pool, "end-f1@example.com", "end.one", false)
	f2 := testutil.SeedPerson(t, pool, "end-f2@example.com", "end.two", false)
	f3 := testutil.SeedPerson(t, pool, "end-f3@example.com", "end.three", false)
	for _, f := range []int64{f1, f2, f3} {
		testutil.SeedFollow(t, pool, f, target, now)
	}
	testutil.SeedFollow(t, pool, f1, other, now)

	state := func(follower, followee int64) (ended bool, reason string) {
		t.Helper()
		var r *string
		if err := pool.QueryRow(ctx,
			`SELECT ended_at IS NOT NULL, end_reason FROM account_follows WHERE follower_id = $1 AND followee_id = $2`,
			follower, followee).Scan(&ended, &r); err != nil {
			t.Fatalf("reading follow %d -> %d: %v", follower, followee, err)
		}
		if r != nil {
			reason = *r
		}
		return ended, reason
	}

	n, err := endFollowPair(ctx, q, f1, target, FollowEndUnfollowed, now.Add(time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("endFollowPair = %d (%v), want 1", n, err)
	}
	if ended, reason := state(f1, target); !ended || reason != "unfollowed" {
		t.Errorf("f1 -> target: ended %v reason %q, want ended with unfollowed", ended, reason)
	}
	if ended, _ := state(f2, target); ended {
		t.Error("endFollowPair ended a follow it was not asked to end")
	}
	if n, err := endFollowPair(ctx, q, f1, target, FollowEndRemoved, now.Add(2*time.Hour)); err != nil || n != 0 {
		t.Errorf("a second endFollowPair = %d (%v), want 0", n, err)
	}
	if _, reason := state(f1, target); reason != "unfollowed" {
		t.Errorf("an ended follow's reason changed to %q, want it kept", reason)
	}

	n, err = endFollowsOf(ctx, q, target, FollowEndProfileOff, now.Add(3*time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("endFollowsOf = %d (%v), want the 2 still active", n, err)
	}
	for _, f := range []int64{f2, f3} {
		if ended, reason := state(f, target); !ended || reason != "profile_off" {
			t.Errorf("follow %d -> target: ended %v reason %q, want ended with profile_off", f, ended, reason)
		}
	}
	if ended, _ := state(f1, other); ended {
		t.Error("endFollowsOf ended a follow of another person")
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM account_follows`).Scan(&rows); err != nil || rows != 4 {
		t.Errorf("rows after ending = %d (%v), want all 4 kept", rows, err)
	}
	if n, err := endFollowsOf(ctx, q, target, FollowEndTagLost, now.Add(4*time.Hour)); err != nil || n != 0 {
		t.Errorf("endFollowsOf with nothing active = %d (%v), want 0", n, err)
	}
}

// TestFollowEndHelpersDeleteQueuedNotes: endFollowPair deletes the queued follow
// notes of one pair and endFollowsOf those of every follower of the followee. A
// sent note, a note about another person's report and a note a worker holds
// (lease in the future) stay, and so do the bell rows (research Q9).
func TestFollowEndHelpersDeleteQueuedNotes(t *testing.T) {
	s := newCleanupScene(t)
	r := s.r
	ctx := context.Background()
	_, otherSession := r.person("cl.other")
	otherReport := r.report(otherSession, SeverityLow, r.base.Add(21*time.Minute), "clear")
	r.queueFollowRow(s.followers[0], otherReport, "queued")
	// A bell row of the first follower about the person's report.
	testutil.MustExec(t, r.pool,
		`INSERT INTO updates (account_id, report_id, kind, created_at) VALUES ($1, $2, 'followed_posted', $3)`,
		s.followers[0], s.report, r.base.Add(21*time.Minute))
	// A note a worker holds right now: its lease runs on.
	heldReport := r.report(s.session, SeverityLow, r.base.Add(22*time.Minute), "clear")
	r.queueFollowRow(s.followers[2], heldReport, "queued")
	testutil.MustExec(t, r.pool,
		`UPDATE push_outbox SET lease_until = now() + interval '1 hour' WHERE account_id = $1 AND report_id = $2`,
		s.followers[2], heldReport)

	n, err := endFollowPair(ctx, r.q, s.followers[0], s.person, FollowEndUnfollowed, r.cur)
	if err != nil || n != 1 {
		t.Fatalf("endFollowPair = %d (%v), want 1", n, err)
	}
	if got := r.followRowsIn(s.followers[0], "queued"); got != 1 {
		t.Errorf("follower 1 holds %d queued follow notes, want 1 (only the one about the other person's report stays)", got)
	}
	if got := r.count(`SELECT COUNT(*) FROM push_outbox WHERE account_id = $1 AND report_id = $2`, s.followers[0], otherReport); got != 1 {
		t.Errorf("the note about another person's report was deleted")
	}
	if got := r.followRowsIn(s.followers[1], "queued"); got != 1 {
		t.Errorf("follower 2 holds %d queued notes after a pair end for follower 1, want 1", got)
	}
	if got := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND kind = 'followed_posted'`, s.followers[0]); got != 1 {
		t.Errorf("bell rows of follower 1 = %d, want 1 (the bell row stays)", got)
	}

	if _, err := endFollowsOf(ctx, r.q, s.person, FollowEndProfileOff, r.cur); err != nil {
		t.Fatalf("endFollowsOf: %v", err)
	}
	if got := r.followRowsIn(s.followers[1], "queued"); got != 0 {
		t.Errorf("follower 2 holds %d queued notes after endFollowsOf, want 0", got)
	}
	if got := r.count(`SELECT COUNT(*) FROM push_outbox WHERE account_id = $1 AND report_id = $2`, s.followers[2], heldReport); got != 1 {
		t.Errorf("the note a worker holds was deleted; its worker checks the follow at send time")
	}
	if got := r.followRowsIn(s.followers[2], "queued"); got != 1 {
		t.Errorf("follower 3 holds %d queued notes, want only the held one", got)
	}
	for i, f := range s.followers {
		if got := r.followRowsIn(f, "sent"); got != 1 {
			t.Errorf("follower %d holds %d sent notes, want 1 (history stays)", i+1, got)
		}
	}
}
