// alertsettings.go holds the one setting left behind the alerts (NOTIF-05, D-26,
// narrowed by quick 261103-jal, owner decision 2026-11-03): the distance from the
// home area that Critical reports and the Reliable bell row reach, and the home
// area anchor the distance is measured from. The switches that used to sit next
// to it are gone from the product: the device switch is the only way to stop
// phone notes, and the Reliable bell row has no switch. The bell row itself is
// raised by refreshReliableAlerts (updatesalerts.go); nothing here raises
// anything.

package service

import (
	"context"
	"errors"
	"fmt"

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
	// AlertsNothingToSaveMessage is the refusal for a body that names no
	// distance.
	AlertsNothingToSaveMessage = "Nothing to save."
)

// AlertSettings is one account's alert distance as stored, with whether a home
// area exists. It never carries the home area cell or any coordinate.
//
// Owner decision 2026-11-03, quick 261103-jal (D8, C10): the four columns
// reliable_alerts_on, critical_push_on, changes_push_on and official_push_on are
// kept in the database and no longer decide delivery, so they leave this struct.
// ChangesPushOn is the one exception until the changes pop-up is removed in the
// same quick task: changesAskFor still reads it.
type AlertSettings struct {
	AlertRadiusKm int
	HasHomeArea   bool
	ChangesPushOn bool
}

// AlertSettingsUpdate is one edit. A nil field leaves that part as stored.
type AlertSettingsUpdate struct {
	AlertRadiusKm *int
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

// AlertSettings reads the account's alert distance. An account that never saved
// a choice reads DefaultAlertRadiusKm. Owner decision 2026-11-03, quick
// 261103-jal: the stored switches are not read; ChangesPushOn is read only for
// the changes pop-up that the same quick task removes.
func (s *AccountService) AlertSettings(ctx context.Context, accountID int64) (AlertSettings, error) {
	out := AlertSettings{AlertRadiusKm: DefaultAlertRadiusKm, ChangesPushOn: DefaultChangesPushOn}
	switch push, err := s.q.GetPushKindSettings(ctx, accountID); {
	case err == nil:
		out.ChangesPushOn = push.ChangesPushOn
	case errors.Is(err, pgx.ErrNoRows):
		// No such account row: the default.
	default:
		return AlertSettings{}, fmt.Errorf("service: reading push switches: %w", err)
	}
	row, err := s.q.GetAlertSettings(ctx, accountID)
	switch {
	case err == nil:
		out.AlertRadiusKm = int(row.AlertRadiusKm)
	case errors.Is(err, pgx.ErrNoRows):
		// No row: the default.
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

// UpdateAlertSettings saves the distance. It must be one of AlertRadii, and a
// body with no distance is refused as nothing to save. The stored
// reliable_alerts_on goes through UpsertAlertSettings unchanged, so enabled_at,
// the start point the Reliable bell row can still be moved later by, is kept as
// stored by a distance change (quick 261103-jal, D7, D8, C10). The answer is the
// state as stored.
func (s *AccountService) UpdateAlertSettings(ctx context.Context, accountID int64, u AlertSettingsUpdate) (AlertSettings, error) {
	if u.AlertRadiusKm == nil {
		return AlertSettings{}, ValidationError{Field: "body", Message: AlertsNothingToSaveMessage}
	}
	if !validAlertRadius(*u.AlertRadiusKm) {
		return AlertSettings{}, ValidationError{Field: "alert_radius_km", Message: AlertRadiusMessage}
	}

	stored := false
	switch row, err := s.q.GetAlertSettings(ctx, accountID); {
	case err == nil:
		stored = row.ReliableAlertsOn
	case errors.Is(err, pgx.ErrNoRows):
		// No row yet: the stored switch reads off.
	default:
		return AlertSettings{}, fmt.Errorf("service: reading alert settings: %w", err)
	}
	if _, err := s.q.UpsertAlertSettings(ctx, sqlcgen.UpsertAlertSettingsParams{
		AccountID:        accountID,
		ReliableAlertsOn: stored,
		AlertRadiusKm:    int16(*u.AlertRadiusKm),
		Now:              s.now().UTC(),
	}); err != nil {
		return AlertSettings{}, fmt.Errorf("service: saving alert settings: %w", err)
	}
	return s.AlertSettings(ctx, accountID)
}
