package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/amigoer/fluxa/internal/provider/health"
	"github.com/amigoer/fluxa/internal/provider/keyauth"
	"github.com/amigoer/fluxa/internal/provider/quota"
	"github.com/amigoer/fluxa/internal/provider/routing"
	"github.com/amigoer/fluxa/internal/provider/types"
)

var ErrCannotApprove = errors.New("provider: caller may not decide this quota request")

type Service struct {
	repo     *Repo
	Breaker  *health.Breaker
	Keys     *keyauth.Cache
	Resolver *routing.Resolver
}

func NewService(repo *Repo) *Service {
	breaker := health.NewBreaker(repo)
	return &Service{
		repo:     repo,
		Breaker:  breaker,
		Keys:     keyauth.NewCache(repo),
		Resolver: routing.NewResolver(repo, repo, breaker),
	}
}

// -- Providers & models -----------------------------------------------

func (s *Service) CreateProvider(ctx context.Context, p types.Provider) (types.Provider, error) {
	p.Status = types.ProviderStatusActive
	created, err := s.repo.CreateProvider(ctx, p)
	if err != nil {
		return types.Provider{}, err
	}
	// Every provider starts healthy; seed its breaker row so
	// health.Breaker never has to special-case "no row yet".
	if _, err := s.repo.GetProviderHealth(ctx, created.ID); err != nil {
		return types.Provider{}, err
	}
	return created, nil
}

func (s *Service) ListProviders(ctx context.Context, orgID string) ([]types.Provider, error) {
	return s.repo.ListProviders(ctx, orgID)
}

func (s *Service) CreateModel(ctx context.Context, m types.Model) (types.Model, error) {
	return s.repo.CreateModel(ctx, m)
}

func (s *Service) ListModels(ctx context.Context, orgID string) ([]types.Model, error) {
	return s.repo.ListModels(ctx, orgID)
}

func (s *Service) ListPublishedModels(ctx context.Context, orgID string) ([]types.Model, error) {
	return s.repo.ListPublishedModels(ctx, orgID)
}

func (s *Service) RecordProcurement(ctx context.Context, rec types.ProcurementRecord) (types.ProcurementRecord, error) {
	return s.repo.RecordProcurement(ctx, rec)
}

func (s *Service) ListProcurementRecords(ctx context.Context, orgID string) ([]types.ProcurementRecord, error) {
	return s.repo.ListProcurementRecords(ctx, orgID)
}

// -- Routing ------------------------------------------------------------

func (s *Service) CreateRoutingRule(ctx context.Context, rule types.RoutingRule) (types.RoutingRule, error) {
	return s.repo.CreateRoutingRule(ctx, rule)
}

func (s *Service) ListRoutingChain(ctx context.Context, scope types.RoutingScope, ownerMemberID *string) ([]types.RoutingRule, error) {
	return s.repo.ListRoutingChain(ctx, scope, ownerMemberID)
}

// -- Virtual keys -----------------------------------------------------

