package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/auth"
	sqlcgen "pinalert/internal/store/sqlc"
)

// SignInResult is what a handler needs after a good sign in or a confirmed
// sign up code: the account, and the session id to put in the cookie. The id
// is always a freshly drawn one; the id of the request is never bound (D-08,
// session fixation, research Pattern 3).
type SignInResult struct {
	AccountID int64
	SessionID string
}

// LiveCode describes the code screen a browser can be shown again after a
// reload: which kind it is, the address the code was sent to (page text only,
// never a URL, D-05) and how many seconds are left to type it.
type LiveCode struct {
	Purpose     string
	Email       string
	SecondsLeft int
	// ResendSecondsLeft is what is left of the resend cooldown since the row's
	// send, rounded up and never negative (review WR-B02), so a reload or a
	// return from the mail app shows the wait that is really left.
	ResendSecondsLeft int
}

// spentCodeMemory is how long after the spend SpentCode still reports a
// confirmed code: long enough for a reload after a lost answer, short enough
// that the next person on a shared phone sees neither the address nor the line
// (review fix round 4, skeptic SK3-02).
const spentCodeMemory = 10 * time.Minute

// SpentCode describes a code this browser itself confirmed a short time ago
// (Phase 7.1 review fix round 3, skeptic SK2-01): which kind it was and
// the address it was sent to (page text only, never a URL, D-05). A confirm
// whose answer was lost leaves the browser on the session id it sent the code
// with, so the sign in page can still tell it what happened.
type SpentCode struct {
	Purpose string
	Email   string
}

// ThrottleKey is the stored form of an address in the sign in throttle: an
// HMAC under the throttle key, so the database holds no address and an unknown
// address behaves exactly like a known one. Delete my account clears the
// throttle with it.
func (s *CredentialService) ThrottleKey(email string) string {
	norm := strings.ToLower(strings.TrimSpace(email))
	if n, err := normalizeEmail(email); err == nil {
		norm = n
	}
	return auth.KeyedHash(s.throttleKey, throttleLabel, norm)
}

// cleanTypedCode drops every space (U+0020 and U+00A0) from what was typed, so
// a pasted "482 913" is "482913". The page script trims too, but a browser
// maxlength can cut a paste, so the server does it as well. ok is false when
// what is left is not exactly 6 ASCII digits.
func cleanTypedCode(raw string) (string, bool) {
	code := strings.Map(func(r rune) rune {
		if r == ' ' || r == 0xA0 {
			return -1
		}
		return r
	}, raw)
	if len(code) != auth.CodeDigits {
		return "", false
	}
	for i := 0; i < len(code); i++ {
		if code[i] < '0' || code[i] > '9' {
			return "", false
		}
	}
	return code, true
}

// proveCode is the shared front half of ConfirmSignUp and ConfirmReset. It
// counts the try before the compare (the bump is one atomic UPDATE, so
// parallel guessing cannot beat the budget), checks the keyed hash, and refuses
// any row that is not a delivery of code, so an account_exists or no_account row
// can never be confirmed. Every refusal is ErrWrongCode.
func (s *CredentialService) proveCode(ctx context.Context, sessionID, purpose, code string) (sqlcgen.BumpAuthCodeAttemptRow, error) {
	row, err := s.q.BumpAuthCodeAttempt(ctx, sqlcgen.BumpAuthCodeAttemptParams{
		SessionID:   sessionID,
		Purpose:     purpose,
		MaxAttempts: int32(s.limits.CodeMaxAttempts),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.BumpAuthCodeAttemptRow{}, ErrWrongCode
		}
		return sqlcgen.BumpAuthCodeAttemptRow{}, fmt.Errorf("service: counting code try: %w", err)
	}
	matches := auth.CodeHashEqual(auth.CodeHash(s.codeKey, purpose, row.Email, code), row.CodeHash)
	if !matches || row.Delivery != DeliveryCode {
		return sqlcgen.BumpAuthCodeAttemptRow{}, ErrWrongCode
	}
	return row, nil
}

// inTx runs fn inside one transaction through the configured runner. Without a
// runner (only the unit test fakes) fn runs on the service's own querier.
func (s *CredentialService) inTx(ctx context.Context, fn func(q CredentialQuerier) error) error {
	if s.tx == nil {
		return fn(s.q)
	}
	return s.tx(ctx, fn)
}

