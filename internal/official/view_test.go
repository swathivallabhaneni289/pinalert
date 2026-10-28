package official

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

var viewNow = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

func viewStatus(ago time.Duration) FeedStatus {
	last := viewNow.Add(-ago)
	return FeedStatus{Feed: FeedSachet, LastSuccessAt: &last}
}

func readerArea() *Area {
	return &Area{DistrictCodes: []int{749}, DistrictName: "Ntr", StateCode: 28, StateName: "Andhra Pradesh"}
}

func homeAreaGuntur() *Area {
	return &Area{DistrictCodes: []int{506}, DistrictName: "Guntur", StateCode: 28, StateName: "Andhra Pradesh"}
}

// testAlert is a Moderate district alert on district 749 unless mut changes it.
func testAlert(id int64, mut func(*StoredAlert)) StoredAlert {
	a := StoredAlert{
		ID:            id,
		Sender:        "IMD-Chennai",
		Issuer:        "IMD Chennai",
		Event:         "Heavy Rain",
		Severity:      "Moderate",
		SentAt:        viewNow.Add(-time.Hour),
		ExpiresAt:     viewNow.Add(3 * time.Hour),
		HeadlineEN:    fmt.Sprintf("Heavy rain expected %d", id),
		AreaDesc:      "Ntr",
		Tier:          TierDistrict,
		DistrictCodes: []int{749},
		DistrictNames: []string{"Ntr"},
		StateCodes:    []int{28},
	}
	if mut != nil {
		mut(&a)
	}
	return a
}

func stateAlert(id int64, mut func(*StoredAlert)) StoredAlert {
	return testAlert(id, func(a *StoredAlert) {
		a.Tier = TierState
		a.DistrictCodes, a.DistrictNames = nil, nil
		a.Sender = "APSDMA"
		a.Issuer = "Andhra Pradesh SDMA"
		if mut != nil {
			mut(a)
		}
	})
}

func unplacedAlert(id int64, mut func(*StoredAlert)) StoredAlert {
	return testAlert(id, func(a *StoredAlert) {
		a.Tier = TierUnplaced
		a.DistrictCodes, a.DistrictNames, a.StateCodes = nil, nil, nil
		if mut != nil {
			mut(a)
		}
	})
}

func withSeverity(s string) func(*StoredAlert) {
	return func(a *StoredAlert) { a.Severity = s }
}

func groupKinds(v View) []string {
	var out []string
	for _, g := range v.Groups {
		out = append(out, g.Kind)
	}
	return out
}

func rowIDs(g Group) []int64 {
	var out []int64
	for _, r := range g.Rows {
		out = append(out, r.ID)
	}
	return out
}

func TestBuildViewGroupsAndOrder(t *testing.T) {
	in := ViewInput{
		Now:    viewNow,
		Status: viewStatus(4 * time.Minute),
		Reader: readerArea(),
		Alerts: []StoredAlert{
			testAlert(1, func(a *StoredAlert) { a.Severity = "Moderate"; a.SentAt = viewNow.Add(-10 * time.Minute) }),
			testAlert(2, func(a *StoredAlert) {
				a.Severity = "Extreme"
				a.SentAt = viewNow.Add(-5 * time.Hour)
				a.ExpiresAt = viewNow.Add(time.Hour)
			}),
			testAlert(3, withSeverity("Severe")),
			stateAlert(4, withSeverity("Severe")),
			unplacedAlert(5, nil),
		},
	}
	v := BuildView(in)
	if v.State != StateFresh || !v.AreaKnown {
		t.Fatalf("state %q areaKnown %v", v.State, v.AreaKnown)
	}
	if got := strings.Join(groupKinds(v), ","); got != "reader,state,unplaced" {
		t.Fatalf("groups = %s", got)
	}
	wantLabels := []string{"Where you are: Ntr district", "Across Andhra Pradesh", "Could not be placed on a map"}
	for i, g := range v.Groups {
		if g.Label != wantLabels[i] {
			t.Errorf("group %d label = %q, want %q", i, g.Label, wantLabels[i])
		}
	}
	if got := rowIDs(v.Groups[0]); fmt.Sprint(got) != "[2 3 1]" {
		t.Errorf("reader rows = %v, want Extreme, Severe, Moderate", got)
	}
	if v.Count != 5 {
		t.Errorf("Count = %d, want 5", v.Count)
	}
	if v.DistrictsLine != "Ntr district" {
		t.Errorf("DistrictsLine = %q", v.DistrictsLine)
	}
	if v.AgeSeconds == nil || *v.AgeSeconds != 240 {
		t.Errorf("AgeSeconds = %v, want 240", v.AgeSeconds)
	}
	if v.LastSuccessAt == nil {
		t.Errorf("LastSuccessAt is nil")
	}
}

