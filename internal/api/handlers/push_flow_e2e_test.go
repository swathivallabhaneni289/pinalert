// push_flow_e2e_test.go is plan 03.3-15: the whole of Phase 3.3 proven end to
// end over the routes a browser uses (POST /api/reports, the vote routes,
// POST /api/me/push/subscription, PATCH /api/me/profile, PATCH /api/me/alerts,
// POST /api/reports/{id}/mute and POST /auth/logout) on a real Postgres, with
// the real dispatcher and the real worker wired to a fake push service. No real
// push service is contacted.
//
// The test drives the pipeline by hand so every step is deterministic: the
// router's report and voting services carry a recording notifier, which keeps
// each poke and forwards it to the real dispatcher; after each action the test
// hands every recorded poke to PushDispatcher.EvaluateReport, exactly as
// PushDispatcher.Run would, and then runs one PushWorker.SweepOnce. All clocks
// are the wall clock, because the dispatcher stamps next_attempt_at with its own
// clock and the worker claims against the database clock; a clock ahead of the
// database would leave every row not yet due.
//
// It runs only on a disposable database (p33_15_*). Every package level
// identifier here carries the pw prefix. Endpoints and keys are throwaway.
package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/api"
	"pinalert/internal/push"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	// pwAreaName is what the fake area namer answers. It differs from every
	// home area name used here, so a leak of either can be told apart.
	pwAreaName = "Central Market"
	pwHomeName = "Home Test District"

	pwUnconfirmed = "Not confirmed yet."
	pwConfirmed   = "Confirmed by people nearby."
	pwFromHome    = " from your home area."
)

// pwPoke is one recorded "this report changed" message.
type pwPoke struct{ report, actor int64 }

// pwNotifier is the recording wrapper between the services and the dispatcher.
// It keeps every poke and forwards it to the real dispatcher's non blocking
// ReportChanged, so the path from a post or a vote to the dispatcher is the
// production one.
type pwNotifier struct {
	mu      sync.Mutex
	pending []pwPoke
	history []pwPoke
	target  service.ReportChangeNotifier
}

func (n *pwNotifier) ReportChanged(reportID, actorAccountID int64) {
	n.mu.Lock()
	p := pwPoke{report: reportID, actor: actorAccountID}
	n.pending = append(n.pending, p)
	n.history = append(n.history, p)
	target := n.target
	n.mu.Unlock()
	if target != nil {
		target.ReportChanged(reportID, actorAccountID)
	}
}

func (n *pwNotifier) setTarget(t service.ReportChangeNotifier) {
	n.mu.Lock()
	n.target = t
	n.mu.Unlock()
}

// take returns the pokes not yet evaluated and forgets them.
func (n *pwNotifier) take() []pwPoke {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := n.pending
	n.pending = nil
	return out
}

// all returns every poke ever recorded.
func (n *pwNotifier) all() []pwPoke {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]pwPoke(nil), n.history...)
}

// pwSent is one message the fake push service accepted.
type pwSent struct {
	endpoint string
	msg      push.Message
}

// pwSender is the fake push service. It answers 201 to everything and records
// each message. fanOut calls it from several goroutines, hence the mutex.
type pwSender struct {
	mu   sync.Mutex
	sent []pwSent
}

func (s *pwSender) Send(_ context.Context, sub push.Subscription, msg push.Message) (push.Result, error) {
	s.mu.Lock()
	s.sent = append(s.sent, pwSent{endpoint: sub.Endpoint, msg: msg})
	s.mu.Unlock()
	return push.Result{Status: http.StatusCreated}, nil
}

// to returns the messages sent to one endpoint, in send order.
func (s *pwSender) to(endpoint string) []push.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []push.Message
	for _, m := range s.sent {
		if m.endpoint == endpoint {
			out = append(out, m.msg)
		}
	}
	return out
}

func (s *pwSender) total() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

func (s *pwSender) everything() []pwSent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]pwSent(nil), s.sent...)
}

