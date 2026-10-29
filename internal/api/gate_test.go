// Package api's own test file (not api_test) so it can drive the real
// NewRouter through an httptest server while still reaching this package's
// unexported requireVerifiedAccount indirectly, the same way router.go
// itself is only reachable as a whole rather than middleware-by-middleware.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api/handlers"
	"pinalert/internal/auth"
	"pinalert/internal/demo"
	"pinalert/internal/mailer"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// gateFakeMailer captures the most recently mailed verification link so
// gate tests can extract the token and drive a real verify without a real
// mail provider — the same recording-mailer shape
// internal/api/handlers/auth_e2e_test.go already established for the
// handlers_test package; kept local here since internal/api cannot import
// that external test package.
type gateFakeMailer struct {
	mu   sync.Mutex
	link string
	// auth holds every auth email by address, oldest first (plan 07.1-03).
	auth map[string][]mailer.AuthEmail
}

// SendAuthEmail records an auth email under the address it was sent to.
func (m *gateFakeMailer) SendAuthEmail(_ context.Context, to string, msg mailer.AuthEmail) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.auth == nil {
		m.auth = map[string][]mailer.AuthEmail{}
	}
	m.auth[to] = append(m.auth[to], msg)
	return nil
}

// codeFor returns the code of the newest auth email sent to email. It fails
// the test when nothing was sent to that address or when the newest email
// carries no code (an account_exists or no_account email).
func (m *gateFakeMailer) codeFor(t *testing.T, email string) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	sent := m.auth[email]
	if len(sent) == 0 {
		t.Fatalf("no auth email was sent to %q", email)
	}
	last := sent[len(sent)-1]
	if last.Code == "" {
		t.Fatalf("the newest auth email to %q is %q and has no code", email, last.Kind)
	}
	return last.Code
}

func (m *gateFakeMailer) SendVerificationLink(_ context.Context, _, link string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.link = link
	return nil
}

func (m *gateFakeMailer) lastLink() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.link
}

// newGateTestServer builds a real NewRouter router against a real,
// migrated Postgres (testutil.NewTestDB) with every dependency the gated
// and exempt routes both need, so the whole route table — not just
// requireVerifiedAccount in isolation — is under test.
func newGateTestServer(t *testing.T) (*httptest.Server, *pgxpool.Pool, *gateFakeMailer) {
	return newGateTestServerWith(t, nil)
}

// gateGenerousBucket and gateGenerousRateLimits open every Phase 4 bucket so
// the existing gate suites never hit a limit. The limit tests set their own
// buckets through the tweak hook of newGateTestServerWith. The request link
// limiter is left at its default on purpose: the spoof test relies on it.
// Amended 2026-10-29 by Phase 7.1 plan 08: that is still true. The sign in
// helper no longer touches the request link limiter, and plan 09 re-aims the
// spoof test at the new send routes.
var gateGenerousBucket = BucketLimit{Every: time.Millisecond, Burst: 1000}

var gateGenerousRateLimits = RateLimits{
	ReportSerious:    gateGenerousBucket,
	ReportOther:      gateGenerousBucket,
	Vote:             gateGenerousBucket,
	VoteChange:       gateGenerousBucket,
	Flag:             gateGenerousBucket,
	SubmitAttempt:    gateGenerousBucket,
	IPBackstopReport: gateGenerousBucket,
	IPBackstopVote:   gateGenerousBucket,
	IPBackstopFlag:   gateGenerousBucket,
}

