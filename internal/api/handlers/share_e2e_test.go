// share_e2e_test.go is the link half of V4 for plan 08-03: it proves
// POST /api/reports/{id}/share end to end over the real router and a real
// Postgres. Every server is built through newE2EServerWith with p20Tweak, which
// opens every bucket first (including Share) so a test only ever meets the
// bucket it sets. Every package level identifier carries the p803 prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

const p803Base = "https://pinalert.example"

// p803URLPattern is the one shape of a share link: the configured base, /r/ and
// a 22 character token.
var p803URLPattern = regexp.MustCompile(`^https://pinalert\.example/r/[A-Za-z0-9_-]{22}$`)

// p803Tweak opens every bucket on a frozen clock, roots share links in
// p803Base, then runs mutate.
func p803Tweak(clk *p20Clock, mutate func(d *api.Deps)) func(d *api.Deps, q *sqlcgen.Queries) {
	return p20Tweak(clk, func(d *api.Deps) {
		d.PublicBaseURL = p803Base
		if mutate != nil {
			mutate(d)
		}
	})
}

// p803ShareResponse is the decoded 200 body.
type p803ShareResponse struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Line  string `json:"line"`
}

// p803Share posts the share route for reportID. Extra headers are set as given;
// host, when set, replaces the request's Host header.
func p803Share(t *testing.T, client *http.Client, baseURL string, reportID int64, host string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/reports/"+strconv.FormatInt(reportID, 10)+"/share", nil)
	if err != nil {
		t.Fatalf("building the share request: %v", err)
	}
	if host != "" {
		// net/http looks cookies up under req.Host, so the session cookie of the
		// real address is attached by hand for a forged host.
		req.Host = host
		for _, c := range client.Jar.Cookies(req.URL) {
			req.AddCookie(c)
		}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST share of report %d: %v", reportID, err)
	}
	return resp
}

// p803ShareToken reads the stored share token of a report, "" when NULL.
func p803ShareToken(t *testing.T, pool *pgxpool.Pool, reportID int64) string {
	t.Helper()
	var token *string
	if err := pool.QueryRow(context.Background(), `SELECT share_token FROM reports WHERE id = $1`, reportID).Scan(&token); err != nil {
		t.Fatalf("reading the share token of report %d: %v", reportID, err)
	}
	if token == nil {
		return ""
	}
	return *token
}

// p803Decode reads a 200 share body.
func p803Decode(t *testing.T, label string, resp *http.Response) (p803ShareResponse, []byte) {
	t.Helper()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("%s: Content-Type = %q, want application/json", label, ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("%s: Cache-Control = %q, want no-store", label, cc)
	}
	raw := p20Status(t, label, resp, http.StatusOK)
	var out p803ShareResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: decoding %s: %v", label, raw, err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("%s: decoding keys of %s: %v", label, raw, err)
	}
	if len(keys) != 3 {
		t.Errorf("%s: body %s has %d members, want url, title and line only", label, raw, len(keys))
	}
	return out, raw
}

func TestShareLinkReturnsTokenURL(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p803Tweak(newP20Clock(), nil))
	author := newVerifiedClient(t, srv, mailer, "p803-link-a@example.com")
	reader := newVerifiedClient(t, srv, mailer, "p803-link-b@example.com")
	id := submitReportGetID(t, author, srv.URL, p20Body("flood", "low"))

	first, raw := p803Decode(t, "first share", p803Share(t, reader, srv.URL, id, "", nil))
	if !p803URLPattern.MatchString(first.URL) {
		t.Fatalf("url = %q, want %s", first.URL, p803URLPattern)
	}
	if first.Line != service.ShareFixedLine {
		t.Errorf("line = %q, want %q", first.Line, service.ShareFixedLine)
	}
	if want := service.ShareTitle(service.CategoryFlood, ""); first.Title != want {
		t.Errorf("title = %q, want %q (no area namer is set)", first.Title, want)
	}
	if !strings.HasSuffix(first.URL, p803ShareToken(t, pool, id)) {
		t.Errorf("url %q does not end with the stored token", first.URL)
	}
	// The body holds no digit outside the url: no count, no distance, no time.
	rest := strings.Replace(string(raw), first.URL, "", 1)
	if strings.ContainsAny(rest, "0123456789") {
		t.Errorf("body %s holds a digit outside the url", raw)
	}

	second, _ := p803Decode(t, "second share", p803Share(t, reader, srv.URL, id, "", nil))
	if second.URL != first.URL {
		t.Errorf("a second call answered %q, want the same link %q", second.URL, first.URL)
	}
	// The author may share their own report, and gets the same link.
	own, _ := p803Decode(t, "author share", p803Share(t, author, srv.URL, id, "", nil))
	if own.URL != first.URL {
		t.Errorf("the author's call answered %q, want %q", own.URL, first.URL)
	}

	// Another report gets another token.
	other := submitReportGetID(t, author, srv.URL, p20Body("flood", "low"))
	third, _ := p803Decode(t, "other report", p803Share(t, reader, srv.URL, other, "", nil))
	if third.URL == first.URL {
		t.Errorf("two reports share one link %q", first.URL)
	}

	// A body is ignored, a bad id is a 400, a signed out call is a 401.
	p20WantError(t, "bad id", p20PostFlagRaw(t, reader, srv.URL+"/api/reports/abc/share", `{"x":1}`, ""), http.StatusBadRequest, "id", "Report id must be a number.")
	anon := newJarClient(t)
	loginResp, err := anon.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	loginResp.Body.Close()
	resp := p803Share(t, anon, srv.URL, id, "", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("signed out share: status = %d, want 401", resp.StatusCode)
	}
}

