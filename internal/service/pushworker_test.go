// pushworker_test.go proves the phone alert worker (plan 03.3-11, task 1) with a
// fake store and a fake push sender: that every queued row is read again at send
// time and dropped when the report, the event, a switch or a mute no longer
// allows it, that the words come from BuildPushText with current facts, that the
// push options are right, that every answer of a push service is handled per
// device, that only the holder of a row's lease sends it, and that no log line or
// returned error holds an endpoint. The same worker runs on a real database in
// the two tests at the end of the file, which skip in the quick run.

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/push"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	pwReportID  = int64(42)
	pwAccountID = int64(7)
)

// The lease the claim returned and the one the renewal returned. Every mark must
// carry the second.
var (
	pwLease1 = pushFactsNow.Add(time.Minute)
	pwLease2 = pushFactsNow.Add(80 * time.Second)
)

// pwSkipReasons is the closed list of the CHECK in migration 00011. A fake that
// meets another word fails the test, because the real table would refuse it.
var pwSkipReasons = map[string]bool{
	"cap_account_hour": true, "not_eligible": true, "no_home_area": true,
	"out_of_range": true, "no_device": true, "stale": true,
}

// pwMark is one write to the outbox row.
type pwMark struct {
	kind        string // sent, skipped, rescheduled, failed
	id          int64
	lease       time.Time
	reason      string
	next        time.Time
	unconfirmed bool
	at          time.Time
}

// pwQuerier is a PushWorkerQuerier that records every call in order.
type pwQuerier struct {
	t  *testing.T
	mu sync.Mutex

	calls []string

	claimRows   []sqlcgen.ClaimDuePushesRow
	claimErr    error
	claimParams []sqlcgen.ClaimDuePushesParams
	staleNow    []time.Time
	pruneEdge   []time.Time

	extendErr    error
	extendParams []sqlcgen.ExtendPushLeaseParams

	facts map[int64]sqlcgen.PushReportFactsRow
	votes []sqlcgen.CurrentVotesForReportsRow

	home       sqlcgen.PushAccountHomeRow
	homeErr    error
	homeParams []sqlcgen.PushAccountHomeParams

	count       int64
	countParams []sqlcgen.CountAccountPushesInWindowParams

	subs []sqlcgen.PushSubscriptionsForAccountRow

	marks     []pwMark
	lostMark  bool  // every row mark changes 0 rows
	markErr   error // every row mark fails with this
	okMarks   []int64
	deleted   []int64
	bumps     []int64
	bumpReply map[int64]sqlcgen.BumpPushSubscriptionFailureRow

	// onExtend runs inside ExtendPushLease before it answers.
	onExtend func(callNo int)
}

func (q *pwQuerier) rec(name string) { q.calls = append(q.calls, name) }

func (q *pwQuerier) callCount(name string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, c := range q.calls {
		if c == name {
			n++
		}
	}
	return n
}

func (q *pwQuerier) PushReportFacts(_ context.Context, id int64) (sqlcgen.PushReportFactsRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("PushReportFacts")
	row, ok := q.facts[id]
	if !ok {
		return sqlcgen.PushReportFactsRow{}, pgx.ErrNoRows
	}
	return row, nil
}

func (q *pwQuerier) CurrentVotesForReports(_ context.Context, _ []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("CurrentVotesForReports")
	return q.votes, nil
}

func (q *pwQuerier) SkipStalePushes(_ context.Context, now time.Time) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("SkipStalePushes")
	q.staleNow = append(q.staleNow, now)
	return 0, nil
}

func (q *pwQuerier) PrunePushOutbox(_ context.Context, edge time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("PrunePushOutbox")
	q.pruneEdge = append(q.pruneEdge, edge)
	return nil
}

func (q *pwQuerier) ClaimDuePushes(_ context.Context, arg sqlcgen.ClaimDuePushesParams) ([]sqlcgen.ClaimDuePushesRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("ClaimDuePushes")
	q.claimParams = append(q.claimParams, arg)
	return q.claimRows, q.claimErr
}

func (q *pwQuerier) ExtendPushLease(_ context.Context, arg sqlcgen.ExtendPushLeaseParams) (time.Time, error) {
	q.mu.Lock()
	q.rec("ExtendPushLease")
	q.extendParams = append(q.extendParams, arg)
	hook, callNo := q.onExtend, len(q.extendParams)
	q.mu.Unlock()
	if hook != nil {
		hook(callNo)
	}
	if q.extendErr != nil {
		return time.Time{}, q.extendErr
	}
	return pwLease2, nil
}

func (q *pwQuerier) mark(m pwMark) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("Mark" + m.kind)
	q.marks = append(q.marks, m)
	if q.markErr != nil {
		return 0, q.markErr
	}
	if q.lostMark {
		return 0, nil
	}
	return 1, nil
}

func (q *pwQuerier) MarkPushSent(_ context.Context, a sqlcgen.MarkPushSentParams) (int64, error) {
	return q.mark(pwMark{kind: "sent", id: a.ID, lease: a.HeldLease, unconfirmed: a.SentUnconfirmed, at: a.Now})
}

func (q *pwQuerier) MarkPushSkipped(_ context.Context, a sqlcgen.MarkPushSkippedParams) (int64, error) {
	if !pwSkipReasons[a.SkipReason] {
		q.t.Errorf("skip reason %q is not in the CHECK list of push_outbox", a.SkipReason)
	}
	return q.mark(pwMark{kind: "skipped", id: a.ID, lease: a.HeldLease, reason: a.SkipReason})
}

func (q *pwQuerier) ReschedulePush(_ context.Context, a sqlcgen.ReschedulePushParams) (int64, error) {
	return q.mark(pwMark{kind: "rescheduled", id: a.ID, lease: a.HeldLease, next: a.NextAttemptAt})
}

func (q *pwQuerier) MarkPushFailed(_ context.Context, a sqlcgen.MarkPushFailedParams) (int64, error) {
	return q.mark(pwMark{kind: "failed", id: a.ID, lease: a.HeldLease})
}

func (q *pwQuerier) PushAccountHome(_ context.Context, a sqlcgen.PushAccountHomeParams) (sqlcgen.PushAccountHomeRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("PushAccountHome")
	q.homeParams = append(q.homeParams, a)
	return q.home, q.homeErr
}

func (q *pwQuerier) CountAccountPushesInWindow(_ context.Context, a sqlcgen.CountAccountPushesInWindowParams) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("CountAccountPushesInWindow")
	q.countParams = append(q.countParams, a)
	return q.count, nil
}

func (q *pwQuerier) PushSubscriptionsForAccount(_ context.Context, _ int64) ([]sqlcgen.PushSubscriptionsForAccountRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("PushSubscriptionsForAccount")
	return q.subs, nil
}

func (q *pwQuerier) MarkPushSubscriptionOK(_ context.Context, a sqlcgen.MarkPushSubscriptionOKParams) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("MarkPushSubscriptionOK")
	q.okMarks = append(q.okMarks, a.ID)
	return nil
}

func (q *pwQuerier) DeletePushSubscriptionByID(_ context.Context, id int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("DeletePushSubscriptionByID")
	q.deleted = append(q.deleted, id)
	return nil
}

func (q *pwQuerier) BumpPushSubscriptionFailure(_ context.Context, id int64) (sqlcgen.BumpPushSubscriptionFailureRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("BumpPushSubscriptionFailure")
	q.bumps = append(q.bumps, id)
	if r, ok := q.bumpReply[id]; ok {
		return r, nil
	}
	return sqlcgen.BumpPushSubscriptionFailureRow{Failures: 1}, nil
}

// pwAnswer is what the fake push service says to one endpoint.
type pwAnswer struct {
	status     int
	retryAfter time.Duration
	err        error
}

// pwSender is a push.Sender that records every message it is given.
type pwSender struct {
	mu      sync.Mutex
	sent    []pwSentMessage
	answers map[string]pwAnswer
	// hook runs inside Send before it answers.
	hook func(n int)
	// inflight and peak measure the concurrency of one fan out.
	inflight, peak int
	delay          time.Duration
}

type pwSentMessage struct {
	sub push.Subscription
	msg push.Message
}

func (s *pwSender) Send(ctx context.Context, sub push.Subscription, msg push.Message) (push.Result, error) {
	s.mu.Lock()
	s.sent = append(s.sent, pwSentMessage{sub: sub, msg: msg})
	n := len(s.sent)
	s.inflight++
	if s.inflight > s.peak {
		s.peak = s.inflight
	}
	hook, delay := s.hook, s.delay
	ans, ok := s.answers[sub.Endpoint]
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.inflight--
		s.mu.Unlock()
	}()
	if hook != nil {
		hook(n)
	}
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return push.Result{}, ctx.Err()
		}
	}
	if !ok {
		return push.Result{Status: 201}, nil
	}
	if ans.err != nil {
		return push.Result{}, ans.err
	}
	return push.Result{Status: ans.status, RetryAfter: ans.retryAfter}, nil
}

func (s *pwSender) messages() []pwSentMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]pwSentMessage(nil), s.sent...)
}

// pwRig is one test's world: a fake store, a fake push service, a worker on the
// fixed clock, and the captured log.
type pwRig struct {
	t        *testing.T
	q        *pwQuerier
	s        *pwSender
	w        *PushWorker
	logs     *bytes.Buffer
	homeCell string
	// homeLat and homeLon are the middle of the home cell; the report sits
	// about 2 km north of it.
	homeLat, homeLon float64
}

const (
	pwEndpointFCM   = "https://fcm.googleapis.com/fcm/send/pw-token-secret-1"
	pwEndpointApple = "https://web.push.apple.com/pw-apple-token-secret"
	pwP256dh        = "pw-p256dh-secret"
	pwAuth          = "pw-auth-secret"
)

func pwStr(s string) *string { return &s }

func pwDevice(id int64, endpoint, hostColumn string) sqlcgen.PushSubscriptionsForAccountRow {
	return sqlcgen.PushSubscriptionsForAccountRow{
		ID: id, Endpoint: endpoint, EndpointHost: hostColumn, P256dh: pwP256dh, Auth: pwAuth,
	}
}

