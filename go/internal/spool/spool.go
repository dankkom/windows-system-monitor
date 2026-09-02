// Package spool implements a SQLite-backed buffer for batches that could not be
// written to PostgreSQL (DB unavailable). It mirrors the Python monitor_pkg.spool.
package spool

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// Spool buffers batches in a SQLite file with a total size cap (oldest evicted).
type Spool struct {
	path      string
	maxBytes  int64
	db        *sql.DB
}

// New opens (or creates) the spool database at path, enforcing maxBytes total.
func New(path string, maxBytes int64) (*Spool, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS batches (
			id INTEGER PRIMARY KEY,
			table_name TEXT NOT NULL,
			columns_json TEXT NOT NULL,
			rows_json TEXT NOT NULL,
			size_bytes INTEGER NOT NULL
		)`); err != nil {
		db.Close()
		return nil, err
	}
	return &Spool{path: path, maxBytes: maxBytes, db: db}, nil
}

// Close releases the underlying connection.
func (s *Spool) Close() error {
	return s.db.Close()
}

// rowToAny converts a batch row (tuple) into a []any suitable for re-insertion.
func rowToAny(row []any) []any {
	return row
}

// Enqueue appends a batch. It raises a wrapped error (caller treats as
// "spool overflow") when the single batch exceeds the cap, mirroring the
// Python ValueError raised by insert_batch.
func (s *Spool) Enqueue(table string, columns []string, rows [][]any) (int, error) {
	payload, err := json.Marshal(rows)
	if err != nil {
		return 0, err
	}
	size := int64(len(payload))
	if size > s.maxBytes {
		return 0, fmt.Errorf("batch is %d bytes, above buffer limit %d", size, s.maxBytes)
	}
	colsJSON, err := json.Marshal(columns)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO batches(table_name, columns_json, rows_json, size_bytes) VALUES (?, ?, ?, ?)`,
		table, string(colsJSON), string(payload), size); err != nil {
		return 0, err
	}
	dropped := 0
	var total int64
	if err := tx.QueryRow(`SELECT COALESCE(SUM(size_bytes), 0) FROM batches`).Scan(&total); err != nil {
		return 0, err
	}
	for total > s.maxBytes {
		var id int64
		if err := tx.QueryRow(`SELECT id FROM batches ORDER BY id LIMIT 1`).Scan(&id); err != nil {
			break
		}
		if _, err := tx.Exec(`DELETE FROM batches WHERE id = ?`, id); err != nil {
			break
		}
		total = 0
		_ = tx.QueryRow(`SELECT COALESCE(SUM(size_bytes), 0) FROM batches`).Scan(&total)
		dropped++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if dropped > 0 {
		log.Printf("buffer limit reached; discarded %d oldest batch(es)", dropped)
	}
	return len(rows), nil
}

// Replay writes up to limit oldest batches through writer, deleting each on
// success. Returns the number replayed.
func (s *Spool) Replay(writer func(table string, columns []string, rows [][]any) error, limit int) (int, error) {
	rows, err := s.db.Query(
		`SELECT id, table_name, columns_json, rows_json FROM batches ORDER BY id LIMIT ?`, limit)
	if err != nil {
		return 0, err
	}
	type entry struct {
		id     int64
		table  string
		cols   string
		rows   string
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.table, &e.cols, &e.rows); err != nil {
			rows.Close()
			return 0, err
		}
		entries = append(entries, e)
	}
	rows.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, e := range entries {
		var cols []string
		var raw [][]json.RawMessage
		if err := json.Unmarshal([]byte(e.cols), &cols); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(e.rows), &raw); err != nil {
			continue
		}
		anyRows := make([][]any, 0, len(raw))
		for _, r := range raw {
			conv := make([]any, 0, len(r))
			for _, cell := range r {
				conv = append(conv, decodeJSON(cell))
			}
			anyRows = append(anyRows, conv)
		}
		if err := writer(e.table, cols, anyRows); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM batches WHERE id = ?`, e.id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(entries), nil
}

// Status reports pending batch counts and bytes.
func (s *Spool) Status() (pendingBatches int, pendingBytes int64, err error) {
	err = s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(size_bytes), 0) FROM batches`).Scan(&pendingBatches, &pendingBytes)
	return
}

// decodeJSON converts a single JSON cell back to a native Go value, mirroring
// Python's json round-trip: integers come back as int64, decimals as float64,
// strings as string, bool as bool, and null as nil. Falls back to string.
func decodeJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return string(raw)
	}
	return normalizeJSON(v)
}

// normalizeJSON converts a json.Number and recursively normalizes values so
// integers keep integer type (like Python json.loads does).
func normalizeJSON(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t.String()
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = normalizeJSON(e)
		}
		return out
	default:
		return v
	}
}
