package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// These tests drive the client against httptest servers that serve the recorded
// fixtures. No test here reaches the real SACHET host.

const testTimeout = 400 * time.Millisecond

func testClient(base string, cfg ClientConfig) *SachetClient {
	c := newSachetClientWithBase(base, cfg, testTimeout)
	c.retryDelay = 0
	return c
}

func serveXML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
	_, _ = w.Write(body)
}

func TestClientUserAgent(t *testing.T) {
	var mu sync.Mutex
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		agents = append(agents, r.UserAgent())
		mu.Unlock()
		if r.URL.Path == sachetRSSPath {
			serveXML(w, fixture(t, "rss_head.xml"))
			return
		}
		serveXML(w, fixture(t, "cap_cwc_flood.xml"))
	}))
	defer srv.Close()

	cfg := ClientConfig{ContactEmail: "ops@example.org", BaseURL: "https://pinalert.example.org"}
	c := testClient(srv.URL, cfg)
	if _, err := c.FetchList(context.Background(), "", ""); err != nil {
		t.Fatalf("FetchList: %v", err)
	}
	res := c.FetchCAPs(context.Background(), []Item{{GUID: "1792897745415016"}})
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("FetchCAPs: %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(agents) != 2 {
		t.Fatalf("agents seen = %d, want 2", len(agents))
	}
	for _, ua := range agents {
		if !strings.HasPrefix(ua, "Pinalert/1.0 (+") {
			t.Errorf("User-Agent %q does not start with Pinalert/1.0 (+", ua)
		}
		if !strings.Contains(ua, "https://pinalert.example.org") || !strings.Contains(ua, "ops@example.org") {
			t.Errorf("User-Agent %q should hold the base URL and the contact", ua)
		}
		if strings.Contains(ua, "Go-http-client") {
			t.Errorf("User-Agent %q is the stock one", ua)
		}
	}

	if got := UserAgent(ClientConfig{}); got != "Pinalert/1.0 (+https://github.com/swathivallabhaneni289/pinalert)" {
		t.Errorf("empty config User-Agent = %q", got)
	}
	if got := UserAgent(ClientConfig{BaseURL: "https://pinalert.example.org"}); got != "Pinalert/1.0 (+https://pinalert.example.org)" {
		t.Errorf("base only User-Agent = %q", got)
	}
	if got := UserAgent(ClientConfig{ContactEmail: "ops@example.org"}); !strings.HasPrefix(got, "Pinalert/1.0 (+https://github.com/") || !strings.Contains(got, "ops@example.org") {
		t.Errorf("contact only User-Agent = %q", got)
	}
	// A header value cannot be split by a hostile setting.
	if got := UserAgent(ClientConfig{ContactEmail: "a@b.c\r\nX-Evil: 1"}); strings.ContainsAny(got, "\r\n") {
		t.Errorf("User-Agent holds a line break: %q", got)
	}
	// The real client points at the fixed host over https.
	if sachetBaseURL != "https://sachet.ndma.gov.in" || NewSachetClient(ClientConfig{}).base != sachetBaseURL {
		t.Errorf("production client does not use the constant host")
	}
}

// recordingTransport notes every host the client tries to reach.
type recordingTransport struct {
	mu    sync.Mutex
	hosts []string
	next  http.RoundTripper
}

func (rt *recordingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	rt.hosts = append(rt.hosts, r.URL.Host)
	rt.mu.Unlock()
	return rt.next.RoundTrip(r)
}

