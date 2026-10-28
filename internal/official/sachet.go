package official

// The SACHET client. SACHET is NDMA's Common Alerting Protocol service for
// India. The only hosts this package ever calls are the constant sachetBaseURL
// in production and, in this package's tests, the base the unexported seam
// constructor is given. It never follows a URL it finds in the feed: the RSS
// item is a pointer, and the CAP message behind it is the alert (research G2).
// A CAP address is built from a guid that passed ValidGUID, on the fixed host,
// and nothing else in an item is ever requested. Redirects are refused.
//
// The client is a good citizen: it names Pinalert and a contact in its
// User-Agent, sends the stored validators on the list request, caps both
// bodies, runs three CAP requests at a time, and retries a failed CAP request
// once. Every failure is reduced to one closed word (see ErrorClass), and no
// error value ever holds a URL or a query.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// sachetBaseURL is the only host production code ever calls.
const sachetBaseURL = "https://sachet.ndma.gov.in"

const (
	sachetRSSPath = "/cap_public_website/rss/rss_india.xml"
	sachetCAPPath = "/cap_public_website/FetchXMLFile"
)

// The client's bounds (research G2 and G9).
const (
	RSSMaxBytes    = 1 << 20
	CAPMaxBytes    = 256 << 10
	CAPConcurrency = 3
	CAPTimeout     = 8 * time.Second
	CAPRetries     = 1
	RSSTimeout     = 15 * time.Second

	defaultRetryDelay = 500 * time.Millisecond
	repoURL           = "https://github.com/swathivallabhaneni289/pinalert"
)

// ClientConfig is what the caller knows about this deployment. BaseURL is
// Pinalert's own public address and ContactEmail the operator's, both taken
// from configuration and used only to say who is calling. Both may be empty.
type ClientConfig struct {
	ContactEmail string
	BaseURL      string
}

// UserAgent builds "Pinalert/1.0 (+<BaseURL>; contact <ContactEmail>)" with
// whichever parts are set. With no base URL it names the repository, the same
// fallback internal/geocode uses. Line breaks and control characters in the
// configuration are removed so a setting cannot split the header.
func UserAgent(cfg ClientConfig) string {
	clean := func(s string) string {
		return strings.Join(strings.Fields(CleanText(s, 200)), " ")
	}
	plus := repoURL
	if b := clean(cfg.BaseURL); b != "" {
		plus = b
	}
	ua := "Pinalert/1.0 (+" + plus
	if e := clean(cfg.ContactEmail); e != "" {
		ua += "; contact " + e
	}
	return ua + ")"
}

// SachetClient reads the SACHET RSS list and CAP messages. Safe for
// concurrent use.
type SachetClient struct {
	base       string
	userAgent  string
	httpClient *http.Client
	rssTimeout time.Duration
	capTimeout time.Duration
	retryDelay time.Duration
}

// NewSachetClient returns a client for the real SACHET host.
func NewSachetClient(cfg ClientConfig) *SachetClient {
	return newSachetClientWithBase(sachetBaseURL, cfg, 0)
}

// newSachetClientWithBase is the test seam: an explicit base for a fake
// upstream and, when timeout is positive, one timeout for both kinds of
// request. No production code calls it with anything but the constant.
func newSachetClientWithBase(base string, cfg ClientConfig, timeout time.Duration) *SachetClient {
	c := &SachetClient{
		base:      strings.TrimRight(base, "/"),
		userAgent: UserAgent(cfg),
		httpClient: &http.Client{
			// A redirect is never followed: the 3xx answer is returned as it is
			// and reported as a status error.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		rssTimeout: RSSTimeout,
		capTimeout: CAPTimeout,
		retryDelay: defaultRetryDelay,
	}
	if timeout > 0 {
		c.rssTimeout, c.capTimeout = timeout, timeout
	}
	return c
}

// fetchError is the only error type this client returns for its own failures.
// Its text is fixed words; the cause behind it is kept for errors.Is but never
// printed, because a transport error would print the request URL.
type fetchError struct {
	class  string // timeout, status, parse or network
	status int    // the HTTP status, for class status
	msg    string
	cause  error
}

func (e *fetchError) Error() string { return "official: sachet " + e.msg }
func (e *fetchError) Unwrap() error { return e.cause }

// parseError is a body that could not be used.
func parseError(msg string) error {
	return &fetchError{class: "parse", msg: msg, cause: ErrParse}
}

// transportError reduces an error from the HTTP client or from reading a body
// to timeout or network, dropping the *url.Error wrapper and its URL.
func transportError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return &fetchError{class: "timeout", msg: "timed out", cause: err}
	}
	return &fetchError{class: "network", msg: "network error", cause: err}
}

// ErrorClass reduces any error from this package to one closed word: timeout,
// status, parse or network. It returns "" for nil. Callers log this word and
// nothing else about a failure.
func ErrorClass(err error) string {
	if err == nil {
		return ""
	}
	var fe *fetchError
	if errors.As(err, &fe) {
		return fe.class
	}
	if errors.Is(err, ErrParse) {
		return "parse"
	}
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return "timeout"
	}
	return "network"
}