// spend consumes the proven row exactly once and deletes every other code row
// of the address, so a code still pending in another browser can no longer be
// used (W9). 0 rows consumed means a parallel submit won. It runs inside the
// confirm transaction, so a later failure leaves the code usable (review
// CR-S02, IN-S07). The spent row itself is kept, used and with no password
// hash, so SpentCode can tell this browser its code was confirmed (review fix
// round 3, skeptic SK2-01).
func spend(ctx context.Context, q CredentialQuerier, row sqlcgen.BumpAuthCodeAttemptRow) error {
	n, err := q.ConsumeAuthCode(ctx, row.ID)
	if err != nil {
		return fmt.Errorf("service: consuming code: %w", err)
	}
	if n == 0 {
		return ErrWrongCode
	}
	if err := q.DeleteOtherAuthCodesForEmail(ctx, sqlcgen.DeleteOtherAuthCodesForEmailParams{Email: row.Email, KeepID: row.ID}); err != nil {
		return fmt.Errorf("service: deleting spent codes: %w", err)
	}
	return nil
}

// lockAddress is the first statement of both confirm transactions (review fix
// skeptic SK-05): one advisory lock per normalised address, keyed by its
// throttle key, so a sign up confirm and a reset confirm for one address, or
// two sign up confirms, run one after the other instead of taking the account
// row and the code rows in opposite orders and deadlocking.
func (s *CredentialService) lockAddress(ctx context.Context, q CredentialQuerier, email string) error {
	if err := q.LockCredentialAddress(ctx, s.ThrottleKey(email)); err != nil {
		return fmt.Errorf("service: locking the address: %w", err)
	}
	return nil
}

// ConfirmSignUp is the typed code of Create account. A right code creates the
// account, or keeps the account made with the old link and so its reports,
// sets the password typed at Create account, clears the sign in throttle and
// signs the browser in on a freshly drawn session id.
//
// Everything after the code is proved runs in one transaction (Phase 7.1
// review CR-S02, IN-S07): the spend, the account, the password, the throttle
// and the bind, so a failure leaves the code usable and nothing half written.
// It first takes the address lock the reset confirm takes too (review fix
// skeptic SK-05), so the two never deadlock. When the account already existed (made with the old link, or one that gained
// a password meanwhile) the password is set with a new epoch, under the account
// row lock, and every session and phone alert device of the account is ended,
// so a session the old binary made from an emailed link, or one opened with a
// replaced password, is never signed in (WR-S02).
func (s *CredentialService) ConfirmSignUp(ctx context.Context, sessionID, rawCode string, newID func() string) (SignInResult, error) {
	code, ok := cleanTypedCode(rawCode)
	if !ok {
		return SignInResult{}, ErrWrongCode
	}
	row, err := s.proveCode(ctx, sessionID, PurposeSignUp, code)
	if err != nil {
		return SignInResult{}, err
	}
	if row.PasswordHash == nil {
		// A code row of delivery code always carries the password hash.
		return SignInResult{}, ErrWrongCode
	}
	var result SignInResult
	err = s.inTx(ctx, func(q CredentialQuerier) error {
		if err := s.lockAddress(ctx, q, row.Email); err != nil {
			return err
		}
		if err := spend(ctx, q, row); err != nil {
			return err
		}
		var accountID, epoch int64
		existing, err := q.LockAccountForPassword(ctx, row.Email)
		switch {
		case err == nil:
			accountID = existing.ID
			epoch, err = s.setPasswordNewEpoch(ctx, q, accountID, *row.PasswordHash)
			if err != nil {
				return err
			}
			// Nothing was created here, only a password set. Mark the kept row so
			// a page reloaded after a lost answer says "Password set" (review
			// fix round 4, skeptic SK3-03). The confirm knows this branch; the
			// page never asks the account table.
			if err := q.MarkAuthCodePasswordSet(ctx, row.ID); err != nil {
				return fmt.Errorf("service: marking the code as a password set: %w", err)
			}
			if _, err := NewSessionService(q, WithSessionClock(s.now)).LogOutAllDevices(ctx, accountID); err != nil {
				return err
			}
		case errors.Is(err, pgx.ErrNoRows):
			if err := q.InsertAccount(ctx, row.Email); err != nil {
				return fmt.Errorf("service: creating account: %w", err)
			}
			created, err := q.LockAccountForPassword(ctx, row.Email)
			if err != nil {
				return fmt.Errorf("service: reading the new account: %w", err)
			}
			accountID, epoch = created.ID, created.PasswordEpoch
			if err := setPassword(ctx, q, accountID, *row.PasswordHash); err != nil {
				return err
			}
		default:
			return fmt.Errorf("service: locking account: %w", err)
		}
		if err := s.clearThrottle(ctx, q, row.Email); err != nil {
			return err
		}
		result, err = s.rotate(ctx, q, sessionID, accountID, epoch, newID)
		return err
	})
	if err != nil {
		return SignInResult{}, err
	}
	return result, nil
}

