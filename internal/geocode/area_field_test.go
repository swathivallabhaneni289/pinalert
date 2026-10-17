package geocode

// Tests for Phase 3.4 plan 28, task 4: every result carries Area, the name of
// the area the place lies in (its own name when it is an area itself), so step 3
// of the first time flow can prefill the home area with an area and never with
// a street or a building name. Every upstream here is an httptest stand in.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func photonProps(props map[string]any) map[string]any {
	props["country"] = "India"
	props["countrycode"] = "IN"
	return photonFeatureJSON(77.6, 12.97, props)
}

func TestPhotonResultsCarryTheAreaTheyLieIn(t *testing.T) {
	cases := []struct {
		name string
		row  map[string]any
		want string
	}{
		{"a street takes its district", photonProps(map[string]any{
			"name": "MG Road", "district": "Shivajinagar", "city": "Bengaluru", "state": "Karnataka",
			"osm_key": "highway", "osm_value": "primary", "type": "street"}), "Shivajinagar"},
		{"a street takes its locality before its district", photonProps(map[string]any{
			"name": "MG Road", "locality": "Brigade Layout", "district": "Shivajinagar", "city": "Bengaluru",
			"osm_key": "highway", "osm_value": "primary", "type": "street"}), "Brigade Layout"},
		{"a building with only a city takes the city", photonProps(map[string]any{
			"name": "Indira Hospital", "housenumber": "4", "street": "MG Road", "city": "Bengaluru", "state": "Karnataka",
			"osm_key": "amenity", "osm_value": "hospital", "type": "house"}), "Bengaluru"},
		{"a street with no locality, district or city has no area", photonProps(map[string]any{
			"name": "Lonely Road", "county": "Some County", "state": "Karnataka",
			"osm_key": "highway", "osm_value": "residential", "type": "street"}), ""},
		{"an area is its own area", photonProps(map[string]any{
			"name": "Indiranagar", "city": "Bengaluru", "state": "Karnataka",
			"osm_key": "boundary", "osm_value": "administrative", "type": "district"}), "Indiranagar"},
		{"a city is its own area", photonProps(map[string]any{
			"name": "Hyderabad", "state": "Telangana",
			"osm_key": "place", "osm_value": "city", "type": "city"}), "Hyderabad"},
		{"a residential land use area is its own area", photonProps(map[string]any{
			"name": "Kothrud", "city": "Pune", "state": "Maharashtra",
			"osm_key": "landuse", "osm_value": "residential", "type": "locality"}), "Kothrud"},
		{"an industrial land use area is not one, so it takes the city", photonProps(map[string]any{
			"name": "Chennai Port", "city": "Chennai", "state": "Tamil Nadu",
			"osm_key": "landuse", "osm_value": "industrial", "type": "locality"}), "Chennai"},
		{"a null district is skipped", photonProps(map[string]any{
			"name": "Ring Road", "district": "null", "city": "Pune", "state": "Maharashtra",
			"osm_key": "highway", "osm_value": "trunk", "type": "street"}), "Pune"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps := newPhotonStub(t, photonAnswers(t, tc.row))
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)
			for lane, search := range lanes {
				answer, err := search(c, context.Background(), "anything", nil)
				if err != nil {
					t.Fatalf("%s: search returned error: %v", lane, err)
				}
				if len(answer.Results) != 1 {
					t.Fatalf("%s: got %d results, want 1", lane, len(answer.Results))
				}
				if got := answer.Results[0].Area; got != tc.want {
					t.Errorf("%s: Area = %q, want %q", lane, got, tc.want)
				}
				// A different text each time so the cache does not answer.
				c = newDualClient(ps.server.URL, rs.server.URL)
			}
		})
	}
}

func TestAreasModeResultsAreTheirOwnArea(t *testing.T) {
	ps := newPhotonStub(t, photonAnswers(t, mixedPhotonRows()...))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	answer, err := c.SearchReaderAreas(context.Background(), "Indira", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if len(answer.Results) == 0 {
		t.Fatalf("no results")
	}
	for _, r := range answer.Results {
		if r.Area != r.Name {
			t.Errorf("result %q has Area %q: an area is its own area", r.Name, r.Area)
		}
	}
}

func TestNominatimResultsCarryTheirAreaOnlyWhenTheyAreOne(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
		 {"name":"MG Road","display_name":"MG Road, Shivajinagar, Bengaluru, India","lat":"12.97","lon":"77.60","category":"highway","type":"primary","addresstype":"road"},
		 {"name":"Indiranagar","display_name":"Indiranagar, Jogpalya, Bengaluru, India","lat":"12.97","lon":"77.64","category":"place","type":"suburb","addresstype":"suburb"},
		 {"name":"","display_name":"Koramangala, Bengaluru, India","lat":"12.93","lon":"77.62","category":"boundary","type":"administrative","addresstype":"city_district"}
		]`))
	}))
	defer server.Close()
	c := newClientWithBaseURL(server.URL, "", defaultLimiterEvery, defaultLimiterBurst)
	answer, err := c.Search(context.Background(), "anything", nil)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	want := []string{"", "Indiranagar", "Koramangala"}
	if len(answer.Results) != len(want) {
		t.Fatalf("got %d results, want %d", len(answer.Results), len(want))
	}
	for i, r := range answer.Results {
		if r.Area != want[i] {
			t.Errorf("result %d (%q): Area = %q, want %q", i, r.Name, r.Area, want[i])
		}
	}
}
