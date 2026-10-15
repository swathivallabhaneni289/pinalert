package service_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// TestParseModeratorEmails pins the rules of the MODERATOR_EMAILS list parser
// (D-08, T-04-07): split on commas, trim, skip empty items, normalise with the
// sign in normaliser, dedupe, and name a bad entry by position only.
func TestParseModeratorEmails(t *testing.T) {
	ok := []struct {
		name string
		in   string
		want []string
	}{
		{"empty string", "", nil},
		{"only separators and spaces", "  ,  ", nil},
		{"mixed case is lowered", "Mod@Example.COM", []string{"mod@example.com"}},
		{"spaces around items", "  a@example.com , b@example.com  ", []string{"a@example.com", "b@example.com"}},
		{"empty items are skipped", "a@example.com,,b@example.com,", []string{"a@example.com", "b@example.com"}},
		{"duplicates collapse after normalising", "a@example.com,A@EXAMPLE.com", []string{"a@example.com"}},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			got, err := service.ParseModeratorEmails(tc.in)
			if err != nil {
				t.Fatalf("ParseModeratorEmails(%q) error = %v, want nil", tc.in, err)
			}
			if got == nil {
				t.Fatalf("ParseModeratorEmails(%q) = nil map, want a non nil map", tc.in)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ParseModeratorEmails(%q) has %d entries, want %d", tc.in, len(got), len(tc.want))
			}
			for _, w := range tc.want {
				if _, in := got[w]; !in {
					t.Errorf("ParseModeratorEmails(%q) is missing %q", tc.in, w)
				}
			}
		})
	}

	bad := []struct {
		name     string
		in       string
		position string
		secrets  []string
	}{
		{"display name form", "a@example.com,Someone <b@example.com>", "2", []string{"b@example.com", "Someone"}},
		{"not an address", "a@example.com,not an address", "2", []string{"not an address"}},
		{"position counts empty items", ",x", "2", []string{"x@"}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			got, err := service.ParseModeratorEmails(tc.in)
			if err == nil {
				t.Fatalf("ParseModeratorEmails(%q) error = nil, want an error", tc.in)
			}
			if got != nil {
				t.Errorf("ParseModeratorEmails(%q) map = %v, want nil on error", tc.in, got)
			}
			if !strings.Contains(err.Error(), tc.position) {
				t.Errorf("error %q does not name position %s", err.Error(), tc.position)
			}
			for _, s := range tc.secrets {
				if strings.Contains(err.Error(), s) {
					t.Errorf("error %q leaks the entry text %q", err.Error(), s)
				}
			}
		})
	}
}

// TestNormalizeEmailMatchesSignIn pins that the exported wrapper returns what
// the sign in normaliser returns, so one person is one identity in both places.
func TestNormalizeEmailMatchesSignIn(t *testing.T) {
	got, err := service.NormalizeEmail(" Mod@Example.COM ")
	if err != nil || got != "mod@example.com" {
		t.Fatalf("NormalizeEmail(padded mixed case) = %q, %v, want mod@example.com, nil", got, err)
	}

	got, err = service.NormalizeEmail("Someone <a@example.com>")
	if got != "" {
		t.Errorf("NormalizeEmail(display name) = %q, want empty", got)
	}
	var ve service.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("NormalizeEmail(display name) error = %v, want a ValidationError", err)
	}
	want := service.ValidationError{Field: "email", Message: "Enter a valid email address."}
	if !reflect.DeepEqual(ve, want) {
		t.Errorf("NormalizeEmail(display name) error = %#v, want %#v", ve, want)
	}
}

// --- Task 2: the fake querier, the view words, the queues and Find ---

// modT0 is the fixed clock of the moderation service tests.
var modT0 = time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)

// fakeModerationQuerier is the in memory service.ModerationQuerier. A missing
// id answers pgx.ErrNoRows. Every call is appended to calls by method name.
type fakeModerationQuerier struct {
	mu sync.Mutex

	review  []sqlcgen.ReviewQueueRow
	flagged []sqlcgen.FlagQueueRow
	pending []sqlcgen.PendingQueueRow
	counts  sqlcgen.ModerationQueueCountsRow

	reports  map[int64]sqlcgen.ReportForModerationRow
	statuses map[int64]sqlcgen.ModerationStatusForReportRow

	votes     []sqlcgen.CurrentVotesForReportsRow
	reporters []sqlcgen.ReporterAccountsForReportsRow

	insertErr error

	calls        []string
	voteArgs     [][]int64
	reporterArgs [][]int64
	inserts      []sqlcgen.InsertModerationActionParams
	kept         []int64
	removed      []int64
}

