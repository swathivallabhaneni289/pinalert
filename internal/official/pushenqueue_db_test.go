package official

// These tests run the push hook against a real database, through the same
// transaction that stores an alert version (pgStore.save), with people whose
// home cells are found in the boundary file. They prove who is queued, that a
// version queues once, and that the rules for later versions hold.

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/areas"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// pushWorld is one test's database, store and people.
type pushWorld struct {
	t    *testing.T
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
	st   *pgStore
	ix   *areas.Index
	n    int
}

func newPushWorld(t *testing.T) *pushWorld {
	t.Helper()
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ix := areas.Default()
	w := &pushWorld{t: t, pool: pool, q: q, ix: ix}
	w.st = &pgStore{db: pool, q: q, hooks: func() []VersionHook { return []VersionHook{PushEnqueueHook(ix)} }}
	return w
}

// person seeds an account with a live device and the given home cell. An empty
// cell leaves the home area unset.
func (w *pushWorld) person(label, cell string) int64 {
	w.t.Helper()
	w.n++
	email := fmt.Sprintf("push-%s-%d@example.com", label, w.n)
	session := fmt.Sprintf("push-sess-%s-%d", label, w.n)
	id := testutil.SeedReporterAccount(w.t, w.pool, email, session)
	testutil.SeedPushSubscription(w.t, w.pool, id, session, "")
	if cell != "" {
		testutil.SetHomeCell(w.t, w.pool, id, "Home", cell)
	}
	return id
}

// version is a Severe district Alert sent ten minutes before pushNow that ends
// three hours after it, from IMD Chennai.
func version(guid, capID string, sev Severity) Version {
	return Version{
		GUID:          guid,
		CapIdentifier: capID,
		Sender:        "IMD-Chennai",
		IssuerLabel:   "IMD Chennai",
		SentAt:        pushNow.Add(-10 * time.Minute),
		Status:        "Actual",
		Scope:         "Public",
		MsgType:       MsgAlert,
		Event:         "Severe Cyclonic Storm",
		Severity:      sev,
		ExpiresAt:     pushNow.Add(3 * time.Hour),
		SourcePubDate: pushNow.Add(-10 * time.Minute),
	}
}

func districts(codes ...int) Placement {
	return Placement{Tier: TierDistrict, DistrictCodes: codes, StateCodes: []int{28}}
}

func (w *pushWorld) save(v Version, p Placement) {
	w.t.Helper()
	if err := w.st.save(context.Background(), v, p, pushNow); err != nil {
		w.t.Fatalf("saving %s: %v", v.CapIdentifier, err)
	}
}

