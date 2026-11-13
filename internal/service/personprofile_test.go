package service_test

// personprofile_test.go proves the profile header read of plan 09-04 over a
// real database (D-03, D-13, D-15, SC5, research Pitfall 9). The tests skip
// without DATABASE_URL (NewTestDB).

import (
	"context"
	"errors"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

func TestProfileUniformNotFound(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	now := time.Now()

	viewer := testutil.SeedPerson(t, pool, "unf-viewer@example.com", "unf.viewer", false)
	testutil.SeedPerson(t, pool, "unf-priv@example.com", "unf.private", false)
	pending := testutil.SeedPublicReporter(t, pool, "unf-pend@example.com", "unf.pending", "unf-pend-sess", now)
	testutil.SeedUsername(t, pool, pending, "unf.pending", "pending")
	deleted := testutil.SeedPublicReporter(t, pool, "unf-del@example.com", "unf.deleted", "unf-del-sess", now)
	if _, err := pool.Exec(ctx, "UPDATE accounts SET deleted_at = now() WHERE id = $1", deleted); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	blocking := testutil.SeedPublicReporter(t, pool, "unf-blk@example.com", "unf.blocking", "unf-blk-sess", now)
	testutil.SeedBlock(t, pool, blocking, viewer)
	testutil.SeedPublicReporter(t, pool, "unf-ok@example.com", "unf.visible", "unf-ok-sess", now)

	for _, name := range []string{"unf.unknown", "unf.private", "unf.pending", "unf.deleted", "unf.blocking", "ab", "has space", ""} {
		got, err := svc.Profile(ctx, viewer, name)
		if err != service.ErrPersonNotFound {
			t.Errorf("Profile(%q) error = %v, want ErrPersonNotFound exactly", name, err)
		}
		if got != (service.PersonProfile{}) {
			t.Errorf("Profile(%q) returned a partial profile: %+v", name, got)
		}
	}
	if _, err := svc.Profile(ctx, viewer, "unf.visible"); err != nil {
		t.Errorf("Profile of a visible person: %v", err)
	}
	// MyUsername has the same answer for an account with no username.
	noName := testutil.SeedReporterAccount(t, pool, "unf-noname@example.com")
	if _, err := svc.MyUsername(ctx, noName); !errors.Is(err, service.ErrPersonNotFound) {
		t.Errorf("MyUsername without a username = %v, want ErrPersonNotFound", err)
	}
	if got, err := svc.MyUsername(ctx, viewer); err != nil || got != "unf.viewer" {
		t.Errorf("MyUsername = %q (%v), want unf.viewer", got, err)
	}
}

func TestProfileBlockedByYouFace(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	now := time.Now()

	viewer := testutil.SeedPerson(t, pool, "face-viewer@example.com", "face.viewer", false)
	person := testutil.SeedPublicReporter(t, pool, "face-person@example.com", "face.person", "face-person-sess", now)
	testutil.SeedBlock(t, pool, viewer, person)

	got, err := svc.Profile(ctx, viewer, "face.person")
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if got != (service.PersonProfile{Username: "face.person", BlockedByYou: true}) {
		t.Errorf("the Blocked by you face = %+v, want only the username and BlockedByYou", got)
	}

	// Once the person goes private the same page is not found.
	if _, err := pool.Exec(ctx, "UPDATE accounts SET profile_public = false WHERE id = $1", person); err != nil {
		t.Fatalf("going private: %v", err)
	}
	if _, err := svc.Profile(ctx, viewer, "face.person"); err != service.ErrPersonNotFound {
		t.Errorf("Profile of a blocked person who went private = %v, want ErrPersonNotFound", err)
	}
}

func TestProfileEvaluateOnTouch(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	now := time.Now()

	owner := testutil.SeedPublicReporter(t, pool, "touch-owner@example.com", "touch.owner", "touch-owner-sess", now)
	follower := testutil.SeedPerson(t, pool, "touch-f@example.com", "touch.follower", false)
	viewer := testutil.SeedPerson(t, pool, "touch-v@example.com", "touch.viewer", false)
	testutil.SeedFollow(t, pool, follower, owner, now)

	got, err := svc.Profile(ctx, viewer, "touch.owner")
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if !got.Reliable || got.FollowersCount != 1 || !got.CanFollow {
		t.Fatalf("while Reliable: %+v, want Reliable, 1 follower and CanFollow", got)
	}

	// 120 days later the judged history has left the 90 day window.
	later := service.NewPeopleService(sqlcgen.New(pool), service.WithPeopleClock(func() time.Time { return now.Add(120 * 24 * time.Hour) }))
	got, err = later.Profile(ctx, follower, "touch.owner")
	if err != nil {
		t.Fatalf("Profile later: %v", err)
	}
	if got.Reliable || got.FollowersCount != 0 || got.FollowersText != "0 Followers" || got.CanFollow || got.Following {
		t.Errorf("after the tag was lost: %+v, want not Reliable, 0 Followers, no Follow and not Following", got)
	}
	if got.StarsNumber != "" {
		t.Errorf("StarsNumber = %q, want none for a reporter with no rated history", got.StarsNumber)
	}
	exists, ended, reason := followRow(t, pool, follower, owner)
	if !exists || !ended || reason != "tag_lost" {
		t.Errorf("the follow row: exists %v ended %v reason %q, want a row ended with tag_lost", exists, ended, reason)
	}
	// The page stays: the profile is still on and opens for others.
	if !got.ProfilePublic {
		t.Error("evaluate on touch switched the profile off")
	}
}

func TestProfileOwnPageStates(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	now := time.Now()

	// On and Reliable.
	rel := testutil.SeedPublicReporter(t, pool, "own-rel@example.com", "own.reliable", "own-rel-sess", now)
	got, err := svc.Profile(ctx, rel, "own.reliable")
	if err != nil {
		t.Fatalf("Profile (on, Reliable): %v", err)
	}
	if !got.IsSelf || !got.ProfilePublic || !got.Reliable || got.CanFollow || got.StateLine != "" { // Quick 261113-t4y: a public profile has no state line.
		t.Errorf("on and Reliable: %+v", got)
	}

	// On, not Reliable.
	plain := testutil.SeedPerson(t, pool, "own-plain@example.com", "own.plain", true)
	got, err = svc.Profile(ctx, plain, "own.plain")
	if err != nil {
		t.Fatalf("Profile (on, not Reliable): %v", err)
	}
	if !got.IsSelf || got.Reliable || got.StateLine != "" {
		t.Errorf("on, not Reliable: %+v", got)
	}

	// Off: your own page still opens, with the full header.
	off := testutil.SeedPerson(t, pool, "own-off@example.com", "own.off", false)
	got, err = svc.Profile(ctx, off, "own.off")
	if err != nil {
		t.Fatalf("Profile (off): %v", err)
	}
	if !got.IsSelf || got.ProfilePublic || got.Username != "own.off" || got.StateLine != "Only you can see this page." { // Amended 2026-11-09 (quick 261109-n25): the short private line.
		t.Errorf("off: %+v", got)
	}
	// Someone else still gets not found for the same page.
	if _, err := svc.Profile(ctx, plain, "own.off"); err != service.ErrPersonNotFound {
		t.Errorf("another person reading an off page = %v, want ErrPersonNotFound", err)
	}

	// A pending username does not close your own page either.
	pend := testutil.SeedPerson(t, pool, "own-pend@example.com", "own.pend", true)
	testutil.SeedUsername(t, pool, pend, "own.pend", "pending")
	if got, err := svc.Profile(ctx, pend, "own.pend"); err != nil || !got.IsSelf {
		t.Errorf("own page with a pending username = %+v (%v), want it to open", got, err)
	}
}

func TestProfileHeaderFields(t *testing.T) {
	ctx := context.Background()
	pool, svc := newPeople(t)
	now := time.Now()

	owner := testutil.SeedPublicReporter(t, pool, "hdr-owner@example.com", "hdr.reporter", "hdr-owner-sess", now)
	viewer := testutil.SeedPerson(t, pool, "hdr-viewer@example.com", "hdr.viewer", false)
	named := testutil.SeedPerson(t, pool, "hdr-named@example.com", "hdr.named", true)
	hidden := testutil.SeedPerson(t, pool, "hdr-hidden@example.com", "hdr.hidden", false)
	target := testutil.SeedPublicReporter(t, pool, "hdr-target@example.com", "hdr.target", "hdr-target-sess", now)
	testutil.SeedFollow(t, pool, named, owner, now)
	testutil.SeedFollow(t, pool, hidden, owner, now)
	testutil.SeedFollow(t, pool, owner, target, now)
	testutil.SeedFollow(t, pool, owner, viewer, now) // the owner follows the viewer

	var created time.Time
	if err := pool.QueryRow(ctx, "SELECT created_at FROM accounts WHERE id = $1", owner).Scan(&created); err != nil {
		t.Fatalf("reading created_at: %v", err)
	}

	got, err := svc.Profile(ctx, viewer, "HDR.Reporter")
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if got.AccountID() != owner {
		t.Errorf("AccountID = %d, want %d", got.AccountID(), owner)
	}
	// The seeded reporter scores 78: 3.9 stars.
	want := service.PersonProfile{
		Username:     "hdr.reporter",
		Name:         "hdr.reporter",
		HomeAreaName: "Benz Circle",
		Initial:      "H",
		StarsNumber:  "3.9",
		StarsLabel:   "Stars: 3.9 out of 5.",
		TagWord:      got.TagWord,
		TagSentence:  got.TagSentence,
		MemberSince:  service.MemberSinceText(created),

		FollowersWord:  "Followers",
		FollowingWord:  "Following",
		FollowersText:  "2 Followers",
		FollowingText:  "2 Following",
		FollowersCount: 2,
		FollowingCount: 2,

		ProfilePublic: true,
		Reliable:      true,
		CanFollow:     true,
		FollowsYou:    true,
	}
	// accountID is unexported; compare field by field through the copy.
	got2 := got
	got2.TagWord, got2.TagSentence = want.TagWord, want.TagSentence
	if got2.Username != want.Username || got2.Name != want.Name || got2.HomeAreaName != want.HomeAreaName ||
		got2.Initial != want.Initial || got2.StarsNumber != want.StarsNumber || got2.StarsLabel != want.StarsLabel ||
		got2.MemberSince != want.MemberSince || got2.FollowersWord != want.FollowersWord || got2.FollowingWord != want.FollowingWord ||
		got2.FollowersText != want.FollowersText || got2.FollowingText != want.FollowingText ||
		got2.FollowersCount != want.FollowersCount || got2.FollowingCount != want.FollowingCount ||
		got2.ProfilePublic != want.ProfilePublic || got2.Reliable != want.Reliable || got2.CanFollow != want.CanFollow ||
		got2.FollowsYou != want.FollowsYou || got2.IsSelf || got2.Following || got2.BlockedByYou || got2.StateLine != "" {
		t.Errorf("header = %+v, want %+v", got, want)
	}
	if got.TagWord != "Reliable reporter" || got.TagSentence == "" {
		t.Errorf("tag %q / %q, want the Reliable reporter word and a counts sentence", got.TagWord, got.TagSentence)
	}

	// Once the viewer follows, Following is set and CanFollow is not.
	if err := svc.Follow(ctx, viewer, "hdr.reporter"); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	got, err = svc.Profile(ctx, viewer, "hdr.reporter")
	if err != nil {
		t.Fatalf("Profile after Follow: %v", err)
	}
	if !got.Following || got.CanFollow || got.FollowersCount != 3 || got.FollowersText != "3 Followers" {
		t.Errorf("after Follow: %+v, want Following, no CanFollow and 3 Followers", got)
	}

	// One follower reads "1 Follower"; the initial is the first letter.
	one := testutil.SeedPublicReporter(t, pool, "hdr-one@example.com", "9lives", "hdr-one-sess", now)
	testutil.SeedFollow(t, pool, viewer, one, now)
	got, err = svc.Profile(ctx, viewer, "9lives")
	if err != nil {
		t.Fatalf("Profile 9lives: %v", err)
	}
	if got.FollowersText != "1 Follower" || got.FollowersWord != "Follower" || got.Initial != "L" {
		t.Errorf("9lives: followers %q word %q initial %q, want \"1 Follower\", \"Follower\" and L", got.FollowersText, got.FollowersWord, got.Initial)
	}
}

// TestProfileStarsParts proves the two parts of the stars and the reader's own
// rating state on the profile (plan 09-14, D-02, D-21, UI-SPEC 1c, review W5):
// below five counted raters only the total shows, from five on the profile also
// shows the part the reports gave and the part the ratings gave (the tapped part
// rounded to half stars), five raters in one place count as one, a New reporter
// has no stars at all, and may_rate and your_rating follow the viewer.
func TestProfileStarsParts(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ratings := service.NewRatingService(q)
	svc := service.NewPeopleService(q, service.WithPeopleSignals(ratings), service.WithPeopleRatings(ratings))
	now := time.Now()

	viewer := testutil.SeedPerson(t, pool, "parts-viewer@example.com", "parts.viewer", false)

	// reporter seeds a public reporter (outcome score 78) with a live report.
	reporter := func(name string) (int64, int64) {
		id := testutil.SeedPublicReporter(t, pool, name+"@example.com", name, name+"-sess", now)
		report := testutil.SeedHistoryReport(t, pool, name+"-sess", "low", "flood", now.Add(time.Hour))
		return id, report
	}
	read := func(name string) service.PersonProfile {
		t.Helper()
		p, err := svc.Profile(ctx, viewer, name)
		if err != nil {
			t.Fatalf("Profile(%q): %v", name, err)
		}
		return p
	}

	t.Run("four raters show only the total", func(t *testing.T) {
		id, report := reporter("parts.four")
		testutil.SeedRaters(t, pool, id, report, []int{5, 5, 5, 5})
		p := read("parts.four")
		if p.StarsNumber != "3.9" || p.StarsLabel != "Stars: 3.9 out of 5." || p.StarsFromReports != "" || p.StarsFromRatings != "" {
			t.Errorf("four raters: %q %q parts %q %q, want the outcome only total and no parts", p.StarsNumber, p.StarsLabel, p.StarsFromReports, p.StarsFromRatings)
		}
	})

	t.Run("twelve raters average four", func(t *testing.T) {
		id, report := reporter("parts.twelve")
		testutil.SeedRaters(t, pool, id, report, []int{4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4})
		p := read("parts.twelve")
		// Outcome 78, tapped 4.0: (78*12 + 20*48 + 12) / 24 is 79 points, 3.9 stars.
		want := "Stars: 3.9 out of 5. 3.9 from reports. 4.0 from ratings by 12 people."
		if p.StarsNumber != "3.9" || p.StarsFromReports != "3.9 from reports" || p.StarsFromRatings != "4.0 from ratings by 12 people" || p.StarsLabel != want {
			t.Errorf("twelve raters: %q parts %q %q label %q, want 3.9, the two parts and %q", p.StarsNumber, p.StarsFromReports, p.StarsFromRatings, p.StarsLabel, want)
		}
	})

	t.Run("the total moves with the ratings", func(t *testing.T) {
		id, report := reporter("parts.high")
		testutil.SeedRaters(t, pool, id, report, []int{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5})
		p := read("parts.high")
		// Outcome 78, tapped 5.0 gives 89 points: 4.4 stars total, 3.9 from reports.
		if p.StarsNumber != "4.4" || p.StarsFromReports != "3.9 from reports" || p.StarsFromRatings != "5.0 from ratings by 12 people" {
			t.Errorf("twelve fives: %q parts %q %q, want 4.4, 3.9 from reports and 5.0 from ratings by 12 people", p.StarsNumber, p.StarsFromReports, p.StarsFromRatings)
		}
	})

	t.Run("the tapped part is rounded to half stars", func(t *testing.T) {
		id, report := reporter("parts.half")
		testutil.SeedRaters(t, pool, id, report, []int{5, 5, 4, 4, 4})
		p := read("parts.half")
		// 22 stars over 5 raters is 4.4, shown as 4.5, never 4.4.
		if p.StarsFromRatings != "4.5 from ratings by 5 people" {
			t.Errorf("five raters, 22 stars: ratings part %q, want %q", p.StarsFromRatings, "4.5 from ratings by 5 people")
		}
	})

	t.Run("five raters in one place count as one", func(t *testing.T) {
		id, report := reporter("parts.cell")
		testutil.SeedRatersInOneCell(t, pool, id, report, []int{5, 5, 5, 5, 5})
		p := read("parts.cell")
		if p.StarsNumber != "3.9" || p.StarsFromReports != "" || p.StarsFromRatings != "" {
			t.Errorf("five raters in one cell: %q parts %q %q, want the outcome only total and no parts", p.StarsNumber, p.StarsFromReports, p.StarsFromRatings)
		}
	})

	t.Run("a New reporter has no stars", func(t *testing.T) {
		testutil.SeedPerson(t, pool, "parts-new@example.com", "parts.new", true)
		p := read("parts.new")
		if p.StarsNumber != "" || p.StarsLabel != "" || p.StarsFromReports != "" || p.StarsFromRatings != "" {
			t.Errorf("a New reporter: %q %q parts %q %q, want no stars at all", p.StarsNumber, p.StarsLabel, p.StarsFromReports, p.StarsFromRatings)
		}
	})

	t.Run("may rate and your rating follow the viewer", func(t *testing.T) {
		id, report := reporter("parts.rate")
		// Before the viewer voted on one of their reports they may not rate.
		if p := read("parts.rate"); p.MayRate || p.YourRating != 0 {
			t.Errorf("a viewer who never voted: may_rate %v your_rating %d, want false and 0", p.MayRate, p.YourRating)
		}
		testutil.MustExec(t, pool,
			`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, 'content', 'confirm', 'tdr1v8y')`,
			report, viewer)
		if p := read("parts.rate"); !p.MayRate || p.YourRating != 0 {
			t.Errorf("a viewer who voted: may_rate %v your_rating %d, want true and 0", p.MayRate, p.YourRating)
		}
		if err := ratings.Rate(ctx, viewer, "parts.rate", 4); err != nil {
			t.Fatalf("Rate: %v", err)
		}
		if p := read("parts.rate"); !p.MayRate || p.YourRating != 4 {
			t.Errorf("after rating 4: may_rate %v your_rating %d, want true and 4", p.MayRate, p.YourRating)
		}

		// Your own page never offers a rating.
		own, err := svc.Profile(ctx, id, "parts.rate")
		if err != nil {
			t.Fatalf("own Profile: %v", err)
		}
		if own.MayRate || own.YourRating != 0 {
			t.Errorf("own page: may_rate %v your_rating %d, want false and 0", own.MayRate, own.YourRating)
		}

		// Without a rating reader both stay false and 0 even for an eligible viewer.
		plain := service.NewPeopleService(q)
		p, err := plain.Profile(ctx, viewer, "parts.rate")
		if err != nil {
			t.Fatalf("Profile without a reader: %v", err)
		}
		if p.MayRate || p.YourRating != 0 {
			t.Errorf("no reader: may_rate %v your_rating %d, want false and 0", p.MayRate, p.YourRating)
		}
	})
}
