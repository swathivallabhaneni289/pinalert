package mailer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countingMailer counts the calls that reach it. Amended 2026-10-29 by Phase
// 7.1 plan 14b: the link email is gone, so auths is the only count.
type countingMailer struct {
	auths atomic.Int64
	err   error
}

func (c *countingMailer) SendAuthEmail(ctx context.Context, to string, msg AuthEmail) error {
	c.auths.Add(1)
	return c.err
}

func (c *countingMailer) total() int { return int(c.auths.Load()) }

var (
	signupMsg  = AuthEmail{Kind: KindCodeSignup, Code: "482913", MinutesValid: 10}
	resetMsg   = AuthEmail{Kind: KindCodeReset, Code: "482913", MinutesValid: 10}
	existsMsg  = AuthEmail{Kind: KindAccountExists}
	noAcctMsg  = AuthEmail{Kind: KindNoAccount}
	ceilingDay = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
)

func TestMailerCeilingRefusesPastLimit(t *testing.T) {
	next := &countingMailer{}
	m := NewDailyCeiling(next, 3, 0, func() time.Time { return ceilingDay })
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); err != nil {
			t.Fatalf("send %d: %v", i+1, err)
		}
	}
	if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); !errors.Is(err, ErrDailyCeiling) {
		t.Fatalf("fourth send error = %v, want ErrDailyCeiling", err)
	}
	if next.total() != 3 {
		t.Errorf("wrapped mailer called %d times, want 3", next.total())
	}
}

func TestMailerCeilingResetsNextUTCDay(t *testing.T) {
	next := &countingMailer{}
	clock := time.Date(2026, 10, 29, 23, 59, 0, 0, time.UTC)
	m := NewDailyCeiling(next, 1, 0, func() time.Time { return clock })
	ctx := context.Background()
	if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); err != nil {
		t.Fatal(err)
	}
	if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); !errors.Is(err, ErrDailyCeiling) {
		t.Fatalf("same day second send error = %v, want ErrDailyCeiling", err)
	}
	// The same instant seen in another zone is still the same UTC date.
	ist := time.FixedZone("IST", 5*3600+1800)
	clock = time.Date(2026, 10, 30, 0, 10, 0, 0, time.UTC).In(ist).Add(-time.Hour)
	if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); !errors.Is(err, ErrDailyCeiling) {
		t.Fatalf("a time still on the same UTC date: error = %v, want ErrDailyCeiling", err)
	}
	clock = time.Date(2026, 10, 30, 0, 1, 0, 0, time.UTC)
	if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); err != nil {
		t.Fatalf("next UTC day send: %v", err)
	}
	if next.total() != 2 {
		t.Errorf("wrapped mailer called %d times, want 2", next.total())
	}
}

func TestMailerCeilingCountsEveryKind(t *testing.T) {
	next := &countingMailer{}
	m := NewDailyCeiling(next, 5, 0, func() time.Time { return ceilingDay })
	ctx := context.Background()
	sends := []func() error{
		func() error { return m.SendAuthEmail(ctx, "a@example.com", signupMsg) },
		func() error { return m.SendAuthEmail(ctx, "a@example.com", resetMsg) },
		func() error { return m.SendAuthEmail(ctx, "a@example.com", existsMsg) },
		func() error { return m.SendAuthEmail(ctx, "a@example.com", noAcctMsg) },
		func() error { return m.SendAuthEmail(ctx, "a@example.com", signupMsg) },
	}
	for i, send := range sends {
		if err := send(); err != nil {
			t.Fatalf("send %d: %v", i+1, err)
		}
	}
	if err := m.SendAuthEmail(ctx, "a@example.com", resetMsg); !errors.Is(err, ErrDailyCeiling) {
		t.Fatalf("sixth send error = %v, want ErrDailyCeiling", err)
	}
	if next.auths.Load() != 5 {
		t.Errorf("wrapped mailer saw %d auth emails, want 5", next.auths.Load())
	}
}

func TestMailerCeilingCountsFailedSends(t *testing.T) {
	next := &countingMailer{err: errors.New("provider down")}
	m := NewDailyCeiling(next, 2, 0, func() time.Time { return ceilingDay })
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); err == nil || errors.Is(err, ErrDailyCeiling) {
			t.Fatalf("send %d error = %v, want the provider error", i+1, err)
		}
	}
	if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); !errors.Is(err, ErrDailyCeiling) {
		t.Fatalf("third send error = %v, want ErrDailyCeiling (failed sends count)", err)
	}
}

