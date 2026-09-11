package service

import (
	"context"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"pinalert/internal/auth"
	"pinalert/internal/mailer"
	sqlcgen "pinalert/internal/store/sqlc"
)

// ResendCooldown is the minimum time a caller must wait between successive
// verification-email requests for the same address (D-03: the midpoint of
// the 30-60 second range CONTEXT.md leaves to Claude's discretion).
// Exported so handlers and templates share one source of truth rather than
// each hard-coding the number separately.
const ResendCooldown = 45 * time.Second

// AuthQuerier is the subset of the sqlc-generated Queries type AuthService
// needs — small enough to fake in a test without a real Postgres. Later
// plans widen this interface as more of the verification flow lands.
type AuthQuerier interface {
	InsertMagicLinkToken(ctx context.Context, arg sqlcgen.InsertMagicLinkTokenParams) (sqlcgen.InsertMagicLinkTokenRow, error)
}

// AuthService validates a requested email address, mints a magic-link
// token, persists it, and hands the emailed link to a Mailer.
type AuthService struct {
	q       AuthQuerier
	mailer  mailer.Mailer
	baseURL string
	now     func() time.Time
}

// AuthOption configures an AuthService at construction time.
type AuthOption func(*AuthService)

// WithClock overrides the AuthService's clock. Tests use this to advance
// time without real sleeps; production code never needs to call it. The
// seam is introduced here — rather than added later alongside plan
// 01.1-05's cooldown logic — because internal/service/auth_test.go lives in
// the external service_test package (matching report_test.go) and so
// cannot reach an unexported field after the fact; adding it now means that
// later plan changes no constructor signature and breaks no call site.
func WithClock(fn func() time.Time) AuthOption {
	return func(s *AuthService) {
		s.now = fn
	}
}

// NewAuthService constructs an AuthService backed by q and m, minting
// absolute verification links rooted at baseURL. The clock defaults to
// time.Now() unless overridden via WithClock.
func NewAuthService(q AuthQuerier, m mailer.Mailer, baseURL string, opts ...AuthOption) *AuthService {
	s := &AuthService{q: q, mailer: m, baseURL: baseURL, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// RequestLink validates rawEmail, mints a single-use magic-link token,
// persists its hash before attempting delivery, and hands the raw link —
// never a code to type back in; D-01 chose a link over a passcode, so no
// passcode field exists anywhere in this flow — to the configured Mailer.
// It returns the normalized address (trimmed, lowercased, no display-name
// form) that was actually mailed, so a caller such as the HTTP handler can
// echo it back without duplicating the normalization rule.
//
// The persist-then-send ordering is mandatory, not incidental: the
// persisted row is what plan 01.1-05's cooldown reads, so a failed or
// quota-exhausted send must still consume the cooldown (RESEARCH.md
// Pitfall 3). A mailer failure is wrapped so the caller can log detail
// server-side while returning a generic error to the visitor.
func (s *AuthService) RequestLink(ctx context.Context, rawEmail string) (string, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return "", err
	}

	raw, hash, err := auth.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("service: generating magic-link token: %w", err)
	}

	if _, err := s.q.InsertMagicLinkToken(ctx, sqlcgen.InsertMagicLinkTokenParams{
		TokenHash: hash,
		Email:     email,
		ExpiresAt: s.now().Add(auth.TokenTTL),
	}); err != nil {
		return "", fmt.Errorf("service: persisting magic-link token: %w", err)
	}

	link := s.baseURL + "/auth/verify?token=" + url.QueryEscape(raw)
	if err := s.mailer.SendVerificationLink(ctx, email, link); err != nil {
		return "", fmt.Errorf("service: sending verification link: %w", err)
	}

	return email, nil
}

// normalizeEmail trims rawEmail, parses it with the stdlib address parser
// (RESEARCH.md "Don't Hand-Roll" — no hand-rolled regex), rejects a display-name form
// such as "Someone <a@b.com>" since only a bare address is accepted, and
// lowercases the result so one human's address is one identity regardless
// of how they typed it.
func normalizeEmail(rawEmail string) (string, error) {
	trimmed := strings.TrimSpace(rawEmail)
	addr, err := mail.ParseAddress(trimmed)
	if err != nil || addr.Name != "" {
		return "", ValidationError{Field: "email", Message: "Enter a valid email address."}
	}
	return strings.ToLower(addr.Address), nil
}
