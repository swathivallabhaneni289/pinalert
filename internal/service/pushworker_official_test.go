// pushworker_official_test.go proves the official lane of the phone alert worker
// (plan 05-16) with a fake store and a fake push sender: that every queued row is
// read again at send time and dropped when the alert, the feed, the switch, the
// home area or the 6 hour cap no longer allows it, that the words come from the
// closed tables, that the options are right, that the answers of a push service
// are handled by the same rules as Phase 3.3, that one alert series keeps one
// Topic and one tag across guids, and that no log line or returned error holds an
// endpoint or a word of the alert. The same worker runs on a real database in
// pushworker_official_db_test.go.

package service

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/areas"
	"pinalert/internal/push"
	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	owAlertID   = int64(500)
	owGUID      = "1792000000000500"
	owCapID     = "IN-1792000000000500_1"
	owAccountID = int64(7)
	owDistrict  = 749
)

// owSkipReasons is the closed list of the CHECK in migration 00013. A fake that
// meets another word fails the test, because the real table would refuse it.
var owSkipReasons = map[string]bool{
	"not_eligible": true, "stale": true, "no_home_area": true, "out_of_area": true,
	"no_device": true, "cap_account": true, "switched_off": true,
	"feed_not_fresh": true, "already_notified": true,
}

// owQuerier is an OfficialPushWorkerQuerier that records every call.
type owQuerier struct {
	t  *testing.T
	mu sync.Mutex

	calls []string

	claimRows []sqlcgen.ClaimDueOfficialPushesRow
	staleNow  []time.Time
	pruneEdge []time.Time

	extendErr error

	alerts   map[int64]sqlcgen.OfficialAlertForPushRow
	alertErr error

	// series is the answer of OfficialAlertSeries by alert id. A missing entry
	// answers no rows, which the worker reads as the alert alone.
	series map[int64][]sqlcgen.OfficialAlertSeriesRow

	// sentSeries is the answer of OfficialPushSentForSeries.
	sentSeries bool
	seriesAsks []sqlcgen.OfficialPushSentForSeriesParams

	feed    sqlcgen.OfficialFeedStatus
	feedErr error

	setting    bool
	settingErr error

	home    *string
	homeErr error

	count       int64
	countParams []sqlcgen.CountAccountOfficialPushesInWindowParams

	subs []sqlcgen.PushSubscriptionsForAccountRow

	marks     []pwMark
	lostMark  bool
	okMarks   []int64
	deleted   []int64
	bumps     []int64
	bumpReply map[int64]sqlcgen.BumpPushSubscriptionFailureRow
}

func (q *owQuerier) rec(name string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.calls = append(q.calls, name)
}

func (q *owQuerier) callCount(name string) int {
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

func (q *owQuerier) SkipStaleOfficialPushes(_ context.Context, now time.Time) (int64, error) {
	q.rec("SkipStaleOfficialPushes")
	q.staleNow = append(q.staleNow, now)
	return 0, nil
}

func (q *owQuerier) PruneOfficialPushOutbox(_ context.Context, edge time.Time) error {
	q.rec("PruneOfficialPushOutbox")
	q.pruneEdge = append(q.pruneEdge, edge)
	return nil
}

func (q *owQuerier) ClaimDueOfficialPushes(_ context.Context, _ sqlcgen.ClaimDueOfficialPushesParams) ([]sqlcgen.ClaimDueOfficialPushesRow, error) {
	q.rec("ClaimDueOfficialPushes")
	rows := q.claimRows
	q.claimRows = nil // a second sweep finds nothing due
	return rows, nil
}

func (q *owQuerier) ExtendOfficialPushLease(_ context.Context, _ sqlcgen.ExtendOfficialPushLeaseParams) (time.Time, error) {
	q.rec("ExtendOfficialPushLease")
	if q.extendErr != nil {
		return time.Time{}, q.extendErr
	}
	return pwLease2, nil
}

func (q *owQuerier) mark(m pwMark) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.marks = append(q.marks, m)
	if q.lostMark {
		return 0, nil
	}
	return 1, nil
}

func (q *owQuerier) MarkOfficialPushSent(_ context.Context, a sqlcgen.MarkOfficialPushSentParams) (int64, error) {
	q.rec("MarkOfficialPushSent")
	return q.mark(pwMark{kind: "sent", id: a.ID, lease: a.HeldLease, at: a.Now})
}

func (q *owQuerier) MarkOfficialPushSkipped(_ context.Context, a sqlcgen.MarkOfficialPushSkippedParams) (int64, error) {
	q.rec("MarkOfficialPushSkipped")
	if !owSkipReasons[a.SkipReason] {
		q.t.Errorf("skip reason %q is not in the CHECK list of migration 00013", a.SkipReason)
	}
	return q.mark(pwMark{kind: "skipped", id: a.ID, lease: a.HeldLease, reason: a.SkipReason})
}

func (q *owQuerier) RescheduleOfficialPush(_ context.Context, a sqlcgen.RescheduleOfficialPushParams) (int64, error) {
	q.rec("RescheduleOfficialPush")
	return q.mark(pwMark{kind: "rescheduled", id: a.ID, lease: a.HeldLease, next: a.NextAttemptAt})
}

func (q *owQuerier) MarkOfficialPushFailed(_ context.Context, a sqlcgen.MarkOfficialPushFailedParams) (int64, error) {
	q.rec("MarkOfficialPushFailed")
	return q.mark(pwMark{kind: "failed", id: a.ID, lease: a.HeldLease})
}

