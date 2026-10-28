// alertsettings.go holds the settings behind the Reliable reporter alert
// (NOTIF-05, D-26): one switch and one distance per account, and the home area
// anchor the distance is measured from. The alert itself is raised by
// refreshReliableAlerts (updatesalerts.go); nothing here raises anything.

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
)

// AlertRadii is the closed list of distances, in kilometres, a person can pick
// for the Reliable reporter alert (D-26). The database holds the same list in a
// CHECK constraint.
var AlertRadii = []int{1, 2, 5, 10}

// DefaultAlertRadiusKm is the distance an account has until it picks another.
const DefaultAlertRadiusKm = 5

// The plain sentences for the alert settings refusals.
const (
	// AlertRadiusMessage is the refusal for a distance outside AlertRadii.
	AlertRadiusMessage = "Choose 1, 2, 5 or 10 km."
	// AlertsNeedHomeAreaMessage is the refusal for turning the alert on while no
	// home area is saved (UI-SPEC E1).
	AlertsNeedHomeAreaMessage = "Add a home area first."
	// AlertsNothingToSaveMessage is the refusal for a body that names neither
	// the switch nor the distance.
	AlertsNothingToSaveMessage = "Nothing to save."
)

// AlertSettings is one account's alert switch and distance as stored, with
// whether a home area exists. EnabledAt is when the switch last went on, nil
// while it is off. It never carries the home area cell or any coordinate.
//
// CriticalPushOn and ChangesPushOn are the two phone switches of Phase 3.3
// (PWA-03, PWA-06) that sit next to the Reliable switch. The Reliable switch and
// the distance stay 3.2's and are not duplicated.
type AlertSettings struct {
	ReliableAlertsOn bool
	AlertRadiusKm    int
	EnabledAt        *time.Time
	HasHomeArea      bool
	CriticalPushOn   bool
	ChangesPushOn    bool
	// OfficialPushOn is Phase 5's switch "Severe official alerts for my area"
	// (D-11), on by default.
	OfficialPushOn bool
}

// AlertSettingsUpdate is one edit. A nil field leaves that part as stored.
type AlertSettingsUpdate struct {
	ReliableAlerts *bool
	AlertRadiusKm  *int
	CriticalPush   *bool
	ChangesPush    *bool
	OfficialPush   *bool
}

// HomeAreaAnchor decodes a stored home area cell into the point a distance is
// measured from. ok is false for anything that is not a lower case geohash of
// HomeAreaCellPrecision characters (the migration 00008 CHECK shape).
//
// Q5 owner confirmed 2026-10-18: distances are measured from the middle of the
// precision 5 home cell, which is about 4.3 to 4.9 km across; the picked point
// is never stored.
//
// This is the one decoder of a stored home area cell: HomeAreaCentre and the
// Reliable alert step both call it.
func HomeAreaAnchor(cell string) (lat, lon float64, ok bool) {
	if !homeAreaCellShape.MatchString(cell) {
		return 0, 0, false
	}
	lat, lon = geohash.DecodeCenter(cell)
	return lat, lon, true
}

// validAlertRadius reports whether km is one of AlertRadii.
func validAlertRadius(km int) bool {
	for _, r := range AlertRadii {
		if r == km {
			return true
		}
	}
	return false
}

// HomeAreaCell answers the account's stored home area cell, ok false when there
// is none (or the stored value is not a cell). Phase 3.3 reuses it. The cell is
// read by server code only and is never written into a page or a response.
func (s *AccountService) HomeAreaCell(ctx context.Context, accountID int64) (cell string, ok bool, err error) {
	stored, err := s.q.GetAccountHomeCell(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("service: reading home area cell: %w", err)
	}
	if stored == nil {
		return "", false, nil
	}
	if _, _, valid := HomeAreaAnchor(*stored); !valid {
		return "", false, nil
	}
	return *stored, true, nil
}

// AlertSettings reads the account's alert switch and distance. An account that
// never saved a choice reads off and DefaultAlertRadiusKm, with Critical on and
// Changes off (decision 12: an account with no row counts as Critical on, the
// same constant in the SQL COALESCE and in Go, pinned by
// TestCriticalDefaultMatchesSQL).
func (s *AccountService) AlertSettings(ctx context.Context, accountID int64) (AlertSettings, error) {
	out := AlertSettings{
		AlertRadiusKm:  DefaultAlertRadiusKm,
		CriticalPushOn: DefaultCriticalPushOn,
		ChangesPushOn:  DefaultChangesPushOn,
		OfficialPushOn: DefaultOfficialPushOn,
	}
	switch official, err := s.q.GetOfficialPushSetting(ctx, accountID); {
	case err == nil:
		out.OfficialPushOn = official
	case errors.Is(err, pgx.ErrNoRows):
		// No such account row: the default.
	default:
		return AlertSettings{}, fmt.Errorf("service: reading the official alerts switch: %w", err)
	}
	switch push, err := s.q.GetPushKindSettings(ctx, accountID); {
	case err == nil:
		out.CriticalPushOn = push.CriticalPushOn
		out.ChangesPushOn = push.ChangesPushOn
	case errors.Is(err, pgx.ErrNoRows):
		// No such account row: the defaults.
	default:
		return AlertSettings{}, fmt.Errorf("service: reading push switches: %w", err)
	}
	row, err := s.q.GetAlertSettings(ctx, accountID)
	switch {
	case err == nil:
		out.ReliableAlertsOn = row.ReliableAlertsOn
		out.AlertRadiusKm = int(row.AlertRadiusKm)
		if row.EnabledAt.Valid {
			at := row.EnabledAt.Time
			out.EnabledAt = &at
		}
	case errors.Is(err, pgx.ErrNoRows):
		// No row: the defaults.
	default:
		return AlertSettings{}, fmt.Errorf("service: reading alert settings: %w", err)
	}
	_, has, err := s.HomeAreaCell(ctx, accountID)
	if err != nil {
		return AlertSettings{}, err
	}
	out.HasHomeArea = has
	return out, nil
}

