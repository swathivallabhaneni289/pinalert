// Command buildareas turns the upstream LGD district boundary file into the
// small derived file that internal/areas embeds in the server binary.
//
// Usage:
//
//	go run ./cmd/buildareas -in /path/to/LGD_Districts.geojsonl
//	go run ./cmd/buildareas -in /path/to/LGD_Districts.geojsonl -out internal/areas/data/districts.json.gz -tolerance 0.01 -decimals 3
//
// The input is the "districts" release of ramSeraph/indian_admin_boundaries
// (asset LGD_Districts.geojsonl.7z, unpacked by hand). It is untrusted data:
// keep it in a scratch folder outside the repo and pass its path here. This
// command only reads it as text; it never runs anything from that folder.
//
// Each line is one GeoJSON Feature. The properties used are dist_lgd (the LGD
// district code, the join key for CAP alerts), dtname, state_lgd and stname.
// The census 2011 code that is also on every line is ignored on purpose: it
// does not match the codes SACHET puts in its messages.
//
// Every ring is simplified with Douglas-Peucker at -tolerance degrees, rounded
// to -decimals places, and polygons that share a code are merged. The output is
// gzip compressed JSON with no timestamp and no file name, so two runs over the
// same input give the same bytes.
//
// Source credit: Local Government Directory (LGD) and Bharatmaps district
// boundaries, compiled by ramSeraph for the Datameet community, dedicated
// CC0 1.0 by the compiler.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultTolerance = 0.01
	defaultDecimals  = 3
	sourceNote       = "LGD district boundaries via ramSeraph/indian_admin_boundaries (CC0 by the compiler, credit Datameet and the original government sources)"
	maxLineBytes     = 64 << 20
)

// tolerance and decimals are set from the flags in main and read by buildFile.
var (
	tolerance = defaultTolerance
	decimals  = defaultDecimals
)

// areaDistrict is one district in the output file. Polys is a list of
// polygons; each polygon is a list of rings (the outer ring first, then holes);
// each ring is a closed list of [lon, lat] points.
type areaDistrict struct {
	Code      int              `json:"code"`
	Name      string           `json:"name"`
	StateCode int              `json:"state_code"`
	State     string           `json:"state"`
	Polys     [][][][2]float64 `json:"polys"`
}

// areaFile is the whole output file.
type areaFile struct {
	V         int            `json:"v"`
	Source    string         `json:"source"`
	Tolerance float64        `json:"tolerance"`
	Decimals  int            `json:"decimals"`
	Districts []areaDistrict `json:"districts"`
}

// stats is what the builder prints on one line.
type stats struct {
	Features  int
	Districts int
	Points    int
	Bytes     int
}

// flexInt reads a JSON number or a numeric string.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		return errors.New("missing number")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("not a number: %q", s)
	}
	if v != math.Trunc(v) || v < 0 || v > math.MaxInt32 {
		return fmt.Errorf("not a whole number: %q", s)
	}
	*f = flexInt(int(v))
	return nil
}

type feature struct {
	Type       string `json:"type"`
	Properties struct {
		DistLGD  *flexInt `json:"dist_lgd"`
		DtName   string   `json:"dtname"`
		StateLGD *flexInt `json:"state_lgd"`
		StName   string   `json:"stname"`
	} `json:"properties"`
	Geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
}

func main() {
	in := flag.String("in", "", "path to the unpacked LGD_Districts.geojsonl")
	out := flag.String("out", "internal/areas/data/districts.json.gz", "where to write the embedded file")
	tol := flag.Float64("tolerance", defaultTolerance, "simplification tolerance in degrees")
	dec := flag.Int("decimals", defaultDecimals, "decimal places kept on every coordinate")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: buildareas -in LGD_Districts.geojsonl [-out file.json.gz] [-tolerance 0.01] [-decimals 3]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *in == "" || *tol < 0 || *dec < 0 || *dec > 8 {
		flag.Usage()
		os.Exit(2)
	}
	tolerance, decimals = *tol, *dec

	src, err := os.Open(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "buildareas:", err)
		os.Exit(1)
	}
	defer src.Close()

	f, st, err := buildFile(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "buildareas:", err)
		os.Exit(1)
	}
	var buf bytes.Buffer
	if err := writeFile(&buf, f); err != nil {
		fmt.Fprintln(os.Stderr, "buildareas:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "buildareas:", err)
		os.Exit(1)
	}
	st.Bytes = buf.Len()
	fmt.Fprintf(os.Stderr, "buildareas: features read %d, districts written %d, points written %d, bytes written %d\n",
		st.Features, st.Districts, st.Points, st.Bytes)
}

