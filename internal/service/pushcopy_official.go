// pushcopy_official.go is the only writer of official lock screen words (plan
// 05-11, owner decisions D-11 and D-16, UI-SPEC 6c). Every input is a closed
// value chosen by internal/official from our own tables: the boundary file's
// district and state names, the closed event table of officialwords.go and the
// issuer table below. The feed's headline, description and instruction have no
// field, so they can never reach a notification (T-05-57), and the feed's own
// words never become a phrase.
//
// The notification is silent on every platform: the service worker asks for no
// noise, and no word here may mention it (owner rule 2026-10-18). iPhone's own
// settings may still decide, which is a known limit told to the owner.
//
// Topic and Tag derive from the alert's guid, so a later version of the same
// alert replaces the shown notification where the platform allows it. Nothing
// here does I/O. This package may import the leaf package internal/areas for
// the issuer table, and never imports internal/official.

package service

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"unicode"
	"unicode/utf8"

	"pinalert/internal/areas"
)

// officialIssuerMaxRunes is the longest issuer name on a lock screen body.
const officialIssuerMaxRunes = 40

const (
	officialIssuerIMD  = "IMD"
	officialIssuerCWC  = "CWC"
	officialSDMASuffix = " SDMA"
)

// OfficialPushInput is everything BuildOfficialPushText may read: five closed
// values and nothing else. It has no field for the feed's headline,
// description or instruction, and TestOfficialPushInputHasNoFeedTextField keeps
// it that way.
type OfficialPushInput struct {
	// Level is the alert's severity, "extreme" or "severe", in any case. Any
	// other value builds nothing.
	Level string
	// EventWord is one of OfficialEventWords or the fallback "Weather alert",
	// chosen by OfficialEventWord from the feed's event field. Any other value
	// builds nothing.
	EventWord string
	// Area is the words of the place: a district name from the boundary file
	// followed by " district" (for example "Krishna district"), or a state name
	// from the boundary file for a state level alert.
	Area string
	// AreaIsState is true for a state level alert. D-16 lets only Extreme alerts
	// reach a phone at state level, so a state level Severe input builds nothing.
	AreaIsState bool
	// Issuer is the authority, one of IMD, CWC or "<State> SDMA" from
	// OfficialIssuer. Any other value builds nothing.
	Issuer string
}

// BuildOfficialPushText builds the title and body of one official alert
// notification. ok is false for an input no notification exists for: a level
// other than extreme or severe, an event word outside the closed list, an empty
// or unsafe area, an issuer outside the closed table, a state level Severe
// alert (D-16), or words over the lock screen budgets.
//
// The title is "{Event} warning for {Area}", or "Weather alert for {Area}" for
// the fallback word, and the body is "{Level}. Issued by {Issuer}.". A long
// area is cut with AreaWords, so the title never passes PushTitleMaxRunes.
func BuildOfficialPushText(in OfficialPushInput) (PushText, bool) {
	level := strings.ToLower(strings.TrimSpace(in.Level))
	if level != "extreme" && level != "severe" {
		return PushText{}, false
	}
	if in.AreaIsState && level != "extreme" {
		return PushText{}, false
	}
	if !validOfficialEventWord(in.EventWord) {
		return PushText{}, false
	}
	if !validOfficialIssuer(in.Issuer) {
		return PushText{}, false
	}
	area := AreaWords(in.Area)
	if area == "" || !safeOfficialArea(area) {
		return PushText{}, false
	}

	var title string
	if in.EventWord == officialFallbackWord {
		title = in.EventWord + " for " + area
	} else {
		title = in.EventWord + " warning for " + area
	}
	body := OfficialLevelWord(level) + ". Issued by " + in.Issuer + "."
	if utf8.RuneCountInString(title) > PushTitleMaxRunes || utf8.RuneCountInString(body) > PushBodyMaxRunes {
		return PushText{}, false
	}
	return PushText{Title: title, Body: body}, true
}

