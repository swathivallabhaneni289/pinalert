package official

// The official alerts service: the read side of the official feed. Alerts
// answers "which official alerts cover me, and can I trust that answer right
// now" for one signed in reader, and Kick keeps SACHET fresh on every read,
// quietly, so a host that sleeps between cron calls still catches up (the third
// refresh trigger, D-15, research G1 layer 3).
//
// The reader's position arrives only as arguments and is turned into a district
// at once. It is never stored and never logged, and the home area cell is read
// from the account row, used to find districts and never returned. Logs here
// hold the feed name and one closed class word.

import (
	"context"
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/areas"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// kickTimeout bounds one background refresh of each feed. Render's free host
// allows a request to run a minute, and the refresher bounds the CAP fetches
// itself. The feeds run at the same time, so each gets the whole 55 seconds.
const kickTimeout = 55 * time.Second

// Config is what the service knows about this deployment, used only to say who
// is calling NDMA.
type Config struct {
	ContactEmail string
	BaseURL      string
}

// Service reads the official alerts for a reader and refreshes them lazily.
// Safe for concurrent use.
type Service struct {
	refresher *Refresher
	q         *sqlcgen.Queries
	ix        *areas.Index
	now       func() time.Time
	kicking   kickSet
	// newTicker builds Run's ticker; nil means a real time.Ticker. Only tests set it.
	newTicker func(d time.Duration) (<-chan time.Time, func())
}

// kickSet records which feeds have a lazy refresh running, so each feed has one
// run at a time on its own. The zero value is ready to use. Load reports whether
// any feed is running.
type kickSet struct {
	mu   sync.Mutex
	busy map[Feed]bool
}

// begin claims the feed's slot; false means a run of that feed is in flight.
func (k *kickSet) begin(feed Feed) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.busy[feed] {
		return false
	}
	if k.busy == nil {
		k.busy = map[Feed]bool{}
	}
	k.busy[feed] = true
	return true
}

func (k *kickSet) end(feed Feed) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.busy, feed)
}

// Load reports whether a lazy refresh of any feed is running.
func (k *kickSet) Load() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.busy) > 0
}

// NewService builds the SACHET and GDACS clients and the refresher over the
// database. The options come after the SACHET registration, the push hook and the
// GDACS registration, so a later plan can add its own version hook or register
// another feed here without main.go changing, and a test can replace a fetcher by
// registering its own.
//
// The push hook (plan 05-11) queues phone alerts inside the transaction of each
// stored alert version, so it is wired here and nowhere else. GDACS (plan 05-21)
// is a second feed of the same refresher with its own status row and a 15 minute
// gap, so the cron door, the ticker and the lazy kick refresh it too; it never
// passes through the alert store or the push hook.
func NewService(pool TxBeginner, q *sqlcgen.Queries, cfg Config, opts ...RefresherOption) *Service {
	ix := areas.Default()
	client := NewSachetClient(ClientConfig{ContactEmail: cfg.ContactEmail, BaseURL: cfg.BaseURL})
	gdacs := NewGdacsClient(ClientConfig{ContactEmail: cfg.ContactEmail, BaseURL: cfg.BaseURL})
	all := make([]RefresherOption, 0, len(opts)+3)
	all = append(all, WithSachet(client, ix))
	all = append(all, WithVersionHook(PushEnqueueHook(areas.Default())))
	all = append(all, WithGdacs(gdacs))
	all = append(all, opts...)
	return newService(NewRefresher(pool, q, all...), q, ix)
}

func newService(r *Refresher, q *sqlcgen.Queries, ix *areas.Index) *Service {
	return &Service{refresher: r, q: q, ix: ix, now: time.Now}
}

// Refresher returns the refresher the service owns.
func (s *Service) Refresher() *Refresher { return s.refresher }

// TickerInterval is the pause between the sweeps Run makes while the host is
// awake (plan 05-12, D-15, research G1 layer 2). It is longer than MinRefreshGap,
// so a sweep right after a cron call finds the feed not due and does nothing.
const TickerInterval = 5 * time.Minute

