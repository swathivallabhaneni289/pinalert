// pushcopy_test.go pins every word a phone lock screen can show (plan
// 03.3-03): the UI-SPEC Surface 3b samples as named table rows, the budgets,
// the plain punctuation rule and the no noise words rule.

package service

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// bannedNoiseWords is the owner's list of eleven words no notification copy
// may use (rule 2026-10-18: no buzzes). Later plans copy this same list.
var bannedNoiseWords = []string{
	"buzz", "buzzes", "vibrate", "vibration", "sound", "ring",
	"silent", "silently", "quiet", "quietly", "hear",
}

func pushF(f float64) *float64 { return &f }

func pushH(h Happening) *Happening { return &h }

func TestBuildPushText(t *testing.T) {
	cases := []struct {
		name      string
		in        PushTextInput
		wantTitle string
		wantBody  string
	}{
		{
			name:      "critical with pick, area, not confirmed",
			in:        PushTextInput{Kind: PushKindCritical, Happening: pushH(HappeningWaterRising), Category: CategoryFlood, Severity: SeverityCritical, AreaName: "Central Market", DistanceKm: pushF(2.0)},
			wantTitle: "Water rising near Central Market",
			wantBody:  "2 km from your home area. Not confirmed yet.",
		},
		{
			name:      "critical with pick, no area, under 1 km",
			in:        PushTextInput{Kind: PushKindCritical, Happening: pushH(HappeningFire), Category: CategoryFire, Severity: SeverityCritical, DistanceKm: pushF(0.4)},
			wantTitle: "Fire",
			wantBody:  "Under 1 km from your home area. Not confirmed yet.",
		},
		{
			name:      "critical without pick falls back to category and severity",
			in:        PushTextInput{Kind: PushKindCritical, Category: CategoryFlood, Severity: SeverityCritical, AreaName: "Central Market", DistanceKm: pushF(3.2)},
			wantTitle: "Flood, Critical near Central Market",
			wantBody:  "3 km from your home area. Not confirmed yet.",
		},
		{
			name:      "critical without pick and without area",
			in:        PushTextInput{Kind: PushKindCritical, Category: CategoryFlood, Severity: SeverityCritical, DistanceKm: pushF(3.2)},
			wantTitle: "Flood, Critical",
			wantBody:  "3 km from your home area. Not confirmed yet.",
		},
		{
			name:      "critical confirmed keeps the title and swaps the body",
			in:        PushTextInput{Kind: PushKindCritical, Happening: pushH(HappeningWaterRising), Category: CategoryFlood, Severity: SeverityCritical, AreaName: "Central Market", DistanceKm: pushF(2.0), Confirmed: true},
			wantTitle: "Water rising near Central Market",
			wantBody:  "Confirmed by people nearby. 2 km from your home area.",
		},
		{
			name:      "critical pick is the same whatever the category (a pick wins)",
			in:        PushTextInput{Kind: PushKindCritical, Happening: pushH(HappeningWaterRising), Category: CategoryRescueNeeded, Severity: SeverityCritical, AreaName: "Central Market", DistanceKm: pushF(2.0)},
			wantTitle: "Water rising near Central Market",
			wantBody:  "2 km from your home area. Not confirmed yet.",
		},
		{
			name:      "critical pick people trapped",
			in:        PushTextInput{Kind: PushKindCritical, Happening: pushH(HappeningPeopleTrapped), Category: CategoryFlood, Severity: SeverityCritical, AreaName: "Old Bridge", DistanceKm: pushF(1.0)},
			wantTitle: "People trapped near Old Bridge",
			wantBody:  "1 km from your home area. Not confirmed yet.",
		},
		{
			name:      "reliable not confirmed",
			in:        PushTextInput{Kind: PushKindReliable, Category: CategoryPowerOutage, Severity: SeverityMedium, AreaName: "Old Bridge", DistanceKm: pushF(4)},
			wantTitle: "Power outage near Old Bridge",
			wantBody:  "From a Reliable reporter. 4 km from your home area. Not confirmed yet.",
		},
		{
			name:      "reliable confirmed",
			in:        PushTextInput{Kind: PushKindReliable, Category: CategoryPowerOutage, Severity: SeverityMedium, AreaName: "Old Bridge", DistanceKm: pushF(4), Confirmed: true},
			wantTitle: "Power outage near Old Bridge",
			wantBody:  "Confirmed by people nearby. 4 km from your home area.",
		},
		{
			name:      "reliable without area",
			in:        PushTextInput{Kind: PushKindReliable, Category: CategoryShelterOpen, Severity: SeverityLow, DistanceKm: pushF(0.2)},
			wantTitle: "Shelter open",
			wantBody:  "From a Reliable reporter. Under 1 km from your home area. Not confirmed yet.",
		},
		{
			name:      "reliable ignores a pick (Critical only)",
			in:        PushTextInput{Kind: PushKindReliable, Happening: pushH(HappeningFire), Category: CategoryTraffic, Severity: SeverityLow, AreaName: "Old Bridge", DistanceKm: pushF(4)},
			wantTitle: "Traffic near Old Bridge",
			wantBody:  "From a Reliable reporter. 4 km from your home area. Not confirmed yet.",
		},
		{
			name:      "changed, own report reached live",
			in:        PushTextInput{Kind: PushKindChanged, Event: UpdateReachedLive, OwnReport: true, Category: CategoryFlood, Severity: SeverityCritical, AreaName: "Central Market", DistanceKm: pushF(2)},
			wantTitle: "Your report is now confirmed",
			wantBody:  "Flood, Critical. Central Market, 2 km from your home area.",
		},
		{
			name:      "changed, voted report resolved, no home area",
			in:        PushTextInput{Kind: PushKindChanged, Event: UpdateResolved, Category: CategoryTraffic, Severity: SeverityLow, AreaName: "Central Market"},
			wantTitle: "A report you voted on was marked resolved",
			wantBody:  "Traffic, Low. Central Market.",
		},
		{
			name:      "changed, hidden is now disputed",
			in:        PushTextInput{Kind: PushKindChanged, Event: UpdateHidden, OwnReport: true, Category: CategoryRoadBlocked, Severity: SeverityMedium, AreaName: "Ring Road", DistanceKm: pushF(7.4)},
			wantTitle: "Your report is now disputed",
			wantBody:  "Road blocked, Medium. Ring Road, 7 km from your home area.",
		},
		{
			name:      "changed, fading",
			in:        PushTextInput{Kind: PushKindChanged, Event: UpdateFading, Category: CategoryOther, Severity: SeverityLow, AreaName: "Old Bridge", DistanceKm: pushF(1.4)},
			wantTitle: "A report you voted on fades in about an hour",
			wantBody:  "Other, Low. Old Bridge, 1 km from your home area.",
		},
		{
			name:      "changed with a pick uses the pick in the body",
			in:        PushTextInput{Kind: PushKindChanged, Event: UpdateReachedLive, Happening: pushH(HappeningWaterRising), Category: CategoryFlood, Severity: SeverityCritical, AreaName: "Central Market", DistanceKm: pushF(2)},
			wantTitle: "A report you voted on is now confirmed",
			wantBody:  "Water rising. Central Market, 2 km from your home area.",
		},
		{
			name:      "changed with no area and no distance",
			in:        PushTextInput{Kind: PushKindChanged, Event: UpdateReachedLive, OwnReport: true, Category: CategoryFlood, Severity: SeverityCritical},
			wantTitle: "Your report is now confirmed",
			wantBody:  "Flood, Critical.",
		},
		{
			name:      "changed with distance only",
			in:        PushTextInput{Kind: PushKindChanged, Event: UpdateReachedLive, Category: CategoryFlood, Severity: SeverityCritical, DistanceKm: pushF(9.6)},
			wantTitle: "A report you voted on is now confirmed",
			wantBody:  "Flood, Critical. 10 km from your home area.",
		},
		{
			name:      "changed never says not confirmed yet, even when the flag is set",
			in:        PushTextInput{Kind: PushKindChanged, Event: UpdateHidden, Confirmed: false, Category: CategoryFlood, Severity: SeverityCritical, AreaName: "Central Market", DistanceKm: pushF(2)},
			wantTitle: "A report you voted on is now disputed",
			wantBody:  "Flood, Critical. Central Market, 2 km from your home area.",
		},
		{
			name:      "long area is cut in the title and the body",
			in:        PushTextInput{Kind: PushKindCritical, Happening: pushH(HappeningFire), Category: CategoryFire, Severity: SeverityCritical, AreaName: "Hospital Road Junction Nearby Station", DistanceKm: pushF(2)},
			wantTitle: "Fire near Hospital Road Junction…",
			wantBody:  "2 km from your home area. Not confirmed yet.",
		},
	}
	if len(cases) < 18 {
		t.Fatalf("only %d cases, want at least 18", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := BuildPushText(c.in)
			if !ok {
				t.Fatalf("BuildPushText(%+v) ok = false, want true", c.in)
			}
			if got.Title != c.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, c.wantTitle)
			}
			if got.Body != c.wantBody {
				t.Errorf("Body = %q, want %q", got.Body, c.wantBody)
			}
		})
	}
}

