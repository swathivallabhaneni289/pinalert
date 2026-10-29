package mailer

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrDailyCeiling is returned, without calling the real mailer, when the send
// ceiling refuses: the day's budget, the per window pacing or the sender's
// share of the reset reserve is spent. The service answers every one of them
// with the same 503.
var ErrDailyCeiling = errors.New("mailer: daily send ceiling reached")

// CeilingLimits are the numbers of the send ceiling (Phase 7.1 review CR-S03).
//
// Every send is in one of two classes, decided by its kind in pairs, so a
// refusal never depends on whether the address has an account: the reset class
// (KindCodeReset and KindNoAccount, the two Forgot password emails) and the
// other class (KindCodeSignup and KindAccountExists, the two Create account
// emails).
type CeilingLimits struct {
	// PerDay is the most sends per UTC date. ResetReserve of them are kept for
	// the reset class: the other class stops at PerDay minus ResetReserve.
	PerDay       int
	ResetReserve int
	// Window paces the sends: at most PerWindow in any Window, of which
	// WindowResetReserve are kept for the reset class. A Window of 0 turns the
	// pacing off.
	Window             time.Duration
	PerWindow          int
	WindowResetReserve int
	// SenderReserveShare is how many reserved slots of the day (a reset class
	// send admitted once the other class has spent its PerDay minus
	// ResetReserve) one sender may take per UTC date. The window's kept slots
	// never count toward it (review fix skeptic SK-06).
	// The sender is the one WithSender put in the context; a send with no sender
	// has no share limit. 0 turns the share off.
	SenderReserveShare int
}

// DefaultCeilingLimits mirrors the ceiling block of cmd/server/main.go, which
// is the one named place for the numbers; a test there pins the two together.
var DefaultCeilingLimits = CeilingLimits{
	PerDay:             90,
	ResetReserve:       20,
	Window:             10 * time.Minute,
	PerWindow:          15,
	WindowResetReserve: 5,
	SenderReserveShare: 3,
}

// senderKey is the context key of the sender WithSender records.
type senderKey struct{}

// WithSender records who is asking for the sends made with ctx, for the
// sender's share of the reset reserve: the client network (the IPv4 address or
// the IPv6 /64) when the host names a client IP header, one shared key for
// requests without a valid session cookie, and "" (no share) otherwise. The
// key never depends on the address the email goes to.
func WithSender(ctx context.Context, sender string) context.Context {
	return context.WithValue(ctx, senderKey{}, sender)
}

func senderFrom(ctx context.Context) string {
	s, _ := ctx.Value(senderKey{}).(string)
	return s
}

// sendCeiling is a Mailer decorator that counts sends per UTC date and per
// window.
type sendCeiling struct {
	next   Mailer
	limits CeilingLimits
	now    func() time.Time

	mu     sync.Mutex
	day    string
	count  int
	recent []time.Time
	shares map[string]int
}

// NewSendCeiling wraps next with the send ceiling. now nil means time.Now.
//
// The Resend free plan allows 100 emails a day and 3,000 a month; the caller
// wires 90 so there is headroom (cmd/server/main.go only). The counts live in
// memory, so a restart, or a free host waking from sleep, resets them. That is
// accepted as part of T-07.1-09 in docs/TRUST-MODEL.md: after a reset the
// provider's own day limit is the only cap, and once it is spent the provider
// refuses every email, Forgot password included. A send the wrapped mailer
// fails still counts, as the provider may have counted it.
//
// The pacing means a burst cannot spend the whole day: a person refused in a
// busy window gets in a few minutes later, not the next day. The sender share
// means one network that spends the other class's budget and then floods
// Forgot password for made up addresses takes at most SenderReserveShare of the
// reserved slots, so a real person on another network can still reset. Both
// rules treat the two kinds of a class the same, so neither tells whether an
// account exists.
func NewSendCeiling(next Mailer, l CeilingLimits, now func() time.Time) Mailer {
	if now == nil {
		now = time.Now
	}
	return &sendCeiling{next: next, limits: l, now: now, shares: map[string]int{}}
}

// NewDailyCeiling wraps next so that at most perDay sends go out per UTC date,
// every kind counted together (all four auth emails), with reservedForReset of
// them kept for the reset class. It is NewSendCeiling with no pacing and no
// sender share.
func NewDailyCeiling(next Mailer, perDay, reservedForReset int, now func() time.Time) Mailer {
	return NewSendCeiling(next, CeilingLimits{PerDay: perDay, ResetReserve: reservedForReset}, now)
}

// admit takes one unit of the budget for a send of the given class from the
// given sender, or reports that none is left. Every decision reads the state
// before this send.
func (c *sendCeiling) admit(reset bool, sender string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	today := now.UTC().Format("2006-01-02")
	if today != c.day {
		c.day = today
		c.count = 0
		c.shares = map[string]int{}
	}
	windowOn := c.limits.Window > 0
	if windowOn {
		cutoff := now.Add(-c.limits.Window)
		kept := c.recent[:0]
		for _, at := range c.recent {
			if at.After(cutoff) {
				kept = append(kept, at)
			}
		}
		c.recent = kept
	}
	inWindow := len(c.recent)

	// Whether the other class would be refused now.
	dayReserveInUse := c.count >= c.limits.PerDay-c.limits.ResetReserve
	otherFull := dayReserveInUse ||
		(windowOn && inWindow >= c.limits.PerWindow-c.limits.WindowResetReserve)
	if !reset {
		if otherFull {
			return ErrDailyCeiling
		}
	} else {
		if c.count >= c.limits.PerDay || (windowOn && inWindow >= c.limits.PerWindow) {
			return ErrDailyCeiling
		}
		// The share counts only the day's reserved slots, never the window's
		// (review fix skeptic SK-06): a busy 10 minutes is normal on a flood
		// day, and a shared carrier address (CGNAT) is one sender, so a share
		// charged by the window would refuse every phone behind it.
		if dayReserveInUse && c.limits.SenderReserveShare > 0 && sender != "" {
			if c.shares[sender] >= c.limits.SenderReserveShare {
				return ErrDailyCeiling
			}
			c.shares[sender]++
		}
	}
	c.count++
	if windowOn {
		c.recent = append(c.recent, now)
	}
	return nil
}

func (c *sendCeiling) SendAuthEmail(ctx context.Context, to string, msg AuthEmail) error {
	reset := msg.Kind == KindCodeReset || msg.Kind == KindNoAccount
	if err := c.admit(reset, senderFrom(ctx)); err != nil {
		return err
	}
	return c.next.SendAuthEmail(ctx, to, msg)
}
