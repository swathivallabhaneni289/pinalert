package api

import (
	"testing"
	"time"
)

// TestSearchBudgetDefaults pins the per account budgets of both searches
// (quick 261016-sne): 8 searches at once, then 1 a second, sized for one person
// typing one address with a 300 ms pause between searches.
func TestSearchBudgetDefaults(t *testing.T) {
	if want := (GeocodeRateLimit{Burst: 8, Every: time.Second}); GeocodeRateLimitDefault != want {
		t.Errorf("GeocodeRateLimitDefault = %+v, want %+v", GeocodeRateLimitDefault, want)
	}
	if want := (PlaceRateLimit{Burst: 8, Every: time.Second}); PlaceRateLimitDefault != want {
		t.Errorf("PlaceRateLimitDefault = %+v, want %+v", PlaceRateLimitDefault, want)
	}
}