// buildFile reads one GeoJSON Feature per line and returns the merged,
// simplified, rounded districts sorted by code. Any malformed line is an error
// that names its line number.
func buildFile(r io.Reader) (areaFile, stats, error) {
	var st stats
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), maxLineBytes)

	byCode := map[int]*areaDistrict{}
	line := 0
	for sc.Scan() {
		line++
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		var ft feature
		if err := json.Unmarshal(raw, &ft); err != nil {
			return areaFile{}, st, fmt.Errorf("line %d: not valid JSON: %v", line, err)
		}
		if ft.Type != "Feature" {
			return areaFile{}, st, fmt.Errorf("line %d: type %q is not a Feature", line, ft.Type)
		}
		if ft.Properties.DistLGD == nil || ft.Properties.StateLGD == nil {
			return areaFile{}, st, fmt.Errorf("line %d: dist_lgd or state_lgd is missing", line)
		}
		polys, err := readGeometry(ft.Geometry.Type, ft.Geometry.Coordinates)
		if err != nil {
			return areaFile{}, st, fmt.Errorf("line %d: %v", line, err)
		}
		st.Features++

		var kept [][][][2]float64
		for _, poly := range polys {
			if p, ok := prepPolygon(poly); ok {
				kept = append(kept, p)
			}
		}
		if len(kept) == 0 {
			return areaFile{}, st, fmt.Errorf("line %d: no usable polygon left after rounding", line)
		}
		code := int(*ft.Properties.DistLGD)
		d, ok := byCode[code]
		if !ok {
			d = &areaDistrict{
				Code:      code,
				Name:      ft.Properties.DtName,
				StateCode: int(*ft.Properties.StateLGD),
				State:     ft.Properties.StName,
			}
			byCode[code] = d
		}
		d.Polys = append(d.Polys, kept...)
	}
	if err := sc.Err(); err != nil {
		return areaFile{}, st, fmt.Errorf("line %d: %v", line+1, err)
	}
	if len(byCode) == 0 {
		return areaFile{}, st, errors.New("no features found")
	}

	f := areaFile{V: 1, Source: sourceNote, Tolerance: tolerance, Decimals: decimals}
	for _, d := range byCode {
		f.Districts = append(f.Districts, *d)
	}
	sort.Slice(f.Districts, func(i, j int) bool { return f.Districts[i].Code < f.Districts[j].Code })
	st.Districts = len(f.Districts)
	for _, d := range f.Districts {
		for _, p := range d.Polys {
			for _, ring := range p {
				st.Points += len(ring)
			}
		}
	}
	return f, st, nil
}

