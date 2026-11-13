package mailer

import (
	"errors"
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

// requiredSettingError is a load error about SMTP_HOST, SMTP_USERNAME or
// SMTP_PASSWORD: one of them is missing, or SMTP_HOST is not a host name. It
// holds the same text a plain error would, and lets a caller (mailcheck) tell
// these apart from a bad SMTP_PORT or MAIL_FROM without matching on words.
type requiredSettingError struct{ msg string }

func (e *requiredSettingError) Error() string { return e.msg }

// requiredSetting builds a requiredSettingError from a format and arguments.
func requiredSetting(format string, args ...any) error {
	return &requiredSettingError{msg: fmt.Sprintf(format, args...)}
}

// IsRequiredSettingError reports whether err, from LoadMailer, is about
// SMTP_HOST, SMTP_USERNAME or SMTP_PASSWORD (missing, or a host that is not a
// host name), as opposed to a bad SMTP_PORT or MAIL_FROM.
func IsRequiredSettingError(err error) bool {
	var target *requiredSettingError
	return errors.As(err, &target)
}

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

// gmailHost is the one host whose password the loader reshapes.
const gmailHost = "smtp.gmail.com"

// normalizePassword drops the spaces of a Google app password and touches no
// other password. Google shows an app password in four groups of four letters
// separated by spaces, and the spaces are not part of it. The spaces go only
// when the host is smtp.gmail.com and what is left is exactly 16 ASCII letters;
// any other value, or any other host, is used as given, so the password of
// another server that holds a space is never silently changed.
func normalizePassword(host, password string) string {
	if !strings.EqualFold(host, gmailHost) {
		return password
	}
	stripped := strings.ReplaceAll(password, " ", "")
	if len(stripped) != 16 {
		return password
	}
	for i := 0; i < len(stripped); i++ {
		c := stripped[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return password
		}
	}
	return stripped
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
	password := strings.TrimSpace(os.Getenv("SMTP_PASSWORD"))
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
			return nil, requiredSetting("mailer: SMTP_HOST must be set when other SMTP settings are set")
		}
		return nil, requiredSetting("mailer: SMTP_HOST, SMTP_USERNAME and SMTP_PASSWORD must be set outside ENV=development")
	}

	if !isPlainHostName(host) {
		return nil, requiredSetting("mailer: SMTP_HOST must be a host name such as smtp.gmail.com, with no port, scheme or space")
	}

	password = normalizePassword(host, password)

	var missing []string
	if username == "" {
		missing = append(missing, "SMTP_USERNAME")
	}
	if password == "" {
		missing = append(missing, "SMTP_PASSWORD")
	}
	if len(missing) > 0 {
		return nil, requiredSetting("mailer: %s must be set when SMTP_HOST is set", strings.Join(missing, " and "))
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
