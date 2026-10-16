package handlers

import (
	"fmt"
	"sort"
	"time"

	"pinalert/internal/service"
)

// The five parts of My activity (Phase 3.4, D-18, UI-SPEC Surface 3). The keys
// are the only part values the page accepts from the address, matched exactly
// and case sensitively, so a hand typed or hostile value can never reach a link
// or a class name (T-34-30).
const (
	activityPartUpdates   = "updates"
	activityPartPosted    = "posted"
	activityPartConfirmed = "confirmed"
	activityPartDisputed  = "disputed"
	activityPartEnded     = "ended"
)

// The old six tab values of quick task 261013-wm5. They are read only aliases
// now: a bookmark with one of them lands on the part that holds the same cards
// (D-18). All, resolved and not-resolved have no part of their own and land on
// the list.
const (
	activityTabPosts    = "posts"
	activityTabVotes    = "votes"
	activityTabDisputes = "disputes"
)

// Card kinds. The kind picks the tag, the action phrase, the icon and the
// event colour of one card, and the part the card is listed in.
const (
	activityKindPost    = "post"
	activityKindVote    = "vote"
	activityKindDispute = "dispute"
)

// The status words of a report row (UI-SPEC P34-C12). The two moderation words
// are the Phase 4 words and are kept as they were built.
const (
	statusOngoing  = "Ongoing"
	statusResolved = "Resolved"
	statusDisputed = "Disputed"
	statusExpired  = "Expired"
	statusHeld     = "Waiting for a text check"
	statusRemoved  = "Removed after review"
)

// activityPartSpec is one part's fixed words: its key, its name, the one line
// the laptop shows under the title and the part screen shows on the phone, the
// hub icon (the file name without .svg) and the plain words its empty state
// shows.
type activityPartSpec struct {
	Key        string
	Name       string
	Line       string
	Icon       string
	EmptyTitle string
	EmptyText  string
}

// activityPartSpecs lists the five parts in the order the page draws them
// (D-18).
var activityPartSpecs = []activityPartSpec{
	{activityPartUpdates, "Updates", "Changes on reports you posted or voted on, newest first.", "bell",
		"No updates yet", "Changes on reports you posted or voted on will show here."},
	{activityPartPosted, "Posted by you", "Reports you posted. Open one to resolve or reopen it.", "circle-plus",
		"No reports yet", "Reports you post will appear here."},
	{activityPartConfirmed, "You confirmed", "Reports you said are true.", "check-circle",
		"No confirmations yet", "Reports you confirm will appear here."},
	{activityPartDisputed, "You disputed", "Reports you said are wrong.", "circle-x",
		"No disputes yet", "Reports you dispute will appear here."},
	{activityPartEnded, "Ended nearby", "Everyone's reports near you that ended in the last 7 days.", "history",
		"No ended reports nearby", "Nothing in this area was resolved or faded out in the last 7 days."},
}

// The hub summary words that do not depend on the rows (P34-C13).
const (
	hubUpdatesNone     = "No updates yet"
	hubPostedNone      = "Nothing posted yet"
	hubConfirmedNone   = "Nothing confirmed yet"
	hubDisputedNone    = "Nothing disputed yet"
	hubEndedSummary    = "Last 7 days, near your area"
	hubLatestPrefix    = "Latest: "
	activityListHref   = "/profile"
	activityPartPrefix = "/profile?part="
)

// activityPartHref is the address of one part screen. The key is always one of
// the five constants, never the raw request value.
func activityPartHref(key string) string {
	return activityPartPrefix + key
}

// parseActivityPart turns the raw part and tab values of the address into a part
// key, or the empty string for the list. Only an exact match counts: an empty,
// unknown, upper case or padded value gives the list. A valid part wins over the
// old tab value; the tab values posts, votes and disputes are read only aliases
// of Posted by you, You confirmed and You disputed (D-18, T-34-30).
func parseActivityPart(rawPart, rawTab string) string {
	for _, spec := range activityPartSpecs {
		if rawPart == spec.Key {
			return spec.Key
		}
	}
	switch rawTab {
	case activityTabPosts:
		return activityPartPosted
	case activityTabVotes:
		return activityPartConfirmed
	case activityTabDisputes:
		return activityPartDisputed
	}
	return ""
}

// activityPart is one part of the page: its words, its address, how many cards
// it holds and the cards themselves. Summary is the hub row's one line and
// ShowCount is false for the two parts the server holds no rows for (Updates is
// Phase 3.2's, Ended nearby is a read the browser makes).
type activityPart struct {
	Key        string
	Name       string
	Line       string
	Icon       string
	Summary    string
	Count      int
	ShowCount  bool
	Href       string
	Rows       []profileReport
	EmptyTitle string
	EmptyText  string
	Current    bool
}

// activityPageView is what buildActivityPage hands the Profile handler: the five
// parts in order, the reporter record card and the key of the part the address
// asked for (empty for the list).
type activityPageView struct {
	Parts       []activityPart
	Record      *profileRecord
	CurrentPart string
}