// SignIn takes the email and the password only. Every try is counted in the
// database before the password is checked, in one guarded statement, so a
// parallel burst cannot run more checks than the lock allows (Phase 7.1 review
// CR-S01); a locked address is refused before any hashing. An unknown address
// and an account with no password are counted and verified against the dummy
// hash the same way, so neither answers faster than a wrong password, and all
// three get the one error. A good sign in clears the count, upgrades an old
// hash and rotates the session id. A busy hasher never checked the password,
// so it gives the counted try back (review fix skeptic SK-02), the same for
// every address; the lock punishes wrong passwords only. The per device and
// per network buckets are the handler's job.
func (s *CredentialService) SignIn(ctx context.Context, sessionID, rawEmail, password string, newID func() string) (SignInResult, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return SignInResult{}, ValidationError{Field: "email", Message: InvalidEmailMessage}
	}
	key := s.ThrottleKey(email)

	try, err := s.reserveTry(ctx, key)
	if err != nil {
		return SignInResult{}, err
	}

	account, found, err := s.lookupAccount(ctx, email)
	if err != nil {
		return SignInResult{}, err
	}
	usable := found && account.PasswordHash != nil
	stored := s.hasher.DummyHash()
	if usable {
		stored = *account.PasswordHash
	}
	normalized := auth.NormalizePassword(password)
	good, needsRehash, err := s.hasher.Verify(ctx, stored, normalized)
	if err != nil {
		if errors.Is(err, auth.ErrHasherBusy) {
			// Best effort: a failed give back still answers ErrBusy, so
			// every address gets the same answer. The guarded statement
			// never undoes a later try.
			_, _ = s.q.RefundSignInTry(ctx, sqlcgen.RefundSignInTryParams{
				Scope:         throttleScope,
				KeyHash:       key,
				Failures:      try.Failures,
				LastFailureAt: try.LastFailureAt,
			})
			return SignInResult{}, ErrBusy
		}
		if !errors.Is(err, auth.ErrMalformedHash) {
			return SignInResult{}, fmt.Errorf("service: verifying password: %w", err)
		}
		// A damaged stored hash counts as a wrong password; the person can
		// recover through Forgot password.
		good = false
	}
	if !(good && usable) {
		// The try was already counted by reserveTry.
		return SignInResult{}, ErrWrongCredentials
	}

	// Bind first, then clear the count, so a refused bind never clears a lock.
	// The bind holds to the password epoch read before the check (review
	// CR-S02): when a reset or a later Create account replaced the password
	// while this one was being checked, nothing is bound and the answer is the
	// one wrong credentials answer.
	result, err := s.rotate(ctx, s.q, sessionID, account.ID, account.PasswordEpoch, newID)
	if err != nil {
		if errors.Is(err, errBindRefused) {
			return SignInResult{}, ErrWrongCredentials
		}
		return SignInResult{}, err
	}
	if err := s.q.ClearAuthThrottle(ctx, sqlcgen.ClearAuthThrottleParams{Scope: throttleScope, KeyHash: key}); err != nil {
		return SignInResult{}, fmt.Errorf("service: clearing sign in throttle: %w", err)
	}
	if needsRehash {
		// Best effort: a failed upgrade, a busy hasher included, never fails a
		// good sign in. The next sign in tries again. The write holds to the
		// epoch too, so it never overwrites a password set meanwhile.
		if fresh, err := s.hasher.Hash(ctx, normalized); err == nil {
			_, _ = s.q.SetAccountPasswordIfEpoch(ctx, sqlcgen.SetAccountPasswordIfEpochParams{
				PasswordHash: fresh, ID: account.ID, PasswordEpoch: account.PasswordEpoch,
			})
		}
	}
	return result, nil
}

