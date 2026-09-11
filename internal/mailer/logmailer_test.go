package mailer_test

import (
	"context"
	"testing"

	"pinalert/internal/mailer"
)

func TestNewLogMailerSatisfiesInterface(t *testing.T) {
	var m mailer.Mailer = mailer.NewLogMailer()

	if err := m.SendVerificationLink(context.Background(), "visitor@example.com", "https://pinalert.example/auth/verify?token=abc"); err != nil {
		t.Fatalf("SendVerificationLink returned %v, want nil", err)
	}
}
