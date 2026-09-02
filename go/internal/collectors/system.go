package collectors

import (
	"encoding/json"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// CollectSystem mirrors collectors.system.collect.
func CollectSystem(hostname string, ts time.Time) (Result, error) {
	boot, _ := host.BootTime()
	var bootTS any
	var uptime *int64
	if boot > 0 {
		bt := time.Unix(int64(boot), 0).UTC()
		bootTS = bt
		u := int64(ts.Sub(bt).Seconds())
		uptime = ptr(u)
	}
	hi, _ := host.Info()
	var osName, osVersion, osBuild, kernelArch any
	var usersJSON json.RawMessage
	if hi != nil {
		osName = hi.OS
		osVersion = hi.PlatformVersion
		kernelArch = hi.KernelArch
		_ = osBuild
	}
	// fallback via runtime
	if osName == nil || osName == "" {
		osName = runtime.GOOS
	}
	if kernelArch == nil || kernelArch == "" {
		kernelArch = runtime.GOARCH
	}
	vm, _ := mem.VirtualMemory()
	var totalRAM *int64
	if vm != nil {
		totalRAM = ptr(int64(vm.Total))
	}
	var phys, logical *int
	if c, err := cpu.Counts(false); err == nil {
		phys = ptr(c)
	}
	if c, err := cpu.Counts(true); err == nil {
		logical = ptr(c)
	}
	var cpuName any
	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		cpuName = infos[0].ModelName
	}
	// users / battery omitted (nullable)
	users := []string{}
	usersJSON, _ = json.Marshal([]any{})
	_ = users
	raw, _ := json.Marshal(map[string]any{
		"boot_time": bootTS,
		"host_info": hi,
	})
	// users is TEXT[]: pass as []string directly
	return Result{
		Table: "monitor.system_info",
		Columns: []string{
			"ts", "hostname", "boot_time", "uptime_seconds", "os_name", "os_version", "os_build", "arch",
			"manufacturer", "model", "total_ram_bytes", "cpu_name", "cpu_cores_physical", "cpu_cores_logical",
			"users", "logged_users", "battery_percent", "battery_secs_left", "battery_power_plugged", "raw",
		},
		Rows: [][]any{{
			ts, hostname,
			bootTS, uptime,
			osName, osVersion, nil, kernelArch,
			nil, nil,
			totalRAM,
			cpuName, phys, logical,
			[]string{},
			json.RawMessage(usersJSON),
			nil, nil, nil,
			json.RawMessage(raw),
		}},
	}, nil
}
