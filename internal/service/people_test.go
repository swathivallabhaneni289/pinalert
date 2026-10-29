package service_test

// people_test.go proves PeopleService over a real database (plan 09-04, D-01,
// D-03, D-12, D-21): the opt in rules, follow, unfollow, remove follower,
// block, the people lists and the Blocked people list. The tests skip without
// DATABASE_URL (NewTestDB).

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

// screenFn is a TextScreener scripted by the text it is asked about.
type screenFn func(text string) (moderation.Result, error)

func (f screenFn) Screen(_ context.Context, text string) (moderation.Result, error) { return f(text) }

var (
	screenAllClear = screenFn(func(string) (moderation.Result, error) {
		return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{}}, nil
	})
	screenAllRejected = screenFn(func(string) (moderation.Result, error) {
		return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{"hate": service.RejectScoreThreshold}}, nil
	})
	screenAllBorderline = screenFn(func(string) (moderation.Result, error) {
		return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{"hate": service.BorderlineScoreThreshold}}, nil
	})
	screenDown = screenFn(func(string) (moderation.Result, error) {
		return moderation.Result{}, errors.New("provider down")
	})
)

func newPeople(t *testing.T, opts ...service.PeopleOption) (*pgxpool.Pool, *service.PeopleService) {
	t.Helper()
	pool := testutil.NewTestDB(t)
	return pool, service.NewPeopleService(sqlcgen.New(pool), opts...)
}

// seedNamedAccount makes an account with a username in the given screen state
// and a Name, the profile switch off.
func seedNamedAccount(t *testing.T, pool *pgxpool.Pool, email, username, screen string, name *string) int64 {
	t.Helper()
	id := testutil.SeedReporterAccount(t, pool, email)
	if username != "" {
		testutil.SeedUsername(t, pool, id, username, screen)
	}
	if name != nil {
		if _, err := pool.Exec(context.Background(), "UPDATE accounts SET display_name = $2 WHERE id = $1", id, *name); err != nil {
			t.Fatalf("seeding a name: %v", err)
		}
	}
	return id
}

func ptr(s string) *string { return &s }

func isPublic(t *testing.T, pool *pgxpool.Pool, id int64) bool {
	t.Helper()
	var on bool
	if err := pool.QueryRow(context.Background(), "SELECT profile_public FROM accounts WHERE id = $1", id).Scan(&on); err != nil {
		t.Fatalf("reading profile_public: %v", err)
	}
	return on
}

// followRow reads one follow row: whether it exists, is ended, and its reason.
func followRow(t *testing.T, pool *pgxpool.Pool, follower, followee int64) (exists, ended bool, reason string) {
	t.Helper()
	var r *string
	err := pool.QueryRow(context.Background(),
		"SELECT ended_at IS NOT NULL, end_reason FROM account_follows WHERE follower_id = $1 AND followee_id = $2",
		follower, followee).Scan(&ended, &r)
	if err != nil {
		return false, false, ""
	}
	if r != nil {
		reason = *r
	}
	return true, ended, reason
}

func activeFollows(t *testing.T, pool *pgxpool.Pool, follower, followee int64) bool {
	t.Helper()
	exists, ended, _ := followRow(t, pool, follower, followee)
	return exists && !ended
}

// seedBulkPeople inserts n public accounts with clear usernames prefix001 and
// up, in one statement, and returns their ids in order.
func seedBulkPeople(t *testing.T, pool *pgxpool.Pool, prefix string, n int, public bool) []int64 {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`INSERT INTO accounts (email, username, username_screen, profile_public, profile_public_at)
		 SELECT $1::text || g::text || '@example.com', $1::text || lpad(g::text, 3, '0'), 'clear', $2::boolean,
		        CASE WHEN $2::boolean THEN now() END
		 FROM generate_series(1, $3::int) g
		 ORDER BY g
		 RETURNING id`, prefix, public, n)
	if err != nil {
		t.Fatalf("bulk seeding %s: %v", prefix, err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scanning a bulk id: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("bulk seeding %s: %v", prefix, err)
	}
	return ids
}

