import json
import psutil
from datetime import datetime, timezone

# LibreHardwareMonitor via pythonnet - persistent Computer instance
_LHM = None
_LHM_ERROR = None


def _clean_text(value):
    """PostgreSQL text/jsonb reject NUL bytes sometimes exposed by hardware firmware."""
    return str(value).replace("\x00", "�")

def _get_lhm_computer():
    global _LHM, _LHM_ERROR
    if _LHM is not None or _LHM_ERROR is not None:
        return _LHM
    try:
        import clr
        import os
        dll_path = r"C:\tools\LibreHardwareMonitor\LibreHardwareMonitorLib.dll"
        if not os.path.exists(dll_path):
            # fallback to script relative path
            dll_path = os.path.join(os.path.dirname(__file__), "..", "LibreHardwareMonitorLib.dll")
        clr.AddReference(dll_path)
        from LibreHardwareMonitor.Hardware import Computer
        c = Computer()
        c.IsCpuEnabled = True
        c.IsGpuEnabled = True
        c.IsMemoryEnabled = True
        c.IsMotherboardEnabled = True
        c.IsControllerEnabled = True
        c.IsNetworkEnabled = True
        c.IsStorageEnabled = True
        c.Open()
        _LHM = c
        return c
    except Exception as e:
        _LHM_ERROR = str(e)
        return None

# Ensure cleanup on exit
import atexit
def _close_lhm():
    global _LHM
    try:
        if _LHM is not None:
            _LHM.Close()
    except:
        pass
atexit.register(_close_lhm)

def _collect_lhm(hostname, ts):
    rows = []
    c = _get_lhm_computer()
    if c is None:
        return rows
    try:
        for hw in c.Hardware:
            try:
                hw.Update()
            except:
                pass
            hw_name = _clean_text(hw.Name)
            hw_type = _clean_text(hw.HardwareType)
            hw_id = _clean_text(hw.Identifier)
            for s in hw.Sensors:
                try:
                    val = s.Value
                    if val is None:
                        continue
                    sensor_type = _clean_text(s.SensorType).lower()  # temperature, load, clock, voltage, fan, power, data, throughput, etc
                    # map SensorType to our sensor_type + unit
                    unit_map = {
                        "temperature": "C",
                        "load": "%",
                        "clock": "MHz",
                        "voltage": "V",
                        "fan": "RPM",
                        "power": "W",
                        "data": "GB",
                        "smalldata": "MB",
                        "throughput": "B/s",
                        "current": "A",
                        "energy": "Wh",
                        "noise": "dBA",
                        "control": "%",
                        "level": "%",
                    }
                    unit = unit_map.get(sensor_type, "")
                    # keep only meaningful sensors (skip noisy 0.0 package Power etc, but keep)
                    sensor_name = _clean_text(s.Name)
                    name = f"{hw_type}:{hw_name}:{sensor_name}"
                    label = f"{hw_name} {sensor_name}"
                    rows.append((
                        ts, hostname,
                        sensor_type,
                        name,
                        label,
                        float(val),
                        unit,
                        json.dumps({
                            "hardware": hw_name,
                            "hardware_type": hw_type,
                            "identifier": _clean_text(s.Identifier),
                            "sensor": sensor_name,
                            "sensor_type": sensor_type,
                            "hardware_id": hw_id,
                            "value": float(val)
                        })
                    ))
                except:
                    continue
            for sub in hw.SubHardware:
                try:
                    sub.Update()
                except:
                    pass
                sub_name = _clean_text(sub.Name)
                sub_type = _clean_text(sub.HardwareType)
                for s in sub.Sensors:
                    try:
                        val = s.Value
                        if val is None:
                            continue
                        sensor_type = _clean_text(s.SensorType).lower()
                        unit_map = {"temperature":"C","load":"%","clock":"MHz","voltage":"V","fan":"RPM","power":"W","data":"GB","smalldata":"MB","throughput":"B/s","current":"A","energy":"Wh","noise":"dBA","control":"%","level":"%"}
                        unit = unit_map.get(sensor_type, "")
                        sensor_name = _clean_text(s.Name)
                        name = f"{sub_type}:{sub_name}:{sensor_name}"
                        label = f"{sub_name} {sensor_name}"
                        rows.append((
                            ts, hostname, sensor_type, name, label, float(val), unit,
                            json.dumps({"hardware": hw_name, "sub_hardware": sub_name, "hardware_type": sub_type, "identifier": _clean_text(s.Identifier), "sensor": sensor_name, "sensor_type": sensor_type, "value": float(val)})
                        ))
                    except:
                        continue
    except Exception as e:
        rows.append((ts, hostname, "error", "lhm_error", str(e)[:500], None, "", json.dumps({"error": str(e), "lhm_error": _LHM_ERROR})))
    return rows

def collect(hostname):
    ts = datetime.now(timezone.utc)
    rows = []

    # 1. LibreHardwareMonitor (máximo de dados - CPU per-core, GPU, memória, rede, etc)
    try:
        lhm_rows = _collect_lhm(hostname, ts)
        rows.extend(lhm_rows)
    except Exception:
        pass

    # 2. psutil fallback adicionais
    try:
        batt = psutil.sensors_battery()
        if batt:
            # evita duplicar se já veio do LHM
            if not any(r[1]==hostname and r[3]=="battery" for r in rows):
                rows.append((ts, hostname, "battery", "battery", "battery", float(batt.percent) if batt.percent else None, "%", json.dumps(batt._asdict())))
    except:
        pass

    # 3. WMI MSAcpi ThermalZone (se LHM não trouxe temps)
    has_temp = any(r[2]=="temperature" and r[5] not in (None, 0.0) for r in rows)
    if not has_temp:
        try:
            import wmi
            c = wmi.WMI(namespace="root\\WMI")
            for t in c.MSAcpi_ThermalZoneTemperature():
                kelvin = t.CurrentTemperature / 10.0
                celsius = kelvin - 273.15
                rows.append((ts, hostname, "temperature", "MSAcpi_ThermalZoneTemperature", getattr(t, 'InstanceName', 'thermal'), float(celsius), "C", json.dumps({"CurrentTemperature": t.CurrentTemperature})))
        except:
            pass

    # Se ainda sem rows, placeholder
    if not rows:
        if _LHM_ERROR:
            rows.append((ts, hostname, "error", "lhm_init_failed", _LHM_ERROR[:500], None, "", json.dumps({"error": _LHM_ERROR, "note": "LHM init failed, no sensors"})))
        else:
            rows.append((ts, hostname, "temperature", "none", "no_sensor", None, "C", json.dumps({"note": "no sensors via LHM/psutil/WMI"})))

    columns = ["ts","hostname","sensor_type","name","label","value","unit","raw"]
    return columns, rows
