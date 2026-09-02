"""
Coletores baseados em psutil: cpu, memory, disk (io/usage), network (io/addrs),
connections e services. Todos exponem a mesma interface: collect_*(hostname) -> (columns, rows).
"""
import json
import logging
import psutil

log = logging.getLogger(__name__)
from datetime import datetime, timezone


# ---------------------------------------------------------------------------
# CPU
# ---------------------------------------------------------------------------

def collect_cpu(hostname):
    ts = datetime.now(timezone.utc)
    cpu_percent = psutil.cpu_percent(interval=None)
    per_core = psutil.cpu_percent(interval=None, percpu=True)
    freq = psutil.cpu_freq()
    counts_logical = psutil.cpu_count(logical=True)
    counts_physical = psutil.cpu_count(logical=False)
    stats = psutil.cpu_stats()
    try:
        load = psutil.getloadavg()[0] if hasattr(psutil, "getloadavg") else None
    except Exception:
        load = None
    columns = [
        "ts", "hostname", "cpu_total_percent", "cpu_per_core_percent",
        "cpu_count_logical", "cpu_count_physical",
        "freq_current_mhz", "freq_min_mhz", "freq_max_mhz",
        "load_1m", "ctx_switches", "interrupts", "raw",
    ]
    row = (
        ts, hostname,
        float(cpu_percent) if cpu_percent is not None else None,
        per_core,
        counts_logical, counts_physical,
        float(freq.current) if freq else None,
        float(freq.min) if freq else None,
        float(freq.max) if freq else None,
        float(load) if load is not None else None,
        int(stats.ctx_switches) if stats else None,
        int(stats.interrupts) if stats else None,
        json.dumps({"freq": str(freq), "stats": str(stats)}),
    )
    return columns, [row]


# ---------------------------------------------------------------------------
# Memory
# ---------------------------------------------------------------------------

def collect_memory(hostname):
    ts = datetime.now(timezone.utc)
    vm = psutil.virtual_memory()
    swap = psutil.swap_memory()
    columns = [
        "ts", "hostname",
        "total_bytes", "available_bytes", "used_bytes", "used_percent",
        "free_bytes", "active_bytes", "inactive_bytes", "cached_bytes",
        "wired_bytes", "buffers_bytes", "shared_bytes", "slab_bytes",
        "swap_total_bytes", "swap_used_bytes", "swap_free_bytes", "swap_used_percent",
        "swap_sin", "swap_sout", "pagefile_total", "pagefile_used", "raw",
    ]
    row = (
        ts, hostname,
        int(vm.total), int(vm.available), int(vm.used), float(vm.percent),
        int(vm.free),
        getattr(vm, "active", None), getattr(vm, "inactive", None),
        getattr(vm, "cached", None), getattr(vm, "wired", None),
        getattr(vm, "buffers", None), getattr(vm, "shared", None), getattr(vm, "slab", None),
        int(swap.total), int(swap.used), int(swap.free), float(swap.percent),
        int(swap.sin), int(swap.sout),
        int(swap.total), int(swap.used),
        json.dumps({"virtual": vm._asdict(), "swap": swap._asdict()}),
    )
    return columns, [row]


# ---------------------------------------------------------------------------
# Disk - usage and I/O
# ---------------------------------------------------------------------------

def collect_disk_usage(hostname):
    ts = datetime.now(timezone.utc)
    rows = []
    columns = ["ts", "hostname", "device", "mountpoint", "fstype",
                "total_bytes", "used_bytes", "free_bytes", "used_percent", "raw"]
    for part in psutil.disk_partitions(all=False):
        try:
            usage = psutil.disk_usage(part.mountpoint)
            rows.append((
                ts, hostname,
                part.device, part.mountpoint, part.fstype,
                int(usage.total), int(usage.used), int(usage.free), float(usage.percent),
                json.dumps({"opts": part.opts, "usage": usage._asdict()}),
            ))
        except Exception as exc:
            rows.append((ts, hostname, part.device, part.mountpoint, part.fstype,
                         None, None, None, None, json.dumps({"error": str(exc)})))
    return columns, rows


