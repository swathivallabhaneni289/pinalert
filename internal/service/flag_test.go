package service_test

// flag_test.go proves the flag rules of plan 04-20 (D-09, R-01, R-04) without
// a database: the five reasons and their copy, the order of the checks inside
// FlagService.Flag, and the fact that the service can only read one report row
// and insert one flag row.

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// The generated Queries value must satisfy the narrow flag querier, because
// NewRouter passes deps.Sessions to NewFlagService.
var _ service.FlagQuerier = (*sqlcgen.Queries)(nil)

// fakeFlagQuerier is the in memory FlagQuerier of these tests.
type fakeFlagQuerier struct {
	row       sqlcgen.ReportVoteContextRow
	readErr   error
	insertErr error
	calls     []string
	inserted  []sqlcgen.InsertReportFlagParams
}

func (f *fakeFlagQuerier) ReportVoteContext(_ context.Context, _ int64) (sqlcgen.ReportVoteContextRow, error) {
	f.calls = append(f.calls, "ReportVoteContext")
	if f.readErr != nil {
		return sqlcgen.ReportVoteContextRow{}, f.readErr
	}
	return f.row, nil
}

func (f *fakeFlagQuerier) InsertReportFlag(_ context.Context, arg sqlcgen.InsertReportFlagParams) (int64, error) {
	f.calls = append(f.calls, "InsertReportFlag")
	if f.insertErr != nil {
		return 0, f.insertErr
	}
	f.inserted = append(f.inserted, arg)
	return 1, nil
}

func TestServerCopyFlag(t *testing.T) {
	messages := []struct {
		name string
		got  string
		want string
	}{
		{"FlagReasonMessage", service.FlagReasonMessage, "Choose a reason to continue."},
		{"FlagOwnReportMessage", service.FlagOwnReportMessage, "You can't flag your own report."},
		{"FlagNotFoundMessage", service.FlagNotFoundMessage, "This report is no longer available."},
		{"FlagAlreadyMessage", service.FlagAlreadyMessage, "You already flagged this report. Moderators will see it."},
	}
	for _, m := range messages {
		t.Run(m.name, func(t *testing.T) {
			if m.got != m.want {
				t.Errorf("%s = %q, want %q", m.name, m.got, m.want)
			}
			assertServerCopy(t, m.got)
		})
	}

	labels := map[service.FlagReason]string{
		service.FlagSpam:      "Spam or gibberish",
		service.FlagAbusive:   "Abusive or hateful",
		service.FlagFalse:     "False or a hoax",
		service.FlagDangerous: "Dangerous or illegal",
		service.FlagOther:     "Something else",
	}
	for reason, want := range labels {
		t.Run("label "+string(reason), func(t *testing.T) {
			got := service.FlagReasonLabel(reason)
			if got != want {
				t.Errorf("FlagReasonLabel(%q) = %q, want %q", reason, got, want)
			}
			assertServerCopy(t, got)
		})
	}
	if got := service.FlagReasonLabel("hoax"); got != "" {
		t.Errorf("FlagReasonLabel(hoax) = %q, want the empty string", got)
	}
}

func TestFlagReasons(t *testing.T) {
	want := []service.FlagReason{"spam", "abusive", "false", "dangerous", "other"}
	if !reflect.DeepEqual(service.FlagReasons, want) {
		t.Fatalf("FlagReasons = %v, want %v", service.FlagReasons, want)
	}
	seen := map[string]service.FlagReason{}
	for _, r := range service.FlagReasons {
		if !r.Valid() {
			t.Errorf("%q should be valid", r)
		}
		label := service.FlagReasonLabel(r)
		if label == "" {
			t.Errorf("%q has no label", r)
		}
		if prev, dup := seen[label]; dup {
			t.Errorf("label %q is shared by %q and %q", label, prev, r)
		}
		seen[label] = r
	}
	for _, bad := range []service.FlagReason{"", "Spam", "hoax", "spam "} {
		if bad.Valid() {
			t.Errorf("%q should not be valid", bad)
		}
	}
}

