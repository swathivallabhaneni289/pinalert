// timeline_e2e_test.go proves plan 08-01 (PAGE-01, SC2) end to end over the real
// router and a real Postgres: the report history is served from the vote log to
// a signed in reader, names no voter, account or cell, and answers one 404 for
// every report the reader may not open. It reuses the p07 trust server and the
// 7.1 sign in helpers by name. Every package level identifier added here
// carries the p801 prefix. The database tests run only on the disposable
// database p8_01_a.
package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/api/handlers"
)

// The three voter points. Each is a few hundred metres from the report at
// (p07ReportLat, p07ReportLon), well inside the one kilometre vote radius, and
// in a cell of its own at the voter precision.
var p801Points = [][2]float64{
	{12.9746, 77.5946},
	{12.9716, 77.5976},
	{12.9686, 77.5946},
}

// p801GetTimeline reads the history of reportID as client and returns the status
// and the raw body.
func p801GetTimeline(t *testing.T, client *http.Client, baseURL string, reportID int64) (int, []byte) {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/reports/" + strconv.FormatInt(reportID, 10) + "/timeline")
	if err != nil {
		t.Fatalf("GET timeline of report %d: %v", reportID, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading timeline body: %v", err)
	}
	if got := resp.Header.Get("Cache-Control"); resp.StatusCode == http.StatusOK && got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	return resp.StatusCode, body
}

// p801Vote casts one vote from point i and requires 200.
func p801Vote(t *testing.T, client *http.Client, baseURL string, reportID int64, action string, point int) {
	t.Helper()
	status, body, err := p07PostVoteRaw(client, baseURL, reportID, action, p801Points[point][0], p801Points[point][1])
	if err != nil || status != http.StatusOK {
		t.Fatalf("%s vote from point %d: status %d, err %v, body %s", action, point, status, err, body)
	}
}

// p801Decode reads a timeline body.
func p801Decode(t *testing.T, raw []byte) handlers.TimelineResponse {
	t.Helper()
	var out handlers.TimelineResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding timeline %s: %v", raw, err)
	}
	return out
}

func TestTimelineForViewer(t *testing.T) {
	srv, mailer, _ := newTrustE2EServer(t, p07FixedClock())
	a := newVerifiedClient(t, srv, mailer, "p801-view-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p801-view-b@example.com")
	c := newVerifiedClient(t, srv, mailer, "p801-view-c@example.com")
	d := newVerifiedClient(t, srv, mailer, "p801-view-d@example.com")

	reportID := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	p801Vote(t, b, srv.URL, reportID, "confirm", 0)
	p801Vote(t, c, srv.URL, reportID, "confirm", 1)
	p801Vote(t, d, srv.URL, reportID, "dispute", 2)

	status, raw := p801GetTimeline(t, b, srv.URL, reportID)
	if status != http.StatusOK {
		t.Fatalf("viewer b: status %d, body %s", status, raw)
	}
	got := p801Decode(t, raw)
	if len(got.Events) == 0 || got.Events[0].Kind != "posted" || got.Events[0].Text != "Posted as Low." {
		t.Fatalf("first event = %+v, want the posted line", got.Events)
	}

	want := []string{
		"Posted as Low.",
		"Confirmed by someone nearby.",
		"You confirmed it.",
		"Confirmed from 2 spots. Shown to everyone.",
		"Disputed by someone nearby.",
	}
	var texts []string
	var yours []string
	var last time.Time
	for _, e := range got.Events {
		texts = append(texts, e.Text)
		at, err := time.Parse(time.RFC3339, e.At)
		if err != nil {
			t.Fatalf("at %q is not RFC 3339: %v", e.At, err)
		}
		if at.Before(last) {
			t.Errorf("events are not in time order: %v after %v", at, last)
		}
		last = at
		if e.Yours {
			yours = append(yours, e.Text)
		}
		if e.Tone != "good" && e.Tone != "neutral" && e.Tone != "bad" {
			t.Errorf("tone %q is not good, neutral or bad", e.Tone)
		}
	}
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Errorf("texts\n got %q\nwant %q", texts, want)
	}
	if len(yours) != 1 || yours[0] != "You confirmed it." {
		t.Errorf("viewer b yours lines = %q", yours)
	}

	// The reporter has no yours line, and everyone sees the same public lines.
	status, raw = p801GetTimeline(t, a, srv.URL, reportID)
	if status != http.StatusOK {
		t.Fatalf("reporter: status %d, body %s", status, raw)
	}
	for _, e := range p801Decode(t, raw).Events {
		if e.Yours {
			t.Errorf("reporter got a yours line %+v", e)
		}
	}

	// The disputer sees their own line.
	status, raw = p801GetTimeline(t, d, srv.URL, reportID)
	if status != http.StatusOK || !strings.Contains(string(raw), "You disputed it.") {
		t.Errorf("viewer d: status %d, body %s", status, raw)
	}
}

