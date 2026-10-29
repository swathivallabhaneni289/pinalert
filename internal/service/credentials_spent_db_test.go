package service_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"pinalert/internal/auth"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestSpentCodeOnDatabase is the service half of the Phase 7.1 review fix round
// 3 (skeptic SK2-01). After a confirm whose answer never reached the browser,
// the browser still carries the session id it sent the code with. SpentCode
// tells that browser, from its own newest code row only, that its code was
// confirmed, so the sign in page can say so. A code that was refused, never
// sent, or confirmed by nobody is not spent, and the confirmed row keeps no
// password hash. It skips when DATABASE_URL is unset.
func TestSpentCodeOnDatabase(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	mail := &credMailer{}
	var offset atomic.Int64
	clock := func() time.Time { return time.Now().Add(time.Duration(offset.Load())) }

	limits := service.DefaultCredentialLimits
	limits.SendCooldown = 0
	svc := service.NewCredentialService(q, mail,
		service.WithHasher(auth.NewHasher(auth.TestParams())),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
		service.WithCredentialLimits(limits),
		service.WithCredentialClock(clock),
	)

	// Nothing asked yet: nothing spent.
	if _, ok, err := svc.SpentCode(ctx, "spent-A"); err != nil || ok {
		t.Fatalf("SpentCode before any code = %v, %v; want false", ok, err)
	}

	// Create account on browser A. A live code is not spent, and a wrong guess
	// does not spend it.
	if err := svc.SignUp(ctx, "spent-A", credEmail, credPassword); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	code := mail.last(t).msg.Code
	// A second browser asks for a code for the same address; the confirm on A
	// must still delete this pending row (W9).
	if err := svc.SignUp(ctx, "spent-Z", credEmail, credPassword); err != nil {
		t.Fatalf("SignUp on Z: %v", err)
	}
	if _, err := svc.ConfirmSignUp(ctx, "spent-A", credWrongCode(code), credSessionIDs("x")); !errors.Is(err, service.ErrWrongCode) {
		t.Fatalf("wrong code: %v", err)
	}
	if _, ok, err := svc.SpentCode(ctx, "spent-A"); err != nil || ok {
		t.Fatalf("SpentCode with a live code = %v, %v; want false", ok, err)
	}

	res, err := svc.ConfirmSignUp(ctx, "spent-A", code, credSessionIDs("spent-B"))
	if err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}
	spent, ok, err := svc.SpentCode(ctx, "spent-A")
	if err != nil || !ok || spent.Purpose != service.PurposeSignUp || spent.Email != credEmail {
		t.Fatalf("SpentCode after the confirm = %+v, %v, %v; want the sign up of %s", spent, ok, err, credEmail)
	}
	if _, ok, _ := svc.LiveCode(ctx, "spent-A"); ok {
		t.Errorf("the spent code is still shown as live")
	}
	// The fresh session holds no code row, and a stranger sees nothing.
	for _, id := range []string{res.SessionID, "spent-stranger"} {
		if _, ok, err := svc.SpentCode(ctx, id); err != nil || ok {
			t.Errorf("SpentCode(%s) = %v, %v; want false", id, ok, err)
		}
	}
	var rows, withHash, pendingZ int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(password_hash) FROM auth_codes WHERE email = $1`, credEmail).Scan(&rows, &withHash); err != nil {
		t.Fatalf("counting code rows: %v", err)
	}
	if rows != 1 || withHash != 0 {
		t.Errorf("code rows of the address after the confirm = %d (%d with a password hash), want only the spent row and no hash", rows, withHash)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_codes WHERE session_id = 'spent-Z'`).Scan(&pendingZ); err != nil || pendingZ != 0 {
		t.Errorf("the other browser's pending rows = %d (err %v), want 0 (W9)", pendingZ, err)
	}
	// The same code a second time is still refused.
	if _, err := svc.ConfirmSignUp(ctx, "spent-A", code, credSessionIDs("y")); !errors.Is(err, service.ErrWrongCode) {
		t.Errorf("the spent code a second time: %v, want ErrWrongCode", err)
	}
	// Resend on a spent row is refused.
	if err := svc.ResendCode(ctx, "spent-A"); !errors.Is(err, service.ErrWrongCode) {
		t.Errorf("Resend on a spent row: %v, want ErrWrongCode", err)
	}

	// Forgot password on browser D.
	if err := svc.RequestReset(ctx, "spent-D", credEmail); err != nil {
		t.Fatalf("RequestReset: %v", err)
	}
	if err := svc.ConfirmReset(ctx, "spent-D", mail.last(t).msg.Code, "a brand new password 55"); err != nil {
		t.Fatalf("ConfirmReset: %v", err)
	}
	spent, ok, err = svc.SpentCode(ctx, "spent-D")
	if err != nil || !ok || spent.Purpose != service.PurposeReset || spent.Email != credEmail {
		t.Fatalf("SpentCode after the reset = %+v, %v, %v; want the reset of %s", spent, ok, err, credEmail)
	}

	// A code whose email did not go out is retired, never spent.
	mail.err = errors.New("provider down")
	if err := svc.SignUp(ctx, "spent-F", "fails@example.com", credPassword); err == nil {
		t.Fatalf("SignUp with a failing mailer returned nil")
	}
	if err := svc.RequestReset(ctx, "spent-G", credEmail); err == nil {
		t.Fatalf("RequestReset with a failing mailer returned nil")
	}
	mail.err = nil
	for _, id := range []string{"spent-F", "spent-G"} {
		if _, ok, err := svc.SpentCode(ctx, id); err != nil || ok {
			t.Errorf("SpentCode(%s) for an email that never went out = %v, %v; want false", id, ok, err)
		}
		if _, ok, _ := svc.LiveCode(ctx, id); ok {
			t.Errorf("LiveCode(%s) for an email that never went out is live", id)
		}
	}

	// Round 4 (skeptic SK3-02): a spent code is told only for a short time after
	// the spend, not for the whole resume window, so the next person on a shared
	// phone does not see the address and the line. A few minutes later it still
	// shows (the lost answer case); eleven minutes later it is gone.
	offset.Store(int64(5 * time.Minute))
	if _, ok, err := svc.SpentCode(ctx, "spent-D"); err != nil || !ok {
		t.Errorf("SpentCode five minutes after the spend = %v, %v; want true", ok, err)
	}
	offset.Store(int64(11 * time.Minute))
	if _, ok, err := svc.SpentCode(ctx, "spent-D"); err != nil || ok {
		t.Errorf("SpentCode eleven minutes after the spend = %v, %v; want false (inside the resume window)", ok, err)
	}

	// Outside the resume window the spent row says nothing.
	offset.Store(int64(limits.ResumeWindow + time.Minute))
	if _, ok, err := svc.SpentCode(ctx, "spent-D"); err != nil || ok {
		t.Errorf("SpentCode after the resume window = %v, %v; want false", ok, err)
	}
}
