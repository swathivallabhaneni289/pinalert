// pushcopy.go writes every word a phone lock screen can show (plan 03.3-03).
// A notification is built here and only here, from the closed inputs of
// PushTextInput. The reporter's words, names, phone numbers, addresses and
// coordinates have no field, so they can never reach a notification (D-14).
// The area name is coarse (D-19, D-20); since quick 261119-sps the update rows
// of the bell show it too, cut by AreaWords like here. Whether the
// words say Not confirmed yet is decided by the caller from ConfirmedByPeople
// and passed in as Confirmed, never from the resolve reason (decision 2, which
// supersedes D-17). The site asks every notification to make no noise; that is
// the service worker's job, and no word here may mention it (owner rule
// 2026-10-18). Nothing here does I/O.

package service

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	// AreaMaxRunes is the longest area name on a lock screen, the ellipsis
	// included (UI-SPEC 3a).
	AreaMaxRunes = 28
	// PushTitleMaxRunes is the title budget. UI-SPEC 3b says "about 50"; the
	// longest title the closed words can build is a no pick Critical title,
	// "Storm/Cyclone damage, Critical near " plus a 28 rune area, which is 64
	// runes, so the hard ceiling the test enforces is 64.
	PushTitleMaxRunes = 64
	// PushBodyMaxRunes is the body budget.
	PushBodyMaxRunes = 120

	// NotConfirmedYetWords closes a Critical or Reliable body until people
	// nearby confirm the report.
	NotConfirmedYetWords = "Not confirmed yet."
	// ConfirmedByPeopleWords opens the body once people nearby confirm it.
	ConfirmedByPeopleWords = "Confirmed by people nearby."
	// FromReliableReporterWords opens a Reliable reporter body.
	FromReliableReporterWords = "From a Reliable reporter."
	// FromSomeoneYouFollowWords opens a follow note body (Phase 9 plan 25,
	// UI-SPEC 2b, P9-C21). It names nobody: a lock screen is read by bystanders,
	// so no username, no report words and no coordinates ever appear (T-09-85).
	FromSomeoneYouFollowWords = "From someone you follow."
)

const (
	ellipsisWord       = "…"
	yourReportWords    = "Your report"
	votedReportWords   = "A report you voted on"
	fromHomeAreaWords  = "from your home area."
	underOneKmWords    = "Under 1 km"
	criticalWordInName = "Critical"
)

// PushTextInput is everything BuildPushText may read. It has no field for the
// reporter's text, a name, a phone number, an address or a coordinate, and
// TestBuildPushTextHasNoReporterTextField keeps it that way.
type PushTextInput struct {
	// Kind is the notification kind. A confirmed update is built by passing the
	// original kind (critical or reliable) with Confirmed true.
	Kind PushKind
	// Happening is the What is happening pick, Critical reports only. Nil for
	// older reports and for non Critical ones.
	Happening *Happening
	Category  Category
	Severity  Severity
	// AreaName is the coarse stored area name. May be empty.
	AreaName string
	// DistanceKm is the distance from the person's home area. Nil when the
	// person has no home area.
	DistanceKm *float64
	// Confirmed is true once people nearby confirm the report, decided by the
	// caller from ConfirmedByPeople.
	Confirmed bool
	// Event is the change a Changed notification announces.
	Event UpdateKind
	// OwnReport is true when the person posted the report.
	OwnReport bool
}

// PushText is the title and body of one notification.
type PushText struct {
	Title string
	Body  string
}

// HappeningWord is the words of one What is happening pick, or "" for a value
// that is not one of the seven.
func HappeningWord(h Happening) string {
	switch h {
	case HappeningWaterRising:
		return "Water rising"
	case HappeningPeopleTrapped:
		return "People trapped"
	case HappeningFire:
		return "Fire"
	case HappeningBuildingCollapsed:
		return "Building collapsed"
	case HappeningLandslide:
		return "Landslide"
	case HappeningMedicalEmergency:
		return "Medical emergency"
	case HappeningOther:
		return "Other"
	}
	return ""
}

// CategoryWord is the words of one category, the same as the profile page
// shows, or "" for a value that is not a category.
func CategoryWord(c Category) string {
	switch c {
	case CategoryFlood:
		return "Flood"
	case CategoryEarthquake:
		return "Earthquake"
	case CategoryFire:
		return "Fire"
	case CategoryStormCyclone:
		return "Storm/Cyclone damage"
	case CategoryRoadBlocked:
		return "Road blocked"
	case CategoryPowerOutage:
		return "Power outage"
	case CategoryShelterOpen:
		return "Shelter open"
	case CategoryTraffic:
		return "Traffic"
	case CategoryRescueNeeded:
		return "Rescue needed"
	case CategoryOther:
		return "Other"
	}
	return ""
}

