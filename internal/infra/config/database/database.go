package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the minimal query surface shared by pool and transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Database is the process-wide pool handle used by DAOs.
type Database interface {
	DBTX
	BeginTx(ctx context.Context) (Transaction, error)
	Ping(ctx context.Context) error
	Close()
}

// Transaction is a DBTX bound to a single PostgreSQL transaction.
type Transaction interface {
	DBTX
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}
