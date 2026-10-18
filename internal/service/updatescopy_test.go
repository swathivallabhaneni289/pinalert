// updatescopy_test.go covers every server written word of an update (plan
// 03.2-02): the event lines, the trust line chosen at read time, the rolled up
// line, the unavailable line and the report first line.

package service

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	sqlcgen "pinalert/internal/store/sqlc"
)

func TestUpdatesCopy(t *testing.T) {
	events := []struct {
		kind UpdateKind
		want string
	}{
		{UpdateReachedLive, "Now confirmed"},
		{UpdateHidden, "Now disputed"},
		{UpdateResolved, "Marked resolved"},
		{UpdateFading, "Fades in about an hour"},
		{UpdateReliableNearby, "Reported nearby by a Reliable reporter."},
		{UpdateMore, ""},
		{"comment", ""},
		{"", ""},
	}
	for _, e := range events {
		if got := EventLine(e.kind); got != e.want {
			t.Errorf("EventLine(%q) = %q, want %q", e.kind, got, e.want)
		}
	}
	for _, k := range UpdateEventKinds {
		if EventLine(k) == "" {
			t.Errorf("event kind %q has no line", k)
		}
	}

	if got := MoreLine(1); got != "And 1 more change" {
		t.Errorf("MoreLine(1) = %q", got)
	}
	if got := MoreLine(3); got != "And 3 more changes" {
		t.Errorf("MoreLine(3) = %q", got)
	}
	if got := MoreLine(12); got != "And 12 more changes" {
		t.Errorf("MoreLine(12) = %q", got)
	}
	if UnavailableLine != "This report is no longer available" {
		t.Errorf("UnavailableLine = %q", UnavailableLine)
	}
	if UnconfirmedLine != "Not confirmed yet." {
		t.Errorf("UnconfirmedLine = %q", UnconfirmedLine)
	}

	// No en dash or em dash anywhere in the two source files (house rule), and
	// no coordinate or email shaped text in any server written line.
	for _, name := range []string{"updatescopy.go", "updates.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, r := range string(src) {
			if r == '–' || r == '—' {
				t.Errorf("%s holds a dash rune %U", name, r)
			}
		}
	}
	lines := []string{UnavailableLine, UnconfirmedLine, MoreLine(1), MoreLine(7)}
	for _, k := range UpdateKinds {
		lines = append(lines, EventLine(k))
	}
	coord := regexp.MustCompile(`\d+\.\d{3,}|@`)
	for _, l := range lines {
		if coord.MatchString(l) {
			t.Errorf("line %q looks like a coordinate or an email address", l)
		}
	}
}

// updateTrustGridRows builds vote rows for report 1 from confirming and
// disputing cell lists.
func updateTrustGridRows(confirmCells, disputeCells []string) []sqlcgen.CurrentVotesForReportsRow {
	var rows []sqlcgen.CurrentVotesForReportsRow
	var acct int64 = 100
	for _, c := range confirmCells {
		acct++
		rows = append(rows, sqlcgen.CurrentVotesForReportsRow{ReportID: 1, AccountID: acct, Kind: string(VoteKindContent), Value: string(VoteConfirm), GeohashCell: c})
	}
	for _, c := range disputeCells {
		acct++
		rows = append(rows, sqlcgen.CurrentVotesForReportsRow{ReportID: 1, AccountID: acct, Kind: string(VoteKindContent), Value: string(VoteDispute), GeohashCell: c})
	}
	return rows
}

