package ratelimit

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// perKeyT0 is the instant every PerKey test starts its fake clock at.
var perKeyT0 = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

// perKeyClock is a mutex guarded fake clock, so goroutines in the concurrent
// test can read it while the test advances it and the race detector stays
// quiet.
type perKeyClock struct {
	mu  sync.Mutex
	now time.Time
}

func newPerKeyClock() *perKeyClock {
	return &perKeyClock{now: perKeyT0}
}

func (c *perKeyClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *perKeyClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *perKeyClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// perKeyMustPanic builds a PerKey from c and fails unless it panics with a
// string containing want.
func perKeyMustPanic(t *testing.T, c Config, want string) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if r == nil {
			t.Fatalf("NewPerKey(%+v) did not panic, want a panic containing %q", c, want)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("NewPerKey(%+v) panicked with %T, want a string", c, r)
		}
		if !strings.Contains(msg, want) {
			t.Fatalf("NewPerKey(%+v) panic = %q, want it to contain %q", c, msg, want)
		}
	}()
	NewPerKey(c)
}

// perKeyAdmit asserts that n reservations in a row for key are admitted with a
// zero wait and a non nil Cancel, and returns the decisions.
func perKeyAdmit(t *testing.T, p *PerKey, key string, n int) []Decision {
	t.Helper()
	out := make([]Decision, 0, n)
	for i := 0; i < n; i++ {
		d := p.Reserve(key)
		if !d.OK {
			t.Fatalf("reservation %d of %d for %q was refused (RetryAfter %v), want admitted", i+1, n, key, d.RetryAfter)
		}
		if d.RetryAfter != 0 {
			t.Fatalf("reservation %d for %q: RetryAfter = %v, want 0 when admitted", i+1, key, d.RetryAfter)
		}
		if d.Cancel == nil {
			t.Fatalf("reservation %d for %q: Cancel is nil, want a non nil func", i+1, key)
		}
		out = append(out, d)
	}
	return out
}

// perKeyRefuse asserts that the next reservation for key is refused with
// exactly the wait given.
func perKeyRefuse(t *testing.T, p *PerKey, key string, wait time.Duration) Decision {
	t.Helper()
	d := p.Reserve(key)
	if d.OK {
		t.Fatalf("reservation for %q was admitted, want refused with a wait of %v", key, wait)
	}
	if d.RetryAfter != wait {
		t.Fatalf("reservation for %q: RetryAfter = %v, want exactly %v", key, d.RetryAfter, wait)
	}
	if d.RetryAfter%time.Second != 0 {
		t.Fatalf("reservation for %q: RetryAfter = %v is not whole seconds", key, d.RetryAfter)
	}
	if d.Cancel == nil {
		t.Fatalf("reservation for %q: Cancel is nil on a refused decision", key)
	}
	return d
}

func TestPerKeyBurstThenRefuses(t *testing.T) {
	clk := newPerKeyClock()
	p := NewPerKey(Config{Every: 15 * time.Second, Burst: 20, Now: clk.Now})

	perKeyAdmit(t, p, "acct:1", 20)
	refused := perKeyRefuse(t, p, "acct:1", 15*time.Second)

	// Cancel on a refused decision is a harmless no-op that credits nothing.
	refused.Cancel()
	perKeyRefuse(t, p, "acct:1", 15*time.Second)
}

func TestPerKeyRefillOnInjectedClock(t *testing.T) {
	clk := newPerKeyClock()
	p := NewPerKey(Config{Every: 10 * time.Second, Burst: 2, Now: clk.Now})

	perKeyAdmit(t, p, "k", 2)
	perKeyRefuse(t, p, "k", 10*time.Second)

	// 7.5 s left, rounded up to 8 s and never down to 7 s.
	clk.Advance(2500 * time.Millisecond)
	perKeyRefuse(t, p, "k", 8*time.Second)

	clk.Advance(7500 * time.Millisecond)
	perKeyAdmit(t, p, "k", 1)
	perKeyRefuse(t, p, "k", 10*time.Second)

	// A long idle stretch refills only up to the burst.
	clk.Advance(time.Minute)
	perKeyAdmit(t, p, "k", 2)
	perKeyRefuse(t, p, "k", 10*time.Second)
}

