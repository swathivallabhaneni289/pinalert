package official

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pinalert/internal/areas"
)

// These tests run the refresher with no database: a map backed feed store, an
// alert store that records what it is asked to save, and fake fetchers.

var refreshT0 = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// fakeFeeds is an in memory official_feed_status with the same claim rule as
// the SQL: a claim wins only when the last attempt is not newer than the cutoff.
type fakeFeeds struct {
	mu   sync.Mutex
	rows map[Feed]*FeedStatus
}

func newFakeFeeds(items int) *fakeFeeds {
	return &fakeFeeds{rows: map[Feed]*FeedStatus{
		FeedSachet: {Feed: FeedSachet, ItemsSeen: items},
		FeedGdacs:  {Feed: FeedGdacs},
	}}
}

func (f *fakeFeeds) row(feed Feed) *FeedStatus {
	if f.rows[feed] == nil {
		f.rows[feed] = &FeedStatus{Feed: feed}
	}
	return f.rows[feed]
}

func (f *fakeFeeds) status(_ context.Context, feed Feed) (FeedStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.row(feed), nil
}

func (f *fakeFeeds) claim(_ context.Context, feed Feed, now, cutoff time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.row(feed)
	if r.LastAttemptAt != nil && r.LastAttemptAt.After(cutoff) {
		return false, nil
	}
	t := now
	r.LastAttemptAt = &t
	return true, nil
}

func (f *fakeFeeds) success(_ context.Context, feed Feed, now time.Time, res Result) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.row(feed)
	t := now
	r.LastSuccessAt, r.LastError, r.ConsecutiveFailures = &t, "", 0
	r.ETag, r.LastModified, r.ItemsSeen = res.ETag, res.LastModified, res.ItemsSeen
	return nil
}

func (f *fakeFeeds) failure(_ context.Context, feed Feed, class string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.row(feed)
	r.LastError = class
	r.ConsecutiveFailures++
	return nil
}

// fakeAlerts remembers the pubDate of each saved guid and can fail on request.
type fakeAlerts struct {
	mu      sync.Mutex
	pub     map[string]time.Time
	saved   []Version
	saveErr error
}

func newFakeAlerts() *fakeAlerts { return &fakeAlerts{pub: map[string]time.Time{}} }

func (a *fakeAlerts) pubDates(_ context.Context, guids []string) (map[string]time.Time, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := map[string]time.Time{}
	for _, g := range guids {
		if t, ok := a.pub[g]; ok {
			out[g] = t
		}
	}
	return out, nil
}

func (a *fakeAlerts) save(_ context.Context, v Version, _ Placement, _ time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.saveErr != nil {
		return a.saveErr
	}
	a.pub[v.GUID] = v.SourcePubDate
	a.saved = append(a.saved, v)
	return nil
}

// clock is a settable clock.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) set(t time.Time) {
	c.mu.Lock()
	c.t = t
	c.mu.Unlock()
}

// fakeFetcher counts its runs, can block, and returns a canned answer.
type fakeFetcher struct {
	calls   atomic.Int32
	release chan struct{} // when set, a run waits for it to be closed
	started chan struct{} // when set, a run signals once it begins
	res     Result
	err     error
	seen    []FeedStatus
	mu      sync.Mutex
}

func (f *fakeFetcher) Fetch(_ context.Context, st FeedStatus) (Result, error) {
	f.calls.Add(1)
	f.mu.Lock()
	f.seen = append(f.seen, st)
	f.mu.Unlock()
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if f.release != nil {
		<-f.release
	}
	return f.res, f.err
}

func newTestRefresher(feeds *fakeFeeds, c *clock) *Refresher {
	return NewRefresher(nil, nil, withStores(feeds, newFakeAlerts()), WithRefresherClock(c.now))
}

func TestRefreshSkipsWithinGap(t *testing.T) {
	feeds := newFakeFeeds(0)
	c := &clock{t: refreshT0}
	r := newTestRefresher(feeds, c)
	f := &fakeFetcher{res: Result{ItemsSeen: 5}}
	r.Register(FeedSachet, f, MinRefreshGap)

	if res, err := r.RefreshIfDue(context.Background(), FeedSachet); err != nil || res.Skipped {
		t.Fatalf("first call: %+v, %v; want a run", res, err)
	}
	c.set(refreshT0.Add(3 * time.Minute))
	res, err := r.RefreshIfDue(context.Background(), FeedSachet)
	if err != nil || !res.Skipped {
		t.Fatalf("call at 3 minutes: %+v, %v; want Skipped", res, err)
	}
	if got := f.calls.Load(); got != 1 {
		t.Fatalf("%d fetches after the skip, want 1", got)
	}
	c.set(refreshT0.Add(4 * time.Minute))
	if res, err := r.RefreshIfDue(context.Background(), FeedSachet); err != nil || res.Skipped {
		t.Fatalf("call at 4 minutes: %+v, %v; want a run", res, err)
	}
	if got := f.calls.Load(); got != 2 {
		t.Fatalf("%d fetches at 4 minutes, want 2", got)
	}
	if _, err := r.RefreshIfDue(context.Background(), "nope"); !errors.Is(err, ErrUnknownFeed) {
		t.Errorf("an unregistered feed gave %v, want ErrUnknownFeed", err)
	}
}

