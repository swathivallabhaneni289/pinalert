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

func TestActivityHeldAndRemovedRows(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	author := newVerifiedClient(t, srv, mailer, "activity-p416-held@example.com")

	clear := submitReportGetID(t, author, srv.URL, p416ReportBody("low", "flood", "Water over the road near the old school"))
	held := submitReportGetID(t, author, srv.URL, p416ReportBody("low", "flood", "Bus stand flooded up to the knees"))
	heldExpired := submitReportGetID(t, author, srv.URL, p416ReportBody("low", "flood", "Canal bank is giving way slowly"))
	resolvedRemoved := submitReportGetID(t, author, srv.URL, p416ReportBody("low", "flood", "Lane behind the temple is under water"))
	removedCritical := submitReportGetID(t, author, srv.URL, p416ReportBody("critical", "flood", "Boat needed at the old market"))
	pendingCritical := submitReportGetID(t, author, srv.URL, p416ReportBody("critical", "flood", "Family stuck on a roof near the canal"))

	p416SetModeration(t, pool, held, "pending")
	p416SetModeration(t, pool, heldExpired, "pending")
	expireReport(t, pool, heldExpired)
	resp := postVote(t, author, srv.URL, resolvedRemoved, "resolve", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	if visibility, _ := decodeVoteResponse(t, resp); visibility != "retracted" {
		t.Fatalf("resolve visibility = %q, want retracted", visibility)
	}
	p416SetModeration(t, pool, resolvedRemoved, "removed")
	p416SetModeration(t, pool, removedCritical, "removed")
	p416SetModeration(t, pool, pendingCritical, "pending")

	body := getProfile(t, author, srv.URL)

	t.Run("held row", func(t *testing.T) {
		row := p15ProfileRow(t, body, held)
		if got := p15Attr(t, row, "data-moderation"); got != "held" {
			t.Errorf("data-moderation = %q, want held", got)
		}
		tokens := p15ClassTokens(t, row)
		if !p416HasClass(tokens, "vis-held") || p416HasClass(tokens, "vis-provisional") || p416HasClass(tokens, "vis-withheld") {
			t.Errorf("class tokens = %v, want vis-held and neither vis-provisional nor vis-withheld", tokens)
		}
		if p15HasAttr(row, "data-trust") || p15HasAttr(row, "data-can-reopen") {
			t.Errorf("held row carries data-trust or data-can-reopen: %s", row)
		}
		if got := p15Attr(t, row, "data-expired"); got != "false" {
			t.Errorf("data-expired = %q, want false", got)
		}
		note, ok := p416Note(row)
		if !ok {
			t.Fatalf("held row has no moderation note: %s", row)
		}
		if got := p416Text(note); got != service.HeldSentence {
			t.Errorf("note = %q, want %q", got, service.HeldSentence)
		}
		meta := strings.Index(row, `class="report-row__meta`)
		if meta == -1 || strings.Index(row, "report-row__moderation-note") < meta {
			t.Errorf("the note does not come after the first meta paragraph: %s", row)
		}
		if strings.Contains(row, "/policy") {
			t.Errorf("held row links /policy: %s", row)
		}
	})

	t.Run("held and expired row", func(t *testing.T) {
		row := p15ProfileRow(t, body, heldExpired)
		if got := p15Attr(t, row, "data-moderation"); got != "held" {
			t.Errorf("data-moderation = %q, want held", got)
		}
		tokens := p15ClassTokens(t, row)
		if !p416HasClass(tokens, "vis-expired") || p416HasClass(tokens, "vis-held") {
			t.Errorf("class tokens = %v, want vis-expired and no vis-held", tokens)
		}
		if got := p15Attr(t, row, "data-expired"); got != "true" {
			t.Errorf("data-expired = %q, want true", got)
		}
		if _, ok := p416Note(row); ok {
			t.Errorf("held and expired row carries a note: %s", row)
		}
		if p15HasAttr(row, "data-trust") {
			t.Errorf("held and expired row carries data-trust: %s", row)
		}
	})

	t.Run("resolved then removed row", func(t *testing.T) {
		row := p15ProfileRow(t, body, resolvedRemoved)
		if got := p15Attr(t, row, "data-moderation"); got != "removed" {
			t.Errorf("data-moderation = %q, want removed", got)
		}
		tokens := p15ClassTokens(t, row)
		if !p416HasClass(tokens, "vis-removed") || p416HasClass(tokens, "vis-retracted") {
			t.Errorf("class tokens = %v, want vis-removed and no vis-retracted", tokens)
		}
		if p15HasAttr(row, "data-can-reopen") || p15HasAttr(row, "data-trust") {
			t.Errorf("removed row carries data-can-reopen or data-trust: %s", row)
		}
		note, ok := p416Note(row)
		if !ok {
			t.Fatalf("removed row has no moderation note: %s", row)
		}
		if got := p416Text(note); got != service.RemovedSentence {
			t.Errorf("note = %q, want %q", got, service.RemovedSentence)
		}
	})

	t.Run("removed critical row", func(t *testing.T) {
		row := p15ProfileRow(t, body, removedCritical)
		if got := p15Attr(t, row, "data-moderation"); got != "removed" {
			t.Errorf("data-moderation = %q, want removed", got)
		}
		tokens := p15ClassTokens(t, row)
		if !p416HasClass(tokens, "vis-removed") {
			t.Errorf("class tokens = %v, want vis-removed", tokens)
		}
		if p15HasAttr(row, "data-trust") || p15HasAttr(row, "data-can-reopen") {
			t.Errorf("removed critical row carries data-trust or data-can-reopen: %s", row)
		}
		note, ok := p416Note(row)
		if !ok || p416Text(note) != service.RemovedSentence {
			t.Errorf("removed critical note = %q (found %v), want %q", p416Text(note), ok, service.RemovedSentence)
		}
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to
		// D-21): the severity word is a visually hidden word in the Status cell,
		// and the chip says Removed after review. The Phase 3.1 and Phase 4
		// hands-on tests are walked on this layout after Phase 3.4 is built.
		if !strings.Contains(row, `<span class="visually-hidden">Critical severity. </span>`) || !strings.Contains(row, ">Removed after review</span>") {
			t.Errorf("removed critical row lost its severity word: %s", row)
		}
	})

	t.Run("pending critical row keeps its trust", func(t *testing.T) {
		row := p15ProfileRow(t, body, pendingCritical)
		if p15HasAttr(row, "data-moderation") {
			t.Errorf("pending serious row carries data-moderation: %s", row)
		}
		if trust := p15ProfileTrust(t, row); trust == nil {
			t.Errorf("pending serious row has no trust object")
		}
		if p416HasClass(p15ClassTokens(t, row), "vis-held") {
			t.Errorf("pending serious row carries vis-held")
		}
		if _, ok := p416Note(row); ok {
			t.Errorf("pending serious row carries a note: %s", row)
		}
	})

	t.Run("clear row", func(t *testing.T) {
		row := p15ProfileRow(t, body, clear)
		if p15HasAttr(row, "data-moderation") {
			t.Errorf("clear row carries data-moderation: %s", row)
		}
		if !p15HasAttr(row, "data-trust") {
			t.Errorf("clear row has no data-trust: %s", row)
		}
		if _, ok := p416Note(row); ok {
			t.Errorf("clear row carries a note: %s", row)
		}
	})
}

func TestProfileReportAcceptsWithheld(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	author := newVerifiedClient(t, srv, mailer, "activity-p416-accepts@example.com")

	held := submitReportGetID(t, author, srv.URL, p416ReportBody("low", "flood", "Tree down across the school road"))
	removed := submitReportGetID(t, author, srv.URL, p416ReportBody("low", "flood", "Culvert washed out on the south lane"))
	p416SetModeration(t, pool, held, "pending")
	p416SetModeration(t, pool, removed, "removed")

	body := getProfile(t, author, srv.URL)
	for _, tc := range []struct {
		id     int64
		reason service.ResolveReason
	}{
		{held, service.ReasonAwaitingTextCheck},
		{removed, service.ReasonRemovedAfterReview},
	} {
		row := p15ProfileRow(t, body, tc.id)
		if got := p15Attr(t, row, "data-visibility"); got != "withheld" {
			t.Errorf("report %d data-visibility = %q, want withheld", tc.id, got)
		}
		if got := p15Attr(t, row, "data-visibility-reason"); got != string(tc.reason) {
			t.Errorf("report %d data-visibility-reason = %q, want %q", tc.id, got, tc.reason)
		}
		for _, tok := range p15ClassTokens(t, row) {
			if tok == "vis-provisional" || tok == "vis-withheld" {
				t.Errorf("report %d carries class %q", tc.id, tok)
			}
		}
	}
}

func TestRemovedPolicyPointerNeedsContact(t *testing.T) {
	seed := func(t *testing.T, srv string, client *http.Client, pool *pgxpool.Pool, email string) (removed, held int64) {
		removed = submitReportGetID(t, client, srv, p416ReportBody("low", "flood", "Pump house wall has fallen in"))
		held = submitReportGetID(t, client, srv, p416ReportBody("low", "flood", "Footbridge is closed by the police"))
		p416SetModeration(t, pool, removed, "removed")
		p416SetModeration(t, pool, held, "pending")
		return removed, held
	}

	t.Run("no_contact", func(t *testing.T) {
		srv, mailer, pool := newE2EServer(t)
		client := newVerifiedClient(t, srv, mailer, "activity-p416-nocontact@example.com")
		removed, _ := seed(t, srv.URL, client, pool, "")
		row := p15ProfileRow(t, getProfile(t, client, srv.URL), removed)
		note, ok := p416Note(row)
		if !ok || p416Text(note) != service.RemovedSentence {
			t.Errorf("note = %q (found %v), want exactly %q", p416Text(note), ok, service.RemovedSentence)
		}
		if strings.Contains(row, `href="/policy"`) {
			t.Errorf("row links /policy with no contact address: %s", row)
		}
	})

	t.Run("contact_set", func(t *testing.T) {
		const contact = "help@pinalert.example"
		srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
			d.Chrome = handlers.Chrome{ContactEmail: contact}
		})
		client := newVerifiedClient(t, srv, mailer, "activity-p416-contact@example.com")
		removed, held := seed(t, srv.URL, client, pool, contact)
		body := getProfile(t, client, srv.URL)

		row := p15ProfileRow(t, body, removed)
		if n := strings.Count(row, `<a href="/policy">Policy page</a>`); n != 1 {
			t.Errorf("row has the Policy link %d times, want 1: %s", n, row)
		}
		note, ok := p416Note(row)
		want := service.RemovedSentence + " " + service.RemovedPolicyPointer
		if !ok || p416Text(note) != want {
			t.Errorf("note = %q (found %v), want %q", p416Text(note), ok, want)
		}
		if strings.Contains(row, contact) {
			t.Errorf("row contains the contact address: %s", row)
		}

		heldRow := p15ProfileRow(t, body, held)
		heldNote, ok := p416Note(heldRow)
		if !ok || p416Text(heldNote) != service.HeldSentence {
			t.Errorf("held note = %q (found %v), want %q", p416Text(heldNote), ok, service.HeldSentence)
		}
		if strings.Contains(heldRow, "/policy") {
			t.Errorf("held row links /policy: %s", heldRow)
		}
	})
}

