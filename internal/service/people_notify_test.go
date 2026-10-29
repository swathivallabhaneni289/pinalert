package service_test

// people_notify_test.go proves the Notify me bell over a real database (plan
// 09-23, D-08, D-12, SC5, T-09-80, T-09-81, T-09-82): the rules of SetNotify, that
// muting turns the bell off, that an unfollow forgets the bell, and that the
// bell ends with every other way a follow ends. The tests skip without
// DATABASE_URL (NewTestDB). Every person is made up, with an example.com
// address.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// notifyRig is one database, a settable clock and a people service on it. The
// clock starts two hours before wall time, the same pattern as the sweep tests,
// so a reporter seeded at t0 reads Reliable reporter through that clock.
type notifyRig struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
	svc  *service.PeopleService
	t0   time.Time
	now  time.Time
}

func newNotifyRig(t *testing.T) *notifyRig {
	t.Helper()
	pool := testutil.NewTestDB(t)
	rig := &notifyRig{pool: pool, q: sqlcgen.New(pool), t0: time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)}
	rig.now = rig.t0
	rig.svc = service.NewPeopleService(rig.q, service.WithPeopleClock(func() time.Time { return rig.now }))
	return rig
}

// followee seeds a public Reliable reporter.
func (r *notifyRig) followee(t *testing.T, name string) int64 {
	t.Helper()
	return testutil.SeedPublicReporter(t, r.pool, name+"@example.com", name, name+"-session", r.t0)
}

// follower seeds a public person who follows the followee now.
func (r *notifyRig) follower(t *testing.T, name string, followee int64) int64 {
	t.Helper()
	id := testutil.SeedPerson(t, r.pool, name+"@example.com", name, true)
	testutil.SeedFollow(t, r.pool, id, followee, r.t0)
	return id
}

// bellRow reads the bell columns of one follow row.
func bellRow(t *testing.T, pool *pgxpool.Pool, follower, followee int64) (notify bool, since *time.Time) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		"SELECT notify, notify_since FROM account_follows WHERE follower_id = $1 AND followee_id = $2",
		follower, followee).Scan(&notify, &since)
	if err != nil {
		t.Fatalf("reading the bell of %d -> %d: %v", follower, followee, err)
	}
	return notify, since
}

// bellOnAndListed reports what the two send time reads say about one pair.
func (r *notifyRig) bellOnAndListed(t *testing.T, follower, followee int64) (on, listed bool) {
	t.Helper()
	ctx := context.Background()
	on, err := r.q.ActiveFollowNotify(ctx, sqlcgen.ActiveFollowNotifyParams{FollowerID: follower, FolloweeID: followee})
	if err != nil {
		t.Fatalf("ActiveFollowNotify: %v", err)
	}
	rows, err := r.q.NotifyFollowsFor(ctx, sqlcgen.NotifyFollowsForParams{FollowerID: follower, RowLimit: 100})
	if err != nil {
		t.Fatalf("NotifyFollowsFor: %v", err)
	}
	for _, row := range rows {
		if row.FolloweeID == followee {
			listed = true
		}
	}
	return on, listed
}

