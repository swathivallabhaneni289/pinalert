package official

// The GDACS client, parser and fetcher (plan 05-21, research G4, D-12).
//
// GDACS is the Global Disaster Alert and Coordination System. This file reads
// its documented SEARCH API (https://www.gdacs.org/gdacsapi/api/events/geteventlist/SEARCH),
// two small requests per refresh:
//
//	A: Orange and Red events of the last 14 days (cyclone, flood, earthquake,
//	   volcano, wildfire), of which only the ones GDACS marks current are kept.
//	B: any level of earthquake, cyclone and flood in the last 3 days, of which
//	   only the points inside the South Asia box are kept, so a Green M4.8
//	   earthquake near the reader still shows.
//
// Droughts are dropped: a drought is one point for a multi country area, which
// would draw a misleading pin.
//
// GDACS asks clients to save the collection and compare its datetime before
// calling again. The 15 minute gap between refreshes (GdacsMinGap, enforced for
// every trigger by the refresher) is how that request is honoured. We
// acknowledge the source as "Global Disaster Alert and Coordination System,
// GDACS" wherever its events are shown. GDACS says its notifications for
// earthquakes, tsunamis and cyclones are automatic estimates that are not
// reviewed by people and that they are not meant to substitute or override any
// official information from local or national disaster management authorities.
// That is why a GDACS event is never a report, never raises the Nearby banner
// and never queues a phone alert: it is listed and drawn as an automatic
// estimate and nothing more.
//
// The client is a good citizen and a closed door. The only host it ever calls is
// the constant gdacsBaseURL (in this package's tests, the base the unexported
// seam constructor is given); every query is built with url.Values; redirects are
// refused; no URL found in an answer is ever requested; the body is capped at
// 2 MB and each request at 40 seconds. The two requests run at the same time, so
// a refresh takes as long as the slower one. Every failure is reduced to one closed
// word (see ErrorClass) and no error value holds a URL or a query. Feed text is
// stored as untrusted text: control characters removed and every field cut to a
// length cap, and a report link is kept only when it starts with
// https://www.gdacs.org/.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

// gdacsBaseURL is the only host production code ever calls for GDACS.
const gdacsBaseURL = "https://www.gdacs.org"

const gdacsSearchPath = "/gdacsapi/api/events/geteventlist/SEARCH"

// The client's bounds (research G4 and G9).
const (
	// GdacsMaxBytes caps one answer.
	GdacsMaxBytes = 2 << 20
	// GdacsTimeout bounds one request. Request A and request B run at the same
	// time, so a refresh takes as long as the slower of the two, inside the
	// caller's context (which still caps it). Raised from 15 s on 2026-10-29 by
	// quick 261029-6wb after the live service took 20.9 s for one request.
	GdacsTimeout = 40 * time.Second
	// GdacsMinGap is the shortest time between two refreshes of the GDACS feed.
	GdacsMinGap = 15 * time.Minute
	// GdacsEndedKeep is how long a pin and a row stay after the hazard's end date.
	GdacsEndedKeep = 48 * time.Hour

	gdacsPageSize     = 100
	gdacsTextMaxRunes = 200
	gdacsURLMaxLen    = 500
	// gdacsReportPrefix is the only start a stored report link may have.
	gdacsReportPrefix = "https://www.gdacs.org/"
)

// The South Asia box: a hazard of request B is kept only inside it. The edges
// are inside.
const (
	SouthAsiaMinLat = 5.0
	SouthAsiaMaxLat = 38.0
	SouthAsiaMinLon = 60.0
	SouthAsiaMaxLon = 100.0
)

// InSouthAsiaBox reports whether a point lies in the South Asia box, edges
// included.
func InSouthAsiaBox(lat, lon float64) bool {
	return lat >= SouthAsiaMinLat && lat <= SouthAsiaMaxLat && lon >= SouthAsiaMinLon && lon <= SouthAsiaMaxLon
}

// Hazard is one GDACS event as stored. The point is the event's centroid; the
// polygon behind it is never fetched.
type Hazard struct {
	EventType    string
	EventID      int64
	EpisodeID    int64
	Name         string
	AlertLevel   string // Red, Orange or Green
	Country      string
	Lat, Lon     float64
	FromDate     time.Time
	ToDate       time.Time
	IsCurrent    bool
	SeverityText string
	ReportURL    string // empty unless it starts with https://www.gdacs.org/
	DateModified time.Time
}

