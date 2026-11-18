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
	DeleteEmailCooldownForEmail(ctx context.Context, email string) error
	// Phase 7.1 plan 06 (T-07.1-22): the typed code rows of the real address and
	// its sign in throttle row go with the other personal rows.
	DeleteAuthCodesForEmail(ctx context.Context, email string) error
	ClearAuthThrottle(ctx context.Context, arg sqlcgen.ClearAuthThrottleParams) error
	// Phase 8 plan 08 (A-P8-6): a deleted account's words go (its comments are
	// blanked and marked deleted, and stay as placeholders where others
	// replied, so threads still read); the likes and comment flags it gave are
	// personal rows and are removed.
	BlankAccountComments(ctx context.Context, arg sqlcgen.BlankAccountCommentsParams) (int64, error)
	DeleteAccountCommentLikes(ctx context.Context, accountID int64) error
	DeleteAccountCommentFlags(ctx context.Context, accountID int64) error
	DeleteAccountUpdates(ctx context.Context, accountID int64) error
	DeleteAccountCursors(ctx context.Context, accountID int64) error
	DeleteAccountMutes(ctx context.Context, accountID int64) error
	DeleteAccountAlertSettings(ctx context.Context, accountID int64) error
	// DeleteAccountPushOutbox and DeleteAccountPushSubscriptions remove the
	// phone alert rows (Phase 3.3, decision 11): the account's subscriptions
	// are personal rows like its updates and mutes.
	DeleteAccountPushOutbox(ctx context.Context, accountID int64) (int64, error)
	// DeleteAccountOfficialPushOutbox removes the queued official alert pushes
	// (Phase 5 plan 05-11), a sibling table of the push outbox.
	DeleteAccountOfficialPushOutbox(ctx context.Context, accountID int64) (int64, error)
	DeleteAccountPushSubscriptions(ctx context.Context, accountID int64) (int64, error)
	// Phase 9 plan 02 (research G6): the person's follows and blocks, in both
	// directions, are hard deleted.
	DeleteAccountFollows(ctx context.Context, accountID int64) (int64, error)
	// Review IN-E06 (2026-10-30): the followers' queued follow phone notes about
	// the deleted person's reports go with the follows.
	DeleteQueuedFollowPushesForFollowee(ctx context.Context, followeeID int64) (int64, error)
	DeleteAccountBlocks(ctx context.Context, accountID int64) (int64, error)
	// Phase 9 plan 13 (research G6, Pitfall 11): the ratings the account gave and
	// the ratings it received are hard deleted.
	DeleteAccountRatings(ctx context.Context, accountID int64) (int64, error)
	// Phase 9 plan 18 (D-12, research G6): the mutes the account made and the
	// mutes of it are hard deleted.
	DeleteAccountPersonMutes(ctx context.Context, accountID int64) (int64, error)
	// Quick 261118-uoe: the notes the person wrote on their asks for review
	// are blanked; the ask rows stay.
	BlankAccountReviewAskNotes(ctx context.Context, accountID int64) (int64, error)
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
	q         AccountDeleteQuerier
	tx        AccountTxFunc
	now       func() time.Time
	throttleK func(email string) string
}

// AccountDeleteOption configures an AccountDeletionService at construction.
type AccountDeleteOption func(*AccountDeletionService)

// signInThrottleScope is the one scope of the auth_throttles table (migration
// 00014); the sign in service of plan 05 writes its rows under it.
const signInThrottleScope = "sign_in"

