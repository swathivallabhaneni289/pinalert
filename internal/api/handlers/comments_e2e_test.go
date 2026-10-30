// comments_e2e_test.go proves the six comment routes end to end over the real
// router and a real Postgres (Phase 8 plan 12, PAGE-02, SC3, D-12 to D-15,
// A-P8-4). Every server is built through newE2EServerWith with p20Tweak, which
// opens every bucket first (including Comment and Like) so a test only ever
// meets the bucket it sets. A fake screener stands in for the provider so each
// outcome can be asked for. Every package level identifier carries the p812
// prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// p812Screener answers each text check outcome on demand: clear, borderline,
// reject or unfinished. It is safe for the server's goroutines.
type p812Screener struct {
	mu      sync.Mutex
	outcome string
	calls   int
}

func (s *p812Screener) set(outcome string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outcome = outcome
}

func (s *p812Screener) Screen(_ context.Context, _ string) (moderation.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	switch s.outcome {
	case "borderline":
		return moderation.Result{Model: "test-model", Scores: map[string]float64{"hate": 0.5}}, nil
	case "reject":
		return moderation.Result{Model: "test-model", Scores: map[string]float64{"hate": 0.95}}, nil
	case "unfinished":
		return moderation.Result{}, fmt.Errorf("provider down")
	default:
		return moderation.Result{Model: "test-model", Scores: map[string]float64{"hate": 0.01}}, nil
	}
}

// p812Env is one server with a reporter and three readers, each with a
// username, and one open report.
type p812Env struct {
	srv      *httptest.Server
	pool     *pgxpool.Pool
	scr      *p812Screener
	reporter *http.Client
	ann      *http.Client
	ben      *http.Client
	cat      *http.Client
	report   int64
}

const (
	p812ReporterEmail = "p812-reporter@example.com"
	p812AnnEmail      = "p812-ann@example.com"
	p812BenEmail      = "p812-ben@example.com"
	p812CatEmail      = "p812-cat@example.com"
)

// p812Setup builds the server. mutate, when non nil, runs on the Deps after the
// buckets are opened, so a test can set one bucket.
func p812Setup(t *testing.T, mutate func(d *api.Deps)) *p812Env {
	t.Helper()
	scr := &p812Screener{outcome: "clear"}
	tweak := p20Tweak(newP20Clock(), mutate)
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		tweak(d, q)
		d.Comments = service.NewCommentService(q, service.WithCommentScreener(scr))
	})
	e := &p812Env{srv: srv, pool: pool, scr: scr}
	e.reporter = newVerifiedClient(t, srv, mailer, p812ReporterEmail)
	e.ann = newVerifiedClient(t, srv, mailer, p812AnnEmail)
	e.ben = newVerifiedClient(t, srv, mailer, p812BenEmail)
	e.cat = newVerifiedClient(t, srv, mailer, p812CatEmail)
	fillProfile(t, e.reporter, srv, "rep_p812")
	fillProfile(t, e.ann, srv, "ann_p812")
	fillProfile(t, e.ben, srv, "ben_p812")
	fillProfile(t, e.cat, srv, "cat_p812")
	e.report = submitReportGetID(t, e.reporter, srv.URL, p20Body("flood", "low"))
	return e
}

// p812Send sends a JSON request and returns the response.
func p812Send(t *testing.T, client *http.Client, method, url, body string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatalf("building %s %s: %v", method, url, err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func (e *p812Env) commentsURL(reportID int64) string {
	return e.srv.URL + "/api/reports/" + strconv.FormatInt(reportID, 10) + "/comments"
}

func (e *p812Env) commentURL(commentID int64, action string) string {
	return e.srv.URL + "/api/comments/" + strconv.FormatInt(commentID, 10) + "/" + action
}

// post posts words (and a reply target when replyTo is not zero) and returns
// the raw response.
func (e *p812Env) post(t *testing.T, client *http.Client, reportID int64, words string, replyTo int64) *http.Response {
	t.Helper()
	body := map[string]any{"words": words}
	if replyTo != 0 {
		body["reply_to"] = replyTo
	}
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling the comment: %v", err)
	}
	return p812Send(t, client, http.MethodPost, e.commentsURL(reportID), string(buf))
}

// mustPost posts and requires 201, returning the decoded comment.
func (e *p812Env) mustPost(t *testing.T, client *http.Client, reportID int64, words string, replyTo int64) handlers.CommentResponse {
	t.Helper()
	raw := p20Status(t, "post "+words, e.post(t, client, reportID, words, replyTo), http.StatusCreated)
	var out handlers.CommentResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the posted comment %s: %v", raw, err)
	}
	return out
}

