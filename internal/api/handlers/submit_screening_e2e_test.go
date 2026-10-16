// V-04 (04-11): POST /api/reports runs the inline text check. These tests drive
// the real router over a real Postgres with a scripted screener. They run only
// on the disposable database p4_0411, never on pinalert_test or pinalert.
package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	p11Lat = 12.9716
	p11Lon = 77.5946
)

// p11Screener is a scripted service.TextScreener that records what it was given.
// With block set it waits for the context to end, like a stuck provider.
type p11Screener struct {
	mu     sync.Mutex
	result moderation.Result
	err    error
	block  bool

	calls       int
	texts       []string
	hadDeadline []bool
	remaining   []time.Duration
}

func (s *p11Screener) Screen(ctx context.Context, text string) (moderation.Result, error) {
	s.mu.Lock()
	s.calls++
	s.texts = append(s.texts, text)
	dl, ok := ctx.Deadline()
	s.hadDeadline = append(s.hadDeadline, ok)
	s.remaining = append(s.remaining, time.Until(dl))
	block, result, err := s.block, s.result, s.err
	s.mu.Unlock()
	if block {
		<-ctx.Done()
		return moderation.Result{}, ctx.Err()
	}
	return result, err
}

// snapshot returns the number of calls and copies of what they recorded.
func (s *p11Screener) snapshot() (calls int, texts []string, hadDeadline []bool, remaining []time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls, append([]string(nil), s.texts...), append([]bool(nil), s.hadDeadline...), append([]time.Duration(nil), s.remaining...)
}

func p11Scores(scores map[string]float64) *p11Screener {
	return &p11Screener{result: moderation.Result{Model: moderation.DefaultModel, Scores: scores}}
}

// p11Server builds the e2e server with scr as the report service's screener.
func p11Server(t *testing.T, scr *p11Screener) (*httptest.Server, *recordingMailer, *pgxpool.Pool) {
	t.Helper()
	return newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		d.Reports = service.NewReportService(q, service.WithScreener(scr))
	})
}

func p11Body(category, severity, description string) map[string]any {
	return map[string]any{
		"category":    category,
		"severity":    severity,
		"description": description,
		"latitude":    p11Lat,
		"longitude":   p11Lon,
	}
}

// p11Post posts body and returns the status and the decoded JSON object.
func p11Post(t *testing.T, client *http.Client, baseURL string, body map[string]any) (int, map[string]any) {
	t.Helper()
	resp := postReport(t, client, baseURL, body)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding response %q: %v", raw, err)
	}
	return resp.StatusCode, out
}

// p11Created reads the created report id and held out of a 201 body, failing
// when held is missing or not a JSON boolean.
func p11Created(t *testing.T, body map[string]any) (int64, bool) {
	t.Helper()
	report, ok := body["report"].(map[string]any)
	if !ok {
		t.Fatalf("no report object in %v", body)
	}
	id, ok := report["id"].(float64)
	if !ok {
		t.Fatalf("no numeric id in %v", report)
	}
	held, ok := report["held"].(bool)
	if !ok {
		t.Fatalf("held missing or not a boolean in %v", report)
	}
	return int64(id), held
}

func p11ReportCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM reports`).Scan(&n); err != nil {
		t.Fatalf("counting reports: %v", err)
	}
	return n
}

func p11CheckCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM moderation_checks`).Scan(&n); err != nil {
		t.Fatalf("counting checks: %v", err)
	}
	return n
}

func p11State(t *testing.T, pool *pgxpool.Pool, id int64) string {
	t.Helper()
	var state string
	if err := pool.QueryRow(context.Background(), `SELECT moderation_state FROM reports WHERE id = $1`, id).Scan(&state); err != nil {
		t.Fatalf("reading moderation_state of %d: %v", id, err)
	}
	return state
}

type p11Check struct {
	Status      string
	Attempts    int
	Verdict     *string
	Model       *string
	Scores      []byte
	NeedsReview bool
}

