// push_signout_e2e_test.go proves plan 03.3-10 end to end over the real router
// and a real Postgres: every way of signing out also stops phone notifications
// for the devices it ends. Plain Log out removes the subscriptions its own
// session registered, Log out of all devices removes every one of the account,
// and Delete my account removes them and the account's queued push rows, each
// leaving another account's rows alone. A failing delete never blocks the log
// out. The DB tests run only with DATABASE_URL set. Every package level
// identifier carries the signout prefix. The endpoints and keys are throwaway.
package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/push"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const signoutEndpointC = "https://fcm.googleapis.com/fcm/send/e2e-endpoint-secret-cccc"

// newSignoutServer is the real router with a push key, the real account
// deletion transaction and an open request link budget.
func newSignoutServer(t *testing.T) (*httptest.Server, *recordingMailer, *pgxpool.Pool) {
	t.Helper()
	var poolRef *pgxpool.Pool
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.Page.PushPublicKey = pushTestKey
		d.AccountTx = func(ctx context.Context, fn func(q service.AccountDeleteQuerier) error) error {
			return service.PgxAccountTx(poolRef)(ctx, fn)
		}
	})
	poolRef = pool
	return srv, mailer, pool
}

func signoutRegister(t *testing.T, client *http.Client, srv *httptest.Server, endpoint string) {
	t.Helper()
	resp := meSend(t, client, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, endpoint))
	meWant(t, "register "+endpoint[len(endpoint)-4:], resp, http.StatusNoContent)
}

func signoutCount(t *testing.T, pool *pgxpool.Pool, table string, accountID int64) int64 {
	t.Helper()
	return delCount(t, pool, `SELECT count(*) FROM `+table+` WHERE account_id = $1`, accountID)
}

func TestPushSubscriptionsRemovedOn(t *testing.T) {
	t.Run("plain log out", func(t *testing.T) {
		srv, mailer, pool := newSignoutServer(t)
		const email = "signout-plain@example.com"
		first := newVerifiedClient(t, srv, mailer, email)
		second := newJarClient(t)
		if status, body, _ := revSignIn(t, second, srv, mailer, pool, email); status != http.StatusOK || !strings.Contains(body, revSignedInMarker) {
			t.Fatalf("second sign in: status %d: %s", status, body)
		}
		accountID := revAccountID(t, pool, email)
		firstSession := sessionIDFromJar(t, first, srv.URL)
		secondSession := sessionIDFromJar(t, second, srv.URL)
		if firstSession == secondSession {
			t.Fatalf("the two browsers share one session")
		}
		signoutRegister(t, first, srv, pushTestEndpointA)
		signoutRegister(t, second, srv, pushTestEndpointB)
		if n := signoutCount(t, pool, "push_subscriptions", accountID); n != 2 {
			t.Fatalf("before log out the account holds %d subscriptions, want 2", n)
		}

		resp := meSend(t, revNoFollow(first), http.MethodPost, srv.URL+"/auth/logout", "")
		meWant(t, "POST /auth/logout", resp, http.StatusSeeOther)
		if loc := resp.Header.Get("Location"); loc != "/login" {
			t.Errorf("Location = %q, want /login", loc)
		}
		if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", cc)
		}

		if rows := pushRows(t, pool, pushTestEndpointA); len(rows) != 0 {
			t.Errorf("the logged out session's subscription remains: %+v", rows)
		}
		rows := pushRows(t, pool, pushTestEndpointB)
		if len(rows) != 1 || rows[0].sessionID != secondSession {
			t.Errorf("the other session's subscription = %+v, want one row for session %s", rows, secondSession)
		}
		// DEC-K: the session row stays bound to its account.
		if n := delCount(t, pool, `SELECT count(*) FROM sessions WHERE session_id = $1 AND account_id = $2`, firstSession, accountID); n != 1 {
			t.Errorf("the logged out session is no longer bound to its account")
		}
	})

	t.Run("log out of all devices", func(t *testing.T) {
		srv, mailer, pool := newSignoutServer(t)
		const email = "signout-all@example.com"
		first := newVerifiedClient(t, srv, mailer, email)
		second := newJarClient(t)
		if status, body, _ := revSignIn(t, second, srv, mailer, pool, email); status != http.StatusOK || !strings.Contains(body, revSignedInMarker) {
			t.Fatalf("second sign in: status %d: %s", status, body)
		}
		other := newVerifiedClient(t, srv, mailer, "signout-all-other@example.com")
		accountID := revAccountID(t, pool, email)
		otherID := revAccountID(t, pool, "signout-all-other@example.com")
		signoutRegister(t, first, srv, pushTestEndpointA)
		signoutRegister(t, second, srv, pushTestEndpointB)
		signoutRegister(t, other, srv, signoutEndpointC)

		if status, _, _, body := logoutAllPost(t, first, srv.URL, `{}`); status != http.StatusNoContent {
			t.Fatalf("POST /api/me/logout-all = %d, want 204: %s", status, body)
		}
		if n := signoutCount(t, pool, "push_subscriptions", accountID); n != 0 {
			t.Errorf("%d subscriptions of the account remain", n)
		}
		if n := signoutCount(t, pool, "push_subscriptions", otherID); n != 1 {
			t.Errorf("the other account holds %d subscriptions, want 1", n)
		}
	})

	t.Run("delete my account", func(t *testing.T) {
		srv, mailer, pool := newSignoutServer(t)
		const email = "signout-del-a@example.com"
		a := newVerifiedClient(t, srv, mailer, email)
		b := newVerifiedClient(t, srv, mailer, "signout-del-b@example.com")
		aID := revAccountID(t, pool, email)
		bID := revAccountID(t, pool, "signout-del-b@example.com")
		reportID := revPost(t, b, srv)
		signoutRegister(t, a, srv, pushTestEndpointA)
		signoutRegister(t, b, srv, pushTestEndpointB)
		for _, id := range []int64{aID, bID} {
			testutil.MustExec(t, pool,
				`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'critical', now() + interval '1 hour')`,
				id, reportID)
		}

		if status, _, body := delPost(t, a, srv, `{"confirm": true}`); status != http.StatusNoContent {
			t.Fatalf("POST /api/me/delete = %d, want 204: %s", status, body)
		}
		for _, table := range []string{"push_subscriptions", "push_outbox"} {
			if n := signoutCount(t, pool, table, aID); n != 0 {
				t.Errorf("%s still has %d rows for the deleted account", table, n)
			}
			if n := signoutCount(t, pool, table, bID); n != 1 {
				t.Errorf("%s has %d rows for the other account, want 1", table, n)
			}
		}
	})
}

