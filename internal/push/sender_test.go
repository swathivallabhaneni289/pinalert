package push

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

const testEndpoint = "https://fcm.googleapis.com/fcm/send/SECRETTOKEN-abc123"

// trackedBody records whether it was drained and closed.
type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

// testKeys makes a throwaway VAPID pair in memory. The values are never
// written anywhere.
func testKeys(t *testing.T, subject string) Config {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	return Config{PublicKey: pub, PrivateKey: priv, Subject: subject}
}

func testSubscription(t *testing.T) Subscription {
	t.Helper()
	pub, auth := browserKeys(t)
	return Subscription{
		Endpoint: testEndpoint,
		P256dh:   base64.RawURLEncoding.EncodeToString(pub),
		Auth:     base64.RawURLEncoding.EncodeToString(auth),
	}
}

func testMessage() Message {
	return Message{
		Payload: Payload{V: PayloadVersion, Kind: "critical", Tag: "report-7", Title: "Water rising near Indiranagar", Body: "About 3 km from your home area. Not confirmed yet.", URL: "/?report=7"},
		TTL:     30 * time.Minute,
		Urgency: UrgencyHigh,
		Topic:   "r7",
	}
}

// recorder is a fake HTTP client that keeps the last request it saw.
type recorder struct {
	req    *http.Request
	body   []byte
	status int
	header http.Header
	resp   *trackedBody
	calls  int
}

func (r *recorder) client() *http.Client {
	return &http.Client{Transport: rtFunc(func(req *http.Request) (*http.Response, error) {
		r.calls++
		r.req = req
		r.body, _ = io.ReadAll(req.Body)
		r.resp = &trackedBody{Reader: strings.NewReader(strings.Repeat("x", 100))}
		status := r.status
		if status == 0 {
			status = http.StatusCreated
		}
		h := r.header
		if h == nil {
			h = http.Header{}
		}
		return &http.Response{StatusCode: status, Header: h, Body: r.resp, Request: req}, nil
	})}
}

func jwtClaims(t *testing.T, authz string) map[string]any {
	t.Helper()
	if !strings.HasPrefix(authz, "vapid t=") {
		t.Fatalf("Authorization %q does not start with vapid t=", authz)
	}
	token := strings.TrimPrefix(authz, "vapid t=")
	token, _, _ = strings.Cut(token, ", k=")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	return claims
}

func TestSenderHeaders(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		wantSub string
		ttl     time.Duration
		wantTTL string
		urgency Urgency
		wantUrg string
	}{
		{name: "bare address", subject: "ops@example.com", wantSub: "mailto:ops@example.com", ttl: 30 * time.Minute, wantTTL: "1800", urgency: UrgencyHigh, wantUrg: "high"},
		{name: "https url", subject: "https://pinalert.example", wantSub: "https://pinalert.example", ttl: time.Hour, wantTTL: "3600", urgency: UrgencyNormal, wantUrg: "normal"},
		{name: "whole seconds only", subject: "ops@example.com", wantSub: "mailto:ops@example.com", ttl: 90*time.Second + 900*time.Millisecond, wantTTL: "90", urgency: UrgencyNormal, wantUrg: "normal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			sender := NewWebPushSender(testKeys(t, tc.subject), rec.client(), DefaultEndpointPolicy())
			msg := testMessage()
			msg.TTL = tc.ttl
			msg.Urgency = tc.urgency
			res, err := sender.Send(context.Background(), testSubscription(t), msg)
			if err != nil {
				t.Fatalf("Send error = %v", err)
			}
			if res.Status != http.StatusCreated {
				t.Fatalf("status = %d, want 201", res.Status)
			}
			if rec.req.Method != http.MethodPost {
				t.Fatalf("method = %s", rec.req.Method)
			}
			if rec.req.URL.String() != testEndpoint {
				t.Fatalf("request went to %s", rec.req.URL)
			}
			h := rec.req.Header
			if h.Get("TTL") != tc.wantTTL {
				t.Fatalf("TTL header = %q, want %q", h.Get("TTL"), tc.wantTTL)
			}
			if h.Get("Urgency") != tc.wantUrg {
				t.Fatalf("Urgency header = %q, want %q", h.Get("Urgency"), tc.wantUrg)
			}
			if h.Get("Topic") != "r7" {
				t.Fatalf("Topic header = %q", h.Get("Topic"))
			}
			if h.Get("Content-Encoding") != "aes128gcm" {
				t.Fatalf("Content-Encoding = %q", h.Get("Content-Encoding"))
			}
			if len(rec.body) == 0 || len(rec.body) > 4096 {
				t.Fatalf("body length = %d, want 1 to 4096", len(rec.body))
			}
			claims := jwtClaims(t, h.Get("Authorization"))
			if claims["aud"] != "https://fcm.googleapis.com" {
				t.Fatalf("aud = %v", claims["aud"])
			}
			if claims["sub"] != tc.wantSub {
				t.Fatalf("sub = %v, want %s", claims["sub"], tc.wantSub)
			}
			if strings.Contains(claims["sub"].(string), "mailto:mailto:") {
				t.Fatal("doubled mailto prefix")
			}
			exp := time.Unix(int64(claims["exp"].(float64)), 0)
			if !exp.After(time.Now()) || exp.After(time.Now().Add(24*time.Hour)) {
				t.Fatalf("exp = %v, want within 24 hours", exp)
			}
		})
	}
}

