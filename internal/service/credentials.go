package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/auth"
	"pinalert/internal/mailer"
	sqlcgen "pinalert/internal/store/sqlc"
)

// CredentialQuerier is the subset of the sqlc-generated Queries type
// CredentialService needs, small enough to fake without a real Postgres. It
// embeds SessionQuerier because ConfirmReset ends every session of the account
// through SessionService.LogOutAllDevices.
type CredentialQuerier interface {
	SessionQuerier

	// The per address send claim: one atomic write for the 45 second cooldown
	// and the hourly count of every auth email kind. A refused claim surfaces
	// as pgx.ErrNoRows.
	ClaimAuthEmailSend(ctx context.Context, arg sqlcgen.ClaimAuthEmailSendParams) (sqlcgen.ClaimAuthEmailSendRow, error)
	GetAuthEmailSendWindow(ctx context.Context, email string) (sqlcgen.GetAuthEmailSendWindowRow, error)

	// The typed code rows.
	IssueAuthCode(ctx context.Context, arg sqlcgen.IssueAuthCodeParams) (int64, error)
	BumpAuthCodeAttempt(ctx context.Context, arg sqlcgen.BumpAuthCodeAttemptParams) (sqlcgen.BumpAuthCodeAttemptRow, error)
	ConsumeAuthCode(ctx context.Context, id int64) (int64, error)
	GetLatestAuthCode(ctx context.Context, arg sqlcgen.GetLatestAuthCodeParams) (sqlcgen.GetLatestAuthCodeRow, error)
	DeleteAuthCodesForEmail(ctx context.Context, email string) error
	PruneAuthCodes(ctx context.Context, before time.Time) (int64, error)

	// The database sign in throttle.
	GetAuthThrottle(ctx context.Context, arg sqlcgen.GetAuthThrottleParams) (sqlcgen.GetAuthThrottleRow, error)
	RecordAuthFailure(ctx context.Context, arg sqlcgen.RecordAuthFailureParams) (sqlcgen.RecordAuthFailureRow, error)
	ClearAuthThrottle(ctx context.Context, arg sqlcgen.ClearAuthThrottleParams) error
	PruneAuthThrottles(ctx context.Context, cutoff time.Time) (int64, error)

	// Accounts and sessions.
	GetAccountForSignIn(ctx context.Context, email string) (sqlcgen.GetAccountForSignInRow, error)
	InsertAccount(ctx context.Context, email string) error
	SetAccountPassword(ctx context.Context, arg sqlcgen.SetAccountPasswordParams) (int64, error)
	BindSessionAccount(ctx context.Context, arg sqlcgen.BindSessionAccountParams) (int64, error)
	GetSessionBinding(ctx context.Context, sessionID string) (sqlcgen.GetSessionBindingRow, error)
	GetAccountBySessionID(ctx context.Context, sessionID string) (sqlcgen.GetAccountBySessionIDRow, error)
	SessionEndedByPasswordSwitch(ctx context.Context, sessionID string) (bool, error)
	RevokeSession(ctx context.Context, arg sqlcgen.RevokeSessionParams) (int64, error)
	DeletePushSubscriptionsForSession(ctx context.Context, sessionID string) (int64, error)
}

var _ CredentialQuerier = (*sqlcgen.Queries)(nil)

// PasswordHasher is the part of *auth.Hasher the service uses. Tests wrap the
// real hasher to record what it was asked to hash and verify.
type PasswordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, encoded, password string) (ok bool, needsRehash bool, err error)
	DummyHash() string
}

var _ PasswordHasher = (*auth.Hasher)(nil)

// The two purposes of a code row and the three things a row can stand for.
// The strings are the values the auth_codes CHECK constraints allow.
const (
	PurposeSignUp = "signup"
	PurposeReset  = "reset"

	// DeliveryCode: a 6 digit code was emailed and can be confirmed.
	DeliveryCode = "code"
	// DeliveryAccountExists: Create account for an address that has a
	// password; the email holds no code, so the row can never be confirmed.
	DeliveryAccountExists = "account_exists"
	// DeliveryNoAccount: Forgot password for an unknown address; same.
	DeliveryNoAccount = "no_account"
)

