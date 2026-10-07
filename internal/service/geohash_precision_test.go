// geohash_precision_test.go is the evidence behind the "What counts as a
// place" section of docs/TRUST-MODEL.md. It closes 02-RESEARCH assumption
// A3 per D-03: geohash precision 7 is a tested, documented choice and not
// an interim value. Places are counted with independentCellCount, the same
// function BuildVoteTally uses for ConfirmCells (D-04, one counting rule).
// A change to voterGeohashPrecision must update this test and that
// document together.
//
// The file is package service (the internal test package, like
// trust_test.go) because voterGeohashPrecision and independentCellCount are
// unexported. It works in metres with its own constants and references no
// identifier from geo.go, so it compiles on its own.

package service

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/mmcloughlin/geohash"
)

// witnessMetresPerDegree is metres per degree of latitude on a sphere of
// radius 6371 km, the radius the feed distance and the 1 km rule use.
const witnessMetresPerDegree = 6371000.0 * math.Pi / 180

// witnessRadiusMetres is the upper incident radius (PITFALLS Pitfall 10:
// a real incident is 100 to 300 m across).
const witnessRadiusMetres = 300.0

const (
	witnessTrials       = 500
	witnessCeilingSteps = 120
)

// witnessCity is one Indian latitude the precision is checked at.
// widthMetres is the expected precision 7 cell width and ceiling7 the most
// precision 7 cells one 300 m circle can touch there.
type witnessCity struct {
	name        string
	lat, lon    float64
	widthMetres float64
	ceiling7    int
}

var witnessCities = []witnessCity{
	{"Chennai", 13.0827, 80.2707, 148.7, 23},
	{"Bengaluru", 12.9716, 77.5946, 148.8, 23},
	{"Kolkata", 22.5726, 88.3639, 141.0, 26},
	{"Delhi", 28.6139, 77.2090, 134.1, 26},
	{"Srinagar", 34.0837, 74.7973, 126.5, 26},
}

// witnessCellSize returns the height and width in metres of the geohash
// cell holding (lat, lon) at the given precision.
func witnessCellSize(lat, lon float64, precision uint) (heightMetres, widthMetres float64) {
	box := geohash.BoundingBox(geohash.EncodeWithPrecision(lat, lon, precision))
	heightMetres = (box.MaxLat - box.MinLat) * witnessMetresPerDegree
	widthMetres = (box.MaxLng - box.MinLng) * witnessMetresPerDegree * math.Cos(lat*math.Pi/180)
	return heightMetres, widthMetres
}

// witnessCeiling is the most cells of a regular grid (heightMetres by
// widthMetres) that one disc of radiusMetres can touch, found by sweeping
// the disc centre across one cell in steps by steps.
func witnessCeiling(heightMetres, widthMetres, radiusMetres float64, steps int) int {
	reachX := int(radiusMetres/widthMetres) + 2
	reachY := int(radiusMetres/heightMetres) + 2
	best := 0
	for i := 0; i <= steps; i++ {
		cx := float64(i) / float64(steps) * widthMetres
		for j := 0; j <= steps; j++ {
			cy := float64(j) / float64(steps) * heightMetres
			count := 0
			for col := -reachX; col <= reachX; col++ {
				x0 := float64(col) * widthMetres
				nx := math.Min(math.Max(cx, x0), x0+widthMetres)
				for row := -reachY; row <= reachY; row++ {
					y0 := float64(row) * heightMetres
					ny := math.Min(math.Max(cy, y0), y0+heightMetres)
					if math.Hypot(nx-cx, ny-cy) < radiusMetres {
						count++
					}
				}
			}
			best = max(best, count)
		}
	}
	return best
}

type witnessStats struct {
	minPlaces, maxPlaces, onePlaceTrials int
	meanPlaces                           float64
}