func TestBuildPushTextRefuses(t *testing.T) {
	cases := []struct {
		name string
		in   PushTextInput
	}{
		{"critical needs a home area", PushTextInput{Kind: PushKindCritical, Happening: pushH(HappeningFire), Category: CategoryFire, Severity: SeverityCritical}},
		{"reliable needs a home area", PushTextInput{Kind: PushKindReliable, Category: CategoryFlood, Severity: SeverityLow}},
		{"changed with more", PushTextInput{Kind: PushKindChanged, Event: UpdateMore, Category: CategoryFlood, Severity: SeverityLow}},
		{"changed with reliable_nearby", PushTextInput{Kind: PushKindChanged, Event: UpdateReliableNearby, Category: CategoryFlood, Severity: SeverityLow}},
		{"changed with critical_nearby", PushTextInput{Kind: PushKindChanged, Event: UpdateCriticalNearby, Category: CategoryFlood, Severity: SeverityLow}},
		{"changed with an unknown event", PushTextInput{Kind: PushKindChanged, Event: "comment", Category: CategoryFlood, Severity: SeverityLow}},
		{"changed with no event", PushTextInput{Kind: PushKindChanged, Category: CategoryFlood, Severity: SeverityLow}},
		{"confirmed update is built from the original kind", PushTextInput{Kind: PushKindConfirmedUpdate, Category: CategoryFlood, Severity: SeverityLow, DistanceKm: pushF(2)}},
		{"official is reserved", PushTextInput{Kind: PushKindOfficial, Category: CategoryFlood, Severity: SeverityLow, DistanceKm: pushF(2)}},
		{"follow is reserved", PushTextInput{Kind: PushKindFollow, Category: CategoryFlood, Severity: SeverityLow, DistanceKm: pushF(2)}},
		{"unknown kind", PushTextInput{Kind: "more", Category: CategoryFlood, Severity: SeverityLow, DistanceKm: pushF(2)}},
		{"empty kind", PushTextInput{Category: CategoryFlood, Severity: SeverityLow, DistanceKm: pushF(2)}},
		{"reliable with an unknown category", PushTextInput{Kind: PushKindReliable, Category: "mystery", Severity: SeverityLow, DistanceKm: pushF(2)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, ok := BuildPushText(c.in); ok {
				t.Errorf("BuildPushText(%+v) = %+v, true; want ok false", c.in, got)
			}
		})
	}
}

