package database_initialization

import (
	"billionmail-core/internal/consts"
	"billionmail-core/internal/migrations"
	"billionmail-core/internal/service/public"
	"context"
	"fmt"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"strconv"
	"time"
)

var registeredHandlers = make([]func(), 0, 256)

// InitDatabase initializes the database configuration
func InitDatabase() (err error) {
	dbPass, err := public.DockerEnv("DBPASS")

	if err != nil {
		return fmt.Errorf("Read database password failed: %v", err)
	}

	dbHost := public.MustGetDockerEnv("DB_HOST", public.AbsPath(consts.POSTGRESQL_SOCK))
	dbPort := public.MustGetDockerEnv("DB_PORT", "")
	dbUser := public.MustGetDockerEnv("DBUSER", "billionmail")
	dbName := public.MustGetDockerEnv("DBNAME", "billionmail")
	dbSSLMode := public.MustGetDockerEnv("DB_SSLMODE", "disable")
	maxConnections, parseErr := strconv.Atoi(public.MustGetDockerEnv("DB_MAX_OPEN_CONNECTIONS", "100"))
	if parseErr != nil || maxConnections < 1 {
		return fmt.Errorf("DB_MAX_OPEN_CONNECTIONS must be a positive integer")
	}

	// Runtime connection settings support both the existing Unix socket and
	// managed PostgreSQL services such as RDS.
	err = gdb.SetConfig(gdb.Config{
		"default": gdb.ConfigGroup{
			gdb.ConfigNode{
				// Debug: true,
				Host:             dbHost,
				Port:             dbPort,
				User:             dbUser,
				Pass:             dbPass,
				Name:             dbName,
				Type:             "pgsql",
				Extra:            "sslmode=" + dbSSLMode,
				Role:             "master",
				MaxOpenConnCount: maxConnections,
			},
		},
	})

	if err != nil {
		return fmt.Errorf("Set database configuration failed: %v", err)
	}

	// Testing database connection until successful
	connectionOK := false
	for i := 0; i < 10; i++ {
		_, err = g.DB().Exec(context.Background(), "SELECT 1")

		if err != nil {
			// Wait for 5 seconds before retrying
			g.Log().Debug(context.Background(), "Database connection failed, retrying in 5 seconds...")
			time.Sleep(time.Second * 5)
			continue
		}

		connectionOK = true
		g.Log().Debug(context.Background(), "Database connection successful")
		break
	}

	if !connectionOK {
		return fmt.Errorf("database connection test failed after 10 attempts")
	}

	// Execute registered handlers
	for _, handler := range registeredHandlers {
		if handler != nil {
			handler()
		}
	}

	// Empty the registered handlers
	registeredHandlers = registeredHandlers[:0]

	// Apply versioned SaaS migrations only after legacy bootstrap tables exist.
	if err = migrations.Up(context.Background()); err != nil {
		return err
	}

	return nil
}

// registerHandler registers a handler for the database initialization
func registerHandler(handler func()) {
	if handler == nil {
		return
	}

	registeredHandlers = append(registeredHandlers, handler)
}