func TestUpdateTrustLineNeverEmpty(t *testing.T) {
	now := updatesNow
	reporter := int64(7)
	type grid struct {
		name     string
		confirms []string
		disputes []string
		resolve  bool
		meta     ReportMeta
		expires  time.Time
	}
	future := now.Add(5 * time.Hour)
	grids := []grid{
		{name: "provisional low, nobody", meta: ReportMeta{Severity: SeverityLow, Category: CategoryFlood}, expires: future},
		{name: "provisional, one confirm", confirms: []string{"tdr1aa"}, meta: ReportMeta{Severity: SeverityMedium, Category: CategoryFlood}, expires: future},
		{name: "live, two confirms", confirms: []string{"tdr1aa", "tdr1bb"}, meta: ReportMeta{Severity: SeverityMedium, Category: CategoryFlood}, expires: future},
		{name: "live critical, nobody", meta: ReportMeta{Severity: SeverityCritical, Category: CategoryFlood}, expires: future},
		{name: "hidden", confirms: []string{"tdr1aa"}, disputes: []string{"tdr1cc", "tdr1dd"}, meta: ReportMeta{Severity: SeverityLow, Category: CategoryFlood}, expires: future},
		{name: "retracted", confirms: []string{"tdr1aa", "tdr1bb"}, resolve: true, meta: ReportMeta{Severity: SeverityLow, Category: CategoryFlood}, expires: future},
		{name: "expired live", confirms: []string{"tdr1aa", "tdr1bb"}, meta: ReportMeta{Severity: SeverityLow, Category: CategoryFlood}, expires: now.Add(-time.Minute)},
		{name: "expired retracted", resolve: true, meta: ReportMeta{Severity: SeverityLow, Category: CategoryFlood}, expires: now.Add(-time.Minute)},
		{name: "many confirms", confirms: []string{"a1", "a2", "a3", "a4", "a5", "a6"}, meta: ReportMeta{Severity: SeverityLow, Category: CategoryTraffic}, expires: future},
	}
	for _, g := range grids {
		rows := updateTrustGridRows(g.confirms, g.disputes)
		if g.resolve {
			rows = append(rows, sqlcgen.CurrentVotesForReportsRow{ReportID: 1, AccountID: reporter, Kind: string(VoteKindResolution), Value: string(VoteResolve), GeohashCell: "zzzzzz"})
		}
		tally := BuildVoteTally(rows, 1, &reporter)
		vis, _ := Resolve(g.meta, tally, now)
		view := ViewOf(vis, g.expires, now)
		trust := BuildTrust(TrustInput{
			Rows: rows, ReportID: 1, ReporterAccountID: &reporter, Meta: g.meta,
			Visibility: vis, CreatedAt: now.Add(-time.Hour), ExpiresAt: g.expires, Now: now,
		})
		confirmedValues := []bool{false, true, ConfirmedByPeople(vis, view, tally)}
		for _, k := range UpdateEventKinds {
			for _, confirmed := range confirmedValues {
				line := UpdateTrustLine(k, trust, view, confirmed)
				if strings.TrimSpace(line) == "" {
					t.Errorf("%s: kind %q view %q confirmed %v gives an empty line", g.name, k, view, confirmed)
				}
			}
		}
	}

	// A Provisional low report and a critical report nobody confirmed both read
	// Not confirmed yet. under the Reliable note; two confirming cells make the
	// same row read the confirmed sentence (owner amendment 2026-10-18).
	low := grids[0]
	rows := updateTrustGridRows(nil, nil)
	tally := BuildVoteTally(rows, 1, &reporter)
	vis, _ := Resolve(low.meta, tally, now)
	view := ViewOf(vis, low.expires, now)
	trust := BuildTrust(TrustInput{Rows: rows, ReportID: 1, ReporterAccountID: &reporter, Meta: low.meta, Visibility: vis, ExpiresAt: low.expires, Now: now})
	if vis != VisibilityProvisional {
		t.Fatalf("low report resolved to %q, want provisional", vis)
	}
	if got := UpdateTrustLine(UpdateReliableNearby, trust, view, ConfirmedByPeople(vis, view, tally)); got != UnconfirmedLine {
		t.Errorf("provisional low report reads %q, want %q", got, UnconfirmedLine)
	}

	crit := grids[3]
	vis, _ = Resolve(crit.meta, VoteTally{}, now)
	view = ViewOf(vis, crit.expires, now)
	trust = BuildTrust(TrustInput{ReportID: 1, ReporterAccountID: &reporter, Meta: crit.meta, Visibility: vis, ExpiresAt: crit.expires, Now: now})
	if vis != VisibilityLive {
		t.Fatalf("critical report resolved to %q, want live", vis)
	}
	if got := UpdateTrustLine(UpdateReliableNearby, trust, view, ConfirmedByPeople(vis, view, VoteTally{})); got != UnconfirmedLine {
		t.Errorf("critical unconfirmed report reads %q, want %q", got, UnconfirmedLine)
	}

	live := grids[2]
	rows = updateTrustGridRows(live.confirms, nil)
	tally = BuildVoteTally(rows, 1, &reporter)
	vis, _ = Resolve(live.meta, tally, now)
	view = ViewOf(vis, live.expires, now)
	trust = BuildTrust(TrustInput{Rows: rows, ReportID: 1, ReporterAccountID: &reporter, Meta: live.meta, Visibility: vis, ExpiresAt: live.expires, Now: now})
	confirmed := ConfirmedByPeople(vis, view, tally)
	if !confirmed {
		t.Fatal("two confirming cells should be confirmed by people")
	}
	if got := UpdateTrustLine(UpdateReliableNearby, trust, view, confirmed); got != trust.ConfirmedSentence || got != "Confirmed by 2 people at different spots." {
		t.Errorf("confirmed row reads %q, want the confirmed sentence %q", got, trust.ConfirmedSentence)
	}
}