// RefreshAll runs every registered feed through RefreshIfDue at the same time, so
// each feed gets the caller's whole deadline (SACHET's CAP fetches no longer eat
// into GDACS's minute) and each feed's minimum gap, single flight and claim apply
// however often this is called. The results come back in the refresher's fixed
// order (SACHET, then GDACS) whatever order the feeds finish in. A feed that
// failed carries its closed word in ErrorClass; one failure does not stop the
// others, and a panic inside one feed is recovered in that feed's goroutine and
// reported as the word network, because nothing outside a goroutine can recover
// its panic. The cron endpoint and the ticker call this and nothing else (the
// lazy kick runs the same feeds, each behind its own gate).
func (s *Service) RefreshAll(ctx context.Context) []Result {
	feeds := s.refresher.Feeds()
	out := make([]Result, len(feeds)) // each goroutine writes only its own index
	var wg sync.WaitGroup
	for i, feed := range feeds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if recover() != nil {
					log.Printf("official: refresh feed=%s class=panic", feed)
					out[i] = Result{Feed: feed, ErrorClass: "network"}
				}
			}()
			res, err := s.refresher.RefreshIfDue(ctx, feed)
			res.Feed = feed
			if err != nil && res.ErrorClass == "" {
				res.ErrorClass = ErrorClass(err)
			}
			out[i] = res
		}()
	}
	wg.Wait()
	return out
}

// Run keeps the feeds fresh while the host is awake: one sweep at start, because
// the free host cold starts, then one every TickerInterval until ctx ends. A
// sleeping host refreshes only when a request, usually the external cron, wakes
// it. Each sweep is bounded by kickTimeout and a panic in one is logged and the
// loop goes on. Like the moderation worker it takes a context so a test can end it.
func (s *Service) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	s.sweep(ctx)
	every := s.newTicker
	if every == nil {
		every = realTicker
	}
	ticks, stop := every(TickerInterval)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			s.sweep(ctx)
		}
	}
}

// sweep is one RefreshAll on a context of its own deadline. The feeds inside it
// run at the same time, each with its own recover in RefreshAll. The refresher
// logs a failed run itself (feed name and class word), and nobody is waiting for
// the result, so it is dropped.
func (s *Service) sweep(ctx context.Context) {
	defer func() {
		if recover() != nil {
			log.Printf("official: ticker sweep class=panic")
		}
	}()
	sctx, cancel := context.WithTimeout(ctx, kickTimeout)
	defer cancel()
	_ = s.RefreshAll(sctx)
}

// realTicker is a time.Ticker as a channel and a stop function.
func realTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// Alerts answers for one reader. lat and lon are both set or both nil; a point
// outside every district counts as no point. The home area is read from the
// account. The feed state comes from the stored last success only.
func (s *Service) Alerts(ctx context.Context, accountID int64, lat, lon *float64) (View, error) {
	now := s.now()
	status, err := s.refresher.Status(ctx, FeedSachet)
	if err != nil {
		return View{}, err
	}

	var reader *Area
	if lat != nil && lon != nil {
		if d, ok := s.ix.Locate(*lat, *lon); ok {
			reader = districtArea(d)
		}
	}
	var home *Area
	cell, err := s.q.GetAccountHomeCell(ctx, accountID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return View{}, err
	case cell != nil:
		home = s.homeArea(*cell)
	}

	hazardStatus, err := s.refresher.Status(ctx, FeedGdacs)
	if err != nil {
		return View{}, err
	}
	in := ViewInput{Now: now, Status: status, Reader: reader, Home: home, HazardStatus: hazardStatus}
	if state, _ := State(hazardStatus.LastSuccessAt, now); state == StateFresh && (reader != nil || home != nil) {
		hazards, err := s.hazardsNear(ctx, now, reader, home)
		if err != nil {
			return View{}, err
		}
		in.Hazards = hazards
	}
	if state, _ := State(status.LastSuccessAt, now); state != StateUnavailable && (reader != nil || home != nil) {
		var districts, states []int32
		for _, a := range []*Area{reader, home} {
			if a == nil {
				continue
			}
			for _, c := range a.DistrictCodes {
				districts = append(districts, int32(c))
			}
			states = append(states, int32(a.StateCode))
		}
		rows, err := s.q.OfficialAlertsInForce(ctx, sqlcgen.OfficialAlertsInForceParams{
			Now: now, DistrictCodes: districts, StateCodes: states,
		})
		if err != nil {
			return View{}, err
		}
		in.Alerts = make([]StoredAlert, 0, len(rows))
		for _, r := range rows {
			in.Alerts = append(in.Alerts, s.storedAlert(r))
		}
	}
	return BuildView(in), nil
}

