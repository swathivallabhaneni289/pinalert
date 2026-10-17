package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"pinalert/internal/account"
	"pinalert/internal/geocode"
	"pinalert/internal/ratelimit"
)

// stubPlaceSearcher is a PlaceSearcher test double recording what it was
// called with and returning canned results or a canned error.
type stubPlaceSearcher struct {
	results []geocode.Result
	matched string
	err     error
	calls   int
	// areaCalls counts the calls that came through SearchReaderAreas (they
	// also count in calls).
	areaCalls int
	gotQ      string
	gotNear *geocode.Point
}

func (s *stubPlaceSearcher) SearchReader(ctx context.Context, q string, near *geocode.Point) (geocode.Answer, error) {
	s.calls++
	s.gotQ = q
	s.gotNear = near
	if s.err != nil {
		return geocode.Answer{}, s.err
	}
	return geocode.Answer{Results: s.results, MatchedQuery: s.matched}, nil
}

// SearchReaderAreas is the areas only search (plan 28). It records the same
// things as SearchReader and counts itself in areaCalls too, so a test can tell
// which of the two the handler chose.
func (s *stubPlaceSearcher) SearchReaderAreas(ctx context.Context, q string, near *geocode.Point) (geocode.Answer, error) {
	s.areaCalls++
	return s.SearchReader(ctx, q, near)
}

// stubPlaceBudget is a PlaceBudget test double recording every key it saw.
type stubPlaceBudget struct {
	allow bool
	keys  []string
}

func (b *stubPlaceBudget) Allow(key string) bool {
	b.keys = append(b.keys, key)
	return b.allow
}

// placeRequest builds a GET /api/geocode/place request for account id (0
// means no account in the context).
func placeRequest(accountID int64, query url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/geocode/place?"+query.Encode(), nil)
	if accountID != 0 {
		req = req.WithContext(account.WithAccount(req.Context(), account.Account{ID: accountID, Email: "a@example.com"}))
	}
	return req
}

func TestGeocodePlaceMessages(t *testing.T) {
	t.Run("no account answers 500", func(t *testing.T) {
		searcher := &stubPlaceSearcher{}
		rec := httptest.NewRecorder()
		GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(0, url.Values{"q": {"velachery"}}))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		if searcher.calls != 0 {
			t.Fatalf("searcher called %d times, want 0", searcher.calls)
		}
	})

	t.Run("budget refused answers 429 with the plain message", func(t *testing.T) {
		searcher := &stubPlaceSearcher{}
		rec := httptest.NewRecorder()
		GeocodePlace(searcher, &stubPlaceBudget{allow: false})(rec, placeRequest(7, url.Values{"q": {"velachery"}}))
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429", rec.Code)
		}
		body := decodeErrorBody(t, rec)
		if body.Error.Field != "q" {
			t.Fatalf("error.field = %q, want q", body.Error.Field)
		}
		if body.Error.Message != "Too many searches. Wait a few seconds and try again." {
			t.Fatalf("error.message = %q", body.Error.Message)
		}
		if searcher.calls != 0 {
			t.Fatalf("searcher called %d times, want 0", searcher.calls)
		}
	})

	for name, err := range map[string]error{
		"busy lane":      geocode.ErrReaderBusy,
		"upstream error": errors.New("boom"),
	} {
		t.Run(name+" answers 503 with the plain message", func(t *testing.T) {
			searcher := &stubPlaceSearcher{err: err}
			rec := httptest.NewRecorder()
			GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(7, url.Values{"q": {"velachery"}}))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
			body := decodeErrorBody(t, rec)
			if body.Error.Field != "q" {
				t.Fatalf("error.field = %q, want q", body.Error.Field)
			}
			if body.Error.Message != "Search unavailable. Try again in a moment." {
				t.Fatalf("error.message = %q", body.Error.Message)
			}
		})
	}

	t.Run("nil searcher answers 503 after the budget", func(t *testing.T) {
		budget := &stubPlaceBudget{allow: true}
		rec := httptest.NewRecorder()
		GeocodePlace(nil, budget)(rec, placeRequest(7, url.Values{"q": {"velachery"}}))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		if body := decodeErrorBody(t, rec); body.Error.Message != "Search unavailable. Try again in a moment." {
			t.Fatalf("error.message = %q", body.Error.Message)
		}
		if len(budget.keys) != 1 {
			t.Fatalf("budget consulted %d times, want 1", len(budget.keys))
		}
	})

	t.Run("nil searcher with a refused budget still answers 429", func(t *testing.T) {
		rec := httptest.NewRecorder()
		GeocodePlace(nil, &stubPlaceBudget{allow: false})(rec, placeRequest(7, url.Values{"q": {"velachery"}}))
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429", rec.Code)
		}
	})
}