func pwClaim(id int64, kind string) sqlcgen.ClaimDuePushesRow {
	row := sqlcgen.ClaimDuePushesRow{
		ID: id, AccountID: pwAccountID, ReportID: pwReportID, Kind: kind,
		LeaseUntil: pwLease1, Attempts: 1,
		ExpiresAt: pushFactsNow.Add(time.Hour),
		CreatedAt: pushFactsNow.Add(-5 * time.Minute),
	}
	if kind == "changed" {
		uid := int64(900 + id)
		row.UpdateID = &uid
		row.UpdateKind = pwStr("reached_live")
	}
	return row
}

// newPWRig builds the default world: one critical row for a critical flood
// report with the pick Fire, a reader whose home cell middle is about 2 km from
// the report, Critical on, and one Google device.
func newPWRig(t *testing.T, opts ...PushWorkerOption) *pwRig {
	t.Helper()
	r := &pwRig{t: t, logs: &bytes.Buffer{}}

	prev := log.Writer()
	log.SetOutput(r.logs)
	t.Cleanup(func() { log.SetOutput(prev) })

	r.homeCell = geohash.EncodeWithPrecision(12.9716, 77.5946, 5)
	lat, lon, ok := HomeAreaAnchor(r.homeCell)
	if !ok {
		t.Fatalf("home cell %q does not decode", r.homeCell)
	}
	r.homeLat, r.homeLon = lat, lon

	row := pushFactsRow("critical", "flood", "clear")
	row.ID = pwReportID
	row.Latitude, row.Longitude = lat+0.018, lon
	row.Happening = pwStr("fire")

	r.q = &pwQuerier{
		t:         t,
		facts:     map[int64]sqlcgen.PushReportFactsRow{pwReportID: row},
		claimRows: []sqlcgen.ClaimDuePushesRow{pwClaim(1, "critical")},
		home: sqlcgen.PushAccountHomeRow{
			HomeAreaCell: &r.homeCell, AlertRadiusKm: 5, CriticalPushOn: true,
		},
		subs:      []sqlcgen.PushSubscriptionsForAccountRow{pwDevice(11, pwEndpointFCM, "fcm.googleapis.com")},
		bumpReply: map[int64]sqlcgen.BumpPushSubscriptionFailureRow{},
	}
	r.s = &pwSender{answers: map[string]pwAnswer{}}
	all := append([]PushWorkerOption{
		WithPushWorkerClock(func() time.Time { return pushFactsNow }),
		WithPushWorkerJitter(func() float64 { return 0.5 }),
	}, opts...)
	r.w = NewPushWorker(r.q, r.s, all...)
	return r
}

// report changes the stored report row.
func (r *pwRig) report(mut func(*sqlcgen.PushReportFactsRow)) {
	r.t.Helper()
	row := r.q.facts[pwReportID]
	mut(&row)
	r.q.facts[pwReportID] = row
}

// claim changes the one claimed row.
func (r *pwRig) claim(mut func(*sqlcgen.ClaimDuePushesRow)) {
	r.t.Helper()
	mut(&r.q.claimRows[0])
}

func (r *pwRig) sweep() error {
	r.t.Helper()
	return r.w.SweepOnce(context.Background())
}

// onlyMark returns the one row mark made, failing when there is not exactly one.
func (r *pwRig) onlyMark() pwMark {
	r.t.Helper()
	if len(r.q.marks) != 1 {
		r.t.Fatalf("row marks = %+v, want exactly one", r.q.marks)
	}
	return r.q.marks[0]
}

// wantSkipped asserts the row was skipped with a reason and nothing was sent.
func (r *pwRig) wantSkipped(reason string) {
	r.t.Helper()
	m := r.onlyMark()
	if m.kind != "skipped" || m.reason != reason {
		r.t.Fatalf("mark = %+v, want skipped with %s", m, reason)
	}
	if n := len(r.s.messages()); n != 0 {
		r.t.Fatalf("%d messages were sent for a skipped row", n)
	}
}

func pwVotes(kind, value string, firstAccount int64, cells []string, n int) []sqlcgen.CurrentVotesForReportsRow {
	var rows []sqlcgen.CurrentVotesForReportsRow
	for i := 0; i < n; i++ {
		rows = append(rows, pushVote(pwReportID, firstAccount+int64(i), kind, value, cells[i]))
	}
	return rows
}

func pwConfirm(n int) []sqlcgen.CurrentVotesForReportsRow {
	return pwVotes("content", "confirm", 100, []string{"tdr1v8y", "tdr1vc3", "tdr1vd1", "tdr1vf2"}, n)
}

func pwDispute(n int) []sqlcgen.CurrentVotesForReportsRow {
	return pwVotes("content", "dispute", 200, []string{"tdr1w01", "tdr1w02", "tdr1w03", "tdr1w04"}, n)
}

// pwReporterResolved is the reporter's own resolve (the reporter is account 900).
func pwReporterResolved() []sqlcgen.CurrentVotesForReportsRow {
	return []sqlcgen.CurrentVotesForReportsRow{pushVote(pwReportID, 900, "resolution", "resolve", "tdr1v8y")}
}

func TestPushBackoff(t *testing.T) {
	cases := []struct {
		attempts   int
		retryAfter time.Duration
		factor     float64
		want       time.Duration
	}{
		{1, 0, 1, 15 * time.Second},
		{2, 0, 1, 60 * time.Second},
		{3, 0, 1, 4 * time.Minute},
		{4, 0, 1, 5 * time.Minute},
		{9, 0, 1, 5 * time.Minute},
		{1000, 0, 1, 5 * time.Minute},
		{0, 0, 1, 15 * time.Second},
		{-3, 0, 1, 15 * time.Second},
		// The cap applies after the jitter factor, so 1.5 never passes 5 minutes.
		{3, 0, 1.5, 5 * time.Minute},
		{1, 0, 0.5, 7500 * time.Millisecond},
		// Never below the Retry-After given, even past the cap.
		{1, 120 * time.Second, 1, 120 * time.Second},
		{3, 10 * time.Minute, 1, 10 * time.Minute},
		{1, 5 * time.Second, 1, 15 * time.Second},
	}
	for _, c := range cases {
		if got := PushBackoff(c.attempts, c.retryAfter, c.factor); got != c.want {
			t.Errorf("PushBackoff(%d, %s, %v) = %s, want %s", c.attempts, c.retryAfter, c.factor, got, c.want)
		}
	}
}

func TestSendErrorClass(t *testing.T) {
	endpoint := "https://fcm.googleapis.com/fcm/send/pw-class-secret"
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"deadline", context.DeadlineExceeded, "timeout"},
		{"deadline in a url error", &url.Error{Op: "Post", URL: endpoint, Err: context.DeadlineExceeded}, "timeout"},
		{"net timeout", &net.DNSError{IsTimeout: true}, "timeout"},
		{"canceled", context.Canceled, "canceled"},
		{"refused address", fmt.Errorf("dial: %w", push.ErrRefusedAddress), "refused_address"},
		{"unsupported service", push.ErrUnsupportedService, "policy"},
		{"bad endpoint", fmt.Errorf("x: %w", push.ErrBadEndpoint), "policy"},
		{"bad keys", push.ErrBadKeys, "policy"},
		{"payload too large", push.ErrPayloadTooLarge, "payload"},
		{"no ttl", push.ErrNoTTL, "payload"},
		{"dial error in a url error", &url.Error{Op: "Post", URL: endpoint, Err: errors.New("dial tcp: connection refused")}, "network"},
		{"anything else", errors.New("boom"), "network"},
	}
	closed := map[string]bool{"timeout": true, "canceled": true, "refused_address": true, "policy": true, "payload": true, "network": true}
	for _, c := range cases {
		got := sendErrorClass(c.err)
		if got != c.want {
			t.Errorf("%s: class = %q, want %q", c.name, got, c.want)
		}
		if !closed[got] {
			t.Errorf("%s: %q is not a closed class word", c.name, got)
		}
	}
}