// throttleScope is the only scope the auth_throttles CHECK allows.
const throttleScope = "sign_in"

// throttleLabel is mixed into the throttle key hash. It is a different value
// from auth.ThrottleKeyLabel, which only derives the key itself.
const throttleLabel = "pinalert/sign-in-throttle/v1"

// The sentences a person can read (UI-SPEC copywriting rows P71-C13, C14, C16
// and C17). Plain ASCII punctuation only; a test pins that.
const (
	WrongCredentialsMessage = "Email or password is wrong."
	WrongCodeMessage        = "That code is wrong or has expired."
	TooManyCodesMessage     = "Too many codes for this email. Try again in an hour."
	InvalidEmailMessage     = "Enter a valid email address."
	PasswordTooShortMessage = "Use at least 8 characters."
	PasswordTooLongMessage  = "Use 128 characters or fewer."
	PasswordCommonMessage   = "That password is too common. Choose another."
	PasswordIsEmailMessage  = "Don't use your email address as your password."
)

// Errors a handler maps to a status. None carries an address, a code or a
// password (precedent WR-S03).
var (
	// ErrWrongCredentials is the one answer to a wrong password, an unknown
	// address and an account with no password.
	ErrWrongCredentials = errors.New("service: wrong credentials")
	// ErrWrongCode covers a wrong, expired, used or spent code.
	ErrWrongCode = errors.New("service: wrong or expired code")
	// ErrTooManyCodes means the hourly count of emails for the address is spent.
	ErrTooManyCodes = errors.New("service: too many codes for this address")
	// ErrMailUnavailable means the daily send ceiling stopped the email.
	ErrMailUnavailable = errors.New("service: mail unavailable")
	// ErrBusy means no password hash slot came free in time (W6); a handler
	// answers 503.
	ErrBusy = errors.New("service: password hasher busy")
)

// SignInLockedError says a sign in was refused before any hashing because the
// address is locked. Wait is how long is left.
type SignInLockedError struct {
	Wait time.Duration
}

func (e SignInLockedError) Error() string { return SignInLockedMessage(e.Wait) }

// SignInLockedMessage is "Too many tries. Wait N minutes, or reset your
// password." with N rounded up, and "Wait 1 minute" for a minute or less.
func SignInLockedMessage(wait time.Duration) string {
	minutes := int((wait + time.Minute - 1) / time.Minute)
	if minutes <= 1 {
		return "Too many tries. Wait 1 minute, or reset your password."
	}
	return fmt.Sprintf("Too many tries. Wait %d minutes, or reset your password.", minutes)
}

// CredentialLimits are the database backed limits this service owns. The
// device and network buckets are the handler's job (plan 07).
type CredentialLimits struct {
	// CodeTTL is how long a code can be typed.
	CodeTTL time.Duration
	// CodeMaxAttempts is the tries one code allows, counted before the compare.
	CodeMaxAttempts int
	// SendCooldown is the wait between two emails to one address.
	SendCooldown time.Duration
	// SendsPerHour caps the emails to one address inside SendWindow.
	SendsPerHour int
	SendWindow   time.Duration
	// FreeFailures wrong passwords per address before the first lock.
	FreeFailures int
	// BaseLock is the first lock; it doubles for each failure beyond, up to
	// MaxLock.
	BaseLock time.Duration
	MaxLock  time.Duration
	// ForgetAfter is how long a quiet address is forgotten. It counts from
	// the later of the last failure and the end of the last lock (B1).
	ForgetAfter time.Duration
	// ResumeWindow is how far back a browser's newest code row is still
	// shown on a reload and still resendable.
	ResumeWindow time.Duration
}

// DefaultCredentialLimits are the numbers shown to the owner (D-11).
var DefaultCredentialLimits = CredentialLimits{
	CodeTTL:         10 * time.Minute,
	CodeMaxAttempts: 5,
	SendCooldown:    ResendCooldown,
	SendsPerHour:    5,
	SendWindow:      time.Hour,
	FreeFailures:    5,
	BaseLock:        5 * time.Minute,
	MaxLock:         60 * time.Minute,
	ForgetAfter:     30 * time.Minute,
	ResumeWindow:    time.Hour,
}

