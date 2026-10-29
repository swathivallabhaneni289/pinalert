package service_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"pinalert/internal/mailer"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// fakeAuthQuerier appends to a shared call log so tests can assert the order
// of the Activity reads directly rather than infer it. Amended 2026-10-29 by
// Phase 7.1 plan 14b: the emailed link and sign in fields and methods are
// gone with the link half of AuthService.
type fakeAuthQuerier struct {
	calls *[]string

	// reportsRows/reportsErr/reportsArg drive ReportsForAccount.
	reportsRows []sqlcgen.ReportsByAccountRow
	reportsErr  error
	reportsArg  *int64

	// votesRows/votesErr/votesCallCount drive CurrentVotesForReports
	// (02-07's ActivityForAccount). votesCallCount lets
	// TestActivityForAccountBatchesVoteReadsOnce assert the exact call
	// count directly rather than inferring it from *calls' length.
	votesRows      []sqlcgen.CurrentVotesForReportsRow
	votesErr       error
	votesCallCount int
	// votesArg is the id slice the last CurrentVotesForReports call received.
	votesArg []int64

	// historyRows/historyErr/historyArgs drive ReporterHistoryReports (plan
	// 03-12). The fake returns only the rows of the requested accounts, like
	// the query, and an empty result when nothing is configured.
	historyRows []sqlcgen.ReporterHistoryReportsRow
	historyErr  error
	historyArgs []sqlcgen.ReporterHistoryReportsParams

	// votedRows/votedErr/votedArgs drive VotedReportsByAccount (quick task
	// 261013-wm5). reporterRows/reporterErr/reporterArgs drive
	// ReporterAccountsForReports for the voted reports; the fake returns only
	// the rows of the requested ids, like the query.
	votedRows    []sqlcgen.VotedReportsByAccountRow
	votedErr     error
	votedArgs    []sqlcgen.VotedReportsByAccountParams
	reporterRows []sqlcgen.ReporterAccountsForReportsRow
	reporterErr  error
	reporterArgs [][]int64

	// undoRows/undoErr/undoArgs drive ResolutionUndoCounts (plan 03.1-07). The
	// fake returns only the rows of the requested ids, like the query.
	undoRows []sqlcgen.ResolutionUndoCountsRow
	undoErr  error
	undoArgs []sqlcgen.ResolutionUndoCountsParams

	// extrasRows/extrasErr/extrasArgs drive ReportExtrasForReports (plan 04-07).
	// Unconfigured (nil extrasRows) the fake answers a clear row per requested
	// id, so every existing test keeps its result.
	extrasRows []sqlcgen.ReportExtrasForReportsRow
	extrasErr  error
	extrasArgs []sqlcgen.ReportExtrasForReportsParams
}

// ReportExtrasForReports records the call. Configured, it returns only the
// rows of the requested ids, like the query's own filter; unconfigured it
// returns one clear row per requested id in request order.
func (f *fakeAuthQuerier) ReportExtrasForReports(ctx context.Context, arg sqlcgen.ReportExtrasForReportsParams) ([]sqlcgen.ReportExtrasForReportsRow, error) {
	*f.calls = append(*f.calls, "ReportExtrasForReports")
	f.extrasArgs = append(f.extrasArgs, arg)
	if f.extrasErr != nil {
		return nil, f.extrasErr
	}
	var out []sqlcgen.ReportExtrasForReportsRow
	if f.extrasRows == nil {
		for _, id := range arg.ReportIds {
			out = append(out, sqlcgen.ReportExtrasForReportsRow{ReportID: id, ModerationState: string(service.ModerationClear)})
		}
		return out, nil
	}
	for _, row := range f.extrasRows {
		if slices.Contains(arg.ReportIds, row.ReportID) {
			out = append(out, row)
		}
	}
	return out, nil
}

// ResolutionUndoCounts records the call and returns the configured rows of the
// requested ids.
func (f *fakeAuthQuerier) ResolutionUndoCounts(ctx context.Context, arg sqlcgen.ResolutionUndoCountsParams) ([]sqlcgen.ResolutionUndoCountsRow, error) {
	*f.calls = append(*f.calls, "ResolutionUndoCounts")
	f.undoArgs = append(f.undoArgs, arg)
	if f.undoErr != nil {
		return nil, f.undoErr
	}
	var out []sqlcgen.ResolutionUndoCountsRow
	for _, row := range f.undoRows {
		if slices.Contains(arg.ReportIds, row.ReportID) {
			out = append(out, row)
		}
	}
	return out, nil
}

