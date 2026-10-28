package official

// The refresher is the one idempotent function that keeps the official tables
// current: RefreshIfDue(ctx, feed). The three triggers (the cron endpoint, the
// ticker while the host is awake, and the lazy kick on a read) all call it and
// nothing else. The host needs all three because D-11 pushes to a phone with
// nobody on the site, and the free host sleeps without traffic; the cron call
// wakes it, the ticker covers the time it is awake, and the lazy kick covers a
// cron that GitHub delayed or disabled. Because every trigger goes through this
// one function, a minimum gap per feed, one run at a time and a bound on CAP
// requests protect the upstream however the triggers behave.
//
// "Last checked" is last_success_at and nothing else: an attempt that failed
// does not move it, and a feed that suddenly returns no items after a full one
// is an anomaly, treated as a failure, so an emptied feed never reads as "no
// alerts". An alert leaves by its own expires_at, cancelled_at or
// superseded_at. Being missing from the RSS never removes one, because the RSS
// is a sliding 24 hour window.
//
// Logs hold the feed name, counts and one closed error word, never a URL, a
// query or any feed text.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/areas"
	sqlcgen "pinalert/internal/store/sqlc"
)

// Feed names one official source. The values are the keys of
// official_feed_status.
type Feed string

const (
	FeedSachet Feed = "sachet"
	FeedGdacs  Feed = "gdacs"
)

// MinRefreshGap is the shortest time between two attempts on one SACHET feed.
const MinRefreshGap = 4 * time.Minute

// CAPPerRun is the most CAP messages one run fetches. A cold start with more
// unseen items takes several runs and loses nothing, because the RSS window is
// 24 hours.
const CAPPerRun = 40

// AnomalyFloor is the item count of the last success from which an empty answer
// counts as a failure.
const AnomalyFloor = 10

// markTimeout bounds the write of a failure mark made after the run's own
// context ended.
const markTimeout = 5 * time.Second

// errAnomaly is the failure for a feed that emptied after a full one.
var errAnomaly = errors.New("official: the feed returned no items after a full one")

// ErrUnknownFeed is returned for a feed nothing was registered for.
var ErrUnknownFeed = errors.New("official: feed is not registered")

// FeedStatus is the stored state of one feed. LastSuccessAt is the time the page
// may call "last checked"; LastAttemptAt only paces the next attempt.
type FeedStatus struct {
	Feed                Feed
	LastAttemptAt       *time.Time
	LastSuccessAt       *time.Time
	LastError           string // one closed word, or empty
	ConsecutiveFailures int
	ETag                string
	LastModified        string
	ItemsSeen           int
}

// Result is what one RefreshIfDue call did. ETag and LastModified are the
// validators to store with a success; a fetcher that has none leaves them empty.
type Result struct {
	Feed       Feed
	Skipped    bool
	ItemsSeen  int
	Fetched    int
	Stored     int
	Cancelled  int
	ErrorClass string

	ETag         string
	LastModified string
	NotModified  bool
}

// Fetcher reads one feed. It is handed the stored status for its validators and
// its item count, and returns an error for a failed run; the refresher turns
// the error into a closed word.
type Fetcher interface {
	Fetch(ctx context.Context, st FeedStatus) (Result, error)
}

// TxBeginner starts a database transaction. *pgxpool.Pool satisfies it.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// VersionHook runs inside the transaction that stores an alert version, after
// the row and its areas are written. q is bound to that transaction. prev is the
// version this one replaced, or nil for a series seen the first time. An error
// rolls the whole version back, so it is retried by the next run.
type VersionHook func(ctx context.Context, q *sqlcgen.Queries, saved SavedVersion, prev *SavedVersion, now time.Time) error

// feedStore is the refresher's view of official_feed_status.
type feedStore interface {
	status(ctx context.Context, feed Feed) (FeedStatus, error)
	claim(ctx context.Context, feed Feed, now, cutoff time.Time) (bool, error)
	success(ctx context.Context, feed Feed, now time.Time, res Result) error
	failure(ctx context.Context, feed Feed, class string) error
}

// alertStore is what the SACHET fetcher needs of the alert tables.
type alertStore interface {
	pubDates(ctx context.Context, guids []string) (map[string]time.Time, error)
	save(ctx context.Context, v Version, p Placement, now time.Time) error
}

// Refresher refreshes the registered feeds. Safe for concurrent use.
type Refresher struct {
	db    TxBeginner
	q     *sqlcgen.Queries
	now   func() time.Time
	feeds feedStore
	store alertStore

	mu       sync.Mutex
	hooks    []VersionHook
	fetchers map[Feed]registration
	inflight map[Feed]*call
}

