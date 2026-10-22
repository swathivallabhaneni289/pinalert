// reliable_filter_e2e_test.go proves NOTIF-04's server side end to end over the
// real router and a real Postgres: every report on GET /api/reports carries
// kept_by_reliable_filter, authored by the server from the reporter's tag and
// the one critical rule, and the field filters nothing. A seeded Reliable
// reporter (testutil.SeedReliableReporter) supplies real history. Every package
// level identifier here carries the p10rf prefix.
package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"testing"
	"time"

	"pinalert/internal/session"
	"pinalert/internal/testutil"
)

// p10rfSecret is the session secret newTrustE2EServer signs cookies with.
const p10rfSecret = "trust-e2e-test-secret-not-for-production"

// p10rfReport is the part of a feed report this file reads.
type p10rfReport struct {
	ID                   int64  `json:"id"`
	Category             string `json:"category"`
	Severity             string `json:"severity"`
	KeptByReliableFilter *bool  `json:"kept_by_reliable_filter"`
	Trust                struct {
		Reporter struct {
			Tag string `json:"tag"`
		} `json:"reporter"`
	} `json:"trust"`
}

// p10rfSessionClient builds a client whose cookie jar holds the signed cookie
// for sessionID, so a seeded session can act through the real router.
func p10rfSessionClient(t *testing.T, baseURL, sessionID string) *http.Client {
	t.Helper()
	mgr, err := session.NewManager([]byte(p10rfSecret), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	jar.SetCookies(u, []*http.Cookie{{Name: "pinalert_session", Value: mgr.Sign(sessionID), Path: "/"}})
	return &http.Client{Jar: jar}
}

// p10rfReadAll reads the whole list (view=all) near the report point and keys
// the reports by id.
func p10rfReadAll(t *testing.T, client *http.Client, baseURL string) map[int64]p10rfReport {
	t.Helper()
	target := baseURL + "/api/reports?lat=" + strconv.FormatFloat(p07ReportLat, 'f', -1, 64) +
		"&lon=" + strconv.FormatFloat(p07ReportLon, 'f', -1, 64) + "&radius_km=5&view=all"
	resp, err := client.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the feed body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/reports: status %d, body %s", resp.StatusCode, body)
	}
	var out struct {
		Reports []p10rfReport `json:"reports"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding the feed body: %v (body %s)", err, body)
	}
	byID := make(map[int64]p10rfReport, len(out.Reports))
	for _, r := range out.Reports {
		byID[r.ID] = r
	}
	return byID
}

// TestSeedReliableReporterReadsReliable: the seeded history makes the account
// read Reliable reporter on a fresh report it files.
func TestSeedReliableReporterReadsReliable(t *testing.T) {
	clock := p07FixedClock()
	srv, mailer, pool := newTrustE2EServer(t, clock)

	const sessionID = "p10rf-reliable-session"
	testutil.SeedReliableReporter(t, pool, "p10rf-reliable@example.com", sessionID, clock())
	reliable := p10rfSessionClient(t, srv.URL, sessionID)
	reader := newVerifiedClient(t, srv, mailer, "p10rf-reader-a@example.com")

	id := submitReportGetID(t, reliable, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	got := p10rfReadAll(t, reader, srv.URL)
	r, ok := got[id]
	if !ok {
		t.Fatalf("the fresh report %d is not listed in %v", id, got)
	}
	if r.Trust.Reporter.Tag != "reliable" {
		t.Errorf("reporter tag = %q, want reliable", r.Trust.Reporter.Tag)
	}
	if r.KeptByReliableFilter == nil || !*r.KeptByReliableFilter {
		t.Errorf("kept_by_reliable_filter = %v, want true", r.KeptByReliableFilter)
	}
}

// TestReliableFilterOnTheWire is NOTIF-04 on the wire: on one read, a Reliable
// reporter's low report and a new reporter's critical report are kept, an old
// rescue_needed report is kept, a new reporter's low report is not, and all four
// are listed because the server hides nothing.
func TestReliableFilterOnTheWire(t *testing.T) {
	clock := p07FixedClock()
	srv, mailer, pool := newTrustE2EServer(t, clock)
	now := clock()

	const reliableSession = "p10rf-wire-reliable-session"
	testutil.SeedReliableReporter(t, pool, "p10rf-wire-reliable@example.com", reliableSession, now)
	reliable := p10rfSessionClient(t, srv.URL, reliableSession)
	fresh := newVerifiedClient(t, srv, mailer, "p10rf-wire-new@example.com")
	reader := newVerifiedClient(t, srv, mailer, "p10rf-wire-reader@example.com")

	reliableLow := submitReportGetID(t, reliable, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	newLow := submitReportGetID(t, fresh, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	critical := floodReportBody(p07ReportLat, p07ReportLon)
	critical["severity"] = "critical"
	newCritical := submitReportGetID(t, fresh, srv.URL, critical)
	oldRescue := testutil.SeedHistoryReport(t, pool, p07SessionID(t, fresh, srv.URL), "low", "rescue_needed", now.Add(7*time.Hour))

	got := p10rfReadAll(t, reader, srv.URL)
	want := map[int64]bool{
		reliableLow: true,
		newLow:      false,
		newCritical: true,
		oldRescue:   true,
	}
	for id, w := range want {
		r, ok := got[id]
		if !ok {
			t.Fatalf("report %d is not listed: the field must filter nothing on the server (listed %v)", id, got)
		}
		if r.KeptByReliableFilter == nil {
			t.Fatalf("report %d has no kept_by_reliable_filter key", id)
		}
		if *r.KeptByReliableFilter != w {
			t.Errorf("report %d (%s %s, tag %q) kept_by_reliable_filter = %v, want %v",
				id, r.Severity, r.Category, r.Trust.Reporter.Tag, *r.KeptByReliableFilter, w)
		}
	}
	if tag := got[reliableLow].Trust.Reporter.Tag; tag != "reliable" {
		t.Errorf("the reliable reporter's report reads tag %q, want reliable", tag)
	}
	if tag := got[newLow].Trust.Reporter.Tag; tag != "new" {
		t.Errorf("the new reporter's report reads tag %q, want new", tag)
	}
}
