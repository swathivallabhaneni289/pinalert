// followsweep_test.go proves the follow sweep (plan 09-06, SC5): follows of a
// person who loses the tag by time alone end with tag_lost, follows of a person
// who stops being followable end with profile_off, a Reliable person keeps
// every follower, followees are read in pages of 200 with one tag read per page,
// and Run stops when its context is cancelled. The database tests skip in the
// quick run; the paging and cancel tests use a fake querier and always run.

package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// sweepFollowCounts returns how many active follows and how many follows ended
// with each reason a followee has.
func sweepFollowCounts(t *testing.T, pool *pgxpool.Pool, followeeID int64) (active, tagLost, profileOff int) {
	t.Helper()

	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FILTER (WHERE ended_at IS NULL),
		        count(*) FILTER (WHERE end_reason = 'tag_lost'),
		        count(*) FILTER (WHERE end_reason = 'profile_off')
		 FROM account_follows WHERE followee_id = $1`, followeeID,
	).Scan(&active, &tagLost, &profileOff)
	if err != nil {
		t.Fatalf("counting follows of %d: %v", followeeID, err)
	}
	return active, tagLost, profileOff
}

// sweepRig seeds a public reporter with two followers at a clock two hours
// before wall time, the same pattern as the other database tests.
func sweepRig(t *testing.T, name string) (*pgxpool.Pool, *sqlcgen.Queries, time.Time, int64) {
	t.Helper()

	pool := testutil.NewTestDB(t)
	t0 := time.Now().UTC().Add(-2 * time.Hour)
	followee := testutil.SeedPublicReporter(t, pool, name+"@example.com", name+"user", name+"-session", t0)
	for i := 0; i < 2; i++ {
		follower := testutil.SeedPerson(t, pool, fmt.Sprintf("%s-follower-%d@example.com", name, i), fmt.Sprintf("%sfan%d", name, i), false)
		testutil.SeedFollow(t, pool, follower, followee, t0)
	}
	return pool, sqlcgen.New(pool), t0, followee
}

func TestFollowSweepKeepsReliable(t *testing.T) {
	pool, q, t0, followee := sweepRig(t, "keeper")

	sweep := NewFollowSweep(q, WithFollowSweepClock(func() time.Time { return t0 }))
	ended, err := sweep.SweepOnce(context.Background())
	if err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if ended != 0 {
		t.Errorf("ended = %d, want 0 for a Reliable reporter", ended)
	}
	if active, _, _ := sweepFollowCounts(t, pool, followee); active != 2 {
		t.Errorf("active follows = %d, want 2", active)
	}
}

func TestFollowSweepEndsOnDecay(t *testing.T) {
	pool, q, t0, followee := sweepRig(t, "decayer")

	// 120 days on, the judged history has left the 90 day window.
	later := t0.Add(120 * 24 * time.Hour)
	sweep := NewFollowSweep(q, WithFollowSweepClock(func() time.Time { return later }))
	ended, err := sweep.SweepOnce(context.Background())
	if err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if ended != 2 {
		t.Errorf("ended = %d, want 2", ended)
	}
	active, tagLost, profileOff := sweepFollowCounts(t, pool, followee)
	if active != 0 || tagLost != 2 || profileOff != 0 {
		t.Errorf("active=%d tag_lost=%d profile_off=%d, want 0, 2, 0", active, tagLost, profileOff)
	}
	var rows int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM account_follows WHERE followee_id = $1", followee,
	).Scan(&rows); err != nil || rows != 2 {
		t.Errorf("rows = %d (err %v), want 2: ended follows must stay (D-21)", rows, err)
	}

	// A second sweep has nothing left to end.
	if again, err := sweep.SweepOnce(context.Background()); err != nil || again != 0 {
		t.Errorf("second sweep ended %d (err %v), want 0", again, err)
	}
}

func TestFollowSweepEndsOnPrivate(t *testing.T) {
	cases := []struct {
		name   string
		change string
	}{
		{"private", "UPDATE accounts SET profile_public = false WHERE id = $1"},
		{"pending username", "UPDATE accounts SET username_screen = 'pending' WHERE id = $1"},
		{"deleted", "UPDATE accounts SET deleted_at = now() WHERE id = $1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool, q, t0, followee := sweepRig(t, "priv")
			if _, err := pool.Exec(context.Background(), tc.change, followee); err != nil {
				t.Fatalf("changing the followee: %v", err)
			}

			// Still Reliable by the clock: only the standing ends the follows.
			sweep := NewFollowSweep(q, WithFollowSweepClock(func() time.Time { return t0 }))
			ended, err := sweep.SweepOnce(context.Background())
			if err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			if ended != 2 {
				t.Errorf("ended = %d, want 2", ended)
			}
			active, tagLost, profileOff := sweepFollowCounts(t, pool, followee)
			if active != 0 || tagLost != 0 || profileOff != 2 {
				t.Errorf("active=%d tag_lost=%d profile_off=%d, want 0, 0, 2", active, tagLost, profileOff)
			}
		})
	}
}

// fakeSweepQuerier serves a fixed list of followee ids and counts the reads.
type fakeSweepQuerier struct {
	mu         sync.Mutex
	followees  []int64
	pageLimits []int32
	pageAfters []int64
	historyN   int
	ended      []int64
	reasons    []string
	// cleaned holds the followee id of every DeleteQueuedFollowPushesForFollowee
	// call (plan 09-26, 2026-10-30): the sweep's follow ends reach the queued note
	// cleanup through endFollowsOf.
	cleaned []int64
}

// DeleteQueuedFollowPushesForFollowee records the cleanup of a followee's queued
// follow notes. The sweep reaches it through endFollowsOf (plan 09-26).
func (f *fakeSweepQuerier) DeleteQueuedFollowPushesForFollowee(_ context.Context, followeeID int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleaned = append(f.cleaned, followeeID)
	return 0, nil
}

// DeleteQueuedFollowPush is part of followEnder; the sweep ends whole followees
// and never a pair, so it only answers.
func (f *fakeSweepQuerier) DeleteQueuedFollowPush(_ context.Context, _ sqlcgen.DeleteQueuedFollowPushParams) (int64, error) {
	return 0, nil
}

func (f *fakeSweepQuerier) ReporterHistoryReports(_ context.Context, _ sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.historyN++
	return nil, nil
}

func (f *fakeSweepQuerier) CurrentVotesForReports(_ context.Context, _ []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	return nil, nil
}

func (f *fakeSweepQuerier) FolloweeIDsPage(_ context.Context, arg sqlcgen.FolloweeIDsPageParams) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pageAfters = append(f.pageAfters, arg.AfterID)
	f.pageLimits = append(f.pageLimits, arg.RowLimit)
	var page []int64
	for _, id := range f.followees {
		if id > arg.AfterID && len(page) < int(arg.RowLimit) {
			page = append(page, id)
		}
	}
	return page, nil
}

func (f *fakeSweepQuerier) FolloweeStanding(_ context.Context, ids []int64) ([]sqlcgen.FolloweeStandingRow, error) {
	rows := make([]sqlcgen.FolloweeStandingRow, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, sqlcgen.FolloweeStandingRow{ID: id, Followable: true})
	}
	return rows, nil
}

// Amended 2026-10-30 by review IN-S01 and IN-E01: the sweep also reads active
// follows that sit beside a block; the fake has none.
func (f *fakeSweepQuerier) BlockedActiveFollowPairs(_ context.Context, _ int32) ([]sqlcgen.BlockedActiveFollowPairsRow, error) {
	return nil, nil
}

func (f *fakeSweepQuerier) EndFollowPair(_ context.Context, _ sqlcgen.EndFollowPairParams) (int64, error) {
	return 0, nil
}

func (f *fakeSweepQuerier) EndFollowsOf(_ context.Context, arg sqlcgen.EndFollowsOfParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ended = append(f.ended, arg.FolloweeID)
	f.reasons = append(f.reasons, arg.Reason)
	return 1, nil
}

func TestFollowSweepPagesBatches(t *testing.T) {
	f := &fakeSweepQuerier{}
	for i := int64(1); i <= 450; i++ {
		f.followees = append(f.followees, i)
	}

	sweep := NewFollowSweep(f)
	ended, err := sweep.SweepOnce(context.Background())
	if err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	// Three pages of 200, 200 and 50, each read once and with the page limit.
	if want := []int64{0, 200, 400}; fmt.Sprint(f.pageAfters) != fmt.Sprint(want) {
		t.Errorf("page starts = %v, want %v", f.pageAfters, want)
	}
	for _, limit := range f.pageLimits {
		if limit != int32(MaxReporterAccountsPerRead) {
			t.Errorf("row limit = %d, want %d", limit, MaxReporterAccountsPerRead)
		}
	}
	if f.historyN != 3 {
		t.Errorf("tag reads = %d, want 3 (one per page)", f.historyN)
	}
	// The fake has no history, so nobody is Reliable and every followee ends.
	if ended != 450 || len(f.ended) != 450 {
		t.Errorf("ended = %d over %d calls, want 450", ended, len(f.ended))
	}
	// Every end also cleaned the queued notes of that followee (plan 09-26).
	if fmt.Sprint(f.cleaned) != fmt.Sprint(f.ended) {
		t.Errorf("queued note cleanups = %d, want one per ended followee (%d)", len(f.cleaned), len(f.ended))
	}
	for _, r := range f.reasons {
		if r != string(FollowEndTagLost) {
			t.Fatalf("reason = %q, want tag_lost", r)
		}
	}
}

func TestFollowSweepStopsOnCancel(t *testing.T) {
	f := &fakeSweepQuerier{}
	sweep := NewFollowSweep(f)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sweep.Run(ctx)
		close(done)
	}()

	// The first sweep runs at start.
	deadline := time.Now().Add(5 * time.Second)
	for {
		f.mu.Lock()
		n := len(f.pageAfters)
		f.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Run did not sweep at start")
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}

	// An already cancelled context does no work at all.
	g := &fakeSweepQuerier{}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	NewFollowSweep(g).Run(cancelled)
	if len(g.pageAfters) != 0 {
		t.Errorf("Run on a cancelled context read %d pages, want 0", len(g.pageAfters))
	}
}
