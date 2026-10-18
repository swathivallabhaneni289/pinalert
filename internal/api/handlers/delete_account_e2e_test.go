// Package handlers_test covers Phase 3.2 plan 09 end to end: Delete my account
// by Option A, anonymise. The account is scrubbed in place, the person's own
// rows are deleted, every session ends, and nothing another account sees moves.
package handlers_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// newDelServer is the harness: the real transaction seam is wired from the test
// pool, so the path production runs is the path tested. txRuns counts how many
// times the seam ran.
func newDelServer(t *testing.T) (srv *httptest.Server, mailer *recordingMailer, pool *pgxpool.Pool, txRuns *atomic.Int64) {
	t.Helper()
	txRuns = new(atomic.Int64)
	var poolRef *pgxpool.Pool
	srv, mailer, pool = newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.ModeratorEmails = modAllowList(t, "mod@example.com")
		d.RequestLinkRateLimit = api.RequestLinkRateLimit{Burst: 1000, Every: time.Millisecond}
		d.AccountTx = func(ctx context.Context, fn func(q service.AccountDeleteQuerier) error) error {
			txRuns.Add(1)
			return service.PgxAccountTx(poolRef)(ctx, fn)
		}
	})
	poolRef = pool
	return srv, mailer, pool, txRuns
}

// delPost POSTs body to /api/me/delete as client and returns the status, the
// Set-Cookie cookies and the response body.
func delPost(t *testing.T, client *http.Client, srv *httptest.Server, body string) (int, []*http.Cookie, string) {
	t.Helper()
	resp, err := client.Post(srv.URL+"/api/me/delete", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/me/delete: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the delete response: %v", err)
	}
	return resp.StatusCode, resp.Cookies(), string(b)
}

// delCount runs a count query.
func delCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", sql, err)
	}
	return n
}

