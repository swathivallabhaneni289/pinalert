package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"golang.org/x/time/rate"
)

// fakeProvider is an httptest upstream that records what one client sent.
type fakeProvider struct {
	srv  *httptest.Server
	hits atomic.Int32

	mu       sync.Mutex
	method   string
	auth     string
	ctype    string
	rawQuery string
	cookies  string
	body     []byte
}

func newFakeProvider(t *testing.T, handler http.HandlerFunc) *fakeProvider {
	t.Helper()
	f := &fakeProvider{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		raw, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.method = r.Method
		f.auth = r.Header.Get("Authorization")
		f.ctype = r.Header.Get("Content-Type")
		f.rawQuery = r.URL.RawQuery
		f.cookies = r.Header.Get("Cookie")
		f.body = raw
		f.mu.Unlock()
		if handler != nil {
			handler(w, r)
			return
		}
		writeOK(w)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeProvider) url() string { return f.srv.URL + "/v1/moderations" }

func (f *fakeProvider) lastBody() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.body...)
}

var thirteenCategories = []string{
	"harassment", "harassment/threatening", "hate", "hate/threatening",
	"illicit", "illicit/violent", "self-harm", "self-harm/instructions",
	"self-harm/intent", "sexual", "sexual/minors", "violence", "violence/graphic",
}

func okBody() string {
	scores := map[string]float64{}
	for i, k := range thirteenCategories {
		scores[k] = float64(i+1) / 100
	}
	raw, _ := json.Marshal(map[string]any{
		"id":    "modr-1",
		"model": DefaultModel,
		"results": []any{map[string]any{
			"flagged":         true,
			"categories":      map[string]bool{"hate": true},
			"category_scores": scores,
		}},
	})
	return string(raw)
}

func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, okBody())
}

func unlimited() *rate.Limiter { return rate.NewLimiter(rate.Inf, 0) }

func testClient(baseURL string, timeout time.Duration) *Client {
	return newClientWithBaseURL(baseURL, "test-key", DefaultModel, timeout, unlimited(), unlimited(), time.Now)
}

func asCallError(t *testing.T, err error) *CallError {
	t.Helper()
	var ce *CallError
	if !errors.As(err, &ce) {
		t.Fatalf("error %v is not a *CallError", err)
	}
	return ce
}

func TestClientRequestShape(t *testing.T) {
	f := newFakeProvider(t, nil)
	c := testClient(f.url(), time.Second)

	res, err := c.Screen(context.Background(), "Flood on MG Road")
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if f.hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", f.hits.Load())
	}
	f.mu.Lock()
	method, auth, ctype, rawQuery, cookies := f.method, f.auth, f.ctype, f.rawQuery, f.cookies
	f.mu.Unlock()
	if method != http.MethodPost {
		t.Errorf("method = %q", method)
	}
	if ctype != "application/json" {
		t.Errorf("content type = %q", ctype)
	}
	if auth != "Bearer test-key" {
		t.Errorf("authorization = %q", auth)
	}
	if rawQuery != "" {
		t.Errorf("query string = %q, want none", rawQuery)
	}
	if cookies != "" {
		t.Errorf("cookie header = %q, want none", cookies)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(f.lastBody(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"input", "model"}) {
		t.Fatalf("body keys = %v, want [input model]", keys)
	}
	var model, input string
	_ = json.Unmarshal(body["model"], &model)
	_ = json.Unmarshal(body["input"], &input)
	if model != DefaultModel || input != "Flood on MG Road" {
		t.Errorf("model %q input %q", model, input)
	}

	if res.Model != DefaultModel {
		t.Errorf("result model = %q", res.Model)
	}
	if len(res.Scores) != 13 {
		t.Fatalf("scores has %d keys, want 13", len(res.Scores))
	}
	if res.Scores["hate"] != 0.03 {
		t.Errorf("hate score = %v", res.Scores["hate"])
	}

	rt := reflect.TypeOf(Result{})
	if rt.NumField() != 2 || rt.Field(0).Name != "Model" || rt.Field(1).Name != "Scores" {
		t.Errorf("Result must have exactly the fields Model and Scores")
	}
}

