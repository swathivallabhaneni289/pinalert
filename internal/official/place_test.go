package official

import (
	"reflect"
	"testing"

	"pinalert/internal/areas"
)

// TestPlaceTiers pins the placement tiers of research G3 on the recorded
// messages: district codes first, then a state from the sender, then a state
// from the end of the area text, else unplaced.
func TestPlaceTiers(t *testing.T) {
	ix := areas.Default()
	state := func(name string) int {
		t.Helper()
		s, ok := ix.StateByName(name)
		if !ok {
			t.Fatalf("state %q is not in the index", name)
		}
		return s.Code
	}

	t.Run("Chennai message places on its districts", func(t *testing.T) {
		v := mustParseCAP(t, fixture(t, "cap_imd_chennai.xml"), fixtureItem("1792915039196007", "IMD Chennai"))
		p := Place(v, ix)
		if p.Tier != TierDistrict {
			t.Fatalf("tier = %q, want district", p.Tier)
		}
		if len(p.DistrictCodes) != 23 {
			t.Errorf("%d districts, want 23: %v", len(p.DistrictCodes), p.DistrictCodes)
		}
		if want := []int{state("Tamil Nadu")}; !reflect.DeepEqual(p.StateCodes, want) {
			t.Errorf("states = %v, want %v", p.StateCodes, want)
		}
	})

	t.Run("Maharashtra update places on its three districts", func(t *testing.T) {
		v := mustParseCAP(t, fixture(t, "cap_maharashtra_update.xml"), fixtureItem("1792859773702029", ""))
		p := Place(v, ix)
		if p.Tier != TierDistrict || !reflect.DeepEqual(p.DistrictCodes, []int{480, 492, 495}) {
			t.Errorf("placement = %+v, want districts 480 492 495", p)
		}
		if want := []int{state("Maharashtra")}; !reflect.DeepEqual(p.StateCodes, want) {
			t.Errorf("states = %v, want %v", p.StateCodes, want)
		}
	})

	t.Run("a state authority with no codes places on its state", func(t *testing.T) {
		v := mustParseCAP(t, fixture(t, "cap_ap_sdma_lightning.xml"), fixtureItem("1792919196941008", "Andhra Pradesh SDMA"))
		p := Place(v, ix)
		want := Placement{Tier: TierState, StateCodes: []int{state("Andhra Pradesh")}}
		if !reflect.DeepEqual(p, want) {
			t.Errorf("placement = %+v, want %+v", p, want)
		}
	})

	t.Run("a trailing state name places on that state", func(t *testing.T) {
		v := mustParseCAP(t, fixture(t, "cap_cwc_flood.xml"), fixtureItem("1792897745415016", "CWC"))
		p := Place(v, ix)
		want := Placement{Tier: TierState, StateCodes: []int{state("Bihar")}}
		if !reflect.DeepEqual(p, want) {
			t.Errorf("placement = %+v, want %+v", p, want)
		}
	})

	t.Run("codes that do not resolve and no state leave it unplaced", func(t *testing.T) {
		v := Version{Sender: "IMD-X", AreaDesc: "6 Mandals", DistrictCodes: []int{99998, 99999}}
		p := Place(v, ix)
		if p.Tier != TierUnplaced || len(p.DistrictCodes) != 0 || len(p.StateCodes) != 0 {
			t.Errorf("placement = %+v, want unplaced and empty", p)
		}
	})

	t.Run("an unknown code is dropped and a known one is kept", func(t *testing.T) {
		v := Version{Sender: "IMD-Chennai", DistrictCodes: []int{568, 99999}}
		p := Place(v, ix)
		if p.Tier != TierDistrict || !reflect.DeepEqual(p.DistrictCodes, []int{568}) {
			t.Errorf("placement = %+v, want district tier with 568 only", p)
		}
	})

	t.Run("codes win over the sender", func(t *testing.T) {
		v := Version{Sender: "Andhra-Pradesh-SDMA", DistrictCodes: []int{568}}
		p := Place(v, ix)
		if p.Tier != TierDistrict || !reflect.DeepEqual(p.StateCodes, []int{state("Tamil Nadu")}) {
			t.Errorf("placement = %+v, want the districts' own state", p)
		}
	})

	t.Run("no index leaves it unplaced", func(t *testing.T) {
		if p := Place(Version{DistrictCodes: []int{568}}, nil); p.Tier != TierUnplaced {
			t.Errorf("tier = %q with no index, want unplaced", p.Tier)
		}
	})
}
