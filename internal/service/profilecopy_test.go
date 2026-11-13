package service_test

// profilecopy_test.go proves every sentence helper of plan 09-04 over 0, 1 and
// many, and that none of them holds a dash.

import (
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
)

func TestProfileCopy(t *testing.T) {
	eq := func(t *testing.T, name, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	t.Run("counts", func(t *testing.T) {
		eq(t, "FollowersCountText(0)", service.FollowersCountText(0), "0 Followers")
		eq(t, "FollowersCountText(1)", service.FollowersCountText(1), "1 Follower")
		eq(t, "FollowersCountText(12)", service.FollowersCountText(12), "12 Followers")
		eq(t, "FollowersWord(1)", service.FollowersWord(1), "Follower")
		eq(t, "FollowersWord(0)", service.FollowersWord(0), "Followers")
		eq(t, "FollowersWord(2)", service.FollowersWord(2), "Followers")
		eq(t, "FollowingCountText(0)", service.FollowingCountText(0), "0 Following")
		eq(t, "FollowingCountText(1)", service.FollowingCountText(1), "1 Following")
		eq(t, "FollowingCountText(30)", service.FollowingCountText(30), "30 Following")
	})

	t.Run("member_since", func(t *testing.T) {
		eq(t, "MemberSinceText", service.MemberSinceText(time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)), "Member since October 2026")
		eq(t, "MemberSinceText january", service.MemberSinceText(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)), "Member since January 2027")
		// 23:30 UTC on the last day of a month is still that month in UTC.
		eq(t, "MemberSinceText zone", service.MemberSinceText(time.Date(2026, 10, 31, 23, 30, 0, 0, time.UTC)), "Member since October 2026")
	})

	t.Run("others", func(t *testing.T) {
		eq(t, "OthersText(0,true)", service.OthersText(0, true), "")
		eq(t, "OthersText(0,false)", service.OthersText(0, false), "")
		eq(t, "OthersText(1,true)", service.OthersText(1, true), "and 1 other")
		eq(t, "OthersText(3,true)", service.OthersText(3, true), "and 3 others")
		eq(t, "OthersText(1,false)", service.OthersText(1, false), "1 other")
		eq(t, "OthersText(3,false)", service.OthersText(3, false), "3 others")
	})

	t.Run("stars", func(t *testing.T) {
		eq(t, "StarsTotalText(72)", service.StarsTotalText(72), "3.6 stars")
		eq(t, "StarsTotalText(69)", service.StarsTotalText(69), "3.4 stars")
		eq(t, "StarsTotalText(0)", service.StarsTotalText(0), "0.0 stars")
		eq(t, "StarsTotalText(100)", service.StarsTotalText(100), "5.0 stars")
		eq(t, "StarsGroupLabel(72)", service.StarsGroupLabel(72), "Stars: 3.6 out of 5.")
	})

	// Plan 09-14 (D-02, UI-SPEC 1c, review W5): the two parts of the stars and the
	// whole accessible name when both show.
	t.Run("stars_parts", func(t *testing.T) {
		eq(t, "StarsFromReportsText(72)", service.StarsFromReportsText(72), "3.6 from reports")
		eq(t, "StarsFromReportsText(69)", service.StarsFromReportsText(69), "3.4 from reports")
		eq(t, "StarsFromRatingsText(8, 12)", service.StarsFromRatingsText(8, 12), "4.0 from ratings by 12 people")
		// 22 stars from 5 raters is 4.4, rounded to half stars: 9 halves, shown 4.5.
		eq(t, "StarsFromRatingsText(TappedHalfStars(5, 22), 5)", service.StarsFromRatingsText(service.TappedHalfStars(5, 22), 5), "4.5 from ratings by 5 people")
		// 21 stars from 5 raters is 4.2, rounded to 4.0.
		eq(t, "StarsFromRatingsText(TappedHalfStars(5, 21), 5)", service.StarsFromRatingsText(service.TappedHalfStars(5, 21), 5), "4.0 from ratings by 5 people")
		eq(t, "StarsGroupLabelParts both", service.StarsGroupLabelParts(72, "3.6 from reports", "4.0 from ratings by 12 people"),
			"Stars: 3.6 out of 5. 3.6 from reports. 4.0 from ratings by 12 people.")
		eq(t, "StarsGroupLabelParts no parts", service.StarsGroupLabelParts(72, "", ""), "Stars: 3.6 out of 5.")
		eq(t, "StarsGroupLabelParts one part", service.StarsGroupLabelParts(72, "3.6 from reports", ""), "Stars: 3.6 out of 5.")
	})

	t.Run("own_state_line", func(t *testing.T) {
		// Quick 261113-t4y (owner decision 2026-11-13, the compact profile): a public
		// profile has no state line, so both follow sentences are gone.
		eq(t, "on and reliable", service.OwnStateLine(true, true), "")
		eq(t, "on not reliable", service.OwnStateLine(true, false), "")
		// Amended 2026-11-09 (quick 261109-n25, fewer words): the private line no
		// longer adds "Turn on Let others follow me in Settings."
		eq(t, "off", service.OwnStateLine(false, true), "Only you can see this page.")
		eq(t, "off not reliable", service.OwnStateLine(false, false), "Only you can see this page.")
	})

	t.Run("sentences", func(t *testing.T) {
		eq(t, "CannotFollowMessage", service.CannotFollowMessage, "You can't follow this person right now.")
		eq(t, "ProfileNotFoundMessage", service.ProfileNotFoundMessage, "Profile not found.")
		eq(t, "FollowingCapMessage", service.FollowingCapMessage, "You can follow up to 200 people.")
	})

	t.Run("no_dash_and_no_sound_words", func(t *testing.T) {
		dashes := string([]rune{0x2013, 0x2014})
		all := []string{
			service.CannotFollowMessage, service.ProfileNotFoundMessage, service.FollowingCapMessage,
			service.FollowersCountText(12), service.FollowingCountText(30),
			service.MemberSinceText(time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)),
			service.OthersText(3, true), service.StarsTotalText(72), service.StarsGroupLabel(72),
			service.StarsFromReportsText(72), service.StarsFromRatingsText(8, 12),
			service.StarsGroupLabelParts(72, "3.6 from reports", "4.0 from ratings by 12 people"),
			service.OwnStateLine(true, true), service.OwnStateLine(true, false), service.OwnStateLine(false, false),
		}
		for _, s := range all {
			if strings.ContainsAny(s, dashes) {
				t.Errorf("%q holds a dash", s)
			}
			low := strings.ToLower(s)
			for _, w := range []string{"sound", "buzz", "vibrat"} {
				if strings.Contains(low, w) {
					t.Errorf("%q mentions %q", s, w)
				}
			}
		}
	})
}
