// ratelimits_visibility_test.go proves the visibility bucket of review WR-E02
// (2026-10-30) exists in every place the limiter lives, like the people buckets:
// the default value, the resolver that fills a zero field, and the literal in
// cmd/server/main.go. A bucket missing from main.go would be a zero bucket in
// production, which refuses every turn on of the profile switch.
package api

import (
	"os"
	"regexp"
	"testing"
	"time"
)

func TestVisibilityBucketWiredEverywhere(t *testing.T) {
	d := RateLimitsDefault
	if d.Visibility != (BucketLimit{Every: 20 * time.Second, Burst: 6}) {
		t.Fatalf("RateLimitsDefault.Visibility = %+v, want one every 20s with a burst of 6", d.Visibility)
	}
	if got := resolveRateLimits(RateLimits{}); got.Visibility != d.Visibility {
		t.Fatalf("resolveRateLimits left Visibility at %+v, want %+v", got.Visibility, d.Visibility)
	}
	custom := BucketLimit{Every: 7, Burst: 3}
	if kept := resolveRateLimits(RateLimits{Visibility: custom}); kept.Visibility != custom {
		t.Fatalf("resolveRateLimits changed a set Visibility bucket: %+v", kept.Visibility)
	}

	raw, err := os.ReadFile("../../cmd/server/main.go")
	if err != nil {
		t.Fatalf("reading cmd/server/main.go: %v", err)
	}
	for _, want := range []string{
		`Visibility:\s+api\.BucketLimit\{Every: visibilityEvery, Burst: visibilityBurst\}`,
		`visibilityBurst\s+= 6`,
		`visibilityEvery\s+= 20 \* time\.Second`,
	} {
		if !regexp.MustCompile(want).MatchString(string(raw)) {
			t.Errorf("cmd/server/main.go does not match %q", want)
		}
	}
}
