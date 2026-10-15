// Package moderation is a standalone client for the OpenAI Moderation
// endpoint. It has no dependency on the service or api packages, so the
// submit path and the background worker can both import it. It wraps
// golang.org/x/time/rate for its request and token budget and sends only the
// cleaned description text, never an account, session, IP address or
// coordinates (D-01).
package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/time/rate"
)

const (
	// moderationURL is the only outbound URL this client ever calls.
	moderationURL = "https://api.openai.com/v1/moderations"
	// DefaultModel is the dated snapshot the policy thresholds were recorded on.
	DefaultModel = "omni-moderation-2024-09-26"
	// inputRuneCap is the most runes of description text one call sends.
	inputRuneCap = 1000
	// requestTimeout is the whole round trip deadline.
	requestTimeout = 3 * time.Second
	// maxRetryAfter is the longest wait a Retry-After header can ask for.
	maxRetryAfter = time.Hour
	// errorBodyLimit bounds how much of an error body is read.
	errorBodyLimit = 64 << 10
	// successBodyLimit bounds how much of a success body is read.
	successBodyLimit = 1 << 20
	// requestsPerMinute is the most requests the client lets through a minute.
	requestsPerMinute = 250
	// tokensPerMinute is the most estimated tokens the client lets through a minute.
	tokensPerMinute = 10000
	// requestBurst is the request bucket size.
	requestBurst = 50
	// tokenBurst is the token bucket size.
	tokenBurst = 2 * inputRuneCap
	// requestRefillPerMinute is the request bucket refill, so burst plus refill is the cap.
	requestRefillPerMinute = requestsPerMinute - requestBurst
	// tokenRefillPerMinute is the token bucket refill, so burst plus refill is the cap.
	tokenRefillPerMinute = tokensPerMinute - tokenBurst
	// quotaCode is the provider error code for an account with no usable quota.
	quotaCode = "insufficient_quota"
)

// Result is what a successful check returns: the model that answered and the
// per category scores. The provider's own per result flagged boolean is never
// decoded, so the policy is the only judge of the scores.
type Result struct {
	Model  string
	Scores map[string]float64
}

// CallError is the only error type Screen returns, always as a pointer.
// Callers use errors.As with a *CallError target. Transient means try again
// later (honour RetryAfter), Quota means the account has no usable quota and
// must not be retried, and neither flag means a permanent fault.
type CallError struct {
	Status     int
	Transient  bool
	Quota      bool
	RetryAfter time.Duration
	cause      string
}

// Error is built only from a fixed phrase, the status number and one class
// word, so it can never carry the key, the text or the provider body. A value
// built outside this package has no cause and uses the phrase "provider error".
func (e *CallError) Error() string {
	cause := e.cause
	if cause == "" {
		cause = "provider error"
	}
	class := "permanent"
	switch {
	case e.Quota:
		class = "quota"
	case e.Transient:
		class = "transient"
	}
	if e.Status != 0 {
		return fmt.Sprintf("moderation: %s (status %d, %s)", cause, e.Status, class)
	}
	return fmt.Sprintf("moderation: %s (%s)", cause, class)
}

// Screener checks one piece of text. The real Client and the development
// PassScreener both satisfy it.
type Screener interface {
	Screen(ctx context.Context, text string) (Result, error)
}

// Client is the provider client. Safe for concurrent use.
type Client struct {
	httpClient *http.Client
	requests   *rate.Limiter
	tokens     *rate.Limiter
	baseURL    string
	apiKey     string
	model      string
	now        func() time.Time
}

var _ Screener = (*Client)(nil)

// NewClient builds the client for the real endpoint. An empty model means
// DefaultModel. One Client is built per process (04-11 passes the same value
// to the submit path and the worker), which is what makes its request and
// token budget process wide: at most 250 requests and 10,000 estimated tokens
// in any minute.
func NewClient(apiKey, model string) *Client {
	return newClientWithBaseURL(
		moderationURL, apiKey, model, requestTimeout,
		rate.NewLimiter(rate.Limit(float64(requestRefillPerMinute)/60), requestBurst),
		rate.NewLimiter(rate.Limit(float64(tokenRefillPerMinute)/60), tokenBurst),
		time.Now,
	)
}

