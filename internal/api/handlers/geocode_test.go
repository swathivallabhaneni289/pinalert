package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pinalert/internal/geocode"
)

// stubSearcher is a local GeocodeSearcher test double recording the query
// it was called with and returning canned results or a canned error.
type stubSearcher struct {
	results  []geocode.Result
	err      error
	gotQuery string
	calls    int
}

func (s *stubSearcher) Search(ctx context.Context, q string) ([]geocode.Result, error) {
	s.calls++
	s.gotQuery = q
	if s.err != nil {
		return nil, s.err
	}
	return s.results, nil
}

// decodeErrorBody decodes a 4xx/5xx response body into its {error:{field,
// message}} envelope for assertions.
func decodeErrorBody(t *testing.T, rec *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding error body: %v (body: %s)", err, rec.Body.String())
	}
	return body
}

// TestGeocode_ValidatesQueryLength asserts a query outside 3-200 runes is
// refused with 400 before any outbound call.
func TestGeocode_ValidatesQueryLength(t *testing.T) {
	longQuery := strings.Repeat("a", 201)

	cases := []struct {
		name string
		q    string
	}{
		{"empty", ""},
		{"two runes", "ab"},
		{"201 runes", longQuery},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubSearcher{}
			req := httptest.NewRequest(http.MethodGet, "/api/geocode?q="+tc.q, nil)
			rec := httptest.NewRecorder()

			Geocode(stub)(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			body := decodeErrorBody(t, rec)
			if body.Error.Field != "q" {
				t.Fatalf("error.field = %q, want q", body.Error.Field)
			}
			if body.Error.Message == "" {
				t.Fatal("error.message is empty")
			}
			if stub.calls != 0 {
				t.Fatalf("stub.calls = %d, want 0", stub.calls)
			}
		})
	}
}

// TestGeocode_CountsQueryLengthInRunes asserts query-length validation
// counts UTF-8 runes, not bytes — a byte-length check would incorrectly
// accept a 2-rune Devanagari query and reject a legitimate longer one.
func TestGeocode_CountsQueryLengthInRunes(t *testing.T) {
	twoRunes := "कम"   // 2 Devanagari runes, 6 bytes
	threeRunes := "कमल" // 3 Devanagari runes, 9 bytes

	t.Run("2-rune Devanagari refused", func(t *testing.T) {
		stub := &stubSearcher{}
		req := httptest.NewRequest(http.MethodGet, "/api/geocode?q="+twoRunes, nil)
		rec := httptest.NewRecorder()

		Geocode(stub)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if stub.calls != 0 {
			t.Fatalf("stub.calls = %d, want 0", stub.calls)
		}
	})

	t.Run("3-rune Devanagari reaches searcher", func(t *testing.T) {
		stub := &stubSearcher{results: []geocode.Result{}}
		req := httptest.NewRequest(http.MethodGet, "/api/geocode?q="+threeRunes, nil)
		rec := httptest.NewRecorder()

		Geocode(stub)(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if stub.calls != 1 {
			t.Fatalf("stub.calls = %d, want 1", stub.calls)
		}
		if stub.gotQuery != threeRunes {
			t.Fatalf("gotQuery = %q, want %q", stub.gotQuery, threeRunes)
		}
	})
}

// TestGeocode_UpstreamFailureReturnsFriendlyError asserts an upstream
// failure maps to 503 plus D-04's exact fallback message, never a panic or
// a 500.
func TestGeocode_UpstreamFailureReturnsFriendlyError(t *testing.T) {
	stub := &stubSearcher{err: fmt.Errorf("geocode: executing request: %w", context.DeadlineExceeded)}
	req := httptest.NewRequest(http.MethodGet, "/api/geocode?q=Bengaluru", nil)
	rec := httptest.NewRecorder()

	Geocode(stub)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	body := decodeErrorBody(t, rec)
	if body.Error.Field != "q" {
		t.Fatalf("error.field = %q, want q", body.Error.Field)
	}
	if body.Error.Message != geocodeUnavailableMessage {
		t.Fatalf("error.message = %q, want %q", body.Error.Message, geocodeUnavailableMessage)
	}
}

// TestGeocode_NilSearcherReturnsFriendlyError asserts a handler built with
// a nil searcher (e.g. a Deps{} literal that omits the field) degrades to
// the 503 fallback rather than panicking.
func TestGeocode_NilSearcherReturnsFriendlyError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/geocode?q=Bengaluru", nil)
	rec := httptest.NewRecorder()

	Geocode(nil)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	body := decodeErrorBody(t, rec)
	if body.Error.Message != geocodeUnavailableMessage {
		t.Fatalf("error.message = %q, want %q", body.Error.Message, geocodeUnavailableMessage)
	}
}