func TestShareLinkIgnoresHostHeader(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p803Tweak(newP20Clock(), nil))
	author := newVerifiedClient(t, srv, mailer, "p803-host-a@example.com")
	reader := newVerifiedClient(t, srv, mailer, "p803-host-b@example.com")
	id := submitReportGetID(t, author, srv.URL, p20Body("flood", "low"))

	got, raw := p803Decode(t, "forged host", p803Share(t, reader, srv.URL, id, "evil.example", map[string]string{
		"X-Forwarded-Host":  "evil.example",
		"X-Forwarded-Proto": "http",
		"Forwarded":         "host=evil.example;proto=http",
		"X-Original-Host":   "evil.example",
	}))
	if !p803URLPattern.MatchString(got.URL) {
		t.Fatalf("url = %q, want %s", got.URL, p803URLPattern)
	}
	if strings.Contains(string(raw), "evil.example") || strings.Contains(string(raw), "localhost") {
		t.Errorf("body %s carries a request supplied host", raw)
	}
	if !strings.HasSuffix(got.URL, p803ShareToken(t, pool, id)) {
		t.Errorf("url %q does not end with the stored token", got.URL)
	}
}

func TestShareLinkWithheldIs404(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p803Tweak(newP20Clock(), nil))
	author := newVerifiedClient(t, srv, mailer, "p803-held-a@example.com")
	reader := newVerifiedClient(t, srv, mailer, "p803-held-b@example.com")

	pending := submitReportGetID(t, author, srv.URL, p20Body("flood", "low"))
	removed := submitReportGetID(t, author, srv.URL, p20Body("flood", "critical"))
	expired := submitReportGetID(t, author, srv.URL, p20Body("flood", "low"))
	p20SetModeration(t, pool, pending, "pending")
	p20SetModeration(t, pool, removed, "removed")
	p20Expire(t, pool, expired)

	var maxID int64
	if err := pool.QueryRow(context.Background(), `SELECT max(id) FROM reports`).Scan(&maxID); err != nil {
		t.Fatalf("reading the highest report id: %v", err)
	}
	missing := maxID + 1000

	wantBody := p20Status(t, "missing report", p803Share(t, reader, srv.URL, missing, "", nil), http.StatusNotFound)
	var parsed struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(wantBody, &parsed); err != nil {
		t.Fatalf("decoding the 404 body %s: %v", wantBody, err)
	}
	if parsed.Error.Field != "report" || parsed.Error.Message != "This report is no longer available." {
		t.Fatalf("404 error = %+v, want field report and the no longer available message", parsed.Error)
	}

	for name, id := range map[string]int64{"pending": pending, "removed": removed} {
		got := p20Status(t, name, p803Share(t, reader, srv.URL, id, "", nil), http.StatusNotFound)
		if string(got) != string(wantBody) {
			t.Errorf("%s: body %s differs from the missing report body %s", name, got, wantBody)
		}
		// The author's own withheld report reads as missing too.
		own := p20Status(t, name+" by the author", p803Share(t, author, srv.URL, id, "", nil), http.StatusNotFound)
		if string(own) != string(wantBody) {
			t.Errorf("%s by the author: body %s differs from %s", name, own, wantBody)
		}
		if token := p803ShareToken(t, pool, id); token != "" {
			t.Errorf("%s: a token %q was written", name, token)
		}
	}

	// A report past its expiry is still inside the history window, so it is a
	// finished report with a link, not a 404.
	p803Decode(t, "expired report", p803Share(t, reader, srv.URL, expired, "", nil))
	if p803ShareToken(t, pool, expired) == "" {
		t.Error("an expired report inside the history window got no token")
	}
}