// TestBuildPushTextHasNoReporterTextField is the T-03.3-06 guard: the input
// type has no field that could carry the reporter's words, a name, a phone
// number, an address, a coordinate or an account.
func TestBuildPushTextHasNoReporterTextField(t *testing.T) {
	forbidden := []string{"description", "text", "name", "phone", "email", "address", "latitude", "longitude", "account"}
	typ := reflect.TypeOf(PushTextInput{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i).Name
		if field == "AreaName" {
			continue
		}
		lower := strings.ToLower(field)
		for _, bad := range forbidden {
			if strings.Contains(lower, bad) {
				t.Errorf("PushTextInput has field %q, which could carry %q", field, bad)
			}
		}
	}
	if typ.NumField() != 9 {
		t.Errorf("PushTextInput has %d fields, want the 9 closed inputs", typ.NumField())
	}
}

// TestCriticalKindTakesNoCategoryInput: the Critical kind keys off severity,
// never off a category, so with a pick the same words come out for every
// category including the retired rescue_needed (plan check warning 1).
func TestCriticalKindTakesNoCategoryInput(t *testing.T) {
	var first PushText
	for i, cat := range Categories {
		in := PushTextInput{Kind: PushKindCritical, Happening: pushH(HappeningWaterRising), Category: cat, Severity: SeverityCritical, AreaName: "Central Market", DistanceKm: pushF(2)}
		got, ok := BuildPushText(in)
		if !ok {
			t.Fatalf("category %q: ok = false", cat)
		}
		if i == 0 {
			first = got
		} else if got != first {
			t.Errorf("category %q changed the Critical words: %+v vs %+v", cat, got, first)
		}
	}
}

