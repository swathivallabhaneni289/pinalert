// This file holds the older numbers label (D-22), the Phase 4 author facing
// moderation strings (D-02, D-08) and the Disclaimer glossary built from the
// app's own trust sentences (D-23, UI-SPEC "How to read what Pinalert shows").
// A sentence that Phase 3 already wrote is read from trustcopy.go and never
// retyped here, so the legal page cannot drift from the app.

package service

import (
	"strconv"
	"strings"
)

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

// olderNumberNotes adds a short note inside the brackets of an older number.
// The national portal lists 102 as the National Ambulance Service, and 108 is
// the ambulance number in most states, so 108 reads "108, or 102 in some
// states" (plan 04-30, owner 2026-10-22: "yes add the 102 note").
var olderNumberNotes = map[string]string{
	"108": ", or 102 in some states",
}

// OlderNumbersLabel returns the sentence that names the older numbers from the
// one numbers setting. Every number in it comes from the list passed in, and
// the only other number it may name is 102, in the note olderNumberNotes keeps
// for 108. An empty list gives an empty string, any number outside 100, 101
// and 108 gives OlderNumbersGenericLabel.
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
		parts = append(parts, svc+" ("+n+olderNumberNotes[n]+")")
	}
	if len(parts) == 1 {
		return "An older number for " + parts[0] + "."
	}
	last := len(parts) - 1
	return "Older numbers for " + strings.Join(parts[:last], ", ") + " and " + parts[last] + "."
}

// The Activity page strings for a report that is held or removed (D-02, D-08)
// and the one new glossary entry. Plan 04-16 renders the Activity ones.
const (
	// HeldLabel names a report waiting for the text check.
	HeldLabel = "Waiting for a text check"
	// HeldSentence explains it to the author.
	HeldSentence = "It isn't shown in any feed until a text check finishes. Reload the page later to see where it stands."
	// RemovedLabel names a report a moderator removed.
	RemovedLabel = "Removed after review"
	// RemovedSentence explains it to the author.
	RemovedSentence = "This report was removed after review, so it no longer shows to anyone."
	// RemovedPolicyPointer ends a removed row only when a contact address is set.
	RemovedPolicyPointer = "If you have a question, see the Policy page."
	// DisclaimerResolvedEntry is the Resolved glossary entry. Phase 3 has no
	// Resolved sentence, so this is the one place it is written.
	DisclaimerResolvedEntry = "The situation in this report was marked as over, so it no longer counts as an active report."
)

// The glossary sentences that are new in Phase 4. Each is a lead that sits
// beside a Phase 3 sentence or stands alone.
const (
	GlossaryUnconfirmedLead = "Not enough different places nearby have confirmed this report yet."
	GlossaryCountsTerm      = "N confirmed and M disputed"
	GlossaryCountsLead      = "These count the different places nearby that confirmed or disputed the report."
	GlossaryCurrencyLead    = "These show whether people nearby have confirmed the report recently, not only when it was first posted."
	GlossaryDisputedText    = "People nearby have disputed this report, so it may be wrong. Check it before you rely on it."
)

// GlossaryEntry is one entry of the Disclaimer glossary: the terms that share
// one explanation, and that explanation.
type GlossaryEntry struct {
	Terms []string
	Text  string
}

// DisclaimerGlossary returns the nine glossary entries in the order the
// Disclaimer shows them. A fresh slice is built on every call.
func DisclaimerGlossary() []GlossaryEntry {
	return []GlossaryEntry{
		{Terms: []string{"Unconfirmed"}, Text: GlossaryUnconfirmedLead + " " + provisionalLeadSentence()},
		{Terms: []string{GlossaryCountsTerm}, Text: GlossaryCountsLead},
		{
			Terms: []string{currencyLabelUpToDate, currencyLabelGettingOld, currencyLabelNeedsReconfirming},
			Text:  GlossaryCurrencyLead + " " + needsReconfirmingSentence,
		},
		{Terms: []string{currencyLabelTooEarly}, Text: tooEarlySentence()},
		{
			Terms: []string{
				reporterLabel(ReporterReliable),
				reporterLabel(ReporterMixed),
				reporterLabel(ReporterUnreliable),
				reporterLabel(ReporterNew),
			},
			Text: GlossaryReporterText(),
		},
		{Terms: []string{"Disputed"}, Text: GlossaryDisputedText},
		{Terms: []string{"Resolved"}, Text: DisclaimerResolvedEntry},
		{Terms: []string{"Expired"}, Text: expiredLeadSentence},
		{Terms: []string{HeldLabel, RemovedLabel}, Text: HeldSentence + " " + RemovedSentence},
	}
}

// GlossaryReporterText explains the four reporter labels. It names the judged
// minimum from ReliabilityMinJudged, so the figure follows the constant.
func GlossaryReporterText() string {
	return "These describe the person who posted the report, from how often people nearby confirmed their earlier reports. Someone is a " +
		reporterLabel(ReporterNew) + " until " + strconv.Itoa(ReliabilityMinJudged) +
		" of their reports have had a clear result."
}