// TestSetNotifyRules: the answers of SetNotify (D-08, D-12, research 5.2).
func TestSetNotifyRules(t *testing.T) {
	rig := newNotifyRig(t)
	ctx := context.Background()

	ava := rig.followee(t, "ava.notify")
	me := rig.follower(t, "me.notify", ava)

	t.Run("a followed, unmuted, visible person can be switched on and off", func(t *testing.T) {
		if on, _ := bellRow(t, rig.pool, me, ava); on {
			t.Fatal("the bell is on before anyone asked; it is off by default")
		}
		if err := rig.svc.SetNotify(ctx, me, "ava.notify", true); err != nil {
			t.Fatalf("SetNotify on: %v", err)
		}
		on, since := bellRow(t, rig.pool, me, ava)
		if !on || since == nil {
			t.Fatalf("after on: notify=%v since=%v, want true and a time", on, since)
		}
		if !since.Equal(rig.now) {
			t.Errorf("notify_since = %v, want the call time %v", since, rig.now)
		}

		// A repeat keeps the first time, so only reports after the first "on" count.
		first := *since
		rig.now = rig.now.Add(30 * time.Minute)
		if err := rig.svc.SetNotify(ctx, me, "Ava.Notify", true); err != nil {
			t.Fatalf("a repeat SetNotify on (typed with capitals): %v", err)
		}
		if _, again := bellRow(t, rig.pool, me, ava); again == nil || !again.Equal(first) {
			t.Errorf("notify_since after a repeat = %v, want it kept at %v", again, first)
		}

		if err := rig.svc.SetNotify(ctx, me, "ava.notify", false); err != nil {
			t.Fatalf("SetNotify off: %v", err)
		}
		on, since = bellRow(t, rig.pool, me, ava)
		if on || since != nil {
			t.Errorf("after off: notify=%v since=%v, want false and no time", on, since)
		}
		if err := rig.svc.SetNotify(ctx, me, "ava.notify", false); err != nil {
			t.Errorf("a repeat SetNotify off = %v, want nil", err)
		}

		// Switching on again after off records a fresh time.
		if err := rig.svc.SetNotify(ctx, me, "ava.notify", true); err != nil {
			t.Fatalf("SetNotify on again: %v", err)
		}
		if _, again := bellRow(t, rig.pool, me, ava); again == nil || !again.Equal(rig.now) {
			t.Errorf("notify_since after on again = %v, want %v", again, rig.now)
		}
	})

	t.Run("the profile carries the viewer's own bell", func(t *testing.T) {
		if err := rig.svc.SetNotify(ctx, me, "ava.notify", true); err != nil {
			t.Fatalf("SetNotify on: %v", err)
		}
		p, err := rig.svc.Profile(ctx, me, "ava.notify")
		if err != nil {
			t.Fatalf("Profile: %v", err)
		}
		if !p.Following || !p.Notify {
			t.Errorf("Following=%v Notify=%v, want both true", p.Following, p.Notify)
		}
		if err := rig.svc.SetNotify(ctx, me, "ava.notify", false); err != nil {
			t.Fatalf("SetNotify off: %v", err)
		}
		p, err = rig.svc.Profile(ctx, me, "ava.notify")
		if err != nil {
			t.Fatalf("Profile: %v", err)
		}
		if p.Notify {
			t.Error("Notify is true after the bell was turned off")
		}
		// The bell is the viewer's own: someone else sees nothing of it.
		other := testutil.SeedPerson(t, rig.pool, "other.notify@example.com", "other.notify", true)
		if err := rig.svc.SetNotify(ctx, me, "ava.notify", true); err != nil {
			t.Fatalf("SetNotify on: %v", err)
		}
		p, err = rig.svc.Profile(ctx, other, "ava.notify")
		if err != nil {
			t.Fatalf("Profile as someone else: %v", err)
		}
		if p.Notify {
			t.Error("another viewer sees Notify true; the bell is the viewer's own")
		}
		own, err := rig.svc.Profile(ctx, ava, "ava.notify")
		if err != nil {
			t.Fatalf("Profile of your own page: %v", err)
		}
		if own.Notify {
			t.Error("Notify is true on your own page")
		}
		if err := rig.svc.SetNotify(ctx, me, "ava.notify", false); err != nil {
			t.Fatalf("SetNotify off: %v", err)
		}
	})

	t.Run("not following, muted, blocked or yourself is unavailable", func(t *testing.T) {
		stranger := rig.followee(t, "stranger.notify")
		if err := rig.svc.SetNotify(ctx, me, "stranger.notify", true); !errors.Is(err, service.ErrNotifyUnavailable) {
			t.Errorf("SetNotify on without following = %v, want ErrNotifyUnavailable", err)
		}
		if err := rig.svc.SetNotify(ctx, me, "stranger.notify", false); !errors.Is(err, service.ErrNotifyUnavailable) {
			t.Errorf("SetNotify off without following = %v, want ErrNotifyUnavailable", err)
		}
		if err := rig.svc.SetNotify(ctx, me, "me.notify", true); !errors.Is(err, service.ErrNotifyUnavailable) {
			t.Errorf("SetNotify on for yourself = %v, want ErrNotifyUnavailable", err)
		}

		testutil.SeedMute(t, rig.pool, me, ava)
		if err := rig.svc.SetNotify(ctx, me, "ava.notify", true); !errors.Is(err, service.ErrNotifyUnavailable) {
			t.Errorf("SetNotify on for a muted person = %v, want ErrNotifyUnavailable", err)
		}
		if on, _ := bellRow(t, rig.pool, me, ava); on {
			t.Error("a refused SetNotify on a muted person left the bell on")
		}
		if err := rig.svc.Unmute(ctx, me, "ava.notify"); err != nil {
			t.Fatalf("Unmute: %v", err)
		}
		if err := rig.svc.SetNotify(ctx, me, "ava.notify", true); err != nil {
			t.Errorf("SetNotify on after Unmute = %v, want nil", err)
		}
		if err := rig.svc.SetNotify(ctx, me, "ava.notify", false); err != nil {
			t.Fatalf("SetNotify off: %v", err)
		}

		testutil.SeedBlock(t, rig.pool, me, stranger)
		testutil.SeedFollow(t, rig.pool, me, stranger, rig.t0)
		if err := rig.svc.SetNotify(ctx, me, "stranger.notify", true); !errors.Is(err, service.ErrNotifyUnavailable) {
			t.Errorf("SetNotify on for a person you blocked = %v, want ErrNotifyUnavailable", err)
		}
	})

	t.Run("a person you may not see is the uniform not found", func(t *testing.T) {
		closed := testutil.SeedPerson(t, rig.pool, "closed.notify@example.com", "closed.notify", false)
		pending := seedNamedAccount(t, rig.pool, "pending.notify@example.com", "pend.notify", "pending", nil)
		testutil.MustExec(t, rig.pool, "UPDATE accounts SET profile_public = true, profile_public_at = now() WHERE id = $1", pending)
		blocker := testutil.SeedPerson(t, rig.pool, "blocker.notify@example.com", "blocker.notify", true)
		testutil.SeedBlock(t, rig.pool, blocker, me)
		for _, id := range []int64{closed, pending, blocker} {
			testutil.SeedFollow(t, rig.pool, me, id, rig.t0)
		}
		for _, c := range []struct{ name, username string }{
			{"a private profile", "closed.notify"},
			{"a pending username", "pend.notify"},
			{"a person who blocked you", "blocker.notify"},
			{"an unknown name", "nobody.notify"},
			{"a malformed name", "no"},
			{"an empty name", ""},
		} {
			if err := rig.svc.SetNotify(ctx, me, c.username, true); !errors.Is(err, service.ErrPersonNotFound) {
				t.Errorf("SetNotify on for %s = %v, want ErrPersonNotFound", c.name, err)
			}
		}
		for _, id := range []int64{closed, pending, blocker} {
			if on, _ := bellRow(t, rig.pool, me, id); on {
				t.Errorf("a refused SetNotify left the bell on for account %d", id)
			}
		}
	})

	t.Run("the cap refuses the 51st bell and a repeat is never refused", func(t *testing.T) {
		capper := testutil.SeedPerson(t, rig.pool, "capper@example.com", "capper.notify", true)
		ids := seedBulkPeople(t, rig.pool, "capped", service.NotifyFollowsCap+1, true)
		for _, id := range ids {
			testutil.SeedFollow(t, rig.pool, capper, id, rig.t0)
		}
		for i := 0; i < service.NotifyFollowsCap; i++ {
			name := fmt.Sprintf("capped%03d", i+1)
			if err := rig.svc.SetNotify(ctx, capper, name, true); err != nil {
				t.Fatalf("bell %d (%s): %v", i+1, name, err)
			}
		}
		extra := fmt.Sprintf("capped%03d", service.NotifyFollowsCap+1)
		err := rig.svc.SetNotify(ctx, capper, extra, true)
		if !errors.Is(err, service.ErrNotifyCap) {
			t.Fatalf("the %dth bell = %v, want ErrNotifyCap", service.NotifyFollowsCap+1, err)
		}
		if on, _ := bellRow(t, rig.pool, capper, ids[service.NotifyFollowsCap]); on {
			t.Error("a refused bell was written")
		}
		if err := rig.svc.SetNotify(ctx, capper, "capped001", true); err != nil {
			t.Errorf("a repeat on for a bell already on at the cap = %v, want nil", err)
		}
		if err := rig.svc.SetNotify(ctx, capper, "capped001", false); err != nil {
			t.Fatalf("turning one off: %v", err)
		}
		if err := rig.svc.SetNotify(ctx, capper, extra, true); err != nil {
			t.Errorf("a bell after one was turned off = %v, want nil", err)
		}
		if got := service.NotifyCapMessage; got != "You can turn on Notify me for up to 50 people." {
			t.Errorf("NotifyCapMessage = %q", got)
		}
		if got := service.NotifyUnavailableMessage; got != "Couldn't change this. Try again." {
			t.Errorf("NotifyUnavailableMessage = %q", got)
		}
	})
}

