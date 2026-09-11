// Package ratelimit provides a reusable, self-pruning per-IP token-bucket
// limiter. It has no dependency on internal/service or internal/api so
// Phase 4's ROBUST-04 (session-first, IP-secondary rate limiting) can reuse
// it unchanged.
//
// This package deliberately does not hand-roll bucket arithmetic — it wraps
// golang.org/x/time/rate.Limiter, this project's declared standard for
// exactly this problem (see .claude/CLAUDE.md's Supporting Libraries table
// and 01.1-RESEARCH.md's "Don't Hand-Roll" table). A homemade
// counter-plus-timestamp map is the failure mode that standard exists to
// prevent.
package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// tooManyRequestsMessage is written for every request this limiter refuses.
// It is deliberately byte-identical to the per-email cooldown's refusal
// message (internal/service.ErrRateLimited's handler mapping) so a caller
// cannot tell which limiter tripped, or whether the address involved has an
// account (T-01-78).
const tooManyRequestsMessage = "Too many requests — try again in a minute."

// ipEntry pairs one key's limiter with the last time it was touched, so an
// idle sweep can evict entries nobody has used in a while without ever
// evicting an entry mid-burst.
type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// defaultIdleWindow is how long an entry may sit untouched before the sweep
// evicts it, for every caller using the exported NewPerIP constructor. It is
// generous relative to DEC-I's 60-second refill interval so a legitimate,
// merely-quiet IP is never evicted mid-session — this window only guards
// against unbounded growth from spoofed or rotating source IPs (T-01-80),
// not against a real visitor pausing between requests.
const defaultIdleWindow = 10 * time.Minute

// PerIP is a map of independent token-bucket limiters keyed by client IP.
// Two different keys have wholly independent budgets — exhausting one never
// affects another. Safe for concurrent use.
type PerIP struct {
	mu         sync.Mutex
	entries    map[string]*ipEntry
	limit      rate.Limit
	burst      int
	idleWindow time.Duration
}

// NewPerIP constructs a PerIP limiter allowing burst requests immediately,
// refilling one token every `every` duration thereafter. Idle entries are
// evicted after defaultIdleWindow via a background sweep so the map never
// grows without bound under spoofed or rotating source IPs (T-01-80). This
// exact two-argument signature is a contract other phases rely on — Phase
// 4's ROBUST-04 reuses this package unchanged — so a caller needing a
// different idle window uses newPerIPWithIdleWindow (unexported; this
// package's own tests drive a short window through it).
func NewPerIP(every time.Duration, burst int) *PerIP {
	return newPerIPWithIdleWindow(every, burst, defaultIdleWindow)
}

// newPerIPWithIdleWindow is NewPerIP's unexported implementation, taking an
// explicit idle window so this package's own tests can exercise the sweep
// under a window measured in milliseconds rather than minutes.
func newPerIPWithIdleWindow(every time.Duration, burst int, idleWindow time.Duration) *PerIP {
	p := &PerIP{
		entries:    make(map[string]*ipEntry),
		limit:      rate.Every(every),
		burst:      burst,
		idleWindow: idleWindow,
	}
	go p.sweepLoop()
	return p
}

// sweepLoop periodically evicts idle entries for the lifetime of the
// process. It never exits — PerIP instances are constructed once at startup
// and live for the process's lifetime, matching every other long-running
// goroutine in this codebase (e.g. the session manager has none, but the
// pattern mirrors a plain time.Ticker-driven goroutine per CLAUDE.md's
// Supporting Libraries guidance).
func (p *PerIP) sweepLoop() {
	ticker := time.NewTicker(p.idleWindow)
	defer ticker.Stop()
	for range ticker.C {
		p.sweep(time.Now())
	}
}

// sweep removes every entry whose lastSeen is older than idleWindow relative
// to now. It holds the same mutex Allow does, so a sweep can never race a
// concurrent Allow call on the same map.
func (p *PerIP) sweep(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, e := range p.entries {
		if now.Sub(e.lastSeen) > p.idleWindow {
			delete(p.entries, key)
		}
	}
}

// Allow reports whether a request keyed by key should proceed. A fresh key
// gets its own limiter, seeded with the full configured burst.
func (p *PerIP) Allow(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	e, ok := p.entries[key]
	if !ok {
		e = &ipEntry{limiter: rate.NewLimiter(p.limit, p.burst)}
		p.entries[key] = e
	}
	e.lastSeen = time.Now()
	return e.limiter.Allow()
}

// entryCount reports how many keys are currently tracked. Exported only to
// this package's own tests via the eviction test below (lowercase, no
// exported accessor) — a test in this same package can read the field
// directly instead.
func (p *PerIP) entryCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}

// Middleware returns an http middleware that keys on the client's IP,
// refusing a request that exceeds the configured budget with a 429 in the
// same {error:{field,message}} envelope the rest of this API uses. It reads
// r.RemoteAddr, which chi's middleware.RealIP has already normalised
// upstream in this project's router — callers wrapping a handler with this
// middleware must apply it after RealIP for that trust assumption to hold.
func (p *PerIP) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r.RemoteAddr)
			if !p.Allow(key) {
				writeTooManyRequests(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the host portion of remoteAddr, falling back to the raw
// string when it does not split cleanly into host:port (e.g. a test harness
// that sets RemoteAddr to a bare IP).
func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// writeTooManyRequests writes the standard 429 envelope. field is "email"
// deliberately — not "ip" — matching the exact UI-SPEC string and the
// per-address refusal's field name, so a caller cannot distinguish which
// limiter tripped.
func writeTooManyRequests(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":{"field":"email","message":"` + tooManyRequestsMessage + `"}}`))
}
