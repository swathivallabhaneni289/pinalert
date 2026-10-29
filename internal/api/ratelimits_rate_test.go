// ratelimits_rate_test.go proves the Rate bucket of Phase 9 plan 14 (D-02,
// T-09-54, review W13) exists in every place the limiter lives: the default
// value, the resolver that fills a zero field, and the literal in
// cmd/server/main.go. A bucket missing from main.go would be a zero bucket in
// production, which refuses every rating.
package api

import (
	"os"
	"regexp"
	"testing"
)

func TestRateBucketWiredEverywhere(t *testing.T) {
	d := RateLimitsDefault
	if d.Rate == (BucketLimit{}) {
		t.Fatalf("RateLimitsDefault has a zero Rate bucket: %+v", d.Rate)
	}

	// A RateLimits with Rate left zero takes the default.
	got := resolveRateLimits(RateLimits{})
	if got.Rate != d.Rate {
		t.Fatalf("resolveRateLimits left Rate at %+v, want %+v", got.Rate, d.Rate)
	}

	// A value the caller set is never overwritten.
	custom := BucketLimit{Every: 7, Burst: 3}
	if kept := resolveRateLimits(RateLimits{Rate: custom}); kept.Rate != custom {
		t.Fatalf("resolveRateLimits changed a set Rate bucket: %+v", kept.Rate)
	}

	raw, err := os.ReadFile("../../cmd/server/main.go")
	if err != nil {
		t.Fatalf("reading cmd/server/main.go: %v", err)
	}
	src := string(raw)
	for _, want := range []string{
		`Rate:\s+api\.BucketLimit\{Every: rateEvery, Burst: rateBurst\}`,
		`rateBurst\s+= 10`,
		`rateEvery\s+= 30 \* time\.Second`,
	} {
		if !regexp.MustCompile(want).MatchString(src) {
			t.Errorf("cmd/server/main.go does not match %q", want)
		}
	}
}