func TestRefreshSkipsWhenAnotherInstanceClaimed(t *testing.T) {
	feeds := newFakeFeeds(0)
	c := &clock{t: refreshT0}
	r := newTestRefresher(feeds, c)
	f := &fakeFetcher{}
	r.Register(FeedSachet, f, MinRefreshGap)

	// Another instance claimed the run after this one read the status: the read
	// said no attempt, the claim finds a fresh one.
	feeds.mu.Lock()
	recent := refreshT0.Add(-time.Minute)
	feeds.row(FeedSachet).LastAttemptAt = &recent
	feeds.mu.Unlock()
	if res, err := r.RefreshIfDue(context.Background(), FeedSachet); err != nil || !res.Skipped || f.calls.Load() != 0 {
		t.Fatalf("got %+v, %v after %d fetches; want Skipped with none", res, err, f.calls.Load())
	}
}

func TestRefreshOneAtATime(t *testing.T) {
	feeds := newFakeFeeds(0)
	c := &clock{t: refreshT0}
	r := newTestRefresher(feeds, c)
	slow := &fakeFetcher{release: make(chan struct{}), started: make(chan struct{}, 1), res: Result{ItemsSeen: 2, Stored: 2}}
	other := &fakeFetcher{res: Result{ItemsSeen: 1}}
	r.Register(FeedSachet, slow, MinRefreshGap)
	r.Register(FeedGdacs, other, MinRefreshGap)

	results := make(chan Result, 2)
	go func() { res, _ := r.RefreshIfDue(context.Background(), FeedSachet); results <- res }()
	<-slow.started
	// A different feed is not held up by the running one.
	if res, err := r.RefreshIfDue(context.Background(), FeedGdacs); err != nil || res.Skipped || other.calls.Load() != 1 {
		t.Fatalf("other feed: %+v, %v after %d fetches; want a run", res, err, other.calls.Load())
	}
	go func() { res, _ := r.RefreshIfDue(context.Background(), FeedSachet); results <- res }()
	time.Sleep(50 * time.Millisecond) // let the second caller reach the wait
	close(slow.release)

	for i := 0; i < 2; i++ {
		select {
		case res := <-results:
			if res.Skipped || res.Stored != 2 {
				t.Errorf("caller %d got %+v, want the shared result", i, res)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("a caller never returned")
		}
	}
	if got := slow.calls.Load(); got != 1 {
		t.Fatalf("%d fetches for two concurrent calls, want 1", got)
	}
}

func TestRefreshAnomaly(t *testing.T) {
	t.Run("empty after a full run is a failure", func(t *testing.T) {
		feeds := newFakeFeeds(12)
		success := refreshT0.Add(-time.Hour)
		feeds.rows[FeedSachet].LastSuccessAt = &success
		c := &clock{t: refreshT0}
		r := newTestRefresher(feeds, c)
		r.Register(FeedSachet, &fakeFetcher{res: Result{ItemsSeen: 0}}, MinRefreshGap)

		res, err := r.RefreshIfDue(context.Background(), FeedSachet)
		if err == nil || res.ErrorClass != "anomaly" {
			t.Fatalf("got %+v, %v; want an anomaly failure", res, err)
		}
		row := feeds.rows[FeedSachet]
		if row.LastError != "anomaly" || row.ConsecutiveFailures != 1 {
			t.Errorf("row = %+v, want error anomaly and one failure", row)
		}
		if row.LastSuccessAt == nil || !row.LastSuccessAt.Equal(success) || row.ItemsSeen != 12 {
			t.Errorf("an anomaly moved the success time or the item count: %+v", row)
		}
	})
	t.Run("empty after a small run is a success", func(t *testing.T) {
		feeds := newFakeFeeds(3)
		c := &clock{t: refreshT0}
		r := newTestRefresher(feeds, c)
		r.Register(FeedSachet, &fakeFetcher{res: Result{ItemsSeen: 0}}, MinRefreshGap)

		if res, err := r.RefreshIfDue(context.Background(), FeedSachet); err != nil || res.ErrorClass != "" {
			t.Fatalf("got %+v, %v; want a success", res, err)
		}
		row := feeds.rows[FeedSachet]
		if row.LastSuccessAt == nil || !row.LastSuccessAt.Equal(refreshT0) || row.ItemsSeen != 0 || row.LastError != "" {
			t.Errorf("row = %+v, want success at t0 with no items", row)
		}
	})
	t.Run("the floor is ten", func(t *testing.T) {
		if AnomalyFloor != 10 {
			t.Fatalf("AnomalyFloor = %d, want 10", AnomalyFloor)
		}
		feeds := newFakeFeeds(AnomalyFloor)
		c := &clock{t: refreshT0}
		r := newTestRefresher(feeds, c)
		r.Register(FeedSachet, &fakeFetcher{res: Result{ItemsSeen: 0}}, MinRefreshGap)
		if _, err := r.RefreshIfDue(context.Background(), FeedSachet); err == nil {
			t.Error("an empty answer after exactly ten items was taken as a success")
		}
	})
}

func TestRefreshFailureWordsAreClosed(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"status":  {&fetchError{class: "status", status: 500, msg: "status 500"}, "status"},
		"parse":   {parseError("bad"), "parse"},
		"timeout": {context.DeadlineExceeded, "timeout"},
		"other":   {errors.New("dial tcp 10.0.0.1: refused"), "network"},
	} {
		t.Run(name, func(t *testing.T) {
			feeds := newFakeFeeds(0)
			r := newTestRefresher(feeds, &clock{t: refreshT0})
			r.Register(FeedSachet, &fakeFetcher{err: tc.err}, MinRefreshGap)
			res, err := r.RefreshIfDue(context.Background(), FeedSachet)
			if err == nil || res.ErrorClass != tc.want || feeds.rows[FeedSachet].LastError != tc.want {
				t.Errorf("got %+v, %v, stored %q; want class %s", res, err, feeds.rows[FeedSachet].LastError, tc.want)
			}
		})
	}
}

