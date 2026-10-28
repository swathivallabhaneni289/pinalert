package areas

import (
	"strings"
)

// stateAliases maps a spelling that turns up in feeds, and is not the file's own
// spelling, onto the normalised name of the state it means in the file. The file
// is the 2019 and later map, so merged and renamed territories need an entry.
var stateAliases = map[string]string{
	// The file says "ANDAMAN & NICOBAR"; feeds add "Islands".
	"andaman and nicobar islands": "andaman and nicobar",
	"andaman nicobar islands":     "andaman and nicobar",
	// The file has one merged territory; feeds still use the older names and
	// the long official name.
	"dadra and nagar haveli and daman and diu": "dadra nagar haveli daman and diu",
	"dadra and nagar haveli":                   "dadra nagar haveli daman and diu",
	"daman and diu":                            "dadra nagar haveli daman and diu",
	// Older or alternative official names.
	"orissa":       "odisha",
	"pondicherry":  "puducherry",
	"uttaranchal":  "uttarakhand",
	"nct of delhi": "delhi",
	"new delhi":    "delhi",
	// Common misspelling of Chhattisgarh in feeds.
	"chattisgarh": "chhattisgarh",
}

// normaliseStateName lower cases a name, reads "&" as "and", reads hyphens,
// commas and repeated spaces as one space and drops a leading "the ".
func normaliseStateName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, "&", " and ")
	s = strings.NewReplacer("-", " ", ",", " ").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimPrefix(s, "the ")
	return s
}

// buildStateNames fills the lookup used by StateByName.
func (ix *Index) buildStateNames() {
	ix.stateNorm = map[string]State{}
	for _, s := range ix.states {
		ix.stateNorm[normaliseStateName(s.Name)] = s
	}
	for alias, target := range stateAliases {
		if s, ok := ix.stateNorm[target]; ok {
			if _, exists := ix.stateNorm[alias]; !exists {
				ix.stateNorm[alias] = s
			}
		}
	}
}

// StateByName finds a state by name, in any case and with "and" or "&".
func (ix *Index) StateByName(name string) (State, bool) {
	key := normaliseStateName(name)
	if key == "" {
		return State{}, false
	}
	s, ok := ix.stateNorm[key]
	return s, ok
}

// StateForSender maps a sender of the form "<State>-SDMA" to its state. Any
// other sender, including IMD and CWC, is not a state authority and gives false.
func (ix *Index) StateForSender(sender string) (State, bool) {
	sender = strings.TrimSpace(sender)
	const suffix = "-sdma"
	if len(sender) <= len(suffix) || !strings.EqualFold(sender[len(sender)-len(suffix):], suffix) {
		return State{}, false
	}
	return ix.StateByName(sender[:len(sender)-len(suffix)])
}

// TrailingState reads a state from the end of an area description: the text
// after the last comma, or after the last " of " when that comes later. A
// description with neither is read whole.
func (ix *Index) TrailingState(areaDesc string) (State, bool) {
	desc := strings.TrimSpace(areaDesc)
	if desc == "" {
		return State{}, false
	}
	cut := strings.LastIndex(desc, ",") + 1 // 0 when there is no comma
	if lower := strings.ToLower(desc); len(lower) == len(desc) {
		if i := strings.LastIndex(lower, " of "); i >= 0 && i+4 > cut {
			cut = i + 4
		}
	} else if i := strings.LastIndex(desc, " of "); i >= 0 && i+4 > cut {
		cut = i + 4
	}
	tail := strings.Trim(strings.TrimSpace(desc[cut:]), " .;:)(")
	return ix.StateByName(tail)
}
