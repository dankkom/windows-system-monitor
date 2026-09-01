import psutil
import platform
import json
from datetime import datetime, timezone

def collect(hostname):
    ts = datetime.now(timezone.utc)
    boot_ts = datetime.fromtimestamp(psutil.boot_time(), tz=timezone.utc)
    uptime = int((ts - boot_ts).total_seconds())
    # OS info via platform + wmi
    os_name = platform.system()
    os_version = platform.version()
    os_build = platform.win32_ver()[1] if hasattr(platform, 'win32_ver') else None
    arch = platform.machine()
    manufacturer = model = cpu_name = None
    total_ram = psutil.virtual_memory().total
    cpu_cores_phys = psutil.cpu_count(logical=False)
    cpu_cores_log = psutil.cpu_count(logical=True)
    try:
        import wmi
        c = wmi.WMI()
        for cs in c.Win32_ComputerSystem():
            manufacturer = cs.Manufacturer
            model = cs.Model
            total_ram = int(cs.TotalPhysicalMemory) if cs.TotalPhysicalMemory else total_ram
            break
        for proc in c.Win32_Processor():
            cpu_name = proc.Name.strip() if proc.Name else None
            cpu_cores_phys = int(proc.NumberOfCores) if proc.NumberOfCores else cpu_cores_phys
            cpu_cores_log = int(proc.NumberOfLogicalProcessors) if proc.NumberOfLogicalProcessors else cpu_cores_log
            break
    except:
        pass
    users = []
    logged = []
    try:
        for u in psutil.users():
            users.append(u.name)
            logged.append({"name": u.name, "terminal": u.terminal, "host": u.host, "started": u.started})
    except:
        pass
    batt_percent = batt_secs = batt_plugged = None
    try:
        batt = psutil.sensors_battery()
        if batt:
            batt_percent = float(batt.percent)
            batt_secs = int(batt.secsleft) if batt.secsleft != psutil.POWER_TIME_UNLIMITED else None
            batt_plugged = bool(batt.power_plugged)
    except:
        pass
    raw = {"platform": platform.uname()._asdict(), "boot_time": str(boot_ts)}
    row = (
        ts, hostname,
        boot_ts, int(uptime),
        os_name, os_version, os_build, arch,
        manufacturer, model,
        int(total_ram),
        cpu_name, int(cpu_cores_phys) if cpu_cores_phys else None, int(cpu_cores_log) if cpu_cores_log else None,
        users,
        json.dumps(logged),
        batt_percent, batt_secs, batt_plugged,
        json.dumps(raw)
    )
    columns = ["ts","hostname","boot_time","uptime_seconds","os_name","os_version","os_build","arch","manufacturer","model","total_ram_bytes","cpu_name","cpu_cores_physical","cpu_cores_logical","users","logged_users","battery_percent","battery_secs_left","battery_power_plugged","raw"]
    return columns, [row]

def collect_eventlog(hostname, hours=1):
    """Resumo de EventLog Windows últimos N horas por log/level"""
    ts = datetime.now(timezone.utc)
    rows = []
    try:
        import win32evtlog
        logs = ["System", "Application"]
        for log_name in logs:
            hand = win32evtlog.OpenEventLog(None, log_name)
            flags = win32evtlog.EVENTLOG_BACKWARDS_READ | win32evtlog.EVENTLOG_SEQUENTIAL_READ
            counts = {}
            latest = {}
            try:
                events = win32evtlog.ReadEventLog(hand, flags, 0)
                for ev in events[:200]:  # limita
                    level = str(ev.EventType)
                    eid = ev.EventID & 0xFFFF
                    key = (log_name, level, eid, ev.SourceName)
                    counts[key] = counts.get(key, 0) + 1
                    if key not in latest:
                        latest[key] = str(ev.StringInserts)[:500] if ev.StringInserts else ""
                    if len(counts) > 50:
                        break
            except:
                pass
            win32evtlog.CloseEventLog(hand)
            for (log_n, level, eid, provider), cnt in counts.items():
                level_map = {1: "Error", 2: "Warning", 4: "Information", 8: "AuditSuccess", 16: "AuditFailure"}
                level_str = level_map.get(int(level), level)
                rows.append((ts, hostname, log_n, level_str, int(eid), provider, int(cnt), latest.get((log_n, level, eid, provider)), json.dumps({"count": cnt})))
    except Exception as e:
        # fallback: no eventlog
        rows.append((ts, hostname, "System", "Information", None, "collector", 0, f"eventlog unavailable: {e}", json.dumps({"error": str(e)})))
    columns = ["ts","hostname","log_name","level","event_id","provider","count","latest_message","raw"]
    return columns, rows