func (q *owQuerier) OfficialAlertForPush(_ context.Context, id int64) (sqlcgen.OfficialAlertForPushRow, error) {
	q.rec("OfficialAlertForPush")
	if q.alertErr != nil {
		return sqlcgen.OfficialAlertForPushRow{}, q.alertErr
	}
	a, ok := q.alerts[id]
	if !ok {
		return sqlcgen.OfficialAlertForPushRow{}, pgx.ErrNoRows
	}
	return a, nil
}

func (q *owQuerier) OfficialAlertSeries(_ context.Context, id int64) ([]sqlcgen.OfficialAlertSeriesRow, error) {
	q.rec("OfficialAlertSeries")
	return q.series[id], nil
}

func (q *owQuerier) OfficialPushSentForSeries(_ context.Context, a sqlcgen.OfficialPushSentForSeriesParams) (bool, error) {
	q.rec("OfficialPushSentForSeries")
	q.mu.Lock()
	defer q.mu.Unlock()
	q.seriesAsks = append(q.seriesAsks, a)
	return q.sentSeries, nil
}

func (q *owQuerier) GetFeedStatus(_ context.Context, feed string) (sqlcgen.OfficialFeedStatus, error) {
	q.rec("GetFeedStatus")
	if feed != "sachet" {
		q.t.Errorf("feed status read for %q, want sachet", feed)
	}
	return q.feed, q.feedErr
}

func (q *owQuerier) GetOfficialPushSetting(_ context.Context, _ int64) (bool, error) {
	q.rec("GetOfficialPushSetting")
	return q.setting, q.settingErr
}

func (q *owQuerier) GetAccountHomeCell(_ context.Context, _ int64) (*string, error) {
	q.rec("GetAccountHomeCell")
	return q.home, q.homeErr
}

func (q *owQuerier) CountAccountOfficialPushesInWindow(_ context.Context, a sqlcgen.CountAccountOfficialPushesInWindowParams) (int64, error) {
	q.rec("CountAccountOfficialPushesInWindow")
	q.countParams = append(q.countParams, a)
	return q.count, nil
}

func (q *owQuerier) PushSubscriptionsForAccount(_ context.Context, _ int64) ([]sqlcgen.PushSubscriptionsForAccountRow, error) {
	q.rec("PushSubscriptionsForAccount")
	return q.subs, nil
}

func (q *owQuerier) MarkPushSubscriptionOK(_ context.Context, a sqlcgen.MarkPushSubscriptionOKParams) error {
	q.rec("MarkPushSubscriptionOK")
	q.mu.Lock()
	defer q.mu.Unlock()
	q.okMarks = append(q.okMarks, a.ID)
	return nil
}

func (q *owQuerier) DeletePushSubscriptionByID(_ context.Context, id int64) error {
	q.rec("DeletePushSubscriptionByID")
	q.mu.Lock()
	defer q.mu.Unlock()
	q.deleted = append(q.deleted, id)
	return nil
}

func (q *owQuerier) BumpPushSubscriptionFailure(_ context.Context, id int64) (sqlcgen.BumpPushSubscriptionFailureRow, error) {
	q.rec("BumpPushSubscriptionFailure")
	q.mu.Lock()
	defer q.mu.Unlock()
	q.bumps = append(q.bumps, id)
	if r, ok := q.bumpReply[id]; ok {
		return r, nil
	}
	return sqlcgen.BumpPushSubscriptionFailureRow{Failures: 1}, nil
}

// owCell finds a precision 5 cell whose districts are exactly the one asked for,
// so a home cell sits inside it with no edge effect.
func owCell(t *testing.T, ix *areas.Index, code int) string {
	t.Helper()
	d, ok := ix.ByCode(code)
	if !ok {
		t.Fatalf("district %d is not in the boundary file", code)
	}
	for lat := d.BBox[1]; lat <= d.BBox[3]; lat += 0.01 {
		for lon := d.BBox[0]; lon <= d.BBox[2]; lon += 0.01 {
			cell := geohash.EncodeWithPrecision(lat, lon, 5)
			if ds := ix.CellDistricts(cell); len(ds) == 1 && ds[0].Code == code {
				return cell
			}
		}
	}
	t.Fatalf("no interior cell found for district %d", code)
	return ""
}

// owRig is one test's world: a fake store, a fake push service, a worker on the
// fixed clock, and the captured log.
type owRig struct {
	t        *testing.T
	q        *owQuerier
	s        *pwSender
	w        *OfficialPushWorker
	ix       *areas.Index
	logs     *bytes.Buffer
	homeCell string
	district areas.District
}

// owAlert is the default alert: an Extreme cyclone message for district 749 from
// IMD, sent 10 minutes ago, seen 5 minutes ago and ending in 3 hours.
func owAlert(ix *areas.Index) sqlcgen.OfficialAlertForPushRow {
	d, _ := ix.ByCode(owDistrict)
	return sqlcgen.OfficialAlertForPushRow{
		ID: owAlertID, Guid: owGUID, CapIdentifier: owCapID, Severity: "Extreme",
		Event: "Severe Cyclonic Storm", Sender: "IMD-Chennai", PlacementTier: "district",
		ExpiresAt: pushFactsNow.Add(3 * time.Hour), SentAt: pushFactsNow.Add(-10 * time.Minute),
		LastSeenAt:    pushFactsNow.Add(-5 * time.Minute),
		DistrictCodes: []int32{owDistrict}, StateCodes: []int32{int32(d.StateCode)},
	}
}