var _ service.ModerationQuerier = (*fakeModerationQuerier)(nil)

func (f *fakeModerationQuerier) record(name string) {
	f.calls = append(f.calls, name)
}

func (f *fakeModerationQuerier) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == name {
			n++
		}
	}
	return n
}

func (f *fakeModerationQuerier) ReviewQueue(context.Context) ([]sqlcgen.ReviewQueueRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ReviewQueue")
	return f.review, nil
}

func (f *fakeModerationQuerier) FlagQueue(context.Context) ([]sqlcgen.FlagQueueRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("FlagQueue")
	return f.flagged, nil
}

func (f *fakeModerationQuerier) PendingQueue(context.Context) ([]sqlcgen.PendingQueueRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("PendingQueue")
	return f.pending, nil
}

func (f *fakeModerationQuerier) ModerationQueueCounts(context.Context) (sqlcgen.ModerationQueueCountsRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ModerationQueueCounts")
	return f.counts, nil
}

func (f *fakeModerationQuerier) ReportForModeration(_ context.Context, id int64) (sqlcgen.ReportForModerationRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ReportForModeration")
	row, ok := f.reports[id]
	if !ok {
		return sqlcgen.ReportForModerationRow{}, pgx.ErrNoRows
	}
	return row, nil
}

func (f *fakeModerationQuerier) ModerationStatusForReport(_ context.Context, id int64) (sqlcgen.ModerationStatusForReportRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ModerationStatusForReport")
	row, ok := f.statuses[id]
	if !ok {
		return sqlcgen.ModerationStatusForReportRow{}, pgx.ErrNoRows
	}
	return row, nil
}

func (f *fakeModerationQuerier) KeepReport(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("KeepReport")
	f.kept = append(f.kept, id)
	return nil
}

func (f *fakeModerationQuerier) RemoveReport(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("RemoveReport")
	f.removed = append(f.removed, id)
	return nil
}

func (f *fakeModerationQuerier) InsertModerationAction(_ context.Context, arg sqlcgen.InsertModerationActionParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("InsertModerationAction")
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserts = append(f.inserts, arg)
	return nil
}

func (f *fakeModerationQuerier) CurrentVotesForReports(_ context.Context, ids []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("CurrentVotesForReports")
	f.voteArgs = append(f.voteArgs, append([]int64(nil), ids...))
	return f.votes, nil
}

func (f *fakeModerationQuerier) ReporterAccountsForReports(_ context.Context, ids []int64) ([]sqlcgen.ReporterAccountsForReportsRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ReporterAccountsForReports")
	f.reporterArgs = append(f.reporterArgs, append([]int64(nil), ids...))
	return f.reporters, nil
}

func modStr(s string) *string { return &s }

// modReviewRow, modFlagRow, modPendingRow and modDetailRow build one row of
// each generated type: a low flood report created 30 minutes before modT0 that
// expires 6 hours after it, in the clear state.
func modReviewRow(id int64) sqlcgen.ReviewQueueRow {
	return sqlcgen.ReviewQueueRow{
		ID: id, Category: "flood", Severity: "low", Description: "Water rising on the main road",
		CreatedAt: modT0.Add(-30 * time.Minute), ExpiresAt: modT0.Add(6 * time.Hour),
		ModerationState: "clear", CheckStatus: "done", Verdict: modStr("pass"),
		Scores: []byte(`{"harassment":0.5,"hate":0.1}`), Model: modStr("omni-moderation-latest"), NeedsReview: true,
		FlagCounts: []byte(`{}`),
	}
}

func modFlagRow(id int64) sqlcgen.FlagQueueRow {
	return sqlcgen.FlagQueueRow{
		ID: id, Category: "flood", Severity: "low", Description: "Water rising on the main road",
		CreatedAt: modT0.Add(-30 * time.Minute), ExpiresAt: modT0.Add(6 * time.Hour),
		ModerationState: "clear", OpenFlagTotal: 3, FlagCounts: []byte(`{"abusive":1,"spam":2}`),
	}
}

