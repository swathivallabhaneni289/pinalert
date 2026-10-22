// submit_idempotent_e2e_test.go is V-09 and V-12 for plan 04-17 (D-19, R-05): it
// proves over the real router and a real Postgres that a post retried with the
// same request id is stored once, costs no allowance, never crosses accounts,
// and that the display only written time never moves created_at or expires_at.
// Every package level identifier carries the p17 prefix. These tests run only
// on the disposable database p4_0417, never on pinalert or pinalert_test.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	p17Lat = 12.9716
	p17Lon = 77.5946

	p17Desc = "Water rising near the market bridge"
)

// p17T0 is a past instant, so a refund that read the wall clock again instead of
// using its reservation instant would land later and be lost.
var p17T0 = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// p17Clock is a mutex guarded limiter clock. Now never moves it; Advance does.
type p17Clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *p17Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *p17Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// p17Tweak builds the tweak hook for newE2EServerWith: it injects the limiter
// clock, opens the request link limiter (each test verifies several accounts on
// a frozen clock), installs scr when it is non nil, then runs mutate.
func p17Tweak(clk *p17Clock, scr service.TextScreener, mutate func(d *api.Deps)) func(d *api.Deps, q *sqlcgen.Queries) {
	return func(d *api.Deps, q *sqlcgen.Queries) {
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

func p17Pass() moderation.Result {
	return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{"hate": 0.01}}
}

// p17CountingScreener passes every text and counts the calls.
type p17CountingScreener struct {
	mu    sync.Mutex
	calls int
}

func (s *p17CountingScreener) Screen(_ context.Context, _ string) (moderation.Result, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return p17Pass(), nil
}

func (s *p17CountingScreener) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// p17BarrierScreener holds the first n calls until n have arrived, so n racing
// posts all reach the insert together. The n-th arrival advances the limiter
// clock one second before it releases them, so every Cancel runs at a later
// reading than its Reserve (brief verified correction 1). It gives up after 2
// seconds, under the 3 second inline deadline, and records that.
type p17BarrierScreener struct {
	mu       sync.Mutex
	n        int
	arrived  int
	timedOut bool
	once     sync.Once
	release  chan struct{}
	clk      *p17Clock
}

func p17NewBarrier(n int, clk *p17Clock) *p17BarrierScreener {
	return &p17BarrierScreener{n: n, release: make(chan struct{}), clk: clk}
}

func (b *p17BarrierScreener) Screen(ctx context.Context, _ string) (moderation.Result, error) {
	b.mu.Lock()
	b.arrived++
	mine := b.arrived
	b.mu.Unlock()
	if mine <= b.n {
		if mine == b.n {
			b.once.Do(func() {
				b.clk.Advance(time.Second)
				close(b.release)
			})
		}
		select {
		case <-b.release:
		case <-ctx.Done():
			b.mu.Lock()
			b.timedOut = true
			b.mu.Unlock()
		case <-time.After(2 * time.Second):
			b.mu.Lock()
			b.timedOut = true
			b.mu.Unlock()
		}
	}
	return p17Pass(), nil
}

func (b *p17BarrierScreener) TimedOut() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.timedOut
}

// p17DownScreener always fails like a provider outage, so the check is
// unfinished and a non serious report is held (D-01).
type p17DownScreener struct{}

func (p17DownScreener) Screen(_ context.Context, _ string) (moderation.Result, error) {
	return moderation.Result{}, &moderation.CallError{Status: 500, Transient: true}
}

// p17Body is a report body at the shared point; extra adds or overrides keys.
func p17Body(category, severity, description string, extra map[string]any) map[string]any {
	body := map[string]any{
		"category":    category,
		"severity":    severity,
		"description": description,
		"latitude":    p17Lat,
		"longitude":   p17Lon,
	}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

// p17PostRaw posts body. It never calls t.Fatalf, so goroutines can use it.
func p17PostRaw(client *http.Client, baseURL string, body map[string]any) (int, []byte, http.Header, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return 0, nil, nil, err
	}
	resp, err := client.Post(baseURL+"/api/reports", "application/json", bytes.NewReader(buf))
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, resp.Header, err
}

