package db

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgxErrCode returns the SQLSTATE code carried by err, if any.
func pgxErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// IsUniqueViolation reports whether err is a Postgres unique-constraint failure
// (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	return pgxErrCode(err) == "23505"
}

// IsForeignKeyViolation reports whether err is a Postgres foreign-key failure
// (SQLSTATE 23503).
func IsForeignKeyViolation(err error) bool {
	return pgxErrCode(err) == "23503"
}