func TestClientSendsOnlyTrimmedText(t *testing.T) {
	f := newFakeProvider(t, nil)
	c := testClient(f.url(), time.Second)

	input := func() string {
		var body struct {
			Input string `json:"input"`
		}
		if err := json.Unmarshal(f.lastBody(), &body); err != nil {
			t.Fatalf("body: %v", err)
		}
		return body.Input
	}

	if _, err := c.Screen(context.Background(), "  \n Flood on MG Road \t "); err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if got := input(); got != "Flood on MG Road" {
		t.Errorf("input = %q, want trimmed text", got)
	}

	long := strings.Repeat("తెలుగు ", 400)
	if utf8.RuneCountInString(strings.TrimSpace(long)) < 1200 {
		t.Fatalf("test text too short")
	}
	if _, err := c.Screen(context.Background(), long); err != nil {
		t.Fatalf("Screen long: %v", err)
	}
	got := input()
	if !utf8.ValidString(got) {
		t.Fatalf("cut text is not valid UTF-8")
	}
	want := string([]rune(strings.TrimSpace(long))[:1000])
	if got != want {
		t.Errorf("cut text has %d runes, want the first 1000", utf8.RuneCountInString(got))
	}

	before := f.hits.Load()
	_, err := c.Screen(context.Background(), " \n\t ")
	ce := asCallError(t, err)
	if ce.Transient || ce.Quota {
		t.Errorf("empty input must be permanent: %+v", ce)
	}
	if f.hits.Load() != before {
		t.Errorf("empty input must make no network call")
	}
}

func TestClientRetryAfter(t *testing.T) {
	dateAhead := func() string { return time.Now().Add(120 * time.Second).UTC().Format(http.TimeFormat) }
	cases := []struct {
		name   string
		status int
		header func() string
		min    time.Duration
		max    time.Duration
	}{
		{"429 seconds", 429, func() string { return "7" }, 7 * time.Second, 7 * time.Second},
		{"503 seconds", 503, func() string { return "7" }, 7 * time.Second, 7 * time.Second},
		{"429 date", 429, dateAhead, 118 * time.Second, 121 * time.Second},
		{"503 date", 503, dateAhead, 118 * time.Second, 121 * time.Second},
		{"garbage", 429, func() string { return "soon" }, 0, 0},
		{"empty", 503, func() string { return "" }, 0, 0},
		{"cap", 429, func() string { return "999999" }, time.Hour, time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if h := tc.header(); h != "" {
					w.Header().Set("Retry-After", h)
				}
				w.WriteHeader(tc.status)
			})
			c := testClient(f.url(), time.Second)
			_, err := c.Screen(context.Background(), "water rising")
			ce := asCallError(t, err)
			if !ce.Transient || ce.Quota || ce.Status != tc.status {
				t.Fatalf("error = %+v", ce)
			}
			if ce.RetryAfter < tc.min || ce.RetryAfter > tc.max {
				t.Errorf("RetryAfter = %v, want between %v and %v", ce.RetryAfter, tc.min, tc.max)
			}
			if f.hits.Load() != 1 {
				t.Errorf("hits = %d, want 1", f.hits.Load())
			}
		})
	}
}

func TestClientTransientAndPermanent(t *testing.T) {
	statusTable := []struct {
		status    int
		transient bool
	}{
		{429, true}, {500, true}, {502, true}, {503, true},
		{400, false}, {401, false}, {403, false}, {404, false},
	}
	for _, tc := range statusTable {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			f := newFakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			})
			_, err := testClient(f.url(), time.Second).Screen(context.Background(), "road closed")
			ce := asCallError(t, err)
			if ce.Status != tc.status || ce.Transient != tc.transient || ce.Quota {
				t.Errorf("error = %+v, want transient %v", ce, tc.transient)
			}
			if f.hits.Load() != 1 {
				t.Errorf("hits = %d, want 1", f.hits.Load())
			}
		})
	}

	t.Run("redirect is not followed", func(t *testing.T) {
		target := newFakeProvider(t, nil)
		f := newFakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.srv.URL+"/elsewhere", http.StatusFound)
		})
		_, err := testClient(f.url(), time.Second).Screen(context.Background(), "road closed")
		ce := asCallError(t, err)
		if ce.Status != http.StatusFound || ce.Transient || ce.Quota {
			t.Errorf("error = %+v, want permanent 302", ce)
		}
		if target.hits.Load() != 0 {
			t.Errorf("redirect target was hit %d times", target.hits.Load())
		}
		if f.hits.Load() != 1 {
			t.Errorf("hits = %d, want 1", f.hits.Load())
		}
	})

	t.Run("closed server", func(t *testing.T) {
		f := newFakeProvider(t, nil)
		addr := f.url()
		f.srv.Close()
		_, err := testClient(addr, time.Second).Screen(context.Background(), "road closed")
		ce := asCallError(t, err)
		if ce.Status != 0 || !ce.Transient {
			t.Errorf("error = %+v, want transient with status 0", ce)
		}
	})

	badBodies := map[string]string{
		"malformed":         "{not json",
		"empty results":     `{"model":"m","results":[]}`,
		"no category score": `{"model":"m","results":[{"flagged":false}]}`,
		"empty body":        "",
	}
	for name, body := range badBodies {
		t.Run(name, func(t *testing.T) {
			f := newFakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, body)
			})
			_, err := testClient(f.url(), time.Second).Screen(context.Background(), "road closed")
			ce := asCallError(t, err)
			if !ce.Transient || ce.Quota || ce.Status != 200 {
				t.Errorf("error = %+v, want transient 200", ce)
			}
			if f.hits.Load() != 1 {
				t.Errorf("hits = %d, want 1", f.hits.Load())
			}
		})
	}

	t.Run("answer without a model name uses the asked model", func(t *testing.T) {
		f := newFakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"results":[{"category_scores":{"hate":0.5}}]}`)
		})
		res, err := testClient(f.url(), time.Second).Screen(context.Background(), "road closed")
		if err != nil {
			t.Fatalf("Screen: %v", err)
		}
		if res.Model != DefaultModel || res.Scores["hate"] != 0.5 {
			t.Errorf("result = %+v", res)
		}
	})
}

func TestClientQuotaIsNotRetried(t *testing.T) {
	bodies := map[string]string{
		"code":     `{"error":{"code":"insufficient_quota","type":"something"}}`,
		"type":     `{"error":{"code":"other","type":"insufficient_quota"}}`,
		"both set": `{"error":{"code":"insufficient_quota","type":"insufficient_quota"}}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			f := newFakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, body)
			})
			_, err := testClient(f.url(), time.Second).Screen(context.Background(), "road closed")
			ce := asCallError(t, err)
			if !ce.Quota || ce.Transient || ce.Status != 429 {
				t.Errorf("error = %+v, want quota, not transient", ce)
			}
			if f.hits.Load() != 1 {
				t.Errorf("hits = %d, want exactly 1", f.hits.Load())
			}
		})
	}
}