func TestPushTextBudgets(t *testing.T) {
	longArea := AreaWords(strings.Repeat("Mmmmmmmmm ", 6))
	if n := utf8.RuneCountInString(longArea); n > AreaMaxRunes {
		t.Fatalf("area is %d runes, want at most %d", n, AreaMaxRunes)
	}
	// A 28 rune area built to be exactly at the limit.
	exact := "Mmmmmmmmmmmmmmmmmmmmmmmmmmmm"
	if utf8.RuneCountInString(exact) != AreaMaxRunes {
		t.Fatalf("test setup: exact area is %d runes", utf8.RuneCountInString(exact))
	}
	areas := []string{"", longArea, AreaWords(exact), "Central Market"}
	maxTitle, maxBody := 0, 0
	check := func(in PushTextInput) {
		got, ok := BuildPushText(in)
		if !ok {
			return
		}
		if n := utf8.RuneCountInString(got.Title); n > PushTitleMaxRunes {
			t.Errorf("title %q is %d runes, budget %d", got.Title, n, PushTitleMaxRunes)
		}
		if n := utf8.RuneCountInString(got.Body); n > PushBodyMaxRunes {
			t.Errorf("body %q is %d runes, budget %d", got.Body, n, PushBodyMaxRunes)
		}
		maxTitle = max(maxTitle, utf8.RuneCountInString(got.Title))
		maxBody = max(maxBody, utf8.RuneCountInString(got.Body))
	}
	for _, area := range areas {
		for _, km := range []float64{0.2, 10, 99.6} {
			for _, conf := range []bool{false, true} {
				for _, h := range Happenings {
					for _, cat := range Categories {
						check(PushTextInput{Kind: PushKindCritical, Happening: pushH(h), Category: cat, Severity: SeverityCritical, AreaName: area, DistanceKm: pushF(km), Confirmed: conf})
					}
				}
				for _, cat := range Categories {
					check(PushTextInput{Kind: PushKindCritical, Category: cat, Severity: SeverityCritical, AreaName: area, DistanceKm: pushF(km), Confirmed: conf})
					check(PushTextInput{Kind: PushKindReliable, Category: cat, Severity: SeverityCritical, AreaName: area, DistanceKm: pushF(km), Confirmed: conf})
					for _, sev := range Severities {
						for _, ev := range []UpdateKind{UpdateReachedLive, UpdateHidden, UpdateResolved, UpdateFading} {
							for _, own := range []bool{false, true} {
								check(PushTextInput{Kind: PushKindChanged, Event: ev, OwnReport: own, Category: cat, Severity: sev, AreaName: area, DistanceKm: pushF(km)})
							}
						}
					}
				}
			}
		}
	}
	if maxTitle == 0 || maxBody == 0 {
		t.Fatal("no text was built, the budget check proved nothing")
	}
}

func TestDistanceWords(t *testing.T) {
	cases := []struct {
		km   float64
		want string
	}{
		{0, "Under 1 km"},
		{0.4, "Under 1 km"},
		{0.99, "Under 1 km"},
		{1.0, "1 km"},
		{1.49, "1 km"},
		{1.5, "2 km"},
		{2.0, "2 km"},
		{3.2, "3 km"},
		{9.6, "10 km"},
		{25, "25 km"},
	}
	for _, c := range cases {
		if got := DistanceWords(c.km); got != c.want {
			t.Errorf("DistanceWords(%v) = %q, want %q", c.km, got, c.want)
		}
	}
}

