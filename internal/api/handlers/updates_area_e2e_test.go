// updates_area_e2e_test.go is quick 261119-sps (owner, 2026-11-19: "in those
// updates it should display the area name"), proven over the real router and a
// real Postgres: a bell row about a report with no stored name answers without
// waiting for the lookup, the lookup runs once in the background, and the next
// read carries the area on the row, in its label and on the Updates screen. Two
// reads at once look a report up once, a failed lookup is not retried, and no
// unauthenticated route ever calls the namer (Phase 8 D-06). It runs only on a
// disposable database, through newE2EServerWith like every e2e in this package.
package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
)

// spsNamer is a counting area namer. With a gate it blocks each call until the
// gate is closed; started gets one value per call.
type spsNamer struct {
	mu      sync.Mutex
	calls   int
	lats    []float64
	lons    []float64
	gate    chan struct{}
	started chan struct{}
	name    string
	err     error
}

func (n *spsNamer) ReverseArea(ctx context.Context, lat, lon float64) (string, error) {
	n.mu.Lock()
	n.calls++
	n.lats = append(n.lats, lat)
	n.lons = append(n.lons, lon)
	n.mu.Unlock()
	if n.started != nil {
		select {
		case n.started <- struct{}{}:
		default:
		}
	}
	if n.gate != nil {
		select {
		case <-n.gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return n.name, n.err
}

func (n *spsNamer) Calls() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.calls
}

// spsRow is a bell row with the area, as a client reads it.
type spsRow struct {
	Kind     string `json:"kind"`
	ReportID *int64 `json:"report_id"`
	Area     string `json:"area"`
	Category string `json:"category"`
	Severity string `json:"severity"`
	Label    string `json:"label"`
}

func spsPoll(t *testing.T, label string, client *http.Client, baseURL string) []spsRow {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/updates")
	if err != nil {
		t.Fatalf("%s: GET /api/updates: %v", label, err)
	}
	raw := p20Status(t, label, resp, http.StatusOK)
	var out struct {
		Updates []spsRow `json:"updates"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: decoding %s: %v", label, raw, err)
	}
	return out.Updates
}

// spsWait polls cond every 20ms until it holds or the deadline passes.
func spsWait(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// spsStored reads the stored area name and whether the lookup was tried.
func spsStored(t *testing.T, pool *pgxpool.Pool, id int64) (name *string, tried bool) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT r.area_name, (p.area_lookup_at IS NOT NULL)
		   FROM reports r LEFT JOIN push_report_progress p ON p.report_id = r.id
		  WHERE r.id = $1`, id).Scan(&name, &tried)
	if err != nil {
		t.Fatalf("reading the area of report %d: %v", id, err)
	}
	return name, tried
}

// spsSetup starts a server whose area namer is namer and makes report id Live,
// so the follower B has one reached_live row about it.
func spsSetup(t *testing.T, prefix string, namer *spsNamer) (baseURL string, pool *pgxpool.Pool, a, b *http.Client, id int64) {
	t.Helper()
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), func(d *api.Deps) { d.AreaNamer = namer }))
	a = newVerifiedClient(t, srv, mailer, prefix+"-a@example.com")
	b = newVerifiedClient(t, srv, mailer, prefix+"-b@example.com")
	c := newVerifiedClient(t, srv, mailer, prefix+"-c@example.com")
	d := newVerifiedClient(t, srv, mailer, prefix+"-d@example.com")
	id = submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	p32uLive(t, srv, b, c, d, id)
	return srv.URL, pool, a, b, id
}

