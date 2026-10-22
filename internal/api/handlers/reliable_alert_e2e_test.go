// reliable_alert_e2e_test.go is plan 03.2-17: the Reliable reporter alert
// (NOTIF-05, D-26 as amended 2026-10-18, D-29) proven end to end over the routes
// a browser uses, which are GET /api/updates, PATCH /api/me/alerts, PATCH
// /api/me/profile, POST /api/reports/{id}/mute and the vote routes, on a real
// Postgres. A seeded Reliable reporter (testutil.SeedReliableReporter) supplies
// real history. It runs only on a disposable database (p32_17). Every package
// level identifier here carries the p17ra prefix.
//
// The updates and account services read the wall clock and the router gives a
// test no way to move it, so the report and voting services get a clock that is
// the wall clock plus a settable offset. "The clock moves forward" is that
// offset growing, which keeps a report posted after the switch went on later
// than the switch's own stamp, and a report posted before it earlier. "An hour
// has passed" for the per reporter cap is made by moving the stored alert rows
// two hours back (p17raAgeAlerts), the same device updates_e2e_test.go uses for
// the cursors.
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// p17raSecret is the session secret newE2EServerWith signs cookies with.
const p17raSecret = "e2e-test-secret-not-for-production"

const (
	p17raAlertEvent = "Reported nearby by a Reliable reporter."
	p17raUnconfirm  = "Not confirmed yet."
)

// p17raClock is the wall clock plus an offset the test grows.
type p17raClock struct{ offset atomic.Int64 }

func (c *p17raClock) Now() time.Time {
	return time.Now().UTC().Add(time.Duration(c.offset.Load()))
}

func (c *p17raClock) Advance(d time.Duration) { c.offset.Add(int64(d)) }

// p17raWorld is one server with a Reliable reporter R and a verified reader A.
type p17raWorld struct {
	srv    *httptest.Server
	pool   *pgxpool.Pool
	clock  *p17raClock
	mailer *recordingMailer
	r      *http.Client
	a      *http.Client
	emailA string
	emailR string
}

// p17raSessionClient builds a client whose cookie jar holds the signed cookie
// for sessionID, so a seeded session can act through the real router.
func p17raSessionClient(t *testing.T, baseURL, sessionID string) *http.Client {
	t.Helper()
	mgr, err := session.NewManager([]byte(p17raSecret), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	jar.SetCookies(u, []*http.Cookie{{Name: "pinalert_session", Value: mgr.Sign(sessionID), Path: "/"}})
	return &http.Client{Jar: jar}
}

// p17raSetup starts a server whose report and voting services share the offset
// clock, seeds the Reliable reporter R and verifies the reader A.
func p17raSetup(t *testing.T, prefix string) *p17raWorld {
	t.Helper()
	clock := &p17raClock{}
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		d.Reports = service.NewReportService(q, service.WithReportClock(clock.Now))
		d.Votes = service.NewVotingService(q, service.WithVotingClock(clock.Now))
		d.RequestLinkRateLimit = api.RequestLinkRateLimit{Burst: 100, Every: time.Millisecond}
	})
	w := &p17raWorld{
		srv: srv, pool: pool, clock: clock, mailer: mailer,
		emailA: prefix + "-a@example.com",
		emailR: prefix + "-r@example.com",
	}
	const sessionID = "p17ra-reporter-session"
	testutil.SeedReliableReporter(t, pool, w.emailR, sessionID+"-"+prefix, clock.Now())
	w.r = p17raSessionClient(t, srv.URL, sessionID+"-"+prefix)
	w.a = newVerifiedClient(t, srv, mailer, w.emailA)
	return w
}

// p17raHome saves the home area of client at the report point and requires 200.
func p17raHome(t *testing.T, w *p17raWorld, client *http.Client) {
	t.Helper()
	body := `{"home_area":{"name":"Central Bengaluru","lat":` + strconv.FormatFloat(p07ReportLat, 'f', -1, 64) +
		`,"lon":` + strconv.FormatFloat(p07ReportLon, 'f', -1, 64) + `}}`
	meWant(t, "set home area", meSend(t, client, http.MethodPatch, w.srv.URL+"/api/me/profile", body), http.StatusOK)
}

// p17raAlerts PATCHes /api/me/alerts for client and requires 200.
func p17raAlerts(t *testing.T, w *p17raWorld, client *http.Client, body string) {
	t.Helper()
	meWant(t, "PATCH /api/me/alerts "+body, meSend(t, client, http.MethodPatch, w.srv.URL+"/api/me/alerts", body), http.StatusOK)
}

