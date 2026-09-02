package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CollectPhysicalDisk mirrors collectors.disk_smart.collect_physical via PowerShell.
func CollectPhysicalDisk(hostname string, ts time.Time) (Result, error) {
	cols := []string{
		"ts", "hostname", "device_id", "friendly_name", "model", "serial_number", "firmware_version",
		"media_type", "bus_type", "health_status", "operational_status", "size_bytes", "allocated_size", "is_boot", "raw",
	}
	ps := `Get-PhysicalDisk | Select-Object DeviceId,FriendlyName,Model,SerialNumber,FirmwareVersion,MediaType,BusType,HealthStatus,OperationalStatus,Size,AllocatedSize,IsBoot | ConvertTo-Json -Depth 3`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).Output()
	if err != nil {
		raw, _ := json.Marshal(map[string]any{"error": err.Error()})
		return Result{Table: "monitor.physical_disk", Columns: cols, Rows: [][]any{{ts, hostname, "error", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, json.RawMessage(raw)}}}, nil
	}
	var data []map[string]any
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return Result{Table: "monitor.physical_disk", Columns: cols, Rows: nil}, nil
	}
	// PowerShell may return single object or array
	if strings.HasPrefix(trimmed, "{") {
		var single map[string]any
		if err := json.Unmarshal([]byte(trimmed), &single); err == nil {
			data = []map[string]any{single}
		}
	} else {
		_ = json.Unmarshal([]byte(trimmed), &data)
	}
	rows := [][]any{}
	for _, d := range data {
		size := toInt64(d["Size"])
		alloc := toInt64(d["AllocatedSize"])
		var isBoot any
		if v, ok := d["IsBoot"]; ok && v != nil {
			if b, ok := v.(bool); ok {
				isBoot = b
			}
		}
		raw, _ := json.Marshal(d)
		rows = append(rows, []any{
			ts, hostname,
			toStr(d["DeviceId"]), d["FriendlyName"], d["Model"],
			d["SerialNumber"], d["FirmwareVersion"],
			d["MediaType"], d["BusType"],
			d["HealthStatus"], d["OperationalStatus"],
			size, alloc, isBoot,
			json.RawMessage(raw),
		})
	}
	return Result{Table: "monitor.physical_disk", Columns: cols, Rows: rows}, nil
}