// p17Expect posts and requires the status, returning the raw body.
func p17Expect(t *testing.T, label string, client *http.Client, baseURL string, body map[string]any, want int) []byte {
	t.Helper()
	status, raw, _, err := p17PostRaw(client, baseURL, body)
	if err != nil {
		t.Fatalf("%s: POST /api/reports: %v", label, err)
	}
	if status != want {
		t.Fatalf("%s: status = %d, want %d, body %s", label, status, want, raw)
	}
	return raw
}

// p17Created is the part of the report object these tests read.
type p17Created struct {
	ID          int64  `json:"id"`
	Held        bool   `json:"held"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at"`
}

// p17DecodeCreated decodes the report object of a 200 or 201 body, returning it
// typed and as a generic map.
func p17DecodeCreated(t *testing.T, raw []byte) (p17Created, map[string]any) {
	t.Helper()
	var typed struct {
		Report p17Created `json:"report"`
	}
	if err := json.Unmarshal(raw, &typed); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	var generic struct {
		Report map[string]any `json:"report"`
	}
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	if typed.Report.ID == 0 || generic.Report == nil {
		t.Fatalf("no report object in %s", raw)
	}
	return typed.Report, generic.Report
}

// p17CountRequest counts the stored reports that hold requestID.
func p17CountRequest(t *testing.T, pool *pgxpool.Pool, requestID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM reports WHERE request_id = $1`, requestID).Scan(&n); err != nil {
		t.Fatalf("counting request id %s: %v", requestID, err)
	}
	return n
}

func p17CountAll(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM reports`).Scan(&n); err != nil {
		t.Fatalf("counting reports: %v", err)
	}
	return n
}

// p17AccountID reads the verified account id behind the client's session.
func p17AccountID(t *testing.T, pool *pgxpool.Pool, client *http.Client, baseURL string) int64 {
	t.Helper()
	sid := sessionIDFromJar(t, client, baseURL)
	var id int64
	if err := pool.QueryRow(context.Background(), `SELECT account_id FROM sessions WHERE session_id = $1`, sid).Scan(&id); err != nil {
		t.Fatalf("reading account id for session: %v", err)
	}
	return id
}

// p17SecondSession returns a client whose fresh session is bound to accountID,
// the way a second device of the same verified account looks.
func p17SecondSession(t *testing.T, baseURL string, pool *pgxpool.Pool, accountID int64) *http.Client {
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

// p17ErrorField decodes the field and message of an error body.
func p17ErrorField(t *testing.T, raw []byte) (string, string) {
	t.Helper()
	var out struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding error body %s: %v", raw, err)
	}
	return out.Error.Field, out.Error.Message
}

