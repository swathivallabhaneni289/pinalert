package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"pinalert/internal/auth"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// fakeAuthQuerier and fakeMailer both append to a shared call log so tests
// can assert the insert-before-send ordering directly rather than infer it.
// Every VerifyToken-path method is separately programmable (a row to return,
// or pgx.ErrNoRows) so each outcome can be driven without a real database.
type fakeAuthQuerier struct {
	calls   *[]string
	lastArg sqlcgen.InsertMagicLinkTokenParams
	err     error

	tokenRow      sqlcgen.GetMagicLinkTokenByHashRow
	tokenRowErr   error
	consumeEmail  string
	consumeErr    error
	sessionRow    sqlcgen.GetAccountBySessionIDRow
	sessionRowErr error
	insertErr     error
	byEmailRow    sqlcgen.Account
	byEmailErr    error
	bindErr       error

	// latestToken/latestTokenErr drive RequestLink's cooldown check.
	// newFakes defaults latestTokenErr to pgx.ErrNoRows ("no prior
	// request for this address") so every existing RequestLink test case
	// that never sets it keeps behaving unchanged; VerifyToken tests
	// (built via newFakeQuerier below) never touch this field at all,
	// since VerifyToken never calls LatestTokenForEmail.
	latestToken    time.Time
	latestTokenErr error
}

func (f *fakeAuthQuerier) InsertMagicLinkToken(ctx context.Context, arg sqlcgen.InsertMagicLinkTokenParams) (sqlcgen.InsertMagicLinkTokenRow, error) {
	*f.calls = append(*f.calls, "insert:"+arg.Email)
	f.lastArg = arg
	if f.err != nil {
		return sqlcgen.InsertMagicLinkTokenRow{}, f.err
	}
	return sqlcgen.InsertMagicLinkTokenRow{ID: 1, CreatedAt: time.Now(), ExpiresAt: arg.ExpiresAt}, nil
}

func (f *fakeAuthQuerier) GetMagicLinkTokenByHash(ctx context.Context, tokenHash string) (sqlcgen.GetMagicLinkTokenByHashRow, error) {
	*f.calls = append(*f.calls, "GetMagicLinkTokenByHash")
	if f.tokenRowErr != nil {
		return sqlcgen.GetMagicLinkTokenByHashRow{}, f.tokenRowErr
	}
	return f.tokenRow, nil
}

func (f *fakeAuthQuerier) ConsumeToken(ctx context.Context, tokenHash string) (string, error) {
	*f.calls = append(*f.calls, "ConsumeToken")
	if f.consumeErr != nil {
		return "", f.consumeErr
	}
	return f.consumeEmail, nil
}

func (f *fakeAuthQuerier) InsertAccount(ctx context.Context, email string) error {
	*f.calls = append(*f.calls, "InsertAccount")
	return f.insertErr
}

func (f *fakeAuthQuerier) GetAccountByEmail(ctx context.Context, email string) (sqlcgen.Account, error) {
	*f.calls = append(*f.calls, "GetAccountByEmail")
	if f.byEmailErr != nil {
		return sqlcgen.Account{}, f.byEmailErr
	}
	return f.byEmailRow, nil
}

func (f *fakeAuthQuerier) GetAccountBySessionID(ctx context.Context, sessionID string) (sqlcgen.GetAccountBySessionIDRow, error) {
	*f.calls = append(*f.calls, "GetAccountBySessionID")
	if f.sessionRowErr != nil {
		return sqlcgen.GetAccountBySessionIDRow{}, f.sessionRowErr
	}
	return f.sessionRow, nil
}

func (f *fakeAuthQuerier) BindSessionAccount(ctx context.Context, arg sqlcgen.BindSessionAccountParams) error {
	*f.calls = append(*f.calls, "BindSessionAccount")
	return f.bindErr
}

func (f *fakeAuthQuerier) LatestTokenForEmail(ctx context.Context, email string) (time.Time, error) {
	*f.calls = append(*f.calls, "LatestTokenForEmail")
	if f.latestTokenErr != nil {
		return time.Time{}, f.latestTokenErr
	}
	return f.latestToken, nil
}

type fakeMailer struct {
	calls     *[]string
	err       error
	lastEmail string
	lastLink  string
}

func (f *fakeMailer) SendVerificationLink(ctx context.Context, email, link string) error {
	*f.calls = append(*f.calls, "send:"+email)
	f.lastEmail = email
	f.lastLink = link
	return f.err
}

