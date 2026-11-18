// flag_check_e2e_test.go proves quick 261118-saj over the real router and a
// real Postgres: flags from three places leave a Critical report on the feed
// with being_checked true, a boolean and nothing more, while an ordinary report
// is hidden as before; the moderator sees the marked report first, still
// showing; Keep takes the mark off and a fresh set of flags does not bring it
// back; a repeat Keep answers 409.
package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/api"
)

// sajFeed reads the raw Nearby answer for the shared point.
func sajFeed(t *testing.T, client *http.Client, baseURL string) (map[int64]map[string]any, string) {
	t.Helper()
	resp := getNearby(t, client, baseURL, vvbLat, vvbLon, false)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the feed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("feed status %d: %s", resp.StatusCode, raw)
	}
	var out struct {
		Reports []map[string]any `json:"reports"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the feed: %v", err)
	}
	byID := map[int64]map[string]any{}
	for _, r := range out.Reports {
		if id, ok := r["id"].(float64); ok {
			byID[int64(id)] = r
		}
	}
	return byID, string(raw)
}

// sajFlagged reads the moderator's flagged list as raw rows, in order.
func sajFlagged(t *testing.T, client *http.Client, baseURL string) []map[string]any {
	t.Helper()
	status, _, _, raw := modGetQueue(t, client, baseURL)
	if status != http.StatusOK {
		t.Fatalf("moderator queue status %d", status)
	}
	var out struct {
		Flagged []map[string]any `json:"flagged"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the queue: %v", err)
	}
	return out.Flagged
}

