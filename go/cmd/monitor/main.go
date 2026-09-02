package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dankkom/windows-system-monitor/go/internal/collectors"
	"github.com/dankkom/windows-system-monitor/go/internal/config"
	"github.com/dankkom/windows-system-monitor/go/internal/dashboard"
	"github.com/dankkom/windows-system-monitor/go/internal/db"
	"github.com/dankkom/windows-system-monitor/go/internal/spool"
)

func main() {
	var once, dryRun, serveFlag, collectFlag, initFlag, retentionFlag, retentionDry bool
	var intervalOverride int
	flag.BoolVar(&once, "once", false, "roda uma coleta de cada e sai")
	flag.BoolVar(&dryRun, "dry-run", false, "não insere no banco, só testa coletores")
	flag.BoolVar(&serveFlag, "serve", false, "inicia apenas o dashboard")
	flag.BoolVar(&collectFlag, "collect", false, "inicia apenas o coletor")
	flag.BoolVar(&initFlag, "init", false, "cria database e aplica schema, depois sai")
	flag.BoolVar(&retentionFlag, "retention", false, "executa retenção (DELETE) conforme .env")
	flag.BoolVar(&retentionDry, "retention-dry-run", false, "simula retenção sem deletar")
	flag.IntVar(&intervalOverride, "interval", 0, "override intervalo base (s) para teste")
	flag.Parse()

	cfg := config.Load()
	if intervalOverride > 0 {
		for k := range cfg.Intervals {
			cfg.Intervals[k] = time.Duration(intervalOverride) * time.Second
		}
	}

	// file log setup
	logDir := cfg.LogDir
	_ = os.MkdirAll(logDir, 0755)
	logFile, _ := os.OpenFile(logDir+"/monitor-go.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if logFile != nil {
		defer logFile.Close()
		log.SetOutput(logFile)
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("starting hostname=%s dry_run=%v once=%v interval_override=%v", cfg.Hostname, dryRun, once, intervalOverride)

	if initFlag {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := db.InitDatabase(ctx, cfg); err != nil {
			log.Fatalf("init: %v", err)
		}
		log.Printf("init complete")
		fmt.Println("init complete")
		return
	}
	if retentionFlag || retentionDry {
		dry := retentionDry || dryRun
		if err := runRetention(cfg, dry); err != nil {
			log.Fatalf("retention: %v", err)
		}
		return
	}

	if serveFlag && !collectFlag {
		if err := runServe(cfg); err != nil {
			log.Fatalf("serve: %v", err)
		}
		return
	}

	// collector path needs DB unless dry-run
	if dryRun {
		runDry(cfg)
		return
	}
	if once {
		if err := runOnce(cfg); err != nil {
			log.Fatalf("once: %v", err)
		}
		return
	}
	if serveFlag && collectFlag {
		// run both concurrently
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := runServe(cfg); err != nil {
				log.Printf("serve error: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := runLoop(cfg); err != nil {
				log.Printf("collect error: %v", err)
			}
		}()
		wg.Wait()
		return
	}
	// default: collector loop (serve is separate scheduled task, but we can also run both)
	if err := runLoop(cfg); err != nil {
		log.Fatalf("loop: %v", err)
	}
}

func runDry(cfg *config.Settings) {
	hostname := cfg.Hostname
	now := time.Now().UTC()
	cases := collectorFuncs(cfg)
	for name, fn := range cases {
		res, err := fn(hostname, now)
		if err != nil {
			log.Printf("[DRY %s] error: %v", name, err)
			continue
		}
		sample := ""
		if len(res.Rows) > 0 {
			sample = fmt.Sprintf("%v", res.Rows[0][:min(3, len(res.Columns))])
		}
		log.Printf("[DRY %s] cols=%v rows=%d sample=%s table=%s", name, res.Columns[:min(3, len(res.Columns))], len(res.Rows), sample, res.Table)
	}
}

func runOnce(cfg *config.Settings) error {
	store, err := db.New(cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		log.Printf("schema check: %v", err)
	}
	hostname := cfg.Hostname
	now := time.Now().UTC()
	for name, fn := range collectorFuncs(cfg) {
		start := time.Now()
		res, err := fn(hostname, now)
		dur := time.Since(start).Milliseconds()
		if err != nil {
			log.Printf("[%s] collect failed: %v", name, err)
			store.InsertHeartbeat(ctx, hostname, name, int(dur), 0, false, err.Error())
			continue
		}
		var n int
		var status string
		if len(res.Rows) == 0 {
			n, status = 0, "empty"
		} else {
			wr := store.InsertBatch(ctx, res.Table, res.Columns, res.Rows)
			n, status = wr.Rows, wr.Status
		}
		log.Printf("[%s] %d rows -> %s (%dms, %s)", name, n, res.Table, dur, status)
		store.InsertHeartbeat(ctx, hostname, name, int(dur), n, status == "stored", "")
	}
	log.Printf("once run complete")
	return nil
}

func runLoop(cfg *config.Settings) error {
	store, err := db.New(cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		log.Printf("schema check failed: %v", err)
		return err
	}
	log.Printf("schema OK")

	lastRun := map[string]time.Time{}
	for name := range collectorFuncs(cfg) {
		lastRun[name] = time.Time{}
	}
	slowBusy := map[string]bool{"services": false, "processes": false}
	var mu sync.Mutex
	sem := make(chan struct{}, 2)

	// signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		<-sigCh
		log.Printf("signal received, shutting down")
		close(done)
	}()

	log.Printf("entering continuous loop")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			log.Printf("monitor stopped gracefully")
			return nil
		case <-ticker.C:
			// replay spool
			if n, err := store.ReplayPending(ctx); err == nil && n > 0 {
				log.Printf("replayed %d buffered batches", n)
			}
			now := time.Now()
			for name, fn := range collectorFuncs(cfg) {
				interval := cfg.Intervals[name]
				if interval == 0 {
					// net_addr uses system interval, etc.
					if name == "net_addr" {
						interval = cfg.Intervals["system"]
					} else {
						continue
					}
				}
				mu.Lock()
				lr := lastRun[name]
				mu.Unlock()
				if now.Sub(lr) < interval {
					continue
				}
				isSlow := name == "services" || name == "processes"
				if isSlow {
					mu.Lock()
					busy := slowBusy[name]
					mu.Unlock()
					if busy {
						continue
					}
					mu.Lock()
					slowBusy[name] = true
					lastRun[name] = now
					mu.Unlock()
					sem <- struct{}{}
					go func(nm string, f func(string, time.Time) (collectors.Result, error)) {
						defer func() {
							mu.Lock()
							slowBusy[nm] = false
							mu.Unlock()
							<-sem
						}()
						runCollector(store, ctx, cfg, nm, f)
					}(name, fn)
				} else {
					mu.Lock()
					lastRun[name] = now
					mu.Unlock()
					runCollector(store, ctx, cfg, name, fn)
				}
			}
		}
	}
}

