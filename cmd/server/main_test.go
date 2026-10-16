package main

import (
	"os"
	"strings"
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

// TestOnboardingGateIsSwitchedOn pins that the real program turns the first-time
// flow gate on (Phase 3.4 plan 10, D-27, orchestrator decision 5). The Deps
// literal is built inside main, so the check reads the source: a refactor that
// drops the field would leave every account unasked for consent.
func TestOnboardingGateIsSwitchedOn(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	if n := strings.Count(string(raw), "OnboardingGate: true,"); n != 1 {
		t.Errorf("main.go sets OnboardingGate: true %d times, want exactly 1", n)
	}
}
