package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationFiles is embedded into the binary so migrations do not depend on
// the application's working directory at runtime.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version int64
	name    string
	upSQL   string
	upHash  string
	downSQL string
}

// Migrate applies every pending migration in version order. Each migration is
// executed in its own transaction. A PostgreSQL advisory lock prevents two
// application instances from migrating the same database concurrently.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("database pool is nil")
	}

	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	const lockKey int64 = 706918241
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, lockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockKey)
	}()

	// Keep this bootstrap idempotent. It allows the application to start from
	// an empty database while the first migration remains the canonical schema
	// definition for fresh environments.
	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			checksum TEXT NOT NULL DEFAULT ''
		)
	`); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	applied, err := appliedMigrations(ctx, conn)
	if err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}

	for _, m := range migrations {
		storedHash, ok := applied[m.version]
		if ok {
			if storedHash != m.upHash {
				return fmt.Errorf("migration %d (%s) has been modified after being applied", m.version, m.name)
			}
			continue
		}

		if err := applyMigration(ctx, conn, m); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, m migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migration %d (%s): begin transaction: %w", m.version, m.name, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, m.upSQL); err != nil {
		return fmt.Errorf("migration %d (%s): execute: %w", m.version, m.name, err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)`,
		m.version, m.upHash,
	); err != nil {
		return fmt.Errorf("migration %d (%s): record: %w", m.version, m.name, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migration %d (%s): commit: %w", m.version, m.name, err)
	}
	return nil
}

func appliedMigrations(ctx context.Context, conn *pgxpool.Conn) (map[int64]string, error) {
	rows, err := conn.Query(ctx, `SELECT version, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]string)
	for rows.Next() {
		var version int64
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, err
		}
		result[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// loadMigrations validates and loads all embedded migration files.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}

	byVersion := make(map[int64]*migration)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		version, name, err := parseMigrationName(entry.Name())
		if err != nil {
			return nil, err
		}
		if _, exists := byVersion[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d", version)
		}

		upPath := path.Join("migrations", entry.Name())
		downPath := path.Join("migrations", fmt.Sprintf("%06d_%s.down.sql", version, name))
		up, err := fs.ReadFile(migrationFiles, upPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		down, err := fs.ReadFile(migrationFiles, downPath)
		if err != nil {
			return nil, fmt.Errorf("read down migration for %s: %w", entry.Name(), err)
		}

		hash := sha256.Sum256(up)
		byVersion[version] = &migration{
			version: version,
			name:    name,
			upSQL:   string(up),
			upHash:  hex.EncodeToString(hash[:]),
			downSQL: string(down),
		}
	}

	versions := make([]int64, 0, len(byVersion))
	for version := range byVersion {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })

	result := make([]migration, 0, len(versions))
	for _, version := range versions {
		result = append(result, *byVersion[version])
	}
	return result, nil
}

func parseMigrationName(filename string) (int64, string, error) {
	base := strings.TrimSuffix(filename, ".up.sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, "", fmt.Errorf("invalid migration filename %q", filename)
	}

	version, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || version <= 0 {
		return 0, "", fmt.Errorf("invalid migration version in %q", filename)
	}
	return version, parts[1], nil
}
