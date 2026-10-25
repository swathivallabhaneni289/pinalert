// push.go is the HTTP surface of a device's phone alert subscription (Phase
// 3.3, D-03, D-13, UI-SPEC 2a). Three gated JSON routes: register this device,
// ask whether this device is registered, and remove this device. Every rule
// lives in service.PushSubscriptionService; this file decodes strictly, charges
// a per account budget and answers.
//
// The endpoint is a secret capability URL. It travels only in a request body,
// is compared in SQL and is never returned, logged or put in an address. A log
// line here names the account id and the outcome and nothing else. The routes
// sit inside the verified account gate, take application/json only and rely on
// the SameSite Lax session cookie against cross site posts (DEC-L).

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/ratelimit"
	"pinalert/internal/service"
	"pinalert/internal/session"
)

// maxPushBodyBytes caps the bodies of the push routes. A subscription is an
// endpoint of at most 2048 characters and two short keys.
const maxPushBodyBytes = 4096

// The answers of the push routes that have no service message.
const (
	pushRateLimitedMessage   = "Too many requests. Try again in a minute."
	pushBodyMalformedMessage = "Request body is missing or malformed."
	pushBodyTooLargeMessage  = "Request body is too large."
	pushContentTypeMessage   = "Send the request as application/json."
)

// PushKeysRequest is the keys object of a browser's PushSubscription.
type PushKeysRequest struct {
	P256dh string `json:"p256dh" example:"base64url of the 65 byte public key"`
	Auth   string `json:"auth" example:"base64url of the 16 byte secret"`
}

// PushSubscriptionRequest is the body of POST /me/push/subscription: the
// browser's PushSubscription.toJSON(). expirationTime is accepted because the
// browser sends it, and is ignored.
type PushSubscriptionRequest struct {
	Endpoint       string          `json:"endpoint" example:"https://fcm.googleapis.com/fcm/send/example"`
	ExpirationTime *float64        `json:"expirationTime" swaggertype:"number"`
	Keys           PushKeysRequest `json:"keys"`
}

// PushEndpointRequest is the body of the status and unsubscribe routes. The
// endpoint is sent only in the body, never in an address.
type PushEndpointRequest struct {
	Endpoint string `json:"endpoint" example:"https://fcm.googleapis.com/fcm/send/example"`
}

// PushStatusResponse answers whether the calling account has this endpoint.
type PushStatusResponse struct {
	Registered bool `json:"registered" example:"true"`
}

// decodePushBody applies the shared rules of the three routes: application/json
// only (415), a maxPushBodyBytes cap (413), unknown fields refused (400) and
// exactly one JSON value. It reports whether the request may go on.
func decodePushBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		writeFieldError(w, http.StatusUnsupportedMediaType, "body", pushContentTypeMessage)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPushBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err = dec.Decode(dst)
	if err == nil {
		// Exactly one value: anything after it, other than space, is refused.
		if _, next := dec.Token(); !errors.Is(next, io.EOF) {
			err = errors.New("trailing data")
		}
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeFieldError(w, http.StatusRequestEntityTooLarge, "body", pushBodyTooLargeMessage)
			return false
		}
		writeFieldError(w, http.StatusBadRequest, "body", pushBodyMalformedMessage)
		return false
	}
	return true
}

// pushAllowed charges the account's budget on limiter under a key of its own.
// It writes the 429 and reports false when the budget is spent. A nil limiter
// allows everything.
func pushAllowed(w http.ResponseWriter, limiter *ratelimit.PerIP, kind string, accountID int64) bool {
	if limiter == nil || limiter.Allow(fmt.Sprintf("%s:%d", kind, accountID)) {
		return true
	}
	writeFieldError(w, http.StatusTooManyRequests, "push", pushRateLimitedMessage)
	return false
}