// readGeometry returns the polygons of a Polygon or MultiPolygon geometry as
// rings of [lon, lat] points. Any other geometry type is an error.
func readGeometry(kind string, coords json.RawMessage) ([][][][2]float64, error) {
	switch kind {
	case "Polygon":
		var p [][][]float64
		if err := json.Unmarshal(coords, &p); err != nil {
			return nil, fmt.Errorf("bad Polygon coordinates: %v", err)
		}
		poly, err := toRings(p)
		if err != nil {
			return nil, err
		}
		return [][][][2]float64{poly}, nil
	case "MultiPolygon":
		var mp [][][][]float64
		if err := json.Unmarshal(coords, &mp); err != nil {
			return nil, fmt.Errorf("bad MultiPolygon coordinates: %v", err)
		}
		out := make([][][][2]float64, 0, len(mp))
		for _, p := range mp {
			poly, err := toRings(p)
			if err != nil {
				return nil, err
			}
			out = append(out, poly)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("geometry %q is neither Polygon nor MultiPolygon", kind)
	}
}

func toRings(p [][][]float64) ([][][2]float64, error) {
	rings := make([][][2]float64, 0, len(p))
	for _, r := range p {
		ring := make([][2]float64, 0, len(r))
		for _, pt := range r {
			if len(pt) < 2 {
				return nil, errors.New("a point has fewer than 2 numbers")
			}
			ring = append(ring, [2]float64{pt[0], pt[1]})
		}
		rings = append(rings, ring)
	}
	return rings, nil
}

// prepPolygon simplifies and rounds every ring of one polygon. A hole that
// degenerates is dropped; an outer ring that degenerates drops the polygon.
func prepPolygon(rings [][][2]float64) ([][][2]float64, bool) {
	var out [][][2]float64
	for i, ring := range rings {
		simple := simplifyRing(ring, tolerance)
		rounded := roundRing(simple, decimals)
		if len(rounded) < 4 {
			// The simplified form collapsed once rounded; try the original.
			rounded = roundRing(ring, decimals)
		}
		if len(rounded) < 4 {
			if i == 0 {
				return nil, false
			}
			continue
		}
		out = append(out, rounded)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// roundRing rounds every coordinate to the given decimals, drops consecutive
// duplicate points and makes sure the ring is closed.
func roundRing(ring [][2]float64, places int) [][2]float64 {
	scale := math.Pow(10, float64(places))
	out := make([][2]float64, 0, len(ring))
	for _, p := range ring {
		q := [2]float64{math.Round(p[0]*scale) / scale, math.Round(p[1]*scale) / scale}
		if n := len(out); n > 0 && out[n-1] == q {
			continue
		}
		out = append(out, q)
	}
	if n := len(out); n > 0 && out[0] != out[n-1] {
		out = append(out, out[0])
	}
	return out
}

// simplifyRing runs an iterative Douglas-Peucker pass over a ring. A closed
// ring stays closed. A ring that would fall under 4 points is returned as it
// was, so a small island never disappears.
func simplifyRing(ring [][2]float64, tol float64) [][2]float64 {
	n := len(ring)
	if n < 5 {
		return append([][2]float64(nil), ring...)
	}
	closed := ring[0] == ring[n-1]
	var keep []bool
	if closed {
		// A closed ring has no single baseline, so split it at the point
		// farthest from the start and simplify the two halves.
		far, best := 0, -1.0
		for i := 1; i < n-1; i++ {
			d := math.Hypot(ring[i][0]-ring[0][0], ring[i][1]-ring[0][1])
			if d > best {
				far, best = i, d
			}
		}
		keep = make([]bool, n)
		keep[0], keep[n-1], keep[far] = true, true, true
		douglasPeucker(ring, 0, far, tol, keep)
		douglasPeucker(ring, far, n-1, tol, keep)
	} else {
		keep = make([]bool, n)
		keep[0], keep[n-1] = true, true
		douglasPeucker(ring, 0, n-1, tol, keep)
	}
	out := make([][2]float64, 0, n)
	for i, k := range keep {
		if k {
			out = append(out, ring[i])
		}
	}
	if len(out) < 4 {
		return append([][2]float64(nil), ring...)
	}
	return out
}

// douglasPeucker marks the points of ring[lo..hi] that must be kept. It uses an
// explicit stack, not recursion, because one district can have tens of
// thousands of points.
func douglasPeucker(ring [][2]float64, lo, hi int, tol float64, keep []bool) {
	type span struct{ lo, hi int }
	stack := []span{{lo, hi}}
	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if s.hi-s.lo < 2 {
			continue
		}
		idx, maxD := -1, 0.0
		for i := s.lo + 1; i < s.hi; i++ {
			d := segDistance(ring[i], ring[s.lo], ring[s.hi])
			if d > maxD {
				idx, maxD = i, d
			}
		}
		if idx >= 0 && maxD > tol {
			keep[idx] = true
			stack = append(stack, span{s.lo, idx}, span{idx, s.hi})
		}
	}
}

// segDistance is the distance from p to the segment a-b, in degrees.
func segDistance(p, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	if dx == 0 && dy == 0 {
		return math.Hypot(p[0]-a[0], p[1]-a[1])
	}
	t := ((p[0]-a[0])*dx + (p[1]-a[1])*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(p[0]-(a[0]+t*dx), p[1]-(a[1]+t*dy))
}

// writeFile writes the file as gzip compressed JSON. The gzip header carries no
// modification time and no name, so the same input always gives the same bytes.
func writeFile(w io.Writer, f areaFile) error {
	body, err := json.Marshal(f)
	if err != nil {
		return err
	}
	zw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	zw.Header = gzip.Header{}
	if _, err := zw.Write(body); err != nil {
		return err
	}
	return zw.Close()
}
