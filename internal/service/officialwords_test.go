package service

import (
	"strings"
	"testing"
)

func TestOfficialEventWords(t *testing.T) {
	cases := []struct {
		event  string
		word   string
		phrase string
	}{
		{"Cyclone", "Cyclone", "Cyclone warning"},
		{"Severe Cyclonic Storm", "Cyclone", "Cyclone warning"},
		{"Very Heavy Rainfall", "Heavy rain", "Heavy rain warning"},
		{"Heavy Rain", "Heavy rain", "Heavy rain warning"},
		{"Thunderstorm & Lightning", "Thunderstorm", "Thunderstorm warning"},
		{"Lightning", "Lightning", "Lightning warning"},
		{"Flood", "Flood", "Flood warning"},
		{"Riverine flood", "Flood", "Flood warning"},
		{"Heat Wave", "Heat wave", "Heat wave warning"},
		{"Cold Wave", "Cold wave", "Cold wave warning"},
		{"Tsunami", "Tsunami", "Tsunami warning"},
		{"Earthquake", "Earthquake", "Earthquake warning"},
		{"Dust storm", "Weather alert", "Weather alert"},
		{"Moderate Rain", "Weather alert", "Weather alert"},
		{"", "Weather alert", "Weather alert"},
	}
	// No hyphen, en dash or em dash in anything the table builds.
	dashes := "-" + string(rune(0x2013)) + string(rune(0x2014))
	for _, c := range cases {
		if got := OfficialEventWord(c.event); got != c.word {
			t.Errorf("OfficialEventWord(%q) = %q, want %q", c.event, got, c.word)
		}
		got := OfficialEventPhrase(c.event)
		if got != c.phrase {
			t.Errorf("OfficialEventPhrase(%q) = %q, want %q", c.event, got, c.phrase)
		}
		if strings.ContainsAny(c.word+got, dashes) {
			t.Errorf("a dash in the output for %q: %q", c.event, got)
		}
	}
}

func TestOfficialEventWordsClosedList(t *testing.T) {
	want := []string{"Cyclone", "Tsunami", "Earthquake", "Flood", "Heavy rain", "Thunderstorm", "Lightning", "Heat wave", "Cold wave"}
	if len(OfficialEventWords) != len(want) {
		t.Fatalf("OfficialEventWords = %v, want %v", OfficialEventWords, want)
	}
	for i := range want {
		if OfficialEventWords[i] != want[i] {
			t.Errorf("OfficialEventWords[%d] = %q, want %q", i, OfficialEventWords[i], want[i])
		}
	}
}

func TestOfficialLevelWord(t *testing.T) {
	cases := map[string]string{
		"Extreme":      "Extreme",
		"severe":       "Severe",
		" MODERATE ":   "Moderate",
		"Minor":        "Minor",
		"Unknown":      "Unknown",
		"":             "Unknown",
		"Catastrophic": "Unknown",
	}
	for in, want := range cases {
		if got := OfficialLevelWord(in); got != want {
			t.Errorf("OfficialLevelWord(%q) = %q, want %q", in, got, want)
		}
	}
}
