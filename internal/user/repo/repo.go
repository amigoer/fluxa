package repo

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("user: not found")

// Repo is the user module's storage layer, composed of one sub-interface
// per feature. Each sub-interface is declared and implemented in its own
// file next to this one; this file only assembles them and holds the
// single Postgres-backed implementation they all share.
type Repo interface {
	OrganizationRepo
	RoleRepo
	DepartmentRepo
	MemberRepo
	IdentityRepo
	AuthSettingsRepo
	MailSettingsRepo
	LocalAccountRepo
	OTPRepo
	SessionRepo
	NotifyRepo
}

type repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) Repo {
	return &repo{pool: pool}
}
