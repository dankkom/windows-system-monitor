#!/usr/bin/env python3
"""
Job de retenção opcional - DESABILITADO por padrão.
Mantém histórico permanente. Só apaga se ENABLE_RETENTION=true em .env
"""
import os
import sys
from pathlib import Path
from datetime import datetime, timezone

# Ensure project root in path
ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

from dotenv import load_dotenv
load_dotenv(ROOT / ".env")

import psycopg
from config import DATABASE_URL

ENABLE = os.getenv("ENABLE_RETENTION", "false").lower() in ("1","true","yes","on")
DRY = "--dry" in sys.argv

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

def run():
    print(f"[{datetime.now(timezone.utc).isoformat()}] Retention ENABLE={ENABLE} DRY={DRY}")
    if not ENABLE:
        print("Retenção DESABILITADA - histórico permanente. Defina ENABLE_RETENTION=true para ativar.")
        return 0
    total_deleted = 0
    with psycopg.connect(DATABASE_URL) as conn:
        with conn.cursor() as cur:
            for table, interval in RETENTION.items():
                sql = f"DELETE FROM {table} WHERE ts < now() - interval %s"
                if DRY:
                    cur.execute(f"SELECT count(*) FROM {table} WHERE ts < now() - interval %s", (interval,))
                    cnt = cur.fetchone()[0]
                    print(f"[DRY] {table} intervalo {interval} -> deletaria {cnt} linhas")
                else:
                    cur.execute(sql, (interval,))
                    cnt = cur.rowcount
                    print(f"[DELETE] {table} intervalo {interval} -> {cnt} linhas")
                    total_deleted += cnt
        if not DRY:
            conn.commit()
    print(f"Total deletado: {total_deleted}")
    return total_deleted

if __name__ == "__main__":
    run()
