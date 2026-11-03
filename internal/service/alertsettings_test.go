package service_test

// alertsettings_test.go proves the alert settings of plan 03.2-13 (NOTIF-05,
// D-26): the home area anchor, the defaults, the closed distance list, and the
// stored switch and stamp kept through a distance change (quick 261103-jal, the
// distance is the one setting left). A fake querier stands in for the database,
// so most of this needs none; one test reads the columns back from a real one.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// fakeProfileQ is an in memory ProfileQuerier. alert nil means the account has
// no alert settings row yet, which the real query reports as pgx.ErrNoRows.
type fakeProfileQ struct {
	alert    *sqlcgen.GetAlertSettingsRow
	homeCell *string
	profile  sqlcgen.GetAccountProfileRow
	// Amended 2026-10-18 by Phase 3.3 plan 12 (decision 12): the fake gained the
	// two push switch methods. push nil means the account has no push switch
	// values saved, which the real query answers with the defaults (Critical on,
	// Changes off).
	push *sqlcgen.GetPushKindSettingsRow
	// Amended 2026-10-29 by plan 05-14 (D-11): the fake gained the official
	// switch methods. official nil means no stored value, which the real query
	// answers with DefaultOfficialPushOn (on).
	official *bool

	upserts         []sqlcgen.UpsertAlertSettingsParams
	pushUpserts     []sqlcgen.UpsertPushKindSettingsParams
	officialUpserts []sqlcgen.UpsertOfficialPushSettingParams
	disables        []sqlcgen.DisableReliableAlertsParams
	profiles        []sqlcgen.UpdateAccountProfileParams

	// Amended 2026-10-29 by Phase 7.1 plan 06 (D-07): the fake gained the three
	// username methods. usernameSets records every SetAccountUsername call;
	// usernameErr is returned by it; usernameZeroRows makes it report no row
	// (too soon); usernameHolders maps a name to the account that holds it for
	// GetAccountIDByUsername; usernameRows is what UsernamesByAccountIDs
	// returns, and usernameReads counts those calls.
	usernameSets     []sqlcgen.SetAccountUsernameParams
	usernameErr      error
	usernameZeroRows bool
	usernameHolders  map[string]int64
	usernameRows     []sqlcgen.UsernamesByAccountIDsRow
	usernameReads    int
}

func (f *fakeProfileQ) SetAccountUsername(_ context.Context, a sqlcgen.SetAccountUsernameParams) (int64, error) {
	f.usernameSets = append(f.usernameSets, a)
	if f.usernameErr != nil {
		return 0, f.usernameErr
	}
	if f.usernameZeroRows {
		return 0, nil
	}
	name, screen := a.Username, a.UsernameScreen
	if f.profile.Username != nil && *f.profile.Username != name {
		f.profile.UsernameChangedAt = pgtype.Timestamptz{Time: a.Now, Valid: true}
	}
	f.profile.Username = &name
	f.profile.UsernameScreen = &screen
	return 1, nil
}

func (f *fakeProfileQ) GetAccountIDByUsername(_ context.Context, username string) (int64, error) {
	if id, ok := f.usernameHolders[username]; ok {
		return id, nil
	}
	return 0, pgx.ErrNoRows
}

func (f *fakeProfileQ) UsernamesByAccountIDs(context.Context, []int64) ([]sqlcgen.UsernamesByAccountIDsRow, error) {
	f.usernameReads++
	return f.usernameRows, nil
}

func (f *fakeProfileQ) GetOfficialPushSetting(context.Context, int64) (bool, error) {
	if f.official == nil {
		return true, nil
	}
	return *f.official, nil
}

func (f *fakeProfileQ) UpsertOfficialPushSetting(_ context.Context, a sqlcgen.UpsertOfficialPushSettingParams) (bool, error) {
	f.officialUpserts = append(f.officialUpserts, a)
	on := a.OfficialPushOn
	f.official = &on
	return on, nil
}

func (f *fakeProfileQ) GetPushKindSettings(context.Context, int64) (sqlcgen.GetPushKindSettingsRow, error) {
	if f.push == nil {
		return sqlcgen.GetPushKindSettingsRow{CriticalPushOn: true}, nil
	}
	return *f.push, nil
}

