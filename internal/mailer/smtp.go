package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// SMTPConfig holds the settings of the SMTP mailer. Password is the one secret
// in it: it is handed to the server after the connection is encrypted and
// nowhere else, and it never appears in an error or a log line.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	// From is the sender, in the form "Pinalert <name@example.com>" or a bare
	// address. It is parsed with net/mail at send time.
	From string
}

// String keeps the password, the user name and the sender out of every fmt
// verb: a config printed by mistake (a debug line, an error that wraps it)
// shows the host and port only. It formats the fields, never c itself, which
// would call String again.
func (c SMTPConfig) String() string {
	return fmt.Sprintf("SMTPConfig{Host:%s Port:%d Username:[redacted] Password:[redacted] From:[redacted]}", c.Host, c.Port)
}

// GoString is the %#v form; it is as redacted as String.
func (c SMTPConfig) GoString() string { return c.String() }

// Format covers every verb, not only the ones a Stringer is asked for: without
// it %d would walk the fields and print the password as "%!d(string=...)". It
// takes precedence over String and GoString in fmt.
func (c SMTPConfig) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, c.String())
}

// smtpDefaultTimeout bounds the whole exchange (dial, TLS, login and send)
// when the caller's context has no deadline of its own.
const smtpDefaultTimeout = 20 * time.Second

// smtpHelloName is the name the client introduces itself with.
const smtpHelloName = "localhost"

// smtpImplicitTLSPort is the port where TLS starts at the first byte. Every
// other port uses STARTTLS.
const smtpImplicitTLSPort = 465

// The kinds of SMTP failure the caller's log can see. None of them holds any
// text from the server's answer, which can repeat the recipient address, and
// none holds a setting's value.
var (
	errSMTPNetwork   = errors.New("could not reach the mail server")
	errSMTPTLS       = errors.New("could not secure the connection to the mail server")
	errSMTPNoTLS     = errors.New("the mail server did not start encryption")
	errSMTPTemporary = errors.New("the mail server asked to try again later")
	errSMTPAuth      = errors.New("the mail server refused the sign in")
	errSMTPNoAuth    = errors.New("the mail server offers no password sign in")
	errSMTPAuthLost  = errors.New("the mail server dropped the connection during sign in")
	errSMTPPermanent = errors.New("the mail server refused the message")

	// errBadRecipient and errBadSender name no address.
	errBadRecipient = errors.New("mailer: the recipient is not one plain address")
	errBadSender    = errors.New("mailer: the sender is not a valid address")
	errBadHeader    = errors.New("mailer: a header holds a line break")
)

// smtpStage tells smtpFailure how far the exchange got, because the same reply
// code means something different at the sign in step than at the message step.
type smtpStage int

const (
	stageConnect smtpStage = iota
	stageTLS
	stageAuth
	stageEnvelope
	stageData
)

// smtpMailer implements Mailer over SMTP using the standard library only.
type smtpMailer struct {
	// cfg is a pointer on purpose. fmt cannot call a method on an unexported
	// field, so a struct that holds this mailer (the send ceiling, the
	// service) would print a config held by value, password and all, under a
	// verb such as %s or %d. A pointer prints as an address there.
	cfg *SMTPConfig
	// rootCAs replaces the system roots when not nil. Only tests in this
	// package set it, to trust the certificate of an in process fake server.
	rootCAs *x509.CertPool
	// forceImplicitTLS makes a test server on a random port behave like port
	// 465. Nothing outside tests sets it.
	forceImplicitTLS bool
}

// String, GoString and Format hide the config (and so the password) when the
// mailer itself is printed. They sit on the value receiver, so a pointer and a
// copy are both covered. fmt cannot call a method on the unexported cfg field,
// which is why the mailer needs its own.
func (m smtpMailer) String() string { return "smtpMailer{" + m.cfg.String() + "}" }

// GoString is the %#v form of the mailer.
func (m smtpMailer) GoString() string { return m.String() }

// Format covers every verb, like SMTPConfig.Format.
func (m smtpMailer) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, m.String())
}

// implicitTLS reports whether the connection is encrypted from the first byte
// (port 465) and not upgraded with STARTTLS.
func (m *smtpMailer) implicitTLS() bool {
	return m.cfg.Port == smtpImplicitTLSPort || m.forceImplicitTLS
}