func TestUpdatesAreaShowsOnTheNextRead(t *testing.T) {
	namer := &spsNamer{gate: make(chan struct{}), started: make(chan struct{}, 4), name: "  Benz Circle "}
	base, pool, _, b, id := spsSetup(t, "sps-next", namer)
	if namer.Calls() != 0 {
		t.Fatalf("the namer was called %d times before any row needed a name", namer.Calls())
	}

	// The first read answers with no area while the lookup is still blocked.
	rows := spsPoll(t, "first read", b, base)
	if len(rows) != 1 || rows[0].ReportID == nil || *rows[0].ReportID != id {
		t.Fatalf("first read rows = %+v, want one row about %d", rows, id)
	}
	if rows[0].Area != "" || !strings.Contains(rows[0].Label, "Flood, Low. ") {
		t.Errorf("first read row = %+v, want no area and the old label", rows[0])
	}
	select {
	case <-namer.started:
	case <-time.After(3 * time.Second):
		t.Fatal("no lookup started after the first read")
	}
	// A read while that lookup is blocked still answers, and starts no second
	// lookup of the same report.
	if again := spsPoll(t, "read while blocked", b, base); len(again) != 1 || again[0].Area != "" {
		t.Errorf("read while blocked = %+v", again)
	}
	close(namer.gate)
	spsWait(t, "the stored name", func() bool { n, _ := spsStored(t, pool, id); return n != nil })
	if n, _ := spsStored(t, pool, id); *n != "Benz Circle" {
		t.Errorf("stored name = %q, want it trimmed", *n)
	}

	rows = spsPoll(t, "next read", b, base)
	if len(rows) != 1 || rows[0].Area != "Benz Circle" {
		t.Fatalf("next read rows = %+v, want area Benz Circle", rows)
	}
	if !strings.Contains(rows[0].Label, "Flood, Low, Benz Circle. ") {
		t.Errorf("label = %q, want the area after the category and severity", rows[0].Label)
	}
	if namer.Calls() != 1 {
		t.Errorf("namer calls = %d, want 1", namer.Calls())
	}
	namer.mu.Lock()
	if namer.lats[0] != p20Lat || namer.lons[0] != p20Lon {
		t.Errorf("the lookup was sent %v,%v, want the report's own point", namer.lats[0], namer.lons[0])
	}
	namer.mu.Unlock()

	// The Updates screen draws the same line.
	resp, err := b.Get(base + "/profile?part=updates")
	if err != nil {
		t.Fatalf("GET the Updates screen: %v", err)
	}
	page := string(p20Status(t, "Updates screen", resp, http.StatusOK))
	if !strings.Contains(page, "Low &middot; Benz Circle</span>") && !strings.Contains(page, "Low · Benz Circle</span>") {
		t.Errorf("the Updates screen does not show the area on the type line")
	}
	if !strings.Contains(page, "Flood, Low, Benz Circle.") {
		t.Errorf("the Updates screen label does not hold the area")
	}
	time.Sleep(100 * time.Millisecond)
	if namer.Calls() != 1 {
		t.Errorf("namer calls after more reads = %d, want 1", namer.Calls())
	}
}

func TestUpdatesAreaTwoReadsAtOnceLookUpOnce(t *testing.T) {
	namer := &spsNamer{name: "Benz Circle"}
	base, pool, _, b, id := spsSetup(t, "sps-race", namer)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := b.Get(base + "/api/updates")
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()
	spsWait(t, "the stored name", func() bool { n, _ := spsStored(t, pool, id); return n != nil })
	time.Sleep(200 * time.Millisecond)
	if namer.Calls() != 1 {
		t.Errorf("namer calls = %d, want 1 for four reads at once", namer.Calls())
	}
}

func TestUpdatesAreaFailedLookupIsNotRetried(t *testing.T) {
	namer := &spsNamer{err: errors.New("lookup down")}
	base, pool, _, b, id := spsSetup(t, "sps-fail", namer)

	spsPoll(t, "first read", b, base)
	spsWait(t, "the tried mark", func() bool { _, tried := spsStored(t, pool, id); return tried && namer.Calls() == 1 })
	for i := 0; i < 3; i++ {
		if rows := spsPoll(t, "later read", b, base); len(rows) != 1 || rows[0].Area != "" {
			t.Errorf("later read rows = %+v, want no area", rows)
		}
	}
	time.Sleep(200 * time.Millisecond)
	if namer.Calls() != 1 {
		t.Errorf("namer calls = %d, want 1", namer.Calls())
	}
	if n, _ := spsStored(t, pool, id); n != nil {
		t.Errorf("a failed lookup stored %q", *n)
	}
}

// TestUnauthenticatedRoutesNeverCallTheNamer extends the Phase 8 D-06 guard
// (share_test.go "the public read never calls the area lookup") to the update
// rows' lookup: with a report that has no name, no request without a session
// (the bell, the Updates screen, the Demo, a public status page) calls it.
func TestUnauthenticatedRoutesNeverCallTheNamer(t *testing.T) {
	namer := &spsNamer{name: "Benz Circle"}
	srv, mailer, _ := newE2EServerWith(t, p20Tweak(newP20Clock(), func(d *api.Deps) { d.AreaNamer = namer }))
	poster := newVerifiedClient(t, srv, mailer, "sps-anon-a@example.com")
	submitReportGetID(t, poster, srv.URL, p20Body("flood", "low"))

	anon := newJarClient(t)
	for _, path := range []string{"/api/updates", "/profile?part=updates", "/profile", "/demo", "/r/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "/login"} {
		resp, err := anon.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
	}
	time.Sleep(200 * time.Millisecond)
	if namer.Calls() != 0 {
		t.Errorf("an unauthenticated route called the namer %d times", namer.Calls())
	}
}
