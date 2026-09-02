#!/usr/bin/env python3
"""
Job de retenção opcional - DESABILITADO por padrão.
Mantém histórico permanente. Só apaga se ENABLE_RETENTION=true em .env
"""
import os
import sys
import time
from pathlib import Path
from datetime import datetime, timezone

from dotenv import load_dotenv
load_dotenv(Path(__file__).resolve().parent.parent / ".env")

import psycopg
from monitor_pkg.config import require_database_url

ENABLE = os.getenv("ENABLE_RETENTION", "false").lower() in ("1","true","yes","on")
DRY = "--dry" in sys.argv
BATCH_LIMIT = int(os.getenv("RETENTION_BATCH_LIMIT", "50000"))
BATCH_SLEEP = float(os.getenv("RETENTION_BATCH_SLEEP", "0.1"))

# Retenção por tabela (pode ajustar via .env)
RETENTION = {
    "monitor.processes":    os.getenv("RETENTION_PROCESSES", "30 days"),
    "monitor.connections":  os.getenv("RETENTION_CONNECTIONS", "7 days"),
    "monitor.sensors":      os.getenv("RETENTION_SENSORS", "90 days"),
    "monitor.cpu":          os.getenv("RETENTION_CPU", "90 days"),
    "monitor.memory":       os.getenv("RETENTION_MEMORY", "90 days"),
    "monitor.gpu":          os.getenv("RETENTION_GPU", "90 days"),
    "monitor.heartbeat":    os.getenv("RETENTION_HEARTBEAT", "30 days"),
    "monitor.eventlog":     os.getenv("RETENTION_EVENTLOG", "30 days"),
    # disk, net, services, system_info mantidos 90d por padrão
    "monitor.disk_io":      os.getenv("RETENTION_DISK_IO", "90 days"),
    "monitor.net_io":       os.getenv("RETENTION_NET_IO", "90 days"),
}

def _delete_paginated(cur, table, interval):
    total = 0
    while True:
        cur.execute(f"DELETE FROM {table} WHERE ts < now() - interval %s AND ctid IN (SELECT ctid FROM {table} WHERE ts < now() - interval %s LIMIT %s)", (interval, interval, BATCH_LIMIT))
        deleted = cur.rowcount
        total += deleted
        if deleted < BATCH_LIMIT:
            break
        time.sleep(BATCH_SLEEP)
    return total

def run():
    print(f"[{datetime.now(timezone.utc).isoformat()}] Retention ENABLE={ENABLE} DRY={DRY} batch_limit={BATCH_LIMIT}")
    if not ENABLE:
        print("Retenção DESABILITADA - histórico permanente. Defina ENABLE_RETENTION=true para ativar.")
        return 0
    total_deleted = 0
    with psycopg.connect(require_database_url()) as conn:
        with conn.cursor() as cur:
            for table, interval in RETENTION.items():
                if DRY:
                    cur.execute(f"SELECT count(*) FROM {table} WHERE ts < now() - interval %s", (interval,))
                    cnt = cur.fetchone()[0]
                    print(f"[DRY] {table} intervalo {interval} -> deletaria {cnt} linhas")
                else:
                    cnt = _delete_paginated(cur, table, interval)
                    print(f"[DELETE] {table} intervalo {interval} -> {cnt} linhas")
                    total_deleted += cnt
        if not DRY:
            conn.commit()
    print(f"Total deletado: {total_deleted}")
    return total_deleted

if __name__ == "__main__":
    run()
