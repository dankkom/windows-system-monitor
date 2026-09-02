// Package db provides PostgreSQL batch insertion with a SQLite spool fallback,
// mirroring monitor_pkg.db.
package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dankkom/windows-system-monitor/go/internal/config"
	"github.com/dankkom/windows-system-monitor/go/internal/spool"
)

var identifierRE = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// WriteResult mirrors the Python WriteResult dataclass.
type WriteResult struct {
	Rows   int
	Status string
}

// Store holds a connection pool and the write-through spool.
type Store struct {
	cfg   *config.Settings
	pool  *pgxpool.Pool
	spool *spool.Spool
}

// New opens the spool for the given settings (the pgx pool is created lazily).
func New(cfg *config.Settings) (*Store, error) {
	spl, err := spool.New(cfg.BufferPath, cfg.BufferMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("open spool: %w", err)
	}
	return &Store{cfg: cfg, spool: spl}, nil
}

// Close releases the pool and spool.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
	if s.spool != nil {
		_ = s.spool.Close()
	}
}

func (s *Store) poolRef(ctx context.Context) (*pgxpool.Pool, error) {
	if s.pool == nil {
		cfg, err := pgxpool.ParseConfig(storeConfigURL(s.cfg))
		if err != nil {
			return nil, err
		}
		cfg.ConnConfig.ConnectTimeout = s.cfg.ConnectTimeout
		p, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			return nil, err
		}
		s.pool = p
	}
	return s.pool, nil
}

// validate ensures table is schema.name with safe identifiers; mirrors
// _validate.
func validate(table string, columns []string) error {
	schema, name, ok := cutSchemaTable(table)
	if !ok {
		return fmt.Errorf("table must be schema.name: %s", table)
	}
	if !identifierRE.MatchString(schema) || !identifierRE.MatchString(name) {
		return fmt.Errorf("invalid SQL identifier: %s", table)
	}
	for _, c := range columns {
		if !identifierRE.MatchString(c) {
			return fmt.Errorf("invalid SQL identifier: %s", c)
		}
	}
	return nil
}

func cutSchemaTable(table string) (schema, name string, ok bool) {
	if i := strings.IndexByte(table, '.'); i >= 0 {
		return table[:i], table[i+1:], true
	}
	return "", "", false
}

func storeConfigURL(cfg *config.Settings) string {
	u, err := cfg.RequireDatabaseURL()
	if err != nil {
		return ""
	}
	return u
}

// insertRemote performs the actual multi-row INSERT, mirroring _insert_remote.
// Caller must have validated the table/columns.
func (s *Store) insertRemote(ctx context.Context, table string, columns []string, rows [][]any) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	schema, name, _ := cutSchemaTable(table)
	pool, err := s.poolRef(ctx)
	if err != nil {
		return 0, err
	}
	colNames := make([]string, len(columns))
	copy(colNames, columns)
	n, err := pool.CopyFrom(ctx, pgx.Identifier{schema, name}, colNames, pgx.CopyFromRows(rows))
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// InsertBatch buffers to spool when the DB is unavailable, mirroring
// insert_batch. Status is one of: stored, buffered, spool_overflow, empty.
func (s *Store) InsertBatch(ctx context.Context, table string, columns []string, rows [][]any) WriteResult {
	if len(rows) == 0 {
		return WriteResult{0, "empty"}
	}
	if err := validate(table, columns); err != nil {
		log.Printf("batch rejected for %s (%d rows); data discarded: %v", table, len(rows), err)
		return WriteResult{0, "spool_overflow"}
	}
	n, err := s.insertRemote(ctx, table, columns, rows)
	if err == nil {
		return WriteResult{n, "stored"}
	}
	if isConnectionError(err) {
		if _, serr := s.spool.Enqueue(table, columns, rows); serr != nil {
			log.Printf("batch rejected for %s (%d rows); data discarded: %v", table, len(rows), serr)
			return WriteResult{0, "spool_overflow"}
		}
		log.Printf("database unavailable; buffered %d rows for %s: %v", len(rows), table, err)
		return WriteResult{len(rows), "buffered"}
	}
	// A server-side SQL error (e.g. column mismatch) is not treated as a
	// connection failure; buffer it too so data is not silently lost, but only
	// if the spool accepts it.
	if _, serr := s.spool.Enqueue(table, columns, rows); serr != nil {
		log.Printf("batch rejected for %s (%d rows); data discarded: %v", table, len(rows), serr)
		return WriteResult{0, "spool_overflow"}
	}
	log.Printf("insert failed (non-connection); buffered %d rows for %s: %v", len(rows), table, err)
	return WriteResult{len(rows), "buffered"}
}

// isConnectionError reports whether err represents a transient connection /
// pool problem that warrants buffering (vs. a local validation failure which
// the caller already handled).
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// A PgError is a server-reported SQL error, not a connection problem.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return false
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	// Everything else (connect failures, pool acquisition errors, network
	// timeouts) is treated as transient.
	return true
}

// ReplayPending writes buffered batches to the DB, returning count replayed.
func (s *Store) ReplayPending(ctx context.Context) (int, error) {
	return s.spool.Replay(func(table string, columns []string, rows [][]any) error {
		if err := validate(table, columns); err != nil {
			return nil
		}
		_, err := s.insertRemote(ctx, table, columns, rows)
		return err
	}, 10)
}

// InsertHeartbeat records a collector heartbeat row.
func (s *Store) InsertHeartbeat(ctx context.Context, hostname, collector string, durationMS int, rowsInserted int, success bool, errText string) WriteResult {
	if len(errText) > 1000 {
		errText = errText[:1000]
	}
	row := []any{hostname, collector, durationMS, rowsInserted, success, nil}
	if errText != "" {
		row[5] = errText
	}
	return s.InsertBatch(ctx, "monitor.heartbeat",
		[]string{"hostname", "collector", "duration_ms", "rows_inserted", "success", "error"},
		[][]any{row})
}

// EnsureSchema verifies the monitor schema exists.
func (s *Store) EnsureSchema(ctx context.Context) error {
	pool, err := s.poolRef(ctx)
	if err != nil {
		return err
	}
	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name='monitor')`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("schema monitor not found - run sql/schema.sql")
	}
	return nil
}
