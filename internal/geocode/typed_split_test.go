package geocode

// Tests for Phase 3.4 plan 34: the typed row of the home area search without a
// comma ("Gayathri Nagar Vijayawada"), placed by a PIN code when no town is
// typed ("Gayathri Nagar 520008"), and the not found flag. Every upstream is an
// httptest stand in; the live answers they replay are in 03.4-34-SUMMARY.md.

import (
	"context"
	"net/url"
	"testing"
)

func ayodhyaNagar() map[string]any {
	return typedRow(80.6316, 16.5302, "Ayodhya Nagar", "place", "suburb", "district", "Vijayawada", "Andhra Pradesh")
}

// postcodeRow is the Photon row for a PIN code (live, 2026-10-18: type other,
// place=postcode, the county carries the district, no city).
func postcodeRow(lon, lat float64, pin, district, county, state string) map[string]any {
	props := map[string]any{
		"name": pin, "state": state, "country": "India", "countrycode": "IN",
		"osm_key": "place", "osm_value": "postcode", "type": "other",
	}
	if district != "" {
		props["district"] = district
	}
	if county != "" {
		props["county"] = county
	}
	return photonFeatureJSON(lon, lat, props)
}

func benguluruCircle() map[string]any {
	return typedRow(77.562, 13.036, "Nethaji Circle", "place", "locality", "locality", "Bengaluru", "Karnataka")
}

func TestParseTypedHomeKeepsThePinAndTheBareText(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantBare string
		wantPin  string
		wantOK   bool
	}{
		{"town after the words, no comma", "Gayathri Nagar Vijayawada", "Gayathri Nagar Vijayawada", "", false},
		{"pin after the words", "Gayathri Nagar 520008", "Gayathri Nagar", "520008", false},
		{"pin after a comma", "Gayathri Nagar, 520008", "Gayathri Nagar", "520008", false},
		{"pin after a dash", "Gayathri Nagar - 520008", "Gayathri Nagar", "520008", false},
		{"pin and state, the state is then the town", "Gayathri Nagar, Andhra Pradesh, 520008", "", "520008", true},
		{"comma form keeps its pin", "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", "", "520008", true},
		{"a bare pin is not a neighbourhood", "520008", "", "", false},
		{"a street word blocks the bare text", "MG Road Bengaluru", "", "", false},
		{"a house number blocks the bare text", "12 Gayathri Nagar Vijayawada", "", "", false},
		{"a plot with a pin blocks the bare text", "Plot 4 Gayathri Nagar 520008", "", "520008", false},
		{"a bare state is no neighbourhood", "Telangana 500034", "", "500034", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTypedHome(tc.in)
			if got.bare != tc.wantBare {
				t.Errorf("bare = %q, want %q", got.bare, tc.wantBare)
			}
			if got.pin != tc.wantPin {
				t.Errorf("pin = %q, want %q", got.pin, tc.wantPin)
			}
			if got.ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", got.ok, tc.wantOK)
			}
		})
	}
}

func TestRegionOfAPostcodeRow(t *testing.T) {
	cases := []struct{ city, county, want string }{
		{"", "Vijayawada (Urban)", "Vijayawada"},
		{"", "Bangalore South", "Bangalore South"},
		{"Pune", "Haveli", "Pune"},
		{"", "Rangareddy District", "Rangareddy"},
		{"", "", ""},
		{"", "(Urban)", ""},
	}
	for _, tc := range cases {
		if got := regionName(tc.city, tc.county); got != tc.want {
			t.Errorf("regionName(%q, %q) = %q, want %q", tc.city, tc.county, got, tc.want)
		}
	}
}

// Truth 2: the town after no comma. The areas answer in hand already names
// Vijayawada, so the town is accepted without a request.
func TestTypedRowWithoutACommaTownNamedInTheAnswerInHand(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if isAreasQuery(q) {
			return []map[string]any{ayodhyaNagar()}
		}
		return []map[string]any{gayatriNagarMainStreet()}
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar Vijayawada", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	row, ok := typedRowOf(t, answer)
	if !ok {
		t.Fatalf("no typed row in %+v", answer.Results)
	}
	if row.Name != "Gayathri Nagar, Vijayawada" || row.Town != "Vijayawada" || row.DisplayName != "Vijayawada" {
		t.Errorf("typed row = %+v, want the comma form's row", row)
	}
	if row.Lat != 16.5035096 || row.Lon != 80.6546059 {
		t.Errorf("typed row at %v, %v, want the street's point", row.Lat, row.Lon)
	}
	if n := totalSent(ps, rs); n != 2 {
		t.Errorf("%d upstream requests, want 2: the areas search and the any place search", n)
	}
	if answer.NoCloseMatch {
		t.Error("NoCloseMatch is set although a typed row is offered")
	}
}