func TestGeocodePlaceBudgetIsPerAccount(t *testing.T) {
	budget := ratelimit.NewPerIP(time.Hour, 2)
	searcher := &stubPlaceSearcher{results: []geocode.Result{}}
	handler := GeocodePlace(searcher, budget)

	status := func(accountID int64) int {
		rec := httptest.NewRecorder()
		handler(rec, placeRequest(accountID, url.Values{"q": {"velachery"}}))
		return rec.Code
	}

	if got := status(1); got != http.StatusOK {
		t.Fatalf("account 1 first = %d, want 200", got)
	}
	if got := status(1); got != http.StatusOK {
		t.Fatalf("account 1 second = %d, want 200", got)
	}
	if got := status(1); got != http.StatusTooManyRequests {
		t.Fatalf("account 1 third = %d, want 429", got)
	}
	if got := status(2); got != http.StatusOK {
		t.Fatalf("account 2 first = %d, want 200 (budget is per account)", got)
	}

	// The key is the account, never the client address.
	stub := &stubPlaceBudget{allow: true}
	req := placeRequest(42, url.Values{"q": {"velachery"}})
	req.RemoteAddr = "203.0.113.9:4444"
	GeocodePlace(searcher, stub)(httptest.NewRecorder(), req)
	if len(stub.keys) != 1 {
		t.Fatalf("budget consulted %d times, want 1", len(stub.keys))
	}
	if stub.keys[0] != "place:42" {
		t.Fatalf("budget key = %q, want place:42", stub.keys[0])
	}
}

func TestGeocodePlaceValidatesInput(t *testing.T) {
	longQ := make([]byte, 201)
	for i := range longQ {
		longQ[i] = 'a'
	}

	bad := []struct {
		name  string
		query url.Values
		field string
	}{
		{"q too short", url.Values{"q": {"ab"}}, "q"},
		{"q missing", url.Values{}, "q"},
		{"q too long", url.Values{"q": {string(longQ)}}, "q"},
		{"lat without lon", url.Values{"q": {"velachery"}, "lat": {"13.08"}}, "lon"},
		{"lon without lat", url.Values{"q": {"velachery"}, "lon": {"80.27"}}, "lat"},
		{"lat not a number", url.Values{"q": {"velachery"}, "lat": {"abc"}, "lon": {"80.27"}}, "lat"},
		{"lat NaN", url.Values{"q": {"velachery"}, "lat": {"NaN"}, "lon": {"80.27"}}, "lat"},
		{"lat infinite", url.Values{"q": {"velachery"}, "lat": {"Inf"}, "lon": {"80.27"}}, "lat"},
		{"lat out of range", url.Values{"q": {"velachery"}, "lat": {"91"}, "lon": {"80.27"}}, "lat"},
		{"lon not a number", url.Values{"q": {"velachery"}, "lat": {"13.08"}, "lon": {"x"}}, "lon"},
		{"lon out of range", url.Values{"q": {"velachery"}, "lat": {"13.08"}, "lon": {"-181"}}, "lon"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			searcher := &stubPlaceSearcher{}
			rec := httptest.NewRecorder()
			GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(7, tc.query))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			body := decodeErrorBody(t, rec)
			if body.Error.Field != tc.field {
				t.Fatalf("error.field = %q, want %q", body.Error.Field, tc.field)
			}
			if body.Error.Message == "" {
				t.Fatal("error.message is empty")
			}
			if searcher.calls != 0 {
				t.Fatalf("searcher called %d times, want 0", searcher.calls)
			}
		})
	}

	t.Run("both coordinates reach the searcher as a point", func(t *testing.T) {
		searcher := &stubPlaceSearcher{results: []geocode.Result{}}
		rec := httptest.NewRecorder()
		q := url.Values{"q": {"  velachery "}, "lat": {"13.0827"}, "lon": {"80.2707"}}
		GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(7, q))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if searcher.gotNear == nil || searcher.gotNear.Lat != 13.0827 || searcher.gotNear.Lon != 80.2707 {
			t.Fatalf("near = %+v, want 13.0827, 80.2707", searcher.gotNear)
		}
		if searcher.gotQ != "velachery" {
			t.Fatalf("q = %q, want the trimmed text", searcher.gotQ)
		}
	})

	t.Run("neither coordinate reaches the searcher as nil", func(t *testing.T) {
		searcher := &stubPlaceSearcher{results: []geocode.Result{}}
		rec := httptest.NewRecorder()
		GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(7, url.Values{"q": {"velachery"}}))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if searcher.calls != 1 || searcher.gotNear != nil {
			t.Fatalf("calls = %d, near = %+v, want one call with nil", searcher.calls, searcher.gotNear)
		}
	})
}

