// followpush_test.go proves the follow note on the phone (Phase 9 plan 25,
// PROF-03, D-09, D-25, research 5.2 "Phone"): the dispatcher's follow branch
// queues one follow push for every bell row the Notify me step wrote, in the
// order Critical, Follow, Reliable, and records its pass once. The database
// tests run the real services on the follow rig of plan 09-22 with the dispatcher
// wired to the same rating source, so a reporter's tag is the rated tag. They
// skip without DATABASE_URL.
//
// Every person here is made up and uses example.com.

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/testutil"
)

// countingRefresher counts the Refresh calls of one dispatcher and passes them on.
type countingRefresher struct {
	inner UpdatesRefresher
	mu    sync.Mutex
	calls map[int64]int
}

func (c *countingRefresher) Refresh(ctx context.Context, accountID int64) error {
	c.mu.Lock()
	if c.calls == nil {
		c.calls = map[int64]int{}
	}
	c.calls[accountID]++
	c.mu.Unlock()
	return c.inner.Refresh(ctx, accountID)
}

func (c *countingRefresher) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.calls {
		n += v
	}
	return n
}

// newFollowPushRig is the follow rig with the dispatcher rebuilt on the rig's
// updates service (the one that has the rating source) and the same source.
func newFollowPushRig(t *testing.T) *followRig {
	t.Helper()
	r := newFollowRig(t)
	clock := func() time.Time { return r.cur }
	r.d = NewPushDispatcher(r.q, r.updates, r.namer, WithDispatchClock(clock), WithDispatchSignals(NewRatingService(r.q)))
	return r
}

// device gives an account one live push subscription on its own session.
func (r *followRig) device(account int64, session string) {
	r.t.Helper()
	testutil.SeedPushSubscription(r.t, r.pool, account, session, "")
}

// followProgress is the stored follow_state of a report, "" while NULL or no row.
func (r *followRig) followProgress(report int64) string {
	r.t.Helper()
	p, err := r.q.GetPushReportProgress(context.Background(), report)
	if errors.Is(err, pgx.ErrNoRows) {
		return ""
	}
	if err != nil {
		r.t.Fatalf("reading progress of report %d: %v", report, err)
	}
	if p.FollowState == nil {
		return ""
	}
	return *p.FollowState
}

// TestFollowPushQueuedFromBellRows: a follower with the bell on and a device
// gets one queued follow push for a Provisional report by the followed person,
// after the bell row exists. A follower with the bell off, a follower with the
// bell on and no device, a report with no reporter account and a repeat
// evaluation add nothing.
func TestFollowPushQueuedFromBellRows(t *testing.T) {
	r := newFollowPushRig(t)
	person, session := r.person("push.writer")
	withBell := r.follower("withbell")
	r.device(withBell, "p9-25-sess-withbell")
	bellOff := r.follower("bell.off")
	r.device(bellOff, "p9-25-sess-belloff")
	noDevice := r.follower("nodevice")

	r.bellOn(withBell, person, r.base.Add(10*time.Minute))
	testutil.SeedFollow(t, r.pool, bellOff, person, r.base)
	r.bellOn(noDevice, person, r.base.Add(10*time.Minute))

	r.cur = r.base.Add(30 * time.Minute)
	report := r.report(session, SeverityLow, r.base.Add(20*time.Minute), "clear")
	r.eval(report, 0)

	got := r.outbox(report)
	if !outboxKindsEqual(got[withBell], "follow") {
		t.Fatalf("the follower with the bell on holds %v, want one follow row", got[withBell])
	}
	if len(got[bellOff]) != 0 {
		t.Errorf("the follower with the bell off holds %v, want none", got[bellOff])
	}
	if len(got[noDevice]) != 0 {
		t.Errorf("the follower with no device holds %v, want none", got[noDevice])
	}
	// One row, two channels: the push is tied to the bell row the Notify me step wrote.
	if n := r.notes(withBell, report); n != 1 {
		t.Errorf("bell rows for the follower with the bell on = %d, want 1", n)
	}
	if n := r.notes(bellOff, report); n != 0 {
		t.Errorf("bell rows for the follower with the bell off = %d, want 0", n)
	}
	if n := r.notes(noDevice, report); n != 1 {
		t.Errorf("bell rows for the follower with no device = %d, want 1 (the bell does not need a device)", n)
	}
	if st := r.followProgress(report); st != "fanned" {
		t.Errorf("follow_state = %q, want fanned", st)
	}

	// Repeat evaluations (sweep and poke) add no row.
	r.eval(report, 0)
	r.eval(report, 77)
	if n := len(r.outbox(report)[withBell]); n != 1 {
		t.Errorf("the follower holds %d rows after repeat evaluations, want 1", n)
	}

	// A report posted without an account is never pushed and is not marked.
	anon := r.report("p9-25-anonymous-session", SeverityLow, r.base.Add(21*time.Minute), "clear")
	r.eval(anon, 0)
	if rows := r.outbox(anon); len(rows) != 0 {
		t.Errorf("a report with no reporter account queued %v, want nothing", rows)
	}
	if st := r.followProgress(anon); st != "" {
		t.Errorf("follow_state of a report with no reporter account = %q, want none", st)
	}
}

