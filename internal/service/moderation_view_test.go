// moderation_view_test.go declares package service (the internal test package,
// not service_test) because TestReportMetaSeriousMatchesBypass compares the
// exported Serious() with the unexported criticalBypass(), and because a view
// predicate added by a later phase may be unexported. It covers R-01 (a
// withheld report is listable in no view) and R-04 (one serious rule serves
// the critical bypass and the report allowance).

package service

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// withheldViewNow is the fixed clock of the view tests in this file.
var withheldViewNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

// withheldViewExpiries is every expiry side a view predicate can meet: unexpired,
// exactly at now, just before now, inside the 24 hour and 3 day windows, at the
// 7 day window, and beyond it.
func withheldViewExpiries() []time.Time {
	return []time.Time{
		withheldViewNow.Add(time.Hour),
		withheldViewNow,
		withheldViewNow.Add(-time.Second),
		withheldViewNow.Add(-RecentWindow / 2),
		withheldViewNow.Add(-RecentWindow),
		withheldViewNow.Add(-3 * 24 * time.Hour),
		withheldViewNow.Add(-HistoryWindow),
		withheldViewNow.Add(-HistoryWindow - time.Hour),
	}
}

// withheldViewBoolMethods returns the name of every bool method declared on the
// Visibility receiver in the non test files of this package, other than Valid.
func withheldViewBoolMethods(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	re := regexp.MustCompile(`(?m)^func \(\w+ Visibility\) (\w+)\([^)]*\) bool \{`)
	var names []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			if m[1] != "Valid" {
				names = append(names, m[1])
			}
		}
	}
	return names
}

// withheldViewFeedViewFuncs returns the name of every function or method in the
// non test files whose declaration line names FeedView, other than the two the
// guard skips by name: FeedView.Valid, and BuildHistoryFacts, which only builds
// the history fact words for a view that is already decided.
func withheldViewFeedViewFuncs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	re := regexp.MustCompile(`(?m)^func (?:\([^)]*\) )?(\w+)\([^\n]*FeedView[^\n]*\{$`)
	var names []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			if m[1] == "Valid" || m[1] == "BuildHistoryFacts" {
				continue
			}
			names = append(names, m[1])
		}
	}
	return names
}

// TestReportMetaSeriousMatchesBypass proves Serious() is criticalBypass() and
// ignores moderation (R-04).
func TestReportMetaSeriousMatchesBypass(t *testing.T) {
	states := []ModerationState{"", ModerationClear, ModerationPending, ModerationRemoved}
	for _, state := range states {
		for _, sev := range Severities {
			for _, cat := range Categories {
				meta := ReportMeta{Severity: sev, Category: cat, Moderation: state}
				bare := ReportMeta{Severity: sev, Category: cat}
				_, reason := Resolve(bare, VoteTally{}, withheldViewNow)
				viaResolve := reason == ReasonCriticalBypass
				got := meta.Serious()
				if got != meta.criticalBypass() || got != bare.criticalBypass() || got != viaResolve {
					t.Errorf("meta=%+v: Serious() = %v, criticalBypass() = %v, bare = %v, via Resolve = %v", meta, got, meta.criticalBypass(), bare.criticalBypass(), viaResolve)
				}
			}
		}
	}
}

