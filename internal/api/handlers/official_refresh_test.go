package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pinalert/internal/official"
)

// These tests run the refresh endpoint with no database: a fake refresher that
// counts its calls stands in for the official service (plan 05-12).

// refreshToken is a throwaway test value of the required length.
const refreshToken = "test-only-refresh-token-0123456789abcdef"

// fakeRefresher counts RefreshAll calls and returns a canned answer. When block
// is set it waits for the context to end, as a refresh that outlives its
// deadline does.
type fakeRefresher struct {
	calls atomic.Int32
	out   []official.Result
	block bool
}

func (f *fakeRefresher) RefreshAll(ctx context.Context) []official.Result {
	f.calls.Add(1)
	if f.block {
		<-ctx.Done()
		return []official.Result{{Feed: official.FeedSachet, ErrorClass: "timeout"}}
	}
	return f.out
}

func refreshRequest(method, auth string) *http.Request {
	req := httptest.NewRequest(method, "/internal/official/refresh", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

// TestRefreshEndpointUnsetIs404: with no token every method answers 404 with the
// bytes http.NotFound writes for an unknown route, and nothing is refreshed.
func TestRefreshEndpointUnsetIs404(t *testing.T) {
	want := httptest.NewRecorder()
	http.NotFound(want, refreshRequest(http.MethodGet, ""))

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete} {
		svc := &fakeRefresher{}
		h := OfficialRefresh(svc, "")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, refreshRequest(method, "Bearer "+refreshToken))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s with no token set = %d, want 404", method, rec.Code)
		}
		if method != http.MethodHead && rec.Body.String() != want.Body.String() {
			t.Errorf("%s body = %q, want the unknown route body %q", method, rec.Body.String(), want.Body.String())
		}
		if got := svc.calls.Load(); got != 0 {
			t.Errorf("%s with no token set ran %d refreshes, want 0", method, got)
		}
	}
}

// TestRefreshEndpointMethods: with a token set, only POST is served. Every other
// method answers 404 (never 405), even with the right token, and refreshes nothing.
func TestRefreshEndpointMethods(t *testing.T) {
	want := httptest.NewRecorder()
	http.NotFound(want, refreshRequest(http.MethodGet, ""))

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions} {
		svc := &fakeRefresher{}
		h := OfficialRefresh(svc, refreshToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, refreshRequest(method, "Bearer "+refreshToken))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", method, rec.Code)
		}
		if method != http.MethodHead && rec.Body.String() != want.Body.String() {
			t.Errorf("%s body = %q, want the unknown route body", method, rec.Body.String())
		}
		if got := rec.Header().Get("Allow"); got != "" {
			t.Errorf("%s set Allow %q: it would tell a stranger the route exists", method, got)
		}
		if got := svc.calls.Load(); got != 0 {
			t.Errorf("%s ran %d refreshes, want 0", method, got)
		}
	}

	svc := &fakeRefresher{}
	rec := httptest.NewRecorder()
	OfficialRefresh(svc, refreshToken).ServeHTTP(rec, refreshRequest(http.MethodPost, "Bearer "+refreshToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST with the token = %d, want 200", rec.Code)
	}
	if got := svc.calls.Load(); got != 1 {
		t.Errorf("POST ran %d refreshes, want 1", got)
	}
}

