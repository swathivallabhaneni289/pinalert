// updatesalerts_test.go proves the Reliable reporter alert step of Refresh
// (plan 03.2-13, NOTIF-05, D-26, D-29): the gate, the anchor, the cap and the
// words, first with a fake querier and a fake nearby reader, then once on a
// real database with a seeded Reliable reporter.

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// alertFake lets a test set the four alert answers that plan 03's fake returns
// as zeros. It embeds that fake so every other call is recorded as before.
type alertFake struct {
	*fakeUpdatesQuerier

	settings *sqlcgen.GetAlertSettingsRow // nil answers pgx.ErrNoRows, the defaults
	homeCell *string
	inWindow map[int64]int64 // alerts already sent, by reporter account id
	// insertErr is returned by every InsertReliableAlert call.
	insertErr error

	windows  []sqlcgen.AlertsFromReporterInWindowParams
	inserted []sqlcgen.InsertReliableAlertParams
}

func (f *alertFake) GetAlertSettings(_ context.Context, id int64) (sqlcgen.GetAlertSettingsRow, error) {
	f.rec("GetAlertSettings", id, true, nil)
	if f.settings == nil {
		return sqlcgen.GetAlertSettingsRow{}, pgx.ErrNoRows
	}
	return *f.settings, nil
}

func (f *alertFake) GetAccountHomeCell(_ context.Context, id int64) (*string, error) {
	f.rec("GetAccountHomeCell", id, true, nil)
	return f.homeCell, nil
}

func (f *alertFake) AlertsFromReporterInWindow(_ context.Context, a sqlcgen.AlertsFromReporterInWindowParams) (int64, error) {
	f.rec("AlertsFromReporterInWindow", a.RecipientID, true, a)
	f.windows = append(f.windows, a)
	return f.inWindow[a.ReporterID], nil
}

func (f *alertFake) InsertReliableAlert(_ context.Context, a sqlcgen.InsertReliableAlertParams) (int64, error) {
	f.rec("InsertReliableAlert", a.AccountID, true, a)
	f.inserted = append(f.inserted, a)
	if f.insertErr != nil {
		return 0, f.insertErr
	}
	return int64(len(f.inserted)), nil
}

// fakeNearby records the queries it is asked and answers canned views.
type fakeNearby struct {
	views   []ReportView
	err     error
	queries []NearbyQuery
}

func (n *fakeNearby) Nearby(_ context.Context, q NearbyQuery) ([]ReportView, error) {
	n.queries = append(n.queries, q)
	return n.views, n.err
}

// alertOn is the stored settings of an account that turned the alert on three
// hours before updatesNow, at the given distance.
func alertOn(radius int16) *sqlcgen.GetAlertSettingsRow {
	return &sqlcgen.GetAlertSettingsRow{
		ReliableAlertsOn: true,
		AlertRadiusKm:    radius,
		EnabledAt:        pgtype.Timestamptz{Time: updatesNow.Add(-3 * time.Hour), Valid: true},
	}
}

func cellPtr(c string) *string { return &c }

// alertView builds one nearby answer.
func alertView(id int64, vis Visibility, tag ReporterTag, createdAgo time.Duration) ReportView {
	return ReportView{
		Report: Report{
			ID:        id,
			CreatedAt: updatesNow.Add(-createdAgo),
			ExpiresAt: updatesNow.Add(6 * time.Hour),
		},
		Visibility: vis,
		Trust:      Trust{Reporter: ReporterSignal{Tag: tag}},
	}
}

func reporterRows(pairs ...int64) []sqlcgen.ReporterAccountsForReportsRow {
	var rows []sqlcgen.ReporterAccountsForReportsRow
	for i := 0; i+1 < len(pairs); i += 2 {
		acct := pairs[i+1]
		rows = append(rows, sqlcgen.ReporterAccountsForReportsRow{ReportID: pairs[i], ReporterAccountID: &acct})
	}
	return rows
}

