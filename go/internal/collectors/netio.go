package collectors

import (
	"encoding/json"
	"net"
	"strings"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
)

// CollectNetIO mirrors collectors.psutil_collectors.collect_net_io.
func CollectNetIO(hostname string, ts time.Time) (Result, error) {
	counters, _ := gnet.IOCounters(true)
	ifaces, _ := gnet.Interfaces()
	ifaceStats := map[string]gnet.InterfaceStat{}
	for i := range ifaces {
		ifaceStats[ifaces[i].Name] = ifaces[i]
	}

	cols := []string{
		"ts", "hostname", "iface",
		"bytes_sent", "bytes_recv", "packets_sent", "packets_recv",
		"errin", "errout", "dropin", "dropout",
		"speed_mbps", "is_up", "mtu", "raw",
	}
	rows := [][]any{}
	for _, c := range counters {
		st, ok := ifaceStats[c.Name]
		var isUp *bool
		var mtuI any // schema: mtu INT (nullable)
		if ok {
			curUp := false
			for _, f := range st.Flags {
				if strings.EqualFold(f, "up") {
					curUp = true
					break
				}
			}
			isUp = ptr(curUp)
			if st.MTU > 0 {
				mtuI = int64(st.MTU)
			}
		}
		raw, _ := json.Marshal(map[string]any{
			"counters": map[string]any{
				"bytes_sent": c.BytesSent, "bytes_recv": c.BytesRecv,
				"packets_sent": c.PacketsSent, "packets_recv": c.PacketsRecv,
				"errin": c.Errin, "errout": c.Errout,
				"dropin": c.Dropin, "dropout": c.Dropout,
			},
		})
		rows = append(rows, []any{
			ts, hostname, c.Name,
			int64(c.BytesSent), int64(c.BytesRecv),
			int64(c.PacketsSent), int64(c.PacketsRecv),
			int64(c.Errin), int64(c.Errout),
			int64(c.Dropin), int64(c.Dropout),
			nil, isUp, mtuI,
			json.RawMessage(raw),
		})
	}
	return Result{Table: "monitor.net_io", Columns: cols, Rows: rows}, nil
}

// CollectNetAddrs mirrors collectors.psutil_collectors.collect_net_addrs.
func CollectNetAddrs(hostname string, ts time.Time) (Result, error) {
	ifaces, _ := gnet.Interfaces()
	cols := []string{"ts", "hostname", "iface", "family", "address", "netmask", "broadcast", "raw"}
	rows := [][]any{}
	for _, iface := range ifaces {
		for _, a := range iface.Addrs {
			family := addrFamily(a.Addr)
			var netmask, broadcast any
			if ip, ipnet, err := net.ParseCIDR(a.Addr); err == nil && ip.To4() != nil {
				netmask = net.IP(ipnet.Mask).String()
				broadcast = broadcastAddr(ipnet)
			}
			raw, _ := json.Marshal(map[string]any{"family": family, "address": a.Addr})
			rows = append(rows, []any{
				ts, hostname, iface.Name, family, a.Addr, netmask, broadcast, json.RawMessage(raw),
			})
		}
	}
	return Result{Table: "monitor.net_addr", Columns: cols, Rows: rows}, nil
}

func addrFamily(addr string) any {
	if strings.Contains(addr, ":") {
		return "AF_INET6"
	}
	return "AF_INET"
}

func broadcastAddr(n *net.IPNet) any {
	ip := n.IP.To4()
	if ip == nil {
		return nil
	}
	bcast := make(net.IP, len(ip))
	for i := range ip {
		bcast[i] = ip[i] | ^n.Mask[i]
	}
	return bcast.String()
}
