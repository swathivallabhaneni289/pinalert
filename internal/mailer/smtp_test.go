package mailer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"
	"testing"
	"time"

	"pinalert/internal/mailer/smtptest"
)

// The made up people and secrets of these tests. None of them may ever appear
// in an error string or a log line.
const (
	testSMTPUser     = "pinalert.sender@example.com"
	testSMTPPassword = "s3cretAppPassw0rd"
	testSMTPTo       = "reader.person@example.com"
	testSMTPCode     = "482913"
)

func testSMTPMailer(srv *smtptest.Server, implicit bool) *smtpMailer {
	return &smtpMailer{
		cfg: SMTPConfig{
			Host:     srv.Host,
			Port:     srv.Port,
			Username: testSMTPUser,
			Password: testSMTPPassword,
			From:     "Pinalert <" + testSMTPUser + ">",
		},
		rootCAs:          srv.Roots,
		forceImplicitTLS: implicit,
	}
}

func testSMTPEmail() AuthEmail {
	return AuthEmail{Kind: KindCodeSignup, Code: testSMTPCode, MinutesValid: 10}
}

// assertNoSecrets fails when text holds any of the secrets or the words a fake
// server sent.
func assertNoSecrets(t *testing.T, what, text string, extra ...string) {
	t.Helper()
	for _, secret := range append([]string{testSMTPPassword, testSMTPUser, testSMTPTo, testSMTPCode, "reader.person", "pinalert.sender"}, extra...) {
		if strings.Contains(text, secret) {
			t.Errorf("%s holds %q: %s", what, secret, text)
		}
	}
}

func indexOfCommand(cmds []string, want string) int {
	for i, c := range cmds {
		if c == want {
			return i
		}
	}
	return -1
}

func hasCommandPrefix(cmds []string, prefix string) bool {
	for _, c := range cmds {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func TestSMTPStartTLSHappensBeforeAuth(t *testing.T) {
	srv := smtptest.Start(t, smtptest.Options{})
	m := testSMTPMailer(srv, false)

	if err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail()); err != nil {
		t.Fatalf("send: %v", err)
	}
	cmds := srv.Commands()
	tlsAt := indexOfCommand(cmds, "plain STARTTLS")
	authAt := indexOfCommand(cmds, "tls AUTH")
	if tlsAt < 0 || authAt < 0 || tlsAt > authAt {
		t.Fatalf("STARTTLS (at %d) must come before AUTH (at %d): %v", tlsAt, authAt, cmds)
	}
	if hasCommandPrefix(cmds, "plain AUTH") {
		t.Fatalf("AUTH was sent before TLS: %v", cmds)
	}
	logins := srv.Logins()
	if len(logins) != 1 || !logins[0].OverTLS || logins[0].Username != testSMTPUser || logins[0].Password != testSMTPPassword {
		t.Fatalf("the server should see exactly one login, over TLS, with the configured credentials: %+v", logins)
	}
	// Every command after the upgrade is encrypted; the EHLO after STARTTLS
	// proves the client started the conversation again.
	if indexOfCommand(cmds, "tls EHLO localhost") < 0 {
		t.Errorf("no EHLO after STARTTLS: %v", cmds)
	}
}

func TestSMTPNoSTARTTLSGetsNoAuth(t *testing.T) {
	// The server also advertises AUTH PLAIN on the plain connection, like a
	// careless one. Go's own PlainAuth would send the password to a localhost
	// server over plain text, so this pins that our code refuses first.
	srv := smtptest.Start(t, smtptest.Options{NoSTARTTLS: true, AdvertiseAuthAlways: true})
	m := testSMTPMailer(srv, false)

	err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail())
	if !errors.Is(err, errSMTPNoTLS) {
		t.Fatalf("err = %v, want errSMTPNoTLS", err)
	}
	if hasCommandPrefix(srv.Commands(), "plain AUTH") || hasCommandPrefix(srv.Commands(), "tls AUTH") {
		t.Fatalf("an AUTH command reached a server with no STARTTLS: %v", srv.Commands())
	}
	if len(srv.Logins()) != 0 {
		t.Fatalf("a login reached the server: %+v", srv.Logins())
	}
	if hasCommandPrefix(srv.Commands(), "plain MAIL") {
		t.Fatalf("the message was started on a plain connection: %v", srv.Commands())
	}
	assertNoSecrets(t, "error", err.Error())
}