func TestSubmitIdempotentReplayCostsNoAllowance(t *testing.T) {
	t.Run("early_replay", func(t *testing.T) {
		clk := &p17Clock{now: p17T0}
		scr := &p17CountingScreener{}
		srv, mailer, _ := newE2EServerWith(t, p17Tweak(clk, scr, func(d *api.Deps) {
			d.RateLimits.ReportOther = api.BucketLimit{Every: time.Hour, Burst: 2}
			d.RateLimits.SubmitAttempt = api.BucketLimit{Every: 10 * time.Minute, Burst: 3}
		}))
		a := newVerifiedClient(t, srv, mailer, "p17-early@example.com")

		const x = "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c01"
		raw := p17Expect(t, "X", a, srv.URL, p17Body("flood", "low", p17Desc, map[string]any{"request_id": x}), http.StatusCreated)
		first, _ := p17DecodeCreated(t, raw)
		if scr.Calls() != 1 {
			t.Fatalf("screener calls after X = %d, want 1", scr.Calls())
		}
		for i := 0; i < 3; i++ {
			raw = p17Expect(t, "X again", a, srv.URL, p17Body("flood", "low", p17Desc, map[string]any{"request_id": x}), http.StatusOK)
			again, _ := p17DecodeCreated(t, raw)
			if again.ID != first.ID {
				t.Fatalf("replay %d: id = %d, want %d", i, again.ID, first.ID)
			}
		}
		if scr.Calls() != 1 {
			t.Fatalf("screener calls after three replays = %d, want 1", scr.Calls())
		}
		p17Expect(t, "Y", a, srv.URL, p17Body("flood", "low", p17Desc, map[string]any{"request_id": "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c02"}), http.StatusCreated)
		if scr.Calls() != 2 {
			t.Fatalf("screener calls after Y = %d, want 2", scr.Calls())
		}
		status, raw, header, err := p17PostRaw(a, srv.URL, p17Body("flood", "low", p17Desc, map[string]any{"request_id": "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c03"}))
		if err != nil {
			t.Fatalf("Z: %v", err)
		}
		if status != http.StatusTooManyRequests {
			t.Fatalf("Z: status = %d, want 429, body %s", status, raw)
		}
		if field, _ := p17ErrorField(t, raw); field != "rate_report" {
			t.Fatalf("Z: field = %q, want rate_report (the allowance, not the attempt bucket)", field)
		}
		if got := header.Get("Retry-After"); got != "3600" {
			t.Fatalf("Z: Retry-After = %q, want 3600", got)
		}
	})

	t.Run("replay_found_at_insert", func(t *testing.T) {
		clk := &p17Clock{now: p17T0}
		barrier := p17NewBarrier(2, clk)
		srv, mailer, pool := newE2EServerWith(t, p17Tweak(clk, barrier, func(d *api.Deps) {
			d.RateLimits.ReportOther = api.BucketLimit{Every: time.Hour, Burst: 2}
		}))
		a := newVerifiedClient(t, srv, mailer, "p17-race@example.com")

		const x = "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c11"
		type result struct {
			status int
			raw    []byte
			err    error
		}
		results := make([]result, 2)
		var wg sync.WaitGroup
		for i := range results {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				status, raw, _, err := p17PostRaw(a, srv.URL, p17Body("flood", "low", p17Desc, map[string]any{"request_id": x}))
				results[i] = result{status, raw, err}
			}(i)
		}
		wg.Wait()
		if barrier.TimedOut() {
			t.Fatal("the barrier timed out: the two posts never met")
		}
		created, replayed := 0, 0
		var ids []int64
		for i, r := range results {
			if r.err != nil {
				t.Fatalf("post %d: %v", i, r.err)
			}
			switch r.status {
			case http.StatusCreated:
				created++
			case http.StatusOK:
				replayed++
			default:
				t.Fatalf("post %d: status = %d, body %s", i, r.status, r.raw)
			}
			c, _ := p17DecodeCreated(t, r.raw)
			ids = append(ids, c.ID)
		}
		if created != 1 || replayed != 1 || ids[0] != ids[1] {
			t.Fatalf("created %d, replayed %d, ids %v; want one 201, one 200, one id", created, replayed, ids)
		}
		if n := p17CountRequest(t, pool, x); n != 1 {
			t.Fatalf("rows for the request id = %d, want 1", n)
		}
		p17Expect(t, "Y", a, srv.URL, p17Body("flood", "low", p17Desc, map[string]any{"request_id": "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c12"}), http.StatusCreated)
		p17Expect(t, "Z", a, srv.URL, p17Body("flood", "low", p17Desc, map[string]any{"request_id": "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c13"}), http.StatusTooManyRequests)
	})
}

