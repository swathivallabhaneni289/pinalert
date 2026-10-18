package geocode

// Tests for Phase 3.4 plan 35: the typed row survives a Photon that fails, times
// out or answers busy. The extra lookups of the typed row (town probe, any place
// match, PIN code) fall back to Nominatim inside the per search cap of four, and
// no_match is true only when every lookup that was needed actually answered.
// Every upstream is an httptest stand in.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// nominatimStub is a Nominatim stand in that records every query and answers
// with whatever route returns for it (a nil slice is an empty answer, a nil
// route a 503).
type nominatimStub struct {
	mu      sync.Mutex
	queries []url.Values
	server  *httptest.Server
}

func newNominatimStub(t *testing.T, route func(q url.Values) []map[string]any) *nominatimStub {
	t.Helper()
	ns := &nominatimStub{}
	ns.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		ns.mu.Lock()
		ns.queries = append(ns.queries, q)
		ns.mu.Unlock()
		if route == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		rows := route(q)
		if rows == nil {
			rows = []map[string]any{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rows)
	}))
	t.Cleanup(ns.server.Close)
	return ns
}

func (ns *nominatimStub) sent() []url.Values {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	return append([]url.Values(nil), ns.queries...)
}

func nomRow(lat, lon float64, name, display, category, addressType string) map[string]any {
	return map[string]any{
		"name": name, "display_name": display,
		"lat": formatCoord(lat), "lon": formatCoord(lon),
		"category": category, "addresstype": addressType, "type": addressType,
	}
}

func formatCoord(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func nomVijayawadaCity() map[string]any {
	return nomRow(16.5115306, 80.6160469, "Vijayawada", "Vijayawada, NTR District, Andhra Pradesh, India", "boundary", "city")
}

func nomGayathriBengaluru() map[string]any {
	return nomRow(13.0, 77.55, "Gayathri Nagar", "Gayathri Nagar, Bengaluru, Karnataka, India", "place", "suburb")
}

// photonDownStub answers every request with a 503.
func photonDownStub(t *testing.T) *photonStub {
	t.Helper()
	return newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
}

// photonSlowStub answers after the delay, which the test sets longer than the
// client's Photon time limit.
func photonSlowStub(t *testing.T, delay time.Duration) *photonStub {
	t.Helper()
	return newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(photonBody(t))
	})
}

func fastTimeouts(c *Client) *Client {
	c.photonTimeout = 40 * time.Millisecond
	c.photonRetryTimeout = 40 * time.Millisecond
	return c
}

func nomTotal(ps *photonStub, ns *nominatimStub) int { return ps.count() + len(ns.sent()) }

func requireTypedRowAt(t *testing.T, answer Answer, name, town string, lat, lon float64) {
	t.Helper()
	row, ok := typedRowOf(t, answer)
	if !ok {
		t.Fatalf("no typed row in %+v", answer.Results)
	}
	if row.Name != name || row.Town != town {
		t.Errorf("typed row = %+v, want %q in %q", row, name, town)
	}
	if row.Lat != lat || row.Lon != lon {
		t.Errorf("typed row at %v, %v, want %v, %v", row.Lat, row.Lon, lat, lon)
	}
	if answer.NoCloseMatch {
		t.Error("NoCloseMatch is true although a typed row was placed")
	}
}

// The owner's text on 2026-10-18: Photon failed, Nominatim answered the whole
// text with Gayathri Nagar in Bengaluru, and no typed row came back.
func TestTownProbeFallsBackToNominatimWhenPhotonFails(t *testing.T) {
	for name, build := range map[string]func(t *testing.T) *photonStub{
		"photon answers 503": photonDownStub,
		"photon times out":   func(t *testing.T) *photonStub { return photonSlowStub(t, 400*time.Millisecond) },
	} {
		t.Run(name, func(t *testing.T) {
			ps := build(t)
			ns := newNominatimStub(t, func(q url.Values) []map[string]any {
				switch q.Get("q") {
				case "Gayathri Nagar Vijayawada":
					return []map[string]any{nomGayathriBengaluru()}
				case "Vijayawada":
					return []map[string]any{nomVijayawadaCity()}
				}
				return nil
			})
			c := fastTimeouts(newDualClient(ps.server.URL, ns.server.URL))

			answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar Vijayawada", nil)
			if err != nil {
				t.Fatalf("SearchReaderAreas returned error: %v", err)
			}
			requireTypedRowAt(t, answer, "Gayathri Nagar, Vijayawada", "Vijayawada", 16.5115306, 80.6160469)
			if n := nomTotal(ps, ns); n > maxUpstreamPerSearch {
				t.Errorf("%d upstream requests, the cap is %d", n, maxUpstreamPerSearch)
			}
		})
	}
}

