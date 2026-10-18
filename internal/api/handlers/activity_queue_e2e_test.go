// activity_queue_e2e_test.go - 04-25: the Activity page carries what the saved
// reports strip needs. The account id is the verified account's own, rendered
// as data-account-id on the page and never the email (T-04-14), and the strip
// container, the announcer and queue.js are present whether or not the author
// has any report on the server (D-20). Driven against the real router over a
// real Postgres through the existing harness.
package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
)

// p25AccountID reads the numeric id of the account registered under email.
func p25AccountID(t *testing.T, pool *pgxpool.Pool, email string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), `SELECT id FROM accounts WHERE email = $1`, email).Scan(&id); err != nil {
		t.Fatalf("reading the account id for %s: %v", email, err)
	}
	return id
}

// p25OpeningTag returns the text from marker to the next closing angle
// bracket. It fails the test when marker is absent, so no check passes on an
// empty string.
func p25OpeningTag(t *testing.T, body, marker string) string {
	t.Helper()
	idx := strings.Index(body, marker)
	if idx == -1 {
		t.Fatalf("could not find %q in the page", marker)
	}
	end := strings.Index(body[idx:], ">")
	if end == -1 {
		t.Fatalf("no closing bracket after %q", marker)
	}
	return body[idx : idx+end+1]
}

// p25SecondDevice returns a client with a fresh session bound to the account
// id, the same recipe TestProfileListsOwnReportsAcrossSessions uses.
func p25SecondDevice(t *testing.T, srv *httptest.Server, pool *pgxpool.Pool, id int64) *http.Client {
	t.Helper()
	device := newJarClient(t)
	resp, err := device.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login (issuing the second device's session cookie): %v", err)
	}
	resp.Body.Close()
	sessionID := sessionIDFromJar(t, device, srv.URL)
	_, err = sqlcgen.New(pool).BindSessionAccount(context.Background(), sqlcgen.BindSessionAccountParams{SessionID: sessionID, AccountID: &id})
	if err != nil {
		t.Fatalf("binding the second device to the account: %v", err)
	}
	return device
}

// TestActivityRendersOwnAccountID proves the Activity page renders the
// verified session's own account id on .profile-page: different per account,
// the same for two sessions of one account, and never the email.
func TestActivityRendersOwnAccountID(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)

	clientA := newVerifiedClient(t, srv, mailer, "p25-a@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "p25-b@example.com")
	idA := p25AccountID(t, pool, "p25-a@example.com")
	idB := p25AccountID(t, pool, "p25-b@example.com")
	if idA == idB {
		t.Fatalf("two accounts share the id %d", idA)
	}
	wantA := `data-account-id="` + strconv.FormatInt(idA, 10) + `"`
	wantB := `data-account-id="` + strconv.FormatInt(idB, 10) + `"`

	bodyA := getProfile(t, clientA, srv.URL)
	if n := strings.Count(bodyA, "data-account-id="); n != 1 {
		t.Fatalf("account A's page has %d data-account-id attributes, want 1", n)
	}
	tagA := p25OpeningTag(t, bodyA, `<main class="profile-page"`)
	if !strings.Contains(tagA, wantA) {
		t.Errorf("account A's .profile-page tag %q does not carry %s", tagA, wantA)
	}
	if strings.Contains(tagA, "@") {
		t.Errorf("account A's .profile-page tag %q holds an email address", tagA)
	}

	bodyB := getProfile(t, clientB, srv.URL)
	tagB := p25OpeningTag(t, bodyB, `<main class="profile-page"`)
	if !strings.Contains(tagB, wantB) {
		t.Errorf("account B's .profile-page tag %q does not carry %s", tagB, wantB)
	}
	if strings.Contains(bodyB, wantA) {
		t.Errorf("account B's page carries account A's id attribute %s", wantA)
	}

	// A second session of account A shows the same id: the key is the
	// account, not the session.
	deviceA := p25SecondDevice(t, srv, pool, idA)
	tagDevice := p25OpeningTag(t, getProfile(t, deviceA, srv.URL), `<main class="profile-page"`)
	if !strings.Contains(tagDevice, wantA) {
		t.Errorf("second session of account A rendered %q, want %s", tagDevice, wantA)
	}
}

// TestActivityQueueMarkupInBothBranches proves the hidden strip container, the
// body level announcer and queue.js are on the page both when the author has
// no report on the server and when the list is drawn.
func TestActivityQueueMarkupInBothBranches(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "p25-markup@example.com")

	check := func(name, body, after string) {
		t.Helper()
		if n := strings.Count(body, `id="queue-strip"`); n != 1 {
			t.Fatalf("%s: id=\"queue-strip\" appears %d times, want 1", name, n)
		}
		if tag := p25OpeningTag(t, body, `<div class="queue-strip" id="queue-strip"`); !strings.Contains(tag, " hidden") {
			t.Errorf("%s: the strip container %q is not hidden", name, tag)
		}
		strip := strings.Index(body, `id="queue-strip"`)
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to
		// D-21): the page heading is the h1 My activity. The Phase 3.1 and Phase 4
		// hands-on tests are walked on this layout after Phase 3.4 is built.
		heading := strings.Index(body, `activity-heading">My activity</h1>`)
		if heading == -1 || heading > strip {
			t.Errorf("%s: the strip does not come after the My activity heading", name)
		}
		next := strings.Index(body, after)
		if next == -1 || strip > next {
			t.Errorf("%s: the strip does not come before %q", name, after)
		}
		if n := strings.Count(body, `id="notice-live"`); n != 1 {
			t.Fatalf("%s: id=\"notice-live\" appears %d times, want 1", name, n)
		}
		live := strings.Index(body, `id="notice-live"`)
		chrome := strings.Index(body, `class="top-chrome"`)
		main := strings.Index(body, `<main class="profile-page"`)
		if chrome == -1 || main == -1 || live < chrome || live > main {
			t.Errorf("%s: #notice-live is not between .top-chrome and main", name)
		}
		if n := strings.Count(body, "/static/js/queue.js"); n != 1 {
			t.Fatalf("%s: queue.js appears %d times, want 1", name, n)
		}
		queue := strings.Index(body, "/static/js/queue.js")
		activity := strings.Index(body, "/static/js/activity.js")
		if activity == -1 || queue > activity {
			t.Errorf("%s: queue.js does not load before activity.js", name)
		}
	}

	check("empty branch", getProfile(t, client, srv.URL), "profile-empty-state")

	submitReport(t, client, srv, "flood", "Water rising near the market bridge")
	check("list branch", getProfile(t, client, srv.URL), "profile-report-list")
}