// TestFollowPushOrderCriticalFollowReliable: when one report matches several
// kinds for one person only the first is queued: Critical, then Follow, then
// Reliable. A Critical report goes as Critical to a follower with Critical on
// and as the follow note to a follower with Critical off, even with the Reliable
// alert on; a plain report goes as the follow note to a follower who also has the
// Reliable alert on, and as Reliable to a reader without the bell.
//
// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box): the
// stored Critical and Reliable flags no longer decide and a Reliable note never
// reaches a phone, so Critical wins for every reader in range (the follow note
// is the Critical report's second alert and loses the shared slot), a plain
// report goes as the follow note to a follower, and a reader without the bell
// holds nothing.
func TestFollowPushOrderCriticalFollowReliable(t *testing.T) {
	r := newFollowPushRig(t)
	person, session := r.person("push.order")

	// Four readers near the report point, each with a device and a home cell.
	criticalOn, _ := r.reader("crit", 0, 5, true, false, false)
	criticalOff, _ := r.reader("nocrit", 0, 5, false, true, false)
	both, _ := r.reader("both", 0, 5, true, true, false)
	reliableOnly, _ := r.reader("relonly", 0, 5, true, true, false)
	for _, id := range []int64{criticalOn, criticalOff, both} {
		r.bellOn(id, person, r.base.Add(10*time.Minute))
	}

	// A Critical report: Critical wins for the reader with Critical on, the follow
	// note wins over Reliable for the reader with Critical off.
	r.cur = r.base.Add(30 * time.Minute)
	critical := r.report(session, SeverityCritical, r.base.Add(20*time.Minute), "clear")
	r.eval(critical, 0)
	got := r.outbox(critical)
	if !outboxKindsEqual(got[criticalOn], "critical") {
		t.Errorf("Critical on with the bell holds %v, want critical alone", got[criticalOn])
	}
	if !outboxKindsEqual(got[criticalOff], "critical") {
		t.Errorf("stored Critical off, Reliable on, bell on holds %v, want critical alone", got[criticalOff])
	}
	if !outboxKindsEqual(got[both], "critical") {
		t.Errorf("Critical and Reliable on with the bell holds %v, want critical alone", got[both])
	}
	if !outboxKindsEqual(got[reliableOnly], "critical") {
		t.Errorf("a reader without the bell and with Critical on holds %v, want critical alone", got[reliableOnly])
	}

	// A plain report by a second person (the Reliable alert lets one reporter alert
	// a reader once an hour, so the first person's Critical report would otherwise
	// use that slot): the follow note beats Reliable for the follower, Reliable
	// still reaches the reader who does not follow.
	second, secondSession := r.person("push.order2")
	for _, id := range []int64{criticalOn, criticalOff, both} {
		r.bellOn(id, second, r.base.Add(10*time.Minute))
	}
	r.cur = r.base.Add(40 * time.Minute)
	plain := r.report(secondSession, SeverityLow, r.base.Add(35*time.Minute), "clear")
	r.eval(plain, 0)
	got = r.outbox(plain)
	for name, id := range map[string]int64{"Critical on": criticalOn, "Critical off": criticalOff, "both on": both} {
		if !outboxKindsEqual(got[id], "follow") {
			t.Errorf("%s with the bell holds %v for a plain report, want follow alone", name, got[id])
		}
	}
	if len(got[reliableOnly]) != 0 {
		t.Errorf("a reader without the bell holds %v for a plain report, want nothing (Reliable is bell only)", got[reliableOnly])
	}
	// At most one first alert per report and person, whatever the kind.
	if n := r.count(`SELECT COUNT(*) FROM (SELECT account_id FROM push_outbox WHERE report_id = ANY($1)
		GROUP BY account_id, report_id HAVING COUNT(*) > 1) x`, []int64{critical, plain}); n != 0 {
		t.Errorf("%d person and report pairs hold more than one row", n)
	}
}