// fakeSachet serves a fixed list and CAP results, and records the requests.
type fakeSachet struct {
	mu       sync.Mutex
	list     ListResult
	listErr  error
	etagSent []string
	capReqs  [][]Item
	capErr   func(Item) error
	notKept  map[string]bool
}

func (s *fakeSachet) FetchList(_ context.Context, etag, _ string) (ListResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.etagSent = append(s.etagSent, etag)
	return s.list, s.listErr
}

func (s *fakeSachet) FetchCAPs(_ context.Context, items []Item) []CAPResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.capReqs = append(s.capReqs, append([]Item(nil), items...))
	out := make([]CAPResult, len(items))
	for i, it := range items {
		out[i] = CAPResult{Item: it}
		if s.capErr != nil {
			if err := s.capErr(it); err != nil {
				out[i].Err = err
				continue
			}
		}
		out[i].Version = Version{
			GUID: it.GUID, CapIdentifier: "IN-" + it.GUID + "_1", MsgType: MsgAlert, Status: "Actual", Scope: "Public",
			Severity: SeverityModerate, SentAt: it.PubDate, ExpiresAt: it.PubDate.Add(time.Hour), SourcePubDate: it.PubDate,
		}
		out[i].Kept = !s.notKept[it.GUID]
	}
	return out
}

func manyItems(n int) []Item {
	items := make([]Item, n)
	for i := range items {
		// pubDate rises with i, so the feed's own order is not newest first.
		items[i] = Item{GUID: fmt.Sprintf("1792%012d", i), PubDate: refreshT0.Add(time.Duration(i) * time.Minute)}
	}
	return items
}

func newSachetRefresher(t *testing.T, api *fakeSachet) (*Refresher, *fakeFeeds, *fakeAlerts, *clock) {
	t.Helper()
	feeds := newFakeFeeds(0)
	alerts := newFakeAlerts()
	c := &clock{t: refreshT0}
	r := NewRefresher(nil, nil, withStores(feeds, alerts), WithRefresherClock(c.now))
	r.Register(FeedSachet, newSachetFetcher(api, areas.Default(),
		func() alertStore { return alerts }, c.now), MinRefreshGap)
	return r, feeds, alerts, c
}

