package geocode

// Tests for quick task 261016-pho: Photon first for both searches, India only,
// close spellings of a place's name ranked first, labels built from parts,
// and Nominatim only when Photon fails or finds nothing. Every upstream here
// is an httptest stand in; nothing calls the real Photon or Nominatim.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// photonStub is a Photon stand in that records every request and answers with
// whatever respond returns for it.
type photonStub struct {
	mu      sync.Mutex
	queries []url.Values
	paths   []string
	agents  []string
	server  *httptest.Server
}

func newPhotonStub(t *testing.T, respond func(w http.ResponseWriter, r *http.Request)) *photonStub {
	t.Helper()
	ps := &photonStub{}
	ps.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ps.mu.Lock()
		ps.queries = append(ps.queries, r.URL.Query())
		ps.paths = append(ps.paths, r.URL.Path)
		ps.agents = append(ps.agents, r.Header.Get("User-Agent"))
		ps.mu.Unlock()
		respond(w, r)
	}))
	t.Cleanup(ps.server.Close)
	return ps
}

func (ps *photonStub) count() int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return len(ps.queries)
}

func (ps *photonStub) last() url.Values {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.queries[len(ps.queries)-1]
}

// photonFeature builds one GeoJSON feature the way Photon sends it:
// coordinates longitude first.
func photonFeatureJSON(lon, lat float64, props map[string]any) map[string]any {
	return map[string]any{
		"type":       "Feature",
		"geometry":   map[string]any{"type": "Point", "coordinates": []float64{lon, lat}},
		"properties": props,
	}
}

func photonBody(t *testing.T, features ...map[string]any) []byte {
	t.Helper()
	if features == nil {
		features = []map[string]any{}
	}
	body, err := json.Marshal(map[string]any{"type": "FeatureCollection", "features": features})
	if err != nil {
		t.Fatalf("marshal photon body: %v", err)
	}
	return body
}

// photonAnswers is a stub responder that always sends the same features.
func photonAnswers(t *testing.T, features ...map[string]any) func(w http.ResponseWriter, r *http.Request) {
	body := photonBody(t, features...)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
}

func indianCity(name string) map[string]any {
	return photonFeatureJSON(78.4867, 17.385, map[string]any{
		"name": name, "state": "Telangana", "country": "India", "countrycode": "IN",
		"osm_key": "place", "osm_value": "city", "type": "city",
	})
}

// newDualClient builds a client with both providers pointed at stand ins and
// limiters fast enough not to matter.
func newDualClient(photonURL, nominatimURL string) *Client {
	return newClientWithProviders(photonURL, nominatimURL, "", time.Millisecond, 10, time.Millisecond, 10)
}

func TestPhotonSearchURLIsHTTPS(t *testing.T) {
	if !strings.HasPrefix(photonSearchURL, "https://photon.komoot.io/") {
		t.Fatalf("photonSearchURL = %q, want https://photon.komoot.io/", photonSearchURL)
	}
}

func TestPhotonAnswersFirstOnBothLanes(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ps := newPhotonStub(t, photonAnswers(t, indianCity("Hyderabad")))
			rs := newRecordingServer(t, func(url.Values) bool { return true })
			c := newDualClient(ps.server.URL, rs.server.URL)

			answer, err := search(c, context.Background(), "Hyderabad", &Point{Lat: 17.4, Lon: 78.45})
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if ps.count() != 1 {
				t.Fatalf("photon got %d requests, want 1", ps.count())
			}
			if n := len(rs.sent()); n != 0 {
				t.Fatalf("nominatim got %d requests, want 0 when photon found something", n)
			}
			if len(answer.Results) != 1 || answer.Results[0].Name != "Hyderabad" || answer.MatchedQuery != "" {
				t.Fatalf("answer = %+v, want Hyderabad from photon and no matched text", answer)
			}
			if answer.provider != providerPhoton {
				t.Fatalf("provider = %q, want %q", answer.provider, providerPhoton)
			}
		})
	}
}

