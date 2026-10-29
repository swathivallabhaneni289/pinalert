// comment_count_e2e_test.go is Phase 8 plan 10 (PAGE-02, SC4, D-03, D-15):
// comment_count and comments_open on the feed and the report link answer, and
// the commenter's follow state, proven over the real router and a real Postgres.
// It runs only on a disposable database (p8_10_a), through newE2EServerWith like
// every e2e here. Every package level identifier carries the p810 prefix.
package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"pinalert/internal/testutil"
)

// p810Int requires key to be a JSON integer on obj and returns it.
func p810Int(t *testing.T, label string, obj map[string]json.RawMessage, key string) int {
	t.Helper()
	raw, ok := obj[key]
	if !ok {
		t.Fatalf("%s: no %s key", label, key)
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatalf("%s: %s = %s, want a JSON integer", label, key, raw)
	}
	return n
}

// p810Bool requires key to be a JSON boolean on obj and returns it.
func p810Bool(t *testing.T, label string, obj map[string]json.RawMessage, key string) bool {
	t.Helper()
	raw, ok := obj[key]
	if !ok {
		t.Fatalf("%s: no %s key", label, key)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("%s: %s = %s, want a JSON boolean", label, key, raw)
	}
	return b
}

// TestCommentCountOnFeed: a report holds two clear comments, one clear reply,
// one pending comment and one deleted comment. Every reader, the pending
// comment's own author included, is told 3 on the feed and on the report link
// (T-08-46, D-03). A report with no comments answers 0.
func TestCommentCountOnFeed(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	a := newVerifiedClient(t, srv, mailer, "p810-count-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p810-count-b@example.com")
	c := newVerifiedClient(t, srv, mailer, "p810-count-c@example.com")
	idA := p20AccountID(t, pool, "p810-count-a@example.com")
	idB := p20AccountID(t, pool, "p810-count-b@example.com")
	idC := p20AccountID(t, pool, "p810-count-c@example.com")

	withComments := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	quiet := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))

	first := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: withComments, AccountID: idC})
	testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: withComments, AccountID: idA})
	testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: withComments, AccountID: idC, ParentID: first})
	testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: withComments, AccountID: idB, State: "pending"})
	testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: withComments, AccountID: idC, Deleted: true})

	readers := []struct {
		name   string
		client *http.Client
	}{
		{"the report author", a},
		{"the pending comment's own author", b},
		{"a commenter", c},
	}
	for _, tc := range readers {
		t.Run(tc.name, func(t *testing.T) {
			objects, focus := p32ViewerFeed(t, tc.name, tc.client, srv.URL, withComments)
			obj, ok := objects[withComments]
			if !ok {
				t.Fatalf("report %d missing from the feed", withComments)
			}
			for where, o := range map[string]map[string]json.RawMessage{"feed": obj, "focus": focus} {
				if got := p810Int(t, tc.name+" ("+where+")", o, "comment_count"); got != 3 {
					t.Errorf("%s (%s): comment_count = %d, want 3", tc.name, where, got)
				}
			}
			empty, ok := objects[quiet]
			if !ok {
				t.Fatalf("report %d missing from the feed", quiet)
			}
			if got := p810Int(t, tc.name+" (quiet)", empty, "comment_count"); got != 0 {
				t.Errorf("%s: a report with no comments answers comment_count %d, want 0", tc.name, got)
			}
		})
	}
}

// TestCommentsOpenOnFeed: an active report answers comments_open true and a
// resolved one false, written by the server (D-15). An account whose only link
// to a report is a clear comment follows it; the follow ends once that comment
// is deleted (review W8, PAGE-02).
func TestCommentsOpenOnFeed(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	a := newVerifiedClient(t, srv, mailer, "p810-open-a@example.com")
	d := newVerifiedClient(t, srv, mailer, "p810-open-d@example.com")
	idD := p20AccountID(t, pool, "p810-open-d@example.com")

	active := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	closing := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))

	objects, focus := p32ViewerFeed(t, "active", a, srv.URL, active)
	if !p810Bool(t, "active (feed)", objects[active], "comments_open") {
		t.Error("an active report on the feed must answer comments_open true")
	}
	if !p810Bool(t, "active (focus)", focus, "comments_open") {
		t.Error("an active report on the report link must answer comments_open true")
	}

	resp := postVote(t, a, srv.URL, closing, "resolve", map[string]any{"latitude": p20Lat, "longitude": p20Lon})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resolve: status %d, want 200", resp.StatusCode)
	}
	_, resolvedFocus := p32ViewerFeed(t, "resolved", a, srv.URL, closing)
	if p810Bool(t, "resolved (focus)", resolvedFocus, "comments_open") {
		t.Error("a resolved report must answer comments_open false")
	}

	// The commenter's only link to the report is one clear comment.
	commentID := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: active, AccountID: idD})
	objects, focus = p32ViewerFeed(t, "commenter", d, srv.URL, active)
	if !p810Bool(t, "commenter (feed)", objects[active], "viewer_follows") {
		t.Error("an account that only commented must follow the report on the feed")
	}
	if !p810Bool(t, "commenter (focus)", focus, "viewer_follows") {
		t.Error("an account that only commented must follow the report on the report link")
	}

	testutil.MustExec(t, pool, `UPDATE comments SET deleted_at = now(), body = '' WHERE id = $1`, commentID)
	objects, focus = p32ViewerFeed(t, "deleted", d, srv.URL, active)
	if p810Bool(t, "deleted (feed)", objects[active], "viewer_follows") {
		t.Error("deleting the only comment must end the follow on the feed (review W8)")
	}
	if p810Bool(t, "deleted (focus)", focus, "viewer_follows") {
		t.Error("deleting the only comment must end the follow on the report link (review W8)")
	}
}
