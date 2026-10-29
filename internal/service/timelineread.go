// timelineread.go is the gated read behind GET /api/reports/{id}/timeline (plan
// 08-01, PAGE-01, SC2). TimelineService is a service of its own, so the
// ReportService Querier, which Phase 5's Demo memquerier mirrors, does not grow.
// The rules live in BuildTimeline; this file only decides who may ask and loads
// the rows.

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// TimelineNotFoundMessage is the one answer for a report that does not exist,
// is too old, or is not shown to the reader. It is the words the Flag control
// uses for the same case, so a withheld report looks like a missing one.
const TimelineNotFoundMessage = "This report is no longer available."

// TimelineQuerier is the narrow store surface of TimelineService: the report's
// meta and reporter in one row (the read VotingService and FlagService make) and
// its whole vote log. Both are read only.
type TimelineQuerier interface {
	ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error)
	TimelineVoteLog(ctx context.Context, arg sqlcgen.TimelineVoteLogParams) ([]sqlcgen.TimelineVoteLogRow, error)
}

var _ TimelineQuerier = (*sqlcgen.Queries)(nil)

// TimelineService reads the history of one report.
type TimelineService struct {
	q       TimelineQuerier
	reports *ReportService
	now     func() time.Time
}

// TimelineOption configures a TimelineService.
type TimelineOption func(*TimelineService)

// WithTimelineClock overrides the clock the read uses to decide whether a report
// has faded.
func WithTimelineClock(fn func() time.Time) TimelineOption {
	return func(s *TimelineService) {
		if fn != nil {
			s.now = fn
		}
	}
}

// NewTimelineService stores its collaborators. reports is the service whose Focus
// read decides who may see a report at all.
func NewTimelineService(q TimelineQuerier, reports *ReportService, opts ...TimelineOption) *TimelineService {
	s := &TimelineService{q: q, reports: reports, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Timeline returns the history of a report for a reader. The order is part of
// the specification:
//
//  1. Focus decides whether the reader may open the report. Every status other
//     than FocusOK (unknown, too old, withheld, in any state no view lists)
//     returns ErrReportNotFound, so nothing about moderation is revealed.
//  2. ReportVoteContext gives the meta, the reporter, the created and expiry
//     times. A report that has become withheld since the Focus read is
//     ErrReportNotFound too.
//  3. TimelineVoteLog gives the log, at most TimelineLogCap rows.
//  4. BuildTimeline replays it.
func (s *TimelineService) Timeline(ctx context.Context, reportID, viewerAccountID int64) ([]TimelineEvent, error) {
	if s.reports == nil {
		return nil, errors.New("service: timeline has no report service")
	}
	now := s.now().UTC()

	res, err := s.reports.Focus(ctx, FocusQuery{ReportID: reportID, ViewerAccountID: viewerAccountID})
	if err != nil {
		return nil, fmt.Errorf("service: timeline focus: %w", err)
	}
	if res.Status != FocusOK {
		return nil, ErrReportNotFound
	}

	rc, err := s.q.ReportVoteContext(ctx, reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrReportNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("service: timeline report context: %w", err)
	}
	meta := ReportMeta{
		Severity:   Severity(rc.Severity),
		Category:   Category(rc.Category),
		Moderation: ModerationState(rc.ModerationState),
	}
	if meta.Withheld() {
		return nil, ErrReportNotFound
	}

	log, err := s.q.TimelineVoteLog(ctx, sqlcgen.TimelineVoteLogParams{ReportID: reportID, RowCap: TimelineLogCap})
	if err != nil {
		return nil, fmt.Errorf("service: timeline vote log: %w", err)
	}

	return BuildTimeline(TimelineInput{
		ReportID:          reportID,
		ReporterAccountID: rc.ReporterAccountID,
		Meta:              meta,
		CreatedAt:         rc.CreatedAt,
		ExpiresAt:         rc.ExpiresAt,
		Now:               now,
		Log:               log,
		ViewerAccountID:   viewerAccountID,
	}), nil
}
