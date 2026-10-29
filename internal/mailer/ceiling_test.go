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
