import logging
import re
from dataclasses import dataclass

import psycopg
from psycopg import sql
from psycopg.rows import dict_row

from monitor_pkg import config
from monitor_pkg.spool import BatchSpool

log = logging.getLogger(__name__)
IDENTIFIER = re.compile(r"^[a-z_][a-z0-9_]*$")
SPOOL = BatchSpool(config.BUFFER_PATH, config.SETTINGS.buffer_max_bytes)


@dataclass(frozen=True)
class WriteResult:
    rows: int
    status: str


def _validate(table: str, columns: list[str]) -> tuple[str, str]:
    try:
        schema, name = table.split(".", 1)
    except ValueError as exc:
        raise ValueError("table must be schema.name") from exc
    if not all(IDENTIFIER.fullmatch(value) for value in (schema, name, *columns)):
        raise ValueError("invalid SQL identifier")
    return schema, name


def get_conn():
    return psycopg.connect(config.DATABASE_URL, connect_timeout=config.SETTINGS.connect_timeout, autocommit=True, row_factory=dict_row)


def _insert_remote(table: str, columns: list[str], rows: list[tuple]) -> int:
    if not rows:
        return 0
    schema, name = _validate(table, columns)
    statement = sql.SQL("INSERT INTO {} ({}) VALUES ({})").format(
        sql.Identifier(schema, name),
        sql.SQL(",").join(map(sql.Identifier, columns)),
        sql.SQL(",").join(sql.Placeholder() for _ in columns),
    )
    with get_conn() as conn, conn.cursor() as cur:
        cur.executemany(statement, rows)
    return len(rows)


def insert_batch(table: str, columns: list[str], rows: list[tuple]) -> WriteResult:
    if not rows:
        return WriteResult(0, "empty")
    try:
        return WriteResult(_insert_remote(table, columns, rows), "stored")
    except (psycopg.OperationalError, psycopg.InterfaceError) as exc:
        SPOOL.enqueue(table, columns, rows)
        log.warning("database unavailable; buffered %s rows for %s: %s", len(rows), table, exc)
        return WriteResult(len(rows), "buffered")
    except ValueError as exc:
        log.warning("batch rejected for %s (%s rows, %s); data discarded", table, len(rows), exc)
        return WriteResult(0, "spool_overflow")


def replay_pending() -> int:
    try:
        return SPOOL.replay(_insert_remote)
    except (psycopg.OperationalError, psycopg.InterfaceError):
        return 0


def insert_heartbeat(hostname, collector, duration_ms, rows_inserted, success, error=None) -> WriteResult:
    return insert_batch("monitor.heartbeat", ["hostname", "collector", "duration_ms", "rows_inserted", "success", "error"], [(hostname, collector, duration_ms, rows_inserted, success, str(error)[:1000] if error else None)])


def ensure_schema():
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute("SELECT 1 FROM information_schema.schemata WHERE schema_name='monitor'")
        if not cur.fetchone():
            raise RuntimeError("schema monitor not found - run sql/schema.sql")
