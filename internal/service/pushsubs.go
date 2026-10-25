package service

// pushsubs.go keeps the phone alert subscriptions (Phase 3.3, D-03, D-13). A
// subscription is per device and per session: it is the browser's push endpoint
// plus the two keys that encrypt a message for that device, stored against the
// account that registered it and the session it was registered from.
//
// The endpoint is a secret capability URL: whoever holds it can send to that
// device. So it is checked against the push service allowlist before it is
// stored, it is read from a request body and never from an address, and it is
// never logged, returned or put in an error message. Registering moves an
// endpoint to the caller, so a shared phone never keeps delivering the previous
// person's alerts (UI-SPEC 2a, shared devices). An account holds at most
// PushDevicesPerAccount devices; the oldest are dropped.
//
// The caller's session row exists whenever this is reached: the verified
// account gate resolves the account through the sessions row, so the foreign
// key from push_subscriptions.session_id is always met.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"pinalert/internal/push"
	sqlcgen "pinalert/internal/store/sqlc"
)

// The messages of the subscription routes. They are plain words for the person;
// none of them names the endpoint or a key.
const (
	// PushServiceUnsupportedMessage answers an endpoint on a host that is not a
	// known push service.
	PushServiceUnsupportedMessage = "This browser's notification service is not supported."
	// PushSubscriptionInvalidMessage answers a malformed endpoint or keys.
	PushSubscriptionInvalidMessage = "This device's notification details are not valid."
	// PushUnavailableMessage answers a register call when the server has no
	// VAPID key, so push is switched off.
	PushUnavailableMessage = "Notifications are not available on this server."
)

// PushSubsQuerier is the narrow store surface of PushSubscriptionService: the
// six subscription queries and nothing that reads another table.
type PushSubsQuerier interface {
	UpsertPushSubscription(ctx context.Context, arg sqlcgen.UpsertPushSubscriptionParams) (int64, error)
	TrimPushSubscriptions(ctx context.Context, arg sqlcgen.TrimPushSubscriptionsParams) (int64, error)
	PushSubscriptionRegistered(ctx context.Context, arg sqlcgen.PushSubscriptionRegisteredParams) (bool, error)
	DeletePushSubscriptionForAccount(ctx context.Context, arg sqlcgen.DeletePushSubscriptionForAccountParams) (int64, error)
	DeletePushSubscriptionsForSession(ctx context.Context, sessionID string) (int64, error)
	DeleteAccountPushSubscriptions(ctx context.Context, accountID int64) (int64, error)
}

var _ PushSubsQuerier = (*sqlcgen.Queries)(nil)

// PushSubscriptionInput is what a browser hands over when it subscribes. The
// keys are the base64 strings of PushSubscription.toJSON().
type PushSubscriptionInput struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// PushSubscriptionService applies the subscription rules.
type PushSubscriptionService struct {
	q      PushSubsQuerier
	policy push.EndpointPolicy
	now    func() time.Time
}

// PushSubsOption configures a PushSubscriptionService.
type PushSubsOption func(*PushSubscriptionService)

// WithPushSubsClock replaces the clock, so a test can pin the stored time.
func WithPushSubsClock(now func() time.Time) PushSubsOption {
	return func(s *PushSubscriptionService) {
		if now != nil {
			s.now = now
		}
	}
}