func TestAreaWords(t *testing.T) {
	ellipsis := "…"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"short unchanged", "Central Market", "Central Market"},
		{"empty stays empty", "", ""},
		{"only spaces", "   ", ""},
		{"surrounding spaces trimmed", "  Central Market  ", "Central Market"},
		{"exactly 28 runes unchanged", strings.Repeat("a", 28), strings.Repeat("a", 28)},
		{"cut at the last word end that leaves room for the ellipsis", "Hospital Road Junction Nearby Station", "Hospital Road Junction" + ellipsis},
		{"a word end at exactly 27 runes is kept", "Hospital Road Junction Near The Old Railway Station", "Hospital Road Junction Near" + ellipsis},
		{"no space is cut at 27 runes", strings.Repeat("b", 40), strings.Repeat("b", 27) + ellipsis},
		{"multi byte runes are counted as runes", strings.Repeat("అ", 40), strings.Repeat("అ", 27) + ellipsis},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AreaWords(c.in)
			if got != c.want {
				t.Errorf("AreaWords(%q) = %q, want %q", c.in, got, c.want)
			}
			if n := utf8.RuneCountInString(got); n > AreaMaxRunes {
				t.Errorf("AreaWords result is %d runes, want at most %d", n, AreaMaxRunes)
			}
		})
	}

	// A 40 rune name with spaces ends in the ellipsis, is at most 28 runes and
	// is cut at a word end (the character before the ellipsis is not a space).
	spaced := "alpha beta gamma delta epsilon zeta eta."
	got := AreaWords(spaced)
	if !strings.HasSuffix(got, ellipsis) || strings.HasSuffix(strings.TrimSuffix(got, ellipsis), " ") {
		t.Errorf("AreaWords(%q) = %q, want a word end then the ellipsis", spaced, got)
	}
}

func TestAlertExampleMatchesUISpec(t *testing.T) {
	got := AlertExample()
	if got.Title != "Water rising near Central Market" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Body != "2 km from your home area. Not confirmed yet." {
		t.Errorf("Body = %q", got.Body)
	}
	built, ok := BuildPushText(PushTextInput{Kind: PushKindCritical, Happening: pushH(HappeningWaterRising), Category: CategoryFlood, Severity: SeverityCritical, AreaName: "Central Market", DistanceKm: pushF(2)})
	if !ok || built != got {
		t.Errorf("AlertExample is not what BuildPushText builds from the sample: %+v vs %+v", got, built)
	}
}

func TestPushWordConstants(t *testing.T) {
	if NotConfirmedYetWords != "Not confirmed yet." {
		t.Errorf("NotConfirmedYetWords = %q", NotConfirmedYetWords)
	}
	if ConfirmedByPeopleWords != "Confirmed by people nearby." {
		t.Errorf("ConfirmedByPeopleWords = %q", ConfirmedByPeopleWords)
	}
	if FromReliableReporterWords != "From a Reliable reporter." {
		t.Errorf("FromReliableReporterWords = %q", FromReliableReporterWords)
	}
	if AreaMaxRunes != 28 || PushTitleMaxRunes < 50 || PushBodyMaxRunes != 120 {
		t.Errorf("budgets = %d, %d, %d", AreaMaxRunes, PushTitleMaxRunes, PushBodyMaxRunes)
	}
	words := map[Severity]string{SeverityLow: "Low", SeverityMedium: "Medium", SeverityCritical: "Critical"}
	for s, want := range words {
		if got := SeverityWord(s); got != want {
			t.Errorf("SeverityWord(%q) = %q, want %q", s, got, want)
		}
	}
	if SeverityWord("mystery") != "" {
		t.Error("an unknown severity must have no word")
	}
	picks := map[Happening]string{
		HappeningWaterRising:       "Water rising",
		HappeningPeopleTrapped:     "People trapped",
		HappeningFire:              "Fire",
		HappeningBuildingCollapsed: "Building collapsed",
		HappeningLandslide:         "Landslide",
		HappeningMedicalEmergency:  "Medical emergency",
		HappeningOther:             "Other",
	}
	if len(picks) != len(Happenings) {
		t.Fatalf("test table has %d picks, Happenings has %d", len(picks), len(Happenings))
	}
	for h, want := range picks {
		if got := HappeningWord(h); got != want {
			t.Errorf("HappeningWord(%q) = %q, want %q", h, got, want)
		}
	}
	if HappeningWord("mystery") != "" {
		t.Error("an unknown pick must have no word")
	}
}