func TestBuildViewOrdersNewestFirstWithinALevel(t *testing.T) {
	in := ViewInput{
		Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
		Alerts: []StoredAlert{
			testAlert(1, func(a *StoredAlert) { a.SentAt = viewNow.Add(-3 * time.Hour) }),
			testAlert(2, func(a *StoredAlert) { a.SentAt = viewNow.Add(-1 * time.Hour) }),
			testAlert(3, func(a *StoredAlert) { a.SentAt = viewNow.Add(-2 * time.Hour) }),
		},
	}
	v := BuildView(in)
	if got := rowIDs(v.Groups[0]); fmt.Sprint(got) != "[2 3 1]" {
		t.Errorf("rows = %v, want newest first", got)
	}
}

func TestBuildViewDistrictsLine(t *testing.T) {
	names := []string{"Ntr", "Krishna", "Guntur", "Eluru", "Bapatla", "Palnadu", "Tirupati"}
	codes := []int{749, 510, 506, 748, 750, 751, 752}
	in := ViewInput{
		Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
		Alerts: []StoredAlert{testAlert(1, func(a *StoredAlert) { a.DistrictCodes, a.DistrictNames = codes, names })},
	}
	v := BuildView(in)
	// The reader's district first, then the others by code.
	if want := "Ntr, Guntur, Krishna and 4 more"; v.DistrictsLine != want {
		t.Errorf("DistrictsLine = %q, want %q", v.DistrictsLine, want)
	}
	if got := districtsLine([]string{"A"}); got != "A district" {
		t.Errorf("one = %q", got)
	}
	if got := districtsLine([]string{"A", "B"}); got != "A and B" {
		t.Errorf("two = %q", got)
	}
	if got := districtsLine([]string{"A", "B", "C"}); got != "A, B and C" {
		t.Errorf("three = %q", got)
	}
	if got := districtsLine([]string{"A", "B", "C", "D"}); got != "A, B, C and 1 more" {
		t.Errorf("four = %q", got)
	}
	if got := districtsLine(nil); got != "" {
		t.Errorf("none = %q", got)
	}
}

func TestBuildViewGroupsNearIdentical(t *testing.T) {
	var alerts []StoredAlert
	for i := 1; i <= 45; i++ {
		i := i
		alerts = append(alerts, stateAlert(int64(i), func(a *StoredAlert) {
			a.Event = "Lightning"
			a.Severity = "Severe"
			a.HeadlineEN = "Lightning with thunder likely"
			a.AreaDesc = fmt.Sprintf("Mandal set %d", i)
			a.SentAt = viewNow.Add(-time.Duration(100-i) * time.Minute)
			a.ExpiresAt = viewNow.Add(time.Duration(i) * time.Minute)
		}))
	}
	// A different headline stays its own row.
	alerts = append(alerts, stateAlert(100, func(a *StoredAlert) {
		a.Event = "Lightning"
		a.Severity = "Severe"
		a.HeadlineEN = "A different sentence"
	}))
	v := BuildView(ViewInput{Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(), Alerts: alerts})
	if v.Count != 2 {
		t.Fatalf("Count = %d, want 2 (one grouped row and one single)", v.Count)
	}
	var grouped *Row
	for i := range v.Groups[0].Rows {
		if v.Groups[0].Rows[i].GroupedCount > 1 {
			grouped = &v.Groups[0].Rows[i]
		}
	}
	if grouped == nil {
		t.Fatal("no grouped row")
	}
	if grouped.GroupedCount != 45 {
		t.Errorf("GroupedCount = %d, want 45", grouped.GroupedCount)
	}
	if grouped.ID != 45 {
		t.Errorf("ID = %d, want the newest, 45", grouped.ID)
	}
	if want := viewNow.Add(-55 * time.Minute); !grouped.IssuedAt.Equal(want) {
		t.Errorf("IssuedAt = %v, want the newest %v", grouped.IssuedAt, want)
	}
	if want := viewNow.Add(45 * time.Minute); !grouped.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want the latest %v", grouped.ExpiresAt, want)
	}
	if grouped.AreaText != "Mandal set 45" {
		t.Errorf("AreaText = %q, want the newest alert's", grouped.AreaText)
	}
}

func TestBuildViewGroupingNeverCrossesSections(t *testing.T) {
	same := func(a *StoredAlert) { a.HeadlineEN = "Same words"; a.Event = "Flood"; a.Severity = "Severe" }
	v := BuildView(ViewInput{
		Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
		Alerts: []StoredAlert{testAlert(1, same), stateAlert(2, same)},
	})
	if v.Count != 2 || len(v.Groups) != 2 {
		t.Fatalf("Count %d groups %v", v.Count, groupKinds(v))
	}
}