func (f *fakeProfileQ) UpsertPushKindSettings(_ context.Context, a sqlcgen.UpsertPushKindSettingsParams) (sqlcgen.UpsertPushKindSettingsRow, error) {
	f.pushUpserts = append(f.pushUpserts, a)
	f.push = &sqlcgen.GetPushKindSettingsRow{CriticalPushOn: a.CriticalPushOn, ChangesPushOn: a.ChangesPushOn}
	return sqlcgen.UpsertPushKindSettingsRow{CriticalPushOn: a.CriticalPushOn, ChangesPushOn: a.ChangesPushOn}, nil
}

func (f *fakeProfileQ) AcceptTerms(context.Context, sqlcgen.AcceptTermsParams) error { return nil }
func (f *fakeProfileQ) MarkFirstRunDone(context.Context, int64) error                { return nil }

func (f *fakeProfileQ) UpdateAccountProfile(_ context.Context, a sqlcgen.UpdateAccountProfileParams) error {
	f.profiles = append(f.profiles, a)
	if a.SetHomeArea {
		f.profile.HomeAreaName = a.HomeAreaName
		f.profile.HomeAreaCell = a.HomeAreaCell
		f.homeCell = a.HomeAreaCell
	}
	return nil
}

func (f *fakeProfileQ) GetAccountProfile(context.Context, int64) (sqlcgen.GetAccountProfileRow, error) {
	return f.profile, nil
}

func (f *fakeProfileQ) GetAccountHomeCell(context.Context, int64) (*string, error) {
	return f.homeCell, nil
}

func (f *fakeProfileQ) GetAlertSettings(context.Context, int64) (sqlcgen.GetAlertSettingsRow, error) {
	if f.alert == nil {
		return sqlcgen.GetAlertSettingsRow{}, pgx.ErrNoRows
	}
	return *f.alert, nil
}

func (f *fakeProfileQ) UpsertAlertSettings(_ context.Context, a sqlcgen.UpsertAlertSettingsParams) (sqlcgen.UpsertAlertSettingsRow, error) {
	f.upserts = append(f.upserts, a)
	row := sqlcgen.UpsertAlertSettingsRow{ReliableAlertsOn: a.ReliableAlertsOn, AlertRadiusKm: a.AlertRadiusKm}
	// The real statement stamps enabled_at when the switch goes on and keeps the
	// stored stamp while it stays on.
	switch {
	case !a.ReliableAlertsOn:
	case f.alert != nil && f.alert.ReliableAlertsOn && f.alert.EnabledAt.Valid:
		row.EnabledAt = f.alert.EnabledAt
	default:
		row.EnabledAt = pgtype.Timestamptz{Time: a.Now, Valid: true}
	}
	f.alert = &sqlcgen.GetAlertSettingsRow{ReliableAlertsOn: row.ReliableAlertsOn, AlertRadiusKm: row.AlertRadiusKm, EnabledAt: row.EnabledAt}
	return row, nil
}

func (f *fakeProfileQ) DisableReliableAlerts(_ context.Context, a sqlcgen.DisableReliableAlertsParams) error {
	f.disables = append(f.disables, a)
	return nil
}

var alertsNow = time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)

func newAlertAccounts(f *fakeProfileQ) *service.AccountService {
	return service.NewAccountService(f, service.WithAccountClock(func() time.Time { return alertsNow }))
}

func TestHomeAreaAnchor(t *testing.T) {
	lat, lon, ok := service.HomeAreaAnchor("tdr1v")
	wantLat, wantLon := geohash.DecodeCenter("tdr1v")
	if !ok || lat != wantLat || lon != wantLon {
		t.Errorf("HomeAreaAnchor(tdr1v) = %v, %v, %v; want %v, %v, true", lat, lon, ok, wantLat, wantLon)
	}
	for _, bad := range []string{"", "TDR1V", "tdr1", "tdr1v8", "tdr1a", "tdr1 ", "tdr1v\n"} {
		if _, _, ok := service.HomeAreaAnchor(bad); ok {
			t.Errorf("HomeAreaAnchor(%q) ok = true, want false", bad)
		}
	}
}

// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D7,
// D8, C10): the service now holds one alert setting, the distance. The Reliable,
// Critical, Changes and official switches left AlertSettings and
// AlertSettingsUpdate; their columns stay in the database for the SQL defaults
// and for the start point of the Reliable bell row. The tests of those switches
// are removed with them; the two tests that pinned a kept rule keep their names.
// ChangesPushOn stays on AlertSettings only for the changes pop-up that the same
// quick task removes.

func TestAlertSettingsDefaults(t *testing.T) {
	cell := "tdr1v"
	f := &fakeProfileQ{homeCell: &cell}
	got, err := newAlertAccounts(f).AlertSettings(context.Background(), 7)
	if err != nil {
		t.Fatalf("AlertSettings: %v", err)
	}
	if got.AlertRadiusKm != service.DefaultAlertRadiusKm || !got.HasHomeArea {
		t.Errorf("defaults = %+v, want 5 km and home area true", got)
	}
	if service.DefaultAlertRadiusKm != 5 {
		t.Errorf("DefaultAlertRadiusKm = %d, want 5", service.DefaultAlertRadiusKm)
	}
	f.homeCell = nil
	got, err = newAlertAccounts(f).AlertSettings(context.Background(), 7)
	if err != nil || got.HasHomeArea {
		t.Errorf("without a home cell: %+v, %v; want HasHomeArea false", got, err)
	}
	if want := []int{1, 2, 5, 10}; len(service.AlertRadii) != 4 || service.AlertRadii[0] != want[0] || service.AlertRadii[1] != want[1] || service.AlertRadii[2] != want[2] || service.AlertRadii[3] != want[3] {
		t.Errorf("AlertRadii = %v, want %v", service.AlertRadii, want)
	}
	// A stored distance reads back as stored.
	f.alert = &sqlcgen.GetAlertSettingsRow{AlertRadiusKm: 10}
	got, err = newAlertAccounts(f).AlertSettings(context.Background(), 7)
	if err != nil || got.AlertRadiusKm != 10 {
		t.Errorf("stored distance: %+v, %v; want 10 km", got, err)
	}
}

func intPtr(i int) *int { return &i }

func wantValidation(t *testing.T, err error, field, message string) {
	t.Helper()
	var ve service.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v, want a ValidationError on %q", err, field)
	}
	if ve.Field != field || ve.Message != message {
		t.Errorf("ValidationError = {%q, %q}, want {%q, %q}", ve.Field, ve.Message, field, message)
	}
}

func TestUpdateAlertSettingsValidates(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"

	t.Run("radius outside the list", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell}
		_, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{AlertRadiusKm: intPtr(3)})
		wantValidation(t, err, "alert_radius_km", service.AlertRadiusMessage)
		if len(f.upserts) != 0 {
			t.Error("a refused update wrote")
		}
	})
	t.Run("nothing to save", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell}
		_, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{})
		wantValidation(t, err, "body", service.AlertsNothingToSaveMessage)
		if len(f.upserts) != 0 {
			t.Error("a refused update wrote")
		}
	})
	t.Run("a distance is saved without a home area", func(t *testing.T) {
		f := &fakeProfileQ{}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{AlertRadiusKm: intPtr(2)})
		if err != nil || got.AlertRadiusKm != 2 || got.HasHomeArea {
			t.Errorf("saving without a home area: %+v, %v; want 2 km, no home area", got, err)
		}
	})
}

