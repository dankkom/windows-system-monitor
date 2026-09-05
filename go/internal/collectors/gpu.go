package collectors

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CollectGPU mirrors collectors.gpu.collect via nvidia-smi subprocess.
func CollectGPU(hostname string, ts time.Time) (Result, error) {
	cols := []string{
		"ts", "hostname", "gpu_index", "name", "uuid", "driver_version",
		"utilization_gpu_percent", "utilization_memory_percent", "utilization_encoder_percent", "utilization_decoder_percent",
		"memory_total_bytes", "memory_used_bytes", "memory_free_bytes",
		"temperature_gpu_c", "temperature_memory_c", "power_draw_w", "power_limit_w", "fan_speed_percent",
		"clock_graphics_mhz", "clock_memory_mhz", "clock_sm_mhz", "pcie_tx_bytes", "pcie_rx_bytes", "raw",
	}
	lines := nvidiaSMIQuery()
	rows := [][]any{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := splitCSV(line)
		get := func(i int) string {
			if i < len(parts) {
				return strings.TrimSpace(parts[i])
			}
			return "N/A"
		}
		parse := func(v string) any {
			v = strings.TrimSpace(v)
			if v == "N/A" || v == "[N/A]" || v == "" {
				return nil
			}
			if strings.Contains(v, ".") {
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					return f
				}
			}
			if iv, err := strconv.ParseInt(v, 10, 64); err == nil {
				return iv
			}
			return v
		}
		mibToBytes := func(v any) any {
			switch t := v.(type) {
			case int64:
				return t * 1024 * 1024
			case float64:
				return int64(t * 1024 * 1024)
			case int:
				return int64(t) * 1024 * 1024
			default:
				return nil
			}
		}
		toFloat := func(v any) any {
			switch t := v.(type) {
			case int64:
				return float64(t)
			case float64:
				return t
			case int:
				return float64(t)
			default:
				return nil
			}
		}
		toInt := func(v any) any {
			switch t := v.(type) {
			case int64:
				return t
			case float64:
				return int64(t)
			case int:
				return int64(t)
			default:
				return nil
			}
		}
		var idx int64
		if v := parse(get(0)); v != nil {
			switch t := v.(type) {
			case int64:
				idx = t
			case float64:
				idx = int64(t)
			}
		}
		name := get(1)
		uuid := get(2)
		driver := get(3)
		var utilGPU, utilMem, utilEnc, utilDec, memTotal, memUsed, memFree, tempGPU, tempMem, powerDraw, powerLimit, fan, clockG, clockM, clockSM, pcieTx, pcieRx any
		if len(parts) >= 21 {
			utilGPU = parse(get(4))
			utilMem = parse(get(5))
			utilEnc = parse(get(6))
			utilDec = parse(get(7))
			memTotal = parse(get(8))
			memUsed = parse(get(9))
			memFree = parse(get(10))
			tempGPU = parse(get(11))
			tempMem = parse(get(12))
			powerDraw = parse(get(13))
			powerLimit = parse(get(14))
			fan = parse(get(15))
			clockG = parse(get(16))
			clockM = parse(get(17))
			clockSM = parse(get(18))
			pcieTx = parse(get(19))
			pcieRx = parse(get(20))
		} else if len(parts) >= 15 {
			utilGPU = parse(get(4))
			utilMem = parse(get(5))
			memTotal = parse(get(6))
			memUsed = parse(get(7))
			memFree = parse(get(8))
			tempGPU = parse(get(9))
			powerDraw = parse(get(10))
			powerLimit = parse(get(11))
			fan = parse(get(12))
			clockG = parse(get(13))
			clockM = parse(get(14))
		} else {
			continue
		}
		raw, _ := json.Marshal(map[string]any{"raw_line": line})
		rows = append(rows, []any{
			ts, hostname, idx, name, uuid, driver,
			toFloat(utilGPU), toFloat(utilMem), toFloat(utilEnc), toFloat(utilDec),
			mibToBytes(memTotal), mibToBytes(memUsed), mibToBytes(memFree),
			toFloat(tempGPU), toFloat(tempMem), toFloat(powerDraw), toFloat(powerLimit), toFloat(fan),
			toFloat(clockG), toFloat(clockM), toFloat(clockSM), toInt(pcieTx), toInt(pcieRx),
			json.RawMessage(raw),
		})
	}
	return Result{Table: "monitor.gpu", Columns: cols, Rows: rows}, nil
}

func findNvidiaSMI() string {
	if p, err := exec.LookPath("nvidia-smi"); err == nil {
		return p
	}
	candidates := []string{
		`C:\Program Files\NVIDIA Corporation\NVSMI\nvidia-smi.exe`,
		`C:\Windows\System32\nvidia-smi.exe`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func nvidiaSMIQuery() []string {
	bin := findNvidiaSMI()
	if bin == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fields := "index,name,uuid,driver_version,utilization.gpu,utilization.memory,utilization.encoder,utilization.decoder,memory.total,memory.used,memory.free,temperature.gpu,temperature.memory,power.draw,power.limit,fan.speed,clocks.current.graphics,clocks.current.memory,clocks.current.sm,pcie.tx,pcie.rx"
	if out, err := exec.CommandContext(ctx, bin, "--query-gpu="+fields, "--format=csv,noheader,nounits").Output(); err == nil {
		return strings.Split(strings.TrimSpace(string(out)), "\n")
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	fields2 := "index,name,uuid,driver_version,utilization.gpu,utilization.memory,memory.total,memory.used,memory.free,temperature.gpu,power.draw,power.limit,fan.speed,clocks.current.graphics,clocks.current.memory"
	if out, err := exec.CommandContext(ctx2, bin, "--query-gpu="+fields2, "--format=csv,noheader,nounits").Output(); err == nil {
		return strings.Split(strings.TrimSpace(string(out)), "\n")
	}
	return nil
}

func splitCSV(s string) []string {
	return strings.Split(s, ",")
}