// simulateWitnesses scatters crowd witnesses uniformly inside a 300 m
// circle, witnessTrials times, and counts the places they fall in with
// independentCellCount. The circle centre is shifted by up to one cell in
// each axis so every alignment with the cell grid is sampled.
func simulateWitnesses(rng *rand.Rand, city witnessCity, precision uint, crowd int) witnessStats {
	cellH, cellW := witnessCellSize(city.lat, city.lon, precision)
	cosCity := math.Cos(city.lat * math.Pi / 180)
	stats := witnessStats{minPlaces: math.MaxInt}
	total := 0
	for trial := 0; trial < witnessTrials; trial++ {
		north := (2*rng.Float64() - 1) * cellH
		east := (2*rng.Float64() - 1) * cellW
		clat := city.lat + north/witnessMetresPerDegree
		clon := city.lon + east/(witnessMetresPerDegree*cosCity)
		cosCentre := math.Cos(clat * math.Pi / 180)
		cells := make([]string, 0, crowd)
		for w := 0; w < crowd; w++ {
			dist := witnessRadiusMetres * math.Sqrt(rng.Float64())
			bearing := 2 * math.Pi * rng.Float64()
			n := dist * math.Cos(bearing)
			e := dist * math.Sin(bearing)
			lat := clat + n/witnessMetresPerDegree
			lon := clon + e/(witnessMetresPerDegree*cosCentre)
			cells = append(cells, geohash.EncodeWithPrecision(lat, lon, precision))
		}
		places := independentCellCount(cells)
		stats.minPlaces = min(stats.minPlaces, places)
		stats.maxPlaces = max(stats.maxPlaces, places)
		if places == 1 {
			stats.onePlaceTrials++
		}
		total += places
	}
	stats.meanPlaces = float64(total) / float64(witnessTrials)
	return stats
}

func witnessLog(t *testing.T, city witnessCity, precision uint, crowd int, s witnessStats) {
	t.Helper()
	t.Logf("%s p%d crowd %d: places %d to %d, mean %.2f, one place in %d of %d",
		city.name, precision, crowd, s.minPlaces, s.maxPlaces, s.meanPlaces, s.onePlaceTrials, witnessTrials)
}

func witnessWithin(got, want, tol float64) bool { return math.Abs(got-want) <= tol }

