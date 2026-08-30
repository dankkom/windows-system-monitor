import psutil
import json
from datetime import datetime, timezone

def collect(hostname):
    ts = datetime.now(timezone.utc)
    rows = []
    try:
        conns = psutil.net_connections(kind='inet')
    except:
        conns = []
    for c in conns:
        try:
            laddr_ip = c.laddr.ip if c.laddr else None
            laddr_port = c.laddr.port if c.laddr else None
            raddr_ip = c.raddr.ip if c.raddr else None
            raddr_port = c.raddr.port if c.raddr else None
            rows.append((
                ts, hostname,
                int(c.fd) if c.fd != -1 else None,
                str(c.family), str(c.type),
                str(laddr_ip) if laddr_ip else None, int(laddr_port) if laddr_port else None,
                str(raddr_ip) if raddr_ip else None, int(raddr_port) if raddr_port else None,
                str(c.status),
                int(c.pid) if c.pid else None,
                json.dumps({"family": str(c.family), "type": str(c.type), "status": c.status})
            ))
        except:
            continue
    columns = ["ts","hostname","fd","family","type","laddr_ip","laddr_port","raddr_ip","raddr_port","status","pid","raw"]
    return columns, rows