// list reads the comments of a report as client and requires 200.
func (e *p812Env) list(t *testing.T, client *http.Client, reportID int64) handlers.CommentListResponse {
	t.Helper()
	raw := p20Status(t, "list", p812Send(t, client, http.MethodGet, e.commentsURL(reportID), ""), http.StatusOK)
	var out handlers.CommentListResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the list %s: %v", raw, err)
	}
	return out
}

// act posts to a comment route (like, unlike, delete) and returns the response.
func (e *p812Env) act(t *testing.T, client *http.Client, commentID int64, action string) *http.Response {
	t.Helper()
	return p812Send(t, client, http.MethodPost, e.commentURL(commentID, action), "")
}

func (e *p812Env) flag(t *testing.T, client *http.Client, commentID int64, body string) *http.Response {
	t.Helper()
	return p812Send(t, client, http.MethodPost, e.commentURL(commentID, "flag"), body)
}

// find returns the comment with id in a list, searching replies, and whether it
// was found.
func p812Find(list []handlers.CommentResponse, id int64) (handlers.CommentResponse, bool) {
	for _, c := range list {
		if c.ID == id {
			return c, true
		}
		if r, ok := p812Find(c.Replies, id); ok {
			return r, true
		}
	}
	return handlers.CommentResponse{}, false
}

// accountID reads an account id by email.
func (e *p812Env) accountID(t *testing.T, email string) int64 {
	t.Helper()
	var id int64
	if err := e.pool.QueryRow(context.Background(), `SELECT id FROM accounts WHERE email = $1`, email).Scan(&id); err != nil {
		t.Fatalf("reading the account of %s: %v", email, err)
	}
	return id
}

// resolve has the reporter close the report, as a stored resolve vote.
func (e *p812Env) resolve(t *testing.T, reportID int64) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(),
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
		 VALUES ($1, $2, 'resolution', 'resolve', 'tdr1v8y', now() - interval '1 hour')`,
		reportID, e.accountID(t, p812ReporterEmail)); err != nil {
		t.Fatalf("seeding the reporter's resolve vote: %v", err)
	}
}

// age makes the report end ago before now.
func (e *p812Env) age(t *testing.T, reportID int64, ago time.Duration) {
	t.Helper()
	secs := int(ago.Seconds())
	if _, err := e.pool.Exec(context.Background(),
		`UPDATE reports SET created_at = now() - make_interval(secs => $2 + 28800),
		                    expires_at = now() - make_interval(secs => $2) WHERE id = $1`,
		reportID, secs); err != nil {
		t.Fatalf("ageing report %d: %v", reportID, err)
	}
}

func (e *p812Env) commentRows(t *testing.T, reportID int64) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM comments WHERE report_id = $1`, reportID).Scan(&n); err != nil {
		t.Fatalf("counting comments: %v", err)
	}
	return n
}

