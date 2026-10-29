// people.go is the JSON surface of followable reporter profiles (Phase 9 plan
// 07): reading a profile, its Followers and Following lists and its recent
// reports, following, unfollowing, removing a follower, blocking, unblocking,
// the Let others follow me switch and the Blocked list. Every handler acts for
// the account the gate resolved and never reads an account id from a request.
// Every rule lives in service.PeopleService and service.ReportService.
//
// One answer for "there is nobody you may see by that name": an unknown, private,
// pending, deleted or blocking username, and a malformed one, all get the same
// 404 body (writePersonNotFound), so the API never tells who opted in (research
// Pitfall 9, T-09-23). Nothing here writes an account id, an email, a session, a
// cell, a voter or a coordinate (T-09-25), and a finished report row carries no
// report id. The routes are gated, and a cross site POST carries no session
// cookie (DEC-L, SameSite=Lax), so no CSRF token is needed.

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// PersonProfileResponse is one profile header, every string written by the
// server (D-13). The account id is never part of it. For a person you blocked
// only username and blocked_by_you are filled.
type PersonProfileResponse struct {
	Username     string `json:"username" example:"asha_k"`
	Name         string `json:"name" example:"Asha"`
	HomeAreaName string `json:"home_area_name" example:"Indiranagar, Bengaluru"`
	Initial      string `json:"initial" example:"A"`
	StarsNumber  string `json:"stars_number" example:"3.6"`
	StarsLabel   string `json:"stars_label" example:"3.6 stars"`
	// StarsFromReports and StarsFromRatings are the two parts of the stars total.
	// Both are empty until five people have rated (a different place each).
	StarsFromReports string `json:"stars_from_reports" example:"3.6 from reports"`
	StarsFromRatings string `json:"stars_from_ratings" example:"4.0 from ratings by 12 people"`
	TagWord          string `json:"tag_word" example:"Reliable reporter"`
	TagSentence      string `json:"tag_sentence" example:"7 of their last 9 reports were confirmed."`
	MemberSince      string `json:"member_since" example:"Member since October 2026"`
	FollowersCount   int    `json:"followers_count" example:"12"`
	FollowingCount   int    `json:"following_count" example:"3"`
	FollowersWord    string `json:"followers_word" example:"Followers"`
	FollowingWord    string `json:"following_word" example:"Following"`
	FollowersText    string `json:"followers_text" example:"12 Followers"`
	FollowingText    string `json:"following_text" example:"3 Following"`
	// StateLine is the line under the buttons on your own page and empty on
	// anyone else's.
	StateLine     string `json:"state_line" example:""`
	IsSelf        bool   `json:"is_self" example:"false"`
	ProfilePublic bool   `json:"profile_public" example:"true"`
	Reliable      bool   `json:"reliable" example:"true"`
	CanFollow     bool   `json:"can_follow" example:"true"`
	Following     bool   `json:"following" example:"false"`
	FollowsYou    bool   `json:"follows_you" example:"false"`
	BlockedByYou  bool   `json:"blocked_by_you" example:"false"`
	// Muted says the caller muted this person (D-12). It is the caller's own
	// state: false on your own page and never revealed to the muted person.
	Muted bool `json:"muted" example:"false"`
	// Notify says the caller has the Notify me bell on for this person. It is true
	// only while the caller follows the person and has not muted them.
	Notify bool `json:"notify" example:"false"`
	// MayRate says the caller may rate this person now. YourRating is the caller's
	// own current rating, 1 to 5, or 0 for none.
	MayRate    bool `json:"may_rate" example:"true"`
	YourRating int  `json:"your_rating" example:"4"`
}

// PersonEntryResponse is one named person of a list. It holds the username and
// nothing else.
type PersonEntryResponse struct {
	Username string `json:"username" example:"ravi_m"`
}

// PeopleListResponse is one page of a Followers or Following list. People who
// are not named (private, pending, deleted or blocked) are counted in
// others_count and others_text. next is the opaque cursor for the next page and
// an empty string on the last page.
type PeopleListResponse struct {
	Entries     []PersonEntryResponse `json:"entries"`
	OthersCount int                   `json:"others_count" example:"2"`
	OthersText  string                `json:"others_text" example:"and 2 others"`
	Next        string                `json:"next" example:""`
}

