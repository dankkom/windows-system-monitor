import psutil
import json
from datetime import datetime, timezone

def collect_io(hostname):
    ts = datetime.now(timezone.utc)
    counters = psutil.net_io_counters(pernic=True, nowrap=True)
    stats = psutil.net_if_stats()
    rows = []
    for iface, c in counters.items():
        st = stats.get(iface)
        rows.append((
            ts, hostname, iface,
            int(c.bytes_sent), int(c.bytes_recv),
            int(c.packets_sent), int(c.packets_recv),
            int(c.errin), int(c.errout),
            int(c.dropin), int(c.dropout),
            float(st.speed) if st else None,
            bool(st.isup) if st else None,
            int(st.mtu) if st else None,
            json.dumps({"counters": c._asdict(), "stats": st._asdict() if st else None})
        ))
    columns = ["ts","hostname","iface","bytes_sent","bytes_recv","packets_sent","packets_recv","errin","errout","dropin","dropout","speed_mbps","is_up","mtu","raw"]
    return columns, rows

def collect_addrs(hostname):
    ts = datetime.now(timezone.utc)
    addrs = psutil.net_if_addrs()
    rows = []
    for iface, lst in addrs.items():
        for a in lst:
            rows.append((
                ts, hostname, iface,
                str(a.family), a.address,
                a.netmask, a.broadcast,
                json.dumps({"family": str(a.family), "address": a.address})
            ))
    columns = ["ts","hostname","iface","family","address","netmask","broadcast","raw"]
    return columns, rows

def collect(hostname):
    return collect_io(hostname)
