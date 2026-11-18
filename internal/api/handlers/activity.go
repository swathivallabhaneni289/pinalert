package handlers

import (
	"sort"
	"strconv"
	"time"

	"pinalert/internal/service"
)

// The parts of My activity (Phase 3.4, D-18, UI-SPEC Surface 3). The keys are
// the only part values the page accepts from the address, matched exactly and
// case sensitively, so a hand typed or hostile value can never reach a link or
// a class name (T-34-30).
//
// Amended 2026-11-17 (quick 261117-vuf, owner, 7.1 UAT: "can we like shorten
// those names", "You'll see the post twice, right?"): Posted left, because your
// posts are on your profile. The list shows three parts, Confirmed, Disputed and
// Ended. Updates is no longer a part of the list: it is the screen the bell's
// "See all updates" opens (/profile?part=updates), drawn alone.
const (
	activityPartUpdates   = "updates"
	activityPartConfirmed = "confirmed"
	activityPartDisputed  = "disputed"
	activityPartEnded     = "ended"
)

// The old six tab values of quick task 261013-wm5. They are read only aliases
// now: a bookmark with one of them lands on the part that holds the same cards
// (D-18). All, resolved and not-resolved have no part of their own and land on
// the list. Amended 2026-11-17 (quick 261117-vuf): posts held the posts, whose
// part left, so it lands on the list too.
const (
	activityTabVotes    = "votes"
	activityTabDisputes = "disputes"
)

// Card kinds. The kind picks the tag, the action phrase, the icon and the
// event colour of one card, and the part the card is listed in. Amended
// 2026-11-17 (quick 261117-vuf): the post kind left with Posted.
const (
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

// activityPartSpecs lists the parts in the order buildActivityPage builds them
// (D-18). Amended 2026-11-17 (quick 261117-vuf): Posted left and the names are
// one word each. Updates stays first here so its screen can still be built and
// filled, but activityShownParts keeps it out of the list and the laptop stack.
var activityPartSpecs = []activityPartSpec{
	{activityPartUpdates, "Updates", "Changes on reports you posted or voted on, newest first.", "bell",
		"No updates yet", "Changes on reports you posted or voted on will show here."},
	{activityPartConfirmed, "Confirmed", "Reports you said are true.", "check-circle",
		"No confirmations yet", "Reports you confirm will appear here."},
	{activityPartDisputed, "Disputed", "Reports you said are wrong.", "circle-x",
		"No disputes yet", "Reports you dispute will appear here."},
	{activityPartEnded, "Ended", "Everyone's reports near you that ended in the last 7 days.", "history",
		"No ended reports nearby", "Nothing in this area was resolved or faded out in the last 7 days."},
}

// The hub summary words that do not depend on the rows (P34-C13).
const (
	hubUpdatesNone     = "No updates yet"
	hubConfirmedNone   = "Nothing confirmed yet"
	hubDisputedNone    = "Nothing disputed yet"
	hubEndedSummary    = "Last 7 days, near your area"
	hubLatestPrefix    = "Latest: "
	activityPartPrefix = "/profile?part="
)

// Quick 261117-lq0 (owner, 2026-11-17): My activity is no longer a main place.
// activityTitle is its phone top bar title on the list, and settingsHref is
// where its back arrow goes on the list and on a part, because Settings is
// where My activity is opened from now. Amended 2026-11-17 (quick 261117-vuf):
// Settings holds one My activity row now, and the Updates screen, which the
// bell opens, takes its back arrow to Nearby (nearbyHref), where the bell is.
const (
	activityTitle = "My activity"
	settingsHref  = "/settings"
	nearbyHref    = "/"
)

// activityPartHref is the address of one part screen. The key is always one of
// the part constants, never the raw request value.
func activityPartHref(key string) string {
	return activityPartPrefix + key
}

// activityShownParts is the parts the page draws (quick 261117-vuf, owner
// 2026-11-17): on the Updates screen only Updates, on both sizes; everywhere
// else, the list and the three part screens, Confirmed, Disputed and Ended,
// with no Updates. The hub and the laptop stack range over exactly these.
func activityShownParts(parts []activityPart, current string) []activityPart {
	shown := make([]activityPart, 0, len(parts))
	for _, p := range parts {
		if (p.Key == activityPartUpdates) == (current == activityPartUpdates) {
			shown = append(shown, p)
		}
	}
	return shown
}

// parseActivityPart turns the raw part and tab values of the address into a part
// key, or the empty string for the list. Only an exact match counts: an empty,
// unknown, upper case or padded value gives the list. A valid part wins over the
// old tab value; the tab values votes and disputes are read only aliases of
// Confirmed and Disputed (D-18, T-34-30). Amended 2026-11-17 (quick
// 261117-vuf): part=posted and tab=posts are no part now and give the list.
func parseActivityPart(rawPart, rawTab string) string {
	for _, spec := range activityPartSpecs {
		if rawPart == spec.Key {
			return spec.Key
		}
	}
	switch rawTab {
	case activityTabVotes:
		return activityPartConfirmed
	case activityTabDisputes:
		return activityPartDisputed
	}
	return ""
}

// activityPart is one part of the page: its words, its address, how many cards
// it holds and the cards themselves. Summary is the hub row's one line.
// ShowCount is false for Ended nearby, whose rows are a read the browser
// makes; Updates shows 0 until Phase 3.2 fills it. Amended 2026-11-18 (owner on
// the 7.1 UAT, "showing 0 ... looks odd"): a part with no rows shows no count,
// so ShowCount is also false when Count is 0.
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
	// The next four are filled only for the Updates part, by fillUpdatesPart
	// (Phase 3.2). Updates are its rows, NewCount is how many are unread (capped
	// at 99, the hub's "{N} new" tag), UpdatesCut is true when more updates are
	// kept than are listed (the "Showing the newest 50." line) and UpdatesFailed
	// is true when the read failed (the part shows its error state).
	Updates       []activityUpdateRow
	NewCount      int
	UpdatesCut    bool
	UpdatesFailed bool
}

