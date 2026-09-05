package collectors

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// lhmDumpPaths are candidate locations for the helper .NET Framework binary.
var lhmDumpPaths = []string{
	`C:\tools\lhm-dump\lhm-dump.exe`,
	`.\lhm-dump.exe`,
	filepath.Join(os.Getenv("ProgramFiles"), "system-monitor", "lhm-dump.exe"),
}

// CollectSensors via lhm-dump.exe helper.
// When helper absent or fails, returns single no_sensor row.
func CollectSensors(hostname string, ts time.Time) (Result, error) {
	cols := []string{"ts", "hostname", "sensor_type", "name", "label", "value", "unit", "raw"}
	helper := findHelper()
	if helper == "" {
		raw, _ := json.Marshal(map[string]any{"error": "lhm-dump.exe not found (C:\\tools\\lhm-dump)"})
		return Result{
			Table:   "monitor.sensors",
			Columns: cols,
			Rows:    [][]any{{ts, hostname, "no_sensor", "no_sensor", nil, nil, nil, json.RawMessage(raw)}},
		}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, helper).Output()
	if err != nil {
		raw, _ := json.Marshal(map[string]any{"error": err.Error()})
		return Result{
			Table:   "monitor.sensors",
			Columns: cols,
			Rows:    [][]any{{ts, hostname, "no_sensor", "no_sensor", nil, nil, nil, json.RawMessage(raw)}},
		}, nil
	}
	var sensors []map[string]any
	if err := json.Unmarshal(out, &sensors); err != nil {
		raw, _ := json.Marshal(map[string]any{"error": "invalid lhm-dump output: " + err.Error()})
		return Result{
			Table:   "monitor.sensors",
			Columns: cols,
			Rows:    [][]any{{ts, hostname, "no_sensor", "no_sensor", nil, nil, nil, json.RawMessage(raw)}},
		}, nil
	}
	rows := [][]any{}
	for _, s := range sensors {
		stype, _ := s["sensor_type"].(string)
		name, _ := s["name"].(string)
		label, _ := s["label"].(string)
		var labelAny any
		if label != "" {
			labelAny = label
		}
		var value any
		if v, ok := s["value"]; ok {
			value = v
		}
		unit, _ := s["unit"].(string)
		var unitAny any
		if unit != "" {
			unitAny = unit
		}
		raw, _ := json.Marshal(s)
		if stype == "" {
			stype = "unknown"
		}
		if name == "" {
			name = "unknown"
		}
		rows = append(rows, []any{ts, hostname, stype, name, labelAny, value, unitAny, json.RawMessage(raw)})
	}
	if len(rows) == 0 {
		raw, _ := json.Marshal(map[string]any{"info": "no sensors returned"})
		rows = [][]any{{ts, hostname, "no_sensor", "no_sensor", nil, nil, nil, json.RawMessage(raw)}}
	}
	return Result{Table: "monitor.sensors", Columns: cols, Rows: rows}, nil
}

func findHelper() string {
	for _, p := range lhmDumpPaths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// also check next to executable
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), "lhm-dump.exe")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return ""
}
