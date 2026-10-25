// pushcopy.go writes every word a phone lock screen can show (plan 03.3-03).
// A notification is built here and only here, from the closed inputs of
// PushTextInput. The reporter's words, names, phone numbers, addresses and
// coordinates have no field, so they can never reach a notification (D-14).
// The area name is coarse and used by push only (D-19, D-20). Whether the
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
// (they need a home area), Changed with an event that is not one of the four
// change events, a category or severity that has no word where one is needed,
// and the kinds confirmed_update, official, follow or an unknown one.
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
		body := subject + "."
		hasDistance := distanceOK(in.DistanceKm)
		switch {
		case area != "" && hasDistance:
			body += " " + area + ", " + DistanceWords(*in.DistanceKm) + " " + fromHomeAreaWords
		case area != "":
			body += " " + area + "."
		case hasDistance:
			body += " " + DistanceWords(*in.DistanceKm) + " " + fromHomeAreaWords
		}
		return PushText{Title: whose + " " + event, Body: body}, true
	}
	return PushText{}, false
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
