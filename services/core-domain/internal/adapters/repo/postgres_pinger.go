package repo

import (
	"context"
	"database/sql"
)

// PostgresPinger reports whether the learning-graph store (Postgres) is
// reachable, backing the Core Domain Service readiness probe. It shares the
// *sql.DB that the ent client is built on, so a ping reflects the same pool
// the service actually queries.
type PostgresPinger struct {
	db *sql.DB
}

func NewPostgresPinger(db *sql.DB) *PostgresPinger {
	return &PostgresPinger{db: db}
}

func (p *PostgresPinger) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}
