// updatesalerts.go is the Reliable reporter alert step of Refresh (plan
// 03.2-13, NOTIF-05, D-26): an opt in update when a Reliable reporter posts
// within the reader's chosen distance of their home area. It is the one place a
// report becomes news for someone who has not voted on or posted it.

package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// refreshReliableAlerts is NOTIF-05's rule, and Phase 3.3's kind 3 reuses it.
// It runs at the end of Refresh and does nothing, with no error, when the
// reader has not switched the alert on, has no home area, or the service has no
// nearby reader. It never stops the other updates: Refresh has already written
// those (D-28).
//
// The order is part of the rule:
//
//  1. Read the account's switch, distance and the time the switch went on, then
//     its home area cell. Either missing ends the step.
//  2. Read the reports around the middle of the home cell (HomeAreaAnchor,
//     Q5) with the chosen distance, through the same Nearby read the feed uses.
//     The anchor is decoded, used for this one read and dropped; nothing is
//     written and no coordinate leaves this function.
//  3. Keep a report only when MayNotifyAboutReport(UpdateReliableNearby, ...)
//     says it may be announced (D-29: the resolver's answer, from posting while
//     it is Live or Provisional, never Hidden, Withheld, Retracted or expired,
//     critical or not, owner amendment 2026-10-18), its reporter's tag is
//     Reliable (the same tag the Show menu tick reads), it is not the reader's
//     own, not one the reader follows or muted, and it was posted after the
//     switch went on.
//  4. Read the reporter accounts of the survivors once. For each, oldest report
//     first: skip it when this reader already got ReliableAlertsPerReporterPerHour
//     alert from that reporter in the last hour, otherwise write it. A reporter
//     who just got an alert in this pass is skipped for the rest of it.
//
// A duplicate (one alert per report ever) or a reader over the hourly ceiling
// is the normal answer pgx.ErrNoRows from the insert, not an error; the report
// stays eligible for a later poll while it lasts.
//
// The note counts toward the one update per report per hour (D-12, W3).
// InsertReliableAlert therefore also seeds the reader's cursor for the report in
// the same statement, and only when the note row is actually written: the
// report's current visibility as the state, and adopted_at and last_update_at
// set to now. A reader who votes on the report later is then adopted silently
// and the hour gap sees the note, so "Now confirmed" cannot follow it inside the
// hour. An existing cursor is left alone. Phase 3.3 kind 3 reuses this step, so
// the seed belongs here and not in the bell's own read.
//
// TRUSTX-03 burst detection is not built, and it is a hard dependency before
// this rule reaches push (ROADMAP 3.2 scope note 2). The per reporter cap and
// the Not confirmed yet wording soften it for the in app bell only; since the
// 2026-10-18 amendment this also covers non critical reports at posting.
func (s *UpdatesService) refreshReliableAlerts(ctx context.Context, accountID int64, now time.Time) error {
	if s.reports == nil {
		return nil
	}

	settings, err := s.q.GetAlertSettings(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("reading alert settings: %w", err)
	}
	if !settings.ReliableAlertsOn || !settings.EnabledAt.Valid {
		return nil
	}
	enabledAt := settings.EnabledAt.Time

	cell, err := s.q.GetAccountHomeCell(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("reading home area cell: %w", err)
	}
	if cell == nil {
		return nil
	}
	lat, lon, ok := HomeAreaAnchor(*cell)
	if !ok {
		return nil
	}

	radius := int(settings.AlertRadiusKm)
	if !validAlertRadius(radius) {
		radius = DefaultAlertRadiusKm
	}
	views, err := s.reports.Nearby(ctx, NearbyQuery{
		Latitude:        lat,
		Longitude:       lon,
		RadiusKm:        float64(radius),
		ViewerAccountID: accountID,
	})
	if err != nil {
		return fmt.Errorf("reading nearby reports: %w", err)
	}

	var candidates []ReportView
	for _, v := range views {
		if !MayNotifyAboutReport(UpdateReliableNearby, v.Visibility, v.ExpiresAt, now) {
			continue
		}
		if v.Trust.Reporter.Tag != ReporterReliable {
			continue
		}
		if v.IsOwnReport || v.ViewerFollows || v.ViewerMuted {
			continue
		}
		if v.CreatedAt.Before(enabledAt) {
			continue
		}
		candidates = append(candidates, v)
	}
	if len(candidates) == 0 {
		return nil
	}
	// Oldest first, so when one reporter has several reports in range the first
	// one posted is the one the reader hears about.
	sort.SliceStable(candidates, func(i, j int) bool {
		if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		}
		return candidates[i].ID < candidates[j].ID
	})

	ids := make([]int64, 0, len(candidates))
	for _, v := range candidates {
		ids = append(ids, v.ID)
	}
	reporterRows, err := s.q.ReporterAccountsForReports(ctx, ids)
	if err != nil {
		return fmt.Errorf("reading reporters: %w", err)
	}
	reporterByReport := make(map[int64]*int64, len(reporterRows))
	for _, rr := range reporterRows {
		reporterByReport[rr.ReportID] = rr.ReporterAccountID
	}

	windowStart := now.Add(-updatesCeilingWindow)
	alerted := make(map[int64]bool, len(candidates))
	for _, v := range candidates {
		reporter := reporterByReport[v.ID]
		if reporter == nil || alerted[*reporter] {
			continue
		}
		sent, err := s.q.AlertsFromReporterInWindow(ctx, sqlcgen.AlertsFromReporterInWindowParams{
			RecipientID: accountID,
			ReporterID:  *reporter,
			WindowStart: windowStart,
		})
		if err != nil {
			return fmt.Errorf("counting alerts from a reporter: %w", err)
		}
		if sent >= ReliableAlertsPerReporterPerHour {
			continue
		}
		_, err = s.q.InsertReliableAlert(ctx, sqlcgen.InsertReliableAlertParams{
			AccountID: accountID,
			ReportID:  v.ID,
			Now:       now,
			HourEdge:  windowStart,
			Ceiling:   UpdatesHourlyCeiling,
			State:     string(v.Visibility),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return fmt.Errorf("writing alert for report %d: %w", v.ID, err)
		}
		alerted[*reporter] = true
	}
	return nil
}
