package handlers

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
)

// TestFeedViewParameterParsing pins the strict parameter rules of GET
// /api/reports (D-04, D-05, D-17, D-28) on the pure parsing helpers, so no
// database and no service is needed.
func TestFeedViewParameterParsing(t *testing.T) {
	viewCases := []struct {
		name      string
		query     string
		want      service.FeedView
		wantField string
	}{
		{"absent", "", service.FeedViewActive, ""},
		{"active", "view=active", service.FeedViewActive, ""},
		{"disputed", "view=disputed", service.FeedViewDisputed, ""},
		{"past", "view=past", service.FeedViewPast, ""},
		{"resolved alias", "view=resolved", service.FeedViewPast, ""},
		{"blank is absent", "view=", service.FeedViewActive, ""},
		{"capital Past", "view=Past", "", "view"},
		{"upper PAST", "view=PAST", "", "view"},
		{"all", "view=all", "", "view"},
		{"unknown", "view=bogus", "", "view"},
		{"show_disputed true alone", "show_disputed=true", service.FeedViewDisputed, ""},
		{"show_disputed true with disputed", "show_disputed=true&view=disputed", service.FeedViewDisputed, ""},
		{"show_disputed true with past", "show_disputed=true&view=past", "", "view"},
		{"show_disputed true with active", "show_disputed=true&view=active", "", "view"},
		{"show_disputed false with past", "show_disputed=false&view=past", service.FeedViewPast, ""},
		{"show_disputed false alone", "show_disputed=false", service.FeedViewActive, ""},
		{"show_disputed non boolean", "show_disputed=yes", "", "show_disputed"},
		{"show_disputed non boolean beats view", "show_disputed=yes&view=past", "", "show_disputed"},
	}
	for _, tc := range viewCases {
		t.Run("view "+tc.name, func(t *testing.T) {
			q, err := url.ParseQuery(tc.query)
			if err != nil {
				t.Fatalf("ParseQuery(%q): %v", tc.query, err)
			}
			got, problem := parseFeedView(q)
			if tc.wantField != "" {
				if problem == nil {
					t.Fatalf("parseFeedView(%q) = %q, want a problem on field %q", tc.query, got, tc.wantField)
				}
				if problem.Field != tc.wantField {
					t.Fatalf("problem field = %q, want %q", problem.Field, tc.wantField)
				}
				if problem.Message == "" {
					t.Fatalf("problem has no message")
				}
				return
			}
			if problem != nil {
				t.Fatalf("parseFeedView(%q) problem = %+v, want none", tc.query, problem)
			}
			if got != tc.want {
				t.Fatalf("parseFeedView(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}

	// The two messages the wire contract names.
	q, _ := url.ParseQuery("view=bogus")
	if _, p := parseFeedView(q); p == nil || p.Message != "view must be active, disputed or past." {
		t.Errorf("unknown view message = %+v", p)
	}
	q, _ = url.ParseQuery("show_disputed=true&view=past")
	if _, p := parseFeedView(q); p == nil || p.Message != "view and show_disputed disagree." {
		t.Errorf("conflict message = %+v", p)
	}

	categoryCases := []struct {
		query     string
		want      service.Category
		wantField bool
	}{
		{"", "", false},
		{"category=", "", false},
		{"category=flood", service.CategoryFlood, false},
		{"category=traffic", service.CategoryTraffic, false},
		{"category=rescue_needed", service.CategoryRescueNeeded, false},
		{"category=Flood", "", true},
		{"category=water", "", true},
	}
	for _, tc := range categoryCases {
		q, _ := url.ParseQuery(tc.query)
		got, problem := parseCategory(q)
		if tc.wantField {
			if problem == nil || problem.Field != "category" || problem.Message != "category must be one of the report categories." {
				t.Errorf("parseCategory(%q) problem = %+v, want field category", tc.query, problem)
			}
			continue
		}
		if problem != nil {
			t.Errorf("parseCategory(%q) problem = %+v, want none", tc.query, problem)
			continue
		}
		if got != tc.want {
			t.Errorf("parseCategory(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}

	reportCases := []struct {
		query     string
		wantID    int64
		wantSent  bool
		wantField bool
	}{
		{"", 0, false, false},
		{"report=", 0, false, false},
		{"report=7", 7, true, false},
		{"report=0", 0, false, true},
		{"report=-3", 0, false, true},
		{"report=abc", 0, false, true},
		{"report=1.5", 0, false, true},
		{"report=%2B5", 0, false, true},
		{"report=99999999999999999999", 0, false, true},
	}
	for _, tc := range reportCases {
		q, _ := url.ParseQuery(tc.query)
		id, sent, problem := parseReportParam(q)
		if tc.wantField {
			if problem == nil || problem.Field != "report" || problem.Message != "report must be a report id." {
				t.Errorf("parseReportParam(%q) problem = %+v, want field report", tc.query, problem)
			}
			continue
		}
		if problem != nil {
			t.Errorf("parseReportParam(%q) problem = %+v, want none", tc.query, problem)
			continue
		}
		if id != tc.wantID || sent != tc.wantSent {
			t.Errorf("parseReportParam(%q) = (%d, %v), want (%d, %v)", tc.query, id, sent, tc.wantID, tc.wantSent)
		}
	}
}

// TestNearbyRejectsBadFeedParametersBeforeTheService runs the handler with a
// nil querier: every case here must be refused with a 400 naming the field
// before the service is reached.
func TestNearbyRejectsBadFeedParametersBeforeTheService(t *testing.T) {
	cases := []struct {
		query string
		field string
	}{
		{"lat=12.97&lon=77.59&view=bogus", "view"},
		{"lat=12.97&lon=77.59&view=PAST", "view"},
		{"lat=12.97&lon=77.59&show_disputed=true&view=past", "view"},
		{"lat=12.97&lon=77.59&category=water", "category"},
		{"lat=12.97&lon=77.59&report=abc", "report"},
		{"lat=12.97&lon=77.59&report=0", "report"},
		{"lat=12.97&lon=77.59&show_disputed=yes", "show_disputed"},
	}
	for _, tc := range cases {
		status, field, _ := nearbyStatus(t, tc.query)
		if status != 400 {
			t.Errorf("%q: status = %d, want 400", tc.query, status)
		}
		if field != tc.field {
			t.Errorf("%q: field = %q, want %q", tc.query, field, tc.field)
		}
	}
}

// TestFeedReportResponseBuiltInOneMapper is a source guard: reports.go builds
// a FeedReportResponse literal in exactly one function, reportViewToResponse,
// so every path that puts a report on the wire shares the approximate guard
// (D-23). Slice literals and make calls are not struct literals and are not
// counted.
func TestFeedReportResponseBuiltInOneMapper(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "reports.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing reports.go: %v", err)
	}
	var owners []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if id, ok := lit.Type.(*ast.Ident); ok && id.Name == "FeedReportResponse" {
				owners = append(owners, fn.Name.Name)
			}
			return true
		})
	}
	if len(owners) != 1 || owners[0] != "reportViewToResponse" {
		t.Fatalf("FeedReportResponse literals are built in %v, want exactly one in reportViewToResponse", owners)
	}

	// The other handler files must not build one either.
	for _, name := range []string{"votes.go", "activity.go", "trust.go", "place.go", "page.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.CompositeLit); ok {
				if id, ok := lit.Type.(*ast.Ident); ok && id.Name == "FeedReportResponse" {
					t.Errorf("%s builds a FeedReportResponse literal", name)
				}
			}
			return true
		})
	}
}

