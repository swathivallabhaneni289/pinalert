// rescue_privacy_e2e_test.go proves D-23 at the wire: a finished Rescue report
// never leaves GET /api/reports with its description, its exact coordinates,
// its precision 8 geohash or its exact distance, on the past view and on a
// report link, from different centres. The assertions run on the raw response
// bytes as well as the decoded struct, so a field the struct does not name
// cannot leak unnoticed. A live Rescue report and finished Flood, Shelter and
// Power reports stay exact.
package handlers_test

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/service"
)

// The distinctive values a leak would reveal. The coordinates have seven
// decimals, so their exact text cannot appear by accident in a coarse point.
const (
	h10RescueDescription = "Grandmother stranded on the roof of the green house behind Gokul temple"
	h10RescueLat         = 12.9716123
	h10RescueLon         = 77.5946457
)

func h10RescueBody() map[string]any {
	return map[string]any{
		"category":    "rescue_needed",
		"severity":    "critical",
		"description": h10RescueDescription,
		"latitude":    h10RescueLat,
		"longitude":   h10RescueLon,
	}
}

// h10AssertBlurred fails unless the report is the blurred form of the Rescue
// fixture as seen from the centre (cLat, cLon), and the raw body carries none
// of the exact values.
func h10AssertBlurred(t *testing.T, label string, r *h10Report, raw []byte, cLat, cLon float64) {
	t.Helper()
	if r == nil {
		t.Fatalf("%s: the finished Rescue report is missing", label)
	}
	body := string(raw)
	exactGeohash := geohash.EncodeWithPrecision(h10RescueLat, h10RescueLon, 8)
	for _, leak := range []string{
		h10RescueDescription,
		"Gokul",
		"12.9716123",
		"77.5946457",
		exactGeohash,
	} {
		if strings.Contains(body, leak) {
			t.Errorf("%s: the raw response contains %q: %s", label, leak, body)
		}
	}
	if r.Description != "" {
		t.Errorf("%s: description = %q, want empty", label, r.Description)
	}
	if !r.Approximate {
		t.Errorf("%s: approximate = false, want true", label)
	}
	if len(r.Geohash) != 6 {
		t.Errorf("%s: geohash %q has %d characters, want 6", label, r.Geohash, len(r.Geohash))
	}
	if r.Geohash == exactGeohash || !strings.HasPrefix(exactGeohash, r.Geohash) {
		t.Errorf("%s: geohash %q is not the 6 character cell of the exact point %q", label, r.Geohash, exactGeohash)
	}
	cLatWant, cLonWant, _ := service.CoarsePoint(h10RescueLat, h10RescueLon)
	if r.Latitude != cLatWant || r.Longitude != cLonWant {
		t.Errorf("%s: point = (%v, %v), want the coarse point (%v, %v)", label, r.Latitude, r.Longitude, cLatWant, cLonWant)
	}
	if r.Latitude == h10RescueLat || r.Longitude == h10RescueLon {
		t.Errorf("%s: a coordinate is still exact: (%v, %v)", label, r.Latitude, r.Longitude)
	}
	if r.DistanceKm == nil {
		t.Fatalf("%s: distance_km is null", label)
	}
	want := h10Haversine(cLat, cLon, r.Latitude, r.Longitude)
	if math.Abs(*r.DistanceKm-want) > 0.001 {
		t.Errorf("%s: distance_km = %v, want %v (the distance to the returned point)", label, *r.DistanceKm, want)
	}
	exactDistance := h10Haversine(cLat, cLon, h10RescueLat, h10RescueLon)
	if math.Abs(*r.DistanceKm-exactDistance) < 0.0000001 {
		t.Errorf("%s: distance_km %v equals the exact distance", label, *r.DistanceKm)
	}
}