func p11CheckRow(t *testing.T, pool *pgxpool.Pool, id int64) p11Check {
	t.Helper()
	var c p11Check
	err := pool.QueryRow(context.Background(),
		`SELECT status, attempts, verdict, model, scores, needs_review FROM moderation_checks WHERE report_id = $1`, id,
	).Scan(&c.Status, &c.Attempts, &c.Verdict, &c.Model, &c.Scores, &c.NeedsReview)
	if err != nil {
		t.Fatalf("reading check row of %d: %v", id, err)
	}
	return c
}

// p11FeedRow reads the default feed (or the disputed view) and finds one id.
func p11FeedRow(t *testing.T, client *http.Client, baseURL string, id int64, disputed bool) (feedReport, bool) {
	t.Helper()
	return findFeedReport(decodeFeedResponse(t, getNearby(t, client, baseURL, p11Lat, p11Lon, disputed)), id)
}

func TestSubmitScreening(t *testing.T) {
	const text = "Water rising near the market bridge"

	t.Run("hateful_refused_at_every_severity", func(t *testing.T) {
		scr := p11Scores(map[string]float64{"hate": service.RejectScoreThreshold})
		srv, mailer, pool := p11Server(t, scr)
		client := newVerifiedClient(t, srv, mailer, "p11-hate@example.com")
		for _, c := range [][2]string{{"flood", "low"}, {"flood", "medium"}, {"flood", "critical"}, {"traffic", "low"}} {
			resp := postReport(t, client, srv.URL, p11Body(c[0], c[1], text))
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("%v: status %d, want 422", c, resp.StatusCode)
			}
			field, msg := decodeErrorResponse(t, resp)
			if field != "description" || msg != service.RejectMessage(service.RejectHateful) {
				t.Fatalf("%v: field %q message %q", c, field, msg)
			}
		}
		// A retired category is refused before the screener runs, so the call
		// count below stays at four.
		resp := postReport(t, client, srv.URL, p11Body("rescue_needed", "low", text))
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("rescue_needed: status %d, want 400", resp.StatusCode)
		}
		if field, msg := decodeErrorResponse(t, resp); field != "category" || msg != service.RescueNeededRetiredMessage {
			t.Fatalf("rescue_needed: field %q message %q", field, msg)
		}
		if n := p11ReportCount(t, pool); n != 0 {
			t.Fatalf("reports stored %d, want 0", n)
		}
		if n := p11CheckCount(t, pool); n != 0 {
			t.Fatalf("check rows %d, want 0", n)
		}
		if calls, _, _, _ := scr.snapshot(); calls != 4 {
			t.Fatalf("screener calls %d, want 4", calls)
		}
	})

	t.Run("harassing_and_sexual_named", func(t *testing.T) {
		for _, c := range []struct {
			key    string
			reason service.RejectReason
		}{{"harassment", service.RejectHarassing}, {"sexual", service.RejectSexual}} {
			scr := p11Scores(map[string]float64{c.key: service.RejectScoreThreshold})
			srv, mailer, _ := p11Server(t, scr)
			client := newVerifiedClient(t, srv, mailer, "p11-named@example.com")
			resp := postReport(t, client, srv.URL, p11Body("flood", "low", text))
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("%s: status %d, want 422", c.key, resp.StatusCode)
			}
			if _, msg := decodeErrorResponse(t, resp); msg != service.RejectMessage(c.reason) {
				t.Fatalf("%s: message %q, want %q", c.key, msg, service.RejectMessage(c.reason))
			}
		}
	})

	t.Run("gibberish_refused_without_provider", func(t *testing.T) {
		scr := p11Scores(map[string]float64{})
		srv, mailer, pool := p11Server(t, scr)
		client := newVerifiedClient(t, srv, mailer, "p11-gib@example.com")
		resp := postReport(t, client, srv.URL, p11Body("flood", "low", "zzzzzzzzzzzzzzzzzzzz"))
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status %d, want 422", resp.StatusCode)
		}
		field, msg := decodeErrorResponse(t, resp)
		if field != "description" || msg != service.RejectMessage(service.RejectGibberish) {
			t.Fatalf("field %q message %q", field, msg)
		}
		if calls, _, _, _ := scr.snapshot(); calls != 0 {
			t.Fatalf("screener calls %d, want 0", calls)
		}
		if n := p11ReportCount(t, pool); n != 0 {
			t.Fatalf("reports stored %d, want 0", n)
		}
	})

	t.Run("violence_alone_posts", func(t *testing.T) {
		scr := p11Scores(map[string]float64{"violence": 1.0, "self-harm": 1.0})
		srv, mailer, pool := p11Server(t, scr)
		client := newVerifiedClient(t, srv, mailer, "p11-viol@example.com")
		status, body := p11Post(t, client, srv.URL, p11Body("flood", "critical", "Man trapped under a collapsed wall, bleeding badly"))
		if status != http.StatusCreated {
			t.Fatalf("status %d, want 201: %v", status, body)
		}
		id, held := p11Created(t, body)
		if held {
			t.Fatal("held true, want false")
		}
		if st := p11State(t, pool, id); st != "clear" {
			t.Fatalf("state %q, want clear", st)
		}
		row := p11CheckRow(t, pool, id)
		if row.Status != "done" || !row.NeedsReview {
			t.Fatalf("check row %+v, want done with needs_review", row)
		}
	})

	t.Run("pass_posts_clear", func(t *testing.T) {
		scr := p11Scores(map[string]float64{"hate": service.BorderlineScoreThreshold / 2})
		srv, mailer, pool := p11Server(t, scr)
		client := newVerifiedClient(t, srv, mailer, "p11-pass@example.com")
		status, body := p11Post(t, client, srv.URL, p11Body("flood", "low", text))
		if status != http.StatusCreated {
			t.Fatalf("status %d, want 201: %v", status, body)
		}
		id, held := p11Created(t, body)
		if held {
			t.Fatal("held true, want false")
		}
		if st := p11State(t, pool, id); st != "clear" {
			t.Fatalf("state %q, want clear", st)
		}
		if n := p11CheckCount(t, pool); n != 0 {
			t.Fatalf("check rows %d, want 0", n)
		}
		fr, ok := p11FeedRow(t, client, srv.URL, id, false)
		if !ok || fr.Visibility != "provisional" {
			t.Fatalf("feed row %+v found %v, want provisional", fr, ok)
		}
	})

	t.Run("borderline_posts_and_is_reviewable", func(t *testing.T) {
		scores := map[string]float64{"hate": service.BorderlineScoreThreshold}
		scr := p11Scores(scores)
		srv, mailer, pool := p11Server(t, scr)
		client := newVerifiedClient(t, srv, mailer, "p11-border@example.com")
		status, body := p11Post(t, client, srv.URL, p11Body("flood", "low", text))
		if status != http.StatusCreated {
			t.Fatalf("status %d, want 201: %v", status, body)
		}
		id, held := p11Created(t, body)
		if held {
			t.Fatal("held true, want false")
		}
		if st := p11State(t, pool, id); st != "clear" {
			t.Fatalf("state %q, want clear", st)
		}
		row := p11CheckRow(t, pool, id)
		if row.Status != "done" || row.Verdict == nil || *row.Verdict != "borderline" || !row.NeedsReview {
			t.Fatalf("check row %+v", row)
		}
		if row.Model == nil || *row.Model != moderation.DefaultModel {
			t.Fatalf("model %v, want %s", row.Model, moderation.DefaultModel)
		}
		var got map[string]float64
		if err := json.Unmarshal(row.Scores, &got); err != nil || got["hate"] != service.BorderlineScoreThreshold {
			t.Fatalf("scores %s (%v)", row.Scores, err)
		}
		fr, ok := p11FeedRow(t, client, srv.URL, id, false)
		if !ok || fr.Visibility != "provisional" {
			t.Fatalf("feed row %+v found %v, want provisional", fr, ok)
		}
	})
}

