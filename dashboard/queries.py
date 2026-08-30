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
    sql = """
    SELECT ts, cpu_total_percent, freq_current_mhz
    FROM monitor.cpu
    WHERE ts > now() - %s::interval
    ORDER BY ts
    """
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            return cur.fetchall()

def q_memory(window="1 hour"):
    sql = "SELECT ts, used_percent, used_bytes/1024.0/1024/1024 as used_gb, swap_used_percent FROM monitor.memory WHERE ts > now() - %s::interval ORDER BY ts"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            return cur.fetchall()

def q_gpu(window="1 hour"):
    sql = "SELECT ts, temperature_gpu_c, utilization_gpu_percent, power_draw_w, memory_used_bytes/1024.0/1024 as vram_mb FROM monitor.gpu WHERE ts > now() - %s::interval ORDER BY ts"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            return cur.fetchall()

def q_sensors(window="1 hour", stype=None, limit=20):
    if stype and stype != "todos":
        sql = "SELECT ts, name, value, unit FROM monitor.sensors WHERE ts > now() - %s::interval AND sensor_type=%s AND value IS NOT NULL ORDER BY ts"
        params = (window, stype)
    else:
        sql = "SELECT ts, name, value, unit, sensor_type FROM monitor.sensors WHERE ts > now() - %s::interval AND value IS NOT NULL ORDER BY ts"
        params = (window,)
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, params)
            return cur.fetchall()

def q_cpu_temps(window="1 hour"):
    sql = """
    SELECT ts, name, value FROM monitor.sensors
    WHERE sensor_type='temperature' AND (name ILIKE '%%cpu%%' OR name ILIKE '%%core%%' OR name ILIKE '%%tctl%%' OR name ILIKE '%%ccd%%')
      AND ts > now() - %s::interval AND value BETWEEN 0 AND 120
    ORDER BY ts
    """
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            return cur.fetchall()

def q_cpu_temps_latest():
    sql = """
    SELECT DISTINCT ON (name) name, value, unit, ts
    FROM monitor.sensors
    WHERE sensor_type='temperature' AND (name ILIKE '%%cpu%%' OR name ILIKE '%%core%%' OR name ILIKE '%%tctl%%' OR name ILIKE '%%ccd%%') AND value BETWEEN 10 AND 120
    ORDER BY name, ts DESC
    """
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            return cur.fetchall()

def q_net_latest():
    sql = """
    SELECT DISTINCT ON (iface) iface, bytes_recv, bytes_sent, ts
    FROM monitor.net_io ORDER BY iface, ts DESC
    """
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            return cur.fetchall()

def q_sensors_latest():
    sql = """
    SELECT DISTINCT ON (name) name, sensor_type, value, unit, ts
    FROM monitor.sensors WHERE value IS NOT NULL ORDER BY name, ts DESC
    """
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            return cur.fetchall()

def q_physical_disk():
    sql = "SELECT DISTINCT ON (device_id) device_id, friendly_name, model, media_type, bus_type, health_status, size_bytes/1024.0/1024/1024 as size_gb, ts FROM monitor.physical_disk ORDER BY device_id, ts DESC"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            return cur.fetchall()

def q_disk_smart_latest():
    sql = """
    SELECT DISTINCT ON (device) device, model, temperature_c, power_on_hours, power_cycle_count, percentage_used, available_spare, media_errors, reallocated_sectors, pending_sectors, smart_passed, ts
    FROM monitor.disk_smart ORDER BY device, ts DESC
    """
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            return cur.fetchall()

def q_disk_smart_history(window="24 hours"):
    sql = """
    SELECT ts, device, temperature_c, power_on_hours, percentage_used FROM monitor.disk_smart
    WHERE ts > now() - %s::interval ORDER BY device, ts
    """
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            return cur.fetchall()

def q_disk_usage():
    sql = "SELECT DISTINCT ON (device) device, mountpoint, used_percent, free_bytes/1024.0/1024/1024 as free_gb, ts FROM monitor.disk_usage ORDER BY device, ts DESC"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            return cur.fetchall()

def q_disk_io(window="1 hour"):
    # delta de bytes por disco
    sql = """
    SELECT ts, device, read_bytes, write_bytes FROM monitor.disk_io
    WHERE ts > now() - %s::interval ORDER BY device, ts
    """
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            return cur.fetchall()

def q_net(window="1 hour"):
    sql = "SELECT ts, iface, bytes_recv, bytes_sent FROM monitor.net_io WHERE ts > now() - %s::interval ORDER BY iface, ts"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            return cur.fetchall()

def q_processes():
    sql = "SELECT name, pid, cpu_percent, memory_percent, memory_rss_bytes/1024.0/1024 as rss_mb, username FROM monitor.processes WHERE ts = (SELECT max(ts) FROM monitor.processes) ORDER BY cpu_percent DESC LIMIT 15"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            return cur.fetchall()

def q_system():
    sql = "SELECT ts, hostname, uptime_seconds, cpu_name, os_build, total_ram_bytes/1024.0/1024/1024 as ram_gb FROM monitor.system_info ORDER BY ts DESC LIMIT 1"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            row = cur.fetchone()
            return row

def q_eventlog(window="1 hour"):
    sql = "SELECT log_name, level, provider, event_id, count, latest_message FROM monitor.eventlog WHERE ts > now() - %s::interval ORDER BY ts DESC LIMIT 20"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql, (window,))
            return cur.fetchall()

def q_heartbeat():
    sql = "SELECT hostname, collector, ts, success, error FROM monitor.v_last_heartbeat ORDER BY collector"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            return cur.fetchall()

def q_db_size():
    sql = "SELECT pg_size_pretty(pg_database_size('system_monitor')) as size, (SELECT count(*) FROM monitor.cpu) as cpu_rows, (SELECT count(*) FROM monitor.sensors) as sensor_rows"
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(sql)
            return cur.fetchone()
