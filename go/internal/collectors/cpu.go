package collectors

import (
	"encoding/json"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
)

// CollectCPU mirrors collectors.psutil_collectors.collect_cpu.
func CollectCPU(hostname string, ts time.Time) (Result, error) {
	total, _ := cpu.Percent(0, false)
	perCore, _ := cpu.Percent(0, true)
	freqs, _ := cpu.Info()
	logical, _ := cpu.Counts(true)
	physical, _ := cpu.Counts(false)
	times, _ := cpu.Times(false)

	var totalPct *float64
	if len(total) > 0 {
		totalPct = ptr(total[0])
	}

	var cur *float64
	if len(freqs) > 0 {
		cur = ptr(freqs[0].Mhz)
	}

	var load1m *float64
	if l, err := load.Avg(); err == nil {
		load1m = ptr(l.Load1)
	}

	raw := map[string]any{
		"freq_count": len(freqs),
		"core_count": len(perCore),
		"times":      len(times),
	}
	rawJSON, _ := json.Marshal(raw)

	return Result{
		Table: "monitor.cpu",
		Columns: []string{
			"ts", "hostname", "cpu_total_percent", "cpu_per_core_percent",
			"cpu_count_logical", "cpu_count_physical",
			"freq_current_mhz", "freq_min_mhz", "freq_max_mhz",
			"load_1m", "ctx_switches", "interrupts", "raw",
		},
		Rows: [][]any{{
			ts, hostname,
			totalPct, perCore,
			logical, physical,
			cur, nil, nil,
			load1m, nil, nil,
			json.RawMessage(rawJSON),
		}},
	}, nil
}
