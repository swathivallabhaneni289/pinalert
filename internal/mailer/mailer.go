// Package mailer defines the single seam Pinalert's verification-email
// delivery goes through. Injecting a Mailer implementation keeps tests and
// local development off the network entirely (RESEARCH.md Pitfall 6): a
// fake in tests, a log-only implementation in development, and — from plan
// 01.1-03 onward — a real Resend-backed implementation in production, all
// satisfying the same interface so the caller never branches on environment.
package mailer

import "context"

// Mailer sends a verification link to email. Implementations must never
// leak provider-specific error detail to callers outside this package —
// wrap and log server-side instead (see 01.1-01-PLAN.md threat T-01-55).
//
// SendAuthEmail sends one of the four password sign in emails (a code, or a
// note that an account does or does not exist). It holds no link (D-01). The
// cleanup plan of phase 07.1 removes SendVerificationLink, leaving this as the
// only method.
type Mailer interface {
	SendVerificationLink(ctx context.Context, email, link string) error
	SendAuthEmail(ctx context.Context, to string, msg AuthEmail) error
}
