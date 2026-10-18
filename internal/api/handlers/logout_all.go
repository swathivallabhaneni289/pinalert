// logout_all.go is the HTTP surface of Log out of all devices (Phase 3.2, D-23,
// LAYOUT-06). The handler acts on the account the gate resolved and on nothing a
// request body can name, ends every session of that account through
// service.SessionService, and clears this browser's cookie as well. The route
// is gated and a cross site POST carries no session cookie (DEC-L, SameSite
// Lax), so no CSRF token is needed. Plain Log out (POST /auth/logout) is a
// different route and is unchanged: it ends this browser only (DEC-K).

package handlers

import (
	"log"
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/service"
	"pinalert/internal/session"
)

// LogOutAllDevices produces the handler of POST /me/logout-all.
//
// @Summary Log out of all devices
// @Description Ends the signed in account's sessions on every device, including this one. Reports and
// @Description votes keep their author, so nothing other people see changes. Send no body or an empty
// @Description object. Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Accept json
// @Produce json
// @Param body body emptyBody false "No fields"
// @Success 204 "Logged out everywhere"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /me/logout-all [post]
func LogOutAllDevices(sessions *service.SessionService, mgr *session.Manager) http.HandlerFunc {
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

		if _, err := sessions.LogOutAllDevices(r.Context(), acc.ID); err != nil {
			// The account id only: the address never reaches the log.
			log.Printf("handlers: LogOutAllDevices: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		mgr.ClearCookie(w)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	}
}
