package push

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

var (
	// ErrNoTTL means the message had no positive whole second TTL. Apple
	// answers BadTtl to a zero TTL, and other services drop a zero TTL
	// message when the device is not reachable right now.
	ErrNoTTL = errors.New("push: a send needs a TTL of at least one second")
	// ErrBadTopic means the Topic is empty, over 32 characters or outside the
	// base64url alphabet.
	ErrBadTopic = errors.New("push: invalid topic")
	// ErrBadUrgency means the Urgency is neither high nor normal.
	ErrBadUrgency = errors.New("push: invalid urgency")
)

// MaxRetryAfter caps a Retry-After answer so a push service cannot park the
// sender for longer than this.
const MaxRetryAfter = 10 * time.Minute

// maxDrain is how much of a response body is read before it is closed.
const maxDrain = 4096

// Urgency is the Web Push Urgency header value the sender supports.
type Urgency string

const (
	// UrgencyHigh is for critical reports.
	UrgencyHigh Urgency = "high"
	// UrgencyNormal is for everything else.
	UrgencyNormal Urgency = "normal"
)

// Subscription is the stored endpoint and keys of one device.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Message is one notification: the envelope plus the delivery options. TTL is
// rounded down to whole seconds and must be at least one second.
type Message struct {
	Payload Payload
	TTL     time.Duration
	Urgency Urgency
	Topic   string
}

// Result is what the push service answered. Status is the HTTP status;
// RetryAfter is the capped Retry-After value, zero when absent or unusable.
// A non 2xx status is not an error: the caller decides what each one means.
type Result struct {
	Status     int
	RetryAfter time.Duration
}

// Sender sends one notification to one device.
type Sender interface {
	Send(ctx context.Context, sub Subscription, msg Message) (Result, error)
}

// WebPushSender sends through webpush-go with a guarded client.
//
// Library traps this wrapper accounts for (research Pitfalls 1 to 5):
// Options.VapidExpiration is ignored (the JWT exp is fixed at 12 hours, inside
// Apple's one day limit); the TTL header is always written, so a zero TTL must
// be refused here; Subscriber gets "mailto:" prepended unless it starts with
// "https:"; the default HTTP client has no timeout and follows redirects; and a
// nil Options panics.
type WebPushSender struct {
	cfg    Config
	client *http.Client
	policy EndpointPolicy
	now    func() time.Time
}

// NewWebPushSender builds a sender. A nil client becomes NewGuardedClient. The
// production caller passes NewGuardedClient() itself; tests pass a client with
// a fake transport.
func NewWebPushSender(cfg Config, client *http.Client, policy EndpointPolicy) *WebPushSender {
	if client == nil {
		client = NewGuardedClient()
	}
	return &WebPushSender{cfg: cfg, client: client, policy: policy, now: time.Now}
}

// Send encrypts and posts one notification. No error it returns holds the
// endpoint, which is a capability URL: the policy errors carry nothing and a
// transport error is unwrapped from its url error and tagged with the host
// only.
func (s *WebPushSender) Send(ctx context.Context, sub Subscription, msg Message) (Result, error) {
	if msg.TTL < time.Second {
		return Result{}, ErrNoTTL
	}
	if !ValidTopic(msg.Topic) {
		return Result{}, ErrBadTopic
	}
	var urgency webpush.Urgency
	switch msg.Urgency {
	case UrgencyHigh:
		urgency = webpush.UrgencyHigh
	case UrgencyNormal:
		urgency = webpush.UrgencyNormal
	default:
		return Result{}, ErrBadUrgency
	}
	host, err := s.policy.Check(sub.Endpoint)
	if err != nil {
		return Result{}, err
	}
	if err := CheckKeys(sub.P256dh, sub.Auth); err != nil {
		return Result{}, err
	}
	body, err := msg.Payload.Encode()
	if err != nil {
		return Result{}, err
	}

	resp, err := webpush.SendNotificationWithContext(ctx, body,
		&webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth}},
		&webpush.Options{
			HTTPClient:      s.client,
			Subscriber:      s.cfg.Subject,
			VAPIDPublicKey:  s.cfg.PublicKey,
			VAPIDPrivateKey: s.cfg.PrivateKey,
			TTL:             int(msg.TTL / time.Second),
			Urgency:         urgency,
			Topic:           msg.Topic,
		})
	if err != nil {
		return Result{}, fmt.Errorf("push: send to %s: %w", host, scrubURLError(err))
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrain))

	return Result{
		Status:     resp.StatusCode,
		RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), s.now()),
	}, nil
}

// parseRetryAfter reads seconds or an HTTP date and caps the result. Anything
// unusable gives zero.
func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.Atoi(value); err == nil {
		if secs < 0 {
			return 0
		}
		if secs > int(MaxRetryAfter/time.Second) {
			return MaxRetryAfter
		}
		d = time.Duration(secs) * time.Second
	} else if when, err := http.ParseTime(value); err == nil {
		d = when.Sub(now)
	} else {
		return 0
	}
	if d < 0 {
		return 0
	}
	if d > MaxRetryAfter {
		return MaxRetryAfter
	}
	return d
}

// scrubURLError drops the request URL from an error. net/http wraps every
// transport failure in a *url.Error whose text carries the full URL, and for a
// push send that URL is the endpoint, a capability that lets anyone send to
// the device. The inner error keeps the useful class (a timeout, a refused
// dial, ErrRefusedAddress) and works with errors.Is and errors.As. This is the
// same rule as the function of the same name in internal/geocode/client.go.
func scrubURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return urlErr.Err
	}
	return err
}
