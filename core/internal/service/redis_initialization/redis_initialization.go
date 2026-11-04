package redis_initialization

import (
	"billionmail-core/internal/service/public"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
	"strconv"
	"strings"
	"time"
)

// InitRedis initialize redis configuration
func InitRedis() (err error) {
	// get redis password from environment variable
	passwd, err := public.DockerEnv("REDISPASS")

	if err != nil {
		return fmt.Errorf("redis env init error: %v", err)
	}

	db := 1
	dbEnv, err := public.DockerEnv("REDISDB")
	if err == nil {
		if dbVal, parseErr := strconv.Atoi(dbEnv); parseErr == nil {
			db = dbVal
		}
	}

	address := public.MustGetDockerEnv("REDIS_ADDR", "127.0.0.1:26379")

	if public.IsRunningInContainer() && address == "127.0.0.1:26379" {
		address = "redis:6379"
	}

	// Initialize Redis configuration
	redisConfig := &gredis.Config{
		Address: address,
		Db:      db,
		Pass:    passwd,
	}
	if strings.EqualFold(public.MustGetDockerEnv("REDIS_TLS", "false"), "true") {
		redisConfig.TLS = true
		redisConfig.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	gredis.SetConfig(redisConfig)

	// Testing redis connection until it successful
	connectionOK := false
	for i := 0; i < 10; i++ {
		k := "bm_test_connection"
		if err := g.Redis().SetEX(context.Background(), k, k, 1); err != nil {
			g.Log().Error(context.Background(), "Redis connection test failed: ", err, " Waiting for 5 seconds before retrying...")
			time.Sleep(5 * time.Second)
			continue
		}

		if _, err := g.Redis().Del(context.Background(), k); err != nil {
			g.Log().Error(context.Background(), "Redis connection test failed: ", err, " Waiting for 5 seconds before retrying...")
			time.Sleep(5 * time.Second)
			continue
		}

		connectionOK = true
		g.Log().Debug(context.Background(), "Redis connection test successful")
		break
	}

	if !connectionOK {
		return fmt.Errorf("redis connection test failed after 10 attempts")
	}

	return nil
}
