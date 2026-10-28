// Package areas says which Indian district, and so which state, a point is in,
// and which district an official alert's LGD code names. It needs no database
// and no network: the boundaries are embedded in the binary.
//
// The data is the LGD district boundary layer from the ramSeraph
// indian_admin_boundaries release "districts", built from Local Government
// Directory and Bharatmaps data. The compiler dedicated it under CC0 1.0 and
// asks for credit to the Datameet community and the original government
// sources, which is given here and on the Policy page.
//
// The embedded file is not the upstream file. cmd/buildareas simplifies every
// ring with Douglas-Peucker at 0.01 degree (about 1.1 km), rounds to 3 decimals
// and merges polygons that share a code, which turns 86 MB into about 0.35 MB.
// Run it again with the upstream file to rebuild:
//
//	go run ./cmd/buildareas -in LGD_Districts.geojsonl
//
// Accuracy limit: because of that simplification a point within about a
// kilometre of a district edge can land in the neighbouring district. That is
// acceptable at district scale, where the answer only chooses which official
// alerts to show and which push recipients to consider. The boundaries are used
// only for point in polygon tests and are never drawn on a map.
//
// Districts are keyed on dist_lgd, the LGD district code that SACHET's CAP
// messages carry. The 2011 census code on the upstream lines does not match
// those messages and is not read.
package areas

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/mmcloughlin/geohash"
)

//go:embed data/districts.json.gz
var embedded []byte

// District is one LGD district. The polygons stay inside the Index.
type District struct {
	Code      int
	Name      string
	StateCode int
	State     string
	// BBox is [minLon, minLat, maxLon, maxLat].
	BBox [4]float64
}

// State is a state or union territory, named as the boundary file names it
// (upper case, for example "TAMIL NADU").
type State struct {
	Code int
	Name string
}

// polygon is an outer ring followed by its holes. A ring is a closed list of
// [lon, lat] points.
type polygon [][][2]float64

// Index answers the lookups. It is read only after Load and safe to share.
type Index struct {
	districts []District  // sorted by code
	polys     [][]polygon // parallel to districts
	byCode    map[int]int // code to position in districts
	states    []State     // sorted by code
	stateNorm map[string]State
}

// fileJSON mirrors the file written by cmd/buildareas.
type fileJSON struct {
	V         int `json:"v"`
	Districts []struct {
		Code      int              `json:"code"`
		Name      string           `json:"name"`
		StateCode int              `json:"state_code"`
		State     string           `json:"state"`
		Polys     [][][][2]float64 `json:"polys"`
	} `json:"districts"`
}

var (
	defaultOnce sync.Once
	defaultIx   *Index
	defaultErr  error
)

// Default decodes the embedded file once and returns the shared index. It
// panics on a build defect, because the file is in the binary and tested.
func Default() *Index {
	defaultOnce.Do(func() {
		defaultIx, defaultErr = Load(embedded)
	})
	if defaultErr != nil {
		panic("areas: embedded district file is unusable: " + defaultErr.Error())
	}
	return defaultIx
}

