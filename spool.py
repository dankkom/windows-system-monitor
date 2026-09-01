import json
import logging
import sqlite3
from datetime import date, datetime
from pathlib import Path
from typing import Callable

log = logging.getLogger(__name__)


def _json_default(value):
    if isinstance(value, (datetime, date)):
        return value.isoformat()
    raise TypeError(f"not JSON serializable: {type(value).__name__}")


class BatchSpool:
    def __init__(self, path: Path, max_bytes: int):
        self.path, self.max_bytes = path, max_bytes
        with self._connect() as conn:
            conn.execute("PRAGMA journal_mode=WAL")
            conn.execute("CREATE TABLE IF NOT EXISTS batches (id INTEGER PRIMARY KEY, table_name TEXT NOT NULL, columns_json TEXT NOT NULL, rows_json TEXT NOT NULL, size_bytes INTEGER NOT NULL)")

    def _connect(self):
        return sqlite3.connect(self.path, timeout=5)

    def enqueue(self, table: str, columns: list[str], rows: list[tuple]) -> int:
        payload = json.dumps(rows, default=_json_default, separators=(",", ":"))
        size = len(payload.encode("utf-8"))
        if size > self.max_bytes:
            raise ValueError(f"batch is {size} bytes, above buffer limit {self.max_bytes}")
        with self._connect() as conn:
            conn.execute("INSERT INTO batches(table_name, columns_json, rows_json, size_bytes) VALUES (?, ?, ?, ?)", (table, json.dumps(columns), payload, size))
            dropped = 0
            while conn.execute("SELECT COALESCE(SUM(size_bytes), 0) FROM batches").fetchone()[0] > self.max_bytes:
                conn.execute("DELETE FROM batches WHERE id = (SELECT id FROM batches ORDER BY id LIMIT 1)")
                dropped += 1
        if dropped:
            log.warning("buffer limit reached; discarded %s oldest batch(es)", dropped)
        return len(rows)

    def replay(self, writer: Callable[[str, list[str], list[tuple]], int], limit: int = 10) -> int:
        replayed = 0
        with self._connect() as conn:
            batches = conn.execute("SELECT id, table_name, columns_json, rows_json FROM batches ORDER BY id LIMIT ?", (limit,)).fetchall()
            for batch_id, table, columns, rows in batches:
                writer(table, json.loads(columns), [tuple(row) for row in json.loads(rows)])
                conn.execute("DELETE FROM batches WHERE id = ?", (batch_id,))
                replayed += 1
        return replayed

    def status(self) -> dict[str, int]:
        with self._connect() as conn:
            count, size = conn.execute("SELECT COUNT(*), COALESCE(SUM(size_bytes), 0) FROM batches").fetchone()
        return {"pending_batches": count, "pending_bytes": size, "max_bytes": self.max_bytes}
