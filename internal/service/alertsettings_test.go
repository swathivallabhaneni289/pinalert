package service_test

// alertsettings_test.go proves the Reliable reporter alert settings of plan
// 03.2-13 (NOTIF-05, D-26): the home area anchor, the defaults, the closed
// distance list, the refusal to turn the alert on without a home area, and the
// alert going off when the home area is cleared. A fake querier stands in for
// the database, so none of this needs one.

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

func TestAlertSettingsDefaults(t *testing.T) {
	cell := "tdr1v"
	f := &fakeProfileQ{homeCell: &cell}
	got, err := newAlertAccounts(f).AlertSettings(context.Background(), 7)
	if err != nil {
		t.Fatalf("AlertSettings: %v", err)
	}
	if got.ReliableAlertsOn || got.AlertRadiusKm != service.DefaultAlertRadiusKm || got.EnabledAt != nil || !got.HasHomeArea {
		t.Errorf("defaults = %+v, want off, 5 km, no EnabledAt, home area true", got)
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
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

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
	})
	t.Run("on without a home area", func(t *testing.T) {
		f := &fakeProfileQ{}
		_, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{ReliableAlerts: boolPtr(true)})
		wantValidation(t, err, "reliable_alerts", service.AlertsNeedHomeAreaMessage)
		if len(f.upserts) != 0 {
			t.Error("a refused update wrote")
		}
		if service.AlertsNeedHomeAreaMessage != "Add a home area first." {
			t.Errorf("AlertsNeedHomeAreaMessage = %q", service.AlertsNeedHomeAreaMessage)
		}
	})
	t.Run("a stored but unreadable cell is no home area", func(t *testing.T) {
		bad := "TDR1V"
		f := &fakeProfileQ{homeCell: &bad}
		_, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{ReliableAlerts: boolPtr(true)})
		wantValidation(t, err, "reliable_alerts", service.AlertsNeedHomeAreaMessage)
	})
	t.Run("off without a home area is allowed", func(t *testing.T) {
		f := &fakeProfileQ{}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{ReliableAlerts: boolPtr(false)})
		if err != nil || got.ReliableAlertsOn {
			t.Errorf("turning off: %+v, %v", got, err)
		}
	})
}

func TestUpdateAlertSettingsMergesAndStamps(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"
	f := &fakeProfileQ{homeCell: &cell}
	svc := newAlertAccounts(f)

	on, err := svc.UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{ReliableAlerts: boolPtr(true)})
	if err != nil {
		t.Fatalf("turning on: %v", err)
	}
	if !on.ReliableAlertsOn || on.AlertRadiusKm != 5 || on.EnabledAt == nil || !on.EnabledAt.Equal(alertsNow) || !on.HasHomeArea {
		t.Errorf("after turning on = %+v, want on, 5 km, EnabledAt now, home area true", on)
	}
	if len(f.upserts) != 1 || f.upserts[0].AccountID != 7 || !f.upserts[0].Now.Equal(alertsNow) {
		t.Errorf("upsert params = %+v", f.upserts)
	}

	// A radius change alone keeps the stored switch and the stored stamp.
	later := newAlertAccounts(f)
	r, err := later.UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{AlertRadiusKm: intPtr(10)})
	if err != nil {
		t.Fatalf("radius change: %v", err)
	}
	if !r.ReliableAlertsOn || r.AlertRadiusKm != 10 || r.EnabledAt == nil || !r.EnabledAt.Equal(alertsNow) {
		t.Errorf("after the radius change = %+v, want on, 10 km, same stamp", r)
	}
	if got := f.upserts[1]; !got.ReliableAlertsOn || got.AlertRadiusKm != 10 {
		t.Errorf("second upsert = %+v", got)
	}

	// A radius change on an account that never turned it on stays off.
	g := &fakeProfileQ{homeCell: &cell}
	off, err := newAlertAccounts(g).UpdateAlertSettings(ctx, 8, service.AlertSettingsUpdate{AlertRadiusKm: intPtr(2)})
	if err != nil || off.ReliableAlertsOn || off.AlertRadiusKm != 2 || off.EnabledAt != nil {
		t.Errorf("radius only on a fresh account = %+v, %v; want off, 2 km", off, err)
	}
}

