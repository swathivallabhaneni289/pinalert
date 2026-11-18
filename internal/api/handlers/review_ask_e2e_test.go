// review_ask_e2e_test.go proves quick 261118-rz6 over the real router and a
// real Postgres: Remove needs a reason, the author's bell row opens their box,
// the box API answers only the author of a removed report (404 for everyone
// and everything else), an ask while one is open or after 3 is a 409, the
// moderator sees the ask with the note under Review asks and answers Keep
// removed or Restore, the author's bell says so, a restored report is on
// Nearby again and three places cannot hide it, and the note is in no public
// answer.
package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"pinalert/internal/api"
	"pinalert/internal/service"
)

// rzBox is the author's box answer.
type rzBox struct {
	Why   string `json:"why"`
	State string `json:"state"`
}

func rzBoxURL(baseURL string, id int64) string {
	return baseURL + "/api/reports/" + strconv.FormatInt(id, 10) + "/review-ask"
}

// rzGet reads the box and returns the status and the raw answer.
func rzGet(t *testing.T, client *http.Client, baseURL string, id int64) (int, []byte) {
	t.Helper()
	resp, err := client.Get(rzBoxURL(baseURL, id))
	if err != nil {
		t.Fatalf("GET box: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the box: %v", err)
	}
	return resp.StatusCode, raw
}

// rzAsk posts an ask with a raw body and returns the status and the raw answer.
func rzAsk(t *testing.T, client *http.Client, baseURL string, id int64, body string) (int, []byte) {
	t.Helper()
	resp, err := client.Post(rzBoxURL(baseURL, id), "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST ask: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the ask answer: %v", err)
	}
	return resp.StatusCode, raw
}

func rzDecodeBox(t *testing.T, raw []byte) rzBox {
	t.Helper()
	var b rzBox
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("decoding the box %s: %v", raw, err)
	}
	return b
}