// codeRowRetention is how long an expired code row is kept before the prune
// deletes it.
const codeRowRetention = 24 * time.Hour

// CredentialService is the server side of Create account, the typed code,
// Sign in, Resend and Forgot password. AuthService is left untouched: it also
// owns the Activity reads and the link flow, which keeps working until the
// cleanup plans.
type CredentialService struct {
	q           CredentialQuerier
	mailer      mailer.Mailer
	sessions    *SessionService
	hasher      PasswordHasher
	codeKey     []byte
	throttleKey []byte
	limits      CredentialLimits
	now         func() time.Time
}

// CredentialOption configures a CredentialService at construction time.
type CredentialOption func(*CredentialService)

// WithCredentialClock overrides the clock. Tests use it; production never
// needs to.
func WithCredentialClock(fn func() time.Time) CredentialOption {
	return func(s *CredentialService) { s.now = fn }
}

// WithHasher replaces the password hasher (tests pass a fast one).
func WithHasher(h PasswordHasher) CredentialOption {
	return func(s *CredentialService) { s.hasher = h }
}

// WithCodeKey sets the key the code hashes are made under.
func WithCodeKey(key []byte) CredentialOption {
	return func(s *CredentialService) { s.codeKey = key }
}

// WithThrottleKey sets the key the throttle address hashes are made under.
func WithThrottleKey(key []byte) CredentialOption {
	return func(s *CredentialService) { s.throttleKey = key }
}

// WithCredentialLimits replaces the limits.
func WithCredentialLimits(l CredentialLimits) CredentialOption {
	return func(s *CredentialService) { s.limits = l }
}

// minKeyBytes is the shortest key accepted from an option.
const minKeyBytes = 32

// NewCredentialService builds a CredentialService. The clock defaults to
// time.Now, the hasher to auth.NewHasher(auth.DefaultParams) and the limits to
// DefaultCredentialLimits. When a key option is missing or shorter than 32
// bytes, a random 32 byte key is drawn once from crypto/rand, so a Deps
// literal in a test still works; cmd/server/main.go always passes keys derived
// from SESSION_SECRET (plan 07), so codes and throttle rows survive a restart.
func NewCredentialService(q CredentialQuerier, m mailer.Mailer, opts ...CredentialOption) *CredentialService {
	s := &CredentialService{
		q:      q,
		mailer: m,
		limits: DefaultCredentialLimits,
		now:    time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.hasher == nil {
		s.hasher = auth.NewHasher(auth.DefaultParams)
	}
	if len(s.codeKey) < minKeyBytes {
		s.codeKey = randomKey()
	}
	if len(s.throttleKey) < minKeyBytes {
		s.throttleKey = randomKey()
	}
	s.sessions = NewSessionService(q, WithSessionClock(func() time.Time { return s.now() }))
	return s
}

func randomKey() []byte {
	k := make([]byte, minKeyBytes)
	if _, err := rand.Read(k); err != nil {
		// crypto/rand failing means the machine has no entropy source; nothing
		// safe can run.
		panic("service: crypto/rand failed: " + err.Error())
	}
	return k
}

// CodeTTL is how long a code can be typed, for the page text.
func (s *CredentialService) CodeTTL() time.Duration { return s.limits.CodeTTL }

// ResendCooldown is the wait between two emails to one address.
func (s *CredentialService) ResendCooldown() time.Duration { return s.limits.SendCooldown }

// SignUp is Create account. It takes an email and a password, hashes the
// password, writes one session bound code row and sends exactly one email, for
// every address (D-01, D-02):
//
//   - an unknown address, and an account made with the old link that has no
//     password: a row with delivery code and the password hash, and the code;
//   - an address with a password: a row with delivery account_exists, no
//     password hash and a code that is never emailed, and the "you already have
//     an account" email.
//
// Every branch returns nil, does the same claim and the same hash, and leaves
// one row, so the answer, the work, a later Resend and a reload of the code
// screen look the same (research Pitfall 7). The address lives in the row,
// never in a URL (D-05). The claim commits before the row and the email, so a
// failed send still consumes it, as the link flow's cooldown does.
func (s *CredentialService) SignUp(ctx context.Context, sessionID, rawEmail, password string) error {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return ValidationError{Field: "email", Message: InvalidEmailMessage}
	}
	if err := checkPassword(password, email); err != nil {
		return err
	}
	if err := s.claimSend(ctx, email); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(ctx, auth.NormalizePassword(password))
	if err != nil {
		return s.hashError(err)
	}
	account, found, err := s.lookupAccount(ctx, email)
	if err != nil {
		return err
	}
	delivery := DeliveryCode
	var stored *string = &hash
	if found && account.PasswordHash != nil {
		delivery = DeliveryAccountExists
		stored = nil
	}
	return s.issueAndSend(ctx, sessionID, PurposeSignUp, delivery, email, stored)
}