// The recorded live answers of 2026-10-18 for the owner's text, replayed with
// Photon down: Nominatim answers the whole text and "Nagar Vijayawada" with
// nothing and "Vijayawada" with the city (place, city), so the search spends all
// four requests and the typed row is placed from the answer in hand, with no
// request left over.
func TestOwnersTextWithPhotonDownUsesTheRecordedNominatimAnswers(t *testing.T) {
	ps := photonDownStub(t)
	ns := newNominatimStub(t, func(q url.Values) []map[string]any {
		if q.Get("q") == "Vijayawada" {
			return []map[string]any{nomRow(16.5115306, 80.6160469, "Vijayawada",
				"Vijayawada, Vijayawada Urban Mandal, NTR District, Andhra Pradesh, 520001, India", "place", "city")}
		}
		return nil
	})
	c := newDualClient(ps.server.URL, ns.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar Vijayawada", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	requireTypedRowAt(t, answer, "Gayathri Nagar, Vijayawada", "Vijayawada", 16.5115306, 80.6160469)
	if n := nomTotal(ps, ns); n != maxUpstreamPerSearch {
		t.Errorf("%d upstream requests, want exactly %d (the Photon try and three Nominatim texts)", n, maxUpstreamPerSearch)
	}
}

// The any place match (a street in the typed town) is found through Nominatim
// when Photon fails, and the lookup is not an areas lookup (no featureType).
func TestAnyPlaceMatchFallsBackToNominatim(t *testing.T) {
	ps := photonDownStub(t)
	ns := newNominatimStub(t, func(q url.Values) []map[string]any {
		if q.Get("featureType") != "" {
			return []map[string]any{nomRow(16.5302, 80.6316, "Ayodhya Nagar", "Ayodhya Nagar, Vijayawada, Andhra Pradesh, India", "place", "suburb")}
		}
		return []map[string]any{nomRow(16.5035096, 80.6546059, "Gayatri Nagar Main", "Gayatri Nagar Main, Vijayawada, Andhra Pradesh, India", "highway", "road")}
	})
	c := newDualClient(ps.server.URL, ns.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	requireTypedRowAt(t, answer, "Gayathri Nagar, Vijayawada", "Vijayawada", 16.5035096, 80.6546059)
	sent := ns.sent()
	if len(sent) != 2 || sent[0].Get("featureType") != "settlement" || sent[1].Get("featureType") != "" {
		t.Errorf("nominatim queries = %v, want the areas search then one any place lookup", sent)
	}
	if n := nomTotal(ps, ns); n > maxUpstreamPerSearch {
		t.Errorf("%d upstream requests, the cap is %d", n, maxUpstreamPerSearch)
	}
}

// The PIN lookup falls back too: Nominatim's postcode row names the town in its
// label (live: "520008, Vijayawada Urban Mandal, NTR District, Andhra Pradesh").
func TestPinLookupFallsBackToNominatim(t *testing.T) {
	ps := photonDownStub(t)
	ns := newNominatimStub(t, func(q url.Values) []map[string]any {
		switch q.Get("q") {
		case "Gayathri Nagar":
			return []map[string]any{nomGayathriBengaluru()}
		case "520008":
			return []map[string]any{nomRow(16.5102171, 80.6670397, "520008", "520008, Vijayawada Urban Mandal, NTR District, Andhra Pradesh, India", "place", "postcode")}
		}
		return nil
	})
	c := newDualClient(ps.server.URL, ns.server.URL)

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar 520008", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	requireTypedRowAt(t, answer, "Gayathri Nagar, Vijayawada", "Vijayawada", 16.5102171, 80.6670397)
	if n := nomTotal(ps, ns); n > maxUpstreamPerSearch {
		t.Errorf("%d upstream requests, the cap is %d", n, maxUpstreamPerSearch)
	}
}

// Photon timing out twice and Nominatim answering the whole text spends three of
// the four requests. The one left must go to the town when the town is named in
// the answer in hand, because the town always places the row.
func TestTheLastRequestGoesToTheTownWhenOnlyOneIsLeft(t *testing.T) {
	ps := photonSlowStub(t, 400*time.Millisecond)
	ns := newNominatimStub(t, func(q url.Values) []map[string]any {
		switch q.Get("q") {
		case "Gayathri Nagar Vijayawada":
			return []map[string]any{nomRow(16.5, 80.64, "Krishnalanka", "Krishnalanka, Vijayawada, NTR District, Andhra Pradesh, India", "place", "suburb")}
		case "Vijayawada":
			return []map[string]any{nomVijayawadaCity()}
		}
		return nil
	})
	c := fastTimeouts(newDualClient(ps.server.URL, ns.server.URL))

	answer, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar Vijayawada", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	requireTypedRowAt(t, answer, "Gayathri Nagar, Vijayawada", "Vijayawada", 16.5115306, 80.6160469)
	if n := nomTotal(ps, ns); n != maxUpstreamPerSearch {
		t.Errorf("%d upstream requests, want exactly %d", n, maxUpstreamPerSearch)
	}
}

// Photon is not asked again for the lookups once it failed in the same search:
// a failed try costs one of the four requests, so the lookups go straight to
// Nominatim.
func TestLookupsSkipPhotonOnceItFailedInThisSearch(t *testing.T) {
	ps := photonDownStub(t)
	ns := newNominatimStub(t, func(q url.Values) []map[string]any {
		switch q.Get("q") {
		case "Gayathri Nagar Vijayawada":
			return []map[string]any{nomGayathriBengaluru()}
		case "Vijayawada":
			return []map[string]any{nomVijayawadaCity()}
		}
		return nil
	})
	c := newDualClient(ps.server.URL, ns.server.URL)
	if _, err := c.SearchReaderAreas(context.Background(), "Gayathri Nagar Vijayawada", nil); err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if ps.count() != 1 {
		t.Errorf("photon got %d requests, want 1 (the search's own try only)", ps.count())
	}
}

// A Photon that works keeps the Nominatim budget untouched.
func TestNominatimIsNotAskedWhenPhotonAnswersTheLookups(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any {
		if isAreasQuery(q) && q.Get("q") == "Vijayawada" {
			return []map[string]any{vijayawadaCity()}
		}
		if isAreasQuery(q) {
			return []map[string]any{benguluruCircle()}
		}
		return nil
	})
	ns := newNominatimStub(t, func(url.Values) []map[string]any { return nil })
	c := newDualClient(ps.server.URL, ns.server.URL)
	answer, err := c.SearchReaderAreas(context.Background(), "Benz Circle Vijayawada", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if _, ok := typedRowOf(t, answer); !ok {
		t.Fatalf("no typed row in %+v", answer.Results)
	}
	if n := len(ns.sent()); n != 0 {
		t.Errorf("nominatim got %d requests, want 0", n)
	}
}

// The no_match rule. The search answered with rows that do not match what was
// typed (so Not found would be the honest line), and then the lookups for the
// typed row either answered with nothing or failed.
func TestNoMatchIsTrueOnlyWhenEveryNeededLookupAnswered(t *testing.T) {
	mainRows := func(q url.Values) []map[string]any {
		if isAreasQuery(q) && q.Get("q") == "Benz Circle Vijayawada" {
			return []map[string]any{benguluruCircle()}
		}
		return nil
	}
	t.Run("every lookup answered with nothing", func(t *testing.T) {
		ps := routedPhoton(t, mainRows)
		ns := newNominatimStub(t, func(url.Values) []map[string]any { return nil })
		c := newDualClient(ps.server.URL, ns.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Benz Circle Vijayawada", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if !answer.NoCloseMatch || answer.Unfinished {
			t.Errorf("NoCloseMatch = %v, Unfinished = %v, want true and false: the lookups answered and nothing was placed (%+v)", answer.NoCloseMatch, answer.Unfinished, answer.Results)
		}
	})
	t.Run("the search itself was answered by nominatim after photon failed, with no row that matches", func(t *testing.T) {
		// "Benz Circle" has no lookup at all: only the search can be unfinished.
		ps := photonDownStub(t)
		ns := newNominatimStub(t, func(q url.Values) []map[string]any {
			return []map[string]any{nomRow(13.036, 77.562, "Nethaji Circle", "Nethaji Circle, Bengaluru, Karnataka, India", "place", "locality")}
		})
		c := newDualClient(ps.server.URL, ns.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Benz Circle", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if answer.NoCloseMatch || !answer.Unfinished {
			t.Errorf("NoCloseMatch = %v, Unfinished = %v, want false and true", answer.NoCloseMatch, answer.Unfinished)
		}
		if len(answer.Results) != 1 {
			t.Errorf("results = %+v, want the one row kept", answer.Results)
		}
	})
	t.Run("photon failed but nominatim found a row that matches", func(t *testing.T) {
		ps := photonDownStub(t)
		ns := newNominatimStub(t, func(q url.Values) []map[string]any {
			return []map[string]any{nomRow(12.936, 77.624, "Koramangala", "Koramangala, Bengaluru, Karnataka, India", "place", "suburb")}
		})
		c := newDualClient(ps.server.URL, ns.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Koramangala", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if answer.NoCloseMatch || answer.Unfinished {
			t.Errorf("NoCloseMatch = %v, Unfinished = %v, want both false for a match", answer.NoCloseMatch, answer.Unfinished)
		}
	})
	t.Run("photon timed out once and the retry answered", func(t *testing.T) {
		var calls atomic.Int32
		ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) {
			n := calls.Add(1)
			if n == 1 {
				time.Sleep(300 * time.Millisecond)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(photonBody(t, benguluruCircle()))
		})
		ns := newNominatimStub(t, func(url.Values) []map[string]any { return nil })
		c := fastTimeouts(newDualClient(ps.server.URL, ns.server.URL))
		answer, err := c.SearchReaderAreas(context.Background(), "Benz Circle", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if !answer.NoCloseMatch || answer.Unfinished {
			t.Errorf("NoCloseMatch = %v, Unfinished = %v, want true and false: the retry answered", answer.NoCloseMatch, answer.Unfinished)
		}
	})
	t.Run("photon and nominatim both fail the lookups", func(t *testing.T) {
		ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("q") == "Benz Circle Vijayawada" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(photonBody(t, benguluruCircle()))
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
		})
		ns := newNominatimStub(t, nil)
		c := newDualClient(ps.server.URL, ns.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Benz Circle Vijayawada", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if answer.NoCloseMatch || !answer.Unfinished {
			t.Errorf("NoCloseMatch = %v, Unfinished = %v although the lookups failed, want false and true (%+v)", answer.NoCloseMatch, answer.Unfinished, answer.Results)
		}
		if _, ok := typedRowOf(t, answer); ok {
			t.Errorf("a typed row was placed from failed lookups: %+v", answer.Results)
		}
	})
	t.Run("the cap left no room for the lookup", func(t *testing.T) {
		ps := routedPhoton(t, func(q url.Values) []map[string]any { return nil })
		ns := newNominatimStub(t, func(url.Values) []map[string]any { return nil })
		c := newDualClient(ps.server.URL, ns.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Kharghar Patamata Navi Mumbai 400710", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if answer.NoCloseMatch || !answer.Unfinished {
			t.Errorf("NoCloseMatch = %v, Unfinished = %v although the lookups were never sent, want false and true", answer.NoCloseMatch, answer.Unfinished)
		}
	})
	t.Run("nothing needed a lookup", func(t *testing.T) {
		ps := routedPhoton(t, func(q url.Values) []map[string]any { return nil })
		ns := newNominatimStub(t, func(url.Values) []map[string]any { return nil })
		c := newDualClient(ps.server.URL, ns.server.URL)
		answer, err := c.SearchReaderAreas(context.Background(), "Zzqqxx Nagar", nil)
		if err != nil {
			t.Fatalf("SearchReaderAreas returned error: %v", err)
		}
		if !answer.NoCloseMatch {
			t.Error("NoCloseMatch = false for text that nothing matches and that needed no lookup")
		}
	})
}

