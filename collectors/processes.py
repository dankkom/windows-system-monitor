import psutil
import json
from datetime import datetime, timezone

def collect(hostname, top_n=50):
    ts = datetime.now(timezone.utc)
    procs = []
    for p in psutil.process_iter(['pid','ppid','name','exe','cmdline','username','status','cpu_percent','memory_percent','memory_info','num_threads','create_time','cwd']):
        try:
            info = p.info
            # cpu_percent needs call; first call may be 0, use cached
            cpu = p.cpu_percent(interval=None)
            mem_info = info.get('memory_info')
            rss = mem_info.rss if mem_info else None
            vms = mem_info.vms if mem_info else None
            shared = getattr(mem_info, 'shared', None) if mem_info else None
            try:
                io = p.io_counters()
                io_r = int(io.read_bytes); io_w = int(io.write_bytes); io_rc = int(io.read_count); io_wc = int(io.write_count)
            except:
                io_r = io_w = io_rc = io_wc = None
            try:
                handles = p.num_handles() if hasattr(p, 'num_handles') else None
            except:
                handles = None
            try:
                fds = p.num_fds() if hasattr(p, 'num_fds') else None
            except:
                fds = None
            cmdline = " ".join(info.get('cmdline') or [])[:2000]
            create_ts = None
            try:
                create_ts = datetime.fromtimestamp(info['create_time'], tz=timezone.utc) if info.get('create_time') else None
            except:
                pass
            procs.append((
                ts, hostname,
                int(info['pid']), int(info['ppid']) if info.get('ppid') else None,
                info.get('name'), info.get('exe'), cmdline, info.get('username'), info.get('status'),
                float(cpu) if cpu is not None else None,
                float(info.get('memory_percent') or 0),
                int(rss) if rss else None,
                int(vms) if vms else None,
                int(shared) if shared else None,
                int(info.get('num_threads') or 0),
                int(handles) if handles else None,
                int(fds) if fds else None,
                io_r, io_w, io_rc, io_wc,
                create_ts,
                info.get('cwd'),
                json.dumps({"pid": info['pid'], "name": info.get('name')})
            ))
        except (psutil.NoSuchProcess, psutil.AccessDenied):
            continue
    # Top by cpu + memory
    procs_sorted = sorted(procs, key=lambda x: ((x[9] or 0) + (x[10] or 0)), reverse=True)[:top_n]
    columns = ["ts","hostname","pid","ppid","name","exe","cmdline","username","status","cpu_percent","memory_percent","memory_rss_bytes","memory_vms_bytes","memory_shared_bytes","num_threads","num_handles","num_fds","io_read_bytes","io_write_bytes","io_read_count","io_write_count","create_time","cwd","raw"]
    return columns, procs_sorted
