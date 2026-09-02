package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dankkom/windows-system-monitor/go/internal/config"
	"github.com/dankkom/windows-system-monitor/go/internal/spool"
)

//go:embed static/*
var staticFS embed.FS

//go:embed templates/*
var templatesFS embed.FS

var windows = map[string]bool{
	"15 minutes": true, "1 hour": true, "6 hours": true, "24 hours": true, "7 days": true, "30 days": true,
}
var windowBuckets = map[string]int{
	"15 minutes": 10, "1 hour": 10, "6 hours": 60, "24 hours": 300, "7 days": 1800, "30 days": 7200,
}

// Handler serves the dashboard UI and /api/* endpoints.
type Handler struct {
	cfg   *config.Settings
	pool  *pgxpool.Pool
	spool *spool.Spool
	mux   *http.ServeMux
}

func New(cfg *config.Settings, pool *pgxpool.Pool, spl *spool.Spool) *Handler {
	h := &Handler{cfg: cfg, pool: pool, spool: spl, mux: http.NewServeMux()}
	h.routes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.mux.ServeHTTP(w, r) }

func (h *Handler) routes() {
	// static
	staticSub, _ := fs.Sub(staticFS, "static")
	h.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	// index
	h.mux.HandleFunc("/", h.handleIndex)
	// api
	h.mux.HandleFunc("/api/cpu", h.wrap(h.handleCPU))
	h.mux.HandleFunc("/api/memory", h.wrap(h.handleMemory))
	h.mux.HandleFunc("/api/gpu", h.wrap(h.handleGPU))
	h.mux.HandleFunc("/api/sensors/cpu_temps", h.wrap(h.handleCPUTemps))
	h.mux.HandleFunc("/api/sensors/cpu_temps_latest", h.wrap(h.handleCPUTempsLatest))
	h.mux.HandleFunc("/api/sensors/latest", h.wrap(h.handleSensorsLatest))
	h.mux.HandleFunc("/api/sensors/history", h.wrap(h.handleSensorsHistory))
	h.mux.HandleFunc("/api/disk/usage", h.wrap(h.handleDiskUsage))
	h.mux.HandleFunc("/api/disk/usage/history", h.wrap(h.handleDiskUsageHistory))
	h.mux.HandleFunc("/api/disk/physical", h.wrap(h.handlePhysical))
	h.mux.HandleFunc("/api/disk/smart", h.wrap(h.handleSmart))
	h.mux.HandleFunc("/api/disk/smart/history", h.wrap(h.handleSmartHistory))
	h.mux.HandleFunc("/api/disk/io", h.wrap(h.handleDiskIO))
	h.mux.HandleFunc("/api/net", h.wrap(h.handleNet))
	h.mux.HandleFunc("/api/net/latest", h.wrap(h.handleNetLatest))
	h.mux.HandleFunc("/api/power", h.wrap(h.handlePower))
	h.mux.HandleFunc("/api/processes", h.wrap(h.handleProcesses))
	h.mux.HandleFunc("/api/system", h.wrap(h.handleSystem))
	h.mux.HandleFunc("/api/heartbeat", h.wrap(h.handleHeartbeat))
	h.mux.HandleFunc("/api/db_size", h.wrap(h.handleDBSize))
	h.mux.HandleFunc("/api/health", h.handleHealth)
	h.mux.HandleFunc("/api/ready", h.wrap(h.handleReady))
	h.mux.HandleFunc("/api/status", h.handleStatus)
}

type apiFunc func(w http.ResponseWriter, r *http.Request) (any, error)

func (h *Handler) wrap(fn apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// no-store for api
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		w.Header().Set("Content-Type", "application/json")
		data, err := fn(w, r)
		if err != nil {
			if strings.Contains(err.Error(), "window must be") || strings.Contains(err.Error(), "sensor type") {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
				return
			}
			// pgx errors -> 503
			log.Printf("dashboard query error: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "database unavailable"})
			return
		}
		if data == nil {
			data = map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(data)
	}
}

func (h *Handler) window(r *http.Request) (string, int, error) {
	v := r.URL.Query().Get("window")
	if v == "" {
		v = "1 hour"
	}
	if !windows[v] {
		return "", 0, &badRequest{msg: "window must be one of: 15 minutes, 1 hour, 6 hours, 24 hours, 7 days, 30 days"}
	}
	return v, windowBuckets[v], nil
}

type badRequest struct{ msg string }

func (e *badRequest) Error() string { return e.msg }

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(templatesFS, "templates/index.html")
	if err != nil {
		// fallback to staticFS if embedded not found (dev)
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	pending, bytes, _ := h.spool.Status()
	_ = json.NewEncoder(w).Encode(map[string]any{"pending_batches": pending, "pending_bytes": bytes, "max_bytes": h.cfg.BufferMaxBytes})
}

func ctxTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 10*time.Second)
}