// TestFollowPushFannedAfterOnePass: a pass that queued nothing is still recorded
// and a later evaluation does not refresh the followers again (review W2); a
// report that is Withheld at its first evaluation is not marked and gets its pass
// once it is listable.
func TestFollowPushFannedAfterOnePass(t *testing.T) {
	r := newFollowPushRig(t)
	counter := &countingRefresher{inner: r.updates}
	clock := func() time.Time { return r.cur }
	r.d = NewPushDispatcher(r.q, counter, r.namer, WithDispatchClock(clock), WithDispatchSignals(NewRatingService(r.q)))

	person, session := r.person("push.pass")
	loner, loneSession := r.person("push.lone")
	follower := r.follower("passfollower")
	r.device(follower, "p9-25-sess-passfollower")
	r.bellOn(follower, person, r.base.Add(10*time.Minute))
	r.cur = r.base.Add(30 * time.Minute)

	// Nobody has the bell on for the lone person: the pass finds nobody, and is
	// still recorded, with no refresh at all.
	lone := r.report(loneSession, SeverityLow, r.base.Add(20*time.Minute), "clear")
	r.eval(lone, 0)
	if st := r.followProgress(lone); st != "fanned" {
		t.Errorf("follow_state of a pass that found nobody = %q, want fanned", st)
	}
	if counter.total() != 0 {
		t.Errorf("%d refreshes for a person with no bell followers, want 0", counter.total())
	}
	_ = loner

	// A report that is held by the text check at its first evaluation: no mark.
	held := r.report(session, SeverityLow, r.base.Add(21*time.Minute), "pending")
	r.eval(held, 0)
	if st := r.followProgress(held); st != "" {
		t.Errorf("follow_state of a Withheld report = %q, want none", st)
	}
	if counter.total() != 0 || len(r.outbox(held)) != 0 {
		t.Errorf("a Withheld report refreshed %d followers and queued %v, want nothing", counter.total(), r.outbox(held))
	}
	testutil.MustExec(t, r.pool, `UPDATE reports SET moderation_state = 'clear' WHERE id = $1`, held)
	r.eval(held, 0)
	if st := r.followProgress(held); st != "fanned" {
		t.Errorf("follow_state once listable = %q, want fanned", st)
	}
	if !outboxKindsEqual(r.outbox(held)[follower], "follow") {
		t.Errorf("the follower holds %v once the report is listable, want one follow row", r.outbox(held)[follower])
	}
	first := counter.total()
	if first != 1 {
		t.Fatalf("%d refreshes for the one follower, want 1", first)
	}

	// Later evaluations, with and without an actor, do not refresh again.
	r.eval(held, 0)
	r.eval(held, 55)
	if counter.total() != first {
		t.Errorf("%d refreshes after repeat evaluations, want still %d", counter.total(), first)
	}

	// Past the initial window a new report gets no pass.
	r.cur = r.base.Add(3 * time.Hour)
	old := r.report(session, SeverityLow, r.base.Add(20*time.Minute), "clear")
	r.eval(old, 0)
	if st := r.followProgress(old); st != "" {
		t.Errorf("follow_state of a report past the window = %q, want none", st)
	}
}