func owClaim(id, alertID int64, capID, reason string) sqlcgen.ClaimDueOfficialPushesRow {
	return sqlcgen.ClaimDueOfficialPushesRow{
		ID: id, AccountID: owAccountID, AlertID: alertID, CapIdentifier: capID, Reason: reason,
		LeaseUntil: pwLease1, Attempts: 1,
		ExpiresAt: pushFactsNow.Add(2 * time.Hour), CreatedAt: pushFactsNow.Add(-5 * time.Minute),
	}
}

func owFresh(at time.Time) sqlcgen.OfficialFeedStatus {
	return sqlcgen.OfficialFeedStatus{Feed: "sachet", LastSuccessAt: pgtype.Timestamptz{Time: at, Valid: true}}
}

// newOWRig builds the default world: one new row for the default alert, a person
// whose home cell is inside district 749, the switch on, a fresh feed, no earlier
// official pushes and one Google device.
func newOWRig(t *testing.T) *owRig {
	t.Helper()
	r := &owRig{t: t, logs: &bytes.Buffer{}, ix: areas.Default()}

	prev := log.Writer()
	log.SetOutput(r.logs)
	t.Cleanup(func() { log.SetOutput(prev) })

	r.homeCell = owCell(t, r.ix, owDistrict)
	r.district, _ = r.ix.ByCode(owDistrict)

	r.q = &owQuerier{
		t:         t,
		claimRows: []sqlcgen.ClaimDueOfficialPushesRow{owClaim(1, owAlertID, owCapID, "new")},
		alerts:    map[int64]sqlcgen.OfficialAlertForPushRow{owAlertID: owAlert(r.ix)},
		series:    map[int64][]sqlcgen.OfficialAlertSeriesRow{},
		feed:      owFresh(pushFactsNow.Add(-2 * time.Minute)),
		setting:   true,
		home:      &r.homeCell,
		subs:      []sqlcgen.PushSubscriptionsForAccountRow{pwDevice(11, pwEndpointFCM, "")},
		bumpReply: map[int64]sqlcgen.BumpPushSubscriptionFailureRow{},
	}
	r.s = &pwSender{answers: map[string]pwAnswer{}}
	r.w = NewOfficialPushWorker(r.q, r.s, r.ix,
		WithOfficialPushWorkerClock(func() time.Time { return pushFactsNow }))
	r.w.jitter = func() float64 { return 0.5 }
	return r
}

func (r *owRig) alert(mut func(*sqlcgen.OfficialAlertForPushRow)) {
	r.t.Helper()
	a := r.q.alerts[owAlertID]
	mut(&a)
	r.q.alerts[owAlertID] = a
}

func (r *owRig) sweep() error {
	r.t.Helper()
	return r.w.SweepOnce(context.Background())
}

func (r *owRig) onlyMark() pwMark {
	r.t.Helper()
	if len(r.q.marks) != 1 {
		r.t.Fatalf("row marks = %+v, want exactly one", r.q.marks)
	}
	return r.q.marks[0]
}

func (r *owRig) wantSkipped(reason string) {
	r.t.Helper()
	m := r.onlyMark()
	if m.kind != "skipped" || m.reason != reason {
		r.t.Fatalf("mark = %+v, want skipped with %s", m, reason)
	}
	if n := len(r.s.messages()); n != 0 {
		r.t.Fatalf("%d messages were sent for a skipped row", n)
	}
	if m.lease != pwLease2 {
		r.t.Errorf("the mark carried lease %v, want the renewed lease %v", m.lease, pwLease2)
	}
}

func (r *owRig) onlyMessage() pwSentMessage {
	r.t.Helper()
	msgs := r.s.messages()
	if len(msgs) != 1 {
		r.t.Fatalf("%d messages sent, want 1", len(msgs))
	}
	return msgs[0]
}

func TestOfficialWorkerSendsSilentToAlerts(t *testing.T) {
	r := newOWRig(t)
	if err := r.sweep(); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	m := r.onlyMessage()
	p := m.msg.Payload
	if p.V != push.PayloadVersion || p.Kind != "official" {
		t.Errorf("payload V %d kind %q, want %d and official", p.V, p.Kind, push.PayloadVersion)
	}
	if p.Tag != OfficialTag(owGUID) || m.msg.Topic != OfficialTopic(owGUID) {
		t.Errorf("tag %q topic %q, want the ones of the alert's guid", p.Tag, m.msg.Topic)
	}
	if p.URL != "/alerts" {
		t.Errorf("url = %q, want /alerts", p.URL)
	}
	wantTitle := "Cyclone warning for " + r.district.Name + " district"
	if p.Title != wantTitle {
		t.Errorf("title = %q, want %q", p.Title, wantTitle)
	}
	if p.Body != "Extreme. Issued by IMD." {
		t.Errorf("body = %q, want Extreme. Issued by IMD.", p.Body)
	}
	if m.sub.Endpoint != pwEndpointFCM {
		t.Errorf("sent to %q, want the one device", m.sub.Endpoint)
	}
	mark := r.onlyMark()
	if mark.kind != "sent" || mark.lease != pwLease2 || mark.id != 1 {
		t.Errorf("mark = %+v, want sent with the renewed lease", mark)
	}
	if len(r.q.okMarks) != 1 || r.q.okMarks[0] != 11 {
		t.Errorf("device marks = %v, want device 11", r.q.okMarks)
	}
	// The sweep drops the rows that are over first, with the 3.3 retention.
	if len(r.q.staleNow) != 1 || !r.q.staleNow[0].Equal(pushFactsNow) {
		t.Errorf("stale skip at %v, want one call at the clock", r.q.staleNow)
	}
	if len(r.q.pruneEdge) != 1 || !r.q.pruneEdge[0].Equal(pushFactsNow.Add(-PushOutboxRetention)) {
		t.Errorf("prune edge = %v, want now minus PushOutboxRetention", r.q.pruneEdge)
	}
}

