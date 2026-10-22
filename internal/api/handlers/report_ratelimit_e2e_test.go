// report_ratelimit_e2e_test.go is V-09 for plan 04-15: it proves the report
// posting limits end to end over the real router and a real Postgres. Every
// server is built through newE2EServerWith with p15Tweak, which opens all seven
// buckets first so a test only ever meets the bucket it sets. Every package
// level identifier carries the p15 prefix. These tests run only on the
// disposable database p4_0415.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	p15Lat      = 12.9716
	p15Lon      = 77.5946
	p15IPHeader = "X-P15-Client-IP"

	p15HatefulMarker      = "p15 hateful marker"
	p15ProviderDownMarker = "p15 provider down"
	p15InsertFailsMarker  = "p15 insert fails here"

	// p15Valid is a description the text check clears.
	p15Valid = "Water rising near the market bridge"
)

// p15T0 is a past instant, so a refund that read the wall clock again would land
// later than the reservation and be lost.
var p15T0 = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// p15Open is a burst no test reaches, so it never refuses on a frozen clock.
var p15Open = api.BucketLimit{Every: time.Millisecond, Burst: 1000}

// p15Clock is a limiter clock. Now returns the current value and then adds one
// tick, so a tick of zero is a frozen clock. Advance jumps it forward.
type p15Clock struct {
	mu   sync.Mutex
	now  time.Time
	tick time.Duration
}

func newP15Clock(tick time.Duration) *p15Clock {
	return &p15Clock{now: p15T0, tick: tick}
}

func (c *p15Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	cur := c.now
	c.now = c.now.Add(c.tick)
	return cur
}

func (c *p15Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// p15Screener is a scripted service.TextScreener that behaves by marker.
type p15Screener struct {
	mu    sync.Mutex
	calls int
}

func (s *p15Screener) Screen(_ context.Context, text string) (moderation.Result, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	switch {
	case strings.Contains(text, p15HatefulMarker):
		return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{"hate": 0.99}}, nil
	case strings.Contains(text, p15ProviderDownMarker):
		return moderation.Result{}, context.DeadlineExceeded
	default:
		return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{"hate": 0.01}}, nil
	}
}

// p15Tweak builds the tweak hook for newE2EServerWith. It opens all seven
// buckets, injects the clock, opens the request link limiter (each test verifies
// several accounts), installs the screener when scr is non nil, then runs mutate.
func p15Tweak(clk *p15Clock, scr *p15Screener, mutate func(d *api.Deps)) func(d *api.Deps, q *sqlcgen.Queries) {
	return func(d *api.Deps, q *sqlcgen.Queries) {
		d.RateLimits = api.RateLimits{
			ReportSerious:    p15Open,
			ReportOther:      p15Open,
			Vote:             p15Open,
			VoteChange:       p15Open,
			Flag:             p15Open,
			SubmitAttempt:    p15Open,
			IPBackstopReport: p15Open,
			IPBackstopVote:   p15Open,
			IPBackstopFlag:   p15Open,
		}
		d.LimiterClock = clk.Now
		d.RequestLinkRateLimit = api.RequestLinkRateLimit{Burst: 100, Every: time.Millisecond}
		if scr != nil {
			d.Reports = service.NewReportService(q, service.WithScreener(scr))
		}
		if mutate != nil {
			mutate(d)
		}
	}
}

// p15Body is a report body at the shared point.
func p15Body(category, severity, description string) map[string]any {
	return map[string]any{
		"category":    category,
		"severity":    severity,
		"description": description,
		"latitude":    p15Lat,
		"longitude":   p15Lon,
	}
}

// p15Post posts body, sending p15IPHeader only when ip is non empty.
func p15Post(t *testing.T, client *http.Client, baseURL string, body map[string]any, ip string) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling request body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/reports", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ip != "" {
		req.Header.Set(p15IPHeader, ip)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/reports: %v", err)
	}
	return resp
}

// p15Expect posts and requires the status, returning the raw body.
func p15Expect(t *testing.T, label string, client *http.Client, baseURL string, body map[string]any, ip string, want int) []byte {
	t.Helper()
	resp := p15Post(t, client, baseURL, body, ip)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s: reading body: %v", label, err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s: status = %d, want %d, body %s", label, resp.StatusCode, want, raw)
	}
	return raw
}