// CreateVirtualKey generates a new key secret, stores only its hash, and
// returns the raw secret alongside the record -- the only time it is
// ever available in full (DESIGN.md 7.2 "鉴权与一致性").
func (s *Service) CreateVirtualKey(ctx context.Context, k types.VirtualKey) (types.VirtualKey, string, error) {
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

func (s *Service) ListVirtualKeys(ctx context.Context, orgID string) ([]types.VirtualKey, error) {
	return s.repo.ListVirtualKeys(ctx, orgID)
}

func (s *Service) RevokeVirtualKey(ctx context.Context, id string) error {
	return s.repo.RevokeVirtualKey(ctx, id)
}

// -- Department quota pools ----------------------------------------------

func (s *Service) SetDepartmentQuotaPool(ctx context.Context, departmentID string, totalCents int64) error {
	return s.repo.UpsertDepartmentQuotaPool(ctx, departmentID, totalCents)
}

func (s *Service) DepartmentQuotaBalance(ctx context.Context, departmentID string) (quota.Balance, error) {
	return quota.GetBalance(ctx, s.repo, departmentID)
}

// -- Quota requests -----------------------------------------------------

func (s *Service) RequestQuota(ctx context.Context, q types.QuotaRequest) (types.QuotaRequest, error) {
	return s.repo.CreateQuotaRequest(ctx, q)
}

func (s *Service) ListMyQuotaRequests(ctx context.Context, memberID string) ([]types.QuotaRequest, error) {
	return s.repo.ListQuotaRequestsByMember(ctx, memberID)
}

func (s *Service) ListPendingQuotaRequestsForDepartment(ctx context.Context, departmentID string) ([]types.QuotaRequest, error) {
	return s.repo.ListPendingQuotaRequestsForDepartment(ctx, departmentID)
}

func (s *Service) ListAllPendingQuotaRequests(ctx context.Context) ([]types.QuotaRequest, error) {
	return s.repo.ListAllPendingQuotaRequests(ctx)
}

// DecideQuotaRequest approves or rejects a quota request. deciderMemberID
// must either be the lead of the requester's department, or the caller
// must hold the quota.approve_any admin fallback -- callerHasApproveAny
// carries that already-checked rbac decision in rather than this
// package reaching back into rbac itself (DESIGN.md 8.4: department
// lead by default, admin fallback otherwise).
func (s *Service) DecideQuotaRequest(ctx context.Context, requestID, deciderMemberID string, approve, callerHasApproveAny bool) error {
	if !callerHasApproveAny {
		can, err := s.repo.CanApproveQuotaRequest(ctx, requestID, deciderMemberID)
		if err != nil {
			return err
		}
		if !can {
			return ErrCannotApprove
		}
	}

	if approve {
		req, err := s.repo.GetQuotaRequest(ctx, requestID)
		if err != nil {
			return err
		}
		if err := s.grantQuota(ctx, req); err != nil {
			return fmt.Errorf("provider: grant approved quota: %w", err)
		}
	}

	status := types.QuotaRequestRejected
	if approve {
		status = types.QuotaRequestApproved
	}
	return s.repo.DecideQuotaRequest(ctx, requestID, status, deciderMemberID)
}

// grantQuota applies an approved request's amount to the requester's
// virtual key: topping up their existing personal key if they have one,
// or creating a first one otherwise. This is what an approval actually
// does, as opposed to just flipping the request's status.
func (s *Service) grantQuota(ctx context.Context, req types.QuotaRequest) error {
	existing, err := s.repo.ListActiveVirtualKeysByMember(ctx, req.RequestedByMemberID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		key := existing[0]
		return s.repo.updateVirtualKeyBudget(ctx, key.ID, key.BudgetCents+req.AmountCents)
	}

	_, _, err = s.CreateVirtualKey(ctx, types.VirtualKey{
		Name:          "个人额度",
		OwnerType:     types.VirtualKeyOwnerMember,
		OwnerMemberID: &req.RequestedByMemberID,
		BudgetCents:   req.AmountCents,
	})
	return err
}

// AdjustMemberQuota lets an admin (or a department lead within their own
// pool) directly grant or resize a member's virtual key budget, the
// second path that coexists with quota requests (DESIGN.md 7.2, open
// question #1 as resolved: both paths write the same virtual_keys row,
// department pool balance is derived, so the two can't drift apart).
func (s *Service) AdjustMemberQuota(ctx context.Context, keyID string, newBudgetCents int64) error {
	if newBudgetCents < 0 {
		return fmt.Errorf("provider: budget cannot be negative")
	}
	// Implemented as revoke-and-recreate would lose the key's secret;
	// instead this updates budget_cents directly through a minimal
	// repo call kept next to CreateVirtualKey for locality.
	return s.repo.updateVirtualKeyBudget(ctx, keyID, newBudgetCents)
}