func TestProfileOptInRules(t *testing.T) {
	ctx := context.Background()

	t.Run("no_username_refused", func(t *testing.T) {
		pool, svc := newPeople(t)
		id := seedNamedAccount(t, pool, "optin-nouser@example.com", "", "", ptr("Asha Rao"))
		wantValidation(t, svc.SetProfilePublic(ctx, id, true), "profile_public", service.PublicUsernameRefusedMessage)
		if isPublic(t, pool, id) {
			t.Fatal("the switch turned on without a username")
		}
	})
	t.Run("pending_username_clear_is_marked_and_switch_turns_on", func(t *testing.T) {
		pool, svc := newPeople(t, service.WithPeopleScreener(screenAllClear))
		id := seedNamedAccount(t, pool, "optin-pending@example.com", "asha.rao", "pending", ptr("Asha Rao"))
		if err := svc.SetProfilePublic(ctx, id, true); err != nil {
			t.Fatalf("SetProfilePublic: %v", err)
		}
		var screen string
		if err := pool.QueryRow(ctx, "SELECT username_screen FROM accounts WHERE id = $1", id).Scan(&screen); err != nil || screen != "clear" {
			t.Fatalf("username_screen = %q (%v), want clear", screen, err)
		}
		if !isPublic(t, pool, id) {
			t.Fatal("the switch did not turn on")
		}
	})
	t.Run("pending_username_borderline_and_rejected_refuse", func(t *testing.T) {
		for name, sc := range map[string]service.TextScreener{"borderline": screenAllBorderline, "rejected": screenAllRejected} {
			pool, svc := newPeople(t, service.WithPeopleScreener(sc))
			id := seedNamedAccount(t, pool, "optin-"+name+"@example.com", "asha.rao", "pending", ptr("Asha Rao"))
			wantValidation(t, svc.SetProfilePublic(ctx, id, true), "profile_public", service.PublicUsernameRefusedMessage)
			if isPublic(t, pool, id) {
				t.Fatalf("%s: the switch turned on", name)
			}
		}
	})
	t.Run("unfinished_check_refuses", func(t *testing.T) {
		pool, svc := newPeople(t, service.WithPeopleScreener(screenDown))
		pending := seedNamedAccount(t, pool, "optin-down-a@example.com", "asha.rao", "pending", ptr("Asha Rao"))
		wantValidation(t, svc.SetProfilePublic(ctx, pending, true), "profile_public", service.PublicNameUnfinishedMessage)
		clearName := seedNamedAccount(t, pool, "optin-down-b@example.com", "asha.two", "clear", ptr("Asha Rao"))
		wantValidation(t, svc.SetProfilePublic(ctx, clearName, true), "profile_public", service.PublicNameUnfinishedMessage)
		if isPublic(t, pool, pending) || isPublic(t, pool, clearName) {
			t.Fatal("a check that could not finish turned the switch on")
		}
	})
	t.Run("name_refused", func(t *testing.T) {
		pool, svc := newPeople(t, service.WithPeopleScreener(screenAllClear))
		reserved := seedNamedAccount(t, pool, "optin-res@example.com", "asha.one", "clear", ptr("NDMA Control Room"))
		wantValidation(t, svc.SetProfilePublic(ctx, reserved, true), "profile_public", service.PublicNameRefusedMessage)
		noName := seedNamedAccount(t, pool, "optin-noname@example.com", "asha.two", "clear", nil)
		wantValidation(t, svc.SetProfilePublic(ctx, noName, true), "profile_public", service.PublicNameRefusedMessage)

		rejecting := service.NewPeopleService(sqlcgen.New(pool), service.WithPeopleScreener(screenAllRejected))
		rejected := seedNamedAccount(t, pool, "optin-rej@example.com", "asha.three", "clear", ptr("Asha Rao"))
		wantValidation(t, rejecting.SetProfilePublic(ctx, rejected, true), "profile_public", service.PublicNameRefusedMessage)
		for _, id := range []int64{reserved, noName, rejected} {
			if isPublic(t, pool, id) {
				t.Fatalf("account %d turned on with a refused name", id)
			}
		}
	})
	t.Run("clear_name_turns_on_and_repeat_is_quiet", func(t *testing.T) {
		pool, svc := newPeople(t, service.WithPeopleScreener(screenAllClear))
		id := seedNamedAccount(t, pool, "optin-ok@example.com", "asha.rao", "clear", ptr("Asha Rao"))
		if err := svc.SetProfilePublic(ctx, id, true); err != nil {
			t.Fatalf("SetProfilePublic: %v", err)
		}
		if !isPublic(t, pool, id) {
			t.Fatal("the switch did not turn on")
		}
		if err := svc.SetProfilePublic(ctx, id, true); err != nil {
			t.Fatalf("a repeat SetProfilePublic: %v", err)
		}
	})
}

