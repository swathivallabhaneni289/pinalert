package handlers

import (
	"reflect"
	"slices"
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
		// The five part values are honoured exactly.
		{"updates", "", "updates"},
		{"posted", "", "posted"},
		{"confirmed", "", "confirmed"},
		{"disputed", "", "disputed"},
		{"ended", "", "ended"},
		// The old tab values are read only aliases.
		{"", "posts", "posted"},
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
		{"Posted", "", ""},
		{"POSTED", "", ""},
		{"posted ", "", ""},
		{" posted", "", ""},
		{"", "Posts", ""},
		{"", "posts ", ""},
		{"posted&part=ended", "", ""},
		// The part wins over the tab when both are sent.
		{"ended", "posts", "ended"},
		{"updates", "votes", "updates"},
	}
	for _, c := range cases {
		if got := parseActivityPart(c.part, c.tab); got != c.want {
			t.Errorf("parseActivityPart(%q, %q) = %q, want %q", c.part, c.tab, got, c.want)
		}
	}
}

// TestActivityPartsInOrder pins the five parts, their names, their left lines
// from UI-SPEC 3a, their hub icons and their addresses.
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
	if want := []string{"updates", "posted", "confirmed", "disputed", "ended"}; !slices.Equal(keys, want) {
		t.Errorf("keys = %v, want %v", keys, want)
	}
	if want := []string{"Updates", "Posted by you", "You confirmed", "You disputed", "Ended nearby"}; !slices.Equal(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
	wantLines := []string{
		"Changes on reports you posted or voted on, newest first.",
		"Reports you posted. Open one to resolve or reopen it.",
		"Reports you said are true.",
		"Reports you said are wrong.",
		"Everyone's reports near you that ended in the last 7 days.",
	}
	if !slices.Equal(lines, wantLines) {
		t.Errorf("lines = %v, want %v", lines, wantLines)
	}
	if want := []string{"bell", "circle-plus", "check-circle", "circle-x", "history"}; !slices.Equal(icons, want) {
		t.Errorf("icons = %v, want %v", icons, want)
	}
	wantHrefs := []string{"/profile?part=updates", "/profile?part=posted", "/profile?part=confirmed", "/profile?part=disputed", "/profile?part=ended"}
	if !slices.Equal(hrefs, wantHrefs) {
		t.Errorf("hrefs = %v, want %v", hrefs, wantHrefs)
	}
	for _, p := range view.Parts {
		if p.EmptyTitle == "" || p.EmptyText == "" {
			t.Errorf("part %q has no empty state words", p.Key)
		}
	}
}

// TestActivityPartEmptyStates pins the five empty titles and texts of UI-SPEC 3c.
func TestActivityPartEmptyStates(t *testing.T) {
	view := wm5Cards(t, service.ActivityPage{}, "")
	want := map[string][2]string{
		"updates":   {"No updates yet", "Changes on reports you posted or voted on will show here."},
		"posted":    {"No reports yet", "Reports you post will appear here."},
		"confirmed": {"No confirmations yet", "Reports you confirm will appear here."},
		"disputed":  {"No disputes yet", "Reports you dispute will appear here."},
		"ended":     {"No ended reports nearby", "Nothing in this area was resolved or faded out in the last 7 days."},
	}
	for _, p := range view.Parts {
		if got := [2]string{p.EmptyTitle, p.EmptyText}; got != want[p.Key] {
			t.Errorf("part %q empty = %v, want %v", p.Key, got, want[p.Key])
		}
	}
	if len(wm5AllRows(view)) != 0 || view.Record != nil {
		t.Errorf("empty account has %d rows and record %v", len(wm5AllRows(view)), view.Record)
	}
}