func TestWorkerSkipsWhenNoLongerEligible(t *testing.T) {
	type tc struct {
		name     string
		kind     string
		event    string // the update row's kind of a changed row; "" leaves the default
		severity string
		category string
		moderate string
		votes    []sqlcgen.CurrentVotesForReportsRow
		wantVis  Visibility
		mutate   func(r *pwRig)
		first    *string
		want     string // "sent" or a skip reason
	}
	lowLive := func(name, kind, event string, want string) tc {
		return tc{name: name, kind: kind, event: event, severity: "low", votes: pwConfirm(2), wantVis: VisibilityLive, want: want}
	}
	off := func(mut func(*sqlcgen.PushAccountHomeRow)) func(r *pwRig) {
		return func(r *pwRig) { mut(&r.q.home) }
	}

	cases := []tc{
		// Critical.
		{name: "critical, still live", kind: "critical", severity: "critical", wantVis: VisibilityLive, want: "sent"},
		{name: "critical, pending check keeps it published", kind: "critical", severity: "critical", moderate: "pending", wantVis: VisibilityLive, want: "sent"},
		{name: "critical, removed after it was queued", kind: "critical", severity: "critical", moderate: "removed", wantVis: VisibilityWithheld, want: "not_eligible"},
		{name: "critical, retracted after it was queued", kind: "critical", severity: "critical", votes: pwReporterResolved(), wantVis: VisibilityRetracted, want: "not_eligible"},
		{name: "critical, report gone", kind: "critical", severity: "critical", wantVis: VisibilityLive, want: "not_eligible",
			mutate: func(r *pwRig) { delete(r.q.facts, pwReportID) }},
		{name: "critical, report expired", kind: "critical", severity: "critical", wantVis: VisibilityLive, want: "stale",
			mutate: func(r *pwRig) {
				r.report(func(row *sqlcgen.PushReportFactsRow) { row.ExpiresAt = pushFactsNow.Add(-time.Minute) })
			}},
		{name: "critical, row's own life is over", kind: "critical", severity: "critical", wantVis: VisibilityLive, want: "stale",
			mutate: func(r *pwRig) {
				r.claim(func(c *sqlcgen.ClaimDuePushesRow) { c.ExpiresAt = pushFactsNow })
			}},
		{name: "critical, the Critical switch is off now", kind: "critical", severity: "critical", wantVis: VisibilityLive, want: "not_eligible",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.CriticalPushOn = false })},
		{name: "critical, muted after it was queued", kind: "critical", severity: "critical", wantVis: VisibilityLive, want: "not_eligible",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.Muted = true })},
		{name: "critical, the account is gone", kind: "critical", severity: "critical", wantVis: VisibilityLive, want: "not_eligible",
			mutate: func(r *pwRig) { r.q.homeErr = pgx.ErrNoRows }},

		// Reliable.
		lowLive("reliable, live", "reliable", "", "sent"),
		{name: "reliable, provisional is still active", kind: "reliable", severity: "low", wantVis: VisibilityProvisional, want: "sent"},
		{name: "reliable, disputed since", kind: "reliable", severity: "low", votes: pwDispute(3), wantVis: VisibilityHidden, want: "not_eligible"},
		{name: "reliable, retracted since", kind: "reliable", severity: "low", votes: pwReporterResolved(), wantVis: VisibilityRetracted, want: "not_eligible"},
		{name: "reliable, the Reliable switch is off now", kind: "reliable", severity: "low", votes: pwConfirm(2), wantVis: VisibilityLive, want: "not_eligible",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.ReliableAlertsOn = false })},
		{name: "reliable, muted", kind: "reliable", severity: "low", wantVis: VisibilityProvisional, want: "not_eligible",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.Muted = true })},

		// Confirmed update.
		{name: "confirmed update, still confirmed", kind: "confirmed_update", severity: "critical", votes: pwConfirm(2), wantVis: VisibilityLive, first: pwStr("critical"), want: "sent"},
		{name: "confirmed update, not confirmed any more", kind: "confirmed_update", severity: "critical", votes: pwConfirm(1), wantVis: VisibilityLive, first: pwStr("critical"), want: "not_eligible"},
		{name: "confirmed update, disputed by most", kind: "confirmed_update", severity: "critical", votes: append(pwConfirm(2), pwDispute(3)...), wantVis: VisibilityLive, first: pwStr("critical"), want: "not_eligible"},
		{name: "confirmed update of a critical alert, Critical off now", kind: "confirmed_update", severity: "critical", votes: pwConfirm(2), wantVis: VisibilityLive, first: pwStr("critical"), want: "not_eligible",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.CriticalPushOn = false; h.ReliableAlertsOn = true })},
		{name: "confirmed update of a reliable alert, Reliable off now", kind: "confirmed_update", severity: "critical", votes: pwConfirm(2), wantVis: VisibilityLive, first: pwStr("reliable"), want: "not_eligible",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.CriticalPushOn = true; h.ReliableAlertsOn = false })},
		{name: "confirmed update of a reliable alert, Reliable on and Critical off", kind: "confirmed_update", severity: "critical", votes: pwConfirm(2), wantVis: VisibilityLive, first: pwStr("reliable"), want: "sent",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.CriticalPushOn = false; h.ReliableAlertsOn = true })},
		{name: "confirmed update, first row pruned, serious report follows Critical", kind: "confirmed_update", severity: "critical", votes: pwConfirm(2), wantVis: VisibilityLive, want: "not_eligible",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.CriticalPushOn = false; h.ReliableAlertsOn = true })},
		{name: "confirmed update, first row pruned, low report follows Reliable", kind: "confirmed_update", severity: "low", votes: pwConfirm(2), wantVis: VisibilityLive, want: "sent",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.CriticalPushOn = false; h.ReliableAlertsOn = true })},
		{name: "confirmed update, muted", kind: "confirmed_update", severity: "critical", votes: pwConfirm(2), wantVis: VisibilityLive, first: pwStr("critical"), want: "not_eligible",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.Muted = true })},
		{name: "confirmed update, removed since", kind: "confirmed_update", severity: "critical", moderate: "removed", votes: pwConfirm(2), wantVis: VisibilityWithheld, first: pwStr("critical"), want: "not_eligible"},

		// Changed rows follow their event (W4).
		lowLive("reached_live, still live", "changed", "reached_live", "sent"),
		{name: "reached_live, critical report still confirmed", kind: "changed", event: "reached_live", severity: "critical", votes: pwConfirm(2), wantVis: VisibilityLive, want: "sent"},
		{name: "reached_live, critical report disputed by most since (still Live, no longer confirmed)", kind: "changed", event: "reached_live", severity: "critical", votes: append(pwConfirm(2), pwDispute(3)...), wantVis: VisibilityLive, want: "not_eligible"},
		{name: "reached_live, critical report with no confirmations", kind: "changed", event: "reached_live", severity: "critical", wantVis: VisibilityLive, want: "not_eligible"},
		{name: "reached_live, hidden since", kind: "changed", event: "reached_live", severity: "low", votes: pwDispute(3), wantVis: VisibilityHidden, want: "not_eligible"},
		{name: "reached_live, retracted since", kind: "changed", event: "reached_live", severity: "low", votes: append(pwConfirm(2), pwReporterResolved()...), wantVis: VisibilityRetracted, want: "not_eligible"},
		{name: "reached_live, provisional again", kind: "changed", event: "reached_live", severity: "low", wantVis: VisibilityProvisional, want: "not_eligible"},
		{name: "hidden, still hidden", kind: "changed", event: "hidden", severity: "low", votes: pwDispute(3), wantVis: VisibilityHidden, want: "sent"},
		{name: "hidden, live again", kind: "changed", event: "hidden", severity: "low", votes: pwConfirm(2), wantVis: VisibilityLive, want: "not_eligible"},
		{name: "hidden, retracted since", kind: "changed", event: "hidden", severity: "low", votes: append(pwDispute(3), pwReporterResolved()...), wantVis: VisibilityRetracted, want: "not_eligible"},
		{name: "resolved, still retracted", kind: "changed", event: "resolved", severity: "low", votes: pwReporterResolved(), wantVis: VisibilityRetracted, want: "sent"},
		{name: "resolved, reopened", kind: "changed", event: "resolved", severity: "low", votes: pwConfirm(2), wantVis: VisibilityLive, want: "not_eligible"},
		{name: "fading, still active", kind: "changed", event: "fading", severity: "low", wantVis: VisibilityProvisional, want: "sent"},
		{name: "fading, hidden since", kind: "changed", event: "fading", severity: "low", votes: pwDispute(3), wantVis: VisibilityHidden, want: "not_eligible"},
		{name: "fading, retracted since", kind: "changed", event: "fading", severity: "low", votes: pwReporterResolved(), wantVis: VisibilityRetracted, want: "not_eligible"},
		{name: "fading, withheld since", kind: "changed", event: "fading", severity: "low", moderate: "removed", wantVis: VisibilityWithheld, want: "not_eligible"},
		{name: "fading, expired", kind: "changed", event: "fading", severity: "low", wantVis: VisibilityProvisional, want: "stale",
			mutate: func(r *pwRig) {
				r.report(func(row *sqlcgen.PushReportFactsRow) { row.ExpiresAt = pushFactsNow.Add(-time.Second) })
			}},
		{name: "changed, update row pruned", kind: "changed", event: "none", severity: "low", wantVis: VisibilityProvisional, want: "not_eligible"},
		{name: "changed, update row of another kind", kind: "changed", event: "reliable_nearby", severity: "low", wantVis: VisibilityProvisional, want: "not_eligible"},
		{name: "changed, the Changes switch is off now", kind: "changed", event: "fading", severity: "low", wantVis: VisibilityProvisional, want: "not_eligible",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.ChangesPushOn = false })},
		{name: "changed, muted", kind: "changed", event: "fading", severity: "low", wantVis: VisibilityProvisional, want: "not_eligible",
			mutate: off(func(h *sqlcgen.PushAccountHomeRow) { h.Muted = true })},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newPWRig(t)
			r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, c.kind)}
			r.q.home.ReliableAlertsOn = true
			r.q.home.ChangesPushOn = true
			r.q.home.CriticalPushOn = true
			r.report(func(row *sqlcgen.PushReportFactsRow) {
				row.Severity = c.severity
				if c.severity != "critical" {
					row.Happening = nil
				}
				if c.moderate != "" {
					row.ModerationState = c.moderate
				}
			})
			r.q.votes = c.votes
			r.q.claimRows[0].FirstKind = c.first
			switch c.event {
			case "":
			case "none":
				r.q.claimRows[0].UpdateKind = nil
			default:
				r.q.claimRows[0].UpdateKind = pwStr(c.event)
			}
			if c.mutate != nil {
				c.mutate(r)
			}

			// The fixture itself: the resolver must give the visibility the row
			// of this table says it has.
			if row, ok := r.q.facts[pwReportID]; ok {
				f := &fakePushFacts{row: row, votes: c.votes}
				facts, _, err := loadPushFacts(context.Background(), f, pwReportID, pushFactsNow)
				if err != nil || facts.Vis != c.wantVis {
					t.Fatalf("fixture: visibility = %q (err %v), want %q", facts.Vis, err, c.wantVis)
				}
			}

			if err := r.sweep(); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			if c.want == "sent" {
				if m := r.onlyMark(); m.kind != "sent" {
					t.Fatalf("mark = %+v, want sent", m)
				}
				if n := len(r.s.messages()); n != 1 {
					t.Fatalf("messages sent = %d, want 1", n)
				}
				return
			}
			r.wantSkipped(c.want)
		})
	}
}