// HazardWord is the plain word for a GDACS event type.
func HazardWord(eventType string) string {
	switch eventType {
	case "EQ":
		return "Earthquake"
	case "TC":
		return "Cyclone"
	case "FL":
		return "Flood"
	case "VO":
		return "Volcano"
	case "WF":
		return "Wildfire"
	}
	return "Hazard"
}

// GdacsLevelWord is the colour word of a GDACS alert level: Red, Orange or Green.
// Any other value reads as Green, the lowest.
func GdacsLevelWord(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "red":
		return "Red"
	case "orange":
		return "Orange"
	}
	return "Green"
}

// SafeGdacsURL returns a GDACS page link when s may be put in front of a reader
// and an empty string otherwise. It must start with https://www.gdacs.org/ (so
// http, another host, a look alike host, a user-info trick and javascript: all
// fail) and hold only printable ASCII without a space, backslash, quote or
// angle bracket.
func SafeGdacsURL(s string) string {
	if !strings.HasPrefix(s, gdacsReportPrefix) || len(s) > gdacsURLMaxLen {
		return ""
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= 0x20 || c >= 0x7f || c == '\\' || c == '"' || c == '<' || c == '>' || c == '\'' {
			return ""
		}
	}
	return s
}

var gdacsEventType = regexp.MustCompile(`^[A-Z]{2}$`)

// gdacsFlag reads iscurrent, which GDACS sends as the string "true" or "false"
// and which a different version could send as a boolean.
type gdacsFlag bool

func (f *gdacsFlag) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	*f = gdacsFlag(strings.EqualFold(s, "true"))
	return nil
}

type gdacsFeature struct {
	Geometry struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		EventType    string    `json:"eventtype"`
		EventID      int64     `json:"eventid"`
		EpisodeID    int64     `json:"episodeid"`
		Name         string    `json:"name"`
		AlertLevel   string    `json:"alertlevel"`
		Country      string    `json:"country"`
		FromDate     string    `json:"fromdate"`
		ToDate       string    `json:"todate"`
		DateModified string    `json:"datemodified"`
		IsCurrent    gdacsFlag `json:"iscurrent"`
		SeverityData struct {
			SeverityText string `json:"severitytext"`
		} `json:"severitydata"`
		URL struct {
			Report string `json:"report"`
		} `json:"url"`
	} `json:"properties"`
}

// ParseGdacs reads a SEARCH answer: a GeoJSON feature collection. A body that
// is not JSON, or that has no features list (GDACS answers an error as a single
// message), is ErrParse. A feature that cannot be used (no point, a point out of
// range, an unknown kind of event type, no id) is skipped and the rest are kept.
// The reader is not limited here: the client limits the body it hands over.
func ParseGdacs(r io.Reader) ([]Hazard, error) {
	var doc struct {
		Features json.RawMessage `json:"features"`
	}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: gdacs: the answer is not JSON", ErrParse)
	}
	var raw []json.RawMessage
	if len(doc.Features) == 0 || json.Unmarshal(doc.Features, &raw) != nil || raw == nil {
		return nil, fmt.Errorf("%w: gdacs: the answer has no features list", ErrParse)
	}
	out := make([]Hazard, 0, len(raw))
	for _, rf := range raw {
		var f gdacsFeature
		if json.Unmarshal(rf, &f) != nil {
			continue
		}
		if h, ok := f.hazard(); ok {
			out = append(out, h)
		}
	}
	return out, nil
}

func (f gdacsFeature) hazard() (Hazard, bool) {
	p := f.Properties
	if f.Geometry.Type != "Point" || len(f.Geometry.Coordinates) < 2 {
		return Hazard{}, false
	}
	lon, lat := f.Geometry.Coordinates[0], f.Geometry.Coordinates[1] // GeoJSON order
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) ||
		lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return Hazard{}, false
	}
	if !gdacsEventType.MatchString(p.EventType) || p.EventID <= 0 {
		return Hazard{}, false
	}
	return Hazard{
		EventType:    p.EventType,
		EventID:      p.EventID,
		EpisodeID:    p.EpisodeID,
		Name:         gdacsText(p.Name),
		AlertLevel:   GdacsLevelWord(p.AlertLevel),
		Country:      gdacsText(p.Country),
		Lat:          lat,
		Lon:          lon,
		FromDate:     parseGdacsTime(p.FromDate),
		ToDate:       parseGdacsTime(p.ToDate),
		IsCurrent:    bool(p.IsCurrent),
		SeverityText: gdacsText(p.SeverityData.SeverityText),
		ReportURL:    SafeGdacsURL(p.URL.Report),
		DateModified: parseGdacsTime(p.DateModified),
	}, true
}

