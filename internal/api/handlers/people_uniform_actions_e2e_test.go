// people_uniform_actions_e2e_test.go pins IN-E07 of the Phase 9 code review
// (2026-10-30): the uniform 404 holds on the routes added after follow too. An
// unknown username, a private person, a pending username, a deleted account and a
// person who blocked the caller answer block, mute, notify and rating with byte
// equal 404 bodies. It runs on the real router and a real Postgres.
package handlers_test

import (
	"context"
	"net/http"
	"testing"

	"pinalert/internal/service"
	"pinalert/internal/testutil"
)

func TestProfileNotFoundIsUniformOnTheActionRoutes(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	viewer, viewerID := p907Person(t, srv, mailer, pool, "in-e07-viewer@example.com", "in_e07_viewer", false)

	testutil.SeedPerson(t, pool, "in-e07-private@example.com", "in_e07_private", false)
	pendingID := testutil.SeedPerson(t, pool, "in-e07-pending@example.com", "in_e07_pending", true)
	testutil.SeedUsername(t, pool, pendingID, "in_e07_pending", "pending")
	deletedID := testutil.SeedPerson(t, pool, "in-e07-deleted@example.com", "in_e07_deleted", true)
	if _, err := pool.Exec(context.Background(), "UPDATE accounts SET deleted_at = now() WHERE id = $1", deletedID); err != nil {
		t.Fatalf("deleting the account: %v", err)
	}
	blockerID := testutil.SeedPerson(t, pool, "in-e07-blocker@example.com", "in_e07_blocker", true)
	testutil.SeedBlock(t, pool, blockerID, viewerID)

	posts := []struct{ suffix, body string }{
		{"/block", ""},
		{"/mute", ""},
		{"/notify", `{"on":true}`},
		{"/rating", `{"stars":3}`},
	}
	for _, p := range posts {
		want := ""
		for _, name := range []string{"in_e07_nobody", "in_e07_private", "in_e07_pending", "in_e07_deleted", "in_e07_blocker"} {
			status, body, _ := p907Do(t, viewer, http.MethodPost, srv.URL+"/api/people/"+name+p.suffix, p.body)
			if status != http.StatusNotFound {
				t.Fatalf("POST %s for %s: status = %d, want 404, body %s", p.suffix, name, status, body)
			}
			if want == "" {
				want = body
			}
			if body != want {
				t.Fatalf("POST %s for %s: body differs from the uniform 404:\n got %q\nwant %q", p.suffix, name, body, want)
			}
		}
		if field, message := p907ErrorOf(t, want); field != "username" || message != service.ProfileNotFoundMessage {
			t.Errorf("POST %s: the uniform 404 says field %q message %q", p.suffix, field, message)
		}
	}
}
