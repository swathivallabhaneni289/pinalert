package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// fakeAuthQuerier and fakeMailer both append to a shared call log so tests
// can assert the insert-before-send ordering directly rather than infer it.
type fakeAuthQuerier struct {
	calls   *[]string
	lastArg sqlcgen.InsertMagicLinkTokenParams
	err     error
}

func (f *fakeAuthQuerier) InsertMagicLinkToken(ctx context.Context, arg sqlcgen.InsertMagicLinkTokenParams) (sqlcgen.InsertMagicLinkTokenRow, error) {
	*f.calls = append(*f.calls, "insert:"+arg.Email)
	f.lastArg = arg
	if f.err != nil {
		return sqlcgen.InsertMagicLinkTokenRow{}, f.err
	}
	return sqlcgen.InsertMagicLinkTokenRow{ID: 1, CreatedAt: time.Now(), ExpiresAt: arg.ExpiresAt}, nil
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

func newFakes() (*fakeAuthQuerier, *fakeMailer, *[]string) {
	calls := &[]string{}
	return &fakeAuthQuerier{calls: calls}, &fakeMailer{calls: calls}, calls
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

	err := svc.RequestLink(context.Background(), "not-an-email")

	assertValidationError(t, err, "email")
	if err.(service.ValidationError).Message != "Enter a valid email address." {
		t.Fatalf("unexpected message: %v", err)
	}
}

func TestRequestLinkRejectsDisplayNameForm(t *testing.T) {
	q, m, _ := newFakes()
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	err := svc.RequestLink(context.Background(), "Someone <a@b.com>")

	assertValidationError(t, err, "email")
}

func TestRequestLinkNormalizesEmail(t *testing.T) {
	q, m, _ := newFakes()
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	if err := svc.RequestLink(context.Background(), "  A@Example.COM "); err != nil {
		t.Fatalf("RequestLink: %v", err)
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

	if err := svc.RequestLink(context.Background(), "visitor@example.com"); err != nil {
		t.Fatalf("RequestLink: %v", err)
	}

	if len(*calls) != 2 || (*calls)[0] != "insert:visitor@example.com" || (*calls)[1] != "send:visitor@example.com" {
		t.Fatalf("unexpected call order: %v", *calls)
	}
}

func TestRequestLinkRecordsInsertEvenWhenMailerFails(t *testing.T) {
	q, m, calls := newFakes()
	m.err = errors.New("provider unavailable")
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	err := svc.RequestLink(context.Background(), "visitor@example.com")

	if err == nil {
		t.Fatal("expected an error when the mailer fails, got nil")
	}
	var ve service.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("expected a non-validation error, got ValidationError: %v", err)
	}
	if len(*calls) != 2 || (*calls)[0] != "insert:visitor@example.com" {
		t.Fatalf("expected the insert to be recorded even though the mailer failed: %v", *calls)
	}
}

func TestRequestLinkLinkShapeAndHash(t *testing.T) {
	q, m, _ := newFakes()
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	if err := svc.RequestLink(context.Background(), "visitor@example.com"); err != nil {
		t.Fatalf("RequestLink: %v", err)
	}

	if !strings.HasPrefix(m.lastLink, "https://pinalert.example/auth/verify?token=") {
		t.Fatalf("link %q does not have the expected prefix", m.lastLink)
	}
	if m.lastLink == q.lastArg.TokenHash {
		t.Fatalf("mailed link must never equal the stored hash")
	}
}

func TestRequestLinkUsesInjectedClockForExpiry(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q, m, _ := newFakes()
	svc := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(func() time.Time { return fixed }))

	if err := svc.RequestLink(context.Background(), "visitor@example.com"); err != nil {
		t.Fatalf("RequestLink: %v", err)
	}

	wantExpiry := fixed.Add(5 * time.Minute)
	if !q.lastArg.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("ExpiresAt = %v, want %v", q.lastArg.ExpiresAt, wantExpiry)
	}
}
