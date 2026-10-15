// flag_e2e_test.go is V-20 for plan 04-20: it proves the flag route end to end
// over the real router and a real Postgres. Every server is built through
// newE2EServerWith with p20Tweak, which opens all seven buckets first so a test
// only ever meets the bucket it sets. Every package level identifier carries
// the p20 prefix. The database tests run only on the disposable database
// p4_0420.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	p20Lat      = 12.9716
	p20Lon      = 77.5946
	p20IPHeader = "X-P20-Client-IP"

	// p20Desc is a description the text check clears.
	p20Desc = "Water rising near the market bridge"
)

// p20T0 is a past instant, so a refill that read the wall clock would be seen.
var p20T0 = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// p20Open is a burst no test reaches, so it never refuses on a frozen clock.
var p20Open = api.BucketLimit{Every: time.Millisecond, Burst: 1000}

// p20Clock is a frozen limiter clock that only moves when Advance is called.
type p20Clock struct {
	mu  sync.Mutex
	now time.Time
}

func newP20Clock() *p20Clock { return &p20Clock{now: p20T0} }

func (c *p20Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *p20Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// p20Tweak builds the tweak hook for newE2EServerWith. It opens all seven
// buckets, injects the clock, opens the request link limiter (several accounts
// verify from one address), then runs mutate. A frozen clock never refills, so
// no bucket a case does not exercise may be left at a default.
func p20Tweak(clk *p20Clock, mutate func(d *api.Deps)) func(d *api.Deps, q *sqlcgen.Queries) {
	return func(d *api.Deps, q *sqlcgen.Queries) {
		d.RateLimits = api.RateLimits{
			ReportSerious: p20Open,
			ReportOther:   p20Open,
			Vote:          p20Open,
			VoteChange:    p20Open,
			Flag:          p20Open,
			SubmitAttempt: p20Open,
			IPBackstop:    p20Open,
		}
		d.LimiterClock = clk.Now
		d.RequestLinkRateLimit = api.RequestLinkRateLimit{Burst: 100, Every: time.Millisecond}
		if mutate != nil {
			mutate(d)
		}
	}
}

// p20Body is a report body at the shared point.
func p20Body(category, severity string) map[string]any {
	return map[string]any{
		"category":    category,
		"severity":    severity,
		"description": p20Desc,
		"latitude":    p20Lat,
		"longitude":   p20Lon,
	}
}

// p20PostFlagRaw posts rawBody to the flag route of reportID-as-text, sending
// p20IPHeader only when ip is non empty.
func p20PostFlagRaw(t *testing.T, client *http.Client, url, rawBody, ip string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(rawBody))
	if err != nil {
		t.Fatalf("building flag request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ip != "" {
		req.Header.Set(p20IPHeader, ip)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// p20Flag flags reportID with reason, sending the address header when ip is set.
func p20Flag(t *testing.T, client *http.Client, baseURL string, reportID int64, reason, ip string) *http.Response {
	t.Helper()
	buf, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		t.Fatalf("marshalling flag body: %v", err)
	}
	url := baseURL + "/api/reports/" + strconv.FormatInt(reportID, 10) + "/flag"
	return p20PostFlagRaw(t, client, url, string(buf), ip)
}

// p20Status flags and requires the status, returning the raw body.
func p20Status(t *testing.T, label string, resp *http.Response, want int) []byte {
	t.Helper()
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

// p20WantError requires status want and the error field and message.
func p20WantError(t *testing.T, label string, resp *http.Response, want int, field, message string) {
	t.Helper()
	if resp.StatusCode != want {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("%s: status = %d, want %d, body %s", label, resp.StatusCode, want, b)
	}
	gotField, gotMessage := decodeErrorResponse(t, resp)
	if gotField != field || gotMessage != message {
		t.Fatalf("%s: error = (%q, %q), want (%q, %q)", label, gotField, gotMessage, field, message)
	}
}

// p20FlagRows maps the account id of every flag on reportID to its reason.
func p20FlagRows(t *testing.T, pool *pgxpool.Pool, reportID int64) map[int64]string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT account_id, reason FROM report_flags WHERE report_id = $1`, reportID)
	if err != nil {
		t.Fatalf("reading flags of report %d: %v", reportID, err)
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var reason string
		if err := rows.Scan(&id, &reason); err != nil {
			t.Fatalf("scanning a flag row: %v", err)
		}
		out[id] = reason
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading flag rows: %v", err)
	}
	return out
}

// p20WantFlagRows fails unless the flags of reportID are exactly want.
func p20WantFlagRows(t *testing.T, label string, pool *pgxpool.Pool, reportID int64, want map[int64]string) {
	t.Helper()
	got := p20FlagRows(t, pool, reportID)
	if len(got) != len(want) {
		t.Fatalf("%s: flags on report %d = %v, want %v", label, reportID, got, want)
	}
	for id, reason := range want {
		if got[id] != reason {
			t.Fatalf("%s: flags on report %d = %v, want %v", label, reportID, got, want)
		}
	}
}

// p20AccountID reads the account id of an email.
func p20AccountID(t *testing.T, pool *pgxpool.Pool, email string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), `SELECT id FROM accounts WHERE email = $1`, email).Scan(&id); err != nil {
		t.Fatalf("reading the account id of %s: %v", email, err)
	}
	return id
}

// p20SecondSession returns a client whose fresh session is bound to accountID,
// the way a second device of the same verified account looks.
func p20SecondSession(t *testing.T, baseURL string, pool *pgxpool.Pool, accountID int64) *http.Client {
	t.Helper()
	c := newJarClient(t)
	resp, err := c.Get(baseURL + "/login")
	if err != nil {
		t.Fatalf("GET /login for the second session: %v", err)
	}
	resp.Body.Close()
	sid := sessionIDFromJar(t, c, baseURL)
	err = sqlcgen.New(pool).BindSessionAccount(context.Background(), sqlcgen.BindSessionAccountParams{SessionID: sid, AccountID: &accountID})
	if err != nil {
		t.Fatalf("binding the second session: %v", err)
	}
	return c
}

// p20SetModeration writes a moderation state straight onto the row.
func p20SetModeration(t *testing.T, pool *pgxpool.Pool, reportID int64, state string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE reports SET moderation_state = $1 WHERE id = $2`, state, reportID); err != nil {
		t.Fatalf("setting moderation_state %q on report %d: %v", state, reportID, err)
	}
}

// p20Expire moves the expiry of a report to one minute ago.
func p20Expire(t *testing.T, pool *pgxpool.Pool, reportID int64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE reports SET expires_at = now() - interval '1 minute' WHERE id = $1`, reportID); err != nil {
		t.Fatalf("expiring report %d: %v", reportID, err)
	}
}

// p20ReportRow is the whole reports row as one JSON text.
func p20ReportRow(t *testing.T, pool *pgxpool.Pool, reportID int64) string {
	t.Helper()
	var row string
	if err := pool.QueryRow(context.Background(), `SELECT to_jsonb(r)::text FROM reports r WHERE r.id = $1`, reportID).Scan(&row); err != nil {
		t.Fatalf("reading the row of report %d: %v", reportID, err)
	}
	return row
}

// p20FeedObjects reads the default feed view and returns every element as raw
// JSON keyed by its id.
func p20FeedObjects(t *testing.T, client *http.Client, baseURL string) map[int64]json.RawMessage {
	t.Helper()
	url := baseURL + "/api/reports?lat=" + strconv.FormatFloat(p20Lat, 'f', -1, 64) +
		"&lon=" + strconv.FormatFloat(p20Lon, 'f', -1, 64) + "&radius_km=5"
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	raw := p20Status(t, "GET /api/reports", resp, http.StatusOK)
	var out struct {
		Reports []json.RawMessage `json:"reports"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the feed %s: %v", raw, err)
	}
	objects := map[int64]json.RawMessage{}
	for _, obj := range out.Reports {
		var head struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(obj, &head); err != nil {
			t.Fatalf("decoding a feed element %s: %v", obj, err)
		}
		objects[head.ID] = obj
	}
	return objects
}

