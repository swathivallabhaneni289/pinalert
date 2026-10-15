// votes_ratelimit_e2e_test.go is V-11 for plan 04-09: it proves the vote
// throttle end to end over the real router and a real Postgres. The first and
// last tests run on the production defaults (api.RateLimits{} resolves to
// api.RateLimitsDefault); the others set only the bucket they study. Every
// server uses a limiter clock that moves forward on every read, so a refund
// that read the clock again after the reservation would be caught. Every
// package level identifier carries the p09 prefix.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// p09T0 is the instant every p09Clock starts at.
var p09T0 = time.Date(2026, 1, 6, 9, 0, 0, 0, time.UTC)

// p09Lat and p09Lon are the report point every test in this file posts at.
const p09Lat, p09Lon = 12.9716, 77.5946

// p09Clock is a limiter clock. Now returns the current value and then adds one
// tick, so time moves on every read. Advance jumps it forward.
type p09Clock struct {
	mu   sync.Mutex
	cur  time.Time
	tick time.Duration
}

func p09NewClock(tick time.Duration) *p09Clock {
	return &p09Clock{cur: p09T0, tick: tick}
}

func (c *p09Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.cur
	c.cur = c.cur.Add(c.tick)
	return now
}

func (c *p09Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cur = c.cur.Add(d)
}

// p09Tweak builds the tweak hook for newE2EServerWith. It injects the clock and
// opens the request link limiter (each test verifies several accounts), then
// runs mutate when it is non nil.
func p09Tweak(clk *p09Clock, mutate func(d *api.Deps)) func(d *api.Deps, q *sqlcgen.Queries) {
	return func(d *api.Deps, _ *sqlcgen.Queries) {
		d.LimiterClock = clk.Now
		d.RequestLinkRateLimit = api.RequestLinkRateLimit{Burst: 100, Every: time.Millisecond}
		if mutate != nil {
			mutate(d)
		}
	}
}

// p09LogSink collects the standard logger's output.
type p09LogSink struct {
	mu sync.Mutex
	sb strings.Builder
}

func (s *p09LogSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sb.Write(p)
}

func (s *p09LogSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sb.String()
}

// p09CaptureLog redirects the standard logger into a sink until the test ends.
func p09CaptureLog(t *testing.T) *p09LogSink {
	t.Helper()
	sink := &p09LogSink{}
	prev := log.Writer()
	log.SetOutput(sink)
	t.Cleanup(func() { log.SetOutput(prev) })
	return sink
}

// p09Vote POSTs a JSON body to baseURL/api/reports/<idPath>/<action>. idPath is
// raw, so a leading zero id is possible.
func p09Vote(t *testing.T, client *http.Client, baseURL, idPath, action string, body any) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling vote body: %v", err)
	}
	url := baseURL + "/api/reports/" + idPath + "/" + action
	resp, err := client.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// p09NearBody is a vote body at the report point.
func p09NearBody() map[string]float64 {
	return map[string]float64{"latitude": p09Lat, "longitude": p09Lon}
}

// p09FarBody is a vote body about 5 km north of the report point.
func p09FarBody() map[string]float64 {
	return map[string]float64{"latitude": p09Lat + 0.045, "longitude": p09Lon}
}

// p09RateBody is a decoded 429.
type p09RateBody struct {
	Status            int
	RetryAfterHeader  string
	ContentType       string
	Keys              []string
	Field             string
	Message           string
	RetryAfterSeconds int
	Serious           bool
	HasNumber         bool
}