func TestOfficialWorkerColdStartRace(t *testing.T) {
	r := newOWRig(t)
	// The host slept: the row was queued by a refresh still running, whose success
	// is not marked yet. The alert row was seen a minute ago; the feed status still
	// shows a success two hours old.
	r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.LastSeenAt = pushFactsNow.Add(-time.Minute) })
	r.q.feed = owFresh(pushFactsNow.Add(-2 * time.Hour))
	if err := r.sweep(); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if got := r.onlyMark(); got.kind != "sent" {
		t.Fatalf("mark = %+v, want sent: a cold start must not drop the push it exists to send", got)
	}
	// The refresh finishes and marks its success; nothing more is due.
	r.q.feed = owFresh(pushFactsNow)
	if err := r.sweep(); err != nil {
		t.Fatalf("second SweepOnce: %v", err)
	}
	if n := len(r.s.messages()); n != 1 {
		t.Errorf("%d pushes in total, want 1", n)
	}
}

func TestOfficialWorkerRechecksAtSendTime(t *testing.T) {
	cancelled := pgtype.Timestamptz{Time: pushFactsNow.Add(-time.Minute), Valid: true}
	tests := []struct {
		name   string
		mut    func(r *owRig)
		reason string
	}{
		{"the alert is gone", func(r *owRig) { delete(r.q.alerts, owAlertID) }, "not_eligible"},
		{"cancelled", func(r *owRig) {
			r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.CancelledAt = cancelled })
		}, "stale"},
		{"superseded", func(r *owRig) {
			r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.SupersededAt = cancelled })
		}, "stale"},
		{"expired", func(r *owRig) {
			r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.ExpiresAt = pushFactsNow.Add(-time.Minute) })
		}, "stale"},
		{"ending in 9 minutes", func(r *owRig) {
			r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.ExpiresAt = pushFactsNow.Add(9 * time.Minute) })
		}, "stale"},
		{"the row's own life is over", func(r *owRig) {
			r.q.claimRows[0].ExpiresAt = pushFactsNow
		}, "stale"},
		{"the alert and the feed are both 31 minutes old", func(r *owRig) {
			r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.LastSeenAt = pushFactsNow.Add(-31 * time.Minute) })
			r.q.feed = owFresh(pushFactsNow.Add(-31 * time.Minute))
		}, "feed_not_fresh"},
		{"no feed status row and the alert is old", func(r *owRig) {
			r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.LastSeenAt = pushFactsNow.Add(-time.Hour) })
			r.q.feed = sqlcgen.OfficialFeedStatus{}
			r.q.feedErr = pgx.ErrNoRows
		}, "feed_not_fresh"},
		// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box,
		// D2): the stored official switch is no longer read, so "the switch is off"
		// moved to its own subtest below that expects the row sent, and a gone
		// account is now found by the home area read.
		{"the account is gone", func(r *owRig) { r.q.homeErr = pgx.ErrNoRows }, "not_eligible"},
		{"no home area", func(r *owRig) { r.q.home = nil }, "no_home_area"},
		{"an empty home area", func(r *owRig) { empty := ""; r.q.home = &empty }, "no_home_area"},
		{"the home is no longer in the alert's districts", func(r *owRig) {
			r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.DistrictCodes = []int32{750} })
		}, "out_of_area"},
		{"the sender is outside the closed issuer table", func(r *owRig) {
			r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.Sender = "Somebody Else" })
		}, "not_eligible"},
		{"the severity fell below Severe", func(r *owRig) {
			r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.Severity = "Moderate" })
		}, "not_eligible"},
		{"no device", func(r *owRig) { r.q.subs = nil }, "no_device"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newOWRig(t)
			tc.mut(r)
			if err := r.sweep(); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			r.wantSkipped(tc.reason)
		})
	}

	t.Run("a stored switch that says off no longer stops the push", func(t *testing.T) {
		r := newOWRig(t)
		r.q.setting = false
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if m := r.onlyMark(); m.kind != "sent" {
			t.Errorf("mark = %+v, want sent: the device switch is the only way to stop it", m)
		}
		if n := r.q.callCount("GetOfficialPushSetting"); n != 0 {
			t.Errorf("the stored switch was read %d times, want none", n)
		}
	})

	t.Run("a feed that is fresh covers an alert not seen for an hour", func(t *testing.T) {
		r := newOWRig(t)
		r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.LastSeenAt = pushFactsNow.Add(-time.Hour) })
		r.q.feed = owFresh(pushFactsNow.Add(-29 * time.Minute))
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if m := r.onlyMark(); m.kind != "sent" {
			t.Errorf("mark = %+v, want sent", m)
		}
	})
}

