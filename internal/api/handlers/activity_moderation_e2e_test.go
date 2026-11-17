// activity_moderation_e2e_test.go proves plan 04-16 over the real router and a
// real Postgres: an author sees why a report of theirs is in no feed (P4-A9,
// P4-A10), a late report shows its Written line (P4-A8), and the Policy
// pointer appears only when a contact address is set (D-24). Nothing produces
// a pending report before the screener runs, so each state is set straight on
// the row, the way withheld_e2e_test.go does.
package handlers_test

import (
	"context"
	"html"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/web"
)

// p416ReportBody is a report body at the one base point every report of this
// plan's tests shares, with the three varying fields set.
func p416ReportBody(severity, category, description string) map[string]any {
	body := floodReportBody(12.9716, 77.5946)
	body["severity"] = severity
	body["category"] = category
	body["description"] = description
	return body
}

// p416SetModeration writes a moderation state straight onto the row.
func p416SetModeration(t *testing.T, pool *pgxpool.Pool, reportID int64, state string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE reports SET moderation_state = $1 WHERE id = $2`, state, reportID); err != nil {
		t.Fatalf("setting moderation_state %q on report %d: %v", state, reportID, err)
	}
}

// p416SetWrittenAgo sets client_written_at to ago before created_at.
func p416SetWrittenAgo(t *testing.T, pool *pgxpool.Pool, reportID int64, ago time.Duration) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE reports SET client_written_at = created_at - make_interval(secs => $1) WHERE id = $2`,
		ago.Seconds(), reportID); err != nil {
		t.Fatalf("setting client_written_at on report %d: %v", reportID, err)
	}
}

var p416NoteRE = regexp.MustCompile(`(?s)<p class="report-row__moderation-note">(.*?)</p>`)

// p416Note returns the inner markup of the row's moderation note paragraph.
func p416Note(row string) (string, bool) {
	m := p416NoteRE.FindStringSubmatch(row)
	if m == nil {
		return "", false
	}
	return m[1], true
}

var (
	p416TagRE   = regexp.MustCompile(`<[^>]*>`)
	p416SpaceRE = regexp.MustCompile(`\s+`)
)

// p416Text removes tags, unescapes entities (html/template writes the
// apostrophe of "isn't" as &#39;), and collapses whitespace.
func p416Text(markup string) string {
	text := html.UnescapeString(p416TagRE.ReplaceAllString(markup, ""))
	return strings.TrimSpace(p416SpaceRE.ReplaceAllString(text, " "))
}

func p416HasClass(tokens []string, want string) bool {
	for _, tok := range tokens {
		if tok == want {
			return true
		}
	}
	return false
}

// p416VotedBy has poster post body and voter confirm it, and returns the id.
func p416VotedBy(t *testing.T, srvURL string, poster, voter *http.Client, body map[string]any) int64 {
	t.Helper()
	id := submitReportGetID(t, poster, srvURL, body)
	resp := postVote(t, voter, srvURL, id, "confirm", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm on report %d status = %d, want 200", id, resp.StatusCode)
	}
	return id
}

// TestActivityHeldAndRemovedRows pinned the author's held and removed rows on My
// activity: their state class, their note and no trust (plan 04-16, P4-A9,
// P4-A10).
//
// Amended 2026-11-17 (quick 261117-vuf, owner on the 7.1 UAT): the author's own
// reports left My activity with Posted (quick 261117-vvb tells the author in the
// bell when one is held, shown again or removed). So the author's page lists
// none of them, held, removed or clear, and the voter's page keeps the rule the
// service already had: a voted report that is withheld (held or removed) is
// dropped, while a clear report and a critical report still waiting for its
// text check (published at once, so not withheld) keep their trust and carry
// no moderation note.
func TestActivityHeldAndRemovedRows(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	author := newVerifiedClient(t, srv, mailer, "activity-p416-held@example.com")
	voter := newVerifiedClient(t, srv, mailer, "activity-p416-held-voter@example.com")

	clear := p416VotedBy(t, srv.URL, author, voter, p416ReportBody("low", "flood", "Water over the road near the old school"))
	held := p416VotedBy(t, srv.URL, author, voter, p416ReportBody("low", "flood", "Bus stand flooded up to the knees"))
	removed := p416VotedBy(t, srv.URL, author, voter, p416ReportBody("low", "flood", "Lane behind the temple is under water"))
	pendingCritical := p416VotedBy(t, srv.URL, author, voter, p416ReportBody("critical", "flood", "Family stuck on a roof near the canal"))
	p416SetModeration(t, pool, held, "pending")
	p416SetModeration(t, pool, removed, "removed")
	p416SetModeration(t, pool, pendingCritical, "pending")

	authorBody := getProfile(t, author, srv.URL)
	for _, id := range []int64{clear, held, removed, pendingCritical} {
		if strings.Contains(authorBody, `data-report-id="`+strconv.FormatInt(id, 10)+`"`) {
			t.Errorf("the author's My activity lists the author's own report %d", id)
		}
	}
	for _, gone := range []string{"data-moderation", "report-row__moderation-note", service.HeldSentence, service.RemovedSentence} {
		if strings.Contains(html.UnescapeString(authorBody), gone) {
			t.Errorf("the author's My activity holds %q", gone)
		}
	}

	voterBody := getProfile(t, voter, srv.URL)
	for _, id := range []int64{held, removed} {
		if strings.Contains(voterBody, `data-report-id="`+strconv.FormatInt(id, 10)+`"`) {
			t.Errorf("the voter's My activity lists withheld report %d", id)
		}
	}
	for name, id := range map[string]int64{"clear": clear, "pending critical": pendingCritical} {
		row := p15ProfileRow(t, voterBody, id)
		if p15HasAttr(row, "data-moderation") {
			t.Errorf("%s row carries data-moderation: %s", name, row)
		}
		if trust := p15ProfileTrust(t, row); trust == nil {
			t.Errorf("%s row has no trust object", name)
		}
		if p416HasClass(p15ClassTokens(t, row), "vis-held") {
			t.Errorf("%s row carries vis-held", name)
		}
		if _, ok := p416Note(row); ok {
			t.Errorf("%s row carries a note: %s", name, row)
		}
	}
}