func TestBuildViewHomeGroup(t *testing.T) {
	homeAlert := testAlert(7, func(a *StoredAlert) { a.DistrictCodes, a.DistrictNames = []int{506}, []string{"Guntur"} })
	readerAlert := testAlert(1, nil)

	t.Run("a different home district adds a group after the reader group", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(), Home: homeAreaGuntur(),
			Alerts: []StoredAlert{homeAlert, readerAlert},
		})
		if got := strings.Join(groupKinds(v), ","); got != "reader,home" {
			t.Fatalf("groups = %s", got)
		}
		if v.Groups[1].Label != "Your home area: Guntur district" {
			t.Errorf("home label = %q", v.Groups[1].Label)
		}
		if v.DistrictsLine != "Ntr and Guntur" {
			t.Errorf("DistrictsLine = %q", v.DistrictsLine)
		}
	})

	t.Run("a home area in the reader's district adds no group", func(t *testing.T) {
		home := &Area{DistrictCodes: []int{749}, DistrictName: "Ntr", StateCode: 28, StateName: "Andhra Pradesh"}
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(), Home: home,
			Alerts: []StoredAlert{readerAlert},
		})
		if got := strings.Join(groupKinds(v), ","); got != "reader" {
			t.Fatalf("groups = %s", got)
		}
	})

	t.Run("an alert for both districts shows once, in the reader group", func(t *testing.T) {
		both := testAlert(9, func(a *StoredAlert) { a.DistrictCodes, a.DistrictNames = []int{749, 506}, []string{"Ntr", "Guntur"} })
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(), Home: homeAreaGuntur(),
			Alerts: []StoredAlert{both},
		})
		if got := strings.Join(groupKinds(v), ","); got != "reader" || v.Count != 1 {
			t.Fatalf("groups = %s count %d", got, v.Count)
		}
	})

	t.Run("no reader point and a home area gives the home group only", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Home: homeAreaGuntur(),
			Alerts: []StoredAlert{homeAlert},
		})
		if !v.AreaKnown {
			t.Errorf("AreaKnown false")
		}
		if got := strings.Join(groupKinds(v), ","); got != "home" {
			t.Fatalf("groups = %s", got)
		}
	})

	t.Run("the state group follows the home state when there is no reader", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Home: homeAreaGuntur(),
			Alerts: []StoredAlert{stateAlert(3, nil)},
		})
		if got := strings.Join(groupKinds(v), ","); got != "state" || v.Groups[0].Label != "Across Andhra Pradesh" {
			t.Fatalf("groups = %s %v", got, v.Groups)
		}
	})

	t.Run("a home state apart from the reader's state gets its own group after it", func(t *testing.T) {
		home := &Area{DistrictCodes: []int{568}, DistrictName: "Chennai", StateCode: 33, StateName: "Tamil Nadu"}
		tn := stateAlert(4, func(a *StoredAlert) { a.StateCodes = []int{33} })
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(), Home: home,
			Alerts: []StoredAlert{tn, stateAlert(3, nil)},
		})
		if got := strings.Join(groupKinds(v), ","); got != "state,state" {
			t.Fatalf("groups = %s", got)
		}
		if v.Groups[0].Label != "Across Andhra Pradesh" || v.Groups[1].Label != "Across Tamil Nadu" {
			t.Errorf("labels = %q, %q", v.Groups[0].Label, v.Groups[1].Label)
		}
	})

	t.Run("neither gives AreaKnown false and no rows", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute),
			Alerts: []StoredAlert{readerAlert, unplacedAlert(5, nil)},
		})
		if v.AreaKnown || len(v.Groups) != 0 || v.Count != 0 {
			t.Fatalf("AreaKnown %v groups %v count %d", v.AreaKnown, v.Groups, v.Count)
		}
		if v.Groups == nil {
			t.Errorf("Groups must be an empty list, not nil")
		}
	})
}