func TestRefreshNotModified(t *testing.T) {
	api := &fakeSachet{list: ListResult{NotModified: true}}
	r, feeds, _, c := newSachetRefresher(t, api)
	feeds.rows[FeedSachet].ItemsSeen = 42
	feeds.rows[FeedSachet].ETag = `W/"old"`

	res, err := r.RefreshIfDue(context.Background(), FeedSachet)
	if err != nil || !res.NotModified {
		t.Fatalf("got %+v, %v; want a not modified success", res, err)
	}
	row := feeds.rows[FeedSachet]
	if row.LastSuccessAt == nil || !row.LastSuccessAt.Equal(c.now()) {
		t.Errorf("a 304 did not move last_success_at: %+v", row)
	}
	if row.ItemsSeen != 42 || row.ETag != `W/"old"` {
		t.Errorf("a 304 changed items_seen to %d or the etag to %q", row.ItemsSeen, row.ETag)
	}
	if len(api.etagSent) != 1 || api.etagSent[0] != `W/"old"` {
		t.Errorf("the stored etag was not sent: %v", api.etagSent)
	}
	if len(api.capReqs) != 0 {
		t.Errorf("a 304 fetched CAP messages")
	}
}

func TestRefreshCapsCAPPerRun(t *testing.T) {
	items := manyItems(100)
	api := &fakeSachet{list: ListResult{Items: items, ETag: `W/"new"`}}
	r, feeds, alerts, c := newSachetRefresher(t, api)

	run := func() Result {
		t.Helper()
		res, err := r.RefreshIfDue(context.Background(), FeedSachet)
		if err != nil || res.Skipped {
			t.Fatalf("run: %+v, %v", res, err)
		}
		c.set(c.now().Add(MinRefreshGap))
		return res
	}

	res := run()
	if res.Fetched != CAPPerRun || res.Stored != CAPPerRun || res.ItemsSeen != 100 {
		t.Fatalf("first run = %+v, want %d fetched and stored of 100 seen", res, CAPPerRun)
	}
	first := api.capReqs[0]
	if len(first) != CAPPerRun {
		t.Fatalf("first run requested %d messages, want %d", len(first), CAPPerRun)
	}
	// The newest 40 are the items 60 to 99, newest first.
	for i, it := range first {
		if want := items[99-i].GUID; it.GUID != want {
			t.Fatalf("first run item %d = %s, want %s", i, it.GUID, want)
		}
	}
	// With work left over the validators are not stored, so a 304 cannot hide it.
	if feeds.rows[FeedSachet].ETag != "" {
		t.Errorf("etag %q stored while 60 items were left", feeds.rows[FeedSachet].ETag)
	}

	if res := run(); res.Fetched != 40 {
		t.Fatalf("second run fetched %d, want 40", res.Fetched)
	}
	if feeds.rows[FeedSachet].ETag != "" {
		t.Errorf("etag stored while 20 items were left")
	}
	if res := run(); res.Fetched != 20 {
		t.Fatalf("third run fetched %d, want 20", res.Fetched)
	}
	if feeds.rows[FeedSachet].ETag != `W/"new"` {
		t.Errorf("etag = %q after the last item, want it stored", feeds.rows[FeedSachet].ETag)
	}
	if len(alerts.saved) != 100 {
		t.Errorf("%d alerts saved, want all 100", len(alerts.saved))
	}
	// A fourth run finds nothing to fetch.
	if res := run(); res.Fetched != 0 || len(api.capReqs) != 3 {
		t.Errorf("fourth run fetched %d with %d CAP batches in all, want none new", res.Fetched, len(api.capReqs))
	}
}

// TestRefreshPartialRunKeepsLastSuccess pins WR-S01 of the Phase 5 review: a
// run that leaves new items for the next run stores what it fetched but does not
// move last_success_at, so the page never calls the list fresh while an unseen
// item may still be in force. The run that finishes the work moves it.
func TestRefreshPartialRunKeepsLastSuccess(t *testing.T) {
	api := &fakeSachet{list: ListResult{Items: manyItems(CAPPerRun + 10), ETag: `W/"new"`}}
	r, feeds, alerts, c := newSachetRefresher(t, api)
	old := refreshT0.Add(-time.Hour)
	feeds.rows[FeedSachet].LastSuccessAt = &old

	res, err := r.RefreshIfDue(context.Background(), FeedSachet)
	if err != nil || !res.Partial || res.Stored != CAPPerRun {
		t.Fatalf("first run = %+v, %v; want a partial run that stored %d", res, err, CAPPerRun)
	}
	if got := feeds.rows[FeedSachet].LastSuccessAt; got == nil || !got.Equal(old) {
		t.Errorf("a partial run moved last_success_at to %v, want %v", got, old)
	}
	if feeds.rows[FeedSachet].ConsecutiveFailures != 0 || feeds.rows[FeedSachet].LastError != "" {
		t.Errorf("a partial run was recorded as a failure: %+v", feeds.rows[FeedSachet])
	}

	c.set(c.now().Add(MinRefreshGap))
	res, err = r.RefreshIfDue(context.Background(), FeedSachet)
	if err != nil || res.Partial || res.Fetched != 10 {
		t.Fatalf("second run = %+v, %v; want the last 10 fetched and the run complete", res, err)
	}
	if got := feeds.rows[FeedSachet].LastSuccessAt; got == nil || !got.Equal(c.now()) {
		t.Errorf("the completing run left last_success_at at %v, want %v", got, c.now())
	}
	if len(alerts.saved) != CAPPerRun+10 {
		t.Errorf("%d saved, want %d", len(alerts.saved), CAPPerRun+10)
	}
}

