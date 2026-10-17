package handlers

// Tests for Phase 3.4 plan 28, task 2: the optional kind=area mode of
// GET /api/geocode/place, used by the home area fields. A call without kind is
// the place search it always was.

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"pinalert/internal/geocode"
)

func TestGeocodePlaceKindArea(t *testing.T) {
	results := []geocode.Result{{Name: "Indiranagar", DisplayName: "Indiranagar, Bengaluru, Karnataka", Lat: 12.98, Lon: 77.64}}

	t.Run("kind=area runs the areas only search on the same budget key", func(t *testing.T) {
		searcher := &stubPlaceSearcher{results: results}
		budget := &stubPlaceBudget{allow: true}
		rec := httptest.NewRecorder()
		GeocodePlace(searcher, budget)(rec, placeRequest(7, url.Values{"q": {"indira"}, "kind": {"area"}}))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if searcher.areaCalls != 1 || searcher.calls != 1 {
			t.Fatalf("areaCalls = %d, calls = %d, want the areas search once", searcher.areaCalls, searcher.calls)
		}
		if len(budget.keys) != 1 || budget.keys[0] != "place:7" {
			t.Fatalf("budget keys = %q, want [place:7]: the areas search counts on the place budget", budget.keys)
		}
		if searcher.gotQ != "indira" {
			t.Fatalf("q = %q", searcher.gotQ)
		}
	})

	t.Run("no kind runs the normal search", func(t *testing.T) {
		searcher := &stubPlaceSearcher{results: results}
		rec := httptest.NewRecorder()
		GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(7, url.Values{"q": {"indira"}}))
		if rec.Code != http.StatusOK || searcher.calls != 1 || searcher.areaCalls != 0 {
			t.Fatalf("status = %d, calls = %d, areaCalls = %d, want 200, 1, 0", rec.Code, searcher.calls, searcher.areaCalls)
		}
	})

	t.Run("an unknown kind is a 400 and nothing is searched", func(t *testing.T) {
		for _, kind := range []string{"street", "Area", "areas"} {
			searcher := &stubPlaceSearcher{results: results}
			rec := httptest.NewRecorder()
			GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(7, url.Values{"q": {"indira"}, "kind": {kind}}))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("kind %q: status = %d, want 400", kind, rec.Code)
			}
			if body := decodeErrorBody(t, rec); body.Error.Field != "kind" {
				t.Fatalf("kind %q: error.field = %q, want kind", kind, body.Error.Field)
			}
			if searcher.calls != 0 {
				t.Fatalf("kind %q: searcher called %d times, want 0", kind, searcher.calls)
			}
		}
	})

	t.Run("an empty kind is the normal search", func(t *testing.T) {
		searcher := &stubPlaceSearcher{results: results}
		rec := httptest.NewRecorder()
		GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(7, url.Values{"q": {"indira"}, "kind": {""}}))
		if rec.Code != http.StatusOK || searcher.areaCalls != 0 || searcher.calls != 1 {
			t.Fatalf("status = %d, calls = %d, areaCalls = %d, want 200, 1, 0", rec.Code, searcher.calls, searcher.areaCalls)
		}
	})

	t.Run("a refused budget still answers 429 for kind=area", func(t *testing.T) {
		searcher := &stubPlaceSearcher{results: results}
		rec := httptest.NewRecorder()
		GeocodePlace(searcher, &stubPlaceBudget{allow: false})(rec, placeRequest(7, url.Values{"q": {"indira"}, "kind": {"area"}}))
		if rec.Code != http.StatusTooManyRequests || searcher.calls != 0 {
			t.Fatalf("status = %d, calls = %d, want 429, 0", rec.Code, searcher.calls)
		}
	})

	t.Run("a failed areas search answers the same 503", func(t *testing.T) {
		searcher := &stubPlaceSearcher{err: geocode.ErrReaderBusy}
		rec := httptest.NewRecorder()
		GeocodePlace(searcher, &stubPlaceBudget{allow: true})(rec, placeRequest(7, url.Values{"q": {"indira"}, "kind": {"area"}}))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		if body := decodeErrorBody(t, rec); body.Error.Message != "Search unavailable. Try again in a moment." {
			t.Fatalf("error.message = %q", body.Error.Message)
		}
	})
}