func newAlertService(f *alertFake, n NearbyReader) *UpdatesService {
	return NewUpdatesService(f, n, WithUpdatesClock(func() time.Time { return updatesNow }))
}

func insertedReportIDs(f *alertFake) []int64 {
	ids := make([]int64, 0, len(f.inserted))
	for _, p := range f.inserted {
		ids = append(ids, p.ReportID)
	}
	return ids
}

func sameIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestReliableAlertsSwitchedOff(t *testing.T) {
	offRow := &sqlcgen.GetAlertSettingsRow{ReliableAlertsOn: false, AlertRadiusKm: 5}
	cases := []struct {
		name     string
		settings *sqlcgen.GetAlertSettingsRow
		homeCell *string
		nilRead  bool
	}{
		{"no settings row", nil, cellPtr("tdr1v"), false},
		{"switch off", offRow, cellPtr("tdr1v"), false},
		{"switch on but no stamp", &sqlcgen.GetAlertSettingsRow{ReliableAlertsOn: true, AlertRadiusKm: 5}, cellPtr("tdr1v"), false},
		{"no home cell", alertOn(5), nil, false},
		{"unreadable home cell", alertOn(5), cellPtr("TDR1V"), false},
		{"nil nearby reader", alertOn(5), cellPtr("tdr1v"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &alertFake{fakeUpdatesQuerier: &fakeUpdatesQuerier{}, settings: c.settings, homeCell: c.homeCell}
			n := &fakeNearby{views: []ReportView{alertView(1, VisibilityLive, ReporterReliable, time.Hour)}}
			var reader NearbyReader = n
			if c.nilRead {
				reader = nil
			}
			if err := newAlertService(f, reader).Refresh(context.Background(), 7); err != nil {
				t.Fatalf("Refresh: %v", err)
			}
			if len(n.queries) != 0 {
				t.Errorf("Nearby was called %d times", len(n.queries))
			}
			if len(f.inserted) != 0 || len(f.windows) != 0 {
				t.Errorf("writes or window reads happened: %v %v", f.inserted, f.windows)
			}
			if len(f.named("ReporterAccountsForReports")) != 0 {
				t.Error("the reporter read ran for a step that does nothing")
			}
		})
	}
	// The bell keeps working: the first pass steps still ran.
	f := &alertFake{fakeUpdatesQuerier: &fakeUpdatesQuerier{}}
	if err := newAlertService(f, &fakeNearby{}).Refresh(context.Background(), 7); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(f.named("PruneUpdates")) != 1 || len(f.named("FollowedOpenReports")) != 1 {
		t.Errorf("the other steps did not run: %v", f.names())
	}
}

