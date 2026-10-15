package handlers

import (
	"sort"
	"strings"
	"time"

	"pinalert/internal/service"
)

// The Activity page's six tabs (quick task 261013-wm5, W-01). The keys are the
// only tab values the page accepts from the address, matched exactly and case
// sensitively, so a hand typed or hostile value can never reach a link or a
// class name (T-wm5-03).
const (
	activityTabAll         = "all"
	activityTabPosts       = "posts"
	activityTabVotes       = "votes"
	activityTabDisputes    = "disputes"
	activityTabResolved    = "resolved"
	activityTabNotResolved = "not-resolved"
)

// Card kinds. The kind picks the tag, the action phrase, the icon and the
// event colour of one card.
const (
	activityKindPost    = "post"
	activityKindVote    = "vote"
	activityKindDispute = "dispute"
)

// activityTabSpec is one tab's fixed words: its key, its label and the plain
// words its empty state shows.
type activityTabSpec struct {
	Key        string
	Label      string
	EmptyTitle string
	EmptyText  string
}

// activityTabSpecs lists the six tabs in the order the page draws them (W-01).
var activityTabSpecs = []activityTabSpec{
	{activityTabAll, "All", "No activity yet", "Reports you post, confirm or dispute will appear here."},
	{activityTabPosts, "My posts", "No reports yet", "Reports you submit will appear here."},
	{activityTabVotes, "My votes", "No votes yet", "Reports you confirm or dispute will appear here."},
	{activityTabDisputes, "My disputes", "No disputes yet", "Reports you dispute will appear here."},
	{activityTabResolved, "Resolved", "Nothing resolved yet", "Reports marked resolved will appear here."},
	{activityTabNotResolved, "Not resolved", "Nothing needs attention", "Reports that are still active and not resolved will appear here."},
}

// activityTab is one tab link of the page: its words, its address, how many
// distinct reports it holds and whether it is the selected one. ShowEmpty is
// true only for the selected tab when it holds no report, so the server draws
// the right empty state with no script.
type activityTab struct {
	Key        string
	Label      string
	Href       string
	Count      int
	Selected   bool
	ShowEmpty  bool
	EmptyTitle string
	EmptyText  string
}

// parseActivityTab turns the raw tab value of the address into a tab key. Only
// an exact match of one of the six keys is honoured; an empty, unknown, upper
// case or padded value gives All (W-01).
func parseActivityTab(raw string) string {
	for _, spec := range activityTabSpecs {
		if raw == spec.Key {
			return spec.Key
		}
	}
	return activityTabAll
}

// activityTabHref is the address of one tab. All is /profile, because the
// default is left out of the address (W-01). The key is always one of the six
// constants, never the raw request value.
func activityTabHref(key string) string {
	if key == activityTabAll {
		return "/profile"
	}
	return "/profile?tab=" + key
}

// activityPageView is what buildActivityPage hands the Profile handler: the
// cards in order, the six tabs and the reporter record card.
type activityPageView struct {
	Reports []profileReport
	Tabs    []activityTab
	Record  *profileRecord
}

// activityTokens lists the tabs a card belongs to besides All (W-03). A voted
// report is under My votes and a disputed one also under My disputes. Resolved
// is the display state Resolved, expired or not, because Resolved wins over
// Expired (State Matrix 10a and 10b). Not resolved is a report that is neither
// expired nor retracted. A past or expired hidden report sits only under All
// and its kind tab (03.2 open point 1).
func activityTokens(kind, visibility string, expired bool) []string {
	var tokens []string
	switch kind {
	case activityKindPost:
		tokens = append(tokens, activityTabPosts)
	case activityKindVote:
		tokens = append(tokens, activityTabVotes)
	case activityKindDispute:
		tokens = append(tokens, activityTabVotes, activityTabDisputes)
	}
	retracted := visibility == string(service.VisibilityRetracted)
	if retracted {
		tokens = append(tokens, activityTabResolved)
	}
	if !retracted && !expired {
		tokens = append(tokens, activityTabNotResolved)
	}
	return tokens
}