// RequestReset is Forgot password. A known address (with or without a
// password) gets a reset code; an unknown address gets the "there is no
// account" email. Both write one row, send one email and return nil. No
// password hash is stored.
func (s *CredentialService) RequestReset(ctx context.Context, sessionID, rawEmail string) error {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return ValidationError{Field: "email", Message: InvalidEmailMessage}
	}
	if err := s.claimSend(ctx, email); err != nil {
		return err
	}
	_, found, err := s.lookupAccount(ctx, email)
	if err != nil {
		return err
	}
	delivery := DeliveryNoAccount
	if found {
		delivery = DeliveryCode
	}
	return s.issueAndSend(ctx, sessionID, PurposeReset, delivery, email, nil)
}

// ResendCode reissues the newest code row of this browser with the same
// purpose, recomputes which email it stands for, retires the old row (inside
// IssueAuthCode) and obeys the same per address claim, so resending never
// multiplies the try budget. A browser with no row inside the resume window,
// or whose row was already used, gets ErrWrongCode.
func (s *CredentialService) ResendCode(ctx context.Context, sessionID string) error {
	now := s.now()
	row, err := s.q.GetLatestAuthCode(ctx, sqlcgen.GetLatestAuthCodeParams{
		SessionID: sessionID,
		Since:     now.Add(-s.limits.ResumeWindow),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrWrongCode
		}
		return fmt.Errorf("service: reading latest code: %w", err)
	}
	if row.UsedAt.Valid {
		return ErrWrongCode
	}
	account, found, err := s.lookupAccount(ctx, row.Email)
	if err != nil {
		return err
	}
	var delivery string
	var stored *string
	switch row.Purpose {
	case PurposeSignUp:
		if found && account.PasswordHash != nil {
			delivery = DeliveryAccountExists
		} else {
			delivery = DeliveryCode
			// A code to be confirmed needs the password typed at Create
			// account; a row that does not carry it cannot be reissued.
			if row.PasswordHash == nil {
				return ErrWrongCode
			}
			stored = row.PasswordHash
		}
	case PurposeReset:
		delivery = DeliveryNoAccount
		if found {
			delivery = DeliveryCode
		}
	default:
		return ErrWrongCode
	}
	if err := s.claimSend(ctx, row.Email); err != nil {
		return err
	}
	return s.issueAndSend(ctx, sessionID, row.Purpose, delivery, row.Email, stored)
}

// checkPassword maps auth.CheckPassword to the field "password" sentences.
func checkPassword(password, email string) error {
	err := auth.CheckPassword(password, email)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, auth.ErrPasswordTooShort):
		return ValidationError{Field: "password", Message: PasswordTooShortMessage}
	case errors.Is(err, auth.ErrPasswordTooLong):
		return ValidationError{Field: "password", Message: PasswordTooLongMessage}
	case errors.Is(err, auth.ErrPasswordCommon):
		return ValidationError{Field: "password", Message: PasswordCommonMessage}
	case errors.Is(err, auth.ErrPasswordIsEmail):
		return ValidationError{Field: "password", Message: PasswordIsEmailMessage}
	}
	return fmt.Errorf("service: checking password: %w", err)
}

