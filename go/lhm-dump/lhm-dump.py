#!/usr/bin/env python3
"""Helper LHM para Go collector - carrega LibreHardwareMonitor via pythonnet e imprime JSON."""
import json, os, sys
try:
    import clr
    dll = r"C:\tools\LibreHardwareMonitor\LibreHardwareMonitorLib.dll"
    if not os.path.exists(dll):
        dll = r"C:\tools\LibreHardwareMonitorLib.dll"
    clr.AddReference(dll)
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
    # update
    for hw in c.Hardware:
        hw.Update()
        for sub in hw.SubHardware:
            sub.Update()
    out=[]
    for hw in c.Hardware:
        for s in hw.Sensors:
            out.append({"sensor_type": str(s.SensorType).lower(), "name": s.Name, "label": str(s.Identifier), "value": float(s.Value) if s.Value is not None else None, "unit": "", "hardware": hw.Name})
        for sub in hw.SubHardware:
            for s in sub.Sensors:
                out.append({"sensor_type": str(s.SensorType).lower(), "name": s.Name, "label": str(s.Identifier), "value": float(s.Value) if s.Value is not None else None, "unit": "", "hardware": sub.Name})
    c.Close()
    json.dump(out, sys.stdout)
except Exception as e:
    json.dump([{"sensor_type":"error","name":str(e),"label":"","value":None,"unit":""}], sys.stdout)
    sys.exit(0)
