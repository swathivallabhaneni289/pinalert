package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"pinalert/internal/auth"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestResetStepsOnDatabase runs the two steps of Forgot password (quick
// 261113-r5w) through the real queries: the confirm window counted from the
// confirm and not from the send, the one winner of two parallel step two calls,
// the table's own refusal of a verified row that is not a reset code, and the
// spend writing the same equal used and expires pair as ConsumeAuthCode. It
// skips when DATABASE_URL is unset.
func TestResetStepsOnDatabase(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	mail := &credMailer{}

	limits := service.DefaultCredentialLimits
	limits.SendCooldown = 0
	limits.SendsPerHour = 100 // this test asks for many codes for one address
	svc := service.NewCredentialService(q, mail,
		service.WithHasher(auth.NewHasher(auth.TestParams())),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
		service.WithCredentialLimits(limits),
		service.WithCredentialTx(service.PgxCredentialTx(pool)),
	)
	// The account exists with a password, made through Create account.
	if err := svc.SignUp(ctx, "steps-up", credEmail, credPassword); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if _, err := svc.ConfirmSignUp(ctx, "steps-up", mail.last(t).msg.Code, credSessionIDs("steps-in")); err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}
	const newPassword = "a brand new password 55"

	// askAndConfirm asks for a reset code and types it on the browser; it returns
	// the session id the browser is moved to, "<session>-f", which holds the
	// confirm (review fix WR-02).
	askAndConfirm := func(t *testing.T, session string) string {
		t.Helper()
		if err := svc.RequestReset(ctx, session, credEmail); err != nil {
			t.Fatalf("RequestReset: %v", err)
		}
		fresh, err := svc.CheckResetCode(ctx, session, mail.last(t).msg.Code, func() string { return session + "-f" })
		if err != nil {
			t.Fatalf("CheckResetCode: %v", err)
		}
		return fresh
	}

	t.Run("step one marks the row, step two spends it with the equal pair", func(t *testing.T) {
		a := askAndConfirm(t, "steps-a")
		if a != "steps-a-f" {
			t.Fatalf("fresh session id = %q", a)
		}
		var verified, used, expires *time.Time
		if err := pool.QueryRow(ctx, `SELECT verified_at, used_at, expires_at FROM auth_codes WHERE session_id = $1`, a).Scan(&verified, &used, &expires); err != nil {
			t.Fatal(err)
		}
		if verified == nil || used != nil {
			t.Fatalf("after step one: verified=%v used=%v; want verified and unused", verified, used)
		}
		var onOld int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_codes WHERE session_id = 'steps-a'`).Scan(&onOld); err != nil || onOld != 0 {
			t.Fatalf("code rows left on the old session id = %d err=%v; want 0", onOld, err)
		}
		if lc, ok, err := svc.LiveCode(ctx, a); err != nil || !ok || !lc.Verified {
			t.Fatalf("LiveCode = %+v ok=%v err=%v; want verified", lc, ok, err)
		}
		if err := svc.ConfirmReset(ctx, a, newPassword); err != nil {
			t.Fatalf("ConfirmReset: %v", err)
		}
		if err := pool.QueryRow(ctx, `SELECT verified_at, used_at, expires_at FROM auth_codes WHERE session_id = $1`, a).Scan(&verified, &used, &expires); err != nil {
			t.Fatal(err)
		}
		if used == nil || !used.Equal(*expires) {
			t.Errorf("after step two: used=%v expires=%v; want the equal pair SpentCode reads", used, expires)
		}
		if spent, ok, err := svc.SpentCode(ctx, a); err != nil || !ok || spent.Purpose != service.PurposeReset {
			t.Errorf("SpentCode = %+v ok=%v err=%v; want the reset", spent, ok, err)
		}
		if _, err := svc.SignIn(ctx, "steps-x", credEmail, newPassword, credSessionIDs("steps-x2")); err != nil {
			t.Errorf("the new password does not sign in: %v", err)
		}
		if _, err := svc.SignIn(ctx, "steps-y", credEmail, credPassword, credSessionIDs("steps-y2")); !errors.Is(err, service.ErrWrongCredentials) {
			t.Errorf("the old password must stop working: %v", err)
		}
	})

	t.Run("two parallel step two calls give one success", func(t *testing.T) {
		b := askAndConfirm(t, "steps-b")
		var wg sync.WaitGroup
		results := make([]error, 2)
		for i := range results {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				results[i] = svc.ConfirmReset(ctx, b, "parallel password "+string(rune('a'+i))+" 4821")
			}(i)
		}
		wg.Wait()
		ok, wrong := 0, 0
		for _, err := range results {
			switch {
			case err == nil:
				ok++
			case errors.Is(err, service.ErrWrongCode):
				wrong++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}
		if ok != 1 || wrong != 1 {
			t.Errorf("outcomes %v; want one success and one ErrWrongCode", results)
		}
	})

	t.Run("the confirm window counts from the confirm, not from the send", func(t *testing.T) {
		c := askAndConfirm(t, "steps-c")
		// Stand the clock back by moving the row: sent 9 minutes ago, confirmed a
		// minute ago, so the code itself ran out a minute ago and the confirm has
		// 4 minutes left (the window is 5 minutes).
		testutil.MustExec(t, pool, `UPDATE auth_codes
			SET created_at = now() - interval '9 minutes 30 seconds',
			    expires_at = now() - interval '30 seconds',
			    verified_at = now() - interval '1 minute'
			WHERE session_id = 'steps-c-f'`)
		if lc, ok, err := svc.LiveCode(ctx, c); err != nil || !ok || !lc.Verified {
			t.Fatalf("LiveCode = %+v ok=%v err=%v; want verified although the code ran out", lc, ok, err)
		}
		if err := svc.ConfirmReset(ctx, c, newPassword); err != nil {
			t.Fatalf("ConfirmReset inside the confirm window: %v", err)
		}
	})

	t.Run("a confirm older than the confirm window is refused", func(t *testing.T) {
		d := askAndConfirm(t, "steps-d")
		testutil.MustExec(t, pool, `UPDATE auth_codes SET verified_at = now() - interval '6 minutes' WHERE session_id = 'steps-d-f'`)
		if err := svc.ConfirmReset(ctx, d, newPassword); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
		if lc, ok, _ := svc.LiveCode(ctx, d); !ok || lc.Verified {
			t.Errorf("LiveCode = %+v ok=%v; want shown but not verified", lc, ok)
		}
	})

	t.Run("step two with no confirm is refused and a newer code replaces a confirm", func(t *testing.T) {
		if err := svc.RequestReset(ctx, "steps-e", credEmail); err != nil {
			t.Fatal(err)
		}
		if err := svc.ConfirmReset(ctx, "steps-e", newPassword); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("unconfirmed code: %v, want ErrWrongCode", err)
		}
		f := askAndConfirm(t, "steps-f")
		if err := svc.RequestReset(ctx, f, credEmail); err != nil {
			t.Fatal(err)
		}
		if err := svc.ConfirmReset(ctx, f, newPassword); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("replaced confirm: %v, want ErrWrongCode", err)
		}
	})

	t.Run("a no account row answers like a wrong code and cannot be verified", func(t *testing.T) {
		if err := svc.RequestReset(ctx, "steps-g", "ghost@example.com"); err != nil {
			t.Fatal(err)
		}
		if id, err := svc.CheckResetCode(ctx, "steps-g", "123456", func() string { return "steps-g-f" }); !errors.Is(err, service.ErrWrongCode) || id != "" {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
		var verified *time.Time
		var attempts int
		if err := pool.QueryRow(ctx, `SELECT verified_at, attempts FROM auth_codes WHERE session_id = 'steps-g'`).Scan(&verified, &attempts); err != nil {
			t.Fatal(err)
		}
		if verified != nil || attempts != 1 {
			t.Errorf("no_account row: verified=%v attempts=%d; want none and 1", verified, attempts)
		}
	})

	t.Run("the table refuses a verified row that is not a reset code", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `UPDATE auth_codes SET verified_at = now() WHERE session_id = 'steps-g'`); err == nil {
			t.Error("a no_account row took a verified_at")
		}
		if err := svc.SignUp(ctx, "steps-h", "other@example.com", credPassword); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE auth_codes SET verified_at = now() WHERE session_id = 'steps-h'`); err == nil {
			t.Error("a sign up row took a verified_at")
		}
	})
}
