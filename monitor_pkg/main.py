#!/usr/bin/env python3
"""
Monitor contínuo Windows -> PostgreSQL system_monitor
Coleta máxima de dados com intervalos configuráveis, batch inserts e resiliência.
"""
import time
import logging
import signal
import sys
from pathlib import Path
from logging.handlers import RotatingFileHandler

# Garante que a raiz do projeto esteja no sys.path ao rodar via task
# (python monitor_pkg\main.py a partir do WorkingDirectory), pois o Python
# só adiciona o diretório do script, não o CWD.
_RUN_ROOT = Path(__file__).resolve().parent.parent
if str(_RUN_ROOT) not in sys.path:
    sys.path.insert(0, str(_RUN_ROOT))

import psycopg

from monitor_pkg import config
from monitor_pkg.db import insert_batch, insert_heartbeat, ensure_schema, replay_pending

# collectors
from collectors import (
    collect_cpu, collect_memory,
    collect_disk_io, collect_disk_usage,
    collect_net_io, collect_net_addrs,
    collect_gpu, collect_sensors, collect_processes,
    collect_connections, collect_services,
    collect_system, collect_eventlog,
    collect_physical, collect_smart,
)

# Setup logging
log = logging.getLogger()
log.setLevel(config.SETTINGS.log_level)
formatter = logging.Formatter("%(asctime)s | %(levelname)s | %(name)s | %(message)s")

ch = logging.StreamHandler()
ch.setFormatter(formatter)
log.addHandler(ch)

fh = RotatingFileHandler(config.LOG_DIR / "monitor.log", maxBytes=10*1024*1024, backupCount=5, encoding="utf-8")
fh.setFormatter(formatter)
log.addHandler(fh)

efh = RotatingFileHandler(config.LOG_DIR / "monitor_error.log", maxBytes=5*1024*1024, backupCount=3, encoding="utf-8")
efh.setLevel(logging.WARNING)
efh.setFormatter(formatter)
log.addHandler(efh)

logger = logging.getLogger("monitor")

HOSTNAME = config.HOSTNAME
running = True

def signal_handler(sig, frame):
    global running
    logger.info(f"Signal {sig} received, shutting down...")
    running = False

signal.signal(signal.SIGINT, signal_handler)
signal.signal(signal.SIGTERM, signal_handler)
try:
    signal.signal(signal.SIGBREAK, signal_handler)
except AttributeError:
    pass

COLLECTORS = {
    "cpu":          (collect_cpu,                                           "monitor.cpu",          config.INTERVALS["cpu"]),
    "memory":       (collect_memory,                                        "monitor.memory",        config.INTERVALS["memory"]),
    "disk_io":      (collect_disk_io,                                       "monitor.disk_io",       config.INTERVALS["disk_io"]),
    "disk_usage":   (collect_disk_usage,                                    "monitor.disk_usage",    config.INTERVALS["disk_usage"]),
    "disk_physical":(collect_physical,                                      "monitor.physical_disk", config.INTERVALS["disk_physical"]),
    "disk_smart":   (collect_smart,                                         "monitor.disk_smart",    config.INTERVALS["disk_smart"]),
    "net_io":       (collect_net_io,                                        "monitor.net_io",        config.INTERVALS["network"]),
    "net_addr":     (collect_net_addrs,                                     "monitor.net_addr",      config.INTERVALS["system"]),
    "gpu":          (collect_gpu,                                           "monitor.gpu",           config.INTERVALS["gpu"]),
    "sensors":      (collect_sensors,                                       "monitor.sensors",       config.INTERVALS["sensors"]),
    "processes":    (lambda h: collect_processes(h, top_n=config.TOP_PROCESSES), "monitor.processes", config.INTERVALS["processes"]),
    "connections":  (collect_connections,                                   "monitor.connections",   config.INTERVALS["connections"]),
    "services":     (collect_services,                                      "monitor.services",      config.INTERVALS["services"]),
    "system":       (collect_system,                                        "monitor.system_info",   config.INTERVALS["system"]),
    "eventlog":     (collect_eventlog,                                      "monitor.eventlog",      config.INTERVALS["eventlog"]),
}

def run_collector(name, func, table, interval_state):
    start = time.time()
    try:
        cols, rows = func(HOSTNAME)
        if rows:
            result = insert_batch(table, cols, rows)
            n, status = result.rows, result.status
        else:
            n, status = 0, "empty"
        duration = (time.time() - start) * 1000
        logger.info("[%s] %s rows -> %s (%.0fms, %s)", name, n, table, duration, status)
        insert_heartbeat(HOSTNAME, name, duration, n, status == "stored", None if status == "stored" else status)
        return status
    except Exception as e:
        duration = (time.time() - start) * 1000
        logger.exception("[%s] failed", name)
        try:
            insert_heartbeat(HOSTNAME, name, duration, 0, False, e)
        except (OSError, RuntimeError) as heartbeat_error:
            logger.warning("heartbeat unavailable: %s", heartbeat_error)
        return "failed"

def main(once=False, dry_run=False):
    global running
    logger.info(f"Starting monitor hostname={HOSTNAME} intervals={config.INTERVALS} dry_run={dry_run} once={once}")
    try:
        if not dry_run:
            ensure_schema()
            logger.info("Schema OK")
    except (psycopg.Error, RuntimeError) as e:
        logger.exception("Schema check failed: %s", e)
        if not dry_run:
            raise

    if dry_run:
        # testa todos coletores sem inserir
        for name, (func, table, interval) in COLLECTORS.items():
            try:
                cols, rows = func(HOSTNAME)
                logger.info(f"[DRY {name}] cols={cols[:3]}... rows={len(rows)} sample={str(rows[0])[:300] if rows else 'empty'}")
            except Exception as e:
                logger.error(f"[DRY {name}] error: {e}", exc_info=True)
        return

    if once:
        for name, (func, table, interval) in COLLECTORS.items():
            run_collector(name, func, table, None)
        logger.info("Once run complete")
        return

    # loop contínuo: verifica cada segundo se intervalo expirou
    last_run = {name: 0 for name in COLLECTORS}
    # warmup psutil cpu_percent
    try:
        import psutil
        psutil.cpu_percent(interval=None)
        psutil.cpu_percent(interval=None, percpu=True)
    except ImportError:
        pass
    logger.info("Entering continuous loop (Ctrl+C to stop)")
    while running:
        replayed = replay_pending()
        if replayed:
            logger.info("replayed %s buffered batches", replayed)
        now = time.time()
        for name, (func, table, interval) in COLLECTORS.items():
            if now - last_run[name] >= interval:
                run_collector(name, func, table, None)
                last_run[name] = now
                if not running:
                    break
        # sleep 1s mas respeita shutdown
        for _ in range(10):
            if not running:
                break
            time.sleep(0.1)

    logger.info("Monitor stopped gracefully")

if __name__ == "__main__":
    import argparse
    p = argparse.ArgumentParser(description="Windows System Monitor -> PostgreSQL")
    p.add_argument("--once", action="store_true", help="roda uma coleta de cada e sai")
    p.add_argument("--dry-run", action="store_true", help="não insere no banco, só testa coletores")
    p.add_argument("--interval", type=int, default=None, help="override intervalo base (s) para teste")
    args = p.parse_args()
    if args.interval:
        for k in config.INTERVALS:
            config.INTERVALS[k] = args.interval
    try:
        main(once=args.once, dry_run=args.dry_run)
    except KeyboardInterrupt:
        logger.info("KeyboardInterrupt")
        sys.exit(0)