// WithThrottleKeyer sets the function that turns a real address into the key
// hash of its sign in throttle row (the same keyed hash the sign in service
// writes), so Delete my account can remove that row. Without a keyer no
// throttle row is cleared.
func WithThrottleKeyer(k func(email string) string) AccountDeleteOption {
	return func(s *AccountDeletionService) {
		s.throttleK = k
	}
}

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
// cursors, mutes, alert settings, the queued phone alerts and phone alert
// subscriptions (Phase 3.3), the queued official alert pushes (Phase 5), and
// the resend cooldown of the real address, and (Phase 7.1) its typed code rows and, when a throttle
// keyer is set, its sign in throttle row. The scrub also clears the password
// hash and frees the username.
//
// Amended 2026-10-29 by Phase 7.1 plan 16: the sign in link tokens step is
// gone, because the link token table was dropped (migration 00015).
//
// Amended 2026-10-29 by Phase 8 plan 08 (A-P8-6): right after the Phase 7.1
// steps the account's comments lose their words and get deleted_at, and the
// likes and comment flags it gave are deleted. A comment others replied to
// reads as a placeholder with no name afterwards.
//
// Amended 2026-10-29 by Phase 9 plan 02 (research G6): after the Phase 5 and 8
// steps and just before the sessions are revoked, the person's follows (both
// directions, active and ended) and blocks (both directions) are hard deleted,
// and the scrub switches the profile off. A deleted person has no public
// profile, so a row naming them in someone else's lists or block list would
// point at nobody. This is the one place deleting an account changes another
// person's lists: a follower's following list and a followee's follower list
// each lose the row.
//
// Amended 2026-10-29 by Phase 9 plan 13 (research G6, Pitfall 11): right after
// the follows and blocks, the star ratings the person gave and the ratings they
// received are hard deleted. A rating the person gave stops moving the rated
// reporter's stars at once, so deleting an account can change another
// reporter's score by removing one tapped rating (and can end that reporter's
// follows the next time the tag is read, if the tag drops).
//
// Amended 2026-11-18 by quick 261118-uoe: right after the person mutes, the
// notes the person wrote on their asks for review (quick 261118-rz6) are set to
// NULL, because a person's own written words go with the account the way their
// comments do. The ask rows themselves stay (report, time, answer, answering
// moderator), so the moderator's history and the cap of 3 asks per report stay
// correct, and an open ask stays open without its note.
//
// Its effect on other people: none beyond the follow, block and rating rows above.
// Reports and votes are not touched, so they
// stay attributed to a row nobody can identify. Every confirmed count, every
// Resolve answer, every ClassifyOutcome result and every reporter rating that
// another account sees is unchanged, because the inputs of all of them (votes,
// sessions.account_id, report authorship) are unchanged, except that a
// reporter's tapped ratings lose the ones this person gave (plan 09-13). Stated plainly, the
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
		if err := q.DeleteEmailCooldownForEmail(ctx, row.Email); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting the resend cooldown: %w", accountID, err)
		}
		// Phase 7.1 (T-07.1-22, research Pitfall 12): the typed code rows ever
		// written for the real address, and the address's sign in throttle row,
		// are personal rows too. The scrub below clears the password hash and
		// frees the username.
		if err := q.DeleteAuthCodesForEmail(ctx, row.Email); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting sign in codes: %w", accountID, err)
		}
		if s.throttleK != nil {
			err := q.ClearAuthThrottle(ctx, sqlcgen.ClearAuthThrottleParams{Scope: signInThrottleScope, KeyHash: s.throttleK(row.Email)})
			if err != nil {
				return fmt.Errorf("service: DeleteAccount: account %d: clearing the sign in throttle: %w", accountID, err)
			}
		}
		// Phase 8 plan 08 (A-P8-6, T-08-36): the account's words are blanked and
		// the likes and comment flags it gave are removed, in the same
		// transaction and before the scrub. The comment rows stay, so replies,
		// other people's likes and the audit trail keep their place.
		commentsNow := pgtype.Timestamptz{Time: s.now(), Valid: true}
		if _, err := q.BlankAccountComments(ctx, sqlcgen.BlankAccountCommentsParams{Now: commentsNow, AccountID: accountID}); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: blanking comments: %w", accountID, err)
		}
		if err := q.DeleteAccountCommentLikes(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting comment likes: %w", accountID, err)
		}
		if err := q.DeleteAccountCommentFlags(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting comment flags: %w", accountID, err)
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
		// Phase 3.3, decision 11: the account's phone alert rows are personal
		// rows like its updates and mutes. The outbox goes first, then the
		// subscriptions, inside the same transaction and before the scrub.
		if _, err := q.DeleteAccountPushOutbox(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting queued phone alerts: %w", accountID, err)
		}
		// Phase 5 plan 05-11 (T-05-62): the official alert pushes queued for the
		// account live in their own table and go with the rest, in the same
		// transaction and before the scrub.
		if _, err := q.DeleteAccountOfficialPushOutbox(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting queued official alerts: %w", accountID, err)
		}
		if _, err := q.DeleteAccountPushSubscriptions(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting push subscriptions: %w", accountID, err)
		}
		// Phase 9 plan 02 (research G6, T-09-07): the person's place in the social
		// graph goes in the same transaction and before the scrub, after every
		// earlier step.
		// Review IN-E06 (2026-10-30): the followers' queued follow notes about this
		// person's reports are deleted first, while the report to person join can
		// still be read; a note a worker already holds is stopped by its send time
		// check, which finds no follow.
		if _, err := q.DeleteQueuedFollowPushesForFollowee(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting queued follow notes: %w", accountID, err)
		}
		if _, err := q.DeleteAccountFollows(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting follows: %w", accountID, err)
		}
		if _, err := q.DeleteAccountBlocks(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting blocks: %w", accountID, err)
		}
		// Phase 9 plan 13 (research G6, Pitfall 11): the ratings the person gave
		// stop moving other people's stars, and the ratings they received go with
		// their profile. Same transaction, before the scrub.
		if _, err := q.DeleteAccountRatings(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting ratings: %w", accountID, err)
		}
		// Phase 9 plan 18 (D-12, research G6): the mutes the person made and the
		// mutes of them. Same transaction, before the scrub, so no mute row
		// outlives the account it names.
		if _, err := q.DeleteAccountPersonMutes(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: deleting person mutes: %w", accountID, err)
		}
		// Quick 261118-uoe: the person's notes on their asks for review go, the
		// asks stay. Same transaction, before the scrub.
		if _, err := q.BlankAccountReviewAskNotes(ctx, accountID); err != nil {
			return fmt.Errorf("service: DeleteAccount: account %d: blanking review ask notes: %w", accountID, err)
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