// p20RateBody is a decoded 429.
type p20RateBody struct {
	Status            int
	RetryAfterHeader  string
	Keys              []string
	Field             string
	Message           string
	RetryAfterSeconds int
	Serious           bool
	HasNumber         bool
}

// p20Decode429 decodes a rate limited response and fails unless it is a 429.
func p20Decode429(t *testing.T, resp *http.Response) p20RateBody {
	t.Helper()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the 429 body: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429, body %s", resp.StatusCode, raw)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decoding the 429 body %s: %v", raw, err)
	}
	out := p20RateBody{Status: resp.StatusCode, RetryAfterHeader: resp.Header.Get("Retry-After")}
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
		t.Fatalf("decoding the 429 fields %s: %v", raw, err)
	}
	out.Field = parsed.Error.Field
	out.Message = parsed.Error.Message
	out.RetryAfterSeconds = parsed.RetryAfterSeconds
	out.Serious = parsed.Serious
	return out
}

// p20LogBuffer is a mutex guarded writer for the captured standard log.
type p20LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *p20LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// Lines returns the captured non empty lines.
func (b *p20LogBuffer) Lines() []string {
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

// p20CaptureLog redirects the standard logger into a buffer with no flags until
// the test ends, then restores the writer and the flags.
func p20CaptureLog(t *testing.T) *p20LogBuffer {
	t.Helper()
	buf := &p20LogBuffer{}
	prevWriter, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevWriter)
		log.SetFlags(prevFlags)
	})
	return buf
}

