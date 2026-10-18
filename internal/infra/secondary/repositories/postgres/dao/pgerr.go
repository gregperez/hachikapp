package dao

import (
	"errors"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

func mapUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}
