package collectors

import (
	"encoding/json"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

// CollectMemory mirrors collectors.psutil_collectors.collect_memory.
func CollectMemory(hostname string, ts time.Time) (Result, error) {
	vm, _ := mem.VirtualMemory()
	swap, _ := mem.SwapMemory()

	if vm == nil {
		vm = &mem.VirtualMemoryStat{}
	}
	if swap == nil {
		swap = &mem.SwapMemoryStat{}
	}

	raw := map[string]any{
		"virtual": map[string]any{
			"total": vm.Total, "available": vm.Available, "used": vm.Used,
			"used_percent": vm.UsedPercent, "free": vm.Free, "active": vm.Active,
			"inactive": vm.Inactive, "buffers": vm.Buffers, "cached": vm.Cached,
			"shared": vm.Shared, "wired": vm.Wired, "slab": vm.Slab,
		},
		"swap": map[string]any{
			"total": swap.Total, "used": swap.Used, "free": swap.Free,
			"used_percent": swap.UsedPercent, "sin": swap.Sin, "sout": swap.Sout,
		},
	}
	rawJSON, _ := json.Marshal(raw)

	rows := [][]any{{
		ts, hostname,
		int64(vm.Total), int64(vm.Available), int64(vm.Used), float64(vm.UsedPercent),
		int64(vm.Free), int64(vm.Active), int64(vm.Inactive), int64(vm.Cached),
		int64(vm.Wired), int64(vm.Buffers), int64(vm.Shared), int64(vm.Slab),
		int64(swap.Total), int64(swap.Used), int64(swap.Free), float64(swap.UsedPercent),
		int64(swap.Sin), int64(swap.Sout),
		int64(swap.Total), int64(swap.Used),
		json.RawMessage(rawJSON),
	}}

	return Result{
		Table: "monitor.memory",
		Columns: []string{
			"ts", "hostname",
			"total_bytes", "available_bytes", "used_bytes", "used_percent",
			"free_bytes", "active_bytes", "inactive_bytes", "cached_bytes",
			"wired_bytes", "buffers_bytes", "shared_bytes", "slab_bytes",
			"swap_total_bytes", "swap_used_bytes", "swap_free_bytes", "swap_used_percent",
			"swap_sin", "swap_sout", "pagefile_total", "pagefile_used", "raw",
		},
		Rows: rows,
	}, nil
}