// p15RateBody is a decoded 429.
type p15RateBody struct {
	Status            int
	RetryAfterHeader  string
	ContentType       string
	Keys              []string
	ErrorKeys         []string
	Field             string
	Message           string
	RetryAfterSeconds int
	Serious           bool
	EmergencyNumber   string
	HasNumber         bool
}

// p15Decode429 decodes a rate limited response and fails unless it is a 429.
func p15Decode429(t *testing.T, resp *http.Response) p15RateBody {
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
	out := p15RateBody{
		Status:           resp.StatusCode,
		RetryAfterHeader: resp.Header.Get("Retry-After"),
		ContentType:      resp.Header.Get("Content-Type"),
	}
	for k := range top {
		out.Keys = append(out.Keys, k)
	}
	sort.Strings(out.Keys)
	var errObj map[string]json.RawMessage
	if err := json.Unmarshal(top["error"], &errObj); err != nil {
		t.Fatalf("decoding 429 error object %s: %v", raw, err)
	}
	for k := range errObj {
		out.ErrorKeys = append(out.ErrorKeys, k)
	}
	sort.Strings(out.ErrorKeys)
	_, out.HasNumber = top["emergency_number"]
	var parsed struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
		RetryAfterSeconds int    `json:"retry_after_seconds"`
		Serious           bool   `json:"serious"`
		EmergencyNumber   string `json:"emergency_number"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("decoding 429 fields %s: %v", raw, err)
	}
	out.Field = parsed.Error.Field
	out.Message = parsed.Error.Message
	out.RetryAfterSeconds = parsed.RetryAfterSeconds
	out.Serious = parsed.Serious
	out.EmergencyNumber = parsed.EmergencyNumber
	return out
}

// p15Blocked posts and requires a 429, returning it decoded.
func p15Blocked(t *testing.T, label string, client *http.Client, baseURL string, body map[string]any, ip string) p15RateBody {
	t.Helper()
	got := p15Decode429(t, p15Post(t, client, baseURL, body, ip))
	if got.Field != "rate_report" {
		t.Fatalf("%s: field = %q, want rate_report", label, got.Field)
	}
	return got
}

// p15AccountID reads the verified account id behind the client's session.
func p15AccountID(t *testing.T, pool *pgxpool.Pool, client *http.Client, baseURL string) int64 {
	t.Helper()
	sid := sessionIDFromJar(t, client, baseURL)
	var id int64
	if err := pool.QueryRow(context.Background(), `SELECT account_id FROM sessions WHERE session_id = $1`, sid).Scan(&id); err != nil {
		t.Fatalf("reading account id for session: %v", err)
	}
	return id
}

// p15SecondSession returns a client whose fresh session is bound to accountID,
// the way a second device of the same verified account looks.
func p15SecondSession(t *testing.T, baseURL string, pool *pgxpool.Pool, accountID int64) *http.Client {
	t.Helper()
	c := newJarClient(t)
	resp, err := c.Get(baseURL + "/login")
	if err != nil {
		t.Fatalf("GET /login for the second session: %v", err)
	}
	resp.Body.Close()
	sid := sessionIDFromJar(t, c, baseURL)
	_, err = sqlcgen.New(pool).BindSessionAccount(context.Background(), sqlcgen.BindSessionAccountParams{SessionID: sid, AccountID: &accountID})
	if err != nil {
		t.Fatalf("binding the second session: %v", err)
	}
	return c
}

// p15CountReports counts every stored report.
func p15CountReports(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM reports`).Scan(&n); err != nil {
		t.Fatalf("counting reports: %v", err)
	}
	return n
}

func p15WantCount(t *testing.T, pool *pgxpool.Pool, want int) {
	t.Helper()
	if got := p15CountReports(t, pool); got != want {
		t.Fatalf("report count = %d, want %d", got, want)
	}
}

// p15RescueRefused posts a rescue_needed report and requires the 400 on the
// field category with the retired message. The refusal comes before the
// posting allowance, so a caller that is blocked or not sees the same answer
// and the report count does not change.
func p15RescueRefused(t *testing.T, label string, client *http.Client, baseURL string) {
	t.Helper()
	raw := p15Expect(t, label, client, baseURL, p15Body("rescue_needed", "low", p15Valid), "", http.StatusBadRequest)
	var out struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: decoding %s: %v", label, raw, err)
	}
	if out.Error.Field != "category" || out.Error.Message != service.RescueNeededRetiredMessage {
		t.Fatalf("%s: field %q message %q, want category and %q", label, out.Error.Field, out.Error.Message, service.RescueNeededRetiredMessage)
	}
}