// NewSMTPMailer constructs a Mailer that sends through an SMTP server. Port 465
// means TLS from the first byte. Any other port means STARTTLS, and the send is
// refused if the server does not offer it. The password is never sent before
// the connection is encrypted, and the server's certificate is always checked
// (TLS 1.2 or newer, against the host name in cfg).
func NewSMTPMailer(cfg SMTPConfig) Mailer {
	c := cfg
	return &smtpMailer{cfg: &c}
}

// NewSMTPMailerWithRootCAs is NewSMTPMailer with its own set of trusted root
// certificates instead of the system's. The server's certificate is still
// checked in full (chain, host name, TLS 1.2 or newer); only the trust anchors
// differ. It exists so tests in other packages (cmd/mailcheck) can reach an in
// process fake server with a certificate made in the test. Nothing in the
// server's own startup path calls it, and TestTestOnlyHooksStayOutOfProductionCode
// fails if a non test file outside this package ever does.
func NewSMTPMailerWithRootCAs(cfg SMTPConfig, roots *x509.CertPool) Mailer {
	c := cfg
	return &smtpMailer{cfg: &c, rootCAs: roots}
}

// SendAuthEmail sends one auth email with a plain text part and an HTML part.
// The returned error names the operation and the kind of failure only. It
// never holds the server's words, the password, the username, the recipient or
// the code.
func (m *smtpMailer) SendAuthEmail(ctx context.Context, to string, msg AuthEmail) error {
	subject, text, htmlBody, err := AuthEmailContent(msg)
	if err != nil {
		return err
	}
	if err := validateRecipient(to); err != nil {
		return err
	}
	sender, err := mail.ParseAddress(m.cfg.From)
	if err != nil || hasLineBreak(m.cfg.From) {
		return errBadSender
	}
	data, err := buildMessage(sender, to, subject, text, htmlBody, time.Now())
	if err != nil {
		return err
	}
	if err := m.deliver(ctx, sender.Address, to, data); err != nil {
		return fmt.Errorf("mailer: sending auth email via smtp: %w", err)
	}
	return nil
}

// validateRecipient accepts exactly one plain address: no line break, no
// display name, no angle brackets, no list.
func validateRecipient(to string) error {
	if to == "" || hasLineBreak(to) {
		return errBadRecipient
	}
	a, err := mail.ParseAddress(to)
	if err != nil || a.Name != "" || a.Address != to {
		return errBadRecipient
	}
	return nil
}

// hasLineBreak reports whether s holds CR, LF or NUL, the bytes that split a
// header or an SMTP command.
func hasLineBreak(s string) bool {
	return strings.ContainsAny(s, "\r\n\x00")
}

// buildMessage assembles the RFC 5322 message: the headers and a
// multipart/alternative body with the text part first and the HTML part second,
// both UTF-8 quoted-printable. It refuses a line break in the recipient or the
// subject (header injection).
func buildMessage(sender *mail.Address, to, subject, text, htmlBody string, now time.Time) ([]byte, error) {
	if hasLineBreak(to) || hasLineBreak(subject) {
		return nil, errBadHeader
	}
	domain := "localhost"
	if i := strings.LastIndex(sender.Address, "@"); i >= 0 && i+1 < len(sender.Address) {
		domain = sender.Address[i+1:]
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, errors.New("mailer: could not make a message id")
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, part := range []struct{ contentType, content string }{
		{"text/plain; charset=UTF-8", text},
		{"text/html; charset=UTF-8", htmlBody},
	} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.contentType},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, errors.New("mailer: could not build the message")
		}
		qw := quotedprintable.NewWriter(pw)
		if _, err := qw.Write([]byte(part.content)); err != nil {
			return nil, errors.New("mailer: could not build the message")
		}
		if err := qw.Close(); err != nil {
			return nil, errors.New("mailer: could not build the message")
		}
	}
	if err := mw.Close(); err != nil {
		return nil, errors.New("mailer: could not build the message")
	}

	var out bytes.Buffer
	header := func(name, value string) {
		out.WriteString(name + ": " + value + "\r\n")
	}
	header("Date", now.Format(time.RFC1123Z))
	header("From", sender.String())
	header("To", to)
	header("Subject", mime.QEncoding.Encode("utf-8", subject))
	header("Message-ID", "<"+hex.EncodeToString(id[:])+"@"+domain+">")
	header("MIME-Version", "1.0")
	header("Content-Type", mime.FormatMediaType("multipart/alternative", map[string]string{"boundary": mw.Boundary()}))
	out.WriteString("\r\n")
	out.Write(body.Bytes())
	return out.Bytes(), nil
}

