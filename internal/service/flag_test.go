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
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/moderation"
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
			// Amended 2026-10-22 by plan 04-31 (owner): a flag carries a set of
			// reasons. Each case sends its one reason as a one element set, and
			// the empty case sends no reason at all.
			var reasons []service.FlagReason
			if tc.reason != "" {
				reasons = []service.FlagReason{tc.reason}
			}
			err := svc.Flag(context.Background(), service.FlagInput{ReportID: reportID, AccountID: accountID, Reasons: reasons})

			switch {
			case tc.wantVal:
				var ve service.ValidationError
				// Amended 2026-10-22 by plan 04-31 (owner): the field is reasons, and
				// an unknown reason has its own message; no reason at all keeps
				// FlagReasonMessage.
				wantMsg := service.FlagReasonsBadMessage
				if tc.reason == "" {
					wantMsg = service.FlagReasonMessage
				}
				if !errors.As(err, &ve) || ve.Field != "reasons" || ve.Message != wantMsg {
					t.Errorf("err = %v, want ValidationError on reasons with %q", err, wantMsg)
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
				want := sqlcgen.InsertReportFlagParams{ReportID: reportID, AccountID: accountID, Reasons: []string{string(tc.reason)}}
				if !reflect.DeepEqual(q.inserted[0], want) {
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

// TestValidateFlagInput pins the shape rules of plan 04-31: at least one
// reason, every reason known and none twice, the set returned in the dialog's
// order, and a note of at most 300 characters after trimming, where empty
// means none.
func TestValidateFlagInput(t *testing.T) {
	r := func(xs ...service.FlagReason) []service.FlagReason { return xs }
	long300 := strings.Repeat("అ", 300) // 300 Telugu letters, 900 bytes
	tests := []struct {
		name        string
		in          service.FlagInput
		wantReasons []service.FlagReason
		wantNote    string
		wantField   string
		wantMsg     string
	}{
		{name: "no reasons", in: service.FlagInput{}, wantField: "reasons", wantMsg: service.FlagReasonMessage},
		{name: "empty list", in: service.FlagInput{Reasons: r()}, wantField: "reasons", wantMsg: service.FlagReasonMessage},
		{name: "unknown reason", in: service.FlagInput{Reasons: r("spam", "hoax")}, wantField: "reasons", wantMsg: service.FlagReasonsBadMessage},
		{name: "empty reason", in: service.FlagInput{Reasons: r("")}, wantField: "reasons", wantMsg: service.FlagReasonsBadMessage},
		{name: "duplicate reason", in: service.FlagInput{Reasons: r("false", "false")}, wantField: "reasons", wantMsg: service.FlagReasonsBadMessage},
		{name: "one reason", in: service.FlagInput{Reasons: r("other")}, wantReasons: r("other")},
		{name: "set is put in dialog order", in: service.FlagInput{Reasons: r("other", "spam", "false")}, wantReasons: r("spam", "false", "other")},
		{name: "all five", in: service.FlagInput{Reasons: r("dangerous", "other", "false", "abusive", "spam")}, wantReasons: r("spam", "abusive", "false", "dangerous", "other")},
		{name: "note is trimmed", in: service.FlagInput{Reasons: r("false"), Note: "  old photo from 2019 \n"}, wantReasons: r("false"), wantNote: "old photo from 2019"},
		{name: "blank note means none", in: service.FlagInput{Reasons: r("false"), Note: " \t\n "}, wantReasons: r("false"), wantNote: ""},
		{name: "300 characters after trimming", in: service.FlagInput{Reasons: r("false"), Note: "  " + long300 + "  "}, wantReasons: r("false"), wantNote: long300},
		{name: "301 characters", in: service.FlagInput{Reasons: r("false"), Note: long300 + "a"}, wantField: "note", wantMsg: service.FlagNoteTooLongMessage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reasons, note, err := service.ValidateFlagInput(tc.in)
			if tc.wantField != "" {
				var ve service.ValidationError
				if !errors.As(err, &ve) || ve.Field != tc.wantField || ve.Message != tc.wantMsg {
					t.Fatalf("err = %v, want ValidationError on %s with %q", err, tc.wantField, tc.wantMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !reflect.DeepEqual(reasons, tc.wantReasons) {
				t.Errorf("reasons = %v, want %v", reasons, tc.wantReasons)
			}
			if note != tc.wantNote {
				t.Errorf("note = %q, want %q", note, tc.wantNote)
			}
		})
	}
}

// TestFlagServiceNote pins how the note is stored and screened (plan 04-31):
// the set and the trimmed note reach the insert, a blank note is NULL and is
// never screened, a note goes through the description's abuse screen but not
// its gibberish rule, so a one word note is stored (owner, 2026-10-23), an
// abusive note is refused and stores nothing, a note
// whose check did not finish is still stored, and a 404 or a 403 never costs
// a screener call.
func TestFlagServiceNote(t *testing.T) {
	other := int64(9)
	self := int64(7)
	clearRow := sqlcgen.ReportVoteContextRow{Severity: "low", Category: "flood", ModerationState: "clear", ReporterAccountID: &other, ExpiresAt: time.Now().Add(time.Hour)}
	own := clearRow
	own.ReporterAccountID = &self
	hateful := moderation.Result{Model: "m", Scores: map[string]float64{"hate": 0.99}}
	borderline := moderation.Result{Model: "m", Scores: map[string]float64{"harassment": 0.5}}

	tests := []struct {
		name        string
		row         sqlcgen.ReportVoteContextRow
		readErr     error
		note        string
		screen      moderation.Result
		screenErr   error
		noScreener  bool
		wantReject  service.RejectReason
		wantErr     error
		wantScreens int
		wantNote    *string
		wantInsert  int
	}{
		{name: "no note is NULL and not screened", row: clearRow, wantInsert: 1},
		{name: "blank note is NULL and not screened", row: clearRow, note: "   ", wantInsert: 1},
		{name: "a note is trimmed, screened once and stored", row: clearRow, note: "  The photo is from 2019.  ", wantScreens: 1, wantNote: strPtr("The photo is from 2019."), wantInsert: 1},
		{name: "a one word note fake is stored", row: clearRow, note: "fake", wantScreens: 1, wantNote: strPtr("fake"), wantInsert: 1},
		{name: "a one word note duplicate is stored", row: clearRow, note: " duplicate ", wantScreens: 1, wantNote: strPtr("duplicate"), wantInsert: 1},
		{name: "a short note old is stored", row: clearRow, note: "old", wantScreens: 1, wantNote: strPtr("old"), wantInsert: 1},
		{name: "a one word note is stored without a screener", row: clearRow, note: "fake", noScreener: true, wantNote: strPtr("fake"), wantInsert: 1},
		{name: "a hateful note is refused", row: clearRow, note: "some hateful words here", screen: hateful, wantScreens: 1, wantReject: service.RejectHateful},
		{name: "a borderline note is stored", row: clearRow, note: "some sharp words here", screen: borderline, wantScreens: 1, wantNote: strPtr("some sharp words here"), wantInsert: 1},
		{name: "an unfinished check still stores the note", row: clearRow, note: "the water has gone down", screenErr: &moderation.CallError{Transient: true}, wantScreens: 1, wantNote: strPtr("the water has gone down"), wantInsert: 1},
		{name: "own report is not screened", row: own, note: "some words here", wantErr: service.ErrFlagOwnReport},
		{name: "missing report is not screened", readErr: pgx.ErrNoRows, note: "some words here", wantErr: service.ErrReportNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &fakeFlagQuerier{row: tc.row, readErr: tc.readErr}
			scr := &fakeTextScreener{result: tc.screen, err: tc.screenErr}
			svc := service.NewFlagService(q, service.WithFlagScreener(scr))
			if tc.noScreener {
				svc = service.NewFlagService(q)
			}
			err := svc.Flag(context.Background(), service.FlagInput{
				ReportID: 41, AccountID: self, Reasons: []service.FlagReason{"other", "false"}, Note: tc.note,
			})
			switch {
			case tc.wantReject != "":
				var nr service.FlagNoteRejectedError
				if !errors.As(err, &nr) || nr.Reason != tc.wantReject || !errors.Is(err, service.ErrFlagNoteRejected) {
					t.Errorf("err = %v, want FlagNoteRejectedError %q", err, tc.wantReject)
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("err = %v, want %v", err, tc.wantErr)
				}
			default:
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			}
			if scr.calls != tc.wantScreens {
				t.Errorf("screener calls = %d, want %d", scr.calls, tc.wantScreens)
			}
			if len(q.inserted) != tc.wantInsert {
				t.Fatalf("inserted %d flags, want %d", len(q.inserted), tc.wantInsert)
			}
			if tc.wantInsert == 1 {
				want := sqlcgen.InsertReportFlagParams{ReportID: 41, AccountID: self, Reasons: []string{"false", "other"}, Note: tc.wantNote}
				if !reflect.DeepEqual(q.inserted[0], want) {
					t.Errorf("inserted %+v, want %+v", q.inserted[0], want)
				}
			}
		})
	}
}

// TestScreenAbuseSkipsGibberish pins the split of plan 04-31's follow up
// (owner, 2026-10-23): ScreenAbuse, which the Flag note uses, never applies
// the gibberish rule, while CheckText, which the report description uses,
// still does, and both refuse abuse the same way.
func TestScreenAbuseSkipsGibberish(t *testing.T) {
	ctx := context.Background()
	pass := &fakeTextScreener{result: moderation.Result{Model: "m", Scores: map[string]float64{"hate": 0.01}}}
	for _, text := range []string{"fake", "duplicate", "old", "asdfffffffffffff"} {
		if got := service.ScreenAbuse(ctx, pass, text); got.Outcome != service.TextClear {
			t.Errorf("ScreenAbuse(%q) = %+v, want clear", text, got)
		}
		if got := service.ScreenAbuse(ctx, nil, text); got.Outcome != service.TextClear {
			t.Errorf("ScreenAbuse(%q) with no screener = %+v, want clear", text, got)
		}
		if got := service.CheckText(ctx, pass, text); got.Outcome != service.TextRejected || got.Reason != service.RejectGibberish {
			t.Errorf("CheckText(%q) = %+v, want the gibberish reject it always gave", text, got)
		}
	}
	for _, tc := range []struct {
		key  string
		want service.RejectReason
	}{{"hate", service.RejectHateful}, {"harassment", service.RejectHarassing}, {"sexual", service.RejectSexual}} {
		abusive := &fakeTextScreener{result: moderation.Result{Model: "m", Scores: map[string]float64{tc.key: 0.99}}}
		if got := service.ScreenAbuse(ctx, abusive, "fake"); got.Outcome != service.TextRejected || got.Reason != tc.want {
			t.Errorf("ScreenAbuse with %s 0.99 = %+v, want rejected %s", tc.key, got, tc.want)
		}
	}
}
