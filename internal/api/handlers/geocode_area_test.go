package handlers

// Tests for Phase 3.4 plan 28, task 4: both searches pass the area of each
// result through, so step 3 of the first time flow can prefill the home area
// with an area and not with a street or a building. The field is left out when
// no area is known.

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"testing"

	"pinalert/internal/geocode"
)

func TestSearchResultsCarryTheArea(t *testing.T) {
	results := []geocode.Result{
		{Name: "MG Road", DisplayName: "MG Road, Shivajinagar, Bengaluru, Karnataka", Lat: 12.97, Lon: 77.60, Area: "Shivajinagar"},
		{Name: "Indiranagar", DisplayName: "Indiranagar, Bengaluru, Karnataka", Lat: 12.98, Lon: 77.64, Area: "Indiranagar"},
		{Name: "Lonely Road", DisplayName: "Lonely Road, Karnataka", Lat: 13.0, Lon: 77.0},
	}
	reporter := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		Geocode(&stubSearcher{results: results}, nil)(rec, geocodeRequest(url.Values{"q": {"anything"}}))
		return rec
	}
	reader := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		GeocodePlace(&stubPlaceSearcher{results: results}, &stubPlaceBudget{allow: true})(rec, placeRequest(7, url.Values{"q": {"anything"}}))
		return rec
	}
	for name, run := range map[string]func() *httptest.ResponseRecorder{"/api/geocode": reporter, "/api/geocode/place": reader} {
		t.Run(name, func(t *testing.T) {
			rec := run()
			if rec.Code != 200 {
				t.Fatalf("status = %d", rec.Code)
			}
			var raw struct {
				Results []map[string]any `json:"results"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(raw.Results) != 3 {
				t.Fatalf("got %d results, want 3", len(raw.Results))
			}
			if got := raw.Results[0]["area"]; got != "Shivajinagar" {
				t.Errorf("street area = %v, want Shivajinagar", got)
			}
			if got := raw.Results[1]["area"]; got != "Indiranagar" {
				t.Errorf("area area = %v, want Indiranagar", got)
			}
			if _, has := raw.Results[2]["area"]; has {
				t.Errorf("a result with no known area carries an area field: %v", raw.Results[2])
			}
		})
	}
}
