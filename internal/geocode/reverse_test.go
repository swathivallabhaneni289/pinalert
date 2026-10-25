package geocode

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// Tests for the reverse lookup that names a report's area (Phase 3.3 plan 07).
// Both upstreams are httptest servers; nothing here reaches a real service.

// reverseUpstream is a fake geocoder that records every request it gets and
// answers with a fixed status and body.
type reverseUpstream struct {
	mu       sync.Mutex
	requests []*http.Request
	status   int
	body     string
	hang     bool
	server   *httptest.Server
}

func newReverseUpstream(t *testing.T, status int, body string) *reverseUpstream {
	t.Helper()
	u := &reverseUpstream{status: status, body: body}
	u.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		u.requests = append(u.requests, r)
		hang := u.hang
		u.mu.Unlock()
		if hang {
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(u.status)
		_, _ = w.Write([]byte(u.body))
	}))
	t.Cleanup(u.server.Close)
	return u
}

func (u *reverseUpstream) hits() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.requests)
}

func (u *reverseUpstream) first(t *testing.T) *http.Request {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.requests) == 0 {
		t.Fatal("the upstream got no request")
	}
	return u.requests[0]
}

// reverseClient builds a client with both providers on, pointed at the two
// fake upstreams through the reverse test seam.
func reverseClient(photon, nominatim *reverseUpstream) *Client {
	c := newClientWithProviders(photonSearchURL, nominatimSearchURL, "",
		defaultLimiterEvery, defaultLimiterBurst, photonLimiterEvery, photonLimiterBurst)
	return c.withReverseURLs(photon.server.URL+"/reverse", nominatim.server.URL+"/reverse")
}

// photonReverseBody builds a Photon answer with one feature.
func photonReverseBody(props string) string {
	return `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[77.5946,12.9716]},"properties":` + props + `}]}`
}

const emptyPhoton = `{"type":"FeatureCollection","features":[]}`

// drain empties a limiter so Allow() is false right now.
func drainLimiters(c *Client, photon, nominatim bool) {
	if photon {
		for c.photonLimiter.Allow() {
		}
	}
	if nominatim {
		for c.limiter.Allow() {
		}
	}
}

