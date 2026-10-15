// This file holds the older numbers label (D-22). Task 2 of plan 04-12 extends
// it with the Phase 4 author facing moderation strings and the Disclaimer
// glossary.

package service

import "strings"

// OlderNumbersGenericLabel is the label of the older numbers row when the
// setting holds a number this file cannot pair with a service.
const OlderNumbersGenericLabel = "Older emergency numbers that many state governments still list."

// olderNumberServices pairs the three older numbers with their service. A
// number outside this map gets the generic label, so a service is never
// guessed for a number (D-22).
var olderNumberServices = map[string]string{
	"100": "police",
	"101": "fire",
	"108": "ambulance",
}

// OlderNumbersLabel returns the sentence that names the older numbers from the
// one numbers setting. Every number in it comes from the list passed in. An
// empty list gives an empty string, any number outside 100, 101 and 108 gives
// OlderNumbersGenericLabel.
func OlderNumbersLabel(older []string) string {
	if len(older) == 0 {
		return ""
	}
	parts := make([]string, 0, len(older))
	for _, n := range older {
		svc, ok := olderNumberServices[n]
		if !ok {
			return OlderNumbersGenericLabel
		}
		parts = append(parts, svc+" ("+n+")")
	}
	if len(parts) == 1 {
		return "An older number for " + parts[0] + "."
	}
	last := len(parts) - 1
	return "Older numbers for " + strings.Join(parts[:last], ", ") + " and " + parts[last] + "."
}