// BlockedPeopleResponse is the caller's own blocked list.
type BlockedPeopleResponse struct {
	Entries []PersonEntryResponse `json:"entries"`
}

// ProfileReportRowResponse is one report of a profile. A live row carries its
// id, description and posting time. A finished row carries no id, no
// description and no time, only the date it ended (never a time), and its place
// is blurred to about 5 km on the server. No row carries a coordinate.
type ProfileReportRowResponse struct {
	ID           *int64 `json:"id,omitempty" example:"42"`
	Live         bool   `json:"live" example:"true"`
	Status       string `json:"status" enums:"ongoing,not_confirmed,disputed,resolved,expired" example:"ongoing"`
	StatusWord   string `json:"status_word" example:"Ongoing"`
	Category     string `json:"category" example:"flood"`
	Severity     string `json:"severity" enums:"low,medium,critical" example:"critical"`
	CategoryWord string `json:"category_word" example:"Flood"`
	SeverityWord string `json:"severity_word" example:"Critical"`
	// Description is present on a live row only.
	Description   string `json:"description,omitempty" example:"Water rising fast near the market bridge."`
	FinishedTitle string `json:"finished_title,omitempty" example:"Flood, Critical"`
	AreaLine      string `json:"area_line,omitempty" example:"Near Benz Circle"`
	DistanceText  string `json:"distance_text,omitempty" example:"About 2 km"`
	DisputedLine  string `json:"disputed_line,omitempty" example:"Disputed: may not be true"`
	// PostedAt is present on a live row only.
	PostedAt string `json:"posted_at,omitempty" example:"2026-09-05T14:03:00.000Z"`
	// EndedOn is present on a finished row only: the date, never a time.
	EndedOn string `json:"ended_on,omitempty" example:"12 October 2026"`
}

// ProfileReportsResponse is a person's reports from the last 30 days, newest
// first, at most 100. cut is true when more existed.
type ProfileReportsResponse struct {
	Rows    []ProfileReportRowResponse `json:"rows"`
	Cut     bool                       `json:"cut" example:"false"`
	CutText string                     `json:"cut_text" example:""`
}

// VisibilityRequest is the body of PATCH /me/visibility.
type VisibilityRequest struct {
	ProfilePublic *bool `json:"profile_public" example:"true"`
}

// VisibilityResponse is the saved state of the Let others follow me switch.
type VisibilityResponse struct {
	ProfilePublic bool `json:"profile_public" example:"true"`
}

// NotifyRequest is the body of POST /people/{username}/notify. On is required.
type NotifyRequest struct {
	On *bool `json:"on" example:"true"`
}

// NotifyResponse is the saved state of the Notify me bell for one person.
type NotifyResponse struct {
	Notify bool `json:"notify" example:"true"`
}

// writePersonNotFound writes the one 404 of every people route that reads a
// person. Its body is the same for every cause, byte for byte.
func writePersonNotFound(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	writeFieldError(w, http.StatusNotFound, "username", service.ProfileNotFoundMessage)
}