func TestPhotonRequestCarriesOnlyFixedParameters(t *testing.T) {
	ps := newPhotonStub(t, photonAnswers(t, indianCity("Hyderabad")))
	rs := newRecordingServer(t, nil)
	c := newClientWithProviders(ps.server.URL+"/api/", rs.server.URL, "contact@example.com", time.Millisecond, 10, time.Millisecond, 10)

	q := "Banjra Hills & more"
	if _, err := c.Search(context.Background(), q, &Point{Lat: 17.4156, Lon: 78.4347}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	got := ps.last()
	if ps.paths[0] != "/api/" {
		t.Errorf("path = %q, want /api/", ps.paths[0])
	}
	want := map[string]string{
		"q":     q,
		"limit": "10",
		"lang":  "en",
		"bbox":  indiaBBox,
		"lat":   "17.415600",
		"lon":   "78.434700",
	}
	for key, value := range want {
		if got.Get(key) != value {
			t.Errorf("%s = %q, want %q", key, got.Get(key), value)
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			t.Errorf("unexpected parameter %q sent to photon", key)
		}
	}
	if !strings.HasPrefix(ps.agents[0], "Pinalert/") || !strings.Contains(ps.agents[0], "contact@example.com") {
		t.Errorf("User-Agent = %q, want the Pinalert agent with the contact", ps.agents[0])
	}

	// No position: no lat or lon.
	if _, err := c.Search(context.Background(), "Gachib", nil); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	got = ps.last()
	if got.Has("lat") || got.Has("lon") {
		t.Errorf("lat or lon sent with no position: %v", got)
	}
	if got.Get("bbox") != indiaBBox {
		t.Errorf("bbox = %q, want the India box on every request", got.Get("bbox"))
	}
}

func TestPhotonKeepsIndiaOnlyAndReadsLonLat(t *testing.T) {
	pakistan := photonFeatureJSON(68.3578, 25.396, map[string]any{
		"name": "Hyderabad", "state": "Sindh", "countrycode": "PK", "osm_key": "place", "type": "city",
	})
	nepal := photonFeatureJSON(85.324, 27.7172, map[string]any{
		"name": "Kathmandu", "countrycode": "NP", "osm_key": "place", "type": "city",
	})
	india := photonFeatureJSON(78.4, 17.4, map[string]any{
		"name": "Hyderabad", "state": "Telangana", "countrycode": "in", "osm_key": "place", "type": "city",
	})
	ps := newPhotonStub(t, photonAnswers(t, pakistan, nepal, india))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.Search(context.Background(), "Hyderabad", nil)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(answer.Results) != 1 {
		t.Fatalf("results = %+v, want only the Indian one", answer.Results)
	}
	got := answer.Results[0]
	if got.Lat != 17.4 || got.Lon != 78.4 {
		t.Fatalf("lat, lon = %v, %v; want 17.4, 78.4 (photon sends longitude first)", got.Lat, got.Lon)
	}
	if got.DisplayName != "Hyderabad, Telangana" {
		t.Fatalf("DisplayName = %q, want %q", got.DisplayName, "Hyderabad, Telangana")
	}
}

func TestPhotonWithOnlyForeignResultsFallsBackToNominatim(t *testing.T) {
	pakistan := photonFeatureJSON(68.3578, 25.396, map[string]any{
		"name": "Hyderabad", "countrycode": "PK", "osm_key": "place", "type": "city",
	})
	ps := newPhotonStub(t, photonAnswers(t, pakistan))
	rs := newRecordingServer(t, func(url.Values) bool { return true })
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.Search(context.Background(), "Hyderabad", nil)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(rs.sent()) != 1 || answer.provider != providerNominatim {
		t.Fatalf("nominatim requests %d, provider %q; want 1 and nominatim", len(rs.sent()), answer.provider)
	}
}

func TestPhotonSkipsBadCoordinates(t *testing.T) {
	bad := map[string]any{
		"type":       "Feature",
		"geometry":   map[string]any{"type": "Point", "coordinates": []float64{78.4}},
		"properties": map[string]any{"name": "Half", "countrycode": "IN", "osm_key": "place"},
	}
	outOfRange := photonFeatureJSON(200, 17.4, map[string]any{"name": "Far", "countrycode": "IN", "osm_key": "place"})
	ps := newPhotonStub(t, photonAnswers(t, bad, outOfRange, indianCity("Good")))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.Search(context.Background(), "Good", nil)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(answer.Results) != 1 || answer.Results[0].Name != "Good" {
		t.Fatalf("results = %+v, want only Good", answer.Results)
	}
}

func TestPhotonLabels(t *testing.T) {
	cases := []struct {
		name  string
		props photonProperties
		want  string
	}{
		{
			"house with no name",
			photonProperties{HouseNumber: "8-2-293/82", Street: "Road No. 12", District: "Banjara Hills", City: "Hyderabad", State: "Telangana"},
			"8-2-293/82, Road No. 12, Banjara Hills, Hyderabad, Telangana",
		},
		{
			"city named like itself",
			photonProperties{Name: "Hyderabad", City: "Hyderabad", State: "Telangana"},
			"Hyderabad, Telangana",
		},
		{
			"missing parts, null text and spaces",
			photonProperties{Name: " Banjara Hills ", Street: "null", Locality: "", District: "  ", City: "Hyderabad", State: "NULL"},
			"Banjara Hills, Hyderabad",
		},
		{
			"repeat in other case",
			photonProperties{Name: "Gachibowli", Locality: "gachibowli", City: "Hyderabad", State: "Telangana"},
			"Gachibowli, Hyderabad, Telangana",
		},
		{"nothing at all", photonProperties{}, ""},
		// Quick 261016-sfu: live, some Hyderabad rows carry a ward as the
		// district and no city. The county then names the city.
		{
			"ward as district and no city: the county names the city",
			photonProperties{Name: "Banjara Hills", District: "Ward 92 Venkateshwara Colony", County: "Hyderabad", State: "Telangana"},
			"Banjara Hills, Ward 92 Venkateshwara Colony, Hyderabad, Telangana",
		},
		{
			"a city is kept and the county left out",
			photonProperties{Name: "Gachibowli", City: "Hyderabad", County: "Rangareddy", State: "Telangana"},
			"Gachibowli, Hyderabad, Telangana",
		},
		{
			"a county named like the place is not repeated",
			photonProperties{Name: "Hyderabad", County: "hyderabad", State: "Telangana"},
			"Hyderabad, Telangana",
		},
		{
			"a null county is skipped",
			photonProperties{Name: "Kondapur", City: " ", County: "null", State: "Telangana"},
			"Kondapur, Telangana",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := photonLabel(tc.props)
			if got != tc.want {
				t.Fatalf("photonLabel = %q, want %q", got, tc.want)
			}
			if strings.Contains(strings.ToLower(got), "null") || strings.Contains(got, ", ,") || strings.HasPrefix(got, ",") || strings.HasSuffix(got, ",") {
				t.Fatalf("label %q holds null or an empty part", got)
			}
		})
	}
}

