package geocode

// Tests for Phase 3.4 plan 32: the typed row of the home area search. When
// OpenStreetMap holds no area for what the person typed (the owner's
// "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008"), the areas answer
// starts with one extra row, the typed text, placed in the typed town. Every
// upstream is an httptest stand in; the live results are in 03.4-32-SUMMARY.md.
//
// How the plan's first truth is read: "no area result's name closely matches
// the first part" counts only a close name that lies in the typed town (its
// label names the town, or its point is within typedTownRadiusKm of the town
// point). An exact name in another town (Gayathri Nagar in Telangana) must not
// hide the row, and a real area whose label does not name the town (Banjara
// Hills with an empty city) must not get a duplicate.

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
)

// typedRow builds a Photon feature in the Indian state given, with a city when
// city is not empty.
func typedRow(lon, lat float64, name, osmKey, osmValue, typ, city, state string) map[string]any {
	props := map[string]any{
		"name": name, "state": state, "country": "India", "countrycode": "IN",
		"osm_key": osmKey, "osm_value": osmValue, "type": typ,
	}
	if city != "" {
		props["city"] = city
	}
	return photonFeatureJSON(lon, lat, props)
}

// routedPhoton is a Photon stand in whose answer depends on the query: route
// gets the query values (layer is present for an areas search).
func routedPhoton(t *testing.T, route func(q url.Values) []map[string]any) *photonStub {
	t.Helper()
	return newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(photonBody(t, route(r.URL.Query())...))
	})
}

func isAreasQuery(q url.Values) bool { return len(q["layer"]) > 0 }

// The Vijayawada rows of the live check.
func vijayawadaCity() map[string]any {
	return typedRow(80.6160469, 16.5115306, "Vijayawada", "place", "city", "city", "", "Andhra Pradesh")
}

func gayatriNagarMainStreet() map[string]any {
	return typedRow(80.6546059, 16.5035096, "Gayatri Nagar Main", "highway", "residential", "street", "Vijayawada", "Andhra Pradesh")
}

func totalSent(ps *photonStub, rs *recordingServer) int { return ps.count() + len(rs.sent()) }

func typedRowOf(t *testing.T, answer Answer) (Result, bool) {
	t.Helper()
	for i, r := range answer.Results {
		if r.Typed {
			if i != 0 {
				t.Fatalf("the typed row is at position %d, want first", i)
			}
			return r, true
		}
	}
	return Result{}, false
}

func TestParseTypedHome(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantSearch string
		wantName   string
		wantTown   string
		wantOK     bool
	}{
		{"owner example", "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", "Gayathri Nagar, Vijayawada", "Gayathri Nagar, Vijayawada", "Vijayawada", true},
		{"pin shares a part with the state", "Gayathri Nagar, Vijayawada, Andhra Pradesh 520008", "Gayathri Nagar, Vijayawada", "Gayathri Nagar, Vijayawada", "Vijayawada", true},
		{"pin and india", "Gayathri Nagar, Vijayawada, Andhra Pradesh 520008, India", "Gayathri Nagar, Vijayawada", "Gayathri Nagar, Vijayawada", "Vijayawada", true},
		{"pin after a dash", "Gayathri Nagar, Vijayawada - 520008", "Gayathri Nagar, Vijayawada", "Gayathri Nagar, Vijayawada", "Vijayawada", true},
		{"state only", "Gayathri Nagar, Vijayawada, Andhra Pradesh", "Gayathri Nagar, Vijayawada", "Gayathri Nagar, Vijayawada", "Vijayawada", true},
		{"already clean", "Gayathri Nagar, Vijayawada", "Gayathri Nagar, Vijayawada", "Gayathri Nagar, Vijayawada", "Vijayawada", true},
		{"state is the town when only two parts", "Calangute, Goa", "Calangute, Goa", "Calangute, Goa", "Goa", true},
		{"state case and spaces", "  Banjara  Hills ,Hyderabad,  TELANGANA ,500034 ", "Banjara Hills, Hyderabad", "Banjara Hills, Hyderabad", "Hyderabad", true},
		{"only the first two parts are saved", "Gayathri Nagar, Patamata, Vijayawada, Andhra Pradesh", "Gayathri Nagar, Patamata, Vijayawada", "Gayathri Nagar, Patamata", "Patamata", true},
		{"a city that is also a territory is kept", "Rohini, Sector Seven, Delhi", "Rohini, Sector Seven, Delhi", "Rohini, Sector Seven", "Sector Seven", true},
		{"no second part, no town", "Gayathri Nagar", "Gayathri Nagar", "", "", false},
		{"only a pin after it, no town", "Gayathri Nagar, 520008", "Gayathri Nagar", "", "", false},
		{"a state typed as the town is kept as it is", "Gayathri Nagar, Karnataka", "Gayathri Nagar, Karnataka", "Gayathri Nagar, Karnataka", "Karnataka", true},
		{"a bare state is left alone", "Telangana", "Telangana", "", "", false},
		{"a house number is a street address", "12, Gayathri Nagar, Vijayawada", "12, Gayathri Nagar, Vijayawada", "", "", false},
		{"a plot number is a street address", "Plot 4, Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", "Plot 4, Gayathri Nagar, Vijayawada", "", "", false},
		{"a road is a street address", "MG Road, Bengaluru", "MG Road, Bengaluru", "", "", false},
		{"a lane is a street address", "Temple Lane, Pune", "Temple Lane, Pune", "", "", false},
		{"a short first part", "Ab, Vijayawada", "Ab, Vijayawada", "", "", false},
		{"a short town", "Gayathri Nagar, Vj", "Gayathri Nagar, Vj", "", "", false},
		{"empty", "  ", "", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTypedHome(tc.in)
			if got.search != tc.wantSearch {
				t.Errorf("search = %q, want %q", got.search, tc.wantSearch)
			}
			if got.ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", got.ok, tc.wantOK)
			}
			if tc.wantOK && (got.name != tc.wantName || got.town != tc.wantTown) {
				t.Errorf("name, town = %q, %q, want %q, %q", got.name, got.town, tc.wantName, tc.wantTown)
			}
		})
	}
}

