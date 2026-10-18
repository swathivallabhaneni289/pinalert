// Package handlers_test covers Phase 3.2 plan 06 end to end: a revoked session
// or a deleted account is refused by the one gate query, ending sessions never
// moves authorship, and a revoked browser can never carry one account's
// history into another account's sign in. The route that revokes arrives in
// plan 16, so these tests revoke directly in SQL with the same statement
// RevokeAccountSessions runs.
package handlers_test

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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	revLat = 12.9716
	revLon = 77.5946
)

// newRevServer is the shared harness: a moderator list holding mod@example.com
// and an open per IP request-link bucket (every test client is 127.0.0.1).
func newRevServer(t *testing.T) (*httptest.Server, *recordingMailer, *pgxpool.Pool) {
	t.Helper()
	return newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.ModeratorEmails = modAllowList(t, "mod@example.com")
		d.RequestLinkRateLimit = api.RequestLinkRateLimit{Burst: 1000, Every: time.Millisecond}
	})
}

// revNoFollow returns a client sharing c's cookie jar that reports a redirect
// as a redirect.
func revNoFollow(c *http.Client) *http.Client {
	return &http.Client{Jar: c.Jar, CheckRedirect: modNoFollow}
}

// revCookieCopy returns a second browser holding a copy of from's cookies.
func revCookieCopy(t *testing.T, from *http.Client, srv *httptest.Server) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	jar.SetCookies(u, from.Jar.Cookies(u))
	return &http.Client{Jar: jar}
}

// revStatus GETs path with c, not following redirects, and returns the status
// and the Location header.
func revStatus(t *testing.T, c *http.Client, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := revNoFollow(c).Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, resp.Header.Get("Location")
}

const revNearbyPath = "/api/reports?lat=12.9716&lon=77.5946&radius_km=5"

// revExpect fails unless GET /api/reports answers wantAPI and GET /profile
// answers wantPage.
func revExpect(t *testing.T, label string, c *http.Client, srv *httptest.Server, wantAPI, wantPage int) {
	t.Helper()
	if got, _ := revStatus(t, c, srv, revNearbyPath); got != wantAPI {
		t.Errorf("%s: GET /api/reports = %d, want %d", label, got, wantAPI)
	}
	got, loc := revStatus(t, c, srv, "/profile")
	if got != wantPage {
		t.Errorf("%s: GET /profile = %d, want %d", label, got, wantPage)
	}
	if wantPage == http.StatusFound && loc != "/login" {
		t.Errorf("%s: GET /profile redirects to %q, want /login", label, loc)
	}
}

// revAccountID reads an account id by email.
func revAccountID(t *testing.T, pool *pgxpool.Pool, email string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), `SELECT id FROM accounts WHERE email = $1`, email).Scan(&id); err != nil {
		t.Fatalf("reading account id for a test address: %v", err)
	}
	return id
}

// revRevoke runs the statement RevokeAccountSessions runs.
func revRevoke(t *testing.T, pool *pgxpool.Pool, accountID int64) {
	t.Helper()
	testutil.MustExec(t, pool, `UPDATE sessions SET revoked_at = now() WHERE account_id = $1 AND revoked_at IS NULL`, accountID)
}

// revSignIn drives request link then open link for email with client, after
// clearing the address's resend cooldown (45 seconds) so a test can sign the
// same address in more than once. It returns the verify response status, body
// and Set-Cookie cookies.
func revSignIn(t *testing.T, client *http.Client, srv *httptest.Server, mailer *recordingMailer, pool *pgxpool.Pool, email string) (int, string, []*http.Cookie) {
	t.Helper()
	testutil.MustExec(t, pool, `DELETE FROM email_cooldowns WHERE email = $1`, email)
	token := requestLink(t, client, srv, mailer, email)
	resp, err := client.Get(srv.URL + "/auth/verify?token=" + url.QueryEscape(token))
	if err != nil {
		t.Fatalf("GET /auth/verify: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /auth/verify body: %v", err)
	}
	return resp.StatusCode, string(body), resp.Cookies()
}

