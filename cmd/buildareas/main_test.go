package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSimplifyRingKeepsClosure(t *testing.T) {
	// A closed 9 point square with a collinear midpoint on every side.
	ring := [][2]float64{
		{0, 0}, {0.5, 0}, {1, 0}, {1, 0.5}, {1, 1}, {0.5, 1}, {0, 1}, {0, 0.5}, {0, 0},
	}
	got := simplifyRing(ring, 0.01)
	if len(got) != 5 {
		t.Fatalf("collinear points should drop to the 4 corners plus the closing point, got %d points: %v", len(got), got)
	}
	if got[0] != got[len(got)-1] {
		t.Fatalf("ring must stay closed, first %v last %v", got[0], got[len(got)-1])
	}
	if got[0] != ring[0] {
		t.Fatalf("ring must still start where it started, got %v", got[0])
	}

	// A thin triangle that a huge tolerance would collapse below 4 points
	// keeps its original points instead of disappearing.
	tri := [][2]float64{{0, 0}, {1, 0}, {0, 1}, {0, 0}}
	kept := simplifyRing(tri, 10)
	if len(kept) != len(tri) {
		t.Fatalf("a ring that would fall under 4 points must keep its original points, got %v", kept)
	}
	for i := range tri {
		if kept[i] != tri[i] {
			t.Fatalf("point %d changed: %v want %v", i, kept[i], tri[i])
		}
	}
}

func TestBuildFileMergesDuplicateCodes(t *testing.T) {
	raw, err := os.ReadFile("testdata/three.geojsonl")
	if err != nil {
		t.Fatal(err)
	}
	f, st, err := buildFile(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("buildFile: %v", err)
	}
	if st.Features != 3 {
		t.Errorf("features read: got %d want 3", st.Features)
	}
	if len(f.Districts) != 2 || st.Districts != 2 {
		t.Fatalf("two districts expected (one code appears twice), got %d", len(f.Districts))
	}
	a, b := f.Districts[0], f.Districts[1]
	if a.Code != 101 || b.Code != 202 {
		t.Fatalf("districts must be sorted by code, got %d then %d", a.Code, b.Code)
	}
	if a.Name != "Alpha" || a.StateCode != 1 || a.State != "Northland" {
		t.Errorf("district 101 fields wrong: %+v", a)
	}
	if b.Name != "Beta" || b.StateCode != 2 || b.State != "Southland" {
		t.Errorf("district 202 fields wrong (string numbers must parse): %+v", b)
	}
	if len(a.Polys) != 2 {
		t.Errorf("duplicated code must hold both polygons, got %d", len(a.Polys))
	}
	if len(a.Polys) > 0 && len(a.Polys[0]) != 2 {
		t.Errorf("first polygon of 101 has an outer ring and a hole, got %d rings", len(a.Polys[0]))
	}
	if len(b.Polys) != 2 {
		t.Errorf("multipolygon 202 should hold 2 polygons, got %d", len(b.Polys))
	}
	// The census code on the input lines (7, 8, 9) must never become a key.
	for _, d := range f.Districts {
		if d.Code < 100 {
			t.Errorf("district %q was keyed on the census code: %d", d.Name, d.Code)
		}
	}
}

func TestWriteFileIsReproducible(t *testing.T) {
	raw, err := os.ReadFile("testdata/three.geojsonl")
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	for _, w := range []*bytes.Buffer{&first, &second} {
		f, _, err := buildFile(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := writeFile(w, f); err != nil {
			t.Fatal(err)
		}
	}
	if first.Len() == 0 || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("two builds of the same input must be byte equal (%d vs %d bytes)", first.Len(), second.Len())
	}
	if first.Bytes()[0] != 0x1f || first.Bytes()[1] != 0x8b {
		t.Fatalf("output must be gzip")
	}
}

func TestBuildFileRejectsBadLines(t *testing.T) {
	cases := map[string]string{
		"not a feature": `{"type":"Point","coordinates":[1,2]}`,
		"bad geometry":  `{"type":"Feature","properties":{"dist_lgd":1,"dtname":"X","state_lgd":1,"stname":"Y"},"geometry":{"type":"Point","coordinates":[1,2]}}`,
		"not json":      `this is not json`,
	}
	for name, line := range cases {
		input := `{"type":"Feature","properties":{"dist_lgd":5,"dtname":"Ok","state_lgd":1,"stname":"Y"},"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1],[0,0]]]}}` + "\n" + line + "\n"
		_, _, err := buildFile(strings.NewReader(input))
		if err == nil {
			t.Errorf("%s: want an error", name)
			continue
		}
		if !strings.Contains(err.Error(), "line 2") {
			t.Errorf("%s: error must name the line number, got %q", name, err)
		}
	}
}

func TestBuildFileSkipsFeaturesWithoutACode(t *testing.T) {
	// The upstream file has two districts with dist_lgd 0 (areas with no LGD
	// code). Merged under code 0 they would be one wrongly named district that
	// no CAP message can ever name, so the builder leaves them out and counts them.
	ok := `{"type":"Feature","properties":{"dist_lgd":5,"dtname":"Ok","state_lgd":1,"stname":"Y"},"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1],[0,0]]]}}`
	zero := `{"type":"Feature","properties":{"dist_lgd":0,"dtname":"Nowhere","state_lgd":1,"stname":"Y"},"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1],[0,0]]]}}`
	f, st, err := buildFile(strings.NewReader(zero + "\n" + ok + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Districts) != 1 || f.Districts[0].Code != 5 {
		t.Fatalf("only the coded district should remain, got %+v", f.Districts)
	}
	if st.Features != 2 || st.NoCode != 1 {
		t.Errorf("stats: features %d (want 2), without a code %d (want 1)", st.Features, st.NoCode)
	}
}