func TestFlagOnePerAccount(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	a := newVerifiedClient(t, srv, mailer, "p20-one-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p20-one-b@example.com")
	c := newVerifiedClient(t, srv, mailer, "p20-one-c@example.com")
	d := newVerifiedClient(t, srv, mailer, "p20-one-d@example.com")
	idB := p20AccountID(t, pool, "p20-one-b@example.com")
	idC := p20AccountID(t, pool, "p20-one-c@example.com")
	r1 := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))

	resp := p20Flag(t, b, srv.URL, r1, "spam", "")
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	raw := p20Status(t, "first flag", resp, http.StatusCreated)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decoding the 201 body %s: %v", raw, err)
	}
	if len(body) != 1 || body["flagged"] != true {
		t.Fatalf("201 body = %s, want only {\"flagged\":true}", raw)
	}
	p20WantFlagRows(t, "after B", pool, r1, map[int64]string{idB: "spam"})

	p20WantError(t, "repeat flag", p20Flag(t, b, srv.URL, r1, "abusive", ""), http.StatusConflict, "report", service.FlagAlreadyMessage)
	p20WantFlagRows(t, "after B again", pool, r1, map[int64]string{idB: "spam"})

	p20Status(t, "C flag", p20Flag(t, c, srv.URL, r1, "false", ""), http.StatusCreated)
	p20WantFlagRows(t, "after C", pool, r1, map[int64]string{idB: "spam", idC: "false"})

	if _, err := pool.Exec(context.Background(), `UPDATE report_flags SET dismissed_at = now() WHERE report_id = $1 AND account_id = $2`, r1, idB); err != nil {
		t.Fatalf("dismissing B's flag: %v", err)
	}
	p20WantError(t, "flag after a dismissal", p20Flag(t, b, srv.URL, r1, "other", ""), http.StatusConflict, "report", service.FlagAlreadyMessage)
	p20WantFlagRows(t, "after the dismissal", pool, r1, map[int64]string{idB: "spam", idC: "false"})

	idURL := srv.URL + "/api/reports/" + strconv.FormatInt(r1, 10) + "/flag"
	p20WantError(t, "unknown reason", p20Flag(t, d, srv.URL, r1, "hoax", ""), http.StatusBadRequest, "reason", service.FlagReasonMessage)
	p20WantError(t, "empty object", p20PostFlagRaw(t, d, idURL, `{}`, ""), http.StatusBadRequest, "reason", service.FlagReasonMessage)
	p20WantError(t, "unknown key", p20PostFlagRaw(t, d, idURL, `{"reason":"spam","note":"x"}`, ""), http.StatusBadRequest, "body", "Request body is missing or malformed.")
	p20WantError(t, "not json", p20PostFlagRaw(t, d, idURL, `not json`, ""), http.StatusBadRequest, "body", "Request body is missing or malformed.")
	p20WantError(t, "bad id", p20PostFlagRaw(t, d, srv.URL+"/api/reports/abc/flag", `{"reason":"spam"}`, ""), http.StatusBadRequest, "id", "Report id must be a number.")
	p20WantFlagRows(t, "after D's refusals", pool, r1, map[int64]string{idB: "spam", idC: "false"})

	anon := newJarClient(t)
	loginResp, err := anon.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	loginResp.Body.Close()
	if resp := p20Flag(t, anon, srv.URL, r1, "spam", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unverified flag: status = %d, want 401", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	p20WantFlagRows(t, "after the unverified call", pool, r1, map[int64]string{idB: "spam", idC: "false"})
}

