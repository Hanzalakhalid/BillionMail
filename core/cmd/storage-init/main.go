package main

import (
	"billionmail-core/internal/service/objectstorage"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	timeout, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	store, err := objectstorage.New(timeout, objectstorage.ConfigFromEnv())
	if err != nil {
		logger.Error("invalid storage configuration", "error", err)
		os.Exit(1)
	}
	for {
		if err = store.EnsureBucket(timeout); err == nil {
			logger.Info("media bucket is ready")
			return
		}
		select {
		case <-timeout.Done():
			logger.Error("media bucket initialization failed", "error", err)
			os.Exit(1)
		case <-time.After(2 * time.Second):
		}
	}
}
