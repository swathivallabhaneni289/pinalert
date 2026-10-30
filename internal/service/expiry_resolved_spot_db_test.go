package service_test

import (
	"fmt"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestConfirmWhileResolvedUsesUpSpotAndPerson pins the accepted limit of review
// IN-03 (quick task 261030-64q, 2026-10-30), written in TRUST-MODEL under rule
// 7. A confirm cast while the report is resolved is refused an extension (rule
// 7) but its row is still written to the append only log. After the reporter
// reopens the report, that row still counts as an earlier confirm: neither that
// spot nor that person can extend the report any more, while a new spot and a
// new person still can. The log is the reference, so the limit is the price of
// the append only rule that stops a confirm, dispute, confirm loop.
func TestConfirmWhileResolvedUsesUpSpotAndPerson(t *testing.T) {
	pool := testutil.NewTestDB(t)
	svc := service.NewVotingService(sqlcgen.New(pool))
	spots := q64Spots(t, 4)

	reporter := testutil.SeedReporterAccount(t, pool, "q64-res-reporter@example.com", "q64-res-sess")
	seed := func(name string) int64 {
		return testutil.SeedReporterAccount(t, pool, fmt.Sprintf("q64-res-%s@example.com", name))
	}
	h, other, fresh := seed("h"), seed("other"), seed("fresh")

	r := q64Report(t, pool, "q64-res-sess", "medium", time.Hour, 3*time.Hour)
	_, before := q64Times(t, pool, r)

	q64Cast(t, svc, service.CastVoteInput{ReportID: r, AccountID: reporter, Kind: service.VoteKindResolution, Value: service.VoteResolve, LocationMissing: true})
	res := q64Cast(t, svc, q64Confirm(r, h, spots[0][0], spots[0][1]))
	q64WantUnchanged(t, "H confirms while the report is resolved", pool, r, h, before, res)
	q64Cast(t, svc, service.CastVoteInput{ReportID: r, AccountID: reporter, Kind: service.VoteKindResolution, Value: service.VoteReopen, LocationMissing: true})

	// The same person again, from another spot, through a dispute and a confirm.
	q64Cast(t, svc, service.CastVoteInput{ReportID: r, AccountID: h, Kind: service.VoteKindContent, Value: service.VoteDispute, Latitude: spots[1][0], Longitude: spots[1][1]})
	res = q64Cast(t, svc, q64Confirm(r, h, spots[1][0], spots[1][1]))
	q64WantUnchanged(t, "H confirms again after the reopen from a new spot", pool, r, h, before, res)

	// Another person from the spot the resolved confirm used up.
	res = q64Cast(t, svc, q64Confirm(r, other, spots[0][0], spots[0][1]))
	q64WantUnchanged(t, "another person confirms from the used up spot", pool, r, other, before, res)

	// A new person from a new spot still extends.
	res = q64Cast(t, svc, q64Confirm(r, fresh, spots[2][0], spots[2][1]))
	q64WantExtends(t, "a new person from a new spot", pool, r, fresh, service.SeverityMedium, before, res)
}
