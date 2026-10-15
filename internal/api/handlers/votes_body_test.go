package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/account"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// bodyVoteFake is a database free service.VotingQuerier for the vote body
// tests. The report belongs to account 7 and sits at 12.9716, 77.5946.
// CurrentVotesForReports returns the row of the vote that was just inserted,
// the way the real query would after the insert, and the reporter history is
// empty, so a vote that passes every refusal answers 200.
type bodyVoteFake struct {
	inserted []sqlcgen.InsertVoteParams
	undoRows []sqlcgen.ResolutionUndoCountsRow
}

func (f *bodyVoteFake) ResolutionUndoCounts(ctx context.Context, arg sqlcgen.ResolutionUndoCountsParams) ([]sqlcgen.ResolutionUndoCountsRow, error) {
	return f.undoRows, nil
}

func (f *bodyVoteFake) ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error) {
	reporter := int64(7)
	expires := time.Now().Add(time.Hour)
	return sqlcgen.ReportVoteContextRow{
		Severity:          string(service.SeverityLow),
		Category:          string(service.CategoryFlood),
		ExpiresAt:         expires,
		ReporterAccountID: &reporter,
		Latitude:          12.9716,
		Longitude:         77.5946,
		CreatedAt:         expires.Add(-service.ExpiryDuration(service.SeverityLow)),
	}, nil
}

func (f *bodyVoteFake) InsertVote(ctx context.Context, arg sqlcgen.InsertVoteParams) error {
	f.inserted = append(f.inserted, arg)
	return nil
}

func (f *bodyVoteFake) CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	rows := make([]sqlcgen.CurrentVotesForReportsRow, 0, len(f.inserted))
	for _, v := range f.inserted {
		rows = append(rows, sqlcgen.CurrentVotesForReportsRow{
			ReportID:    v.ReportID,
			AccountID:   v.AccountID,
			Kind:        v.Kind,
			Value:       v.Value,
			GeohashCell: v.GeohashCell,
			CreatedAt:   time.Now(),
		})
	}
	return rows, nil
}

func (f *bodyVoteFake) ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error) {
	return []sqlcgen.ReporterHistoryReportsRow{}, nil
}

// postVoteBody sends one JSON body to the confirm or resolve route as the
// given account and returns the status, error.field and error.message (both
// empty on a 200).
func postVoteBody(t *testing.T, route string, accountID int64, body string) (status int, field, message string, fake *bodyVoteFake) {
	t.Helper()
	fake = &bodyVoteFake{}
	svc := service.NewVotingService(fake)

	r := chi.NewRouter()
	withAccount := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := account.WithAccount(req.Context(), account.Account{ID: accountID, Email: "voter@example.com"})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
	r.Use(withAccount)
	r.Post("/api/reports/{id}/confirm", CastVote(svc, service.VoteKindContent, service.VoteConfirm))
	r.Post("/api/reports/{id}/resolve", CastVote(svc, service.VoteKindResolution, service.VoteResolve))

	req := httptest.NewRequest(http.MethodPost, "/api/reports/1/"+route, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	status = rec.Code
	if status != http.StatusOK {
		parsed := decodeErrorBody(t, rec)
		field, message = parsed.Error.Field, parsed.Error.Message
	}
	return status, field, message, fake
}

// TestVoteBodyWithoutCoordinates pins IN-01: a vote body needs both
// coordinates, except for the reporter's own resolve or reopen, which the
// service excuses from the database reporter id. A JSON null counts as absent.
func TestVoteBodyWithoutCoordinates(t *testing.T) {
	cases := []struct {
		name      string
		route     string
		accountID int64
		body      string
		status    int
		field     string
		message   string
	}{
		{"non reporter confirm with no keys", "confirm", 9, `{}`, http.StatusBadRequest, "latitude", ""},
		{"non reporter confirm with only latitude", "confirm", 9, `{"latitude":13.0}`, http.StatusBadRequest, "longitude", "longitude is required."},
		{"non reporter confirm with only longitude", "confirm", 9, `{"longitude":77.6}`, http.StatusBadRequest, "latitude", "latitude is required."},
		{"non reporter confirm with null latitude", "confirm", 9, `{"latitude":null,"longitude":77.6}`, http.StatusBadRequest, "latitude", "latitude is required."},
		{"reporter resolve with no keys", "resolve", 7, `{}`, http.StatusOK, "", ""},
		{"reporter resolve at an explicit 0,0", "resolve", 7, `{"latitude":0,"longitude":0}`, http.StatusOK, "", ""},
		{"non reporter confirm at the report", "confirm", 9, `{"latitude":12.9716,"longitude":77.5946}`, http.StatusOK, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, field, message, fake := postVoteBody(t, tc.route, tc.accountID, tc.body)
			if status != tc.status {
				t.Fatalf("status = %d, want %d (field %q, message %q)", status, tc.status, field, message)
			}
			if field != tc.field {
				t.Errorf("field = %q, want %q", field, tc.field)
			}
			if tc.message != "" && message != tc.message {
				t.Errorf("message = %q, want %q", message, tc.message)
			}
			if tc.status == http.StatusBadRequest && len(fake.inserted) != 0 {
				t.Errorf("InsertVote called %d times, want 0", len(fake.inserted))
			}
			if tc.status == http.StatusOK && len(fake.inserted) != 1 {
				t.Errorf("InsertVote called %d times, want 1", len(fake.inserted))
			}
		})
	}
}