// p17raOptIn gives client a home area and switches the alert on at 5 km.
func p17raOptIn(t *testing.T, w *p17raWorld, client *http.Client) {
	t.Helper()
	p17raHome(t, w, client)
	p17raAlerts(t, w, client, `{"reliable_alerts":true,"alert_radius_km":5}`)
}

// p17raPost files a report of the given category and severity as client.
func p17raPost(t *testing.T, w *p17raWorld, client *http.Client, category, severity string) int64 {
	t.Helper()
	body := floodReportBody(p07ReportLat, p07ReportLon)
	body["category"] = category
	body["severity"] = severity
	return submitReportGetID(t, client, w.srv.URL, body)
}

// p17raVote casts a vote from the i-th voter cell and returns the visibility
// the vote answer names.
func p17raVote(t *testing.T, w *p17raWorld, client *http.Client, id int64, action string, cell int) string {
	t.Helper()
	status, raw, err := p07PostVoteRaw(client, w.srv.URL, id, action, p32uCells[cell][0], p32uCells[cell][1])
	if err != nil || status != http.StatusOK {
		t.Fatalf("%s on report %d from cell %d: status %d, err %v, body %s", action, id, cell, status, err, raw)
	}
	var out struct {
		Visibility string `json:"visibility"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the vote answer %s: %v", raw, err)
	}
	return out.Visibility
}

// p17raAgeAlerts moves every Reliable alert row of the reader two hours back,
// which is "an hour has passed" for the per reporter cap.
func p17raAgeAlerts(t *testing.T, w *p17raWorld) {
	t.Helper()
	testutil.MustExec(t, w.pool,
		`UPDATE updates SET created_at = created_at - interval '2 hours'
		 WHERE kind = 'reliable_nearby'
		   AND account_id = (SELECT id FROM accounts WHERE email = $1)`, w.emailA)
}

// p17raCountAlerts counts the stored Reliable alert rows of the reader.
func p17raCountAlerts(t *testing.T, w *p17raWorld) int {
	t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM updates WHERE kind = 'reliable_nearby'
		   AND account_id = (SELECT id FROM accounts WHERE email = $1)`, w.emailA).Scan(&n); err != nil {
		t.Fatalf("counting the reliable alert rows: %v", err)
	}
	return n
}

// p17raWantAlert requires row to be the Reliable alert for report id.
func p17raWantAlert(t *testing.T, label string, row p32uRow, id int64) {
	t.Helper()
	if row.Kind != "reliable_nearby" || row.Event != p17raAlertEvent || !row.Available {
		t.Errorf("%s: row = %+v, want an available reliable_nearby row reading %q", label, row, p17raAlertEvent)
	}
	if row.ReportID == nil || *row.ReportID != id {
		t.Errorf("%s: report_id = %v, want %d", label, row.ReportID, id)
	}
}

// p17raWantNoAlert requires the bell to hold nothing at all.
func p17raWantNoAlert(t *testing.T, label string, w *p17raWorld, client *http.Client) {
	t.Helper()
	got, raw := p32uPoll(t, label, client, w.srv.URL)
	if len(got.Updates) != 0 || got.Unread != 0 {
		t.Fatalf("%s: bell = %s, want nothing", label, raw)
	}
}

// TestReliableAlertOverTheBell: an opted in reader with a home area sees one
// update for an unconfirmed critical report by a Reliable reporter, a second
// poll writes no second row, and a report posted before the switch went on
// never alerts (D-26 as amended 2026-10-18, NOTIF-05).
func TestReliableAlertOverTheBell(t *testing.T) {
	w := p17raSetup(t, "p17ra-bell")

	// Posted before the switch: it must never alert.
	before := p17raPost(t, w, w.r, "flood", "low")

	p17raOptIn(t, w, w.a)
	w.clock.Advance(time.Minute)

	critical := p17raPost(t, w, w.r, "flood", "critical")

	got, raw := p32uPoll(t, "A first poll", w.a, w.srv.URL)
	if len(got.Updates) != 1 || got.Unread != 1 {
		t.Fatalf("first poll = %s, want one unread row", raw)
	}
	row := got.Updates[0]
	p17raWantAlert(t, "first poll", row, critical)
	if row.Trust != p17raUnconfirm {
		t.Errorf("trust = %q, want %q", row.Trust, p17raUnconfirm)
	}
	if !row.Unread {
		t.Errorf("a new alert reads as read: %+v", row)
	}
	if row.ReportID != nil && *row.ReportID == before {
		t.Errorf("the report posted before the switch alerted")
	}
	for _, banned := range []string{"p17ra-bell-r@example.com", "session"} {
		if strings.Contains(strings.ToLower(string(raw)), banned) {
			t.Errorf("the answer names %q: %s", banned, raw)
		}
	}

	again, rawAgain := p32uPoll(t, "A second poll", w.a, w.srv.URL)
	if len(again.Updates) != 1 || again.Updates[0].ID != row.ID {
		t.Fatalf("second poll = %s, want the same single row", rawAgain)
	}
	if n := p17raCountAlerts(t, w); n != 1 {
		t.Errorf("stored alert rows = %d, want 1", n)
	}
}

