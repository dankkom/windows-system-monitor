import json
from datetime import datetime, timezone

def collect(hostname):
    ts = datetime.now(timezone.utc)
    rows = []
    try:
        import wmi
        c = wmi.WMI()
        for svc in c.Win32_Service():
            rows.append((
                ts, hostname,
                svc.Name,
                svc.DisplayName,
                svc.State,
                svc.StartMode,
                int(svc.ProcessId) if svc.ProcessId else None,
                json.dumps({"Name": svc.Name, "State": svc.State, "StartMode": svc.StartMode, "Status": getattr(svc, 'Status', None)})
            ))
    except Exception as e:
        # fallback via psutil
        try:
            import psutil
            for svc in psutil.win_service_iter():
                try:
                    info = svc.as_dict()
                    rows.append((ts, hostname, info.get('name'), info.get('display_name'), info.get('status'), info.get('start_type'), int(info.get('pid')) if info.get('pid') else None, json.dumps(info, default=str)))
                except:
                    continue
        except Exception as e2:
            rows.append((ts, hostname, "error", str(e2)[:200], "error", None, None, json.dumps({"error": str(e)})))
    columns = ["ts","hostname","name","display_name","status","start_type","pid","raw"]
    return columns, rows