func TestWorkerBuildsWordsAtSendTime(t *testing.T) {
	type tc struct {
		name      string
		kind      string
		first     *string
		severity  string
		votes     []sqlcgen.CurrentVotesForReportsRow
		happening *string
		area      string
		home      func(h *sqlcgen.PushAccountHomeRow)
		own       bool
		claim     func(c *sqlcgen.ClaimDuePushesRow)
		title     string
		body      string
		unconf    bool
	}
	cases := []tc{
		{name: "critical, not confirmed yet", kind: "critical", severity: "critical", happening: pwStr("fire"), area: "Central Market",
			title: "Fire near Central Market", body: "2 km from your home area. Not confirmed yet.", unconf: true},
		{name: "the same row built again once two cells confirmed it", kind: "critical", severity: "critical", votes: pwConfirm(2), happening: pwStr("fire"), area: "Central Market",
			title: "Fire near Central Market", body: "Confirmed by people nearby. 2 km from your home area."},
		{name: "critical with no area name", kind: "critical", severity: "critical", happening: pwStr("fire"),
			title: "Fire", body: "2 km from your home area. Not confirmed yet.", unconf: true},
		{name: "critical with no pick uses the category", kind: "critical", severity: "critical", area: "Central Market",
			title: "Flood, Critical near Central Market", body: "2 km from your home area. Not confirmed yet.", unconf: true},
		{name: "reliable, not confirmed yet", kind: "reliable", severity: "low", area: "Central Market",
			title: "Flood near Central Market", body: "From a Reliable reporter. 2 km from your home area. Not confirmed yet.", unconf: true},
		{name: "confirmed update of a low report uses the Reliable title", kind: "confirmed_update", severity: "low", votes: pwConfirm(2), area: "Central Market",
			title: "Flood near Central Market", body: "Confirmed by people nearby. 2 km from your home area."},
		{name: "confirmed update of a critical alert keeps the Critical title", kind: "confirmed_update", first: pwStr("critical"), severity: "critical", votes: pwConfirm(2), happening: pwStr("fire"), area: "Central Market",
			title: "Fire near Central Market", body: "Confirmed by people nearby. 2 km from your home area."},
		{name: "confirmed update of a Reliable alert about a serious report keeps the Reliable title", kind: "confirmed_update", first: pwStr("reliable"), severity: "critical", votes: pwConfirm(2), happening: pwStr("fire"), area: "Central Market",
			home:  func(h *sqlcgen.PushAccountHomeRow) { h.CriticalPushOn = false; h.ReliableAlertsOn = true },
			title: "Flood near Central Market", body: "Confirmed by people nearby. 2 km from your home area."},
		{name: "confirmed update, first row pruned, serious report falls back to Critical", kind: "confirmed_update", severity: "critical", votes: pwConfirm(2), happening: pwStr("fire"), area: "Central Market",
			title: "Fire near Central Market", body: "Confirmed by people nearby. 2 km from your home area."},
		{name: "changed, someone else's report", kind: "changed", severity: "low", votes: pwConfirm(2), area: "Central Market",
			title: "A report you voted on is now confirmed", body: "Flood, Low. Central Market, 2 km from your home area."},
		{name: "changed, my own report", kind: "changed", severity: "low", votes: pwConfirm(2), area: "Central Market", own: true,
			title: "Your report is now confirmed", body: "Flood, Low. Central Market, 2 km from your home area."},
		{name: "changed without a home area has no distance", kind: "changed", severity: "low", votes: pwConfirm(2), area: "Central Market",
			home:  func(h *sqlcgen.PushAccountHomeRow) { h.HomeAreaCell = nil },
			title: "A report you voted on is now confirmed", body: "Flood, Low. Central Market."},
		{name: "a long area name is cut", kind: "critical", severity: "critical", happening: pwStr("fire"), area: "Mahatma Gandhi Road Metro Station Complex",
			title: "Fire near Mahatma Gandhi Road Metro…", body: "2 km from your home area. Not confirmed yet.", unconf: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newPWRig(t)
			r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, c.kind)}
			r.q.claimRows[0].FirstKind = c.first
			r.q.home.ReliableAlertsOn = true
			r.q.home.ChangesPushOn = true
			r.q.votes = c.votes
			if c.home != nil {
				c.home(&r.q.home)
			}
			r.report(func(row *sqlcgen.PushReportFactsRow) {
				row.Severity = c.severity
				row.Happening = c.happening
				if c.area == "" {
					row.AreaName = nil
				} else {
					row.AreaName = pwStr(c.area)
				}
				if c.own {
					acct := pwAccountID
					row.ReporterAccountID = &acct
				}
			})
			if err := r.sweep(); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			msgs := r.s.messages()
			if len(msgs) != 1 {
				t.Fatalf("messages = %d, want 1 (marks %+v)", len(msgs), r.q.marks)
			}
			p := msgs[0].msg.Payload
			if p.Title != c.title || p.Body != c.body {
				t.Errorf("words = %q / %q, want %q / %q", p.Title, p.Body, c.title, c.body)
			}
			m := r.onlyMark()
			if m.kind != "sent" || m.unconfirmed != c.unconf {
				t.Errorf("mark = %+v, want sent with sent_unconfirmed %v", m, c.unconf)
			}
			if len([]rune(p.Title)) > PushTitleMaxRunes || len([]rune(p.Body)) > PushBodyMaxRunes {
				t.Errorf("words over budget: %d and %d runes", len([]rune(p.Title)), len([]rune(p.Body)))
			}
		})
	}

	t.Run("the words are not stored: a second sweep after the report changed words it again", func(t *testing.T) {
		r := newPWRig(t)
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.q.votes = pwConfirm(2)
		r.q.marks = nil
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		msgs := r.s.messages()
		if len(msgs) != 2 {
			t.Fatalf("messages = %d, want 2", len(msgs))
		}
		if !strings.Contains(msgs[0].msg.Payload.Body, NotConfirmedYetWords) || strings.Contains(msgs[1].msg.Payload.Body, NotConfirmedYetWords) {
			t.Errorf("bodies = %q then %q", msgs[0].msg.Payload.Body, msgs[1].msg.Payload.Body)
		}
	})
}

func TestWorkerMessageOptions(t *testing.T) {
	type tc struct {
		name    string
		kind    string
		sev     string
		votes   []sqlcgen.CurrentVotesForReportsRow
		first   *string
		expires time.Duration // the report's remaining life
		urgency push.Urgency
		ttl     time.Duration
		stale   bool
	}
	cases := []tc{
		{name: "critical, long life", kind: "critical", sev: "critical", expires: 5 * time.Hour, urgency: push.UrgencyHigh, ttl: PushCriticalTTLMax},
		{name: "critical, 20 minutes left", kind: "critical", sev: "critical", expires: 20 * time.Minute, urgency: push.UrgencyHigh, ttl: 20 * time.Minute},
		{name: "critical, 45 seconds left", kind: "critical", sev: "critical", expires: 45 * time.Second, urgency: push.UrgencyHigh, ttl: 45 * time.Second},
		{name: "critical, under a second left is stale", kind: "critical", sev: "critical", expires: 900 * time.Millisecond, stale: true},
		{name: "reliable, long life", kind: "reliable", sev: "low", expires: 5 * time.Hour, urgency: push.UrgencyNormal, ttl: PushOtherTTL},
		{name: "reliable, 10 minutes left", kind: "reliable", sev: "low", expires: 10 * time.Minute, urgency: push.UrgencyNormal, ttl: 10 * time.Minute},
		{name: "changed", kind: "changed", sev: "low", votes: pwConfirm(2), expires: 5 * time.Hour, urgency: push.UrgencyNormal, ttl: PushOtherTTL},
		{name: "confirmed update of a critical alert is normal", kind: "confirmed_update", sev: "critical", votes: pwConfirm(2), first: pwStr("critical"), expires: 5 * time.Hour, urgency: push.UrgencyNormal, ttl: PushOtherTTL},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newPWRig(t)
			r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(5, c.kind)}
			r.q.claimRows[0].FirstKind = c.first
			r.q.claimRows[0].ExpiresAt = pushFactsNow.Add(c.expires)
			r.q.home.ReliableAlertsOn = true
			r.q.home.ChangesPushOn = true
			r.q.votes = c.votes
			r.report(func(row *sqlcgen.PushReportFactsRow) {
				row.Severity = c.sev
				row.ExpiresAt = pushFactsNow.Add(c.expires)
			})
			if err := r.sweep(); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			if c.stale {
				r.wantSkipped("stale")
				return
			}
			msgs := r.s.messages()
			if len(msgs) != 1 {
				t.Fatalf("messages = %d, want 1", len(msgs))
			}
			m := msgs[0].msg
			if m.Payload.V != 1 {
				t.Errorf("payload version = %d", m.Payload.V)
			}
			if m.Payload.Kind != c.kind {
				t.Errorf("payload kind = %q, want the row kind %q", m.Payload.Kind, c.kind)
			}
			if m.Payload.Tag != "report-42" || m.Payload.URL != "/?report=42" || m.Topic != "r42" {
				t.Errorf("tag %q url %q topic %q", m.Payload.Tag, m.Payload.URL, m.Topic)
			}
			if !push.ValidTopic(m.Topic) {
				t.Errorf("topic %q is not valid", m.Topic)
			}
			if m.Urgency != c.urgency {
				t.Errorf("urgency = %q, want %q", m.Urgency, c.urgency)
			}
			if m.TTL != c.ttl || m.TTL <= 0 {
				t.Errorf("TTL = %s, want %s", m.TTL, c.ttl)
			}
			if _, err := m.Payload.Encode(); err != nil {
				t.Errorf("payload does not encode: %v", err)
			}
			sub := msgs[0].sub
			if sub.Endpoint != pwEndpointFCM || sub.P256dh != pwP256dh || sub.Auth != pwAuth {
				t.Errorf("the message went to the wrong subscription")
			}
		})
	}
}

func TestWorkerConfirmedUpdateSkipsApple(t *testing.T) {
	setup := func(t *testing.T, kind string, devices ...sqlcgen.PushSubscriptionsForAccountRow) *pwRig {
		r := newPWRig(t)
		r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, kind)}
		if kind == "confirmed_update" {
			r.q.claimRows[0].FirstKind = pwStr("critical")
			r.q.votes = pwConfirm(2)
		}
		r.q.subs = devices
		return r
	}
	endpointsSent := func(r *pwRig) []string {
		var out []string
		for _, m := range r.s.messages() {
			out = append(out, m.sub.Endpoint)
		}
		return out
	}

	t.Run("a confirmed update goes to the Google device and not the Apple one", func(t *testing.T) {
		r := setup(t, "confirmed_update",
			pwDevice(11, pwEndpointApple, "web.push.apple.com"), pwDevice(12, pwEndpointFCM, "fcm.googleapis.com"))
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if got := endpointsSent(r); len(got) != 1 || got[0] != pwEndpointFCM {
			t.Fatalf("sent to %v, want only the Google device", got)
		}
		if m := r.onlyMark(); m.kind != "sent" {
			t.Fatalf("mark = %+v", m)
		}
	})
	t.Run("an account with only Apple devices has the update skipped as no_device", func(t *testing.T) {
		r := setup(t, "confirmed_update", pwDevice(11, pwEndpointApple, "web.push.apple.com"))
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.wantSkipped("no_device")
	})
	t.Run("the first alert still goes to an Apple device", func(t *testing.T) {
		r := setup(t, "critical", pwDevice(11, pwEndpointApple, "web.push.apple.com"))
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if got := endpointsSent(r); len(got) != 1 || got[0] != pwEndpointApple {
			t.Fatalf("sent to %v", got)
		}
	})
	t.Run("N6: the stored host column saying Google does not hide an Apple endpoint", func(t *testing.T) {
		r := setup(t, "confirmed_update", pwDevice(11, pwEndpointApple, "fcm.googleapis.com"))
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.wantSkipped("no_device")
	})
	t.Run("N6: the stored host column saying Apple does not hold back a Google endpoint", func(t *testing.T) {
		r := setup(t, "confirmed_update", pwDevice(11, pwEndpointFCM, "web.push.apple.com"))
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if got := endpointsSent(r); len(got) != 1 || got[0] != pwEndpointFCM {
			t.Fatalf("sent to %v, want the Google device", got)
		}
	})
}

