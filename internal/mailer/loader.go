package mailer

import (
	"fmt"
	"log"
	"os"
)

// defaultFrom is used when RESEND_API_KEY is set but RESEND_FROM is not —
// Resend's own sandbox sender, which is deliverable to the Resend account
// owner's own signup address without any domain verification. See
// README.md's "Email delivery (Resend)" section for the pre-launch
// requirement to replace this with a DNS-verified custom-domain address
// before real visitors can receive mail.
const defaultFrom = "Pinalert <onboarding@resend.dev>"

// LoadMailer selects a Mailer implementation from the process environment,
// mirroring cmd/server/main.go's loadSessionSecret fail-fast shape exactly:
//
//   - RESEND_API_KEY set: return the Resend-backed implementation,
//     regardless of env. RESEND_FROM overrides the sandbox default when set.
//   - RESEND_API_KEY unset and env == "development": return the log-only
//     implementation and warn, so local development and CI never touch the
//     network.
//   - RESEND_API_KEY unset and env is anything else (including ""): return
//     an error naming RESEND_API_KEY. A silent degradation here means
//     visitors quietly never receive their links in production, so the
//     caller (cmd/server/main.go) must turn this into a startup fatal
//     rather than let the server boot with a log-only mailer no real
//     visitor can read.
//
// LoadMailer itself never calls log.Fatal — returning an error keeps it
// testable; cmd/server/main.go is where a fatal belongs.
func LoadMailer(env string) (Mailer, error) {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey != "" {
		from := os.Getenv("RESEND_FROM")
		if from == "" {
			from = defaultFrom
		}
		return NewResendMailer(apiKey, from), nil
	}

	if env != "development" {
		return nil, fmt.Errorf("mailer: RESEND_API_KEY must be set outside ENV=development")
	}

	log.Println("WARNING: RESEND_API_KEY unset; mail will be printed to the log instead of sent")
	return NewLogMailer(), nil
}
