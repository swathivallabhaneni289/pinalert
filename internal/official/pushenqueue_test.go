package official

// These tests pin the D-16 rule that decides whether an arriving alert version
// is queued for phones, the escalation rule for later versions, the home area
// match through the cells' districts, and the wiring of the hook into the
// service. None of them needs a database.

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/areas"
	sqlcgen "pinalert/internal/store/sqlc"
)

var pushNow = time.Date(2026, 10, 25, 9, 30, 0, 0, time.UTC)

// pushVersion is a district placed Severe Alert sent ten minutes ago that ends
// in three hours, the base of every grid case.
func pushVersion() SavedVersion {
	return SavedVersion{
		ID: 1,
		Version: Version{
			GUID:          "1792000000000011",
			CapIdentifier: "IN-1792000000000011_1",
			Sender:        "IMD-Chennai",
			SentAt:        pushNow.Add(-10 * time.Minute),
			Status:        "Actual",
			Scope:         "Public",
			MsgType:       MsgAlert,
			Event:         "Severe Cyclonic Storm",
			Severity:      SeveritySevere,
			ExpiresAt:     pushNow.Add(3 * time.Hour),
		},
		Placement: Placement{Tier: TierDistrict, DistrictCodes: []int{749}, StateCodes: []int{28}},
	}
}

func with(v SavedVersion, edit func(*SavedVersion)) SavedVersion {
	v.Placement.DistrictCodes = append([]int(nil), v.Placement.DistrictCodes...)
	v.Placement.StateCodes = append([]int(nil), v.Placement.StateCodes...)
	edit(&v)
	return v
}

