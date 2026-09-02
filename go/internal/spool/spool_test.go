package spool

import (
	"path/filepath"
	"testing"
)

func newSpool(t *testing.T, maxBytes int64) *Spool {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.sqlite3"), maxBytes)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func replayCollect(t *testing.T, s *Spool, limit int) [][]any {
	t.Helper()
	acc := [][]any{}
	n, err := s.Replay(func(_ string, _ []string, rows [][]any) error {
		acc = append(acc, rows...)
		return nil
	}, limit)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if n != len(acc) {
		t.Fatalf("replay count %d != collected %d", n, len(acc))
	}
	return acc
}

func TestReplayPreservesBatchOrder(t *testing.T) {
	s := newSpool(t, 100_000)
	if _, err := s.Enqueue("monitor.cpu", []string{"value"}, [][]any{{1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enqueue("monitor.cpu", []string{"value"}, [][]any{{2}}); err != nil {
		t.Fatal(err)
	}
	got := replayCollect(t, s, 10)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d: %v", len(got), got)
	}
	if got[0][0] != int64(1) || got[1][0] != int64(2) {
		t.Fatalf("order/values wrong: %v", got)
	}
	b, size, err := s.Status()
	if err != nil || b != 0 || size != 0 {
		t.Fatalf("status after replay: batches=%d bytes=%d err=%v", b, size, err)
	}
}

func TestCapacityDiscardsOldestBatch(t *testing.T) {
	s := newSpool(t, 30)
	if _, err := s.Enqueue("monitor.cpu", []string{"value"}, [][]any{{"first payload"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enqueue("monitor.cpu", []string{"value"}, [][]any{{"second payload"}}); err != nil {
		t.Fatal(err)
	}
	got := replayCollect(t, s, 10)
	if len(got) != 1 || got[0][0] != "second payload" {
		t.Fatalf("expected only newest batch, got %v", got)
	}
}

func TestRejectsBatchLargerThanLimit(t *testing.T) {
	s := newSpool(t, 10)
	if _, err := s.Enqueue("monitor.cpu", []string{"value"}, [][]any{{"payload larger than ten bytes"}}); err == nil {
		t.Fatal("expected overflow error")
	}
}
