package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// migrationFiles is embedded so every API and worker image applies the exact
// same schema without depending on a mutable host directory.
//
//go:embed sql/*.sql
var migrationFiles embed.FS

const migrationLockID int64 = 7249150381

// Up applies all pending transactional migrations. Files deliberately use the
// Goose annotation format, allowing the Goose CLI to inspect and operate the
// same migration history in production.
func Up(ctx context.Context) error {
	if _, err := g.DB().Exec(ctx, `CREATE TABLE IF NOT EXISTS goose_db_version (
        id BIGSERIAL PRIMARY KEY, version_id BIGINT NOT NULL, is_applied BOOLEAN NOT NULL,
        tstamp TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return fmt.Errorf("create migration history: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "sql")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		versionText := strings.SplitN(entry.Name(), "_", 2)[0]
		version, parseErr := strconv.ParseInt(versionText, 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid migration filename %s", entry.Name())
		}
		raw, readErr := migrationFiles.ReadFile("sql/" + entry.Name())
		if readErr != nil {
			return readErr
		}
		if err = apply(ctx, version, upSQL(string(raw))); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func upSQL(contents string) string {
	if index := strings.Index(contents, "-- +goose Down"); index >= 0 {
		contents = contents[:index]
	}
	lines := strings.Split(contents, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "-- +goose") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func apply(ctx context.Context, version int64, statement string) error {
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT pg_advisory_xact_lock(?)", migrationLockID); err != nil {
			return err
		}
		count, err := tx.Model("goose_db_version").Where("version_id", version).Where("is_applied", true).Count()
		if err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if strings.TrimSpace(statement) != "" {
			if _, err = tx.Exec(statement); err != nil {
				return err
			}
		}
		_, err = tx.Model("goose_db_version").Data(g.Map{"version_id": version, "is_applied": true}).Insert()
		return err
	})
}
