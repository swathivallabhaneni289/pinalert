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
type AlertSettings struct {
	ReliableAlertsOn bool
	AlertRadiusKm    int
	EnabledAt        *time.Time
	HasHomeArea      bool
}

// AlertSettingsUpdate is one edit. A nil field leaves that part as stored.
type AlertSettingsUpdate struct {
	ReliableAlerts *bool
	AlertRadiusKm  *int
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
// never saved a choice reads off and DefaultAlertRadiusKm.
func (s *AccountService) AlertSettings(ctx context.Context, accountID int64) (AlertSettings, error) {
	out := AlertSettings{AlertRadiusKm: DefaultAlertRadiusKm}
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
// result and saves it. The distance must be one of AlertRadii, and the switch
// cannot be on without a home area. The answer is the state as stored, so a
// switch that goes on carries the time the database stamped.
func (s *AccountService) UpdateAlertSettings(ctx context.Context, accountID int64, u AlertSettingsUpdate) (AlertSettings, error) {
	if u.ReliableAlerts == nil && u.AlertRadiusKm == nil {
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
	if on && !cur.HasHomeArea {
		return AlertSettings{}, ValidationError{Field: "reliable_alerts", Message: AlertsNeedHomeAreaMessage}
	}

	row, err := s.q.UpsertAlertSettings(ctx, sqlcgen.UpsertAlertSettingsParams{
		AccountID:        accountID,
		ReliableAlertsOn: on,
		AlertRadiusKm:    int16(radius),
		Now:              s.now().UTC(),
	})
	if err != nil {
		return AlertSettings{}, fmt.Errorf("service: saving alert settings: %w", err)
	}
	out := AlertSettings{
		ReliableAlertsOn: row.ReliableAlertsOn,
		AlertRadiusKm:    int(row.AlertRadiusKm),
		HasHomeArea:      cur.HasHomeArea,
	}
	if row.EnabledAt.Valid {
		at := row.EnabledAt.Time
		out.EnabledAt = &at
	}
	return out, nil
}