// Kick starts a background refresh of every registered feed (SACHET and, since
// plan 05-21, GDACS), each only when its own last attempt is more than
// LazyKickAfter old. It returns at once, runs at most one refresh of a feed at a
// time on a context of its own (the request that triggered it may end first), and
// never reports an error; a failure is logged as the feed name and one class
// word. The refresher's own minimum gap per feed and single flight still apply,
// so however many readers kick, the upstream services are asked rarely.
func (s *Service) Kick() {
	for _, feed := range s.refresher.Feeds() {
		s.kickFeed(feed)
	}
}

func (s *Service) kickFeed(feed Feed) {
	if !s.kicking.begin(feed) {
		return
	}
	go func() {
		defer s.kicking.end(feed)
		defer func() {
			if recover() != nil {
				log.Printf("official: lazy refresh feed=%s class=panic", feed)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), kickTimeout)
		defer cancel()

		st, err := s.refresher.Status(ctx, feed)
		if err != nil {
			log.Printf("official: lazy refresh feed=%s class=%s", feed, ErrorClass(err))
			return
		}
		if st.LastAttemptAt != nil && s.now().Sub(*st.LastAttemptAt) < LazyKickAfter {
			return
		}
		// RefreshIfDue logs a failed run itself (feed name and class word), and
		// a lazy read has nobody to tell, so its result is dropped.
		_, _ = s.refresher.RefreshIfDue(ctx, feed)
	}()
}