func TestBuildViewBannerRule(t *testing.T) {
	t.Run("a Severe on the reader's district", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
			Alerts: []StoredAlert{testAlert(1, func(a *StoredAlert) { a.Severity = "Severe"; a.Event = "Severe Cyclonic Storm" })},
		})
		b := v.Banner
		if b == nil {
			t.Fatal("no banner")
		}
		if b.Level != "severe" || b.LevelWord != "Severe" || b.EventPhrase != "Cyclone warning" || b.Area != "Ntr district" || b.More != 0 || b.AlertID != 1 {
			t.Errorf("banner = %+v", *b)
		}
	})

	t.Run("an Extreme across the reader's state", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
			Alerts: []StoredAlert{stateAlert(2, func(a *StoredAlert) { a.Severity = "Extreme"; a.Event = "Flood" })},
		})
		if v.Banner == nil || v.Banner.Area != "Andhra Pradesh" || v.Banner.EventPhrase != "Flood warning" {
			t.Fatalf("banner = %+v", v.Banner)
		}
	})

	t.Run("only a Severe across the state gives none", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
			Alerts: []StoredAlert{stateAlert(2, withSeverity("Severe"))},
		})
		if v.Banner != nil {
			t.Fatalf("banner = %+v", *v.Banner)
		}
	})

	t.Run("a Moderate on the district gives none", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
			Alerts: []StoredAlert{testAlert(1, nil)},
		})
		if v.Banner != nil {
			t.Fatalf("banner = %+v", *v.Banner)
		}
	})

	t.Run("an Extreme on the home district only gives none", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(), Home: homeAreaGuntur(),
			Alerts: []StoredAlert{testAlert(3, func(a *StoredAlert) {
				a.Severity = "Extreme"
				a.DistrictCodes, a.DistrictNames = []int{506}, []string{"Guntur"}
			})},
		})
		if v.Banner != nil {
			t.Fatalf("banner = %+v", *v.Banner)
		}
		if v.Count != 1 {
			t.Errorf("the home alert must still be listed, count %d", v.Count)
		}
	})

	t.Run("no reader point gives none even for an Extreme at home", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Home: homeAreaGuntur(),
			Alerts: []StoredAlert{testAlert(3, func(a *StoredAlert) {
				a.Severity = "Extreme"
				a.DistrictCodes, a.DistrictNames = []int{506}, []string{"Guntur"}
			})},
		})
		if v.Banner != nil {
			t.Fatalf("banner = %+v", *v.Banner)
		}
	})

	t.Run("an unplaced Extreme never raises it", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
			Alerts: []StoredAlert{unplacedAlert(5, withSeverity("Extreme"))},
		})
		if v.Banner != nil {
			t.Fatalf("banner = %+v", *v.Banner)
		}
	})

	t.Run("two eligible rows pick the Extreme one with More 1", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
			Alerts: []StoredAlert{
				testAlert(1, func(a *StoredAlert) { a.Severity = "Severe"; a.SentAt = viewNow.Add(-time.Minute) }),
				stateAlert(2, func(a *StoredAlert) { a.Severity = "Extreme"; a.Event = "Cyclone" }),
			},
		})
		if v.Banner == nil || v.Banner.AlertID != 2 || v.Banner.More != 1 || v.Banner.LevelWord != "Extreme" {
			t.Fatalf("banner = %+v", v.Banner)
		}
	})

	t.Run("equal levels pick the newest", func(t *testing.T) {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
			Alerts: []StoredAlert{
				testAlert(1, func(a *StoredAlert) { a.Severity = "Severe"; a.SentAt = viewNow.Add(-3 * time.Hour) }),
				testAlert(2, func(a *StoredAlert) { a.Severity = "Severe"; a.SentAt = viewNow.Add(-1 * time.Hour) }),
			},
		})
		if v.Banner == nil || v.Banner.AlertID != 2 {
			t.Fatalf("banner = %+v", v.Banner)
		}
	})

	t.Run("a grouped row counts once in More", func(t *testing.T) {
		var alerts []StoredAlert
		for i := 1; i <= 5; i++ {
			i := i
			alerts = append(alerts, testAlert(int64(i), func(a *StoredAlert) {
				a.Severity = "Severe"
				a.HeadlineEN = "Same line"
			}))
		}
		alerts = append(alerts, testAlert(10, func(a *StoredAlert) { a.Severity = "Severe"; a.HeadlineEN = "Other line" }))
		v := BuildView(ViewInput{Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(), Alerts: alerts})
		if v.Banner == nil || v.Banner.More != 1 {
			t.Fatalf("banner = %+v", v.Banner)
		}
	})

	t.Run("stale or unavailable with eligible rows gives none", func(t *testing.T) {
		for _, ago := range []time.Duration{45 * time.Minute, 3 * time.Hour, 7 * time.Hour} {
			v := BuildView(ViewInput{
				Now: viewNow, Status: viewStatus(ago), Reader: readerArea(),
				Alerts: []StoredAlert{testAlert(1, withSeverity("Extreme"))},
			})
			if v.Banner != nil {
				t.Errorf("banner at %v: %+v", ago, *v.Banner)
			}
		}
	})
}

