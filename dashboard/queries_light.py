from __future__ import annotations

from collections import Counter, defaultdict
from datetime import datetime
from zoneinfo import ZoneInfo

import psycopg

from monitor_pkg.config import INTERVALS, SETTINGS, require_database_url


def get_conn():
    return psycopg.connect(require_database_url(), connect_timeout=SETTINGS.connect_timeout)


def _bucket_expr(column: str = "ts") -> str:
    return f"to_timestamp(floor(extract(epoch FROM {column}) / %s) * %s)"


def _iso(value: datetime) -> str:
    return value.isoformat()


CPU_SENSOR_PATTERNS = ["%cpu%", "%core%", "%tctl%", "%ccd%"]
CPU_SENSOR_WHERE = "(" + " OR ".join("name ILIKE %s" for _ in CPU_SENSOR_PATTERNS) + ")"


def q_cpu(window="1 hour", bucket_seconds=10):
    bucket = _bucket_expr()
    sql = f"""SELECT {bucket} AS bucket, avg(cpu_total_percent), avg(freq_current_mhz)
        FROM monitor.cpu WHERE ts > now() - %s::interval GROUP BY bucket ORDER BY bucket"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql, (bucket_seconds, bucket_seconds, window))
        return [{"ts": _iso(r[0]), "cpu": r[1], "freq": r[2]} for r in cur.fetchall()]


def q_memory(window="1 hour", bucket_seconds=10):
    bucket = _bucket_expr()
    sql = f"""SELECT {bucket} AS bucket, avg(used_percent), avg(used_bytes)/1073741824.0,
               avg(swap_used_percent)
        FROM monitor.memory WHERE ts > now() - %s::interval GROUP BY bucket ORDER BY bucket"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql, (bucket_seconds, bucket_seconds, window))
        return [
            {"ts": _iso(r[0]), "used_percent": r[1], "used_gb": float(r[2]) if r[2] is not None else None, "swap": r[3]}
            for r in cur.fetchall()
        ]


def q_gpu(window="1 hour", bucket_seconds=10):
    bucket = _bucket_expr()
    sql = f"""SELECT {bucket} AS bucket, avg(temperature_gpu_c), avg(utilization_gpu_percent),
               avg(power_draw_w), avg(memory_used_bytes)/1048576.0
        FROM monitor.gpu WHERE ts > now() - %s::interval GROUP BY bucket ORDER BY bucket"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql, (bucket_seconds, bucket_seconds, window))
        return [
            {"ts": _iso(r[0]), "temp": r[1], "util": r[2], "power": r[3], "vram": float(r[4]) if r[4] is not None else None}
            for r in cur.fetchall()
        ]


def q_cpu_temps(window="1 hour", bucket_seconds=10):
    bucket = _bucket_expr()
    sql = f"""SELECT {bucket} AS bucket, name, avg(value)
        FROM monitor.sensors
        WHERE sensor_type='temperature'
          AND {CPU_SENSOR_WHERE}
          AND ts > now() - %s::interval AND value BETWEEN 0 AND 120
        GROUP BY bucket, name ORDER BY bucket, name"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql, (bucket_seconds, bucket_seconds, *CPU_SENSOR_PATTERNS, window))
        return [{"ts": _iso(r[0]), "name": r[1], "value": r[2]} for r in cur.fetchall()]


def q_cpu_temps_latest():
    sql = f"""SELECT DISTINCT ON (name) name, value, unit, ts FROM monitor.sensors
        WHERE sensor_type='temperature'
          AND {CPU_SENSOR_WHERE}
          AND value BETWEEN 10 AND 120 ORDER BY name, ts DESC"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql, (*CPU_SENSOR_PATTERNS,))
        return [{"name": r[0], "value": r[1], "unit": r[2], "ts": _iso(r[3])} for r in cur.fetchall()]


def q_sensors_latest():
    sql = """SELECT DISTINCT ON (sensor_type, name) name, sensor_type, value, unit, ts
        FROM monitor.sensors WHERE value IS NOT NULL ORDER BY sensor_type, name, ts DESC"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql)
        return [{"name": r[0], "type": r[1], "value": r[2], "unit": r[3], "ts": _iso(r[4])} for r in cur.fetchall()]


