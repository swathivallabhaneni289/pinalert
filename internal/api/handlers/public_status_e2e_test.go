// public_status_e2e_test.go proves the public status page GET /r/{token} end to
// end over the real router and a real Postgres (Phase 8 plan 04; D-05, D-06,
// SC5). Every package level identifier carries the p804 prefix. Reports are
// written straight into the database, with the share token set, because the
// page reads only a token and the tests are about what it shows.
package handlers_test

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	p804Base = "https://pinalert.example"
	// p804Secret is the reporter's description. It must never reach the page.
	p804Secret = "p804 private words about the boat behind the school"
)

var (
	p804Seq     int
	p804MetaRE  = regexp.MustCompile(`<meta property="(og:[a-z]+)" content="([^"]*)">`)
	p804H1RE    = regexp.MustCompile(`<h1 class="public-title">([^<]*)</h1>`)
	p804RobotRE = regexp.MustCompile(`<meta name="robots" content="noindex, nofollow">`)
)

// p804Server builds the real router on a fresh database with the configured
// base, then runs mutate on the Deps.
func p804Server(t *testing.T, mutate func(d *api.Deps)) (string, *pgxpool.Pool) {
	t.Helper()
	srv, _, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.PublicBaseURL = p804Base
		if mutate != nil {
			mutate(d)
		}
	})
	return srv.URL, pool
}

// p804Report is one stored report with its share token.
type p804Report struct {
	id       int64
	token    string
	reporter int64
	session  string
}

// p804Make stores one report of a fresh account and gives it a share token.
func p804Make(t *testing.T, pool *pgxpool.Pool, severity, category, moderation string, createdAgo, expiresIn time.Duration) p804Report {
	t.Helper()
	p804Seq++
	session := fmt.Sprintf("p804-session-%d", p804Seq)
	reporter := testutil.SeedReporterAccount(t, pool, fmt.Sprintf("p804-reporter-%d@example.com", p804Seq), session)
	now := time.Now()
	id := testutil.InsertStoredReport(t, pool, testutil.StoredReport{
		SessionID:       session,
		Category:        category,
		Severity:        severity,
		Description:     p804Secret,
		Latitude:        16.5062,
		Longitude:       80.6480,
		CreatedAt:       now.Add(-createdAgo),
		ExpiresAt:       now.Add(expiresIn),
		ModerationState: moderation,
	})
	token, err := service.NewShareToken()
	if err != nil {
		t.Fatalf("making a share token: %v", err)
	}
	testutil.MustExec(t, pool, `UPDATE reports SET share_token = $1 WHERE id = $2`, token, id)
	return p804Report{id: id, token: token, reporter: reporter, session: session}
}

func p804Voter(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	p804Seq++
	return testutil.SeedReporterAccount(t, pool, fmt.Sprintf("p804-voter-%d@example.com", p804Seq))
}

func p804Vote(t *testing.T, pool *pgxpool.Pool, reportID, account int64, kind, value, cell string, ago time.Duration) {
	t.Helper()
	testutil.MustExec(t, pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		reportID, account, kind, value, cell, time.Now().Add(-ago))
}

// p804Page is one answer of the page.
type p804Page struct {
	status int
	header http.Header
	body   string
}

// p804Get opens the page with a client that has no cookie jar, like a crawler.
// host, when set, replaces the request's Host header; headers are set as given.
func p804Get(t *testing.T, baseURL, path, host string, headers map[string]string) p804Page {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		t.Fatalf("building GET %s: %v", path, err)
	}
	if host != "" {
		req.Host = host
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading GET %s: %v", path, err)
	}
	return p804Page{status: resp.StatusCode, header: resp.Header, body: string(raw)}
}

// p804Meta reads the og: tags of a page, unescaped.
func p804Meta(body string) map[string]string {
	out := map[string]string{}
	for _, m := range p804MetaRE.FindAllStringSubmatch(body, -1) {
		out[m[1]] = html.UnescapeString(m[2])
	}
	return out
}

// p804H1 reads the text of the h1, unescaped.
func p804H1(t *testing.T, body string) string {
	t.Helper()
	m := p804H1RE.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("the page has no h1: %s", body)
	}
	return html.UnescapeString(m[1])
}