// Load decodes a gzip compressed district file.
func Load(raw []byte) (*Index, error) {
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("not gzip: %w", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}
	var f fileJSON
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("bad json: %w", err)
	}
	if f.V != 1 {
		return nil, fmt.Errorf("unknown file version %d", f.V)
	}
	if len(f.Districts) == 0 {
		return nil, fmt.Errorf("no districts")
	}

	ix := &Index{byCode: map[int]int{}, stateNorm: map[string]State{}}
	sort.SliceStable(f.Districts, func(i, j int) bool { return f.Districts[i].Code < f.Districts[j].Code })
	stateSeen := map[int]State{}
	for _, d := range f.Districts {
		if _, dup := ix.byCode[d.Code]; dup {
			return nil, fmt.Errorf("code %d appears twice", d.Code)
		}
		if len(d.Polys) == 0 {
			return nil, fmt.Errorf("district %d has no polygon", d.Code)
		}
		dist := District{
			Code: d.Code, Name: d.Name, StateCode: d.StateCode, State: d.State,
			BBox: [4]float64{180, 90, -180, -90},
		}
		polys := make([]polygon, 0, len(d.Polys))
		for _, p := range d.Polys {
			if len(p) == 0 {
				continue
			}
			for _, ring := range p {
				if len(ring) < 4 {
					return nil, fmt.Errorf("district %d has a ring of %d points", d.Code, len(ring))
				}
			}
			for _, pt := range p[0] {
				if pt[0] < dist.BBox[0] {
					dist.BBox[0] = pt[0]
				}
				if pt[1] < dist.BBox[1] {
					dist.BBox[1] = pt[1]
				}
				if pt[0] > dist.BBox[2] {
					dist.BBox[2] = pt[0]
				}
				if pt[1] > dist.BBox[3] {
					dist.BBox[3] = pt[1]
				}
			}
			polys = append(polys, polygon(p))
		}
		if len(polys) == 0 {
			return nil, fmt.Errorf("district %d has no usable polygon", d.Code)
		}
		ix.byCode[d.Code] = len(ix.districts)
		ix.districts = append(ix.districts, dist)
		ix.polys = append(ix.polys, polys)
		if _, ok := stateSeen[d.StateCode]; !ok {
			stateSeen[d.StateCode] = State{Code: d.StateCode, Name: d.State}
		}
	}
	for _, s := range stateSeen {
		ix.states = append(ix.states, s)
	}
	sort.Slice(ix.states, func(i, j int) bool { return ix.states[i].Code < ix.states[j].Code })
	ix.buildStateNames()
	return ix, nil
}

// Districts returns every district in code order. The slice is a copy.
func (ix *Index) Districts() []District {
	return append([]District(nil), ix.districts...)
}

// States returns every state in code order. The slice is a copy.
func (ix *Index) States() []State {
	return append([]State(nil), ix.states...)
}

// ByCode returns the district with the given LGD district code.
func (ix *Index) ByCode(code int) (District, bool) {
	i, ok := ix.byCode[code]
	if !ok {
		return District{}, false
	}
	return ix.districts[i], true
}

// Locate returns the district that contains the point. Districts are scanned
// in code order, so a point on a shared edge always gets the same answer.
func (ix *Index) Locate(lat, lon float64) (District, bool) {
	for i := range ix.districts {
		b := ix.districts[i].BBox
		if lon < b[0] || lon > b[2] || lat < b[1] || lat > b[3] {
			continue
		}
		for _, p := range ix.polys[i] {
			if inPolygon(lon, lat, p) {
				return ix.districts[i], true
			}
		}
	}
	return District{}, false
}

// CellDistricts returns the districts found at the centre and the four corners
// of a 5 character geohash cell (the stored home area shape), without repeats,
// in code order. It returns nil for anything that is not such a cell. A cell is
// about 4.9 km square, so testing five points biases toward telling the person,
// never away. The reader view, the push queue and the push worker all use this
// one function so the home area rule lives in one place.
func (ix *Index) CellDistricts(cell string) []District {
	if !validCell(cell) {
		return nil
	}
	box := geohash.BoundingBox(cell)
	centreLat, centreLng := box.Center()
	pts := [][2]float64{
		{centreLat, centreLng},
		{box.MinLat, box.MinLng},
		{box.MinLat, box.MaxLng},
		{box.MaxLat, box.MinLng},
		{box.MaxLat, box.MaxLng},
	}
	seen := map[int]bool{}
	var out []District
	for _, p := range pts {
		d, ok := ix.Locate(p[0], p[1])
		if !ok || seen[d.Code] {
			continue
		}
		seen[d.Code] = true
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

const geohashAlphabet = "0123456789bcdefghjkmnpqrstuvwxyz"

func validCell(cell string) bool {
	if len(cell) != 5 {
		return false
	}
	for i := 0; i < len(cell); i++ {
		found := false
		for j := 0; j < len(geohashAlphabet); j++ {
			if cell[i] == geohashAlphabet[j] {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// inRing is the even odd rule: cast a ray to the right and count the edges it
// crosses. x is the longitude and y the latitude.
func inRing(x, y float64, r [][2]float64) bool {
	in := false
	for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
		xi, yi, xj, yj := r[i][0], r[i][1], r[j][0], r[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}

// inPolygon is true when the point is inside the outer ring and inside none of
// the holes.
func inPolygon(x, y float64, p polygon) bool {
	if len(p) == 0 || !inRing(x, y, p[0]) {
		return false
	}
	for _, hole := range p[1:] {
		if inRing(x, y, hole) {
			return false
		}
	}
	return true
}