def q_sensors_history(window="1 hour", bucket_seconds=10, sensor_type="power"):
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(
            """SELECT name FROM monitor.sensors WHERE sensor_type=%s AND value IS NOT NULL
               GROUP BY name ORDER BY max(ts) DESC LIMIT 8""",
            (sensor_type,),
        )
        names = [r[0] for r in cur.fetchall()]
        if not names:
            return []
        bucket = _bucket_expr()
        sql = f"""SELECT {bucket} AS bucket, name, avg(value), max(unit)
            FROM monitor.sensors WHERE sensor_type=%s AND name = ANY(%s)
              AND value IS NOT NULL AND ts > now() - %s::interval
            GROUP BY bucket, name ORDER BY bucket, name"""
        cur.execute(sql, (bucket_seconds, bucket_seconds, sensor_type, names, window))
        return [{"ts": _iso(r[0]), "name": r[1], "value": r[2], "unit": r[3]} for r in cur.fetchall()]


def q_disk_usage():
    sql = """SELECT DISTINCT ON (device) device, mountpoint, total_bytes, used_bytes, free_bytes,
             used_percent, ts FROM monitor.disk_usage ORDER BY device, ts DESC"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql)
        return [
            {"device": r[0], "mount": r[1], "total_bytes": r[2], "used_bytes": r[3], "free_bytes": r[4],
             "used_percent": r[5], "free_gb": r[4] / 1073741824 if r[4] is not None else None, "ts": _iso(r[6])}
            for r in cur.fetchall()
        ]


def q_disk_usage_history(window="1 hour", bucket_seconds=60):
    bucket = _bucket_expr()
    sql = f"""SELECT {bucket} AS bucket, device, max(mountpoint), avg(total_bytes), avg(used_bytes),
               avg(free_bytes), avg(used_percent)
        FROM monitor.disk_usage WHERE ts > now() - %s::interval
        GROUP BY bucket, device ORDER BY bucket, device"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql, (bucket_seconds, bucket_seconds, window))
        return [
            {"ts": _iso(r[0]), "device": r[1], "mount": r[2], "total_bytes": int(r[3]) if r[3] is not None else None,
             "used_bytes": int(r[4]) if r[4] is not None else None, "free_bytes": int(r[5]) if r[5] is not None else None,
             "used_percent": r[6]}
            for r in cur.fetchall()
        ]


def q_physical_disk():
    sql = """SELECT DISTINCT ON (device_id) device_id, friendly_name, model, media_type, bus_type,
             health_status, size_bytes/1073741824.0, ts FROM monitor.physical_disk ORDER BY device_id, ts DESC"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql)
        return [
            {"device_id": r[0], "friendly_name": r[1], "model": r[2], "media_type": r[3], "bus_type": r[4],
             "health": r[5], "size_gb": float(r[6]) if r[6] is not None else None, "ts": _iso(r[7])}
            for r in cur.fetchall()
        ]


def q_disk_smart_latest():
    sql = """SELECT DISTINCT ON (device) device, model, temperature_c, power_on_hours, power_cycle_count,
             percentage_used, available_spare, media_errors, reallocated_sectors, pending_sectors, smart_passed, ts
             FROM monitor.disk_smart ORDER BY device, ts DESC"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql)
        return [
            {"device": r[0], "model": r[1], "temp": r[2], "poh": r[3], "pcycles": r[4], "wear": r[5],
             "spare": r[6], "media_err": r[7], "realloc": r[8], "pending": r[9], "passed": r[10], "ts": _iso(r[11])}
            for r in cur.fetchall()
        ]