const (
	p15OtherMsg15Min = "You've posted several reports in a short time. You can post again in 15 minutes. Your text is still here."
	p15SeriousMsg5   = "You've posted several serious reports in a short time. You can post again in 5 minutes. Your text is still here."
)

func TestReportRateLimit(t *testing.T) {
	if got := api.RateLimitsDefault.ReportOther; got != (api.BucketLimit{Every: 15 * time.Minute, Burst: 8}) {
		t.Fatalf("RateLimitsDefault.ReportOther = %+v", got)
	}
	if got := api.RateLimitsDefault.ReportSerious; got != (api.BucketLimit{Every: 5 * time.Minute, Burst: 5}) {
		t.Fatalf("RateLimitsDefault.ReportSerious = %+v", got)
	}
	clk := newP15Clock(0)
	srv, mailer, pool := newE2EServerWith(t, p15Tweak(clk, nil, func(d *api.Deps) {
		d.RateLimits.ReportOther = api.RateLimitsDefault.ReportOther
		d.RateLimits.ReportSerious = api.RateLimitsDefault.ReportSerious
	}))
	a := newVerifiedClient(t, srv, mailer, "p15-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p15-b@example.com")

	for i := 0; i < 8; i++ {
		sev := "low"
		if i%2 == 1 {
			sev = "medium"
		}
		p15Expect(t, "other "+strconv.Itoa(i), a, srv.URL, p15Body("flood", sev, p15Valid), "", http.StatusCreated)
	}
	got := p15Blocked(t, "9th other", a, srv.URL, p15Body("flood", "low", p15Valid), "")
	if got.RetryAfterHeader != "900" || got.RetryAfterSeconds != 900 {
		t.Fatalf("9th other: Retry-After %q seconds %d, want 900", got.RetryAfterHeader, got.RetryAfterSeconds)
	}
	if got.Message != p15OtherMsg15Min {
		t.Fatalf("9th other: message = %q", got.Message)
	}
	if got.Serious || got.HasNumber {
		t.Fatalf("9th other: serious %v, has number %v", got.Serious, got.HasNumber)
	}
	p15WantCount(t, pool, 8)

	for i := 0; i < 3; i++ {
		p15Expect(t, "critical "+strconv.Itoa(i), a, srv.URL, p15Body("flood", "critical", p15Valid), "", http.StatusCreated)
	}
	for i := 0; i < 2; i++ {
		p15Expect(t, "traffic "+strconv.Itoa(i), a, srv.URL, p15Body("traffic", "critical", p15Valid), "", http.StatusCreated)
	}
	got = p15Blocked(t, "6th serious", a, srv.URL, p15Body("traffic", "critical", p15Valid), "")
	if got.RetryAfterHeader != "300" || got.RetryAfterSeconds != 300 {
		t.Fatalf("6th serious: Retry-After %q seconds %d, want 300", got.RetryAfterHeader, got.RetryAfterSeconds)
	}
	if got.Message != p15SeriousMsg5 {
		t.Fatalf("6th serious: message = %q", got.Message)
	}
	if !got.Serious || got.EmergencyNumber != "112" {
		t.Fatalf("6th serious: serious %v number %q", got.Serious, got.EmergencyNumber)
	}
	p15WantCount(t, pool, 13)

	// A rescue_needed post is refused with a 400 even while the serious bucket
	// is empty: it never reaches the gate, so it uses no allowance.
	p15RescueRefused(t, "rescue while blocked", a, srv.URL)
	p15WantCount(t, pool, 13)

	// The key is the account, not the session (D-10).
	second := p15SecondSession(t, srv.URL, pool, p15AccountID(t, pool, a, srv.URL))
	got = p15Blocked(t, "second session other", second, srv.URL, p15Body("flood", "low", p15Valid), "")
	if got.RetryAfterHeader != "900" || got.Serious {
		t.Fatalf("second session other: Retry-After %q serious %v", got.RetryAfterHeader, got.Serious)
	}
	got = p15Blocked(t, "second session serious", second, srv.URL, p15Body("flood", "critical", p15Valid), "")
	if !got.Serious {
		t.Fatalf("second session serious: serious = false")
	}

	// Another account is unaffected.
	p15Expect(t, "b low", b, srv.URL, p15Body("flood", "low", p15Valid), "", http.StatusCreated)
	p15Expect(t, "b critical", b, srv.URL, p15Body("flood", "critical", p15Valid), "", http.StatusCreated)

	// Reading stays unlimited.
	resp, err := a.Get(srv.URL + "/api/reports?lat=12.9716&lon=77.5946")
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/reports while drained = %d, want 200", resp.StatusCode)
	}

	// The refill: 1 other per 15 minutes, 3 serious in the same time.
	clk.Advance(15 * time.Minute)
	p15Expect(t, "refill other", a, srv.URL, p15Body("flood", "low", p15Valid), "", http.StatusCreated)
	got = p15Blocked(t, "refill other again", a, srv.URL, p15Body("flood", "low", p15Valid), "")
	if got.RetryAfterHeader != "900" {
		t.Fatalf("refill other again: Retry-After %q, want 900", got.RetryAfterHeader)
	}
	for i := 0; i < 3; i++ {
		p15Expect(t, "refill serious "+strconv.Itoa(i), a, srv.URL, p15Body("flood", "critical", p15Valid), "", http.StatusCreated)
	}
	got = p15Blocked(t, "refill serious again", a, srv.URL, p15Body("flood", "critical", p15Valid), "")
	if got.RetryAfterHeader != "300" || !got.Serious {
		t.Fatalf("refill serious again: Retry-After %q serious %v", got.RetryAfterHeader, got.Serious)
	}
}