// VotedReportsByAccount records the call and returns the configured rows.
func (f *fakeAuthQuerier) VotedReportsByAccount(ctx context.Context, arg sqlcgen.VotedReportsByAccountParams) ([]sqlcgen.VotedReportsByAccountRow, error) {
	*f.calls = append(*f.calls, "VotedReportsByAccount")
	f.votedArgs = append(f.votedArgs, arg)
	if f.votedErr != nil {
		return nil, f.votedErr
	}
	return f.votedRows, nil
}

// ReporterAccountsForReports records the call and returns the configured rows
// of the requested ids.
func (f *fakeAuthQuerier) ReporterAccountsForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.ReporterAccountsForReportsRow, error) {
	*f.calls = append(*f.calls, "ReporterAccountsForReports")
	f.reporterArgs = append(f.reporterArgs, reportIDs)
	if f.reporterErr != nil {
		return nil, f.reporterErr
	}
	var out []sqlcgen.ReporterAccountsForReportsRow
	for _, row := range f.reporterRows {
		if slices.Contains(reportIDs, row.ReportID) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeAuthQuerier) ReportsByAccount(ctx context.Context, accountID *int64) ([]sqlcgen.ReportsByAccountRow, error) {
	*f.calls = append(*f.calls, "ReportsByAccount")
	f.reportsArg = accountID
	if f.reportsErr != nil {
		return nil, f.reportsErr
	}
	return f.reportsRows, nil
}

// CurrentVotesForReports drives 02-07's ActivityForAccount. Its signature
// is copied verbatim from trust.go's VotingQuerier — same generated
// method, a second consumer-defined interface naming it.
func (f *fakeAuthQuerier) CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	*f.calls = append(*f.calls, "CurrentVotesForReports")
	f.votesCallCount++
	f.votesArg = reportIDs
	if f.votesErr != nil {
		return nil, f.votesErr
	}
	return f.votesRows, nil
}

// ReporterHistoryReports records the call and returns the rows of the
// requested accounts, which is the query's own account filter.
func (f *fakeAuthQuerier) ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error) {
	*f.calls = append(*f.calls, "ReporterHistoryReports")
	f.historyArgs = append(f.historyArgs, arg)
	if f.historyErr != nil {
		return nil, f.historyErr
	}
	var out []sqlcgen.ReporterHistoryReportsRow
	for _, row := range f.historyRows {
		if slices.Contains(arg.AccountIds, row.AccountID) {
			out = append(out, row)
		}
	}
	return out, nil
}

type fakeMailer struct {
	calls *[]string
	err   error
}

func (f *fakeMailer) SendVerificationLink(ctx context.Context, email, link string) error {
	*f.calls = append(*f.calls, "send:"+email)
	return f.err
}

func (f *fakeMailer) SendAuthEmail(ctx context.Context, to string, msg mailer.AuthEmail) error {
	*f.calls = append(*f.calls, "auth:"+string(msg.Kind))
	return f.err
}

// newFakes builds the fake querier and mailer on one shared call log.
func newFakes() (*fakeAuthQuerier, *fakeMailer, *[]string) {
	calls := &[]string{}
	return &fakeAuthQuerier{calls: calls}, &fakeMailer{calls: calls}, calls
}

// --- ReportsForAccount ---

// TestReportsForAccountPassesAccountIDAndReturnsRows proves the service
// method forwards accountID as a pointer to the generated querier (the
// underlying column is nullable) and returns exactly the rows the querier
// hands back, unmodified — this is a thin pass-through, not a place any
// filtering/transformation happens.
func TestReportsForAccountPassesAccountIDAndReturnsRows(t *testing.T) {
	q, m, _ := newFakes()
	q.reportsRows = []sqlcgen.ReportsByAccountRow{
		{ID: 1, Category: "flood", Severity: "low", Description: "first"},
		{ID: 2, Category: "fire", Severity: "critical", Description: "second"},
	}
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	rows, err := svc.ReportsForAccount(context.Background(), 42)
	if err != nil {
		t.Fatalf("ReportsForAccount: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if q.reportsArg == nil || *q.reportsArg != 42 {
		t.Fatalf("ReportsByAccount called with accountID = %v, want pointer to 42", q.reportsArg)
	}
}

// TestReportsForAccountWrapsQuerierError proves a querier failure is
// wrapped (not swallowed or mapped to an empty slice), so a caller can
// distinguish "no reports" from "the database read failed".
func TestReportsForAccountWrapsQuerierError(t *testing.T) {
	q, m, _ := newFakes()
	q.reportsErr = errors.New("connection reset")
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	_, err := svc.ReportsForAccount(context.Background(), 7)
	if err == nil {
		t.Fatal("expected an error when the querier fails")
	}
	if !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("error %q does not wrap the underlying querier error", err.Error())
	}
}