// reserveTry counts one sign in try for the address before the password check
// (CR-S01). It returns SignInLockedError when the address is locked. When the
// guarded statement refused but a fresh read finds the address no longer
// locked (a reset cleared it, or the lock ended in between), it tries once
// more; a second refusal is answered as locked with at least one second to
// wait, never a negative wait. The counted row comes back so a busy hasher can
// give the try back with RefundSignInTry.
func (s *CredentialService) reserveTry(ctx context.Context, key string) (sqlcgen.ReserveSignInTryRow, error) {
	for attempt := 0; attempt < 2; attempt++ {
		now := s.now()
		row, err := s.q.ReserveSignInTry(ctx, sqlcgen.ReserveSignInTryParams{
			Scope:           throttleScope,
			KeyHash:         key,
			Now:             now,
			ForgetCutoff:    now.Add(-s.limits.ForgetAfter),
			FreeFailures:    int32(s.limits.FreeFailures),
			BaseLockSeconds: int32(s.limits.BaseLock / time.Second),
			MaxLockSeconds:  int32(s.limits.MaxLock / time.Second),
		})
		if err == nil {
			return row, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.ReserveSignInTryRow{}, fmt.Errorf("service: counting sign in try: %w", err)
		}
		t, err := s.q.GetAuthThrottle(ctx, sqlcgen.GetAuthThrottleParams{Scope: throttleScope, KeyHash: key})
		switch {
		case err == nil:
			if t.LockedUntil.Valid && t.LockedUntil.Time.After(now) {
				return sqlcgen.ReserveSignInTryRow{}, SignInLockedError{Wait: t.LockedUntil.Time.Sub(now)}
			}
		case errors.Is(err, pgx.ErrNoRows):
		default:
			return sqlcgen.ReserveSignInTryRow{}, fmt.Errorf("service: reading sign in throttle: %w", err)
		}
	}
	return sqlcgen.ReserveSignInTryRow{}, SignInLockedError{Wait: time.Second}
}

// ConfirmReset is the typed code of Forgot password, with the new password.
// A bad new password is refused before any try is counted. A right code sets
// the NFKC form of the new password, deletes the address's code rows, ends
// every session and push subscription of the account, and clears the throttle
// (a locked address can always be recovered this way). It never signs the
// browser in: the person signs in with the new password.
//
// The writes run in one transaction, after the address lock both confirms take
// first (review fix skeptic SK-05), the password first (Phase 7.1 review
// CR-S02): the password is set with a new epoch, which takes the account row
// lock, then the code is spent, the throttle cleared and every session ended.
// A sign in that read the old epoch cannot bind after it, and the gate refuses
// any session still on the old epoch. A failure anywhere changes nothing and
// leaves the code usable.
func (s *CredentialService) ConfirmReset(ctx context.Context, sessionID, rawCode, newPassword string) error {
	latest, err := s.q.GetLatestAuthCode(ctx, sqlcgen.GetLatestAuthCodeParams{
		SessionID: sessionID,
		Since:     s.now().Add(-s.limits.ResumeWindow),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrWrongCode
		}
		return fmt.Errorf("service: reading latest code: %w", err)
	}
	if latest.Purpose != PurposeReset || latest.UsedAt.Valid {
		return ErrWrongCode
	}
	if err := checkPassword(newPassword, latest.Email); err != nil {
		return err
	}
	code, ok := cleanTypedCode(rawCode)
	if !ok {
		return ErrWrongCode
	}
	row, err := s.proveCode(ctx, sessionID, PurposeReset, code)
	if err != nil {
		return err
	}
	// Everything that can fail softly runs before the code is spent, so a busy
	// hasher or a vanished account does not burn a right code.
	account, found, err := s.lookupAccount(ctx, row.Email)
	if err != nil {
		return err
	}
	if !found {
		return ErrWrongCode
	}
	hash, err := s.hasher.Hash(ctx, auth.NormalizePassword(newPassword))
	if err != nil {
		return s.hashError(err)
	}
	return s.inTx(ctx, func(q CredentialQuerier) error {
		if err := s.lockAddress(ctx, q, row.Email); err != nil {
			return err
		}
		if _, err := s.setPasswordNewEpoch(ctx, q, account.ID, hash); err != nil {
			return err
		}
		if err := spend(ctx, q, row); err != nil {
			return err
		}
		if err := s.clearThrottle(ctx, q, row.Email); err != nil {
			return err
		}
		if _, err := NewSessionService(q, WithSessionClock(s.now)).LogOutAllDevices(ctx, account.ID); err != nil {
			return err
		}
		return nil
	})
}

