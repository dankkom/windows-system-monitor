// Package config loads runtime settings from config.toml and environment.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Settings holds validated runtime configuration.
type Settings struct {
	DatabaseURL        string
	Hostname           string
	ConnectTimeout     time.Duration
	RetrySeconds       time.Duration
	BufferMaxBytes     int64
	DashboardHost      string
	DashboardPort      int
	DashboardTimezone  string
	PowerAuxBaselineW  float64
	PowerPSUEfficiency float64
	PowerGPUIdleW      *float64
	PowerGPUMaxW       *float64
	Intervals          map[string]time.Duration
	TopProcesses       int
	BufferPath         string
	LogDir             string
	EnableRetention    bool
	Retention          map[string]string
	RetentionBatch     int
	RetentionSleep     time.Duration
	ConfigPath         string
}

// fileConfig mirrors config.toml structure.
type fileConfig struct {
	DB struct {
		URL            string `toml:"url"`
		ConnectTimeout int    `toml:"connect_timeout"`
		RetrySeconds   int    `toml:"retry_seconds"`
		BufferMaxBytes int64  `toml:"buffer_max_bytes"`
	} `toml:"db"`
	Dashboard struct {
		Host     string `toml:"host"`
		Port     int    `toml:"port"`
		Timezone string `toml:"timezone"`
	} `toml:"dashboard"`
	Power struct {
		AuxBaselineW  float64  `toml:"aux_baseline_w"`
		PSUEfficiency float64  `toml:"psu_efficiency"`
		GPUIdleW      *float64 `toml:"gpu_idle_w"`
		GPUMaxW       *float64 `toml:"gpu_max_w"`
	} `toml:"power"`
	Retention struct {
		Enabled      bool    `toml:"enabled"`
		BatchLimit   int     `toml:"batch_limit"`
		BatchSleep   float64 `toml:"batch_sleep"`
		Processes    string  `toml:"processes"`
		Connections  string  `toml:"connections"`
		Sensors      string  `toml:"sensors"`
		CPU          string  `toml:"cpu"`
		Memory       string  `toml:"memory"`
		GPU          string  `toml:"gpu"`
		Heartbeat    string  `toml:"heartbeat"`
		Eventlog     string  `toml:"eventlog"`
		DiskIO       string  `toml:"disk_io"`
		NetIO        string  `toml:"net_io"`
	} `toml:"retention"`
	Intervals struct {
		CPU          int `toml:"cpu"`
		Memory       int `toml:"memory"`
		DiskIO       int `toml:"disk_io"`
		DiskUsage    int `toml:"disk_usage"`
		DiskPhysical int `toml:"disk_physical"`
		DiskSmart    int `toml:"disk_smart"`
		Network      int `toml:"network"`
		GPU          int `toml:"gpu"`
		Sensors      int `toml:"sensors"`
		Processes    int `toml:"processes"`
		Connections  int `toml:"connections"`
		Services     int `toml:"services"`
		System       int `toml:"system"`
		Eventlog     int `toml:"eventlog"`
	} `toml:"intervals"`
	Collector struct {
		TopProcesses int    `toml:"top_processes"`
		Hostname     string `toml:"hostname"`
	} `toml:"collector"`
	Log struct {
		Level string `toml:"level"`
	} `toml:"log"`
}

// RequireDatabaseURL resolves and validates DATABASE_URL.
func (s *Settings) RequireDatabaseURL() (string, error) {
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		url = strings.TrimSpace(s.DatabaseURL)
	}
	if !strings.HasPrefix(url, "postgresql://") && !strings.HasPrefix(url, "postgres://") {
		return "", errors.New("DATABASE_URL must be a PostgreSQL URL (e.g. postgresql://user:pass@host:5432/db)")
	}
	return url, nil
}