// newClientWithBaseURL is NewClient's seam: tests pass a fake server URL, a
// short timeout, their own limiters and a fixed clock. No redirect is ever
// followed, so the Authorization header cannot reach another host.
func newClientWithBaseURL(baseURL, apiKey, model string, timeout time.Duration, requests, tokens *rate.Limiter, now func() time.Time) *Client {
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		requests: requests,
		tokens:   tokens,
		baseURL:  baseURL,
		apiKey:   apiKey,
		model:    model,
		now:      now,
	}
}

type moderationRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type moderationResponse struct {
	Model   string `json:"model"`
	Results []struct {
		CategoryScores map[string]float64 `json:"category_scores"`
	} `json:"results"`
}

type errorEnvelope struct {
	Error struct {
		Code string `json:"code"`
		Type string `json:"type"`
	} `json:"error"`
}

// refuse builds the local budget refusal from a reservation delay. A
// reservation that was not OK (more runes than the bucket can ever hold)
// waits one minute; otherwise the delay rounds up to whole seconds.
func refuse(d time.Duration, ok bool) *CallError {
	wait := time.Minute
	if ok {
		wait = ((d + time.Second - 1) / time.Second) * time.Second
		if wait < time.Second {
			wait = time.Second
		}
	}
	return &CallError{Transient: true, RetryAfter: wait, cause: "local budget spent"}
}

// Screen sends the trimmed text, cut to its first 1000 runes, to the provider
// and returns the category scores. It makes at most one HTTP attempt and never
// retries; bounded retries belong to the background worker. Every failure is a
// *CallError.
func (c *Client) Screen(ctx context.Context, text string) (Result, error) {
	input := strings.TrimSpace(text)
	if utf8.RuneCountInString(input) > inputRuneCap {
		input = string([]rune(input)[:inputRuneCap])
	}
	if input == "" {
		return Result{}, &CallError{cause: "empty input"}
	}

	// Both grants are taken at one captured instant, so a refusal can return
	// them with CancelAt at that same instant and spend nothing.
	now := c.now()
	n := utf8.RuneCountInString(input)
	rr := c.requests.ReserveN(now, 1)
	if !rr.OK() || rr.DelayFrom(now) > 0 {
		ok, d := rr.OK(), rr.DelayFrom(now)
		rr.CancelAt(now)
		return Result{}, refuse(d, ok)
	}
	tr := c.tokens.ReserveN(now, n)
	if !tr.OK() || tr.DelayFrom(now) > 0 {
		ok, d := tr.OK(), tr.DelayFrom(now)
		tr.CancelAt(now)
		rr.CancelAt(now)
		return Result{}, refuse(d, ok)
	}

	body, err := json.Marshal(moderationRequest{Model: c.model, Input: input})
	if err != nil {
		return Result{}, &CallError{cause: "request not built"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return Result{}, &CallError{cause: "request not built"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, &CallError{Transient: true, cause: "provider unreachable or timed out"}
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	switch {
	case status == http.StatusTooManyRequests:
		var env errorEnvelope
		_ = json.NewDecoder(io.LimitReader(resp.Body, errorBodyLimit)).Decode(&env)
		if env.Error.Code == quotaCode || env.Error.Type == quotaCode {
			return Result{}, &CallError{Status: status, Quota: true, cause: "provider status"}
		}
		return Result{}, &CallError{
			Status: status, Transient: true, cause: "provider status",
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), c.now()),
		}
	case status >= 500 && status <= 599:
		return Result{}, &CallError{
			Status: status, Transient: true, cause: "provider status",
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), c.now()),
		}
	case status < 200 || status > 299:
		return Result{}, &CallError{Status: status, cause: "provider status"}
	}

	var out moderationResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, successBodyLimit)).Decode(&out); err != nil ||
		len(out.Results) == 0 || out.Results[0].CategoryScores == nil {
		return Result{}, &CallError{Status: status, Transient: true, cause: "malformed provider answer"}
	}
	model := out.Model
	if model == "" {
		model = c.model
	}
	return Result{Model: model, Scores: out.Results[0].CategoryScores}, nil
}

// parseRetryAfter reads a Retry-After header given in seconds or as an HTTP
// date. Negative, unparseable and empty values give 0; the result is capped
// at one hour.
func parseRetryAfter(h string, now time.Time) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.ParseInt(h, 10, 64); err == nil {
		if secs <= 0 {
			return 0
		}
		if secs > int64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(h); err == nil {
		d = t.Sub(now)
		if d < 0 {
			return 0
		}
	} else {
		return 0
	}
	if d > maxRetryAfter {
		return maxRetryAfter
	}
	return d
}
