// Package main starts the Revenue Leakage Engine application binary.
package main

import (
	"context"

	"github.com/deplagene/revenueleakageengine/internal/migrator"
	"github.com/deplagene/revenueleakageengine/internal/platform/sqlite"
	"github.com/theartofdevel/logging"
)

const (
	localSQLitePath = "./local.db"
	migrationsPath  = "internal/platform/sqlite/migrations"
)

// main is the process entrypoint.
func main() {
	ctx := context.Background()

	// TODO: add chi router

	// TODO: add config
	logger := logging.NewLogger(
		logging.WithLevel(""),
		logging.WithIsJSON(true),
	)

	ctx = logging.ContextWithLogger(ctx, logger)

	// db
	db, err := sqlite.Open(ctx, sqlite.LocalDSN(localSQLitePath))
	if err != nil {
		logger.Error("main", "error", err)
		return
	}

	defer db.Close()

	// TODO: лучше логгировать
	migrator := migrator.NewMigrator(db, migrationsPath)
	if err := migrator.Up(ctx); err != nil {
		logger.Error("main", "error", err)
		return
	}
}