def collect_disk_io(hostname):
    ts = datetime.now(timezone.utc)
    try:
        counters = psutil.disk_io_counters(perdisk=True)
    except Exception:
        counters = {}
    columns = ["ts", "hostname", "device",
                "read_count", "write_count", "read_bytes", "write_bytes",
                "read_time_ms", "write_time_ms", "busy_time_ms", "raw"]
    rows = [
        (
            ts, hostname, dev,
            int(c.read_count), int(c.write_count),
            int(c.read_bytes), int(c.write_bytes),
            int(c.read_time), int(c.write_time),
            int(getattr(c, "busy_time", 0) or 0),
            json.dumps(c._asdict()),
        )
        for dev, c in counters.items()
    ]
    return columns, rows


# ---------------------------------------------------------------------------
# Network - I/O and addresses
# ---------------------------------------------------------------------------

def collect_net_io(hostname):
    ts = datetime.now(timezone.utc)
    counters = psutil.net_io_counters(pernic=True, nowrap=True)
    stats = psutil.net_if_stats()
    columns = [
        "ts", "hostname", "iface",
        "bytes_sent", "bytes_recv", "packets_sent", "packets_recv",
        "errin", "errout", "dropin", "dropout",
        "speed_mbps", "is_up", "mtu", "raw",
    ]
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
            json.dumps({"counters": c._asdict(), "stats": st._asdict() if st else None}),
        ))
    return columns, rows


def collect_net_addrs(hostname):
    ts = datetime.now(timezone.utc)
    addrs = psutil.net_if_addrs()
    columns = ["ts", "hostname", "iface", "family", "address", "netmask", "broadcast", "raw"]
    rows = [
        (ts, hostname, iface, str(a.family), a.address, a.netmask, a.broadcast,
         json.dumps({"family": str(a.family), "address": a.address}))
        for iface, lst in addrs.items()
        for a in lst
    ]
    return columns, rows


# ---------------------------------------------------------------------------
# Connections
# ---------------------------------------------------------------------------

def collect_connections(hostname):
    ts = datetime.now(timezone.utc)
    columns = ["ts", "hostname", "fd", "family", "type",
                "laddr_ip", "laddr_port", "raddr_ip", "raddr_port", "status", "pid", "raw"]
    rows = []
    try:
        conns = psutil.net_connections(kind="inet")
    except Exception:
        conns = []
    for c in conns:
        try:
            rows.append((
                ts, hostname,
                int(c.fd) if c.fd != -1 else None,
                str(c.family), str(c.type),
                str(c.laddr.ip) if c.laddr else None,
                int(c.laddr.port) if c.laddr else None,
                str(c.raddr.ip) if c.raddr else None,
                int(c.raddr.port) if c.raddr else None,
                str(c.status),
                int(c.pid) if c.pid else None,
                json.dumps({"family": str(c.family), "type": str(c.type), "status": c.status}),
            ))
        except Exception:
            continue
    return columns, rows


# ---------------------------------------------------------------------------
# Services
# ---------------------------------------------------------------------------

def collect_services(hostname):
    ts = datetime.now(timezone.utc)
    columns = ["ts", "hostname", "name", "display_name", "status", "start_type", "pid", "raw"]
    rows = []
    try:
        import wmi
        c = wmi.WMI()
        for svc in c.Win32_Service():
            rows.append((
                ts, hostname,
                svc.Name, svc.DisplayName, svc.State, svc.StartMode,
                int(svc.ProcessId) if svc.ProcessId else None,
                json.dumps({"Name": svc.Name, "State": svc.State,
                            "StartMode": svc.StartMode,
                            "Status": getattr(svc, "Status", None)}),
            ))
    except Exception as exc:
        try:
            for svc in psutil.win_service_iter():
                try:
                    info = svc.as_dict()
                    rows.append((
                        ts, hostname,
                        info.get("name"), info.get("display_name"),
                        info.get("status"), info.get("start_type"),
                        int(info.get("pid")) if info.get("pid") else None,
                        json.dumps(info, default=str),
                    ))
                except Exception:
                    continue
        except Exception as exc2:
            log.warning("collect_services failed (wmi=%s, fallback=%s)", exc, exc2)
    return columns, rows
