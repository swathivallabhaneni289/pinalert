package handlers_test

// flag_note_e2e_test.go pins plan 04-31 (owner, 2026-10-22, 04-UAT test 10)
// end to end: the flag route takes a set of reasons and an optional note, the
// older single reason field still works on its own, the note goes through the
// report description's text check through the same screener, and the
// moderator queue counts each reason once per flag and shows each flag's note
// with its reasons and nothing that names the flagger.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"pinalert/internal/api"
	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

const p431HatefulNote = "these are hateful words about people"

// p431Screener refuses one note as hateful and passes everything else.
type p431Screener struct{}

func (p431Screener) Screen(_ context.Context, text string) (moderation.Result, error) {
	if strings.TrimSpace(text) == p431HatefulNote {
		return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{"hate": 0.99}}, nil
	}
	return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{"hate": 0.01}}, nil
}

// p431Post sends a raw flag body and returns the status and the body.
func p431Post(t *testing.T, client *http.Client, url, body string) (int, []byte) {
	t.Helper()
	resp, err := client.Post(url, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("posting flag: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

// p431WantField checks an error body's field and message.
func p431WantField(t *testing.T, label string, status int, raw []byte, wantStatus int, field, message string) {
	t.Helper()
	if status != wantStatus {
		t.Errorf("%s: status %d, want %d (%s)", label, status, wantStatus, raw)
		return
	}
	var body struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("%s: decoding %s: %v", label, raw, err)
	}
	if body.Error.Field != field || body.Error.Message != message {
		t.Errorf("%s: error %q %q, want %q %q", label, body.Error.Field, body.Error.Message, field, message)
	}
}

func TestFlagReasonsAndNoteEndToEnd(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		d.ModeratorEmails = modAllowList(t, "mod@example.com")
		d.RequestLinkRateLimit = api.RequestLinkRateLimit{Burst: 100, Every: time.Millisecond}
		d.Reports = service.NewReportService(q, service.WithScreener(p431Screener{}))
	})
	author := newVerifiedClient(t, srv, mailer, "author@example.com")
	readers := make([]*http.Client, 7)
	for i := range readers {
		readers[i] = newVerifiedClient(t, srv, mailer, "reader-"+strconv.Itoa(i)+"@example.com")
	}
	mod := newVerifiedClient(t, srv, mailer, "mod@example.com")
	reportID := submitReportGetID(t, author, srv.URL, floodReportBody(modLat, modLon))
	url := srv.URL + "/api/reports/" + strconv.FormatInt(reportID, 10) + "/flag"

	stored := func() string {
		t.Helper()
		var s string
		err := pool.QueryRow(context.Background(), `SELECT COALESCE(string_agg(array_to_string(reasons, '+') || ':' || COALESCE(note, '-'), ' | ' ORDER BY id), '')
			FROM report_flags WHERE report_id = $1`, reportID).Scan(&s)
		if err != nil {
			t.Fatalf("reading flags: %v", err)
		}
		return s
	}

	// Refusals store nothing.
	d := readers[5]
	st, raw := p431Post(t, d, url, `{"reasons":[]}`)
	p431WantField(t, "no reasons", st, raw, http.StatusBadRequest, "reasons", service.FlagReasonMessage)
	st, raw = p431Post(t, d, url, `{}`)
	p431WantField(t, "empty object", st, raw, http.StatusBadRequest, "reasons", service.FlagReasonMessage)
	st, raw = p431Post(t, d, url, `{"reasons":["spam","spam"]}`)
	p431WantField(t, "a repeated reason", st, raw, http.StatusBadRequest, "reasons", service.FlagReasonsBadMessage)
	st, raw = p431Post(t, d, url, `{"reasons":["spam","hoax"]}`)
	p431WantField(t, "an unknown reason", st, raw, http.StatusBadRequest, "reasons", service.FlagReasonsBadMessage)
	st, raw = p431Post(t, d, url, `{"reasons":["spam"],"reason":"spam"}`)
	p431WantField(t, "old and new field together", st, raw, http.StatusBadRequest, "body", "Request body is missing or malformed.")
	st, raw = p431Post(t, d, url, `{"reasons":["spam"],"note":"`+strings.Repeat("a", 301)+`"}`)
	p431WantField(t, "a 301 character note", st, raw, http.StatusBadRequest, "note", service.FlagNoteTooLongMessage)
	st, raw = p431Post(t, d, url, `{"reasons":["abusive"],"note":"  `+p431HatefulNote+` "}`)
	p431WantField(t, "a hateful note", st, raw, http.StatusUnprocessableEntity, "note", service.RejectMessage(service.RejectHateful))
	if got := stored(); got != "" {
		t.Fatalf("refusals stored flags: %q", got)
	}

	// Accepted: a set in any order is stored in the dialog's order with the
	// trimmed note; a blank note is no note; the old single field alone is a
	// one element set; 300 four byte characters fit in the body limit.
	// 300 characters of four bytes each (words of mathematical bold letters
	// split by a wave sign, so the note is words and not gibberish): about 1200 bytes, over the old 1 KiB
	// body cap.
	emoji := strings.Repeat("\U0001D41A\U0001D41B\U0001D41C\U0001F30A", 75)
	if n := utf8.RuneCountInString(emoji); n != 300 || len(emoji) < 1100 {
		t.Fatalf("the long note is %d characters and %d bytes", n, len(emoji))
	}
	escaped, _ := json.Marshal(emoji)
	for i, body := range []string{
		`{"reasons":["other","spam"],"note":"  The photo is <b>old</b>  "}`,
		`{"reasons":["false"],"note":"   "}`,
		`{"reason":"false"}`,
		`{"reasons":["false","dangerous"],"note":` + string(escaped) + `}`,
		`{"reasons":["dangerous","abusive","spam","other","false"]}`,
	} {
		if st, raw := p431Post(t, readers[i], url, body); st != http.StatusCreated {
			t.Fatalf("flag %d %s: status %d (%s), want 201", i, body, st, raw)
		}
	}
	want := "spam+other:The photo is <b>old</b> | false:- | false:- | false+dangerous:" + emoji + " | spam+abusive+false+dangerous+other:-"
	if got := stored(); got != want {
		t.Errorf("stored flags:\n got %q\nwant %q", got, want)
	}
	// One flag per account per report still holds with the new fields.
	st, raw = p431Post(t, readers[0], url, `{"reasons":["false"],"note":"again with more words"}`)
	p431WantField(t, "a second flag", st, raw, http.StatusConflict, "report", service.FlagAlreadyMessage)

	// The moderator queue.
	status, _, q, rawQueue := modGetQueue(t, mod, srv.URL)
	if status != http.StatusOK || len(q.Flagged) != 1 {
		t.Fatalf("queue status %d with %d flagged rows: %s", status, len(q.Flagged), rawQueue)
	}
	flags, _ := q.Flagged[0]["flags"].(map[string]any)
	if flags["total"] != float64(5) {
		t.Errorf("flags total = %v, want 5", flags["total"])
	}
	wantReasons := []any{
		map[string]any{"reason": "spam", "label": "Spam or gibberish", "count": float64(2)},
		map[string]any{"reason": "abusive", "label": "Abusive or hateful", "count": float64(1)},
		map[string]any{"reason": "false", "label": "False or a hoax", "count": float64(4)},
		map[string]any{"reason": "dangerous", "label": "Dangerous or illegal", "count": float64(2)},
		map[string]any{"reason": "other", "label": "Something else", "count": float64(2)},
	}
	if !reflect.DeepEqual(flags["reasons"], wantReasons) {
		t.Errorf("flag reasons = %v, want %v", flags["reasons"], wantReasons)
	}
	wantNotes := []any{
		map[string]any{"note": emoji, "reasons": []any{
			map[string]any{"reason": "false", "label": "False or a hoax"},
			map[string]any{"reason": "dangerous", "label": "Dangerous or illegal"},
		}},
		map[string]any{"note": "The photo is <b>old</b>", "reasons": []any{
			map[string]any{"reason": "spam", "label": "Spam or gibberish"},
			map[string]any{"reason": "other", "label": "Something else"},
		}},
	}
	if !reflect.DeepEqual(flags["notes"], wantNotes) {
		t.Errorf("flag notes = %v, want %v", flags["notes"], wantNotes)
	}
	var decoded any
	if err := json.Unmarshal(rawQueue, &decoded); err != nil {
		t.Fatalf("decoding raw queue: %v", err)
	}
	var keys []string
	modWalkKeys(decoded, &keys)
	for _, k := range keys {
		lower := strings.ToLower(k)
		for _, banned := range []string{"email", "session", "account", "flagger"} {
			if strings.Contains(lower, banned) {
				t.Errorf("the queue carries a key %q that names an identity", k)
			}
		}
	}
	for i := range readers {
		if strings.Contains(string(rawQueue), "reader-"+strconv.Itoa(i)+"@example.com") {
			t.Errorf("the queue names reader %d", i)
		}
	}

	// The note never reaches the author or any reader: the feed and the
	// author's own view carry no note text.
	for label, client := range map[string]*http.Client{"author": author, "reader": readers[0]} {
		resp, err := client.Get(srv.URL + "/api/reports?lat=" + strconv.FormatFloat(modLat, 'f', -1, 64) + "&lon=" + strconv.FormatFloat(modLon, 'f', -1, 64))
		if err != nil {
			t.Fatalf("%s feed: %v", label, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"id":`+strconv.FormatInt(reportID, 10)) {
			t.Fatalf("the %s feed: status %d without the report: %s", label, resp.StatusCode, body)
		}
		if strings.Contains(string(body), "The photo is") || strings.Contains(string(body), `"note"`) {
			t.Errorf("the %s feed carries the note: %s", label, body)
		}
	}
	// A reader cannot open the moderator queue at all.
	resp, err := readers[0].Get(srv.URL + "/api/moderation/queue")
	if err != nil {
		t.Fatalf("reader queue: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Errorf("a reader opened the moderator queue")
	}

	// A one word note is a good flag note (owner, 2026-10-23): it skips the
	// gibberish rule that a report description meets, and is stored as sent.
	// Reader 5 made only refusals above, so it can still flag.
	for _, f := range []struct {
		client *http.Client
		body   string
	}{
		{readers[5], `{"reasons":["other"],"note":"fake"}`},
		{readers[6], `{"reasons":["spam"],"note":" duplicate "}`},
	} {
		if st, raw := p431Post(t, f.client, url, f.body); st != http.StatusCreated {
			t.Errorf("one word note %s: status %d (%s), want 201", f.body, st, raw)
		}
	}
	if got, wantTail := stored(), " | other:fake | spam:duplicate"; !strings.HasSuffix(got, wantTail) {
		t.Errorf("stored flags end %q, want the suffix %q", got, wantTail)
	}
}