func TestCommentScreening(t *testing.T) {
	e := p812Setup(t, nil)

	t.Run("clear text is posted", func(t *testing.T) {
		e.scr.set("clear")
		resp := e.post(t, e.ann, e.report, "  The road is open again.  ", 0)
		if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", cc)
		}
		raw := p20Status(t, "clear post", resp, http.StatusCreated)
		var c handlers.CommentResponse
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatalf("decoding %s: %v", raw, err)
		}
		if c.Words != "The road is open again." || c.Held || !c.IsOwn || !c.CanDelete || c.Author != "ann_p812" || c.AuthorKind != "user" {
			t.Errorf("comment = %+v, want an own, deletable, unheld comment by ann_p812 with trimmed words", c)
		}
	})

	t.Run("borderline text is posted and marked for review", func(t *testing.T) {
		e.scr.set("borderline")
		c := e.mustPost(t, e.ann, e.report, "this is close to the line", 0)
		if c.Held {
			t.Errorf("a borderline comment must not be held")
		}
		var review bool
		if err := e.pool.QueryRow(context.Background(), `SELECT needs_review FROM comments WHERE id = $1`, c.ID).Scan(&review); err != nil {
			t.Fatalf("reading the row: %v", err)
		}
		if !review {
			t.Errorf("needs_review = false, want true")
		}
	})

	t.Run("refused text answers 422 with the report sentence and stores nothing", func(t *testing.T) {
		e.scr.set("reject")
		before := e.commentRows(t, e.report)
		resp := e.post(t, e.ann, e.report, "something hateful", 0)
		p20WantError(t, "rejected post", resp, http.StatusUnprocessableEntity, "words", service.RejectMessage(service.RejectHateful))
		if after := e.commentRows(t, e.report); after != before {
			t.Errorf("comment rows %d -> %d, want nothing stored", before, after)
		}
	})

	t.Run("text the check could not finish is held and shown to its author only", func(t *testing.T) {
		e.scr.set("unfinished")
		c := e.mustPost(t, e.ann, e.report, "waiting for the check", 0)
		if !c.Held {
			t.Errorf("held = false, want true")
		}
		if _, ok := p812Find(e.list(t, e.ann, e.report).Comments, c.ID); !ok {
			t.Errorf("the author does not see the held comment")
		}
		if _, ok := p812Find(e.list(t, e.ben, e.report).Comments, c.ID); ok {
			t.Errorf("another account sees a held comment")
		}
	})

	t.Run("text rules answer 400 with their words", func(t *testing.T) {
		e.scr.set("clear")
		p20WantError(t, "empty", e.post(t, e.ann, e.report, "   ", 0), http.StatusBadRequest, "words", service.CommentEmptyMessage)
		p20WantError(t, "too long", e.post(t, e.ann, e.report, strings.Repeat("a", 281), 0), http.StatusBadRequest, "words", service.CommentTooLongMessage)
		p20WantError(t, "characters", e.post(t, e.ann, e.report, "bad\u0007bell", 0), http.StatusBadRequest, "words", service.CommentCharactersMessage)
		p20WantError(t, "unknown member", p812Send(t, e.ann, http.MethodPost, e.commentsURL(e.report), `{"words":"hello","extra":1}`), http.StatusBadRequest, "body", "Request body is missing or malformed.")
		p20WantError(t, "bad report id", p812Send(t, e.ann, http.MethodPost, e.srv.URL+"/api/reports/abc/comments", `{"words":"hello"}`), http.StatusBadRequest, "id", "Report id must be a number.")
	})
}

func TestCommentListQuery(t *testing.T) {
	e := p812Setup(t, nil)
	e.mustPost(t, e.ann, e.report, "first", 0)
	for _, bad := range []string{"abc", "0", "-3"} {
		resp := p812Send(t, e.ben, http.MethodGet, e.commentsURL(e.report)+"?after="+bad, "")
		p20WantError(t, "after="+bad, resp, http.StatusBadRequest, "after", "The after value must be a positive number.")
	}
	resp := p812Send(t, e.ben, http.MethodGet, e.commentsURL(e.report)+"?after=999999999", "")
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	p20Status(t, "after far ahead", resp, http.StatusOK)
	// A report that does not exist is 404.
	p20WantError(t, "unknown report", p812Send(t, e.ben, http.MethodGet, e.commentsURL(987654321), ""),
		http.StatusNotFound, "report", "This report is no longer available.")
}

