package service

// pushworker_flag_check_test.go pins the send time gate of quick 261118-saj:
// a row of any kind about a report with the Being checked mark is skipped as
// not eligible, a Critical row about a report kept from the mark is sent, and
// a report kept after a review hide still sends no first alert.

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

func TestWorkerFlagCheckGate(t *testing.T) {
	set := pgtype.Timestamptz{Time: pushFactsNow.Add(-10 * time.Minute), Valid: true}
	cases := []struct {
		name          string
		kind          string
		event         string
		first         *string
		checkAt, kept pgtype.Timestamptz
		want          string
	}{
		{name: "critical, marked", kind: "critical", checkAt: set, want: "not_eligible"},
		{name: "critical, kept from the mark", kind: "critical", checkAt: set, kept: set, want: "sent"},
		{name: "critical, kept after a hide", kind: "critical", kept: set, want: "not_eligible"},
		{name: "critical, never flagged (control)", kind: "critical", want: "sent"},
		{name: "follow, marked", kind: "follow", checkAt: set, want: "not_eligible"},
		{name: "confirmed update, marked", kind: "confirmed_update", first: pwStr("critical"), checkAt: set, want: "not_eligible"},
		{name: "changed, marked", kind: "changed", event: "fading", checkAt: set, want: "not_eligible"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newPWRig(t)
			r.q.claimRows = []sqlcgen.ClaimDuePushesRow{pwClaim(1, c.kind)}
			r.q.claimRows[0].FirstKind = c.first
			if c.event != "" {
				r.q.claimRows[0].UpdateKind = pwStr(c.event)
			}
			r.report(func(row *sqlcgen.PushReportFactsRow) {
				row.FlagCheckAt = c.checkAt
				row.ReviewKeptAt = c.kept
			})
			if err := r.sweep(); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			if c.want == "sent" {
				if m := r.onlyMark(); m.kind != "sent" {
					t.Fatalf("mark = %+v, want sent", m)
				}
				return
			}
			r.wantSkipped(c.want)
			if n := len(r.s.messages()); n != 0 {
				t.Errorf("%d messages sent, want 0", n)
			}
		})
	}
}
