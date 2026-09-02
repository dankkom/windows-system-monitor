package collectors

import (
	"encoding/json"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
)

// CollectConnections mirrors collectors.psutil_collectors.collect_connections.
func CollectConnections(hostname string, ts time.Time) (Result, error) {
	cols := []string{"ts", "hostname", "fd", "family", "type", "laddr_ip", "laddr_port", "raddr_ip", "raddr_port", "status", "pid", "raw"}
	conns, _ := gnet.Connections("inet")
	rows := [][]any{}
	for _, c := range conns {
		var laddrIP, laddrPort, raddrIP, raddrPort any
		if c.Laddr.IP != "" {
			laddrIP = c.Laddr.IP
			laddrPort = int64(c.Laddr.Port)
		}
		if c.Raddr.IP != "" {
			raddrIP = c.Raddr.IP
			raddrPort = int64(c.Raddr.Port)
		}
		var pid any
		if c.Pid != 0 {
			pid = int64(c.Pid)
		}
		var fd any
		if c.Fd != 0 {
			fd = int64(c.Fd)
		}
		raw, _ := json.Marshal(map[string]any{"family": c.Family, "type": c.Type, "status": c.Status})
		rows = append(rows, []any{
			ts, hostname, fd, strFamily(c.Family), strType(c.Type),
			laddrIP, laddrPort, raddrIP, raddrPort, c.Status, pid, json.RawMessage(raw),
		})
	}
	return Result{Table: "monitor.connections", Columns: cols, Rows: rows}, nil
}

func strFamily(f uint32) string {
	switch f {
	case 2:
		return "AF_INET"
	case 23:
		return "AF_INET6"
	default:
		return "AF_UNSPEC"
	}
}

func strType(t uint32) string {
	switch t {
	case 1:
		return "SOCK_STREAM"
	case 2:
		return "SOCK_DGRAM"
	default:
		return "UNKNOWN"
	}
}