func TestReportRateLimitSeparateBuckets(t *testing.T) {
	clk := newP15Clock(0)
	srv, mailer, _ := newE2EServerWith(t, p15Tweak(clk, nil, func(d *api.Deps) {
		d.RateLimits.ReportOther = api.BucketLimit{Every: time.Hour, Burst: 1}
		d.RateLimits.ReportSerious = api.BucketLimit{Every: time.Hour, Burst: 1}
	}))
	a := newVerifiedClient(t, srv, mailer, "p15-sep-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p15-sep-b@example.com")
	otherMsg := service.ReportLimitMessage(false, time.Hour)
	seriousMsg := service.ReportLimitMessage(true, time.Hour)

	p15Expect(t, "a low", a, srv.URL, p15Body("flood", "low", p15Valid), "", http.StatusCreated)
	got := p15Blocked(t, "a medium", a, srv.URL, p15Body("flood", "medium", p15Valid), "")
	if got.Serious || got.Message != otherMsg || got.RetryAfterHeader != "3600" || !strings.Contains(got.Message, "1 hour") {
		t.Fatalf("a medium: %+v", got)
	}
	p15Expect(t, "a critical fire", a, srv.URL, p15Body("fire", "critical", p15Valid), "", http.StatusCreated)
	got = p15Blocked(t, "a traffic critical", a, srv.URL, p15Body("traffic", "critical", p15Valid), "")
	if !got.Serious || got.Message != seriousMsg || got.EmergencyNumber != "112" {
		t.Fatalf("a traffic critical: %+v", got)
	}
	p15RescueRefused(t, "a rescue", a, srv.URL)
	got = p15Blocked(t, "a low again", a, srv.URL, p15Body("flood", "low", p15Valid), "")
	if got.Serious || got.Message != otherMsg || got.HasNumber {
		t.Fatalf("a low again: %+v", got)
	}

	p15Expect(t, "b critical", b, srv.URL, p15Body("flood", "critical", p15Valid), "", http.StatusCreated)
	got = p15Blocked(t, "b critical again", b, srv.URL, p15Body("flood", "critical", p15Valid), "")
	if !got.Serious {
		t.Fatalf("b critical again: serious = false")
	}
	p15Expect(t, "b low", b, srv.URL, p15Body("flood", "low", p15Valid), "", http.StatusCreated)
}

