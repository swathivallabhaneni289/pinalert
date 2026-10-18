package handlers

// Tests for Phase 3.4 plan 34: the place search says no_match when the areas
// search found no row that closely matches what was typed and placed no typed
// row. Only kind=area ever carries it.

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"testing"

	"pinalert/internal/geocode"
)

// noMatchSearcher answers every search with NoCloseMatch set, as the areas
// search does; the handler must still keep it off the normal place search.
type noMatchSearcher struct {
	stubPlaceSearcher
	noMatch bool
}

func (s *noMatchSearcher) SearchReader(ctx context.Context, q string, near *geocode.Point) (geocode.Answer, error) {
	answer, err := s.stubPlaceSearcher.SearchReader(ctx, q, near)
	answer.NoCloseMatch = s.noMatch
	return answer, err
}

func (s *noMatchSearcher) SearchReaderAreas(ctx context.Context, q string, near *geocode.Point) (geocode.Answer, error) {
	s.areaCalls++
	return s.SearchReader(ctx, q, near)
}

func placeBody(t *testing.T, query url.Values, s PlaceSearcher) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	GeocodePlace(s, &stubPlaceBudget{allow: true})(rec, placeRequest(7, query))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return raw
}

func TestPlaceAreaSearchCarriesNoMatchWhenTheSearchSaysSo(t *testing.T) {
	s := &noMatchSearcher{noMatch: true, stubPlaceSearcher: stubPlaceSearcher{results: []geocode.Result{{Name: "Nethaji Circle", DisplayName: "Nethaji Circle, Bengaluru", Lat: 13.03, Lon: 77.56}}}}
	raw := placeBody(t, url.Values{"q": {"Benz Circle"}, "kind": {"area"}}, s)
	if raw["no_match"] != true {
		t.Errorf("no_match = %v, want true; body %v", raw["no_match"], raw)
	}
	if rows, _ := raw["results"].([]any); len(rows) != 1 {
		t.Errorf("results = %v, want the one row kept next to the line", raw["results"])
	}
}

func TestPlaceAreaSearchLeavesNoMatchOutWhenFalse(t *testing.T) {
	s := &noMatchSearcher{noMatch: false}
	raw := placeBody(t, url.Values{"q": {"Koramangala"}, "kind": {"area"}}, s)
	if _, has := raw["no_match"]; has {
		t.Errorf("no_match present: %v", raw)
	}
}

func TestPlaceSearchWithoutKindNeverCarriesNoMatch(t *testing.T) {
	s := &noMatchSearcher{noMatch: true}
	raw := placeBody(t, url.Values{"q": {"Benz Circle"}}, s)
	if _, has := raw["no_match"]; has {
		t.Errorf("the normal place search carried no_match: %v", raw)
	}
}
