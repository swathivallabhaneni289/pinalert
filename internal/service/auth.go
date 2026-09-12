package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/auth"
	"pinalert/internal/mailer"
	sqlcgen "pinalert/internal/store/sqlc"
)

// VerifyOutcome discriminates the five ways GET /auth/verify (plan 01.1-02)
// can land. The handler passes the value straight into the outcome
// template's discriminator, so these are kept lowercase and stable — never
// renamed once shipped, since a template branch and this constant must stay
// in lockstep.
type VerifyOutcome string

const (
	OutcomeVerified  VerifyOutcome = "verified"
	OutcomeExpired   VerifyOutcome = "expired"
	OutcomeUsed      VerifyOutcome = "used"
	OutcomeMalformed VerifyOutcome = "malformed"
	OutcomeConflict  VerifyOutcome = "conflict"
)

// rawTokenBytes is the decoded length internal/auth.GenerateToken produces
// (tokenBytes there) — a token that doesn't decode to exactly this many
// bytes is malformed by construction, never a database lookup away from
// being valid.
const rawTokenBytes = 32

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
	GetMagicLinkTokenByHash(ctx context.Context, tokenHash string) (sqlcgen.GetMagicLinkTokenByHashRow, error)
	ConsumeToken(ctx context.Context, tokenHash string) (string, error)
	InsertAccount(ctx context.Context, email string) error
	GetAccountByEmail(ctx context.Context, email string) (sqlcgen.Account, error)
	GetAccountBySessionID(ctx context.Context, sessionID string) (sqlcgen.GetAccountBySessionIDRow, error)
	BindSessionAccount(ctx context.Context, arg sqlcgen.BindSessionAccountParams) error
	LatestTokenForEmail(ctx context.Context, email string) (time.Time, error)
	ReportsByAccount(ctx context.Context, accountID *int64) ([]sqlcgen.ReportsByAccountRow, error)
}

// ErrRateLimited is returned by RequestLink when a second verification-
// email request for the same normalised address arrives inside
// ResendCooldown of the last one (D-03/D-04, IDENT-04). It is a distinct
// sentinel — never a ValidationError — so the handler maps it to 429
// (errors.Is), not 400. Declared via errors.New rather than as a typed
// error since no additional data ever needs to travel with it.
var ErrRateLimited = errors.New("service: rate limited")

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

