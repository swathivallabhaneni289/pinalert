package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"image/jpeg"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Amended 2026-11-18 by quick 261118-v75: the 85 pictures now ship embedded in
// the binary, so the tests that pinned the NASA fetch (fetched once, refusing a
// bad upstream, the size limit of an upstream body, the 60 second failure
// memory and the NASA constant) are gone with the code they tested. In their
// place: every one of the 85 serves its embedded bytes, the handler cannot reach
// the network, and the picture folder holds exactly the 85 JPEGs and the repo no
// other raster image.

func globeServe(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func mustGlobeTiles(t *testing.T) *GlobeTiles {
	t.Helper()
	g, err := NewGlobeTiles()
	if err != nil {
		t.Fatalf("NewGlobeTiles: %v", err)
	}
	return g
}

func TestGlobeTilePaths(t *testing.T) {
	paths := GlobeTilePaths()
	if len(paths) != 85 {
		t.Fatalf("GlobeTilePaths has %d entries, want 85", len(paths))
	}
	shape := regexp.MustCompile(`^/globe/tiles/([0-3])/([0-9]+)/([0-9]+)\.jpg$`)
	perZoom := map[string]int{}
	seen := map[string]bool{}
	for _, p := range paths {
		m := shape.FindStringSubmatch(p)
		if m == nil {
			t.Fatalf("%q does not match the tile path shape", p)
		}
		if seen[p] {
			t.Fatalf("%q is listed twice", p)
		}
		seen[p] = true
		perZoom[m[1]]++
		var z, y, x int
		if _, err := fmt.Sscanf(m[1]+" "+m[2]+" "+m[3], "%d %d %d", &z, &y, &x); err != nil {
			t.Fatalf("parsing %q: %v", p, err)
		}
		if y >= 1<<z || x >= 1<<z {
			t.Fatalf("%q has an index out of range for zoom %d", p, z)
		}
	}
	want := map[string]int{"0": 1, "1": 4, "2": 16, "3": 64}
	for z, n := range want {
		if perZoom[z] != n {
			t.Errorf("zoom %s has %d paths, want %d", z, perZoom[z], n)
		}
	}

	g := mustGlobeTiles(t)
	for _, p := range paths {
		if _, ok := g.Handler(p); !ok {
			t.Errorf("Handler(%q) = false, want true for a listed path", p)
		}
	}
	for _, p := range []string{
		"/globe/tiles/4/0/0.jpg",
		"/globe/tiles/1/2/0.jpg",
		"/globe/tiles/1/0/2.jpg",
		"/globe/tiles/0/0/0.png",
		"/globe/tiles/0/0/0.jpeg",
		"/globe/tiles/0/0/0.jpg/",
		"/globe/tiles/00/0/0.jpg",
		"/globe/tiles/0/0/../0.jpg",
		"/globe/tiles",
		"",
	} {
		if h, ok := g.Handler(p); ok || h != nil {
			t.Errorf("Handler(%q) = true, want false for a path outside the list", p)
		}
	}
}

// TestGlobeTilesServeEmbeddedBytes: every one of the 85 answers 200 with
// image/jpeg, the month long cache header, nosniff, no Set-Cookie, and exactly
// the bytes of its embedded file.
func TestGlobeTilesServeEmbeddedBytes(t *testing.T) {
	g := mustGlobeTiles(t)
	for _, p := range GlobeTilePaths() {
		h, ok := g.Handler(p)
		if !ok {
			t.Fatalf("no handler for %s", p)
		}
		rec := globeServe(h, p)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, rec.Code)
			continue
		}
		for k, want := range map[string]string{
			"Content-Type":           "image/jpeg",
			"Cache-Control":          "public, max-age=2592000",
			"X-Content-Type-Options": "nosniff",
		} {
			if got := rec.Header().Get(k); got != want {
				t.Errorf("GET %s: %s = %q, want %q", p, k, got, want)
			}
		}
		if got := rec.Header().Values("Set-Cookie"); len(got) != 0 {
			t.Errorf("GET %s: Set-Cookie = %v, want none", p, got)
		}
		want, err := globeTileFS.ReadFile(globeTileFile(globeTiles[p]))
		if err != nil {
			t.Fatalf("reading the embedded file of %s: %v", p, err)
		}
		if !bytes.Equal(rec.Body.Bytes(), want) {
			t.Errorf("GET %s: the body is not the embedded file", p)
		}
	}
	if strings.Contains(GlobeCacheControl, "no-store") {
		t.Errorf("GlobeCacheControl must not carry no-store")
	}
}

// failingTransport fails the test on any outbound call.
type failingTransport struct{ t *testing.T }

func (f failingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.t.Errorf("the globe handler made an outbound request to %s", r.URL)
	return nil, errors.New("no network in this test")
}

