package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pinalert/internal/service"
)

// sendWaitOf runs err through errors.As and fails when it is not a
// SendWaitError that is also an ErrRateLimited.
func sendWaitOf(t *testing.T, err error) service.SendWaitError {
	t.Helper()
	var sw service.SendWaitError
	if !errors.As(err, &sw) {
		t.Fatalf("err = %v, want a SendWaitError", err)
	}
	if !errors.Is(err, service.ErrRateLimited) {
		t.Fatalf("err = %v, a SendWaitError must still be ErrRateLimited", err)
	}
	if errors.Is(err, service.ErrTooManyCodes) {
		t.Fatalf("err = %v, a SendWaitError must not be ErrTooManyCodes", err)
	}
	return sw
}

func TestSendWaitMessage(t *testing.T) {
	cases := []struct {
		wait time.Duration
		want string
	}{
		{20 * time.Second, "A code was just sent to this email. Try again in 20 seconds."},
		{19*time.Second + time.Millisecond, "A code was just sent to this email. Try again in 20 seconds."},
		{2 * time.Second, "A code was just sent to this email. Try again in 2 seconds."},
		{time.Second, "A code was just sent to this email. Try again in 1 second."},
		{500 * time.Millisecond, "A code was just sent to this email. Try again in 1 second."},
		{0, "A code was just sent to this email. Try again in 1 second."},
	}
	for _, c := range cases {
		if got := service.SendWaitMessage(c.wait); got != c.want {
			t.Errorf("SendWaitMessage(%v) = %q, want %q", c.wait, got, c.want)
		}
	}
}

// TestSendWaitTimeLeft pins the real time left in a refusal inside the 45
// second wait, and that the other refusals keep their sentinels.
func TestSendWaitTimeLeft(t *testing.T) {
	ctx := context.Background()

	t.Run("sign up 25 seconds after the first send has 20 seconds left", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		r.clk.Advance(25 * time.Second)
		hashed, rows, sent := len(r.hasher.hashed), len(r.q.codes), len(r.mail.sent)
		sw := sendWaitOf(t, r.svc.SignUp(ctx, "sess-a", credEmail, credPassword))
		if sw.Wait != 20*time.Second {
			t.Errorf("Wait = %v, want 20s", sw.Wait)
		}
		if sw.Error() != "A code was just sent to this email. Try again in 20 seconds." {
			t.Errorf("Error() = %q", sw.Error())
		}
		if len(r.hasher.hashed) != hashed || len(r.q.codes) != rows || len(r.mail.sent) != sent {
			t.Error("a refused claim must not hash, write or mail")
		}
	})

	t.Run("44.5 seconds after the send has 1 second left", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		r.clk.Advance(44*time.Second + 500*time.Millisecond)
		sw := sendWaitOf(t, r.svc.SignUp(ctx, "sess-a", credEmail, credPassword))
		if sw.Wait != time.Second {
			t.Errorf("Wait = %v, want 1s", sw.Wait)
		}
		if got := service.SendWaitMessage(sw.Wait); got != "A code was just sent to this email. Try again in 1 second." {
			t.Errorf("message = %q, want the 1 second sentence", got)
		}
	})

	t.Run("a refusal never shows less than 1 second", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		r.clk.Advance(time.Second)
		sw := sendWaitOf(t, r.svc.SignUp(ctx, "sess-a", credEmail, credPassword))
		if sw.Wait != 44*time.Second {
			t.Errorf("Wait = %v, want 44s", sw.Wait)
		}
	})

	t.Run("resend 25 seconds after the send has 20 seconds left", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		r.clk.Advance(25 * time.Second)
		sw := sendWaitOf(t, r.svc.ResendCode(ctx, "sess-a"))
		if sw.Wait != 20*time.Second {
			t.Errorf("Wait = %v, want 20s", sw.Wait)
		}
	})

	t.Run("forgot password 25 seconds after a sign up code has 20 seconds left", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		r.clk.Advance(25 * time.Second)
		sw := sendWaitOf(t, r.svc.RequestReset(ctx, "sess-b", credEmail))
		if sw.Wait != 20*time.Second {
			t.Errorf("Wait = %v, want 20s", sw.Wait)
		}
	})

	t.Run("the hourly count spent is still too many codes", func(t *testing.T) {
		r := newCredRig(t)
		for i := 0; i < 5; i++ {
			if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
				t.Fatalf("send %d: %v", i+1, err)
			}
			r.clk.Advance(46 * time.Second)
		}
		err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword)
		if !errors.Is(err, service.ErrTooManyCodes) {
			t.Fatalf("err = %v, want ErrTooManyCodes", err)
		}
		var sw service.SendWaitError
		if errors.As(err, &sw) {
			t.Errorf("the hourly refusal must not carry a send wait: %v", err)
		}
		// Inside the cooldown with the count spent, the hour sentence wins.
		r2 := newCredRig(t)
		for i := 0; i < 5; i++ {
			if err := r2.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
				t.Fatalf("send %d: %v", i+1, err)
			}
			r2.clk.Advance(46 * time.Second)
		}
		r2.clk.Advance(-30 * time.Second)
		if err := r2.svc.SignUp(ctx, "sess-a", credEmail, credPassword); !errors.Is(err, service.ErrTooManyCodes) {
			t.Fatalf("spent count inside the cooldown: %v, want ErrTooManyCodes", err)
		}
	})
}