func TestMailerCeilingConcurrent(t *testing.T) {
	next := &countingMailer{}
	m := NewDailyCeiling(next, 20, 0, func() time.Time { return ceilingDay })
	var wg sync.WaitGroup
	var refused atomic.Int64
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.SendAuthEmail(context.Background(), "a@example.com", signupMsg); errors.Is(err, ErrDailyCeiling) {
				refused.Add(1)
			}
		}()
	}
	wg.Wait()
	if next.total() != 20 {
		t.Errorf("wrapped mailer called %d times, want exactly 20", next.total())
	}
	if refused.Load() != 30 {
		t.Errorf("refused %d, want 30", refused.Load())
	}
}

// TestMailerCeilingPacesPerWindow pins the pacing of review CR-S03: at most
// PerWindow sends in any Window, WindowResetReserve of them kept for the reset
// class, and the window slides, so a refused person gets in minutes later.
func TestMailerCeilingPacesPerWindow(t *testing.T) {
	ctx := context.Background()
	clock := ceilingDay
	next := &countingMailer{}
	m := NewSendCeiling(next, CeilingLimits{PerDay: 100, ResetReserve: 10, Window: 10 * time.Minute, PerWindow: 4, WindowResetReserve: 1},
		func() time.Time { return clock })
	for i, msg := range []AuthEmail{signupMsg, existsMsg, signupMsg} {
		if err := m.SendAuthEmail(ctx, "a@example.com", msg); err != nil {
			t.Fatalf("send %d: %v", i+1, err)
		}
	}
	for _, msg := range []AuthEmail{signupMsg, existsMsg} {
		if err := m.SendAuthEmail(ctx, "a@example.com", msg); !errors.Is(err, ErrDailyCeiling) {
			t.Fatalf("a fourth Create account email in the window: %v, want ErrDailyCeiling", err)
		}
	}
	if err := m.SendAuthEmail(ctx, "a@example.com", noAcctMsg); err != nil {
		t.Fatalf("the window's kept reset slot: %v", err)
	}
	for _, msg := range []AuthEmail{resetMsg, noAcctMsg} {
		if err := m.SendAuthEmail(ctx, "a@example.com", msg); !errors.Is(err, ErrDailyCeiling) {
			t.Fatalf("a fifth send in the window: %v, want ErrDailyCeiling", err)
		}
	}
	clock = clock.Add(9 * time.Minute)
	if err := m.SendAuthEmail(ctx, "a@example.com", resetMsg); !errors.Is(err, ErrDailyCeiling) {
		t.Fatalf("9 minutes later the window is still full: %v", err)
	}
	clock = clock.Add(time.Minute)
	if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); err != nil {
		t.Fatalf("10 minutes later the window has slid: %v", err)
	}
	if next.total() != 5 {
		t.Errorf("wrapped mailer called %d times, want 5", next.total())
	}
}

// TestMailerCeilingSenderShare pins the sender share of review CR-S03: once the
// other class is refused, one sender takes at most SenderReserveShare of the
// kept slots a day, both reset kinds alike, while another sender and a send
// with no sender still get in.
func TestMailerCeilingSenderShare(t *testing.T) {
	clock := ceilingDay
	next := &countingMailer{}
	m := NewSendCeiling(next, CeilingLimits{PerDay: 10, ResetReserve: 5, SenderReserveShare: 2},
		func() time.Time { return clock })
	a := WithSender(context.Background(), "net:198.51.100.7")
	b := WithSender(context.Background(), "net:203.0.113.20")
	// Before the other class is full, reset sends take no share.
	for i := 0; i < 5; i++ {
		msg := signupMsg
		if i == 0 {
			msg = noAcctMsg
		}
		if err := m.SendAuthEmail(a, "a@example.com", msg); err != nil {
			t.Fatalf("send %d: %v", i+1, err)
		}
	}
	for i, msg := range []AuthEmail{noAcctMsg, resetMsg} {
		if err := m.SendAuthEmail(a, "a@example.com", msg); err != nil {
			t.Fatalf("sender A's kept slot %d: %v", i+1, err)
		}
	}
	for _, msg := range []AuthEmail{noAcctMsg, resetMsg} {
		if err := m.SendAuthEmail(a, "a@example.com", msg); !errors.Is(err, ErrDailyCeiling) {
			t.Fatalf("sender A past its share: %v, want ErrDailyCeiling", err)
		}
	}
	if err := m.SendAuthEmail(b, "b@example.com", resetMsg); err != nil {
		t.Fatalf("sender B: %v", err)
	}
	if err := m.SendAuthEmail(context.Background(), "c@example.com", resetMsg); err != nil {
		t.Fatalf("a send with no sender: %v", err)
	}
	// The next UTC day the shares start again.
	clock = clock.Add(24 * time.Hour)
	for i := 0; i < 5; i++ {
		_ = m.SendAuthEmail(a, "a@example.com", signupMsg)
	}
	if err := m.SendAuthEmail(a, "a@example.com", noAcctMsg); err != nil {
		t.Fatalf("sender A the next day: %v", err)
	}
}

