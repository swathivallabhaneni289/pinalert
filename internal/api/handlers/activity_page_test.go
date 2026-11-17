package handlers

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// These tests belong to the My activity view model. They began as quick task
// 261013-wm5's six tab tests and were amended 2026-10-16 by Phase 3.4 (owner
// decision, 03.4-CONTEXT D-18 to D-21): the six tabs became five parts, the
// token and hidden machinery went, and the old tab values became read only
// aliases. The membership of one card in exactly one part, the card order (W-05),
// the record card source (W-10) and the words each kind of card carries (W-06)
// are kept. The Phase 3.1 and Phase 4 hands-on tests are walked on this layout
// after Phase 3.4 is built.
//
// Amended 2026-11-17 (quick 261117-vuf, owner, 7.1 UAT): Posted left (your
// posts are on your profile) and the names are one word, Confirmed, Disputed
// and Ended. The account's own posts are no longer cards, so the record card
// and the poster's Reopen left with them; the tests that pinned those now pin
// that posts are not listed and no card offers Reopen. Updates is still built
// for its own screen, and activityShownParts keeps it out of the list.

var wm5Now = time.Date(2031, 3, 1, 12, 0, 0, 0, time.UTC)

// wm5Post builds one of the account's own posts with the given visibility,
// created two hours before wm5Now. expired moves its expiry into the past.
func wm5Post(id int64, vis service.Visibility, expired bool) service.ActivityReport {
	expires := wm5Now.Add(6 * time.Hour)
	if expired {
		expires = wm5Now.Add(-time.Minute)
	}
	return service.ActivityReport{
		Row: sqlcgen.ReportsByAccountRow{
			ID:          id,
			Category:    string(service.CategoryFlood),
			Severity:    string(service.SeverityMedium),
			Description: "post description",
			CreatedAt:   wm5Now.Add(-2 * time.Hour),
			ExpiresAt:   expires,
		},
		Category:   service.CategoryFlood,
		Severity:   service.SeverityMedium,
		Visibility: vis,
		Reason:     service.ResolveReason("test"),
		// The service sets the full budget on every post that has no undo
		// rows (plan 03.1-07), so the fixture does too.
		UndosLeft: service.UndoLimit,
	}
}

// wm5Vote builds a voted report with the given vote value, voted ten minutes
// before wm5Now.
func wm5Vote(id int64, value service.VoteValue, vis service.Visibility, expired bool) service.VotedReport {
	ar := wm5Post(id, vis, expired)
	ar.Row.Description = "voted description"
	return service.VotedReport{ActivityReport: ar, Vote: value, VotedAt: wm5Now.Add(-10 * time.Minute)}
}

// wm5Cards builds the page view for the given part key ("" is the list).
func wm5Cards(t *testing.T, page service.ActivityPage, part string) activityPageView {
	t.Helper()
	view, err := buildActivityPage(page, part, wm5Now)
	if err != nil {
		t.Fatalf("buildActivityPage: %v", err)
	}
	return view
}

// wm5AllRows flattens the rows of every part, in part order.
func wm5AllRows(view activityPageView) []profileReport {
	var all []profileReport
	for _, p := range view.Parts {
		all = append(all, p.Rows...)
	}
	return all
}

func wm5Card(t *testing.T, view activityPageView, id int64) profileReport {
	t.Helper()
	for _, c := range wm5AllRows(view) {
		if c.ReportID == id {
			return c
		}
	}
	t.Fatalf("no card for report %d", id)
	return profileReport{}
}

// wm5Part returns the part with the given key.
func wm5Part(t *testing.T, view activityPageView, key string) activityPart {
	t.Helper()
	for _, p := range view.Parts {
		if p.Key == key {
			return p
		}
	}
	t.Fatalf("no part %q", key)
	return activityPart{}
}

func TestParseActivityPart(t *testing.T) {
	cases := []struct{ part, tab, want string }{
		// The four part values are honoured exactly.
		{"updates", "", "updates"},
		{"confirmed", "", "confirmed"},
		{"disputed", "", "disputed"},
		{"ended", "", "ended"},
		// Posted left (quick 261117-vuf): its value and its old tab alias
		// land on the list.
		{"posted", "", ""},
		{"", "posts", ""},
		// The other old tab values are read only aliases.
		{"", "votes", "confirmed"},
		{"", "disputes", "disputed"},
		// All, resolved and not-resolved land on the list.
		{"", "all", ""},
		{"", "resolved", ""},
		{"", "not-resolved", ""},
		// Empty, unknown, upper case and padded values give the list.
		{"", "", ""},
		{"nonsense", "", ""},
		{"", "nonsense", ""},
		{"Ended", "", ""},
		{"CONFIRMED", "", ""},
		{"ended ", "", ""},
		{" ended", "", ""},
		{"", "Votes", ""},
		{"", "votes ", ""},
		{"disputed&part=ended", "", ""},
		// The part wins over the tab when both are sent.
		{"ended", "votes", "ended"},
		{"updates", "votes", "updates"},
	}
	for _, c := range cases {
		if got := parseActivityPart(c.part, c.tab); got != c.want {
			t.Errorf("parseActivityPart(%q, %q) = %q, want %q", c.part, c.tab, got, c.want)
		}
	}
}

