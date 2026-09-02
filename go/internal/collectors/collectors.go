// Package collectors produces PostgreSQL rows for each telemetry source,
// mirroring the Python collectors package. Each collector returns a Result
// with the target table, its columns, and the rows to insert.
package collectors

import "time"

// Result is a batch destined for a single monitor.* table.
type Result struct {
	Table   string
	Columns []string
	Rows    [][]any
}

// Func is a collector: hostname and timestamp in, rows out.
type Func func(hostname string, ts time.Time) (Result, error)

// ptr helpers return pointers for nullable columns.
func ptr[T any](v T) *T { return &v }