// --- ActivityForAccount (02-07 Task 2) ---

// TestActivityForAccountResolvesVisibility proves the Activity page's
// visibility comes from the same service.Resolve every other read path
// calls: a report carrying a standing reporter resolve vote comes back
// retracted/resolved, and a report with no votes at all comes back
// provisional/awaiting-confirmation.
func TestActivityForAccountResolvesVisibility(t *testing.T) {
	q, m, _ := newFakes()
	future := time.Now().Add(1 * time.Hour)
	q.reportsRows = []sqlcgen.ReportsByAccountRow{
		{ID: 1, Category: "flood", Severity: "low", Description: "resolved report", ExpiresAt: future},
		{ID: 2, Category: "flood", Severity: "low", Description: "untouched report", ExpiresAt: future},
	}
	q.votesRows = []sqlcgen.CurrentVotesForReportsRow{
		{ReportID: 1, AccountID: 99, Kind: "resolution", Value: "resolve", GeohashCell: "abc"},
	}
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	reports, err := svc.ActivityForAccount(context.Background(), 99)
	if err != nil {
		t.Fatalf("ActivityForAccount: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("len(reports) = %d, want 2", len(reports))
	}

	var resolved, untouched *service.ActivityReport
	for i := range reports {
		switch reports[i].Row.ID {
		case 1:
			resolved = &reports[i]
		case 2:
			untouched = &reports[i]
		}
	}
	if resolved == nil || untouched == nil {
		t.Fatalf("expected both report ids 1 and 2 in the result, got %+v", reports)
	}
	if resolved.Visibility != service.VisibilityRetracted {
		t.Errorf("report 1 visibility = %v, want %v (the reporter's own resolve vote, D-13)",
			resolved.Visibility, service.VisibilityRetracted)
	}
	if resolved.Reason != service.ReasonResolved {
		t.Errorf("report 1 reason = %v, want %v", resolved.Reason, service.ReasonResolved)
	}
	if untouched.Visibility != service.VisibilityProvisional {
		t.Errorf("report 2 (no votes) visibility = %v, want %v", untouched.Visibility, service.VisibilityProvisional)
	}
	if untouched.Reason != service.ReasonAwaitingConfirmation {
		t.Errorf("report 2 reason = %v, want %v", untouched.Reason, service.ReasonAwaitingConfirmation)
	}
}

// TestActivityForAccountBatchesVoteReadsOnce proves N reports cost exactly
// one CurrentVotesForReports call, and an account with no reports at all
// costs zero — the same batched shape 02-04's Nearby uses, never an N+1.
func TestActivityForAccountBatchesVoteReadsOnce(t *testing.T) {
	q, m, _ := newFakes()
	future := time.Now().Add(1 * time.Hour)
	q.reportsRows = []sqlcgen.ReportsByAccountRow{
		{ID: 1, Category: "flood", Severity: "low", ExpiresAt: future},
		{ID: 2, Category: "fire", Severity: "medium", ExpiresAt: future},
		{ID: 3, Category: "other", Severity: "critical", ExpiresAt: future},
	}
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	if _, err := svc.ActivityForAccount(context.Background(), 42); err != nil {
		t.Fatalf("ActivityForAccount: %v", err)
	}
	if q.votesCallCount != 1 {
		t.Fatalf("CurrentVotesForReports called %d times for 3 reports, want exactly 1", q.votesCallCount)
	}

	q2, m2, _ := newFakes()
	svc2 := service.NewAuthService(q2, m2, "https://pinalert.example")
	reports, err := svc2.ActivityForAccount(context.Background(), 42)
	if err != nil {
		t.Fatalf("ActivityForAccount (no reports): %v", err)
	}
	if len(reports) != 0 {
		t.Fatalf("expected an empty slice for an account with no reports, got %d", len(reports))
	}
	if q2.votesCallCount != 0 {
		t.Fatalf("CurrentVotesForReports called %d times for an account with no reports, want 0", q2.votesCallCount)
	}
}

// TestActivityForAccountValidatesCategoryAndSeverityOnce proves category
// and severity are validated exactly once, in the service, with the
// canonical fallbacks landing on BOTH the returned struct and the values
// the resolver saw — proven by a garbage severity NOT triggering the
// critical bypass, which is what would happen if the resolver saw a wrong
// (or a second, disagreeing) fallback.
func TestActivityForAccountValidatesCategoryAndSeverityOnce(t *testing.T) {
	q, m, _ := newFakes()
	q.reportsRows = []sqlcgen.ReportsByAccountRow{
		{ID: 1, Category: "not-a-real-category", Severity: "not-a-real-severity", ExpiresAt: time.Now().Add(1 * time.Hour)},
	}
	svc := service.NewAuthService(q, m, "https://pinalert.example")

	reports, err := svc.ActivityForAccount(context.Background(), 7)
	if err != nil {
		t.Fatalf("ActivityForAccount: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("len(reports) = %d, want 1", len(reports))
	}
	got := reports[0]
	if got.Category != service.CategoryOther {
		t.Errorf("Category = %v, want the canonical fallback %v", got.Category, service.CategoryOther)
	}
	if got.Severity != service.SeverityLow {
		t.Errorf("Severity = %v, want the canonical fallback %v", got.Severity, service.SeverityLow)
	}
	if got.Visibility != service.VisibilityProvisional {
		t.Errorf("Visibility = %v, want %v — a garbage severity must fall back to low, not "+
			"critical, or the resolver would wrongly bypass the confirmation gate",
			got.Visibility, service.VisibilityProvisional)
	}
}

// TestActivityCarriesModeration is R-01 and D-02 on the Activity read: each row
// carries its stored state, a withheld row has visibility withheld and a zero
// trust object, a pending serious row is public and keeps its trust, the
// display only written time rides along, and a row the extras read did not
// return is dropped.
func TestActivityCarriesModeration(t *testing.T) {
	ctx := context.Background()
	const account int64 = 42

	rowOf := func(id int64, severity service.Severity, category service.Category) sqlcgen.ReportsByAccountRow {
		r := p12ActivityRow(id)
		r.Severity = string(severity)
		r.Category = string(category)
		return r
	}
	written := p12Now.Add(-time.Hour).Add(-4 * time.Hour)

	newFake := func() (*fakeAuthQuerier, *fakeMailer, *[]string) {
		q, m, calls := newFakes()
		q.reportsRows = []sqlcgen.ReportsByAccountRow{
			rowOf(1, service.SeverityLow, service.CategoryFlood),
			rowOf(2, service.SeverityCritical, service.CategoryFlood),
			rowOf(3, service.SeverityLow, service.CategoryFlood),
			rowOf(4, service.SeverityCritical, service.CategoryFlood),
			rowOf(5, service.SeverityLow, service.CategoryRescueNeeded),
			rowOf(6, service.SeverityLow, service.CategoryFlood),
			rowOf(7, service.SeverityLow, service.CategoryFlood),
		}
		q.extrasRows = []sqlcgen.ReportExtrasForReportsRow{
			{ReportID: 1, ModerationState: string(service.ModerationRemoved)},
			{ReportID: 2, ModerationState: string(service.ModerationRemoved)},
			{ReportID: 3, ModerationState: string(service.ModerationPending)},
			{ReportID: 4, ModerationState: string(service.ModerationPending)},
			{ReportID: 5, ModerationState: string(service.ModerationPending)},
			{ReportID: 6, ModerationState: string(service.ModerationClear), ClientWrittenAt: pgtype.Timestamptz{Time: written, Valid: true}},
		}
		return q, m, calls
	}
	svcOf := func(q *fakeAuthQuerier, m *fakeMailer) *service.AuthService {
		return service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(p12Clock))
	}

	t.Run("rows carry state, visibility and trust", func(t *testing.T) {
		q, m, calls := newFake()
		acts, err := svcOf(q, m).ActivityForAccount(ctx, account)
		if err != nil {
			t.Fatalf("ActivityForAccount: %v", err)
		}
		ids := make([]int64, 0, len(acts))
		byID := map[int64]service.ActivityReport{}
		for _, a := range acts {
			ids = append(ids, a.Row.ID)
			byID[a.Row.ID] = a
		}
		if !equalIDs(ids, []int64{1, 2, 3, 4, 5, 6}) {
			t.Fatalf("ids = %v, want [1 2 3 4 5 6]", ids)
		}

		zero := service.Trust{}
		for id, want := range map[int64]struct {
			mod    service.ModerationState
			reason service.ResolveReason
		}{
			1: {service.ModerationRemoved, service.ReasonRemovedAfterReview},
			2: {service.ModerationRemoved, service.ReasonRemovedAfterReview},
			3: {service.ModerationPending, service.ReasonAwaitingTextCheck},
		} {
			a := byID[id]
			if a.Moderation != want.mod || a.Visibility != service.VisibilityWithheld || a.Reason != want.reason {
				t.Errorf("row %d = %s/%s/%s, want %s/withheld/%s", id, a.Moderation, a.Visibility, a.Reason, want.mod, want.reason)
			}
			if !reflect.DeepEqual(a.Trust, zero) {
				t.Errorf("row %d trust = %+v, want the zero Trust", id, a.Trust)
			}
		}
		for _, id := range []int64{4, 5} {
			a := byID[id]
			if a.Moderation != service.ModerationPending || a.Visibility != service.VisibilityLive || a.Reason != service.ReasonCriticalBypass {
				t.Errorf("row %d = %s/%s/%s, want pending/live/critical_bypass", id, a.Moderation, a.Visibility, a.Reason)
			}
			if reflect.DeepEqual(a.Trust, zero) {
				t.Errorf("row %d trust is the zero Trust, want a full trust object", id)
			}
		}
		a6 := byID[6]
		if a6.Moderation != service.ModerationClear {
			t.Errorf("row 6 Moderation = %s, want clear", a6.Moderation)
		}
		if a6.WrittenAt == nil || !a6.WrittenAt.Equal(written) {
			t.Errorf("row 6 WrittenAt = %v, want %v", a6.WrittenAt, written)
		}
		if reflect.DeepEqual(a6.Trust, zero) {
			t.Error("row 6 trust is the zero Trust, want a full trust object")
		}
		if byID[4].WrittenAt != nil {
			t.Errorf("row 4 WrittenAt = %v, want nil", byID[4].WrittenAt)
		}

		if n := slices.Index(*calls, "ReportExtrasForReports"); n != len(*calls)-1 || n < 1 || (*calls)[n-1] != "CurrentVotesForReports" {
			t.Errorf("calls = %v, want one ReportExtrasForReports right after CurrentVotesForReports", *calls)
		}
		if len(q.extrasArgs) != 1 || q.extrasArgs[0].ViewerAccountID != account || !equalIDs(q.extrasArgs[0].ReportIds, []int64{1, 2, 3, 4, 5, 6, 7}) {
			t.Errorf("extrasArgs = %+v, want one call for account 42 over the own report ids", q.extrasArgs)
		}
	})

	t.Run("an account with no reports and no votes makes no extras call", func(t *testing.T) {
		q, m, _ := newFakes()
		if _, err := svcOf(q, m).ActivityForAccount(ctx, account); err != nil {
			t.Fatalf("ActivityForAccount: %v", err)
		}
		if len(q.extrasArgs) != 0 {
			t.Errorf("extras called %d times, want 0", len(q.extrasArgs))
		}
	})

	t.Run("an extras error is wrapped", func(t *testing.T) {
		boom := errors.New("extras unavailable")
		q, m, _ := newFake()
		q.extrasErr = boom
		if _, err := svcOf(q, m).ActivityForAccount(ctx, account); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want it to wrap %v", err, boom)
		}
	})

	t.Run("a voted report that resolves to withheld is dropped and the ids follow the posts", func(t *testing.T) {
		q, m, _ := newFakes()
		q.reportsRows = []sqlcgen.ReportsByAccountRow{p12ActivityRow(1)}
		q.votedRows = []sqlcgen.VotedReportsByAccountRow{
			wm5VotedRow(10, service.VoteConfirm, p12Now.Add(-10*time.Minute)),
			wm5VotedRow(11, service.VoteConfirm, p12Now.Add(-20*time.Minute)),
		}
		q.reporterRows = []sqlcgen.ReporterAccountsForReportsRow{reporterRow(10, ptrID(7)), reporterRow(11, ptrID(8))}
		q.extrasRows = []sqlcgen.ReportExtrasForReportsRow{
			{ReportID: 1, ModerationState: string(service.ModerationClear)},
			{ReportID: 10, ModerationState: string(service.ModerationRemoved)},
			{ReportID: 11, ModerationState: string(service.ModerationClear)},
		}
		page, err := svcOf(q, m).ActivityPageForAccount(ctx, account)
		if err != nil {
			t.Fatalf("ActivityPageForAccount: %v", err)
		}
		if len(page.Posts) != 1 {
			t.Errorf("posts = %d, want 1", len(page.Posts))
		}
		if len(page.Votes) != 1 || page.Votes[0].Row.ID != 11 {
			t.Errorf("votes = %+v, want only report 11", page.Votes)
		}
		if len(q.extrasArgs) != 1 || !equalIDs(q.extrasArgs[0].ReportIds, []int64{1, 10, 11}) {
			t.Errorf("extrasArgs = %+v, want one call over [1 10 11]", q.extrasArgs)
		}
	})
}