// The no comma form gives the same row as the comma form.
func TestNoCommaFormGivesTheSameRowAsTheCommaForm(t *testing.T) {
	route := func(q url.Values) []map[string]any {
		switch {
		case isAreasQuery(q) && q.Get("q") == "Vijayawada":
			return []map[string]any{vijayawadaCity()}
		case isAreasQuery(q):
			return []map[string]any{ayodhyaNagar()}
		}
		return nil
	}
	var rows [2]Result
	for i, text := range []string{"Gayathri Nagar, Vijayawada", "Gayathri Nagar Vijayawada"} {
		ps := routedPhoton(t, route)
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), text, nil)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		row, ok := typedRowOf(t, answer)
		if !ok {
			t.Fatalf("%q: no typed row in %+v", text, answer.Results)
		}
		rows[i] = row
	}
	if rows[0] != rows[1] {
		t.Errorf("comma form %+v, no comma form %+v, want the same row", rows[0], rows[1])
	}
}

// Benz Circle Vijayawada (the orchestrator's check): the answer in hand holds
// only Bengaluru circles, so the last word is looked up as a town, and the
// city level row it finds places the typed row.
func TestTypedRowWithoutACommaTownFoundByALookup(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		switch {
		case isAreasQuery(q) && q.Get("q") == "Vijayawada":
			return []map[string]any{vijayawadaCity()}
		case isAreasQuery(q):
			return []map[string]any{benguluruCircle()}
		}
		return nil
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Benz Circle Vijayawada", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	row, ok := typedRowOf(t, answer)
	if !ok {
		t.Fatalf("no typed row in %+v", answer.Results)
	}
	if row.Name != "Benz Circle, Vijayawada" || row.Town != "Vijayawada" {
		t.Errorf("typed row = %+v", row)
	}
	if row.Lat != 16.5115306 || row.Lon != 80.6160469 {
		t.Errorf("typed row at %v, %v, want Vijayawada's point", row.Lat, row.Lon)
	}
	// The areas search, the town lookup and the any place search.
	if n := totalSent(ps, rs); n != 3 {
		t.Errorf("%d upstream requests, want 3", n)
	}
}

// The last two words are the town when a city has a two word name.
func TestTypedRowWithoutACommaTwoWordTown(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		switch {
		case isAreasQuery(q) && q.Get("q") == "Navi Mumbai":
			return []map[string]any{typedRow(73.0297, 19.0330, "Navi Mumbai", "place", "city", "city", "", "Maharashtra")}
		case isAreasQuery(q) && q.Get("q") == "Mumbai":
			return []map[string]any{typedRow(72.8777, 19.0760, "Mumbai", "place", "city", "city", "", "Maharashtra")}
		case isAreasQuery(q):
			return []map[string]any{typedRow(72.84, 19.12, "Andheri", "place", "suburb", "district", "Mumbai", "Maharashtra")}
		}
		return nil
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Kharghar Navi Mumbai", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	row, ok := typedRowOf(t, answer)
	if !ok {
		t.Fatalf("no typed row in %+v", answer.Results)
	}
	if row.Name != "Kharghar, Navi Mumbai" || row.Town != "Navi Mumbai" {
		t.Errorf("typed row = %+v, want Kharghar in Navi Mumbai", row)
	}
}

// A locality word is never a town: Nagar is also a real town in Himachal
// Pradesh, and Circle a word in a hundred place names.
func TestALocalityWordIsNeverTriedAsATown(t *testing.T) {
	for _, text := range []string{"Gayathri Nagar", "Benz Circle", "Banjara Hills", "Rajiv Colony"} {
		t.Run(text, func(t *testing.T) {
			ps := routedPhoton(t, func(q url.Values) []map[string]any {
				if isAreasQuery(q) {
					return []map[string]any{typedRow(77.2, 32.1, "Nagar", "place", "town", "city", "", "Himachal Pradesh")}
				}
				return nil
			})
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)
			answer, err := c.SearchReaderAreas(context.Background(), text, nil)
			if err != nil {
				t.Fatalf("SearchReaderAreas returned error: %v", err)
			}
			if _, ok := typedRowOf(t, answer); ok {
				t.Fatalf("a typed row was placed in a town made of a locality word: %+v", answer.Results)
			}
			if n := totalSent(ps, rs); n != 1 {
				t.Errorf("%d upstream requests, want only the areas search", n)
			}
		})
	}
}