func TestClientTimeout(t *testing.T) {
	f := newFakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	c := testClient(f.url(), 50*time.Millisecond)
	start := time.Now()
	_, err := c.Screen(context.Background(), "road closed")
	elapsed := time.Since(start)
	ce := asCallError(t, err)
	if !ce.Transient || ce.Status != 0 {
		t.Errorf("error = %+v, want transient with status 0", ce)
	}
	if elapsed >= time.Second {
		t.Errorf("took %v, want well under one second", elapsed)
	}

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := testClient(f.url(), time.Second).Screen(ctx, "road closed")
		ce := asCallError(t, err)
		if !ce.Transient || ce.Status != 0 {
			t.Errorf("error = %+v", ce)
		}
	})

	t.Run("production constructor", func(t *testing.T) {
		d := NewClient("k", "")
		if d.httpClient.Timeout != 3*time.Second {
			t.Errorf("timeout = %v, want 3s", d.httpClient.Timeout)
		}
		if d.model != DefaultModel {
			t.Errorf("model = %q", d.model)
		}
		if d.baseURL != moderationURL {
			t.Errorf("baseURL = %q", d.baseURL)
		}
		if d.httpClient.CheckRedirect == nil {
			t.Errorf("redirects must be refused")
		}
		if got := NewClient("k", "other-model").model; got != "other-model" {
			t.Errorf("explicit model = %q", got)
		}
	})
}

func TestClientBudgetRefusesWithoutNetwork(t *testing.T) {
	fixed := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return fixed }
	hourly := func(burst int) *rate.Limiter { return rate.NewLimiter(rate.Every(time.Hour), burst) }

	f := newFakeProvider(t, nil)
	c := newClientWithBaseURL(f.url(), "test-key", DefaultModel, time.Second, hourly(2), hourly(20), clock)
	ctx := context.Background()

	if _, err := c.Screen(ctx, strings.Repeat("a", 10)); err != nil {
		t.Fatalf("10 runes: %v", err)
	}
	if f.hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", f.hits.Load())
	}

	_, err := c.Screen(ctx, strings.Repeat("a", 15))
	ce := asCallError(t, err)
	if !ce.Transient || ce.Quota || ce.RetryAfter <= 0 {
		t.Fatalf("15 runes: error = %+v, want transient with positive RetryAfter", ce)
	}
	if f.hits.Load() != 1 {
		t.Fatalf("a token refusal made a network call: hits = %d", f.hits.Load())
	}

	if _, err := c.Screen(ctx, strings.Repeat("a", 5)); err != nil {
		t.Fatalf("5 runes after a refusal: %v (the refused call's request grant must come back)", err)
	}
	if f.hits.Load() != 2 {
		t.Fatalf("hits = %d, want 2", f.hits.Load())
	}

	_, err = c.Screen(ctx, strings.Repeat("a", 5))
	ce = asCallError(t, err)
	if !ce.Transient || ce.RetryAfter <= 0 {
		t.Fatalf("request refusal: error = %+v", ce)
	}
	if f.hits.Load() != 2 {
		t.Fatalf("a request refusal made a network call: hits = %d", f.hits.Load())
	}

	fresh := newClientWithBaseURL(f.url(), "test-key", DefaultModel, time.Second, hourly(2), hourly(20), clock)
	_, err = fresh.Screen(ctx, strings.Repeat("a", 21))
	ce = asCallError(t, err)
	if !ce.Transient || ce.RetryAfter != time.Minute {
		t.Fatalf("21 runes: error = %+v, want RetryAfter of one minute", ce)
	}
	if f.hits.Load() != 2 {
		t.Fatalf("an oversize refusal made a network call: hits = %d", f.hits.Load())
	}
	// The refusal must not have spent the request budget.
	if _, err := fresh.Screen(ctx, "ok"); err != nil {
		t.Fatalf("after oversize refusal: %v", err)
	}

	t.Run("limits add up to the free tier", func(t *testing.T) {
		if requestBurst+requestRefillPerMinute != 250 {
			t.Errorf("request budget = %d", requestBurst+requestRefillPerMinute)
		}
		if tokenBurst+tokenRefillPerMinute != 10000 {
			t.Errorf("token budget = %d", tokenBurst+tokenRefillPerMinute)
		}
		if tokenBurst < inputRuneCap {
			t.Errorf("token burst %d is below one full input", tokenBurst)
		}
	})
}

