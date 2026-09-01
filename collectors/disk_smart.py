import json
import subprocess
import shutil
import os
from datetime import datetime, timezone

SMARTCTL = r"C:\Program Files\smartmontools\bin\smartctl.exe"
if not os.path.exists(SMARTCTL):
    SMARTCTL = shutil.which("smartctl") or "smartctl"

def _run(cmd, timeout=10):
    try:
        out = subprocess.check_output(cmd, text=True, timeout=timeout, stderr=subprocess.STDOUT)
        return out
    except subprocess.CalledProcessError as e:
        return e.output
    except Exception:
        return None

def collect_physical(hostname):
    ts = datetime.now(timezone.utc)
    rows = []
    # Via PowerShell Get-PhysicalDisk (works non-elevated without reliability counter)
    try:
        ps = r"Get-PhysicalDisk | Select-Object DeviceId,FriendlyName,Model,SerialNumber,FirmwareVersion,MediaType,BusType,HealthStatus,OperationalStatus,Size,AllocatedSize,IsBoot | ConvertTo-Json -Depth 3"
        out = subprocess.check_output(["powershell", "-NoProfile", "-Command", ps], text=True, timeout=10)
        import json as j
        data = j.loads(out)
        if isinstance(data, dict):
            data = [data]
        for d in data:
            rows.append((
                ts, hostname,
                str(d.get("DeviceId")), d.get("FriendlyName"), d.get("Model"),
                d.get("SerialNumber"), d.get("FirmwareVersion"),
                d.get("MediaType"), d.get("BusType"),
                d.get("HealthStatus"), d.get("OperationalStatus"),
                int(d.get("Size") or 0), int(d.get("AllocatedSize") or 0),
                bool(d.get("IsBoot")) if d.get("IsBoot") is not None else None,
                json.dumps(d, default=str)
            ))
    except Exception as e:
        rows.append((ts, hostname, "error", None, None, None, None, None, None, None, None, None, None, None, json.dumps({"error": str(e)})))
    columns = ["ts","hostname","device_id","friendly_name","model","serial_number","firmware_version","media_type","bus_type","health_status","operational_status","size_bytes","allocated_size","is_boot","raw"]
    return columns, rows

