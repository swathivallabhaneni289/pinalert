package main

import (
	"testing"

	"pinalert/internal/api"
)

// TestSearchBudgetsMatchTheRouterDefaults keeps the two search budgets this
// program sets equal to the router's own defaults (quick 261016-sne), so the
// numbers a reader finds beside the defaults are the ones that run.
func TestSearchBudgetsMatchTheRouterDefaults(t *testing.T) {
	geocode := api.GeocodeRateLimit{Burst: geocodeRateLimitBurst, Every: geocodeRateLimitEvery}
	if geocode != api.GeocodeRateLimitDefault {
		t.Errorf("report search budget = %+v, want %+v", geocode, api.GeocodeRateLimitDefault)
	}
	place := api.PlaceRateLimit{Burst: placeRateLimitBurst, Every: placeRateLimitEvery}
	if place != api.PlaceRateLimitDefault {
		t.Errorf("place search budget = %+v, want %+v", place, api.PlaceRateLimitDefault)
	}
}
