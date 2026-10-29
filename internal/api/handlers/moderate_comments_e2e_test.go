// moderate_comments_e2e_test.go proves the moderator side of comments over the
// real router and a real Postgres (Phase 8 plan 13, V18, D-12, D-14, T-08-63 to
// T-08-66): the list a moderator reads, Keep of a held comment, Remove, and the
// same plain 404 for everyone who is not a moderator. It reuses the p812 screener
// and helpers of comments_e2e_test.go. Every package level identifier added here
// carries the p813 prefix.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	p813ModEmail      = "p813-mod@example.com"
	p813ReporterEmail = "p813-reporter@example.com"
	p813AnnEmail      = "p813-ann@example.com"
	p813BenEmail      = "p813-ben@example.com"
)

// p813Env is one server with a moderator, a reporter and two commenters.
type p813Env struct {
	srv       *httptest.Server
	pool      *pgxpool.Pool
	scr       *p812Screener
	moderator *http.Client
	reporter  *http.Client
	ann       *http.Client
	ben       *http.Client
	report    int64
}

func p813Setup(t *testing.T) *p813Env {
	t.Helper()
	scr := &p812Screener{outcome: "clear"}
	// The transaction func is bound to the pool once the server exists.
	var pool *pgxpool.Pool
	tx := func(ctx context.Context, fn func(q service.CommentModerationQuerier) error) error {
		return service.PgxCommentModerationTx(pool)(ctx, fn)
	}
	tweak := p20Tweak(newP20Clock(), func(d *api.Deps) {
		d.ModeratorEmails = modAllowList(t, p813ModEmail)
		d.CommentModerationTx = tx
	})
	srv, mailer, p := newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		tweak(d, q)
		d.Comments = service.NewCommentService(q, service.WithCommentScreener(scr))
	})
	pool = p
	e := &p813Env{srv: srv, pool: pool, scr: scr}
	e.moderator = newVerifiedClient(t, srv, mailer, p813ModEmail)
	e.reporter = newVerifiedClient(t, srv, mailer, p813ReporterEmail)
	e.ann = newVerifiedClient(t, srv, mailer, p813AnnEmail)
	e.ben = newVerifiedClient(t, srv, mailer, p813BenEmail)
	fillProfile(t, e.reporter, srv, "rep_p813")
	fillProfile(t, e.ann, srv, "ann_p813")
	fillProfile(t, e.ben, srv, "ben_p813")
	e.report = submitReportGetID(t, e.reporter, srv.URL, p20Body("flood", "low"))
	return e
}

// send sends a JSON request to a path of the server.
func (e *p813Env) send(t *testing.T, client *http.Client, method, path, body string) *http.Response {
	t.Helper()
	return p812Send(t, client, method, e.srv.URL+path, body)
}

// postComment posts words as client on the report and returns the comment.
func (e *p813Env) postComment(t *testing.T, client *http.Client, words string) handlers.CommentResponse {
	t.Helper()
	body, err := json.Marshal(map[string]any{"words": words})
	if err != nil {
		t.Fatalf("marshalling the comment: %v", err)
	}
	raw := p20Status(t, "post "+words,
		e.send(t, client, http.MethodPost, "/api/reports/"+strconv.FormatInt(e.report, 10)+"/comments", string(body)),
		http.StatusCreated)
	var out handlers.CommentResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the posted comment %s: %v", raw, err)
	}
	return out
}

// p813Item is one comment of the moderator list as a client reads it.
type p813Item struct {
	ID          int64    `json:"id"`
	ReportID    int64    `json:"report_id"`
	Category    string   `json:"category"`
	Words       string   `json:"words"`
	At          string   `json:"at"`
	Reasons     []string `json:"reasons"`
	FlagReasons []string `json:"flag_reasons"`
	Held        bool     `json:"held"`
	NeedsReview bool     `json:"needs_review"`
	Flags       int      `json:"flags"`
}

