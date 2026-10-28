// officialwords.go holds the closed word tables that turn an official alert
// into the few words Pinalert itself authors: the event phrase of the Nearby
// banner and the level word of a row. Feed text never becomes a phrase. It can
// be Telugu, very long or hostile, so the event field is only ever matched
// against the keyword list below and the result is one of the fixed words.
//
// The table lives in this package, not in the official alerts package, because
// the phone notification words of plan 05-11 must use the same table and this
// package never imports the official one. Nothing here does I/O.

package service

import "strings"

// officialFallbackWord is the word for an event that matches no keyword. It is
// also the whole phrase, with no "warning" after it.
const officialFallbackWord = "Weather alert"

// OfficialEventWords is the closed list of event words, in the order the
// keywords are tried.
var OfficialEventWords = []string{
	"Cyclone", "Tsunami", "Earthquake", "Flood", "Heavy rain",
	"Thunderstorm", "Lightning", "Heat wave", "Cold wave",
}

// officialEventKeywords pairs each word of OfficialEventWords with the lower
// case fragments of a CAP event that mean it. The order is the priority: a
// "Thunderstorm & Lightning" event is a Thunderstorm, and "Severe Cyclonic
// Storm" is a Cyclone because "cyclon" is the fragment, not "cyclone".
var officialEventKeywords = []struct {
	word      string
	fragments []string
}{
	{"Cyclone", []string{"cyclon"}},
	{"Tsunami", []string{"tsunami"}},
	{"Earthquake", []string{"earthquake"}},
	{"Flood", []string{"flood"}},
	{"Heavy rain", []string{"heavy rain"}},
	{"Thunderstorm", []string{"thunderstorm"}},
	{"Lightning", []string{"lightning"}},
	{"Heat wave", []string{"heat wave", "heatwave"}},
	{"Cold wave", []string{"cold wave", "coldwave"}},
}

// OfficialEventWord returns the closed word for a CAP event, matched
// case insensitively by keyword, or the fallback word when none matches.
func OfficialEventWord(capEvent string) string {
	e := strings.ToLower(capEvent)
	for _, k := range officialEventKeywords {
		for _, f := range k.fragments {
			if strings.Contains(e, f) {
				return k.word
			}
		}
	}
	return officialFallbackWord
}

// OfficialEventPhrase is the banner phrase: the event word followed by
// " warning", or the fallback word alone for an event that matches no keyword.
func OfficialEventPhrase(capEvent string) string {
	w := OfficialEventWord(capEvent)
	if w == officialFallbackWord {
		return w
	}
	return w + " warning"
}

// OfficialLevelWord maps a CAP severity onto the five level words, ignoring
// case and padding. Anything else is Unknown.
func OfficialLevelWord(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "extreme":
		return "Extreme"
	case "severe":
		return "Severe"
	case "moderate":
		return "Moderate"
	case "minor":
		return "Minor"
	}
	return "Unknown"
}
