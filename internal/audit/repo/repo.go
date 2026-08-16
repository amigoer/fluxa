package repo

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo is the audit module's storage layer, composed of one sub-interface
// per log kind. Each sub-interface is declared and implemented in its own
// file next to this one; this file only assembles them and holds the
// single Postgres-backed implementation they all share.
type Repo interface {
	CallLogRepo
	OperationLogRepo
}

type repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) Repo {
	return &repo{pool: pool}
}
