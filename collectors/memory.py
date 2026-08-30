import psutil
import json
from datetime import datetime, timezone

def collect(hostname):
    ts = datetime.now(timezone.utc)
    vm = psutil.virtual_memory()
    swap = psutil.swap_memory()
    # pagefile on Windows via WMI fallback via psutil
    try:
        # Use psutil swap as pagefile approximation
        page_total = swap.total
        page_used = swap.used
    except:
        page_total = page_used = None

    raw = {"virtual": vm._asdict(), "swap": swap._asdict()}
    row = (
        ts, hostname,
        int(vm.total), int(vm.available), int(vm.used), float(vm.percent),
        int(vm.free), getattr(vm, 'active', None), getattr(vm, 'inactive', None),
        getattr(vm, 'cached', None), getattr(vm, 'wired', None),
        getattr(vm, 'buffers', None), getattr(vm, 'shared', None), getattr(vm, 'slab', None),
        int(swap.total), int(swap.used), int(swap.free), float(swap.percent),
        int(swap.sin), int(swap.sout),
        int(page_total) if page_total else None,
        int(page_used) if page_used else None,
        json.dumps(raw)
    )
    columns = ["ts","hostname","total_bytes","available_bytes","used_bytes","used_percent","free_bytes","active_bytes","inactive_bytes","cached_bytes","wired_bytes","buffers_bytes","shared_bytes","slab_bytes","swap_total_bytes","swap_used_bytes","swap_free_bytes","swap_used_percent","swap_sin","swap_sout","pagefile_total","pagefile_used","raw"]
    # psutil vm may lack some fields on Windows -> convert None handling via raw; ensure ints
    # Some fields missing -> set to None
    def _fix(v):
        return int(v) if isinstance(v, int) else (int(v) if isinstance(v, float) else v)
    # Already built row with None safe
    return columns, [row]
