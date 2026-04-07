package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/haibread/ai-registry/migrations"
)

// Migrate runs all pending up-migrations against the database at dsn.
// It is idempotent: if the schema is already up-to-date, it returns nil.
func Migrate(dsn string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, pgx5DSN(dsn))
	if err != nil {
		return fmt.Errorf("initialising migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}

// pgx5DSN converts a postgres:// DSN to the pgx5:// scheme expected by the
// golang-migrate pgx/v5 driver.
func pgx5DSN(dsn string) string {
	return strings.Replace(dsn, "postgres://", "pgx5://", 1)
}