// TestActivityPartsInOrder pins the parts, their one word names (quick
// 261117-vuf), their left lines from UI-SPEC 3a, their hub icons and their
// addresses. Updates is built first for its own screen.
func TestActivityPartsInOrder(t *testing.T) {
	view := wm5Cards(t, service.ActivityPage{}, "")
	var keys, names, lines, icons, hrefs []string
	for _, p := range view.Parts {
		keys = append(keys, p.Key)
		names = append(names, p.Name)
		lines = append(lines, p.Line)
		icons = append(icons, p.Icon)
		hrefs = append(hrefs, p.Href)
	}
	if want := []string{"updates", "confirmed", "disputed", "ended"}; !slices.Equal(keys, want) {
		t.Errorf("keys = %v, want %v", keys, want)
	}
	if want := []string{"Updates", "Confirmed", "Disputed", "Ended"}; !slices.Equal(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
	wantLines := []string{
		"Changes on reports you posted or voted on, newest first.",
		"Reports you said are true.",
		"Reports you said are wrong.",
		"Everyone's reports near you that ended in the last 7 days.",
	}
	if !slices.Equal(lines, wantLines) {
		t.Errorf("lines = %v, want %v", lines, wantLines)
	}
	if want := []string{"bell", "check-circle", "circle-x", "history"}; !slices.Equal(icons, want) {
		t.Errorf("icons = %v, want %v", icons, want)
	}
	wantHrefs := []string{"/profile?part=updates", "/profile?part=confirmed", "/profile?part=disputed", "/profile?part=ended"}
	if !slices.Equal(hrefs, wantHrefs) {
		t.Errorf("hrefs = %v, want %v", hrefs, wantHrefs)
	}
	for _, p := range view.Parts {
		if p.EmptyTitle == "" || p.EmptyText == "" {
			t.Errorf("part %q has no empty state words", p.Key)
		}
	}
}

// TestActivityShownParts pins what each screen draws (quick 261117-vuf): the
// list and the three part screens draw exactly Confirmed, Disputed and Ended in
// that order, and the Updates screen draws Updates alone.
func TestActivityShownParts(t *testing.T) {
	keysOf := func(parts []activityPart) []string {
		var keys []string
		for _, p := range parts {
			keys = append(keys, p.Key)
		}
		return keys
	}
	three := []string{"confirmed", "disputed", "ended"}
	for _, current := range []string{"", "confirmed", "disputed", "ended"} {
		view := wm5Cards(t, wm5MixedPage(), current)
		if got := keysOf(activityShownParts(view.Parts, view.CurrentPart)); !slices.Equal(got, three) {
			t.Errorf("screen %q draws %v, want %v", current, got, three)
		}
	}
	view := wm5Cards(t, wm5MixedPage(), "updates")
	if got := keysOf(activityShownParts(view.Parts, view.CurrentPart)); !slices.Equal(got, []string{"updates"}) {
		t.Errorf("the Updates screen draws %v, want only updates", got)
	}
	if view.CurrentName() != "Updates" {
		t.Errorf("the Updates screen is named %q, want Updates", view.CurrentName())
	}
}

// TestActivityPartEmptyStates pins the empty titles and texts of UI-SPEC 3c.
func TestActivityPartEmptyStates(t *testing.T) {
	view := wm5Cards(t, service.ActivityPage{}, "")
	want := map[string][2]string{
		"updates":   {"No updates yet", "Changes on reports you posted or voted on will show here."},
		"confirmed": {"No confirmations yet", "Reports you confirm will appear here."},
		"disputed":  {"No disputes yet", "Reports you dispute will appear here."},
		"ended":     {"No ended reports nearby", "Nothing in this area was resolved or faded out in the last 7 days."},
	}
	for _, p := range view.Parts {
		if got := [2]string{p.EmptyTitle, p.EmptyText}; got != want[p.Key] {
			t.Errorf("part %q empty = %v, want %v", p.Key, got, want[p.Key])
		}
	}
	if len(wm5AllRows(view)) != 0 {
		t.Errorf("empty account has %d rows", len(wm5AllRows(view)))
	}
}

// TestActivityPartMembership is W-03 rewritten for the parts (D-21): a confirm
// vote is only in Confirmed and a dispute only in Disputed, whatever the
// report's state, and the account's own post is in no part at all (quick
// 261117-vuf: it is on the profile). Counts equal row counts; Updates and Ended
// hold no server rows. It also pins the status word of each state.
func TestActivityPartMembership(t *testing.T) {
	type tc struct {
		name       string
		vote       *service.VotedReport
		wantPart   string
		wantKind   string
		wantStatus string
	}
	vote := func(v service.VoteValue, vis service.Visibility, expired bool) *service.VotedReport {
		p := wm5Vote(1, v, vis, expired)
		return &p
	}
	cases := []tc{
		{"vote confirm live", vote(service.VoteConfirm, service.VisibilityLive, false), "confirmed", "vote", "Ongoing"},
		{"vote confirm hidden unexpired", vote(service.VoteConfirm, service.VisibilityHidden, false), "confirmed", "vote", "Disputed"},
		{"vote confirm retracted unexpired", vote(service.VoteConfirm, service.VisibilityRetracted, false), "confirmed", "vote", "Resolved"},
		{"vote dispute provisional", vote(service.VoteDispute, service.VisibilityProvisional, false), "disputed", "dispute", "Ongoing"},
		{"vote dispute expired", vote(service.VoteDispute, service.VisibilityProvisional, true), "disputed", "dispute", "Expired"},
		{"vote dispute retracted expired", vote(service.VoteDispute, service.VisibilityRetracted, true), "disputed", "dispute", "Resolved"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page := service.ActivityPage{Votes: []service.VotedReport{*c.vote}}
			view := wm5Cards(t, page, "")
			card := wm5Card(t, view, 1)
			for _, p := range view.Parts {
				in := len(p.Rows) == 1
				if in != (p.Key == c.wantPart) {
					t.Errorf("part %q holds %d rows, the card belongs only in %q", p.Key, len(p.Rows), c.wantPart)
				}
				if p.Count != len(p.Rows) {
					t.Errorf("part %q count %d but %d rows", p.Key, p.Count, len(p.Rows))
				}
			}
			if card.Kind != c.wantKind {
				t.Errorf("Kind = %q, want %q", card.Kind, c.wantKind)
			}
			if card.StatusWord != c.wantStatus {
				t.Errorf("StatusWord = %q, want %q", card.StatusWord, c.wantStatus)
			}
		})
	}

	t.Run("updates and ended hold no server rows", func(t *testing.T) {
		view := wm5Cards(t, wm5MixedPage(), "")
		for _, key := range []string{"updates", "ended"} {
			p := wm5Part(t, view, key)
			if len(p.Rows) != 0 || p.Count != 0 {
				t.Errorf("part %q rows %d count %d, want no server rows", key, len(p.Rows), p.Count)
			}
			// Updates shows the number 0 until Phase 3.2; Ended nearby shows no
			// count until the browser read of a later plan fills it.
			if wantShow := key == "updates"; p.ShowCount != wantShow {
				t.Errorf("part %q ShowCount = %v, want %v", key, p.ShowCount, wantShow)
			}
		}
	})

	t.Run("the account's own posts are no cards", func(t *testing.T) {
		// Quick 261117-vuf: whatever the post's state, including a resolved
		// unexpired one that once offered Reopen, it is listed in no part.
		for _, vis := range []service.Visibility{service.VisibilityLive, service.VisibilityProvisional, service.VisibilityRetracted, service.VisibilityHidden} {
			for _, expired := range []bool{false, true} {
				view := wm5Cards(t, service.ActivityPage{Posts: []service.ActivityReport{wm5Post(1, vis, expired)}}, "")
				if n := len(wm5AllRows(view)); n != 0 {
					t.Errorf("post %s expired %v made %d cards, want none", vis, expired, n)
				}
			}
		}
	})
}