// TestCommentReplyPagesQuery proves the reply paging of review fix round 2
// (SK-01): replies_of names a thread root and reply_after the last reply read,
// and the answer is that root alone with its replies after the cursor, oldest
// first. An id that is not a shown root of the report answers an empty list,
// and a bad value answers 400 with its field.
func TestCommentReplyPagesQuery(t *testing.T) {
	e := p812Setup(t, nil)
	top := e.mustPost(t, e.ann, e.report, "is the bridge open", 0)
	r1 := e.mustPost(t, e.ben, e.report, "closed at the north end", top.ID)
	r2 := e.mustPost(t, e.ann, e.report, "thanks", top.ID)
	other := e.mustPost(t, e.ben, e.report, "power is back on our road", 0)

	read := func(label, query string) handlers.CommentListResponse {
		t.Helper()
		resp := p812Send(t, e.ben, http.MethodGet, e.commentsURL(e.report)+query, "")
		if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", label, cc)
		}
		raw := p20Status(t, label, resp, http.StatusOK)
		var out handlers.CommentListResponse
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s: decoding %s: %v", label, raw, err)
		}
		return out
	}

	all := read("whole thread", fmt.Sprintf("?replies_of=%d", top.ID))
	if len(all.Comments) != 1 || all.Comments[0].ID != top.ID || len(all.Comments[0].Replies) != 2 {
		t.Fatalf("whole thread = %+v, want the root with its 2 replies", all)
	}
	if all.Comments[0].Replies[0].ID != r1.ID || all.Comments[0].Replies[1].ID != r2.ID || all.More || all.NextAfter != r2.ID {
		t.Errorf("whole thread = %+v, want r1 then r2, no more, next_after %d", all, r2.ID)
	}
	if all.Comments[0].ReplyCount != 2 || all.CommentCount != 4 {
		t.Errorf("reply_count %d, comment_count %d, want 2 and 4", all.Comments[0].ReplyCount, all.CommentCount)
	}

	tail := read("after r1", fmt.Sprintf("?replies_of=%d&reply_after=%d", top.ID, r1.ID))
	if len(tail.Comments) != 1 || len(tail.Comments[0].Replies) != 1 || tail.Comments[0].Replies[0].ID != r2.ID {
		t.Errorf("after r1 = %+v, want the root with r2 alone", tail)
	}

	for label, id := range map[string]int64{"a reply id": r1.ID, "an unknown id": 999999999} {
		got := read(label, fmt.Sprintf("?replies_of=%d", id))
		if len(got.Comments) != 0 || got.More || got.NextAfter != 0 {
			t.Errorf("%s = %+v, want an empty list", label, got)
		}
	}
	if got := read("a root with no replies", fmt.Sprintf("?replies_of=%d", other.ID)); len(got.Comments) != 1 || len(got.Comments[0].Replies) != 0 {
		t.Errorf("a root with no replies = %+v, want the root alone", got)
	}

	for _, bad := range []string{"abc", "0", "-3"} {
		p20WantError(t, "replies_of="+bad, p812Send(t, e.ben, http.MethodGet, e.commentsURL(e.report)+"?replies_of="+bad, ""),
			http.StatusBadRequest, "replies_of", "The replies_of value must be a positive number.")
		p20WantError(t, "reply_after="+bad, p812Send(t, e.ben, http.MethodGet, fmt.Sprintf("%s?replies_of=%d&reply_after=%s", e.commentsURL(e.report), top.ID, bad), ""),
			http.StatusBadRequest, "reply_after", "The reply_after value must be a positive number.")
	}
	p20WantError(t, "reply_after alone", p812Send(t, e.ben, http.MethodGet, fmt.Sprintf("%s?reply_after=%d", e.commentsURL(e.report), r1.ID), ""),
		http.StatusBadRequest, "reply_after", "The reply_after value needs replies_of.")
	p20WantError(t, "after with replies_of", p812Send(t, e.ben, http.MethodGet, fmt.Sprintf("%s?replies_of=%d&after=%d", e.commentsURL(e.report), top.ID, r1.ID), ""),
		http.StatusBadRequest, "after", "The after value cannot be used with replies_of.")
}

