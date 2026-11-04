package queue

import (
	"crypto/tls"
	"os"
	"strconv"
	"strings"

	"github.com/hibiken/asynq"
)

func RedisClientOpt() asynq.RedisClientOpt {
	address := envOr("REDIS_ADDR", "redis:6379")
	database, _ := strconv.Atoi(envOr("REDISDB", "1"))
	options := asynq.RedisClientOpt{
		Addr: address, Password: os.Getenv("REDISPASS"), DB: database,
		Username: os.Getenv("REDIS_USERNAME"),
	}
	if strings.EqualFold(os.Getenv("REDIS_TLS"), "true") {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return options
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