// TestActivityStatusWordsForModeratedRows pins the two Phase 4 words and their
// precedence over the state words. Amended 2026-11-17 (quick 261117-vuf): the
// cards are voted cards now, since own posts are no longer listed.
func TestActivityStatusWordsForModeratedRows(t *testing.T) {
	held := wm5Vote(1, service.VoteConfirm, service.VisibilityWithheld, false)
	held.Reason = service.ReasonAwaitingTextCheck
	removed := wm5Vote(2, service.VoteConfirm, service.VisibilityWithheld, false)
	removed.Reason = service.ReasonRemovedAfterReview
	heldExpired := wm5Vote(3, service.VoteDispute, service.VisibilityWithheld, true)
	heldExpired.Reason = service.ReasonAwaitingTextCheck

	view := wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{held, removed, heldExpired}}, "")
	want := map[int64]string{1: "Waiting for a text check", 2: "Removed after review", 3: "Expired"}
	for id, w := range want {
		if got := wm5Card(t, view, id).StatusWord; got != w {
			t.Errorf("report %d StatusWord = %q, want %q", id, got, w)
		}
	}
}

// wm5MixedPage holds a live post, a resolved post, a confirmed vote and a
// disputed vote. Since quick 261117-vuf only the two votes become cards.
func wm5MixedPage() service.ActivityPage {
	return service.ActivityPage{
		Posts: []service.ActivityReport{
			wm5Post(1, service.VisibilityLive, false),
			wm5Post(2, service.VisibilityRetracted, false),
		},
		Votes: []service.VotedReport{
			wm5Vote(3, service.VoteConfirm, service.VisibilityLive, false),
			wm5Vote(4, service.VoteDispute, service.VisibilityHidden, false),
		},
	}
}

