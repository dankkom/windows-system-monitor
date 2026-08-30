import json
import logging
import psycopg
from psycopg.rows import dict_row
from config import DATABASE_URL

log = logging.getLogger(__name__)

def get_conn():
    return psycopg.connect(DATABASE_URL, autocommit=True, row_factory=dict_row)

def insert_batch(table: str, columns: list[str], rows: list[tuple]):
    if not rows:
        return 0
    placeholders = ",".join(["%s"] * len(columns))
    cols = ",".join(columns)
    sql = f"INSERT INTO {table} ({cols}) VALUES ({placeholders})"
    try:
        with get_conn() as conn:
            with conn.cursor() as cur:
                cur.executemany(sql, rows)
        return len(rows)
    except Exception as e:
        log.error(f"insert_batch {table} failed: {e}")
        raise

def insert_heartbeat(hostname, collector, duration_ms, rows_inserted, success, error=None):
    try:
        insert_batch("monitor.heartbeat",
            ["hostname","collector","duration_ms","rows_inserted","success","error"],
            [(hostname, collector, duration_ms, rows_inserted, success, str(error)[:1000] if error else None)]
        )
    except Exception as e:
        log.warning(f"heartbeat insert failed: {e}")

def ensure_schema():
    # schema already created via sql/schema.sql, just ensure connection works
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT 1 FROM information_schema.schemata WHERE schema_name='monitor'")
            if not cur.fetchone():
                raise RuntimeError("schema monitor not found - run sql/schema.sql")