func TestCommentResponseOmitsIdentifiers(t *testing.T) {
	e := p812Setup(t, nil)
	top := e.mustPost(t, e.ann, e.report, "top level", 0)
	e.mustPost(t, e.ben, e.report, "a reply", top.ID)

	forbidden := []string{"account_id", "account", "email", "session", "session_id", "geohash", "geohash_cell", "cell", "user_id"}
	var walk func(label string, v any)
	walk = func(label string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				for _, bad := range forbidden {
					if strings.EqualFold(k, bad) {
						t.Errorf("%s: key %q is an identifier on the wire", label, k)
					}
				}
				walk(label+"."+k, child)
			}
		case []any:
			for i, child := range x {
				walk(fmt.Sprintf("%s[%d]", label, i), child)
			}
		}
	}

	postRaw := p20Status(t, "post", e.post(t, e.cat, e.report, "one more", 0), http.StatusCreated)
	listRaw := p20Status(t, "list", p812Send(t, e.cat, http.MethodGet, e.commentsURL(e.report), ""), http.StatusOK)
	for label, raw := range map[string][]byte{"post": postRaw, "list": listRaw} {
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			t.Fatalf("%s: decoding %s: %v", label, raw, err)
		}
		walk(label, generic)
		for _, secret := range []string{"@example.com", "geohash", "tdr1"} {
			if strings.Contains(string(raw), secret) {
				t.Errorf("%s body holds %q: %s", label, secret, raw)
			}
		}
	}

	// The exact member set of one comment and of the list.
	var l map[string]json.RawMessage
	if err := json.Unmarshal(listRaw, &l); err != nil {
		t.Fatalf("decoding the list: %v", err)
	}
	wantList := []string{"comments", "comment_count", "comments_open", "more", "next_after"}
	if len(l) != len(wantList) {
		t.Errorf("the list has %d members, want %v", len(l), wantList)
	}
	for _, k := range wantList {
		if _, ok := l[k]; !ok {
			t.Errorf("the list lacks %q", k)
		}
	}
	var comments []map[string]json.RawMessage
	if err := json.Unmarshal(l["comments"], &comments); err != nil || len(comments) == 0 {
		t.Fatalf("decoding comments: %v (%d)", err, len(comments))
	}
	// Amended 2026-10-29 by Phase 9 plan 10 (D-12): "hidden" joins the member set,
	// the flag of the Comment hidden placeholder; every other assertion stays.
	wantComment := []string{"id", "author", "author_kind", "is_own", "is_reporter", "can_delete", "viewer_liked", "viewer_flagged",
		"likes", "reply_count", "at", "deleted", "removed", "hidden", "held", "words", "replies"}
	for _, k := range wantComment {
		if _, ok := comments[0][k]; !ok {
			t.Errorf("a comment lacks %q", k)
		}
	}
	for k := range comments[0] {
		known := k == "reply_to_author"
		for _, w := range wantComment {
			if k == w {
				known = true
			}
		}
		if !known {
			t.Errorf("a comment holds the unexpected member %q", k)
		}
	}
	if _, ok := comments[0]["reply_to_author"]; ok {
		t.Errorf("a top level comment holds reply_to_author")
	}

	// A reply names the author answered, and is_reporter marks the reporter.
	got := e.list(t, e.cat, e.report)
	reply, ok := p812Find(got.Comments, top.ID)
	if !ok || len(reply.Replies) != 1 || reply.Replies[0].ReplyToAuthor != "ann_p812" {
		t.Errorf("thread = %+v, want one reply that answers ann_p812", reply)
	}
	if _, err := time.Parse(time.RFC3339, reply.At); err != nil {
		t.Errorf("at = %q, want RFC 3339: %v", reply.At, err)
	}
	rc := e.mustPost(t, e.reporter, e.report, "from the reporter", 0)
	if rc.IsReporter {
		t.Errorf("is_reporter is true on the viewer's own comment")
	}
	seen, ok := p812Find(e.list(t, e.cat, e.report).Comments, rc.ID)
	if !ok || !seen.IsReporter || seen.IsOwn {
		t.Errorf("reporter's comment as cat sees it = %+v, want is_reporter and not is_own", seen)
	}
}

func TestCommentClosedRoutes(t *testing.T) {
	e := p812Setup(t, nil)

	t.Run("a resolved report", func(t *testing.T) {
		c1 := e.mustPost(t, e.ann, e.report, "before the close", 0)
		c2 := e.mustPost(t, e.ann, e.report, "also before", 0)
		e.resolve(t, e.report)

		l := e.list(t, e.ben, e.report)
		if l.CommentsOpen || l.CommentCount != 2 {
			t.Errorf("list open=%v count=%d, want closed and still readable", l.CommentsOpen, l.CommentCount)
		}
		p20WantError(t, "post", e.post(t, e.ben, e.report, "too late", 0), http.StatusConflict, "comments", service.CommentsClosedMessage)
		p20WantError(t, "reply", e.post(t, e.ben, e.report, "too late", c1.ID), http.StatusConflict, "comments", service.CommentsClosedMessage)
		p20WantError(t, "like", e.act(t, e.ben, c1.ID, "like"), http.StatusConflict, "comments", service.CommentsClosedMessage)
		p20WantError(t, "unlike", e.act(t, e.ben, c1.ID, "unlike"), http.StatusConflict, "comments", service.CommentsClosedMessage)
		p20Status(t, "flag", e.flag(t, e.ben, c1.ID, `{"reasons":["spam"]}`), http.StatusCreated)
		p20Status(t, "delete", e.act(t, e.ann, c2.ID, "delete"), http.StatusOK)
	})

	t.Run("a report that ended longer ago than the history window", func(t *testing.T) {
		rep := submitReportGetID(t, e.reporter, e.srv.URL, p20Body("flood", "low"))
		c := e.mustPost(t, e.ann, rep, "while it was live", 0)
		e.age(t, rep, service.HistoryWindow+2*time.Hour)

		const gone = "This report is no longer available."
		p20WantError(t, "list", p812Send(t, e.ben, http.MethodGet, e.commentsURL(rep), ""), http.StatusNotFound, "report", gone)
		p20WantError(t, "post", e.post(t, e.ben, rep, "hello", 0), http.StatusNotFound, "report", gone)
		p20WantError(t, "like", e.act(t, e.ben, c.ID, "like"), http.StatusNotFound, "report", gone)
		p20WantError(t, "flag", e.flag(t, e.ben, c.ID, `{"reasons":["spam"]}`), http.StatusNotFound, "report", gone)
		// Deleting your own comment never depends on the report (review W5).
		p20Status(t, "delete", e.act(t, e.ann, c.ID, "delete"), http.StatusOK)
	})
}