def q_disk_smart_history(window="1 hour", bucket_seconds=300):
    bucket = _bucket_expr()
    sql = f"""SELECT {bucket} AS bucket, device, avg(temperature_c)
        FROM monitor.disk_smart WHERE ts > now() - %s::interval AND temperature_c IS NOT NULL
        GROUP BY bucket, device ORDER BY bucket, device"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql, (bucket_seconds, bucket_seconds, window))
        return [{"ts": _iso(r[0]), "device": r[1], "temp": r[2]} for r in cur.fetchall()]


def q_disk_io(window="1 hour", bucket_seconds=10):
    bucket = _bucket_expr("ts")
    max_gap = INTERVALS["disk_io"] * 3
    sql = f"""WITH raw AS (
          SELECT *, extract(epoch FROM ts-lag(ts) OVER w) AS dt,
            read_count-lag(read_count) OVER w AS drc, write_count-lag(write_count) OVER w AS dwc,
            read_bytes-lag(read_bytes) OVER w AS drb, write_bytes-lag(write_bytes) OVER w AS dwb,
            read_time_ms-lag(read_time_ms) OVER w AS drt, write_time_ms-lag(write_time_ms) OVER w AS dwt,
            busy_time_ms-lag(busy_time_ms) OVER w AS dbusy
          FROM monitor.disk_io WHERE ts > now() - %s::interval WINDOW w AS (PARTITION BY device ORDER BY ts)
        ), valid AS (
          SELECT *, {bucket} AS bucket FROM raw
          WHERE dt > 0 AND dt <= %s AND drc >= 0 AND dwc >= 0 AND drb >= 0 AND dwb >= 0
            AND drt >= 0 AND dwt >= 0 AND dbusy >= 0
        )
        SELECT bucket, device, sum(drb), sum(dwb), sum(drb)/nullif(sum(dt),0), sum(dwb)/nullif(sum(dt),0),
          sum(drc)/nullif(sum(dt),0), sum(dwc)/nullif(sum(dt),0),
          sum(drt)/nullif(sum(drc),0), sum(dwt)/nullif(sum(dwc),0),
          least(100.0, 100.0*sum(dbusy)/nullif(sum(dt)*1000,0)), count(*)
        FROM valid GROUP BY bucket, device ORDER BY bucket, device"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql, (window, bucket_seconds, bucket_seconds, max_gap))
        rows = cur.fetchall()
    cumulative = defaultdict(lambda: [0, 0])
    result = []
    for r in rows:
        cumulative[r[1]][0] += r[2]
        cumulative[r[1]][1] += r[3]
        result.append({
            "ts": _iso(r[0]), "device": r[1], "read": cumulative[r[1]][0], "write": cumulative[r[1]][1],
            "read_delta": r[2], "write_delta": r[3], "read_bps": float(r[4]), "write_bps": float(r[5]),
            "read_iops": float(r[6]), "write_iops": float(r[7]),
            "read_latency_ms": float(r[8]) if r[8] is not None else None,
            "write_latency_ms": float(r[9]) if r[9] is not None else None,
            "busy_percent": float(r[10]) if r[10] is not None else None, "samples": r[11], "valid": True,
        })
    return result


def q_net(window="1 hour", bucket_seconds=10):
    bucket = _bucket_expr("ts")
    max_gap = INTERVALS["network"] * 3
    sql = f"""WITH raw AS (
          SELECT *, extract(epoch FROM ts-lag(ts) OVER w) AS dt,
            bytes_recv-lag(bytes_recv) OVER w AS dbr, bytes_sent-lag(bytes_sent) OVER w AS dbs,
            packets_recv-lag(packets_recv) OVER w AS dpr, packets_sent-lag(packets_sent) OVER w AS dps,
            errin-lag(errin) OVER w AS dei, errout-lag(errout) OVER w AS deo,
            dropin-lag(dropin) OVER w AS ddi, dropout-lag(dropout) OVER w AS ddo
          FROM monitor.net_io WHERE ts > now() - %s::interval WINDOW w AS (PARTITION BY iface ORDER BY ts)
        ), valid AS (
          SELECT *, {bucket} AS bucket FROM raw
          WHERE dt > 0 AND dt <= %s AND dbr >= 0 AND dbs >= 0 AND dpr >= 0 AND dps >= 0
            AND dei >= 0 AND deo >= 0 AND ddi >= 0 AND ddo >= 0
        )
        SELECT bucket, iface, sum(dbr), sum(dbs), sum(dbr)/nullif(sum(dt),0), sum(dbs)/nullif(sum(dt),0),
          sum(dpr)/nullif(sum(dt),0), sum(dps)/nullif(sum(dt),0), sum(dei), sum(deo), sum(ddi), sum(ddo),
          max(speed_mbps), bool_or(is_up), max(mtu), count(*)
        FROM valid GROUP BY bucket, iface ORDER BY bucket, iface"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql, (window, bucket_seconds, bucket_seconds, max_gap))
        rows = cur.fetchall()
    cumulative = defaultdict(lambda: [0, 0])
    result = []
    for r in rows:
        cumulative[r[1]][0] += r[2]
        cumulative[r[1]][1] += r[3]
        recv_bps, sent_bps = float(r[4]), float(r[5])
        speed_bps = float(r[12]) * 1_000_000 if r[12] else None
        utilization = max(recv_bps, sent_bps) * 8 / speed_bps * 100 if speed_bps else None
        result.append({
            "ts": _iso(r[0]), "iface": r[1], "recv": cumulative[r[1]][0], "sent": cumulative[r[1]][1],
            "recv_delta": r[2], "sent_delta": r[3], "recv_bps": recv_bps, "sent_bps": sent_bps,
            "recv_pps": float(r[6]), "sent_pps": float(r[7]), "errin": r[8], "errout": r[9],
            "dropin": r[10], "dropout": r[11], "speed_mbps": r[12], "is_up": r[13], "mtu": r[14],
            "utilization_percent": utilization, "samples": r[15], "valid": True,
        })
    return result


def q_net_latest():
    sql = """SELECT DISTINCT ON (iface) iface, bytes_recv, bytes_sent, speed_mbps, is_up, mtu, ts
             FROM monitor.net_io ORDER BY iface, ts DESC"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql)
        return [
            {"iface": r[0], "recv": r[1], "sent": r[2], "speed_mbps": r[3], "is_up": r[4], "mtu": r[5], "ts": _iso(r[6])}
            for r in cur.fetchall()
        ]