// SeverityWord is the word of one severity, or "" for an unknown value.
func SeverityWord(s Severity) string {
	switch s {
	case SeverityLow:
		return "Low"
	case SeverityMedium:
		return "Medium"
	case SeverityCritical:
		return criticalWordInName
	}
	return ""
}

// DistanceWords gives whole kilometres from the home area: "Under 1 km" below
// one kilometre, otherwise the rounded number. It never gives metres.
func DistanceWords(km float64) string {
	if km < 1 {
		return underOneKmWords
	}
	return strconv.Itoa(int(math.Round(km))) + " km"
}

// AreaWords trims the area name and, when it is longer than AreaMaxRunes,
// cuts it at a word end and ends it with an ellipsis, so the result is never
// longer than AreaMaxRunes runes. A name with no space is cut at
// AreaMaxRunes-1 runes. An empty name stays empty.
func AreaWords(name string) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) <= AreaMaxRunes {
		return string(runes)
	}
	room := AreaMaxRunes - 1
	cut := room
	if runes[room] != ' ' {
		cut = -1
		for i := room - 1; i > 0; i-- {
			if runes[i] == ' ' {
				cut = i
				break
			}
		}
		if cut < 0 {
			cut = room
		}
	}
	return strings.TrimRight(string(runes[:cut]), " ") + ellipsisWord
}

// distanceOK reports whether d is a usable distance: set and a finite number.
func distanceOK(d *float64) bool {
	return d != nil && !math.IsNaN(*d) && !math.IsInf(*d, 0)
}

// pickOrCategory gives the words of the pick (empty when the report has none
// or an unknown one) and of the category (empty when unknown).
func pickOrCategory(in PushTextInput) (pick, category string) {
	if in.Happening != nil {
		pick = HappeningWord(*in.Happening)
	}
	category = CategoryWord(in.Category)
	return pick, category
}

// titleWithArea joins a subject and an area: "{subject} near {area}", or the
// subject alone when there is no area.
func titleWithArea(subject, area string) string {
	if area == "" {
		return subject
	}
	return subject + " near " + area
}

// BuildPushText builds the title and body of one notification. ok is false for
// an input no notification exists for: Critical or Reliable without a distance
// (they need a home area), Changed with an event that is not one of the six
// change events (the four 3.2 events and the two comment events), a category or severity that has no word where one is needed,
// and the kinds confirmed_update, official or an unknown one. A follow note is
// not about distance: it is built without a distance too (Phase 9 plan 25), so
// a person with no home area still gets it.
func BuildPushText(in PushTextInput) (PushText, bool) {
	area := AreaWords(in.AreaName)
	switch in.Kind {
	case PushKindCritical:
		if !distanceOK(in.DistanceKm) {
			return PushText{}, false
		}
		pick, category := pickOrCategory(in)
		subject := pick
		if subject == "" {
			if category == "" {
				return PushText{}, false
			}
			subject = category + ", " + criticalWordInName
		}
		return PushText{
			Title: titleWithArea(subject, area),
			Body:  homeAreaBody(in, false),
		}, true

	case PushKindReliable:
		if !distanceOK(in.DistanceKm) {
			return PushText{}, false
		}
		category := CategoryWord(in.Category)
		if category == "" {
			return PushText{}, false
		}
		return PushText{
			Title: titleWithArea(category, area),
			Body:  homeAreaBody(in, true),
		}, true

	case PushKindFollow:
		// Phase 9 plan 25 (D-09, UI-SPEC 2b, P9-C21). The title is the pick for a
		// Critical report, else the category, then the area, and names nobody.
		// The body opens with who it is from in closed words only; the distance
		// part is left out when there is no home area, and the note is still
		// built. A confirmed note reads the same as a confirmed Reliable one.
		pick, category := pickOrCategory(in)
		subject := pick
		if subject == "" {
			subject = category
		}
		if subject == "" {
			return PushText{}, false
		}
		return PushText{
			Title: titleWithArea(subject, area),
			Body:  followBody(in),
		}, true

	case PushKindChanged:
		event, ok := changeEventWords(in.Event)
		if !ok {
			return PushText{}, false
		}
		pick, category := pickOrCategory(in)
		subject := pick
		if subject == "" {
			severity := SeverityWord(in.Severity)
			if category == "" || severity == "" {
				return PushText{}, false
			}
			subject = category + ", " + severity
		}
		whose := votedReportWords
		if in.OwnReport {
			whose = yourReportWords
		}
		if isCommentEvent(in.Event) {
			// Phase 8 plan 15 (D-16, UI-SPEC P8-C16): the words go after the
			// report's category, never after "Your report", because a reply
			// note goes to the author of a comment who may not own the report.
			// The body is the one every Changed note has: it holds no comment
			// words, no username and no count (03.3 D-14).
			if category == "" {
				return PushText{}, false
			}
			return PushText{
				Title: commentTitle(category, area, in.Event),
				Body:  changedBody(subject, area, in.DistanceKm),
			}, true
		}
		return PushText{Title: whose + " " + event, Body: changedBody(subject, area, in.DistanceKm)}, true
	}
	return PushText{}, false
}