// TestReliableAlerts proves which reports become candidates and how the cap and
// the insert are called.
func TestReliableAlerts(t *testing.T) {
	const acct = 7
	young := 30 * time.Minute
	f := &alertFake{
		fakeUpdatesQuerier: &fakeUpdatesQuerier{},
		settings:           alertOn(5),
		homeCell:           cellPtr("tdr1v"),
	}
	hidden := alertView(3, VisibilityHidden, ReporterReliable, young)
	expired := alertView(4, VisibilityLive, ReporterReliable, young)
	expired.ExpiresAt = updatesNow.Add(-time.Minute)
	own := alertView(6, VisibilityLive, ReporterReliable, young)
	own.IsOwnReport = true
	follows := alertView(7, VisibilityLive, ReporterReliable, young)
	follows.ViewerFollows = true
	muted := alertView(8, VisibilityLive, ReporterReliable, young)
	muted.ViewerMuted = true
	before := alertView(9, VisibilityLive, ReporterReliable, 4*time.Hour) // before the switch went on
	noAccount := alertView(13, VisibilityLive, ReporterReliable, 10*time.Minute) // no rated account, so it cannot be capped

	n := &fakeNearby{views: []ReportView{
		// Provisional low report by a Reliable reporter: the note comes at posting.
		alertView(1, VisibilityProvisional, ReporterReliable, 40*time.Minute),
		// Critical report nobody confirmed: Live through the bypass.
		alertView(2, VisibilityLive, ReporterReliable, 20*time.Minute),
		hidden,
		expired,
		alertView(5, VisibilityLive, ReporterMixed, young),
		own, follows, muted, before,
		alertView(10, VisibilityWithheld, ReporterReliable, young),
		alertView(11, VisibilityRetracted, ReporterReliable, young),
		alertView(12, VisibilityLive, ReporterNew, young),
		noAccount,
	}}
	f.reporters = reporterRows(1, 101, 2, 102, 3, 103, 4, 104, 5, 105, 6, 106, 7, 107, 8, 108, 9, 109, 10, 110, 11, 111, 12, 112)

	if err := newAlertService(f, n).Refresh(context.Background(), acct); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if got, want := insertedReportIDs(f), []int64{1, 2}; !sameIDs(got, want) {
		t.Fatalf("alerts written for reports %v, want %v", got, want)
	}
	// One reporter read over the candidates only, oldest first.
	reads := f.named("ReporterAccountsForReports")
	if len(reads) != 1 {
		t.Fatalf("ReporterAccountsForReports calls = %d, want 1", len(reads))
	}
	if ids := reads[0].Params.([]int64); !sameIDs(ids, []int64{1, 2, 13}) {
		t.Errorf("reporter read over %v, want the candidates [1 2 13]", ids)
	}
	for i, p := range f.inserted {
		if p.AccountID != acct || !p.Now.Equal(updatesNow) || !p.HourEdge.Equal(updatesNow.Add(-time.Hour)) || p.Ceiling != UpdatesHourlyCeiling {
			t.Errorf("insert %d params = %+v", i, p)
		}
	}
	if len(f.windows) != 2 {
		t.Fatalf("window reads = %d, want 2", len(f.windows))
	}
	for i, w := range f.windows {
		wantReporter := int64(101 + i)
		if w.RecipientID != acct || w.ReporterID != wantReporter || !w.WindowStart.Equal(updatesNow.Add(-time.Hour)) {
			t.Errorf("window read %d = %+v, want reporter %d from one hour ago", i, w, wantReporter)
		}
	}
}