func TestClientBuildsCAPURLFromGUIDOnly(t *testing.T) {
	var mu sync.Mutex
	var capRequests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == sachetRSSPath {
			body := strings.ReplaceAll(string(fixture(t, "rss_head.xml")),
				"https://sachet.ndma.gov.in/cap_public_website/FetchXMLFile?identifier=1792919196941008",
				"http://evil.example/x")
			serveXML(w, []byte(body))
			return
		}
		mu.Lock()
		capRequests = append(capRequests, r.URL.RequestURI())
		mu.Unlock()
		serveXML(w, fixture(t, "cap_ap_sdma_lightning.xml"))
	}))
	defer srv.Close()

	c := testClient(srv.URL, ClientConfig{})
	rt := &recordingTransport{next: http.DefaultTransport}
	c.httpClient.Transport = rt

	list, err := c.FetchList(context.Background(), "", "")
	if err != nil {
		t.Fatalf("FetchList: %v", err)
	}
	var evil Item
	for _, it := range list.Items {
		if strings.Contains(it.Link, "evil.example") {
			evil = it
		}
	}
	if evil.GUID != "1792919196941008" {
		t.Fatalf("the evil link item was not found among %d items", len(list.Items))
	}
	res := c.FetchCAPs(context.Background(), []Item{evil})
	if len(res) != 1 || res[0].Err != nil || !res[0].Kept {
		t.Fatalf("FetchCAPs: %+v", res)
	}
	if res[0].Version.GUID != evil.GUID || res[0].Version.CapIdentifier != "IN-1792919196941008_8" {
		t.Errorf("version = %+v", res[0].Version)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(capRequests) != 1 || capRequests[0] != "/cap_public_website/FetchXMLFile?identifier=1792919196941008" {
		t.Errorf("CAP requests = %v", capRequests)
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	want := strings.TrimPrefix(srv.URL, "http://")
	for _, h := range rt.hosts {
		if h != want {
			t.Errorf("client reached host %q, want only %q", h, want)
		}
	}
}

func TestClientRefusesBadGUID(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		serveXML(w, fixture(t, "cap_cwc_flood.xml"))
	}))
	defer srv.Close()

	c := testClient(srv.URL, ClientConfig{})
	items := []Item{
		{GUID: "../123"},
		{GUID: ""},
		{GUID: "1792897745415016&x=1"},
		{GUID: "http://evil.example/1234567890"},
	}
	res := c.FetchCAPs(context.Background(), items)
	if len(res) != len(items) {
		t.Fatalf("results = %d, want %d", len(res), len(items))
	}
	for i, r := range res {
		if r.Err == nil || r.Kept {
			t.Errorf("item %d (%q): want an error and not kept, got %+v", i, items[i].GUID, r)
		}
		if ErrorClass(r.Err) != "parse" {
			t.Errorf("item %d: class = %q, want parse", i, ErrorClass(r.Err))
		}
		if r.Item.GUID != items[i].GUID {
			t.Errorf("item %d: result is for %q", i, r.Item.GUID)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("server saw %d requests, want none", hits.Load())
	}
}

func TestClientRefusesRedirect(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		serveXML(w, fixture(t, "cap_cwc_flood.xml"))
	}))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/elsewhere", http.StatusFound)
	}))
	defer srv.Close()

	c := testClient(srv.URL, ClientConfig{})
	if _, err := c.FetchList(context.Background(), "", ""); err == nil || ErrorClass(err) != "status" {
		t.Errorf("FetchList over a redirect: err=%v class=%q, want status", err, ErrorClass(err))
	}
	res := c.FetchCAPs(context.Background(), []Item{{GUID: "1792897745415016"}})
	if res[0].Err == nil || ErrorClass(res[0].Err) != "status" {
		t.Errorf("FetchCAPs over a redirect: %+v", res[0])
	}
	if targetHits.Load() != 0 {
		t.Errorf("the redirect target was requested %d times", targetHits.Load())
	}
}

func TestClientConditionalGet(t *testing.T) {
	const etag = `W/"79131-1792919710087"`
	const lastMod = "Sun, 25 Oct 2026 09:15:09 GMT"
	var mu sync.Mutex
	var gotNoneMatch, gotSince []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotNoneMatch = append(gotNoneMatch, r.Header.Get("If-None-Match"))
		gotSince = append(gotSince, r.Header.Get("If-Modified-Since"))
		mu.Unlock()
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", lastMod)
		serveXML(w, fixture(t, "rss_head.xml"))
	}))
	defer srv.Close()

	c := testClient(srv.URL, ClientConfig{})
	first, err := c.FetchList(context.Background(), "", "")
	if err != nil {
		t.Fatalf("first FetchList: %v", err)
	}
	if first.NotModified || len(first.Items) != 5 || first.ETag != etag || first.LastModified != lastMod {
		t.Errorf("first = notModified=%v items=%d etag=%q lastModified=%q", first.NotModified, len(first.Items), first.ETag, first.LastModified)
	}
	second, err := c.FetchList(context.Background(), first.ETag, first.LastModified)
	if err != nil {
		t.Fatalf("second FetchList: %v", err)
	}
	if !second.NotModified || len(second.Items) != 0 {
		t.Errorf("second = notModified=%v items=%d, want a 304", second.NotModified, len(second.Items))
	}
	if second.ETag != etag || second.LastModified != lastMod {
		t.Errorf("a 304 should hand back the validators it was given: %q %q", second.ETag, second.LastModified)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotNoneMatch[0] != "" || gotSince[0] != "" {
		t.Errorf("first request sent validators: %q %q", gotNoneMatch[0], gotSince[0])
	}
	if gotNoneMatch[1] != etag || gotSince[1] != lastMod {
		t.Errorf("second request validators = %q %q", gotNoneMatch[1], gotSince[1])
	}
}