// TestOfficialWorkerAreaMatch pins the placement rule of plan 05-11 at send
// time: a district placed alert matches by district only, so the state rows it
// also carries do not widen it; only a state placed Extreme alert matches by
// state; an unplaced alert never matches.
func TestOfficialWorkerAreaMatch(t *testing.T) {
	r0 := newOWRig(t)
	stateCode := int32(r0.district.StateCode)
	tests := []struct {
		name   string
		mut    func(a *sqlcgen.OfficialAlertForPushRow)
		reason string // empty means sent
		area   string
	}{
		{"district tier, home district listed", func(a *sqlcgen.OfficialAlertForPushRow) {}, "", r0.district.Name + " district"},
		{"district tier, only the state matches", func(a *sqlcgen.OfficialAlertForPushRow) {
			a.DistrictCodes = []int32{750}
		}, "out_of_area", ""},
		{"state tier Extreme matches by state", func(a *sqlcgen.OfficialAlertForPushRow) {
			a.PlacementTier = "state"
			a.DistrictCodes = nil
			a.StateCodes = []int32{stateCode}
		}, "", OfficialStateWords(r0.district.State)},
		{"state tier Severe is never pushed", func(a *sqlcgen.OfficialAlertForPushRow) {
			a.PlacementTier = "state"
			a.Severity = "Severe"
			a.DistrictCodes = nil
			a.StateCodes = []int32{stateCode}
		}, "not_eligible", ""},
		{"state tier in another state", func(a *sqlcgen.OfficialAlertForPushRow) {
			a.PlacementTier = "state"
			a.DistrictCodes = nil
			a.StateCodes = []int32{stateCode + 1}
		}, "out_of_area", ""},
		{"unplaced never matches", func(a *sqlcgen.OfficialAlertForPushRow) {
			a.PlacementTier = "unplaced"
		}, "not_eligible", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newOWRig(t)
			r.alert(tc.mut)
			if err := r.sweep(); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			if tc.reason != "" {
				r.wantSkipped(tc.reason)
				return
			}
			m := r.onlyMessage()
			if !strings.HasSuffix(m.msg.Payload.Title, " for "+tc.area) {
				t.Errorf("title = %q, want it to end with %q", m.msg.Payload.Title, " for "+tc.area)
			}
		})
	}
}

func TestOfficialWorkerCapSixHours(t *testing.T) {
	t.Run("three sent in the window skips the fourth", func(t *testing.T) {
		r := newOWRig(t)
		r.q.count = 3
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		r.wantSkipped("cap_account")
		if len(r.q.countParams) != 1 {
			t.Fatalf("count asked %d times, want once", len(r.q.countParams))
		}
		if p := r.q.countParams[0]; p.AccountID != owAccountID || !p.Since.Equal(pushFactsNow.Add(-OfficialPushCapWindow)) {
			t.Errorf("count params = %+v, want the account and now minus 6 hours", p)
		}
	})
	t.Run("two sent in the window still sends", func(t *testing.T) {
		r := newOWRig(t)
		r.q.count = 2
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if m := r.onlyMark(); m.kind != "sent" {
			t.Errorf("mark = %+v, want sent", m)
		}
	})
	t.Run("the cap is the named constant", func(t *testing.T) {
		if OfficialPushPerAccountPer6Hours != 3 || OfficialPushCapWindow != 6*time.Hour {
			t.Errorf("cap %d in %v, want 3 in 6 hours", OfficialPushPerAccountPer6Hours, OfficialPushCapWindow)
		}
	})
}

func TestOfficialWorkerUrgencyAndTTL(t *testing.T) {
	tests := []struct {
		name    string
		sev     string
		expires time.Duration
		urgency push.Urgency
		ttl     time.Duration
	}{
		{"Extreme is high", "Extreme", 5 * time.Hour, push.UrgencyHigh, OfficialPushTTL},
		{"Severe is normal", "Severe", 5 * time.Hour, push.UrgencyNormal, OfficialPushTTL},
		{"never past the alert's end", "Extreme", 90 * time.Minute, push.UrgencyHigh, 90 * time.Minute},
		{"just over the margin", "Severe", 11 * time.Minute, push.UrgencyNormal, 11 * time.Minute},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newOWRig(t)
			r.alert(func(a *sqlcgen.OfficialAlertForPushRow) {
				a.Severity = tc.sev
				a.ExpiresAt = pushFactsNow.Add(tc.expires)
			})
			r.q.claimRows[0].ExpiresAt = pushFactsNow.Add(tc.expires)
			if err := r.sweep(); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			m := r.onlyMessage().msg
			if m.Urgency != tc.urgency {
				t.Errorf("urgency = %v, want %v", m.Urgency, tc.urgency)
			}
			if m.TTL != tc.ttl {
				t.Errorf("ttl = %v, want %v", m.TTL, tc.ttl)
			}
			if m.TTL > OfficialPushTTL {
				t.Errorf("ttl %v is over OfficialPushTTL", m.TTL)
			}
			if !push.ValidTopic(m.Topic) {
				t.Errorf("topic %q is not valid", m.Topic)
			}
			if m.Topic != OfficialTopic(owGUID) {
				t.Errorf("topic = %q, want the alert's", m.Topic)
			}
		})
	}
}