func TestReportRateLimitBody(t *testing.T) {
	t.Run("other_shape", func(t *testing.T) {
		clk := newP15Clock(0)
		srv, mailer, _ := newE2EServerWith(t, p15Tweak(clk, nil, func(d *api.Deps) {
			d.RateLimits.ReportOther = api.BucketLimit{Every: 15 * time.Minute, Burst: 1}
		}))
		a := newVerifiedClient(t, srv, mailer, "p15-shape-a@example.com")
		p15Expect(t, "first", a, srv.URL, p15Body("flood", "low", p15Valid), "", http.StatusCreated)
		got := p15Blocked(t, "second", a, srv.URL, p15Body("flood", "low", p15Valid), "")
		if !strings.HasPrefix(got.ContentType, "application/json") {
			t.Fatalf("Content-Type = %q", got.ContentType)
		}
		if strings.Join(got.Keys, ",") != "error,retry_after_seconds,serious" {
			t.Fatalf("top level keys = %v", got.Keys)
		}
		if strings.Join(got.ErrorKeys, ",") != "field,message" {
			t.Fatalf("error keys = %v", got.ErrorKeys)
		}
		secs, err := strconv.Atoi(got.RetryAfterHeader)
		if err != nil || secs != got.RetryAfterSeconds || secs != 900 {
			t.Fatalf("Retry-After %q, retry_after_seconds %d", got.RetryAfterHeader, got.RetryAfterSeconds)
		}
		if got.Message != service.ReportLimitMessage(false, 900*time.Second) {
			t.Fatalf("message = %q", got.Message)
		}
		low := strings.ToLower(got.Message)
		if strings.Contains(low, "too many") || strings.Contains(low, "emergency") {
			t.Fatalf("message names a forbidden phrase: %q", got.Message)
		}
	})

	t.Run("serious_shape_default_number", func(t *testing.T) {
		clk := newP15Clock(0)
		srv, mailer, _ := newE2EServerWith(t, p15Tweak(clk, nil, func(d *api.Deps) {
			d.RateLimits.ReportSerious = api.BucketLimit{Every: 5 * time.Minute, Burst: 1}
		}))
		a := newVerifiedClient(t, srv, mailer, "p15-ser-a@example.com")
		p15Expect(t, "first", a, srv.URL, p15Body("flood", "critical", p15Valid), "", http.StatusCreated)
		got := p15Blocked(t, "second", a, srv.URL, p15Body("flood", "critical", p15Valid), "")
		if strings.Join(got.Keys, ",") != "emergency_number,error,retry_after_seconds,serious" {
			t.Fatalf("top level keys = %v", got.Keys)
		}
		if got.EmergencyNumber != "112" || !got.Serious {
			t.Fatalf("number %q serious %v", got.EmergencyNumber, got.Serious)
		}
		if got.Message != service.ReportLimitMessage(true, 300*time.Second) {
			t.Fatalf("message = %q", got.Message)
		}
		low := strings.ToLower(got.Message)
		if strings.Contains(low, "112") || strings.Contains(low, "emergency") {
			t.Fatalf("message carries the number or the emergency sentence: %q", got.Message)
		}
	})

	t.Run("serious_number_from_setting", func(t *testing.T) {
		clk := newP15Clock(0)
		srv, mailer, _ := newE2EServerWith(t, p15Tweak(clk, nil, func(d *api.Deps) {
			d.RateLimits.ReportSerious = api.BucketLimit{Every: 5 * time.Minute, Burst: 1}
			d.Chrome = handlers.Chrome{Numbers: handlers.EmergencyNumbers{Primary: "999"}}
		}))
		a := newVerifiedClient(t, srv, mailer, "p15-num-a@example.com")
		p15Expect(t, "first", a, srv.URL, p15Body("flood", "critical", p15Valid), "", http.StatusCreated)
		got := p15Blocked(t, "second", a, srv.URL, p15Body("flood", "critical", p15Valid), "")
		if got.EmergencyNumber != "999" {
			t.Fatalf("emergency_number = %q, want 999", got.EmergencyNumber)
		}
	})

	t.Run("serious_blocked_by_ip_backstop", func(t *testing.T) {
		clk := newP15Clock(0)
		srv, mailer, pool := newE2EServerWith(t, p15Tweak(clk, nil, func(d *api.Deps) {
			d.ClientIPHeader = p15IPHeader
			d.RateLimits.IPBackstopReport = api.BucketLimit{Every: time.Hour, Burst: 1}
		}))
		a := newVerifiedClient(t, srv, mailer, "p15-ip-a@example.com")
		b := newVerifiedClient(t, srv, mailer, "p15-ip-b@example.com")
		p15Expect(t, "a critical", a, srv.URL, p15Body("flood", "critical", p15Valid), "203.0.113.7", http.StatusCreated)
		got := p15Blocked(t, "a critical again", a, srv.URL, p15Body("flood", "critical", p15Valid), "203.0.113.7")
		if !got.Serious || got.EmergencyNumber != "112" || got.RetryAfterHeader != "3600" {
			t.Fatalf("a critical again: %+v", got)
		}
		if got.Message != service.NetworkLimitMessage(true, time.Hour) {
			t.Fatalf("a critical again: message = %q, want the network message", got.Message)
		}
		got = p15Blocked(t, "b low", b, srv.URL, p15Body("flood", "low", p15Valid), "203.0.113.7")
		if got.Serious || got.HasNumber {
			t.Fatalf("b low: %+v", got)
		}
		if got.Message != service.NetworkLimitMessage(true, time.Hour) {
			t.Fatalf("b low: message = %q, want the network message", got.Message)
		}
		p15Expect(t, "a low other ip", a, srv.URL, p15Body("flood", "low", p15Valid), "203.0.113.8", http.StatusCreated)
		p15Expect(t, "a low no header", a, srv.URL, p15Body("flood", "low", p15Valid), "", http.StatusCreated)
		p15WantCount(t, pool, 3)
	})

	t.Run("backstop_off_without_setting", func(t *testing.T) {
		clk := newP15Clock(0)
		srv, mailer, _ := newE2EServerWith(t, p15Tweak(clk, nil, func(d *api.Deps) {
			d.RateLimits.IPBackstopReport = api.BucketLimit{Every: time.Hour, Burst: 1}
		}))
		a := newVerifiedClient(t, srv, mailer, "p15-off-a@example.com")
		for i := 0; i < 3; i++ {
			p15Expect(t, "post "+strconv.Itoa(i), a, srv.URL, p15Body("flood", "low", p15Valid), "203.0.113.7", http.StatusCreated)
		}
	})

	// WR-S02: each action family has its own backstop, so votes from one
	// shared address cannot starve report posting from it, and the vote
	// refusal uses the neutral network sentence.
	t.Run("votes_do_not_starve_reports", func(t *testing.T) {
		clk := newP15Clock(0)
		srv, mailer, pool := newE2EServerWith(t, p15Tweak(clk, nil, func(d *api.Deps) {
			d.ClientIPHeader = p15IPHeader
			d.RateLimits.IPBackstopVote = api.BucketLimit{Every: time.Hour, Burst: 1}
		}))
		a := newVerifiedClient(t, srv, mailer, "p15-family-a@example.com")
		b := newVerifiedClient(t, srv, mailer, "p15-family-b@example.com")
		vote := func(client *http.Client) *http.Response {
			t.Helper()
			req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/reports/900001/confirm",
				bytes.NewReader([]byte(`{"latitude":12.9716,"longitude":77.5946}`)))
			if err != nil {
				t.Fatalf("building vote request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(p15IPHeader, "203.0.113.7")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("POST vote: %v", err)
			}
			return resp
		}
		first := vote(b)
		first.Body.Close()
		if first.StatusCode != http.StatusNotFound {
			t.Fatalf("first vote = %d, want 404 (admitted, missing report)", first.StatusCode)
		}
		got := p15Decode429(t, vote(a))
		if got.Field != "rate_vote" || got.Serious || got.HasNumber {
			t.Fatalf("second vote: %+v, want rate_vote, serious false, no number", got)
		}
		if got.Message != service.NetworkLimitMessage(false, time.Hour) {
			t.Fatalf("second vote: message = %q, want the network message", got.Message)
		}
		p15Expect(t, "a low from the same address", a, srv.URL, p15Body("flood", "low", p15Valid), "203.0.113.7", http.StatusCreated)
		p15Expect(t, "a critical from the same address", a, srv.URL, p15Body("flood", "critical", p15Valid), "203.0.113.7", http.StatusCreated)
		p15WantCount(t, pool, 2)
	})
}