// p804CheckHeaders asserts what every answer of the page carries.
func p804CheckHeaders(t *testing.T, label string, p p804Page) {
	t.Helper()
	want := map[string]string{
		"Content-Type":    "text/html; charset=utf-8",
		"Cache-Control":   "no-store",
		"X-Robots-Tag":    "noindex, nofollow",
		"Referrer-Policy": "no-referrer",
	}
	for k, v := range want {
		if got := p.header.Get(k); got != v {
			t.Errorf("%s: %s = %q, want %q", label, k, got, v)
		}
	}
	if got := p.header.Values("Set-Cookie"); len(got) != 0 {
		t.Errorf("%s: the page set a cookie: %v", label, got)
	}
	if !p804RobotRE.MatchString(p.body) {
		t.Errorf("%s: the page has no noindex robots meta", label)
	}
}

func TestSharePageShowsOnlyAllowedFields(t *testing.T) {
	base, pool := p804Server(t, nil)
	rep := p804Make(t, pool, "medium", "flood", "clear", 12*time.Minute, 2*time.Hour)
	testutil.MustExec(t, pool, `UPDATE reports SET area_name = 'Benz Circle' WHERE id = $1`, rep.id)
	testutil.MustExec(t, pool, `UPDATE accounts SET username = 'p804reporter', username_screen = 'clear' WHERE id = $1`, rep.reporter)
	// The reporter's email, to prove it never reaches the page.
	var email string
	if err := pool.QueryRow(context.Background(), `SELECT email FROM accounts WHERE id = $1`, rep.reporter).Scan(&email); err != nil {
		t.Fatalf("reading the reporter email: %v", err)
	}
	p804Vote(t, pool, rep.id, p804Voter(t, pool), "content", "confirm", "tdr1v8y", 5*time.Minute)
	p804Vote(t, pool, rep.id, p804Voter(t, pool), "content", "confirm", "tdr1vc3", 4*time.Minute)

	page := p804Get(t, base, "/r/"+rep.token, "", nil)
	if page.status != http.StatusOK {
		t.Fatalf("GET /r/<token> = %d: %s", page.status, page.body)
	}
	p804CheckHeaders(t, "allowed fields", page)

	for _, want := range []string{
		"Flood report near Benz Circle",
		"Active",
		"Confirmed by 2 people at different spots.",
		"Medium",
		"12 minutes ago",
		"Sign in to see more",
		"Disclaimer",
		"Policy",
	} {
		if !strings.Contains(page.body, want) {
			t.Errorf("the page does not show %q", want)
		}
	}
	lower := strings.ToLower(page.body)
	for _, banned := range []string{
		p804Secret, "boat behind the school", "16.5062", "80.6480", "16.50", "80.64",
		"p804reporter", strings.ToLower(email), "verified", "<script", "still happening",
		"comment", "km away", "latitude", "longitude",
	} {
		if strings.Contains(lower, strings.ToLower(banned)) {
			t.Errorf("the page shows %q, which it must never show", banned)
		}
	}
}