type registration struct {
	fetcher Fetcher
	gap     time.Duration
}

// call is one run in flight, shared by the callers that arrive meanwhile.
type call struct {
	done chan struct{}
	res  Result
	err  error
}

// RefresherOption configures a Refresher.
type RefresherOption func(*Refresher)

// WithRefresherClock replaces the clock, for tests.
func WithRefresherClock(now func() time.Time) RefresherOption {
	return func(r *Refresher) { r.now = now }
}

// WithVersionHook adds a hook that runs inside the transaction of every stored
// alert version. Hooks run in the order they were added.
func WithVersionHook(h VersionHook) RefresherOption {
	return func(r *Refresher) {
		r.mu.Lock()
		r.hooks = append(r.hooks, h)
		r.mu.Unlock()
	}
}

// WithSachet registers the SACHET feed with the standard minimum gap. ix places
// each alert on districts or a state.
func WithSachet(c *SachetClient, ix *areas.Index) RefresherOption {
	return func(r *Refresher) {
		r.Register(FeedSachet, newSachetFetcher(c, ix,
			func() alertStore { return r.store }, func() time.Time { return r.now() }), MinRefreshGap)
	}
}

// withStores replaces the database access, for unit tests that run no database.
func withStores(f feedStore, a alertStore) RefresherOption {
	return func(r *Refresher) { r.feeds, r.store = f, a }
}

// NewRefresher returns a Refresher over the database. Options apply in order, so
// WithRefresherClock should come before WithSachet.
func NewRefresher(db TxBeginner, q *sqlcgen.Queries, opts ...RefresherOption) *Refresher {
	r := &Refresher{
		db:       db,
		q:        q,
		now:      time.Now,
		fetchers: map[Feed]registration{},
		inflight: map[Feed]*call{},
	}
	pg := &pgStore{db: db, q: q, hooks: r.hookList}
	r.feeds, r.store = pg, pg
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *Refresher) hookList() []VersionHook {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]VersionHook(nil), r.hooks...)
}

// Register sets the fetcher of a feed and the shortest time between attempts.
func (r *Refresher) Register(feed Feed, f Fetcher, minGap time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fetchers[feed] = registration{fetcher: f, gap: minGap}
}

// Feeds lists the registered feeds in a fixed order: SACHET, then GDACS, then any
// other by name. The order is what the cron summary and the ticker follow.
func (r *Refresher) Feeds() []Feed {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Feed, 0, len(r.fetchers))
	for _, f := range []Feed{FeedSachet, FeedGdacs} {
		if _, ok := r.fetchers[f]; ok {
			out = append(out, f)
		}
	}
	var rest []Feed
	for f := range r.fetchers {
		if f != FeedSachet && f != FeedGdacs {
			rest = append(rest, f)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i] < rest[j] })
	return append(out, rest...)
}

// Status returns the stored state of a feed.
func (r *Refresher) Status(ctx context.Context, feed Feed) (FeedStatus, error) {
	return r.feeds.status(ctx, feed)
}

// RefreshIfDue refreshes the feed unless it was attempted less than its minimum
// gap ago. Only one run per feed is in flight: a caller that arrives meanwhile
// waits for that run and gets its result. A failed run returns its error and
// the Result with the closed word in ErrorClass.
func (r *Refresher) RefreshIfDue(ctx context.Context, feed Feed) (Result, error) {
	r.mu.Lock()
	reg, ok := r.fetchers[feed]
	if !ok {
		r.mu.Unlock()
		return Result{Feed: feed}, ErrUnknownFeed
	}
	if c, running := r.inflight[feed]; running {
		r.mu.Unlock()
		select {
		case <-c.done:
			return c.res, c.err
		case <-ctx.Done():
			return Result{Feed: feed, ErrorClass: ErrorClass(ctx.Err())}, ctx.Err()
		}
	}
	c := &call{done: make(chan struct{})}
	r.inflight[feed] = c
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.inflight, feed)
		r.mu.Unlock()
		close(c.done)
	}()
	c.res, c.err = r.run(ctx, feed, reg)
	return c.res, c.err
}