// p15BackstopBucketName is the bucket name the gate logs for the report
// family of the network backstop (WR-S02).
const p15BackstopBucketName = "ip_backstop_report"

// p15FailInsertsWithMarker makes any insert whose description holds
// p15InsertFailsMarker fail with a real database error, whichever query or
// transaction SubmitChecked uses. The constraint is dropped before the harness
// closes the pool (cleanups run last registered first).
func p15FailInsertsWithMarker(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `ALTER TABLE reports ADD CONSTRAINT p15_insert_fails CHECK (description NOT LIKE '%p15 insert fails%')`); err != nil {
		t.Fatalf("adding the insert failing constraint: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `ALTER TABLE reports DROP CONSTRAINT IF EXISTS p15_insert_fails`); err != nil {
			t.Errorf("dropping the insert failing constraint: %v", err)
		}
	})
}

// p15LogBuffer is a mutex guarded writer for the standard logger.
type p15LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *p15LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *p15LogBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

// Lines returns the captured non empty lines.
func (b *p15LogBuffer) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, line := range strings.Split(b.buf.String(), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// p15CaptureLog redirects the standard logger into a buffer with no flags until
// the test ends, then restores the writer and the flags.
func p15CaptureLog(t *testing.T) *p15LogBuffer {
	t.Helper()
	buf := &p15LogBuffer{}
	prevWriter, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevWriter)
		log.SetFlags(prevFlags)
	})
	return buf
}