func modPendingRow(id int64) sqlcgen.PendingQueueRow {
	return sqlcgen.PendingQueueRow{
		ID: id, Category: "flood", Severity: "low", Description: "Water rising on the main road",
		CreatedAt: modT0.Add(-30 * time.Minute), ExpiresAt: modT0.Add(6 * time.Hour),
		ModerationState: "pending", CheckStatus: "pending", FlagCounts: []byte(`{}`),
	}
}

func modDetailRow(id int64) sqlcgen.ReportForModerationRow {
	return sqlcgen.ReportForModerationRow{
		ID: id, Category: "flood", Severity: "low", Description: "Water rising on the main road",
		CreatedAt: modT0.Add(-30 * time.Minute), ExpiresAt: modT0.Add(6 * time.Hour),
		ModerationState: "clear", FlagCounts: []byte(`{}`),
	}
}

func modService(q service.ModerationQuerier) *service.ModerationService {
	return service.NewModerationService(q, service.WithModerationClock(func() time.Time { return modT0 }))
}

func TestModerationViewOf(t *testing.T) {
	hourAgo := modT0.Add(-time.Hour)
	monthAgo := modT0.Add(-30 * 24 * time.Hour)
	future := modT0.Add(time.Hour)
	tests := []struct {
		name     string
		vis      service.Visibility
		expires  time.Time
		wantView string
		wantWord string
	}{
		{"live unexpired", service.VisibilityLive, future, "active", "Showing to everyone"},
		{"provisional unexpired", service.VisibilityProvisional, future, "active", "Showing to everyone"},
		{"hidden unexpired", service.VisibilityHidden, future, "disputed", "Disputed"},
		{"retracted unexpired", service.VisibilityRetracted, future, "resolved", "Resolved"},
		{"retracted expired an hour ago", service.VisibilityRetracted, hourAgo, "resolved", "Resolved"},
		{"live expired an hour ago", service.VisibilityLive, hourAgo, "past", "Expired"},
		{"hidden expired an hour ago", service.VisibilityHidden, hourAgo, "past", "Expired"},
		{"live expired 30 days ago", service.VisibilityLive, monthAgo, "past", "Expired"},
		{"provisional expired 30 days ago", service.VisibilityProvisional, monthAgo, "past", "Expired"},
		{"hidden expired 30 days ago", service.VisibilityHidden, monthAgo, "past", "Expired"},
		{"retracted expired 30 days ago", service.VisibilityRetracted, monthAgo, "resolved", "Resolved"},
		{"unknown visibility", service.Visibility(""), future, "none", "No longer in any view"},
		{"withheld unexpired", service.VisibilityWithheld, future, "none", ""},
		{"withheld expired 30 days ago", service.VisibilityWithheld, monthAgo, "none", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			view, word := service.ModerationViewOf(tc.vis, tc.expires, modT0)
			if view != tc.wantView || word != tc.wantWord {
				t.Errorf("ModerationViewOf = (%q, %q), want (%q, %q)", view, word, tc.wantView, tc.wantWord)
			}
			if tc.vis != service.VisibilityWithheld {
				want := string(service.ViewOf(tc.vis, tc.expires, modT0))
				if want == "" {
					want = "none"
				}
				if view != want {
					t.Errorf("view %q differs from ViewOf %q", view, want)
				}
			}
		})
	}
}

