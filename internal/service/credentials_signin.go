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

// spend consumes the proven row exactly once and deletes every other code row
// of the address, so a code still pending in another browser can no longer be
// used (W9). 0 rows consumed means a parallel submit won.
func (s *CredentialService) spend(ctx context.Context, row sqlcgen.BumpAuthCodeAttemptRow) error {
	n, err := s.q.ConsumeAuthCode(ctx, row.ID)
	if err != nil {
		return fmt.Errorf("service: consuming code: %w", err)
	}
	if n == 0 {
		return ErrWrongCode
	}
	if err := s.q.DeleteAuthCodesForEmail(ctx, row.Email); err != nil {
		return fmt.Errorf("service: deleting spent codes: %w", err)
	}
	return nil
}

// ConfirmSignUp is the typed code of Create account. A right code creates the
// account, or keeps the account made with the old link and so its reports,
// sets the password typed at Create account, clears the sign in throttle and
// signs the browser in on a freshly drawn session id.
//
// If the account already has a password when the code is confirmed (a race
// between two Create account requests), LogOutAllDevices runs before the
// password is replaced, so no session opened with the old password survives.
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
	if err := s.spend(ctx, row); err != nil {
		return SignInResult{}, err
	}
	if err := s.q.InsertAccount(ctx, row.Email); err != nil {
		return SignInResult{}, fmt.Errorf("service: creating account: %w", err)
	}
	account, found, err := s.lookupAccount(ctx, row.Email)
	if err != nil {
		return SignInResult{}, err
	}
	if !found {
		return SignInResult{}, errors.New("service: account missing after insert")
	}
	if account.PasswordHash != nil {
		if _, err := s.sessions.LogOutAllDevices(ctx, account.ID); err != nil {
			return SignInResult{}, err
		}
	}
	if err := s.setPassword(ctx, account.ID, *row.PasswordHash); err != nil {
		return SignInResult{}, err
	}
	if err := s.clearThrottle(ctx, row.Email); err != nil {
		return SignInResult{}, err
	}
	return s.rotate(ctx, sessionID, account.ID, newID)
}

// SignIn takes the email and the password only. Every try is counted in the
// database before the password is checked, in one guarded statement, so a
// parallel burst cannot run more checks than the lock allows (Phase 7.1 review
// CR-S01); a locked address is refused before any hashing. An unknown address
// and an account with no password are counted and verified against the dummy
// hash the same way, so neither answers faster than a wrong password, and all
// three get the one error. A good sign in clears the count, upgrades an old
// hash and rotates the session id. A busy hasher keeps the counted try, the
// same for every address. The per device and per network buckets are the
// handler's job.
func (s *CredentialService) SignIn(ctx context.Context, sessionID, rawEmail, password string, newID func() string) (SignInResult, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return SignInResult{}, ValidationError{Field: "email", Message: InvalidEmailMessage}
	}
	key := s.ThrottleKey(email)

	if err := s.reserveTry(ctx, key); err != nil {
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
	result, err := s.rotate(ctx, sessionID, account.ID, newID)
	if err != nil {
		return SignInResult{}, err
	}
	if err := s.q.ClearAuthThrottle(ctx, sqlcgen.ClearAuthThrottleParams{Scope: throttleScope, KeyHash: key}); err != nil {
		return SignInResult{}, fmt.Errorf("service: clearing sign in throttle: %w", err)
	}
	if needsRehash {
		// Best effort: a failed upgrade, a busy hasher included, never fails a
		// good sign in. The next sign in tries again.
		if fresh, err := s.hasher.Hash(ctx, normalized); err == nil {
			_, _ = s.q.SetAccountPassword(ctx, sqlcgen.SetAccountPasswordParams{PasswordHash: fresh, ID: account.ID})
		}
	}
	return result, nil
}

