package mailer

import (
	"fmt"
	"log"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// defaultSMTPPort is the submission port, where the connection starts plain
// and is upgraded with STARTTLS before any password is sent.
const defaultSMTPPort = 587

// isPlainHostName reports whether host is a bare host name: no colon (a port,
// or a scheme such as smtps://), no slash, and no space or control character
// anywhere. Such a value cannot be dialed, and without this check every send
// would fail later as "could not reach the mail server".
func isPlainHostName(host string) bool {
	for _, r := range host {
		if r == ':' || r == '/' || unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return host != ""
}

// LoadMailer selects a Mailer implementation from the process environment,
// mirroring cmd/server/main.go's loadSessionSecret fail-fast shape exactly.
// The settings are SMTP_HOST, SMTP_PORT (default 587), SMTP_USERNAME,
// SMTP_PASSWORD and MAIL_FROM (default "Pinalert <SMTP_USERNAME>"):
//
//   - SMTP_HOST set: it must be a bare host name (no port, scheme, slash or
//     space), SMTP_USERNAME and SMTP_PASSWORD are required too, and the SMTP
//     implementation is returned, regardless of env.
//   - No SMTP setting set and env == "development": return the log-only
//     implementation and warn, so local development and CI never touch the
//     network.
//   - No SMTP setting set and env is anything else (including ""): return an
//     error naming the settings. A silent degradation here means visitors
//     quietly never receive their emailed codes in production, so the caller
//     (cmd/server/main.go) must turn this into a startup fatal rather than let
//     the server boot with a log-only mailer no real visitor can read.
//   - Some SMTP setting set but not SMTP_HOST: an error outside development
//     (a half set group is a typo to fix, not a reason to boot without mail),
//     and in development a warning and the log-only implementation.
//
// An error names a setting, never its value. LoadMailer itself never calls
// log.Fatal; returning an error keeps it testable, and cmd/server/main.go is
// where a fatal belongs.
func LoadMailer(env string) (Mailer, error) {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	portText := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	username := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	// Google shows an app password in four groups of four letters separated by
	// spaces; the spaces are not part of it.
	password := strings.ReplaceAll(strings.TrimSpace(os.Getenv("SMTP_PASSWORD")), " ", "")
	fromText := strings.TrimSpace(os.Getenv("MAIL_FROM"))

	anySet := host != "" || portText != "" || username != "" || password != "" || fromText != ""

	if host == "" {
		if env == "development" {
			if anySet {
				log.Println("WARNING: SMTP_HOST unset but other SMTP settings are set; mail will be printed to the log instead of sent")
			} else {
				log.Println("WARNING: SMTP_HOST unset; mail will be printed to the log instead of sent")
			}
			return NewLogMailer(), nil
		}
		if anySet {
			return nil, fmt.Errorf("mailer: SMTP_HOST must be set when other SMTP settings are set")
		}
		return nil, fmt.Errorf("mailer: SMTP_HOST, SMTP_USERNAME and SMTP_PASSWORD must be set outside ENV=development")
	}

	if !isPlainHostName(host) {
		return nil, fmt.Errorf("mailer: SMTP_HOST must be a host name such as smtp.gmail.com, with no port, scheme or space")
	}

	var missing []string
	if username == "" {
		missing = append(missing, "SMTP_USERNAME")
	}
	if password == "" {
		missing = append(missing, "SMTP_PASSWORD")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("mailer: %s must be set when SMTP_HOST is set", strings.Join(missing, " and "))
	}

	port := defaultSMTPPort
	if portText != "" {
		p, err := strconv.Atoi(portText)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("mailer: SMTP_PORT must be a port number from 1 to 65535")
		}
		port = p
	}

	if fromText == "" {
		fromText = (&mail.Address{Name: "Pinalert", Address: username}).String()
	}
	from, err := mail.ParseAddress(fromText)
	if err != nil || strings.ContainsAny(fromText, "\r\n\x00") {
		return nil, fmt.Errorf("mailer: MAIL_FROM must be one address such as Pinalert <name@example.com> (when MAIL_FROM is unset, SMTP_USERNAME must be an email address)")
	}

	return NewSMTPMailer(SMTPConfig{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from.String(),
	}), nil
}