// UpdateAlertSettings merges the update over the stored state, validates the
// result and saves it. The distance must be one of AlertRadii, and the Reliable
// switch cannot be on without a home area. The answer is the state as stored, so
// a switch that goes on carries the time the database stamped.
//
// Phase 3.3 adds the two phone switches (decision 12). D-08: asking for Critical
// on needs a home area, because Critical reports are measured from it; Changes
// to reports I posted or voted on needs none, and turning either off never does.
// UI-SPEC 2b: the stored Critical value is kept while the home area is missing,
// so UpdateProfile's clear path does not touch it and setting an area again
// brings Critical back as it was. Every part is checked before anything is
// written; 3.2's row is written only when one of its fields is set and the push
// row only when one of its two fields is set.
func (s *AccountService) UpdateAlertSettings(ctx context.Context, accountID int64, u AlertSettingsUpdate) (AlertSettings, error) {
	if u.ReliableAlerts == nil && u.AlertRadiusKm == nil && u.CriticalPush == nil && u.ChangesPush == nil && u.OfficialPush == nil {
		return AlertSettings{}, ValidationError{Field: "body", Message: AlertsNothingToSaveMessage}
	}
	if u.AlertRadiusKm != nil && !validAlertRadius(*u.AlertRadiusKm) {
		return AlertSettings{}, ValidationError{Field: "alert_radius_km", Message: AlertRadiusMessage}
	}

	cur, err := s.AlertSettings(ctx, accountID)
	if err != nil {
		return AlertSettings{}, err
	}
	on, radius := cur.ReliableAlertsOn, cur.AlertRadiusKm
	if u.ReliableAlerts != nil {
		on = *u.ReliableAlerts
	}
	if u.AlertRadiusKm != nil {
		radius = *u.AlertRadiusKm
	}
	critical, changes := cur.CriticalPushOn, cur.ChangesPushOn
	if u.CriticalPush != nil {
		critical = *u.CriticalPush
	}
	if u.ChangesPush != nil {
		changes = *u.ChangesPush
	}
	if on && !cur.HasHomeArea {
		return AlertSettings{}, ValidationError{Field: "reliable_alerts", Message: AlertsNeedHomeAreaMessage}
	}
	if u.CriticalPush != nil && *u.CriticalPush && !cur.HasHomeArea {
		return AlertSettings{}, ValidationError{Field: "critical_push", Message: AlertsNeedHomeAreaMessage}
	}
	// Phase 5 (D-11): the official switch follows the same home area rule as
	// Critical, because official alerts are matched to the home district. Turning
	// it off never needs an area, and the stored value is kept while the area is
	// missing.
	if u.OfficialPush != nil && *u.OfficialPush && !cur.HasHomeArea {
		return AlertSettings{}, ValidationError{Field: "official_push", Message: AlertsNeedHomeAreaMessage}
	}

	out := cur
	if u.ReliableAlerts != nil || u.AlertRadiusKm != nil {
		row, err := s.q.UpsertAlertSettings(ctx, sqlcgen.UpsertAlertSettingsParams{
			AccountID:        accountID,
			ReliableAlertsOn: on,
			AlertRadiusKm:    int16(radius),
			Now:              s.now().UTC(),
		})
		if err != nil {
			return AlertSettings{}, fmt.Errorf("service: saving alert settings: %w", err)
		}
		out.ReliableAlertsOn = row.ReliableAlertsOn
		out.AlertRadiusKm = int(row.AlertRadiusKm)
		out.EnabledAt = nil
		if row.EnabledAt.Valid {
			at := row.EnabledAt.Time
			out.EnabledAt = &at
		}
	}
	if u.CriticalPush != nil || u.ChangesPush != nil {
		row, err := s.q.UpsertPushKindSettings(ctx, sqlcgen.UpsertPushKindSettingsParams{
			AccountID:      accountID,
			CriticalPushOn: critical,
			ChangesPushOn:  changes,
			Now:            s.now().UTC(),
		})
		if err != nil {
			return AlertSettings{}, fmt.Errorf("service: saving push switches: %w", err)
		}
		out.CriticalPushOn = row.CriticalPushOn
		out.ChangesPushOn = row.ChangesPushOn
	}
	if u.OfficialPush != nil {
		stored, err := s.q.UpsertOfficialPushSetting(ctx, sqlcgen.UpsertOfficialPushSettingParams{
			AccountID:      accountID,
			OfficialPushOn: *u.OfficialPush,
			Now:            s.now().UTC(),
		})
		if err != nil {
			return AlertSettings{}, fmt.Errorf("service: saving the official alerts switch: %w", err)
		}
		out.OfficialPushOn = stored
	}
	return out, nil
}
