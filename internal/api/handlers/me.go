// me.go is the HTTP surface of the caller's own account rows (Phase 3.4, D-24b
// and D-27): agreeing to the terms, saving the optional name and home area, and
// marking the first-time flow done. Each handler acts on the account the gate
// resolved and on nothing a request body can name. Every rule lives in
// service.AccountService. The routes are gated, and a cross site POST carries
// no session cookie (DEC-L, SameSite=Lax), so no CSRF token is needed.

package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/service"
	"pinalert/internal/session"
)

// maxMeBodyBytes caps the bodies of the /api/me routes. A profile body is a
// name and one picked place, so a few kilobytes is far more than it needs.
const maxMeBodyBytes = 4096

// UpdateProfileRequest is the body of PATCH /api/me/profile. Each key is
// optional: a key left out changes nothing. An empty display_name clears the
// name. home_area is either an object that saves the picked place or null,
// which removes the saved one.
type UpdateProfileRequest struct {
	DisplayName *string          `json:"display_name" example:"Asha"`
	HomeArea    *HomeAreaRequest `json:"home_area"`
}

// HomeAreaRequest is the picked place. The server keeps only the name and a
// cell about 5 km across, computed from the point; the point is never stored.
type HomeAreaRequest struct {
	Name string  `json:"name" example:"Indiranagar, Bengaluru"`
	Lat  float64 `json:"lat" example:"12.9784"`
	Lon  float64 `json:"lon" example:"77.6408"`
}

// ProfileResponse is the caller's own profile as stored. Both keys are always
// present and are null when nothing is saved. No coordinates are ever part of it.
type ProfileResponse struct {
	DisplayName  *string `json:"display_name" example:"Asha"`
	HomeAreaName *string `json:"home_area_name" example:"Indiranagar, Bengaluru"`
}

// emptyBody is the shape of a body that carries no fields.
type emptyBody struct{}

// decodeEmptyBody accepts no body at all or an empty JSON object, and reports
// whether the request may go on. Anything else gets a 400 here.
func decodeEmptyBody(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxMeBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var body emptyBody
	if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeFieldError(w, http.StatusBadRequest, "body", "Request body is malformed.")
		return false
	}
	return true
}

