package mailer

import (
	"context"
	"log"
)

// logMailer implements Mailer by printing the kind and code of each auth
// email to the process log instead of sending real email: the
// development/test fallback so the flow can be completed end to end without a
// provider key or network access. It deliberately logs the code (accepted
// risk AR-28, 01.1-01-PLAN.md): this is the only way to complete the flow
// without a mail account, and selecting this implementation is a fatal error
// outside ENV=development.
type logMailer struct{}

// NewLogMailer constructs a Mailer that logs instead of sending real email.
func NewLogMailer() Mailer {
	return logMailer{}
}

// SendAuthEmail prints the kind and the code of an auth email, never the
// address and never a link (accepted dev only risk AR-28). The code is empty
// for the two kinds that carry none. LoadMailer selects this implementation
// only under ENV=development.
func (logMailer) SendAuthEmail(ctx context.Context, to string, msg AuthEmail) error {
	log.Printf("mailer: [dev] auth email kind=%s code=%s", msg.Kind, msg.Code)
	return nil
}
