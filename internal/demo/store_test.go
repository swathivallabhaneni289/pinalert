package demo

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
)

// storeClock is a hand moved clock for the store tests.
type storeClock struct{ t time.Time }

func (c *storeClock) now() time.Time          { return c.t }
func (c *storeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestStore() (*Store, *storeClock) {
	c := &storeClock{t: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)}
	return NewStore(WithStoreClock(c.now)), c
}

func TestStoreCreateGet(t *testing.T) {
	s, _ := newTestStore()
	w, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	if len(w.ID) != 22 || !ValidVisitID(w.ID) {
		t.Fatalf("id %q is not a 22 character base64url visit id", w.ID)
	}
	got, ok := s.Get(w.ID)
	if !ok || got != w {
		t.Fatalf("Get(%q) = %v, %v; want the same world", w.ID, got, ok)
	}
	for _, bad := range []string{"", "short", "ABCDEFGHIJKLMNOPQRSTUV", w.ID + "x", "../" + w.ID[3:]} {
		if g, ok := s.Get(bad); ok || g != nil {
			t.Fatalf("Get(%q) returned a world", bad)
		}
	}
	other, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	if other.ID == w.ID {
		t.Fatalf("two worlds got the same id")
	}
}

func TestStoreWorldsShareNothing(t *testing.T) {
	s, _ := newTestStore()
	a, _ := s.Create()
	b, _ := s.Create()
	if _, err := a.Reports.SubmitChecked(context.Background(), a.SessionFor(VisitorAccountID), VisitorAccountID, service.SubmitInput{
		Category: service.CategoryFlood, Severity: service.SeverityLow, Description: "A report in world a only",
		Latitude: baseLat, Longitude: baseLon,
	}); err != nil {
		t.Fatal(err)
	}
	views, err := b.Reports.Nearby(context.Background(), service.NearbyQuery{Latitude: baseLat, Longitude: baseLon, RadiusKm: 5, ViewerAccountID: VisitorAccountID, All: true})
	if err != nil || len(views) != 0 {
		t.Fatalf("world b sees world a: views=%v err=%v", views, err)
	}
}

func TestStoreTTLAndLRU(t *testing.T) {
	s, c := newTestStore()

	// Idle drop: a world unused for the idle time is gone at the next Get.
	idle, _ := s.Create()
	c.advance(WorldIdleTTL - time.Second)
	if _, ok := s.Get(idle.ID); !ok {
		t.Fatalf("a world idle for just under the TTL was dropped")
	}
	c.advance(WorldIdleTTL + time.Second)
	if _, ok := s.Get(idle.ID); ok {
		t.Fatalf("a world idle past the TTL was kept")
	}

	// It is also gone at the next Create, which sweeps.
	old, _ := s.Create()
	c.advance(WorldIdleTTL + time.Minute)
	if _, err := s.Create(); err != nil {
		t.Fatal(err)
	}
	if s.Len() != 1 {
		t.Fatalf("Len = %d after the sweep at Create, want 1 (the new world)", s.Len())
	}
	if _, ok := s.Get(old.ID); ok {
		t.Fatalf("the idle world survived a Create sweep")
	}

	// LRU: fill to the ceiling, touch the oldest, add one more.
	s, c = newTestStore()
	var ids []string
	for i := 0; i < MaxWorlds; i++ {
		w, err := s.Create()
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, w.ID)
		c.advance(time.Second)
	}
	if s.Len() != MaxWorlds {
		t.Fatalf("Len = %d, want %d", s.Len(), MaxWorlds)
	}
	if _, ok := s.Get(ids[0]); !ok { // the oldest, now the most recently used
		t.Fatalf("the first world is missing")
	}
	c.advance(time.Second)
	newest, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	if s.Len() != MaxWorlds {
		t.Fatalf("Len = %d after the 101st Create, want %d", s.Len(), MaxWorlds)
	}
	if _, ok := s.Get(ids[1]); ok {
		t.Fatalf("the least recently used world (the second) survived")
	}
	if _, ok := s.Get(ids[0]); !ok {
		t.Fatalf("a world just read by Get was evicted")
	}
	if _, ok := s.Get(newest.ID); !ok {
		t.Fatalf("the new world is missing")
	}
}

func TestStoreWorldSetup(t *testing.T) {
	c := &storeClock{t: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)}
	calls := 0
	s := NewStore(WithStoreClock(c.now), WithWorldSetup(func(w *World) {
		calls++
		w.SetClockOffset(-time.Hour)
		w.SetClockOffset(0)
	}))
	if _, err := s.Create(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("setup ran %d times for one world", calls)
	}
}

func TestValidVisitID(t *testing.T) {
	good := strings.Repeat("a", 22)
	cases := map[string]bool{
		good:                     true,
		"ABCDEFGHIJKLMNOPQRSTUV": true,
		"abcdefghijklmnopqrstuv": true,
		"0123456789_-0123456789": true,
		"":                       false,
		strings.Repeat("a", 21):  false,
		strings.Repeat("a", 23):  false,
		"abcdefghijklmnopqrstu/": false,
		"abcdefghijklmnopqrstu.": false,
		"abcdefghijklmnopqrstu=": false,
		"abcdefghijklmnopqrstu ": false,
		"abcdefghijklmnopqrstué": false,
		"abcdefghijklmnopqrstéé": false,
	}
	for id, want := range cases {
		if got := ValidVisitID(id); got != want {
			t.Errorf("ValidVisitID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestStoreConstants(t *testing.T) {
	if MaxWorlds != 100 || WorldIdleTTL != 30*time.Minute || VisitIDBytes != 16 {
		t.Fatalf("caps changed: %d worlds, %v idle, %d id bytes", MaxWorlds, WorldIdleTTL, VisitIDBytes)
	}
}

// TestDemoPackageHasNoDatabase pins D-02 and D-05: a Demo world touches no
// database. It reads the package's non test Go files and fails if any imports
// a pgx package or database/sql.
func TestDemoPackageHasNoDatabase(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		checked++
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if path == "database/sql" || strings.HasPrefix(path, "github.com/jackc/pgx") {
				t.Errorf("%s imports %s: a Demo world must not reach a database", name, path)
			}
		}
	}
	if checked < 3 {
		t.Fatalf("only %d non test files were read, the guard is not looking at the package", checked)
	}
}
