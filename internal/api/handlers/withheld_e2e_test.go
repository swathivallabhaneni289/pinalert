// withheld_e2e_test.go proves plan 04-07's rule over the real router and a real
// Postgres: a removed report, and a pending report that is not serious, is in
// no feed view, takes no vote and answers a report link as unavailable, while a
// pending serious report stays published and votable and its author's Activity
// page still lists every report they filed (R-01, D-02). Nothing produces a
// pending report before 04-11, so each state is set straight on the row.
package handlers_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const p407Lat, p407Lon = 13.05, 77.65

// p407ReportBody is a report body at the one base point every report of this
// test shares.
func p407ReportBody(category, severity, description string) map[string]any {
	return map[string]any{
		"category":    category,
		"severity":    severity,
		"description": description,
		"latitude":    p407Lat,
		"longitude":   p407Lon,
	}
}

// p407SetModeration writes a moderation state straight onto the row.
func p407SetModeration(t *testing.T, pool *pgxpool.Pool, reportID int64, state string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE reports SET moderation_state = $1 WHERE id = $2`, state, reportID); err != nil {
		t.Fatalf("setting moderation_state %q on report %d: %v", state, reportID, err)
	}
}

// p407AssertNoWithheld fails when a feed carries one of the withheld ids, the
// visibility withheld or one of the two withheld reasons.
func p407AssertNoWithheld(t *testing.T, label string, reports []feedReport, withheldIDs ...int64) {
	t.Helper()
	for _, r := range reports {
		for _, id := range withheldIDs {
			if r.ID == id {
				t.Errorf("%s: withheld report %d is listed", label, id)
			}
		}
		if r.Visibility == "withheld" {
			t.Errorf("%s: report %d carries visibility withheld", label, r.ID)
		}
		if r.VisibilityReason == "removed_after_review" || r.VisibilityReason == "awaiting_text_check" {
			t.Errorf("%s: report %d carries reason %q", label, r.ID, r.VisibilityReason)
		}
	}
}

func TestWithheldReportsInNoFeed(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	author := newVerifiedClient(t, srv, mailer, "withheld-author@example.com")
	reader := newVerifiedClient(t, srv, mailer, "withheld-reader@example.com")

	const (
		descRemovedLow      = "Culvert washed out on the south lane"
		descRemovedCritical = "Boat needed at the old market"
		descHeldLow         = "Tree down across the school road"
		descPendingCritical = "Family on the roof at the mill"
		descPendingRescue   = "Neighbour trapped in the cellar"
		descClearLow        = "Pump station is running again"
	)
	removedLow := submitReportGetID(t, author, srv.URL, p407ReportBody("flood", "low", descRemovedLow))
	removedCritical := submitReportGetID(t, author, srv.URL, p407ReportBody("flood", "critical", descRemovedCritical))
	heldLow := submitReportGetID(t, author, srv.URL, p407ReportBody("flood", "low", descHeldLow))
	pendingCritical := submitReportGetID(t, author, srv.URL, p407ReportBody("flood", "critical", descPendingCritical))
	pendingRescue := submitReportGetID(t, author, srv.URL, p407ReportBody("rescue_needed", "low", descPendingRescue))
	clearLow := submitReportGetID(t, author, srv.URL, p407ReportBody("flood", "low", descClearLow))

	p407SetModeration(t, pool, removedLow, "removed")
	p407SetModeration(t, pool, removedCritical, "removed")
	p407SetModeration(t, pool, heldLow, "pending")
	p407SetModeration(t, pool, pendingCritical, "pending")
	p407SetModeration(t, pool, pendingRescue, "pending")

	withheld := []int64{removedLow, removedCritical, heldLow}
	published := []int64{pendingCritical, pendingRescue, clearLow}
	clients := map[string]*http.Client{"author": author, "reader": reader}

	t.Run("the withheld reports are in no feed view for either account", func(t *testing.T) {
		for who, client := range clients {
			for _, showDisputed := range []bool{false, true} {
				label := who + " show_disputed=" + strconv.FormatBool(showDisputed)
				reports := decodeFeedResponse(t, getNearby(t, client, srv.URL, p407Lat, p407Lon, showDisputed))
				p407AssertNoWithheld(t, label, reports, withheld...)
				if showDisputed {
					continue
				}
				for _, id := range published {
					r, ok := findFeedReport(reports, id)
					if !ok {
						t.Errorf("%s: published report %d is missing", label, id)
						continue
					}
					if (id == pendingCritical || id == pendingRescue) && r.Visibility != "live" {
						t.Errorf("%s: report %d visibility = %q, want live", label, id, r.Visibility)
					}
				}
			}
			for _, view := range []string{"disputed", "past"} {
				list, body := h10Get(t, client, h10URL(srv.URL, p407Lat, p407Lon, "view="+view))
				for _, r := range list.Reports {
					for _, id := range withheld {
						if r.ID == id {
							t.Errorf("%s view=%s: withheld report %d is listed", who, view, id)
						}
					}
					if r.Visibility == "withheld" {
						t.Errorf("%s view=%s: report %d carries visibility withheld", who, view, r.ID)
					}
				}
				for _, word := range []string{"withheld", "removed_after_review", "awaiting_text_check"} {
					if strings.Contains(string(body), word) {
						t.Errorf("%s view=%s: the body carries %q", who, view, word)
					}
				}
			}
		}
	})

	t.Run("a vote on a withheld report is a 404 and writes nothing", func(t *testing.T) {
		loc := map[string]any{"latitude": p407Lat, "longitude": p407Lon}
		for _, id := range withheld {
			resp := postVote(t, reader, srv.URL, id, "confirm", loc)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("confirm on withheld report %d: status %d, want 404", id, resp.StatusCode)
			}
			if field, _ := decodeErrorResponse(t, resp); field != "report" {
				t.Errorf("confirm on withheld report %d: field %q, want report", id, field)
			}
			if n := countVotesForReport(t, pool, id); n != 0 {
				t.Errorf("report %d has %d vote rows after a refused vote, want 0", id, n)
			}
		}

		// The author's own resolve is refused too: the reporter exemption does
		// not reach a report nobody can see.
		resp := postVote(t, author, srv.URL, removedLow, "resolve", loc)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("own resolve on the removed report: status %d, want 404", resp.StatusCode)
		}
		if field, _ := decodeErrorResponse(t, resp); field != "report" {
			t.Errorf("own resolve on the removed report: field %q, want report", field)
		}
		if n := countVotesForReport(t, pool, removedLow); n != 0 {
			t.Errorf("removed report has %d vote rows after the author's refused resolve, want 0", n)
		}

		// A pending serious report is published, so it takes a vote.
		vis, _ := decodeVoteResponse(t, postVote(t, reader, srv.URL, pendingCritical, "confirm", loc))
		if vis != "live" {
			t.Errorf("confirm on the pending critical report: visibility %q, want live", vis)
		}
		if n := countVotesForReport(t, pool, pendingCritical); n != 1 {
			t.Errorf("pending critical report has %d vote rows, want 1", n)
		}
	})

	t.Run("the author's Activity page still lists every report they filed", func(t *testing.T) {
		body := getProfile(t, author, srv.URL)
		for _, desc := range []string{descRemovedLow, descRemovedCritical, descHeldLow, descPendingCritical, descPendingRescue, descClearLow} {
			if !strings.Contains(body, desc) {
				t.Errorf("/profile does not list %q", desc)
			}
		}
	})

	t.Run("a faded withheld report is in no Past view and its link is unavailable", func(t *testing.T) {
		for _, id := range []int64{heldLow, removedLow, clearLow} {
			h10MovePast(t, pool, id, 7200, 3600)
		}
		for who, client := range clients {
			list, _ := h10Get(t, client, h10URL(srv.URL, p407Lat, p407Lon, "view=past"))
			if h10Find(list.Reports, heldLow) != nil || h10Find(list.Reports, removedLow) != nil {
				t.Errorf("%s: a withheld report is in the Past view", who)
			}
			if h10Find(list.Reports, clearLow) == nil {
				t.Errorf("%s: the clear faded report is missing from the Past view, so the read is not live", who)
			}
			for _, id := range []int64{heldLow, removedLow} {
				link, _ := h10Get(t, client, h10URL(srv.URL, p407Lat, p407Lon, "report="+strconv.FormatInt(id, 10)))
				if link.Focus == nil || link.Focus.Status != "unavailable" {
					t.Errorf("%s: link to withheld report %d: focus = %+v, want unavailable", who, id, link.Focus)
				}
			}
		}
	})
}