func TestFlagOwnReport(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	a := newVerifiedClient(t, srv, mailer, "p20-own-a@example.com")
	idA := p20AccountID(t, pool, "p20-own-a@example.com")
	r1 := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))

	p20WantError(t, "own report", p20Flag(t, a, srv.URL, r1, "spam", ""), http.StatusForbidden, "report", service.FlagOwnReportMessage)
	p20WantFlagRows(t, "after the own flag", pool, r1, map[int64]string{})

	second := p20SecondSession(t, srv.URL, pool, idA)
	p20WantError(t, "own report from a second session", p20Flag(t, second, srv.URL, r1, "spam", ""), http.StatusForbidden, "report", service.FlagOwnReportMessage)
	p20WantFlagRows(t, "after the second session", pool, r1, map[int64]string{})
}

func TestFlagWithheldReport(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	a := newVerifiedClient(t, srv, mailer, "p20-held-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p20-held-b@example.com")
	idB := p20AccountID(t, pool, "p20-held-b@example.com")

	removedLow := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	removedCritical := submitReportGetID(t, a, srv.URL, p20Body("flood", "critical"))
	heldLow := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	pendingCritical := submitReportGetID(t, a, srv.URL, p20Body("flood", "critical"))
	pendingRescue := submitReportGetID(t, a, srv.URL, p20Body("rescue_needed", "low"))
	expired := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	p20SetModeration(t, pool, removedLow, "removed")
	p20SetModeration(t, pool, removedCritical, "removed")
	p20SetModeration(t, pool, heldLow, "pending")
	p20SetModeration(t, pool, pendingCritical, "pending")
	p20SetModeration(t, pool, pendingRescue, "pending")
	p20Expire(t, pool, expired)

	var maxID int64
	if err := pool.QueryRow(context.Background(), `SELECT max(id) FROM reports`).Scan(&maxID); err != nil {
		t.Fatalf("reading the highest report id: %v", err)
	}
	missing := maxID + 1000

	wantBody := p20Status(t, "missing report", p20Flag(t, b, srv.URL, missing, "spam", ""), http.StatusNotFound)
	var parsed struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(wantBody, &parsed); err != nil {
		t.Fatalf("decoding the 404 body %s: %v", wantBody, err)
	}
	if parsed.Error.Field != "report" || parsed.Error.Message != service.FlagNotFoundMessage {
		t.Fatalf("missing report error = %+v, want field report with FlagNotFoundMessage", parsed.Error)
	}

	for name, id := range map[string]int64{"removed low": removedLow, "removed critical": removedCritical, "pending low": heldLow} {
		got := p20Status(t, name, p20Flag(t, b, srv.URL, id, "spam", ""), http.StatusNotFound)
		if !bytes.Equal(got, wantBody) {
			t.Errorf("%s: body %s differs from the missing report body %s", name, got, wantBody)
		}
		p20WantFlagRows(t, name, pool, id, map[int64]string{})
	}

	// The author's own withheld report reads as missing, never as a 403.
	got := p20Status(t, "author on own removed report", p20Flag(t, a, srv.URL, removedLow, "spam", ""), http.StatusNotFound)
	if !bytes.Equal(got, wantBody) {
		t.Errorf("author on own removed report: body %s differs from %s", got, wantBody)
	}

	for name, id := range map[string]int64{"pending critical": pendingCritical, "pending rescue": pendingRescue, "expired": expired} {
		p20Status(t, name, p20Flag(t, b, srv.URL, id, "dangerous", ""), http.StatusCreated)
		p20WantFlagRows(t, name, pool, id, map[int64]string{idB: "dangerous"})
	}
}

