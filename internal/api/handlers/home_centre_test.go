package handlers

// home_centre_test.go pins plan 03.4-33 task 2: when the device location is not
// available, Nearby and the Low data view start at the account's home area cell
// centre, and at the configured fallback centre when the account has none. The
// page carries only the coarse cell centre, never a finer position.

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/account"
)

// fakeHomeCentre answers HomeAreaCentre from fixed values and remembers which
// account it was asked about.
type fakeHomeCentre struct {
	lat, lon float64
	ok       bool
	err      error
	asked    []int64
}

func (f *fakeHomeCentre) HomeAreaCentre(_ context.Context, accountID int64) (float64, float64, bool, error) {
	f.asked = append(f.asked, accountID)
	return f.lat, f.lon, f.ok, f.err
}

// The configured fallback of these tests is Bengaluru; the home area is a cell
// near Vijayawada, so the two cannot be confused.
const (
	hcFallbackLat = 12.9716
	hcFallbackLon = 77.5946
)

func hcHomeCentre() (float64, float64) {
	return geohash.DecodeCenter(geohash.EncodeWithPrecision(16.5115, 80.6160, 5))
}

func hcAttr(t *testing.T, body, name string) (string, bool) {
	t.Helper()
	m := regexp.MustCompile(name + `="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func hcNumber(t *testing.T, body, name string) float64 {
	t.Helper()
	raw, ok := hcAttr(t, body, name)
	if !ok {
		t.Fatalf("no %s attribute in the page", name)
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		t.Fatalf("%s = %q is not a number: %v", name, raw, err)
	}
	return v
}

func hcRender(t *testing.T, low bool, lookup HomeCentreLookup) (int, string) {
	t.Helper()
	tmpl, err := ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}
	cfg := PageConfig{FallbackLat: hcFallbackLat, FallbackLon: hcFallbackLon, DefaultRadiusKm: 10, HomeCentre: lookup}
	handler := Page(tmpl, cfg)
	target := "/"
	if low {
		handler = LowPage(tmpl, cfg)
		target = LowDataPath
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req = req.WithContext(account.WithAccount(req.Context(), account.Account{ID: 77, Email: "home@example.com"}))
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec.Code, rec.Body.String()
}

func hcNear(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestPagesStartAtTheHomeAreaCentre: an account with a home area gets the cell
// centre as the fallback centre on Nearby and on the Low data view.
func TestPagesStartAtTheHomeAreaCentre(t *testing.T) {
	wantLat, wantLon := hcHomeCentre()
	for _, low := range []bool{false, true} {
		name := "nearby"
		if low {
			name = "low"
		}
		t.Run(name, func(t *testing.T) {
			lookup := &fakeHomeCentre{lat: wantLat, lon: wantLon, ok: true}
			code, body := hcRender(t, low, lookup)
			if code != http.StatusOK {
				t.Fatalf("status %d, want 200", code)
			}
			if got := hcNumber(t, body, "data-fallback-lat"); !hcNear(got, wantLat) {
				t.Errorf("data-fallback-lat = %v, want the home cell centre %v", got, wantLat)
			}
			if got := hcNumber(t, body, "data-fallback-lon"); !hcNear(got, wantLon) {
				t.Errorf("data-fallback-lon = %v, want the home cell centre %v", got, wantLon)
			}
			if len(lookup.asked) != 1 || lookup.asked[0] != 77 {
				t.Errorf("the lookup was asked about %v, want the gate's account 77 once", lookup.asked)
			}
		})
	}
}

// TestNearbyHomeAreaCentreStartsCloserThanTheCity: the map zoom for a home area
// centre is 13 (about 7 km across a phone, the zoom of a device fix) and the
// city fallback keeps the page's default, so the attribute is absent then. The
// Low data view has no map and so no zoom attribute.
func TestNearbyHomeAreaCentreStartsCloserThanTheCity(t *testing.T) {
	lat, lon := hcHomeCentre()
	_, body := hcRender(t, false, &fakeHomeCentre{lat: lat, lon: lon, ok: true})
	if got, ok := hcAttr(t, body, "data-fallback-zoom"); !ok || got != "13" {
		t.Errorf("data-fallback-zoom = %q (present %v), want 13 for a home area centre", got, ok)
	}
	_, none := hcRender(t, false, &fakeHomeCentre{ok: false})
	if _, ok := hcAttr(t, none, "data-fallback-zoom"); ok {
		t.Errorf("an account with no home area must not carry data-fallback-zoom: the page keeps its city zoom")
	}
	_, low := hcRender(t, true, &fakeHomeCentre{lat: lat, lon: lon, ok: true})
	if strings.Contains(low, "data-fallback-zoom") {
		t.Errorf("the Low data view has no map and must not carry data-fallback-zoom")
	}
}

// TestPagesKeepTheConfiguredFallbackWithoutAHomeArea: no home area, no lookup,
// or a lookup that fails all leave the configured centre. A failing lookup is
// logged and never turns the page into an error.
func TestPagesKeepTheConfiguredFallbackWithoutAHomeArea(t *testing.T) {
	cases := map[string]HomeCentreLookup{
		"no home area":   &fakeHomeCentre{ok: false},
		"no lookup":      nil,
		"lookup failure": &fakeHomeCentre{ok: true, lat: 1, lon: 2, err: errors.New("database is down")},
	}
	for name, lookup := range cases {
		for _, low := range []bool{false, true} {
			code, body := hcRender(t, low, lookup)
			if code != http.StatusOK {
				t.Errorf("%s (low %v): status %d, want 200", name, low, code)
				continue
			}
			if got := hcNumber(t, body, "data-fallback-lat"); !hcNear(got, hcFallbackLat) {
				t.Errorf("%s (low %v): data-fallback-lat = %v, want the configured %v", name, low, got, hcFallbackLat)
			}
			if got := hcNumber(t, body, "data-fallback-lon"); !hcNear(got, hcFallbackLon) {
				t.Errorf("%s (low %v): data-fallback-lon = %v, want the configured %v", name, low, got, hcFallbackLon)
			}
			if _, ok := hcAttr(t, body, "data-fallback-zoom"); ok {
				t.Errorf("%s (low %v): no zoom attribute without a usable home area", name, low)
			}
		}
	}
}

// TestHomeAreaCentreIgnoresNumbersWhenNone: a lookup that says "none" has its
// numbers ignored, whatever they are.
func TestHomeAreaCentreIgnoresNumbersWhenNone(t *testing.T) {
	_, body := hcRender(t, false, &fakeHomeCentre{ok: false, lat: 40, lon: 40})
	if got := hcNumber(t, body, "data-fallback-lat"); !hcNear(got, hcFallbackLat) {
		t.Errorf("data-fallback-lat = %v: numbers beside ok=false must be ignored", got)
	}
}