func TestReverseAreaCoarseOnly(t *testing.T) {
	const streetProps = `"street":"MG Road","housenumber":"12","postcode":"560001"`

	t.Run("a locality row gives its name, never the street, house or postcode", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, photonReverseBody(
			`{"name":"Central Market","type":"locality","district":"Shivajinagar","city":"Bengaluru",`+streetProps+`}`))
		nominatim := newReverseUpstream(t, 200, `{}`)
		c := reverseClient(photon, nominatim)

		got, err := c.ReverseArea(context.Background(), 12.9716, 77.5946)
		if err != nil {
			t.Fatalf("ReverseArea error = %v", err)
		}
		if got != "Central Market" {
			t.Fatalf("name = %q, want Central Market", got)
		}
		for _, bad := range []string{"MG Road", "12", "560001"} {
			if strings.Contains(got, bad) {
				t.Fatalf("name %q holds %q, which is not coarse", got, bad)
			}
		}
		if nominatim.hits() != 0 {
			t.Fatalf("Nominatim was asked %d times, want 0 when Photon answered", nominatim.hits())
		}

		req := photon.first(t)
		if req.URL.Path != "/reverse" {
			t.Fatalf("path = %q, want /reverse", req.URL.Path)
		}
		q := req.URL.Query()
		if lat, _ := strconv.ParseFloat(q.Get("lat"), 64); math.Abs(lat-12.9716) > 1e-9 {
			t.Fatalf("lat = %q", q.Get("lat"))
		}
		if lon, _ := strconv.ParseFloat(q.Get("lon"), 64); math.Abs(lon-77.5946) > 1e-9 {
			t.Fatalf("lon = %q", q.Get("lon"))
		}
		if q.Get("lang") != "en" || q.Get("limit") != "1" {
			t.Fatalf("lang = %q limit = %q, want en and 1", q.Get("lang"), q.Get("limit"))
		}
		layers := q["layer"]
		if len(layers) != 3 || layers[0] != "locality" || layers[1] != "district" || layers[2] != "city" {
			t.Fatalf("layers = %v, want locality, district, city only", layers)
		}
		if ua := req.Header.Get("User-Agent"); ua != c.userAgent() {
			t.Fatalf("User-Agent = %q, want %q", ua, c.userAgent())
		}
	})

	t.Run("a row that is not an area level takes locality, district, then city", func(t *testing.T) {
		cases := []struct {
			name, props, want string
		}{
			{"the row has no type, so its name is not trusted",
				`{"name":"Central Market","district":"Shivajinagar","city":"Bengaluru"}`, "Shivajinagar"},
			{"a house row never gives its name",
				`{"name":"Lakeview Apartments","type":"house","locality":"Koramangala","city":"Bengaluru"}`, "Koramangala"},
			{"only the city is known",
				`{"type":"street","name":"MG Road","city":"Bengaluru"}`, "Bengaluru"},
			{"a null district is skipped",
				`{"type":"street","district":"null","city":"Pune"}`, "Pune"},
			{"spaces inside the name are collapsed",
				`{"name":"  Old   Town ","type":"district"}`, "Old Town"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				photon := newReverseUpstream(t, 200, photonReverseBody(tc.props))
				nominatim := newReverseUpstream(t, 200, `{}`)
				got, err := reverseClient(photon, nominatim).ReverseArea(context.Background(), 12.9, 77.6)
				if err != nil {
					t.Fatalf("ReverseArea error = %v", err)
				}
				if got != tc.want {
					t.Fatalf("name = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("a Photon row with only a street, house and postcode gives no name", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, photonReverseBody(`{"type":"house",`+streetProps+`}`))
		nominatim := newReverseUpstream(t, 200, `{"address":{"road":"MG Road","house_number":"12","postcode":"560001"}}`)
		got, err := reverseClient(photon, nominatim).ReverseArea(context.Background(), 12.9, 77.6)
		if !errors.Is(err, ErrNoArea) {
			t.Fatalf("error = %v (name %q), want ErrNoArea", err, got)
		}
		if got != "" {
			t.Fatalf("name = %q, want empty", got)
		}
	})
}

func TestReverseAreaFallsBackToNominatim(t *testing.T) {
	cases := []struct {
		name        string
		photonCode  int
		photonBody  string
		nominatim   string
		want        string
		wantNoAreaE bool
	}{
		{"Photon found nothing", 200, emptyPhoton,
			`{"address":{"road":"100 Feet Road","house_number":"12","postcode":"560038","suburb":"Indiranagar","city":"Bengaluru"}}`,
			"Indiranagar", false},
		{"Photon answered an error status", 500, `{}`,
			`{"address":{"road":"Ring Road","town":"Hosur"}}`, "Hosur", false},
		{"the suburb comes before the neighbourhood", 200, emptyPhoton,
			`{"address":{"neighbourhood":"Block 4","suburb":"Jayanagar","city":"Bengaluru"}}`, "Jayanagar", false},
		{"a village is taken", 200, emptyPhoton,
			`{"address":{"village":"Kandi","state_district":"Sangareddy"}}`, "Kandi", false},
		{"only a road and a house number", 200, emptyPhoton,
			`{"address":{"road":"MG Road","house_number":"12","postcode":"560001"}}`, "", true},
		{"Nominatim's own error object", 200, emptyPhoton, `{"error":"Unable to geocode"}`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			photon := newReverseUpstream(t, tc.photonCode, tc.photonBody)
			nominatim := newReverseUpstream(t, 200, tc.nominatim)
			c := reverseClient(photon, nominatim)
			c.contactEmail = "owner@example.com"

			got, err := c.ReverseArea(context.Background(), 12.9784, 77.6408)
			if tc.wantNoAreaE {
				if !errors.Is(err, ErrNoArea) {
					t.Fatalf("error = %v (name %q), want ErrNoArea", err, got)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}

			req := nominatim.first(t)
			if req.URL.Path != "/reverse" {
				t.Fatalf("path = %q, want /reverse", req.URL.Path)
			}
			q := req.URL.Query()
			if q.Get("format") != "jsonv2" || q.Get("zoom") != "14" || q.Get("addressdetails") != "1" {
				t.Fatalf("query = %v, want format jsonv2, zoom 14, addressdetails 1", q)
			}
			if lat, _ := strconv.ParseFloat(q.Get("lat"), 64); math.Abs(lat-12.9784) > 1e-9 {
				t.Fatalf("lat = %q", q.Get("lat"))
			}
			if lon, _ := strconv.ParseFloat(q.Get("lon"), 64); math.Abs(lon-77.6408) > 1e-9 {
				t.Fatalf("lon = %q", q.Get("lon"))
			}
			if q.Get("email") != "owner@example.com" {
				t.Fatalf("email = %q, want the contact email", q.Get("email"))
			}
			if ua := req.Header.Get("User-Agent"); ua != c.userAgent() {
				t.Fatalf("User-Agent = %q, want %q", ua, c.userAgent())
			}
		})
	}
}

func TestCoarseNominatimName(t *testing.T) {
	cases := []struct {
		name string
		addr map[string]string
		want string
	}{
		{"suburb first", map[string]string{"suburb": "A", "neighbourhood": "B", "quarter": "C", "village": "D", "town": "E", "city": "F"}, "A"},
		{"neighbourhood second", map[string]string{"neighbourhood": "B", "quarter": "C", "village": "D", "town": "E", "city": "F"}, "B"},
		{"quarter third", map[string]string{"quarter": "C", "village": "D", "town": "E", "city": "F"}, "C"},
		{"village fourth", map[string]string{"village": "D", "town": "E", "city": "F"}, "D"},
		{"town fifth", map[string]string{"town": "E", "city": "F"}, "E"},
		{"city last", map[string]string{"city": "F", "road": "MG Road"}, "F"},
		{"road, house number and postcode are never taken", map[string]string{"road": "MG Road", "house_number": "12", "postcode": "560001"}, ""},
		{"a blank part is skipped", map[string]string{"suburb": "  ", "town": "Hosur"}, "Hosur"},
		{"nothing", map[string]string{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := coarseNominatimName(tc.addr); got != tc.want {
				t.Fatalf("coarseNominatimName = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReverseAreaCutsAndRefusesEmptyNames(t *testing.T) {
	t.Run("a long name is cut to 120 runes at a word end", func(t *testing.T) {
		long := strings.Repeat("Shivajinagar ", 12) + "end"
		photon := newReverseUpstream(t, 200, photonReverseBody(fmt.Sprintf(`{"type":"locality","name":%q}`, long)))
		nominatim := newReverseUpstream(t, 200, `{}`)
		got, err := reverseClient(photon, nominatim).ReverseArea(context.Background(), 12.9, 77.6)
		if err != nil {
			t.Fatalf("ReverseArea error = %v", err)
		}
		if n := utf8.RuneCountInString(got); n > ReverseAreaMaxRunes || n < ReverseAreaMaxRunes-len("Shivajinagar") {
			t.Fatalf("name has %d runes, want at most %d and a whole word", n, ReverseAreaMaxRunes)
		}
		if strings.HasSuffix(got, " ") || !strings.HasSuffix(got, "Shivajinagar") {
			t.Fatalf("name %q does not end at a word end", got)
		}
	})

	t.Run("a name without letters or digits is not a name", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, photonReverseBody(`{"type":"locality","name":" . - , "}`))
		nominatim := newReverseUpstream(t, 200, `{"address":{"suburb":" ... "}}`)
		got, err := reverseClient(photon, nominatim).ReverseArea(context.Background(), 12.9, 77.6)
		if !errors.Is(err, ErrNoArea) || got != "" {
			t.Fatalf("got %q, %v; want empty and ErrNoArea", got, err)
		}
	})
}

func TestReverseAreaNeverWaitsForAToken(t *testing.T) {
	const quick = 100 * time.Millisecond

	t.Run("both limiters empty returns busy at once without a request", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, emptyPhoton)
		nominatim := newReverseUpstream(t, 200, `{"address":{"suburb":"Indiranagar"}}`)
		c := reverseClient(photon, nominatim)
		drainLimiters(c, true, true)

		start := time.Now()
		got, err := c.ReverseArea(context.Background(), 12.9, 77.6)
		elapsed := time.Since(start)

		if !errors.Is(err, ErrReaderBusy) || got != "" {
			t.Fatalf("got %q, %v; want empty and ErrReaderBusy", got, err)
		}
		if elapsed > quick {
			t.Fatalf("took %v, want under %v: the lookup must not wait for a token", elapsed, quick)
		}
		if photon.hits() != 0 || nominatim.hits() != 0 {
			t.Fatalf("upstream hits photon=%d nominatim=%d, want none", photon.hits(), nominatim.hits())
		}
	})

	t.Run("no Photon token skips Photon and goes to Nominatim", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, photonReverseBody(`{"type":"locality","name":"Never Asked"}`))
		nominatim := newReverseUpstream(t, 200, `{"address":{"suburb":"Indiranagar"}}`)
		c := reverseClient(photon, nominatim)
		drainLimiters(c, true, false)

		start := time.Now()
		got, err := c.ReverseArea(context.Background(), 12.9, 77.6)
		if err != nil || got != "Indiranagar" {
			t.Fatalf("got %q, %v; want Indiranagar", got, err)
		}
		if time.Since(start) > quick*3 {
			t.Fatalf("took %v, want a quick answer", time.Since(start))
		}
		if photon.hits() != 0 {
			t.Fatalf("Photon was asked %d times without a token", photon.hits())
		}
	})

	t.Run("Photon found nothing and no Nominatim token is busy at once", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, emptyPhoton)
		nominatim := newReverseUpstream(t, 200, `{"address":{"suburb":"Indiranagar"}}`)
		c := reverseClient(photon, nominatim)
		drainLimiters(c, false, true)

		start := time.Now()
		_, err := c.ReverseArea(context.Background(), 12.9, 77.6)
		if !errors.Is(err, ErrReaderBusy) {
			t.Fatalf("error = %v, want ErrReaderBusy", err)
		}
		if time.Since(start) > quick*3 {
			t.Fatalf("took %v, want a quick answer", time.Since(start))
		}
		if nominatim.hits() != 0 {
			t.Fatalf("Nominatim was asked %d times without a token", nominatim.hits())
		}
	})

	t.Run("a lookup leaves the tokens a person's search would need as it found them", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, photonReverseBody(`{"type":"locality","name":"Central Market"}`))
		nominatim := newReverseUpstream(t, 200, `{}`)
		c := reverseClient(photon, nominatim)
		if _, err := c.ReverseArea(context.Background(), 12.9, 77.6); err != nil {
			t.Fatalf("ReverseArea error = %v", err)
		}
		// Photon answered, so the Nominatim token was never taken.
		if !c.limiter.Allow() {
			t.Fatal("the Nominatim token was spent although Photon answered")
		}
	})
}