func TestSenderRefusesZeroTTL(t *testing.T) {
	cases := []struct {
		name string
		ttl  time.Duration
	}{
		{"zero", 0},
		{"negative", -time.Second},
		{"under a second", 999 * time.Millisecond},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			sender := NewWebPushSender(testKeys(t, "ops@example.com"), rec.client(), DefaultEndpointPolicy())
			msg := testMessage()
			msg.TTL = tc.ttl
			if _, err := sender.Send(context.Background(), testSubscription(t), msg); !errors.Is(err, ErrNoTTL) {
				t.Fatalf("error = %v, want ErrNoTTL", err)
			}
			if rec.calls != 0 {
				t.Fatal("a request was made")
			}
		})
	}
	t.Run("bad topic", func(t *testing.T) {
		for _, topic := range []string{"", "r 1", strings.Repeat("a", 33)} {
			rec := &recorder{}
			sender := NewWebPushSender(testKeys(t, "ops@example.com"), rec.client(), DefaultEndpointPolicy())
			msg := testMessage()
			msg.Topic = topic
			if _, err := sender.Send(context.Background(), testSubscription(t), msg); !errors.Is(err, ErrBadTopic) {
				t.Fatalf("topic %q error = %v, want ErrBadTopic", topic, err)
			}
			if rec.calls != 0 {
				t.Fatal("a request was made")
			}
		}
	})
}

func TestSenderRechecksEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		wantErr  error
	}{
		{"unknown host", "https://push.example.org/abc", ErrUnsupportedService},
		{"loopback literal", "https://127.0.0.1/abc", ErrBadEndpoint},
		{"plain http", "http://fcm.googleapis.com/abc", ErrBadEndpoint},
		{"lookalike", "https://evil-notify.windows.com/abc", ErrUnsupportedService},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			sender := NewWebPushSender(testKeys(t, "ops@example.com"), rec.client(), DefaultEndpointPolicy())
			sub := testSubscription(t)
			sub.Endpoint = tc.endpoint
			if _, err := sender.Send(context.Background(), sub, testMessage()); !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if rec.calls != 0 {
				t.Fatal("a request was made")
			}
		})
	}
	t.Run("bad keys", func(t *testing.T) {
		rec := &recorder{}
		sender := NewWebPushSender(testKeys(t, "ops@example.com"), rec.client(), DefaultEndpointPolicy())
		sub := testSubscription(t)
		sub.P256dh = "short"
		if _, err := sender.Send(context.Background(), sub, testMessage()); !errors.Is(err, ErrBadKeys) {
			t.Fatalf("error = %v, want ErrBadKeys", err)
		}
		if rec.calls != 0 {
			t.Fatal("a request was made")
		}
	})
}