func TestWorkerCapPerAccountHour(t *testing.T) {
	for _, kind := range []string{"critical", "reliable", "changed"} {
		t.Run(kind+" over the cap is skipped and nothing else is queued", func(t *testing.T) {
			r := newPWRig(t)
			r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, kind)}
			r.q.home.ReliableAlertsOn = true
			r.q.home.ChangesPushOn = true
			r.q.count = PushPerAccountPerHour
			if kind == "changed" {
				r.q.claimRows[0].UpdateKind = pwStr("fading")
			}
			r.report(func(row *sqlcgen.PushReportFactsRow) {
				if kind != "critical" {
					row.Severity = "low"
					row.Happening = nil
				}
			})
			if err := r.sweep(); err != nil {
				t.Fatal(err)
			}
			r.wantSkipped("cap_account_hour")
			if len(r.q.countParams) != 1 {
				t.Fatalf("count reads = %d", len(r.q.countParams))
			}
			p := r.q.countParams[0]
			if p.AccountID != pwAccountID || !p.Since.Equal(pushFactsNow.Add(-time.Hour)) {
				t.Errorf("count params = %+v, want the account and a rolling hour", p)
			}
		})
		t.Run(kind+" under the cap is sent", func(t *testing.T) {
			r := newPWRig(t)
			r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, kind)}
			r.q.home.ReliableAlertsOn = true
			r.q.home.ChangesPushOn = true
			r.q.count = PushPerAccountPerHour - 1
			if kind == "changed" {
				r.q.claimRows[0].UpdateKind = pwStr("fading")
			}
			r.report(func(row *sqlcgen.PushReportFactsRow) {
				if kind != "critical" {
					row.Severity = "low"
					row.Happening = nil
				}
			})
			if err := r.sweep(); err != nil {
				t.Fatal(err)
			}
			if m := r.onlyMark(); m.kind != "sent" {
				t.Fatalf("mark = %+v", m)
			}
		})
	}
	t.Run("a confirmed update is not counted and still goes", func(t *testing.T) {
		r := newPWRig(t)
		r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, "confirmed_update")}
		r.q.claimRows[0].FirstKind = pwStr("critical")
		r.q.votes = pwConfirm(2)
		r.q.count = 99
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if m := r.onlyMark(); m.kind != "sent" {
			t.Fatalf("mark = %+v", m)
		}
		if n := r.q.callCount("CountAccountPushesInWindow"); n != 0 {
			t.Errorf("the cap was read %d times for a confirmed update", n)
		}
	})
}

func TestWorkerNoDeviceOrNoHome(t *testing.T) {
	t.Run("no home area skips a critical row", func(t *testing.T) {
		r := newPWRig(t)
		r.q.home.HomeAreaCell = nil
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.wantSkipped("no_home_area")
	})
	t.Run("a stored value that is not a cell counts as no home area", func(t *testing.T) {
		r := newPWRig(t)
		r.q.home.HomeAreaCell = pwStr("NOT A CELL")
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.wantSkipped("no_home_area")
	})
	t.Run("a reliable row without a home area is skipped", func(t *testing.T) {
		r := newPWRig(t)
		r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, "reliable")}
		r.q.home.ReliableAlertsOn = true
		r.q.home.HomeAreaCell = nil
		r.report(func(row *sqlcgen.PushReportFactsRow) { row.Severity = "low"; row.Happening = nil })
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.wantSkipped("no_home_area")
	})
	t.Run("a home area now beyond the account's radius is out of range", func(t *testing.T) {
		r := newPWRig(t)
		r.q.home.AlertRadiusKm = 1
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.wantSkipped("out_of_range")
	})
	t.Run("the report sits a little over 2 km away: a 2 km radius is out and a 3 km radius is in", func(t *testing.T) {
		r := newPWRig(t)
		r.q.home.AlertRadiusKm = 2
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.wantSkipped("out_of_range")

		r = newPWRig(t)
		r.q.home.AlertRadiusKm = 3
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if m := r.onlyMark(); m.kind != "sent" {
			t.Fatalf("mark = %+v", m)
		}
	})
	t.Run("a confirmed update follows the same range", func(t *testing.T) {
		r := newPWRig(t)
		r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, "confirmed_update")}
		r.q.claimRows[0].FirstKind = pwStr("critical")
		r.q.votes = pwConfirm(2)
		r.q.home.AlertRadiusKm = 1
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.wantSkipped("out_of_range")
	})
	t.Run("an account with no device has the row skipped", func(t *testing.T) {
		r := newPWRig(t)
		r.q.subs = nil
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.wantSkipped("no_device")
	})
	t.Run("a device whose endpoint no longer passes the policy is not sent to and counts a failure", func(t *testing.T) {
		r := newPWRig(t)
		r.q.subs = []sqlcgen.PushSubscriptionsForAccountRow{pwDevice(11, "https://evil.example.com/push/pw-evil-token", "fcm.googleapis.com")}
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		r.wantSkipped("no_device")
		if len(r.q.bumps) != 1 || r.q.bumps[0] != 11 {
			t.Errorf("failure bumps = %v, want one for device 11", r.q.bumps)
		}
	})
}

