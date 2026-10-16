package handlers

// Handler tests for quick task 261016-adr: the report form's address search
// takes the pin as optional lat and lon, and both searches say which shorter
// text matched.

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"pinalert/internal/account"
	"pinalert/internal/geocode"
)

// geocodeRequest builds a GET /api/geocode request from a verified account,
// which the handler needs for its per account budget (quick 261016-sne).
func geocodeRequest(query url.Values) *http.Request {
	return geocodeRequestAs(7, query)
}

// geocodeRequestAs is geocodeRequest for a given account id.
func geocodeRequestAs(accountID int64, query url.Values) *http.Request {
	return withAccount(httptest.NewRequest(http.MethodGet, "/api/geocode?"+query.Encode(), nil), accountID)
}

// withAccount puts a verified account in the request's context.
func withAccount(req *http.Request, accountID int64) *http.Request {
	return req.WithContext(account.WithAccount(req.Context(), account.Account{ID: accountID, Email: "a@example.com"}))
}

func TestGeocodePassesThePinAsNear(t *testing.T) {
	t.Run("with lat and lon", func(t *testing.T) {
		stub := &stubSearcher{results: []geocode.Result{}}
		rec := httptest.NewRecorder()
		Geocode(stub, nil)(rec, geocodeRequest(url.Values{"q": {"Banjara Hills"}, "lat": {"17.41560"}, "lon": {"78.43470"}}))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		if stub.gotNear == nil || stub.gotNear.Lat != 17.4156 || stub.gotNear.Lon != 78.4347 {
			t.Fatalf("near = %+v, want 17.4156, 78.4347", stub.gotNear)
		}
	})
	t.Run("without them", func(t *testing.T) {
		stub := &stubSearcher{results: []geocode.Result{}}
		rec := httptest.NewRecorder()
		Geocode(stub, nil)(rec, geocodeRequest(url.Values{"q": {"Banjara Hills"}}))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if stub.calls != 1 || stub.gotNear != nil {
			t.Fatalf("calls %d, near %+v; want 1 call and no position", stub.calls, stub.gotNear)
		}
	})
}

func TestGeocodeValidatesLatLon(t *testing.T) {
	cases := []struct {
		name      string
		query     url.Values
		wantField string
	}{
		{"lat alone", url.Values{"lat": {"17.4"}}, "lon"},
		{"lon alone", url.Values{"lon": {"78.4"}}, "lat"},
		{"lat too big", url.Values{"lat": {"91"}, "lon": {"78.4"}}, "lat"},
		{"lat too small", url.Values{"lat": {"-90.5"}, "lon": {"78.4"}}, "lat"},
		{"lon too big", url.Values{"lat": {"17.4"}, "lon": {"180.01"}}, "lon"},
		{"lat not a number", url.Values{"lat": {"north"}, "lon": {"78.4"}}, "lat"},
		{"lat NaN", url.Values{"lat": {"NaN"}, "lon": {"78.4"}}, "lat"},
		{"lon infinite", url.Values{"lat": {"17.4"}, "lon": {"Inf"}}, "lon"},
		{"empty lat", url.Values{"lat": {""}, "lon": {"78.4"}}, "lat"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubSearcher{results: []geocode.Result{}}
			tc.query.Set("q", "Banjara Hills")
			rec := httptest.NewRecorder()
			Geocode(stub, nil)(rec, geocodeRequest(tc.query))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if body := decodeErrorBody(t, rec); body.Error.Field != tc.wantField || body.Error.Message == "" {
				t.Fatalf("error = %+v, want field %q with a message", body.Error, tc.wantField)
			}
			if stub.calls != 0 {
				t.Fatalf("searcher called %d times, want 0 (a refused request costs Nominatim nothing)", stub.calls)
			}
		})
	}
}

// matchedQueryOf decodes the top level of an answer and reports
// matched_query and whether the key was present at all.
func matchedQueryOf(t *testing.T, body []byte) (string, bool) {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		t.Fatalf("decoding body: %v (%s)", err, body)
	}
	for key := range top {
		if key != "results" && key != "matched_query" {
			t.Fatalf("unexpected top level key %q", key)
		}
	}
	raw, ok := top["matched_query"]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("matched_query is not a string: %s", raw)
	}
	return s, true
}

func TestBothSearchesCarryMatchedQuery(t *testing.T) {
	result := []geocode.Result{{Name: "Banjara Hills", DisplayName: "Banjara Hills, Hyderabad, Telangana, India", Lat: 17.4156, Lon: 78.4347}}
	const matched = "Banjara Hills, Hyderabad"

	reporter := func(matchedText string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		Geocode(&stubSearcher{results: result, matched: matchedText}, nil)(rec,
			geocodeRequest(url.Values{"q": {"Flat 4B, Banjara Hills, Hyderabad"}}))
		return rec
	}
	reader := func(matchedText string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		GeocodePlace(&stubPlaceSearcher{results: result, matched: matchedText}, &stubPlaceBudget{allow: true})(rec,
			placeRequest(7, url.Values{"q": {"Flat 4B, Banjara Hills, Hyderabad"}}))
		return rec
	}

	for name, run := range map[string]func(string) *httptest.ResponseRecorder{"/api/geocode": reporter, "/api/geocode/place": reader} {
		t.Run(name+" shorter text matched", func(t *testing.T) {
			rec := run(matched)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			got, ok := matchedQueryOf(t, rec.Body.Bytes())
			if !ok || got != matched {
				t.Fatalf("matched_query = %q (present %v), want %q", got, ok, matched)
			}
		})
		t.Run(name+" whole text matched", func(t *testing.T) {
			rec := run("")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got, ok := matchedQueryOf(t, rec.Body.Bytes()); ok {
				t.Fatalf("matched_query = %q present, want it left out", got)
			}
		})
	}
}

// TestGeocodeFailureLogHoldsNoPin extends IN-04 to the report form's search
// now that it carries the pin: the log line names the error class only.
func TestGeocodeFailureLogHoldsNoPin(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	failure := &url.Error{Op: "Get", URL: "https://nominatim.example/search?q=flooded-colony&viewbox=78.0,17.9,79.0,16.9", Err: geocode.ErrReaderBusy}
	rec := httptest.NewRecorder()
	Geocode(&stubSearcher{err: failure}, nil)(rec,
		geocodeRequest(url.Values{"q": {"flooded-colony"}, "lat": {"17.41560"}, "lon": {"78.43470"}}))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	line := buf.String()
	for _, secret := range []string{"flooded-colony", "17.4", "78.4", "viewbox", "nominatim.example"} {
		if strings.Contains(line, secret) {
			t.Fatalf("log line %q contains %q", line, secret)
		}
	}
}