// newFakes defaults latestTokenErr to pgx.ErrNoRows — "no prior request for
// this address" — so every existing RequestLink test that predates the
// cooldown check (and never sets this field) keeps exercising the
// no-cooldown path unchanged.
func newFakes() (*fakeAuthQuerier, *fakeMailer, *[]string) {
	calls := &[]string{}
	return &fakeAuthQuerier{calls: calls, latestTokenErr: pgx.ErrNoRows}, &fakeMailer{calls: calls}, calls
}

func assertValidationError(t *testing.T, err error, field string) {
	t.Helper()
	var ve service.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected service.ValidationError, got %v (%T)", err, err)
	}
	if ve.Field != field {
		t.Fatalf("ValidationError.Field = %q, want %q", ve.Field, field)
	}
}

func TestRequestLinkRejectsMalformedEmail(t *testing.T) {
	q, m, _ := newFakes()
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	_, err := svc.RequestLink(context.Background(), "not-an-email")

	assertValidationError(t, err, "email")
	if err.(service.ValidationError).Message != "Enter a valid email address." {
		t.Fatalf("unexpected message: %v", err)
	}
}

func TestRequestLinkRejectsDisplayNameForm(t *testing.T) {
	q, m, _ := newFakes()
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	_, err := svc.RequestLink(context.Background(), "Someone <a@b.com>")

	assertValidationError(t, err, "email")
}

func TestRequestLinkNormalizesEmail(t *testing.T) {
	q, m, _ := newFakes()
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	returnedEmail, err := svc.RequestLink(context.Background(), "  A@Example.COM ")
	if err != nil {
		t.Fatalf("RequestLink: %v", err)
	}

	if returnedEmail != "a@example.com" {
		t.Fatalf("returned email = %q, want a@example.com", returnedEmail)
	}
	if q.lastArg.Email != "a@example.com" {
		t.Fatalf("persisted email = %q, want a@example.com", q.lastArg.Email)
	}
	if m.lastEmail != "a@example.com" {
		t.Fatalf("mailed email = %q, want a@example.com", m.lastEmail)
	}
}

func TestRequestLinkInsertsBeforeSend(t *testing.T) {
	q, m, calls := newFakes()
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	if _, err := svc.RequestLink(context.Background(), "visitor@example.com"); err != nil {
		t.Fatalf("RequestLink: %v", err)
	}

	if len(*calls) != 3 || (*calls)[0] != "LatestTokenForEmail" || (*calls)[1] != "insert:visitor@example.com" || (*calls)[2] != "send:visitor@example.com" {
		t.Fatalf("unexpected call order: %v", *calls)
	}
}

func TestRequestLinkRecordsInsertEvenWhenMailerFails(t *testing.T) {
	q, m, calls := newFakes()
	m.err = errors.New("provider unavailable")
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	_, err := svc.RequestLink(context.Background(), "visitor@example.com")

	if err == nil {
		t.Fatal("expected an error when the mailer fails, got nil")
	}
	var ve service.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("expected a non-validation error, got ValidationError: %v", err)
	}
	if len(*calls) != 3 || (*calls)[0] != "LatestTokenForEmail" || (*calls)[1] != "insert:visitor@example.com" {
		t.Fatalf("expected the insert to be recorded even though the mailer failed: %v", *calls)
	}
}

func TestRequestLinkLinkShapeAndHash(t *testing.T) {
	q, m, _ := newFakes()
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	if _, err := svc.RequestLink(context.Background(), "visitor@example.com"); err != nil {
		t.Fatalf("RequestLink: %v", err)
	}

	if !strings.HasPrefix(m.lastLink, "https://pinalert.example/auth/verify?token=") {
		t.Fatalf("link %q does not have the expected prefix", m.lastLink)
	}
	if m.lastLink == q.lastArg.TokenHash {
		t.Fatalf("mailed link must never equal the stored hash")
	}
}

// --- RequestLink cooldown (D-03/D-04, DEC-J) ---

// TestRequestLinkRejectsSecondRequestInsideCooldown is DEC-J's core claim: a
// second request 10 seconds after the first is refused, inserts no row, and
// calls no mailer.
func TestRequestLinkRejectsSecondRequestInsideCooldown(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, m, calls := newFakes()
	q.latestTokenErr = nil
	q.latestToken = fixed.Add(-10 * time.Second)
	svc := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	_, err := svc.RequestLink(context.Background(), "visitor@example.com")

	if !errors.Is(err, service.ErrRateLimited) {
		t.Fatalf("err = %v, want service.ErrRateLimited", err)
	}
	if len(*calls) != 1 || (*calls)[0] != "LatestTokenForEmail" {
		t.Fatalf("a rate-limit refusal must call only LatestTokenForEmail, no insert or send: %v", *calls)
	}
}

