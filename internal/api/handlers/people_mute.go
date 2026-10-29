package handlers

// The person mute routes (Phase 9 plan 19, D-12). A mute is one way and quiet:
// the muted person is never told and nothing that lists reports hides a report
// because of it. Both actions answer 204; the list answers 200 with entries.

import (
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// MutedPeopleResponse is the caller's own muted people list.
type MutedPeopleResponse struct {
	Entries []PersonEntryResponse `json:"entries"`
}

// MutePerson produces the handler of POST /people/{username}/mute.
//
// @Summary Mute a person
// @Description Stops person level notes about a person for the caller only, such as the Reliable
// @Description reporter note. Nothing else changes: no follow ends, no report is hidden from any feed,
// @Description map, list or profile, and the muted person is never told. Send no body or an empty
// @Description object. Muting someone already muted changes nothing and answers 204. A person you may
// @Description not see, and yourself, answer the same 404 as the profile.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 204 "Muted"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/mute [post]
func MutePerson(people *service.PeopleService) http.HandlerFunc {
	return personAction("MutePerson", people.Mute)
}

// UnmutePerson produces the handler of POST /people/{username}/unmute.
//
// @Summary Unmute a person
// @Description Undoes a mute. Send no body or an empty object. It works for any username in the
// @Description caller's own list, also one whose profile went private since. A well formed username that
// @Description matches nobody, or someone the caller did not mute, answers 204 and changes nothing. A
// @Description malformed username answers the same 404 as the profile.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 204 "Unmuted"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /people/{username}/unmute [post]
func UnmutePerson(people *service.PeopleService) http.HandlerFunc {
	return personAction("UnmutePerson", people.Unmute)
}

// MutedPeopleList produces the handler of GET /me/muted-people.
//
// @Summary List the people the caller muted
// @Description Returns the usernames of the people the signed in account muted, newest first, at most
// @Description 500. A person whose new username is still being checked is left out until it clears.
// @Description Only the caller reads this list.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Produce json
// @Success 200 {object} MutedPeopleResponse
// @Failure 401 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /me/muted-people [get]
func MutedPeopleList(people *service.PeopleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		entries, err := people.MutedPeople(r.Context(), acc.ID)
		if err != nil {
			peopleFailure(w, "MutedPeopleList", err)
			return
		}
		out := MutedPeopleResponse{Entries: make([]PersonEntryResponse, 0, len(entries))}
		for _, e := range entries {
			out.Entries = append(out.Entries, PersonEntryResponse{Username: e.Username})
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, out)
	}
}