func TestSMTPUntrustedCertificateGetsNoAuth(t *testing.T) {
	srv := smtptest.Start(t, smtptest.Options{})
	m := testSMTPMailer(srv, false)
	m.rootCAs = nil // the system roots do not know the test certificate

	err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail())
	if !errors.Is(err, errSMTPTLS) {
		t.Fatalf("err = %v, want errSMTPTLS", err)
	}
	if len(srv.Logins()) != 0 || hasCommandPrefix(srv.Commands(), "tls AUTH") {
		t.Fatalf("a login reached a server whose certificate was not trusted: %v %+v", srv.Commands(), srv.Logins())
	}
	assertNoSecrets(t, "error", err.Error())
}

func TestSMTPWrongHostNameIsRefused(t *testing.T) {
	// The certificate is trusted but names only other.example.com, so the name
	// check against the configured host must fail, on STARTTLS and on implicit
	// TLS alike, before any password is sent.
	for _, implicit := range []bool{false, true} {
		srv := smtptest.Start(t, smtptest.Options{WrongName: true, ImplicitTLS: implicit})
		m := testSMTPMailer(srv, implicit)

		err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail())
		if !errors.Is(err, errSMTPTLS) {
			t.Fatalf("implicit=%v: err = %v, want errSMTPTLS", implicit, err)
		}
		if len(srv.Logins()) != 0 || hasCommandPrefix(srv.Commands(), "tls AUTH") {
			t.Fatalf("implicit=%v: a login reached the server: %v %+v", implicit, srv.Commands(), srv.Logins())
		}
		assertNoSecrets(t, "error", err.Error())
	}
}

func TestSMTPImplicitTLS(t *testing.T) {
	srv := smtptest.Start(t, smtptest.Options{ImplicitTLS: true})
	m := testSMTPMailer(srv, true)

	if err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail()); err != nil {
		t.Fatalf("send: %v", err)
	}
	cmds := srv.Commands()
	if hasCommandPrefix(cmds, "plain ") {
		t.Fatalf("a command crossed the wire in plain text: %v", cmds)
	}
	if indexOfCommand(cmds, "tls STARTTLS") >= 0 {
		t.Fatalf("STARTTLS was used on an implicit TLS connection: %v", cmds)
	}
	if logins := srv.Logins(); len(logins) != 1 || !logins[0].OverTLS {
		t.Fatalf("logins: %+v", logins)
	}
	if srv.Message() == "" {
		t.Fatal("the server received no message")
	}
}

func TestSMTPPort465MeansImplicitTLS(t *testing.T) {
	if !(&smtpMailer{cfg: SMTPConfig{Port: 465}}).implicitTLS() {
		t.Error("port 465 should start TLS at the first byte")
	}
	for _, port := range []int{587, 25, 2525} {
		if (&smtpMailer{cfg: SMTPConfig{Port: port}}).implicitTLS() {
			t.Errorf("port %d should use STARTTLS", port)
		}
	}
}

func TestSMTPMessageShape(t *testing.T) {
	srv := smtptest.Start(t, smtptest.Options{})
	m := testSMTPMailer(srv, false)
	if err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail()); err != nil {
		t.Fatalf("send: %v", err)
	}
	raw := srv.Message()
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("the message does not parse: %v", err)
	}

	from, err := msg.Header.AddressList("From")
	if err != nil || len(from) != 1 || from[0].Address != testSMTPUser || from[0].Name != "Pinalert" {
		t.Errorf("From = %v %v", from, err)
	}
	to, err := msg.Header.AddressList("To")
	if err != nil || len(to) != 1 || to[0].Address != testSMTPTo {
		t.Errorf("To = %v %v", to, err)
	}
	if got := msg.Header.Get("Subject"); got != "Your Pinalert code" {
		t.Errorf("Subject = %q", got)
	}
	if d, err := msg.Header.Date(); err != nil || time.Since(d) > time.Minute || time.Until(d) > time.Minute {
		t.Errorf("Date = %v %v", d, err)
	}
	if id := msg.Header.Get("Message-Id"); !regexp.MustCompile(`^<[0-9a-f]{32}@example\.com>$`).MatchString(id) {
		t.Errorf("Message-ID = %q, want a random id at the From domain", id)
	}
	if got := msg.Header.Get("Mime-Version"); got != "1.0" {
		t.Errorf("MIME-Version = %q", got)
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/alternative" || params["boundary"] == "" {
		t.Fatalf("Content-Type = %q %v", msg.Header.Get("Content-Type"), err)
	}

	mr := multipart.NewReader(msg.Body, params["boundary"])
	var types, bodies []string
	for {
		part, err := mr.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("part: %v", err)
		}
		if cte := part.Header.Get("Content-Transfer-Encoding"); cte != "quoted-printable" {
			t.Errorf("part encoding = %q, want quoted-printable", cte)
		}
		types = append(types, part.Header.Get("Content-Type"))
		decoded, err := io.ReadAll(quotedprintable.NewReader(part))
		if err != nil {
			t.Fatalf("decoding a part: %v", err)
		}
		bodies = append(bodies, string(decoded))
	}
	if len(types) != 2 || types[0] != "text/plain; charset=UTF-8" || types[1] != "text/html; charset=UTF-8" {
		t.Fatalf("parts = %v, want the text part and then the HTML part, both UTF-8", types)
	}
	_, wantText, wantHTML, _ := AuthEmailContent(testSMTPEmail())
	if strings.ReplaceAll(bodies[0], "\r\n", "\n") != wantText || bodies[1] != wantHTML {
		t.Errorf("the parts differ from AuthEmailContent:\n%q\n%q", bodies[0], bodies[1])
	}
	for i, b := range bodies {
		if !strings.Contains(b, testSMTPCode) {
			t.Errorf("part %d lacks the code", i)
		}
		if strings.Contains(strings.ToLower(b), "http") || strings.Contains(b, "://") {
			t.Errorf("part %d holds a link: %q", i, b)
		}
	}
	for _, line := range strings.Split(raw, "\r\n") {
		if len(line) > 998 {
			t.Errorf("a line of %d bytes is over the SMTP limit", len(line))
		}
	}
}