// RequestLink validates rawEmail, enforces D-03/D-04's per-address resend
// cooldown, mints a single-use magic-link token, persists its hash before
// attempting delivery, and hands the raw link — never a code to type back
// in; D-01 chose a link over a passcode, so no passcode field exists
// anywhere in this flow — to the configured Mailer. It returns the
// normalized address (trimmed, lowercased, no display-name form) that was
// actually mailed, so a caller such as the HTTP handler can echo it back
// without duplicating the normalization rule.
//
// The cooldown check happens between email normalisation and token
// generation — before anything is persisted or sent — so a refused request
// never writes a row and therefore can never extend or reset the cooldown
// it is itself being refused by (DEC-J). Because the check reads the
// normalised lowercase address and this row is always persisted lowercase,
// case variants of one address ("A@Example.com" vs "a@example.com") share
// one cooldown.
//
// The persist-then-send ordering below the cooldown check is mandatory, not
// incidental: the persisted row is exactly what the cooldown check above
// reads on the NEXT call, so a failed or quota-exhausted send must still
// consume the cooldown (RESEARCH.md Pitfall 3, DEC-J) — these are two
// different points in the request flow and must not be collapsed into one.
// A mailer failure is wrapped so the caller can log detail server-side
// while returning a generic error to the visitor.
func (s *AuthService) RequestLink(ctx context.Context, rawEmail string) (string, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return "", err
	}

	lastRequested, err := s.q.LatestTokenForEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("service: reading latest token for %s: %w", email, err)
		}
		// No prior request for this address — proceed.
	} else if s.now().Sub(lastRequested) < ResendCooldown {
		return "", ErrRateLimited
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

// VerifyToken decides the outcome of a clicked magic link for the given
// sessionID. It returns the outcome, an email address (populated only for
// OutcomeVerified and OutcomeConflict — empty for every other outcome), and
// a transport error for anything that isn't a clean outcome decision — a
// database error other than pgx.ErrNoRows is always returned as an error,
// never mapped to a fabricated outcome, so the handler can log it and
// render a generic 500. For OutcomeVerified the email is the
// newly-verified address; for OutcomeConflict it is the session's EXISTING
// verified account (what the visitor is currently signed in as), since the
// token's own target address was never proven in this browser.
//
// Sequence, in this exact order (DEC-D, DEC-E, RESEARCH.md Pattern 1 and
// Pitfall 2):
//  1. Reject an empty or malformed-shape token as OutcomeMalformed without
//     touching the database — a cheap shape check that keeps garbage out.
//  2. Hash the raw token and look up the row; pgx.ErrNoRows maps to
//     OutcomeMalformed (unknown token).
//  3. If the row is already used or expired, return that outcome. These
//     reads exist only to pick the right copy for the visitor, never to
//     authorise.
//  4. If the current session is already verified as a different account,
//     return OutcomeConflict WITHOUT consuming the token, so the rightful
//     owner's single-use link survives being opened in the wrong browser.
//  5. Consume the token atomically. This is the branch that actually
//     decides the request.
//  6. Lazily resolve (create-if-absent) the account for the token's email —
//     only now that ownership of the address is proven.
//  7. Bind the account onto the current session.
//  8. Return OutcomeVerified.
func (s *AuthService) VerifyToken(ctx context.Context, sessionID, rawToken string) (VerifyOutcome, string, error) {
	if !isWellFormedToken(rawToken) {
		return OutcomeMalformed, "", nil
	}

	hash := auth.HashToken(rawToken)
	row, err := s.q.GetMagicLinkTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OutcomeMalformed, "", nil
		}
		return "", "", fmt.Errorf("service: reading magic-link token: %w", err)
	}

	if row.UsedAt.Valid {
		return OutcomeUsed, "", nil
	}
	if !row.ExpiresAt.After(s.now()) {
		return OutcomeExpired, "", nil
	}

	existing, err := s.q.GetAccountBySessionID(ctx, sessionID)
	switch {
	case err == nil:
		if existing.Email != row.Email {
			// DEC-D/DEC-E: refuse to re-bind, and never consume the token —
			// the rightful owner's single-use link must survive being
			// opened in the wrong (already-verified) browser. The returned
			// email is the session's EXISTING verified account (what the
			// visitor is currently signed in as), not the token's target
			// address — that address was never proven in this browser,
			// which is the entire reason this is a conflict.
			return OutcomeConflict, existing.Email, nil
		}
	case errors.Is(err, pgx.ErrNoRows):
		// Unverified session opening a link it may not have requested
		// itself (D-07: cross-device verification is the supported happy
		// path, not an anomaly).
	default:
		return "", "", fmt.Errorf("service: reading current session's account: %w", err)
	}

	email, err := s.q.ConsumeToken(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if row.UsedAt.Valid {
				return OutcomeUsed, "", nil
			}
			return OutcomeExpired, "", nil
		}
		return "", "", fmt.Errorf("service: consuming magic-link token: %w", err)
	}

	if err := s.q.InsertAccount(ctx, email); err != nil {
		return "", "", fmt.Errorf("service: resolving account for %s: %w", email, err)
	}
	account, err := s.q.GetAccountByEmail(ctx, email)
	if err != nil {
		return "", "", fmt.Errorf("service: reading resolved account for %s: %w", email, err)
	}

	if err := s.q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{
		SessionID: sessionID,
		AccountID: &account.ID,
	}); err != nil {
		return "", "", fmt.Errorf("service: binding session to account: %w", err)
	}

	return OutcomeVerified, email, nil
}

// isWellFormedToken reports whether raw could plausibly be a value
// GenerateToken produced: non-empty, valid base64url (RawURLEncoding, no
// padding — matching GenerateToken's own encoding), and decoding to exactly
// rawTokenBytes bytes.
func isWellFormedToken(raw string) bool {
	if raw == "" {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return false
	}
	return len(decoded) == rawTokenBytes
}

// ReportsForAccount returns every report filed by any session ever bound to
// accountID, newest first — the profile page's core read (01.1-07-PLAN.md).
// Aggregation stays in the service layer rather than the handler, matching
// how ReportService owns the nearby query: sqlcgen.ReportsByAccountRow is a
// generated type callers should not need to know about directly, and a
// second caller (Phase 2's own profile-shaped read, if one ever exists)
// gets this same seam rather than a second hand-rolled query. accountID is
// always non-nil in practice — it's the ID of an account the request's
// gate has already resolved via a real database row (internal/api/gate.go)
// — but the underlying column is nullable (sessions.account_id), which is
// why sqlcgen.Queries.ReportsByAccount itself takes a pointer.
func (s *AuthService) ReportsForAccount(ctx context.Context, accountID int64) ([]sqlcgen.ReportsByAccountRow, error) {
	rows, err := s.q.ReportsByAccount(ctx, &accountID)
	if err != nil {
		return nil, fmt.Errorf("service: reading reports for account %d: %w", accountID, err)
	}
	return rows, nil
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
