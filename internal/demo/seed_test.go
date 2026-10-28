package demo

import (
	"context"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/areas"
	"pinalert/internal/service"
)

// seededWorld builds a world and seeds it, failing the test on any refusal by
// the real services.
func seededWorld(t *testing.T) *World {
	t.Helper()
	w := NewWorld("abcdefghijklmnopqrstuv", time.Now)
	if err := Seed(w); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	return w
}

// trueKm is the true haversine distance, the form the service's vote radius
// check uses.
func trueKm(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	dPhi := (lat2 - lat1) * rad
	dLam := (lon2 - lon1) * rad
	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLam/2)*math.Sin(dLam/2)
	return 2 * 6371.0 * math.Asin(math.Sqrt(a))
}

func TestSeedCoversCategoriesAndSeverities(t *testing.T) {
	w := seededWorld(t)
	n := w.q.reportCount()
	if n < 15 || n > 25 {
		t.Fatalf("seeded %d reports, want 15 to 25", n)
	}
	cats := map[service.Category]int{}
	sevs := map[service.Severity]int{}
	for _, r := range w.q.reports {
		cats[service.Category(r.p.Category)]++
		sevs[service.Severity(r.p.Severity)]++
	}
	for _, c := range service.PostableCategories {
		if cats[c] == 0 {
			t.Errorf("no seed report in category %q", c)
		}
	}
	for _, s := range service.Severities {
		if sevs[s] < 2 {
			t.Errorf("only %d seed reports of severity %q, want at least 2", sevs[s], s)
		}
	}
}

func TestSeedStatesComeFromTheResolver(t *testing.T) {
	w := seededWorld(t)
	ctx := context.Background()

	// The table holds reports and votes only: the number of votes in the log is
	// exactly the number the table plans, so no state was written another way.
	planned := 0
	for _, sr := range seedReports {
		planned += sr.confirms + sr.disputes
	}
	if got := w.q.voteCount(); got != planned {
		t.Fatalf("vote log holds %d rows, the seed table plans %d", got, planned)
	}

	views, err := w.Nearby(ctx, service.NearbyQuery{Latitude: PretendLat, Longitude: PretendLon, RadiusKm: 20, ViewerAccountID: VisitorAccountID, All: true})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, disputed := 0, 0
	for _, v := range views {
		if v.ConfirmedByPeople {
			confirmed++
		}
		if v.View == service.FeedViewDisputed {
			disputed++
			if v.Visibility != service.VisibilityHidden {
				t.Errorf("a Disputed view with visibility %q", v.Visibility)
			}
		}
	}
	if confirmed < 3 {
		t.Errorf("%d seed reports read as confirmed by people, want at least 3", confirmed)
	}
	if disputed < 1 {
		t.Errorf("%d seed reports are in the Disputed view, want at least 1", disputed)
	}

	page, err := w.Reports.History(ctx, service.HistoryQuery{Latitude: PretendLat, Longitude: PretendLon, RadiusKm: 20, ViewerAccountID: VisitorAccountID})
	if err != nil {
		t.Fatal(err)
	}
	past, resolved := 0, 0
	for _, v := range page.Reports {
		switch v.View {
		case service.FeedViewPast:
			past++
		case service.FeedViewResolved:
			resolved++
		}
	}
	if past != 1 {
		t.Errorf("%d seed reports are in the Past view, want exactly 1", past)
	}
	if resolved != 0 {
		t.Errorf("%d seed reports are Resolved, want none (a Resolved seed would hide a state the resolver did not give)", resolved)
	}
}

func TestSeedNearAndFar(t *testing.T) {
	w := seededWorld(t)
	near, far := 0, 0
	for _, r := range w.q.reports {
		d := trueKm(PretendLat, PretendLon, r.p.Latitude, r.p.Longitude)
		switch {
		case d <= 0.85:
			near++
		case d >= 1.2:
			far++
		default:
			t.Errorf("report %d is %.2f km away: too close to the 1 km edge to teach either answer", r.id, d)
		}
	}
	if near < 8 || near > 12 {
		t.Errorf("%d reports within 1 km of the pretend position, want 8 to 12", near)
	}
	if far < 8 || far > 12 {
		t.Errorf("%d reports farther than 1 km, want 8 to 12", far)
	}
}

var digitRun = regexp.MustCompile(`[0-9]{6,}`)

func TestSeedHasNoPersonalData(t *testing.T) {
	w := seededWorld(t)
	banned := []string{"Road No", "Door No", "H.No", "Flat", "@"}
	for _, r := range w.q.reports {
		d := r.p.Description
		if len([]rune(d)) >= 200 {
			t.Errorf("description of report %d is %d characters, want under 200", r.id, len([]rune(d)))
		}
		if digitRun.MatchString(d) {
			t.Errorf("description of report %d holds a run of 6 or more digits: %q", r.id, d)
		}
		for _, b := range banned {
			if strings.Contains(d, b) {
				t.Errorf("description of report %d holds %q: %q", r.id, b, d)
			}
		}
		for _, ch := range d {
			isTelugu := unicode.Is(unicode.Telugu, ch)
			isPlain := ch >= 0x20 && ch < 0x7f
			if !isTelugu && !isPlain {
				t.Errorf("description of report %d holds the character %q: %q", r.id, ch, d)
			}
		}
	}
}