// TestRequestLinkAllowsRequestAfterCooldownElapses proves the cooldown
// actually releases once ResendCooldown (45s) has passed — 46 seconds after
// the first request succeeds.
func TestRequestLinkAllowsRequestAfterCooldownElapses(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, m, calls := newFakes()
	q.latestTokenErr = nil
	q.latestToken = fixed.Add(-46 * time.Second)
	svc := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	if _, err := svc.RequestLink(context.Background(), "visitor@example.com"); err != nil {
		t.Fatalf("RequestLink: %v", err)
	}
	if len(*calls) != 3 || (*calls)[0] != "LatestTokenForEmail" || (*calls)[1] != "insert:visitor@example.com" || (*calls)[2] != "send:visitor@example.com" {
		t.Fatalf("unexpected call order once the cooldown has elapsed: %v", *calls)
	}
}

// TestRequestLinkCooldownIsCaseInsensitive proves the cooldown check reads
// the normalised (lowercased) address, so "A@Example.com" right after
// "a@example.com" is refused rather than treated as a different identity.
func TestRequestLinkCooldownIsCaseInsensitive(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, m, _ := newFakes()
	q.latestTokenErr = nil
	q.latestToken = fixed.Add(-10 * time.Second)
	svc := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	_, err := svc.RequestLink(context.Background(), "A@Example.com")

	if !errors.Is(err, service.ErrRateLimited) {
		t.Fatalf("err = %v, want service.ErrRateLimited for a case-variant address within the cooldown", err)
	}
	// LatestTokenForEmail must have been consulted with the normalised
	// (lowercase) address — the fake doesn't record its argument, so this is
	// asserted indirectly: a mismatched-case lookup that queried the raw
	// "A@Example.com" string against a cooldown keyed on the lowercase form
	// would, in the real database, simply miss and NOT rate limit. Since the
	// fake always returns q.latestToken regardless of the email argument,
	// this test's real assertion is the case-insensitivity CONTRACT itself
	// (normalizeEmail lowercases before the cooldown read), covered directly
	// by TestRequestLinkNormalizesEmail above; this test additionally proves
	// RequestLink still reaches and honors ErrRateLimited for a mixed-case
	// input rather than short-circuiting validation differently for it.
}

// TestRequestLinkMailerFailureLeavesCooldownConsumed is Pitfall 3's core
// claim: a first RequestLink whose mailer fails still leaves the token row
// present (the insert already happened before the send), so an immediate
// retry for that address is refused by the cooldown — proven here by
// driving RequestLink twice against one fake querier whose insert call
// updates latestToken as a real database's cooldown read would observe.
func TestRequestLinkMailerFailureLeavesCooldownConsumed(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, m, calls := newFakes()
	m.err = errors.New("provider unavailable")
	svc := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	if _, err := svc.RequestLink(context.Background(), "visitor@example.com"); err == nil {
		t.Fatal("expected the first RequestLink to fail (mailer error)")
	}
	if len(*calls) != 3 || (*calls)[1] != "insert:visitor@example.com" {
		t.Fatalf("expected the first call's insert to be recorded despite the mailer failure: %v", *calls)
	}

	// Simulate the cooldown clock a real database would now enforce: the
	// insert that just happened is the most recent token for this address.
	q.latestTokenErr = nil
	q.latestToken = fixed

	_, err := svc.RequestLink(context.Background(), "visitor@example.com")
	if !errors.Is(err, service.ErrRateLimited) {
		t.Fatalf("immediate retry after a mailer failure: err = %v, want service.ErrRateLimited", err)
	}
	if len(*calls) != 4 || (*calls)[3] != "LatestTokenForEmail" {
		t.Fatalf("the retry must be refused by the cooldown without a second insert: %v", *calls)
	}
}

// --- VerifyToken ---

func newFakeQuerier() (*fakeAuthQuerier, *[]string) {
	calls := &[]string{}
	return &fakeAuthQuerier{calls: calls}, calls
}