// setPassword stores the first password of a brand new account without moving
// its epoch; 0 rows means the account is deleted.
func setPassword(ctx context.Context, q CredentialQuerier, accountID int64, hash string) error {
	n, err := q.SetAccountPassword(ctx, sqlcgen.SetAccountPasswordParams{PasswordHash: hash, ID: accountID})
	if err != nil {
		return fmt.Errorf("service: setting password of account %d: %w", accountID, err)
	}
	if n == 0 {
		return fmt.Errorf("service: account %d is gone, password not set", accountID)
	}
	return nil
}

// setPasswordNewEpoch stores a password on an account that already existed and
// moves its password epoch (review CR-S02), returning the new epoch.
func (s *CredentialService) setPasswordNewEpoch(ctx context.Context, q CredentialQuerier, accountID int64, hash string) (int64, error) {
	epoch, err := q.SetAccountPasswordNewEpoch(ctx, sqlcgen.SetAccountPasswordNewEpochParams{PasswordHash: hash, ID: accountID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("service: account %d is gone, password not set", accountID)
		}
		return 0, fmt.Errorf("service: setting password of account %d: %w", accountID, err)
	}
	return epoch, nil
}

func (s *CredentialService) clearThrottle(ctx context.Context, q CredentialQuerier, email string) error {
	if err := q.ClearAuthThrottle(ctx, sqlcgen.ClearAuthThrottleParams{Scope: throttleScope, KeyHash: s.ThrottleKey(email)}); err != nil {
		return fmt.Errorf("service: clearing sign in throttle: %w", err)
	}
	return nil
}

// errBindRefused means the fresh session id could not be bound: the account's
// password epoch moved since it was read, the account is gone, or the id is
// already bound elsewhere. SignIn answers it as wrong credentials.
var errBindRefused = errors.New("service: binding a fresh session was refused")

// rotate signs the browser in on a freshly drawn session id (research Pattern
// 3). The request's own id is never bound, so a cookie planted before the sign
// in gains nothing. When the request's session was bound to a different
// account, its phone alert subscriptions are deleted and the session is
// revoked, so the next person on a shared phone never gets the previous
// person's alerts and the old cookie stops working (Pitfall 10, W8). A session
// that is unbound, unstored or bound to the same account is left alone. The
// fresh id is bound at the password epoch the caller read (review CR-S02); a
// refused bind is errBindRefused. The handler sets the cookie.
//
// The bind comes first and the revoke after it (review fix skeptic SK-07), so a
// refused bind leaves the browser signed in to its previous account with its
// alerts. Outside a transaction (SignIn), a revoke that fails after a good bind
// returns the error and leaves a bound id no cookie ever carries, which nobody
// can use.
func (s *CredentialService) rotate(ctx context.Context, q CredentialQuerier, oldID string, accountID, epoch int64, newID func() string) (SignInResult, error) {
	replaced := false
	binding, err := q.GetSessionBinding(ctx, oldID)
	switch {
	case err == nil:
		replaced = binding.AccountID != nil && *binding.AccountID != accountID
	case errors.Is(err, pgx.ErrNoRows):
		// The session row was never stored: unbound.
	default:
		return SignInResult{}, fmt.Errorf("service: reading session binding: %w", err)
	}

	id := newID()
	n, err := q.BindSessionAccountAtEpoch(ctx, sqlcgen.BindSessionAccountAtEpochParams{
		SessionID: id, AccountID: accountID, PasswordEpoch: epoch,
	})
	if err != nil {
		return SignInResult{}, fmt.Errorf("service: binding session to account %d: %w", accountID, err)
	}
	if n == 0 {
		return SignInResult{}, errBindRefused
	}

	if replaced {
		if _, err := q.DeletePushSubscriptionsForSession(ctx, oldID); err != nil {
			return SignInResult{}, fmt.Errorf("service: deleting push subscriptions of the replaced session: %w", err)
		}
		if _, err := q.RevokeSession(ctx, sqlcgen.RevokeSessionParams{Now: s.now(), SessionID: oldID}); err != nil {
			return SignInResult{}, fmt.Errorf("service: revoking the replaced session: %w", err)
		}
	}
	return SignInResult{AccountID: accountID, SessionID: id}, nil
}