func TestSubmitIdempotent(t *testing.T) {
	newServer := func(t *testing.T, scr service.TextScreener, email string) (string, *http.Client, *pgxpool.Pool) {
		t.Helper()
		clk := &p17Clock{now: p17T0}
		srv, mailer, pool := newE2EServerWith(t, p17Tweak(clk, scr, nil))
		return srv.URL, newVerifiedClient(t, srv, mailer, email), pool
	}

	t.Run("replay_returns_stored_report", func(t *testing.T) {
		url, a, pool := newServer(t, nil, "p17-replay@example.com")
		const x = "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c21"
		body := p17Body("flood", "medium", p17Desc, map[string]any{"request_id": x})
		first := p17Expect(t, "first", a, url, body, http.StatusCreated)
		second := p17Expect(t, "second", a, url, body, http.StatusOK)
		_, m1 := p17DecodeCreated(t, first)
		_, m2 := p17DecodeCreated(t, second)
		if !reflect.DeepEqual(m1, m2) {
			t.Fatalf("report objects differ:\n%v\n%v", m1, m2)
		}
		if n := p17CountRequest(t, pool, x); n != 1 {
			t.Fatalf("rows = %d, want 1", n)
		}
	})

	t.Run("replay_ignores_changed_body", func(t *testing.T) {
		url, a, pool := newServer(t, nil, "p17-changed@example.com")
		const x = "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c22"
		p17Expect(t, "D1", a, url, p17Body("flood", "low", "First description of the flood", map[string]any{"request_id": x}), http.StatusCreated)
		raw := p17Expect(t, "D2", a, url, p17Body("flood", "low", "Second description, quite different", map[string]any{"request_id": x}), http.StatusOK)
		got, _ := p17DecodeCreated(t, raw)
		if got.Description != "First description of the flood" {
			t.Fatalf("replay description = %q, want the first", got.Description)
		}
		if n := p17CountRequest(t, pool, x); n != 1 {
			t.Fatalf("rows = %d, want 1", n)
		}
	})

	t.Run("absent_or_empty_id_stores_each_time", func(t *testing.T) {
		url, a, pool := newServer(t, nil, "p17-absent@example.com")
		seen := map[int64]bool{}
		bodies := []map[string]any{
			p17Body("flood", "low", p17Desc, nil),
			p17Body("flood", "low", p17Desc, nil),
			p17Body("flood", "low", p17Desc, map[string]any{"request_id": ""}),
			p17Body("flood", "low", p17Desc, map[string]any{"request_id": ""}),
		}
		for i, body := range bodies {
			raw := p17Expect(t, "post", a, url, body, http.StatusCreated)
			c, _ := p17DecodeCreated(t, raw)
			if seen[c.ID] {
				t.Fatalf("post %d: id %d repeated", i, c.ID)
			}
			seen[c.ID] = true
		}
		var n int
		err := pool.QueryRow(context.Background(), `SELECT count(*) FROM reports WHERE request_id IS NULL AND request_account_id IS NULL`).Scan(&n)
		if err != nil || n != 4 {
			t.Fatalf("rows with NULL request id and account = %d (err %v), want 4", n, err)
		}
	})

	t.Run("malformed_id_is_400", func(t *testing.T) {
		url, a, pool := newServer(t, nil, "p17-malformed@example.com")
		bad := []string{
			"abcdefghij-1234",
			strings.Repeat("a", 65),
			"abc def ghi jkl mno",
			"under_score_id_0000",
			"abcdefghij-1234é",
			"../../etc/passwd-0000",
		}
		for _, id := range bad {
			raw := p17Expect(t, "id "+id, a, url, p17Body("flood", "low", p17Desc, map[string]any{"request_id": id}), http.StatusBadRequest)
			field, msg := p17ErrorField(t, raw)
			if field != "request_id" || msg != service.RequestIDInvalidMessage {
				t.Fatalf("id %q: field %q message %q", id, field, msg)
			}
		}
		raw := p17Expect(t, "number", a, url, p17Body("flood", "low", p17Desc, map[string]any{"request_id": 123}), http.StatusBadRequest)
		if field, _ := p17ErrorField(t, raw); field != "body" {
			t.Fatalf("number: field = %q, want body", field)
		}
		if n := p17CountAll(t, pool); n != 0 {
			t.Fatalf("report count = %d, want 0", n)
		}
	})

	t.Run("boundary_ids_accepted", func(t *testing.T) {
		url, a, _ := newServer(t, nil, "p17-boundary@example.com")
		p17Expect(t, "16", a, url, p17Body("flood", "low", p17Desc, map[string]any{"request_id": "abcdefghij-12345"}), http.StatusCreated)
		p17Expect(t, "64", a, url, p17Body("flood", "low", p17Desc, map[string]any{"request_id": strings.Repeat("b", 64)}), http.StatusCreated)
	})

	t.Run("held_replay_keeps_held", func(t *testing.T) {
		url, a, _ := newServer(t, p17DownScreener{}, "p17-held@example.com")
		const h = "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c23"
		body := p17Body("flood", "low", p17Desc, map[string]any{"request_id": h})
		first, _ := p17DecodeCreated(t, p17Expect(t, "first", a, url, body, http.StatusCreated))
		if !first.Held {
			t.Fatal("first answer: held = false, want true")
		}
		second, _ := p17DecodeCreated(t, p17Expect(t, "second", a, url, body, http.StatusOK))
		if !second.Held || second.ID != first.ID {
			t.Fatalf("replay: held %v id %d, want held true id %d", second.Held, second.ID, first.ID)
		}
	})

	t.Run("answer_has_no_internal_fields", func(t *testing.T) {
		url, a, _ := newServer(t, nil, "p17-fields@example.com")
		const x = "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c24"
		body := p17Body("flood", "low", p17Desc, map[string]any{"request_id": x, "written_at": p17MilliISO(time.Now().Add(-time.Hour))})
		for _, want := range []int{http.StatusCreated, http.StatusOK} {
			raw := p17Expect(t, "post", a, url, body, want)
			_, m := p17DecodeCreated(t, raw)
			if _, ok := m["held"]; !ok {
				t.Fatalf("status %d: report has no held key: %v", want, m)
			}
			for _, key := range []string{"request_id", "written_at", "request_account_id", "client_written_at"} {
				if _, ok := m[key]; ok {
					t.Fatalf("status %d: report carries %s: %v", want, key, m)
				}
			}
		}
	})
}