// An empty answer is not found, also for text that looks like a street address
// (the page shows Not found for an empty list, as before plan 35).
func TestEmptyCompleteAnswerIsNotFoundEvenForAStreetAddress(t *testing.T) {
	ps := routedPhoton(t, func(q url.Values) []map[string]any { return nil })
	ns := newNominatimStub(t, func(url.Values) []map[string]any { return nil })
	c := newDualClient(ps.server.URL, ns.server.URL)
	answer, err := c.SearchReaderAreas(context.Background(), "12 Gayathri Nagar", nil)
	if err != nil {
		t.Fatalf("SearchReaderAreas returned error: %v", err)
	}
	if len(answer.Results) != 0 || !answer.NoCloseMatch {
		t.Errorf("results %+v, NoCloseMatch %v, want an empty answer that is not found", answer.Results, answer.NoCloseMatch)
	}
}

func TestRegionFromPostcodeLabel(t *testing.T) {
	cases := map[string]string{
		"520008, Vijayawada Urban Mandal, NTR District, Andhra Pradesh, India": "Vijayawada",
		"560034, Bangalore South, Karnataka, India":                            "Bangalore South",
		"400710, Thane, Maharashtra, India":                                    "Thane",
		"520008":                                                               "",
		"520008, India":                                                        "",
	}
	for display, want := range cases {
		pin, _, _ := strings.Cut(display, ",")
		if got := regionFromPostcodeLabel(pin, display); got != want {
			t.Errorf("regionFromPostcodeLabel(%q) = %q, want %q", display, got, want)
		}
	}
}