// pwNamer is the fake area namer.
type pwNamer struct{ calls atomic.Int32 }

func (n *pwNamer) ReverseArea(context.Context, float64, float64) (string, error) {
	n.calls.Add(1)
	return pwAreaName, nil
}

// pwWorld is one server with the real dispatcher and worker around a fake push
// service.
type pwWorld struct {
	srv      *httptest.Server
	pool     *pgxpool.Pool
	mailer   *recordingMailer
	notifier *pwNotifier
	sender   *pwSender
	namer    *pwNamer
	disp     *service.PushDispatcher
	worker   *service.PushWorker
}

func pwSetup(t *testing.T) *pwWorld {
	t.Helper()
	w := &pwWorld{notifier: &pwNotifier{}, sender: &pwSender{}, namer: &pwNamer{}}
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		d.Page.PushPublicKey = pushTestKey
		reports := service.NewReportService(q, service.WithReportChangeNotifier(w.notifier))
		d.Reports = reports
		d.Votes = service.NewVotingService(q, service.WithVoteChangeNotifier(w.notifier))
		updates := service.NewUpdatesService(q, reports)
		w.disp = service.NewPushDispatcher(q, updates, w.namer)
		w.worker = service.NewPushWorker(q, w.sender)
	})
	w.srv, w.mailer, w.pool = srv, mailer, pool
	w.notifier.setTarget(w.disp)
	return w
}

// run hands every recorded poke to the dispatcher, as its Run loop would, and
// then lets the worker send what was queued.
func (w *pwWorld) run(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	for _, p := range w.notifier.take() {
		if err := w.disp.EvaluateReport(ctx, p.report, p.actor); err != nil {
			t.Fatalf("dispatcher, report %d actor %d: %v", p.report, p.actor, err)
		}
	}
	if err := w.worker.SweepOnce(ctx); err != nil {
		t.Fatalf("worker sweep: %v", err)
	}
}

// later runs the sweeps the server runs on timers and requires that they send
// nothing new.
func (w *pwWorld) laterSweepsSendNothing(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	before := w.sender.total()
	if err := w.disp.SweepOnce(ctx); err != nil {
		t.Fatalf("dispatcher sweep: %v", err)
	}
	if err := w.disp.ChangesSweepOnce(ctx); err != nil {
		t.Fatalf("dispatcher changes sweep: %v", err)
	}
	if err := w.worker.SweepOnce(ctx); err != nil {
		t.Fatalf("worker sweep: %v", err)
	}
	if got := w.sender.total(); got != before {
		t.Fatalf("later sweeps sent %d more messages, want 0", got-before)
	}
}

// pwReader is one verified account with a home area and a device.
type pwReader struct {
	name     string
	email    string
	id       int64
	client   *http.Client
	endpoint string
}

func pwEmail(name string) string { return "pw-" + name + "@example.com" }

// pwHomeAt saves the home area of client and requires 200.
func pwHomeAt(t *testing.T, w *pwWorld, client *http.Client, lat, lon float64) {
	t.Helper()
	body := `{"home_area":{"name":"` + pwHomeName + `","lat":` + strconv.FormatFloat(lat, 'f', -1, 64) +
		`,"lon":` + strconv.FormatFloat(lon, 'f', -1, 64) + `}}`
	meWant(t, "set home area", meSend(t, client, http.MethodPatch, w.srv.URL+"/api/me/profile", body), http.StatusOK)
}

// pwRegister registers one device of client and requires 204.
func pwRegister(t *testing.T, w *pwWorld, client *http.Client, endpoint string) {
	t.Helper()
	resp := meSend(t, client, http.MethodPost, w.srv.URL+"/api/me/push/subscription", pushSubBody(t, endpoint))
	meWant(t, "register "+endpoint, resp, http.StatusNoContent)
}