// LiveCode is the thin read behind the sign in page: the newest unconsumed
// code row of this browser inside the resume window, so a reload shows the
// code screen again. SecondsLeft is rounded up and never negative; an expired
// row is still returned with 0 so the page can say the code has expired. A
// consumed or older row is false.
func (s *CredentialService) LiveCode(ctx context.Context, sessionID string) (LiveCode, bool, error) {
	now := s.now()
	row, err := s.q.GetLatestAuthCode(ctx, sqlcgen.GetLatestAuthCodeParams{
		SessionID: sessionID,
		Since:     now.Add(-s.limits.ResumeWindow),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LiveCode{}, false, nil
		}
		return LiveCode{}, false, fmt.Errorf("service: reading latest code: %w", err)
	}
	if row.UsedAt.Valid {
		return LiveCode{}, false, nil
	}
	// The send time is the stored expiry minus the lifetime: both come from
	// this service's clock, while created_at is the database's (review WR-B02).
	sentAt := row.ExpiresAt.Add(-s.limits.CodeTTL)
	return LiveCode{
		Purpose:           row.Purpose,
		Email:             row.Email,
		SecondsLeft:       secondsUntil(now, row.ExpiresAt),
		ResendSecondsLeft: secondsUntil(now, sentAt.Add(s.limits.SendCooldown)),
	}, true, nil
}

// SpentCode is the read behind the sign in page after a code submit that ran
// out of time (review fix round 3, skeptic SK2-01). It is true only when this
// browser's own newest code row inside the resume window was spent by a
// confirm: a delivery of code (an account_exists or no_account row can never be
// confirmed), counted at least once, and used at the very moment its life
// ended, the pair ConsumeAuthCode alone writes. A live row, a row retired
// because its email did not go out, and a row of any other browser are false.
// It reads nothing about the address's account, so it tells this browser only
// what its own right code already did.
//
// It counts a spent code only for spentCodeMemory after the spend, not for the
// whole resume window (review fix round 4, skeptic SK3-02): ConfirmReset keeps
// the browser's session id, so a longer memory would show the address and the
// line to the next person on a shared phone. The lost answer case is a reload
// within seconds or a few minutes, which this still covers. used_at is the
// transaction's now(), and expires_at equals it, so the spend time is read from
// the row.
func (s *CredentialService) SpentCode(ctx context.Context, sessionID string) (SpentCode, bool, error) {
	now := s.now()
	row, err := s.q.GetLatestAuthCode(ctx, sqlcgen.GetLatestAuthCodeParams{
		SessionID: sessionID,
		Since:     now.Add(-s.limits.ResumeWindow),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SpentCode{}, false, nil
		}
		return SpentCode{}, false, fmt.Errorf("service: reading latest code: %w", err)
	}
	if !row.UsedAt.Valid || row.Delivery != DeliveryCode || row.Attempts < 1 || !row.UsedAt.Time.Equal(row.ExpiresAt) {
		return SpentCode{}, false, nil
	}
	if !row.UsedAt.Time.After(now.Add(-spentCodeMemory)) {
		return SpentCode{}, false, nil
	}
	return SpentCode{Purpose: row.Purpose, Email: row.Email}, true, nil
}

// secondsUntil is the whole seconds from now to deadline, rounded up, and 0
// once the deadline has passed.
func secondsUntil(now, deadline time.Time) int {
	left := deadline.Sub(now)
	if left <= 0 {
		return 0
	}
	return int((left + time.Second - 1) / time.Second)
}

// SignedIn is true only for a live session of an account that has a password
// (W1): a session made after the migration on an older account is not let in,
// so the one time fill in cannot be skipped.
func (s *CredentialService) SignedIn(ctx context.Context, sessionID string) (bool, error) {
	row, err := s.q.GetAccountBySessionID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("service: reading session account: %w", err)
	}
	return row.HasPassword, nil
}

// EndedByPasswordSwitch tells a browser only about its own session: true when
// it is bound to a live account that has no password yet.
func (s *CredentialService) EndedByPasswordSwitch(ctx context.Context, sessionID string) (bool, error) {
	ended, err := s.q.SessionEndedByPasswordSwitch(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("service: reading password switch: %w", err)
	}
	return ended, nil
}
