// push_e2e_test.go proves the three phone alert subscription routes of plan
// 03.3-08 end to end over the real router and a real Postgres: register, ask
// and remove a device, the move of a shared endpoint between accounts, the gate,
// the per account budgets, the missing key answer and the strict JSON rules.
// They run only with DATABASE_URL set. Every package level identifier carries
// the push prefix. The endpoints and keys are built in code and are throwaway.
package handlers_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	pushTestKey       = "throwaway-public-key-for-tests"
	pushTestEndpointA = "https://fcm.googleapis.com/fcm/send/e2e-endpoint-secret-aaaa"
	pushTestEndpointB = "https://updates.push.services.mozilla.com/wpush/v2/e2e-endpoint-secret-bbbb"
)

// pushKeys builds a well formed key pair in code.
func pushKeys() (p256dh, auth string) {
	pub := make([]byte, 65)
	pub[0] = 0x04
	for i := 1; i < len(pub); i++ {
		pub[i] = byte(i * 3)
	}
	secret := make([]byte, 16)
	for i := range secret {
		secret[i] = byte(7 + i)
	}
	return base64.RawURLEncoding.EncodeToString(pub), base64.RawURLEncoding.EncodeToString(secret)
}

// pushSubBody is the JSON body of a register call, as a browser sends it.
func pushSubBody(t *testing.T, endpoint string) string {
	t.Helper()
	p, a := pushKeys()
	raw, err := json.Marshal(map[string]any{
		"endpoint":       endpoint,
		"expirationTime": nil,
		"keys":           map[string]string{"p256dh": p, "auth": a},
	})
	if err != nil {
		t.Fatalf("marshalling a subscription: %v", err)
	}
	return string(raw)
}

func pushEndpointBody(endpoint string) string {
	raw, _ := json.Marshal(map[string]string{"endpoint": endpoint})
	return string(raw)
}

// newPushServer is newE2EServer with a push key on the page config.
func newPushServer(t *testing.T) (*httptest.Server, *recordingMailer, *pgxpool.Pool) {
	t.Helper()
	return newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.Page.PushPublicKey = pushTestKey
	})
}

type pushRow struct {
	accountID int64
	sessionID string
	host      string
}

