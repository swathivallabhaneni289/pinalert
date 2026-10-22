package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

// DeleteConfirmMessage is the field message when the delete request does not
// say confirm true.
const DeleteConfirmMessage = "Confirm to delete your account."

// ErrAccountAlreadyDeleted is returned when the account is already marked
// deleted, or was marked deleted by a request that won the race. Nothing is
// changed.
var ErrAccountAlreadyDeleted = errors.New("service: account is already deleted")

// deletedEmailDomain is the reserved .invalid top level domain (RFC 2606),
// which can never receive mail and never resolves.
const deletedEmailDomain = "deleted.invalid"

// AccountDeleteQuerier is the slice of the generated queries that deleting an
// account uses. It is declared here, in the consumer, the way ModerationQuerier
// is. RevokeAccountSessions is the same statement Log out of all devices runs.
type AccountDeleteQuerier interface {
	AccountEmailForDelete(ctx context.Context, id int64) (sqlcgen.AccountEmailForDeleteRow, error)
	DeleteMagicLinkTokensForEmail(ctx context.Context, email string) error
	DeleteEmailCooldownForEmail(ctx context.Context, email string) error
	DeleteAccountUpdates(ctx context.Context, accountID int64) error
	DeleteAccountCursors(ctx context.Context, accountID int64) error
	DeleteAccountMutes(ctx context.Context, accountID int64) error
	DeleteAccountAlertSettings(ctx context.Context, accountID int64) error
	RevokeAccountSessions(ctx context.Context, arg sqlcgen.RevokeAccountSessionsParams) (int64, error)
	ScrubDeletedAccount(ctx context.Context, arg sqlcgen.ScrubDeletedAccountParams) (int64, error)
}

var _ AccountDeleteQuerier = (*sqlcgen.Queries)(nil)

// AccountTxFunc runs fn inside one database transaction with a querier bound
// to that transaction, commits when fn returns nil and rolls back otherwise.
type AccountTxFunc func(ctx context.Context, fn func(q AccountDeleteQuerier) error) error

// PgxAccountTx builds the production AccountTxFunc over a pool.
func PgxAccountTx(db TxBeginner) AccountTxFunc {
	return func(ctx context.Context, fn func(q AccountDeleteQuerier) error) error {
		return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			return fn(sqlcgen.New(tx))
		})
	}
}

// AccountDeletionService deletes an account by the rule written on
// DeleteAccount.
type AccountDeletionService struct {
	q   AccountDeleteQuerier
	tx  AccountTxFunc
	now func() time.Time
}

// AccountDeleteOption configures an AccountDeletionService at construction.
type AccountDeleteOption func(*AccountDeletionService)

// WithAccountDeleteClock overrides the clock that stamps deleted_at and the
// session revocation. Tests use it; production code never needs to.
func WithAccountDeleteClock(fn func() time.Time) AccountDeleteOption {
	return func(s *AccountDeletionService) {
		s.now = fn
	}
}

// NewAccountDeletionService builds the service over q. tx may be nil, which
// runs the writes on q without a transaction (every test harness that does not
// wire one). It never dereferences q.
func NewAccountDeletionService(q AccountDeleteQuerier, tx AccountTxFunc, opts ...AccountDeleteOption) *AccountDeletionService {
	s := &AccountDeletionService{q: q, tx: tx, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// DeletedEmailFor is the address a deleted account is left with. The id keeps
// the UNIQUE email column satisfied and the reserved .invalid top level domain
// means it can never receive mail.
func DeletedEmailFor(accountID int64) string {
	return fmt.Sprintf("deleted-%d@%s", accountID, deletedEmailDomain)
}

// DeleteAccount deletes one account by Option A, anonymise.
//
// The rule: the accounts row stays and keeps its id, so every foreign key
// from votes, sessions, report flags and moderation actions stays valid. Its
// email becomes DeletedEmailFor(id) and its name, home area, terms consent and
// first run mark are cleared, and deleted_at is set. Every session of the
// account is revoked. The person's own rows are deleted: updates, update
// cursors, mutes, alert settings, and the magic link tokens and resend
// cooldown of the real address.
//
// Its effect on other people: none. Reports and votes are not touched, so they
// stay attributed to a row nobody can identify. Every confirmed count, every
// Resolve answer, every ClassifyOutcome result and every reporter rating that
// another account sees is unchanged, because the inputs of all of them (votes,
// sessions.account_id, report authorship) are unchanged. Stated plainly, the
// report text the person wrote and the voter cell at precision 7 remain. A
// moderator's moderation_actions rows stay valid, and MODERATOR_EMAILS no
// longer matches the scrubbed address.
//
// The steps run in one transaction when a tx func is set. The real address is
// read first, because the token and cooldown deletes need it and the scrub
// removes it. The scrub is last, so a failure before it leaves a usable
// account rather than a half deleted one.
//
// Q2 owner confirmed 2026-10-18: Option A. Option B (remove reports and votes, counts recompute) replaces this function's body.
func (s *AccountDeletionService) DeleteAccount(ctx context.Context, accountID int64) error {
	run := func(q AccountDeleteQuerier) error {
		row, err := q.AccountEmailForDelete(ctx, accountID)
		if err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: reading the account: %w", accountID, err)
		}
		if row.DeletedAt.Valid {
			return fmt.Errorf("service: DeleteAccount: account %d: %w", accountID, ErrAccountAlreadyDeleted)
		}
		if err := q.DeleteMagicLinkTokensForEmail(ctx, row.Email); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting sign in links: %w", accountID, err)
		}
		if err := q.DeleteEmailCooldownForEmail(ctx, row.Email); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting the resend cooldown: %w", accountID, err)
		}
		if err := q.DeleteAccountUpdates(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting updates: %w", accountID, err)
		}
		if err := q.DeleteAccountCursors(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting update cursors: %w", accountID, err)
		}
		if err := q.DeleteAccountMutes(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting mutes: %w", accountID, err)
		}
		if err := q.DeleteAccountAlertSettings(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting alert settings: %w", accountID, err)
		}
		now := pgtype.Timestamptz{Time: s.now(), Valid: true}
		if _, err := q.RevokeAccountSessions(ctx, sqlcgen.RevokeAccountSessionsParams{Now: now, AccountID: &accountID}); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: revoking sessions: %w", accountID, err)
		}
		rows, err := q.ScrubDeletedAccount(ctx, sqlcgen.ScrubDeletedAccountParams{
			ScrubbedEmail: DeletedEmailFor(accountID),
			Now:           now,
			ID:            accountID,
		})
		if err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: scrubbing the account: %w", accountID, err)
		}
		if rows == 0 {
			// A second request scrubbed it between the read and here.
			return fmt.Errorf("service: DeleteAccount: account %d: %w", accountID, ErrAccountAlreadyDeleted)
		}
		return nil
	}

	if s.tx != nil {
		return s.tx(ctx, run)
	}
	return run(s.q)
}