func TestSharePageStates(t *testing.T) {
	base, pool := p804Server(t, nil)

	type state struct {
		name     string
		token    string
		status   int
		h1       string
		contains []string
		absent   []string
	}
	var states []state

	// Active, confirmed.
	confirmed := p804Make(t, pool, "medium", "flood", "clear", 12*time.Minute, 2*time.Hour)
	p804Vote(t, pool, confirmed.id, p804Voter(t, pool), "content", "confirm", "tdr1v8y", 5*time.Minute)
	states = append(states, state{"active confirmed", confirmed.token, 200, "Flood report",
		[]string{"Active", "1 person confirmed this.", "Severity", "Posted"}, []string{"Not confirmed yet."}})

	// Active, not confirmed.
	unconfirmed := p804Make(t, pool, "low", "power_outage", "clear", 3*time.Minute, 2*time.Hour)
	states = append(states, state{"active unconfirmed", unconfirmed.token, 200, "Power outage report",
		[]string{"Active", "Not confirmed yet.", "3 minutes ago"}, nil})

	// Critical with a pick.
	critical := p804Make(t, pool, "critical", "flood", "clear", 3*time.Minute, 2*time.Hour)
	testutil.MustExec(t, pool, `UPDATE reports SET happening = 'water_rising' WHERE id = $1`, critical.id)
	states = append(states, state{"critical with a pick", critical.token, 200, "Flood report",
		[]string{"Critical", "What is happening", "Water rising"}, nil})

	// Disputed.
	disputed := p804Make(t, pool, "medium", "road_blocked", "clear", 20*time.Minute, 2*time.Hour)
	p804Vote(t, pool, disputed.id, p804Voter(t, pool), "content", "dispute", "tdr1v8y", 5*time.Minute)
	p804Vote(t, pool, disputed.id, p804Voter(t, pool), "content", "dispute", "tdr1vc3", 4*time.Minute)
	states = append(states, state{"disputed", disputed.token, 200, "Road blocked report",
		[]string{"Disputed", "2 people at different spots disputed this."}, nil})

	// Resolved.
	resolved := p804Make(t, pool, "medium", "flood", "clear", 90*time.Minute, 2*time.Hour)
	p804Vote(t, pool, resolved.id, resolved.reporter, "resolution", "resolve", "tdr1v8y", 40*time.Minute)
	states = append(states, state{"resolved", resolved.token, 200, "Flood report",
		[]string{"Resolved", "Resolved by the poster.", "40 minutes ago"}, nil})

	// Ended.
	ended := p804Make(t, pool, "medium", "flood", "clear", 8*time.Hour, -2*time.Hour)
	states = append(states, state{"ended", ended.token, 200, "Flood report",
		[]string{"Ended", service.PublicEndedSentence}, nil})

	// Removed by a moderator.
	removed := p804Make(t, pool, "medium", "flood", "removed", 20*time.Minute, 2*time.Hour)
	states = append(states, state{"removed", removed.token, 410, service.PublicRemovedTitle,
		[]string{"This report was removed", "Sign in to see more"}, []string{"Severity", "Posted"}})

	// Held, too old, unknown and malformed all read the same unavailable page.
	held := p804Make(t, pool, "low", "flood", "pending", 5*time.Minute, time.Hour)
	tooOld := p804Make(t, pool, "low", "flood", "clear", 10*24*time.Hour, -9*24*time.Hour)
	for name, token := range map[string]string{
		"held":      held.token,
		"too old":   tooOld.token,
		"unknown":   strings.Repeat("Z", service.ShareTokenLen),
		"malformed": "not-a-token",
		"numeric":   "1",
	} {
		states = append(states, state{name, token, 404, service.PublicUnavailableTitle,
			[]string{"This report is no longer available", "Sign in to see more"}, []string{"Severity", "Posted", "This report was removed"}})
	}

	for _, s := range states {
		t.Run(s.name, func(t *testing.T) {
			page := p804Get(t, base, "/r/"+s.token, "", nil)
			if page.status != s.status {
				t.Fatalf("status = %d, want %d: %s", page.status, s.status, page.body)
			}
			p804CheckHeaders(t, s.name, page)
			if h1 := p804H1(t, page.body); !strings.HasPrefix(h1, s.h1) {
				t.Errorf("h1 = %q, want it to start with %q", h1, s.h1)
			}
			for _, want := range s.contains {
				if !strings.Contains(page.body, want) {
					t.Errorf("the page does not show %q", want)
				}
			}
			for _, banned := range s.absent {
				if strings.Contains(page.body, banned) {
					t.Errorf("the page shows %q", banned)
				}
			}
			if strings.Contains(page.body, p804Secret) {
				t.Errorf("the page shows the reporter's words")
			}
		})
	}
}