// TestPhotonReadsTheCounty pins the county as Photon sends it, by its JSON
// name, on a row with no city.
func TestPhotonReadsTheCounty(t *testing.T) {
	area := photonFeatureJSON(78.4347, 17.4156, map[string]any{
		"name": "Banjara Hills", "district": "Ward 92 Venkateshwara Colony", "county": "Hyderabad",
		"state": "Telangana", "countrycode": "IN", "osm_key": "place", "osm_value": "suburb",
	})
	ps := newPhotonStub(t, photonAnswers(t, area))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	answer, err := c.Search(context.Background(), "Banjra Hills", nil)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(answer.Results) != 1 || answer.Results[0].DisplayName != "Banjara Hills, Ward 92 Venkateshwara Colony, Hyderabad, Telangana" {
		t.Fatalf("results = %+v, want the label to name Hyderabad", answer.Results)
	}
}

func TestPhotonHouseWithNoNameKeepsAnEmptyName(t *testing.T) {
	house := photonFeatureJSON(78.4347, 17.4156, map[string]any{
		"housenumber": "8-2-293/82", "street": "Road No. 12", "district": "Banjara Hills",
		"city": "Hyderabad", "state": "Telangana", "countrycode": "IN", "osm_key": "place", "osm_value": "house", "type": "house",
	})
	ps := newPhotonStub(t, photonAnswers(t, house))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.Search(context.Background(), "Road No 12 Banjara Hills Hyderabad", nil)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(answer.Results) != 1 {
		t.Fatalf("results = %+v, want one", answer.Results)
	}
	got := answer.Results[0]
	if got.Name != "" || got.DisplayName != "8-2-293/82, Road No. 12, Banjara Hills, Hyderabad, Telangana" {
		t.Fatalf("result = %+v, want no name and the built label", got)
	}
}

