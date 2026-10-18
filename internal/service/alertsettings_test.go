package service_test

// alertsettings_test.go proves the Reliable reporter alert settings of plan
// 03.2-13 (NOTIF-05, D-26): the home area anchor, the defaults, the closed
// distance list, the refusal to turn the alert on without a home area, and the
// alert going off when the home area is cleared. A fake querier stands in for
// the database, so none of this needs one.

import (
	"context"
	"errors"
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

	upserts  []sqlcgen.UpsertAlertSettingsParams
	disables []sqlcgen.DisableReliableAlertsParams
	profiles []sqlcgen.UpdateAccountProfileParams
}

func (f *fakeProfileQ) AcceptTerms(context.Context, sqlcgen.AcceptTermsParams) error { return nil }
func (f *fakeProfileQ) MarkFirstRunDone(context.Context, int64) error                  { return nil }

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