func TestShareRateLimit(t *testing.T) {
	t.Run("a the default shape", func(t *testing.T) {
		if got, want := api.RateLimitsDefault.Share, (api.BucketLimit{Every: 30 * time.Second, Burst: 10}); got != want {
			t.Fatalf("RateLimitsDefault.Share = %+v, want %+v", got, want)
		}
		if got, want := api.RateLimitsDefault.IPBackstopShare, (api.BucketLimit{Every: 1200 * time.Millisecond, Burst: 1000}); got != want {
			t.Fatalf("RateLimitsDefault.IPBackstopShare = %+v, want %+v", got, want)
		}
	})

	t.Run("b the burst, the exact wait and per account scope", func(t *testing.T) {
		clk := newP20Clock()
		srv, mailer, pool := newE2EServerWith(t, p803Tweak(clk, func(d *api.Deps) {
			d.RateLimits.Share = api.BucketLimit{Every: time.Hour, Burst: 2}
		}))
		author := newVerifiedClient(t, srv, mailer, "p803-rate-a@example.com")
		b := newVerifiedClient(t, srv, mailer, "p803-rate-b@example.com")
		c := newVerifiedClient(t, srv, mailer, "p803-rate-c@example.com")
		var ids []int64
		for i := 0; i < 3; i++ {
			ids = append(ids, submitReportGetID(t, author, srv.URL, p20Body("flood", "low")))
		}

		p803Decode(t, "r1", p803Share(t, b, srv.URL, ids[0], "", nil))
		p803Decode(t, "r2", p803Share(t, b, srv.URL, ids[1], "", nil))
		got := p20Decode429(t, p803Share(t, b, srv.URL, ids[2], "", nil))
		if got.RetryAfterHeader != "3600" {
			t.Errorf("Retry-After = %q, want 3600", got.RetryAfterHeader)
		}
		if got.Field != "rate_share" || got.Message != service.ShareLimitMessage(time.Hour) {
			t.Errorf("429 error = (%q, %q), want rate_share with ShareLimitMessage(1h)", got.Field, got.Message)
		}
		if got.RetryAfterSeconds != 3600 || got.Serious || got.HasNumber {
			t.Errorf("429 body = %+v, want 3600 seconds, serious false, no emergency_number", got)
		}
		if token := p803ShareToken(t, pool, ids[2]); token != "" {
			t.Errorf("a blocked share wrote the token %q", token)
		}

		p803Decode(t, "C is unaffected", p803Share(t, c, srv.URL, ids[2], "", nil))
		clk.Advance(time.Hour)
		p803Decode(t, "B after the wait", p803Share(t, b, srv.URL, ids[2], "", nil))
	})

	t.Run("c a refused report still costs a token", func(t *testing.T) {
		srv, mailer, _ := newE2EServerWith(t, p803Tweak(newP20Clock(), func(d *api.Deps) {
			d.RateLimits.Share = api.BucketLimit{Every: time.Hour, Burst: 1}
		}))
		author := newVerifiedClient(t, srv, mailer, "p803-rate-c-a@example.com")
		b := newVerifiedClient(t, srv, mailer, "p803-rate-c-b@example.com")
		id := submitReportGetID(t, author, srv.URL, p20Body("flood", "low"))

		p20Status(t, "missing report", p803Share(t, b, srv.URL, id+1000, "", nil), http.StatusNotFound)
		if got := p20Decode429(t, p803Share(t, b, srv.URL, id, "", nil)); got.Field != "rate_share" {
			t.Errorf("field = %q, want rate_share", got.Field)
		}
	})

	t.Run("d the address backstop", func(t *testing.T) {
		srv, mailer, _ := newE2EServerWith(t, p803Tweak(newP20Clock(), func(d *api.Deps) {
			d.ClientIPHeader = p20IPHeader
			d.RateLimits.IPBackstopShare = api.BucketLimit{Every: time.Hour, Burst: 1}
		}))
		author := newVerifiedClient(t, srv, mailer, "p803-rate-d-a@example.com")
		b := newVerifiedClient(t, srv, mailer, "p803-rate-d-b@example.com")
		c := newVerifiedClient(t, srv, mailer, "p803-rate-d-c@example.com")
		id := submitReportGetID(t, author, srv.URL, p20Body("flood", "low"))
		ip := func(addr string) map[string]string { return map[string]string{p20IPHeader: addr} }

		p803Decode(t, "B from .7", p803Share(t, b, srv.URL, id, "", ip("203.0.113.7")))
		got := p20Decode429(t, p803Share(t, c, srv.URL, id, "", ip("203.0.113.7")))
		if got.Field != "rate_share" || got.Serious {
			t.Errorf("backstop 429 = %+v, want rate_share and serious false", got)
		}
		if got.Message != service.NetworkLimitMessage(false, time.Hour) {
			t.Errorf("backstop 429 message = %q, want the network message", got.Message)
		}
		p803Decode(t, "C from .8", p803Share(t, c, srv.URL, id, "", ip("203.0.113.8")))
	})
}