// peopleFailure maps the errors every people handler can get. It returns after
// writing the answer.
func peopleFailure(w http.ResponseWriter, name string, err error) {
	switch {
	case errors.Is(err, service.ErrPersonNotFound):
		writePersonNotFound(w)
		return
	case errors.Is(err, service.ErrCannotFollow):
		writeFieldError(w, http.StatusConflict, "follow", service.CannotFollowMessage)
		return
	case errors.Is(err, service.ErrFollowingCap):
		writeFieldError(w, http.StatusConflict, "follow", service.FollowingCapMessage)
		return
	case errors.Is(err, service.ErrNotifyUnavailable):
		writeFieldError(w, http.StatusConflict, "notify", service.NotifyUnavailableMessage)
		return
	case errors.Is(err, service.ErrNotifyCap):
		writeFieldError(w, http.StatusConflict, "notify", service.NotifyCapMessage)
		return
	}
	var ve service.ValidationError
	if errors.As(err, &ve) {
		writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
		return
	}
	log.Printf("handlers: %s: %v", name, err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// profileResponse copies a service profile onto the wire shape.
func profileResponse(p service.PersonProfile) PersonProfileResponse {
	return PersonProfileResponse{
		Username:         p.Username,
		Name:             p.Name,
		HomeAreaName:     p.HomeAreaName,
		Initial:          p.Initial,
		StarsNumber:      p.StarsNumber,
		StarsLabel:       p.StarsLabel,
		StarsFromReports: p.StarsFromReports,
		StarsFromRatings: p.StarsFromRatings,
		TagWord:          p.TagWord,
		TagSentence:      p.TagSentence,
		MemberSince:      p.MemberSince,
		FollowersCount:   p.FollowersCount,
		FollowingCount:   p.FollowingCount,
		FollowersWord:    p.FollowersWord,
		FollowingWord:    p.FollowingWord,
		FollowersText:    p.FollowersText,
		FollowingText:    p.FollowingText,
		StateLine:        p.StateLine,
		IsSelf:           p.IsSelf,
		ProfilePublic:    p.ProfilePublic,
		Reliable:         p.Reliable,
		CanFollow:        p.CanFollow,
		Following:        p.Following,
		FollowsYou:       p.FollowsYou,
		BlockedByYou:     p.BlockedByYou,
		Muted:            p.Muted,
		Notify:           p.Notify,
		MayRate:          p.MayRate,
		YourRating:       p.YourRating,
	}
}

// PersonProfile produces the handler of GET /people/{username}.
//
// @Summary Read a person's profile
// @Description Returns the profile header of the person with this username: name, home area, stars,
// @Description the reporter tag, the member since line and the two counts, every string written by the
// @Description server. A person who has not turned on a public profile, one whose username is still
// @Description being checked, one who has deleted the account and one who blocked the caller all get
// @Description the same 404 as a username that does not exist, so nobody can tell who opted in. A person
// @Description the caller blocked returns only the username with blocked_by_you true. Your own profile
// @Description always opens. The counts include people who are shown only as a number.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Produce json
// @Param username path string true "Username"
// @Success 200 {object} PersonProfileResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username} [get]
func PersonProfile(people *service.PeopleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		p, err := people.Profile(r.Context(), acc.ID, chi.URLParam(r, "username"))
		if err != nil {
			peopleFailure(w, "PersonProfile", err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, profileResponse(p))
	}
}

// listResponse copies a service list onto the wire shape.
func listResponse(l service.PeopleList) PeopleListResponse {
	out := PeopleListResponse{
		Entries:     make([]PersonEntryResponse, 0, len(l.Entries)),
		OthersCount: l.OthersCount,
		OthersText:  l.OthersText,
		Next:        l.Next,
	}
	for _, e := range l.Entries {
		out.Entries = append(out.Entries, PersonEntryResponse{Username: e.Username})
	}
	return out
}

// PersonFollowers produces the handler of GET /people/{username}/followers.
//
// @Summary List the people who follow a person
// @Description Returns the named people who follow this person, newest first, 50 per page. People who
// @Description are not named (private, pending, deleted or blocked) are counted in others_count with
// @Description the sentence others_text. Pass the next value as after to read the next page. The same
// @Description 404 as the profile is returned for a person the caller may not see.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Produce json
// @Param username path string true "Username"
// @Param after query string false "Cursor from the previous page"
// @Success 200 {object} PeopleListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/followers [get]
func PersonFollowers(people *service.PeopleService) http.HandlerFunc {
	return peopleList("PersonFollowers", people.Followers)
}

// PersonFollowing produces the handler of GET /people/{username}/following.
//
// @Summary List the people a person follows
// @Description Returns the named people this person follows, newest first, 50 per page, with the same
// @Description rules as the Followers list.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Produce json
// @Param username path string true "Username"
// @Param after query string false "Cursor from the previous page"
// @Success 200 {object} PeopleListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/following [get]
func PersonFollowing(people *service.PeopleService) http.HandlerFunc {
	return peopleList("PersonFollowing", people.Following)
}

// peopleList is the one body of the two list handlers.
func peopleList(name string, read func(ctx context.Context, viewerID int64, username, after string) (service.PeopleList, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		l, err := read(r.Context(), acc.ID, chi.URLParam(r, "username"), r.URL.Query().Get("after"))
		if err != nil {
			peopleFailure(w, name, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, listResponse(l))
	}
}

// PersonReports produces the handler of GET /people/{username}/reports.
//
// @Summary List a person's recent reports
// @Description Returns the reports this person posted in the last 30 days, newest first, at most 100
// @Description (cut is true when more existed). A live report carries its id, words, description and
// @Description posting time. A finished report is blurred to about 5 km on the server and carries no
// @Description id, no description and no time, only the date it ended. No row carries a coordinate.
// @Description With lat and lon (both or neither) a live row also says how far away it is from that
// @Description point. The same 404 as the profile is returned for a person the caller may not see.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Produce json
// @Param username path string true "Username"
// @Param lat query number false "Your latitude, with lon"
// @Param lon query number false "Your longitude, with lat"
// @Success 200 {object} ProfileReportsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/reports [get]
func PersonReports(people *service.PeopleService, reports *service.ReportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")

		// The viewer's own centre: both numbers or neither.
		q := r.URL.Query()
		var centre *service.GeoPoint
		hasLat, hasLon := q.Get("lat") != "", q.Get("lon") != ""
		switch {
		case hasLat != hasLon:
			writeFieldError(w, http.StatusBadRequest, "lat", "Send lat and lon together.")
			return
		case hasLat:
			lat, ok := parseCoordinate(w, q, "lat", -90, 90)
			if !ok {
				return
			}
			lon, ok := parseCoordinate(w, q, "lon", -180, 180)
			if !ok {
				return
			}
			centre = &service.GeoPoint{Lat: lat, Lon: lon}
		}

		// Profile first: it is the one place that decides whether this viewer may
		// see this person, and it answers the same 404 for every refusal. A
		// person the viewer blocked has no reports to show either.
		p, err := people.Profile(r.Context(), acc.ID, chi.URLParam(r, "username"))
		if err != nil {
			peopleFailure(w, "PersonReports", err)
			return
		}
		if p.BlockedByYou {
			writePersonNotFound(w)
			return
		}
		if reports == nil {
			log.Printf("handlers: PersonReports: no report service")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		page, err := reports.ProfileReports(r.Context(), service.ProfileQuery{
			AccountID:       p.AccountID(),
			ViewerAccountID: acc.ID,
			Centre:          centre,
		})
		if err != nil {
			peopleFailure(w, "PersonReports", err)
			return
		}

		out := ProfileReportsResponse{
			Rows:    make([]ProfileReportRowResponse, 0, len(page.Rows)),
			Cut:     page.Cut,
			CutText: page.CutText,
		}
		for _, row := range page.Rows {
			out.Rows = append(out.Rows, profileReportRowResponse(row))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// profileReportRowResponse maps one service row. A finished row never gets an
// id, a description or a time, whatever the service row holds.
func profileReportRowResponse(row service.ProfileReportRow) ProfileReportRowResponse {
	v := row.View
	out := ProfileReportRowResponse{
		Live:          row.Live,
		Status:        row.Status,
		StatusWord:    row.StatusWord,
		Category:      string(v.Category),
		Severity:      string(v.Severity),
		CategoryWord:  service.CategoryWord(v.Category),
		SeverityWord:  service.SeverityWord(v.Severity),
		FinishedTitle: row.FinishedTitle,
		AreaLine:      row.AreaLine,
		DistanceText:  row.DistanceText,
		DisputedLine:  row.DisputedLine,
	}
	if row.Live {
		id := v.ID
		out.ID = &id
		out.Description = v.Description
		out.PostedAt = v.CreatedAt.UTC().Format(rfc3339Milli)
	} else {
		out.EndedOn = row.EndedOn
	}
	return out
}

// personAction is the one body of the body free people POSTs: the account from
// the gate, the username from the path, an empty body, one service call and a
// 204.
func personAction(name string, act func(ctx context.Context, accountID int64, username string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !decodeEmptyBody(w, r) {
			return
		}
		if err := act(r.Context(), acc.ID, chi.URLParam(r, "username")); err != nil {
			peopleFailure(w, name, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// FollowPerson produces the handler of POST /people/{username}/follow.
//
// @Summary Follow a person
// @Description Starts following a person who turned on a public profile and holds the Reliable reporter
// @Description tag. Send no body or an empty object. Following someone already followed changes nothing
// @Description and answers 204. A person you cannot follow right now (yourself, someone you blocked, or
// @Description someone who is not a Reliable reporter) answers 409 with the field follow; so does the
// @Description limit of 200 people. A person you may not see answers the same 404 as the profile.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 204 "Following"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/follow [post]
func FollowPerson(people *service.PeopleService) http.HandlerFunc {
	return personAction("FollowPerson", people.Follow)
}

// UnfollowPerson produces the handler of POST /people/{username}/unfollow.
//
// @Summary Stop following a person
// @Description Stops following a person. Send no body or an empty object. A well formed username that
// @Description matches nobody, or someone you do not follow, answers 204 and changes nothing, so this
// @Description route reveals nothing. A malformed username answers the same 404 as the profile.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 204 "Not following"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/unfollow [post]
func UnfollowPerson(people *service.PeopleService) http.HandlerFunc {
	return personAction("UnfollowPerson", people.Unfollow)
}

// RemoveFollower produces the handler of POST /people/{username}/remove-follower.
//
// @Summary Remove one follower
// @Description Ends the follow of the person with this username on the signed in account. Send no body
// @Description or an empty object. A well formed username that matches nobody answers 204 and changes
// @Description nothing. A malformed username answers the same 404 as the profile.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param username path string true "Username of the follower"
// @Success 204 "Removed"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/remove-follower [post]
func RemoveFollower(people *service.PeopleService) http.HandlerFunc {
	return personAction("RemoveFollower", people.RemoveFollower)
}

// BlockPerson produces the handler of POST /people/{username}/block.
//
// @Summary Block a person
// @Description Blocks a person. Every follow between the two of you ends and neither can follow the
// @Description other. The blocked person gets the same 404 as an unknown username on your profile. Send
// @Description no body or an empty object. A person you may not see answers the same 404 as the profile.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 204 "Blocked"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/block [post]
func BlockPerson(people *service.PeopleService) http.HandlerFunc {
	return personAction("BlockPerson", people.Block)
}

// UnblockPerson produces the handler of POST /people/{username}/unblock.
//
// @Summary Unblock a person
// @Description Undoes a block. Send no body or an empty object. A well formed username that matches
// @Description nobody, or someone you did not block, answers 204 and changes nothing. A malformed
// @Description username answers the same 404 as the profile. Follows that ended with the block do not
// @Description come back.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 204 "Unblocked"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/unblock [post]
func UnblockPerson(people *service.PeopleService) http.HandlerFunc {
	return personAction("UnblockPerson", people.Unblock)
}

// RemoveAllFollowers produces the handler of POST /me/followers/remove-all.
//
// @Summary Remove every follower
// @Description Ends every follow of the signed in account at once. Send no body or an empty object. It
// @Description answers 204 even when nobody follows the account.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Success 204 "All followers removed"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /me/followers/remove-all [post]
func RemoveAllFollowers(people *service.PeopleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !decodeEmptyBody(w, r) {
			return
		}
		if _, err := people.RemoveAllFollowers(r.Context(), acc.ID); err != nil {
			peopleFailure(w, "RemoveAllFollowers", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// BlockedPeopleList produces the handler of GET /me/blocked.
//
// @Summary List the people the caller blocked
// @Description Returns the usernames of the people the signed in account blocked, newest first, at most
// @Description 500. A person whose new username is still being checked is left out until it clears.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Produce json
// @Success 200 {object} BlockedPeopleResponse
// @Failure 401 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /me/blocked [get]
func BlockedPeopleList(people *service.PeopleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		entries, err := people.BlockedPeople(r.Context(), acc.ID)
		if err != nil {
			peopleFailure(w, "BlockedPeopleList", err)
			return
		}
		out := BlockedPeopleResponse{Entries: make([]PersonEntryResponse, 0, len(entries))}
		for _, e := range entries {
			out.Entries = append(out.Entries, PersonEntryResponse{Username: e.Username})
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, out)
	}
}

// NotifyPerson produces the handler of POST /people/{username}/notify.
//
// @Summary Turn Notify me on or off for a person
// @Description Switches Notify me for one person the caller follows. It is off by default and exists only
// @Description while the caller follows the person and has not muted them. Turning it on records the time,
// @Description so only reports posted later raise a note; turning it off clears it. Send a body with on
// @Description set to true or false; a repeat changes nothing. The answer is the saved state. A person the
// @Description caller does not follow, has muted or has blocked, and the caller's own username, answer 409
// @Description with the field notify and the sentence "Couldn't change this. Try again."; so does turning on
// @Description a bell when 50 are already on, with the sentence for that limit. A person you may not see
// @Description answers the same 404 as the profile.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Param body body NotifyRequest true "The switch"
// @Success 200 {object} NotifyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/notify [post]
func NotifyPerson(people *service.PeopleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxMeBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req NotifyRequest
		if err := decodeOneValue(dec, &req); err != nil || req.On == nil {
			writeFieldError(w, http.StatusBadRequest, "on", "Send on as true or false.")
			return
		}
		if err := people.SetNotify(r.Context(), acc.ID, chi.URLParam(r, "username"), *req.On); err != nil {
			peopleFailure(w, "NotifyPerson", err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, NotifyResponse{Notify: *req.On})
	}
}

// UpdateVisibility produces the handler of PATCH /me/visibility.
//
// @Summary Turn Let others follow me on or off
// @Description Saves whether the signed in account has a public profile that others can follow. Turning
// @Description it on needs a clear username and a name that can be shown in public; a username or name
// @Description that is refused, or that could not be checked just now, answers 422 with the field
// @Description profile_public and the reason, and the switch stays off. Turning it off ends every
// @Description follower, and turning it on later starts with none. The answer is the saved state.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param body body VisibilityRequest true "The switch"
// @Success 200 {object} VisibilityResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /me/visibility [patch]
func UpdateVisibility(people *service.PeopleService, gate *RateGate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		// Turning the switch on may call the text screener twice, which can take
		// longer than the server's default write window.
		extendWriteDeadline(w)

		r.Body = http.MaxBytesReader(w, r.Body, maxMeBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req VisibilityRequest
		if err := decodeOneValue(dec, &req); err != nil || req.ProfilePublic == nil {
			writeFieldError(w, http.StatusBadRequest, "profile_public", "Send profile_public as true or false.")
			return
		}

		// Turning it on may call the shared text screener, so that change has a
		// tight bucket of its own; turning it off never meets it (WR-E02).
		if *req.ProfilePublic && !gate.AdmitVisibilityOn(w, acc.ID) {
			return
		}
		if err := people.SetProfilePublic(r.Context(), acc.ID, *req.ProfilePublic); err != nil {
			// A refused or unchecked name is the only 422.
			var ve service.ValidationError
			if errors.As(err, &ve) {
				writeFieldError(w, http.StatusUnprocessableEntity, ve.Field, ve.Message)
				return
			}
			peopleFailure(w, "UpdateVisibility", err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, VisibilityResponse{ProfilePublic: *req.ProfilePublic})
	}
}

// decodeOneValue decodes exactly one JSON value from dec into dst: a body with
// anything after the value, other than space, is refused, the way decodePushBody
// refuses it (review IN-E04, 2026-10-30). The Phase 9 body routes use it.
func decodeOneValue(dec *json.Decoder, dst any) error {
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, next := dec.Token(); !errors.Is(next, io.EOF) {
		return errors.New("trailing data")
	}
	return nil
}