func TestGeohashPrecisionWitnesses(t *testing.T) {
	t.Run("voter precision is 7", func(t *testing.T) {
		if voterGeohashPrecision != 7 {
			t.Fatalf("voterGeohashPrecision is %d; docs/TRUST-MODEL.md derives precision 7 and must change with it", voterGeohashPrecision)
		}
	})

	t.Run("cell size at five Indian latitudes", func(t *testing.T) {
		for _, c := range witnessCities {
			h, w := witnessCellSize(c.lat, c.lon, voterGeohashPrecision)
			if !witnessWithin(h, 152.7, 0.5) {
				t.Errorf("%s p%d: cell height %.2f m, want 152.7 within 0.5", c.name, voterGeohashPrecision, h)
			}
			if !witnessWithin(w, c.widthMetres, 0.5) {
				t.Errorf("%s p%d: cell width %.2f m, want %.1f within 0.5", c.name, voterGeohashPrecision, w, c.widthMetres)
			}
			t.Logf("%s p%d: height %.2f m, width %.2f m", c.name, voterGeohashPrecision, h, w)
		}
	})

	t.Run("derivation table precisions 5 to 8 at Chennai", func(t *testing.T) {
		chennai := witnessCities[0]
		want := []struct {
			precision uint
			h, w      float64
		}{
			{5, 4886.5, 4759.7},
			{6, 610.8, 1189.9},
			{7, 152.7, 148.7},
			{8, 19.1, 37.2},
		}
		for _, r := range want {
			h, w := witnessCellSize(chennai.lat, chennai.lon, r.precision)
			if !witnessWithin(h, r.h, r.h*0.005) {
				t.Errorf("Chennai p%d: height %.2f m, want %.1f within 0.5 percent", r.precision, h, r.h)
			}
			if !witnessWithin(w, r.w, r.w*0.005) {
				t.Errorf("Chennai p%d: width %.2f m, want %.1f within 0.5 percent", r.precision, w, r.w)
			}
			t.Logf("Chennai p%d: height %.2f m, width %.2f m", r.precision, h, w)
		}
	})

	t.Run("300 m circle ceilings", func(t *testing.T) {
		for _, c := range witnessCities {
			h7, w7 := witnessCellSize(c.lat, c.lon, 7)
			got7 := witnessCeiling(h7, w7, witnessRadiusMetres, witnessCeilingSteps)
			if got7 != c.ceiling7 {
				t.Errorf("%s p7: ceiling %d, want %d", c.name, got7, c.ceiling7)
			}
			if got7 > 26 {
				t.Errorf("%s p7: ceiling %d exceeds 26", c.name, got7)
			}
			h6, w6 := witnessCellSize(c.lat, c.lon, 6)
			got6 := witnessCeiling(h6, w6, witnessRadiusMetres, witnessCeilingSteps)
			if got6 != 4 {
				t.Errorf("%s p6: ceiling %d, want 4", c.name, got6)
			}
			t.Logf("%s: p7 cell %.1f m by %.1f m, ceiling %d; p6 ceiling %d", c.name, h7, w7, got7, got6)
		}
	})

	t.Run("precision 7 witnesses in a 300 m circle", func(t *testing.T) {
		for _, c := range witnessCities {
			rng := rand.New(rand.NewSource(42))
			for _, crowd := range []int{2, 5, 50, 200} {
				s := simulateWitnesses(rng, c, voterGeohashPrecision, crowd)
				witnessLog(t, c, voterGeohashPrecision, crowd, s)
				switch crowd {
				case 2:
					if s.onePlaceTrials > 75 {
						t.Errorf("%s p7 crowd 2: %d of %d trials counted one place, want at most 75", c.name, s.onePlaceTrials, witnessTrials)
					}
				case 5:
					if s.meanPlaces < 4.0 {
						t.Errorf("%s p7 crowd 5: mean places %.2f, want at least 4.0", c.name, s.meanPlaces)
					}
				case 50, 200:
					if s.maxPlaces > c.ceiling7 {
						t.Errorf("%s p7 crowd %d: max places %d exceeds ceiling %d (raise witnessCeilingSteps, never loosen the ceiling)", c.name, crowd, s.maxPlaces, c.ceiling7)
					}
					if s.maxPlaces > crowd {
						t.Errorf("%s p7 crowd %d: max places %d exceeds the crowd", c.name, crowd, s.maxPlaces)
					}
					if crowd == 200 && s.minPlaces < 12 {
						t.Errorf("%s p7 crowd 200: min places %d, want at least 12", c.name, s.minPlaces)
					}
				}
			}
		}
	})

	t.Run("precision 6 merges independent witnesses", func(t *testing.T) {
		for _, c := range witnessCities {
			rng := rand.New(rand.NewSource(42))
			// All four crowd sizes are logged so docs/TRUST-MODEL.md can
			// quote a full table; only crowds 2 and 200 are asserted.
			for _, crowd := range []int{2, 5, 50, 200} {
				s := simulateWitnesses(rng, c, 6, crowd)
				witnessLog(t, c, 6, crowd, s)
				if crowd == 2 && s.onePlaceTrials < 200 {
					t.Errorf("%s p6 crowd 2: %d of %d trials merged to one place, want at least 200", c.name, s.onePlaceTrials, witnessTrials)
				}
				if crowd == 200 && s.maxPlaces > 4 {
					t.Errorf("%s p6 crowd 200: max places %d, want at most 4", c.name, s.maxPlaces)
				}
			}
		}
	})

	t.Run("precision 8 counts a crowd nearly one by one", func(t *testing.T) {
		for _, c := range witnessCities {
			rng := rand.New(rand.NewSource(42))
			// All four crowd sizes are logged for the document; only
			// crowd 200 is asserted.
			for _, crowd := range []int{2, 5, 50, 200} {
				s := simulateWitnesses(rng, c, 8, crowd)
				witnessLog(t, c, 8, crowd, s)
				if crowd == 200 && s.meanPlaces < 100 {
					t.Errorf("%s p8 crowd 200: mean places %.2f, want at least 100", c.name, s.meanPlaces)
				}
			}
		}
	})

	t.Run("cell edge pair counts as two places", func(t *testing.T) {
		bengaluru := witnessCities[1]
		cell := geohash.EncodeWithPrecision(bengaluru.lat, bengaluru.lon, voterGeohashPrecision)
		box := geohash.BoundingBox(cell)
		midLon := (box.MinLng + box.MaxLng) / 2
		midLat := (box.MinLat + box.MaxLat) / 2
		half := 0.5 / witnessMetresPerDegree
		south := geohash.EncodeWithPrecision(box.MaxLat-half, midLon, voterGeohashPrecision)
		north := geohash.EncodeWithPrecision(box.MaxLat+half, midLon, voterGeohashPrecision)
		if got := independentCellCount([]string{south, north}); got != 2 {
			t.Errorf("points 1 m apart across a cell edge: %d places, want 2 (cells %s and %s)", got, south, north)
		}
		t.Log(fmt.Sprintf("cell edge pair: %s and %s", south, north))
		a := geohash.EncodeWithPrecision(midLat, midLon, voterGeohashPrecision)
		b := geohash.EncodeWithPrecision(midLat+1/witnessMetresPerDegree, midLon, voterGeohashPrecision)
		if got := independentCellCount([]string{a, b}); got != 1 {
			t.Errorf("points 1 m apart near the cell centre: %d places, want 1", got)
		}
	})
}