// TestMuteTurnsBellOff: muting turns the bell off, keeps the follow, and
// Unmute leaves the bell off (D-12, UI-SPEC 1e).
func TestMuteTurnsBellOff(t *testing.T) {
	rig := newNotifyRig(t)
	ctx := context.Background()

	ava := rig.followee(t, "ava.mutebell")
	me := rig.follower(t, "me.mutebell", ava)

	if err := rig.svc.SetNotify(ctx, me, "ava.mutebell", true); err != nil {
		t.Fatalf("SetNotify on: %v", err)
	}
	if err := rig.svc.Mute(ctx, me, "ava.mutebell"); err != nil {
		t.Fatalf("Mute: %v", err)
	}
	on, since := bellRow(t, rig.pool, me, ava)
	if on || since != nil {
		t.Errorf("after Mute: notify=%v since=%v, want false and no time", on, since)
	}
	if !activeFollows(t, rig.pool, me, ava) {
		t.Error("muting ended the follow; only the bell goes")
	}
	p, err := rig.svc.Profile(ctx, me, "ava.mutebell")
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if !p.Muted || !p.Following || p.Notify {
		t.Errorf("Muted=%v Following=%v Notify=%v, want true, true, false", p.Muted, p.Following, p.Notify)
	}

	if err := rig.svc.Unmute(ctx, me, "ava.mutebell"); err != nil {
		t.Fatalf("Unmute: %v", err)
	}
	if on, _ := bellRow(t, rig.pool, me, ava); on {
		t.Error("Unmute brought the bell back; it stays off until the person turns it on")
	}
	p, err = rig.svc.Profile(ctx, me, "ava.mutebell")
	if err != nil {
		t.Fatalf("Profile after Unmute: %v", err)
	}
	if p.Notify {
		t.Error("Notify is true after Unmute")
	}
	if err := rig.svc.SetNotify(ctx, me, "ava.mutebell", true); err != nil {
		t.Errorf("SetNotify on after Unmute = %v, want nil", err)
	}

	t.Run("muting a person with no follow or a bell already off changes nothing else", func(t *testing.T) {
		stranger := rig.followee(t, "stranger.mutebell")
		if err := rig.svc.Mute(ctx, me, "stranger.mutebell"); err != nil {
			t.Errorf("Mute of someone not followed = %v, want nil", err)
		}
		if n := muteRows(t, rig.pool, me, stranger); n != 1 {
			t.Errorf("mute rows = %d, want 1", n)
		}
		// Muting one person leaves every other bell alone.
		bob := rig.followee(t, "bob.mutebell")
		testutil.SeedFollow(t, rig.pool, me, bob, rig.t0)
		if err := rig.svc.SetNotify(ctx, me, "bob.mutebell", true); err != nil {
			t.Fatalf("SetNotify on: %v", err)
		}
		if err := rig.svc.Mute(ctx, me, "ava.mutebell"); err != nil {
			t.Fatalf("Mute: %v", err)
		}
		if on, _ := bellRow(t, rig.pool, me, bob); !on {
			t.Error("muting one person turned another person's bell off")
		}
	})
}