// pwAlerts PATCHes /api/me/alerts and requires 200.
func pwAlerts(t *testing.T, w *pwWorld, client *http.Client, body string) {
	t.Helper()
	meWant(t, "PATCH /api/me/alerts "+body, meSend(t, client, http.MethodPatch, w.srv.URL+"/api/me/alerts", body), http.StatusOK)
}

// pwReaderAt makes a verified reader with a home area at the given point and one
// fcm device.
func (w *pwWorld) readerAt(t *testing.T, name string, lat, lon float64) *pwReader {
	t.Helper()
	email := pwEmail(name)
	client := newVerifiedClient(t, w.srv, w.mailer, email)
	pwHomeAt(t, w, client, lat, lon)
	endpoint := "https://fcm.googleapis.com/fcm/send/pw-" + name
	pwRegister(t, w, client, endpoint)
	return &pwReader{name: name, email: email, id: revAccountID(t, w.pool, email), client: client, endpoint: endpoint}
}

// near is a reader whose home area is at the report point.
func (w *pwWorld) near(t *testing.T, name string) *pwReader {
	t.Helper()
	return w.readerAt(t, name, p07ReportLat, p07ReportLon)
}

// person is a verified account with no home area and no device.
func (w *pwWorld) person(t *testing.T, name string) *pwReader {
	t.Helper()
	email := pwEmail(name)
	client := newVerifiedClient(t, w.srv, w.mailer, email)
	return &pwReader{name: name, email: email, id: revAccountID(t, w.pool, email), client: client}
}

// pwPostCritical files a Critical report with the pick Water rising at the
// report point and returns its id.
func pwPostCritical(t *testing.T, w *pwWorld, client *http.Client, description string) int64 {
	t.Helper()
	body := floodReportBody(p07ReportLat, p07ReportLon)
	body["severity"] = "critical"
	body["happening"] = "water_rising"
	if description != "" {
		body["description"] = description
	}
	return submitReportGetID(t, client, w.srv.URL, body)
}

// pwVote casts a vote from the i-th voter cell and requires 200.
func pwVote(t *testing.T, w *pwWorld, client *http.Client, reportID int64, action string, cell int) []byte {
	t.Helper()
	status, raw, err := p07PostVoteRaw(client, w.srv.URL, reportID, action, p32uCells[cell][0], p32uCells[cell][1])
	if err != nil || status != http.StatusOK {
		t.Fatalf("%s on report %d from cell %d: status %d, err %v, body %s", action, reportID, cell, status, err, raw)
	}
	return raw
}

// pwConfirmTwice confirms the report from two distinct cells, running the
// pipeline after each vote, as the server does after each one.
func pwConfirmTwice(t *testing.T, w *pwWorld, reportID int64, v1, v2 *pwReader) {
	t.Helper()
	pwVote(t, w, v1.client, reportID, "confirm", 1)
	w.run(t)
	pwVote(t, w, v2.client, reportID, "confirm", 2)
	w.run(t)
}

func pwHaversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthKm = 6371.0088
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthKm * math.Asin(math.Sqrt(a))
}

// pwHomeKm is the distance from the middle of the precision 5 cell a home area
// is stored as to the report point, which is how the server measures it.
func pwHomeKm(t *testing.T, homeLat, homeLon float64) float64 {
	t.Helper()
	cell := geohash.EncodeWithPrecision(homeLat, homeLon, service.HomeAreaCellPrecision)
	lat, lon, ok := service.HomeAreaAnchor(cell)
	if !ok {
		t.Fatalf("home cell %q does not decode", cell)
	}
	return pwHaversineKm(p07ReportLat, p07ReportLon, lat, lon)
}

// pwUnconfirmedBody and pwConfirmedBody are the Critical bodies for a reader
// whose home area is at the report point.
func pwUnconfirmedBody(t *testing.T) string {
	t.Helper()
	return service.DistanceWords(pwHomeKm(t, p07ReportLat, p07ReportLon)) + pwFromHome + " " + pwUnconfirmed
}

