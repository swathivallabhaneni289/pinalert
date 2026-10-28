package service_test

// These tests run the official push worker (plan 05-16) on a real database with
// the real queries: rows are queued by the real push hook of plan 05-11 inside a
// transaction, the worker claims, re-checks and marks them under real leases, and
// a fake push service answers. No real push service is contacted. The package is
// service_test because internal/official imports internal/service. The full
// chain from a real SACHET refresh to a sent push is in
// internal/official/pushsend_db_test.go, which can reach the refresher's fake
// upstream.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/areas"
	"pinalert/internal/official"
	"pinalert/internal/push"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	dbEndpointFCM   = "https://fcm.googleapis.com/fcm/send/odb-token-secret-1"
	dbEndpointApple = "https://web.push.apple.com/odb-apple-token-secret"
)

// dbSender is a push.Sender that records every message and answers 201.
type dbSender struct {
	mu   sync.Mutex
	sent []dbSent
}

type dbSent struct {
	endpoint string
	msg      push.Message
}

func (s *dbSender) Send(_ context.Context, sub push.Subscription, msg push.Message) (push.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, dbSent{endpoint: sub.Endpoint, msg: msg})
	return push.Result{Status: 201}, nil
}

func (s *dbSender) messages() []dbSent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]dbSent(nil), s.sent...)
}

func (s *dbSender) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = nil
}

// dbInteriorCell finds a precision 5 cell inside exactly one district.
func dbInteriorCell(t *testing.T, ix *areas.Index, code int) string {
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

// dbWorld is one subtest's database, people, clock and worker.
type dbWorld struct {
	t      *testing.T
	pool   *pgxpool.Pool
	q      *sqlcgen.Queries
	ix     *areas.Index
	now    time.Time
	cell   string
	sender *dbSender
	worker *service.OfficialPushWorker
	n      int
}

func newDBWorld(t *testing.T, pool *pgxpool.Pool) *dbWorld {
	t.Helper()
	testutil.Truncate(t, pool)
	ix := areas.Default()
	w := &dbWorld{t: t, pool: pool, q: sqlcgen.New(pool), ix: ix, now: time.Now().UTC(), cell: dbInteriorCell(t, ix, 749), sender: &dbSender{}}
	w.worker = service.NewOfficialPushWorker(w.q, w.sender, ix, service.WithOfficialPushWorkerClock(func() time.Time { return w.now }))
	// A fresh feed: the worker's currency check reads the real sachet row.
	testutil.SetOfficialFeedStatus(t, pool, "sachet", &w.now, &w.now, 1)
	return w
}

// person seeds an account with the given device endpoints and a home cell in 749.
func (w *dbWorld) person(label string, endpoints ...string) int64 {
	w.t.Helper()
	w.n++
	email := label + "-odb@example.com"
	id := testutil.SeedReporterAccount(w.t, w.pool, email, label+"-odb-sess-0")
	for i, ep := range endpoints {
		testutil.SeedPushSubscription(w.t, w.pool, id, label+"-odb-sess-"+string(rune('0'+i)), ep)
	}
	testutil.SetHomeCell(w.t, w.pool, id, "Home", w.cell)
	return id
}

// alert seeds one stored alert on district 749 and returns its id and version.
func (w *dbWorld) alert(guid, severity, msgType string, refs []string) (int64, official.SavedVersion) {
	w.t.Helper()
	d, _ := w.ix.ByCode(749)
	sent := w.now.Add(-10 * time.Minute)
	expires := w.now.Add(3 * time.Hour)
	capID := "IN-" + guid + "_1"
	id := testutil.SeedOfficialAlert(w.t, w.pool, testutil.OfficialAlertSeed{
		GUID: guid, CapIdentifier: capID, Sender: "IMD-Chennai", MsgType: msgType, Severity: severity,
		Event: "Severe Cyclonic Storm", Tier: "district", DistrictCodes: []int{749}, StateCodes: []int{d.StateCode},
		SentAt: sent, ExpiresAt: expires,
	})
	if len(refs) > 0 {
		testutil.MustExec(w.t, w.pool, `UPDATE official_alerts SET references_text = $2 WHERE id = $1`, id, strings.Join(refs, " "))
	}
	sev := official.SeverityExtreme
	if severity == "Severe" {
		sev = official.SeveritySevere
	}
	v := official.Version{
		GUID: guid, CapIdentifier: capID, Sender: "IMD-Chennai", IssuerLabel: "IMD Chennai",
		SentAt: sent, Status: "Actual", Scope: "Public", MsgType: official.MsgType(msgType), References: refs,
		Event: "Severe Cyclonic Storm", Severity: sev, ExpiresAt: expires, SourcePubDate: sent,
	}
	return id, official.SavedVersion{
		ID: id, Version: v,
		Placement: official.Placement{Tier: official.TierDistrict, DistrictCodes: []int{749}, StateCodes: []int{d.StateCode}},
	}
}

// queue runs the real push hook of plan 05-11 in a transaction, as the refresh
// does for a stored version.
func (w *dbWorld) queue(saved official.SavedVersion) {
	w.t.Helper()
	ctx := context.Background()
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		w.t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if err := official.PushEnqueueHook(w.ix)(ctx, w.q.WithTx(tx), saved, nil, w.now); err != nil {
		w.t.Fatalf("hook: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		w.t.Fatalf("commit: %v", err)
	}
}

func (w *dbWorld) sweep() {
	w.t.Helper()
	if err := w.worker.SweepOnce(context.Background()); err != nil {
		w.t.Fatalf("SweepOnce: %v", err)
	}
}

func (w *dbWorld) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("counting with %q: %v", query, err)
	}
	return n
}

