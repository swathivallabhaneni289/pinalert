// Package smtptest is a small in process SMTP server for tests. It listens on
// a random port of 127.0.0.1, presents a certificate made on the spot, and
// records the conversation, so a test can check that the client encrypted the
// connection before it sent a password. It never reaches any other host, and
// it is only for tests: nothing outside a _test.go file imports it, and
// TestTestOnlyHooksStayOutOfProductionCode in internal/mailer fails if one does.
package smtptest

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// Options shape how the fake server behaves. The zero value is a well behaved
// server: it offers STARTTLS, advertises AUTH PLAIN only after TLS, and says
// yes to everything.
type Options struct {
	// ImplicitTLS starts TLS at the first byte, like port 465.
	ImplicitTLS bool
	// NoSTARTTLS leaves STARTTLS out of the EHLO answer and refuses the command.
	NoSTARTTLS bool
	// AdvertiseAuthAlways lists AUTH PLAIN in the EHLO answer before TLS too,
	// like a careless server, to prove that the client does not use it.
	AdvertiseAuthAlways bool
	// AuthMechanisms replaces the mechanisms listed after AUTH once the
	// connection is encrypted ("PLAIN LOGIN" by default). "LOGIN" alone is a
	// server that offers no PLAIN sign in.
	AuthMechanisms string
	// DropAtAuth closes the connection, with no answer, when the AUTH command
	// arrives.
	DropAtAuth bool
	// STARTTLSReply makes the server advertise STARTTLS and then answer the
	// command with this line (such as "454 4.7.0 TLS not available") instead of
	// "220 Ready to start TLS", and stay on the plain connection.
	STARTTLSReply string
	// Greeting replaces the "220 ..." greeting line.
	Greeting string
	// AuthReply, MailReply, RcptReply and DataReply replace the answer to that
	// step. Each is a full reply line such as "535 5.7.8 bad credentials".
	AuthReply string
	MailReply string
	RcptReply string
	DataReply string
	// WrongName makes the certificate name only other.example.com, so a client
	// that checks the host name must refuse it.
	WrongName bool
	// Hang accepts the connection and then says nothing at all.
	Hang bool
}

// Login is one AUTH PLAIN the server received.
type Login struct {
	Username string
	Password string
	OverTLS  bool
}

// Server is a running fake SMTP server.
type Server struct {
	// Host and Port are where the client dials. Host is 127.0.0.1.
	Host string
	Port int
	// Roots trusts the server's certificate.
	Roots *x509.CertPool

	opts Options
	ln   net.Listener
	cert tls.Certificate

	mu       sync.Mutex
	conns    int
	commands []string
	logins   []Login
	message  string
	open     []net.Conn
	wg       sync.WaitGroup
}

// Start runs a fake server until the test ends.
func Start(tb testing.TB, opts Options) *Server {
	tb.Helper()
	cert, roots := makeCert(tb, opts.WrongName)
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("smtptest: listen: %v", err)
	}
	var ln net.Listener = inner
	if opts.ImplicitTLS {
		ln = tls.NewListener(inner, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	}
	addr := inner.Addr().(*net.TCPAddr)
	s := &Server{Host: "127.0.0.1", Port: addr.Port, Roots: roots, opts: opts, ln: ln, cert: cert}
	s.wg.Add(1)
	go s.acceptLoop()
	tb.Cleanup(func() {
		ln.Close()
		s.mu.Lock()
		for _, c := range s.open {
			c.Close()
		}
		s.mu.Unlock()
		s.wg.Wait()
	})
	return s
}

// Connections is how many connections the server accepted.
func (s *Server) Connections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conns
}

// Commands lists every command line the client sent, in order, each prefixed
// with "plain " or "tls " for the state of the connection when it arrived. The
// arguments of AUTH are left out.
func (s *Server) Commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commands...)
}

// Logins lists the AUTH PLAIN credentials received.
func (s *Server) Logins() []Login {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Login(nil), s.logins...)
}

// Message is the last message received in DATA, with dot stuffing removed.
func (s *Server) Message() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.message
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns++
		s.open = append(s.open, c)
		s.mu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer c.Close()
			s.serve(c)
		}()
	}
}