func TestPerKeyIndependentKeys(t *testing.T) {
	clk := newPerKeyClock()

	t.Run("two keys in one instance", func(t *testing.T) {
		p := NewPerKey(Config{Every: 15 * time.Second, Burst: 20, Now: clk.Now})
		perKeyAdmit(t, p, "acct:1", 20)
		perKeyRefuse(t, p, "acct:1", 15*time.Second)
		perKeyAdmit(t, p, "acct:2", 20)
	})

	t.Run("two instances on one key", func(t *testing.T) {
		serious := NewPerKey(Config{Every: 5 * time.Minute, Burst: 5, Now: clk.Now})
		other := NewPerKey(Config{Every: 15 * time.Minute, Burst: 8, Now: clk.Now})
		perKeyAdmit(t, serious, "acct:1", 5)
		perKeyRefuse(t, serious, "acct:1", 300*time.Second)
		perKeyAdmit(t, other, "acct:1", 8)
		perKeyRefuse(t, other, "acct:1", 900*time.Second)
	})

	t.Run("cooldown keys", func(t *testing.T) {
		cool := NewPerKey(Config{Every: 30 * time.Second, Burst: 1, Now: clk.Now})
		perKeyAdmit(t, cool, "acct:1:rpt:9:content", 1)
		perKeyRefuse(t, cool, "acct:1:rpt:9:content", 30*time.Second)
		perKeyAdmit(t, cool, "acct:1:rpt:9:resolution", 1)
		perKeyAdmit(t, cool, "acct:2:rpt:9:content", 1)
	})
}

// TestPerKeyRefundAtReservationInstant: a frozen clock would pass against a
// refund taken at the later instant, so every case advances the clock between
// Reserve and Cancel.
func TestPerKeyRefundAtReservationInstant(t *testing.T) {
	t.Run("refund after a clock advance re-admits", func(t *testing.T) {
		clk := newPerKeyClock()
		p := NewPerKey(Config{Every: 30 * time.Second, Burst: 1, Now: clk.Now})
		x := perKeyAdmit(t, p, "k", 1)[0]
		clk.Advance(5 * time.Second)
		x.Cancel()
		perKeyAdmit(t, p, "k", 1)
	})

	t.Run("cancel twice restores one token", func(t *testing.T) {
		clk := newPerKeyClock()
		p := NewPerKey(Config{Every: 10 * time.Second, Burst: 2, Now: clk.Now})
		x := perKeyAdmit(t, p, "k", 2)[0]
		clk.Advance(time.Second)
		x.Cancel()
		x.Cancel()
		perKeyAdmit(t, p, "k", 1)
		perKeyRefuse(t, p, "k", 9*time.Second)
	})

	t.Run("refund never lifts a bucket above its burst", func(t *testing.T) {
		clk := newPerKeyClock()
		p := NewPerKey(Config{Every: 10 * time.Second, Burst: 2, Now: clk.Now})
		x := perKeyAdmit(t, p, "k", 1)[0]
		clk.Advance(time.Minute)
		x.Cancel()
		perKeyAdmit(t, p, "k", 2)
		perKeyRefuse(t, p, "k", 10*time.Second)
	})
}

// TestPerKeyInterleavedRefundIsLost documents x/time v0.16.0 behaviour: the
// refund is best effort when a later reservation for the same key happened in
// between. An x/time upgrade that changes it means revisiting 04-09 and 04-15,
// never editing these numbers.
func TestPerKeyInterleavedRefundIsLost(t *testing.T) {
	clk := newPerKeyClock()
	p := NewPerKey(Config{Every: 30 * time.Second, Burst: 1, Now: clk.Now})

	a := perKeyAdmit(t, p, "k", 1)[0]
	clk.Advance(time.Second)
	perKeyRefuse(t, p, "k", 29*time.Second)
	a.Cancel()
	clk.Advance(time.Second)
	perKeyRefuse(t, p, "k", 28*time.Second)
	clk.Set(perKeyT0.Add(31 * time.Second))
	perKeyAdmit(t, p, "k", 1)
}

func TestNewPerKeyPanicsOnInvalidConfig(t *testing.T) {
	perKeyMustPanic(t, Config{Every: 0, Burst: 1}, "Every")
	perKeyMustPanic(t, Config{Every: -time.Second, Burst: 1}, "Every")
	perKeyMustPanic(t, Config{Every: time.Second, Burst: 0}, "Burst")
}

func TestPerKeyNilNowUsesWallClock(t *testing.T) {
	p := NewPerKey(Config{Every: time.Hour, Burst: 1})
	perKeyAdmit(t, p, "k", 1)
	perKeyRefuse(t, p, "k", time.Hour)
}