// pushRows reads every stored subscription for an endpoint.
func pushRows(t *testing.T, pool *pgxpool.Pool, endpoint string) []pushRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT account_id, session_id, endpoint_host FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	if err != nil {
		t.Fatalf("reading subscriptions: %v", err)
	}
	defer rows.Close()
	var out []pushRow
	for rows.Next() {
		var r pushRow
		if err := rows.Scan(&r.accountID, &r.sessionID, &r.host); err != nil {
			t.Fatalf("scanning a subscription: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// pushAccountAndSession returns the account id and the bound session id of an
// email address.
func pushAccountAndSession(t *testing.T, pool *pgxpool.Pool, email string) (int64, string) {
	t.Helper()
	var id int64
	var sid string
	err := pool.QueryRow(context.Background(),
		`SELECT a.id, s.session_id FROM accounts a JOIN sessions s ON s.account_id = a.id WHERE a.email = $1`, email).Scan(&id, &sid)
	if err != nil {
		t.Fatalf("reading the account and session of %s: %v", email, err)
	}
	return id, sid
}

// pushNoSecrets fails when a response body holds an endpoint or a key.
func pushNoSecrets(t *testing.T, label, body string) {
	t.Helper()
	p, a := pushKeys()
	for _, secret := range []string{"e2e-endpoint-secret", pushTestEndpointA, pushTestEndpointB, p, a} {
		if strings.Contains(body, secret) {
			t.Fatalf("%s: the response holds a secret: %s", label, body)
		}
	}
}

func pushStatus(t *testing.T, client *http.Client, srv *httptest.Server, endpoint string) bool {
	t.Helper()
	resp := meSend(t, client, http.MethodPost, srv.URL+"/api/me/push/status", pushEndpointBody(endpoint))
	body := meWant(t, "status", resp, http.StatusOK)
	pushNoSecrets(t, "status", body)
	var got struct {
		Registered bool `json:"registered"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("status body %q: %v", body, err)
	}
	return got.Registered
}

func TestPushSubscriptionRoutes(t *testing.T) {
	srv, mailer, pool := newPushServer(t)
	client := newJarClient(t)
	verifySession(t, client, srv, mailer, "push-routes@example.com")
	accID, sessID := pushAccountAndSession(t, pool, "push-routes@example.com")

	if pushStatus(t, client, srv, pushTestEndpointA) {
		t.Fatal("a device that never registered reads as registered")
	}

	resp := meSend(t, client, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, pushTestEndpointA))
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
	body := meWant(t, "register", resp, http.StatusNoContent)
	pushNoSecrets(t, "register", body)

	rows := pushRows(t, pool, pushTestEndpointA)
	if len(rows) != 1 || rows[0].accountID != accID || rows[0].sessionID != sessID || rows[0].host != "fcm.googleapis.com" {
		t.Fatalf("stored rows = %+v, want one for account %d session %s on fcm.googleapis.com", rows, accID, sessID)
	}
	if !pushStatus(t, client, srv, pushTestEndpointA) {
		t.Fatal("a registered device reads as not registered")
	}

	// A second register of the same endpoint keeps one row.
	resp = meSend(t, client, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, pushTestEndpointA))
	meWant(t, "register again", resp, http.StatusNoContent)
	if rows := pushRows(t, pool, pushTestEndpointA); len(rows) != 1 {
		t.Fatalf("a second register left %d rows, want 1", len(rows))
	}

	resp = meSend(t, client, http.MethodPost, srv.URL+"/api/me/push/unsubscribe", pushEndpointBody(pushTestEndpointA))
	meWant(t, "unsubscribe", resp, http.StatusNoContent)
	if rows := pushRows(t, pool, pushTestEndpointA); len(rows) != 0 {
		t.Fatalf("unsubscribe left %d rows", len(rows))
	}
	if pushStatus(t, client, srv, pushTestEndpointA) {
		t.Fatal("a removed device reads as registered")
	}
	// Removing a device that is not there is not an error.
	resp = meSend(t, client, http.MethodPost, srv.URL+"/api/me/push/unsubscribe", pushEndpointBody(pushTestEndpointA))
	meWant(t, "unsubscribe again", resp, http.StatusNoContent)

	// The refusals below run as a second account, because every call to the
	// write routes spends the caller's budget of five, refused or not.
	refused := newJarClient(t)
	verifySession(t, refused, srv, mailer, "push-routes-refused@example.com")

	// An unknown host is refused with the exact message and stores nothing.
	resp = meSend(t, refused, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, "https://evil.example.com/send/abc"))
	body = meWant(t, "unknown host", resp, http.StatusBadRequest)
	var errBody struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &errBody); err != nil {
		t.Fatalf("error body %q: %v", body, err)
	}
	if errBody.Error.Field != "endpoint" || errBody.Error.Message != "This browser's notification service is not supported." {
		t.Fatalf("unknown host error = %+v", errBody.Error)
	}
	pushNoSecrets(t, "unknown host", body)

	// A malformed endpoint and bad keys are refused.
	resp = meSend(t, refused, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, "http://fcm.googleapis.com/fcm/send/x"))
	body = meWant(t, "plain http", resp, http.StatusBadRequest)
	if !strings.Contains(body, "This device's notification details are not valid.") {
		t.Fatalf("plain http body = %s", body)
	}
	badKeys := `{"endpoint":"` + pushTestEndpointA + `","keys":{"p256dh":"AAAA","auth":"AAAA"}}`
	resp = meSend(t, refused, http.MethodPost, srv.URL+"/api/me/push/subscription", badKeys)
	body = meWant(t, "bad keys", resp, http.StatusBadRequest)
	if !strings.Contains(body, `"keys"`) {
		t.Fatalf("bad keys body = %s", body)
	}
	pushNoSecrets(t, "bad keys", body)
	if rows := pushRows(t, pool, pushTestEndpointA); len(rows) != 0 {
		t.Fatalf("a refused subscription stored %d rows", len(rows))
	}

	// A status question without an endpoint is a 400.
	resp = meSend(t, client, http.MethodPost, srv.URL+"/api/me/push/status", `{}`)
	meWant(t, "status without endpoint", resp, http.StatusBadRequest)
}

func TestPushSubscriptionMovesBetweenAccounts(t *testing.T) {
	srv, mailer, pool := newPushServer(t)
	a := newJarClient(t)
	verifySession(t, a, srv, mailer, "push-move-a@example.com")
	b := newJarClient(t)
	verifySession(t, b, srv, mailer, "push-move-b@example.com")
	idA, _ := pushAccountAndSession(t, pool, "push-move-a@example.com")
	idB, sessB := pushAccountAndSession(t, pool, "push-move-b@example.com")

	meWant(t, "A registers", meSend(t, a, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, pushTestEndpointA)), http.StatusNoContent)
	if rows := pushRows(t, pool, pushTestEndpointA); len(rows) != 1 || rows[0].accountID != idA {
		t.Fatalf("after A registered: %+v", rows)
	}

	// B signs in on the same phone and registers the same endpoint.
	meWant(t, "B registers", meSend(t, b, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, pushTestEndpointA)), http.StatusNoContent)
	rows := pushRows(t, pool, pushTestEndpointA)
	if len(rows) != 1 || rows[0].accountID != idB || rows[0].sessionID != sessB {
		t.Fatalf("after B registered: %+v, want one row for account %d", rows, idB)
	}
	if pushStatus(t, a, srv, pushTestEndpointA) {
		t.Fatal("account A still reads the moved endpoint as registered")
	}
	if !pushStatus(t, b, srv, pushTestEndpointA) {
		t.Fatal("account B does not read the endpoint as registered")
	}

	// A removing the endpoint it no longer holds changes nothing for B.
	meWant(t, "A unsubscribes", meSend(t, a, http.MethodPost, srv.URL+"/api/me/push/unsubscribe", pushEndpointBody(pushTestEndpointA)), http.StatusNoContent)
	if rows := pushRows(t, pool, pushTestEndpointA); len(rows) != 1 || rows[0].accountID != idB {
		t.Fatalf("A's unsubscribe touched B's row: %+v", rows)
	}
}

func TestPushRoutesNeedAnAccount(t *testing.T) {
	srv, _, pool := newPushServer(t)
	anon := newJarClient(t)
	for _, tc := range []struct{ path, body string }{
		{"/api/me/push/subscription", pushSubBody(t, pushTestEndpointA)},
		{"/api/me/push/status", pushEndpointBody(pushTestEndpointA)},
		{"/api/me/push/unsubscribe", pushEndpointBody(pushTestEndpointA)},
	} {
		resp := meSend(t, anon, http.MethodPost, srv.URL+tc.path, tc.body)
		body := meWant(t, tc.path, resp, http.StatusUnauthorized)
		pushNoSecrets(t, tc.path, body)
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM push_subscriptions`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("an unverified caller stored %d rows (err %v)", n, err)
	}
}

func TestPushRoutesRateLimited(t *testing.T) {
	srv, mailer, _ := newPushServer(t)
	a := newJarClient(t)
	verifySession(t, a, srv, mailer, "push-limit-a@example.com")
	b := newJarClient(t)
	verifySession(t, b, srv, mailer, "push-limit-b@example.com")

	// Five registers at once are allowed, the sixth inside a minute is not.
	for i := 0; i < 5; i++ {
		meWant(t, "register in budget", meSend(t, a, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, pushTestEndpointA)), http.StatusNoContent)
	}
	resp := meSend(t, a, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, pushTestEndpointA))
	body := meWant(t, "sixth register", resp, http.StatusTooManyRequests)
	pushNoSecrets(t, "429", body)
	// Removing a device spends the same budget.
	meWant(t, "unsubscribe over budget", meSend(t, a, http.MethodPost, srv.URL+"/api/me/push/unsubscribe", pushEndpointBody(pushTestEndpointA)), http.StatusTooManyRequests)

	// Another account has its own budget.
	meWant(t, "other account register", meSend(t, b, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, pushTestEndpointB)), http.StatusNoContent)

	// The status question has its own, larger budget: ten, then refused.
	for i := 0; i < 10; i++ {
		meWant(t, "status in budget", meSend(t, b, http.MethodPost, srv.URL+"/api/me/push/status", pushEndpointBody(pushTestEndpointB)), http.StatusOK)
	}
	meWant(t, "eleventh status", meSend(t, b, http.MethodPost, srv.URL+"/api/me/push/status", pushEndpointBody(pushTestEndpointB)), http.StatusTooManyRequests)
	// The status budget does not touch the write budget.
	meWant(t, "register after status spent", meSend(t, b, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, pushTestEndpointB)), http.StatusNoContent)
}