// TestActivityForAccountReadsReporterHistoryOnce proves the Activity page reads
// the account's history once, keyed by the gate's account id, folds the history
// ids into its one vote read, and gives every row the account's signal.
func TestActivityForAccountReadsReporterHistoryOnce(t *testing.T) {
	ctx := context.Background()
	const account int64 = 42

	t.Run("one history call and one vote call over own and history ids", func(t *testing.T) {
		q, m, calls := newFakes()
		q.reportsRows = []sqlcgen.ReportsByAccountRow{p12ActivityRow(1), p12ActivityRow(2), p12ActivityRow(3)}
		history, votes := p12Corroborated(account, 4, 5)
		q.historyRows = history
		q.votesRows = votes
		svc := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(p12Clock))

		acts, err := svc.ActivityForAccount(ctx, account)
		if err != nil {
			t.Fatalf("ActivityForAccount: %v", err)
		}
		// 261013-wm5: the page now reads the account's votes in the same pass,
		// so VotedReportsByAccount comes second. Every other assertion is kept.
		wantCalls := []string{"ReportsByAccount", "VotedReportsByAccount", "ReporterHistoryReports", "CurrentVotesForReports", "ReportExtrasForReports"}
		if !slices.Equal(*calls, wantCalls) {
			t.Fatalf("calls = %v, want %v", *calls, wantCalls)
		}
		wantArgs := sqlcgen.ReporterHistoryReportsParams{
			Now:         p12Now,
			WindowStart: p12Now.Add(-service.ReliabilityWindow),
			RowCap:      int32(service.ReliabilityHistoryReadCap),
			AccountIds:  []int64{account},
		}
		if len(q.historyArgs) != 1 || !reflect.DeepEqual(q.historyArgs[0], wantArgs) {
			t.Errorf("historyArgs = %+v, want one entry %+v", q.historyArgs, wantArgs)
		}
		if want := []int64{1, 2, 3, 4, 5, 6, 7, 8}; !slices.Equal(q.votesArg, want) {
			t.Errorf("votesArg = %v, want %v", q.votesArg, want)
		}
		if q.votesCallCount != 1 {
			t.Errorf("votesCallCount = %d, want 1", q.votesCallCount)
		}
		want := service.ReporterSignals([]int64{account}, history, votes, p12Now)[account]
		if want.Tag != service.ReporterReliable {
			t.Fatalf("expected signal tag = %q, want reliable", want.Tag)
		}
		if len(acts) != 3 {
			t.Fatalf("got %d rows, want 3", len(acts))
		}
		for _, a := range acts {
			if !reflect.DeepEqual(a.Trust.Reporter, want) {
				t.Errorf("row %d reporter = %+v, want %+v", a.Row.ID, a.Trust.Reporter, want)
			}
		}
	})

	t.Run("an account with no reports reads no history", func(t *testing.T) {
		q, m, calls := newFakes()
		svc := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(p12Clock))

		acts, err := svc.ActivityForAccount(ctx, account)
		if err != nil {
			t.Fatalf("ActivityForAccount: %v", err)
		}
		if len(acts) != 0 {
			t.Errorf("got %d rows, want 0", len(acts))
		}
		if len(q.historyArgs) != 0 {
			t.Errorf("history called %d times, want 0 (%v)", len(q.historyArgs), *calls)
		}
	})

	t.Run("a failed history read is wrapped and no votes are read", func(t *testing.T) {
		boom := errors.New("history unavailable")
		q, m, _ := newFakes()
		q.reportsRows = []sqlcgen.ReportsByAccountRow{p12ActivityRow(1)}
		q.historyErr = boom
		svc := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(p12Clock))

		if _, err := svc.ActivityForAccount(ctx, account); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want it to wrap %v", err, boom)
		}
		if q.votesCallCount != 0 {
			t.Errorf("votesCallCount = %d, want 0", q.votesCallCount)
		}
	})
}