func TestSubmitIdempotentConcurrent(t *testing.T) {
	const posts = 8
	clk := &p17Clock{now: p17T0}
	barrier := p17NewBarrier(posts, clk)
	srv, mailer, pool := newE2EServerWith(t, p17Tweak(clk, barrier, func(d *api.Deps) {
		d.RateLimits.ReportOther = api.BucketLimit{Every: time.Hour, Burst: posts}
	}))
	a := newVerifiedClient(t, srv, mailer, "p17-concurrent@example.com")

	const x = "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c31"
	type result struct {
		status int
		raw    []byte
		err    error
	}
	results := make([]result, posts)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			status, raw, _, err := p17PostRaw(a, srv.URL, p17Body("flood", "low", p17Desc, map[string]any{"request_id": x}))
			results[i] = result{status, raw, err}
		}(i)
	}
	close(start)
	wg.Wait()
	if barrier.TimedOut() {
		t.Fatal("the barrier timed out: the posts never all met")
	}

	created, replayed := 0, 0
	ids := map[int64]bool{}
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("post %d: %v", i, r.err)
		}
		switch r.status {
		case http.StatusCreated:
			created++
		case http.StatusOK:
			replayed++
		default:
			t.Fatalf("post %d: status = %d, body %s", i, r.status, r.raw)
		}
		c, _ := p17DecodeCreated(t, r.raw)
		ids[c.ID] = true
	}
	if created != 1 || replayed != posts-1 || len(ids) != 1 {
		t.Fatalf("created %d, replayed %d, distinct ids %d; want 1, %d, 1", created, replayed, len(ids), posts-1)
	}
	if n := p17CountRequest(t, pool, x); n != 1 {
		t.Fatalf("rows for the request id = %d, want 1", n)
	}
	// The seven refunds came back at their reservation instants although the
	// clock moved on; without them the bucket of eight would be empty.
	p17Expect(t, "distinct", a, srv.URL, p17Body("flood", "low", p17Desc, map[string]any{"request_id": "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c32"}), http.StatusCreated)
}

