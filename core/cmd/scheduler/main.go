package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"billionmail-core/internal/service/database_initialization"
	"billionmail-core/internal/service/scheduler"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/google/uuid"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := database_initialization.InitWorkerDatabase(ctx); err != nil {
		logger.Error("initialize database", "error", err)
		os.Exit(1)
	}
	hostname, _ := os.Hostname()
	runner := scheduler.Scheduler{WorkerID: hostname + "-" + uuid.NewString()[:8], AllowedEvents: map[string]bool{"platform.schedule.probe": true, "asset.quarantine.cleanup": true}}
	var lastSuccess atomic.Int64
	lastSuccess.Store(time.Now().Unix())
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if time.Now().Unix()-lastSuccess.Load() > 30 {
			http.Error(w, "scheduler tick overdue", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	address := os.Getenv("SCHEDULER_HEALTH_ADDR")
	if address == "" {
		address = ":8081"
	}
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health listener", "error", err)
			stop()
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = server.Shutdown(shutdown)
			cancel()
			return
		case <-ticker.C:
			tickCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			count, err := runner.Tick(tickCtx)
			cancel()
			if err != nil {
				logger.Error("scheduler tick", "error", err)
			} else {
				lastSuccess.Store(time.Now().Unix())
				if count > 0 {
					logger.Info("scheduled occurrences queued", "count", count)
				}
			}
		}
	}
}