// CurrentName is the name of the current part, empty on the list. The phone top
// bar shows it as the title of a part screen.
func (v activityPageView) CurrentName() string {
	for _, p := range v.Parts {
		if p.Key == v.CurrentPart {
			return p.Name
		}
	}
	return ""
}

// activityEntry carries one card with the event time its position is sorted on.
type activityEntry struct {
	card profileReport
	at   time.Time
}

// partOfKind is the one part a card of this kind is listed in (D-21): a post is
// only in Posted by you, a confirm vote only in You confirmed and a dispute only
// in You disputed.
func partOfKind(kind string) string {
	switch kind {
	case activityKindPost:
		return activityPartPosted
	case activityKindDispute:
		return activityPartDisputed
	default:
		return activityPartConfirmed
	}
}

// buildActivityPage is the whole My activity view model: every card is in
// exactly one part, cards are ordered newest event first with ties by report id,
// and each part carries a real count of its cards and the hub row's one line.
// The record card is built from the account's own posts only, never from the
// merged list, whose first card can be someone else's report (W-10). Voted
// cards can never offer Reopen, whatever the report's state (W-09, T-wm5-05).
// current is the part key from parseActivityPart; any other value reads as the
// list. Nothing here changes a trust figure, it only places cards.
func buildActivityPage(page service.ActivityPage, current string, now time.Time) (activityPageView, error) {
	known := false
	for _, spec := range activityPartSpecs {
		if current == spec.Key {
			known = true
		}
	}
	if !known {
		current = ""
	}

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
		pr.WhenText = "Posted " + pr.When
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
		pr.When = profileRelativeTime(vr.VotedAt, now)
		if vr.Vote == service.VoteDispute {
			pr.Kind = activityKindDispute
			pr.Tag = "Your dispute"
			pr.Action = "You disputed this report"
			pr.WhenText = "You disputed " + pr.When
		} else {
			pr.Kind = activityKindVote
			pr.Tag = "Your vote"
			pr.Action = "You confirmed this report"
			pr.WhenText = "You confirmed " + pr.When
		}
		// The disc shows the report's own category glyph on every card (UI-SPEC
		// 3a); the part the card is in already says what the person did.
		pr.IconGlyph = pr.CategoryGlyph
		entries = append(entries, activityEntry{card: pr, at: vr.VotedAt})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].at.Equal(entries[j].at) {
			return entries[i].at.After(entries[j].at)
		}
		return entries[i].card.ReportID > entries[j].card.ReportID
	})

	rowsByPart := make(map[string][]profileReport, len(activityPartSpecs))
	for _, e := range entries {
		key := partOfKind(e.card.Kind)
		rowsByPart[key] = append(rowsByPart[key], e.card)
	}

	parts := make([]activityPart, 0, len(activityPartSpecs))
	for _, spec := range activityPartSpecs {
		rows := rowsByPart[spec.Key]
		p := activityPart{
			Key:        spec.Key,
			Name:       spec.Name,
			Line:       spec.Line,
			Icon:       spec.Icon,
			Href:       activityPartHref(spec.Key),
			Rows:       rows,
			Count:      len(rows),
			EmptyTitle: spec.EmptyTitle,
			EmptyText:  spec.EmptyText,
			Current:    spec.Key == current,
		}
		switch spec.Key {
		case activityPartUpdates:
			p.Summary = hubUpdatesNone
		case activityPartPosted:
			p.ShowCount = true
			p.Summary = postedSummary(rows)
		case activityPartConfirmed:
			p.ShowCount = true
			p.Summary = latestSummary(rows, hubConfirmedNone)
		case activityPartDisputed:
			p.ShowCount = true
			p.Summary = latestSummary(rows, hubDisputedNone)
		case activityPartEnded:
			p.Summary = hubEndedSummary
		}
		parts = append(parts, p)
	}

	view := activityPageView{Parts: parts, CurrentPart: current}
	if len(page.Posts) > 0 {
		view.Record = newProfileRecord(page.Posts[0].Trust.Reporter)
	}
	return view, nil
}

// postedSummary is the hub line of Posted by you: how many posts are still
// happening (open and not faded) and how many were resolved. A post that is
// neither, such as one that faded out unresolved, is in the count but in
// neither number.
func postedSummary(rows []profileReport) string {
	if len(rows) == 0 {
		return hubPostedNone
	}
	ongoing, resolved := 0, 0
	for _, r := range rows {
		switch {
		case r.Visibility == string(service.VisibilityRetracted):
			resolved++
		case !r.Expired && r.Moderation == "":
			ongoing++
		}
	}
	return fmt.Sprintf("%d still happening, %d resolved", ongoing, resolved)
}

// latestSummary is the hub line of a voted part: the text of the newest card
// whose text is shown. A finished Rescue report of someone else shows only its
// coarse title, so a card with real text is preferred and the coarse title is
// the fallback when every card is coarse.
func latestSummary(rows []profileReport, none string) string {
	if len(rows) == 0 {
		return none
	}
	for _, r := range rows {
		if !r.Approximate {
			return hubLatestPrefix + r.Description
		}
	}
	return hubLatestPrefix + rows[0].Description
}
