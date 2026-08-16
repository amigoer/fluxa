package service

import (
	"github.com/amigoer/fluxa/internal/audit/repo"
)

// Service is the audit module's business layer, composed of one
// sub-interface per feature. Each sub-interface is declared and
// implemented in its own file next to this one; this file only assembles
// them and holds the single implementation they all share.
type Service interface {
	CallLogService
	OperationLogService
	MutationRecorder
}

type service struct {
	repo repo.Repo
}

func New(repo repo.Repo) Service {
	return &service{repo: repo}
}