// rzQueue reads the two lists of quick 261118-rz6 from the moderator queue.
func rzQueue(t *testing.T, client *http.Client, baseURL string) (asks, removed []map[string]any, counts map[string]float64, raw []byte) {
	t.Helper()
	status, _, _, raw := modGetQueue(t, client, baseURL)
	if status != http.StatusOK {
		t.Fatalf("queue status %d", status)
	}
	var out struct {
		Asks    []map[string]any   `json:"asks"`
		Removed []map[string]any   `json:"removed"`
		Counts  map[string]float64 `json:"counts"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the queue: %v", err)
	}
	return out.Asks, out.Removed, out.Counts, raw
}

func TestReviewAskFlow(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), func(d *api.Deps) {
		d.ModeratorEmails = modAllowList(t, "rz6-mod@example.com")
	}))
	author := newVerifiedClient(t, srv, mailer, "rz6-author@example.com")
	stranger := newVerifiedClient(t, srv, mailer, "rz6-stranger@example.com")
	moderator := newVerifiedClient(t, srv, mailer, "rz6-mod@example.com")
	flaggers := make([]*http.Client, 3)
	for i := range flaggers {
		flaggers[i] = newVerifiedClient(t, srv, mailer, "rz6-flagger-"+strconv.Itoa(i)+"@example.com")
	}

	const descRemoved = "Bridge closed near the market, water over the deck"
	const note = "I took the photo myself this morning."
	removed := submitReportGetID(t, author, srv.URL, vvbBody(descRemoved))
	shown := submitReportGetID(t, author, srv.URL, vvbBody("Shelter open at the school hall"))
	held := submitReportGetID(t, author, srv.URL, vvbBody("Power out on the whole street"))
	review := submitReportGetID(t, author, srv.URL, vvbBody("Road blocked by a fallen tree"))
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE reports SET moderation_state = 'pending' WHERE id = $1`, held); err != nil {
		t.Fatalf("holding: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE reports SET moderation_state = 'review' WHERE id = $1`, review); err != nil {
		t.Fatalf("hiding: %v", err)
	}

	t.Run("Remove without a reason is a 400 and changes nothing", func(t *testing.T) {
		for _, body := range []string{`{}`, `{"reason":""}`, `{"reason":"text_check"}`, `{"reason":"rude"}`} {
			status, out := modActionBody(t, moderator, srv.URL, removed, "remove", body)
			if status != http.StatusBadRequest || out.Error.Field != "reason" || out.Error.Message != service.RemovalReasonMessage {
				t.Errorf("remove with %s: %d %+v, want 400 on reason", body, status, out.Error)
			}
		}
		if status, out := modActionBody(t, moderator, srv.URL, removed, "remove", `{"reason":"spam","x":1}`); status != http.StatusBadRequest || out.Error.Field != "body" {
			t.Errorf("an unknown field: %d %+v, want 400 on body", status, out.Error)
		}
		var state string
		if err := pool.QueryRow(ctx, `SELECT moderation_state FROM reports WHERE id = $1`, removed).Scan(&state); err != nil || state != "clear" {
			t.Errorf("state after refused removes = %q, %v", state, err)
		}
	})

	t.Run("Remove with False, and the author's bell row opens the box", func(t *testing.T) {
		if status, out := modActionBody(t, moderator, srv.URL, removed, "remove", `{"reason":"false"}`); status != http.StatusOK || out.Report["removed_reason"] != "false" {
			t.Fatalf("remove: %d %+v", status, out)
		}
		rows, raw := vvbBell(t, author, srv.URL)
		row, ok := rows["own_removed"]
		if !ok || row.ReviewReportID == nil || *row.ReviewReportID != removed || row.ReportID != nil || row.Link != "" {
			t.Fatalf("own_removed row = %+v in %s", row, raw)
		}
		status, boxRaw := rzGet(t, author, srv.URL, removed)
		if status != http.StatusOK {
			t.Fatalf("GET box: %d %s", status, boxRaw)
		}
		if box := rzDecodeBox(t, boxRaw); box.Why != "Why: False" || box.State != "can_ask" {
			t.Errorf("box = %+v, want Why: False and can_ask", box)
		}
		for _, banned := range []string{descRemoved, "flag", "moderator", "spam"} {
			if strings.Contains(strings.ToLower(string(boxRaw)), strings.ToLower(banned)) {
				t.Errorf("the box carries %q: %s", banned, boxRaw)
			}
		}
	})

	t.Run("a stranger, a shown, a held and a hidden report all answer 404", func(t *testing.T) {
		cases := []struct {
			name   string
			client *http.Client
			id     int64
		}{
			{"a stranger", stranger, removed},
			{"the moderator", moderator, removed},
			{"a shown report", author, shown},
			{"a held report", author, held},
			{"a report under review", author, review},
			{"no report", author, 999999},
		}
		for _, c := range cases {
			if status, raw := rzGet(t, c.client, srv.URL, c.id); status != http.StatusNotFound {
				t.Errorf("%s: GET %d %s, want 404", c.name, status, raw)
			}
			if status, raw := rzAsk(t, c.client, srv.URL, c.id, `{}`); status != http.StatusNotFound {
				t.Errorf("%s: POST %d %s, want 404", c.name, status, raw)
			}
		}
		anon := &http.Client{}
		resp, err := anon.Get(rzBoxURL(srv.URL, removed))
		if err != nil {
			t.Fatalf("anonymous GET: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("anonymous GET: %d, want 401", resp.StatusCode)
		}
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM report_review_asks`).Scan(&n); err != nil || n != 0 {
			t.Errorf("%d asks stored by refused calls (%v)", n, err)
		}
	})

	t.Run("an ask with a note, then 409 while it is open; a too long note is a 400", func(t *testing.T) {
		if status, raw := rzAsk(t, author, srv.URL, removed, `{"note":"`+strings.Repeat("a", 201)+`"}`); status != http.StatusBadRequest {
			t.Errorf("a 201 character note: %d %s", status, raw)
		}
		status, raw := rzAsk(t, author, srv.URL, removed, `{"note":"  `+note+`  "}`)
		if status != http.StatusCreated {
			t.Fatalf("ask: %d %s", status, raw)
		}
		if box := rzDecodeBox(t, raw); box.State != "asked" || box.Why != "Why: False" {
			t.Errorf("ask answer = %+v", box)
		}
		if strings.Contains(string(raw), note) {
			t.Error("the ask answer echoes the note")
		}
		status, raw = rzAsk(t, author, srv.URL, removed, `{}`)
		if status != http.StatusConflict || !strings.Contains(string(raw), service.ReviewAskOpenMessage) {
			t.Errorf("a second ask while open: %d %s", status, raw)
		}
	})

	t.Run("the moderator sees the ask with the note and keeps it removed", func(t *testing.T) {
		asks, list, counts, _ := rzQueue(t, moderator, srv.URL)
		if len(asks) != 1 || int64(asks[0]["id"].(float64)) != removed || asks[0]["ask_note"] != note || asks[0]["removed_reason"] != "false" {
			t.Fatalf("asks = %+v", asks)
		}
		if len(list) != 1 || int64(list[0]["id"].(float64)) != removed || list[0]["ask_note"] != nil {
			t.Errorf("removed list = %+v", list)
		}
		if counts["asks"] != 1 || counts["removed"] != 1 {
			t.Errorf("counts = %+v", counts)
		}
		if status, out := modAction(t, moderator, srv.URL, removed, "keep-removed"); status != http.StatusOK || out.Report["moderation"] != "removed" {
			t.Fatalf("keep removed: %d %+v", status, out)
		}
		if status, _ := modAction(t, moderator, srv.URL, removed, "keep-removed"); status != http.StatusConflict {
			t.Errorf("a repeated keep removed: %d, want 409", status)
		}
		rows, raw := vvbBell(t, author, srv.URL)
		row, ok := rows["own_stays_removed"]
		if !ok || row.Event != "Your report stays removed." || row.ReviewReportID == nil || *row.ReviewReportID != removed {
			t.Errorf("own_stays_removed row = %+v in %s", row, raw)
		}
		if _, b := rzGet(t, author, srv.URL, removed); rzDecodeBox(t, b).State != "can_ask" {
			t.Errorf("after one answered ask the box is %s", b)
		}
	})

	t.Run("asks two and three, then 409 with no asks left", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			if status, raw := rzAsk(t, author, srv.URL, removed, `{}`); status != http.StatusCreated {
				t.Fatalf("ask %d: %d %s", i+2, status, raw)
			}
			if status, _ := modAction(t, moderator, srv.URL, removed, "keep-removed"); status != http.StatusOK {
				t.Fatalf("keep removed %d: %d", i+2, status)
			}
		}
		if _, b := rzGet(t, author, srv.URL, removed); rzDecodeBox(t, b).State != "no_asks_left" {
			t.Errorf("after three asks the box is %s", b)
		}
		status, raw := rzAsk(t, author, srv.URL, removed, `{}`)
		if status != http.StatusConflict || !strings.Contains(string(raw), service.ReviewAskNoneLeftMessage) {
			t.Errorf("a fourth ask: %d %s", status, raw)
		}
		rows, _ := vvbBell(t, author, srv.URL)
		if _, ok := rows["own_stays_removed"]; !ok {
			t.Error("the stays removed row is gone")
		}
	})

	t.Run("the full updates list draws the box rows as buttons", func(t *testing.T) {
		page := getPageBody(t, author, srv.URL, "/profile?part=updates")
		for _, words := range []string{"Your report was removed after review.", "Your report stays removed."} {
			row := vvbRowOf(t, page, words)
			if !strings.Contains(row, `data-review-ask="`+strconv.FormatInt(removed, 10)+`"`) || strings.Contains(row, "href=") {
				t.Errorf("%q is not a box button: %s", words, row)
			}
		}
		if strings.Contains(page, note) {
			t.Error("the updates list carries the note")
		}
		if !strings.Contains(page, `id="review-ask-box"`) {
			t.Error("the page has no box")
		}
	})

	t.Run("Restore shows it again, the bell says it is back, and three places cannot hide it", func(t *testing.T) {
		if status, out := modAction(t, moderator, srv.URL, removed, "restore"); status != http.StatusOK || out.Report["moderation"] != "clear" {
			t.Fatalf("restore: %d %+v", status, out)
		}
		if status, _ := modAction(t, moderator, srv.URL, removed, "restore"); status != http.StatusConflict {
			t.Errorf("a repeated restore: %d, want 409", status)
		}
		rows, raw := vvbBell(t, author, srv.URL)
		back, ok := rows["own_restored"]
		if !ok || back.Event != "Your report is back." || back.ReportID == nil || *back.ReportID != removed {
			t.Errorf("own_restored row = %+v in %s", back, raw)
		}
		for _, kind := range []string{"own_removed", "own_stays_removed"} {
			if rows[kind].ReviewReportID != nil {
				t.Errorf("%s still opens the box of a shown report", kind)
			}
		}
		if status, _ := rzGet(t, author, srv.URL, removed); status != http.StatusNotFound {
			t.Errorf("the box of a restored report: %d, want 404", status)
		}
		inFeed := func() bool {
			_, ok := findFeedReport(decodeFeedResponse(t, getNearby(t, stranger, srv.URL, vvbLat, vvbLon, false)), removed)
			return ok
		}
		if !inFeed() {
			t.Fatal("the restored report is not on Nearby")
		}
		for k, f := range flaggers {
			vvbFlagAt(t, f, srv.URL, removed, k+1)
		}
		if !inFeed() {
			t.Error("flags from three places hid a restored report")
		}
		page := getPageBody(t, author, srv.URL, "/profile?part=updates")
		if !strings.Contains(vvbRowOf(t, page, "Your report is back."), `href="/?report=`+strconv.FormatInt(removed, 10)+`"`) {
			t.Error("the back row does not open the report")
		}
	})

	t.Run("Restore straight from the Removed list", func(t *testing.T) {
		if status, _ := modAction(t, moderator, srv.URL, shown, "remove"); status != http.StatusOK {
			t.Fatalf("remove: %d", status)
		}
		if status, _ := modAction(t, moderator, srv.URL, shown, "keep-removed"); status != http.StatusConflict {
			t.Errorf("keep removed with no ask: %d, want 409", status)
		}
		_, list, _, _ := rzQueue(t, moderator, srv.URL)
		if len(list) != 1 || int64(list[0]["id"].(float64)) != shown || list[0]["removed_reason"] != "spam" {
			t.Fatalf("removed list = %+v", list)
		}
		if status, _ := modAction(t, moderator, srv.URL, shown, "restore"); status != http.StatusOK {
			t.Fatalf("restore: %d", status)
		}
		asks, list, counts, _ := rzQueue(t, moderator, srv.URL)
		if len(asks) != 0 || len(list) != 0 || counts["asks"] != 0 || counts["removed"] != 0 {
			t.Errorf("after restore: asks %v removed %v counts %v", asks, list, counts)
		}
	})

	t.Run("the note is in no public answer", func(t *testing.T) {
		// The author has two reports with own rows now, so the raw bell is read.
		_, bellRaw := p32uPoll(t, "bell", author, srv.URL)
		bell := string(bellRaw)
		feed := getNearby(t, stranger, srv.URL, vvbLat, vvbLon, false)
		defer feed.Body.Close()
		var buf strings.Builder
		if _, err := io.Copy(&buf, feed.Body); err != nil {
			t.Fatalf("reading the feed: %v", err)
		}
		link, _ := h10Get(t, stranger, h10URL(srv.URL, vvbLat, vvbLon, "report="+strconv.FormatInt(removed, 10)))
		linkRaw, err := json.Marshal(link)
		if err != nil {
			t.Fatalf("encoding the link answer: %v", err)
		}
		for name, body := range map[string]string{"bell": bell, "feed": buf.String(), "report link": string(linkRaw)} {
			if strings.Contains(body, note) || strings.Contains(body, "ask_note") || strings.Contains(body, "removed_reason") {
				t.Errorf("the %s carries the note or the moderation fields", name)
			}
		}
	})
}