// TestMailerCeilingShareOnlyOnDayReserve is SK-06 of the Phase 7.1 review fix
// skeptic: the sender share applies only while the day's reserve is being
// spent, never because the 10 minute window is busy. A shared carrier address
// (CGNAT) is one sender, so on a busy but normal day the phones behind it must
// not be refused Forgot password after three resets. Before the fix the fourth
// reset from that address in a busy window was refused.
func TestMailerCeilingShareOnlyOnDayReserve(t *testing.T) {
	clock := ceilingDay
	next := &countingMailer{}
	m := NewSendCeiling(next, DefaultCeilingLimits, func() time.Time { return clock })
	carrier := WithSender(context.Background(), "net:49.36.1.1")
	for i := 0; i < 10; i++ {
		if err := m.SendAuthEmail(context.Background(), "a@example.com", signupMsg); err != nil {
			t.Fatalf("sign up send %d: %v", i+1, err)
		}
	}
	// The window's other class is now full; the day's reserve is untouched.
	for i := 0; i < 5; i++ {
		clock = clock.Add(time.Minute)
		msg := resetMsg
		if i%2 == 1 {
			msg = noAcctMsg
		}
		if err := m.SendAuthEmail(carrier, "b@example.com", msg); err != nil {
			t.Fatalf("reset %d from one carrier address in a busy window: %v", i+1, err)
		}
	}
	// The window cap itself still holds.
	if err := m.SendAuthEmail(carrier, "b@example.com", resetMsg); !errors.Is(err, ErrDailyCeiling) {
		t.Fatalf("a 16th send in the window: %v, want ErrDailyCeiling", err)
	}
	if next.total() != 15 {
		t.Errorf("wrapped mailer called %d times, want 15", next.total())
	}
}

func TestMailerCeilingReservesResetSends(t *testing.T) {
	ctx := context.Background()

	t.Run("other kinds stop at perDay minus reserve", func(t *testing.T) {
		next := &countingMailer{}
		m := NewDailyCeiling(next, 10, 3, func() time.Time { return ceilingDay })
		for i := 0; i < 7; i++ {
			msg := signupMsg
			if i%2 == 1 {
				msg = existsMsg
			}
			if err := m.SendAuthEmail(ctx, "a@example.com", msg); err != nil {
				t.Fatalf("send %d: %v", i+1, err)
			}
		}
		if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); !errors.Is(err, ErrDailyCeiling) {
			t.Fatalf("eighth sign up send error = %v, want ErrDailyCeiling", err)
		}
		// The reserve is still there for both reset kinds, then the day ends at 10.
		for i, msg := range []AuthEmail{resetMsg, noAcctMsg, resetMsg} {
			if err := m.SendAuthEmail(ctx, "a@example.com", msg); err != nil {
				t.Fatalf("reset send %d: %v", i+1, err)
			}
		}
		for _, msg := range []AuthEmail{resetMsg, noAcctMsg} {
			if err := m.SendAuthEmail(ctx, "a@example.com", msg); !errors.Is(err, ErrDailyCeiling) {
				t.Fatalf("reset send past 10 error = %v, want ErrDailyCeiling", err)
			}
		}
		if next.total() != 10 {
			t.Errorf("wrapped mailer called %d times, want 10", next.total())
		}
	})

	t.Run("reset kinds sent first count toward the same total", func(t *testing.T) {
		next := &countingMailer{}
		m := NewDailyCeiling(next, 10, 3, func() time.Time { return ceilingDay })
		for i := 0; i < 5; i++ {
			if err := m.SendAuthEmail(ctx, "a@example.com", resetMsg); err != nil {
				t.Fatalf("reset send %d: %v", i+1, err)
			}
		}
		for i := 0; i < 2; i++ {
			if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); err != nil {
				t.Fatalf("sign up send %d: %v", i+1, err)
			}
		}
		if err := m.SendAuthEmail(ctx, "a@example.com", signupMsg); !errors.Is(err, ErrDailyCeiling) {
			t.Fatalf("sign up send at 7 error = %v, want ErrDailyCeiling", err)
		}
		if next.total() != 7 {
			t.Errorf("wrapped mailer called %d times, want 7", next.total())
		}
	})
}