// TestOfficialWorkerAnswers pins the per device answer table on the official
// lane: the same rules as 3.3's TestWorkerResponses, because both lanes call
// handleDeviceAnswers.
func TestOfficialWorkerAnswers(t *testing.T) {
	type want struct {
		mark    string
		deleted bool
		bumped  bool
		okMark  bool
	}
	tests := []struct {
		name   string
		answer pwAnswer
		reply  *sqlcgen.BumpPushSubscriptionFailureRow
		want   want
	}{
		{"201 is fine", pwAnswer{status: 201}, nil, want{mark: "sent", okMark: true}},
		{"404 deletes the device", pwAnswer{status: 404}, nil, want{mark: "failed", deleted: true}},
		{"410 deletes the device", pwAnswer{status: 410}, nil, want{mark: "failed", deleted: true}},
		{"429 backs off with no failure count", pwAnswer{status: 429, retryAfter: 2 * time.Minute}, nil, want{mark: "rescheduled"}},
		{"500 backs off with no failure count", pwAnswer{status: 500}, nil, want{mark: "rescheduled"}},
		{"403 counts one failure", pwAnswer{status: 403}, nil, want{mark: "failed", bumped: true}},
		{"403 fifth time on a device never successful deletes it", pwAnswer{status: 403},
			&sqlcgen.BumpPushSubscriptionFailureRow{Failures: PushDeviceFailureLimit}, want{mark: "failed", bumped: true, deleted: true}},
		{"403 fifth time on a device that worked last week keeps it", pwAnswer{status: 403},
			&sqlcgen.BumpPushSubscriptionFailureRow{Failures: PushDeviceFailureLimit, LastOkAt: pgtype.Timestamptz{Time: pushFactsNow.Add(-24 * time.Hour), Valid: true}},
			want{mark: "failed", bumped: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newOWRig(t)
			r.s.answers[pwEndpointFCM] = tc.answer
			if tc.reply != nil {
				r.q.bumpReply[11] = *tc.reply
			}
			if err := r.sweep(); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			m := r.onlyMark()
			if m.kind != tc.want.mark {
				t.Fatalf("mark = %+v, want %s", m, tc.want.mark)
			}
			if got := len(r.q.deleted) == 1 && r.q.deleted[0] == 11; got != tc.want.deleted {
				t.Errorf("deleted = %v, want deleted %v", r.q.deleted, tc.want.deleted)
			}
			if got := len(r.q.bumps) == 1; got != tc.want.bumped {
				t.Errorf("bumps = %v, want bumped %v", r.q.bumps, tc.want.bumped)
			}
			if got := len(r.q.okMarks) == 1; got != tc.want.okMark {
				t.Errorf("device ok marks = %v, want %v", r.q.okMarks, tc.want.okMark)
			}
			if m.kind == "rescheduled" {
				wantWait := PushBackoff(1, tc.answer.retryAfter, 1.0)
				if !m.next.Equal(pushFactsNow.Add(wantWait)) {
					t.Errorf("next attempt %v, want now + %v", m.next, wantWait)
				}
			}
		})
	}

	t.Run("the fourth failed claim fails the row", func(t *testing.T) {
		r := newOWRig(t)
		r.q.claimRows[0].Attempts = PushMaxAttempts
		r.s.answers[pwEndpointFCM] = pwAnswer{status: 500}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if m := r.onlyMark(); m.kind != "failed" {
			t.Errorf("mark = %+v, want failed", m)
		}
	})

	t.Run("no retry is set past the end of the row", func(t *testing.T) {
		r := newOWRig(t)
		r.q.claimRows[0].ExpiresAt = pushFactsNow.Add(5 * time.Second)
		r.s.answers[pwEndpointFCM] = pwAnswer{status: 500}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if m := r.onlyMark(); m.kind != "failed" {
			t.Errorf("mark = %+v, want failed because the backoff passes the row's end", m)
		}
	})
}

func TestOfficialWorkerStopsOnLostLease(t *testing.T) {
	r := newOWRig(t)
	r.q.extendErr = pgx.ErrNoRows
	if err := r.sweep(); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if len(r.q.marks) != 0 || len(r.s.messages()) != 0 {
		t.Errorf("marks %+v and %d messages for a row whose lease was lost, want none", r.q.marks, len(r.s.messages()))
	}
	if n := r.q.callCount("OfficialAlertForPush"); n != 0 {
		t.Errorf("the alert was read %d times after the lease was lost, want 0", n)
	}
	if !strings.Contains(r.logs.String(), "lease_lost") {
		t.Errorf("log %q does not say lease_lost", r.logs.String())
	}

	t.Run("a mark that changes no row is only logged", func(t *testing.T) {
		r := newOWRig(t)
		r.q.lostMark = true
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if !strings.Contains(r.logs.String(), "lease_lost") {
			t.Errorf("log %q does not say lease_lost", r.logs.String())
		}
	})
}

func TestOfficialWorkerNeverLogsSecrets(t *testing.T) {
	r := newOWRig(t)
	secretErr := &url.Error{Op: "Post", URL: pwEndpointFCM, Err: errors.New("connection refused: " + pwEndpointFCM)}
	r.s.answers[pwEndpointFCM] = pwAnswer{err: secretErr}
	err := r.sweep()
	seen := r.logs.String()
	if err != nil {
		seen += err.Error()
	}
	for _, secret := range []string{
		"pw-token-secret", "fcm/send", pwP256dh, pwAuth,
		"Cyclone", "Extreme", "IMD", strings.ToLower(r.district.Name), r.district.Name,
		owGUID, OfficialTopic(owGUID), OfficialTag(owGUID), owCapID, "Severe",
	} {
		if strings.Contains(strings.ToLower(seen), strings.ToLower(secret)) {
			t.Errorf("the log or the error holds %q: %q", secret, seen)
		}
	}
	if !strings.Contains(seen, "host fcm.googleapis.com") {
		t.Errorf("the log %q should name the push service host", seen)
	}
}