func TestClientErrorClasses(t *testing.T) {
	rssPad := func(n int) []byte {
		return []byte("<rss><channel>" + strings.Repeat(" ", n) + "</channel></rss>")
	}
	capPad := func(n int) []byte {
		return []byte(`<cap:alert xmlns:cap="urn:oasis:names:tc:emergency:cap:1.2"><cap:sent>2026-10-25T08:38:44+05:30</cap:sent>` +
			strings.Repeat(" ", n) + `</cap:alert>`)
	}
	type tc struct {
		name    string
		handler http.HandlerFunc
		class   string
		text    string // the error text must hold this, when set
	}
	cases := []tc{
		{"500", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }, "status", ""},
		{"404", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }, "status", ""},
		{"timeout", func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(3 * time.Second):
			}
		}, "timeout", ""},
		{"garbage", func(w http.ResponseWriter, r *http.Request) { serveXML(w, []byte("this is not xml")) }, "parse", ""},
		{"not an xml content type", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(rssPad(10))
		}, "parse", ""},
		{"oversize rss", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == sachetRSSPath {
				serveXML(w, rssPad(RSSMaxBytes+10))
				return
			}
			serveXML(w, capPad(CAPMaxBytes+10))
		}, "parse", "too large"},
	}
	for _, c := range cases {
		t.Run(c.name+" list", func(t *testing.T) {
			srv := httptest.NewServer(c.handler)
			defer srv.Close()
			_, err := testClient(srv.URL, ClientConfig{}).FetchList(context.Background(), "", "")
			if err == nil {
				t.Fatal("want an error")
			}
			if got := ErrorClass(err); got != c.class {
				t.Errorf("class = %q, want %q (err %v)", got, c.class, err)
			}
			if c.text != "" && !strings.Contains(err.Error(), c.text) {
				t.Errorf("error %q does not say %q", err, c.text)
			}
			if strings.Contains(err.Error(), srv.URL) {
				t.Errorf("error holds the request URL: %q", err)
			}
		})
		t.Run(c.name+" cap", func(t *testing.T) {
			srv := httptest.NewServer(c.handler)
			defer srv.Close()
			res := testClient(srv.URL, ClientConfig{}).FetchCAPs(context.Background(), []Item{{GUID: "1792897745415016"}})
			err := res[0].Err
			if err == nil {
				t.Fatal("want an error")
			}
			if got := ErrorClass(err); got != c.class {
				t.Errorf("class = %q, want %q (err %v)", got, c.class, err)
			}
			if c.text != "" && !strings.Contains(err.Error(), c.text) {
				t.Errorf("error %q does not say %q", err, c.text)
			}
			if strings.Contains(err.Error(), "identifier=") || strings.Contains(err.Error(), srv.URL) {
				t.Errorf("error holds the request URL: %q", err)
			}
		})
	}

	t.Run("connection refused", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()
		c := testClient(url, ClientConfig{})
		_, err := c.FetchList(context.Background(), "", "")
		if err == nil || ErrorClass(err) != "network" {
			t.Errorf("list: err=%v class=%q, want network", err, ErrorClass(err))
		}
		res := c.FetchCAPs(context.Background(), []Item{{GUID: "1792897745415016"}})
		if res[0].Err == nil || ErrorClass(res[0].Err) != "network" {
			t.Errorf("cap: %+v", res[0])
		}
		for _, e := range []error{err, res[0].Err} {
			if strings.Contains(e.Error(), "identifier=") {
				t.Errorf("error holds the query: %q", e)
			}
		}
	})

	t.Run("limits", func(t *testing.T) {
		if RSSMaxBytes != 1<<20 || CAPMaxBytes != 256<<10 || CAPConcurrency != 3 || CAPRetries != 1 ||
			CAPTimeout != 8*time.Second || RSSTimeout != 15*time.Second {
			t.Errorf("a client bound moved")
		}
	})

	t.Run("class of unknown errors", func(t *testing.T) {
		if got := ErrorClass(nil); got != "" {
			t.Errorf("ErrorClass(nil) = %q", got)
		}
		if got := ErrorClass(context.DeadlineExceeded); got != "timeout" {
			t.Errorf("deadline = %q", got)
		}
		if got := ErrorClass(http.ErrAbortHandler); got != "network" {
			t.Errorf("other = %q", got)
		}
	})
}