func TestOptOutEndsFollowers(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	now := time.Now()

	owner := testutil.SeedPublicReporter(t, pool, "optout-owner@example.com", "optout.owner", "optout-owner-sess", now)
	other := testutil.SeedPublicReporter(t, pool, "optout-other@example.com", "optout.other", "optout-other-sess", now)
	f1 := testutil.SeedPerson(t, pool, "optout-f1@example.com", "optout.one", false)
	f2 := testutil.SeedPerson(t, pool, "optout-f2@example.com", "optout.two", false)
	testutil.SeedFollow(t, pool, f1, owner, now)
	testutil.SeedFollow(t, pool, f2, owner, now)
	testutil.SeedFollow(t, pool, owner, other, now) // the owner's own Following row

	if err := svc.SetProfilePublic(ctx, owner, false); err != nil {
		t.Fatalf("switching off: %v", err)
	}
	if isPublic(t, pool, owner) {
		t.Fatal("the profile is still on")
	}
	for _, f := range []int64{f1, f2} {
		exists, ended, reason := followRow(t, pool, f, owner)
		if !exists || !ended || reason != "profile_off" {
			t.Errorf("follow %d -> owner: exists %v ended %v reason %q, want a row ended with profile_off", f, exists, ended, reason)
		}
	}
	if !activeFollows(t, pool, owner, other) {
		t.Error("switching off ended the owner's own Following row")
	}

	if err := svc.SetProfilePublic(ctx, owner, true); err != nil {
		t.Fatalf("switching on again: %v", err)
	}
	counts, err := svc.Counts(ctx, owner)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.Followers != 0 || counts.Following != 1 {
		t.Errorf("after off and on: followers %d following %d, want 0 and 1", counts.Followers, counts.Following)
	}
}