func TestParseTypedHomeNameIsAtMostTheSavedLimit(t *testing.T) {
	long := ""
	for len(long) < 130 {
		long += "Abcdefgh "
	}
	if got := parseTypedHome(long + ", Vijayawada"); got.ok {
		t.Fatalf("a %d letter name was offered, the saved name allows 120", len(got.name))
	}
}

func TestCloseSpellingVariantsOfTheOwnersExample(t *testing.T) {
	if !closeSpelling("Gayathri Nagar", "Gayatri Nagar") {
		t.Fatal("Gayathri Nagar and Gayatri Nagar must count as close")
	}
	if !leadingNameMatches("Gayathri Nagar", "Gayatri Nagar Main") {
		t.Fatal("a street that starts with a close spelling of the first part must match")
	}
	if leadingNameMatches("Gayathri Nagar", "Sri Gayathri Rice Store") {
		t.Fatal("a shop that merely contains the word must not match")
	}
}

// Truth 2, first branch: an any place match in the same town whose name is a
// close spelling of the first part gives the position.
func TestTypedRowPlacedByACloseSpellingInTheSameTown(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if isAreasQuery(q) {
			return []map[string]any{typedRow(80.6316, 16.5302, "Ayodhya Nagar", "place", "suburb", "district", "Vijayawada", "Andhra Pradesh")}
		}
		return []map[string]any{
			typedRow(80.6551, 16.4897, "Sri Gayatri Rice Store", "shop", "yes", "house", "Vijayawada", "Andhra Pradesh"),
			typedRow(80.6546, 16.5035, "Gayatri Nagar", "place", "neighbourhood", "locality", "Vijayawada", "Andhra Pradesh"),
		}
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	row, ok := typedRowOf(t, answer)
	if !ok {
		t.Fatalf("no typed row in %+v", answer.Results)
	}
	if row.Name != "Gayathri Nagar, Vijayawada" || row.Town != "Vijayawada" || row.DisplayName != "Vijayawada" {
		t.Errorf("typed row = %+v, want name %q, town and second line Vijayawada", row, "Gayathri Nagar, Vijayawada")
	}
	if row.Lat != 16.5035 || row.Lon != 80.6546 {
		t.Errorf("typed row at %v, %v, want the close spelling's point 16.5035, 80.6546", row.Lat, row.Lon)
	}
	if n := totalSent(ps, rs); n != 2 {
		t.Errorf("%d upstream requests, want 2: the areas search and the any place search", n)
	}
	if len(answer.Results) != 2 {
		t.Errorf("%d rows, want the typed row and the one area", len(answer.Results))
	}
}