// TestSendWaitSameForKnownAndUnknown is the enumeration check: the claim comes
// before the account lookup on every route, so an address with a password, an
// address with none and an unknown address all get the same refusal and the
// same time left.
func TestSendWaitSameForKnownAndUnknown(t *testing.T) {
	ctx := context.Background()
	const knownEmail = "known.person@example.com"
	const unknownEmail = "unknown.person@example.com"

	// answer sends once, waits 25 seconds, and returns what the second send says.
	answer := func(t *testing.T, r *credRig, email string, send func() error) string {
		t.Helper()
		if err := send(); err != nil {
			t.Fatalf("first send to %s: %v", email, err)
		}
		r.clk.Advance(25 * time.Second)
		sw := sendWaitOf(t, send())
		return sw.Error() + " | " + sw.Wait.String()
	}

	t.Run("sign up", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, knownEmail, credPassword)
		known := answer(t, r, knownEmail, func() error { return r.svc.SignUp(ctx, "sess-k", knownEmail, credPassword) })
		r2 := newCredRig(t)
		unknown := answer(t, r2, unknownEmail, func() error { return r2.svc.SignUp(ctx, "sess-u", unknownEmail, credPassword) })
		if known != unknown {
			t.Errorf("known address answers %q, unknown %q", known, unknown)
		}
	})

	t.Run("forgot password", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, knownEmail, credPassword)
		known := answer(t, r, knownEmail, func() error { return r.svc.RequestReset(ctx, "sess-k", knownEmail) })
		r2 := newCredRig(t)
		unknown := answer(t, r2, unknownEmail, func() error { return r2.svc.RequestReset(ctx, "sess-u", unknownEmail) })
		if known != unknown {
			t.Errorf("known address answers %q, unknown %q", known, unknown)
		}
	})

	t.Run("resend after a sign up on a known and an unknown address", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, knownEmail, credPassword)
		known := answer(t, r, knownEmail, func() error {
			if r.q.count("ClaimAuthEmailSend") == 0 {
				return r.svc.SignUp(ctx, "sess-k", knownEmail, credPassword)
			}
			return r.svc.ResendCode(ctx, "sess-k")
		})
		r2 := newCredRig(t)
		unknown := answer(t, r2, unknownEmail, func() error {
			if r2.q.count("ClaimAuthEmailSend") == 0 {
				return r2.svc.SignUp(ctx, "sess-u", unknownEmail, credPassword)
			}
			return r2.svc.ResendCode(ctx, "sess-u")
		})
		if known != unknown {
			t.Errorf("known address answers %q, unknown %q", known, unknown)
		}
	})

	t.Run("two forgot password requests say the same on a known and an unknown address", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, knownEmail, credPassword)
		if err := r.svc.RequestReset(ctx, "sess-k", knownEmail); err != nil {
			t.Fatal(err)
		}
		r.clk.Advance(25 * time.Second)
		known := sendWaitOf(t, r.svc.RequestReset(ctx, "sess-k2", knownEmail))
		r2 := newCredRig(t)
		if err := r2.svc.RequestReset(ctx, "sess-u", unknownEmail); err != nil {
			t.Fatal(err)
		}
		r2.clk.Advance(25 * time.Second)
		unknown := sendWaitOf(t, r2.svc.RequestReset(ctx, "sess-u2", unknownEmail))
		if known.Wait != unknown.Wait || known.Wait != 20*time.Second {
			t.Errorf("known waits %v, unknown %v, want both 20s", known.Wait, unknown.Wait)
		}
	})
}