// TestReliableAlertAtPostingWhenNotCritical: a low report by a Reliable
// reporter alerts at posting while it is Provisional and reads Not confirmed
// yet; the same row reads the confirmed sentence once people confirm it, with no
// second row; a later report that two readers dispute is Hidden and never alerts
// (owner amendment 2026-10-18: listable means Live or Provisional, never Hidden).
func TestReliableAlertAtPostingWhenNotCritical(t *testing.T) {
	w := p17raSetup(t, "p17ra-low")
	b := newVerifiedClient(t, w.srv, w.mailer, "p17ra-low-b@example.com")
	c := newVerifiedClient(t, w.srv, w.mailer, "p17ra-low-c@example.com")
	d := newVerifiedClient(t, w.srv, w.mailer, "p17ra-low-d@example.com")

	p17raOptIn(t, w, w.a)
	w.clock.Advance(time.Minute)

	low := p17raPost(t, w, w.r, "flood", "low")
	got, raw := p32uPoll(t, "A at posting", w.a, w.srv.URL)
	if len(got.Updates) != 1 {
		t.Fatalf("at posting = %s, want one alert row", raw)
	}
	first := got.Updates[0]
	p17raWantAlert(t, "at posting", first, low)
	if first.Trust != p17raUnconfirm {
		t.Errorf("trust at posting = %q, want %q", first.Trust, p17raUnconfirm)
	}

	// Two readers confirm from distinct cells, then a third, until the row reads
	// the confirmed sentence. The same row changes its words; none is added.
	p17raVote(t, w, b, low, "confirm", 0)
	p17raVote(t, w, c, low, "confirm", 1)
	after, rawAfter := p32uPoll(t, "A after confirmations", w.a, w.srv.URL)
	if len(after.Updates) != 1 || after.Updates[0].ID != first.ID {
		t.Fatalf("after confirmations = %s, want the same single row (id %d)", rawAfter, first.ID)
	}
	if !strings.HasPrefix(after.Updates[0].Trust, "Confirmed by") {
		t.Errorf("trust after confirmations = %q, want the confirmed sentence", after.Updates[0].Trust)
	}
	if n := p17raCountAlerts(t, w); n != 1 {
		t.Errorf("stored alert rows = %d, want 1", n)
	}

	// An hour later R posts another low report and two readers at distinct cells
	// dispute it before A polls again: Hidden, so it never alerts.
	p17raAgeAlerts(t, w)
	w.clock.Advance(time.Minute)
	second := p17raPost(t, w, w.r, "flood", "low")
	p17raVote(t, w, c, second, "dispute", 1)
	if vis := p17raVote(t, w, d, second, "dispute", 2); vis != "hidden" {
		t.Fatalf("report %d visibility after two disputes = %q, want hidden", second, vis)
	}
	hiddenPoll, rawHidden := p32uPoll(t, "A after the hidden report", w.a, w.srv.URL)
	if len(hiddenPoll.Updates) != 1 || hiddenPoll.Updates[0].ID != first.ID {
		t.Fatalf("after the hidden report = %s, want only the first row", rawHidden)
	}
	if n := p17raCountAlerts(t, w); n != 1 {
		t.Errorf("stored alert rows after the hidden report = %d, want 1", n)
	}
}

