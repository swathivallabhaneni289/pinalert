package store_test

import (
	"context"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestFollowNotifyQueries covers the Notify me queries of plan 09-21 (D-08,
// D-21): the flag is off by default, SetFollowNotify keeps the first time it
// was turned on, every read counts active follows only, and ending a follow by
// any path (EndFollowPair, EndFollowsOf) turns the flag off, so a follow
// restored or followed again comes back with the bell off.
func TestFollowNotifyQueries(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()

	rep := testutil.SeedPerson(t, pool, "notify-rep@example.com", "notify.rep", true)
	rep2 := testutil.SeedPerson(t, pool, "notify-rep2@example.com", "notify.rep2", true)
	f1 := testutil.SeedPerson(t, pool, "notify-f1@example.com", "notify.f1", false)
	f2 := testutil.SeedPerson(t, pool, "notify-f2@example.com", "notify.f2", false)
	for _, pair := range [][2]int64{{f1, rep}, {f2, rep}, {f1, rep2}} {
		if follow(t, q, pair[0], pair[1], peopleNow) != 1 {
			t.Fatalf("seeding follow %d -> %d failed", pair[0], pair[1])
		}
	}

	notifyOn := func(follower, followee int64) bool {
		t.Helper()
		on, err := q.ActiveFollowNotify(ctx, sqlcgen.ActiveFollowNotifyParams{FollowerID: follower, FolloweeID: followee})
		if err != nil {
			t.Fatalf("ActiveFollowNotify: %v", err)
		}
		return on
	}
	set := func(follower, followee int64, on bool, at time.Time) int64 {
		t.Helper()
		n, err := q.SetFollowNotify(ctx, sqlcgen.SetFollowNotifyParams{
			FollowerID: follower, FolloweeID: followee, Notify: on, Now: at,
		})
		if err != nil {
			t.Fatalf("SetFollowNotify: %v", err)
		}
		return n
	}
	count := func(follower int64) int64 {
		t.Helper()
		n, err := q.CountNotifyFollows(ctx, follower)
		if err != nil {
			t.Fatalf("CountNotifyFollows: %v", err)
		}
		return n
	}
	followers := func(followee int64) []int64 {
		t.Helper()
		ids, err := q.NotifyFollowersOf(ctx, sqlcgen.NotifyFollowersOfParams{FolloweeID: followee, RowLimit: 50})
		if err != nil {
			t.Fatalf("NotifyFollowersOf: %v", err)
		}
		return ids
	}

	// Off by default.
	if notifyOn(f1, rep) || count(f1) != 0 || len(followers(rep)) != 0 {
		t.Fatal("a new follow must have Notify me off")
	}

	// On, with the time of the first turn on kept by a repeat.
	first := peopleNow.Add(time.Hour)
	if set(f1, rep, true, first) != 1 || !notifyOn(f1, rep) {
		t.Fatal("turning Notify me on failed")
	}
	if set(f1, rep, true, first.Add(time.Hour)) != 1 {
		t.Fatal("repeating Notify me on changed a different number of rows")
	}
	if set(f1, rep2, true, first) != 1 || set(f2, rep, true, first) != 1 {
		t.Fatal("turning the other bells on failed")
	}
	rows, err := q.NotifyFollowsFor(ctx, sqlcgen.NotifyFollowsForParams{FollowerID: f1, RowLimit: 50})
	if err != nil {
		t.Fatalf("NotifyFollowsFor: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("NotifyFollowsFor listed %d people, want 2", len(rows))
	}
	for _, r := range rows {
		if !r.NotifySince.Equal(first) {
			t.Errorf("followee %d: notify_since %v, want the first turn on %v", r.FolloweeID, r.NotifySince, first)
		}
	}
	if got := count(f1); got != 2 {
		t.Errorf("CountNotifyFollows = %d, want 2", got)
	}
	if got := followers(rep); len(got) != 2 || got[0] != f1 || got[1] != f2 {
		t.Errorf("NotifyFollowersOf(rep) = %v, want [%d %d]", got, f1, f2)
	}
	limited, err := q.NotifyFollowersOf(ctx, sqlcgen.NotifyFollowersOfParams{FolloweeID: rep, RowLimit: 1})
	if err != nil || len(limited) != 1 {
		t.Errorf("NotifyFollowersOf with a limit of 1 gave %d rows (%v)", len(limited), err)
	}

	// Off: SetFollowNotify false and TurnOffFollowNotify.
	if set(f1, rep2, false, first) != 1 || notifyOn(f1, rep2) {
		t.Error("turning Notify me off with SetFollowNotify failed")
	}
	n, err := q.TurnOffFollowNotify(ctx, sqlcgen.TurnOffFollowNotifyParams{FollowerID: f2, FolloweeID: rep})
	if err != nil || n != 1 || notifyOn(f2, rep) {
		t.Errorf("TurnOffFollowNotify = %d, %v, on %v; want 1 row and off", n, err, notifyOn(f2, rep))
	}
	if n, _ := q.TurnOffFollowNotify(ctx, sqlcgen.TurnOffFollowNotifyParams{FollowerID: f2, FolloweeID: rep}); n != 0 {
		t.Errorf("TurnOffFollowNotify on an off bell changed %d rows, want 0", n)
	}

	// Ending a follow by EndFollowPair turns the bell off, every read stops
	// seeing it, and following again starts with the bell off (D-21).
	if endPair(t, q, f1, rep, "unfollowed", peopleNow.Add(2*time.Hour)) != 1 {
		t.Fatal("ending the follow failed")
	}
	if notifyOn(f1, rep) || count(f1) != 0 || len(followers(rep)) != 0 {
		t.Error("an ended follow must have no bell in any read")
	}
	if set(f1, rep, true, first) != 0 {
		t.Error("SetFollowNotify must not touch an ended follow")
	}
	var notify bool
	var since *time.Time
	if err := pool.QueryRow(ctx, `SELECT notify, notify_since FROM account_follows WHERE follower_id = $1 AND followee_id = $2`,
		f1, rep).Scan(&notify, &since); err != nil || notify || since != nil {
		t.Errorf("the ended row: notify %v, since %v (%v), want off with no time", notify, since, err)
	}
	if follow(t, q, f1, rep, peopleNow.Add(3*time.Hour)) != 1 || notifyOn(f1, rep) {
		t.Error("following again must start with the bell off")
	}

	// EndFollowsOf (Remove all followers, a switch off, a block) does the same.
	if set(f1, rep, true, first) != 1 || set(f2, rep, true, first) != 1 {
		t.Fatal("turning the bells on before EndFollowsOf failed")
	}
	ended, err := q.EndFollowsOf(ctx, sqlcgen.EndFollowsOfParams{FolloweeID: rep, Reason: "profile_off", Now: peopleNow.Add(4 * time.Hour)})
	if err != nil || ended != 2 {
		t.Fatalf("EndFollowsOf ended %d, %v; want 2", ended, err)
	}
	if notifyOn(f1, rep) || notifyOn(f2, rep) || len(followers(rep)) != 0 {
		t.Error("EndFollowsOf must turn every bell off")
	}
	var onRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM account_follows WHERE followee_id = $1 AND (notify OR notify_since IS NOT NULL)`,
		rep).Scan(&onRows); err != nil || onRows != 0 {
		t.Errorf("%d rows of rep still carry a bell after EndFollowsOf (%v)", onRows, err)
	}
	if got := count(f1); got != 0 {
		t.Errorf("CountNotifyFollows after the ends = %d, want 0", got)
	}
}
