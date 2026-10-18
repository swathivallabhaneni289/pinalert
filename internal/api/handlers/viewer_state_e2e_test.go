// viewer_state_e2e_test.go is plan 03.2-05: viewer_follows and viewer_muted on
// the Nearby feed and on the report link answer, proven over the real router
// and a real Postgres for three different readers. It runs only on a
// disposable database (p32_05), through newE2EServerWith like every e2e here.
package handlers_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"pinalert/internal/testutil"
)

// p32ViewerFeed reads GET /api/reports as client with view=all and the report
// link parameter for focusID. It returns the raw object of every feed report
// and the raw focus report object.
func p32ViewerFeed(t *testing.T, label string, client *http.Client, baseURL string, focusID int64) (map[int64]map[string]json.RawMessage, map[string]json.RawMessage) {
	t.Helper()
	url := baseURL + "/api/reports?lat=" + strconv.FormatFloat(p20Lat, 'f', -1, 64) +
		"&lon=" + strconv.FormatFloat(p20Lon, 'f', -1, 64) + "&radius_km=5&view=all&report=" + strconv.FormatInt(focusID, 10)
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("%s: GET %s: %v", label, url, err)
	}
	raw := p20Status(t, label+": GET /api/reports", resp, http.StatusOK)
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("%s: Cache-Control = %q, want no-store (T-03.2-20)", label, cc)
	}
	var out struct {
		Reports []map[string]json.RawMessage `json:"reports"`
		Focus   struct {
			Status string                     `json:"status"`
			Report map[string]json.RawMessage `json:"report"`
		} `json:"focus"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: decoding the feed %s: %v", label, raw, err)
	}
	if out.Focus.Status != "ok" || out.Focus.Report == nil {
		t.Fatalf("%s: focus = %+v, want status ok with a report (body %s)", label, out.Focus, raw)
	}
	objects := map[int64]map[string]json.RawMessage{}
	for _, obj := range out.Reports {
		var id int64
		if err := json.Unmarshal(obj["id"], &id); err != nil {
			t.Fatalf("%s: decoding a feed element id: %v", label, err)
		}
		objects[id] = obj
	}
	return objects, out.Focus.Report
}

// p32Bool requires key to be a JSON boolean on obj and returns it. It also
// bans any key other than viewer_flagged that names a flag (the flag e2e rule).
func p32Bool(t *testing.T, label string, obj map[string]json.RawMessage, key string) bool {
	t.Helper()
	raw, ok := obj[key]
	if !ok {
		t.Fatalf("%s: no %s key", label, key)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("%s: %s = %s, want a JSON boolean", label, key, raw)
	}
	for k := range obj {
		if k != "viewer_flagged" && strings.Contains(strings.ToLower(k), "flag") {
			t.Errorf("%s: carries the extra key %q", label, k)
		}
	}
	return b
}

// TestViewerStateOnTheWire: A posts a report, B confirms it, C only reads, and
// B has muted it. Each reader sees only their own state, on the feed and on the
// report link (T-03.2-02).
func TestViewerStateOnTheWire(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	a := newVerifiedClient(t, srv, mailer, "p32-state-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p32-state-b@example.com")
	c := newVerifiedClient(t, srv, mailer, "p32-state-c@example.com")
	idB := p20AccountID(t, pool, "p32-state-b@example.com")

	id := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	status, body, err := p07PostVoteRaw(b, srv.URL, id, "confirm", p20Lat, p20Lon)
	if err != nil || status != http.StatusOK {
		t.Fatalf("B confirm: status %d, err %v, body %s", status, err, body)
	}
	testutil.MustExec(t, pool, `INSERT INTO report_mutes (account_id, report_id) VALUES ($1, $2)`, idB, id)

	cases := []struct {
		name           string
		client         *http.Client
		follows, muted bool
	}{
		{"A posted it", a, true, false},
		{"B confirmed and muted it", b, true, true},
		{"C only reads", c, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			objects, focus := p32ViewerFeed(t, tc.name, tc.client, srv.URL, id)
			obj, ok := objects[id]
			if !ok {
				t.Fatalf("report %d missing from the feed", id)
			}
			for where, o := range map[string]map[string]json.RawMessage{"feed": obj, "focus": focus} {
				label := tc.name + " (" + where + ")"
				if got := p32Bool(t, label, o, "viewer_follows"); got != tc.follows {
					t.Errorf("%s: viewer_follows = %v, want %v", label, got, tc.follows)
				}
				if got := p32Bool(t, label, o, "viewer_muted"); got != tc.muted {
					t.Errorf("%s: viewer_muted = %v, want %v", label, got, tc.muted)
				}
				p32Bool(t, label, o, "viewer_flagged")
			}
		})
	}

	// Every element of the feed carries both keys as booleans.
	objects, _ := p32ViewerFeed(t, "C every element", c, srv.URL, id)
	for _, obj := range objects {
		p32Bool(t, "C feed report", obj, "viewer_follows")
		p32Bool(t, "C feed report", obj, "viewer_muted")
	}
}
