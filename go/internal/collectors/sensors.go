package collectors

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// lhmDumpPaths are candidate locations for the helper .NET binary.
var lhmDumpPaths = []string{
	`C:\tools\lhm-dump\lhm-dump.exe`,
	`.\lhm-dump.exe`,
	filepath.Join(os.Getenv("ProgramFiles"), "system-monitor", "lhm-dump.exe"),
}

var lhmPyHelpers = []string{
	`C:\scripts\system-monitor\go\lhm-dump\lhm-dump.py`,
	`.\lhm-dump.py`,
}

var lhmPythonBins = []string{
	`C:\scripts\system-monitor\.venv\Scripts\python.exe`,
	`python`,
}

// CollectSensors mirrors collectors.sensors.collect via lhm-dump.exe helper.
// When the helper is absent, returns a single no_sensor row (like Python fallback).
func CollectSensors(hostname string, ts time.Time) (Result, error) {
	cols := []string{"ts", "hostname", "sensor_type", "name", "label", "value", "unit", "raw"}
	helper := findHelper()
	pyHelper := findPyHelper()
	var out []byte
	var err error
	if helper != "" {
		out, err = exec.Command(helper).Output()
		if err != nil && pyHelper != "" {
			// fallback to python helper on exe failure
			helper = ""
		}
	}
	if helper == "" && pyHelper != "" {
		pyBin := findPython()
		out, err = exec.Command(pyBin, pyHelper).Output()
		if err != nil {
			raw, _ := json.Marshal(map[string]any{"error": err.Error()})
			return Result{
				Table:   "monitor.sensors",
				Columns: cols,
				Rows:    [][]any{{ts, hostname, "no_sensor", "no_sensor", nil, nil, nil, json.RawMessage(raw)}},
			}, nil
		}
	} else if helper == "" {
		raw, _ := json.Marshal(map[string]any{"error": "lhm helper not found"})
		return Result{
			Table:   "monitor.sensors",
			Columns: cols,
			Rows:    [][]any{{ts, hostname, "no_sensor", "no_sensor", nil, nil, nil, json.RawMessage(raw)}},
		}, nil
	}
	if err != nil {
		raw, _ := json.Marshal(map[string]any{"error": err.Error()})
		return Result{
			Table:   "monitor.sensors",
			Columns: cols,
			Rows:    [][]any{{ts, hostname, "no_sensor", "no_sensor", nil, nil, nil, json.RawMessage(raw)}},
		}, nil
	}
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

func findPyHelper() string {
	for _, p := range lhmPyHelpers {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), "lhm-dump.py")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	// also search relative to go module root
	if _, err := os.Stat(`C:\scripts\system-monitor\go\lhm-dump\lhm-dump.py`); err == nil {
		return `C:\scripts\system-monitor\go\lhm-dump\lhm-dump.py`
	}
	return ""
}

func findPython() string {
	for _, p := range lhmPythonBins {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "python"
}