// changedBody is the body of every Changed note: the subject, then the area and
// the distance from the home area when known.
func changedBody(subject, area string, distanceKm *float64) string {
	body := subject + "."
	hasDistance := distanceOK(distanceKm)
	switch {
	case area != "" && hasDistance:
		body += " " + area + ", " + DistanceWords(*distanceKm) + " " + fromHomeAreaWords
	case area != "":
		body += " " + area + "."
	case hasDistance:
		body += " " + DistanceWords(*distanceKm) + " " + fromHomeAreaWords
	}
	return body
}

// isCommentEvent says whether e is one of the two comment events (Phase 8).
func isCommentEvent(e UpdateKind) bool {
	return e == UpdateCommentOnYours || e == UpdateReplyToYours
}

// commentTitle is the title of a comment note: "{Category} report near {area}
// has a new comment", or "{Category} report has a new comment" when the area
// is empty or the whole line would pass PushTitleMaxRunes (the longest category
// with the longest area does). The category is never cut, so the title keeps
// the 64 rune cap by dropping the area instead.
func commentTitle(category, area string, e UpdateKind) string {
	words, _ := changeEventWords(e)
	subject := category + " report"
	if area != "" {
		withArea := titleWithArea(subject, area) + " " + words
		if utf8.RuneCountInString(withArea) <= PushTitleMaxRunes {
			return withArea
		}
	}
	return subject + " " + words
}

// homeAreaBody is the body of a Critical or Reliable notification. Once
// confirmed both read the same; before, the Reliable one opens with who
// posted it, and both close with Not confirmed yet.
func homeAreaBody(in PushTextInput, reliable bool) string {
	distance := DistanceWords(*in.DistanceKm) + " " + fromHomeAreaWords
	if in.Confirmed {
		return ConfirmedByPeopleWords + " " + distance
	}
	if reliable {
		return FromReliableReporterWords + " " + distance + " " + NotConfirmedYetWords
	}
	return distance + " " + NotConfirmedYetWords
}

// followBody is the body of a follow note: "From someone you follow. {Distance}
// from your home area. Not confirmed yet.", or without the distance part when
// the person has no home area. Once confirmed it is "Confirmed by people
// nearby. {Distance} from your home area." or "Confirmed by people nearby.".
func followBody(in PushTextInput) string {
	distance := ""
	if distanceOK(in.DistanceKm) {
		distance = " " + DistanceWords(*in.DistanceKm) + " " + fromHomeAreaWords
	}
	if in.Confirmed {
		return ConfirmedByPeopleWords + distance
	}
	return FromSomeoneYouFollowWords + distance + " " + NotConfirmedYetWords
}

// changeEventWords is the event half of a Changed title, the lock screen form
// of 3.2's event lines. ok is false for any other kind, more and the push only
// kind included.
func changeEventWords(e UpdateKind) (string, bool) {
	switch e {
	case UpdateReachedLive:
		return "is now confirmed", true
	case UpdateHidden:
		return "is now disputed", true
	case UpdateResolved:
		return "was marked resolved", true
	case UpdateFading:
		return "fades in about an hour", true
	case UpdateCommentOnYours:
		// Phase 8 plan 15 (D-16, P8-C16). Fixed words only: no comment text, no
		// username and no count ever reach a lock screen (03.3 D-14).
		return "has a new comment", true
	case UpdateReplyToYours:
		return "has a new reply", true
	}
	return "", false
}

// AlertExample is the sample the onboarding step 5 shows (UI-SPEC P33-C5),
// built by BuildPushText from fixed input so the example can never drift from
// a real notification.
func AlertExample() PushText {
	pick := HappeningWaterRising
	km := 2.0
	text, _ := BuildPushText(PushTextInput{
		Kind:       PushKindCritical,
		Happening:  &pick,
		Category:   CategoryFlood,
		Severity:   SeverityCritical,
		AreaName:   "Central Market",
		DistanceKm: &km,
	})
	return text
}