func TestFollowRules(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	now := time.Now()

	reliable := testutil.SeedPublicReporter(t, pool, "follow-rel@example.com", "follow.rel", "follow-rel-sess", now)
	viewer := testutil.SeedPerson(t, pool, "follow-viewer@example.com", "follow.viewer", false)

	t.Run("follow_is_idempotent", func(t *testing.T) {
		if err := svc.Follow(ctx, viewer, "follow.rel"); err != nil {
			t.Fatalf("Follow: %v", err)
		}
		if err := svc.Follow(ctx, viewer, "@Follow.Rel"); err != nil {
			t.Fatalf("a repeat Follow with a typed @ and capitals: %v", err)
		}
		var n int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM account_follows WHERE follower_id = $1 AND followee_id = $2", viewer, reliable).Scan(&n); err != nil || n != 1 {
			t.Fatalf("follow rows = %d (%v), want 1", n, err)
		}
	})

	t.Run("not_found_for_every_invisible_person", func(t *testing.T) {
		testutil.SeedPerson(t, pool, "follow-priv@example.com", "follow.priv", false)
		pending := testutil.SeedPublicReporter(t, pool, "follow-pend@example.com", "follow.pend", "follow-pend-sess", now)
		testutil.SeedUsername(t, pool, pending, "follow.pend", "pending")
		deleted := testutil.SeedPublicReporter(t, pool, "follow-del@example.com", "follow.del", "follow-del-sess", now)
		if _, err := pool.Exec(ctx, "UPDATE accounts SET deleted_at = now() WHERE id = $1", deleted); err != nil {
			t.Fatalf("deleting: %v", err)
		}
		blocking := testutil.SeedPublicReporter(t, pool, "follow-blk@example.com", "follow.blk", "follow-blk-sess", now)
		testutil.SeedBlock(t, pool, blocking, viewer)

		for _, name := range []string{"follow.nobody", "follow.priv", "follow.pend", "follow.del", "follow.blk", "ab", "bad name", ""} {
			if err := svc.Follow(ctx, viewer, name); !errors.Is(err, service.ErrPersonNotFound) {
				t.Errorf("Follow(%q) = %v, want ErrPersonNotFound", name, err)
			}
		}
	})

	t.Run("cannot_follow", func(t *testing.T) {
		if err := svc.Follow(ctx, reliable, "follow.rel"); !errors.Is(err, service.ErrCannotFollow) {
			t.Errorf("following yourself = %v, want ErrCannotFollow", err)
		}
		testutil.SeedPerson(t, pool, "follow-notag@example.com", "follow.notag", true)
		if err := svc.Follow(ctx, viewer, "follow.notag"); !errors.Is(err, service.ErrCannotFollow) {
			t.Errorf("following a person without the tag = %v, want ErrCannotFollow", err)
		}
		youBlocked := testutil.SeedPublicReporter(t, pool, "follow-yb@example.com", "follow.yb", "follow-yb-sess", now)
		testutil.SeedBlock(t, pool, viewer, youBlocked)
		if err := svc.Follow(ctx, viewer, "follow.yb"); !errors.Is(err, service.ErrCannotFollow) {
			t.Errorf("following a person you blocked = %v, want ErrCannotFollow", err)
		}
	})

	t.Run("cap", func(t *testing.T) {
		capped := testutil.SeedPerson(t, pool, "follow-capped@example.com", "follow.capped", false)
		testutil.SeedFollow(t, pool, capped, reliable, now)
		fillers := seedBulkPeople(t, pool, "capfill", service.FollowingCap-1, true)
		for _, id := range fillers {
			testutil.SeedFollow(t, pool, capped, id, now)
		}
		testutil.SeedPublicReporter(t, pool, "follow-extra@example.com", "follow.extra", "follow-extra-sess", now)
		if err := svc.Follow(ctx, capped, "follow.extra"); !errors.Is(err, service.ErrFollowingCap) {
			t.Errorf("the 201st follow = %v, want ErrFollowingCap", err)
		}
		if err := svc.Follow(ctx, capped, "follow.rel"); err != nil {
			t.Errorf("following someone already followed at the cap = %v, want nil", err)
		}
		if got := service.FollowingCapMessage; got != "You can follow up to 200 people." {
			t.Errorf("FollowingCapMessage = %q", got)
		}
	})

	t.Run("unfollow_and_refollow", func(t *testing.T) {
		if err := svc.Unfollow(ctx, viewer, "follow.rel"); err != nil {
			t.Fatalf("Unfollow: %v", err)
		}
		exists, ended, reason := followRow(t, pool, viewer, reliable)
		if !exists || !ended || reason != "unfollowed" {
			t.Fatalf("after Unfollow: exists %v ended %v reason %q, want a row ended with unfollowed", exists, ended, reason)
		}
		if err := svc.Follow(ctx, viewer, "follow.rel"); err != nil {
			t.Fatalf("following again: %v", err)
		}
		if !activeFollows(t, pool, viewer, reliable) {
			t.Fatal("the follow was not restored")
		}
		// An unknown but well formed username succeeds and changes nothing; a
		// malformed one is not found.
		if err := svc.Unfollow(ctx, viewer, "follow.nobody"); err != nil {
			t.Errorf("Unfollow of nobody = %v, want nil", err)
		}
		if err := svc.Unfollow(ctx, viewer, "no"); !errors.Is(err, service.ErrPersonNotFound) {
			t.Errorf("Unfollow of a malformed name = %v, want ErrPersonNotFound", err)
		}
		// Unfollow works after the person went private.
		if _, err := pool.Exec(ctx, "UPDATE accounts SET profile_public = false WHERE id = $1", reliable); err != nil {
			t.Fatalf("going private: %v", err)
		}
		if err := svc.Unfollow(ctx, viewer, "follow.rel"); err != nil {
			t.Fatalf("Unfollow of a private person: %v", err)
		}
		if activeFollows(t, pool, viewer, reliable) {
			t.Fatal("the follow of a private person was not ended")
		}
	})
}

