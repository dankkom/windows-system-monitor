import psutil
import json
from datetime import datetime, timezone

def collect(hostname):
    ts = datetime.now(timezone.utc)
    cpu_percent = psutil.cpu_percent(interval=None)
    per_core = psutil.cpu_percent(interval=None, percpu=True)
    freq = psutil.cpu_freq()
    counts_logical = psutil.cpu_count(logical=True)
    counts_physical = psutil.cpu_count(logical=False)
    stats = psutil.cpu_stats()
    # load avg not available on Windows, emulate via cpu_percent
    try:
        load = psutil.getloadavg()[0] if hasattr(psutil, "getloadavg") else None
    except:
        load = None

    row = (
        ts, hostname,
        float(cpu_percent) if cpu_percent is not None else None,
        per_core,
        counts_logical,
        counts_physical,
        float(freq.current) if freq else None,
        float(freq.min) if freq else None,
        float(freq.max) if freq else None,
        float(load) if load is not None else None,
        int(stats.ctx_switches) if stats else None,
        int(stats.interrupts) if stats else None,
        json.dumps({"freq": str(freq), "stats": str(stats)})
    )
    columns = ["ts","hostname","cpu_total_percent","cpu_per_core_percent","cpu_count_logical","cpu_count_physical","freq_current_mhz","freq_min_mhz","freq_max_mhz","load_1m","ctx_switches","interrupts","raw"]
    return columns, [row]

# Alias for compatibility
collect_cpu = collect