// TestGlobeTilesNeverReachTheNetwork proves the server never contacts NASA, two
// ways. At run time, Go's default transport is swapped for one that fails the
// test, and all 85 are built and served. In the source, globe.go is parsed and
// its code (comments excluded) may import nothing that dials, may not name an
// HTTP client, a request builder or a transport, and may hold no URL literal.
func TestGlobeTilesNeverReachTheNetwork(t *testing.T) {
	saved := http.DefaultTransport
	http.DefaultTransport = failingTransport{t}
	t.Cleanup(func() { http.DefaultTransport = saved })

	g := mustGlobeTiles(t)
	for _, p := range GlobeTilePaths() {
		h, _ := g.Handler(p)
		if rec := globeServe(h, p); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, rec.Code)
		}
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "globe.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing globe.go: %v", err)
	}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		switch path {
		case "net", "net/url", "net/http/httputil", "crypto/tls", "io", "mime", "context", "sync", "time":
			t.Errorf("globe.go imports %q, which only a network fetch needs", path)
		}
	}
	banned := map[string]bool{
		"Client": true, "Transport": true, "DefaultClient": true, "DefaultTransport": true,
		"NewRequest": true, "NewRequestWithContext": true, "Get": true, "Head": true, "Post": true,
		"Do": true, "Dial": true, "DialContext": true,
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := v.X.(*ast.Ident); ok && id.Name == "http" && banned[v.Sel.Name] {
				t.Errorf("globe.go uses http.%s at %s", v.Sel.Name, fset.Position(v.Pos()))
			}
			if v.Sel.Name == "Do" {
				t.Errorf("globe.go calls .Do at %s", fset.Position(v.Pos()))
			}
		case *ast.BasicLit:
			if v.Kind == token.STRING && (strings.Contains(v.Value, "://") || strings.Contains(strings.ToLower(v.Value), "nasa.gov")) {
				t.Errorf("globe.go holds the address literal %s at %s", v.Value, fset.Position(v.Pos()))
			}
		}
		return true
	})
}

// TestGlobeTileFolderContract: the picture folder holds exactly the 85 files
// {z}/{y}/{x}.jpg for zoom 0 to 3 and nothing else, each a 256 by 256 JPEG
// starting with FF D8 FF and under 1 MB. And no other raster image sits in the
// repo's source tree: among the files git tracks (git ls-files at the module
// root), skipping any path with a part that starts with a dot (.github,
// .planning and .claude), the only raster files are these 85. Untracked
// folders in a checkout, such as local design references, do not count, so
// the test reads the same on the Mac as on a fresh clone. Without git, or
// outside a git work tree, that half is skipped.
func TestGlobeTileFolderContract(t *testing.T) {
	want := map[string]bool{}
	for z := 0; z <= GlobeMaxZoom; z++ {
		for y := 0; y < 1<<z; y++ {
			for x := 0; x < 1<<z; x++ {
				want[fmt.Sprintf("%d/%d/%d.jpg", z, y, x)] = true
			}
		}
	}
	if len(want) != 85 {
		t.Fatalf("built %d expected names, want 85", len(want))
	}

	found := 0
	err := filepath.WalkDir("globetiles", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel("globetiles", path)
		rel = filepath.ToSlash(rel)
		if !want[rel] {
			t.Errorf("globetiles/%s is not one of the 85 tiles", rel)
			return nil
		}
		found++
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("reading %s: %v", path, err)
			return nil
		}
		if len(body) == 0 || len(body) >= 1<<20 {
			t.Errorf("%s is %d bytes, want more than 0 and under 1 MB", path, len(body))
		}
		if !bytes.HasPrefix(body, []byte{0xff, 0xd8, 0xff}) {
			t.Errorf("%s does not start with the JPEG bytes FF D8 FF", path)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(body))
		if err != nil || cfg.Width != 256 || cfg.Height != 256 {
			t.Errorf("%s: decode %v, size %d by %d, want a 256 by 256 JPEG", path, err, cfg.Width, cfg.Height)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking globetiles: %v", err)
	}
	if found != 85 {
		t.Errorf("globetiles holds %d of the 85 tiles", found)
	}

	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("finding the module root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("%s is not the module root: %v", root, err)
	}
	tilesAbs, err := filepath.Abs("globetiles")
	if err != nil {
		t.Fatalf("finding globetiles: %v", err)
	}
	tilesRel, err := filepath.Rel(root, tilesAbs)
	if err != nil {
		t.Fatalf("placing globetiles under the module root: %v", err)
	}
	tilesPrefix := filepath.ToSlash(tilesRel) + "/"

	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git is not installed, so the tracked file scan is skipped: %v", err)
	}
	inside := exec.Command(gitBin, "-C", root, "rev-parse", "--is-inside-work-tree")
	if out, err := inside.Output(); err != nil || strings.TrimSpace(string(out)) != "true" {
		t.Skipf("%s is not a git work tree, so the tracked file scan is skipped", root)
	}
	list, err := exec.Command(gitBin, "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("listing the tracked files: %v", err)
	}
	raster := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".bmp", ".avif", ".heic", ".tif", ".tiff"}
	tracked := 0
	for _, rel := range strings.Split(string(list), "\x00") {
		if rel == "" {
			continue
		}
		tracked++
		if strings.HasPrefix(rel, tilesPrefix) {
			continue
		}
		hidden := false
		for _, part := range strings.Split(rel, "/") {
			if strings.HasPrefix(part, ".") || part == "node_modules" {
				hidden = true
				break
			}
		}
		if hidden {
			continue
		}
		name := strings.ToLower(rel)
		for _, ext := range raster {
			if strings.HasSuffix(name, ext) {
				t.Errorf("%s: the only raster images allowed in the repo are the 85 globe tiles", rel)
			}
		}
	}
	if tracked == 0 {
		t.Fatalf("git ls-files at %s listed no files", root)
	}
}
