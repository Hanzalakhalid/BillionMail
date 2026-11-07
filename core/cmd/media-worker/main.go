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

	"billionmail-core/internal/service/antivirus"
	"billionmail-core/internal/service/assets"
	"billionmail-core/internal/service/database_initialization"
	"billionmail-core/internal/service/objectstorage"
	platformqueue "billionmail-core/internal/service/queue"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := database_initialization.InitWorkerDatabase(ctx); err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	store, err := objectstorage.New(ctx, objectstorage.ConfigFromEnv())
	if err != nil {
		logger.Error("storage configuration failed", "error", err)
		os.Exit(1)
	}
	scanner := antivirus.Clamd{Address: os.Getenv("CLAMD_ADDR")}
	validator := assets.Validator{Store: store, Scanner: scanner}
	redisOptions := platformqueue.RedisClientOpt()
	client := asynq.NewClient(redisOptions)
	defer client.Close()
	var healthyAt atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if healthyAt.Load() == 0 || time.Now().Unix()-healthyAt.Load() > 30 {
			http.Error(w, "media dependencies are unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	address := os.Getenv("MEDIA_HEALTH_ADDR")
	if address == "" {
		address = ":8082"
	}
	health := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := health.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health listener failed", "error", err)
			stop()
		}
	}()
	hostname, _ := os.Hostname()
	dispatcher := platformqueue.Dispatcher{Publisher: platformqueue.AsynqPublisher{Client: client}, WorkerID: "media-" + hostname + "-" + uuid.NewString()[:8]}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := dispatcher.Dispatch(ctx); err != nil && ctx.Err() == nil {
					logger.Error("outbox dispatch failed", "error", err)
				}
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			check, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, dbErr := g.DB().Exec(check, "SELECT 1")
			storageErr := store.Health(check)
			scannerErr := scanner.Health(check)
			redisErr := client.Ping()
			cancel()
			if dbErr == nil && storageErr == nil && scannerErr == nil && redisErr == nil {
				healthyAt.Store(time.Now().Unix())
			} else {
				healthyAt.Store(0)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	processor := platformqueue.Processor{Consumer: "media-worker", Handlers: map[string]platformqueue.Handler{"asset.upload.validate": validator.Handle, "asset.quarantine.cleanup": validator.Cleanup}}
	server := asynq.NewServer(redisOptions, asynq.Config{Concurrency: 2, Queues: map[string]int{"media": 1}, ShutdownTimeout: 30 * time.Second,
		ErrorHandler: asynq.ErrorHandlerFunc(func(taskCtx context.Context, task *asynq.Task, failure error) {
			errorCtx, cancel := context.WithTimeout(context.WithoutCancel(taskCtx), 5*time.Second)
			defer cancel()
			if err := platformqueue.RecordTaskFailure(errorCtx, task, failure); err != nil {
				logger.Error("record media failure", "error", err)
			}
		}),
	})
	tasks := asynq.NewServeMux()
	tasks.HandleFunc(platformqueue.OutboxTaskType, processor.HandleTask)
	go func() {
		<-ctx.Done()
		server.Shutdown()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = health.Shutdown(shutdown)
	}()
	if err := server.Run(tasks); err != nil {
		logger.Error("media worker stopped", "error", err)
		os.Exit(1)
	}
}
