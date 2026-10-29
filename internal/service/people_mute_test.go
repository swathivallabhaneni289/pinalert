package service_test

// people_mute_test.go proves person mute over a real database (plan 09-18, D-12,
// research G4, T-09-65, T-09-66): the rules of Mute and Unmute, that a mute
// never hides a report from any read, the Muted people list and its count, the
// Muted field of the profile, and that deleting an account removes the mutes on
// both sides. The tests skip without DATABASE_URL (NewTestDB).

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// muteRows counts the person_mutes rows of one pair.
func muteRows(t *testing.T, pool *pgxpool.Pool, muter, muted int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM person_mutes WHERE muter_id = $1 AND muted_id = $2", muter, muted).Scan(&n); err != nil {
		t.Fatalf("counting person_mutes: %v", err)
	}
	return n
}

// TestMuteRules: the answers of Mute and Unmute (D-12, review W7).
func TestMuteRules(t *testing.T) {
	pool, svc := newPeople(t)
	ctx := context.Background()

	me := testutil.SeedPerson(t, pool, "mute-me@example.com", "mute.me", false)
	open := testutil.SeedPerson(t, pool, "mute-open@example.com", "open.person", true)
	closed := testutil.SeedPerson(t, pool, "mute-closed@example.com", "closed.person", false)
	pending := seedNamedAccount(t, pool, "mute-pending@example.com", "pend.ing", "pending", nil)
	testutil.MustExec(t, pool, "UPDATE accounts SET profile_public = true, profile_public_at = now() WHERE id = $1", pending)
	blocker := testutil.SeedPerson(t, pool, "mute-blocker@example.com", "has.blocked", true)
	testutil.SeedBlock(t, pool, blocker, me)

	t.Run("a visible person can be muted and the row is written once", func(t *testing.T) {
		if err := svc.Mute(ctx, me, "open.person"); err != nil {
			t.Fatalf("Mute: %v", err)
		}
		if err := svc.Mute(ctx, me, "Open.Person"); err != nil {
			t.Fatalf("a repeat Mute (typed with capitals): %v", err)
		}
		if n := muteRows(t, pool, me, open); n != 1 {
			t.Errorf("rows after muting twice = %d, want 1", n)
		}
		if n := muteRows(t, pool, open, me); n != 0 {
			t.Errorf("the opposite row = %d, want 0 (a mute is one way)", n)
		}
	})

	t.Run("an unseen, unknown, own or malformed name is one not found", func(t *testing.T) {
		for _, c := range []struct{ name, username string }{
			{"a private profile", "closed.person"},
			{"a pending username", "pend.ing"},
			{"a person who blocked you", "has.blocked"},
			{"an unknown name", "nobody.here"},
			{"yourself", "mute.me"},
			{"a malformed name", "no"},
			{"an empty name", ""},
		} {
			if err := svc.Mute(ctx, me, c.username); !errors.Is(err, service.ErrPersonNotFound) {
				t.Errorf("Mute of %s = %v, want ErrPersonNotFound", c.name, err)
			}
		}
		for _, id := range []int64{closed, pending, blocker, me} {
			if n := muteRows(t, pool, me, id); n != 0 {
				t.Errorf("a refused mute wrote %d rows for account %d", n, id)
			}
		}
	})

	t.Run("a mute does not end a follow and writes nothing else", func(t *testing.T) {
		testutil.SeedFollow(t, pool, me, open, time.Now().UTC())
		if err := svc.Mute(ctx, me, "open.person"); err != nil {
			t.Fatalf("Mute: %v", err)
		}
		if !activeFollows(t, pool, me, open) {
			t.Error("muting ended the follow; a mute is not an unfollow")
		}
	})

	t.Run("unmute works for any live account, also after it went private", func(t *testing.T) {
		testutil.MustExec(t, pool, "UPDATE accounts SET profile_public = false WHERE id = $1", open)
		if err := svc.Unmute(ctx, me, "open.person"); err != nil {
			t.Fatalf("Unmute after the person went private: %v", err)
		}
		if n := muteRows(t, pool, me, open); n != 0 {
			t.Errorf("rows after Unmute = %d, want 0", n)
		}
	})

	t.Run("unmute of a well formed name that matches nobody is nil, a malformed one is not found", func(t *testing.T) {
		if err := svc.Unmute(ctx, me, "nobody.here"); err != nil {
			t.Errorf("Unmute of an unknown well formed name = %v, want nil", err)
		}
		if err := svc.Unmute(ctx, me, "closed.person"); err != nil {
			t.Errorf("Unmute of someone never muted = %v, want nil", err)
		}
		if err := svc.Unmute(ctx, me, "no"); !errors.Is(err, service.ErrPersonNotFound) {
			t.Errorf("Unmute of a malformed name = %v, want ErrPersonNotFound", err)
		}
	})

	t.Run("an unmute removes only the caller's own row", func(t *testing.T) {
		other := testutil.SeedPerson(t, pool, "mute-other@example.com", "other.muter", true)
		testutil.SeedMute(t, pool, other, closed)
		testutil.SeedMute(t, pool, me, closed)
		if err := svc.Unmute(ctx, me, "closed.person"); err != nil {
			t.Fatalf("Unmute: %v", err)
		}
		if n := muteRows(t, pool, other, closed); n != 1 {
			t.Errorf("another person's mute = %d rows, want it untouched (1)", n)
		}
	})
}

