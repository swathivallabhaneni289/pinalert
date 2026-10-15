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

// These tests belong to quick task 261013-wm5. They pin the six tab view model
// of the Activity page: tab parsing, the membership truth table (W-03), the
// counts of distinct reports, the card order (W-05), the record card source
// (W-10) and the words each kind of card carries (W-06).

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

func wm5Cards(t *testing.T, page service.ActivityPage, tab string) activityPageView {
	t.Helper()
	view, err := buildActivityPage(page, tab, wm5Now)
	if err != nil {
		t.Fatalf("buildActivityPage: %v", err)
	}
	return view
}

func wm5Card(t *testing.T, view activityPageView, id int64) profileReport {
	t.Helper()
	for _, c := range view.Reports {
		if c.ReportID == id {
			return c
		}
	}
	t.Fatalf("no card for report %d", id)
	return profileReport{}
}

func TestParseActivityTab(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"", "all"},
		{"all", "all"},
		{"posts", "posts"},
		{"votes", "votes"},
		{"disputes", "disputes"},
		{"resolved", "resolved"},
		{"not-resolved", "not-resolved"},
		{"Posts", "all"},
		{"POSTS", "all"},
		{"nonsense", "all"},
		{"posts ", "all"},
		{" posts", "all"},
		{"posts&tab=votes", "all"},
	}
	for _, c := range cases {
		if got := parseActivityTab(c.raw); got != c.want {
			t.Errorf("parseActivityTab(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestActivityTabHrefs(t *testing.T) {
	want := map[string]string{
		"all":          "/profile",
		"posts":        "/profile?tab=posts",
		"votes":        "/profile?tab=votes",
		"disputes":     "/profile?tab=disputes",
		"resolved":     "/profile?tab=resolved",
		"not-resolved": "/profile?tab=not-resolved",
	}
	view := wm5Cards(t, service.ActivityPage{}, "")
	if len(view.Tabs) != 6 {
		t.Fatalf("got %d tabs, want 6", len(view.Tabs))
	}
	var keys []string
	var labels []string
	for _, tab := range view.Tabs {
		keys = append(keys, tab.Key)
		labels = append(labels, tab.Label)
		if tab.Href != want[tab.Key] {
			t.Errorf("tab %q href = %q, want %q", tab.Key, tab.Href, want[tab.Key])
		}
	}
	if w := []string{"all", "posts", "votes", "disputes", "resolved", "not-resolved"}; !slices.Equal(keys, w) {
		t.Errorf("tab keys = %v, want %v", keys, w)
	}
	if w := []string{"All", "My posts", "My votes", "My disputes", "Resolved", "Not resolved"}; !slices.Equal(labels, w) {
		t.Errorf("tab labels = %v, want %v", labels, w)
	}
}

// TestActivityMembershipTruthTable is W-03 row by row, including which cards
// may offer Reopen (W-09).
func TestActivityMembershipTruthTable(t *testing.T) {
	type tc struct {
		name       string
		post       *service.ActivityReport
		vote       *service.VotedReport
		wantTabs   string
		wantReopen bool
		wantKind   string
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
		{"post live unexpired", post(service.VisibilityLive, false), nil, "posts not-resolved", false, "post"},
		{"post provisional expired", post(service.VisibilityProvisional, true), nil, "posts", false, "post"},
		{"post retracted unexpired", post(service.VisibilityRetracted, false), nil, "posts resolved", true, "post"},
		{"post retracted expired", post(service.VisibilityRetracted, true), nil, "posts resolved", false, "post"},
		{"post hidden unexpired", post(service.VisibilityHidden, false), nil, "posts not-resolved", false, "post"},
		{"vote confirm live", nil, vote(service.VoteConfirm, service.VisibilityLive, false), "votes not-resolved", false, "vote"},
		{"vote confirm hidden unexpired", nil, vote(service.VoteConfirm, service.VisibilityHidden, false), "votes not-resolved", false, "vote"},
		{"vote confirm retracted unexpired", nil, vote(service.VoteConfirm, service.VisibilityRetracted, false), "votes resolved", false, "vote"},
		{"vote dispute provisional", nil, vote(service.VoteDispute, service.VisibilityProvisional, false), "votes disputes not-resolved", false, "dispute"},
		{"vote dispute expired", nil, vote(service.VoteDispute, service.VisibilityProvisional, true), "votes disputes", false, "dispute"},
		{"vote dispute retracted expired", nil, vote(service.VoteDispute, service.VisibilityRetracted, true), "votes disputes resolved", false, "dispute"},
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
			card := wm5Card(t, wm5Cards(t, page, ""), 1)
			if card.Tabs != c.wantTabs {
				t.Errorf("Tabs = %q, want %q", card.Tabs, c.wantTabs)
			}
			if card.CanReopen != c.wantReopen {
				t.Errorf("CanReopen = %v, want %v", card.CanReopen, c.wantReopen)
			}
			if card.Kind != c.wantKind {
				t.Errorf("Kind = %q, want %q", card.Kind, c.wantKind)
			}
		})
	}
}