func TestPhotonRanksCloseSpellingsOfPlacesFirst(t *testing.T) {
	names := func(rs []Result) []string {
		var out []string
		for _, r := range rs {
			out = append(out, r.Name)
		}
		return out
	}
	shop := func(name string) map[string]any {
		return photonFeatureJSON(78.47, 17.36, map[string]any{
			"name": name, "street": "Abids Road", "city": "Hyderabad", "countrycode": "IN",
			"osm_key": "shop", "osm_value": "clothes", "type": "house",
		})
	}
	amenity := func(name string) map[string]any {
		return photonFeatureJSON(78.41, 17.42, map[string]any{
			"name": name, "city": "Hyderabad", "countrycode": "IN",
			"osm_key": "amenity", "osm_value": "hospital", "type": "house",
		})
	}
	road := func(name string) map[string]any {
		return photonFeatureJSON(78.41, 17.421, map[string]any{
			"name": name, "city": "Hyderabad", "countrycode": "IN",
			"osm_key": "highway", "osm_value": "residential", "type": "street",
		})
	}

	cases := []struct {
		name     string
		q        string
		features []map[string]any
		want     []string
	}{
		{
			// Live 2026-10-16: Photon put a shop named after the typo first.
			"Hyderbad shows the city first",
			"Hyderbad",
			[]map[string]any{shop("Hyderbad"), indianCity("Hyderabad"), shop("Hyderbad Collection")},
			[]string{"Hyderabad", "Hyderbad", "Hyderbad Collection"},
		},
		{
			"an exact hospital name keeps its place above a longer road name",
			"Apollo Hospital",
			[]map[string]any{amenity("Apollo Hospital"), road("Apollo Hospital Road")},
			[]string{"Apollo Hospital", "Apollo Hospital Road"},
		},
		{
			"an exact place name beats a station with the same name",
			"Ameerpet",
			[]map[string]any{amenity("Ameerpet"), indianCity("Ameerpet")},
			[]string{"Ameerpet", "Ameerpet"},
		},
		{
			"no close place leaves photon's order alone",
			"Gachib",
			[]map[string]any{shop("Gachibowli Stores"), indianCity("Gachibowli")},
			[]string{"Gachibowli Stores", "Gachibowli"},
		},
		{
			"short text needs an exact name",
			"Abid",
			[]map[string]any{shop("Abid Shoes"), indianCity("Abids")},
			[]string{"Abid Shoes", "Abids"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps := newPhotonStub(t, photonAnswers(t, tc.features...))
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)
			answer, err := c.Search(context.Background(), tc.q, nil)
			if err != nil {
				t.Fatalf("Search returned error: %v", err)
			}
			if got := names(answer.Results); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("order = %q, want %q", got, tc.want)
			}
		})
	}

	// The exact place in the Ameerpet case must be the city, not the hospital.
	ps := newPhotonStub(t, photonAnswers(t, amenity("Ameerpet"), indianCity("Ameerpet")))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	answer, err := c.Search(context.Background(), "Ameerpet", nil)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if answer.Results[0].Lat != 17.385 {
		t.Fatalf("first result = %+v, want the place, not the hospital", answer.Results[0])
	}
}

func TestPhotonAnswerIsCutToFive(t *testing.T) {
	var features []map[string]any
	for i := 0; i < 10; i++ {
		features = append(features, indianCity("Place "+string(rune('A'+i))))
	}
	ps := newPhotonStub(t, photonAnswers(t, features...))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	answer, err := c.Search(context.Background(), "Place", nil)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(answer.Results) != 5 || answer.Results[0].Name != "Place A" || answer.Results[4].Name != "Place E" {
		t.Fatalf("results = %+v, want the first five", answer.Results)
	}
}

