package database

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
	"jwt/base/internal/infra/primary/config/prop"
	"sync"
)

var (
	databasePostgresPgxSingleton *PostgresPgx
	databasePostgresSQLDBOnce    sync.Once
)

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

func NewConnectionPostgresPgx(prop *prop.DatabaseProperty) *PostgresPgx {
	databasePostgresSQLDBOnce.Do(func() {
		/*psqlInfo := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		prop.User, prop.Password, prop.Host, prop.Port, prop.Name)*/
		
		psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+"password=%s dbname=%s sslmode=disable", prop.Host, prop.Port, prop.User, prop.Password, prop.Name)
		cnxCfg, err := pgxpool.ParseConfig(psqlInfo)
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