func pwConfirmedBody(t *testing.T) string {
	t.Helper()
	return pwConfirmed + " " + service.DistanceWords(pwHomeKm(t, p07ReportLat, p07ReportLon)) + pwFromHome
}

func pwText(m push.Message) string {
	return strings.Join([]string{m.Payload.Title, m.Payload.Body, m.Payload.URL, m.Payload.Tag, m.Payload.Kind, m.Topic}, "\n")
}

// pwWantOne requires exactly want messages at the reader's device.
func pwWantCount(t *testing.T, w *pwWorld, r *pwReader, want int) []push.Message {
	t.Helper()
	got := w.sender.to(r.endpoint)
	if len(got) != want {
		t.Fatalf("%s got %d messages, want %d: %+v", r.name, len(got), want, got)
	}
	return got
}

// TestPhoneAlertCriticalEndToEnd is ROADMAP criteria 3 and 6 and decisions 2
// and 3: the person near a new Critical report gets exactly one notification
// that names the pick and the area and reads Not confirmed yet, the same
// notification is replaced in place once people nearby confirm the report, and
// the far reader, the author and the person who muted the report get none.
func TestPhoneAlertCriticalEndToEnd(t *testing.T) {
	w := pwSetup(t)
	a := w.near(t, "crit-a")
	// B's home area is about 15 km north of the report.
	b := w.readerAt(t, "crit-b", p07ReportLat+0.135, p07ReportLon)
	c := w.near(t, "crit-c")
	r := w.near(t, "crit-r")
	v1 := w.person(t, "crit-v1")
	v2 := w.person(t, "crit-v2")

	if km := pwHomeKm(t, p07ReportLat, p07ReportLon); km > 5 {
		t.Fatalf("A's home cell middle is %.1f km from the report, want within the 5 km default", km)
	}
	if lat, lon := p07ReportLat+0.135, p07ReportLon; pwHomeKm(t, lat, lon) <= 10 {
		t.Fatalf("B's home cell middle is not beyond 10 km")
	}

	id := pwPostCritical(t, w, r.client, "")
	if got := w.notifier.all(); len(got) != 1 || got[0] != (pwPoke{report: id, actor: 0}) {
		t.Fatalf("pokes after the post = %+v, want one for report %d with actor 0", got, id)
	}

	// C mutes the report before the pipeline runs.
	resp := p32uPost(t, c.client, w.srv.URL+"/api/reports/"+strconv.FormatInt(id, 10)+"/mute", "")
	p32uStatus(t, "C mutes the report", resp, http.StatusNoContent)

	w.run(t)

	first := pwWantCount(t, w, a, 1)[0]
	idText := strconv.FormatInt(id, 10)
	if first.Payload.Kind != "critical" || first.Payload.Tag != "report-"+idText || first.Payload.URL != "/?report="+idText {
		t.Errorf("first message kind/tag/url = %q %q %q", first.Payload.Kind, first.Payload.Tag, first.Payload.URL)
	}
	if first.Payload.Title != "Water rising near "+pwAreaName {
		t.Errorf("first title = %q", first.Payload.Title)
	}
	if want := pwUnconfirmedBody(t); first.Payload.Body != want {
		t.Errorf("first body = %q, want %q", first.Payload.Body, want)
	}
	if first.Urgency != push.UrgencyHigh {
		t.Errorf("urgency = %q, want high", first.Urgency)
	}
	if first.TTL <= 0 || first.TTL > time.Hour {
		t.Errorf("TTL = %v, want above 0 and at most 1h", first.TTL)
	}
	if first.Topic != "r"+idText {
		t.Errorf("topic = %q, want r%s", first.Topic, idText)
	}
	for _, none := range []*pwReader{b, c, r} {
		pwWantCount(t, w, none, 0)
	}

	// Two readers at distinct cells confirm; the same notification is replaced.
	pwConfirmTwice(t, w, id, v1, v2)
	if got := w.notifier.all(); len(got) != 3 || got[1] != (pwPoke{id, v1.id}) || got[2] != (pwPoke{id, v2.id}) {
		t.Fatalf("pokes = %+v, want the post then the two votes", got)
	}
	both := pwWantCount(t, w, a, 2)
	second := both[1]
	if second.Payload.Kind != "confirmed_update" || second.Payload.Tag != first.Payload.Tag {
		t.Errorf("update kind/tag = %q %q, want confirmed_update and the first tag %q", second.Payload.Kind, second.Payload.Tag, first.Payload.Tag)
	}
	if second.Payload.Title != first.Payload.Title {
		t.Errorf("update title = %q, want the first title %q", second.Payload.Title, first.Payload.Title)
	}
	if want := pwConfirmedBody(t); second.Payload.Body != want {
		t.Errorf("update body = %q, want %q", second.Payload.Body, want)
	}
	for _, none := range []*pwReader{b, c, r} {
		pwWantCount(t, w, none, 0)
	}

	w.laterSweepsSendNothing(t)
	pwWantCount(t, w, a, 2)
}