func TestBlockEndsFollowsBothWays(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	now := time.Now()

	a := testutil.SeedPublicReporter(t, pool, "blk-a@example.com", "blk.alpha", "blk-a-sess", now)
	b := testutil.SeedPublicReporter(t, pool, "blk-b@example.com", "blk.bravo", "blk-b-sess", now)
	testutil.SeedFollow(t, pool, a, b, now)
	testutil.SeedFollow(t, pool, b, a, now)

	if err := svc.Block(ctx, a, "blk.bravo"); err != nil {
		t.Fatalf("Block: %v", err)
	}
	for _, pair := range [][2]int64{{a, b}, {b, a}} {
		exists, ended, reason := followRow(t, pool, pair[0], pair[1])
		if !exists || !ended || reason != "blocked" {
			t.Errorf("follow %d -> %d: exists %v ended %v reason %q, want a row ended with blocked", pair[0], pair[1], exists, ended, reason)
		}
	}
	var blocks int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM person_blocks WHERE blocker_id = $1 AND blocked_id = $2", a, b).Scan(&blocks); err != nil || blocks != 1 {
		t.Fatalf("block rows = %d (%v), want 1", blocks, err)
	}
	if err := svc.Block(ctx, a, "blk.bravo"); err != nil {
		t.Errorf("a repeat Block = %v, want nil", err)
	}
	// Neither can follow the other now.
	if err := svc.Follow(ctx, a, "blk.bravo"); !errors.Is(err, service.ErrCannotFollow) {
		t.Errorf("the blocker following = %v, want ErrCannotFollow", err)
	}
	if err := svc.Follow(ctx, b, "blk.alpha"); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("the blocked person following = %v, want ErrPersonNotFound", err)
	}
	// And the blocked person cannot block back: the blocker is not visible to them.
	if err := svc.Block(ctx, b, "blk.alpha"); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("the blocked person blocking back = %v, want ErrPersonNotFound", err)
	}

	// Refusals of Block.
	if err := svc.Block(ctx, a, "blk.alpha"); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("blocking yourself = %v, want ErrPersonNotFound", err)
	}
	testutil.SeedPerson(t, pool, "blk-priv@example.com", "blk.private", false)
	for _, name := range []string{"blk.private", "blk.nobody", "no"} {
		if err := svc.Block(ctx, a, name); !errors.Is(err, service.ErrPersonNotFound) {
			t.Errorf("Block(%q) = %v, want ErrPersonNotFound", name, err)
		}
	}

	// Unblock removes the block row and leaves the follows ended.
	if err := svc.Unblock(ctx, a, "blk.bravo"); err != nil {
		t.Fatalf("Unblock: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM person_blocks WHERE blocker_id = $1 AND blocked_id = $2", a, b).Scan(&blocks); err != nil || blocks != 0 {
		t.Fatalf("block rows after Unblock = %d (%v), want 0", blocks, err)
	}
	if activeFollows(t, pool, a, b) || activeFollows(t, pool, b, a) {
		t.Error("Unblock restored a follow the block ended")
	}
	if err := svc.Unblock(ctx, a, "blk.nobody"); err != nil {
		t.Errorf("Unblock of nobody = %v, want nil", err)
	}
	if err := svc.Unblock(ctx, a, "!"); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Unblock of a malformed name = %v, want ErrPersonNotFound", err)
	}
}

