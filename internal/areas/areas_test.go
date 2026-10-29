package areas

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mmcloughlin/geohash"
)

func TestEmbeddedFileSizeBand(t *testing.T) {
	n := len(embedded)
	if n < 200_000 || n > 800_000 {
		t.Fatalf("embedded file is %d bytes, want between 200000 and 800000", n)
	}
}

func TestEmbeddedFileParses(t *testing.T) {
	ix := Default()
	ds := ix.Districts()
	if len(ds) < 780 || len(ds) > 800 {
		t.Fatalf("got %d districts, want between 780 and 800", len(ds))
	}
	seen := map[int]bool{}
	for _, d := range ds {
		if d.Code <= 0 {
			t.Errorf("district %q has a non positive code %d", d.Name, d.Code)
		}
		if seen[d.Code] {
			t.Errorf("code %d appears twice", d.Code)
		}
		seen[d.Code] = true
		if d.Name == "" || d.State == "" {
			t.Errorf("code %d has no name or state: %+v", d.Code, d)
		}
	}
	for i, polys := range ix.polys {
		if len(polys) == 0 {
			t.Errorf("district %d has no polygon", ix.districts[i].Code)
		}
		for _, p := range polys {
			for _, ring := range p {
				if len(ring) < 4 {
					t.Errorf("district %d has a ring of %d points", ix.districts[i].Code, len(ring))
				}
			}
		}
	}
}

func TestLocateAnchors(t *testing.T) {
	ix := Default()
	cases := []struct {
		name     string
		lat, lon float64
		code     int
		state    string
	}{
		{"Vijayawada", 16.5062, 80.6480, 749, "andhra pradesh"},
		{"Chennai", 13.0827, 80.2707, 568, "tamil nadu"},
		{"Kolhapur", 16.7050, 74.2433, 480, "maharashtra"},
	}
	for _, c := range cases {
		d, ok := ix.Locate(c.lat, c.lon)
		if !ok {
			t.Errorf("%s: not found", c.name)
			continue
		}
		if d.Code != c.code {
			t.Errorf("%s: got code %d (%s), want %d", c.name, d.Code, d.Name, c.code)
		}
		if strings.ToLower(d.State) != c.state {
			t.Errorf("%s: got state %q, want %q", c.name, d.State, c.state)
		}
	}
}

func TestByCodeCapCodes(t *testing.T) {
	ix := Default()
	want := map[int]string{
		574: "kanchipuram",
		568: "chennai",
		730: "chengalpattu",
		733: "tenkasi",
		731: "ranipet",
		492: "ratnagiri",
		480: "kolhapur",
		495: "sindhudurg",
	}
	for code, name := range want {
		d, ok := ix.ByCode(code)
		if !ok {
			t.Errorf("code %d did not resolve", code)
			continue
		}
		if strings.ToLower(d.Name) != name {
			t.Errorf("code %d: got %q want %q", code, d.Name, name)
		}
	}
	for _, code := range []int{0, 99999} {
		if _, ok := ix.ByCode(code); ok {
			t.Errorf("code %d should not resolve", code)
		}
	}
}

func TestRayCasting(t *testing.T) {
	square := [][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}
	hole := [][2]float64{{4, 4}, {6, 4}, {6, 6}, {4, 6}, {4, 4}}
	far := [][2]float64{{20, 20}, {30, 20}, {30, 30}, {20, 30}, {20, 20}}

	if !inRing(5, 5, square) {
		t.Error("centre of the square should be inside")
	}
	if inRing(15, 5, square) {
		t.Error("point right of the square should be outside")
	}
	withHole := polygon{square, hole}
	if inPolygon(5, 5, withHole) {
		t.Error("a point inside the hole is not inside the polygon")
	}
	if !inPolygon(2, 2, withHole) {
		t.Error("a point in the ring body should be inside")
	}
	// A multipolygon is a list of polygons; the second part must count too.
	multi := []polygon{{square}, {far}}
	in := func(x, y float64) bool {
		for _, p := range multi {
			if inPolygon(x, y, p) {
				return true
			}
		}
		return false
	}
	if !in(25, 25) {
		t.Error("a point inside the second part of a multipolygon should be inside")
	}
	if in(15, 15) {
		t.Error("a point between the parts should be outside")
	}
	// A point exactly on an edge may go either way but must never flip.
	first := inRing(10, 5, square)
	for i := 0; i < 50; i++ {
		if inRing(10, 5, square) != first {
			t.Fatal("an edge point changed its answer between calls")
		}
	}
	firstCorner := inRing(0, 0, square)
	for i := 0; i < 50; i++ {
		if inRing(0, 0, square) != firstCorner {
			t.Fatal("a corner point changed its answer between calls")
		}
	}
}