func TestBuildMessageEncodesNonASCIIAsUTF8(t *testing.T) {
	sender, _ := mail.ParseAddress("Pinalert <pinalert.sender@example.com>")
	data, err := buildMessage(sender, testSMTPTo, "Your Pinalert kod é", "Héllo ñ text\n", "<p>Héllo ñ</p>\n", time.Now())
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rawSubject := msg.Header.Get("Subject")
	if !strings.HasPrefix(rawSubject, "=?utf-8?q?") {
		t.Errorf("a non ASCII subject should be Q encoded, got %q", rawSubject)
	}
	decodedSubject, err := new(mime.WordDecoder).DecodeHeader(rawSubject)
	if err != nil || decodedSubject != "Your Pinalert kod é" {
		t.Errorf("decoded subject = %q %v", decodedSubject, err)
	}
	_, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	mr := multipart.NewReader(msg.Body, params["boundary"])
	part, err := mr.NextRawPart()
	if err != nil {
		t.Fatalf("part: %v", err)
	}
	rawPart, _ := io.ReadAll(part)
	if !strings.Contains(string(rawPart), "H=C3=A9llo") {
		t.Errorf("the text part is not UTF-8 quoted-printable: %q", rawPart)
	}
	for _, b := range rawPart {
		if b > 127 {
			t.Fatalf("the encoded part holds a non ASCII byte: %q", rawPart)
		}
	}
	decoded, _ := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(rawPart)))
	if string(decoded) != "Héllo ñ text\r\n" && string(decoded) != "Héllo ñ text\n" {
		t.Errorf("decoded = %q", decoded)
	}
}

func TestSMTPRefusesLineBreaksBeforeConnecting(t *testing.T) {
	srv := smtptest.Start(t, smtptest.Options{})
	m := testSMTPMailer(srv, false)

	for _, to := range []string{
		testSMTPTo + "\r\nBcc: victim@example.com",
		testSMTPTo + "\nBcc: victim@example.com",
		"Reader <" + testSMTPTo + ">",
		"<" + testSMTPTo + ">",
		testSMTPTo + ", other@example.com",
		"",
		"not an address",
	} {
		err := m.SendAuthEmail(context.Background(), to, testSMTPEmail())
		if !errors.Is(err, errBadRecipient) {
			t.Errorf("to %q: err = %v, want errBadRecipient", to, err)
		}
	}

	sender, _ := mail.ParseAddress("Pinalert <pinalert.sender@example.com>")
	for _, subject := range []string{"Hi\r\nBcc: x@example.com", "Hi\nthere", "Hi\rthere"} {
		if _, err := buildMessage(sender, testSMTPTo, subject, "t", "h", time.Now()); !errors.Is(err, errBadHeader) {
			t.Errorf("subject %q: err = %v, want errBadHeader", subject, err)
		}
	}
	if _, err := buildMessage(sender, testSMTPTo+"\r\nBcc: x@example.com", "Hi", "t", "h", time.Now()); !errors.Is(err, errBadHeader) {
		t.Errorf("to with CRLF: err = %v, want errBadHeader", err)
	}

	bad := testSMTPMailer(srv, false)
	bad.cfg.From = "Pinalert <pinalert.sender@example.com>\r\nBcc: victim@example.com"
	if err := bad.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail()); !errors.Is(err, errBadSender) {
		t.Errorf("from with CRLF: err = %v, want errBadSender", err)
	}

	if n := srv.Connections(); n != 0 {
		t.Errorf("%d connections were made; every refusal must happen before connecting", n)
	}
}