func TestOfficialWorkerEscalationSameTopic(t *testing.T) {
	chrome := pwDevice(11, pwEndpointFCM, "")
	apple := pwDevice(12, pwEndpointApple, "")

	t.Run("first push goes to every device", func(t *testing.T) {
		r := newOWRig(t)
		r.q.subs = []sqlcgen.PushSubscriptionsForAccountRow{chrome, apple}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if n := len(r.s.messages()); n != 2 {
			t.Errorf("%d devices got the first push, want 2", n)
		}
		if n := r.q.callCount("OfficialPushSentForSeries"); n != 1 {
			t.Errorf("series sent check ran %d times, want once (an Apple device is in the list)", n)
		}
	})

	t.Run("a Chrome only account never asks the series check", func(t *testing.T) {
		r := newOWRig(t)
		r.q.sentSeries = true
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if n := r.q.callCount("OfficialPushSentForSeries"); n != 0 {
			t.Errorf("series sent check ran %d times with no Apple device, want 0", n)
		}
		if len(r.s.messages()) != 1 {
			t.Errorf("%d messages, want 1", len(r.s.messages()))
		}
	})

	t.Run("an escalation reaches Chrome with the same topic and tag but not Apple", func(t *testing.T) {
		r := newOWRig(t)
		r.q.subs = []sqlcgen.PushSubscriptionsForAccountRow{chrome, apple}
		if err := r.sweep(); err != nil {
			t.Fatalf("first SweepOnce: %v", err)
		}
		first := r.s.messages()[0].msg
		r.s.mu.Lock()
		r.s.sent = nil
		r.s.mu.Unlock()
		r.q.marks = nil
		r.q.sentSeries = true // the first push was sent
		r.q.claimRows = []sqlcgen.ClaimDueOfficialPushesRow{owClaim(2, owAlertID, "IN-1792000000000500_2", "escalated")}
		r.alert(func(a *sqlcgen.OfficialAlertForPushRow) { a.Severity = "Extreme" })
		if err := r.sweep(); err != nil {
			t.Fatalf("second SweepOnce: %v", err)
		}
		msgs := r.s.messages()
		if len(msgs) != 1 || msgs[0].sub.Endpoint != pwEndpointFCM {
			t.Fatalf("escalation messages = %+v, want one to the Chrome device only", msgs)
		}
		if msgs[0].msg.Topic != first.Topic || msgs[0].msg.Payload.Tag != first.Payload.Tag {
			t.Errorf("escalation topic %q tag %q differ from the first %q %q", msgs[0].msg.Topic, msgs[0].msg.Payload.Tag, first.Topic, first.Payload.Tag)
		}
		if m := r.onlyMark(); m.kind != "sent" {
			t.Errorf("mark = %+v, want sent", m)
		}
	})

	t.Run("an Apple only account gets the escalated row skipped as already notified", func(t *testing.T) {
		r := newOWRig(t)
		r.q.subs = []sqlcgen.PushSubscriptionsForAccountRow{apple}
		r.q.sentSeries = true
		r.q.claimRows = []sqlcgen.ClaimDueOfficialPushesRow{owClaim(2, owAlertID, "IN-1792000000000500_2", "escalated")}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		r.wantSkipped("already_notified")
	})

	t.Run("an escalated row with no earlier sent row reaches its Apple device", func(t *testing.T) {
		r := newOWRig(t)
		r.q.subs = []sqlcgen.PushSubscriptionsForAccountRow{apple}
		r.q.sentSeries = false // the first one was skipped by the cap
		r.q.claimRows = []sqlcgen.ClaimDueOfficialPushesRow{owClaim(2, owAlertID, "IN-1792000000000500_2", "escalated")}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		msgs := r.s.messages()
		if len(msgs) != 1 || msgs[0].sub.Endpoint != pwEndpointApple {
			t.Fatalf("messages = %+v, want one to the Apple device", msgs)
		}
	})

	t.Run("the host comes from the policy, never from a stored column", func(t *testing.T) {
		r := newOWRig(t)
		// The stored column lies: it says Google for an Apple endpoint.
		r.q.subs = []sqlcgen.PushSubscriptionsForAccountRow{pwDevice(12, pwEndpointApple, "fcm.googleapis.com")}
		r.q.sentSeries = true
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		r.wantSkipped("already_notified")
	})
}

