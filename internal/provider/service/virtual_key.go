package service

import (
	"context"
	"time"

	"github.com/amigoer/fluxa/internal/provider/keyauth"
	"github.com/amigoer/fluxa/internal/provider/types"
)

// VirtualKeyService issues and retires the keys callers authenticate
// with, and is where a completed call's cost is deducted from one.
type VirtualKeyService interface {
	CreateVirtualKey(ctx context.Context, k types.VirtualKey) (types.VirtualKey, string, error)
	ListVirtualKeys(ctx context.Context, orgID string) ([]types.VirtualKey, error)
	RevokeVirtualKey(ctx context.Context, id string) error
	ReserveQuota(ctx context.Context, keyID string, amountMicroCents int64) (string, bool, error)
	SettleQuota(ctx context.Context, reservationID string, actualMicroCents int64) error
	ReleaseQuota(ctx context.Context, reservationID string) error
	ExpireStaleReservations(ctx context.Context) (int64, error)
}

// CreateVirtualKey generates a new key secret, stores only its hash, and
// returns the raw secret alongside the record -- the only time it is
// ever available in full (DESIGN.md 7.2 "鉴权与一致性").
func (s *service) CreateVirtualKey(ctx context.Context, k types.VirtualKey) (types.VirtualKey, string, error) {
	raw, prefix, err := keyauth.GenerateSecret()
	if err != nil {
		return types.VirtualKey{}, "", err
	}
	k.SecretHash = keyauth.HashSecret(raw)
	k.SecretPrefix = prefix
	k.Status = types.VirtualKeyStatusActive

	created, err := s.repo.CreateVirtualKey(ctx, k)
	if err != nil {
		return types.VirtualKey{}, "", err
	}
	return created, raw, nil
}

func (s *service) ListVirtualKeys(ctx context.Context, orgID string) ([]types.VirtualKey, error) {
	return s.repo.ListVirtualKeys(ctx, orgID)
}

// RevokeVirtualKey retires a key and drops it from the authentication
// cache in the same breath.
//
// Without the eviction the key stayed usable for up to the cache's TTL,
// which is the wrong answer to "revoke this key now" whatever the window
// is -- a leaked key is revoked precisely because somebody else has it.
// The reservation SQL refuses a revoked key too, so a call could not
// have been *made* in that window, but everything before the
// reservation still ran, the refusal came back as "out of quota" rather
// than "revoked", and the endpoints that authenticate without reserving
// had nothing stopping them at all.
func (s *service) RevokeVirtualKey(ctx context.Context, id string) error {
	secretHash, err := s.repo.RevokeVirtualKey(ctx, id)
	if err != nil {
		return err
	}
	s.keys.Forget(secretHash)
	return nil
}

// reservationTTL is how long a reservation survives without being
// settled before the sweeper takes it back. It has to outlast the
// longest call the gateway will wait on -- the upstream client's own
// 5-minute deadline -- or a slow-but-healthy request would have its
// budget pulled out from under it while it was still running.
const reservationTTL = 10 * time.Minute

// ReserveQuota admits a call against keyID's remaining budget before it
// is made, returning the reservation to settle it with. ok is false when
// the budget cannot cover it, and the caller must not proceed.
func (s *service) ReserveQuota(ctx context.Context, keyID string, amountMicroCents int64) (string, bool, error) {
	return s.repo.ReserveFromVirtualKey(ctx, keyID, amountMicroCents, reservationTTL)
}

// SettleQuota closes a reservation out at what the call actually cost.
func (s *service) SettleQuota(ctx context.Context, reservationID string, actualMicroCents int64) error {
	return s.repo.SettleReservation(ctx, reservationID, actualMicroCents)
}

// ReleaseQuota gives a reservation back without charging for it: the
// call failed, was refused downstream, or never reached the provider.
func (s *service) ReleaseQuota(ctx context.Context, reservationID string) error {
	return s.repo.SettleReservation(ctx, reservationID, 0)
}

// ExpireStaleReservations releases reservations left behind by calls
// that never settled -- a killed process, a panic past the recover.
// Wired to a periodic sweep in cmd/server.
func (s *service) ExpireStaleReservations(ctx context.Context) (int64, error) {
	return s.repo.ExpireStaleReservations(ctx)
}