// queued returns the accounts with a row for the alert, sorted.
func (w *pushWorld) queued(guid string) []int64 {
	w.t.Helper()
	rows, err := w.pool.Query(context.Background(),
		`SELECT o.account_id FROM official_push_outbox o JOIN official_alerts a ON a.id = o.alert_id WHERE a.guid = $1 ORDER BY o.account_id, o.id`, guid)
	if err != nil {
		w.t.Fatalf("reading the queue: %v", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			w.t.Fatalf("scanning: %v", err)
		}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (w *pushWorld) rowCount(guid string, accountID int64, reason string) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM official_push_outbox o JOIN official_alerts a ON a.id = o.alert_id
		 WHERE a.guid = $1 AND o.account_id = $2 AND o.reason = $3`, guid, accountID, reason).Scan(&n); err != nil {
		w.t.Fatalf("counting rows: %v", err)
	}
	return n
}

func sameIDs(got []int64, want ...int64) bool {
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestOfficialPushEnqueue(t *testing.T) {
	w := newPushWorld(t)
	ntr := homeCell(t, w.ix, 749, false)
	ntrEdge := homeCell(t, w.ix, 749, true)
	otherAP := homeCell(t, w.ix, 746, false)
	chennai := homeCell(t, w.ix, 568, false)

	inDistrict := w.person("ntr", ntr)
	onEdge := w.person("edge", ntrEdge)
	inState := w.person("kakinada", otherAP)
	elsewhere := w.person("chennai", chennai)

	switchedOff := w.person("off", ntr)
	if _, err := w.q.UpsertOfficialPushSetting(context.Background(), sqlcgen.UpsertOfficialPushSettingParams{
		AccountID: switchedOff, OfficialPushOn: false, Now: pushNow}); err != nil {
		t.Fatalf("switch off: %v", err)
	}
	noDevice := testutil.SeedReporterAccount(t, w.pool, "push-nodevice@example.com", "push-nodevice-sess")
	testutil.SetHomeCell(t, w.pool, noDevice, "Home", ntr)
	noHome := w.person("nohome", "")
	_ = []int64{elsewhere, switchedOff, noDevice, noHome}

	// A district Severe on 749 reaches the person in 749 and the one whose cell
	// has a corner in it, and nobody else.
	const gDistrict = "1792000000000101"
	w.save(version(gDistrict, "IN-1792000000000101_1", SeveritySevere), districts(749))
	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box,
	// D2): the stored official switch is no longer read, so the account whose
	// stored switch is off is queued with the others.
	if got := w.queued(gDistrict); !sameIDs(got, inDistrict, onEdge, switchedOff) {
		t.Fatalf("district Severe queued %v, want the home in the district %d, the corner %d and the stored-off account %d", got, inDistrict, onEdge, switchedOff)
	}
	if w.rowCount(gDistrict, inDistrict, "new") != 1 {
		t.Error("the first queued row must carry the reason new")
	}
	var capID string
	var expires time.Time
	if err := w.pool.QueryRow(context.Background(),
		`SELECT cap_identifier, expires_at FROM official_push_outbox WHERE account_id = $1`, inDistrict).Scan(&capID, &expires); err != nil {
		t.Fatalf("reading the row: %v", err)
	}
	if capID != "IN-1792000000000101_1" {
		t.Errorf("cap_identifier = %q, want the version's identifier", capID)
	}
	if want := pushNow.Add(service.OfficialPushTTL); !expires.Equal(want) {
		t.Errorf("expires_at = %v, want now plus the push TTL %v (the alert ends later)", expires, want)
	}

	// Refreshing the same feed again queues no second row.
	w.save(version(gDistrict, "IN-1792000000000101_1", SeveritySevere), districts(749))
	if got := w.queued(gDistrict); !sameIDs(got, inDistrict, onEdge, switchedOff) {
		t.Errorf("after a repeat save the queue is %v, want the same three rows", got)
	}

	// An alert that ends sooner than the push TTL expires with the alert.
	const gShort = "1792000000000102"
	short := version(gShort, "IN-1792000000000102_1", SeveritySevere)
	short.ExpiresAt = pushNow.Add(30 * time.Minute)
	w.save(short, districts(749))
	if err := w.pool.QueryRow(context.Background(),
		`SELECT o.expires_at FROM official_push_outbox o JOIN official_alerts a ON a.id = o.alert_id WHERE a.guid = $1 AND o.account_id = $2`,
		gShort, inDistrict).Scan(&expires); err != nil {
		t.Fatalf("reading the short row: %v", err)
	}
	if !expires.Equal(short.ExpiresAt) {
		t.Errorf("expires_at = %v, want the alert's end %v", expires, short.ExpiresAt)
	}

	// A state level Extreme reaches the whole state: all three Andhra Pradesh
	// homes, not the person in Chennai.
	const gStateExtreme = "1792000000000103"
	w.save(version(gStateExtreme, "IN-1792000000000103_1", SeverityExtreme), Placement{Tier: TierState, StateCodes: []int{28}})
	if got := w.queued(gStateExtreme); !sameIDs(got, inDistrict, onEdge, inState, switchedOff) {
		t.Errorf("state Extreme queued %v, want the four homes in Andhra Pradesh (the stored-off account included)", got)
	}

	// A state level Severe queues nobody (D-16).
	const gStateSevere = "1792000000000104"
	w.save(version(gStateSevere, "IN-1792000000000104_1", SeveritySevere), Placement{Tier: TierState, StateCodes: []int{28}})
	if got := w.queued(gStateSevere); len(got) != 0 {
		t.Errorf("state Severe queued %v, want nobody", got)
	}
	// A district Moderate and an unplaced Extreme queue nobody either.
	const gModerate = "1792000000000105"
	w.save(version(gModerate, "IN-1792000000000105_1", SeverityModerate), districts(749))
	const gUnplaced = "1792000000000106"
	w.save(version(gUnplaced, "IN-1792000000000106_1", SeverityExtreme), Placement{Tier: TierUnplaced})
	if got := append(w.queued(gModerate), w.queued(gUnplaced)...); len(got) != 0 {
		t.Errorf("Moderate and unplaced alerts queued %v, want nobody", got)
	}

	// A sender outside the closed issuer table queues nobody and the version is
	// still stored (a hook that fails would roll it back).
	const gStranger = "1792000000000107"
	stranger := version(gStranger, "IN-1792000000000107_1", SeveritySevere)
	stranger.Sender = "Unknown-Office"
	w.save(stranger, districts(749))
	if got := w.queued(gStranger); len(got) != 0 {
		t.Errorf("an unknown issuer queued %v, want nobody", got)
	}
	var stored int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM official_alerts WHERE guid = $1`, gStranger).Scan(&stored); err != nil || stored != 1 {
		t.Errorf("the alert with an unknown issuer was stored %d times (%v), want 1", stored, err)
	}

	// The home cell never reaches the queue: the table holds ids and closed words.
	var nonIDColumns int
	if err := w.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.columns WHERE table_name = 'official_push_outbox'
		 AND column_name IN ('home_area_cell', 'latitude', 'longitude', 'endpoint', 'title', 'body')`).Scan(&nonIDColumns); err != nil || nonIDColumns != 0 {
		t.Errorf("official_push_outbox has %d position or word columns (%v), want none", nonIDColumns, err)
	}
}

func TestOfficialPushEnqueueUpdates(t *testing.T) {
	w := newPushWorld(t)
	inNtr := w.person("ntr", homeCell(t, w.ix, 749, false))
	inBapatla := w.person("bapatla", homeCell(t, w.ix, 750, false))
	inKakinada := w.person("kakinada", homeCell(t, w.ix, 746, false))

	// The series: a Severe alert on 749 first.
	const g = "1792000000000201"
	w.save(version(g, "IN-1792000000000201_1", SeveritySevere), districts(749))
	if got := w.queued(g); !sameIDs(got, inNtr) {
		t.Fatalf("the first version queued %v, want only the person in 749", got)
	}

	// An Update that only changes the wording queues nothing.
	reworded := version(g, "IN-1792000000000201_2", SeveritySevere)
	reworded.MsgType = MsgUpdate
	reworded.HeadlineEN = "Wording changed"
	w.save(reworded, districts(749))
	if got := w.queued(g); !sameIDs(got, inNtr) {
		t.Errorf("a wording only Update left the queue at %v, want the one row", got)
	}

	// An Update raising Severe to Extreme queues a new row per recipient with the
	// reason escalated.
	raised := version(g, "IN-1792000000000201_3", SeverityExtreme)
	raised.MsgType = MsgUpdate
	w.save(raised, districts(749))
	if w.rowCount(g, inNtr, "escalated") != 1 || w.rowCount(g, inNtr, "new") != 1 {
		t.Errorf("after the raise the person has %d new and %d escalated rows, want 1 and 1",
			w.rowCount(g, inNtr, "new"), w.rowCount(g, inNtr, "escalated"))
	}
	if got := w.queued(g); !sameIDs(got, inNtr, inNtr) {
		t.Errorf("after the raise the queue is %v, want two rows for the one person", got)
	}

	// An Update adding district 750 queues only the people whose home is in 750.
	added := version(g, "IN-1792000000000201_4", SeverityExtreme)
	added.MsgType = MsgUpdate
	w.save(added, districts(749, 750))
	if w.rowCount(g, inBapatla, "escalated") != 1 {
		t.Errorf("the person in the added district has %d escalated rows, want 1", w.rowCount(g, inBapatla, "escalated"))
	}
	if w.rowCount(g, inNtr, "escalated") != 1 {
		t.Errorf("the person who was already covered has %d escalated rows, want the earlier one only", w.rowCount(g, inNtr, "escalated"))
	}
	if w.rowCount(g, inKakinada, "escalated")+w.rowCount(g, inKakinada, "new") != 0 {
		t.Error("a person outside both districts was queued")
	}

	// The same Update stored again changes nothing.
	w.save(added, districts(749, 750))
	if got := w.queued(g); len(got) != 3 {
		t.Errorf("after a repeat the queue is %v, want 3 rows in all (new and escalated for one, escalated for the other)", got)
	}

	// An Update under another guid that names the earlier alert in its
	// references is judged against it: a wording only Update queues nothing, and
	// one that raises the severity is escalated.
	const g2 = "1792000000000202"
	w.save(version(g2, "IN-1792000000000202_1", SeveritySevere), districts(749))
	if got := w.queued(g2); !sameIDs(got, inNtr) {
		t.Fatalf("second alert queued %v, want the person in 749", got)
	}
	const g3 = "1792000000000203"
	cross := version(g3, "IN-1792000000000203_1", SeveritySevere)
	cross.MsgType = MsgUpdate
	cross.References = []string{"IN-1792000000000202_1"}
	w.save(cross, districts(749))
	if got := w.queued(g3); len(got) != 0 {
		t.Errorf("a wording only Update under a new guid queued %v, want nobody", got)
	}
	const g4 = "1792000000000204"
	crossRaised := version(g4, "IN-1792000000000204_1", SeverityExtreme)
	crossRaised.MsgType = MsgUpdate
	crossRaised.References = []string{"IN-1792000000000202_1"}
	w.save(crossRaised, districts(749))
	if w.rowCount(g4, inNtr, "escalated") != 1 {
		t.Errorf("a raising Update under a new guid gave %d escalated rows, want 1", w.rowCount(g4, inNtr, "escalated"))
	}

	// An alert that arrives after the Update or Cancel that replaces it (the
	// refresher stores the newest first) is not pushed.
	const old = "1792000000000205"
	replacing := version("1792000000000206", "IN-1792000000000206_1", SeverityExtreme)
	replacing.MsgType = MsgCancel
	replacing.References = []string{"IN-1792000000000205_1"}
	w.save(replacing, districts(749))
	w.save(version(old, "IN-1792000000000205_1", SeveritySevere), districts(749))
	if got := w.queued(old); len(got) != 0 {
		t.Errorf("an alert stored after the Cancel that withdraws it queued %v, want nobody", got)
	}
}

func TestOfficialPushRecipientCap(t *testing.T) {
	w := newPushWorld(t)
	cell := homeCell(t, w.ix, 749, false)
	var ids []int64
	for i := 0; i < 5; i++ {
		ids = append(ids, w.person(fmt.Sprintf("cap%d", i), cell))
	}
	other := w.person("outside", homeCell(t, w.ix, 568, false))
	_ = other

	old := recipientCap
	recipientCap = 3
	t.Cleanup(func() { recipientCap = old })

	const g = "1792000000000301"
	w.save(version(g, "IN-1792000000000301_1", SeveritySevere), districts(749))
	if got := w.queued(g); !sameIDs(got, ids[0], ids[1], ids[2]) {
		t.Errorf("with a cap of 3 the queue is %v, want the three lowest account ids %v", got, ids[:3])
	}
}

// TestOfficialPushCandidatesFilteredByAreaFirst pins WR-S05 of the Phase 5
// review: the candidate read is bounded only after it keeps the alert's area,
// so many lower id accounts elsewhere cannot crowd out a person in the district.
func TestOfficialPushCandidatesFilteredByAreaFirst(t *testing.T) {
	w := newPushWorld(t)
	old := recipientCap
	recipientCap = 2
	t.Cleanup(func() { recipientCap = old })

	elsewhere := homeCell(t, w.ix, 568, false) // Chennai
	for i := 0; i < recipientCap*candidateFactor+2; i++ {
		w.person(fmt.Sprintf("far%d", i), elsewhere)
	}
	inNtr := w.person("ntr", homeCell(t, w.ix, 749, false))
	edge := w.person("edge", homeCell(t, w.ix, 749, true))

	const g = "1792000000000401"
	w.save(version(g, "IN-1792000000000401_1", SeveritySevere), districts(749))
	if got := w.queued(g); !sameIDs(got, inNtr, edge) {
		t.Errorf("queued %v, want the two people in 749 (%d and %d) past the %d lower ids elsewhere",
			got, inNtr, edge, recipientCap*candidateFactor+2)
	}

	// A state tier Extreme alert reaches the person in the state the same way.
	const gs = "1792000000000402"
	v := version(gs, "IN-1792000000000402_1", SeverityExtreme)
	w.save(v, Placement{Tier: TierState, StateCodes: []int{28}})
	if got := w.queued(gs); !sameIDs(got, inNtr, edge) {
		t.Errorf("state alert queued %v, want %d and %d", got, inNtr, edge)
	}
}

// TestWithdrawnFirstNeverListedOrPushed pins CR-S01 of the Phase 5 review at
// the page and the queue: a Severe or Extreme alert whose Cancel, or whose
// Update under another guid, was stored first is not listed, not counted, does
// not raise the banner and is not queued for a phone.
func TestWithdrawnFirstNeverListedOrPushed(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind MsgType
		sev  Severity
	}{
		{"cancel stored first", MsgCancel, SeveritySevere},
		{"update stored first", MsgUpdate, SeverityExtreme},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newPushWorld(t)
			cell := homeCell(t, w.ix, 749, false)
			inNtr := w.person("ntr", cell)
			lat, lon := geohash.BoundingBox(cell).Center()
			const target, other = "1792000000000301", "1792000000000302"
			const targetID = "IN-1792000000000301_1"

			first := version(other, "IN-1792000000000302_1", SeverityModerate)
			first.MsgType = tc.kind
			first.References = []string{targetID}
			first.SentAt, first.SourcePubDate = pushNow.Add(-5*time.Minute), pushNow.Add(-5*time.Minute)
			w.save(first, districts(749))
			w.save(version(target, targetID, tc.sev), districts(749))

			if got := w.queued(target); len(got) != 0 {
				t.Errorf("the withdrawn alert was queued for %v", got)
			}
			ctx := context.Background()
			if _, err := w.q.MarkFeedAttempt(ctx, sqlcgen.MarkFeedAttemptParams{Feed: string(FeedSachet), Now: pushNow, Cutoff: pushNow}); err != nil {
				t.Fatalf("marking the attempt: %v", err)
			}
			if err := w.q.MarkFeedSuccess(ctx, sqlcgen.MarkFeedSuccessParams{Feed: string(FeedSachet), Now: pushNow, ItemsSeen: 2}); err != nil {
				t.Fatalf("marking the success: %v", err)
			}
			svc := newService(NewRefresher(w.pool, w.q), w.q, w.ix)
			svc.now = func() time.Time { return pushNow }
			v, err := svc.Alerts(ctx, inNtr, &lat, &lon)
			if err != nil {
				t.Fatalf("Alerts: %v", err)
			}
			if v.State != StateFresh {
				t.Fatalf("view state %q, want fresh so the list is read", v.State)
			}
			stored, err := w.q.OfficialAlertForUpdate(ctx, target)
			if err != nil {
				t.Fatalf("reading the target: %v", err)
			}
			for _, g := range v.Groups {
				for _, r := range g.Rows {
					if r.ID == stored.ID {
						t.Errorf("the withdrawn alert is listed in group %q", g.Kind)
					}
				}
			}
			if v.Banner != nil {
				t.Errorf("banner %+v, want none: the only Severe or Extreme alert was withdrawn", *v.Banner)
			}
			if tc.kind == MsgCancel && v.Count != 0 {
				t.Errorf("count %d, want 0", v.Count)
			}
			if tc.kind == MsgUpdate && v.Count != 1 {
				t.Errorf("count %d, want 1: only the Moderate update", v.Count)
			}
		})
	}
}