func TestUpdateTrustLineChoices(t *testing.T) {
	trust := Trust{LeadSentence: "LEAD", ConfirmedSentence: "CONFIRMED", DisputedSentence: "DISPUTED"}
	noLead := Trust{ConfirmedSentence: "CONFIRMED", DisputedSentence: "DISPUTED"}
	cases := []struct {
		name      string
		kind      UpdateKind
		trust     Trust
		view      FeedView
		confirmed bool
		want      string
	}{
		{name: "reliable active unconfirmed", kind: UpdateReliableNearby, trust: trust, view: FeedViewActive, want: "Not confirmed yet."},
		{name: "reliable active confirmed", kind: UpdateReliableNearby, trust: trust, view: FeedViewActive, confirmed: true, want: "CONFIRMED"},
		{name: "reliable disputed", kind: UpdateReliableNearby, trust: trust, view: FeedViewDisputed, want: "DISPUTED"},
		{name: "reliable resolved with lead", kind: UpdateReliableNearby, trust: trust, view: FeedViewResolved, want: "LEAD"},
		{name: "reliable past without lead", kind: UpdateReliableNearby, trust: noLead, view: FeedViewPast, want: "CONFIRMED"},
		{name: "reliable no view", kind: UpdateReliableNearby, trust: noLead, view: FeedViewNone, want: "CONFIRMED"},
		{name: "hidden", kind: UpdateHidden, trust: trust, view: FeedViewDisputed, want: "DISPUTED"},
		{name: "reached live", kind: UpdateReachedLive, trust: trust, view: FeedViewActive, confirmed: true, want: "CONFIRMED"},
		{name: "resolved with lead", kind: UpdateResolved, trust: trust, view: FeedViewResolved, want: "LEAD"},
		{name: "resolved without lead", kind: UpdateResolved, trust: noLead, view: FeedViewResolved, want: "CONFIRMED"},
		{name: "fading with lead", kind: UpdateFading, trust: trust, view: FeedViewActive, want: "LEAD"},
		{name: "fading without lead", kind: UpdateFading, trust: noLead, view: FeedViewActive, want: "CONFIRMED"},
		{name: "more has no trust line", kind: UpdateMore, trust: trust, view: FeedViewActive, want: ""},
		{name: "unknown kind has no trust line", kind: "comment", trust: trust, view: FeedViewActive, want: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := UpdateTrustLine(c.kind, c.trust, c.view, c.confirmed); got != c.want {
				t.Errorf("UpdateTrustLine = %q, want %q", got, c.want)
			}
		})
	}
}

func TestReportFirstLine(t *testing.T) {
	long := strings.Repeat("a", 300)
	multibyte := strings.Repeat("అ", 200)
	cases := []struct {
		name     string
		report   Report
		finished bool
		want     string
	}{
		{name: "multi line text keeps the first line", report: Report{Category: CategoryFlood, Description: "Water at the bridge\nsecond line\nthird"}, want: "Water at the bridge"},
		{name: "windows line break", report: Report{Category: CategoryFlood, Description: "Road is blocked\r\nand more"}, want: "Road is blocked"},
		{name: "leading blank lines and spaces are trimmed", report: Report{Category: CategoryFlood, Description: "\n  \n   Shelter open at school  \nmore"}, want: "Shelter open at school"},
		{name: "300 rune line is cut to 160", report: Report{Category: CategoryFlood, Description: long}, want: strings.Repeat("a", ReportLineMaxRunes)},
		{name: "cut counts runes not bytes", report: Report{Category: CategoryFlood, Description: multibyte}, want: strings.Repeat("అ", ReportLineMaxRunes)},
		{name: "exactly 160 is kept whole", report: Report{Category: CategoryFlood, Description: strings.Repeat("b", 160)}, want: strings.Repeat("b", 160)},
		{name: "finished rescue shows no text", report: Report{Category: CategoryRescueNeeded, Description: "Family on the roof at 12 Lake Road"}, finished: true, want: ""},
		{name: "unfinished rescue shows the line", report: Report{Category: CategoryRescueNeeded, Description: "Family on the roof\nphone"}, finished: false, want: "Family on the roof"},
		{name: "finished flood shows the line", report: Report{Category: CategoryFlood, Description: "Water gone"}, finished: true, want: "Water gone"},
		{name: "empty description", report: Report{Category: CategoryFlood}, want: ""},
		{name: "all space description", report: Report{Category: CategoryFlood, Description: " \t \n  "}, want: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ReportFirstLine(c.report, c.finished)
			if got != c.want {
				t.Errorf("ReportFirstLine = %q, want %q", got, c.want)
			}
			if utf8.RuneCountInString(got) > ReportLineMaxRunes {
				t.Errorf("line has %d runes, max %d", utf8.RuneCountInString(got), ReportLineMaxRunes)
			}
		})
	}
}
