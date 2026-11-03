package service

// ratings_wiring_test.go proves the tapped ratings source is plugged into every
// reader of a reporter tag (Phase 9 plan 14, research Pitfall 2, T-09-52). A
// reader left without the source would show a tag that ignores the ratings while
// every other surface, and the phone, show the rated one. The test reads the two
// files that build the services as text, so a new reader that is added without
// the source fails here and names what is missing.

import (
	"os"
	"strings"
	"testing"
)

// ratingsWiringCalls lists, per file (paths relative to internal/service, where
// go test runs), every call the file must contain. Plan 09-26 added
// WithPushWorkerSignals(ratingService) to the main.go list when the push worker
// gained its option (review W8).
var ratingsWiringCalls = []struct {
	file  string
	calls []string
}{
	{
		file: "../../cmd/server/main.go",
		calls: []string{
			"WithReportSignals(ratingService)",
			"WithVotingSignals(ratingService)",
			"WithVotingRatePrompts(ratingService)",
			"WithAuthSignals(ratingService)",
			// Amended 2026-11-03 by quick 261103-jal (review WR-04): the dispatcher
			// no longer reads a reporter tag (its Reliable path is gone), so
			// WithDispatchSignals is removed and is no longer listed here.
			"WithFollowSweepSignals(ratingService)",
			// 2026-10-30, plan 09-26 (review W8): the push worker rechecks a follow
			// note against the rated tag at send time.
			"WithPushWorkerSignals(ratingService)",
		},
	},
	{
		file: "../api/router.go",
		calls: []string{
			"WithPeopleSignals(deps.Ratings)",
		},
	},
}

// TestRatingsSourceWiredEverywhere fails, naming the file and the call, unless
// every tag reader is given the one ratings source.
func TestRatingsSourceWiredEverywhere(t *testing.T) {
	for _, f := range ratingsWiringCalls {
		raw, err := os.ReadFile(f.file)
		if err != nil {
			t.Fatalf("reading %s: %v", f.file, err)
		}
		text := string(raw)
		for _, call := range f.calls {
			if !strings.Contains(text, call) {
				t.Errorf("%s does not contain %s: a reader of the reporter tag is left without the ratings source", f.file, call)
			}
		}
	}
}
