package demo

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestDemoParityWithSQL is the drift guard for the in memory twin (research
// G5). It runs one scripted scenario twice, once on a World and once on the
// same services built over a real database, and compares the answers step by
// step: the CastVote result (visibility, reason, trust object, undos left) and
// the Nearby view each of two readers gets (visibility, view word, the viewer
// fields, the confirm and dispute place counts, every trust sentence, the
// currency fields, the list contents). If the Go twin of a query ever stops
// agreeing with the SQL, this fails.
//
// Ids and timestamps are ignored, except that the report lifetime is compared
// to the nearest second; the distance is compared with a small tolerance.
//
// The test needs DATABASE_URL (testutil.NewTestDB skips without it).

// actor is one person in the scenario: where they stand and who they are in
// each run.
type actor struct {
	name     string
	lat, lon float64
	account  int64
	session  string
}

// runner is one side of the comparison.
type runner struct {
	label   string
	reports *service.ReportService
	votes   *service.VotingService
}

// castSnap is the part of a CastVoteResult that must agree.
type castSnap struct {
	Visibility service.Visibility
	Reason     service.ResolveReason
	Trust      service.Trust
	UndosLeft  int
}

// viewSnap is the part of a ReportView that must agree.
type viewSnap struct {
	Category             service.Category
	Severity             service.Severity
	Description          string
	Latitude, Longitude  float64
	Geohash              string
	LifetimeSeconds      int64
	Visibility           service.Visibility
	VisibilityReason     service.ResolveReason
	View                 service.FeedView
	ViewerVote           service.VoteValue
	ViewerResolution     service.VoteValue
	IsOwnReport          bool
	UndosLeft            int
	Approximate          bool
	Trust                service.Trust
	ViewerFlagged        bool
	ViewerFollows        bool
	ViewerMuted          bool
	ConfirmedByPeople    bool
	KeptByReliableFilter bool
	// ConfirmExtends was added 2026-10-30 (quick task 261030-64q, review WR-02):
	// with LifetimeSeconds it lets this test see an extension.
	ConfirmExtends bool
}

func snapView(v service.ReportView) (viewSnap, float64) {
	dist := 0.0
	if v.DistanceKm != nil {
		dist = *v.DistanceKm
	}
	return viewSnap{
		Category:             v.Category,
		Severity:             v.Severity,
		Description:          v.Description,
		Latitude:             v.Latitude,
		Longitude:            v.Longitude,
		Geohash:              v.Geohash,
		LifetimeSeconds:      int64(math.Round(v.ExpiresAt.Sub(v.CreatedAt).Seconds())),
		Visibility:           v.Visibility,
		VisibilityReason:     v.VisibilityReason,
		View:                 v.View,
		ViewerVote:           v.ViewerVote,
		ViewerResolution:     v.ViewerResolution,
		IsOwnReport:          v.IsOwnReport,
		UndosLeft:            v.UndosLeft,
		Approximate:          v.Approximate,
		Trust:                v.Trust,
		ViewerFlagged:        v.ViewerFlagged,
		ViewerFollows:        v.ViewerFollows,
		ViewerMuted:          v.ViewerMuted,
		ConfirmedByPeople:    v.ConfirmedByPeople,
		KeptByReliableFilter: v.KeptByReliableFilter,
		ConfirmExtends:       v.ConfirmExtends,
	}, dist
}

// stepResult is everything observed after one step.
type stepResult struct {
	label string
	err   string
	cast  *castSnap
	// views is keyed by reader name; dists are the matching distances.
	views map[string][]viewSnap
	dists map[string][]float64
}

// errName turns an error into a comparable word. The sentinels the scenario
// expects are named; anything else keeps its text.
func errName(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, service.ErrVoterTooFar):
		return "ErrVoterTooFar"
	case errors.Is(err, service.ErrCannotVoteOwnReport):
		return "ErrCannotVoteOwnReport"
	case errors.Is(err, service.ErrReportNotFound):
		return "ErrReportNotFound"
	case errors.Is(err, service.ErrReportExpired):
		return "ErrReportExpired"
	}
	return err.Error()
}