// hazardsNear reads the current GDACS hazards around the reader's and the home
// districts. Each district's bounding box is a cheap first cut in SQL; the point
// is then placed on a district in Go, and only hazards that land in one of the
// wanted districts are returned, once each.
func (s *Service) hazardsNear(ctx context.Context, now time.Time, areasWanted ...*Area) ([]StoredHazard, error) {
	wanted := map[int]bool{}
	for _, a := range areasWanted {
		if a == nil {
			continue
		}
		for _, c := range a.DistrictCodes {
			wanted[c] = true
		}
	}
	seen := map[int64]bool{}
	var out []StoredHazard
	for code := range wanted {
		d, ok := s.ix.ByCode(code)
		if !ok {
			continue
		}
		rows, err := s.q.OfficialHazardsInBox(ctx, sqlcgen.OfficialHazardsInBoxParams{
			EndedAfter: now.Add(-GdacsEndedKeep),
			MinLat:     d.BBox[1], MaxLat: d.BBox[3],
			MinLon: d.BBox[0], MaxLon: d.BBox[2],
		})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if seen[r.ID] {
				continue
			}
			h := s.storedHazard(r)
			if !wanted[h.DistrictCode] {
				continue
			}
			seen[r.ID] = true
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// storedHazard converts one row to the view's input, placing its point on a
// district (0 when it is in none) and re-checking the report link.
func (s *Service) storedHazard(r sqlcgen.OfficialHazard) StoredHazard {
	h := StoredHazard{
		ID:           r.ID,
		EventType:    r.EventType,
		EventID:      r.EventID,
		Name:         r.Name,
		AlertLevel:   GdacsLevelWord(r.AlertLevel),
		Country:      r.Country,
		Lat:          r.Latitude,
		Lon:          r.Longitude,
		SeverityText: r.SeverityText,
		ReportURL:    SafeGdacsURL(r.ReportUrl),
	}
	if r.FromDate.Valid {
		h.FromDate = r.FromDate.Time.UTC()
	}
	if r.ToDate.Valid {
		h.ToDate = r.ToDate.Time.UTC()
	}
	if d, ok := s.ix.Locate(r.Latitude, r.Longitude); ok {
		h.DistrictCode = d.Code
	}
	return h
}

// PinsView is the answer to "which GDACS hazards may the map draw". Pins is empty
// unless State is fresh, so an old hazard is never drawn as current (ROBUST-05,
// ALERT-04). AgeSeconds is nil for a feed that never succeeded.
type PinsView struct {
	State      FreshState
	AgeSeconds *int64
	Pins       []StoredHazard
}

// Pins answers for the map. The hazards are the current ones: marked current by
// GDACS and not ended more than GdacsEndedKeep ago. It needs no reader position:
// the pins are the same for everyone.
func (s *Service) Pins(ctx context.Context) (PinsView, error) {
	now := s.now()
	st, err := s.refresher.Status(ctx, FeedGdacs)
	if err != nil {
		return PinsView{}, err
	}
	state, age := State(st.LastSuccessAt, now)
	out := PinsView{State: state, Pins: []StoredHazard{}}
	if st.LastSuccessAt != nil && !st.LastSuccessAt.IsZero() {
		secs := int64(age / time.Second)
		out.AgeSeconds = &secs
	}
	if state != StateFresh {
		return out, nil
	}
	rows, err := s.q.CurrentOfficialHazards(ctx, now.Add(-GdacsEndedKeep))
	if err != nil {
		return PinsView{}, err
	}
	for _, r := range rows {
		out.Pins = append(out.Pins, s.storedHazard(r))
	}
	return out, nil
}

// homeArea turns a stored 5 character home cell into the districts found at its
// centre and its four corners, so a cell on a district edge counts for both
// sides (research G3, biased toward telling the person). The name and state
// are those of the district at the centre. It is nil for anything that is not
// such a cell and for a cell outside every district. The cell itself is never
// returned.
func (s *Service) homeArea(cell string) *Area {
	districts := s.ix.CellDistricts(cell)
	if len(districts) == 0 {
		return nil
	}
	primary := districts[0]
	if lat, lon, ok := service.HomeAreaAnchor(cell); ok {
		if d, found := s.ix.Locate(lat, lon); found {
			primary = d
		}
	}
	a := districtArea(primary)
	a.DistrictCodes = make([]int, 0, len(districts))
	for _, d := range districts {
		a.DistrictCodes = append(a.DistrictCodes, d.Code)
	}
	return a
}

// districtArea is the area of one district in our own boundary data names.
func districtArea(d areas.District) *Area {
	return &Area{
		DistrictCodes: []int{d.Code},
		DistrictName:  d.Name,
		StateCode:     d.StateCode,
		StateName:     titleName(d.State),
	}
}

// storedAlert converts one in force row, with the boundary names of its
// districts, into the view's input.
func (s *Service) storedAlert(r sqlcgen.OfficialAlertsInForceRow) StoredAlert {
	a := StoredAlert{
		ID:                  r.ID,
		Source:              sourceSachet,
		Sender:              r.Sender,
		Issuer:              r.Issuer,
		Event:               r.Event,
		Severity:            r.Severity,
		SentAt:              r.SentAt.UTC(),
		ExpiresAt:           r.ExpiresAt.UTC(),
		ExpiresInferred:     r.ExpiresInferred,
		HeadlineEN:          r.HeadlineEn,
		HeadlineOriginal:    r.HeadlineOriginal,
		OriginalLang:        r.OriginalLang,
		DescriptionEN:       r.DescriptionEn,
		DescriptionOriginal: r.DescriptionOriginal,
		Instruction:         r.Instruction,
		AreaDesc:            r.AreaDesc,
		Tier:                Tier(r.PlacementTier),
	}
	for _, c := range r.DistrictCodes {
		a.DistrictCodes = append(a.DistrictCodes, int(c))
		name := ""
		if d, ok := s.ix.ByCode(int(c)); ok {
			name = d.Name
		}
		a.DistrictNames = append(a.DistrictNames, name)
	}
	for _, c := range r.StateCodes {
		a.StateCodes = append(a.StateCodes, int(c))
	}
	return a
}