func TestFlagServiceRules(t *testing.T) {
	const reportID, accountID int64 = 41, 7
	other := int64(9)
	self := accountID

	row := func(sev service.Severity, cat service.Category, mod service.ModerationState, reporter *int64) sqlcgen.ReportVoteContextRow {
		return sqlcgen.ReportVoteContextRow{
			Severity:          string(sev),
			Category:          string(cat),
			ModerationState:   string(mod),
			ReporterAccountID: reporter,
			ExpiresAt:         time.Now().Add(time.Hour),
		}
	}
	expired := row(service.SeverityLow, service.CategoryFlood, service.ModerationClear, &other)
	expired.ExpiresAt = time.Now().Add(-time.Hour)

	boom := errors.New("boom")

	tests := []struct {
		name       string
		reason     service.FlagReason
		row        sqlcgen.ReportVoteContextRow
		readErr    error
		insertErr  error
		wantErr    error // matched with errors.Is
		wantVal    bool  // a ValidationError on field reason
		wantOther  bool  // a wrapped, non sentinel error
		wantCalls  []string
		wantInsert int
	}{
		{name: "invalid reason makes no call", reason: "hoax", row: row("low", "flood", "clear", &other), wantVal: true, wantCalls: nil},
		{name: "empty reason makes no call", reason: "", row: row("low", "flood", "clear", &other), wantVal: true, wantCalls: nil},
		{name: "missing report", reason: service.FlagSpam, readErr: pgx.ErrNoRows, wantErr: service.ErrReportNotFound, wantCalls: []string{"ReportVoteContext"}},
		{name: "read error is wrapped", reason: service.FlagSpam, readErr: boom, wantErr: boom, wantOther: true, wantCalls: []string{"ReportVoteContext"}},
		{name: "removed low report", reason: service.FlagSpam, row: row(service.SeverityLow, service.CategoryFlood, service.ModerationRemoved, &other), wantErr: service.ErrReportNotFound, wantCalls: []string{"ReportVoteContext"}},
		{name: "removed critical report", reason: service.FlagSpam, row: row(service.SeverityCritical, service.CategoryFlood, service.ModerationRemoved, &other), wantErr: service.ErrReportNotFound, wantCalls: []string{"ReportVoteContext"}},
		{name: "pending low report", reason: service.FlagSpam, row: row(service.SeverityLow, service.CategoryFlood, service.ModerationPending, &other), wantErr: service.ErrReportNotFound, wantCalls: []string{"ReportVoteContext"}},
		{name: "pending critical report is published", reason: service.FlagFalse, row: row(service.SeverityCritical, service.CategoryFlood, service.ModerationPending, &other), wantCalls: []string{"ReportVoteContext", "InsertReportFlag"}, wantInsert: 1},
		{name: "pending low rescue needed is published", reason: service.FlagDangerous, row: row(service.SeverityLow, service.CategoryRescueNeeded, service.ModerationPending, &other), wantCalls: []string{"ReportVoteContext", "InsertReportFlag"}, wantInsert: 1},
		{name: "clear report", reason: service.FlagAbusive, row: row(service.SeverityLow, service.CategoryFlood, service.ModerationClear, &other), wantCalls: []string{"ReportVoteContext", "InsertReportFlag"}, wantInsert: 1},
		{name: "zero moderation value reads as clear", reason: service.FlagOther, row: row(service.SeverityLow, service.CategoryFlood, "", &other), wantCalls: []string{"ReportVoteContext", "InsertReportFlag"}, wantInsert: 1},
		{name: "own report", reason: service.FlagSpam, row: row(service.SeverityLow, service.CategoryFlood, service.ModerationClear, &self), wantErr: service.ErrFlagOwnReport, wantCalls: []string{"ReportVoteContext"}},
		{name: "own removed report reads as not found", reason: service.FlagSpam, row: row(service.SeverityLow, service.CategoryFlood, service.ModerationRemoved, &self), wantErr: service.ErrReportNotFound, wantCalls: []string{"ReportVoteContext"}},
		{name: "report with no reporter account", reason: service.FlagSpam, row: row(service.SeverityLow, service.CategoryFlood, service.ModerationClear, nil), wantCalls: []string{"ReportVoteContext", "InsertReportFlag"}, wantInsert: 1},
		{name: "expired report can still be flagged", reason: service.FlagSpam, row: expired, wantCalls: []string{"ReportVoteContext", "InsertReportFlag"}, wantInsert: 1},
		{name: "repeat flag", reason: service.FlagSpam, row: row(service.SeverityLow, service.CategoryFlood, service.ModerationClear, &other), insertErr: pgx.ErrNoRows, wantErr: service.ErrAlreadyFlagged, wantCalls: []string{"ReportVoteContext", "InsertReportFlag"}},
		{name: "insert error is wrapped", reason: service.FlagSpam, row: row(service.SeverityLow, service.CategoryFlood, service.ModerationClear, &other), insertErr: boom, wantErr: boom, wantOther: true, wantCalls: []string{"ReportVoteContext", "InsertReportFlag"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &fakeFlagQuerier{row: tc.row, readErr: tc.readErr, insertErr: tc.insertErr}
			svc := service.NewFlagService(q)
			err := svc.Flag(context.Background(), service.FlagInput{ReportID: reportID, AccountID: accountID, Reason: tc.reason})

			switch {
			case tc.wantVal:
				var ve service.ValidationError
				if !errors.As(err, &ve) || ve.Field != "reason" || ve.Message != service.FlagReasonMessage {
					t.Errorf("err = %v, want ValidationError on reason with FlagReasonMessage", err)
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("err = %v, want errors.Is %v", err, tc.wantErr)
				}
				if tc.wantOther {
					for _, s := range []error{service.ErrReportNotFound, service.ErrFlagOwnReport, service.ErrAlreadyFlagged} {
						if errors.Is(err, s) {
							t.Errorf("err = %v must not be the sentinel %v", err, s)
						}
					}
				}
			default:
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			}

			if fmt.Sprint(q.calls) != fmt.Sprint(tc.wantCalls) {
				t.Errorf("calls = %v, want %v", q.calls, tc.wantCalls)
			}
			if len(q.inserted) != tc.wantInsert {
				t.Fatalf("inserted %d flags, want %d", len(q.inserted), tc.wantInsert)
			}
			if tc.wantInsert == 1 {
				want := sqlcgen.InsertReportFlagParams{ReportID: reportID, AccountID: accountID, Reason: string(tc.reason)}
				if q.inserted[0] != want {
					t.Errorf("inserted %+v, want %+v", q.inserted[0], want)
				}
			}
		})
	}
}

func TestFlagQuerierCannotWriteReports(t *testing.T) {
	typ := reflect.TypeOf((*service.FlagQuerier)(nil)).Elem()
	if typ.NumMethod() != 2 {
		t.Fatalf("FlagQuerier has %d methods, want 2", typ.NumMethod())
	}
	var names []string
	for i := 0; i < typ.NumMethod(); i++ {
		names = append(names, typ.Method(i).Name)
	}
	sort.Strings(names)
	if fmt.Sprint(names) != "[InsertReportFlag ReportVoteContext]" {
		t.Errorf("FlagQuerier methods = %v, want InsertReportFlag and ReportVoteContext", names)
	}
}
