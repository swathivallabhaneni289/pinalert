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
type Mailer interface {
	SendVerificationLink(ctx context.Context, email, link string) error
}