func TestBuildViewStaleAndUnavailable(t *testing.T) {
	alerts := []StoredAlert{
		testAlert(1, withSeverity("Extreme")),
		testAlert(2, func(a *StoredAlert) { a.ExpiresAt = viewNow.Add(-time.Minute) }), // past its own expiry
		unplacedAlert(3, nil),
	}

	stale := BuildView(ViewInput{Now: viewNow, Status: viewStatus(2 * time.Hour), Reader: readerArea(), Alerts: alerts})
	if stale.State != StateStale {
		t.Fatalf("state = %q", stale.State)
	}
	if stale.Banner != nil {
		t.Errorf("stale must have no banner")
	}
	if stale.Count != 2 {
		t.Errorf("stale keeps the rows still inside their expiry, count %d", stale.Count)
	}
	if stale.AgeSeconds == nil || *stale.AgeSeconds != 7200 || stale.LastSuccessAt == nil {
		t.Errorf("stale age %v last %v", stale.AgeSeconds, stale.LastSuccessAt)
	}

	for name, st := range map[string]FeedStatus{
		"seven hours": viewStatus(7 * time.Hour),
		"never":       {Feed: FeedSachet},
	} {
		un := BuildView(ViewInput{Now: viewNow, Status: st, Reader: readerArea(), Alerts: alerts})
		if un.State != StateUnavailable {
			t.Fatalf("%s: state = %q", name, un.State)
		}
		if len(un.Groups) != 0 || un.Count != 0 || un.Banner != nil || un.DistrictsLine != "" {
			t.Errorf("%s: unavailable sent data: %+v", name, un)
		}
		if un.Groups == nil {
			t.Errorf("%s: Groups must be an empty list, not nil", name)
		}
	}
	never := BuildView(ViewInput{Now: viewNow, Status: FeedStatus{Feed: FeedSachet}, Reader: readerArea()})
	if never.AgeSeconds != nil || never.LastSuccessAt != nil {
		t.Errorf("never checked must carry no age and no time: %+v", never)
	}
	old := BuildView(ViewInput{Now: viewNow, Status: viewStatus(7 * time.Hour), Reader: readerArea()})
	if old.AgeSeconds == nil || *old.AgeSeconds != 7*3600 || old.LastSuccessAt == nil {
		t.Errorf("unavailable keeps the last good time: %+v", old)
	}

	// Only a fresh answer may have an empty list; stale and unavailable never
	// say fresh.
	if empty := BuildView(ViewInput{Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea()}); empty.State != StateFresh || empty.Count != 0 {
		t.Errorf("fresh and empty = %+v", empty)
	}
}

func TestBuildViewLanguage(t *testing.T) {
	build := func(mut func(*StoredAlert)) Row {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
			Alerts: []StoredAlert{testAlert(1, mut)},
		})
		return v.Groups[0].Rows[0]
	}

	t.Run("english and telugu", func(t *testing.T) {
		r := build(func(a *StoredAlert) {
			a.HeadlineEN = "Heavy rain expected"
			a.HeadlineOriginal = "భారీ వర్షం"
			a.DescriptionOriginal = "వివరాలు"
			a.OriginalLang = "TL"
		})
		if r.Headline != "Heavy rain expected" || r.OriginalLanguageOnly {
			t.Errorf("headline %q only %v", r.Headline, r.OriginalLanguageOnly)
		}
		d := r.Details
		if d.OriginalLangLabel != "Telugu" || d.OriginalLangCode != "te" || d.OriginalHeadline != "భారీ వర్షం" || d.OriginalDescription != "వివరాలు" {
			t.Errorf("details = %+v", d)
		}
	})

	t.Run("only marathi", func(t *testing.T) {
		r := build(func(a *StoredAlert) {
			a.HeadlineEN = ""
			a.HeadlineOriginal = "मुसळधार पाऊस"
			a.OriginalLang = "MR"
		})
		if r.Headline != "मुसळधार पाऊस" || !r.OriginalLanguageOnly {
			t.Errorf("headline %q only %v", r.Headline, r.OriginalLanguageOnly)
		}
		if r.Details.OriginalLangLabel != "Marathi" || r.Details.OriginalLangCode != "mr" {
			t.Errorf("details = %+v", r.Details)
		}
	})

	t.Run("an unknown label keeps the label and no code", func(t *testing.T) {
		r := build(func(a *StoredAlert) {
			a.HeadlineOriginal = "x"
			a.OriginalLang = "XX"
		})
		if r.Details.OriginalLangLabel != "XX" || r.Details.OriginalLangCode != "" {
			t.Errorf("details = %+v", r.Details)
		}
	})

	t.Run("no headline at all falls back to the event", func(t *testing.T) {
		r := build(func(a *StoredAlert) { a.HeadlineEN = ""; a.HeadlineOriginal = ""; a.Event = "Dust storm" })
		if r.Headline != "Dust storm" || r.OriginalLanguageOnly {
			t.Errorf("headline %q only %v", r.Headline, r.OriginalLanguageOnly)
		}
	})
}

func TestLangLabel(t *testing.T) {
	cases := []struct{ in, label, code string }{
		{"TL", "Telugu", "te"}, {"tl", "Telugu", "te"}, {"MR", "Marathi", "mr"}, {"ML", "Malayalam", "ml"},
		{"HI", "Hindi", "hi"}, {"TA", "Tamil", "ta"}, {"en", "English", "en"}, {"en-IN", "English", "en"},
		{"XX", "XX", ""}, {"", "", ""},
	}
	for _, c := range cases {
		label, code := langLabel(c.in)
		if label != c.label || code != c.code {
			t.Errorf("langLabel(%q) = %q, %q; want %q, %q", c.in, label, code, c.label, c.code)
		}
	}
}