func TestFlagRateLimit(t *testing.T) {
	// postN makes n reports by author and returns their ids.
	postN := func(t *testing.T, author *http.Client, url string, n int) []int64 {
		t.Helper()
		var ids []int64
		for i := 0; i < n; i++ {
			ids = append(ids, submitReportGetID(t, author, url, p20Body("flood", "low")))
		}
		return ids
	}

	t.Run("a the default shape", func(t *testing.T) {
		if got, want := api.RateLimitsDefault.Flag, (api.BucketLimit{Every: 2 * time.Minute, Burst: 10}); got != want {
			t.Fatalf("RateLimitsDefault.Flag = %+v, want %+v", got, want)
		}
	})

	t.Run("b the burst, the exact wait and per account scope", func(t *testing.T) {
		clk := newP20Clock()
		srv, mailer, pool := newE2EServerWith(t, p20Tweak(clk, func(d *api.Deps) {
			d.RateLimits.Flag = api.BucketLimit{Every: time.Hour, Burst: 2}
		}))
		a := newVerifiedClient(t, srv, mailer, "p20-rate-b-a@example.com")
		b := newVerifiedClient(t, srv, mailer, "p20-rate-b-b@example.com")
		c := newVerifiedClient(t, srv, mailer, "p20-rate-b-c@example.com")
		idB := p20AccountID(t, pool, "p20-rate-b-b@example.com")
		ids := postN(t, a, srv.URL, 3)

		p20Status(t, "r1", p20Flag(t, b, srv.URL, ids[0], "spam", ""), http.StatusCreated)
		p20Status(t, "r2", p20Flag(t, b, srv.URL, ids[1], "spam", ""), http.StatusCreated)
		got := p20Decode429(t, p20Flag(t, b, srv.URL, ids[2], "spam", ""))
		if got.RetryAfterHeader != "3600" {
			t.Errorf("Retry-After = %q, want 3600", got.RetryAfterHeader)
		}
		if got.Field != "rate_flag" || got.Message != service.FlagLimitMessage(time.Hour) {
			t.Errorf("429 error = (%q, %q), want rate_flag with FlagLimitMessage(1h)", got.Field, got.Message)
		}
		if got.RetryAfterSeconds != 3600 || got.Serious || got.HasNumber {
			t.Errorf("429 body = %+v, want 3600 seconds, serious false, no emergency_number", got)
		}
		p20WantFlagRows(t, "blocked flag", pool, ids[2], map[int64]string{})

		p20Status(t, "C is unaffected", p20Flag(t, c, srv.URL, ids[2], "spam", ""), http.StatusCreated)
		clk.Advance(time.Hour)
		p20Status(t, "B after the wait", p20Flag(t, b, srv.URL, ids[2], "spam", ""), http.StatusCreated)
		p20WantFlagRows(t, "after the wait", pool, ids[2], map[int64]string{idB: "spam", p20AccountID(t, pool, "p20-rate-b-c@example.com"): "spam"})
	})

	t.Run("c an admitted repeat costs a token", func(t *testing.T) {
		srv, mailer, _ := newE2EServerWith(t, p20Tweak(newP20Clock(), func(d *api.Deps) {
			d.RateLimits.Flag = api.BucketLimit{Every: time.Hour, Burst: 2}
		}))
		a := newVerifiedClient(t, srv, mailer, "p20-rate-c-a@example.com")
		b := newVerifiedClient(t, srv, mailer, "p20-rate-c-b@example.com")
		ids := postN(t, a, srv.URL, 2)

		p20Status(t, "r1", p20Flag(t, b, srv.URL, ids[0], "spam", ""), http.StatusCreated)
		p20WantError(t, "r1 again", p20Flag(t, b, srv.URL, ids[0], "spam", ""), http.StatusConflict, "report", service.FlagAlreadyMessage)
		if got := p20Decode429(t, p20Flag(t, b, srv.URL, ids[1], "spam", "")); got.Field != "rate_flag" {
			t.Errorf("field = %q, want rate_flag", got.Field)
		}
	})

	t.Run("d a second session shares the account bucket", func(t *testing.T) {
		srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), func(d *api.Deps) {
			d.RateLimits.Flag = api.BucketLimit{Every: time.Hour, Burst: 1}
		}))
		a := newVerifiedClient(t, srv, mailer, "p20-rate-d-a@example.com")
		b := newVerifiedClient(t, srv, mailer, "p20-rate-d-b@example.com")
		idB := p20AccountID(t, pool, "p20-rate-d-b@example.com")
		ids := postN(t, a, srv.URL, 2)
		second := p20SecondSession(t, srv.URL, pool, idB)

		p20Status(t, "r1", p20Flag(t, b, srv.URL, ids[0], "spam", ""), http.StatusCreated)
		if got := p20Decode429(t, p20Flag(t, second, srv.URL, ids[1], "spam", "")); got.Field != "rate_flag" {
			t.Errorf("field = %q, want rate_flag", got.Field)
		}
	})

	t.Run("e the address backstop", func(t *testing.T) {
		srv, mailer, _ := newE2EServerWith(t, p20Tweak(newP20Clock(), func(d *api.Deps) {
			d.ClientIPHeader = p20IPHeader
			d.RateLimits.IPBackstop = api.BucketLimit{Every: time.Hour, Burst: 1}
		}))
		a := newVerifiedClient(t, srv, mailer, "p20-rate-e-a@example.com")
		b := newVerifiedClient(t, srv, mailer, "p20-rate-e-b@example.com")
		c := newVerifiedClient(t, srv, mailer, "p20-rate-e-c@example.com")
		d := newVerifiedClient(t, srv, mailer, "p20-rate-e-d@example.com")
		ids := postN(t, a, srv.URL, 1)

		p20Status(t, "B from .7", p20Flag(t, b, srv.URL, ids[0], "spam", "203.0.113.7"), http.StatusCreated)
		got := p20Decode429(t, p20Flag(t, c, srv.URL, ids[0], "spam", "203.0.113.7"))
		if got.Field != "rate_flag" || got.Serious {
			t.Errorf("backstop 429 = %+v, want rate_flag and serious false", got)
		}
		p20Status(t, "C from .8", p20Flag(t, c, srv.URL, ids[0], "spam", "203.0.113.8"), http.StatusCreated)
		p20Status(t, "D with no header", p20Flag(t, d, srv.URL, ids[0], "spam", ""), http.StatusCreated)
	})

	t.Run("f block log lines", func(t *testing.T) {
		srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), func(d *api.Deps) {
			d.ClientIPHeader = p20IPHeader
			d.RateLimits.IPBackstop = api.BucketLimit{Every: time.Hour, Burst: 1}
			d.RateLimits.Flag = api.BucketLimit{Every: time.Hour, Burst: 1}
		}))
		const emailB = "p20-rate-f-b@example.com"
		a := newVerifiedClient(t, srv, mailer, "p20-rate-f-a@example.com")
		b := newVerifiedClient(t, srv, mailer, emailB)
		c := newVerifiedClient(t, srv, mailer, "p20-rate-f-c@example.com")
		idB := strconv.FormatInt(p20AccountID(t, pool, emailB), 10)
		ids := postN(t, a, srv.URL, 3)
		logs := p20CaptureLog(t)

		p20Status(t, "C from .7", p20Flag(t, c, srv.URL, ids[0], "spam", "203.0.113.7"), http.StatusCreated)
		p20Decode429(t, p20Flag(t, b, srv.URL, ids[0], "spam", "203.0.113.7"))
		p20Status(t, "B with no header", p20Flag(t, b, srv.URL, ids[1], "spam", ""), http.StatusCreated)
		p20Decode429(t, p20Flag(t, b, srv.URL, ids[2], "spam", ""))

		var blocks []string
		for _, line := range logs.Lines() {
			if strings.Contains(line, "bucket=") {
				blocks = append(blocks, line)
			}
			if strings.Contains(line, emailB) || strings.Contains(line, "203.0.113.") {
				t.Errorf("log line %q carries an email or an address", line)
			}
		}
		if len(blocks) != 2 {
			t.Fatalf("block lines = %q, want 2", blocks)
		}
		if !strings.Contains(blocks[0], "bucket=ip_backstop ") || !strings.Contains(blocks[1], "bucket=flag ") {
			t.Errorf("block lines = %q, want the backstop then the flag bucket", blocks)
		}
		for _, line := range blocks {
			if !strings.Contains(line, " account="+idB+" ") {
				t.Errorf("block line %q does not carry the numeric account id %s", line, idB)
			}
		}
	})
}

