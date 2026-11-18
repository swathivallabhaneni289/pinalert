// This file serves the Earth pictures of the sign in globe (Phase 5, plan 04,
// decision D-13; quick 261118-v75). The globe is drawn from the NASA Blue Marble
// layer, which has 85 small pictures down to zoom 3. The 85 pictures ship with
// Pinalert in the globetiles folder beside this file, are built into the binary
// with go:embed, and are served from Pinalert's own address
// /globe/tiles/{z}/{y}/{x}.jpg. The server never contacts NASA: the visitor's
// address and browser details never reach NASA, the first visitor after a start
// waits for nothing, and a NASA outage does not matter.
//
// Where the pictures came from: the NASA GIBS layer BlueMarble_NextGeneration,
// downloaded once on 2026-11-18 from
//
//	https://gibs.earthdata.nasa.gov/wmts/epsg3857/best/BlueMarble_NextGeneration/default/GoogleMapsCompatible_Level8/{z}/{y}/{x}.jpeg
//
// for zoom 0 to 3 and every row y and column x from 0 to 2^z-1, and saved as
// globetiles/{z}/{y}/{x}.jpg (about 0.75 MB in all). NASA's data use policy says
// its material is CC0 unless marked, may be reproduced and distributed without
// further permission, and that NASA should be acknowledged as the source; the
// Policy page and the credit line under the globe acknowledge it. To fetch them
// again, ask the same address for the same 85 tiles (with curl, for example),
// check each starts with the bytes FF D8 FF, is 256 by 256 and is under 1 MB, and
// replace the files. These 85 are the only raster pictures in the project outside
// .planning, an exception the owner approved on 2026-11-18.
//
// The route fails closed. Only the 85 valid indices exist, and anything else is
// not a route. The router registers these routes and builds its session bypass
// from the one list, GlobeTilePaths, so the two cannot drift apart. A tile
// request carries no session: nothing here reads or writes an account, a session
// or the database.

package handlers

import (
	"bytes"
	"embed"
	"fmt"
	"net/http"
)

// globeTileFS holds the 85 pictures, at globetiles/{z}/{y}/{x}.jpg.
//
//go:embed globetiles
var globeTileFS embed.FS

const (
	// GlobeMaxZoom is the deepest zoom the globe uses. Zoom 3 is 85 tiles in all.
	GlobeMaxZoom = 3
	// GlobeTileMaxBytes is the most one picture may hold.
	GlobeTileMaxBytes = 1 << 20
	// GlobeCacheControl lets a browser keep a tile for a month.
	GlobeCacheControl = "public, max-age=2592000"
)

// globeTilePrefix and globeTileSuffix frame the served address.
const (
	globeTilePrefix = "/globe/tiles/"
	globeTileSuffix = ".jpg"
)

// globeTile is one entry of the closed set.
type globeTile struct{ z, y, x int }

// globeTiles is the closed set, keyed by served path, built once.
var globeTiles = buildGlobeTiles()

func buildGlobeTiles() map[string]globeTile {
	set := make(map[string]globeTile, 85)
	for z := 0; z <= GlobeMaxZoom; z++ {
		for y := 0; y < 1<<z; y++ {
			for x := 0; x < 1<<z; x++ {
				set[fmt.Sprintf("%s%d/%d/%d%s", globeTilePrefix, z, y, x, globeTileSuffix)] = globeTile{z: z, y: y, x: x}
			}
		}
	}
	return set
}

// globeTileFile is where the picture of a tile sits in globeTileFS.
func globeTileFile(t globeTile) string {
	return fmt.Sprintf("globetiles/%d/%d/%d.jpg", t.z, t.y, t.x)
}

// GlobeTilePaths is the one list of the 85 addresses /globe/tiles/{z}/{y}/{x}.jpg
// for zoom 0 to 3. The router registers one exact GET route for each entry and
// skips the session middleware for exactly these paths. It returns a fresh
// slice, in zoom, row, column order.
func GlobeTilePaths() []string {
	paths := make([]string, 0, len(globeTiles))
	for z := 0; z <= GlobeMaxZoom; z++ {
		for y := 0; y < 1<<z; y++ {
			for x := 0; x < 1<<z; x++ {
				paths = append(paths, fmt.Sprintf("%s%d/%d/%d%s", globeTilePrefix, z, y, x, globeTileSuffix))
			}
		}
	}
	return paths
}

// GlobeTiles serves the 85 embedded tiles. Safe for concurrent use: it is only
// read after NewGlobeTiles returns.
type GlobeTiles struct {
	bodies map[string][]byte // served path -> picture bytes
}

// NewGlobeTiles reads the 85 embedded pictures once. It fails only on a build
// defect: a picture missing from the embed, empty, over GlobeTileMaxBytes, or
// not starting with the JPEG bytes FF D8 FF.
func NewGlobeTiles() (*GlobeTiles, error) {
	bodies := make(map[string][]byte, len(globeTiles))
	for path, tile := range globeTiles {
		name := globeTileFile(tile)
		body, err := globeTileFS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("globe tile %s: %w", name, err)
		}
		if len(body) == 0 || len(body) > GlobeTileMaxBytes || !bytes.HasPrefix(body, []byte{0xff, 0xd8, 0xff}) {
			return nil, fmt.Errorf("globe tile %s is not a JPEG of at most %d bytes", name, GlobeTileMaxBytes)
		}
		bodies[path] = body
	}
	return &GlobeTiles{bodies: bodies}, nil
}

// Handler returns the handler for one entry of GlobeTilePaths, and false for
// any other path. The router registers it for GET only.
func (g *GlobeTiles) Handler(path string) (http.Handler, bool) {
	body, ok := g.bodies[path]
	if !ok {
		return nil, false
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h := w.Header()
		h.Set("Content-Type", "image/jpeg")
		h.Set("Cache-Control", GlobeCacheControl)
		h.Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}), true
}