func TestModerationQueuesBatchReads(t *testing.T) {
	t.Run("one batched read of each kind", func(t *testing.T) {
		f := &fakeModerationQuerier{
			review:  []sqlcgen.ReviewQueueRow{modReviewRow(1), modReviewRow(2)},
			flagged: []sqlcgen.FlagQueueRow{modFlagRow(2)},
			pending: []sqlcgen.PendingQueueRow{modPendingRow(3)},
			counts:  sqlcgen.ModerationQueueCountsRow{ReviewCount: 101, FlaggedCount: 1, PendingCount: 1},
		}
		got, err := modService(f).Queues(context.Background())
		if err != nil {
			t.Fatalf("Queues error = %v", err)
		}
		if n := f.count("CurrentVotesForReports"); n != 1 {
			t.Errorf("CurrentVotesForReports called %d times, want 1", n)
		}
		if n := f.count("ReporterAccountsForReports"); n != 1 {
			t.Errorf("ReporterAccountsForReports called %d times, want 1", n)
		}
		want := []int64{1, 2, 3}
		if !reflect.DeepEqual(f.voteArgs[0], want) || !reflect.DeepEqual(f.reporterArgs[0], want) {
			t.Errorf("batched ids = %v and %v, want %v", f.voteArgs[0], f.reporterArgs[0], want)
		}
		if got.Counts.Review != 101 || got.Counts.Flagged != 1 || got.Counts.Pending != 1 {
			t.Errorf("Counts = %+v, want 101, 1, 1", got.Counts)
		}
		if len(got.Review) != 2 {
			t.Errorf("len(Review) = %d, want 2 (the count is not the list length)", len(got.Review))
		}
		if len(got.Flagged) != 1 || got.Flagged[0].ID != 2 || len(got.Review) < 2 || got.Review[1].ID != 2 {
			t.Errorf("id 2 must appear in both Review and Flagged, got review %v flagged %v", got.Review, got.Flagged)
		}
		if len(got.Pending) != 1 || got.Pending[0].ID != 3 {
			t.Errorf("Pending = %v, want id 3", got.Pending)
		}
	})

	t.Run("empty lists make no batched read", func(t *testing.T) {
		f := &fakeModerationQuerier{}
		got, err := modService(f).Queues(context.Background())
		if err != nil {
			t.Fatalf("Queues error = %v", err)
		}
		if f.count("CurrentVotesForReports") != 0 || f.count("ReporterAccountsForReports") != 0 {
			t.Errorf("empty lists made batched reads: %v", f.calls)
		}
		if got.Review == nil || got.Flagged == nil || got.Pending == nil {
			t.Errorf("lists must be non nil: %+v", got)
		}
	})
}

