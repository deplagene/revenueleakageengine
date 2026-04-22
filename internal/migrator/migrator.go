package migrator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/theartofdevel/logging"
)

type Migrator struct {
	db            *sql.DB
	migrationPath string
}

func NewMigrator(db *sql.DB, migrationPath string) *Migrator {
	return &Migrator{
		db:            db,
		migrationPath: migrationPath,
	}
}

func (m *Migrator) Up(ctx context.Context) error {
	const op = "internal.migrator.Migrator.Up"
	logger := logging.L(ctx)

	if err := goose.SetDialect("sqlite3"); err != nil {
		logger.Error(op, "error", err)
		return fmt.Errorf("could not set goose dialect %s: %w", op, err)
	}

	logger.Info(op, "migration_path", m.migrationPath)
	if err := goose.UpContext(ctx, m.db, m.migrationPath); err != nil {
		logger.Error(op, "error", err, "migration_path", m.migrationPath)
		return fmt.Errorf("could not up migrations %s: %w", op, err)
	}

	logger.Info(op+".done", "migration_path", m.migrationPath)
	return nil
}

func (m *Migrator) Down(ctx context.Context) error {
	const op = "internal.migrator.Migrator.Down"
	logger := logging.L(ctx)

	if err := goose.SetDialect("sqlite3"); err != nil {
		logger.Error(op, "error", err)
		return fmt.Errorf("could not set goose dialect %s: %w", op, err)
	}

	logger.Info(op, "migration_path", m.migrationPath)
	if err := goose.DownContext(ctx, m.db, m.migrationPath); err != nil {
		logger.Error(op, "error", err, "migration_path", m.migrationPath)
		return fmt.Errorf("could not down migrations %s: %w", op, err)
	}

	logger.Info(op+".done", "migration_path", m.migrationPath)
	return nil
}
