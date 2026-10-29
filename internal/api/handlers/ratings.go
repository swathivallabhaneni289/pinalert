// ratings.go holds the two rating routes (Phase 9 plan 14, D-02, UI-SPEC 3a):
// POST /people/{username}/rating saves 1 to 5 stars and
// POST /people/{username}/rating/clear removes the caller's rating. Both take the
// rater from the verified gate account, never from the body. Every reason a
// rating is refused gets the same sentence, so the routes are not an oracle about
// who voted on whose report (T-09-53), and a person the caller may not see gets
// the same uniform 404 as the profile.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// RateRequest is the body of POST /people/{username}/rating. stars is a whole
// number from 1 to 5.
type RateRequest struct {
	Stars *int `json:"stars" example:"4"`
}

// ratingFailure maps the errors the two rating handlers can get. It returns
// after writing the answer. Refusals are 409 with the field stars and one fixed
// sentence each: ErrRatingNotAllowed for every reason a rating is not allowed,
// ErrRatingTooSoon for a change inside the day. Anything else is a people error.
func ratingFailure(w http.ResponseWriter, name string, err error) {
	switch {
	case errors.Is(err, service.ErrRatingNotAllowed):
		w.Header().Set("Cache-Control", "no-store")
		writeFieldError(w, http.StatusConflict, "stars", service.RatingNotAllowedMessage)
		return
	case errors.Is(err, service.ErrRatingTooSoon):
		w.Header().Set("Cache-Control", "no-store")
		writeFieldError(w, http.StatusConflict, "stars", service.RatingTooSoonMessage)
		return
	}
	peopleFailure(w, name, err)
}

// RatePerson produces the handler of POST /people/{username}/rating.
//
// @Summary Rate a person
// @Description Saves the caller's rating of a person, 1 to 5 stars. Only someone who confirmed, disputed,
// @Description resolved or reopened one of that person's reports may rate them, and never yourself. The
// @Description first rating is free; a different rating can be given again after a day. The same stars
// @Description again change nothing and answer 204. Every reason a rating is not allowed answers 409 with
// @Description the field stars and the same sentence, and a change inside the day answers 409 with the
// @Description field stars and its own sentence, so the route does not tell who voted on whose report.
// @Description Stars outside 1 to 5, or a body that is not {"stars": n}, answers 400 with the field stars.
// @Description A person the caller may not see answers the same 404 as the profile.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Param body body RateRequest true "The stars"
// @Success 204 "Saved"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router       /people/{username}/rating [post]
func RatePerson(ratings *service.RatingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if ratings == nil {
			writePersonNotFound(w)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxMeBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req RateRequest
		if err := dec.Decode(&req); err != nil || req.Stars == nil {
			writeFieldError(w, http.StatusBadRequest, "stars", service.RatingStarsMessage)
			return
		}
		if err := ratings.Rate(r.Context(), acc.ID, chi.URLParam(r, "username"), *req.Stars); err != nil {
			ratingFailure(w, "RatePerson", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ClearPersonRating produces the handler of POST /people/{username}/rating/clear.
//
// @Summary Remove your rating of a person
// @Description Removes the caller's rating of a person. Send no body or an empty object. Removing counts
// @Description as a change, so it needs a day since the last change and answers 409 with the field stars
// @Description inside it. A person you have not rated answers 204 and changes nothing. A person the caller
// @Description may not see answers the same 404 as the profile.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags people
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Success 204 "Removed"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router       /people/{username}/rating/clear [post]
func ClearPersonRating(ratings *service.RatingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if ratings == nil {
			writePersonNotFound(w)
			return
		}
		if !decodeEmptyBody(w, r) {
			return
		}
		if err := ratings.Clear(r.Context(), acc.ID, chi.URLParam(r, "username")); err != nil {
			ratingFailure(w, "ClearPersonRating", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
