package db

import (
	"errors"
	"strings"

	"github.com/jackc/pgconn"
)

// IsDuplicateKeyError returns true if the passed error indicates that the last INSERT failed because
// a unique constraint was violated
func IsDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	ok := errors.As(err, &pgErr)
	if ok {
		return pgErr.Code == "23505"
	}
	return strings.Contains(err.Error(), "(SQLSTATE 23505)")
}