// activityEntry carries one card with the event time its position is sorted on.
type activityEntry struct {
	card profileReport
	at   time.Time
}

// buildActivityPage is the whole Activity page view model: every card is on
// exactly one list (W-04), cards are ordered newest event first (W-05), the six
// tabs carry real counts of distinct reports (W-02), and the selected tab marks
// which cards start hidden and which empty state shows. The record card is
// built from the account's own posts only, never from the merged list, whose
// first card can be someone else's report (W-10). Voted cards can never offer
// Reopen, whatever the report's state (W-09, T-wm5-05).
func buildActivityPage(page service.ActivityPage, rawTab string, now time.Time) (activityPageView, error) {
	selected := parseActivityTab(rawTab)

	entries := make([]activityEntry, 0, len(page.Posts)+len(page.Votes))
	for _, ar := range page.Posts {
		pr, err := newProfileReport(ar, now)
		if err != nil {
			return activityPageView{}, err
		}
		pr.Kind = activityKindPost
		pr.Tag = "Your post"
		pr.Action = "You posted this report"
		pr.When = profileRelativeTime(ar.Row.CreatedAt, now)
		pr.IconGlyph = pr.CategoryGlyph
		entries = append(entries, activityEntry{card: pr, at: ar.Row.CreatedAt})
	}
	for _, vr := range page.Votes {
		pr, err := newProfileReport(vr.ActivityReport, now)
		if err != nil {
			return activityPageView{}, err
		}
		pr.CanReopen = false
		// The "used both undos" line is about the poster's own post. Clear it here
		// so a voted card never prints it, whatever UndosLeft holds (IN-03).
		pr.UndosUsed = false
		if vr.Vote == service.VoteDispute {
			pr.Kind = activityKindDispute
			pr.Tag = "Your dispute"
			pr.Action = "You disputed this report"
			pr.IconGlyph = "activity-glyph--dispute"
		} else {
			pr.Kind = activityKindVote
			pr.Tag = "Your vote"
			pr.Action = "You confirmed this report"
			pr.IconGlyph = "activity-glyph--vote"
		}
		pr.When = profileRelativeTime(vr.VotedAt, now)
		entries = append(entries, activityEntry{card: pr, at: vr.VotedAt})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].at.Equal(entries[j].at) {
			return entries[i].at.After(entries[j].at)
		}
		return entries[i].card.ReportID > entries[j].card.ReportID
	})

	counts := make(map[string]int, len(activityTabSpecs))
	cards := make([]profileReport, 0, len(entries))
	for _, e := range entries {
		card := e.card
		tokens := activityTokens(card.Kind, card.Visibility, card.Expired)
		card.Tabs = strings.Join(tokens, " ")
		hasSelected := selected == activityTabAll
		for _, tok := range tokens {
			counts[tok]++
			if tok == selected {
				hasSelected = true
			}
		}
		card.Hidden = !hasSelected
		cards = append(cards, card)
	}
	counts[activityTabAll] = len(cards)

	tabs := make([]activityTab, 0, len(activityTabSpecs))
	for _, spec := range activityTabSpecs {
		isSelected := spec.Key == selected
		tabs = append(tabs, activityTab{
			Key:        spec.Key,
			Label:      spec.Label,
			Href:       activityTabHref(spec.Key),
			Count:      counts[spec.Key],
			Selected:   isSelected,
			ShowEmpty:  isSelected && counts[spec.Key] == 0,
			EmptyTitle: spec.EmptyTitle,
			EmptyText:  spec.EmptyText,
		})
	}

	view := activityPageView{Reports: cards, Tabs: tabs}
	if len(page.Posts) > 0 {
		view.Record = newProfileRecord(page.Posts[0].Trust.Reporter)
	}
	return view, nil
}