// TestActivityPartMembership is W-03 rewritten for the parts (D-21): a post is
// only in Posted by you, a confirm vote only in You confirmed, a dispute only in
// You disputed, whatever the report's state. Counts equal row counts; Updates and
// Ended nearby hold no server rows. It also pins which cards may offer Reopen
// (W-09) and the status word of each state.
func TestActivityPartMembership(t *testing.T) {
	type tc struct {
		name       string
		post       *service.ActivityReport
		vote       *service.VotedReport
		wantPart   string
		wantReopen bool
		wantKind   string
		wantStatus string
	}
	post := func(vis service.Visibility, expired bool) *service.ActivityReport {
		p := wm5Post(1, vis, expired)
		return &p
	}
	vote := func(v service.VoteValue, vis service.Visibility, expired bool) *service.VotedReport {
		p := wm5Vote(1, v, vis, expired)
		return &p
	}
	cases := []tc{
		{"post live unexpired", post(service.VisibilityLive, false), nil, "posted", false, "post", "Ongoing"},
		{"post provisional expired", post(service.VisibilityProvisional, true), nil, "posted", false, "post", "Expired"},
		{"post retracted unexpired", post(service.VisibilityRetracted, false), nil, "posted", true, "post", "Resolved"},
		{"post retracted expired", post(service.VisibilityRetracted, true), nil, "posted", false, "post", "Resolved"},
		{"post hidden unexpired", post(service.VisibilityHidden, false), nil, "posted", false, "post", "Disputed"},
		{"vote confirm live", nil, vote(service.VoteConfirm, service.VisibilityLive, false), "confirmed", false, "vote", "Ongoing"},
		{"vote confirm hidden unexpired", nil, vote(service.VoteConfirm, service.VisibilityHidden, false), "confirmed", false, "vote", "Disputed"},
		{"vote confirm retracted unexpired", nil, vote(service.VoteConfirm, service.VisibilityRetracted, false), "confirmed", false, "vote", "Resolved"},
		{"vote dispute provisional", nil, vote(service.VoteDispute, service.VisibilityProvisional, false), "disputed", false, "dispute", "Ongoing"},
		{"vote dispute expired", nil, vote(service.VoteDispute, service.VisibilityProvisional, true), "disputed", false, "dispute", "Expired"},
		{"vote dispute retracted expired", nil, vote(service.VoteDispute, service.VisibilityRetracted, true), "disputed", false, "dispute", "Resolved"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var page service.ActivityPage
			if c.post != nil {
				page.Posts = []service.ActivityReport{*c.post}
			}
			if c.vote != nil {
				page.Votes = []service.VotedReport{*c.vote}
			}
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
			if card.CanReopen != c.wantReopen {
				t.Errorf("CanReopen = %v, want %v", card.CanReopen, c.wantReopen)
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
}

// TestActivityStatusWordsForModeratedRows pins the two Phase 4 words and their
// precedence over the state words.
func TestActivityStatusWordsForModeratedRows(t *testing.T) {
	held := wm5Post(1, service.VisibilityWithheld, false)
	held.Reason = service.ReasonAwaitingTextCheck
	removed := wm5Post(2, service.VisibilityWithheld, false)
	removed.Reason = service.ReasonRemovedAfterReview
	heldExpired := wm5Post(3, service.VisibilityWithheld, true)
	heldExpired.Reason = service.ReasonAwaitingTextCheck

	view := wm5Cards(t, service.ActivityPage{Posts: []service.ActivityReport{held, removed, heldExpired}}, "")
	want := map[int64]string{1: "Waiting for a text check", 2: "Removed after review", 3: "Expired"}
	for id, w := range want {
		if got := wm5Card(t, view, id).StatusWord; got != w {
			t.Errorf("report %d StatusWord = %q, want %q", id, got, w)
		}
	}
}

// wm5MixedPage holds four cards: a live post, a resolved post, a confirmed
// vote and a disputed vote.
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
	want := map[string]int{"updates": 0, "posted": 2, "confirmed": 1, "disputed": 1, "ended": 0}
	for _, p := range view.Parts {
		if p.Count != want[p.Key] || len(p.Rows) != want[p.Key] {
			t.Errorf("part %q count %d rows %d, want %d", p.Key, p.Count, len(p.Rows), want[p.Key])
		}
	}
	if n := len(wm5AllRows(view)); n != 4 {
		t.Errorf("got %d rows, want 4 distinct reports each in exactly one part", n)
	}
}

// TestActivityHubSummaries pins the hub row line of each part (P34-C13).
func TestActivityHubSummaries(t *testing.T) {
	t.Run("an empty account", func(t *testing.T) {
		view := wm5Cards(t, service.ActivityPage{}, "")
		want := map[string]string{
			"updates":   "No updates yet",
			"posted":    "Nothing posted yet",
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

	t.Run("posted counts open and resolved posts", func(t *testing.T) {
		page := service.ActivityPage{Posts: []service.ActivityReport{
			wm5Post(1, service.VisibilityLive, false),
			wm5Post(2, service.VisibilityProvisional, false),
			wm5Post(3, service.VisibilityRetracted, false),
			wm5Post(4, service.VisibilityProvisional, true), // faded: neither number
		}}
		got := wm5Part(t, wm5Cards(t, page, ""), "posted").Summary
		if want := "2 still happening, 1 resolved"; got != want {
			t.Errorf("summary = %q, want %q", got, want)
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
	if view.CurrentPart != "disputed" || view.CurrentName() != "You disputed" {
		t.Errorf("current = %q / %q, want disputed / You disputed", view.CurrentPart, view.CurrentName())
	}
	for _, p := range view.Parts {
		if p.Current != (p.Key == "disputed") {
			t.Errorf("part %q Current = %v", p.Key, p.Current)
		}
	}
	list := wm5Cards(t, wm5MixedPage(), "bogus")
	if list.CurrentPart != "" || list.CurrentName() != "" {
		t.Errorf("an unknown part reads as %q, want the list", list.CurrentPart)
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

func TestActivityRecordComesFromOwnPostsNeverFromTheNewestCard(t *testing.T) {
	post := wm5Post(1, service.VisibilityLive, false)
	post.Trust.Reporter = service.ReporterSignal{Tag: service.ReporterReliable, Label: "Reliable reporter", JudgedReports: 5, CorroboratedReports: 5}
	vote := wm5Vote(2, service.VoteConfirm, service.VisibilityLive, false)
	vote.Trust.Reporter = service.ReporterSignal{Tag: service.ReporterNew, Label: "New reporter"}

	view := wm5Cards(t, service.ActivityPage{
		Posts: []service.ActivityReport{post},
		Votes: []service.VotedReport{vote},
	}, "")
	if view.Record == nil || view.Record.Label != "Reliable reporter" || view.Record.Counts != "5 of 5 confirmed" {
		t.Errorf("record = %+v, want the account's own reliable record", view.Record)
	}

	voteOnly := wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{vote}}, "")
	if voteOnly.Record != nil {
		t.Errorf("record = %+v, want nil for an account with no posts", voteOnly.Record)
	}
}

func TestActivityCardWordsPerKind(t *testing.T) {
	page := service.ActivityPage{
		Posts: []service.ActivityReport{wm5Post(1, service.VisibilityLive, false)},
		Votes: []service.VotedReport{
			wm5Vote(2, service.VoteConfirm, service.VisibilityLive, false),
			wm5Vote(3, service.VoteDispute, service.VisibilityLive, false),
		},
	}
	view := wm5Cards(t, page, "")
	type want struct{ tag, action, icon, kind string }
	cases := map[int64]want{
		1: {"Your post", "You posted this report", "icon-glyph--flood", "post"},
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
	if c := wm5Card(t, view, 1); c.When != "2 hours ago" || c.Detail != "Flood" || c.WhenText != "Posted 2 hours ago" {
		t.Errorf("post When %q Detail %q WhenText %q, want %q, %q and %q", c.When, c.Detail, c.WhenText, "2 hours ago", "Flood", "Posted 2 hours ago")
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
	post := wm5Post(1, service.VisibilityLive, false)
	post.Row.Latitude, post.Row.Longitude = 13.0827, 80.2707
	vote := wm5Vote(2, service.VoteConfirm, service.VisibilityRetracted, false)
	vote.Category = service.CategoryRescueNeeded
	vote.Row.Category = string(service.CategoryRescueNeeded)
	vote.Row.Latitude, vote.Row.Longitude = 13.05, 80.25 // the coarse centre the service wrote
	vote.Approximate = true

	view := wm5Cards(t, service.ActivityPage{Posts: []service.ActivityReport{post}, Votes: []service.VotedReport{vote}}, "")
	if c := wm5Card(t, view, 1); c.Lat != 13.0827 || c.Lon != 80.2707 || c.Approximate {
		t.Errorf("post point = %v, %v approximate %v, want its own exact point", c.Lat, c.Lon, c.Approximate)
	}
	if c := wm5Card(t, view, 2); c.Lat != 13.05 || c.Lon != 80.25 || !c.Approximate {
		t.Errorf("blurred rescue point = %v, %v approximate %v, want the coarse centre", c.Lat, c.Lon, c.Approximate)
	}
}

func TestActivityDetailKeepsTheShelterWords(t *testing.T) {
	post := wm5Post(1, service.VisibilityLive, false)
	post.Category = service.CategoryShelterOpen
	post.Row.Category = string(service.CategoryShelterOpen)
	capacity := "limited"
	headcount := int32(40)
	post.Row.ShelterCapacityStatus = &capacity
	post.Row.ShelterHeadcount = &headcount

	card := wm5Card(t, wm5Cards(t, service.ActivityPage{Posts: []service.ActivityReport{post}}, ""), 1)
	if want := "Shelter open · Limited · 40 people"; card.Detail != want {
		t.Errorf("Detail = %q, want %q", card.Detail, want)
	}
}

// TestProfileReopenFollowsUndoLimit is D-22 on the card: the poster's Reopen
// shares the budget of two undos with the take back, so a closed unexpired own
// post with no undos left offers no Reopen and says so, a post with one left
// still offers it, and an expired post offers neither.
func TestProfileReopenFollowsUndoLimit(t *testing.T) {
	cases := []struct {
		name                 string
		left                 int
		expired              bool
		wantReopen, wantUsed bool
	}{
		{"no undos left", 0, false, false, true},
		{"one undo left", 1, false, true, false},
		{"full budget", service.UndoLimit, false, true, false},
		{"expired with none left", 0, true, false, false},
		{"expired with one left", 1, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			post := wm5Post(1, service.VisibilityRetracted, c.expired)
			post.UndosLeft = c.left
			card := wm5Card(t, wm5Cards(t, service.ActivityPage{Posts: []service.ActivityReport{post}}, ""), 1)
			if card.CanReopen != c.wantReopen {
				t.Errorf("CanReopen = %v, want %v", card.CanReopen, c.wantReopen)
			}
			if card.UndosUsed != c.wantUsed {
				t.Errorf("UndosUsed = %v, want %v", card.UndosUsed, c.wantUsed)
			}
		})
	}

	t.Run("a live post with no undos left shows neither", func(t *testing.T) {
		post := wm5Post(1, service.VisibilityLive, false)
		post.UndosLeft = 0
		card := wm5Card(t, wm5Cards(t, service.ActivityPage{Posts: []service.ActivityReport{post}}, ""), 1)
		if card.CanReopen || card.UndosUsed {
			t.Errorf("CanReopen %v UndosUsed %v, want both false for a report that is not closed", card.CanReopen, card.UndosUsed)
		}
	})

	t.Run("a voted card never offers either", func(t *testing.T) {
		// The service leaves the full budget on a voted card (it reads undo
		// counts for own posts only), which is what the fixture carries.
		vote := wm5Vote(2, service.VoteConfirm, service.VisibilityRetracted, false)
		card := wm5Card(t, wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{vote}}, ""), 2)
		if card.CanReopen || card.UndosUsed {
			t.Errorf("voted card CanReopen %v UndosUsed %v, want both false", card.CanReopen, card.UndosUsed)
		}
	})

	t.Run("a voted card is kept from both even when its undos left is 0", func(t *testing.T) {
		// IN-03: the protection must not rest on the default budget. With none
		// left on a closed, unexpired voted card, newProfileReport alone would set
		// UndosUsed, and the page must still clear it.
		for _, left := range []int{0, 1, service.UndoLimit} {
			vote := wm5Vote(2, service.VoteConfirm, service.VisibilityRetracted, false)
			vote.UndosLeft = left
			card := wm5Card(t, wm5Cards(t, service.ActivityPage{Votes: []service.VotedReport{vote}}, ""), 2)
			if card.CanReopen || card.UndosUsed {
				t.Errorf("undos left %d: voted card CanReopen %v UndosUsed %v, want both false", left, card.CanReopen, card.UndosUsed)
			}
		}
	})
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