// deliver runs one SMTP exchange. The whole exchange is bounded by the
// context's deadline, or by smtpDefaultTimeout when it has none. The caller
// wraps the returned kind.
func (m *smtpMailer) deliver(ctx context.Context, envelopeFrom, rcpt string, data []byte) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, smtpDefaultTimeout)
		defer cancel()
	}
	deadline, _ := ctx.Deadline()

	tlsCfg := &tls.Config{
		ServerName: m.cfg.Host,
		MinVersion: tls.VersionTLS12,
		RootCAs:    m.rootCAs,
	}
	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))

	var dialer net.Dialer
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return smtpFailure(ctx, err, stageConnect)
	}
	defer raw.Close()
	// A cancelled context closes the connection, which unblocks any read.
	stop := context.AfterFunc(ctx, func() { raw.Close() })
	defer stop()
	if err := raw.SetDeadline(deadline); err != nil {
		return errSMTPNetwork
	}

	implicit := m.implicitTLS()
	conn := raw
	if implicit {
		tc := tls.Client(raw, tlsCfg)
		if err := tc.HandshakeContext(ctx); err != nil {
			return smtpFailure(ctx, err, stageTLS)
		}
		conn = tc
	}

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return smtpFailure(ctx, err, stageConnect)
	}
	defer client.Close()

	if err := client.Hello(smtpHelloName); err != nil {
		return smtpFailure(ctx, err, stageConnect)
	}
	if !implicit {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errSMTPNoTLS
		}
		if err := client.StartTLS(tlsCfg); err != nil {
			return smtpFailure(ctx, err, stageTLS)
		}
	}
	// Defence in depth: nothing past this line runs on a plain connection, so
	// the password can never be sent before TLS.
	if _, ok := client.TLSConnectionState(); !ok {
		return errSMTPNoTLS
	}

	if ok, mechs := client.Extension("AUTH"); !ok || !strings.Contains(" "+strings.ToUpper(mechs)+" ", " PLAIN ") {
		// Not a refused password: no sign in was tried.
		return errSMTPNoAuth
	}
	auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	if err := client.Auth(auth); err != nil {
		return smtpFailure(ctx, err, stageAuth)
	}
	if err := client.Mail(envelopeFrom); err != nil {
		return smtpFailure(ctx, err, stageEnvelope)
	}
	if err := client.Rcpt(rcpt); err != nil {
		return smtpFailure(ctx, err, stageEnvelope)
	}
	w, err := client.Data()
	if err != nil {
		return smtpFailure(ctx, err, stageData)
	}
	if _, err := w.Write(data); err != nil {
		return smtpFailure(ctx, err, stageData)
	}
	if err := w.Close(); err != nil {
		return smtpFailure(ctx, err, stageData)
	}
	// The message is accepted; a failing QUIT changes nothing.
	_ = client.Quit()
	return nil
}

// smtpFailure maps an error from the exchange to an error that keeps its kind
// and drops its text. A cancelled or timed out context is returned as the
// context's own error, so errors.Is still finds it. A reply code is kept (a
// number the server sets); the server's words are not, because they can repeat
// the recipient address.
func smtpFailure(ctx context.Context, err error, stage smtpStage) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return context.DeadlineExceeded
	}
	var reply *textproto.Error
	if errors.As(err, &reply) {
		switch {
		case stage == stageTLS:
			// The server answered STARTTLS with something other than "go
			// ahead" (454, 502, 554 and so on). That is a refused encryption,
			// not a refused message, whatever the reply code says.
			return fmt.Errorf("%w (reply %d)", errSMTPNoTLS, reply.Code)
		case stage == stageAuth:
			return fmt.Errorf("%w (reply %d)", errSMTPAuth, reply.Code)
		case reply.Code >= 400 && reply.Code < 500:
			return fmt.Errorf("%w (reply %d)", errSMTPTemporary, reply.Code)
		default:
			return fmt.Errorf("%w (reply %d)", errSMTPPermanent, reply.Code)
		}
	}
	switch stage {
	case stageTLS:
		return errSMTPTLS
	case stageAuth:
		// No reply from the server: the connection broke in the middle of the
		// sign in. The password was not refused, so it is not errSMTPAuth.
		return errSMTPAuthLost
	}
	return errSMTPNetwork
}