func TestFlagNeverChangesVisibility(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	a := newVerifiedClient(t, srv, mailer, "p20-vis-a@example.com")
	viewer := newVerifiedClient(t, srv, mailer, "p20-vis-v@example.com")
	low := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	critical := submitReportGetID(t, a, srv.URL, p20Body("flood", "critical"))

	before := p20FeedObjects(t, viewer, srv.URL)
	for _, id := range []int64{low, critical} {
		if _, ok := before[id]; !ok {
			t.Fatalf("report %d is not in the viewer's feed before any flag", id)
		}
	}
	rowLow, rowCritical := p20ReportRow(t, pool, low), p20ReportRow(t, pool, critical)

	reasons := []string{"spam", "abusive", "false", "dangerous", "other"}
	for i, reason := range reasons {
		c := newVerifiedClient(t, srv, mailer, "p20-vis-f"+strconv.Itoa(i)+"@example.com")
		p20Status(t, "flag low "+reason, p20Flag(t, c, srv.URL, low, reason, ""), http.StatusCreated)
		p20Status(t, "flag critical "+reason, p20Flag(t, c, srv.URL, critical, reasons[(i+1)%len(reasons)], ""), http.StatusCreated)
	}

	after := p20FeedObjects(t, viewer, srv.URL)
	for _, id := range []int64{low, critical} {
		got, ok := after[id]
		if !ok {
			t.Fatalf("report %d left the viewer's feed after five flags", id)
		}
		if !bytes.Equal(got, before[id]) {
			t.Errorf("report %d: feed object changed\nbefore %s\nafter  %s", id, before[id], got)
		}
	}
	if got := p20ReportRow(t, pool, low); got != rowLow {
		t.Errorf("low report row changed\nbefore %s\nafter  %s", rowLow, got)
	}
	if got := p20ReportRow(t, pool, critical); got != rowCritical {
		t.Errorf("critical report row changed\nbefore %s\nafter  %s", rowCritical, got)
	}
	for _, id := range []int64{low, critical} {
		var state string
		if err := pool.QueryRow(context.Background(), `SELECT moderation_state FROM reports WHERE id = $1`, id).Scan(&state); err != nil {
			t.Fatalf("reading moderation_state: %v", err)
		}
		if state != "clear" {
			t.Errorf("report %d moderation_state = %q, want clear", id, state)
		}
	}
}