// reportIDs returns the sorted ids of views.
func reportIDs(views []service.ReportView) []int64 {
	ids := make([]int64, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sameInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestMuteNeverHidesReports (T-09-65, D-12): after a mute, Nearby, Focus,
// History and ProfileReports list exactly what they listed before, and the
// reporter's profile still opens. Mute stops person level notes only.
func TestMuteNeverHidesReports(t *testing.T) {
	pool, people := newPeople(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	const lat, lon = 12.9716, 77.5946

	const session = "mute-reporter-session"
	reporter := testutil.SeedPublicReporter(t, pool, "mute-reporter@example.com", "muted.reporter", session, now)
	reader := testutil.SeedPerson(t, pool, "mute-reader@example.com", "the.reader", false)
	live := testutil.SeedHistoryReport(t, pool, session, "low", "flood", now.Add(6*time.Hour))
	critical := testutil.SeedHistoryReport(t, pool, session, "critical", "flood", now.Add(6*time.Hour))

	reports := service.NewReportService(sqlcgen.New(pool), service.WithReportClock(func() time.Time { return now }))
	type snapshot struct {
		nearby, history []int64
		focus           service.FocusStatus
		profileRows     int
	}
	read := func() snapshot {
		t.Helper()
		nearby, err := reports.Nearby(ctx, service.NearbyQuery{
			Latitude: lat, Longitude: lon, RadiusKm: 10, ViewerAccountID: reader, All: true,
		})
		if err != nil {
			t.Fatalf("Nearby: %v", err)
		}
		focus, err := reports.Focus(ctx, service.FocusQuery{ReportID: live, Latitude: lat, Longitude: lon, ViewerAccountID: reader})
		if err != nil {
			t.Fatalf("Focus: %v", err)
		}
		history, err := reports.History(ctx, service.HistoryQuery{Latitude: lat, Longitude: lon, RadiusKm: 10, ViewerAccountID: reader})
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		page, err := reports.ProfileReports(ctx, service.ProfileQuery{AccountID: reporter, ViewerAccountID: reader, Now: now})
		if err != nil {
			t.Fatalf("ProfileReports: %v", err)
		}
		return snapshot{
			nearby:      reportIDs(nearby),
			history:     reportIDs(history.Reports),
			focus:       focus.Status,
			profileRows: len(page.Rows),
		}
	}

	before := read()
	if !sameInts(before.nearby, []int64{live, critical}) {
		t.Fatalf("seed check: Nearby listed %v, want the two live reports %v", before.nearby, []int64{live, critical})
	}
	if before.focus != service.FocusOK || len(before.history) == 0 || before.profileRows == 0 {
		t.Fatalf("seed check: focus %q, %d history reports, %d profile rows, want all non empty", before.focus, len(before.history), before.profileRows)
	}

	if err := people.Mute(ctx, reader, "muted.reporter"); err != nil {
		t.Fatalf("Mute: %v", err)
	}
	after := read()
	if !sameInts(after.nearby, before.nearby) {
		t.Errorf("Nearby after the mute = %v, want %v", after.nearby, before.nearby)
	}
	if after.focus != before.focus {
		t.Errorf("Focus after the mute = %q, want %q", after.focus, before.focus)
	}
	if !sameInts(after.history, before.history) {
		t.Errorf("History after the mute = %v, want %v", after.history, before.history)
	}
	if after.profileRows != before.profileRows {
		t.Errorf("ProfileReports rows after the mute = %d, want %d", after.profileRows, before.profileRows)
	}
	if _, err := people.Profile(ctx, reader, "muted.reporter"); err != nil {
		t.Errorf("the muted person's profile no longer opens: %v", err)
	}
}

// TestMutedPeopleList: newest first, live accounts with a clear username only,
// private people included, capped at MutedPeopleListCap; the count counts the
// same live named people (D-12).
func TestMutedPeopleList(t *testing.T) {
	pool, svc := newPeople(t)
	ctx := context.Background()

	me := testutil.SeedPerson(t, pool, "mlist-me@example.com", "list.owner", false)
	a := testutil.SeedPerson(t, pool, "mlist-a@example.com", "older.one", true)
	b := testutil.SeedPerson(t, pool, "mlist-b@example.com", "newer.one", false)
	gone := testutil.SeedPerson(t, pool, "mlist-gone@example.com", "gone.one", true)
	pend := seedNamedAccount(t, pool, "mlist-pend@example.com", "pend.one", "pending", nil)

	if got, err := svc.MutedPeople(ctx, me); err != nil || len(got) != 0 {
		t.Fatalf("an empty list = %v, %v; want empty and no error", got, err)
	}
	if n, err := svc.MutedPeopleCount(ctx, me); err != nil || n != 0 {
		t.Fatalf("an empty count = %d, %v; want 0", n, err)
	}

	for i, id := range []int64{a, gone, pend, b} {
		testutil.SeedMute(t, pool, me, id)
		testutil.MustExec(t, pool, "UPDATE person_mutes SET created_at = $3 WHERE muter_id = $1 AND muted_id = $2",
			me, id, time.Date(2026, 10, 1, 12, i, 0, 0, time.UTC))
	}
	testutil.MustExec(t, pool, "UPDATE accounts SET deleted_at = now() WHERE id = $1", gone)

	got, err := svc.MutedPeople(ctx, me)
	if err != nil {
		t.Fatalf("MutedPeople: %v", err)
	}
	want := []string{"newer.one", "older.one"}
	if len(got) != len(want) {
		t.Fatalf("list = %v, want %v (a deleted account and a pending name are left out)", got, want)
	}
	for i, e := range got {
		if e.Username != want[i] {
			t.Errorf("entry %d = %q, want %q (newest first, a private profile included)", i, e.Username, want[i])
		}
	}
	// The count sees the same live named people; a pending name is counted, as
	// the Blocked people count does, and a deleted account is not.
	if n, err := svc.MutedPeopleCount(ctx, me); err != nil || n != 3 {
		t.Errorf("count = %d, %v; want 3 (older, newer and the pending name)", n, err)
	}
	if other, err := svc.MutedPeople(ctx, a); err != nil || len(other) != 0 {
		t.Errorf("another person's list = %v, %v; want empty (mutes are private)", other, err)
	}

	t.Run("the list is capped", func(t *testing.T) {
		big := testutil.SeedPerson(t, pool, "mlist-big@example.com", "big.owner", false)
		ids := seedBulkPeople(t, pool, "mutebulk", service.MutedPeopleListCap+1, true)
		for _, id := range ids {
			testutil.SeedMute(t, pool, big, id)
		}
		list, err := svc.MutedPeople(ctx, big)
		if err != nil {
			t.Fatalf("MutedPeople: %v", err)
		}
		if len(list) != service.MutedPeopleListCap {
			t.Errorf("list length = %d, want the cap %d", len(list), service.MutedPeopleListCap)
		}
		if n, err := svc.MutedPeopleCount(ctx, big); err != nil || n != service.MutedPeopleListCap+1 {
			t.Errorf("count = %d, %v; want %d (the count is not capped)", n, err, service.MutedPeopleListCap+1)
		}
	})
}

// TestProfileShowsMuted: Muted is the viewer's own state (D-12, T-09-66).
func TestProfileShowsMuted(t *testing.T) {
	pool, svc := newPeople(t)
	ctx := context.Background()
	now := time.Now().UTC()

	target := testutil.SeedPublicReporter(t, pool, "pm-target@example.com", "muted.target", "pm-target-sess", now)
	muter := testutil.SeedPerson(t, pool, "pm-muter@example.com", "the.muter", false)
	bystander := testutil.SeedPerson(t, pool, "pm-bystander@example.com", "by.stander", false)
	testutil.SeedMute(t, pool, muter, target)

	profile := func(viewer int64, username string) service.PersonProfile {
		t.Helper()
		p, err := svc.Profile(ctx, viewer, username)
		if err != nil {
			t.Fatalf("Profile(%q) as %d: %v", username, viewer, err)
		}
		return p
	}
	if !profile(muter, "muted.target").Muted {
		t.Error("the muter's view of the profile has Muted false, want true")
	}
	if profile(bystander, "muted.target").Muted {
		t.Error("a viewer who muted nobody sees Muted true")
	}
	if profile(target, "muted.target").Muted {
		t.Error("your own page has Muted true")
	}

	// The muted person looks at the muter: they are never told (T-09-66).
	testutil.MustExec(t, pool, "UPDATE accounts SET profile_public = true, profile_public_at = now() WHERE id = $1", muter)
	if profile(target, "the.muter").Muted {
		t.Error("the muted person sees Muted true on the muter's profile; a mute must stay quiet")
	}

	// Unmuting turns it off again.
	if err := svc.Unmute(ctx, muter, "muted.target"); err != nil {
		t.Fatalf("Unmute: %v", err)
	}
	if profile(muter, "muted.target").Muted {
		t.Error("Muted is still true after Unmute")
	}

	// A private profile is still one not found, muted or not.
	testutil.SeedMute(t, pool, muter, bystander)
	if _, err := svc.Profile(ctx, muter, "by.stander"); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("a muted private profile = %v, want ErrPersonNotFound", err)
	}
}

// TestDeleteAccountRemovesPersonMutes (D-12, research G6): deleting an account
// hard deletes the mutes it made and the mutes of it, and leaves every mute
// between other people alone.
func TestDeleteAccountRemovesPersonMutes(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	a := testutil.SeedPerson(t, pool, "mute-del-a@example.com", "going.away", true)
	b := testutil.SeedPerson(t, pool, "mute-del-b@example.com", "staying.one", true)
	c := testutil.SeedPerson(t, pool, "mute-del-c@example.com", "staying.two", true)
	testutil.SeedMute(t, pool, a, b) // a mute by a
	testutil.SeedMute(t, pool, c, a) // a mute of a
	testutil.SeedMute(t, pool, b, c) // between other people

	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("counting: %v", err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM person_mutes WHERE muter_id = $1 OR muted_id = $1`, a); n != 2 {
		t.Fatalf("seed failed: %d mute rows name the account, want 2", n)
	}

	if err := service.NewAccountDeletionService(sqlcgen.New(pool), service.PgxAccountTx(pool)).DeleteAccount(ctx, a); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if n := count(`SELECT count(*) FROM person_mutes WHERE muter_id = $1 OR muted_id = $1`, a); n != 0 {
		t.Errorf("mute rows still naming the deleted account = %d, want 0", n)
	}
	if n := count(`SELECT count(*) FROM person_mutes WHERE muter_id = $1 AND muted_id = $2`, b, c); n != 1 {
		t.Errorf("the mute between two other people = %d, want 1 untouched", n)
	}
}