// p09Decode429 decodes a rate limited response and fails unless it is a 429.
func p09Decode429(t *testing.T, resp *http.Response) p09RateBody {
	t.Helper()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading 429 body: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429, body %s", resp.StatusCode, raw)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decoding 429 body %s: %v", raw, err)
	}
	out := p09RateBody{
		Status:           resp.StatusCode,
		RetryAfterHeader: resp.Header.Get("Retry-After"),
		ContentType:      resp.Header.Get("Content-Type"),
	}
	for k := range top {
		out.Keys = append(out.Keys, k)
	}
	sort.Strings(out.Keys)
	_, out.HasNumber = top["emergency_number"]
	var parsed struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
		RetryAfterSeconds int  `json:"retry_after_seconds"`
		Serious           bool `json:"serious"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("decoding 429 fields %s: %v", raw, err)
	}
	out.Field = parsed.Error.Field
	out.Message = parsed.Error.Message
	out.RetryAfterSeconds = parsed.RetryAfterSeconds
	out.Serious = parsed.Serious
	return out
}

// p09AccountID reads the verified account id behind the client's session.
func p09AccountID(t *testing.T, pool *pgxpool.Pool, client *http.Client, srvURL string) int64 {
	t.Helper()
	sid := sessionIDFromJar(t, client, srvURL)
	var id int64
	if err := pool.QueryRow(context.Background(), `SELECT account_id FROM sessions WHERE session_id = $1`, sid).Scan(&id); err != nil {
		t.Fatalf("reading account id for session: %v", err)
	}
	return id
}

// p09Status closes the body and returns the status code.
func p09Status(resp *http.Response) int {
	resp.Body.Close()
	return resp.StatusCode
}

// p09BlockLines returns the captured lines that hold a rate limit block.
func p09BlockLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "bucket=") {
			out = append(out, line)
		}
	}
	return out
}

func TestVoteRateLimit(t *testing.T) {
	clk := p09NewClock(time.Millisecond)
	srv, mailer, pool := newE2EServerWith(t, p09Tweak(clk, func(d *api.Deps) { d.RateLimits = api.RateLimits{} }))

	a := newVerifiedClient(t, srv, mailer, "p09-rate-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p09-rate-b@example.com")
	c := newVerifiedClient(t, srv, mailer, "p09-rate-c@example.com")
	report := submitReportGetID(t, a, srv.URL, floodReportBody(p09Lat, p09Lon))
	rid := fmt.Sprint(report)
	bID := p09AccountID(t, pool, b, srv.URL)
	cID := p09AccountID(t, pool, c, srv.URL)

	sink := p09CaptureLog(t)

	for i := 0; i < 20; i++ {
		resp := p09Vote(t, b, srv.URL, fmt.Sprint(900001+i), "confirm", p09NearBody())
		if got := p09Status(resp); got != http.StatusNotFound {
			t.Fatalf("vote %d on a missing report = %d, want 404", i+1, got)
		}
	}

	rb := p09Decode429(t, p09Vote(t, b, srv.URL, rid, "confirm", p09NearBody()))
	if rb.RetryAfterHeader != "15" {
		t.Errorf("Retry-After = %q, want 15", rb.RetryAfterHeader)
	}
	if !strings.HasPrefix(rb.ContentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", rb.ContentType)
	}
	if want := []string{"error", "retry_after_seconds", "serious"}; strings.Join(rb.Keys, ",") != strings.Join(want, ",") {
		t.Errorf("top level keys = %v, want %v", rb.Keys, want)
	}
	if rb.Field != "rate_vote" || rb.RetryAfterSeconds != 15 || rb.Serious {
		t.Errorf("field=%q retry_after_seconds=%d serious=%v, want rate_vote 15 false", rb.Field, rb.RetryAfterSeconds, rb.Serious)
	}
	const wantVote = "You're voting very quickly. You can vote again in 15 seconds."
	if rb.Message != wantVote || rb.Message != service.VoteLimitMessage(15*time.Second) {
		t.Errorf("message = %q, want %q", rb.Message, wantVote)
	}
	if want := fmt.Sprintf("bucket=vote account=%d wait_seconds=15", bID); !strings.Contains(sink.String(), want) {
		t.Errorf("log lacks %q, got %q", want, sink.String())
	}

	clk.Advance(15 * time.Second)
	if got := p09Status(p09Vote(t, b, srv.URL, rid, "confirm", p09NearBody())); got != http.StatusOK {
		t.Errorf("confirm after the wait = %d, want 200", got)
	}

	if got := p09Status(p09Vote(t, c, srv.URL, rid, "confirm", p09NearBody())); got != http.StatusOK {
		t.Fatalf("voter C confirm = %d, want 200", got)
	}
	cb := p09Decode429(t, p09Vote(t, c, srv.URL, rid, "dispute", p09NearBody()))
	if cb.Field != "rate_vote_change" || cb.RetryAfterHeader != "30" {
		t.Errorf("dispute after confirm: field=%q Retry-After=%q, want rate_vote_change 30", cb.Field, cb.RetryAfterHeader)
	}
	const wantChange = "You changed this vote a moment ago. You can change it again in 30 seconds."
	if cb.Message != wantChange || cb.Message != service.VoteChangeLimitMessage(30*time.Second) {
		t.Errorf("message = %q, want %q", cb.Message, wantChange)
	}
	if want := fmt.Sprintf("bucket=vote_change account=%d wait_seconds=30", cID); !strings.Contains(sink.String(), want) {
		t.Errorf("log lacks %q, got %q", want, sink.String())
	}
	if got := p09Status(p09Vote(t, c, srv.URL, rid, "resolve", p09NearBody())); got == http.StatusTooManyRequests {
		t.Errorf("resolve after confirm answered 429, want the other vote kind unblocked")
	}

	lines := p09BlockLines(sink.String())
	if len(lines) == 0 {
		t.Fatalf("no block lines captured")
	}
	for _, line := range lines {
		for _, banned := range []string{"p09-rate-b@example.com", "p09-rate-c@example.com", "p09-rate-a@example.com", "Water rising"} {
			if strings.Contains(line, banned) {
				t.Errorf("block line %q holds %q", line, banned)
			}
		}
	}
}

func TestVoteRateLimitChangeCooldown(t *testing.T) {
	clk := p09NewClock(time.Millisecond)
	srv, mailer, _ := newE2EServerWith(t, p09Tweak(clk, func(d *api.Deps) { d.RateLimits = api.RateLimits{} }))

	a := newVerifiedClient(t, srv, mailer, "p09-cool-a@example.com")
	c := newVerifiedClient(t, srv, mailer, "p09-cool-c@example.com")
	r1 := submitReportGetID(t, a, srv.URL, floodReportBody(p09Lat, p09Lon))
	r2 := submitReportGetID(t, a, srv.URL, floodReportBody(p09Lat, p09Lon))
	id1, id2 := fmt.Sprint(r1), fmt.Sprint(r2)

	if got := p09Status(p09Vote(t, c, srv.URL, id1, "confirm", p09NearBody())); got != http.StatusOK {
		t.Fatalf("confirm R1 = %d, want 200", got)
	}
	again := p09Decode429(t, p09Vote(t, c, srv.URL, id1, "confirm", p09NearBody()))
	if again.Field != "rate_vote_change" || again.RetryAfterHeader != "30" {
		t.Errorf("second confirm: field=%q Retry-After=%q, want rate_vote_change 30", again.Field, again.RetryAfterHeader)
	}
	lead := p09Decode429(t, p09Vote(t, c, srv.URL, "0"+id1, "dispute", p09NearBody()))
	if lead.Field != "rate_vote_change" {
		t.Errorf("dispute on a leading zero path: field=%q, want rate_vote_change", lead.Field)
	}
	if got := p09Status(p09Vote(t, c, srv.URL, id2, "confirm", p09NearBody())); got != http.StatusOK {
		t.Errorf("confirm R2 = %d, want 200 (another report is not blocked)", got)
	}
	if got := p09Status(p09Vote(t, c, srv.URL, id1, "resolve", p09NearBody())); got != http.StatusOK {
		t.Fatalf("resolve R1 = %d, want 200", got)
	}
	reopen := p09Decode429(t, p09Vote(t, c, srv.URL, id1, "reopen", p09NearBody()))
	if reopen.Field != "rate_vote_change" {
		t.Errorf("reopen after resolve: field=%q, want rate_vote_change (one shared resolution key)", reopen.Field)
	}
	clk.Advance(30 * time.Second)
	if got := p09Status(p09Vote(t, c, srv.URL, id1, "dispute", p09NearBody())); got != http.StatusOK {
		t.Errorf("dispute after the wait = %d, want 200", got)
	}
}

func TestVoteRateLimitChangeCooldownRefundedOnRefusal(t *testing.T) {
	clk := p09NewClock(time.Millisecond)
	srv, mailer, _ := newE2EServerWith(t, p09Tweak(clk, func(d *api.Deps) {
		d.RateLimits.Vote = api.BucketLimit{Every: time.Hour, Burst: 4}
		d.RateLimits.VoteChange = api.RateLimitsDefault.VoteChange
	}))

	a := newVerifiedClient(t, srv, mailer, "p09-refund-a@example.com")
	d := newVerifiedClient(t, srv, mailer, "p09-refund-d@example.com")
	report := submitReportGetID(t, a, srv.URL, floodReportBody(p09Lat, p09Lon))
	rid := fmt.Sprint(report)

	badBody := map[string]any{"latitude": p09Lat, "longitude": p09Lon, "cell": "x"}
	resp := p09Vote(t, d, srv.URL, rid, "confirm", badBody)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("confirm with an unknown field = %d, want 400", resp.StatusCode)
	}
	if field, _ := decodeErrorResponse(t, resp); field != "body" {
		t.Errorf("400 field = %q, want body", field)
	}

	resp = p09Vote(t, d, srv.URL, rid, "confirm", p09FarBody())
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("far confirm = %d, want 403", resp.StatusCode)
	}
	if field, _ := decodeErrorResponse(t, resp); field != "location" {
		t.Errorf("403 field = %q, want location", field)
	}

	if got := p09Status(p09Vote(t, d, srv.URL, rid, "confirm", p09NearBody())); got != http.StatusOK {
		t.Fatalf("near confirm = %d, want 200 (the two refusals refunded the cooldown)", got)
	}

	if cb := p09Decode429(t, p09Vote(t, d, srv.URL, rid, "dispute", p09NearBody())); cb.Field != "rate_vote_change" {
		t.Errorf("dispute: field=%q, want rate_vote_change", cb.Field)
	}

	rb := p09Decode429(t, p09Vote(t, d, srv.URL, rid, "resolve", p09NearBody()))
	if rb.Field != "rate_vote" || rb.RetryAfterHeader != "3600" {
		t.Errorf("resolve: field=%q Retry-After=%q, want rate_vote 3600 (the vote bucket is never refunded)", rb.Field, rb.RetryAfterHeader)
	}
}

func TestVoteRateLimitAllFiveRoutes(t *testing.T) {
	clk := p09NewClock(time.Millisecond)
	srv, mailer, _ := newE2EServerWith(t, p09Tweak(clk, func(d *api.Deps) {
		d.RateLimits.Vote = api.BucketLimit{Every: time.Hour, Burst: 1}
	}))

	var first *http.Client
	for i, action := range []string{"confirm", "dispute", "resolve", "reopen", "unresolve"} {
		v := newVerifiedClient(t, srv, mailer, fmt.Sprintf("p09-five-%d@example.com", i))
		if first == nil {
			first = v
		}
		if got := p09Status(p09Vote(t, v, srv.URL, "900101", action, p09NearBody())); got != http.StatusNotFound {
			t.Fatalf("%s on a missing report = %d, want 404", action, got)
		}
		rb := p09Decode429(t, p09Vote(t, v, srv.URL, "900101", action, p09NearBody()))
		if rb.Field != "rate_vote" || rb.RetryAfterHeader != "3600" || rb.RetryAfterSeconds != 3600 || rb.Serious || rb.HasNumber {
			t.Errorf("%s second attempt: %+v, want rate_vote 3600 serious false and no emergency_number", action, rb)
		}
		if want := "You're voting very quickly. You can vote again in 1 hour."; rb.Message != want {
			t.Errorf("%s message = %q, want %q", action, rb.Message, want)
		}
	}

	for i := 0; i < 5; i++ {
		resp, err := first.Get(srv.URL + "/api/reports?lat=12.9716&lon=77.5946")
		if err != nil {
			t.Fatalf("GET /api/reports: %v", err)
		}
		if got := p09Status(resp); got != http.StatusOK {
			t.Fatalf("feed read %d = %d, want 200", i+1, got)
		}
	}
}
