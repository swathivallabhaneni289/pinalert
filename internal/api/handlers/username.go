// username.go is the live username check of Phase 7.1 (D-07, SC4): the route the
// About you step and Settings call while a person types, to say whether the name
// is free before it is saved. The save itself goes through PATCH /api/me/profile.

package handlers

import (
	"log"
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// UsernameCheckResponse is the answer of GET /username/check. State is one of
// available, yours, taken, invalid, reserved or wait. Message is the sentence
// to show, ready to print. Days is set only for wait and says how many days are
// left before the name can change again; it is left out otherwise.
type UsernameCheckResponse struct {
	State   string `json:"state" example:"available" enums:"available,yours,taken,invalid,reserved,wait"`
	Message string `json:"message" example:"asha_k is available."`
	Days    int    `json:"days,omitempty" example:"7"`
}

// UsernameCheck produces the handler of GET /username/check.
//
// @Summary Check a username while typing
// @Description Says whether a username can be saved by the signed in account: available, yours
// @Description (it already is the account's name), taken (another account holds it), invalid (not
// @Description 3 to 20 letters, numbers, dots or underscores), reserved (kept for official use) or
// @Description wait (the name was changed less than 7 days ago; days says how many days are left).
// @Description The check judges the format, the reserved words and whether another account holds
// @Description the name. It does not call the text screening: that runs once, when the name is saved,
// @Description so a name that reads available can still be refused with 422 on save. Each account
// @Description may check 10 times, then once a second; past that the answer is 429 with Retry-After.
// @Description The answer is never cached.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Produce json
// @Param u query string true "The username to check, with or without a leading @"
// @Success 200 {object} UsernameCheckResponse
// @Failure 401 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse
// @Failure 500 {object} ErrorResponse
// @Router /username/check [get]
func UsernameCheck(accounts *service.AccountService, gate *AuthGate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			// A route mounted outside the gated group, a programming error.
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")

		if check := gate.ReserveUsernameCheck(acc.ID); !check.OK {
			writeRateLimited(w, "username", authRateLimitedMessage, check.RetryAfter, false, "")
			return
		}

		// The name is read from the query. The request logger drops query
		// strings, and this handler logs only the account id, so the name never
		// reaches a log (T-07.1-29).
		got, err := accounts.CheckUsername(r.Context(), acc.ID, r.URL.Query().Get("u"))
		if err != nil {
			log.Printf("handlers: UsernameCheck: account %d: %v", acc.ID, err)
			writeFieldError(w, http.StatusInternalServerError, "username", service.UsernameCheckFailedMessage)
			return
		}
		writeJSON(w, http.StatusOK, UsernameCheckResponse{State: got.State, Message: got.Message, Days: got.Days})
	}
}
