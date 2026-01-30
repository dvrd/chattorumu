package postgres

import (
	"errors"

	"github.com/lib/pq"
)

const pqUniqueViolation = "23505"

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