// A street that starts with the close spelling counts (Gayatri Nagar Main for
// Gayathri Nagar).
func TestTypedRowPlacedByAStreetThatStartsWithTheCloseSpelling(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if isAreasQuery(q) {
			return []map[string]any{typedRow(80.6316, 16.5302, "Ayodhya Nagar", "place", "suburb", "district", "Vijayawada", "Andhra Pradesh")}
		}
		return []map[string]any{gayatriNagarMainStreet()}
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	row, ok := typedRowOf(t, answer)
	if !ok {
		t.Fatalf("no typed row in %+v", answer.Results)
	}
	if row.Lat != 16.5035096 || row.Lon != 80.6546059 {
		t.Errorf("typed row at %v, %v, want the street's point", row.Lat, row.Lon)
	}
}

// Truth 2, second branch: nothing close in town, so the town itself from an
// areas search for the second part. This is what the owner's example gets live.
func TestTypedRowPlacedByTheTownWhenNothingCloseIsFound(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		switch {
		case isAreasQuery(q) && q.Get("q") == "Vijayawada":
			return []map[string]any{vijayawadaCity()}
		case isAreasQuery(q):
			return []map[string]any{typedRow(80.6316, 16.5302, "Ayodhya Nagar", "place", "suburb", "district", "Vijayawada", "Andhra Pradesh")}
		}
		return []map[string]any{
			typedRow(80.6551, 16.4897, "Sri Gayathri Rice Store", "shop", "yes", "house", "Vijayawada", "Andhra Pradesh"),
			typedRow(80.6344, 16.5137, "Sri Gayathri Ayurvedic Hospital", "amenity", "hospital", "house", "Vijayawada", "Andhra Pradesh"),
		}
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	row, ok := typedRowOf(t, answer)
	if !ok {
		t.Fatalf("no typed row in %+v", answer.Results)
	}
	if row.Name != "Gayathri Nagar, Vijayawada" || row.Town != "Vijayawada" {
		t.Errorf("typed row = %+v", row)
	}
	if row.Lat != 16.5115306 || row.Lon != 80.6160469 {
		t.Errorf("typed row at %v, %v, want the town's point 16.5115306, 80.6160469", row.Lat, row.Lon)
	}
	if n := totalSent(ps, rs); n != 3 {
		t.Errorf("%d upstream requests, want 3: areas, any place, town", n)
	}
}

// The town point is read from the answer already in hand when it holds the
// town, so no request is spent on it.
func TestTownPointIsTakenFromTheAreasAnswerWhenItIsThere(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if isAreasQuery(q) {
			return []map[string]any{vijayawadaCity()}
		}
		return nil
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	row, ok := typedRowOf(t, answer)
	if !ok || row.Lat != 16.5115306 {
		t.Fatalf("typed row = %+v, %v", row, ok)
	}
	if n := totalSent(ps, rs); n != 2 {
		t.Errorf("%d upstream requests, want 2 (areas, any place)", n)
	}
}

func TestNoTypedRowWhenNeitherTheCloseSpellingNorTheTownIsFound(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if isAreasQuery(q) && q.Get("q") != "Vijayawada" {
			return []map[string]any{typedRow(80.6316, 16.5302, "Ayodhya Nagar", "place", "suburb", "district", "Vijayawada", "Andhra Pradesh")}
		}
		return nil
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if _, ok := typedRowOf(t, answer); ok {
		t.Fatalf("a typed row came back with no position found: %+v", answer.Results)
	}
	if len(answer.Results) != 1 || answer.Results[0].Name != "Ayodhya Nagar" {
		t.Errorf("the plain areas answer changed: %+v", answer.Results)
	}
}

// A close named area in the typed town means the area is already offered.
func TestNoTypedRowWhenAnAreaWithTheNameIsInTheTown(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		return []map[string]any{typedRow(80.6546, 16.5035, "Gayatri Nagar", "place", "neighbourhood", "locality", "Vijayawada", "Andhra Pradesh")}
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if _, ok := typedRowOf(t, answer); ok {
		t.Fatalf("a typed row was added over a real area in the town: %+v", answer.Results)
	}
	if n := totalSent(ps, rs); n != 1 {
		t.Errorf("%d upstream requests, want 1: a real area needs no extra lookup", n)
	}
}