// TestReliableAlertCapPerReporter: two critical reports by one Reliable reporter
// inside an hour give one alert; once the hour has passed the second alerts
// while it is still unexpired.
func TestReliableAlertCapPerReporter(t *testing.T) {
	w := p17raSetup(t, "p17ra-cap")

	p17raOptIn(t, w, w.a)
	w.clock.Advance(time.Minute)

	first := p17raPost(t, w, w.r, "flood", "critical")
	w.clock.Advance(time.Minute)
	second := p17raPost(t, w, w.r, "flood", "critical")

	got, raw := p32uPoll(t, "A inside the hour", w.a, w.srv.URL)
	if len(got.Updates) != 1 {
		t.Fatalf("inside the hour = %s, want one alert", raw)
	}
	p17raWantAlert(t, "inside the hour", got.Updates[0], first)
	again, rawAgain := p32uPoll(t, "A inside the hour again", w.a, w.srv.URL)
	if len(again.Updates) != 1 {
		t.Fatalf("a second poll inside the hour = %s, want still one alert", rawAgain)
	}

	p17raAgeAlerts(t, w)
	later, rawLater := p32uPoll(t, "A after the hour", w.a, w.srv.URL)
	if len(later.Updates) != 2 {
		t.Fatalf("after the hour = %s, want two alerts", rawLater)
	}
	newest := later.Updates[0]
	p17raWantAlert(t, "after the hour, newest", newest, second)
	if newest.Trust != p17raUnconfirm {
		t.Errorf("trust of the second alert = %q, want %q", newest.Trust, p17raUnconfirm)
	}
	p17raWantAlert(t, "after the hour, older", later.Updates[1], first)
	if n := p17raCountAlerts(t, w); n != 2 {
		t.Errorf("stored alert rows = %d, want 2", n)
	}
}

// TestReliableAlertSkipsOwnMutedAndOld: a report posted before the switch went
// on never alerts, a report the reader muted never alerts, and the reporter's
// own report never alerts the reporter. A later report alerts the reader, which
// shows the quiet cases are the rule and not a broken setup.
func TestReliableAlertSkipsOwnMutedAndOld(t *testing.T) {
	w := p17raSetup(t, "p17ra-skip")

	old := p17raPost(t, w, w.r, "flood", "low")
	p17raOptIn(t, w, w.a)
	w.clock.Advance(time.Minute)

	muted := p17raPost(t, w, w.r, "flood", "critical")
	resp := p32uPost(t, w.a, w.srv.URL+"/api/reports/"+strconv.FormatInt(muted, 10)+"/mute", "")
	p32uStatus(t, "A mutes the report", resp, http.StatusNoContent)

	// R is also a reader with a home area and the switch on; its own reports
	// never reach its own bell.
	p17raOptIn(t, w, w.r)
	w.clock.Advance(time.Minute)

	p17raWantNoAlert(t, "A with an old and a muted report", w, w.a)
	p17raWantNoAlert(t, "R with its own reports", w, w.r)

	// Control: a report posted after the mute, from the same reporter, alerts A
	// once the per reporter hour is not in the way (nothing was sent yet).
	fresh := p17raPost(t, w, w.r, "flood", "critical")
	got, raw := p32uPoll(t, "A after a fresh report", w.a, w.srv.URL)
	if len(got.Updates) != 1 {
		t.Fatalf("after a fresh report = %s, want exactly one alert", raw)
	}
	p17raWantAlert(t, "after a fresh report", got.Updates[0], fresh)
	if got.Updates[0].ReportID != nil && (*got.Updates[0].ReportID == old || *got.Updates[0].ReportID == muted) {
		t.Errorf("the alert is for the old or the muted report: %s", raw)
	}
	p17raWantNoAlert(t, "R after its fresh report", w, w.r)
}

// TestReliableAlertStopsWhenSwitchedOff: turning the switch off through PATCH
// /api/me/alerts stops new alerts, and removing the home area leaves the switch
// off in the database (D-26, D-29).
func TestReliableAlertStopsWhenSwitchedOff(t *testing.T) {
	w := p17raSetup(t, "p17ra-off")

	p17raOptIn(t, w, w.a)
	w.clock.Advance(time.Minute)
	p17raAlerts(t, w, w.a, `{"reliable_alerts":false}`)

	p17raPost(t, w, w.r, "flood", "critical")
	p17raWantNoAlert(t, "A with the switch off", w, w.a)
	if n := p17raCountAlerts(t, w); n != 0 {
		t.Errorf("stored alert rows with the switch off = %d, want 0", n)
	}

	// Turn it on again, then remove the home area: the switch goes off with it
	// and a new report still gives nothing.
	p17raAlerts(t, w, w.a, `{"reliable_alerts":true}`)
	if r := p13alRead(t, w.pool, w.emailA); !r.found || !r.on {
		t.Fatalf("stored after turning on again = %+v, want on", r)
	}
	meWant(t, "clear home area", meSend(t, w.a, http.MethodPatch, w.srv.URL+"/api/me/profile", `{"home_area":null}`), http.StatusOK)
	if r := p13alRead(t, w.pool, w.emailA); !r.found || r.on || r.enabledAt != nil {
		t.Errorf("stored after clearing the home area = %+v, want off with no enabled_at", r)
	}
	w.clock.Advance(time.Minute)
	p17raPost(t, w, w.r, "flood", "critical")
	p17raWantNoAlert(t, "A with no home area", w, w.a)
}