func TestSubmitScreeningSendsOnlyText(t *testing.T) {
	scr := p11Scores(map[string]float64{})
	srv, mailer, _ := p11Server(t, scr)
	const email = "p11-only-text@example.com"
	client := newVerifiedClient(t, srv, mailer, email)

	desc := "  Water rising near the school gate, two families on the roof  \n"
	status, body := p11Post(t, client, srv.URL, p11Body("flood", "low", desc))
	if status != http.StatusCreated {
		t.Fatalf("status %d, want 201: %v", status, body)
	}
	calls, texts, hadDeadline, remaining := scr.snapshot()
	if calls != 1 || len(texts) != 1 {
		t.Fatalf("screener calls %d, want 1", calls)
	}
	if texts[0] != strings.TrimSpace(desc) {
		t.Fatalf("screener received %q, want %q", texts[0], strings.TrimSpace(desc))
	}
	for _, leak := range []string{email, "12.97", "77.59"} {
		if strings.Contains(texts[0], leak) {
			t.Fatalf("screener text carries %q: %q", leak, texts[0])
		}
	}
	if !hadDeadline[0] || remaining[0] <= 0 || remaining[0] > service.InlineScreenTimeout {
		t.Fatalf("deadline set %v remaining %v, want in (0, %v]", hadDeadline[0], remaining[0], service.InlineScreenTimeout)
	}

	// Validation precedes the provider: a 400 never reaches the screener.
	resp := postReport(t, client, srv.URL, p11Body("not-a-category", "low", desc))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
	if field, _ := decodeErrorResponse(t, resp); field != "category" {
		t.Fatalf("field %q, want category", field)
	}
	if after, _, _, _ := scr.snapshot(); after != 1 {
		t.Fatalf("screener calls %d after a 400, want 1", after)
	}
}