func TestModerationRowMapping(t *testing.T) {
	ctx := context.Background()

	t.Run("scores and model only for a done needs_review check", func(t *testing.T) {
		f := &fakeModerationQuerier{review: []sqlcgen.ReviewQueueRow{modReviewRow(1)}}
		got, err := modService(f).Queues(ctx)
		if err != nil {
			t.Fatalf("Queues error = %v", err)
		}
		row := got.Review[0]
		if !reflect.DeepEqual(row.Scores, map[string]float64{"harassment": 0.5, "hate": 0.1}) || row.Model != "omni-moderation-latest" {
			t.Errorf("Scores = %v, Model = %q", row.Scores, row.Model)
		}
		if row.View != "active" || row.VisibilityWord != "Showing to everyone" {
			t.Errorf("view = %q, word = %q", row.View, row.VisibilityWord)
		}

		d := modDetailRow(2)
		d.CheckStatus, d.NeedsReview, d.Scores, d.Model = modStr("done"), false, []byte(`{"hate":0.9}`), modStr("m")
		f2 := &fakeModerationQuerier{
			reports:  map[int64]sqlcgen.ReportForModerationRow{2: d},
			statuses: map[int64]sqlcgen.ModerationStatusForReportRow{2: {ID: 2}},
		}
		found, _, err := modService(f2).Find(ctx, 2)
		if err != nil {
			t.Fatalf("Find error = %v", err)
		}
		if found.Scores != nil || found.Model != "" {
			t.Errorf("needs_review false: Scores = %v, Model = %q, want nil and empty", found.Scores, found.Model)
		}
	})

	t.Run("flag counts in dialog order with labels", func(t *testing.T) {
		f := &fakeModerationQuerier{flagged: []sqlcgen.FlagQueueRow{modFlagRow(1)}}
		got, err := modService(f).Queues(ctx)
		if err != nil {
			t.Fatalf("Queues error = %v", err)
		}
		row := got.Flagged[0]
		want := []service.ModerationFlagCount{
			{Reason: service.FlagSpam, Label: "Spam or gibberish", Count: 2},
			{Reason: service.FlagAbusive, Label: "Abusive or hateful", Count: 1},
		}
		if row.FlagTotal != 3 || !reflect.DeepEqual(row.FlagReasons, want) {
			t.Errorf("FlagTotal = %d, FlagReasons = %v, want 3 and %v", row.FlagTotal, row.FlagReasons, want)
		}
	})

	t.Run("empty and null flag counts give an empty non nil slice", func(t *testing.T) {
		for _, raw := range [][]byte{nil, []byte(`{}`), []byte(`null`)} {
			p := modPendingRow(1)
			p.FlagCounts = raw
			f := &fakeModerationQuerier{pending: []sqlcgen.PendingQueueRow{p}}
			got, err := modService(f).Queues(ctx)
			if err != nil {
				t.Fatalf("Queues(%q) error = %v", raw, err)
			}
			if got.Pending[0].FlagReasons == nil || len(got.Pending[0].FlagReasons) != 0 {
				t.Errorf("flag_counts %q gave %v, want a non nil empty slice", raw, got.Pending[0].FlagReasons)
			}
		}
	})

	t.Run("pending rows are withheld unless serious", func(t *testing.T) {
		low := modPendingRow(1)
		crit := modPendingRow(2)
		crit.Severity = "critical"
		removed := modPendingRow(3)
		removed.Severity, removed.ModerationState = "critical", "removed"
		f := &fakeModerationQuerier{pending: []sqlcgen.PendingQueueRow{low, crit, removed}}
		got, err := modService(f).Queues(ctx)
		if err != nil {
			t.Fatalf("Queues error = %v", err)
		}
		type want struct {
			serious bool
			view    string
			word    string
		}
		for i, w := range []want{{false, "none", ""}, {true, "active", "Showing to everyone"}, {true, "none", ""}} {
			r := got.Pending[i]
			if r.Serious != w.serious || r.View != w.view || r.VisibilityWord != w.word {
				t.Errorf("row %d: serious %v view %q word %q, want %+v", r.ID, r.Serious, r.View, r.VisibilityWord, w)
			}
		}
		if got.Pending[0].Moderation != service.ModerationPending || got.Pending[2].Moderation != service.ModerationRemoved {
			t.Errorf("moderation states = %q and %q", got.Pending[0].Moderation, got.Pending[2].Moderation)
		}
	})

	t.Run("written time only past the gap", func(t *testing.T) {
		far, near := modReviewRow(1), modReviewRow(2)
		far.ClientWrittenAt = pgtype.Timestamptz{Time: far.CreatedAt.Add(-4 * time.Hour), Valid: true}
		near.ClientWrittenAt = pgtype.Timestamptz{Time: near.CreatedAt.Add(-10 * time.Minute), Valid: true}
		f := &fakeModerationQuerier{review: []sqlcgen.ReviewQueueRow{far, near}}
		got, err := modService(f).Queues(ctx)
		if err != nil {
			t.Fatalf("Queues error = %v", err)
		}
		if got.Review[0].WrittenAt == nil {
			t.Error("a time written 4 hours earlier must show")
		}
		if got.Review[1].WrittenAt != nil {
			t.Error("a time written 10 minutes earlier must not show")
		}
	})
}

func TestModerationFind(t *testing.T) {
	ctx := context.Background()
	f := &fakeModerationQuerier{
		reports: map[int64]sqlcgen.ReportForModerationRow{1: modDetailRow(1), 2: modDetailRow(2)},
		statuses: map[int64]sqlcgen.ModerationStatusForReportRow{
			1: {ID: 1, InReview: true, InPending: true},
			2: {ID: 2},
		},
	}
	svc := modService(f)

	if _, _, err := svc.Find(ctx, 99); !errors.Is(err, service.ErrReportNotFound) {
		t.Errorf("unknown id error = %v, want ErrReportNotFound", err)
	}
	_, lists, err := svc.Find(ctx, 1)
	if err != nil || !reflect.DeepEqual(lists, []string{"review", "pending"}) {
		t.Errorf("Find(1) lists = %v, %v, want [review pending]", lists, err)
	}
	_, lists, err = svc.Find(ctx, 2)
	if err != nil || lists == nil || len(lists) != 0 {
		t.Errorf("Find(2) lists = %v, %v, want a non nil empty list", lists, err)
	}
}