// observe reads the Nearby view of each reader and stores it on the result.
func (r runner) observe(t *testing.T, res *stepResult, readers []actor) {
	t.Helper()
	res.views = make(map[string][]viewSnap)
	res.dists = make(map[string][]float64)
	for _, rd := range readers {
		views, err := r.reports.Nearby(context.Background(), service.NearbyQuery{
			Latitude: rd.lat, Longitude: rd.lon, RadiusKm: 2, ViewerAccountID: rd.account, All: true,
		})
		if err != nil {
			t.Fatalf("%s: Nearby for %s after %s: %v", r.label, rd.name, res.label, err)
		}
		snaps := []viewSnap{}
		dists := []float64{}
		for _, v := range views {
			s, d := snapView(v)
			snaps = append(snaps, s)
			dists = append(dists, d)
		}
		res.views[rd.name] = snaps
		res.dists[rd.name] = dists
	}
}

// postFunc posts the scenario's report on one side and returns its id. A nil
// postFunc means the plain SubmitChecked post of a fresh report.
type postFunc func(t *testing.T, r runner, reporter actor) int64

// scenario runs the script on one side. reporter, a, b, c are the four people.
func scenario(t *testing.T, r runner, reporter, a, b, c actor, post postFunc) []stepResult {
	t.Helper()
	ctx := context.Background()
	readers := []actor{reporter, a}
	var out []stepResult

	record := func(label string, err error, cast *service.CastVoteResult) {
		res := stepResult{label: label, err: errName(err)}
		if cast != nil {
			undos := 0
			if cast.UndosLeft != nil {
				undos = *cast.UndosLeft
			}
			res.cast = &castSnap{Visibility: cast.Visibility, Reason: cast.Reason, Trust: cast.Trust, UndosLeft: undos}
		}
		r.observe(t, &res, readers)
		out = append(out, res)
	}

	// 1. The reporter posts a medium flood report.
	var id int64
	if post != nil {
		id = post(t, r, reporter)
	} else {
		posted, err := r.reports.SubmitChecked(ctx, reporter.session, reporter.account, service.SubmitInput{
			Category: service.CategoryFlood, Severity: service.SeverityMedium,
			Description: "Water is knee deep across the lane near the market",
			Latitude:    reporter.lat, Longitude: reporter.lon,
		})
		if err != nil {
			t.Fatalf("%s: SubmitChecked: %v", r.label, err)
		}
		id = posted.Report.ID
	}
	record("post", nil, nil)

	content := func(who actor, v service.VoteValue) (service.CastVoteResult, error) {
		return r.votes.CastVote(ctx, service.CastVoteInput{
			ReportID: id, AccountID: who.account, Kind: service.VoteKindContent, Value: v, Latitude: who.lat, Longitude: who.lon,
		})
	}
	resolution := func(who actor, v service.VoteValue) (service.CastVoteResult, error) {
		return r.votes.CastVote(ctx, service.CastVoteInput{
			ReportID: id, AccountID: who.account, Kind: service.VoteKindResolution, Value: v, LocationMissing: true,
		})
	}
	step := func(label string, f func() (service.CastVoteResult, error)) {
		res, err := f()
		if err != nil {
			record(label, err, nil)
			return
		}
		record(label, nil, &res)
	}

	// Two refusals first: the own report block and the 1 km rule.
	step("reporter confirms own report", func() (service.CastVoteResult, error) { return content(reporter, service.VoteConfirm) })
	far := actor{name: "far", lat: northOf(reporter.lat, 1.5), lon: reporter.lon, account: a.account, session: a.session}
	step("confirm from 1.5 km", func() (service.CastVoteResult, error) { return content(far, service.VoteConfirm) })

	// The six votes.
	step("A confirms", func() (service.CastVoteResult, error) { return content(a, service.VoteConfirm) })
	step("B confirms", func() (service.CastVoteResult, error) { return content(b, service.VoteConfirm) })
	step("C disputes", func() (service.CastVoteResult, error) { return content(c, service.VoteDispute) })
	step("A changes to dispute", func() (service.CastVoteResult, error) { return content(a, service.VoteDispute) })
	step("reporter resolves", func() (service.CastVoteResult, error) { return resolution(reporter, service.VoteResolve) })
	step("reporter reopens", func() (service.CastVoteResult, error) { return resolution(reporter, service.VoteReopen) })
	return out
}

// northOf is a latitude km kilometres north of lat.
func northOf(lat, km float64) float64 { return lat + km/111.195 }

func eastOf(lat, lon, km float64) float64 {
	return lon + km/(111.195*math.Cos(lat*math.Pi/180))
}

func TestDemoParityWithSQL(t *testing.T) { runDemoParity(t, false) }

// agedLead is how long before the scenario the aged report was posted, and
// agedLife the fade time it was given: posted 7 hours ago with the normal 8
// hour life, it fades in 1 hour.
const (
	agedLead = 7 * time.Hour
	agedLife = 8 * time.Hour
)