func TestRemoveFollower(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	now := time.Now()

	owner := testutil.SeedPublicReporter(t, pool, "rmf-owner@example.com", "rmf.owner", "rmf-owner-sess", now)
	f1 := testutil.SeedPerson(t, pool, "rmf-f1@example.com", "rmf.one", false) // private: removal still works
	f2 := testutil.SeedPerson(t, pool, "rmf-f2@example.com", "rmf.two", true)
	testutil.SeedFollow(t, pool, f1, owner, now)
	testutil.SeedFollow(t, pool, f2, owner, now)

	if err := svc.RemoveFollower(ctx, owner, "rmf.one"); err != nil {
		t.Fatalf("RemoveFollower: %v", err)
	}
	exists, ended, reason := followRow(t, pool, f1, owner)
	if !exists || !ended || reason != "removed" {
		t.Fatalf("after RemoveFollower: exists %v ended %v reason %q, want a row ended with removed", exists, ended, reason)
	}
	if !activeFollows(t, pool, f2, owner) {
		t.Error("RemoveFollower ended another follower")
	}
	if err := svc.RemoveFollower(ctx, owner, "rmf.one"); err != nil {
		t.Errorf("a repeat RemoveFollower = %v, want nil", err)
	}
	if err := svc.RemoveFollower(ctx, owner, "rmf.nobody"); err != nil {
		t.Errorf("RemoveFollower of nobody = %v, want nil", err)
	}
	if err := svc.RemoveFollower(ctx, owner, "xy"); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("RemoveFollower of a malformed name = %v, want ErrPersonNotFound", err)
	}
	// Removing a person who does not follow you changes nothing.
	if err := svc.RemoveFollower(ctx, f2, "rmf.owner"); err != nil {
		t.Errorf("RemoveFollower of a non follower = %v, want nil", err)
	}
	if !activeFollows(t, pool, f2, owner) {
		t.Error("a follower removing the owner ended the follow of someone else")
	}
}

func TestRemoveAllFollowers(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	now := time.Now()

	owner := testutil.SeedPublicReporter(t, pool, "rma-owner@example.com", "rma.owner", "rma-owner-sess", now)
	other := testutil.SeedPublicReporter(t, pool, "rma-other@example.com", "rma.other", "rma-other-sess", now)
	var followers []int64
	for i := 0; i < 3; i++ {
		f := testutil.SeedPerson(t, pool, fmt.Sprintf("rma-f%d@example.com", i), fmt.Sprintf("rma.f%d", i), false)
		testutil.SeedFollow(t, pool, f, owner, now)
		followers = append(followers, f)
	}
	testutil.SeedFollow(t, pool, owner, other, now)
	testutil.SeedFollow(t, pool, followers[0], other, now)

	n, err := svc.RemoveAllFollowers(ctx, owner)
	if err != nil || n != 3 {
		t.Fatalf("RemoveAllFollowers = %d (%v), want 3", n, err)
	}
	for _, f := range followers {
		if _, ended, reason := followRow(t, pool, f, owner); !ended || reason != "removed" {
			t.Errorf("follower %d: ended %v reason %q, want ended with removed", f, ended, reason)
		}
	}
	if !activeFollows(t, pool, owner, other) || !activeFollows(t, pool, followers[0], other) {
		t.Error("RemoveAllFollowers ended a follow that is not a follower of the owner")
	}
	if n, err := svc.RemoveAllFollowers(ctx, owner); err != nil || n != 0 {
		t.Errorf("a second RemoveAllFollowers = %d (%v), want 0", n, err)
	}
}