func TestSeedVotersAreIndependent(t *testing.T) {
	w := seededWorld(t)
	pretendCell := geohash.EncodeWithPrecision(PretendLat, PretendLon, 7)
	cells := map[int64]map[string]bool{}
	accounts := map[int64]map[int64]bool{}
	for _, v := range w.q.votes {
		if v.cell == pretendCell {
			t.Errorf("a seed vote on report %d sits in the pretend position's cell, so the visitor's own vote would add nothing", v.reportID)
		}
		if cells[v.reportID] == nil {
			cells[v.reportID] = map[string]bool{}
			accounts[v.reportID] = map[int64]bool{}
		}
		if cells[v.reportID][v.cell] {
			t.Errorf("two seed votes on report %d share the cell %s", v.reportID, v.cell)
		}
		if accounts[v.reportID][v.accountID] {
			t.Errorf("one seed account voted twice on report %d", v.reportID)
		}
		if v.accountID < SeedVoterAccountBase || v.accountID >= SeedVoterAccountBase+100 {
			t.Errorf("seed voter account %d is outside the seed voter range", v.accountID)
		}
		cells[v.reportID][v.cell] = true
		accounts[v.reportID][v.accountID] = true
	}
}

func TestSeedRestoresTheClock(t *testing.T) {
	wall := time.Now().UTC()
	w := NewWorld("abcdefghijklmnopqrstuv", func() time.Time { return wall })
	if err := Seed(w); err != nil {
		t.Fatal(err)
	}
	if !w.Now().Equal(wall) {
		t.Fatalf("after Seed the world clock is %v, want the wall clock %v", w.Now(), wall)
	}
	// Times are relative to the world start: nothing is in the future and the
	// newest report is a few minutes old.
	newest := time.Time{}
	for _, r := range w.q.reports {
		if r.p.CreatedAt.After(wall) {
			t.Errorf("report %d is created in the future: %v", r.id, r.p.CreatedAt)
		}
		if r.p.CreatedAt.After(newest) {
			newest = r.p.CreatedAt
		}
	}
	if age := wall.Sub(newest); age < 5*time.Minute || age > 30*time.Minute {
		t.Errorf("newest seed report is %v old, want between 5 and 30 minutes", age)
	}
}

func TestSeedSetupSeedsAStoreWorld(t *testing.T) {
	s := NewStore(WithWorldSetup(SeedSetup))
	w, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	if w.q.reportCount() == 0 {
		t.Fatal("a world created with SeedSetup is empty")
	}
}

func TestSampleAlerts(t *testing.T) {
	if n := len(SampleAlerts); n < 2 || n > 3 {
		t.Fatalf("%d sample alerts, want 2 or 3", n)
	}
	d, ok := areas.Default().Locate(PretendLat, PretendLon)
	if !ok {
		t.Fatal("the pretend position is in no district")
	}
	keys := map[string]bool{}
	extreme, severe, moderate := 0, 0, 0
	for _, a := range SampleAlerts {
		if keys[a.Key] {
			t.Errorf("duplicate sample alert key %q", a.Key)
		}
		keys[a.Key] = true
		if !a.Sample || !strings.HasPrefix(a.Key, "sample-") {
			t.Errorf("alert %q is not marked as a sample", a.Key)
		}
		if a.IssuedAgo <= 0 || a.EndsIn <= 0 {
			t.Errorf("alert %q has no relative times", a.Key)
		}
		switch a.Severity {
		case "Extreme":
			extreme++
			if a.Tier != "district" || !strings.EqualFold(a.Area, d.Name) {
				t.Errorf("Extreme alert %q is tier %q for %q, want district %q", a.Key, a.Tier, a.Area, d.Name)
			}
		case "Severe":
			severe++
			if a.Tier != "district" || !strings.EqualFold(a.Area, d.Name) {
				t.Errorf("Severe alert %q is tier %q for %q, want district %q", a.Key, a.Tier, a.Area, d.Name)
			}
		case "Moderate":
			moderate++
			if a.Tier != "state" || !strings.EqualFold(a.Area, "Andhra Pradesh") {
				t.Errorf("Moderate alert %q is tier %q for %q, want state Andhra Pradesh", a.Key, a.Tier, a.Area)
			}
		default:
			t.Errorf("alert %q has severity %q", a.Key, a.Severity)
		}
	}
	if extreme != 1 || severe != 1 || moderate != 1 {
		t.Errorf("extreme=%d severe=%d moderate=%d, want one of each", extreme, severe, moderate)
	}
}
