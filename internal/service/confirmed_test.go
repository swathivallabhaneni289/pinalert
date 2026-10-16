package service

import (
	"testing"
	"time"
)

// confirmedNow is the fixed clock of the Confirmed tests.
var confirmedNow = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

// confirmedOf runs the real resolver, the real partition and ConfirmedByPeople
// in the order the Nearby read does, so the table pins the whole chain.
func confirmedOf(meta ReportMeta, tally VoteTally, expiresAt time.Time) bool {
	vis, _ := Resolve(meta, tally, confirmedNow)
	view := ViewOf(vis, expiresAt, confirmedNow)
	return ConfirmedByPeople(vis, view, tally)
}

func TestConfirmedByPeople(t *testing.T) {
	future := confirmedNow.Add(time.Hour)

	low := ReportMeta{Severity: SeverityLow, Category: CategoryFlood}
	medium := ReportMeta{Severity: SeverityMedium, Category: CategoryRoadBlocked}
	critical := ReportMeta{Severity: SeverityCritical, Category: CategoryFire}
	oldRescue := ReportMeta{Severity: SeverityLow, Category: CategoryRescueNeeded}

	cases := []struct {
		name      string
		meta      ReportMeta
		tally     VoteTally
		expiresAt time.Time
		want      bool
	}{
		{"removed after review", ReportMeta{Severity: SeverityLow, Category: CategoryFlood, Moderation: ModerationRemoved}, VoteTally{ConfirmCells: 3}, future, false},
		{"removed critical after review", ReportMeta{Severity: SeverityCritical, Category: CategoryFire, Moderation: ModerationRemoved}, VoteTally{ConfirmCells: 3}, future, false},
		{"awaiting text check", ReportMeta{Severity: SeverityLow, Category: CategoryFlood, Moderation: ModerationPending}, VoteTally{ConfirmCells: 3}, future, false},
		{"retracted with confirm cells 3", low, VoteTally{ConfirmCells: 3, ReporterResolved: true}, future, false},
		{"retracted by independent resolve", low, VoteTally{ConfirmCells: 3, ResolveCells: 2}, future, false},
		{"low hidden by disputes", low, VoteTally{ConfirmCells: 1, DisputeCells: 3}, future, false},
		{"low provisional with confirm 0", low, VoteTally{}, future, false},
		{"low provisional with confirm 1", low, VoteTally{ConfirmCells: 1}, future, false},
		{"low live by confirmation", low, VoteTally{ConfirmCells: 2}, future, true},
		{"medium live with confirm 5 and dispute 1", medium, VoteTally{ConfirmCells: 5, DisputeCells: 1}, future, true},
		{"low live confirm 3 dispute 2", low, VoteTally{ConfirmCells: 3, DisputeCells: 2}, future, true},
		{"low tie at the threshold stays confirmed", low, VoteTally{ConfirmCells: 2, DisputeCells: 2}, future, true},
		{"critical with confirm 0", critical, VoteTally{}, future, false},
		{"critical with confirm 1", critical, VoteTally{ConfirmCells: 1}, future, false},
		{"critical with confirm 2", critical, VoteTally{ConfirmCells: 2}, future, true},
		{"critical with confirm 2 and dispute 5", critical, VoteTally{ConfirmCells: 2, DisputeCells: 5}, future, false},
		{"critical with confirm 3 and dispute 2", critical, VoteTally{ConfirmCells: 3, DisputeCells: 2}, future, true},
		{"critical with confirm 0 and dispute 5", critical, VoteTally{DisputeCells: 5}, future, false},
		{"old rescue_needed with confirm 0", oldRescue, VoteTally{}, future, false},
		{"old rescue_needed with confirm 2", oldRescue, VoteTally{ConfirmCells: 2}, future, true},
		{"old rescue_needed with confirm 2 and dispute 4", oldRescue, VoteTally{ConfirmCells: 2, DisputeCells: 4}, future, false},
		{"expired equal to now, low live", low, VoteTally{ConfirmCells: 2}, confirmedNow, false},
		{"expired equal to now, critical confirmed", critical, VoteTally{ConfirmCells: 2}, confirmedNow, false},
		{"expired in the past, medium live", medium, VoteTally{ConfirmCells: 5}, confirmedNow.Add(-time.Minute), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := confirmedOf(c.meta, c.tally, c.expiresAt); got != c.want {
				t.Fatalf("ConfirmedByPeople = %v, want %v", got, c.want)
			}
		})
	}
}

// TestConfirmedByPeopleFailsClosed pins that a visibility or view this package
// does not know is never confirmed.
func TestConfirmedByPeopleFailsClosed(t *testing.T) {
	tally := VoteTally{ConfirmCells: 5}
	if ConfirmedByPeople(Visibility("bogus"), FeedViewActive, tally) {
		t.Error("unknown visibility must not be confirmed")
	}
	if ConfirmedByPeople(VisibilityLive, FeedViewNone, tally) {
		t.Error("an empty view must not be confirmed")
	}
	if ConfirmedByPeople(VisibilityLive, FeedViewDisputed, tally) {
		t.Error("a disputed view must not be confirmed")
	}
	if ConfirmedByPeople(VisibilityLive, FeedViewPast, tally) {
		t.Error("a past view must not be confirmed")
	}
	if ConfirmedByPeople(VisibilityProvisional, FeedViewActive, tally) {
		t.Error("a provisional report must not be confirmed")
	}
}

// TestConfirmedByPeopleAgreesWithResolveForNonCritical ties the server authored
// Confirmed membership to the resolver for every report the critical bypass does
// not touch: confirmed exactly when Resolve says Live because confirmed. If
// Resolve's hidden or provisional rung ever changes, this fails and
// disputeMajority has to change with it.
func TestConfirmedByPeopleAgreesWithResolveForNonCritical(t *testing.T) {
	future := confirmedNow.Add(time.Hour)
	metas := []ReportMeta{
		{Severity: SeverityLow, Category: CategoryFlood},
		{Severity: SeverityMedium, Category: CategoryPowerOutage},
	}
	for _, meta := range metas {
		for confirm := 0; confirm <= 4; confirm++ {
			for dispute := 0; dispute <= 4; dispute++ {
				tally := VoteTally{ConfirmCells: confirm, DisputeCells: dispute}
				vis, reason := Resolve(meta, tally, confirmedNow)
				want := vis == VisibilityLive && reason == ReasonConfirmed
				got := confirmedOf(meta, tally, future)
				if got != want {
					t.Errorf("%s/%s confirm %d dispute %d: ConfirmedByPeople = %v, resolver says %v/%v",
						meta.Severity, meta.Category, confirm, dispute, got, vis, reason)
				}
			}
		}
	}
}

func TestListableInAll(t *testing.T) {
	cases := []struct {
		v    Visibility
		want bool
	}{
		{VisibilityLive, true},
		{VisibilityProvisional, true},
		{VisibilityHidden, true},
		{VisibilityRetracted, false},
		{VisibilityWithheld, false},
		{Visibility(""), false},
		{Visibility("bogus"), false},
		{Visibility("LIVE"), false},
	}
	for _, c := range cases {
		if got := c.v.ListableInAll(); got != c.want {
			t.Errorf("%q.ListableInAll() = %v, want %v", string(c.v), got, c.want)
		}
	}
}
