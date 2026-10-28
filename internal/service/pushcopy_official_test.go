// pushcopy_official_test.go pins every word an official alert puts on a lock
// screen (plan 05-11, D-11 and UI-SPEC 6c): the closed inputs, the title and
// body, the issuer table and the stable Topic and Tag.

package service

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"pinalert/internal/areas"
	"pinalert/internal/push"
)

func TestBuildOfficialPushText(t *testing.T) {
	cases := []struct {
		name  string
		in    OfficialPushInput
		title string
		body  string
	}{
		{"district extreme from IMD",
			OfficialPushInput{Level: "extreme", EventWord: "Cyclone", Area: "Krishna district", Issuer: "IMD"},
			"Cyclone warning for Krishna district", "Extreme. Issued by IMD."},
		{"district severe from CWC",
			OfficialPushInput{Level: "severe", EventWord: "Flood", Area: "Krishna district", Issuer: "CWC"},
			"Flood warning for Krishna district", "Severe. Issued by CWC."},
		{"state level extreme",
			OfficialPushInput{Level: "extreme", EventWord: "Flood", Area: "Andhra Pradesh", AreaIsState: true, Issuer: "Andhra Pradesh SDMA"},
			"Flood warning for Andhra Pradesh", "Extreme. Issued by Andhra Pradesh SDMA."},
		{"the fallback word has no warning after it",
			OfficialPushInput{Level: "severe", EventWord: "Weather alert", Area: "Krishna district", Issuer: "IMD"},
			"Weather alert for Krishna district", "Severe. Issued by IMD."},
		{"a level written with capitals is read the same",
			OfficialPushInput{Level: "Extreme", EventWord: "Heavy rain", Area: "Chennai district", Issuer: "IMD"},
			"Heavy rain warning for Chennai district", "Extreme. Issued by IMD."},
		{"every event word is allowed",
			OfficialPushInput{Level: "severe", EventWord: "Thunderstorm", Area: "Pune district", Issuer: "Maharashtra SDMA"},
			"Thunderstorm warning for Pune district", "Severe. Issued by Maharashtra SDMA."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := BuildOfficialPushText(c.in)
			if !ok {
				t.Fatalf("BuildOfficialPushText(%+v) ok = false", c.in)
			}
			if got.Title != c.title || got.Body != c.body {
				t.Errorf("got %q / %q, want %q / %q", got.Title, got.Body, c.title, c.body)
			}
		})
	}

	// A long area is cut with AreaWords and the title stays within the budget,
	// for the longest event phrase too.
	long := OfficialPushInput{Level: "extreme", EventWord: "Thunderstorm",
		Area: "Sri Potti Sriramulu Nellore Very Long Name district", Issuer: "Andaman and Nicobar SDMA"}
	got, ok := BuildOfficialPushText(long)
	if !ok {
		t.Fatal("a long area must still build")
	}
	if n := utf8.RuneCountInString(got.Title); n > PushTitleMaxRunes {
		t.Errorf("title is %d runes, over PushTitleMaxRunes %d: %q", n, PushTitleMaxRunes, got.Title)
	}
	if utf8.RuneCountInString(got.Body) > PushBodyMaxRunes {
		t.Errorf("body is over PushBodyMaxRunes: %q", got.Body)
	}
	if want := "Thunderstorm warning for " + AreaWords(long.Area); got.Title != want {
		t.Errorf("long title = %q, want the area cut by AreaWords: %q", got.Title, want)
	}
}

