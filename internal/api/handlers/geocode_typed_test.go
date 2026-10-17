package handlers

// Tests for Phase 3.4 plan 32, task 1: the typed row of the home area search
// passes through the place handler as typed true and the town it was placed in,
// and a row that is not typed carries neither field.

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"testing"

	"pinalert/internal/geocode"
)

func TestPlaceResultsCarryTheTypedFlagAndTown(t *testing.T) {
	results := []geocode.Result{
		{Name: "Gayathri Nagar, Vijayawada", DisplayName: "Vijayawada", Lat: 16.5115, Lon: 80.616, Typed: true, Town: "Vijayawada"},
		{Name: "Ayodhya Nagar", DisplayName: "Ayodhya Nagar, Vijayawada, Andhra Pradesh", Lat: 16.53, Lon: 80.63, Area: "Ayodhya Nagar"},
	}
	rec := httptest.NewRecorder()
	GeocodePlace(&stubPlaceSearcher{results: results}, &stubPlaceBudget{allow: true})(rec, placeRequest(7, url.Values{"q": {"Gayathri Nagar, Vijayawada"}, "kind": {"area"}}))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var raw struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(raw.Results))
	}
	if raw.Results[0]["typed"] != true || raw.Results[0]["town"] != "Vijayawada" {
		t.Errorf("typed row = %v, want typed true and town Vijayawada", raw.Results[0])
	}
	if raw.Results[0]["name"] != "Gayathri Nagar, Vijayawada" {
		t.Errorf("typed row name = %v", raw.Results[0]["name"])
	}
	for _, key := range []string{"typed", "town"} {
		if _, has := raw.Results[1][key]; has {
			t.Errorf("a row that is not typed carries %q: %v", key, raw.Results[1])
		}
	}
}

func TestReportSearchResultsNeverCarryTheTypedFields(t *testing.T) {
	rec := httptest.NewRecorder()
	Geocode(&stubSearcher{results: []geocode.Result{{Name: "Indiranagar", DisplayName: "Indiranagar, Bengaluru", Lat: 12.98, Lon: 77.64}}}, nil)(rec, geocodeRequest(url.Values{"q": {"Indiranagar"}}))
	var raw struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"typed", "town"} {
		if _, has := raw.Results[0][key]; has {
			t.Errorf("report search row carries %q", key)
		}
	}
}