// retryable says whether a CAP request is worth one more try: a timeout, a
// network failure or a 5xx answer. A 4xx, a redirect or an unreadable body is
// not.
func retryable(err error) bool {
	var fe *fetchError
	if !errors.As(err, &fe) {
		return false
	}
	switch fe.class {
	case "timeout", "network":
		return true
	case "status":
		return fe.status >= 500
	}
	return false
}

type response struct {
	status int
	header http.Header
	body   []byte
}

// get makes one bounded GET. A 304 comes back as a response with no body; any
// other status but 200 is a status error. The body is read through a limit of
// maxBytes plus one byte so an oversize body is detected, not truncated.
func (c *SachetClient) get(ctx context.Context, target string, timeout time.Duration, maxBytes int, headers map[string]string) (*response, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, &fetchError{class: "network", msg: "bad request"}
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	for k, v := range headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, transportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return &response{status: resp.StatusCode, header: resp.Header}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &fetchError{class: "status", status: resp.StatusCode, msg: fmt.Sprintf("status %d", resp.StatusCode)}
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "xml") {
		return nil, parseError("unexpected content type")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)+1))
	if err != nil {
		return nil, transportError(err)
	}
	if len(body) > maxBytes {
		return nil, parseError("body too large")
	}
	return &response{status: resp.StatusCode, header: resp.Header, body: body}, nil
}

// ListResult is the answer to the RSS list request. On a 304, NotModified is
// true, Items is empty and the validators are the ones that were sent.
type ListResult struct {
	Items        []Item
	ETag         string
	LastModified string
	NotModified  bool
}

// FetchList reads the India RSS list. etag and lastModified are the validators
// stored from the last success; empty ones are not sent.
func (c *SachetClient) FetchList(ctx context.Context, etag, lastModified string) (ListResult, error) {
	resp, err := c.get(ctx, c.base+sachetRSSPath, c.rssTimeout, RSSMaxBytes, map[string]string{
		"If-None-Match":     etag,
		"If-Modified-Since": lastModified,
	})
	if err != nil {
		return ListResult{}, err
	}
	res := ListResult{ETag: etag, LastModified: lastModified}
	if v := resp.header.Get("ETag"); v != "" {
		res.ETag = v
	}
	if v := resp.header.Get("Last-Modified"); v != "" {
		res.LastModified = v
	}
	if resp.status == http.StatusNotModified {
		res.NotModified = true
		return res, nil
	}
	items, err := ParseRSS(bytes.NewReader(resp.body))
	if err != nil {
		return ListResult{}, err
	}
	res.Items = items
	return res, nil
}

// CAPResult is the outcome for one item. Version is set when Err is nil; Kept
// says whether the message passed Version.Keep.
type CAPResult struct {
	Item    Item
	Version Version
	Kept    bool
	Err     error
}

// FetchCAPs fetches the CAP message behind each item, at most CAPConcurrency at
// a time, and returns one result per item in the input order. A failed item
// never stops the others.
func (c *SachetClient) FetchCAPs(ctx context.Context, items []Item) []CAPResult {
	results := make([]CAPResult, len(items))
	done := make(chan struct{}, len(items))
	sem := make(chan struct{}, CAPConcurrency)
	started := 0
	for i := range items {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			results[i] = CAPResult{Item: items[i], Err: transportError(ctx.Err())}
			continue
		}
		started++
		go func(i int) {
			defer func() { <-sem; done <- struct{}{} }()
			results[i] = c.fetchCAP(ctx, items[i])
		}(i)
	}
	for ; started > 0; started-- {
		<-done
	}
	return results
}

// fetchCAP builds the address from the guid alone, so nothing in the item but
// a validated id ever reaches the network.
func (c *SachetClient) fetchCAP(ctx context.Context, it Item) CAPResult {
	res := CAPResult{Item: it}
	if !ValidGUID(it.GUID) {
		res.Err = parseError("item id is not valid")
		return res
	}
	target := c.base + sachetCAPPath + "?" + url.Values{"identifier": {it.GUID}}.Encode()
	for attempt := 0; ; attempt++ {
		v, err := c.fetchCAPOnce(ctx, target, it)
		if err == nil {
			res.Version, res.Kept = v, v.Keep()
			return res
		}
		if attempt >= CAPRetries || !retryable(err) || ctx.Err() != nil {
			res.Err = err
			return res
		}
		select {
		case <-time.After(c.retryDelay):
		case <-ctx.Done():
			res.Err = transportError(ctx.Err())
			return res
		}
	}
}

func (c *SachetClient) fetchCAPOnce(ctx context.Context, target string, it Item) (Version, error) {
	resp, err := c.get(ctx, target, c.capTimeout, CAPMaxBytes, nil)
	if err != nil {
		return Version{}, err
	}
	return ParseCAP(bytes.NewReader(resp.body), it)
}