func TestActivityCountsEqualRowCounts(t *testing.T) {
	view := wm5Cards(t, wm5MixedPage(), "")
	want := map[string]int{"updates": 0, "confirmed": 1, "disputed": 1, "ended": 0}
	for _, p := range view.Parts {
		if p.Count != want[p.Key] || len(p.Rows) != want[p.Key] {
			t.Errorf("part %q count %d rows %d, want %d", p.Key, p.Count, len(p.Rows), want[p.Key])
		}
	}
	if n := len(wm5AllRows(view)); n != 2 {
		t.Errorf("got %d rows, want 2 distinct voted reports each in exactly one part", n)
	}
}

// TestActivityHubSummaries pins the hub row line of each part (P34-C13).
func TestActivityHubSummaries(t *testing.T) {
	t.Run("an empty account", func(t *testing.T) {
		view := wm5Cards(t, service.ActivityPage{}, "")
		want := map[string]string{
			"updates":   "No updates yet",
			"confirmed": "Nothing confirmed yet",
			"disputed":  "Nothing disputed yet",
			"ended":     "Last 7 days, near your area",
		}
		for _, p := range view.Parts {
			if p.Summary != want[p.Key] {
				t.Errorf("part %q summary = %q, want %q", p.Key, p.Summary, want[p.Key])
			}
		}
	})

	t.Run("confirmed and disputed read the newest text", func(t *testing.T) {
		older := wm5Vote(7, service.VoteConfirm, service.VisibilityLive, false)
		older.Row.Description = "older text"
		older.VotedAt = wm5Now.Add(-time.Hour)
		newer := wm5Vote(8, service.VoteConfirm, service.VisibilityLive, false)
		newer.Row.Description = "newer text"
		dispute := wm5Vote(9, service.VoteDispute, service.VisibilityLive, false)
		dispute.Row.Description = "wrong text"
		view := wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{older, newer, dispute}}, "")
		if got, want := wm5Part(t, view, "confirmed").Summary, "Latest: newer text"; got != want {
			t.Errorf("confirmed summary = %q, want %q", got, want)
		}
		if got, want := wm5Part(t, view, "disputed").Summary, "Latest: wrong text"; got != want {
			t.Errorf("disputed summary = %q, want %q", got, want)
		}
	})

	t.Run("a blurred Rescue card is skipped for a card with text", func(t *testing.T) {
		coarse := wm5Vote(5, service.VoteConfirm, service.VisibilityRetracted, false)
		coarse.Category = service.CategoryRescueNeeded
		coarse.Row.Category = string(service.CategoryRescueNeeded)
		coarse.Approximate = true
		older := wm5Vote(6, service.VoteConfirm, service.VisibilityLive, false)
		older.Row.Description = "plain text"
		older.VotedAt = wm5Now.Add(-time.Hour)
		view := wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{coarse, older}}, "")
		if got, want := wm5Part(t, view, "confirmed").Summary, "Latest: plain text"; got != want {
			t.Errorf("summary = %q, want %q", got, want)
		}

		onlyCoarse := wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{coarse}}, "")
		if got, want := wm5Part(t, onlyCoarse, "confirmed").Summary, "Latest: Rescue needed"; got != want {
			t.Errorf("summary = %q, want the coarse title %q", got, want)
		}
	})
}