func TestTimelineResponseOmitsIdentifiers(t *testing.T) {
	srv, mailer, _ := newTrustE2EServer(t, p07FixedClock())
	const reporterEmail = "p801-omit-reporter@example.com"
	const voterEmail = "p801-omit-voter@example.com"
	a := newVerifiedClient(t, srv, mailer, reporterEmail)
	b := newVerifiedClient(t, srv, mailer, voterEmail)
	c := newVerifiedClient(t, srv, mailer, "p801-omit-voter2@example.com")

	reportID := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	p801Vote(t, b, srv.URL, reportID, "confirm", 0)
	p801Vote(t, c, srv.URL, reportID, "confirm", 1)

	voterCell := geohash.EncodeWithPrecision(p801Points[0][0], p801Points[0][1], voterGeohashPrecision)
	sessionA := p07SessionID(t, a, srv.URL)
	sessionB := p07SessionID(t, b, srv.URL)

	allowed := map[string]bool{"events": true, "kind": true, "tone": true, "text": true, "at": true, "yours": true}
	banned := []string{"account_id", "account", "email", "session", "session_id", "geohash", "geohash_cell", "cell", "latitude", "longitude"}

	for label, client := range map[string]*http.Client{"reporter": a, "voter": b} {
		status, raw := p801GetTimeline(t, client, srv.URL, reportID)
		if status != http.StatusOK {
			t.Fatalf("%s: status %d, body %s", label, status, raw)
		}
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("%s: not JSON: %v", label, err)
		}
		p07WalkJSONKeys(t, label, decoded)
		var walk func(v any)
		walk = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				for k, val := range x {
					if !allowed[k] {
						t.Errorf("%s: unexpected key %q", label, k)
					}
					for _, b := range banned {
						if k == b {
							t.Errorf("%s: banned key %q", label, k)
						}
					}
					walk(val)
				}
			case []any:
				for _, val := range x {
					walk(val)
				}
			}
		}
		walk(decoded)
		text := string(raw)
		for _, s := range []string{reporterEmail, voterEmail, "@", voterCell, sessionA, sessionB} {
			if strings.Contains(text, s) {
				t.Errorf("%s: body contains %q: %s", label, s, raw)
			}
		}
		if strings.Contains(strings.ToLower(text), "place") {
			t.Errorf("%s: body names a place: %s", label, raw)
		}
	}
}

func TestTimelineWithheldIs404(t *testing.T) {
	srv, mailer, pool := newTrustE2EServer(t, p07FixedClock())
	a := newVerifiedClient(t, srv, mailer, "p801-withheld-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p801-withheld-b@example.com")

	shown := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	pending := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	removed := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	old := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))

	ctx := context.Background()
	for id, state := range map[int64]string{pending: "pending", removed: "removed"} {
		if _, err := pool.Exec(ctx, `UPDATE reports SET moderation_state = $2 WHERE id = $1`, id, state); err != nil {
			t.Fatalf("setting moderation state: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE reports SET created_at = now() - interval '10 days', expires_at = now() - interval '9 days' WHERE id = $1`, old); err != nil {
		t.Fatalf("ageing a report: %v", err)
	}

	if status, raw := p801GetTimeline(t, b, srv.URL, shown); status != http.StatusOK {
		t.Fatalf("a shown report: status %d, body %s", status, raw)
	}

	ids := map[string]int64{"pending": pending, "removed": removed, "too old": old, "unknown": 987654321}
	var first []byte
	for label, id := range ids {
		status, raw := p801GetTimeline(t, b, srv.URL, id)
		if status != http.StatusNotFound {
			t.Fatalf("%s: status %d, want 404, body %s", label, status, raw)
		}
		if first == nil {
			first = raw
		} else if string(first) != string(raw) {
			t.Errorf("%s: body %s differs from %s", label, raw, first)
		}
		for _, banned := range []string{"pending", "removed", "moderation"} {
			if strings.Contains(string(raw), banned) {
				t.Errorf("%s: body names %q: %s", label, banned, raw)
			}
		}
	}

	// A malformed id is a 400, not a 404.
	resp, err := b.Get(srv.URL + "/api/reports/abc/timeline")
	if err != nil {
		t.Fatalf("GET malformed id: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed id: status %d, want 400", resp.StatusCode)
	}
}

func TestTimelineRequiresSignIn(t *testing.T) {
	srv, mailer, _ := newTrustE2EServer(t, p07FixedClock())
	a := newVerifiedClient(t, srv, mailer, "p801-signin-a@example.com")
	reportID := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	status, raw := p801GetTimeline(t, &http.Client{Jar: jar}, srv.URL, reportID)
	if status != http.StatusUnauthorized {
		t.Errorf("signed out: status %d, want 401, body %s", status, raw)
	}
}
