package web

// home_search_not_found_contract_test.go pins the page side of plan 03.4-34: the
// home area search in step 4 and in Settings asks the person to add their town
// when the server finds no close match and places no typed row, and it never
// sends a map centre (the configured fallback centre, today Bengaluru, must not
// pull the search).

import (
	"strings"
	"testing"
)

const notFoundLine = "'Not found. Add your town, for example: Gayathri Nagar, Vijayawada'"

func TestHomeAreaSearchesUseTheNotFoundLine(t *testing.T) {
	for _, file := range []string{"js/welcome.js", "js/settings.js"} {
		t.Run(file, func(t *testing.T) {
			src := p421ReadStatic(t, file)
			if n := strings.Count(src, notFoundLine); n != 1 {
				t.Errorf("%s holds the not found sentence %d times, want 1", file, n)
			}
			if strings.Contains(src, "No matches found") {
				t.Errorf("%s still holds the old No matches found sentence", file)
			}
			// Only a body flag that is exactly true counts.
			p13MustContain(t, file, src, "no_match === true")
			// The sentence has no dash of any kind.
			for _, dash := range []string{"—", "–"} {
				if strings.Contains(notFoundLine, dash) {
					t.Errorf("the not found sentence holds a dash")
				}
			}
		})
	}
}

func TestHomeAreaSearchesNeverSendACentre(t *testing.T) {
	for _, file := range []string{"js/welcome.js", "js/settings.js"} {
		t.Run(file, func(t *testing.T) {
			src := p421ReadStatic(t, file)
			p13MustContain(t, file, src, "'/api/geocode/place?kind=area&q='")
			for _, banned := range []string{"fallbackLat", "fallbackLon", "data-fallback", "lat=", "lon=", "&lat", "&lon"} {
				if strings.Contains(src, banned) {
					t.Errorf("%s holds %q: the home area search must carry no centre", file, banned)
				}
			}
		})
	}
}