// TestActivityCurrentPart: only the asked for part is current, and anything the
// builder does not know reads as the list.
func TestActivityCurrentPart(t *testing.T) {
	view := wm5Cards(t, wm5MixedPage(), "disputed")
	if view.CurrentPart != "disputed" || view.CurrentName() != "Disputed" {
		t.Errorf("current = %q / %q, want disputed / Disputed", view.CurrentPart, view.CurrentName())
	}
	for _, p := range view.Parts {
		if p.Current != (p.Key == "disputed") {
			t.Errorf("part %q Current = %v", p.Key, p.Current)
		}
	}
	for _, unknown := range []string{"bogus", "posted"} {
		list := wm5Cards(t, wm5MixedPage(), unknown)
		if list.CurrentPart != "" || list.CurrentName() != "" {
			t.Errorf("part %q reads as %q, want the list", unknown, list.CurrentPart)
		}
	}
}

func TestActivityOrderIsNewestEventFirstTiesByReportID(t *testing.T) {
	page := service.ActivityPage{
		Posts: []service.ActivityReport{
			wm5Post(5, service.VisibilityLive, false), // event: created two hours ago
		},
		Votes: []service.VotedReport{
			wm5Vote(9, service.VoteConfirm, service.VisibilityLive, false),  // ten minutes ago
			wm5Vote(12, service.VoteConfirm, service.VisibilityLive, false), // ten minutes ago, ties with 9
		},
	}
	view := wm5Cards(t, page, "")
	var got []int64
	for _, c := range wm5Part(t, view, "confirmed").Rows {
		got = append(got, c.ReportID)
	}
	if want := []int64{12, 9}; !slices.Equal(got, want) {
		t.Errorf("confirmed order = %v, want %v", got, want)
	}
}

func TestActivityCardWordsPerKind(t *testing.T) {
	page := service.ActivityPage{
		Votes: []service.VotedReport{
			wm5Vote(2, service.VoteConfirm, service.VisibilityLive, false),
			wm5Vote(3, service.VoteDispute, service.VisibilityLive, false),
		},
	}
	view := wm5Cards(t, page, "")
	type want struct{ tag, action, icon, kind string }
	cases := map[int64]want{
		2: {"Your vote", "You confirmed this report", "icon-glyph--flood", "vote"},
		3: {"Your dispute", "You disputed this report", "icon-glyph--flood", "dispute"},
	}
	for id, w := range cases {
		c := wm5Card(t, view, id)
		got := want{c.Tag, c.Action, c.IconGlyph, c.Kind}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("card %d = %+v, want %+v", id, got, w)
		}
	}
	if c := wm5Card(t, view, 2); c.Detail != "Flood" {
		t.Errorf("vote Detail %q, want %q", c.Detail, "Flood")
	}
	if c := wm5Card(t, view, 2); c.When != "10 min ago" || c.WhenText != "You confirmed 10 min ago" {
		t.Errorf("vote When %q WhenText %q, want 10 min ago (the time of the vote) and %q", c.When, c.WhenText, "You confirmed 10 min ago")
	}
	if c := wm5Card(t, view, 3); c.WhenText != "You disputed 10 min ago" {
		t.Errorf("dispute WhenText %q, want %q", c.WhenText, "You disputed 10 min ago")
	}
}

// TestActivityCardCarriesItsPoint: Lat and Lon are the row's own point, which is
// already the coarse cell centre for a finished Rescue report (T-34-31).
func TestActivityCardCarriesItsPoint(t *testing.T) {
	live := wm5Vote(1, service.VoteConfirm, service.VisibilityLive, false)
	live.Row.Latitude, live.Row.Longitude = 13.0827, 80.2707
	vote := wm5Vote(2, service.VoteConfirm, service.VisibilityRetracted, false)
	vote.Category = service.CategoryRescueNeeded
	vote.Row.Category = string(service.CategoryRescueNeeded)
	vote.Row.Latitude, vote.Row.Longitude = 13.05, 80.25 // the coarse centre the service wrote
	vote.Approximate = true

	view := wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{live, vote}}, "")
	if c := wm5Card(t, view, 1); c.Lat != 13.0827 || c.Lon != 80.2707 || c.Approximate {
		t.Errorf("live card point = %v, %v approximate %v, want its own exact point", c.Lat, c.Lon, c.Approximate)
	}
	if c := wm5Card(t, view, 2); c.Lat != 13.05 || c.Lon != 80.25 || !c.Approximate {
		t.Errorf("blurred rescue point = %v, %v approximate %v, want the coarse centre", c.Lat, c.Lon, c.Approximate)
	}
}

