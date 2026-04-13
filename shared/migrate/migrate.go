package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"path"
	"sort"
	"strings"
	"time"
)

func Run(ctx context.Context, db *sql.DB, migrations embed.FS, dirName string) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return fmt.Errorf("migrate: create tracking table: %w", err)
	}

	entries, err := fs.ReadDir(migrations, dirName)
	if err != nil {
		return fmt.Errorf("migrate: read dir %s: %w", dirName, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	for _, fname := range files {
		version := strings.TrimSuffix(fname, ".up.sql")
		applied, err := isApplied(ctx, db, version)
		if err != nil {
			return fmt.Errorf("migrate: check version %s: %w", version, err)
		}
		if applied {
			continue
		}

		content, err := fs.ReadFile(migrations, path.Join(dirName, fname))
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", fname, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("migrate: begin tx for %s: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate: exec %s: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`,
			version, time.Now().UTC(),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate: record %s: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrate: commit %s: %w", version, err)
		}
		log.Printf("migrate: applied %s", version)
	}
	return nil
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)
	return err
}

func isApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=$1`, version).Scan(&count)
	return count > 0, err
}