// wm5MixedPage holds four cards: a live post, a resolved post, a confirmed
// vote and a disputed vote. The counts are All 4, posts 2, votes 2, disputes 1,
// resolved 1, not resolved 3.
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

func TestActivityCountsEqualTokenCounts(t *testing.T) {
	view := wm5Cards(t, wm5MixedPage(), "")
	want := map[string]int{"all": 4, "posts": 2, "votes": 2, "disputes": 1, "resolved": 1, "not-resolved": 3}
	for _, tab := range view.Tabs {
		if tab.Count != want[tab.Key] {
			t.Errorf("tab %q count = %d, want %d", tab.Key, tab.Count, want[tab.Key])
		}
		n := 0
		for _, c := range view.Reports {
			if tab.Key == "all" || slices.Contains(strings.Fields(c.Tabs), tab.Key) {
				n++
			}
		}
		if n != tab.Count {
			t.Errorf("tab %q count = %d but %d cards carry its token", tab.Key, tab.Count, n)
		}
	}
	if len(view.Reports) != 4 {
		t.Errorf("got %d cards, want 4 distinct reports", len(view.Reports))
	}
}

func TestActivityHiddenAndShowEmptyFollowTheSelectedTab(t *testing.T) {
	t.Run("a selected tab with cards hides the rest", func(t *testing.T) {
		view := wm5Cards(t, wm5MixedPage(), "disputes")
		var visible []int64
		for _, c := range view.Reports {
			if !c.Hidden {
				visible = append(visible, c.ReportID)
			}
		}
		if !slices.Equal(visible, []int64{4}) {
			t.Errorf("visible cards = %v, want [4]", visible)
		}
		for _, tab := range view.Tabs {
			if tab.Selected != (tab.Key == "disputes") {
				t.Errorf("tab %q Selected = %v", tab.Key, tab.Selected)
			}
			if tab.ShowEmpty {
				t.Errorf("tab %q shows its empty state although the selected tab has a card", tab.Key)
			}
		}
	})

	t.Run("All hides nothing", func(t *testing.T) {
		view := wm5Cards(t, wm5MixedPage(), "bogus")
		for _, c := range view.Reports {
			if c.Hidden {
				t.Errorf("card %d hidden under All", c.ReportID)
			}
		}
		if !view.Tabs[0].Selected {
			t.Error("All is not selected for an unknown key")
		}
	})

	t.Run("a selected tab with no cards shows its empty state", func(t *testing.T) {
		page := service.ActivityPage{Posts: []service.ActivityReport{wm5Post(1, service.VisibilityLive, false)}}
		view := wm5Cards(t, page, "votes")
		for _, tab := range view.Tabs {
			want := tab.Key == "votes"
			if tab.ShowEmpty != want {
				t.Errorf("tab %q ShowEmpty = %v, want %v", tab.Key, tab.ShowEmpty, want)
			}
		}
		if !view.Reports[0].Hidden {
			t.Error("the post is not hidden under My votes")
		}
	})

	t.Run("an empty account shows the All empty state", func(t *testing.T) {
		view := wm5Cards(t, service.ActivityPage{}, "")
		if !view.Tabs[0].ShowEmpty || view.Tabs[0].EmptyTitle != "No activity yet" {
			t.Errorf("All tab = %+v, want a shown empty state", view.Tabs[0])
		}
		if len(view.Reports) != 0 || view.Record != nil {
			t.Errorf("empty account has %d cards and record %v", len(view.Reports), view.Record)
		}
	})
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
	for _, c := range view.Reports {
		got = append(got, c.ReportID)
	}
	if want := []int64{12, 9, 5}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
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
	if view.Reports[0].ReportID != 2 {
		t.Fatalf("fixture: the newest card is %d, want the vote 2", view.Reports[0].ReportID)
	}
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
		2: {"Your vote", "You confirmed this report", "activity-glyph--vote", "vote"},
		3: {"Your dispute", "You disputed this report", "activity-glyph--dispute", "dispute"},
	}
	for id, w := range cases {
		c := wm5Card(t, view, id)
		got := want{c.Tag, c.Action, c.IconGlyph, c.Kind}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("card %d = %+v, want %+v", id, got, w)
		}
	}
	if c := wm5Card(t, view, 1); c.When != "2 hours ago" || c.Detail != "Flood" {
		t.Errorf("post When %q Detail %q, want %q and %q", c.When, c.Detail, "2 hours ago", "Flood")
	}
	if c := wm5Card(t, view, 2); c.When != "10 min ago" {
		t.Errorf("vote When = %q, want 10 min ago (the time of the vote)", c.When)
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