// individual handlers

func (h *Handler) handleCPU(_ http.ResponseWriter, r *http.Request) (any, error) {
	window, bucket, err := h.window(r)
	if err != nil {
		return nil, err
	}
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QCPU(ctx, h.pool, window, bucket)
}
func (h *Handler) handleMemory(_ http.ResponseWriter, r *http.Request) (any, error) {
	window, bucket, err := h.window(r)
	if err != nil {
		return nil, err
	}
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QMemory(ctx, h.pool, window, bucket)
}
func (h *Handler) handleGPU(_ http.ResponseWriter, r *http.Request) (any, error) {
	window, bucket, err := h.window(r)
	if err != nil {
		return nil, err
	}
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QGPU(ctx, h.pool, window, bucket)
}
func (h *Handler) handleCPUTemps(_ http.ResponseWriter, r *http.Request) (any, error) {
	window, bucket, err := h.window(r)
	if err != nil {
		return nil, err
	}
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QCPUTemps(ctx, h.pool, window, bucket)
}
func (h *Handler) handleCPUTempsLatest(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QCPUTempsLatest(ctx, h.pool)
}
func (h *Handler) handleSensorsLatest(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QSensorsLatest(ctx, h.pool)
}
func (h *Handler) handleSensorsHistory(_ http.ResponseWriter, r *http.Request) (any, error) {
	window, bucket, err := h.window(r)
	if err != nil {
		return nil, err
	}
	sensorType := strings.TrimSpace(r.URL.Query().Get("type"))
	if sensorType == "" {
		sensorType = "power"
	}
	if len(sensorType) > 40 {
		return nil, &badRequest{msg: "sensor type must contain between 1 and 40 characters"}
	}
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QSensorsHistory(ctx, h.pool, window, bucket, sensorType)
}
func (h *Handler) handleDiskUsage(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QDiskUsage(ctx, h.pool)
}
func (h *Handler) handleDiskUsageHistory(_ http.ResponseWriter, r *http.Request) (any, error) {
	window, bucket, err := h.window(r)
	if err != nil {
		return nil, err
	}
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QDiskUsageHistory(ctx, h.pool, window, bucket)
}
func (h *Handler) handlePhysical(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QPhysicalDisk(ctx, h.pool)
}
func (h *Handler) handleSmart(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QDiskSmartLatest(ctx, h.pool)
}
func (h *Handler) handleSmartHistory(_ http.ResponseWriter, r *http.Request) (any, error) {
	window, bucket, err := h.window(r)
	if err != nil {
		return nil, err
	}
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QDiskSmartHistory(ctx, h.pool, window, bucket)
}
func (h *Handler) handleDiskIO(_ http.ResponseWriter, r *http.Request) (any, error) {
	window, bucket, err := h.window(r)
	if err != nil {
		return nil, err
	}
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QDiskIO(ctx, h.pool, window, bucket, h.cfg.Intervals["disk_io"])
}
func (h *Handler) handleNet(_ http.ResponseWriter, r *http.Request) (any, error) {
	window, bucket, err := h.window(r)
	if err != nil {
		return nil, err
	}
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QNet(ctx, h.pool, window, bucket, h.cfg.Intervals["network"])
}
func (h *Handler) handleNetLatest(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QNetLatest(ctx, h.pool)
}
func (h *Handler) handlePower(_ http.ResponseWriter, r *http.Request) (any, error) {
	window, bucket, err := h.window(r)
	if err != nil {
		return nil, err
	}
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QPower(ctx, h.pool, h.cfg, window, bucket)
}
func (h *Handler) handleProcesses(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QProcesses(ctx, h.pool)
}
func (h *Handler) handleSystem(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QSystem(ctx, h.pool)
}
func (h *Handler) handleHeartbeat(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QHeartbeat(ctx, h.pool)
}
func (h *Handler) handleDBSize(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	return QDBSize(ctx, h.pool)
}
func (h *Handler) handleReady(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	var exists bool
	err := h.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name='monitor')").Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, &badRequest{msg: "schema monitor not found"}
	}
	return map[string]any{"status": "ready"}, nil
}