// activityUpdateRow is one row of the Updates part. Every word is written on the
// server by the same code the bell reads (D-05), so the page works before any
// script runs. Href is empty for a row that is not a link: the rolled up row and
// a report that is no longer available (UI-SPEC A3). It holds no account,
// session, cell or coordinate.
type activityUpdateRow struct {
	ID        int64
	Href      string
	Available bool
	Unread    bool
	IsMore    bool
	Event     string
	// ReportLine is the first line of the report's text under the Rescue rule.
	ReportLine string
	Category   string
	Severity   string
	// SeverityClass is the closed class of the severity dot, empty when the row
	// has none.
	SeverityClass string
	Trust         string
	When          string
	// Label is the row's accessible name (UI-SPEC P32-C8).
	Label string
	// OwnPlain marks the plain row that says the reader's own report is hidden
	// for review. Unlike the other plain rows it carries the new mark while
	// unread, as the bell does (quick 261117-vvb review, 2026-11-18).
	OwnPlain bool
	// ReviewReportID is set only on the removed and stays removed rows while
	// the reader's report is still removed (quick 261118-rz6): the row is a
	// button that opens the reader's box for that report, and carries the new
	// mark while unread like a link row. Zero on every other row.
	ReviewReportID int64
}

// updateSeverityClass is the closed lookup behind the severity dot class: only
// the three known severities give a class, so no stored value is ever written
// into a class attribute.
func updateSeverityClass(s service.Severity) string {
	switch s {
	case service.SeverityLow:
		return "sev-low"
	case service.SeverityMedium:
		return "sev-medium"
	case service.SeverityCritical:
		return "sev-critical"
	default:
		return ""
	}
}

// updatesHubSummary is the hub row's one line for the newest update: "{Line 1}
// · {Category}" for an event row, and the row's own line for the Reliable note,
// the follow note (Phase 9, D-09: its line already names the person and the
// category), an unavailable report and the rolled up row (UI-SPEC B2).
func updatesHubSummary(item service.UpdateItem) string {
	switch {
	case item.Kind == service.UpdateReliableNearby, item.Kind == service.UpdateFollowedPosted, !item.Available, item.Kind == service.UpdateMore, item.Category == "":
		return item.Event
	default:
		return item.Event + " · " + updateCategoryWord(item.Category)
	}
}

