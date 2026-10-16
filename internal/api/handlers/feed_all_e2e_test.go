// feed_all_e2e_test.go proves plan 03.4-02's read over the real router and a
// real Postgres: view=all is the union of the active and the disputed answers
// for the same centre, never holds a retracted, expired, held or removed
// report, and every report carries the server authored confirmed_by_people
// (D-13, D-14, TRUST-02).
package handlers_test

import (
	"encoding/json"
	"net/http"
	"sort"
	"testing"
)

// allReport is the part of a feed report this file reads. ConfirmedByPeople is
// a pointer so a missing key reads as nil and fails the "always present" check.
type allReport struct {
	ID                int64  `json:"id"`
	View              string `json:"view"`
	Visibility        string `json:"visibility"`
	ConfirmedByPeople *bool  `json:"confirmed_by_people"`
}

type allList struct {
	View    string      `json:"view"`
	Reports []allReport `json:"reports"`
	Recent  *struct {
		Resolved24h int `json:"resolved_24h"`
		Expired24h  int `json:"expired_24h"`
	} `json:"recent"`
}

// allGet reads GET /api/reports for the h10 centre with extra query text.
func allGet(t *testing.T, client *http.Client, base string, lat, lon float64, extra string) allList {
	t.Helper()
	_, body := h10Get(t, client, h10URL(base, lat, lon, extra))
	var out allList
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding %q: %v (%s)", extra, err, body)
	}
	return out
}

