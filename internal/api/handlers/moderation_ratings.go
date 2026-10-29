// moderation_ratings.go is the moderator's tool for tapped star ratings (Phase 9
// plan 15, D-14, research G2 and G3): list one reporter's ratings by rater
// username, remove one, or remove everything one rater gave. Like the rest of
// moderation.go these routes are reached only through requireModerator in
// internal/api, which answers a person who is not a moderator with the group's
// byte equal 404 before any handler runs and puts the moderator's account in the
// request context. The RatingService moderator methods check no rights, so every
// handler here starts with modGuard, which refuses a request that did not come
// through the group. There are no swag directives on purpose: this is an
// internal surface and stays out of the public API reference. It is the only
// place a rater's username is sent to anyone.

package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/service"
)

// modPersonNotFoundMessage is the sentence a moderator route gives for a
// username that names nobody.
const modPersonNotFoundMessage = "No such person."

// modRatingJSON is one rating of the moderator list.
type modRatingJSON struct {
	Rater     string    `json:"rater"`
	Stars     int       `json:"stars"`
	UpdatedAt time.Time `json:"updated_at"`
	Removed   bool      `json:"removed"`
	Cleared   bool      `json:"cleared"`
}

// modRatingsJSON is the answer of GET /api/moderation/people/{username}/ratings.
type modRatingsJSON struct {
	Ratings []modRatingJSON `json:"ratings"`
}

// modRatingRemoveRequest is the body of the one rating removal.
type modRatingRemoveRequest struct {
	Rater *string `json:"rater"`
}

// modRatingsRemovedJSON is the answer of the removal by one rater.
type modRatingsRemovedJSON struct {
	Removed int64 `json:"removed"`
}

// modRatingsFailure maps the errors the three handlers can get and returns
// after writing the answer.
func modRatingsFailure(w http.ResponseWriter, name string, err error) {
	if errors.Is(err, service.ErrPersonNotFound) {
		writeFieldError(w, http.StatusNotFound, "username", modPersonNotFoundMessage)
		return
	}
	log.Printf("handlers: %s: %v", name, err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// ModerationRatingsList produces the handler of GET
// /api/moderation/people/{username}/ratings: the ratings one reporter holds,
// newest first, with the rater's username on each.
func ModerationRatingsList(ratings *service.RatingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := modGuard(w, r, http.MethodGet); !ok {
			return
		}
		if ratings == nil {
			writeFieldError(w, http.StatusNotFound, "username", modPersonNotFoundMessage)
			return
		}
		writeModRatings(w, r, ratings, "ModerationRatingsList")
	}
}

// writeModRatings reads one reporter's ratings (the username of the path) and
// writes them, or the failure. The single removal answers with it too, so the
// screen redraws from what is stored and not from a guess.
func writeModRatings(w http.ResponseWriter, r *http.Request, ratings *service.RatingService, name string) {
	rows, err := ratings.ModeratorList(r.Context(), chi.URLParam(r, "username"))
	if err != nil {
		modRatingsFailure(w, name, err)
		return
	}
	out := modRatingsJSON{Ratings: make([]modRatingJSON, 0, len(rows))}
	for _, row := range rows {
		out.Ratings = append(out.Ratings, modRatingJSON{
			Rater:     row.RaterUsername,
			Stars:     row.Stars,
			UpdatedAt: row.UpdatedAt,
			Removed:   row.Removed,
			Cleared:   row.Cleared,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ModerationRatingRemove produces the handler of POST
// /api/moderation/people/{username}/ratings/remove with the body
// {"rater": "name"}: the one rating that rater gave this reporter stops
// counting. Removing a rating that is not there changes nothing and succeeds.
// The answer is the reporter's list as it is now, the same as the list route.
func ModerationRatingRemove(ratings *service.RatingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := modGuard(w, r, http.MethodPost)
		if !ok {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxMeBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req modRatingRemoveRequest
		if err := dec.Decode(&req); err != nil || req.Rater == nil {
			writeFieldError(w, http.StatusBadRequest, "rater", "Send the rater's username.")
			return
		}
		if ratings == nil {
			writeFieldError(w, http.StatusNotFound, "username", modPersonNotFoundMessage)
			return
		}
		if err := ratings.ModeratorRemove(r.Context(), acc.ID, chi.URLParam(r, "username"), *req.Rater); err != nil {
			modRatingsFailure(w, "ModerationRatingRemove", err)
			return
		}
		writeModRatings(w, r, ratings, "ModerationRatingRemove")
	}
}

// ModerationRaterRemoveAll produces the handler of POST
// /api/moderation/raters/{username}/remove-ratings: every rating the rater gave
// stops counting and the rater cannot rate again. The answer says how many.
func ModerationRaterRemoveAll(ratings *service.RatingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := modGuard(w, r, http.MethodPost)
		if !ok {
			return
		}
		if !decodeEmptyBody(w, r) {
			return
		}
		if ratings == nil {
			writeFieldError(w, http.StatusNotFound, "username", modPersonNotFoundMessage)
			return
		}
		n, err := ratings.ModeratorRemoveRater(r.Context(), acc.ID, chi.URLParam(r, "username"))
		if err != nil {
			modRatingsFailure(w, "ModerationRaterRemoveAll", err)
			return
		}
		writeJSON(w, http.StatusOK, modRatingsRemovedJSON{Removed: n})
	}
}
