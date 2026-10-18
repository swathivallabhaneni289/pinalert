// updatesservice_test.go proves UpdatesService (plan 03.2-03) with a fake
// querier and no database: Refresh turns the resolver's current answer into at
// most one stored update per report per hour, silently on first sight and on
// the reader's own action, and List writes every word at read time.

package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

// fakeUpdatesCall is one recorded querier call: its name, whether it carried an
// account id and which, and its params.
type fakeUpdatesCall struct {
	Name    string
	HasAcct bool
	Acct    int64
	Params  any
}

// fakeUpdatesQuerier records every call in order and returns canned rows. It
// also implements the alert methods of plan 13 with zero values.
type fakeUpdatesQuerier struct {
	calls []fakeUpdatesCall

	followed  []sqlcgen.FollowedOpenReportsRow
	cursors   []sqlcgen.UpdateCursorsForReportsRow
	votes     []sqlcgen.CurrentVotesForReportsRow
	reporters []sqlcgen.ReporterAccountsForReportsRow
	updates   []sqlcgen.UpdatesForAccountRow
	reports   []sqlcgen.ReportsForUpdatesRow

	unread, total int64
	markedRows    int64

	advanceErr  error
	fadingErr   error
	followedErr error
}

func (f *fakeUpdatesQuerier) rec(name string, acct int64, hasAcct bool, params any) {
	f.calls = append(f.calls, fakeUpdatesCall{Name: name, HasAcct: hasAcct, Acct: acct, Params: params})
}