func TestLocateOutsideIndia(t *testing.T) {
	ix := Default()
	if d, ok := ix.Locate(51.5, -0.12); ok {
		t.Errorf("London resolved to %+v", d)
	}
	if d, ok := ix.Locate(0, 0); ok {
		t.Errorf("0,0 resolved to %+v", d)
	}
}

// twoDistrictIndex builds a tiny index in which the geohash cell around the
// given point is split between two districts, to test CellDistricts without
// depending on where real district borders run.
func twoDistrictIndex(t *testing.T, lat, lon float64) (*Index, string) {
	t.Helper()
	cell := geohash.EncodeWithPrecision(lat, lon, 5)
	box := geohash.BoundingBox(cell)
	split := box.MinLng + (box.MaxLng-box.MinLng)*0.25 // west quarter is district 1
	rect := func(w, e float64) [][][][2]float64 {
		return [][][][2]float64{{{
			{w, box.MinLat - 1}, {e, box.MinLat - 1}, {e, box.MaxLat + 1}, {w, box.MaxLat + 1}, {w, box.MinLat - 1},
		}}}
	}
	f := map[string]any{
		"v": 1, "source": "test", "tolerance": 0.01, "decimals": 3,
		"districts": []map[string]any{
			{"code": 1, "name": "West", "state_code": 9, "state": "TESTLAND", "polys": rect(box.MinLng-1, split)},
			{"code": 2, "name": "East", "state_code": 9, "state": "TESTLAND", "polys": rect(split, box.MaxLng+1)},
		},
	}
	body, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(body)
	zw.Close()
	ix, err := Load(buf.Bytes())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return ix, cell
}

func TestCellDistricts(t *testing.T) {
	ix := Default()
	cell := geohash.EncodeWithPrecision(16.5062, 80.6480, 5)
	got := ix.CellDistricts(cell)
	found := false
	for _, d := range got {
		if d.Code == 749 {
			found = true
		}
	}
	if !found {
		t.Errorf("cell %s of Vijayawada should include 749, got %+v", cell, got)
	}

	syn, scell := twoDistrictIndex(t, 16.5062, 80.6480)
	two := syn.CellDistricts(scell)
	if len(two) != 2 || two[0].Code != 1 || two[1].Code != 2 {
		t.Errorf("a cell split between two districts should return both in code order, got %+v", two)
	}

	for _, bad := range []string{"", "abc!!", "tdr1", "TDR1V", "tdr1vv"} {
		if got := ix.CellDistricts(bad); got != nil {
			t.Errorf("CellDistricts(%q) = %+v, want nil", bad, got)
		}
	}
}

func TestLocateIsFast(t *testing.T) {
	ix := Default()
	pts := [][2]float64{{16.5062, 80.6480}, {13.0827, 80.2707}, {16.7050, 74.2433}, {51.5, -0.12}}
	start := time.Now()
	for i := 0; i < 1000; i++ {
		p := pts[i%len(pts)]
		ix.Locate(p[0], p[1])
	}
	if el := time.Since(start); el > time.Second {
		t.Fatalf("1000 lookups took %v, want under 1s", el)
	}
}

// TestDisplayNamesApply pins WR-S07 of the Phase 5 review: every entry of the
// display table matches its code and raw LGD name in the embedded file, so the
// shown name is used, and a district outside the table keeps its LGD name.
func TestDisplayNamesApply(t *testing.T) {
	ix := Default()
	for code, n := range displayNames {
		d, ok := ix.ByCode(code)
		if !ok {
			t.Errorf("display table code %d is not in the boundary file", code)
			continue
		}
		if d.Name != n.shown {
			t.Errorf("district %d is named %q, want %q (is its LGD name still %q?)", code, d.Name, n.shown, n.raw)
		}
	}
	if d, _ := ix.ByCode(749); d.Name != "NTR" {
		t.Errorf("district 749 = %q, want NTR", d.Name)
	}
	if d, _ := ix.ByCode(79); d.Name != "New Delhi" {
		t.Errorf("district 79 = %q, want New Delhi as the file has it", d.Name)
	}
	if got := displayName(749, "Something else"); got != "Something else" {
		t.Errorf("a code whose raw name changed is shown as %q, want its own name", got)
	}
}
