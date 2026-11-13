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

// The hold helpers (holdHasher, bumpHookQuerier, waitClosed) are in
// credentials_reset_races_db_test.go.

// TestResetStepTwoWithResendDuringIt: a Resend that lands between step two's read
// and its transaction retires the row first, so the spend writes 0 rows and
// everything rolls back (review fix WR-03 item 2). A Resend that comes after the
// spend only leaves a fresh live code for the address (review fix IN-01).
func TestResetStepTwoWithResendDuringIt(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	mail := &credMailer{}
	const holdPassword = "held resend new password 62"

	limits := service.DefaultCredentialLimits
	limits.SendCooldown = 0
	limits.SendsPerHour = 100
	hasher := newHoldHasher(holdPassword)
	svc := service.NewCredentialService(sqlcgen.New(pool), mail,
		service.WithHasher(hasher),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
		service.WithCredentialLimits(limits),
		service.WithCredentialTx(service.PgxCredentialTx(pool)),
	)
	if err := svc.SignUp(ctx, "res-up", credEmail, credPassword); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if _, err := svc.ConfirmSignUp(ctx, "res-up", mail.last(t).msg.Code, credSessionIDs("res-in")); err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}
	if err := svc.RequestReset(ctx, "res-a", credEmail); err != nil {
		t.Fatal(err)
	}
	fresh, err := svc.CheckResetCode(ctx, "res-a", mail.last(t).msg.Code, func() string { return "res-a-f" })
	if err != nil {
		t.Fatal(err)
	}
	var epochBefore int64
	if err := pool.QueryRow(ctx, `SELECT password_epoch FROM accounts WHERE email = $1`, credEmail).Scan(&epochBefore); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- svc.ConfirmReset(ctx, fresh, holdPassword) }()
	waitClosed(t, hasher.reached, "step two to reach the hasher")
	if err := svc.ResendCode(ctx, fresh); err != nil {
		t.Fatalf("ResendCode during the hold: %v", err)
	}
	close(hasher.release)
	if err := <-done; !errors.Is(err, service.ErrWrongCode) {
		t.Fatalf("step two = %v, want ErrWrongCode", err)
	}
	var epochAfter int64
	if err := pool.QueryRow(ctx, `SELECT password_epoch FROM accounts WHERE email = $1`, credEmail).Scan(&epochAfter); err != nil {
		t.Fatal(err)
	}
	if epochAfter != epochBefore {
		t.Errorf("the password epoch moved from %d to %d: the refused step two was not rolled back", epochBefore, epochAfter)
	}
	if _, err := svc.SignIn(ctx, "res-x", credEmail, credPassword, credSessionIDs("res-x2")); err != nil {
		t.Errorf("the old password stopped working: %v", err)
	}
	if _, err := svc.SignIn(ctx, "res-y", credEmail, holdPassword, credSessionIDs("res-y2")); !errors.Is(err, service.ErrWrongCredentials) {
		t.Errorf("the held new password was set: %v", err)
	}
	// The Resend's code is live and not confirmed.
	if lc, ok, err := svc.LiveCode(ctx, fresh); err != nil || !ok || lc.Verified {
		t.Errorf("LiveCode after the Resend = %+v ok=%v err=%v; want a live code, not confirmed", lc, ok, err)
	}

	// A Resend after the spend leaves a fresh live code for the address: the
	// reset is done and the new row is simply there.
	fresh2, err := svc.CheckResetCode(ctx, fresh, mail.last(t).msg.Code, func() string { return "res-a-f2" })
	if err != nil {
		t.Fatalf("step one on the Resend's code: %v", err)
	}
	if err := svc.ConfirmReset(ctx, fresh2, "a plain new password 48"); err != nil {
		t.Fatalf("step two on the Resend's code: %v", err)
	}
	if err := svc.RequestReset(ctx, "res-b", credEmail); err != nil {
		t.Fatalf("a code asked for after the reset: %v", err)
	}
	if lc, ok, err := svc.LiveCode(ctx, "res-b"); err != nil || !ok || lc.Verified {
		t.Errorf("a code asked for after the reset: %+v ok=%v err=%v; want live and not confirmed", lc, ok, err)
	}
}

