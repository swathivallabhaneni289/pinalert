package demo

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/service"
)

// fakeClock is a world wall clock the test moves by hand.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Now().UTC().Truncate(time.Second)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// practiceInput is a medium (non critical) practice report at the pretend
// position, the kind the Demo uses to show the climb.
func practiceInput() service.SubmitInput {
	return service.SubmitInput{
		Category:    service.CategoryFlood,
		Severity:    service.SeverityMedium,
		Description: "Practice report: water is knee deep across the lane",
		Latitude:    PretendLat,
		Longitude:   PretendLon,
	}
}

// viewOf reads one report back through the world, as the page does.
func viewOf(t *testing.T, w *World, id int64) service.ReportView {
	t.Helper()
	views, err := w.Nearby(context.Background(), service.NearbyQuery{Latitude: PretendLat, Longitude: PretendLon, RadiusKm: 5, All: true})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	for _, v := range views {
		if v.ID == id {
			return v
		}
	}
	t.Fatalf("report %d is not in the Nearby answer", id)
	return service.ReportView{}
}

func TestPlanNeighboursGeometry(t *testing.T) {
	posted := time.Now().UTC()
	plan := PlanNeighbours(77, PretendLat, PretendLon, posted)
	if len(plan) != 4 {
		t.Fatalf("PlanNeighbours gave %d votes, want 4", len(plan))
	}
	reportCell := geohash.EncodeWithPrecision(PretendLat, PretendLon, 7)
	seen := map[string]bool{reportCell: true}
	for i, v := range plan {
		k := i + 1
		if v.ReportID != 77 {
			t.Errorf("vote %d is for report %d", k, v.ReportID)
		}
		d := trueKm(PretendLat, PretendLon, v.Lat, v.Lon)
		if d > service.MaxVoteDistanceKm {
			t.Errorf("neighbour %d stands %.3f km away, past the vote radius", k, d)
		}
		if d < 0.25 {
			t.Errorf("neighbour %d stands %.3f km away, want at least 0.25", k, d)
		}
		cell := geohash.EncodeWithPrecision(v.Lat, v.Lon, 7)
		if seen[cell] {
			t.Errorf("neighbour %d shares the cell %s with the report or another neighbour", k, cell)
		}
		seen[cell] = true
		if want := NeighbourAccountBase + int64(k); v.AccountID != want {
			t.Errorf("neighbour %d has account %d, want %d", k, v.AccountID, want)
		}
		if v.AccountID == VisitorAccountID {
			t.Errorf("neighbour %d is the visitor", k)
		}
		if want := posted.Add(time.Duration(k) * 4 * time.Second); !v.DueAt.Equal(want) {
			t.Errorf("neighbour %d is due at %v, want %v", k, v.DueAt, want)
		}
	}
	if NeighbourInterval != 4*time.Second {
		t.Errorf("NeighbourInterval = %v", NeighbourInterval)
	}
}

func TestOffsetPointDistanceAndBearing(t *testing.T) {
	for _, bearing := range []float64{0, 90, 180, 270, 45} {
		lat, lon := offsetPoint(PretendLat, PretendLon, 500, bearing)
		if d := trueKm(PretendLat, PretendLon, lat, lon) * 1000; d < 499 || d > 501 {
			t.Errorf("bearing %v: moved %.1f m, want 500", bearing, d)
		}
	}
	n, _ := offsetPoint(PretendLat, PretendLon, 500, 0)
	if n <= PretendLat {
		t.Error("bearing 0 did not move north")
	}
	_, e := offsetPoint(PretendLat, PretendLon, 500, 90)
	if e <= PretendLon {
		t.Error("bearing 90 did not move east")
	}
}

