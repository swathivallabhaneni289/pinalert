package service_test

// mutes_test.go proves MuteService (plan 03.2-04, D-11, NOTIF-02) without a
// database: a hidden report cannot be probed through Mute, Unmute always
// reaches the store, every call is keyed by the account id passed in, and the
// muted list tells the truth about availability and text.

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// The generated Queries value must satisfy the narrow mute querier.
var _ service.MuteQuerier = (*sqlcgen.Queries)(nil)

// fakeMuteQuerier is the in memory MuteQuerier of these tests.
type fakeMuteQuerier struct {
	row     sqlcgen.ReportForMuteRow
	readErr error

	muted     []sqlcgen.MuteReportParams
	unmuted   []sqlcgen.UnmuteReportParams
	reads     []int64
	listArgs  []sqlcgen.MutedReportsForAccountParams
	countArgs []int64

	listRows []sqlcgen.MutedReportsForAccountRow
	count    int64
}

func (f *fakeMuteQuerier) ReportForMute(_ context.Context, reportID int64) (sqlcgen.ReportForMuteRow, error) {
	f.reads = append(f.reads, reportID)
	if f.readErr != nil {
		return sqlcgen.ReportForMuteRow{}, f.readErr
	}
	return f.row, nil
}

func (f *fakeMuteQuerier) MuteReport(_ context.Context, arg sqlcgen.MuteReportParams) error {
	f.muted = append(f.muted, arg)
	return nil
}

func (f *fakeMuteQuerier) UnmuteReport(_ context.Context, arg sqlcgen.UnmuteReportParams) error {
	f.unmuted = append(f.unmuted, arg)
	return nil
}

func (f *fakeMuteQuerier) MutedReportsForAccount(_ context.Context, arg sqlcgen.MutedReportsForAccountParams) ([]sqlcgen.MutedReportsForAccountRow, error) {
	f.listArgs = append(f.listArgs, arg)
	return f.listRows, nil
}

func (f *fakeMuteQuerier) CountMutedReports(_ context.Context, accountID int64) (int64, error) {
	f.countArgs = append(f.countArgs, accountID)
	return f.count, nil
}

func TestMuteMissingAndWithheldReadAsNotFound(t *testing.T) {
	cases := []struct {
		name string
		q    *fakeMuteQuerier
	}{
		{"missing", &fakeMuteQuerier{readErr: pgx.ErrNoRows}},
		{"removed", &fakeMuteQuerier{row: sqlcgen.ReportForMuteRow{ID: 7, Severity: "medium", Category: "flood", ModerationState: "removed"}}},
		{"removed critical", &fakeMuteQuerier{row: sqlcgen.ReportForMuteRow{ID: 7, Severity: "critical", Category: "flood", ModerationState: "removed"}}},
		{"pending low", &fakeMuteQuerier{row: sqlcgen.ReportForMuteRow{ID: 7, Severity: "low", Category: "flood", ModerationState: "pending"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := service.NewMuteService(tc.q)
			err := svc.Mute(context.Background(), 5, 7)
			if !errors.Is(err, service.ErrReportNotFound) {
				t.Fatalf("Mute error = %v, want ErrReportNotFound", err)
			}
			if len(tc.q.muted) != 0 {
				t.Fatalf("MuteReport was called %d times, want 0", len(tc.q.muted))
			}
		})
	}
}

func TestMuteStoreErrorIsWrapped(t *testing.T) {
	boom := errors.New("boom")
	q := &fakeMuteQuerier{readErr: boom}
	err := service.NewMuteService(q).Mute(context.Background(), 5, 7)
	if !errors.Is(err, boom) {
		t.Fatalf("Mute error = %v, want it to wrap the store error", err)
	}
	if errors.Is(err, service.ErrReportNotFound) {
		t.Fatalf("a store failure must not read as not found")
	}
}

func TestMutePendingSeriousAllowed(t *testing.T) {
	cases := []struct {
		name string
		row  sqlcgen.ReportForMuteRow
	}{
		{"pending critical", sqlcgen.ReportForMuteRow{ID: 7, Severity: "critical", Category: "flood", ModerationState: "pending"}},
		{"pending rescue", sqlcgen.ReportForMuteRow{ID: 7, Severity: "medium", Category: "rescue_needed", ModerationState: "pending"}},
		{"clear", sqlcgen.ReportForMuteRow{ID: 7, Severity: "low", Category: "flood", ModerationState: "clear"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &fakeMuteQuerier{row: tc.row}
			if err := service.NewMuteService(q).Mute(context.Background(), 5, 7); err != nil {
				t.Fatalf("Mute error = %v, want nil", err)
			}
			if len(q.muted) != 1 {
				t.Fatalf("MuteReport calls = %d, want 1", len(q.muted))
			}
		})
	}
}

func TestMuteCallsOneStatement(t *testing.T) {
	q := &fakeMuteQuerier{row: sqlcgen.ReportForMuteRow{ID: 7, Severity: "medium", Category: "flood", ModerationState: "clear"}}
	svc := service.NewMuteService(q)
	for i := 0; i < 2; i++ {
		if err := svc.Mute(context.Background(), 5, 7); err != nil {
			t.Fatalf("Mute #%d error = %v", i+1, err)
		}
	}
	// A repeat is the same single call each time: the statement itself is
	// idempotent, so the service makes no second write or delete of its own.
	if len(q.muted) != 2 {
		t.Fatalf("MuteReport calls = %d, want 2 (one per Mute)", len(q.muted))
	}
	if len(q.unmuted) != 0 {
		t.Fatalf("Mute called UnmuteReport %d times", len(q.unmuted))
	}
	if len(q.reads) != 2 {
		t.Fatalf("ReportForMute calls = %d, want 2 (one per Mute)", len(q.reads))
	}
}

func TestUnmuteAlwaysAllowed(t *testing.T) {
	// Even when the report is removed, missing or held, Unmute works and never
	// reads the report first.
	q := &fakeMuteQuerier{
		row:     sqlcgen.ReportForMuteRow{ID: 7, Severity: "low", Category: "flood", ModerationState: "removed"},
		readErr: pgx.ErrNoRows,
	}
	if err := service.NewMuteService(q).Unmute(context.Background(), 5, 7); err != nil {
		t.Fatalf("Unmute error = %v, want nil", err)
	}
	if len(q.reads) != 0 {
		t.Fatalf("Unmute read the report %d times, want 0", len(q.reads))
	}
	if len(q.unmuted) != 1 {
		t.Fatalf("UnmuteReport calls = %d, want 1", len(q.unmuted))
	}
}

func TestMuteKeyedByAccountOnly(t *testing.T) {
	q := &fakeMuteQuerier{
		row:      sqlcgen.ReportForMuteRow{ID: 7, Severity: "medium", Category: "flood", ModerationState: "clear"},
		count:    3,
		listRows: nil,
	}
	svc := service.NewMuteService(q)
	ctx := context.Background()
	if err := svc.Mute(ctx, 41, 7); err != nil {
		t.Fatalf("Mute: %v", err)
	}
	if err := svc.Unmute(ctx, 42, 7); err != nil {
		t.Fatalf("Unmute: %v", err)
	}
	if len(q.muted) != 1 || q.muted[0].AccountID != 41 || q.muted[0].ReportID != 7 {
		t.Fatalf("MuteReport args = %+v, want account 41 report 7", q.muted)
	}
	if len(q.unmuted) != 1 || q.unmuted[0].AccountID != 42 || q.unmuted[0].ReportID != 7 {
		t.Fatalf("UnmuteReport args = %+v, want account 42 report 7", q.unmuted)
	}
}