func TestBuildViewBadgesAndLevels(t *testing.T) {
	sev := map[string][2]string{
		"Extreme": {"extreme", "Extreme"}, "Severe": {"severe", "Severe"}, "Moderate": {"moderate", "Moderate"},
		"Minor": {"minor", "Minor"}, "Unknown": {"unknown", "Unknown"}, "": {"unknown", "Unknown"},
	}
	for in, want := range sev {
		v := BuildView(ViewInput{
			Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
			Alerts: []StoredAlert{testAlert(1, withSeverity(in))},
		})
		r := v.Groups[0].Rows[0]
		if r.Level != want[0] || r.LevelWord != want[1] {
			t.Errorf("%q: level %q word %q", in, r.Level, r.LevelWord)
		}
		if r.Source != "sachet" || r.Badge != "Official" || r.Demo {
			t.Errorf("%q: source %q badge %q demo %v", in, r.Source, r.Badge, r.Demo)
		}
		if r.Issuer != "IMD Chennai" || r.Details.SourceLine != "NDMA SACHET" {
			t.Errorf("issuer %q source line %q", r.Issuer, r.Details.SourceLine)
		}
	}

	demo := BuildView(ViewInput{
		Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(), Demo: true,
		Alerts: []StoredAlert{testAlert(1, nil), stateAlert(2, nil)},
	})
	for _, g := range demo.Groups {
		for _, r := range g.Rows {
			if !r.Demo || r.Badge != "" {
				t.Errorf("demo row = %+v", r)
			}
		}
	}
}

func TestBuildViewRowFacts(t *testing.T) {
	v := BuildView(ViewInput{
		Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
		Alerts: []StoredAlert{testAlert(1, func(a *StoredAlert) {
			a.ExpiresInferred = true
			a.Issuer = ""
			a.DescriptionEN = "It will rain."
			a.Instruction = "Stay indoors."
			a.AreaDesc = strings.Repeat("Mandal name, ", 40)
		})},
	})
	r := v.Groups[0].Rows[0]
	if r.Issuer != "IMD-Chennai" {
		t.Errorf("issuer falls back to the sender, got %q", r.Issuer)
	}
	if !r.ExpiresInferred || r.GroupedCount != 1 {
		t.Errorf("inferred %v grouped %d", r.ExpiresInferred, r.GroupedCount)
	}
	if !strings.HasSuffix(r.AreaText, "…") || len([]rune(r.AreaText)) > 161 {
		t.Errorf("AreaText = %q", r.AreaText)
	}
	if len(r.Details.Areas) < 400 {
		t.Errorf("Details.Areas must hold the full text, got %d bytes", len(r.Details.Areas))
	}
	if r.Details.Description != "It will rain." || r.Details.Instruction != "Stay indoors." {
		t.Errorf("details = %+v", r.Details)
	}
}

