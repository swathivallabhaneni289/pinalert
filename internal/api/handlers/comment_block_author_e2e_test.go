// comment_block_author_e2e_test.go proves POST /api/comments/{id}/block-author
// end to end over the real router and a real Postgres (quick 261030-u78,
// WR-E04, D-12): any commenter can be blocked from a comment, the route
// reveals nothing (204, or one 404 with the same bytes as a like on a deleted
// comment), and it spends the per account people action bucket.
//
// Every person here is made up and uses example.com.
package handlers_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/service"
	"pinalert/internal/testutil"
)

func u78Blocks(t *testing.T, e *p812Env) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM person_blocks`).Scan(&n); err != nil {
		t.Fatalf("counting blocks: %v", err)
	}
	return n
}

// u78Report stores a report the reader cannot or can see, around now.
func u78Report(t *testing.T, e *p812Env, category, moderation string, createdAgo, expiresIn time.Duration) int64 {
	t.Helper()
	now := time.Now()
	return testutil.InsertStoredReport(t, e.pool, testutil.StoredReport{
		SessionID: "u78-seeded-session", Category: category, Severity: "low", Description: "water",
		Latitude: 16.5062, Longitude: 80.6480,
		CreatedAt: now.Add(-createdAgo), ExpiresAt: now.Add(expiresIn), ModerationState: moderation,
	})
}

func TestCommentBlockAuthorRoute(t *testing.T) {
	e := p812Setup(t, func(d *api.Deps) { d.RateLimits.PeopleAction = e2eGenerousBucket })
	annComment := e.mustPost(t, e.ann, e.report, "ann words", 0)

	resp := e.act(t, e.ben, annComment.ID, "block-author")
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if body := p20Status(t, "block ann", resp, http.StatusNoContent); len(body) != 0 {
		t.Errorf("body = %q, want empty", body)
	}
	p20Status(t, "block ann again", e.act(t, e.ben, annComment.ID, "block-author"), http.StatusNoContent)

	if got := u78Blocks(t, e); got != 1 {
		t.Errorf("block rows = %d, want 1 after a repeat", got)
	}
	_, list := p907Get(t, e.ben, e.srv, "/api/me/blocked", http.StatusOK)
	if !strings.Contains(listString(list), "ann_p812") {
		t.Errorf("Settings list = %v, want ann_p812 (a private commenter)", list)
	}
	if _, found := p812Find(e.list(t, e.ben, e.report).Comments, annComment.ID); found {
		t.Error("ben still sees ann's comment")
	}
	if c, found := p812Find(e.list(t, e.cat, e.report).Comments, annComment.ID); !found || c.Words != "ann words" {
		t.Error("cat lost ann's comment")
	}

	t.Run("a commenter who blocked the caller still answers 204", func(t *testing.T) {
		catComment := e.mustPost(t, e.cat, e.report, "cat words", 0)
		reporterComment := e.mustPost(t, e.reporter, e.report, "reporter words", 0)
		// cat blocks the reporter first; the reporter blocking cat afterwards
		// must answer like any other block and so reveal nothing.
		p20Status(t, "cat blocks reporter", e.act(t, e.cat, reporterComment.ID, "block-author"), http.StatusNoContent)
		p20Status(t, "reporter blocks cat", e.act(t, e.reporter, catComment.ID, "block-author"), http.StatusNoContent)
	})

	t.Run("a malformed id is a 400 on field id", func(t *testing.T) {
		resp := p812Send(t, e.ben, http.MethodPost, e.srv.URL+"/api/comments/abc/block-author", "")
		p20WantError(t, "malformed id", resp, http.StatusBadRequest, "id", "Comment id must be a number.")
	})
}

// listString flattens a decoded JSON object to text for a contains check.
func listString(obj map[string]any) string {
	var b strings.Builder
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for _, y := range x {
				walk(y)
			}
		case []any:
			for _, y := range x {
				walk(y)
			}
		case string:
			b.WriteString(x + " ")
		}
	}
	walk(obj)
	return b.String()
}

func TestCommentBlockAuthorUniformNotFound(t *testing.T) {
	e := p812Setup(t, func(d *api.Deps) { d.RateLimits.PeopleAction = e2eGenerousBucket })

	annDeleted := e.mustPost(t, e.ann, e.report, "to be deleted", 0)
	p20Status(t, "ann deletes", e.act(t, e.ann, annDeleted.ID, "delete"), http.StatusOK)
	benOwn := e.mustPost(t, e.ben, e.report, "ben words", 0)

	annID := e.accountID(t, p812AnnEmail)
	withheld := u78Report(t, e, "flood", "removed", time.Hour, time.Hour)
	ended := u78Report(t, e, "flood", "clear", 10*24*time.Hour, -8*24*time.Hour)
	rescue := u78Report(t, e, string(service.CategoryRescueNeeded), "clear", 2*time.Hour, -time.Hour)
	gone := testutil.SeedPerson(t, e.pool, "u78-route-gone@example.com", "gone_u78", true)
	testutil.MustExec(t, e.pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, gone)

	seed := func(report, account int64, state string) int64 {
		return testutil.SeedComment(t, e.pool, testutil.CommentSeed{ReportID: report, AccountID: account, State: state})
	}
	cases := []struct {
		name string
		id   int64
	}{
		{"an unknown comment", 987654321},
		{"ben's own comment", benOwn.ID},
		{"a deleted comment", annDeleted.ID},
		{"a removed comment", seed(e.report, annID, "removed")},
		{"ann's held comment seen by ben", seed(e.report, annID, "pending")},
		{"a comment on a withheld report", seed(withheld, annID, "clear")},
		{"a comment on a report that ended before the history window", seed(ended, annID, "clear")},
		{"a comment on a finished Rescue report", seed(rescue, annID, "clear")},
		{"a comment by a deleted account", seed(e.report, gone, "clear")},
	}

	likeStatus, likeBody, likeHeader := p907Do(t, e.ben, http.MethodPost, e.commentURL(annDeleted.ID, "like"), "")
	if likeStatus != http.StatusNotFound {
		t.Fatalf("like on a deleted comment: status %d, body %s", likeStatus, likeBody)
	}
	if field, message := p907ErrorOf(t, likeBody); field != "comment" || message != service.CommentGoneMessage {
		t.Fatalf("like body error = (%q, %q)", field, message)
	}

	before := u78Blocks(t, e)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body, header := p907Do(t, e.ben, http.MethodPost, e.commentURL(tc.id, "block-author"), "")
			if status != http.StatusNotFound {
				t.Fatalf("status = %d, want 404, body %s", status, body)
			}
			if body != likeBody {
				t.Errorf("body = %s, want the same bytes as the like route: %s", body, likeBody)
			}
			if field, message := p907ErrorOf(t, body); field != "comment" || message != service.CommentGoneMessage {
				t.Errorf("error = (%q, %q), want the comment is gone words", field, message)
			}
			if got, want := header.Get("Content-Type"), likeHeader.Get("Content-Type"); got != want {
				t.Errorf("Content-Type = %q, want %q", got, want)
			}
			if cc := header.Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", cc)
			}
		})
	}
	if got := u78Blocks(t, e); got != before {
		t.Errorf("block rows went from %d to %d across the refusals", before, got)
	}
}

func TestCommentBlockAuthorRateLimit(t *testing.T) {
	e := p812Setup(t, func(d *api.Deps) {
		d.RateLimits.PeopleAction = api.BucketLimit{Every: time.Hour, Burst: 2}
	})
	annComment := e.mustPost(t, e.ann, e.report, "ann words", 0)

	for i := 1; i <= 2; i++ {
		if status, body, _ := p907Do(t, e.ben, http.MethodPost, e.commentURL(annComment.ID, "block-author"), ""); status == http.StatusTooManyRequests {
			t.Fatalf("call %d was already limited, the setup spent the bucket: %s", i, body)
		} else if status != http.StatusNoContent {
			t.Fatalf("call %d: status = %d, body %s", i, status, body)
		}
	}
	status, body, header := p907Do(t, e.ben, http.MethodPost, e.commentURL(annComment.ID, "block-author"), "")
	if status != http.StatusTooManyRequests {
		t.Fatalf("third call: status = %d, want 429, body %s", status, body)
	}
	if header.Get("Retry-After") == "" {
		t.Error("429 without Retry-After")
	}
	// One shared bucket with the profile block.
	if status, body, _ := p907Do(t, e.ben, http.MethodPost, e.srv.URL+"/api/people/cat_p812/block", ""); status != http.StatusTooManyRequests {
		t.Errorf("profile block: status = %d, want 429 from the shared bucket, body %s", status, body)
	}
	// Another account has its own.
	p20Status(t, "cat is not limited", e.act(t, e.cat, annComment.ID, "block-author"), http.StatusNoContent)
}
