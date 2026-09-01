import os
import psycopg
from pathlib import Path
from dotenv import load_dotenv

ROOT = Path(__file__).resolve().parent.parent
load_dotenv(ROOT / ".env")
DATABASE_URL = os.getenv("DATABASE_URL", "postgresql://postgres:22932293@localhost:5432/system_monitor")

def get_conn():
    return psycopg.connect(DATABASE_URL)

def q_cpu(window="1 hour"):
    sql = "SELECT ts, cpu_total_percent, freq_current_mhz FROM monitor.cpu WHERE ts > now() - %s::interval ORDER BY ts"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            rows = cur.fetchall()
            return [{"ts": r[0].isoformat(), "cpu": r[1], "freq": r[2]} for r in rows]

def q_memory(window="1 hour"):
    sql = "SELECT ts, used_percent, used_bytes/1024.0/1024/1024 as used_gb, swap_used_percent FROM monitor.memory WHERE ts > now() - %s::interval ORDER BY ts"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            rows = cur.fetchall()
            return [{"ts": r[0].isoformat(), "used_percent": r[1], "used_gb": float(r[2]) if r[2] else None, "swap": r[3]} for r in rows]

def q_gpu(window="1 hour"):
    sql = "SELECT ts, temperature_gpu_c, utilization_gpu_percent, power_draw_w, memory_used_bytes/1024.0/1024 as vram_mb FROM monitor.gpu WHERE ts > now() - %s::interval ORDER BY ts"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            rows = cur.fetchall()
            return [{"ts": r[0].isoformat(), "temp": r[1], "util": r[2], "power": r[3], "vram": float(r[4]) if r[4] else None} for r in rows]

def q_cpu_temps(window="1 hour"):
    sql = "SELECT ts, name, value FROM monitor.sensors WHERE sensor_type='temperature' AND (name ILIKE '%%cpu%%' OR name ILIKE '%%core%%' OR name ILIKE '%%tctl%%' OR name ILIKE '%%ccd%%') AND ts > now() - %s::interval AND value BETWEEN 0 AND 120 ORDER BY ts"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            rows = cur.fetchall()
            return [{"ts": r[0].isoformat(), "name": r[1], "value": r[2]} for r in rows]

def q_cpu_temps_latest():
    sql = "SELECT DISTINCT ON (name) name, value, unit, ts FROM monitor.sensors WHERE sensor_type='temperature' AND (name ILIKE '%%cpu%%' OR name ILIKE '%%core%%' OR name ILIKE '%%tctl%%' OR name ILIKE '%%ccd%%') AND value BETWEEN 10 AND 120 ORDER BY name, ts DESC"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            rows = cur.fetchall()
            return [{"name": r[0], "value": r[1], "unit": r[2], "ts": r[3].isoformat()} for r in rows]

def q_sensors_latest():
    sql = "SELECT DISTINCT ON (name) name, sensor_type, value, unit, ts FROM monitor.sensors WHERE value IS NOT NULL ORDER BY name, ts DESC"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            rows = cur.fetchall()
            return [{"name": r[0], "type": r[1], "value": r[2], "unit": r[3], "ts": r[4].isoformat()} for r in rows]

def q_disk_usage():
    sql = "SELECT DISTINCT ON (device) device, mountpoint, used_percent, free_bytes/1024.0/1024/1024 as free_gb, ts FROM monitor.disk_usage ORDER BY device, ts DESC"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            rows = cur.fetchall()
            return [{"device": r[0], "mount": r[1], "used_percent": r[2], "free_gb": float(r[3]) if r[3] else None, "ts": r[4].isoformat()} for r in rows]

def q_physical_disk():
    sql = "SELECT DISTINCT ON (device_id) device_id, friendly_name, model, media_type, bus_type, health_status, size_bytes/1024.0/1024/1024 as size_gb, ts FROM monitor.physical_disk ORDER BY device_id, ts DESC"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            rows = cur.fetchall()
            return [{"device_id": r[0], "friendly_name": r[1], "model": r[2], "media_type": r[3], "bus_type": r[4], "health": r[5], "size_gb": float(r[6]) if r[6] else None, "ts": r[7].isoformat()} for r in rows]

def q_disk_smart_latest():
    sql = "SELECT DISTINCT ON (device) device, model, temperature_c, power_on_hours, power_cycle_count, percentage_used, available_spare, media_errors, reallocated_sectors, pending_sectors, smart_passed, ts FROM monitor.disk_smart ORDER BY device, ts DESC"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            rows = cur.fetchall()
            return [{"device": r[0], "model": r[1], "temp": r[2], "poh": r[3], "pcycles": r[4], "wear": r[5], "spare": r[6], "media_err": r[7], "realloc": r[8], "pending": r[9], "passed": r[10], "ts": r[11].isoformat()} for r in rows]

def q_disk_io(window="1 hour"):
    sql = "SELECT ts, device, read_bytes, write_bytes FROM monitor.disk_io WHERE ts > now() - %s::interval ORDER BY device, ts"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            rows = cur.fetchall()
            return [{"ts": r[0].isoformat(), "device": r[1], "read": r[2], "write": r[3]} for r in rows]

def q_net(window="1 hour"):
    sql = "SELECT ts, iface, bytes_recv, bytes_sent FROM monitor.net_io WHERE ts > now() - %s::interval ORDER BY iface, ts"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            rows = cur.fetchall()
            return [{"ts": r[0].isoformat(), "iface": r[1], "recv": r[2], "sent": r[3]} for r in rows]

def q_net_latest():
    sql = "SELECT DISTINCT ON (iface) iface, bytes_recv, bytes_sent, ts FROM monitor.net_io ORDER BY iface, ts DESC"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            rows = cur.fetchall()
            return [{"iface": r[0], "recv": r[1], "sent": r[2], "ts": r[3].isoformat()} for r in rows]

def q_processes():
    sql = "SELECT name, pid, cpu_percent, memory_percent, memory_rss_bytes/1024.0/1024 as rss_mb, username FROM monitor.processes WHERE ts = (SELECT max(ts) FROM monitor.processes) ORDER BY cpu_percent DESC LIMIT 15"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            rows = cur.fetchall()
            return [{"name": r[0], "pid": r[1], "cpu": r[2], "mem": r[3], "rss_mb": float(r[4]) if r[4] else None, "user": r[5]} for r in rows]

def q_system():
    sql = "SELECT ts, hostname, uptime_seconds, cpu_name, os_build, total_ram_bytes/1024.0/1024/1024 as ram_gb FROM monitor.system_info ORDER BY ts DESC LIMIT 1"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            r = cur.fetchone()
            if not r:
                return None
            return {"ts": r[0].isoformat(), "hostname": r[1], "uptime": r[2], "cpu_name": r[3], "os_build": r[4], "ram_gb": float(r[5]) if r[5] else None}

def q_heartbeat():
    sql = "SELECT hostname, collector, ts, success, error FROM monitor.v_last_heartbeat ORDER BY collector"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            rows = cur.fetchall()
            return [{"host": r[0], "collector": r[1], "ts": r[2].isoformat(), "success": r[3], "error": r[4]} for r in rows]

def q_db_size():
    sql = "SELECT pg_size_pretty(pg_database_size('system_monitor')) as size, (SELECT count(*) FROM monitor.cpu) as cpu_rows, (SELECT count(*) FROM monitor.sensors) as sensor_rows"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            r = cur.fetchone()
            return {"size": r[0], "cpu_rows": r[1], "sensor_rows": r[2]}
