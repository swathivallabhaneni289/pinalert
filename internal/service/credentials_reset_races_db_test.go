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

// holdHasher is the real, cheap hasher, except that hashing the one password
// holdPassword waits until release is closed. Step two of Forgot password hashes
// the new password after it read the verified row and before its transaction, so
// the wait holds step two exactly between its read and its spend, and the test
// can write another code row, or send a Resend, in that gap.
type holdHasher struct {
	*auth.Hasher
	holdPassword string
	reached      chan struct{}
	release      chan struct{}
	once         sync.Once
}

func newHoldHasher(holdPassword string) *holdHasher {
	return &holdHasher{
		Hasher:       auth.NewHasher(auth.TestParams()),
		holdPassword: holdPassword,
		reached:      make(chan struct{}),
		release:      make(chan struct{}),
	}
}

func (h *holdHasher) Hash(ctx context.Context, password string) (string, error) {
	if password == h.holdPassword {
		h.once.Do(func() { close(h.reached) })
		select {
		case <-h.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return h.Hasher.Hash(ctx, password)
}

// bumpHookQuerier calls hook after every counted try of a code, so a test can
// stop step one between the count and the mark (the mark runs on the
// transaction's own querier and is not seen here).
type bumpHookQuerier struct {
	*sqlcgen.Queries
	mu   sync.Mutex
	hook func()
}

func (q *bumpHookQuerier) setHook(f func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.hook = f
}

func (q *bumpHookQuerier) BumpAuthCodeAttempt(ctx context.Context, arg sqlcgen.BumpAuthCodeAttemptParams) (sqlcgen.BumpAuthCodeAttemptRow, error) {
	row, err := q.Queries.BumpAuthCodeAttempt(ctx, arg)
	q.mu.Lock()
	hook := q.hook
	q.mu.Unlock()
	if hook != nil && err == nil {
		hook()
	}
	return row, err
}

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// TestResetStepRacesOnDatabase pins the races the doc comments of both reset
// steps promise (quick 261113-r5w, review fixes WR-03, IN-01, IN-07), with real
// transactions on the real queries.
func TestResetStepRacesOnDatabase(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	mail := &credMailer{}
	const holdPassword = "held brand new password 73"

	limits := service.DefaultCredentialLimits
	limits.SendCooldown = 0
	limits.SendsPerHour = 100
	hasher := newHoldHasher(holdPassword)
	hook := &bumpHookQuerier{Queries: sqlcgen.New(pool)}
	svc := service.NewCredentialService(hook, mail,
		service.WithHasher(hasher),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
		service.WithCredentialLimits(limits),
		service.WithCredentialTx(service.PgxCredentialTx(pool)),
	)
	if err := svc.SignUp(ctx, "race-up", credEmail, credPassword); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if _, err := svc.ConfirmSignUp(ctx, "race-up", mail.last(t).msg.Code, credSessionIDs("race-in")); err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}

	// Review fix IN-07: a code row written in the same session while step two
	// hashes makes the verified row no longer the newest, so step two spends
	// nothing and changes nothing.
	t.Run("a code written in the same session during step two is the wrong code answer", func(t *testing.T) {
		if err := svc.RequestReset(ctx, "race-a", credEmail); err != nil {
			t.Fatal(err)
		}
		fresh, err := svc.CheckResetCode(ctx, "race-a", mail.last(t).msg.Code, func() string { return "race-a-f" })
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- svc.ConfirmReset(ctx, fresh, holdPassword) }()
		waitClosed(t, hasher.reached, "step two to reach the hasher")
		// Create account on the same browser writes a newer row in the session.
		if err := svc.SignUp(ctx, fresh, "other-race@example.com", credPassword); err != nil {
			t.Fatalf("SignUp during the hold: %v", err)
		}
		close(hasher.release)
		if err := <-done; !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("step two = %v, want ErrWrongCode", err)
		}
		if _, err := svc.SignIn(ctx, "race-x", credEmail, credPassword, credSessionIDs("race-x2")); err != nil {
			t.Errorf("the old password stopped working: %v", err)
		}
		var used int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_codes WHERE session_id = $1 AND purpose = 'reset' AND used_at IS NOT NULL`, fresh).Scan(&used); err != nil || used != 0 {
			t.Errorf("used reset rows after the refused step two = %d err=%v; want 0", used, err)
		}
	})
}