func TestNeighboursClimbToConfirmed(t *testing.T) {
	clk := newFakeClock()
	w := NewWorld("abcdefghijklmnopqrstuv", clk.Now)
	res, err := w.PostPractice(context.Background(), practiceInput())
	if err != nil {
		t.Fatalf("PostPractice: %v", err)
	}
	id := res.Report.ID
	t0 := res.Report.CreatedAt

	// The words are the server's (trustcopy.go), compared exactly.
	const needTwo = "Needs 2 people at different spots to confirm it."
	const needOne = "Needs 1 more person at a different spot to confirm it."
	const confirmedTwo = "Confirmed by 2 people at different spots."

	v := viewOf(t, w, id)
	if v.Visibility != service.VisibilityProvisional || v.Trust.LeadSentence != needTwo {
		t.Fatalf("at t0: visibility=%v lead=%q, want provisional and %q", v.Visibility, v.Trust.LeadSentence, needTwo)
	}

	clk.Advance(3 * time.Second)
	if v = viewOf(t, w, id); v.Trust.LeadSentence != needTwo || v.Trust.ConfirmedPlaces != 0 {
		t.Fatalf("at t0+3s: lead=%q confirmed=%d, nobody should have voted yet", v.Trust.LeadSentence, v.Trust.ConfirmedPlaces)
	}

	clk.Advance(1 * time.Second) // t0 + 4 s
	if !w.Now().Equal(t0.Add(4 * time.Second)) {
		t.Fatalf("clock is %v, want t0+4s", w.Now())
	}
	v = viewOf(t, w, id)
	if v.Trust.ConfirmedPlaces != 1 || v.Trust.LeadSentence != needOne {
		t.Fatalf("at t0+4s: confirmed=%d lead=%q, want 1 and %q", v.Trust.ConfirmedPlaces, v.Trust.LeadSentence, needOne)
	}
	if v.Visibility != service.VisibilityProvisional {
		t.Fatalf("at t0+4s the report is %v, want still provisional", v.Visibility)
	}

	clk.Advance(4 * time.Second) // t0 + 8 s
	v = viewOf(t, w, id)
	if v.Visibility != service.VisibilityLive || !v.ConfirmedByPeople {
		t.Fatalf("at t0+8s: visibility=%v confirmedByPeople=%v, want live and confirmed", v.Visibility, v.ConfirmedByPeople)
	}
	if v.Trust.ConfirmedSentence != confirmedTwo {
		t.Fatalf("at t0+8s the sentence is %q, want %q", v.Trust.ConfirmedSentence, confirmedTwo)
	}
	if v.Trust.ConfirmedPlaces != service.IndependentAgreementThreshold {
		t.Fatalf("confirmed places = %d, want the threshold %d", v.Trust.ConfirmedPlaces, service.IndependentAgreementThreshold)
	}
}

func TestNeighboursStopAtThreshold(t *testing.T) {
	clk := newFakeClock()
	w := NewWorld("abcdefghijklmnopqrstuv", clk.Now)
	ctx := context.Background()
	res, err := w.PostPractice(ctx, practiceInput())
	if err != nil {
		t.Fatal(err)
	}
	id := res.Report.ID

	clk.Advance(8 * time.Second)
	if err := w.ApplyDue(ctx); err != nil {
		t.Fatal(err)
	}
	if got := w.q.voteCount(); got != service.IndependentAgreementThreshold {
		t.Fatalf("after confirmation the log holds %d votes, want %d", got, service.IndependentAgreementThreshold)
	}

	clk.Advance(22 * time.Second) // t0 + 30 s
	if err := w.ApplyDue(ctx); err != nil {
		t.Fatal(err)
	}
	if got := w.q.voteCount(); got != service.IndependentAgreementThreshold {
		t.Fatalf("at t0+30s the log holds %d votes, want %d (no further neighbour votes)", got, service.IndependentAgreementThreshold)
	}
	if v := viewOf(t, w, id); v.Trust.ConfirmedPlaces != service.IndependentAgreementThreshold {
		t.Fatalf("confirmed places = %d", v.Trust.ConfirmedPlaces)
	}

	// A reader who comes back late finds only the votes the script needs, not all
	// four that have fallen due.
	w2 := NewWorld("bbbbbbbbbbbbbbbbbbbbbb", clk.Now)
	if _, err := w2.PostPractice(ctx, practiceInput()); err != nil {
		t.Fatal(err)
	}
	clk.Advance(5 * time.Minute)
	if err := w2.ApplyDue(ctx); err != nil {
		t.Fatal(err)
	}
	if got := w2.q.voteCount(); got != service.IndependentAgreementThreshold {
		t.Fatalf("a late read cast %d votes, want %d", got, service.IndependentAgreementThreshold)
	}
}

func TestApplyDueDropsAPlanItCannotFinish(t *testing.T) {
	clk := newFakeClock()
	w := NewWorld("abcdefghijklmnopqrstuv", clk.Now)
	ctx := context.Background()
	if _, err := w.PostPractice(ctx, practiceInput()); err != nil {
		t.Fatal(err)
	}
	// Past the report's life the real service refuses every vote. The read must
	// still work, and the plan must be dropped rather than retried forever.
	clk.Advance(9 * time.Hour)
	if err := w.ApplyDue(ctx); err != nil {
		t.Fatalf("ApplyDue returned %v, want nil", err)
	}
	if _, err := w.Nearby(ctx, service.NearbyQuery{Latitude: PretendLat, Longitude: PretendLon, RadiusKm: 5, All: true}); err != nil {
		t.Fatalf("Nearby after a refused neighbour vote: %v", err)
	}
	if n := w.pendingNeighbourVotes(); n != 0 {
		t.Fatalf("%d planned votes remain after the report expired", n)
	}
	if got := w.q.voteCount(); got != 0 {
		t.Fatalf("%d votes were cast on an expired report", got)
	}
}
