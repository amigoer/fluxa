package handler

import (
	"context"
	"errors"

	"github.com/amigoer/fluxa/internal/user/repo"
	"github.com/amigoer/fluxa/internal/user/types"
)

// Being reachable by a one-time code is a property of the person, not of
// which table happens to hold a row for them. A member created by an
// identity source has no local_accounts row, and treating that row as
// the sole answer to "do we know this address?" is what would strand
// every such member the day their source is switched off.
//
// These three keep that in one place, so the login and registration
// paths cannot drift apart on it.

// knownForLocalLogin reports whether a code may be sent to identifier.
func (h *Handler) knownForLocalLogin(ctx context.Context, identifier string) bool {
	if _, err := h.repo.FindLocalAccountByIdentifier(ctx, identifier); err == nil {
		return true
	}
	_, err := h.repo.FindMemberByIdentifier(ctx, identifier)
	return err == nil
}

// memberForLocalLogin resolves a verified identifier to its member, and
// leaves behind the local_accounts row the member was missing so the
// next sign-in takes the direct path.
func (h *Handler) memberForLocalLogin(ctx context.Context, identifier string) (types.Member, error) {
	if account, err := h.repo.FindLocalAccountByIdentifier(ctx, identifier); err == nil {
		return h.repo.GetMember(ctx, account.MemberID)
	} else if !errors.Is(err, repo.ErrNotFound) {
		return types.Member{}, err
	}

	member, err := h.repo.FindMemberByIdentifier(ctx, identifier)
	if err != nil {
		return types.Member{}, err
	}
	if err := h.ensureLocalAccount(ctx, member, identifier); err != nil {
		return types.Member{}, err
	}
	return member, nil
}

// ensureLocalAccount gives member a local_accounts row for identifier if
// they have none. Idempotent: a second sign-in through the same path
// must not add a second row.
func (h *Handler) ensureLocalAccount(ctx context.Context, member types.Member, identifier string) error {
	if _, err := h.repo.FindLocalAccountByIdentifier(ctx, identifier); err == nil {
		return nil
	} else if !errors.Is(err, repo.ErrNotFound) {
		return err
	}

	account := types.LocalAccount{MemberID: member.ID}
	if isPhone(identifier) {
		account.Phone = &identifier
	} else {
		account.Email = &identifier
	}
	_, err := h.repo.CreateLocalAccount(ctx, account)
	return err
}
