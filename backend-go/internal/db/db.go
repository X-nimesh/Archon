// Package db owns the PostgreSQL connection pool and migration runner.
package db

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/archon/backend-go/migrations"
)

// Pool is the application-wide connection pool.
type Pool = pgxpool.Pool

// migrationDir is the prefix inside the embedded migrations FS.
const migrationDir = "."

// Connect opens a pool and blocks until Postgres answers a ping, so the server
// never starts accepting requests against a database that is not ready yet.
func Connect(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// migration is one versioned pair of up/down scripts.
type migration struct {
	version int
	name    string
	up      string
	down    string
}

// migrationAdvisoryLockID serialises migration runs across processes.
//
// `CREATE EXTENSION IF NOT EXISTS` is not atomic — two instances booting at the
// same time can both pass the existence check and then collide on the
// pg_extension unique index. A session-level advisory lock makes the whole
// check-then-create sequence exclusive, so only one instance migrates at a time.
// The value is arbitrary but must be stable and project-specific.
const migrationAdvisoryLockID int64 = 8_240_117_553_001

// Migrate applies every migration that has not run yet, in version order.
//
// Each migration runs inside its own transaction together with the bookkeeping
// insert, so a failure leaves neither a half-applied schema nor a recorded
// version. A checksum mismatch on an already-applied migration is reported as an
// error rather than silently ignored: editing a shipped migration breaks every
// environment that already ran it.
func Migrate(ctx context.Context, pool *Pool) (int, error) {
	// The lock is session-scoped, so it must be taken and released on one
	// dedicated connection.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationAdvisoryLockID); err != nil {
		return 0, fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		// Best effort: the lock is released automatically when the session ends.
		_, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationAdvisoryLockID)
	}()

	all, err := loadMigrations()
	if err != nil {
		return 0, err
	}

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     INTEGER PRIMARY KEY,
			name        TEXT        NOT NULL,
			checksum    TEXT        NOT NULL,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return 0, fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return 0, err
	}

	count := 0

	for _, m := range all {
		if existing, ok := applied[m.version]; ok {
			if existing != checksum(m.up) {
				return count, fmt.Errorf(
					"migration %04d_%s was modified after being applied; "+
						"write a new migration instead of editing an applied one",
					m.version, m.name)
			}
			continue
		}

		if err := applyMigration(ctx, conn, m); err != nil {
			return count, err
		}

		count++
	}

	return count, nil
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, m migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", m.version, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful commit

	if _, err := tx.Exec(ctx, m.up); err != nil {
		return fmt.Errorf("apply migration %d_%s: %w", m.version, m.name, err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`,
		m.version, m.name, checksum(m.up)); err != nil {
		return fmt.Errorf("record migration %d: %w", m.version, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %d: %w", m.version, err)
	}

	return nil
}

func appliedVersions(ctx context.Context, conn *pgxpool.Conn) (map[int]string, error) {
	rows, err := conn.Query(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]string)
	for rows.Next() {
		var version int
		var sum string
		if err := rows.Scan(&version, &sum); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[version] = sum
	}

	return applied, rows.Err()
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, migrationDir)
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	var out []migration

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		base := strings.TrimSuffix(name, ".up.sql")
		version, migName, ok := parseVersion(base)
		if !ok {
			return nil, fmt.Errorf("migration %q: want NNNN_name.sql", name)
		}

		// path.Join normalises away the leading "./" that fs paths reject.
		up, err := migrations.FS.ReadFile(path.Join(migrationDir, name))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}

		downName := base + ".down.sql"
		down, err := migrations.FS.ReadFile(path.Join(migrationDir, downName))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w (every up migration needs a matching down)", downName, err)
		}

		out = append(out, migration{
			version: version,
			name:    migName,
			up:      string(up),
			down:    string(down),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })

	return out, nil
}

// parseVersion splits "0001_create_users" into (1, "create_users").
func parseVersion(base string) (int, string, bool) {
	prefix, rest, found := strings.Cut(base, "_")
	if !found {
		return 0, "", false
	}

	var version int
	if _, err := fmt.Sscanf(prefix, "%d", &version); err != nil || version <= 0 {
		return 0, "", false
	}

	return version, rest, true
}

// checksum is intentionally simple: this guards against accidental local edits,
// not against a determined attacker with repo access.
func checksum(sql string) string {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(sql); i++ {
		h ^= uint64(sql[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}

// IsUniqueViolation reports whether err is a Postgres 23505.
