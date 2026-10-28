package official

import (
	"context"
	"testing"
	"time"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/areas"
)

// TestHomeAreaCorners: a 5 character cell is tested at its centre and its four
// corners and every district found counts; anything that is not such a cell
// gives nothing.
func TestHomeAreaCorners(t *testing.T) {
	ix := areas.Default()
	s := &Service{ix: ix}

	// Vijayawada's cell is inside district 749.
	cell := geohash.EncodeWithPrecision(16.5062, 80.6480, 5)
	a := s.homeArea(cell)
	if a == nil {
		t.Fatalf("homeArea(%q) = nil", cell)
	}
	found749 := false
	for _, c := range a.DistrictCodes {
		if c == 749 {
			found749 = true
		}
	}
	if !found749 || a.StateCode != 28 || a.StateName != "Andhra Pradesh" {
		t.Errorf("homeArea(%q) = %+v", cell, *a)
	}

	// Scan a box of south India for a cell that straddles two districts, and
	// check the area carries exactly the districts the index finds, once each.
	var straddle string
	for lat := 15.5; lat < 17.5 && straddle == ""; lat += 0.01 {
		for lon := 79.0; lon < 81.5; lon += 0.01 {
			c := geohash.EncodeWithPrecision(lat, lon, 5)
			if len(ix.CellDistricts(c)) > 1 {
				straddle = c
				break
			}
		}
	}
	if straddle == "" {
		t.Fatal("found no cell that straddles two districts to test with")
	}
	want := ix.CellDistricts(straddle)
	got := s.homeArea(straddle)
	if got == nil || len(got.DistrictCodes) != len(want) {
		t.Fatalf("homeArea(%q) = %+v, want %d districts", straddle, got, len(want))
	}
	seen := map[int]bool{}
	for i, c := range got.DistrictCodes {
		if seen[c] {
			t.Errorf("district %d twice", c)
		}
		seen[c] = true
		if c != want[i].Code {
			t.Errorf("district %d = %d, want %d", i, c, want[i].Code)
		}
	}
	if got.DistrictName == "" || got.StateName == "" {
		t.Errorf("names empty: %+v", *got)
	}

	for _, bad := range []string{"", "tdr", "tdr1vv1", "TDR1V", "tdr1a", "!!!!!"} {
		if got := s.homeArea(bad); got != nil {
			t.Errorf("homeArea(%q) = %+v, want nil", bad, *got)
		}
	}
	// A valid cell far from every district (open sea) has no area either.
	if got := s.homeArea(geohash.EncodeWithPrecision(0, -30, 5)); got != nil {
		t.Errorf("a cell in the sea gave %+v", *got)
	}
}

// kickService is a Service over fake stores with a blocking SACHET fetcher.
func kickService(attemptAgo time.Duration, f *fakeFetcher) (*Service, *clock) {
	clk := &clock{t: refreshT0}
	feeds := newFakeFeeds(0)
	if attemptAgo > 0 {
		at := refreshT0.Add(-attemptAgo)
		feeds.rows[FeedSachet].LastAttemptAt = &at
	}
	r := NewRefresher(nil, nil, withStores(feeds, newFakeAlerts()), WithRefresherClock(clk.now))
	r.Register(FeedSachet, f, 0)
	s := newService(r, nil, areas.Default())
	s.now = clk.now
	return s, clk
}

