package geocode

// Tests for Phase 3.4 plan 28, task 2: the areas only mode of the reader lane
// (SearchReaderAreas), used by the home area fields. Photon is restricted with
// its layer parameter and the rows are checked again here; Nominatim is
// restricted with featureType=settlement and its rows are checked again. Every
// upstream is an httptest stand in; the live parameters were checked once by
// hand and are recorded in 03.4-28-SUMMARY.md.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"
)

func photonRow(lon, lat float64, name, osmKey, osmValue, typ string) map[string]any {
	return photonFeatureJSON(lon, lat, map[string]any{
		"name": name, "city": "Bengaluru", "state": "Karnataka", "country": "India", "countrycode": "IN",
		"osm_key": osmKey, "osm_value": osmValue, "type": typ,
	})
}

// A mixed Photon answer: two real areas, a residential land use area (how
// OpenStreetMap draws Kothrud), and rows that must never be offered.
func mixedPhotonRows() []map[string]any {
	return []map[string]any{
		photonRow(77.64, 12.98, "Indiranagar", "boundary", "administrative", "district"),
		photonRow(77.58, 12.97, "Indira Road", "highway", "residential", "street"),
		photonRow(77.60, 12.96, "Indira Hospital", "amenity", "hospital", "house"),
		photonRow(77.61, 12.95, "Indira Mills", "landuse", "industrial", "locality"),
		photonRow(77.62, 12.94, "Indira Plaza", "landuse", "commercial", "locality"),
		photonRow(73.80, 18.50, "Indira Nagar", "landuse", "residential", "locality"),
		photonRow(77.63, 12.93, "Indira Layout", "place", "suburb", "locality"),
		photonRow(77.66, 12.92, "Indira Villa", "place", "house", "house"),
	}
}

func resultNames(answer Answer) []string {
	names := []string{}
	for _, r := range answer.Results {
		names = append(names, r.Name)
	}
	return names
}

func TestAreasPhotonSendsTheAreaLayersAndNormalSearchDoesNot(t *testing.T) {
	ps := newPhotonStub(t, photonAnswers(t, indianCity("Hyderabad")))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	if _, err := c.SearchReaderAreas(context.Background(), "Hyderabad", nil); err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if got, want := ps.last()["layer"], []string{"locality", "district", "city"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("areas mode layer = %q, want %q", got, want)
	}

	ps2 := newPhotonStub(t, photonAnswers(t, indianCity("Hyderabad")))
	c2 := newDualClient(ps2.server.URL, rs.server.URL)
	if _, err := c2.SearchReader(context.Background(), "Hyderabad", nil); err != nil {
		t.Fatalf("SearchReader returned error: %v", err)
	}
	if _, has := ps2.last()["layer"]; has {
		t.Fatalf("normal mode sent layer = %q, want none", ps2.last()["layer"])
	}
}

func TestAreasPhotonNeverReturnsStreetsBuildingsOrPointPlaces(t *testing.T) {
	ps := newPhotonStub(t, photonAnswers(t, mixedPhotonRows()...))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Indira", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	want := []string{"Indiranagar", "Indira Nagar", "Indira Layout"}
	if got := resultNames(answer); !reflect.DeepEqual(got, want) {
		t.Fatalf("areas mode names = %q, want %q", got, want)
	}

	// The same rows in normal mode keep everything, so the filter is the mode's.
	ps2 := newPhotonStub(t, photonAnswers(t, mixedPhotonRows()...))
	c2 := newDualClient(ps2.server.URL, rs.server.URL)
	normal, err := c2.SearchReader(context.Background(), "Indira", nil)
	if err != nil {
		t.Fatalf("SearchReader returned error: %v", err)
	}
	if len(normal.Results) != maxResults {
		t.Fatalf("normal mode returned %d rows, want %d: the areas filter must not touch it", len(normal.Results), maxResults)
	}
}

func TestAreasNominatimSendsFeatureTypeSettlementAndNormalSearchDoesNot(t *testing.T) {
	rs := newRecordingServer(t, func(url.Values) bool { return true })
	c := newClientWithBaseURL(rs.server.URL, "", defaultLimiterEvery, defaultLimiterBurst)
	if _, err := c.SearchReaderAreas(context.Background(), "Banjara", nil); err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if got := rs.sent()[0].Get("featureType"); got != "settlement" {
		t.Fatalf("areas mode featureType = %q, want settlement", got)
	}

	rs2 := newRecordingServer(t, func(url.Values) bool { return true })
	c2 := newClientWithBaseURL(rs2.server.URL, "", defaultLimiterEvery, defaultLimiterBurst)
	if _, err := c2.SearchReader(context.Background(), "Banjara", nil); err != nil {
		t.Fatalf("SearchReader returned error: %v", err)
	}
	if _, has := rs2.sent()[0]["featureType"]; has {
		t.Fatalf("normal mode sent featureType")
	}
}

