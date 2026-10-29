// person_mute_page_e2e_test.go proves the Phase 9 plan 20 Mute menu item of the
// profile page over the real router and a real Postgres (D-12, UI-SPEC 1e): the
// item is the first of the More menu on another person's page, reads Mute until
// the viewer muted them and Unmute after, never appears on your own page, and
// the muted person sees nothing of it on their own view of the page. It reuses
// the 09-07 and 09-08 helpers by name. Every package level identifier added
// here carries the p920 prefix.
package handlers_test

import (
	"net/http"
	"strings"
	"testing"
)

// TestPersonPageMuteItem checks the server drawn label and flag of the item.
func TestPersonPageMuteItem(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	viewer, _ := p907Person(t, srv, mailer, pool, "p920-viewer@example.com", "mute_viewer", true)
	owner, _ := p907Reporter(t, srv, mailer, pool, "p920-owner@example.com", "mute_owner")

	_, body, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/u/mute_owner", "")
	item, ok := p908Tag(body, "person-mute-item")
	if !ok {
		t.Fatalf("another person's page has no Mute item")
	}
	if !strings.Contains(item, `data-muted="false"`) || !strings.Contains(item, `role="menuitem"`) {
		t.Errorf("the Mute item tag = %q, want data-muted false and role menuitem", item)
	}
	if !strings.Contains(body, `data-muted="false">Mute</button>`) {
		t.Errorf("the item does not read Mute before a mute")
	}
	// The item is the first of the menu, before Remove follower and Block.
	menu := body[strings.Index(body, `id="person-more-menu"`):]
	mute, remove, block := strings.Index(menu, `id="person-mute-item"`), strings.Index(menu, `id="person-remove-follower"`), strings.Index(menu, `id="person-block"`)
	if mute < 0 || mute > remove || remove > block {
		t.Errorf("menu order is mute %d, remove follower %d, block %d, want that order", mute, remove, block)
	}

	p907Post(t, viewer, srv, "/api/people/mute_owner/mute", http.StatusNoContent)
	_, muted, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/u/mute_owner", "")
	if !strings.Contains(muted, `data-muted="true">Unmute</button>`) {
		t.Errorf("after a mute the item does not read Unmute")
	}
	if p908Shown(muted, "person-blocked") {
		t.Errorf("a mute must not draw the blocked face")
	}

	// Their own view of the page carries no Mute item, and the muter's page
	// shows no sign of the mute to them.
	_, own, _ := p907Do(t, owner, http.MethodGet, srv.URL+"/u/mute_owner", "")
	if strings.Contains(own, `id="person-mute-item"`) {
		t.Errorf("your own page must not draw the Mute item")
	}
	_, theirView, _ := p907Do(t, owner, http.MethodGet, srv.URL+"/u/mute_viewer", "")
	if strings.Contains(theirView, `data-muted="true"`) {
		t.Errorf("the muted person's view of the muter's page shows a mute")
	}

	p907Post(t, viewer, srv, "/api/people/mute_owner/unmute", http.StatusNoContent)
	_, back, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/u/mute_owner", "")
	if !strings.Contains(back, `data-muted="false">Mute</button>`) {
		t.Errorf("after an unmute the item does not read Mute")
	}
}