// TestRemovedPolicyPointerNeedsContact pinned the Policy pointer on the
// author's removed row (D-24). Amended 2026-11-17 (quick 261117-vuf): the
// author's removed row left My activity with Posted, so with or without a
// contact address the page draws no removed row, no note and no Policy link.
func TestRemovedPolicyPointerNeedsContact(t *testing.T) {
	for name, contact := range map[string]string{"no_contact": "", "contact_set": "help@pinalert.example"} {
		t.Run(name, func(t *testing.T) {
			srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
				d.Chrome = handlers.Chrome{ContactEmail: contact}
			})
			client := newVerifiedClient(t, srv, mailer, "activity-p416-"+name+"@example.com")
			removed := submitReportGetID(t, client, srv.URL, p416ReportBody("low", "flood", "Pump house wall has fallen in"))
			p416SetModeration(t, pool, removed, "removed")

			body := getProfile(t, client, srv.URL)
			if strings.Contains(body, `data-report-id="`+strconv.FormatInt(removed, 10)+`"`) {
				t.Errorf("My activity lists the author's removed report")
			}
			if strings.Contains(body, `<a href="/policy">Policy page</a>`) || strings.Contains(body, "report-row__moderation-note") {
				t.Errorf("My activity draws a removed note or its Policy link")
			}
		})
	}
}

// TestActivityWrittenLine proves P4-A8 on My activity: a report that reached
// the server long after it was written shows its Written line on the Report
// cell's detail line, and one written minutes before upload shows none.
// Amended 2026-11-17 (quick 261117-vuf): the cards are a voter's, since the
// author's own posts left My activity; the held late row left with them.
func TestActivityWrittenLine(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	author := newVerifiedClient(t, srv, mailer, "activity-p416-written@example.com")
	voter := newVerifiedClient(t, srv, mailer, "activity-p416-written-voter@example.com")

	late := p416VotedBy(t, srv.URL, author, voter, p416ReportBody("low", "flood", "Road to the ferry is cut off"))
	near := p416VotedBy(t, srv.URL, author, voter, p416ReportBody("low", "flood", "Water is up to the first step"))
	p416SetWrittenAgo(t, pool, late, 4*time.Hour)
	p416SetWrittenAgo(t, pool, near, 10*time.Minute)

	body := getProfile(t, voter, srv.URL)

	lateRow := p15ProfileRow(t, body, late)
	const line = `Flood &middot; Written 4 hours ago</p>`
	if n := strings.Count(lateRow, line); n != 1 {
		t.Fatalf("late row has the Written line %d times, want 1: %s", n, lateRow)
	}
	directly := regexp.MustCompile(`(?s)<p class="report-row__title">.*?</p>\s*<p class="report-row__meta[^"]*">` + regexp.QuoteMeta(line))
	if !directly.MatchString(lateRow) {
		t.Errorf("the Written line does not follow the title paragraph on the detail line: %s", lateRow)
	}

	if nearRow := p15ProfileRow(t, body, near); strings.Contains(nearRow, "Written") {
		t.Errorf("a report written 10 minutes before upload shows a Written line: %s", nearRow)
	}
}

// TestActivityModerationCopyMatchesServer proves the two label words in
// visibility.js are the Go constants, so the browser table and the server copy
// cannot drift apart (P4-A9). It needs no database.
func TestActivityModerationCopyMatchesServer(t *testing.T) {
	raw, err := fs.ReadFile(web.StaticFS, "static/js/visibility.js")
	if err != nil {
		t.Fatalf("reading visibility.js: %v", err)
	}
	text := string(raw)
	for _, want := range []string{
		"held: '" + service.HeldLabel + "'",
		"removed: '" + service.RemovedLabel + "'",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("visibility.js does not contain %s", want)
		}
	}
}