func TestSubmitScreeningHeldNotInFeed(t *testing.T) {
	const text = "Water rising near the market bridge"

	check := func(t *testing.T, scr *p11Screener, email string) (int64, *http.Client, *httptest.Server, *pgxpool.Pool) {
		t.Helper()
		srv, mailer, pool := p11Server(t, scr)
		client := newVerifiedClient(t, srv, mailer, email)
		start := time.Now()
		status, body := p11Post(t, client, srv.URL, p11Body("flood", "low", text))
		if status != http.StatusCreated {
			t.Fatalf("status %d, want 201: %v", status, body)
		}
		if took := time.Since(start); took > 6*time.Second {
			t.Fatalf("post took %v, want under 6s", took)
		}
		id, held := p11Created(t, body)
		if !held {
			t.Fatal("held false, want true")
		}
		if st := p11State(t, pool, id); st != "pending" {
			t.Fatalf("state %q, want pending", st)
		}
		return id, client, srv, pool
	}

	t.Run("deadline_holds", func(t *testing.T) {
		id, client, srv, pool := check(t, &p11Screener{block: true}, "p11-deadline@example.com")
		row := p11CheckRow(t, pool, id)
		if row.Status != "pending" || row.Attempts != 0 || row.Verdict != nil {
			t.Fatalf("check row %+v, want pending, 0 attempts, no verdict", row)
		}
		if _, ok := p11FeedRow(t, client, srv.URL, id, false); ok {
			t.Fatal("held report listed in the default feed")
		}
		if _, ok := p11FeedRow(t, client, srv.URL, id, true); ok {
			t.Fatal("held report listed in the disputed view")
		}
	})

	t.Run("retry_after_honoured", func(t *testing.T) {
		scr := &p11Screener{err: &moderation.CallError{Status: 429, Transient: true, RetryAfter: 120 * time.Second}}
		id, _, _, pool := check(t, scr, "p11-retry@example.com")
		var later bool
		err := pool.QueryRow(context.Background(),
			`SELECT next_attempt_at > now() + interval '100 seconds' FROM moderation_checks WHERE report_id = $1`, id).Scan(&later)
		if err != nil || !later {
			t.Fatalf("next_attempt_at later than 100 s: %v (%v)", later, err)
		}
	})

	t.Run("permanent_error_fails_closed", func(t *testing.T) {
		check(t, &p11Screener{err: &moderation.CallError{Status: 401}}, "p11-perm@example.com")
	})
}

