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
// optional: a key left out changes nothing. Amended 2026-10-29 by Phase 7.1
// plan 10 (D-06, D-07): username is the public, unique name; display_name and
// home_area can be changed but no longer emptied, so an empty display_name and
// a null home_area are refused with 400.
type UpdateProfileRequest struct {
	DisplayName *string          `json:"display_name" example:"Asha"`
	HomeArea    *HomeAreaRequest `json:"home_area"`
	Username    *string          `json:"username" example:"asha_k"`
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
// @Summary Save the username, name and home area
// @Description Updates the username, the name and the home area of the signed in account. A key left
// @Description out changes nothing. The name and the home area can be changed but not emptied: an
// @Description empty display_name and a null home_area are refused with 400. A home_area object saves
// @Description the picked place. The home area keeps only the place name and a cell about 5 km across;
// @Description the coordinates you send are used once to find the cell and are never stored. The answer
// @Description is the profile as stored, with no coordinates.
// @Description The username is 3 to 20 letters, numbers, dots or underscores, unique, and can be changed
// @Description once every 7 days. Each refusal has one fixed status. 400 with the field username: the
// @Description name is taken, has the wrong characters, is kept for official use, or is changed too
// @Description soon. 422 with the field username: the text screening refused the name; this is the only
// @Description 422. 429 with the field username and a Retry-After header: the account spent its save
// @Description allowance (5, then one every 10 seconds).
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Accept json
// @Produce json
// @Param body body UpdateProfileRequest true "The rows to change"
// @Success 200 {object} ProfileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse
// @Failure 500 {object} ErrorResponse
// @Router /me/profile [patch]
func UpdateProfile(svc *service.AccountService, gate *AuthGate) http.HandlerFunc {
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
			Username    *string         `json:"username"`
		}
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		// A request that names a username spends one token of the per account
		// save allowance before anything else (D-07). The token is not given
		// back when the service refuses the name: a refused guess counts, which
		// is what stops a script from walking the name space (W12).
		if req.Username != nil {
			if save := gate.ReserveUsernameSave(acc.ID); !save.OK {
				w.Header().Set("Cache-Control", "no-store")
				writeRateLimited(w, "username", authRateLimitedMessage, save.RetryAfter, false, "")
				return
			}
		}

		update := service.ProfileUpdate{DisplayName: req.DisplayName, Username: req.Username}
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
			// The screening refusal is the only 422. It is a sentinel, not a
			// ValidationError, so it is matched first and can never fall into
			// the 400 branch below.
			if errors.Is(err, service.ErrUsernameRefused) {
				writeFieldError(w, http.StatusUnprocessableEntity, "username", service.UsernameRefusedMessage)
				return
			}
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

// AlertSettingsRequest is the body of PATCH /me/alerts. The one key,
// alert_radius_km, is how far from the home area alerts reach.
//
// Owner decision 2026-11-03, quick 261103-jal (D7): the four boolean keys of
// earlier versions, reliable_alerts, critical_push, changes_push and
// official_push, are no longer documented. They stay on the struct, hidden from
// the API reference, because the handler decodes with DisallowUnknownFields and
// an old cached page still sends them; they are decoded and ignored.
type AlertSettingsRequest struct {
	AlertRadiusKm *int `json:"alert_radius_km" example:"5"`

	ReliableAlerts *bool `json:"reliable_alerts" swaggerignore:"true"`
	CriticalPush   *bool `json:"critical_push" swaggerignore:"true"`
	ChangesPush    *bool `json:"changes_push" swaggerignore:"true"`
	OfficialPush   *bool `json:"official_push" swaggerignore:"true"`
}

// ignoredKeySent reports whether the body named one of the four ignored keys.
func (req AlertSettingsRequest) ignoredKeySent() bool {
	return req.ReliableAlerts != nil || req.CriticalPush != nil || req.ChangesPush != nil || req.OfficialPush != nil
}

// AlertSettingsResponse is the caller's alert settings as stored. It carries no
// position and no home area cell, only whether a home area is saved.
type AlertSettingsResponse struct {
	AlertRadiusKm int  `json:"alert_radius_km" example:"5"`
	HasHomeArea   bool `json:"has_home_area" example:"true"`
}

// UpdateAlertSettings produces the handler of PATCH /me/alerts.
//
// @Summary Choose how far from the home area alerts reach
// @Description Saves the one alert setting of the signed in account: alert_radius_km, how far from the
// @Description home area alerts reach, 1, 2, 5 or 10 km (5 km until changed). It is measured from the
// @Description middle of the saved home area, which is a cell about 5 km across; no position is ever sent
// @Description or stored for this. It applies to Critical reports near the home area and to the Reliable
// @Description reporter row in the bell. Whether a device gets notifications is decided only by that
// @Description device's own registration, not by this call. Keys from earlier versions (the four
// @Description on or off switches for the Reliable alert, Critical reports, changes and official alerts) are
// @Description accepted and ignored. A body with no alert_radius_km is refused with the field body. The
// @Description answer is the stored distance and whether a home area is saved.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Accept json
// @Produce json
// @Param body body AlertSettingsRequest true "The settings to change"
// @Success 200 {object} AlertSettingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /me/alerts [patch]
func UpdateAlertSettings(accounts *service.AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			// A route mounted outside the gated group, a programming error.
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxMeBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req AlertSettingsRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		// A body that names only the ignored keys (an old cached page) changes
		// nothing and answers with the stored settings.
		var (
			got service.AlertSettings
			err error
		)
		if req.AlertRadiusKm == nil && req.ignoredKeySent() {
			got, err = accounts.AlertSettings(r.Context(), acc.ID)
		} else {
			got, err = accounts.UpdateAlertSettings(r.Context(), acc.ID, service.AlertSettingsUpdate{AlertRadiusKm: req.AlertRadiusKm})
		}
		if err != nil {
			var ve service.ValidationError
			if errors.As(err, &ve) {
				writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
				return
			}
			log.Printf("handlers: UpdateAlertSettings: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, AlertSettingsResponse{
			AlertRadiusKm: got.AlertRadiusKm,
			HasHomeArea:   got.HasHomeArea,
		})
	}
}

// FirstRunDone produces the handler of POST /me/first-run-done.
//
// @Summary Mark the first-time flow as finished
// @Description Records that the signed in account finished the first-time flow. It needs the current
// @Description terms to be agreed first: without that it answers 409 with the field terms. It also
// @Description needs the username, the name and the home area to be saved (step 4 of the flow is
// @Description required): without them it answers 409 with the field profile. Send no body or an
// @Description empty object. A repeat call keeps the first time.
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
			if errors.Is(err, service.ErrProfileIncomplete) {
				writeFieldError(w, http.StatusConflict, "profile", service.ProfileIncompleteMessage)
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