func TestPushRegisterWithoutKeyIs503(t *testing.T) {
	// The default harness leaves PageConfig.PushPublicKey empty.
	srv, mailer, pool := newE2EServer(t)
	client := newJarClient(t)
	verifySession(t, client, srv, mailer, "push-nokey@example.com")

	resp := meSend(t, client, http.MethodPost, srv.URL+"/api/me/push/subscription", pushSubBody(t, pushTestEndpointA))
	body := meWant(t, "register without a key", resp, http.StatusServiceUnavailable)
	if !strings.Contains(body, "Notifications are not available on this server.") {
		t.Fatalf("503 body = %s", body)
	}
	pushNoSecrets(t, "503", body)
	if rows := pushRows(t, pool, pushTestEndpointA); len(rows) != 0 {
		t.Fatalf("a 503 stored %d rows", len(rows))
	}
}

func TestPushRoutesStrictJSON(t *testing.T) {
	srv, mailer, pool := newPushServer(t)
	client := newJarClient(t)
	verifySession(t, client, srv, mailer, "push-strict@example.com")
	// A second account takes the later requests, because every request to the
	// write routes spends the caller's budget of five, refused or not.
	client2 := newJarClient(t)
	verifySession(t, client2, srv, mailer, "push-strict-2@example.com")
	url := srv.URL + "/api/me/push/subscription"

	// An unknown field is refused.
	p, a := pushKeys()
	unknown := `{"endpoint":"` + pushTestEndpointA + `","keys":{"p256dh":"` + p + `","auth":"` + a + `"},"account_id":1}`
	body := meWant(t, "unknown field", meSend(t, client, http.MethodPost, url, unknown), http.StatusBadRequest)
	pushNoSecrets(t, "unknown field", body)
	unknownInner := `{"endpoint":"` + pushTestEndpointA + `","keys":{"p256dh":"` + p + `","auth":"` + a + `","extra":1}}`
	meWant(t, "unknown inner field", meSend(t, client, http.MethodPost, url, unknownInner), http.StatusBadRequest)

	// A body that is not JSON, and one with trailing data, are refused.
	meWant(t, "not json", meSend(t, client, http.MethodPost, url, "endpoint=x"), http.StatusBadRequest)
	meWant(t, "trailing data", meSend(t, client, http.MethodPost, url, pushSubBody(t, pushTestEndpointA)+`{}`), http.StatusBadRequest)

	// Another content type is refused before the body is read.
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(pushSubBody(t, pushTestEndpointA)))
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST text/plain: %v", err)
	}
	body = meWant(t, "text/plain", resp, http.StatusUnsupportedMediaType)
	pushNoSecrets(t, "415", body)
	// No content type at all is refused the same way.
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/me/push/status", strings.NewReader(pushEndpointBody(pushTestEndpointA)))
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST without content type: %v", err)
	}
	meWant(t, "no content type", resp, http.StatusUnsupportedMediaType)
	// A charset parameter is fine.
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/me/push/status", strings.NewReader(pushEndpointBody(pushTestEndpointA)))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST with charset: %v", err)
	}
	meWant(t, "json with charset", resp, http.StatusOK)

	// A 5 KB body is over the cap.
	big := `{"endpoint":"` + strings.Repeat("a", 5*1024) + `"}`
	resp = meSend(t, client2, http.MethodPost, srv.URL+"/api/me/push/unsubscribe", big)
	got := resp.StatusCode
	body = meBody(t, resp)
	if got != http.StatusRequestEntityTooLarge && got != http.StatusBadRequest {
		t.Fatalf("5 KB body status = %d, want 413 or 400, body %s", got, body)
	}
	if strings.Contains(body, strings.Repeat("a", 64)) {
		t.Fatalf("the 5 KB refusal echoes the body: %.200s", body)
	}

	// An endpoint over the policy length (but under the body cap) is refused
	// by the policy and echoed nowhere.
	long := "https://fcm.googleapis.com/fcm/send/" + strings.Repeat("b", 2100)
	body = meWant(t, "long endpoint", meSend(t, client2, http.MethodPost, url, pushSubBody(t, long)), http.StatusBadRequest)
	if strings.Contains(body, "bbbbbbbbbb") {
		t.Fatalf("the long endpoint refusal echoes the endpoint: %.200s", body)
	}

	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM push_subscriptions`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("refused requests stored %d rows (err %v)", n, err)
	}
}