// TestGeocode_AllowlistsResponseFields asserts the 200 body contains only
// the four allowlisted fields per result, with lat/lon as JSON numbers, and
// none of Nominatim's other fields (T-07-04).
func TestGeocode_AllowlistsResponseFields(t *testing.T) {
	stub := &stubSearcher{results: []geocode.Result{
		{Name: "Bengaluru", DisplayName: "Bengaluru, Karnataka, India", Lat: 12.9767936, Lon: 77.5900820},
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/geocode?q=Bengaluru", nil)
	rec := httptest.NewRecorder()

	Geocode(stub)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	rawBody := rec.Body.String()
	for _, forbidden := range []string{"place_id", "licence", "osm_id", "place_rank", "boundingbox", "importance"} {
		if strings.Contains(rawBody, forbidden) {
			t.Fatalf("body contains forbidden field %q: %s", forbidden, rawBody)
		}
	}

	var decoded struct {
		Results []map[string]json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if len(decoded.Results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(decoded.Results))
	}

	wantKeys := map[string]bool{"name": true, "display_name": true, "lat": true, "lon": true}
	result := decoded.Results[0]
	if len(result) != len(wantKeys) {
		t.Fatalf("result has %d keys, want %d: %v", len(result), len(wantKeys), result)
	}
	for key := range result {
		if !wantKeys[key] {
			t.Fatalf("unexpected key %q in result", key)
		}
	}

	var latRaw, lonRaw float64
	if err := json.Unmarshal(result["lat"], &latRaw); err != nil {
		t.Fatalf("lat is not a JSON number: %v (%s)", err, result["lat"])
	}
	if err := json.Unmarshal(result["lon"], &lonRaw); err != nil {
		t.Fatalf("lon is not a JSON number: %v (%s)", err, result["lon"])
	}
}

// TestGeocode_EmptyResultsSerialiseAsArray asserts an empty result set
// marshals as [] not null, since the browser's "No matches found" path
// (D-04) keys off an empty array.
func TestGeocode_EmptyResultsSerialiseAsArray(t *testing.T) {
	stub := &stubSearcher{results: []geocode.Result{}}
	req := httptest.NewRequest(http.MethodGet, "/api/geocode?q=Xyzzyplugh", nil)
	rec := httptest.NewRecorder()

	Geocode(stub)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	rawBody := rec.Body.String()
	if !strings.Contains(rawBody, `"results":[]`) {
		t.Fatalf("body does not contain \"results\":[]: %s", rawBody)
	}
	if strings.Contains(rawBody, `"results":null`) {
		t.Fatalf("body contains \"results\":null: %s", rawBody)
	}
}

// TestGeocode_SetsPrivateCacheControl asserts the 200 response carries the
// geocodeCacheControl value in its Cache-Control header.
func TestGeocode_SetsPrivateCacheControl(t *testing.T) {
	stub := &stubSearcher{results: []geocode.Result{}}
	req := httptest.NewRequest(http.MethodGet, "/api/geocode?q=Bengaluru", nil)
	rec := httptest.NewRecorder()

	Geocode(stub)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != geocodeCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, geocodeCacheControl)
	}
}