// The exact name in another town must not hide the row (the live check: three
// Gayathri Nagars in Chittoor, Telangana and Bengaluru).
func TestNoTypedRowIsSuppressedByTheSameNameInAnotherTown(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		switch {
		case isAreasQuery(q) && q.Get("q") == "Vijayawada":
			return []map[string]any{vijayawadaCity()}
		case isAreasQuery(q):
			return []map[string]any{
				typedRow(78.59589, 17.3187, "Gayathri Nagar", "place", "neighbourhood", "locality", "", "Telangana"),
				typedRow(77.559, 13.0, "Gayatri Nagar", "place", "neighbourhood", "locality", "Bengaluru", "Karnataka"),
			}
		}
		return nil
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	row, ok := typedRowOf(t, answer)
	if !ok {
		t.Fatalf("the same name in other towns hid the typed row: %+v", answer.Results)
	}
	if row.Lat != 16.5115306 {
		t.Errorf("typed row at %v, want Vijayawada's point", row.Lat)
	}
	if len(answer.Results) != 3 {
		t.Errorf("%d rows, want the typed row and the two other towns' areas", len(answer.Results))
	}
	if n := totalSent(ps, rs); n > maxUpstreamPerSearch {
		t.Errorf("%d upstream requests, the cap is %d", n, maxUpstreamPerSearch)
	}
}

// A real area whose label does not name the typed town (Banjara Hills with an
// empty city) is found to be in the town by distance, so no duplicate row.
func TestNoTypedRowForARealAreaWhoseLabelLacksTheTown(t *testing.T) {
	for name, county := range map[string]string{"empty city": "", "a mandal as county": "Shaikpet mandal"} {
		t.Run(name, func(t *testing.T) {
			ps := routedPhoton(t, func(q url.Values) []map[string]any {
				switch {
				case isAreasQuery(q) && q.Get("q") == "Hyderabad":
					return []map[string]any{indianCity("Hyderabad")}
				case isAreasQuery(q):
					props := map[string]any{
						"name": "Banjara Hills", "state": "Telangana", "country": "India", "countrycode": "IN",
						"osm_key": "place", "osm_value": "neighbourhood", "type": "locality",
					}
					if county != "" {
						props["county"] = county
					}
					return []map[string]any{photonFeatureJSON(78.4347, 17.4156, props)}
				}
				return nil
			})
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)

			answer, err := c.SearchReaderAreas(context.Background(), "Banjara Hills, Hyderabad, Telangana, 500034", nil)
			if err != nil {
				t.Fatalf("SearchReaderAreas returned error: %v", err)
			}
			if _, ok := typedRowOf(t, answer); ok {
				t.Fatalf("a typed row was added over Banjara Hills: %+v", answer.Results)
			}
			if len(answer.Results) != 1 {
				t.Errorf("%d rows, want the one real area", len(answer.Results))
			}
		})
	}
}

// A first part with a digit, or a street word, is a street address: no row and
// no extra request.
func TestNoTypedRowAndNoExtraRequestForAStreetAddress(t *testing.T) {
	for _, text := range []string{
		"Plot 4, Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008",
		"12 Gayathri Nagar, Vijayawada",
		"MG Road, Vijayawada",
	} {
		t.Run(text, func(t *testing.T) {
			ps := routedPhoton(t, func(q url.Values) []map[string]any {
				return []map[string]any{typedRow(80.6316, 16.5302, "Ayodhya Nagar", "place", "suburb", "district", "Vijayawada", "Andhra Pradesh")}
			})
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

func TestNoTypedRowWithoutASecondPart(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any { return nil })
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if _, ok := typedRowOf(t, answer); ok {
		t.Fatalf("a typed row came back with no town typed: %+v", answer.Results)
	}
}

// Truth 4: the area search is sent without the PIN code and the state.
func TestAreaSearchIsSentWithoutPinAndState(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any { return []map[string]any{vijayawadaCity()} })
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	if _, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil); err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if got := ps.queries[0].Get("q"); got != "Gayathri Nagar, Vijayawada" {
		t.Errorf("the first Photon query = %q, want the text without the state and PIN", got)
	}
}

func TestNominatimAreaSearchIsSentWithoutPinAndState(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any { return nil })
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	if _, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil); err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if texts := rs.texts(); len(texts) == 0 || texts[0] != "Gayathri Nagar, Vijayawada" {
		t.Errorf("Nominatim queries = %q, want the first to be the text without the state and PIN", texts)
	}
}