func positiveInt(name string, def int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func positiveInt64(name string, def int64) int64 {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func nonnegativeFloat(name string, def float64, hasDefault bool) (float64, bool) {
	raw := os.Getenv(name)
	if strings.TrimSpace(raw) == "" {
		if hasDefault {
			return def, true
		}
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v < 0 {
		return def, hasDefault
	}
	return v, true
}

// baseDir resolves the project/install root by searching for config.toml upwards.
func baseDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	candidates := []string{wd, filepath.Join(wd, ".."), filepath.Join(wd, "..", "..")}
	for _, cand := range candidates {
		if _, err := os.Stat(filepath.Join(cand, "config.toml")); err == nil {
			abs, _ := filepath.Abs(cand)
			return abs
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, cand := range []string{dir, filepath.Join(dir, ".."), filepath.Join(dir, "..", "..")} {
			if _, err := os.Stat(filepath.Join(cand, "config.toml")); err == nil {
				abs, _ := filepath.Abs(cand)
				return abs
			}
		}
	}
	return wd
}

func findConfigFile(base string) (string, *fileConfig) {
	candidates := []string{
		filepath.Join(base, "config.toml"),
		filepath.Join(base, "..", "config.toml"),
	}
	// also check exe dir
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "config.toml"),
			filepath.Join(dir, "..", "config.toml"),
		)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			var fc fileConfig
			if _, err := toml.DecodeFile(p, &fc); err == nil {
				return p, &fc
			}
		}
	}
	return "", nil
}

