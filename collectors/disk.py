import psutil
import json
from datetime import datetime, timezone

def collect_usage(hostname):
    ts = datetime.now(timezone.utc)
    rows = []
    for part in psutil.disk_partitions(all=False):
        try:
            usage = psutil.disk_usage(part.mountpoint)
            rows.append((
                ts, hostname,
                part.device, part.mountpoint, part.fstype,
                int(usage.total), int(usage.used), int(usage.free), float(usage.percent),
                json.dumps({"opts": part.opts, "usage": usage._asdict()})
            ))
        except Exception as e:
            rows.append((ts, hostname, part.device, part.mountpoint, part.fstype, None, None, None, None, json.dumps({"error": str(e)})))
    columns = ["ts","hostname","device","mountpoint","fstype","total_bytes","used_bytes","free_bytes","used_percent","raw"]
    return columns, rows

def collect_io(hostname):
    ts = datetime.now(timezone.utc)
    try:
        counters = psutil.disk_io_counters(perdisk=True)
    except:
        counters = {}
    rows = []
    for dev, c in counters.items():
        rows.append((
            ts, hostname, dev,
            int(c.read_count), int(c.write_count),
            int(c.read_bytes), int(c.write_bytes),
            int(c.read_time), int(c.write_time),
            int(getattr(c, 'busy_time', 0) or 0),
            json.dumps(c._asdict())
        ))
    columns = ["ts","hostname","device","read_count","write_count","read_bytes","write_bytes","read_time_ms","write_time_ms","busy_time_ms","raw"]
    return columns, rows

def collect(hostname):
    # convenience: returns both but caller should call separately; here return usage
    return collect_usage(hostname)