func TestReverseAreaTimeCap(t *testing.T) {
	photon := newReverseUpstream(t, 200, emptyPhoton)
	nominatim := newReverseUpstream(t, 200, `{}`)
	photon.hang = true
	nominatim.hang = true
	c := reverseClient(photon, nominatim)

	start := time.Now()
	got, err := c.ReverseArea(context.Background(), 12.9, 77.6)
	elapsed := time.Since(start)

	if err == nil || got != "" {
		t.Fatalf("got %q, %v; want empty and an error", got, err)
	}
	if elapsed > 2500*time.Millisecond {
		t.Fatalf("took %v, want within about %v", elapsed, ReverseTimeout)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want a deadline error", err)
	}
	if ReverseTimeout != 2*time.Second {
		t.Fatalf("ReverseTimeout = %v, want 2s", ReverseTimeout)
	}
}

func TestReverseAreaScrubsCoordinates(t *testing.T) {
	const lat, lon = 12.345678, 77.654321
	latText, lonText := "12.345678", "77.654321"

	assertScrubbed := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("want an error")
		}
		text := err.Error()
		for _, digits := range []string{latText, lonText, "12.3456", "77.6543", url.QueryEscape(latText)} {
			if strings.Contains(text, digits) {
				t.Fatalf("error %q holds the coordinate text %q", text, digits)
			}
		}
	}

	t.Run("a refused connection", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, emptyPhoton)
		nominatim := newReverseUpstream(t, 200, `{}`)
		c := reverseClient(photon, nominatim)
		photon.server.Close()
		nominatim.server.Close()
		_, err := c.ReverseArea(context.Background(), lat, lon)
		assertScrubbed(t, err)
	})

	t.Run("an error status", func(t *testing.T) {
		photon := newReverseUpstream(t, 503, `{}`)
		nominatim := newReverseUpstream(t, 429, `{}`)
		_, err := reverseClient(photon, nominatim).ReverseArea(context.Background(), lat, lon)
		assertScrubbed(t, err)
		var status *UpstreamStatusError
		if !errors.As(err, &status) || status.Status != 429 {
			t.Fatalf("error = %v, want the Nominatim status error", err)
		}
	})

	t.Run("an unreadable answer", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, `not json`)
		nominatim := newReverseUpstream(t, 200, `not json either`)
		_, err := reverseClient(photon, nominatim).ReverseArea(context.Background(), lat, lon)
		assertScrubbed(t, err)
	})

	t.Run("a cancelled caller", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, emptyPhoton)
		nominatim := newReverseUpstream(t, 200, `{}`)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := reverseClient(photon, nominatim).ReverseArea(ctx, lat, lon)
		assertScrubbed(t, err)
	})
}

