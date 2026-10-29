// people_changed_e2e_test.go proves the review fix IN-B03 (Phase 9 round 2): the
// 204 of Unmute and Unblock says, in the Pinalert-Changed header, whether a row
// was deleted, and says the same "0" for a name nobody holds and for a person the
// caller never muted or blocked, so the route still reveals nothing. Every
// package level identifier added here carries the p9ch prefix.
package handlers_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"pinalert/internal/api/handlers"
)

// TestChangedHeaderIsAgreedInOnePlace pins that the browser script reads the
// same header name the handler writes and that the swagger text names it.
func TestChangedHeaderIsAgreedInOnePlace(t *testing.T) {
	script, err := os.ReadFile("../../../web/static/js/people-list.js")
	if err != nil {
		t.Fatalf("reading people-list.js: %v", err)
	}
	if !strings.Contains(string(script), "var CHANGED_HEADER = '"+handlers.ChangedHeader+"';") {
		t.Errorf("people-list.js does not read the header %q the handler writes", handlers.ChangedHeader)
	}
	spec, err := os.ReadFile("../../../docs/swagger.yaml")
	if err != nil {
		t.Fatalf("reading swagger.yaml: %v", err)
	}
	if n := strings.Count(string(spec), "            "+handlers.ChangedHeader+":"); n != 2 {
		t.Errorf("swagger.yaml documents %s on %d answers, want 2 (unmute and unblock)", handlers.ChangedHeader, n)
	}
}

func TestUnmuteAndUnblockSayWhetherARowChanged(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	me, meID := p907Person(t, srv, mailer, pool, "p9ch-me@example.com", "ch_me", true)
	_, _ = p907Reporter(t, srv, mailer, pool, "p9ch-tgt@example.com", "ch_tgt")
	_, _ = p907Person(t, srv, mailer, pool, "p9ch-tgt2@example.com", "ch_tgt2", true)

	rows := func(table, col string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE "+col+" = $1", meID).Scan(&n); err != nil {
			t.Fatalf("counting %s: %v", table, err)
		}
		return n
	}
	changedHeader := func(path string) string {
		t.Helper()
		status, body, header := p907Do(t, me, http.MethodPost, srv.URL+path, "")
		if status != http.StatusNoContent || body != "" {
			t.Fatalf("POST %s: status %d body %q, want 204 and no body", path, status, body)
		}
		return header.Get(handlers.ChangedHeader)
	}

	for _, tc := range []struct {
		action, undo, table, col, old, renamed string
	}{
		{"mute", "unmute", "person_mutes", "muter_id", "ch_tgt", "ch_tgt_new"},
		{"block", "unblock", "person_blocks", "blocker_id", "ch_tgt2", "ch_tgt2_new"},
	} {
		p907Post(t, me, srv, "/api/people/"+tc.old+"/"+tc.action, http.StatusNoContent)
		if got := rows(tc.table, tc.col); got != 1 {
			t.Fatalf("%s: rows after %s = %d, want 1", tc.undo, tc.action, got)
		}

		// A name nobody holds and a person never acted on both answer 0.
		if got := changedHeader("/api/people/ch_nobody_at_all/" + tc.undo); got != "0" {
			t.Errorf("%s of a name nobody holds: %s = %q, want 0", tc.undo, handlers.ChangedHeader, got)
		}

		// The person renames since the list was read: the old name now matches
		// nobody, the answer is 204 with 0, and the row is still there.
		if _, err := pool.Exec(context.Background(), "UPDATE accounts SET username = $2 WHERE username = $1", tc.old, tc.renamed); err != nil {
			t.Fatalf("renaming %s: %v", tc.old, err)
		}
		if got := changedHeader("/api/people/" + tc.old + "/" + tc.undo); got != "0" {
			t.Errorf("%s by the old name: %s = %q, want 0", tc.undo, handlers.ChangedHeader, got)
		}
		if got := rows(tc.table, tc.col); got != 1 {
			t.Errorf("%s by the old name removed the row: rows = %d, want 1", tc.undo, got)
		}

		// By the new name it deletes the row and answers 1; a repeat answers 0.
		if got := changedHeader("/api/people/" + tc.renamed + "/" + tc.undo); got != "1" {
			t.Errorf("%s by the new name: %s = %q, want 1", tc.undo, handlers.ChangedHeader, got)
		}
		if got := rows(tc.table, tc.col); got != 0 {
			t.Errorf("%s by the new name left %d rows", tc.undo, got)
		}
		if got := changedHeader("/api/people/" + tc.renamed + "/" + tc.undo); got != "0" {
			t.Errorf("a repeated %s: %s = %q, want 0", tc.undo, handlers.ChangedHeader, got)
		}
	}

	// The other body free people routes carry no such header, and a malformed
	// name is still the uniform 404 without it.
	if _, _, header := p907Do(t, me, http.MethodPost, srv.URL+"/api/people/ch_nobody_at_all/unfollow", ""); header.Get(handlers.ChangedHeader) != "" {
		t.Errorf("unfollow carries %s", handlers.ChangedHeader)
	}
	for _, path := range []string{"/unmute", "/unblock"} {
		status, _, header := p907Do(t, me, http.MethodPost, srv.URL+"/api/people/bad%21name"+path, "")
		if status != http.StatusNotFound || header.Get(handlers.ChangedHeader) != "" {
			t.Errorf("malformed name on %s: status %d header %q, want 404 and none", path, status, header.Get(handlers.ChangedHeader))
		}
	}
}
