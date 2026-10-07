// geo_test.go declares package service (the internal test package, not
// service_test) because haversineKm and withinVoteRadius are unexported, so
// the tests must live inside the package to reach them, as trust_test.go
// does for independentCellCount. Every expected number below was measured
// with R = 6371 km using the exact haversine form geo.go implements.

package service

import (
	"math"
	"math/rand/v2"
	"strconv"
	"testing"
)

// kmApproxEqual reports whether a and b differ by at most tol.
func kmApproxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestHaversine(t *testing.T) {
	cases := []struct {
		name                   string
		lat1, lon1, lat2, lon2 float64
		want, tol              float64
	}{
		{"Chennai to Bengaluru", 13.0827, 80.2707, 12.9716, 77.5946, 290.1720, 0.01},
		{"0.003 degrees of latitude near Bengaluru", 12.9716, 77.5946, 12.9746, 77.5946, 0.3336, 0.0005},
		{"0.01 degrees of longitude at 13N", 13, 77.5, 13, 77.51, 1.0835, 0.0005},
		{"0.01 degrees of longitude at 28.6N", 28.6, 77.5, 28.6, 77.51, 0.9763, 0.0005},
		{"0.01 degrees of longitude at 34N", 34, 77.5, 34, 77.51, 0.9219, 0.0005},
		{"1 km east at 8N", 8, 77.5, 8, 77.5 + 0.009082, 1.0, 0.0001},
		{"1 km east at 13N", 13, 77.5, 13, 77.5 + 0.009230, 1.0, 0.0001},
		{"1 km east at 28.6N", 28.6, 77.5, 28.6, 77.5 + 0.010243, 1.0, 0.0001},
		{"1 km east at 34N", 34, 77.5, 34, 77.5 + 0.010848, 1.0, 0.0001},
		{"1 km east at 37N", 37, 77.5, 37, 77.5 + 0.011261, 1.0, 0.0001},
		{"antipodes stay finite", 0, 0, 0, 180, 20015.0868, 0.01},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := haversineKm(tc.lat1, tc.lon1, tc.lat2, tc.lon2)
			if math.IsNaN(got) || !kmApproxEqual(got, tc.want, tc.tol) {
				t.Errorf("haversineKm = %v, want %v within %v", got, tc.want, tc.tol)
			}
			back := haversineKm(tc.lat2, tc.lon2, tc.lat1, tc.lon1)
			if !kmApproxEqual(got, back, 1e-9) {
				t.Errorf("forward %v and reverse %v differ by more than 1e-9", got, back)
			}
		})
	}

	t.Run("identical points are exactly zero", func(t *testing.T) {
		if got := haversineKm(12.9716, 77.5946, 12.9716, 77.5946); got != 0 {
			t.Errorf("haversineKm = %v, want exactly 0", got)
		}
	})
}

func TestWithinVoteRadius(t *testing.T) {
	t.Run("identical points are within", func(t *testing.T) {
		if !withinVoteRadius(12.9716, 77.5946, 12.9716, 77.5946) {
			t.Error("identical points should be within the radius")
		}
	})

	t.Run("Chennai to Bengaluru is not within", func(t *testing.T) {
		if withinVoteRadius(13.0827, 80.2707, 12.9716, 77.5946) {
			t.Error("290 km apart should not be within the radius")
		}
	})

	// Every non finite value in each of the four argument positions in turn
	// is refused: 3 values times 4 positions is 12 cases.
	bad := map[string]float64{"NaN": math.NaN(), "+Inf": math.Inf(1), "-Inf": math.Inf(-1)}
	positions := []string{"report latitude", "report longitude", "voter latitude", "voter longitude"}
	for name, v := range bad {
		for pos := range positions {
			t.Run(name+" in "+positions[pos], func(t *testing.T) {
				args := [4]float64{12.9716, 77.5946, 12.9716, 77.5946}
				args[pos] = v
				if withinVoteRadius(args[0], args[1], args[2], args[3]) {
					t.Errorf("withinVoteRadius%v = true, want false (fail closed)", args)
				}
			})
		}
	}
}

func TestWithinVoteRadiusBoundary(t *testing.T) {
	const rLat, rLon = 12.9716, 77.5946
	cases := []struct {
		name         string
		rLat, rLon   float64
		vLat, vLon   float64
		wantInRadius bool
	}{
		{"0.9896 km north is within", rLat, rLon, 12.9805, 77.5946, true},
		{"1.0008 km north is not within", rLat, rLon, 12.9806, 77.5946, false},
		{"0.01 degrees of longitude at 13N is 1.0835 km, not within", 13, 77.5, 13, 77.51, false},
		{"0.01 degrees of longitude at 28.6N is 0.9763 km, within", 28.6, 77.5, 28.6, 77.51, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := withinVoteRadius(tc.rLat, tc.rLon, tc.vLat, tc.vLon); got != tc.wantInRadius {
				t.Errorf("withinVoteRadius = %v, want %v", got, tc.wantInRadius)
			}
		})
	}
}

// TestHaversineInsideBoundingBox is a seeded property test: every point the
// haversine accepts lies inside BoundingBox for the same radius, so the box
// is a safe prefilter for the true circle at Indian latitudes (V-03).
func TestHaversineInsideBoundingBox(t *testing.T) {
	const samples = 2000
	const centreLon = 77.5
	for _, centreLat := range []float64{8, 13, 28.6, 34, 37} {
		t.Run(strconv.FormatFloat(centreLat, 'f', -1, 64)+"N", func(t *testing.T) {
			rng := rand.New(rand.NewPCG(42, 1))
			latSpan := 1.2 / 111.0
			lonSpan := 1.2 / (111.0 * math.Cos(centreLat*math.Pi/180))
			latMin, latMax, lonMin, lonMax := BoundingBox(centreLat, centreLon, MaxVoteDistanceKm)

			accepted := 0
			for i := 0; i < samples; i++ {
				lat := centreLat + (rng.Float64()*2-1)*latSpan
				lon := centreLon + (rng.Float64()*2-1)*lonSpan
				if haversineKm(centreLat, centreLon, lat, lon) > MaxVoteDistanceKm {
					continue
				}
				accepted++
				if lat < latMin || lat > latMax || lon < lonMin || lon > lonMax {
					t.Fatalf("point (%v, %v) is within the radius but outside the box [%v,%v] x [%v,%v]",
						lat, lon, latMin, latMax, lonMin, lonMax)
				}
			}
			if accepted < 500 {
				t.Errorf("only %d of %d samples were accepted, want at least 500 so the property is not vacuous", accepted, samples)
			}
		})
	}
}

func TestTooFarMessage(t *testing.T) {
	const want = "Too far to vote. You need to be within 1 km of this report."
	got := TooFarMessage()
	if got != want {
		t.Errorf("TooFarMessage() = %q, want %q", got, want)
	}
	built := "Too far to vote. You need to be within " + strconv.FormatFloat(MaxVoteDistanceKm, 'f', -1, 64) + " km of this report."
	if got != built {
		t.Errorf("TooFarMessage() = %q, want the string built from MaxVoteDistanceKm %q", got, built)
	}
	for _, r := range got {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == ' ' || r == ',' || r == '.' || r == '\'' || r == '+'
		if !ok {
			t.Errorf("TooFarMessage() contains %q (U+%04X), outside the closed character set", r, r)
		}
	}
}
