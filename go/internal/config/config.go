// Package config loads runtime settings from environment and .env, mirroring
// monitor_pkg.config.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
}

// loadDotEnv applies KEY=VALUE pairs from path into the environment, skipping
// lines already set. A minimal .env parser for KEY=VALUE lines and # comments.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

// RequireDatabaseURL resolves and validates DATABASE_URL, mirroring the Python
// require_database_url. Returns an error when a DB connection is needed but the
// URL is absent/malformed.
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

// baseDir resolves the project root by searching for .env upwards.
func baseDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	// search wd, parent, grandparent for .env
	for _, cand := range []string{wd, filepath.Join(wd, ".."), filepath.Join(wd, "..", "..")} {
		if _, err := os.Stat(filepath.Join(cand, ".env")); err == nil {
			abs, _ := filepath.Abs(cand)
			return abs
		}
	}
	// also check executable dir
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, cand := range []string{dir, filepath.Join(dir, ".."), filepath.Join(dir, "..", "..")} {
			if _, err := os.Stat(filepath.Join(cand, ".env")); err == nil {
				abs, _ := filepath.Abs(cand)
				return abs
			}
		}
	}
	return wd
}

// Load reads and validates settings from the environment and .env file.
func Load() *Settings {
	base := baseDir()
	loadDotEnv(filepath.Join(base, ".env"))
	// also try parent if not found
	if os.Getenv("DATABASE_URL") == "" {
		loadDotEnv(filepath.Join(base, "..", ".env"))
	}

	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	connectTimeout := time.Duration(positiveInt("DATABASE_CONNECT_TIMEOUT", 10)) * time.Second
	retrySeconds := time.Duration(positiveInt("DATABASE_RETRY_SECONDS", 30)) * time.Second
	bufferMax := positiveInt64("BUFFER_MAX_BYTES", 2*1024*1024*1024)

	baseline, _ := nonnegativeFloat("POWER_AUX_BASELINE_W", 30.0, true)
	efficiency, _ := nonnegativeFloat("POWER_PSU_EFFICIENCY", 0.90, true)
	if efficiency <= 0 || efficiency > 1 {
		efficiency = 0.90
	}

	gpuIdle, hasIdle := nonnegativeFloat("POWER_GPU_IDLE_W", 0, false)
	gpuMax, hasMax := nonnegativeFloat("POWER_GPU_MAX_W", 0, false)
	var idlePtr, maxPtr *float64
	if hasIdle && hasMax {
		idlePtr, maxPtr = &gpuIdle, &gpuMax
	}

	port := positiveInt("DASHBOARD_PORT", 8501)
	if port > 65535 {
		port = 8501
	}

	intervals := map[string]time.Duration{
		"cpu":            seconds("INTERVAL_CPU", 10),
		"memory":         seconds("INTERVAL_MEMORY", 10),
		"disk_io":        seconds("INTERVAL_DISK_IO", 10),
		"disk_usage":     seconds("INTERVAL_DISK_USAGE", 60),
		"disk_physical":  seconds("INTERVAL_DISK_PHYSICAL", 300),
		"disk_smart":     seconds("INTERVAL_DISK_SMART", 300),
		"network":        seconds("INTERVAL_NETWORK", 10),
		"gpu":            seconds("INTERVAL_GPU", 10),
		"sensors":        seconds("INTERVAL_SENSORS", 15),
		"processes":      seconds("INTERVAL_PROCESSES", 30),
		"connections":    seconds("INTERVAL_CONNECTIONS", 30),
		"services":       seconds("INTERVAL_SERVICES", 60),
		"system":         seconds("INTERVAL_SYSTEM", 60),
		"eventlog":       seconds("INTERVAL_EVENTLOG", 60),
	}

	logDir := filepath.Join(base, "logs")
	_ = os.MkdirAll(logDir, 0755)

	host := os.Getenv("DASHBOARD_HOST")
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	tz := strings.TrimSpace(os.Getenv("DASHBOARD_TIMEZONE"))
	if tz == "" {
		tz = "America/Sao_Paulo"
	}
	return &Settings{
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
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
		TopProcesses:       positiveInt("TOP_PROCESSES", 50),
		BufferPath:         filepath.Join(logDir, "pending_batches.sqlite3"),
		LogDir:             logDir,
	}
}

func seconds(name string, def int) time.Duration {
	return time.Duration(positiveInt(name, def)) * time.Second
}
