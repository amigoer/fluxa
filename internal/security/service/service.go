package service

import (
	"github.com/amigoer/fluxa/internal/security/repo"
)

// Service is the security module's business layer, composed of one
// sub-interface per feature. Each sub-interface is declared and
// implemented in its own file next to this one; this file only assembles
// them and holds the single implementation they all share.
type Service interface {
	ScanService
	RuleService
	EventService
}

type service struct {
	repo repo.Repo
}

func New(repo repo.Repo) Service {
	return &service{repo: repo}
}
