package service

import (
	"context"

	"github.com/amigoer/fluxa/internal/provider/keyauth"
	"github.com/amigoer/fluxa/internal/provider/types"
)

// VirtualKeyService issues and retires the keys callers authenticate
// with, and is where a completed call's cost is deducted from one.
type VirtualKeyService interface {
	CreateVirtualKey(ctx context.Context, k types.VirtualKey) (types.VirtualKey, string, error)
	ListVirtualKeys(ctx context.Context, orgID string) ([]types.VirtualKey, error)
	RevokeVirtualKey(ctx context.Context, id string) error
	SpendFromVirtualKey(ctx context.Context, keyID string, amountCents int64) (bool, error)
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

func (s *service) RevokeVirtualKey(ctx context.Context, id string) error {
	return s.repo.RevokeVirtualKey(ctx, id)
}

// SpendFromVirtualKey is the gateway's entry point for deducting a
// completed call's cost from the virtual key that authenticated it; see
// the repo method of the same name for the atomicity and monthly-rollover
// behavior.
func (s *service) SpendFromVirtualKey(ctx context.Context, keyID string, amountCents int64) (bool, error) {
	return s.repo.SpendFromVirtualKey(ctx, keyID, amountCents)
}