func TestSenderRetryAfter(t *testing.T) {
	future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	cases := []struct {
		name   string
		status int
		header string
		want   time.Duration
		slack  time.Duration
	}{
		{name: "seconds", status: 429, header: "120", want: 2 * time.Minute},
		{name: "capped", status: 429, header: "99999", want: MaxRetryAfter},
		{name: "none", status: 201, header: "", want: 0},
		{name: "garbage", status: 429, header: "soon", want: 0},
		{name: "negative", status: 429, header: "-5", want: 0},
		{name: "http date", status: 503, header: future, want: 90 * time.Second, slack: 5 * time.Second},
		{name: "past date", status: 503, header: time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{status: tc.status, header: http.Header{}}
			if tc.header != "" {
				rec.header.Set("Retry-After", tc.header)
			}
			sender := NewWebPushSender(testKeys(t, "ops@example.com"), rec.client(), DefaultEndpointPolicy())
			res, err := sender.Send(context.Background(), testSubscription(t), testMessage())
			if err != nil {
				t.Fatalf("Send error = %v", err)
			}
			if res.Status != tc.status {
				t.Fatalf("status = %d, want %d", res.Status, tc.status)
			}
			diff := res.RetryAfter - tc.want
			if diff < 0 {
				diff = -diff
			}
			if diff > tc.slack {
				t.Fatalf("RetryAfter = %v, want %v", res.RetryAfter, tc.want)
			}
			if !rec.resp.closed {
				t.Fatal("response body was not closed")
			}
		})
	}
}

func TestSenderNeverLogsEndpoint(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	sub := testSubscription(t)
	msg := testMessage()
	secrets := []string{"SECRETTOKEN", "abc123", sub.P256dh, sub.Auth, msg.Payload.Title, msg.Payload.Body}

	rec := &recorder{status: 429, header: http.Header{"Retry-After": []string{"30"}}}
	sender := NewWebPushSender(testKeys(t, "ops@example.com"), rec.client(), DefaultEndpointPolicy())
	_, _ = sender.Send(context.Background(), sub, msg)

	failing := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("boom") })}
	sender = NewWebPushSender(testKeys(t, "ops@example.com"), failing, DefaultEndpointPolicy())
	_, err := sender.Send(context.Background(), sub, msg)
	if err == nil {
		t.Fatal("expected a transport error")
	}

	bad := sub
	bad.Endpoint = "https://push.example.org/SECRETTOKEN-abc123"
	_, err2 := sender.Send(context.Background(), bad, msg)

	out := buf.String() + "\n" + err.Error() + "\n" + err2.Error()
	for _, s := range secrets {
		if strings.Contains(out, s) {
			t.Fatalf("log or error output contains a secret %q: %s", s, out)
		}
	}
}

func TestSenderScrubsTransportError(t *testing.T) {
	sentinel := errors.New("sentinel transport failure")
	client := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) { return nil, sentinel })}
	sender := NewWebPushSender(testKeys(t, "ops@example.com"), client, DefaultEndpointPolicy())
	_, err := sender.Send(context.Background(), testSubscription(t), testMessage())
	if err == nil {
		t.Fatal("expected an error")
	}
	text := err.Error()
	for _, leak := range []string{testEndpoint, "/fcm/send", "SECRETTOKEN", "abc123"} {
		if strings.Contains(text, leak) {
			t.Fatalf("error text %q contains %q", text, leak)
		}
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		t.Fatal("errors.As still reaches a *url.Error carrying the endpoint")
	}
	if !errors.Is(err, sentinel) {
		t.Fatal("errors.Is lost the sentinel")
	}
	if !strings.Contains(text, "fcm.googleapis.com") {
		t.Fatalf("error text %q should name the host", text)
	}
}

func TestScrubURLError(t *testing.T) {
	inner := errors.New("inner")
	if got := scrubURLError(&url.Error{Op: "Post", URL: "https://x.example/secret", Err: inner}); got != inner {
		t.Fatalf("got %v, want the inner error", got)
	}
	plain := errors.New("plain")
	if got := scrubURLError(plain); got != plain {
		t.Fatalf("got %v, want the same error", got)
	}
}