// modList reads the moderator list and returns the items and the raw body.
func (e *p813Env) modList(t *testing.T) (map[int64]p813Item, []p813Item, []byte) {
	t.Helper()
	resp := e.send(t, e.moderator, http.MethodGet, "/api/moderation/comments", "")
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	raw := p20Status(t, "moderator list", resp, http.StatusOK)
	var out struct {
		Comments []p813Item `json:"comments"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the moderator list %s: %v", raw, err)
	}
	if out.Comments == nil {
		t.Fatalf("comments is null in %s, want an array", raw)
	}
	byID := make(map[int64]p813Item, len(out.Comments))
	for _, it := range out.Comments {
		byID[it.ID] = it
	}
	return byID, out.Comments, raw
}

// shown reports whether viewer sees the comment with this id in the public list.
func (e *p813Env) shown(t *testing.T, viewer *http.Client, id int64) (handlers.CommentResponse, bool) {
	t.Helper()
	raw := p20Status(t, "list",
		e.send(t, viewer, http.MethodGet, "/api/reports/"+strconv.FormatInt(e.report, 10)+"/comments", ""), http.StatusOK)
	var out handlers.CommentListResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the list %s: %v", raw, err)
	}
	return p812Find(out.Comments, id)
}

func (e *p813Env) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("counting (%s): %v", sql, err)
	}
	return n
}

func TestModerateComments(t *testing.T) {
	e := p813Setup(t)

	// A clear comment that Ben flags.
	flagged := e.postComment(t, e.ann, "this road is fine to drive")
	p20Status(t, "flag", e.send(t, e.ben, http.MethodPost,
		"/api/comments/"+strconv.FormatInt(flagged.ID, 10)+"/flag", `{"reasons":["spam","false"]}`), http.StatusCreated)

	// A held comment (the check could not finish) and a borderline one.
	e.scr.set("unfinished")
	held := e.postComment(t, e.ann, "held words about the bridge")
	if !held.Held {
		t.Fatalf("the unfinished check left the comment unheld: %+v", held)
	}
	e.scr.set("borderline")
	near := e.postComment(t, e.ann, "near the line words")
	e.scr.set("clear")

	// A second flagged comment that gets removed.
	doomed := e.postComment(t, e.ann, "remove these words")
	p20Status(t, "flag", e.send(t, e.ben, http.MethodPost,
		"/api/comments/"+strconv.FormatInt(doomed.ID, 10)+"/flag", `{"reasons":["abusive"]}`), http.StatusCreated)

	// The held comment is hidden from other people until Keep.
	if _, ok := e.shown(t, e.ben, held.ID); ok {
		t.Fatal("a held comment is listed to another reader before Keep")
	}

	t.Run("the list shows the words and the reasons and names nobody", func(t *testing.T) {
		byID, items, raw := e.modList(t)
		if len(items) != 4 {
			t.Fatalf("list holds %d comments, want 4: %s", len(items), raw)
		}
		check := func(name string, id int64, words string, reasons []string, flags int, held, review bool) {
			it, ok := byID[id]
			if !ok {
				t.Errorf("%s: comment %d missing", name, id)
				return
			}
			if it.Words != words || it.ReportID != e.report || it.Category != "flood" || it.At == "" {
				t.Errorf("%s: %+v, want words %q on report %d (flood) with a time", name, it, words, e.report)
			}
			if strings.Join(it.Reasons, "|") != strings.Join(reasons, "|") {
				t.Errorf("%s: reasons %v, want %v", name, it.Reasons, reasons)
			}
			if it.Flags != flags || it.Held != held || it.NeedsReview != review {
				t.Errorf("%s: flags %d held %v review %v, want %d %v %v", name, it.Flags, it.Held, it.NeedsReview, flags, held, review)
			}
		}
		check("flagged", flagged.ID, "this road is fine to drive", []string{"Flagged by people"}, 1, false, false)
		check("held", held.ID, "held words about the bridge", []string{"Check not finished"}, 0, true, false)
		check("near", near.ID, "near the line words", []string{"Near the line"}, 0, false, true)
		check("doomed", doomed.ID, "remove these words", []string{"Flagged by people"}, 1, false, false)
		if got := byID[flagged.ID].FlagReasons; strings.Join(got, "|") != "Spam or gibberish|False or a hoax" {
			t.Errorf("flag reasons = %v, want the two dialog labels in order", got)
		}

		// No key and no value names an author or a flagger (T-08-65).
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			t.Fatalf("decoding generically: %v", err)
		}
		var keys []string
		modWalkKeys(generic, &keys)
		allowed := map[string]bool{"comments": true, "id": true, "report_id": true, "category": true, "words": true, "at": true,
			"reasons": true, "flag_reasons": true, "held": true, "needs_review": true, "flags": true}
		for _, k := range keys {
			if !allowed[k] {
				t.Errorf("the list carries an unexpected key %q", k)
			}
		}
		for _, secret := range []string{"ann_p813", "ben_p813", "rep_p813", "example.com", "account"} {
			if strings.Contains(string(raw), secret) {
				t.Errorf("the list contains %q", secret)
			}
		}
	})

	t.Run("Keep releases the held comment and tells the reporter once", func(t *testing.T) {
		// The earlier clear comments already told the reporter; read those rows so
		// the unread index leaves room for exactly the row the Keep must write.
		if _, err := e.pool.Exec(context.Background(),
			`UPDATE updates SET read_at = now() WHERE kind = 'comment_on_yours' AND report_id = $1`, e.report); err != nil {
			t.Fatalf("marking the earlier bell rows read: %v", err)
		}
		unread := `SELECT count(*) FROM updates WHERE kind = 'comment_on_yours' AND report_id = $1 AND read_at IS NULL`
		if n := e.count(t, unread, e.report); n != 0 {
			t.Fatalf("unread bell rows before Keep = %d, want 0", n)
		}
		raw := p20Status(t, "keep held", e.send(t, e.moderator, http.MethodPost,
			"/api/moderation/comments/"+strconv.FormatInt(held.ID, 10)+"/keep", ""), http.StatusOK)
		var act struct {
			ID     int64  `json:"id"`
			Action string `json:"action"`
		}
		if err := json.Unmarshal(raw, &act); err != nil || act.ID != held.ID || act.Action != "keep" {
			t.Errorf("keep answer = %s (%v), want id %d action keep", raw, err, held.ID)
		}
		if c, ok := e.shown(t, e.ben, held.ID); !ok || c.Words != "held words about the bridge" {
			t.Errorf("after Keep another reader sees %+v (found %v), want the words", c, ok)
		}
		if n := e.count(t, unread, e.report); n != 1 {
			t.Errorf("unread bell rows after Keep = %d, want exactly 1", n)
		}
		// A second Keep is refused and writes nothing more.
		raw2 := p20Status(t, "keep held again", e.send(t, e.moderator, http.MethodPost,
			"/api/moderation/comments/"+strconv.FormatInt(held.ID, 10)+"/keep", ""), http.StatusConflict)
		if !strings.Contains(string(raw2), service.ModCommentAlreadyHandledMessage) {
			t.Errorf("second Keep body = %s, want the already handled message", raw2)
		}
		if n := e.count(t, `SELECT count(*) FROM updates WHERE kind = 'comment_on_yours' AND report_id = $1`, e.report); n != 2 {
			t.Errorf("bell rows after a second Keep = %d, want 2 (the read ones and the one new row)", n)
		}
		if n := e.count(t, `SELECT count(*) FROM comment_moderation_actions WHERE comment_id = $1 AND action = 'keep'`, held.ID); n != 1 {
			t.Errorf("keep audit rows = %d, want 1", n)
		}
	})

	t.Run("Keep clears the borderline mark", func(t *testing.T) {
		p20Status(t, "keep near", e.send(t, e.moderator, http.MethodPost,
			"/api/moderation/comments/"+strconv.FormatInt(near.ID, 10)+"/keep", ""), http.StatusOK)
		if n := e.count(t, `SELECT count(*) FROM comments WHERE id = $1 AND needs_review`, near.ID); n != 0 {
			t.Error("needs_review still set after Keep")
		}
		if _, ok := e.shown(t, e.ben, near.ID); !ok {
			t.Error("the kept comment is no longer shown")
		}
	})

	t.Run("a flag alone hides nothing, Remove does", func(t *testing.T) {
		if _, ok := e.shown(t, e.ben, doomed.ID); !ok {
			t.Fatal("a flagged comment is already hidden before any decision")
		}
		p20Status(t, "remove", e.send(t, e.moderator, http.MethodPost,
			"/api/moderation/comments/"+strconv.FormatInt(doomed.ID, 10)+"/remove", ""), http.StatusOK)
		if _, ok := e.shown(t, e.ben, doomed.ID); ok {
			t.Error("a removed comment with no replies is still listed")
		}
		if _, ok := e.shown(t, e.ann, doomed.ID); ok {
			t.Error("a removed comment is still listed to its author")
		}
		p20WantError(t, "remove again", e.send(t, e.moderator, http.MethodPost,
			"/api/moderation/comments/"+strconv.FormatInt(doomed.ID, 10)+"/remove", ""),
			http.StatusNotFound, "id", service.ModCommentNotFoundMessage)
		if n := e.count(t, `SELECT count(*) FROM comment_moderation_actions WHERE comment_id = $1 AND action = 'remove'`, doomed.ID); n != 1 {
			t.Errorf("remove audit rows = %d, want 1", n)
		}
	})

	t.Run("a flagged comment kept by a moderator leaves the list", func(t *testing.T) {
		p20Status(t, "keep flagged", e.send(t, e.moderator, http.MethodPost,
			"/api/moderation/comments/"+strconv.FormatInt(flagged.ID, 10)+"/keep", ""), http.StatusOK)
		if _, ok := e.shown(t, e.ben, flagged.ID); !ok {
			t.Error("a kept comment is no longer shown")
		}
		_, items, raw := e.modList(t)
		if len(items) != 0 {
			t.Errorf("list after every decision = %s, want empty", raw)
		}
		if !bytes.Contains(raw, []byte(`"comments":[]`)) {
			t.Errorf("empty list = %s, want an empty array", raw)
		}
	})

	t.Run("bad ids and wrong methods", func(t *testing.T) {
		p20WantError(t, "unknown", e.send(t, e.moderator, http.MethodPost, "/api/moderation/comments/999999/keep", ""),
			http.StatusNotFound, "id", service.ModCommentNotFoundMessage)
		p20WantError(t, "not a number", e.send(t, e.moderator, http.MethodPost, "/api/moderation/comments/abc/remove", ""),
			http.StatusBadRequest, "id", "Comment id must be a number.")
		for _, tc := range []struct{ method, path string }{
			{http.MethodPost, "/api/moderation/comments"},
			{http.MethodGet, "/api/moderation/comments/1/keep"},
			{http.MethodPut, "/api/moderation/comments/1/remove"},
		} {
			resp := e.send(t, e.moderator, tc.method, tc.path, "")
			resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s %s as a moderator = %d, want 405", tc.method, tc.path, resp.StatusCode)
			}
		}
	})
}

// p813Body reads the status and body of one request.
func p813Body(t *testing.T, client *http.Client, method, url string) (int, []byte) {
	t.Helper()
	resp := modDo(t, client, method, url)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s %s: %v", method, url, err)
	}
	return resp.StatusCode, b
}

func TestModerateCommentsNonModerator404(t *testing.T) {
	e := p813Setup(t)
	anonymous := &http.Client{CheckRedirect: modNoFollow}
	e.ben.CheckRedirect = modNoFollow

	paths := []string{
		"/api/moderation/comments",
		"/api/moderation/comments/1/keep",
		"/api/moderation/comments/1/remove",
	}
	methods := []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	visitors := []struct {
		name   string
		client *http.Client
	}{{"anonymous", anonymous}, {"ordinary_account", e.ben}}

	for _, v := range visitors {
		for _, path := range paths {
			for _, method := range methods {
				t.Run(v.name+"/"+method+"/"+path, func(t *testing.T) {
					modSameAsUnknown(t, v.client, e.srv.URL, method, path, "/api/no-such-route")
				})
			}
		}
		// The bodies are byte equal to the report moderation routes' answer.
		for _, path := range paths {
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				status, body := p813Body(t, v.client, method, e.srv.URL+path)
				wantStatus, wantBody := p813Body(t, v.client, method, e.srv.URL+"/api/moderation/reports/1/keep")
				if status != wantStatus || !bytes.Equal(body, wantBody) {
					t.Errorf("%s %s %s = %d %q, report route = %d %q, want equal", v.name, method, path, status, body, wantStatus, wantBody)
				}
			}
		}
	}

	// Nothing was changed by the refused requests.
	if n := e.count(t, `SELECT count(*) FROM comment_moderation_actions`); n != 0 {
		t.Errorf("audit rows = %d after refused requests, want 0", n)
	}
}