// revSession reads a session row: its account id (nil when unbound) and
// whether revoked_at is set.
func revSession(t *testing.T, pool *pgxpool.Pool, sessionID string) (accountID *int64, revoked bool) {
	t.Helper()
	if err := pool.QueryRow(context.Background(),
		`SELECT account_id, revoked_at IS NOT NULL FROM sessions WHERE session_id = $1`, sessionID,
	).Scan(&accountID, &revoked); err != nil {
		t.Fatalf("reading the session row: %v", err)
	}
	return accountID, revoked
}

// TestRevokedSessionIsRefused: after A's sessions are revoked, A's browser and
// a copy of its cookie both get 401 on the API and 302 to /login on pages. A
// third account is unaffected.
func TestRevokedSessionIsRefused(t *testing.T) {
	srv, mailer, pool := newRevServer(t)
	a := newVerifiedClient(t, srv, mailer, "rev-a@example.com")
	aCopy := revCookieCopy(t, a, srv)
	other := newVerifiedClient(t, srv, mailer, "rev-other@example.com")

	revExpect(t, "A before", a, srv, http.StatusOK, http.StatusOK)
	revExpect(t, "A copy before", aCopy, srv, http.StatusOK, http.StatusOK)

	revRevoke(t, pool, revAccountID(t, pool, "rev-a@example.com"))

	revExpect(t, "A after", a, srv, http.StatusUnauthorized, http.StatusFound)
	revExpect(t, "A copy after", aCopy, srv, http.StatusUnauthorized, http.StatusFound)
	revExpect(t, "other after", other, srv, http.StatusOK, http.StatusOK)
}

// TestDeletedAccountSessionIsRefused: an account with deleted_at set is refused
// the same way with its session rows untouched.
func TestDeletedAccountSessionIsRefused(t *testing.T) {
	srv, mailer, pool := newRevServer(t)
	a := newVerifiedClient(t, srv, mailer, "del-a@example.com")
	aCopy := revCookieCopy(t, a, srv)
	other := newVerifiedClient(t, srv, mailer, "del-other@example.com")
	aID := revAccountID(t, pool, "del-a@example.com")

	revExpect(t, "A before", a, srv, http.StatusOK, http.StatusOK)

	testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, aID)

	revExpect(t, "A after", a, srv, http.StatusUnauthorized, http.StatusFound)
	revExpect(t, "A copy after", aCopy, srv, http.StatusUnauthorized, http.StatusFound)
	revExpect(t, "other after", other, srv, http.StatusOK, http.StatusOK)

	accountID, revoked := revSession(t, pool, sessionIDFromJar(t, a, srv.URL))
	if accountID == nil || *accountID != aID || revoked {
		t.Errorf("session row after the deletion: account %v, revoked %v, want account %d and not revoked", accountID, revoked, aID)
	}
}

