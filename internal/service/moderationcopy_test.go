package service_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"pinalert/internal/service"
)

func TestServerCopyModeration(t *testing.T) {
	exact := []struct {
		name, got, want string
	}{
		{"HeldLabel", service.HeldLabel, "Waiting for a text check"},
		{"HeldSentence", service.HeldSentence, "It isn't shown in any feed until a text check finishes. Reload the page later to see where it stands."},
		{"RemovedLabel", service.RemovedLabel, "Removed after review"},
		{"RemovedSentence", service.RemovedSentence, "This report was removed after review, so it no longer shows to anyone."},
		{"RemovedPolicyPointer", service.RemovedPolicyPointer, "If you have a question, see the Policy page."},
		{"DisclaimerResolvedEntry", service.DisclaimerResolvedEntry, "The situation in this report was marked as over, so it no longer counts as an active report."},
	}
	for _, tc := range exact {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	all := []string{
		service.HeldLabel, service.HeldSentence, service.RemovedLabel, service.RemovedSentence,
		service.RemovedPolicyPointer, service.DisclaimerResolvedEntry, service.GlossaryUnconfirmedLead,
		service.GlossaryCountsTerm, service.GlossaryCountsLead, service.GlossaryCurrencyLead,
		service.GlossaryDisputedText, service.OlderNumbersGenericLabel, service.GlossaryReporterText(),
		service.OlderNumbersLabel([]string{"100", "101", "108"}),
		service.OlderNumbersLabel([]string{"100", "108"}),
	}
	for _, s := range all {
		assertServerCopy(t, s)
	}
}

// glossaryTermOrder is the flattened term order the Disclaimer shows.
var glossaryTermOrder = []string{
	"Unconfirmed", "N confirmed and M disputed",
	"Report up to date", "Report getting old", "Report needs re-confirming",
	"Too early to tell",
	"Reliable reporter", "Mixed record", "Unreliable reporter", "New reporter",
	"Disputed", "Resolved", "Expired",
	"Waiting for a text check", "Removed after review",
}

func TestDisclaimerGlossaryShape(t *testing.T) {
	g := service.DisclaimerGlossary()
	if len(g) != 9 {
		t.Fatalf("entries = %d, want 9", len(g))
	}
	var flat []string
	for _, e := range g {
		flat = append(flat, e.Terms...)
	}
	if strings.Join(flat, "|") != strings.Join(glossaryTermOrder, "|") {
		t.Errorf("terms = %q, want %q", flat, glossaryTermOrder)
	}
	counts := []int{1, 1, 3, 1, 4, 1, 1, 1, 2}
	for i, e := range g {
		if len(e.Terms) != counts[i] {
			t.Errorf("entry %d has %d terms, want %d", i+1, len(e.Terms), counts[i])
		}
	}
	threshold := "Needs " + strconv.Itoa(service.IndependentAgreementThreshold) + " people at different spots to confirm it."
	if !strings.HasSuffix(g[0].Text, threshold) {
		t.Errorf("entry 1 text %q must end with %q", g[0].Text, threshold)
	}
	if g[1].Text != service.GlossaryCountsLead {
		t.Errorf("entry 2 text = %q, want the counts lead alone", g[1].Text)
	}
	if g[7].Text != "Expired, so it can no longer be confirmed." {
		t.Errorf("entry 8 text = %q", g[7].Text)
	}
	if g[6].Text != service.DisclaimerResolvedEntry {
		t.Errorf("entry 7 text = %q, want DisclaimerResolvedEntry", g[6].Text)
	}
	if want := service.HeldSentence + " " + service.RemovedSentence; g[8].Text != want {
		t.Errorf("entry 9 text = %q, want %q", g[8].Text, want)
	}
	if g[4].Text != service.GlossaryReporterText() {
		t.Errorf("entry 5 text = %q, want GlossaryReporterText", g[4].Text)
	}
	if !strings.HasPrefix(g[2].Text, service.GlossaryCurrencyLead+" ") {
		t.Errorf("entry 3 text = %q", g[2].Text)
	}
	if g[0].Terms == nil || len(service.DisclaimerGlossary()) != 9 {
		t.Errorf("DisclaimerGlossary must return the entries on every call")
	}
}

func TestGlossaryReporterTextNamesTheMinimum(t *testing.T) {
	text := service.GlossaryReporterText()
	if !strings.Contains(text, strconv.Itoa(service.ReliabilityMinJudged)) || !strings.Contains(text, "New reporter") {
		t.Errorf("GlossaryReporterText = %q", text)
	}
}

func TestDisclaimerGlossaryReadsTrustCopy(t *testing.T) {
	raw, err := os.ReadFile("moderationcopy.go")
	if err != nil {
		t.Fatalf("reading moderationcopy.go: %v", err)
	}
	var code []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		code = append(code, line)
	}
	src := strings.Join(code, "\n")
	for _, id := range []string{
		"provisionalLeadSentence()", "currencyLabelUpToDate", "currencyLabelGettingOld",
		"currencyLabelNeedsReconfirming", "currencyLabelTooEarly", "needsReconfirmingSentence",
		"tooEarlySentence()", "reporterLabel(", "expiredLeadSentence", "ReliabilityMinJudged",
	} {
		if !strings.Contains(src, id) {
			t.Errorf("moderationcopy.go code must use %s", id)
		}
	}
	for _, frag := range []string{
		"Counts places", "to confirm it", "Gone quiet", "have confirmed this yet", "no longer be confirmed",
		"Report up to date", "Report getting old", "Report needs re-confirming", "Too early to tell",
		"Reliable reporter", "Mixed record", "Unreliable reporter", "New reporter",
	} {
		if strings.Contains(src, frag) {
			t.Errorf("moderationcopy.go code retypes the Phase 3 text %q", frag)
		}
	}
}

func TestOlderNumbersLabel(t *testing.T) {
	cases := []struct {
		name  string
		older []string
		want  string
	}{
		{"default list", []string{"100", "101", "108"}, "Older numbers for police (100), fire (101) and ambulance (108)."},
		{"police and ambulance", []string{"100", "108"}, "Older numbers for police (100) and ambulance (108)."},
		{"police alone", []string{"100"}, "An older number for police (100)."},
		{"unmapped number", []string{"100", "555"}, service.OlderNumbersGenericLabel},
		{"only unmapped", []string{"555"}, service.OlderNumbersGenericLabel},
		{"nil", nil, ""},
		{"empty", []string{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.OlderNumbersLabel(tc.older); got != tc.want {
				t.Errorf("OlderNumbersLabel(%v) = %q, want %q", tc.older, got, tc.want)
			}
		})
	}
	if service.OlderNumbersGenericLabel != "Older emergency numbers that many state governments still list." {
		t.Errorf("OlderNumbersGenericLabel = %q", service.OlderNumbersGenericLabel)
	}
}