func TestSubmitIdempotentPerAccount(t *testing.T) {
	clk := &p17Clock{now: p17T0}
	srv, mailer, pool := newE2EServerWith(t, p17Tweak(clk, nil, nil))
	a := newVerifiedClient(t, srv, mailer, "p17-acct-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p17-acct-b@example.com")

	const x = "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c41"
	const descA = "Account A says the road is flooded"
	const descB = "Account B says the bridge is closed"
	a1, _ := p17DecodeCreated(t, p17Expect(t, "A", a, srv.URL, p17Body("flood", "low", descA, map[string]any{"request_id": x}), http.StatusCreated))
	b1, _ := p17DecodeCreated(t, p17Expect(t, "B", b, srv.URL, p17Body("flood", "low", descB, map[string]any{"request_id": x}), http.StatusCreated))
	if a1.ID == b1.ID {
		t.Fatalf("both accounts got report %d", a1.ID)
	}
	if n := p17CountRequest(t, pool, x); n != 2 {
		t.Fatalf("rows for the request id = %d, want 2", n)
	}

	got, _ := p17DecodeCreated(t, p17Expect(t, "A again", a, srv.URL, p17Body("flood", "low", descB, map[string]any{"request_id": x}), http.StatusOK))
	if got.ID != a1.ID || got.Description != descA {
		t.Fatalf("A again: id %d description %q, want %d %q", got.ID, got.Description, a1.ID, descA)
	}
	got, _ = p17DecodeCreated(t, p17Expect(t, "B again", b, srv.URL, p17Body("flood", "low", descA, map[string]any{"request_id": x}), http.StatusOK))
	if got.ID != b1.ID || got.Description != descB {
		t.Fatalf("B again: id %d description %q, want %d %q", got.ID, got.Description, b1.ID, descB)
	}

	// The key is the account, not the session (Pitfall 9).
	second := p17SecondSession(t, srv.URL, pool, p17AccountID(t, pool, a, srv.URL))
	got, _ = p17DecodeCreated(t, p17Expect(t, "A second session", second, srv.URL, p17Body("flood", "low", descB, map[string]any{"request_id": x}), http.StatusOK))
	if got.ID != a1.ID {
		t.Fatalf("A second session: id %d, want %d", got.ID, a1.ID)
	}
	if n := p17CountRequest(t, pool, x); n != 2 {
		t.Fatalf("rows for the request id after replays = %d, want 2", n)
	}
}

// p17MilliISO formats t like the browser's toISOString.
func p17MilliISO(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// p17StoredTimes reads the stored created_at, expires_at and the display only
// client_written_at (nil for NULL) of one report.
func p17StoredTimes(t *testing.T, pool *pgxpool.Pool, id int64) (created, expires time.Time, written *time.Time) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT created_at, expires_at, client_written_at FROM reports WHERE id = $1`, id).Scan(&created, &expires, &written)
	if err != nil {
		t.Fatalf("reading stored times of report %d: %v", id, err)
	}
	return created.UTC(), expires.UTC(), written
}

// p17FeedEntry reads GET /api/reports as client and returns the entry of the
// report id, failing when it is not listed.
func p17FeedEntry(t *testing.T, client *http.Client, baseURL string, id int64) map[string]json.RawMessage {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/reports?lat=12.9716&lon=77.5946")
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/reports: status %d err %v body %s", resp.StatusCode, err, raw)
	}
	var out struct {
		Reports []map[string]json.RawMessage `json:"reports"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding feed %s: %v", raw, err)
	}
	for _, entry := range out.Reports {
		var got int64
		if err := json.Unmarshal(entry["id"], &got); err == nil && got == id {
			return entry
		}
	}
	t.Fatalf("report %d is not in the feed: %s", id, raw)
	return nil
}