func TestActivityDetailKeepsTheShelterWords(t *testing.T) {
	vote := wm5Vote(1, service.VoteConfirm, service.VisibilityLive, false)
	vote.Category = service.CategoryShelterOpen
	vote.Row.Category = string(service.CategoryShelterOpen)
	capacity := "limited"
	headcount := int32(40)
	vote.Row.ShelterCapacityStatus = &capacity
	vote.Row.ShelterHeadcount = &headcount

	card := wm5Card(t, wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{vote}}, ""), 1)
	if want := "Shelter open · Limited · 40 people"; card.Detail != want {
		t.Errorf("Detail = %q, want %q", card.Detail, want)
	}
}

// TestProfileOffersNoReopen replaces the old Reopen and undo limit pins (D-22,
// W-09, IN-03). Amended 2026-11-17 (quick 261117-vuf): the poster's Reopen
// lived only on Posted cards, which left, so the card has no Reopen or "used
// both undos" hook at all, and no card of any state carries one. The take back
// and the report's own Reopen are unchanged elsewhere.
func TestProfileOffersNoReopen(t *testing.T) {
	typ := reflect.TypeOf(profileReport{})
	for _, name := range []string{"CanReopen", "UndosUsed"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Errorf("profileReport still has %s; the Reopen hook left with Posted", name)
		}
	}
	// A resolved unexpired post with undos left, the one card that once offered
	// Reopen, is now no card at all.
	post := wm5Post(1, service.VisibilityRetracted, false)
	post.UndosLeft = service.UndoLimit
	if n := len(wm5AllRows(wm5Cards(t, service.ActivityPage{Posts: []service.ActivityReport{post}}, ""))); n != 0 {
		t.Errorf("a resolved own post made %d cards, want none", n)
	}
}

// TestProfileVotedRescueCardIsApproximate is D-27 and D-23 on the card: an
// approximate voted card carries the words of the coarse report, never the
// description, and a card that is not approximate is left as it was.
func TestProfileVotedRescueCardIsApproximate(t *testing.T) {
	vote := wm5Vote(5, service.VoteConfirm, service.VisibilityRetracted, false)
	vote.Category = service.CategoryRescueNeeded
	vote.Row.Category = string(service.CategoryRescueNeeded)
	vote.Row.Description = ""
	vote.Approximate = true

	card := wm5Card(t, wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{vote}}, ""), 5)
	if card.Description != "Rescue needed" {
		t.Errorf("Description = %q, want %q", card.Description, "Rescue needed")
	}
	if !strings.HasSuffix(card.Detail, " · Approximate location") {
		t.Errorf("Detail = %q, want it to end with the approximate location words", card.Detail)
	}

	exact := wm5Vote(6, service.VoteConfirm, service.VisibilityLive, false)
	exact.Category = service.CategoryRescueNeeded
	exact.Row.Category = string(service.CategoryRescueNeeded)
	exactCard := wm5Card(t, wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{exact}}, ""), 6)
	if exactCard.Description != "voted description" || strings.Contains(exactCard.Detail, "Approximate") {
		t.Errorf("a live rescue card changed: description %q detail %q", exactCard.Description, exactCard.Detail)
	}
}

// TestCategoryLabelsCoverEveryKnownCategory: the Activity page words come from
// categoryLabels, so every category a stored report may carry needs a label.
// Traffic is new for 2026-10-16 and rescue_needed stays for old rows.
func TestCategoryLabelsCoverEveryKnownCategory(t *testing.T) {
	for _, c := range service.Categories {
		if categoryLabels[c] == "" {
			t.Errorf("category %q has no label in categoryLabels", c)
		}
	}
	if len(categoryLabels) != len(service.Categories) {
		t.Errorf("categoryLabels has %d entries, service.Categories has %d", len(categoryLabels), len(service.Categories))
	}
	if got := categoryLabels[service.CategoryTraffic]; got != "Traffic" {
		t.Errorf("traffic label = %q, want %q", got, "Traffic")
	}
	if got := categoryLabels[service.CategoryRescueNeeded]; got != "Rescue needed" {
		t.Errorf("rescue_needed label = %q, want %q", got, "Rescue needed")
	}
}

// p32Item builds one update item for the Updates part tests. The report id is
// the item id plus 100, so the link is easy to read.
func p32Item(id int64, kind service.UpdateKind, unread bool, ageMinutes int) service.UpdateItem {
	rid := id + 100
	return service.UpdateItem{
		ID: id, Kind: kind, ReportID: &rid, Available: true, Unread: unread,
		CreatedAt: wm5Now.Add(-time.Duration(ageMinutes) * time.Minute),
		Event:     service.EventLine(kind),
		Category:  service.CategoryFlood, Severity: service.SeverityCritical,
		Trust: "Confirmed by 3 people at different spots.",
	}
}

