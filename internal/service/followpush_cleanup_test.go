// followpush_cleanup_test.go proves that every way a follow or a bell ends also
// ends the phone notes still waiting for it (Phase 9 plan 26, PROF-03, SC5
// pending alerts, research Q9, review W1, D-24): the cleanup lives inside the two
// follow ending helpers, so Unfollow, Remove follower, Remove all followers,
// switching the profile off, evaluate on touch, a rating write, a moderator
// removal and the sweep all get it, Block ends its follows through the same
// helper, and Mute and turning the bell off delete the notes of their own pair.
// A note already sent and the bell rows stay as history. The tests run on a real
// database and skip without DATABASE_URL.
//
// Every person here is made up and uses example.com.

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"pinalert/internal/testutil"
)

// cleanupScene is a Reliable person with three followers, each of whom has the
// bell on, one queued follow note about the person's newest report, and one
// follow note already sent about an older one.
type cleanupScene struct {
	r          *followRig
	person     int64
	personName string
	session    string
	report     int64
	old        int64
	followers  [3]int64
	names      [3]string
}

func newCleanupScene(t *testing.T) *cleanupScene {
	t.Helper()
	r := newFollowRig(t)
	s := &cleanupScene{r: r, personName: "cl.person"}
	s.person, s.session = r.person(s.personName)
	s.old = r.report(s.session, SeverityLow, r.base.Add(5*time.Minute), "clear")
	s.report = r.report(s.session, SeverityLow, r.base.Add(20*time.Minute), "clear")
	for i := range s.followers {
		s.names[i] = fmt.Sprintf("cl.follower%d", i+1)
		s.followers[i] = testutil.SeedPerson(t, r.pool, fmt.Sprintf("p9-26-cl-f%d@example.com", i+1), s.names[i], true)
		r.bellOn(s.followers[i], s.person, r.base.Add(10*time.Minute))
		r.queueFollowRow(s.followers[i], s.report, "queued")
		r.queueFollowRow(s.followers[i], s.old, "sent")
	}
	return s
}