// TestUnfollowForgetsBell: an unfollow clears the bell, and following again
// starts with it off (D-12, D-21).
func TestUnfollowForgetsBell(t *testing.T) {
	rig := newNotifyRig(t)
	ctx := context.Background()

	ava := rig.followee(t, "ava.forget")
	me := testutil.SeedPerson(t, rig.pool, "me.forget@example.com", "me.forget", true)

	if err := rig.svc.Follow(ctx, me, "ava.forget"); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	if on, _ := bellRow(t, rig.pool, me, ava); on {
		t.Fatal("a new follow starts with the bell on")
	}
	if err := rig.svc.SetNotify(ctx, me, "ava.forget", true); err != nil {
		t.Fatalf("SetNotify on: %v", err)
	}
	if err := rig.svc.Unfollow(ctx, me, "ava.forget"); err != nil {
		t.Fatalf("Unfollow: %v", err)
	}
	on, since := bellRow(t, rig.pool, me, ava)
	if on || since != nil {
		t.Errorf("after Unfollow: notify=%v since=%v, want false and no time", on, since)
	}
	if err := rig.svc.SetNotify(ctx, me, "ava.forget", true); !errors.Is(err, service.ErrNotifyUnavailable) {
		t.Errorf("SetNotify on after Unfollow = %v, want ErrNotifyUnavailable", err)
	}

	if err := rig.svc.Follow(ctx, me, "ava.forget"); err != nil {
		t.Fatalf("Follow again: %v", err)
	}
	if on, since := bellRow(t, rig.pool, me, ava); on || since != nil {
		t.Errorf("after following again: notify=%v since=%v, want false and no time", on, since)
	}
	p, err := rig.svc.Profile(ctx, me, "ava.forget")
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if !p.Following || p.Notify {
		t.Errorf("Following=%v Notify=%v, want true and false", p.Following, p.Notify)
	}
}