func TestSubmitScreeningPendingSeriousPublished(t *testing.T) {
	scr := &p11Screener{err: &moderation.CallError{Status: 503, Transient: true}}
	srv, mailer, pool := p11Server(t, scr)
	client := newVerifiedClient(t, srv, mailer, "p11-serious@example.com")

	for _, c := range [][2]string{{"flood", "critical"}, {"traffic", "critical"}} {
		status, body := p11Post(t, client, srv.URL, p11Body(c[0], c[1], "Family stuck on a roof, water rising fast"))
		if status != http.StatusCreated {
			t.Fatalf("%v: status %d, want 201: %v", c, status, body)
		}
		id, held := p11Created(t, body)
		if held {
			t.Fatalf("%v: held true, want false", c)
		}
		if st := p11State(t, pool, id); st != "pending" {
			t.Fatalf("%v: state %q, want pending", c, st)
		}
		if row := p11CheckRow(t, pool, id); row.Status != "pending" {
			t.Fatalf("%v: check row %+v, want pending", c, row)
		}
		fr, ok := p11FeedRow(t, client, srv.URL, id, false)
		if !ok || fr.Visibility != "live" || fr.VisibilityReason != "critical_bypasses_gates" {
			t.Fatalf("%v: feed row %+v found %v, want live via the critical bypass", c, fr, ok)
		}
	}

	// A new post can no longer be rescue_needed.
	status, _ := p11Post(t, client, srv.URL, p11Body("rescue_needed", "low", "Family stuck on a roof, water rising fast"))
	if status != http.StatusBadRequest {
		t.Fatalf("rescue_needed post: status %d, want 400", status)
	}

	// The read side rule for a row already stored as rescue_needed is
	// unchanged: an unfinished text check still shows it live through the
	// critical or rescue bypass.
	stored := storeReportAs(t, pool, client, srv.URL, p11Body("rescue_needed", "low", "Family stuck on a roof, water rising fast"))
	testutil.MustExec(t, pool, `UPDATE reports SET moderation_state = 'pending' WHERE id = $1`, stored)
	fr, ok := p11FeedRow(t, client, srv.URL, stored, false)
	if !ok || fr.Visibility != "live" || fr.VisibilityReason != "critical_bypasses_gates" {
		t.Fatalf("stored pending rescue: feed row %+v found %v, want live via the critical bypass", fr, ok)
	}
}

// TestSubmitScreeningSwaggerFields reads the committed API reference and checks
// what this plan documents: held on the created report, the 422 on the post
// route, and no withheld value in any documented enum (R-01). It needs no
// database.
func TestSubmitScreeningSwaggerFields(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "swagger.json"))
	if err != nil {
		t.Fatalf("reading docs/swagger.json: %v", err)
	}
	var spec struct {
		Definitions map[string]struct {
			Properties map[string]struct {
				Type string `json:"type"`
			} `json:"properties"`
		} `json:"definitions"`
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Schema struct {
					Ref string `json:"$ref"`
				} `json:"schema"`
			} `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parsing docs/swagger.json: %v", err)
	}
	if got := spec.Definitions["handlers.ReportResponse"].Properties["held"].Type; got != "boolean" {
		t.Fatalf("handlers.ReportResponse.held type %q, want boolean", got)
	}
	got := spec.Paths["/reports"]["post"].Responses["422"].Schema.Ref
	if got != "#/definitions/handlers.ErrorResponse" {
		t.Fatalf("POST /reports 422 schema %q, want handlers.ErrorResponse", got)
	}
	for _, banned := range []string{"withheld", "awaiting_text_check", "removed_after_review"} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("docs/swagger.json mentions %q", banned)
		}
	}
}