def collect_smart(hostname):
    ts = datetime.now(timezone.utc)
    rows = []
    # scan devices
    scan_out = _run([SMARTCTL, "--scan"], timeout=10)
    devices = []
    if scan_out:
        for line in scan_out.splitlines():
            line=line.strip()
            if not line or line.startswith("#"):
                continue
            # format: /dev/sda -d ata # ...
            parts = line.split()
            if len(parts) >= 1:
                dev = parts[0]
                dtype = None
                if "-d" in parts:
                    try:
                        idx = parts.index("-d")
                        dtype = parts[idx+1]
                    except:
                        pass
                devices.append((dev, dtype))
    if not devices:
        # fallback list known
        devices = [("/dev/sda","ata"), ("/dev/sdb","ata"), ("/dev/sdc","ata"), ("/dev/sdd","nvme")]

    for dev, dtype in devices:
        try:
            cmd = [SMARTCTL, "-a", dev, "-j"]
            if dtype:
                # smartctl scan already includes correct -d via dev alone works for sda/sdd, but for nvme we use as is
                pass
            out = _run(cmd, timeout=10)
            if not out:
                continue
            data = json.loads(out)
            # skip if no device
            if "device" not in data:
                continue
            model = data.get("model_name") or data.get("model_family") or ""
            serial = data.get("serial_number") or ""
            firmware = data.get("firmware_version") or ""
            device_type = data.get("device",{}).get("type") or dtype or ""
            protocol = data.get("device",{}).get("protocol") or ""
            smart_passed = None
            if "smart_status" in data:
                smart_passed = data["smart_status"].get("passed")
            # temps
            temp = None
            power_on_hours = None
            power_cycles = None
            percentage_used = None
            available_spare = None
            avail_thresh = None
            media_errors = None
            reallocated = None
            pending = None
            host_reads = None
            host_writes = None
            data_units_read = None
            data_units_written = None
            total_lbas_read = None
            total_lbas_written = None
            unsafe_shutdowns = None

            # NVMe path
            if "nvme_smart_health_information_log" in data:
                info = data["nvme_smart_health_information_log"]
                temp = info.get("temperature")
                # smartctl reports temperature in Celsius already for NVMe, but sometimes Kelvin? Check: our earlier NVMe temp 49 matches
                power_on_hours = info.get("power_on_hours")
                power_cycles = info.get("power_cycles")
                percentage_used = info.get("percentage_used")
                available_spare = info.get("available_spare")
                avail_thresh = info.get("available_spare_threshold")
                media_errors = info.get("media_errors")
                host_reads = info.get("host_reads")
                host_writes = info.get("host_writes")
                data_units_read = info.get("data_units_read")
                data_units_written = info.get("data_units_written")
                unsafe_shutdowns = info.get("unsafe_shutdowns")
            # ATA path
            if "ata_smart_attributes" in data:
                tbl = data["ata_smart_attributes"].get("table", [])
                for attr in tbl:
                    nid = attr.get("id")
                    name = attr.get("name")
                    raw = attr.get("raw",{})
                    val = raw.get("value") if isinstance(raw, dict) else None
                    rstr = raw.get("string","") if isinstance(raw, dict) else ""
                    # string parsing for temperature/hours (raw.value é bit-packed, usar string)
                    if nid == 9 and name == "Power_On_Hours":
                        try:
                            power_on_hours = int(rstr.split()[0]) if rstr else int(val) if isinstance(val,int) else None
                        except:
                            power_on_hours = int(val) if isinstance(val,int) else None
                    elif nid == 12 and name == "Power_Cycle_Count":
                        try:
                            power_cycles = int(rstr.split()[0]) if rstr else int(val) if isinstance(val,int) else None
                        except:
                            power_cycles = int(val) if isinstance(val,int) else None
                    elif nid == 5 and name == "Reallocated_Sector_Ct":
                        reallocated = int(val) if isinstance(val,int) else None
                    elif nid == 197 and name == "Current_Pending_Sector":
                        pending = int(val) if isinstance(val,int) else None
                    elif nid == 194 and "Temperature" in name:
                        # raw string like "41 (0 14 0 0 0)" -> first number is current
                        try:
                            s = rstr
                            # extract first int before space
                            temp = int(s.split()[0]) if s else None
                        except:
                            pass
                    elif nid == 194 and temp is None:
                        temp = int(val) if isinstance(val,int) else None
                # total LBAs
                for attr in tbl:
                    if attr.get("id")==241:
                        total_lbas_written = int(attr.get("raw",{}).get("value") or 0) if attr.get("raw",{}).get("value") is not None else None
                    if attr.get("id")==242:
                        total_lbas_read = int(attr.get("raw",{}).get("value") or 0) if attr.get("raw",{}).get("value") is not None else None
                # also temperature from airflow 190
                if temp is None:
                    for attr in tbl:
                        if attr.get("id")==190:
                            try:
                                s = attr.get("raw",{}).get("string","")
                                temp = int(s.split()[0]) if s else None
                            except:
                                pass
            # fallback temperature from ata_smart_data?
            if temp is None and "temperature" in data:
                # sometimes top-level
                temp = data["temperature"].get("current") if isinstance(data.get("temperature"), dict) else None

            rows.append((
                ts, hostname, dev, model, serial, firmware,
                device_type, protocol, smart_passed,
                float(temp) if isinstance(temp,(int,float)) else None,
                int(power_on_hours) if isinstance(power_on_hours,(int,float)) else None,
                int(power_cycles) if isinstance(power_cycles,(int,float)) else None,
                float(percentage_used) if isinstance(percentage_used,(int,float)) else None,
                float(available_spare) if isinstance(available_spare,(int,float)) else None,
                float(avail_thresh) if isinstance(avail_thresh,(int,float)) else None,
                int(media_errors) if isinstance(media_errors,(int,float)) else None,
                int(reallocated) if isinstance(reallocated,(int,float)) else None,
                int(pending) if isinstance(pending,(int,float)) else None,
                int(host_reads) if isinstance(host_reads,(int,float)) else None,
                int(host_writes) if isinstance(host_writes,(int,float)) else None,
                int(data_units_read) if isinstance(data_units_read,(int,float)) else None,
                int(data_units_written) if isinstance(data_units_written,(int,float)) else None,
                int(total_lbas_read) if isinstance(total_lbas_read,(int,float)) else None,
                int(total_lbas_written) if isinstance(total_lbas_written,(int,float)) else None,
                int(unsafe_shutdowns) if isinstance(unsafe_shutdowns,(int,float)) else None,
                json.dumps(data)
            ))
        except Exception as e:
            rows.append((ts, hostname, dev, "error", None, None, None, None, None, None, None, None, None, None, None, None, None, None, None, None, None, None, None, None, None, json.dumps({"error": str(e), "device": dev})))
    columns = ["ts","hostname","device","model","serial_number","firmware_version","device_type","protocol","smart_passed","temperature_c","power_on_hours","power_cycle_count","percentage_used","available_spare","available_spare_threshold","media_errors","reallocated_sectors","pending_sectors","host_reads","host_writes","data_units_read","data_units_written","total_lbas_read","total_lbas_written","unsafe_shutdowns","raw"]
    return columns, rows