// claimSend takes the per address send claim. A refused claim is
// ErrTooManyCodes when the hourly count is spent and ErrRateLimited otherwise.
func (s *CredentialService) claimSend(ctx context.Context, email string) error {
	now := s.now()
	_, err := s.q.ClaimAuthEmailSend(ctx, sqlcgen.ClaimAuthEmailSendParams{
		Email:          email,
		Now:            now,
		WindowCutoff:   now.Add(-s.limits.SendWindow),
		CooldownCutoff: now.Add(-s.limits.SendCooldown),
		MaxSends:       int32(s.limits.SendsPerHour),
	})
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		// No address in the message: the handler logs this error (WR-S03).
		return fmt.Errorf("service: claiming email send: %w", err)
	}
	win, werr := s.q.GetAuthEmailSendWindow(ctx, email)
	if werr != nil {
		if errors.Is(werr, pgx.ErrNoRows) {
			return ErrRateLimited
		}
		return fmt.Errorf("service: reading email send window: %w", werr)
	}
	if win.WindowStartedAt.Valid &&
		win.WindowStartedAt.Time.After(now.Add(-s.limits.SendWindow)) &&
		int(win.WindowSends) >= s.limits.SendsPerHour {
		return ErrTooManyCodes
	}
	return ErrRateLimited
}

// lookupAccount reads the account of a normalised address. A deleted account
// yields no row.
func (s *CredentialService) lookupAccount(ctx context.Context, email string) (sqlcgen.GetAccountForSignInRow, bool, error) {
	row, err := s.q.GetAccountForSignIn(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.GetAccountForSignInRow{}, false, nil
		}
		return sqlcgen.GetAccountForSignInRow{}, false, fmt.Errorf("service: reading account: %w", err)
	}
	return row, true, nil
}

// hashError maps a hasher error: a full queue is ErrBusy (W6).
func (s *CredentialService) hashError(err error) error {
	if errors.Is(err, auth.ErrHasherBusy) {
		return ErrBusy
	}
	return fmt.Errorf("service: hashing password: %w", err)
}

// issueAndSend writes the one code row of the request, prunes stale rows (W7)
// and sends the one email the row stands for. Persist before send.
func (s *CredentialService) issueAndSend(ctx context.Context, sessionID, purpose, delivery, email string, passwordHash *string) error {
	// Every branch draws and hashes a code so the work is the same; only a
	// delivery of code ever emails it.
	code, err := auth.NewCode()
	if err != nil {
		return fmt.Errorf("service: drawing code: %w", err)
	}
	now := s.now()
	if _, err := s.q.IssueAuthCode(ctx, sqlcgen.IssueAuthCodeParams{
		SessionID:    sessionID,
		Purpose:      purpose,
		Delivery:     delivery,
		Email:        email,
		CodeHash:     auth.CodeHash(s.codeKey, purpose, email, code),
		PasswordHash: passwordHash,
		ExpiresAt:    now.Add(s.limits.CodeTTL),
	}); err != nil {
		return fmt.Errorf("service: issuing code: %w", err)
	}
	// Housekeeping: a failure here never fails a request.
	_, _ = s.q.PruneAuthCodes(ctx, now.Add(-codeRowRetention))
	_, _ = s.q.PruneAuthThrottles(ctx, now.Add(-s.limits.ForgetAfter))

	msg := mailer.AuthEmail{}
	switch {
	case delivery == DeliveryAccountExists:
		msg.Kind = mailer.KindAccountExists
	case delivery == DeliveryNoAccount:
		msg.Kind = mailer.KindNoAccount
	case purpose == PurposeReset:
		msg.Kind = mailer.KindCodeReset
		msg.Code = code
		msg.MinutesValid = minutesValid(s.limits.CodeTTL)
	default:
		msg.Kind = mailer.KindCodeSignup
		msg.Code = code
		msg.MinutesValid = minutesValid(s.limits.CodeTTL)
	}
	if err := s.mailer.SendAuthEmail(ctx, email, msg); err != nil {
		if errors.Is(err, mailer.ErrDailyCeiling) {
			return ErrMailUnavailable
		}
		return fmt.Errorf("service: sending auth email: %w", err)
	}
	return nil
}

// minutesValid is the lifetime in whole minutes, at least 1, so the words of
// the email follow the lifetime the service enforces.
func minutesValid(ttl time.Duration) int {
	m := int(ttl / time.Minute)
	if m < 1 {
		return 1
	}
	return m
}
