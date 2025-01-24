package database

import (
	"github.com/jackc/pgx/v5"
)

type Database interface {
	QueryRow(sql string, args ...interface{}) pgx.Row
	QueryRows(sql string, args ...interface{}) (pgx.Rows, error)
	Exec(sql string, args ...interface{}) error
}