// p20ViewerFlagged reads the feed as client and returns the raw object of every
// report. It also checks, on every element, that viewer_flagged is present as a
// JSON boolean and that no other key names a flag (no count, no identity).
func p20ViewerFlagged(t *testing.T, label string, client *http.Client, baseURL string) (map[int64]bool, map[int64]json.RawMessage) {
	t.Helper()
	objects := p20FeedObjects(t, client, baseURL)
	out := map[int64]bool{}
	for id, obj := range objects {
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(obj, &keys); err != nil {
			t.Fatalf("%s: decoding report %d: %v", label, id, err)
		}
		raw, ok := keys["viewer_flagged"]
		if !ok {
			t.Fatalf("%s: report %d has no viewer_flagged key", label, id)
		}
		var flagged bool
		if err := json.Unmarshal(raw, &flagged); err != nil {
			t.Fatalf("%s: report %d viewer_flagged = %s, want a JSON boolean", label, id, raw)
		}
		out[id] = flagged
		for k := range keys {
			if k != "viewer_flagged" && strings.Contains(strings.ToLower(k), "flag") {
				t.Errorf("%s: report %d carries the extra key %q", label, id, k)
			}
		}
	}
	return out, objects
}

func TestFlagViewerMarker(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	a := newVerifiedClient(t, srv, mailer, "p20-mark-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p20-mark-b@example.com")
	c := newVerifiedClient(t, srv, mailer, "p20-mark-c@example.com")
	idB := p20AccountID(t, pool, "p20-mark-b@example.com")
	r1 := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	r2 := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))

	_, authorBefore := p20ViewerFlagged(t, "A before", a, srv.URL)
	p20Status(t, "B flags r1", p20Flag(t, b, srv.URL, r1, "spam", ""), http.StatusCreated)

	got, _ := p20ViewerFlagged(t, "B", b, srv.URL)
	if !got[r1] || got[r2] {
		t.Errorf("B's viewer_flagged = r1 %v, r2 %v, want true and false", got[r1], got[r2])
	}
	got, _ = p20ViewerFlagged(t, "C", c, srv.URL)
	if got[r1] || got[r2] {
		t.Errorf("C's viewer_flagged = r1 %v, r2 %v, want false and false", got[r1], got[r2])
	}
	got, authorAfter := p20ViewerFlagged(t, "A after", a, srv.URL)
	if got[r1] || got[r2] {
		t.Errorf("A's viewer_flagged = r1 %v, r2 %v, want false and false", got[r1], got[r2])
	}
	if !bytes.Equal(authorAfter[r1], authorBefore[r1]) {
		t.Errorf("the author's object for r1 changed after B flagged it\nbefore %s\nafter  %s", authorBefore[r1], authorAfter[r1])
	}

	if _, err := pool.Exec(context.Background(), `UPDATE report_flags SET dismissed_at = now() WHERE report_id = $1 AND account_id = $2`, r1, idB); err != nil {
		t.Fatalf("dismissing B's flag: %v", err)
	}
	got, _ = p20ViewerFlagged(t, "B after dismissal", b, srv.URL)
	if !got[r1] {
		t.Errorf("a dismissed flag must still read true for B")
	}

	p20Status(t, "B flags r2", p20Flag(t, b, srv.URL, r2, "other", ""), http.StatusCreated)
	got, _ = p20ViewerFlagged(t, "B with both", b, srv.URL)
	if !got[r1] || !got[r2] {
		t.Errorf("B's viewer_flagged = r1 %v, r2 %v, want true and true", got[r1], got[r2])
	}
}

