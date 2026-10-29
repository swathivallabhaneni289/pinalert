package service_test

// publicname_fix_test.go pins WR-S04 of the Phase 9 code review (2026-10-30):
// the profile switch and a Name edit racing each other can never leave a public
// profile showing a Name that was not screened. The tests skip without
// DATABASE_URL.

import (
	"context"
	"errors"
	"testing"

	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

func storedName(t *testing.T, id int64, q *sqlcgen.Queries) string {
	t.Helper()
	row, err := q.GetPersonByID(context.Background(), id)
	if err != nil || row.DisplayName == nil {
		t.Fatalf("reading the Name: %v", err)
	}
	return *row.DisplayName
}

// A Name edit that lands while the switch's screen call runs must stop the
// switch: the screened Name is no longer the stored one.
func TestSwitchOnRefusesWhenTheNameChangedDuringTheScreen(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	id := seedNamedAccount(t, pool, "race.switch@example.com", "race.switch", "clear", ptr("Old Name"))

	screen := screenFn(func(text string) (moderation.Result, error) {
		if text == "Old Name" {
			// The concurrent edit commits while this call is in flight.
			testutil.MustExec(t, pool, `UPDATE accounts SET display_name = 'NDMA Control Room' WHERE id = $1`, id)
		}
		return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{}}, nil
	})
	svc := service.NewPeopleService(q, service.WithPeopleScreener(screen))
	err := svc.SetProfilePublic(ctx, id, true)
	var ve service.ValidationError
	if !errors.As(err, &ve) || ve.Field != "profile_public" || ve.Message != service.PublicNameUnfinishedMessage {
		t.Fatalf("SetProfilePublic = %v, want a profile_public ValidationError with the unfinished message", err)
	}
	if isPublic(t, pool, id) {
		t.Fatal("the profile went public with a Name that was not screened")
	}
}

// The other order: the profile turned on after the edit read the switch as off.
// The write of the unscreened Name must not land on a public profile.
func TestNameEditCannotLandOnAProfileThatTurnedPublic(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	id := seedNamedAccount(t, pool, "race.edit@example.com", "race.edit", "clear", ptr("Old Name"))
	testutil.MustExec(t, pool, `UPDATE accounts SET profile_public = true, profile_public_at = now() WHERE id = $1`, id)

	// The edit's read said "off" (a stale answer); the database says public.
	stale := profileReader{row: sqlcgen.GetPersonByIDRow{ID: id, DisplayName: ptr("Old Name"), ProfilePublic: false}}
	staleThenFresh := &switchReader{first: stale, real: q}
	svc := service.NewAccountService(q, service.WithPublicProfileReader(staleThenFresh))
	name := "NDMA Control Room"
	_, err := svc.UpdateProfile(ctx, id, service.ProfileUpdate{DisplayName: &name})
	var ve service.ValidationError
	if !errors.As(err, &ve) || ve.Field != "display_name" || ve.Message != service.PublicNameUnfinishedMessage {
		t.Fatalf("UpdateProfile = %v, want a display_name ValidationError with the unfinished message", err)
	}
	if got := storedName(t, id, q); got != "Old Name" {
		t.Fatalf("the stored Name = %q, want it unchanged", got)
	}

	// A screened edit of a public profile still lands.
	clean := screenAllClear
	svc2 := service.NewAccountService(q, service.WithAccountScreener(clean), service.WithPublicProfileReader(q))
	ok := "Ravi Kumar"
	if _, err := svc2.UpdateProfile(ctx, id, service.ProfileUpdate{DisplayName: &ok}); err != nil {
		t.Fatalf("a screened edit: %v", err)
	}
	if got := storedName(t, id, q); got != "Ravi Kumar" {
		t.Fatalf("the stored Name = %q, want Ravi Kumar", got)
	}
}

// switchReader answers the first read from a fixed stale row and every later
// read from the database.
type switchReader struct {
	first service.PublicProfileReader
	real  service.PublicProfileReader
	calls int
}

func (r *switchReader) GetPersonByID(ctx context.Context, id int64) (sqlcgen.GetPersonByIDRow, error) {
	r.calls++
	if r.calls == 1 {
		return r.first.GetPersonByID(ctx, id)
	}
	return r.real.GetPersonByID(ctx, id)
}
