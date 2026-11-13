// Package mailer defines the single seam Pinalert's email delivery goes
// through. Injecting a Mailer implementation keeps tests and local
// development off the network entirely (RESEARCH.md Pitfall 6): a fake in
// tests, a log-only implementation in development, and a real SMTP backed
// implementation (Gmail) in production, all satisfying the same interface so the
// caller never branches on environment.
package mailer

import "context"

// Mailer sends one of the four password sign in emails (a code, or a note that
// an account does or does not exist) through SendAuthEmail. It holds no link
// (D-01): an emailed sign in link no longer exists, and this is the only
// method. Implementations must never leak provider-specific error detail to
// callers outside this package; wrap and log server-side instead (see
// 01.1-01-PLAN.md threat T-01-55).
type Mailer interface {
	SendAuthEmail(ctx context.Context, to string, msg AuthEmail) error
}