// TestCategoryWordsCoverEveryCategory also checks the words are the ones the
// profile page shows. The handlers package cannot be imported from here (an
// import cycle), so the test reads its source and looks for each quoted word.
func TestCategoryWordsCoverEveryCategory(t *testing.T) {
	src, err := os.ReadFile("../api/handlers/auth.go")
	if err != nil {
		t.Fatalf("read auth.go: %v", err)
	}
	for _, c := range Categories {
		w := CategoryWord(c)
		if w == "" {
			t.Errorf("CategoryWord(%q) is empty", c)
			continue
		}
		if !strings.Contains(string(src), `"`+w+`"`) {
			t.Errorf("CategoryWord(%q) = %q is not a label in handlers/auth.go", c, w)
		}
	}
	if CategoryWord("mystery") != "" {
		t.Error("an unknown category must have no word")
	}
}

func TestPushCopyPlainPunctuation(t *testing.T) {
	enDash, emDash := string(rune(0x2013)), string(rune(0x2014))
	for _, file := range []string{"pushcopy.go", "pushkinds.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(src), enDash) || strings.Contains(string(src), emDash) {
			t.Errorf("%s holds an en dash or an em dash", file)
		}
	}
	for _, s := range allPushTexts() {
		if strings.Contains(s, enDash) || strings.Contains(s, emDash) {
			t.Errorf("notification text %q holds an en dash or an em dash", s)
		}
	}
}

func TestPushCopyHasNoNoiseWords(t *testing.T) {
	if len(bannedNoiseWords) != 11 {
		t.Fatalf("bannedNoiseWords has %d entries, want 11", len(bannedNoiseWords))
	}
	re := regexp.MustCompile(`(?i)\b(` + strings.Join(bannedNoiseWords, "|") + `)\b`)
	texts := allPushTexts()
	if len(texts) == 0 {
		t.Fatal("no notification text was built, the scan proved nothing")
	}
	for _, s := range texts {
		if m := re.FindString(s); m != "" {
			t.Errorf("notification text %q holds the noise word %q", s, m)
		}
	}
}

// allPushTexts builds every title and body over the full grid of kinds,
// picks, categories, severities, events, confirmed values and distances.
func allPushTexts() []string {
	var out []string
	add := func(in PushTextInput) {
		if got, ok := BuildPushText(in); ok {
			out = append(out, got.Title, got.Body)
		}
	}
	kinds := append(append([]PushKind{}, PushKinds...), ReservedPushKinds...)
	events := []UpdateKind{"", UpdateReachedLive, UpdateHidden, UpdateResolved, UpdateFading, UpdateReliableNearby, UpdateMore, UpdateCriticalNearby}
	picks := []*Happening{nil}
	for _, h := range Happenings {
		picks = append(picks, pushH(h))
	}
	dists := []*float64{nil, pushF(0.3), pushF(1), pushF(2.5), pushF(10)}
	for _, k := range kinds {
		for _, p := range picks {
			for _, c := range Categories {
				for _, s := range Severities {
					for _, e := range events {
						for _, d := range dists {
							for _, conf := range []bool{false, true} {
								for _, own := range []bool{false, true} {
									add(PushTextInput{Kind: k, Happening: p, Category: c, Severity: s, AreaName: "Central Market", DistanceKm: d, Confirmed: conf, Event: e, OwnReport: own})
								}
							}
						}
					}
				}
			}
		}
	}
	ex := AlertExample()
	out = append(out, ex.Title, ex.Body)
	return out
}