func TestWorkerResponses(t *testing.T) {
	two := func(a, b pwAnswer) func(r *pwRig) {
		return func(r *pwRig) {
			r.q.subs = []sqlcgen.PushSubscriptionsForAccountRow{
				pwDevice(11, pwEndpointFCM, "fcm.googleapis.com"),
				pwDevice(12, "https://updates.push.services.mozilla.com/wpush/v2/pw-token-2", "updates.push.services.mozilla.com"),
			}
			r.s.answers[r.q.subs[0].Endpoint] = a
			r.s.answers[r.q.subs[1].Endpoint] = b
		}
	}
	one := func(a pwAnswer) func(r *pwRig) {
		return func(r *pwRig) { r.s.answers[pwEndpointFCM] = a }
	}
	netErr := &url.Error{Op: "Post", URL: pwEndpointFCM, Err: errors.New("dial tcp 203.0.113.9:443: i/o timeout")}

	type tc struct {
		name     string
		setup    func(r *pwRig)
		attempts int32

		mark       string        // sent, rescheduled, failed
		wait       time.Duration // exact wait for rescheduled; at least this for a Retry-After
		okMarks    []int64
		deleted    []int64
		bumps      []int64
		sentCount  int
		bumpReply  map[int64]sqlcgen.BumpPushSubscriptionFailureRow
		unconfirmd bool
	}
	staleOK := pgtype.Timestamptz{Time: pushFactsNow.Add(-8 * 24 * time.Hour), Valid: true}
	recentOK := pgtype.Timestamptz{Time: pushFactsNow.Add(-2 * 24 * time.Hour), Valid: true}
	cases := []tc{
		{name: "201 marks the device fine and the row sent", setup: one(pwAnswer{status: 201}), mark: "sent", okMarks: []int64{11}, sentCount: 1, unconfirmd: true},
		{name: "200 is accepted too", setup: one(pwAnswer{status: 200}), mark: "sent", okMarks: []int64{11}, sentCount: 1, unconfirmd: true},
		{name: "404 deletes the device", setup: one(pwAnswer{status: 404}), mark: "failed", deleted: []int64{11}, sentCount: 1},
		{name: "410 deletes the device", setup: one(pwAnswer{status: 410}), mark: "failed", deleted: []int64{11}, sentCount: 1},
		{name: "429 waits for Retry-After and counts nothing", setup: one(pwAnswer{status: 429, retryAfter: 120 * time.Second}), mark: "rescheduled", wait: 120 * time.Second, sentCount: 1},
		{name: "429 without Retry-After uses the backoff", setup: one(pwAnswer{status: 429}), mark: "rescheduled", wait: 15 * time.Second, sentCount: 1},
		{name: "500 backs off 15 seconds and counts nothing", setup: one(pwAnswer{status: 500}), mark: "rescheduled", wait: 15 * time.Second, sentCount: 1},
		{name: "503 on the third attempt backs off 4 minutes", setup: one(pwAnswer{status: 503}), attempts: 3, mark: "rescheduled", wait: 4 * time.Minute, sentCount: 1},
		{name: "a network error backs off and counts nothing", setup: one(pwAnswer{err: netErr}), mark: "rescheduled", wait: 15 * time.Second, sentCount: 1},
		{name: "a timeout backs off and counts nothing", setup: one(pwAnswer{err: context.DeadlineExceeded}), mark: "rescheduled", wait: 15 * time.Second, sentCount: 1},
		{name: "a canceled send backs off and counts nothing", setup: one(pwAnswer{err: context.Canceled}), mark: "rescheduled", wait: 15 * time.Second, sentCount: 1},
		{name: "a refused address backs off and counts nothing", setup: one(pwAnswer{err: push.ErrRefusedAddress}), mark: "rescheduled", wait: 15 * time.Second, sentCount: 1},
		{name: "the fourth failed attempt fails the row", setup: one(pwAnswer{status: 500}), attempts: PushMaxAttempts, mark: "failed", sentCount: 1},
		{name: "400 counts against the device and is not retried", setup: one(pwAnswer{status: 400}), mark: "failed", bumps: []int64{11}, sentCount: 1},
		{name: "403 counts against the device and is not retried", setup: one(pwAnswer{status: 403}), mark: "failed", bumps: []int64{11}, sentCount: 1},
		{name: "401 is not retried and counts nothing (our key, not the device)", setup: one(pwAnswer{status: 401}), mark: "failed", sentCount: 1},
		{name: "413 is not retried and counts nothing", setup: one(pwAnswer{status: 413}), mark: "failed", sentCount: 1},
		{name: "a payload error is not retried and counts nothing", setup: one(pwAnswer{err: push.ErrPayloadTooLarge}), mark: "failed", sentCount: 1},

		// The failure limit (W6).
		{name: "five 403s on a device that never succeeded delete it", setup: one(pwAnswer{status: 403}), mark: "failed", bumps: []int64{11}, deleted: []int64{11}, sentCount: 1,
			bumpReply: map[int64]sqlcgen.BumpPushSubscriptionFailureRow{11: {Failures: PushDeviceFailureLimit}}},
		{name: "five 403s on a device that succeeded 2 days ago keep it", setup: one(pwAnswer{status: 403}), mark: "failed", bumps: []int64{11}, sentCount: 1,
			bumpReply: map[int64]sqlcgen.BumpPushSubscriptionFailureRow{11: {Failures: PushDeviceFailureLimit, LastOkAt: recentOK}}},
		{name: "five 403s on a device whose last success is 8 days old delete it", setup: one(pwAnswer{status: 403}), mark: "failed", bumps: []int64{11}, deleted: []int64{11}, sentCount: 1,
			bumpReply: map[int64]sqlcgen.BumpPushSubscriptionFailureRow{11: {Failures: PushDeviceFailureLimit, LastOkAt: staleOK}}},
		{name: "four failures are below the limit", setup: one(pwAnswer{status: 400}), mark: "failed", bumps: []int64{11}, sentCount: 1,
			bumpReply: map[int64]sqlcgen.BumpPushSubscriptionFailureRow{11: {Failures: PushDeviceFailureLimit - 1}}},

		// Several devices (N5).
		{name: "one device took it and the other failed: the row is sent and nothing is retried", setup: two(pwAnswer{status: 201}, pwAnswer{status: 500}), mark: "sent", okMarks: []int64{11}, sentCount: 2, unconfirmd: true},
		{name: "one took it and the other answered 403: the row is sent", setup: two(pwAnswer{status: 403}, pwAnswer{status: 201}), mark: "sent", okMarks: []int64{12}, bumps: []int64{11}, sentCount: 2, unconfirmd: true},
		{name: "both failing retriably reschedule once, by the longest Retry-After", setup: two(pwAnswer{status: 429, retryAfter: 90 * time.Second}, pwAnswer{status: 503, retryAfter: 200 * time.Second}), mark: "rescheduled", wait: 200 * time.Second, sentCount: 2},
		{name: "one retriable and one permanent: retried", setup: two(pwAnswer{status: 500}, pwAnswer{status: 403}), mark: "rescheduled", wait: 15 * time.Second, bumps: []int64{12}, sentCount: 2},
		{name: "both permanent: failed at once", setup: two(pwAnswer{status: 404}, pwAnswer{status: 410}), mark: "failed", deleted: []int64{11, 12}, sentCount: 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newPWRig(t)
			if c.attempts != 0 {
				r.claim(func(row *sqlcgen.ClaimDuePushesRow) { row.Attempts = c.attempts })
			}
			for id, reply := range c.bumpReply {
				r.q.bumpReply[id] = reply
			}
			c.setup(r)
			if err := r.sweep(); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			m := r.onlyMark()
			if m.kind != c.mark {
				t.Fatalf("mark = %+v, want %s", m, c.mark)
			}
			if m.lease != pwLease2 {
				t.Errorf("the mark carried lease %v, want the renewed lease", m.lease)
			}
			switch c.mark {
			case "rescheduled":
				if got := m.next.Sub(pushFactsNow); got != c.wait {
					t.Errorf("next attempt in %s, want %s", got, c.wait)
				}
			case "sent":
				if !m.at.Equal(pushFactsNow) {
					t.Errorf("sent at %v", m.at)
				}
				if m.unconfirmed != c.unconfirmd {
					t.Errorf("sent_unconfirmed = %v, want %v", m.unconfirmed, c.unconfirmd)
				}
			}
			if n := len(r.s.messages()); n != c.sentCount {
				t.Errorf("sends = %d, want %d", n, c.sentCount)
			}
			if fmt.Sprint(r.q.okMarks) != fmt.Sprint(c.okMarks) {
				t.Errorf("devices marked fine = %v, want %v", r.q.okMarks, c.okMarks)
			}
			if fmt.Sprint(sortedInt64s(r.q.deleted)) != fmt.Sprint(c.deleted) {
				t.Errorf("devices deleted = %v, want %v", r.q.deleted, c.deleted)
			}
			if fmt.Sprint(sortedInt64s(r.q.bumps)) != fmt.Sprint(c.bumps) {
				t.Errorf("failure counts bumped = %v, want %v", r.q.bumps, c.bumps)
			}
		})
	}

	t.Run("W6: a run of 500s and network errors never deletes a device", func(t *testing.T) {
		for _, a := range []pwAnswer{{status: 500}, {status: 502}, {err: netErr}, {err: context.DeadlineExceeded}, {err: push.ErrRefusedAddress}} {
			r := newPWRig(t)
			r.s.answers[pwEndpointFCM] = a
			for i := 0; i < 8; i++ {
				r.q.marks = nil
				if err := r.sweep(); err != nil {
					t.Fatal(err)
				}
			}
			if len(r.q.bumps) != 0 || len(r.q.deleted) != 0 {
				t.Errorf("answer %+v: bumps %v deleted %v", a, r.q.bumps, r.q.deleted)
			}
		}
	})

	t.Run("a device policy refusal at the push service counts like a 403", func(t *testing.T) {
		r := newPWRig(t)
		r.s.answers[pwEndpointFCM] = pwAnswer{err: push.ErrUnsupportedService}
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if m := r.onlyMark(); m.kind != "failed" {
			t.Fatalf("mark = %+v", m)
		}
		if fmt.Sprint(r.q.bumps) != "[11]" {
			t.Errorf("bumps = %v", r.q.bumps)
		}
	})

	t.Run("one row's fan out is bounded in concurrency", func(t *testing.T) {
		r := newPWRig(t)
		r.q.subs = nil
		for i := int64(0); i < 8; i++ {
			r.q.subs = append(r.q.subs, pwDevice(100+i, fmt.Sprintf("https://fcm.googleapis.com/fcm/send/pw-many-%d", i), "fcm.googleapis.com"))
		}
		r.s.delay = 5 * time.Millisecond
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if r.s.peak > PushSendConcurrency {
			t.Errorf("peak concurrency %d over %d", r.s.peak, PushSendConcurrency)
		}
		if n := len(r.s.messages()); n != 8 {
			t.Errorf("sends = %d, want 8", n)
		}
	})

	t.Run("the fan out runs under one PushSendTimeout", func(t *testing.T) {
		r := newPWRig(t)
		// A sender that reports the deadline of its context.
		probe := &pwDeadlineSender{}
		r.w = NewPushWorker(r.q, probe, WithPushWorkerClock(func() time.Time { return pushFactsNow }), WithPushWorkerJitter(func() float64 { return 0.5 }))
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if probe.remaining <= 0 || probe.remaining > PushSendTimeout {
			t.Errorf("the send context had %s left, want at most %s", probe.remaining, PushSendTimeout)
		}
	})
}