// TestPhotonDropsRepeatedLabels: live on 2026-10-16 "Hyderbad" listed
// "Hyderabad, Telangana" twice (the city and its district) and "Ring Road"
// listed one road twice (split OpenStreetMap ways). A row that reads the same
// as one above it is dropped, so the five rows are five different choices.
func TestPhotonDropsRepeatedLabels(t *testing.T) {
	district := photonFeatureJSON(78.47, 17.39, map[string]any{
		"name": "Hyderabad", "state": "Telangana", "countrycode": "IN", "osm_key": "boundary", "type": "county",
	})
	roadA := photonFeatureJSON(78.44, 17.33, map[string]any{
		"name": "Inner Ring Road", "locality": "New Balaji Nagar", "state": "Telangana", "countrycode": "IN", "osm_key": "highway",
	})
	roadB := photonFeatureJSON(78.45, 17.34, map[string]any{
		"name": "Inner Ring Road", "locality": "new balaji nagar", "state": "Telangana", "countrycode": "IN", "osm_key": "highway",
	})
	ps := newPhotonStub(t, photonAnswers(t, indianCity("Hyderabad"), district, roadA, roadB, indianCity("Zaheerabad")))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	answer, err := c.Search(context.Background(), "Hyderbad", nil)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	var labels []string
	for _, r := range answer.Results {
		labels = append(labels, r.DisplayName)
	}
	want := []string{"Hyderabad, Telangana", "Inner Ring Road, New Balaji Nagar, Telangana", "Zaheerabad, Telangana"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %q, want %q", labels, want)
	}
	if answer.Results[0].Lat != 17.385 {
		t.Fatalf("first row = %+v, want the first of the repeated rows kept", answer.Results[0])
	}
}

func TestPhotonFailureFallsBackToNominatim(t *testing.T) {
	short1 := "Sai Residency, Road No 12, Banjara Hills, Hyderabad"
	cases := map[string]func(t *testing.T) func(w http.ResponseWriter, r *http.Request){
		"status 500": func(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
			return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }
		},
		"bad body": func(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
			return func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`not json`)) }
		},
		"timeout": func(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
			return func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-time.After(3 * time.Second):
				}
			}
		},
		// "Nothing found" is not a failure: since quick 261016-sfu it runs
		// the order pinned in order_test.go.
	}
	for name, respond := range cases {
		for lane, search := range lanes {
			t.Run(name+" "+lane, func(t *testing.T) {
				ps := newPhotonStub(t, respond(t))
				rs := newRecordingServer(t, func(q url.Values) bool { return q.Get("q") == short1 })
				c := newDualClient(ps.server.URL, rs.server.URL)
				c.photonTimeout = 100 * time.Millisecond

				start := time.Now()
				answer, err := search(c, context.Background(), longAddress, nil)
				if err != nil {
					t.Fatalf("search returned error: %v", err)
				}
				if elapsed := time.Since(start); elapsed > 2*time.Second {
					t.Errorf("search took %v, want the photon timeout to cut it short", elapsed)
				}
				if ps.count() != 1 {
					t.Errorf("photon got %d requests, want 1", ps.count())
				}
				if got, want := rs.texts(), []string{longAddress, short1}; !reflect.DeepEqual(got, want) {
					t.Fatalf("nominatim texts = %q, want %q", got, want)
				}
				if answer.MatchedQuery != short1 || len(answer.Results) != 1 || answer.provider != providerNominatim {
					t.Fatalf("answer = %+v, want the nominatim answer matched on %q", answer, short1)
				}
			})
		}
	}
}

// TestRequestCapIsFourAcrossBothProviders pins the reading of the cap: four
// upstream requests per search in total, so a search whose Photon request
// failed has three Nominatim tries left. When Photon finds nothing instead,
// TestOrderWhenPhotonFindsNothing pins the order.
func TestRequestCapIsFourAcrossBothProviders(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)
			near := &Point{Lat: 17.4, Lon: 78.45}
			answer, err := search(c, context.Background(), longAddress, near)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if total := ps.count() + len(rs.sent()); total != 4 || ps.count() != 1 {
				t.Fatalf("photon %d + nominatim %d requests, want 1 + 3", ps.count(), len(rs.sent()))
			}
			sent := rs.sent()
			if sent[0].Get("bounded") != "1" || sent[1].Has("bounded") || sent[2].Get("q") != "Sai Residency, Road No 12, Banjara Hills, Hyderabad" {
				t.Fatalf("nominatim tries = %v, want full boxed, full widened, first shorter boxed", sent)
			}
			if answer.Results == nil || len(answer.Results) != 0 {
				t.Fatalf("answer = %+v, want an empty non nil list", answer)
			}
		})
	}
}

