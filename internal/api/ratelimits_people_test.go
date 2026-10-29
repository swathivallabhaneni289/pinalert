// ratelimits_people_test.go proves the two people buckets of Phase 9 plan 07
// exist in every place the limiter lives: the default value, the resolver that
// fills a zero field, and the literal in cmd/server/main.go. A bucket missing
// from main.go would be a zero bucket in production, which refuses everything.
package api

import (
	"os"
	"regexp"
	"testing"
)

func TestPeopleBucketsWiredEverywhere(t *testing.T) {
	d := RateLimitsDefault
	if d.PeopleRead == (BucketLimit{}) || d.PeopleAction == (BucketLimit{}) {
		t.Fatalf("RateLimitsDefault has a zero people bucket: read %+v action %+v", d.PeopleRead, d.PeopleAction)
	}

	got := resolveRateLimits(RateLimits{})
	if got.PeopleRead != d.PeopleRead {
		t.Fatalf("resolveRateLimits left PeopleRead at %+v, want %+v", got.PeopleRead, d.PeopleRead)
	}
	if got.PeopleAction != d.PeopleAction {
		t.Fatalf("resolveRateLimits left PeopleAction at %+v, want %+v", got.PeopleAction, d.PeopleAction)
	}

	// A value the caller set is never overwritten.
	custom := BucketLimit{Every: 7, Burst: 3}
	kept := resolveRateLimits(RateLimits{PeopleRead: custom, PeopleAction: custom})
	if kept.PeopleRead != custom || kept.PeopleAction != custom {
		t.Fatalf("resolveRateLimits changed a set people bucket: %+v %+v", kept.PeopleRead, kept.PeopleAction)
	}

	raw, err := os.ReadFile("../../cmd/server/main.go")
	if err != nil {
		t.Fatalf("reading cmd/server/main.go: %v", err)
	}
	src := string(raw)
	for _, want := range []string{
		`PeopleRead:\s+api\.BucketLimit\{Every: peopleReadEvery, Burst: peopleReadBurst\}`,
		`PeopleAction:\s+api\.BucketLimit\{Every: peopleActionEvery, Burst: peopleActionBurst\}`,
		`peopleReadBurst\s+= 60`,
		`peopleReadEvery\s+= time\.Second`,
		`peopleActionBurst\s+= 20`,
		`peopleActionEvery\s+= 3 \* time\.Second`,
	} {
		if !regexp.MustCompile(want).MatchString(src) {
			t.Errorf("cmd/server/main.go does not match %q", want)
		}
	}
}
