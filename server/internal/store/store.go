// Package store owns the database: the pgx pool, the readiness ping and the
// sqlc-generated queries (make sqlc writes *.sql.go next to this file).
// Services call methods here; nothing else in the service sees a driver.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store holds the connection pool for the life of the process.
type Store struct {
	Pool *pgxpool.Pool
}

// Open parses the URL, builds the pool and pings once so a bad URL or an
// unreachable database fails at startup, not on the first request.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	s := &Store{Pool: pool}
	if err := s.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

// Name identifies the dependency on /readyz.
func (s *Store) Name() string { return "database" }

// Ping runs the readiness probe against the pool. Bounded so a hung database
// turns /readyz red instead of hanging it.
func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	return nil
}

// Close drains the pool. Call it after the HTTP server has stopped.
func (s *Store) Close() { s.Pool.Close() }

// AppliedMigrations reads which goose versions are applied now: for each
// version its latest row decides, since goose records a Down as a new row.
func (s *Store) AppliedMigrations(ctx context.Context) (map[int64]bool, error) {
	rows, err := s.Pool.Query(ctx, `SELECT DISTINCT ON (version_id) version_id, is_applied
		FROM goose_db_version ORDER BY version_id, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("read goose_db_version (has make migrate ever run?): %w", err)
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var v int64
		var applied bool
		if err := rows.Scan(&v, &applied); err != nil {
			return nil, fmt.Errorf("scan goose_db_version: %w", err)
		}
		out[v] = applied
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read goose_db_version: %w", err)
	}
	return out, nil
}