func TestFetchCAPsConcurrencyAndRetry(t *testing.T) {
	var inFlight, maxInFlight atomic.Int32
	var mu sync.Mutex
	perGUID := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		defer inFlight.Add(-1)
		time.Sleep(25 * time.Millisecond)

		guid := r.URL.Query().Get("identifier")
		mu.Lock()
		perGUID[guid]++
		attempt := perGUID[guid]
		mu.Unlock()

		switch guid {
		case "1000000000000002": // fails once, then works
			if attempt == 1 {
				http.Error(w, "busy", http.StatusServiceUnavailable)
				return
			}
		case "1000000000000004": // fails twice
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		case "1000000000000006": // a client error is not retried
			http.NotFound(w, r)
			return
		}
		serveXML(w, fixture(t, "cap_cwc_flood.xml"))
	}))
	defer srv.Close()

	var items []Item
	for i := 1; i <= 7; i++ {
		items = append(items, Item{GUID: "100000000000000" + strconv.Itoa(i)})
	}
	items = append(items, Item{GUID: "1000000000000008"})
	c := testClient(srv.URL, ClientConfig{})
	res := c.FetchCAPs(context.Background(), items)

	if len(res) != len(items) {
		t.Fatalf("results = %d, want %d", len(res), len(items))
	}
	for i, r := range res {
		if r.Item.GUID != items[i].GUID {
			t.Errorf("result %d is for %q, want %q (order must follow the input)", i, r.Item.GUID, items[i].GUID)
		}
	}
	if got := maxInFlight.Load(); got > CAPConcurrency || got < 2 {
		t.Errorf("max concurrent requests = %d, want between 2 and %d", got, CAPConcurrency)
	}

	byGUID := map[string]CAPResult{}
	for _, r := range res {
		byGUID[r.Item.GUID] = r
	}
	if r := byGUID["1000000000000002"]; r.Err != nil || !r.Kept {
		t.Errorf("retried item: %+v", r)
	}
	if r := byGUID["1000000000000004"]; r.Err == nil || ErrorClass(r.Err) != "status" {
		t.Errorf("twice failing item: %+v", r)
	}
	if r := byGUID["1000000000000006"]; r.Err == nil {
		t.Errorf("404 item should fail: %+v", r)
	}
	for _, g := range []string{"1000000000000001", "1000000000000003", "1000000000000005", "1000000000000007", "1000000000000008"} {
		if r := byGUID[g]; r.Err != nil || !r.Kept || r.Version.GUID != g {
			t.Errorf("%s: %+v", g, r)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if perGUID["1000000000000002"] != 2 {
		t.Errorf("retried item requested %d times, want 2", perGUID["1000000000000002"])
	}
	if perGUID["1000000000000004"] != 1+CAPRetries {
		t.Errorf("twice failing item requested %d times, want %d", perGUID["1000000000000004"], 1+CAPRetries)
	}
	if perGUID["1000000000000006"] != 1 {
		t.Errorf("404 item requested %d times, want 1", perGUID["1000000000000006"])
	}
}

func TestFetchCAPsMarksUnkeptMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveXML(w, fixture(t, "cap_exercise_synthetic.xml"))
	}))
	defer srv.Close()
	res := testClient(srv.URL, ClientConfig{}).FetchCAPs(context.Background(), []Item{{GUID: "1792000000000001"}})
	if res[0].Err != nil || res[0].Kept {
		t.Errorf("an Exercise message should parse and not be kept: %+v", res[0])
	}
}

func TestFetchCAPsStopsWhenContextEnds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveXML(w, fixture(t, "cap_cwc_flood.xml"))
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	items := []Item{{GUID: "1000000000000001"}, {GUID: "1000000000000002"}}
	res := testClient(srv.URL, ClientConfig{}).FetchCAPs(ctx, items)
	for i, r := range res {
		if r.Err == nil {
			t.Errorf("item %d: want an error once the context has ended", i)
		}
		if r.Item.GUID != items[i].GUID {
			t.Errorf("item %d result is for %q", i, r.Item.GUID)
		}
	}
}