func TestSubmitIdempotentSwaggerFields(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "swagger.json"))
	if err != nil {
		t.Fatalf("reading docs/swagger.json: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Schema struct {
					Ref string `json:"$ref"`
				} `json:"schema"`
			} `json:"responses"`
		} `json:"paths"`
		Definitions map[string]struct {
			Required   []string                  `json:"required"`
			Properties map[string]map[string]any `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("decoding docs/swagger.json: %v", err)
	}

	req, ok := spec.Definitions["handlers.SubmitReportRequest"]
	if !ok {
		t.Fatal("no handlers.SubmitReportRequest definition")
	}
	id := req.Properties["request_id"]
	if id["type"] != "string" || id["minLength"] != float64(16) || id["maxLength"] != float64(64) {
		t.Fatalf("request_id = %v, want a string of 16 to 64 characters", id)
	}
	written := req.Properties["written_at"]
	if written["type"] != "string" || written["format"] != "date-time" {
		t.Fatalf("written_at on the request = %v, want a date-time string", written)
	}
	for _, name := range req.Required {
		if name == "request_id" || name == "written_at" {
			t.Fatalf("%s must not be required", name)
		}
	}

	if held := spec.Definitions["handlers.ReportResponse"].Properties["held"]; held["type"] != "boolean" {
		t.Fatalf("ReportResponse.held = %v, want a boolean", held)
	}
	feed := spec.Definitions["handlers.FeedReportResponse"].Properties["written_at"]
	if feed["type"] != "string" || feed["format"] != "date-time" {
		t.Fatalf("FeedReportResponse.written_at = %v, want a date-time string", feed)
	}

	post := spec.Paths["/reports"]["post"]
	for _, code := range []string{"200", "201"} {
		if got := post.Responses[code].Schema.Ref; got != "#/definitions/handlers.SubmitReportResponse" {
			t.Fatalf("POST /reports %s schema = %q", code, got)
		}
	}

	for name, def := range spec.Definitions {
		for _, internal := range []string{"client_written_at", "request_account_id"} {
			if _, ok := def.Properties[internal]; ok {
				t.Fatalf("definition %s exposes %s", name, internal)
			}
		}
	}
}

func TestWrittenAt(t *testing.T) {
	newServer := func(t *testing.T, email string) (string, *http.Client, *pgxpool.Pool) {
		t.Helper()
		clk := &p17Clock{now: p17T0}
		srv, mailer, pool := newE2EServerWith(t, p17Tweak(clk, nil, nil))
		return srv.URL, newVerifiedClient(t, srv, mailer, email), pool
	}
	post := func(t *testing.T, a *http.Client, url string, written any) p17Created {
		t.Helper()
		raw := p17Expect(t, "post", a, url, p17Body("flood", "low", p17Desc, map[string]any{"written_at": written}), http.StatusCreated)
		c, _ := p17DecodeCreated(t, raw)
		return c
	}

	t.Run("unreadable_is_null", func(t *testing.T) {
		url, a, pool := newServer(t, "p17-unreadable@example.com")
		for _, bad := range []string{"not-a-date", "", "2026-13-45T99:00:00Z"} {
			c := post(t, a, url, bad)
			if _, _, w := p17StoredTimes(t, pool, c.ID); w != nil {
				t.Fatalf("written_at %q: stored %v, want NULL", bad, *w)
			}
		}
		raw := p17Expect(t, "number", a, url, p17Body("flood", "low", p17Desc, map[string]any{"written_at": 5}), http.StatusBadRequest)
		if field, _ := p17ErrorField(t, raw); field != "body" {
			t.Fatalf("number: field = %q, want body", field)
		}
	})

	t.Run("old_value_clamped_to_14_days", func(t *testing.T) {
		url, a, pool := newServer(t, "p17-old@example.com")
		c := post(t, a, url, p17MilliISO(time.Now().Add(-20*24*time.Hour)))
		created, _, w := p17StoredTimes(t, pool, c.ID)
		want := created.Add(-service.WrittenAtMaxAge)
		if w == nil || !w.Equal(want) {
			t.Fatalf("stored written_at = %v, want %v", w, want)
		}
		var got string
		if err := json.Unmarshal(p17FeedEntry(t, a, url, c.ID)["written_at"], &got); err != nil {
			t.Fatalf("feed entry has no written_at: %v", err)
		}
		if got != p17MilliISO(want) {
			t.Fatalf("feed written_at = %s, want %s", got, p17MilliISO(want))
		}
	})

	t.Run("future_value_clamped_to_now", func(t *testing.T) {
		url, a, pool := newServer(t, "p17-future@example.com")
		c := post(t, a, url, p17MilliISO(time.Now().Add(2*time.Hour)))
		created, _, w := p17StoredTimes(t, pool, c.ID)
		if w == nil || w.After(created) || created.Sub(*w) > time.Second {
			t.Fatalf("stored written_at = %v, created_at = %v; want not after and within 1s", w, created)
		}
		if _, ok := p17FeedEntry(t, a, url, c.ID)["written_at"]; ok {
			t.Fatal("feed entry carries written_at for a gap under 30 minutes")
		}
	})

	t.Run("recent_gap_has_no_label", func(t *testing.T) {
		url, a, pool := newServer(t, "p17-recent@example.com")
		sent := p17MilliISO(time.Now().Add(-10 * time.Minute))
		c := post(t, a, url, sent)
		want, err := time.Parse(time.RFC3339, sent)
		if err != nil {
			t.Fatalf("parsing %s: %v", sent, err)
		}
		if _, _, w := p17StoredTimes(t, pool, c.ID); w == nil || !w.Equal(want) {
			t.Fatalf("stored written_at = %v, want %v", w, want)
		}
		if _, ok := p17FeedEntry(t, a, url, c.ID)["written_at"]; ok {
			t.Fatal("feed entry carries written_at for a gap under 30 minutes")
		}
	})

	t.Run("old_gap_has_label", func(t *testing.T) {
		url, a, pool := newServer(t, "p17-gap@example.com")
		sent := p17MilliISO(time.Now().Add(-4 * time.Hour))
		c := post(t, a, url, sent)
		want, err := time.Parse(time.RFC3339, sent)
		if err != nil {
			t.Fatalf("parsing %s: %v", sent, err)
		}
		if _, _, w := p17StoredTimes(t, pool, c.ID); w == nil || !w.Equal(want) {
			t.Fatalf("stored written_at = %v, want %v", w, want)
		}
		var got string
		if err := json.Unmarshal(p17FeedEntry(t, a, url, c.ID)["written_at"], &got); err != nil {
			t.Fatalf("feed entry has no written_at: %v", err)
		}
		if got != sent {
			t.Fatalf("feed written_at = %s, want %s", got, sent)
		}
	})
}

func TestWrittenAtNeverFeedsExpiry(t *testing.T) {
	clk := &p17Clock{now: p17T0}
	srv, mailer, pool := newE2EServerWith(t, p17Tweak(clk, nil, nil))
	a := newVerifiedClient(t, srv, mailer, "p17-expiry@example.com")

	for _, sev := range []service.Severity{service.SeverityLow, service.SeverityMedium, service.SeverityCritical} {
		before := time.Now().UTC().Add(-time.Second)
		body := p17Body("flood", string(sev), p17Desc, map[string]any{"written_at": p17MilliISO(time.Now().Add(-9 * time.Hour))})
		raw := p17Expect(t, string(sev), a, srv.URL, body, http.StatusCreated)
		after := time.Now().UTC().Add(time.Second)
		c, _ := p17DecodeCreated(t, raw)

		created, expires, _ := p17StoredTimes(t, pool, c.ID)
		if created.Before(before) || created.After(after) {
			t.Fatalf("%s: created_at = %v, want between %v and %v", sev, created, before, after)
		}
		want := service.ExpiryDuration(sev)
		if got := expires.Sub(created); got != want {
			t.Fatalf("%s: expires_at - created_at = %v, want %v", sev, got, want)
		}
		// The answer is written in the server's zone, so compare the instants.
		answered, err := time.Parse(time.RFC3339, c.ExpiresAt)
		if err != nil || !answered.Equal(created.Add(want).Truncate(time.Millisecond)) {
			t.Fatalf("%s: answer expires_at = %s (err %v), want %s", sev, c.ExpiresAt, err, p17MilliISO(created.Add(want)))
		}
		p17FeedEntry(t, a, srv.URL, c.ID)
	}
}