// TestDeleteAccountLeavesOtherScreensUnchanged is the proof for Pitfall 14. A
// is a rated reporter with a live report R that two people confirmed, and A
// confirmed B's report S. Reader D sees R and S before and after A deletes the
// account: the bytes are the same, so no count, view, confirmed-by figure or
// reporter rating moved. The votes and reports tables keep every row.
func TestDeleteAccountLeavesOtherScreensUnchanged(t *testing.T) {
	srv, mailer, pool, txRuns := newDelServer(t)
	a := newVerifiedClient(t, srv, mailer, "del-screens-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "del-screens-b@example.com")
	c := newVerifiedClient(t, srv, mailer, "del-screens-c@example.com")
	d := newVerifiedClient(t, srv, mailer, "del-screens-d@example.com")
	aID := revAccountID(t, pool, "del-screens-a@example.com")
	aSession := sessionIDFromJar(t, a, srv.URL)

	// A's rating: six expired reports on A's own session, five of them
	// corroborated by two seed voters from two cells, as
	// TestReporterSignalFromSeededHistory seeds a rated reporter.
	voterA := testutil.SeedReporterAccount(t, pool, "del-screens-voter-a@example.com")
	voterB := testutil.SeedReporterAccount(t, pool, "del-screens-voter-b@example.com")
	expiresAt := time.Now().Add(-time.Hour)
	for i := 0; i < 6; i++ {
		id := testutil.SeedHistoryReport(t, pool, aSession, "low", "flood", expiresAt)
		if i == 5 {
			continue
		}
		for _, v := range []struct {
			account int64
			cell    string
		}{{voterA, p14HistoryCellA}, {voterB, p14HistoryCellB}} {
			testutil.MustExec(t, pool,
				`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at) VALUES ($1, $2, 'content', 'confirm', $3, $4)`,
				id, v.account, v.cell, expiresAt.Add(-2*time.Hour))
		}
	}

	r := revPost(t, a, srv)
	s := submitReportGetID(t, b, srv.URL, floodReportBody(revLat, revLon))
	north := map[string]any{"latitude": revLat + 0.003, "longitude": revLon}
	south := map[string]any{"latitude": revLat - 0.003, "longitude": revLon}
	for _, v := range []struct {
		client *http.Client
		report int64
		at     map[string]any
	}{{b, r, north}, {c, r, south}, {a, s, north}} {
		decodeVoteResponse(t, postVote(t, v.client, srv.URL, v.report, "confirm", v.at))
	}

	rBefore := revReportJSON(t, d, srv, r)
	sBefore := revReportJSON(t, d, srv, s)
	if !bytes.Contains(rBefore, []byte("Reliable reporter")) {
		t.Fatalf("the fixture is wrong: A is not a rated reporter on R: %s", rBefore)
	}
	votesBefore := delCount(t, pool, `SELECT count(*) FROM votes`)
	reportsBefore := delCount(t, pool, `SELECT count(*) FROM reports`)
	sessionsBound := delCount(t, pool, `SELECT count(*) FROM sessions WHERE account_id = $1`, aID)

	status, _, body := delPost(t, a, srv, `{"confirm": true}`)
	if status != http.StatusNoContent {
		t.Fatalf("POST /api/me/delete = %d, want 204: %s", status, body)
	}
	if got := txRuns.Load(); got != 1 {
		t.Errorf("the transaction seam ran %d times, want 1", got)
	}

	if after := revReportJSON(t, d, srv, r); !bytes.Equal(rBefore, after) {
		t.Errorf("R changed for another reader:\nbefore %s\nafter  %s", rBefore, after)
	}
	if after := revReportJSON(t, d, srv, s); !bytes.Equal(sBefore, after) {
		t.Errorf("S changed for another reader:\nbefore %s\nafter  %s", sBefore, after)
	}
	if got := delCount(t, pool, `SELECT count(*) FROM votes`); got != votesBefore {
		t.Errorf("votes rows: %d before, %d after", votesBefore, got)
	}
	if got := delCount(t, pool, `SELECT count(*) FROM reports`); got != reportsBefore {
		t.Errorf("reports rows: %d before, %d after", reportsBefore, got)
	}
	if got := delCount(t, pool, `SELECT count(*) FROM sessions WHERE account_id = $1`, aID); got != sessionsBound {
		t.Errorf("sessions still naming the account: %d before, %d after", sessionsBound, got)
	}
}

// TestDeleteAccountEndsEverySession: A's browser and a copy of its cookie are
// both refused after the delete, and the response clears the cookie.
func TestDeleteAccountEndsEverySession(t *testing.T) {
	srv, mailer, pool, _ := newDelServer(t)
	a := newVerifiedClient(t, srv, mailer, "del-sessions-a@example.com")
	aCopy := revCookieCopy(t, a, srv)
	other := newVerifiedClient(t, srv, mailer, "del-sessions-other@example.com")

	revExpect(t, "A before", a, srv, http.StatusOK, http.StatusOK)
	revExpect(t, "A copy before", aCopy, srv, http.StatusOK, http.StatusOK)

	status, cookies, body := delPost(t, a, srv, `{"confirm": true}`)
	if status != http.StatusNoContent {
		t.Fatalf("POST /api/me/delete = %d, want 204: %s", status, body)
	}
	cleared := false
	for _, ck := range cookies {
		if ck.Name == "pinalert_session" && ck.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Errorf("the response did not clear the session cookie: %v", cookies)
	}

	revExpect(t, "A after", a, srv, http.StatusUnauthorized, http.StatusFound)
	revExpect(t, "A copy after", aCopy, srv, http.StatusUnauthorized, http.StatusFound)
	revExpect(t, "other after", other, srv, http.StatusOK, http.StatusOK)

	aID := delCount(t, pool, `SELECT id FROM accounts WHERE deleted_at IS NOT NULL`)
	if live := delCount(t, pool, `SELECT count(*) FROM sessions WHERE account_id = $1 AND revoked_at IS NULL`, aID); live != 0 {
		t.Errorf("%d sessions of the deleted account are still live", live)
	}
}

// TestDeleteAccountRemovesPersonalRows: the person's own rows are gone, the
// accounts row is scrubbed in place, and another account's rows are untouched.
func TestDeleteAccountRemovesPersonalRows(t *testing.T) {
	srv, mailer, pool, _ := newDelServer(t)
	const email = "del-rows-a@example.com"
	a := newVerifiedClient(t, srv, mailer, email)
	b := newVerifiedClient(t, srv, mailer, "del-rows-b@example.com")
	aID := revAccountID(t, pool, email)
	bID := revAccountID(t, pool, "del-rows-b@example.com")
	rID := revPost(t, b, srv)

	for _, id := range []int64{aID, bID} {
		testutil.MustExec(t, pool, `INSERT INTO updates (account_id, report_id, kind) VALUES ($1, $2, 'resolved')`, id, rID)
		testutil.MustExec(t, pool,
			`INSERT INTO update_cursors (account_id, report_id, last_state, adopted_at) VALUES ($1, $2, 'live', now())`, id, rID)
		testutil.MustExec(t, pool, `INSERT INTO report_mutes (account_id, report_id) VALUES ($1, $2)`, id, rID)
		testutil.MustExec(t, pool, `INSERT INTO account_alert_settings (account_id, reliable_alerts_on) VALUES ($1, true)`, id)
	}
	testutil.MustExec(t, pool,
		`UPDATE accounts SET display_name = 'Asha', home_area_name = 'Indiranagar', home_area_cell = 'tdr1v',
		        terms_agreed_at = now(), terms_version = 'v1', first_run_done_at = now() WHERE id = $1`, aID)

	if delCount(t, pool, `SELECT count(*) FROM magic_link_tokens WHERE email = $1`, email) == 0 ||
		delCount(t, pool, `SELECT count(*) FROM email_cooldowns WHERE email = $1`, email) == 0 {
		t.Fatalf("the fixture is wrong: the sign in left no token or cooldown row for the address")
	}
	votesBefore := delCount(t, pool, `SELECT count(*) FROM votes`)
	reportsBefore := delCount(t, pool, `SELECT count(*) FROM reports`)

	if status, _, body := delPost(t, a, srv, `{"confirm": true}`); status != http.StatusNoContent {
		t.Fatalf("POST /api/me/delete = %d, want 204: %s", status, body)
	}

	for _, table := range []string{"updates", "update_cursors", "report_mutes", "account_alert_settings"} {
		if n := delCount(t, pool, `SELECT count(*) FROM `+table+` WHERE account_id = $1`, aID); n != 0 {
			t.Errorf("%s still has %d rows for the deleted account", table, n)
		}
		if n := delCount(t, pool, `SELECT count(*) FROM `+table+` WHERE account_id = $1`, bID); n != 1 {
			t.Errorf("%s has %d rows for the other account, want 1", table, n)
		}
	}
	if n := delCount(t, pool, `SELECT count(*) FROM magic_link_tokens WHERE email = $1`, email); n != 0 {
		t.Errorf("%d sign in links of the old address remain", n)
	}
	if n := delCount(t, pool, `SELECT count(*) FROM email_cooldowns WHERE email = $1`, email); n != 0 {
		t.Errorf("%d resend cooldowns of the old address remain", n)
	}

	var gotEmail string
	var name, areaName, areaCell, termsVersion *string
	var termsAt, firstRun, deletedAt *time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT email, display_name, home_area_name, home_area_cell, terms_version, terms_agreed_at, first_run_done_at, deleted_at
		   FROM accounts WHERE id = $1`, aID,
	).Scan(&gotEmail, &name, &areaName, &areaCell, &termsVersion, &termsAt, &firstRun, &deletedAt); err != nil {
		t.Fatalf("reading the scrubbed account: %v", err)
	}
	if gotEmail != service.DeletedEmailFor(aID) || !strings.HasSuffix(gotEmail, "@deleted.invalid") {
		t.Errorf("email = %q, want %q", gotEmail, service.DeletedEmailFor(aID))
	}
	if name != nil || areaName != nil || areaCell != nil || termsVersion != nil || termsAt != nil || firstRun != nil {
		t.Errorf("personal columns not cleared: name %v area %v/%v terms %v/%v firstrun %v", name, areaName, areaCell, termsVersion, termsAt, firstRun)
	}
	if deletedAt == nil {
		t.Errorf("deleted_at is not set")
	}

	if got := delCount(t, pool, `SELECT count(*) FROM votes`); got != votesBefore {
		t.Errorf("votes rows: %d before, %d after", votesBefore, got)
	}
	if got := delCount(t, pool, `SELECT count(*) FROM reports`); got != reportsBefore {
		t.Errorf("reports rows: %d before, %d after", reportsBefore, got)
	}
}

// TestDeletedEmailCanSignUpAgainWithNoHistory: the same address signs up again
// as a new account that has none of the old account's reports or votes.
func TestDeletedEmailCanSignUpAgainWithNoHistory(t *testing.T) {
	srv, mailer, pool, _ := newDelServer(t)
	const email = "del-again-a@example.com"
	a := newVerifiedClient(t, srv, mailer, email)
	b := newVerifiedClient(t, srv, mailer, "del-again-b@example.com")
	aID := revAccountID(t, pool, email)
	bReport := submitReportGetID(t, b, srv.URL, floodReportBody(revLat, revLon))
	revPost(t, a, srv)
	decodeVoteResponse(t, postVote(t, a, srv.URL, bReport, "confirm", map[string]any{"latitude": revLat + 0.003, "longitude": revLon}))

	if status, _, body := delPost(t, a, srv, `{"confirm": true}`); status != http.StatusNoContent {
		t.Fatalf("POST /api/me/delete = %d, want 204: %s", status, body)
	}

	fresh := newJarClient(t)
	if status, body, _ := revSignIn(t, fresh, srv, mailer, pool, email); status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("signing up again: status %d: %s", status, body)
	}
	newID := revAccountID(t, pool, email)
	if newID == aID {
		t.Fatalf("the new account reuses the deleted account id %d", aID)
	}
	profile := revProfile(t, fresh, srv)
	if strings.Contains(profile, revMarker) {
		t.Errorf("the new account's My activity lists the deleted account's report")
	}
	if n := delCount(t, pool, `SELECT count(*) FROM votes WHERE account_id = $1`, newID); n != 0 {
		t.Errorf("the new account has %d votes", n)
	}
	if n := delCount(t, pool,
		`SELECT count(*) FROM reports r JOIN sessions s ON s.session_id = r.session_id WHERE s.account_id = $1`, newID); n != 0 {
		t.Errorf("the new account owns %d reports", n)
	}
	if got := delCount(t, pool, `SELECT count(*) FROM accounts WHERE id = $1 AND deleted_at IS NOT NULL`, aID); got != 1 {
		t.Errorf("the deleted account row is gone or revived")
	}
}

// TestDeletedBrowserSignsInGetsFreshSession: a browser that still holds the
// deleted account's cookie (the delete clears only the cookie of the browser
// that sent it) signs in again with the old address. It gets a new session id
// and cookie, the old session row still names the deleted account, and it
// inherits nothing.
func TestDeletedBrowserSignsInGetsFreshSession(t *testing.T) {
	srv, mailer, pool, _ := newDelServer(t)
	const email = "del-browser-a@example.com"
	sender := newVerifiedClient(t, srv, mailer, email)
	a := revCookieCopy(t, sender, srv)
	aID := revAccountID(t, pool, email)
	revPost(t, a, srv)
	oldID := sessionIDFromJar(t, a, srv.URL)

	if status, _, body := delPost(t, sender, srv, `{"confirm": true}`); status != http.StatusNoContent {
		t.Fatalf("POST /api/me/delete = %d, want 204: %s", status, body)
	}
	revExpect(t, "stale browser after the delete", a, srv, http.StatusUnauthorized, http.StatusFound)

	status, body, _ := revSignIn(t, a, srv, mailer, pool, email)
	if status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("signing in again: status %d: %s", status, body)
	}
	newSession := sessionIDFromJar(t, a, srv.URL)
	if newSession == oldID {
		t.Fatalf("the browser kept its old session id %q", oldID)
	}

	oldAccount, oldRevoked := revSession(t, pool, oldID)
	if oldAccount == nil || *oldAccount != aID || !oldRevoked {
		t.Errorf("old session row: account %v, revoked %v, want account %d and revoked", oldAccount, oldRevoked, aID)
	}
	newAccount, newRevoked := revSession(t, pool, newSession)
	newID := revAccountID(t, pool, email)
	if newAccount == nil || *newAccount != newID || newID == aID || newRevoked {
		t.Errorf("new session row: account %v, revoked %v, want the new account %d (not %d) and live", newAccount, newRevoked, newID, aID)
	}
	if got, _ := revStatus(t, a, srv, "/profile"); got != http.StatusOK {
		t.Fatalf("GET /profile after signing in again = %d, want 200", got)
	}
	if strings.Contains(revProfile(t, a, srv), revMarker) {
		t.Errorf("the browser inherited the deleted account's report")
	}
}

// TestDeleteAccountNeedsConfirm: a body that does not say confirm true deletes
// nothing, and a caller with no session cannot delete anything.
func TestDeleteAccountNeedsConfirm(t *testing.T) {
	srv, mailer, pool, txRuns := newDelServer(t)
	a := newVerifiedClient(t, srv, mailer, "del-confirm-a@example.com")

	for _, body := range []string{
		``,
		`{}`,
		`{"confirm": false}`,
		`{"confirm": "yes"}`,
		`{"confirm": true, "extra": 1}`,
		`not json`,
	} {
		if status, _, got := delPost(t, a, srv, body); status != http.StatusBadRequest {
			t.Errorf("body %q: status %d, want 400: %s", body, status, got)
		}
	}
	if status, _, got := delPost(t, a, srv, `{"confirm": false}`); status != http.StatusBadRequest || !strings.Contains(got, `"confirm"`) ||
		!strings.Contains(got, service.DeleteConfirmMessage) {
		t.Errorf(`{"confirm": false}: status %d body %s, want 400 naming the confirm field and %q`, status, got, service.DeleteConfirmMessage)
	}
	if got := txRuns.Load(); got != 0 {
		t.Errorf("the transaction seam ran %d times for refused bodies", got)
	}
	if n := delCount(t, pool, `SELECT count(*) FROM accounts WHERE deleted_at IS NOT NULL`); n != 0 {
		t.Errorf("%d accounts were deleted by refused bodies", n)
	}
	revExpect(t, "A after refused bodies", a, srv, http.StatusOK, http.StatusOK)

	anon := newJarClient(t)
	if status, _, got := delPost(t, anon, srv, `{"confirm": true}`); status != http.StatusUnauthorized {
		t.Errorf("an anonymous caller got %d, want 401: %s", status, got)
	}

	// A deleted account's own copy of the request is refused at the gate.
	if status, _, got := delPost(t, a, srv, `{"confirm": true}`); status != http.StatusNoContent {
		t.Fatalf("POST /api/me/delete = %d, want 204: %s", status, got)
	}
	if status, _, _ := delPost(t, a, srv, `{"confirm": true}`); status != http.StatusUnauthorized {
		t.Errorf("a second delete from the deleted account got %d, want 401", status)
	}
}