// newGateTestServerWith is newGateTestServer with a hook. Every bucket starts
// open; tweak, when non nil, runs after the Deps literal and before the router
// is built, so it can override any field.
func newGateTestServerWith(t *testing.T, tweak func(d *Deps, q *sqlcgen.Queries)) (*httptest.Server, *pgxpool.Pool, *gateFakeMailer) {
	t.Helper()
	pool := testutil.NewTestDB(t)

	mgr, err := session.NewManager([]byte("gate-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}

	queries := sqlcgen.New(pool)
	fm := &gateFakeMailer{}
	deps := Deps{
		Session:     mgr,
		Sessions:    queries,
		Reports:     service.NewReportService(queries),
		AuthService: service.NewAuthService(queries, fm, "https://pinalert.example"),
		Auth:        handlers.AuthConfig{AssetVersion: "test"},
		Template:    tmpl,
		RateLimits:  gateGenerousRateLimits,
		// Amended 2026-10-29 by Phase 7.1 plan 08: the code and password
		// service with the test hasher and open buckets, which
		// gateVerifySession needs.
		Credentials: newGateTestCredentials(queries, fm),
		AuthLimits:  gateGenerousAuthLimits,
	}
	if tweak != nil {
		tweak(&deps, queries)
	}
	router := NewRouter(deps)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, pool, fm
}

// newGateJarClient returns a cookie-jar-backed client that never follows a
// redirect automatically — gate tests need to observe the 302 to /login
// itself, not whatever /login eventually returns.
func newGateJarClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// gateTestPassword is the password every gate test account uses. It passes the
// password rule of plan 07.1-02 (length, not common, not the email) and is the
// same value as e2eTestPassword in package handlers_test, which this package
// cannot import.
const gateTestPassword = "wide river morning 47"

// gateGenerousAuthLimits opens every bucket of the code and password routes
// (Phase 7.1, plan 08), for the reason gateGenerousRateLimits exists: the gate
// suites must never meet a limit by accident. The limit tests set their own
// buckets through the tweak hook of newGateTestServerWith.
var gateGenerousAuthLimits = AuthLimits{
	SignInDevice:     gateGenerousBucket,
	SignInIP:         gateGenerousBucket,
	CodeSendDevice:   gateGenerousBucket,
	CodeSendIP:       gateGenerousBucket,
	CodeSendNoCookie: gateGenerousBucket,
	CodeTryDevice:    gateGenerousBucket,
	UsernameCheck:    gateGenerousBucket,
	UsernameSave:     gateGenerousBucket,
}

// newGateTestCredentials builds the CredentialService the gate router uses. It
// is the only place in this package that passes the fast test hasher;
// production never does. The keys are fixed 32 byte test values and
// SendCooldown is one millisecond, so a sign up followed by a reset on the same
// address in one test is not refused by the 45 second per address cooldown.
func newGateTestCredentials(q *sqlcgen.Queries, m *gateFakeMailer) *service.CredentialService {
	limits := service.DefaultCredentialLimits
	limits.SendCooldown = time.Millisecond
	return service.NewCredentialService(q, m,
		service.WithHasher(auth.NewHasher(auth.TestParams())),
		service.WithCodeKey(bytes.Repeat([]byte("c"), 32)),
		service.WithThrottleKey(bytes.Repeat([]byte("t"), 32)),
		service.WithCredentialLimits(limits),
	)
}

// gatePostJSON posts body as JSON to path on srv with client and returns the
// status and the body bytes.
func gatePostJSON(t *testing.T, client *http.Client, srv *httptest.Server, path string, body any) (int, []byte) {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling request body for %s: %v", path, err)
	}
	resp, err := client.Post(srv.URL+path, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the answer of %s: %v", path, err)
	}
	return resp.StatusCode, raw
}

// gateVerifySession signs client in as email against srv, so the browser it
// represents ends up bound to a verified account.
//
// Amended 2026-10-29 by Phase 7.1 plan 08: it used to request a login link and
// open it. It now tries POST /api/auth/sign-in with gateTestPassword first (a
// 200 means the account already has that password, so a second browser for the
// same address needs no second code email). A 400 on the field credentials
// means no such account yet, and it then creates the account with
// POST /api/auth/sign-up and the code the recording mailer captured. Any other
// answer fails the test. Step 4 of onboarding (the profile) is not filled here.
func gateVerifySession(t *testing.T, client *http.Client, srv *httptest.Server, mailer *gateFakeMailer, email string) {
	t.Helper()

	status, raw := gatePostJSON(t, client, srv, "/api/auth/sign-in", map[string]string{"email": email, "password": gateTestPassword})
	if status != http.StatusOK {
		var failure handlers.ErrorResponse
		if status != http.StatusBadRequest || json.Unmarshal(raw, &failure) != nil || failure.Error.Field != "credentials" {
			t.Fatalf("POST /api/auth/sign-in for %s: status=%d body=%s", email, status, raw)
		}
		if got, body := gatePostJSON(t, client, srv, "/api/auth/sign-up", map[string]string{"email": email, "password": gateTestPassword}); got != http.StatusOK {
			t.Fatalf("POST /api/auth/sign-up for %s: status=%d body=%s", email, got, body)
		}
		status, raw = gatePostJSON(t, client, srv, "/api/auth/sign-up/code", map[string]string{"code": mailer.codeFor(t, email)})
	}
	if status != http.StatusOK || !strings.Contains(string(raw), `"next"`) {
		t.Fatalf("signing in %s: status=%d body=%s", email, status, raw)
	}
}

// gateSessionIDFromJar extracts the unsigned session id portion of the
// pinalert_session cookie the jar currently holds for srvURL.
func gateSessionIDFromJar(t *testing.T, client *http.Client, srvURL string) string {
	t.Helper()
	parsed, err := url.Parse(srvURL)
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	for _, c := range client.Jar.Cookies(parsed) {
		if c.Name == "pinalert_session" {
			id := strings.SplitN(c.Value, ".", 2)[0]
			if id == "" {
				t.Fatalf("session cookie %q has no id portion before the signature", c.Value)
			}
			return id
		}
	}
	t.Fatalf("no pinalert_session cookie found in jar for %s", srvURL)
	return ""
}

// TestAccessGateBlocksUnverified is this plan's core claim: an unverified
// session gets no app-shell markup from GET / (a redirect, not a partial
// render) and no report data from either method on /api/reports (a 401
// with the standard error envelope, and no row written on POST).
func TestAccessGateBlocksUnverified(t *testing.T) {
	srv, pool, _ := newGateTestServer(t)
	client := newGateJarClient(t)

	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("GET / status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("GET / Location = %q, want %q", loc, "/login")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading GET / body: %v", err)
	}
	if strings.Contains(string(body), `id="app-shell"`) {
		t.Fatalf("unverified GET / response body contains the app-shell element: %s", body)
	}

	getResp, err := client.Get(srv.URL + "/api/reports?lat=12.9716&lon=77.5946")
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(getResp.Body)
		t.Fatalf("GET /api/reports status = %d, want 401: %s", getResp.StatusCode, b)
	}
	var errBody struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&errBody); err != nil {
		t.Fatalf("decoding GET /api/reports error body: %v", err)
	}
	if errBody.Error.Field != "auth" {
		t.Fatalf("GET /api/reports error field = %q, want %q", errBody.Error.Field, "auth")
	}

	// Amended 2026-10-29 by plan 05-08: a new row. The official alerts route
	// sits behind the same gate, and an unverified caller gets the 401
	// envelope, never a list that could be read as "no alerts".
	officialResp, err := client.Get(srv.URL + "/api/official/alerts?lat=16.5062&lon=80.6480")
	if err != nil {
		t.Fatalf("GET /api/official/alerts: %v", err)
	}
	defer officialResp.Body.Close()
	officialBody, err := io.ReadAll(officialResp.Body)
	if err != nil {
		t.Fatalf("reading GET /api/official/alerts body: %v", err)
	}
	if officialResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/official/alerts status = %d, want 401: %s", officialResp.StatusCode, officialBody)
	}
	var officialErr struct {
		Error struct {
			Field string `json:"field"`
		} `json:"error"`
	}
	if err := json.Unmarshal(officialBody, &officialErr); err != nil || officialErr.Error.Field != "auth" {
		t.Fatalf("GET /api/official/alerts error body = %s (%v), want the auth envelope", officialBody, err)
	}
	if strings.Contains(string(officialBody), "groups") {
		t.Fatalf("an unverified GET /api/official/alerts body must carry no alert data: %s", officialBody)
	}

	postResp, err := client.Post(srv.URL+"/api/reports", "application/json", bytes.NewReader([]byte(`{
		"category": "flood",
		"severity": "low",
		"description": "unverified post attempt should be refused before any row is written",
		"latitude": 12.9716,
		"longitude": 77.5946
	}`)))
	if err != nil {
		t.Fatalf("POST /api/reports: %v", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(postResp.Body)
		t.Fatalf("POST /api/reports status = %d, want 401: %s", postResp.StatusCode, b)
	}

	var reportCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM reports`).Scan(&reportCount); err != nil {
		t.Fatalf("counting reports: %v", err)
	}
	if reportCount != 0 {
		t.Fatalf("expected zero reports rows after an unverified POST, got %d", reportCount)
	}
}

// TestAccessGateAllowsVerified proves the gate does not regress Phase 1's
// walking skeleton: a session verified through the real magic-link flow
// gets the app shell from GET / and report data from GET /api/reports,
// exactly as it did before this plan.
func TestAccessGateAllowsVerified(t *testing.T) {
	srv, _, mailer := newGateTestServer(t)
	client := newGateJarClient(t)

	gateVerifySession(t, client, srv, mailer, "verified-gate@example.com")

	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading GET / body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200: %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `id="app-shell"`) {
		t.Fatalf("verified GET / response is missing the app-shell element: %s", body)
	}

	getResp, err := client.Get(srv.URL + "/api/reports?lat=12.9716&lon=77.5946")
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(getResp.Body)
		t.Fatalf("GET /api/reports status = %d, want 200: %s", getResp.StatusCode, b)
	}
}

// TestAccessGateExemptRoutes is a table-driven check over DEC-G's exempt
// list: an unverified session must reach every one of these seventeen routes
// and get neither a redirect nor a 401. The first seven are the login surface,
// the static files, the API reference and the two legal pages; the last ten
// are the installable app assets.
//
// Amended 2026-10-18 by Phase 3.3 plan 04 (DEC-G, Phase 1.1 D-05, decision 13):
// ten rows were added for the manifest, the service worker and the icons
// (nothing sensitive, fetched by the browser before sign in), each of which
// must answer 200, and a second test below proves the near misses stay gated.
// These rows run only on a database: newGateTestServer skips without
// DATABASE_URL.
//
// Amended 2026-10-28 by plan 05-04 (D-13): one row "globe tile" was added for
// GET /globe/tiles/0/0/0.jpg (the sign in globe's Earth pictures, public like the
// icons and answered from a fake upstream here), which must answer 200, and the
// near misses /globe/tiles/9/0/0.jpg and /globe/tiles must not.
//
// Amended 2026-10-29 by plan 05-10 (D-04, owner 2026-10-17): the Demo opens
// without an account. One row was added for GET /demo/api/v/<visit>/reports,
// answered 200 from a Demo store created in the test, and a negative loop at the
// end proves /api/reports, /alerts and /api/official/alerts stay gated for a
// caller with no session (401, 302 and 401). Every older row is unchanged.
//
// Amended 2026-10-29 by plan 05-12 (D-15): the external cron's door joins the
// exempt list. With a token configured and a fake refresher, POST
// /internal/official/refresh with the bearer token must answer 200 for a caller
// with no session (no redirect to /login, and the 401 the endpoint gives a wrong
// token is its own, checked separately), a GET of it is a 404, and on a server
// with no token it is a 404. Every older row and loop is unchanged.
func TestAccessGateExemptRoutes(t *testing.T) {
	globeUp := fakeGlobeUpstream(t)
	demoStore := demo.NewStore(demo.WithWorldSetup(demo.SeedSetup))
	demoWorld, err := demoStore.Create()
	if err != nil {
		t.Fatalf("creating a Demo world: %v", err)
	}
	demoFeedPath := "/demo/api/v/" + demoWorld.ID + "/reports?lat=16.5424&lon=80.6386&radius_km=10"
	srv, _, _ := newGateTestServerWith(t, func(d *Deps, _ *sqlcgen.Queries) {
		d.GlobeUpstream = globeUp.URL
		d.Demo = demoStore
		// Amended 2026-10-29 by plan 05-12: the refresh door for the rows below.
		d.Official = &refreshStub{}
		d.OfficialRefreshToken = routeRefreshToken
	})

	appAssets := []string{
		"/manifest.webmanifest",
		"/sw.js",
		"/icons/app.svg",
		"/icons/app-192.png",
		"/icons/app-512.png",
		"/icons/app-maskable-512.png",
		"/icons/apple-touch-icon.png",
		"/icons/badge-96.png",
		"/apple-touch-icon.png",
		"/apple-touch-icon-precomposed.png",
	}
	mustBeOK := map[string]bool{"/disclaimer": true, "/policy": true, "/globe/tiles/0/0/0.jpg": true, demoFeedPath: true}
	for _, p := range appAssets {
		mustBeOK[p] = true
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"login gate", http.MethodGet, "/login", ""},
		{"request link", http.MethodPost, "/api/auth/request-link", `{"email":"exempt-route@example.com"}`},
		{"verify landing", http.MethodGet, "/auth/verify", ""},
		{"static asset", http.MethodGet, "/static/css/main.css", ""},
		{"swagger ui", http.MethodGet, "/swagger/index.html", ""},
		{"disclaimer page", http.MethodGet, "/disclaimer", ""},
		{"policy page", http.MethodGet, "/policy", ""},
		{"globe tile", http.MethodGet, "/globe/tiles/0/0/0.jpg", ""},
		{"demo feed", http.MethodGet, demoFeedPath, ""},
	}
	for _, p := range appAssets {
		cases = append(cases, struct {
			name   string
			method string
			path   string
			body   string
		}{"app asset " + p, http.MethodGet, p, ""})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newGateJarClient(t)

			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}
			req, err := http.NewRequest(tc.method, srv.URL+tc.path, bodyReader)
			if err != nil {
				t.Fatalf("building request: %v", err)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", tc.method, tc.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 300 && resp.StatusCode < 400 {
				t.Fatalf("%s %s returned a redirect (%d) — this route must be reachable without verification", tc.method, tc.path, resp.StatusCode)
			}
			if resp.StatusCode == http.StatusUnauthorized {
				t.Fatalf("%s %s returned 401 — this route must be reachable without verification", tc.method, tc.path)
			}
			if mustBeOK[tc.path] && resp.StatusCode != http.StatusOK {
				t.Fatalf("%s %s returned %d: this public page or app asset must answer 200 without verification", tc.method, tc.path, resp.StatusCode)
			}
		})
	}

	// Near misses of the app asset paths are not exempt: no prefix or suffix
	// match opens a hole in the gate. They are not routes at all, so the answer
	// is never a 200 from the asset handlers.
	for _, p := range []string{"/icons/other.png", "/sw.js.map", "/sw.js/", "/icons/"} {
		t.Run("near miss "+p, func(t *testing.T) {
			resp, err := newGateJarClient(t).Get(srv.URL + p)
			if err != nil {
				t.Fatalf("GET %s: %v", p, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Fatalf("GET %s returned 200: only the ten listed app asset paths are public", p)
			}
		})
	}

	// Amended 2026-10-28 by plan 05-04 (D-13): near misses of the globe tiles.
	// Only the 85 listed tile paths are public.
	for _, p := range []string{"/globe/tiles/9/0/0.jpg", "/globe/tiles"} {
		t.Run("globe near miss "+p, func(t *testing.T) {
			resp, err := newGateJarClient(t).Get(srv.URL + p)
			if err != nil {
				t.Fatalf("GET %s: %v", p, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Fatalf("GET %s returned 200: only the 85 listed globe tile paths are public", p)
			}
		})
	}

	// Amended 2026-10-29 by plan 05-10 (D-04): opening /demo opens nothing else.
	// The real report feed, the Official alerts page and its data stay behind the
	// gate for a caller with no session: 401 for the two API reads, 302 to /login
	// for the page. An exact status, not "not 200", so a 404 or a 503 hiding a
	// bypass would fail.
	for _, g := range []struct {
		path string
		want int
	}{
		{"/api/reports?lat=16.5424&lon=80.6386", http.StatusUnauthorized},
		{"/api/official/alerts?lat=16.5424&lon=80.6386", http.StatusUnauthorized},
		{"/alerts", http.StatusFound},
	} {
		t.Run("still gated "+g.path, func(t *testing.T) {
			resp, err := newGateJarClient(t).Get(srv.URL + g.path)
			if err != nil {
				t.Fatalf("GET %s: %v", g.path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != g.want {
				t.Fatalf("GET %s = %d, want %d: it must stay behind the gate", g.path, resp.StatusCode, g.want)
			}
		})
	}

	// Amended 2026-10-29 by plan 05-12 (D-15): the cron's door is outside the gate
	// and guarded by its own token. A cookieless POST with the token is a 200, not
	// a redirect or the gate's 401; without the token it is the endpoint's own 401;
	// a GET is a 404; and a server with no token configured answers 404.
	refresh := func(t *testing.T, base, method, auth string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, base+"/internal/official/refresh", nil)
		if err != nil {
			t.Fatalf("building request: %v", err)
		}
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := newGateJarClient(t).Do(req)
		if err != nil {
			t.Fatalf("%s /internal/official/refresh: %v", method, err)
		}
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}
	t.Run("official refresh with the token", func(t *testing.T) {
		if resp := refresh(t, srv.URL, http.MethodPost, "Bearer "+routeRefreshToken); resp.StatusCode != http.StatusOK {
			t.Fatalf("POST with the token = %d, want 200 without a session", resp.StatusCode)
		}
	})
	t.Run("official refresh without the token", func(t *testing.T) {
		if resp := refresh(t, srv.URL, http.MethodPost, ""); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("POST with no token = %d, want the endpoint's own 401", resp.StatusCode)
		}
	})
	t.Run("official refresh by GET", func(t *testing.T) {
		if resp := refresh(t, srv.URL, http.MethodGet, "Bearer "+routeRefreshToken); resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET = %d, want 404", resp.StatusCode)
		}
	})
	t.Run("official refresh with no token configured", func(t *testing.T) {
		off, _, _ := newGateTestServer(t)
		if resp := refresh(t, off.URL, http.MethodPost, "Bearer "+routeRefreshToken); resp.StatusCode != http.StatusNotFound {
			t.Fatalf("POST with no token configured = %d, want 404", resp.StatusCode)
		}
	})
}

// TestAccessGateBlocksUnverifiedGeocode is this plan's V4 Access Control
// proof: an unverified session gets 401 with the standard {error:{field}}
// envelope from GET /api/geocode, and the raw body never contains
// "results", so a refusal can never be mistaken for an empty result set.
// This test skips without DATABASE_URL (via testutil.NewTestDB), so the
// phase gate is `make test`, not a short-mode run.
func TestAccessGateBlocksUnverifiedGeocode(t *testing.T) {
	srv, _, _ := newGateTestServer(t)
	client := newGateJarClient(t)

	resp, err := client.Get(srv.URL + "/api/geocode?q=bengaluru")
	if err != nil {
		t.Fatalf("GET /api/geocode: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading GET /api/geocode body: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/geocode status = %d, want 401: %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "results") {
		t.Fatalf("GET /api/geocode unverified response body must not contain \"results\": %s", body)
	}

	var errBody struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errBody); err != nil {
		t.Fatalf("decoding GET /api/geocode error body: %v", err)
	}
	if errBody.Error.Field != "auth" {
		t.Fatalf("GET /api/geocode error field = %q, want %q", errBody.Error.Field, "auth")
	}
}

// TestAccessGateTreatsMissingSessionRowAsUnverified proves RESEARCH.md
// Pattern 2 end to end through the real gate: a browser whose sessions row
// is deleted out from under it is redirected to /login exactly like a
// never-verified visitor, never a 500.
func TestAccessGateTreatsMissingSessionRowAsUnverified(t *testing.T) {
	srv, pool, mailer := newGateTestServer(t)
	client := newGateJarClient(t)

	gateVerifySession(t, client, srv, mailer, "missing-row@example.com")
	sessionID := gateSessionIDFromJar(t, client, srv.URL)

	if _, err := pool.Exec(context.Background(), `DELETE FROM sessions WHERE session_id = $1`, sessionID); err != nil {
		t.Fatalf("deleting sessions row to simulate the persist-failure path: %v", err)
	}
	var rowCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE session_id = $1`, sessionID).Scan(&rowCount); err != nil {
		t.Fatalf("counting sessions rows after delete: %v", err)
	}
	if rowCount != 0 {
		t.Fatalf("expected zero sessions rows after delete, got %d", rowCount)
	}

	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET / status = %d, want %d (redirect, not 500) after the sessions row was deleted: %s", resp.StatusCode, http.StatusFound, b)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("GET / Location = %q, want %q", loc, "/login")
	}
}
