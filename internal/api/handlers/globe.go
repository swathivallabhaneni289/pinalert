// This file serves the Earth pictures of the sign in globe (Phase 5, plan 04,
// decision D-13). The globe is drawn from the NASA Blue Marble layer, which has
// 85 small pictures down to zoom 3. Pinalert fetches each one once from the
// NASA GIBS host and keeps it in memory, then serves it from its own address
// /globe/tiles/{z}/{y}/{x}.jpg. Why a copy:
//
//   - a repeat visit downloads nothing, because the answer is cacheable for a
//     month;
//   - the visitor's address and browser details never reach NASA, only this
//     server's does, and only 85 times at most per start;
//   - a NASA outage after the first fetch does not matter.
//
// NASA's data use policy says its material is CC0 unless marked, may be
// reproduced and distributed without further permission, and that NASA should
// be acknowledged as the source. The no-store header GIBS sends is a cache
// directive, not a licence term. The copy lives only in memory: nothing is ever
// written to the disk and no picture is ever added to the repo. The credit line
// and the Policy sentence that acknowledge NASA are plans 05-07 and 05-20.
//
// The route fails closed. Only the 85 valid indices exist; the upstream host is
// a constant (a test may pass another base through GlobeTilesConfig.Upstream,
// production never does); redirects are refused; the body is limited to 1 MB
// and must be an image/jpeg; one fetch per tile runs at a time; a failed fetch
// is remembered for 60 seconds so a broken upstream is not hammered; and a
// failure answers 404, which the globe treats like any tile failure and gives
// up quietly.
//
// The router registers these routes and builds its session bypass from the one
// list, GlobeTilePaths, so the two cannot drift apart. A tile request carries no
// session: nothing here reads or writes an account, a session or the database.

package handlers

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
)

// The only upstream: the NASA GIBS Blue Marble layer, tile rows and columns
// appended as /{z}/{y}/{x}.jpeg. It is a constant so this route can never become
// an open proxy (T-05-17): no part of a request is ever used as a host, and the
// path is built from the validated numbers of the 85 listed tiles.
const globeUpstreamBase = "https://gibs.earthdata.nasa.gov/wmts/epsg3857/best/BlueMarble_NextGeneration/default/GoogleMapsCompatible_Level8"

const (
	// GlobeMaxZoom is the deepest zoom the globe uses. Zoom 3 is 85 tiles in all.
	GlobeMaxZoom = 3
	// GlobeTileMaxBytes is the most one upstream body may hold.
	GlobeTileMaxBytes = 1 << 20
	// GlobeFetchTimeout bounds one upstream fetch.
	GlobeFetchTimeout = 10 * time.Second
	// GlobeFailureMemory is how long a failed fetch is remembered before the
	// upstream is asked again.
	GlobeFailureMemory = 60 * time.Second
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

// GlobeTilesConfig configures NewGlobeTiles. Every field has a working zero
// value.
type GlobeTilesConfig struct {
	// Upstream replaces the NASA base address. Only tests set it. Empty means
	// the constant NASA address.
	Upstream string
	// UserAgent is sent to the upstream, so the host can tell who is asking.
	UserAgent string
	// Client is the HTTP client. Whatever is passed, redirects are refused and
	// a timeout is set.
	Client *http.Client
}

// GlobeTiles serves the 85 tiles from a memory copy. Safe for concurrent use.
type GlobeTiles struct {
	upstream  string
	userAgent string
	client    *http.Client
	now       func() time.Time // a test seam

	bodies sync.Map // served path -> []byte, successes only

	mu       sync.Mutex
	inflight map[string]*globeCall
	failed   map[string]time.Time
}

// globeCall is one upstream fetch other callers of the same tile wait for.
type globeCall struct {
	done chan struct{}
	body []byte // nil when the fetch failed
}

// NewGlobeTiles builds the handler set. It never fetches anything itself; the
// first request for a tile does.
func NewGlobeTiles(cfg GlobeTilesConfig) *GlobeTiles {
	upstream := strings.TrimRight(cfg.Upstream, "/")
	if upstream == "" {
		upstream = globeUpstreamBase
	}
	var client http.Client
	if cfg.Client != nil {
		client = *cfg.Client
	}
	if client.Timeout == 0 {
		client.Timeout = GlobeFetchTimeout
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &GlobeTiles{
		upstream:  upstream,
		userAgent: cfg.UserAgent,
		client:    &client,
		now:       time.Now,
		inflight:  map[string]*globeCall{},
		failed:    map[string]time.Time{},
	}
}

// Handler returns the handler for one entry of GlobeTilePaths, and false for
// any other path. The router registers it for GET only.
func (g *GlobeTiles) Handler(path string) (http.Handler, bool) {
	tile, ok := globeTiles[path]
	if !ok {
		return nil, false
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := g.get(r.Context(), path, tile)
		h := w.Header()
		if body == nil {
			// Not cacheable, so a browser asks again once the upstream is back.
			h.Set("Cache-Control", "no-store")
			h.Set("X-Content-Type-Options", "nosniff")
			http.NotFound(w, r)
			return
		}
		h.Set("Content-Type", "image/jpeg")
		h.Set("Cache-Control", GlobeCacheControl)
		h.Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}), true
}

// get returns the tile's bytes, or nil when it cannot be had. At most one fetch
// of a tile runs at a time; later callers wait for its answer.
func (g *GlobeTiles) get(ctx context.Context, path string, tile globeTile) []byte {
	if v, ok := g.bodies.Load(path); ok {
		return v.([]byte)
	}

	g.mu.Lock()
	// Checked again under the lock, so a caller that arrives just after a fetch
	// finished never starts a second one.
	if v, ok := g.bodies.Load(path); ok {
		g.mu.Unlock()
		return v.([]byte)
	}
	if at, bad := g.failed[path]; bad {
		if g.now().Sub(at) < GlobeFailureMemory {
			g.mu.Unlock()
			return nil
		}
		delete(g.failed, path)
	}
	if call, running := g.inflight[path]; running {
		g.mu.Unlock()
		select {
		case <-call.done:
			return call.body
		case <-ctx.Done():
			return nil
		}
	}
	call := &globeCall{done: make(chan struct{})}
	g.inflight[path] = call
	g.mu.Unlock()

	// The fetch does not use the first caller's context: if that caller goes
	// away, the others still get their picture.
	body := g.fetch(tile)

	g.mu.Lock()
	if body != nil {
		g.bodies.Store(path, body)
	} else {
		g.failed[path] = g.now()
	}
	delete(g.inflight, path)
	call.body = body
	close(call.done)
	g.mu.Unlock()
	return body
}

// fetch asks the upstream for one tile and returns its bytes, or nil on any
// failure: a transport error, a status other than 200, a redirect, a type other
// than image/jpeg, an empty body or one over GlobeTileMaxBytes.
func (g *GlobeTiles) fetch(tile globeTile) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), GlobeFetchTimeout)
	defer cancel()
	url := fmt.Sprintf("%s/%d/%d/%d.jpeg", g.upstream, tile.z, tile.y, tile.x)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	if g.userAgent != "" {
		req.Header.Set("User-Agent", g.userAgent)
	}
	req.Header.Set("Accept", "image/jpeg")
	resp, err := g.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || mt != "image/jpeg" {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, GlobeTileMaxBytes+1))
	if err != nil || len(body) == 0 || len(body) > GlobeTileMaxBytes {
		return nil
	}
	return body
}