func TestCommentNeedsUsernameRoute(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	e := &p812Env{srv: srv, pool: pool}
	reporter := newVerifiedClient(t, srv, mailer, "p812-nu-reporter@example.com")
	fresh := newVerifiedClient(t, srv, mailer, "p812-nu-fresh@example.com")
	rep := submitReportGetID(t, reporter, srv.URL, p20Body("flood", "low"))

	p20WantError(t, "post with no username", e.post(t, fresh, rep, "hello", 0),
		http.StatusConflict, "username", service.CommentNeedsUsernameMessage)
	if n := e.commentRows(t, rep); n != 0 {
		t.Errorf("comment rows = %d, want 0", n)
	}
	// Reading is open to an account with no username.
	p20Status(t, "list with no username", p812Send(t, fresh, http.MethodGet, e.commentsURL(rep), ""), http.StatusOK)
}

func TestCommentDeleteRoutes(t *testing.T) {
	e := p812Setup(t, nil)
	top := e.mustPost(t, e.ann, e.report, "my words", 0)
	reply := e.mustPost(t, e.ben, e.report, "an answer", top.ID)

	t.Run("someone else's comment is refused, even for the reporter", func(t *testing.T) {
		p20WantError(t, "reporter deletes a reply", e.act(t, e.reporter, reply.ID, "delete"),
			http.StatusForbidden, "comment", service.CommentNotYoursMessage)
		p20WantError(t, "cat deletes", e.act(t, e.cat, top.ID, "delete"),
			http.StatusForbidden, "comment", service.CommentNotYoursMessage)
	})

	t.Run("own comment with a reply becomes a placeholder", func(t *testing.T) {
		raw := p20Status(t, "delete", e.act(t, e.ann, top.ID, "delete"), http.StatusOK)
		var out handlers.DeleteCommentResponse
		if err := json.Unmarshal(raw, &out); err != nil || !out.Deleted {
			t.Fatalf("delete answered %s (%v), want deleted true", raw, err)
		}
		got, ok := p812Find(e.list(t, e.cat, e.report).Comments, top.ID)
		if !ok {
			t.Fatalf("the placeholder is not listed")
		}
		if !got.Deleted || got.Words != "" || got.Author != "" || got.Likes != 0 || len(got.Replies) != 1 {
			t.Errorf("placeholder = %+v, want deleted, no words, no name and its one reply", got)
		}
	})

	t.Run("a second delete and an unknown comment", func(t *testing.T) {
		p20Status(t, "repeat delete", e.act(t, e.ann, top.ID, "delete"), http.StatusOK)
		p20WantError(t, "unknown", e.act(t, e.ann, 987654321, "delete"), http.StatusNotFound, "comment", service.CommentGoneMessage)
		p20WantError(t, "bad id", p812Send(t, e.ann, http.MethodPost, e.srv.URL+"/api/comments/abc/delete", ""),
			http.StatusBadRequest, "id", "Comment id must be a number.")
	})

	t.Run("a deleted comment with no reply leaves the list", func(t *testing.T) {
		lone := e.mustPost(t, e.cat, e.report, "just me", 0)
		p20Status(t, "delete", e.act(t, e.cat, lone.ID, "delete"), http.StatusOK)
		if _, ok := p812Find(e.list(t, e.ann, e.report).Comments, lone.ID); ok {
			t.Errorf("a deleted comment with no replies is still listed")
		}
	})
}