// sortedInt64s returns a sorted copy.
func sortedInt64s(in []int64) []int64 {
	out := append([]int64(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// pwDeadlineSender records how long the context of its one send had left.
type pwDeadlineSender struct {
	remaining time.Duration
}

func (s *pwDeadlineSender) Send(ctx context.Context, _ push.Subscription, _ push.Message) (push.Result, error) {
	if dl, ok := ctx.Deadline(); ok {
		s.remaining = time.Until(dl)
	}
	return push.Result{Status: 201}, nil
}

func TestWorkerBackoffStopsAtExpiry(t *testing.T) {
	t.Run("the next attempt would pass the row's expiry", func(t *testing.T) {
		r := newPWRig(t)
		r.claim(func(c *sqlcgen.ClaimDuePushesRow) { c.ExpiresAt = pushFactsNow.Add(10 * time.Second) })
		r.s.answers[pwEndpointFCM] = pwAnswer{status: 500}
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if m := r.onlyMark(); m.kind != "failed" {
			t.Fatalf("mark = %+v, want failed", m)
		}
	})
	t.Run("the next attempt would pass the report's expiry", func(t *testing.T) {
		r := newPWRig(t)
		r.report(func(row *sqlcgen.PushReportFactsRow) { row.ExpiresAt = pushFactsNow.Add(10 * time.Second) })
		r.s.answers[pwEndpointFCM] = pwAnswer{status: 500}
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if m := r.onlyMark(); m.kind != "failed" {
			t.Fatalf("mark = %+v, want failed", m)
		}
	})
	t.Run("a retry that still fits is scheduled", func(t *testing.T) {
		r := newPWRig(t)
		r.claim(func(c *sqlcgen.ClaimDuePushesRow) { c.ExpiresAt = pushFactsNow.Add(20 * time.Second) })
		r.s.answers[pwEndpointFCM] = pwAnswer{status: 500}
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if m := r.onlyMark(); m.kind != "rescheduled" {
			t.Fatalf("mark = %+v, want rescheduled", m)
		}
	})
	t.Run("a Retry-After past the expiry fails the row", func(t *testing.T) {
		r := newPWRig(t)
		r.claim(func(c *sqlcgen.ClaimDuePushesRow) { c.ExpiresAt = pushFactsNow.Add(time.Minute) })
		r.s.answers[pwEndpointFCM] = pwAnswer{status: 429, retryAfter: 5 * time.Minute}
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if m := r.onlyMark(); m.kind != "failed" {
			t.Fatalf("mark = %+v, want failed", m)
		}
	})
}

func TestWorkerNeverLogsSecrets(t *testing.T) {
	r := newPWRig(t)
	r.q.subs = []sqlcgen.PushSubscriptionsForAccountRow{
		pwDevice(11, pwEndpointFCM, "fcm.googleapis.com"),
		pwDevice(12, "https://updates.push.services.mozilla.com/wpush/v2/pw-token-mozilla-secret", "updates.push.services.mozilla.com"),
	}
	r.s.answers[pwEndpointFCM] = pwAnswer{err: &url.Error{Op: "Post", URL: pwEndpointFCM, Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}}}
	r.s.answers[r.q.subs[1].Endpoint] = pwAnswer{status: 500}
	r.q.markErr = errors.New("database is shutting down")
	r.report(func(row *sqlcgen.PushReportFactsRow) { row.AreaName = pwStr("Central Market") })

	err := r.sweep()
	if err == nil {
		t.Fatal("the failed mark should come back as an error")
	}
	secrets := []string{
		pwEndpointFCM, "/fcm/send/pw-token-secret-1", "pw-token-secret",
		"/wpush/v2/pw-token-mozilla-secret", "pw-token-mozilla-secret",
		pwP256dh, pwAuth,
		"Fire near Central Market", "Central Market", "from your home area", "Not confirmed yet",
		"12.97", "77.59",
	}
	haystacks := map[string]string{"log": r.logs.String(), "returned error": err.Error()}
	for name, text := range haystacks {
		for _, s := range secrets {
			if strings.Contains(text, s) {
				t.Errorf("the %s holds %q:\n%s", name, s, text)
			}
		}
	}
	if !strings.Contains(r.logs.String(), "fcm.googleapis.com") {
		t.Errorf("the log should name the host, which is the one allowed part of an endpoint:\n%s", r.logs.String())
	}
	if !strings.Contains(r.logs.String(), "network") {
		t.Errorf("the log should carry the class word network:\n%s", r.logs.String())
	}
	if !strings.Contains(err.Error(), "row 1") {
		t.Errorf("the returned error should name the row id: %v", err)
	}
}

func TestWorkerStopsOnLostLease(t *testing.T) {
	if time.Duration(PushLeaseSeconds)*time.Second < PushSendTimeout+PushLeaseMargin {
		t.Fatalf("the lease (%d s) is shorter than a row's fan out (%s) plus its margin (%s)", PushLeaseSeconds, PushSendTimeout, PushLeaseMargin)
	}

	t.Run("the lease is renewed first, with the claim's lease", func(t *testing.T) {
		r := newPWRig(t)
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if len(r.q.extendParams) != 1 {
			t.Fatalf("renewals = %d", len(r.q.extendParams))
		}
		p := r.q.extendParams[0]
		if p.ID != 1 || !p.HeldLease.Equal(pwLease1) || p.LeaseSeconds != PushLeaseSeconds {
			t.Errorf("renewal params = %+v", p)
		}
		first := -1
		for i, c := range r.q.calls {
			if c == "ExtendPushLease" {
				first = i
				break
			}
		}
		for i, c := range r.q.calls[:first] {
			if c != "SkipStalePushes" && c != "PrunePushOutbox" && c != "ClaimDuePushes" {
				t.Errorf("call %d (%s) came before the lease renewal", i, c)
			}
		}
		if m := r.onlyMark(); m.lease != pwLease2 {
			t.Errorf("the mark carried %v, want the renewed lease %v, not the claim's", m.lease, pwLease2)
		}
	})

	t.Run("a row whose renewal finds another holder is neither read, sent nor marked", func(t *testing.T) {
		r := newPWRig(t)
		r.q.extendErr = pgx.ErrNoRows
		if err := r.sweep(); err != nil {
			t.Fatalf("a lost lease is not an error: %v", err)
		}
		for _, c := range []string{"PushReportFacts", "PushAccountHome", "PushSubscriptionsForAccount", "MarkPushSubscriptionOK"} {
			if n := r.q.callCount(c); n != 0 {
				t.Errorf("%s was called %d times for a row this worker no longer holds", c, n)
			}
		}
		if n := len(r.s.messages()); n != 0 {
			t.Errorf("%d messages sent", n)
		}
		if len(r.q.marks) != 0 {
			t.Errorf("marks = %+v", r.q.marks)
		}
		if !strings.Contains(r.logs.String(), "lease_lost") || !strings.Contains(r.logs.String(), "row 1") {
			t.Errorf("the log should say lease_lost with the row id:\n%s", r.logs.String())
		}
	})

	t.Run("a renewal that fails for another reason is a database error", func(t *testing.T) {
		r := newPWRig(t)
		r.q.extendErr = errors.New("connection reset")
		err := r.sweep()
		if err == nil || !strings.Contains(err.Error(), "row 1") {
			t.Fatalf("err = %v, want a database error naming the row", err)
		}
		if n := len(r.s.messages()); n != 0 {
			t.Errorf("%d messages sent", n)
		}
	})

	t.Run("a mark that changes no row is logged and ends the row", func(t *testing.T) {
		r := newPWRig(t)
		r.q.lostMark = true
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if len(r.q.marks) != 1 {
			t.Errorf("marks = %+v, want exactly one attempt", r.q.marks)
		}
		if !strings.Contains(r.logs.String(), "lease_lost") {
			t.Errorf("the log should say lease_lost:\n%s", r.logs.String())
		}
	})

	t.Run("a lost skip mark is logged too", func(t *testing.T) {
		r := newPWRig(t)
		r.q.lostMark = true
		r.q.home.Muted = true
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(r.logs.String(), "lease_lost") {
			t.Errorf("the log should say lease_lost:\n%s", r.logs.String())
		}
	})
}

func TestWorkerSweep(t *testing.T) {
	t.Run("a sweep drops stale rows and old finished rows first, then claims", func(t *testing.T) {
		r := newPWRig(t)
		r.q.claimRows = nil
		if err := r.sweep(); err != nil {
			t.Fatal(err)
		}
		want := []string{"SkipStalePushes", "PrunePushOutbox", "ClaimDuePushes"}
		if fmt.Sprint(r.q.calls) != fmt.Sprint(want) {
			t.Fatalf("calls = %v, want %v", r.q.calls, want)
		}
		if !r.q.staleNow[0].Equal(pushFactsNow) {
			t.Errorf("stale edge = %v", r.q.staleNow[0])
		}
		if !r.q.pruneEdge[0].Equal(pushFactsNow.Add(-PushOutboxRetention)) {
			t.Errorf("prune edge = %v, want now minus the retention", r.q.pruneEdge[0])
		}
		p := r.q.claimParams[0]
		if p.BatchLimit != PushClaimBatch || p.LeaseSeconds != PushLeaseSeconds {
			t.Errorf("claim params = %+v", p)
		}
	})

	t.Run("a claim error comes back at once", func(t *testing.T) {
		r := newPWRig(t)
		r.q.claimErr = errors.New("no database")
		if err := r.sweep(); err == nil {
			t.Fatal("want the claim error")
		}
	})

	t.Run("it stops between rows when the context is done", func(t *testing.T) {
		r := newPWRig(t)
		r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, "critical"), pwClaim(2, "critical"), pwClaim(3, "critical")}
		ctx, cancel := context.WithCancel(context.Background())
		r.s.hook = func(n int) { cancel() }
		if err := r.w.SweepOnce(ctx); err != nil {
			t.Logf("SweepOnce: %v", err)
		}
		if n := len(r.q.extendParams); n != 1 {
			t.Errorf("rows worked = %d, want 1", n)
		}
		// The first row's answer was already in: its mark is still written.
		if len(r.q.marks) != 1 || r.q.marks[0].kind != "sent" {
			t.Errorf("marks = %+v, want the first row sent", r.q.marks)
		}
	})

	t.Run("rows are worked in claim order and each error names its row", func(t *testing.T) {
		r := newPWRig(t)
		r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, "critical"), pwClaim(2, "critical")}
		r.q.markErr = errors.New("write failed")
		err := r.sweep()
		if err == nil || !strings.Contains(err.Error(), "row 1") || !strings.Contains(err.Error(), "row 2") {
			t.Fatalf("err = %v, want both rows named", err)
		}
	})

	t.Run("Run sweeps once at once and stops with its context", func(t *testing.T) {
		r := newPWRig(t, WithPushWorkerInterval(time.Hour))
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { r.w.Run(ctx); close(done) }()
		deadline := time.Now().Add(2 * time.Second)
		for r.q.callCount("ClaimDuePushes") == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if r.q.callCount("ClaimDuePushes") == 0 {
			t.Fatal("no sweep at start")
		}
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not stop")
		}
	})
}

// pwOutboxRow reads the state of the one row of an account, report and kind.
func pwOutboxRow(t *testing.T, r *pushDBRig, account, reportID int64, kind string) (state, skip string, unconfirmed bool) {
	t.Helper()
	var skipReason *string
	err := r.pool.QueryRow(context.Background(),
		`SELECT state, skip_reason, sent_unconfirmed FROM push_outbox WHERE account_id = $1 AND report_id = $2 AND kind = $3`,
		account, reportID, kind).Scan(&state, &skipReason, &unconfirmed)
	if err != nil {
		t.Fatalf("reading the %s row of account %d: %v", kind, account, err)
	}
	if skipReason != nil {
		skip = *skipReason
	}
	return state, skip, unconfirmed
}

// pwDBWorker builds a worker on the rig's own clock, which runs behind wall time
// (the dispatcher wrote the rows with it). A worker on wall time would find
// every row stale.
func pwDBWorker(r *pushDBRig, sender push.Sender) *PushWorker {
	return NewPushWorker(r.q, sender,
		WithPushWorkerClock(func() time.Time { return r.cur }),
		WithPushWorkerJitter(func() float64 { return 0.5 }))
}