func waitKickDone(t *testing.T, s *Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for s.kicking.Load() {
		if time.Now().After(deadline) {
			t.Fatal("the kick never finished")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestKickIsNonBlocking(t *testing.T) {
	t.Run("returns at once and runs one at a time", func(t *testing.T) {
		f := &fakeFetcher{release: make(chan struct{}), started: make(chan struct{}, 4)}
		s, _ := kickService(11*time.Minute, f)

		begin := time.Now()
		s.Kick()
		if took := time.Since(begin); took > 200*time.Millisecond {
			t.Fatalf("Kick took %v while the fetcher was blocked", took)
		}
		select {
		case <-f.started:
		case <-time.After(2 * time.Second):
			t.Fatal("the refresh never started")
		}

		begin = time.Now()
		s.Kick() // a second Kick during the run starts nothing
		s.Kick()
		if took := time.Since(begin); took > 200*time.Millisecond {
			t.Fatalf("a second Kick took %v", took)
		}
		time.Sleep(50 * time.Millisecond)
		if n := f.calls.Load(); n != 1 {
			t.Fatalf("fetcher ran %d times during one kick, want 1", n)
		}

		close(f.release)
		waitKickDone(t, s)
		if n := f.calls.Load(); n != 1 {
			t.Fatalf("fetcher ran %d times, want 1", n)
		}
	})

	t.Run("does nothing when the last attempt is under ten minutes old", func(t *testing.T) {
		f := &fakeFetcher{}
		s, _ := kickService(LazyKickAfter-time.Minute, f)
		s.Kick()
		waitKickDone(t, s)
		if n := f.calls.Load(); n != 0 {
			t.Fatalf("fetcher ran %d times, want 0", n)
		}
	})

	t.Run("runs for a feed never attempted", func(t *testing.T) {
		f := &fakeFetcher{}
		s, _ := kickService(0, f)
		s.Kick()
		waitKickDone(t, s)
		if n := f.calls.Load(); n != 1 {
			t.Fatalf("fetcher ran %d times, want 1", n)
		}
	})

	t.Run("a failed run is quiet and lets the next kick through", func(t *testing.T) {
		f := &fakeFetcher{err: context.DeadlineExceeded}
		s, clk := kickService(11*time.Minute, f)
		s.Kick()
		waitKickDone(t, s)
		clk.set(refreshT0.Add(30 * time.Minute))
		s.Kick()
		waitKickDone(t, s)
		if n := f.calls.Load(); n != 2 {
			t.Fatalf("fetcher ran %d times, want 2", n)
		}
	})
}

// waitCalls waits until the fetcher has run n times.
func waitCalls(t *testing.T, f *fakeFetcher, n int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for f.calls.Load() < n {
		if time.Now().After(deadline) {
			t.Fatalf("the fetcher ran %d times, want %d", f.calls.Load(), n)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestServiceRunTicks: Run sweeps once at start and once per tick, and returns
// when its context ends. The ticker is a channel the test drives, so nothing
// sleeps for a real interval (plan 05-12).
func TestServiceRunTicks(t *testing.T) {
	if TickerInterval != 5*time.Minute {
		t.Fatalf("TickerInterval = %v, want 5m", TickerInterval)
	}
	f := &fakeFetcher{res: Result{ItemsSeen: 3}}
	s, _ := kickService(0, f) // the gap is 0, so every sweep fetches
	ticks := make(chan time.Time)
	var gotEvery time.Duration
	stopped := make(chan struct{})
	s.newTicker = func(d time.Duration) (<-chan time.Time, func()) {
		gotEvery = d
		return ticks, func() { close(stopped) }
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	waitCalls(t, f, 1) // the sweep at boot
	ticks <- refreshT0
	waitCalls(t, f, 2)
	ticks <- refreshT0
	waitCalls(t, f, 3)

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context ended")
	}
	select {
	case <-stopped:
	default:
		t.Error("Run left its ticker running")
	}
	if gotEvery != TickerInterval {
		t.Errorf("the ticker ran every %v, want %v", gotEvery, TickerInterval)
	}
	if n := f.calls.Load(); n != 3 {
		t.Errorf("%d fetches, want 3 (boot and two ticks)", n)
	}

	// A context that is already over starts nothing.
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	s.Run(cancelled)
	if n := f.calls.Load(); n != 3 {
		t.Errorf("Run on a finished context fetched: %d fetches, want 3", n)
	}
}

// TestRefreshAllRespectsGap: two RefreshAll calls a minute apart fetch once per
// feed, because each feed goes through RefreshIfDue and its minimum gap. A call
// past the gap fetches again. Feeds run in a fixed order.
func TestRefreshAllRespectsGap(t *testing.T) {
	clk := &clock{t: refreshT0}
	r := NewRefresher(nil, nil, withStores(newFakeFeeds(0), newFakeAlerts()), WithRefresherClock(clk.now))
	sachet := &fakeFetcher{res: Result{ItemsSeen: 5, Stored: 2}}
	gdacs := &fakeFetcher{res: Result{ItemsSeen: 1}}
	r.Register(FeedGdacs, gdacs, MinRefreshGap) // registered first on purpose
	r.Register(FeedSachet, sachet, MinRefreshGap)
	s := newService(r, nil, areas.Default())

	first := s.RefreshAll(context.Background())
	if len(first) != 2 || first[0].Feed != FeedSachet || first[1].Feed != FeedGdacs {
		t.Fatalf("first call = %+v, want sachet then gdacs", first)
	}
	if first[0].Skipped || first[0].Stored != 2 || first[0].ItemsSeen != 5 {
		t.Errorf("sachet result = %+v", first[0])
	}

	clk.set(refreshT0.Add(time.Minute))
	second := s.RefreshAll(context.Background())
	for _, res := range second {
		if !res.Skipped {
			t.Errorf("%s was fetched again a minute later: %+v", res.Feed, res)
		}
	}
	if sachet.calls.Load() != 1 || gdacs.calls.Load() != 1 {
		t.Fatalf("fetches after two calls: sachet %d, gdacs %d, want 1 each", sachet.calls.Load(), gdacs.calls.Load())
	}

	clk.set(refreshT0.Add(MinRefreshGap))
	s.RefreshAll(context.Background())
	if sachet.calls.Load() != 2 || gdacs.calls.Load() != 2 {
		t.Fatalf("fetches after the gap: sachet %d, gdacs %d, want 2 each", sachet.calls.Load(), gdacs.calls.Load())
	}
}

// TestRefreshAllReportsFailures: a failed feed carries its closed word, and the
// other feed still runs.
func TestRefreshAllReportsFailures(t *testing.T) {
	clk := &clock{t: refreshT0}
	r := NewRefresher(nil, nil, withStores(newFakeFeeds(0), newFakeAlerts()), WithRefresherClock(clk.now))
	r.Register(FeedSachet, &fakeFetcher{err: context.DeadlineExceeded}, MinRefreshGap)
	gdacs := &fakeFetcher{res: Result{ItemsSeen: 1}}
	r.Register(FeedGdacs, gdacs, MinRefreshGap)
	s := newService(r, nil, areas.Default())

	got := s.RefreshAll(context.Background())
	if len(got) != 2 || got[0].ErrorClass != "timeout" || got[1].ErrorClass != "" {
		t.Fatalf("results = %+v, want sachet timeout then gdacs clean", got)
	}
	if gdacs.calls.Load() != 1 {
		t.Errorf("gdacs ran %d times after a sachet failure, want 1", gdacs.calls.Load())
	}
	if got := newService(NewRefresher(nil, nil), nil, areas.Default()).RefreshAll(context.Background()); len(got) != 0 {
		t.Errorf("a service with no feed returned %+v, want none", got)
	}
}
