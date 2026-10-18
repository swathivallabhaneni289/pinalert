package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

// SessionQuerier is the subset of the sqlc-generated Queries type
// SessionService needs, small enough to fake without a real Postgres.
type SessionQuerier interface {
	// RevokeAccountSessions marks every live session of one account revoked
	// and returns how many it changed.
	RevokeAccountSessions(ctx context.Context, arg sqlcgen.RevokeAccountSessionsParams) (int64, error)
}

var _ SessionQuerier = (*sqlcgen.Queries)(nil)

// SessionService ends sessions. It never clears sessions.account_id: that
// binding is how reports, votes and the reporter's own resolve reach their
// author, so ending a session must leave authorship and every count on other
// people's screens exactly as they were (Phase 3.2, Pitfall 14).
type SessionService struct {
	q   SessionQuerier
	now func() time.Time
}

// SessionOption configures a SessionService at construction time.
type SessionOption func(*SessionService)

// WithSessionClock overrides the SessionService's clock. Tests use it to fix
// the revocation time; production code never needs to call it.
func WithSessionClock(fn func() time.Time) SessionOption {
	return func(s *SessionService) {
		s.now = fn
	}
}

// NewSessionService builds a SessionService over q. It never dereferences q.
func NewSessionService(q SessionQuerier, opts ...SessionOption) *SessionService {
	s := &SessionService{q: q, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// LogOutAllDevices revokes every live session of the account in one statement
// and returns how many it ended, this browser included. A browser holding a
// revoked session, or a copy of its cookie, is refused by the gate from its
// next request (GetAccountBySessionID reads revoked_at). The call is keyed by
// the account id passed in and reads nothing from a request, so Delete my
// account can call it too. It is safe to call twice: the second call changes
// no rows and returns zero.
func (s *SessionService) LogOutAllDevices(ctx context.Context, accountID int64) (int64, error) {
	n, err := s.q.RevokeAccountSessions(ctx, sqlcgen.RevokeAccountSessionsParams{
		Now:       pgtype.Timestamptz{Time: s.now(), Valid: true},
		AccountID: &accountID,
	})
	if err != nil {
		// The account id only: no address may reach the server log.
		return 0, fmt.Errorf("service: revoking sessions of account %d: %w", accountID, err)
	}
	return n, nil
}