func TestRefreshRetriesWhatFailed(t *testing.T) {
	items := manyItems(3)
	failing := items[1].GUID
	api := &fakeSachet{list: ListResult{Items: items, ETag: `W/"e"`}}
	api.capErr = func(it Item) error {
		if it.GUID == failing {
			return &fetchError{class: "status", status: 503, msg: "status 503"}
		}
		return nil
	}
	r, feeds, alerts, c := newSachetRefresher(t, api)

	res, err := r.RefreshIfDue(context.Background(), FeedSachet)
	if err != nil || res.Stored != 2 {
		t.Fatalf("partial run: %+v, %v; want a success with 2 stored", res, err)
	}
	if feeds.rows[FeedSachet].ETag != "" {
		t.Errorf("etag stored although an item failed")
	}

	api.capErr = nil
	c.set(c.now().Add(MinRefreshGap))
	res, err = r.RefreshIfDue(context.Background(), FeedSachet)
	if err != nil || res.Stored != 1 || len(api.capReqs[1]) != 1 || api.capReqs[1][0].GUID != failing {
		t.Fatalf("retry: %+v, %v, requested %v; want only the failed item", res, err, api.capReqs[1])
	}
	if len(alerts.saved) != 3 {
		t.Errorf("%d saved, want 3", len(alerts.saved))
	}
	if feeds.rows[FeedSachet].ETag != `W/"e"` {
		t.Errorf("etag %q not stored after everything was fetched", feeds.rows[FeedSachet].ETag)
	}
}

func TestRefreshFailsOnlyWhenEveryFetchFailed(t *testing.T) {
	items := manyItems(2)
	api := &fakeSachet{list: ListResult{Items: items}}
	api.capErr = func(Item) error { return &fetchError{class: "timeout", msg: "timed out"} }
	r, feeds, _, _ := newSachetRefresher(t, api)

	res, err := r.RefreshIfDue(context.Background(), FeedSachet)
	if err == nil || res.ErrorClass != "timeout" {
		t.Fatalf("got %+v, %v; want a timeout failure", res, err)
	}
	if row := feeds.rows[FeedSachet]; row.LastSuccessAt != nil || row.ConsecutiveFailures != 1 {
		t.Errorf("row = %+v, want no success and one failure", row)
	}
}

func TestRefreshDoesNotRefetchWhatIsNotShown(t *testing.T) {
	items := manyItems(2)
	api := &fakeSachet{list: ListResult{Items: items}, notKept: map[string]bool{items[0].GUID: true}}
	r, _, alerts, c := newSachetRefresher(t, api)

	for i := 0; i < 3; i++ {
		if _, err := r.RefreshIfDue(context.Background(), FeedSachet); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		c.set(c.now().Add(MinRefreshGap))
	}
	if len(alerts.saved) != 1 {
		t.Errorf("%d saved, want only the kept message", len(alerts.saved))
	}
	if len(api.capReqs) != 1 {
		t.Errorf("%d CAP batches over three runs, want 1: an exercise message must not be fetched again", len(api.capReqs))
	}
}

func TestRefreshLeavesInvalidGUIDsAlone(t *testing.T) {
	items := append(manyItems(1), Item{GUID: "not-a-guid", PubDate: refreshT0})
	api := &fakeSachet{list: ListResult{Items: items}}
	r, _, _, _ := newSachetRefresher(t, api)
	if _, err := r.RefreshIfDue(context.Background(), FeedSachet); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(api.capReqs) != 1 || len(api.capReqs[0]) != 1 {
		t.Errorf("requests = %v, want one batch with the one valid item", api.capReqs)
	}
}
