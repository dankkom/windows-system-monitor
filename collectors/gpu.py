import json
import subprocess
import shutil
from datetime import datetime, timezone

def _nvidia_smi_query():
    if not shutil.which("nvidia-smi"):
        return []
    # Query CSV
    fields = "index,name,uuid,driver_version,utilization.gpu,utilization.memory,utilization.encoder,utilization.decoder,memory.total,memory.used,memory.free,temperature.gpu,temperature.memory,power.draw,power.limit,fan.speed,clocks.current.graphics,clocks.current.memory,clocks.current.sm,pcie.tx,pcie.rx"
    # pcie fields may not exist on some drivers, handle fallback
    try:
        cmd = ["nvidia-smi", f"--query-gpu={fields}", "--format=csv,noheader,nounits"]
        out = subprocess.check_output(cmd, text=True, timeout=10)
        return out.strip().splitlines()
    except Exception as e:
        # fallback without pcie
        fields2 = "index,name,uuid,driver_version,utilization.gpu,utilization.memory,memory.total,memory.used,memory.free,temperature.gpu,power.draw,power.limit,fan.speed,clocks.current.graphics,clocks.current.memory"
        try:
            cmd = ["nvidia-smi", f"--query-gpu={fields2}", "--format=csv,noheader,nounits"]
            out = subprocess.check_output(cmd, text=True, timeout=10)
            return out.strip().splitlines()
        except Exception as e2:
            return []

def _parse_val(v):
    v=v.strip()
    if v in ("N/A", "[N/A]", "", " "):
        return None
    try:
        if "." in v:
            return float(v)
        return int(v)
    except:
        return v

def collect(hostname):
    ts = datetime.now(timezone.utc)
    lines = _nvidia_smi_query()
    rows = []
    # Try pynvml as supplement for power/fan if needed
    for line in lines:
        if not line.strip():
            continue
        parts = [p.strip() for p in line.split(",")]
        # Need to map dynamically based on length
        # Expect up to 21 fields, handle variable
        def get(i):
            return parts[i] if i < len(parts) else "N/A"
        try:
            idx = int(_parse_val(get(0)) or 0)
            name = get(1)
            uuid = get(2)
            driver = get(3)
            # Map known positions; if fallback length 15, adjust
            if len(parts) >= 21:
                util_gpu = _parse_val(get(4))
                util_mem = _parse_val(get(5))
                util_enc = _parse_val(get(6))
                util_dec = _parse_val(get(7))
                mem_total = _parse_val(get(8))
                mem_used = _parse_val(get(9))
                mem_free = _parse_val(get(10))
                temp_gpu = _parse_val(get(11))
                temp_mem = _parse_val(get(12))
                power_draw = _parse_val(get(13))
                power_limit = _parse_val(get(14))
                fan = _parse_val(get(15))
                clock_g = _parse_val(get(16))
                clock_m = _parse_val(get(17))
                clock_sm = _parse_val(get(18))
                pcie_tx = _parse_val(get(19))
                pcie_rx = _parse_val(get(20))
            elif len(parts) >= 15:
                util_gpu = _parse_val(get(4))
                util_mem = _parse_val(get(5))
                mem_total = _parse_val(get(6))
                mem_used = _parse_val(get(7))
                mem_free = _parse_val(get(8))
                temp_gpu = _parse_val(get(9))
                power_draw = _parse_val(get(10))
                power_limit = _parse_val(get(11))
                fan = _parse_val(get(12))
                clock_g = _parse_val(get(13))
                clock_m = _parse_val(get(14))
                util_enc = util_dec = temp_mem = clock_sm = pcie_tx = pcie_rx = None
            else:
                continue
            # bytes: memory values are MiB
            def mib_to_bytes(v):
                return int(v * 1024 * 1024) if isinstance(v, (int,float)) else None
            rows.append((
                ts, hostname, idx, name, uuid, driver,
                float(util_gpu) if isinstance(util_gpu,(int,float)) else None,
                float(util_mem) if isinstance(util_mem,(int,float)) else None,
                float(util_enc) if isinstance(util_enc,(int,float)) else None,
                float(util_dec) if isinstance(util_dec,(int,float)) else None,
                mib_to_bytes(mem_total), mib_to_bytes(mem_used), mib_to_bytes(mem_free),
                float(temp_gpu) if isinstance(temp_gpu,(int,float)) else None,
                float(temp_mem) if isinstance(temp_mem,(int,float)) else None,
                float(power_draw) if isinstance(power_draw,(int,float)) else None,
                float(power_limit) if isinstance(power_limit,(int,float)) else None,
                float(fan) if isinstance(fan,(int,float)) else None,
                float(clock_g) if isinstance(clock_g,(int,float)) else None,
                float(clock_m) if isinstance(clock_m,(int,float)) else None,
                float(clock_sm) if isinstance(clock_sm,(int,float)) else None,
                int(pcie_tx) if isinstance(pcie_tx,(int,float)) else None,
                int(pcie_rx) if isinstance(pcie_rx,(int,float)) else None,
                json.dumps({"raw_line": line})
            ))
        except Exception as e:
            rows.append((ts, hostname, 0, "error", None, None, None,None,None,None,None,None,None,None,None,None,None,None,None,None,None,None,None, json.dumps({"error": str(e), "line": line})))
    # Fallback try pynvml if no rows but nvidia-smi missing
    if not rows:
        try:
            import pynvml
            pynvml.nvmlInit()
            count = pynvml.nvmlDeviceGetCount()
            for i in range(count):
                handle = pynvml.nvmlDeviceGetHandleByIndex(i)
                name = pynvml.nvmlDeviceGetName(handle)
                if isinstance(name, bytes): name = name.decode()
                uuid = pynvml.nvmlDeviceGetUUID(handle)
                if isinstance(uuid, bytes): uuid = uuid.decode()
                util = pynvml.nvmlDeviceGetUtilizationRates(handle)
                mem = pynvml.nvmlDeviceGetMemoryInfo(handle)
                temp = pynvml.nvmlDeviceGetTemperature(handle, pynvml.NVML_TEMPERATURE_GPU)
                power = pynvml.nvmlDeviceGetPowerUsage(handle) / 1000.0
                rows.append((ts, hostname, i, name, uuid, None, float(util.gpu), float(util.memory), None,None, int(mem.total), int(mem.used), int(mem.free), float(temp), None, float(power), None, None, None,None,None,None,None, json.dumps({"source":"pynvml"})))
        except Exception:
            pass
    columns = ["ts","hostname","gpu_index","name","uuid","driver_version","utilization_gpu_percent","utilization_memory_percent","utilization_encoder_percent","utilization_decoder_percent","memory_total_bytes","memory_used_bytes","memory_free_bytes","temperature_gpu_c","temperature_memory_c","power_draw_w","power_limit_w","fan_speed_percent","clock_graphics_mhz","clock_memory_mhz","clock_sm_mhz","pcie_tx_bytes","pcie_rx_bytes","raw"]
    return columns, rows
