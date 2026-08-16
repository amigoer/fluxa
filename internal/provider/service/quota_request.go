package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/amigoer/fluxa/internal/provider/types"
)

var ErrCannotApprove = errors.New("provider: caller may not decide this quota request")

// QuotaRequestService is the request-and-approve path to more budget:
// employees ask, department leads (or admins holding the fallback)
// decide, and an approval actually moves the money.
type QuotaRequestService interface {
	RequestQuota(ctx context.Context, q types.QuotaRequest) (types.QuotaRequest, error)
	ListMyQuotaRequests(ctx context.Context, memberID string) ([]types.QuotaRequest, error)
	ListPendingQuotaRequestsForDepartment(ctx context.Context, departmentID string) ([]types.QuotaRequest, error)
	ListAllPendingQuotaRequests(ctx context.Context) ([]types.QuotaRequest, error)
	DecideQuotaRequest(ctx context.Context, requestID, deciderMemberID string, approve, callerHasApproveAny bool) error
}

func (s *service) RequestQuota(ctx context.Context, q types.QuotaRequest) (types.QuotaRequest, error) {
	return s.repo.CreateQuotaRequest(ctx, q)
}

func (s *service) ListMyQuotaRequests(ctx context.Context, memberID string) ([]types.QuotaRequest, error) {
	return s.repo.ListQuotaRequestsByMember(ctx, memberID)
}

func (s *service) ListPendingQuotaRequestsForDepartment(ctx context.Context, departmentID string) ([]types.QuotaRequest, error) {
	return s.repo.ListPendingQuotaRequestsForDepartment(ctx, departmentID)
}

func (s *service) ListAllPendingQuotaRequests(ctx context.Context) ([]types.QuotaRequest, error) {
	return s.repo.ListAllPendingQuotaRequests(ctx)
}

// DecideQuotaRequest approves or rejects a quota request. deciderMemberID
// must either be the lead of the requester's department, or the caller
// must hold the quota.approve_any admin fallback -- callerHasApproveAny
// carries that already-checked rbac decision in rather than this
// package reaching back into rbac itself (DESIGN.md 8.4: department
// lead by default, admin fallback otherwise).
func (s *service) DecideQuotaRequest(ctx context.Context, requestID, deciderMemberID string, approve, callerHasApproveAny bool) error {
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
func (s *service) grantQuota(ctx context.Context, req types.QuotaRequest) error {
	existing, err := s.repo.ListActiveVirtualKeysByMember(ctx, req.RequestedByMemberID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		key := existing[0]
		return s.repo.UpdateVirtualKeyBudget(ctx, key.ID, key.BudgetCents+req.AmountCents)
	}

	_, _, err = s.CreateVirtualKey(ctx, types.VirtualKey{
		Name:          "个人额度",
		OwnerType:     types.VirtualKeyOwnerMember,
		OwnerMemberID: &req.RequestedByMemberID,
		BudgetCents:   req.AmountCents,
	})
	return err
}