// AcceptTerms produces the handler of POST /me/consent.
//
// @Summary Agree to the current terms
// @Description Records that the signed in account agreed to the current version of the Disclaimer
// @Description and Policy text. Send no body or an empty object. A repeat call for the same version
// @Description keeps the first agreed time. A different version is stored with a new time.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Accept json
// @Produce json
// @Success 204 "Agreed"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /me/consent [post]
func AcceptTerms(svc *service.AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			// A route mounted outside the gated group, a programming error.
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !decodeEmptyBody(w, r) {
			return
		}
		if err := svc.AcceptTerms(r.Context(), acc.ID); err != nil {
			log.Printf("handlers: AcceptTerms: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// UpdateProfile produces the handler of PATCH /me/profile.
//
// @Summary Save or clear the optional name and home area
// @Description Updates the optional name and home area of the signed in account. A key left out
// @Description changes nothing. An empty display_name clears the name. A home_area object saves the
// @Description picked place, and home_area null removes it. The home area keeps only the place name
// @Description and a cell about 5 km across; the coordinates you send are used once to find the cell
// @Description and are never stored. The answer is the profile as stored, with no coordinates.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Accept json
// @Produce json
// @Param body body UpdateProfileRequest true "The rows to change"
// @Success 200 {object} ProfileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /me/profile [patch]
func UpdateProfile(svc *service.AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxMeBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		// home_area is read raw so a missing key, null and an object can be
		// told apart: a plain pointer would read the first two the same way.
		var req struct {
			DisplayName *string         `json:"display_name"`
			HomeArea    json.RawMessage `json:"home_area"`
		}
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		update := service.ProfileUpdate{DisplayName: req.DisplayName}
		switch {
		case req.HomeArea == nil:
			// Key absent: unchanged.
		case bytes.Equal(bytes.TrimSpace(req.HomeArea), []byte("null")):
			update.HomeArea.Action = service.HomeAreaClear
		default:
			var in struct {
				Name *string  `json:"name"`
				Lat  *float64 `json:"lat"`
				Lon  *float64 `json:"lon"`
			}
			inner := json.NewDecoder(bytes.NewReader(req.HomeArea))
			inner.DisallowUnknownFields()
			if err := inner.Decode(&in); err != nil || in.Name == nil || in.Lat == nil || in.Lon == nil {
				writeFieldError(w, http.StatusBadRequest, "home_area", "Send the place name with its latitude and longitude.")
				return
			}
			update.HomeArea = service.HomeAreaChange{
				Action: service.HomeAreaSet,
				Input:  service.HomeAreaInput{Name: *in.Name, Lat: *in.Lat, Lon: *in.Lon},
			}
		}

		profile, err := svc.UpdateProfile(r.Context(), acc.ID, update)
		if err != nil {
			var ve service.ValidationError
			if errors.As(err, &ve) {
				writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
				return
			}
			log.Printf("handlers: UpdateProfile: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, ProfileResponse{DisplayName: profile.DisplayName, HomeAreaName: profile.HomeAreaName})
	}
}

// FirstRunDone produces the handler of POST /me/first-run-done.
//
// @Summary Mark the first-time flow as finished
// @Description Records that the signed in account finished the first-time flow. It needs the current
// @Description terms to be agreed first: without that it answers 409 with the field terms. Send no
// @Description body or an empty object. A repeat call keeps the first time.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Accept json
// @Produce json
// @Success 204 "Done"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /me/first-run-done [post]
func FirstRunDone(svc *service.AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !decodeEmptyBody(w, r) {
			return
		}
		if err := svc.MarkFirstRunDone(r.Context(), acc.ID); err != nil {
			if errors.Is(err, service.ErrTermsNotAgreed) {
				writeFieldError(w, http.StatusConflict, "terms", service.TermsNotAgreedMessage)
				return
			}
			log.Printf("handlers: FirstRunDone: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// DeleteAccountRequest is the body of POST /me/delete. confirm must be true;
// a missing key, false or any other value deletes nothing.
type DeleteAccountRequest struct {
	Confirm *bool `json:"confirm" example:"true"`
}

// DeleteAccount produces the handler of POST /me/delete.
//
// @Summary Delete my account
// @Description Deletes the signed in account. The sign in address is replaced by one that can never
// @Description receive mail, the name, home area and consent are cleared, and every session of the
// @Description account ends, this one included. The updates, muted reports, alert settings and
// @Description sign in links of the account are deleted. The reports and votes the person made stay,
// @Description with no name or link to the person, so every count other people see is unchanged. The
// @Description same address can sign up again as a brand new account with no history. The body must
// @Description say confirm true; anything else deletes nothing.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Accept json
// @Produce json
// @Param body body DeleteAccountRequest true "Must carry confirm true"
// @Success 204 "Deleted"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /me/delete [post]
func DeleteAccount(deletions *service.AccountDeletionService, mgr *session.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxMeBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req DeleteAccountRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}
		if req.Confirm == nil || !*req.Confirm {
			writeFieldError(w, http.StatusBadRequest, "confirm", service.DeleteConfirmMessage)
			return
		}

		if err := deletions.DeleteAccount(r.Context(), acc.ID); err != nil {
			if errors.Is(err, service.ErrAccountAlreadyDeleted) {
				writeFieldError(w, http.StatusNotFound, "account", "This account is already deleted.")
				return
			}
			// The account id only: the address never reaches the log.
			log.Printf("handlers: DeleteAccount: account %d: %v", acc.ID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		mgr.ClearCookie(w)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	}
}