func TestReliableAlertsCapPerReporter(t *testing.T) {
	const acct = 7
	young := 30 * time.Minute

	t.Run("a reporter already alerted inside the hour is skipped", func(t *testing.T) {
		f := &alertFake{
			fakeUpdatesQuerier: &fakeUpdatesQuerier{},
			settings:           alertOn(5),
			homeCell:           cellPtr("tdr1v"),
			inWindow:           map[int64]int64{101: ReliableAlertsPerReporterPerHour},
		}
		f.reporters = reporterRows(1, 101, 2, 102)
		n := &fakeNearby{views: []ReportView{
			alertView(1, VisibilityLive, ReporterReliable, young),
			alertView(2, VisibilityLive, ReporterReliable, young),
		}}
		if err := newAlertService(f, n).Refresh(context.Background(), acct); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		if got := insertedReportIDs(f); !sameIDs(got, []int64{2}) {
			t.Errorf("alerts written for %v, want only report 2", got)
		}
	})

	t.Run("two reports of one reporter in one pass write one alert", func(t *testing.T) {
		f := &alertFake{fakeUpdatesQuerier: &fakeUpdatesQuerier{}, settings: alertOn(5), homeCell: cellPtr("tdr1v")}
		f.reporters = reporterRows(1, 101, 2, 101)
		// The newer report is listed first, as the nearest first read may do.
		n := &fakeNearby{views: []ReportView{
			alertView(2, VisibilityLive, ReporterReliable, 10*time.Minute),
			alertView(1, VisibilityLive, ReporterReliable, 50*time.Minute),
		}}
		if err := newAlertService(f, n).Refresh(context.Background(), acct); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		if got := insertedReportIDs(f); !sameIDs(got, []int64{1}) {
			t.Errorf("alerts written for %v, want only the older report 1", got)
		}
	})

	t.Run("a duplicate or a ceiling answer is normal and does not use up the reporter", func(t *testing.T) {
		f := &alertFake{
			fakeUpdatesQuerier: &fakeUpdatesQuerier{},
			settings:           alertOn(5),
			homeCell:           cellPtr("tdr1v"),
			insertErr:          pgx.ErrNoRows,
		}
		f.reporters = reporterRows(1, 101, 2, 101)
		n := &fakeNearby{views: []ReportView{
			alertView(1, VisibilityLive, ReporterReliable, 50*time.Minute),
			alertView(2, VisibilityLive, ReporterReliable, 10*time.Minute),
		}}
		if err := newAlertService(f, n).Refresh(context.Background(), acct); err != nil {
			t.Fatalf("Refresh returned an error for pgx.ErrNoRows: %v", err)
		}
		if len(f.inserted) != 2 {
			t.Errorf("insert attempts = %d, want 2 (nothing was written, so the reporter is not used up)", len(f.inserted))
		}
	})

	t.Run("other errors are wrapped", func(t *testing.T) {
		f := &alertFake{
			fakeUpdatesQuerier: &fakeUpdatesQuerier{},
			settings:           alertOn(5),
			homeCell:           cellPtr("tdr1v"),
			insertErr:          errors.New("boom"),
		}
		f.reporters = reporterRows(1, 101)
		n := &fakeNearby{views: []ReportView{alertView(1, VisibilityLive, ReporterReliable, young)}}
		err := newAlertService(f, n).Refresh(context.Background(), acct)
		if err == nil || !strings.Contains(err.Error(), "service: Refresh: reliable alerts: ") {
			t.Errorf("error = %v, want the reliable alerts wrapper", err)
		}
		g := &alertFake{fakeUpdatesQuerier: &fakeUpdatesQuerier{}, settings: alertOn(5), homeCell: cellPtr("tdr1v")}
		bad := &fakeNearby{err: errors.New("nearby is down")}
		err = newAlertService(g, bad).Refresh(context.Background(), acct)
		if err == nil || !strings.Contains(err.Error(), "service: Refresh: reliable alerts: ") {
			t.Errorf("nearby failure error = %v, want the reliable alerts wrapper", err)
		}
	})
}

// TestAlertAnchorIsHomeCell: the read is anchored at the middle of the stored
// home cell with the chosen distance, for the reading account.
func TestAlertAnchorIsHomeCell(t *testing.T) {
	f := &alertFake{fakeUpdatesQuerier: &fakeUpdatesQuerier{}, settings: alertOn(2), homeCell: cellPtr("tdr1v")}
	n := &fakeNearby{}
	if err := newAlertService(f, n).Refresh(context.Background(), 7); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(n.queries) != 1 {
		t.Fatalf("Nearby calls = %d, want 1", len(n.queries))
	}
	q := n.queries[0]
	wantLat, wantLon := geohash.DecodeCenter("tdr1v")
	if q.Latitude != wantLat || q.Longitude != wantLon {
		t.Errorf("anchor = %v, %v, want the middle of tdr1v %v, %v", q.Latitude, q.Longitude, wantLat, wantLon)
	}
	if q.RadiusKm != 2 || q.ViewerAccountID != 7 {
		t.Errorf("query = %+v, want radius 2 and viewer 7", q)
	}
	if q.All || q.IncludeDisputed || q.Category != "" {
		t.Errorf("query asks for more than the default listable reports: %+v", q)
	}
	if len(f.inserted) != 0 {
		t.Error("an empty answer wrote an alert")
	}
}

