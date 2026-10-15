package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// PerKey is the keyed limiter for Phase 4's report, vote and flag limits (D-10,
// D-11). Like PerIP it wraps golang.org/x/time/rate instead of hand rolling
// bucket arithmetic. It keeps no network or HTTP knowledge: keys are opaque
// strings, and the gate in internal/api/handlers builds them from the verified
// account.
//
// The store is in memory (limiter store, Claude's Discretion). A restart or a
// free host sleep forgets every bucket and gives each key a fresh burst. That is
// accepted: an idle sleep refills a normal bucket anyway, and the moderator's
// Remove is the backstop (D-11).
//
// A PerKey is safe for concurrent use.
type PerKey struct {
	mu         sync.Mutex
	entries    map[string]*keyEntry
	limit      rate.Limit
	burst      int
	every      time.Duration
	idleWindow time.Duration
	now        func() time.Time
}

// Config describes one allowance: Burst reservations at once, then one more
// every Every.
type Config struct {
	// Every is the refill interval, one token per Every. It must be positive.
	Every time.Duration
	// Burst is how many reservations a key gets at one instant. It must be at
	// least 1.
	Burst int
	// IdleWindow is how long a bucket may sit untouched before the sweep evicts
	// it. A zero value, or one below Burst * Every, is raised to Burst * Every.
	IdleWindow time.Duration
	// Now is the clock. Nil means the wall clock; tests inject a fake one.
	Now func() time.Time
}

// Decision is the outcome of one Reserve call.
type Decision struct {
	// OK reports whether the reservation was admitted.
	OK bool
	// RetryAfter is zero when OK, else the wait rounded up to whole seconds.
	RetryAfter time.Duration
	// Cancel is never nil. When OK it refunds the token; otherwise it does
	// nothing. It is safe to call on every decision and at most one call has an
	// effect. The refund is best effort when another reservation for the same
	// key happened in between (TestPerKeyInterleavedRefundIsLost).
	Cancel func()
}

// keyEntry pairs one key's limiter with the last time it was touched.
type keyEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewPerKey builds a PerKey from c. It panics on an invalid Config, as a
// startup programming error.
func NewPerKey(c Config) *PerKey {
	if c.Every <= 0 {
		panic("ratelimit: NewPerKey: Every must be positive")
	}
	if c.Burst < 1 {
		panic("ratelimit: NewPerKey: Burst must be at least 1")
	}
	now := c.Now
	if now == nil {
		now = time.Now
	}
	return &PerKey{
		entries:    make(map[string]*keyEntry),
		limit:      rate.Every(c.Every),
		burst:      c.Burst,
		every:      c.Every,
		idleWindow: c.IdleWindow,
		now:        now,
	}
}

// noRefund is the Cancel of every refused decision.
func noRefund() {}

// roundUpToSecond rounds d up to whole seconds with integer arithmetic only.
func roundUpToSecond(d time.Duration) time.Duration {
	return (d + time.Second - 1) / time.Second * time.Second
}

// Reserve takes one token for key, or reports how long to wait.
func (p *PerKey) Reserve(key string) Decision {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.now()
	e, ok := p.entries[key]
	if !ok {
		e = &keyEntry{limiter: rate.NewLimiter(p.limit, p.burst)}
		p.entries[key] = e
	}
	e.lastSeen = now

	r := e.limiter.ReserveN(now, 1)
	if !r.OK() {
		return Decision{RetryAfter: roundUpToSecond(p.every), Cancel: noRefund}
	}
	if d := r.DelayFrom(now); d > 0 {
		r.CancelAt(now)
		return Decision{RetryAfter: roundUpToSecond(d), Cancel: noRefund}
	}
	var once sync.Once
	return Decision{OK: true, Cancel: func() {
		once.Do(func() { r.CancelAt(p.now()) })
	}}
}
