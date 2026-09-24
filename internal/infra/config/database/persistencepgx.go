package database

import (
	"context"
	"fmt"
	"sync"

	"github.com/WilsonSayago/base-golang/internal/infra/config/prop"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

var (
	databasePostgresPgxSingleton *PostgresPgx
	databasePostgresSQLDBOnce    sync.Once
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

// pgxTransaction implementa la interfaz Transaction usando pgx
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

// Database define la interfaz para operaciones de base de datos
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

// ExecContext executes a statement with the caller's context.
func (db *PostgresPgx) ExecContext(ctx context.Context, sql string, args ...interface{}) error {
	_, err := db.pool.Exec(ctx, sql, args...)
	return err
}

// BeginTx inicia una nueva transacción
func (db *PostgresPgx) BeginTx() (Transaction, error) {
	return db.BeginTxContext(db.ctx)
}

// BeginTxContext starts a transaction bound to ctx.
func (db *PostgresPgx) BeginTxContext(ctx context.Context) (Transaction, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgxTransaction{tx: tx, ctx: ctx}, nil
}

// Pool exposes the underlying pool for tests and advanced adapters.
func (db *PostgresPgx) Pool() *pgxpool.Pool {
	return db.pool
}

// NewPostgresPgxForTest constructs a non-singleton pool wrapper (tests only).
func NewPostgresPgxForTest(pool *pgxpool.Pool) *PostgresPgx {
	return &PostgresPgx{pool: pool, ctx: context.Background()}
}

func NewConnectionPostgresPgx(prop *prop.DatabaseProperty) *PostgresPgx {
	databasePostgresSQLDBOnce.Do(func() {
		/*psqlInfo := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		prop.User, prop.Password, prop.Host, prop.Port, prop.Name)*/

		psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+"password=%s dbname=%s sslmode=disable", prop.Host, prop.Port, prop.User, prop.Password, prop.Name)
		cnxCfg, err := pgxpool.ParseConfig(psqlInfo)
		if err != nil {
			log.Fatalf("could not parse database config: %v", err)
		}
		cnxCfg.MaxConns = prop.DbMaxConns
		pool, err := pgxpool.NewWithConfig(context.Background(), cnxCfg)
		if err != nil {
			log.Fatalf("could not create database connection: %v", err)
		}

		log.Info("Connection successfully to database")

		databasePostgresPgxSingleton = &PostgresPgx{pool, context.Background()}
	})
	return databasePostgresPgxSingleton
}

func GetDatabasePostgresPgx() *PostgresPgx {
	return databasePostgresPgxSingleton
}

func (db *PostgresPgx) Start() {
	log.Info("Started database connection/ping")

	err := db.pool.Ping(context.Background())
	if err != nil {
		log.Fatalf("database is unreachable: %v", err)
	}
	log.Info("Connection successfully to database")
	log.Info("Finished database connection/ping")
}