// TestDemoParityExtensionWithSQL is TestDemoParityWithSQL on an aged report, so
// a confirmation from a new spot really moves the fade time (backlog 999.12,
// review WR-02, 2026-10-30). On a fresh report an extension adds only the
// milliseconds between the post and the vote, which LifetimeSeconds rounds
// away, so the first test cannot tell an extension from none. Here the first
// confirm moves the fade time from 1 hour to 8 hours ahead, a lifetime of
// about 15 hours, in the SQL and in the twin alike, and ConfirmExtends is
// compared too. Both runs stamp votes from their own clock, so the answers
// agree to the second and no further.
func TestDemoParityExtensionWithSQL(t *testing.T) { runDemoParity(t, true) }

func runDemoParity(t *testing.T, aged bool) {
	pool := testutil.NewTestDB(t)

	// Positions: the reporter at the base point, A 300 m north, B 600 m east,
	// C 450 m south west. A and B and C must fall in different precision 7
	// cells and within 1 km of the report, or the counts would not exercise
	// independence.
	reporterAt := [2]float64{baseLat, baseLon}
	aAt := [2]float64{northOf(baseLat, 0.3), baseLon}
	bAt := [2]float64{baseLat, eastOf(baseLat, baseLon, 0.6)}
	cAt := [2]float64{northOf(baseLat, -0.32), eastOf(baseLat, baseLon, -0.32)}
	cells := map[string][2]float64{"reporter": reporterAt, "A": aAt, "B": bAt, "C": cAt}
	seen := map[string]string{}
	for name, p := range cells {
		cell := geohash.EncodeWithPrecision(p[0], p[1], 7)
		if other, dup := seen[cell]; dup {
			t.Fatalf("%s and %s share the precision 7 cell %s", name, other, cell)
		}
		seen[cell] = name
		if d := distanceKm(reporterAt, p); d > service.MaxVoteDistanceKm {
			t.Fatalf("%s is %.3f km from the report, past the vote radius", name, d)
		}
	}

	// The database run: real accounts and sessions through the test helpers.
	dbActor := func(name, email string, p [2]float64) actor {
		session := "parity-" + name
		acct := testutil.SeedReporterAccount(t, pool, email, session)
		return actor{name: name, lat: p[0], lon: p[1], account: acct, session: session}
	}
	dbRun := runner{
		label:   "database",
		reports: service.NewReportService(sqlcgen.New(pool)),
		votes:   service.NewVotingService(sqlcgen.New(pool)),
	}
	var dbPost, worldPost postFunc
	if aged {
		dbPost = func(t *testing.T, r runner, reporter actor) int64 {
			t.Helper()
			now := time.Now()
			return testutil.InsertStoredReport(t, pool, testutil.StoredReport{
				SessionID: reporter.session, Category: "flood", Severity: "medium",
				Description: "Water is knee deep across the lane near the market",
				Latitude:    reporter.lat, Longitude: reporter.lon,
				CreatedAt: now.Add(-agedLead), ExpiresAt: now.Add(agedLife - agedLead),
			})
		}
	}
	dbRes := scenario(t, dbRun,
		dbActor("reporter", "parity-reporter@example.test", reporterAt),
		dbActor("A", "parity-a@example.test", aAt),
		dbActor("B", "parity-b@example.test", bAt),
		dbActor("C", "parity-c@example.test", cAt),
		dbPost,
	)

	// The world run: synthetic ids, the real services over the twin.
	w := NewWorld("parityparityparityparity"[:22], time.Now)
	worldActor := func(name string, acct int64, p [2]float64) actor {
		return actor{name: name, lat: p[0], lon: p[1], account: acct, session: w.SessionFor(acct)}
	}
	worldRun := runner{label: "world", reports: w.Reports, votes: w.Votes}
	if aged {
		// The world posts through the real SubmitChecked with its clock moved
		// into the past, as seeding does, then puts the clock back.
		worldPost = func(t *testing.T, r runner, reporter actor) int64 {
			t.Helper()
			w.SetClockOffset(-agedLead)
			defer w.SetClockOffset(0)
			posted, err := r.reports.SubmitChecked(context.Background(), reporter.session, reporter.account, service.SubmitInput{
				Category: service.CategoryFlood, Severity: service.SeverityMedium,
				Description: "Water is knee deep across the lane near the market",
				Latitude:    reporter.lat, Longitude: reporter.lon,
			})
			if err != nil {
				t.Fatalf("%s: SubmitChecked: %v", r.label, err)
			}
			return posted.Report.ID
		}
	}
	worldRes := scenario(t, worldRun,
		worldActor("reporter", VisitorAccountID, reporterAt),
		worldActor("A", NeighbourAccountBase, aAt),
		worldActor("B", NeighbourAccountBase+1, bAt),
		worldActor("C", NeighbourAccountBase+2, cAt),
		worldPost,
	)

	if len(dbRes) != len(worldRes) {
		t.Fatalf("the runs have %d and %d steps", len(dbRes), len(worldRes))
	}
	sawListed, sawUnlisted := false, false
	for i := range dbRes {
		d, m := dbRes[i], worldRes[i]
		if d.label != m.label {
			t.Fatalf("step %d: labels %q and %q", i, d.label, m.label)
		}
		if d.err != m.err {
			t.Errorf("%s: database error %q, world error %q", d.label, d.err, m.err)
		}
		if !reflect.DeepEqual(d.cast, m.cast) {
			t.Errorf("%s: CastVote results differ\n database: %+v\n world:    %+v", d.label, d.cast, m.cast)
		}
		names := make([]string, 0, len(d.views))
		for n := range d.views {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			dv, mv := d.views[n], m.views[n]
			if len(dv) == 0 {
				sawUnlisted = true
			} else {
				sawListed = true
			}
			if !reflect.DeepEqual(dv, mv) {
				t.Errorf("%s: the Nearby view for %s differs\n database: %+v\n world:    %+v", d.label, n, dv, mv)
				continue
			}
			for j := range d.dists[n] {
				if math.Abs(d.dists[n][j]-m.dists[n][j]) > 1e-6 {
					t.Errorf("%s: distance for %s differs: database %v, world %v", d.label, n, d.dists[n][j], m.dists[n][j])
				}
			}
		}
	}
	// The scenario must exercise both an empty and a filled list, or the
	// resolve step proved nothing.
	if !sawListed || !sawUnlisted {
		t.Fatalf("the scenario never listed and unlisted the report (listed=%v unlisted=%v)", sawListed, sawUnlisted)
	}

	// Spot check that the interesting steps really happened, so a pair of equally
	// broken runs cannot pass: A and B confirmed from separate cells.
	for _, r := range [][]stepResult{dbRes, worldRes} {
		bConfirms := stepByLabel(r, "B confirms")
		if bConfirms == nil || bConfirms.cast == nil || bConfirms.cast.Trust.ConfirmedPlaces != 2 {
			t.Errorf("after B confirms, confirmed places = %+v, want 2", bConfirms)
		}
		// The extension itself (review WR-02): after A's confirm the report of the
		// aged run fades 8 hours after the vote, a lifetime of 15 hours, where a
		// run with no extension would still read 8 hours.
		aConfirms := stepByLabel(r, "A confirms")
		if aged {
			wantLife := int64((agedLead + agedLife) / time.Second)
			if aConfirms == nil || len(aConfirms.views["reporter"]) != 1 ||
				math.Abs(float64(aConfirms.views["reporter"][0].LifetimeSeconds-wantLife)) > 2 {
				t.Errorf("after A confirms on the aged report, lifetime = %+v, want about %d seconds", aConfirms, wantLife)
			}
		}
		resolved := stepByLabel(r, "reporter resolves")
		if resolved == nil || len(resolved.views["reporter"]) != 0 {
			t.Errorf("a resolved report is still listed for its reporter: %+v", resolved)
		}
		reopened := stepByLabel(r, "reporter reopens")
		if reopened == nil || len(reopened.views["reporter"]) != 1 {
			t.Errorf("a reopened report is not listed again: %+v", reopened)
		}
		own := stepByLabel(r, "reporter confirms own report")
		if own == nil || own.err != "ErrCannotVoteOwnReport" {
			t.Errorf("own confirm = %+v", own)
		}
		farStep := stepByLabel(r, "confirm from 1.5 km")
		if farStep == nil || farStep.err != "ErrVoterTooFar" {
			t.Errorf("far confirm = %+v", farStep)
		}
	}
}

func stepByLabel(steps []stepResult, label string) *stepResult {
	for i := range steps {
		if steps[i].label == label {
			return &steps[i]
		}
	}
	return nil
}

func distanceKm(a, b [2]float64) float64 {
	return haversineKm(a[0], a[1], b[0], b[1])
}