// fillUpdatesPart puts the answer of UpdatesService.List into the Updates part
// of the view (D-05). It touches no other part. With failed true the part is
// flagged and shows no rows; the page still renders (UI-SPEC B3 error state).
// buildActivityPage keeps its signature and its empty Updates part, so the Profile
// handler calls this after it.
func fillUpdatesPart(view *activityPageView, page service.UpdatesPage, failed bool, now time.Time) {
	for i := range view.Parts {
		p := &view.Parts[i]
		if p.Key != activityPartUpdates {
			continue
		}
		if failed {
			p.UpdatesFailed = true
			return
		}
		rows := make([]activityUpdateRow, 0, len(page.Items))
		for _, item := range page.Items {
			row := activityUpdateRow{
				ID:            item.ID,
				Available:     item.Available,
				Unread:        item.Unread,
				IsMore:        item.Kind == service.UpdateMore,
				OwnPlain:      item.Available && item.Kind == service.UpdateOwnReview,
				Event:         item.Event,
				ReportLine:    item.ReportLine,
				Category:      updateCategoryWord(item.Category),
				Severity:      updateSeverityWord(item.Severity),
				SeverityClass: updateSeverityClass(item.Severity),
				Trust:         item.Trust,
				When:          profileRelativeTime(item.CreatedAt, now),
				Label:         updateRowLabel(item, now),
			}
			if item.Available && item.ReportID != nil && item.Kind != service.UpdateMore {
				row.Href = "/?report=" + strconv.FormatInt(*item.ReportID, 10)
			}
			// Quick 261118-rz6 (replacing vvb's Policy link): the rows that say
			// the reader's own report was removed or stays removed open the
			// reader's box while it is still removed, like the bell's rows. The
			// other rows about the reader's own report link only through
			// ReportID above, so the hidden row stays a plain row.
			if item.ReviewReportID != nil {
				row.ReviewReportID = *item.ReviewReportID
			}
			rows = append(rows, row)
		}
		p.Updates = rows
		p.Count = len(rows)
		p.ShowCount = p.Count > 0
		p.NewCount = page.Unread
		p.UpdatesCut = page.Total > len(rows)
		if len(page.Items) > 0 {
			p.Summary = updatesHubSummary(page.Items[0])
		}
		return
	}
}

// activityPageView is what buildActivityPage hands the Profile handler: every
// part in the order of activityPartSpecs and the key of the part the address
// asked for (empty for the list). Amended 2026-11-17 (quick 261117-vuf): the
// reporter record card left with Posted, and activityShownParts picks the parts
// a screen draws.
type activityPageView struct {
	Parts       []activityPart
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

// partOfKind is the one part a card of this kind is listed in (D-21): a confirm
// vote only in Confirmed and a dispute only in Disputed.
func partOfKind(kind string) string {
	if kind == activityKindDispute {
		return activityPartDisputed
	}
	return activityPartConfirmed
}

// buildActivityPage is the whole My activity view model: every card is in
// exactly one part, cards are ordered newest event first with ties by report id,
// and each part carries a real count of its cards and the hub row's one line.
// current is the part key from parseActivityPart; any other value reads as the
// list. Nothing here changes a trust figure, it only places cards.
//
// Amended 2026-11-17 (quick 261117-vuf): the account's own posts are no longer
// cards here (they are on the profile), so page.Posts is not read, and the
// reporter record card built from them left too. Every card is a voted card, and
// a voted card never offers Reopen, whatever the report's state (W-09,
// T-wm5-05): the page has no Reopen control at all now.
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

	entries := make([]activityEntry, 0, len(page.Votes))
	for _, vr := range page.Votes {
		pr, err := newProfileReport(vr.ActivityReport, now)
		if err != nil {
			return activityPageView{}, err
		}
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
			// Phase 3.2: fillUpdatesPart fills the rows, the count, the "{N} new"
			// tag and the summary after this; here it starts as the empty part,
			// which shows no count.
			p.Summary = hubUpdatesNone
		case activityPartConfirmed:
			p.ShowCount = len(rows) > 0
			p.Summary = latestSummary(rows, hubConfirmedNone)
		case activityPartDisputed:
			p.ShowCount = len(rows) > 0
			p.Summary = latestSummary(rows, hubDisputedNone)
		case activityPartEnded:
			p.Summary = hubEndedSummary
		}
		parts = append(parts, p)
	}

	return activityPageView{Parts: parts, CurrentPart: current}, nil
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