func TestFlagSwaggerCoversRoute(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "swagger.json"))
	if err != nil {
		t.Fatalf("reading docs/swagger.json: %v", err)
	}
	var spec struct {
		Paths       map[string]map[string]json.RawMessage `json:"paths"`
		Definitions map[string]struct {
			Properties map[string]map[string]any `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("decoding docs/swagger.json: %v", err)
	}

	op, ok := spec.Paths["/reports/{id}/flag"]["post"]
	if !ok {
		t.Fatal("docs/swagger.json has no post operation for /reports/{id}/flag")
	}
	p15Assert429(t, "/reports/{id}/flag post", op)

	var parsed struct {
		Parameters []struct {
			In     string `json:"in"`
			Schema struct {
				Ref string `json:"$ref"`
			} `json:"schema"`
		} `json:"parameters"`
		Responses map[string]struct {
			Schema struct {
				Ref string `json:"$ref"`
			} `json:"schema"`
		} `json:"responses"`
	}
	if err := json.Unmarshal(op, &parsed); err != nil {
		t.Fatalf("decoding the flag operation: %v", err)
	}
	for _, code := range []string{"201", "400", "401", "403", "404", "409", "429", "500"} {
		if _, ok := parsed.Responses[code]; !ok {
			t.Errorf("the flag operation documents no %s response", code)
		}
	}
	if got := parsed.Responses["201"].Schema.Ref; got != "#/definitions/handlers.FlagResponse" {
		t.Errorf("201 schema = %q, want #/definitions/handlers.FlagResponse", got)
	}
	bodyRef := ""
	for _, p := range parsed.Parameters {
		if p.In == "body" {
			bodyRef = p.Schema.Ref
		}
	}
	if bodyRef != "#/definitions/handlers.FlagRequest" {
		t.Errorf("body parameter schema = %q, want #/definitions/handlers.FlagRequest", bodyRef)
	}

	reqProps := spec.Definitions["handlers.FlagRequest"].Properties
	if len(reqProps) != 1 {
		t.Errorf("FlagRequest has %d properties, want only reason", len(reqProps))
	}
	reason, ok := reqProps["reason"]
	if !ok || reason["type"] != "string" {
		t.Fatalf("FlagRequest.reason = %v, want a string property", reason)
	}
	enum, ok := reason["enum"].([]any)
	if !ok {
		t.Fatalf("FlagRequest.reason enum = %v, want a list", reason["enum"])
	}
	if len(enum) != len(service.FlagReasons) {
		t.Fatalf("FlagRequest.reason enum = %v, want %v", enum, service.FlagReasons)
	}
	for i, el := range enum {
		s, isString := el.(string)
		if !isString {
			t.Fatalf("FlagRequest.reason enum element %d = %#v (%T), want a JSON string", i, el, el)
		}
		if s != string(service.FlagReasons[i]) {
			t.Errorf("FlagRequest.reason enum element %d = %q, want %q", i, s, service.FlagReasons[i])
		}
	}

	respProps := spec.Definitions["handlers.FlagResponse"].Properties
	if len(respProps) != 1 || respProps["flagged"]["type"] != "boolean" {
		t.Errorf("FlagResponse properties = %v, want only flagged as a boolean", respProps)
	}
	if got := spec.Definitions["handlers.FeedReportResponse"].Properties["viewer_flagged"]["type"]; got != "boolean" {
		t.Errorf("FeedReportResponse.viewer_flagged type = %v, want boolean", got)
	}
	for _, def := range []string{"handlers.FlagRequest", "handlers.FlagResponse"} {
		for name := range spec.Definitions[def].Properties {
			if p07IdentifierPropertyName.MatchString(name) {
				t.Errorf("%s has the identifier looking property %q", def, name)
			}
		}
	}
}