func (s *Server) record(overTLS bool, line string) {
	state := "plain "
	if overTLS {
		state = "tls "
	}
	s.mu.Lock()
	s.commands = append(s.commands, state+line)
	s.mu.Unlock()
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func (s *Server) serve(raw net.Conn) {
	conn := raw
	overTLS := s.opts.ImplicitTLS
	r := bufio.NewReader(conn)
	write := func(lines ...string) bool {
		for _, l := range lines {
			if _, err := conn.Write([]byte(l + "\r\n")); err != nil {
				return false
			}
		}
		return true
	}
	if s.opts.Hang {
		buf := make([]byte, 64)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
		}
	}
	if !write(orDefault(s.opts.Greeting, "220 localhost ESMTP ready")) {
		return
	}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		verb := strings.ToUpper(line)
		if i := strings.IndexByte(verb, ' '); i >= 0 {
			verb = verb[:i]
		}
		if verb == "AUTH" {
			s.record(overTLS, "AUTH")
		} else {
			s.record(overTLS, line)
		}
		switch verb {
		case "EHLO", "HELO":
			lines := []string{"250-localhost", "250-8BITMIME"}
			if !s.opts.NoSTARTTLS && !overTLS {
				lines = append(lines, "250-STARTTLS")
			}
			if overTLS || s.opts.AdvertiseAuthAlways {
				lines = append(lines, "250-AUTH "+orDefault(s.opts.AuthMechanisms, "PLAIN LOGIN"))
			}
			lines = append(lines, "250 SIZE 35882577")
			if !write(lines...) {
				return
			}
		case "STARTTLS":
			if s.opts.NoSTARTTLS || overTLS {
				if !write("502 5.5.1 STARTTLS not available") {
					return
				}
				continue
			}
			if s.opts.STARTTLSReply != "" {
				if !write(s.opts.STARTTLSReply) {
					return
				}
				continue
			}
			if !write("220 2.0.0 Ready to start TLS") {
				return
			}
			tc := tls.Server(raw, &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS12})
			if err := tc.Handshake(); err != nil {
				return
			}
			conn = tc
			r = bufio.NewReader(conn)
			overTLS = true
		case "AUTH":
			if s.opts.DropAtAuth {
				return // closes the connection with no answer
			}
			fields := strings.Fields(line)
			if len(fields) >= 3 && strings.EqualFold(fields[1], "PLAIN") {
				if dec, err := base64.StdEncoding.DecodeString(fields[2]); err == nil {
					parts := strings.Split(string(dec), "\x00")
					if len(parts) == 3 {
						s.mu.Lock()
						s.logins = append(s.logins, Login{Username: parts[1], Password: parts[2], OverTLS: overTLS})
						s.mu.Unlock()
					}
				}
			}
			if !write(orDefault(s.opts.AuthReply, "235 2.7.0 Accepted")) {
				return
			}
		case "MAIL":
			if !write(orDefault(s.opts.MailReply, "250 2.1.0 OK")) {
				return
			}
		case "RCPT":
			if !write(orDefault(s.opts.RcptReply, "250 2.1.5 OK")) {
				return
			}
		case "DATA":
			if !write("354 Go ahead") {
				return
			}
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				l = strings.TrimRight(l, "\r\n")
				if l == "." {
					break
				}
				if strings.HasPrefix(l, "..") {
					l = l[1:]
				}
				b.WriteString(l + "\r\n")
			}
			s.mu.Lock()
			s.message = b.String()
			s.mu.Unlock()
			if !write(orDefault(s.opts.DataReply, "250 2.0.0 OK queued")) {
				return
			}
		case "QUIT":
			write("221 2.0.0 Bye")
			return
		default:
			if !write("502 5.5.2 Command not recognized") {
				return
			}
		}
	}
}

// makeCert makes a self signed certificate for 127.0.0.1 and localhost, valid
// for an hour, and a pool that trusts it.
func makeCert(tb testing.TB, wrongName bool) (tls.Certificate, *x509.CertPool) {
	tb.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatalf("smtptest: key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "smtptest"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	if wrongName {
		tmpl.DNSNames = []string{"other.example.com"}
	} else {
		tmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		tmpl.DNSNames = []string{"localhost"}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		tb.Fatalf("smtptest: certificate: %v", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		tb.Fatalf("smtptest: parse certificate: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}
