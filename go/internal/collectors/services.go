package collectors

import (
	"encoding/json"
	"time"

	"github.com/yusufpapurcu/wmi"
)

// win32Service mirrors WMI Win32_Service minimal fields.
type win32Service struct {
	Name        string
	DisplayName string
	State       string
	StartMode   string
	ProcessId   uint32
	Status      string
}

// CollectServices mirrors collectors.psutil_collectors.collect_services via WMI.
func CollectServices(hostname string, ts time.Time) (Result, error) {
	cols := []string{"ts", "hostname", "name", "display_name", "status", "start_type", "pid", "raw"}
	var dst []win32Service
	err := wmi.Query("SELECT Name, DisplayName, State, StartMode, ProcessId, Status FROM Win32_Service", &dst)
	rows := [][]any{}
	if err == nil {
		for _, svc := range dst {
			var pid any
			if svc.ProcessId != 0 {
				pid = int64(svc.ProcessId)
			}
			raw, _ := json.Marshal(map[string]any{"Name": svc.Name, "State": svc.State, "StartMode": svc.StartMode, "Status": svc.Status})
			rows = append(rows, []any{
				ts, hostname, svc.Name, svc.DisplayName, svc.State, svc.StartMode, pid, json.RawMessage(raw),
			})
		}
	}
	return Result{Table: "monitor.services", Columns: cols, Rows: rows}, nil
}
