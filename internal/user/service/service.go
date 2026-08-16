package service

import (
	"github.com/amigoer/fluxa/internal/user/repo"
)

// Service is the user module's business layer, composed of one
// sub-interface per feature. Each sub-interface is declared and
// implemented in its own file next to this one; this file only assembles
// them and holds the single implementation they all share.
type Service interface {
	BootstrapService
	PrincipalService
	MemberService
	DepartmentService
	RoleService
	IdentityService
	AuthSettingsService
	NotifyService
}

type service struct {
	repo repo.Repo
}

func New(repo repo.Repo) Service {
	return &service{repo: repo}
}