// revReportJSON returns the raw JSON of report id in reader's nearby feed.
func revReportJSON(t *testing.T, reader *http.Client, srv *httptest.Server, id int64) []byte {
	t.Helper()
	resp, err := reader.Get(srv.URL + revNearbyPath)
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/reports: status %d, err %v: %s", resp.StatusCode, err, body)
	}
	var out struct {
		Reports []json.RawMessage `json:"reports"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding the feed: %v", err)
	}
	for _, raw := range out.Reports {
		var head struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			t.Fatalf("decoding a feed row: %v", err)
		}
		if head.ID == id {
			return raw
		}
	}
	t.Fatalf("report %d is not in the feed: %s", id, body)
	return nil
}

const revMarker = "Authorship marker: water over the old culvert"

// revPost files a report with the marker text as client.
func revPost(t *testing.T, client *http.Client, srv *httptest.Server) int64 {
	t.Helper()
	body := floodReportBody(revLat, revLon)
	body["description"] = revMarker
	return submitReportGetID(t, client, srv.URL, body)
}

// revProfile reads GET /profile as client and returns the body.
func revProfile(t *testing.T, client *http.Client, srv *httptest.Server) string {
	t.Helper()
	return getProfile(t, client, srv.URL)
}

// TestRevocationKeepsAuthorship: revoking A's sessions moves nothing. The
// report as another reader sees it is byte for byte the same, and A's session
// rows still name A.
func TestRevocationKeepsAuthorship(t *testing.T) {
	srv, mailer, pool := newRevServer(t)
	a := newVerifiedClient(t, srv, mailer, "auth-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "auth-b@example.com")
	reader := newVerifiedClient(t, srv, mailer, "auth-reader@example.com")
	aID := revAccountID(t, pool, "auth-a@example.com")

	id := revPost(t, a, srv)
	resp := postVote(t, b, srv.URL, id, "confirm", map[string]any{"latitude": revLat, "longitude": revLon})
	decodeVoteResponse(t, resp) // fails the test unless B's confirm answered 200

	before := revReportJSON(t, reader, srv, id)
	revRevoke(t, pool, aID)
	after := revReportJSON(t, reader, srv, id)
	if !bytes.Equal(before, after) {
		t.Errorf("the report changed for another reader when A's sessions were revoked:\nbefore %s\nafter  %s", before, after)
	}

	var bound, revoked int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*), count(*) FILTER (WHERE revoked_at IS NOT NULL) FROM sessions WHERE account_id = $1`, aID,
	).Scan(&bound, &revoked); err != nil {
		t.Fatalf("counting A's sessions: %v", err)
	}
	if bound == 0 || bound != revoked {
		t.Errorf("A's sessions: %d still name A, %d revoked; want every one of them kept and revoked", bound, revoked)
	}
	var nullBound int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM sessions s WHERE s.revoked_at IS NOT NULL AND s.account_id IS NULL`).Scan(&nullBound); err != nil {
		t.Fatalf("counting unbound revoked sessions: %v", err)
	}
	if nullBound != 0 {
		t.Errorf("%d revoked sessions lost their account", nullBound)
	}

	// A signing in again elsewhere still has the report on My activity.
	a2 := newJarClient(t)
	if status, body, _ := revSignIn(t, a2, srv, mailer, pool, "auth-a@example.com"); status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("A signing in again: status %d: %s", status, body)
	}
	if !strings.Contains(revProfile(t, a2, srv), revMarker) {
		t.Errorf("A's report is missing from My activity after A signed in again")
	}
}

// TestRevokedBrowserSignsInAgainAsSameEmail: A has two browsers, both revoked.
// One signs in again as A: it keeps its session id, revoked_at clears, and the
// pages work. The other stays refused.
func TestRevokedBrowserSignsInAgainAsSameEmail(t *testing.T) {
	srv, mailer, pool := newRevServer(t)
	first := newVerifiedClient(t, srv, mailer, "same-a@example.com")
	second := newJarClient(t)
	if status, body, _ := revSignIn(t, second, srv, mailer, pool, "same-a@example.com"); status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("second browser sign in: status %d: %s", status, body)
	}
	firstID, secondID := sessionIDFromJar(t, first, srv.URL), sessionIDFromJar(t, second, srv.URL)
	if firstID == secondID {
		t.Fatalf("the two browsers share session id %q", firstID)
	}

	revRevoke(t, pool, revAccountID(t, pool, "same-a@example.com"))
	revExpect(t, "second revoked", second, srv, http.StatusUnauthorized, http.StatusFound)

	status, body, cookies := revSignIn(t, second, srv, mailer, pool, "same-a@example.com")
	if status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("revoked browser signing in as the same address: status %d: %s", status, body)
	}
	for _, c := range cookies {
		if c.Name == "pinalert_session" {
			t.Errorf("sign in as the same address set a new session cookie, want the old session kept")
		}
	}
	if got := sessionIDFromJar(t, second, srv.URL); got != secondID {
		t.Errorf("session id changed from %q to %q", secondID, got)
	}
	if _, revoked := revSession(t, pool, secondID); revoked {
		t.Errorf("revoked_at was not cleared on the revived session")
	}
	revExpect(t, "second revived", second, srv, http.StatusOK, http.StatusOK)

	if _, revoked := revSession(t, pool, firstID); !revoked {
		t.Errorf("the other browser's session was revived too")
	}
	revExpect(t, "first still refused", first, srv, http.StatusUnauthorized, http.StatusFound)
}

// TestRevokedBrowserSignsInAsDifferentEmailGetsFreshSession is the rule that
// keeps history where it belongs: a revoked browser that signs in as someone
// else gets a new session id and cookie, and the old row keeps its author.
func TestRevokedBrowserSignsInAsDifferentEmailGetsFreshSession(t *testing.T) {
	srv, mailer, pool := newRevServer(t)
	browser := newVerifiedClient(t, srv, mailer, "swap-a@example.com")
	aID := revAccountID(t, pool, "swap-a@example.com")
	revPost(t, browser, srv)
	oldID := sessionIDFromJar(t, browser, srv.URL)

	revRevoke(t, pool, aID)

	status, body, cookies := revSignIn(t, browser, srv, mailer, pool, "swap-b@example.com")
	if status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("revoked browser signing in as a new address: status %d: %s", status, body)
	}
	var issued []string
	for _, c := range cookies {
		if c.Name == "pinalert_session" {
			issued = append(issued, c.Value)
		}
	}
	if len(issued) != 1 {
		t.Fatalf("sign in set %d pinalert_session cookies, want exactly 1", len(issued))
	}
	newID := sessionIDFromJar(t, browser, srv.URL)
	if newID == oldID || strings.SplitN(issued[0], ".", 2)[0] != newID {
		t.Fatalf("session id %q after sign in, old %q, cookie set %q: want a fresh id carried by the cookie", newID, oldID, issued[0])
	}

	// The old row keeps its author and stays revoked.
	oldAccount, oldRevoked := revSession(t, pool, oldID)
	if oldAccount == nil || *oldAccount != aID || !oldRevoked {
		t.Errorf("old session row: account %v, revoked %v, want account %d and revoked", oldAccount, oldRevoked, aID)
	}
	bID := revAccountID(t, pool, "swap-b@example.com")
	newAccount, newRevoked := revSession(t, pool, newID)
	if newAccount == nil || *newAccount != bID || newRevoked {
		t.Errorf("new session row: account %v, revoked %v, want account %d and live", newAccount, newRevoked, bID)
	}

	// The browser is now B and sees none of A's reports.
	if got, _ := revStatus(t, browser, srv, "/profile"); got != http.StatusOK {
		t.Fatalf("GET /profile as the new account = %d, want 200", got)
	}
	if strings.Contains(revProfile(t, browser, srv), revMarker) {
		t.Errorf("the new account's My activity lists A's report")
	}

	// A signing in again elsewhere still has it.
	a2 := newJarClient(t)
	if status, body, _ := revSignIn(t, a2, srv, mailer, pool, "swap-a@example.com"); status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("A signing in again: status %d: %s", status, body)
	}
	if !strings.Contains(revProfile(t, a2, srv), revMarker) {
		t.Errorf("A's report is missing from My activity after A signed in again")
	}
}

// TestModeratorGateRefusesRevokedSession: a moderator whose session is revoked
// gets the moderator routes' plain not found like any other visitor.
func TestModeratorGateRefusesRevokedSession(t *testing.T) {
	srv, mailer, pool := newRevServer(t)
	mod := newVerifiedClient(t, srv, mailer, "mod@example.com")

	if got, _ := revStatus(t, mod, srv, "/moderate"); got != http.StatusOK {
		t.Fatalf("GET /moderate before the revocation = %d, want 200", got)
	}

	revRevoke(t, pool, revAccountID(t, pool, "mod@example.com"))

	noFollow := revNoFollow(mod)
	modSameAsUnknown(t, noFollow, srv.URL, http.MethodGet, "/moderate", "/no-such-page")
	modSameAsUnknown(t, noFollow, srv.URL, http.MethodGet, "/api/moderation/queue", "/api/no-such-route")
}