// Truth 5: the extra lookups count inside the per search cap of four.
func TestExtraLookupsStayInsideTheCapOfFour(t *testing.T) {
	t.Run("the search itself used all four, so no extra lookup is sent", func(t *testing.T) {
		ps := routedPhoton(t, func(q url.Values) []map[string]any { return nil })
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Patamata Lanka, Vijayawada", nil)
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
	t.Run("one request left is spent on the any place search", func(t *testing.T) {
		ps := routedPhoton(t, func(q url.Values) []map[string]any { return nil })
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		if _, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh", nil); err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if n := totalSent(ps, rs); n > maxUpstreamPerSearch {
			t.Errorf("%d upstream requests, the cap is %d", n, maxUpstreamPerSearch)
		}
	})
	t.Run("a Photon timeout and its retry count too", func(t *testing.T) {
		var first = true
		ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) {
			if first {
				first = false
				<-r.Context().Done()
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(photonBody(t, typedRow(80.6316, 16.5302, "Ayodhya Nagar", "place", "suburb", "district", "Vijayawada", "Andhra Pradesh")))
		})
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		c.photonTimeout = 50 * time.Millisecond
		if _, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada", nil); err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if n := totalSent(ps, rs); n > maxUpstreamPerSearch {
			t.Errorf("%d upstream requests, the cap is %d", n, maxUpstreamPerSearch)
		}
	})
}

// A busy limiter during the extra lookups returns the plain answer, never an
// error for a search that already has results.
func TestBusyLimiterDuringTheExtraLookupsReturnsThePlainAnswer(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		return []map[string]any{typedRow(80.6316, 16.5302, "Ayodhya Nagar", "place", "suburb", "district", "Vijayawada", "Andhra Pradesh")}
	})
	rs := newRecordingServer(t, nil)
	c := newClientWithProviders(ps.server.URL, rs.server.URL, "", time.Millisecond, 10, time.Hour, 1)
	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error %v, want the plain answer", err)
	}
	if len(answer.Results) != 1 || answer.Results[0].Typed {
		t.Errorf("answer = %+v, want the one plain area", answer.Results)
	}
}

// A repeat of the same text sends nothing: the areas answer and every extra
// lookup are cached.
func TestRepeatedTypedSearchSendsNothingAndGivesTheSameRow(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if isAreasQuery(q) && q.Get("q") == "Vijayawada" {
			return []map[string]any{vijayawadaCity()}
		}
		return []map[string]any{typedRow(80.6316, 16.5302, "Ayodhya Nagar", "place", "suburb", "district", "Vijayawada", "Andhra Pradesh")}
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	first, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil)
	if err != nil {
		t.Fatalf("first search: %v", err)
	}
	sent := totalSent(ps, rs)
	second, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil)
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if n := totalSent(ps, rs); n != sent {
		t.Errorf("the repeat sent %d more requests", n-sent)
	}
	if !reflect.DeepEqual(first.Results, second.Results) {
		t.Errorf("the repeat answered differently:\n%+v\n%+v", first.Results, second.Results)
	}
}

// The typed row never reaches the normal place search, and the list stays at
// five rows.
func TestTypedRowIsAreasModeOnlyAndTheListStaysAtFive(t *testing.T) {
	five := []map[string]any{}
	for _, name := range []string{"Ayodhya Nagar", "Devi Nagar", "Rajiv Nagar", "JD Nagar", "Pushpatotha Nagar"} {
		five = append(five, typedRow(80.63, 16.53, name, "place", "neighbourhood", "locality", "Vijayawada", "Andhra Pradesh"))
	}
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if isAreasQuery(q) && q.Get("q") == "Vijayawada" {
			return []map[string]any{vijayawadaCity()}
		}
		if isAreasQuery(q) {
			return five
		}
		return nil
	})
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	normal, err := c.SearchReader(context.Background(), "Gayathri Nagar, Vijayawada", nil)
	if err != nil {
		t.Fatalf("SearchReader returned error: %v", err)
	}
	if _, ok := typedRowOf(t, normal); ok {
		t.Fatalf("the normal place search got a typed row: %+v", normal.Results)
	}

	areas, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if _, ok := typedRowOf(t, areas); !ok {
		t.Fatalf("no typed row: %+v", areas.Results)
	}
	if len(areas.Results) != maxResults {
		t.Errorf("%d rows, want at most %d", len(areas.Results), maxResults)
	}
}

func TestDistanceKm(t *testing.T) {
	// Vijayawada city to Gayatri Nagar Main is a few km; Vijayawada to
	// Hyderabad is about 250 km.
	if d := distanceKm(16.5115306, 80.6160469, 16.5035096, 80.6546059); d < 3 || d > 6 {
		t.Errorf("Vijayawada to the street = %.1f km, want about 4", d)
	}
	if d := distanceKm(16.5115, 80.6160, 17.385, 78.4867); d < 230 || d > 270 {
		t.Errorf("Vijayawada to Hyderabad = %.1f km, want about 250", d)
	}
}

