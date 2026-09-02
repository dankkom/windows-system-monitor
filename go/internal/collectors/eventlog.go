package collectors

import (
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
)

// CollectEventLog mirrors collectors.system.collect_eventlog.
// Uses wevtutil to summarize recent System/Application logs (last hour).
func CollectEventLog(hostname string, ts time.Time) (Result, error) {
	cols := []string{"ts", "hostname", "log_name", "level", "event_id", "provider", "count", "latest_message", "raw"}
	rows := [][]any{}
	for _, logName := range []string{"System", "Application"} {
		// Query last 1h via wevtutil; fallback to empty if unavailable
		out, err := exec.Command("wevtutil", "qe", logName, "/q:*[System[TimeCreated[timediff(@SystemTime) <= 3600000]]]", "/c:200", "/f:text").Output()
		if err != nil || len(out) == 0 {
			continue
		}
		// wevtutil emits OEM codepage (often cp850/cp1252) -> decode as Windows-1252 then sanitize
		decoded, _ := charmap.Windows1252.NewDecoder().Bytes(out)
		sanitized := strings.ToValidUTF8(string(decoded), "�")
		if len(decoded) == 0 {
			sanitized = strings.ToValidUTF8(string(out), "�")
		}
		sanitized = strings.ReplaceAll(sanitized, "\x00", "")
		counts := map[string]int{}
		latest := map[string]string{}
		// Simple parse: each event block separated by blank line; grab Event ID and Level
		blocks := strings.Split(sanitized, "\n\n")
		for _, b := range blocks {
			if strings.TrimSpace(b) == "" {
				continue
			}
			var eid, level, provider string
			for _, line := range strings.Split(b, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "Event ID:") {
					eid = strings.TrimSpace(strings.TrimPrefix(line, "Event ID:"))
				} else if strings.HasPrefix(line, "Level:") {
					level = strings.TrimSpace(strings.TrimPrefix(line, "Level:"))
				} else if strings.HasPrefix(line, "Provider:") {
					provider = strings.TrimSpace(strings.TrimPrefix(line, "Provider:"))
				}
			}
			if eid == "" {
				continue
			}
			key := logName + "|" + level + "|" + eid + "|" + provider
			counts[key]++
			if _, ok := latest[key]; !ok {
				latest[key] = truncate(b, 500)
			}
			if len(counts) > 50 {
				break
			}
		}
		for key, cnt := range counts {
			parts := strings.SplitN(key, "|", 4)
			provider := ""
			if len(parts) == 4 {
				provider = parts[3]
			}
			var eid any
			if parts[2] != "" {
				// eid numeric if possible
				var v int64
				for _, c := range parts[2] {
					if c >= '0' && c <= '9' {
						v = v*10 + int64(c-'0')
					}
				}
				eid = v
			}
			level := parts[1]
			if level == "" {
				level = "Information"
			}
			raw, _ := json.Marshal(map[string]any{"count": cnt})
			rows = append(rows, []any{
				ts, hostname, logName, level, eid, provider, int64(cnt), latest[key], json.RawMessage(raw),
			})
		}
	}
	if len(rows) == 0 {
		raw, _ := json.Marshal(map[string]any{"info": "no recent events or wevtutil unavailable"})
		rows = append(rows, []any{ts, hostname, "System", "Information", nil, "collector", int64(0), "no events", json.RawMessage(raw)})
	}
	return Result{Table: "monitor.eventlog", Columns: cols, Rows: rows}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