func TestFlagCheckFlow(t *testing.T) {
	srv, mailer, _ := newE2EServerWith(t, p20Tweak(newP20Clock(), func(d *api.Deps) {
		d.ModeratorEmails = modAllowList(t, "saj-mod@example.com")
	}))
	author := newVerifiedClient(t, srv, mailer, "saj-author@example.com")
	reader := newVerifiedClient(t, srv, mailer, "saj-reader@example.com")
	moderator := newVerifiedClient(t, srv, mailer, "saj-mod@example.com")
	flaggers := make([]*http.Client, 6)
	for i := range flaggers {
		flaggers[i] = newVerifiedClient(t, srv, mailer, "saj-flagger-"+strconv.Itoa(i)+"@example.com")
	}

	critical := vvbBody("Bridge washed out on the ring road")
	critical["severity"] = "critical"
	critical["happening"] = "water_rising"
	marked := submitReportGetID(t, author, srv.URL, critical)
	ordinary := submitReportGetID(t, author, srv.URL, vvbBody("Puddles near the bus stop"))

	feed, _ := sajFeed(t, reader, srv.URL)
	if got, ok := feed[marked]["being_checked"].(bool); !ok || got {
		t.Fatalf("before any flag being_checked = %v (present %v), want false and always present", feed[marked]["being_checked"], ok)
	}

	for k := 0; k < 3; k++ {
		vvbFlagAt(t, flaggers[k], srv.URL, marked, k+1)
		vvbFlagAt(t, flaggers[k], srv.URL, ordinary, k+1)
	}

	t.Run("the critical report stays on the feed with the mark, the ordinary one hides", func(t *testing.T) {
		feed, raw := sajFeed(t, reader, srv.URL)
		row, ok := feed[marked]
		if !ok {
			t.Fatal("the marked critical report left the feed")
		}
		if row["being_checked"] != true || row["visibility"] != "live" {
			t.Errorf("marked row: being_checked %v visibility %v, want true and live", row["being_checked"], row["visibility"])
		}
		if _, ok := feed[ordinary]; ok {
			t.Error("the ordinary report is still on the feed after three places")
		}
		// Only a boolean: no count, cell or flagger reaches a reader.
		for _, banned := range []string{"flag_count", "flags", "flagger", "flag_check_at", "flagged_by", "open_flag"} {
			if strings.Contains(raw, `"`+banned) {
				t.Errorf("the feed carries %q", banned)
			}
		}
		for k := 1; k <= 3; k++ {
			lat, lon := vvbPlace(k)
			if cell := geohash.EncodeWithPrecision(lat, lon, 7); strings.Contains(raw, cell) {
				t.Errorf("the feed carries the place cell %q", cell)
			}
		}
		// The author sees the same and has no bell row for the mark.
		authorFeed, _ := sajFeed(t, author, srv.URL)
		if authorFeed[marked]["being_checked"] != true {
			t.Error("the author's own feed does not carry the mark")
		}
		rows, _ := vvbBell(t, author, srv.URL)
		if _, ok := rows["own_review"]; !ok {
			t.Error("the ordinary report's own_review row is missing")
		}
		if n := len(rows); n != 1 {
			t.Errorf("the author's bell holds %d own rows, want only the ordinary report's own_review: %v", n, rows)
		}
	})

	t.Run("a marked report is served everywhere a clear Critical report is", func(t *testing.T) {
		// Votes, comments, the history and the share link all answer as for any
		// shown report; the hidden ordinary one answers 404 on each.
		resp := postVote(t, reader, srv.URL, marked, "confirm", map[string]any{"latitude": vvbLat, "longitude": vvbLon})
		p20Status(t, "confirm on the marked report", resp, http.StatusOK)
		for _, path := range []string{"/comments", "/timeline"} {
			r, err := reader.Get(srv.URL + "/api/reports/" + strconv.FormatInt(marked, 10) + path)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			p20Status(t, "GET "+path+" of the marked report", r, http.StatusOK)
			r, err = reader.Get(srv.URL + "/api/reports/" + strconv.FormatInt(ordinary, 10) + path)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			p20Status(t, "GET "+path+" of the hidden report", r, http.StatusNotFound)
		}
		feed, _ := sajFeed(t, reader, srv.URL)
		if feed[marked]["being_checked"] != true || feed[marked]["your_vote"] != "confirm" {
			t.Errorf("after the vote: being_checked %v your_vote %v", feed[marked]["being_checked"], feed[marked]["your_vote"])
		}
	})

	t.Run("the moderator sees it first, still showing", func(t *testing.T) {
		flagged := sajFlagged(t, moderator, srv.URL)
		if len(flagged) < 2 {
			t.Fatalf("flagged list has %d rows, want 2", len(flagged))
		}
		pos := map[int64]int{}
		for i, r := range flagged {
			pos[int64(r["id"].(float64))] = i
			if _, ok := r["being_checked"].(bool); !ok {
				t.Errorf("row %v has no boolean being_checked", r["id"])
			}
		}
		m := flagged[pos[marked]]
		if m["being_checked"] != true || m["moderation"] != "clear" || m["view"] != "active" {
			t.Errorf("marked row: being_checked %v moderation %v view %v", m["being_checked"], m["moderation"], m["view"])
		}
		if flagged[pos[ordinary]]["being_checked"] != false || flagged[pos[ordinary]]["moderation"] != "review" {
			t.Errorf("hidden row: being_checked %v moderation %v", flagged[pos[ordinary]]["being_checked"], flagged[pos[ordinary]]["moderation"])
		}
	})

	t.Run("Keep takes the mark off for good and a repeat is 409", func(t *testing.T) {
		if status, out := modAction(t, moderator, srv.URL, marked, "keep"); status != http.StatusOK {
			t.Fatalf("Keep: %d %+v", status, out)
		}
		if status, _ := modAction(t, moderator, srv.URL, marked, "keep"); status != http.StatusConflict {
			t.Errorf("a repeat Keep answered %d, want 409", status)
		}
		for k := 3; k < 6; k++ {
			vvbFlagAt(t, flaggers[k], srv.URL, marked, k+1)
		}
		feed, _ := sajFeed(t, reader, srv.URL)
		if row, ok := feed[marked]; !ok || row["being_checked"] != false {
			t.Errorf("after Keep and three new places: on feed %v, being_checked %v; want shown with no mark", ok, row["being_checked"])
		}
		for _, r := range sajFlagged(t, moderator, srv.URL) {
			if int64(r["id"].(float64)) == marked && r["being_checked"] != false {
				t.Error("a kept report is tagged Still showing again")
			}
		}
	})

	t.Run("Remove from the mark, then a repeat is 409", func(t *testing.T) {
		second := submitReportGetID(t, author, srv.URL, critical)
		for k := 0; k < 3; k++ {
			vvbFlagAt(t, flaggers[k], srv.URL, second, k+1)
		}
		feed, _ := sajFeed(t, reader, srv.URL)
		if feed[second]["being_checked"] != true {
			t.Fatalf("setup: the second report is not marked: %v", feed[second])
		}
		if status, _ := modAction(t, moderator, srv.URL, second, "remove"); status != http.StatusOK {
			t.Fatalf("Remove: %d", status)
		}
		if status, _ := modAction(t, moderator, srv.URL, second, "remove"); status != http.StatusConflict {
			t.Errorf("a repeat Remove answered %d, want 409", status)
		}
		feed, _ = sajFeed(t, reader, srv.URL)
		if _, ok := feed[second]; ok {
			t.Error("a removed report is still on the feed")
		}
	})
}