func TestModerationURLIsHTTPS(t *testing.T) {
	if !strings.HasPrefix(moderationURL, "https://") {
		t.Fatalf("moderationURL = %q, want https", moderationURL)
	}
	u, err := url.Parse(moderationURL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Host != "api.openai.com" || u.RawQuery != "" {
		t.Errorf("host %q query %q", u.Host, u.RawQuery)
	}
	if DefaultModel != "omni-moderation-2024-09-26" {
		t.Errorf("DefaultModel = %q", DefaultModel)
	}
	if inputRuneCap != 1000 {
		t.Errorf("inputRuneCap = %d", inputRuneCap)
	}
}

func TestClientErrorsCarryNoSecrets(t *testing.T) {
	const key = "sk-test-SECRETKEY"
	const text = "MG Road flooded near 9876543210"
	const marker = "BODYMARKER"

	respond := func(status int, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		}
	}
	quotaBody := `{"error":{"code":"insufficient_quota","message":"` + marker + `"}}`
	plainBody := `{"error":{"message":"` + marker + `"}}`

	check := func(t *testing.T, err error) {
		t.Helper()
		ce := asCallError(t, err)
		msg := ce.Error()
		if msg == "" {
			t.Errorf("empty error text")
		}
		for _, secret := range []string{key, text, marker, "9876543210", "MG Road"} {
			if strings.Contains(msg, secret) {
				t.Errorf("error text %q contains %q", msg, secret)
			}
		}
	}
	run := func(h http.HandlerFunc, timeout time.Duration) error {
		f := newFakeProvider(t, h)
		c := newClientWithBaseURL(f.url(), key, DefaultModel, timeout, unlimited(), unlimited(), time.Now)
		_, err := c.Screen(context.Background(), text)
		return err
	}

	t.Run("400", func(t *testing.T) { check(t, run(respond(400, plainBody), time.Second)) })
	t.Run("429", func(t *testing.T) { check(t, run(respond(429, plainBody), time.Second)) })
	t.Run("quota", func(t *testing.T) { check(t, run(respond(429, quotaBody), time.Second)) })
	t.Run("503", func(t *testing.T) { check(t, run(respond(503, plainBody), time.Second)) })
	t.Run("malformed", func(t *testing.T) { check(t, run(respond(200, marker), time.Second)) })
	t.Run("timeout", func(t *testing.T) {
		slow := func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
			}
		}
		check(t, run(slow, 50*time.Millisecond))
	})
	t.Run("budget", func(t *testing.T) {
		f := newFakeProvider(t, nil)
		spent := rate.NewLimiter(rate.Every(time.Hour), 1)
		c := newClientWithBaseURL(f.url(), key, DefaultModel, time.Second, spent, unlimited(), time.Now)
		if _, err := c.Screen(context.Background(), text); err != nil {
			t.Fatalf("first call: %v", err)
		}
		_, err := c.Screen(context.Background(), text)
		check(t, err)
	})
	t.Run("zero value built outside the package", func(t *testing.T) {
		msg := (&CallError{Status: 401}).Error()
		if msg == "" || !strings.Contains(msg, "401") {
			t.Errorf("Error() = %q, want non empty and naming 401", msg)
		}
	})
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"seconds", "7", 7 * time.Second},
		{"padded seconds", "  30 ", 30 * time.Second},
		{"zero", "0", 0},
		{"negative", "-5", 0},
		{"garbage", "soon", 0},
		{"empty", "", 0},
		{"cap", "999999", time.Hour},
		{"date ahead", now.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second},
		{"date past", now.Add(-90 * time.Second).Format(http.TimeFormat), 0},
		{"date far ahead", now.Add(48 * time.Hour).Format(http.TimeFormat), time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseRetryAfter(tc.in, now); got != tc.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