// queueFollowRow writes one follow row for the follower about the report, due at
// once, in the given state ("queued" or "sent").
func (r *followRig) queueFollowRow(follower, report int64, state string) {
	r.t.Helper()
	if state == "sent" {
		testutil.MustExec(r.t, r.pool,
			`INSERT INTO push_outbox (account_id, report_id, kind, state, sent_at, expires_at)
			 VALUES ($1, $2, 'follow', 'sent', $3, $4)`,
			follower, report, r.base.Add(6*time.Minute), r.base.Add(time.Hour))
		return
	}
	testutil.MustExec(r.t, r.pool,
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'follow', $3)`,
		follower, report, r.base.Add(2*time.Hour))
}

// followRowsIn counts a follower's follow rows in one state.
func (r *followRig) followRowsIn(follower int64, state string) int {
	r.t.Helper()
	return r.count(`SELECT COUNT(*) FROM push_outbox WHERE account_id = $1 AND kind = 'follow' AND state = $2`, follower, state)
}

// wantQueued checks the queued follow notes of each follower, by index, and that
// every sent note is still there.
func (s *cleanupScene) wantQueued(t *testing.T, what string, want [3]int) {
	t.Helper()
	for i, f := range s.followers {
		if got := s.r.followRowsIn(f, "queued"); got != want[i] {
			t.Errorf("%s: follower %d holds %d queued follow notes, want %d", what, i+1, got, want[i])
		}
		if got := s.r.followRowsIn(f, "sent"); got != 1 {
			t.Errorf("%s: follower %d holds %d sent follow notes, want 1 (history stays)", what, i+1, got)
		}
	}
}

// people builds the people service on the rig, at the given time, with the one
// ratings source.
func (s *cleanupScene) people(at time.Time) *PeopleService {
	return NewPeopleService(s.r.q, WithPeopleClock(func() time.Time { return at }), WithPeopleSignals(NewRatingService(s.r.q)))
}

// peopleWithComments is people plus the comment source BlockCommentAuthor needs,
// its report check running on the scene clock (quick 261030-u78).
func (s *cleanupScene) peopleWithComments(at time.Time) *PeopleService {
	comments := NewCommentService(s.r.q, WithCommentClock(func() time.Time { return at }))
	return NewPeopleService(s.r.q, WithPeopleClock(func() time.Time { return at }),
		WithPeopleSignals(NewRatingService(s.r.q)), WithPeopleCommentAuthors(comments))
}

// TestFollowEndsOnSwitchOff (V13): the person switches the profile off. The
// follows end with profile_off, the queued follow notes of every follower are
// gone, and the worker sends nothing, although the notes were really queued by
// the dispatcher with devices behind them.
func TestFollowEndsOnSwitchOff(t *testing.T) {
	r := newFollowPushRig(t)
	ctx := context.Background()
	person, session := r.person("w26.switchoff")
	var followers []int64
	for _, name := range []string{"one", "two"} {
		followers = append(followers, r.followerWithDevice(name, person))
	}
	report := r.postedByPerson(session)
	for _, f := range followers {
		if !outboxKindsEqual(r.outbox(report)[f], "follow") {
			t.Fatalf("follower %d holds %v, want a queued follow note", f, r.outbox(report)[f])
		}
	}

	svc := NewPeopleService(r.q, WithPeopleClock(func() time.Time { return r.cur }), WithPeopleSignals(NewRatingService(r.q)))
	if err := svc.SetProfilePublic(ctx, person, false); err != nil {
		t.Fatalf("SetProfilePublic off: %v", err)
	}
	if n := r.count(`SELECT COUNT(*) FROM account_follows WHERE followee_id = $1 AND ended_at IS NOT NULL AND end_reason = 'profile_off'`, person); n != 2 {
		t.Errorf("follows ended with profile_off = %d, want 2", n)
	}
	if rows := r.outbox(report); len(rows) != 0 {
		t.Errorf("outbox after the switch off = %v, want no queued follow notes", rows)
	}

	sender := &pwSender{answers: map[string]pwAnswer{}}
	if err := pwFollowWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := len(sender.messages()); got != 0 {
		t.Errorf("messages after the switch off = %d, want 0", got)
	}
	// The bell rows stay as history.
	for _, f := range followers {
		if n := r.notes(f, report); n != 1 {
			t.Errorf("bell rows of follower %d = %d, want 1", f, n)
		}
	}
}

// TestFollowCleanupOnEveryEndPath: each way a follow ends deletes the queued
// follow notes it should. Unfollow and Remove follower delete the pair's; the
// others delete every follower's.
func TestFollowCleanupOnEveryEndPath(t *testing.T) {
	ctx := context.Background()
	pairOnly := [3]int{0, 1, 1}
	everyone := [3]int{0, 0, 0}
	later := func(s *cleanupScene) time.Time { return s.r.base.Add(120 * 24 * time.Hour) }

	paths := []struct {
		name string
		want [3]int
		run  func(t *testing.T, s *cleanupScene)
	}{
		{"Unfollow", pairOnly, func(t *testing.T, s *cleanupScene) {
			if err := s.people(s.r.cur).Unfollow(ctx, s.followers[0], s.personName); err != nil {
				t.Fatal(err)
			}
		}},
		{"Remove follower", pairOnly, func(t *testing.T, s *cleanupScene) {
			if err := s.people(s.r.cur).RemoveFollower(ctx, s.person, s.names[0]); err != nil {
				t.Fatal(err)
			}
		}},
		{"Remove all followers", everyone, func(t *testing.T, s *cleanupScene) {
			if _, err := s.people(s.r.cur).RemoveAllFollowers(ctx, s.person); err != nil {
				t.Fatal(err)
			}
		}},
		{"switching the profile off", everyone, func(t *testing.T, s *cleanupScene) {
			if err := s.people(s.r.cur).SetProfilePublic(ctx, s.person, false); err != nil {
				t.Fatal(err)
			}
		}},
		{"evaluate on touch", everyone, func(t *testing.T, s *cleanupScene) {
			if _, err := s.people(later(s)).Profile(ctx, s.followers[0], s.personName); err != nil {
				t.Fatal(err)
			}
		}},
		{"the sweep", everyone, func(t *testing.T, s *cleanupScene) {
			sweep := NewFollowSweep(s.r.q, WithFollowSweepClock(func() time.Time { return later(s) }), WithFollowSweepSignals(NewRatingService(s.r.q)))
			if _, err := sweep.SweepOnce(ctx); err != nil {
				t.Fatal(err)
			}
		}},
		{"a rating write that drops the tag", everyone, func(t *testing.T, s *cleanupScene) {
			r := s.r
			rated := testutil.SeedHistoryReport(t, r.pool, s.session, "low", "flood", r.base.Add(-time.Hour))
			testutil.SeedRaters(t, r.pool, s.person, rated, []int{1, 1, 1, 1})
			fifth := testutil.SeedReporterAccount(t, r.pool, "p9-26-cl-fifth@example.com")
			testutil.SeedUsername(t, r.pool, fifth, "cl.fifth", "clear")
			r.vote(rated, fifth, "content", "confirm", "tdr1x55", r.base)
			svc := NewRatingService(r.q, WithRatingClock(func() time.Time { return r.cur }))
			if err := svc.Rate(ctx, fifth, s.personName, 1); err != nil {
				t.Fatalf("Rate: %v", err)
			}
		}},
		{"a moderator removal that leaves the tag down", everyone, func(t *testing.T, s *cleanupScene) {
			r := s.r
			rated := testutil.SeedHistoryReport(t, r.pool, s.session, "low", "flood", r.base.Add(-time.Hour))
			raters := testutil.SeedRaters(t, r.pool, s.person, rated, []int{1, 1, 1, 1, 1, 1})
			testutil.SeedUsername(t, r.pool, raters[0], "cl.rater0", "clear")
			mod := testutil.SeedReporterAccount(t, r.pool, "p9-26-cl-mod@example.com")
			svc := NewRatingService(r.q, WithRatingClock(func() time.Time { return r.cur }))
			if err := svc.ModeratorRemove(ctx, mod, s.personName, "cl.rater0"); err != nil {
				t.Fatalf("ModeratorRemove: %v", err)
			}
		}},
	}
	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			s := newCleanupScene(t)
			s.wantQueued(t, "before", [3]int{1, 1, 1})
			p.run(t, s)
			s.wantQueued(t, p.name, p.want)
		})
	}
}

// TestFollowCleanupOnBlockMuteBellOff: Block in either direction, Mute and
// turning the bell off each delete the queued follow notes of their own pair and
// no one else's.
func TestFollowCleanupOnBlockMuteBellOff(t *testing.T) {
	ctx := context.Background()
	pairOnly := [3]int{0, 1, 1}
	paths := []struct {
		name string
		run  func(t *testing.T, s *cleanupScene)
	}{
		{"the follower blocks the person", func(t *testing.T, s *cleanupScene) {
			if err := s.people(s.r.cur).Block(ctx, s.followers[0], s.personName); err != nil {
				t.Fatal(err)
			}
		}},
		{"the person blocks the follower", func(t *testing.T, s *cleanupScene) {
			if err := s.people(s.r.cur).Block(ctx, s.person, s.names[0]); err != nil {
				t.Fatal(err)
			}
		}},
		// Amended 2026-10-30 by quick 261030-u78: the two comment paths run through
		// the same block tail and must end up with the same cleanup.
		{"the follower blocks the person from a comment", func(t *testing.T, s *cleanupScene) {
			c := testutil.SeedComment(t, s.r.pool, testutil.CommentSeed{ReportID: s.report, AccountID: s.person})
			if err := s.peopleWithComments(s.r.cur).BlockCommentAuthor(ctx, s.followers[0], c); err != nil {
				t.Fatal(err)
			}
		}},
		{"the person blocks the follower from a comment", func(t *testing.T, s *cleanupScene) {
			c := testutil.SeedComment(t, s.r.pool, testutil.CommentSeed{ReportID: s.report, AccountID: s.followers[0]})
			if err := s.peopleWithComments(s.r.cur).BlockCommentAuthor(ctx, s.person, c); err != nil {
				t.Fatal(err)
			}
		}},
		{"Mute", func(t *testing.T, s *cleanupScene) {
			if err := s.people(s.r.cur).Mute(ctx, s.followers[0], s.personName); err != nil {
				t.Fatal(err)
			}
		}},
		{"the bell turned off", func(t *testing.T, s *cleanupScene) {
			if err := s.people(s.r.cur).SetNotify(ctx, s.followers[0], s.personName, false); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			s := newCleanupScene(t)
			s.wantQueued(t, "before", [3]int{1, 1, 1})
			p.run(t, s)
			s.wantQueued(t, p.name, pairOnly)
		})
	}

	// Turning the bell on again does not bring a deleted note back.
	s := newCleanupScene(t)
	svc := s.people(s.r.cur)
	if err := svc.SetNotify(ctx, s.followers[0], s.personName, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetNotify(ctx, s.followers[0], s.personName, true); err != nil {
		t.Fatal(err)
	}
	s.wantQueued(t, "bell off then on", [3]int{0, 1, 1})
}