_CPU_HW_TYPES = ("cpu", "processor")


def _power_source_priority(name: str) -> int:
    """Ordena fontes de potência da CPU, a mais baixa é a canônica.

    Reconhece tanto os nomes literais ('CPU Package'/'CPU Platform') quanto o
    formato composto do LibreHardwareMonitor '<hw_type>:<hw_name>:<sensor>',
    em que o tipo de hardware vem isolado no primeiro segmento (ex.:
    'Cpu:AMD Ryzen 7 5700X:Package').
    """
    normalized = name.casefold()
    if "cpu package" in normalized:
        return 0
    if "cpu platform" in normalized:
        return 1
    parts = normalized.split(":")
    if len(parts) >= 2:
        hardware = parts[0].strip()
        leaf = parts[-1].strip()
        if hardware in _CPU_HW_TYPES:
            if leaf in ("package", "cpu package"):
                return 0
            if leaf in ("platform", "platform controller"):
                return 1
    return 99


def _integrate_power(series, field, max_gap_seconds, timezone):
    cumulative = 0.0
    covered = 0.0
    daily = defaultdict(float)
    previous = None
    for point in series:
        value = point.get(field)
        point[f"cumulative_{field}_wh"] = cumulative
        if previous and value is not None and previous.get(field) is not None:
            dt = (point["_ts"] - previous["_ts"]).total_seconds()
            if 0 < dt <= max_gap_seconds:
                wh = (previous[field] + value) / 2 * dt / 3600
                cumulative += wh
                covered += dt
                midpoint = previous["_ts"] + (point["_ts"] - previous["_ts"]) / 2
                daily[midpoint.astimezone(timezone).date().isoformat()] += wh
                point[f"cumulative_{field}_wh"] = cumulative
        previous = point
    return cumulative, covered, dict(sorted(daily.items()))


def _period_totals(estimated_daily, measured_daily=None):
    measured_daily = measured_daily or {}
    weekly = defaultdict(lambda: [0.0, 0.0])
    monthly = defaultdict(lambda: [0.0, 0.0])
    days = []
    for day in sorted(set(estimated_daily) | set(measured_daily)):
        estimated = estimated_daily.get(day, 0.0)
        measured = measured_daily.get(day, 0.0)
        date = datetime.fromisoformat(day).date()
        iso = date.isocalendar()
        weekly[f"{iso.year}-W{iso.week:02d}"][0] += measured
        weekly[f"{iso.year}-W{iso.week:02d}"][1] += estimated
        monthly[date.strftime("%Y-%m")][0] += measured
        monthly[date.strftime("%Y-%m")][1] += estimated
        days.append({"period": day, "measured_wh": measured, "estimated_wh": estimated})
    return {
        "daily": days,
        "weekly": [
            {"period": key, "measured_wh": value[0], "estimated_wh": value[1]}
            for key, value in sorted(weekly.items())
        ],
        "monthly": [
            {"period": key, "measured_wh": value[0], "estimated_wh": value[1]}
            for key, value in sorted(monthly.items())
        ],
    }