// Load reads and validates settings from config.toml, env vars and defaults.
// Precedence: env var > config.toml > default.
func Load() *Settings {
	base := baseDir()
	configPath, fc := findConfigFile(base)

	// helpers that respect env > toml > default
	intOr := func(envName string, tomlVal, def int) int {
		if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				return n
			}
			return def
		}
		if fc != nil && tomlVal > 0 {
			return tomlVal
		}
		return def
	}
	int64Or := func(envName string, tomlVal, def int64) int64 {
		if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
			if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && n > 0 {
				return n
			}
			return def
		}
		if fc != nil && tomlVal > 0 {
			return tomlVal
		}
		return def
	}
	strOr := func(envName string, tomlVal, def string) string {
		if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
			return v
		}
		if fc != nil && strings.TrimSpace(tomlVal) != "" {
			return strings.TrimSpace(tomlVal)
		}
		return def
	}

	// DatabaseURL: env > toml > ""
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" && fc != nil {
		databaseURL = strings.TrimSpace(fc.DB.URL)
	}

	hostname := strOr("HOSTNAME", func() string {
		if fc != nil {
			return fc.Collector.Hostname
		}
		return ""
	}(), "")
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	ctVal := intOr("DATABASE_CONNECT_TIMEOUT", 0, 0)
	if ctVal == 0 {
		if fc != nil && fc.DB.ConnectTimeout > 0 {
			ctVal = fc.DB.ConnectTimeout
		} else {
			ctVal = 10
		}
	}
	connectTimeout := time.Duration(ctVal) * time.Second

	retryVal := intOr("DATABASE_RETRY_SECONDS", 0, 0)
	if retryVal == 0 {
		if fc != nil && fc.DB.RetrySeconds > 0 {
			retryVal = fc.DB.RetrySeconds
		} else {
			retryVal = 30
		}
	}
	retrySeconds := time.Duration(retryVal) * time.Second

	bufferMax := int64Or("BUFFER_MAX_BYTES", 0, 0)
	if bufferMax == 0 {
		if fc != nil && fc.DB.BufferMaxBytes > 0 {
			bufferMax = fc.DB.BufferMaxBytes
		} else {
			bufferMax = 2 * 1024 * 1024 * 1024
		}
	}

	// Power: env > toml > default 30.0W
	baseline := func() float64 {
		if v := strings.TrimSpace(os.Getenv("POWER_AUX_BASELINE_W")); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
				return f
			}
		}
		if fc != nil && fc.Power.AuxBaselineW != 0 {
			return fc.Power.AuxBaselineW
		}
		return 30.0
	}()

	efficiency := func() float64 {
		if v := strings.TrimSpace(os.Getenv("POWER_PSU_EFFICIENCY")); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f <= 1 {
				return f
			}
		}
		if fc != nil && fc.Power.PSUEfficiency != 0 {
			if fc.Power.PSUEfficiency > 0 && fc.Power.PSUEfficiency <= 1 {
				return fc.Power.PSUEfficiency
			}
		}
		return 0.90
	}()

	gpuIdle, hasIdle := nonnegativeFloat("POWER_GPU_IDLE_W", 0, false)
	gpuMax, hasMax := nonnegativeFloat("POWER_GPU_MAX_W", 0, false)
	// toml overrides if env absent
	if os.Getenv("POWER_GPU_IDLE_W") == "" && fc != nil && fc.Power.GPUIdleW != nil {
		gpuIdle = *fc.Power.GPUIdleW
		hasIdle = true
	}
	if os.Getenv("POWER_GPU_MAX_W") == "" && fc != nil && fc.Power.GPUMaxW != nil {
		gpuMax = *fc.Power.GPUMaxW
		hasMax = true
	}
	var idlePtr, maxPtr *float64
	if hasIdle && hasMax {
		idlePtr, maxPtr = &gpuIdle, &gpuMax
	}

	port := intOr("DASHBOARD_PORT", 0, 0)
	if port == 0 {
		if fc != nil && fc.Dashboard.Port > 0 {
			port = fc.Dashboard.Port
		} else {
			port = 8501
		}
	}
	if port > 65535 {
		port = 8501
	}

	// Intervals: env > toml > default
	intervals := map[string]time.Duration{
		"cpu":            time.Duration(intOr("INTERVAL_CPU", 0, 0))*time.Second,
		"memory":         time.Duration(intOr("INTERVAL_MEMORY", 0, 0))*time.Second,
		"disk_io":        time.Duration(intOr("INTERVAL_DISK_IO", 0, 0))*time.Second,
		"disk_usage":     time.Duration(intOr("INTERVAL_DISK_USAGE", 0, 0))*time.Second,
		"disk_physical":  time.Duration(intOr("INTERVAL_DISK_PHYSICAL", 0, 0))*time.Second,
		"disk_smart":     time.Duration(intOr("INTERVAL_DISK_SMART", 0, 0))*time.Second,
		"network":        time.Duration(intOr("INTERVAL_NETWORK", 0, 0))*time.Second,
		"gpu":            time.Duration(intOr("INTERVAL_GPU", 0, 0))*time.Second,
		"sensors":        time.Duration(intOr("INTERVAL_SENSORS", 0, 0))*time.Second,
		"processes":      time.Duration(intOr("INTERVAL_PROCESSES", 0, 0))*time.Second,
		"connections":    time.Duration(intOr("INTERVAL_CONNECTIONS", 0, 0))*time.Second,
		"services":       time.Duration(intOr("INTERVAL_SERVICES", 0, 0))*time.Second,
		"system":         time.Duration(intOr("INTERVAL_SYSTEM", 0, 0))*time.Second,
		"eventlog":       time.Duration(intOr("INTERVAL_EVENTLOG", 0, 0))*time.Second,
	}
	// Fill defaults where still 0, using toml intervals if present
	defaults := map[string]int{
		"cpu": 10, "memory": 10, "disk_io": 10, "disk_usage": 60, "disk_physical": 300, "disk_smart": 300,
		"network": 10, "gpu": 10, "sensors": 15, "processes": 30, "connections": 30, "services": 60, "system": 60, "eventlog": 60,
	}
	tomlIntervals := map[string]int{}
	if fc != nil {
		tomlIntervals = map[string]int{
			"cpu": fc.Intervals.CPU, "memory": fc.Intervals.Memory, "disk_io": fc.Intervals.DiskIO,
			"disk_usage": fc.Intervals.DiskUsage, "disk_physical": fc.Intervals.DiskPhysical, "disk_smart": fc.Intervals.DiskSmart,
			"network": fc.Intervals.Network, "gpu": fc.Intervals.GPU, "sensors": fc.Intervals.Sensors,
			"processes": fc.Intervals.Processes, "connections": fc.Intervals.Connections, "services": fc.Intervals.Services,
			"system": fc.Intervals.System, "eventlog": fc.Intervals.Eventlog,
		}
	}
	for k, def := range defaults {
		if intervals[k] == 0 {
			if fc != nil && tomlIntervals[k] > 0 {
				intervals[k] = time.Duration(tomlIntervals[k]) * time.Second
			} else {
				intervals[k] = time.Duration(def) * time.Second
			}
		}
	}

	enableRetention := parseBoolEnv("ENABLE_RETENTION", false)
	if os.Getenv("ENABLE_RETENTION") == "" && fc != nil {
		enableRetention = fc.Retention.Enabled
	}
	retention := map[string]string{
		"monitor.processes":   envOrToml("RETENTION_PROCESSES", func() string { if fc != nil { return fc.Retention.Processes }; return "" }(), "30 days"),
		"monitor.connections": envOrToml("RETENTION_CONNECTIONS", func() string { if fc != nil { return fc.Retention.Connections }; return "" }(), "7 days"),
		"monitor.sensors":     envOrToml("RETENTION_SENSORS", func() string { if fc != nil { return fc.Retention.Sensors }; return "" }(), "90 days"),
		"monitor.cpu":         envOrToml("RETENTION_CPU", func() string { if fc != nil { return fc.Retention.CPU }; return "" }(), "90 days"),
		"monitor.memory":      envOrToml("RETENTION_MEMORY", func() string { if fc != nil { return fc.Retention.Memory }; return "" }(), "90 days"),
		"monitor.gpu":         envOrToml("RETENTION_GPU", func() string { if fc != nil { return fc.Retention.GPU }; return "" }(), "90 days"),
		"monitor.heartbeat":   envOrToml("RETENTION_HEARTBEAT", func() string { if fc != nil { return fc.Retention.Heartbeat }; return "" }(), "30 days"),
		"monitor.eventlog":    envOrToml("RETENTION_EVENTLOG", func() string { if fc != nil { return fc.Retention.Eventlog }; return "" }(), "30 days"),
		"monitor.disk_io":     envOrToml("RETENTION_DISK_IO", func() string { if fc != nil { return fc.Retention.DiskIO }; return "" }(), "90 days"),
		"monitor.net_io":      envOrToml("RETENTION_NET_IO", func() string { if fc != nil { return fc.Retention.NetIO }; return "" }(), "90 days"),
	}
	batchLimit := intOr("RETENTION_BATCH_LIMIT", 0, 0)
	if batchLimit == 0 {
		if fc != nil && fc.Retention.BatchLimit > 0 {
			batchLimit = fc.Retention.BatchLimit
		} else {
			batchLimit = 50000
		}
	}
	batchSleep := 100 * time.Millisecond
	if v := strings.TrimSpace(os.Getenv("RETENTION_BATCH_SLEEP")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			batchSleep = time.Duration(f * float64(time.Second))
		}
	} else if fc != nil && fc.Retention.BatchSleep != 0 {
		batchSleep = time.Duration(fc.Retention.BatchSleep * float64(time.Second))
	}

	logDir := filepath.Join(base, "logs")
	_ = os.MkdirAll(logDir, 0755)

	host := strOr("DASHBOARD_HOST", func() string {
		if fc != nil {
			return fc.Dashboard.Host
		}
		return ""
	}(), "127.0.0.1")
	tz := strOr("DASHBOARD_TIMEZONE", func() string {
		if fc != nil {
			return fc.Dashboard.Timezone
		}
		return ""
	}(), "America/Sao_Paulo")
	if tz == "" {
		tz = "America/Sao_Paulo"
	}

	topProcs := intOr("TOP_PROCESSES", 0, 0)
	if topProcs == 0 {
		if fc != nil && fc.Collector.TopProcesses > 0 {
			topProcs = fc.Collector.TopProcesses
		} else {
			topProcs = 50
		}
	}

	return &Settings{
		DatabaseURL:        databaseURL,
		Hostname:           hostname,
		ConnectTimeout:     connectTimeout,
		RetrySeconds:       retrySeconds,
		BufferMaxBytes:     bufferMax,
		DashboardHost:      host,
		DashboardPort:      port,
		DashboardTimezone:  tz,
		PowerAuxBaselineW:  baseline,
		PowerPSUEfficiency: efficiency,
		PowerGPUIdleW:      idlePtr,
		PowerGPUMaxW:       maxPtr,
		Intervals:          intervals,
		TopProcesses:       topProcs,
		BufferPath:         filepath.Join(logDir, "pending_batches.sqlite3"),
		LogDir:             logDir,
		EnableRetention:    enableRetention,
		Retention:          retention,
		RetentionBatch:     batchLimit,
		RetentionSleep:     batchSleep,
		ConfigPath:         configPath,
	}
}

func seconds(name string, def int) time.Duration {
	return time.Duration(positiveInt(name, def)) * time.Second
}

func parseBoolEnv(name string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func envOr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

func envOrToml(envName, tomlVal, def string) string {
	if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
		return v
	}
	if strings.TrimSpace(tomlVal) != "" {
		return strings.TrimSpace(tomlVal)
	}
	return def
}