// TestOfficialQualifies is the D-16 grid: which versions are queued, and why.
func TestOfficialQualifies(t *testing.T) {
	base := pushVersion()
	stateOf := func(sev Severity) SavedVersion {
		return with(base, func(v *SavedVersion) {
			v.Version.Severity = sev
			v.Placement = Placement{Tier: TierState, StateCodes: []int{28}}
		})
	}
	withSeverity := func(sev Severity) SavedVersion {
		return with(base, func(v *SavedVersion) { v.Version.Severity = sev })
	}
	update := func(edit func(*SavedVersion)) SavedVersion {
		return with(base, func(v *SavedVersion) {
			v.Version.MsgType = MsgUpdate
			edit(v)
		})
	}
	prevOf := func(v SavedVersion) *SavedVersion { return &v }

	cases := []struct {
		name   string
		saved  SavedVersion
		prev   *SavedVersion
		reason string
		ok     bool
	}{
		// A series seen the first time.
		{"district Severe", base, nil, "new", true},
		{"district Extreme", withSeverity(SeverityExtreme), nil, "new", true},
		{"state Extreme", stateOf(SeverityExtreme), nil, "new", true},
		{"state Severe never pushes (D-16)", stateOf(SeveritySevere), nil, "", false},
		{"district Moderate", withSeverity(SeverityModerate), nil, "", false},
		{"district Minor", withSeverity(SeverityMinor), nil, "", false},
		{"district Unknown", withSeverity(SeverityUnknown), nil, "", false},
		{"state Moderate", stateOf(SeverityModerate), nil, "", false},
		{"unplaced Extreme", with(base, func(v *SavedVersion) {
			v.Version.Severity = SeverityExtreme
			v.Placement = Placement{Tier: TierUnplaced}
		}), nil, "", false},
		{"a Cancel", with(base, func(v *SavedVersion) { v.Version.MsgType = MsgCancel }), nil, "", false},
		{"an Update seen the first time is new", update(func(v *SavedVersion) {}), nil, "new", true},
		{"a type that is neither", with(base, func(v *SavedVersion) { v.Version.MsgType = "Ack" }), nil, "", false},

		// The one hour and ten minute windows.
		{"sent 59 minutes ago", with(base, func(v *SavedVersion) { v.Version.SentAt = pushNow.Add(-59 * time.Minute) }), nil, "new", true},
		{"sent exactly an hour ago", with(base, func(v *SavedVersion) { v.Version.SentAt = pushNow.Add(-time.Hour) }), nil, "new", true},
		{"sent 61 minutes ago", with(base, func(v *SavedVersion) { v.Version.SentAt = pushNow.Add(-61 * time.Minute) }), nil, "", false},
		{"ends in 11 minutes", with(base, func(v *SavedVersion) { v.Version.ExpiresAt = pushNow.Add(11 * time.Minute) }), nil, "new", true},
		{"ends in exactly 10 minutes", with(base, func(v *SavedVersion) { v.Version.ExpiresAt = pushNow.Add(10 * time.Minute) }), nil, "", false},
		{"ends in 9 minutes", with(base, func(v *SavedVersion) { v.Version.ExpiresAt = pushNow.Add(9 * time.Minute) }), nil, "", false},
		{"already ended", with(base, func(v *SavedVersion) { v.Version.ExpiresAt = pushNow.Add(-time.Minute) }), nil, "", false},

		// A later version of a series seen before.
		{"Moderate raised to Severe", update(func(v *SavedVersion) {}), prevOf(withSeverity(SeverityModerate)), "escalated", true},
		{"Severe raised to Extreme", update(func(v *SavedVersion) { v.Version.Severity = SeverityExtreme }), prevOf(base), "escalated", true},
		{"Unknown raised to Extreme", update(func(v *SavedVersion) { v.Version.Severity = SeverityExtreme }), prevOf(withSeverity(SeverityUnknown)), "escalated", true},
		{"a wording only Update", update(func(v *SavedVersion) { v.Version.HeadlineEN = "new wording" }), prevOf(base), "", false},
		{"the same severity and districts", update(func(v *SavedVersion) {}), prevOf(update(func(v *SavedVersion) {})), "", false},
		{"Extreme lowered to Severe", update(func(v *SavedVersion) {}), prevOf(withSeverity(SeverityExtreme)), "", false},
		{"Severe lowered to Moderate", update(func(v *SavedVersion) { v.Version.Severity = SeverityModerate }), prevOf(base), "", false},
		{"a district added", update(func(v *SavedVersion) { v.Placement.DistrictCodes = []int{749, 750} }), prevOf(base), "escalated", true},
		{"a district removed", update(func(v *SavedVersion) { v.Placement.DistrictCodes = []int{749} }),
			prevOf(with(base, func(v *SavedVersion) { v.Placement.DistrictCodes = []int{749, 750} })), "", false},
		{"state Severe raised to state Extreme", with(stateOf(SeverityExtreme), func(v *SavedVersion) { v.Version.MsgType = MsgUpdate }),
			prevOf(stateOf(SeveritySevere)), "escalated", true},
		{"state Moderate raised to state Severe still never pushes", with(stateOf(SeveritySevere), func(v *SavedVersion) { v.Version.MsgType = MsgUpdate }),
			prevOf(stateOf(SeverityModerate)), "", false},
		{"an Update after a stored Cancel is new", update(func(v *SavedVersion) {}),
			prevOf(with(base, func(v *SavedVersion) { v.Version.MsgType = MsgCancel })), "new", true},
		{"an escalation still obeys the window", update(func(v *SavedVersion) {
			v.Version.Severity = SeverityExtreme
			v.Version.SentAt = pushNow.Add(-2 * time.Hour)
		}), prevOf(base), "", false},
		{"an escalation still obeys the end margin", update(func(v *SavedVersion) {
			v.Version.Severity = SeverityExtreme
			v.Version.ExpiresAt = pushNow.Add(5 * time.Minute)
		}), prevOf(base), "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason, ok := qualifies(c.saved, c.prev, pushNow)
			if ok != c.ok || reason != c.reason {
				t.Errorf("qualifies = %q, %v; want %q, %v", reason, ok, c.reason, c.ok)
			}
		})
	}
}