// TestWorkerOnRealOutbox runs the real dispatcher and the real worker on a real
// database with a fake push service: the words, the options, the in place
// confirmed update, and the Apple rule, which reads the endpoint and not the
// stored host column (the seed helper always stores fcm.googleapis.com there).
func TestWorkerOnRealOutbox(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const reporterSession = "p33-11-real-reporter"
	reporter := testutil.SeedReporterAccount(t, r.pool, r.email("reporter"), reporterSession)
	readerID, _ := r.reader("reader", 1, 0, true, false, false)

	// A second reader whose only device is on an Apple endpoint.
	const appleSession = "p33-11-real-apple"
	appleID := testutil.SeedReporterAccount(t, r.pool, r.email("apple"), appleSession)
	testutil.SetHomeCell(t, r.pool, appleID, "Home", r.cell(1))
	const appleEndpoint = "https://web.push.apple.com/p33-11-apple-token"
	testutil.SeedPushSubscription(t, r.pool, appleID, appleSession, appleEndpoint)

	v1 := testutil.SeedReporterAccount(t, r.pool, r.email("v1"), "p33-11-real-v1")
	v2 := testutil.SeedReporterAccount(t, r.pool, r.email("v2"), "p33-11-real-v2")

	r.cur = r.base.Add(10 * time.Minute)
	reportID := r.report(reporterSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
	testutil.MustExec(t, r.pool, `UPDATE reports SET happening = 'fire' WHERE id = $1`, reportID)
	r.eval(reportID, 0)
	if got := r.outbox(reportID); len(got[readerID]) != 1 || len(got[appleID]) != 1 || len(got[reporter]) != 0 {
		t.Fatalf("outbox = %v, want one critical row for each reader and none for the reporter", got)
	}

	sender := &pwSender{answers: map[string]pwAnswer{}}
	worker := pwDBWorker(r, sender)

	wantDistance := DistanceWords(r.distKm(r.cell(1))) + " from your home area."
	var readerEndpoint string
	if err := r.pool.QueryRow(ctx, `SELECT endpoint FROM push_subscriptions WHERE account_id = $1`, readerID).Scan(&readerEndpoint); err != nil {
		t.Fatalf("reading the reader's endpoint: %v", err)
	}
	messageTo := func(msgs []pwSentMessage, endpoint string) *pwSentMessage {
		for i := range msgs {
			if msgs[i].sub.Endpoint == endpoint {
				return &msgs[i]
			}
		}
		return nil
	}

	// 1. The first sweep sends the initial alert to both readers.
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	msgs := sender.messages()
	if len(msgs) != 2 {
		t.Fatalf("messages after the first sweep = %d, want 2", len(msgs))
	}
	first := messageTo(msgs, readerEndpoint)
	if first == nil {
		t.Fatal("the reader's device got nothing")
	}
	p := first.msg.Payload
	wantTag := fmt.Sprintf("report-%d", reportID)
	if p.Title != "Fire near Central Market" || p.Body != wantDistance+" Not confirmed yet." {
		t.Errorf("words = %q / %q", p.Title, p.Body)
	}
	if p.Kind != "critical" || p.Tag != wantTag || p.URL != fmt.Sprintf("/?report=%d", reportID) || first.msg.Topic != fmt.Sprintf("r%d", reportID) {
		t.Errorf("envelope = %+v topic %q", p, first.msg.Topic)
	}
	if first.msg.Urgency != push.UrgencyHigh || first.msg.TTL <= 0 || first.msg.TTL > PushCriticalTTLMax {
		t.Errorf("urgency %q ttl %s", first.msg.Urgency, first.msg.TTL)
	}
	for _, id := range []int64{readerID, appleID} {
		state, _, unconfirmed := pwOutboxRow(t, r, id, reportID, "critical")
		if state != "sent" || !unconfirmed {
			t.Errorf("account %d: critical row state %q, sent_unconfirmed %v, want sent and true", id, state, unconfirmed)
		}
	}
	if messageTo(msgs, appleEndpoint) == nil {
		t.Error("the Apple device should get the first alert")
	}

	// 2. A second sweep sends nothing.
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if n := len(sender.messages()); n != 2 {
		t.Fatalf("messages after the second sweep = %d, want still 2", n)
	}

	// 3. Two cells confirm the report; the dispatcher queues the in place
	// confirmed update and the worker sends one, to the Google device only.
	r.vote(reportID, v1, "content", "confirm", "tdr1v8y", r.cur)
	r.vote(reportID, v2, "content", "confirm", "tdr1vc3", r.cur)
	r.eval(reportID, v1)
	r.eval(reportID, v2)
	r.eval(reportID, 0)
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("third sweep: %v", err)
	}
	msgs = sender.messages()
	if len(msgs) != 3 {
		t.Fatalf("messages after the confirmation = %d, want 3", len(msgs))
	}
	update := msgs[2]
	if update.sub.Endpoint != readerEndpoint {
		t.Errorf("the confirmed update went to the wrong device")
	}
	up := update.msg.Payload
	if up.Kind != "confirmed_update" || up.Tag != wantTag {
		t.Errorf("update envelope = %+v, want the same tag %q", up, wantTag)
	}
	if up.Title != "Fire near Central Market" || up.Body != "Confirmed by people nearby. "+wantDistance {
		t.Errorf("update words = %q / %q", up.Title, up.Body)
	}
	if update.msg.Urgency != push.UrgencyNormal {
		t.Errorf("update urgency = %q", update.msg.Urgency)
	}
	if state, _, _ := pwOutboxRow(t, r, readerID, reportID, "confirmed_update"); state != "sent" {
		t.Errorf("the reader's confirmed update is %q, want sent", state)
	}
	if state, skip, _ := pwOutboxRow(t, r, appleID, reportID, "confirmed_update"); state != "skipped" || skip != "no_device" {
		t.Errorf("the Apple reader's confirmed update is %q (%q), want skipped as no_device", state, skip)
	}
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("fourth sweep: %v", err)
	}
	if n := len(sender.messages()); n != 3 {
		t.Errorf("messages after the fourth sweep = %d, want still 3", n)
	}
	// The stored columns that held words: none of the outbox holds a title.
	var holdsWords int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM push_outbox WHERE row_to_json(push_outbox)::text ILIKE '%Central Market%'`).Scan(&holdsWords); err != nil {
		t.Fatal(err)
	}
	if holdsWords != 0 {
		t.Errorf("%d outbox rows store words", holdsWords)
	}
}

// TestWorkerLeaseNeverDoubleSends proves, on a real database, that a second
// claimer never takes a row the first worker is sending, and that a worker whose
// claim lease has passed and been taken over sends nothing and marks nothing
// (review W2).
func TestWorkerLeaseNeverDoubleSends(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const reporterSession = "p33-11-lease-reporter"
	testutil.SeedReporterAccount(t, r.pool, r.email("reporter"), reporterSession)
	for _, name := range []string{"a", "b", "c"} {
		r.reader(name, 1, 0, true, false, false)
	}

	r.cur = r.base.Add(10 * time.Minute)
	reportID := r.report(reporterSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
	r.eval(reportID, 0)

	var ordered []int64
	rows, err := r.pool.Query(ctx, `SELECT id, account_id FROM push_outbox WHERE report_id = $1 ORDER BY id`, reportID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, acct int64
		if err := rows.Scan(&id, &acct); err != nil {
			t.Fatal(err)
		}
		ordered = append(ordered, id)
	}
	rows.Close()
	if len(ordered) != 3 {
		t.Fatalf("queued rows = %d, want 3", len(ordered))
	}

	leaseOf := func(id int64) time.Time {
		var lease *time.Time
		if err := r.pool.QueryRow(ctx, `SELECT lease_until FROM push_outbox WHERE id = $1`, id).Scan(&lease); err != nil {
			t.Fatal(err)
		}
		if lease == nil {
			return time.Time{}
		}
		return *lease
	}

	var oldLease2 time.Time
	var secondClaim []sqlcgen.ClaimDuePushesRow
	sender := &pwSender{answers: map[string]pwAnswer{}}
	sender.hook = func(n int) {
		if n != 1 {
			return
		}
		// The slow send. A second worker claims now.
		got, err := r.q.ClaimDuePushes(ctx, sqlcgen.ClaimDuePushesParams{BatchLimit: PushClaimBatch, LeaseSeconds: PushLeaseSeconds})
		if err != nil {
			t.Errorf("second claim: %v", err)
			return
		}
		if len(got) != 0 {
			t.Errorf("a second claimer took %d rows while the first worker holds them", len(got))
		}
		// A batch that outlived its lease: rows 2 and 3 lapse.
		oldLease2 = leaseOf(ordered[1])
		if _, err := r.pool.Exec(ctx, `UPDATE push_outbox SET lease_until = now() - interval '1 minute' WHERE id = ANY($1)`, []int64{ordered[1], ordered[2]}); err != nil {
			t.Errorf("aging the leases: %v", err)
			return
		}
		secondClaim, err = r.q.ClaimDuePushes(ctx, sqlcgen.ClaimDuePushesParams{BatchLimit: PushClaimBatch, LeaseSeconds: PushLeaseSeconds})
		if err != nil {
			t.Errorf("second claim after the lapse: %v", err)
			return
		}
		if len(secondClaim) != 2 || secondClaim[0].ID != ordered[1] || secondClaim[1].ID != ordered[2] {
			t.Errorf("the second claim got %+v, want rows %d and %d", secondClaim, ordered[1], ordered[2])
		}
	}

	worker := pwDBWorker(r, sender)
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}

	if n := len(sender.messages()); n != 1 {
		t.Fatalf("messages sent = %d, want exactly 1", n)
	}
	state := func(id int64) string {
		var s string
		if err := r.pool.QueryRow(ctx, `SELECT state FROM push_outbox WHERE id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if s := state(ordered[0]); s != "sent" {
		t.Errorf("row 1 is %q, want sent", s)
	}
	for i, id := range ordered[1:] {
		if s := state(id); s != "queued" {
			t.Errorf("row %d is %q, want still queued for the second claimer", i+2, s)
		}
		if len(secondClaim) == 2 {
			if got := leaseOf(id); !got.Equal(secondClaim[i].LeaseUntil) {
				t.Errorf("row %d lease = %v, want the second claimer's %v", i+2, got, secondClaim[i].LeaseUntil)
			}
		}
	}

	// The first worker's old lease no longer marks anything.
	n, err := r.q.MarkPushSent(ctx, sqlcgen.MarkPushSentParams{Now: r.cur, ID: ordered[1], HeldLease: oldLease2})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("MarkPushSent with the lapsed lease changed %d rows", n)
	}
	if s := state(ordered[1]); s != "queued" {
		t.Errorf("row 2 is %q after the stale mark", s)
	}
}