def q_power(window="1 hour", bucket_seconds=10):
    bucket = _bucket_expr()
    sensor_sql = f"""SELECT {bucket} AS bucket, name, avg(value), max(unit), max(ts)
        FROM monitor.sensors WHERE sensor_type='power' AND value >= 0 AND ts > now() - %s::interval
        GROUP BY bucket, name ORDER BY bucket, name"""
    gpu_sql = f"""SELECT {bucket} AS bucket, avg(power_draw_w), avg(utilization_gpu_percent), max(ts)
        FROM monitor.gpu WHERE ts > now() - %s::interval GROUP BY bucket ORDER BY bucket"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sensor_sql, (bucket_seconds, bucket_seconds, window))
        sensor_rows = cur.fetchall()
        cur.execute(gpu_sql, (bucket_seconds, bucket_seconds, window))
        gpu_rows = cur.fetchall()

    counts = Counter(r[1] for r in sensor_rows if _power_source_priority(r[1]) < 99)
    cpu_name = min(counts, key=lambda name: (_power_source_priority(name), -counts[name]), default=None)
    by_ts = defaultdict(dict)
    source_stats = defaultdict(list)
    for ts, name, value, unit, actual_ts in sensor_rows:
        source_stats[name].append(float(value))
        by_ts[ts]["actual_ts"] = max(actual_ts, by_ts[ts].get("actual_ts", actual_ts))
        if name == cpu_name:
            by_ts[ts]["cpu_w"] = float(value)
    for ts, power, utilization, actual_ts in gpu_rows:
        by_ts[ts]["actual_ts"] = max(actual_ts, by_ts[ts].get("actual_ts", actual_ts))
        by_ts[ts]["gpu_measured_w"] = float(power) if power is not None else None
        by_ts[ts]["gpu_utilization"] = float(utilization) if utilization is not None else None

    gpu_model = SETTINGS.power_gpu_idle_w is not None and SETTINGS.power_gpu_max_w is not None
    series = []
    last_cpu = last_gpu = last_utilization = None
    for ts in sorted(by_ts):
        raw_point = by_ts[ts]
        if raw_point.get("cpu_w") is not None:
            last_cpu = (raw_point["cpu_w"], ts)
        if raw_point.get("gpu_measured_w") is not None:
            last_gpu = (raw_point["gpu_measured_w"], ts)
        if raw_point.get("gpu_utilization") is not None:
            last_utilization = (raw_point["gpu_utilization"], ts)
        cpu = last_cpu[0] if last_cpu and (ts - last_cpu[1]).total_seconds() <= INTERVALS["sensors"] * 3 else None
        gpu_measured = last_gpu[0] if last_gpu and (ts - last_gpu[1]).total_seconds() <= INTERVALS["gpu"] * 3 else None
        gpu_utilization = (
            last_utilization[0]
            if last_utilization and (ts - last_utilization[1]).total_seconds() <= INTERVALS["gpu"] * 3
            else None
        )
        gpu_estimated = None
        if gpu_measured is None and gpu_model and gpu_utilization is not None:
            fraction = min(100.0, max(0.0, gpu_utilization)) / 100
            gpu_estimated = SETTINGS.power_gpu_idle_w + (SETTINGS.power_gpu_max_w - SETTINGS.power_gpu_idle_w) * fraction
        measured = (cpu or 0) + (gpu_measured or 0) if cpu is not None or gpu_measured is not None else None
        estimated = None
        if cpu is not None:
            estimated = (cpu + (gpu_measured if gpu_measured is not None else gpu_estimated or 0) + SETTINGS.power_aux_baseline_w) / SETTINGS.power_psu_efficiency
        actual_ts = raw_point.get("actual_ts", ts)
        series.append({
            "_ts": actual_ts, "ts": _iso(actual_ts), "cpu_w": cpu, "gpu_measured_w": gpu_measured,
            "gpu_estimated_w": gpu_estimated, "measured_w": measured, "estimated_w": estimated,
        })

    max_gap = max(bucket_seconds * 2, max(INTERVALS["gpu"], INTERVALS["sensors"]) * 3)
    timezone = ZoneInfo(SETTINGS.dashboard_timezone)
    measured_wh, measured_covered, measured_daily = _integrate_power(series, "measured_w", max_gap, timezone)
    estimated_wh, estimated_covered, daily = _integrate_power(series, "estimated_w", max_gap, timezone)
    window_seconds = {
        "15 minutes": 900, "1 hour": 3600, "6 hours": 21600, "24 hours": 86400,
        "7 days": 604800, "30 days": 2592000,
    }[window]
    sources = []
    for name, values in sorted(source_stats.items()):
        sources.append({
            "name": name, "unit": "W", "latest": values[-1], "minimum": min(values),
            "average": sum(values) / len(values), "maximum": max(values), "samples": len(values),
            "included": name == cpu_name, "reason": "CPU canônica" if name == cpu_name else "excluído para evitar sobreposição",
        })
    gpu_measured_available = any(p["gpu_measured_w"] is not None for p in series)
    gpu_values = [float(r[1]) for r in gpu_rows if r[1] is not None]
    sources.append({
        "name": "GPU (NVML)", "unit": "W", "latest": gpu_values[-1] if gpu_values else None,
        "minimum": min(gpu_values) if gpu_values else None, "average": sum(gpu_values) / len(gpu_values) if gpu_values else None,
        "maximum": max(gpu_values) if gpu_values else None, "samples": len(gpu_values),
        "included": gpu_measured_available or gpu_model,
        "reason": "potência nativa" if gpu_measured_available else "modelo por utilização" if gpu_model else "potência indisponível; modelo desativado",
    })
    defaults = SETTINGS.power_aux_baseline_w == 30.0 and SETTINGS.power_psu_efficiency == 0.90
    quality = "partial" if not (gpu_measured_available or gpu_model) else "estimated_default" if defaults else "estimated_calibrated"
    for point in series:
        point["cumulative_measured_wh"] = point.pop("cumulative_measured_w_wh")
        point["cumulative_estimated_wh"] = point.pop("cumulative_estimated_w_wh")
        point.pop("_ts")
    return {
        "meta": {
            "window": window, "bucket_seconds": bucket_seconds, "timezone": SETTINGS.dashboard_timezone,
            "quality": quality, "cpu_source": cpu_name, "gpu_power_available": gpu_measured_available,
            "gpu_model_enabled": gpu_model, "aux_baseline_w": SETTINGS.power_aux_baseline_w,
            "psu_efficiency": SETTINGS.power_psu_efficiency,
            "coverage_percent": min(100.0, estimated_covered / window_seconds * 100),
        },
        "summary": {
            "measured_wh": measured_wh, "estimated_wh": estimated_wh,
            "average_estimated_w": estimated_wh * 3600 / estimated_covered if estimated_covered else None,
            "peak_estimated_w": max((p["estimated_w"] for p in series if p["estimated_w"] is not None), default=None),
            "measured_coverage_percent": min(100.0, measured_covered / window_seconds * 100),
        },
        "series": series, "periods": _period_totals(daily, measured_daily), "sources": sources,
    }


def q_processes():
    sql = """SELECT name, pid, cpu_percent, memory_percent, memory_rss_bytes/1048576.0, username
             FROM monitor.processes WHERE ts = (SELECT max(ts) FROM monitor.processes)
             ORDER BY cpu_percent DESC LIMIT 15"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql)
        return [
            {"name": r[0], "pid": r[1], "cpu": r[2], "mem": r[3], "rss_mb": float(r[4]) if r[4] is not None else None, "user": r[5]}
            for r in cur.fetchall()
        ]


def q_system():
    sql = """SELECT ts, hostname, uptime_seconds, cpu_name, os_build, total_ram_bytes/1073741824.0
             FROM monitor.system_info ORDER BY ts DESC LIMIT 1"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql)
        r = cur.fetchone()
        if not r:
            return None
        return {"ts": _iso(r[0]), "hostname": r[1], "uptime": r[2], "cpu_name": r[3], "os_build": r[4],
                "ram_gb": float(r[5]) if r[5] is not None else None}


def q_heartbeat():
    sql = "SELECT hostname, collector, ts, success, error FROM monitor.v_last_heartbeat ORDER BY collector"
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql)
        return [{"host": r[0], "collector": r[1], "ts": _iso(r[2]), "success": r[3], "error": r[4]} for r in cur.fetchall()]


def q_db_size():
    sql = """SELECT pg_size_pretty(pg_database_size(current_database())),
             (SELECT count(*) FROM monitor.cpu), (SELECT count(*) FROM monitor.sensors)"""
    with get_conn() as conn, conn.cursor() as cur:
        cur.execute(sql)
        r = cur.fetchone()
        return {"size": r[0], "cpu_rows": r[1], "sensor_rows": r[2]}

