package backup

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// sqliteBackup creates a consistent point-in-time snapshot of a live SQLite database.
// VACUUM INTO holds only a shared read lock on the source, so it is safe against a
// concurrently running Luanti server.
func sqliteBackup(ctx context.Context, src, dst string) error {
	db, err := sql.Open("sqlite", src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}

	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", dst); err != nil {
		return fmt.Errorf("VACUUM INTO %s -> %s: %w", src, dst, err)
	}

	return nil
}