// NewPushSubscriptionService stores the querier and the endpoint policy. It
// never dereferences the querier, so a router built with a nil sessions store
// still constructs.
func NewPushSubscriptionService(q PushSubsQuerier, policy push.EndpointPolicy, opts ...PushSubsOption) *PushSubscriptionService {
	s := &PushSubscriptionService{q: q, policy: policy, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Register validates and stores one device for the account and session, then
// trims the account to PushDevicesPerAccount devices. An endpoint on a host
// that is not a known push service and a malformed endpoint or key set are
// ValidationErrors and store nothing. An endpoint held by another account moves
// to this one.
func (s *PushSubscriptionService) Register(ctx context.Context, accountID int64, sessionID string, in PushSubscriptionInput) error {
	if sessionID == "" {
		// The gate always provides one; a missing id is a programming error.
		return fmt.Errorf("service: Register: account %d: no session id", accountID)
	}
	host, err := s.policy.Check(in.Endpoint)
	if err != nil {
		if errors.Is(err, push.ErrUnsupportedService) {
			return ValidationError{Field: "endpoint", Message: PushServiceUnsupportedMessage}
		}
		return ValidationError{Field: "endpoint", Message: PushSubscriptionInvalidMessage}
	}
	if err := push.CheckKeys(in.P256dh, in.Auth); err != nil {
		return ValidationError{Field: "keys", Message: PushSubscriptionInvalidMessage}
	}

	if _, err := s.q.UpsertPushSubscription(ctx, sqlcgen.UpsertPushSubscriptionParams{
		AccountID:    accountID,
		SessionID:    sessionID,
		Endpoint:     in.Endpoint,
		EndpointHost: host,
		P256dh:       in.P256dh,
		Auth:         in.Auth,
		Now:          s.now(),
	}); err != nil {
		return fmt.Errorf("service: Register: account %d: %w", accountID, scrubStoreError(err))
	}
	if _, err := s.q.TrimPushSubscriptions(ctx, sqlcgen.TrimPushSubscriptionsParams{
		AccountID: accountID,
		Keep:      PushDevicesPerAccount,
	}); err != nil {
		return fmt.Errorf("service: Register: account %d: trim: %w", accountID, scrubStoreError(err))
	}
	return nil
}

// Registered reports whether this account has this endpoint registered. It
// does not check the allowlist: a device stored under a service that was later
// dropped from the list must still be able to ask and to leave.
func (s *PushSubscriptionService) Registered(ctx context.Context, accountID int64, endpoint string) (bool, error) {
	if endpoint == "" {
		return false, ValidationError{Field: "endpoint", Message: PushSubscriptionInvalidMessage}
	}
	ok, err := s.q.PushSubscriptionRegistered(ctx, sqlcgen.PushSubscriptionRegisteredParams{
		AccountID: accountID,
		Endpoint:  endpoint,
	})
	if err != nil {
		return false, fmt.Errorf("service: Registered: account %d: %w", accountID, scrubStoreError(err))
	}
	return ok, nil
}

// Unregister removes this endpoint for the calling account only. An endpoint
// the account does not hold is not an error: the device is already off.
func (s *PushSubscriptionService) Unregister(ctx context.Context, accountID int64, endpoint string) error {
	if endpoint == "" {
		return ValidationError{Field: "endpoint", Message: PushSubscriptionInvalidMessage}
	}
	if _, err := s.q.DeletePushSubscriptionForAccount(ctx, sqlcgen.DeletePushSubscriptionForAccountParams{
		AccountID: accountID,
		Endpoint:  endpoint,
	}); err != nil {
		return fmt.Errorf("service: Unregister: account %d: %w", accountID, scrubStoreError(err))
	}
	return nil
}

// DeleteForSession removes the devices a session registered (plain Log out).
func (s *PushSubscriptionService) DeleteForSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return errors.New("service: DeleteForSession: no session id")
	}
	if _, err := s.q.DeletePushSubscriptionsForSession(ctx, sessionID); err != nil {
		return fmt.Errorf("service: DeleteForSession: %w", scrubStoreError(err))
	}
	return nil
}

// DeleteForAccount removes every device of an account (Log out of all devices
// and Delete my account).
func (s *PushSubscriptionService) DeleteForAccount(ctx context.Context, accountID int64) error {
	if _, err := s.q.DeleteAccountPushSubscriptions(ctx, accountID); err != nil {
		return fmt.Errorf("service: DeleteForAccount: account %d: %w", accountID, scrubStoreError(err))
	}
	return nil
}

// scrubStoreError returns err in a form that is safe to log. A database error
// can carry the offending values in its detail text, and a value here may be an
// endpoint or a key, so a database error is reduced to its SQLSTATE.
func scrubStoreError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return fmt.Errorf("database error, sqlstate %s", pg.Code)
	}
	return err
}