func TestUpdateAlertSettingsKeepsTheStoredSwitchAndStamp(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"

	// A fresh account: the distance is saved, the stored switch reads off and no
	// stamp is made.
	g := &fakeProfileQ{homeCell: &cell}
	fresh, err := newAlertAccounts(g).UpdateAlertSettings(ctx, 8, service.AlertSettingsUpdate{AlertRadiusKm: intPtr(2)})
	if err != nil || fresh.AlertRadiusKm != 2 || !fresh.HasHomeArea {
		t.Fatalf("radius only on a fresh account = %+v, %v; want 2 km and a home area", fresh, err)
	}
	if len(g.upserts) != 1 || g.upserts[0].ReliableAlertsOn || g.upserts[0].AlertRadiusKm != 2 || g.upserts[0].AccountID != 8 || !g.upserts[0].Now.Equal(alertsNow) {
		t.Errorf("fresh account upsert = %+v, want switch off, 2 km, account 8, clock time", g.upserts)
	}

	// An account whose row says on with a stamp keeps both through a distance change.
	stamp := alertsNow.Add(-72 * time.Hour)
	f := &fakeProfileQ{homeCell: &cell, alert: &sqlcgen.GetAlertSettingsRow{
		ReliableAlertsOn: true, AlertRadiusKm: 5, EnabledAt: pgtype.Timestamptz{Time: stamp, Valid: true},
	}}
	got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{AlertRadiusKm: intPtr(10)})
	if err != nil || got.AlertRadiusKm != 10 {
		t.Fatalf("radius change: %+v, %v; want 10 km", got, err)
	}
	if len(f.upserts) != 1 || !f.upserts[0].ReliableAlertsOn || f.upserts[0].AlertRadiusKm != 10 {
		t.Errorf("upsert = %+v, want the stored switch passed through with 10 km", f.upserts)
	}
	if f.alert == nil || !f.alert.EnabledAt.Valid || !f.alert.EnabledAt.Time.Equal(stamp) {
		t.Errorf("stored row after the change = %+v, want the stamp %v kept", f.alert, stamp)
	}
}