// A town lookup that finds a village or a locality, not a city, places nothing.
func TestATownLookupNeedsACityLevelRow(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if isAreasQuery(q) && q.Get("q") == "Vijayawada" {
			return []map[string]any{typedRow(80.1, 16.1, "Vijayawada", "place", "village", "city", "", "Andhra Pradesh")}
		}
		if isAreasQuery(q) {
			return []map[string]any{benguluruCircle()}
		}
		return nil
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	answer, err := c.SearchReaderAreas(context.Background(), "Benz Circle Vijayawada", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if _, ok := typedRowOf(t, answer); ok {
		t.Fatalf("a village was taken for the town: %+v", answer.Results)
	}
}

// Truth 3: a PIN code places the row when no town is typed. The row's second
// line is the town the PIN row names, and the saved name drops the PIN.
func TestTypedRowPlacedByAPinCode(t *testing.T) {
	for _, text := range []string{"Gayathri Nagar 520008", "Gayathri Nagar, 520008", "Gayathri Nagar - 520008"} {
		t.Run(text, func(t *testing.T) {
			ps := routedPhoton(t, func(q url.Values) []map[string]any {
				switch {
				case q.Get("q") == "520008":
					if isAreasQuery(q) {
						t.Error("the PIN was looked up as an areas search")
					}
					return []map[string]any{postcodeRow(80.667, 16.510, "520008", "", "Vijayawada (Urban)", "Andhra Pradesh")}
				case isAreasQuery(q):
					return []map[string]any{
						typedRow(78.596, 17.319, "Gayathri Nagar", "place", "neighbourhood", "locality", "", "Telangana"),
						typedRow(80.674, 16.510, "Padmavathi Nagar", "place", "neighbourhood", "locality", "Kanuru", "Andhra Pradesh"),
					}
				}
				return nil
			})
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)

			answer, err := c.SearchReaderAreas(context.Background(), text, nil)
			if err != nil {
				t.Fatalf("SearchReaderAreas returned error: %v", err)
			}
			row, ok := typedRowOf(t, answer)
			if !ok {
				t.Fatalf("no typed row in %+v", answer.Results)
			}
			if row.Name != "Gayathri Nagar, Vijayawada" || row.Town != "Vijayawada" || row.DisplayName != "Vijayawada" {
				t.Errorf("typed row = %+v, want the name without the PIN and Vijayawada as the town", row)
			}
			// The any place search for the name finds nothing, so the PIN's own point.
			if row.Lat != 16.510 || row.Lon != 80.667 {
				t.Errorf("typed row at %v, %v, want the PIN row's point 16.510, 80.667", row.Lat, row.Lon)
			}
			ps.mu.Lock()
			defer ps.mu.Unlock()
			if got := ps.queries[0].Get("q"); got != "Gayathri Nagar" {
				t.Errorf("the areas search was sent %q, want the text without the PIN", got)
			}
		})
	}
}

// The PIN is looked up only when no town is found, and never for a street address.
func TestAPinIsNotLookedUpWhenATownIsTypedOrForAStreetAddress(t *testing.T) {
	for _, text := range []string{
		"Gayathri Nagar, Vijayawada, 520008",
		"Plot 4 Gayathri Nagar 520008",
		"12 Gayathri Nagar 520008",
		"MG Road 520008",
	} {
		t.Run(text, func(t *testing.T) {
			ps := routedPhoton(t, func(q url.Values) []map[string]any {
				if q.Get("q") == "520008" {
					t.Error("the PIN was looked up")
				}
				if isAreasQuery(q) && q.Get("q") == "Vijayawada" {
					return []map[string]any{vijayawadaCity()}
				}
				return nil
			})
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)
			if _, err := c.SearchReaderAreas(context.Background(), text, nil); err != nil {
				t.Fatalf("SearchReaderAreas returned error: %v", err)
			}
		})
	}
}