// TestRefreshEndpointToken: a missing, wrong, differently schemed or nearly right
// token answers 401 and refreshes nothing; the right one answers 200.
func TestRefreshEndpointToken(t *testing.T) {
	bad := map[string]string{
		"no header":          "",
		"wrong token":        "Bearer wrong",
		"basic scheme":       "Basic " + refreshToken,
		"lowercase scheme":   "bearer " + refreshToken,
		"no space":           "Bearer" + refreshToken,
		"one char dropped":   "Bearer " + refreshToken[:len(refreshToken)-1],
		"one char extra":     "Bearer " + refreshToken + "x",
		"one char different": "Bearer " + refreshToken[:len(refreshToken)-1] + "z",
		"token only":         refreshToken,
		"empty bearer":       "Bearer ",
	}
	for name, auth := range bad {
		t.Run(name, func(t *testing.T) {
			svc := &fakeRefresher{}
			rec := httptest.NewRecorder()
			OfficialRefresh(svc, refreshToken).ServeHTTP(rec, refreshRequest(http.MethodPost, auth))
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if got := svc.calls.Load(); got != 0 {
				t.Errorf("a refused call ran %d refreshes, want 0", got)
			}
			if strings.Contains(rec.Body.String(), refreshToken) || strings.Contains(rec.Header().Get("WWW-Authenticate"), refreshToken) {
				t.Errorf("the refusal repeats the token")
			}
		})
	}

	svc := &fakeRefresher{}
	rec := httptest.NewRecorder()
	OfficialRefresh(svc, refreshToken).ServeHTTP(rec, refreshRequest(http.MethodPost, "Bearer "+refreshToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("right token = %d, want 200", rec.Code)
	}
	if got := svc.calls.Load(); got != 1 {
		t.Errorf("right token ran %d refreshes, want 1", got)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
}

// TestRefreshEndpointNoRefresher: a token with no service behind it is a plain
// 503 and never a panic.
func TestRefreshEndpointNoRefresher(t *testing.T) {
	rec := httptest.NewRecorder()
	OfficialRefresh(nil, refreshToken).ServeHTTP(rec, refreshRequest(http.MethodPost, "Bearer "+refreshToken))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

// TestRefreshEndpointSummaryWords: the 200 body is a feeds array whose entries
// carry feed, skipped, items, stored and error, error being "" or one closed
// word. No URL, no feed text and no token appear.
func TestRefreshEndpointSummaryWords(t *testing.T) {
	svc := &fakeRefresher{out: []official.Result{
		{Feed: official.FeedSachet, ItemsSeen: 12, Stored: 3},
		{Feed: official.FeedGdacs, Skipped: true},
		{Feed: "other", ErrorClass: "status"},
		{Feed: "more", ErrorClass: "https://example.invalid/secret?token=1 broke"},
	}}
	rec := httptest.NewRecorder()
	OfficialRefresh(svc, refreshToken).ServeHTTP(rec, refreshRequest(http.MethodPost, "Bearer "+refreshToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}

	var raw struct {
		Feeds []map[string]any `json:"feeds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if len(raw.Feeds) != 4 {
		t.Fatalf("%d feeds in the summary, want 4", len(raw.Feeds))
	}
	for _, f := range raw.Feeds {
		for _, key := range []string{"feed", "skipped", "items", "stored", "error"} {
			if _, ok := f[key]; !ok {
				t.Errorf("feed entry %v lacks %q", f, key)
			}
		}
		if len(f) != 5 {
			t.Errorf("feed entry %v has %d keys, want exactly 5", f, len(f))
		}
	}

	var sum RefreshSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	closed := map[string]bool{"": true, "timeout": true, "status": true, "parse": true, "anomaly": true, "network": true}
	for _, f := range sum.Feeds {
		if !closed[f.Error] {
			t.Errorf("feed %s error %q is not a closed word", f.Feed, f.Error)
		}
	}
	if got := sum.Feeds[0]; got.Feed != "sachet" || got.Items != 12 || got.Stored != 3 || got.Skipped || got.Error != "" {
		t.Errorf("first entry = %+v", got)
	}
	if got := sum.Feeds[1]; !got.Skipped {
		t.Errorf("second entry = %+v, want skipped", got)
	}
	if got := sum.Feeds[2]; got.Error != "status" {
		t.Errorf("third entry error = %q, want status", got.Error)
	}
	if got := sum.Feeds[3]; got.Error != "network" {
		t.Errorf("an unknown class word became %q, want network", got.Error)
	}
	body := rec.Body.String()
	for _, banned := range []string{"http", "example.invalid", refreshToken} {
		if strings.Contains(body, banned) {
			t.Errorf("summary holds %q: %s", banned, body)
		}
	}
}

// TestRefreshEndpointEmptySummary: with no feed registered the summary is an
// empty array, not null.
func TestRefreshEndpointEmptySummary(t *testing.T) {
	rec := httptest.NewRecorder()
	OfficialRefresh(&fakeRefresher{}, refreshToken).ServeHTTP(rec, refreshRequest(http.MethodPost, "Bearer "+refreshToken))
	if got := strings.TrimSpace(rec.Body.String()); got != `{"feeds":[]}` {
		t.Errorf("body = %s, want {\"feeds\":[]}", got)
	}
}

// TestRefreshEndpointDeadline: a refresh that outlives the deadline is cancelled
// by it and the answer still arrives.
func TestRefreshEndpointDeadline(t *testing.T) {
	if RefreshDeadline != 55*time.Second {
		t.Fatalf("RefreshDeadline = %v, want 55s", RefreshDeadline)
	}
	svc := &fakeRefresher{block: true}
	h := officialRefresh(svc, refreshToken, 50*time.Millisecond)

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, refreshRequest(http.MethodPost, "Bearer "+refreshToken))
		done <- rec
	}()
	select {
	case rec := <-done:
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var sum RefreshSummary
		if err := json.Unmarshal(rec.Body.Bytes(), &sum); err != nil {
			t.Fatal(err)
		}
		if len(sum.Feeds) != 1 || sum.Feeds[0].Error != "timeout" {
			t.Errorf("summary = %+v, want one feed with the timeout word", sum)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the answer never arrived after the deadline")
	}
}

// TestValidateRefreshToken: empty is allowed (the route is off), under 32
// characters is refused with a message that never holds the value.
func TestValidateRefreshToken(t *testing.T) {
	if err := ValidateRefreshToken(""); err != nil {
		t.Errorf("an empty token gave %v, want nil", err)
	}
	if err := ValidateRefreshToken(strings.Repeat("a", RefreshTokenMinLen)); err != nil {
		t.Errorf("a %d character token gave %v, want nil", RefreshTokenMinLen, err)
	}
	short := strings.Repeat("s", RefreshTokenMinLen-1)
	err := ValidateRefreshToken(short)
	if err == nil {
		t.Fatal("a 31 character token was accepted")
	}
	if err.Error() != "OFFICIAL_REFRESH_TOKEN must be at least 32 characters" {
		t.Errorf("message = %q", err.Error())
	}
	if strings.Contains(err.Error(), short) {
		t.Error("the message repeats the token")
	}
}
