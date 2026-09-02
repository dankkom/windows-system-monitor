package collectors

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// CollectProcesses mirrors collectors.processes.collect (top N by cpu+mem).
func CollectProcesses(hostname string, ts time.Time, topN int) (Result, error) {
	pids, _ := process.Pids()
	type rec struct {
		row []any
		sum float64
	}
	var recs []rec
	for _, pid := range pids {
		p, err := process.NewProcess(pid)
		if err != nil {
			continue
		}
		name, _ := p.Name()
		ppid, _ := p.Ppid()
		exe, _ := p.Exe()
		cmdline, _ := p.Cmdline()
		if len(cmdline) > 2000 {
			cmdline = cmdline[:2000]
		}
		username, _ := p.Username()
		status, _ := p.Status()
		statusStr := strings.Join(status, ",")
		cpuPct, _ := p.CPUPercent()
		memPct, _ := p.MemoryPercent()
		var rss, vms, shared *int64
		if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			rss = ptr(int64(mi.RSS))
			vms = ptr(int64(mi.VMS))
			// gopsutil v4 MemInfo has no Shared on Windows; leave nil
			_ = shared
		}
		var numThreads *int64
		if n, err := p.NumThreads(); err == nil {
			numThreads = ptr(int64(n))
		}
		var numHandles *int64 // NumHandles not available in gopsutil v4; leave nil
		var ioR, ioW, ioRC, ioWC *int64
		if io, err := p.IOCounters(); err == nil && io != nil {
			ioR = ptr(int64(io.ReadBytes))
			ioW = ptr(int64(io.WriteBytes))
			ioRC = ptr(int64(io.ReadCount))
			ioWC = ptr(int64(io.WriteCount))
		}
		var createTS any
		if ct, err := p.CreateTime(); err == nil && ct > 0 {
			createTS = time.UnixMilli(ct).UTC()
		}
		cwd, _ := p.Cwd()
		raw, _ := json.Marshal(map[string]any{"pid": pid, "name": name})
		sum := cpuPct + float64(memPct)
		row := []any{
			ts, hostname,
			int64(pid), int64(ppid),
			name, exe, cmdline, username, statusStr,
			cpuPct, float64(memPct),
			rss, vms, nil,
			numThreads, numHandles, nil,
			ioR, ioW, ioRC, ioWC,
			createTS, cwd,
			json.RawMessage(raw),
		}
		// normalize nil pointers to nil any
		for i, v := range row {
			if isNilPtr(v) {
				row[i] = nil
			}
		}
		recs = append(recs, rec{row: row, sum: sum})
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].sum > recs[j].sum })
	if topN > 0 && len(recs) > topN {
		recs = recs[:topN]
	}
	rows := make([][]any, 0, len(recs))
	for _, r := range recs {
		rows = append(rows, r.row)
	}
	return Result{
		Table: "monitor.processes",
		Columns: []string{
			"ts", "hostname", "pid", "ppid", "name", "exe", "cmdline", "username", "status",
			"cpu_percent", "memory_percent", "memory_rss_bytes", "memory_vms_bytes", "memory_shared_bytes",
			"num_threads", "num_handles", "num_fds",
			"io_read_bytes", "io_write_bytes", "io_read_count", "io_write_count",
			"create_time", "cwd", "raw",
		},
		Rows: rows,
	}, nil
}

func isNilPtr(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case *int64:
		return t == nil
	case *float64:
		return t == nil
	case *bool:
		return t == nil
	default:
		return false
	}
}