// TestDistanceSaveKeepsTheStoredSwitchAndStampInTheDatabase reads the same rule
// back from a real database: the row is written by SQL with the old switch on and
// a stamp three hours back, a distance change goes through the service, and the
// columns read back as they were except the distance.
func TestDistanceSaveKeepsTheStoredSwitchAndStampInTheDatabase(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	svc := service.NewAccountService(q, service.WithAccountClock(func() time.Time { return time.Now().UTC() }))

	id := testutil.SeedReporterAccount(t, pool, "jal-keep@example.com")
	stamp := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Second)
	testutil.MustExec(t, pool,
		`INSERT INTO account_alert_settings (account_id, reliable_alerts_on, alert_radius_km, enabled_at, critical_push_on, changes_push_on, updated_at)
		 VALUES ($1, true, 5, $2, false, true, now())`, id, stamp)

	got, err := svc.UpdateAlertSettings(ctx, id, service.AlertSettingsUpdate{AlertRadiusKm: intPtr(2)})
	if err != nil || got.AlertRadiusKm != 2 {
		t.Fatalf("UpdateAlertSettings = %+v, %v; want 2 km", got, err)
	}
	var on, critical, changes bool
	var radius int
	var enabled *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT reliable_alerts_on, alert_radius_km, enabled_at, critical_push_on, changes_push_on FROM account_alert_settings WHERE account_id = $1`, id,
	).Scan(&on, &radius, &enabled, &critical, &changes); err != nil {
		t.Fatalf("reading the row back: %v", err)
	}
	if !on || radius != 2 || critical || !changes {
		t.Errorf("stored row = on %v, radius %d, critical %v, changes %v; want on, 2, off, on", on, radius, critical, changes)
	}
	if enabled == nil || !enabled.Equal(stamp) {
		t.Errorf("enabled_at = %v, want the seeded stamp %v kept", enabled, stamp)
	}
}

func TestClearingHomeAreaTurnsAlertsOff(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"

	// Amended 2026-10-29 by Phase 7.1 plan 06 (D-15): the clear subtest used to
	// assert that clearing the home area turns the Reliable alert off. The home
	// area can no longer be cleared through UpdateProfile, so the subtest now
	// asserts the refusal and that nothing was written or switched off. The set
	// and unchanged subtests below are unchanged.
	t.Run("clear", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell}
		_, err := newAlertAccounts(f).UpdateProfile(ctx, 7, service.ProfileUpdate{HomeArea: service.HomeAreaChange{Action: service.HomeAreaClear}})
		wantMessage(t, err, "home_area", service.HomeAreaRequiredMessage)
		if len(f.disables) != 0 || len(f.profiles) != 0 {
			t.Errorf("a refused clear wrote %d profiles and switched off %d alerts, want none", len(f.profiles), len(f.disables))
		}
	})
	t.Run("set", func(t *testing.T) {
		f := &fakeProfileQ{}
		_, err := newAlertAccounts(f).UpdateProfile(ctx, 7, service.ProfileUpdate{HomeArea: service.HomeAreaChange{
			Action: service.HomeAreaSet,
			Input:  service.HomeAreaInput{Name: "Indiranagar", Lat: 12.9784, Lon: 77.6408},
		}})
		if err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if len(f.disables) != 0 {
			t.Errorf("setting a home area turned the alert off: %+v", f.disables)
		}
	})
	t.Run("unchanged", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell}
		name := "Asha"
		_, err := newAlertAccounts(f).UpdateProfile(ctx, 7, service.ProfileUpdate{DisplayName: &name})
		if err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if len(f.disables) != 0 {
			t.Errorf("a name edit turned the alert off: %+v", f.disables)
		}
	})
}

func TestHomeAreaCellReadsTheStoredCell(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"
	f := &fakeProfileQ{homeCell: &cell}
	got, ok, err := newAlertAccounts(f).HomeAreaCell(ctx, 7)
	if err != nil || !ok || got != "tdr1v" {
		t.Errorf("HomeAreaCell = %q, %v, %v; want tdr1v, true", got, ok, err)
	}
	f.homeCell = nil
	got, ok, err = newAlertAccounts(f).HomeAreaCell(ctx, 7)
	if err != nil || ok || got != "" {
		t.Errorf("HomeAreaCell without a home area = %q, %v, %v; want empty, false", got, ok, err)
	}
}

// TestCriticalDefaultMatchesSQL pins decision 12: an account with no settings
// row is Critical on and Changes off, and the SQL and the Go constants say the
// same thing. If one moves without the other this fails. Amended 2026-11-03 by
// quick 261103-jal (D8): the defaults are kept for the SQL COALESCE only and no
// longer decide delivery, and the service no longer reads them for the settings.
func TestCriticalDefaultMatchesSQL(t *testing.T) {
	sql, err := os.ReadFile(filepath.Join("..", "store", "queries", "push.sql"))
	if err != nil {
		t.Fatalf("reading push.sql: %v", err)
	}
	for _, want := range []string{"COALESCE(s.critical_push_on, true)", "COALESCE(s.changes_push_on, false)"} {
		if !strings.Contains(string(sql), want) {
			t.Errorf("push.sql does not hold %q", want)
		}
	}
	if !service.DefaultCriticalPushOn || service.DefaultChangesPushOn {
		t.Errorf("Go defaults: Critical %v, Changes %v; want true and false", service.DefaultCriticalPushOn, service.DefaultChangesPushOn)
	}
}

// TestClearingHomeAreaRefusedWritesNothing pins D-21 of 3.3 as it stands after
// Phase 7.1 plan 06 (D-15): the home area cannot be cleared through
// UpdateProfile, so nothing is switched off and no settings row is written. It
// replaces TestClearingHomeAreaKeepsCritical and TestClearingHomeAreaKeepsOfficial
// (quick 261103-jal: those stored values left the service).
func TestClearingHomeAreaRefusedWritesNothing(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"
	f := &fakeProfileQ{homeCell: &cell, push: &sqlcgen.GetPushKindSettingsRow{CriticalPushOn: true, ChangesPushOn: true}}
	_, err := newAlertAccounts(f).UpdateProfile(ctx, 7, service.ProfileUpdate{HomeArea: service.HomeAreaChange{Action: service.HomeAreaClear}})
	wantMessage(t, err, "home_area", service.HomeAreaRequiredMessage)
	if len(f.disables) != 0 || len(f.pushUpserts) != 0 || len(f.officialUpserts) != 0 || len(f.upserts) != 0 {
		t.Errorf("a refused clear wrote: disables %d, push %d, official %d, alert %d; want none",
			len(f.disables), len(f.pushUpserts), len(f.officialUpserts), len(f.upserts))
	}
}