func TestSMTPFailureKinds(t *testing.T) {
	// The server's words repeat the address and the user name, as real servers
	// do. The error must keep the kind and the reply code and drop the words.
	words := " 5.1.1 The email account " + testSMTPTo + " for " + testSMTPUser + " cannot be found SERVERWORDS"
	cases := []struct {
		name string
		opts smtptest.Options
		kind error
		code string
	}{
		{"535 maps to the auth kind", smtptest.Options{AuthReply: "535" + words}, errSMTPAuth, "(reply 535)"},
		{"454 at sign in is still the auth kind", smtptest.Options{AuthReply: "454" + words}, errSMTPAuth, "(reply 454)"},
		{"4xx at the recipient maps to temporary", smtptest.Options{RcptReply: "450" + words}, errSMTPTemporary, "(reply 450)"},
		{"4xx at the message maps to temporary", smtptest.Options{DataReply: "452" + words}, errSMTPTemporary, "(reply 452)"},
		{"5xx at the recipient maps to permanent", smtptest.Options{RcptReply: "550" + words}, errSMTPPermanent, "(reply 550)"},
		{"5xx at the sender maps to permanent", smtptest.Options{MailReply: "553" + words}, errSMTPPermanent, "(reply 553)"},
		{"5xx at the message maps to permanent", smtptest.Options{DataReply: "554" + words}, errSMTPPermanent, "(reply 554)"},
		{"a refusing greeting is permanent", smtptest.Options{Greeting: "554" + words}, errSMTPPermanent, "(reply 554)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logged bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&logged)
			defer log.SetOutput(previous)
			srv := smtptest.Start(t, tc.opts)
			m := testSMTPMailer(srv, false)

			err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail())
			if !errors.Is(err, tc.kind) {
				t.Fatalf("err = %v, want kind %v", err, tc.kind)
			}
			if !strings.Contains(err.Error(), tc.code) {
				t.Errorf("err = %q, want the reply code %s", err, tc.code)
			}
			assertNoSecrets(t, "error", err.Error(), "SERVERWORDS", "cannot be found")
			assertNoSecrets(t, "log", logged.String(), "SERVERWORDS")
		})
	}
}

func TestSMTPUnreachableServerIsANetworkFailure(t *testing.T) {
	srv := smtptest.Start(t, smtptest.Options{})
	m := testSMTPMailer(srv, false)
	// Dial a port nobody listens on: port 1 is refused on a loopback interface.
	m.cfg.Port = 1

	err := m.SendAuthEmail(context.Background(), testSMTPTo, testSMTPEmail())
	if !errors.Is(err, errSMTPNetwork) {
		t.Fatalf("err = %v, want errSMTPNetwork", err)
	}
	assertNoSecrets(t, "error", err.Error())
	if strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("the error names the server address: %q", err)
	}
}

func TestSMTPHungServerIsCutAtTheDeadline(t *testing.T) {
	srv := smtptest.Start(t, smtptest.Options{Hang: true})
	m := testSMTPMailer(srv, false)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := m.SendAuthEmail(ctx, testSMTPTo, testSMTPEmail())
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("the send took %v, want it cut near the 300ms deadline", elapsed)
	}
	assertNoSecrets(t, "error", err.Error())
}

func TestSMTPCancelledContextStopsTheSend(t *testing.T) {
	srv := smtptest.Start(t, smtptest.Options{Hang: true})
	m := testSMTPMailer(srv, false)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	err := m.SendAuthEmail(ctx, testSMTPTo, testSMTPEmail())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestSMTPBadAuthEmailIsRefusedBeforeConnecting(t *testing.T) {
	srv := smtptest.Start(t, smtptest.Options{})
	m := testSMTPMailer(srv, false)
	err := m.SendAuthEmail(context.Background(), testSMTPTo, AuthEmail{Kind: KindCodeSignup, Code: "12", MinutesValid: 10})
	if !errors.Is(err, ErrBadAuthEmail) {
		t.Fatalf("err = %v, want ErrBadAuthEmail", err)
	}
	if srv.Connections() != 0 {
		t.Errorf("a bad email opened %d connections", srv.Connections())
	}
}

func TestSMTPCodelessKindsSend(t *testing.T) {
	for _, kind := range []AuthEmailKind{KindAccountExists, KindNoAccount, KindCodeReset} {
		srv := smtptest.Start(t, smtptest.Options{})
		m := testSMTPMailer(srv, false)
		msg := AuthEmail{Kind: kind, Code: testSMTPCode, MinutesValid: 10}
		if kind != KindCodeReset {
			msg.Code, msg.MinutesValid = "", 0
		}
		if err := m.SendAuthEmail(context.Background(), testSMTPTo, msg); err != nil {
			t.Errorf("kind %s: %v", kind, err)
		}
	}
}