func (r *Refresher) run(ctx context.Context, feed Feed, reg registration) (Result, error) {
	now := r.now()
	st, err := r.feeds.status(ctx, feed)
	if err != nil {
		return Result{Feed: feed}, fmt.Errorf("official: reading the feed status: %w", err)
	}
	if st.LastAttemptAt != nil && now.Sub(*st.LastAttemptAt) < reg.gap {
		return Result{Feed: feed, Skipped: true}, nil
	}
	// The claim is the guard between instances: it succeeds for one caller only.
	claimed, err := r.feeds.claim(ctx, feed, now, now.Add(-reg.gap))
	if err != nil {
		return Result{Feed: feed}, fmt.Errorf("official: claiming the run: %w", err)
	}
	if !claimed {
		return Result{Feed: feed, Skipped: true}, nil
	}

	res, ferr := reg.fetcher.Fetch(ctx, st)
	res.Feed = feed
	if ferr == nil && !res.NotModified && res.ItemsSeen == 0 && st.ItemsSeen >= AnomalyFloor {
		ferr = errAnomaly
	}
	if ferr != nil {
		class := failureClass(ferr)
		res.ErrorClass = class
		mctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), markTimeout)
		defer cancel()
		if err := r.feeds.failure(mctx, feed, class); err != nil {
			log.Printf("official: feed=%s could not record a failure", feed)
		}
		log.Printf("official: feed=%s refresh failed class=%s", feed, class)
		return res, ferr
	}
	if err := r.feeds.success(ctx, feed, now, res); err != nil {
		res.ErrorClass = "network"
		return res, fmt.Errorf("official: recording the success: %w", err)
	}
	log.Printf("official: feed=%s refreshed items=%d fetched=%d stored=%d cancelled=%d notmodified=%t",
		feed, res.ItemsSeen, res.Fetched, res.Stored, res.Cancelled, res.NotModified)
	return res, nil
}

// failureClass is the closed word recorded for a failed run.
func failureClass(err error) string {
	if errors.Is(err, errAnomaly) {
		return "anomaly"
	}
	return ErrorClass(err)
}

// ---- SACHET ----

// sachetAPI is the part of SachetClient the fetcher uses; the tests swap it.
type sachetAPI interface {
	FetchList(ctx context.Context, etag, lastModified string) (ListResult, error)
	FetchCAPs(ctx context.Context, items []Item) []CAPResult
}

type sachetFetcher struct {
	api   sachetAPI
	ix    *areas.Index
	store func() alertStore
	now   func() time.Time

	mu       sync.Mutex
	notShown map[string]time.Time // guid to the pubDate of a message that is not kept
}

func newSachetFetcher(api sachetAPI, ix *areas.Index, store func() alertStore, now func() time.Time) *sachetFetcher {
	return &sachetFetcher{api: api, ix: ix, store: store, now: now, notShown: map[string]time.Time{}}
}

// Fetch reads the RSS list with the stored validators, fetches the CAP message
// of at most CAPPerRun new or changed items, newest first, and stores each kept
// version in its own transaction. A version whose CAP fetch or storage failed
// stays new and is tried again next run. The run fails, with the first failure's
// word, only when every message it needed failed.
//
// The validators are handed back only when the run finished its work. If some
// items were left over or failed, the old validators are kept, so the next
// request cannot be answered 304 and the rest is fetched.
func (f *sachetFetcher) Fetch(ctx context.Context, st FeedStatus) (Result, error) {
	list, err := f.api.FetchList(ctx, st.ETag, st.LastModified)
	if err != nil {
		return Result{}, err
	}
	if list.NotModified {
		return Result{ItemsSeen: st.ItemsSeen, ETag: st.ETag, LastModified: st.LastModified, NotModified: true}, nil
	}
	res := Result{ItemsSeen: len(list.Items), ETag: list.ETag, LastModified: list.LastModified}
	if len(list.Items) == 0 {
		return res, nil
	}

	guids := make([]string, 0, len(list.Items))
	for _, it := range list.Items {
		if ValidGUID(it.GUID) {
			guids = append(guids, it.GUID)
		}
	}
	stored, err := f.store().pubDates(ctx, guids)
	if err != nil {
		return res, err
	}
	need := f.selectNew(list.Items, stored)
	leftover := len(need) > CAPPerRun
	if leftover {
		need = need[:CAPPerRun]
	}

	failed := 0
	var first error
	if len(need) > 0 {
		for _, cr := range f.api.FetchCAPs(ctx, need) {
			if cr.Err != nil {
				failed++
				if first == nil {
					first = cr.Err
				}
				continue
			}
			res.Fetched++
			if !cr.Kept {
				f.remember(cr.Item)
				continue
			}
			p := Place(cr.Version, f.ix)
			if err := f.store().save(ctx, cr.Version, p, f.now()); err != nil {
				failed++
				if first == nil {
					first = err
				}
				continue
			}
			if cr.Version.MsgType == MsgCancel {
				res.Cancelled++
			} else {
				res.Stored++
			}
		}
	}
	if len(need) > 0 && failed == len(need) {
		return res, first
	}
	if leftover || failed > 0 {
		res.ETag, res.LastModified = st.ETag, st.LastModified
	}
	return res, nil
}