func TestBuildOfficialPushTextRefuses(t *testing.T) {
	good := OfficialPushInput{Level: "extreme", EventWord: "Cyclone", Area: "Krishna district", Issuer: "IMD"}
	if _, ok := BuildOfficialPushText(good); !ok {
		t.Fatal("the base input must build")
	}
	cases := []struct {
		name string
		edit func(in *OfficialPushInput)
	}{
		{"a moderate level", func(in *OfficialPushInput) { in.Level = "moderate" }},
		{"a minor level", func(in *OfficialPushInput) { in.Level = "minor" }},
		{"an unknown level", func(in *OfficialPushInput) { in.Level = "unknown" }},
		{"an empty level", func(in *OfficialPushInput) { in.Level = "" }},
		{"a made up level", func(in *OfficialPushInput) { in.Level = "catastrophic" }},
		{"an event word outside the closed list", func(in *OfficialPushInput) { in.EventWord = "Meteor strike" }},
		{"an event word from the feed", func(in *OfficialPushInput) { in.EventWord = "Severe Cyclonic Storm" }},
		{"an empty event word", func(in *OfficialPushInput) { in.EventWord = "" }},
		{"an event word in another case", func(in *OfficialPushInput) { in.EventWord = "cyclone" }},
		{"an empty area", func(in *OfficialPushInput) { in.Area = "" }},
		{"an area of spaces", func(in *OfficialPushInput) { in.Area = "   " }},
		{"an area with a line break", func(in *OfficialPushInput) { in.Area = "Krishna\ndistrict" }},
		{"an area with an em dash", func(in *OfficialPushInput) { in.Area = "Krishna — district" }},
		{"an empty issuer", func(in *OfficialPushInput) { in.Issuer = "" }},
		{"an issuer that is not in the table", func(in *OfficialPushInput) { in.Issuer = "NDMA" }},
		{"an issuer with a digit", func(in *OfficialPushInput) { in.Issuer = "Kerala2 SDMA" }},
		{"an issuer with punctuation", func(in *OfficialPushInput) { in.Issuer = "Kerala-SDMA" }},
		{"an issuer with no state", func(in *OfficialPushInput) { in.Issuer = " SDMA" }},
		{"an issuer over 40 runes", func(in *OfficialPushInput) {
			in.Issuer = strings.Repeat("A", 40) + " SDMA"
		}},
		{"an issuer with a lower case authority", func(in *OfficialPushInput) { in.Issuer = "imd" }},
		{"a state level Severe alert (D-16)", func(in *OfficialPushInput) {
			in.Level, in.AreaIsState, in.Area = "severe", true, "Andhra Pradesh"
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := good
			c.edit(&in)
			if got, ok := BuildOfficialPushText(in); ok {
				t.Errorf("BuildOfficialPushText(%+v) = %+v, true; want ok false", in, got)
			}
		})
	}

	// The longest legal issuer still builds.
	long := good
	long.Issuer = strings.Repeat("A", 35) + " SDMA"
	if _, ok := BuildOfficialPushText(long); !ok {
		t.Error("a 40 rune issuer must build")
	}
}

// TestOfficialPushInputHasNoFeedTextField keeps the input closed: exactly the
// five fields, none of which can carry the feed's headline, description or
// instruction (T-05-57).
func TestOfficialPushInputHasNoFeedTextField(t *testing.T) {
	typ := reflect.TypeOf(OfficialPushInput{})
	want := []string{"Level", "EventWord", "Area", "AreaIsState", "Issuer"}
	if typ.NumField() != len(want) {
		t.Fatalf("OfficialPushInput has %d fields, want exactly %v", typ.NumField(), want)
	}
	for i, name := range want {
		if typ.Field(i).Name != name {
			t.Errorf("field %d is %q, want %q", i, typ.Field(i).Name, name)
		}
	}
}

var officialNoiseRE = regexp.MustCompile(`(?i)\b(` + strings.Join(bannedNoiseWords, "|") + `)\b`)

// officialPushTexts builds every title and body over the closed grid: every
// event word, both levels, both area kinds and every issuer form, the SDMA of
// every state included.
func officialPushTexts(t *testing.T) []string {
	t.Helper()
	ix := areas.Default()
	issuers := []string{"IMD", "CWC"}
	for _, s := range ix.States() {
		iss := OfficialIssuer(titleStateName(s.Name)+"-SDMA", ix)
		if iss == "" {
			t.Errorf("state %q has no issuer form", s.Name)
			continue
		}
		issuers = append(issuers, iss)
	}
	words := append(append([]string{}, OfficialEventWords...), officialFallbackWord)
	areaNames := []string{"Krishna district", "Sri Potti Sriramulu Nellore Very Long Name district"}
	var out []string
	for _, w := range words {
		for _, level := range []string{"extreme", "severe"} {
			for _, iss := range issuers {
				for _, a := range areaNames {
					if got, ok := BuildOfficialPushText(OfficialPushInput{Level: level, EventWord: w, Area: a, Issuer: iss}); ok {
						out = append(out, got.Title, got.Body)
					} else {
						t.Errorf("district input %s/%s/%s/%s did not build", w, level, iss, a)
					}
				}
				if got, ok := BuildOfficialPushText(OfficialPushInput{Level: "extreme", EventWord: w, Area: "Andhra Pradesh", AreaIsState: true, Issuer: iss}); ok {
					out = append(out, got.Title, got.Body)
				} else {
					t.Errorf("state input %s/%s did not build", w, iss)
				}
			}
		}
	}
	return out
}

func TestOfficialPushCopyHasNoNoiseWords(t *testing.T) {
	texts := officialPushTexts(t)
	if len(texts) == 0 {
		t.Fatal("no official notification text was built, the scan proved nothing")
	}
	for _, s := range texts {
		if m := officialNoiseRE.FindString(s); m != "" {
			t.Errorf("official notification text %q holds the noise word %q", s, m)
		}
		lower := strings.ToLower(s)
		for _, bad := range []string{"buzz", "sound", "vibrat"} {
			if strings.Contains(lower, bad) {
				t.Errorf("official notification text %q mentions %q", s, bad)
			}
		}
		if strings.ContainsAny(s, "–—") {
			t.Errorf("official notification text %q holds a dash", s)
		}
		if strings.ContainsAny(s, "\r\n\t") {
			t.Errorf("official notification text %q holds a control character", s)
		}
	}
}