func TestClearingHomeAreaTurnsAlertsOff(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"

	t.Run("clear", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell}
		_, err := newAlertAccounts(f).UpdateProfile(ctx, 7, service.ProfileUpdate{HomeArea: service.HomeAreaChange{Action: service.HomeAreaClear}})
		if err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if len(f.disables) != 1 || f.disables[0].AccountID != 7 || !f.disables[0].Now.Equal(alertsNow) {
			t.Errorf("DisableReliableAlerts calls = %+v, want one for account 7 at the clock time", f.disables)
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

// The tests below belong to plan 03.3-12 (PWA-03, PWA-06, decision 12): the
// Critical and Changes phone switches that sit next to 3.2's Reliable switch.

// pushRow builds the stored push switch row the fake answers.
func pushRow(critical, changes bool) *sqlcgen.GetPushKindSettingsRow {
	return &sqlcgen.GetPushKindSettingsRow{CriticalPushOn: critical, ChangesPushOn: changes}
}

func TestAlertSettingsPushKindDefaults(t *testing.T) {
	cell := "tdr1v"
	f := &fakeProfileQ{homeCell: &cell}
	got, err := newAlertAccounts(f).AlertSettings(context.Background(), 7)
	if err != nil {
		t.Fatalf("AlertSettings: %v", err)
	}
	if !got.CriticalPushOn || got.ChangesPushOn {
		t.Errorf("no settings row: Critical %v, Changes %v; want Critical on, Changes off", got.CriticalPushOn, got.ChangesPushOn)
	}
	if got.ReliableAlertsOn || got.AlertRadiusKm != service.DefaultAlertRadiusKm || !got.HasHomeArea {
		t.Errorf("3.2 values changed: %+v", got)
	}

	// Stored values read back as stored.
	f.push = pushRow(false, true)
	got, err = newAlertAccounts(f).AlertSettings(context.Background(), 7)
	if err != nil || got.CriticalPushOn || !got.ChangesPushOn {
		t.Errorf("stored values: %+v, %v; want Critical off, Changes on", got, err)
	}
}

func TestUpdateAlertSettingsPushKinds(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"

	t.Run("critical off alone writes only the push row", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell, push: pushRow(true, true)}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{CriticalPush: boolPtr(false)})
		if err != nil {
			t.Fatalf("UpdateAlertSettings: %v", err)
		}
		if len(f.upserts) != 0 {
			t.Errorf("a push only edit wrote 3.2's row: %+v", f.upserts)
		}
		if len(f.pushUpserts) != 1 {
			t.Fatalf("push upserts = %+v, want one", f.pushUpserts)
		}
		if p := f.pushUpserts[0]; p.AccountID != 7 || p.CriticalPushOn || !p.ChangesPushOn || !p.Now.Equal(alertsNow) {
			t.Errorf("push upsert = %+v, want account 7, Critical off, stored Changes kept on, clock time", p)
		}
		if got.CriticalPushOn || !got.ChangesPushOn || got.ReliableAlertsOn || got.AlertRadiusKm != service.DefaultAlertRadiusKm {
			t.Errorf("answer = %+v, want all four values as stored", got)
		}
	})
	t.Run("changes on needs no home area", func(t *testing.T) {
		f := &fakeProfileQ{}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{ChangesPush: boolPtr(true)})
		if err != nil {
			t.Fatalf("UpdateAlertSettings: %v", err)
		}
		if !got.ChangesPushOn || !got.CriticalPushOn || got.HasHomeArea {
			t.Errorf("answer = %+v, want Changes on, Critical at its default, no home area", got)
		}
		if len(f.pushUpserts) != 1 || !f.pushUpserts[0].CriticalPushOn || !f.pushUpserts[0].ChangesPushOn {
			t.Errorf("push upserts = %+v, want one with the default Critical kept and Changes on", f.pushUpserts)
		}
	})
	t.Run("critical on without a home area is refused", func(t *testing.T) {
		f := &fakeProfileQ{push: pushRow(false, false)}
		_, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{CriticalPush: boolPtr(true)})
		wantValidation(t, err, "critical_push", service.AlertsNeedHomeAreaMessage)
		if len(f.pushUpserts) != 0 || len(f.upserts) != 0 {
			t.Error("a refused update wrote")
		}
	})
	t.Run("critical on with a home area is saved", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell, push: pushRow(false, false)}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{CriticalPush: boolPtr(true)})
		if err != nil || !got.CriticalPushOn {
			t.Errorf("turning on: %+v, %v", got, err)
		}
	})
	t.Run("critical off without a home area is allowed", func(t *testing.T) {
		f := &fakeProfileQ{}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{CriticalPush: boolPtr(false)})
		if err != nil || got.CriticalPushOn {
			t.Errorf("turning off: %+v, %v", got, err)
		}
	})
	t.Run("a refused field blocks the whole edit", func(t *testing.T) {
		f := &fakeProfileQ{}
		_, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{CriticalPush: boolPtr(true), ChangesPush: boolPtr(true), AlertRadiusKm: intPtr(2)})
		wantValidation(t, err, "critical_push", service.AlertsNeedHomeAreaMessage)
		if len(f.pushUpserts) != 0 || len(f.upserts) != 0 {
			t.Error("a refused update wrote one of its parts")
		}
	})
	t.Run("a mixed edit writes both rows", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{ReliableAlerts: boolPtr(true), ChangesPush: boolPtr(true)})
		if err != nil {
			t.Fatalf("UpdateAlertSettings: %v", err)
		}
		if len(f.upserts) != 1 || len(f.pushUpserts) != 1 {
			t.Errorf("writes: %d and %d, want one each", len(f.upserts), len(f.pushUpserts))
		}
		if !got.ReliableAlertsOn || !got.ChangesPushOn || !got.CriticalPushOn || got.EnabledAt == nil {
			t.Errorf("answer = %+v", got)
		}
	})
	t.Run("a 3.2 only edit leaves the push row alone", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell, push: pushRow(false, true)}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{AlertRadiusKm: intPtr(10)})
		if err != nil {
			t.Fatalf("UpdateAlertSettings: %v", err)
		}
		if len(f.pushUpserts) != 0 {
			t.Errorf("a radius edit wrote the push row: %+v", f.pushUpserts)
		}
		if got.CriticalPushOn || !got.ChangesPushOn || got.AlertRadiusKm != 10 {
			t.Errorf("answer = %+v, want stored push values and 10 km", got)
		}
	})
	t.Run("nothing to save still holds", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell}
		_, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{})
		wantValidation(t, err, "body", service.AlertsNothingToSaveMessage)
	})
}