func TestReportRateLimitRefunds(t *testing.T) {
	clk := newP15Clock(time.Second)
	scr := &p15Screener{}
	srv, mailer, pool := newE2EServerWith(t, p15Tweak(clk, scr, func(d *api.Deps) {
		d.RateLimits.ReportOther = api.BucketLimit{Every: time.Hour, Burst: 1}
		d.RateLimits.ReportSerious = api.BucketLimit{Every: time.Hour, Burst: 1}
	}))
	p15FailInsertsWithMarker(t, pool)
	a := newVerifiedClient(t, srv, mailer, "p15-ref-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p15-ref-b@example.com")
	post := func(label string, c *http.Client, category, severity, text string, want int) []byte {
		return p15Expect(t, label, c, srv.URL, p15Body(category, severity, text), "", want)
	}
	fieldAndMessage := func(label string, raw []byte) (string, string) {
		var out struct {
			Error struct {
				Field   string `json:"field"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s: decoding %s: %v", label, raw, err)
		}
		return out.Error.Field, out.Error.Message
	}

	// (a) validation error: nothing reserved.
	raw := post("a short", a, "flood", "low", "too short", http.StatusBadRequest)
	if field, _ := fieldAndMessage("a short", raw); field != "description" {
		t.Fatalf("a: field = %q", field)
	}
	// (b) hateful text.
	raw = post("b hateful", a, "flood", "low", "flood on the road "+p15HatefulMarker, http.StatusUnprocessableEntity)
	if field, msg := fieldAndMessage("b", raw); field != "description" || msg != service.RejectMessage(service.RejectHateful) {
		t.Fatalf("b: field %q message %q", field, msg)
	}
	// (c) gibberish prefilter.
	raw = post("c gibberish", a, "flood", "low", "zzzzzzzzzzzzzzzz", http.StatusUnprocessableEntity)
	if field, msg := fieldAndMessage("c", raw); field != "description" || msg != service.RejectMessage(service.RejectGibberish) {
		t.Fatalf("c: field %q message %q", field, msg)
	}
	// (d) insert error.
	post("d insert fails", a, "flood", "low", "flood on the road "+p15InsertFailsMarker, http.StatusInternalServerError)
	// (e) admitted only because (a) never reserved and (b), (c), (d) refunded.
	post("e valid", a, "flood", "low", p15Valid, http.StatusCreated)
	// (f) the allowance is spent, and the wait is nearly the full hour.
	got := p15Blocked(t, "f", a, srv.URL, p15Body("flood", "low", p15Valid), "")
	if got.RetryAfterSeconds < 3500 || got.RetryAfterSeconds > 3600 {
		t.Fatalf("f: retry_after_seconds = %d, want 3500 to 3600", got.RetryAfterSeconds)
	}

	// Serious allowance.
	post("g hateful critical", a, "flood", "critical", "flood on the road "+p15HatefulMarker, http.StatusUnprocessableEntity)
	post("h insert fails critical", a, "flood", "critical", "flood on the road "+p15InsertFailsMarker, http.StatusInternalServerError)
	post("i valid critical", a, "flood", "critical", p15Valid, http.StatusCreated)
	got = p15Blocked(t, "j", a, srv.URL, p15Body("flood", "critical", p15Valid), "")
	if !got.Serious {
		t.Fatalf("j: serious = false")
	}

	// A held report keeps its cost.
	raw = post("k held", b, "flood", "low", "flood on the road "+p15ProviderDownMarker, http.StatusCreated)
	var created struct {
		Report struct {
			Held bool `json:"held"`
		} `json:"report"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || !created.Report.Held {
		t.Fatalf("k: held = %v (err %v) in %s", created.Report.Held, err, raw)
	}
	p15Blocked(t, "l", b, srv.URL, p15Body("flood", "low", p15Valid), "")

	p15WantCount(t, pool, 3)
}

func TestReportRateLimitSubmitAttemptBucket(t *testing.T) {
	clk := newP15Clock(0)
	scr := &p15Screener{}
	srv, mailer, _ := newE2EServerWith(t, p15Tweak(clk, scr, func(d *api.Deps) {
		d.RateLimits.SubmitAttempt = api.BucketLimit{Every: time.Hour, Burst: 2}
	}))
	a := newVerifiedClient(t, srv, mailer, "p15-att-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p15-att-b@example.com")

	p15Expect(t, "a short", a, srv.URL, p15Body("flood", "low", "too short"), "", http.StatusBadRequest)
	p15Expect(t, "b hateful", a, srv.URL, p15Body("flood", "low", "flood on the road "+p15HatefulMarker), "", http.StatusUnprocessableEntity)
	p15Expect(t, "c valid", a, srv.URL, p15Body("flood", "low", p15Valid), "", http.StatusCreated)
	got := p15Blocked(t, "d", a, srv.URL, p15Body("flood", "low", p15Valid), "")
	if got.Serious || got.HasNumber || got.RetryAfterHeader != "3600" || got.Message != service.ReportLimitMessage(false, time.Hour) {
		t.Fatalf("d: %+v", got)
	}
	if !strings.Contains(got.Message, "1 hour") {
		t.Fatalf("d: message = %q", got.Message)
	}
	got = p15Blocked(t, "e", a, srv.URL, p15Body("flood", "critical", p15Valid), "")
	if !got.Serious || got.EmergencyNumber != "112" || got.Message != service.ReportLimitMessage(true, time.Hour) {
		t.Fatalf("e: %+v", got)
	}
	p15Expect(t, "b valid", b, srv.URL, p15Body("flood", "low", p15Valid), "", http.StatusCreated)
}

func TestReportRateLimitLogsBlocks(t *testing.T) {
	clk := newP15Clock(0)
	srv, mailer, pool := newE2EServerWith(t, p15Tweak(clk, nil, func(d *api.Deps) {
		d.ClientIPHeader = p15IPHeader
		d.RateLimits.ReportOther = api.BucketLimit{Every: time.Hour, Burst: 1}
		d.RateLimits.ReportSerious = api.BucketLimit{Every: time.Hour, Burst: 1}
		d.RateLimits.IPBackstopReport = api.BucketLimit{Every: time.Hour, Burst: 2}
	}))
	const email = "p15-log-a@example.com"
	const ip = "203.0.113.7"
	a := newVerifiedClient(t, srv, mailer, email)
	id := strconv.FormatInt(p15AccountID(t, pool, a, srv.URL), 10)
	logs := p15CaptureLog(t)

	steps := []struct {
		label    string
		severity string
		ip       string
		want     int
		bucket   string
	}{
		{"low ok", "low", ip, http.StatusCreated, ""},
		{"low blocked by allowance", "low", ip, http.StatusTooManyRequests, "report_other"},
		{"critical blocked by backstop", "critical", ip, http.StatusTooManyRequests, p15BackstopBucketName},
		{"critical ok", "critical", "", http.StatusCreated, ""},
		{"critical blocked by allowance", "critical", "", http.StatusTooManyRequests, "report_serious"},
	}
	for _, s := range steps {
		logs.Reset()
		p15Expect(t, s.label, a, srv.URL, p15Body("flood", s.severity, p15Valid), s.ip, s.want)
		lines := logs.Lines()
		var blockLines []string
		for _, line := range lines {
			if strings.Contains(line, "bucket=") {
				blockLines = append(blockLines, line)
			}
			for _, secret := range []string{email, p15Valid, ip} {
				if strings.Contains(line, secret) {
					t.Fatalf("%s: log line %q carries %q", s.label, line, secret)
				}
			}
		}
		if s.bucket == "" {
			if len(blockLines) != 0 {
				t.Fatalf("%s: unexpected block lines %q", s.label, blockLines)
			}
			continue
		}
		if len(blockLines) == 0 {
			t.Fatalf("%s: no block line for bucket %s in %q", s.label, s.bucket, lines)
		}
		found := false
		for _, line := range blockLines {
			if strings.Contains(line, "bucket="+s.bucket+" ") {
				found = true
			}
			if !strings.Contains(line, " account="+id+" ") || !strings.Contains(line, " wait_seconds=") {
				t.Fatalf("%s: block line %q lacks the account id or the wait", s.label, line)
			}
		}
		if !found {
			t.Fatalf("%s: no block line names bucket %s in %q", s.label, s.bucket, blockLines)
		}
	}
}