// A typed town that cannot be placed (a state typed as the town: the areas
// layers never return a state) falls back to the PIN, when there is one.
func TestPinPlacesATypedTownThatCannotBePlaced(t *testing.T) {
	route := func(q url.Values) []map[string]any {
		switch {
		case q.Get("q") == "520008":
			return []map[string]any{postcodeRow(80.667, 16.510, "520008", "", "Vijayawada (Urban)", "Andhra Pradesh")}
		case isAreasQuery(q):
			return []map[string]any{ayodhyaNagar()}
		}
		return nil
	}
	t.Run("with a PIN the row is placed in the town the PIN names", func(t *testing.T) {
		ps := routedPhoton(t, route)
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Andhra Pradesh, 520008", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		row, ok := typedRowOf(t, answer)
		if !ok {
			t.Fatalf("no typed row in %+v", answer.Results)
		}
		if row.Name != "Gayathri Nagar, Vijayawada" || row.Town != "Vijayawada" || row.Lat != 16.510 {
			t.Errorf("typed row = %+v, want Gayathri Nagar in Vijayawada at the PIN's point", row)
		}
		if n := totalSent(ps, rs); n > maxUpstreamPerSearch {
			t.Errorf("%d upstream requests, the cap is %d", n, maxUpstreamPerSearch)
		}
	})
	t.Run("without a PIN nothing is placed and the line is asked for", func(t *testing.T) {
		ps := routedPhoton(t, route)
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Andhra Pradesh", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if _, ok := typedRowOf(t, answer); ok {
			t.Fatalf("a typed row was placed in a state: %+v", answer.Results)
		}
		if !answer.NoCloseMatch {
			t.Error("NoCloseMatch is not set")
		}
	})
}

// A PIN row that names no town places nothing.
func TestAPinRowWithoutATownPlacesNothing(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if q.Get("q") == "520008" {
			return []map[string]any{postcodeRow(80.667, 16.510, "520008", "Some Locality", "", "Andhra Pradesh")}
		}
		return nil
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar 520008", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if _, ok := typedRowOf(t, answer); ok {
		t.Fatalf("a typed row was placed in a locality that is not a town: %+v", answer.Results)
	}
	if !answer.NoCloseMatch {
		t.Error("NoCloseMatch is not set although nothing matched and no row was placed")
	}
}

// A street address is never offered, with a town or without one.
func TestNoCommaStreetAddressGetsNoRowAndNoExtraRequest(t *testing.T) {
	for _, text := range []string{
		"MG Road Bengaluru",
		"12 Gayathri Nagar Vijayawada",
		"Plot 4 Gayathri Nagar Vijayawada",
		"Flat Gayathri Nagar Vijayawada",
	} {
		t.Run(text, func(t *testing.T) {
			ps := routedPhoton(t, func(q url.Values) []map[string]any { return []map[string]any{ayodhyaNagar()} })
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)
			answer, err := c.SearchReaderAreas(context.Background(), text, nil)
			if err != nil {
				t.Fatalf("SearchReaderAreas returned error: %v", err)
			}
			if _, ok := typedRowOf(t, answer); ok {
				t.Fatalf("a street address was offered: %+v", answer.Results)
			}
			if n := totalSent(ps, rs); n != 1 {
				t.Errorf("%d upstream requests, want 1", n)
			}
		})
	}
}

// Truth 5: the extra lookups stay inside the cap of four.
func TestSplitAndPinLookupsStayInsideTheCapOfFour(t *testing.T) {
	t.Run("the search used all four, so no lookup is sent for the town or the PIN", func(t *testing.T) {
		ps := routedPhoton(t, func(q url.Values) []map[string]any { return nil })
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Kharghar Patamata Navi Mumbai 400710", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if n := totalSent(ps, rs); n != maxUpstreamPerSearch {
			t.Errorf("%d upstream requests, want exactly %d", n, maxUpstreamPerSearch)
		}
		if _, ok := typedRowOf(t, answer); ok {
			t.Errorf("a typed row with no position: %+v", answer.Results)
		}
	})
	t.Run("one areas request, then two town lookups and the PIN fill the cap", func(t *testing.T) {
		ps := routedPhoton(t, func(q url.Values) []map[string]any {
			if q.Get("q") == "400710" {
				return []map[string]any{postcodeRow(73.07, 19.04, "400710", "", "Thane", "Maharashtra")}
			}
			if isAreasQuery(q) {
				return []map[string]any{typedRow(77.19, 28.65, "Karol Bagh", "place", "suburb", "district", "Delhi", "Delhi")}
			}
			return nil
		})
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Kharghar Patamata Navi Mumbai 400710", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		// areas, "Navi Mumbai", "Mumbai", the PIN: four. The any place search
		// for the name has no request left, so the PIN row's point is used.
		if n := totalSent(ps, rs); n != maxUpstreamPerSearch {
			t.Errorf("%d upstream requests, want exactly %d", n, maxUpstreamPerSearch)
		}
		row, ok := typedRowOf(t, answer)
		if !ok || row.Town != "Thane" {
			t.Fatalf("typed row = %+v, %v, want one placed in Thane by the PIN", row, ok)
		}
	})
	t.Run("a search that finds nothing at all stays inside the cap", func(t *testing.T) {
		ps := routedPhoton(t, func(q url.Values) []map[string]any { return nil })
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		if _, err := c.SearchReaderAreas(context.Background(), "Benz Circle Vijayawada 520008", nil); err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if n := totalSent(ps, rs); n > maxUpstreamPerSearch {
			t.Errorf("%d upstream requests, the cap is %d", n, maxUpstreamPerSearch)
		}
	})
}

