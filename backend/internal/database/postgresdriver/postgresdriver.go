// Package postgresdriver registers the PostgreSQL GORM driver at init time.
//
// It exists as a separate package on purpose: only the server entrypoint
// imports it, so gorm.io/driver/postgres never enters the desktop binary's
// dependency graph. The repository guards that boundary.
package postgresdriver

import (
	"infinite-canvas/backend/internal/database"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func init() {
	database.RegisterPostgresOpener(func(dsn string) (*gorm.DB, error) {
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	})
}
