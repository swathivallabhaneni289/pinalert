// ip_backstop_test.go is V-10 for plan 04-09: it proves the network address
// backstop over the real router and a real Postgres. The backstop must throttle
// one client address across accounts when a deploy setting names the header,
// and must skip itself when the address is missing, unparseable or not
// configured. It never falls back to the connection's peer address and never
// shares one bucket.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// p09BackstopHeader is the header the tests name as the client address header.
const p09BackstopHeader = "X-Test-Client-IP"

// p09BackstopVote POSTs a confirm to a missing report (so the handler answers
// 404 whenever the gate admits) with the given extra headers.
func p09BackstopVote(t *testing.T, client *http.Client, srvURL string, headers map[string]string) (int, []byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srvURL+"/api/reports/999999/confirm",
		bytes.NewReader([]byte(`{"latitude":12.9716,"longitude":77.5946}`)))
	if err != nil {
		t.Fatalf("building vote request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST vote: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading vote response: %v", err)
	}
	return resp.StatusCode, body, resp.Header
}

// p09BackstopAccountID reads the verified account id behind the client session.
func p09BackstopAccountID(t *testing.T, pool *pgxpool.Pool, client *http.Client, srvURL string) int64 {
	t.Helper()
	sid := gateSessionIDFromJar(t, client, srvURL)
	var id int64
	if err := pool.QueryRow(context.Background(), `SELECT account_id FROM sessions WHERE session_id = $1`, sid).Scan(&id); err != nil {
		t.Fatalf("reading account id for session: %v", err)
	}
	return id
}

// p09BackstopLogSink collects the standard logger's output.
type p09BackstopLogSink struct {
	mu sync.Mutex
	sb strings.Builder
}

func (s *p09BackstopLogSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sb.Write(p)
}

func (s *p09BackstopLogSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sb.String()
}

func TestIPBackstop(t *testing.T) {
	tweakFor := func(header string) func(d *Deps, q *sqlcgen.Queries) {
		return func(d *Deps, q *sqlcgen.Queries) {
			d.Votes = service.NewVotingService(q)
			d.ClientIPHeader = header
			d.RateLimits.IPBackstop = BucketLimit{Every: time.Hour, Burst: 2}
			d.RequestLinkRateLimit = RequestLinkRateLimit{Burst: 100, Every: time.Millisecond}
		}
	}

	t.Run("shared_address_across_accounts", func(t *testing.T) {
		srv, pool, mailer := newGateTestServerWith(t, tweakFor(p09BackstopHeader))
		a := newGateJarClient(t)
		b := newGateJarClient(t)
		const emailA = "p09-backstop-a@example.com"
		gateVerifySession(t, a, srv, mailer, emailA)
		gateVerifySession(t, b, srv, mailer, "p09-backstop-b@example.com")
		aID := p09BackstopAccountID(t, pool, a, srv.URL)

		sink := &p09BackstopLogSink{}
		prev := log.Writer()
		log.SetOutput(sink)
		t.Cleanup(func() { log.SetOutput(prev) })

		same := map[string]string{p09BackstopHeader: "203.0.113.7"}
		if code, body, _ := p09BackstopVote(t, a, srv.URL, same); code != http.StatusNotFound {
			t.Fatalf("A first vote = %d, want 404, body %s", code, body)
		}
		if code, body, _ := p09BackstopVote(t, b, srv.URL, same); code != http.StatusNotFound {
			t.Fatalf("B first vote = %d, want 404, body %s", code, body)
		}
		code, body, hdr := p09BackstopVote(t, a, srv.URL, same)
		if code != http.StatusTooManyRequests {
			t.Fatalf("A third vote from the shared address = %d, want 429, body %s", code, body)
		}
		n, err := strconv.Atoi(hdr.Get("Retry-After"))
		if err != nil || n < 1 {
			t.Fatalf("Retry-After = %q, want an integer of at least 1", hdr.Get("Retry-After"))
		}
		var parsed struct {
			Error struct {
				Field   string `json:"field"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("decoding 429 body %s: %v", body, err)
		}
		if parsed.Error.Field != "rate_vote" {
			t.Errorf("field = %q, want rate_vote", parsed.Error.Field)
		}
		if want := service.VoteLimitMessage(time.Duration(n) * time.Second); parsed.Error.Message != want {
			t.Errorf("message = %q, want %q", parsed.Error.Message, want)
		}

		other := map[string]string{p09BackstopHeader: "203.0.113.8"}
		if code, body, _ := p09BackstopVote(t, b, srv.URL, other); code != http.StatusNotFound {
			t.Errorf("B vote from another address = %d, want 404, body %s", code, body)
		}

		logged := sink.String()
		if want := "bucket=ip_backstop account=" + strconv.FormatInt(aID, 10); !strings.Contains(logged, want) {
			t.Errorf("log lacks %q, got %q", want, logged)
		}
		for _, line := range strings.Split(logged, "\n") {
			if !strings.Contains(line, "bucket=") {
				continue
			}
			if strings.Contains(line, "203.0.113.7") || strings.Contains(line, emailA) {
				t.Errorf("block line %q holds a network address or an email", line)
			}
		}
	})

	skips := []struct {
		name    string
		header  string
		headers map[string]string
	}{
		{"header_missing_skips_backstop", p09BackstopHeader, nil},
		{"header_unparseable_skips_backstop", p09BackstopHeader, map[string]string{p09BackstopHeader: "not an address"}},
		{"setting_unset_skips_backstop", "", map[string]string{p09BackstopHeader: "203.0.113.7", "X-Forwarded-For": "203.0.113.7"}},
	}
	for _, tc := range skips {
		t.Run(tc.name, func(t *testing.T) {
			srv, _, mailer := newGateTestServerWith(t, tweakFor(tc.header))
			c := newGateJarClient(t)
			gateVerifySession(t, c, srv, mailer, "p09-backstop-skip@example.com")
			for i := 0; i < 5; i++ {
				if code, body, _ := p09BackstopVote(t, c, srv.URL, tc.headers); code != http.StatusNotFound {
					t.Fatalf("vote %d = %d, want 404 (the backstop must be skipped), body %s", i+1, code, body)
				}
			}
		})
	}
}
