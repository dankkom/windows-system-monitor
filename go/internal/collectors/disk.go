package collectors

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

// normalizeDevice strips trailing backslash from Windows device/mount paths so
// "C:" and "C:\" collapse to a single partition (avoids duplicate disks).
func normalizeDevice(s string) string {
	return strings.TrimRight(s, `\`)
}

// CollectDiskIO mirrors collectors.psutil_collectors.collect_disk_io.
func CollectDiskIO(hostname string, ts time.Time) (Result, error) {
	counters, _ := disk.IOCounters()
	cols := []string{
		"ts", "hostname", "device",
		"read_count", "write_count", "read_bytes", "write_bytes",
		"read_time_ms", "write_time_ms", "busy_time_ms", "raw",
	}
	rows := [][]any{}
	for dev, c := range counters {
		raw, _ := json.Marshal(map[string]any{
			"read_count": c.ReadCount, "write_count": c.WriteCount,
			"read_bytes": c.ReadBytes, "write_bytes": c.WriteBytes,
			"read_time": c.ReadTime, "write_time": c.WriteTime,
			"busy_time": c.IoTime,
		})
		rows = append(rows, []any{
			ts, hostname, normalizeDevice(dev),
			int64(c.ReadCount), int64(c.WriteCount),
			int64(c.ReadBytes), int64(c.WriteBytes),
			int64(c.ReadTime), int64(c.WriteTime),
			int64(c.IoTime),
			json.RawMessage(raw),
		})
	}
	return Result{Table: "monitor.disk_io", Columns: cols, Rows: rows}, nil
}

// CollectDiskUsage mirrors collectors.psutil_collectors.collect_disk_usage.
func CollectDiskUsage(hostname string, ts time.Time) (Result, error) {
	parts, _ := disk.Partitions(false)
	cols := []string{
		"ts", "hostname", "device", "mountpoint", "fstype",
		"total_bytes", "used_bytes", "free_bytes", "used_percent", "raw",
	}
	rows := [][]any{}
	for _, p := range parts {
		dev := normalizeDevice(p.Device)
		mnt := normalizeDevice(p.Mountpoint)
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil || usage == nil {
			raw, _ := json.Marshal(map[string]any{"error": errText(err)})
			rows = append(rows, []any{
				ts, hostname, dev, mnt, p.Fstype,
				nil, nil, nil, nil, json.RawMessage(string(raw)),
			})
			continue
		}
		raw, _ := json.Marshal(map[string]any{
			"opts": p.Opts,
			"usage": map[string]any{
				"total": usage.Total, "used": usage.Used, "free": usage.Free,
				"used_percent": usage.UsedPercent,
			},
		})
		rows = append(rows, []any{
			ts, hostname, dev, mnt, p.Fstype,
			int64(usage.Total), int64(usage.Used), int64(usage.Free),
			float64(usage.UsedPercent), json.RawMessage(raw),
		})
	}
	return Result{Table: "monitor.disk_usage", Columns: cols, Rows: rows}, nil
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