// signoutFailingQuerier fails the one delete Logout makes and counts the try.
type signoutFailingQuerier struct {
	service.PushSubsQuerier
	tried int
}

func (f *signoutFailingQuerier) DeletePushSubscriptionsForSession(ctx context.Context, sessionID string) (int64, error) {
	f.tried++
	return 0, context.DeadlineExceeded
}

// TestLogoutStillWorksWhenDeleteFails: a subscription service whose store fails
// stops the log out instead of hiding the failure.
//
// Changed 2026-10-25 (review S-03): this test used to expect the usual 303 to
// /login with the cookie cleared. Plain Log out never revokes the session row
// (DEC-K), so a subscription left behind would keep sending the previous
// person's alerts to a shared phone with nothing else to stop it. The handler
// now answers the app's usual 500 and keeps the cookie, so the person can press
// Log out again. The name is kept so the history stays easy to follow. It needs
// no database.
func TestLogoutStillWorksWhenDeleteFails(t *testing.T) {
	mgr, err := session.NewManager([]byte("signout-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	q := &signoutFailingQuerier{}
	subs := service.NewPushSubscriptionService(q, push.DefaultEndpointPolicy())
	h := mgr.Middleware(func(context.Context, string) error { return nil })(handlers.Logout(mgr, subs))

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "pinalert_session", Value: mgr.Sign(mgr.NewID())})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if q.tried != 1 {
		t.Fatalf("the delete was tried %d times, want 1", q.tried)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("Location %q, want none: the person stays to try again", loc)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control %q, want no-store", cc)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "pinalert_session" && c.MaxAge < 0 {
			t.Errorf("the cookie was cleared although this device's alerts are still on: %v", c)
		}
	}
}

// TestLogoutWithoutSubscriptionServiceStillWorks: a nil service (a harness that
// builds the handler without one) is skipped, not dereferenced.
func TestLogoutWithoutSubscriptionServiceStillWorks(t *testing.T) {
	mgr, err := session.NewManager([]byte("signout-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	h := mgr.Middleware(func(context.Context, string) error { return nil })(handlers.Logout(mgr, nil))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/logout", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}
}