func TestPhotonCache(t *testing.T) {
	t.Run("a photon answer is served from its cache on both lanes", func(t *testing.T) {
		ps := newPhotonStub(t, photonAnswers(t, indianCity("Hyderabad")))
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		near := &Point{Lat: 17.40, Lon: 78.45}
		if _, err := c.Search(context.Background(), "Hyderabad", near); err != nil {
			t.Fatalf("Search returned error: %v", err)
		}
		again, err := c.SearchReader(context.Background(), "  HYDERABAD ", &Point{Lat: 17.41, Lon: 78.46})
		if err != nil {
			t.Fatalf("SearchReader returned error: %v", err)
		}
		if ps.count() != 1 || len(rs.sent()) != 0 {
			t.Fatalf("photon %d, nominatim %d requests; want 1 and 0", ps.count(), len(rs.sent()))
		}
		if len(again.Results) != 1 || again.Results[0].Name != "Hyderabad" {
			t.Fatalf("cached answer = %+v", again)
		}
		// Another area is another entry.
		if _, err := c.Search(context.Background(), "Hyderabad", &Point{Lat: 28.61, Lon: 77.21}); err != nil {
			t.Fatalf("Search returned error: %v", err)
		}
		if ps.count() != 2 {
			t.Fatalf("photon got %d requests, want 2 for another area", ps.count())
		}
	})
	t.Run("nothing found is remembered, and the fallback answer too", func(t *testing.T) {
		ps := newPhotonStub(t, photonAnswers(t))
		rs := newRecordingServer(t, func(url.Values) bool { return true })
		c := newDualClient(ps.server.URL, rs.server.URL)
		for i := 0; i < 2; i++ {
			answer, err := c.Search(context.Background(), "Banjara Hills", nil)
			if err != nil {
				t.Fatalf("Search %d returned error: %v", i, err)
			}
			if len(answer.Results) != 1 {
				t.Fatalf("Search %d answer = %+v", i, answer)
			}
		}
		if ps.count() != 1 || len(rs.sent()) != 1 {
			t.Fatalf("photon %d, nominatim %d requests; want 1 and 1", ps.count(), len(rs.sent()))
		}
	})
	t.Run("a photon error is never cached", func(t *testing.T) {
		ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
		rs := newRecordingServer(t, func(url.Values) bool { return true })
		c := newDualClient(ps.server.URL, rs.server.URL)
		for i := 0; i < 2; i++ {
			if _, err := c.SearchReader(context.Background(), "Banjara Hills", nil); err != nil {
				t.Fatalf("SearchReader %d returned error: %v", i, err)
			}
		}
		if ps.count() != 2 {
			t.Fatalf("photon got %d requests, want 2 (an error must not be cached)", ps.count())
		}
		if c.photon.size() != 0 {
			t.Fatalf("photon cache holds %d entries after errors, want 0", c.photon.size())
		}
	})
	t.Run("an empty fallback answer after a photon error is not cached", func(t *testing.T) {
		ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
		rs := newRecordingServer(t, nil)
		c := newDualClient(ps.server.URL, rs.server.URL)
		for i := 0; i < 2; i++ {
			answer, err := c.Search(context.Background(), "Banjra Hills", nil)
			if err != nil {
				t.Fatalf("Search %d returned error: %v", i, err)
			}
			if answer.Results == nil || len(answer.Results) != 0 {
				t.Fatalf("Search %d answer = %+v, want an empty list", i, answer)
			}
		}
		if ps.count() != 2 || c.reporter.size() != 0 {
			t.Fatalf("photon %d requests, reporter cache %d entries; want 2 and 0", ps.count(), c.reporter.size())
		}
	})
}