// TestFinishedRescueNeverLeaksOnPastOrLink: live it is exact, finished (by the
// poster's resolve, and by running out its time) it is blurred on the past view
// and on a report link, from two different centres and for two different
// viewers.
func TestFinishedRescueNeverLeaksOnPastOrLink(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	poster := newVerifiedClient(t, srv, mailer, "h10-rescue-poster@example.com")
	reader := newVerifiedClient(t, srv, mailer, "h10-rescue-reader@example.com")

	id := storeReportAs(t, pool, poster, srv.URL, h10RescueBody())
	idStr := strconv.FormatInt(id, 10)

	// Live: exact on the default view and on a link.
	live, liveRaw := h10Get(t, poster, h10URL(srv.URL, h10Lat, h10Lon, ""))
	got := h10Find(live.Reports, id)
	if got == nil {
		t.Fatalf("live rescue report %d missing from the default view: %s", id, liveRaw)
	}
	if got.Description != h10RescueDescription || got.Latitude != h10RescueLat || got.Longitude != h10RescueLon {
		t.Errorf("live rescue is not exact: %+v", got)
	}
	if want := geohash.EncodeWithPrecision(h10RescueLat, h10RescueLon, 8); got.Geohash != want {
		t.Errorf("live geohash = %q, want %q", got.Geohash, want)
	}
	if got.Approximate {
		t.Errorf("live rescue is marked approximate")
	}
	liveLink, _ := h10Get(t, reader, h10URL(srv.URL, h10Lat, h10Lon, "report="+idStr))
	if liveLink.Focus == nil || liveLink.Focus.Report == nil || liveLink.Focus.Report.Description != h10RescueDescription {
		t.Errorf("live rescue on a link is not exact: %+v", liveLink.Focus)
	}

	// Finish it.
	h10Resolve(t, poster, srv.URL, id, h10RescueLat, h10RescueLon)

	centres := []struct {
		name     string
		client   *http.Client
		lat, lon float64
	}{
		{"poster at the spot", poster, h10Lat, h10Lon},
		{"reader across town", reader, 12.9800, 77.6000},
	}
	for _, c := range centres {
		past, raw := h10Get(t, c.client, h10URL(srv.URL, c.lat, c.lon, "view=past"))
		h10AssertBlurred(t, c.name+" past view", h10Find(past.Reports, id), raw, c.lat, c.lon)

		link, raw := h10Get(t, c.client, h10URL(srv.URL, c.lat, c.lon, "report="+idStr))
		if link.Focus == nil || link.Focus.Status != "ok" {
			t.Fatalf("%s: focus = %+v, want ok", c.name, link.Focus)
		}
		h10AssertBlurred(t, c.name+" link", link.Focus.Report, raw, c.lat, c.lon)

		// The link is also blurred when the past view is the tab being read.
		both, raw := h10Get(t, c.client, h10URL(srv.URL, c.lat, c.lon, "view=past&report="+idStr))
		h10AssertBlurred(t, c.name+" past view with link", both.Focus.Report, raw, c.lat, c.lon)
	}

	// Two centres give two different distances, both to the coarse point.
	a, _ := h10Get(t, poster, h10URL(srv.URL, h10Lat, h10Lon, "view=past"))
	b, _ := h10Get(t, reader, h10URL(srv.URL, 12.9800, 77.6000, "view=past"))
	ra, rb := h10Find(a.Reports, id), h10Find(b.Reports, id)
	if ra == nil || rb == nil || ra.DistanceKm == nil || rb.DistanceKm == nil {
		t.Fatalf("both centres should list the report: %+v %+v", ra, rb)
	}
	if *ra.DistanceKm == *rb.DistanceKm {
		t.Errorf("both centres report the same distance %v", *ra.DistanceKm)
	}

	// A Rescue report that ran out its time is blurred the same way.
	expiredID := storeReportAs(t, pool, poster, srv.URL, h10RescueBody())
	h10MovePast(t, pool, expiredID, 3*3600, 3600)
	past, raw := h10Get(t, reader, h10URL(srv.URL, 12.9800, 77.6000, "view=past"))
	h10AssertBlurred(t, "expired rescue past view", h10Find(past.Reports, expiredID), raw, 12.9800, 77.6000)
	link, raw := h10Get(t, reader, h10URL(srv.URL, 12.9800, 77.6000, "report="+strconv.FormatInt(expiredID, 10)))
	if link.Focus == nil || link.Focus.Status != "ok" {
		t.Fatalf("expired rescue focus = %+v, want ok", link.Focus)
	}
	h10AssertBlurred(t, "expired rescue link", link.Focus.Report, raw, 12.9800, 77.6000)
}

// TestLiveRescueAndFinishedFloodStayExact: only a finished Rescue report is
// blurred. A live Rescue report and finished Flood, Shelter and Power reports
// keep their text and exact coordinates (D-23).
func TestLiveRescueAndFinishedFloodStayExact(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "h10-exact@example.com")

	const lat, lon = 12.9716123, 77.5946457
	bodies := map[string]map[string]any{
		"flood": {
			"category": "flood", "severity": "low",
			"description": "Water over the knee on the service road",
			"latitude":    lat, "longitude": lon,
		},
		"shelter_open": {
			"category": "shelter_open", "severity": "low",
			"description": "School hall open with beds for families",
			"latitude":    lat, "longitude": lon,
			"shelter_capacity_status": "available",
		},
		"power_outage": {
			"category": "power_outage", "severity": "low",
			"description": "Transformer down, whole lane without power",
			"latitude":    lat, "longitude": lon,
		},
	}
	ids := map[string]int64{}
	for name, body := range bodies {
		id := h10Post(t, client, srv.URL, body)
		h10Resolve(t, client, srv.URL, id, lat, lon)
		ids[name] = id
	}
	liveRescue := storeReportAs(t, pool, client, srv.URL, h10RescueBody())

	past, raw := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, "view=past"))
	for name, id := range ids {
		r := h10Find(past.Reports, id)
		if r == nil {
			t.Errorf("%s: finished report %d is missing from the past view", name, id)
			continue
		}
		want := bodies[name]["description"].(string)
		if r.Description != want {
			t.Errorf("%s: description = %q, want %q", name, r.Description, want)
		}
		if r.Latitude != lat || r.Longitude != lon {
			t.Errorf("%s: point = (%v, %v), want exact (%v, %v)", name, r.Latitude, r.Longitude, lat, lon)
		}
		if wantHash := geohash.EncodeWithPrecision(lat, lon, 8); r.Geohash != wantHash {
			t.Errorf("%s: geohash = %q, want %q", name, r.Geohash, wantHash)
		}
		if r.Approximate {
			t.Errorf("%s: approximate = true on a finished %s report", name, name)
		}
		if r.DistanceKm == nil || math.Abs(*r.DistanceKm-h10Haversine(h10Lat, h10Lon, lat, lon)) > 0.001 {
			t.Errorf("%s: distance_km = %v, want the exact distance", name, r.DistanceKm)
		}
	}
	if h10Find(past.Reports, liveRescue) != nil {
		t.Errorf("the live rescue report %d is listed in the past view: %s", liveRescue, raw)
	}

	active, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, ""))
	r := h10Find(active.Reports, liveRescue)
	if r == nil {
		t.Fatalf("live rescue report %d missing from the default view", liveRescue)
	}
	if r.Description != h10RescueDescription || r.Latitude != h10RescueLat || r.Longitude != h10RescueLon || r.Approximate {
		t.Errorf("live rescue is not exact: %+v", r)
	}
}