func TestGeocodePlaceAllowlistsFields(t *testing.T) {
	searcher := &stubPlaceSearcher{results: []geocode.Result{
		{Name: "Velachery", DisplayName: "Velachery, Chennai, Tamil Nadu, India", Lat: 12.9791, Lon: 80.2212},
	}}
	rec := httptest.NewRecorder()
	GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(7, url.Values{"q": {"velachery"}}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Fatalf("Cache-Control = %q, want private, max-age=300", got)
	}

	var raw struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if len(raw.Results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(raw.Results))
	}
	want := map[string]bool{"name": true, "display_name": true, "lat": true, "lon": true}
	if len(raw.Results[0]) != len(want) {
		t.Fatalf("result has %d fields, want %d: %v", len(raw.Results[0]), len(want), raw.Results[0])
	}
	for key := range raw.Results[0] {
		if !want[key] {
			t.Fatalf("unexpected field %q in result", key)
		}
	}

	t.Run("empty result list marshals as an array", func(t *testing.T) {
		empty := &stubPlaceSearcher{}
		rec := httptest.NewRecorder()
		GeocodePlace(empty, &stubPlaceBudget{allow: true})(rec, placeRequest(7, url.Values{"q": {"nowhere"}}))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Body.String(); got != "{\"results\":[]}\n" && got != "{\"results\":[]}" {
			t.Fatalf("body = %q, want an empty results array", got)
		}
	})
}

// TestGeocodePlaceFailureLogHoldsNoReaderData pins IN-04: when the upstream
// fails, the log line must carry a fixed message and the error class only,
// never the request URL, the search text, the contact email or a coordinate.
func TestGeocodePlaceFailureLogHoldsNoReaderData(t *testing.T) {
	const (
		secretQuery = "flooded-colony-near-my-house"
		secretEmail = "owner-contact@example.org"
		secretLat   = "13.082680"
		secretLon   = "80.270718"
	)
	rawURL := "https://nominatim.example/search?q=" + secretQuery + "&email=" + secretEmail +
		"&viewbox=" + secretLon + "," + secretLat
	cases := map[string]error{
		"wrapped url error": fmt.Errorf("geocode: executing request: %w",
			&url.Error{Op: "Get", URL: rawURL, Err: errors.New("connection refused")}),
		"plain text error":  errors.New("failed calling " + rawURL),
		"deadline in url":   &url.Error{Op: "Get", URL: rawURL, Err: context.DeadlineExceeded},
		"upstream status":   &geocode.UpstreamStatusError{Status: 429},
		"reader lane busy":  geocode.ErrReaderBusy,
		"cancelled request": context.Canceled,
	}
	for name, failure := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			t.Cleanup(func() { log.SetOutput(os.Stderr) })

			searcher := &stubPlaceSearcher{err: failure}
			rec := httptest.NewRecorder()
			query := url.Values{"q": {secretQuery}, "lat": {secretLat}, "lon": {secretLon}}
			GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(7, query))

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
			logged := buf.String()
			if logged == "" {
				t.Fatal("nothing was logged for a failed search")
			}
			for _, secret := range []string{secretQuery, secretEmail, secretLat, secretLon, "nominatim.example", "viewbox"} {
				if strings.Contains(logged, secret) {
					t.Fatalf("log line %q contains %q", logged, secret)
				}
			}
		})
	}
}
