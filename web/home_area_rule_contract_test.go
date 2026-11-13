package web

// home_area_rule_contract_test.go pins the client half of "a town or area,
// never a street" (Phase 7.1 plan 06, SC4, revision W14). The account service
// refuses a home area name that is a numbered street address, but the place
// search is what keeps streets, buildings and point places from being offered at
// all: both pages that pick a home area, the first-time flow's step 4 and
// Settings, must build their place search with kind=area (the areas only search
// of internal/geocode/areas.go, whose never a street tests are
// TestAreasPhotonNeverReturnsStreetsBuildingsOrPointPlaces and
// TestAreasNominatimNeverReturnsStreetsOrBuildings). If either page drops
// kind=area the server check alone would be the only guard.

import (
	"regexp"
	"strings"
	"testing"
)

func TestHomeAreaSearchIsAreasOnly(t *testing.T) {
	const place = "/api/geocode/place"
	for _, file := range []string{"js/welcome.js", "js/profile-edit.js"} {
		js := p421ReadStatic(t, file)
		if !strings.Contains(js, place+"?kind=area&q=") {
			t.Errorf("%s does not build its place search with %s?kind=area&q=", file, place)
		}
		// Every place search URL in the file must carry kind=area: a second
		// URL without it would be an unfiltered search.
		for _, m := range regexp.MustCompile(regexp.QuoteMeta(place)+`[^'"\x60]*`).FindAllString(js, -1) {
			if !strings.Contains(m, "kind=area") {
				t.Errorf("%s holds a place search URL without kind=area: %q", file, m)
			}
		}
	}
}
