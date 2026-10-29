package service_test

// people_cursor_list_test.go pins SK-W03 of the Phase 9 skeptic round
// (2026-10-30) over a real database: a next cursor issued on one people list is
// refused, with the same calm sentence, on any other list. The tests skip
// without DATABASE_URL (NewTestDB).

import (
	"context"
	"testing"
	"time"

	"pinalert/internal/testutil"
)

func TestPeopleCursorOpensOnlyOnItsOwnList(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	base := time.Now().Add(-time.Hour).Truncate(time.Second)

	ownerA := testutil.SeedPublicReporter(t, pool, "cursorlist-a@example.com", "cursorlist.a", "cursorlist-a-sess", base)
	testutil.SeedPublicReporter(t, pool, "cursorlist-b@example.com", "cursorlist.b", "cursorlist-b-sess", base)
	viewer := testutil.SeedPerson(t, pool, "cursorlist-v@example.com", "cursorlist.viewer", false)
	ids := seedBulkPeople(t, pool, "cl", 55, true)
	for i, id := range ids {
		testutil.SeedFollow(t, pool, id, ownerA, base.Add(time.Duration(i)*time.Second))
	}

	first, err := svc.Followers(ctx, viewer, "cursorlist.a", "")
	if err != nil || first.Next == "" {
		t.Fatalf("first page: next %q, err %v, want a cursor", first.Next, err)
	}
	if _, err := svc.Followers(ctx, viewer, "cursorlist.a", first.Next); err != nil {
		t.Fatalf("the list it was issued for refused it: %v", err)
	}
	if _, err := svc.Followers(ctx, viewer, "cursorlist.b", first.Next); err == nil {
		t.Error("another person's Followers list accepted the cursor")
	} else {
		wantValidation(t, err, "after", "That page could not be read.")
	}
	if _, err := svc.Following(ctx, viewer, "cursorlist.a", first.Next); err == nil {
		t.Error("the Following list of the same person accepted a Followers cursor")
	} else {
		wantValidation(t, err, "after", "That page could not be read.")
	}
}