func TestOfficialWorkerSeriesAcrossGuids(t *testing.T) {
	const (
		idA, idB, idC = int64(500), int64(501), int64(502)
		gA            = "1792000000000500"
		gB            = "1792000000000501"
		gC            = "1792000000000502"
	)
	t0 := pushFactsNow.Add(-3 * time.Hour)
	chrome := pwDevice(11, pwEndpointFCM, "")
	apple := pwDevice(12, pwEndpointApple, "")

	build := func(t *testing.T, subs ...sqlcgen.PushSubscriptionsForAccountRow) *owRig {
		r := newOWRig(t)
		a := owAlert(r.ix)
		b, c := a, a
		b.ID, b.Guid, b.CapIdentifier = idB, gB, "IN-"+gB+"_1"
		c.ID, c.Guid, c.CapIdentifier = idC, gC, "IN-"+gC+"_1"
		r.q.alerts = map[int64]sqlcgen.OfficialAlertForPushRow{idA: a, idB: b, idC: c}
		// A second walk order on purpose: the worker must not rely on the order.
		r.q.series[idB] = []sqlcgen.OfficialAlertSeriesRow{
			{ID: idB, Guid: gB, SentAt: t0.Add(time.Hour), NamesMember: true},
			{ID: idA, Guid: gA, SentAt: t0},
		}
		r.q.series[idC] = []sqlcgen.OfficialAlertSeriesRow{
			{ID: idB, Guid: gB, SentAt: t0.Add(time.Hour), NamesMember: true},
			{ID: idC, Guid: gC, SentAt: t0.Add(2 * time.Hour), NamesMember: true},
			{ID: idA, Guid: gA, SentAt: t0},
		}
		r.q.subs = subs
		return r
	}
	ids := func(p sqlcgen.OfficialPushSentForSeriesParams) []int64 {
		out := append([]int64(nil), p.AlertIds...)
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}
	same := func(a, b []int64) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	t.Run("an Update under a new guid keeps the root's topic and tag and leaves Apple out", func(t *testing.T) {
		r := build(t, chrome, apple)
		r.q.sentSeries = true
		r.q.claimRows = []sqlcgen.ClaimDueOfficialPushesRow{owClaim(2, idB, "IN-"+gB+"_1", "escalated")}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		msgs := r.s.messages()
		if len(msgs) != 1 || msgs[0].sub.Endpoint != pwEndpointFCM {
			t.Fatalf("messages = %+v, want one to the Chrome device only", msgs)
		}
		if msgs[0].msg.Topic != OfficialTopic(gA) || msgs[0].msg.Payload.Tag != OfficialTag(gA) {
			t.Errorf("topic %q tag %q, want the ones of the root guid g1", msgs[0].msg.Topic, msgs[0].msg.Payload.Tag)
		}
		if msgs[0].msg.Topic == OfficialTopic(gB) {
			t.Error("the topic came from the Update's own guid")
		}
		if len(r.q.seriesAsks) != 1 || !same(ids(r.q.seriesAsks[0]), []int64{idA, idB}) || r.q.seriesAsks[0].AccountID != owAccountID {
			t.Errorf("series asks = %+v, want the account and alert ids %d and %d", r.q.seriesAsks, idA, idB)
		}
	})

	t.Run("an Apple only account is skipped as already notified", func(t *testing.T) {
		r := build(t, apple)
		r.q.sentSeries = true
		r.q.claimRows = []sqlcgen.ClaimDueOfficialPushesRow{owClaim(2, idB, "IN-"+gB+"_1", "escalated")}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		r.wantSkipped("already_notified")
	})

	t.Run("A to B to C still resolves to the root g1", func(t *testing.T) {
		r := build(t, chrome, apple)
		r.q.sentSeries = true
		r.q.claimRows = []sqlcgen.ClaimDueOfficialPushesRow{owClaim(3, idC, "IN-"+gC+"_1", "escalated")}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		msgs := r.s.messages()
		if len(msgs) != 1 || msgs[0].msg.Topic != OfficialTopic(gA) || msgs[0].msg.Payload.Tag != OfficialTag(gA) {
			t.Fatalf("messages = %+v, want one under the root g1", msgs)
		}
		if len(r.q.seriesAsks) != 1 || !same(ids(r.q.seriesAsks[0]), []int64{idA, idB, idC}) {
			t.Errorf("series asks = %+v, want all three alert ids", r.q.seriesAsks)
		}
	})

	t.Run("a cycle falls back to the earliest sent, then the lowest id", func(t *testing.T) {
		r := build(t, chrome)
		r.q.series[idC] = []sqlcgen.OfficialAlertSeriesRow{
			{ID: idC, Guid: gC, SentAt: t0.Add(time.Hour), NamesMember: true},
			{ID: idB, Guid: gB, SentAt: t0, NamesMember: true},
			{ID: idA, Guid: gA, SentAt: t0, NamesMember: true},
		}
		r.q.claimRows = []sqlcgen.ClaimDueOfficialPushesRow{owClaim(3, idC, "IN-"+gC+"_1", "new")}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		msgs := r.s.messages()
		if len(msgs) != 1 || msgs[0].msg.Topic != OfficialTopic(gA) {
			t.Fatalf("messages = %+v, want the root of the earliest sent_at and lowest id (g1)", msgs)
		}
	})

	t.Run("several roots take the earliest sent_at, then the lowest id", func(t *testing.T) {
		r := build(t, chrome)
		r.q.series[idC] = []sqlcgen.OfficialAlertSeriesRow{
			{ID: idC, Guid: gC, SentAt: t0.Add(2 * time.Hour), NamesMember: true},
			{ID: idB, Guid: gB, SentAt: t0.Add(30 * time.Minute)},
			{ID: idA, Guid: gA, SentAt: t0.Add(time.Hour)},
		}
		r.q.claimRows = []sqlcgen.ClaimDueOfficialPushesRow{owClaim(3, idC, "IN-"+gC+"_1", "new")}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		msgs := r.s.messages()
		if len(msgs) != 1 || msgs[0].msg.Topic != OfficialTopic(gB) {
			t.Fatalf("messages = %+v, want the earlier sent root (g2)", msgs)
		}
	})

	t.Run("an alert that is its own series keeps its own guid", func(t *testing.T) {
		r := build(t, chrome)
		r.q.claimRows = []sqlcgen.ClaimDueOfficialPushesRow{owClaim(1, idA, "IN-"+gA+"_1", "new")}
		if err := r.sweep(); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		msgs := r.s.messages()
		if len(msgs) != 1 || msgs[0].msg.Topic != OfficialTopic(gA) {
			t.Fatalf("messages = %+v, want the alert's own guid", msgs)
		}
	})
}

func TestOfficialWorkerConstants(t *testing.T) {
	if OfficialPushFreshWindow != 30*time.Minute {
		t.Errorf("OfficialPushFreshWindow = %v, want 30 minutes", OfficialPushFreshWindow)
	}
}

// compile time check that the fake satisfies the interface the worker takes.
var _ OfficialPushWorkerQuerier = (*owQuerier)(nil)
