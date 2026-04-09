package postgres

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies all pending migrations from migrationsPath against databaseURL.
// It returns migrate.ErrNoChange when the schema is already up to date, nil when
// migrations were applied, or a wrapped error on failure.
func RunMigrations(databaseURL, migrationsPath string) error {
	m, err := migrate.New("file://"+migrationsPath, databaseURL)
	if err != nil {
		return fmt.Errorf("postgres: create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return migrate.ErrNoChange
		}
		return fmt.Errorf("postgres: run migrations: %w", err)
	}

	return nil
}