// TestCriticalDefaultMatchesSQL pins decision 12: an account with no settings
// row is Critical on and Changes off, and the SQL and the Go constants say the
// same thing. If one moves without the other this fails.
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
	// The service default for an account with no row is the Go constant.
	f := &fakeProfileQ{}
	got, err := newAlertAccounts(f).AlertSettings(context.Background(), 7)
	if err != nil || got.CriticalPushOn != service.DefaultCriticalPushOn || got.ChangesPushOn != service.DefaultChangesPushOn {
		t.Errorf("AlertSettings with no row = %+v, %v; want the Go defaults", got, err)
	}
}

// TestClearingHomeAreaKeepsCritical pins D-21 and UI-SPEC 2b: removing the home
// area turns 3.2's Reliable switch off but leaves the stored Critical value, so
// setting an area again brings Critical back as it was.
func TestClearingHomeAreaKeepsCritical(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"
	f := &fakeProfileQ{homeCell: &cell, push: pushRow(true, true)}
	svc := newAlertAccounts(f)
	_, err := svc.UpdateProfile(ctx, 7, service.ProfileUpdate{HomeArea: service.HomeAreaChange{Action: service.HomeAreaClear}})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if len(f.disables) != 1 {
		t.Errorf("DisableReliableAlerts calls = %d, want 1 (3.2 still turns Reliable off)", len(f.disables))
	}
	if len(f.pushUpserts) != 0 {
		t.Errorf("clearing the area wrote the push row: %+v", f.pushUpserts)
	}
	got, err := svc.AlertSettings(ctx, 7)
	if err != nil {
		t.Fatalf("AlertSettings: %v", err)
	}
	if !got.CriticalPushOn || !got.ChangesPushOn || got.HasHomeArea {
		t.Errorf("after clearing = %+v, want Critical and Changes kept, no home area", got)
	}
}

// Plan 05-14 (D-11): the switch "Severe official alerts for my area". The fake
// holds the stored value in official; nil means the account has no stored value,
// which the real query answers with DefaultOfficialPushOn.

func TestAlertSettingsOfficialDefault(t *testing.T) {
	cell := "tdr1v"
	f := &fakeProfileQ{homeCell: &cell}
	got, err := newAlertAccounts(f).AlertSettings(context.Background(), 7)
	if err != nil {
		t.Fatalf("AlertSettings: %v", err)
	}
	if !got.OfficialPushOn || got.OfficialPushOn != service.DefaultOfficialPushOn {
		t.Errorf("no stored value: OfficialPushOn %v, want true (the default)", got.OfficialPushOn)
	}
	off := false
	f.official = &off
	got, err = newAlertAccounts(f).AlertSettings(context.Background(), 7)
	if err != nil || got.OfficialPushOn {
		t.Errorf("stored off: %+v, %v; want OfficialPushOn false", got, err)
	}
}