func allIDs(reports []allReport) []int64 {
	ids := make([]int64, 0, len(reports))
	for _, r := range reports {
		ids = append(ids, r.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func allFind(reports []allReport, id int64) *allReport {
	for i := range reports {
		if reports[i].ID == id {
			return &reports[i]
		}
	}
	return nil
}

func TestFeedViewAll(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	author := newVerifiedClient(t, srv, mailer, "feed-all-author@example.com")
	voterB := newVerifiedClient(t, srv, mailer, "feed-all-b@example.com")
	voterC := newVerifiedClient(t, srv, mailer, "feed-all-c@example.com")

	bLat, bLon := h10Lat+0.003, h10Lon
	cLat, cLon := h10Lat-0.003, h10Lon
	assertDistinctCells(t, bLat, bLon, cLat, cLon)
	bLoc := map[string]any{"latitude": bLat, "longitude": bLon}
	cLoc := map[string]any{"latitude": cLat, "longitude": cLon}

	body := func(category, severity, desc string) map[string]any {
		return map[string]any{"category": category, "severity": severity, "description": desc, "latitude": h10Lat, "longitude": h10Lon}
	}
	both := func(id int64, action string) {
		t.Helper()
		decodeVoteResponse(t, postVote(t, voterB, srv.URL, id, action, bLoc))
		decodeVoteResponse(t, postVote(t, voterC, srv.URL, id, action, cLoc))
	}

	liveConfirmed := submitReportGetID(t, author, srv.URL, body("flood", "low", "Water over the road near the bridge"))
	provisional := submitReportGetID(t, author, srv.URL, body("power_outage", "low", "Street lights are out on the main road"))
	hidden := submitReportGetID(t, author, srv.URL, body("road_blocked", "low", "Road shut by a fallen tree at the junction"))
	critCold := submitReportGetID(t, author, srv.URL, body("fire", "critical", "Fire in the godown behind the market"))
	critConfirmed := submitReportGetID(t, author, srv.URL, body("flood", "critical", "Water inside the ground floor homes"))
	retracted := submitReportGetID(t, author, srv.URL, body("flood", "low", "Underpass is flooded knee deep"))
	oldRescue := storeReportAs(t, pool, author, srv.URL, body("rescue_needed", "low", "Neighbour stuck on the first floor"))
	expired := storeReportAs(t, pool, author, srv.URL, body("flood", "low", "Puddle by the school gate has gone"))
	held := storeReportAs(t, pool, author, srv.URL, body("flood", "low", "Tree down across the school road"))
	removed := storeReportAs(t, pool, author, srv.URL, body("flood", "low", "Culvert washed out on the south lane"))

	both(liveConfirmed, "confirm")
	both(critConfirmed, "confirm")
	both(hidden, "dispute")
	loc := map[string]any{"latitude": h10Lat, "longitude": h10Lon}
	decodeVoteResponse(t, postVote(t, author, srv.URL, retracted, "resolve", loc))
	h10MovePast(t, pool, expired, 7200, 3600)
	p407SetModeration(t, pool, held, "pending")
	p407SetModeration(t, pool, removed, "removed")

	// Reading as the voter B keeps every report listable and still not own.
	reader := voterB
	all := allGet(t, reader, srv.URL, h10Lat, h10Lon, "view=all")
	active := allGet(t, reader, srv.URL, h10Lat, h10Lon, "view=active")
	disputed := allGet(t, reader, srv.URL, h10Lat, h10Lon, "view=disputed")

	if all.View != "all" {
		t.Errorf("view = %q, want all", all.View)
	}

	t.Run("all is exactly the union of active and disputed with no duplicate", func(t *testing.T) {
		want := append(allIDs(active.Reports), allIDs(disputed.Reports)...)
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		got := allIDs(all.Reports)
		if len(got) != len(want) {
			t.Fatalf("all ids %v, want the union %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("all ids %v, want the union %v", got, want)
			}
			if i > 0 && got[i] == got[i-1] {
				t.Fatalf("duplicate id %d in all", got[i])
			}
		}
		for _, id := range []int64{liveConfirmed, provisional, hidden, critCold, critConfirmed, oldRescue} {
			if allFind(all.Reports, id) == nil {
				t.Errorf("report %d is missing from all", id)
			}
		}
	})

	t.Run("a finished, held or removed report is never in all", func(t *testing.T) {
		for _, id := range []int64{retracted, expired, held, removed} {
			if r := allFind(all.Reports, id); r != nil {
				t.Errorf("report %d is in all: %+v", id, *r)
			}
		}
	})

	t.Run("each report keeps its own view word", func(t *testing.T) {
		if r := allFind(all.Reports, hidden); r == nil || r.View != "disputed" || r.Visibility != "hidden" {
			t.Errorf("hidden report in all = %+v, want view disputed and visibility hidden", r)
		}
		for _, id := range []int64{liveConfirmed, provisional, critCold, critConfirmed, oldRescue} {
			if r := allFind(all.Reports, id); r == nil || r.View != "active" {
				t.Errorf("report %d in all = %+v, want view active", id, r)
			}
		}
	})

	t.Run("confirmed_by_people is present on every report and true only where people confirmed", func(t *testing.T) {
		wantTrue := map[int64]bool{liveConfirmed: true, critConfirmed: true}
		for _, r := range all.Reports {
			if r.ConfirmedByPeople == nil {
				t.Fatalf("report %d has no confirmed_by_people key", r.ID)
			}
			if *r.ConfirmedByPeople != wantTrue[r.ID] {
				t.Errorf("report %d confirmed_by_people = %v, want %v", r.ID, *r.ConfirmedByPeople, wantTrue[r.ID])
			}
		}
		// The unconfirmed Critical and the old rescue report are shown, live
		// through the bypass, and still not confirmed.
		for _, id := range []int64{critCold, oldRescue} {
			r := allFind(all.Reports, id)
			if r == nil || r.Visibility != "live" || r.ConfirmedByPeople == nil || *r.ConfirmedByPeople {
				t.Errorf("report %d = %+v, want live and not confirmed", id, r)
			}
		}
	})

	t.Run("the other views carry the field too", func(t *testing.T) {
		for _, list := range []allList{active, disputed} {
			for _, r := range list.Reports {
				if r.ConfirmedByPeople == nil {
					t.Errorf("view %s report %d has no confirmed_by_people key", list.View, r.ID)
				}
			}
		}
	})

	t.Run("an empty unfiltered all answer carries recent, a filtered one does not", func(t *testing.T) {
		const farLat, farLon = 28.6139, 77.2090
		empty := allGet(t, reader, srv.URL, farLat, farLon, "view=all")
		if len(empty.Reports) != 0 {
			t.Fatalf("far centre returned %d reports", len(empty.Reports))
		}
		if empty.Recent == nil {
			t.Error("empty all answer has no recent block")
		}
		filtered := allGet(t, reader, srv.URL, farLat, farLon, "view=all&category=flood")
		if filtered.Recent != nil {
			t.Error("a category filtered all answer must not carry recent")
		}
	})

	t.Run("a category filter narrows all and show_disputed with all is refused", func(t *testing.T) {
		fire := allGet(t, reader, srv.URL, h10Lat, h10Lon, "view=all&category=fire")
		if len(fire.Reports) != 1 || fire.Reports[0].ID != critCold {
			t.Errorf("fire only = %v, want just %d", allIDs(fire.Reports), critCold)
		}
		field, _ := h10GetRefused(t, reader, h10URL(srv.URL, h10Lat, h10Lon, "view=all&show_disputed=true"))
		if field != "view" {
			t.Errorf("refusal field = %q, want view", field)
		}
	})
}
