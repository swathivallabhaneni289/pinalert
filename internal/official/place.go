package official

import (
	"sort"

	"pinalert/internal/areas"
)

// Tier says how precisely an alert could be placed (research G3).
type Tier string

const (
	// TierDistrict: one or more LGD District Codes in the message resolved.
	TierDistrict Tier = "district"
	// TierState: no code resolved, but the sender is a state authority or the
	// area text ends in a state name.
	TierState Tier = "state"
	// TierUnplaced: neither. The alert is still stored and is listed for
	// everyone; it is never in a banner or a push.
	TierUnplaced Tier = "unplaced"
)

// Placement is where an alert applies. DistrictCodes and StateCodes are sorted
// and unique. A district tier placement carries the states of its districts; a
// state tier placement carries one state and no districts.
type Placement struct {
	Tier          Tier
	DistrictCodes []int
	StateCodes    []int
}

// Place puts a version on the map by the tiers of research G3.
//
// Tier 1 reads the LGD District Codes the message carries, keeping only those
// the boundary index knows. Tier 3 reads the state from a <State>-SDMA sender
// and then from a trailing state name in the area text. An alert that neither
// can place is unplaced. Research tier 2 (an inline polygon or circle) is not
// built: it never appeared in the feed and nothing depends on it.
func Place(v Version, ix *areas.Index) Placement {
	if ix == nil {
		return Placement{Tier: TierUnplaced}
	}
	districts := map[int]bool{}
	states := map[int]bool{}
	for _, code := range v.DistrictCodes {
		if d, ok := ix.ByCode(code); ok {
			districts[d.Code] = true
			states[d.StateCode] = true
		}
	}
	if len(districts) > 0 {
		return Placement{Tier: TierDistrict, DistrictCodes: sortedKeys(districts), StateCodes: sortedKeys(states)}
	}
	if s, ok := ix.StateForSender(v.Sender); ok {
		return Placement{Tier: TierState, StateCodes: []int{s.Code}}
	}
	if s, ok := ix.TrailingState(v.AreaDesc); ok {
		return Placement{Tier: TierState, StateCodes: []int{s.Code}}
	}
	return Placement{Tier: TierUnplaced}
}

func sortedKeys(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