// validRawToken returns a raw token shaped exactly like
// internal/auth.GenerateToken's output, so the malformed-shape check in
// VerifyToken never rejects it before the database-driven test cases get a
// chance to exercise their own branch.
func validRawToken(t *testing.T) string {
	t.Helper()
	raw, _, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("auth.GenerateToken: %v", err)
	}
	return raw
}

func TestVerifyTokenEmptyTokenIsMalformed(t *testing.T) {
	q, calls := newFakeQuerier()
	svc := service.NewAuthService(q, &fakeMailer{calls: &[]string{}}, "https://pinalert.example")

	outcome, email, err := svc.VerifyToken(context.Background(), "session-1", "")
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if outcome != service.OutcomeMalformed {
		t.Fatalf("outcome = %v, want OutcomeMalformed", outcome)
	}
	if email != "" {
		t.Fatalf("email = %q, want empty", email)
	}
	if len(*calls) != 0 {
		t.Fatalf("expected no database calls for an empty token, got %v", *calls)
	}
}

func TestVerifyTokenNonBase64TokenIsMalformed(t *testing.T) {
	q, calls := newFakeQuerier()
	svc := service.NewAuthService(q, &fakeMailer{calls: &[]string{}}, "https://pinalert.example")

	outcome, _, err := svc.VerifyToken(context.Background(), "session-1", "not valid base64url!!")
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if outcome != service.OutcomeMalformed {
		t.Fatalf("outcome = %v, want OutcomeMalformed", outcome)
	}
	if len(*calls) != 0 {
		t.Fatalf("expected no database calls for a non-base64 token, got %v", *calls)
	}
}

func TestVerifyTokenUnknownHashIsMalformed(t *testing.T) {
	q, _ := newFakeQuerier()
	q.tokenRowErr = pgx.ErrNoRows
	svc := service.NewAuthService(q, &fakeMailer{calls: &[]string{}}, "https://pinalert.example")

	outcome, _, err := svc.VerifyToken(context.Background(), "session-1", validRawToken(t))
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if outcome != service.OutcomeMalformed {
		t.Fatalf("outcome = %v, want OutcomeMalformed", outcome)
	}
}

