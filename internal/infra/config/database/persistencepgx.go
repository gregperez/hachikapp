package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/config/prop"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

var (
	databasePostgresPgxSingleton *PostgresPgx
	databasePostgresSQLDBOnce    sync.Once
	databaseOpenErr              error
)

type pgxTransaction struct {
	tx pgx.Tx
}

func (t *pgxTransaction) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

func (t *pgxTransaction) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

func (t *pgxTransaction) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return t.tx.Exec(ctx, sql, args...)
}

func (t *pgxTransaction) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *pgxTransaction) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

type PostgresPgx struct {
	pool *pgxpool.Pool
}

func (db *PostgresPgx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.pool.QueryRow(ctx, sql, args...)
}

func (db *PostgresPgx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return db.pool.Query(ctx, sql, args...)
}

func (db *PostgresPgx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return db.pool.Exec(ctx, sql, args...)
}

// QueryRows is kept for transitional callers; prefer Query.
func (db *PostgresPgx) QueryRows(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return db.Query(ctx, sql, args...)
}

func (db *PostgresPgx) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgxTransaction{tx: tx}, nil
}

func (db *PostgresPgx) Pool() *pgxpool.Pool {
	return db.pool
}

func (db *PostgresPgx) Ping(ctx context.Context) error {
	if db == nil || db.pool == nil {
		return fmt.Errorf("database pool is nil")
	}
	return db.pool.Ping(ctx)
}

func (db *PostgresPgx) Close() {
	if db != nil && db.pool != nil {
		db.pool.Close()
	}
}

// NewPostgresPgxForTest constructs a non-singleton pool wrapper (tests only).
func NewPostgresPgxForTest(pool *pgxpool.Pool) *PostgresPgx {
	return &PostgresPgx{pool: pool}
}

// OpenPool creates a pgx pool from validated app config (same DSN as migrations).
func OpenPool(ctx context.Context, cfg appconfig.DatabaseConfig) (*PostgresPgx, error) {
	dsn, err := cfg.DSN()
	if err != nil {
		return nil, err
	}
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.ConnectTimeout > 0 {
		poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}

	openCtx := ctx
	if cfg.ConnectTimeout > 0 {
		var cancel context.CancelFunc
		openCtx, cancel = context.WithTimeout(ctx, cfg.ConnectTimeout)
		defer cancel()
	}

	pool, err := pgxpool.NewWithConfig(openCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}

	return &PostgresPgx{pool: pool}, nil
}

// SetSingleton installs the process-wide pool used by legacy Once wiring.
func SetSingleton(db *PostgresPgx) {
	databasePostgresPgxSingleton = db
}

// NewConnectionPostgresPgx opens (once) using YAML database properties.
func NewConnectionPostgresPgx(prop *prop.DatabaseProperty) *PostgresPgx {
	databasePostgresSQLDBOnce.Do(func() {
		ssl := prop.SSLMode
		if ssl == "" {
			ssl = "disable"
		}
		cfg := appconfig.DatabaseConfig{
			Host:            prop.Host,
			Port:            prop.Port,
			Name:            prop.Name,
			User:            prop.User,
			Password:        prop.Password,
			SSLMode:         ssl,
			MaxConns:        prop.DbMaxConns,
			ConnectTimeout:  5 * time.Second,
			MaxConnLifetime: time.Hour,
		}
		db, err := OpenPool(context.Background(), cfg)
		if err != nil {
			databaseOpenErr = err
			log.Errorf("database open failed: %v", err)
			return
		}
		databasePostgresPgxSingleton = db
		log.Info("database pool ready")
	})
	return databasePostgresPgxSingleton
}

func GetDatabasePostgresPgx() *PostgresPgx {
	return databasePostgresPgxSingleton
}

func OpenError() error {
	return databaseOpenErr
}

// Start pings the pool (legacy lifecycle hook).
func (db *PostgresPgx) Start() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("database is unreachable: %v", err)
	}
}