// TestFillUpdatesPart: the Updates part gets its rows, counts, summary and cut
// flag from the same List the bell reads (D-05), and nothing else on the page
// changes.
func TestFillUpdatesPart(t *testing.T) {
	base := func(t *testing.T) activityPageView {
		t.Helper()
		page := service.ActivityPage{Posts: []service.ActivityReport{wm5Post(1, service.VisibilityLive, false)}}
		return wm5Cards(t, page, "")
	}
	otherParts := func(v activityPageView) []activityPart {
		var out []activityPart
		for _, p := range v.Parts {
			if p.Key != activityPartUpdates {
				out = append(out, p)
			}
		}
		return out
	}

	t.Run("no items keeps the empty hub", func(t *testing.T) {
		view := base(t)
		fillUpdatesPart(&view, service.UpdatesPage{}, false, wm5Now)
		p := wm5Part(t, view, "updates")
		if p.Count != 0 || p.NewCount != 0 || len(p.Updates) != 0 || p.Summary != "No updates yet" || p.UpdatesCut || p.UpdatesFailed {
			t.Errorf("empty part = %+v", p)
		}
	})

	t.Run("three items, two unread", func(t *testing.T) {
		view := base(t)
		before := otherParts(view)
		page := service.UpdatesPage{
			Items: []service.UpdateItem{
				p32Item(30, service.UpdateReachedLive, true, 12),
				p32Item(20, service.UpdateHidden, true, 90),
				p32Item(10, service.UpdateFading, false, 300),
			},
			Unread: 2, Total: 3,
		}
		fillUpdatesPart(&view, page, false, wm5Now)
		p := wm5Part(t, view, "updates")
		if p.Count != 3 || p.NewCount != 2 || p.UpdatesCut || p.UpdatesFailed {
			t.Errorf("counts = %d / %d / cut %v / failed %v", p.Count, p.NewCount, p.UpdatesCut, p.UpdatesFailed)
		}
		if p.Summary != "Now confirmed · Flood" {
			t.Errorf("summary = %q", p.Summary)
		}
		if len(p.Updates) != 3 {
			t.Fatalf("rows = %d, want 3", len(p.Updates))
		}
		for i, wantID := range []int64{30, 20, 10} {
			row := p.Updates[i]
			if row.ID != wantID {
				t.Errorf("row %d id = %d, want %d", i, row.ID, wantID)
			}
			if want := "/?report=" + strconv.FormatInt(wantID+100, 10); row.Href != want {
				t.Errorf("row %d href = %q, want %q", i, row.Href, want)
			}
			if row.SeverityClass != "sev-critical" || row.Category != "Flood" || row.Severity != "Critical" {
				t.Errorf("row %d words = %+v", i, row)
			}
			if row.Label == "" || row.When == "" {
				t.Errorf("row %d has no label or time: %+v", i, row)
			}
		}
		if !strings.HasPrefix(p.Updates[0].Label, "New. Now confirmed.") || strings.HasPrefix(p.Updates[2].Label, "New.") {
			t.Errorf("labels = %q / %q", p.Updates[0].Label, p.Updates[2].Label)
		}
		if !p.Updates[0].Unread || p.Updates[2].Unread {
			t.Errorf("unread flags = %v / %v", p.Updates[0].Unread, p.Updates[2].Unread)
		}
		if !reflect.DeepEqual(before, otherParts(view)) {
			t.Errorf("another part changed")
		}
	})

	// Added 2026-10-29 by Phase 8 plan 11 (D-16, A-P8-3): the two comment kinds
	// reach the Updates part like any other event row and read their own line.
	t.Run("the two comment kinds read their own lines", func(t *testing.T) {
		for _, c := range []struct {
			kind service.UpdateKind
			want string
		}{
			{service.UpdateCommentOnYours, "New comment on your report"},
			{service.UpdateReplyToYours, "New reply to your comment"},
		} {
			view := base(t)
			it := p32Item(6, c.kind, true, 1)
			fillUpdatesPart(&view, service.UpdatesPage{Items: []service.UpdateItem{it}, Unread: 1, Total: 1}, false, wm5Now)
			p := wm5Part(t, view, "updates")
			if len(p.Updates) != 1 || !strings.Contains(p.Summary, c.want) || !strings.Contains(p.Updates[0].Label, c.want) {
				t.Errorf("%s: summary %q rows %+v, want the line %q", c.kind, p.Summary, p.Updates, c.want)
			}
			if p.Updates[0].Href != "/?report=106" {
				t.Errorf("%s: href = %q, want the report link", c.kind, p.Updates[0].Href)
			}
		}
	})

	t.Run("a Reliable note reads its first sentence", func(t *testing.T) {
		view := base(t)
		it := p32Item(5, service.UpdateReliableNearby, false, 1)
		it.Trust = service.UnconfirmedLine
		fillUpdatesPart(&view, service.UpdatesPage{Items: []service.UpdateItem{it}, Total: 1}, false, wm5Now)
		if got, want := wm5Part(t, view, "updates").Summary, "Reported nearby by a Reliable reporter."; got != want {
			t.Errorf("summary = %q, want %q", got, want)
		}
	})

	// Added 2026-10-29 by Phase 9 plan 21 (D-09): a follow note's hub summary is
	// its own line, as for the Reliable note, because that line already names
	// the person; no category word is appended.
	t.Run("a follow note reads its own line", func(t *testing.T) {
		view := base(t)
		it := p32Item(5, service.UpdateFollowedPosted, false, 1)
		it.Event = service.FollowedPostedLine("ravi.k")
		it.Trust = service.UnconfirmedLine
		fillUpdatesPart(&view, service.UpdatesPage{Items: []service.UpdateItem{it}, Total: 1}, false, wm5Now)
		if got, want := wm5Part(t, view, "updates").Summary, "ravi.k posted a report."; got != want {
			t.Errorf("summary = %q, want %q", got, want)
		}
	})

	t.Run("an unavailable or rolled up newest row reads its own line", func(t *testing.T) {
		view := base(t)
		gone := service.UpdateItem{ID: 9, Kind: service.UpdateHidden, CreatedAt: wm5Now.Add(-time.Hour), Event: service.UnavailableLine}
		fillUpdatesPart(&view, service.UpdatesPage{Items: []service.UpdateItem{gone}, Total: 1}, false, wm5Now)
		p := wm5Part(t, view, "updates")
		if p.Summary != service.UnavailableLine {
			t.Errorf("summary = %q", p.Summary)
		}
		if p.Updates[0].Href != "" || p.Updates[0].IsMore {
			t.Errorf("an unavailable row must not link: %+v", p.Updates[0])
		}

		view = base(t)
		more := service.UpdateItem{ID: 8, Kind: service.UpdateMore, Available: true, MoreCount: 4, CreatedAt: wm5Now.Add(-time.Hour), Event: service.MoreLine(4)}
		fillUpdatesPart(&view, service.UpdatesPage{Items: []service.UpdateItem{more}, Total: 1}, false, wm5Now)
		p = wm5Part(t, view, "updates")
		if p.Summary != "And 4 more changes" || !p.Updates[0].IsMore || p.Updates[0].Href != "" {
			t.Errorf("more row = %+v / summary %q", p.Updates[0], p.Summary)
		}
	})

	t.Run("a list cut at 50 says so", func(t *testing.T) {
		view := base(t)
		items := make([]service.UpdateItem, 0, 50)
		for i := 0; i < 50; i++ {
			items = append(items, p32Item(int64(1000-i), service.UpdateHidden, false, i+1))
		}
		fillUpdatesPart(&view, service.UpdatesPage{Items: items, Total: 73}, false, wm5Now)
		p := wm5Part(t, view, "updates")
		if p.Count != 50 || !p.UpdatesCut {
			t.Errorf("count %d cut %v, want 50 and cut", p.Count, p.UpdatesCut)
		}
	})

	t.Run("a failed read flags the part and leaves the others alone", func(t *testing.T) {
		view := base(t)
		before := otherParts(view)
		fillUpdatesPart(&view, service.UpdatesPage{Items: []service.UpdateItem{p32Item(1, service.UpdateHidden, true, 1)}}, true, wm5Now)
		p := wm5Part(t, view, "updates")
		if !p.UpdatesFailed || len(p.Updates) != 0 || p.Count != 0 || p.NewCount != 0 {
			t.Errorf("failed part = %+v", p)
		}
		if !reflect.DeepEqual(before, otherParts(view)) {
			t.Errorf("another part changed")
		}
	})

	t.Run("the severity class comes from a closed lookup", func(t *testing.T) {
		for sev, want := range map[service.Severity]string{
			service.SeverityLow: "sev-low", service.SeverityMedium: "sev-medium", service.SeverityCritical: "sev-critical", "": "", "weird": "",
		} {
			if got := updateSeverityClass(sev); got != want {
				t.Errorf("updateSeverityClass(%q) = %q, want %q", sev, got, want)
			}
		}
	})
}