// named returns the recorded calls with this name, in order.
func (f *fakeUpdatesQuerier) named(name string) []fakeUpdatesCall {
	var out []fakeUpdatesCall
	for _, c := range f.calls {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

// names returns every recorded call name in order.
func (f *fakeUpdatesQuerier) names() []string {
	out := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		out = append(out, c.Name)
	}
	return out
}

func (f *fakeUpdatesQuerier) FollowedOpenReports(_ context.Context, a sqlcgen.FollowedOpenReportsParams) ([]sqlcgen.FollowedOpenReportsRow, error) {
	f.rec("FollowedOpenReports", a.AccountID, true, a)
	return f.followed, f.followedErr
}

func (f *fakeUpdatesQuerier) UpdateCursorsForReports(_ context.Context, a sqlcgen.UpdateCursorsForReportsParams) ([]sqlcgen.UpdateCursorsForReportsRow, error) {
	f.rec("UpdateCursorsForReports", a.AccountID, true, a)
	return f.cursors, nil
}

func (f *fakeUpdatesQuerier) InsertBaselineCursor(_ context.Context, a sqlcgen.InsertBaselineCursorParams) error {
	f.rec("InsertBaselineCursor", a.AccountID, true, a)
	return nil
}

func (f *fakeUpdatesQuerier) AdoptCursorState(_ context.Context, a sqlcgen.AdoptCursorStateParams) (int64, error) {
	f.rec("AdoptCursorState", a.AccountID, true, a)
	return 1, nil
}

func (f *fakeUpdatesQuerier) AdvanceCursorAndInsertUpdate(_ context.Context, a sqlcgen.AdvanceCursorAndInsertUpdateParams) (int64, error) {
	f.rec("AdvanceCursorAndInsertUpdate", a.AccountID, true, a)
	if f.advanceErr != nil {
		return 0, f.advanceErr
	}
	return 1, nil
}

func (f *fakeUpdatesQuerier) MarkFadingAndInsertUpdate(_ context.Context, a sqlcgen.MarkFadingAndInsertUpdateParams) (int64, error) {
	f.rec("MarkFadingAndInsertUpdate", a.AccountID, true, a)
	if f.fadingErr != nil {
		return 0, f.fadingErr
	}
	return 1, nil
}

func (f *fakeUpdatesQuerier) PruneUpdates(_ context.Context, a sqlcgen.PruneUpdatesParams) error {
	f.rec("PruneUpdates", a.AccountID, true, a)
	return nil
}

func (f *fakeUpdatesQuerier) PruneCursors(_ context.Context, a sqlcgen.PruneCursorsParams) error {
	f.rec("PruneCursors", a.AccountID, true, a)
	return nil
}

func (f *fakeUpdatesQuerier) UpdatesForAccount(_ context.Context, a sqlcgen.UpdatesForAccountParams) ([]sqlcgen.UpdatesForAccountRow, error) {
	f.rec("UpdatesForAccount", a.AccountID, true, a)
	return f.updates, nil
}

func (f *fakeUpdatesQuerier) CountUpdatesForAccount(_ context.Context, a sqlcgen.CountUpdatesForAccountParams) (int64, error) {
	f.rec("CountUpdatesForAccount", a.AccountID, true, a)
	return f.total, nil
}

func (f *fakeUpdatesQuerier) UnreadUpdateCount(_ context.Context, a sqlcgen.UnreadUpdateCountParams) (int64, error) {
	f.rec("UnreadUpdateCount", a.AccountID, true, a)
	return f.unread, nil
}

func (f *fakeUpdatesQuerier) MarkUpdatesRead(_ context.Context, a sqlcgen.MarkUpdatesReadParams) (int64, error) {
	f.rec("MarkUpdatesRead", a.AccountID, true, a)
	return f.markedRows, nil
}

func (f *fakeUpdatesQuerier) ReportsForUpdates(_ context.Context, ids []int64) ([]sqlcgen.ReportsForUpdatesRow, error) {
	f.rec("ReportsForUpdates", 0, false, ids)
	return f.reports, nil
}

func (f *fakeUpdatesQuerier) CurrentVotesForReports(_ context.Context, ids []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	f.rec("CurrentVotesForReports", 0, false, ids)
	return f.votes, nil
}

func (f *fakeUpdatesQuerier) ReporterAccountsForReports(_ context.Context, ids []int64) ([]sqlcgen.ReporterAccountsForReportsRow, error) {
	f.rec("ReporterAccountsForReports", 0, false, ids)
	return f.reporters, nil
}

func (f *fakeUpdatesQuerier) GetAccountHomeCell(_ context.Context, id int64) (*string, error) {
	f.rec("GetAccountHomeCell", id, true, nil)
	return nil, nil
}

func (f *fakeUpdatesQuerier) GetAlertSettings(_ context.Context, id int64) (sqlcgen.GetAlertSettingsRow, error) {
	f.rec("GetAlertSettings", id, true, nil)
	return sqlcgen.GetAlertSettingsRow{}, nil
}

func (f *fakeUpdatesQuerier) AlertsFromReporterInWindow(_ context.Context, a sqlcgen.AlertsFromReporterInWindowParams) (int64, error) {
	f.rec("AlertsFromReporterInWindow", a.RecipientID, true, a)
	return 0, nil
}

func (f *fakeUpdatesQuerier) InsertReliableAlert(_ context.Context, a sqlcgen.InsertReliableAlertParams) (int64, error) {
	f.rec("InsertReliableAlert", a.AccountID, true, a)
	return 0, nil
}

// newTestUpdatesService builds a service over q with the clock pinned.
func newTestUpdatesService(q UpdatesQuerier) *UpdatesService {
	return NewUpdatesService(q, nil, WithUpdatesClock(func() time.Time { return updatesNow }))
}

func pgTS(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// followedRow builds one followed report row.
func followedRow(id int64, sev Severity, cat Category, expires time.Time, mod ModerationState, muted bool) sqlcgen.FollowedOpenReportsRow {
	return sqlcgen.FollowedOpenReportsRow{
		ID: id, Severity: string(sev), Category: string(cat),
		CreatedAt: updatesNow.Add(-3 * time.Hour), ExpiresAt: expires,
		ModerationState: string(mod), Muted: muted,
	}
}

// confirmVote is one content confirm vote by another account from a cell.
func confirmVote(reportID, account int64, cell string, at time.Time) sqlcgen.CurrentVotesForReportsRow {
	return sqlcgen.CurrentVotesForReportsRow{
		ReportID: reportID, AccountID: account, Kind: string(VoteKindContent),
		Value: string(VoteConfirm), GeohashCell: cell, CreatedAt: at,
	}
}

func cursorRow(reportID int64, state Visibility, lastUpdate *time.Time, adopted time.Time, fadingFor *time.Time) sqlcgen.UpdateCursorsForReportsRow {
	row := sqlcgen.UpdateCursorsForReportsRow{ReportID: reportID, LastState: string(state), AdoptedAt: adopted}
	if lastUpdate != nil {
		row.LastUpdateAt = pgTS(*lastUpdate)
	}
	if fadingFor != nil {
		row.FadingFor = pgTS(*fadingFor)
	}
	return row
}

func TestRefreshBaselinesSilently(t *testing.T) {
	f := &fakeUpdatesQuerier{
		followed: []sqlcgen.FollowedOpenReportsRow{
			followedRow(1, SeverityLow, CategoryFlood, updatesNow.Add(5*time.Hour), ModerationClear, false),
		},
	}
	if err := newTestUpdatesService(f).Refresh(context.Background(), 7); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	base := f.named("InsertBaselineCursor")
	if len(base) != 1 {
		t.Fatalf("InsertBaselineCursor calls = %d, want 1 (calls %v)", len(base), f.names())
	}
	p := base[0].Params.(sqlcgen.InsertBaselineCursorParams)
	if p.AccountID != 7 || p.ReportID != 1 || p.State != string(VisibilityProvisional) || !p.Now.Equal(updatesNow) {
		t.Errorf("baseline params = %+v", p)
	}
	for _, n := range []string{"AdvanceCursorAndInsertUpdate", "MarkFadingAndInsertUpdate", "AdoptCursorState"} {
		if got := f.named(n); len(got) != 0 {
			t.Errorf("%s was called %d times on a first look", n, len(got))
		}
	}
}

func TestRefreshAnnouncesStateChange(t *testing.T) {
	adopted := updatesNow.Add(-5 * time.Hour)
	f := &fakeUpdatesQuerier{
		followed: []sqlcgen.FollowedOpenReportsRow{
			followedRow(1, SeverityLow, CategoryFlood, updatesNow.Add(5*time.Hour), ModerationClear, false),
		},
		cursors: []sqlcgen.UpdateCursorsForReportsRow{cursorRow(1, VisibilityProvisional, nil, adopted, nil)},
		votes: []sqlcgen.CurrentVotesForReportsRow{
			confirmVote(1, 20, "cell-a", updatesNow.Add(-30*time.Minute)),
			confirmVote(1, 21, "cell-b", updatesNow.Add(-20*time.Minute)),
		},
		reporters: []sqlcgen.ReporterAccountsForReportsRow{{ReportID: 1}},
	}
	if err := newTestUpdatesService(f).Refresh(context.Background(), 7); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	adv := f.named("AdvanceCursorAndInsertUpdate")
	if len(adv) != 1 {
		t.Fatalf("AdvanceCursorAndInsertUpdate calls = %d, want 1 (calls %v)", len(adv), f.names())
	}
	p := adv[0].Params.(sqlcgen.AdvanceCursorAndInsertUpdateParams)
	if p.AccountID != 7 || p.ReportID != 1 {
		t.Errorf("ids = %d/%d", p.AccountID, p.ReportID)
	}
	if p.OldState != string(VisibilityProvisional) || p.NewState != string(VisibilityLive) || p.Kind != string(UpdateReachedLive) {
		t.Errorf("states = %q -> %q kind %q", p.OldState, p.NewState, p.Kind)
	}
	if !p.Now.Equal(updatesNow) || !p.GapEdge.Equal(updatesNow.Add(-time.Hour)) || !p.HourEdge.Equal(updatesNow.Add(-time.Hour)) {
		t.Errorf("times = now %v gap %v hour %v", p.Now, p.GapEdge, p.HourEdge)
	}
	if !p.BucketStart.Equal(HourBucket(updatesNow)) || p.Ceiling != UpdatesHourlyCeiling {
		t.Errorf("bucket %v ceiling %d", p.BucketStart, p.Ceiling)
	}
}

func TestRefreshOwnActionAdopts(t *testing.T) {
	adopted := updatesNow.Add(-5 * time.Hour)
	f := &fakeUpdatesQuerier{
		// Critical reports resolve Live with no votes.
		followed: []sqlcgen.FollowedOpenReportsRow{
			followedRow(2, SeverityCritical, CategoryFlood, updatesNow.Add(5*time.Hour), ModerationClear, false),
		},
		cursors: []sqlcgen.UpdateCursorsForReportsRow{cursorRow(2, VisibilityProvisional, nil, adopted, nil)},
		votes: []sqlcgen.CurrentVotesForReportsRow{
			confirmVote(2, 7, "cell-own", updatesNow.Add(-time.Minute)),
		},
	}
	if err := newTestUpdatesService(f).Refresh(context.Background(), 7); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	ad := f.named("AdoptCursorState")
	if len(ad) != 1 {
		t.Fatalf("AdoptCursorState calls = %d, want 1 (calls %v)", len(ad), f.names())
	}
	p := ad[0].Params.(sqlcgen.AdoptCursorStateParams)
	if p.OldState != string(VisibilityProvisional) || p.NewState != string(VisibilityLive) || p.AccountID != 7 || p.ReportID != 2 {
		t.Errorf("adopt params = %+v", p)
	}
	if len(f.named("AdvanceCursorAndInsertUpdate")) != 0 {
		t.Error("the reader's own action produced an update")
	}
}

func TestRefreshSkipsMutedAndWithheld(t *testing.T) {
	adopted := updatesNow.Add(-5 * time.Hour)
	exp := updatesNow.Add(5 * time.Hour)
	f := &fakeUpdatesQuerier{
		followed: []sqlcgen.FollowedOpenReportsRow{
			followedRow(1, SeverityCritical, CategoryFlood, exp, ModerationClear, true),   // muted
			followedRow(2, SeverityLow, CategoryFlood, exp, ModerationRemoved, false),     // removed
			followedRow(3, SeverityLow, CategoryFlood, exp, ModerationPending, false),     // held
			followedRow(4, SeverityCritical, CategoryFlood, exp, ModerationRemoved, true), // both
		},
		cursors: []sqlcgen.UpdateCursorsForReportsRow{
			cursorRow(1, VisibilityProvisional, nil, adopted, nil),
			cursorRow(2, VisibilityProvisional, nil, adopted, nil),
			cursorRow(3, VisibilityLive, nil, adopted, nil),
		},
	}
	if err := newTestUpdatesService(f).Refresh(context.Background(), 7); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	for _, n := range []string{"InsertBaselineCursor", "AdoptCursorState", "AdvanceCursorAndInsertUpdate", "MarkFadingAndInsertUpdate"} {
		if got := f.named(n); len(got) != 0 {
			t.Errorf("%s was called %d times for muted or withheld reports", n, len(got))
		}
	}
}

func TestRefreshFading(t *testing.T) {
	expires := updatesNow.Add(40 * time.Minute)
	last := updatesNow.Add(-2 * time.Hour)
	f := &fakeUpdatesQuerier{
		followed: []sqlcgen.FollowedOpenReportsRow{
			followedRow(5, SeverityCritical, CategoryFlood, expires, ModerationClear, false),
		},
		cursors: []sqlcgen.UpdateCursorsForReportsRow{cursorRow(5, VisibilityLive, &last, updatesNow.Add(-6*time.Hour), nil)},
	}
	if err := newTestUpdatesService(f).Refresh(context.Background(), 7); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	fd := f.named("MarkFadingAndInsertUpdate")
	if len(fd) != 1 {
		t.Fatalf("MarkFadingAndInsertUpdate calls = %d, want 1 (calls %v)", len(fd), f.names())
	}
	p := fd[0].Params.(sqlcgen.MarkFadingAndInsertUpdateParams)
	if !p.ExpiresAt.Equal(expires) || p.AccountID != 7 || p.ReportID != 5 {
		t.Errorf("fading params = %+v", p)
	}
	if !p.GapEdge.Equal(updatesNow.Add(-time.Hour)) || !p.HourEdge.Equal(updatesNow.Add(-time.Hour)) ||
		!p.BucketStart.Equal(HourBucket(updatesNow)) || p.Ceiling != UpdatesHourlyCeiling {
		t.Errorf("fading edges = %+v", p)
	}
	if len(f.named("AdvanceCursorAndInsertUpdate")) != 0 {
		t.Error("a fading change also wrote a state event")
	}
}

func TestRefreshPrunesFirst(t *testing.T) {
	f := &fakeUpdatesQuerier{}
	if err := newTestUpdatesService(f).Refresh(context.Background(), 7); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	names := f.names()
	if len(names) < 3 || names[0] != "PruneUpdates" || names[1] != "PruneCursors" || names[2] != "FollowedOpenReports" {
		t.Fatalf("call order = %v, want PruneUpdates, PruneCursors, FollowedOpenReports first", names)
	}
	edge := updatesNow.Add(-UpdatesRetention)
	pu := f.calls[0].Params.(sqlcgen.PruneUpdatesParams)
	pc := f.calls[1].Params.(sqlcgen.PruneCursorsParams)
	if !pu.Edge.Equal(edge) || !pc.Edge.Equal(edge) {
		t.Errorf("prune edges = %v and %v, want %v", pu.Edge, pc.Edge, edge)
	}
	fo := f.calls[2].Params.(sqlcgen.FollowedOpenReportsParams)
	if fo.RowCap != FollowedReportsCap || !fo.Now.Equal(updatesNow) {
		t.Errorf("followed params = %+v", fo)
	}
	// An empty followed set stops before the batched reads.
	for _, n := range []string{"CurrentVotesForReports", "ReporterAccountsForReports", "UpdateCursorsForReports"} {
		if got := f.named(n); len(got) != 0 {
			t.Errorf("%s ran for an empty followed set", n)
		}
	}
}

func TestRefreshKeyedByAccountOnly(t *testing.T) {
	adopted := updatesNow.Add(-5 * time.Hour)
	last := updatesNow.Add(-2 * time.Hour)
	exp := updatesNow.Add(40 * time.Minute)
	f := &fakeUpdatesQuerier{
		followed: []sqlcgen.FollowedOpenReportsRow{
			followedRow(1, SeverityLow, CategoryFlood, updatesNow.Add(5*time.Hour), ModerationClear, false),
			followedRow(2, SeverityCritical, CategoryFlood, exp, ModerationClear, false),
			followedRow(3, SeverityCritical, CategoryFlood, updatesNow.Add(5*time.Hour), ModerationClear, false),
		},
		cursors: []sqlcgen.UpdateCursorsForReportsRow{
			cursorRow(2, VisibilityLive, &last, adopted, nil),
			cursorRow(3, VisibilityProvisional, nil, adopted, nil),
		},
		votes: []sqlcgen.CurrentVotesForReportsRow{confirmVote(3, 4242, "cell-x", updatesNow.Add(-time.Hour))},
	}
	const account = 91
	if err := newTestUpdatesService(f).Refresh(context.Background(), account); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	seen := map[string]bool{}
	for _, c := range f.calls {
		seen[c.Name] = true
		if c.HasAcct && c.Acct != account {
			t.Errorf("%s was keyed by account %d, want %d", c.Name, c.Acct, account)
		}
	}
	for _, n := range []string{"PruneUpdates", "InsertBaselineCursor", "MarkFadingAndInsertUpdate", "AdvanceCursorAndInsertUpdate"} {
		if !seen[n] {
			t.Errorf("scenario never reached %s, so the check proves less than it should", n)
		}
	}
}

func TestRefreshNoRowsIsNotAnError(t *testing.T) {
	adopted := updatesNow.Add(-5 * time.Hour)
	last := updatesNow.Add(-2 * time.Hour)
	f := &fakeUpdatesQuerier{
		followed: []sqlcgen.FollowedOpenReportsRow{
			followedRow(1, SeverityCritical, CategoryFlood, updatesNow.Add(5*time.Hour), ModerationClear, false),
			followedRow(2, SeverityCritical, CategoryFlood, updatesNow.Add(40*time.Minute), ModerationClear, false),
		},
		cursors: []sqlcgen.UpdateCursorsForReportsRow{
			cursorRow(1, VisibilityProvisional, nil, adopted, nil),
			cursorRow(2, VisibilityLive, &last, adopted, nil),
		},
		advanceErr: pgx.ErrNoRows,
		fadingErr:  pgx.ErrNoRows,
	}
	if err := newTestUpdatesService(f).Refresh(context.Background(), 7); err != nil {
		t.Fatalf("Refresh returned %v for a lost insert race", err)
	}

	f2 := &fakeUpdatesQuerier{
		followed:   f.followed,
		cursors:    f.cursors,
		advanceErr: errors.New("connection reset by peer"),
	}
	err := newTestUpdatesService(f2).Refresh(context.Background(), 7)
	if err == nil || !strings.Contains(err.Error(), "service: Refresh") {
		t.Fatalf("Refresh error = %v, want a wrapped service: Refresh error", err)
	}
}

func TestUpdatesServiceHasNoTimer(t *testing.T) {
	// Time events are evaluated lazily on each refresh (D-12): the service file
	// holds no ticker, timer or goroutine.
	src := mustReadFile(t, "updatesservice.go")
	for _, banned := range []string{"time.NewTicker", "time.Tick(", "time.AfterFunc", "go func"} {
		if strings.Contains(src, banned) {
			t.Errorf("updatesservice.go contains %q", banned)
		}
	}
	if strings.ContainsRune(src, rune(0x2013)) || strings.ContainsRune(src, rune(0x2014)) {
		t.Error("updatesservice.go holds a dash character")
	}
}

func mustReadFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