// validOfficialEventWord reports whether w is exactly one of the closed event
// words or the fallback word.
func validOfficialEventWord(w string) bool {
	if w == officialFallbackWord {
		return true
	}
	for _, x := range OfficialEventWords {
		if x == w {
			return true
		}
	}
	return false
}

// safeOfficialArea refuses an area with a control character or a dash of the
// kinds the project never writes. The area comes from our boundary file, so
// this is a second guard, not the first.
func safeOfficialArea(area string) bool {
	for _, r := range area {
		if unicode.IsControl(r) || r == '–' || r == '—' {
			return false
		}
	}
	return true
}

// validOfficialIssuer reports whether s is IMD, CWC or "<State> SDMA": ASCII
// letters and single spaces before the word SDMA, at most 40 runes in all.
func validOfficialIssuer(s string) bool {
	if s == officialIssuerIMD || s == officialIssuerCWC {
		return true
	}
	if !strings.HasSuffix(s, officialSDMASuffix) || utf8.RuneCountInString(s) > officialIssuerMaxRunes {
		return false
	}
	name := strings.TrimSuffix(s, officialSDMASuffix)
	if name == "" || name[0] == ' ' || name[len(name)-1] == ' ' {
		return false
	}
	prevSpace := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c == ' ':
			if prevSpace {
				return false
			}
			prevSpace = true
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
			prevSpace = false
		default:
			return false
		}
	}
	return true
}

// OfficialIssuer maps the sender of an alert to the closed issuer table: "IMD"
// for IMD or IMD-<office>, "CWC" for CWC or CWC-<office>, "<State> SDMA" for a
// <State>-SDMA sender whose state the boundary index knows, and "" for anything
// else. An alert whose issuer is "" is not pushed. It lives here so the send
// worker, which is in this package, can use it too.
func OfficialIssuer(sender string, ix *areas.Index) string {
	s := strings.TrimSpace(sender)
	upper := strings.ToUpper(s)
	switch {
	case upper == officialIssuerIMD || strings.HasPrefix(upper, officialIssuerIMD+"-"):
		return officialIssuerIMD
	case upper == officialIssuerCWC || strings.HasPrefix(upper, officialIssuerCWC+"-"):
		return officialIssuerCWC
	}
	if ix == nil {
		return ""
	}
	st, ok := ix.StateForSender(s)
	if !ok {
		return ""
	}
	issuer := titleStateName(st.Name) + officialSDMASuffix
	if !validOfficialIssuer(issuer) {
		return ""
	}
	return issuer
}

// titleStateName turns a boundary file state name ("ANDAMAN & NICOBAR") into
// the words used on a lock screen ("Andaman and Nicobar"): each word starts with
// a capital, "&" reads as "and", a comma reads as a space, and "and" and "of"
// stay small after the first word.
func titleStateName(name string) string {
	name = strings.NewReplacer("&", " and ", ",", " ").Replace(name)
	words := strings.Fields(strings.ToLower(name))
	for i, w := range words {
		if i > 0 && (w == "and" || w == "of") {
			continue
		}
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// OfficialStateWords is the words of a state on a lock screen, the form
// BuildOfficialPushText takes for a state level alert.
func OfficialStateWords(boundaryName string) string {
	return titleStateName(boundaryName)
}

// OfficialTopic is the Web Push Topic of an alert: the first 16 bytes of
// sha256("official:" + guid) in base64url without padding, which is 22
// characters. It is stable for a guid, so a later version of the same alert
// replaces an undelivered message at a push service that honours Topic.
func OfficialTopic(guid string) string {
	sum := sha256.Sum256([]byte("official:" + guid))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

// OfficialTag is the notification tag of an alert: "official-" plus the topic.
// A later version under the same tag replaces the shown notification silently.
func OfficialTag(guid string) string {
	return "official-" + OfficialTopic(guid)
}
