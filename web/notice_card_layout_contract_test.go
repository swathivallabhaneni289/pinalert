package web

import (
	"strings"
	"testing"
)

// TestNoticeCardAsksTheLayout pins review finding WR-W02 (Phase 4 web
// review). The phone layout shows the list under the map whatever data-view
// says, and data-view stays at map on a phone, so a listPaneHidden that read
// the attribute treated the list as hidden on every phone and the notice
// card covered list rows that already showed the same queue or stale line.
// listPaneHidden now asks the layout, and the card is synced again on resize
// because the answer depends on the layout.
func TestNoticeCardAsksTheLayout(t *testing.T) {
	raw, err := StaticFS.ReadFile("static/js/feed.js")
	if err != nil {
		t.Fatalf("reading feed.js: %v", err)
	}
	code := stripJSComments(string(raw))
	hidden := jsFunctionBody(t, code, "listPaneHidden()")
	for _, want := range []string{".pane--list", "getClientRects()"} {
		if !strings.Contains(hidden, want) {
			t.Errorf("listPaneHidden must ask the layout of the list pane, lacks %q: %q", want, hidden)
		}
	}
	for _, banned := range []string{"data-view", "narrowQuery"} {
		if strings.Contains(hidden, banned) {
			t.Errorf("listPaneHidden reads %q; the phone shows the list whatever the attribute or width says", banned)
		}
	}
	if !strings.Contains(code, "window.addEventListener('resize', syncNoticeCard)") {
		t.Errorf("feed.js must sync the notice card on resize")
	}
}