func TestCommentFlagRoutes(t *testing.T) {
	e := p812Setup(t, nil)
	c := e.mustPost(t, e.ann, e.report, "something questionable", 0)

	raw := p20Status(t, "flag", e.flag(t, e.ben, c.ID, `{"reasons":["spam","false"],"note":"Looks copied."}`), http.StatusCreated)
	var out handlers.FlagResponse
	if err := json.Unmarshal(raw, &out); err != nil || !out.Flagged {
		t.Fatalf("flag answered %s (%v), want flagged true", raw, err)
	}
	p20WantError(t, "repeat flag", e.flag(t, e.ben, c.ID, `{"reasons":["spam"]}`),
		http.StatusConflict, "flag", "You already flagged this comment. Moderators will see it.")
	p20WantError(t, "own comment", e.flag(t, e.ann, c.ID, `{"reasons":["spam"]}`),
		http.StatusConflict, "flag", service.CommentOwnFlagMessage)
	p20WantError(t, "unknown comment", e.flag(t, e.ben, 987654321, `{"reasons":["spam"]}`),
		http.StatusNotFound, "comment", service.CommentGoneMessage)
	p20WantError(t, "no reason", e.flag(t, e.cat, c.ID, `{"reasons":[]}`), http.StatusBadRequest, "reasons", service.FlagReasonMessage)
	p20WantError(t, "bad body", e.flag(t, e.cat, c.ID, `not json`), http.StatusBadRequest, "body", "Request body is missing or malformed.")

	t.Run("a refused note answers 422 and stores nothing", func(t *testing.T) {
		e.scr.set("reject")
		defer e.scr.set("clear")
		resp := e.flag(t, e.cat, c.ID, `{"reasons":["abusive"],"note":"something hateful"}`)
		p20WantError(t, "rejected note", resp, http.StatusUnprocessableEntity, "note", service.RejectMessage(service.RejectHateful))
		var n int
		if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM comment_flags WHERE comment_id = $1`, c.ID).Scan(&n); err != nil {
			t.Fatalf("counting flags: %v", err)
		}
		if n != 1 {
			t.Errorf("flag rows = %d, want only ben's", n)
		}
	})

	t.Run("a flag never hides the comment", func(t *testing.T) {
		got, ok := p812Find(e.list(t, e.ben, e.report).Comments, c.ID)
		if !ok || got.Words != "something questionable" || !got.ViewerFlagged {
			t.Errorf("as ben sees it = %+v (listed %v), want it listed, unchanged, viewer_flagged", got, ok)
		}
		other, ok := p812Find(e.list(t, e.cat, e.report).Comments, c.ID)
		if !ok || other.ViewerFlagged {
			t.Errorf("as cat sees it = %+v (listed %v), want it listed and not flagged by cat", other, ok)
		}
	})
}

func TestCommentRoutesAreGated(t *testing.T) {
	e := p812Setup(t, nil)
	c := e.mustPost(t, e.ann, e.report, "a comment", 0)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing a cookie jar: %v", err)
	}
	anon := &http.Client{Jar: jar}
	rid := strconv.FormatInt(e.report, 10)
	cid := strconv.FormatInt(c.ID, 10)
	routes := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/reports/" + rid + "/comments", ""},
		{http.MethodPost, "/api/reports/" + rid + "/comments", `{"words":"hello"}`},
		{http.MethodPost, "/api/comments/" + cid + "/like", ""},
		{http.MethodPost, "/api/comments/" + cid + "/unlike", ""},
		{http.MethodPost, "/api/comments/" + cid + "/delete", ""},
		{http.MethodPost, "/api/comments/" + cid + "/flag", `{"reasons":["spam"]}`},
		// Amended 2026-10-30 by quick 261030-u78: Block beside Flag is gated too.
		{http.MethodPost, "/api/comments/" + cid + "/block-author", ""},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			resp := p812Send(t, anon, rt.method, e.srv.URL+rt.path, rt.body)
			p20WantError(t, "anonymous", resp, http.StatusUnauthorized, "auth", "Sign in to continue.")
		})
	}
	if n := e.commentRows(t, e.report); n != 1 {
		t.Errorf("comment rows = %d, want only the seeded one", n)
	}
}

func TestCommentRateLimit(t *testing.T) {
	e := p812Setup(t, func(d *api.Deps) {
		d.RateLimits.Comment = api.BucketLimit{Every: time.Hour, Burst: 2}
	})

	// A rejected post spends its token too: it cost a screener call.
	e.scr.set("reject")
	p20WantError(t, "first post, refused", e.post(t, e.ann, e.report, "something hateful", 0),
		http.StatusUnprocessableEntity, "words", service.RejectMessage(service.RejectHateful))
	e.scr.set("clear")
	e.mustPost(t, e.ann, e.report, "second post", 0)

	resp := e.post(t, e.ann, e.report, "third post, my words", 0)
	if resp.StatusCode != http.StatusTooManyRequests {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("third post = %d, want 429, body %s", resp.StatusCode, b)
	}
	retry := resp.Header.Get("Retry-After")
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("reading the 429: %v", err)
	}
	var body handlers.RateLimitResponse
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decoding the 429 %s: %v", raw, err)
	}
	secs, convErr := strconv.Atoi(retry)
	if convErr != nil || secs <= 0 || secs != body.RetryAfterSeconds {
		t.Errorf("Retry-After = %q and retry_after_seconds = %d, want the same positive number", retry, body.RetryAfterSeconds)
	}
	if body.Error.Field != "rate_comment" {
		t.Errorf("field = %q, want rate_comment", body.Error.Field)
	}
	if want := service.CommentLimitMessage(time.Duration(secs) * time.Second); body.Error.Message != want {
		t.Errorf("message = %q, want %q", body.Error.Message, want)
	}
	if !strings.Contains(body.Error.Message, "Your words are still here.") {
		t.Errorf("message %q does not tell the writer the words are kept", body.Error.Message)
	}
	if n := e.commentRows(t, e.report); n != 1 {
		t.Errorf("comment rows = %d, want 1 (only the second post stored)", n)
	}

	// The limit is per account.
	e.mustPost(t, e.ben, e.report, "ben is not limited", 0)
	// Likes and comment flags do not spend the comment bucket.
	c := e.list(t, e.ben, e.report).Comments[0]
	p20Status(t, "like by the limited account", e.act(t, e.ann, c.ID, "like"), http.StatusOK)
}

func TestLikeRateLimit(t *testing.T) {
	e := p812Setup(t, func(d *api.Deps) {
		d.RateLimits.Like = api.BucketLimit{Every: time.Hour, Burst: 2}
	})
	c := e.mustPost(t, e.ann, e.report, "a comment to react to", 0)

	raw := p20Status(t, "first like", e.act(t, e.ben, c.ID, "like"), http.StatusOK)
	var st handlers.LikeResponse
	if err := json.Unmarshal(raw, &st); err != nil || st.Likes != 1 || !st.ViewerLiked {
		t.Fatalf("like answered %s (%v), want 1 like and viewer_liked", raw, err)
	}
	raw = p20Status(t, "unlike", e.act(t, e.ben, c.ID, "unlike"), http.StatusOK)
	if err := json.Unmarshal(raw, &st); err != nil || st.Likes != 0 || st.ViewerLiked {
		t.Fatalf("unlike answered %s (%v), want 0 likes and not viewer_liked", raw, err)
	}

	resp := e.act(t, e.ben, c.ID, "like")
	if resp.StatusCode != http.StatusTooManyRequests {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("third like = %d, want 429, body %s", resp.StatusCode, b)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Errorf("a 429 carries no Retry-After header")
	}
	rawLimit, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("reading the 429: %v", err)
	}
	var body handlers.RateLimitResponse
	if err := json.Unmarshal(rawLimit, &body); err != nil {
		t.Fatalf("decoding the 429 %s: %v", rawLimit, err)
	}
	if body.Error.Field != "rate_like" {
		t.Errorf("field = %q, want rate_like", body.Error.Field)
	}
	if want := service.LikeLimitMessage(time.Duration(body.RetryAfterSeconds) * time.Second); body.Error.Message != want {
		t.Errorf("message = %q, want %q", body.Error.Message, want)
	}

	// An unlike and a delete spend the same bucket.
	p20WantError(t, "unlike while limited", e.act(t, e.ben, c.ID, "unlike"), http.StatusTooManyRequests, "rate_like", body.Error.Message)
	p20WantError(t, "delete while limited", e.act(t, e.ben, c.ID, "delete"), http.StatusTooManyRequests, "rate_like", body.Error.Message)
	// The limit is per account, and posting is a different bucket.
	p20Status(t, "like by another account", e.act(t, e.cat, c.ID, "like"), http.StatusOK)
	e.mustPost(t, e.ben, e.report, "posting is not limited by likes", 0)
}
