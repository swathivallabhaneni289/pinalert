// activity_undo_e2e_test.go belongs to plan 03.1-07. It drives the real router
// over a real Postgres to prove two things on the Activity page: the poster's
// Reopen follows the limit of two undos (D-22), and a voted card of someone
// else's finished Rescue report is blurred and carries no description (D-27).
// It reuses the helpers of activity_e2e_test.go and votes_e2e_test.go.
package handlers_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// p31AccountID returns the id of the verified account with the given email.
func p31AccountID(t *testing.T, pool *pgxpool.Pool, email string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM accounts WHERE email = $1`, strings.ToLower(email),
	).Scan(&id); err != nil {
		t.Fatalf("looking up account %q: %v", email, err)
	}
	return id
}

// p31SeedResolutionRow appends one resolution row of an account to the votes
// log at the given time, the way the real route would.
func p31SeedResolutionRow(t *testing.T, pool *pgxpool.Pool, reportID, accountID int64, value string, at time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
		 VALUES ($1, $2, 'resolution', $3, 'p31cell', $4)`,
		reportID, accountID, value, at,
	); err != nil {
		t.Fatalf("seeding %s row on report %d: %v", value, reportID, err)
	}
}

// TestTakeBackLimitHidesActivityReopen proves D-22 on the page: a poster who
// has used both undos on a report they then resolved sees no Reopen hook and
// gets the used hook instead, while a poster with one undo left keeps Reopen.
func TestTakeBackLimitHidesActivityReopen(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	email := "activity-p31-limit@example.com"
	client := newVerifiedClient(t, srv, mailer, email)
	accountID := p31AccountID(t, pool, email)
	loc := map[string]any{"latitude": 12.9716, "longitude": 77.5946}

	// Two undos used earlier in the log, then the standing resolve.
	spent := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	p31SeedResolutionRow(t, pool, spent, accountID, "unresolve", time.Now().Add(-3*time.Hour))
	p31SeedResolutionRow(t, pool, spent, accountID, "unresolve", time.Now().Add(-2*time.Hour))
	resp := postVote(t, client, srv.URL, spent, "resolve", loc)
	if vis, _ := decodeVoteResponse(t, resp); vis != "retracted" {
		t.Fatalf("resolve visibility = %q, want retracted", vis)
	}

	// One undo used, then the standing resolve.
	spare := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	p31SeedResolutionRow(t, pool, spare, accountID, "unresolve", time.Now().Add(-2*time.Hour))
	resp = postVote(t, client, srv.URL, spare, "resolve", loc)
	if vis, _ := decodeVoteResponse(t, resp); vis != "retracted" {
		t.Fatalf("resolve visibility = %q, want retracted", vis)
	}

	body := getProfile(t, client, srv.URL)

	spentRow := p15ProfileRow(t, body, spent)
	if p15HasAttr(spentRow, "data-can-reopen") {
		t.Errorf("a report with both undos used still carries data-can-reopen: %s", spentRow)
	}
	if got := p15Attr(t, spentRow, "data-undos-used"); got != "true" {
		t.Errorf("data-undos-used = %q, want true", got)
	}
	if got := p15Attr(t, spentRow, "data-visibility"); got != "retracted" {
		t.Errorf("data-visibility = %q, want retracted", got)
	}

	spareRow := p15ProfileRow(t, body, spare)
	if !p15HasAttr(spareRow, "data-can-reopen") {
		t.Errorf("a report with one undo left lost its Reopen hook: %s", spareRow)
	}
	if p15HasAttr(spareRow, "data-undos-used") {
		t.Errorf("a report with one undo left carries data-undos-used: %s", spareRow)
	}
}

// TestActivityVotedRescueCard proves D-27 and T-31-30: account A confirmed a
// Rescue report of B, B then resolved it, and A's card shows the coarse words
// and none of the description, while B's own card keeps the exact text.
func TestActivityVotedRescueCard(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	a := newVerifiedClient(t, srv, mailer, "activity-p31-rescue-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "activity-p31-rescue-b@example.com")
	loc := map[string]any{"latitude": 12.9716, "longitude": 77.5946}
	const secret = "Grandmother stuck on the roof of the blue house on Temple Street"

	rescueID := submitReportGetID(t, b, srv.URL, map[string]any{
		"category":    "rescue_needed",
		"severity":    "critical",
		"description": secret,
		"latitude":    12.9716,
		"longitude":   77.5946,
	})
	p15VoteTrust(t, postVote(t, a, srv.URL, rescueID, "confirm", loc))

	// While the report is live, A's card still shows the text.
	liveRow := p15ProfileRow(t, getProfile(t, a, srv.URL), rescueID)
	if !strings.Contains(liveRow, secret) {
		t.Fatalf("a live Rescue voted card lost its description: %s", liveRow)
	}

	p15VoteTrust(t, postVote(t, b, srv.URL, rescueID, "resolve", loc))

	body := getProfile(t, a, srv.URL)
	row := p15ProfileRow(t, body, rescueID)
	if strings.Contains(body, secret) {
		t.Errorf("A's /profile still carries the finished Rescue description")
	}
	if !strings.Contains(row, `<p class="report-row__title">Rescue needed</p>`) {
		t.Errorf("A's finished Rescue card title is not Rescue needed: %s", row)
	}
	if !strings.Contains(row, "Approximate location") {
		t.Errorf("A's finished Rescue card does not say Approximate location: %s", row)
	}
	if p15HasAttr(row, "data-can-reopen") || p15HasAttr(row, "data-undos-used") {
		t.Errorf("a voted card carries a Reopen hook or the used hook: %s", row)
	}

	// The poster's own card stays exact.
	ownRow := p15ProfileRow(t, getProfile(t, b, srv.URL), rescueID)
	if !strings.Contains(ownRow, secret) {
		t.Errorf("the poster's own card lost its description: %s", ownRow)
	}
	if strings.Contains(ownRow, "Approximate location") {
		t.Errorf("the poster's own card is marked approximate: %s", ownRow)
	}
}