// TestReliableAlertsSource keeps the alert step on the one gate and the one
// decoder, and away from the resolver and from timers.
func TestReliableAlertsSource(t *testing.T) {
	src := mustReadFile(t, "updatesalerts.go")
	for _, want := range []string{"MayNotifyAboutReport(UpdateReliableNearby", "HomeAreaAnchor(", "ReliableAlertsPerReporterPerHour"} {
		if !strings.Contains(src, want) {
			t.Errorf("updatesalerts.go does not contain %q", want)
		}
	}
	for _, banned := range []string{" Resolve(", "(Resolve(", "KeptByReliableFilter(", "time.NewTicker", "go func", "DecodeCenter("} {
		if strings.Contains(src, banned) {
			t.Errorf("updatesalerts.go contains %q", banned)
		}
	}
	if strings.ContainsRune(src, rune(0x2013)) || strings.ContainsRune(src, rune(0x2014)) {
		t.Error("updatesalerts.go holds a dash character")
	}
}

// ---- A real database ----

// TestReliableAlertsOnRealHistory runs the alert end to end on a real database
// with a seeded Reliable reporter. The reports are inserted with the test's own
// clock, because SubmitChecked stamps wall time. The test clock starts two
// hours before wall time so every step stays before the database's own now().
func TestReliableAlertsOnRealHistory(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	cur := base
	clock := func() time.Time { return cur }

	reports := NewReportService(q, WithReportClock(clock))
	updates := NewUpdatesService(q, reports, WithUpdatesClock(clock))
	accounts := NewAccountService(q, WithAccountClock(clock))

	const reporterSession = "p32-13-reliable-session"
	testutil.SeedReliableReporter(t, pool, "p32-13-reliable@example.com", reporterSession, base)
	recipient := testutil.SeedReporterAccount(t, pool, "p32-13-recipient@example.com")

	// The home area: the cell is computed from the picked point, which is then
	// forgotten. Reports are placed at the middle of that cell.
	const homeLat, homeLon = 12.9716, 77.5946
	if _, err := accounts.UpdateProfile(ctx, recipient, ProfileUpdate{HomeArea: HomeAreaChange{
		Action: HomeAreaSet,
		Input:  HomeAreaInput{Name: "Bengaluru", Lat: homeLat, Lon: homeLon},
	}}); err != nil {
		t.Fatalf("setting the home area: %v", err)
	}
	cell, ok, err := accounts.HomeAreaCell(ctx, recipient)
	if err != nil || !ok {
		t.Fatalf("HomeAreaCell = %q, %v, %v", cell, ok, err)
	}
	spotLat, spotLon, _ := HomeAreaAnchor(cell)

	insertReport := func(severity Severity, createdAt time.Time) int64 {
		t.Helper()
		var id int64
		err := pool.QueryRow(ctx,
			`INSERT INTO reports
				(session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at)
			 VALUES ($1, 'flood', $2, 'p32-13 report', $3, $4, $5, $6, $7)
			 RETURNING id`,
			reporterSession, string(severity), spotLat, spotLon,
			geohash.EncodeWithPrecision(spotLat, spotLon, 8), createdAt, createdAt.Add(ExpiryDuration(severity)),
		).Scan(&id)
		if err != nil {
			t.Fatalf("inserting a report: %v", err)
		}
		return id
	}
	alertCount := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND kind = 'reliable_nearby'`, recipient).Scan(&n); err != nil {
			t.Fatalf("counting alerts: %v", err)
		}
		return n
	}
	alertsFor := func(reportID int64) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND kind = 'reliable_nearby' AND report_id = $2`, recipient, reportID).Scan(&n); err != nil {
			t.Fatalf("counting alerts for a report: %v", err)
		}
		return n
	}
	itemFor := func(reportID int64) (UpdateItem, bool) {
		t.Helper()
		page, err := updates.List(ctx, recipient, ActivityUpdatesListSize)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, it := range page.Items {
			if it.ReportID != nil && *it.ReportID == reportID {
				return it, true
			}
		}
		return UpdateItem{}, false
	}
	refreshTwice := func() {
		t.Helper()
		for i := 0; i < 2; i++ {
			if err := updates.Refresh(ctx, recipient); err != nil {
				t.Fatalf("Refresh %d: %v", i, err)
			}
		}
	}

	// A report posted before the switch was turned on never alerts.
	cur = base.Add(-10 * time.Minute)
	earlier := insertReport(SeverityLow, base.Add(-30*time.Minute))

	// Turn the switch on at base.
	cur = base
	if _, err := accounts.UpdateAlertSettings(ctx, recipient, AlertSettingsUpdate{ReliableAlerts: boolPtrForAlerts(true)}); err != nil {
		t.Fatalf("turning the alert on: %v", err)
	}

	// A critical report nobody confirmed: it alerts once, from posting.
	critical := insertReport(SeverityCritical, base.Add(5*time.Minute))
	cur = base.Add(10 * time.Minute)
	refreshTwice()
	if got := alertCount(); got != 1 {
		t.Fatalf("alerts after two refreshes = %d, want exactly 1", got)
	}
	if alertsFor(earlier) != 0 {
		t.Error("a report posted before the switch went on alerted")
	}
	it, found := itemFor(critical)
	if !found {
		t.Fatalf("the critical report has no row in the list")
	}
	if it.Kind != UpdateReliableNearby || it.Event != "Reported nearby by a Reliable reporter." {
		t.Errorf("row = kind %q event %q", it.Kind, it.Event)
	}
	if it.Trust != UnconfirmedLine {
		t.Errorf("trust line = %q, want %q", it.Trust, UnconfirmedLine)
	}

	// A second report by the same reporter inside the hour writes nothing.
	low := insertReport(SeverityLow, base.Add(20*time.Minute))
	cur = base.Add(30 * time.Minute)
	refreshTwice()
	if got := alertCount(); got != 1 {
		t.Fatalf("alerts inside the hour = %d, want still 1", got)
	}

	// Past the hour it alerts, at posting, while it is Provisional.
	cur = base.Add(75 * time.Minute)
	refreshTwice()
	if got := alertCount(); got != 2 {
		t.Fatalf("alerts after the hour = %d, want 2", got)
	}
	if alertsFor(low) != 1 || alertsFor(critical) != 1 {
		t.Errorf("alerts per report: low %d critical %d, want 1 and 1", alertsFor(low), alertsFor(critical))
	}
	it, found = itemFor(low)
	if !found || it.Trust != UnconfirmedLine {
		t.Fatalf("low report row = %+v (found %v), want the Not confirmed yet line", it, found)
	}

	// Two people confirm it from two places: the same row now reads the
	// confirmed sentence, and no second row appears.
	voterA := testutil.SeedReporterAccount(t, pool, "p32-13-voter-a@example.com")
	voterB := testutil.SeedReporterAccount(t, pool, "p32-13-voter-b@example.com")
	for _, v := range []struct {
		account int64
		cell    string
	}{{voterA, "tdr1v8y"}, {voterB, "tdr1vc3"}} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
			 VALUES ($1, $2, 'content', 'confirm', $3, $4)`,
			low, v.account, v.cell, cur); err != nil {
			t.Fatalf("seeding a confirming vote: %v", err)
		}
	}
	cur = base.Add(80 * time.Minute)
	refreshTwice()
	it, found = itemFor(low)
	if !found {
		t.Fatal("the low report lost its row")
	}
	if it.Trust == UnconfirmedLine || it.Trust == "" {
		t.Errorf("trust line after two confirmations = %q, want the confirmed sentence", it.Trust)
	}
	if alertsFor(low) != 1 {
		t.Errorf("alerts for the low report = %d, want 1 (the words change, the row does not)", alertsFor(low))
	}
}

func boolPtrForAlerts(b bool) *bool { return &b }