// A repeat of the same text sends nothing: every lookup is cached.
func TestRepeatedNoCommaSearchSendsNothing(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if isAreasQuery(q) && q.Get("q") == "Vijayawada" {
			return []map[string]any{vijayawadaCity()}
		}
		if isAreasQuery(q) {
			return []map[string]any{benguluruCircle()}
		}
		return nil
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	if _, err := c.SearchReaderAreas(context.Background(), "Benz Circle Vijayawada", nil); err != nil {
		t.Fatalf("first search: %v", err)
	}
	sent := totalSent(ps, rs)
	if _, err := c.SearchReaderAreas(context.Background(), "Benz Circle Vijayawada", nil); err != nil {
		t.Fatalf("second search: %v", err)
	}
	if n := totalSent(ps, rs); n != sent {
		t.Errorf("the repeat sent %d more requests", n-sent)
	}
}

// Truth 4: the not found flag. It is set when no row matches the typed words
// closely and no typed row is placed.
func TestNoCloseMatchFlag(t *testing.T) {
	cases := []struct {
		name string
		text string
		rows []map[string]any
		want bool
	}{
		{"Benz Circle alone, only other circles", "Benz Circle", []map[string]any{benguluruCircle()}, true},
		{"nothing at all", "Zzqqxx Nagar", nil, true},
		{"Koramangala is found", "Koramangala", []map[string]any{typedRow(77.624, 12.936, "Koramangala", "place", "suburb", "locality", "", "Karnataka")}, false},
		{"Koramangala blocks are found", "Koramangala", []map[string]any{typedRow(77.624, 12.939, "Koramangala 6th Block", "place", "neighbourhood", "locality", "", "Karnataka")}, false},
		{"half typed Koram", "Koram", []map[string]any{typedRow(77.624, 12.936, "Koramangala", "place", "suburb", "locality", "", "Karnataka")}, false},
		{"town after the area, the area is found", "Koramangala Bengaluru", []map[string]any{typedRow(77.624, 12.936, "Koramangala", "place", "suburb", "locality", "Bengaluru", "Karnataka")}, false},
		{"a close spelling counts", "Gayatri Nagar", []map[string]any{typedRow(77.559, 13.0, "Gayathri Nagar", "place", "neighbourhood", "locality", "Bengaluru", "Karnataka")}, false},
		{"a street address does not ask for a town", "12 Gayathri Nagar", []map[string]any{benguluruCircle()}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps := routedPhoton(t, func(q url.Values) []map[string]any { return tc.rows })
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)
			answer, err := c.SearchReaderAreas(context.Background(), tc.text, nil)
			if err != nil {
				t.Fatalf("SearchReaderAreas returned error: %v", err)
			}
			if answer.NoCloseMatch != tc.want {
				t.Errorf("NoCloseMatch = %v, want %v (rows %+v)", answer.NoCloseMatch, tc.want, answer.Results)
			}
		})
	}
}

// The normal place search never sets the flag.
func TestNoCloseMatchIsAreasModeOnly(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any { return []map[string]any{benguluruCircle()} })
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	answer, err := c.SearchReader(context.Background(), "Benz Circle", nil)
	if err != nil {
		t.Fatalf("SearchReader returned error: %v", err)
	}
	if answer.NoCloseMatch {
		t.Error("the normal place search set NoCloseMatch")
	}
}

// Truth 1: the home area search is never sent a point the server made up. With
// no near point it sends no lat or lon to Photon.
func TestAreasSearchWithoutANearPointSendsNoCentre(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any { return nil })
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	if _, err := c.SearchReaderAreas(context.Background(), "Benz Circle Vijayawada", nil); err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, q := range ps.queries {
		if q.Has("lat") || q.Has("lon") {
			t.Errorf("a Photon request carried a centre: %v", q)
		}
	}
}