// TestApproximateGuardBlanksDescription is defence in depth beside the
// service: a view marked approximate never carries description text, even if
// a caller forgot to blank it.
func TestApproximateGuardBlanksDescription(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	base := service.ReportView{
		Report: service.Report{
			ID: 5, Category: service.CategoryRescueNeeded, Severity: service.SeverityCritical,
			Description: "Two people on the roof of the blue house at the end of the lane",
			Latitude:    12.97, Longitude: 77.59, Geohash: "tdr1v",
			CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
		},
		View:        service.FeedViewResolved,
		Approximate: true,
	}
	got := reportViewToResponse(base)
	if got.Description != "" {
		t.Fatalf("approximate view description = %q, want empty", got.Description)
	}
	if !got.Approximate {
		t.Fatalf("approximate flag lost")
	}

	exact := base
	exact.Approximate = false
	if got := reportViewToResponse(exact); got.Description == "" {
		t.Fatalf("an exact view lost its description")
	}
}

// TestFeedReportWireShapeKeys pins the always present keys: your_resolution
// and history marshal as null when empty, and undos_left is an integer.
func TestFeedReportWireShapeKeys(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	v := service.ReportView{
		Report: service.Report{
			ID: 9, Category: service.CategoryFlood, Severity: service.SeverityLow,
			Description: "Water on the road", CreatedAt: now, ExpiresAt: now.Add(time.Hour),
		},
		View:       service.FeedViewActive,
		Visibility: service.VisibilityLive,
		UndosLeft:  2,
	}
	raw, err := json.Marshal(reportViewToResponse(v))
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	for _, key := range []string{"your_resolution", "history"} {
		val, ok := m[key]
		if !ok {
			t.Errorf("key %q missing from %s", key, raw)
		} else if val != nil {
			t.Errorf("key %q = %v, want null", key, val)
		}
	}
	if m["view"] != "active" {
		t.Errorf("view = %v, want active", m["view"])
	}
	if n, ok := m["undos_left"].(float64); !ok || n != 2 {
		t.Errorf("undos_left = %v, want 2", m["undos_left"])
	}
	if m["approximate"] != false {
		t.Errorf("approximate = %v, want false", m["approximate"])
	}

	// A resolved report carries its history and the viewer's resolution.
	v.View = service.FeedViewResolved
	v.ViewerResolution = service.VoteResolve
	v.History = &service.HistoryFacts{
		State: service.HistoryStateResolved, At: now, ClosedBy: service.ClosedByYou, ClosedText: "You closed it",
	}
	raw, _ = json.Marshal(reportViewToResponse(v))
	body := string(raw)
	for _, want := range []string{`"your_resolution":"resolve"`, `"state":"resolved"`, `"closed_by":"you"`, `"view":"resolved"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body %s is missing %s", body, want)
		}
	}
	for _, absent := range []string{`"outcome"`, `"lifetime_text"`} {
		if strings.Contains(body, absent) {
			t.Errorf("a resolved history carries %s: %s", absent, body)
		}
	}
}
