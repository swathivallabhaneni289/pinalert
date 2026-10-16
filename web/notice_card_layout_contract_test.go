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
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-03, D-01): the
// card element moved into the list pane (the rule is unchanged: it still asks
// the layout, and on a phone the list is shown, so the queue and stale rows
// stay with the list's own lines and the slow row is the one the card holds).
// TestPhoneNoticeRowsSitInsideTheSheet pins the new place. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
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

// TestPhoneNoticeRowsSitInsideTheSheet pins D-03 and D-01 (UI-SPEC 2f): the
// notice card is a child of the list pane right after the list head, with the
// same three rows in the same order, and on a phone it is a static strip inside
// the sheet, never fixed over the list, while the laptop keeps its fixed bottom
// left place. The card's rows, words and ids are unchanged.
func TestPhoneNoticeRowsSitInsideTheSheet(t *testing.T) {
	tmpl := p13ReadTemplate(t, "index.html.tmpl")
	paneAt := strings.Index(tmpl, `<section class="pane pane--list" id="report-sheet"`)
	headAt := strings.Index(tmpl, `class="list-head"`)
	cardAt := strings.Index(tmpl, `id="notice-card"`)
	listAt := strings.Index(tmpl, `<ol id="report-list">`)
	if !(paneAt != -1 && headAt > paneAt && cardAt > headAt && cardAt < listAt) {
		t.Fatalf("the card must be inside the list pane after the list head (%d, %d, %d, %d)", paneAt, headAt, cardAt, listAt)
	}
	// Nothing sits between the list head and the card but the head's own end,
	// so the card is the first thing under the head.
	between := tmpl[headAt:cardAt]
	if strings.Contains(between, `id="place-line"`) || strings.Contains(between, `id="feed-stale"`) {
		t.Errorf("the card must come right after the list head, found lines between them")
	}
	p13Ordered(t, tmpl, "notice rows", `id="notice-slow"`, `id="notice-queue"`, `id="notice-stale"`)

	// A phone: static, inside the sheet's flow, full width.
	card := p31mRule(t, "static/css/phone.css", phoneBlockMedia, ".pane--list > .notice-card")
	p16MustDecl(t, "phone notice card", card, map[string]string{
		"position": "static", "flex": "none", "max-width": "none", "max-height": "none",
	})
	// A laptop: the built fixed place, bottom left over the map.
	base := p31mRule(t, "static/css/chrome.css", "", ".notice-card")
	p16MustDecl(t, "laptop notice card", base, map[string]string{"position": "fixed", "z-index": "1000"})
}