// pushServiceError answers a service error: a ValidationError is a 400 on its
// field, anything else is logged by account id and answered with a bare 500.
func pushServiceError(w http.ResponseWriter, handler string, accountID int64, err error) {
	var ve service.ValidationError
	if errors.As(err, &ve) {
		writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
		return
	}
	// The service wraps its errors with the account id and scrubs database
	// detail, so the text holds no endpoint or key.
	log.Printf("handlers: %s: account %d: %v", handler, accountID, err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// RegisterPushSubscription produces the handler of POST /me/push/subscription.
//
// @Summary Register this device for phone alerts
// @Description Stores the browser's push subscription for the signed in account and the calling
// @Description session. The endpoint must be on a known push service and the keys must have the
// @Description right shape, otherwise the answer is 400 with the field endpoint or keys. An endpoint
// @Description already held by another account moves to this one, so a shared phone never keeps the
// @Description previous person's alerts. An account keeps at most 8 devices; the oldest is dropped.
// @Description The endpoint is sent only in the body and is never returned or logged. Answers 503
// @Description when the server has no push key. The body is application/json of at most 4 KB. Each
// @Description account may register 5 devices at once and then one a minute; past that the answer is
// @Description 429. Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Accept json
// @Produce json
// @Param body body PushSubscriptionRequest true "The browser's PushSubscription"
// @Success 204 "Registered"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 413 {object} ErrorResponse
// @Failure 415 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /me/push/subscription [post]
func RegisterPushSubscription(svc *service.PushSubscriptionService, pushKey string, limiter *ratelimit.PerIP) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		acc, ok := account.FromContext(r.Context())
		if !ok {
			// A route mounted outside the gated group, a programming error.
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		sessionID, ok := session.FromContext(r.Context())
		if !ok || sessionID == "" {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		// No VAPID key: every page carries an empty key, so the device reads
		// unsupported and never gets here. A direct call gets a plain 503.
		if pushKey == "" {
			writeFieldError(w, http.StatusServiceUnavailable, "push", service.PushUnavailableMessage)
			return
		}
		if !pushAllowed(w, limiter, "push-write", acc.ID) {
			return
		}
		var req PushSubscriptionRequest
		if !decodePushBody(w, r, &req) {
			return
		}
		if err := svc.Register(r.Context(), acc.ID, sessionID, service.PushSubscriptionInput{
			Endpoint: req.Endpoint,
			P256dh:   req.Keys.P256dh,
			Auth:     req.Keys.Auth,
		}); err != nil {
			pushServiceError(w, "RegisterPushSubscription", acc.ID, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// PushSubscriptionStatus produces the handler of POST /me/push/status.
//
// @Summary Ask whether this device is registered for phone alerts
// @Description Answers whether the signed in account has this endpoint registered. The endpoint is
// @Description sent only in the body, because it is a secret address; it is compared and never
// @Description returned or logged. The body is application/json of at most 4 KB. Each account may ask
// @Description 10 times at once and then once every 6 seconds; past that the answer is 429.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Accept json
// @Produce json
// @Param body body PushEndpointRequest true "The device's endpoint"
// @Success 200 {object} PushStatusResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 413 {object} ErrorResponse
// @Failure 415 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /me/push/status [post]
func PushSubscriptionStatus(svc *service.PushSubscriptionService, limiter *ratelimit.PerIP) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !pushAllowed(w, limiter, "push-status", acc.ID) {
			return
		}
		var req PushEndpointRequest
		if !decodePushBody(w, r, &req) {
			return
		}
		registered, err := svc.Registered(r.Context(), acc.ID, req.Endpoint)
		if err != nil {
			pushServiceError(w, "PushSubscriptionStatus", acc.ID, err)
			return
		}
		writeJSON(w, http.StatusOK, PushStatusResponse{Registered: registered})
	}
}

// UnregisterPushSubscription produces the handler of POST /me/push/unsubscribe.
//
// @Summary Remove this device from phone alerts
// @Description Removes this endpoint for the signed in account only; an endpoint that belongs to
// @Description another account, or to nobody, changes nothing and still answers 204. The endpoint is
// @Description sent only in the body and is never returned or logged. The body is application/json of
// @Description at most 4 KB. Each account may remove 5 devices at once and then one a minute; past
// @Description that the answer is 429. Requires a session verified by email; an unverified caller
// @Description receives 401.
// @Tags me
// @Accept json
// @Produce json
// @Param body body PushEndpointRequest true "The device's endpoint"
// @Success 204 "Removed"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 413 {object} ErrorResponse
// @Failure 415 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /me/push/unsubscribe [post]
func UnregisterPushSubscription(svc *service.PushSubscriptionService, limiter *ratelimit.PerIP) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !pushAllowed(w, limiter, "push-write", acc.ID) {
			return
		}
		var req PushEndpointRequest
		if !decodePushBody(w, r, &req) {
			return
		}
		if err := svc.Unregister(r.Context(), acc.ID, req.Endpoint); err != nil {
			pushServiceError(w, "UnregisterPushSubscription", acc.ID, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
