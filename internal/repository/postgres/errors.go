package postgres

import (
	"errors"

	"github.com/lib/pq"
)

const pqUniqueViolation = "23505"

// IsUniqueViolation checks if an error is a PostgreSQL unique constraint violation
// If constraint is empty, it returns true for any unique violation
// If constraint is specified, it only returns true for that specific constraint
func IsUniqueViolation(err error, constraint string) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}

	if string(pqErr.Code) != pqUniqueViolation {
		return false
	}

	if constraint == "" {
		return true
	}

	return pqErr.Constraint == constraint
}
