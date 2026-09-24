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

// Transaction representa una transacción de base de datos
type Transaction interface {
	QueryRow(sql string, args ...interface{}) pgx.Row
	QueryRows(sql string, args ...interface{}) (pgx.Rows, error)
	Exec(sql string, args ...interface{}) error
	ExecResult(sql string, args ...interface{}) (pgconn.CommandTag, error)
	Commit() error
	Rollback() error
}

type pgxTransaction struct {
	tx  pgx.Tx
	ctx context.Context
}

func (t *pgxTransaction) QueryRow(sql string, args ...interface{}) pgx.Row {
	return t.tx.QueryRow(t.ctx, sql, args...)
}

func (t *pgxTransaction) QueryRows(sql string, args ...interface{}) (pgx.Rows, error) {
	return t.tx.Query(t.ctx, sql, args...)
}

func (t *pgxTransaction) Exec(sql string, args ...interface{}) error {
	_, err := t.tx.Exec(t.ctx, sql, args...)
	return err
}

func (t *pgxTransaction) ExecResult(sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return t.tx.Exec(t.ctx, sql, args...)
}

func (t *pgxTransaction) Commit() error {
	return t.tx.Commit(t.ctx)
}

func (t *pgxTransaction) Rollback() error {
	return t.tx.Rollback(t.ctx)
}

// DatabaseOperations defines the interface for database operations.
type DatabaseOperations interface {
	QueryRow(sql string, args ...interface{}) pgx.Row
	QueryRows(sql string, args ...interface{}) (pgx.Rows, error)
	Exec(sql string, args ...interface{}) error
	BeginTx() (Transaction, error)
}

type PostgresPgx struct {
	pool *pgxpool.Pool
	ctx  context.Context
}

func (db *PostgresPgx) QueryRow(sql string, args ...interface{}) pgx.Row {
	return db.pool.QueryRow(db.ctx, sql, args...)
}

func (db *PostgresPgx) QueryRows(sql string, args ...interface{}) (pgx.Rows, error) {
	rows, err := db.pool.Query(db.ctx, sql, args...)
	return rows, err
}

func (db *PostgresPgx) Exec(sql string, args ...interface{}) error {
	_, err := db.pool.Exec(db.ctx, sql, args...)
	return err
}

func (db *PostgresPgx) ExecContext(ctx context.Context, sql string, args ...interface{}) error {
	_, err := db.pool.Exec(ctx, sql, args...)
	return err
}

func (db *PostgresPgx) BeginTx() (Transaction, error) {
	return db.BeginTxContext(db.ctx)
}

func (db *PostgresPgx) BeginTxContext(ctx context.Context) (Transaction, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgxTransaction{tx: tx, ctx: ctx}, nil
}

func (db *PostgresPgx) Pool() *pgxpool.Pool {
	return db.pool
}

// Ping verifies connectivity with a timeout.
func (db *PostgresPgx) Ping(ctx context.Context) error {
	if db == nil || db.pool == nil {
		return fmt.Errorf("database pool is nil")
	}
	return db.pool.Ping(ctx)
}

// Close closes the underlying pool.
func (db *PostgresPgx) Close() {
	if db != nil && db.pool != nil {
		db.pool.Close()
	}
}

// NewPostgresPgxForTest constructs a non-singleton pool wrapper (tests only).
func NewPostgresPgxForTest(pool *pgxpool.Pool) *PostgresPgx {
	return &PostgresPgx{pool: pool, ctx: context.Background()}
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

	return &PostgresPgx{pool: pool, ctx: context.Background()}, nil
}

// SetSingleton installs the process-wide pool used by legacy Once wiring.
func SetSingleton(db *PostgresPgx) {
	databasePostgresPgxSingleton = db
}

// NewConnectionPostgresPgx opens (once) using YAML database properties.
// Prefer OpenPool + SetSingleton from the application bootstrap.
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

// OpenError returns the last singleton open error, if any.
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
