package service

// mutes.go is the service behind the Mute control and the Muted reports window
// of Settings (plan 03.2-04, D-11, NOTIF-02). A signed in reader can mute any
// report they can see; muting clears that report's unread updates and stops
// new ones, and unmuting starts again from a fresh silent baseline. Every call
// is keyed by the account id the caller passes in, which comes from the
// verified account gate and never from a request value (D-29 seam).

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// MuteQuerier is the narrow store surface of MuteService. It reads one report
// identity and the account's own mute rows and nothing else, so a mute has no
// way to write a report column or reach another account's rows.
type MuteQuerier interface {
	ReportForMute(ctx context.Context, reportID int64) (sqlcgen.ReportForMuteRow, error)
	MuteReport(ctx context.Context, arg sqlcgen.MuteReportParams) error
	UnmuteReport(ctx context.Context, arg sqlcgen.UnmuteReportParams) error
	MutedReportsForAccount(ctx context.Context, arg sqlcgen.MutedReportsForAccountParams) ([]sqlcgen.MutedReportsForAccountRow, error)
	CountMutedReports(ctx context.Context, accountID int64) (int64, error)
}

// The generated Queries value satisfies the narrow querier.
var _ MuteQuerier = (*sqlcgen.Queries)(nil)

// MuteService applies the mute rules.
type MuteService struct {
	q   MuteQuerier
	now func() time.Time
}

// MuteOption configures a MuteService.
type MuteOption func(*MuteService)

// WithMuteClock replaces the clock, for tests.
func WithMuteClock(now func() time.Time) MuteOption {
	return func(s *MuteService) {
		if now != nil {
			s.now = now
		}
	}
}

// NewMuteService stores the querier. It never dereferences it, so a router
// built with a nil store still constructs.
func NewMuteService(q MuteQuerier, opts ...MuteOption) *MuteService {
	s := &MuteService{q: q, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Mute mutes one report for the account. The report is read once; a missing
// report is ErrReportNotFound, and so is a withheld one (removed at any
// severity, or pending and not serious), so a held report cannot be told apart
// from a missing one. A pending serious report is published while its check
// retries, so it may be muted. The caller's own report may be muted, unlike
// Flag.
//
// Muting deletes the account's unread updates for the report in the same
// statement and Refresh skips muted reports from then on (D-11). A later vote
// on the report does not unmute it. A repeat Mute is the same single statement
// and is harmless.
func (s *MuteService) Mute(ctx context.Context, accountID, reportID int64) error {
	row, err := s.q.ReportForMute(ctx, reportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReportNotFound
		}
		return fmt.Errorf("service: Mute: reading report %d: %w", reportID, err)
	}

	meta := ReportMeta{
		Severity:   Severity(row.Severity),
		Category:   Category(row.Category),
		Moderation: ModerationState(row.ModerationState),
	}
	if meta.Withheld() {
		return ErrReportNotFound
	}

	if err := s.q.MuteReport(ctx, sqlcgen.MuteReportParams{AccountID: accountID, ReportID: reportID}); err != nil {
		return fmt.Errorf("service: Mute: muting report %d: %w", reportID, err)
	}
	return nil
}

// Unmute clears the account's mute on a report. It never reads the report: it
// must work on a report that is now held, removed or gone, so Settings can
// always clear a mute (D-11). The statement also drops the account's cursor for
// the report, so the next refresh starts a fresh silent baseline and nothing
// from the muted time is announced late.
func (s *MuteService) Unmute(ctx context.Context, accountID, reportID int64) error {
	if err := s.q.UnmuteReport(ctx, sqlcgen.UnmuteReportParams{AccountID: accountID, ReportID: reportID}); err != nil {
		return fmt.Errorf("service: Unmute: unmuting report %d: %w", reportID, err)
	}
	return nil
}