// TestOfficialPushTextBudgets: every text in the grid fits the lock screen
// budgets that Phase 3.3 set.
func TestOfficialPushTextBudgets(t *testing.T) {
	texts := officialPushTexts(t)
	for i := 0; i+1 < len(texts); i += 2 {
		if n := utf8.RuneCountInString(texts[i]); n > PushTitleMaxRunes {
			t.Errorf("title %q is %d runes, over %d", texts[i], n, PushTitleMaxRunes)
		}
		if n := utf8.RuneCountInString(texts[i+1]); n > PushBodyMaxRunes {
			t.Errorf("body %q is %d runes, over %d", texts[i+1], n, PushBodyMaxRunes)
		}
	}
}

func TestOfficialIssuer(t *testing.T) {
	ix := areas.Default()
	cases := []struct {
		sender string
		want   string
	}{
		{"IMD-Chennai", "IMD"},
		{"IMD", "IMD"},
		{"imd-delhi", "IMD"},
		{"CWC", "CWC"},
		{"CWC-Lucknow", "CWC"},
		{"Andhra-Pradesh-SDMA", "Andhra Pradesh SDMA"},
		{"Tamil-Nadu-SDMA", "Tamil Nadu SDMA"},
		{"Kerala-SDMA", "Kerala SDMA"},
		{"  Kerala-SDMA  ", "Kerala SDMA"},
		{"NDMA", ""},
		{"IMDX", ""},
		{"CWCX-Lucknow", ""},
		{"Nowhere-SDMA", ""},
		{"-SDMA", ""},
		{"", ""},
		{"<script>-SDMA", ""},
	}
	for _, c := range cases {
		if got := OfficialIssuer(c.sender, ix); got != c.want {
			t.Errorf("OfficialIssuer(%q) = %q, want %q", c.sender, got, c.want)
		}
	}
	if got := OfficialIssuer("Kerala-SDMA", nil); got != "" {
		t.Errorf("with no boundary index a state authority gives %q, want empty", got)
	}
	if got := OfficialIssuer("IMD-Chennai", nil); got != "IMD" {
		t.Errorf("IMD needs no boundary index, got %q", got)
	}

	for in, want := range map[string]string{
		"ANDAMAN & NICOBAR":              "Andaman and Nicobar",
		"DADRA,NAGAR HAVELI,DAMAN & DIU": "Dadra Nagar Haveli Daman and Diu",
		"JAMMU & KASHMIR":                "Jammu and Kashmir",
		"ANDHRA PRADESH":                 "Andhra Pradesh",
	} {
		if got := OfficialStateWords(in); got != want {
			t.Errorf("OfficialStateWords(%q) = %q, want %q", in, got, want)
		}
	}

	// Every state's authority passes the closed issuer check, so no real
	// sender is dropped because of an odd character in a state name.
	for _, s := range ix.States() {
		sender := strings.ReplaceAll(titleStateName(s.Name), " ", "-") + "-SDMA"
		iss := OfficialIssuer(sender, ix)
		if iss == "" {
			t.Errorf("sender %q (state %q) gives no issuer", sender, s.Name)
			continue
		}
		if !validOfficialIssuer(iss) {
			t.Errorf("issuer %q for state %q fails validOfficialIssuer", iss, s.Name)
		}
		if _, ok := BuildOfficialPushText(OfficialPushInput{Level: "extreme", EventWord: "Flood", Area: "X district", Issuer: iss}); !ok {
			t.Errorf("issuer %q does not build", iss)
		}
	}
}

func TestOfficialTopic(t *testing.T) {
	a := OfficialTopic("1792919196941008")
	if len(a) != 22 {
		t.Errorf("topic %q is %d characters, want 22", a, len(a))
	}
	if !push.ValidTopic(a) {
		t.Errorf("topic %q fails push.ValidTopic", a)
	}
	if a != OfficialTopic("1792919196941008") {
		t.Error("the topic is not stable across calls")
	}
	b := OfficialTopic("1792919196941009")
	if a == b {
		t.Error("two guids share a topic")
	}
	if !push.ValidTopic(b) {
		t.Errorf("topic %q fails push.ValidTopic", b)
	}

	tag := OfficialTag("1792919196941008")
	if tag != "official-"+a {
		t.Errorf("tag = %q, want official- plus the topic", tag)
	}
	// The payload's tag rule: a non empty string the service worker passes to
	// showNotification unchanged. Keep it to the base64url alphabet and short.
	if tag == "" || len(tag) > 64 {
		t.Errorf("tag %q is empty or over 64 characters", tag)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(tag) {
		t.Errorf("tag %q holds a character outside the base64url alphabet", tag)
	}
	if tag == OfficialTag("1792919196941009") {
		t.Error("two guids share a tag")
	}
}