// TestPhoneAlertReliableAtPosting is decision 6 and ROADMAP criterion 3: a
// Reliable reporter's low report is pushed at posting, while it is still
// Provisional, to the reader who turned that switch on and to nobody else.
func TestPhoneAlertReliableAtPosting(t *testing.T) {
	w := pwSetup(t)
	const sessionID = "pw-reliable-reporter-session"
	testutil.SeedReliableReporter(t, w.pool, pwEmail("rel-reporter"), sessionID, time.Now().UTC())
	reporter := p17raSessionClient(t, w.srv.URL, sessionID)

	a := w.near(t, "rel-a")
	pwAlerts(t, w, a.client, `{"reliable_alerts":true,"alert_radius_km":5}`)
	// N is near too, with the Reliable switch off: a low report is not Critical,
	// so N gets nothing.
	n := w.near(t, "rel-n")

	id := submitReportGetID(t, reporter, w.srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	w.run(t)

	msg := pwWantCount(t, w, a, 1)[0]
	if msg.Payload.Kind != "reliable" || msg.Payload.Tag != "report-"+strconv.FormatInt(id, 10) {
		t.Errorf("kind/tag = %q %q", msg.Payload.Kind, msg.Payload.Tag)
	}
	if msg.Payload.Title != "Flood near "+pwAreaName {
		t.Errorf("title = %q", msg.Payload.Title)
	}
	body := msg.Payload.Body
	if !strings.HasPrefix(body, "From a Reliable reporter.") || !strings.HasSuffix(body, pwUnconfirmed) {
		t.Errorf("body = %q, want it to start From a Reliable reporter. and end %s", body, pwUnconfirmed)
	}
	if want := "From a Reliable reporter. " + service.DistanceWords(pwHomeKm(t, p07ReportLat, p07ReportLon)) + pwFromHome + " " + pwUnconfirmed; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
	if msg.Urgency != push.UrgencyNormal || msg.TTL <= 0 || msg.TTL > 30*time.Minute {
		t.Errorf("urgency/TTL = %q %v, want normal and at most 30m", msg.Urgency, msg.TTL)
	}
	pwWantCount(t, w, n, 0)
	w.laterSweepsSendNothing(t)
}

// TestPhoneAlertChangesToMyReport is decision 3 for the Changes kind: the
// person who posted a report and turned Changes on is told once it is
// confirmed, and the voter whose vote caused the change is not told about their
// own vote.
func TestPhoneAlertChangesToMyReport(t *testing.T) {
	w := pwSetup(t)
	r := w.near(t, "chg-r")
	v1 := w.near(t, "chg-v1")
	v2 := w.near(t, "chg-v2")
	for _, p := range []*pwReader{r, v1, v2} {
		pwAlerts(t, w, p.client, `{"changes_push":true}`)
	}
	// Critical is on by default and r, v1 and v2 live near the report, so a
	// Critical report would push to v1 and v2. A low report does not.
	id := submitReportGetID(t, r.client, w.srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	w.run(t)
	pwWantCount(t, w, r, 0)

	pwConfirmTwice(t, w, id, v1, v2)

	got := pwWantCount(t, w, r, 1)[0]
	if got.Payload.Kind != "changed" || got.Payload.Title != "Your report is now confirmed" {
		t.Errorf("reporter message kind/title = %q %q, want changed and Your report is now confirmed", got.Payload.Kind, got.Payload.Title)
	}
	if got.Payload.Tag != "report-"+strconv.FormatInt(id, 10) {
		t.Errorf("tag = %q", got.Payload.Tag)
	}
	// v2 cast the vote that confirmed the report: nothing for their own vote,
	// including after the timed changes sweep.
	pwWantCount(t, w, v2, 0)
	w.laterSweepsSendNothing(t)
	pwWantCount(t, w, r, 1)
	pwWantCount(t, w, v2, 0)
	// v1 voted earlier. Their first refresh happens at v2's vote, when the
	// report is already confirmed, so 3.2 takes a silent baseline and v1 gets
	// nothing; the loop only guards the kind of any message they might get.
	for _, m := range w.sender.to(v1.endpoint) {
		if m.Payload.Kind != "changed" || !strings.HasPrefix(m.Payload.Title, "A report you voted on") {
			t.Errorf("v1 got an unexpected message: %+v", m.Payload)
		}
	}
}

// TestPhoneAlertCapShowsNothingExtra is decision 6, T-03.3-08 and the 03.3-UAT
// gap for test 8 (2026-10-28): the hourly cap never holds a Critical alert near
// the home area, because it is never stored in the bell, so five Critical
// reports from five reporters near one reader in one hour give five
// notifications, every one of kind critical, with no summary and no extra kind.
// They count toward the cap, so the next push of another kind in the same hour
// (a Reliable reporter's report, with that switch on) is held back: the phone
// gets nothing for it and its outbox row is skipped as cap_account_hour.
func TestPhoneAlertCapShowsNothingExtra(t *testing.T) {
	w := pwSetup(t)
	a := w.near(t, "cap-a")
	pwAlerts(t, w, a.client, `{"reliable_alerts":true,"alert_radius_km":5}`)
	const critical = 5
	var ids []int64
	for i := 0; i < critical; i++ {
		reporter := w.person(t, "cap-r"+strconv.Itoa(i))
		ids = append(ids, pwPostCritical(t, w, reporter.client, ""))
		w.run(t)
	}

	got := pwWantCount(t, w, a, critical)
	if service.PushPerAccountPerHour != 4 {
		t.Fatalf("the hourly cap is %d, this test is written for 4", service.PushPerAccountPerHour)
	}
	tags := map[string]bool{}
	for i, m := range got {
		if m.Payload.Kind != "critical" {
			t.Errorf("message %d is kind %q, want critical (no summary or extra kind)", i, m.Payload.Kind)
		}
		if strings.Contains(strings.ToLower(pwText(m)), "more") {
			t.Errorf("message %d reads like a summary: %q", i, pwText(m))
		}
		tags[m.Payload.Tag] = true
	}
	if len(tags) != critical {
		t.Errorf("the %d messages carry %d distinct tags, want %d", critical, len(tags), critical)
	}
	if !tags["report-"+strconv.FormatInt(ids[critical-1], 10)] {
		t.Errorf("the fifth Critical report did not reach the phone")
	}
	if n := delCount(t, w.pool, `SELECT count(*) FROM push_outbox WHERE account_id = $1 AND state = 'skipped' AND skip_reason = 'cap_account_hour'`, a.id); n != 0 {
		t.Errorf("rows skipped by the hourly cap = %d before the sixth push, want 0 (Critical is never held)", n)
	}

	// The sixth push, of another kind, in the same hour: held back, nothing shown.
	const sessionID = "pw-cap-reliable-reporter-session"
	testutil.SeedReliableReporter(t, w.pool, pwEmail("cap-rel"), sessionID, time.Now().UTC())
	relReporter := p17raSessionClient(t, w.srv.URL, sessionID)
	submitReportGetID(t, relReporter, w.srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	w.run(t)

	pwWantCount(t, w, a, critical)
	if n := delCount(t, w.pool, `SELECT count(*) FROM push_outbox WHERE account_id = $1 AND kind = 'reliable' AND state = 'skipped' AND skip_reason = 'cap_account_hour'`, a.id); n != 1 {
		t.Errorf("reliable rows skipped by the hourly cap = %d, want 1", n)
	}
	if total := w.sender.total(); total != critical {
		t.Errorf("the fake push service saw %d messages in all, want %d", total, critical)
	}
}

// TestPhoneAlertAppleGetsNoUpdate is decision 3 for iPhone: an Apple endpoint
// gets the first Critical notification and no confirmed update, while the same
// person's other device gets both.
func TestPhoneAlertAppleGetsNoUpdate(t *testing.T) {
	w := pwSetup(t)
	a := w.near(t, "apple-a")
	apple := "https://web.push.apple.com/pw-apple-a"
	pwRegister(t, w, a.client, apple)
	r := w.person(t, "apple-r")
	v1 := w.person(t, "apple-v1")
	v2 := w.person(t, "apple-v2")

	id := pwPostCritical(t, w, r.client, "")
	w.run(t)
	if got := w.sender.to(apple); len(got) != 1 || got[0].Payload.Kind != "critical" {
		t.Fatalf("the Apple endpoint got %+v after the post, want one critical message", got)
	}
	pwWantCount(t, w, a, 1)

	pwConfirmTwice(t, w, id, v1, v2)

	if got := w.sender.to(apple); len(got) != 1 {
		t.Errorf("the Apple endpoint got %d messages, want 1 (no confirmed update)", len(got))
	}
	both := pwWantCount(t, w, a, 2)
	if both[1].Payload.Kind != "confirmed_update" {
		t.Errorf("the other device's second message is %q, want confirmed_update", both[1].Payload.Kind)
	}
}

// TestPhoneAlertStopsAfterLogout is decision 11: after Log out from the session
// that registered the device, a new Critical report sends nothing to it, while
// another reader's device still gets it.
func TestPhoneAlertStopsAfterLogout(t *testing.T) {
	w := pwSetup(t)
	a := w.near(t, "out-a")
	d := w.near(t, "out-d")
	r := w.person(t, "out-r")

	pwPostCritical(t, w, r.client, "")
	w.run(t)
	pwWantCount(t, w, a, 1)
	pwWantCount(t, w, d, 1)

	resp := meSend(t, revNoFollow(a.client), http.MethodPost, w.srv.URL+"/auth/logout", "")
	meWant(t, "POST /auth/logout", resp, http.StatusSeeOther)
	if rows := pushRows(t, w.pool, a.endpoint); len(rows) != 0 {
		t.Fatalf("the logged out device is still stored: %+v", rows)
	}

	pwPostCritical(t, w, r.client, "")
	w.run(t)
	pwWantCount(t, w, a, 1)
	pwWantCount(t, w, d, 2)
}

// TestPhoneAlertPayloadHasNoReporterWords is T-03.3-06 over HTTP: the report's
// description holds a phone number and a street name, and nothing the fake push
// service received, before or after the confirmation, holds either, nor the
// description, nor coordinate digits, nor an email address.
func TestPhoneAlertPayloadHasNoReporterWords(t *testing.T) {
	w := pwSetup(t)
	a := w.near(t, "words-a")
	r := w.person(t, "words-r")
	v1 := w.person(t, "words-v1")
	v2 := w.person(t, "words-v2")

	const description = "Call 9876543210 at 14 Gandhi Road near the old temple"
	id := pwPostCritical(t, w, r.client, description)
	var stored string
	if err := w.pool.QueryRow(context.Background(), `SELECT description FROM reports WHERE id = $1`, id).Scan(&stored); err != nil {
		t.Fatalf("reading the stored description: %v", err)
	}
	if stored != description {
		t.Fatalf("stored description = %q, want the posted one (the test needs it saved as written)", stored)
	}
	w.run(t)
	pwConfirmTwice(t, w, id, v1, v2)

	pwWantCount(t, w, a, 2)
	forbidden := []string{
		description, "9876543210", "Gandhi", "14 Gandhi", "old temple",
		"12.97", "77.59", "12.9716", "77.5946",
		"@", "example.com", "words-r", "words-v1",
	}
	for _, s := range w.sender.everything() {
		raw, err := json.Marshal(s.msg)
		if err != nil {
			t.Fatalf("marshalling a recorded message: %v", err)
		}
		for _, bad := range forbidden {
			if strings.Contains(string(raw), bad) || strings.Contains(pwText(s.msg), bad) {
				t.Errorf("a pushed message holds %q: %s", bad, raw)
			}
		}
	}
}

// TestPhoneAlertAreaNameStaysOffFeedAndBell is D-19 and D-20: once the dispatcher
// stored the coarse area name for the lock screen, no feed, bell, vote or page
// response carries it, nor an area_name field.
func TestPhoneAlertAreaNameStaysOffFeedAndBell(t *testing.T) {
	w := pwSetup(t)
	a := w.near(t, "area-a")
	r := w.person(t, "area-r")
	v1 := w.person(t, "area-v1")

	id := pwPostCritical(t, w, r.client, "")
	w.run(t)

	var name *string
	if err := w.pool.QueryRow(context.Background(), `SELECT area_name FROM reports WHERE id = $1`, id).Scan(&name); err != nil {
		t.Fatalf("reading the stored area name: %v", err)
	}
	if name == nil || *name != pwAreaName {
		t.Fatalf("stored area name = %v, want %q (without it this test proves nothing)", name, pwAreaName)
	}
	if n := w.namer.calls.Load(); n != 1 {
		t.Errorf("the area lookup ran %d times for one report, want 1", n)
	}
	if got := w.sender.to(a.endpoint); len(got) != 1 || !strings.Contains(got[0].Payload.Title, pwAreaName) {
		t.Fatalf("the lock screen did not carry the area name: %+v", got)
	}

	voteBody := pwVote(t, w, v1.client, id, "confirm", 1)
	bodies := map[string]string{"POST confirm": string(voteBody)}
	for _, client := range []*http.Client{a.client, r.client, v1.client} {
		resp := getNearby(t, client, w.srv.URL, p07ReportLat, p07ReportLon, true)
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/reports: status %d, err %v", resp.StatusCode, err)
		}
		bodies["GET /api/reports"+strconv.Itoa(len(bodies))] = string(raw)

		resp, err = client.Get(w.srv.URL + "/api/updates")
		if err != nil {
			t.Fatalf("GET /api/updates: %v", err)
		}
		raw = p20Status(t, "GET /api/updates", resp, http.StatusOK)
		bodies["GET /api/updates"+strconv.Itoa(len(bodies))] = string(raw)

		resp, err = client.Get(w.srv.URL + "/?report=" + strconv.FormatInt(id, 10))
		if err != nil {
			t.Fatalf("GET /?report=: %v", err)
		}
		if resp.Request.URL.Path != "/" {
			t.Fatalf("GET /?report= ended at %s, want the page itself", resp.Request.URL.Path)
		}
		raw = p20Status(t, "GET /?report=", resp, http.StatusOK)
		bodies["GET /?report="+strconv.Itoa(len(bodies))] = string(raw)
	}
	for label, body := range bodies {
		if strings.Contains(body, pwAreaName) {
			t.Errorf("%s holds the area name", label)
		}
		if strings.Contains(strings.ToLower(body), "area_name") || strings.Contains(body, "areaName") {
			t.Errorf("%s holds an area name field", label)
		}
	}
}