// TestBellEndsWithEveryFollowEnd: every way a follow ends takes the bell with
// it (review W15, D-12, SC5). After each, ActiveFollowNotify answers false and
// NotifyFollowsFor no longer lists the followee.
func TestBellEndsWithEveryFollowEnd(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		end  func(t *testing.T, rig *notifyRig, follower, followee int64, followerName, followeeName string)
	}{
		{"unfollow", func(t *testing.T, rig *notifyRig, follower, followee int64, fn, en string) {
			if err := rig.svc.Unfollow(ctx, follower, en); err != nil {
				t.Fatalf("Unfollow: %v", err)
			}
		}},
		{"block by the follower", func(t *testing.T, rig *notifyRig, follower, followee int64, fn, en string) {
			if err := rig.svc.Block(ctx, follower, en); err != nil {
				t.Fatalf("Block: %v", err)
			}
		}},
		{"block by the followee", func(t *testing.T, rig *notifyRig, follower, followee int64, fn, en string) {
			if err := rig.svc.Block(ctx, followee, fn); err != nil {
				t.Fatalf("Block: %v", err)
			}
		}},
		{"remove follower", func(t *testing.T, rig *notifyRig, follower, followee int64, fn, en string) {
			if err := rig.svc.RemoveFollower(ctx, followee, fn); err != nil {
				t.Fatalf("RemoveFollower: %v", err)
			}
		}},
		{"remove all followers", func(t *testing.T, rig *notifyRig, follower, followee int64, fn, en string) {
			if _, err := rig.svc.RemoveAllFollowers(ctx, followee); err != nil {
				t.Fatalf("RemoveAllFollowers: %v", err)
			}
		}},
		{"the followee switches the profile off", func(t *testing.T, rig *notifyRig, follower, followee int64, fn, en string) {
			if err := rig.svc.SetProfilePublic(ctx, followee, false); err != nil {
				t.Fatalf("SetProfilePublic off: %v", err)
			}
		}},
		{"the sweep ends a tag_lost follow", func(t *testing.T, rig *notifyRig, follower, followee int64, fn, en string) {
			// 120 days on, the judged history has left the 90 day window.
			later := rig.t0.Add(120 * 24 * time.Hour)
			sweep := service.NewFollowSweep(rig.q, service.WithFollowSweepClock(func() time.Time { return later }))
			ended, err := sweep.SweepOnce(ctx)
			if err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			if ended != 1 {
				t.Fatalf("the sweep ended %d follows, want 1", ended)
			}
		}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newNotifyRig(t)
			followeeName := fmt.Sprintf("ee.end%d", i)
			followerName := fmt.Sprintf("er.end%d", i)
			followee := rig.followee(t, followeeName)
			follower := rig.follower(t, followerName, followee)

			if err := rig.svc.SetNotify(ctx, follower, followeeName, true); err != nil {
				t.Fatalf("SetNotify on: %v", err)
			}
			if on, listed := rig.bellOnAndListed(t, follower, followee); !on || !listed {
				t.Fatalf("before the end: on=%v listed=%v, want both true", on, listed)
			}

			tc.end(t, rig, follower, followee, followerName, followeeName)

			if on, listed := rig.bellOnAndListed(t, follower, followee); on || listed {
				t.Errorf("after the follow ended: ActiveFollowNotify=%v NotifyFollowsFor lists=%v, want both false", on, listed)
			}
			if activeFollows(t, rig.pool, follower, followee) {
				t.Error("the follow is still active")
			}
			if on, since := bellRow(t, rig.pool, follower, followee); on || since != nil {
				t.Errorf("the ended row keeps notify=%v since=%v, want false and no time", on, since)
			}
		})
	}
}
