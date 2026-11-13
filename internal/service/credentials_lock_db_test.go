package service_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/auth"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// holdPoint pauses the first call of one querier method inside a confirm
// transaction, after the statement ran, until the test releases it.
type holdPoint struct {
	method  string
	armed   atomic.Bool
	reached chan struct{}
	release chan struct{}
}

func newHoldPoint(method string) *holdPoint {
	h := &holdPoint{method: method, reached: make(chan struct{}), release: make(chan struct{})}
	h.armed.Store(true)
	return h
}

func (h *holdPoint) pause(method string) {
	if method == h.method && h.armed.CompareAndSwap(true, false) {
		close(h.reached)
		<-h.release
	}
}

// holdQuerier is the transaction's querier with a hold after the two
// statements the lock order test needs to stop at.
type holdQuerier struct {
	service.CredentialQuerier
	hold *holdPoint
}

func (q holdQuerier) SetAccountPasswordNewEpoch(ctx context.Context, arg sqlcgen.SetAccountPasswordNewEpochParams) (int64, error) {
	n, err := q.CredentialQuerier.SetAccountPasswordNewEpoch(ctx, arg)
	q.hold.pause("SetAccountPasswordNewEpoch")
	return n, err
}

func (q holdQuerier) ConsumeAuthCode(ctx context.Context, id int64) (int64, error) {
	n, err := q.CredentialQuerier.ConsumeAuthCode(ctx, id)
	q.hold.pause("ConsumeAuthCode")
	return n, err
}

// waitForBlockedLock waits until some backend waits on a lock it was not
// granted, so the test knows the second transaction is stuck behind the first.
func waitForBlockedLock(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_locks WHERE NOT granted`).Scan(&waiting); err != nil {
			t.Fatalf("reading pg_locks: %v", err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the second confirm never waited on a lock")
}

func isDeadlock(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "40P01"
}

// receive reads one result or fails after 15 seconds, so a stuck goroutine
// fails the test instead of hanging it.
func receive(t *testing.T, label string, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(15 * time.Second):
		t.Fatalf("%s never returned", label)
		return nil
	}
}

// TestConfirmLockOrder is SK-05 of the Phase 7.1 review fix skeptic. Both
// confirm transactions first take one advisory lock for the address, so two of
// them for one address run one after the other and never deadlock. Before the
// fix they took the account row and the code rows in opposite orders, and
// Postgres aborted one of them with 40P01, which reached the person as a 500.
func TestConfirmLockOrder(t *testing.T) {
	newRig := func(t *testing.T, hold *holdPoint) (*pgxpool.Pool, *service.CredentialService, *credMailer) {
		pool := testutil.NewTestDB(t)
		q := sqlcgen.New(pool)
		mail := &credMailer{}
		limits := service.DefaultCredentialLimits
		limits.SendCooldown = 0
		svc := service.NewCredentialService(q, mail,
			service.WithHasher(auth.NewHasher(auth.TestParams())),
			service.WithCodeKey(credCodeKey),
			service.WithThrottleKey(credThrottleKey),
			service.WithCredentialLimits(limits),
			service.WithCredentialTx(func(ctx context.Context, fn func(q service.CredentialQuerier) error) error {
				return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
					return fn(holdQuerier{CredentialQuerier: sqlcgen.New(tx), hold: hold})
				})
			}),
		)
		return pool, svc, mail
	}
	checkOutcome := func(t *testing.T, first, second error) {
		t.Helper()
		if isDeadlock(first) || isDeadlock(second) {
			t.Fatalf("the two confirms deadlocked: first %v, second %v", first, second)
		}
		ok, wrong := 0, 0
		for _, err := range []error{first, second} {
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
			t.Errorf("outcomes: first %v, second %v; want one success and one ErrWrongCode", first, second)
		}
	}

	t.Run("a legacy account confirms a reset and a sign up at once", func(t *testing.T) {
		hold := newHoldPoint("SetAccountPasswordNewEpoch")
		pool, svc, mail := newRig(t, hold)
		ctx := context.Background()
		const legacy = "lock-order-legacy@example.com"
		testutil.MustExec(t, pool, `INSERT INTO accounts (email) VALUES ($1)`, legacy)
		if err := svc.SignUp(ctx, "sess-up", legacy, credPassword); err != nil {
			t.Fatalf("SignUp: %v", err)
		}
		signUpCode := mail.last(t).msg.Code
		if err := svc.RequestReset(ctx, "sess-reset", legacy); err != nil {
			t.Fatalf("RequestReset: %v", err)
		}
		resetCode := mail.last(t).msg.Code

		resetDone := make(chan error, 1)
		freshReset, err := svc.CheckResetCode(ctx, "sess-reset", resetCode, credSessionIDs("lock-reset"))
		if err != nil {
			t.Fatalf("CheckResetCode: %v", err)
		}
		go func() { resetDone <- svc.ConfirmReset(ctx, freshReset, "a brand new password 55") }()
		select {
		case <-hold.reached:
		case err := <-resetDone:
			t.Fatalf("the reset finished before the hold: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatal("the reset never reached the hold")
		}
		signUpDone := make(chan error, 1)
		go func() {
			_, err := svc.ConfirmSignUp(ctx, "sess-up", signUpCode, credSessionIDs("lock-up"))
			signUpDone <- err
		}()
		waitForBlockedLock(t, pool)
		close(hold.release)
		checkOutcome(t, receive(t, "the reset", resetDone), receive(t, "the sign up", signUpDone))
	})

	t.Run("two browsers confirm two sign up codes for one new address at once", func(t *testing.T) {
		hold := newHoldPoint("ConsumeAuthCode")
		pool, svc, mail := newRig(t, hold)
		ctx := context.Background()
		const fresh = "lock-order-new@example.com"
		if err := svc.SignUp(ctx, "sess-1", fresh, credPassword); err != nil {
			t.Fatalf("first SignUp: %v", err)
		}
		code1 := mail.last(t).msg.Code
		if err := svc.SignUp(ctx, "sess-2", fresh, credPassword); err != nil {
			t.Fatalf("second SignUp: %v", err)
		}
		code2 := mail.last(t).msg.Code

		firstDone := make(chan error, 1)
		go func() {
			_, err := svc.ConfirmSignUp(ctx, "sess-1", code1, credSessionIDs("lock-1"))
			firstDone <- err
		}()
		select {
		case <-hold.reached:
		case err := <-firstDone:
			t.Fatalf("the first confirm finished before the hold: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatal("the first confirm never reached the hold")
		}
		secondDone := make(chan error, 1)
		go func() {
			_, err := svc.ConfirmSignUp(ctx, "sess-2", code2, credSessionIDs("lock-2"))
			secondDone <- err
		}()
		waitForBlockedLock(t, pool)
		close(hold.release)
		checkOutcome(t, receive(t, "the first confirm", firstDone), receive(t, "the second confirm", secondDone))
	})
}
