package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const DriverName = "sqlite"

// LocalDSN returns a SQLite DSN suitable for local MVP development.
func LocalDSN(path string) string {
	return fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)",
		path,
	)
}

// Open creates a SQLite connection pool and verifies that the database is reachable.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	const op = "platform.sqlite.Open"

	db, err := sql.Open(DriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return db, nil
}
