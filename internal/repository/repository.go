// Package repository is the only layer allowed to write SQL. Services call
// repositories through plain Go method signatures and never see a query
// string — that keeps persistence details out of business logic and makes
// the service layer unit-testable against fakes.
package repository

import (
	"errors"
	"log"

	"github.com/jackc/pgx/v5/pgconn"
)

const pgUniqueViolation = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		isUnique := pgErr.Code == pgUniqueViolation
		if isUnique {
			log.Printf("[repo] isUniqueViolation: detected unique violation code=%s constraint=%s detail=%s",
				pgErr.Code, pgErr.ConstraintName, pgErr.Detail)
		}
		return isUnique
	}
	return false
}

func init() {
	log.Println("[repo] Repository package initialized")
}