// TestWithheldInNoView proves a withheld report is listable in no view (R-01):
// ListableInFeed and ViewOf, the Phase 3.1 view predicate, are driven with every
// visibility, and a source guard fails when a new bool method on Visibility or a
// new FeedView function is not exercised here.
func TestWithheldInNoView(t *testing.T) {
	if !VisibilityWithheld.Valid() {
		t.Fatal("VisibilityWithheld should be valid")
	}
	found := false
	for _, v := range Visibilities {
		if v == VisibilityWithheld {
			found = true
		}
	}
	if !found {
		t.Fatal("Visibilities should contain VisibilityWithheld")
	}

	t.Run("ListableInFeed", func(t *testing.T) {
		otherListable := false
		for _, v := range Visibilities {
			for _, includeDisputed := range []bool{false, true} {
				got := v.ListableInFeed(includeDisputed)
				if v == VisibilityWithheld && got {
					t.Errorf("withheld is listable with includeDisputed=%v", includeDisputed)
				}
				if v != VisibilityWithheld && got {
					otherListable = true
				}
			}
		}
		if !otherListable {
			t.Error("no other visibility is listable, so the predicate looks constant false")
		}
	})

	t.Run("ViewOf", func(t *testing.T) {
		otherValid := false
		for _, v := range Visibilities {
			for _, exp := range withheldViewExpiries() {
				got := ViewOf(v, exp, withheldViewNow)
				if got != FeedViewNone && !got.Valid() {
					t.Errorf("ViewOf(%q, %v) = %q, which is neither none nor a FeedView", v, exp, got)
				}
				if v == VisibilityWithheld && got != FeedViewNone {
					t.Errorf("ViewOf(withheld, %v) = %q, want none", exp, got)
				}
				if v != VisibilityWithheld && got.Valid() {
					otherValid = true
				}
			}
		}
		if !otherValid {
			t.Error("no other visibility lands in a view, so the partition looks empty")
		}
	})

	t.Run("ListableInAll", func(t *testing.T) {
		otherListable := false
		for _, v := range Visibilities {
			got := v.ListableInAll()
			if v == VisibilityWithheld && got {
				t.Error("withheld is listable in All")
			}
			if v != VisibilityWithheld && got {
				otherListable = true
			}
		}
		if !otherListable {
			t.Error("no other visibility is listable in All, so the predicate looks constant false")
		}
	})

	t.Run("ConfirmedByPeople", func(t *testing.T) {
		tally := VoteTally{ConfirmCells: IndependentAgreementThreshold + 3}
		for _, v := range Visibilities {
			for _, view := range append([]FeedView{FeedViewNone}, FeedViews...) {
				if v == VisibilityWithheld && ConfirmedByPeople(v, view, tally) {
					t.Errorf("a withheld report is confirmed in view %q", view)
				}
			}
		}
		if !ConfirmedByPeople(VisibilityLive, FeedViewActive, tally) {
			t.Error("a live confirmed active report is not confirmed, so the function looks constant false")
		}
	})

	// Added 2026-10-17 by Phase 3.2 plan 02 (D-29): the update predicate is a
	// free function, so the declaration guards do not reach it; this row does.
	t.Run("MayNotifyAboutReport", func(t *testing.T) {
		for _, k := range UpdateKinds {
			if MayNotifyAboutReport(k, VisibilityWithheld, withheldViewNow.Add(time.Hour), withheldViewNow) {
				t.Errorf("a withheld report may notify under kind %q", k)
			}
		}
		if !MayNotifyAboutReport(UpdateReachedLive, VisibilityLive, withheldViewNow.Add(time.Hour), withheldViewNow) {
			t.Error("a live unexpired report may not notify, so the predicate looks constant false")
		}
	})

	t.Run("source guard", func(t *testing.T) {
		src, err := os.ReadFile("moderation_view_test.go")
		if err != nil {
			t.Fatalf("read this file: %v", err)
		}
		own := string(src)
		methods := withheldViewBoolMethods(t)
		if len(methods) == 0 {
			t.Fatal("found no bool method on Visibility, the guard is not looking at the right files")
		}
		for _, name := range methods {
			if !strings.Contains(own, "."+name+"(") {
				t.Errorf("Visibility.%s returns bool but this test never calls it; prove withheld is not listable through it", name)
			}
		}
		funcs := withheldViewFeedViewFuncs(t)
		if len(funcs) == 0 {
			t.Fatal("found no FeedView function, the guard is not looking at the right files")
		}
		for _, name := range funcs {
			if !strings.Contains(own, name+"(") {
				t.Errorf("%s names FeedView but this test never calls it; prove withheld lands in no view through it", name)
			}
		}
	})
}