func TestPeopleListsSplit(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	base := time.Now().Add(-time.Hour)

	owner := testutil.SeedPublicReporter(t, pool, "split-owner@example.com", "split.owner", "split-owner-sess", base)
	viewer := testutil.SeedPerson(t, pool, "split-viewer@example.com", "split.viewer", false)

	named1 := testutil.SeedPerson(t, pool, "split-n1@example.com", "split.named1", true)
	named2 := testutil.SeedPerson(t, pool, "split-n2@example.com", "split.named2", true)
	private := testutil.SeedPerson(t, pool, "split-p@example.com", "split.private", false)
	pending := testutil.SeedPerson(t, pool, "split-pe@example.com", "split.pending", true)
	testutil.SeedUsername(t, pool, pending, "split.pending", "pending")
	blocked := testutil.SeedPerson(t, pool, "split-b@example.com", "split.blocked", true)
	testutil.SeedBlock(t, pool, viewer, blocked)
	for i, f := range []int64{named1, named2, private, pending, blocked} {
		testutil.SeedFollow(t, pool, f, owner, base.Add(time.Duration(i)*time.Minute))
	}

	got, err := svc.Followers(ctx, viewer, "split.owner", "")
	if err != nil {
		t.Fatalf("Followers: %v", err)
	}
	if len(got.Entries) != 2 || got.Entries[0].Username != "split.named2" || got.Entries[1].Username != "split.named1" {
		t.Fatalf("named followers = %+v, want split.named2 then split.named1 (newest first)", got.Entries)
	}
	if got.OthersCount != 3 || got.OthersText != "and 3 others" || got.Next != "" {
		t.Errorf("others %d %q next %q, want 3, \"and 3 others\" and no cursor", got.OthersCount, got.OthersText, got.Next)
	}

	// Following: the owner follows one named person and one private person.
	testutil.SeedFollow(t, pool, owner, named1, base)
	testutil.SeedFollow(t, pool, owner, private, base)
	got, err = svc.Following(ctx, viewer, "split.owner", "")
	if err != nil {
		t.Fatalf("Following: %v", err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Username != "split.named1" || got.OthersCount != 1 || got.OthersText != "and 1 other" {
		t.Errorf("Following = %+v, want one named person and \"and 1 other\"", got)
	}

	// Nobody named: only a number.
	loner := testutil.SeedPublicReporter(t, pool, "split-loner@example.com", "split.loner", "split-loner-sess", base)
	testutil.SeedFollow(t, pool, private, loner, base)
	testutil.SeedFollow(t, pool, pending, loner, base)
	got, err = svc.Followers(ctx, viewer, "split.loner", "")
	if err != nil || len(got.Entries) != 0 || got.OthersText != "2 others" {
		t.Errorf("Followers of loner = %+v (%v), want no names and \"2 others\"", got, err)
	}

	// A person who is not visible is not found; your own private page opens.
	if _, err := svc.Followers(ctx, viewer, "split.private", ""); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Followers of a private person = %v, want ErrPersonNotFound", err)
	}
	if _, err := svc.Following(ctx, viewer, "split.blocked", ""); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Following of a person you blocked = %v, want ErrPersonNotFound", err)
	}
	if _, err := svc.Followers(ctx, viewer, "nobody.here", ""); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Followers of nobody = %v, want ErrPersonNotFound", err)
	}
	if _, err := svc.Followers(ctx, private, "split.private", ""); err != nil {
		t.Errorf("Followers of your own private page = %v, want it to open", err)
	}
	blocksViewer := testutil.SeedPublicReporter(t, pool, "split-bv@example.com", "split.bv", "split-bv-sess", base)
	testutil.SeedBlock(t, pool, blocksViewer, viewer)
	if _, err := svc.Followers(ctx, viewer, "split.bv", ""); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("Followers of a person who blocked you = %v, want ErrPersonNotFound", err)
	}
	if _, err := svc.Followers(ctx, viewer, "split.owner", "not a cursor!"); err == nil {
		t.Error("a malformed cursor was accepted")
	} else {
		wantValidation(t, err, "after", "That page could not be read.")
	}
}

