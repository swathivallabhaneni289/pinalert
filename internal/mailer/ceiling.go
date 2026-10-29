package mailer

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrDailyCeiling is returned, without calling the real mailer, when the
// day's send budget is spent.
var ErrDailyCeiling = errors.New("mailer: daily send ceiling reached")

// dailyCeiling is a Mailer decorator that counts sends per UTC date.
type dailyCeiling struct {
	next             Mailer
	perDay           int
	reservedForReset int
	now              func() time.Time

	mu    sync.Mutex
	day   string
	count int
}

// NewDailyCeiling wraps next so that at most perDay sends go out per UTC date,
// every kind counted together (the link email and all four auth emails). Of
// those, reservedForReset are kept for the two Forgot password emails
// (KindCodeReset and KindNoAccount together, so a refusal never depends on
// whether the address has an account): every other kind stops at perDay minus
// reservedForReset, so a flood of sign ups cannot stop a real person
// resetting a password. now nil means time.Now.
//
// The Resend free plan allows 100 emails a day and 3,000 a month; the caller
// wires 90 so there is headroom (cmd/server/main.go only). The count lives in
// memory, so a restart resets it. That is accepted because the provider
// enforces the true limit. A send the wrapped mailer fails still counts, as
// the provider may have counted it.
func NewDailyCeiling(next Mailer, perDay, reservedForReset int, now func() time.Time) Mailer {
	if now == nil {
		now = time.Now
	}
	return &dailyCeiling{next: next, perDay: perDay, reservedForReset: reservedForReset, now: now}
}

// admit takes one unit of the day's budget, or reports that none is left.
func (c *dailyCeiling) admit(reset bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	today := c.now().UTC().Format("2006-01-02")
	if today != c.day {
		c.day = today
		c.count = 0
	}
	limit := c.perDay
	if !reset {
		limit = c.perDay - c.reservedForReset
	}
	if c.count >= limit {
		return ErrDailyCeiling
	}
	c.count++
	return nil
}

func (c *dailyCeiling) SendVerificationLink(ctx context.Context, email, link string) error {
	if err := c.admit(false); err != nil {
		return err
	}
	return c.next.SendVerificationLink(ctx, email, link)
}

func (c *dailyCeiling) SendAuthEmail(ctx context.Context, to string, msg AuthEmail) error {
	reset := msg.Kind == KindCodeReset || msg.Kind == KindNoAccount
	if err := c.admit(reset); err != nil {
		return err
	}
	return c.next.SendAuthEmail(ctx, to, msg)
}