// gdacsText cleans feed text and cuts it to the length cap, on one line.
func gdacsText(s string) string {
	return strings.Join(strings.Fields(CleanText(s, gdacsTextMaxRunes)), " ")
}

// parseGdacsTime reads a GDACS time, which has no zone ("2026-10-26T00:00:00")
// and is UTC. The zero time means it could not be read.
func parseGdacsTime(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02T15:04:05", time.RFC3339, "2006-01-02T15:04:05.999999999", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// filterGdacsA keeps what request A is for: events GDACS marks current, never a
// drought.
func filterGdacsA(hs []Hazard) []Hazard {
	out := make([]Hazard, 0, len(hs))
	for _, h := range hs {
		if h.EventType != "DR" && h.IsCurrent {
			out = append(out, h)
		}
	}
	return out
}

// endedGdacsA lists the events request A marks not current, never a drought, that
// the kept list does not hold.
func endedGdacsA(a, kept []Hazard) []Hazard {
	type key struct {
		t  string
		id int64
	}
	in := make(map[key]bool, len(kept))
	for _, h := range kept {
		in[key{h.EventType, h.EventID}] = true
	}
	var out []Hazard
	for _, h := range a {
		if h.EventType != "DR" && !h.IsCurrent && !in[key{h.EventType, h.EventID}] {
			out = append(out, h)
		}
	}
	return out
}

// filterGdacsB keeps what request B is for: points inside the South Asia box,
// never a drought. Whether an event is current is stored as it is, so a row
// that stopped being current is updated and leaves the map.
func filterGdacsB(hs []Hazard) []Hazard {
	out := make([]Hazard, 0, len(hs))
	for _, h := range hs {
		if h.EventType != "DR" && InSouthAsiaBox(h.Lat, h.Lon) {
			out = append(out, h)
		}
	}
	return out
}

// mergeHazards joins the two lists in order, once per (event type, event id).
// For an event in both, the copy modified later wins; on a tie the earlier list's
// copy stays.
func mergeHazards(lists ...[]Hazard) []Hazard {
	type key struct {
		t  string
		id int64
	}
	var out []Hazard
	at := map[key]int{}
	for _, list := range lists {
		for _, h := range list {
			k := key{h.EventType, h.EventID}
			if i, ok := at[k]; ok {
				if h.DateModified.After(out[i].DateModified) {
					out[i] = h
				}
				continue
			}
			at[k] = len(out)
			out = append(out, h)
		}
	}
	return out
}

// SearchRequest is one SEARCH call: the event types, the days it covers (UTC
// dates, both inclusive) and the alert levels.
type SearchRequest struct {
	EventTypes []string // EQ, TC, FL, VO, WF
	From, To   time.Time
	Levels     []string // red, orange, green
}

// GdacsClient reads the GDACS SEARCH API. Safe for concurrent use.
type GdacsClient struct {
	base       string
	userAgent  string
	httpClient *http.Client
	timeout    time.Duration
}

// NewGdacsClient returns a client for the real GDACS host. cfg only says who is
// calling, in the User-Agent.
func NewGdacsClient(cfg ClientConfig) *GdacsClient {
	return newGdacsClientWithBase(gdacsBaseURL, cfg, 0)
}

// newGdacsClientWithBase is the test seam: an explicit base for a fake upstream
// and, when timeout is positive, a timeout for each request. No production code
// calls it with anything but the constant.
func newGdacsClientWithBase(base string, cfg ClientConfig, timeout time.Duration) *GdacsClient {
	c := &GdacsClient{
		base:      strings.TrimRight(base, "/"),
		userAgent: UserAgent(cfg),
		httpClient: &http.Client{
			// A redirect is never followed: the 3xx answer is returned as it is
			// and reported as a status error.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		timeout: GdacsTimeout,
	}
	if timeout > 0 {
		c.timeout = timeout
	}
	return c
}

// gdacsErr tags an error from the shared helpers as GDACS's, so the text reads
// "official: gdacs ..." and the class word is kept.
func gdacsErr(err error) error {
	var fe *fetchError
	if errors.As(err, &fe) {
		cp := *fe
		cp.feed = "gdacs"
		return &cp
	}
	return err
}

// Search makes one bounded SEARCH call and parses the answer. It does no
// filtering.
func (c *GdacsClient) Search(ctx context.Context, req SearchRequest) ([]Hazard, error) {
	q := url.Values{}
	q.Set("eventlist", strings.Join(req.EventTypes, ";"))
	q.Set("fromdate", req.From.UTC().Format("2006-01-02"))
	q.Set("todate", req.To.UTC().Format("2006-01-02"))
	q.Set("alertlevel", strings.Join(req.Levels, ";"))
	q.Set("pagenumber", "1")
	q.Set("pagesize", strconv.Itoa(gdacsPageSize))
	target := c.base + gdacsSearchPath + "?" + q.Encode()

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	hreq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, gdacsErr(&fetchError{class: "network", msg: "bad request"})
	}
	hreq.Header.Set("User-Agent", c.userAgent)
	hreq.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(hreq)
	if err != nil {
		return nil, gdacsErr(transportError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, gdacsErr(&fetchError{class: "status", status: resp.StatusCode, msg: fmt.Sprintf("status %d", resp.StatusCode)})
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(GdacsMaxBytes)+1))
	if err != nil {
		return nil, gdacsErr(transportError(err))
	}
	if len(body) > GdacsMaxBytes {
		return nil, gdacsErr(parseError("body too large"))
	}
	hs, err := ParseGdacs(bytes.NewReader(body))
	if err != nil {
		return nil, gdacsErr(parseError("unreadable answer"))
	}
	return hs, nil
}

// searchSafely is Search with a recover, because it runs in a goroutine of its
// own, where no caller's recover can see a panic. A panic is reported as a
// network failure and never crashes the process.
func (c *GdacsClient) searchSafely(ctx context.Context, req SearchRequest) (hs []Hazard, err error) {
	defer func() {
		if recover() != nil {
			hs, err = nil, gdacsErr(&fetchError{class: "network", msg: "request failed"})
		}
	}()
	return c.Search(ctx, req)
}

// GdacsResult is what one refresh read: the hazards worth storing and how many
// raw features the two answers held.
type GdacsResult struct {
	Hazards []Hazard
	Seen    int
	// Ended are the events request A returned marked not current and that are
	// not in Hazards. A row stored for one is marked not current, so it leaves
	// the map also when request B does not return it (WR-S03 of the Phase 5
	// review). Nothing new is stored for them.
	Ended []Hazard
}

// FetchHazards runs request A and request B at the given time, at the same time
// as each other, filters, drops droughts and joins the two lists once per event.
// If either request fails the whole call fails and returns no hazards, so a
// refresh never claims a check that half of it did not make. The error returned
// is the first one that happened, and at that moment the other request is
// cancelled, so the call returns at the failure and does not wait for the other
// request's timeout. Both requests are always finished with before it returns.
func (c *GdacsClient) FetchHazards(ctx context.Context, now time.Time) (GdacsResult, error) {
	now = now.UTC()
	reqA := SearchRequest{
		EventTypes: []string{"TC", "FL", "EQ", "VO", "WF"},
		From:       now.AddDate(0, 0, -14), To: now,
		Levels: []string{"orange", "red"},
	}
	reqB := SearchRequest{
		EventTypes: []string{"EQ", "TC", "FL"},
		From:       now.AddDate(0, 0, -3), To: now,
		Levels: []string{"green", "orange", "red"},
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
		a, b     []Hazard
	)
	run := func(req SearchRequest, into *[]Hazard) {
		defer wg.Done()
		hs, err := c.searchSafely(ctx, req)
		if err != nil {
			// Only the first failure is kept: the other request's later
			// "context canceled" is a result of this cancel, not a cause.
			once.Do(func() {
				firstErr = err
				cancel()
			})
			return
		}
		*into = hs
	}
	wg.Add(2)
	go run(reqA, &a)
	go run(reqB, &b)
	wg.Wait()
	if firstErr != nil {
		return GdacsResult{}, firstErr
	}
	merged := mergeHazards(filterGdacsA(a), filterGdacsB(b))
	return GdacsResult{
		Hazards: merged,
		Seen:    len(a) + len(b),
		Ended:   endedGdacsA(a, merged),
	}, nil
}

// ---- the fetcher ----

// gdacsAPI is the part of GdacsClient the fetcher uses; the tests swap it.
type gdacsAPI interface {
	FetchHazards(ctx context.Context, now time.Time) (GdacsResult, error)
}

// hazardStore is what the fetcher needs of the hazards table.
type hazardStore interface {
	upsert(ctx context.Context, h Hazard, now time.Time) error
	end(ctx context.Context, h Hazard, now time.Time) error
}

// gdacsFetcher is the Fetcher of the gdacs feed. It never goes through the
// alert store or a version hook, so a GDACS event can neither be a banner row
// nor queue a phone alert, and it touches no table but official_hazards.
type gdacsFetcher struct {
	api   gdacsAPI
	store func() hazardStore
	now   func() time.Time
}

func newGdacsFetcher(api gdacsAPI, store func() hazardStore, now func() time.Time) *gdacsFetcher {
	return &gdacsFetcher{api: api, store: store, now: now}
}

// Fetch reads both SEARCH requests and upserts the kept hazards. ItemsSeen and
// Fetched are the raw features the two answers held (not the kept ones), so a
// quiet week with nothing worth keeping is a clean success and is never taken for
// an emptied feed. Stored is the number of hazards written. A failed request
// fails the run before anything is written.
func (f *gdacsFetcher) Fetch(ctx context.Context, _ FeedStatus) (Result, error) {
	now := f.now()
	got, err := f.api.FetchHazards(ctx, now)
	if err != nil {
		return Result{}, err
	}
	res := Result{ItemsSeen: got.Seen, Fetched: got.Seen}
	store := f.store()
	for _, h := range got.Hazards {
		if err := store.upsert(ctx, h, now); err != nil {
			return res, &fetchError{class: "network", msg: "could not store a hazard", cause: err, feed: "gdacs"}
		}
		res.Stored++
	}
	for _, h := range got.Ended {
		if err := store.end(ctx, h, now); err != nil {
			return res, &fetchError{class: "network", msg: "could not store a hazard", cause: err, feed: "gdacs"}
		}
	}
	return res, nil
}

// pgHazards is the database side of the fetcher.
type pgHazards struct{ q *sqlcgen.Queries }

func (s pgHazards) upsert(ctx context.Context, h Hazard, now time.Time) error {
	p := sqlcgen.UpsertOfficialHazardParams{
		EventType:    h.EventType,
		EventID:      h.EventID,
		Name:         h.Name,
		AlertLevel:   h.AlertLevel,
		Country:      h.Country,
		Latitude:     h.Lat,
		Longitude:    h.Lon,
		FromDate:     optionalTimestamptz(h.FromDate),
		ToDate:       optionalTimestamptz(h.ToDate),
		IsCurrent:    h.IsCurrent,
		SeverityText: h.SeverityText,
		ReportUrl:    h.ReportURL,
		DateModified: optionalTimestamptz(h.DateModified),
		Now:          now,
	}
	if h.EpisodeID > 0 {
		ep := h.EpisodeID
		p.EpisodeID = &ep
	}
	return s.q.UpsertOfficialHazard(ctx, p)
}

// end marks a stored row of the event not current. A row that is not stored
// stays not stored.
func (s pgHazards) end(ctx context.Context, h Hazard, now time.Time) error {
	return s.q.EndOfficialHazard(ctx, sqlcgen.EndOfficialHazardParams{EventType: h.EventType, EventID: h.EventID, Now: now})
}

// optionalTimestamptz is NULL for the zero time.
func optionalTimestamptz(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

// WithGdacs registers the GDACS feed with its 15 minute minimum gap, so the
// cron endpoint, the ticker and the lazy kick refresh it through RefreshIfDue
// like every feed, with a status row of its own.
func WithGdacs(c *GdacsClient) RefresherOption {
	return func(r *Refresher) {
		r.Register(FeedGdacs, newGdacsFetcher(c,
			func() hazardStore { return pgHazards{q: r.q} }, func() time.Time { return r.now() }), GdacsMinGap)
	}
}