func TestShareOpenGraph(t *testing.T) {
	base, pool := p804Server(t, nil)
	rep := p804Make(t, pool, "medium", "flood", "clear", 12*time.Minute, 2*time.Hour)
	testutil.MustExec(t, pool, `UPDATE reports SET area_name = 'Benz Circle' WHERE id = $1`, rep.id)
	p804Vote(t, pool, rep.id, p804Voter(t, pool), "content", "confirm", "tdr1v8y", 5*time.Minute)

	// The Host header is forged: no URL on the page may follow it.
	page := p804Get(t, base, "/r/"+rep.token, "evil.example", nil)
	if page.status != http.StatusOK {
		t.Fatalf("GET /r/<token> = %d: %s", page.status, page.body)
	}
	meta := p804Meta(page.body)
	h1 := p804H1(t, page.body)
	if meta["og:title"] != h1 {
		t.Errorf("og:title = %q, want the h1 text %q", meta["og:title"], h1)
	}
	if meta["og:title"] != "Flood report near Benz Circle" {
		t.Errorf("og:title = %q, want %q", meta["og:title"], "Flood report near Benz Circle")
	}
	if meta["og:description"] != service.ShareFixedLine {
		t.Errorf("og:description = %q, want %q", meta["og:description"], service.ShareFixedLine)
	}
	if want := p804Base + "/r/" + rep.token; meta["og:url"] != want {
		t.Errorf("og:url = %q, want %q", meta["og:url"], want)
	}
	if !strings.HasPrefix(meta["og:url"], "https://pinalert.example/r/") {
		t.Errorf("og:url = %q does not start with the configured base", meta["og:url"])
	}
	if meta["og:image"] != "https://pinalert.example/icons/app-512.png" {
		t.Errorf("og:image = %q, want the app mark under the configured base", meta["og:image"])
	}
	if meta["og:type"] != "website" {
		t.Errorf("og:type = %q, want website", meta["og:type"])
	}
	if strings.Contains(page.body, "evil.example") {
		t.Errorf("the page follows the forged Host header")
	}
	// No count and no claim in the preview.
	for _, name := range []string{"og:title", "og:description"} {
		if strings.ContainsAny(meta[name], "0123456789") {
			t.Errorf("%s = %q holds a digit", name, meta[name])
		}
	}
	// The preview image is the app mark and stays under WhatsApp's 600 KB.
	img := p804Get(t, base, "/icons/app-512.png", "", nil)
	if img.status != http.StatusOK {
		t.Fatalf("GET /icons/app-512.png = %d", img.status)
	}
	if len(img.body) >= 600*1024 {
		t.Errorf("the preview image is %d bytes, want under 600 KB", len(img.body))
	}

	// An unavailable link points its og:url at the base, never at the bad path.
	gone := p804Get(t, base, "/r/not-a-token", "", nil)
	if got := p804Meta(gone.body)["og:url"]; got != p804Base+"/" {
		t.Errorf("unavailable og:url = %q, want %q", got, p804Base+"/")
	}
}

func TestSharePageRateLimited(t *testing.T) {
	const ipHeader = "X-Test-Client-IP"
	base, pool := p804Server(t, func(d *api.Deps) {
		d.ClientIPHeader = ipHeader
		d.SharePageRateLimit = api.SharePageRateLimit{Every: time.Hour, Burst: 1}
	})
	rep := p804Make(t, pool, "low", "flood", "clear", 5*time.Minute, time.Hour)
	from := func(ip string) map[string]string { return map[string]string{ipHeader: ip} }

	first := p804Get(t, base, "/r/"+rep.token, "", from("203.0.113.7"))
	if first.status != http.StatusOK {
		t.Fatalf("first GET = %d, want 200: %s", first.status, first.body)
	}
	second := p804Get(t, base, "/r/"+rep.token, "", from("203.0.113.7"))
	if second.status != http.StatusTooManyRequests {
		t.Fatalf("second GET = %d, want 429", second.status)
	}
	if got := second.header.Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want 1", got)
	}
	p804CheckHeaders(t, "rate limited", second)
	for _, want := range []string{service.PublicTooManyTitle, service.PublicTooManyLine, "Sign in to see more"} {
		if !strings.Contains(second.body, want) {
			t.Errorf("the refusal page does not show %q", want)
		}
	}
	for _, banned := range []string{"Severity", "Posted", "Flood report"} {
		if strings.Contains(second.body, banned) {
			t.Errorf("the refusal page shows %q", banned)
		}
	}
	// Another address has its own budget.
	other := p804Get(t, base, "/r/"+rep.token, "", from("198.51.100.9"))
	if other.status != http.StatusOK {
		t.Errorf("a different address got %d, want 200", other.status)
	}
}

func TestSharePageNoLimitWithoutClientIPHeader(t *testing.T) {
	base, pool := p804Server(t, func(d *api.Deps) {
		// A burst of 1 would refuse the second request if the limiter existed.
		d.SharePageRateLimit = api.SharePageRateLimit{Every: time.Hour, Burst: 1}
	})
	rep := p804Make(t, pool, "low", "flood", "clear", 5*time.Minute, time.Hour)
	for i := 0; i < 10; i++ {
		page := p804Get(t, base, "/r/"+rep.token, "", nil)
		if page.status != http.StatusOK {
			t.Fatalf("GET %d = %d, want 200: with no CLIENT_IP_HEADER every request is admitted", i+1, page.status)
		}
	}
}