func TestPeopleListsKeyset(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	base := time.Now().Add(-time.Hour).Truncate(time.Second)

	owner := testutil.SeedPublicReporter(t, pool, "keyset-owner@example.com", "keyset.owner", "keyset-owner-sess", base)
	viewer := testutil.SeedPerson(t, pool, "keyset-viewer@example.com", "keyset.viewer", false)
	ids := seedBulkPeople(t, pool, "kf", 55, true)
	// Pairs share a created_at, so the username tie break is exercised.
	for i, id := range ids {
		testutil.SeedFollow(t, pool, id, owner, base.Add(time.Duration(i/2)*time.Second))
	}

	first, err := svc.Followers(ctx, viewer, "keyset.owner", "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first.Entries) != service.PeopleListPageSize || first.Next == "" {
		t.Fatalf("first page has %d entries and cursor %q, want %d and a cursor", len(first.Entries), first.Next, service.PeopleListPageSize)
	}
	second, err := svc.Followers(ctx, viewer, "keyset.owner", first.Next)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second.Entries) != 5 || second.Next != "" {
		t.Fatalf("second page has %d entries and cursor %q, want 5 and none", len(second.Entries), second.Next)
	}
	seen := map[string]bool{}
	for _, e := range append(append([]service.PersonEntry{}, first.Entries...), second.Entries...) {
		if seen[e.Username] {
			t.Fatalf("%s appears twice across the pages", e.Username)
		}
		seen[e.Username] = true
	}
	if len(seen) != 55 {
		t.Fatalf("the two pages hold %d people, want 55", len(seen))
	}
	if second.OthersCount != 0 || second.OthersText != "" {
		t.Errorf("others %d %q, want none", second.OthersCount, second.OthersText)
	}
}

func TestBlockedPeople(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)

	me := testutil.SeedPerson(t, pool, "bp-me@example.com", "bp.me", false)
	b := testutil.SeedPerson(t, pool, "bp-b@example.com", "bp.bravo", true)
	c := testutil.SeedPerson(t, pool, "bp-c@example.com", "bp.charlie", true)
	d := testutil.SeedPerson(t, pool, "bp-d@example.com", "bp.delta", true)
	testutil.SeedBlock(t, pool, me, b)
	testutil.SeedBlock(t, pool, me, c)
	testutil.SeedBlock(t, pool, me, d)
	// c went private since; d's username is pending now; a deleted account drops out.
	if _, err := pool.Exec(ctx, "UPDATE accounts SET profile_public = false WHERE id = $1", c); err != nil {
		t.Fatalf("going private: %v", err)
	}
	testutil.SeedUsername(t, pool, d, "bp.delta", "pending")
	gone := testutil.SeedPerson(t, pool, "bp-g@example.com", "bp.gone", true)
	testutil.SeedBlock(t, pool, me, gone)
	if _, err := pool.Exec(ctx, "UPDATE accounts SET deleted_at = now() WHERE id = $1", gone); err != nil {
		t.Fatalf("deleting: %v", err)
	}

	got, err := svc.BlockedPeople(ctx, me)
	if err != nil {
		t.Fatalf("BlockedPeople: %v", err)
	}
	if len(got) != 2 || got[0].Username != "bp.charlie" || got[1].Username != "bp.bravo" {
		t.Fatalf("BlockedPeople = %+v, want bp.charlie (private now, still listed) then bp.bravo", got)
	}
	// Only the owner's blocks are listed.
	if other, err := svc.BlockedPeople(ctx, b); err != nil || len(other) != 0 {
		t.Errorf("BlockedPeople of another account = %+v (%v), want none", other, err)
	}

	counts, err := svc.Counts(ctx, me)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.Blocked != 3 || counts.Followers != 0 || counts.Following != 0 {
		t.Errorf("Counts = %+v, want blocked 3 (live accounts), followers 0, following 0", counts)
	}
}