// selectNew returns the items whose CAP message must be fetched: a guid that is
// not stored yet or whose pubDate differs from the stored one, and that is not
// a message already seen not to be kept. Newest first, by pubDate. An item
// whose guid is not a valid id can never be requested and is left out.
func (f *sachetFetcher) selectNew(items []Item, stored map[string]time.Time) []Item {
	f.mu.Lock()
	defer f.mu.Unlock()

	current := make(map[string]bool, len(items))
	var need []Item
	for _, it := range items {
		if !ValidGUID(it.GUID) {
			continue
		}
		current[it.GUID] = true
		if seen, ok := f.notShown[it.GUID]; ok && seen.Equal(it.PubDate) {
			continue
		}
		if had, ok := stored[it.GUID]; ok && had.Equal(it.PubDate) {
			continue
		}
		need = append(need, it)
	}
	for guid := range f.notShown { // keep the memory the size of the window
		if !current[guid] {
			delete(f.notShown, guid)
		}
	}
	sort.SliceStable(need, func(i, j int) bool { return need[i].PubDate.After(need[j].PubDate) })
	return need
}

func (f *sachetFetcher) remember(it Item) {
	f.mu.Lock()
	f.notShown[it.GUID] = it.PubDate
	f.mu.Unlock()
}

// ---- database access ----

// pgStore is the database side of the refresher.
type pgStore struct {
	db    TxBeginner
	q     *sqlcgen.Queries
	hooks func() []VersionHook
}

func (s *pgStore) status(ctx context.Context, feed Feed) (FeedStatus, error) {
	row, err := s.q.GetFeedStatus(ctx, string(feed))
	if errors.Is(err, pgx.ErrNoRows) {
		return FeedStatus{Feed: feed}, nil
	}
	if err != nil {
		return FeedStatus{}, err
	}
	st := FeedStatus{
		Feed:                Feed(row.Feed),
		LastAttemptAt:       optionalTime(row.LastAttemptAt),
		LastSuccessAt:       optionalTime(row.LastSuccessAt),
		ConsecutiveFailures: int(row.ConsecutiveFailures),
		ETag:                row.LastEtag,
		LastModified:        row.LastModified,
		ItemsSeen:           int(row.ItemsSeen),
	}
	if row.LastError != nil {
		st.LastError = *row.LastError
	}
	return st, nil
}

func (s *pgStore) claim(ctx context.Context, feed Feed, now, cutoff time.Time) (bool, error) {
	n, err := s.q.MarkFeedAttempt(ctx, sqlcgen.MarkFeedAttemptParams{Feed: string(feed), Now: now, Cutoff: cutoff})
	return n == 1, err
}

func (s *pgStore) success(ctx context.Context, feed Feed, now time.Time, res Result) error {
	return s.q.MarkFeedSuccess(ctx, sqlcgen.MarkFeedSuccessParams{
		Feed: string(feed), Now: now, Etag: res.ETag, LastModified: res.LastModified, ItemsSeen: int32(res.ItemsSeen)})
}

func (s *pgStore) failure(ctx context.Context, feed Feed, class string) error {
	return s.q.MarkFeedFailure(ctx, sqlcgen.MarkFeedFailureParams{Feed: string(feed), ErrorClass: class})
}

func (s *pgStore) pubDates(ctx context.Context, guids []string) (map[string]time.Time, error) {
	out := make(map[string]time.Time, len(guids))
	if len(guids) == 0 {
		return out, nil
	}
	rows, err := s.q.StoredPubDates(ctx, guids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		var t time.Time
		if r.SourcePubdate.Valid {
			t = r.SourcePubdate.Time.UTC()
		}
		out[r.Guid] = t
	}
	return out, nil
}

// save stores one version and runs the hooks in the same transaction.
func (s *pgStore) save(ctx context.Context, v Version, p Placement, now time.Time) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("official: starting a transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // a no-op after Commit
	saved, prev, err := saveVersion(ctx, tx, s.q, v, p, now)
	if err != nil {
		return err
	}
	qt := s.q.WithTx(tx)
	for _, h := range s.hooks() {
		if err := h(ctx, qt, saved, prev, now); err != nil {
			return fmt.Errorf("official: version hook: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("official: committing a version: %w", err)
	}
	return nil
}