func TestAreasNominatimNeverReturnsStreetsOrBuildings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
		 {"name":"MG Road","display_name":"MG Road, Shivajinagar, Bengaluru, India","lat":"12.97","lon":"77.60","category":"highway","type":"primary","addresstype":"road"},
		 {"name":"Indira Hospital","display_name":"Indira Hospital, Bengaluru, India","lat":"12.96","lon":"77.61","category":"amenity","type":"hospital","addresstype":"amenity"},
		 {"name":"Indiranagar","display_name":"Indiranagar, Jogpalya, Bengaluru, India","lat":"12.97","lon":"77.64","category":"place","type":"suburb","addresstype":"suburb"},
		 {"name":"Koramangala","display_name":"Koramangala, Bengaluru, India","lat":"12.93","lon":"77.62","category":"boundary","type":"administrative","addresstype":"city_district"}
		]`))
	}))
	defer server.Close()
	c := newClientWithBaseURL(server.URL, "", defaultLimiterEvery, defaultLimiterBurst)

	answer, err := c.SearchReaderAreas(context.Background(), "Indira", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if got, want := resultNames(answer), []string{"Indiranagar", "Koramangala"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("areas mode names = %q, want %q", got, want)
	}
}

func TestAreasModeReachesEveryUpstreamRequest(t *testing.T) {
	t.Run("photon failed, nominatim plan runs", func(t *testing.T) {
		ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusInternalServerError) })
		rs := newRecordingServer(t, func(url.Values) bool { return false })
		c := newDualClient(ps.server.URL, rs.server.URL)
		if _, err := c.SearchReaderAreas(context.Background(), "Sai Residency, Banjara Hills", nil); err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if len(rs.sent()) == 0 {
			t.Fatalf("nominatim was never asked")
		}
		for i, q := range rs.sent() {
			if q.Get("featureType") != "settlement" {
				t.Errorf("nominatim request %d lacks featureType=settlement", i)
			}
		}
	})
	t.Run("photon found nothing, nominatim and shorter photon tries run", func(t *testing.T) {
		ps := newPhotonStub(t, photonAnswers(t))
		rs := newRecordingServer(t, func(url.Values) bool { return false })
		c := newDualClient(ps.server.URL, rs.server.URL)
		if _, err := c.SearchReaderAreas(context.Background(), "Sai Residency, Banjara Hills, Hyderabad", nil); err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if len(rs.sent()) == 0 || ps.count() < 2 {
			t.Fatalf("nominatim got %d requests and photon %d, want both asked more than once", len(rs.sent()), ps.count())
		}
		for i, q := range rs.sent() {
			if q.Get("featureType") != "settlement" {
				t.Errorf("nominatim request %d lacks featureType=settlement", i)
			}
		}
		ps.mu.Lock()
		defer ps.mu.Unlock()
		for i, q := range ps.queries {
			if !reflect.DeepEqual(q["layer"], []string{"locality", "district", "city"}) {
				t.Errorf("photon request %d layer = %q", i, q["layer"])
			}
		}
	})
	t.Run("photon timed out, the retry is area restricted too", func(t *testing.T) {
		var mu sync.Mutex
		var seen []url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			seen = append(seen, r.URL.Query())
			n := len(seen)
			mu.Unlock()
			if n == 1 {
				<-r.Context().Done()
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(photonBody(t, indianCity("Hyderabad")))
		}))
		defer server.Close()
		rs := newRecordingServer(t, nil)
		c := newDualClient(server.URL, rs.server.URL)
		c.photonTimeout = 50 * time.Millisecond
		if _, err := c.SearchReaderAreas(context.Background(), "Hyderabad", nil); err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		mu.Lock()
		defer mu.Unlock()
		if len(seen) != 2 {
			t.Fatalf("photon got %d requests, want the try and its retry", len(seen))
		}
		for i, q := range seen {
			if len(q["layer"]) != 3 {
				t.Errorf("photon request %d layer = %q", i, q["layer"])
			}
		}
	})
}

func TestAreasAnswersAreCachedUnderTheirOwnKey(t *testing.T) {
	rows := mixedPhotonRows()
	ps := newPhotonStub(t, photonAnswers(t, rows...))
	rs := newRecordingServer(t, nil)
	c := newDualClient(ps.server.URL, rs.server.URL)
	ctx := context.Background()

	normal, err := c.SearchReader(ctx, "Indira", nil)
	if err != nil {
		t.Fatalf("SearchReader returned error: %v", err)
	}
	areas, err := c.SearchReaderAreas(ctx, "Indira", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if ps.count() != 2 {
		t.Fatalf("photon got %d requests, want 2: areas mode must not be served the normal answer from the cache", ps.count())
	}
	if len(areas.Results) >= len(normal.Results) {
		t.Fatalf("areas mode returned %d rows, normal %d: the cached normal answer leaked into areas mode", len(areas.Results), len(normal.Results))
	}

	// Each mode is cached: a repeat of either sends nothing.
	if _, err := c.SearchReaderAreas(ctx, "Indira", nil); err != nil {
		t.Fatalf("repeat areas search: %v", err)
	}
	if _, err := c.SearchReader(ctx, "Indira", nil); err != nil {
		t.Fatalf("repeat normal search: %v", err)
	}
	if ps.count() != 2 {
		t.Fatalf("photon got %d requests after the repeats, want still 2", ps.count())
	}
}

func TestAreasNominatimAnswersAreCachedUnderTheirOwnKey(t *testing.T) {
	rs := newRecordingServer(t, func(url.Values) bool { return true })
	c := newClientWithProviders("", rs.server.URL, "", time.Millisecond, 10, time.Millisecond, 10)
	ctx := context.Background()
	if _, err := c.SearchReader(ctx, "Banjara", nil); err != nil {
		t.Fatalf("SearchReader returned error: %v", err)
	}
	if _, err := c.SearchReaderAreas(ctx, "Banjara", nil); err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if n := len(rs.sent()); n != 2 {
		t.Fatalf("nominatim got %d requests, want 2: the modes must not share a cache key", n)
	}
}

func TestCacheKeyCarriesTheMode(t *testing.T) {
	normal := cacheKeyFor(context.Background(), "indira", nil)
	areas := cacheKeyFor(withAreasOnly(context.Background()), "indira", nil)
	if normal == areas {
		t.Fatalf("normal and areas cache keys are both %q", normal)
	}
	if normal != cacheKey("indira", nil) {
		t.Fatalf("the normal key changed: %q, want %q", normal, cacheKey("indira", nil))
	}
}