// TestResetStepOneRaces: two right codes at once give one verify and one winner,
// and a Resend that lands between the count of a try and the mark makes the mark
// write nothing (review fix WR-03 items 3 and 4).
func TestResetStepOneRaces(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	mail := &credMailer{}

	limits := service.DefaultCredentialLimits
	limits.SendCooldown = 0
	limits.SendsPerHour = 100
	hook := &bumpHookQuerier{Queries: sqlcgen.New(pool)}
	svc := service.NewCredentialService(hook, mail,
		service.WithHasher(auth.NewHasher(auth.TestParams())),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
		service.WithCredentialLimits(limits),
		service.WithCredentialTx(service.PgxCredentialTx(pool)),
	)
	if err := svc.SignUp(ctx, "one-up", credEmail, credPassword); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if _, err := svc.ConfirmSignUp(ctx, "one-up", mail.last(t).msg.Code, credSessionIDs("one-in")); err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}

	t.Run("two right codes at once give one winner and one verify", func(t *testing.T) {
		if err := svc.RequestReset(ctx, "one-a", credEmail); err != nil {
			t.Fatal(err)
		}
		code := mail.last(t).msg.Code
		// Both requests count their try before either marks the row.
		var arrived sync.WaitGroup
		arrived.Add(2)
		all := make(chan struct{})
		go func() { arrived.Wait(); close(all) }()
		hook.setHook(func() {
			arrived.Done()
			select {
			case <-all:
			case <-time.After(10 * time.Second):
			}
		})
		defer hook.setHook(nil)

		var wg sync.WaitGroup
		ids := make([]string, 2)
		errs := make([]error, 2)
		for i := range ids {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				name := "one-a-f" + string(rune('1'+i))
				ids[i], errs[i] = svc.CheckResetCode(ctx, "one-a", code, func() string { return name })
			}(i)
		}
		wg.Wait()
		winners, wrong, winner := 0, 0, ""
		for i := range errs {
			switch {
			case errs[i] == nil:
				winners++
				winner = ids[i]
			case errors.Is(errs[i], service.ErrWrongCode) && ids[i] == "":
				wrong++
			default:
				t.Errorf("request %d: id %q err %v", i, ids[i], errs[i])
			}
		}
		if winners != 1 || wrong != 1 {
			t.Fatalf("outcomes %v %v; want one winner and one ErrWrongCode", errs, ids)
		}
		var rows, verified int
		if err := pool.QueryRow(ctx, `SELECT count(*), count(verified_at) FROM auth_codes WHERE session_id = $1`, winner).Scan(&rows, &verified); err != nil || rows != 1 || verified != 1 {
			t.Errorf("rows on the winner's session = %d verified %d err=%v; want 1 and 1", rows, verified, err)
		}
		var elsewhere int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_codes WHERE session_id IN ('one-a', 'one-a-f1', 'one-a-f2') AND session_id <> $1`, winner).Scan(&elsewhere); err != nil || elsewhere != 0 {
			t.Errorf("code rows left on the other ids = %d err=%v; want 0", elsewhere, err)
		}
		if lc, ok, err := svc.LiveCode(ctx, winner); err != nil || !ok || !lc.Verified {
			t.Errorf("LiveCode(winner) = %+v ok=%v err=%v; want confirmed", lc, ok, err)
		}
	})

	t.Run("a Resend between the count and the mark makes the mark write nothing", func(t *testing.T) {
		if err := svc.RequestReset(ctx, "one-b", credEmail); err != nil {
			t.Fatal(err)
		}
		code := mail.last(t).msg.Code
		reached := make(chan struct{})
		release := make(chan struct{})
		hook.setHook(func() {
			close(reached)
			<-release
		})
		defer hook.setHook(nil)

		type answer struct {
			id  string
			err error
		}
		done := make(chan answer, 1)
		go func() {
			id, err := svc.CheckResetCode(ctx, "one-b", code, func() string { return "one-b-f" })
			done <- answer{id, err}
		}()
		waitClosed(t, reached, "step one to count its try")
		hook.setHook(nil) // the Resend below must not wait at the hook
		if err := svc.ResendCode(ctx, "one-b"); err != nil {
			t.Fatalf("ResendCode during the hold: %v", err)
		}
		close(release)
		got := <-done
		if !errors.Is(got.err, service.ErrWrongCode) || got.id != "" {
			t.Fatalf("step one = %q, %v; want no id and ErrWrongCode", got.id, got.err)
		}
		var onFresh int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_codes WHERE session_id = 'one-b-f'`).Scan(&onFresh); err != nil || onFresh != 0 {
			t.Errorf("rows on the fresh id = %d err=%v; want 0: nothing rotated", onFresh, err)
		}
		if lc, ok, err := svc.LiveCode(ctx, "one-b"); err != nil || !ok || lc.Verified {
			t.Errorf("LiveCode on the old id = %+v ok=%v err=%v; want the Resend's live code, not confirmed", lc, ok, err)
		}
	})
}