func runCollector(store *db.Store, ctx context.Context, cfg *config.Settings, name string, fn func(string, time.Time) (collectors.Result, error)) {
	start := time.Now()
	res, err := fn(cfg.Hostname, time.Now().UTC())
	dur := time.Since(start).Milliseconds()
	if err != nil {
		log.Printf("[%s] failed: %v", name, err)
		store.InsertHeartbeat(ctx, cfg.Hostname, name, int(dur), 0, false, err.Error())
		return
	}
	var n int
	var status string
	if len(res.Rows) == 0 {
		n, status = 0, "empty"
	} else {
		wr := store.InsertBatch(ctx, res.Table, res.Columns, res.Rows)
		n, status = wr.Rows, wr.Status
	}
	log.Printf("[%s] %d rows -> %s (%dms, %s)", name, n, res.Table, dur, status)
	store.InsertHeartbeat(ctx, cfg.Hostname, name, int(dur), n, status == "stored", "")
}

func collectorFuncs(cfg *config.Settings) map[string]func(string, time.Time) (collectors.Result, error) {
	return map[string]func(string, time.Time) (collectors.Result, error){
		"cpu":           collectors.CollectCPU,
		"memory":        collectors.CollectMemory,
		"disk_io":       collectors.CollectDiskIO,
		"disk_usage":    collectors.CollectDiskUsage,
		"disk_physical": collectors.CollectPhysicalDisk,
		"disk_smart":    collectors.CollectDiskSmart,
		"net_io":        collectors.CollectNetIO,
		"net_addr":      collectors.CollectNetAddrs,
		"gpu":           collectors.CollectGPU,
		"sensors":       collectors.CollectSensors,
		"processes": func(h string, ts time.Time) (collectors.Result, error) {
			return collectors.CollectProcesses(h, ts, cfg.TopProcesses)
		},
		"connections": collectors.CollectConnections,
		"services":    collectors.CollectServices,
		"system":      collectors.CollectSystem,
		"eventlog":    collectors.CollectEventLog,
	}
}

func runServe(cfg *config.Settings) error {
	// need DB pool
	ctx := context.Background()
	pool, err := newPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("pool: %w", err)
	}
	defer pool.Close()
	spl, err := spool.New(cfg.BufferPath, cfg.BufferMaxBytes)
	if err != nil {
		return err
	}
	defer spl.Close()

	h := dashboard.New(cfg, pool, spl)
	addr := fmt.Sprintf("%s:%d", cfg.DashboardHost, cfg.DashboardPort)
	if addr == ":0" || stringsHasEmptyHost(cfg.DashboardHost) {
		addr = fmt.Sprintf("127.0.0.1:%d", cfg.DashboardPort)
	}
	srv := &http.Server{Addr: addr, Handler: h, ReadTimeout: 10 * time.Second, WriteTimeout: 20 * time.Second}
	log.Printf("dashboard listening on http://%s", addr)
	// signal for serve
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("dashboard shutting down")
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func newPool(ctx context.Context, cfg *config.Settings) (*pgxpool.Pool, error) {
	url, err := cfg.RequireDatabaseURL()
	if err != nil {
		return nil, err
	}
	pcfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	pcfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	return pgxpool.NewWithConfig(ctx, pcfg)
}

func runRetention(cfg *config.Settings, dry bool) error {
	if !cfg.EnableRetention && !dry {
		msg := "retention disabled (ENABLE_RETENTION=false) - historico permanente"
		log.Println(msg)
		fmt.Println(msg)
		return nil
	}
	store, err := db.New(cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if !cfg.EnableRetention && dry {
		log.Printf("retention dry-run even though disabled - counting anyway")
	}
	results, err := store.RunRetention(ctx, cfg, dry)
	if err != nil && !dry && !cfg.EnableRetention {
		// already handled above, but db would return error
		return nil
	}
	if err != nil {
		return err
	}
	var total int64
	for _, n := range results {
		total += n
	}
	if dry {
		log.Printf("retention dry-run complete, total would delete: %d", total)
		fmt.Printf("retention dry-run complete, total would delete: %d\n", total)
	} else {
		log.Printf("retention complete, total deleted: %d", total)
		fmt.Printf("retention complete, total deleted: %d\n", total)
	}
	return nil
}

func stringsHasEmptyHost(h string) bool { return h == "" || h == "0.0.0.0" }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