func TestOfficialNewlyCovered(t *testing.T) {
	mk := func(codes ...int) SavedVersion {
		return SavedVersion{Placement: Placement{Tier: TierDistrict, DistrictCodes: codes}}
	}
	prev := mk(1, 2)
	cases := []struct {
		name  string
		prev  *SavedVersion
		saved SavedVersion
		want  []int
	}{
		{"one added", &prev, mk(2, 3), []int{3}},
		{"none added", &prev, mk(1, 2), nil},
		{"all new when there is no predecessor", nil, mk(5, 4), []int{4, 5}},
		{"fewer", &prev, mk(1), nil},
		{"two added", &prev, mk(1, 2, 9, 7), []int{7, 9}},
	}
	for _, c := range cases {
		got := newlyCovered(c.prev, c.saved)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: newlyCovered = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestOfficialRecipientScope: a new alert, or one whose severity rose, reaches
// everyone its placement names; an Update that only adds districts reaches only
// the people in the added ones.
func TestOfficialRecipientScope(t *testing.T) {
	base := pushVersion()
	added := with(base, func(v *SavedVersion) {
		v.Version.MsgType = MsgUpdate
		v.Placement.DistrictCodes = []int{749, 750}
	})
	got := recipientScope(added, &base, "escalated")
	if !reflect.DeepEqual(got.Placement.DistrictCodes, []int{750}) {
		t.Errorf("districts added only: scope = %v, want [750]", got.Placement.DistrictCodes)
	}

	raised := with(added, func(v *SavedVersion) { v.Version.Severity = SeverityExtreme })
	got = recipientScope(raised, &base, "escalated")
	if !reflect.DeepEqual(got.Placement.DistrictCodes, []int{749, 750}) {
		t.Errorf("severity raised: scope = %v, want every district [749 750]", got.Placement.DistrictCodes)
	}

	got = recipientScope(base, nil, "new")
	if !reflect.DeepEqual(got.Placement.DistrictCodes, []int{749}) {
		t.Errorf("new alert: scope = %v, want [749]", got.Placement.DistrictCodes)
	}
}

// homeCell finds a precision 5 cell for the tests, by scanning the district's
// bounding box. interior means every one of the five points sits in the
// district; edge means a corner is in the district while the centre is not.
// TestCoverPrefixesHoldEveryMatchingCell pins WR-S05 of the Phase 5 review: the
// prefixes the candidate read filters by hold every home cell that the match in
// Go would accept, for district and state alerts, so the SQL filter never drops
// a person the alert reaches.
func TestCoverPrefixesHoldEveryMatchingCell(t *testing.T) {
	ix := areas.Default()
	for _, saved := range []SavedVersion{
		{Placement: Placement{Tier: TierDistrict, DistrictCodes: []int{749}}},
		{Placement: Placement{Tier: TierDistrict, DistrictCodes: []int{568, 515}}},
		{Placement: Placement{Tier: TierState, StateCodes: []int{28}}},
	} {
		in := map[string]bool{}
		for _, p := range coverPrefixes(ix, saved) {
			in[p] = true
		}
		var boxes [][4]float64
		for _, d := range ix.Districts() {
			if matchDistricts([]areas.District{d}, saved) {
				boxes = append(boxes, d.BBox)
			}
		}
		checked := 0
		for _, b := range boxes {
			for lat := b[1] - 0.1; lat <= b[3]+0.1; lat += 0.02 {
				for lon := b[0] - 0.1; lon <= b[2]+0.1; lon += 0.02 {
					cell := geohash.EncodeWithPrecision(lat, lon, 5)
					if !matchDistricts(ix.CellDistricts(cell), saved) {
						continue
					}
					checked++
					if !in[cell[:candidatePrefixLen]] {
						t.Fatalf("%+v: home cell %s matches but its prefix is not covered", saved.Placement, cell)
					}
				}
			}
		}
		if checked == 0 {
			t.Fatalf("%+v: no matching cell was checked", saved.Placement)
		}
	}
}

func homeCell(t *testing.T, ix *areas.Index, code int, edge bool) string {
	t.Helper()
	d, ok := ix.ByCode(code)
	if !ok {
		t.Fatalf("district %d is not in the boundary file", code)
	}
	const pad = 0.1
	for lat := d.BBox[1] - pad; lat <= d.BBox[3]+pad; lat += 0.01 {
		for lon := d.BBox[0] - pad; lon <= d.BBox[2]+pad; lon += 0.01 {
			cell := geohash.EncodeWithPrecision(lat, lon, 5)
			ds := ix.CellDistricts(cell)
			clat, clon := geohash.BoundingBox(cell).Center()
			centre, centreFound := ix.Locate(clat, clon)
			switch {
			case !edge && len(ds) == 1 && ds[0].Code == code:
				return cell
			case edge && containsCode(ds, code) && !(centreFound && centre.Code == code):
				return cell
			}
		}
	}
	t.Fatalf("no %s cell found for district %d", map[bool]string{false: "interior", true: "edge"}[edge], code)
	return ""
}

func containsCode(ds []areas.District, code int) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestOfficialHomeMatches(t *testing.T) {
	ix := areas.Default()
	ntr := homeCell(t, ix, 749, false)
	ntrEdge := homeCell(t, ix, 749, true)
	bapatla := homeCell(t, ix, 750, false)
	chennai := homeCell(t, ix, 568, false)

	district := pushVersion()
	state := with(district, func(v *SavedVersion) {
		v.Version.Severity = SeverityExtreme
		v.Placement = Placement{Tier: TierState, StateCodes: []int{28}}
	})
	unplaced := with(district, func(v *SavedVersion) { v.Placement = Placement{Tier: TierUnplaced} })
	// A district placed alert also carries its state rows; they must not widen
	// the match to the whole state.
	cases := []struct {
		name  string
		cell  string
		saved SavedVersion
		want  bool
	}{
		{"home in the district", ntr, district, true},
		{"a corner of the home cell in the district", ntrEdge, district, true},
		{"home in a neighbouring district", bapatla, district, false},
		{"home in another state", chennai, district, false},
		{"state alert, home in the state", ntr, state, true},
		{"state alert, home in another district of the state", bapatla, state, true},
		{"state alert, home in another state", chennai, state, false},
		{"unplaced alert", ntr, unplaced, false},
		{"not a cell", "zzzzz", district, false},
		{"empty cell", "", district, false},
	}
	for _, c := range cases {
		if got := homeMatches(c.cell, c.saved, ix); got != c.want {
			t.Errorf("%s: homeMatches = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestNewServiceRegistersPushHook: NewService adds the push hook before the
// caller's options, and the caller's hook still runs.
func TestNewServiceRegistersPushHook(t *testing.T) {
	plain := NewService(nil, nil, Config{})
	if n := len(plain.Refresher().hookList()); n != 1 {
		t.Fatalf("NewService registered %d hooks, want the push hook alone", n)
	}
	called := false
	extra := NewService(nil, nil, Config{}, WithVersionHook(func(context.Context, *sqlcgen.Queries, SavedVersion, *SavedVersion, time.Time) error {
		called = true
		return nil
	}))
	hooks := extra.Refresher().hookList()
	if len(hooks) != 2 {
		t.Fatalf("NewService with a caller hook registered %d hooks, want 2", len(hooks))
	}
	// The push hook comes first and, for a version that does not qualify, ends
	// before it touches the database; the caller's hook comes second.
	if err := hooks[0](context.Background(), nil, SavedVersion{}, nil, pushNow); err != nil {
		t.Errorf("the push hook on a version that does not qualify returned %v, want nil", err)
	}
	if called {
		t.Error("the first hook is the caller's, want the push hook first")
	}
	if err := hooks[1](context.Background(), nil, SavedVersion{}, nil, pushNow); err != nil || !called {
		t.Errorf("the second hook must be the caller's: called %v, err %v", called, err)
	}
}