// rowFor returns the state and skip reason of the outbox row of an account and
// a cap identifier.
func (w *dbWorld) rowFor(account int64, capID string) (state, skip string) {
	w.t.Helper()
	var reason *string
	if err := w.pool.QueryRow(context.Background(),
		`SELECT state, skip_reason FROM official_push_outbox WHERE account_id = $1 AND cap_identifier = $2`,
		account, capID).Scan(&state, &reason); err != nil {
		w.t.Fatalf("reading the outbox row of account %d %s: %v", account, capID, err)
	}
	if reason != nil {
		skip = *reason
	}
	return state, skip
}

func TestOfficialWorkerOnRealOutbox(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("a queued alert reaches the phone once, silently, and nothing else changes", func(t *testing.T) {
		w := newDBWorld(t, pool)
		person := w.person("one", dbEndpointFCM)
		_, saved := w.alert("1792000000000601", "Extreme", "Alert", nil)
		w.queue(saved)
		if n := w.count(`SELECT count(*) FROM official_push_outbox WHERE state = 'queued'`); n != 1 {
			t.Fatalf("%d rows queued by the real hook, want 1", n)
		}
		reports := w.count(`SELECT count(*) FROM reports`)
		votes := w.count(`SELECT count(*) FROM votes`)
		sessions := w.count(`SELECT count(*) FROM sessions`)

		w.sweep()
		msgs := w.sender.messages()
		if len(msgs) != 1 || msgs[0].endpoint != dbEndpointFCM {
			t.Fatalf("messages = %+v, want one to the person's device", msgs)
		}
		d, _ := w.ix.ByCode(749)
		p := msgs[0].msg.Payload
		if p.Kind != "official" || p.URL != "/alerts" || p.Title != "Cyclone warning for "+d.Name+" district" || p.Body != "Extreme. Issued by IMD." {
			t.Errorf("payload = %+v", p)
		}
		if msgs[0].msg.Urgency != push.UrgencyHigh || msgs[0].msg.TTL != service.OfficialPushTTL {
			t.Errorf("urgency %v ttl %v, want high and %v", msgs[0].msg.Urgency, msgs[0].msg.TTL, service.OfficialPushTTL)
		}
		if state, _ := w.rowFor(person, "IN-1792000000000601_1"); state != "sent" {
			t.Errorf("row state = %q, want sent", state)
		}

		w.sweep()
		if n := len(w.sender.messages()); n != 1 {
			t.Errorf("%d messages after a second sweep, want still 1", n)
		}
		if w.count(`SELECT count(*) FROM reports`) != reports || w.count(`SELECT count(*) FROM votes`) != votes ||
			w.count(`SELECT count(*) FROM sessions`) != sessions {
			t.Error("the reports, votes or sessions counts changed")
		}
	})

	t.Run("the real sachet status row decides currency when the alert was not seen lately", func(t *testing.T) {
		w := newDBWorld(t, pool)
		a := w.person("first", dbEndpointFCM)
		w.person("second", dbEndpointFCM+"-2")
		id, saved := w.alert("1792000000000602", "Extreme", "Alert", nil)
		w.queue(saved)
		testutil.MustExec(t, pool, `UPDATE official_alerts SET last_seen_at = $2 WHERE id = $1`, id, w.now.Add(-2*time.Hour))

		// The feed status says a success five minutes ago: every row is sent.
		recent := w.now.Add(-5 * time.Minute)
		testutil.SetOfficialFeedStatus(t, pool, "sachet", &recent, &recent, 1)
		w.sweep()
		if n := len(w.sender.messages()); n != 2 {
			t.Fatalf("%d messages with a fresh feed, want 2", n)
		}

		// A second alert, queued while the feed is old, is dropped.
		w.sender.reset()
		old := w.now.Add(-45 * time.Minute)
		testutil.SetOfficialFeedStatus(t, pool, "sachet", &old, &old, 1)
		id2, saved2 := w.alert("1792000000000603", "Extreme", "Alert", nil)
		w.queue(saved2)
		testutil.MustExec(t, pool, `UPDATE official_alerts SET last_seen_at = $2 WHERE id = $1`, id2, w.now.Add(-2*time.Hour))
		w.sweep()
		if n := len(w.sender.messages()); n != 0 {
			t.Fatalf("%d messages with an old feed and an old alert, want 0", n)
		}
		if _, skip := w.rowFor(a, "IN-1792000000000603_1"); skip != "feed_not_fresh" {
			t.Errorf("skip reason = %q, want feed_not_fresh", skip)
		}
	})

	t.Run("one alert series keeps one topic and tag and an Apple device gets only the first push", func(t *testing.T) {
		w := newDBWorld(t, pool)
		both := w.person("both", dbEndpointFCM, dbEndpointApple)
		appleOnly := w.person("apple", dbEndpointApple+"-2")
		_, a := w.alert("1792000000000611", "Severe", "Alert", nil)
		w.queue(a)
		w.sweep()
		first := w.sender.messages()
		if len(first) != 3 {
			t.Fatalf("%d first messages, want 3 (every device gets the first push)", len(first))
		}
		w.sender.reset()

		// SACHET sends an Update under a new guid that names the earlier message.
		_, b := w.alert("1792000000000612", "Extreme", "Update", []string{"IN-1792000000000611_1"})
		w.queue(b)
		if n := w.count(`SELECT count(*) FROM official_push_outbox WHERE state = 'queued' AND reason = 'escalated'`); n != 2 {
			t.Fatalf("%d escalated rows queued, want 2", n)
		}
		w.sweep()
		msgs := w.sender.messages()
		if len(msgs) != 1 || msgs[0].endpoint != dbEndpointFCM {
			t.Fatalf("escalation messages = %+v, want one to the Chrome device only", msgs)
		}
		if msgs[0].msg.Topic != service.OfficialTopic("1792000000000611") || msgs[0].msg.Payload.Tag != service.OfficialTag("1792000000000611") {
			t.Errorf("topic %q tag %q, want the ones of the series root guid", msgs[0].msg.Topic, msgs[0].msg.Payload.Tag)
		}
		if state, _ := w.rowFor(both, "IN-1792000000000612_1"); state != "sent" {
			t.Errorf("the Chrome and Apple account's escalated row is %q, want sent", state)
		}
		if state, skip := w.rowFor(appleOnly, "IN-1792000000000612_1"); state != "skipped" || skip != "already_notified" {
			t.Errorf("the Apple only account's escalated row is %q/%q, want skipped/already_notified", state, skip)
		}
	})

	t.Run("the switch, the cap and a deleted account are read again at send time", func(t *testing.T) {
		w := newDBWorld(t, pool)
		off := w.person("off", dbEndpointFCM)
		capped := w.person("capped", dbEndpointFCM+"-2")
		gone := w.person("gone", dbEndpointFCM+"-3")
		_, saved := w.alert("1792000000000621", "Extreme", "Alert", nil)
		w.queue(saved)

		if _, err := w.q.UpsertOfficialPushSetting(context.Background(), sqlcgen.UpsertOfficialPushSettingParams{
			AccountID: off, OfficialPushOn: false, Now: w.now}); err != nil {
			t.Fatalf("switch off: %v", err)
		}
		_, other := w.alert("1792000000000622", "Severe", "Alert", nil)
		for i := 0; i < service.OfficialPushPerAccountPer6Hours; i++ {
			testutil.MustExec(t, pool,
				`INSERT INTO official_push_outbox (account_id, alert_id, cap_identifier, reason, state, sent_at, created_at, expires_at)
				 VALUES ($1, $2, $3, 'new', 'sent', $4, $4, $5)`,
				capped, other.ID, "IN-cap-"+string(rune('a'+i)), w.now.Add(-time.Hour), w.now.Add(time.Hour))
		}
		testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, gone)

		w.sweep()
		if n := len(w.sender.messages()); n != 0 {
			t.Fatalf("%d messages sent, want none", n)
		}
		for _, tc := range []struct {
			name    string
			account int64
			reason  string
		}{
			{"switch off", off, "switched_off"},
			{"capped", capped, "cap_account"},
			{"deleted", gone, "no_device"},
		} {
			if state, skip := w.rowFor(tc.account, "IN-1792000000000621_1"); state != "skipped" || skip != tc.reason {
				t.Errorf("%s: row is %q/%q, want skipped/%s", tc.name, state, skip, tc.reason)
			}
		}
	})
}
