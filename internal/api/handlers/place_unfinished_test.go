package handlers

// Tests for Phase 3.4 plan 35: the place search says unfinished when a lookup
// the areas search needed could not be answered. Only kind=area ever carries it,
// and it never goes with no_match.

import (
	"context"
	"net/url"
	"testing"

	"pinalert/internal/geocode"
)

type unfinishedSearcher struct {
	stubPlaceSearcher
	unfinished bool
}

func (s *unfinishedSearcher) SearchReader(ctx context.Context, q string, near *geocode.Point) (geocode.Answer, error) {
	answer, err := s.stubPlaceSearcher.SearchReader(ctx, q, near)
	answer.Unfinished = s.unfinished
	return answer, err
}

func (s *unfinishedSearcher) SearchReaderAreas(ctx context.Context, q string, near *geocode.Point) (geocode.Answer, error) {
	s.areaCalls++
	return s.SearchReader(ctx, q, near)
}

func TestPlaceAreaSearchCarriesUnfinishedWhenTheSearchSaysSo(t *testing.T) {
	s := &unfinishedSearcher{unfinished: true, stubPlaceSearcher: stubPlaceSearcher{results: []geocode.Result{{Name: "Nethaji Circle", DisplayName: "Nethaji Circle, Bengaluru", Lat: 13.03, Lon: 77.56}}}}
	raw := placeBody(t, url.Values{"q": {"Benz Circle"}, "kind": {"area"}}, s)
	if raw["unfinished"] != true {
		t.Errorf("unfinished = %v, want true; body %v", raw["unfinished"], raw)
	}
	if _, has := raw["no_match"]; has {
		t.Errorf("no_match present beside unfinished: %v", raw)
	}
}

func TestPlaceAreaSearchLeavesUnfinishedOutWhenFalse(t *testing.T) {
	s := &unfinishedSearcher{}
	raw := placeBody(t, url.Values{"q": {"Koramangala"}, "kind": {"area"}}, s)
	if _, has := raw["unfinished"]; has {
		t.Errorf("unfinished present: %v", raw)
	}
}

func TestPlaceSearchWithoutKindNeverCarriesUnfinished(t *testing.T) {
	s := &unfinishedSearcher{unfinished: true}
	raw := placeBody(t, url.Values{"q": {"Benz Circle"}}, s)
	if _, has := raw["unfinished"]; has {
		t.Errorf("the normal place search carried unfinished: %v", raw)
	}
}