func TestReverseAreaRefusesBadCoordinates(t *testing.T) {
	cases := []struct {
		name     string
		lat, lon float64
	}{
		{"latitude above 90", 90.0001, 77},
		{"latitude below -90", -90.0001, 77},
		{"longitude above 180", 12, 180.0001},
		{"longitude below -180", 12, -180.0001},
		{"NaN latitude", math.NaN(), 77},
		{"NaN longitude", 12, math.NaN()},
		{"infinite latitude", math.Inf(1), 77},
		{"infinite longitude", 12, math.Inf(-1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			photon := newReverseUpstream(t, 200, emptyPhoton)
			nominatim := newReverseUpstream(t, 200, `{}`)
			got, err := reverseClient(photon, nominatim).ReverseArea(context.Background(), tc.lat, tc.lon)
			if !errors.Is(err, errBadReverseCoordinates) || got != "" {
				t.Fatalf("got %q, %v; want empty and the bad coordinates error", got, err)
			}
			if photon.hits() != 0 || nominatim.hits() != 0 {
				t.Fatalf("a request was made for refused coordinates (photon %d, nominatim %d)", photon.hits(), nominatim.hits())
			}
		})
	}

	t.Run("the edges are allowed", func(t *testing.T) {
		photon := newReverseUpstream(t, 200, photonReverseBody(`{"type":"city","name":"Edge"}`))
		nominatim := newReverseUpstream(t, 200, `{}`)
		got, err := reverseClient(photon, nominatim).ReverseArea(context.Background(), -90, 180)
		if err != nil || got != "Edge" {
			t.Fatalf("got %q, %v; want Edge", got, err)
		}
	})
}

func TestReverseAreaHasNoSecondClientOrLimiter(t *testing.T) {
	// The constants are the only hosts the real client ever reverse looks up on
	// (T-07-01), and the real client starts with them set.
	c := NewClient("")
	if c.photonReverse != photonReverseURL || c.nominatimReverse != nominatimReverseURL {
		t.Fatalf("reverse URLs = %q, %q; want the constants", c.photonReverse, c.nominatimReverse)
	}
	if photonReverseURL != "https://photon.komoot.io/reverse" || nominatimReverseURL != "https://nominatim.openstreetmap.org/reverse" {
		t.Fatalf("reverse constants changed: %q, %q", photonReverseURL, nominatimReverseURL)
	}
	// A Nominatim only client keeps Photon off for reverse as well.
	if off := newClientWithBaseURL("http://127.0.0.1:1", "", time.Second, 1); off.photonReverse != "" {
		t.Fatalf("a client with Photon off has photonReverse %q", off.photonReverse)
	}
}
