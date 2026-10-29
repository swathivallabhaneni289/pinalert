// coarsen_test.go pins the shared coarsening of a finished Rescue report (plan
// 03.1-01, D-23, D-28). It is package service because it reaches haversineKm.
// Every helper starts with p31 so it cannot collide with another plan's name.
// No database is needed.

package service

import (
	"math"
	"testing"

	"github.com/mmcloughlin/geohash"
)

func TestCoarsePoint(t *testing.T) {
	t.Run("centre of the precision 6 cell", func(t *testing.T) {
		lat, lon, cell := CoarsePoint(13.0827, 80.2707)
		if len(cell) != ApproxGeohashPrecision {
			t.Fatalf("cell %q has %d characters, want %d", cell, len(cell), ApproxGeohashPrecision)
		}
		if want := geohash.EncodeWithPrecision(13.0827, 80.2707, 6); cell != want {
			t.Errorf("cell = %q, want %q", cell, want)
		}
		cLat, cLon := geohash.DecodeCenter(cell)
		if lat != cLat || lon != cLon {
			t.Errorf("point = (%v, %v), want the cell centre (%v, %v)", lat, lon, cLat, cLon)
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		for lat := 8.0; lat <= 35.0; lat += 1.37 {
			for lon := 68.0; lon <= 97.5; lon += 1.91 {
				la, lo, cell := CoarsePoint(lat, lon)
				la2, lo2, cell2 := CoarsePoint(la, lo)
				if la != la2 || lo != lo2 || cell != cell2 {
					t.Errorf("(%v, %v) coarsened to (%v, %v, %q) and again to (%v, %v, %q)", lat, lon, la, lo, cell, la2, lo2, cell2)
				}
			}
		}
	})

	t.Run("two points in one cell coarsen alike", func(t *testing.T) {
		la1, lo1, c1 := CoarsePoint(13.08270, 80.27070)
		la2, lo2, c2 := CoarsePoint(13.08275, 80.27075)
		if c1 != c2 || la1 != la2 || lo1 != lo2 {
			t.Errorf("points about 7 m apart gave (%v, %v, %q) and (%v, %v, %q)", la1, lo1, c1, la2, lo2, c2)
		}
	})

	t.Run("cell size", func(t *testing.T) {
		// Precision 6 is 30 bits, 15 for longitude and 15 for latitude, so a
		// cell is 360/2^15 degrees wide and 180/2^15 degrees tall. That is
		// about 1.22 km wide at the equator and 0.61 km tall, the "about 1 km"
		// of D-23. The most a point can sit from its cell centre is half the
		// diagonal, about 0.68 km, below the 0.8 km the plan promises.
		const metresPerDegree = 6371000.0 * math.Pi / 180
		wDeg := 360.0 / math.Pow(2, 15)
		hDeg := 180.0 / math.Pow(2, 15)
		widthKm := wDeg * metresPerDegree / 1000
		heightKm := hDeg * metresPerDegree / 1000
		if widthKm < 1.1 || widthKm > 1.3 {
			t.Errorf("equator cell width = %.3f km, want about 1.22", widthKm)
		}
		if heightKm < 0.55 || heightKm > 0.67 {
			t.Errorf("cell height = %.3f km, want about 0.61", heightKm)
		}
		if worst := math.Hypot(widthKm, heightKm) / 2; worst >= 0.8 {
			t.Errorf("worst case distance to the centre = %.3f km, want below 0.8", worst)
		}
		// The library agrees with the derivation.
		box := geohash.BoundingBox(geohash.EncodeWithPrecision(20, 78, 6))
		if got := box.MaxLng - box.MinLng; math.Abs(got-wDeg) > 1e-9 {
			t.Errorf("library cell width = %v degrees, want %v", got, wDeg)
		}
		if got := box.MaxLat - box.MinLat; math.Abs(got-hDeg) > 1e-9 {
			t.Errorf("library cell height = %v degrees, want %v", got, hDeg)
		}
	})
}

func p31Rescue() Report {
	d := 1.234
	return Report{
		ID:          7,
		Category:    CategoryRescueNeeded,
		Severity:    SeverityCritical,
		Description: "Family on the roof, water rising",
		Latitude:    13.082712,
		Longitude:   80.270741,
		Geohash:     geohash.EncodeWithPrecision(13.082712, 80.270741, 8),
		DistanceKm:  &d,
	}
}

func TestCoarsenFinishedRescue(t *testing.T) {
	centre := &GeoPoint{Lat: 13.10, Lon: 80.30}

	t.Run("finished rescue changes all four fields", func(t *testing.T) {
		in := p31Rescue()
		got, ok := CoarsenFinishedRescue(in, true, centre)
		if !ok {
			t.Fatal("approximate = false, want true")
		}
		wantLat, wantLon, wantCell := CoarsePoint(in.Latitude, in.Longitude)
		if got.Latitude != wantLat || got.Longitude != wantLon {
			t.Errorf("point = (%v, %v), want (%v, %v)", got.Latitude, got.Longitude, wantLat, wantLon)
		}
		if got.Geohash != wantCell || len(got.Geohash) != 6 {
			t.Errorf("geohash = %q, want %q", got.Geohash, wantCell)
		}
		if got.Description != "" {
			t.Errorf("description = %q, want empty", got.Description)
		}
		if got.DistanceKm == nil {
			t.Fatal("distance is nil, want a coarse distance")
		}
		wantDist := haversineKm(centre.Lat, centre.Lon, wantLat, wantLon)
		if math.Abs(*got.DistanceKm-wantDist) > 1e-9 {
			t.Errorf("distance = %v, want %v", *got.DistanceKm, wantDist)
		}
		if got.ID != in.ID || got.Category != in.Category || got.Severity != in.Severity {
			t.Errorf("identity fields changed: %+v", got)
		}
		if !got.CreatedAt.Equal(in.CreatedAt) || !got.ExpiresAt.Equal(in.ExpiresAt) {
			t.Errorf("times changed: %+v", got)
		}
		// The input is a value copy and must be untouched.
		if in.Description == "" || in.Geohash == got.Geohash || *in.DistanceKm != 1.234 {
			t.Errorf("input was modified: %+v", in)
		}
	})

	t.Run("no exact distance survives without a centre", func(t *testing.T) {
		got, ok := CoarsenFinishedRescue(p31Rescue(), true, nil)
		if !ok {
			t.Fatal("approximate = false, want true")
		}
		if got.DistanceKm != nil {
			t.Errorf("distance = %v, want nil", *got.DistanceKm)
		}
	})

	t.Run("not finished is unchanged", func(t *testing.T) {
		in := p31Rescue()
		got, ok := CoarsenFinishedRescue(in, false, centre)
		if ok {
			t.Error("approximate = true for a live rescue report")
		}
		if got.Latitude != in.Latitude || got.Longitude != in.Longitude || got.Geohash != in.Geohash ||
			got.Description != in.Description || got.DistanceKm != in.DistanceKm {
			t.Errorf("a live rescue report changed: %+v", got)
		}
	})

	t.Run("other categories are unchanged", func(t *testing.T) {
		for _, cat := range []Category{CategoryFlood, CategoryShelterOpen, CategoryPowerOutage} {
			in := p31Rescue()
			in.Category = cat
			got, ok := CoarsenFinishedRescue(in, true, centre)
			if ok {
				t.Errorf("%s: approximate = true", cat)
			}
			if got.Latitude != in.Latitude || got.Longitude != in.Longitude || got.Geohash != in.Geohash ||
				got.Description != in.Description || got.DistanceKm != in.DistanceKm {
				t.Errorf("%s: report changed: %+v", cat, got)
			}
		}
	})
}

func TestCoarsenBoundAcrossIndia(t *testing.T) {
	worst := 0.0
	for lat := 8.0; lat <= 35.0; lat += 0.173 {
		for lon := 68.0; lon <= 97.5; lon += 0.211 {
			la, lo, _ := CoarsePoint(lat, lon)
			if d := haversineKm(lat, lon, la, lo); d > worst {
				worst = d
			}
		}
	}
	if worst >= 0.8 {
		t.Errorf("worst coarse error over India = %.3f km, want below 0.8", worst)
	}
	t.Logf("worst coarse error over India = %.3f km", worst)
}

func TestCoarsePointAt(t *testing.T) {
	t.Run("precision 5 cell and centre", func(t *testing.T) {
		lat, lon, cell := CoarsePointAt(13.0827, 80.2707, 5)
		if want := geohash.EncodeWithPrecision(13.0827, 80.2707, 5); cell != want || len(cell) != 5 {
			t.Fatalf("cell = %q, want %q", cell, want)
		}
		cLat, cLon := geohash.DecodeCenter(cell)
		if lat != cLat || lon != cLon {
			t.Errorf("point = (%v, %v), want the cell centre (%v, %v)", lat, lon, cLat, cLon)
		}
	})

	t.Run("CoarsePoint is precision 6", func(t *testing.T) {
		la, lo, c := CoarsePoint(13.0827, 80.2707)
		la2, lo2, c2 := CoarsePointAt(13.0827, 80.2707, 6)
		if la != la2 || lo != lo2 || c != c2 {
			t.Errorf("CoarsePoint = (%v, %v, %q), CoarsePointAt 6 = (%v, %v, %q)", la, lo, c, la2, lo2, c2)
		}
	})

	t.Run("precision 5 cell is about 5 km", func(t *testing.T) {
		box := geohash.BoundingBox(geohash.EncodeWithPrecision(20, 78, 5))
		if got := box.MaxLng - box.MinLng; math.Abs(got-360.0/8192) > 1e-9 {
			t.Errorf("cell width = %v degrees, want %v", got, 360.0/8192)
		}
		if got := box.MaxLat - box.MinLat; math.Abs(got-180.0/4096) > 1e-9 {
			t.Errorf("cell height = %v degrees, want %v", got, 180.0/4096)
		}
	})
}

func TestCoarsenFinishedForProfile(t *testing.T) {
	centre := &GeoPoint{Lat: 13.10, Lon: 80.30}

	t.Run("flood is blurred to precision 5 with no text", func(t *testing.T) {
		in := p31Rescue()
		in.Category = CategoryFlood
		got := CoarsenFinishedForProfile(in, centre)
		wantLat, wantLon, wantCell := CoarsePointAt(in.Latitude, in.Longitude, 5)
		if got.Latitude != wantLat || got.Longitude != wantLon || got.Geohash != wantCell || len(got.Geohash) != 5 {
			t.Errorf("point = (%v, %v, %q), want (%v, %v, %q)", got.Latitude, got.Longitude, got.Geohash, wantLat, wantLon, wantCell)
		}
		if got.Description != "" {
			t.Errorf("description = %q, want empty", got.Description)
		}
		if got.DistanceKm == nil {
			t.Fatal("distance is nil, want a coarse distance")
		}
		if want := haversineKm(centre.Lat, centre.Lon, wantLat, wantLon); math.Abs(*got.DistanceKm-want) > 1e-9 {
			t.Errorf("distance = %v, want %v", *got.DistanceKm, want)
		}
		if in.Description == "" || *in.DistanceKm != 1.234 {
			t.Errorf("input was modified: %+v", in)
		}
	})

	t.Run("rescue keeps precision 6 and no text", func(t *testing.T) {
		in := p31Rescue()
		got := CoarsenFinishedForProfile(in, centre)
		_, _, wantCell := CoarsePointAt(in.Latitude, in.Longitude, 6)
		if got.Geohash != wantCell || len(got.Geohash) != 6 {
			t.Errorf("geohash = %q, want %q", got.Geohash, wantCell)
		}
		if got.Description != "" {
			t.Errorf("description = %q, want empty", got.Description)
		}
	})

	t.Run("no centre means no distance", func(t *testing.T) {
		got := CoarsenFinishedForProfile(p31Rescue(), nil)
		if got.DistanceKm != nil {
			t.Errorf("distance = %v, want nil", *got.DistanceKm)
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		for _, cat := range []Category{CategoryFlood, CategoryRescueNeeded, CategoryShelterOpen} {
			in := p31Rescue()
			in.Category = cat
			once := CoarsenFinishedForProfile(in, centre)
			twice := CoarsenFinishedForProfile(once, centre)
			if once.Latitude != twice.Latitude || once.Longitude != twice.Longitude || once.Geohash != twice.Geohash ||
				once.Description != twice.Description || *once.DistanceKm != *twice.DistanceKm {
				t.Errorf("%s: not idempotent: %+v then %+v", cat, once, twice)
			}
		}
	})
}