func TestUpdateAlertSettingsOfficial(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"

	t.Run("official off alone writes only the official row", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell, push: pushRow(true, true)}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{OfficialPush: boolPtr(false)})
		if err != nil {
			t.Fatalf("UpdateAlertSettings: %v", err)
		}
		if len(f.upserts) != 0 || len(f.pushUpserts) != 0 {
			t.Errorf("an official only edit wrote 3.2's or 3.3's row: %+v %+v", f.upserts, f.pushUpserts)
		}
		if len(f.officialUpserts) != 1 {
			t.Fatalf("official upserts = %+v, want one", f.officialUpserts)
		}
		if p := f.officialUpserts[0]; p.AccountID != 7 || p.OfficialPushOn || !p.Now.Equal(alertsNow) {
			t.Errorf("official upsert = %+v, want account 7, off, clock time", p)
		}
		if got.OfficialPushOn || !got.CriticalPushOn || !got.ChangesPushOn || got.ReliableAlertsOn || got.AlertRadiusKm != service.DefaultAlertRadiusKm {
			t.Errorf("answer = %+v, want all five values as stored", got)
		}
	})
	t.Run("official on without a home area is refused and writes nothing", func(t *testing.T) {
		off := false
		f := &fakeProfileQ{official: &off}
		_, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{OfficialPush: boolPtr(true), ChangesPush: boolPtr(true)})
		wantValidation(t, err, "official_push", service.AlertsNeedHomeAreaMessage)
		if len(f.officialUpserts) != 0 || len(f.pushUpserts) != 0 || len(f.upserts) != 0 {
			t.Error("a refused update wrote one of its parts")
		}
	})
	t.Run("official on with a home area is saved", func(t *testing.T) {
		off := false
		f := &fakeProfileQ{homeCell: &cell, official: &off}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{OfficialPush: boolPtr(true)})
		if err != nil || !got.OfficialPushOn || len(f.officialUpserts) != 1 {
			t.Errorf("turning on: %+v, %v, %d writes", got, err, len(f.officialUpserts))
		}
	})
	t.Run("official off without a home area is allowed", func(t *testing.T) {
		f := &fakeProfileQ{}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{OfficialPush: boolPtr(false)})
		if err != nil || got.OfficialPushOn || got.HasHomeArea {
			t.Errorf("turning off: %+v, %v", got, err)
		}
	})
	t.Run("a 3.3 only edit leaves the official row alone", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{ChangesPush: boolPtr(true)})
		if err != nil {
			t.Fatalf("UpdateAlertSettings: %v", err)
		}
		if len(f.officialUpserts) != 0 || !got.OfficialPushOn {
			t.Errorf("a Changes edit touched the official row: %+v, answer %+v", f.officialUpserts, got)
		}
	})
	t.Run("a mixed edit writes each row once", func(t *testing.T) {
		f := &fakeProfileQ{homeCell: &cell}
		got, err := newAlertAccounts(f).UpdateAlertSettings(ctx, 7, service.AlertSettingsUpdate{ReliableAlerts: boolPtr(true), CriticalPush: boolPtr(false), OfficialPush: boolPtr(false)})
		if err != nil {
			t.Fatalf("UpdateAlertSettings: %v", err)
		}
		if len(f.upserts) != 1 || len(f.pushUpserts) != 1 || len(f.officialUpserts) != 1 {
			t.Errorf("writes: %d, %d, %d; want one each", len(f.upserts), len(f.pushUpserts), len(f.officialUpserts))
		}
		if !got.ReliableAlertsOn || got.CriticalPushOn || got.OfficialPushOn {
			t.Errorf("answer = %+v", got)
		}
	})
}

// TestClearingHomeAreaKeepsOfficial: removing the home area leaves the stored
// official value as it was (D-11, the 3.3 plan 12 pattern), and setting an area
// again reads it back unchanged.
func TestClearingHomeAreaKeepsOfficial(t *testing.T) {
	ctx := context.Background()
	cell := "tdr1v"
	off := false
	f := &fakeProfileQ{homeCell: &cell, official: &off}
	svc := newAlertAccounts(f)
	if _, err := svc.UpdateProfile(ctx, 7, service.ProfileUpdate{HomeArea: service.HomeAreaChange{Action: service.HomeAreaClear}}); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if len(f.officialUpserts) != 0 {
		t.Errorf("clearing the area wrote the official row: %+v", f.officialUpserts)
	}
	got, err := svc.AlertSettings(ctx, 7)
	if err != nil || got.OfficialPushOn || got.HasHomeArea {
		t.Errorf("after clearing = %+v, %v; want official kept off, no home area", got, err)
	}

	on := true
	f2 := &fakeProfileQ{homeCell: &cell, official: &on}
	svc2 := newAlertAccounts(f2)
	if _, err := svc2.UpdateProfile(ctx, 7, service.ProfileUpdate{HomeArea: service.HomeAreaChange{Action: service.HomeAreaClear}}); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	got, err = svc2.AlertSettings(ctx, 7)
	if err != nil || !got.OfficialPushOn || got.HasHomeArea {
		t.Errorf("after clearing (on) = %+v, %v; want official kept on, no home area", got, err)
	}
	f2.homeCell = &cell
	got, err = svc2.AlertSettings(ctx, 7)
	if err != nil || !got.OfficialPushOn || !got.HasHomeArea {
		t.Errorf("after setting an area again = %+v, %v; want official on as it was", got, err)
	}
}