// CollectDiskSmart mirrors collectors.disk_smart.collect_smart via smartctl -j.
func CollectDiskSmart(hostname string, ts time.Time) (Result, error) {
	cols := []string{
		"ts", "hostname", "device", "model", "serial_number", "firmware_version", "device_type", "protocol", "smart_passed",
		"temperature_c", "power_on_hours", "power_cycle_count", "percentage_used", "available_spare", "available_spare_threshold",
		"media_errors", "reallocated_sectors", "pending_sectors", "host_reads", "host_writes", "data_units_read", "data_units_written",
		"total_lbas_read", "total_lbas_written", "unsafe_shutdowns", "raw",
	}
	smartctl := smartctlPath()
	if _, err := os.Stat(smartctl); err != nil {
		if _, err := exec.LookPath("smartctl"); err != nil {
			return Result{Table: "monitor.disk_smart", Columns: cols, Rows: nil}, nil
		}
		smartctl = "smartctl"
	}
	scanOut, _ := exec.Command(smartctl, "--scan").Output()
	devices := parseScan(string(scanOut))
	if len(devices) == 0 {
		devices = []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd"}
	}
	rows := [][]any{}
	for _, dev := range devices {
		out, err := exec.Command(smartctl, "-a", dev, "-j").Output()
		if err != nil {
			// smartctl returns non-zero on warnings; still has output
			if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) == 0 {
				// keep out if any
			} else if len(out) == 0 {
				continue
			}
		}
		if len(out) == 0 {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal(out, &data); err != nil {
			continue
		}
		if _, ok := data["device"]; !ok {
			continue
		}
		model := firstStr(data, "model_name", "model_family")
		serial := strVal(data["serial_number"])
		firmware := strVal(data["firmware_version"])
		var deviceType, protocol string
		if devInfo, ok := data["device"].(map[string]any); ok {
			deviceType = strVal(devInfo["type"])
			protocol = strVal(devInfo["protocol"])
		}
		var smartPassed any
		if ss, ok := data["smart_status"].(map[string]any); ok {
			smartPassed = ss["passed"]
		}
		var temp, powerOnHours, powerCycles, percentageUsed, availSpare, availThresh, mediaErrors, reallocated, pending, hostReads, hostWrites, dataUnitsRead, dataUnitsWritten, totalLbasRead, totalLbasWritten, unsafeShutdowns any
		if nvme, ok := data["nvme_smart_health_information_log"].(map[string]any); ok {
			temp = nvme["temperature"]
			powerOnHours = nvme["power_on_hours"]
			powerCycles = nvme["power_cycles"]
			percentageUsed = nvme["percentage_used"]
			availSpare = nvme["available_spare"]
			availThresh = nvme["available_spare_threshold"]
			mediaErrors = nvme["media_errors"]
			hostReads = nvme["host_reads"]
			hostWrites = nvme["host_writes"]
			dataUnitsRead = nvme["data_units_read"]
			dataUnitsWritten = nvme["data_units_written"]
			unsafeShutdowns = nvme["unsafe_shutdowns"]
		}
		if ata, ok := data["ata_smart_attributes"].(map[string]any); ok {
			if tbl, ok := ata["table"].([]any); ok {
				for _, e := range tbl {
					attr, _ := e.(map[string]any)
					idF, _ := attr["id"].(float64)
					name, _ := attr["name"].(string)
					rawMap, _ := attr["raw"].(map[string]any)
					var val any
					if rawMap != nil {
						val = rawMap["value"]
					}
					rstr := ""
					if rawMap != nil {
						rstr, _ = rawMap["string"].(string)
					}
					switch {
					case int(idF) == 9 && name == "Power_On_Hours":
						if rstr != "" {
							if v, err := parseFirstInt(rstr); err == nil {
								powerOnHours = v
							}
						} else if v, ok := val.(float64); ok {
							powerOnHours = int64(v)
						}
					case int(idF) == 12 && name == "Power_Cycle_Count":
						if rstr != "" {
							if v, err := parseFirstInt(rstr); err == nil {
								powerCycles = v
							}
						} else if v, ok := val.(float64); ok {
							powerCycles = int64(v)
						}
					case int(idF) == 5 && name == "Reallocated_Sector_Ct":
						if v, ok := val.(float64); ok {
							reallocated = int64(v)
						}
					case int(idF) == 197 && name == "Current_Pending_Sector":
						if v, ok := val.(float64); ok {
							pending = int64(v)
						}
					case int(idF) == 194 && strings.Contains(name, "Temperature"):
						if rstr != "" {
							if v, err := parseFirstInt(rstr); err == nil {
								temp = float64(v)
							}
						}
					case int(idF) == 241:
						if v, ok := val.(float64); ok {
							totalLbasWritten = int64(v)
						}
					case int(idF) == 242:
						if v, ok := val.(float64); ok {
							totalLbasRead = int64(v)
						}
					}
				}
				if temp == nil {
					for _, e := range tbl {
						attr, _ := e.(map[string]any)
						idF, _ := attr["id"].(float64)
						if int(idF) == 190 {
							if rawMap, ok := attr["raw"].(map[string]any); ok {
								if rstr, ok := rawMap["string"].(string); ok && rstr != "" {
									if v, err := parseFirstInt(rstr); err == nil {
										temp = float64(v)
									}
								}
							}
						}
					}
				}
			}
		}
		if temp == nil {
			if t, ok := data["temperature"].(map[string]any); ok {
				temp = t["current"]
			} else if v, ok := data["temperature"].(float64); ok {
				temp = v
			}
		}
		raw, _ := json.Marshal(data)
		rows = append(rows, []any{
			ts, hostname, dev, model, serial, firmware, deviceType, protocol, smartPassed,
			toFloatAny(temp), toIntAny(powerOnHours), toIntAny(powerCycles), toFloatAny(percentageUsed), toFloatAny(availSpare), toFloatAny(availThresh),
			toIntAny(mediaErrors), toIntAny(reallocated), toIntAny(pending), toIntAny(hostReads), toIntAny(hostWrites), toIntAny(dataUnitsRead), toIntAny(dataUnitsWritten),
			toIntAny(totalLbasRead), toIntAny(totalLbasWritten), toIntAny(unsafeShutdowns),
			json.RawMessage(raw),
		})
	}
	return Result{Table: "monitor.disk_smart", Columns: cols, Rows: rows}, nil
}

func smartctlPath() string {
	p := `C:\Program Files\smartmontools\bin\smartctl.exe`
	if _, err := os.Stat(p); err == nil {
		return p
	}
	if lp, err := exec.LookPath("smartctl"); err == nil {
		return lp
	}
	return filepath.Join(p)
}

func parseScan(out string) []string {
	var devs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			devs = append(devs, parts[0])
		}
	}
	return devs
}

func firstStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
func strVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func toInt64(v any) any {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		// PowerShell size comes as int
		return nil
	default:
		return nil
	}
}
func toStr(v any) any {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return nil
	}
	return v
}
func toFloatAny(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	default:
		return nil
	}
}
func toIntAny(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return nil
	}
}
func parseFirstInt(s string) (int64, error) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, fmt.Errorf("no integer in %q", s)
	}
	return parseInt(f[0])
}
func parseInt(s string) (int64, error) {
	// strip non-digit suffix/prefix
	i := 0
	for i < len(s) && (s[i] < '0' || s[i] > '9') && s[i] != '-' {
		i++
	}
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if i == j {
		return 0, fmt.Errorf("no integer in %q", s)
	}
	var n int64
	for _, c := range s[i:j] {
		n = n*10 + int64(c-'0')
	}
	return n, nil
}
