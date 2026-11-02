package database_initialization

import (
	"context"
	"github.com/gogf/gf/v2/frame/g"
)

func init() {
	registerHandler(func() {
		sqlList := []string{
			`-- Push notification send logs
			CREATE TABLE IF NOT EXISTS bm_push_logs (
				id BIGSERIAL PRIMARY KEY,
				title VARCHAR(255) NOT NULL DEFAULT '',
				message TEXT NOT NULL,
				url TEXT NOT NULL DEFAULT '',
				audience VARCHAR(64) NOT NULL DEFAULT '', -- all, group:<id>
				targeted INTEGER NOT NULL DEFAULT 0, -- contacts targeted (group sends only)
				notification_id VARCHAR(255) NOT NULL DEFAULT '',
				status VARCHAR(16) NOT NULL DEFAULT 'pending', -- sent, failed
				error TEXT NOT NULL DEFAULT '',
				create_time INTEGER NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())
			)`,
			`CREATE INDEX IF NOT EXISTS idx_push_logs_create_time ON bm_push_logs(create_time);`,
		}

		for _, sql := range sqlList {
			_, err := g.DB().Exec(context.Background(), sql)
			if err != nil {
				g.Log().Error(context.Background(), "Failed to create push logs table:", err)
				return
			}
		}
	})
}
