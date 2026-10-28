package official

// This test runs the whole phone chain of plan 05-16 on a real database: a real
// refresh of an Extreme cyclone alert from the fake SACHET upstream through the
// refresher, the real push hook of plan 05-11 queuing inside the version's
// transaction, and the official push worker of internal/service sending to a fake
// push service. It lives in this package because the fake upstream and the
// refresher's test seams are here, and this package may import internal/service
// (the reverse is not possible). No real push service or SACHET host is reached.

import (
	"context"
	"sync"
	"testing"

	"pinalert/internal/areas"
	"pinalert/internal/push"
	"pinalert/internal/service"
	"pinalert/internal/testutil"
)

// recordingSender is a push.Sender that records every message and answers 201.
type recordingSender struct {
	mu   sync.Mutex
	sent []sentPush
}

type sentPush struct {
	endpoint string
	msg      push.Message
}

func (s *recordingSender) Send(_ context.Context, sub push.Subscription, msg push.Message) (push.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, sentPush{endpoint: sub.Endpoint, msg: msg})
	return push.Result{Status: 201}, nil
}

func (s *recordingSender) messages() []sentPush {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sentPush(nil), s.sent...)
}

func TestOfficialPushSendsAfterRealRefresh(t *testing.T) {
	// The worker reads the feed row by this name; the service package cannot import
	// FeedSachet, so the name is pinned here.
	if string(FeedSachet) != "sachet" {
		t.Fatalf("FeedSachet = %q, the push worker reads the status row %q", FeedSachet, "sachet")
	}

	ix := areas.Default()
	e := newOfficialEnv(t, WithVersionHook(PushEnqueueHook(ix)))

	// The Chennai fixture, made an Extreme cyclone message sent 10 minutes before
	// the test clock and ending 3 hours after it (the clock is 15:00 in India).
	body := capFixture(t, "cap_imd_chennai.xml")
	body = replaceOnce(t, body, "<cap:sent>2026-10-25T13:26:49+05:30</cap:sent>", "<cap:sent>2026-10-25T14:50:00+05:30</cap:sent>")
	body = replaceOnce(t, body, "<cap:severity>Moderate</cap:severity>", "<cap:severity>Extreme</cap:severity>")
	body = replaceOnce(t, body, "<cap:event>Moderate Rain</cap:event>", "<cap:event>Severe Cyclonic Storm</cap:event>")
	body = replaceOnce(t, body, "<cap:expires>2026-10-25T16:00:00+05:30</cap:expires>", "<cap:expires>2026-10-25T18:00:00+05:30</cap:expires>")
	e.srv.set([]fakeItem{{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"}}, "")
	e.srv.setCAP(guidChennai, body)

	// A person whose home cell lies inside Chennai (568), with one device.
	chennai, ok := ix.ByCode(568)
	if !ok {
		t.Fatal("district 568 is not in the boundary file")
	}
	cell := homeCell(t, ix, 568, false)
	account := testutil.SeedReporterAccount(t, e.pool, "refresh-chain@example.com", "refresh-chain-sess")
	const endpoint = "https://fcm.googleapis.com/fcm/send/refresh-chain-1"
	testutil.SeedPushSubscription(t, e.pool, account, "refresh-chain-sess", endpoint)
	testutil.SetHomeCell(t, e.pool, account, "Home", cell)

	reports := e.count(t, `SELECT count(*) FROM reports`)
	votes := e.count(t, `SELECT count(*) FROM votes`)
	sessions := e.count(t, `SELECT count(*) FROM sessions`)

	if _, err := e.refresh(t); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if n := e.count(t, `SELECT count(*) FROM official_push_outbox WHERE account_id = $1 AND state = 'queued' AND reason = 'new'`, account); n != 1 {
		t.Fatalf("%d rows queued for the person by the refresh, want 1 (a vacuous run would send nothing)", n)
	}

	sender := &recordingSender{}
	worker := service.NewOfficialPushWorker(e.q, sender, ix, service.WithOfficialPushWorkerClock(e.clk.now))
	if err := worker.SweepOnce(context.Background()); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	msgs := sender.messages()
	if len(msgs) != 1 || msgs[0].endpoint != endpoint {
		t.Fatalf("messages = %+v, want one to the person's device", msgs)
	}
	p := msgs[0].msg.Payload
	if p.Kind != "official" || p.URL != "/alerts" || p.Tag != service.OfficialTag(guidChennai) {
		t.Errorf("payload = %+v, want kind official, url /alerts and the alert's tag", p)
	}
	if want := "Cyclone warning for " + chennai.Name + " district"; p.Title != want {
		t.Errorf("title = %q, want %q", p.Title, want)
	}
	if p.Body != "Extreme. Issued by IMD." {
		t.Errorf("body = %q, want Extreme. Issued by IMD.", p.Body)
	}
	if msgs[0].msg.Urgency != push.UrgencyHigh || msgs[0].msg.Topic != service.OfficialTopic(guidChennai) {
		t.Errorf("urgency %v topic %q, want high and the alert's topic", msgs[0].msg.Urgency, msgs[0].msg.Topic)
	}
	if n := e.count(t, `SELECT count(*) FROM official_push_outbox WHERE account_id = $1 AND state = 'sent'`, account); n != 1 {
		t.Errorf("%d sent rows, want 1", n)
	}

	// A second sweep, and a second refresh of the same list, send nothing more.
	if err := worker.SweepOnce(context.Background()); err != nil {
		t.Fatalf("second SweepOnce: %v", err)
	}
	if _, err := e.refresh(t); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if err := worker.SweepOnce(context.Background()); err != nil {
		t.Fatalf("third SweepOnce: %v", err)
	}
	if n := len(sender.messages()); n != 1 {
		t.Errorf("%d pushes in total, want 1", n)
	}
	if e.count(t, `SELECT count(*) FROM reports`) != reports || e.count(t, `SELECT count(*) FROM votes`) != votes ||
		e.count(t, `SELECT count(*) FROM sessions`) != sessions {
		t.Error("the reports, votes or sessions counts changed")
	}
}