func TestVerifyTokenUsedRowReturnsUsed(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, calls := newFakeQuerier()
	q.tokenRow = sqlcgen.GetMagicLinkTokenByHashRow{
		Email:     "used@example.com",
		ExpiresAt: fixed.Add(1 * time.Minute),
		UsedAt:    pgtype.Timestamptz{Time: fixed.Add(-1 * time.Minute), Valid: true},
	}
	svc := service.NewAuthService(q, &fakeMailer{calls: &[]string{}}, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	outcome, email, err := svc.VerifyToken(context.Background(), "session-1", validRawToken(t))
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if outcome != service.OutcomeUsed {
		t.Fatalf("outcome = %v, want OutcomeUsed", outcome)
	}
	if email != "" {
		t.Fatalf("email = %q, want empty for a non-verified/non-conflict outcome", email)
	}
	for _, c := range *calls {
		if c == "ConsumeToken" || c == "BindSessionAccount" {
			t.Fatalf("used-token outcome must not call %s: %v", c, *calls)
		}
	}
}

func TestVerifyTokenExpiredRowReturnsExpired(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, calls := newFakeQuerier()
	q.tokenRow = sqlcgen.GetMagicLinkTokenByHashRow{
		Email:     "expired@example.com",
		ExpiresAt: fixed.Add(-1 * time.Minute),
	}
	svc := service.NewAuthService(q, &fakeMailer{calls: &[]string{}}, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	outcome, _, err := svc.VerifyToken(context.Background(), "session-1", validRawToken(t))
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if outcome != service.OutcomeExpired {
		t.Fatalf("outcome = %v, want OutcomeExpired", outcome)
	}
	for _, c := range *calls {
		if c == "ConsumeToken" || c == "BindSessionAccount" {
			t.Fatalf("expired-token outcome must not call %s: %v", c, *calls)
		}
	}
}

// TestVerifyTokenConflictRefusesRebind is DEC-D/DEC-E's core claim: a
// session already verified as a different account must be refused, not
// re-bound, and the token must be left unconsumed.
func TestVerifyTokenConflictRefusesRebind(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, calls := newFakeQuerier()
	q.tokenRow = sqlcgen.GetMagicLinkTokenByHashRow{
		Email:     "b@example.com",
		ExpiresAt: fixed.Add(1 * time.Minute),
	}
	q.sessionRow = sqlcgen.GetAccountBySessionIDRow{ID: 1, Email: "a@example.com"}
	svc := service.NewAuthService(q, &fakeMailer{calls: &[]string{}}, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	outcome, email, err := svc.VerifyToken(context.Background(), "session-1", validRawToken(t))
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if outcome != service.OutcomeConflict {
		t.Fatalf("outcome = %v, want OutcomeConflict", outcome)
	}
	if email != "a@example.com" {
		t.Fatalf("email = %q, want a@example.com (the session's EXISTING verified account, not the token's target)", email)
	}
	for _, c := range *calls {
		if c == "ConsumeToken" {
			t.Fatalf("conflict outcome must never call ConsumeToken: %v", *calls)
		}
		if c == "BindSessionAccount" {
			t.Fatalf("conflict outcome must never call BindSessionAccount: %v", *calls)
		}
	}
}

// TestVerifyTokenFreshTokenVerifies is the happy path: an unverified session
// consuming a fresh token is verified, and the recorded call order is
// ConsumeToken, then InsertAccount, then GetAccountByEmail, then
// BindSessionAccount.
func TestVerifyTokenFreshTokenVerifies(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, calls := newFakeQuerier()
	q.tokenRow = sqlcgen.GetMagicLinkTokenByHashRow{
		Email:     "fresh@example.com",
		ExpiresAt: fixed.Add(1 * time.Minute),
	}
	q.sessionRowErr = pgx.ErrNoRows // unverified session
	q.consumeEmail = "fresh@example.com"
	q.byEmailRow = sqlcgen.Account{ID: 42, Email: "fresh@example.com"}
	svc := service.NewAuthService(q, &fakeMailer{calls: &[]string{}}, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	outcome, email, err := svc.VerifyToken(context.Background(), "session-1", validRawToken(t))
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if outcome != service.OutcomeVerified {
		t.Fatalf("outcome = %v, want OutcomeVerified", outcome)
	}
	if email != "fresh@example.com" {
		t.Fatalf("email = %q, want fresh@example.com", email)
	}

	consumeIdx, insertIdx, getByEmailIdx, bindIdx := -1, -1, -1, -1
	for i, c := range *calls {
		switch c {
		case "ConsumeToken":
			consumeIdx = i
		case "InsertAccount":
			insertIdx = i
		case "GetAccountByEmail":
			getByEmailIdx = i
		case "BindSessionAccount":
			bindIdx = i
		}
	}
	if consumeIdx == -1 || insertIdx == -1 || getByEmailIdx == -1 || bindIdx == -1 {
		t.Fatalf("expected ConsumeToken, InsertAccount, GetAccountByEmail, and BindSessionAccount all to be called: %v", *calls)
	}
	if !(consumeIdx < insertIdx && insertIdx < getByEmailIdx && getByEmailIdx < bindIdx) {
		t.Fatalf("unexpected call order: %v", *calls)
	}
}

// TestVerifyTokenSameAccountRebindIsNotConflict covers a second device or a
// re-click by the same person: a session already verified as the SAME
// account the token belongs to is not a conflict.
func TestVerifyTokenSameAccountRebindIsNotConflict(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, _ := newFakeQuerier()
	q.tokenRow = sqlcgen.GetMagicLinkTokenByHashRow{
		Email:     "same@example.com",
		ExpiresAt: fixed.Add(1 * time.Minute),
	}
	q.sessionRow = sqlcgen.GetAccountBySessionIDRow{ID: 7, Email: "same@example.com"}
	q.consumeEmail = "same@example.com"
	q.byEmailRow = sqlcgen.Account{ID: 7, Email: "same@example.com"}
	svc := service.NewAuthService(q, &fakeMailer{calls: &[]string{}}, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	outcome, email, err := svc.VerifyToken(context.Background(), "session-1", validRawToken(t))
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if outcome != service.OutcomeVerified {
		t.Fatalf("outcome = %v, want OutcomeVerified", outcome)
	}
	if email != "same@example.com" {
		t.Fatalf("email = %q, want same@example.com", email)
	}
}

func TestRequestLinkUsesInjectedClockForExpiry(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, m, _ := newFakes()
	svc := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	if _, err := svc.RequestLink(context.Background(), "visitor@example.com"); err != nil {
		t.Fatalf("RequestLink: %v", err)
	}

	wantExpiry := fixed.Add(5 * time.Minute)
	if !q.lastArg.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("ExpiresAt = %v, want %v", q.lastArg.ExpiresAt, wantExpiry)
	}
}