// TestReaderNeverQueuesAheadOfAReporterOnPhoton is D-16 on Photon's own
// limiter: with a reporter waiting for Photon's next token, a reader gives up
// busy at once and sends nothing to either provider.
func TestReaderNeverQueuesAheadOfAReporterOnPhoton(t *testing.T) {
	ps := newPhotonStub(t, photonAnswers(t, indianCity("Hyderabad")))
	rs := newRecordingServer(t, nil)
	c := newClientWithProviders(ps.server.URL, rs.server.URL, "", time.Millisecond, 10, 300*time.Millisecond, 1)

	// The first reporter takes Photon's only token.
	if _, err := c.Search(context.Background(), "first", nil); err != nil {
		t.Fatalf("first Search returned error: %v", err)
	}
	// The second waits for the next one, which puts the bucket in debt.
	done := make(chan error, 1)
	go func() {
		_, err := c.Search(context.Background(), "second", nil)
		done <- err
	}()
	time.Sleep(60 * time.Millisecond)

	start := time.Now()
	_, err := c.SearchReader(context.Background(), "reader query", nil)
	if !errors.Is(err, ErrReaderBusy) {
		t.Fatalf("SearchReader error = %v, want ErrReaderBusy", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("SearchReader took %v, want it not to queue", elapsed)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waiting Search returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiting Search was never answered")
	}
	if ps.count() != 2 || len(rs.sent()) != 0 {
		t.Fatalf("photon %d, nominatim %d requests; want 2 (both reporters) and 0", ps.count(), len(rs.sent()))
	}
}

func TestReporterFallsBackWhenPhotonsLimiterRunsOut(t *testing.T) {
	ps := newPhotonStub(t, photonAnswers(t, indianCity("Hyderabad")))
	rs := newRecordingServer(t, func(url.Values) bool { return true })
	c := newClientWithProviders(ps.server.URL, rs.server.URL, "", time.Millisecond, 10, 10*time.Second, 1)

	if _, err := c.Search(context.Background(), "first", nil); err != nil {
		t.Fatalf("first Search returned error: %v", err)
	}
	start := time.Now()
	answer, err := c.Search(context.Background(), "second", nil)
	if err != nil {
		t.Fatalf("second Search returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("second Search took %v, want the photon wait bounded", elapsed)
	}
	if ps.count() != 1 || len(rs.sent()) != 1 || answer.provider != providerNominatim {
		t.Fatalf("photon %d, nominatim %d, provider %q; want 1, 1 and nominatim", ps.count(), len(rs.sent()), answer.provider)
	}
}

func TestCancelledSearchDuringPhotonDoesNotFallBack(t *testing.T) {
	const secretQuery = "flooded-colony-near-my-house"
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) {
				cancel()
				select {
				case <-r.Context().Done():
				case <-time.After(3 * time.Second):
				}
			})
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)

			_, err := search(c, ctx, secretQuery, &Point{Lat: 13.082680, Lon: 80.270718})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			var urlErr *url.Error
			if errors.As(err, &urlErr) {
				t.Fatalf("error still wraps a *url.Error: %v", err)
			}
			for _, secret := range []string{secretQuery, "13.082680", "80.270718", "bbox"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error %q contains %q", err.Error(), secret)
				}
			}
			if n := len(rs.sent()); n != 0 {
				t.Fatalf("nominatim got %d requests after the search was cancelled, want 0", n)
			}
		})
	}
}

func TestCloseSpelling(t *testing.T) {
	cases := []struct {
		q, name string
		want    bool
	}{
		{"Hyderbad", "Hyderabad", true},
		{"Banjra Hills", "Banjara Hills", true},
		{"banjara  hills", "Banjara Hills", true},
		{"Hyderabad", "Hyderabad", true},
		{"Abid", "Abids", false},
		{"Abids", "Abids", true},
		{"Gachib", "Gachibowli", false},
		{"Hyderbad", "", false},
		{"హైదరాబాద్", "హైదరాబాద్", true},
	}
	for _, tc := range cases {
		if got := closeSpelling(tc.q, tc.name); got != tc.want {
			t.Errorf("closeSpelling(%q, %q) = %v, want %v", tc.q, tc.name, got, tc.want)
		}
	}
}