func TestActivityWrittenLine(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	author := newVerifiedClient(t, srv, mailer, "activity-p416-written@example.com")

	late := submitReportGetID(t, author, srv.URL, p416ReportBody("low", "flood", "Road to the ferry is cut off"))
	near := submitReportGetID(t, author, srv.URL, p416ReportBody("low", "flood", "Water is up to the first step"))
	lateHeld := submitReportGetID(t, author, srv.URL, p416ReportBody("low", "flood", "Wall of the cattle shed collapsed"))
	p416SetWrittenAgo(t, pool, late, 4*time.Hour)
	p416SetWrittenAgo(t, pool, near, 10*time.Minute)
	p416SetWrittenAgo(t, pool, lateHeld, 4*time.Hour)
	p416SetModeration(t, pool, lateHeld, "pending")

	body := getProfile(t, author, srv.URL)

	lateRow := p15ProfileRow(t, body, late)
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21,
	// UI-SPEC 3a): the Written line shares the Report cell's detail line,
	// "{Category} · {Written line}", instead of a paragraph of its own. The
	// Phase 3.1 and Phase 4 hands-on tests are walked on this layout after Phase
	// 3.4 is built.
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

	heldRow := p15ProfileRow(t, body, lateHeld)
	writtenAt := strings.Index(heldRow, line)
	noteAt := strings.Index(heldRow, "report-row__moderation-note")
	if writtenAt == -1 || noteAt == -1 || writtenAt > noteAt {
		t.Errorf("held row: Written line at %d, note at %d, want the line first: %s", writtenAt, noteAt, heldRow)
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