func TestTitleName(t *testing.T) {
	cases := map[string]string{
		"ANDHRA PRADESH":                   "Andhra Pradesh",
		"JAMMU & KASHMIR":                  "Jammu & Kashmir",
		"DADRA NAGAR HAVELI DAMAN AND DIU": "Dadra Nagar Haveli Daman and Diu",
		"":                                 "",
	}
	for in, want := range cases {
		if got := titleName(in); got != want {
			t.Errorf("titleName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestBuildViewDemoBanner (plan 05-19, D-05, D-07): the banner carries the Demo
// flag of its input, so the Nearby banner can draw the Demo tag, and a Demo row
// never names NDMA or any issuer's channel as its source (T-05-99).
func TestBuildViewDemoBanner(t *testing.T) {
	in := ViewInput{
		Now: viewNow, Status: viewStatus(time.Minute), Reader: readerArea(),
		Alerts: []StoredAlert{testAlert(1, withSeverity("Extreme"))},
	}
	real := BuildView(in)
	if real.Banner == nil {
		t.Fatal("an Extreme alert in the reader's district gave no banner")
	}
	if real.Banner.Demo {
		t.Errorf("a real view's banner has Demo true")
	}

	in.Demo = true
	sample := BuildView(in)
	if sample.Banner == nil {
		t.Fatal("a Demo view with an Extreme alert gave no banner")
	}
	if !sample.Banner.Demo {
		t.Errorf("a Demo view's banner has Demo false")
	}
	if got := sample.Groups[0].Rows[0].Details.SourceLine; got == "" || strings.Contains(got, "NDMA") {
		t.Errorf("a Demo row's source line = %q, want a non empty line that does not name NDMA", got)
	}
	if got := real.Groups[0].Rows[0].Details.SourceLine; got != "NDMA SACHET" {
		t.Errorf("a real row's source line = %q, want NDMA SACHET", got)
	}
}

// ---- GDACS hazards (plan 05-21) ----

func hazardStatus(ago time.Duration) FeedStatus {
	last := viewNow.Add(-ago)
	return FeedStatus{Feed: FeedGdacs, LastSuccessAt: &last}
}

// testHazard is a Red earthquake whose point is in district 749 unless mut
// changes it.
func testHazard(id int64, mut func(*StoredHazard)) StoredHazard {
	h := StoredHazard{
		ID:           id,
		EventType:    "EQ",
		EventID:      1000 + id,
		Name:         "Earthquake in India",
		AlertLevel:   "Red",
		Country:      "India",
		Lat:          16.5,
		Lon:          80.6,
		FromDate:     viewNow.Add(-3 * time.Hour),
		ToDate:       viewNow.Add(-time.Hour),
		SeverityText: "Magnitude 6.1M, Depth:10km",
		ReportURL:    "https://www.gdacs.org/report.aspx?eventid=1&eventtype=EQ",
		DistrictCode: 749,
	}
	if mut != nil {
		mut(&h)
	}
	return h
}

func TestBuildViewHazardRows(t *testing.T) {
	in := ViewInput{
		Now:          viewNow,
		Status:       viewStatus(4 * time.Minute),
		HazardStatus: hazardStatus(5 * time.Minute),
		Reader:       readerArea(),
		Home:         homeAreaGuntur(),
		Hazards: []StoredHazard{
			testHazard(1, nil),
			testHazard(2, func(h *StoredHazard) {
				h.AlertLevel, h.EventType, h.Name = "Orange", "TC", "Tropical Cyclone ARNAB-26"
				h.DistrictCode = 506
			}),
			testHazard(3, func(h *StoredHazard) {
				h.AlertLevel, h.EventType, h.Name = "Green", "FL", "Flood in India"
				h.DistrictCode = 749
			}),
			testHazard(4, func(h *StoredHazard) { h.DistrictCode = 600 }), // another district
			testHazard(5, func(h *StoredHazard) { h.DistrictCode = 0 }),   // outside every district
		},
	}
	v := BuildView(in)
	if v.HazardState != StateFresh || v.HazardAgeSeconds == nil || *v.HazardAgeSeconds != 300 || v.HazardLastSuccessAt == nil {
		t.Fatalf("hazard state %q age %v last %v, want fresh at 300 seconds", v.HazardState, v.HazardAgeSeconds, v.HazardLastSuccessAt)
	}
	if got := strings.Join(groupKinds(v), ","); got != "reader,home" {
		t.Fatalf("groups = %s, want reader and home only", got)
	}
	if got := rowIDs(v.Groups[0]); fmt.Sprint(got) != "[1 3]" {
		t.Errorf("reader rows = %v, want the Red earthquake then the Green flood", got)
	}
	if got := rowIDs(v.Groups[1]); fmt.Sprint(got) != "[2]" {
		t.Errorf("home rows = %v, want the Orange cyclone", got)
	}
	if v.Count != 3 {
		t.Errorf("Count = %d, want 3 (the two elsewhere are not listed)", v.Count)
	}

	red := v.Groups[0].Rows[0]
	if red.Source != "gdacs" || red.Badge != "GDACS" || red.Level != "extreme" || red.LevelWord != "Red" || red.Issuer != "GDACS" {
		t.Errorf("red row = %+v", red)
	}
	if red.Details.SourceLine != "GDACS. Automatic estimate. Follow your state disaster management authority." {
		t.Errorf("source line = %q", red.Details.SourceLine)
	}
	if red.Headline != "Earthquake in India" || red.AreaText != "India" || red.Details.Areas != "India" ||
		red.Details.Description != "Magnitude 6.1M, Depth:10km" || red.Demo || red.GroupedCount != 1 {
		t.Errorf("red row = %+v", red)
	}
	if !red.IssuedAt.Equal(in.Hazards[0].FromDate) || !red.ExpiresAt.Equal(in.Hazards[0].ToDate) {
		t.Errorf("red row times = %v to %v", red.IssuedAt, red.ExpiresAt)
	}
	green := v.Groups[0].Rows[1]
	if green.Level != "neutral" || green.LevelWord != "Green" || green.Badge != "GDACS" {
		t.Errorf("green row = %+v", green)
	}
	if orange := v.Groups[1].Rows[0]; orange.Level != "severe" || orange.LevelWord != "Orange" || orange.Source != "gdacs" {
		t.Errorf("orange row = %+v", orange)
	}

	t.Run("a hazard with no name still has a headline", func(t *testing.T) {
		in2 := in
		in2.Hazards = []StoredHazard{testHazard(1, func(h *StoredHazard) { h.Name = "" })}
		if got := BuildView(in2).Groups[0].Rows[0].Headline; got != "Earthquake" {
			t.Errorf("headline = %q, want the hazard word", got)
		}
	})

	t.Run("hazard rows come only with a fresh hazard feed", func(t *testing.T) {
		for name, st := range map[string]FeedStatus{
			"stale":       hazardStatus(2 * time.Hour),
			"unavailable": hazardStatus(7 * time.Hour),
			"never":       {Feed: FeedGdacs},
		} {
			in2 := in
			in2.HazardStatus = st
			v2 := BuildView(in2)
			if v2.Count != 0 || len(v2.Groups) != 0 {
				t.Errorf("%s: %d rows in %v, want none: an old hazard is not listed as current", name, v2.Count, groupKinds(v2))
			}
		}
		in2 := in
		in2.HazardStatus = hazardStatus(2 * time.Hour)
		if v2 := BuildView(in2); v2.HazardState != StateStale || v2.State != StateFresh {
			t.Errorf("hazard state %q alerts state %q, want stale and fresh: the two feeds are told apart", v2.HazardState, v2.State)
		}
		in2.HazardStatus = FeedStatus{Feed: FeedGdacs}
		if v2 := BuildView(in2); v2.HazardState != StateUnavailable || v2.HazardAgeSeconds != nil || v2.HazardLastSuccessAt != nil {
			t.Errorf("never checked: state %q age %v last %v", v2.HazardState, v2.HazardAgeSeconds, v2.HazardLastSuccessAt)
		}
	})

	t.Run("a hazard in the home district is the reader's row when they match", func(t *testing.T) {
		in2 := in
		in2.Home = &Area{DistrictCodes: []int{749}, DistrictName: "Ntr", StateCode: 28, StateName: "Andhra Pradesh"}
		v2 := BuildView(in2)
		if got := strings.Join(groupKinds(v2), ","); got != "reader" {
			t.Errorf("groups = %s, want reader only", got)
		}
	})

	t.Run("no area known means no rows", func(t *testing.T) {
		in2 := in
		in2.Reader, in2.Home = nil, nil
		if v2 := BuildView(in2); len(v2.Groups) != 0 || v2.AreaKnown {
			t.Errorf("groups %v areaKnown %v", groupKinds(v2), v2.AreaKnown)
		}
	})

	t.Run("hazards sort among the alerts by their level", func(t *testing.T) {
		in2 := in
		in2.Alerts = []StoredAlert{
			testAlert(21, withSeverity("Extreme")),
			testAlert(22, withSeverity("Moderate")),
		}
		v2 := BuildView(in2)
		// Extreme alert 21 was sent an hour ago; the Red hazard started three
		// hours ago, so the alert is first among the rank 0 rows.
		if got := rowIDs(v2.Groups[0]); fmt.Sprint(got) != "[21 1 22 3]" {
			t.Errorf("reader rows = %v, want [21 1 22 3]", got)
		}
	})
}

// TestHazardsNeverRaiseBanner: a GDACS event is an automatic estimate, so
// however serious its level it is never the Nearby banner and never counts as
// one of the "more" rows (D-12, T-05-111).
func TestHazardsNeverRaiseBanner(t *testing.T) {
	in := ViewInput{
		Now:          viewNow,
		Status:       viewStatus(4 * time.Minute),
		HazardStatus: hazardStatus(5 * time.Minute),
		Reader:       readerArea(),
		Hazards:      []StoredHazard{testHazard(1, nil), testHazard(2, func(h *StoredHazard) { h.AlertLevel = "Orange" })},
	}
	v := BuildView(in)
	if v.Count != 2 {
		t.Fatalf("Count = %d, want the two hazard rows listed", v.Count)
	}
	if v.Banner != nil {
		t.Fatalf("banner = %+v, want none for GDACS rows alone", *v.Banner)
	}

	in.Alerts = []StoredAlert{testAlert(9, withSeverity("Severe"))}
	v = BuildView(in)
	if v.Banner == nil || v.Banner.AlertID != 9 || v.Banner.More != 0 {
		t.Fatalf("banner = %+v, want the SACHET alert 9 with no more rows", v.Banner)
	}

	// A hazard can never be the picked row, even with the same rank and newer.
	in.Alerts = []StoredAlert{testAlert(9, withSeverity("Extreme"))}
	in.Hazards = []StoredHazard{testHazard(1, func(h *StoredHazard) { h.FromDate = viewNow.Add(-time.Minute) })}
	v = BuildView(in)
	if v.Banner == nil || v.Banner.AlertID != 9 {
		t.Fatalf("banner = %+v, want the SACHET alert even when the hazard is newer", v.Banner)
	}
}