// reserveTry counts one sign in try for the address before the password check
// (CR-S01). It returns SignInLockedError when the address is locked. When the
// guarded statement refused but a fresh read finds the address no longer
// locked (a reset cleared it, or the lock ended in between), it tries once
// more; a second refusal is answered as locked with at least one second to
// wait, never a negative wait.
func (s *CredentialService) reserveTry(ctx context.Context, key string) error {
	for attempt := 0; attempt < 2; attempt++ {
		now := s.now()
		_, err := s.q.ReserveSignInTry(ctx, sqlcgen.ReserveSignInTryParams{
			Scope:           throttleScope,
			KeyHash:         key,
			Now:             now,
			ForgetCutoff:    now.Add(-s.limits.ForgetAfter),
			FreeFailures:    int32(s.limits.FreeFailures),
			BaseLockSeconds: int32(s.limits.BaseLock / time.Second),
			MaxLockSeconds:  int32(s.limits.MaxLock / time.Second),
		})
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("service: counting sign in try: %w", err)
		}
		t, err := s.q.GetAuthThrottle(ctx, sqlcgen.GetAuthThrottleParams{Scope: throttleScope, KeyHash: key})
		switch {
		case err == nil:
			if t.LockedUntil.Valid && t.LockedUntil.Time.After(now) {
				return SignInLockedError{Wait: t.LockedUntil.Time.Sub(now)}
			}
		case errors.Is(err, pgx.ErrNoRows):
		default:
			return fmt.Errorf("service: reading sign in throttle: %w", err)
		}
	}
	return SignInLockedError{Wait: time.Second}
}

// ConfirmReset is the typed code of Forgot password, with the new password.
// A bad new password is refused before any try is counted. A right code sets
// the NFKC form of the new password, deletes the address's code rows, ends
// every session and push subscription of the account, and clears the throttle
// (a locked address can always be recovered this way). It never signs the
// browser in: the person signs in with the new password.
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
	if err := s.spend(ctx, row); err != nil {
		return err
	}
	if err := s.setPassword(ctx, account.ID, hash); err != nil {
		return err
	}
	if err := s.clearThrottle(ctx, row.Email); err != nil {
		return err
	}
	if _, err := s.sessions.LogOutAllDevices(ctx, account.ID); err != nil {
		return err
	}
	return nil
}

// setPassword stores a password hash; 0 rows means the account is deleted.
func (s *CredentialService) setPassword(ctx context.Context, accountID int64, hash string) error {
	n, err := s.q.SetAccountPassword(ctx, sqlcgen.SetAccountPasswordParams{PasswordHash: hash, ID: accountID})
	if err != nil {
		return fmt.Errorf("service: setting password of account %d: %w", accountID, err)
	}
	if n == 0 {
		return fmt.Errorf("service: account %d is gone, password not set", accountID)
	}
	return nil
}

func (s *CredentialService) clearThrottle(ctx context.Context, email string) error {
	if err := s.q.ClearAuthThrottle(ctx, sqlcgen.ClearAuthThrottleParams{Scope: throttleScope, KeyHash: s.ThrottleKey(email)}); err != nil {
		return fmt.Errorf("service: clearing sign in throttle: %w", err)
	}
	return nil
}

// rotate signs the browser in on a freshly drawn session id (research Pattern
// 3). The request's own id is never bound, so a cookie planted before the sign
// in gains nothing. When the request's session was bound to a different
// account, its phone alert subscriptions are deleted and the session is
// revoked first, so the next person on a shared phone never gets the previous
// person's alerts and the old cookie stops working (Pitfall 10, W8). A session
// that is unbound, unstored or bound to the same account is left alone. The
// handler sets the cookie.
func (s *CredentialService) rotate(ctx context.Context, oldID string, accountID int64, newID func() string) (SignInResult, error) {
	binding, err := s.q.GetSessionBinding(ctx, oldID)
	switch {
	case err == nil:
		if binding.AccountID != nil && *binding.AccountID != accountID {
			if _, err := s.q.DeletePushSubscriptionsForSession(ctx, oldID); err != nil {
				return SignInResult{}, fmt.Errorf("service: deleting push subscriptions of the replaced session: %w", err)
			}
			if _, err := s.q.RevokeSession(ctx, sqlcgen.RevokeSessionParams{Now: s.now(), SessionID: oldID}); err != nil {
				return SignInResult{}, fmt.Errorf("service: revoking the replaced session: %w", err)
			}
		}
	case errors.Is(err, pgx.ErrNoRows):
		// The session row was never stored: unbound.
	default:
		return SignInResult{}, fmt.Errorf("service: reading session binding: %w", err)
	}

	id := newID()
	n, err := s.q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{SessionID: id, AccountID: &accountID})
	if err != nil {
		return SignInResult{}, fmt.Errorf("service: binding session to account %d: %w", accountID, err)
	}
	if n == 0 {
		return SignInResult{}, fmt.Errorf("service: binding a fresh session to account %d was refused", accountID)
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
	left := row.ExpiresAt.Sub(now)
	secs := int((left + time.Second - 1) / time.Second)
	if left <= 0 || secs < 0 {
		secs = 0
	}
	return LiveCode{Purpose: row.Purpose, Email: row.Email, SecondsLeft: secs}, true, nil
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
